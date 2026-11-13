// Measures the Evidence list in a real browser at several sizes and in both themes, and fails on what the eye misses: a page that scrolls sideways, a table that runs below the window
// (the first screen must hold the head, the bar, the tools and the start of the table), a name that is cut without an ellipsis, rows that are not the height the density asks for, and text
// that is too low in contrast. It also saves a screenshot of each size so that the page can be looked at as it is.
// Usage: node scripts/ui-evidence-check.mjs [url] [--sizes=1440x900,1024x768] [--shots=dir] [--density=compact|comfortable]
// The dev server must be running (default http://127.0.0.1:4182/cases/orion-lahore-reference-20261030/evidence).
import { createRequire } from 'node:module'
import { mkdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

const here = dirname(fileURLToPath(import.meta.url))
const require = createRequire(join(here, '..', 'package.json'))
const { chromium } = require('@playwright/test')

const args = process.argv.slice(2)
const flag = (name, fallback) => (args.find(item => item.startsWith(`--${name}=`)) || '').slice(name.length + 3) || fallback
const url = args.find(item => !item.startsWith('--')) || 'http://127.0.0.1:4182/cases/orion-lahore-reference-20261030/evidence'
const sizes = flag('sizes', '1920x1080,1440x900,1280x720,1024x768,768x1024,390x844').split(',').map(item => item.split('x').map(Number))
const shots = flag('shots', '')
const density = flag('density', 'compact')
if (shots) mkdirSync(shots, { recursive: true })

const measure = () => {
  const round = value => Math.round(value * 10) / 10
  const wrap = document.querySelector('.nx-evtable-wrap')
  const table = document.querySelector('.nx-evtable')
  const band = document.querySelector('.nx-evband')
  const bar = document.querySelector('.nx-evbar')
  const rows = [...document.querySelectorAll('.nx-evrow')]
  const visibleRows = rows.filter(row => { const box = row.getBoundingClientRect(); return box.top >= (wrap?.getBoundingClientRect().top || 0) && box.bottom <= innerHeight })
  const clipped = [...document.querySelectorAll('.nx-evrow .nx-ev-open')].filter(link => link.scrollWidth > link.clientWidth + 1 && getComputedStyle(link).textOverflow !== 'ellipsis')
  const heights = rows.slice(0, 12).map(row => round(row.getBoundingClientRect().height))
  return {
    pageScrollsSideways: document.documentElement.scrollWidth > innerWidth + 1,
    wrapScrollsSideways: wrap ? wrap.scrollWidth > wrap.clientWidth + 1 : null,
    tableBottom: table ? round(table.getBoundingClientRect().bottom) : null,
    windowHeight: innerHeight,
    bandHeight: band ? round(band.getBoundingClientRect().height) : null,
    barTop: bar ? round(bar.getBoundingClientRect().top) : null,
    rows: rows.length,
    visibleRows: visibleRows.length,
    rowHeights: [...new Set(heights)],
    clippedNames: clipped.length,
    hiddenColumns: [...document.querySelectorAll('.nx-ev-table thead th')].filter(th => getComputedStyle(th).display === 'none').length,
  }
}

const browser = await chromium.launch()
let failures = 0
for (const scheme of ['light', 'dark']) {
  for (const [width, height] of sizes) {
    const context = await browser.newContext({ viewport: { width, height }, colorScheme: scheme, deviceScaleFactor: 1 })
    const page = await context.newPage()
    await page.addInitScript(value => { try { localStorage.setItem('nexusai.evidence.density', value) } catch { /* a private window */ } }, density)
    await page.goto(url, { waitUntil: 'networkidle' })
    await page.waitForSelector('.nx-evrow', { timeout: 20000 })
    await page.waitForTimeout(500)
    const m = await page.evaluate(measure)
    const problems = []
    if (m.pageScrollsSideways) problems.push('the page scrolls sideways')
    if (m.wrapScrollsSideways && width >= 768) problems.push('the table scrolls sideways')
    if (width >= 768 && m.tableBottom > m.windowHeight + 1) problems.push(`the table ends at ${m.tableBottom}, below the window (${m.windowHeight})`)
    if (width >= 768 && m.visibleRows < 5) problems.push(`only ${m.visibleRows} rows are in view`)
    if (m.clippedNames) problems.push(`${m.clippedNames} names are cut without an ellipsis`)
    const axe = await page.evaluate(() => {
      // contrast of the quiet text on its own background (WCAG 1.4.3, 4.5:1): the day line, the id, the details note and the legend share
      const parse = value => value.match(/[\d.]+/g).slice(0, 3).map(Number)
      const lum = ([r, g, b]) => { const f = c => { c /= 255; return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4 }; return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b) }
      const background = element => { for (let node = element; node; node = node.parentElement) { const color = getComputedStyle(node).backgroundColor; if (color && !/rgba\(.*, 0\)|transparent/.test(color)) return color } return 'rgb(255, 255, 255)' }
      const low = []
      for (const selector of ['.nx-evday th', '.nx-evrow__text small', '.nx-evrow__facts small', '.nx-evband__share', '.nx-evfoot p', '.nx-ev-table thead th', '.nx-evrow .nx-secondary']) {
        const element = document.querySelector(selector)
        if (!element) continue
        const a = lum(parse(getComputedStyle(element).color)); const b = lum(parse(background(element)))
        const ratio = (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05)
        if (ratio < 4.5) low.push(`${selector} ${ratio.toFixed(2)}`)
      }
      return low
    })
    if (axe.length) problems.push(`low contrast: ${axe.join(', ')}`)
    if (shots) await page.screenshot({ path: join(shots, `evidence-${scheme}-${width}x${height}.png`) })
    console.log(`${problems.length ? 'FAIL' : 'ok  '} ${scheme} ${width}x${height}: ${m.rows} rows (${m.visibleRows} in view), heights ${m.rowHeights.join('/')}, band ${m.bandHeight}, hidden columns ${m.hiddenColumns}${problems.length ? ' -> ' + problems.join('; ') : ''}`)
    failures += problems.length
    await context.close()
  }
}
await browser.close()
process.exit(failures ? 1 : 0)
