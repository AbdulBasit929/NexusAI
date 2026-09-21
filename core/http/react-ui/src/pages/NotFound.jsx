import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import EmptyState from '../components/EmptyState'

export default function NotFound() {
  const navigate = useNavigate()
  const { t } = useTranslation('auth')

  return (
    <div className="page page--narrow">
      <EmptyState
        state="unavailable"
        icon="fa-compass"
        eyebrow="Navigation / 404"
        title={t('notFound.title')}
        headingLevel={1}
        body={t('notFound.text')}
        actions={<button className="btn btn-primary" onClick={() => navigate('/app')}>
          <i className="fas fa-home" /> {t('notFound.goHome')}
        </button>}
      />
    </div>
  )
}
