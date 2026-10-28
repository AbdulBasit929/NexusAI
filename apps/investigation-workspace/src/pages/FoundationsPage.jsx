import { useMemo, useState } from 'react'
import {
  Activity,
  ArrowRight,
  Check,
  CheckCircle2,
  Circle,
  CircleDashed,
  CircleDot,
  Clock3,
  Database,
  FileStack,
  FolderKanban,
  Gauge,
  Info,
  LayoutDashboard,
  Link2,
  MapPin,
  Moon,
  PanelTop,
  Search,
  Settings2,
  ShieldCheck,
  Sun,
  TableProperties,
  TriangleAlert,
} from 'lucide-react'
import tokenReport from '../../design/tokens.generated.json'
import '../styles/pages/foundations.css'

const directions = {
  a: { name: 'Teal instrument', note: 'Continuity, with quiet teal-tinted neutrals.' },
  b: { name: 'Ink and signal', note: 'Neutral record surfaces with one blue-teal signal.' },
}
const navItems = [
  [LayoutDashboard, 'Dashboard', true],
  [FolderKanban, 'Cases', false],
  [Activity, 'Activity', false],
  [Settings2, 'Settings', false],
]

const statusItems = [
  [CheckCircle2, 'Ready', 'ready'],
  [Clock3, 'Processing', 'processing'],
  [TriangleAlert, 'Failed', 'failed'],
  [CircleDashed, 'Excluded', 'excluded'],
  [Info, 'Information', 'info'],
]

const strengthItems = [
  [CircleDot, 'Observed', 'observed'],
  [Circle, 'Inferred', 'inferred'],
  [CircleDashed, 'Candidate', 'candidate'],
]

function Segmented({ label, value, options, onChange }) {
  return (
    <fieldset className="nx-segmented">
      <legend>{label}</legend>
      <div>
        {options.map(option => (
          <button
            key={option.value}
            type="button"
            aria-pressed={value === option.value}
            onClick={() => onChange(option.value)}
          >
            {option.icon ? <option.icon aria-hidden="true" /> : null}
            {option.label}
          </button>
        ))}
      </div>
    </fieldset>
  )
}

function PreviewChrome({ title, children }) {
  return (
    <div className="nx-preview-shell">
      <header className="nx-preview-shell__header">
        <span className="nx-preview-mark" aria-hidden="true"><span /></span>
        <strong>NexusAI</strong>
        <span className="nx-preview-search"><Search aria-hidden="true" /> Find a case, source or question</span>
        <span className="nx-preview-case">Synthetic case</span>
      </header>
      <aside className="nx-preview-shell__rail" aria-label="Preview navigation">
        <span className="nx-preview-shell__label">Workspace</span>
        {navItems.map(([Icon, label, active]) => (
          <span className="nx-preview-nav" data-active={active || undefined} key={label}>
            <Icon aria-hidden="true" /> <span>{label}</span>
          </span>
        ))}
      </aside>
      <main className="nx-preview-shell__main">
        <div className="nx-preview-title"><span>Generated demonstration</span><h3>{title}</h3></div>
        {children}
      </main>
    </div>
  )
}

function DashboardPreview() {
  return (
    <PreviewChrome title="Dashboard">
      <section className="nx-preview-hero">
        <div><span className="nx-kicker">Tuesday, 28 October</span><h4>Continue the investigation</h4><p>Three generated cases need review. Nothing on this screen is live case data.</p></div>
        <button type="button">Open attention queue <ArrowRight aria-hidden="true" /></button>
      </section>
      <section className="nx-preview-metrics" aria-label="Synthetic metrics">
        {[
          ['Open cases', '12', 'Across this workspace'],
          ['Ready evidence', '42', 'Files available to inspect'],
          ['Needs review', '3', 'Processing or quality notes'],
        ].map(([label, value, note]) => <article key={label}><span>{label}</span><strong>{value}</strong><small>{note}</small></article>)}
      </section>
      <section className="nx-preview-region nx-preview-region--split">
        <div><span className="nx-kicker">Attention</span><h4>What needs a decision</h4><ul><li><TriangleAlert aria-hidden="true" /> One generated file failed processing</li><li><Clock3 aria-hidden="true" /> Two files are still processing</li></ul></div>
        <div><span className="nx-kicker">Continue</span><h4>Recent cases</h4><p><strong>Northbridge demonstration</strong><br />Last opened 18 minutes ago</p><p><strong>Harbour route demonstration</strong><br />Last opened yesterday</p></div>
      </section>
    </PreviewChrome>
  )
}

