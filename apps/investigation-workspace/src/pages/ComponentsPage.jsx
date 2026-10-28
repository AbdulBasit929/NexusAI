import { useState } from 'react'
import {
  Activity,
  Bell,
  BriefcaseBusiness,
  ChevronDown,
  CircleUserRound,
  Database,
  FileQuestion,
  Folders,
  Gauge,
  LayoutDashboard,
  Menu,
  MessageSquareText,
  Network,
  PanelLeftClose,
  Search,
  Settings2,
  ShieldCheck,
  SlidersHorizontal,
  SunMoon,
  Timeline,
  TriangleAlert,
} from 'lucide-react'
import '../styles/pages/components.css'

const directions = {
  a: {
    name: 'Ledger rail',
    summary: 'The safest, most explicit default: a 248 px rail gives Workspace and Current case equal weight.',
    strength: 'Fastest orientation for occasional and new analysts.',
    tradeoff: 'Uses the most horizontal space before the analyst collapses it.',
  },
  b: {
    name: 'Case atlas',
    summary: 'An expanded rail where the current case is a distinct working context and global destinations recede.',
    strength: 'Strongest case-centric mental model and clearest change of scope.',
    tradeoff: 'The highlighted case block adds visual weight to the rail.',
  },
  c: {
    name: 'Signal dock',
    summary: 'A 56 px icon rail is the desktop default; the case band and command surface carry orientation.',
    strength: 'Maximum canvas for tables, maps and connection work.',
    tradeoff: 'Icon recall and tooltip discovery make it less forgiving for infrequent users.',
  },
}

const workspaceItems = [
  [LayoutDashboard, 'Dashboard'],
  [MessageSquareText, 'Ask across cases'],
  [Folders, 'Cases'],
  [Activity, 'Activity'],
]

const caseItems = [
  [Gauge, 'Overview'],
  [Database, 'Evidence'],
  [MessageSquareText, 'Ask'],
  [Network, 'Connections'],
  [Timeline, 'Timeline'],
  [SlidersHorizontal, 'Data'],
  [Activity, 'Activity'],
]

function Segmented({ label, value, onChange, options }) {
  return (
    <div className="nx-component-segmented" role="group" aria-label={label}>
      <span>{label}</span>
      <div>{options.map(option => (
        <button key={option.value} type="button" aria-pressed={value === option.value} onClick={() => onChange(option.value)}>{option.label}</button>
      ))}</div>
    </div>
  )
}

function Mark() {
  return <svg viewBox="0 0 32 32" aria-hidden="true"><path d="M7 24V8l18 16V8" /><path d="M7 8h6l12 11" /></svg>
}

function RailItem({ item, active, compact }) {
  const [Icon, label] = item
  return (
    <span className={active ? 'is-active' : undefined} title={compact ? label : undefined}>
      <Icon aria-hidden="true" />
      <i>{label}</i>
      {label === 'Evidence' ? <b aria-label="1 failed source">1</b> : null}
    </span>
  )
}

function PreviewRail({ direction }) {
  const compact = direction === 'c'
  return (
    <div className="nx-shell-preview__rail" aria-label="Navigation preview">
      <div className="nx-shell-preview__rail-head"><BriefcaseBusiness aria-hidden="true" /><span><strong>Operation Glass Harbor</strong><small>CASE-2026-041</small></span></div>
      <nav aria-label="Workspace preview">
        <small>Workspace</small>
        {workspaceItems.map(item => <RailItem key={item[1]} item={item} compact={compact} />)}
      </nav>
      <nav className="nx-shell-preview__case-nav" aria-label="Case preview">
        <small>Current case</small>
        {caseItems.map((item, index) => <RailItem key={item[1]} item={item} active={index === 0} compact={compact} />)}
      </nav>
      <div className="nx-shell-preview__rail-foot"><Settings2 aria-hidden="true" /><i>Settings</i></div>
    </div>
  )
}

