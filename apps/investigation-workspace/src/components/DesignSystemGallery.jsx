import { useEffect, useId, useRef, useState } from 'react'
import {
  ArrowRight,
  Ban,
  Bell,
  Check,
  CheckCircle2,
  ChevronDown,
  CircleAlert,
  Clock3,
  ExternalLink,
  FileSearch,
  Info,
  Layers3,
  LoaderCircle,
  Menu,
  MoreHorizontal,
  PanelRight,
  Search,
  ShieldCheck,
  SlidersHorizontal,
  X,
} from 'lucide-react'
import { BrandLockup } from './BrandLockup.jsx'
import { PaletteOptions } from './PaletteOptions.jsx'
import { PageTemplatesDemo } from './PageTemplatesDemo.jsx'

const focusable = 'a[href], button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])'

const statuses = [
  { id: 'ready', label: 'Ready', icon: CheckCircle2 },
  { id: 'processing', label: 'Processing', icon: Clock3 },
  { id: 'failed', label: 'Failed', icon: CircleAlert },
  { id: 'excluded', label: 'Excluded', icon: Ban },
]

const evidenceStrength = [
  { id: 'strong', label: 'Strong evidence', detail: 'Openable source record' },
  { id: 'medium', label: 'Medium evidence', detail: 'Supported observation' },
  { id: 'weak', label: 'Weak evidence', detail: 'Review before relying' },
]

function OverlayDemo({ kind, title, description, onClose, returnFocus, children }) {
  const panel = useRef(null)
  const titleId = useId()

  useEffect(() => {
    const node = panel.current
    const items = [...node.querySelectorAll(focusable)]
    items[0]?.focus()

    function handleKey(event) {
      if (event.key === 'Escape') {
        event.preventDefault()
        onClose()
        return
      }
      if (event.key !== 'Tab' || !items.length) return
      const first = items[0]
      const last = items.at(-1)
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }

    document.addEventListener('keydown', handleKey)
    return () => {
      document.removeEventListener('keydown', handleKey)
      returnFocus?.current?.focus()
    }
  }, [onClose, returnFocus])

  return (
    <div className={`gallery-overlay gallery-overlay--${kind}`} onMouseDown={event => event.target === event.currentTarget && onClose()}>
      <section ref={panel} className="gallery-overlay__panel" role="dialog" aria-modal="true" aria-labelledby={titleId}>
        <header>
          <div><span className="eyebrow">Interaction preview</span><h3 id={titleId}>{title}</h3></div>
          <button type="button" className="gallery-icon-button" aria-label={`Close ${kind}`} onClick={onClose}><X /></button>
        </header>
        <p>{description}</p>
        {children}
      </section>
    </div>
  )
}

