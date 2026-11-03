import { useState } from 'react'
import { AlertTriangle, Check, FileSearch, Info, Search } from 'lucide-react'
import { contrast, contrastChecks, themes } from '../../../design/tokens-v2.mjs'
import { AppShell } from '../shell/AppShell.jsx'
import { DataTable } from '../ui/DataTable.jsx'

// The design language on one page, drawn from the real tokens and the real component classes (not a picture of them): colour in three themes with
// the contrast of every pair that is used together, type, space, components in their states, and motion. The owner looks at this before a page is built on it.
const ROLES = [
  ['bg', 'Canvas'], ['surface', 'Surface'], ['sunken', 'Sunken'], ['border', 'Border'], ['borderStrong', 'Control border'],
  ['text', 'Text'], ['text2', 'Text 2'], ['text3', 'Text 3'], ['accent', 'Accent'], ['accentSubtle', 'Accent tint'],
  ['ok', 'Success'], ['warn', 'Warning'], ['danger', 'Error'], ['info', 'Information'],
]
const THEME_NAMES = { light: 'Light', dark: 'Dark', contrast: 'High contrast' }

function Swatches({ name }) {
  const theme = themes[name]
  return (
    <div className="nx-dl-theme" data-nx-theme={name}>
      <h3>{THEME_NAMES[name]}</h3>
      <ul className="nx-dl-swatches">
        {ROLES.map(([role, label]) => (
          <li key={role}>
            <span className="nx-dl-chip" style={{ background: theme[role] }} aria-hidden="true" />
            <span className="nx-dl-name">{label}</span>
            <span className="nx-dl-hex nx-mono">{theme[role]}</span>
          </li>
        ))}
      </ul>
      <ul className="nx-dl-data" aria-label={`${THEME_NAMES[name]} data colours`}>
        {theme.data.map((value, index) => <li key={value} style={{ background: value }} title={`Data ${index + 1} ${value}`} />)}
      </ul>
      <p className="nx-dl-ratio">Text on canvas {contrast(theme.text, theme.bg).toFixed(1)}:1 · muted text on surface {contrast(theme.text3, theme.surface).toFixed(1)}:1 · control border on surface {contrast(theme.borderStrong, theme.surface).toFixed(1)}:1</p>
    </div>
  )
}

const rows = [
  { id: 1, contact: '923000001038', calls: 93, talk: '6,183', last: '2026-04-17' },
  { id: 2, contact: '923000001001', calls: 71, talk: '4,902', last: '2026-04-12' },
  { id: 3, contact: '923000001021', calls: 69, talk: '5,118', last: '2026-04-09' },
]
const columns = [
  { key: 'contact', label: 'Contact' }, { key: 'calls', label: 'Calls' }, { key: 'talk', label: 'Talk time (s)' }, { key: 'last', label: 'Last contact' },
]