function MiniWorkbench() {
  return (
    <div className="nx-mini-workbench">
      <div className="nx-mini-workbench__tabs"><span data-active="true"><Link2 aria-hidden="true" /> Links</span><span><MapPin aria-hidden="true" /> Places</span><span><TableProperties aria-hidden="true" /> Records</span></div>
      <div className="nx-mini-workbench__body">
        <div className="nx-mini-graph" aria-label="Synthetic relationship diagram">
          <span className="nx-mini-edge nx-mini-edge--one" />
          <span className="nx-mini-edge nx-mini-edge--two" />
          <span className="nx-mini-node nx-mini-node--a">A</span>
          <span className="nx-mini-node nx-mini-node--b">B</span>
          <span className="nx-mini-node nx-mini-node--c">C</span>
        </div>
        <div className="nx-mini-records"><strong>Source records</strong><span>4 matching events</span><span>2 identifiers</span><span>1 location</span></div>
      </div>
      <div className="nx-mini-timeline"><span /><i /><i /><i /></div>
    </div>
  )
}

function AskPreview() {
  return (
    <PreviewChrome title="Ask">
      <section className="nx-preview-question"><span>Question</span><p>Which generated identifiers appear together most often?</p></section>
      <section className="nx-preview-answer">
        <span className="nx-kicker">Answer</span>
        <h4>Two identifiers appear together in four source records.<sup>1</sup></h4>
        <div className="nx-strength" data-strength="observed"><CircleDot aria-hidden="true" /> Observed in records</div>
      </section>
      <MiniWorkbench />
      <section className="nx-preview-citation"><ShieldCheck aria-hidden="true" /><div><strong>1 · Generated call records</strong><span>Rows 18–21 · synthetic locator</span></div><button type="button">Open rows</button></section>
    </PreviewChrome>
  )
}

function OverviewPreview() {
  return (
    <PreviewChrome title="Case overview">
      <section className="nx-preview-case-band"><div><span className="nx-kicker">Generated case</span><h4>Northbridge demonstration</h4><p>Reference SYN-2026-014</p></div><span className="nx-status" data-status="ready"><CheckCircle2 aria-hidden="true" /> Ready</span></section>
      <section className="nx-preview-metrics nx-preview-metrics--four" aria-label="Synthetic case summary">
        {[
          ['Evidence', '42'], ['Source rows', '184,620'], ['Processing', '2'], ['Quality notes', '3'],
        ].map(([label, value]) => <article key={label}><span>{label}</span><strong>{value}</strong></article>)}
      </section>
      <section className="nx-preview-region nx-preview-region--split">
        <div><span className="nx-kicker">Evidence families</span><h4>What is in this case</h4>{['Call records','Internet sessions','Subscriber records'].map((label, index) => <p className="nx-family-row" key={label}><span>{label}</span><i style={{ '--nx-demo-width': `${88 - index * 23}%` }} /></p>)}</div>
        <div><span className="nx-kicker">Data quality</span><h4>Notes to resolve</h4><p><TriangleAlert aria-hidden="true" /> One generated source is excluded from counts.</p><p><Info aria-hidden="true" /> Two identifiers need analyst confirmation.</p></div>
      </section>
    </PreviewChrome>
  )
}

const previewScreens = [
  ['Dashboard', DashboardPreview],
  ['Ask answer', AskPreview],
  ['Case overview', OverviewPreview],
]

