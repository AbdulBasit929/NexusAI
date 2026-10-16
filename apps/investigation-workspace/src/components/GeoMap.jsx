import { useEffect, useMemo, useRef, useState } from 'react'
import * as maplibregl from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import { chartTheme } from './charts/chartTheme.js'
import { useThemeVersion } from '../lib/useThemeVersion.js'

// The map (roadmap 7.8.5, from the visual spike's map.js): MapLibre GL JS with the offline raster basemap that tools/tilecache serves, drawn muted so that the analysis
// layers stand out. With no basemap configured, or none reachable, the map is a plain background AND SAYS SO: a map is never left blank without a reason.
//
//   a cell site     a dot sized by its events, with a circle of its stated radius: a COVERAGE AREA, not the position of a person
//   a track         a line through the sites a subscriber used, in time order
//
// MapLibre needs its worker file served by us (no CDN): vite.config.js serves and emits /maplibre/maplibre-gl-worker.mjs and its shared module.
let workerSet = false
function ensureWorker() {
  if (workerSet) return
  workerSet = true
  try { maplibregl.setWorkerUrl(new URL('/maplibre/maplibre-gl-worker.mjs', globalThis.location.href).href) } catch { /* a test or a page without a base URL */ }
}

const circle = (lon, lat, metres, steps = 48) => {
  const ring = []
  for (let i = 0; i <= steps; i++) {
    const a = (i / steps) * 2 * Math.PI
    ring.push([lon + (metres * Math.cos(a)) / (111320 * Math.cos(lat * Math.PI / 180)), lat + (metres * Math.sin(a)) / 110574])
  }
  return ring
}
const collection = features => ({ type: 'FeatureCollection', features })

// The basemap setting comes from a git-ignored file next to the page (public/basemap.local.json: { "tiles": { "tilejson": "/tiles/pakistan.json", "attribution": "..." } }); the shared
// tile server's address is never in the repository and never reaches a browser: the browser asks this origin, and the tile cache answers.
export function useBasemap() {
  const [state, setState] = useState({ status: 'checking', tiles: null })
  useEffect(() => {
    const controller = new AbortController()
    fetch('/basemap.local.json', { cache: 'no-store', signal: controller.signal })
      .then(r => (r.ok && (r.headers.get('content-type') || '').includes('json') ? r.json() : null))
      .then(file => setState(file?.tiles?.tilejson || file?.tiles?.url ? { status: 'configured', tiles: file.tiles } : { status: 'off', tiles: null }))
      .catch(error => { if (error?.name !== 'AbortError') setState({ status: 'off', tiles: null }) })
    return () => controller.abort()
  }, [])
  return state
}

const BASEMAP_TEXT = {
  off: 'No basemap is configured, so the map shows a plain background. Start the tile cache with "npm run dev:tiles".',
  down: 'The tile cache did not answer, so the map shows a plain background.',
  ok: 'Offline basemap through the organisation tile cache.',
}

