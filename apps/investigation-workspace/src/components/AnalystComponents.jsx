import { Children, useId, useState } from 'react'

const arabicScript = /[\u0600-\u06ff\u0750-\u077f\u0870-\u08ff\ufb50-\ufdff\ufe70-\ufefc]/u
const identifierToken = /(\b(?:\+?\d[\d -]{3,}\d|[0-9a-f]{8}-[0-9a-f-]{12,}|[0-9a-f]{12,}|[A-Z]{1,4}-?\d{1,5}|\d{1,2}:\d{2}(?::\d{2})?|\d{4,})\b)/giu

const stateContent = {
  answered: ['Analysis complete', 'A verified finding is available from the selected evidence.', 'complete'],
  partial: ['Analysis complete with limits', 'The finding is supported, with stated coverage limits.', 'attention'],
  'zero-result': ['Analysis complete · no match', 'The requested analysis completed and found no matching evidence.', 'empty'],
  clarify: ['One detail is needed', 'No finding was stated until the intended analysis can be verified.', 'attention'],
  processing: ['Evidence is being processed', 'This analysis does not include evidence that is not ready.', 'processing'],
  unsupported: ['Analysis unavailable', 'The requested analysis is not available for this case.', 'unavailable'],
  failed: ['Analysis could not be completed', 'The case evidence was not changed.', 'failed'],
}

const emptyContent = {
  'complete-zero': ['Analysis complete · zero findings', 'The analysis ran across the stated scope and returned zero findings.', 'complete'],
  'no-match': ['No match in this view', 'Available evidence was checked, but none matches the current filters.', 'empty'],
  unavailable: ['Analysis unavailable', 'This capability or source is not available in the current case.', 'unavailable'],
  'not-processed': ['Evidence not processed', 'This evidence has not been processed, so no finding can be stated from it.', 'processing'],
}

const routeContent = {
  loading: ['Loading', 'The authoritative request is still in progress.', 'processing'],
  ready: ['Ready', 'The requested view resolved and is available.', 'complete'],
  empty: ['No records', 'The authoritative request completed with no records in this scope.', 'empty'],
  partial: ['Partially available', 'Completed artifacts remain visible; one named scope did not complete.', 'attention'],
  error: ['Could not load', 'The request failed. A reference is available for support.', 'failed'],
  forbidden: ['Access denied', 'Your authorized access does not include this view.', 'unavailable'],
  unavailable: ['Not available', 'The service does not expose the data this view requires.', 'unavailable'],
}

function StateIcon({ kind }) {
  const paths = {
    complete: <path d="M5 12.5 10 17l9-10" />,
    attention: <><path d="M12 3 2.8 20h18.4L12 3Z" /><path d="M12 9v4m0 3h.01" /></>,
    empty: <><circle cx="10.5" cy="10.5" r="6.5" /><path d="m15.5 15.5 5 5" /></>,
    processing: <><path d="M12 3v3m0 12v3M3 12h3m12 0h3" /><circle cx="12" cy="12" r="6" /></>,
    unavailable: <><circle cx="12" cy="12" r="9" /><path d="m7 17 10-10" /></>,
    failed: <><circle cx="12" cy="12" r="9" /><path d="m9 9 6 6m0-6-6 6" /></>,
  }
  return (
    <svg className="state-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      {paths[kind] || paths.unavailable}
    </svg>
  )
}

function isolateIdentifiers(value) {
  if (typeof value !== 'string') return value
  return value.split(identifierToken).map((part, index) => (
    index % 2 === 1
      ? <bdi key={`${part}-${index}`} dir="ltr" className="language-text__identifier">{part}</bdi>
      : part
  ))
}

export function LanguageText({ as: Element = 'span', children, className = '', isolate = true, identifier = false, ...props }) {
  const plain = Children.toArray(children).filter(child => typeof child === 'string' || typeof child === 'number').join('')
  const isUrdu = arabicScript.test(plain)
  return (
    <Element
      {...props}
      dir={identifier ? 'ltr' : 'auto'}
      lang={props.lang || (!identifier && isUrdu ? 'ur' : undefined)}
      className={`language-text${!identifier && isUrdu ? ' language-text--urdu' : ''}${identifier ? ' language-text__identifier' : ''}${className ? ` ${className}` : ''}`}
    >
      {isolate && !identifier ? Children.map(children, child => isolateIdentifiers(child)) : children}
    </Element>
  )
}

export function ResultStateBanner({ state, label, description }) {
  const [defaultLabel, defaultDescription, kind] = stateContent[state] || stateContent.unsupported
  return (
    <section className={`result-state-banner result-state-banner--${kind}`} aria-label="Analysis state" data-result-state={state}>
      <StateIcon kind={kind} />
      <span><strong>{label || defaultLabel}</strong><small>{description || defaultDescription}</small></span>
    </section>
  )
}

// One state-panel system for empty, loading, error, forbidden and unavailable views. The tone tints the
// icon mark only; the heading and description always say the state in words, sentence case, never blaming
// the analyst (Atlassian empty-state guidance). Loading shows layout-holding placeholder lines, no numbers.
const stateTone = { complete: 'positive', attention: 'caution', empty: 'neutral', processing: 'caution', unavailable: 'neutral', failed: 'critical' }

