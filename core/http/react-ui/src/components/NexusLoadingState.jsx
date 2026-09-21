import LoadingSpinner from './LoadingSpinner'
import NexusBrand, { BrandLockup } from './NexusBrand'

export default function NexusLoadingState({
  label = 'Loading workspace…',
  staticBrand = false,
  brandProps = undefined,
  fullScreen = false,
  className = '',
}) {
  const classes = [
    'nexus-loading-state',
    fullScreen ? 'nexus-loading-state--fullscreen' : '',
    className,
  ].filter(Boolean).join(' ')

  return (
    <div
      className={classes}
      role="status"
      aria-live="polite"
    >
      {staticBrand ? <BrandLockup {...brandProps} /> : <NexusBrand />}
      <span className="nexus-loading-state__progress">
        <LoadingSpinner size="lg" />
        <span>{label}</span>
      </span>
    </div>
  )
}
