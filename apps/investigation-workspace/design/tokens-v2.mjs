// NexusAI design tokens, version 2 (2026-11-03).
//
// Colours are chosen in OKLCH (equal steps look equal), converted to sRGB hex here, and every pair that is
// used together is CHECKED (see `contrastChecks`, asserted by src/v2/styles/tokens.vitest.js and by
// `node design/make-tokens.mjs --check`). Nothing is eyeballed: a palette that fails a pair does not build.
//
// Three themes: light, dark, contrast (high contrast, light based). Components use only the semantic roles
// (`--nx-*`); the primitives below never reach a component.

const clamp01 = value => Math.min(1, Math.max(0, value))

function oklabToLinear(L, a, b) {
  const l_ = L + 0.3963377774 * a + 0.2158037573 * b
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b
  const s_ = L - 0.0894841775 * a - 1.2914855480 * b
  const l = l_ ** 3
  const m = m_ ** 3
  const s = s_ ** 3
  return [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s,
  ]
}

const encode = value => (value <= 0.0031308 ? 12.92 * value : 1.055 * value ** (1 / 2.4) - 0.055)
const decode = value => (value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4)
const inGamut = rgb => rgb.every(channel => channel >= -0.0005 && channel <= 1.0005)

// Reduces chroma until the colour is inside sRGB, so a requested (L, C, H) never clips to a different hue.
export function oklchToHex(L, C, H) {
  const hue = (H * Math.PI) / 180
  let chroma = C
  let rgb = oklabToLinear(L, chroma * Math.cos(hue), chroma * Math.sin(hue))
  for (let i = 0; i < 40 && !inGamut(rgb); i += 1) {
    chroma *= 0.94
    rgb = oklabToLinear(L, chroma * Math.cos(hue), chroma * Math.sin(hue))
  }
  const hex = rgb.map(channel => Math.round(clamp01(encode(clamp01(channel))) * 255).toString(16).padStart(2, '0')).join('')
  return `#${hex}`
}

