// Placeholders that hold the layout while a request is in flight (Atlassian, NN/G: use for waits of about
// 2 to 10 seconds, match the size of the real content, and disappear the moment it arrives). They carry no
// numbers and no progress: a wait longer than that gets the honest processing state, never a fake bar.
// The shapes are hidden from assistive technology; one status message announces the wait.
export function Skeleton({ lines = 3, label = 'Loading', className = '' }) {
  return (
    <div className={`skeleton${className ? ` ${className}` : ''}`} role="status" aria-busy="true">
      <span className="visually-hidden">{label}</span>
      <div aria-hidden="true">
        {Array.from({ length: lines }, (_, index) => <span key={index} className="skeleton__line" style={{ '--skeleton-width': `${index === lines - 1 ? 60 : 100 - (index % 2) * 12}%` }} />)}
      </div>
    </div>
  )
}

// List-shaped placeholder: rows with a mark and two lines, for queues and lists.
export function SkeletonRows({ rows = 3, label = 'Loading', className = '' }) {
  return (
    <div className={`skeleton-rows${className ? ` ${className}` : ''}`} role="status" aria-busy="true">
      <span className="visually-hidden">{label}</span>
      <div aria-hidden="true">
        {Array.from({ length: rows }, (_, index) => <div key={index} className="skeleton-row"><span className="skeleton__mark" /><div><span className="skeleton__line" style={{ '--skeleton-width': '55%' }} /><span className="skeleton__line" style={{ '--skeleton-width': '80%' }} /></div></div>)}
      </div>
    </div>
  )
}

// Shown while a route's code loads, before that page can draw its own shell. It reproduces the chrome and
// the page rhythm, so the first real paint does not move anything.
export function ShellSkeleton({ label = 'Opening investigation' }) {
  return (
    <div className="shell-skeleton" role="status" aria-busy="true">
      <span className="visually-hidden">{label}</span>
      <div aria-hidden="true" className="shell-skeleton__frame">
        <div className="shell-skeleton__header" />
        <div className="shell-skeleton__rail" />
        <div className="shell-skeleton__content">
          <span className="skeleton__line skeleton__line--heading" />
          <span className="skeleton__line" style={{ '--skeleton-width': '46%' }} />
          <div className="skeleton-cards__grid">{[0, 1, 2].map(index => <div key={index} className="skeleton-card"><span className="skeleton__line skeleton__line--title" /><span className="skeleton__line" /><span className="skeleton__line" style={{ '--skeleton-width': '70%' }} /></div>)}</div>
        </div>
      </div>
    </div>
  )
}

export function SkeletonCards({ count = 3, label = 'Loading', className = '' }) {
  return (
    <div className={`skeleton-cards${className ? ` ${className}` : ''}`} role="status" aria-busy="true">
      <span className="visually-hidden">{label}</span>
      <div aria-hidden="true">
        {Array.from({ length: count }, (_, index) => <div key={index} className="skeleton-card"><span className="skeleton__line skeleton__line--title" /><span className="skeleton__line" /><span className="skeleton__line" style={{ '--skeleton-width': '70%' }} /></div>)}
      </div>
    </div>
  )
}
