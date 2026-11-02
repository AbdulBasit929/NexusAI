import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getAnswerReading } from '../lib/apiClient.js'

// What an answer rests on (docs/data/answer-evidence-and-interpretation-20261102.md). Two parts, both read from what the engine returned with the answer:
//  * AnswerEvidence: the rows behind the figure, the files they are in, and whether the figure was recomputed from them.
//  * AnswerReading: a few plain sentences by the local language model, asked for after the figure is on the screen.
// Neither changes the figure. Wording follows the engine's own: a record shows that something was recorded, not what it means.

const number = value => new Intl.NumberFormat('en-US').format(Number(value) || 0)
const plural = (count, word) => `${number(count)} ${word}${Number(count) === 1 ? '' : 's'}`

const CHECK_LABEL = {
  yes: { text: 'Recomputed from the rows: matches', tone: 'ok' },
  no: { text: 'Recomputed from the rows: does not match', tone: 'warn' },
  not_checked: { text: 'Not recomputed from the rows', tone: 'muted' },
}

function openLink(row, citations) {
  const item = (citations?.items || []).find(candidate => candidate.locator?.rowNumber === row.row_number && String(candidate.href || '').includes(row.evidence_id))
  return item?.href || ''
}

export function AnswerEvidence({ evidence, citations }) {
  if (!evidence) return null
  if (evidence.state !== 'itemised') {
    return (
      <p className="answer-basis__note" role="note">
        <strong>The records behind this answer are not listed.</strong> {evidence.reason ? `${evidence.reason[0].toUpperCase()}${evidence.reason.slice(1)}.` : ''}
      </p>
    )
  }
  const check = CHECK_LABEL[evidence.reconciled] || CHECK_LABEL.not_checked
  const files = evidence.files || []
  const rows = evidence.rows || []
  return (
    <details className="answer-basis__evidence">
      <summary>
        <span className="answer-basis__lead">Behind this answer: {plural(evidence.total_rows, 'row')} in {plural(files.length, 'file')}</span>
        <span className={`answer-basis__check answer-basis__check--${check.tone}`}>{check.text}</span>
      </summary>
      {evidence.check ? <p className="answer-basis__sentence">{evidence.check}</p> : null}
      {files.length > 0 ? (
        <table className="answer-basis__table" aria-label="Source files behind this answer">
          <thead><tr><th scope="col">Source file</th><th scope="col" className="answer-basis__num">Rows</th></tr></thead>
          <tbody>{files.map(file => <tr key={file.evidence_id || file.source_file}><td>{file.source_file || 'a file of this case'}</td><td className="answer-basis__num">{number(file.rows)}</td></tr>)}</tbody>
        </table>
      ) : null}
      {rows.length > 0 ? (
        <>
          <p className="answer-basis__sentence">{evidence.scope ? `Shown: ${evidence.scope}.` : 'Rows shown.'}{evidence.total_rows > rows.length ? ` ${number(rows.length)} of ${number(evidence.total_rows)}.` : ''}</p>
          <table className="answer-basis__table" aria-label="Rows behind this answer">
            <thead><tr><th scope="col">Source file</th><th scope="col" className="answer-basis__num">Row</th><th scope="col">What the row says</th><th scope="col"><span className="visually-hidden">Open</span></th></tr></thead>
            <tbody>
              {rows.map(row => {
                const href = openLink(row, citations)
                return (
                  <tr key={row.key}>
                    <td>{row.source_file || 'a file of this case'}</td>
                    <td className="answer-basis__num">{row.row_number || <span title="The row's place in the file could not be found">—</span>}</td>
                    <td className="answer-basis__preview">{row.preview || ''}</td>
                    <td>{href ? <Link to={href}>Open</Link> : null}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </>
      ) : null}
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
