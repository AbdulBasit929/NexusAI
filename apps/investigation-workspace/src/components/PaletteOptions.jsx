import { useMemo } from 'react'
import { Activity, Ban, Check, CheckCircle2, CircleAlert, Clock3, Folders, LayoutDashboard, Search, Settings2, X } from 'lucide-react'
import { measurePalette, paletteOptions } from '../lib/paletteOptions.js'
import '../styles/palette-options.css'

const railItems = [
  ['Dashboard', LayoutDashboard, true],
  ['Cases', Folders, false],
  ['Activity', Activity, false],
  ['Settings', Settings2, false],
]

const statusRows = [
  ['ready', 'Ready', CheckCircle2],
  ['processing', 'Processing', Clock3],
  ['failed', 'Failed', CircleAlert],
  ['withheld', 'Withheld', Ban],
]

// Illustrative bar lengths only; the labels are generic and no value is presented as case data.
const barLengths = [92, 74, 58, 44, 30, 18]

function gradient(stops, direction) {
  return stops.length > 1 ? `linear-gradient(${direction}, ${stops[0]}, ${stops[1]})` : stops[0]
}

function variables(mode) {
  const vars = {
    '--pv-header': gradient(mode.header.stops, '112deg'),
    '--pv-header-text': mode.header.text,
    '--pv-header-muted': mode.header.muted,
    '--pv-header-line': mode.header.line,
    '--pv-rail': gradient(mode.rail.stops, '180deg'),
    '--pv-rail-text': mode.rail.text,
    '--pv-rail-muted': mode.rail.muted,
    '--pv-rail-section': mode.rail.section,
    '--pv-rail-active': mode.rail.activeBg,
    '--pv-rail-active-text': mode.rail.activeText,
    '--pv-rail-marker': mode.rail.marker,
    '--pv-canvas': mode.canvas,
    '--pv-card': mode.card,
    '--pv-raised': mode.raised,
    '--pv-border': mode.border,
    '--pv-border-strong': mode.borderStrong,
    '--pv-text': mode.text,
    '--pv-muted': mode.muted,
    '--pv-accent': mode.accent,
    '--pv-accent-on': mode.accentOn,
    '--pv-focus': mode.focus,
  }
  mode.data.forEach((colour, index) => { vars[`--pv-data-${index + 1}`] = colour })
  Object.entries(mode.status).forEach(([name, colour]) => { vars[`--pv-status-${name}`] = colour })
  return vars
}

function ShellPreview({ option, modeName, mode }) {
  return (
    <figure className="palette-preview" style={variables(mode)} data-palette-option={option.id} data-palette-mode={modeName}>
      <figcaption>Option {option.id} · {modeName === 'light' ? 'Light theme' : 'Dark theme'}</figcaption>
      <div className="palette-preview__shell" role="img" aria-label={`Option ${option.id} in the ${modeName} theme: gradient header, sidebar with Dashboard active, a card with text, a button, status badges and six chart series.`}>
        <div className="palette-preview__header"><span className="palette-preview__mark" /><strong>NexusAI</strong><span className="palette-preview__search"><Search />Find a case, source or question</span></div>
        <div className="palette-preview__frame">
          <div className="palette-preview__rail">
            <span className="palette-preview__section">Workspace</span>
            {railItems.map(([label, Icon, active]) => <span key={label} className={`palette-preview__link${active ? ' is-active' : ''}`}><Icon />{label}</span>)}
          </div>
          <div className="palette-preview__canvas">
            <div className="palette-preview__card">
              <strong>Card heading</strong>
              <span className="palette-preview__muted">Secondary text sits on the card surface.</span>
              <span className="palette-preview__actions">
                <span className="palette-preview__button">Primary action</span>
                <span className="palette-preview__button palette-preview__button--focus">Focus ring</span>
                <span className="palette-preview__link-text">Text link</span>
              </span>
              <span className="palette-preview__badges">
                {statusRows.map(([id, label, Icon]) => <span key={id} className={`palette-preview__badge palette-preview__badge--${id}`}><Icon />{label}</span>)}
              </span>
            </div>
            <div className="palette-preview__card">
              <strong>Category comparison</strong>
              <span className="palette-preview__bars">
                {barLengths.map((length, index) => <span key={length} className="palette-preview__bar"><small>Series {index + 1}</small><span><i style={{ inlineSize: `${length}%`, background: `var(--pv-data-${index + 1})` }} /></span></span>)}
              </span>
              <span className="palette-preview__muted">Demonstration marks, not case data.</span>
            </div>
          </div>
        </div>
      </div>
    </figure>
  )
}

function ContrastTable({ option, modeName, mode }) {
  const rows = useMemo(() => measurePalette(mode), [mode])
  const failing = rows.filter(row => !row.pass).length
  return (
    <details className="palette-contrast">
      <summary>{modeName === 'light' ? 'Light' : 'Dark'} theme contrast <span>{failing ? `${failing} below target` : `${rows.length} of ${rows.length} pass`}</span></summary>
      <table aria-label={`Option ${option.id} ${modeName} theme measured contrast`}>
        <thead><tr><th scope="col">Pair</th><th scope="col">Measured</th><th scope="col">Needs</th><th scope="col">Result</th></tr></thead>
        <tbody>
          {rows.map(row => <tr key={row.label}><th scope="row">{row.label}</th><td>{row.ratio.toFixed(2)}:1</td><td>{row.required}:1</td><td><span className={row.pass ? 'is-pass' : 'is-fail'}>{row.pass ? <Check /> : <X />}{row.pass ? 'Pass' : 'Fail'}</span></td></tr>)}
        </tbody>
      </table>
    </details>
  )
}

export function PaletteOptions() {
  return (
    <section className="gallery-section palette-options" aria-labelledby="palette-options-title">
      <div className="gallery-section__heading">
        <div><span className="eyebrow">Identity decision</span><h2 id="palette-options-title">Shell palette options, derived from the header</h2></div>
        <p>All three start from the header gradient (navy to deep teal). Each is shown in both themes with contrast measured on the real backgrounds, gradient midpoints included. Pick one and it becomes the shell for every page.</p>
      </div>
      {paletteOptions.map(option => (
        <article key={option.id} className="palette-option" aria-labelledby={`palette-option-${option.id}`}>
          <header><span className="palette-option__id" aria-hidden="true">{option.id}</span><div><h3 id={`palette-option-${option.id}`}>Option {option.id}: {option.name}</h3><p>{option.summary}</p></div></header>
          <div className="palette-option__previews">
            {Object.entries(option.modes).map(([modeName, mode]) => <ShellPreview key={modeName} option={option} modeName={modeName} mode={mode} />)}
          </div>
          <div className="palette-option__contrast">
            {Object.entries(option.modes).map(([modeName, mode]) => <ContrastTable key={modeName} option={option} modeName={modeName} mode={mode} />)}
          </div>
        </article>
      ))}
    </section>
  )
}
