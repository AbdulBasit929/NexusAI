import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getAnswerReading, getEvidenceLine } from '../lib/apiClient.js'

// What an answer rests on (docs/data/answer-evidence-and-interpretation-20261102.md). Parts, all read from what the engine returned with the answer:
//  * AnswerEvidence: the records behind the figure, in the words of an analyst (headings, the file and the row), and whether the figure was recomputed from them. An answer that is made of lines (the
//    contacts of a number, the links of a path, the counterparties of an account) lists its lines, each with its records, and opens the records of any line on request.
//  * AnswerReading: a few plain sentences by the local language model, asked for after the figure is on the screen.
// Neither changes the figure. Wording follows the engine's own: a record shows that something was recorded, not what it means.

const number = value => new Intl.NumberFormat('en-US').format(Number(value) || 0)
const plural = (count, word) => `${number(count)} ${word}${Number(count) === 1 ? '' : 's'}`

const CHECK_LABEL = {
  yes: { text: 'Recomputed from the records: matches', tone: 'ok' },
  no: { text: 'Recomputed from the records: does not match', tone: 'warn' },
  not_checked: { text: 'Not recomputed from the records', tone: 'muted' },
}

// The link that opens a record in its file, at its row. It needs the file, its version and the row's own hash; a record that could not be placed in its file has no link.
export function recordHref(caseId, row) {
  if (!(row?.evidence_id && row.version_id && row.row_number && row.row_hash)) return ''
  const query = new URLSearchParams({ version: row.version_id, row: String(row.row_number), row_hash: row.row_hash })
  return `/cases/${encodeURIComponent(caseId)}/evidence/${encodeURIComponent(row.evidence_id)}?${query}`
}

function FilesTable({ files }) {
  if (!files?.length || !files.some(file => file.rows > 0)) return null
  return (
    <table className="answer-basis__table" aria-label="Source files behind this answer">
      <thead><tr><th scope="col">Source file</th><th scope="col" className="answer-basis__num">Records</th></tr></thead>
      <tbody>{files.map(file => <tr key={file.evidence_id || file.source_file}><td>{file.source_file || 'a file of this case'}</td><td className="answer-basis__num">{number(file.rows)}</td></tr>)}</tbody>
    </table>
  )
}

