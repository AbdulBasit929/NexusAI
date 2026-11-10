import { useRef, useState } from 'react'
import { Table2 } from 'lucide-react'
import { DataTable } from './DataTable.jsx'
import { Dialog } from './Dialog.jsx'

// Every chart has its twin (directive 7.6): the same marks as a table under the same names, sortable, filterable, with Copy as CSV, and a sentence that says what is drawn.
// The sentence is the chart's description for a screen reader; the table is the keyboard route to every value. The table opens in a dialog from a button in the panel's header, so a panel never
// changes size when its table is read. The dialog moves focus in, keeps Tab inside, closes on Escape or on a click outside, and returns focus to the button that opened it.
export function TableTwin({ label, columns, rows, exportName, summary }) {
  const [open, setOpen] = useState(false)
  const trigger = useRef(null)
  if (!rows.length) return null
  return (
    <>
      {summary ? <p className="nx-visually-hidden">{summary}</p> : null}
      <button ref={trigger} type="button" className="nx-twin__open" aria-haspopup="dialog" aria-label={`Show ${label.toLowerCase()} as a table`} title="Show as table" onClick={() => setOpen(true)}><Table2 aria-hidden="true" /></button>
      {open ? (
        <Dialog title={label} closeLabel="Close the table" onClose={() => { setOpen(false); trigger.current?.focus() }}>
          <DataTable columns={columns} rows={rows} label={label} caption={label} initialLimit={25} exportName={exportName} />
        </Dialog>
      ) : null}
    </>
  )
}
