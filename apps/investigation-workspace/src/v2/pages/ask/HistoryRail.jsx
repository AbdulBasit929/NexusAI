import { useMemo, useState } from 'react'
import { History, Pin, PinOff, Plus, Search } from 'lucide-react'
import { toggleQuestionPin } from '../../../lib/workspaceState.js'
import { OUTCOME_LABEL } from '../../../lib/acrossCases.js'

// Earlier questions of this case, newest first, pinned ones on top. They are kept in this browser only (the page says so); a question here is asked again
// when it is chosen, which is quick when the engine has answered it before. The server-side history replaces this list when the conversation service lands.
function groupOf(entry, today) {
  if (entry.pinned) return 'Pinned'
  const at = new Date(entry.updatedAt)
  const days = Math.floor((today - new Date(at.getFullYear(), at.getMonth(), at.getDate())) / 86_400_000)
  if (days <= 0) return 'Today'
  if (days === 1) return 'Yesterday'
  return 'Earlier'
}

export function HistoryRail({ caseId, entries, onPick, onNew, activeQuery }) {
  const [filter, setFilter] = useState('')
  const groups = useMemo(() => {
    const needle = filter.trim().toLocaleLowerCase()
    const now = new Date()
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
    const matching = entries.filter(entry => !needle || `${entry.label || ''} ${entry.query}`.toLocaleLowerCase().includes(needle))
    const out = new Map()
    for (const entry of matching) {
      const key = groupOf(entry, today)
      out.set(key, [...(out.get(key) || []), entry])
    }
    return ['Pinned', 'Today', 'Yesterday', 'Earlier'].filter(key => out.has(key)).map(key => ({ key, items: out.get(key) }))
  }, [entries, filter])

  return (
    <aside className="nx-history" aria-label="Question history">
      <header className="nx-history__head">
        <h2><History aria-hidden="true" />History</h2>
        <button type="button" className="nx-btn nx-btn--sm" onClick={onNew}><Plus aria-hidden="true" />New</button>
      </header>
      {entries.length > 6 ? (
        <label className="nx-history__search"><Search aria-hidden="true" /><span className="nx-visually-hidden">Search your earlier questions</span><input type="search" className="nx-input" placeholder="Search questions" value={filter} onChange={event => setFilter(event.target.value)} /></label>
      ) : null}
      <div className="nx-history__list">
        {groups.length ? groups.map(group => (
          <section key={group.key} aria-label={group.key}>
            <h3 className="nx-history__group">{group.key}</h3>
            <ul>
              {group.items.map(entry => (
                <li key={entry.id} className="nx-history__item" data-active={entry.query === activeQuery ? 'true' : undefined}>
                  <button type="button" className="nx-history__ask" onClick={() => onPick(entry)}>
                    <span className="nx-history__q" dir="auto">{entry.label || entry.query}</span>
                    <span className="nx-history__s">{OUTCOME_LABEL[entry.state] || 'Answered'}</span>
                  </button>
                  <button type="button" className="nx-btn nx-btn--ghost nx-btn--icon nx-btn--sm nx-history__pin" aria-pressed={entry.pinned} aria-label={entry.pinned ? 'Unpin this question' : 'Pin this question'} title={entry.pinned ? 'Unpin' : 'Pin'} onClick={() => toggleQuestionPin(caseId, entry.id)}>
                    {entry.pinned ? <PinOff aria-hidden="true" /> : <Pin aria-hidden="true" />}
                  </button>
                </li>
              ))}
            </ul>
          </section>
        )) : <p className="nx-history__empty">{entries.length ? 'No question matches.' : 'Questions you ask in this case appear here.'}</p>}
      </div>
      <p className="nx-history__note">Kept in this browser only.</p>
    </aside>
  )
}
