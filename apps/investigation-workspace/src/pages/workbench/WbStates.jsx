import { AlertCircle, CircleSlash, FileQuestion, Layers, MapPinOff, ServerCrash } from 'lucide-react'
import { STATES } from '../../lib/workbenchProof.js'

const ICONS = { empty: CircleSlash, broad: Layers, abstained: FileQuestion, partial: AlertCircle, clarify: FileQuestion, unsupported: FileQuestion, failed: ServerCrash, 'no-map': MapPinOff }

// The designed states of the workbench (the contract's list): each says what happened, why, and what to do next. They take the place of the picture, never a toast.
// Either a state of the design proof by id, or a state the live page describes itself: { icon, title, body, actions: [{ label, onClick }] }.
export function WbState({ id, icon, title, body, actions, compact = false, children }) {
  const proof = STATES[id]
  const Icon = ICONS[icon || id] || AlertCircle
  const heading = title || proof?.title
  if (!heading) return null
  const list = actions || (proof?.actions || []).map(label => ({ label }))
  return (
    <section className={`wbp-state wbp-state--${id || icon}${compact ? ' wbp-state--compact' : ''}`} role="status" aria-live="polite">
      <Icon size={compact ? 20 : 24} strokeWidth={1.5} aria-hidden="true" />
      <h2>{heading}</h2>
      <p>{body || proof?.body}</p>
      {children}
      {list.length ? <ul>{list.map(a => <li key={a.label}><button type="button" className="wbp-btn" onClick={a.onClick}>{a.label}</button></li>)}</ul> : null}
    </section>
  )
}
