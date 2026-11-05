// Chart colours are read from the live tokens (`--nx-*`), so a chart follows the theme, the high-contrast choice and the density without a second palette.
export function readChartTheme() {
  const root = globalThis.document?.documentElement
  const css = root ? globalThis.getComputedStyle(root) : null
  const v = (name, fallback = '') => (css ? css.getPropertyValue(name).trim() || fallback : fallback)
  const reduce = root?.getAttribute('data-motion') === 'reduce' || Boolean(globalThis.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches)
  return {
    text: v('--nx-text', '#111'), text2: v('--nx-text-2', '#444'), text3: v('--nx-text-3', '#666'),
    line: v('--nx-border', '#ddd'), lineSubtle: v('--nx-border-subtle', '#eee'), overlay: v('--nx-surface-overlay', '#fff'),
    accent: v('--nx-accent', '#096480'), accentSubtle: v('--nx-accent-subtle-2', '#d7edf5'),
    data: [1, 2, 3, 4, 5, 6].map(index => v(`--nx-data-${index}`, '#4466aa')), muted: v('--nx-data-muted', '#999'),
    font: v('--nx-font-ui', 'sans-serif'),
    reduceMotion: reduce,
  }
}
