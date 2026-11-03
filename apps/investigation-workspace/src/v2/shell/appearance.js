import { useCallback, useEffect, useSyncExternalStore } from 'react'

// Per-browser appearance choices. The theme lives in components/ThemeControl.jsx (one source for Settings and the menu);
// density, motion and the rail are here. Each is read and written inside try/catch because storage can be blocked, and
// each is applied to <html> so CSS can react (data-density, data-motion).
const changeEvent = 'nexusai:appearance-change'

function readStored(key, allowed, fallback) {
  try {
    const value = globalThis.localStorage?.getItem(key)
    return allowed.includes(value) ? value : fallback
  } catch { return fallback }
}

function writeStored(key, value, fallback) {
  try {
    if (value === fallback) globalThis.localStorage?.removeItem(key)
    else globalThis.localStorage?.setItem(key, value)
  } catch { /* the page still changes for this tab */ }
  globalThis.window?.dispatchEvent(new Event(changeEvent))
}

function subscribe(listener) {
  globalThis.window?.addEventListener('storage', listener)
  globalThis.window?.addEventListener(changeEvent, listener)
  return () => {
    globalThis.window?.removeEventListener('storage', listener)
    globalThis.window?.removeEventListener(changeEvent, listener)
  }
}

function useStoredChoice(key, allowed, fallback, apply) {
  const value = useSyncExternalStore(subscribe, () => readStored(key, allowed, fallback), () => fallback)
  useEffect(() => { apply?.(value) }, [value, apply])
  const choose = useCallback(next => { if (allowed.includes(next)) writeStored(key, next, fallback) }, [key, allowed, fallback])
  return [value, choose]
}

const densityChoices = ['compact', 'default', 'comfortable']
const applyDensity = value => {
  const root = globalThis.document?.documentElement
  if (!root) return
  if (value === 'default') root.removeAttribute('data-density')
  else root.setAttribute('data-density', value)
}
export const DENSITY_KEY = 'nexusai.viewer.density'
export function useDensity() { return useStoredChoice(DENSITY_KEY, densityChoices, 'default', applyDensity) }

const motionChoices = ['system', 'reduce']
const applyMotion = value => {
  const root = globalThis.document?.documentElement
  if (!root) return
  if (value === 'reduce') root.setAttribute('data-motion', 'reduce')
  else root.removeAttribute('data-motion')
}
export const MOTION_KEY = 'nexusai.viewer.motion'
export function useMotion() { return useStoredChoice(MOTION_KEY, motionChoices, 'system', applyMotion) }

// The rail: 'expanded' or 'collapsed'. A page that carries its own left column (Ask, the workbench, a file viewer) asks for the
// collapsed rail by default; the analyst's own choice, once made on such a page, is remembered for that kind of page.
const railChoices = ['expanded', 'collapsed']
export function useRail(kind = 'default', defaultValue = 'expanded') {
  const key = `nexusai.viewer.rail.${kind}`
  return useStoredChoice(key, railChoices, defaultValue, undefined)
}