function PaletteSection() {
  const ramp = Array.from({ length: 12 }, (_, index) => index + 1)
  return (
    <section className="nx-foundation-section" aria-labelledby="foundation-colour">
      <header><span>01 · Colour</span><h2 id="foundation-colour">Ramps and semantic roles</h2><p>Twelve authored jobs per ramp; resolved colours come from the generated token file.</p></header>
      <div className="nx-ramp"><strong>Neutral</strong><div>{ramp.map(step => <span key={step} style={{ background: `var(--nx-primitive-neutral-${step})` }}><i>{step}</i></span>)}</div></div>
      <div className="nx-ramp"><strong>Accent</strong><div>{ramp.map(step => <span key={step} style={{ background: `var(--nx-primitive-accent-${step})` }}><i>{step}</i></span>)}</div></div>
      <div className="nx-surface-grid">
        {['0','1','2','3','ask'].map(name => <article key={name} style={{ background: `var(--nx-surface-${name})` }}><strong>Surface {name}</strong><span>Primary text</span><small>Secondary and muted text remain readable.</small></article>)}
      </div>
      <div className="nx-role-row" aria-label="Status colours">{statusItems.map(([Icon,label,id]) => <span className="nx-status" data-status={id} key={id}><Icon aria-hidden="true" />{label}</span>)}</div>
      <div className="nx-role-row" aria-label="Evidence strength">{strengthItems.map(([Icon,label,id]) => <span className="nx-strength" data-strength={id} key={id}><Icon aria-hidden="true" />{label}</span>)}</div>
      <div className="nx-data-colours">{Array.from({ length: 6 }, (_, index) => <span key={index}><i style={{ background: `var(--nx-data-${index + 1})` }} /><em>Data {index + 1}</em></span>)}<span><i style={{ background: 'var(--nx-data-gray)' }} /><em>Context</em></span></div>
      <div className="nx-scale-row"><strong>Sequential</strong>{Array.from({ length: 7 }, (_, index) => <span key={index} style={{ background: `var(--nx-seq-${index + 1})` }} />)}</div>
      <div className="nx-scale-row"><strong>Diverging</strong>{Array.from({ length: 7 }, (_, index) => <span key={index} style={{ background: `var(--nx-div-${index + 1})` }} />)}</div>
    </section>
  )
}

function TypeSection() {
  return (
    <section className="nx-foundation-section" aria-labelledby="foundation-type">
      <header><span>02 · Type</span><h2 id="foundation-type">A record that reads in two scripts</h2><p>Noto keeps Latin and Arabic-script metrics related; identifiers remain isolated and left-to-right.</p></header>
      <div className="nx-type-specimen">
        <article><span>Display · 36/1.55</span><strong className="nx-type-display">Every figure opens its source.</strong></article>
        <article><span>Body · 14/1.55</span><p>There are 184,620 generated source rows in this demonstration. The number is tabular and the locator stays beside it.</p></article>
        <article dir="auto" className="nx-urdu"><span>Urdu shaping proof</span><p>یہ ایک مصنوعی تحقیقاتی ریکارڈ ہے۔ ہر عدد اپنے ماخذ سے منسلک ہے۔</p></article>
        <article><span>Mixed direction</span><p dir="auto">رابطہ <bdi className="nx-identifier">923001110001</bdi> چار مصنوعی ریکارڈز میں ظاہر ہوا۔</p></article>
        <article><span>Mono and tabular</span><code>SHA256 8d7f…4b19 · ROW 000184620</code></article>
      </div>
    </section>
  )
}