function PreviewHeader({ caseView }) {
  return (
    <header className="nx-shell-preview__header">
      <button className="nx-shell-preview__menu" type="button" aria-label="Open navigation"><Menu /></button>
      <span className="nx-shell-preview__brand"><Mark /><strong>NexusAI</strong></span>
      <button className="nx-shell-preview__switcher" type="button"><span>{caseView ? 'Operation Glass Harbor' : 'All cases'}</span><ChevronDown /></button>
      <button className="nx-shell-preview__command" type="button"><Search /><span>Find or run a command</span><kbd>Ctrl K</kbd></button>
      <div className="nx-shell-preview__utilities">
        <button type="button" aria-label="Activity"><Bell /><b>2</b></button>
        <button type="button" aria-label="Appearance"><SunMoon /></button>
        <button type="button" aria-label="Analyst profile"><CircleUserRound /></button>
      </div>
    </header>
  )
}

function CaseBand({ caseView }) {
  if (!caseView) return null
  return (
    <section className="nx-shell-preview__case-band" aria-label="Case context preview">
      <div><strong>Operation Glass Harbor</strong><span><ShieldCheck /> Active</span><span><TriangleAlert /> 1 source needs review</span></div>
      <nav aria-label="Case sections preview">
        {caseItems.map(([, label], index) => <span key={label} className={index === 0 ? 'is-current' : undefined}>{label}</span>)}
      </nav>
      <button type="button"><span>Overview</span><ChevronDown /></button>
    </section>
  )
}

function WorkSurface({ caseView }) {
  return (
    <div className="nx-shell-preview__main" aria-label="Working surface preview">
      <div className="nx-shell-preview__title-row">
        <div><small>{caseView ? 'Case workspace' : 'Workspace'}</small><h3>{caseView ? 'Case overview' : 'Dashboard'}</h3><p>{caseView ? 'Evidence readiness, quality and next actions.' : 'Work that needs your attention across every case.'}</p></div>
        <button type="button">{caseView ? 'Add evidence' : 'Create case'}</button>
      </div>
      <section className="nx-shell-preview__notice"><TriangleAlert /><span><strong>{caseView ? 'One source is excluded from totals' : 'Two cases need review'}</strong><small>Open the underlying record before relying on the affected count.</small></span><button type="button">Review</button></section>
      <section className="nx-shell-preview__metrics" aria-label="Generated demonstration metrics">
        {[['Ready sources','18'],['Generated rows','184,620'],['Processing','2'],['Failed','1']].map(([label, value]) => <article key={label}><span>{label}</span><strong>{value}</strong></article>)}
      </section>
      <section className="nx-shell-preview__work">
        <div><header><span><strong>Evidence readiness</strong><small>Generated demonstration</small></span><button type="button">View evidence</button></header>{['Call records','Internet sessions','Subscriber records','Documents'].map((label, index) => <p key={label}><span>{label}</span><i><b style={{ inlineSize: `${91 - index * 14}%` }} /></i><em>{[8420,4601,318,24][index].toLocaleString()}</em></p>)}</div>
        <div><header><span><strong>Recent activity</strong><small>Generated demonstration</small></span></header>{['Evidence import completed','Question answered from 4 sources','Source marked for review'].map((label, index) => <p key={label}><span className={index === 2 ? 'is-warning' : undefined}>{index === 2 ? <TriangleAlert /> : <Activity />}</span><i><strong>{label}</strong><small>{index + 2} hours ago</small></i></p>)}</div>
      </section>
    </div>
  )
}

function DetailsPanel() {
  return (
    <div className="nx-shell-preview__details" aria-label="Details preview">
      <header><span><small>Details</small><strong>Evidence health</strong></span><button type="button" aria-label="Close details">×</button></header>
      <dl><div><dt>Scope</dt><dd>18 ready sources</dd></div><div><dt>Coverage</dt><dd>96.4%</dd></div><div><dt>Last activity</dt><dd>14 minutes ago</dd></div></dl>
      <section><TriangleAlert /><div><strong>Review required</strong><p>One structured source failed validation and is excluded.</p><button type="button">Open source</button></div></section>
    </div>
  )
}

