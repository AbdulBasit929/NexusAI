import { describe, expect, it } from 'vitest'
import { createQueue, fitWithin, Lru, passagePreview, rowsPreview, textDirection } from './previewStore.js'

describe('the size of a small copy', () => {
  it('shrinks a big picture to the edge and keeps its shape, and never enlarges a small one', () => {
    expect(fitWithin(3840, 2160, 480)).toEqual({ width: 480, height: 270 })
    expect(fitWithin(1000, 4000, 480)).toEqual({ width: 120, height: 480 })
    expect(fitWithin(200, 100, 480)).toEqual({ width: 200, height: 100 })
  })

  it('gives a square when the picture reported no size', () => {
    expect(fitWithin(0, 0, 480)).toEqual({ width: 480, height: 480 })
    expect(fitWithin(undefined, 10, 480)).toEqual({ width: 480, height: 480 })
  })
})

describe('the direction of a text', () => {
  it('reads Arabic-script text right to left and anything else left to right', () => {
    expect(textDirection('میرا رابطہ نمبر ۹۲ ۳۰۰ ہے۔')).toBe('rtl')
    expect(textDirection('دا ښار د هېواد')).toBe('rtl')
    expect(textDirection('Along the same line, men are required')).toBe('ltr')
    expect(textDirection('')).toBe('ltr')
    expect(textDirection('12345 --')).toBe('ltr')
  })
})

describe('the first rows of a table', () => {
  const records = [
    { raw_payload: { _xlsx_sheet_name: 'Sheet1', cell_id: '4101-77', first_seen: '2026-10-01 08:00', empty: '', imsi: '410010000000001', extra: 'x' } },
    { raw_payload: { _xlsx_sheet_name: 'Sheet1', cell_id: '4101-78', first_seen: '2026-10-01 08:05', empty: '', imsi: '410010000000002', extra: 'y' } },
    { raw_payload: { cell_id: '4101-79', first_seen: '2026-10-01 08:09', imsi: '410010000000003' } },
    { raw_payload: { cell_id: '4101-80' } },
  ]

  it('shows up to three columns that hold a value in the first row, with the file\'s own names and order, and up to three rows', () => {
    expect(rowsPreview(records)).toEqual({
      columns: ['cell id', 'first seen', 'imsi'],
      rows: [['4101-77', '2026-10-01 08:00', '410010000000001'], ['4101-78', '2026-10-01 08:05', '410010000000002'], ['4101-79', '2026-10-01 08:09', '410010000000003']],
    })
  })

  it('cuts a long cell and says nothing when there is nothing to show', () => {
    const long = rowsPreview([{ raw_payload: { note: 'a'.repeat(60) } }])
    expect(long.rows[0][0]).toHaveLength(22)
    expect(long.rows[0][0].endsWith('…')).toBe(true)
    expect(rowsPreview([])).toBeNull()
    expect(rowsPreview(null)).toBeNull()
    expect(rowsPreview([{ raw_payload: { _only: 'hidden' } }])).toBeNull()
    expect(rowsPreview([{ raw_payload: 'text' }])).toBeNull()
  })
})

describe('the first passages of a document', () => {
  const artifact = (text, locator, type = 'forensics.document-native-text-passage/v1') => ({ artifact_type: type, citation_locator: locator, metadata: { text } })

  it('takes the first passages in reading order, whatever order the service listed them in, up to three', () => {
    const detail = { derived_artifacts: [artifact('second', { paragraph: 4, passage: 1 }), artifact('first', { paragraph: 1, passage: 1 }), artifact('first b', { paragraph: 1, passage: 2 }), artifact('fourth', { paragraph: 9, passage: 1 })] }
    expect(passagePreview(detail)).toEqual({ lines: ['first', 'first b', 'second'], dir: 'ltr' })
  })

  it('knows the direction, cuts a long passage and ignores what is not a passage', () => {
    expect(passagePreview({ derived_artifacts: [artifact('میرا رابطہ نمبر ہے', { passage: 1 })] }).dir).toBe('rtl')
    expect(passagePreview({ derived_artifacts: [artifact('word '.repeat(200), { passage: 1 })] }, { maxChars: 40 }).lines[0]).toHaveLength(40)
    expect(passagePreview({ derived_artifacts: [artifact('not text', {}, 'forensics.audio-observation/v1')] })).toBeNull()
    expect(passagePreview({ derived_artifacts: [artifact('   ', { passage: 1 })] })).toBeNull()
    expect(passagePreview(null)).toBeNull()
  })
})

describe('the cache', () => {
  it('keeps the most recently used keys and hands what it pushes out to the caller', () => {
    const evicted = []
    const cache = new Lru(2, (value, key) => evicted.push(key))
    cache.set('a', 1)
    cache.set('b', 2)
    cache.get('a')
    cache.set('c', 3)
    expect([...cache.map.keys()]).toEqual(['a', 'c'])
    expect(evicted).toEqual(['b'])
    cache.set('a', 9)
    expect(evicted).toEqual(['b', 'a'])
    expect(cache.size).toBe(2)
  })
})

describe('the line of work', () => {
  it('runs at most the limit together, in the order asked, and drops a job whose card has gone', async () => {
    const queue = createQueue(2)
    let together = 0
    let most = 0
    const ran = []
    const job = name => queue.run(async () => { together += 1; most = Math.max(most, together); ran.push(name); await new Promise(resolve => setTimeout(resolve, 5)); together -= 1; return name })
    const gone = queue.run(async () => { ran.push('gone'); return 'gone' }, () => true)
    const results = await Promise.all([job('a'), job('b'), job('c'), job('d'), gone])
    expect(results).toEqual(['a', 'b', 'c', 'd', null])
    expect(most).toBe(2)
    expect(ran).toEqual(['a', 'b', 'c', 'd'])
  })

  it('goes on after a job fails', async () => {
    const queue = createQueue(1)
    const bad = queue.run(async () => { throw new Error('no') })
    const good = queue.run(async () => 'yes')
    await expect(bad).rejects.toThrow('no')
    await expect(good).resolves.toBe('yes')
  })
})
