// The NexusAI mark: an N drawn as one continuous stroke on a dark tile. Code-native, so it follows the theme (decision O-4 is still open for the owner).
export function BrandMark() {
  return (
    <span className="nx-brand-mark" aria-hidden="true">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round"><path d="M6 19V5l12 14V5" /></svg>
    </span>
  )
}
