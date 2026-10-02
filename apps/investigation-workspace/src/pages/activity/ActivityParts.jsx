import { Link } from 'react-router-dom'
import { Check, CheckCheck, Clock, Copy, FileText, MessageSquareText } from 'lucide-react'
import { Suspense, lazy, useState } from 'react'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { relativeAge } from '../../lib/dashboardCases.js'
import { OUTCOMES, asTimestamp, clockOf, exactTime, familyLabel, needsLook, outcomeLabel, outcomeOf, reviewedKey } from '../../lib/activityFeed.js'
import { pulseOption } from '../../lib/activityChart.js'
import { formatNumber } from '../../lib/format.js'

// Code-split with the other charts: ECharts is fetched only when a pulse is shown.
const EChart = lazy(() => import('../../components/charts/EChart.jsx'))

// An outcome as a dot and its words: calm at a glance, still readable without colour because the words are always there.
export function Status({ item }) {
  return <span className={`ac-status ac-status--${outcomeOf(item)}`}><i aria-hidden="true" />{outcomeLabel(item)}</span>
}

// The hero: the numbers that matter on the left, the pulse of the last weeks on the right. The line is drawn from the
// dated entries only and says so when none carries a time.
export function Pulse({ stats, data, keys, labelOf, caption }) {
  const hasLine = data.days.length > 0
  const chart = hasLine ? { buildOption: theme => pulseOption(data, theme, labelOf), label: `Entries per day over the last ${data.days.length} days: ${data.series.map(entry => `${labelOf(entry.key)} ${entry.total}`).join(', ')}` } : null
  return (
    <section className="ac-pulse" aria-label="Activity pulse">
      <dl className="ac-pulse__stats">
        {stats.map(stat => <div key={stat.label} className={stat.tone ? `is-${stat.tone}` : undefined}><dt>{stat.label}</dt><dd>{stat.value}</dd>{stat.note ? <small>{stat.note}</small> : null}</div>)}
      </dl>
      <div className="ac-pulse__chart">
        <header><h2>Pulse</h2><p>{caption}</p></header>
        {hasLine ? (
          <>
            <Suspense fallback={<p className="chart-card__loading" role="status">Loading chart…</p>}><EChart buildOption={chart.buildOption} height={132} label={chart.label} fill /></Suspense>
            <ul className="ac-pulse__legend" aria-label="Legend">{keys.map((key, index) => <li key={key} style={{ '--c': `var(--analyst-data-${(index % 6) + 1})` }}><i aria-hidden="true" />{labelOf(key)}</li>)}</ul>
          </>
        ) : <p className="ac-hint">No entry carries a time, so none can be placed on a day.</p>}
      </div>
    </section>
  )
}

// The journal as a table of entries under day headings: time, entry, case (when across cases) and status in aligned
// columns. A row is one button; the chosen row is marked with an accent bar and read in the pane beside it. Up and Down
// move the choice.
export function Feed({ groups, selectedId, onSelect, total, shown, showCase = false, reviewed = null, idOf = item => item.id }) {
  function onKey(event) {
    if (!['ArrowDown', 'ArrowUp'].includes(event.key)) return
    const buttons = [...event.currentTarget.querySelectorAll('button[data-id]')]
    const at = buttons.indexOf(globalThis.document.activeElement)
    const next = buttons[at + (event.key === 'ArrowDown' ? 1 : -1)]
    if (next) { event.preventDefault(); next.focus(); onSelect(next.dataset.id) }
  }
  return (
    <section className={`ac-feed${showCase ? ' ac-feed--cases' : ''}`} aria-labelledby="ac-feed-title">
      <header>
        <h2 id="ac-feed-title">Journal</h2>
        <p role="status">Showing <b>{formatNumber(shown)}</b> of {formatNumber(total)}</p>
      </header>
      <div className="ac-cols" aria-hidden="true"><span>Time</span><span>Entry</span><span>Status</span></div>
      <div className="ac-feed__body" onKeyDown={onKey}>
        {groups.map(group => (
          <section key={group.key || 'undated'} aria-label={group.label}>
            <h3 className="ac-day"><span>{group.label}</span><small>{formatNumber(group.items.length)}</small></h3>
            <ol aria-label="Activity">
              {group.items.map(item => {
                const Kind = item.kind === 'evidence' ? FileText : MessageSquareText
                const done = reviewed?.has(reviewedKey(item))
                return (
                  <li key={idOf(item)}>
                    <button type="button" data-id={idOf(item)} aria-current={idOf(item) === selectedId ? 'true' : undefined} className={done ? 'is-done' : undefined} onClick={() => onSelect(idOf(item))}>
                      <span className="ac-time">{asTimestamp(item.recordedAt) ? clockOf(item.recordedAt).replace(' UTC', '') : '—'}</span>
                      <span className="ac-entry"><Kind aria-hidden="true" /><span><b><LanguageText>{item.title}</LanguageText></b>{showCase || item.family ? <small>{showCase ? <bdi className="ac-casetag">{item.caseId}</bdi> : null}{showCase && item.family ? ' · ' : ''}{item.family ? familyLabel(item.family) : ''}</small> : null}</span></span>
                      {done ? <span className="ac-status ac-status--reviewed"><CheckCheck aria-hidden="true" />Reviewed</span> : <Status item={item} />}
                    </button>
                  </li>
                )
              })}
            </ol>
          </section>
        ))}
      </div>
    </section>
  )
}

