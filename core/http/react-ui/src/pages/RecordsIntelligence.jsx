import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useOutletContext } from 'react-router-dom'
import PageHeader from '../components/PageHeader'
import ConfirmDialog from '../components/ConfirmDialog'
import { agentCollectionsApi, modelsApi, recordsApi } from '../utils/api'

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
    relationship_network: 'show relationship network',
    entity_timeline: 'build timeline',
    evidence: 'find source evidence',
    executive_case_brief: 'summarize this case',
  }
  return known[name] || template?.description || templateTitle(name)
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
      /\b(raw[_\s-]?payload|imei|imsi|msisdn|call type|duration seconds|network volume|location)\b/.test(text) ||
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
  const add = (label, query, template = '') => {
    if (!query || chips.some(chip => chip.query === query && chip.template === template)) return
    chips.push({ label, query, template })
  }
  const families = new Set((status?.record_families || []).map(family => String(family.record_type || '').toLowerCase()))
  const available = new Set((templates || []).map(template => template.name))
  if (available.has('source_file_audit')) add('Source audit', 'which files were ingested?', 'source_file_audit')
  if (available.has('case_readiness')) add('Case readiness', 'is this case ready for production?', 'case_readiness')
  if (sidecarMetric(status, 'duplicate_rows') > 0 && available.has('duplicate_upload_audit')) add('Duplicates', 'show duplicate uploads', 'duplicate_upload_audit')
  if (available.has('limitations_and_data_quality')) add('Limitations', 'what limitations and missing data exist?', 'limitations_and_data_quality')
  if (families.has('cdr')) {
    if (available.has('frequent_contacts')) add('Frequent contacts', 'who are the frequent contacts?', 'frequent_contacts')
    if (available.has('suspicious_patterns')) add('Off-peak activity', 'show hourly nocturnal anomalies', 'suspicious_patterns')
  }
  if (families.has('anpr') && available.has('anpr_sightings')) add('Plate sightings', 'show plate sightings', 'anpr_sightings')
  if (families.size > 0 && available.has('entity_activity')) add('Entity coverage', 'show available entities', 'entity_activity')
  if (available.has('evidence')) add('KB evidence', 'find source evidence', 'evidence')
  if (chips.length === 0) {
    add('Source audit', 'which files were ingested?', 'source_file_audit')
    add('Case readiness', 'is this case ready for production?', 'case_readiness')
    add('Limitations', 'what limitations and missing data exist?', 'limitations_and_data_quality')
  }
  return chips.slice(0, 8)
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
    .replace(/\[Deterministic Fact \(SQL\)\]/g, 'SQL fact:')
    .replace(/\[Semantic Context \(KB\)\]/g, 'KB context:')
    .trim()
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

