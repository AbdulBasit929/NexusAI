import { Link } from 'react-router-dom'
import { LanguageText } from './AnalystComponents.jsx'

// ONE page header for every route.
//
// Anatomy (Primer PageHeader, Atlassian page header): breadcrumb trail, then the title with a short
// description on the left and the page's actions on the right, then an optional strip of facts.
//
// The title says what the page is, so there is no eyebrow label repeating it. `eyebrow` is still accepted
// so existing pages keep compiling, but it is deliberately not rendered.
//
// `breadcrumbs` is only the trail BELOW the case bar: the shell already shows Cases and the case, so a case
// page passes just its deeper trail (for example Evidence, then Source). `meta` is scannable key/value
// context, and `actions` is the right-aligned action cluster. All are optional.
export function PageHeader({ title, description, breadcrumbs = [], meta = [], actions = null, identifierTitle = false }) {
  const facts = meta.filter(Boolean)
  return (
    <header className="page-header">
      {breadcrumbs.length > 0 && (
        <nav className="page-header__breadcrumbs" aria-label="Page trail">
          <ol>
            {breadcrumbs.map((crumb, index) => (
              <li key={`${crumb.label}-${index}`}>
                {crumb.to && index < breadcrumbs.length - 1
                  ? <Link to={crumb.to}>{crumb.label}</Link>
                  : <span aria-current={index === breadcrumbs.length - 1 ? 'page' : undefined}>{crumb.label}</span>}
              </li>
            ))}
          </ol>
        </nav>
      )}
      <div className="page-header__band">
        <div className="page-header__identity">
          <h1>{identifierTitle ? <LanguageText as="bdi" identifier>{title}</LanguageText> : title}</h1>
          {description && <p className="page-header__description">{description}</p>}
        </div>
        {actions && <div className="page-header__actions">{actions}</div>}
      </div>
      {facts.length > 0 && (
        <dl className="page-header__meta">
          {facts.map((item, index) => (
            <div key={`${item.label}-${index}`} className={item.stale ? 'is-stale' : undefined}>
              <dt>{item.label}</dt>
              <dd>{item.identifier ? <LanguageText as="bdi" identifier>{item.value}</LanguageText> : item.value}</dd>
            </div>
          ))}
        </dl>
      )}
    </header>
  )
}
