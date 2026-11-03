import { lazy, Suspense, useMemo, useState } from 'react'
import { Link, useInRouterContext } from 'react-router-dom'
import { ExternalLink } from 'lucide-react'
import { ResultChart } from '../../../components/ResultChart.jsx'
import { CarrierBadge } from '../../../components/CarrierBadge.jsx'
import { graphTargets, mapSites, resultShapes } from '../../../lib/resultShapes.js'
import { modelFromRows } from '../../../lib/connectionsModel.js'
import { isDrawable } from '../../../lib/workbench/answerMarks.js'
import { formatNumber } from '../../../lib/format.js'
import { DataTable } from '../../ui/DataTable.jsx'

// Code-split with the rest: Cytoscape and MapLibre are fetched only when an answer is a graph or a map.
const LinkChart = lazy(() => import('../../../components/LinkChart.jsx').then(m => ({ default: m.LinkChart })))
const GeoMap = lazy(() => import('../../../components/GeoMap.jsx').then(m => ({ default: m.GeoMap })))
const WorkbenchResult = lazy(() => import('../../../pages/workbench/WorkbenchResult.jsx').then(m => ({ default: m.WorkbenchResult })))

const plural = (n, noun) => `${formatNumber(n)} ${noun}${n === 1 ? '' : 's'}`
const normalized = value => String(value ?? '').replaceAll(',', '').trim().toLocaleLowerCase()

// The result of an answer in the view that fits its shape, the table always one click away. A table is shown whole, with its row count said.
function AnswerTable({ result, citations }) {
  if (!result.columns.length || !result.rows.length) return <p className="nx-muted">No tabular result was returned.</p>
  function renderCell(row, column, value) {
    const citation = citations?.items?.find(item => item.claimValues?.some(candidate => normalized(candidate) === normalized(value)))
    const href = citation?.href || row.__citationHref
    return href ? <a className="nx-cell-link" href={href}>{String(value)}<span className="nx-visually-hidden"> — open source</span></a> : String(value)
  }
  return (
    <DataTable
      columns={result.columns} rows={result.rows} totals={result.totals} label="Result table" caption="The rows of this answer"
      renderCell={renderCell} exportName="nexusai-investigation-result.csv"
      note={result.truncated ? `the service returned ${result.availableRows.toLocaleString()} of ${result.totalRows.toLocaleString()} matching rows; the export holds the returned rows` : undefined}
    />
  )
}

function OpenIn({ to, children }) {
  const inRouter = useInRouterContext()
  return inRouter ? <Link className="nx-btn nx-btn--sm" to={to}>{children}<ExternalLink aria-hidden="true" /></Link> : null
}

function AnswerGraph({ rows, caseId }) {
  const [selected, setSelected] = useState(null)
  const model = useMemo(() => modelFromRows(rows, { weight: ['weight', 'events', 'value'].find(key => rows.some(row => row[key] != null)) || null }), [rows])
  const node = model.nodes.find(n => n.id === selected)
  const params = new URLSearchParams({ t: graphTargets(rows).join(',') })
  return (
    <figure className="nx-figure">
      <Suspense fallback={<p className="nx-figure__loading" role="status">Loading the chart…</p>}>
        <LinkChart model={model} selectedId={selected} onSelectNode={setSelected} label="Link chart of this answer" />
      </Suspense>
      <figcaption>
        <span>{plural(model.edges.length, 'link')} between {plural(model.nodes.length, 'number')}. A link is a record of contact, not a finding about the people.</span>
        {node ? <span className="nx-figure__selected"><CarrierBadge carrier={node.carrier} size={16} showName /><b>{node.label}</b></span> : null}
        {caseId ? <OpenIn to={`/cases/${encodeURIComponent(caseId)}/connections?${params}`}>Open in Connections</OpenIn> : null}
      </figcaption>
    </figure>
  )
}

function AnswerMap({ rows, caseId }) {
  const sites = useMemo(() => mapSites(rows), [rows])
  const [selected, setSelected] = useState(null)
  return (
    <figure className="nx-figure">
      <Suspense fallback={<p className="nx-figure__loading" role="status">Loading the map…</p>}>
        <GeoMap sites={sites} selectedId={selected} onSelect={setSelected} height={380} label="Map of this answer" />
      </Suspense>
      <figcaption>
        <span>{plural(sites.length, 'cell site')}. A cell site is a coverage area with a stated radius, not the position of a person.</span>
        {caseId ? <OpenIn to={`/cases/${encodeURIComponent(caseId)}/timeline?view=places`}>Open in Places</OpenIn> : null}
      </figcaption>
    </figure>
  )
}

export function ResultBlock({ presentation, caseId, onAsk }) {
  const shapes = useMemo(() => resultShapes(presentation), [presentation])
  const [chosen, setChosen] = useState(null)
  const drawable = useMemo(() => !!caseId && !!presentation.question && isDrawable(presentation.original, presentation.question), [caseId, presentation])
  const spatial = shapes.views.some(view => view.id === 'graph' || view.id === 'map')
  const table = <AnswerTable result={presentation.result} citations={presentation.citations} />
  const classic = (() => {
    if (!spatial) return <><ResultChart presentation={presentation} />{table}</>
    const current = shapes.views.some(view => view.id === chosen) ? chosen : shapes.first
    return (
      <div className="nx-result-views">
        <div className="nx-segmented" role="tablist" aria-label="How to see this result">
          {shapes.views.map(view => (
            <button key={view.id} type="button" role="tab" id={`rv-${view.id}`} aria-selected={current === view.id} aria-controls={`rv-panel-${view.id}`} onClick={() => setChosen(view.id)}>{view.label}</button>
          ))}
        </div>
        <div role="tabpanel" id={`rv-panel-${current}`} aria-labelledby={`rv-${current}`}>
          {current === 'graph' ? <AnswerGraph rows={shapes.rows} caseId={caseId} /> : null}
          {current === 'map' ? <AnswerMap rows={shapes.rows} caseId={caseId} /> : null}
          {current === 'chart' ? <ResultChart presentation={presentation} /> : null}
          {current === 'table' ? table : null}
        </div>
      </div>
    )
  })()
  if (!drawable) return classic
  return (
    <Suspense fallback={<p className="nx-figure__loading" role="status">Preparing the picture of this answer…</p>}>
      <WorkbenchResult presentation={presentation} caseId={caseId} onAsk={onAsk} fallback={classic} />
    </Suspense>
  )
}
