import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useOutletContext, useParams } from 'react-router-dom'
import PageHeader from '../components/PageHeader'
import ConfirmDialog from '../components/ConfirmDialog'
import EmptyState from '../components/EmptyState'
import { agentCollectionsApi, agentsApi, modelsApi, recordsApi } from '../utils/api'
import { useActiveCase } from '../contexts/ActiveCaseContext'

const recordTypes = [
  ['auto', 'Auto detect'],
  ['generic', 'Generic'],
  ['cdr', 'CDR'],
  ['anpr', 'ANPR'],
  ['ipdr', 'IPDR'],
  ['subscriber', 'Subscriber'],
  ['tower_location', 'Tower / Location'],
  ['transaction', 'Transaction'],
  ['access_log', 'Access Log'],
]

const operators = [
  ['eq', 'Equals'],
  ['ne', 'Not equals'],
  ['contains', 'Contains'],
  ['gt', '>'],
  ['gte', '>='],
  ['lt', '<'],
  ['lte', '<='],
]

const runtimeQueryExamples = [
  {
    label: 'CDR GPRS rows',
    icon: 'fa-signal',
    query: 'show CDR records where call type is GPRS limit 3 oldest first',
  },
  {
    label: 'IMEI exists',
    icon: 'fa-fingerprint',
    query: 'show rows from source file seed_cdr_large.csv where IMEI exists limit 3 oldest first',
  },
  {
    label: 'Gulberg location',
    icon: 'fa-location-dot',
    query: 'find records where location contains Gulberg limit 3 oldest first',
  },
  {
    label: 'Duration > 60',
    icon: 'fa-greater-than',
    query: 'show CDR records where duration seconds greater than 60 and call type is not SMS limit 3 oldest first',
  },
]

const helperExamples = [
  { label: 'Shortest call', icon: 'fa-arrow-down-wide-short', body: { record_type: 'cdr', helper: 'shortest_call', limit: 1 } },
  { label: 'Longest call', icon: 'fa-arrow-up-wide-short', body: { record_type: 'cdr', helper: 'longest_call', limit: 1 } },
  { label: 'Failed calls', icon: 'fa-phone-slash', body: { record_type: 'cdr', helper: 'failed_calls', limit: 50 } },
  { label: 'Repeat plate locations', icon: 'fa-car-side', body: { record_type: 'anpr', helper: 'repeat_plate_locations', limit: 50 } },
  { label: 'Subscriber rows', icon: 'fa-id-card', body: { record_type: 'subscriber', limit: 50 } },
  { label: 'Tower rows', icon: 'fa-tower-cell', body: { record_type: 'tower_location', limit: 50 } },
  { label: 'Transactions', icon: 'fa-money-bill-transfer', body: { record_type: 'transaction', limit: 50 } },
]

function splitDelimitedLine(line, delimiter) {
  const cells = []
  let current = ''
  let quoted = false
  for (let i = 0; i < line.length; i++) {
    const ch = line[i]
    if (ch === '"') {
      quoted = !quoted
      continue
    }
    if (ch === delimiter && !quoted) {
      cells.push(current.trim())
      current = ''
      continue
    }
    current += ch
  }
  cells.push(current.trim())
  return cells
}

function normalizeField(name) {
  return String(name || '').toLowerCase().trim().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '')
}

function detectRecordType(fields) {
  const normalized = fields.map(normalizeField)
  const has = (...keys) => keys.some(k => normalized.includes(k))
  if (has('source_number', 'msisdn_a', 'calling_number') && has('target_number', 'msisdn_b', 'called_number')) return 'cdr'
  if (has('plate', 'plate_number', 'license_plate', 'registration_no')) return 'anpr'
  if (has('source_ip', 'src_ip', 'destination_ip', 'dst_ip', 'bytes')) return 'ipdr'
  if (has('subscriber_name', 'customer_name', 'msisdn', 'imsi')) return 'subscriber'
  if (has('cell_id', 'lac', 'latitude', 'longitude')) return 'tower_location'
  if (has('transaction_id', 'txn_id', 'amount', 'account')) return 'transaction'
  if (has('http_method', 'method', 'path', 'user_agent', 'status')) return 'access_log'
  return 'generic'
}

async function previewStructuredFile(file) {
  if (!file) return null
  const text = await file.slice(0, 256 * 1024).text()
  const trimmed = text.trim()
  if (!trimmed) return { fields: [], recordType: 'generic', rows: 0 }
  const ext = file.name.split('.').pop()?.toLowerCase()
  let fields = []
  let rows = 0
  try {
    if (ext === 'json' || trimmed.startsWith('[')) {
      const parsed = JSON.parse(trimmed)
      const first = Array.isArray(parsed) ? parsed[0] : parsed
      fields = first && typeof first === 'object' ? Object.keys(first) : []
      rows = Array.isArray(parsed) ? parsed.length : 1
    } else if (ext === 'jsonl' || ext === 'ndjson' || trimmed.startsWith('{')) {
      const firstLine = trimmed.split(/\r?\n/).find(Boolean)
      const first = JSON.parse(firstLine)
      fields = first && typeof first === 'object' ? Object.keys(first) : []
      rows = trimmed.split(/\r?\n/).filter(Boolean).length
    } else {
      const firstLine = trimmed.split(/\r?\n/)[0]
      const delimiter = ext === 'tsv' || firstLine.includes('\t') ? '\t' : ','
      fields = splitDelimitedLine(firstLine, delimiter).filter(Boolean)
      rows = Math.max(0, trimmed.split(/\r?\n/).filter(Boolean).length - 1)
    }
  } catch {
    const firstLine = trimmed.split(/\r?\n/)[0]
    fields = firstLine.includes('=') ? firstLine.split(/\s+/).map(p => p.split(/[=:]/)[0]).filter(Boolean) : ['line']
    rows = trimmed.split(/\r?\n/).filter(Boolean).length
  }
  return { fields, recordType: detectRecordType(fields), rows }
}

function compactValue(value) {
  if (value == null || value === '') return '-'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function formatFileSize(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let size = bytes
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }
  return `${size.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`
}

function csvEscape(value) {
  const text = compactValue(value)
  return /[",\r\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text
}

function seededRandom(seed = 20260714) {
  let state = seed >>> 0
  return () => {
    state = (1664525 * state + 1013904223) >>> 0
    return state / 4294967296
  }
}

function pick(rng, values) {
  return values[Math.floor(rng() * values.length)]
}

function isoAt(base, minutes) {
  return new Date(base.getTime() + minutes * 60000).toISOString()
}

function makeTextFile(name, content, type = 'text/plain') {
  return new File([content], name, { type })
}

function generateSeedPack({ cdrRows = 2500 } = {}) {
  const rng = seededRandom(20260714)
  const areas = [
    ['Gulberg', 'CELL-GLB-01', 31.5204, 74.3587],
    ['DHA Phase 5', 'CELL-DHA-05', 31.4697, 74.4097],
    ['Model Town', 'CELL-MT-03', 31.4832, 74.3229],
    ['Liberty Market', 'CELL-LB-02', 31.5102, 74.3441],
    ['Gate 4', 'CAM-G4-07', 31.5011, 74.3512],
    ['North Road', 'CAM-NR-09', 31.5591, 74.3273],
  ]
  const numbers = ['923461678183', '923001110001', '923001110002', '923331234567', '923451112233']
  const targets = ['923009998881', '923009998882', '923009998883', '923009998884', '923009998885']
  const cdrHeader = ['MSISDN', 'call_org_num', 'CALL_DIALED_NUM', 'IMSI', 'IMEI', 'CALL_START_DT_TM', 'CALL_END_DT_TM', 'INBOUND_OUTBOUND_IND', 'Call_Network_Volume', 'Lac_Id', 'Site_Id', 'Cell_SITE_ID', 'lat', 'longitude', 'CALL_TYPE', 'location']
  const cdrBase = new Date('2026-04-02T00:00:00Z')
  const cdrRowsCsv = [cdrHeader.join(',')]
  for (let i = 0; i < cdrRows; i++) {
    const [area, cell, lat, lon] = pick(rng, areas.slice(0, 4))
    const callType = pick(rng, ['CALL', 'CALL', 'SMS', 'GPRS', 'GPRS', 'VOLTE'])
    const status = rng() > 0.09 ? 'completed' : 'failed'
    const duration = callType === 'CALL' || callType === 'VOLTE' ? Math.floor(5 + rng() * 1800) : 0
    const source = pick(rng, numbers)
    const target = pick(rng, targets)
    const direction = pick(rng, ['INCOMING', 'OUTGOING'])
    const start = new Date(cdrBase.getTime() + i * Math.floor(1 + rng() * 7) * 60000)
    const end = new Date(start.getTime() + (status === 'completed' ? duration : 0) * 1000)
    const rawDate = (date) => date.toISOString().slice(0, 19).replace('T', ' ')
    cdrRowsCsv.push([
      source,
      direction === 'OUTGOING' ? source : target,
      direction === 'OUTGOING' ? target : source,
      `41001${Math.floor(1000000000 + rng() * 8999999999)}`,
      `35678901${Math.floor(100000 + rng() * 899999)}`,
      rawDate(start),
      rawDate(end),
      direction,
      callType === 'GPRS' ? Math.floor(rng() * 8000000) : 0,
      Math.floor(1000 + rng() * 8999),
      Math.floor(100 + rng() * 899),
      cell,
      (lat + (rng() - 0.5) / 400).toFixed(6),
      (lon + (rng() - 0.5) / 400).toFixed(6),
      callType,
      area,
    ].map(csvEscape).join(','))
  }

  const anprHeader = ['timestamp', 'plate_number', 'camera_id', 'location', 'latitude', 'longitude', 'confidence']
  const anprRows = [anprHeader.join(',')]
  const plates = ['ABC-123', 'XYZ-789', 'LEA-447', 'LHR-2026']
  for (let i = 0; i < Math.max(100, Math.floor(cdrRows / 8)); i++) {
    const [area, , lat, lon] = pick(rng, areas.slice(4))
    anprRows.push([
      isoAt(new Date('2026-07-14T06:00:00Z'), i * Math.floor(2 + rng() * 11)),
      pick(rng, plates),
      pick(rng, ['CAM-7', 'CAM-9', 'CAM-12']),
      area,
      (lat + (rng() - 0.5) / 500).toFixed(6),
      (lon + (rng() - 0.5) / 500).toFixed(6),
      (0.74 + rng() * 0.25).toFixed(3),
    ].map(csvEscape).join(','))
  }

  const ipdrLines = []
  const ips = ['10.20.1.7', '10.20.1.8', '10.20.2.12', '172.16.4.22']
  for (let i = 0; i < Math.max(250, Math.floor(cdrRows / 2)); i++) {
    ipdrLines.push(JSON.stringify({
      timestamp: isoAt(new Date('2026-07-14T00:00:00Z'), i),
      source_ip: pick(rng, ips),
      destination_ip: pick(rng, ['8.8.8.8', '1.1.1.1', '104.18.12.20']),
      domain: pick(rng, ['mail.example.test', 'maps.example.test', 'chat.example.test']),
      protocol: pick(rng, ['TCP', 'UDP', 'HTTPS', 'DNS']),
      bytes: Math.floor(200 + rng() * 8000000),
      subscriber_id: pick(rng, numbers),
      session_id: `seed-ipdr-${String(i + 1).padStart(8, '0')}`,
    }))
  }

  const accessRows = [['timestamp', 'source_ip', 'http_method', 'path', 'status', 'user_agent'].join(',')]
  for (let i = 0; i < Math.max(100, Math.floor(cdrRows / 10)); i++) {
    accessRows.push([
      isoAt(new Date('2026-07-14T08:00:00Z'), i),
      pick(rng, ips),
      pick(rng, ['GET', 'POST', 'POST']),
      pick(rng, ['/login', '/case/records-demo', '/api/records/query', '/download/report']),
      pick(rng, [200, 200, 200, 201, 400, 401, 403, 500]),
      pick(rng, ['AnalystWorkbench/1.0', 'CasePortal/2.4', 'Mozilla/5.0']),
    ].map(csvEscape).join(','))
  }

  const subscriberRows = [
    ['msisdn', 'subscriber_name', 'subscriber_id', 'activation_date', 'city', 'status'].join(','),
    ['923461678183', 'A. Khan', 'SUB-9183', '2024-02-18', 'Lahore', 'active'].join(','),
    ['923001110001', 'S. Malik', 'SUB-0001', '2023-11-04', 'Lahore', 'active'].join(','),
    ['923331234567', 'R. Ahmed', 'SUB-4567', '2022-08-21', 'Islamabad', 'active'].join(','),
  ]

  const towerRows = [
    ['cell_site_id', 'site_name', 'location', 'city', 'latitude', 'longitude', 'operator'].join(','),
    ['LHR-GUL-014', 'Gulberg Main Market', 'Gulberg', 'Lahore', '31.520604', '74.358747', 'DemoTel'].join(','),
    ['LHR-DHA-033', 'DHA Phase 5', 'DHA Lahore', 'Lahore', '31.462115', '74.409674', 'DemoTel'].join(','),
    ['ISB-F7-009', 'F-7 Markaz', 'F-7 Islamabad', 'Islamabad', '33.720499', '73.056764', 'DemoTel'].join(','),
  ]

  const transactionRows = [
    ['timestamp', 'account', 'counterparty', 'transaction_amount', 'currency', 'merchant', 'merchant_location', 'channel', 'status'].join(','),
    ['2026-07-10T19:42:01Z', 'ACCT-778812', 'MERCH-4421', '14500', 'PKR', 'North Market Electronics', 'Gulberg', 'pos', 'approved'].join(','),
    ['2026-07-10T22:16:28Z', 'ACCT-778812', 'ACCT-881004', '75000', 'PKR', 'Peer Transfer', 'Lahore', 'mobile', 'approved'].join(','),
    ['2026-07-13T03:20:03Z', 'ACCT-551090', 'MERCH-2290', '31000', 'PKR', 'Clifton Services', 'Karachi', 'pos', 'reversed'].join(','),
  ]

  const policy = `# Records Handling Policy

Collection records-demo is a controlled evidence workspace. Structured CDR, ANPR, IPDR, subscriber, tower, transaction, and access-log files must be ingested through the Records pipeline for exact analytics. Knowledge Base retrieval is used for policy context, source previews, case notes, and citations.

Exact counts, durations, relationships, and timelines must come from deterministic records queries. Duplicate uploads should be flagged by SHA-256 and should not create a new batch unless an analyst explicitly forces a re-ingest for audit testing.
`
  const notes = `Case notes for records-demo

Targets of interest:
- Phone number 923461678183 appears in CDR records.
- Plate ABC-123 appears near Gate 4 and North Road.
- Cross-source timeline should combine structured observations with this narrative note.
`

  return [
    makeTextFile('ui_seed_cdr_large.csv', cdrRowsCsv.join('\r\n'), 'text/csv'),
    makeTextFile('ui_seed_anpr.csv', anprRows.join('\r\n'), 'text/csv'),
    makeTextFile('ui_seed_ipdr.jsonl', ipdrLines.join('\n'), 'application/x-ndjson'),
    makeTextFile('ui_seed_access_log.csv', accessRows.join('\r\n'), 'text/csv'),
    makeTextFile('ui_seed_subscribers.csv', subscriberRows.join('\r\n'), 'text/csv'),
    makeTextFile('ui_seed_towers.csv', towerRows.join('\r\n'), 'text/csv'),
    makeTextFile('ui_seed_transactions.csv', transactionRows.join('\r\n'), 'text/csv'),
    makeTextFile('ui_policy_records_handling.md', policy, 'text/markdown'),
    makeTextFile('ui_case_notes_records_demo.txt', notes, 'text/plain'),
  ]
}

function isLongValue(value) {
  const text = compactValue(value)
  return text.length > 18 || /^[0-9a-f-]{16,}$/i.test(text)
}

function numericValue(value) {
  const n = Number(value)
  return Number.isFinite(n) ? n : 0
}

function sidecarMetric(status, key) {
  return numericValue(status?.summary?.[key])
}

function templateCategory(template) {
  const name = String(template?.name || '').toLowerCase()
  const description = String(template?.description || '').toLowerCase()
  const route = Array.isArray(template?.route) ? template.route.join(' ') : String(template?.route || '')
  const haystack = `${name} ${description} ${route}`
  if (/^(anpr_)/.test(name)) return 'ANPR'
  if (/^(ipdr_)/.test(name)) return 'IPDR'
  if (/^(subscriber_)/.test(name)) return 'Subscriber Identity'
  if (/^(tower_)/.test(name) && name !== 'tower_activity') return 'Tower & Site Reference'
  if (/^(frequent_contacts|call_type_breakdown|temporal_activity|duration_extremes|geospatial_movement|device_identity_changes|service_usage|tower_activity)$/.test(name)) return 'CDR'
  if (/readiness|quality|duplicate|schema|source|file|limitation|missing/.test(haystack)) return 'Operations'
  if (/timeline|activity|hour|daily|location|movement|sighting|anpr|tower/.test(haystack)) return 'Timeline & Movement'
  if (/relationship|contact|network|entity|subscriber/.test(haystack)) return 'Entities & Links'
  if (/evidence|brief|report|semantic|citation|kb/.test(haystack)) return 'Evidence & Briefing'
  return 'Records Analytics'
}

function templateTitle(name) {
  return String(name || 'Template').replace(/_/g, ' ').replace(/\b\w/g, ch => ch.toUpperCase())
}

function templateQuery(template) {
  const name = String(template?.name || '')
  const known = {
    source_file_audit: 'which files were ingested?',
    duplicate_upload_audit: 'show duplicate uploads',
    limitations_and_data_quality: 'what limitations and missing data exist?',
    case_readiness: 'is this case ready for production?',
    schema_profile: 'show detected headers and schema',
    entity_activity: 'show available entities',
    frequent_contacts: 'who are the frequent contacts?',
    call_type_breakdown: 'show CDR call type breakdown',
    temporal_activity: 'show CDR activity by hour and day',
    duration_extremes: 'show shortest longest and average CDR durations',
    geospatial_movement: 'show cited CDR cell and location sequence for TARGET',
    device_identity_changes: 'show IMEI and IMSI changes for TARGET',
    service_usage: 'show voice SMS data and service usage for TARGET',
    tower_activity: 'show tower and cell activity for TARGET',
    ipdr_endpoint_summary: 'show IPDR endpoint summary',
    ipdr_domain_summary: 'show DNS domain summary',
    ipdr_protocol_breakdown: 'show network protocol breakdown',
    ipdr_session_volume: 'show hourly session volume',
    ipdr_subscriber_sessions: 'show IPDR sessions for TARGET',
    ipdr_concurrent_sessions: 'show concurrent IPDR sessions for TARGET',
    ipdr_timeline: 'show IPDR timeline for TARGET',
    relationship_network: 'show relationship network',
    entity_timeline: 'build evidence timeline for TARGET',
    anpr_sightings: 'show exact ANPR sightings',
    anpr_camera_sequence: 'show camera sequence for TARGET',
    anpr_camera_activity: 'show ANPR camera activity',
    anpr_co_travel: 'show same-camera co-observations for TARGET',
    anpr_route_timing: 'show consecutive sighting timing for TARGET',
    anpr_plate_variants: 'show observed plate variants for TARGET',
    anpr_timeline: 'show ANPR timeline for TARGET',
    subscriber_identity_lookup: 'look up subscriber identity observations for TARGET',
    subscriber_validity_timeline: 'show subscriber validity timeline for TARGET',
    subscriber_device_links: 'show subscriber device links for TARGET',
    subscriber_status_summary: 'show subscriber status summary',
    subscriber_conflict_audit: 'show subscriber identity conflicts',
    subscriber_reuse_candidates: 'show subscriber identifier reuse candidates',
    tower_site_lookup: 'look up tower site reference observations for TARGET',
    tower_reference_timeline: 'show tower reference history for TARGET',
    tower_coordinate_audit: 'audit tower coordinates datums and uncertainty',
    tower_status_summary: 'show tower status summary',
    tower_alias_conflicts: 'show tower alias conflicts',
    tower_cdr_join: 'run a time-aware tower join for TARGET',
    evidence: 'find source evidence',
    executive_case_brief: 'summarize this case',
  }
  return known[name] || template?.description || templateTitle(name)
}

function templateTargetInput(template) {
  return (Array.isArray(template?.inputs) ? template.inputs : []).find(input => input?.name === 'target') || null
}

function targetEntryMatchesTemplate(entry, template) {
  const input = templateTargetInput(template)
  const kinds = (input?.accepted_kinds || []).map(kind => String(kind).toLowerCase())
  if (!entry || kinds.length === 0) return true
  const value = String(entry.value || '').toLowerCase()
  const entityType = String(entry.entityType || '').toLowerCase()
  const recordTypes = new Set((entry.recordTypes || []).map(type => String(type).toLowerCase()))
  const isIP = /^(?:\d{1,3}\.){3}\d{1,3}$/.test(value) || value.includes(':')
  return kinds.some(kind => {
    if (kind.includes('plate')) return recordTypes.has('anpr') && entityType === 'primary'
    if (kind.includes('camera')) return recordTypes.has('anpr') && entityType === 'secondary'
    if (kind.includes('location')) return entityType === 'location'
    if (kind.includes('imei')) return entityType === 'imei'
    if (kind.includes('imsi')) return entityType === 'imsi'
    if (kind.includes('ipv4') || kind.includes('ipv6') || kind.includes('nat ip')) return recordTypes.has('ipdr') && isIP
    if (kind.includes('domain')) return recordTypes.has('ipdr') && !isIP && value.includes('.')
    if (kind.includes('subscriber reference')) return recordTypes.has('subscriber') && (entityType === 'identity' || entityType === 'secondary')
    if (kind.includes('subscriber')) return (recordTypes.has('ipdr') || recordTypes.has('subscriber')) && (entityType === 'primary' || entityType === 'phone' || entityType === 'identity' || entityType === 'secondary')
    if (kind.includes('msisdn') || kind.includes('number')) return entityType === 'phone' || (recordTypes.has('cdr') && entityType === 'primary')
    if (kind.includes('cell') || kind.includes('site') || kind.includes('sector') || kind === 'lac' || kind === 'tac' || kind.includes('cgi') || kind.includes('nodeb')) return entityType === 'cell' || (recordTypes.has('tower_location') && (entityType === 'primary' || entityType === 'secondary')) || (recordTypes.has('cdr') && entityType === 'secondary')
    return entityType === kind
  })
}

function materializeTemplateExample(template, target, dateFrom, dateTo, targetEntries = []) {
  const knownTarget = targetEntries.find(entry => entry.value === target)
  const compatibleTarget = knownTarget && !targetEntryMatchesTemplate(knownTarget, template) ? '' : target
  const fallback = templateQuery(template)
  return String(template?.example_query || fallback)
    .replaceAll('{target}', compatibleTarget || '[select a compatible exact case identifier]')
    .replaceAll('{date_from}', dateFrom || '[case start]')
    .replaceAll('{date_to}', dateTo || '[case end]')
}

function isoDateTimeBound(value) {
  if (!value) return undefined
  const parsed = new Date(value)
  return Number.isNaN(parsed.getTime()) ? value : parsed.toISOString()
}

function isCanonicalRuntimeQuery(query) {
  const text = String(query || '').toLowerCase()
  if (!text.trim()) return false
  return (
    /\b(cdr|anpr|ipdr|subscriber|tower|transaction|access log|record|records|rows)\b/.test(text) &&
    (
      /\bwhere\b/.test(text) ||
      /\bbatch[_\s-]?id\b/.test(text) ||
      /\bsource[_\s-]?file\b/.test(text) ||
      /\bfield (exists|not exists)\b/.test(text) ||
      /\b(raw[_\s-]?payload|normalized[_\s-]?fields)\b/.test(text) ||
      /(?:!=|<>|>=|<=|>|<)/.test(text) ||
      /\b(greater than|less than|at least|at most|not equal|is not|contains)\b/.test(text)
    )
  )
}

function templateForRuntimeQuery(query, selectedTemplate) {
  if (isCanonicalRuntimeQuery(query)) return 'canonical_records'
  return selectedTemplate
}

function forensicPromptChips(status, templates) {
  const chips = []
  const add = (label, query, template = '', targetRequired = false) => {
    if (!query || chips.some(chip => chip.query === query && chip.template === template)) return
    chips.push({ label, query, template, targetRequired })
  }
  const families = new Set((status?.record_families || []).map(family => String(family.record_type || '').toLowerCase()))
  const available = new Set((templates || []).map(template => template.name))
  if (available.has('source_file_audit')) add('Source audit', 'which files were ingested?', 'source_file_audit')
  if (sidecarMetric(status, 'duplicate_rows') > 0 && available.has('duplicate_upload_audit')) add('Duplicates', 'show duplicate uploads', 'duplicate_upload_audit')
  if (families.has('cdr')) {
    if (available.has('frequent_contacts')) add('Frequent contacts', 'who are the frequent contacts?', 'frequent_contacts')
    if (available.has('call_type_breakdown')) add('Call types', 'show CDR call type breakdown', 'call_type_breakdown')
    if (available.has('device_identity_changes')) add('Device changes', 'show IMEI and IMSI changes for TARGET', 'device_identity_changes', true)
  }
  if (families.has('ipdr')) {
    if (available.has('ipdr_endpoint_summary')) add('Network endpoints', 'show IPDR endpoint summary', 'ipdr_endpoint_summary')
    if (available.has('ipdr_protocol_breakdown')) add('Network protocols', 'show network protocol breakdown', 'ipdr_protocol_breakdown')
    if (available.has('ipdr_timeline')) add('Network timeline', 'show IPDR timeline for TARGET', 'ipdr_timeline', true)
  }
  if (families.has('anpr')) {
    if (available.has('anpr_sightings')) add('Plate sightings', 'show exact ANPR sightings', 'anpr_sightings')
    if (available.has('anpr_camera_activity')) add('Camera activity', 'show ANPR camera activity', 'anpr_camera_activity')
    if (available.has('anpr_camera_sequence')) add('Camera sequence', 'show camera sequence for TARGET', 'anpr_camera_sequence', true)
    if (available.has('anpr_co_travel')) add('Co-observations', 'show same-camera co-observations for TARGET', 'anpr_co_travel', true)
  }
  if (families.has('subscriber')) {
    if (available.has('subscriber_identity_lookup')) add('Identity lookup', 'look up subscriber identity observations for TARGET', 'subscriber_identity_lookup', true)
    if (available.has('subscriber_validity_timeline')) add('Validity timeline', 'show subscriber validity timeline for TARGET', 'subscriber_validity_timeline', true)
    if (available.has('subscriber_status_summary')) add('Subscriber status', 'show subscriber status summary', 'subscriber_status_summary')
    if (available.has('subscriber_conflict_audit')) add('Identity conflicts', 'show subscriber identity conflicts', 'subscriber_conflict_audit')
    if (available.has('subscriber_reuse_candidates')) add('Reuse candidates', 'show subscriber identifier reuse candidates', 'subscriber_reuse_candidates')
  }
  if (families.has('tower_location')) {
    if (available.has('tower_site_lookup')) add('Tower lookup', 'look up tower site reference observations for TARGET', 'tower_site_lookup', true)
    if (available.has('tower_reference_timeline')) add('Reference history', 'show tower reference history for TARGET', 'tower_reference_timeline', true)
    if (available.has('tower_coordinate_audit')) add('Coordinate audit', 'audit tower coordinates datums and uncertainty', 'tower_coordinate_audit')
    if (available.has('tower_status_summary')) add('Tower status', 'show tower status summary', 'tower_status_summary')
    if (available.has('tower_alias_conflicts')) add('Tower conflicts', 'show tower alias conflicts', 'tower_alias_conflicts')
  }
  if (families.size > 0 && available.has('entity_activity')) add('Entity coverage', 'show available entities', 'entity_activity')
  if (available.has('evidence')) add('KB evidence', 'find source evidence', 'evidence')
  if (chips.length === 0) {
    add('Source audit', 'which files were ingested?', 'source_file_audit')
    add('Case readiness', 'is this case ready for production?', 'case_readiness')
    add('Limitations', 'what limitations and missing data exist?', 'limitations_and_data_quality')
  }
  return chips.slice(0, 12)
}

function flattenForensicRows(result) {
  const rows = []
  const visit = (value, section) => {
    if (rows.length >= 100 || value == null) return
    if (Array.isArray(value)) {
      value.forEach(item => visit(item, section))
      return
    }
    if (typeof value !== 'object') return
    const entries = Object.entries(value)
    const isRow = entries.some(([, entryValue]) => entryValue == null || typeof entryValue !== 'object')
    if (isRow) {
      rows.push({ section, ...value })
      return
    }
    entries.forEach(([key, entryValue]) => visit(entryValue, key))
  }
  visit(result?.records, 'records')
  visit(result?.evidence, 'evidence')
  return rows
}

function forensicRowsForResult(result) {
  const enterpriseRows = result?.enterprise?.data_grid?.rows
  if (Array.isArray(enterpriseRows)) return enterpriseRows
  return flattenForensicRows(result)
}

function forensicColumnDefsForResult(result, rows) {
  const enterpriseColumns = result?.enterprise?.data_grid?.columns
  if (Array.isArray(enterpriseColumns) && enterpriseColumns.length > 0) {
    return enterpriseColumns
      .map(column => ({
        key: column.key || column.name,
        header: column.header || templateTitle(column.key || column.name),
      }))
      .filter(column => column.key && column.key !== 'raw_row')
      .slice(0, 16)
  }
  const keys = []
  rows.slice(0, 20).forEach(row => {
    Object.keys(row || {}).forEach(key => {
      if (!keys.includes(key) && key !== 'raw_row') keys.push(key)
    })
  })
  return keys
    .sort((a, b) => {
      const ai = preferredColumns.indexOf(a)
      const bi = preferredColumns.indexOf(b)
      if (ai !== -1 || bi !== -1) return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi)
      return a.localeCompare(b)
    })
    .slice(0, 14)
    .map(key => ({ key, header: templateTitle(key) }))
}

