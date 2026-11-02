import { describe, expect, it } from 'vitest'
import { readSpreadsheetDate } from './spreadsheetDates.js'

describe('readSpreadsheetDate', () => {
  it('reads the number of a date column as the moment it stands for, to the second', () => {
    // the typed ingest read this very cell as 2026-02-10 15:53:41 UTC, which is 20:53:41 in Karachi, the file's own time
    expect(readSpreadsheetDate('Date & Time', '46063.870613425926')).toBe('2026-02-10 20:53:41')
    expect(readSpreadsheetDate('STRT_TM', '46112')).toBe('2026-03-31 00:00:00')
    expect(readSpreadsheetDate('Timestamp', '45000.5')).toBe('2023-03-15 12:00:00')
  })

  it('says nothing for a column that is not a date, for text, and for a number outside the years of a file', () => {
    expect(readSpreadsheetDate('Duration', '46063')).toBe('')
    expect(readSpreadsheetDate('Sr #', '91')).toBe('')
    expect(readSpreadsheetDate('Date & Time', '31/03/2026 14:46:23')).toBe('')
    expect(readSpreadsheetDate('Date & Time', '123')).toBe('')
    expect(readSpreadsheetDate('Date & Time', '99999')).toBe('')
    expect(readSpreadsheetDate('Date & Time', '')).toBe('')
    expect(readSpreadsheetDate(undefined, '46063')).toBe('')
  })
})
