import { describe, expect, it } from 'vitest'
import { clampZoom, findMatches, fitScale, formatClock, pageMatchCounts, peaksFrom, recordFields, splitByMatches, ZOOM_MAX, ZOOM_MIN } from './viewerTools.js'

describe('formatClock', () => {
  it('reads seconds as minutes and hours, and bad input as zero', () => {
    expect(formatClock(0)).toBe('0:00')
    expect(formatClock(65.9)).toBe('1:05')
    expect(formatClock(3723)).toBe('1:02:03')
    expect(formatClock(-4)).toBe('0:00')
    expect(formatClock('x')).toBe('0:00')
    expect(formatClock(undefined)).toBe('0:00')
  })
})

describe('find in document', () => {
  it('finds every occurrence, case-insensitively, as offsets', () => {
    expect(findMatches('Call at 08:42, call again', 'call')).toEqual([{ start: 0, end: 4 }, { start: 15, end: 19 }])
    expect(findMatches('abc', '')).toEqual([])
    expect(findMatches('', 'a')).toEqual([])
  })

  it('treats the query as text, not as a pattern', () => {
    expect(findMatches('a.b a+b', '.')).toEqual([{ start: 1, end: 2 }])
    expect(findMatches('a+b', 'a+b')).toEqual([{ start: 0, end: 3 }])
    expect(findMatches('(x)', '(x)')).toHaveLength(1)
  })

  it('caps a runaway search', () => {
    expect(findMatches('a'.repeat(2000), 'a')).toHaveLength(500)
  })

  it('cuts text into plain and matched runs, and counts matches per page', () => {
    const matches = findMatches('one two one', 'one')
    expect(splitByMatches('one two one', matches)).toEqual([{ text: 'one', match: true, index: 0 }, { text: ' two ', match: false }, { text: 'one', match: true, index: 1 }])
    expect(splitByMatches('plain', [])).toEqual([{ text: 'plain', match: false }])
    const counts = pageMatchCounts([{ number: 1, text: 'a a' }, { number: 2, text: 'b' }], 'a')
    expect([...counts.entries()]).toEqual([[1, 2], [2, 0]])
  })
})

describe('zoom', () => {
  it('keeps zoom within limits and falls back to 1 for nonsense', () => {
    expect(clampZoom(100)).toBe(ZOOM_MAX)
    expect(clampZoom(0)).toBe(ZOOM_MIN)
    expect(clampZoom('abc')).toBe(1)
    expect(clampZoom(2)).toBe(2)
  })

  it('fits an image to its frame without enlarging it', () => {
    expect(fitScale({ width: 500, height: 400 }, { width: 1000, height: 1000 })).toBe(0.4)
    expect(fitScale({ width: 500, height: 400 }, { width: 100, height: 100 })).toBe(1)
    expect(fitScale(null, { width: 10, height: 10 })).toBe(1)
  })
})

describe('peaksFrom', () => {
  it('summarises samples as normalised peaks', () => {
    const peaks = peaksFrom([0, 0.5, -1, 0.25, 0, 0, 0.1, -0.1], 4)
    expect(peaks).toHaveLength(4)
    expect(Math.max(...peaks)).toBe(1)
    expect(peaks[1]).toBe(1)
    expect(peaks[2]).toBe(0)
  })

  it('returns silence for no samples', () => {
    expect(peaksFrom([], 3)).toEqual([0, 0, 0])
    expect(peaksFrom(null, 2)).toEqual([0, 0])
  })
})

describe('recordFields', () => {
  it('lists a record in column order and drops empty fields and internal columns', () => {
    const columns = [{ key: 'a', label: 'Caller' }, { key: 'b', label: 'Receiver' }, { key: '__source', label: 'Source' }, { key: 'c' }]
    expect(recordFields({ a: '0300', b: '', __source: 'x', c: 0 }, columns)).toEqual([{ key: 'a', label: 'Caller', value: '0300' }, { key: 'c', label: 'c', value: 0 }])
  })
})

describe('regions and cues', () => {
  it('reads a box as fractions whether it came as fractions or pixels', async () => {
    const { normalizeBox } = await import('./viewerTools.js')
    expect(normalizeBox([0.1, 0.2, 0.3, 0.4], null)).toEqual([0.1, 0.2, 0.3, 0.4])
    expect(normalizeBox([100, 50, 200, 100], { width: 1000, height: 500 })).toEqual([0.1, 0.1, 0.2, 0.2])
    expect(normalizeBox([100, 50, 200, 100], null)).toBeNull()
    expect(normalizeBox(['a', 1, 1, 1], null)).toBeNull()
  })

  it('brings a region to the middle of the frame with room around it', async () => {
    const { focusRegion } = await import('./viewerTools.js')
    const view = focusRegion([0.4, 0.4, 0.2, 0.2], { width: 1000, height: 1000 }, { width: 500, height: 500 })
    expect(view.x).toBeCloseTo(0)
    expect(view.y).toBeCloseTo(0)
    expect(view.zoom).toBeGreaterThan(1)
    const offCentre = focusRegion([0.0, 0.0, 0.2, 0.2], { width: 1000, height: 1000 }, { width: 500, height: 500 })
    expect(offCentre.x).toBeGreaterThan(0)
    expect(focusRegion(null, { width: 1, height: 1 }, { width: 1, height: 1 })).toBeNull()
  })

  it('finds the cue being spoken', async () => {
    const { activeCueIndex } = await import('./viewerTools.js')
    const cues = [{ start: 0, end: 2 }, { start: 2, end: 5 }, { start: 6, end: null }]
    expect(activeCueIndex(cues, 1)).toBe(0)
    expect(activeCueIndex(cues, 2)).toBe(1)
    expect(activeCueIndex(cues, 5.5)).toBe(-1)
    expect(activeCueIndex(cues, 99)).toBe(2)
  })
})

describe('scrollWithin', () => {
  const box = (top, height) => ({ getBoundingClientRect: () => ({ top, bottom: top + height, height }) })
  it('scrolls only the container, to the element or its centre, and not at all when it is already in view', async () => {
    const { scrollWithin } = await import('./viewerTools.js')
    const container = { ...box(100, 200), scrollTop: 50, clientHeight: 200, scrollTo: ({ top }) => { container.scrollTop = top } }
    scrollWithin(container, box(150, 20))
    expect(container.scrollTop).toBe(50)
    scrollWithin(container, box(320, 20))
    expect(container.scrollTop).toBe(50 + 320 - 100 - 200 + 20)
    scrollWithin(container, box(100, 20), 'center')
    expect(container.scrollTop).toBe(Math.max(0, container.scrollTop))
    expect(() => scrollWithin(null, null)).not.toThrow()
  })
})
