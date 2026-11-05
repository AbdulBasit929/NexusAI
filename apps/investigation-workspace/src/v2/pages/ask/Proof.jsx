import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { ArrowUpRight, CircleAlert, CircleCheckBig, CircleHelp } from 'lucide-react'
import { recordHref } from '../../../components/AnswerBasis.jsx'
import { getEvidenceLine } from '../../../lib/apiClient.js'
import { formatNumber } from '../../../lib/format.js'
import { DataTable } from '../../ui/DataTable.jsx'

// What an answer rests on, in the right-hand panel (docs/data/answer-evidence-and-interpretation-20261102.md). All of it is read from what the engine returned with the answer:
// the records behind the figure in the words of an analyst (headings, the file and the row), whether the figure was recomputed from them, and, for an answer made of lines (the
// contacts of a number, the links of a path), each line with its own records, opened on request. Nothing here changes the figure.
const plural = (count, word) => `${formatNumber(count)} ${word}${Number(count) === 1 ? '' : 's'}`

const CHECK = {
  yes: { text: 'Recomputed from the records: matches', short: 'matches', tone: 'ok', Icon: CircleCheckBig },
  no: { text: 'Recomputed from the records: does not match', short: 'does not match', tone: 'warn', Icon: CircleAlert },
  not_checked: { text: 'Not recomputed from the records', short: 'not recomputed', tone: 'muted', Icon: CircleHelp },
}

export function CheckChip({ reconciled }) {
  const check = CHECK[reconciled] || CHECK.not_checked
  return <span className={`nx-check nx-check--${check.tone}`}><check.Icon aria-hidden="true" />{check.text}</span>
}

// The files the records come from, with how many records each gave.
function ProofFiles({ files, caseId }) {
  if (!files?.length || !files.some(file => file.rows > 0)) return null
  return (
    <ul className="nx-proof__files" aria-label="Source files behind this answer">
      {files.map(file => (
        <li key={file.evidence_id || file.source_file}>
          {file.evidence_id ? <Link to={`/cases/${encodeURIComponent(caseId)}/evidence/${encodeURIComponent(file.evidence_id)}`} dir="auto">{file.source_file || 'a file of this case'}</Link> : <span dir="auto">{file.source_file || 'a file of this case'}</span>}
          <span className="nx-tabular">{plural(file.rows, 'record')}</span>
        </li>
      ))}
    </ul>
  )
}

// The records, as a table under the file's own headings, each with the file and the row it was read from (a link that opens the file at that row).
function ProofTable({ evidence, caseId, total }) {
  const rows = evidence.rows || []
  const headings = evidence.columns || []
  const columns = useMemo(() => [
    ...(headings.length ? headings.map((heading, index) => ({ key: `c${index}`, label: heading })) : [{ key: 'preview', label: 'What the record says' }]),
    { key: 'where', label: 'In your file' },
  ], [headings])
  const data = useMemo(() => rows.map(row => {
    const entry = { id: row.key, where: row }
    if (headings.length) headings.forEach((_, index) => { entry[`c${index}`] = (row.cells || [])[index] || '' })
    else entry.preview = row.preview || ''
    return entry
  }), [rows, headings])
  if (!rows.length) return null
  const sentence = evidence.say ? `${evidence.say[0].toUpperCase()}${evidence.say.slice(1)}.` : evidence.scope ? `Listed: ${evidence.scope}.` : 'Records listed.'
  return (
    <div className="nx-proof__records">
      <p className="nx-proof__say">{sentence}{total > rows.length ? ` ${formatNumber(rows.length)} of ${formatNumber(total)} are listed.` : ''}</p>
      <DataTable columns={columns} rows={data} label="Records behind this answer" caption="The records behind this answer, each with the file and row it was read from" initialLimit={20} step={50} maxHeight={380} exportName="nexusai-records-behind-answer.csv"
        renderCell={(row, column, value) => {
          if (column.key !== 'where') return String(value)
          const source = value
          const href = recordHref(caseId, source)
          const where = `${source.source_file || 'a file of this case'}${source.row_number ? ` · row ${source.row_number}` : ''}`
          if (!source.row_number) return <span className="nx-muted" title="The place of this record in the file could not be found">{where} · row not found</span>
          return href ? <Link className="nx-where" to={href} title="Open the file at this row"><span dir="auto">{where}</span><ArrowUpRight aria-hidden="true" /></Link> : <span>{where}</span>
        }} />
    </div>
  )
}