export function luminance(hex) {
  const [r, g, b] = [1, 3, 5].map(index => decode(parseInt(hex.slice(index, index + 2), 16) / 255))
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

export function contrast(foreground, background) {
  const [light, dark] = [luminance(foreground), luminance(background)].sort((x, y) => y - x)
  return (light + 0.05) / (dark + 0.05)
}

const c = (L, C, H) => oklchToHex(L, C, H)

// ----- primitives ----------------------------------------------------------------------------------------------
const NEUTRAL_HUE = 252
const ACCENT_HUE = 226

// Chart colours (six, in a fixed order: blue, orange, teal, violet, green, magenta). Found by search and checked with the dataviz validator on 2026-11-04 against the real
// surfaces (#feffff, #1e2227): every colour in the lightness band for its theme (OKLCH 0.43-0.77 light, 0.48-0.67 dark), chroma at least 0.10, at least 3:1 on the surface,
// neighbours at least 13.7 apart under red-green colour blindness and 25 apart for full colour vision. `dataSeparation` below re-checks the neighbours on every build,
// so a hand edit that breaks them does not ship. The order is the safety mechanism: assign in this order, never cycle, fold the tail into "Other".
const DATA_LIGHT = ['#337dd3', '#9f4500', '#179a8c', '#9b5ad6', '#5d820b', '#ad5693']
const DATA_DARK = ['#4690e7', '#d05e07', '#089587', '#a463e0', '#76a509', '#b85f9d']

const light = {
  // surfaces: a calm paper-grey canvas, white working surfaces, four distinct tones
  bg: c(0.968, 0.005, NEUTRAL_HUE),
  surface: c(0.999, 0.001, NEUTRAL_HUE),
  sunken: c(0.950, 0.006, NEUTRAL_HUE),
  raised: c(1, 0, 0),
  overlay: c(1, 0, 0),
  hover: c(0.945, 0.006, NEUTRAL_HUE),
  pressed: c(0.92, 0.007, NEUTRAL_HUE),
  borderSubtle: c(0.925, 0.006, NEUTRAL_HUE),
  border: c(0.885, 0.008, NEUTRAL_HUE),
  borderStrong: c(0.63, 0.014, NEUTRAL_HUE),
  text: c(0.21, 0.016, NEUTRAL_HUE),
  text2: c(0.40, 0.016, NEUTRAL_HUE),
  text3: c(0.50, 0.015, NEUTRAL_HUE),
  accent: c(0.47, 0.105, ACCENT_HUE),
  accentHover: c(0.42, 0.10, ACCENT_HUE),
  accentPressed: c(0.37, 0.095, ACCENT_HUE),
  accentSubtle: c(0.962, 0.022, ACCENT_HUE),
  accentSubtle2: c(0.93, 0.04, ACCENT_HUE),
  accentBorder: c(0.80, 0.07, ACCENT_HUE),
  accentText: c(0.41, 0.10, ACCENT_HUE),
  onAccent: c(1, 0, 0),
  focus: c(0.52, 0.13, ACCENT_HUE),
  ok: c(0.50, 0.12, 152), okBg: c(0.965, 0.03, 152), okBorder: c(0.84, 0.07, 152), okText: c(0.39, 0.10, 152),
  warn: c(0.60, 0.13, 72), warnBg: c(0.968, 0.04, 85), warnBorder: c(0.85, 0.09, 80), warnText: c(0.42, 0.09, 65),
  danger: c(0.53, 0.18, 26), dangerBg: c(0.966, 0.025, 25), dangerBorder: c(0.84, 0.08, 25), dangerText: c(0.45, 0.16, 26),
  info: c(0.52, 0.12, 252), infoBg: c(0.964, 0.02, 252), infoBorder: c(0.83, 0.06, 252), infoText: c(0.43, 0.12, 252),
  data: DATA_LIGHT,
  dataMuted: c(0.66, 0.012, NEUTRAL_HUE),
  seq: [0.955, 0.90, 0.83, 0.74, 0.64, 0.53, 0.42].map((L, i) => c(L, 0.02 + i * 0.016, ACCENT_HUE)),
  scrim: 'rgb(16 22 32 / 0.40)',
  shadowRaised: '0 1px 2px rgb(16 22 32 / 0.06), 0 1px 1px rgb(16 22 32 / 0.04)',
  shadowOverlay: '0 12px 32px rgb(16 22 32 / 0.16), 0 2px 6px rgb(16 22 32 / 0.08), 0 0 0 1px rgb(16 22 32 / 0.06)',
  highlight: c(0.93, 0.07, 95),
}

const dark = {
  bg: c(0.208, 0.010, NEUTRAL_HUE),
  surface: c(0.250, 0.011, NEUTRAL_HUE),
  sunken: c(0.188, 0.010, NEUTRAL_HUE),
  raised: c(0.280, 0.012, NEUTRAL_HUE),
  overlay: c(0.305, 0.013, NEUTRAL_HUE),
  hover: c(0.312, 0.013, NEUTRAL_HUE),
  pressed: c(0.342, 0.014, NEUTRAL_HUE),
  borderSubtle: c(0.330, 0.012, NEUTRAL_HUE),
  border: c(0.385, 0.014, NEUTRAL_HUE),
  borderStrong: c(0.59, 0.016, NEUTRAL_HUE),
  text: c(0.94, 0.006, NEUTRAL_HUE),
  text2: c(0.80, 0.010, NEUTRAL_HUE),
  text3: c(0.69, 0.012, NEUTRAL_HUE),
  accent: c(0.70, 0.105, ACCENT_HUE),
  accentHover: c(0.75, 0.10, ACCENT_HUE),
  accentPressed: c(0.66, 0.10, ACCENT_HUE),
  accentSubtle: c(0.265, 0.040, ACCENT_HUE),
  accentSubtle2: c(0.31, 0.055, ACCENT_HUE),
  accentBorder: c(0.46, 0.08, ACCENT_HUE),
  accentText: c(0.80, 0.095, ACCENT_HUE),
  onAccent: c(0.17, 0.02, ACCENT_HUE),
  focus: c(0.76, 0.11, ACCENT_HUE),
  ok: c(0.72, 0.13, 152), okBg: c(0.255, 0.05, 152), okBorder: c(0.42, 0.08, 152), okText: c(0.82, 0.12, 152),
  warn: c(0.80, 0.14, 82), warnBg: c(0.27, 0.05, 80), warnBorder: c(0.46, 0.09, 80), warnText: c(0.85, 0.12, 85),
  danger: c(0.70, 0.17, 25), dangerBg: c(0.26, 0.06, 25), dangerBorder: c(0.46, 0.11, 25), dangerText: c(0.82, 0.10, 25),
  info: c(0.72, 0.11, 252), infoBg: c(0.26, 0.04, 252), infoBorder: c(0.44, 0.08, 252), infoText: c(0.82, 0.09, 252),
  data: DATA_DARK,
  dataMuted: c(0.62, 0.012, NEUTRAL_HUE),
  seq: [0.30, 0.38, 0.46, 0.54, 0.63, 0.72, 0.82].map((L, i) => c(L, 0.04 + i * 0.012, ACCENT_HUE)),
  scrim: 'rgb(0 0 0 / 0.55)',
  shadowRaised: '0 1px 2px rgb(0 0 0 / 0.30)',
  shadowOverlay: '0 16px 40px rgb(0 0 0 / 0.50), 0 2px 8px rgb(0 0 0 / 0.30), 0 0 0 1px rgb(255 255 255 / 0.07)',
  highlight: c(0.40, 0.08, 95),
}

// High contrast: a white page, black text, 2 px black-ish borders, a darker accent. Used for the "Contrast" choice.
const contrastTheme = {
  ...light,
  bg: c(1, 0, 0), surface: c(1, 0, 0), sunken: c(0.96, 0.003, NEUTRAL_HUE), hover: c(0.93, 0.004, NEUTRAL_HUE), pressed: c(0.89, 0.005, NEUTRAL_HUE),
  borderSubtle: c(0.62, 0.01, NEUTRAL_HUE), border: c(0.45, 0.012, NEUTRAL_HUE), borderStrong: c(0.25, 0.012, NEUTRAL_HUE),
  text: c(0.08, 0.01, NEUTRAL_HUE), text2: c(0.22, 0.012, NEUTRAL_HUE), text3: c(0.32, 0.012, NEUTRAL_HUE),
  accent: c(0.36, 0.10, ACCENT_HUE), accentHover: c(0.30, 0.095, ACCENT_HUE), accentPressed: c(0.25, 0.09, ACCENT_HUE), accentText: c(0.32, 0.10, ACCENT_HUE),
  accentSubtle: c(0.95, 0.03, ACCENT_HUE), accentSubtle2: c(0.90, 0.05, ACCENT_HUE), accentBorder: c(0.40, 0.09, ACCENT_HUE), focus: c(0.30, 0.12, ACCENT_HUE),
  okText: c(0.30, 0.10, 152), warnText: c(0.33, 0.08, 65), dangerText: c(0.36, 0.15, 26), infoText: c(0.34, 0.12, 252),
  okBorder: c(0.50, 0.10, 152), warnBorder: c(0.50, 0.10, 80), dangerBorder: c(0.50, 0.14, 25), infoBorder: c(0.50, 0.10, 252),
}

export const themes = { light, dark, contrast: contrastTheme }

// ----- the sidebar: a tone of its own, so the frame and the work are told apart at a glance ----------------------------------
// Two chromes per theme. "ink" (the default) is a deep ink sidebar against the lighter working area in light mode, and the deepest tone in dark mode;
// "soft" is a tinted neutral one step off the canvas. The analyst chooses in Appearance. The contrast theme uses one light chrome for both.
const chromeSet = (bg, border, text, text2, text3, hover, active, activeText, accent, focus, caseBg, ok, warn, danger, attention) => ({ bg, border, text, text2, text3, hover, active, activeText, accent, focus, caseBg, ok, warn, danger, attention })
const inkMarks = [c(0.76, 0.13, 152), c(0.82, 0.14, 82), c(0.72, 0.17, 25), c(0.8, 0.11, 25)]
const contrastChrome = chromeSet(c(1, 0, 0), c(0.25, 0.012, 252), c(0.08, 0.01, 252), c(0.2, 0.012, 252), c(0.3, 0.012, 252), c(0.92, 0.004, 252), c(0.9, 0.05, 226), c(0.2, 0.1, 226), c(0.36, 0.1, 226), c(0.3, 0.12, 226), c(0.97, 0.003, 252), c(0.4, 0.12, 152), c(0.45, 0.12, 72), c(0.42, 0.17, 26), c(0.36, 0.15, 26))
export const chromes = {
  light: {
    ink: chromeSet(c(0.205, 0.024, 252), c(0.285, 0.021, 252), c(0.95, 0.008, 252), c(0.81, 0.012, 252), c(0.71, 0.014, 252), c(0.265, 0.025, 252), c(0.35, 0.065, 226), c(0.97, 0.02, 226), c(0.76, 0.11, 226), c(0.8, 0.11, 226), c(0.24, 0.025, 252), ...inkMarks),
    soft: chromeSet(c(0.935, 0.006, 252), c(0.89, 0.008, 252), c(0.21, 0.016, 252), c(0.38, 0.016, 252), c(0.47, 0.015, 252), c(0.905, 0.008, 252), c(0.9, 0.045, 226), c(0.36, 0.1, 226), c(0.47, 0.105, 226), c(0.5, 0.12, 226), c(0.985, 0.002, 252), c(0.5, 0.12, 152), c(0.6, 0.13, 72), c(0.53, 0.18, 26), c(0.45, 0.16, 26)),
  },
  dark: {
    ink: chromeSet(c(0.128, 0.011, 252), c(0.225, 0.013, 252), c(0.94, 0.006, 252), c(0.79, 0.01, 252), c(0.68, 0.012, 252), c(0.19, 0.013, 252), c(0.30, 0.062, 226), c(0.96, 0.02, 226), c(0.76, 0.11, 226), c(0.8, 0.11, 226), c(0.168, 0.012, 252), ...inkMarks),
    soft: chromeSet(c(0.168, 0.01, 252), c(0.265, 0.012, 252), c(0.94, 0.006, 252), c(0.79, 0.01, 252), c(0.68, 0.012, 252), c(0.225, 0.012, 252), c(0.31, 0.055, 226), c(0.96, 0.02, 226), c(0.74, 0.105, 226), c(0.78, 0.11, 226), c(0.205, 0.011, 252), c(0.72, 0.13, 152), c(0.8, 0.14, 82), c(0.7, 0.17, 25), c(0.82, 0.1, 25)),
  },
  contrast: { ink: contrastChrome, soft: contrastChrome },
}

export function checkChrome(name) {
  const failures = []
  for (const [variant, ch] of Object.entries(chromes[name])) {
    const pairs = [
      ['text', 'bg', 4.5], ['text2', 'bg', 4.5], ['text3', 'bg', 4.5], ['text', 'hover', 4.5], ['text2', 'hover', 4.5], ['activeText', 'active', 4.5],
      ['text', 'caseBg', 4.5], ['text3', 'caseBg', 4.5], ['accent', 'bg', 3], ['focus', 'bg', 3],
      ['ok', 'bg', 3], ['warn', 'bg', 3], ['danger', 'bg', 3], ['attention', 'bg', 4.5],
    ]
    for (const [fg, bg, minimum] of pairs) {
      const ratio = contrast(ch[fg], ch[bg])
      if (ratio < minimum) failures.push({ theme: `${name}/${variant}`, what: `sidebar ${fg} on ${bg}`, fg, bg, ratio: Number(ratio.toFixed(2)), minimum })
    }
  }
  return failures
}

// ----- the pairs that are used together and the ratio each must reach (WCAG 2.2) -------------------------------
// [foreground role, background role, minimum ratio, what it is]
const TEXT = 4.5
const UI = 3
export const contrastChecks = [
  ['text', 'bg', TEXT, 'body text on the canvas'], ['text', 'surface', TEXT, 'body text on a surface'], ['text', 'sunken', TEXT, 'body text on a sunken surface'],
  ['text', 'overlay', TEXT, 'body text on a menu'], ['text', 'hover', TEXT, 'body text on a hovered row'],
  ['text2', 'bg', TEXT, 'secondary text on the canvas'], ['text2', 'surface', TEXT, 'secondary text on a surface'], ['text2', 'sunken', TEXT, 'secondary text on a sunken surface'], ['text2', 'hover', TEXT, 'secondary text on a hovered row'],
  ['text3', 'bg', TEXT, 'muted text on the canvas'], ['text3', 'surface', TEXT, 'muted text on a surface'], ['text3', 'sunken', TEXT, 'muted text on a sunken surface'], ['text3', 'hover', TEXT, 'muted text on a hovered row'],
  ['accentText', 'surface', TEXT, 'accent text on a surface'], ['accentText', 'bg', TEXT, 'accent text on the canvas'], ['accentText', 'accentSubtle', TEXT, 'accent text on its tint'], ['accentText', 'accentSubtle2', TEXT, 'accent text on its stronger tint'],
  ['onAccent', 'accent', TEXT, 'text on the solid accent'], ['onAccent', 'accentHover', TEXT, 'text on the hovered accent'], ['onAccent', 'accentPressed', TEXT, 'text on the pressed accent'],
  ['okText', 'okBg', TEXT, 'success text on its tint'], ['warnText', 'warnBg', TEXT, 'warning text on its tint'], ['dangerText', 'dangerBg', TEXT, 'error text on its tint'], ['infoText', 'infoBg', TEXT, 'information text on its tint'],
  ['okText', 'surface', TEXT, 'success text on a surface'], ['warnText', 'surface', TEXT, 'warning text on a surface'], ['dangerText', 'surface', TEXT, 'error text on a surface'], ['infoText', 'surface', TEXT, 'information text on a surface'],
  ['borderStrong', 'surface', UI, 'control border on a surface'], ['borderStrong', 'bg', UI, 'control border on the canvas'],
  ['focus', 'surface', UI, 'focus ring on a surface'], ['focus', 'bg', UI, 'focus ring on the canvas'], ['focus', 'sunken', UI, 'focus ring on a sunken surface'],
  ['accent', 'surface', UI, 'accent mark on a surface'], ['ok', 'surface', UI, 'success mark on a surface'], ['warn', 'surface', UI, 'warning mark on a surface'], ['danger', 'surface', UI, 'error mark on a surface'], ['info', 'surface', UI, 'information mark on a surface'],
]

export function checkTheme(name) {
  const theme = themes[name]
  const failures = []
  for (const [fg, bg, minimum, what] of contrastChecks) {
    const ratio = contrast(theme[fg], theme[bg])
    if (ratio < minimum) failures.push({ theme: name, what, fg, bg, ratio: Number(ratio.toFixed(2)), minimum })
  }
  theme.data.forEach((mark, index) => {
    const ratio = contrast(mark, theme.surface)
    if (ratio < UI) failures.push({ theme: name, what: `data colour ${index + 1} on a surface`, fg: `data${index + 1}`, bg: 'surface', ratio: Number(ratio.toFixed(2)), minimum: UI })
  })
  const { cvd, normal } = dataSeparation(name)
  if (cvd < 8) failures.push({ theme: name, what: 'neighbouring data colours under red-green colour blindness', fg: 'data', bg: 'data', ratio: Number(cvd.toFixed(1)), minimum: 8 })
  if (normal < 15) failures.push({ theme: name, what: 'neighbouring data colours with full colour vision', fg: 'data', bg: 'data', ratio: Number(normal.toFixed(1)), minimum: 15 })
  return [...failures, ...checkChrome(name)]
}

// Neighbouring categorical colours must differ in lightness as well as hue, so they survive a greyscale or colour-blind reading.
export function dataLightnessGaps(name) {
  const lum = themes[name].data.map(luminance)
  return lum.slice(1).map((value, index) => Math.abs(value - lum[index]))
}

// ... and they must stay apart for readers with red-green colour blindness. Only neighbours touch (in a stack, a bar list, a line chart), so only neighbours are measured:
// Euclidean distance in OKLab x100, under protanopia and deuteranopia (Machado, Oliveira and Fernandes 2009, severity 1) and under full colour vision.
// The method asks for at least 8 under colour blindness and 15 for everyone else.
const MACHADO = {
  protan: [[0.152286, 1.052583, -0.204868], [0.114503, 0.786281, 0.099216], [-0.003882, -0.048116, 1.051998]],
  deutan: [[0.367322, 0.860646, -0.227968], [0.280085, 0.672501, 0.047413], [-0.011820, 0.042940, 0.968881]],
}
const toLinear = hex => [1, 3, 5].map(index => decode(parseInt(hex.slice(index, index + 2), 16) / 255))
const simulate = (rgb, kind) => MACHADO[kind].map(row => clamp01(row[0] * rgb[0] + row[1] * rgb[1] + row[2] * rgb[2]))

function toOklab([r, g, b]) {
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b)
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b)
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b)
  return [0.2104542553 * l + 0.7936177850 * m - 0.0040720468 * s, 1.9779984951 * l - 2.4285922050 * m + 0.4505937099 * s, 0.0259040371 * l + 0.7827717662 * m - 0.8086757660 * s]
}

