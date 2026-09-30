import { useId, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Check, HardDrive, Pencil, Pin, Search, X } from 'lucide-react'
import { Card } from './Card.jsx'
import { LanguageText } from './AnalystComponents.jsx'
import { renameQuestionHistory, toggleQuestionPin, useQuestionHistoryAcross } from '../lib/workspaceState.js'

export function relativeTime(iso, now = Date.now()) {
  const then = Date.parse(iso)
  if (!Number.isFinite(then)) return ''
  const seconds = Math.max(0, Math.round((now - then) / 1000))
  if (seconds < 60) return 'just now'
  const format = new Intl.RelativeTimeFormat('en', { numeric: 'auto', style: 'narrow' })
  if (seconds < 3600) return format.format(-Math.floor(seconds / 60), 'minute')
  if (seconds < 86400) return format.format(-Math.floor(seconds / 3600), 'hour')
  if (seconds < 86400 * 30) return format.format(-Math.floor(seconds / 86400), 'day')
  return new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short' }).format(then)
}

export function filterQuestions(entries, needle) {
  const text = needle.trim().toLocaleLowerCase()
  if (!text) return entries
  return entries.filter(entry => `${entry.label || ''} ${entry.query} ${entry.caseId}`.toLocaleLowerCase().includes(text))
}

const keyOf = entry => `${entry.caseId}|${entry.id}`
const RECENT_LIMIT = 8

function Row({ entry, editing, label, setLabel, onBegin, onSave, onCancel }) {
  const name = entry.label || entry.query
  const when = relativeTime(entry.updatedAt)
  if (editing) {
    return (
      <li className="question-row question-row--editing">
        <form onSubmit={onSave}>
          <label><span className="visually-hidden">Rename question</span><input value={label} maxLength={120} autoFocus onChange={event => setLabel(event.target.value)} /></label>
          <button type="submit" aria-label="Save question name"><Check aria-hidden="true" /></button>
          <button type="button" aria-label="Cancel rename" onClick={onCancel}><X aria-hidden="true" /></button>
        </form>
      </li>
    )
  }
  return (
    <li className="question-row">
      <Link className="question-row__main" to={`/cases/${encodeURIComponent(entry.caseId)}/investigate?question=${encodeURIComponent(entry.query)}`} title={entry.label ? entry.query : undefined}>
        <LanguageText className="question-row__title">{name}</LanguageText>
        <span className="question-row__meta"><LanguageText as="bdi" identifier>{entry.caseId}</LanguageText>{when ? <> · {when}</> : null}</span>
      </Link>
      <div className="question-row__actions">
        <button type="button" aria-pressed={Boolean(entry.pinned)} aria-label={`${entry.pinned ? 'Unpin' : 'Pin'} question: ${name}`} title={entry.pinned ? 'Unpin' : 'Pin'} onClick={() => toggleQuestionPin(entry.caseId, entry.id)}><Pin aria-hidden="true" /></button>
        <button type="button" aria-label={`Rename question: ${name}`} title="Rename" onClick={() => onBegin(entry)}><Pencil aria-hidden="true" /></button>
      </div>
    </li>
  )
}

// Browsable question history across every case, shown where questions are asked and resumed. Pin and rename are
// visible buttons (there is room, and hidden actions are harder to reach). History lives in this browser only and
// says so; nothing here comes from the server.
export function QuestionHistory({ caseIds }) {
  const all = useQuestionHistoryAcross(caseIds)
  const [needle, setNeedle] = useState('')
  const [showAll, setShowAll] = useState(false)
  const [editing, setEditing] = useState('')
  const [label, setLabel] = useState('')
  const filterId = useId()

  const matches = useMemo(() => filterQuestions(all, needle), [all, needle])
  const pinned = matches.filter(entry => entry.pinned)
  const recentAll = matches.filter(entry => !entry.pinned)
  const recent = showAll ? recentAll : recentAll.slice(0, RECENT_LIMIT)

  function rows(list) {
    return list.map(entry => (
      <Row key={keyOf(entry)} entry={entry} editing={editing === keyOf(entry)} label={label} setLabel={setLabel}
        onBegin={target => { setEditing(keyOf(target)); setLabel(target.label || target.query) }}
        onSave={event => { event.preventDefault(); renameQuestionHistory(entry.caseId, entry.id, label); setEditing('') }}
        onCancel={() => setEditing('')} />
    ))
  }

  return (
    <Card
      className="question-history"
      title="Your questions"
      description="Pick up where you left off. Pinned questions stay on top."
      actions={<span className="question-history__scope"><HardDrive aria-hidden="true" />Saved in this browser only</span>}
    >
      {all.length === 0 ? (
        <p className="question-history__empty">Questions you ask are saved here so you can reopen them.</p>
      ) : (
        <>
          <div className="question-history__filter">
            <label htmlFor={filterId} className="visually-hidden">Filter your questions</label>
            <Search aria-hidden="true" />
            <input id={filterId} type="search" value={needle} placeholder="Filter by question or case" onChange={event => setNeedle(event.target.value)} />
          </div>
          {matches.length === 0 ? <p className="question-history__empty" role="status">No saved question matches. Only your saved questions were searched.</p> : null}
          {pinned.length ? <section aria-labelledby={`${filterId}-pinned`}><h3 id={`${filterId}-pinned`}>Pinned</h3><ul>{rows(pinned)}</ul></section> : null}
          {recentAll.length ? (
            <section aria-labelledby={`${filterId}-recent`}>
              <h3 id={`${filterId}-recent`}>Recent</h3>
              <ul>{rows(recent)}</ul>
              {recentAll.length > RECENT_LIMIT ? <button type="button" className="question-history__more" aria-expanded={showAll} onClick={() => setShowAll(value => !value)}>{showAll ? 'Show fewer' : `Show all ${recentAll.length}`}</button> : null}
            </section>
          ) : null}
        </>
      )}
    </Card>
  )
}