function InteractionControls() {
  const [activeTab, setActiveTab] = useState('summary')
  const [density, setDensity] = useState('comfortable')
  const [chip, setChip] = useState('all')
  const [popover, setPopover] = useState(false)
  const [modal, setModal] = useState(false)
  const [drawer, setDrawer] = useState(false)
  const [toast, setToast] = useState(false)
  const modalTrigger = useRef(null)
  const drawerTrigger = useRef(null)
  const tabs = ['summary', 'source', 'activity']

  function moveTab(event) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    const current = tabs.indexOf(activeTab)
    const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : (current + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length
    setActiveTab(tabs[next])
    event.currentTarget.parentElement.querySelector(`[data-tab="${tabs[next]}"]`)?.focus()
  }

  return (
    <>
      <section className="gallery-section" aria-labelledby="gallery-controls-title">
        <div className="gallery-section__heading"><div><span className="eyebrow">Selection</span><h2 id="gallery-controls-title">Controls that keep scope visible</h2></div><p>Tabs change views. Chips change scope. Static badges never impersonate actions.</p></div>
        <div className="gallery-showcase gallery-showcase--controls">
          <div className="gallery-control-group">
            <span className="gallery-label">Tabs</span>
            <div className="gallery-tabs" role="tablist" aria-label="Evidence detail views">
              {tabs.map(tab => <button key={tab} type="button" role="tab" data-tab={tab} aria-selected={activeTab === tab} tabIndex={activeTab === tab ? 0 : -1} onKeyDown={moveTab} onClick={() => setActiveTab(tab)}>{tab[0].toUpperCase() + tab.slice(1)}</button>)}
            </div>
            <div className="gallery-tab-panel" role="tabpanel"><FileSearch aria-hidden="true" /><span><strong>{activeTab[0].toUpperCase() + activeTab.slice(1)} view</strong><small>The selected view keeps the same source context.</small></span></div>
          </div>

          <div className="gallery-control-group">
            <span className="gallery-label">Segmented control</span>
            <div className="gallery-segmented" role="group" aria-label="Display density">
              {['compact', 'comfortable'].map(option => <button key={option} type="button" aria-pressed={density === option} onClick={() => setDensity(option)}>{option[0].toUpperCase() + option.slice(1)}</button>)}
            </div>
            <span className="gallery-label">Scope chips</span>
            <div className="gallery-chips" role="group" aria-label="Evidence family demonstration">
              {['all', 'documents', 'images'].map(option => <button key={option} type="button" aria-pressed={chip === option} onClick={() => setChip(option)}>{option[0].toUpperCase() + option.slice(1)}</button>)}
            </div>
          </div>
        </div>
      </section>

      <section className="gallery-section" aria-labelledby="gallery-overlays-title">
        <div className="gallery-section__heading"><div><span className="eyebrow">Layers</span><h2 id="gallery-overlays-title">Overlays with a clear way back</h2></div><p>Every transient layer closes with Escape and returns focus to its trigger.</p></div>
        <div className="gallery-showcase gallery-showcase--overlays">
          <div className="gallery-tooltip-demo">
            <button type="button" className="gallery-icon-button" aria-describedby="gallery-tooltip"><Info /></button>
            <span id="gallery-tooltip" role="tooltip">Openable locator available</span>
          </div>
          <div className="gallery-popover-demo">
            <button type="button" className="gallery-button gallery-button--secondary" aria-expanded={popover} onClick={() => setPopover(value => !value)}><SlidersHorizontal />View options<ChevronDown /></button>
            {popover ? <div className="gallery-popover" role="dialog" aria-label="View options"><strong>View options</strong><label><input type="checkbox" defaultChecked /> Show metadata</label><label><input type="checkbox" /> Wrap long values</label></div> : null}
          </div>
          <button ref={drawerTrigger} type="button" className="gallery-button gallery-button--secondary" onClick={() => setDrawer(true)}><PanelRight />Open drawer</button>
          <button ref={modalTrigger} type="button" className="gallery-button gallery-button--secondary" onClick={() => setModal(true)}><Layers3 />Open modal</button>
          <button type="button" className="gallery-button gallery-button--secondary" onClick={() => setToast(true)}><Bell />Show toast</button>
        </div>
      </section>

      {drawer ? <OverlayDemo kind="drawer" title="Source details" description="A drawer preserves the working view behind it." onClose={() => setDrawer(false)} returnFocus={drawerTrigger}><button type="button" className="gallery-button gallery-button--primary" onClick={() => setDrawer(false)}>Done<Check /></button></OverlayDemo> : null}
      {modal ? <OverlayDemo kind="modal" title="Confirm a bounded action" description="The action is stated clearly and the background is inert while this dialog is open." onClose={() => setModal(false)} returnFocus={modalTrigger}><div className="gallery-overlay__actions"><button type="button" className="gallery-button gallery-button--quiet" onClick={() => setModal(false)}>Cancel</button><button type="button" className="gallery-button gallery-button--primary" onClick={() => setModal(false)}>Confirm<ArrowRight /></button></div></OverlayDemo> : null}
      {toast ? <aside className="gallery-toast" role="status"><CheckCircle2 /><span><strong>Preference saved</strong><small>This demonstration does not change case data.</small></span><button type="button" aria-label="Dismiss notification" onClick={() => setToast(false)}><X /></button></aside> : null}
    </>
  )
}

