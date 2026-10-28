import { useEffect, useId, useState } from 'react'
import { Link } from 'react-router-dom'
import { Check, Contrast, Download, Monitor, Moon, Rows3, Sun } from 'lucide-react'
import { useDisplayPreferences, useThemePreference } from '../../components/ThemeControl.jsx'
import { formatNumber } from '../../lib/format.js'
import { shortcutSections } from '../../lib/workspaceShortcuts.js'

const THEMES = [
  { value: 'system', label: 'Use device setting', detail: 'Follows this device as its appearance changes.', icon: Monitor },
  { value: 'light', label: 'Light', detail: 'A cool, high-contrast working surface.', icon: Sun },
  { value: 'dark', label: 'Dark', detail: 'A low-glare slate working surface.', icon: Moon },
  { value: 'high-contrast', label: 'High contrast', detail: 'Maximum separation for text, controls and working surfaces.', icon: Contrast },
]

// A small picture of the workspace in each theme, drawn with fixed colours so each option previews itself whatever theme
// is active now. "Use device setting" shows both halves.
function Preview({ mode }) {
  const light = { bg: '#eef3f4', panel: '#fbfdfd', line: '#c3d3d7', ink: '#111827', rail: '#0f2f3a', accent: '#0f7f95' }
  const dark = { bg: '#0b161b', panel: '#12212a', line: '#27414c', ink: '#e6eef0', rail: '#0a1f27', accent: '#4dd3e8' }
  const contrast = { bg: '#000000', panel: '#000000', line: '#ffffff', ink: '#ffffff', rail: '#000000', accent: '#ffff00' }
  const half = (c, key) => (
    <g key={key}>
      <rect width="120" height="76" fill={c.bg} />
      <rect width="22" height="76" fill={c.rail} />
      <rect x="28" y="8" width="52" height="6" rx="3" fill={c.ink} opacity=".85" />
      <rect x="28" y="20" width="86" height="22" rx="4" fill={c.panel} stroke={c.line} />
      <polyline points="32,38 44,30 54,35 66,26 80,33 96,24 108,31" fill="none" stroke={c.accent} strokeWidth="1.6" />
      <rect x="28" y="48" width="40" height="22" rx="4" fill={c.panel} stroke={c.line} />
      <rect x="74" y="48" width="40" height="22" rx="4" fill={c.panel} stroke={c.line} />
      <rect x="78" y="54" width="18" height="4" rx="2" fill={c.accent} />
    </g>
  )
  return (
    <svg viewBox="0 0 120 76" aria-hidden="true" className="st-preview">
      {mode === 'system' ? (
        <>
          <defs><clipPath id="st-left"><rect width="60" height="76" /></clipPath><clipPath id="st-right"><rect x="60" width="60" height="76" /></clipPath></defs>
          <g clipPath="url(#st-left)">{half(light, 'l')}</g><g clipPath="url(#st-right)">{half(dark, 'd')}</g>
        </>
      ) : half(mode === 'dark' ? dark : mode === 'high-contrast' ? contrast : light, mode)}
    </svg>
  )
}

// Three pictured choices as one radio group. Keyboard: arrow keys move the choice, as any radio group does.
export function AppearancePicker() {
  const group = useId()
  const { preference, resolved, choose } = useThemePreference()
  const display = useDisplayPreferences()
  return (
    <div className="st-appearance">
      <fieldset className="st-themes">
        <legend>Color theme</legend>
        <div>
        {THEMES.map(option => {
          const Icon = option.icon
          return (
            <label key={option.value} className="st-theme">
              <input type="radio" name={`theme-${group}`} value={option.value} checked={preference === option.value} onChange={() => choose(option.value)} />
              <Preview mode={option.value} />
              <span className="st-theme__copy"><strong><Icon aria-hidden="true" />{option.label}</strong><small>{option.value === 'system' ? `${option.detail} Currently ${resolved}.` : option.detail}</small></span>
              <Check className="st-theme__check" aria-hidden="true" />
            </label>
          )
        })}
        </div>
      </fieldset>
      <fieldset className="st-display-options">
        <legend>Display preferences</legend>
        <label><span><Rows3 aria-hidden="true" /><strong>Density</strong><small>Changes row and control spacing.</small></span><select value={display.density} onChange={event => display.choose('density', event.target.value)}><option value="compact">Compact</option><option value="default">Default</option><option value="comfortable">Comfortable</option></select></label>
        <label><span><strong>Answer detail</strong><small>Sets the preferred answer presentation depth.</small></span><select value={display.detail} onChange={event => display.choose('detail', event.target.value)}><option value="concise">Concise</option><option value="standard">Standard</option><option value="detailed">Detailed</option></select></label>
        <label><span><strong>Reduce motion</strong><small>Minimizes shell animation and transitions.</small></span><input type="checkbox" checked={display.motion === 'reduce'} onChange={event => display.choose('motion', event.target.checked ? 'reduce' : 'system')} /></label>
      </fieldset>
    </div>
  )
}

