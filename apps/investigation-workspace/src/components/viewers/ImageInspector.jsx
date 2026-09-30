import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Eye, EyeOff, Maximize, RotateCw, ScanSearch, ZoomIn, ZoomOut } from 'lucide-react'
import { EvidenceStrengthBadge } from '../Citations.jsx'
import { imageRegions } from '../../lib/viewerPresentation.js'
import { strength } from '../../lib/viewerStrength.js'
import { clampZoom, fitScale, focusRegion, normalizeBox } from '../../lib/viewerTools.js'

function parsed(value) { if (!value) return null; try { return JSON.parse(value) } catch { return null } }

function boxStyle(fraction) {
  const [x, y, width, height] = fraction
  return { insetInlineStart: `${x * 100}%`, insetBlockStart: `${y * 100}%`, inlineSize: `${width * 100}%`, blockSize: `${height * 100}%` }
}

// The image as something to inspect: zoom (buttons, wheel, keys), drag to pan, rotate, fit; the regions the service read are
// drawn over it and listed beside it, and choosing one brings it to the middle of the frame.
export function ImageInspector({ detail, objectUrl, citedBBox }) {
  const regions = useMemo(() => imageRegions(detail, parsed(citedBBox)), [detail, citedBBox])
  const [overlays, setOverlays] = useState(regions.length > 0)
  const [natural, setNatural] = useState(null)
  const [frame, setFrame] = useState(null)
  const [view, setView] = useState({ zoom: 1, x: 0, y: 0, turn: 0 })
  const [chosen, setChosen] = useState(null)
  const stage = useRef(null)
  const drag = useRef(null)

  useEffect(() => {
    const node = stage.current
    if (!node) return undefined
    const measure = () => setFrame({ width: node.clientWidth, height: node.clientHeight })
    measure()
    if (!globalThis.ResizeObserver) return undefined
    const observer = new globalThis.ResizeObserver(measure)
    observer.observe(node)
    return () => observer.disconnect()
  }, [])

  const scale = natural && frame ? fitScale({ width: frame.width - 24, height: frame.height - 24 }, natural) : 1
  const layer = natural ? { width: natural.width * scale, height: natural.height * scale } : null
  const zoomBy = useCallback(factor => setView(value => ({ ...value, zoom: clampZoom(value.zoom * factor) })), [])
  const reset = () => { setChosen(null); setView({ zoom: 1, x: 0, y: 0, turn: 0 }) }

  function focus(region) {
    const fraction = normalizeBox(region.bbox, natural)
    const target = fraction && layer && frame ? focusRegion(fraction, layer, frame) : null
    setChosen(region.id)
    if (target) setView({ zoom: target.zoom, x: target.x, y: target.y, turn: 0 })
  }
  const citedRegion = regions.find(region => region.cited)
  useEffect(() => { if (citedRegion && layer && frame) focus(citedRegion) /* eslint-disable-next-line react-hooks/exhaustive-deps */ }, [citedRegion?.id, natural?.width, frame?.width])

  useEffect(() => {
    const node = stage.current
    if (!node) return undefined
    const onWheel = event => { event.preventDefault(); zoomBy(event.deltaY < 0 ? 1.15 : 1 / 1.15) }
    node.addEventListener('wheel', onWheel, { passive: false })
    return () => node.removeEventListener('wheel', onWheel)
  }, [zoomBy])

  function onKey(event) {
    const pan = 40
    if (event.key === '+' || event.key === '=') zoomBy(1.25)
    else if (event.key === '-') zoomBy(0.8)
    else if (event.key === '0') reset()
    else if (event.key.toLowerCase() === 'r') setView(value => ({ ...value, turn: (value.turn + 90) % 360 }))
    else if (event.key === 'ArrowLeft') setView(value => ({ ...value, x: value.x + pan }))
    else if (event.key === 'ArrowRight') setView(value => ({ ...value, x: value.x - pan }))
    else if (event.key === 'ArrowUp') setView(value => ({ ...value, y: value.y + pan }))
    else if (event.key === 'ArrowDown') setView(value => ({ ...value, y: value.y - pan }))
    else return
    event.preventDefault()
  }
  const down = event => { drag.current = { x: event.clientX - view.x, y: event.clientY - view.y }; event.currentTarget.setPointerCapture?.(event.pointerId) }
  const move = event => { if (drag.current) setView(value => ({ ...value, x: event.clientX - drag.current.x, y: event.clientY - drag.current.y })) }
  const up = () => { drag.current = null }

  return (
    <section className="source-viewer image-viewer ii" aria-labelledby="image-viewer-heading">
      <header>
        <h2 id="image-viewer-heading">Image source</h2>
        <div className="ii__tools" role="toolbar" aria-label="Image tools">
          <button type="button" onClick={() => zoomBy(0.8)} aria-label="Zoom out"><ZoomOut aria-hidden="true" /></button>
          <output className="ii__zoom" aria-live="polite">{Math.round(view.zoom * scale * 100)}%</output>
          <button type="button" onClick={() => zoomBy(1.25)} aria-label="Zoom in"><ZoomIn aria-hidden="true" /></button>
          <button type="button" onClick={() => setView(value => ({ ...value, turn: (value.turn + 90) % 360 }))} aria-label="Rotate 90 degrees"><RotateCw aria-hidden="true" /></button>
          <button type="button" onClick={reset} aria-label="Fit to view"><Maximize aria-hidden="true" /></button>
          {regions.length ? <button type="button" className="ii__toggle" aria-pressed={overlays} onClick={() => setOverlays(value => !value)}>{overlays ? <EyeOff aria-hidden="true" /> : <Eye aria-hidden="true" />}{overlays ? 'Hide observation overlays' : 'Show observation overlays'}</button> : null}
        </div>
      </header>
      <div className="ii__layout">
        <div ref={stage} className="ii__stage" tabIndex={0} role="group" aria-label="Image. Plus and minus zoom, arrow keys move, R rotates, 0 fits." onKeyDown={onKey} onPointerDown={down} onPointerMove={move} onPointerUp={up} onPointerCancel={up} onDoubleClick={reset}>
          <div className="ii__layer" style={layer ? { inlineSize: layer.width, blockSize: layer.height, transform: `translate(-50%, -50%) translate(${view.x}px, ${view.y}px) rotate(${view.turn}deg) scale(${view.zoom})` } : { visibility: 'hidden' }}>
            <img src={objectUrl} alt={`Source ${detail.item?.original_filename || 'image'}`} draggable="false" onLoad={event => setNatural({ width: event.currentTarget.naturalWidth || 1, height: event.currentTarget.naturalHeight || 1 })} />
            {overlays && natural ? regions.map(region => {
              const fraction = normalizeBox(region.bbox, natural)
              return fraction ? <span key={region.id} className={`evidence-overlay${region.cited ? ' evidence-overlay--cited arrival-highlight' : ''}${chosen === region.id ? ' is-chosen' : ''}`} style={boxStyle(fraction)}><span>{region.label}</span>{region.confidence == null ? null : <small>{Math.round(region.confidence * 100)}%</small>}</span> : null
            }) : null}
          </div>
        </div>
        {regions.length ? (
          <ul className="overlay-key ii__regions" aria-label="Regions read from the image">
            {regions.map(region => (
              <li key={region.id} className={chosen === region.id ? 'is-chosen' : undefined}>
                <button type="button" onClick={() => focus(region)} aria-label={`Show region ${region.label}`}><ScanSearch aria-hidden="true" /><span>{region.label}</span></button>
                <EvidenceStrengthBadge strength={strength(region.confidence, region.cited)} />
              </li>
            ))}
          </ul>
        ) : <p className="viewer-gap">No OCR or plate regions were supplied with this evidence.</p>}
      </div>
    </section>
  )
}
