import { useEffect, useMemo, useRef, useState } from 'react'
import { Check, ChevronDown, ChevronUp, Copy, Search } from 'lucide-react'
import { EvidenceStrengthBadge } from '../Citations.jsx'
import { LanguageText } from '../AnalystComponents.jsx'
import { documentPages } from '../../lib/viewerPresentation.js'
import { strength } from '../../lib/viewerStrength.js'
import { findMatches, pageMatchCounts, splitByMatches } from '../../lib/viewerTools.js'
import { HighlightedText } from './HighlightedText.jsx'

// The document as a reader: a page rail with where the search words occur, find-in-document with a count and next/previous,
// page navigation, and copy for the page in view. The page text is what the service extracted; when it sent none, the
// retained file itself is shown, so nothing is invented.
export function DocumentReader({ detail, objectUrl, page, charSpan, onPageChange }) {
  const pages = useMemo(() => documentPages(detail), [detail])
  const requested = Number(page) || pages[0]?.number || 1
  const selectedIndex = Math.max(0, pages.findIndex(item => item.number === requested))
  const selected = pages[selectedIndex]
  const [query, setQuery] = useState('')
  const [hit, setHit] = useState(0)
  const [copied, setCopied] = useState(false)
  const body = useRef(null)
  const counts = useMemo(() => pageMatchCounts(pages, query), [pages, query])
  const matches = useMemo(() => findMatches(selected?.text || '', query), [selected, query])
  const total = [...counts.values()].reduce((sum, value) => sum + value, 0)
  const searching = query.trim().length > 0

  useEffect(() => { setHit(0) }, [query, requested])
  useEffect(() => { body.current?.querySelector('.dr-hit.is-active, #exact-source')?.scrollIntoView?.({ block: 'center' }) }, [hit, requested, query, charSpan])

  function step(direction) {
    if (matches.length) { setHit(value => (value + direction + matches.length) % matches.length); return }
    const order = pages.filter(item => counts.get(item.number))
    const next = direction > 0 ? order.find(item => item.number > requested) || order[0] : [...order].reverse().find(item => item.number < requested) || order[order.length - 1]
    if (next) onPageChange(next.number)
  }
  async function copy() {
    try { await globalThis.navigator.clipboard.writeText(selected?.text || ''); setCopied(true); globalThis.setTimeout(() => setCopied(false), 1500) } catch { setCopied(false) }
  }

  return (
    <section className="source-viewer document-viewer dr" aria-labelledby="document-viewer-heading">
      <header>
        <h2 id="document-viewer-heading">Document source · page {requested}</h2>
        {charSpan ? <EvidenceStrengthBadge strength={strength(null, true)} ariaLabel="Evidence strength" /> : null}
      </header>
      {selected?.text ? (
        <div className="dr__layout">
          {pages.length > 1 ? (
            <nav className="dr__rail" aria-label="Document pages">
              <ol>
                {pages.map(item => (
                  <li key={item.number}>
                    <button type="button" aria-current={item.number === requested ? 'page' : undefined} onClick={() => onPageChange(item.number)}>
                      <span>Page {item.number}</span>
                      {searching && counts.get(item.number) ? <b title={`${counts.get(item.number)} matches`}>{counts.get(item.number)}</b> : null}
                    </button>
                  </li>
                ))}
              </ol>
            </nav>
          ) : null}
          <div className="dr__reader">
            <div className="dr__tools">
              <label className="dr__search"><Search aria-hidden="true" /><span className="visually-hidden">Find in document</span><input type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder="Find in document" /></label>
              {searching ? (
                <span className="dr__count" role="status">{total ? `${matches.length ? `${Math.min(hit + 1, matches.length)} of ${matches.length} on this page · ` : ''}${total} in document` : 'No matches'}</span>
              ) : null}
              {searching ? (
                <span className="dr__step">
                  <button type="button" onClick={() => step(-1)} disabled={!total} aria-label="Previous match"><ChevronUp aria-hidden="true" /></button>
                  <button type="button" onClick={() => step(1)} disabled={!total} aria-label="Next match"><ChevronDown aria-hidden="true" /></button>
                </span>
              ) : null}
              <button type="button" className="dr__copy" onClick={copy}>{copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}{copied ? 'Copied' : 'Copy page text'}</button>
            </div>
            <div className="dr__page" ref={body}>
              {searching && matches.length ? (
                <LanguageText as="pre" className="source-text" isolate={false}>{splitByMatches(selected.text, matches).map((part, index) => part.match ? <mark key={index} className={`dr-hit${part.index === hit ? ' is-active' : ''}`}>{part.text}</mark> : part.text)}</LanguageText>
              ) : <HighlightedText text={selected.text} span={charSpan} />}
            </div>
            {pages.length > 1 ? (
              <div className="dr__pager">
                <button type="button" disabled={selectedIndex === 0} onClick={() => onPageChange(pages[selectedIndex - 1].number)}>Previous page</button>
                <span>Page {selectedIndex + 1} of {pages.length}</span>
                <button type="button" disabled={selectedIndex === pages.length - 1} onClick={() => onPageChange(pages[selectedIndex + 1].number)}>Next page</button>
              </div>
            ) : null}
          </div>
        </div>
      ) : objectUrl ? <iframe title="Document source" src={`${objectUrl}#page=${requested}`} /> : <p>The retained document cannot be displayed inline.</p>}
    </section>
  )
}
