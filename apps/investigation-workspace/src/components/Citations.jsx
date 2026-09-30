import { useId, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { LanguageText } from './AnalystComponents.jsx'
import { contributionShare } from '../lib/answerSummary.js'

const previewInset = 12
const previewGap = 8
const previewMaxWidth = 320

function locatorText(locator = {}) {
  const parts = []
  if (locator.rowNumber !== null && locator.rowNumber !== undefined) parts.push(`row ${locator.rowNumber}`)
  if (locator.rowHash) parts.push(`hash ${locator.rowHash}`)
  if (locator.page !== null && locator.page !== undefined) parts.push(`page ${locator.page}`)
  if (locator.passage !== null && locator.passage !== undefined) parts.push(`passage ${locator.passage}`)
  if (locator.charSpan) parts.push(`characters ${JSON.stringify(locator.charSpan)}`)
  if (locator.startSeconds !== null && locator.startSeconds !== undefined) parts.push(`${locator.startSeconds} seconds`)
  if (locator.endSeconds !== null && locator.endSeconds !== undefined) parts.push(`to ${locator.endSeconds} seconds`)
  if (locator.frameSeconds !== null && locator.frameSeconds !== undefined) parts.push(`frame ${locator.frameSeconds} seconds`)
  if (locator.bbox) parts.push(`region ${JSON.stringify(locator.bbox)}`)
  if (locator.artifactId) parts.push(`finding ${locator.artifactId}`)
  return parts.join(' · ')
}

function proofRoleText(value) {
  const text = String(value || 'source lineage').replaceAll('_', ' ').trim()
  return text.charAt(0).toUpperCase() + text.slice(1)
}

export function EvidenceStrengthBadge({ strength, ariaLabel }) {
  const value = strength || { id: 'weak', label: 'Strength unavailable' }
  return <span className={`evidence-strength evidence-strength--${value.id}`} aria-label={ariaLabel || `Confidence classification: ${value.label}`}><span className="evidence-strength__icon" aria-hidden="true">{value.id === 'strong' ? '●' : value.id === 'medium' ? '◐' : '○'}</span><span>{value.label}</span></span>
}

function CitationPreview({ anchor, citation, id, number, onClose }) {
  const previewRef = useRef(null)
  const [position, setPosition] = useState({ insetInlineStart: previewInset, insetBlockStart: previewInset, width: previewMaxWidth })

  useLayoutEffect(() => {
    function placePreview() {
      if (!anchor.current || !previewRef.current) return
      const anchorBox = anchor.current.getBoundingClientRect()
      const previewBox = previewRef.current.getBoundingClientRect()
      const width = Math.min(previewMaxWidth, globalThis.innerWidth - (previewInset * 2))
      const physicalInlineStart = Math.min(
        Math.max(previewInset, anchorBox.left + (anchorBox.width / 2) - (width / 2)),
        globalThis.innerWidth - width - previewInset,
      )
      const rightToLeft = globalThis.getComputedStyle(globalThis.document.documentElement).direction === 'rtl'
      const inlineStart = rightToLeft
        ? globalThis.innerWidth - physicalInlineStart - width
        : physicalInlineStart
      const below = anchorBox.bottom + previewGap
      const blockStart = below + previewBox.height <= globalThis.innerHeight - previewInset
        ? below
        : Math.max(previewInset, anchorBox.top - previewBox.height - previewGap)
      setPosition({ insetInlineStart: inlineStart, insetBlockStart: blockStart, width })
    }
    placePreview()
    globalThis.addEventListener('resize', placePreview)
    globalThis.addEventListener('scroll', placePreview, true)
    return () => {
      globalThis.removeEventListener('resize', placePreview)
      globalThis.removeEventListener('scroll', placePreview, true)
    }
  }, [anchor, citation])

  useLayoutEffect(() => {
    function dismiss(event) {
      if (event.key === 'Escape') onClose()
    }
    globalThis.document.addEventListener('keydown', dismiss)
    return () => globalThis.document.removeEventListener('keydown', dismiss)
  }, [onClose])

  return createPortal(
    <span
      ref={previewRef}
      id={id}
      className="citation-preview"
      role="tooltip"
      style={position}
    >
      <span className="citation-preview__label">Source {number}</span>
      <strong>{citation.label}</strong>
      <span>{citation.detail}</span>
      {citation.timestamp ? <span><span className="citation-preview__field">Timestamp</span> <time dateTime={citation.timestamp}>{citation.timestamp}</time></span> : <span className="citation-preview__missing">Timestamp was not supplied.</span>}
      {citation.preview ? <LanguageText>{citation.preview}</LanguageText> : <span className="citation-preview__missing">No source excerpt was supplied with this locator.</span>}
      <EvidenceStrengthBadge strength={citation.strength} />
      <span>{proofRoleText(citation.proofRole)}</span>
      <span className="citation-preview__locator"><span>Exact locator</span> <LanguageText as="bdi" identifier>{locatorText(citation.locator)}</LanguageText></span>
    </span>,
    globalThis.document.body,
  )
}

export function CitationMarker({ citation, number = citation.sourceNumber || 1, children = null }) {
  const previewId = useId()
  const anchor = useRef(null)
  const [open, setOpen] = useState(false)
  return (
    <span
      className="citation-marker-wrap"
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
    >
      <a
        ref={anchor}
        className={`citation-marker${children ? ' citation-marker--claim' : ''}`}
        href={citation.href}
        aria-label={`Open source ${number}: ${citation.label}, ${citation.detail}`}
        aria-describedby={open ? previewId : undefined}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onKeyDown={event => {
          if (event.key === 'Escape') {
            event.preventDefault()
            event.stopPropagation()
            setOpen(false)
          }
        }}
      >
        {children ? <><span className="citation-marker__claim">{children}</span><span className="citation-marker__number" aria-hidden="true">{number}</span></> : <span aria-hidden="true">{number}</span>}
      </a>
      {open && (
        <CitationPreview
          anchor={anchor}
          citation={citation}
          id={previewId}
          number={number}
          onClose={() => setOpen(false)}
        />
      )}
    </span>
  )
}

export function ClaimText({ text, citations = [], segments = [] }) {
  const firstClaim = segments.findIndex(segment => segment.claim)
  if (segments.length) return <span className="claim-text">{segments.map((segment, index) => {
    const citation = segment.citations[0]
    if (segment.claim && citation) return <CitationMarker key={`${citation.href}-${index}`} citation={citation}><LanguageText>{segment.text}</LanguageText></CitationMarker>
    return <span key={`${segment.text}-${index}`} className={segment.claim ? `claim-fragment claim-fragment--unlinked${index === firstClaim ? ' claim-fragment--primary' : ''}` : undefined}><LanguageText>{segment.text}</LanguageText></span>
  })}</span>
  if (!citations.length) return <LanguageText className="claim-text claim-text--unlinked">{text}</LanguageText>
  return (
    <span className="claim-text">
      <LanguageText>{text}</LanguageText>
      <span className="claim-citations" aria-label="Sources for this statement">
        {citations.slice(0, 3).map((citation, index) => (
          <CitationMarker key={citation.id || citation.href} citation={citation} number={citation.sourceNumber || index + 1} />
        ))}
      </span>
    </span>
  )
}

export function SourcePanel({ citations }) {
  const groups = citations.groups || citations.items
  if (!groups.length) {
    return <p className="source-empty" role="note">No openable source locator accompanied this response.</p>
  }
  const meanings = [...new Map(groups.map(item => [item.strength?.id || 'weak', item.strength])).values()].filter(Boolean)
  return (
    <>
      <p className="source-representative-note">Representative source rows from the contributing evidence. Expand a file to inspect exact sample locations.</p>
      <div className="source-meaning" role="note" aria-label="Evidence meaning">
        <span>Evidence meaning</span>
        {meanings.map(item => <EvidenceStrengthBadge key={item.id} strength={item} />)}
      </div>
      <ol className="source-list">
        {groups.map((citation, index) => {
          const samples = citation.samples || [citation]
          const total = citation.totalContributing
          const contributing = citation.contributingCount || samples.length
          return (
            <li key={`${citation.href}-${index}`}>
              <details className="source-group">
                <summary>
                  <span className="source-number" aria-hidden="true">{citation.sourceNumber || index + 1}</span>
                  <span>
                    <strong><LanguageText>{citation.label}</LanguageText></strong>
                    <small>{total ? `${contributing.toLocaleString()} of ${total.toLocaleString()} contributing rows` : `${samples.length} representative ${samples.length === 1 ? 'location' : 'locations'}`}</small>
                    {contributionShare(citation) !== null ? <span className="source-share" aria-hidden="true"><i style={{ inlineSize: `${Math.max(2, contributionShare(citation))}%` }} /></span> : null}
                    <small><EvidenceStrengthBadge strength={citation.strength} /></small>
                  </span>
                </summary>
                <ol className="source-samples">
                  {samples.map((sample, sampleIndex) => (
                    <li key={sample.href}>
                      <a href={sample.href} className="source-link">
                        <span>Sample {sampleIndex + 1}</span>
                        <small><LanguageText>{sample.detail}</LanguageText></small>
                      </a>
                    </li>
                  ))}
                </ol>
              </details>
            </li>
          )
        })}
      </ol>
    </>
  )
}
