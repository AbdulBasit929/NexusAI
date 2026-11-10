import { formatNumber } from '../../../lib/format.js'
import { Dialog } from '../../ui/Dialog.jsx'

// What changed between two runs of one question, from the service's comparison (GET /turns/{id}/diff or the `diff` of a re-run). It leads with the one sentence that matters: nothing changed, the
// figures changed, or the data did NOT change but the answer did. Rows are compared as sets (a table that only came in another order is "reordered", not changed). Only the first rows of each kind
// are listed; the counts are exact.
const LIST = 10
const cells = row => (Array.isArray(row) ? row.map(value => (value === null || value === undefined ? '—' : String(value))).join(' · ') : String(row))
const when = value => { const at = value ? new Date(value) : null; return at && !Number.isNaN(at.getTime()) ? at.toLocaleString([], { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' }) : '' }

export function diffLead(diff) {
  if (!diff) return ''
  if (!diff.changed) return 'Nothing changed: the same answer from the same data.'
  if (diff.same_data_different_answer) return `The data did not change, but the answer did. ${diff.query?.changed ? 'The model wrote a different query.' : 'The query is the same.'}`
  if (diff.figures_changed) return 'The figures changed.'
  if (diff.evidence_changed) return 'The evidence behind the answer changed.'
  return 'The wording of the answer changed.'
}

function Block({ title, children }) {
  return <section className="nx-diff__block"><h3>{title}</h3>{children}</section>
}

export function RunDiff({ diff, onClose }) {
  const rows = diff.rows || {}
  const detail = diff.data_version?.detail
  const before = when(diff.since)
  return (
    <Dialog title="What changed since the earlier run" wide closeLabel="Close the comparison" onClose={onClose}>
      <div className="nx-diff">
        <p className={`nx-diff__lead${diff.same_data_different_answer ? ' is-warn' : ''}`}>{diffLead(diff)}{before ? <span className="nx-muted"> Compared with the run of {before}.</span> : null}</p>
        {diff.complete === false ? <p className="nx-callout nx-callout--info" role="note">A result had more than 200 rows. The counts and the digest cover every row; the lists show the first ones.</p> : null}
        {diff.headline?.changed ? (
          <Block title="The answer">
            <dl className="nx-diff__pair">
              <div><dt>Before</dt><dd dir="auto">{diff.headline.before}</dd></div>
              <div><dt>Now</dt><dd dir="auto">{diff.headline.after}</dd></div>
            </dl>
          </Block>
        ) : null}
        {diff.metrics?.length ? (
          <Block title="Figures">
            <table className="nx-diff__table">
              <thead><tr><th scope="col">Figure</th><th scope="col">Before</th><th scope="col">Now</th><th scope="col">Difference</th></tr></thead>
              <tbody>{diff.metrics.map(metric => <tr key={metric.label}><th scope="row">{metric.label}</th><td className="nx-tabular">{metric.before}</td><td className="nx-tabular">{metric.after}</td><td className="nx-tabular">{metric.delta}</td></tr>)}</tbody>
            </table>
          </Block>
        ) : null}
        {rows.added_total || rows.removed_total || rows.changed_total || rows.before_count !== rows.after_count ? (
          <Block title={`Rows: ${formatNumber(rows.before_count || 0)} before, ${formatNumber(rows.after_count || 0)} now`}>
            {rows.reordered ? <p className="nx-muted">The same rows came in another order.</p> : null}
            {rows.added_total ? <><h4>{formatNumber(rows.added_total)} added</h4><ul className="nx-diff__list nx-diff__list--add">{rows.added.slice(0, LIST).map((row, index) => <li key={index} dir="auto">{cells(row)}</li>)}</ul></> : null}
            {rows.removed_total ? <><h4>{formatNumber(rows.removed_total)} removed</h4><ul className="nx-diff__list nx-diff__list--remove">{rows.removed.slice(0, LIST).map((row, index) => <li key={index} dir="auto">{cells(row)}</li>)}</ul></> : null}
            {rows.changed_total ? <><h4>{formatNumber(rows.changed_total)} changed</h4><ul className="nx-diff__list">{rows.changed.slice(0, LIST).map(item => <li key={item.key} dir="auto"><strong>{item.key}</strong>: {item.cells.map(cell => `${cell.column} ${cell.before} → ${cell.after}`).join('; ')}</li>)}</ul></> : null}
          </Block>
        ) : null}
        {diff.evidence && (diff.evidence.added?.length || diff.evidence.removed?.length || diff.evidence.replaced?.length) ? (
          <Block title="Evidence behind the answer">
            <ul className="nx-diff__list">
              {(diff.evidence.added || []).map(item => <li key={`a-${item.evidence_id}`}>Added: {item.name || item.evidence_id}</li>)}
              {(diff.evidence.removed || []).map(item => <li key={`r-${item.evidence_id}`}>No longer used: {item.name || item.evidence_id}</li>)}
              {(diff.evidence.replaced || []).map(item => <li key={`p-${item.evidence_id}`}>A new version of {item.name || item.evidence_id}</li>)}
            </ul>
          </Block>
        ) : null}
        {detail ? (
          <Block title="What changed in the case's data">
            <ul className="nx-diff__list">
              {(detail.evidence_added || []).map(item => <li key={`ea-${item.evidence_id}`}>File added: {item.name || item.evidence_id}</li>)}
              {(detail.evidence_removed || []).map(item => <li key={`er-${item.evidence_id}`}>File removed: {item.name || item.evidence_id}</li>)}
              {(detail.evidence_replaced || []).map(item => <li key={`ep-${item.evidence_id}`}>File replaced: {item.name || item.evidence_id}</li>)}
              {(detail.families || []).map(item => <li key={item.family}>{item.family}: {formatNumber(item.rows_before)} rows before, {formatNumber(item.rows_after)} now</li>)}
              {(detail.settings_changed || []).map(name => <li key={name}>Setting changed: {name}</li>)}
            </ul>
          </Block>
        ) : null}
        {diff.data_version?.known === false ? <p className="nx-muted">One of the runs has no stored version of the data, so a change in the data cannot be told from a change in the answer.</p> : null}
      </div>
    </Dialog>
  )
}
