import { useEffect, useId, useSyncExternalStore } from 'react'

const storageKey = 'nexusai.viewer.theme'
const changeEvent = 'nexusai:theme-change'
const mediaQuery = '(prefers-color-scheme: dark)'
const preferences = new Set(['system', 'light', 'dark'])
let memoryPreference = 'system'

const choices = [
  { value: 'system', label: 'Use device setting', detail: 'Follows this device as its appearance changes.' },
  { value: 'light', label: 'Light', detail: 'A cool, high-contrast working surface.' },
  { value: 'dark', label: 'Dark', detail: 'A low-glare slate working surface.' },
]

function readPreference() {
  try {
    const stored = globalThis.localStorage?.getItem(storageKey)
    memoryPreference = preferences.has(stored) ? stored : 'system'
  } catch { /* the in-memory preference remains usable for this tab */ }
  return memoryPreference
}

function systemTheme() {
  return globalThis.matchMedia?.(mediaQuery).matches ? 'dark' : 'light'
}

function snapshot() {
  const preference = readPreference()
  const resolved = preference === 'system' ? systemTheme() : preference
  return `${preference}:${resolved}`
}

function subscribe(listener) {
  const media = globalThis.matchMedia?.(mediaQuery)
  const changed = () => listener()
  globalThis.window?.addEventListener('storage', changed)
  globalThis.window?.addEventListener(changeEvent, changed)
  media?.addEventListener?.('change', changed)
  return () => {
    globalThis.window?.removeEventListener('storage', changed)
    globalThis.window?.removeEventListener(changeEvent, changed)
    media?.removeEventListener?.('change', changed)
  }
}

function applyToDocument(preference) {
  const root = globalThis.document?.documentElement
  if (!root) return
  if (preference === 'system') root.removeAttribute('data-theme')
  else root.setAttribute('data-theme', preference)
}

function choosePreference(preference) {
  if (!preferences.has(preference)) return
  memoryPreference = preference
  try {
    if (preference === 'system') globalThis.localStorage?.removeItem(storageKey)
    else globalThis.localStorage?.setItem(storageKey, preference)
  } catch { /* the document and in-memory state still update */ }
  applyToDocument(preference)
  globalThis.window?.dispatchEvent(new Event(changeEvent))
}

// The single source of the theme preference. Settings and the header menu both read and write it here, so
// they can never disagree.
export function useThemePreference() {
  const current = useSyncExternalStore(subscribe, snapshot, snapshot)
  const [preference, resolved] = current.split(':')
  useEffect(() => { applyToDocument(preference) }, [preference])
  return { preference, resolved, choose: choosePreference }
}

export function ThemeControl() {
  const groupName = useId()
  const { preference, resolved } = useThemePreference()

  return (
    <fieldset className="theme-control">
      <legend>Color theme</legend>
      <div className="theme-control__options">
        {choices.map(choice => (
          <label key={choice.value} className="theme-control__option">
            <input
              type="radio"
              name={`theme-${groupName}`}
              value={choice.value}
              checked={preference === choice.value}
              onChange={() => choosePreference(choice.value)}
            />
            <span className="theme-control__copy">
              <strong>{choice.label}</strong>
              <small>{choice.value === 'system' ? `${choice.detail} Currently ${resolved}.` : choice.detail}</small>
            </span>
          </label>
        ))}
      </div>
    </fieldset>
  )
}
