import { useEffect, useState } from 'react'
import { MapPin } from 'lucide-react'
import { getSourceRow } from '../lib/apiClient.js'
import { readSpreadsheetDate } from '../lib/spreadsheetDates.js'
import { LanguageText } from './AnalystComponents.jsx'

// The row an answer points at, as the analyst's own file has it: the column names of the file (MSISDN, STRT_TM, BNUMBER ...) and the values in that row, in the file's own column order, each with the
// column's letter when the file is a workbook. No identifier, no hash: those are in the page's technical details. When the row cannot be shown by column name the card says so and why, and the rows
// of the file stay available below it.
export function SourceRowCard({ caseId, evidenceId, row, rowHash, fetchRow = getSourceRow }) {
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
    return (
      <section className="srow srow--waiting" role="status" aria-label={`Row ${row} of the file`}>
        <span className="srow__icon" aria-hidden="true"><MapPin /></span>
        <p>Reading row {row} of the file…</p>
      </section>
    )
  }
  if (state.error || state.data?.state !== 'found') {
    const reason = state.data?.reason || 'the row could not be read'
    return (
      <section className="srow srow--missing" aria-label={`Row ${row} of the file`}>
        <span className="srow__icon" aria-hidden="true"><MapPin /></span>
        <div>
          <h2>Row {row}</h2>
          <p>This row cannot be shown by column name: {reason}. The rows of the file are listed below.</p>
        </div>
      </section>
    )
  }
  const { data } = state
  return (
    <section className="srow" aria-label="This row in your file">
      <header className="srow__head">
        <span className="srow__icon" aria-hidden="true"><MapPin /></span>
        <div>
          <h2>This row in your file</h2>
          <p><LanguageText>{data.source_file}</LanguageText> · row {data.row_number}{data.kind ? ` · ${data.kind}` : ''}</p>
        </div>
      </header>
      <dl className="srow__cells">
        {data.cells.map(cell => {
          const date = readSpreadsheetDate(cell.name, cell.value)
          return (
            <div key={cell.name}>
              <dt>
                {cell.cell ? <span className="srow__col" title={`Cell ${cell.cell} of the workbook`}>{cell.cell.replace(/\d+$/, '')}</span> : null}
                <LanguageText>{cell.name}</LanguageText>
              </dt>
              <dd>
                <LanguageText>{cell.value}</LanguageText>
                {date ? <small className="srow__reading">As a date: <b>{date}</b></small> : null}
              </dd>
            </div>
          )
        })}
      </dl>
      <p className="srow__note">The column names and the values are as the file has them; empty cells are left out.</p>
    </section>
  )
}