const apart = (a, b) => 100 * Math.hypot(a[0] - b[0], a[1] - b[1], a[2] - b[2])

export function dataSeparation(name) {
  const colours = themes[name].data.map(toLinear)
  let cvd = Infinity
  let normal = Infinity
  for (let index = 0; index < colours.length - 1; index += 1) {
    normal = Math.min(normal, apart(toOklab(colours[index]), toOklab(colours[index + 1])))
    for (const kind of ['protan', 'deutan']) cvd = Math.min(cvd, apart(toOklab(simulate(colours[index], kind)), toOklab(simulate(colours[index + 1], kind))))
  }
  return { cvd, normal }
}

// ----- CSS output ------------------------------------------------------------------------------------------------
const roleToVar = {
  bg: 'bg', surface: 'surface', sunken: 'surface-sunken', raised: 'surface-raised', overlay: 'surface-overlay',
  hover: 'hover', pressed: 'pressed', borderSubtle: 'border-subtle', border: 'border', borderStrong: 'border-strong',
  text: 'text', text2: 'text-2', text3: 'text-3', accent: 'accent', accentHover: 'accent-hover', accentPressed: 'accent-pressed',
  accentSubtle: 'accent-subtle', accentSubtle2: 'accent-subtle-2', accentBorder: 'accent-border', accentText: 'accent-text', onAccent: 'on-accent',
  focus: 'focus', ok: 'ok', okBg: 'ok-bg', okBorder: 'ok-border', okText: 'ok-text', warn: 'warn', warnBg: 'warn-bg', warnBorder: 'warn-border', warnText: 'warn-text',
  danger: 'danger', dangerBg: 'danger-bg', dangerBorder: 'danger-border', dangerText: 'danger-text', info: 'info', infoBg: 'info-bg', infoBorder: 'info-border', infoText: 'info-text',
  dataMuted: 'data-muted', scrim: 'scrim', shadowRaised: 'shadow-raised', shadowOverlay: 'shadow-overlay', highlight: 'highlight',
}

