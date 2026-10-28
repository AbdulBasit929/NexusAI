import { useEffect, useId, useSyncExternalStore } from 'react'

const storageKey = 'nexusai.viewer.theme'
const changeEvent = 'nexusai:theme-change'
const mediaQuery = '(prefers-color-scheme: dark)'
const preferences = new Set(['system', 'light', 'dark', 'high-contrast'])
let memoryPreference = 'system'

const choices = [
  { value: 'system', label: 'Use device setting', detail: 'Follows this device as its appearance changes.' },
  { value: 'light', label: 'Light', detail: 'A cool, high-contrast working surface.' },
  { value: 'dark', label: 'Dark', detail: 'A low-glare slate working surface.' },
  { value: 'high-contrast', label: 'High contrast', detail: 'Maximum separation between text, controls and working surfaces.' },
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

const displayChangeEvent = 'nexusai:display-change'
const displayOptions = {
  density: { key: 'nexusai.viewer.density', values: new Set(['compact', 'default', 'comfortable']), fallback: 'default', attribute: 'data-density' },
  motion: { key: 'nexusai.viewer.reduce-motion', values: new Set(['system', 'reduce']), fallback: 'system', attribute: 'data-reduce-motion' },
  detail: { key: 'nexusai.viewer.answer-detail', values: new Set(['concise', 'standard', 'detailed']), fallback: 'standard', attribute: 'data-answer-detail' },
}

function readDisplayOption(name) {
  const option = displayOptions[name]
  try {
    const stored = globalThis.localStorage?.getItem(option.key)
    return option.values.has(stored) ? stored : option.fallback
  } catch { return option.fallback }
}

function applyDisplayOption(name, value) {
  const option = displayOptions[name]
  const root = globalThis.document?.documentElement
  if (!root) return
  if (value === option.fallback) root.removeAttribute(option.attribute)
  else root.setAttribute(option.attribute, value)
}

export function chooseDisplayPreference(name, value) {
  const option = displayOptions[name]
  if (!option?.values.has(value)) return
  try {
    if (value === option.fallback) globalThis.localStorage?.removeItem(option.key)
    else globalThis.localStorage?.setItem(option.key, value)
  } catch { /* preferences still apply for this document */ }
  applyDisplayOption(name, value)
  globalThis.window?.dispatchEvent(new Event(displayChangeEvent))
}

function displaySnapshot() {
  return Object.keys(displayOptions).map(name => `${name}:${readDisplayOption(name)}`).join('|')
}

function subscribeDisplay(listener) {
  const changed = () => listener()
  globalThis.window?.addEventListener('storage', changed)
  globalThis.window?.addEventListener(displayChangeEvent, changed)
  return () => {
    globalThis.window?.removeEventListener('storage', changed)
    globalThis.window?.removeEventListener(displayChangeEvent, changed)
  }
}

export function useDisplayPreferences() {
  const current = useSyncExternalStore(subscribeDisplay, displaySnapshot, displaySnapshot)
  const values = Object.fromEntries(current.split('|').map(item => item.split(':')))
  useEffect(() => { for (const [name, value] of Object.entries(values)) applyDisplayOption(name, value) }, [current])
  return { ...values, choose: chooseDisplayPreference }
}
