import { ClaimText } from './Citations.jsx'
import { ResultTable } from './ResultTable.jsx'
import { LanguageText } from './AnalystComponents.jsx'

export function ResultComparison({ items, onRemove, onClear }) {
  if (!items.length) return null
  return <section className="result-comparison" aria-labelledby="comparison-heading"><header><div><p className="eyebrow">Working comparison</p><h2 id="comparison-heading">Compare results</h2></div><button type="button" onClick={onClear}>Clear comparison</button></header><div className="result-comparison__grid">{items.map(item => <article key={item.id}><header><LanguageText as="h3">{item.query}</LanguageText><button type="button" onClick={() => onRemove(item.id)}>Remove</button></header><p className="comparison-answer"><ClaimText text={item.presentation.answer} segments={item.presentation.claimSegments} citations={item.presentation.citations.markerItems || item.presentation.citations.items} /></p>{item.presentation.result.rows.length ? <ResultTable result={item.presentation.result} citations={item.presentation.citations} /> : <p>No tabular result accompanied this answer.</p>}</article>)}</div>{items.length < 2 ? <p className="comparison-hint">Add one more completed result to compare it side by side.</p> : null}</section>
}
