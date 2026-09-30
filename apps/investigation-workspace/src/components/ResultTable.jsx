import { VirtualizedTable } from './VirtualizedTable.jsx'
import { LanguageText } from './AnalystComponents.jsx'

function normalized(value) { return String(value ?? '').replaceAll(',', '').trim().toLocaleLowerCase() }

export function ResultTable({ result, citations }) {
  if (!result.columns.length || !result.rows.length) return <p className="result-empty">No tabular result was returned.</p>
  function renderCell(row, column, value) {
    const citation = citations?.items?.find(item => item.claimValues?.some(candidate => normalized(candidate) === normalized(value)))
    const rowHref = row.__citationHref
    const content = <LanguageText>{value == null || value === '' ? '—' : value}</LanguageText>
    return citation || rowHref ? <a className="result-value-link" href={citation?.href || rowHref}>{content}<span className="visually-hidden"> — open source</span></a> : content
  }
  return (
    <>
      <VirtualizedTable columns={result.columns} rows={result.rows} totals={result.totals} label="Investigation result table" renderCell={renderCell} exportName="nexusai-investigation-result.csv" />
      {result.truncated && <p className="table-note">The service returned {result.availableRows.toLocaleString()} of {result.totalRows.toLocaleString()} matching rows. Export contains the returned rows only.</p>}
    </>
  )
}