function mapArray(value) {
  return Array.isArray(value) ? value.filter(item => item && typeof item === 'object') : []
}

function queryMatchesRow(row, query) {
  const needle = String(query || '').trim().toLowerCase()
  if (!needle) return true
  return Object.values(row || {}).some(value => compactValue(value).toLowerCase().includes(needle))
}

function sortableValue(value) {
  if (value == null) return ''
  const numeric = Number(value)
  if (Number.isFinite(numeric)) return numeric
  const date = Date.parse(value)
  if (Number.isFinite(date)) return date
  return String(value).toLowerCase()
}

function rowTimestamp(row) {
  return row?.timestamp || row?.event_time || row?.observed_at || row?.call_start_ts || row?.first_seen || row?.last_seen || row?.completed_at
}

function rowTitle(row) {
  return row?.event_label || row?.record_type || row?.section || row?.source_file || 'Observation'
}

function downloadTextFile(name, type, text) {
  const blob = new Blob([text], { type })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = name
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

function cleanBriefText(value) {
  return String(value || '')
    .replace(/^#{1,6}\s+/gm, '')
    .replace(/^Executive Intelligence Brief\s*/i, '')
    .replace(/\[Deterministic Fact \(SQL\)\]/g, 'SQL fact:')
    .replace(/\[Semantic Context \(KB\)\]/g, 'KB context:')
    .trim()
}

function meaningfulValue(value) {
  if (value == null || value === '') return false
  if (Array.isArray(value)) return value.length > 0
  if (typeof value === 'object') return Object.keys(value).length > 0
  return true
}

function findingMeasureSummary(value) {
  if (!meaningfulValue(value)) return ''
  if (typeof value !== 'object' || Array.isArray(value)) return compactValue(value)
  return Object.entries(value)
    .filter(([, entryValue]) => meaningfulValue(entryValue))
    .filter(([key]) => !['operation_id'].includes(key))
    .slice(0, 5)
    .map(([key, entryValue]) => `${templateTitle(key)}: ${compactValue(entryValue)}`)
    .join(' · ')
}

function analystResultTitle(result) {
  const template = String(result?.template || '').trim()
  if (template) return templateTitle(template)
  const intent = String(result?.intent || '').replace(/^.*\./, '').trim()
  return intent ? templateTitle(intent) : 'Analysis result'
}

function statusTone(status) {
  switch (status) {
    case 'answered':
      return 'ready'
    case 'answered_with_limitations':
      return 'review'
    case 'needs_input':
      return 'input'
    case 'no_results':
      return 'empty'
    default:
      return 'neutral'
  }
}

function statusLabel(status) {
  switch (status) {
    case 'answered':
      return 'Answered'
    case 'answered_with_limitations':
      return 'Answered with limitations'
    case 'needs_input':
      return 'Needs input'
    case 'no_results':
      return 'No matching records'
    default:
      return 'Ready'
  }
}

function evidenceTone(status) {
  switch (String(status || '').toLowerCase()) {
    case 'completed':
      return 'ready'
    case 'failed':
      return 'review'
    case 'processing':
    case 'queued':
    case 'registered':
      return 'input'
    default:
      return 'neutral'
  }
}

function coverageFacts(coverage = {}) {
  const facts = []
  const add = (label, value) => {
    if (value == null || value === '' || (Array.isArray(value) && value.length === 0)) return
    facts.push({ label, value })
  }
  add('Date Coverage', [coverage.collection_min_timestamp, coverage.collection_max_timestamp].filter(Boolean).join(' - '))
  add('Indexed Records', coverage.total_indexed_records)
  add('Nearest Before', coverage.nearest_activity?.before)
  add('Nearest After', coverage.nearest_activity?.after)
  add('Target Examples', Array.isArray(coverage.valid_target_examples) ? coverage.valid_target_examples.join(', ') : '')
  return facts
}

const preferredColumns = [
  'section',
  'row_number',
  'record_type',
  'call_id',
  'plate_number',
  'timestamp',
  'call_time',
  'source_number',
  'target_number',
  'msisdn',
  'duration_seconds',
  'status',
  'area',
  'cell_id',
  'batch_id',
  'collection_name',
  'source_file',
  'source_entry',
]

function isChatSynthesisModel(model) {
  const id = String(model?.id || model?.name || '').trim()
  if (!id) return false
  if (/(embed|embedding|rerank|whisper|transcrib|tts|stt|asr|audio|vad|vision|clip|ocr|image|video)/i.test(id)) return false
  const capabilities = Array.isArray(model?.capabilities) ? model.capabilities : []
  return capabilities.length === 0 || capabilities.some(capability => ['FLAG_CHAT', 'FLAG_COMPLETION', 'chat', 'completion'].includes(capability))
}

function enterpriseV1ToRecordsResult(result) {
  if (result?.contract_version !== 'forensics.enterprise-response/v1') return result
  const primaryTable = Array.isArray(result.tables) ? result.tables[0] : null
  return {
    ...result,
    intent: 'case_query',
    template: result.execution_trace?.parameters?.template || 'auto',
    route: result.execution_trace?.route || [],
    planner: { field_hints: [], query_plan: result.execution_trace?.parameters || {} },
    capability: result.unsupported_operations?.length
      ? { status: 'unavailable', explanation: result.unsupported_operations[0]?.reason, missing_capabilities: result.unsupported_operations.map(item => item.required_capability).filter(Boolean) }
      : { status: 'available' },
    answer: {
      records_summary: result.deterministic_findings?.[0]?.statement || '',
      records_status: result.status === 'no_results' ? 'no_matching_records' : 'matched',
      llm_summary: result.model_interpretation?.text || '',
      llm_fallback_reason: result.model_interpretation?.fallback ? 'Model interpretation fell back.' : '',
    },
    enterprise: {
      status: result.status,
      summary: result.executive_answer,
      metrics: [
        { label: 'Deterministic Findings', value: result.deterministic_findings?.length || 0, source: 'records_sql' },
        { label: 'Citations', value: result.semantic_evidence?.length || 0, source: 'evidence_registry' },
        { label: 'Tables', value: result.tables?.length || 0, source: 'typed_response' },
      ],
      data_grid: {
        columns: (primaryTable?.columns || []).map(column => ({ ...column, header: column.label || column.header || column.key })),
        rows: primaryTable?.rows || [],
        count: primaryTable?.rows?.length || 0,
      },
      provenance: (result.semantic_evidence || []).map(citation => ({
        ...citation,
        source: 'evidence_registry',
        source_file: citation.source_name,
        citation: citation.id,
        source_entry: citation.locator,
      })),
      limitations: result.limitations || [],
      recommended_actions: result.next_actions || [],
      coverage: { collection_id: result.collection_id, route: result.execution_trace?.route || [] },
      synthesis: {
        claims: (result.deterministic_findings || []).map(item => ({ source: 'Deterministic Fact (SQL)', claim: item.statement, value: item.measures })),
      },
      telemetry: result.execution_trace?.latency_ms || {},
      visualizations: Array.isArray(result.visualizations) ? result.visualizations : [],
      execution_trace: result.execution_trace || {},
    },
  }
}

function TypedForensicVisualization({ visualization }) {
  const spec = visualization?.spec || {}
  const rows = Array.isArray(spec.rows) ? spec.rows : []
  const citations = Array.isArray(visualization?.citation_ids) ? visualization.citation_ids : []
  if (!visualization || rows.length === 0) return null

  if (visualization.type === 'bar') {
    const valueField = spec.value_field || 'value'
    const categoryField = spec.category_field || 'label'
    const categoryFields = Array.isArray(spec.category_fields) ? spec.category_fields : []
    const maximum = Math.max(1, ...rows.map(row => numericValue(row?.[valueField])))
    return (
      <figure className="records-visual" aria-label={visualization.title || 'Deterministic bar chart'}>
        <figcaption><strong>{visualization.title}</strong><span>Exact values from {valueField}</span></figcaption>
        <div className="records-bar-list">
          {rows.slice(0, 20).map((row, index) => {
            const value = numericValue(row?.[valueField])
            const combinedCategory = categoryFields.map(field => compactValue(row?.[field])).filter(Boolean).join(' → ')
            const label = compactValue(combinedCategory || row?.[categoryField] || row?.service_class || row?.call_type || row?.metric || `Row ${index + 1}`)
            return <div className="records-bar-row" key={`${label}-${index}`}><span title={label}>{label}</span><div><i style={{ width: `${Math.max(2, (value / maximum) * 100)}%` }} /></div><strong>{value.toLocaleString()}</strong></div>
          })}
        </div>
        {citations.length > 0 && <small>Provenance: {citations.join(', ')}</small>}
      </figure>
    )
  }

  if (visualization.type === 'timeline') {
    const timeFields = Array.isArray(spec.time_fields) ? spec.time_fields : ['observed_at']
    const labelFields = Array.isArray(spec.label_fields) ? spec.label_fields : ['event_label']
    return (
      <figure className="records-visual" aria-label={visualization.title || 'Deterministic timeline'}>
        <figcaption><strong>{visualization.title}</strong><span>Chronological, source-bound events</span></figcaption>
        <ol className="records-visual-timeline">
          {rows.slice(0, 30).map((row, index) => <li key={index}><time>{compactValue(timeFields.map(field => row?.[field]).find(Boolean))}</time><strong>{compactValue(labelFields.map(field => row?.[field]).find(Boolean) || row?.call_dialed_num)}</strong><span>{compactValue([row?.camera_id, row?.location].filter(Boolean).join(' · '))} · {compactValue(row?.source_file)}{row?.row_number != null ? ` · row ${row.row_number}` : ''}</span></li>)}
        </ol>
        {citations.length > 0 && <small>Provenance: {citations.join(', ')}</small>}
      </figure>
    )
  }

  if (visualization.type === 'map') {
    const latitudeField = spec.latitude_field || 'latitude'
    const longitudeField = spec.longitude_field || 'longitude'
    const points = rows.filter(row => row?.[latitudeField] != null && row?.[longitudeField] != null)
    return (
      <figure className="records-visual" aria-label={visualization.title || 'Deterministic location sequence'}>
        <figcaption><strong>{visualization.title}</strong><span>Coordinate inventory; no route is inferred between observations</span></figcaption>
        {points.length > 0 ? <div className="records-results-wrap"><table className="records-results-table"><thead><tr><th>#</th><th>Observed</th><th>Camera / location</th><th>Latitude</th><th>Longitude</th><th>Elapsed / distance</th><th>Source locator</th></tr></thead><tbody>{points.slice(0, 30).map((row, index) => <tr key={index}><td>{index + 1}</td><td>{compactValue(row.observed_at || row.to_observed_at || row.call_start_ts)}</td><td>{compactValue(row.location || row.to_location || row.to_camera_id || row.cell_site_id || row.site_id)}</td><td>{compactValue(row[latitudeField])}</td><td>{compactValue(row[longitudeField])}</td><td>{row.elapsed_seconds != null ? `${numericValue(row.elapsed_seconds).toLocaleString()} sec` : '-'}{row.straight_line_distance_km != null ? ` · ${compactValue(row.straight_line_distance_km)} km straight-line` : ''}</td><td>{compactValue(row.source_file)}{row.row_number != null ? `:${row.row_number}` : ''}</td></tr>)}</tbody></table></div> : <div className="records-alert">No rows include both latitude and longitude. NexusAI will not fabricate map points.</div>}
        {citations.length > 0 && <small>Provenance: {citations.join(', ')}</small>}
      </figure>
    )
  }
  return null
}

export default function RecordsIntelligence({ caseId: caseIdProp = '', embedded = false }) {
  const { addToast } = useOutletContext()
  const { caseId: routeCaseId = '' } = useParams()
  const { activeCase } = useActiveCase()
  const requestedCaseId = (caseIdProp || routeCaseId || '').trim()
  const providerCaseId = activeCase?.caseId || ''
  const caseBindingError = embedded && (!providerCaseId || (requestedCaseId && requestedCaseId !== providerCaseId))
  const boundCaseId = embedded ? (caseBindingError ? '' : providerCaseId) : requestedCaseId
  const boundCollectionId = embedded ? (caseBindingError ? '' : activeCase?.collectionId || providerCaseId) : boundCaseId
  const showSeedLab = useMemo(() => {
    if (embedded) return false
    const params = new URLSearchParams(window.location.search)
    return params.get('seed_lab') === '1' || window.localStorage?.getItem('localai.records.seedLab') === '1'
  }, [embedded])
  const [collections, setCollections] = useState([])
  const [batches, setBatches] = useState([])
  const [loadingBatches, setLoadingBatches] = useState(true)
  const [files, setFiles] = useState([])
  const [collectionName, setCollectionName] = useState(boundCaseId)
  const [recordType, setRecordType] = useState('auto')
  const [preview, setPreview] = useState(null)
  const [uploading, setUploading] = useState(false)
  const [uploadProgress, setUploadProgress] = useState(null)
  const [ingestResults, setIngestResults] = useState([])
  const [confirmDelete, setConfirmDelete] = useState(null)
  const [seedRows, setSeedRows] = useState(2500)
  const [seedCollection, setSeedCollection] = useState('records-demo')
  const [seeding, setSeeding] = useState(false)
  const [tenantId, setTenantId] = useState('default')
  const [sidecarCollection, setSidecarCollection] = useState(boundCaseId || 'records-demo')
  const [sidecarStatus, setSidecarStatus] = useState(null)
  const [sidecarError, setSidecarError] = useState('')
  const [loadingSidecar, setLoadingSidecar] = useState(false)
  const [evidenceCatalog, setEvidenceCatalog] = useState(null)
  const [evidenceError, setEvidenceError] = useState('')
  const [loadingEvidence, setLoadingEvidence] = useState(false)
  const [evidenceSearch, setEvidenceSearch] = useState('')
  const [evidenceStatusFilter, setEvidenceStatusFilter] = useState('')
  const [evidenceModalityFilter, setEvidenceModalityFilter] = useState('')
  const [selectedEvidence, setSelectedEvidence] = useState(null)
  const [loadingEvidenceDetail, setLoadingEvidenceDetail] = useState(false)
  const [reprocessingEvidence, setReprocessingEvidence] = useState(false)
  const [templates, setTemplates] = useState([])
  const [templatesError, setTemplatesError] = useState('')
  const [capabilities, setCapabilities] = useState(null)
  const [capabilitiesError, setCapabilitiesError] = useState('')
  const [loadingCapabilities, setLoadingCapabilities] = useState(false)
  const [agentRegistry, setAgentRegistry] = useState(null)
  const [agentRegistryError, setAgentRegistryError] = useState('')
  const [loadingAgentRegistry, setLoadingAgentRegistry] = useState(false)
  const [forensicQuery, setForensicQuery] = useState(() => new URLSearchParams(window.location.search).get('prompt') || 'which files were ingested?')
  const [forensicTarget, setForensicTarget] = useState('')
  const [discoveredTargets, setDiscoveredTargets] = useState([])
  const [forensicTemplate, setForensicTemplate] = useState('')
  const [forensicDateFrom, setForensicDateFrom] = useState('')
  const [forensicDateTo, setForensicDateTo] = useState('')
  const [forensicLimit, setForensicLimit] = useState(25)
  const [forensicSynthesisModel, setForensicSynthesisModel] = useState('')
  const [modelOptions, setModelOptions] = useState([])
  const [forensicResult, setForensicResult] = useState(null)
  const [forensicRows, setForensicRows] = useState([])
  const [forensicSearch, setForensicSearch] = useState('')
  const [forensicSort, setForensicSort] = useState('')
  const [forensicSortDirection, setForensicSortDirection] = useState('asc')
  const [forensicWorkspaceTab, setForensicWorkspaceTab] = useState('overview')
  const [templateCategoryFilter, setTemplateCategoryFilter] = useState('All')
  const [templateExplorerOpen, setTemplateExplorerOpen] = useState(false)
  const [auditOpen, setAuditOpen] = useState(false)
  const [advancedQueryOpen, setAdvancedQueryOpen] = useState(false)
  const [caseEvidenceOpen, setCaseEvidenceOpen] = useState(false)
  const [dataManagementOpen, setDataManagementOpen] = useState(false)
  const [runningForensicQuery, setRunningForensicQuery] = useState(false)
  const forensicResultRef = useRef(null)
  const forensicDateFromRef = useRef(null)
  const forensicDateToRef = useRef(null)

  const [selectedBatch, setSelectedBatch] = useState('')
  const [queryRecordType, setQueryRecordType] = useState('')
  const [filterField, setFilterField] = useState('')
  const [filterOp, setFilterOp] = useState('eq')
  const [filterValue, setFilterValue] = useState('')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')
  const [sortField, setSortField] = useState('')
  const [sortDirection, setSortDirection] = useState('asc')
  const [limit, setLimit] = useState(50)
  const [aggregateOp, setAggregateOp] = useState('count')
  const [aggregateField, setAggregateField] = useState('')
  const [groupBy, setGroupBy] = useState('')
  const [querying, setQuerying] = useState(false)
  const [results, setResults] = useState([])
  const [resultMeta, setResultMeta] = useState(null)
  const analysisCollectionId = boundCollectionId || sidecarCollection.trim()
  const activeScopeKey = `${boundCaseId}|${analysisCollectionId}`
  const activeScopeRef = useRef(activeScopeKey)
  const structuredRequestRef = useRef(0)
  const forensicRequestRef = useRef(0)
  activeScopeRef.current = activeScopeKey

  useEffect(() => {
    if (!boundCaseId) return
    structuredRequestRef.current += 1
    forensicRequestRef.current += 1
    setCollectionName(boundCollectionId)
    setSidecarCollection(boundCollectionId)
    setBatches([])
    setSelectedBatch('')
    setSidecarStatus(null)
    setEvidenceCatalog(null)
    setSelectedEvidence(null)
    setCapabilities(null)
    setForensicResult(null)
    setForensicRows([])
    setDiscoveredTargets([])
    setResults([])
    setResultMeta(null)
    setFiles([])
    setPreview(null)
    setIngestResults([])
    setConfirmDelete(null)
  }, [boundCaseId, boundCollectionId])

  useEffect(() => {
    if (!forensicResult) return undefined
    const frame = window.requestAnimationFrame(() => {
      forensicResultRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    })
    return () => window.cancelAnimationFrame(frame)
  }, [forensicResult])

  const loadBatches = useCallback(async () => {
    if (embedded && !boundCollectionId) {
      setBatches([])
      setLoadingBatches(false)
      return
    }
    const requestScope = activeScopeKey
    setLoadingBatches(true)
    try {
      const data = await recordsApi.listBatches(boundCollectionId ? { collection_name: boundCollectionId } : {})
      if (activeScopeRef.current !== requestScope) return
      const returnedBatches = Array.isArray(data.batches) ? data.batches : []
      setBatches(boundCollectionId
        ? returnedBatches.filter(batch => batch.collection_name === boundCollectionId)
        : returnedBatches)
    } catch (err) {
      if (activeScopeRef.current === requestScope) addToast(`Failed to load record batches: ${err.message}`, 'error')
    } finally {
      if (activeScopeRef.current === requestScope) setLoadingBatches(false)
    }
  }, [activeScopeKey, addToast, boundCollectionId, embedded])

  const loadSidecarStatus = useCallback(async () => {
    const collectionID = analysisCollectionId
    if (!collectionID) return
    const requestScope = activeScopeKey
    setLoadingSidecar(true)
    setSidecarError('')
    try {
      const data = await recordsApi.forensicStatus({
        tenant_id: tenantId.trim() || 'default',
        collection_id: collectionID,
        limit: 10,
      })
      if (activeScopeRef.current !== requestScope) return
      setSidecarStatus(data)
    } catch (err) {
      if (activeScopeRef.current !== requestScope) return
      setSidecarStatus(null)
      setSidecarError(err.message)
    } finally {
      if (activeScopeRef.current === requestScope) setLoadingSidecar(false)
    }
  }, [activeScopeKey, analysisCollectionId, tenantId])

  const loadEvidenceCatalog = useCallback(async () => {
    const collectionID = analysisCollectionId
    if (!collectionID) return
    const requestScope = activeScopeKey
    setLoadingEvidence(true)
    setEvidenceError('')
    try {
      const data = boundCaseId
        ? await recordsApi.forensicCaseEvidence(boundCaseId, { limit: 75 })
        : await recordsApi.forensicEvidence({
          tenant_id: tenantId.trim() || 'default',
          collection_id: collectionID,
          limit: 75,
        })
      if (activeScopeRef.current !== requestScope) return
      setEvidenceCatalog(data)
    } catch (err) {
      if (activeScopeRef.current !== requestScope) return
      setEvidenceCatalog(null)
      setSelectedEvidence(null)
      setEvidenceError(err.message)
    } finally {
      if (activeScopeRef.current === requestScope) setLoadingEvidence(false)
    }
  }, [activeScopeKey, analysisCollectionId, boundCaseId, tenantId])

  const loadEvidenceDetail = useCallback(async (item) => {
    const evidenceID = item?.evidence_id
    if (!evidenceID) return
    const requestScope = activeScopeKey
    setLoadingEvidenceDetail(true)
    try {
      const data = await recordsApi.forensicEvidenceDetail(evidenceID, {
        tenant_id: tenantId.trim() || 'default',
        collection_id: analysisCollectionId || undefined,
        limit: 25,
      })
      if (activeScopeRef.current !== requestScope) return
      setSelectedEvidence(data)
    } catch (err) {
      if (activeScopeRef.current === requestScope) addToast(`Failed to load evidence detail: ${err.message}`, 'error')
    } finally {
      if (activeScopeRef.current === requestScope) setLoadingEvidenceDetail(false)
    }
  }, [activeScopeKey, addToast, analysisCollectionId, tenantId])

  const reprocessSelectedEvidence = async () => {
    const evidenceID = selectedEvidence?.item?.evidence_id
    const collectionID = analysisCollectionId
    if (!evidenceID || !collectionID) return
    setReprocessingEvidence(true)
    try {
      const idempotencyKey = `records-ui-${Date.now()}-${String(evidenceID).slice(0, 12)}`
      const result = await recordsApi.forensicEvidenceReprocess(evidenceID, {
        tenant_id: tenantId.trim() || 'default',
        collection_id: collectionID,
        reason: 'Operator requested reprocessing from Records Intelligence after reviewing evidence status.',
        max_attempts: 5,
      }, idempotencyKey)
      addToast(`Evidence reprocessing ${result?.status || 'queued'}: ${result?.job_id || evidenceID}`, result?.publication_pending ? 'warning' : 'success')
      await Promise.allSettled([
        loadEvidenceDetail(selectedEvidence.item),
        loadEvidenceCatalog(),
        loadSidecarStatus(),
        loadCapabilities(),
      ])
    } catch (err) {
      addToast(`Evidence reprocessing failed: ${err.message}`, 'error')
    } finally {
      setReprocessingEvidence(false)
    }
  }

  const loadTemplates = useCallback(async () => {
    setTemplatesError('')
    try {
      const data = await recordsApi.forensicTemplates()
      setTemplates(Array.isArray(data.templates) ? data.templates : [])
    } catch (err) {
      setTemplates([])
      setTemplatesError(err.message)
    }
  }, [])

  const loadCapabilities = useCallback(async () => {
    const collectionID = analysisCollectionId
    if (!collectionID) return
    const requestScope = activeScopeKey
    setLoadingCapabilities(true)
    setCapabilitiesError('')
    try {
      const data = await recordsApi.forensicCapabilities({
        tenant_id: tenantId.trim() || 'default',
        collection_id: collectionID,
      })
      if (activeScopeRef.current !== requestScope) return
      setCapabilities(data)
    } catch (err) {
      if (activeScopeRef.current !== requestScope) return
      setCapabilities(null)
      setCapabilitiesError(err.message)
    } finally {
      if (activeScopeRef.current === requestScope) setLoadingCapabilities(false)
    }
  }, [activeScopeKey, analysisCollectionId, tenantId])

  const loadAgentRegistry = useCallback(async () => {
    setLoadingAgentRegistry(true)
    setAgentRegistryError('')
    try {
      const data = await agentsApi.list()
      setAgentRegistry({
        agents: Array.isArray(data?.agents) ? data.agents : [],
        statuses: data?.statuses && typeof data.statuses === 'object' ? data.statuses : {},
      })
    } catch (err) {
      setAgentRegistry(null)
      setAgentRegistryError(err.message || 'Specialist registry is unavailable.')
    } finally {
      setLoadingAgentRegistry(false)
    }
  }, [])

  useEffect(() => {
    loadBatches()
  }, [loadBatches])

  useEffect(() => {
    if (!embedded) agentCollectionsApi.list().then(data => setCollections(Array.isArray(data.collections) ? data.collections : [])).catch(() => {})
    modelsApi.listCapabilities().then(data => {
      const models = Array.isArray(data?.data) ? data.data : []
      setModelOptions(models.filter(model => !model?.disabled && isChatSynthesisModel(model)).map(model => model.id).sort())
    }).catch(() => {
      modelsApi.listV1().then(data => {
        const models = Array.isArray(data?.data) ? data.data : []
        setModelOptions(models.filter(isChatSynthesisModel).map(model => model?.id || model?.name).filter(Boolean).sort())
      }).catch(() => setModelOptions([]))
    })
    loadTemplates()
    loadAgentRegistry()
  }, [embedded, loadAgentRegistry, loadTemplates])

  useEffect(() => {
    loadSidecarStatus()
    loadEvidenceCatalog()
    loadCapabilities()
  }, [loadSidecarStatus, loadEvidenceCatalog, loadCapabilities])

  useEffect(() => {
    let cancelled = false
    previewStructuredFile(files[0]).then(p => {
      if (cancelled) return
      setPreview(p)
      if (p?.recordType && recordType === 'auto') setQueryRecordType(p.recordType)
    })
    return () => { cancelled = true }
  }, [files, recordType])

  const selectedBatchInfo = useMemo(
    () => batches.find(b => b.id === selectedBatch),
    [batches, selectedBatch]
  )

  const availableFields = useMemo(() => {
    const fields = new Set()
    batches.forEach(batch => {
      if (selectedBatch && batch.id !== selectedBatch) return
      ;(batch.fields || []).forEach(field => {
        fields.add(field.canonical_name || field.normalized_name || field.original_name)
      })
    })
    return Array.from(fields).filter(Boolean).sort()
  }, [batches, selectedBatch])

  const buildQuery = () => {
    const body = {
      limit: Number(limit) || 50,
      filters: [],
    }
    if (boundCollectionId) body.collection_name = boundCollectionId
    if (selectedBatch) body.batch_ids = [selectedBatch]
    if (queryRecordType) body.record_type = queryRecordType
    if (filterField && filterValue !== '') {
      body.filters.push({ field: filterField, op: filterOp, value: filterValue })
    }
    if (dateFrom || dateTo) {
      body.time_range = { field: 'timestamp', from: dateFrom || undefined, to: dateTo || undefined }
    }
    if (sortField) {
      body.sort = [{ field: sortField, direction: sortDirection }]
    }
    return body
  }

  const ingestFiles = async (filesToIngest, targetCollection, targetRecordType) => {
    if (!filesToIngest.length) return
    const requestedCollection = targetCollection.trim()
    if (boundCollectionId && requestedCollection && requestedCollection !== boundCollectionId) {
      throw new Error(`This workspace is bound to ${boundCollectionId}; alternate collection targets are not allowed.`)
    }
    const normalizedCollection = boundCollectionId || requestedCollection
    if (!normalizedCollection) throw new Error('Collection is required so evidence, provenance, and exact results stay in one case scope.')
    const knownCollections = collections.map(item => typeof item === 'string' ? item : item?.name).filter(Boolean)
    if (!boundCollectionId && !knownCollections.includes(normalizedCollection)) {
      await agentCollectionsApi.create(normalizedCollection)
      setCollections(previous => [...previous, normalizedCollection].sort())
    }
    setUploadProgress({ done: 0, total: filesToIngest.length, current: filesToIngest[0]?.name || '' })
    setIngestResults([])
    const results = []
    for (let i = 0; i < filesToIngest.length; i++) {
      const currentFile = filesToIngest[i]
      setUploadProgress({ done: i, total: filesToIngest.length, current: currentFile.name })
      const evidenceForm = new FormData()
      evidenceForm.append('file', currentFile)
      evidenceForm.append('record_type', targetRecordType || 'auto')
      evidenceForm.append('declared_modality', 'structured_records')
      evidenceForm.append('evidence_role', 'source')
      evidenceForm.append('case_id', normalizedCollection)
      evidenceForm.append('jurisdiction', 'PK')
      evidenceForm.append('source_timezone_state', 'unknown')
      evidenceForm.append('source_date_order_state', 'unresolved')
      const evidenceRegistration = await agentCollectionsApi.upload(normalizedCollection, evidenceForm)
      if (evidenceRegistration?.records_status === 'failed') {
        throw new Error(evidenceRegistration.records_warning || `${currentFile.name} could not be registered with the forensic processor.`)
      }
      let result
      try {
        result = await recordsApi.ingest(currentFile, normalizedCollection, targetRecordType)
      } catch (err) {
        result = { warnings: [`Legacy batch view skipped: ${err.message}`] }
      }
      result = { ...result, evidence_registration: evidenceRegistration }
      results.push(result)
      setIngestResults([...results])
      if (result?.batch?.id) setSelectedBatch(result.batch.id)
    }
    const totalRows = results.reduce((sum, result) => sum + (result?.batch?.row_count || 0), 0)
    addToast(`Registered ${results.length} evidence file${results.length === 1 ? '' : 's'} for forensic processing${totalRows ? ` and indexed ${totalRows} legacy batch rows` : ''}`, 'success')
    if (!boundCollectionId) setSidecarCollection(normalizedCollection)
    await loadBatches()
    await Promise.allSettled([loadSidecarStatus(), loadEvidenceCatalog()])
  }

  const handleUpload = async (e) => {
    e.preventDefault()
    if (files.length === 0) return
    setUploading(true)
    try {
      await ingestFiles(files, collectionName, recordType)
    } catch (err) {
      addToast(`Record ingestion failed: ${err.message}`, 'error')
    } finally {
      setUploading(false)
      setUploadProgress(null)
    }
  }

  const prepareSeedPack = () => {
    const generated = generateSeedPack({ cdrRows: Number(seedRows) || 2500 })
    setFiles(generated)
    setCollectionName(seedCollection || 'records-demo')
    setRecordType('auto')
    addToast(`Generated ${generated.length} seed files in the upload list`, 'success')
  }

  const ingestSeedPack = async () => {
    const generated = generateSeedPack({ cdrRows: Number(seedRows) || 2500 })
    setFiles(generated)
    setCollectionName(seedCollection || 'records-demo')
    setRecordType('auto')
    setSeeding(true)
    try {
      await ingestFiles(generated, seedCollection || 'records-demo', 'auto')
    } catch (err) {
      addToast(`Seed ingest failed: ${err.message}`, 'error')
    } finally {
      setSeeding(false)
      setUploadProgress(null)
    }
  }

  const runQuery = async (body = buildQuery()) => {
    const requestScope = activeScopeKey
    const requestVersion = ++structuredRequestRef.current
    const scopedBody = boundCollectionId ? { ...body, collection_name: boundCollectionId } : body
    setQuerying(true)
    try {
      const data = await recordsApi.query(scopedBody)
      if (activeScopeRef.current !== requestScope || structuredRequestRef.current !== requestVersion) return
      setResults(Array.isArray(data.records) ? data.records : [])
      setResultMeta({ kind: 'query', total: data.total, count: data.count })
    } catch (err) {
      if (activeScopeRef.current === requestScope && structuredRequestRef.current === requestVersion) addToast(`Records query failed: ${err.message}`, 'error')
    } finally {
      if (activeScopeRef.current === requestScope && structuredRequestRef.current === requestVersion) setQuerying(false)
    }
  }

  const runAggregate = async () => {
    const requestScope = activeScopeKey
    const requestVersion = ++structuredRequestRef.current
    setQuerying(true)
    try {
      const body = {
        ...buildQuery(),
        operation: aggregateOp,
        field: aggregateField || undefined,
        group_by: groupBy ? groupBy.split(',').map(v => v.trim()).filter(Boolean) : undefined,
        top_n: Number(limit) || 50,
      }
      const data = await recordsApi.aggregate(body)
      if (activeScopeRef.current !== requestScope || structuredRequestRef.current !== requestVersion) return
      const rows = []
      ;[...(data.groups || []), ...(data.results || [])].forEach(item => {
        rows.push({ ...(item.key || {}), value: item.value, count: item.count, record: item.record })
      })
      ;(data.values || []).forEach(value => rows.push({ value }))
      if (data.record || data.value !== undefined) rows.push({ value: data.value, record: data.record })
      if (rows.length === 0 && data.count !== undefined) rows.push({ count: data.count })
      setResults(rows)
      setResultMeta({ kind: 'aggregate', total: rows.length, count: rows.length })
    } catch (err) {
      if (activeScopeRef.current === requestScope && structuredRequestRef.current === requestVersion) addToast(`Records aggregate failed: ${err.message}`, 'error')
    } finally {
      if (activeScopeRef.current === requestScope && structuredRequestRef.current === requestVersion) setQuerying(false)
    }
  }

  const runForensicQuery = async (queryOverride, templateOverride, targetOverride, limitOverride) => {
    const targetText = (targetOverride ?? forensicTarget).trim()
    const unresolvedQuery = (queryOverride ?? forensicQuery).trim()
    const templateName = templateOverride ?? templateForRuntimeQuery(
      unresolvedQuery.replace(/\bTARGET\b/gi, targetText),
      advancedQueryOpen ? forensicTemplate : ''
    )
    const requestedTemplate = templates.find(template => template.name === templateName)
    const selectedTargetEntry = discoveredTargets.find(entry => entry.value === targetText)
    if (targetText && selectedTargetEntry && !targetEntryMatchesTemplate(selectedTargetEntry, requestedTemplate)) {
      addToast('The selected discovered identifier is not compatible with this workflow. Select one of the accepted target kinds.', 'warning')
      return
    }
    if ((/\bTARGET\b/i.test(unresolvedQuery) || templateTargetInput(requestedTemplate)?.required) && !targetText) {
      setForensicQuery(unresolvedQuery)
      const acceptedKinds = templateTargetInput(requestedTemplate)?.accepted_kinds || []
      addToast(`Select an exact case identifier before running this workflow${acceptedKinds.length ? ` (${acceptedKinds.join(', ')})` : ''}.`, 'warning')
      return
    }
    const queryText = unresolvedQuery.replace(/\bTARGET\b/gi, targetText)
    if (!queryText && !templateName) return
    if (embedded && (!boundCaseId || !boundCollectionId)) {
      addToast('The active case is unavailable. Reload the Case Workspace before running analysis.', 'error')
      return
    }
    const requestDateFrom = forensicDateFromRef.current?.value || forensicDateFrom
    const requestDateTo = forensicDateToRef.current?.value || forensicDateTo
    if (requestDateFrom !== forensicDateFrom) setForensicDateFrom(requestDateFrom)
    if (requestDateTo !== forensicDateTo) setForensicDateTo(requestDateTo)
    setRunningForensicQuery(true)
    const requestScope = activeScopeKey
    const requestVersion = ++forensicRequestRef.current
    try {
      const request = {
        tenant_id: tenantId.trim() || 'default',
        collection_id: boundCollectionId || sidecarCollection.trim() || collectionName.trim() || 'records-demo',
        case_id: boundCaseId || sidecarCollection.trim() || collectionName.trim() || 'records-demo',
        query: queryText,
        target: targetText || undefined,
        template: templateName || undefined,
        date_from: isoDateTimeBound(requestDateFrom),
        date_to: isoDateTimeBound(requestDateTo),
        limit: Number(limitOverride ?? forensicLimit) || 20,
        max_kb_results: 3,
        synthesis_model: forensicSynthesisModel || undefined,
      }
      const rawData = boundCaseId
        ? await recordsApi.forensicCaseQuery(boundCaseId, request)
        : await recordsApi.forensicQuery(request)
      if (activeScopeRef.current !== requestScope || forensicRequestRef.current !== requestVersion) return
      const data = enterpriseV1ToRecordsResult(rawData)
      const resultRows = forensicRowsForResult(data)
      setForensicResult(data)
      setForensicRows(resultRows)
      if (templateName === 'entity_activity') {
        const exactEntries = resultRows
          .filter(row => compactValue(row.entity_value) && compactValue(row.entity_value) !== '-')
          .map(row => ({
            value: compactValue(row.entity_value),
            entityType: compactValue(row.entity_type),
            recordTypes: Array.isArray(row.record_types) ? row.record_types : [],
          }))
        setDiscoveredTargets(exactEntries)
      }
      setForensicWorkspaceTab('overview')
      setForensicSearch('')
      // A completed request must not overwrite a question the analyst edited while it was running.
      setForensicQuery(current => current === unresolvedQuery ? queryText : current)
      if (data?.intent === 'clarification') {
        addToast(data?.answer?.clarification || 'Clarification required', 'warning')
      }
    } catch (err) {
      if (activeScopeRef.current === requestScope && forensicRequestRef.current === requestVersion) addToast(`Forensic query failed: ${err.message}`, 'error')
    } finally {
      if (activeScopeRef.current === requestScope && forensicRequestRef.current === requestVersion) setRunningForensicQuery(false)
    }
  }

  const activatePromptChip = (chip) => {
    setForensicTemplate(chip.template || '')
    setForensicQuery(chip.query)
    if (chip.targetRequired && !forensicTarget.trim()) {
      addToast('This workflow is prepared. Enter an exact target, then select Analyze.', 'warning')
      return
    }
    runForensicQuery(chip.query, chip.template || '', forensicTarget)
  }

  const deleteBatch = async (batch) => {
    if (boundCollectionId && batch?.collection_name !== boundCollectionId) {
      addToast('Batch deletion blocked because it is outside the active case.', 'error')
      setConfirmDelete(null)
      return
    }
    try {
      await recordsApi.deleteBatch(batch.id)
      addToast('Batch deleted', 'success')
      setConfirmDelete(null)
      if (selectedBatch === batch.id) setSelectedBatch('')
      loadBatches()
    } catch (err) {
      addToast(`Failed to delete batch: ${err.message}`, 'error')
    }
  }

  const columns = useMemo(() => {
    const keys = []
    results.slice(0, 20).forEach(row => {
      Object.keys(row || {}).forEach(key => {
        if (!keys.includes(key) && key !== 'record') keys.push(key)
      })
    })
    return keys
      .sort((a, b) => {
        const ai = preferredColumns.indexOf(a)
        const bi = preferredColumns.indexOf(b)
        if (ai !== -1 || bi !== -1) return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi)
        return a.localeCompare(b)
      })
      .slice(0, 16)
  }, [results])

  const forensicColumnDefs = useMemo(
    () => forensicColumnDefsForResult(forensicResult, forensicRows),
    [forensicResult, forensicRows]
  )
  const forensicColumns = useMemo(() => forensicColumnDefs.map(column => column.key), [forensicColumnDefs])
  const filteredForensicRows = useMemo(() => {
    const rows = forensicRows.filter(row => queryMatchesRow(row, forensicSearch))
    if (!forensicSort) return rows
    return [...rows].sort((a, b) => {
      const av = sortableValue(a?.[forensicSort])
      const bv = sortableValue(b?.[forensicSort])
      if (av < bv) return forensicSortDirection === 'asc' ? -1 : 1
      if (av > bv) return forensicSortDirection === 'asc' ? 1 : -1
      return 0
    })
  }, [forensicRows, forensicSearch, forensicSort, forensicSortDirection])
  const timelineRows = useMemo(
    () => forensicRows
      .filter(row => rowTimestamp(row))
      .sort((a, b) => sortableValue(rowTimestamp(a)) - sortableValue(rowTimestamp(b)))
      .slice(0, 40),
    [forensicRows]
  )
  const templateCategories = useMemo(() => {
    const values = ['All', ...new Set(templates.map(templateCategory))]
    return values
  }, [templates])
  const visibleTemplates = useMemo(() => {
    return templates
      .filter(template => templateCategoryFilter === 'All' || templateCategory(template) === templateCategoryFilter)
      .slice(0, templateCategoryFilter === 'All' ? 12 : 24)
  }, [templates, templateCategoryFilter])
  const promptChips = useMemo(
    () => forensicPromptChips(sidecarStatus, templates),
    [sidecarStatus, templates]
  )
  const evidenceItems = useMemo(() => {
    const catalogItems = Array.isArray(evidenceCatalog?.items) ? evidenceCatalog.items : []
    if (catalogItems.length > 0) return catalogItems
    return Array.isArray(sidecarStatus?.recent_evidence) ? sidecarStatus.recent_evidence : []
  }, [evidenceCatalog, sidecarStatus])
  const evidenceSummary = evidenceCatalog?.summary || sidecarStatus?.summary || {}
  const capabilityFamilies = Array.isArray(capabilities?.families) ? capabilities.families : []
  const capabilitySummary = capabilities?.summary || {}
  const statusFamilies = new Set((sidecarStatus?.record_families || []).map(family => String(family.record_type || '').toLowerCase()))
  const hasCDRFamily = statusFamilies.has('cdr')
  const hasIPDRFamily = statusFamilies.has('ipdr')
  const hasANPRFamily = (sidecarStatus?.record_families || []).some(family => String(family.record_type || '').toLowerCase() === 'anpr')
  const hasANPROperations = templates.some(template => String(template?.name || '').startsWith('anpr_'))
  const hasSubscriberFamily = statusFamilies.has('subscriber')
  const hasSubscriberOperations = templates.some(template => String(template?.name || '').startsWith('subscriber_'))
  const hasTowerFamily = statusFamilies.has('tower_location')
  const hasTowerOperations = templates.some(template => String(template?.name || '').startsWith('tower_') && template?.name !== 'tower_activity')
  const operationalAgents = useMemo(() => new Set((agentRegistry?.agents || []).filter(name => agentRegistry?.statuses?.[name] !== false)), [agentRegistry])
  const specialistActions = useMemo(() => [
    { name: 'Tower_Location_Reference_Analyst', label: 'Tower Specialist', icon: 'fa-tower-cell', familyAvailable: hasTowerFamily, title: 'Continue this question with the case-bound tower and location reference specialist' },
    { name: 'Subscriber_Identity_Analyst', label: 'Subscriber Specialist', icon: 'fa-address-card', familyAvailable: hasSubscriberFamily, title: 'Continue this question with the privacy-aware subscriber identity specialist' },
    { name: 'Vehicle_ANPR_Geospatial_Analyst', label: 'ANPR Specialist', icon: 'fa-car-side', familyAvailable: hasANPRFamily, title: 'Continue this question with the case-bound ANPR and geospatial specialist' },
    { name: 'Network_IPDR_Capture_Analyst', label: 'IPDR Specialist', icon: 'fa-network-wired', familyAvailable: hasIPDRFamily, title: 'Continue this question with the case-bound network IPDR specialist' },
    { name: 'Communications_CDR_Analyst', label: 'CDR Specialist', icon: 'fa-phone-volume', familyAvailable: hasCDRFamily, title: 'Continue this question with the case-bound communications CDR specialist' },
    { name: 'Forensic_Records_Analyst', label: 'Ask Analyst', icon: 'fa-shield-halved', familyAvailable: true, primary: true, title: 'Continue this question in the governed forensic analyst chat' },
  ], [hasANPRFamily, hasCDRFamily, hasIPDRFamily, hasSubscriberFamily, hasTowerFamily])
  const evidenceStatusOptions = useMemo(
    () => Array.from(new Set(evidenceItems.map(item => item.processing_status).filter(Boolean))).sort(),
    [evidenceItems]
  )
  const evidenceModalityOptions = useMemo(
    () => Array.from(new Set(evidenceItems.map(item => item.modality).filter(Boolean))).sort(),
    [evidenceItems]
  )
  const filteredEvidenceItems = useMemo(() => {
    const search = evidenceSearch.trim().toLowerCase()
    return evidenceItems.filter(item => {
      if (evidenceStatusFilter && item.processing_status !== evidenceStatusFilter) return false
      if (evidenceModalityFilter && item.modality !== evidenceModalityFilter) return false
      if (!search) return true
      return [item.evidence_id, item.source_file, item.original_filename, item.sha256, item.kb_entry_ref, item.detected_type, item.case_id]
        .some(value => compactValue(value).toLowerCase().includes(search))
    })
  }, [evidenceItems, evidenceSearch, evidenceStatusFilter, evidenceModalityFilter])
  const selectedEvidenceItem = selectedEvidence?.item || null
  const selectedEvidenceRows = Array.isArray(selectedEvidence?.records_preview) ? selectedEvidence.records_preview : []
  const selectedEvidenceJobs = Array.isArray(selectedEvidence?.ingest_jobs) ? selectedEvidence.ingest_jobs : []
  const selectedEvidenceAssets = Array.isArray(selectedEvidence?.kb_assets) ? selectedEvidence.kb_assets : []
  const enterprise = forensicResult?.enterprise || {}
  const enterpriseMetrics = mapArray(enterprise.metrics)
  const enterpriseClaims = mapArray(enterprise.synthesis?.claims)
  const enterpriseProvenance = mapArray(enterprise.provenance)
  const enterpriseActions = mapArray(enterprise.recommended_actions)
  const enterpriseVisualizations = mapArray(enterprise.visualizations)
  const enterpriseLimitations = Array.isArray(enterprise.limitations) ? enterprise.limitations : []
  const enterpriseCoverage = enterprise.coverage || {}
  const coverageSummary = coverageFacts(enterpriseCoverage)
  const resultStatus = enterprise.status || (forensicResult ? 'answered' : 'ready')
  const validatedModelNarrative = !forensicResult?.answer?.llm_fallback_reason
    ? String(forensicResult?.answer?.llm_summary || '').trim()
    : ''
  const primaryBrief = cleanBriefText(validatedModelNarrative || enterprise.summary || forensicResult?.answer?.records_summary || forensicResult?.answer?.evidence_summary || '')
  const analystClaims = enterpriseClaims
    .filter(claim => String(claim?.claim || '').trim())
    .map(claim => ({ ...claim, measureSummary: findingMeasureSummary(claim.value) }))
    .slice(0, 6)
  const resultTitle = analystResultTitle(forensicResult)
  const resultLatency = Object.values(enterprise.telemetry || {})
    .reduce((total, value) => total + numericValue(value), 0)
  const resultPreviewRows = filteredForensicRows.slice(0, 8)
  const previewColumnDefs = forensicColumnDefs.slice(0, 7)
  const selectedTemplate = useMemo(
    () => templates.find(template => template.name === forensicTemplate) || null,
    [templates, forensicTemplate]
  )
  const selectedTargetInput = templateTargetInput(selectedTemplate)
  const recentTargets = useMemo(() => {
    const values = []
    const add = value => {
      const text = compactValue(value)
      if (!text || text === '-' || values.includes(text)) return
      values.push(text)
    }
    forensicRows.slice(0, 80).forEach(row => {
      add(row.primary_target || row.primary_entity || row.msisdn || row.source_number || row.subscriber_identifier)
      add(row.secondary_target || row.secondary_entity || row.target_number || row.call_dialed_num || row.plate_number || row.plate_search_key)
      add(row.imsi || row.imei || row.source_ip || row.destination_ip || row.nat_source_ip || row.nat_destination_ip)
    })
    discoveredTargets
      .filter(entry => !selectedTemplate || targetEntryMatchesTemplate(entry, selectedTemplate))
      .forEach(entry => add(entry.value))
    return values.slice(0, 12)
  }, [forensicRows, discoveredTargets, selectedTemplate])
  const maskedTargetHints = useMemo(
    () => (Array.isArray(enterpriseCoverage.valid_target_examples) ? enterpriseCoverage.valid_target_examples : [])
      .map(compactValue)
      .filter(value => value && value !== '-')
      .slice(0, 8),
    [enterpriseCoverage.valid_target_examples]
  )

  const exportResults = () => {
    if (results.length === 0 || columns.length === 0) return
    const csv = [
      columns.map(csvEscape).join(','),
      ...results.map(row => columns.map(col => csvEscape(row[col])).join(',')),
    ].join('\r\n')
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    const scope = (boundCaseId || analysisCollectionId || 'unbound').replace(/[^a-zA-Z0-9._-]+/g, '-')
    link.download = `records-${scope}-${resultMeta?.kind || 'results'}-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.csv`
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
  }

  const exportForensic = (format = 'csv') => {
    if (!forensicResult) return
    const stamp = new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')
    const scope = (boundCaseId || analysisCollectionId || 'unbound').replace(/[^a-zA-Z0-9._-]+/g, '-')
    if (format === 'json') {
      downloadTextFile(`forensic-audit-${scope}-${stamp}.json`, 'application/json;charset=utf-8', JSON.stringify(forensicResult, null, 2))
      return
    }
    if (filteredForensicRows.length === 0 || forensicColumns.length === 0) return
    const csv = [
      forensicColumns.map(csvEscape).join(','),
      ...filteredForensicRows.map(row => forensicColumns.map(col => csvEscape(row[col])).join(',')),
    ].join('\r\n')
    downloadTextFile(`forensic-grid-${scope}-${stamp}.csv`, 'text/csv;charset=utf-8', csv)
  }

  if (caseBindingError) return (
    <div className="page page--narrow">
      <EmptyState
        state="error"
        eyebrow="Case boundary"
        title="Analysis scope unavailable"
        headingLevel={1}
        body="Records Intelligence could not verify the active Case Workspace identity. No query or data operation was started."
      />
    </div>
  )

  return (
    <div className={`page page--wide ${embedded ? 'records-embedded' : ''}`}>
      <style>{`
        .records-grid { display: grid; grid-template-columns: minmax(280px, 0.95fr) minmax(320px, 1.4fr); gap: var(--spacing-lg); align-items: start; }
        .records-panel { display: flex; flex-direction: column; gap: var(--spacing-md); }
        .records-embedded { padding:0; }.records-embedded>.card { box-shadow:none; border-radius:var(--radius-lg); }.records-embedded .records-query-presets .btn:nth-child(n+7){display:none}
        .records-scope-lock { display:flex; align-items:center; justify-content:space-between; gap:var(--spacing-md); margin-bottom:var(--spacing-md); padding:var(--spacing-sm) var(--spacing-md); border:1px solid color-mix(in srgb,var(--color-primary) 42%,var(--color-border)); border-radius:var(--radius-md); background:linear-gradient(120deg,color-mix(in srgb,var(--color-primary) 9%,var(--color-bg-primary)),var(--color-bg-primary)); }
        .records-scope-lock__identity { display:flex; align-items:center; gap:var(--spacing-sm); min-width:0; }.records-scope-lock__identity>i{display:grid;place-items:center;width:34px;height:34px;border-radius:10px;color:white;background:linear-gradient(135deg,#2563eb,#4338ca);flex:0 0 auto}.records-scope-lock strong,.records-scope-lock span{display:block}.records-scope-lock span{color:var(--color-text-secondary);font-size:.78rem;margin-top:2px}.records-scope-lock code{max-width:42%;overflow-wrap:anywhere;text-align:right;color:var(--color-text-primary)}
        .records-form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--spacing-sm); }
        .records-field { display: flex; flex-direction: column; gap: var(--spacing-xs); }
        .records-field label { font-size: 0.8125rem; font-weight: 600; color: var(--color-text-secondary); }
        .records-preview-fields { display: flex; flex-wrap: wrap; gap: var(--spacing-xs); }
        .records-chip { font-size: 0.75rem; padding: 3px 8px; border-radius: 999px; background: var(--color-bg-secondary); color: var(--color-text-secondary); border: 1px solid var(--color-border); }
        .records-file-list { display: flex; flex-direction: column; gap: 6px; max-height: 160px; overflow: auto; }
        .records-file-row { display: flex; justify-content: space-between; gap: var(--spacing-sm); padding: 6px 8px; border: 1px solid var(--color-border); border-radius: var(--radius-sm); background: var(--color-bg-secondary); font-size: 0.8125rem; }
        .records-batch-list { display: flex; flex-direction: column; gap: var(--spacing-sm); max-height: 420px; overflow: auto; }
        .records-batch { border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: var(--spacing-sm); background: var(--color-bg-primary); }
        .records-batch-header { display: flex; justify-content: space-between; gap: var(--spacing-sm); align-items: flex-start; }
        .records-batch-title { font-weight: 700; word-break: break-word; }
        .records-batch-meta { margin-top: 4px; display: flex; flex-wrap: wrap; gap: 6px; font-size: 0.75rem; color: var(--color-text-muted); }
        .records-helper-row { display: flex; flex-wrap: wrap; gap: var(--spacing-xs); }
        .records-command { display: grid; grid-template-columns: 1fr; gap: var(--spacing-sm); align-items: stretch; }
        .records-command-main { display: flex; flex-direction: column; gap: var(--spacing-sm); min-width: 0; }
        .records-case-strip { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: var(--spacing-sm); align-content: start; }
        .records-kpi-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: var(--spacing-sm); }
        .records-kpi { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: var(--spacing-sm); background: var(--color-bg-secondary); min-width: 0; }
        .records-kpi span { display: block; font-size: 0.75rem; color: var(--color-text-muted); }
        .records-kpi strong { display: block; margin-top: 2px; font-size: 1.125rem; color: var(--color-text-primary); overflow-wrap: anywhere; }
        .records-family-list { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: var(--spacing-sm); }
        .records-family { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: var(--spacing-sm); background: var(--color-bg-secondary); min-width: 0; }
        .records-family strong { display: block; margin-bottom: 4px; color: var(--color-text-primary); overflow-wrap: anywhere; }
        .records-family span { display: block; color: var(--color-text-secondary); font-size: 0.8125rem; }
        .records-primary-query { display: grid; grid-template-columns: minmax(320px, 1.3fr) minmax(200px, 0.55fr) auto; gap: var(--spacing-sm); align-items: end; }
        .records-primary-query--unbound { grid-template-columns: minmax(180px, 0.4fr) minmax(280px, 1fr) minmax(180px, 0.45fr) auto; }
        .records-advanced-query { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: var(--spacing-sm); padding: var(--spacing-sm); border: 1px solid var(--color-border); border-radius: var(--radius-sm); background: var(--color-bg-secondary); }
        .records-progress-nav { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--spacing-sm); margin-bottom: var(--spacing-lg); }
        .records-progress-step { border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-bg-primary); padding: var(--spacing-sm) var(--spacing-md); display: flex; align-items: center; gap: var(--spacing-sm); text-align: left; color: var(--color-text-primary); }
        .records-progress-step > span:first-child { display: grid; place-items: center; width: 30px; height: 30px; flex: 0 0 30px; border-radius: 999px; background: var(--color-bg-secondary); color: var(--color-text-secondary); font-weight: 700; }
        .records-progress-step strong, .records-progress-step small { display: block; }
        .records-progress-step small { margin-top: 2px; color: var(--color-text-secondary); line-height: 1.35; }
        .records-section-toggle { display: flex; justify-content: space-between; align-items: center; gap: var(--spacing-sm); padding: var(--spacing-md); border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-bg-secondary); }
        .records-query-presets { display: flex; flex-wrap: wrap; gap: var(--spacing-xs); }
        .records-query-presets .btn { white-space: normal; text-align: left; }
        .records-runtime-panel { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: var(--spacing-sm); background: var(--color-bg-secondary); display: grid; grid-template-columns: minmax(180px, 0.42fr) minmax(0, 1fr); gap: var(--spacing-sm); align-items: center; }
        .records-runtime-panel strong { display: block; color: var(--color-text-primary); margin-bottom: 2px; }
        .records-runtime-panel span { display: block; color: var(--color-text-secondary); font-size: 0.8125rem; line-height: 1.4; }
        .records-runtime-actions { display: flex; flex-wrap: wrap; gap: var(--spacing-xs); justify-content: flex-end; }
        .records-runtime-actions .btn { white-space: normal; text-align: left; }
        .records-family-command { display: grid; grid-template-columns: minmax(220px, .48fr) minmax(0, 1fr); gap: var(--spacing-md); align-items: center; border: 1px solid rgba(14, 116, 144, .3); border-radius: var(--radius-md); padding: var(--spacing-md); background: linear-gradient(135deg, rgba(8, 145, 178, .10), rgba(37, 99, 235, .06)); }
        .records-family-command h3 { margin: 0 0 4px; font-size: .95rem; color: var(--color-text-primary); }
        .records-family-command p { margin: 0; color: var(--color-text-secondary); font-size: .8125rem; line-height: 1.45; }
        .records-family-command .records-runtime-actions { align-items: center; }
        .records-family-command-note { display: inline-flex; gap: 6px; align-items: center; margin-top: 8px; color: var(--color-text-muted); font-size: .75rem; }
        .records-action-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: var(--spacing-sm); }
        .records-action { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: var(--spacing-sm); background: var(--color-bg-primary); text-align: left; display: flex; flex-direction: column; gap: 4px; min-height: 84px; }
        .records-action strong { color: var(--color-text-primary); overflow-wrap: anywhere; }
        .records-action span { color: var(--color-text-secondary); font-size: 0.8125rem; line-height: 1.4; }
        .records-result-shell { display: block; scroll-margin-top: var(--spacing-lg); }
        .records-status-pill { display: inline-flex; align-items: center; gap: 6px; border-radius: 999px; border: 1px solid var(--color-border); padding: 5px 10px; font-size: 0.8125rem; font-weight: 700; background: var(--color-bg-primary); color: var(--color-text-secondary); }
        .records-status-pill[data-tone="ready"] { border-color: rgba(34, 197, 94, 0.45); color: #166534; background: rgba(34, 197, 94, 0.08); }
        .records-status-pill[data-tone="review"] { border-color: rgba(245, 158, 11, 0.45); color: #92400e; background: rgba(245, 158, 11, 0.1); }
        .records-status-pill[data-tone="input"], .records-status-pill[data-tone="empty"] { border-color: rgba(59, 130, 246, 0.45); color: #1d4ed8; background: rgba(59, 130, 246, 0.08); }
        .records-template-tabs, .records-workspace-tabs { display: flex; flex-wrap: wrap; gap: var(--spacing-xs); }
        .records-target-tools { display: flex; flex-wrap: wrap; align-items: center; gap: var(--spacing-xs); padding: var(--spacing-sm); border: 1px solid var(--color-border); border-radius: var(--radius-sm); background: var(--color-bg-secondary); }
        .records-target-tools > span { color: var(--color-text-secondary); font-size: .8125rem; }
        .records-target-hints { width: 100%; color: var(--color-text-muted); font-size: .75rem; line-height: 1.4; }
        .records-template-card { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: var(--spacing-sm); background: var(--color-bg-primary); text-align: left; display: flex; flex-direction: column; gap: 8px; min-height: 150px; }
        .records-template-card strong { color: var(--color-text-primary); overflow-wrap: anywhere; }
        .records-template-card span { color: var(--color-text-secondary); font-size: 0.8125rem; line-height: 1.45; }
        .records-template-card details { border-top: 1px solid var(--color-border); padding-top: 7px; }
        .records-template-card summary { cursor: pointer; color: var(--color-text-primary); font-size: .8125rem; font-weight: 700; }
        .records-template-detail { display: grid; gap: 6px; margin-top: 8px; color: var(--color-text-secondary); font-size: .78rem; line-height: 1.45; }
        .records-template-detail code { white-space: normal; overflow-wrap: anywhere; color: var(--color-text-primary); }
        .records-template-card .btn { margin-top: auto; width: fit-content; }
        .records-template-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: var(--spacing-sm); }
        .records-workspace { border: 1px solid var(--color-border); border-radius: var(--radius-md); overflow: hidden; background: var(--color-bg-primary); }
        .records-workspace-header { display: flex; flex-direction: column; align-items: stretch; gap: var(--spacing-sm); padding: var(--spacing-md); border-bottom: 1px solid var(--color-border); background: var(--color-bg-secondary); }
        .records-workspace-body { padding: var(--spacing-md); display: flex; flex-direction: column; gap: var(--spacing-md); background: var(--color-bg-primary); }
        .records-result-heading { display: flex; justify-content: space-between; align-items: flex-start; gap: var(--spacing-md); flex-wrap: wrap; }
        .records-result-heading h2 { margin: 0; font-size: 1.2rem; color: var(--color-text-primary); }
        .records-result-meta { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 5px; color: var(--color-text-secondary); font-size: .8125rem; }
        .records-result-meta span + span::before { content: '·'; margin-right: 8px; color: var(--color-text-muted); }
        .records-workspace-tabs { width: fit-content; max-width: 100%; padding: 3px; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-bg-primary); }
        .records-workspace-tabs .btn { border-color: transparent; }
        .records-answer-hero { padding: clamp(16px, 2vw, 24px); border: 1px solid rgba(59, 130, 246, .35); border-radius: var(--radius-lg); background: linear-gradient(145deg, rgba(59, 130, 246, .10), rgba(14, 165, 233, .035) 55%, var(--color-bg-primary)); box-shadow: 0 10px 30px rgba(15, 23, 42, .08); }
        .records-answer-label { display: flex; align-items: center; gap: 7px; margin-bottom: 8px; color: var(--color-text-secondary); font-size: .75rem; font-weight: 800; letter-spacing: .08em; text-transform: uppercase; }
        .records-brief { white-space: pre-wrap; color: var(--color-text-primary); font-size: .98rem; line-height: 1.7; }
        .records-section-heading { display: flex; justify-content: space-between; align-items: baseline; gap: var(--spacing-sm); margin-top: 2px; }
        .records-section-heading h3 { margin: 0; font-size: .95rem; color: var(--color-text-primary); }
        .records-section-heading span { color: var(--color-text-muted); font-size: .75rem; }
        .records-finding-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); gap: var(--spacing-sm); }
        .records-finding { border: 1px solid var(--color-border); border-left: 3px solid var(--color-primary); border-radius: var(--radius-md); padding: var(--spacing-sm); background: var(--color-bg-secondary); min-width: 0; }
        .records-finding strong { display: block; margin-bottom: 5px; color: var(--color-text-primary); }
        .records-finding p { margin: 0; color: var(--color-text-secondary); line-height: 1.5; font-size: .85rem; }
        .records-finding small { display: block; margin-top: 7px; color: var(--color-text-muted); overflow-wrap: anywhere; }
        .records-inline-actions { display: flex; flex-wrap: wrap; gap: var(--spacing-xs); }
        .records-audit-details { border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-bg-secondary); }
        .records-audit-details > summary { cursor: pointer; list-style: none; padding: 11px 13px; font-weight: 700; color: var(--color-text-primary); }
        .records-audit-details > summary::-webkit-details-marker { display: none; }
        .records-audit-details > div { padding: 0 13px 13px; display: flex; flex-direction: column; gap: var(--spacing-sm); }
        .records-metric-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: var(--spacing-sm); }
        .records-claim-list, .records-provenance-list, .records-timeline { display: flex; flex-direction: column; gap: var(--spacing-sm); }
        .records-claim, .records-provenance, .records-timeline-item { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: var(--spacing-sm); background: var(--color-bg-secondary); min-width: 0; }
        .records-claim strong, .records-provenance strong, .records-timeline-item strong { display: block; color: var(--color-text-primary); margin-bottom: 4px; overflow-wrap: anywhere; }
        .records-claim span, .records-provenance span, .records-timeline-item span { display: block; color: var(--color-text-secondary); font-size: 0.8125rem; overflow-wrap: anywhere; }
        .records-table-tools { display: grid; grid-template-columns: minmax(220px, 1fr) minmax(150px, 0.4fr) minmax(120px, 0.28fr) auto auto; gap: var(--spacing-sm); align-items: end; }
        .records-audit-modal { position: fixed; inset: 0; z-index: 60; background: rgba(0, 0, 0, 0.45); display: flex; align-items: center; justify-content: center; padding: var(--spacing-lg); }
        .records-audit-dialog { width: min(920px, 100%); max-height: 82vh; overflow: auto; border-radius: var(--radius-md); background: var(--color-bg-primary); border: 1px solid var(--color-border); box-shadow: var(--shadow-lg); }
        .records-audit-dialog header { display: flex; align-items: center; justify-content: space-between; gap: var(--spacing-sm); padding: var(--spacing-md); border-bottom: 1px solid var(--color-border); }
        .records-audit-dialog pre { margin: 0; padding: var(--spacing-md); overflow: auto; font-size: 0.75rem; color: var(--color-text-secondary); }
        .records-telemetry { border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: var(--spacing-md); background: var(--color-bg-primary); display: flex; flex-direction: column; gap: var(--spacing-sm); }
        .records-visual-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(300px,1fr)); gap:var(--spacing-md); }.records-visual { margin:0; min-width:0; border:1px solid var(--color-border); border-radius:var(--radius-md); padding:var(--spacing-md); background:var(--color-bg-secondary); }.records-visual figcaption { display:flex; justify-content:space-between; gap:var(--spacing-sm); align-items:start; margin-bottom:var(--spacing-md); }.records-visual figcaption strong,.records-visual figcaption span { display:block; }.records-visual figcaption span,.records-visual>small { color:var(--color-text-muted); font-size:.75rem; }.records-bar-list { display:flex; flex-direction:column; gap:8px; }.records-bar-row { display:grid; grid-template-columns:minmax(105px,.45fr) minmax(90px,1fr) auto; gap:8px; align-items:center; font-size:.78rem; }.records-bar-row>span { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.records-bar-row>div { height:12px; border-radius:999px; background:var(--color-bg-primary); overflow:hidden; }.records-bar-row i { display:block; height:100%; border-radius:999px; background:linear-gradient(90deg,#2563eb,#4f46e5); }.records-visual-timeline { list-style:none; padding:0; margin:0; display:flex; flex-direction:column; gap:8px; }.records-visual-timeline li { position:relative; display:grid; grid-template-columns:minmax(150px,.35fr) minmax(150px,.4fr) minmax(0,1fr); gap:var(--spacing-sm); padding:9px 10px 9px 18px; border-left:3px solid var(--color-primary); background:var(--color-bg-primary); border-radius:0 var(--radius-sm) var(--radius-sm) 0; }.records-visual-timeline time,.records-visual-timeline span { color:var(--color-text-muted); font-size:.76rem; overflow-wrap:anywhere; }
        .records-planner { display: flex; flex-wrap: wrap; gap: var(--spacing-xs); align-items: center; }
        .records-alert { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: var(--spacing-sm); background: var(--color-bg-secondary); color: var(--color-text-secondary); overflow-wrap: anywhere; }
        .records-alert-warning { border-color: rgba(245, 158, 11, 0.45); background: rgba(245, 158, 11, 0.1); color: var(--color-warning, #92400e); }
        .records-evidence-panel { border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-bg-primary); overflow: hidden; }
        .records-evidence-header { display: flex; justify-content: space-between; align-items: center; gap: var(--spacing-sm); flex-wrap: wrap; padding: var(--spacing-sm); border-bottom: 1px solid var(--color-border); background: var(--color-bg-secondary); }
        .records-evidence-body { display: grid; grid-template-columns: minmax(0, 1fr) minmax(260px, 0.38fr); gap: var(--spacing-md); padding: var(--spacing-md); align-items: start; }
        .records-evidence-list { display: flex; flex-direction: column; gap: var(--spacing-sm); max-height: 440px; overflow: auto; }
        .records-evidence-item { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: var(--spacing-sm); background: var(--color-bg-secondary); text-align: left; display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: var(--spacing-sm); align-items: start; }
        .records-evidence-title { font-weight: 700; color: var(--color-text-primary); overflow-wrap: anywhere; }
        .records-evidence-meta { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 6px; color: var(--color-text-muted); font-size: 0.75rem; }
        .records-evidence-detail { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: var(--spacing-sm); background: var(--color-bg-secondary); min-width: 0; display: flex; flex-direction: column; gap: var(--spacing-sm); }
        .records-evidence-detail code { white-space: normal; overflow-wrap: anywhere; }
        .records-coverage-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: var(--spacing-sm); }
        .records-drawer-toggle { display: flex; justify-content: space-between; align-items: center; gap: var(--spacing-sm); flex-wrap: wrap; }
        .records-results-wrap { overflow: auto; max-height: 560px; border: 1px solid var(--color-border); border-radius: var(--radius-md); }
        .records-results-table { width: 100%; border-collapse: collapse; font-size: 0.8125rem; }
        .records-results-table th, .records-results-table td { padding: 8px 10px; border-bottom: 1px solid var(--color-border); text-align: left; vertical-align: top; }
        .records-results-table th { position: sticky; top: 0; background: var(--color-bg-secondary); z-index: 1; white-space: nowrap; }
        .records-results-table td { max-width: 240px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
        .records-results-table td.records-cell-long { font-family: var(--font-mono, monospace); max-width: 190px; }
        .records-results-toolbar { display: flex; justify-content: space-between; gap: var(--spacing-sm); align-items: center; flex-wrap: wrap; }
        .records-guidance { color: var(--color-text-secondary); font-size: 0.875rem; line-height: 1.5; }
        .records-seed-grid { display: grid; grid-template-columns: minmax(220px, 1fr) minmax(140px, 0.35fr) auto auto; gap: var(--spacing-sm); align-items: end; }
        .records-checklist { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--spacing-sm); }
        .records-check { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: var(--spacing-sm); background: var(--color-bg-secondary); min-width: 0; }
        .records-check strong { display: block; color: var(--color-text-primary); margin-bottom: 2px; }
        @media (max-width: 980px) { .records-grid, .records-form-grid, .records-command, .records-result-shell, .records-primary-query, .records-primary-query--unbound, .records-advanced-query, .records-progress-nav, .records-table-tools, .records-runtime-panel, .records-family-command, .records-evidence-body { grid-template-columns: 1fr; } .records-runtime-actions { justify-content: flex-start; } }
        @media (max-width: 1160px) { .records-seed-grid, .records-checklist, .records-kpi-grid { grid-template-columns: 1fr; } .records-case-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
        @media (max-width: 620px) { .records-scope-lock { align-items:flex-start; flex-direction:column; } .records-scope-lock code { max-width:100%; text-align:left; } }
      `}</style>

      {!embedded && <PageHeader
        title="Records Intelligence"
        supporting="Ask a case question first. Open evidence review or data management only when you need those controls."
      />}

      {embedded && <section className="records-scope-lock" aria-label={`Analysis locked to active case ${boundCaseId}`}>
        <div className="records-scope-lock__identity">
          <i className="fas fa-shield-halved" aria-hidden="true" />
          <div><strong>Active case boundary enforced</strong><span>Queries, evidence, batches, uploads, and exports stay inside this case.</span></div>
        </div>
        <code title={boundCaseId}>{boundCaseId}</code>
      </section>}

      {!embedded && !forensicResult && <div className="records-progress-nav" aria-label="Records workflow">
        <button className="records-progress-step" type="button" onClick={() => { setCaseEvidenceOpen(false); setDataManagementOpen(false) }}>
          <span>1</span><span><strong>Ask and analyze</strong><small>Run a plain-language question and review exact, cited results.</small></span>
        </button>
        <button className="records-progress-step" type="button" onClick={() => { setCaseEvidenceOpen(true); setDataManagementOpen(false) }}>
          <span>2</span><span><strong>Review evidence</strong><small>Inspect source files, processing status, provenance, and capability coverage.</small></span>
        </button>
        <button className="records-progress-step" type="button" onClick={() => { setCaseEvidenceOpen(false); setDataManagementOpen(true) }}>
          <span>3</span><span><strong>Manage data</strong><small>Upload records, inspect batches, and use advanced exact-query tools.</small></span>
        </button>
      </div>}

      <div className="card records-panel" style={{ marginBottom: 'var(--spacing-lg)' }}>
        <div className="records-results-toolbar">
          <div>
            <h2 style={{ fontSize: '1rem', margin: 0 }}>Analysis desk</h2>
            <div className="records-guidance">
              Ask one evidence question. NexusAI selects exact records, cited knowledge, or both from this collection's live capabilities.
            </div>
          </div>
          <div className="records-runtime-actions">
            {loadingAgentRegistry && <span className="records-status-pill" role="status"><i className="fas fa-spinner fa-spin" /> Verifying specialists</span>}
            {!loadingAgentRegistry && specialistActions.filter(action => action.familyAvailable).map(action => operationalAgents.has(action.name) ? (
              <Link
                className={`btn ${action.primary ? 'btn-primary' : 'btn-secondary'} btn-sm`}
                key={action.name}
                to={`/app/agents/${action.name}/chat?prompt=${encodeURIComponent(forensicQuery)}${boundCaseId ? `&case=${encodeURIComponent(boundCaseId)}` : ''}`}
                title={action.title}
              >
                <i className={`fas ${action.icon}`} /> {action.label}
              </Link>
            ) : (
              <span className="records-status-pill" data-tone="review" key={action.name} aria-label={`${action.label} unavailable`} title={`${action.label} is not currently registered and operational.`}>
                <i className={`fas ${action.icon}`} /> {action.label} unavailable
              </span>
            ))}
            <button className="btn btn-secondary btn-sm" type="button" onClick={() => { loadSidecarStatus(); loadEvidenceCatalog(); loadCapabilities(); loadAgentRegistry() }} disabled={loadingSidecar || loadingEvidence || loadingCapabilities || loadingAgentRegistry}>
              {loadingSidecar || loadingEvidence || loadingCapabilities || loadingAgentRegistry ? <><i className="fas fa-spinner fa-spin" /> Refreshing</> : <><i className="fas fa-rotate" /> Refresh</>}
            </button>
          </div>
        </div>
        {agentRegistryError && <div className="records-alert records-alert-warning" role="alert">Specialist availability could not be verified. Deterministic analysis remains available; specialist handoff is disabled. {agentRegistryError}</div>}

        {hasANPRFamily && hasANPROperations && (
          <section className="records-family-command" aria-label="ANPR intelligence shortcuts">
            <div>
              <h3><i className="fas fa-camera" /> ANPR Intelligence</h3>
              <p>Audit exact sightings, camera coverage, plate variants, temporal co-observations, and consecutive-sighting timing with source-row citations.</p>
              <span className="records-family-command-note"><i className="fas fa-shield-halved" /> No ownership, passenger, association, road-route, or image-OCR claim is inferred.</span>
            </div>
            <div className="records-runtime-actions">
              <button className="btn btn-secondary btn-sm" type="button" onClick={() => runForensicQuery('show exact ANPR sightings', 'anpr_sightings', forensicTarget)} disabled={runningForensicQuery}>
                <i className="fas fa-list-ol" /> Exact sightings
              </button>
              <button className="btn btn-secondary btn-sm" type="button" onClick={() => runForensicQuery('show ANPR camera activity', 'anpr_camera_activity', '')} disabled={runningForensicQuery}>
                <i className="fas fa-chart-column" /> Camera activity
              </button>
              <button className="btn btn-secondary btn-sm" type="button" onClick={() => activatePromptChip({ query: 'show ANPR timeline for TARGET', template: 'anpr_timeline', targetRequired: true })}>
                <i className="fas fa-route" /> Prepare plate timeline
              </button>
            </div>
          </section>
        )}

        {hasSubscriberFamily && hasSubscriberOperations && (
          <section className="records-family-command records-family-command--subscriber" aria-label="Subscriber identity intelligence shortcuts">
            <div>
              <h3><i className="fas fa-address-card" /> Subscriber Identity Intelligence</h3>
              <p>Review exact subscriber identifiers, explicit validity windows, device/SIM observations, conflicts, and reuse candidates with source-row citations.</p>
              <span className="records-family-command-note"><i className="fas fa-user-shield" /> Full CNIC and names stay out of default results; no identity, ownership, current-control, SIM-swap, or fraud claim is inferred.</span>
            </div>
            <div className="records-runtime-actions">
              <button className="btn btn-secondary btn-sm" type="button" onClick={() => activatePromptChip({ query: 'look up subscriber identity observations for TARGET', template: 'subscriber_identity_lookup', targetRequired: true })} disabled={runningForensicQuery}>
                <i className="fas fa-id-card-clip" /> Prepare lookup
              </button>
              <button className="btn btn-secondary btn-sm" type="button" onClick={() => runForensicQuery('show subscriber status summary', 'subscriber_status_summary', '')} disabled={runningForensicQuery}>
                <i className="fas fa-chart-column" /> Status summary
              </button>
              <button className="btn btn-secondary btn-sm" type="button" onClick={() => runForensicQuery('show subscriber identity conflicts', 'subscriber_conflict_audit', '')} disabled={runningForensicQuery}>
                <i className="fas fa-triangle-exclamation" /> Conflict audit
              </button>
            </div>
          </section>
        )}

        {hasTowerFamily && hasTowerOperations && (
          <section className="records-family-command" aria-label="Tower and site reference intelligence shortcuts">
            <div>
              <h3><i className="fas fa-tower-cell" /> Tower &amp; Site Reference Intelligence</h3>
              <p>Review exact site aliases, reference history, supplied coordinates and uncertainty, status, conflicts, and time-aware CDR joins with citations.</p>
              <span className="records-family-command-note"><i className="fas fa-location-crosshairs" /> Reference coordinates are context only; no RF coverage, handset position, subscriber presence, home, route, ownership, or association is inferred.</span>
            </div>
            <div className="records-runtime-actions">
              <button className="btn btn-secondary btn-sm" type="button" onClick={() => activatePromptChip({ query: 'look up tower site reference observations for TARGET', template: 'tower_site_lookup', targetRequired: true })} disabled={runningForensicQuery}>
                <i className="fas fa-magnifying-glass-location" /> Prepare lookup
              </button>
              <button className="btn btn-secondary btn-sm" type="button" onClick={() => runForensicQuery('audit tower coordinates datums and uncertainty', 'tower_coordinate_audit', '')} disabled={runningForensicQuery}>
                <i className="fas fa-map-location-dot" /> Coordinate audit
              </button>
              <button className="btn btn-secondary btn-sm" type="button" onClick={() => runForensicQuery('show tower status summary', 'tower_status_summary', '')} disabled={runningForensicQuery}>
                <i className="fas fa-chart-column" /> Status summary
              </button>
            </div>
          </section>
        )}

        <div className="records-command">
          <div className="records-command-main">
            <div className={`records-primary-query ${boundCaseId ? '' : 'records-primary-query--unbound'}`}>
              {!boundCaseId && (
                <div className="records-field">
                  <label htmlFor="sidecar-collection">Case Collection</label>
                  <input id="sidecar-collection" className="input" list="records-collections" value={sidecarCollection} onChange={(e) => setSidecarCollection(e.target.value)} />
                </div>
              )}
              <div className="records-field">
                <label htmlFor="forensic-query">Natural Query</label>
                <input id="forensic-query" className="input" value={forensicQuery} onChange={(e) => setForensicQuery(e.target.value)} />
                <datalist id="forensic-recent-targets">
                  {recentTargets.map(target => <option key={target} value={target} />)}
                </datalist>
              </div>
              <div className="records-field">
                <label htmlFor="forensic-target">
                  Exact Target <span className="records-guidance">({selectedTargetInput?.required ? 'required' : 'optional'})</span>
                </label>
                <input id="forensic-target" className="input" list="forensic-recent-targets" value={forensicTarget} onChange={(e) => setForensicTarget(e.target.value)} placeholder="Phone, IP, subscriber, IMEI, or plate" />
                {selectedTargetInput?.accepted_kinds?.length > 0 && (
                  <span className="records-guidance">Accepted: {selectedTargetInput.accepted_kinds.join(', ')}</span>
                )}
              </div>
              <button className="btn btn-primary" type="button" onClick={() => runForensicQuery()} disabled={runningForensicQuery}>
                {runningForensicQuery ? <><i className="fas fa-spinner fa-spin" /> Analyzing</> : <><i className="fas fa-magnifying-glass-chart" /> Analyze</>}
              </button>
            </div>

            <div className="records-target-tools" aria-label="Exact case identifier tools">
              <button
                className="btn btn-secondary btn-sm"
                type="button"
                onClick={() => runForensicQuery('show available entities', 'entity_activity', '', 100)}
                disabled={runningForensicQuery}
              >
                <i className="fas fa-fingerprint" /> Discover exact case identifiers
              </button>
              {recentTargets.length > 0 && <span>Select an authorized identifier:</span>}
              {recentTargets.map(target => (
                <button key={target} className="btn btn-secondary btn-sm" type="button" onClick={() => setForensicTarget(target)}>
                  {target}
                </button>
              ))}
              {maskedTargetHints.length > 0 && (
                <div className="records-target-hints">
                  Masked coverage hints (not executable): {maskedTargetHints.join(', ')}. Use discovery to retrieve exact authorized values.
                </div>
              )}
            </div>

            <div className="records-results-toolbar">
              <div className="records-guidance">Auto planning is recommended. It selects deterministic Records SQL, cited Knowledge Base retrieval, or both.</div>
              <button className="btn btn-secondary btn-sm" type="button" onClick={() => setAdvancedQueryOpen(open => !open)}>
                <i className={`fas ${advancedQueryOpen ? 'fa-chevron-up' : 'fa-sliders'}`} /> {advancedQueryOpen ? 'Hide advanced options' : 'Advanced options'}
              </button>
            </div>

            {advancedQueryOpen && (
              <div className="records-advanced-query">
                <div className="records-field">
                  <label htmlFor="forensic-template">Deterministic Template</label>
                  <select id="forensic-template" className="input" value={forensicTemplate} onChange={(e) => setForensicTemplate(e.target.value)}>
                    <option value="">Auto</option>
                    {templates.map(template => <option key={template.name} value={template.name}>{template.name}</option>)}
                  </select>
                </div>
                <div className="records-field">
                  <label htmlFor="forensic-model">Optional Explanation Model</label>
                  <select id="forensic-model" className="input" value={forensicSynthesisModel} onChange={(e) => setForensicSynthesisModel(e.target.value)}>
                    <option value="">Deterministic only (recommended)</option>
                    {modelOptions.map(model => <option key={model} value={model}>{model}</option>)}
                  </select>
                  <span className="records-guidance">Only role-compatible chat models are listed. Exact facts and citations never come from this model.</span>
                </div>
                <div className="records-field">
                  <label htmlFor="forensic-limit">Maximum Rows</label>
                  <input id="forensic-limit" className="input" type="number" min="1" max="100" value={forensicLimit} onChange={(e) => setForensicLimit(e.target.value)} />
                </div>
                <div className="records-field">
                  <label htmlFor="forensic-date-from">Start Time <span className="records-guidance">(inclusive)</span></label>
                  <input ref={forensicDateFromRef} id="forensic-date-from" className="input" type="datetime-local" value={forensicDateFrom} onChange={(e) => setForensicDateFrom(e.target.value)} />
                </div>
                <div className="records-field">
                  <label htmlFor="forensic-date-to">End Time <span className="records-guidance">(exclusive)</span></label>
                  <input ref={forensicDateToRef} id="forensic-date-to" className="input" type="datetime-local" value={forensicDateTo} onChange={(e) => setForensicDateTo(e.target.value)} />
                </div>
              </div>
            )}

            <div className="records-query-presets">
              {promptChips.slice(0, 6).map(chip => (
                <button
                  key={`${chip.template}:${chip.query}`}
                  className="btn btn-secondary btn-sm"
                  type="button"
                  onClick={() => activatePromptChip(chip)}
                >
                  {chip.label}
                </button>
              ))}
            </div>

            {advancedQueryOpen && <div className="records-runtime-panel">
              <div>
                <strong>Runtime Field Queries</strong>
                <span>SQL-backed examples for raw analyst filters, normalized fields, and exact row provenance.</span>
              </div>
              <div className="records-runtime-actions">
                {runtimeQueryExamples.map(example => (
                  <button
                    key={example.query}
                    className="btn btn-secondary btn-sm"
                    type="button"
                    onClick={() => {
                      setForensicQuery(example.query)
                      setForensicTemplate('canonical_records')
                      runForensicQuery(example.query, 'canonical_records')
                    }}
                    title={example.query}
                  >
                    <i className={`fas ${example.icon}`} /> {example.label}
                  </button>
                ))}
              </div>
            </div>}
          </div>

          <div className="records-case-strip">
            <div className="records-kpi">
              <span>Accepted Rows</span>
              <strong>{sidecarMetric(sidecarStatus, 'accepted_rows').toLocaleString()}</strong>
            </div>
            <div className="records-kpi">
              <span>Duplicates</span>
              <strong>{sidecarMetric(sidecarStatus, 'duplicate_rows').toLocaleString()}</strong>
            </div>
            <div className="records-kpi">
              <span>KB Assets</span>
              <strong>{sidecarMetric(sidecarStatus, 'kb_assets_total').toLocaleString()}</strong>
            </div>
            <div className="records-kpi">
              <span>Families</span>
              <strong>{numericValue(capabilitySummary.families_total || (sidecarStatus?.record_families || []).length).toLocaleString()}</strong>
            </div>
          </div>
        </div>

        {sidecarError && <div className="records-alert records-alert-warning">{sidecarError}</div>}
        {(sidecarStatus?.missing_kb_assets || []).length > 0 && (
          <div className="records-alert records-alert-warning">
            Missing KB assets: {sidecarStatus.missing_kb_assets.slice(0, 5).join(', ')}
          </div>
        )}

        {templatesError && <div className="records-alert records-alert-warning">{templatesError}</div>}
        {capabilitiesError && <div className="records-alert records-alert-warning">Capability catalog: {capabilitiesError}</div>}

        <div className="records-section-toggle">
          <div>
            <strong>Case evidence and capabilities</strong>
            <div className="records-guidance">Open when you need source status, processing history, provenance, or pending-adapter details.</div>
          </div>
          <button className="btn btn-secondary" type="button" onClick={() => setCaseEvidenceOpen(open => !open)}>
            <i className={`fas ${caseEvidenceOpen ? 'fa-chevron-up' : 'fa-folder-tree'}`} /> {caseEvidenceOpen ? 'Hide evidence review' : 'Review evidence'}
          </button>
        </div>

        {caseEvidenceOpen && <>
        <div className="records-evidence-panel">
          <div className="records-evidence-header">
            <div>
              <h3 style={{ fontSize: '0.95rem', margin: 0 }}>Query Capability Map</h3>
              <div className="records-guidance">Collection-aware truth for all required evidence families. Pending adapters and models are shown explicitly and are blocked from simulated answers.</div>
            </div>
            <div className="records-helper-row">
              <span className="records-status-pill" data-tone="ready"><i className="fas fa-database" /> {numericValue(capabilitySummary.queryable).toLocaleString()} queryable</span>
              <span className="records-status-pill" data-tone="input"><i className="fas fa-book-open" /> {numericValue(capabilitySummary.semantic_only).toLocaleString()} semantic</span>
              <span className="records-status-pill" data-tone="review"><i className="fas fa-flask" /> {numericValue(capabilitySummary.registered_pending).toLocaleString()} pending</span>
            </div>
          </div>
          <div className="records-workspace-body">
            {capabilityFamilies.length > 0 ? (
              <div className="records-family-list">
                {capabilityFamilies.map(family => (
                  <div className="records-family" key={family.id} title={family.availability_reason}>
                    <strong>{family.label}</strong>
                    <span className="records-status-pill" data-tone={family.availability === 'queryable' ? 'ready' : family.availability === 'no_data' ? 'empty' : 'review'}>{family.availability}</span>
                    <span>{numericValue(family.indexed_records).toLocaleString()} indexed · {numericValue(family.registered_evidence).toLocaleString()} evidence</span>
                    <span>{family.formats?.join(', ')}</span>
                    <span>{family.availability_reason}</span>
                    {(family.suggested_queries || []).slice(0, 1).map(query => (
                      <button key={query} className="btn btn-secondary btn-sm" type="button" onClick={() => { setForensicQuery(query); setForensicTemplate('') }}>
                        Try: {query}
                      </button>
                    ))}
                  </div>
                ))}
              </div>
            ) : (
              <div className="records-alert">{loadingCapabilities ? 'Loading collection capabilities...' : 'No capability catalog was returned.'}</div>
            )}
          </div>
        </div>

        <div className="records-evidence-panel">
          <div className="records-evidence-header">
            <div>
              <h3 style={{ fontSize: '0.95rem', margin: 0 }}>Evidence Catalog</h3>
              <div className="records-guidance">Unified registry for raw uploads, Knowledge Base entries, structured batches, and downstream processors.</div>
            </div>
            <div className="records-helper-row">
              <span className="records-status-pill" data-tone={sidecarMetric({ summary: evidenceSummary }, 'evidence_failed') > 0 ? 'review' : 'ready'}>
                <i className="fas fa-folder-tree" /> {numericValue(evidenceSummary.evidence_total).toLocaleString()} evidence
              </span>
              <button className="btn btn-secondary btn-sm" type="button" onClick={loadEvidenceCatalog} disabled={loadingEvidence}>
                {loadingEvidence ? <><i className="fas fa-spinner fa-spin" /> Loading</> : <><i className="fas fa-rotate" /> Catalog</>}
              </button>
            </div>
          </div>
          <div className="records-evidence-body">
            <div className="records-panel">
              <div className="records-kpi-grid">
                <div className="records-kpi"><span>Completed</span><strong>{numericValue(evidenceSummary.completed || evidenceSummary.evidence_completed).toLocaleString()}</strong></div>
                <div className="records-kpi"><span>In Flight</span><strong>{numericValue(evidenceSummary.processing || evidenceSummary.queued || evidenceSummary.evidence_in_flight).toLocaleString()}</strong></div>
                <div className="records-kpi"><span>Failed</span><strong>{numericValue(evidenceSummary.failed || evidenceSummary.evidence_failed).toLocaleString()}</strong></div>
                <div className="records-kpi"><span>Total Size</span><strong>{formatFileSize(numericValue(evidenceSummary.total_size_bytes))}</strong></div>
              </div>
              <div className="records-table-tools">
                <div className="records-field">
                  <label htmlFor="evidence-search">Search Evidence</label>
                  <input id="evidence-search" className="input" value={evidenceSearch} onChange={(e) => setEvidenceSearch(e.target.value)} placeholder="filename, hash, evidence id, KB entry" />
                </div>
                <div className="records-field">
                  <label htmlFor="evidence-status">Status</label>
                  <select id="evidence-status" className="input" value={evidenceStatusFilter} onChange={(e) => setEvidenceStatusFilter(e.target.value)}>
                    <option value="">Any status</option>
                    {evidenceStatusOptions.map(status => <option key={status} value={status}>{status}</option>)}
                  </select>
                </div>
                <div className="records-field">
                  <label htmlFor="evidence-modality">Modality</label>
                  <select id="evidence-modality" className="input" value={evidenceModalityFilter} onChange={(e) => setEvidenceModalityFilter(e.target.value)}>
                    <option value="">Any modality</option>
                    {evidenceModalityOptions.map(modality => <option key={modality} value={modality}>{modality}</option>)}
                  </select>
                </div>
                <button className="btn btn-secondary" type="button" onClick={() => { setEvidenceSearch(''); setEvidenceStatusFilter(''); setEvidenceModalityFilter('') }}>
                  <i className="fas fa-filter-circle-xmark" /> Clear
                </button>
                <button className="btn btn-secondary" type="button" onClick={() => runForensicQuery('which files were ingested?', 'source_file_audit')}>
                  <i className="fas fa-list-check" /> Audit
                </button>
              </div>
              {evidenceError && <div className="records-alert records-alert-warning">{evidenceError}</div>}
              {filteredEvidenceItems.length > 0 ? (
                <div className="records-evidence-list">
                  {filteredEvidenceItems.slice(0, 50).map(item => (
                    <button
                      key={item.evidence_id || `${item.source_file}-${item.sha256}`}
                      type="button"
                      className="records-evidence-item"
                      onClick={() => loadEvidenceDetail(item)}
                    >
                      <div>
                        <div className="records-evidence-title">{item.source_file || item.original_filename || item.evidence_id}</div>
                        <div className="records-evidence-meta">
                          <span>{item.detected_type || 'unknown'}</span>
                          <span>{item.modality || 'unknown'}</span>
                          {item.case_id && <span>{item.case_id}</span>}
                          {item.records_batch_id && <span>batch {item.records_batch_id}</span>}
                        </div>
                      </div>
                      <span className="records-status-pill" data-tone={evidenceTone(item.processing_status)}>
                        {item.processing_status || 'unknown'}
                      </span>
                    </button>
                  ))}
                </div>
              ) : (
                <div className="records-alert">
                  {loadingEvidence ? 'Loading evidence catalog...' : 'No evidence items matched the current collection and filters.'}
                </div>
              )}
            </div>
            <div className="records-evidence-detail">
              <div className="records-results-toolbar">
                <strong>Evidence Detail</strong>
                {loadingEvidenceDetail && <span className="records-status-pill" data-tone="input"><i className="fas fa-spinner fa-spin" /> Loading</span>}
              </div>
              {selectedEvidenceItem ? (
                <>
                  <div>
                    <div className="records-evidence-title">{selectedEvidenceItem.source_file || selectedEvidenceItem.original_filename}</div>
                    <div className="records-evidence-meta">
                      <span>{selectedEvidenceItem.detected_type}</span>
                      <span>{selectedEvidenceItem.processing_route}</span>
                      <span>{formatFileSize(numericValue(selectedEvidenceItem.size_bytes))}</span>
                    </div>
                  </div>
                  <div className="records-guidance">
                    <strong>ID:</strong> <code>{selectedEvidenceItem.evidence_id}</code>
                  </div>
                  {selectedEvidenceItem.kb_entry_ref && (
                    <div className="records-guidance">
                      <strong>KB Entry:</strong> <code>{selectedEvidenceItem.kb_entry_ref}</code>
                    </div>
                  )}
                  <div className="records-kpi-grid">
                    <div className="records-kpi"><span>Jobs</span><strong>{selectedEvidenceJobs.length}</strong></div>
                    <div className="records-kpi"><span>KB Assets</span><strong>{selectedEvidenceAssets.length}</strong></div>
                    <div className="records-kpi"><span>Preview Rows</span><strong>{selectedEvidenceRows.length}</strong></div>
                    <div className="records-kpi"><span>Warnings</span><strong>{Array.isArray(selectedEvidenceItem.warnings) ? selectedEvidenceItem.warnings.length : 0}</strong></div>
                  </div>
                  <button className="btn btn-secondary btn-sm" type="button" onClick={reprocessSelectedEvidence} disabled={reprocessingEvidence}>
                    {reprocessingEvidence ? <><i className="fas fa-spinner fa-spin" /> Queuing immutable retry</> : <><i className="fas fa-rotate" /> Reprocess Evidence</>}
                  </button>
                  {selectedEvidenceJobs.slice(0, 3).map(job => (
                    <div className="records-provenance" key={job.job_id}>
                      <strong>{job.status} ingest job</strong>
                      <span>{job.record_type} - {numericValue(job.accepted_rows).toLocaleString()} accepted rows</span>
                      <span>Attempt {numericValue(job.attempt_count).toLocaleString()} of {numericValue(job.max_attempts).toLocaleString()} · generation {numericValue(job.reprocess_generation).toLocaleString()}</span>
                      <span>{job.job_id}</span>
                    </div>
                  ))}
                  {selectedEvidenceRows.slice(0, 3).map((row, index) => (
                    <div className="records-provenance" key={`${row.record_id}-${index}`}>
                      <strong>{row.record_type || 'record'} row {compactValue(row.row_number)}</strong>
                      <span>{compactValue(row.timestamp)}</span>
                      <span>{compactValue(row.primary_target || row.secondary_target || row.source_file)}</span>
                    </div>
                  ))}
                </>
              ) : (
                <div className="records-guidance">
                  Select an evidence item to inspect linked ingest jobs, KB assets, canonical row previews, and entity rollups.
                </div>
              )}
            </div>
          </div>
        </div>
        </>}

        {forensicResult ? (
          <div className="records-result-shell" ref={forensicResultRef} aria-live="polite">
            <div>
              <div className="records-workspace">
                <div className="records-workspace-header">
                  <div className="records-result-heading">
                    <div>
                      <span className="records-status-pill" data-tone={statusTone(resultStatus)}>
                        <i className="fas fa-circle-check" /> {statusLabel(resultStatus)}
                      </span>
                      <h2>{resultTitle}</h2>
                      <div className="records-result-meta">
                        <span>{forensicRows.length.toLocaleString()} exact row{forensicRows.length === 1 ? '' : 's'}</span>
                        <span>{enterpriseProvenance.length.toLocaleString()} source reference{enterpriseProvenance.length === 1 ? '' : 's'}</span>
                        <span>{(forensicResult.route || []).includes('records_sql') ? 'Deterministic SQL' : (forensicResult.route || []).join(' + ') || 'Bounded analysis'}</span>
                        {resultLatency > 0 && <span>{resultLatency.toLocaleString()} ms</span>}
                      </div>
                    </div>
                    <div className="records-inline-actions">
                      <button className="btn btn-secondary btn-sm" type="button" onClick={() => exportForensic('csv')} disabled={filteredForensicRows.length === 0}>
                        <i className="fas fa-file-csv" /> Export rows
                      </button>
                      <button className="btn btn-secondary btn-sm" type="button" onClick={() => exportForensic('json')}>
                        <i className="fas fa-shield-halved" /> Export audit
                      </button>
                    </div>
                  </div>
                  <div className="records-workspace-tabs">
                    {[
                      ['overview', 'Analyst summary'],
                      ['grid', 'All records'],
                      ['timeline', 'Timeline'],
                      ['evidence', 'Evidence'],
                      ['audit', 'Audit details'],
                    ].map(([key, label]) => (
                      <button
                        key={key}
                        type="button"
                        className={`btn btn-sm ${forensicWorkspaceTab === key ? 'btn-primary' : 'btn-secondary'}`}
                        onClick={() => setForensicWorkspaceTab(key)}
                      >
                        {label}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="records-workspace-body">
                  {forensicWorkspaceTab === 'overview' && (
                    <>
                      {forensicResult.capability?.status === 'unavailable' && (
                        <div className="records-alert records-alert-warning">
                          {forensicResult.capability.explanation} {(forensicResult.capability.missing_capabilities || []).join('; ')}
                        </div>
                      )}
                      {forensicResult.answer?.clarification_required && (
                        <div className="records-alert records-alert-warning">{forensicResult.answer.clarification}</div>
                      )}
                      <div className="records-answer-hero">
                        <div className="records-answer-label"><i className="fas fa-sparkles" /> Analyst answer</div>
                        <div className="records-brief">
                          {primaryBrief || 'No plain-language summary was returned. Review the exact rows and evidence below.'}
                        </div>
                      </div>

                      <div className="records-metric-grid" aria-label="Result summary">
                        <div className="records-kpi"><span>Exact rows</span><strong>{forensicRows.length.toLocaleString()}</strong></div>
                        <div className="records-kpi"><span>Evidence references</span><strong>{enterpriseProvenance.length.toLocaleString()}</strong></div>
                        <div className="records-kpi"><span>Deterministic findings</span><strong>{analystClaims.length.toLocaleString()}</strong></div>
                        <div className="records-kpi"><span>Execution time</span><strong>{resultLatency > 0 ? `${resultLatency.toLocaleString()} ms` : 'Not reported'}</strong></div>
                      </div>

                      {enterpriseVisualizations.length > 0 && (
                        <section>
                          <div className="records-section-heading"><h3>Visual analysis</h3><span>Derived only from returned structured rows</span></div>
                          <div className="records-visual-grid">{enterpriseVisualizations.map(visualization => <TypedForensicVisualization key={visualization.id} visualization={visualization} />)}</div>
                        </section>
                      )}

                      {analystClaims.length > 0 && (
                        <section>
                          <div className="records-section-heading"><h3>Key findings</h3><span>{analystClaims.length} cited deterministic finding{analystClaims.length === 1 ? '' : 's'}</span></div>
                          <div className="records-finding-grid">
                            {analystClaims.map((claim, index) => (
                              <article className="records-finding" key={index}>
                                <strong>{claim.source || `Finding ${index + 1}`}</strong>
                                <p>{claim.claim}</p>
                                {claim.measureSummary && <small>{claim.measureSummary}</small>}
                              </article>
                            ))}
                          </div>
                        </section>
                      )}

                      {resultPreviewRows.length > 0 && previewColumnDefs.length > 0 && (
                        <section>
                          <div className="records-section-heading">
                            <h3>Exact result preview</h3>
                            <button className="btn btn-secondary btn-sm" type="button" onClick={() => setForensicWorkspaceTab('grid')}>Review all {forensicRows.length.toLocaleString()} rows</button>
                          </div>
                          <div className="records-results-wrap">
                            <table className="records-results-table">
                              <thead><tr>{previewColumnDefs.map(column => <th key={column.key}>{column.header}</th>)}</tr></thead>
                              <tbody>{resultPreviewRows.map((row, index) => <tr key={index}>{previewColumnDefs.map(column => <td key={column.key} title={compactValue(row[column.key])}>{compactValue(row[column.key])}</td>)}</tr>)}</tbody>
                            </table>
                          </div>
                        </section>
                      )}

                      {resultStatus === 'no_results' && coverageSummary.length > 0 && (
                        <section>
                          <div className="records-section-heading"><h3>Coverage checked</h3><span>A negative result is not proof of absence</span></div>
                          <div className="records-coverage-grid">
                            {coverageSummary.map(item => <div className="records-kpi" key={item.label}><span>{item.label}</span><strong>{compactValue(item.value)}</strong></div>)}
                          </div>
                        </section>
                      )}

                      {enterpriseActions.length > 0 && (
                        <section>
                          <div className="records-section-heading"><h3>Recommended next checks</h3><span>Run with the same case scope</span></div>
                          <div className="records-action-grid">
                            {enterpriseActions.map((action, index) => (
                              <button key={`${action.template}-${action.query}-${index}`} type="button" className="records-action" onClick={() => activatePromptChip({ query: action.query || '', template: action.template || '', targetRequired: /\bTARGET\b/i.test(action.query || '') })}>
                                <strong>{action.label || templateTitle(action.template || action.query)}</strong>
                                <span>{action.reason || action.query}</span>
                              </button>
                            ))}
                          </div>
                        </section>
                      )}

                      {enterpriseLimitations.length > 0 && (
                        <details className="records-audit-details">
                          <summary><i className="fas fa-shield-halved" /> Evidence limits and cautions ({enterpriseLimitations.length})</summary>
                          <div>{enterpriseLimitations.map((limitation, index) => <div className="records-alert records-alert-warning" key={index}>{limitation}</div>)}</div>
                        </details>
                      )}
                    </>
                  )}

                  {forensicWorkspaceTab === 'grid' && (
                    <>
                      <div className="records-table-tools">
                        <div className="records-field">
                          <label htmlFor="forensic-search">Search Results</label>
                          <input id="forensic-search" className="input" value={forensicSearch} onChange={(e) => setForensicSearch(e.target.value)} />
                        </div>
                        <div className="records-field">
                          <label htmlFor="forensic-sort">Sort By</label>
                          <select id="forensic-sort" className="input" value={forensicSort} onChange={(e) => setForensicSort(e.target.value)}>
                            <option value="">Default</option>
                            {forensicColumnDefs.map(column => <option key={column.key} value={column.key}>{column.header}</option>)}
                          </select>
                        </div>
                        <div className="records-field">
                          <label htmlFor="forensic-sort-dir">Direction</label>
                          <select id="forensic-sort-dir" className="input" value={forensicSortDirection} onChange={(e) => setForensicSortDirection(e.target.value)}>
                            <option value="asc">Asc</option>
                            <option value="desc">Desc</option>
                          </select>
                        </div>
                        <button className="btn btn-secondary" type="button" onClick={() => exportForensic('csv')} disabled={filteredForensicRows.length === 0}>
                          <i className="fas fa-file-csv" /> CSV
                        </button>
                        <button className="btn btn-secondary" type="button" onClick={() => setAuditOpen(true)}>
                          <i className="fas fa-code" /> Audit
                        </button>
                      </div>

                      {filteredForensicRows.length > 0 ? (
                        <div className="records-results-wrap">
                          <table className="records-results-table">
                            <thead>
                              <tr>{forensicColumnDefs.map(column => <th key={column.key}>{column.header}</th>)}</tr>
                            </thead>
                            <tbody>
                              {filteredForensicRows.map((row, index) => (
                                <tr key={index}>
                                  {forensicColumnDefs.map(column => (
                                    <td key={column.key} className={isLongValue(row[column.key]) ? 'records-cell-long' : ''} title={compactValue(row[column.key])}>
                                      {compactValue(row[column.key])}
                                    </td>
                                  ))}
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>
                      ) : (
                        <div className="records-guidance">
                          {forensicResult.answer?.records_status || forensicResult.answer?.evidence_status || 'No rows returned for the current search.'}
                        </div>
                      )}
                    </>
                  )}

                  {forensicWorkspaceTab === 'timeline' && (
                    timelineRows.length > 0 ? (
                      <div className="records-timeline">
                        {timelineRows.map((row, index) => (
                          <div className="records-timeline-item" key={index}>
                            <strong>{compactValue(rowTimestamp(row))}</strong>
                            <span>{compactValue(rowTitle(row))}</span>
                            <span>{compactValue(row.location || row.primary_entity || row.source_file || row.entity_value)}</span>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="records-alert">
                        This result does not include timestamped rows. Use a timeline, movement, source-record, or activity template when chronological review is needed.
                      </div>
                    )
                  )}

                  {forensicWorkspaceTab === 'evidence' && (
                    <>
                      <div className="records-section-heading"><h3>Evidence and provenance</h3><span>{enterpriseProvenance.length} reference{enterpriseProvenance.length === 1 ? '' : 's'}</span></div>
                      {enterpriseProvenance.length > 0 ? (
                        <div className="records-provenance-list">
                          {enterpriseProvenance.map((item, index) => (
                            <div className="records-provenance" key={index}>
                              <strong>{item.source || 'Source'}</strong>
                              {item.source_file && <span>Source: {item.source_file}</span>}
                              {item.source_entry && <span>Locator: {item.source_entry}</span>}
                              {item.citation && <span>Citation: {item.citation}</span>}
                              {!item.source_file && !item.source_entry && !item.citation && <span>No source label returned</span>}
                              {item.row_number !== undefined && <span>Row: {compactValue(item.row_number)}</span>}
                              {item.timestamp && <span>Time: {compactValue(item.timestamp)}</span>}
                              {item.preview && <span>{compactValue(item.preview)}</span>}
                            </div>
                          ))}
                        </div>
                      ) : (
                        <div className="records-alert">
                          No row-level provenance was returned for this query. Source-file and KB citation templates provide the strongest audit trail.
                        </div>
                      )}
                    </>
                  )}

                  {forensicWorkspaceTab === 'audit' && (
                    <>
                      <div className="records-section-heading"><h3>Execution and coverage</h3><span>Technical detail for review and export</span></div>
                      <div className="records-planner">
                        <span className="records-chip">{forensicResult.intent}</span>
                        <span className="records-chip">{forensicResult.template}</span>
                        {(forensicResult.route || []).map(route => <span className="records-chip" key={route}>{route}</span>)}
                        {(forensicResult.planner?.field_hints || []).map(hint => <span className="records-chip" key={hint}>{hint}</span>)}
                        {forensicResult.planner?.date_from && <span className="records-chip">From {forensicResult.planner.date_from}</span>}
                        {forensicResult.planner?.date_to && <span className="records-chip">To {forensicResult.planner.date_to}</span>}
                      </div>
                      {enterpriseMetrics.length > 0 && (
                        <div className="records-metric-grid">
                          {enterpriseMetrics.slice(0, 8).map((metric, index) => <div className="records-kpi" key={`${metric.label}-${index}`}><span>{metric.label}</span><strong>{compactValue(metric.value)}</strong>{metric.source && <span>{metric.source}</span>}</div>)}
                        </div>
                      )}
                      {coverageSummary.length > 0 ? (
                        <div className="records-coverage-grid">
                          {coverageSummary.map(item => (
                            <div className="records-kpi" key={item.label}>
                              <span>{item.label}</span>
                              <strong>{compactValue(item.value)}</strong>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <div className="records-alert">
                          No coverage metadata was returned for this query. Run a date-bound or no-result workflow to inspect collection bounds.
                        </div>
                      )}
                      {(enterpriseCoverage.record_families_present || []).length > 0 && (
                        <div className="records-family-list">
                          {enterpriseCoverage.record_families_present.map(family => (
                            <div className="records-family" key={family.record_type}>
                              <strong>{family.record_type}</strong>
                              <span>{numericValue(family.count).toLocaleString()} indexed</span>
                            </div>
                          ))}
                        </div>
                      )}
                      <div className="records-kpi-grid">
                        <div className="records-kpi"><span>Jobs</span><strong>{sidecarMetric(sidecarStatus, 'jobs_total').toLocaleString()}</strong></div>
                        <div className="records-kpi"><span>Failed jobs</span><strong>{sidecarMetric(sidecarStatus, 'failed_jobs').toLocaleString()}</strong></div>
                        <div className="records-kpi"><span>Rejected rows</span><strong>{sidecarMetric(sidecarStatus, 'rejected_rows').toLocaleString()}</strong></div>
                        <div className="records-kpi"><span>Missing KB assets</span><strong>{(sidecarStatus?.missing_kb_assets || []).length.toLocaleString()}</strong></div>
                      </div>
                      {enterpriseLimitations.length > 0 && (
                        <div className="records-provenance-list">
                          {enterpriseLimitations.map((limitation, index) => (
                            <div className="records-alert records-alert-warning" key={index}>{limitation}</div>
                          ))}
                        </div>
                      )}
                      {(sidecarStatus?.recent_errors || []).length > 0 ? (
                        <div className="records-provenance-list">
                          {sidecarStatus.recent_errors.slice(0, 6).map((error, index) => (
                            <div className="records-provenance" key={index}>
                              <strong>{error.source_file || error.job_id || 'Ingest error'}</strong>
                              <span>{error.error_message || error.message || compactValue(error)}</span>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <div className="records-guidance">No recent ingest error rows were returned by the status endpoint.</div>
                      )}
                    </>
                  )}
                </div>
              </div>
            </div>
          </div>
        ) : (
          <div className="records-workspace">
            <div className="records-workspace-body">
              <span className="records-status-pill" data-tone="neutral"><i className="fas fa-circle-play" /> Ready to analyze</span>
              <div className="records-brief">
                Start with a source audit, case readiness check, entity coverage, or ask a natural-language records question. The answer area will show the decision, exact rows, provenance, and next actions here.
              </div>
              <div className="records-action-grid">
                {promptChips.slice(0, 4).map(chip => (
                  <button
                    key={`${chip.template}:${chip.query}`}
                    className="records-action"
                    type="button"
                    onClick={() => activatePromptChip(chip)}
                  >
                    <strong>{chip.label}</strong>
                    <span>{chip.query}</span>
                  </button>
                ))}
              </div>
            </div>
          </div>
        )}

        {templates.length > 0 && (
          <div className="records-panel">
            <div className="records-results-toolbar">
              <div>
                <h3 style={{ fontSize: '0.95rem', margin: 0 }}>Template Discovery</h3>
                <div className="records-guidance">Use this when you want to choose the exact deterministic workflow.</div>
              </div>
              <button className="btn btn-secondary btn-sm" type="button" onClick={() => setTemplateExplorerOpen(open => !open)}>
                <i className={`fas ${templateExplorerOpen ? 'fa-chevron-up' : 'fa-chevron-down'}`} /> {templateExplorerOpen ? 'Hide' : 'Show'} Templates
              </button>
            </div>
            {templateExplorerOpen && (
              <>
                <div className="records-template-tabs">
                  {templateCategories.map(category => (
                    <button
                      key={category}
                      type="button"
                      className={`btn btn-sm ${templateCategoryFilter === category ? 'btn-primary' : 'btn-secondary'}`}
                      onClick={() => setTemplateCategoryFilter(category)}
                    >
                      {category}
                    </button>
                  ))}
                </div>
                <div className="records-template-grid">
                  {visibleTemplates.map(template => {
                    const targetInput = templateTargetInput(template)
                    const example = materializeTemplateExample(template, forensicTarget, forensicDateFrom, forensicDateTo, discoveredTargets)
                    return (
                    <article key={template.name} className="records-template-card">
                      <strong>{templateTitle(template.name)}</strong>
                      <span>{template.description || 'Deterministic records workflow.'}</span>
                      <div className="records-planner">
                        <span className="records-chip">{templateCategory(template)}</span>
                        {targetInput?.required && <span className="records-chip">Exact target required</span>}
                        {template.route && <span className="records-chip">{Array.isArray(template.route) ? template.route.join(' + ') : template.route}</span>}
                      </div>
                      {(template.calculation || targetInput || template.output_description) && (
                        <details>
                          <summary>Inputs &amp; calculation</summary>
                          <div className="records-template-detail">
                            {targetInput && <div><strong>Target:</strong> {targetInput.required ? 'Required' : 'Optional'} — {(targetInput.accepted_kinds || []).join(', ') || 'exact case identifier'}</div>}
                            {template.measures?.length > 0 && <div><strong>Measures:</strong> {template.measures.join(', ')}</div>}
                            {template.group_by?.length > 0 && <div><strong>Grouped by:</strong> {template.group_by.join(', ')}</div>}
                            {template.calculation && <div><strong>Calculation:</strong> {template.calculation}</div>}
                            {template.output_description && <div><strong>Result:</strong> {template.output_description}</div>}
                            <div><strong>Example:</strong> <code>{example}</code></div>
                            {template.limitations?.length > 0 && <div><strong>Limits:</strong> {template.limitations.join(' ')}</div>}
                          </div>
                        </details>
                      )}
                      <button
                        className="btn btn-secondary btn-sm"
                        type="button"
                        onClick={() => {
                          const currentEntry = discoveredTargets.find(entry => entry.value === forensicTarget)
                          if (currentEntry && !targetEntryMatchesTemplate(currentEntry, template)) {
                            setForensicTarget('')
                            addToast('Target cleared because this workflow requires a different identifier type.', 'warning')
                          }
                          setForensicTemplate(template.name)
                          setForensicQuery(templateQuery(template))
                        }}
                      >
                        Use workflow
                      </button>
                    </article>
                    )
                  })}
                </div>
              </>
            )}
          </div>
        )}
      </div>

      {showSeedLab && (
      <div className="card records-panel" style={{ marginBottom: 'var(--spacing-lg)' }}>
        <h2 style={{ fontSize: '1rem', margin: 0 }}>Seed Lab</h2>
        <div className="records-seed-grid">
          <div className="records-field">
            <label htmlFor="seed-collection">Seed Collection</label>
            <input id="seed-collection" className="input" value={seedCollection} onChange={(e) => setSeedCollection(e.target.value)} />
          </div>
          <div className="records-field">
            <label htmlFor="seed-rows">CDR Rows</label>
            <input id="seed-rows" className="input" type="number" min="100" max="25000" step="100" value={seedRows} onChange={(e) => setSeedRows(e.target.value)} />
          </div>
          <button className="btn btn-secondary" type="button" onClick={prepareSeedPack} disabled={seeding || uploading}>
            <i className="fas fa-flask" /> Generate Files
          </button>
          <button className="btn btn-primary" type="button" onClick={ingestSeedPack} disabled={seeding || uploading}>
            {seeding ? <><i className="fas fa-spinner fa-spin" /> Seeding...</> : <><i className="fas fa-database" /> Generate + Ingest</>}
          </button>
        </div>
        <div className="records-checklist">
          <div className="records-check">
            <strong>KB</strong>
            Policy and case-note seed documents are uploaded with the collection so RAG has narrative evidence.
          </div>
          <div className="records-check">
            <strong>Records</strong>
            CDR, ANPR, IPDR, and access-log files use canonical fields for exact query, aggregate, and correlation tests.
          </div>
          <div className="records-check">
            <strong>Agents</strong>
            Enable KB and Forensic Records in the agent form, set collection to this seed collection, then ask hybrid questions.
          </div>
        </div>
      </div>
      )}

      <div className="card records-drawer-toggle" style={{ marginBottom: 'var(--spacing-lg)' }}>
        <div>
          <h2 style={{ fontSize: '1rem', margin: 0 }}>Data Management</h2>
          <div className="records-guidance">{boundCollectionId
            ? `Upload records, inspect batches, or run exact query tools within ${boundCollectionId}.`
            : 'Upload records, inspect batches, or run manual exact query tools when operational maintenance is needed.'}</div>
        </div>
        <button className="btn btn-secondary" type="button" onClick={() => setDataManagementOpen(open => !open)}>
          <i className={`fas ${dataManagementOpen ? 'fa-chevron-up' : 'fa-chevron-down'}`} /> {dataManagementOpen ? 'Hide' : 'Open'}
        </button>
      </div>

      {dataManagementOpen && (
      <div className="records-grid">
        <div className="records-panel">
          <form className="card records-panel" onSubmit={handleUpload}>
            <h2 style={{ fontSize: '1rem', margin: 0 }}>Upload Structured Records</h2>
            <div className="records-field">
              <label htmlFor="records-file">Files</label>
              <input id="records-file" className="input" type="file" multiple accept=".csv,.tsv,.xlsx,.json,.jsonl,.ndjson,.parquet,.txt,.log" onChange={(e) => setFiles(Array.from(e.target.files || []))} />
            </div>
            {files.length > 0 && (
              <div className="records-file-list">
                {files.slice(0, 8).map(selectedFile => (
                  <div className="records-file-row" key={`${selectedFile.name}-${selectedFile.size}`}>
                    <span title={selectedFile.name}>{selectedFile.name}</span>
                    <span>{formatFileSize(selectedFile.size)}</span>
                  </div>
                ))}
                {files.length > 8 && <div className="records-guidance">+{files.length - 8} more files selected.</div>}
              </div>
            )}
            <div className="records-form-grid">
              <div className="records-field">
                <label htmlFor="records-collection">{boundCollectionId ? 'Active Case Collection' : 'Collection'}</label>
                <input
                  id="records-collection"
                  className="input"
                  {...(!boundCollectionId ? { list: 'records-collections' } : {})}
                  value={collectionName}
                  onChange={(e) => setCollectionName(e.target.value)}
                  placeholder="required case collection"
                  readOnly={!!boundCollectionId}
                  aria-describedby={boundCollectionId ? 'records-collection-boundary' : undefined}
                  required
                />
                {boundCollectionId && <span id="records-collection-boundary" className="records-guidance">Locked by the active Case Workspace. Switch cases from the case header.</span>}
                {!boundCollectionId && <datalist id="records-collections">
                  {collections.map(col => <option key={col} value={typeof col === 'string' ? col : col.name} />)}
                </datalist>}
              </div>
              <div className="records-field">
                <label htmlFor="records-type">Record Type</label>
                <select id="records-type" className="input" value={recordType} onChange={(e) => setRecordType(e.target.value)}>
                  {recordTypes.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                </select>
              </div>
            </div>

            {preview && (
              <div className="records-guidance">
                <strong>Preview:</strong> first file has about {preview.rows} sampled rows, likely <strong>{preview.recordType}</strong>.
                <div className="records-preview-fields" style={{ marginTop: 8 }}>
                  {preview.fields.slice(0, 24).map(field => <span className="records-chip" key={field}>{field}</span>)}
                </div>
              </div>
            )}

            <button className="btn btn-primary" type="submit" disabled={files.length === 0 || uploading}>
              {uploading ? <><i className="fas fa-spinner fa-spin" /> Ingesting...</> : <><i className="fas fa-upload" /> Ingest Records</>}
            </button>
            {uploadProgress && (
              <div className="records-guidance">
                Processing file {uploadProgress.done + 1} of {uploadProgress.total}: <strong>{uploadProgress.current}</strong>
              </div>
            )}

            {ingestResults.length > 0 && (
              <div className="records-guidance">
                <strong>Ingest summary:</strong> {ingestResults.length} evidence file{ingestResults.length === 1 ? '' : 's'} registered for forensic processing;{' '}
                {ingestResults.reduce((sum, result) => sum + (result?.batch?.row_count || 0), 0)} rows also available in the legacy batch view.
                {ingestResults.slice(0, 5).map(result => result?.batch && (
                  <div key={result.batch.id}>
                    <strong>{result.batch.source_file}</strong>: {result.batch.row_count} rows as {result.batch.record_type}
                  </div>
                ))}
                {ingestResults.flatMap(result => result?.warnings || []).map((warning, i) => <div key={i}>{warning}</div>)}
              </div>
            )}
          </form>

          <div className="card records-panel">
            <h2 style={{ fontSize: '1rem', margin: 0 }}>Batches</h2>
            {loadingBatches ? (
              <div style={{ textAlign: 'center', padding: 'var(--spacing-md)' }}><i className="fas fa-spinner fa-spin" /></div>
            ) : batches.length === 0 ? (
              <p className="records-guidance">No structured record batches have been ingested yet.</p>
            ) : (
              <div className="records-batch-list">
                {batches.map(batch => (
                  <div className="records-batch" key={batch.id}>
                    <div className="records-batch-header">
                      <div>
                        <div className="records-batch-title">{batch.source_file}</div>
                        <div className="records-batch-meta">
                          <span>{batch.record_type}</span>
                          <span>{batch.row_count} rows</span>
                          {batch.collection_name && <span>{batch.collection_name}</span>}
                        </div>
                      </div>
                      <div style={{ display: 'flex', gap: 6 }}>
                        <button className="btn btn-secondary btn-sm" type="button" onClick={() => setSelectedBatch(batch.id)} title="Use in query">
                          <i className="fas fa-filter" />
                        </button>
                        <button className="btn btn-danger btn-sm" type="button" onClick={() => setConfirmDelete(batch)} title="Delete batch">
                          <i className="fas fa-trash" />
                        </button>
                      </div>
                    </div>
                    <div className="records-batch-meta">
                      <span>{batch.id}</span>
                      {batch.source_entry && batch.collection_name && (
                        <Link to={`/app/collections/${encodeURIComponent(batch.collection_name)}`}>KB source</Link>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        <div className="records-panel">
          <div className="card records-panel">
            <h2 style={{ fontSize: '1rem', margin: 0 }}>Query Builder</h2>
            <div className="records-guidance">
              Search in Knowledge Base retrieves evidence and previews. Records queries compute exact counts, filters, rankings, min/max, and lists from parsed rows.
            </div>
            <div className="records-form-grid">
              <div className="records-field">
                <label htmlFor="query-batch">Batch</label>
                <select id="query-batch" className="input" value={selectedBatch} onChange={(e) => setSelectedBatch(e.target.value)}>
                  <option value="">All batches</option>
                  {batches.map(batch => <option key={batch.id} value={batch.id}>{batch.source_file} ({batch.record_type})</option>)}
                </select>
              </div>
              <div className="records-field">
                <label htmlFor="query-type">Record Type</label>
                <select id="query-type" className="input" value={queryRecordType} onChange={(e) => setQueryRecordType(e.target.value)}>
                  <option value="">Any type</option>
                  {recordTypes.filter(([v]) => v !== 'auto').map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                </select>
              </div>
              <div className="records-field">
                <label htmlFor="query-field">Filter Field</label>
                <input id="query-field" className="input" list="records-fields" value={filterField} onChange={(e) => setFilterField(e.target.value)} placeholder="field name" />
              </div>
              <div className="records-field">
                <label htmlFor="query-op">Operator</label>
                <select id="query-op" className="input" value={filterOp} onChange={(e) => setFilterOp(e.target.value)}>
                  {operators.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                </select>
              </div>
              <div className="records-field">
                <label htmlFor="query-value">Value</label>
                <input id="query-value" className="input" value={filterValue} onChange={(e) => setFilterValue(e.target.value)} placeholder="exact value or text" />
              </div>
              <div className="records-field">
                <label htmlFor="query-sort">Sort Field</label>
                <input id="query-sort" className="input" list="records-fields" value={sortField} onChange={(e) => setSortField(e.target.value)} placeholder="optional" />
              </div>
              <div className="records-field">
                <label htmlFor="query-from">From</label>
                <input id="query-from" className="input" type="datetime-local" value={dateFrom} onChange={(e) => setDateFrom(e.target.value)} />
              </div>
              <div className="records-field">
                <label htmlFor="query-to">To</label>
                <input id="query-to" className="input" type="datetime-local" value={dateTo} onChange={(e) => setDateTo(e.target.value)} />
              </div>
              <div className="records-field">
                <label htmlFor="query-direction">Sort</label>
                <select id="query-direction" className="input" value={sortDirection} onChange={(e) => setSortDirection(e.target.value)}>
                  <option value="asc">Ascending</option>
                  <option value="desc">Descending</option>
                </select>
              </div>
              <div className="records-field">
                <label htmlFor="query-limit">Limit</label>
                <input id="query-limit" className="input" type="number" min="1" max="1000" value={limit} onChange={(e) => setLimit(e.target.value)} />
              </div>
            </div>
            <datalist id="records-fields">
              {availableFields.map(field => <option key={field} value={field} />)}
            </datalist>
            {selectedBatchInfo && (
              <div className="records-preview-fields">
                {(selectedBatchInfo.fields || []).slice(0, 18).map(field => (
                  <span className="records-chip" key={`${selectedBatchInfo.id}-${field.original_name}`}>{field.canonical_name || field.normalized_name}</span>
                ))}
              </div>
            )}
            <div style={{ display: 'flex', gap: 'var(--spacing-sm)', flexWrap: 'wrap' }}>
              <button className="btn btn-primary" type="button" onClick={() => runQuery()} disabled={querying}>
                {querying ? <><i className="fas fa-spinner fa-spin" /> Running...</> : <><i className="fas fa-table" /> Run Query</>}
              </button>
              <button className="btn btn-secondary" type="button" onClick={runAggregate} disabled={querying}>
                <i className="fas fa-chart-column" /> Run Aggregate
              </button>
            </div>
            <div className="records-form-grid">
              <div className="records-field">
                <label htmlFor="aggregate-op">Aggregate</label>
                <select id="aggregate-op" className="input" value={aggregateOp} onChange={(e) => setAggregateOp(e.target.value)}>
                  <option value="count">Count</option>
                  <option value="distinct">Distinct</option>
                  <option value="min">Min</option>
                  <option value="max">Max</option>
                  <option value="top_by_field">Top N</option>
                </select>
              </div>
              <div className="records-field">
                <label htmlFor="aggregate-field">Aggregate Field</label>
                <input id="aggregate-field" className="input" list="records-fields" value={aggregateField} onChange={(e) => setAggregateField(e.target.value)} placeholder="duration_seconds, plate_number..." />
              </div>
              <div className="records-field" style={{ gridColumn: '1 / -1' }}>
                <label htmlFor="aggregate-group">Group By</label>
                <input id="aggregate-group" className="input" value={groupBy} onChange={(e) => setGroupBy(e.target.value)} placeholder="comma-separated fields, e.g. area,date" />
              </div>
            </div>
            <div>
              <div className="records-guidance" style={{ marginBottom: 8 }}>Deterministic helper templates</div>
              <div className="records-helper-row">
                {helperExamples.map(example => (
                  <button
                    key={example.label}
                    className="btn btn-secondary btn-sm"
                    type="button"
                    onClick={() => runQuery({ ...example.body, batch_ids: selectedBatch ? [selectedBatch] : undefined })}
                  >
                    <i className={`fas ${example.icon}`} /> {example.label}
                  </button>
                ))}
              </div>
            </div>
          </div>

          <div className="card records-panel">
            <div className="records-results-toolbar">
              <div>
                <h2 style={{ fontSize: '1rem', margin: 0 }}>Results</h2>
                {resultMeta && <div className="records-guidance">{resultMeta.kind}: {resultMeta.count} shown, {resultMeta.total} matched.</div>}
              </div>
              <button className="btn btn-secondary btn-sm" type="button" onClick={exportResults} disabled={results.length === 0}>
                <i className="fas fa-file-csv" /> Export CSV
              </button>
            </div>
            {results.length === 0 ? (
              <p className="records-guidance">Run a query or aggregate to see exact structured results.</p>
            ) : (
              <div className="records-results-wrap">
                <table className="records-results-table">
                  <thead>
                    <tr>{columns.map(col => <th key={col}>{col}</th>)}</tr>
                  </thead>
                  <tbody>
                    {results.map((row, index) => (
                      <tr key={index}>
                        {columns.map(col => (
                          <td key={col} className={isLongValue(row[col]) ? 'records-cell-long' : ''} title={compactValue(row[col])}>
                            {col === 'source_file' && row.collection_name ? (
                              <Link to={`/app/collections/${encodeURIComponent(row.collection_name)}`}>{compactValue(row[col])}</Link>
                            ) : (
                              compactValue(row[col])
                            )}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      </div>
      )}

      {auditOpen && forensicResult && (
        <div className="records-audit-modal" role="dialog" aria-modal="true" aria-label="Forensic audit payload">
          <div className="records-audit-dialog">
            <header>
              <div>
                <h2 style={{ fontSize: '1rem', margin: 0 }}>Developer Audit Payload</h2>
                <div className="records-guidance">Complete sidecar response for trace review and support escalation.</div>
              </div>
              <div className="records-helper-row">
                <button className="btn btn-secondary btn-sm" type="button" onClick={() => exportForensic('json')}>
                  <i className="fas fa-download" /> JSON
                </button>
                <button className="btn btn-secondary btn-sm" type="button" onClick={() => setAuditOpen(false)}>
                  <i className="fas fa-xmark" /> Close
                </button>
              </div>
            </header>
            <pre>{JSON.stringify(forensicResult, null, 2)}</pre>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={!!confirmDelete}
        title="Delete Batch"
        message={confirmDelete ? `Delete parsed records from ${confirmDelete.source_file}? Raw Knowledge Base evidence is not deleted.` : ''}
        confirmLabel="Delete"
        danger
        onConfirm={() => deleteBatch(confirmDelete)}
        onCancel={() => setConfirmDelete(null)}
      />
    </div>
  )
}