export default function RecordsIntelligence() {
  const { addToast } = useOutletContext()
  const showSeedLab = useMemo(() => {
    const params = new URLSearchParams(window.location.search)
    return params.get('seed_lab') === '1' || window.localStorage?.getItem('localai.records.seedLab') === '1'
  }, [])
  const [collections, setCollections] = useState([])
  const [batches, setBatches] = useState([])
  const [loadingBatches, setLoadingBatches] = useState(true)
  const [files, setFiles] = useState([])
  const [collectionName, setCollectionName] = useState('')
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
  const [sidecarCollection, setSidecarCollection] = useState('records-demo')
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
  const [templates, setTemplates] = useState([])
  const [templatesError, setTemplatesError] = useState('')
  const [forensicQuery, setForensicQuery] = useState('which files were ingested?')
  const [forensicTemplate, setForensicTemplate] = useState('source_file_audit')
  const [forensicLimit, setForensicLimit] = useState(25)
  const [forensicSynthesisModel, setForensicSynthesisModel] = useState('')
  const [modelOptions, setModelOptions] = useState([])
  const [forensicResult, setForensicResult] = useState(null)
  const [forensicRows, setForensicRows] = useState([])
  const [forensicSearch, setForensicSearch] = useState('')
  const [forensicSort, setForensicSort] = useState('')
  const [forensicSortDirection, setForensicSortDirection] = useState('asc')
  const [forensicWorkspaceTab, setForensicWorkspaceTab] = useState('brief')
  const [templateCategoryFilter, setTemplateCategoryFilter] = useState('All')
  const [templateExplorerOpen, setTemplateExplorerOpen] = useState(false)
  const [telemetryOpen, setTelemetryOpen] = useState(false)
  const [auditOpen, setAuditOpen] = useState(false)
  const [dataManagementOpen, setDataManagementOpen] = useState(false)
  const [runningForensicQuery, setRunningForensicQuery] = useState(false)

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

  const loadBatches = useCallback(async () => {
    setLoadingBatches(true)
    try {
      const data = await recordsApi.listBatches()
      setBatches(Array.isArray(data.batches) ? data.batches : [])
    } catch (err) {
      addToast(`Failed to load record batches: ${err.message}`, 'error')
    } finally {
      setLoadingBatches(false)
    }
  }, [addToast])

  const loadSidecarStatus = useCallback(async () => {
    const collectionID = sidecarCollection.trim()
    if (!collectionID) return
    setLoadingSidecar(true)
    setSidecarError('')
    try {
      const data = await recordsApi.forensicStatus({
        tenant_id: tenantId.trim() || 'default',
        collection_id: collectionID,
        limit: 10,
      })
      setSidecarStatus(data)
    } catch (err) {
      setSidecarStatus(null)
      setSidecarError(err.message)
    } finally {
      setLoadingSidecar(false)
    }
  }, [sidecarCollection, tenantId])

  const loadEvidenceCatalog = useCallback(async () => {
    const collectionID = sidecarCollection.trim()
    if (!collectionID) return
    setLoadingEvidence(true)
    setEvidenceError('')
    try {
      const data = await recordsApi.forensicEvidence({
        tenant_id: tenantId.trim() || 'default',
        collection_id: collectionID,
        limit: 75,
      })
      setEvidenceCatalog(data)
    } catch (err) {
      setEvidenceCatalog(null)
      setSelectedEvidence(null)
      setEvidenceError(err.message)
    } finally {
      setLoadingEvidence(false)
    }
  }, [sidecarCollection, tenantId])

  const loadEvidenceDetail = useCallback(async (item) => {
    const evidenceID = item?.evidence_id
    if (!evidenceID) return
    setLoadingEvidenceDetail(true)
    try {
      const data = await recordsApi.forensicEvidenceDetail(evidenceID, {
        tenant_id: tenantId.trim() || 'default',
        limit: 25,
      })
      setSelectedEvidence(data)
    } catch (err) {
      addToast(`Failed to load evidence detail: ${err.message}`, 'error')
    } finally {
      setLoadingEvidenceDetail(false)
    }
  }, [addToast, tenantId])

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

  useEffect(() => {
    loadBatches()
    agentCollectionsApi.list().then(data => setCollections(Array.isArray(data.collections) ? data.collections : [])).catch(() => {})
    modelsApi.listV1().then(data => {
      const models = Array.isArray(data?.data) ? data.data : []
      setModelOptions(models.map(model => model?.id || model?.name).filter(Boolean).sort())
    }).catch(() => setModelOptions([]))
    loadTemplates()
  }, [loadBatches, loadTemplates])

  useEffect(() => {
    loadSidecarStatus()
    loadEvidenceCatalog()
  }, [loadSidecarStatus, loadEvidenceCatalog])

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
    setUploadProgress({ done: 0, total: filesToIngest.length, current: filesToIngest[0]?.name || '' })
    setIngestResults([])
    const results = []
    for (let i = 0; i < filesToIngest.length; i++) {
      const currentFile = filesToIngest[i]
      setUploadProgress({ done: i, total: filesToIngest.length, current: currentFile.name })
      const result = await recordsApi.ingest(currentFile, targetCollection, targetRecordType)
      results.push(result)
      setIngestResults([...results])
      if (result?.batch?.id) setSelectedBatch(result.batch.id)
    }
    const totalRows = results.reduce((sum, result) => sum + (result?.batch?.row_count || 0), 0)
    addToast(`Ingested ${totalRows} records from ${results.length} file${results.length === 1 ? '' : 's'}`, 'success')
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
    setQuerying(true)
    try {
      const data = await recordsApi.query(body)
      setResults(Array.isArray(data.records) ? data.records : [])
      setResultMeta({ kind: 'query', total: data.total, count: data.count })
    } catch (err) {
      addToast(`Records query failed: ${err.message}`, 'error')
    } finally {
      setQuerying(false)
    }
  }

  const runAggregate = async () => {
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
      addToast(`Records aggregate failed: ${err.message}`, 'error')
    } finally {
      setQuerying(false)
    }
  }

  const runForensicQuery = async (queryOverride, templateOverride) => {
    const queryText = (queryOverride ?? forensicQuery).trim()
    const templateName = templateOverride ?? templateForRuntimeQuery(queryText, forensicTemplate)
    if (!queryText && !templateName) return
    setRunningForensicQuery(true)
    try {
      const data = await recordsApi.forensicQuery({
        tenant_id: tenantId.trim() || 'default',
        collection_id: sidecarCollection.trim() || collectionName.trim() || 'records-demo',
        query: queryText,
        template: templateName || undefined,
        limit: Number(forensicLimit) || 20,
        max_kb_results: 3,
        synthesis_model: forensicSynthesisModel || undefined,
      })
      setForensicResult(data)
      setForensicRows(forensicRowsForResult(data))
      setForensicWorkspaceTab('brief')
      setForensicSearch('')
      if (data?.intent === 'clarification') {
        addToast(data?.answer?.clarification || 'Clarification required', 'warning')
      }
    } catch (err) {
      addToast(`Forensic query failed: ${err.message}`, 'error')
    } finally {
      setRunningForensicQuery(false)
    }
  }

  const deleteBatch = async (batch) => {
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
      .slice(0, 8)
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
  const enterpriseLimitations = Array.isArray(enterprise.limitations) ? enterprise.limitations : []
  const enterpriseCoverage = enterprise.coverage || {}
  const coverageSummary = coverageFacts(enterpriseCoverage)
  const resultStatus = enterprise.status || (forensicResult ? 'answered' : 'ready')
  const primaryBrief = cleanBriefText(enterprise.summary || forensicResult?.answer?.records_summary || forensicResult?.answer?.evidence_summary || '')
  const primaryMetrics = enterpriseMetrics
    .filter(metric => ['Template', 'Planner Confidence', 'Display Rows', 'Records Row Count', 'Evidence Count', 'Provenance Items'].includes(metric.label))
    .filter(metric => compactValue(metric.value) !== '-')
    .slice(0, 6)
  const recentTargets = useMemo(() => {
    const values = []
    const add = value => {
      const text = compactValue(value)
      if (!text || text === '-' || values.includes(text)) return
      values.push(text)
    }
    forensicRows.slice(0, 80).forEach(row => {
      add(row.primary_target || row.primary_entity || row.msisdn || row.source_number)
      add(row.secondary_target || row.secondary_entity || row.target_number || row.call_dialed_num || row.plate_number)
    })
    ;(enterpriseCoverage.valid_target_examples || []).forEach(add)
    return values.slice(0, 12)
  }, [forensicRows, enterpriseCoverage.valid_target_examples])

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
    link.download = `records-${resultMeta?.kind || 'results'}-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.csv`
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
  }

  const exportForensic = (format = 'csv') => {
    if (!forensicResult) return
    const stamp = new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')
    if (format === 'json') {
      downloadTextFile(`forensic-audit-${stamp}.json`, 'application/json;charset=utf-8', JSON.stringify(forensicResult, null, 2))
      return
    }
    if (filteredForensicRows.length === 0 || forensicColumns.length === 0) return
    const csv = [
      forensicColumns.map(csvEscape).join(','),
      ...filteredForensicRows.map(row => forensicColumns.map(col => csvEscape(row[col])).join(',')),
    ].join('\r\n')
    downloadTextFile(`forensic-grid-${stamp}.csv`, 'text/csv;charset=utf-8', csv)
  }

  return (
    <div className="page page--wide">
      <style>{`
        .records-grid { display: grid; grid-template-columns: minmax(280px, 0.95fr) minmax(320px, 1.4fr); gap: var(--spacing-lg); align-items: start; }
        .records-panel { display: flex; flex-direction: column; gap: var(--spacing-md); }
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
        .records-sidecar-form { display: grid; grid-template-columns: minmax(120px, 0.5fr) minmax(180px, 1fr) minmax(108px, 0.34fr) minmax(108px, 0.34fr) 72px 96px; gap: var(--spacing-sm); align-items: end; }
        .records-query-presets { display: flex; flex-wrap: wrap; gap: var(--spacing-xs); }
        .records-query-presets .btn { white-space: normal; text-align: left; }
        .records-runtime-panel { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: var(--spacing-sm); background: var(--color-bg-secondary); display: grid; grid-template-columns: minmax(180px, 0.42fr) minmax(0, 1fr); gap: var(--spacing-sm); align-items: center; }
        .records-runtime-panel strong { display: block; color: var(--color-text-primary); margin-bottom: 2px; }
        .records-runtime-panel span { display: block; color: var(--color-text-secondary); font-size: 0.8125rem; line-height: 1.4; }
        .records-runtime-actions { display: flex; flex-wrap: wrap; gap: var(--spacing-xs); justify-content: flex-end; }
        .records-runtime-actions .btn { white-space: normal; text-align: left; }
        .records-action-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: var(--spacing-sm); }
        .records-action { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: var(--spacing-sm); background: var(--color-bg-primary); text-align: left; display: flex; flex-direction: column; gap: 4px; min-height: 84px; }
        .records-action strong { color: var(--color-text-primary); overflow-wrap: anywhere; }
        .records-action span { color: var(--color-text-secondary); font-size: 0.8125rem; line-height: 1.4; }
        .records-result-shell { display: grid; grid-template-columns: minmax(0, 1fr) minmax(220px, 0.32fr); gap: var(--spacing-md); align-items: start; }
        .records-result-aside { display: flex; flex-direction: column; gap: var(--spacing-sm); }
        .records-status-pill { display: inline-flex; align-items: center; gap: 6px; border-radius: 999px; border: 1px solid var(--color-border); padding: 5px 10px; font-size: 0.8125rem; font-weight: 700; background: var(--color-bg-primary); color: var(--color-text-secondary); }
        .records-status-pill[data-tone="ready"] { border-color: rgba(34, 197, 94, 0.45); color: #166534; background: rgba(34, 197, 94, 0.08); }
        .records-status-pill[data-tone="review"] { border-color: rgba(245, 158, 11, 0.45); color: #92400e; background: rgba(245, 158, 11, 0.1); }
        .records-status-pill[data-tone="input"], .records-status-pill[data-tone="empty"] { border-color: rgba(59, 130, 246, 0.45); color: #1d4ed8; background: rgba(59, 130, 246, 0.08); }
        .records-template-tabs, .records-workspace-tabs { display: flex; flex-wrap: wrap; gap: var(--spacing-xs); }
        .records-template-card { border: 1px solid var(--color-border); border-radius: var(--radius-sm); padding: var(--spacing-sm); background: var(--color-bg-primary); text-align: left; display: flex; flex-direction: column; gap: 6px; min-height: 112px; }
        .records-template-card strong { color: var(--color-text-primary); overflow-wrap: anywhere; }
        .records-template-card span { color: var(--color-text-secondary); font-size: 0.8125rem; line-height: 1.45; }
        .records-template-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(190px, 1fr)); gap: var(--spacing-sm); }
        .records-workspace { border: 1px solid var(--color-border); border-radius: var(--radius-md); overflow: hidden; background: var(--color-bg-primary); }
        .records-workspace-header { display: flex; justify-content: space-between; align-items: center; gap: var(--spacing-sm); flex-wrap: wrap; padding: var(--spacing-sm); border-bottom: 1px solid var(--color-border); background: var(--color-bg-secondary); }
        .records-workspace-body { padding: var(--spacing-md); display: flex; flex-direction: column; gap: var(--spacing-md); background: var(--color-bg-primary); }
        .records-brief { white-space: pre-wrap; color: var(--color-text-primary); line-height: 1.55; }
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
        @media (max-width: 980px) { .records-grid, .records-form-grid, .records-command, .records-result-shell, .records-sidecar-form, .records-table-tools, .records-runtime-panel, .records-evidence-body { grid-template-columns: 1fr; } .records-runtime-actions { justify-content: flex-start; } }
        @media (max-width: 1160px) { .records-seed-grid, .records-checklist, .records-kpi-grid { grid-template-columns: 1fr; } .records-case-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
      `}</style>

      <PageHeader
        title="Records Intelligence"
        supporting="Exact analytics for structured records, paired with Knowledge Base evidence and agent tools."
      />

      <div className="card records-panel" style={{ marginBottom: 'var(--spacing-lg)' }}>
        <div className="records-results-toolbar">
          <div>
            <h2 style={{ fontSize: '1rem', margin: 0 }}>Forensic Operations</h2>
            <div className="records-guidance">
              Sidecar-backed status and deterministic hybrid analytics.
            </div>
          </div>
          <button className="btn btn-secondary btn-sm" type="button" onClick={() => { loadSidecarStatus(); loadEvidenceCatalog() }} disabled={loadingSidecar || loadingEvidence}>
            {loadingSidecar || loadingEvidence ? <><i className="fas fa-spinner fa-spin" /> Refreshing</> : <><i className="fas fa-rotate" /> Refresh</>}
          </button>
        </div>

        <div className="records-command">
          <div className="records-command-main">
            <div className="records-sidecar-form">
              <div className="records-field">
                <label htmlFor="sidecar-collection">Case Collection</label>
                <input id="sidecar-collection" className="input" list="records-collections" value={sidecarCollection} onChange={(e) => setSidecarCollection(e.target.value)} />
              </div>
              <div className="records-field">
                <label htmlFor="forensic-query">Natural Query</label>
                <input id="forensic-query" className="input" list="forensic-recent-targets" value={forensicQuery} onChange={(e) => setForensicQuery(e.target.value)} />
                <datalist id="forensic-recent-targets">
                  {recentTargets.map(target => <option key={target} value={target} />)}
                </datalist>
              </div>
              <div className="records-field">
                <label htmlFor="forensic-template">Template</label>
                <select id="forensic-template" className="input" value={forensicTemplate} onChange={(e) => setForensicTemplate(e.target.value)}>
                  <option value="">Auto</option>
                  {templates.map(template => <option key={template.name} value={template.name}>{template.name}</option>)}
                </select>
              </div>
              <div className="records-field">
                <label htmlFor="forensic-model">Synthesis Model</label>
                <select id="forensic-model" className="input" value={forensicSynthesisModel} onChange={(e) => setForensicSynthesisModel(e.target.value)}>
                  <option value="">Auto / deterministic</option>
                  {modelOptions.map(model => <option key={model} value={model}>{model}</option>)}
                </select>
              </div>
              <div className="records-field">
                <label htmlFor="forensic-limit">Limit</label>
                <input id="forensic-limit" className="input" type="number" min="1" max="100" value={forensicLimit} onChange={(e) => setForensicLimit(e.target.value)} />
              </div>
              <button className="btn btn-primary" type="button" onClick={() => runForensicQuery()} disabled={runningForensicQuery}>
                {runningForensicQuery ? <><i className="fas fa-spinner fa-spin" /> Running</> : <><i className="fas fa-magnifying-glass-chart" /> Run</>}
              </button>
            </div>

            <div className="records-query-presets">
              {promptChips.map(chip => (
                <button
                  key={`${chip.template}:${chip.query}`}
                  className="btn btn-secondary btn-sm"
                  type="button"
                  onClick={() => {
                    setForensicQuery(chip.query)
                    setForensicTemplate(chip.template || '')
                    runForensicQuery(chip.query, chip.template || '')
                  }}
                >
                  {chip.label}
                </button>
              ))}
            </div>

            <div className="records-runtime-panel">
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
            </div>
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
              <strong>{(sidecarStatus?.record_families || []).length.toLocaleString()}</strong>
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
                  {selectedEvidenceJobs.slice(0, 3).map(job => (
                    <div className="records-provenance" key={job.job_id}>
                      <strong>{job.status} ingest job</strong>
                      <span>{job.record_type} - {numericValue(job.accepted_rows).toLocaleString()} accepted rows</span>
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

        {forensicResult ? (
          <div className="records-result-shell">
            <div>
              <div className="records-workspace">
                <div className="records-workspace-header">
                  <div className="records-planner">
                    <span className="records-status-pill" data-tone={statusTone(resultStatus)}>
                      <i className="fas fa-circle-check" /> {statusLabel(resultStatus)}
                    </span>
                    <span className="records-chip">{forensicResult.intent}</span>
                    <span className="records-chip">{forensicResult.template}</span>
                    {(forensicResult.route || []).map(route => <span className="records-chip" key={route}>{route}</span>)}
                    {(forensicResult.planner?.field_hints || []).map(hint => <span className="records-chip" key={hint}>{hint}</span>)}
                    {forensicResult.planner?.date_from && <span className="records-chip">{forensicResult.planner.date_from}</span>}
                  </div>
                  <div className="records-workspace-tabs">
                    {[
                      ['brief', 'Executive Brief'],
                      ['grid', 'Data Grid'],
                      ['timeline', 'Entity Timeline'],
                      ['provenance', 'Provenance & Sources'],
                      ['coverage', 'Coverage & Limitations'],
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
                  {forensicWorkspaceTab === 'brief' && (
                    <>
                      {forensicResult.answer?.clarification_required && (
                        <div className="records-alert records-alert-warning">{forensicResult.answer.clarification}</div>
                      )}
                      <div className="records-brief">
                        {primaryBrief || 'The sidecar returned no executive summary for this query.'}
                      </div>
                      {resultStatus === 'no_results' && coverageSummary.length > 0 && (
                        <div className="records-coverage-grid">
                          {coverageSummary.map(item => (
                            <div className="records-kpi" key={item.label}>
                              <span>{item.label}</span>
                              <strong>{compactValue(item.value)}</strong>
                            </div>
                          ))}
                        </div>
                      )}
                      {enterpriseActions.length > 0 && (
                        <div className="records-action-grid">
                          {enterpriseActions.map((action, index) => (
                            <button
                              key={`${action.template}-${action.query}-${index}`}
                              type="button"
                              className="records-action"
                              onClick={() => {
                                setForensicQuery(action.query || '')
                                setForensicTemplate(action.template || '')
                                runForensicQuery(action.query || '', action.template || '')
                              }}
                            >
                              <strong>{action.label || templateTitle(action.template || action.query)}</strong>
                              <span>{action.reason || action.query}</span>
                            </button>
                          ))}
                        </div>
                      )}
                      {primaryMetrics.length > 0 && (
                        <div className="records-metric-grid">
                          {primaryMetrics.map((metric, index) => (
                            <div className="records-kpi" key={`${metric.label}-${index}`}>
                              <span>{metric.label}</span>
                              <strong>{compactValue(metric.value)}</strong>
                              {metric.source && <span>{metric.source}</span>}
                            </div>
                          ))}
                        </div>
                      )}
                      {enterpriseClaims.length > 0 && (
                        <div className="records-claim-list">
                          {enterpriseClaims.map((claim, index) => (
                            <div className="records-claim" key={index}>
                              <strong>{claim.source || 'Claim'}</strong>
                              <span>{claim.claim}</span>
                              {claim.value !== undefined && <span>Value: {compactValue(claim.value)}</span>}
                            </div>
                          ))}
                        </div>
                      )}
                      {enterpriseLimitations.length > 0 && (
                        <div className="records-panel">
                          {enterpriseLimitations.map((limitation, index) => (
                            <div className="records-alert records-alert-warning" key={index}>{limitation}</div>
                          ))}
                        </div>
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

                  {forensicWorkspaceTab === 'provenance' && (
                    <>
                      {enterpriseProvenance.length > 0 ? (
                        <div className="records-provenance-list">
                          {enterpriseProvenance.map((item, index) => (
                            <div className="records-provenance" key={index}>
                              <strong>{item.source || 'Source'}</strong>
                              <span>{item.source_file || item.source_entry || item.citation || 'No source label returned'}</span>
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
                      {enterpriseLimitations.length > 0 && (
                        <div className="records-panel">
                          {enterpriseLimitations.map((limitation, index) => (
                            <div className="records-alert records-alert-warning" key={index}>{limitation}</div>
                          ))}
                        </div>
                      )}
                    </>
                  )}

                  {forensicWorkspaceTab === 'coverage' && (
                    <>
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
                      {enterpriseLimitations.length > 0 && (
                        <div className="records-panel">
                          {enterpriseLimitations.map((limitation, index) => (
                            <div className="records-alert records-alert-warning" key={index}>{limitation}</div>
                          ))}
                        </div>
                      )}
                    </>
                  )}

                  <div className="records-results-toolbar">
                    <button className="btn btn-secondary btn-sm" type="button" onClick={() => setTelemetryOpen(open => !open)}>
                      <i className="fas fa-gauge-high" /> Telemetry
                    </button>
                    <button className="btn btn-secondary btn-sm" type="button" onClick={() => exportForensic('json')}>
                      <i className="fas fa-download" /> Export Audit
                    </button>
                  </div>

                  {telemetryOpen && (
                    <div className="records-telemetry">
                      <div className="records-kpi-grid">
                        <div className="records-kpi"><span>Jobs</span><strong>{sidecarMetric(sidecarStatus, 'jobs_total').toLocaleString()}</strong></div>
                        <div className="records-kpi"><span>Failed Jobs</span><strong>{sidecarMetric(sidecarStatus, 'failed_jobs').toLocaleString()}</strong></div>
                        <div className="records-kpi"><span>Rejected Rows</span><strong>{sidecarMetric(sidecarStatus, 'rejected_rows').toLocaleString()}</strong></div>
                        <div className="records-kpi"><span>Missing KB Assets</span><strong>{(sidecarStatus?.missing_kb_assets || []).length.toLocaleString()}</strong></div>
                      </div>
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
                    </div>
                  )}
                </div>
              </div>
            </div>

            <div className="records-result-aside">
              <div className="records-telemetry">
                <strong>Case Health</strong>
                <div className="records-family-list">
                  {(sidecarStatus?.record_families || []).slice(0, 6).map(family => (
                    <div className="records-family" key={family.record_type}>
                      <strong>{family.record_type}</strong>
                      <span>{numericValue(family.accepted_rows).toLocaleString()} accepted</span>
                      <span>{numericValue(family.duplicate_rows).toLocaleString()} duplicates</span>
                    </div>
                  ))}
                </div>
              </div>
              {enterpriseLimitations.length > 0 && (
                <div className="records-telemetry">
                  <strong>Coverage</strong>
                  {enterpriseLimitations.slice(0, 4).map((limitation, index) => (
                    <div className="records-alert records-alert-warning" key={index}>{limitation}</div>
                  ))}
                </div>
              )}
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
                    onClick={() => {
                      setForensicQuery(chip.query)
                      setForensicTemplate(chip.template || '')
                      runForensicQuery(chip.query, chip.template || '')
                    }}
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
                  {visibleTemplates.map(template => (
                    <button
                      key={template.name}
                      className="records-template-card"
                      type="button"
                      onClick={() => {
                        const query = templateQuery(template)
                        setForensicTemplate(template.name)
                        setForensicQuery(query)
                      }}
                    >
                      <strong>{templateTitle(template.name)}</strong>
                      <span>{template.description || 'Deterministic records workflow.'}</span>
                      <div className="records-planner">
                        <span className="records-chip">{templateCategory(template)}</span>
                        {template.route && <span className="records-chip">{Array.isArray(template.route) ? template.route.join(' + ') : template.route}</span>}
                      </div>
                    </button>
                  ))}
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
          <div className="records-guidance">Upload records, inspect batches, or run manual exact query tools when operational maintenance is needed.</div>
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
              <input id="records-file" className="input" type="file" multiple accept=".csv,.tsv,.json,.jsonl,.ndjson,.parquet,.txt,.log" onChange={(e) => setFiles(Array.from(e.target.files || []))} />
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
                <label htmlFor="records-collection">Collection</label>
                <input
                  id="records-collection"
                  className="input"
                  list="records-collections"
                  value={collectionName}
                  onChange={(e) => setCollectionName(e.target.value)}
                  placeholder="optional KB collection"
                />
                <datalist id="records-collections">
                  {collections.map(col => <option key={col} value={typeof col === 'string' ? col : col.name} />)}
                </datalist>
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
                <strong>Ingest summary:</strong> {ingestResults.length} batch{ingestResults.length === 1 ? '' : 'es'} stored{' '}
                {ingestResults.reduce((sum, result) => sum + (result?.batch?.row_count || 0), 0)} rows.
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
