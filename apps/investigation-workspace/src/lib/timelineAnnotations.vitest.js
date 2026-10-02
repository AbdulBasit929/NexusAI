import { afterEach, describe, expect, it } from 'vitest'
import { readPins, withNote, withPin } from './timelineAnnotations.js'

afterEach(() => globalThis.localStorage?.clear())

describe('timeline pins', () => {
  it('pins and unpins a day, keeping dates in order', () => {
    let pins = withPin([], '2026-02-02')
    pins = withPin(pins, '2026-01-01')
    expect(pins.map(pin => pin.date)).toEqual(['2026-01-01', '2026-02-02'])
    expect(withPin(pins, '2026-01-01').map(pin => pin.date)).toEqual(['2026-02-02'])
  })

  it('keeps a note on a day and caps its length', () => {
    const pins = withNote([], '2026-02-02', 'x'.repeat(500))
    expect(pins[0].note).toHaveLength(400)
    expect(withNote(pins, '2026-02-02', 'short')[0].note).toBe('short')
  })

  it('reads only well-formed pins from storage, per case', () => {
    globalThis.localStorage.setItem('nexusai.viewer.timeline.pins.a', JSON.stringify([{ date: '2026-02-02', note: 'n' }, { date: 'bad' }, null]))
    expect(readPins('a')).toEqual([{ date: '2026-02-02', note: 'n' }])
    expect(readPins('b')).toEqual([])
    globalThis.localStorage.setItem('nexusai.viewer.timeline.pins.c', '{not json')
    expect(readPins('c')).toEqual([])
  })
})
