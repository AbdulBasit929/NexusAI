import { useEffect, useId, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { Check, Monitor, Moon, Sun } from 'lucide-react'
import { useThemePreference } from './ThemeControl.jsx'

const options = [
  { value: 'system', label: 'System', icon: Monitor },
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
]

// Header appearance control. A compact icon button that shows the theme in use opens a small panel with one
// native radio group (arrow keys, one tab stop, no custom key handling to get wrong). Follows the APG menu
// button conventions: aria-expanded on the button, Escape and outside click close, focus returns to the button.
export function AppearanceMenu() {
  const { preference, resolved, choose } = useThemePreference()
  const [open, setOpen] = useState(false)
  const root = useRef(null)
  const button = useRef(null)
  const panelId = useId()
  const groupName = useId()
  const Icon = resolved === 'dark' ? Moon : Sun

  useEffect(() => {
    if (!open) return undefined
    const checked = root.current?.querySelector('input:checked')
    checked?.focus()
    function keydown(event) {
      if (event.key === 'Escape') { event.preventDefault(); setOpen(false); button.current?.focus() }
    }
    function pointer(event) {
      if (root.current && !root.current.contains(event.target)) setOpen(false)
    }
    globalThis.document.addEventListener('keydown', keydown)
    globalThis.document.addEventListener('mousedown', pointer)
    return () => {
      globalThis.document.removeEventListener('keydown', keydown)
      globalThis.document.removeEventListener('mousedown', pointer)
    }
  }, [open])

  return (
    <div className="appearance-menu" ref={root}>
      <button ref={button} type="button" className="appearance-menu__button" aria-haspopup="true" aria-expanded={open} aria-controls={open ? panelId : undefined} aria-label={`Appearance, ${preference === 'system' ? `system, currently ${resolved}` : preference}`} title="Appearance" onClick={() => setOpen(value => !value)}>
        <Icon aria-hidden="true" />
      </button>
      {open ? (
        <div id={panelId} className="appearance-menu__panel" role="group" aria-label="Appearance">
          <fieldset>
            <legend>Theme</legend>
            <div className="appearance-menu__options">
              {options.map(({ value, label, icon: OptionIcon }) => (
                <label key={value} className={`appearance-menu__option${preference === value ? ' is-selected' : ''}`}>
                  <input type="radio" name={`appearance-${groupName}`} value={value} checked={preference === value} onChange={() => choose(value)} />
                  <span className={`appearance-menu__swatch appearance-menu__swatch--${value}`} aria-hidden="true"><OptionIcon /></span>
                  <span className="appearance-menu__label">{label}{preference === value ? <Check aria-hidden="true" /> : null}</span>
                </label>
              ))}
            </div>
          </fieldset>
          <p className="appearance-menu__note" role="status">{preference === 'system' ? `Following this device. Currently ${resolved}.` : `Always ${preference} on this browser.`}</p>
          <Link className="appearance-menu__link" to="/settings" onClick={() => setOpen(false)}>All display settings</Link>
        </div>
      ) : null}
    </div>
  )
}
