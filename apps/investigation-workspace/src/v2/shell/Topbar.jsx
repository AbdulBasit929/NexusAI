import { Link, NavLink } from 'react-router-dom'
import { Bell, Check, Contrast, Menu as MenuIcon, Monitor, Moon, Rows3, Search, Sun, SunMoon } from 'lucide-react'
import { useThemePreference } from '../../components/ThemeControl.jsx'
import { formatNumber } from '../../lib/format.js'
import { useDensity, useMotion } from './appearance.js'
import { CaseLabel } from './CaseLabel.jsx'
import { Menu } from './Menu.jsx'
import { trailFor } from './nav.js'

function Breadcrumbs({ pathname }) {
  const trail = trailFor(pathname)
  return (
    <nav className="nx-crumbs" aria-label="Breadcrumb">
      <ol>
        {trail.map((item, index) => {
          const last = index === trail.length - 1
          const text = item.caseId ? <span className="nx-crumbs__case"><CaseLabel caseId={item.caseId} /></span> : item.label
          return (
            <li key={`${item.label || item.caseId}-${index}`}>
              {last || !item.to ? <span aria-current={last ? 'page' : undefined} className={item.caseId ? 'nx-crumbs__case' : undefined}>{item.caseId ? <CaseLabel caseId={item.caseId} /> : item.label}</span> : <Link to={item.to}>{text}</Link>}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}

function ActivityMenu({ attention }) {
  const total = attention.needReview + attention.processing
  return (
    <Menu label={total ? `Activity: ${total} need attention or are running` : 'Activity: nothing needs you'} icon={
      <span style={{ position: 'relative', display: 'inline-grid' }}>
        <Bell aria-hidden="true" />
        {total ? <span className="nx-dot nx-dot--accent" style={{ position: 'absolute', insetBlockStart: -2, insetInlineEnd: -2, outline: '2px solid var(--nx-bg)' }} aria-hidden="true" /> : null}
      </span>
    }>
      {close => (
        <>
          <div className="nx-menu__label">Activity</div>
          {attention.needReview ? <NavLink className="nx-menu__item" to="/cases" onClick={close}><span className="nx-dot nx-dot--danger" aria-hidden="true" />{formatNumber(attention.needReview)} {attention.needReview === 1 ? 'case needs' : 'cases need'} review</NavLink> : null}
          {attention.processing ? <NavLink className="nx-menu__item" to="/cases" onClick={close}><span className="nx-dot nx-dot--busy" aria-hidden="true" />{formatNumber(attention.processing)} {attention.processing === 1 ? 'case is' : 'cases are'} processing evidence</NavLink> : null}
          {!total ? <p style={{ padding: 'var(--nx-s-2) var(--nx-s-3)', color: 'var(--nx-text-2)', fontSize: 'var(--nx-fs-sm)' }}>Nothing needs you right now.</p> : null}
          <hr className="nx-menu__rule" />
          <NavLink className="nx-menu__item" to="/activity" onClick={close}>Open the activity log</NavLink>
        </>
      )}
    </Menu>
  )
}

const themeChoices = [
  { value: 'system', label: 'Match this device', icon: Monitor },
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
  { value: 'contrast', label: 'High contrast', icon: Contrast },
]
const densityChoices = [
  { value: 'compact', label: 'Compact' },
  { value: 'default', label: 'Default' },
  { value: 'comfortable', label: 'Comfortable' },
]

function AppearanceMenu() {
  const { preference, choose } = useThemePreference()
  const [density, setDensity] = useDensity()
  const [motion, setMotion] = useMotion()
  return (
    <Menu label="Appearance" icon={<SunMoon aria-hidden="true" />}>
      {() => (
        <>
          <div className="nx-menu__label">Theme</div>
          {themeChoices.map(({ value, label, icon: Icon }) => (
            <button key={value} type="button" role="menuitemradio" aria-checked={preference === value} className="nx-menu__item" onClick={() => choose(value)}>
              <Icon aria-hidden="true" />{label}{preference === value ? <Check className="nx-menu__check" aria-hidden="true" /> : null}
            </button>
          ))}
          <hr className="nx-menu__rule" />
          <div className="nx-menu__label">Density</div>
          {densityChoices.map(({ value, label }) => (
            <button key={value} type="button" role="menuitemradio" aria-checked={density === value} className="nx-menu__item" onClick={() => setDensity(value)}>
              <Rows3 aria-hidden="true" />{label}{density === value ? <Check className="nx-menu__check" aria-hidden="true" /> : null}
            </button>
          ))}
          <hr className="nx-menu__rule" />
          <button type="button" role="menuitemcheckbox" aria-checked={motion === 'reduce'} className="nx-menu__item" onClick={() => setMotion(motion === 'reduce' ? 'system' : 'reduce')}>
            Reduce motion{motion === 'reduce' ? <Check className="nx-menu__check" aria-hidden="true" /> : null}
          </button>
        </>
      )}
    </Menu>
  )
}

export function Topbar({ pathname, attention, onOpenNav, onOpenPalette }) {
  return (
    <header className="nx-ui nx-topbar">
      <button type="button" className="nx-btn nx-btn--ghost nx-btn--icon nx-nav-toggle" aria-label="Open the navigation" onClick={onOpenNav}><MenuIcon aria-hidden="true" /></button>
      <Breadcrumbs pathname={pathname} />
      <span className="nx-topbar__spacer" />
      <button type="button" className="nx-search-trigger" aria-label="Search cases, evidence and questions" aria-haspopup="dialog" onClick={onOpenPalette}>
        <Search aria-hidden="true" /><span>Search or jump to…</span><kbd className="nx-kbd" aria-hidden="true">Ctrl K</kbd>
      </button>
      <ActivityMenu attention={attention} />
      <AppearanceMenu />
    </header>
  )
}