function StructureSection({ density, reduced, setReduced }) {
  return (
    <section className="nx-foundation-section" aria-labelledby="foundation-structure">
      <header><span>03 · Structure</span><h2 id="foundation-structure">Space, depth, density, icons and motion</h2><p>Tonal depth first. Density moves as a coordinated system. Motion explains state and then gets out of the way.</p></header>
      <div className="nx-token-columns">
        <article><h3>Spacing</h3><div className="nx-space-specimen">{[2,4,8,12,16,20,24,32,40,48,64].map(value => <span key={value}><i style={{ inlineSize: `var(--nx-space-${value})` }} />{value}</span>)}</div></article>
        <article><h3>Radius</h3><div className="nx-radius-specimen">{[1,2,3,4].map(value => <span key={value} style={{ borderRadius: `var(--nx-radius-${value})` }}>R{value}</span>)}</div></article>
        <article><h3>Depth</h3><div className="nx-depth-specimen"><span>Working</span><span>Raised</span><span>Overlay</span></div></article>
        <article><h3>Icons</h3><div className="nx-icon-specimen"><Search /><Database /><FileStack /><Gauge /><PanelTop /></div><small>16 / 18 / 20 px · 1.75 stroke · `currentColor`</small></article>
      </div>
      <div className="nx-density-proof" data-density-preview={density}><div><strong>Coordinated {density} density</strong><button type="button"><Search aria-hidden="true" /> Filter records</button></div><table><thead><tr><th>Identifier</th><th>Events</th><th>Strength</th></tr></thead><tbody>{['923001110001','923001110014','IMEI 351756…'].map((id,index) => <tr key={id}><td><code>{id}</code></td><td>{4-index}</td><td>{index ? 'Inferred' : 'Observed'}</td></tr>)}</tbody></table></div>
      <div className="nx-motion-proof" data-reduced={reduced || undefined}>
        <div><strong>Productive motion</strong><span>Hover or focus these samples. No lift, scale or loop.</span></div>
        <button type="button">Tone feedback</button><button type="button" className="nx-motion-proof__panel">Panel reveal</button>
        <label><input type="checkbox" checked={reduced} onChange={event => setReduced(event.target.checked)} /> Emulate reduced motion</label>
      </div>
    </section>
  )
}

function ContrastSection({ themeKey }) {
  const rows = useMemo(() => tokenReport.contrastPairs.filter(pair => pair.theme === themeKey), [themeKey])
  return (
    <section className="nx-foundation-section" aria-labelledby="foundation-contrast">
      <header><span>04 · Verification</span><h2 id="foundation-contrast">Generated contrast table</h2><p>{rows.length} used token pairs in this selected theme. This table is generated from the same resolved colours as the CSS.</p></header>
      <div className="nx-contrast-summary"><Check aria-hidden="true" /><strong>{rows.filter(row => row.pass).length} of {rows.length} pass</strong><span>WCAG 2.2 ratios, resolved in sRGB</span></div>
      <div className="nx-table-scroll"><table className="nx-contrast-table"><thead><tr><th>Foreground</th><th>Background</th><th>Use</th><th>Measured</th><th>Required</th><th>Result</th></tr></thead><tbody>{rows.map(row => <tr key={`${row.foreground}-${row.background}`}><td>{row.foreground}</td><td>{row.background}</td><td>{row.kind}</td><td>{row.ratio.toFixed(2)}:1</td><td>{row.required}:1</td><td><span data-pass={row.pass}>{row.pass ? 'Pass' : 'Fail'}</span></td></tr>)}</tbody></table></div>
    </section>
  )
}

function SimulationSection({ simulation, setSimulation }) {
  const matrix = {
    protanopia: '0.152286 1.052583 -0.204868 0 0 0.114503 0.786281 0.099216 0 0 -0.003882 -0.048116 1.051998 0 0 0 0 0 1 0',
    deuteranopia: '0.367322 0.860646 -0.227968 0 0 0.280085 0.672501 0.047413 0 0 -0.011820 0.042940 0.968881 0 0 0 0 0 1 0',
    tritanopia: '1.255528 -0.076749 -0.178779 0 0 -0.078411 0.930809 0.147602 0 0 0.004733 0.691367 0.303900 0 0 0 0 0 1 0',
  }
  return (
    <section className="nx-foundation-section" aria-labelledby="foundation-cvd">
      <svg className="nx-filter-defs" aria-hidden="true"><defs>{Object.entries(matrix).map(([name, values]) => <filter id={`nx-${name}`} key={name}><feColorMatrix type="matrix" values={values} /></filter>)}</defs></svg>
      <header><span>05 · Colour vision</span><h2 id="foundation-cvd">Simulation and redundant encoding</h2><p>The Machado preview is diagnostic. Words, shapes, patterns and labels carry the actual guarantee.</p></header>
      <Segmented label="Simulation" value={simulation} onChange={setSimulation} options={['normal','protanopia','deuteranopia','tritanopia','greyscale'].map(value => ({ value, label: value[0].toUpperCase() + value.slice(1) }))} />
      <div className="nx-cvd-proof" data-simulation={simulation}>
        <div className="nx-data-colours">{Array.from({ length: 6 }, (_, index) => <span key={index}><i style={{ background: `var(--nx-data-${index + 1})` }} /><em>Series {index + 1}</em></span>)}</div>
        <div className="nx-role-row">{strengthItems.map(([Icon,label,id]) => <span className="nx-strength" data-strength={id} key={id}><Icon aria-hidden="true" />{label}</span>)}</div>
      </div>
    </section>
  )
}

