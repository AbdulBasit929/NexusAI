import { useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Download, Search } from 'lucide-react'
import { CaseShell } from '../components/CaseShell.jsx'
import { Card } from '../components/Card.jsx'
import { PageHeader } from '../components/PageHeader.jsx'
import { formatNumber } from '../lib/format.js'

// The data inventory of a case: every file, number, tower and internet record the files hold, as the typed tables read them.
//
// The inventory is a local file built from the database by tools/real_data/build_inventory_real.py and served by this dev server from OUTSIDE the repository
// (vite.config.js, /real-data/...), because it holds real values that must never be committed. A case without one shows a plain statement, not an error.
// Written for the owner's request of 2026-10-14: all data viewable and extractable, with the names the files give, towers included.

const TABS = [
  ['overview', 'Overview'], ['files', 'Files'], ['subscribers', 'Subscribers'], ['towers', 'Towers'], ['internet', 'Internet records'], ['downloads', 'Downloads'],
]

const DOWNLOADS = [
  ['files', 'Files', 'Every file, its layout, row count, period and the columns it holds'],
  ['subscribers', 'Subscribers', 'The number each call-record file belongs to, with counts and period'],
  ['towers', 'Towers', 'Every cell: the name the files give it, position, traffic and numbers'],
  ['devices', 'Devices', 'Handset (IMEI) and SIM (IMSI) pairings with first and last time'],
  ['call_records', 'Call records', 'Every call and SMS of the subscribers, one row each'],
  ['tower_dump_rows', 'Tower dump rows', 'Every row of the tower dump files'],
  ['internet_sessions', 'Internet sessions', 'Every internet record, with addresses, ports, applications and volumes'],
  ['app_usage', 'Application usage', 'Every application usage record'],
  ['numbers_seen', 'All numbers seen', 'Every number in any file, with its role and event count'],
]

const n = value => (value === null || value === undefined || value === '' ? '—' : typeof value === 'number' ? formatNumber(value) : String(value))
const seconds = value => {
  const total = Number(value || 0)
  if (total < 3600) return `${Math.floor(total / 60)} min`
  return `${(total / 3600).toFixed(1)} h`
}

function useInventory(caseId) {
  const [state, setState] = useState({ status: 'loading', data: null })
  useEffect(() => {
    const controller = new AbortController()
    setState({ status: 'loading', data: null })
    fetch('/real-data/inventory.json', { signal: controller.signal, cache: 'no-store' })
      .then(response => {
        if (!response.ok || !(response.headers.get('content-type') || '').includes('json')) return null
        return response.json()
      })
      .then(data => setState(data && data.collection === caseId ? { status: 'ready', data } : { status: 'none', data: null }))
      .catch(error => { if (error?.name !== 'AbortError') setState({ status: 'none', data: null }) })
    return () => controller.abort()
  }, [caseId])
  return state
}

