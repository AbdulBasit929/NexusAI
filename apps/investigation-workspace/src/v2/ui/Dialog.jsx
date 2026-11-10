import { useEffect, useId, useRef } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'

// A modal dialog over the page: focus moves in, Tab stays inside, Escape and a click outside close it, and the caller returns focus to the control that opened it (`onClose`).
// Used for a chart's table and for the comparison of two runs of a question. Nothing behind it changes size.
const FOCUSABLE = 'button, a[href], input, select, textarea, summary, [tabindex]:not([tabindex="-1"])'

export function Dialog({ title, onClose, closeLabel = 'Close', wide = false, children }) {
  const titleId = useId()
  const box = useRef(null)
  useEffect(() => { box.current?.focus() }, [])
  const keys = event => {
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); onClose(); return }
    if (event.key !== 'Tab') return
    const items = [...box.current.querySelectorAll(FOCUSABLE)].filter(item => !item.disabled)
    if (!items.length) { event.preventDefault(); return }
    const first = items[0]
    const last = items.at(-1)
    const inside = document.activeElement
    if (event.shiftKey && (inside === first || inside === box.current)) { event.preventDefault(); last.focus() }
    else if (!event.shiftKey && inside === last) { event.preventDefault(); first.focus() }
  }
  return createPortal(
    <div className="nx-scope nx-ui nx-overlay nx-overlay--center" onMouseDown={event => event.target === event.currentTarget && onClose()}>
      <div ref={box} className={`nx-dialog nx-twin-dialog${wide ? ' nx-twin-dialog--wide' : ''}`} role="dialog" aria-modal="true" aria-labelledby={titleId} tabIndex={-1} onKeyDown={keys}>
        <header className="nx-twin-dialog__head">
          <h2 id={titleId}>{title}</h2>
          <button type="button" className="nx-twin__open" aria-label={closeLabel} onClick={onClose}><X aria-hidden="true" /></button>
        </header>
        <div className="nx-twin-dialog__body">{children}</div>
      </div>
    </div>,
    document.body,
  )
}
