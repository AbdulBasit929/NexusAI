import { AlertCircle, CircleSlash, FileQuestion, Layers } from 'lucide-react'
import { STATES } from '../../lib/workbenchProof.js'

const ICONS = { empty: CircleSlash, broad: Layers, abstained: FileQuestion, partial: AlertCircle }

// The designed states of the workbench (the contract's list): each says what happened, why, and what to do next. They replace the canvas, never a toast.
export function WbState({ id }) {
  const state = STATES[id]
  if (!state) return null
  const Icon = ICONS[id]
  return (
    <section className={`wbp-state wbp-state--${id}`} role="status" aria-live="polite">
      <Icon size={24} strokeWidth={1.5} aria-hidden="true" />
      <h2>{state.title}</h2>
      <p>{state.body}</p>
      <ul>{state.actions.map(a => <li key={a}><button type="button" className="wbp-btn">{a}</button></li>)}</ul>
    </section>
  )
}
