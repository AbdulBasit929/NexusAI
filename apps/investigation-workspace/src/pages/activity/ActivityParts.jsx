import { Link } from 'react-router-dom'
import { AlertTriangle, Check, CircleSlash, Clock, Copy, FileText, MessageSquareText, X } from 'lucide-react'
import { useState } from 'react'
import { LanguageText } from '../../components/AnalystComponents.jsx'
import { relativeAge } from '../../lib/dashboardCases.js'
import { OUTCOMES, asTimestamp, clockOf, exactTime, familyLabel, outcomeLabel, outcomeOf } from '../../lib/activityFeed.js'
import { formatNumber } from '../../lib/format.js'

const TONE_ICON = { ok: Check, attention: AlertTriangle, failed: X, unavailable: CircleSlash }

export function OutcomeChip({ item }) {
  const tone = outcomeOf(item)
  const Icon = TONE_ICON[tone]
  return <span className={`ac-chip ac-chip--${tone}`}><Icon aria-hidden="true" />{outcomeLabel(item)}</span>
}

// The feed: entries grouped under sticky day headings. Each row is one button (kind mark, title, where it came from, the
// outcome and the time); the chosen row is marked and read in the inspector. Up and down move the choice.
export function Feed({ groups, selectedId, onSelect, total, shown }) {
  function onKey(event) {
    if (!['ArrowDown', 'ArrowUp'].includes(event.key)) return
    const buttons = [...event.currentTarget.querySelectorAll('button[data-id]')]
    const at = buttons.indexOf(globalThis.document.activeElement)
    const next = buttons[at + (event.key === 'ArrowDown' ? 1 : -1)]
    if (next) { event.preventDefault(); next.focus(); onSelect(next.dataset.id) }
  }
  return (
    <section className="ac-feed" aria-labelledby="ac-feed-title">
      <header>
        <h2 id="ac-feed-title">Journal</h2>
        <p role="status">Showing <b>{formatNumber(shown)}</b> of {formatNumber(total)}</p>
      </header>
      <div className="ac-feed__body" onKeyDown={onKey}>
        {groups.map(group => (
          <section key={group.key || 'undated'} aria-label={group.label}>
            <h3 className="ac-day">{group.label}<small>{formatNumber(group.items.length)}</small></h3>
            <ol aria-label="Activity">
              {group.items.map(item => {
                const Kind = item.kind === 'evidence' ? FileText : MessageSquareText
                return (
                  <li key={item.id}>
                    <button type="button" data-id={item.id} aria-current={item.id === selectedId ? 'true' : undefined} onClick={() => onSelect(item.id)}>
                      <span className="ac-time">{asTimestamp(item.recordedAt) ? clockOf(item.recordedAt).replace(' UTC', '') : '—'}</span>
                      <span className={`ac-kind ac-kind--${item.kind}`} aria-hidden="true"><Kind /></span>
                      <span className="ac-row__text"><b><LanguageText>{item.title}</LanguageText></b><small>{item.kind === 'evidence' ? 'Evidence' : 'Question'} · {item.sourceLabel}{item.family ? ` · ${familyLabel(item.family)}` : ''}</small></span>
                      <OutcomeChip item={item} />
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

// The chosen entry in full: outcome, exact time, where it came from, its scope, the answer (withheld text is marked, never
// shown as if it were a result) and the one action that makes sense.
export function Detail({ item, caseId }) {
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
        <div className="ac-detail__meta"><OutcomeChip item={item} />{time ? <span title={exactTime(item.recordedAt)}><Clock aria-hidden="true" />{relativeAge(new Date(item.recordedAt))}</span> : <span>Time not reported</span>}</div>
      </header>
      <div className="ac-detail__body">
        <section aria-labelledby="ac-answer">
          <h3 id="ac-answer">{item.kind === 'evidence' ? 'Status' : 'Answer'}</h3>
          <LanguageText as="p" className={item.answerWithheld ? 'ac-answer ac-answer--withheld' : 'ac-answer'}>{item.answer}</LanguageText>
        </section>
        <dl className="ac-facts">
          <div><dt>Time</dt><dd>{exactTime(item.recordedAt)}</dd></div>
          <div><dt>Source</dt><dd>{item.sourceLabel}</dd></div>
          {item.scope.map((entry, index) => <div key={`${entry.label}-${entry.value}-${index}`}><dt>{entry.label}</dt><dd><LanguageText>{entry.value}</LanguageText></dd></div>)}
        </dl>
      </div>
      <div className="ac-detail__actions">
        {item.kind === 'analysis'
          ? <Link className="ac-btn ac-btn--primary" to={`/cases/${encodeURIComponent(caseId)}/investigate?question=${encodeURIComponent(item.query)}`}><MessageSquareText aria-hidden="true" />Reopen with this question</Link>
          : <Link className="ac-btn ac-btn--primary" to={`/cases/${encodeURIComponent(caseId)}/evidence/${encodeURIComponent(item.evidenceId)}`}><FileText aria-hidden="true" />Open evidence</Link>}
        {item.kind === 'analysis' ? <button type="button" className="ac-btn" onClick={copy}>{copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}{copied ? 'Copied' : 'Copy question'}</button> : null}
      </div>
    </aside>
  )
}

// How the visible entries split by outcome; each row filters the journal.
export function OutcomeCard({ counts, total, active, onPick }) {
  return (
    <section className="ac-card" aria-labelledby="ac-outcomes">
      <header><h2 id="ac-outcomes">Outcomes</h2><small>{formatNumber(total)} entries</small></header>
      <div className="ac-stack" role="img" aria-label={OUTCOMES.map(group => `${group.label} ${counts[group.id]}`).join(', ')}>
        {OUTCOMES.map(group => counts[group.id] ? <i key={group.id} className={`ac-seg ac-seg--${group.id}`} style={{ flexGrow: counts[group.id] }} /> : null)}
      </div>
      <ul className="ac-legend">
        {OUTCOMES.map(group => {
          const share = total ? Math.floor((counts[group.id] / total) * 100) : 0
          return <li key={group.id}><button type="button" aria-pressed={active === group.id} onClick={() => onPick(active === group.id ? 'all' : group.id)}><i className={`ac-seg ac-seg--${group.id}`} aria-hidden="true" /><span>{group.label}</span><b>{formatNumber(counts[group.id])}</b><small>{share}%</small></button></li>
        })}
      </ul>
    </section>
  )
}

// Entries per day over the last two weeks that have any, as bars: the rhythm of work on this case.
export function RhythmCard({ days }) {
  const peak = Math.max(1, ...days.map(day => day.count))
  return (
    <section className="ac-card" aria-labelledby="ac-rhythm">
      <header><h2 id="ac-rhythm">Last 14 days</h2><small>Dated entries</small></header>
      {days.length ? (
        <ol className="ac-bars" aria-label="Entries per day">
          {days.map(day => <li key={day.day} title={`${day.day}: ${formatNumber(day.count)}`}><span style={{ blockSize: `${Math.max(day.count ? 8 : 2, (day.count / peak) * 100)}%` }} /><span className="visually-hidden">{day.day}: {formatNumber(day.count)} entries</span></li>)}
        </ol>
      ) : <p className="ac-hint">No entry carries a time, so none can be placed on a day.</p>}
    </section>
  )
}

export function CoverageCard() {
  return (
    <section className="ac-card" aria-labelledby="ac-boundary-title">
      <header><h2 id="ac-boundary-title">Working record, not audit history</h2></header>
      <ul className="ac-cover">
        <li><Check aria-hidden="true" /><span><b>Included</b>Questions asked in this tab and saved in this browser, and the recent evidence entries the collection status reports.</span></li>
        <li><X aria-hidden="true" /><span><b>Not available</b>Who acted, custody, team-wide and export activity. The current service does not record them.</span></li>
      </ul>
    </section>
  )
}
