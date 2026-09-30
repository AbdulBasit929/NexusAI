import { useEffect } from 'react'

function config() { return globalThis.window?.__INVESTIGATION_WORKSPACE_CONFIG__ || {} }

export function BrandMark() {
  const custom = config().logo_url
  if (custom) return <img className="brand-mark" src={custom} alt="" />
  return <svg className="brand-mark" viewBox="0 0 32 32" aria-hidden="true" focusable="false"><path d="M7 24V8l18 16V8" /><path d="M7 8h6l12 11" /></svg>
}

export function BrandLockup({ identityLed = false }) {
  const runtime = config()
  const name = runtime.instance_name || 'NexusAI'
  const tagline = runtime.instance_tagline || 'Private AI infrastructure for your applications.'
  const horizontal = runtime.logo_horizontal_url
  useEffect(() => {
    globalThis.document.title = name
    const favicon = runtime.favicon_url || '/favicon.svg'
    let link = globalThis.document.querySelector("link[rel~='icon']")
    if (!link) {
      link = globalThis.document.createElement('link')
      link.rel = 'icon'
      globalThis.document.head.append(link)
    }
    link.href = favicon
  }, [name, runtime.favicon_url])
  if (horizontal) return <img className="brand-lockup__custom" src={horizontal} alt={name} />
  return <span className="brand-lockup"><BrandMark /><span><strong>{name}</strong>{identityLed ? <small>{tagline}</small> : null}</span></span>
}