export function RouteState({ state, label, description, failedScope, reference, children }) {
  const [defaultLabel, defaultDescription, icon] = routeContent[state] || routeContent.unavailable
  return (
    <section className={`route-state state-panel state-panel--${stateTone[icon] || 'neutral'} route-state--${state}`} role={state === 'loading' ? 'status' : state === 'error' ? 'alert' : undefined} aria-busy={state === 'loading' ? 'true' : undefined}>
      <span className="state-mark"><StateIcon kind={icon} /></span>
      <div className="state-panel__copy">
        <h2>{label || defaultLabel}</h2>
        <p>{description || defaultDescription}</p>
        {failedScope ? <p><strong>Unavailable scope:</strong> {failedScope}</p> : null}
        {reference ? <p><strong>Reference:</strong> <LanguageText as="bdi" identifier>{reference}</LanguageText></p> : null}
        {state === 'loading' ? <div className="state-panel__skeleton" aria-hidden="true"><span className="skeleton__line" /><span className="skeleton__line" style={{ '--skeleton-width': '60%' }} /></div> : null}
        {children ? <div className="state-panel__actions">{children}</div> : null}
      </div>
    </section>
  )
}

export function FindingCard({ classification = 'fact', citationCount = 0, title, children }) {
  const labels = { fact: 'Supported finding', candidate: 'Candidate observation', contradiction: 'Contradictory evidence' }
  return (
    <section className={`finding-card finding-card--${classification}`} aria-label={labels[classification] || labels.fact}>
      <header>
        <span>{labels[classification] || labels.fact}</span>
        <span>{citationCount} {citationCount === 1 ? 'source' : 'sources'}</span>
      </header>
      {title && <h2>{title}</h2>}
      <div className="finding-card__content">{children}</div>
    </section>
  )
}

export function QualityPanel({ accepted = null, rejected = null, duplicates = null, missing = null }) {
  const values = [
    ['Accepted', accepted, 'Source entries retained for analysis.'],
    ['Rejected', rejected, 'Source entries that could not be admitted.'],
    ['Duplicates', duplicates, 'Repeated source entries excluded from totals.'],
    ['Missing', missing, 'Required values absent from admitted entries.'],
  ]
  return (
    <section className="quality-panel" aria-labelledby="quality-panel-title">
      <h2 id="quality-panel-title">Evidence quality</h2>
      <dl>{values.map(([label, value, definition]) => <div key={label}><dt>{label}</dt><dd><strong>{value === null || value === undefined ? 'Not reported' : Number(value).toLocaleString()}</strong><span>{definition}</span></dd></div>)}</dl>
    </section>
  )
}

export function ProcessingBadge({ state = 'not-processed' }) {
  const normalized = String(state || 'not-processed').toLowerCase().replaceAll('_', '-')
  const labels = { queued: 'Queued', registered: 'Queued', running: 'Processing', processing: 'Processing', completed: 'Ready', complete: 'Ready', failed: 'Failed', skipped: 'Excluded', duplicate: 'Excluded', excluded: 'Excluded', 'not-processed': 'Not processed' }
  const kind = ['completed', 'complete'].includes(normalized) ? 'complete' : normalized === 'failed' ? 'failed' : ['queued', 'registered', 'running', 'processing'].includes(normalized) ? 'processing' : 'unavailable'
  return <span className={`processing-badge processing-badge--${kind}`}><StateIcon kind={kind} />{labels[normalized] || 'Not processed'}</span>
}

export function SourceMetric({ label, value, unit, detail, primary = false }) {
  return (
    <div className={`source-metric${primary ? ' source-metric--primary' : ''}`}>
      <dt>{label}</dt>
      <dd><strong>{typeof value === 'number' ? value.toLocaleString() : value}</strong>{unit && <span>{unit}</span>}</dd>
      {detail && <small>{detail}</small>}
    </div>
  )
}

export function TechnicalDisclosure({ summary = 'Technical details', children }) {
  const [expanded, setExpanded] = useState(false)
  const contentId = useId()
  return <section className="technical-disclosure"><button type="button" aria-expanded={expanded} aria-controls={contentId} onClick={() => setExpanded(value => !value)}><span>{summary}</span><span aria-hidden="true">{expanded ? '−' : '+'}</span></button>{expanded ? <div id={contentId}>{children}</div> : null}</section>
}

export function EmptyState({ kind = 'unavailable', label, description, children }) {
  const [defaultLabel, defaultDescription, icon] = emptyContent[kind] || emptyContent.unavailable
  return (
    <section className={`empty-state state-panel state-panel--${stateTone[icon] || 'neutral'} empty-state--${kind}`}>
      <span className="state-mark"><StateIcon kind={icon} /></span>
      <div className="state-panel__copy"><h2>{label || defaultLabel}</h2><p>{description || defaultDescription}</p>{children ? <div className="state-panel__actions">{children}</div> : null}</div>
    </section>
  )
}
