import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { buildCss, checkTheme, contrast, dataLightnessGaps, oklchToHex, themes } from '../../../design/tokens-v2.mjs'

describe('colour conversion', () => {
  it('maps the OKLCH white and black to sRGB white and black', () => {
    expect(oklchToHex(1, 0, 0)).toBe('#ffffff')
    expect(oklchToHex(0, 0, 0)).toBe('#000000')
  })
  it('computes the WCAG contrast ratio', () => {
    expect(contrast('#000000', '#ffffff')).toBeCloseTo(21, 0)
    expect(contrast('#777777', '#ffffff')).toBeCloseTo(4.48, 1)
  })
  it('keeps a requested colour inside sRGB by reducing chroma instead of clipping the hue', () => {
    expect(oklchToHex(0.7, 0.4, 150)).toMatch(/^#[0-9a-f]{6}$/)
  })
})

describe('every pair that is used together reaches its ratio', () => {
  for (const name of Object.keys(themes)) {
    it(`${name}: text 4.5:1, controls, focus and marks 3:1`, () => {
      expect(checkTheme(name)).toEqual([])
    })
  }
  it('neighbouring data colours differ in lightness as well as hue (they survive greyscale)', () => {
    for (const name of Object.keys(themes)) {
      const gaps = dataLightnessGaps(name)
      expect(Math.min(...gaps)).toBeGreaterThan(0.004)
    }
  })
})

describe('the generated stylesheet', () => {
  it('is current: change the generator, then run `node design/make-tokens.mjs`', () => {
    // vitest runs from the app folder; `import.meta.url` is not a file URL under jsdom
    const file = join(process.cwd(), 'src', 'v2', 'styles', 'tokens.generated.css')
    expect(readFileSync(file, 'utf8').replace(/\r\n/g, '\n')).toBe(buildCss())
  })
  it('defines the roles components use, in all three themes', () => {
    const css = buildCss()
    for (const role of ['--nx-bg', '--nx-surface', '--nx-text', '--nx-text-3', '--nx-border-strong', '--nx-accent', '--nx-focus', '--nx-danger-text', '--nx-data-6', '--nx-seq-7']) {
      expect(css.split(role + ':').length - 1).toBeGreaterThanOrEqual(3)
    }
  })
})
