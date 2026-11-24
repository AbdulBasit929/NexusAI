import { useEffect, useState } from 'react'
import { MapPin } from 'lucide-react'
import { LanguageText } from '../../../components/AnalystComponents.jsx'
import { getSourceRow } from '../../../lib/apiClient.js'
import { readSpreadsheetDate } from '../../../lib/spreadsheetDates.js'

// The row an answer points at, as the analyst's own file has it: the column names of the file and the values of that row, in the file's own order, each with the column's letter when the
// file is a workbook. One card, one line to say where it is, then the values. No identifier and no hash (those are in the page's technical details). When the row cannot be shown by column
// name the card says so and why; the rows of the file stay available under it.
export function CitedRow({ caseId, evidenceId, row, rowHash, fetchRow = getSourceRow }) {
  const [state, setState] = useState({ loading: true })
  useEffect(() => {
    if (!row) return undefined
    const controller = new AbortController()
    setState({ loading: true })
    fetchRow({ caseId, evidenceId, row, rowHash, signal: controller.signal })
      .then(data => setState({ data, loading: false }))
      .catch(error => { if (error?.name !== 'AbortError') setState({ error, loading: false }) })
    return () => controller.abort()
  }, [caseId, evidenceId, row, rowHash, fetchRow])
  if (!row) return null
  if (state.loading) {
    return <section className="nx-cited nx-cited--quiet" role="status" aria-label={`Row ${row} of the file`}><MapPin aria-hidden="true" /><p>Reading row {row} of the file…</p></section>
  }
  if (state.error || state.data?.state !== 'found') {
    const reason = state.data?.reason || 'the row could not be read'
    return (
      <section className="nx-cited nx-cited--quiet" aria-label={`Row ${row} of the file`}>
        <MapPin aria-hidden="true" />
        <p><strong>Row {row}</strong> cannot be shown by column name: {reason}. The rows of the file are listed below.</p>
      </section>
    )
  }
  const { data } = state
  return (
    <section className="nx-cited" aria-label="The row your answer points at">
      <header className="nx-cited__head">
        <MapPin aria-hidden="true" />
        <h2>Row {data.row_number}</h2>
        <p>cited by your answer{data.kind ? ` · ${data.kind}` : ''}</p>
      </header>
      <dl className="nx-cited__cells">
        {data.cells.map(cell => {
          const date = readSpreadsheetDate(cell.name, cell.value)
          return (
            <div key={cell.name}>
              <dt>{cell.cell ? <span className="nx-rows__letter" title={`Cell ${cell.cell} of the workbook`}>{cell.cell.replace(/\d+$/, '')}</span> : null}<LanguageText>{cell.name}</LanguageText></dt>
              <dd><LanguageText>{cell.value}</LanguageText>{date ? <small>As a date: <b>{date}</b></small> : null}</dd>
            </div>
          )
        })}
      </dl>
    </section>
  )
}
