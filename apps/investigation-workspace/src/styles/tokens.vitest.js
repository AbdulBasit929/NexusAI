import fs from 'node:fs'
import path from 'node:path'
import { describe, expect, test } from 'vitest'

const source = fs.readFileSync(path.join(process.cwd(), 'src/styles/tokens.css'), 'utf8')

function block(pattern) {
  const match = source.match(pattern)
  if (!match) throw new Error(`Token block not found: ${pattern}`)
  return Object.fromEntries([...match[1].matchAll(/--([\w-]+):\s*([^;]+);/g)].map(item => [item[1], item[2].trim()]))
}

const light = block(/^:root\s*\{([\s\S]*?)\n\}/m)
const darkMedia = block(/:root:not\(\[data-theme='light'\]\)\s*\{([\s\S]*?)\n\s*\}/m)
const darkForced = block(/:root\[data-theme='dark'\]\s*\{([\s\S]*?)\n\}/m)

function luminance(hex) {
  const channels = hex.slice(1).match(/../g).map(value => parseInt(value, 16) / 255)
    .map(value => value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4)
  return (0.2126 * channels[0]) + (0.7152 * channels[1]) + (0.0722 * channels[2])
}

function mix(top, base, alpha) {
  const channel = index => Math.round(parseInt(top.slice(1 + index * 2, 3 + index * 2), 16) * alpha + parseInt(base.slice(1 + index * 2, 3 + index * 2), 16) * (1 - alpha))
  return `#${[0, 1, 2].map(index => channel(index).toString(16).padStart(2, '0')).join('')}`
}

function contrast(foreground, background) {
  const first = luminance(foreground)
  const second = luminance(background)
  return (Math.max(first, second) + 0.05) / (Math.min(first, second) + 0.05)
}

const surfaces = ['analyst-surface-0', 'analyst-surface-1', 'analyst-surface-2', 'analyst-surface-3', 'analyst-surface-ask', 'analyst-surface-highlight']