function RecordsTable({ evidence, caseId, total }) {
  const rows = evidence.rows || []
  const headings = evidence.columns || []
  if (!rows.length) return null
  return (
    <>
      <p className="answer-basis__sentence">
        {evidence.say ? `${evidence.say[0].toUpperCase()}${evidence.say.slice(1)}.` : evidence.scope ? `Listed: ${evidence.scope}.` : 'Records listed.'}
        {total > rows.length ? ` ${number(rows.length)} of ${number(total)} are listed.` : ''}
      </p>
      <div className="answer-basis__scroll">
        <table className="answer-basis__table" aria-label="Records behind this answer">
          <thead>
            <tr>
              {headings.map(heading => <th scope="col" key={heading}>{heading}</th>)}
              {headings.length === 0 ? <th scope="col">What the record says</th> : null}
              <th scope="col">In your file</th>
              <th scope="col"><span className="visually-hidden">Open</span></th>
            </tr>
          </thead>
          <tbody>
            {rows.map(row => {
              const href = recordHref(caseId, row)
              return (
                <tr key={row.key}>
                  {headings.map((heading, index) => <td key={heading}>{(row.cells || [])[index] || ''}</td>)}
                  {headings.length === 0 ? <td className="answer-basis__preview">{row.preview || ''}</td> : null}
                  <td>{row.source_file || 'a file of this case'}{row.row_number ? ` · row ${row.row_number}` : <span title="The place of this record in the file could not be found"> · row not found</span>}</td>
                  <td>{href ? <Link to={href}>Open in file</Link> : null}</td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </>
  )
}

// The lines of an answer made of summaries: how many records each stands for, and whether the line's own numbers came back from them. The first line's records are with the answer; any other is fetched.
function LinesView({ evidence, caseId, fetchLine }) {
  const lines = evidence.lines || []
  const [selected, setSelected] = useState(1)
  const [fetched, setFetched] = useState({})
  const firstHere = (evidence.rows || []).length > 0
  const first = { rows: evidence.rows, files: evidence.files, columns: evidence.columns, say: evidence.say, total_rows: evidence.total_rows }
  const detail = selected === 1 && firstHere ? first : fetched[selected]
  // a line whose records came with the answer is shown at once; any other, and the first when the time ran out, is fetched when it is opened (and counted then)
  useEffect(() => {
    if ((selected === 1 && firstHere) || fetched[selected] || !evidence.lines_token) return undefined
    const controller = new AbortController()
    fetchLine({ caseId, token: evidence.lines_token, line: selected, signal: controller.signal })
      .then(data => setFetched(state => ({ ...state, [selected]: { ...data.evidence, total_rows: data.evidence?.total_rows, summary: data.summary } })))
      .catch(error => { if (error?.name !== 'AbortError') setFetched(state => ({ ...state, [selected]: { failed: true } })) })
    return () => controller.abort()
  }, [selected, firstHere, evidence.lines_token, caseId, fetched, fetchLine])
  // what the table shows for a line: what came with the answer, or what was counted when the line was opened
  const shown = line => (line.pending && fetched[line.index]?.summary ? { ...line, ...fetched[line.index].summary, pending: false } : line)
  return (
    <>
      <div className="answer-basis__scroll">
        <table className="answer-basis__table" aria-label="Lines of this answer">
          <thead><tr><th scope="col" className="answer-basis__num">#</th><th scope="col">Line</th><th scope="col" className="answer-basis__num">Records</th><th scope="col" className="answer-basis__num">Files</th><th scope="col">Check</th><th scope="col"><span className="visually-hidden">Open</span></th></tr></thead>
          <tbody>
            {lines.map(raw => {
              const line = shown(raw)
              return (
              <tr key={line.index} className={line.index === selected ? 'is-selected' : undefined}>
                <td className="answer-basis__num">{line.index}</td>
                <td>{line.label}</td>
                <td className="answer-basis__num">{line.pending || (!line.records && !line.files && line.reconciled === 'not_checked') ? '—' : number(line.records)}</td>
                <td className="answer-basis__num">{line.files ? number(line.files) : '—'}</td>
                <td className={`answer-basis__check answer-basis__check--${(CHECK_LABEL[line.reconciled] || CHECK_LABEL.not_checked).tone}`}>{line.reconciled === 'yes' ? 'matches' : line.reconciled === 'no' ? 'does not match' : ''}</td>
                <td><button type="button" className="answer-basis__show" aria-pressed={line.index === selected} onClick={() => setSelected(line.index)}>{line.index === selected ? 'Showing records' : 'Show records'}</button></td>
              </tr>
              )
            })}
          </tbody>
        </table>
      </div>
      {evidence.lines_total > lines.length ? <p className="answer-basis__sentence">{number(lines.length)} of {number(evidence.lines_total)} lines are listed.</p> : null}
      {selected !== 1 || detail || !firstHere ? <h4 className="answer-basis__subhead">Records behind line {selected}: {lines.find(line => line.index === selected)?.label}</h4> : null}
      {(() => { const open = lines.find(line => line.index === selected); const text = open ? shown(open).check : ''; return text && text !== 'Counted when you open it.' ? <p className="answer-basis__sentence">{text}</p> : null })()}
      {!detail ? <p className="answer-basis__sentence" role="status">Reading the records of line {selected}…</p> : detail.failed ? <p className="answer-basis__note" role="note">The records of this line could not be listed.</p> : <>
        <FilesTable files={detail.files} />
        <RecordsTable evidence={detail} caseId={caseId} total={lines.find(line => line.index === selected)?.records || detail.total_rows} />
      </>}
    </>
  )
}

export function AnswerEvidence({ evidence, caseId, fetchLine = getEvidenceLine }) {
  if (!evidence) return null
  if (evidence.state !== 'itemised') {
    return (
      <p className="answer-basis__note" role="note">
        <strong>The records behind this answer are not listed.</strong> {evidence.reason ? `${evidence.reason[0].toUpperCase()}${evidence.reason.slice(1)}.` : ''}
      </p>
    )
  }
  const check = CHECK_LABEL[evidence.reconciled] || CHECK_LABEL.not_checked
  const isLines = evidence.kind === 'lines'
  const files = evidence.files || []
  return (
    <details className="answer-basis__evidence">
      <summary>
        <span className="answer-basis__lead">
          {isLines
            ? `Behind this answer: ${plural(evidence.lines_total || (evidence.lines || []).length, 'line')}, each standing for records`
            : `Behind this answer: ${plural(evidence.total_rows, 'record')} in ${plural(files.length, 'file')}`}
        </span>
        <span className={`answer-basis__check answer-basis__check--${check.tone}`}>{check.text}</span>
      </summary>
      {evidence.check ? <p className="answer-basis__sentence">{evidence.check}</p> : null}
      {isLines ? <LinesView evidence={evidence} caseId={caseId} fetchLine={fetchLine} /> : <>
        <FilesTable files={files} />
        <RecordsTable evidence={evidence} caseId={caseId} total={evidence.total_rows} />
      </>}
    </details>
  )
}

// The reading arrives after the figure. While it is written the card says so; when the model gives none that passes the check against the rows, it says that too.
export function AnswerReading({ caseId, reading, fetchReading = getAnswerReading }) {
  const token = reading?.state === 'pending' ? reading.token : ''
  const [result, setResult] = useState(null)
  useEffect(() => {
    if (!token) return undefined
    const controller = new AbortController()
    setResult(null)
    fetchReading({ caseId, token, signal: controller.signal })
      .then(value => setResult(value))
      .catch(error => { if (error?.name !== 'AbortError') setResult({ state: 'unavailable', reason: 'it could not be fetched; ask the question again' }) })
    return () => controller.abort()
  }, [caseId, token, fetchReading])
  if (!reading) return null
  if (!result) {
    return <p className="answer-basis__reading answer-basis__reading--waiting" role="status"><span className="ch-dots" aria-hidden="true"><i /><i /><i /></span>Reading the result…</p>
  }
  if (result.state !== 'ready') {
    return <p className="answer-basis__note" role="note">No reading is shown: {result.reason || 'none could be checked against the rows'}.</p>
  }
  return (
    <section className="answer-basis__reading" aria-label="Reading of this result by the language model">
      <h4>Reading of this result</h4>
      <p>{result.text}</p>
      <p className="answer-basis__disclosure">{result.disclosure}</p>
    </section>
  )
}