export function formatSize(bytes) {
  if (bytes < 1024) return `${bytes} B`
  return `${(bytes / 1024).toFixed(bytes < 10240 ? 1 : 0)} KB`
}

// What this browser keeps, kind by kind: how many items, how big, and a clear for each. Questions and drafts go through the
// reviewed confirmation above the table; the others confirm in place. Nothing here can touch case evidence.
export function InventoryTable({ inventory, onClearQuestions, onClear }) {
  const [confirming, setConfirming] = useState('')
  useEffect(() => { setConfirming('') }, [inventory])
  return (
    <div className="st-table" role="region" aria-label="What this browser remembers" tabIndex={0}>
      <table>
        <thead><tr><th scope="col">Kind</th><th scope="col" className="is-num">Items</th><th scope="col" className="is-num">Size</th><th scope="col"><span className="visually-hidden">Remove</span></th></tr></thead>
        <tbody>
          {inventory.rows.map(row => (
            <tr key={row.id}>
              <th scope="row"><b>{row.label}</b><small>{row.note}</small></th>
              <td className="is-num">{formatNumber(row.items)}</td>
              <td className="is-num">{row.items || row.keys.length ? formatSize(row.bytes) : '—'}</td>
              <td className="is-act">
                {!row.keys.length ? <span className="st-none">Nothing stored</span>
                  : ['questions', 'drafts'].includes(row.id) ? <button type="button" className="st-link" onClick={onClearQuestions}>Review and clear…</button>
                    : confirming === row.id ? <span className="st-inline"><button type="button" className="st-danger" onClick={() => onClear(row)}>Confirm clear</button><button type="button" className="st-link" onClick={() => setConfirming('')}>Cancel</button></span>
                      : <button type="button" className="st-link" onClick={() => setConfirming(row.id)} aria-label={`Clear ${row.label.toLowerCase()}`}>Clear</button>}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function ExportButton({ onExport }) {
  return <button type="button" className="st-btn" onClick={onExport}><Download aria-hidden="true" />Download a copy (JSON)</button>
}

export function ShortcutsCard() {
  return (
    <section className="st-card" aria-labelledby="st-shortcuts-title">
      <header><h2 id="st-shortcuts-title">Keyboard shortcuts</h2><small>Where each one works</small></header>
      <div className="st-keys">
        {shortcutSections.map(block => (
          <div key={block.group}>
            <h3>{block.group}</h3>
            <dl>{block.rows.map(({ keys, what, together, sequence }) => (
              <div key={keys.join('')}><dt>{keys.map((key, index) => <span key={key}>{index ? <i aria-hidden="true">{sequence ? 'then' : together ? '+' : '/'}</i> : null}<kbd>{key}</kbd></span>)}</dt><dd>{what}</dd></div>
            ))}</dl>
          </div>
        ))}
      </div>
    </section>
  )
}

export function BoundaryCard({ caseCount }) {
  return (
    <section className="st-card" aria-labelledby="settings-boundary-title">
      <header><h2 id="settings-boundary-title">What these settings affect</h2><small>Storage boundary</small></header>
      <dl className="st-boundary">
        <div><dt>Kept here</dt><dd>Theme, navigation preference, saved questions, unfinished drafts, pinned timeline days and reviewed activity.</dd></div>
        <div><dt>Not changed</dt><dd>Case evidence, processed records, findings and source citations.</dd></div>
        <div><dt>Not shared</dt><dd>Other browsers and other analysts do not receive this local working data.</dd></div>
        <div><dt>This workspace</dt><dd>{formatNumber(caseCount)} {caseCount === 1 ? 'case' : 'cases'} configured. <Link to="/cases">Open the case directory</Link></dd></div>
      </dl>
    </section>
  )
}
