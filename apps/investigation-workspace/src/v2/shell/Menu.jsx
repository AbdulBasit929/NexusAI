import { useEffect, useId, useRef, useState } from 'react'

// A button that opens a small menu. Outside click and Escape close it (Escape returns focus to the button), arrow keys move
// between items, and the menu is a labelled `menu` with `menuitem` / `menuitemradio` children (the caller supplies them with
// the `nx-menu__item` class). `children` is a function that receives `close`.
export function Menu({ label, icon, trigger, align = 'end', up = false, buttonClassName = 'nx-btn nx-btn--ghost nx-btn--icon', children }) {
  const [open, setOpen] = useState(false)
  const wrap = useRef(null)
  const button = useRef(null)
  const id = useId()

  useEffect(() => {
    if (!open) return undefined
    const first = wrap.current?.querySelector('.nx-menu__item')
    first?.focus()
    function outside(event) { if (wrap.current && !wrap.current.contains(event.target)) setOpen(false) }
    function keys(event) {
      if (event.key === 'Escape') { event.preventDefault(); setOpen(false); button.current?.focus(); return }
      if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
      const items = [...(wrap.current?.querySelectorAll('.nx-menu__item') || [])]
      if (!items.length) return
      event.preventDefault()
      const index = items.indexOf(globalThis.document.activeElement)
      const next = event.key === 'ArrowDown' ? (index + 1) % items.length : (index - 1 + items.length) % items.length
      items[next].focus()
    }
    globalThis.document.addEventListener('mousedown', outside)
    globalThis.document.addEventListener('keydown', keys)
    return () => {
      globalThis.document.removeEventListener('mousedown', outside)
      globalThis.document.removeEventListener('keydown', keys)
    }
  }, [open])

  const close = () => { setOpen(false); button.current?.focus() }
  return (
    <div className="nx-menu-wrap" ref={wrap}>
      <button ref={button} type="button" className={buttonClassName} aria-label={label} title={label} aria-haspopup="menu" aria-expanded={open} aria-controls={open ? id : undefined} onClick={() => setOpen(value => !value)}>
        {trigger || icon}
      </button>
      {open ? <div id={id} role="menu" aria-label={label} className={`nx-menu${align === 'start' ? ' nx-menu--start' : ''}${up ? ' nx-menu--up' : ''}`}>{children(close)}</div> : null}
    </div>
  )
}
