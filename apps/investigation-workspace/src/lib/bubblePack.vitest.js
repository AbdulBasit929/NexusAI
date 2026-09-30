import { describe, expect, it } from 'vitest'
import { packCircles } from './bubblePack.js'

const items = [{ id: 'cdr', value: 22000 }, { id: 'ipdr', value: 3400 }, { id: 'anpr', value: 900 }, { id: 'access_log', value: 120 }, { id: 'transaction', value: 40 }]
const W = 560
const H = 300

describe('packCircles', () => {
  it('places every positive item inside the frame without overlap', () => {
    const packed = packCircles(items, W, H)
    expect(packed).toHaveLength(items.length)
    for (const circle of packed) {
      expect(circle.x - circle.r).toBeGreaterThanOrEqual(0)
      expect(circle.x + circle.r).toBeLessThanOrEqual(W)
      expect(circle.y - circle.r).toBeGreaterThanOrEqual(0)
      expect(circle.y + circle.r).toBeLessThanOrEqual(H)
    }
    for (let i = 0; i < packed.length; i += 1) for (let j = i + 1; j < packed.length; j += 1) {
      expect(Math.hypot(packed[i].x - packed[j].x, packed[i].y - packed[j].y)).toBeGreaterThanOrEqual(packed[i].r + packed[j].r)
    }
  })

  it('makes area follow the value: a bigger count never has a smaller circle', () => {
    const packed = packCircles(items, W, H)
    const radii = packed.map(circle => circle.r)
    expect([...radii].sort((a, b) => b - a)).toEqual(radii)
    expect(packed[0].id).toBe('cdr')
    expect(packed[0].r).toBeGreaterThan(packed[1].r)
  })

  it('is deterministic, so the map does not shuffle between refreshes', () => {
    expect(packCircles(items, W, H)).toEqual(packCircles([...items].reverse(), W, H))
  })

  it('handles one item, none, and zero or invalid values', () => {
    const single = packCircles([{ id: 'cdr', value: 5 }], W, H)
    expect(single).toHaveLength(1)
    expect(single[0].x).toBeCloseTo(W / 2, 0)
    expect(packCircles([], W, H)).toEqual([])
    expect(packCircles([{ id: 'a', value: 0 }, { id: 'b', value: -3 }], W, H)).toEqual([])
  })

  it('still fits many families by shrinking the largest', () => {
    const many = Array.from({ length: 12 }, (_, index) => ({ id: `f${index}`, value: 1000 - index * 60 }))
    expect(packCircles(many, W, H)).toHaveLength(12)
  })
})
