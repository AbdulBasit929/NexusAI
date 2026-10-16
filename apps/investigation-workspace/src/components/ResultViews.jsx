import { lazy, Suspense, useMemo, useState } from 'react'
import { Link, useInRouterContext, useParams } from 'react-router-dom'
import { ResultChart } from './ResultChart.jsx'
import { ResultTable } from './ResultTable.jsx'
import { CarrierBadge } from './CarrierBadge.jsx'
import { graphTargets, mapSites, resultShapes } from '../lib/resultShapes.js'
import { modelFromRows } from '../lib/connectionsModel.js'
import { formatNumber } from '../lib/format.js'

// Code-split with the rest: Cytoscape and MapLibre are fetched only when an answer is a graph or a map.
const LinkChart = lazy(() => import('./LinkChart.jsx').then(m => ({ default: m.LinkChart })))
const GeoMap = lazy(() => import('./GeoMap.jsx').then(m => ({ default: m.GeoMap })))

const plural = (n, noun) => `${formatNumber(n)} ${noun}${n === 1 ? '' : 's'}`

function OpenIn({ to, children }) {
  const inRouter = useInRouterContext()
  return inRouter ? <Link className="cx-button" to={to}>{children}</Link> : null
}

function AnswerGraph({ rows, caseId }) {
  const [selected, setSelected] = useState(null)
  const model = useMemo(() => modelFromRows(rows, { weight: ['weight', 'events', 'value'].find(key => rows.some(row => row[key] != null)) || null }), [rows])
  const node = model.nodes.find(n => n.id === selected)
  const targets = graphTargets(rows)
  const params = new URLSearchParams({ t: targets.join(',') })
  return (
    <figure className="result-graph">
      <Suspense fallback={<p className="chart-card__loading" role="status">Loading the chart…</p>}>
        <LinkChart model={model} selectedId={selected} onSelectNode={setSelected} label="Link chart of this answer" />
      </Suspense>
      <figcaption>
        <span>{plural(model.edges.length, 'link')} between {plural(model.nodes.length, 'number')}. A link is a record of contact, not a finding about the people.</span>
        {node ? <span className="result-graph__selected"><CarrierBadge carrier={node.carrier} size={16} showName /><b>{node.label}</b></span> : null}
        {caseId ? <OpenIn to={`/cases/${encodeURIComponent(caseId)}/connections?${params}`}>Open in Connections</OpenIn> : null}
      </figcaption>
    </figure>
  )
}

function AnswerMap({ rows, caseId }) {
  const sites = useMemo(() => mapSites(rows), [rows])
  const [selected, setSelected] = useState(null)
  return (
    <figure className="result-map">
      <Suspense fallback={<p className="chart-card__loading" role="status">Loading the map…</p>}>
        <GeoMap sites={sites} selectedId={selected} onSelect={setSelected} height={380} label="Map of this answer" />
      </Suspense>
      <figcaption>
        <span>{plural(sites.length, 'cell site')}. A cell site is a coverage area with a stated radius, not the position of a person.</span>
        {caseId ? <OpenIn to={`/cases/${encodeURIComponent(caseId)}/timeline?view=places`}>Open in Places</OpenIn> : null}
      </figcaption>
    </figure>
  )
}

// The result of an answer, in the view that fits its shape, with the table always one click away. An answer that is only a table, or a table with a bar chart, is shown exactly as it
// always was (the chart above the table); a graph or a map gets a switcher.
export function ResultViews({ presentation, caseId: caseIdProp = '' }) {
  const params = useParams()
  const caseId = caseIdProp || params?.id || ''
  const shapes = useMemo(() => resultShapes(presentation), [presentation])
  const [chosen, setChosen] = useState(null)
  const spatial = shapes.views.some(view => view.id === 'graph' || view.id === 'map')
  const table = <ResultTable result={presentation.result} citations={presentation.citations} />
  if (!spatial) return <><ResultChart presentation={presentation} />{table}</>
  const current = shapes.views.some(view => view.id === chosen) ? chosen : shapes.first
  return (
    <div className="result-views">
      <div className="result-views__tabs" role="tablist" aria-label="How to see this result">
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
}
