// A spreadsheet stores a date as the number of days since 30 December 1899 (a time of day is the fraction). A workbook that was written without a date format shows it as that number (46063.870613),
// and so would any spreadsheet program. For a column that is named like a date or a time and holds such a number, this says which moment the number stands for, so the analyst can read it. The file's own
// value is never replaced: callers show it beside, or under, this reading. The moment is the wall-clock time of the file; no time zone is added.
const DATE_NAME = /(date|time|\btm\b|_tm$|timestamp|stamp)/i
const SERIAL = /^\d{5}(?:\.\d+)?$/

export function readSpreadsheetDate(columnName, value) {
  const text = String(value ?? '').trim()
  if (!DATE_NAME.test(String(columnName || '')) || !SERIAL.test(text)) return ''
  const serial = Number(text)
  if (serial < 25569 || serial > 73050) return '' // 1970 to 2099: anything else is a number, not a date
  const moment = new Date(Math.round(serial * 86400) * 1000 + Date.UTC(1899, 11, 30))
  const pad = number => String(number).padStart(2, '0')
  return `${moment.getUTCFullYear()}-${pad(moment.getUTCMonth() + 1)}-${pad(moment.getUTCDate())} ${pad(moment.getUTCHours())}:${pad(moment.getUTCMinutes())}:${pad(moment.getUTCSeconds())}`
}
