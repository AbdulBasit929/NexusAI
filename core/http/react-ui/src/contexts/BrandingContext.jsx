import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { brandingApi } from '../utils/api'
import { investigationWorkspaceIdentity } from '../analyst/investigationWorkspace'

// Bundled defaults — used when the backend hasn't applied an override (or
// when /api/branding is briefly unreachable on first load).
const DEFAULT_BRANDING = {
  instanceName: 'NexusAI',
  instanceTagline: 'Private AI infrastructure for your applications.',
  logoUrl: '',
  logoHorizontalUrl: '',
  faviconUrl: '/favicon.svg',
}

const BrandingContext = createContext(null)

// Reads /api/branding (public — works pre-auth so the login screen renders
// the configured branding) and exposes the resolved values plus a refresh()
// callback used by the Settings page after save/upload.
export function BrandingProvider({ children }) {
  const [branding, setBranding] = useState(DEFAULT_BRANDING)
  const [loaded, setLoaded] = useState(false)

  const refresh = useCallback(async () => {
    try {
      const data = await brandingApi.get()
      const instanceName = data?.instance_name || DEFAULT_BRANDING.instanceName
      const isNexusAI = instanceName.trim().toLowerCase() === 'nexusai'
      setBranding({
        instanceName,
        instanceTagline: data?.instance_tagline || (isNexusAI ? DEFAULT_BRANDING.instanceTagline : ''),
        logoUrl: data?.logo_url || DEFAULT_BRANDING.logoUrl,
        logoHorizontalUrl: data?.logo_horizontal_url || DEFAULT_BRANDING.logoHorizontalUrl,
        faviconUrl: data?.favicon_url || DEFAULT_BRANDING.faviconUrl,
      })
    } catch (_e) {
      // /api/branding should always succeed (it's public and zero-side-effect).
      // If it doesn't, fall through to defaults so the UI still renders.
    } finally {
      setLoaded(true)
    }
  }, [])

  useEffect(() => { refresh() }, [refresh])

  // Drive document.title and the favicon link from branding state. Bust the
  // favicon cache by appending a query so changes show up without forcing a
  // hard reload — most browsers respect the URL change.
  useEffect(() => {
    if (!loaded) return
    const surfaceName = window.location.pathname.startsWith('/analyst')
      ? investigationWorkspaceIdentity.name
      : branding.instanceName
    document.title = surfaceName
    const applicationName = document.querySelector("meta[name='application-name']")
    if (applicationName) applicationName.content = surfaceName
    const description = document.querySelector("meta[name='description']")
    if (description && branding.instanceTagline) description.content = branding.instanceTagline
    const link = document.querySelector("link[rel='icon']") || document.querySelector("link[rel='shortcut icon']")
    if (link) {
      const href = branding.faviconUrl
      link.href = href.includes('?') ? href : `${href}?v=${Date.now()}`
    }
  }, [branding, loaded])

  return (
    <BrandingContext.Provider value={{ ...branding, loaded, refresh }}>
      {children}
    </BrandingContext.Provider>
  )
}

export function useBranding() {
  const ctx = useContext(BrandingContext)
  if (!ctx) throw new Error('useBranding must be used within a BrandingProvider')
  return ctx
}
