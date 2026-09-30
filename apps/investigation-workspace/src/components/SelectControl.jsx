import { useId, useState } from 'react'

export function SelectControl({ label, value, options, onChange, optionLabel = item => item }) {
  const labelId = useId()
  const [open, setOpen] = useState(false)
  const all = ['all', ...options]
  return (
    <div className="select-control">
      <span id={labelId}>{label}</span>
      <button type="button" className="select-control__trigger" aria-haspopup="listbox" aria-expanded={open} aria-labelledby={`${labelId} ${labelId}-value`} onClick={() => setOpen(value => !value)}><span id={`${labelId}-value`}>{value === 'all' ? 'All' : optionLabel(value)}</span><span aria-hidden="true">⌄</span></button>
      {open ? <div className="select-control__menu" role="listbox" aria-labelledby={labelId}>{all.map(option => <button key={option} type="button" role="option" aria-selected={value === option} onClick={() => { onChange(option); setOpen(false) }}>{option === 'all' ? 'All' : optionLabel(option)}</button>)}</div> : null}
    </div>
  )
}
