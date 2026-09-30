import { useId } from 'react'

// The one surface primitive. Every card in the product is a labelled region with the same anatomy:
// a title that names the question or subject, an optional description, optional actions, a body, and an
// optional footer for source and coverage notes. `tone` only changes the border; it never carries meaning
// alone, because the title and body always state it in words.
export function Card({ title, description = null, actions = null, footer = null, tone = 'default', level = 2, as: Element = 'section', className = '', children }) {
  const headingId = useId()
  const Heading = `h${level}`
  return (
    <Element className={`card card--${tone}${className ? ` ${className}` : ''}`} aria-labelledby={title ? headingId : undefined}>
      {title || actions ? (
        <header className="card__header">
          <div>
            {title ? <Heading id={headingId} className="card__title">{title}</Heading> : null}
            {description ? <p className="card__description">{description}</p> : null}
          </div>
          {actions ? <div className="card__actions">{actions}</div> : null}
        </header>
      ) : null}
      <div className="card__body">{children}</div>
      {footer ? <footer className="card__footer">{footer}</footer> : null}
    </Element>
  )
}
