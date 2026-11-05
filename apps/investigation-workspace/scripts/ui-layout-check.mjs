// Measures the dashboard in a real browser and fails on what the eye misses: panels of one band that do not share a top and a bottom, content that overflows its panel, a page that scrolls sideways,
// and a first screen that does not hold the hero. Usage: node scripts/ui-layout-check.mjs [url] [--sizes=1440x900,1280x720] [--scheme=light|dark] [--shots=dir]
// The dev server must be running (default http://127.0.0.1:4182/). Needs `@playwright/test` (already a dev dependency).
import { createRequire } from 'node:module'
import { mkdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

const here = dirname(fileURLToPath(import.meta.url))
const require = createRequire(join(here, '..', 'package.json'))
const { chromium } = require('@playwright/test')

const args = process.argv.slice(2)
const flag = (name, fallback) => (args.find(item => item.startsWith(`--${name}=`)) || '').slice(name.length + 3) || fallback
const url = args.find(item => !item.startsWith('--')) || 'http://127.0.0.1:4182/'
const sizes = flag('sizes', '1920x1080,1440x900,1280x720,1024x768,768x1024,390x844').split(',').map(item => item.split('x').map(Number))
const scheme = flag('scheme', 'light')
const shots = flag('shots', '')
const withHistory = flag('history', '1') !== '0'
if (shots) mkdirSync(shots, { recursive: true })

const measure = () => {
  const main = document.querySelector('.nx-main')
  const round = value => Math.round(value * 10) / 10
  const panels = [...document.querySelectorAll('.nx-panel')].map(panel => {
    const box = panel.getBoundingClientRect()
    const body = panel.querySelector('.nx-panel__body')
    const title = panel.querySelector('.nx-panel__title')?.textContent?.trim() || '(untitled)'
    return {
      title,
      band: panel.closest('[data-band]')?.getAttribute('data-band') || '',
      top: round(box.top - main.getBoundingClientRect().top + main.scrollTop),
      bottom: round(box.bottom - main.getBoundingClientRect().top + main.scrollTop),
      left: round(box.left), right: round(box.right), height: round(box.height), width: round(box.width),
      overflowY: body ? body.scrollHeight - body.clientHeight : 0,
      overflowX: body ? body.scrollWidth - body.clientWidth : 0,
      squashed: body ? [...body.querySelectorAll('*')].filter(el => !el.closest('svg') && !/^(TABLE|THEAD|TBODY|TR|TD|TH|CAPTION)$/.test(el.tagName) && el.clientHeight > 0 && el.scrollHeight - el.clientHeight > 2 && getComputedStyle(el).overflowY === 'visible').slice(0, 3).map(el => (el.className && el.className.baseVal === undefined ? '.' + String(el.className).split(' ')[0] : el.tagName.toLowerCase()) + ' (' + (el.scrollHeight - el.clientHeight) + 'px)') : [],
      titleLines: Math.round((panel.querySelector('.nx-panel__title')?.getBoundingClientRect().height || 0) / 17),
      headTop: round(panel.querySelector('.nx-panel__title')?.getBoundingClientRect().top - box.top),
      slack: (() => { if (!body) return 0; const inner = body.getBoundingClientRect().bottom - parseFloat(getComputedStyle(body).paddingBottom); const used = Math.max(0, ...[...body.children].filter(child => getComputedStyle(child).position !== 'absolute').map(child => child.getBoundingClientRect().bottom)); return round(inner - used) })(),
      visibleAtLoad: box.bottom <= main.getBoundingClientRect().bottom + 0.5 && box.top >= main.getBoundingClientRect().top - 0.5,
    }
  })
  const tiles = [...document.querySelectorAll('.nx-tile')].map(tile => { const box = tile.getBoundingClientRect(); return { top: round(box.top), height: round(box.height) } })
  return {
    pageOverflowX: main ? main.scrollWidth - main.clientWidth : 0,
    wide: [...document.querySelectorAll('.nx-main *')].filter(el => el.getBoundingClientRect().right > main.getBoundingClientRect().right + 1 && getComputedStyle(el).position !== 'fixed').slice(0, 4).map(el => (el.className && el.className.baseVal === undefined ? '.' + String(el.className).split(' ')[0] : el.tagName.toLowerCase()) + ' +' + Math.round(el.getBoundingClientRect().right - main.getBoundingClientRect().right) + 'px'),
    viewport: { w: window.innerWidth, h: window.innerHeight },
    mainBox: (() => { const box = main.getBoundingClientRect(); return { top: round(box.top), bottom: round(box.bottom) } })(),
    panels, tiles,
  }
}

const problems = []
const browser = await chromium.launch()
try {
  for (const [width, height] of sizes) {
    const context = await browser.newContext({ viewport: { width, height }, colorScheme: scheme, reducedMotion: 'reduce' })
    if (withHistory) {
      await context.addInitScript(() => {
        const entry = (id, query, at) => JSON.stringify([{ id: `q-${id}`, query, label: '', answer: '', state: 'answered', pinned: false, updatedAt: at }])
        try {
          localStorage.setItem('nexusai.viewer.questions.orion-lahore-reference-20261030', entry('1', 'Who called the target at night?', '2026-11-05T05:40:00Z'))
          localStorage.setItem('nexusai.viewer.questions.nexusai-forensic-demo', entry('2', 'Which numbers share a handset?', '2026-11-04T16:10:00Z'))
          localStorage.setItem('nexusai.viewer.questions.nexusai-real-private-demo', entry('3', 'Largest transfers in March', '2026-11-03T09:00:00Z'))
        } catch { /* storage blocked: the empty state is what is checked */ }
      })
    }
    const page = await context.newPage()
    await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 60000 })
    await page.waitForSelector('.nx-lane', { timeout: 90000 }).catch(() => problems.push(`${width}x${height}: the timeline never drew`))
    await page.waitForTimeout(3500)
    const data = await page.evaluate(measure)
    const found = []
    if (data.pageOverflowX > 1) found.push(`the page scrolls sideways by ${data.pageOverflowX}px (${data.wide.join(', ') || 'no element wider than the page'})`)
    for (const panel of data.panels) {
      if (panel.overflowY > 1) found.push(`"${panel.title}" overflows its body by ${panel.overflowY}px (height)`)
      if (panel.overflowX > 1) found.push(`"${panel.title}" overflows its body by ${panel.overflowX}px (width)`)
      if (panel.squashed.length) found.push(`"${panel.title}" has content squeezed into too little room: ${panel.squashed.join(', ')}`)
      if (panel.titleLines > 1) found.push(`"${panel.title}" wraps its title onto ${panel.titleLines} lines`)
      if (panel.slack > 40) found.push(`"${panel.title}" has ${panel.slack}px of empty space at the bottom`)
    }
    // panels of one band that sit side by side share a top and a bottom
    const bands = new Map()
    for (const panel of data.panels) if (panel.band) bands.set(panel.band, [...(bands.get(panel.band) || []), panel])
    for (const [band, items] of bands) {
      const rows = new Map()
      for (const panel of items) rows.set(Math.round(panel.top / 4), [...(rows.get(Math.round(panel.top / 4)) || []), panel])
      for (const row of rows.values()) {
        if (row.length < 2) continue
        const tops = row.map(panel => panel.top), bottoms = row.map(panel => panel.bottom)
        if (Math.max(...tops) - Math.min(...tops) > 1) found.push(`band "${band}": ${row.map(panel => panel.title).join(' / ')} do not share a top`)
        if (Math.max(...bottoms) - Math.min(...bottoms) > 1) found.push(`band "${band}": ${row.map(panel => panel.title).join(' / ')} do not share a bottom (${bottoms.join(', ')})`)
      }
    }
    // the titles of panels that sit side by side start at the same height
    for (const items of [...bands.values()]) {
      const rows = new Map()
      for (const panel of items) rows.set(Math.round(panel.top / 4), [...(rows.get(Math.round(panel.top / 4)) || []), panel])
      for (const row of rows.values()) if (row.length > 1 && Math.max(...row.map(panel => panel.headTop)) - Math.min(...row.map(panel => panel.headTop)) > 1) found.push(`titles in one row start at different heights: ${row.map(panel => panel.title + ' ' + panel.headTop).join(', ')}`)
    }
    // the indicator tiles are one row of one height when they sit in one row
    const tops = [...new Set(data.tiles.map(tile => tile.top))]
    for (const top of tops) {
      const heights = new Set(data.tiles.filter(tile => tile.top === top).map(tile => tile.height))
      if (heights.size > 1) found.push(`the indicator tiles in one row differ in height (${[...heights].join(', ')})`)
    }
    // on a screen wide and tall enough for the hero, the hero is inside the first screen
    const hero = data.panels.filter(panel => panel.band === 'hero')
    if (width >= 1180 && height >= 860 && hero.length) {
      const late = hero.filter(panel => !panel.visibleAtLoad)
      if (late.length) found.push(`first screen: ${late.map(panel => `"${panel.title}" ends at ${panel.bottom} (screen ends at ${data.mainBox.bottom - data.mainBox.top})`).join('; ')}`)
    }
    const summary = hero.length ? hero.map(panel => `${panel.title} ${panel.width}x${panel.height}`).join(' · ') : '(no hero)'
    console.log(`${width}x${height} ${found.length ? 'FAIL' : 'ok  '}  ${summary}`)
    for (const line of found) { console.log(`    - ${line}`); problems.push(`${width}x${height}: ${line}`) }
    if (shots) await page.screenshot({ path: join(shots, `${width}x${height}-${scheme}.png`) })
    await context.close()
  }
} finally { await browser.close() }
console.log(problems.length ? `\n${problems.length} problem(s)` : '\nNo layout problems found.')
process.exit(problems.length ? 1 : 0)
