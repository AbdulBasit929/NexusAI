import { useEffect, useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Table2, X } from 'lucide-react'
import { DataTable } from './DataTable.jsx'

// Every chart has its twin (directive 7.6): the same marks as a table under the same names, sortable, filterable, with Copy as CSV, and a sentence that says what is drawn.
// The sentence is the chart's description for a screen reader; the table is the keyboard route to every value. The table opens in a dialog from a button in the panel's header, so a panel never
// changes size when its table is read. The dialog moves focus in, keeps Tab inside, closes on Escape or on a click outside, and returns focus to the button that opened it.
const FOCUSABLE = 'button, a[href], input, select, textarea, summary, [tabindex]:not([tabindex="-1"])'

function TableDialog({ label, columns, rows, exportName, onClose }) {
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
      <div ref={box} className="nx-dialog nx-twin-dialog" role="dialog" aria-modal="true" aria-labelledby={titleId} tabIndex={-1} onKeyDown={keys}>
        <header className="nx-twin-dialog__head">
          <h2 id={titleId}>{label}</h2>
          <button type="button" className="nx-twin__open" aria-label="Close the table" onClick={onClose}><X aria-hidden="true" /></button>
        </header>
        <div className="nx-twin-dialog__body"><DataTable columns={columns} rows={rows} label={label} caption={label} initialLimit={25} exportName={exportName} /></div>
      </div>
    </div>,
    document.body,
  )
}

export function TableTwin({ label, columns, rows, exportName, summary }) {
  const [open, setOpen] = useState(false)
  const trigger = useRef(null)
  if (!rows.length) return null
  return (
    <>
      {summary ? <p className="nx-visually-hidden">{summary}</p> : null}
      <button ref={trigger} type="button" className="nx-twin__open" aria-haspopup="dialog" aria-label={`Show ${label.toLowerCase()} as a table`} title="Show as table" onClick={() => setOpen(true)}><Table2 aria-hidden="true" /></button>
      {open ? <TableDialog label={label} columns={columns} rows={rows} exportName={exportName} onClose={() => { setOpen(false); trigger.current?.focus() }} /> : null}
    </>
  )
}