// The lines of an answer made of summaries: how many records each stands for, and whether the line's own numbers came back from them. The first line's records are with the answer;
// any other is fetched when it is opened (and counted then).
function ProofLines({ evidence, caseId, fetchLine }) {
  const lines = evidence.lines || []
  const [selected, setSelected] = useState(1)
  const [fetched, setFetched] = useState({})
  const firstHere = (evidence.rows || []).length > 0
  const first = { rows: evidence.rows, files: evidence.files, columns: evidence.columns, say: evidence.say, total_rows: evidence.total_rows }
  const detail = selected === 1 && firstHere ? first : fetched[selected]
  useEffect(() => {
    if ((selected === 1 && firstHere) || fetched[selected] || !evidence.lines_token) return undefined
    const controller = new AbortController()
    fetchLine({ caseId, token: evidence.lines_token, line: selected, signal: controller.signal })
      .then(data => setFetched(state => ({ ...state, [selected]: { ...data.evidence, total_rows: data.evidence?.total_rows, summary: data.summary } })))
      .catch(error => { if (error?.name !== 'AbortError') setFetched(state => ({ ...state, [selected]: { failed: true } })) })
    return () => controller.abort()
  }, [selected, firstHere, evidence.lines_token, caseId, fetched, fetchLine])
  // what the list shows for a line: what came with the answer, or what was counted when the line was opened
  const shown = line => (line.pending && fetched[line.index]?.summary ? { ...line, ...fetched[line.index].summary, pending: false } : line)
  const biggest = Math.max(0, ...lines.map(line => Number(shown(line).records) || 0))
  const open = lines.find(line => line.index === selected)
  const openText = open ? shown(open).check : ''
  return (
    <>
      <ol className="nx-lines" aria-label="Lines of this answer">
        {lines.map(raw => {
          const line = shown(raw)
          const active = line.index === selected
          const check = CHECK[line.reconciled] || CHECK.not_checked
          const counted = !(line.pending || (!line.records && !line.files && line.reconciled === 'not_checked'))
          return (
            <li key={line.index}>
              <button type="button" className={`nx-line${active ? ' is-active' : ''}`} aria-pressed={active} onClick={() => setSelected(line.index)} title={active ? 'The records of this line are shown below' : 'Show the records of this line'}>
                <span className="nx-line__rank">{line.index}</span>
                <span className="nx-line__label" dir="auto">{line.label}</span>
                <span className="nx-line__records">
                  {counted ? <><span className="nx-line__bar" aria-hidden="true"><i style={{ '--share': biggest ? line.records / biggest : 0 }} /></span><span className="nx-tabular">{formatNumber(line.records)}</span></> : <span className="nx-muted">—</span>}
                </span>
                <span className={`nx-line__check nx-line__check--${check.tone}`} title={check.text}>{line.reconciled === 'yes' || line.reconciled === 'no' ? <check.Icon aria-hidden="true" /> : null}<span className="nx-visually-hidden">{check.text}</span></span>
              </button>
            </li>
          )
        })}
      </ol>
      {evidence.lines_total > lines.length ? <p className="nx-proof__say">{formatNumber(lines.length)} of {formatNumber(evidence.lines_total)} lines are listed.</p> : null}
      <div className="nx-proof__line" aria-live="polite">
        <h4>Records behind line {selected}{open ? `: ${open.label}` : ''}</h4>
        {openText && openText !== 'Counted when you open it.' ? <p className="nx-proof__say">{openText}</p> : null}
        {!detail ? <p className="nx-proof__say" role="status">Reading the records of line {selected}…</p> : detail.failed ? <p className="nx-note" role="note">The records of this line could not be listed.</p> : (
          <>
            <ProofFiles files={detail.files} caseId={caseId} />
            <ProofTable evidence={detail} caseId={caseId} total={open?.records || detail.total_rows} />
          </>
        )}
      </div>
    </>
  )
}

