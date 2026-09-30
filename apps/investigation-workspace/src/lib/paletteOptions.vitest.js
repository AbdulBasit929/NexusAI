import { describe, expect, test } from 'vitest'
import { contrastRatio, mixHex } from './contrast.js'
import { measurePalette, paletteOptions } from './paletteOptions.js'

describe('contrast helpers', () => {
  test('match the WCAG reference values', () => {
    expect(contrastRatio('#000000', '#ffffff')).toBeCloseTo(21, 5)
    expect(contrastRatio('#777777', '#ffffff')).toBeCloseTo(4.48, 2)
    expect(mixHex('#000000', '#ffffff', 0.5)).toBe('#808080')
  })
})

describe('shell palette options', () => {
  test('offers three options that each define both themes', () => {
    expect(paletteOptions.map(option => option.id)).toEqual(['A', 'B', 'C'])
    for (const option of paletteOptions) expect(Object.keys(option.modes)).toEqual(['light', 'dark'])
  })

  test.each(paletteOptions.flatMap(option => Object.entries(option.modes).map(([name, mode]) => [`${option.id} ${name}`, mode])))('%s meets every measured contrast target', (_name, mode) => {
    const failing = measurePalette(mode).filter(row => !row.pass).map(row => `${row.label} ${row.ratio.toFixed(2)}:1`)
    expect(failing).toEqual([])
  })

  test('every option derives its header from the same navy-to-teal gradient', () => {
    for (const option of paletteOptions) {
      expect(option.modes.light.header.stops).toEqual(['#111827', '#123b43'])
      expect(option.modes.dark.header.stops).toEqual(['#070b12', '#10343a'])
    }
  })

  test('the categorical data palette is shared by every option and has six distinct colours', () => {
    for (const mode of ['light', 'dark']) {
      const sets = paletteOptions.map(option => option.modes[mode].data.join())
      expect(new Set(sets).size).toBe(1)
      expect(new Set(paletteOptions[0].modes[mode].data).size).toBe(6)
    }
  })
})
