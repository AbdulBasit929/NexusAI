import { useBranding } from '../contexts/BrandingContext'
import { apiUrl } from '../utils/basePath'

export const NEXUSAI_PRIMARY_TAGLINE = 'Private AI infrastructure for your applications.'
export const NEXUSAI_CONTEXT_LINE = 'Enterprise Forensic Intelligence'

function isNexusAI(name) {
  return name.trim().toLowerCase() === 'nexusai'
}

function initialFor(name) {
  return name.trim().charAt(0).toUpperCase() || 'N'
}

export function BrandLockup({
  instanceName = 'NexusAI',
  instanceTagline = NEXUSAI_PRIMARY_TAGLINE,
  logoUrl = '',
  logoHorizontalUrl = '',
  variant = 'lockup',
  showContext = false,
  className = '',
}) {
  const nexusIdentity = isNexusAI(instanceName)
  const assetUrl = variant === 'mark' ? logoUrl : (logoHorizontalUrl || logoUrl)
  const useConfiguredAsset = !nexusIdentity && assetUrl
  const classes = ['nexus-brand', `nexus-brand--${variant}`, className].filter(Boolean).join(' ')
  const resolvedTagline = nexusIdentity
    ? (instanceTagline || NEXUSAI_PRIMARY_TAGLINE)
    : instanceTagline

  if (variant === 'mark') {
    return useConfiguredAsset ? (
      <img src={apiUrl(assetUrl)} alt="" className={`${classes} nexus-brand__asset`} />
    ) : (
      <span className={classes} aria-hidden="true">{initialFor(instanceName)}</span>
    )
  }

  if (variant === 'signature') {
    return (
      <div className={classes}>
        <div className="nexus-brand__identity">
          {useConfiguredAsset ? (
            <img src={apiUrl(assetUrl)} alt="" className="nexus-brand__asset" />
          ) : (
            <span className="nexus-brand__mark" aria-hidden="true">{initialFor(instanceName)}</span>
          )}
          <h1 className="nexus-brand__name">{instanceName}</h1>
        </div>
        {showContext && nexusIdentity && (
          <p className="nexus-brand__context">{NEXUSAI_CONTEXT_LINE}</p>
        )}
        {resolvedTagline && (
          <p className="nexus-brand__tagline">{resolvedTagline}</p>
        )}
      </div>
    )
  }

  return useConfiguredAsset ? (
    <img src={apiUrl(assetUrl)} alt={instanceName} className={`${classes} nexus-brand__asset`} />
  ) : (
    <span className={classes} aria-label={instanceName}>
      <span className="nexus-brand__mark" aria-hidden="true">{initialFor(instanceName)}</span>
      <strong className="nexus-brand__name">{instanceName}</strong>
    </span>
  )
}

export default function NexusBrand(props) {
  const branding = useBranding()
  return <BrandLockup {...branding} {...props} />
}
