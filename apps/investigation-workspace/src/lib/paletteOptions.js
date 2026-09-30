import { contrastRatio, mixHex } from './contrast.js'

// Three candidate shell identities, all derived from the header gradient
// (navy #111827 to deep teal #123b43 in light, #070b12 to #10343a in dark).
// Values here are proposals for owner review; nothing consumes them outside the gallery yet.

const header = {
  light: { stops: ['#111827', '#123b43'], text: '#f8fafc', muted: '#c5d2df', line: '#70839a' },
  dark: { stops: ['#070b12', '#10343a'], text: '#f8fafc', muted: '#c5d2df', line: '#70839a' },
}

// One categorical set for every page and option: Okabe-Ito hues, darkened where a light card needs 3:1 for marks.
const data = {
  light: ['#0072b2', '#c2410c', '#00795f', '#7e3fb2', '#8a6100', '#475569'],
  dark: ['#56b4e9', '#e69f00', '#35c5a0', '#cc79a7', '#f0e442', '#b4c0cc'],
}

const status = {
  light: { ready: '#166534', processing: '#854d0e', failed: '#a1122f', withheld: '#475569' },
  dark: { ready: '#79d7a5', processing: '#f6c76b', failed: '#ff9aaa', withheld: '#c6d0df' },
}

export const paletteOptions = [
  {
    id: 'A',
    name: 'Continuous dark chrome',
    summary: 'The header gradient runs down the sidebar. A cool teal-tinted canvas carries raised near-white cards, and the active route is marked in the teal accent.',
    modes: {
      light: {
        header: header.light,
        rail: { stops: ['#111827', '#123b43'], text: '#e6edf3', muted: '#a9bcc8', section: '#a9bcc8', activeBg: '#1d4c56', activeText: '#ffffff', marker: '#5eead4', focus: '#7dd3fc' },
        canvas: '#e8f0f1', card: '#fbfdfd', raised: '#ffffff', border: '#c3d3d7', borderStrong: '#5b7078',
        text: '#111827', muted: '#3f5262', accent: '#0e7490', accentOn: '#ffffff', focus: '#0b5f78',
        status: status.light, data: data.light,
      },
      dark: {
        header: header.dark,
        rail: { stops: ['#070b12', '#10343a'], text: '#e6edf3', muted: '#a9bcc8', section: '#a9bcc8', activeBg: '#17434c', activeText: '#ffffff', marker: '#5eead4', focus: '#7dd3fc' },
        canvas: '#0a1519', card: '#111f25', raised: '#182a31', border: '#2b444d', borderStrong: '#86a0aa',
        text: '#f1f5f9', muted: '#a9bac5', accent: '#4fd1e5', accentOn: '#06222a', focus: '#67e0f2',
        status: status.dark, data: data.dark,
      },
    },
  },
  {
    id: 'B',
    name: 'Light rail, navy anchors',
    summary: 'The sidebar is a light surface tinted from the header hue. Navy carries section titles and the active route; the canvas sits one step darker than the cards.',
    modes: {
      light: {
        header: header.light,
        rail: { stops: ['#e3ecee'], text: '#1b2838', muted: '#43566a', section: '#111827', activeBg: '#111827', activeText: '#ffffff', marker: '#0e7490', focus: '#0b5f78' },
        canvas: '#d8e4e7', card: '#f7fafb', raised: '#ffffff', border: '#b4c7cc', borderStrong: '#55707a',
        text: '#111827', muted: '#3c4f60', accent: '#0b6680', accentOn: '#ffffff', focus: '#0b5f78',
        status: status.light, data: data.light,
      },
      dark: {
        header: header.dark,
        rail: { stops: ['#0d1b21'], text: '#e6edf3', muted: '#a9bcc8', section: '#d3e3ea', activeBg: '#1b3b45', activeText: '#ffffff', marker: '#5eead4', focus: '#7dd3fc' },
        canvas: '#081115', card: '#10202a', raised: '#172b36', border: '#2a434e', borderStrong: '#86a0aa',
        text: '#f1f5f9', muted: '#a9bac5', accent: '#4fd1e5', accentOn: '#06222a', focus: '#67e0f2',
        status: status.dark, data: data.dark,
      },
    },
  },
  {
    id: 'C',
    name: 'Flat workspace, hairline layers',
    summary: 'Sidebar and canvas share one tone and are separated by hairlines. Depth comes from four distinct elevation steps, not from colour blocks. Dark-first.',
    modes: {
      light: {
        header: header.light,
        rail: { stops: ['#f1f6f7'], text: '#1b2838', muted: '#43566a', section: '#43566a', activeBg: '#d9eef2', activeText: '#0a4655', marker: '#0e7490', focus: '#0b5f78' },
        canvas: '#f1f6f7', card: '#ffffff', raised: '#ffffff', border: '#cddadd', borderStrong: '#5b7078',
        text: '#111827', muted: '#3f5262', accent: '#0e7490', accentOn: '#ffffff', focus: '#0b5f78',
        status: status.light, data: data.light,
      },
      dark: {
        header: header.dark,
        rail: { stops: ['#0a1216'], text: '#e6edf3', muted: '#a9bcc8', section: '#a9bcc8', activeBg: '#14303a', activeText: '#ffffff', marker: '#4fd1e5', focus: '#7dd3fc' },
        canvas: '#0a1216', card: '#101b21', raised: '#16262d', border: '#233841', borderStrong: '#86a0aa',
        text: '#f1f5f9', muted: '#a9bac5', accent: '#4fd1e5', accentOn: '#06222a', focus: '#67e0f2',
        status: status.dark, data: data.dark,
      },
    },
  },
]

