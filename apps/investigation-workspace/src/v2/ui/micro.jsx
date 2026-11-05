// Small pictures that carry one number's story and nothing else: no axes, no gridlines. Each has a name for a screen reader and the exact values in its marks' titles,
// and every one is made of plain elements, so it scales with its box, prints, and follows the theme. Positions are passed as custom properties and placed by the
// stylesheet with logical properties, so a right-to-left page mirrors them without a line of script.

// Parts of a whole in one row, with a 2px surface gap between them (the gap separates the parts; no outline is drawn). `parts`: [{ id, value, colour, title }].
export function SegBar({ parts, label, height = 8 }) {
  const shown = parts.filter(part => part.value > 0)
  return (
    <span className="nx-seg" role="img" aria-label={label} style={{ blockSize: height }}>
      {shown.map((part, index) => <i key={part.id} className="nx-seg__part" style={{ flexGrow: part.value, background: part.colour, '--i': index }} title={part.title} />)}
    </span>
  )
}

// One bar for each period, scaled to the largest in the row (the shape of the row is the point; the exact value is in each bar's title). A period with nothing is a gap, not a bar of zero.
export function BarStrip({ values, label, height = 28, max: sharedMax }) {
  const max = sharedMax || Math.max(1, ...values.map(item => item.value))
  return (
    <span className="nx-bars" role="img" aria-label={label} style={{ blockSize: height }}>
      {values.map((item, index) => <i key={item.key} className={`nx-bars__bar${item.value ? '' : ' is-empty'}`} style={item.value ? { '--h': `${Math.max(6, (item.value / max) * 100)}%`, '--i': Math.min(index, 24) } : undefined} title={item.title} />)}
    </span>
  )
}

// How long something took, as a range: the line runs from the fastest to the slowest, the thicker part holds the middle half, the dot is the median.
// All the strips of a chart share one axis (0 to `axisMax`), so they can be compared at a glance.
export function RangeStrip({ stats, axisMax, label, reference }) {
  const from = value => `${Math.min(100, Math.max(0, (value / axisMax) * 100))}%`
  const to = value => `${100 - Math.min(100, Math.max(0, (value / axisMax) * 100))}%`
  return (
    <span className="nx-range" role="img" aria-label={label} style={{ '--min': from(stats.min), '--max': to(stats.max), '--p25': from(stats.p25), '--p75': to(stats.p75), '--median': from(stats.median), ...(reference != null ? { '--ref': from(reference) } : {}) }}>
      {reference != null ? <i className="nx-range__ref" /> : null}
      <i className="nx-range__line" />
      <i className="nx-range__body" />
      <i className="nx-range__dot" />
    </span>
  )
}

// Columns for equal periods, each a stack of parts (from the bottom), scaled to the tallest period. A period with nothing is a thin tick, not an invisible gap. `columns`: [{ key, title, parts: [{ id, value, colour }] }].
export function ColumnChart({ columns, label, height = 64 }) {
  const max = Math.max(1, ...columns.map(column => column.parts.reduce((total, part) => total + part.value, 0)))
  return (
    <span className="nx-cols" role="img" aria-label={label} style={{ blockSize: height }}>
      {columns.map((column, index) => {
        const total = column.parts.reduce((sum, part) => sum + part.value, 0)
        return (
          <span key={column.key} className={`nx-cols__col${total ? '' : ' is-empty'}`} title={column.title} style={total ? { '--h': `${Math.max(6, (total / max) * 100)}%`, '--i': index } : undefined}>
            {column.parts.filter(part => part.value > 0).map(part => <i key={part.id} style={{ flexGrow: part.value, background: part.colour }} />)}
          </span>
        )
      })}
    </span>
  )
}

// Two values on one axis joined by a line: the low one a filled dot, the high one a ring. Both start from zero on a shared axis, so the length of the line is the difference.
export function Dumbbell({ low, high, axisMax, label }) {
  const from = value => `${Math.min(100, Math.max(0, (value / axisMax) * 100))}%`
  return (
    <span className="nx-dumb" role="img" aria-label={label} style={{ '--lo': from(low), '--hi': from(high), '--hi-end': `${100 - Math.min(100, Math.max(0, (high / axisMax) * 100))}%` }}>
      <i className="nx-dumb__line" /><i className="nx-dumb__dot nx-dumb__dot--lo" /><i className="nx-dumb__dot nx-dumb__dot--hi" />
    </span>
  )
}

// One dot for each thing, in the colour of its cause (a unit chart: the count is the number of dots). More than `max` dots end in "+N".
export function DotRow({ dots, max = 24, label }) {
  const shown = dots.slice(0, max)
  return (
    <span className="nx-dots" role="img" aria-label={label}>
      {shown.map((dot, index) => <i key={dot.id || index} style={{ background: dot.colour, '--i': Math.min(index, 24) }} title={dot.title} />)}
      {dots.length > max ? <b>+{dots.length - max}</b> : null}
    </span>
  )
}

// A distribution in adjacent bins on one scale; the bin that holds the median is the accent, the others are quiet. `bins`: [{ key, count, title, mark }].
export function Histogram({ bins, label, height = 36 }) {
  const max = Math.max(1, ...bins.map(bin => bin.count))
  return (
    <span className="nx-hist" role="img" aria-label={label} style={{ blockSize: height }}>
      {bins.map((bin, index) => <i key={bin.key} className={`${bin.mark ? 'is-mark' : ''}${bin.count ? '' : ' is-empty'}`.trim() || undefined} style={bin.count ? { '--h': `${Math.max(8, (bin.count / max) * 100)}%`, '--i': index } : undefined} title={bin.title} />)}
    </span>
  )
}
