import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import { CornerDownLeft, Database, FileText, Folders, MessageSquareText, Search } from 'lucide-react'
import { configuredCaseIds, listEvidence } from '../../lib/apiClient.js'
import { clearCaseSessionScope, useQuestionHistoryAcross } from '../../lib/workspaceState.js'
import { humanizeCaseId } from '../lib/caseLabel.js'
import { caseNav, workspaceNav } from './nav.js'

// Search or jump to: cases, the evidence of the case you are in, pages, and your earlier questions. Typing a sentence offers to ask it.
// An ARIA combobox in a modal dialog: the input keeps focus, results are a listbox, the active result is announced.
const kindIcon = { go: Folders, evidence: Database, question: MessageSquareText, ask: Search, file: FileText }

export function CommandPalette({ open, onClose, caseId }) {
  const navigate = useNavigate()
  const input = useRef(null)
  const list = useRef(null)
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)
  const [evidence, setEvidence] = useState({ status: 'idle', items: [] })
  const history = useQuestionHistoryAcross(configuredCaseIds())

  useEffect(() => {
    if (!open) return undefined
    setQuery(''); setActive(0)
    globalThis.setTimeout(() => input.current?.focus(), 0)
    if (!caseId) { setEvidence({ status: 'idle', items: [] }); return undefined }
    const controller = new AbortController()
    setEvidence({ status: 'loading', items: [] })
    listEvidence({ caseId, signal: controller.signal, filters: { limit: 100 } })
      .then(data => setEvidence({ status: 'ready', items: data?.items || [] }))
      .catch(error => { if (error.name !== 'AbortError') setEvidence({ status: 'unavailable', items: [] }) })
    return () => controller.abort()
  }, [open, caseId])

  const groups = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase()
    const match = (...parts) => !needle || parts.join(' ').toLocaleLowerCase().includes(needle)
    const pages = workspaceNav.map(item => ({ kind: 'go', label: item.label, detail: 'Workspace', to: item.to }))
      .concat(caseId ? caseNav(caseId).map(item => ({ kind: 'go', label: item.label, detail: 'This case', to: item.to, caseId })) : [])
    const cases = configuredCaseIds().map(id => ({ kind: 'go', label: humanizeCaseId(id).title || id, detail: id, to: `/cases/${encodeURIComponent(id)}/overview`, caseId: id }))
    const files = evidence.items.map(item => ({
      kind: 'file', label: item.original_filename || item.source_file || item.evidence_id, detail: 'Evidence in this case',
      to: `/cases/${encodeURIComponent(caseId)}/evidence/${encodeURIComponent(item.evidence_id)}`, caseId,
    }))
    const asQuestion = item => ({
      kind: 'question', label: item.label || item.query, detail: `${item.pinned ? 'Pinned' : 'Recent'} question`,
      to: `/cases/${encodeURIComponent(item.caseId)}/investigate?question=${encodeURIComponent(item.query)}`, caseId: item.caseId,
    })
    const out = []
    if (caseId && needle.split(/\s+/).length >= 3) {
      out.push({ id: 'ask', label: 'Ask', items: [{ kind: 'ask', label: `Ask this case: ${query.trim()}`, detail: 'Opens Ask and runs the question', to: `/cases/${encodeURIComponent(caseId)}/investigate?question=${encodeURIComponent(query.trim())}&run=1`, caseId }] })
    }
    out.push(
      { id: 'pages', label: 'Go to', items: pages.filter(item => match(item.label, item.detail)).slice(0, 8) },
      { id: 'cases', label: 'Cases', items: cases.filter(item => match(item.label, item.detail)).slice(0, 6) },
      { id: 'files', label: 'Evidence', items: files.filter(item => match(item.label)).slice(0, 8), status: evidence.status },
      { id: 'questions', label: 'Earlier questions', items: history.map(asQuestion).filter(item => match(item.label)).slice(0, 6) },
    )
    return out.filter(group => group.items.length || group.status === 'loading')
  }, [query, caseId, evidence, history])
  const flat = groups.flatMap(group => group.items)

  useEffect(() => { setActive(value => Math.min(value, Math.max(0, flat.length - 1))) }, [flat.length])
  useEffect(() => { list.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' }) }, [active])

  if (!open || typeof document === 'undefined') return null

  function run(command) {
    if (!command) return
    onClose({ restoreFocus: false })
    if (command.caseId && command.caseId !== caseId) clearCaseSessionScope()
    navigate(command.to)
  }
  function keys(event) {
    if (event.key === 'Escape') { event.preventDefault(); onClose({ restoreFocus: true }) }
    if (event.key === 'ArrowDown') { event.preventDefault(); setActive(value => Math.min(flat.length - 1, value + 1)) }
    if (event.key === 'ArrowUp') { event.preventDefault(); setActive(value => Math.max(0, value - 1)) }
    if (event.key === 'Enter') { event.preventDefault(); run(flat[active]) }
    if (event.key === 'Tab') event.preventDefault()
  }
  let offset = 0
  return createPortal(
    <div className="nx-scope nx-overlay" onMouseDown={event => event.target === event.currentTarget && onClose({ restoreFocus: true })}>
      <div className="nx-dialog nx-palette" role="dialog" aria-modal="true" aria-label="Search or jump to" onKeyDown={keys}>
        <div className="nx-palette__field">
          <Search aria-hidden="true" />
          <input ref={input} value={query} placeholder="Search cases, evidence and questions, or type a question" aria-label="Search" role="combobox" aria-expanded="true" aria-controls="nx-palette-results" aria-activedescendant={flat.length ? `nx-palette-${active}` : undefined} aria-autocomplete="list"
            onChange={event => { setQuery(event.target.value); setActive(0) }} />
          <kbd className="nx-kbd" aria-hidden="true">Esc</kbd>
        </div>
        <div className="nx-palette__results" id="nx-palette-results" role="listbox" aria-label="Results" ref={list}>
          {groups.map(group => {
            const start = offset
            offset += group.items.length
            return (
              <div key={group.id} role="group" aria-label={group.label}>
                <div className="nx-palette__group">{group.label}</div>
                {group.items.map((command, index) => {
                  const Icon = kindIcon[command.kind] || Search
                  const position = start + index
                  return (
                    <div key={`${command.to}-${command.label}`} id={`nx-palette-${position}`} role="option" aria-selected={active === position} className="nx-palette__option" onMouseMove={() => setActive(position)} onClick={() => run(command)}>
                      <Icon aria-hidden="true" />
                      <span className="nx-palette__label">{command.label}</span>
                      <span className="nx-palette__detail">{command.detail}</span>
                      {active === position ? <CornerDownLeft className="nx-palette__enter" aria-hidden="true" /> : null}
                    </div>
                  )
                })}
                {group.id === 'files' && group.status === 'loading' ? <p className="nx-palette__note" role="status">Looking in this case’s evidence…</p> : null}
              </div>
            )
          })}
          {!flat.length && !evidence.status.startsWith('load') ? <p className="nx-palette__note">Nothing matches. Press Enter on a sentence of three words or more to ask it in this case.</p> : null}
        </div>
      </div>
    </div>,
    document.body,
  )
}