export default function FoundationsPage() {
  const [direction, setDirection] = useState('a')
  const [theme, setTheme] = useState('light')
  const [viewport, setViewport] = useState('1440')
  const [density, setDensity] = useState('default')
  const [reduced, setReduced] = useState(false)
  const [simulation, setSimulation] = useState('normal')
  const themeKey = theme === 'high-contrast' ? 'high-contrast' : `${direction}-${theme}`
  return (
    <main id="workspace-main" tabIndex="-1" className="nx-foundations">
      <header className="nx-foundations__masthead">
        <div><span className="nx-kicker">U1 · Foundations · generated content only</span><h1>A calm instrument for evidence</h1><p>Compare two directions using the same screens, data, density and verification. Colour is the only owner decision here.</p></div>
        <span className="nx-foundations__status"><CheckCircle2 aria-hidden="true" /> 160 token pairs pass</span>
      </header>

      <section className="nx-foundations__controls" aria-label="Foundation preview controls">
        <Segmented label="Direction" value={direction} onChange={setDirection} options={Object.entries(directions).map(([value,item]) => ({ value, label: `${value.toUpperCase()} · ${item.name}` }))} />
        <Segmented label="Theme" value={theme} onChange={setTheme} options={[
          { value:'light', label:'Light', icon:Sun }, { value:'dark', label:'Dark', icon:Moon }, { value:'high-contrast', label:'High contrast', icon:ShieldCheck },
        ]} />
        <Segmented label="Preview width" value={viewport} onChange={setViewport} options={[{value:'1440',label:'1440 px'},{value:'390',label:'390 px'}]} />
        <Segmented label="Density" value={density} onChange={setDensity} options={['compact','default','comfortable'].map(value => ({ value, label:value }))} />
      </section>

      <section className="nx-direction-summary" data-foundation-direction={direction} data-theme-preview={theme}>
        <span>{direction.toUpperCase()}</span><div><h2>{directions[direction].name}</h2><p>{directions[direction].note}</p></div>
      </section>

      <div className="nx-foundations__system" data-foundation-direction={direction} data-theme-preview={theme} data-density-preview={density}>
        <PaletteSection />
        <TypeSection />
        <StructureSection density={density} reduced={reduced} setReduced={setReduced} />
        <ContrastSection themeKey={themeKey} />
        <SimulationSection simulation={simulation} setSimulation={setSimulation} />

        <section className="nx-foundation-section nx-foundation-section--screens" aria-labelledby="foundation-screens">
          <header><span>06 · Decision screens</span><h2 id="foundation-screens">The same three screens, every time</h2><p>{directions[direction].name} · {theme} · {viewport} px · {density}. Synthetic values are labelled and exist only for this gallery.</p></header>
          <div className="nx-screen-gallery" data-preview-width={viewport}>
            {previewScreens.map(([name, Screen]) => <figure className="nx-screen-frame" key={name}><figcaption>{name} · {viewport} px</figcaption><div className="nx-screen-frame__viewport"><Screen /></div></figure>)}
          </div>
        </section>
      </div>
    </main>
  )
}