function declarations(theme) {
  const lines = Object.entries(roleToVar).map(([role, name]) => `  --nx-${name}: ${theme[role]};`)
  theme.data.forEach((value, index) => lines.push(`  --nx-data-${index + 1}: ${value};`))
  theme.seq.forEach((value, index) => lines.push(`  --nx-seq-${index + 1}: ${value};`))
  // evidence strength: a word and a shape carry the meaning, the colour only supports it
  lines.push(`  --nx-strength-observed: ${theme.text};`, `  --nx-strength-inferred: ${theme.accent};`, `  --nx-strength-candidate: ${theme.warn};`, `  --nx-strength-confirmed: ${theme.ok};`)
  return lines.join('\n')
}

const chromeVars = { bg: 'side-bg', border: 'side-border', text: 'side-text', text2: 'side-text-2', text3: 'side-text-3', hover: 'side-hover', active: 'side-active', activeText: 'side-active-text', accent: 'side-accent', focus: 'side-focus', caseBg: 'side-case-bg', ok: 'side-ok', warn: 'side-warn', danger: 'side-danger', attention: 'side-attention' }
function chromeDeclarations(name, variant) {
  return Object.entries(chromeVars).map(([key, variable]) => `  --nx-${variable}: ${chromes[name][variant][key]};`).join('\n')
}

