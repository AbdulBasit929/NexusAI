import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useAuth } from '../context/AuthContext'
import { getShellRouteContext } from '../utils/shellContext'
import { useActiveCase } from '../contexts/ActiveCaseContext'

export default function AppShellHeader({ pathname }) {
  const navigate = useNavigate()
  const { t } = useTranslation('nav')
  const { authEnabled, user, isAdmin } = useAuth()
  const { activeCase } = useActiveCase()
  const context = getShellRouteContext(pathname, activeCase)
  const hasIdentity = authEnabled && user
  const role = hasIdentity
    ? t(isAdmin ? 'roles.systemAdministrator' : 'roles.analyst')
    : t('roles.localWorkspace')
  const identity = user?.name || user?.email || ''

  return (
    <header className="app-shell-header" role="region" aria-label={t('applicationContext')}>
      <div className="app-shell-header__route">
        <span className="app-shell-header__area">{context.area}</span>
        <strong>{context.title}</strong>
      </div>

      {context.caseId && (
        <div className="app-shell-header__case" aria-label={`${t('caseContext')}: ${context.caseId}`}>
          <i className="fas fa-folder-closed" aria-hidden="true" />
          <span>{t('caseContext')}</span>
          <code dir="auto" title={context.caseId}>{context.caseId}</code>
        </div>
      )}

      <div className="app-shell-header__identity">
        <span className="app-shell-header__identity-mark" aria-hidden="true" />
        <span className="app-shell-header__identity-copy">
          <strong>{role}</strong>
          {identity && <small>{identity}</small>}
        </span>
        {hasIdentity && (
          <button
            type="button"
            className="app-shell-header__account"
            onClick={() => navigate('/app/account')}
            aria-label={t('accountFor', { name: identity })}
          >
            {user.avatarUrl ? (
              <img src={user.avatarUrl} alt="" />
            ) : (
              <i className="fas fa-user-circle" aria-hidden="true" />
            )}
          </button>
        )}
      </div>
    </header>
  )
}
