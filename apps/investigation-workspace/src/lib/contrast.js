// WCAG 2.x relative-luminance contrast, used to measure palette candidates on
// their real backgrounds (gradient stops included) rather than assert them.

function channels(hex) {
  return hex.slice(1).match(/../g).map(value => parseInt(value, 16) / 255)
}

export function luminance(hex) {
  const [red, green, blue] = channels(hex).map(value => (value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4))
  return (0.2126 * red) + (0.7152 * green) + (0.0722 * blue)
}

export function contrastRatio(foreground, background) {
  const first = luminance(foreground)
  const second = luminance(background)
  return (Math.max(first, second) + 0.05) / (Math.min(first, second) + 0.05)
}

// Linear blend in sRGB, matching how a CSS linear-gradient interpolates between two stops.
export function mixHex(from, to, amount) {
  const start = channels(from)
  const end = channels(to)
  return `#${start.map((value, index) => Math.round((value + ((end[index] - value) * amount)) * 255).toString(16).padStart(2, '0')).join('')}`
}
