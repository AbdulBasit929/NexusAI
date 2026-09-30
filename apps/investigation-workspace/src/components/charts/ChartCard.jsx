import { lazy, Suspense, useId, useState } from 'react'
import { ChartColumn, Table2 } from 'lucide-react'

// Code-split: ECharts is fetched only when a chart is actually shown.
const EChart = lazy(() => import('./EChart.jsx'))

// One frame for every chart. The title is the question the chart answers. The Table view holds the exact
// values and the real links behind them, so the chart is never the only way to reach a record.
export function ChartCard({ title, description, chart, columns, rows, footer = null, tableFirst = false }) {
  const headingId = useId()
  const [view, setView] = useState(tableFirst ? 'table' : 'chart')
  return (
    <section className="chart-card" aria-labelledby={headingId}>
      <header className="chart-card__header">
        <div><h2 id={headingId}>{title}</h2>{description ? <p>{description}</p> : null}</div>
        <div className="chart-card__toggle" role="group" aria-label={`View of: ${title}`}>
          <button type="button" aria-pressed={view === 'chart'} onClick={() => setView('chart')}><ChartColumn aria-hidden="true" />Chart</button>
          <button type="button" aria-pressed={view === 'table'} onClick={() => setView('table')}><Table2 aria-hidden="true" />Table</button>
        </div>
      </header>
      {view === 'chart' ? (
        <div className="chart-card__plot">
          <Suspense fallback={<p className="chart-card__loading" role="status">Loading chart…</p>}>
            <EChart buildOption={chart.buildOption} height={chart.height} label={chart.label} onSelect={chart.onSelect} />
          </Suspense>
          {chart.legend ? <ul className="chart-card__legend" aria-label="Legend">{chart.legend.map(item => <li key={item.label}><span className={`chart-card__swatch chart-card__swatch--${item.tone}`} aria-hidden="true" />{item.label}</li>)}</ul> : null}
        </div>
      ) : (
        <div className="chart-card__table">
          <table>
            <thead><tr>{columns.map(column => <th key={column.key} scope="col" className={column.numeric ? 'is-numeric' : undefined}>{column.label}</th>)}</tr></thead>
            <tbody>{rows.map(row => <tr key={row.key}>{columns.map((column, index) => {
              const Cell = index === 0 ? 'th' : 'td'
              return <Cell key={column.key} scope={index === 0 ? 'row' : undefined} className={column.numeric ? 'is-numeric' : undefined}>{column.render ? column.render(row) : row[column.key]}</Cell>
            })}</tr>)}</tbody>
          </table>
        </div>
      )}
      {footer ? <footer className="chart-card__footer">{footer}</footer> : null}
    </section>
  )
}