function ShellPreview({ direction, viewport, theme, caseView }) {
  return (
    <div className="nx-shell-frame" data-preview-width={viewport}>
      <div className="nx-shell-preview" data-direction={direction} data-foundation-direction="b" data-theme-preview={theme}>
        <PreviewHeader caseView={caseView} />
        <PreviewRail direction={direction} />
        <CaseBand caseView={caseView} />
        <WorkSurface caseView={caseView} />
        {caseView ? <DetailsPanel /> : null}
      </div>
    </div>
  )
}

function DecisionCard({ id, selected, onSelect }) {
  const direction = directions[id]
  return (
    <article className="nx-component-option" data-selected={selected || undefined}>
      <span>{id.toUpperCase()}</span><div><h2>{direction.name}</h2><p>{direction.summary}</p></div>
      <dl><div><dt>Best at</dt><dd>{direction.strength}</dd></div><div><dt>Cost</dt><dd>{direction.tradeoff}</dd></div></dl>
      <button type="button" aria-pressed={selected} onClick={() => onSelect(id)}>Preview {id.toUpperCase()}</button>
    </article>
  )
}

export default function ComponentsPage() {
  const [direction, setDirection] = useState('b')
  const [theme, setTheme] = useState('light')
  const [viewport, setViewport] = useState('1440')
  const [scope, setScope] = useState('case')
  const current = directions[direction]
  return (
    <main id="workspace-main" tabIndex="-1" className="nx-components-page">
      <header className="nx-components-page__masthead">
        <div><span>U2 · Shell and navigation · synthetic content only</span><h1>Choose how the instrument holds its work</h1><p>The information architecture, keyboard model and responsive rules are fixed. Compare which desktop emphasis should become NexusAI’s default.</p></div>
        <span><ShieldCheck /> Direction B selected</span>
      </header>

      <section className="nx-component-options" aria-label="Shell directions">
        {Object.keys(directions).map(id => <DecisionCard key={id} id={id} selected={direction === id} onSelect={setDirection} />)}
      </section>

      <section className="nx-components-page__controls" aria-label="Shell preview controls">
        <Segmented label="Theme" value={theme} onChange={setTheme} options={[{value:'light',label:'Light'},{value:'dark',label:'Dark'},{value:'high-contrast',label:'High contrast'}]} />
        <Segmented label="Preview width" value={viewport} onChange={setViewport} options={[{value:'1440',label:'1440 px'},{value:'390',label:'390 px'}]} />
        <Segmented label="Scope" value={scope} onChange={setScope} options={[{value:'case',label:'Case workspace'},{value:'global',label:'Global workspace'}]} />
      </section>

      <section className="nx-components-page__preview" aria-labelledby="shell-preview-title">
        <header><span>{direction.toUpperCase()}</span><div><h2 id="shell-preview-title">{current.name}</h2><p>{current.summary}</p></div><small>{theme} · {viewport} px · {scope}</small></header>
        <ShellPreview direction={direction} viewport={viewport} theme={theme} caseView={scope === 'case'} />
      </section>

      <section className="nx-components-page__contract" aria-labelledby="fixed-contract-title">
        <header><span>Fixed across A, B and C</span><h2 id="fixed-contract-title">These are not owner decisions</h2></header>
        <div>
          <article><PanelLeftClose /><strong>Responsive rail</strong><p>Remembered expanded/collapsed state; collapsed below 1024 px; modal drawer below 768 px.</p></article>
          <article><Search /><strong>Command surface</strong><p>Ctrl/Cmd+K opens Go to, Do and Ask groups; selection uses one combobox/listbox focus model.</p></article>
          <article><FileQuestion /><strong>Truthful global states</strong><p>Offline, not found, access denied and unexpected failure remain distinct, actionable states.</p></article>
          <article><SunMoon /><strong>Appearance</strong><p>System, light, dark and high contrast; density and reduced motion apply before the working surface paints.</p></article>
        </div>
      </section>
    </main>
  )
}
