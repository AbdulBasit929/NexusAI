import { Fragment, useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { useParams, useNavigate, useOutletContext, useSearchParams } from 'react-router-dom'
import { agentsApi, recordsApi } from '../utils/api'
import { apiUrl } from '../utils/basePath'
import { renderMarkdown, highlightAll, enhanceCodeBlocks } from '../utils/markdown'
import { extractCodeArtifacts, extractMetadataArtifacts, renderMarkdownWithArtifacts } from '../utils/artifacts'
import CanvasPanel from '../components/CanvasPanel'
import ResourceCards from '../components/ResourceCards'
import ConfirmDialog from '../components/ConfirmDialog'
import { useAgentChat } from '../hooks/useAgentChat'
import { useActiveCase } from '../contexts/ActiveCaseContext'
import { relativeTime, normalizeTimestampMs } from '../utils/format'
import { copyToClipboard } from '../utils/clipboard'
import { analystCitation, analystResultState, analystScopedConversationContext, analystSourceNavigationParams, analystSourceTimeRange } from '../analyst/analystAskPresentation'
import { activityFindingGroups } from '../analyst/analystActivityPresentation'
import { analystAuthoritativeEntityChoices, analystNaturalAnswer, analystSingleResultFacts, analystUsefulFindings, analystUsefulFollowUps } from '../analyst/analystPrimaryAnswer'

function getLastMessagePreview(conv) {
  if (!conv.messages || conv.messages.length === 0) return ''
  for (let i = conv.messages.length - 1; i >= 0; i--) {
    const msg = conv.messages[i]
    if (msg.sender === 'user' || msg.sender === 'agent') {
      return (msg.content || '').slice(0, 40).replace(/\n/g, ' ')
    }
  }
  return ''
}

function stripHtml(html) {
  if (!html) return ''
  return html.replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim()
}

function summarizeStatus(text) {
  const plain = stripHtml(text)
  // Extract a short label from "Thinking: ...", "Reasoning: ...", etc.
  const match = plain.match(/^(Thinking|Reasoning|Action taken|Result)[:\s]*/i)
  if (match) return match[1]
  return plain.length > 60 ? plain.slice(0, 60) + '...' : plain
}

function requestIdFromEvent(data) {
  return String(data?.message_id || data?.id || '').replace(/-(user|agent|error)$/, '')
}

function presentationText(value) {
  if (value == null) return ''
  if (typeof value === 'string' || typeof value === 'number') return String(value)
  if (typeof value === 'object') return value.text || value.label || value.value || value.summary || JSON.stringify(value)
  return String(value)
}

function humanizeForensicField(value) {
  const acronyms = new Map([
    ['cdr', 'CDR'], ['ipdr', 'IPDR'], ['anpr', 'ANPR'], ['msisdn', 'MSISDN'],
    ['imsi', 'IMSI'], ['imei', 'IMEI'], ['cnic', 'CNIC'], ['id', 'ID'], ['kb', 'KB'],
  ])
  return String(value || '').split('_').filter(Boolean).map(part => acronyms.get(part.toLowerCase()) || `${part.charAt(0).toUpperCase()}${part.slice(1)}`).join(' ')
}

function forensicCellText(value) {
  if (value == null || value === '') return '—'
  if (Array.isArray(value)) return value.map(forensicCellText).join(', ')
  if (typeof value === 'object') return value.label || value.text || value.summary || 'View details'
  return String(value)
}

function forensicValueDirection(value) {
  const text = forensicCellText(value)
  return /(?:\b\d{4,}\b|\b(?:\d{1,3}\.){3}\d{1,3}\b|[A-Z0-9]{2,}[-_:][A-Z0-9._:-]+)/i.test(text) ? 'ltr' : 'auto'
}

const unsortableNarrativeColumns = /(?:passage|content|transcript|summary|description|excerpt|snippet|narrative|reason|warning)/i

function forensicSortableColumn(rows, column, columnIndex) {
  if (!rows.length || unsortableNarrativeColumns.test(column)) return false
  const values = rows.map(row => Array.isArray(row) ? row[columnIndex] : row?.[column]).filter(value => value != null && value !== '')
  return values.length > 1 && values.every(value => ['string', 'number', 'boolean'].includes(typeof value))
}

function forensicNumericColumn(rows, column, columnIndex) {
  const values = rows.map(row => Array.isArray(row) ? row[columnIndex] : row?.[column]).filter(value => value != null && value !== '')
  return values.length > 0 && values.every(value => typeof value === 'number' || /^-?\d+(?:\.\d+)?$/.test(String(value)))
}

function forensicResultState(status) {
  const states = {
    results_present: ['Results found', 'complete'], complete_zero_results: ['Processing complete · 0 detections', 'empty'],
    no_match_for_filter: ['No matching evidence', 'empty'], not_processed: ['Not processed', 'processing'],
    processing: ['Processing', 'processing'], failed: ['Processing failed', 'failed'], unavailable: ['Capability unavailable', 'unavailable'],
    invalid_request: ['Clarification required', 'input'],
    answered: ['Complete', 'complete'], answered_with_limitations: ['Complete with limitations', 'partial'],
    partial_analysis: ['Partial', 'partial'], no_results: ['No matching evidence', 'empty'],
    needs_input: ['Clarification required', 'input'], unsupported: ['Capability unavailable', 'unavailable'],
    capability_unavailable: ['Capability unavailable', 'unavailable'], data_unavailable: ['Processing required', 'processing'],
    processing_incomplete: ['Processing required', 'processing'], execution_failed: ['Query failed', 'failed'],
    unauthorized: ['Permission denied', 'failed'],
  }
  return states[status] || [humanizeForensicField(status || 'complete'), 'complete']
}

function cleanForensicText(value) {
  return presentationText(value)
    .replace(/\*\*/g, '')
    .replace(/^#{1,6}\s+/gm, '')
    .replace(/\s+/g, ' ')
    .trim()
}

const internalForensicMetrics = new Set([
  'template', 'route', 'planner confidence', 'display rows', 'evidence count', 'provenance items',
])
const analystFailureMessage = 'The question could not be completed. Review the question or try again.'

function forensicMetricValue(metric) {
  return metric?.value ?? metric?.count ?? metric?.total
}

function forensicMetricLabel(metric) {
  return String(metric?.label || metric?.name || '').trim()
}

function forensicToolStage(name, done) {
  if (String(name).includes('hybrid_query')) return done ? 'Exact records analyzed' : 'Running exact records analysis'
  if (String(name).includes('evidence')) return done ? 'Evidence verified' : 'Verifying evidence and provenance'
  if (String(name).includes('report')) return done ? 'Case brief prepared' : 'Preparing evidence-backed case brief'
  return done ? 'Analysis step complete' : 'Planning governed analysis'
}

function forensicFirstValue(row, fields = []) {
  return fields.map(field => row?.[field]).find(value => value != null && value !== '')
}

function forensicVisualRowKey(row, index = 0) {
  return String(forensicFirstValue(row, ['row_hash', 'cdr_row_hash', 'evidence_id']) || [
    forensicFirstValue(row, ['source_file', 'cdr_source_file']),
    forensicFirstValue(row, ['row_number', 'cdr_row_number']),
    forensicFirstValue(row, ['cdr_observed_at', 'call_start_ts', 'observed_at', 'valid_from_at']),
    index,
  ].filter(value => value != null && value !== '').join(':'))
}

function forensicSourceLocator(row) {
  const file = forensicFirstValue(row, ['source_file', 'cdr_source_file'])
  const rowNumber = forensicFirstValue(row, ['row_number', 'cdr_row_number'])
  const hash = forensicFirstValue(row, ['row_hash', 'cdr_row_hash'])
  return [file, rowNumber != null ? `row ${rowNumber}` : '', hash ? `hash ${String(hash).slice(0, 12)}` : ''].filter(Boolean).join(' · ') || 'Query-level cited source'
}

function forensicSourceRoute(portalMode, caseId, source) {
  if (!portalMode) return `/app/cases/${encodeURIComponent(caseId)}/evidence?evidence=${encodeURIComponent(source.evidence_id)}`
  const params = analystSourceNavigationParams(source, source.evidence_id)
  params.set('case', caseId)
  params.set('panel', 'evidence')
  return `/analyst?${params.toString()}`
}

function ForensicVisualEvidence({ visualizations, onOpenSource }) {
  // Summary charts aggregate query results and do not identify a selectable
  // source observation. Only timeline/map views can truthfully drive the
  // observation drawer and its location/provenance fields.
  const allRows = visualizations
    .filter(item => item?.type === 'timeline' || item?.type === 'map')
    .flatMap(item => Array.isArray(item?.spec?.rows) ? item.spec.rows : [])
  const [selectedKey, setSelectedKey] = useState(() => allRows.length ? forensicVisualRowKey(allRows[0], 0) : '')
  const selectedRow = allRows.find((row, index) => forensicVisualRowKey(row, index) === selectedKey) || allRows[0]

  return <div className="forensic-visual-stack">
    {visualizations.map((item, visualIndex) => {
      const spec = item?.spec || {}
      const rows = Array.isArray(spec.rows) ? spec.rows : []
      const citations = Array.isArray(item?.citation_ids) ? item.citation_ids : []
      if (!rows.length) return null
      if (item.type === 'timeline') {
        const timeFields = Array.isArray(spec.time_fields) ? spec.time_fields : ['observed_at']
        const labelFields = Array.isArray(spec.label_fields) ? spec.label_fields : ['event_label']
        return <figure className="forensic-visual" key={item.id || visualIndex} aria-label={item.title || 'Evidence timeline'}>
          <figcaption><strong>{item.title || 'Evidence timeline'}</strong><span>Source-bound chronology; select an event to inspect the same observation across views.</span></figcaption>
          <ol className="forensic-timeline">{rows.slice(0, 30).map((row, index) => {
            const key = forensicVisualRowKey(row, index)
            const status = forensicFirstValue(row, [spec.match_status_field, 'match_status'])
            return <li key={key} className={key === selectedKey ? 'is-selected' : ''}><button type="button" onClick={() => setSelectedKey(key)} aria-pressed={key === selectedKey}><time>{forensicCellText(forensicFirstValue(row, timeFields))}</time><strong>{forensicCellText(forensicFirstValue(row, labelFields))}</strong>{status && <span className={`forensic-match forensic-match--${String(status).replaceAll('_', '-')}`}>{humanizeForensicField(status)}</span>}<small>{forensicSourceLocator(row)}</small></button></li>
          })}</ol>
          {citations.length > 0 && <small className="forensic-visual-provenance">Citations: {citations.join(', ')}</small>}
        </figure>
      }
      if (item.type === 'map') {
        const latitudeField = spec.latitude_field || 'latitude'
        const longitudeField = spec.longitude_field || 'longitude'
        const labelFields = Array.isArray(spec.label_fields) ? spec.label_fields : ['site_identifier', 'location']
        const points = rows.filter(row => Number.isFinite(Number(row?.[latitudeField])) && Number.isFinite(Number(row?.[longitudeField])))
        const latitudes = points.map(row => Number(row[latitudeField]))
        const longitudes = points.map(row => Number(row[longitudeField]))
        const minLat = Math.min(...latitudes), maxLat = Math.max(...latitudes), minLng = Math.min(...longitudes), maxLng = Math.max(...longitudes)
        const spread = (value, min, max) => max === min ? 50 : 8 + ((value - min) / (max - min)) * 84
        return <figure className="forensic-visual" key={item.id || visualIndex} aria-label={item.title || 'Evidence coordinate map'}>
          <figcaption><strong>{item.title || 'Evidence coordinate map'}</strong><span>Supplied coordinate plot with uncertainty; no path, RF coverage, or presence is inferred.</span></figcaption>
          {points.length ? <div className="forensic-coordinate-plot" role="img" aria-label={`${points.length} supplied coordinate observations`}>
            <span className="forensic-coordinate-axis">N</span>
            {points.slice(0, 30).map((row, index) => {
              const key = forensicVisualRowKey(row, index)
              const uncertainty = forensicFirstValue(row, [spec.uncertainty_field, 'uncertainty_radius_m'])
              const status = forensicFirstValue(row, [spec.match_status_field, 'match_status'])
              const label = forensicCellText(forensicFirstValue(row, labelFields))
              return <button type="button" key={key} className={`${key === selectedKey ? 'is-selected' : ''}${status ? ` is-${String(status).replaceAll('_', '-')}` : ''}`} style={{ left: `${spread(Number(row[longitudeField]), minLng, maxLng)}%`, bottom: `${spread(Number(row[latitudeField]), minLat, maxLat)}%` }} onClick={() => setSelectedKey(key)} aria-label={`${label}; ${row[latitudeField]}, ${row[longitudeField]}${uncertainty != null ? `; uncertainty ${uncertainty} metres` : ''}`} aria-pressed={key === selectedKey}><i />{uncertainty != null && <em>{forensicCellText(uncertainty)} m</em>}</button>
            })}
          </div> : <div className="forensic-visual-empty">No result row contains both latitude and longitude. NexusAI did not fabricate map points.</div>}
          {citations.length > 0 && <small className="forensic-visual-provenance">Citations: {citations.join(', ')}</small>}
        </figure>
      }
      return <div className="forensic-visual-summary" key={item.id || visualIndex}><i className="fas fa-chart-column" /><span><strong>{item.title || 'Analysis view'}</strong><small>{rows.length} exact result rows</small></span></div>
    })}
    {selectedRow && <aside className="forensic-evidence-drawer" aria-label="Selected visual evidence details"><div><span>Selected evidence observation</span><strong>{forensicCellText(forensicFirstValue(selectedRow, ['match_status', 'site_identifier', 'cell_site_id', 'msisdn', 'subscriber_reference']))}</strong></div><dl>
      <div><dt>Observed / valid</dt><dd>{forensicCellText(forensicFirstValue(selectedRow, ['cdr_observed_at', 'call_start_ts', 'observed_at', 'valid_from_at']))}</dd></div>
      <div><dt>Coordinates</dt><dd>{selectedRow.latitude != null && selectedRow.longitude != null ? `${selectedRow.latitude}, ${selectedRow.longitude}` : 'Not supplied'}</dd></div>
      <div><dt>Datum / uncertainty</dt><dd>{[selectedRow.coordinate_datum, selectedRow.uncertainty_radius_m != null ? `${selectedRow.uncertainty_radius_m} m` : '', selectedRow.uncertainty_class].filter(Boolean).join(' · ') || 'Not supplied'}</dd></div>
      <div><dt>Source locator</dt><dd>{forensicSourceLocator(selectedRow)}</dd></div>
    </dl>{selectedRow.evidence_id && onOpenSource && <button className="forensic-open-visual-source" type="button" onClick={() => onOpenSource({ evidence_id: selectedRow.evidence_id })}>Open source <i className="fas fa-arrow-up-right-from-square" aria-hidden="true" /></button>}</aside>}
  </div>
}

function ForensicPresentation({ metadata, narrative, onFollowUp, onOpenSource, resolveCitationEvidenceId, entityChoices = [], answerLabel = 'Ask NexusAI answer', analystMode = false }) {
  const [showAllRows, setShowAllRows] = useState(false)
  const [selectedColumns, setSelectedColumns] = useState([])
  const [rowFilter, setRowFilter] = useState('')
  const [sortState, setSortState] = useState({ column: '', direction: 'asc' })
  const [expandedRow, setExpandedRow] = useState(-1)
  const view = metadata?.presentation
  if (!view || view.contract_version !== 'forensics.agent-presentation/v1') return null
  const table = view.table || {}
  const rows = Array.isArray(table.rows) ? table.rows : []
  const rawColumns = Array.isArray(table.columns) && table.columns.length
    ? table.columns
    : (rows[0] && typeof rows[0] === 'object' && !Array.isArray(rows[0]) ? Object.keys(rows[0]).slice(0, 12) : [])
  const rawMetrics = Array.isArray(view.metrics) ? view.metrics : []
  const templateMetric = rawMetrics.find(metric => forensicMetricLabel(metric).toLowerCase() === 'template')
  const tracedOperation = view.trace?.operation
  const operation = String(
    view.operation_id
      || view.operation
      || tracedOperation?.operation_id
      || (typeof tracedOperation === 'string' ? tracedOperation : '')
      || forensicMetricValue(templateMetric)
      || '',
  ).replace(/^.*\./, '')
  const sourceFileColumns = ['source_file', 'record_types', 'processing_states', 'accepted_rows', 'rejected_rows', 'duplicate_rows', 'last_observed_at']
  const allColumns = operation === 'source_file_audit'
    ? sourceFileColumns.filter(column => rows.some(row => row && Object.hasOwn(row, column)))
    : operation === 'temporal_activity'
      ? rawColumns.slice(0, 12)
      : rawColumns.slice(0, 20)
  const priorityColumns = (Array.isArray(table.priority_columns) ? table.priority_columns : [])
    .filter(column => allColumns.includes(column))
  const columns = (selectedColumns.length ? selectedColumns : (priorityColumns.length ? priorityColumns : allColumns.slice(0, 5)))
    .filter(column => allColumns.includes(column))
  const sortableColumns = new Set(columns.filter(column => forensicSortableColumn(rows, column, allColumns.indexOf(column))))
  const numericColumns = new Set(columns.filter(column => forensicNumericColumn(rows, column, allColumns.indexOf(column))))
  const metrics = rawMetrics.filter(metric => {
    const label = forensicMetricLabel(metric).toLowerCase()
    const value = forensicMetricValue(metric)
    return label && !internalForensicMetrics.has(label) && value != null && value !== '' && value !== 'Evidence Count'
  }).map(metric => {
    const label = forensicMetricLabel(metric).toLowerCase() === 'records row count'
      ? (operation === 'source_file_audit' ? 'Source files' : operation === 'frequent_contacts' ? 'Ranked contacts' : 'Results')
      : forensicMetricLabel(metric)
    return { ...metric, label }
  })
  const rawFindings = Array.isArray(view.findings) ? view.findings : []
  const findingGroups = activityFindingGroups({ findings: rawFindings })
  const findings = analystMode ? findingGroups.visible : rawFindings
  const citations = Array.isArray(view.citations) ? view.citations : []
  const limitations = Array.isArray(view.limitations) ? view.limitations : []
  const nextActions = Array.isArray(view.next_actions) ? view.next_actions : []
  const visualizations = Array.isArray(view.visualizations) ? view.visualizations : []
  const relationships = Array.isArray(view.relationships) ? view.relationships : []
  const rowColumnValue = (row, column) => Array.isArray(row) ? row[allColumns.indexOf(column)] : row?.[column]
  const filteredRows = rows.filter(row => !rowFilter || columns.some(column => forensicCellText(rowColumnValue(row, column)).toLowerCase().includes(rowFilter.toLowerCase())))
  const sortedRows = sortState.column ? [...filteredRows].sort((left, right) => {
    const a = forensicCellText(rowColumnValue(left, sortState.column)), b = forensicCellText(rowColumnValue(right, sortState.column))
    const order = a.localeCompare(b, undefined, { numeric: true, sensitivity: 'base' })
    return sortState.direction === 'asc' ? order : -order
  }) : filteredRows
  const visibleRows = showAllRows ? sortedRows.slice(0, 100) : sortedRows.slice(0, 5)
  const modelInterpretation = cleanForensicText(view.model_interpretation && typeof view.model_interpretation === 'object'
    ? view.model_interpretation.summary || view.model_interpretation.text || ''
    : '')
  const elapsed = `${Math.max(1, Number(view.elapsed_ms || 0)).toLocaleString()} ms`
  const needsInput = view.status === 'needs_input' || Boolean(view.clarification)
  const state = analystResultState(view.result_state || view.status)
  const [legacyStateLabel, legacyStateTone] = forensicResultState(view.result_state || view.status)
  const resultStateLabel = analystMode ? state.label : legacyStateLabel
  const resultStateTone = analystMode ? state.tone : legacyStateTone
  const sourceTime = analystSourceTimeRange(view)
  const textDirection = ['ltr', 'rtl', 'auto'].includes(view.text_direction) ? view.text_direction : view.language === 'ur' ? 'rtl' : view.language === 'mixed' ? 'auto' : 'ltr'
  const executiveAnswer = needsInput
    ? cleanForensicText(view.clarification)
    : operation === 'source_file_audit'
      ? `${Number(table.count ?? rows.length).toLocaleString()} source files are registered in this case.`
      : operation === 'case_readiness'
        ? 'These checks assess whether this evidence case is ready for analysis. They do not certify that the NexusAI platform is production-ready.'
        : cleanForensicText(view.executive_answer || 'Deterministic records result')
  const limitationsContent = !needsInput && limitations.length > 0 && <div className="forensic-limitations"><i className="fas fa-triangle-exclamation" /><div><strong>{analystMode ? 'Limitations' : 'Important context'}</strong><ul>{limitations.map((item, index) => <li key={index}>{item}</li>)}</ul></div></div>
  const auditContent = <details className="forensic-trace"><summary>{analystMode ? 'Technical details' : 'How this was determined'}</summary><div><span><strong>Authority</strong> {String(view.execution_authority || '').replaceAll('_', ' ')}</span><span><strong>Operation</strong> {view.operation_id || 'Governed forensic analysis'}</span><span><strong>Specialist</strong> {String(view.specialist || '').replaceAll('_', ' ')}</span><span><strong>Elapsed</strong> {elapsed}</span><span><strong>Row proof</strong> {humanizeForensicField(view.proof_state?.rows || 'complete')}</span><span><strong>Citation proof</strong> {humanizeForensicField(view.proof_state?.citations || 'complete')}</span></div>{analystMode && findingGroups.technical.length > 0 && <details><summary>Execution notes</summary><ul>{findingGroups.technical.map((finding, index) => <li key={index}>{presentationText(finding)}</li>)}</ul></details>}<details><summary>Raw audit details</summary><pre>{JSON.stringify(view.trace || {}, null, 2)}</pre></details>{narrative && <details><summary>Compatibility narrative</summary><div className="forensic-narrative" dangerouslySetInnerHTML={{ __html: renderMarkdown(narrative) }} /></details>}</details>
  const renderColumnHeader = column => {
    const sortable = sortableColumns.has(column)
    const active = sortState.column === column
    return <th scope="col" key={column} aria-sort={active ? (sortState.direction === 'asc' ? 'ascending' : 'descending') : undefined}>{sortable ? <button type="button" onClick={() => setSortState(current => ({ column, direction: current.column === column && current.direction === 'asc' ? 'desc' : 'asc' }))} aria-label={`Sort by ${humanizeForensicField(column)}${active ? `, currently ${sortState.direction === 'asc' ? 'ascending' : 'descending'}` : ''}`}>{humanizeForensicField(column)}<i className={`fas ${active ? `fa-sort-${sortState.direction === 'asc' ? 'up' : 'down'}` : 'fa-sort'}`} aria-hidden="true" /></button> : <span>{humanizeForensicField(column)}</span>}</th>
  }
  const renderTableCell = (row, column) => {
    const value = rowColumnValue(row, column)
    return <td key={column} data-numeric={numericColumns.has(column) ? 'true' : undefined}><bdi dir={forensicValueDirection(value)}>{forensicCellText(value)}</bdi></td>
  }
  const documentPassages = allColumns.includes('passage')
    ? rows.slice(0, 5).map((row, index) => ({
      key: row?.citation_id || row?.evidence_id || `${row?.source_file || 'document'}-${index}`,
      source: forensicCellText(row?.source_file),
      location: forensicCellText(row?.source_location || row?.citation_locator || row?.locator),
      passage: forensicCellText(row?.passage),
    }))
    : []
  const retrievalMode = humanizeForensicField(view.trace?.retrieval_strategy?.mode || view.trace?.retrieval_mode || view.retrieval_mode || '')

  if (analystMode) {
    const answer = analystNaturalAnswer(view)
    const primaryFacts = analystSingleResultFacts(view)
    const primaryFindings = analystUsefulFindings(view, answer)
    const primaryFollowUps = analystUsefulFollowUps(nextActions)
    const analystSources = citations.map((item, index) => {
      const display = analystCitation(item, index)
      const evidenceId = item?.evidence_id || resolveCitationEvidenceId?.(item)
      return { item, index, display, evidenceId, canOpen: Boolean(evidenceId && onOpenSource) }
    })

    return (
      <section className={`analyst-simple-answer analyst-simple-answer--${resultStateTone}`} dir={textDirection} lang={view.language || 'en'} aria-label={needsInput ? 'Clarification required' : 'Answer'}>
        <header className="analyst-simple-answer__header">
          <span className="analyst-simple-answer__eyebrow"><i className={`fas ${needsInput ? 'fa-circle-question' : 'fa-magnifying-glass-chart'}`} aria-hidden="true" /><h3>{needsInput ? 'One detail needed' : 'Answer'}</h3></span>
          <p><bdi dir="auto">{answer}</bdi></p>
        </header>

        {primaryFacts.length > 0 && <dl className="analyst-simple-facts">{primaryFacts.map(fact => <div key={fact.label}><i className={`fas ${/observed|time/i.test(fact.label) ? 'fa-clock' : /location/i.test(fact.label) ? 'fa-location-dot' : /camera/i.test(fact.label) ? 'fa-video' : 'fa-circle-info'}`} aria-hidden="true" /><span><dt>{fact.label}</dt><dd><bdi dir={fact.direction || 'auto'}>{fact.value}</bdi></dd></span></div>)}</dl>}

        {needsInput && entityChoices.length > 0 && <section className="analyst-clarification-choices" aria-label="Available entity choices"><h4>Choose a number from this investigation</h4><div>{entityChoices.map(choice => <button type="button" key={choice.label} onClick={() => onFollowUp(choice.query)}><i className="fas fa-phone" aria-hidden="true" /><bdi dir="ltr">{choice.label}</bdi></button>)}</div></section>}

        {primaryFindings.length > 0 && <section className="analyst-simple-section"><h4>Key findings</h4><ul className="analyst-simple-findings">{primaryFindings.map((finding, index) => <li key={index}><bdi dir="auto">{finding}</bdi></li>)}</ul></section>}

        {!needsInput && documentPassages.length > 0 && <section className="analyst-simple-section analyst-document-passages"><h4>{table.title || 'Source passages'} <span>· {table.count ?? rows.length}</span></h4><div>{documentPassages.map(item => <article key={item.key}><header><i className="fas fa-file-lines" aria-hidden="true" /><strong><bdi dir="auto">{item.source}</bdi></strong>{item.location !== '—' && <small><bdi dir="auto">{item.location}</bdi></small>}</header><p><bdi dir="auto">{item.passage}</bdi></p></article>)}</div></section>}

        {!needsInput && visualizations.length > 0 && <section className="analyst-simple-section"><h4>Evidence view</h4><ForensicVisualEvidence visualizations={visualizations} onOpenSource={onOpenSource} /></section>}

        {!needsInput && analystSources.length > 0 && <section className="analyst-simple-section analyst-simple-evidence"><h4>Evidence <span>· {analystSources.length}</span></h4><div>{analystSources.slice(0, 3).map(({ item, index, display, evidenceId, canOpen }) => {
          const SourceElement = canOpen ? 'button' : 'div'
          return <SourceElement type={canOpen ? 'button' : undefined} key={`${evidenceId || display.label}-${index}`} onClick={canOpen ? () => onOpenSource({ ...item, evidence_id: evidenceId }) : undefined} aria-label={canOpen ? `Open evidence source ${display.label}` : undefined}><i className="fas fa-file-lines" aria-hidden="true" /><span><strong><bdi dir="auto">{display.label}</bdi></strong><small>{humanizeForensicField(item?.evidence_type || item?.record_type || item?.source_type || 'evidence source')}{display.detail ? <><span aria-hidden="true"> · </span><bdi dir="auto">{display.detail}</bdi></> : null}</small></span>{canOpen && <i className="fas fa-arrow-up-right-from-square" aria-hidden="true" />}</SourceElement>
        })}</div>{analystSources.length > 3 && <details><summary>Show {analystSources.length - 3} more</summary><div>{analystSources.slice(3).map(({ item, index, display, evidenceId, canOpen }) => {
          const SourceElement = canOpen ? 'button' : 'div'
          return <SourceElement type={canOpen ? 'button' : undefined} key={`${evidenceId || display.label}-${index}`} onClick={canOpen ? () => onOpenSource({ ...item, evidence_id: evidenceId }) : undefined} aria-label={canOpen ? `Open evidence source ${display.label}` : undefined}><i className="fas fa-file-lines" aria-hidden="true" /><span><strong><bdi dir="auto">{display.label}</bdi></strong><small>{humanizeForensicField(item?.evidence_type || item?.record_type || item?.source_type || 'evidence source')}{display.detail ? <><span aria-hidden="true"> · </span><bdi dir="auto">{display.detail}</bdi></> : null}</small></span>{canOpen && <i className="fas fa-arrow-up-right-from-square" aria-hidden="true" />}</SourceElement>
        })}</div></details>}</section>}

        {!needsInput && documentPassages.length > 0 && limitations.length > 0 && <section className="analyst-simple-section analyst-simple-limitations"><h4>Limitations</h4><ul>{limitations.map((item, index) => <li key={index}>{cleanForensicText(item).replaceAll('_', ' ')}</li>)}</ul></section>}

        {!needsInput && documentPassages.length > 0 && <section className="analyst-simple-section analyst-simple-method"><h4>How this was determined</h4><p>NexusAI searched only the selected evidence scope and returned bounded passages from retained current-version text{retrievalMode ? ` using ${retrievalMode.toLowerCase()} retrieval` : ''}. Source labels and locations shown above come from recorded extraction metadata.</p></section>}

        {primaryFollowUps.length > 0 && <div className="analyst-simple-followups" aria-label="Suggested questions">{primaryFollowUps.map(item => <button type="button" key={item.label} onClick={() => onFollowUp(item.query)}>{item.label}</button>)}</div>}

        {(rows.length > 0 || metrics.length > 0 || limitations.length > 0 || sourceTime || modelInterpretation || findingGroups.technical.length > 0 || view.trace) && <details className="analyst-simple-details"><summary>More details</summary><div className="analyst-simple-details__content">
          {sourceTime && <p><strong>Applied time</strong> <bdi dir="ltr">{sourceTime.label}</bdi></p>}
          {documentPassages.length === 0 && limitations.length > 0 && <section><h4>Important context</h4><ul>{limitations.map((item, index) => <li key={index}>{cleanForensicText(item).replaceAll('_', ' ')}</li>)}</ul></section>}
          {rows.length > 0 && <section><h4>{table.title || 'Results'} · {table.count ?? rows.length}</h4><div className="forensic-table-tools"><label><span className="sr-only">Filter result rows</span><i className="fas fa-filter" /><input value={rowFilter} onChange={event => setRowFilter(event.target.value)} placeholder="Filter displayed results" /></label><details><summary>Columns</summary><div className="forensic-column-picker">{allColumns.map(column => <label key={column}><input type="checkbox" checked={columns.includes(column)} onChange={() => setSelectedColumns(current => { const active = current.length ? current : columns; return active.includes(column) ? active.filter(item => item !== column) : [...active, column] })} />{humanizeForensicField(column)}</label>)}</div></details></div><div className="forensic-table-wrap" tabIndex="0" aria-label="Detailed evidence-backed records"><table><caption className="sr-only">{table.title || 'Evidence-backed records'}</caption><thead><tr>{columns.map(renderColumnHeader)}</tr></thead><tbody>{visibleRows.map((row, index) => <tr key={`detail-row-${index}`}>{columns.map(column => renderTableCell(row, column))}</tr>)}</tbody></table></div>{sortedRows.length > 5 && <button className="forensic-show-all" type="button" onClick={() => setShowAllRows(value => !value)} aria-expanded={showAllRows}>{showAllRows ? 'Show top 5' : `Show up to ${Math.min(sortedRows.length, 100)}`}<i className={`fas fa-chevron-${showAllRows ? 'up' : 'down'}`} /></button>}</section>}
          {metrics.length > 0 && <dl className="analyst-simple-details__metrics">{metrics.map((metric, index) => <div key={metric.label || index}><dt>{metric.label || metric.name}</dt><dd>{forensicCellText(forensicMetricValue(metric))}</dd></div>)}</dl>}
          {modelInterpretation && <section><h4>Interpretation</h4><p>{modelInterpretation}</p></section>}
          {auditContent}
        </div></details>}
      </section>
    )
  }

  return (
    <section className={`forensic-result forensic-result--${resultStateTone}${needsInput ? ' forensic-result--clarification' : ''}`} dir={textDirection} lang={view.language || 'en'} aria-label={needsInput ? 'Clarification required' : 'Forensic analysis result'}>
      <div className="forensic-result-header">
        <div><span className="forensic-result-eyebrow">{needsInput ? 'One detail needed' : answerLabel}</span><h3>{executiveAnswer}</h3></div>
        <div className="forensic-result-badges">
          <span><i className={`fas ${needsInput ? 'fa-circle-question' : 'fa-shield-halved'}`} /> {needsInput ? 'No query executed' : analystMode ? 'Evidence result' : 'Verified records'}</span>
          <span className={`forensic-state forensic-state--${resultStateTone}`}>{resultStateLabel}</span>
          {!analystMode && !needsInput && view.model_status === 'succeeded' && <span className="is-model"><i className="fas fa-microchip" /> Grounded explanation</span>}
          {!analystMode && !needsInput && view.model_status === 'fallback' && <span><i className="fas fa-bolt" /> Exact answer · explanation unavailable</span>}
        </div>
      </div>
      {analystMode && state.description && <div className={`analyst-answer-state analyst-answer-state--${state.tone}`} role="status"><i className={`fas ${state.icon}`} aria-hidden="true" /><span><strong>{state.title}</strong><small>{needsInput && view.clarification ? executiveAnswer : state.description}</small></span></div>}
      {analystMode && sourceTime && <div className="analyst-source-time"><i className="fas fa-clock" aria-hidden="true" /><span><strong>Applied source-time range</strong><bdi dir="ltr">{sourceTime.label}</bdi></span></div>}
      {metrics.length > 0 && <div className="forensic-metric-grid">{metrics.map((metric, index) => {
        const item = typeof metric === 'object' ? metric : { value: metric }
        return <div className="forensic-metric" key={item.label || item.name || index}><span>{item.label || item.name || `Metric ${index + 1}`}</span><strong>{forensicCellText(forensicMetricValue(item))}</strong></div>
      })}</div>}
      {findings.length > 0 && <div className="forensic-result-section"><h4>Key findings</h4><ul className="forensic-findings">{findings.map((item, index) => <li className={`is-${String(item?.claim_type || item?.kind || 'deterministic_fact').replaceAll('_', '-')}`} key={index}><span>{humanizeForensicField(item?.claim_type || item?.kind || 'deterministic fact')}</span><bdi dir="auto">{presentationText(item)}</bdi></li>)}</ul></div>}
      {relationships.length > 0 && <div className="forensic-result-section"><h4>Relationships</h4><div className="forensic-relationships">{relationships.map((item, index) => <article key={item.relationship_id || index}><span>{humanizeForensicField(item.type || item.strength)}</span><bdi dir="ltr">{item.source || 'Observed entity'} → {item.target || 'Related entity'}</bdi><small>{humanizeForensicField(item.strength || 'bounded relationship')} · {item.citation_ids?.length || 0} citations</small></article>)}</div></div>}
      {rows.length > 0 && <div className="forensic-result-section"><div className="forensic-section-heading"><h4>{table.title || 'Key results'}</h4><span>{table.count ?? rows.length} {table.count_label || 'results'} · {table.column_visibility === 'progressive' ? `${columns.length} of ${allColumns.length} columns` : `${columns.length} columns`}</span></div><div className="forensic-table-tools"><label><span className="sr-only">Filter result rows</span><i className="fas fa-filter" /><input value={rowFilter} onChange={event => setRowFilter(event.target.value)} placeholder="Filter displayed results" /></label><details><summary>Columns</summary><div className="forensic-column-picker">{allColumns.map(column => <label key={column}><input type="checkbox" checked={columns.includes(column)} onChange={() => setSelectedColumns(current => { const active = current.length ? current : columns; return active.includes(column) ? active.filter(item => item !== column) : [...active, column] })} />{humanizeForensicField(column)}</label>)}</div></details></div><div className="forensic-table-wrap" tabIndex="0" aria-label="Scrollable evidence-backed records"><table><caption className="sr-only">{table.title || 'Evidence-backed forensic records'}</caption><thead><tr>{columns.map(renderColumnHeader)}<th className="forensic-mobile-detail-heading" scope="col">Details</th></tr></thead><tbody>{visibleRows.map((row, index) => <Fragment key={`row-${index}`}><tr>{columns.map(column => renderTableCell(row, column))}<td className="forensic-mobile-detail"><button type="button" aria-expanded={expandedRow === index} onClick={() => setExpandedRow(current => current === index ? -1 : index)}>View</button></td></tr>{expandedRow === index && <tr className="forensic-expanded-row" key={`detail-${index}`}><td colSpan={columns.length + 1}><dl>{allColumns.map(column => <div key={column}><dt>{humanizeForensicField(column)}</dt><dd><bdi dir={forensicValueDirection(row?.[column])}>{forensicCellText(row?.[column])}</bdi></dd></div>)}</dl></td></tr>}</Fragment>)}</tbody></table></div>{sortedRows.length > 5 && <button className="forensic-show-all" type="button" onClick={() => setShowAllRows(value => !value)} aria-expanded={showAllRows}>{showAllRows ? 'Show top 5' : `Show up to ${Math.min(sortedRows.length, 100)}`}<i className={`fas fa-chevron-${showAllRows ? 'up' : 'down'}`} /></button>}{sortedRows.length > 100 && <p className="forensic-result-bound">Showing the first 100 matching rows. Refine the filter to inspect another bounded subset.</p>}</div>}
      {visualizations.length > 0 && <div className="forensic-result-section"><h4>Visual evidence views</h4><ForensicVisualEvidence visualizations={visualizations} onOpenSource={analystMode ? onOpenSource : undefined} /></div>}
      {!needsInput && citations.length > 0 && <div className="forensic-result-section"><h4>{analystMode ? 'Evidence' : 'Evidence sources'}</h4><div className="forensic-citations">{citations.map((item, index) => {
        const display = analystMode ? analystCitation(item, index) : null
        const detail = display?.detail || item?.detail || item?.source_file || item?.source_entry || item?.evidence_id || (typeof item === 'string' ? item : '')
        const sourceLabel = display?.label || item?.label || (item?.source_file ? 'Source file' : `Source ${index + 1}`)
        const evidenceId = item?.evidence_id || resolveCitationEvidenceId?.(item)
        const canOpen = Boolean(evidenceId && onOpenSource)
        const SourceElement = canOpen ? 'button' : 'div'
        return <SourceElement className="forensic-source" type={canOpen ? 'button' : undefined} key={index} onClick={canOpen ? () => onOpenSource({ ...item, evidence_id: evidenceId }) : undefined} aria-label={canOpen ? `Open evidence source ${sourceLabel}` : undefined}><i className="fas fa-file-shield" /><span>{analystMode && <small className="forensic-source__kind">Source</small>}<strong>{analystMode ? sourceLabel : `${item?.proof_role === 'aggregate_contribution_lineage' ? 'Aggregate contribution lineage' : 'Representative evidence'} · ${sourceLabel}`}</strong>{detail && <small><bdi dir="auto">{detail}</bdi></small>}{item?.completeness && <em>{humanizeForensicField(item.completeness)}</em>}</span>{canOpen && <i className="fas fa-arrow-up-right-from-square forensic-source__open" aria-hidden="true" />}</SourceElement>
      })}</div></div>}
      {!needsInput && modelInterpretation && <div className="forensic-model-interpretation"><span>{analystMode ? 'Interpretation' : 'Analyst context'}</span><p>{modelInterpretation}</p>{analystMode && <small>Explanatory context based on the verified findings above.</small>}</div>}
      {nextActions.length > 0 && <div className="forensic-result-section"><h4>Follow-up questions</h4><div className="forensic-followups">{nextActions.map((item, index) => <button type="button" key={index} onClick={() => onFollowUp(item?.query || presentationText(item))}>{presentationText(item)}<i className="fas fa-arrow-right" /></button>)}</div></div>}
      {analystMode ? <>{limitationsContent}<details className="forensic-details"><summary>How this was determined</summary><div className="forensic-details__content">{auditContent}</div></details></> : <>{limitationsContent}{auditContent}</>}
    </section>
  )
}

const forensicPromptCards = [
  {
    label: 'Audit evidence',
    description: 'List ingested sources and verify traceability.',
    icon: 'fa-shield-halved',
    query: 'which files were ingested?',
    family: 'always',
  },
  {
    label: 'Correlate an entity',
    description: 'Find exact links across every structured record family.',
    icon: 'fa-diagram-project',
    query: 'correlate 35678901234567 across record families',
    family: 'cross_family',
  },
  {
    label: 'Assess case readiness',
    description: 'Check coverage, rejects, duplicates and known limitations.',
    icon: 'fa-list-check',
    query: 'is this case ready for analysis?',
    family: 'always',
  },
  {
    label: 'Find frequent contacts',
    description: 'Choose a number or subscriber for a safe ranked CDR analysis.',
    icon: 'fa-phone-volume',
    query: 'who are the frequent contacts?',
    family: 'cdr',
  },
  {
    label: 'Review plate sightings',
    description: 'Inspect normalized ANPR activity and source evidence.',
    icon: 'fa-car-side',
    query: 'show plate sightings',
    family: 'anpr',
  },
  {
    label: 'Explain limitations',
    description: 'Surface missing data and quality constraints explicitly.',
    icon: 'fa-triangle-exclamation',
    query: 'what limitations and missing data exist?',
    family: 'always',
  },
]

function AgentActivityGroup({ items }) {
  const [expanded, setExpanded] = useState(false)
  if (!items || items.length === 0) return null

  const latest = items[items.length - 1]
  const summary = summarizeStatus(latest.content)

  return (
    <div className="chat-message chat-message-assistant">
      <div className="chat-message-avatar" style={{ background: 'var(--color-bg-tertiary)', color: 'var(--color-text-muted)' }}>
        <i className="fas fa-cogs" />
      </div>
      <div className="chat-activity-group">
        <button className="chat-activity-toggle" onClick={() => setExpanded(!expanded)}>
          <span className="chat-activity-summary">
            {summary}
            {items.length > 1 && <span className="chat-activity-count">+{items.length - 1}</span>}
          </span>
          <i className={`fas fa-chevron-${expanded ? 'up' : 'down'}`} />
        </button>
        {expanded && (
          <div className="chat-activity-details">
            {items.map((item, idx) => (
              <div key={idx} className="chat-activity-item">
                <span className="chat-activity-item-label">{new Date(item.timestamp).toLocaleTimeString()}</span>
                <div className="chat-activity-item-content" style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{item.content}</div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

export default function AgentChat({ agentName = '', portalMode = false, portalPrompts = [], portalScope = null, newConversationSignal = 0, resolveCitationEvidenceId }) {
  const params = useParams()
  const name = agentName || params.name
  const navigate = useNavigate()
  const { addToast } = useOutletContext()
  const [searchParams, setSearchParams] = useSearchParams()
  const userId = searchParams.get('user_id') || undefined
  const requestedCase = searchParams.get('case') || ''
  const requestedPrompt = searchParams.get('prompt') || ''

  const {
    activeCase,
    defaultCase,
    caseOptions,
    state: activeCaseState,
    error: activeCaseError,
    registryState,
    ensureCaseRegistry,
  } = useActiveCase()

  const [agentConfig, setAgentConfig] = useState(undefined)
  const [agentConfigError, setAgentConfigError] = useState('')
  const isForensicAgent = Boolean(agentConfig?.enable_forensic_records)
  const forensicCaseId = isForensicAgent ? activeCase?.caseId || '' : ''
  const forensicCollectionId = isForensicAgent ? activeCase?.collectionId || '' : ''
  const forensicEvidenceScopeId = portalScope?.kind === 'evidence' ? portalScope.evidenceId || 'selected-evidence-pending' : 'current-workspace'
  const activeChatScope = isForensicAgent ? `${forensicCaseId}::${forensicCollectionId}::${forensicEvidenceScopeId}` : 'generic'
  const chatStateReady = agentConfig !== undefined && (!isForensicAgent || Boolean(forensicCaseId && forensicCollectionId))

  const {
    conversations, activeConversation, activeId, scopeReady: conversationScopeReady,
    addConversation, switchConversation, deleteConversation,
    deleteAllConversations, renameConversation, addMessage, addMessageToConversation, clearMessages,
  } = useAgentChat(name, isForensicAgent ? forensicCaseId : '', chatStateReady)

  const liveMessages = useMemo(() => activeConversation?.messages || [], [activeConversation?.messages])

  const [input, setInput] = useState('')
  const [processingChatId, setProcessingChatId] = useState(null)
  const [cancelPending, setCancelPending] = useState(false)
  const [canvasMode, setCanvasMode] = useState(false)
  const [canvasOpen, setCanvasOpen] = useState(false)
  const [selectedArtifactId, setSelectedArtifactId] = useState(null)
  const [sidebarOpen, setSidebarOpen] = useState(() => !portalMode && (typeof window === 'undefined' || window.innerWidth > 1023))
  const [editingName, setEditingName] = useState(null)
  const [editName, setEditName] = useState('')
  const [chatSearch, setChatSearch] = useState('')
  const [showSavedOnly, setShowSavedOnly] = useState(false)
  const [analysisHistory, setAnalysisHistory] = useState([])
  const [analysisHistoryCursor, setAnalysisHistoryCursor] = useState('')
  const [analysisHistoryHasMore, setAnalysisHistoryHasMore] = useState(false)
  const [analysisHistoryState, setAnalysisHistoryState] = useState('idle')
  const [analysisHistoryError, setAnalysisHistoryError] = useState('')
  const [selectedAnalysis, setSelectedAnalysis] = useState(null)
  const [lastHistoryImport, setLastHistoryImport] = useState(null)
  const [confirmDialog, setConfirmDialog] = useState(null)
  const [streamContent, setStreamContent] = useState('')
  const [streamReasoning, setStreamReasoning] = useState('')
  const [streamToolCalls, setStreamToolCalls] = useState([])
  const [requestElapsedMs, setRequestElapsedMs] = useState(0)
  const [retryCandidate, setRetryCandidate] = useState(null)
  const [forensicCapabilities, setForensicCapabilities] = useState(null)
  const [showJumpLatest, setShowJumpLatest] = useState(false)
  const initialConversationSignalRef = useRef(newConversationSignal)
  const messagesEndRef = useRef(null)
  const messagesRef = useRef(null)
  const textareaRef = useRef(null)
  const stickToBottomRef = useRef(true)
  const eventSourceRef = useRef(null)
  const messageIdCounter = useRef(0)
  const addMessageRef = useRef(addMessage)
  addMessageRef.current = addMessage
  const addMessageToConvRef = useRef(addMessageToConversation)
  addMessageToConvRef.current = addMessageToConversation
  const activeIdRef = useRef(activeId)
  activeIdRef.current = activeId
  const activeChatScopeRef = useRef(activeChatScope)
  activeChatScopeRef.current = activeChatScope
  const activeCaseIdRef = useRef(forensicCaseId)
  activeCaseIdRef.current = forensicCaseId
  const activeCollectionIdRef = useRef(forensicCollectionId)
  activeCollectionIdRef.current = forensicCollectionId
  const isForensicAgentRef = useRef(isForensicAgent)
  isForensicAgentRef.current = isForensicAgent
  // Tracks which conversation initiated the current request — SSE responses
  // are pinned to this ID so switching tabs doesn't misdirect them.
  const processingChatIdRef = useRef(null)
  // Backend request ID currently allowed to complete the visible spinner.
  // Older SSE responses may still be rendered in their original conversation,
  // but must never finish a newer request.
  const processingRequestIdRef = useRef(null)
  const processingScopeRef = useRef(null)
  // Maps backend messageID → conversationId for robust SSE routing across navigations.
  const pendingRequestsRef = useRef(new Map())
  const requestStartedAtRef = useRef(0)
  const lastSubmittedQuestionRef = useRef('')

  const processing = !selectedAnalysis && processingChatId === activeId

  useEffect(() => {
    if (!portalMode || newConversationSignal === initialConversationSignalRef.current || !conversationScopeReady) return
    initialConversationSignalRef.current = newConversationSignal
    setSelectedAnalysis(null)
    setInput('')
    addConversation()
    textareaRef.current?.focus()
  }, [addConversation, conversationScopeReady, newConversationSignal, portalMode])

  const authoritativeEntityChoices = useMemo(() => analystAuthoritativeEntityChoices(analysisHistory), [analysisHistory])

  useEffect(() => {
    if (!processing) return undefined
    const update = () => setRequestElapsedMs(Math.max(0, Date.now() - requestStartedAtRef.current))
    update()
    const timer = window.setInterval(update, 250)
    return () => window.clearInterval(timer)
  }, [processing])
  const activeModel = agentConfig?.model || 'Automatic'
  const forensicCaseError = isForensicAgent
    ? (['error', 'inaccessible'].includes(activeCaseState)
        ? activeCaseError || 'The active forensic case is unavailable.'
        : registryState === 'ready' && !activeCase && !defaultCase
          ? 'No authorized selectable forensic case is available.'
          : '')
    : ''
  const forensicChatUnavailable = isForensicAgent && (!forensicCaseId || !forensicCollectionId || activeCaseState !== 'ready')
  const messages = selectedAnalysis ? [
    { id: `${selectedAnalysis.analysis_id}-query`, sender: 'user', content: selectedAnalysis.query, timestamp: new Date(selectedAnalysis.created_at).getTime() },
    selectedAnalysis.answer
      ? { id: `${selectedAnalysis.analysis_id}-answer`, sender: 'agent', content: selectedAnalysis.answer, metadata: selectedAnalysis.metadata, timestamp: new Date(selectedAnalysis.completed_at || selectedAnalysis.updated_at).getTime() }
      : { id: `${selectedAnalysis.analysis_id}-status`, sender: 'system', content: `Retained request status: ${selectedAnalysis.status}`, timestamp: new Date(selectedAnalysis.updated_at).getTime() },
  ] : liveMessages

  useEffect(() => {
    processingChatIdRef.current = null
    processingRequestIdRef.current = null
    processingScopeRef.current = null
    setProcessingChatId(null)
    setCancelPending(false)
    setStreamContent('')
    setStreamReasoning('')
    setStreamToolCalls([])
    setRequestElapsedMs(0)
    setRetryCandidate(null)
    setCanvasOpen(false)
    setSelectedArtifactId(null)
    setSelectedAnalysis(null)
    setLastHistoryImport(null)
  }, [activeChatScope])

  const loadAnalysisHistory = useCallback(async ({ append = false, cursor = '' } = {}) => {
    if (!isForensicAgent || !forensicCaseId || !forensicCollectionId) return
    setAnalysisHistoryState('loading')
    setAnalysisHistoryError('')
    try {
      const page = await agentsApi.analysisHistory(name, userId, {
        caseId: forensicCaseId,
        collectionId: forensicCollectionId,
      }, { cursor, saved: showSavedOnly, limit: 25 })
      setAnalysisHistory(previous => append ? [...previous, ...(page.items || [])] : (page.items || []))
      setAnalysisHistoryCursor(page.next_cursor || '')
      setAnalysisHistoryHasMore(Boolean(page.has_more))
      setAnalysisHistoryState('ready')
    } catch (err) {
      setAnalysisHistoryError(err.message)
      setAnalysisHistoryState('error')
    }
  }, [forensicCaseId, forensicCollectionId, isForensicAgent, name, showSavedOnly, userId])

  useEffect(() => {
    setAnalysisHistory([])
    setAnalysisHistoryCursor('')
    setAnalysisHistoryHasMore(false)
    if (chatStateReady && isForensicAgent) loadAnalysisHistory()
  }, [activeChatScope, chatStateReady, isForensicAgent, loadAnalysisHistory])

  useEffect(() => {
    let active = true
    setAgentConfigError('')
    agentsApi.getConfig(name, userId)
      .then(config => {
        if (active) setAgentConfig(config)
      })
      .catch(err => {
        if (active) {
          setAgentConfig(null)
          setAgentConfigError(err.message)
        }
      })
    return () => { active = false }
  }, [name, userId])

  useEffect(() => {
    if (isForensicAgent && registryState === 'idle') ensureCaseRegistry().catch(() => {})
  }, [ensureCaseRegistry, isForensicAgent, registryState])

  useEffect(() => {
    if (!isForensicAgent || requestedCase || registryState !== 'ready') return
    const candidate = activeCase || defaultCase
    if (!candidate?.caseId) return
    const next = new URLSearchParams(searchParams)
    next.set('case', candidate.caseId)
    setSearchParams(next, { replace: true })
  }, [activeCase, defaultCase, isForensicAgent, registryState, requestedCase, searchParams, setSearchParams])

  useEffect(() => {
    let active = true
    const requestScope = activeChatScope
    if (!forensicCollectionId) {
      setForensicCapabilities(null)
      return () => { active = false }
    }
    recordsApi.forensicCapabilities({ collection_id: forensicCollectionId })
      .then(data => { if (active && requestScope === activeChatScopeRef.current) setForensicCapabilities(data) })
      .catch(() => { if (active) setForensicCapabilities(null) })
    return () => { active = false }
  }, [activeChatScope, forensicCollectionId])

  const availableForensicPrompts = useMemo(() => {
    const capabilityFamilies = forensicCapabilities?.families || []
    const families = new Set(capabilityFamilies
      .filter(family => family.available || family.availability === 'queryable' || Number(family.indexed_records) > 0)
      .map(family => String(family.id || family.record_type || '').toLowerCase()))
    const queryableCount = Number(forensicCapabilities?.summary?.queryable || 0)
    const aliases = { anpr: 'anpr_vehicle_sightings', ipdr: 'ipdr_network_sessions' }
    const base = forensicPromptCards.filter(prompt => prompt.family === 'always' || families.has(aliases[prompt.family] || prompt.family) || (prompt.family === 'cross_family' && queryableCount > 1))
    const familyById = new Map(capabilityFamilies.map(family => [String(family.id || '').toLowerCase(), family]))
    const icons = { cdr: 'fa-phone-volume', ipdr_network_sessions: 'fa-network-wired', anpr_vehicle_sightings: 'fa-car-side', subscriber_identity: 'fa-id-card', tower_location: 'fa-tower-cell', financial_transactions: 'fa-money-bill-transfer', cross_family: 'fa-diagram-project' }
    const corpusPrompts = (forensicCapabilities?.query_corpus?.entries || [])
      .filter(entry => entry.suggested !== false && ((entry.family_id === 'cross_family' && queryableCount > 1) || families.has(entry.family_id)))
      .map(entry => ({
        label: familyById.get(entry.family_id)?.label || (entry.family_id === 'cross_family' ? 'Cross-family correlation' : 'Curated analysis'),
        description: `Validated ${entry.locale || 'en'} query · ${entry.expected_template}`,
        icon: icons[entry.family_id] || 'fa-magnifying-glass-chart',
        query: entry.query,
        family: entry.family_id,
        corpusId: entry.id,
      }))
    const seen = new Set()
    return [...base, ...corpusPrompts].filter(prompt => {
      const key = prompt.query.toLowerCase()
      if (seen.has(key)) return false
      seen.add(key)
      return true
    }).slice(0, 10)
  }, [forensicCapabilities])
  const visibleForensicPrompts = portalMode ? portalPrompts : availableForensicPrompts

  const initialPromptAppliedRef = useRef('')
  useEffect(() => {
    if (!requestedPrompt || initialPromptAppliedRef.current === requestedPrompt) return
    setInput(requestedPrompt)
    initialPromptAppliedRef.current = requestedPrompt

    // A prompt in the URL is an inbound hand-off, not durable editor state.
    // Consume it once so refresh/back navigation cannot overwrite later typing.
    const next = new URLSearchParams(searchParams)
    next.delete('prompt')
    setSearchParams(next, { replace: true })
  }, [requestedPrompt, searchParams, setSearchParams])

  let latestUserQuestion = ''
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index].sender !== 'user') continue
    latestUserQuestion = (messages[index].content || '').trim()
    break
  }
  const askReturnQuestion = input.trim() || latestUserQuestion || lastSubmittedQuestionRef.current || requestedPrompt.trim()
  const askReturnUrl = forensicCaseId
    ? `/app/cases/${encodeURIComponent(forensicCaseId)}/ask${askReturnQuestion ? `?prompt=${encodeURIComponent(askReturnQuestion)}` : ''}`
    : ''

  const nextId = useCallback(() => {
    messageIdCounter.current += 1
    return messageIdCounter.current
  }, [])

  // Connect to SSE endpoint — only reconnect when agent name changes
  useEffect(() => {
    const url = apiUrl(agentsApi.sseUrl(name, userId))
    const es = new EventSource(url)
    eventSourceRef.current = es

    const eventMatchesActiveCase = (data, requestScope = '') => {
      if (!isForensicAgentRef.current) return true
      const eventCaseId = data?.case_id || data?.metadata?.case_id || ''
      const eventCollectionId = data?.collection_id || data?.metadata?.collection_id || ''
      if (eventCaseId && eventCaseId !== activeCaseIdRef.current) return false
      if (eventCollectionId && eventCollectionId !== activeCollectionIdRef.current) return false
      return Boolean(requestScope && requestScope === activeChatScopeRef.current)
    }

    const clearProcessingRequest = (requestId) => {
      if (!requestId || requestId !== processingRequestIdRef.current) return false
      pendingRequestsRef.current.delete(requestId)
      processingRequestIdRef.current = null
      processingChatIdRef.current = null
      processingScopeRef.current = null
      setProcessingChatId(null)
      setCancelPending(false)
      setStreamContent('')
      setStreamReasoning('')
      setStreamToolCalls([])
      setRequestElapsedMs(0)
      return true
    }

    const handleLifecycleStatus = (data, reconciled = false) => {
      const requestId = requestIdFromEvent(data)
      if (!requestId) return
      const pendingRequest = pendingRequestsRef.current.get(requestId)
      if (pendingRequest && processingChatIdRef.current && pendingRequest.conversationId !== processingChatIdRef.current) return
      const requestScope = pendingRequest?.scope || processingScopeRef.current || ''
      if (!eventMatchesActiveCase(data, requestScope)) return
      if (processingRequestIdRef.current && requestId !== processingRequestIdRef.current) return
      if (data.status === 'processing') {
        if (!processingChatIdRef.current) {
          processingChatIdRef.current = activeIdRef.current
          setProcessingChatId(activeIdRef.current)
        }
        if (!processingRequestIdRef.current) {
          processingRequestIdRef.current = requestId
          pendingRequestsRef.current.set(requestId, {
            conversationId: processingChatIdRef.current || activeIdRef.current,
            scope: processingScopeRef.current || activeChatScopeRef.current,
          })
        }
        setStreamContent('')
        setStreamReasoning('')
        setStreamToolCalls([])
      } else if (data.status === 'completed' || data.status === 'needs_input') {
        if (!reconciled) return
        const targetId = pendingRequest?.conversationId || processingChatIdRef.current
        if (targetId) {
          addMessageToConvRef.current(targetId, {
            id: nextId(), sender: 'system', content: 'Request completed offline. Replay unavailable.', timestamp: Date.now(),
          })
        }
        addToast('Agent request completed while disconnected', 'info')
        clearProcessingRequest(requestId)
      } else if (data.status === 'error' || String(data.status).startsWith('error:')) {
        if (pendingRequest?.message) setRetryCandidate({ requestId, ...pendingRequest })
        addToast(portalMode ? analystFailureMessage : String(data.status).replace(/^error:\s*/i, '') || 'Agent error', 'error')
        clearProcessingRequest(requestId)
      } else if (data.status === 'cancelled') {
        const targetId = pendingRequest?.conversationId || processingChatIdRef.current
        if (targetId) {
          addMessageToConvRef.current(targetId, {
            id: nextId(), sender: 'system', content: 'Request stopped by analyst.', timestamp: Date.now(),
          })
        }
        addToast('Agent request stopped', 'info')
        if (pendingRequest?.message) setRetryCandidate({ requestId, ...pendingRequest })
        clearProcessingRequest(requestId)
      } else if (data.status === 'timed_out') {
        const targetId = pendingRequest?.conversationId || processingChatIdRef.current
        if (targetId) {
          addMessageToConvRef.current(targetId, {
            id: nextId(), sender: 'system', content: 'Request timed out. No automatic retry was sent.', timestamp: Date.now(),
          })
        }
        addToast('Agent request timed out', 'warning')
        if (pendingRequest?.message) setRetryCandidate({ requestId, ...pendingRequest })
        clearProcessingRequest(requestId)
      }
    }

    es.addEventListener('json_message', (e) => {
      try {
        const data = JSON.parse(e.data)
        const sender = data.sender || (data.role === 'user' ? 'user' : 'agent')
        // Skip user message echoes — already added locally in handleSend
        if (sender === 'user') return
        const msg = {
          id: nextId(),
          sender,
          content: data.content || data.message || '',
          // Backend timestamp encoding varies by deploy mode (RFC3339 string,
          // Unix ms, or Unix ns); normalize to JS milliseconds.
          timestamp: normalizeTimestampMs(data.timestamp),
        }
        const answerMetadata = data.answer_metadata || data.metadata
        if (answerMetadata && typeof answerMetadata === 'object' && Object.keys(answerMetadata).length > 0) {
          msg.metadata = answerMetadata
        }
        // Route to conversation: try messageID mapping first, then processingChatIdRef, then active
        const baseId = requestIdFromEvent(data)
        const pendingRequest = pendingRequestsRef.current.get(baseId)
        const requestScope = pendingRequest?.scope || processingScopeRef.current || ''
        if (sender === 'agent' && requestScope) {
          msg.metadata = { ...(msg.metadata || {}), request_scope: requestScope }
        }
        if (!eventMatchesActiveCase(data, requestScope)) {
          if (baseId) pendingRequestsRef.current.delete(baseId)
          return
        }
        const targetId = pendingRequest?.conversationId
          || processingChatIdRef.current
          || activeIdRef.current
        addMessageToConvRef.current(targetId, msg)
        // Clear streaming + processing state when the final agent message arrives
        if (sender === 'agent') {
          pendingRequestsRef.current.delete(baseId)
          clearProcessingRequest(baseId)
        }
      } catch (_err) {
        // ignore malformed messages
      }
    })

    es.addEventListener('json_message_status', (e) => {
      try {
        const data = JSON.parse(e.data)
        const requestId = requestIdFromEvent(data)
        if (!requestId) return
        const pendingRequest = pendingRequestsRef.current.get(requestId)
        if (pendingRequest && processingChatIdRef.current && pendingRequest.conversationId !== processingChatIdRef.current) return
        const requestScope = pendingRequest?.scope || processingScopeRef.current || ''
        if (!eventMatchesActiveCase(data, requestScope)) return
        if (processingRequestIdRef.current && requestId !== processingRequestIdRef.current) return
        if (data.status === 'processing') {
          // Track which conversation is processing so responses go to the right place.
          // Only set if not already pinned by handleSend (avoids race when user switches conversations).
          if (!processingChatIdRef.current) {
            processingChatIdRef.current = activeIdRef.current
            setProcessingChatId(activeIdRef.current)
          }
          if (!processingRequestIdRef.current) {
            processingRequestIdRef.current = requestId
            pendingRequestsRef.current.set(requestId, {
              conversationId: processingChatIdRef.current || activeIdRef.current,
              scope: processingScopeRef.current || activeChatScopeRef.current,
            })
          }
          setStreamContent('')
          setStreamReasoning('')
          setStreamToolCalls([])
      } else if (data.status === 'completed' || data.status === 'needs_input') {
          // Don't clear processingChatIdRef, processingChatId, or streaming state here —
          // they'll be cleared when the agent's json_message arrives,
          // so reasoning and tool calls remain visible until the response replaces them
          // and late-arriving messages still route to the correct conversation.
        } else if (data.status === 'error' || String(data.status).startsWith('error:')) {
          if (pendingRequest?.message) setRetryCandidate({ requestId, ...pendingRequest })
          addToast(portalMode ? analystFailureMessage : String(data.status).replace(/^error:\s*/i, '') || 'Agent error', 'error')
          clearProcessingRequest(requestId)
        } else if (data.status === 'cancelled') {
          const targetId = pendingRequest?.conversationId || processingChatIdRef.current
          if (targetId) {
            addMessageToConvRef.current(targetId, {
              id: nextId(), sender: 'system', content: 'Request stopped by analyst.', timestamp: Date.now(),
            })
          }
          addToast('Agent request stopped', 'info')
          if (pendingRequest?.message) setRetryCandidate({ requestId, ...pendingRequest })
          clearProcessingRequest(requestId)
        } else if (data.status === 'timed_out') {
          const targetId = pendingRequest?.conversationId || processingChatIdRef.current
          if (targetId) {
            addMessageToConvRef.current(targetId, {
              id: nextId(), sender: 'system', content: 'Request timed out. No automatic retry was sent.', timestamp: Date.now(),
            })
          }
          addToast('Agent request timed out', 'warning')
          if (pendingRequest?.message) setRetryCandidate({ requestId, ...pendingRequest })
          clearProcessingRequest(requestId)
        }
      } catch (_err) {
        // ignore
      }
    })

    es.addEventListener('stream_event', (e) => {
      try {
        const data = JSON.parse(e.data)
        const requestId = requestIdFromEvent(data)
        if (!requestId || requestId !== processingRequestIdRef.current) return
        const pendingRequest = pendingRequestsRef.current.get(requestId)
        const requestScope = pendingRequest?.scope || processingScopeRef.current || ''
        if (!eventMatchesActiveCase(data, requestScope)) return
        if (data.type === 'reasoning') {
          setStreamReasoning(prev => prev + (data.content || ''))
        } else if (data.type === 'content') {
          setStreamContent(prev => prev + (data.content || ''))
        } else if (data.type === 'tool_call') {
          const name = data.tool_name || ''
          const args = data.tool_args || ''
          setStreamToolCalls(prev => {
            if (name) {
              return [...prev, { name, args }]
            }
            if (prev.length === 0) return prev
            const updated = [...prev]
            updated[updated.length - 1] = { ...updated[updated.length - 1], args: updated[updated.length - 1].args + args }
            return updated
          })
        } else if (data.type === 'tool_result') {
          const tname = data.tool_name || ''
          setStreamToolCalls(prev => {
            const updated = [...prev]
            const idx = updated.findLastIndex(tc => tc.name === tname && !tc.result)
            if (idx >= 0) {
              updated[idx] = { ...updated[idx], result: data.tool_result || 'done' }
            }
            return updated
          })
        } else if (data.type === 'done') {
          // Content will be finalized by json_message event
        }
      } catch (_err) {
        // ignore
      }
    })

    es.addEventListener('status', (e) => {
      const text = e.data
      if (!text) return
      if (isForensicAgentRef.current && processingScopeRef.current !== activeChatScopeRef.current) return
      const targetId = processingChatIdRef.current || activeIdRef.current
      addMessageToConvRef.current(targetId, {
        id: nextId(),
        sender: 'system',
        content: text,
        timestamp: Date.now(),
      })
    })

    es.addEventListener('json_error', (e) => {
      let requestId = ''
      try {
        const data = JSON.parse(e.data)
        requestId = requestIdFromEvent(data)
        if (!requestId) {
          addToast(portalMode ? analystFailureMessage : data.error || data.message || 'Agent event stream error', 'error')
          return
        }
        const pendingRequest = pendingRequestsRef.current.get(requestId)
        const requestScope = pendingRequest?.scope || processingScopeRef.current || ''
        if (!eventMatchesActiveCase(data, requestScope)) return
        if (requestId !== processingRequestIdRef.current) return
        addToast(portalMode ? analystFailureMessage : data.error || data.message || 'Agent error', 'error')
      } catch (_err) {
        if (isForensicAgentRef.current && processingScopeRef.current !== activeChatScopeRef.current) return
        addToast(portalMode ? analystFailureMessage : 'Agent error', 'error')
        return
      }
      clearProcessingRequest(requestId)
    })

    es.onerror = () => {
      // EventSource reconnects automatically. The analyst workspace keeps this
      // transport detail out of the primary flow; an active request continues
      // to show its truthful processing state until reconciliation completes.
      if (!portalMode) addToast('SSE connection lost, attempting to reconnect...', 'warning')
    }

    es.onopen = async () => {
      const requestId = processingRequestIdRef.current
      const requestScope = processingScopeRef.current
      if (!requestId || !isForensicAgentRef.current || requestScope !== activeChatScopeRef.current) return
      try {
        const status = await agentsApi.chatStatus(name, requestId, userId, {
          caseId: activeCaseIdRef.current,
          collectionId: activeCollectionIdRef.current,
        })
        if (requestId !== processingRequestIdRef.current || requestScope !== processingScopeRef.current) return
        handleLifecycleStatus(status, true)
      } catch (_err) {
        if (requestId !== processingRequestIdRef.current || requestScope !== processingScopeRef.current) return
        const targetId = pendingRequestsRef.current.get(requestId)?.conversationId || processingChatIdRef.current
        if (targetId) {
          addMessageToConvRef.current(targetId, {
            id: nextId(), sender: 'system', content: 'Request status could not be reconciled. No retry was sent.', timestamp: Date.now(),
          })
        }
        addToast('Agent request status unavailable after reconnect', 'warning')
        clearProcessingRequest(requestId)
      }
    }

    return () => {
      es.close()
      eventSourceRef.current = null
      processingChatIdRef.current = null
      processingRequestIdRef.current = null
      processingScopeRef.current = null
      pendingRequestsRef.current.clear()
    }
  }, [name, userId, addToast, nextId, portalMode])

  // Track whether the user is pinned to the bottom. If they scroll up
  // while a response is streaming, stop forcing them back down.
  useEffect(() => {
    const el = messagesRef.current
    if (!el) return
    const onScroll = () => {
      const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
      stickToBottomRef.current = distanceFromBottom < 80
      setShowJumpLatest(!stickToBottomRef.current)
    }
    el.addEventListener('scroll', onScroll, { passive: true })
    return () => el.removeEventListener('scroll', onScroll)
  }, [])

  const scrollMessagesToEnd = useCallback((behavior = 'auto') => {
    const el = messagesRef.current
    if (!el) return
    // Keep scrolling owned by the transcript. scrollIntoView may also move the
    // page and portal shell, leaving the fixed Ask surface apparently blank.
    el.scrollTo({ top: el.scrollHeight, behavior })
  }, [])

  // Auto-scroll only when the user hasn't scrolled away from the bottom.
  useEffect(() => {
    if (!stickToBottomRef.current) return
    scrollMessagesToEnd('smooth')
    setShowJumpLatest(false)
  }, [messages, streamContent, streamReasoning, streamToolCalls, scrollMessagesToEnd])

  // When switching conversations, snap to bottom and re-pin.
  useEffect(() => {
    stickToBottomRef.current = true
    setShowJumpLatest(false)
    scrollMessagesToEnd('auto')
  }, [activeId, scrollMessagesToEnd])

  const jumpToLatest = useCallback(() => {
    stickToBottomRef.current = true
    setShowJumpLatest(false)
    scrollMessagesToEnd('smooth')
    messagesRef.current?.focus({ preventScroll: true })
  }, [scrollMessagesToEnd])

  // Highlight code blocks + add per-block copy buttons (parity with Chat). A
  // MutationObserver on the messages container fires reliably for streamed and
  // loaded messages; it disconnects while mutating so its own edits do not
  // retrigger it.
  useEffect(() => {
    const el = messagesRef.current
    if (!el) return
    let obs
    const run = () => {
      obs?.disconnect()
      highlightAll(el)
      enhanceCodeBlocks(el)
      obs?.observe(el, { childList: true, subtree: true })
    }
    obs = new MutationObserver(run)
    run()
    return () => obs.disconnect()
  }, [])

  const agentMessages = useMemo(() => messages.filter(m => m.sender === 'agent'), [messages])
  const codeArtifacts = useMemo(
    () => canvasMode ? extractCodeArtifacts(agentMessages, 'sender', 'agent') : [],
    [agentMessages, canvasMode]
  )
  const metaArtifacts = useMemo(
    () => canvasMode ? extractMetadataArtifacts(messages, name) : [],
    [messages, canvasMode, name]
  )
  const artifacts = useMemo(() => [...codeArtifacts, ...metaArtifacts], [codeArtifacts, metaArtifacts])

  const prevArtifactCountRef = useRef(0)
  useEffect(() => {
    prevArtifactCountRef.current = artifacts.length
  }, [activeId])
  useEffect(() => {
    if (artifacts.length > prevArtifactCountRef.current && artifacts.length > 0) {
      setSelectedArtifactId(artifacts[artifacts.length - 1].id)
      if (!canvasOpen) setCanvasOpen(true)
    }
    prevArtifactCountRef.current = artifacts.length
  }, [artifacts])

  // Event delegation for artifact cards
  useEffect(() => {
    const el = messagesRef.current
    if (!el || !canvasMode) return
    const handler = (e) => {
      const openBtn = e.target.closest('.artifact-card-open')
      const downloadBtn = e.target.closest('.artifact-card-download')
      const card = e.target.closest('.artifact-card')
      if (downloadBtn) {
        e.stopPropagation()
        const id = downloadBtn.dataset.artifactId
        const artifact = artifacts.find(a => a.id === id)
        if (artifact?.code) {
          const blob = new Blob([artifact.code], { type: 'text/plain' })
          const url = URL.createObjectURL(blob)
          const a = document.createElement('a')
          a.href = url
          a.download = artifact.title || 'download.txt'
          a.click()
          URL.revokeObjectURL(url)
        }
        return
      }
      if (openBtn || card) {
        const id = (openBtn || card).dataset.artifactId
        if (id) {
          setSelectedArtifactId(id)
          setCanvasOpen(true)
        }
      }
    }
    el.addEventListener('click', handler)
    return () => el.removeEventListener('click', handler)
  }, [canvasMode, artifacts])

  const openArtifactById = useCallback((id) => {
    setSelectedArtifactId(id)
    setCanvasOpen(true)
  }, [])

  const handleSend = useCallback(async (messageOverride) => {
    const msg = (typeof messageOverride === 'string' ? messageOverride : input).trim()
    if (!msg || processing || selectedAnalysis || agentConfig === undefined || !conversationScopeReady) return
    if (isForensicAgent && (!forensicCaseId || !forensicCollectionId || forensicCaseError || activeCaseState !== 'ready')) {
      addToast(`Failed to send message: ${forensicCaseError || 'Select an authorized active case before sending a forensic question.'}`, 'error')
      return
    }
    const requestScope = activeChatScope
    const conversationContext = isForensicAgent
      ? analystScopedConversationContext(liveMessages, activeId, requestScope, msg)
      : null
    const queryScope = isForensicAgent && portalScope ? {
      kind: portalScope.kind === 'evidence' ? 'selected_evidence' : 'current_workspace',
      evidence_id: portalScope.evidenceId || '',
      evidence_version_id: portalScope.evidenceVersionId || '',
      source_family: portalScope.sourceFamily || '',
      available_result_families: portalScope.availableResultFamilies || [],
    } : null
    lastSubmittedQuestionRef.current = msg
    setRetryCandidate(null)
    setInput('')
    if (textareaRef.current) textareaRef.current.style.height = 'auto'
    // A new analyst turn always becomes the active transcript edge, even if
    // the analyst had scrolled upward while reviewing the previous answer.
    stickToBottomRef.current = true
    setShowJumpLatest(false)
    // Add user message locally immediately (like standard chat)
    addMessage({ id: nextId(), sender: 'user', content: msg, timestamp: Date.now() })
    setProcessingChatId(activeId)
    requestStartedAtRef.current = Date.now()
    setRequestElapsedMs(0)
    processingChatIdRef.current = activeId
    processingRequestIdRef.current = null
    processingScopeRef.current = requestScope
    try {
      const resp = await agentsApi.chat(name, msg, userId, isForensicAgent ? {
        caseId: forensicCaseId,
        collectionId: forensicCollectionId,
        conversationContext,
        queryScope,
      } : null)
      if (isForensicAgent && activeChatScopeRef.current !== requestScope) return
      // Map backend messageID → conversation so SSE events route correctly
      if (resp && resp.message_id) {
        pendingRequestsRef.current.set(resp.message_id, { conversationId: activeId, scope: requestScope, message: msg })
        // The HTTP 202 response is authoritative. It corrects any status-event
        // race without allowing a response from another conversation to take over.
        if (processingChatIdRef.current === activeId) {
          processingRequestIdRef.current = resp.message_id
        }
      }
    } catch (err) {
      if (isForensicAgent && activeChatScopeRef.current !== requestScope) return
      addToast(portalMode ? 'The question could not be completed. Review the question and try again.' : `Failed to send message: ${err.message}`, 'error')
      processingChatIdRef.current = null
      processingRequestIdRef.current = null
      processingScopeRef.current = null
      setProcessingChatId(null)
    }
  }, [input, processing, selectedAnalysis, name, activeId, addToast, userId, addMessage, nextId, agentConfig, conversationScopeReady, isForensicAgent, forensicCaseId, forensicCollectionId, forensicCaseError, activeCaseState, activeChatScope, liveMessages, portalMode, portalScope])

  const handleRetry = useCallback(async () => {
    const candidate = retryCandidate
    if (!candidate || processing || !isForensicAgent || candidate.scope !== activeChatScope) return
    const idempotencyKey = candidate.idempotencyKey
      || (globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}-retry`)
    const stableCandidate = { ...candidate, idempotencyKey }
    setRetryCandidate(stableCandidate)
    setProcessingChatId(candidate.conversationId)
    requestStartedAtRef.current = Date.now()
    setRequestElapsedMs(0)
    processingChatIdRef.current = candidate.conversationId
    processingRequestIdRef.current = null
    processingScopeRef.current = candidate.scope
    try {
      const resp = await agentsApi.retryChat(name, candidate.requestId, candidate.message, idempotencyKey, userId, {
        caseId: forensicCaseId,
        collectionId: forensicCollectionId,
        queryScope: portalScope ? {
          kind: portalScope.kind === 'evidence' ? 'selected_evidence' : 'current_workspace',
          evidence_id: portalScope.evidenceId || '',
          evidence_version_id: portalScope.evidenceVersionId || '',
          source_family: portalScope.sourceFamily || '',
          available_result_families: portalScope.availableResultFamilies || [],
        } : null,
      })
      pendingRequestsRef.current.set(resp.message_id, {
        conversationId: candidate.conversationId,
        scope: candidate.scope,
        message: candidate.message,
      })
      processingRequestIdRef.current = resp.message_id
      setRetryCandidate(null)
      addToast(resp.idempotent_replay ? 'Existing retry resumed; no duplicate was sent' : 'Retry started', 'info')
    } catch (err) {
      processingChatIdRef.current = null
      processingRequestIdRef.current = null
      processingScopeRef.current = null
      setProcessingChatId(null)
      addToast(portalMode ? 'The retry could not be completed. No duplicate request was sent.' : `Retry failed: ${err.message}`, 'error')
    }
  }, [activeChatScope, addToast, forensicCaseId, forensicCollectionId, isForensicAgent, name, processing, retryCandidate, userId, portalMode, portalScope])

  const handleCancel = useCallback(async () => {
    const requestId = processingRequestIdRef.current
    if (!requestId || !isForensicAgent || cancelPending) return
    setCancelPending(true)
    try {
      await agentsApi.cancelChat(name, requestId, userId, {
        caseId: forensicCaseId,
        collectionId: forensicCollectionId,
      })
    } catch (err) {
      setCancelPending(false)
      addToast(portalMode ? 'The stop request could not be confirmed. Check Activity before trying again.' : `Failed to stop request: ${err.message}`, 'error')
    }
  }, [addToast, cancelPending, forensicCaseId, forensicCollectionId, isForensicAgent, name, userId, portalMode])

  const handleKeyDown = (e) => {
    if (
      e.key === 'Enter' &&
      !e.shiftKey &&
      !e.ctrlKey &&
      !e.metaKey &&
      !e.altKey &&
      !e.nativeEvent?.isComposing &&
      e.keyCode !== 229
    ) {
      e.preventDefault()
      handleSend()
    }
  }

  const copyMessage = async (content) => {
    const ok = await copyToClipboard(content)
    addToast(
      ok ? 'Copied to clipboard' : 'Could not copy to clipboard',
      ok ? 'success' : 'error',
      ok ? 2000 : 3000,
    )
  }

  const senderToRole = (sender) => {
    if (sender === 'agent') return 'assistant'
    if (sender === 'user') return 'user'
    return 'system'
  }

  const startRename = (id, currentName) => {
    setEditingName(id)
    setEditName(currentName)
  }

  const finishRename = () => {
    if (editingName && editName.trim()) {
      renameConversation(editingName, editName.trim())
    }
    setEditingName(null)
  }

  const toggleRetainedAnalysis = async (entry) => {
    try {
      const updated = await agentsApi.saveAnalysis(name, entry.analysis_id, !entry.saved, entry.saved_title || '', userId, {
        caseId: forensicCaseId,
        collectionId: forensicCollectionId,
      })
      setAnalysisHistory(items => items.map(item => item.analysis_id === updated.analysis_id ? updated : item))
      if (selectedAnalysis?.analysis_id === updated.analysis_id) setSelectedAnalysis(updated)
      if (showSavedOnly && !updated.saved) setAnalysisHistory(items => items.filter(item => item.analysis_id !== updated.analysis_id))
      addToast(updated.saved ? 'Analysis saved to the governed case history' : 'Analysis removed from saved results', 'success')
    } catch (err) {
      addToast(`Could not update saved analysis: ${err.message}`, 'error')
    }
  }

  const importBrowserHistory = async () => {
    setConfirmDialog(null)
    setAnalysisHistoryState('loading')
    try {
      const result = await agentsApi.importAnalysisHistory(name, conversations, userId, {
        caseId: forensicCaseId,
        collectionId: forensicCollectionId,
      })
      setLastHistoryImport(result)
      addToast(`Imported ${result.imported} analysis ${result.imported === 1 ? 'entry' : 'entries'}; ${result.skipped} duplicates skipped`, 'success')
      await loadAnalysisHistory()
    } catch (err) {
      setAnalysisHistoryState('error')
      addToast(`Browser history import failed: ${err.message}`, 'error')
    }
  }

  const rollbackBrowserImport = async () => {
    const importID = lastHistoryImport?.import_id
    setConfirmDialog(null)
    if (!importID) return
    try {
      const result = await agentsApi.rollbackAnalysisImport(name, importID, userId, {
        caseId: forensicCaseId,
        collectionId: forensicCollectionId,
      })
      setLastHistoryImport(null)
      setSelectedAnalysis(null)
      addToast(`Rolled back ${result.deleted} imported entries; saved or held entries were preserved`, 'info')
      await loadAnalysisHistory()
    } catch (err) {
      addToast(`Import rollback failed: ${err.message}`, 'error')
    }
  }

  const filteredConversations = conversations
    .filter(c => chatSearch.trim() ? (() => {
      const q = chatSearch.toLowerCase()
      if ((c.name || '').toLowerCase().includes(q)) return true
      return c.messages?.some(m => {
        return (m.content || '').toLowerCase().includes(q)
      })
    })() : true)

  return (
    <div className={`chat-layout${sidebarOpen ? '' : ' chat-sidebar-collapsed'}${portalMode ? ' analyst-chat-layout' : ''}`}>
      <style>{`
        .forensic-agent-ribbon { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: var(--spacing-md); align-items: center; padding: 10px var(--spacing-md); border-bottom: 1px solid var(--color-border); background: linear-gradient(110deg, color-mix(in srgb, var(--color-primary) 10%, var(--color-bg-secondary)), var(--color-bg-secondary)); }
        .forensic-agent-context { display: flex; flex-wrap: wrap; gap: 6px 14px; min-width: 0; align-items: center; }
        .forensic-agent-context strong { color: var(--color-text-primary); }
        .forensic-agent-context span { display: inline-flex; align-items: center; gap: 6px; color: var(--color-text-secondary); font-size: 0.78rem; min-width: 0; }
        .forensic-agent-context code { max-width: 360px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--color-text-primary); }
        .forensic-context-details { color:var(--color-text-muted); font-size:.72rem; }.forensic-context-details summary { cursor:pointer; font-weight:700; }.forensic-context-details > div { display:flex; flex-wrap:wrap; gap:6px 14px; margin-top:8px; padding:8px 10px; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-bg-primary); }
        .forensic-assurance { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 6px; }
        .forensic-assurance span { display: inline-flex; align-items: center; gap: 5px; padding: 4px 8px; border: 1px solid var(--color-border); border-radius: 999px; color: var(--color-text-secondary); background: color-mix(in srgb, var(--color-bg-primary) 82%, transparent); font-size: 0.72rem; white-space: nowrap; }
        .forensic-assurance i { color: var(--color-success); }
        .forensic-empty { width: min(920px, 100%); margin: auto; padding: var(--spacing-xl); }
        .forensic-empty-hero { text-align: left; border: 1px solid var(--color-border); border-radius: var(--radius-lg); padding: clamp(20px, 4vw, 34px); background: linear-gradient(145deg, color-mix(in srgb, var(--color-primary) 9%, var(--color-bg-secondary)), var(--color-bg-secondary)); box-shadow: var(--shadow-sm); }
        .forensic-empty-kicker { color: var(--color-primary); font-size: 0.75rem; font-weight: 700; letter-spacing: 0.08em; text-transform: uppercase; }
        .forensic-empty-hero h2 { margin: 8px 0; color: var(--color-text-primary); font-size: clamp(1.35rem, 2.5vw, 2rem); }
        .forensic-empty-hero p { margin: 0; max-width: 720px; color: var(--color-text-secondary); line-height: 1.6; }
        .forensic-prompt-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; margin-top: var(--spacing-md); }
        .forensic-prompt-card { display: grid; grid-template-columns: auto 1fr; gap: 10px; text-align: left; padding: 13px; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-bg-primary); color: var(--color-text-primary); cursor: pointer; transition: border-color .15s ease, transform .15s ease, background .15s ease; }
        .forensic-prompt-card:hover { border-color: var(--color-primary); background: color-mix(in srgb, var(--color-primary) 6%, var(--color-bg-primary)); transform: translateY(-1px); }
        .forensic-prompt-card:disabled { cursor:not-allowed; opacity:.58; transform:none; }
        .forensic-prompt-card > i { color: var(--color-primary); margin-top: 2px; }
        .forensic-prompt-card strong, .forensic-prompt-card span { display: block; }
        .forensic-prompt-card span { color: var(--color-text-muted); font-size: 0.74rem; line-height: 1.35; margin-top: 3px; }
        .forensic-query-shortcuts { display: flex; gap: 6px; overflow-x: auto; padding-bottom: 8px; scrollbar-width: thin; }
        .forensic-query-shortcuts button { flex: 0 0 auto; }
        .forensic-input-guidance { display: flex; align-items: center; justify-content: space-between; gap: var(--spacing-sm); margin-top: 7px; color: var(--color-text-muted); font-size: 0.72rem; }
        .forensic-input-guidance strong { color: var(--color-text-secondary); }
        .forensic-result { display:grid; gap:16px; min-width:0; }
        .forensic-result--clarification { padding:18px; border:1px solid color-mix(in srgb, var(--color-primary) 34%, var(--color-border)); border-radius:var(--radius-lg); background:color-mix(in srgb, var(--color-primary) 5%, var(--color-bg-primary)); }
        .forensic-result-header { display:flex; align-items:flex-start; justify-content:space-between; gap:14px; padding-bottom:14px; border-bottom:1px solid var(--color-border); }
        .forensic-result-eyebrow { color:var(--color-primary); font-size:.68rem; font-weight:800; letter-spacing:.09em; text-transform:uppercase; }
        .forensic-result-header h3 { margin:5px 0 0; color:var(--color-text-primary); font-size:1rem; line-height:1.45; }
        .forensic-result-badges,.forensic-live-header { display:flex; flex-wrap:wrap; gap:6px; }
        .forensic-result-badges span,.forensic-live-header span { display:inline-flex; align-items:center; gap:5px; padding:5px 8px; border:1px solid var(--color-border); border-radius:999px; background:var(--color-bg-secondary); color:var(--color-text-secondary); font-size:.68rem; white-space:nowrap; }
        .forensic-result-badges i,.forensic-live-header i { color:var(--color-success); }
        .forensic-metric-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(130px,1fr)); gap:8px; }
        .forensic-metric { padding:10px 12px; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-bg-secondary); }
        .forensic-metric span,.forensic-metric strong { display:block; }
        .forensic-metric span { color:var(--color-text-muted); font-size:.68rem; }
        .forensic-metric strong { margin-top:3px; color:var(--color-text-primary); font-size:1.05rem; }
        .forensic-model-interpretation { padding:12px 14px; border-left:3px solid var(--color-primary); border-radius:0 var(--radius-md) var(--radius-md) 0; background:color-mix(in srgb, var(--color-primary) 5%, var(--color-bg-secondary)); }
        .forensic-model-interpretation span { color:var(--color-primary); font-size:.68rem; font-weight:800; letter-spacing:.06em; text-transform:uppercase; }
        .forensic-model-interpretation p { margin:4px 0 0; color:var(--color-text-secondary); line-height:1.55; }
        .forensic-result-section h4 { margin:0 0 8px; color:var(--color-text-primary); font-size:.79rem; }
        .forensic-result-section ul,.forensic-result-section ol { margin:0; padding-left:19px; color:var(--color-text-secondary); }
        .forensic-section-heading { display:flex; justify-content:space-between; gap:12px; align-items:center; }
        .forensic-section-heading span { color:var(--color-text-muted); font-size:.68rem; }
        .forensic-table-wrap { max-width:100%; overflow:auto; border:1px solid var(--color-border); border-radius:var(--radius-md); }
        .forensic-table-wrap table { width:100%; border-collapse:collapse; font-size:.72rem; }
        .forensic-table-wrap th,.forensic-table-wrap td { padding:8px 10px; border-bottom:1px solid var(--color-border); text-align:left; white-space:nowrap; max-width:260px; overflow:hidden; text-overflow:ellipsis; }
        .forensic-table-wrap th { position:sticky; top:0; background:var(--color-bg-secondary); color:var(--color-text-primary); }
        .forensic-table-wrap th button { display:flex; align-items:center; gap:6px; width:100%; border:0; padding:0; background:none; color:inherit; font:inherit; font-weight:700; cursor:pointer; }
        .forensic-table-tools { display:flex; flex-wrap:wrap; gap:8px; margin:8px 0; align-items:flex-start; }
        .forensic-table-tools > label { display:flex; align-items:center; gap:7px; min-width:min(300px,100%); padding:7px 9px; border:1px solid var(--color-border); border-radius:var(--radius-md); }
        .forensic-table-tools input[type="text"],.forensic-table-tools label > input:not([type]) { min-width:0; flex:1; border:0; outline:0; background:transparent; color:var(--color-text-primary); }
        .forensic-table-tools details { position:relative; }
        .forensic-table-tools summary { cursor:pointer; padding:8px 11px; border:1px solid var(--color-border); border-radius:var(--radius-md); list-style:none; }
        .forensic-column-picker { position:absolute; z-index:8; inset-inline-end:0; width:min(320px,80vw); max-height:280px; overflow:auto; padding:10px; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-bg-primary); box-shadow:var(--shadow-lg); }
        .forensic-column-picker label { display:flex; gap:8px; padding:6px; color:var(--color-text-secondary); }
        .forensic-findings { display:grid; gap:7px; list-style:none; padding:0; }.forensic-findings li { display:grid; gap:3px; padding:9px 11px; border-inline-start:3px solid var(--color-primary); background:var(--color-bg-secondary); }.forensic-findings li > span { color:var(--color-text-muted); font-size:.66rem; font-weight:700; text-transform:uppercase; letter-spacing:.04em; }.forensic-findings .is-candidate-correlation,.forensic-findings .is-observed-association { border-color:var(--color-warning); }.forensic-findings .is-conflict { border-color:var(--color-error); }
        .forensic-relationships { display:grid; grid-template-columns:repeat(auto-fit,minmax(220px,1fr)); gap:8px; }.forensic-relationships article { display:grid; gap:5px; padding:10px; border:1px solid var(--color-border); border-radius:var(--radius-md); }.forensic-relationships article > span { color:var(--color-warning); font-size:.68rem; font-weight:700; text-transform:uppercase; }.forensic-relationships small { color:var(--color-text-muted); }
        .forensic-source em { display:block; margin-top:3px; color:var(--color-text-muted); font-size:.62rem; font-style:normal; text-transform:uppercase; }.forensic-state { font-weight:700; }.forensic-state--failed,.forensic-state--unavailable { color:var(--color-error); }.forensic-state--partial,.forensic-state--processing { color:var(--color-warning); }
        .forensic-mobile-detail,.forensic-mobile-detail-heading,.forensic-expanded-row { display:none; }.forensic-result-bound { color:var(--color-text-muted); font-size:.7rem; }
        .forensic-show-all { display:inline-flex; align-items:center; gap:7px; margin-top:8px; padding:7px 10px; border:0; background:transparent; color:var(--color-primary); font-weight:700; cursor:pointer; }
        .forensic-visual-stack { display:grid; grid-template-columns:minmax(0,1fr) minmax(220px,.55fr); gap:10px; align-items:start; }
        .forensic-visual { min-width:0; margin:0; padding:12px; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-bg-secondary); }
        .forensic-visual figcaption { display:flex; justify-content:space-between; gap:10px; margin-bottom:10px; }.forensic-visual figcaption strong,.forensic-visual figcaption span { display:block; }.forensic-visual figcaption span,.forensic-visual-provenance { color:var(--color-text-muted); font-size:.68rem; }
        .forensic-timeline { position:relative; display:grid; gap:5px; max-height:320px; overflow:auto; padding:0 0 0 14px !important; list-style:none; }.forensic-timeline::before { content:''; position:absolute; top:4px; bottom:4px; left:5px; width:1px; background:var(--color-border); }.forensic-timeline li { position:relative; }.forensic-timeline li::before { content:''; position:absolute; top:13px; left:-12px; width:7px; height:7px; border-radius:50%; background:var(--color-primary); box-shadow:0 0 0 3px var(--color-bg-secondary); }.forensic-timeline button { display:grid; width:100%; grid-template-columns:minmax(118px,.5fr) minmax(120px,1fr) auto; gap:7px; align-items:center; padding:7px 9px; border:1px solid transparent; border-radius:var(--radius-sm); background:transparent; color:inherit; text-align:left; cursor:pointer; }.forensic-timeline button:hover,.forensic-timeline .is-selected button { border-color:var(--color-primary); background:color-mix(in srgb,var(--color-primary) 7%,var(--color-bg-secondary)); }.forensic-timeline time,.forensic-timeline small { color:var(--color-text-muted); font-size:.66rem; }.forensic-timeline small { grid-column:2 / -1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
        .forensic-match { padding:3px 6px; border-radius:999px; background:color-mix(in srgb,var(--color-success) 12%,var(--color-bg-secondary)); color:var(--color-success); font-size:.6rem; font-weight:700; }.forensic-match--ambiguous-overlapping-references { background:color-mix(in srgb,var(--color-warning) 14%,var(--color-bg-secondary)); color:var(--color-warning); }.forensic-match--unmatched-no-reference,.forensic-match--unmatched-outside-validity-window { background:color-mix(in srgb,var(--color-danger) 10%,var(--color-bg-secondary)); color:var(--color-danger); }
        .forensic-coordinate-plot { position:relative; height:260px; overflow:hidden; border:1px solid var(--color-border); border-radius:var(--radius-md); background-image:linear-gradient(var(--color-border) 1px,transparent 1px),linear-gradient(90deg,var(--color-border) 1px,transparent 1px); background-size:25% 25%; background-color:color-mix(in srgb,var(--color-primary) 3%,var(--color-bg-primary)); }.forensic-coordinate-axis { position:absolute; top:7px; right:9px; color:var(--color-text-muted); font-size:.65rem; font-weight:800; }.forensic-coordinate-plot button { position:absolute; width:18px; height:18px; transform:translate(-50%,50%); border:2px solid var(--color-bg-primary); border-radius:50%; background:var(--color-primary); box-shadow:0 0 0 2px color-mix(in srgb,var(--color-primary) 28%,transparent); cursor:pointer; }.forensic-coordinate-plot button.is-selected { width:23px; height:23px; box-shadow:0 0 0 5px color-mix(in srgb,var(--color-primary) 25%,transparent); }.forensic-coordinate-plot button.is-ambiguous-overlapping-references { background:var(--color-warning); }.forensic-coordinate-plot button.is-unmatched-no-reference,.forensic-coordinate-plot button.is-unmatched-outside-validity-window { background:var(--color-danger); }.forensic-coordinate-plot em { position:absolute; top:18px; left:50%; transform:translateX(-50%); color:var(--color-text-muted); font-size:.58rem; font-style:normal; white-space:nowrap; }.forensic-visual-empty { padding:18px; border:1px dashed var(--color-border); border-radius:var(--radius-md); color:var(--color-text-muted); font-size:.72rem; }.forensic-visual-provenance { display:block; margin-top:8px; }
        .forensic-visual-summary { display:flex; gap:9px; padding:10px; border:1px solid var(--color-border); border-radius:var(--radius-md); }.forensic-visual-summary i { color:var(--color-primary); }.forensic-visual-summary span,.forensic-visual-summary small { display:block; }.forensic-visual-summary small { color:var(--color-text-muted); margin-top:3px; }
        .forensic-evidence-drawer { grid-column:2; grid-row:1 / span 2; padding:12px; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-bg-secondary); }.forensic-evidence-drawer > div span,.forensic-evidence-drawer > div strong { display:block; }.forensic-evidence-drawer > div span { color:var(--color-primary); font-size:.65rem; font-weight:800; letter-spacing:.06em; text-transform:uppercase; }.forensic-evidence-drawer > div strong { margin-top:4px; overflow-wrap:anywhere; }.forensic-evidence-drawer dl { display:grid; gap:8px; margin:12px 0 0; }.forensic-evidence-drawer dl div { padding-top:8px; border-top:1px solid var(--color-border); }.forensic-evidence-drawer dt { color:var(--color-text-muted); font-size:.64rem; }.forensic-evidence-drawer dd { margin:3px 0 0; color:var(--color-text-secondary); font-size:.7rem; overflow-wrap:anywhere; }
        .forensic-citations { display:grid; grid-template-columns:repeat(auto-fit,minmax(220px,1fr)); gap:8px; }
        .forensic-source { display:flex; width:100%; gap:10px; align-items:flex-start; padding:10px 12px; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-bg-secondary); color:inherit; text-align:left; font:inherit; min-width:0; }
        button.forensic-source { cursor:pointer; } button.forensic-source:hover { border-color:var(--color-primary); background:color-mix(in srgb,var(--color-primary) 6%,var(--color-bg-secondary)); } button.forensic-source:focus-visible { outline:2px solid var(--color-primary); outline-offset:2px; }.forensic-source__open { margin-left:auto !important; color:var(--color-text-muted) !important; }
        .forensic-source > i { color:var(--color-primary); margin-top:2px; }.forensic-source span,.forensic-source strong,.forensic-source small { display:block; min-width:0; }.forensic-source strong { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:var(--color-text-primary); font-size:.76rem; }.forensic-source small { margin-top:3px; color:var(--color-text-muted); font-size:.68rem; line-height:1.35; }
        .forensic-limitations { display:flex; gap:10px; padding:11px; border:1px solid color-mix(in srgb,var(--color-warning) 45%,var(--color-border)); border-radius:var(--radius-md); background:color-mix(in srgb,var(--color-warning) 7%,var(--color-bg-primary)); color:var(--color-text-secondary); }
        .forensic-limitations > i { color:var(--color-warning); }.forensic-limitations ul { margin:4px 0 0; padding-left:18px; }
        .forensic-followups { display:flex; flex-wrap:wrap; gap:7px; }.forensic-followups button { display:inline-flex; align-items:center; gap:8px; padding:7px 10px; border:1px solid var(--color-border); border-radius:999px; background:var(--color-bg-secondary); color:var(--color-text-primary); cursor:pointer; }.forensic-followups button:hover { border-color:var(--color-primary); }
        .forensic-trace { padding-top:10px; border-top:1px solid var(--color-border); color:var(--color-text-muted); font-size:.72rem; }.forensic-trace summary { cursor:pointer; font-weight:700; }.forensic-trace > div { display:flex; flex-wrap:wrap; gap:8px 14px; margin-top:8px; }.forensic-trace > details { margin-top:9px; padding:8px 10px; border:1px solid var(--color-border); border-radius:var(--radius-sm); }.forensic-trace pre { max-height:260px; overflow:auto; white-space:pre-wrap; word-break:break-word; color:var(--color-text-secondary); }.forensic-narrative { display:block !important; color:var(--color-text-secondary); }
        .forensic-live-header { justify-content:space-between; margin-bottom:10px; }.forensic-live-time { margin-left:auto; color:var(--color-text-muted); font-size:.68rem; }
        @media (max-width: 980px) { .forensic-agent-ribbon { grid-template-columns: 1fr; } .forensic-assurance { justify-content: flex-start; } .forensic-prompt-grid { grid-template-columns: 1fr 1fr; } }
        @media (max-width: 760px) { .forensic-visual-stack { grid-template-columns:1fr; }.forensic-evidence-drawer { grid-column:1; grid-row:auto; }.forensic-timeline button { grid-template-columns:1fr auto; }.forensic-timeline time { grid-column:1 / -1; }.forensic-timeline small { grid-column:1 / -1; } }
        @media (max-width: 620px) { .forensic-prompt-grid { grid-template-columns: 1fr; } .forensic-agent-context { display: grid; } .forensic-empty { padding: var(--spacing-md); } .forensic-result-header { display:grid; }.forensic-result-badges { justify-content:flex-start; }.forensic-table-wrap { width:100%; max-width:calc(100vw - 44px); }.forensic-table-tools > label { min-width:100%; }.forensic-table-wrap th:nth-child(n+4):not(.forensic-mobile-detail-heading),.forensic-table-wrap td:nth-child(n+4):not(.forensic-mobile-detail) { display:none; }.forensic-mobile-detail,.forensic-mobile-detail-heading { display:table-cell; }.forensic-mobile-detail button { border:1px solid var(--color-border); border-radius:6px; background:transparent; color:var(--color-primary); }.forensic-expanded-row { display:table-row !important; }.forensic-expanded-row td { display:table-cell !important; white-space:normal; max-width:none; }.forensic-expanded-row dl { display:grid; gap:8px; margin:0; }.forensic-expanded-row dl div { display:grid; grid-template-columns:minmax(90px,35%) 1fr; gap:8px; }.forensic-expanded-row dt { color:var(--color-text-muted); }.forensic-expanded-row dd { margin:0; overflow-wrap:anywhere; }.forensic-column-picker { inset-inline-start:0; inset-inline-end:auto; } }
      `}</style>
      {/* Conversation sidebar */}
      {!portalMode && <div id="agent-chat-sidebar" className={`chat-sidebar${sidebarOpen ? '' : ' hidden'}`}>
        <div className="chat-sidebar-header">
          <button className="btn btn-primary btn-sm" style={{ flex: 1 }} onClick={() => { setSelectedAnalysis(null); addConversation(); if (window.innerWidth <= 1023) setSidebarOpen(false) }}>
            <i className="fas fa-plus" /> New Chat
          </button>
          <button
            className="btn btn-secondary btn-sm"
            onClick={() => {
              setConfirmDialog({
                title: 'Delete All Conversations',
                message: 'Delete all conversations? This cannot be undone.',
                confirmLabel: 'Delete All',
                danger: true,
                onConfirm: () => { setConfirmDialog(null); deleteAllConversations() },
              })
            }}
            title="Delete all conversations"
            style={{ padding: '6px 8px' }}
          >
            <i className="fas fa-trash" />
          </button>
        </div>

        <div style={{ padding: '0 var(--spacing-sm)' }}>
          <div className="chat-search-wrapper">
            <i className="fas fa-search chat-search-icon" />
            <input
              className="chat-search-input"
              type="text"
              value={chatSearch}
              onChange={(e) => setChatSearch(e.target.value)}
              placeholder="Search conversations..."
            />
            {chatSearch && (
              <button className="chat-search-clear" onClick={() => setChatSearch('')}>
                <i className="fas fa-times" />
              </button>
            )}
          </div>
          {isForensicAgent && (
            <button
              className={`btn btn-sm ${showSavedOnly ? 'btn-primary' : 'btn-secondary'}`}
              type="button"
              onClick={() => setShowSavedOnly(value => !value)}
              title="Filter the server-authoritative governed case history"
              style={{ width: '100%', marginBottom: 'var(--spacing-xs)' }}
            >
              <i className="fas fa-bookmark" /> {showSavedOnly ? 'All case history' : 'Saved analyses'}
            </button>
          )}
        </div>

        {isForensicAgent && (
          <div style={{ padding: 'var(--spacing-xs) var(--spacing-sm) var(--spacing-sm)', borderBottom: '1px solid var(--color-border)' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 6, marginBottom: 6 }}>
              <strong style={{ fontSize: '0.75rem' }}>Retained case history</strong>
              <button className="btn btn-secondary btn-sm" type="button" onClick={() => loadAnalysisHistory()} aria-label="Refresh case history" title="Refresh retained history">
                <i className={`fas fa-${analysisHistoryState === 'loading' ? 'circle-notch fa-spin' : 'rotate'}`} />
              </button>
            </div>
            {analysisHistoryError && <div className="text-muted" style={{ fontSize: '0.72rem', marginBottom: 6 }}>Unavailable: {analysisHistoryError}</div>}
            {analysisHistory.map(entry => (
              <div key={entry.analysis_id} className={`chat-list-item ${selectedAnalysis?.analysis_id === entry.analysis_id ? 'active' : ''}`} onClick={() => { setSelectedAnalysis(entry); if (window.innerWidth <= 1023) setSidebarOpen(false) }}>
                <i className={`fas ${entry.saved ? 'fa-bookmark' : 'fa-clock-rotate-left'}`} style={{ fontSize: '0.7rem', flexShrink: 0 }} />
                <div className="chat-list-item-info">
                  <div className="chat-list-item-name">{entry.saved_title || entry.query || 'Retained analysis'}</div>
                  <span className="chat-list-item-preview">{entry.status} · {relativeTime(new Date(entry.created_at).getTime())}</span>
                </div>
                <div className="chat-list-item-actions">
                  <button type="button" onClick={(event) => { event.stopPropagation(); toggleRetainedAnalysis(entry) }} aria-label={entry.saved ? 'Unsave retained analysis' : 'Save retained analysis'} title={entry.saved ? 'Remove saved state' : 'Save analysis'}>
                    <i className={`${entry.saved ? 'fas' : 'far'} fa-bookmark`} aria-hidden="true" />
                  </button>
                </div>
              </div>
            ))}
            {analysisHistoryState === 'ready' && analysisHistory.length === 0 && <div className="text-muted" style={{ fontSize: '0.72rem' }}>No retained analyses for this case.</div>}
            {analysisHistoryHasMore && <button className="btn btn-secondary btn-sm" type="button" style={{ width: '100%', marginTop: 6 }} onClick={() => loadAnalysisHistory({ append: true, cursor: analysisHistoryCursor })}>Load more</button>}
            <button
              className="btn btn-secondary btn-sm"
              type="button"
              style={{ width: '100%', marginTop: 6 }}
              disabled={!conversations.some(conversation => conversation.messages?.length)}
              onClick={() => setConfirmDialog({
                title: 'Import browser history',
                message: 'Import this case’s browser-local conversations into governed server history? Existing matching entries are skipped. This never happens automatically.',
                confirmLabel: 'Import explicitly',
                onConfirm: importBrowserHistory,
              })}
            >
              <i className="fas fa-file-import" /> Import local history
            </button>
            {lastHistoryImport?.rollback_available && (
              <button className="btn btn-secondary btn-sm" type="button" style={{ width: '100%', marginTop: 6 }} onClick={() => setConfirmDialog({
                title: 'Roll back latest import',
                message: 'Delete unsaved, non-held entries from the latest browser import? Saved and legal-hold entries will be preserved.',
                confirmLabel: 'Roll back import', danger: true, onConfirm: rollbackBrowserImport,
              })}>
                <i className="fas fa-rotate-left" /> Undo latest import
              </button>
            )}
          </div>
        )}

        <div className="chat-list">
          {isForensicAgent && <div className="text-muted" style={{ padding: '4px var(--spacing-sm)', fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.06em' }}>Live browser sessions</div>}
          {filteredConversations.map(conv => (
            <div
              key={conv.id}
              className={`chat-list-item ${conv.id === activeId ? 'active' : ''}`}
              onClick={() => { setSelectedAnalysis(null); switchConversation(conv.id); if (window.innerWidth <= 1023) setSidebarOpen(false) }}
            >
              <i className="fas fa-message" style={{ fontSize: '0.7rem', flexShrink: 0, marginTop: '2px' }} />
              {editingName === conv.id ? (
                <input
                  className="input"
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  onBlur={finishRename}
                  onKeyDown={(e) => e.key === 'Enter' && finishRename()}
                  autoFocus
                  onClick={(e) => e.stopPropagation()}
                  style={{ padding: '2px 4px', fontSize: '0.8125rem' }}
                />
              ) : (
                <div className="chat-list-item-info">
                  <div className="chat-list-item-top">
                    <span
                      className="chat-list-item-name"
                      onDoubleClick={() => startRename(conv.id, conv.name)}
                    >
                      {processingChatId === conv.id && <i className="fas fa-circle-notch fa-spin" style={{ marginRight: '6px', fontSize: '0.7rem', opacity: 0.7 }} />}
                      {conv.name}
                    </span>
                    <span className="chat-list-item-time">{relativeTime(conv.updatedAt)}</span>
                  </div>
                  <span className="chat-list-item-preview">
                    {getLastMessagePreview(conv) || 'No messages yet'}
                  </span>
                </div>
              )}
              <div className="chat-list-item-actions">
                <button
                  onClick={(e) => { e.stopPropagation(); startRename(conv.id, conv.name) }}
                  title="Rename"
                >
                  <i className="fas fa-edit" />
                </button>
                {conversations.length > 1 && (
                  <button
                    className="chat-list-item-delete"
                    onClick={(e) => { e.stopPropagation(); deleteConversation(conv.id) }}
                    title="Delete conversation"
                  >
                    <i className="fas fa-trash" />
                  </button>
                )}
              </div>
            </div>
          ))}
          {filteredConversations.length === 0 && chatSearch && (
            <div style={{ padding: 'var(--spacing-sm)', textAlign: 'center', color: 'var(--color-text-muted)', fontSize: '0.8rem' }}>
              No conversations match your search
            </div>
          )}
        </div>
      </div>}

    <div className="chat-main">
      {/* Header */}
      <div className="chat-header">
        {!portalMode && <button
          className="btn btn-secondary btn-sm"
          onClick={() => setSidebarOpen(prev => !prev)}
          title={sidebarOpen ? 'Hide chat list' : 'Show chat list'}
          aria-label={sidebarOpen ? 'Hide conversations and case history' : 'Show conversations and case history'}
          aria-controls="agent-chat-sidebar"
          aria-expanded={sidebarOpen}
          style={{ flexShrink: 0 }}
        >
          <i className={`fas fa-${sidebarOpen ? 'angles-left' : 'angles-right'}`} aria-hidden="true" />
        </button>}
        <span className={`chat-header-title${portalMode ? ' sr-only' : ''}`}>
          <i className={`fas ${isForensicAgent ? 'fa-shield-halved' : 'fa-robot'}`} style={{ marginRight: 'var(--spacing-xs)' }} />
          {selectedAnalysis ? 'Retained analysis' : isForensicAgent && portalMode ? 'Ask' : isForensicAgent ? 'Ask NexusAI' : name}
        </span>
        <div className="chat-header-actions">
          {!portalMode && <><label className="canvas-mode-toggle" title="Extract code blocks and media into a side panel for preview, copy, and download">
            <i className="fas fa-columns" />
            <span className="canvas-mode-label">Canvas</span>
            <span className={`toggle${canvasMode ? ' toggle--on' : ''}`}>
              <input
                type="checkbox"
                checked={canvasMode}
                onChange={(e) => {
                  setCanvasMode(e.target.checked)
                  if (!e.target.checked) setCanvasOpen(false)
                }}
              />
              <span className="toggle__track">
                <span className="toggle__thumb" />
              </span>
            </span>
          </label>
          {canvasMode && artifacts.length > 0 && !canvasOpen && (
            <button
              className="btn btn-secondary btn-sm"
              onClick={() => { setSelectedArtifactId(artifacts[0]?.id); setCanvasOpen(true) }}
              title="Open canvas panel"
            >
              <i className="fas fa-layer-group" /> {artifacts.length}
            </button>
          )}
          <button className="btn btn-secondary btn-sm" onClick={() => navigate(`/app/agents/${encodeURIComponent(name)}/status${userId ? `?user_id=${encodeURIComponent(userId)}` : ''}`)} title="View status & observables">
            <i className="fas fa-chart-bar" /> Status
          </button></>}
          {selectedAnalysis ? (
            <button className="btn btn-secondary btn-sm" type="button" onClick={() => setSelectedAnalysis(null)}>
              <i className="fas fa-arrow-left" /> Live chat
            </button>
          ) : (!portalMode || messages.length > 0) && (
            <button className="btn btn-secondary btn-sm" onClick={() => clearMessages()} disabled={messages.length === 0} title="Clear chat history">
              <i className="fas fa-eraser" /> Clear
            </button>
          )}
        </div>
      </div>

      {isForensicAgent && !portalMode && (
        <div className="forensic-agent-ribbon" aria-label={portalMode ? 'Ask workspace context' : 'Ask NexusAI case context'}>
          {portalMode ? <div className="analyst-ask-scope" data-scope={portalScope?.kind || 'workspace'}>
            <span className="analyst-ask-scope__icon" aria-hidden="true"><i className={`fas ${portalScope?.kind === 'evidence' ? 'fa-file-shield' : 'fa-folder-open'}`} /></span>
            <span><small>Investigation context</small><strong><bdi dir="auto">{portalScope?.label || 'Current workspace'}</bdi></strong><em>{portalScope?.kind === 'evidence' ? `Evidence in ${portalScope.detail}` : portalScope?.detail || 'Entire workspace'}</em></span>
            {portalScope?.kind === 'evidence' && <button type="button" onClick={() => navigate(`/analyst?case=${encodeURIComponent(forensicCaseId)}&panel=evidence&evidence=${encodeURIComponent(portalScope.evidenceId)}`)}>View evidence <i className="fas fa-arrow-up-right-from-square" aria-hidden="true" /></button>}
          </div> : <div className="forensic-agent-context">
            <label className="forensic-case-selector"><i className="fas fa-folder-open" /><strong>Case</strong>
              <select
                value={forensicCaseId}
                onChange={(event) => {
                  const next = new URLSearchParams(searchParams)
                  next.set('case', event.target.value)
                  setSearchParams(next, { replace: true })
                }}
                aria-label="Active forensic case"
                disabled={registryState !== 'ready'}
              >
                {!forensicCaseId && <option value="">Select case</option>}
                {caseOptions.map(item => <option key={item.caseId} value={item.caseId}>{item.displayName}</option>)}
              </select>
            </label>
            <span><i className="fas fa-wand-magic-sparkles" /><strong>{portalMode ? 'Ask' : 'Ask NexusAI'}</strong> {portalMode ? 'The workspace chooses an available verified analysis' : 'Automatic specialist'}</span>
            <details className="forensic-context-details"><summary>Status &amp; scope</summary><div><span><strong>Collection</strong> <code>{forensicCollectionId || 'Resolving'}</code></span><span><strong>Model</strong> <code title={activeModel}>{activeModel}</code></span><span><strong>Tenant</strong> <code>{agentConfig?.forensic_tenant_id || 'default'}</code></span></div></details>
          </div>}
          <div className="forensic-assurance" aria-label="Answer assurance">
            <span><i className="fas fa-shield-halved" /> {portalMode ? 'Findings stay connected to evidence' : 'Verified records & cited evidence'}</span>
            <button
              className="btn btn-secondary btn-sm"
              type="button"
              disabled={!askReturnUrl}
              onClick={() => navigate(askReturnUrl)}
              title="Return to the canonical case question workspace"
            >
              <i className="fas fa-arrow-left" /> Back to {portalMode ? 'Ask' : 'Ask NexusAI'}
            </button>
          </div>
        </div>
      )}

      {agentConfigError && (
        <div className="alert alert-warning" style={{ margin: 'var(--spacing-sm) var(--spacing-md) 0' }}>
          {portalMode ? 'Question answering is temporarily unavailable. Your workspace and retained evidence are unchanged.' : `Agent runtime details are unavailable: ${agentConfigError}`}
        </div>
      )}

      {isForensicAgent && forensicCaseError && (
        <div className="alert alert-error" role="alert" style={{ margin: 'var(--spacing-sm) var(--spacing-md) 0' }}>
          {portalMode ? 'This workspace cannot be used for questions right now. Check your access or choose another workspace.' : `Case scope is invalid: ${forensicCaseError}`}
        </div>
      )}

      {isForensicAgent && !forensicCaseError && forensicChatUnavailable && (
        <div className="alert alert-info" role="status" style={{ margin: 'var(--spacing-sm) var(--spacing-md) 0' }}>
          Checking access to this investigation…
        </div>
      )}

      {/* Messages */}
      <div className="chat-messages" ref={messagesRef} tabIndex={-1} aria-label="Conversation messages">
        {messages.length === 0 && !processing && (
          isForensicAgent ? (
            <div className="forensic-empty">
              <div className="forensic-empty-hero">
                {portalMode && <span className="forensic-empty-icon" aria-hidden="true"><i className="fas fa-magnifying-glass" /></span>}
                {!portalMode && <div className="forensic-empty-kicker">Evidence-first investigation workspace</div>}
                <h2>{portalMode ? 'Ask about your evidence' : 'Ask one question across the complete case'}</h2>
                <p>
                  {portalMode
                    ? 'Ask a question, or start with one of these suggestions.'
                    : 'NexusAI automatically routes exact counts, joins and timelines to deterministic SQL, retrieves relevant Knowledge Base evidence, and uses the local model only to explain bounded, cited results. Choose a workflow or type any case question below.'}
                </p>
                <div className="forensic-prompt-grid">
                  {(portalMode ? visibleForensicPrompts.slice(0, 3) : visibleForensicPrompts).map(prompt => (
                    <button className="forensic-prompt-card" type="button" key={prompt.corpusId || prompt.query} onClick={() => portalMode ? setInput(prompt.query) : handleSend(prompt.query)} disabled={forensicChatUnavailable || !conversationScopeReady}>
                      <i className={`fas ${prompt.icon}`} />
                      <span><strong>{prompt.label}</strong>{prompt.description && <span>{prompt.description}</span>}</span>
                    </button>
                  ))}
                </div>
              </div>
            </div>
          ) : (
            <div className="chat-empty-state">
              <div className="chat-empty-icon">
                <i className="fas fa-robot" />
              </div>
              <h2 className="chat-empty-title">Chat with {name}</h2>
              <p className="chat-empty-text">Send a message to start a conversation with this agent.</p>
              <div className="chat-empty-hints">
                <span><i className="fas fa-keyboard" /> Enter to send</span>
                <span><i className="fas fa-level-down-alt" /> Shift+Enter for newline</span>
              </div>
            </div>
          )
        )}
        {(() => {
          const elements = []
          let systemBuf = []
          const flushSystem = (key) => {
            if (systemBuf.length > 0) {
              if (!portalMode) elements.push(<AgentActivityGroup key={`sag-${key}`} items={[...systemBuf]} />)
              systemBuf = []
            }
          }
          messages.forEach((msg, idx) => {
            const role = senderToRole(msg.sender)
            if (role === 'system') {
              systemBuf.push(msg)
              return
            }
            flushSystem(idx)
            elements.push(
              <div key={msg.id} className={`chat-message chat-message-${role}`}>
                <div className="chat-message-avatar">
                  <i className={`fas ${role === 'user' ? 'fa-user' : 'fa-robot'}`} />
                </div>
                <div className="chat-message-bubble">
                  <div className="chat-message-content">
                    {role === 'user' ? (
                      <div dir="auto" style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{msg.content}</div>
                    ) : msg.metadata?.presentation ? (
                      <ForensicPresentation metadata={msg.metadata} narrative={msg.content} answerLabel={portalMode ? 'Answer' : 'Ask NexusAI answer'} analystMode={portalMode} entityChoices={authoritativeEntityChoices} onFollowUp={(query) => handleSend(query)} resolveCitationEvidenceId={resolveCitationEvidenceId} onOpenSource={(source) => navigate(forensicSourceRoute(portalMode, forensicCaseId, source))} />
                    ) : (
                      <div dangerouslySetInnerHTML={{
                        __html: canvasMode
                          ? renderMarkdownWithArtifacts(msg.content, idx)
                          : renderMarkdown(msg.content)
                      }} />
                    )}
                  </div>
                  {role === 'assistant' && msg.metadata && (
                    <ResourceCards
                      metadata={msg.metadata}
                      messageIndex={idx}
                      agentName={name}
                      onOpenArtifact={openArtifactById}
                    />
                  )}
                  <div className="chat-message-actions">
                    <button onClick={() => copyMessage(msg.content)} title="Copy">
                      <i className="fas fa-copy" />
                    </button>
                  </div>
                  <div className="chat-message-timestamp">
                    {new Date(msg.timestamp).toLocaleTimeString()}
                  </div>
                </div>
              </div>
            )
          })
          flushSystem('end')
          return elements
        })()}
        {portalMode && processing && (
          <section className="analyst-answer-state analyst-answer-state--processing analyst-answer-state--live" role="status" aria-live="polite" aria-label="Analysis in progress">
            <div className="analyst-answer-state__label"><i className="fas fa-spinner fa-spin" aria-hidden="true" /><span><strong>{cancelPending ? 'Stopping request…' : streamContent ? 'Preparing answer…' : 'Processing your request…'}</strong><small>{Math.floor(requestElapsedMs / 1000)}s elapsed. {cancelPending ? 'Waiting for the server to confirm cancellation.' : 'The answer will appear here when it is ready. You can stop this request below.'}</small></span></div>
            {!cancelPending && <div className="analyst-answer-skeleton" aria-hidden="true"><span className="analyst-answer-skeleton__title" /><span /><span /><span className="analyst-answer-skeleton__short" /><div><i /><i /><i /></div></div>}
          </section>
        )}
        {!portalMode && processing && (streamReasoning || streamContent || streamToolCalls.length > 0) && (
          <div className="chat-message chat-message-assistant" role="status" aria-live="polite" aria-label="Forensic analysis in progress">
            <div className="chat-message-avatar">
              <i className="fas fa-robot" />
            </div>
            <div className="chat-message-bubble">
              <div className="forensic-live-header"><span><i className="fas fa-shield-halved" /> Deterministic workflow</span><span><i className="fas fa-stopwatch" /> {(requestElapsedMs / 1000).toFixed(1)}s</span></div>
              {streamReasoning && (
                <details className="chat-activity-group" open={!streamContent} style={{ marginBottom: streamContent ? 'var(--spacing-sm)' : 0 }}>
                  <summary className="chat-activity-toggle" style={{ cursor: 'pointer' }}>
                    <span className={`chat-activity-summary${!streamContent ? ' chat-activity-shimmer' : ''}`}>
                      {streamContent ? 'Model explanation prepared' : 'Preparing bounded explanation…'}
                    </span>
                  </summary>
                  <div className="chat-activity-details">
                    <div className="chat-activity-item chat-activity-thinking">
                      <div className="chat-activity-item-content chat-activity-live"
                        dangerouslySetInnerHTML={{ __html: renderMarkdown(streamReasoning) }} />
                    </div>
                  </div>
                </details>
              )}
              {streamToolCalls.length > 0 && (
                <div className="chat-activity-group" style={{ marginBottom: 'var(--spacing-sm)' }}>
                  {streamToolCalls.map((tc, idx) => (
                    <details key={idx} className="chat-activity-item chat-activity-tool-call" style={{ padding: 'var(--spacing-xs) var(--spacing-sm)' }} open={!tc.result}>
                      <summary className="chat-activity-item-label" style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 'var(--spacing-xs)' }}>
                        <i className={`fas ${tc.result ? 'fa-check' : 'fa-bolt'}`} />
                        <strong>{forensicToolStage(tc.name, Boolean(tc.result))}</strong>
                        <span style={{ opacity: 0.5, fontSize: '0.85em' }}>
                          {tc.result ? 'done' : 'calling...'}
                        </span>
                      </summary>
                      {tc.args && (
                        <pre style={{ margin: '4px 0', fontSize: '0.75rem', opacity: 0.8, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                          {(() => { try { return JSON.stringify(JSON.parse(tc.args), null, 2) } catch { return tc.args } })()}
                        </pre>
                      )}
                      {tc.result && (
                        <pre style={{ margin: '4px 0', fontSize: '0.75rem', opacity: 0.7, whiteSpace: 'pre-wrap', wordBreak: 'break-word', maxHeight: '200px', overflow: 'auto' }}>
                          {tc.result}
                        </pre>
                      )}
                    </details>
                  ))}
                </div>
              )}
              {streamContent && (
                <div className="chat-message-content">
                  <span dangerouslySetInnerHTML={{ __html: renderMarkdown(streamContent) }} />
                  <span className="chat-streaming-cursor" />
                </div>
              )}
            </div>
          </div>
        )}
        {!portalMode && processing && !streamReasoning && !streamContent && streamToolCalls.length === 0 && (
          <div className="chat-message chat-message-assistant" role="status" aria-live="polite" aria-label="Forensic analysis in progress">
            <div className="chat-message-avatar" style={{ background: 'var(--color-bg-tertiary)', color: 'var(--color-text-muted)' }}>
              <i className="fas fa-cogs" />
            </div>
            <div className="chat-activity-group chat-activity-streaming">
              <div className="chat-activity-toggle" style={{ cursor: 'default' }}>
                <span className="chat-activity-summary chat-activity-shimmer">Planning governed case analysis…</span>
                <span className="forensic-live-time">{(requestElapsedMs / 1000).toFixed(1)}s · deterministic authority</span>
              </div>
            </div>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>
      {showJumpLatest && <button type="button" className="chat-jump-latest" onClick={jumpToLatest}><i className="fas fa-arrow-down" /> Jump to latest</button>}

      {/* Input area */}
      <div className="chat-input-area">
        {selectedAnalysis && (
          <div className="alert alert-info" role="status" style={{ marginBottom: 'var(--spacing-sm)', display: 'flex', justifyContent: 'space-between', gap: 'var(--spacing-sm)', alignItems: 'center' }}>
            <span><strong>Retained system-of-record view.</strong> Request {selectedAnalysis.message_id} · {selectedAnalysis.execution_authority} · {selectedAnalysis.status}</span>
            <button className="btn btn-secondary btn-sm" type="button" onClick={() => toggleRetainedAnalysis(selectedAnalysis)}>
              <i className={`${selectedAnalysis.saved ? 'fas' : 'far'} fa-bookmark`} aria-hidden="true" /> {selectedAnalysis.saved ? 'Unsave' : 'Save'}
            </button>
          </div>
        )}
        {retryCandidate && retryCandidate.scope === activeChatScope && (
          <div className="alert alert-warning" role="status" style={{ marginBottom: 'var(--spacing-sm)', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 'var(--spacing-sm)' }}>
            <span><strong>Request did not complete.</strong> Check Activity before retrying. The retry will not create a duplicate request.</span>
            <button className="btn btn-secondary btn-sm" type="button" onClick={handleRetry} disabled={processing}>
              <i className="fas fa-rotate" /> Retry once
            </button>
          </div>
        )}
        {isForensicAgent && !selectedAnalysis && (
          <div className="forensic-query-shortcuts" aria-label="Suggested forensic queries">
            {visibleForensicPrompts.slice(0, 4).map(prompt => (
              <button className="btn btn-secondary btn-sm" type="button" key={prompt.corpusId || prompt.query} onClick={() => portalMode ? setInput(prompt.query) : handleSend(prompt.query)} disabled={processing || forensicChatUnavailable || !conversationScopeReady}>
                <i className={`fas ${prompt.icon}`} /> {prompt.label}
              </button>
            ))}
          </div>
        )}
        <div className="chat-input-wrapper">
          <textarea
            ref={textareaRef}
            className="chat-input"
            value={input}
            onChange={(e) => {
              setInput(e.target.value)
              const ta = e.target
              ta.style.height = 'auto'
              ta.style.height = Math.min(ta.scrollHeight, 150) + 'px'
            }}
            onKeyDown={handleKeyDown}
            placeholder={portalMode ? 'Ask about your evidence...' : isForensicAgent ? 'Ask about calls, plates, transactions, entities, evidence or case readiness...' : 'Type a message...'}
            aria-label={isForensicAgent ? 'Analyst query' : 'Message'}
            dir="auto"
            disabled={Boolean(selectedAnalysis) || processing || agentConfig === undefined || !conversationScopeReady || forensicChatUnavailable}
            rows={1}
          />
          {processing && isForensicAgent ? (
            <button className="chat-send-btn" onClick={handleCancel} disabled={cancelPending} aria-label="Stop request" title="Stop request">
              <i className={`fas ${cancelPending ? 'fa-circle-notch fa-spin' : 'fa-stop'}`} aria-hidden="true" />
            </button>
          ) : (
            <button
              className="chat-send-btn"
              onClick={handleSend}
              disabled={Boolean(selectedAnalysis) || processing || agentConfig === undefined || !conversationScopeReady || forensicChatUnavailable || !input.trim()}
              aria-label={portalMode ? 'Ask question' : 'Send message'}
              title={portalMode ? 'Ask question' : 'Send message'}
            >
              <i className={`fas ${portalMode ? 'fa-arrow-up' : 'fa-paper-plane'}`} aria-hidden="true" />{portalMode && <span className="sr-only">Ask</span>}
            </button>
          )}
        </div>
        {isForensicAgent && !selectedAnalysis && !portalMode && (
          <div className="forensic-input-guidance">
            <span><strong>Authority:</strong> deterministic records and cited evidence</span>
            <span><strong>Model role:</strong> plain-language explanation after exact analysis</span>
          </div>
        )}
      </div>
    </div>
    {canvasOpen && artifacts.length > 0 && (
      <CanvasPanel
        artifacts={artifacts}
        selectedId={selectedArtifactId}
        onSelect={setSelectedArtifactId}
        onClose={() => setCanvasOpen(false)}
      />
    )}
    <ConfirmDialog
      open={!!confirmDialog}
      title={confirmDialog?.title}
      message={confirmDialog?.message}
      confirmLabel={confirmDialog?.confirmLabel}
      danger={confirmDialog?.danger}
      onConfirm={confirmDialog?.onConfirm}
      onCancel={() => setConfirmDialog(null)}
    />
    </div>
  )
}
