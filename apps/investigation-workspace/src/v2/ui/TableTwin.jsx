import { ChevronRight } from 'lucide-react'
import { DataTable } from './DataTable.jsx'

// Every chart has its twin (directive 7.6): the same marks as a table under the same names, sortable, filterable, with Copy as CSV, and a sentence that says what is drawn.
// The sentence is the chart's description for a screen reader; the table is the keyboard route to every value.
export function TableTwin({ label, columns, rows, exportName, summary }) {
  if (!rows.length) return null
  return (
    <>
      {summary ? <p className="nx-visually-hidden">{summary}</p> : null}
      <details className="nx-disclosure nx-twin">
        <summary><ChevronRight aria-hidden="true" />Show as table</summary>
        <div className="nx-twin__body"><DataTable columns={columns} rows={rows} label={label} caption={label} initialLimit={12} exportName={exportName} /></div>
      </details>
    </>
  )
}
