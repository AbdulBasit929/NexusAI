import { useEffect, useId, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { Check, Contrast, Monitor, Moon, Rows3, Sun } from 'lucide-react'
import { useDisplayPreferences, useThemePreference } from './ThemeControl.jsx'

const themes = [
  { value: 'system', label: 'System', icon: Monitor },
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
  { value: 'high-contrast', label: 'High contrast', icon: Contrast },
]

function Choice({ checked, name, value, onChange, children }) {
  return <label className={checked ? 'is-selected' : undefined}><input type="radio" name={name} value={value} checked={checked} onChange={onChange} /><span>{children}</span>{checked ? <Check aria-hidden="true" /> : null}</label>
}

// Appearance is a disclosure containing form controls, not an ARIA menu. Native radio groups keep their
// keyboard behavior; Escape and outside click dismiss the panel and return focus to its trigger.
export function AppearanceMenu() {
  const { preference, resolved, choose } = useThemePreference()
  const display = useDisplayPreferences()
  const [open, setOpen] = useState(false)
  const root = useRef(null)
  const button = useRef(null)
  const panelId = useId()
  const themeName = useId()
  const densityName = useId()
  const detailName = useId()
  const Icon = preference === 'high-contrast' ? Contrast : resolved === 'dark' ? Moon : Sun

  useEffect(() => {
    if (!open) return undefined
    root.current?.querySelector('input:checked')?.focus()
    function keydown(event) {
      if (event.key === 'Escape') { event.preventDefault(); setOpen(false); button.current?.focus() }
    }
    function pointer(event) { if (root.current && !root.current.contains(event.target)) setOpen(false) }
    globalThis.document.addEventListener('keydown', keydown)
    globalThis.document.addEventListener('mousedown', pointer)
    return () => {
      globalThis.document.removeEventListener('keydown', keydown)
      globalThis.document.removeEventListener('mousedown', pointer)
    }
  }, [open])

  return (
    <div className="appearance-menu" ref={root}>
      <button ref={button} type="button" className="appearance-menu__button" aria-haspopup="dialog" aria-expanded={open} aria-controls={open ? panelId : undefined} aria-label={`Appearance, ${preference === 'system' ? `system, currently ${resolved}` : preference}`} title="Appearance" onClick={() => setOpen(value => !value)}><Icon aria-hidden="true" /></button>
      {open ? (
        <section id={panelId} className="appearance-menu__panel" aria-label="Appearance preferences">
          <header><span><strong>Appearance</strong><small>Saved in this browser</small></span></header>
          <fieldset><legend>Theme</legend><div className="appearance-menu__themes">
            {themes.map(({ value, label, icon: OptionIcon }) => <Choice key={value} checked={preference === value} name={`appearance-${themeName}`} value={value} onChange={() => choose(value)}><OptionIcon aria-hidden="true" />{label}</Choice>)}
          </div></fieldset>
          <fieldset><legend>Density</legend><div className="appearance-menu__choices">
            {[['compact','Compact'],['default','Default'],['comfortable','Comfortable']].map(([value,label]) => <Choice key={value} checked={display.density === value} name={`density-${densityName}`} value={value} onChange={() => display.choose('density', value)}><Rows3 aria-hidden="true" />{label}</Choice>)}
          </div></fieldset>
          <fieldset><legend>Answer detail</legend><div className="appearance-menu__choices">
            {[['concise','Concise'],['standard','Standard'],['detailed','Detailed']].map(([value,label]) => <Choice key={value} checked={display.detail === value} name={`detail-${detailName}`} value={value} onChange={() => display.choose('detail', value)}>{label}</Choice>)}
          </div></fieldset>
          <label className="appearance-menu__motion"><input type="checkbox" checked={display.motion === 'reduce'} onChange={event => display.choose('motion', event.target.checked ? 'reduce' : 'system')} /><span><strong>Reduce motion</strong><small>Overrides motion in the NexusAI shell.</small></span></label>
          <p className="appearance-menu__note" role="status">{preference === 'system' ? `Following this device. Currently ${resolved}.` : `${themes.find(item => item.value === preference)?.label} is active.`}</p>
          <Link className="appearance-menu__link" to="/settings" onClick={() => setOpen(false)}>All display settings</Link>
        </section>
      ) : null}
    </div>
  )
}
