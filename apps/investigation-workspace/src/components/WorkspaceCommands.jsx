import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import { ArrowRight, Search, X } from 'lucide-react'
import { configuredCaseIds, listEvidence } from '../lib/apiClient.js'
import { clearCaseSessionScope, useQuestionHistoryAcross } from '../lib/workspaceState.js'
import { isEditableTarget, routeShortcuts, shortcutSections } from '../lib/workspaceShortcuts.js'

const globalRoutes = [
  ['Dashboard', '/'], ['Ask across cases', '/investigate'], ['Cases', '/cases'], ['Activity', '/activity'], ['Settings', '/settings'],
]

export function WorkspaceCommands({ caseId = '', showTrigger = false }) {
  const navigate = useNavigate()
  const input = useRef(null)
  const palette = useRef(null)
  const shortcuts = useRef(null)
  const restoreFocus = useRef(true)
  const routeSequence = useRef({ armed: false, timer: null })
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [shortcutsOpen, setShortcutsOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)
  const [evidenceState, setEvidenceState] = useState({ status: 'idle', items: [] })
  const history = useQuestionHistoryAcross(configuredCaseIds())

  useEffect(() => {
    function keydown(event) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault(); restoreFocus.current = true; setPaletteOpen(true); return
      }
      if (event.key === '?' && !isEditableTarget(event.target) && !event.metaKey && !event.ctrlKey && !event.altKey) {
        event.preventDefault(); restoreFocus.current = true; setShortcutsOpen(true)
        return
      }
      if (event.key === '/' && !isEditableTarget(event.target) && !event.metaKey && !event.ctrlKey && !event.altKey) {
        event.preventDefault(); restoreFocus.current = true; setPaletteOpen(true); return
      }
      if (!isEditableTarget(event.target) && !event.metaKey && !event.ctrlKey && !event.altKey && event.key.toLowerCase() === 'g') {
        routeSequence.current.armed = true
        globalThis.clearTimeout(routeSequence.current.timer)
        routeSequence.current.timer = globalThis.setTimeout(() => { routeSequence.current.armed = false }, 1200)
        return
      }
      if (routeSequence.current.armed && !isEditableTarget(event.target)) {
        routeSequence.current.armed = false
        globalThis.clearTimeout(routeSequence.current.timer)
        const path = routeShortcuts[event.key.toLowerCase()]
        if (path) { event.preventDefault(); navigate(path) }
      }
    }
    globalThis.document.addEventListener('keydown', keydown)
    return () => { globalThis.document.removeEventListener('keydown', keydown); globalThis.clearTimeout(routeSequence.current.timer) }
  }, [navigate])

  useEffect(() => {
    if (!paletteOpen) return undefined
    setQuery(''); setActive(0)
    globalThis.setTimeout(() => input.current?.focus(), 0)
    if (!caseId) { setEvidenceState({ status: 'idle', items: [] }); return undefined }
    const controller = new AbortController()
    setEvidenceState({ status: 'loading', items: [] })
    listEvidence({ caseId, signal: controller.signal, filters: { limit: 100 } })
      .then(data => setEvidenceState({ status: 'ready', items: data?.items || [] }))
      .catch(error => { if (error.name !== 'AbortError') setEvidenceState({ status: 'unavailable', items: [] }) })
    return () => controller.abort()
  }, [paletteOpen, caseId])

  useEffect(() => {
    if (!paletteOpen && !shortcutsOpen) return undefined
    const node = paletteOpen ? palette.current : shortcuts.current
    const previous = globalThis.document.activeElement
    const timer = globalThis.setTimeout(() => (paletteOpen ? input.current : node?.querySelector('button, [tabindex]'))?.focus(), 0)
    function modalKeys(event) {
      if (event.key === 'Escape') {
        event.preventDefault(); setPaletteOpen(false); setShortcutsOpen(false); return
      }
      if (event.key !== 'Tab' || !node) return
      const items = [...node.querySelectorAll('button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])')]
      if (!items.length) return
      const first = items[0]; const last = items.at(-1)
      if (event.shiftKey && globalThis.document.activeElement === first) { event.preventDefault(); last.focus() }
      if (!event.shiftKey && globalThis.document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    globalThis.document.addEventListener('keydown', modalKeys)
    return () => {
      globalThis.clearTimeout(timer)
      globalThis.document.removeEventListener('keydown', modalKeys)
      if (restoreFocus.current) previous?.focus?.()
    }
  }, [paletteOpen, shortcutsOpen])

  const groups = useMemo(() => {
    const routes = globalRoutes.map(([label, path]) => ({ label, detail: 'Workspace destination', path }))
    const cases = configuredCaseIds().map(id => ({ label: id, detail: 'Configured case', path: `/cases/${encodeURIComponent(id)}/overview`, caseId: id }))
    const evidence = []
    const questions = [{ label: 'Ask across cases…', detail: 'Start with all authorized cases', path: '/investigate' }]
    const actions = [{ label: 'Create a case', detail: 'Start a new investigation workspace', path: '/cases/new' }]
    if (caseId) {
      questions.unshift({ label: `Ask in ${caseId}…`, detail: 'Start in the active case', path: `/cases/${encodeURIComponent(caseId)}/investigate`, caseId })
      actions.unshift({ label: 'Add evidence', detail: `Open evidence for ${caseId}`, path: `/cases/${encodeURIComponent(caseId)}/evidence`, caseId })
      for (const [label, route] of [['Overview', 'overview'], ['Evidence', 'evidence'], ['Ask this case', 'investigate'], ['Connections', 'connections'], ['Timeline', 'timeline'], ['Data', 'data'], ['Case activity', 'activity']]) {
        routes.push({ label, detail: 'Active case', path: `/cases/${encodeURIComponent(caseId)}/${route}`, caseId })
      }
      for (const item of evidenceState.items) evidence.push({
        label: item.original_filename || item.source_file || item.evidence_id,
        detail: 'Active-case evidence',
        path: `/cases/${encodeURIComponent(caseId)}/evidence/${encodeURIComponent(item.evidence_id)}`,
        caseId,
      })
    }
    // Saved questions from every case, so the palette opens as "pick up where you left off" (Linear shows recent
    // items when its search opens). They live in this browser only and the group says so.
    const asQuestion = item => ({
      label: item.label || item.query,
      detail: `${item.pinned ? 'Pinned' : 'Recent'} question · ${item.caseId}`,
      path: `/cases/${encodeURIComponent(item.caseId)}/investigate?question=${encodeURIComponent(item.query)}`,
      caseId: item.caseId,
    })
    questions.push(...history.filter(item => item.pinned).map(asQuestion), ...history.filter(item => !item.pinned).map(asQuestion))
    const needle = query.trim().toLocaleLowerCase()
    const filter = (items, limit) => items.filter(item => !needle || `${item.label} ${item.detail}`.toLocaleLowerCase().includes(needle)).slice(0, limit)
    const goTo = filter([...routes, ...cases, ...evidence], 8)
    const available = [
      { id: 'go-to', label: 'Go to', scope: caseId ? 'Workspace and active case' : 'Workspace', items: goTo, status: caseId ? evidenceState.status : undefined },
      { id: 'do', label: 'Do', scope: caseId ? 'Active case actions' : 'Workspace actions', items: filter(actions, 8) },
      { id: 'ask', label: 'Ask', scope: 'Starts a question only after selection', items: filter(questions, 8) },
    ]
    return available.filter(group => group.items.length > 0 || group.status === 'loading' || group.status === 'unavailable')
  }, [caseId, evidenceState, history, query])
  const commands = groups.flatMap(group => group.items)

  useEffect(() => { setActive(value => Math.min(value, Math.max(0, commands.length - 1))) }, [commands.length])

  function run(command) {
    if (!command) return
    // Focus return is correct when a modal is dismissed. Navigation is
    // different: returning to the old-route trigger after the new route has
    // mounted steals focus from the destination's primary task.
    restoreFocus.current = false
    setPaletteOpen(false)
    if (command.caseId && command.caseId !== caseId) clearCaseSessionScope()
    navigate(command.path)
  }

  function paletteKeys(event) {
    if (event.key === 'Escape') { event.preventDefault(); setPaletteOpen(false) }
    if (event.key === 'ArrowDown') { event.preventDefault(); setActive(value => Math.min(commands.length - 1, value + 1)) }
    if (event.key === 'ArrowUp') { event.preventDefault(); setActive(value => Math.max(0, value - 1)) }
    if (event.key === 'Enter') { event.preventDefault(); run(commands[active]) }
  }

  return <>
    {showTrigger ? <button type="button" className="command-trigger" aria-label="Quick find" aria-haspopup="dialog" aria-expanded={paletteOpen} onClick={() => { restoreFocus.current = true; setPaletteOpen(true) }}>
      <Search aria-hidden="true" />
      <span>Find a case, source or question</span><kbd>Ctrl K</kbd>
    </button> : null}
    {paletteOpen && typeof document !== 'undefined' ? createPortal(
      <div className="command-backdrop" onMouseDown={event => event.target === event.currentTarget && setPaletteOpen(false)}>
        <section ref={palette} className="command-palette" role="dialog" aria-modal="true" aria-labelledby="command-title" onKeyDown={paletteKeys}>
          <header><div><span className="eyebrow">Workspace commands</span><h2 id="command-title">Find or run a command</h2></div><button type="button" aria-label="Close command palette" onClick={() => setPaletteOpen(false)}><X aria-hidden="true" /></button></header>
          <label className="command-search"><span>Search destinations, actions and saved questions</span><span className="command-search__field"><Search aria-hidden="true" /><input ref={input} value={query} placeholder="Type a destination or action" onChange={event => { setQuery(event.target.value); setActive(0) }} role="combobox" aria-expanded="true" aria-controls="command-results" aria-autocomplete="list" aria-activedescendant={commands[active] ? `command-option-${active}` : undefined} /><kbd>Esc</kbd></span></label>
          <div id="command-results" className="command-results" role="listbox" aria-label="Quick find results">
            {groups.map(group => {
              let offset = 0
              for (const candidate of groups) { if (candidate.id === group.id) break; offset += candidate.items.length }
              return <section key={group.id} className="command-group" role="group" aria-labelledby={`command-group-${group.id}`}>
                <header><h3 id={`command-group-${group.id}`}>{group.label}</h3><small>{group.scope}</small></header>
                {group.items.map((command, index) => {
                  const commandIndex = offset + index
                  return <div id={`command-option-${commandIndex}`} key={`${command.path}-${command.label}`} role="option" aria-selected={active === commandIndex}><button type="button" tabIndex={-1} onMouseEnter={() => setActive(commandIndex)} onClick={() => run(command)}><span className="command-result__copy"><strong>{command.label}</strong><small>{command.detail}</small></span><ArrowRight aria-hidden="true" /></button></div>
                })}
                {group.id === 'go-to' && group.status === 'loading' ? <p className="command-group__state" role="status">Loading active-case evidence…</p> : null}
                {group.id === 'go-to' && group.status === 'unavailable' ? <p className="command-group__state">Evidence destinations are unavailable. Workspace and case routes remain available.</p> : null}
              </section>
            })}
            {!commands.length && query ? <p className="command-empty">No result matches this search. The scope was not widened.</p> : null}
          </div>
        </section>
      </div>,
      document.body
    ) : null}
    {shortcutsOpen && typeof document !== 'undefined' ? createPortal(
      <div className="command-backdrop" onMouseDown={event => event.target === event.currentTarget && setShortcutsOpen(false)}>
        <section ref={shortcuts} className="shortcut-sheet" role="dialog" aria-modal="true" aria-labelledby="shortcut-title">
          <header><h2 id="shortcut-title">Keyboard shortcuts</h2><button type="button" onClick={() => setShortcutsOpen(false)}>Close</button></header>
          <div className="shortcut-sheet__groups">{shortcutSections.map(section => <section key={section.group}><h3>{section.group}</h3><dl>{section.rows.map(row => <div key={`${section.group}-${row.keys.join('-')}`}><dt>{row.keys.map((key, index) => <span key={`${key}-${index}`}>{index ? <i aria-hidden="true">{row.sequence ? 'then' : row.together ? '+' : '/'}</i> : null}<kbd>{key}</kbd></span>)}</dt><dd>{row.what}</dd></div>)}</dl></section>)}</div>
        </section>
      </div>,
      document.body
    ) : null}
  </>
}
