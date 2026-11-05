// A small line with the shape of a series and nothing else: no axes, the last point marked "now". Drawn only from a real series (the caller passes at least seven real points);
// a series that is all zero is drawn flat, never as a trend. The accessible name states what it shows. Line 2px, area a wash of the same hue, end marker with a surface ring.
export function Sparkline({ values, width = 96, height = 28, label, tone = 'accent' }) {
  const count = values.length
  if (count < 2) return null
  const max = Math.max(...values, 0)
  const pad = 5
  const step = (width - pad) / (count - 1)
  const y = value => (max ? height - 3 - (value / max) * (height - 3 - pad) : height - 3)
  const points = values.map((value, index) => [index * step, y(value)])
  const line = points.map(([x, py], index) => `${index ? 'L' : 'M'}${x.toFixed(1)} ${py.toFixed(1)}`).join(' ')
  const [endX, endY] = points[points.length - 1]
  return (
    <svg className={`nx-spark nx-spark--${tone}`} viewBox={`0 0 ${width} ${height}`} width={width} height={height} role="img" aria-label={label}>
      <path d={`${line} L${endX.toFixed(1)} ${height} L0 ${height} Z`} className="nx-spark__area" />
      <path d={line} className="nx-spark__line" fill="none" />
      <circle cx={endX} cy={endY} r="3.5" className="nx-spark__dot" />
    </svg>
  )
}

// The last `span` calendar days ending on `last`, one count per day (a day with no events is a zero).
export function dailySeries(days, last, span, shiftDays) {
  if (!last) return []
  const byDate = new Map(days.map(day => [day.date, day.total]))
  return Array.from({ length: span }, (_, index) => byDate.get(shiftDays(last, index - (span - 1))) || 0)
}
