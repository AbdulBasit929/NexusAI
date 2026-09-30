import { useMemo, useState } from 'react'
import { ChartColumn, Table2 } from 'lucide-react'
import { Card } from '../../components/Card.jsx'
import { SkeletonRows } from '../../components/Skeleton.jsx'
import { packCircles } from '../../lib/bubblePack.js'
import { formatNumber } from '../../lib/format.js'

const WIDTH = 560
const HEIGHT = 300
const SHOWN = 12

function percent(share) {
  if (!share) return '0%'
  if (share < 0.01) return '<1%'
  return `${Math.floor(share * 100)}%`
}

// D5. What is the evidence made of? Record families as packed bubbles: area follows accepted rows, so the case's
// shape reads at a glance; the exact counts and shares are on the legend and in the table, which are also the
// keyboard path. Each family keeps one colour on every widget. A click selects a family, which filters the case
// queue below; selecting it again clears. Bubbles are buttons positioned in percent, so type stays a readable size.
// Rules: UI_REDESIGN_BRIEF §11.
export function EvidenceMap({ families, order, selectedId, onSelect, kpis, loading }) {
  const [view, setView] = useState('map')
  const shown = families.slice(0, SHOWN)
  const packed = useMemo(() => packCircles(shown.map(family => ({ id: family.id, value: family.value })), WIDTH, HEIGHT), [shown])
  const byId = new Map(shown.map(family => [family.id, family]))
  const colour = id => `var(--analyst-data-${(Math.max(0, order.indexOf(id)) % 6) + 1})`
  const select = id => onSelect(selectedId === id ? '' : id)

  let body
  if (loading && !families.length) body = <SkeletonRows rows={3} label="Reading record families" />
  else if (!families.length) body = <p className="dash-card__empty">No structured records have been accepted yet.</p>
  else if (view === 'table') {
    body = (
      <div className="dash-table">
        <table>
          <thead><tr><th scope="col">Record family</th><th scope="col" className="is-numeric">Accepted rows</th><th scope="col" className="is-numeric">Share</th><th scope="col" className="is-numeric">Cases</th><th scope="col">Filter</th></tr></thead>
          <tbody>{families.map(family => (
            <tr key={family.id}>
              <th scope="row">{family.label}</th>
              <td className="is-numeric">{formatNumber(family.value)}</td>
              <td className="is-numeric">{percent(family.share)}</td>
              <td className="is-numeric">{formatNumber(family.caseCount)}</td>
              <td><button type="button" className="dash-link-button" aria-label={`Show only cases with ${family.label}`} aria-pressed={selectedId === family.id} onClick={() => select(family.id)}>Show cases</button></td>
            </tr>
          ))}</tbody>
        </table>
      </div>
    )
  } else {
    body = (
      <>
        <div className="em-map" role="group" aria-label="Record families sized by accepted rows">
          {packed.map((circle, index) => {
            const family = byId.get(circle.id)
            const selected = selectedId === circle.id
            const big = circle.r >= 46
            return (
              <button
                key={circle.id}
                type="button"
                className={`em-bubble${selected ? ' is-selected' : ''}${selectedId && !selected ? ' is-dim' : ''}`}
                style={{ left: `${(circle.x / WIDTH) * 100}%`, top: `${(circle.y / HEIGHT) * 100}%`, inlineSize: `${((circle.r * 2) / WIDTH) * 100}%`, background: colour(circle.id), animationDelay: `${index * 45}ms` }}
                aria-pressed={selected}
                aria-label={`${family.label}: ${formatNumber(family.value)} accepted rows, ${percent(family.share)}. Show only cases with this.`}
                title={`${family.label}: ${formatNumber(family.value)}`}
                onClick={() => select(circle.id)}
              >
                {big ? <span aria-hidden="true"><b>{family.label}</b><i>{formatNumber(family.value)}</i></span> : null}
              </button>
            )
          })}
        </div>
        <ul className="em-legend" aria-label="Record families">
          {shown.map(family => (
            <li key={family.id}>
              <button type="button" className={selectedId === family.id ? 'is-selected' : undefined} aria-pressed={selectedId === family.id} onClick={() => select(family.id)}>
                <span className="em-legend__swatch" style={{ background: colour(family.id) }} aria-hidden="true" />
                <span className="em-legend__label">{family.label}</span>
                <span className="em-legend__value">{formatNumber(family.value)}</span>
                <span className="em-legend__share">{percent(family.share)}</span>
              </button>
            </li>
          ))}
        </ul>
        {families.length > SHOWN ? <p className="dash-card__footnote">{formatNumber(families.length - SHOWN)} smaller families are in the table.</p> : null}
      </>
    )
  }

  return (
    <div id="dashboard-families" className="dash-slot">
      <Card
        className="evidence-map"
        title="What is the evidence made of?"
        actions={families.length ? (
          <div className="chart-card__toggle" role="group" aria-label="View of: What is the evidence made of?">
            <button type="button" aria-pressed={view === 'map'} onClick={() => setView('map')}><ChartColumn aria-hidden="true" />Map</button>
            <button type="button" aria-pressed={view === 'table'} onClick={() => setView('table')}><Table2 aria-hidden="true" />Table</button>
          </div>
        ) : null}
        footer={families.length ? <span>{formatNumber(kpis.acceptedRows)} accepted rows. Structured records only, not every file.</span> : null}
      >
        {body}
      </Card>
    </div>
  )
}
