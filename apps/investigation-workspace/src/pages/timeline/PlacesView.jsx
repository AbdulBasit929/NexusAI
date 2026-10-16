import { lazy, Suspense, useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { MessageSquareText, Pause, Play } from 'lucide-react'
import { SkeletonRows } from '../../components/Skeleton.jsx'
import { EmptyState } from '../../components/AnalystComponents.jsx'
import { CarrierBadge } from '../../components/CarrierBadge.jsx'
import { getPlaces } from '../../lib/apiClient.js'
import { carrierFor, formatMobile } from '../../lib/carriers.js'
import { formatNumber } from '../../lib/format.js'

// The map is fetched only when Places is shown.
const GeoMap = lazy(() => import('../../components/GeoMap.jsx').then(m => ({ default: m.GeoMap })))

const stamp = hour => Date.parse(String(hour).replace(' ', 'T') + (/[zZ]|[+-]\d\d:?\d\d$/.test(String(hour)) ? '' : 'Z'))
const when = ms => new Date(ms).toISOString().replace('T', ' ').slice(0, 16)

function usePlaces(caseId) {
  const [state, setState] = useState({ status: 'loading', data: null, error: null })
  useEffect(() => {
    const controller = new AbortController()
    setState({ status: 'loading', data: null, error: null })
    getPlaces({ caseId, signal: controller.signal })
      .then(data => setState({ status: data?.available ? 'ready' : 'unavailable', data, error: null }))
      .catch(error => { if (error?.name !== 'AbortError') setState({ status: 'failed', data: null, error }) })
    return () => controller.abort()
  }, [caseId])
  return state
}

// Places (roadmap 7.8.3 row 3): where the subscribers' phones were, as the cell sites say, on a map with the offline basemap. A subscriber's movement is a line through the sites it
// used in time order, played along a time slider. A cell site is a COVERAGE AREA, not the position of a person, and the page says so beside every map.
export function PlacesView({ caseId }) {
  const state = usePlaces(caseId)
  const [follow, setFollow] = useState('')
  const [selected, setSelected] = useState(null)
  const [step, setStep] = useState(null)
  const [playing, setPlaying] = useState(false)
  const timer = useRef(null)

  const data = state.data
  const sitesById = useMemo(() => new Map((data?.sites || []).map(s => [s.cell_key, s])), [data])
  // the subscribers, those with positioned visits first: a subscriber whose cells have no position cannot be followed on a map
  const subjects = useMemo(() => {
    const counts = new Map()
    for (const v of data?.visits || []) {
      const site = sitesById.get(v.cell_key)
      const entry = counts.get(v.subject) || { events: 0, located: 0 }
      entry.events += v.events
      if (site && site.lat != null && site.lon != null) entry.located += v.events
      counts.set(v.subject, entry)
    }
    return [...counts].sort((a, b) => b[1].located - a[1].located || b[1].events - a[1].events).map(([number, c]) => ({ number, events: c.events, located: c.located }))
  }, [data, sitesById])
  useEffect(() => { if (!follow && subjects.length) setFollow(subjects[0].number) }, [subjects, follow])

  const visits = useMemo(() => (data?.visits || []).filter(v => v.subject === follow).sort((a, b) => stamp(a.hour) - stamp(b.hour)), [data, follow])
  const positioned = useMemo(() => visits.filter(v => { const s = sitesById.get(v.cell_key); return s && s.lat != null && s.lon != null }), [visits, sitesById])
  useEffect(() => { setStep(null); setPlaying(false) }, [follow])
  const last = positioned.length
  const upTo = step == null ? last : Math.min(step, last)
  const track = useMemo(() => {
    const out = []
    for (const v of positioned.slice(0, upTo)) {
      const s = sitesById.get(v.cell_key)
      if (!out.length || out.at(-1).cell_key !== v.cell_key) out.push({ cell_key: v.cell_key, lat: s.lat, lon: s.lon })
    }
    return out
  }, [positioned, upTo, sitesById])

  useEffect(() => {
    if (!playing) return undefined
    timer.current = globalThis.setInterval(() => setStep(previous => { const next = (previous == null || previous >= last ? 0 : previous) + Math.max(1, Math.round(last / 120)); if (next >= last) { setPlaying(false); return last } return next }), 120)
    return () => globalThis.clearInterval(timer.current)
  }, [playing, last])

  const sites = useMemo(() => (data?.sites || []).map(s => ({ id: s.cell_key, lat: s.lat, lon: s.lon, label: s.name || s.cell_key, value: s.events, radius: s.radius_m || 0 })), [data])
  const detail = selected ? sitesById.get(selected) : null
  const carrier = follow ? carrierFor({ number: follow }) : null

  if (state.status === 'loading') return <SkeletonRows rows={6} label="Reading the places of this case" />
  if (state.status === 'failed') return <EmptyState kind="unavailable" label="The places could not be read" description={`The request failed (${state.error?.reference || 'no reference'}). Nothing was changed.`} />
  if (state.status === 'unavailable') return <EmptyState kind="unavailable" label="This case cannot be drawn on a map yet" description={`${data?.reason || 'The case has no typed call records.'} A map over part of the records would look like a finding and is not shown.`} />
  const withPosition = data.totals?.sites_with_position || 0
  return (
    <div className="pl">
      <div className="pl__bar">
        <label className="pl__field"><span>Follow</span>
          <select value={follow} onChange={event => setFollow(event.target.value)} aria-label="Subscriber to follow">
            {subjects.map(s => <option key={s.number} value={s.number}>{formatMobile(s.number)} · {formatNumber(s.events)} events, {s.located ? `${formatNumber(s.located)} on a located cell` : 'no located cell'}</option>)}
          </select>
        </label>
        {carrier ? <CarrierBadge carrier={carrier.id} size={18} showName how={carrier.how} /> : null}
        <div className="pl__time">
          <button type="button" className="cx-button" onClick={() => setPlaying(value => !value)} disabled={last < 2} aria-pressed={playing}>{playing ? <Pause aria-hidden="true" /> : <Play aria-hidden="true" />}{playing ? 'Pause' : 'Play'}</button>
          <input type="range" min="0" max={last} value={upTo} onChange={event => { setPlaying(false); setStep(Number(event.target.value)) }} aria-label="Time along the movement" disabled={last < 2} />
          <output className="pl__clock">{upTo > 0 ? when(stamp(positioned[Math.min(upTo, last) - 1].hour)) : last ? 'start' : 'no positioned visits'}</output>
        </div>
      </div>
      <div className="pl__split">
        <figure className="pl__map">
          <Suspense fallback={<p className="chart-card__loading" role="status">Loading the map…</p>}>
            <GeoMap sites={sites} track={track} selectedId={selected} onSelect={setSelected} height={560} label="Map of the cell sites of this case" />
          </Suspense>
          <figcaption>{formatNumber(withPosition)} of {formatNumber(data.totals?.sites || 0)} cell sites have a position in the records. {data.caveats?.join(' ')}</figcaption>
        </figure>
        <aside className="cx-panel" aria-label="Cell sites" aria-live="polite">
          {detail ? (
            <>
              <h2>{detail.name || detail.cell_key}</h2>
              <dl className="cx-facts">
                <div><dt>Events</dt><dd>{formatNumber(detail.events)}</dd></div>
                <div><dt>Subscribers</dt><dd>{formatNumber(detail.visitors)}</dd></div>
                <div><dt>Position</dt><dd>{detail.lat != null ? `${detail.lat.toFixed(5)}, ${detail.lon.toFixed(5)}` : 'not in the records'}</dd></div>
                <div><dt>Stated radius</dt><dd>{detail.radius_m ? `${formatNumber(detail.radius_m)} m` : 'not stated'}</dd></div>
              </dl>
              <p className="cx-muted">Cell key {detail.cell_key}</p>
              <div className="cx-actions">
                <Link className="cx-button" to={`/cases/${encodeURIComponent(caseId)}/investigate?${new URLSearchParams({ question: `Which cell sites did ${follow} use most? Give the top five.` })}`}><MessageSquareText aria-hidden="true" />Ask about this subscriber's cells</Link>
                <button type="button" className="cx-button" onClick={() => setSelected(null)}>Clear</button>
              </div>
            </>
          ) : (
            <>
              <h2>Busiest cell sites</h2>
              <ul className="cx-list">
                {[...(data.sites || [])].sort((a, b) => b.events - a.events).slice(0, 12).map(s => (
                  <li key={s.cell_key}><button type="button" className="cx-numchip" onClick={() => setSelected(s.cell_key)}>{s.name || s.cell_key}</button><span>{formatNumber(s.events)} events{s.lat == null ? ' · no position' : ''}</span></li>
                ))}
              </ul>
              <p className="cx-muted">Select a site on the map or in this list. The line is the movement of the subscriber chosen above, through the sites it used, in time order.</p>
            </>
          )}
        </aside>
      </div>
    </div>
  )
}
