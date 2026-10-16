import { useEffect, useState } from 'react'

// A number that changes when the colours of the page change: the theme chosen in the appearance menu (the data-theme attribute) or, with no choice made, the system's
// light or dark setting. A canvas (the link chart, the map) paints with colours it read from the tokens when it was drawn, so it redraws when this number changes.
export function useThemeVersion() {
  const [version, setVersion] = useState(0)
  useEffect(() => {
    const root = globalThis.document?.documentElement
    if (!root) return undefined
    const bump = () => setVersion(value => value + 1)
    const observer = typeof MutationObserver === 'undefined' ? null : new MutationObserver(bump)
    observer?.observe(root, { attributes: true, attributeFilter: ['data-theme'] })
    const media = typeof globalThis.matchMedia === 'function' ? globalThis.matchMedia('(prefers-color-scheme: dark)') : null
    media?.addEventListener?.('change', bump)
    return () => { observer?.disconnect(); media?.removeEventListener?.('change', bump) }
  }, [])
  return version
}
