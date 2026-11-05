import { TableTwin } from './TableTwin.jsx'

// The card every dashboard block sits in: one title, an optional short note on the right, an optional table button, one body. A panel is a region with a name, so a screen reader can jump between
// them. `twin` ({ label, columns, rows, exportName, summary }) puts a "show as table" button in the header; the table opens in a dialog, so the panel keeps its size.
export function Panel({ id, title, sub, subTitle, action, twin, className = '', flush = false, children }) {
  return (
    <section className={`nx-panel${className ? ` ${className}` : ''}`} aria-labelledby={id}>
      <header className="nx-panel__head">
        <h2 id={id} className="nx-panel__title">{title}</h2>
        {sub ? <span className="nx-panel__sub" title={subTitle}>{sub}</span> : null}
        {action}
        {twin ? <TableTwin {...twin} /> : null}
      </header>
      <div className={`nx-panel__body${flush ? ' nx-panel__body--flush' : ''}`}>{children}</div>
    </section>
  )
}

// What a panel shows while its figures are still being read: the shape of the content, not a spinner.
export function PanelSkeleton({ rows = 3 }) {
  return <div className="nx-skeleton-list" aria-hidden="true">{Array.from({ length: rows }, (_, index) => <i key={index} className="nx-skeleton" />)}</div>
}

// "Nothing here" in one line, with the mark that says whether that is good news (`ok`) or just empty.
export function Calm({ icon: Icon, tone = 'plain', children }) {
  return <div className={`nx-calm${tone === 'plain' ? ' nx-calm--plain' : ''}`}>{Icon ? <Icon aria-hidden="true" /> : null}<p>{children}</p></div>
}
