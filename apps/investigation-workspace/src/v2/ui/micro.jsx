// Small pictures that carry one number's story and nothing else: no axes, no gridlines. Each has a name for a screen reader and the exact values in its marks' titles,
// and every one is made of plain elements, so it scales with its box, prints, and follows the theme. Positions are passed as custom properties and placed by the
// stylesheet with logical properties, so a right-to-left page mirrors them without a line of script.

// Parts of a whole in one row, with a 2px surface gap between them (the gap separates the parts; no outline is drawn). `parts`: [{ id, value, colour, title }].
export function SegBar({ parts, label, height = 8 }) {
  const shown = parts.filter(part => part.value > 0)
  return (
    <span className="nx-seg" role="img" aria-label={label} style={{ blockSize: height }}>
      {shown.map(part => <i key={part.id} className="nx-seg__part" style={{ flexGrow: part.value, background: part.colour }} title={part.title} />)}
    </span>
  )
}

// One bar for each period, scaled to the largest in the row (the shape of the row is the point; the exact value is in each bar's title). A period with nothing is a gap, not a bar of zero.
export function BarStrip({ values, label, height = 28 }) {
  const max = Math.max(1, ...values.map(item => item.value))
  return (
    <span className="nx-bars" role="img" aria-label={label} style={{ blockSize: height }}>
      {values.map(item => <i key={item.key} className={`nx-bars__bar${item.value ? '' : ' is-empty'}`} style={item.value ? { '--h': `${Math.max(6, (item.value / max) * 100)}%` } : undefined} title={item.title} />)}
    </span>
  )
}

// How long something took, as a range: the line runs from the fastest to the slowest, the thicker part holds the middle half, the dot is the median.
// All the strips of a chart share one axis (0 to `axisMax`), so they can be compared at a glance.
export function RangeStrip({ stats, axisMax, label }) {
  const from = value => `${Math.min(100, Math.max(0, (value / axisMax) * 100))}%`
  const to = value => `${100 - Math.min(100, Math.max(0, (value / axisMax) * 100))}%`
  return (
    <span className="nx-range" role="img" aria-label={label} style={{ '--min': from(stats.min), '--max': to(stats.max), '--p25': from(stats.p25), '--p75': to(stats.p75), '--median': from(stats.median) }}>
      <i className="nx-range__line" />
      <i className="nx-range__body" />
      <i className="nx-range__dot" />
    </span>
  )
}