export function DesignSystemGallery() {
  return (
    <div className="design-gallery">
      <section className="identity-field" aria-labelledby="identity-field-title">
        <div className="identity-field__content">
          <BrandLockup identityLed />
          <div><span className="identity-field__kicker"><ShieldCheck />Mineral Signal visual system</span><h2 id="identity-field-title">Clarity for evidence-heavy work.</h2><p>Graphite grounds the workspace. Cobalt guides action. Status colours keep their own meaning.</p></div>
          <div className="identity-field__actions"><button type="button" className="gallery-button gallery-button--light"><Search />Find evidence</button><button type="button" className="gallery-button gallery-button--ghost-light">View principles<ExternalLink /></button></div>
        </div>
        <svg className="identity-field__graphic" viewBox="0 0 460 260" aria-hidden="true">
          <defs><linearGradient id="signal-line" x1="0" x2="1"><stop stopColor="#60a5fa" /><stop offset="1" stopColor="#5eead4" /></linearGradient></defs>
          <path d="M34 198 116 118l70 40 85-108 72 65 84-78" />
          <circle cx="116" cy="118" r="8" /><circle cx="186" cy="158" r="8" /><circle cx="271" cy="50" r="8" /><circle cx="343" cy="115" r="8" />
          <rect x="50" y="64" width="128" height="76" rx="16" /><rect x="248" y="120" width="148" height="82" rx="16" />
        </svg>
      </section>

      <PaletteOptions />

      <section className="gallery-section" aria-labelledby="gallery-foundations-title">
        <div className="gallery-section__heading"><div><span className="eyebrow">Foundations</span><h2 id="gallery-foundations-title">Colour, surface and depth</h2></div><p>Solid work surfaces sit over a tinted canvas; identity fields carry the richer gradient.</p></div>
        <div className="gallery-foundations">
          <div className="gallery-swatches" aria-label="Core palette">
            {[['graphite', 'Mineral graphite'], ['primary', 'Action cobalt'], ['accent', 'Identity teal'], ['cyan', 'Signal cyan']].map(([tone, label]) => <div key={tone} className={`gallery-swatch gallery-swatch--${tone}`}><span /><strong>{label}</strong></div>)}
          </div>
          <div className="gallery-elevation" aria-label="Surface elevations"><div><span>Inset</span></div><div><span>Work surface</span></div><div><span>Overlay</span></div></div>
        </div>
      </section>

      <section className="gallery-section" aria-labelledby="gallery-actions-title">
        <div className="gallery-section__heading"><div><span className="eyebrow">Actions</span><h2 id="gallery-actions-title">A compact, legible action hierarchy</h2></div><p>One dominant action per region. Secondary and quiet actions recede without disappearing.</p></div>
        <div className="gallery-showcase gallery-showcase--buttons">
          <button type="button" className="gallery-button gallery-button--primary">Primary action<ArrowRight /></button>
          <button type="button" className="gallery-button gallery-button--secondary"><Search />Secondary</button>
          <button type="button" className="gallery-button gallery-button--quiet">Quiet action</button>
          <button type="button" className="gallery-button gallery-button--danger"><CircleAlert />Destructive</button>
          <button type="button" className="gallery-button gallery-button--primary" aria-busy="true"><LoaderCircle className="gallery-spin" />Working</button>
          <button type="button" className="gallery-button gallery-button--secondary" disabled>Unavailable</button>
          <button type="button" className="gallery-icon-button" aria-label="More actions"><MoreHorizontal /></button>
        </div>
      </section>

      <section className="gallery-section" aria-labelledby="gallery-state-title">
        <div className="gallery-section__heading"><div><span className="eyebrow">Meaning</span><h2 id="gallery-state-title">State never relies on colour alone</h2></div><p>Processing state and evidence strength remain separate vocabularies.</p></div>
        <div className="gallery-state-grid">
          <div><span className="gallery-label">Processing</span><div className="gallery-badges">{statuses.map(({ id, label, icon: Icon }) => <span key={id} className={`gallery-badge gallery-badge--${id}`}><Icon />{label}</span>)}</div></div>
          <div><span className="gallery-label">Evidence strength</span><div className="gallery-strengths">{evidenceStrength.map(item => <div key={item.id} className={`gallery-strength gallery-strength--${item.id}`}><span><ShieldCheck />{item.label}</span><small>{item.detail}</small></div>)}</div></div>
        </div>
      </section>

      <InteractionControls />

      <PageTemplatesDemo />

      <section className="gallery-section" aria-labelledby="gallery-feedback-title">
        <div className="gallery-section__heading"><div><span className="eyebrow">Waiting</span><h2 id="gallery-feedback-title">Skeletons preserve layout without inventing progress</h2></div><p>The animated shimmer is removed when the analyst prefers reduced motion.</p></div>
        <div className="gallery-skeleton" role="status" aria-label="Loading example"><span className="gallery-skeleton__mark" /><div><span /><span /><span /></div></div>
      </section>
    </div>
  )
}