export function ProofRecords({ evidence, caseId, fetchLine = getEvidenceLine }) {
  if (!evidence) return null
  if (evidence.state !== 'itemised') {
    return <p className="nx-note" role="note"><strong>The records behind this answer are not listed.</strong> {evidence.reason ? `${evidence.reason[0].toUpperCase()}${evidence.reason.slice(1)}.` : ''}</p>
  }
  const isLines = evidence.kind === 'lines'
  const files = evidence.files || []
  return (
    <div className="nx-proof">
      <div className="nx-proof__lead">
        <strong>{isLines ? `${plural(evidence.lines_total || (evidence.lines || []).length, 'line')}, each standing for records` : `${plural(evidence.total_rows, 'record')} in ${plural(files.length, 'file')}`}</strong>
        <CheckChip reconciled={evidence.reconciled} />
      </div>
      {evidence.check ? <p className="nx-proof__say">{evidence.check}</p> : null}
      {isLines ? <ProofLines evidence={evidence} caseId={caseId} fetchLine={fetchLine} /> : (
        <>
          <ProofFiles files={files} caseId={caseId} />
          <ProofTable evidence={evidence} caseId={caseId} total={evidence.total_rows} />
        </>
      )}
    </div>
  )
}

// ---------- the sources: the files an answer draws on, each with its share of the records and a few places to open ----------
const SHAPE = { strong: 'observed', medium: 'inferred', weak: 'candidate' }
function StrengthMark({ shape }) {
  if (shape === 'observed') return <svg viewBox="0 0 12 12" aria-hidden="true"><circle cx="6" cy="6" r="4.5" fill="currentColor" /></svg>
  if (shape === 'inferred') return <svg viewBox="0 0 12 12" aria-hidden="true"><circle cx="6" cy="6" r="4.5" fill="none" stroke="currentColor" strokeWidth="1.5" /><path d="M6 1.5a4.5 4.5 0 0 1 0 9z" fill="currentColor" /></svg>
  return <svg viewBox="0 0 12 12" aria-hidden="true"><circle cx="6" cy="6" r="4.5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeDasharray="2.2 1.8" /></svg>
}

const internal = href => (typeof href === 'string' && href.startsWith('/') ? href : '')

export function ProofSources({ citations }) {
  const groups = citations?.groups || citations?.items || []
  if (!groups.length) return <p className="nx-note" role="note">No openable source accompanied this answer.</p>
  return (
    <ol className="nx-sources-list">
      {groups.map((group, index) => {
        const samples = group.samples || [group]
        const total = group.totalContributing
        const contributing = group.contributingCount || samples.length
        const share = total ? Math.max(2, Math.min(100, (contributing / total) * 100)) : null
        const shape = SHAPE[group.strength?.id] || 'candidate'
        return (
          <li key={`${group.href}-${index}`}>
            <details className="nx-source">
              <summary>
                <span className="nx-source__number" aria-hidden="true">{group.sourceNumber || index + 1}</span>
                <span className="nx-source__text">
                  <strong dir="auto">{group.label}</strong>
                  <small>{total ? `${formatNumber(contributing)} of ${formatNumber(total)} contributing rows` : `${samples.length} representative ${samples.length === 1 ? 'location' : 'locations'}`}</small>
                </span>
                {group.strength ? <span className="nx-strength" data-strength={shape}><StrengthMark shape={shape} />{group.strength.label}</span> : null}
                {share !== null ? <span className="nx-source__share" aria-hidden="true"><i style={{ inlineSize: `${share}%` }} /></span> : null}
              </summary>
              <ol className="nx-source__samples">
                {samples.map((sample, sampleIndex) => (
                  <li key={sample.href || sampleIndex}>
                    {internal(sample.href) ? <Link to={sample.href}><span>Place {sampleIndex + 1}</span><small dir="auto">{sample.detail}</small><ArrowUpRight aria-hidden="true" /></Link> : <a href={sample.href}><span>Place {sampleIndex + 1}</span><small dir="auto">{sample.detail}</small><ArrowUpRight aria-hidden="true" /></a>}
                  </li>
                ))}
              </ol>
            </details>
          </li>
        )
      })}
    </ol>
  )
}
