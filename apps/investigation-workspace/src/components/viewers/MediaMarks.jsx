import { useEffect, useRef } from 'react'
import { displayedRect, readsAt } from '../../lib/mediaReadings.js'
import { strength } from '../../lib/viewerStrength.js'

const MAX_TEXT_BOXES = 6

// A plate as plates are read: monospaced, spaced, set in a plain outline. `diff` ([{ character, differs }]) marks the characters that differ from the plate
// the readings settled on.
export function PlateChip({ children, diff = null }) {
  return (
    <span className="mx-plate">
      <bdi dir="ltr">{diff ? diff.map((part, index) => (part.differs ? <b key={index} className="is-diff">{part.character}</b> : part.character)) : children}</bdi>
    </span>
  )
}

// How sure the reader was, as a small bar and the figure; the colour follows the same bands as the rest of the product (high, to review, low).
export function Conf({ value }) {
  if (value === null || value === undefined) return null
  const level = strength(value)
  return <span className={`mx-conf mx-conf--${level.id}`} style={{ '--p': Math.max(0, Math.min(1, value)) }} title={level.label}><i aria-hidden="true" /><b>{Math.round(value * 100)}%</b></span>
}

// The ORIGINAL pixels of a detected region, drawn from the picture or the video frame itself: nothing is sharpened, tinted or redrawn, it is only fitted to a
// height so that it can be read beside the reading. The file is never changed.
export function RegionCrop({ source, box, height = 44, tick = 0, label }) {
  const canvas = useRef(null)
  useEffect(() => {
    const node = canvas.current
    const width = source?.naturalWidth || source?.videoWidth
    if (!node || !width || !box) return
    const [x, y, w, h] = box
    const scale = height / h
    node.height = Math.round(height)
    node.width = Math.max(1, Math.round(w * scale))
    const context = node.getContext('2d')
    context.imageSmoothingEnabled = true
    context.imageSmoothingQuality = 'high'
    try { context.drawImage(source, x, y, w, h, 0, 0, node.width, node.height) } catch { /* the frame is not ready yet */ }
  }, [source, box, height, tick])
  return <canvas ref={canvas} className="mx-crop" role="img" aria-label={label} />
}

// A region drawn as an outline only: nothing is painted inside it, so the detected pixels stay as they are; its label sits outside, above (or below when the
// region is at the top edge), on a solid chip that is readable on any picture. In a zoomed or turned image the label keeps its size and stays upright.
export function MediaBox({ style, tone = 'plate', label, below = false, chosen = false, cited = false }) {
  return (
    <span className={`mx-box mx-box--${tone}${chosen ? ' is-chosen' : ''}${cited ? ' is-cited' : ''}`} style={style}>
      {label ? <span className={`mx-box__label${below ? ' is-below' : ''}`}>{label}</span> : null}
    </span>
  )
}

// The plates and the words read on the picture at the playing second, drawn over the picture where it really is in the player (the player may leave bars beside
// or above it). Plates come from the plate groups (one box per plate, the reading nearest in time); words are few at a time.
export function VideoBoxes({ plates, texts, time, media, natural }) {
  const node = media?.current
  if (!node || !natural) return null
  const shown = [
    ...plates.flatMap(group => readsAt(group.reads || [], time, 0.6, () => group.id).map(read => ({ key: `p-${read.id}`, tone: 'plate', label: read.plate, box: read.box }))),
    ...readsAt(texts, time, 0.6, read => read.text).slice(0, MAX_TEXT_BOXES).map(read => ({ key: `t-${read.id}`, tone: 'text', label: read.text, box: read.box })),
  ]
  const rect = shown.length ? displayedRect(node.clientWidth, node.clientHeight, natural.width, natural.height) : null
  if (!rect) return null
  return (
    <div className="mx-overlay" style={{ left: rect.left, top: rect.top, width: rect.width, height: rect.height }} aria-hidden="true">
      {shown.map(item => {
        const [x, y, width, height] = item.box
        return <MediaBox key={item.key} tone={item.tone} label={item.label} below={y / natural.height < 0.08} style={{ left: `${(x / natural.width) * 100}%`, top: `${(y / natural.height) * 100}%`, width: `${(width / natural.width) * 100}%`, height: `${(height / natural.height) * 100}%` }} />
      })}
    </div>
  )
}