export function buildCss() {
  return `/* GENERATED by design/make-tokens.mjs from design/tokens-v2.mjs. Do not edit; change the generator and run \`node design/make-tokens.mjs\`. */
@layer nx-tokens {
  :root,
  [data-nx-theme='light'] {
    color-scheme: light;
${declarations(themes.light)}
${chromeDeclarations('light', 'ink')}
  }
  :root[data-chrome='soft'] {
${chromeDeclarations('light', 'soft')}
  }
  @media (prefers-color-scheme: dark) {
    :root:not([data-theme='light']):not([data-theme='contrast']) {
      color-scheme: dark;
${declarations(themes.dark).replace(/^/gm, '    ')}
${chromeDeclarations('dark', 'ink').replace(/^/gm, '    ')}
    }
    :root:not([data-theme='light']):not([data-theme='contrast'])[data-chrome='soft'] {
${chromeDeclarations('dark', 'soft').replace(/^/gm, '    ')}
    }
  }
  :root[data-theme='dark'],
  [data-nx-theme='dark'] {
    color-scheme: dark;
${declarations(themes.dark)}
${chromeDeclarations('dark', 'ink')}
  }
  :root[data-theme='dark'][data-chrome='soft'] {
${chromeDeclarations('dark', 'soft')}
  }
  :root[data-theme='contrast'],
  [data-nx-theme='contrast'] {
    color-scheme: light;
${declarations(themes.contrast)}
${chromeDeclarations('contrast', 'ink')}
  }
}
`
}