// A table that searches and sorts in the browser. `rows` are plain objects; `columns` say how to show them.
function DataTable({ columns, rows, searchKeys, initialSort, caption, onSelect, selectedKey, rowKey, pageSize = 100 }) {
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState(initialSort || { key: columns[0].key, dir: 'desc' })
  const [shown, setShown] = useState(pageSize)
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    const hit = q ? rows.filter(row => searchKeys.some(key => String(row[key] ?? '').toLowerCase().includes(q))) : rows
    const dir = sort.dir === 'asc' ? 1 : -1
    return [...hit].sort((a, b) => {
      const x = a[sort.key], y = b[sort.key]
      if (x === y) return 0
      if (x === null || x === undefined) return 1
      if (y === null || y === undefined) return -1
      return (typeof x === 'number' && typeof y === 'number' ? x - y : String(x).localeCompare(String(y))) * dir
    })
  }, [rows, query, sort, searchKeys])
  useEffect(() => { setShown(pageSize) }, [query, sort, pageSize])
  return (
    <div className="dp-table">
      <div className="dp-table__toolbar">
        <label>
          <span className="visually-hidden">Search {caption}</span>
          <Search aria-hidden="true" />
          <input type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder={`Search ${caption}`} />
        </label>
        <p aria-live="polite">{formatNumber(filtered.length)} of {formatNumber(rows.length)} {caption}</p>
      </div>
      <div className="dp-table__scroll">
        <table>
          <caption className="visually-hidden">{caption}</caption>
          <thead>
            <tr>
              {columns.map(column => (
                <th key={column.key} scope="col" className={column.numeric ? 'is-numeric' : ''} aria-sort={sort.key === column.key ? (sort.dir === 'asc' ? 'ascending' : 'descending') : 'none'}>
                  <button type="button" onClick={() => setSort(sort.key === column.key ? { key: column.key, dir: sort.dir === 'asc' ? 'desc' : 'asc' } : { key: column.key, dir: column.numeric ? 'desc' : 'asc' })}>{column.label}</button>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {filtered.slice(0, shown).map(row => {
              const key = rowKey(row)
              return (
                <tr key={key} className={selectedKey === key ? 'is-selected' : ''} onClick={onSelect ? () => onSelect(row) : undefined} tabIndex={onSelect ? 0 : undefined}
                  onKeyDown={onSelect ? event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onSelect(row) } } : undefined} aria-selected={onSelect ? selectedKey === key : undefined}>
                  {columns.map(column => <td key={column.key} className={column.numeric ? 'is-numeric' : ''}>{column.render ? column.render(row) : n(row[column.key])}</td>)}
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
      {filtered.length > shown ? <button type="button" className="dp-more" onClick={() => setShown(shown + pageSize * 2)}>Show more ({formatNumber(filtered.length - shown)} left)</button> : null}
    </div>
  )
}

// Towers on a plain map: no library, no tiles, so it works offline. Positions are the cell sites' own coordinates; a cell is not the position of a person.
function TowerMap({ towers, selected, onSelect }) {
  const placed = towers.filter(t => t.lat !== null && t.lon !== null)
  if (!placed.length) return <p className="dp-muted">None of the towers has a position in the files.</p>
  const lats = placed.map(t => t.lat), lons = placed.map(t => t.lon)
  const [minLat, maxLat, minLon, maxLon] = [Math.min(...lats), Math.max(...lats), Math.min(...lons), Math.max(...lons)]
  const kx = Math.cos(((minLat + maxLat) / 2) * Math.PI / 180)
  const width = 760, height = 420, pad = 24
  const spanX = Math.max((maxLon - minLon) * kx, 1e-6), spanY = Math.max(maxLat - minLat, 1e-6)
  const scale = Math.min((width - 2 * pad) / spanX, (height - 2 * pad) / spanY)
  const px = t => pad + ((t.lon - minLon) * kx) * scale
  const py = t => height - pad - (t.lat - minLat) * scale
  const top = Math.max(...placed.map(t => t.events))
  return (
    <figure className="dp-map">
      <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label={`Map of ${placed.length} towers with positions; larger circles carry more events`}>
        <rect width={width} height={height} className="dp-map__bg" />
        {placed.map(t => (
          <circle key={t.cell_key} cx={px(t)} cy={py(t)} r={3 + 9 * Math.sqrt(t.events / top)} className={`dp-map__dot ${t.dump_events > t.call_events ? 'is-dump' : 'is-call'}${selected === t.cell_key ? ' is-selected' : ''}`}
            onClick={() => onSelect(t)}><title>{`${t.name || t.cell_key}: ${formatNumber(t.events)} events, ${formatNumber(t.numbers)} numbers`}</title></circle>
        ))}
      </svg>
      <figcaption>
        <span className="dp-key dp-key--call" /> mostly call records <span className="dp-key dp-key--dump" /> mostly tower dump · circle size = events · north is up. A cell is not the position of a person.
        {placed.length < towers.length ? ` ${formatNumber(towers.length - placed.length)} towers have no position in the files and are only in the table.` : ''}
      </figcaption>
    </figure>
  )
}

function Kpi({ label, value, note }) {
  return <div className="dp-kpi"><strong>{value}</strong><span>{label}</span>{note ? <small>{note}</small> : null}</div>
}

export default function DataPage() {
  const { id: caseId } = useParams()
  const { status, data } = useInventory(caseId)
  const [tab, setTab] = useState('overview')
  const [subscriber, setSubscriber] = useState(null)
  const [tower, setTower] = useState(null)

  const totals = useMemo(() => {
    if (!data) return null
    const rows = data.files.reduce((sum, f) => sum + f.rows, 0)
    return { files: data.files.length, rows, subscribers: data.subscribers.length, towers: data.towers_total, sessions: data.sessions.total }
  }, [data])

  const header = <PageHeader title="Data" description="Everything the files hold: numbers, towers, devices, internet records, with the names the files give them." breadcrumbs={[{ label: 'Data' }]} />

  if (status !== 'ready') {
    return (
      <CaseShell caseId={caseId}>
        <main id="workspace-main" className="dp" tabIndex={-1}>
          {header}
          <Card title={status === 'loading' ? 'Loading the inventory' : 'No data inventory for this case'}>
            <p>{status === 'loading' ? 'Reading the inventory from this machine.' : 'An inventory is built locally for cases whose files may be shown here. This case has none.'}</p>
          </Card>
        </main>
      </CaseShell>
    )
  }

  const fileColumns = [
    { key: 'id', label: 'ID' }, { key: 'name', label: 'File' }, { key: 'layout_label', label: 'Format', render: f => <span title={f.layout}>{f.layout_label || f.layout}</span> },
    { key: 'rows', label: 'Rows', numeric: true }, { key: 'repeats', label: 'Repeats', numeric: true }, { key: 'subscribers', label: 'Numbers', numeric: true },
    { key: 'first', label: 'From' }, { key: 'last', label: 'To' },
    { key: 'columns', label: 'Columns', numeric: true, render: f => <details><summary>{f.columns.length}</summary><p className="dp-columns">{f.columns.join(' · ')}</p></details> },
  ]
  const subscriberColumns = [
    { key: 'number', label: 'Number' }, { key: 'label', label: 'Format' }, { key: 'events', label: 'Events', numeric: true }, { key: 'calls', label: 'Calls', numeric: true }, { key: 'sms', label: 'SMS', numeric: true },
    { key: 'outgoing', label: 'Outgoing', numeric: true }, { key: 'incoming', label: 'Incoming', numeric: true }, { key: 'counterparties', label: 'Other parties', numeric: true },
    { key: 'total_duration_s', label: 'Talk time', numeric: true, render: s => seconds(s.total_duration_s) }, { key: 'cells', label: 'Cells', numeric: true }, { key: 'first', label: 'From' }, { key: 'last', label: 'To' },
  ]
  const towerColumns = [
    { key: 'name', label: 'Name in the files' }, { key: 'cell_key', label: 'Cell key' }, { key: 'lat', label: 'Latitude', numeric: true, render: t => (t.lat === null ? '—' : t.lat.toFixed(5)) },
    { key: 'lon', label: 'Longitude', numeric: true, render: t => (t.lon === null ? '—' : t.lon.toFixed(5)) }, { key: 'events', label: 'Events', numeric: true },
    { key: 'call_events', label: 'Call', numeric: true }, { key: 'dump_events', label: 'Dump', numeric: true }, { key: 'numbers', label: 'Numbers', numeric: true }, { key: 'files', label: 'Files', numeric: true },
    { key: 'first', label: 'From' }, { key: 'last', label: 'To' },
  ]

  return (
    <CaseShell caseId={caseId}>
      <main id="workspace-main" className="dp" tabIndex={-1}>
        {header}
        <p className="dp-banner" role="note">Real data supplied for testing. Shown on this machine only, to the team lead who supplied it; built {data.generated_utc} UTC.</p>
        <div className="dp-tabs" role="tablist" aria-label="Data sections">
          {TABS.map(([key, label]) => <button key={key} type="button" role="tab" id={`dp-tab-${key}`} aria-selected={tab === key} aria-controls={`dp-panel-${key}`} tabIndex={tab === key ? 0 : -1} onClick={() => setTab(key)}
            onKeyDown={event => { const i = TABS.findIndex(([k]) => k === tab); if (event.key === 'ArrowRight') setTab(TABS[(i + 1) % TABS.length][0]); if (event.key === 'ArrowLeft') setTab(TABS[(i + TABS.length - 1) % TABS.length][0]) }}>{label}</button>)}
        </div>

        <div role="tabpanel" id={`dp-panel-${tab}`} aria-labelledby={`dp-tab-${tab}`} className="dp-panel">
          {tab === 'overview' && (
            <>
              <div className="dp-kpis">
                <Kpi label="files" value={formatNumber(totals.files)} /><Kpi label="rows read" value={formatNumber(totals.rows)} />
                <Kpi label="subscribers" value={formatNumber(totals.subscribers)} note="call-record files" /><Kpi label="towers" value={formatNumber(totals.towers)} />
                <Kpi label="internet records" value={formatNumber(totals.sessions)} /><Kpi label="applications" value={formatNumber(data.app_usage.length)} />
              </div>
              <Card title="Towers on the map" description="Every tower that has a position in the files. Select one to see it below.">
                <TowerMap towers={data.towers} selected={tower?.cell_key} onSelect={setTower} />
                {tower ? <dl className="dp-detail"><dt>Tower</dt><dd>{tower.name || tower.cell_key}</dd><dt>Cell key</dt><dd>{tower.cell_key}</dd><dt>Events</dt><dd>{n(tower.events)} ({n(tower.call_events)} call, {n(tower.dump_events)} dump)</dd><dt>Numbers</dt><dd>{n(tower.numbers)}</dd><dt>Period</dt><dd>{n(tower.first)} to {n(tower.last)}</dd></dl> : null}
              </Card>
              <Card title="Where to go" description="The link chart, the map with movement and tower dumps, device timelines and daily activity are in the visual spike, which opens on this same data.">
                <p><a href="http://127.0.0.1:5200/" target="_blank" rel="noreferrer">Open the link chart and map</a> (the address is the one the spike was started on; its header says if it moved).</p>
              </Card>
            </>
          )}
          {tab === 'files' && <Card title="Files" description="Each file as the system read it: its format, how many rows, the period it covers and the columns it holds."><DataTable caption="files" columns={fileColumns} rows={data.files} searchKeys={['id', 'name', 'layout', 'layout_label', 'family']} initialSort={{ key: 'id', dir: 'asc' }} rowKey={f => f.id} /></Card>}
          {tab === 'subscribers' && (
            <>
              <Card title="Subscribers" description="The number each call-record file belongs to. Select one for its most frequent other parties and its devices.">
                <DataTable caption="subscribers" columns={subscriberColumns} rows={data.subscribers} searchKeys={['number', 'label']} initialSort={{ key: 'events', dir: 'desc' }} rowKey={s => s.number} selectedKey={subscriber?.number} onSelect={setSubscriber} />
              </Card>
              {subscriber ? (
                <>
                  <Card title={`Most frequent other parties of ${subscriber.number}`} description="Counts are events (calls and SMS) in this subscriber's own file.">
                    <DataTable caption="other parties" rows={subscriber.top_counterparties} searchKeys={['number', 'kind']} initialSort={{ key: 'events', dir: 'desc' }} rowKey={c => c.number}
                      columns={[{ key: 'number', label: 'Number or name' }, { key: 'kind', label: 'Class' }, { key: 'events', label: 'Events', numeric: true }, { key: 'outgoing', label: 'Outgoing', numeric: true }, { key: 'incoming', label: 'Incoming', numeric: true },
                        { key: 'duration_s', label: 'Talk time', numeric: true, render: c => seconds(c.duration_s) }, { key: 'first', label: 'First' }, { key: 'last', label: 'Last' }]} />
                  </Card>
                  <Card title={`Devices seen with ${subscriber.number}`} description="Handset (IMEI) and SIM (IMSI) pairings in order of first use. A change is a new handset or SIM.">
                    {subscriber.devices.length ? <DataTable caption="devices" rows={subscriber.devices} searchKeys={['imsi', 'imei']} initialSort={{ key: 'first', dir: 'asc' }} rowKey={d => `${d.imsi}-${d.imei}`}
                      columns={[{ key: 'imsi', label: 'SIM (IMSI)' }, { key: 'imei', label: 'Handset (IMEI)' }, { key: 'events', label: 'Events', numeric: true }, { key: 'first', label: 'First' }, { key: 'last', label: 'Last' }]} /> : <p className="dp-muted">This file records no handset or SIM.</p>}
                  </Card>
                </>
              ) : null}
            </>
          )}
          {tab === 'towers' && (
            <>
              <Card title="Towers" description={`All ${formatNumber(data.towers_total)} cells seen in any file, with the name the files give them. Select one to mark it on the map.`}>
                <TowerMap towers={data.towers} selected={tower?.cell_key} onSelect={setTower} />
                <DataTable caption="towers" columns={towerColumns} rows={data.towers} searchKeys={['name', 'cell_key']} initialSort={{ key: 'events', dir: 'desc' }} rowKey={t => t.cell_key} selectedKey={tower?.cell_key} onSelect={setTower} />
              </Card>
              {tower ? <Card title={tower.name || tower.cell_key}><dl className="dp-detail"><dt>Cell key</dt><dd>{tower.cell_key}</dd><dt>Other names</dt><dd>{tower.names_seen.slice(1).join(' · ') || '—'}</dd><dt>Position</dt><dd>{tower.lat === null ? 'not in the files' : `${tower.lat.toFixed(5)}, ${tower.lon.toFixed(5)}`}</dd><dt>Events</dt><dd>{n(tower.events)}</dd><dt>Numbers</dt><dd>{n(tower.numbers)}</dd><dt>Files</dt><dd>{n(tower.files)}</dd><dt>Period</dt><dd>{n(tower.first)} to {n(tower.last)}</dd></dl></Card> : null}
            </>
          )}
          {tab === 'internet' && (
            <>
              <Card title="Internet subscribers" description="The numbers in the internet-record files, with sessions, public addresses and volumes.">
                <DataTable caption="internet subscribers" rows={data.sessions.subscribers} searchKeys={['number', 'layout']} initialSort={{ key: 'sessions', dir: 'desc' }} rowKey={s => s.number}
                  columns={[{ key: 'number', label: 'Number' }, { key: 'sessions', label: 'Sessions', numeric: true }, { key: 'public_addresses', label: 'Public addresses', numeric: true }, { key: 'bytes_up', label: 'Bytes up', numeric: true }, { key: 'bytes_down', label: 'Bytes down', numeric: true }, { key: 'first', label: 'From' }, { key: 'last', label: 'To' }]} />
              </Card>
              <Card title="Public addresses" description="Addresses shared by more than one subscriber are what address attribution has to separate by time and port.">
                <DataTable caption="public addresses" rows={data.sessions.top_public_addresses} searchKeys={['address']} initialSort={{ key: 'sessions', dir: 'desc' }} rowKey={a => a.address}
                  columns={[{ key: 'address', label: 'Address' }, { key: 'sessions', label: 'Sessions', numeric: true }, { key: 'subscribers', label: 'Subscribers', numeric: true }, { key: 'port_from', label: 'Lowest port', numeric: true }, { key: 'port_to', label: 'Highest port', numeric: true }]} />
              </Card>
              <Card title="Applications and hosts">
                <DataTable caption="applications" rows={data.sessions.top_applications} searchKeys={['application']} initialSort={{ key: 'sessions', dir: 'desc' }} rowKey={a => a.application} columns={[{ key: 'application', label: 'Application' }, { key: 'sessions', label: 'Sessions', numeric: true }]} />
                <DataTable caption="hosts" rows={data.sessions.top_hosts} searchKeys={['host']} initialSort={{ key: 'sessions', dir: 'desc' }} rowKey={h => h.host} columns={[{ key: 'host', label: 'Host' }, { key: 'sessions', label: 'Sessions', numeric: true }]} />
                <DataTable caption="application usage records" rows={data.app_usage} searchKeys={['application', 'category']} initialSort={{ key: 'records', dir: 'desc' }} rowKey={a => `${a.application}-${a.category}`} columns={[{ key: 'application', label: 'Application' }, { key: 'category', label: 'Category' }, { key: 'records', label: 'Records', numeric: true }, { key: 'numbers', label: 'Numbers', numeric: true }]} />
              </Card>
            </>
          )}
          {tab === 'downloads' && (
            <Card title="Extract the data" description="Each table as a spreadsheet file (CSV, UTF-8). They are copies of what is on this page and of the complete records, made on this machine.">
              <ul className="dp-downloads">
                {DOWNLOADS.map(([key, label, text]) => (
                  <li key={key}><a href={`/real-data/${key}.csv`} download={`${key}.csv`}><Download aria-hidden="true" /> <strong>{label}</strong></a><span>{text}</span></li>
                ))}
              </ul>
            </Card>
          )}
        </div>
      </main>
    </CaseShell>
  )
}
