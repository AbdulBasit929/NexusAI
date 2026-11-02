import { useEffect, useState } from 'react'
import { getSourceRow } from '../lib/apiClient.js'
import { LanguageText } from './AnalystComponents.jsx'

// The row an answer points at, as the analyst's own file has it: the column names of the file (MSISDN, STRT_TM, BNUMBER ...) and the values in that row. No identifier, no hash: those are in the
// page's technical details. When the row cannot be shown by column name the card says so and why, and the stored records of the file stay available below it.
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
  if (state.loading) return <p className="source-row source-row--waiting" role="status">Reading row {row} of the file…</p>
  if (state.error || state.data?.state !== 'found') {
    const reason = state.data?.reason || 'the row could not be read'
    return (
      <section className="source-row source-row--missing" aria-label={`Row ${row} of the file`}>
        <h2>Row {row}</h2>
        <p>This row cannot be shown by column name: {reason}. The stored records of the file are listed below.</p>
      </section>
    )
  }
  const { data } = state
  return (
    <section className="source-row" aria-label="This row in your file">
      <header>
        <h2>This row in your file</h2>
        <p><LanguageText>{data.source_file}</LanguageText> · row {data.row_number}{data.kind ? ` · ${data.kind}` : ''}</p>
      </header>
      <dl className="source-row__cells">
        {data.cells.map(cell => <div key={cell.name}><dt><LanguageText>{cell.name}</LanguageText></dt><dd><LanguageText>{cell.value}</LanguageText></dd></div>)}
      </dl>
      <p className="source-row__note">The column names and the values are as the file has them; empty cells are left out.</p>
    </section>
  )
}
