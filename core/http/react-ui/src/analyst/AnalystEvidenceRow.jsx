import { evidenceFindingPreview, evidenceTypePresentation } from './analystDataPresentation'
import { friendlySourceName } from './analystPresentation'

function formatBytes(value) {
  const bytes = Number(value) || 0
  if (!bytes) return 'Size not reported'
  const units = ['B', 'KB', 'MB', 'GB']
  const unit = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / (1024 ** unit)).toLocaleString(undefined, { maximumFractionDigits: 1 })} ${units[unit]}`
}

export default function AnalystEvidenceRow({ item, status, capability, onOpen }) {
  const type = evidenceTypePresentation(item)
  const added = item.created_at ? new Date(item.created_at).toLocaleDateString(undefined, { dateStyle: 'medium' }) : 'Date not reported'
  return (
    <button className="analyst-evidence-row" data-kind={type.category} type="button" onClick={onOpen}>
      <span className="analyst-evidence-row__icon" aria-hidden="true"><i className={`fas ${type.icon}`} /></span>
      <span className="analyst-evidence-row__body">
        <span className="analyst-evidence-row__kind">{type.label}</span>
        <strong>{friendlySourceName(item)}</strong>
        <span className="analyst-evidence-row__preview">{evidenceFindingPreview(item, status, capability)}</span>
        <span className="analyst-evidence-row__meta">{formatBytes(item.size_bytes)}<span aria-hidden="true">·</span>{added}</span>
      </span>
      <span className="analyst-evidence-row__state">
        <span className="analyst-simple-status" data-tone={status.tone}>{status.label}</span>
        <i className="fas fa-chevron-right" aria-hidden="true" />
      </span>
    </button>
  )
}

