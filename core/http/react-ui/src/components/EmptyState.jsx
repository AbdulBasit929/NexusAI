const STATE_ICONS = {
  empty: 'fa-box-open',
  error: 'fa-circle-exclamation',
  forbidden: 'fa-shield-halved',
  partial: 'fa-circle-half-stroke',
  unavailable: 'fa-ban',
}

// Shared truthful route state. Legacy callers can keep passing icon/title and
// children; newer callers add a state and details without duplicating markup.
export default function EmptyState({
  icon,
  eyebrow,
  title,
  body,
  children,
  details,
  actions,
  state = 'empty',
  headingLevel = 2,
  className = '',
}) {
  const resolvedIcon = icon || STATE_ICONS[state] || STATE_ICONS.empty
  const resolvedBody = body ?? children
  const iconClass = resolvedIcon.includes(' ') ? resolvedIcon : `fas ${resolvedIcon}`
  const isError = state === 'error'

  return (
    <div
      className={`empty-state empty-state--${state} ${className}`.trim()}
      data-state={state}
      {...(isError ? { role: 'alert', 'aria-live': 'assertive' } : {})}
    >
      {eyebrow && <span className="empty-state__eyebrow">{eyebrow}</span>}
      <i className={`empty-state-icon ${iconClass}`} aria-hidden="true" />
      {title && (headingLevel === 1
        ? <h1 className="empty-state-title">{title}</h1>
        : <h2 className="empty-state-title">{title}</h2>)}
      {resolvedBody && <p className="empty-state-text">{resolvedBody}</p>}
      {details && <div className="empty-state__details">{details}</div>}
      {actions && <div className="empty-state__actions">{actions}</div>}
    </div>
  )
}