describe('visual-system tokens', () => {
  test('keeps the seven-step type scale and four deliberate weights', () => {
    expect(Object.keys(light).filter(name => /^analyst-type-.+-size$/.test(name))).toHaveLength(7)
    expect([
      light['analyst-font-weight-regular'],
      light['analyst-font-weight-medium'],
      light['analyst-font-weight-semibold'],
      light['analyst-font-weight-bold'],
    ]).toEqual(['400', '500', '600', '700'])
  })

  test('uses the specified namespace, dimensions, and spacing rhythm', () => {
    expect(Object.keys(light).every(name => name.startsWith('analyst-'))).toBe(true)
    expect(light['analyst-control-height']).toBe('42px')
    expect(light['analyst-content-wide']).toBe('1280px')
    expect(light['analyst-content-reading']).toBe('1060px')
    expect([light['analyst-radius-sm'], light['analyst-radius-md'], light['analyst-radius-lg']]).toEqual(['8px', '12px', '16px'])
    expect(Object.keys(light).filter(name => /^analyst-space-\d+$/.test(name)).map(name => light[name])).toEqual(['8px', '12px', '16px', '20px', '24px', '32px', '48px'])
  })

  test('declares Latin, Urdu, and identifier font roles', () => {
    expect(light['analyst-font-ui']).toContain('Noto Sans Variable')
    expect(light['analyst-font-urdu']).toContain('Noto Sans Arabic Variable')
    expect(light['analyst-font-mono']).toContain('Noto Sans Mono Variable')
  })

  test('declares the governed surface, status, evidence, and focus vocabulary', () => {
    for (const token of ['analyst-surface-0', 'analyst-surface-1', 'analyst-surface-2', 'analyst-surface-3', 'analyst-surface-ask', 'analyst-focus-ring', 'analyst-status-ready', 'analyst-status-processing', 'analyst-status-failed', 'analyst-status-excluded', 'analyst-evidence-strong', 'analyst-evidence-medium', 'analyst-evidence-weak']) expect(light[token]).toMatch(/^#[0-9a-f]{6}$/i)
    expect(light['analyst-brand-primary']).toBe('#2563eb')
    expect(light['analyst-brand-accent']).toBe('#14b8a6')
    expect(light['analyst-theme-name']).toBe("'Header Teal Light'")
    expect(darkForced['analyst-theme-name']).toBe("'Header Teal Dark'")
  })

  test('keeps the media and forced dark palettes identical', () => {
    const darkColorTokens = Object.keys(darkForced).filter(name => darkForced[name].startsWith('#'))
    for (const name of darkColorTokens) expect(darkMedia[name]).toBe(darkForced[name])
  })

  test.each([['light', light], ['dark', { ...light, ...darkForced }]])('%s text tokens meet 4.5:1 on every adjacent surface', (_name, tokens) => {
    for (const textToken of ['analyst-text', 'analyst-text-secondary', 'analyst-text-muted', 'analyst-accent']) {
      for (const surface of surfaces) expect(contrast(tokens[textToken], tokens[surface])).toBeGreaterThanOrEqual(4.5)
    }
    expect(contrast(tokens['analyst-accent-text'], tokens['analyst-accent'])).toBeGreaterThanOrEqual(4.5)
    expect(contrast(tokens['analyst-text-on-accent'], tokens['analyst-accent'])).toBeGreaterThanOrEqual(4.5)
    expect(contrast(tokens['analyst-text-inverse'], tokens['analyst-surface-inverse'])).toBeGreaterThanOrEqual(4.5)
    for (const textToken of ['analyst-status-ready', 'analyst-status-processing', 'analyst-status-failed', 'analyst-status-excluded', 'analyst-evidence-strong', 'analyst-evidence-medium', 'analyst-evidence-weak']) {
      for (const surface of ['analyst-surface-1', 'analyst-surface-2', 'analyst-surface-3']) {
        expect(contrast(tokens[textToken], tokens[surface])).toBeGreaterThanOrEqual(4.5)
      }
    }
  })

  test.each([['light', light], ['dark', { ...light, ...darkForced }]])('%s controls and focus meet 3:1 on every adjacent surface', (_name, tokens) => {
    for (const surface of surfaces) {
      expect(contrast(tokens['analyst-line-strong'], tokens[surface])).toBeGreaterThanOrEqual(3)
      expect(contrast(tokens['analyst-focus-ring'], tokens[surface])).toBeGreaterThanOrEqual(3)
    }
  })

  test.each([['light', light], ['dark', { ...light, ...darkForced }]])('%s sidebar text, active route and focus hold on both gradient stops', (_name, tokens) => {
    const stops = tokens['analyst-navigation-surface'].match(/#[0-9a-f]{6}/gi)
    expect(stops).toHaveLength(2)
    for (const stop of stops) {
      expect(contrast(tokens['analyst-rail-text'], stop)).toBeGreaterThanOrEqual(4.5)
      expect(contrast(tokens['analyst-rail-muted'], stop)).toBeGreaterThanOrEqual(4.5)
      expect(contrast(tokens['analyst-rail-marker'], stop)).toBeGreaterThanOrEqual(3)
      expect(contrast(tokens['analyst-rail-focus'], stop)).toBeGreaterThanOrEqual(3)
    }
    expect(contrast(tokens['analyst-rail-active-text'], tokens['analyst-rail-active'])).toBeGreaterThanOrEqual(4.5)
    expect(contrast(tokens['analyst-rail-marker'], tokens['analyst-rail-active'])).toBeGreaterThanOrEqual(3)
  })

  test.each([['light', light], ['dark', { ...light, ...darkForced }]])('%s header action and live status chip stay legible on both gradient stops', (_name, tokens) => {
    for (const stop of [tokens['analyst-shell-base'], tokens['analyst-shell-end']]) {
      expect(contrast('#fde9b0', stop)).toBeGreaterThanOrEqual(4.5)
      expect(contrast(tokens['analyst-rail-marker'], stop)).toBeGreaterThanOrEqual(3)
      expect(contrast(tokens['analyst-rail-focus'], stop)).toBeGreaterThanOrEqual(3)
    }
    // Ghost-accent "Ask a question": mint text over a 12% mint wash, border at 80% mint, on both gradient stops.
    for (const stop of [tokens['analyst-shell-base'], tokens['analyst-shell-end']]) {
      const wash = mix('#5eead4', stop, 0.12)
      expect(contrast('#ccfbf1', wash)).toBeGreaterThanOrEqual(4.5)
      expect(contrast('#ffffff', mix('#5eead4', stop, 0.22))).toBeGreaterThanOrEqual(4.5)
      expect(contrast(mix('#5eead4', stop, 0.8), wash)).toBeGreaterThanOrEqual(3)
    }
  })

  test.each([['light', light], ['dark', { ...light, ...darkForced }]])('%s state-panel marks are 3:1 on their tinted background and the tint sits on a card', (_name, tokens) => {
    for (const [tone, tint] of [['analyst-status-ready', 'analyst-positive-surface'], ['analyst-status-processing', 'analyst-caution-surface'], ['analyst-status-failed', 'analyst-critical-surface'], ['analyst-text-secondary', 'analyst-surface-emphasis']]) {
      expect(contrast(tokens[tone], tokens[tint])).toBeGreaterThanOrEqual(3)
    }
    expect(contrast(tokens['analyst-text'], tokens['analyst-critical-surface'])).toBeGreaterThanOrEqual(4.5)
    expect(contrast(tokens['analyst-text'], tokens['analyst-caution-surface'])).toBeGreaterThanOrEqual(4.5)
  })

  test.each([['light', light], ['dark', { ...light, ...darkForced }]])('%s chart colours are 3:1 against every work surface', (_name, tokens) => {
    for (let index = 1; index <= 6; index += 1) {
      for (const surface of ['analyst-surface-1', 'analyst-surface-3']) expect(contrast(tokens[`analyst-data-${index}`], tokens[surface])).toBeGreaterThanOrEqual(3)
    }
  })

  test.each([['light', light], ['dark', { ...light, ...darkForced }]])('%s shell text and controls remain legible across the identity field', (_name, tokens) => {
    for (const surface of ['analyst-shell-base', 'analyst-shell-end']) {
      expect(contrast(tokens['analyst-shell-text'], tokens[surface])).toBeGreaterThanOrEqual(4.5)
      expect(contrast(tokens['analyst-shell-muted'], tokens[surface])).toBeGreaterThanOrEqual(4.5)
      expect(contrast(tokens['analyst-shell-line'], tokens[surface])).toBeGreaterThanOrEqual(3)
    }
    expect(contrast(tokens['analyst-shell-line'], tokens['analyst-shell-control'])).toBeGreaterThanOrEqual(3)
  })
})