export function GeoMap({ sites = [], track = [], selectedId = null, onSelect, height = 480, label = 'Map', dim = 0.6 }) {
  const host = useRef(null)
  const map = useRef(null)
  const ready = useRef(false)
  const basemap = useBasemap()
  const themeVersion = useThemeVersion()
  const [status, setStatus] = useState('off')
  const positioned = useMemo(() => sites.filter(s => Number.isFinite(s.lat) && Number.isFinite(s.lon)), [sites])
  const handlers = useRef({})
  handlers.current = { onSelect }

  useEffect(() => {
    if (basemap.status === 'checking' || !host.current) return undefined
    ensureWorker()
    const theme = chartTheme()
    const sources = {}
    const layers = [{ id: 'bg', type: 'background', paint: { 'background-color': theme.card } }]
    if (basemap.tiles) {
      sources.base = basemap.tiles.tilejson ? { type: 'raster', url: basemap.tiles.tilejson, tileSize: basemap.tiles.tileSize || 256, attribution: basemap.tiles.attribution }
        : { type: 'raster', tiles: [basemap.tiles.url], tileSize: basemap.tiles.tileSize || 256, minzoom: basemap.tiles.minzoom, maxzoom: basemap.tiles.maxzoom, attribution: basemap.tiles.attribution }
      layers.push({ id: 'base', type: 'raster', source: 'base', paint: { 'raster-opacity': dim, 'raster-saturation': -0.4 } })
    }
    const instance = new maplibregl.Map({ container: host.current, style: { version: 8, sources, layers }, center: [69.3, 30.4], zoom: 4.5, attributionControl: false, dragRotate: false, maxZoom: 19 })
    map.current = instance
    instance.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-right')
    instance.addControl(new maplibregl.ScaleControl({ unit: 'metric' }), 'bottom-left')
    instance.addControl(new maplibregl.AttributionControl({ compact: true }), 'bottom-right')
    setStatus(basemap.tiles ? 'ok' : 'off')
    let errors = 0
    instance.on('error', event => { if (basemap.tiles && /tile|raster|fetch|Failed/i.test(String(event?.error?.message || '')) && ++errors > 3) setStatus('down') })
    instance.on('load', () => {
      ready.current = true
      instance.addSource('coverage', { type: 'geojson', data: collection([]) })
      instance.addSource('sites', { type: 'geojson', data: collection([]) })
      instance.addSource('track', { type: 'geojson', data: collection([]) })
      instance.addLayer({ id: 'coverage-fill', type: 'fill', source: 'coverage', paint: { 'fill-color': theme.data[0], 'fill-opacity': 0.12 } })
      instance.addLayer({ id: 'coverage-line', type: 'line', source: 'coverage', paint: { 'line-color': theme.data[0], 'line-opacity': 0.5, 'line-width': 1 } })
      instance.addLayer({ id: 'track-line', type: 'line', source: 'track', paint: { 'line-color': theme.data[1], 'line-width': 2.5, 'line-opacity': 0.9 } })
      instance.addLayer({ id: 'sites', type: 'circle', source: 'sites', paint: { 'circle-color': theme.data[0], 'circle-opacity': 0.9, 'circle-stroke-color': theme.card, 'circle-stroke-width': 1.5, 'circle-radius': ['interpolate', ['linear'], ['sqrt', ['get', 'value']], 0, 4, 8, 9, 40, 20] } })
      instance.addLayer({ id: 'sites-selected', type: 'circle', source: 'sites', filter: ['==', ['get', 'id'], ''], paint: { 'circle-color': 'rgba(0,0,0,0)', 'circle-stroke-color': theme.failed, 'circle-stroke-width': 3, 'circle-radius': 16 } })
      instance.on('click', 'sites', event => handlers.current.onSelect?.(event.features?.[0]?.properties?.id || null))
      instance.on('mouseenter', 'sites', () => { instance.getCanvas().style.cursor = 'pointer' })
      instance.on('mouseleave', 'sites', () => { instance.getCanvas().style.cursor = '' })
      instance.fire('nx-ready')
    })
    // the container can be laid out after the map is made (a lazy chunk, a switcher): follow its size, or the canvas keeps the default 300 pixels
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(() => instance.resize())
    observer?.observe(host.current)
    return () => { observer?.disconnect(); ready.current = false; instance.remove(); map.current = null }
  }, [basemap.status, basemap.tiles, dim, themeVersion])

  // data
  useEffect(() => {
    const instance = map.current
    if (!instance) return undefined
    const apply = () => {
      if (!instance.getSource('sites')) return
      instance.getSource('sites').setData(collection(positioned.map(s => ({ type: 'Feature', properties: { id: String(s.id), label: s.label || '', value: Number(s.value) || 1 }, geometry: { type: 'Point', coordinates: [s.lon, s.lat] } }))))
      instance.getSource('coverage').setData(collection(positioned.filter(s => s.radius > 0).map(s => ({ type: 'Feature', properties: { id: String(s.id) }, geometry: { type: 'Polygon', coordinates: [circle(s.lon, s.lat, s.radius)] } }))))
      const line = track.filter(p => Number.isFinite(p.lat) && Number.isFinite(p.lon)).map(p => [p.lon, p.lat])
      instance.getSource('track').setData(collection(line.length > 1 ? [{ type: 'Feature', properties: {}, geometry: { type: 'LineString', coordinates: line } }] : []))
      if (positioned.length) {
        const bounds = new maplibregl.LngLatBounds()
        for (const s of positioned) bounds.extend([s.lon, s.lat])
        // no closer than zoom 13 (about 20 m a pixel): a cell site is a coverage area, and a closer view shows less of the place and asks the tile cache for tiles it may not hold
        instance.fitBounds(bounds, { padding: 60, maxZoom: 13, duration: 0 })
      }
    }
    if (ready.current) apply(); else instance.once('nx-ready', apply)
    return () => { instance.off('nx-ready', apply) }
  }, [positioned, track, basemap.status, basemap.tiles, themeVersion])

  useEffect(() => {
    const instance = map.current
    if (!instance || !ready.current || !instance.getLayer('sites-selected')) return
    instance.setFilter('sites-selected', ['==', ['get', 'id'], selectedId ? String(selectedId) : ''])
    const s = positioned.find(p => String(p.id) === String(selectedId))
    if (s) instance.easeTo({ center: [s.lon, s.lat], duration: 350 })
  }, [selectedId, positioned, themeVersion])

  const summary = `${positioned.length} cell sites with a position${track.length > 1 ? `, and a track through ${track.length} of them` : ''}`
  return (
    <div className="geo-map" style={{ blockSize: height }}>
      <div ref={host} className="geo-map__canvas" role="application" tabIndex={0} aria-label={`${label}. ${summary}. The same sites are in the table beside the map.`} />
      <span className={`geo-map__status geo-map__status--${status}`} title={BASEMAP_TEXT[status]}>{status === 'ok' ? 'offline basemap' : status === 'down' ? 'basemap not reachable' : 'no basemap: plain background'}</span>
      {!positioned.length ? <p className="geo-map__empty" role="status">None of these cell sites has a position in the records, so there is nothing to draw. The table lists them.</p> : null}
    </div>
  )
}
