import { LanguageText } from '../AnalystComponents.jsx'

function number(value) { const result = Number(value); return Number.isFinite(result) ? result : null }
function parsed(value) { if (!value) return null; try { return JSON.parse(value) } catch { return null } }

function spanBounds(value) {
  const span = parsed(value) || value
  if (Array.isArray(span)) return [number(span[0]), number(span[1])]
  if (span && typeof span === 'object') return [number(span.start ?? span.char_start), number(span.end ?? span.char_end)]
  return [null, null]
}

// A page of source text with the cited character span marked, when the citation gave one that fits the text.
export function HighlightedText({ text, span }) {
  const [start, end] = spanBounds(span)
  if (start === null || end === null || start < 0 || end <= start || start >= text.length) return <LanguageText as="pre" className="source-text">{text}</LanguageText>
  return <LanguageText as="pre" className="source-text" isolate={false}>{text.slice(0, start)}<mark id="exact-source" className="arrival-highlight">{text.slice(start, Math.min(end, text.length))}</mark>{text.slice(end)}</LanguageText>
}
