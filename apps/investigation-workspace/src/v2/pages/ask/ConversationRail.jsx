import { useEffect, useMemo, useRef, useState } from 'react'
import { Archive, Check, EllipsisVertical, History, Pencil, Pin, PinOff, Plus, Search, Trash2, Undo2 } from 'lucide-react'
import { relativeAge } from '../../../lib/dashboardCases.js'
import { Menu } from '../../shell/Menu.jsx'

// The conversations of this case, from the conversation service: pinned first, then by the day of the last question. Words typed in the search box are matched by the server in every
// question and answer (Urdu script, Roman Urdu and numbers are folded); a conversation that matched shows the question that did. A conversation with a newer run that differs carries
// a "Changed" mark. Nothing is deleted: "Delete" hides it, and the line below the list offers to bring it back.
const OUTCOME = { answered: 'Answered', abstained: 'No answer', clarification: 'Needed a detail', withheld: 'Withheld', unsupported: 'Out of scope', refused: 'Declined', failed: 'Failed' }

export function dayGroup(item, now = new Date()) {
  if (item.pinned) return 'Pinned'
  const at = new Date(item.last_turn_at || item.created_at)
  if (Number.isNaN(at.getTime())) return 'Earlier'
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const days = Math.floor((today - new Date(at.getFullYear(), at.getMonth(), at.getDate())) / 86_400_000)
  if (days <= 0) return 'Today'
  if (days === 1) return 'Yesterday'
  if (days < 7) return 'This week'
  return 'Earlier'
}
const ORDER = ['Pinned', 'Today', 'Yesterday', 'This week', 'Earlier']

function Rename({ item, onDone }) {
  const [value, setValue] = useState(item.title || item.display_title || '')
  const input = useRef(null)
  useEffect(() => { input.current?.focus(); input.current?.select() }, [])
  const save = () => onDone(value)
  return (
    <form className="nx-history__rename" onSubmit={event => { event.preventDefault(); save() }}>
      <label className="nx-visually-hidden" htmlFor={`rename-${item.thread_id}`}>Name this conversation</label>
      <input id={`rename-${item.thread_id}`} ref={input} className="nx-input" value={value} maxLength={200} onChange={event => setValue(event.target.value)} onKeyDown={event => { if (event.key === 'Escape') { event.preventDefault(); onDone(null) } }} />
      <button type="submit" className="nx-btn nx-btn--ghost nx-btn--icon nx-btn--sm" aria-label="Save the name" title="Save"><Check aria-hidden="true" /></button>
    </form>
  )
}

export function ConversationRail({ rail, activeId, onOpen, onNew, query, onQuery }) {
  const [renaming, setRenaming] = useState(null)
  const [hidden, setHidden] = useState(null)
  const groups = useMemo(() => {
    const out = new Map()
    for (const item of rail.items) out.set(dayGroup(item), [...(out.get(dayGroup(item)) || []), item])
    return ORDER.filter(key => out.has(key)).map(key => ({ key, items: out.get(key) }))
  }, [rail.items])
  useEffect(() => {
    if (!hidden) return undefined
    const timer = globalThis.setTimeout(() => setHidden(null), 9000)
    return () => globalThis.clearTimeout(timer)
  }, [hidden])
  const searching = Boolean(query.trim())

  return (
    <aside className="nx-history" aria-label="Conversations">
      <header className="nx-history__head">
        <h2><History aria-hidden="true" />History</h2>
        <button type="button" className="nx-btn nx-btn--sm" onClick={onNew}><Plus aria-hidden="true" />New</button>
      </header>
      <label className="nx-history__search"><Search aria-hidden="true" /><span className="nx-visually-hidden">Search conversations</span><input type="search" className="nx-input" placeholder="Search questions and answers" value={query} onChange={event => onQuery(event.target.value)} /></label>
      <div className="nx-history__list">
        {groups.length ? groups.map(group => (
          <section key={group.key} aria-label={group.key}>
            <h3 className="nx-history__group">{group.key}</h3>
            <ul>
              {group.items.map(item => (
                <li key={item.thread_id} className="nx-history__item" data-active={item.thread_id === activeId ? 'true' : undefined}>
                  {renaming === item.thread_id ? (
                    <Rename item={item} onDone={async name => { setRenaming(null); if (name !== null) await rail.rename(item.thread_id, name) }} />
                  ) : (
                    <>
                      <button type="button" className="nx-history__ask" aria-current={item.thread_id === activeId ? 'true' : undefined} onClick={() => onOpen(item)}>
                        <span className="nx-history__q" dir="auto">{item.display_title}</span>
                        <span className="nx-history__s">
                          {[item.turn_count > 1 ? `${item.turn_count} questions` : '', OUTCOME[item.last_outcome] || '', item.last_turn_at ? relativeAge(new Date(item.last_turn_at)) : ''].filter(Boolean).join(' · ')}
                          {item.changed_since ? <span className="nx-history__changed" title="A question here has a newer run that gives a different answer">Changed</span> : null}
                        </span>
                        {searching && item.matches?.[0] ? <span className="nx-history__match" dir="auto">{item.matches[0].question}</span> : null}
                      </button>
                      <Menu label={`More for ${item.display_title}`} icon={<EllipsisVertical aria-hidden="true" />} buttonClassName="nx-btn nx-btn--ghost nx-btn--icon nx-btn--sm nx-history__pin">
                        {close => (
                          <>
                            <button type="button" role="menuitem" className="nx-menu__item" onClick={() => { close(); rail.pin(item.thread_id, !item.pinned) }}>{item.pinned ? <PinOff aria-hidden="true" /> : <Pin aria-hidden="true" />}{item.pinned ? 'Unpin' : 'Pin'}</button>
                            <button type="button" role="menuitem" className="nx-menu__item" onClick={() => { close(); setRenaming(item.thread_id) }}><Pencil aria-hidden="true" />Rename</button>
                            <button type="button" role="menuitem" className="nx-menu__item" onClick={() => { close(); rail.archive(item.thread_id, true) }}><Archive aria-hidden="true" />Archive</button>
                            <button type="button" role="menuitem" className="nx-menu__item" onClick={async () => { close(); await rail.hide(item.thread_id); setHidden(item) }}><Trash2 aria-hidden="true" />Delete</button>
                          </>
                        )}
                      </Menu>
                    </>
                  )}
                </li>
              ))}
            </ul>
          </section>
        )) : <p className="nx-history__empty">{rail.status === 'loading' ? 'Reading the history…' : rail.status === 'error' ? (rail.error || 'The history could not be read.') : searching ? 'No conversation matches.' : 'Questions you ask in this case appear here.'}</p>}
        {rail.next ? <button type="button" className="nx-btn nx-btn--ghost nx-btn--sm nx-history__more" onClick={rail.loadMore}>Show more</button> : null}
      </div>
      {hidden ? (
        <p className="nx-history__note" role="status">Deleted “{hidden.display_title}”. <button type="button" className="nx-link-button" onClick={async () => { await rail.restore(hidden.thread_id); setHidden(null) }}><Undo2 aria-hidden="true" />Undo</button></p>
      ) : <p className="nx-history__note">Saved in this case for the people who may open it.</p>}
    </aside>
  )
}