export default function DesignLanguagePage() {
  const [pressed, setPressed] = useState(true)
  const pairs = Object.keys(themes).length * contrastChecks.length
  return (
    <AppShell>
      <main id="workspace-main" className="nx-ui nx-page nx-dl" tabIndex={-1}>
        <header className="nx-page-head">
          <div className="nx-page-head__text">
            <h1>Design language</h1>
            <p>Quiet chrome, loud evidence, one accent. Colour is chosen in OKLCH and every pair that is used together is checked at build time ({pairs} pairs across three themes; the build fails below WCAG 4.5:1 for text and 3:1 for controls, focus and marks).</p>
          </div>
        </header>

        <section className="nx-region" aria-labelledby="dl-colour">
          <div className="nx-region__head"><h2 id="dl-colour">Colour</h2><p>Three themes, the same roles. Depth is tone first; shadow only on menus and dialogs.</p></div>
          <div className="nx-dl-themes">{Object.keys(themes).map(name => <Swatches key={name} name={name} />)}</div>
        </section>

        <section className="nx-region" aria-labelledby="dl-type">
          <div className="nx-region__head"><h2 id="dl-type">Type</h2><p>Noto Sans for the interface, Noto Sans Mono for identifiers, Noto Sans Arabic for Urdu. 13 px for dense work, 14 px body.</p></div>
          <div className="nx-card nx-dl-type">
            <p style={{ fontSize: 'var(--nx-fs-3xl)', fontWeight: 600, letterSpacing: '-0.02em' }}>Who did this number talk to most?</p>
            <p style={{ fontSize: 'var(--nx-fs-2xl)', fontWeight: 600 }}>Calls per day, 1 Feb to 15 Apr 2026</p>
            <p style={{ fontSize: 'var(--nx-fs-lg)' }}>The number 923000001038 is the contact with the most records: 93 calls and texts in two files, recomputed from the records.</p>
            <p>Body text, 14 px. Evidence is shown whole, with the file and the row it came from. <span className="nx-secondary">Secondary text for context.</span> <span className="nx-muted">Muted text for metadata.</span></p>
            <p className="nx-tabular nx-mono">923000001038 · 2026-04-17 16:58:18 PKT · 14,673 · 6,183 s</p>
            <p dir="rtl" style={{ fontFamily: 'var(--nx-font-ur)', fontSize: 'var(--nx-fs-lg)' }}>یہ نمبر سب سے زیادہ کس سے بات کرتا ہے؟ ثبوت مکمل اور قابلِ تصدیق ہے۔</p>
          </div>
        </section>

        <section className="nx-region" aria-labelledby="dl-components">
          <div className="nx-region__head"><h2 id="dl-components">Components</h2><p>Real classes, real states. Props say meaning (primary, danger), never colour.</p></div>
          <div className="nx-card nx-dl-components">
            <div className="nx-dl-row">
              <button type="button" className="nx-btn nx-btn--primary">Primary</button>
              <button type="button" className="nx-btn">Secondary</button>
              <button type="button" className="nx-btn nx-btn--ghost">Ghost</button>
              <button type="button" className="nx-btn nx-btn--danger">Danger</button>
              <button type="button" className="nx-btn" disabled>Disabled</button>
              <button type="button" className="nx-btn nx-btn--sm">Small</button>
              <button type="button" className="nx-btn nx-btn--primary nx-btn--lg">Large</button>
              <button type="button" className="nx-btn nx-btn--ghost nx-btn--icon" aria-label="Search"><Search aria-hidden="true" /></button>
            </div>
            <div className="nx-dl-row">
              <span className="nx-badge">Neutral</span><span className="nx-badge nx-badge--accent">Selected</span><span className="nx-badge nx-badge--ok">Ready</span>
              <span className="nx-badge nx-badge--warn">Processing</span><span className="nx-badge nx-badge--danger">Needs review</span><span className="nx-badge nx-badge--info">Information</span>
              <span className="nx-strength" data-strength="observed"><svg viewBox="0 0 12 12" aria-hidden="true"><circle cx="6" cy="6" r="4.5" fill="currentColor" /></svg>Observed</span>
              <span className="nx-strength" data-strength="inferred"><svg viewBox="0 0 12 12" aria-hidden="true"><circle cx="6" cy="6" r="4.5" fill="none" stroke="currentColor" strokeWidth="1.5" /><path d="M6 1.5a4.5 4.5 0 0 1 0 9z" fill="currentColor" /></svg>Inferred</span>
              <span className="nx-strength" data-strength="candidate"><svg viewBox="0 0 12 12" aria-hidden="true"><circle cx="6" cy="6" r="4.5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeDasharray="2.2 1.8" /></svg>Candidate</span>
              <kbd className="nx-kbd">Ctrl</kbd><kbd className="nx-kbd">K</kbd>
            </div>
            <div className="nx-dl-row">
              <button type="button" className="nx-chip" aria-pressed={pressed} onClick={() => setPressed(value => !value)}>Calls</button>
              <button type="button" className="nx-chip" aria-pressed="false">Texts</button>
              <span className="nx-segmented" role="group" aria-label="Density"><button type="button" aria-pressed="false">Compact</button><button type="button" aria-pressed="true">Default</button><button type="button" aria-pressed="false">Comfortable</button></span>
              <input className="nx-input" style={{ inlineSize: 220 }} placeholder="A text field" aria-label="A text field" />
            </div>
            <div className="nx-dl-grid">
              <div className="nx-callout nx-callout--info"><Info aria-hidden="true" /><p><strong>Information.</strong> Calm, one sentence, one next step.</p></div>
              <div className="nx-callout nx-callout--warn"><AlertTriangle aria-hidden="true" /><p><strong>Partial answer.</strong> Two files are still processing.</p></div>
              <div className="nx-callout nx-callout--ok"><Check aria-hidden="true" /><p><strong>Recomputed from the records:</strong> matches.</p></div>
              <div className="nx-empty nx-card"><span className="nx-empty__icon"><FileSearch aria-hidden="true" /></span><h3>No records matched</h3><p>The scope searched is named, so you can widen it.</p></div>
            </div>
            <DataTable columns={columns} rows={rows} label="Sample table" caption="A sample table" />
          </div>
        </section>

        <section className="nx-region" aria-labelledby="dl-tokens">
          <div className="nx-region__head"><h2 id="dl-tokens">Space, shape, motion</h2><p>A 4 px grid; radius 4, 6, 8, 12; motion 70 to 700 ms, none where the work repeats.</p></div>
          <div className="nx-card nx-dl-row">
            {[4, 8, 12, 16, 24, 32, 48].map(value => <span key={value} className="nx-dl-space"><i style={{ inlineSize: value, blockSize: value }} />{value}</span>)}
            {['--nx-r-1', '--nx-r-2', '--nx-r-3', '--nx-r-4'].map(token => <span key={token} className="nx-dl-radius" style={{ borderRadius: `var(${token})` }}>{token.replace('--nx-r-', 'r')}</span>)}
          </div>
        </section>
      </main>
    </AppShell>
  )
}
