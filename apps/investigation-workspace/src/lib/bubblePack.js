// Deterministic circle packing for the evidence map. Circle area is proportional to the value (radius to its square
// root), so the picture reads at a glance while the exact counts stay on the labels and in the table. No randomness:
// the same data always gives the same layout, so the map does not shuffle between refreshes.
const MIN_RADIUS = 18
const PADDING = 5

function place(items, width, height, largest) {
  const max = items[0].value
  const centre = { x: width / 2, y: height / 2 }
  const placed = []
  for (const item of items) {
    const radius = Math.max(MIN_RADIUS, largest * Math.sqrt(item.value / max))
    let spot = null
    for (let distance = 0; distance <= Math.hypot(width, height) && !spot; distance += 4) {
      const steps = Math.max(1, Math.round((distance * Math.PI * 2) / 10))
      for (let step = 0; step < steps && !spot; step += 1) {
        const angle = (step / steps) * Math.PI * 2
        const x = centre.x + distance * Math.cos(angle) * 1.35
        const y = centre.y + distance * Math.sin(angle)
        const inside = x - radius >= PADDING && x + radius <= width - PADDING && y - radius >= PADDING && y + radius <= height - PADDING
        if (inside && placed.every(other => Math.hypot(x - other.x, y - other.y) >= radius + other.r + PADDING)) spot = { x, y }
      }
    }
    if (!spot) return null
    placed.push({ id: item.id, value: item.value, x: spot.x, y: spot.y, r: radius })
  }
  return placed
}

export function packCircles(items, width, height) {
  const positive = items.filter(item => Number(item.value) > 0).map(item => ({ ...item, value: Number(item.value) })).sort((left, right) => right.value - left.value || String(left.id).localeCompare(String(right.id)))
  if (!positive.length) return []
  let largest = Math.min(width, height) * 0.46
  for (let attempt = 0; attempt < 60; attempt += 1) {
    const placed = place(positive, width, height, largest)
    if (placed) return placed
    largest *= 0.93
  }
  return []
}