// The chosen entry in full, as hairline-separated sections: what it is and how it ended, the answer or status (withheld text
// is marked, never shown as a result), the facts, and the actions that make sense.
export function Detail({ item, caseId, onReview = null, isReviewed = false, showCase = false }) {
  const [copied, setCopied] = useState(false)
  const time = asTimestamp(item.recordedAt)
  async function copy() {
    try { await globalThis.navigator.clipboard.writeText(item.query || item.title); setCopied(true); globalThis.setTimeout(() => setCopied(false), 1500) } catch { setCopied(false) }
  }
  return (
    <aside className="ac-detail" aria-label="Selected entry">
      <header>
        <p className="ac-kicker">{item.kind === 'evidence' ? 'Evidence update' : 'Question'}</p>
        <h2><LanguageText>{item.title}</LanguageText></h2>
        <p className="ac-detail__meta"><Status item={item} />{time ? <span title={exactTime(item.recordedAt)}><Clock aria-hidden="true" />{relativeAge(new Date(item.recordedAt))}</span> : <span>Time not reported</span>}</p>
      </header>
      <section className="ac-detail__section" aria-labelledby="ac-answer">
        <h3 id="ac-answer">{item.kind === 'evidence' ? 'Status' : 'Answer'}</h3>
        <LanguageText as="p" className={item.answerWithheld ? 'ac-answer ac-answer--withheld' : 'ac-answer'}>{item.answer}</LanguageText>
      </section>
      <dl className="ac-facts ac-detail__section">
        {showCase ? <div><dt>Case</dt><dd><Link to={`/cases/${encodeURIComponent(caseId)}`}><bdi>{caseId}</bdi></Link></dd></div> : null}
        <div><dt>Time</dt><dd>{exactTime(item.recordedAt)}</dd></div>
        <div><dt>Source</dt><dd>{item.sourceLabel}</dd></div>
        {item.scope.map((entry, index) => <div key={`${entry.label}-${entry.value}-${index}`}><dt>{entry.label}</dt><dd><LanguageText>{entry.value}</LanguageText></dd></div>)}
      </dl>
      <div className="ac-detail__actions">
        {item.kind === 'analysis'
          ? <Link className="ac-btn ac-btn--primary" to={`/cases/${encodeURIComponent(caseId)}/investigate?question=${encodeURIComponent(item.query)}`}><MessageSquareText aria-hidden="true" />Reopen with this question</Link>
          : <Link className="ac-btn ac-btn--primary" to={`/cases/${encodeURIComponent(caseId)}/evidence/${encodeURIComponent(item.evidenceId)}`}><FileText aria-hidden="true" />Open evidence</Link>}
        <div className="ac-detail__secondary">
          {onReview && needsLook(item) ? <button type="button" className="ac-btn" onClick={onReview} aria-pressed={isReviewed}><CheckCheck aria-hidden="true" />{isReviewed ? 'Reviewed. Mark as open again' : 'Mark as reviewed'}</button> : null}
          {item.kind === 'analysis' ? <button type="button" className="ac-btn" onClick={copy}>{copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}{copied ? 'Copied' : 'Copy question'}</button> : null}
        </div>
      </div>
    </aside>
  )
}

// How the entries in view split by outcome, as ruled bars; each row filters the journal where a filter is offered.
export function OutcomeCard({ counts, total, active, onPick }) {
  const peak = Math.max(1, ...OUTCOMES.map(group => counts[group.id]))
  return (
    <section className="ac-panel" aria-labelledby="ac-outcomes">
      <header><h2 id="ac-outcomes">Outcomes</h2><small>{formatNumber(total)} entries</small></header>
      <ul className="ac-legend" role="img" aria-label={OUTCOMES.map(group => `${group.label} ${counts[group.id]}`).join(', ')}>
        {OUTCOMES.map(group => {
          const share = total ? Math.floor((counts[group.id] / total) * 100) : 0
          const row = <><span>{group.label}</span><span className="ac-meter"><i className={`ac-seg ac-seg--${group.id}`} style={{ inlineSize: `${(counts[group.id] / peak) * 100}%` }} /></span><b>{formatNumber(counts[group.id])}</b><small>{share}%</small></>
          return <li key={group.id}>{onPick ? <button type="button" aria-pressed={active === group.id} onClick={() => onPick(active === group.id ? 'all' : group.id)}>{row}</button> : <div className="ac-legend__row">{row}</div>}</li>
        })}
      </ul>
    </section>
  )
}

export function CoverageCard() {
  return (
    <section className="ac-panel" aria-labelledby="ac-boundary-title">
      <header><h2 id="ac-boundary-title">Working record, not audit history</h2></header>
      <dl className="ac-cover">
        <div><dt><Check aria-hidden="true" />Included</dt><dd>Questions asked in this tab and saved in this browser, and the recent evidence entries the collection status reports.</dd></div>
        <div><dt className="is-no"><span aria-hidden="true">–</span>Not available</dt><dd>Who acted, custody, team-wide and export activity. The current service does not record them.</dd></div>
      </dl>
    </section>
  )
}
