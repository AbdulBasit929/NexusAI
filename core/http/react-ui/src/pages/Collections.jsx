import { useState, useEffect, useCallback } from 'react'
import { useNavigate, useOutletContext } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { agentCollectionsApi } from '../utils/api'
import { useAuth } from '../context/AuthContext'
import { useUserMap } from '../hooks/useUserMap'
import UserGroupSection from '../components/UserGroupSection'
import PageHeader from '../components/PageHeader'
import ConfirmDialog from '../components/ConfirmDialog'
import EmptyState from '../components/EmptyState'
import NexusLoadingState from '../components/NexusLoadingState'
import { useActiveCase } from '../contexts/ActiveCaseContext'

function isRetainedSystemCollection(collection) {
  const name = String(typeof collection === 'string' ? collection : collection?.name || '')
  const lower = name.toLowerCase()
  return lower === 'forensic_records_analyst' ||
    lower === 'communications_cdr_analyst' ||
    lower === 'network_ipdr_capture_analyst' ||
    lower === 'vehicle_anpr_geospatial_analyst' ||
    lower === 'records-demo' ||
    lower.startsWith('nexusai-structured-demo') ||
    (lower.startsWith('forensic-') && /acceptance|validation|audit|golden|warning/.test(lower))
}

export default function Collections() {
  const { addToast } = useOutletContext()
  const navigate = useNavigate()
  const { t } = useTranslation('collections')
  const { isAdmin, authEnabled, user } = useAuth()
  const userMap = useUserMap()
  const { caseOptions, registryState, ensureCaseRegistry, setActiveCase } = useActiveCase()
  const [collections, setCollections] = useState([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [newName, setNewName] = useState('')
  const [creating, setCreating] = useState(false)
  const [userGroups, setUserGroups] = useState(null)
  const [confirmDialog, setConfirmDialog] = useState(null)
  const [showRetainedSystem, setShowRetainedSystem] = useState(false)

  const visibleCollections = showRetainedSystem
    ? collections
    : collections.filter(collection => !isRetainedSystemCollection(collection))
  const hiddenCollectionCount = collections.length - visibleCollections.length

  const fetchCollections = useCallback(async () => {
    setLoading(true)
    setLoadError('')
    try {
      const data = await agentCollectionsApi.list(isAdmin && authEnabled)
      setCollections(Array.isArray(data.collections) ? data.collections : [])
      setUserGroups(data.user_groups || null)
    } catch (err) {
      setLoadError(err.message || t('errors.unavailable'))
      addToast(t('toasts.loadFailed', { message: err.message }), 'error')
    } finally {
      setLoading(false)
    }
  }, [addToast, isAdmin, authEnabled, t])

  useEffect(() => {
    fetchCollections()
  }, [fetchCollections])

  useEffect(() => {
    if (registryState === 'idle') ensureCaseRegistry()
  }, [ensureCaseRegistry, registryState])

  const handleCreate = async () => {
    const name = newName.trim()
    if (!name) return
    setCreating(true)
    try {
      await agentCollectionsApi.create(name)
      addToast(t('toasts.created', { name }), 'success')
      setNewName('')
      fetchCollections()
    } catch (err) {
      addToast(t('toasts.createFailed', { message: err.message }), 'error')
    } finally {
      setCreating(false)
    }
  }

  const handleDelete = (name, userId) => {
    setConfirmDialog({
      title: t('deleteDialog.title'),
      message: t('deleteDialog.message', { name }),
      confirmLabel: t('deleteDialog.confirm'),
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null)
        try {
          await agentCollectionsApi.reset(name, userId)
          addToast(t('toasts.deleted', { name }), 'success')
          fetchCollections()
        } catch (err) {
          addToast(t('toasts.deleteFailed', { message: err.message }), 'error')
        }
      },
    })
  }

  const handleReset = (name, userId) => {
    setConfirmDialog({
      title: t('resetDialog.title'),
      message: t('resetDialog.message', { name }),
      confirmLabel: t('resetDialog.confirm'),
      danger: true,
      onConfirm: async () => {
        setConfirmDialog(null)
        try {
          await agentCollectionsApi.reset(name, userId)
          addToast(t('toasts.reset', { name }), 'success')
          fetchCollections()
        } catch (err) {
          addToast(t('toasts.resetFailed', { message: err.message }), 'error')
        }
      },
    })
  }

  return (
    <div className="page page--wide">
      <style>{`
        .collections-create-bar {
          display: flex;
          gap: var(--spacing-sm);
          margin-bottom: var(--spacing-lg);
        }
        .collections-create-bar .input {
          flex: 1;
        }
        .collections-grid {
          display: grid;
          grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
          gap: var(--spacing-md);
        }
        .collections-card-name {
          font-size: 1rem;
          font-weight: 600;
          margin-bottom: var(--spacing-sm);
          word-break: break-word;
        }
        .collections-card-actions {
          display: flex;
          gap: var(--spacing-xs);
          margin-top: var(--spacing-md);
        }
        .collections-scope-bar { display: flex; justify-content: space-between; align-items: center; gap: var(--spacing-sm); flex-wrap: wrap; margin-bottom: var(--spacing-md); padding: var(--spacing-sm) var(--spacing-md); border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-bg-secondary); }
        .collections-scope-bar strong { display: block; color: var(--color-text-primary); }
        .collections-scope-bar span { color: var(--color-text-secondary); font-size: .8125rem; }
      `}</style>

      <PageHeader title={t('title')} supporting="Administrative knowledge collections, sources, and lifecycle controls. Case analysis opens only through an authorized case mapping." />

      <div className="collections-scope-bar" role="note" aria-label="Knowledge administration boundary">
          <div>
            <strong>Knowledge administration</strong>
            <span>{visibleCollections.length} active collection{visibleCollections.length === 1 ? '' : 's'} · case identity is never inferred from a collection name{hiddenCollectionCount ? ` · ${hiddenCollectionCount} retained system collection${hiddenCollectionCount === 1 ? '' : 's'} hidden` : ''}</span>
          </div>
          {hiddenCollectionCount > 0 && (
          <button className="btn btn-secondary btn-sm" type="button" onClick={() => setShowRetainedSystem(value => !value)}>
            <i className={`fas ${showRetainedSystem ? 'fa-eye-slash' : 'fa-box-archive'}`} /> {showRetainedSystem ? 'Hide retained system collections' : 'Review retained system collections'}
          </button>
          )}
      </div>

      <div className="collections-create-bar">
        <input
          className="input"
          type="text"
          placeholder={t('newPlaceholder')}
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') handleCreate() }}
        />
        <button className="btn btn-primary" onClick={handleCreate} disabled={creating || !newName.trim()}>
          {creating ? <><i className="fas fa-spinner fa-spin" /> {t('actions.creating')}</> : <><i className="fas fa-plus" /> {t('actions.create')}</>}
        </button>
      </div>

      {loading ? (
        <NexusLoadingState label={t('states.loading')} />
      ) : loadError ? (
        <EmptyState
          state="error"
          eyebrow={t('states.eyebrow')}
          title={t('errors.title')}
          body={t('errors.body')}
          details={loadError}
          actions={(
            <button className="btn btn-primary" type="button" onClick={fetchCollections}>
              <i className="fas fa-rotate" aria-hidden="true" /> {t('actions.retry')}
            </button>
          )}
        />
      ) : visibleCollections.length === 0 && !userGroups ? (
        <EmptyState
          state="empty"
          eyebrow={t('states.eyebrow')}
          icon="fa-folder-open"
          title={t('empty.title')}
          body={t('empty.text')}
        />
      ) : (
        <>
        {userGroups && <h2 style={{ fontSize: '1.1rem', fontWeight: 600, marginBottom: 'var(--spacing-md)' }}>{t('sections.yourCollections')}</h2>}
        {visibleCollections.length === 0 ? (
          <p style={{ color: 'var(--color-text-secondary)', marginBottom: 'var(--spacing-md)' }}>{t('empty.noPersonal')}</p>
        ) : (
        <div className="collections-grid">
          {visibleCollections.map((collection) => {
            const name = typeof collection === 'string' ? collection : collection.name
            const authorizedCase = caseOptions.find(item => item.collectionId === name)
            return (
              <div className="card" key={name} style={{ cursor: 'pointer' }} onClick={() => navigate(`/app/collections/${encodeURIComponent(name)}`)}>
                <div className="collections-card-name">
                  <i className="fas fa-folder" style={{ marginRight: 'var(--spacing-xs)', color: 'var(--color-primary)' }} />
                  {name}
                </div>
                <div className="collections-card-actions" onClick={(e) => e.stopPropagation()}>
                  <button className="btn btn-secondary btn-sm" onClick={() => navigate(`/app/collections/${encodeURIComponent(name)}`)} title={t('actions.viewDetails')}>
                    <i className="fas fa-eye" /> {t('actions.details')}
                  </button>
                  <button className="btn btn-secondary btn-sm" onClick={() => handleReset(name)} title={t('actions.resetCollection')}>
                    <i className="fas fa-rotate" /> {t('actions.reset')}
                  </button>
                  {authorizedCase && (
                    <button className="btn btn-primary btn-sm" type="button" onClick={() => setActiveCase(authorizedCase.caseId, 'overview')} aria-label={`Open authorized case ${authorizedCase.displayName}`}>
                      <i className="fas fa-shield-halved" /> Open case
                    </button>
                  )}
                  <button className="btn btn-danger btn-sm" onClick={() => handleDelete(name)} title={t('actions.deleteCollection')}>
                    <i className="fas fa-trash" />
                  </button>
                </div>
              </div>
            )
          })}
        </div>
        )}
        </>
      )}

      {userGroups && (
        <UserGroupSection
          title={t('sections.otherUsersCollections')}
          userGroups={userGroups}
          userMap={userMap}
          currentUserId={user?.id}
          itemKey="collections"
          renderGroup={(items, userId) => (
            <div className="collections-grid">
              {(items || []).map((col) => {
                const name = typeof col === 'string' ? col : col.name
                return (
                  <div className="card" key={name}>
                    <div className="collections-card-name">
                      <i className="fas fa-folder" style={{ marginRight: 'var(--spacing-xs)', color: 'var(--color-primary)' }} />
                      {name}
                    </div>
                    <div className="collections-card-actions">
                      <button className="btn btn-secondary btn-sm" onClick={() => navigate(`/app/collections/${encodeURIComponent(name)}?user_id=${encodeURIComponent(userId)}`)} title={t('actions.viewDetails')}>
                        <i className="fas fa-eye" /> {t('actions.details')}
                      </button>
                      <button className="btn btn-secondary btn-sm" onClick={() => handleReset(name, userId)} title={t('actions.resetCollection')}>
                        <i className="fas fa-rotate" /> {t('actions.reset')}
                      </button>
                      <button className="btn btn-danger btn-sm" onClick={() => handleDelete(name, userId)} title={t('actions.deleteCollection')}>
                        <i className="fas fa-trash" />
                      </button>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        />
      )}

      <ConfirmDialog
        open={!!confirmDialog}
        title={confirmDialog?.title}
        message={confirmDialog?.message}
        confirmLabel={confirmDialog?.confirmLabel}
        danger={confirmDialog?.danger}
        onConfirm={confirmDialog?.onConfirm}
        onCancel={() => setConfirmDialog(null)}
      />
    </div>
  )
}
