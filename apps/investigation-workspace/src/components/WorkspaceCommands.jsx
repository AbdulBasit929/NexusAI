import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowRight, Search, X } from 'lucide-react'
import { configuredCaseIds, listEvidence } from '../lib/apiClient.js'
import { clearCaseSessionScope, useQuestionHistoryAcross } from '../lib/workspaceState.js'

function isTyping(target) {
  return target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || target?.isContentEditable
}

const globalRoutes = [
  ['Dashboard', '/'], ['Investigate', '/investigate'], ['Cases', '/cases'], ['Activity', '/activity'], ['Settings', '/settings'],
]

export function WorkspaceCommands({ caseId = '', showTrigger = false }) {
  const navigate = useNavigate()
  const input = useRef(null)
  const palette = useRef(null)
  const shortcuts = useRef(null)
  const restoreFocus = useRef(true)
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
      if (event.key === '?' && !isTyping(event.target) && !event.metaKey && !event.ctrlKey && !event.altKey) {
        event.preventDefault(); restoreFocus.current = true; setShortcutsOpen(true)
      }
    }
    globalThis.document.addEventListener('keydown', keydown)
    return () => globalThis.document.removeEventListener('keydown', keydown)
  }, [])

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
    const routes = globalRoutes.map(([label, path]) => ({ label, detail: 'Workspace', path }))
    const cases = configuredCaseIds().map(id => ({ label: id, detail: 'Configured case', path: `/cases/${encodeURIComponent(id)}/overview`, caseId: id }))
    const evidence = []
    const questions = []
    const actions = []
    if (caseId) {
      actions.push({ label: `Ask in ${caseId}…`, detail: 'Start an investigation', path: `/cases/${encodeURIComponent(caseId)}/investigate`, caseId })
      for (const [label, route] of [['Overview', 'overview'], ['Evidence', 'evidence'], ['Ask this case', 'investigate'], ['Timeline', 'timeline'], ['Case activity', 'activity']]) {
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
    const pinnedQuestions = history.filter(item => item.pinned).map(asQuestion)
    questions.push(...history.filter(item => !item.pinned).map(asQuestion))
    const needle = query.trim().toLocaleLowerCase()
    const filter = (items, limit) => items.filter(item => !needle || `${item.label} ${item.detail}`.toLocaleLowerCase().includes(needle)).slice(0, limit)
    const available = [
      { id: 'actions', label: 'Start here', scope: 'Active case', items: filter(actions, 2) },
      { id: 'pinned', label: 'Pinned questions', scope: 'This browser only', items: filter(pinnedQuestions, 5) },
      { id: 'questions', label: 'Recent questions', scope: 'This browser only', items: filter(questions, 6) },
      { id: 'cases', label: 'Cases', scope: 'Configured collections only', items: filter(cases, 8) },
      { id: 'evidence', label: 'Evidence', scope: caseId ? 'Up to 100 items returned for the active case' : 'Open a case to search its evidence', items: filter(evidence, 8), status: evidenceState.status },
      { id: 'routes', label: 'Routes', scope: 'Workspace and active case', items: filter(routes, 10) },
    ]
    // Empty groups are not drawn. Evidence stays while it loads or is unavailable, so the analyst is told why.
    const inScope = caseId ? available : available.filter(group => ['pinned', 'questions', 'cases', 'routes'].includes(group.id))
    return inScope.filter(group => group.items.length > 0 || group.status === 'loading' || group.status === 'unavailable')
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
    {paletteOpen ? <div className="command-backdrop" onMouseDown={event => event.target === event.currentTarget && setPaletteOpen(false)}>
      <section ref={palette} className="command-palette" role="dialog" aria-modal="true" aria-labelledby="command-title" onKeyDown={paletteKeys}>
        <header><div><span className="eyebrow">Workspace navigation</span><h2 id="command-title">Quick find</h2></div><button type="button" aria-label="Close quick find" onClick={() => setPaletteOpen(false)}><X aria-hidden="true" /></button></header>
        <label className="command-search"><span>Search configured cases, active-case evidence, routes and browser-local questions</span><span className="command-search__field"><Search aria-hidden="true" /><input ref={input} value={query} placeholder="Search the workspace" onChange={event => { setQuery(event.target.value); setActive(0) }} role="combobox" aria-expanded="true" aria-controls="command-results" aria-autocomplete="list" /><kbd>Esc</kbd></span></label>
        <div id="command-results" className="command-results" role="listbox" aria-label="Quick find results">
          {groups.map(group => {
            let offset = 0
            for (const candidate of groups) { if (candidate.id === group.id) break; offset += candidate.items.length }
            return <section key={group.id} className="command-group" role="group" aria-labelledby={`command-group-${group.id}`}>
              <header><h3 id={`command-group-${group.id}`}>{group.label}</h3><small>{group.scope}</small></header>
              {group.items.map((command, index) => {
                const commandIndex = offset + index
                return <div key={`${command.path}-${command.label}`} role="option" aria-selected={active === commandIndex}><button type="button" onMouseEnter={() => setActive(commandIndex)} onClick={() => run(command)}><span className="command-result__copy"><strong>{command.label}</strong><small>{command.detail}</small></span><ArrowRight aria-hidden="true" /></button></div>
              })}
              {group.id === 'evidence' && group.status === 'loading' ? <p className="command-group__state" role="status">Loading active-case evidence…</p> : null}
              {group.id === 'evidence' && group.status === 'unavailable' ? <p className="command-group__state">Evidence search is unavailable. Routes, configured cases and local questions remain available.</p> : null}
            </section>
          })}
          {!commands.length && query ? <p className="command-empty">No result matches this search. The scope was not widened.</p> : null}
        </div>
      </section>
    </div> : null}
    {shortcutsOpen ? <div className="command-backdrop" onMouseDown={event => event.target === event.currentTarget && setShortcutsOpen(false)}>
      <section ref={shortcuts} className="shortcut-sheet" role="dialog" aria-modal="true" aria-labelledby="shortcut-title">
        <header><h2 id="shortcut-title">Keyboard shortcuts</h2><button type="button" onClick={() => setShortcutsOpen(false)}>Close</button></header>
        <dl><div><dt><kbd>Ctrl</kbd>/<kbd>⌘</kbd> + <kbd>K</kbd></dt><dd>Open the command palette</dd></div><div><dt><kbd>?</kbd></dt><dd>Open this shortcut sheet</dd></div><div><dt><kbd>Alt</kbd> + <kbd>A</kbd></dt><dd>Focus the question composer</dd></div><div><dt><kbd>Esc</kbd></dt><dd>Close the active drawer or dialog</dd></div></dl>
      </section>
    </div> : null}
  </>
}
