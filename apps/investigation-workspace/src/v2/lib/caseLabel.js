// A case with no name an analyst gave it is known by its identifier, for example `orion-lahore-reference-20261030`. The
// identifier stays available (as a title and in monospace where there is room); this reads it as a title and a date so the
// screen does not lead with a slug. It only re-formats the identifier: nothing is invented.
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
const DATE_TAIL = /[-_ ]?(20\d{2})(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])$/

export function humanizeCaseId(id) {
  const raw = String(id || '').trim()
  if (!raw) return { title: '', date: '' }
  let text = raw
  let date = ''
  const match = text.match(DATE_TAIL)
  if (match && match.index > 0) {
    text = text.slice(0, match.index)
    date = `${Number(match[3])} ${MONTHS[Number(match[2]) - 1]} ${match[1]}`
  }
  const words = text.split(/[-_\s]+/).filter(Boolean)
  const title = words.map(word => (/^\d/.test(word) ? word : `${word[0].toUpperCase()}${word.slice(1)}`)).join(' ')
  return { title: title || raw, date }
}