// Backgrounds a gradient passes through: both stops plus the midpoint, since text must hold everywhere.
function gradientSamples(stops) {
  return stops.length > 1 ? [stops[0], mixHex(stops[0], stops[1], 0.5), stops[1]] : stops
}

function worst(foreground, backgrounds) {
  return Math.min(...backgrounds.map(background => contrastRatio(foreground, background)))
}

export function measurePalette(mode) {
  const headerBackgrounds = gradientSamples(mode.header.stops)
  const railBackgrounds = gradientSamples(mode.rail.stops)
  const checks = [
    ['Header text on the header gradient', worst(mode.header.text, headerBackgrounds), 4.5],
    ['Header secondary text on the header gradient', worst(mode.header.muted, headerBackgrounds), 4.5],
    ['Header control outline on the header gradient', worst(mode.header.line, headerBackgrounds), 3],
    ['Sidebar link text on the sidebar', worst(mode.rail.text, railBackgrounds), 4.5],
    ['Sidebar secondary text on the sidebar', worst(mode.rail.muted, railBackgrounds), 4.5],
    ['Sidebar section title on the sidebar', worst(mode.rail.section, railBackgrounds), 4.5],
    ['Active route text on its highlight', contrastRatio(mode.rail.activeText, mode.rail.activeBg), 4.5],
    ['Active route marker on the sidebar', worst(mode.rail.marker, railBackgrounds), 3],
    ['Active route marker on its highlight', contrastRatio(mode.rail.marker, mode.rail.activeBg), 3],
    ['Sidebar focus ring on the sidebar', worst(mode.rail.focus, railBackgrounds), 3],
    ['Body text on the canvas', contrastRatio(mode.text, mode.canvas), 4.5],
    ['Body text on a card', contrastRatio(mode.text, mode.card), 4.5],
    ['Secondary text on the canvas', contrastRatio(mode.muted, mode.canvas), 4.5],
    ['Secondary text on a card', contrastRatio(mode.muted, mode.card), 4.5],
    ['Link and accent text on a card', contrastRatio(mode.accent, mode.card), 4.5],
    ['Link and accent text on the canvas', contrastRatio(mode.accent, mode.canvas), 4.5],
    ['Button label on the accent fill', contrastRatio(mode.accentOn, mode.accent), 4.5],
    ['Control outline on a card', contrastRatio(mode.borderStrong, mode.card), 3],
    ['Focus ring on a card', contrastRatio(mode.focus, mode.card), 3],
    ['Focus ring on the canvas', contrastRatio(mode.focus, mode.canvas), 3],
    ...Object.entries(mode.status).map(([name, colour]) => [`${name[0].toUpperCase()}${name.slice(1)} status text on a card`, contrastRatio(colour, mode.card), 4.5]),
    ...mode.data.map((colour, index) => [`Chart series ${index + 1} on a card`, contrastRatio(colour, mode.card), 3]),
  ]
  return checks.map(([label, ratio, required]) => ({ label, ratio, required, pass: ratio >= required }))
}
