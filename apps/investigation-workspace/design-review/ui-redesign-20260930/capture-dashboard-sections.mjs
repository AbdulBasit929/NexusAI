// Usage: node design-review/ui-redesign-20260930/capture-dashboard-sections.mjs <label> <selector> [selector...]
// Element screenshots of dashboard sections at 1440 and 375, light and dark, against the real API. Reports
// console errors, overflow and duplicate ids for the page.
import { chromium } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const [label, ...selectors] = process.argv.slice(2)
const out = join(dirname(fileURLToPath(import.meta.url)), 'dashboard', label)
mkdirSync(out, { recursive: true })
const browser = await chromium.launch()
for (const [width, height] of [[1440, 900], [375, 812]]) {
  for (const scheme of ['light', 'dark']) {
    const context = await browser.newContext({ viewport: { width, height }, colorScheme: scheme, reducedMotion: 'reduce' })
    const page = await context.newPage()
    const errors = []
    page.on('console', message => { if (message.type() === 'error') errors.push(message.text()) })
    page.on('pageerror', error => errors.push(String(error)))
    await page.goto('http://127.0.0.1:4181/', { waitUntil: 'networkidle' })
    await page.waitForTimeout(900)
    for (const [index, selector] of selectors.entries()) {
      await page.locator(selector).first().screenshot({ path: join(out, `${index + 1}-${width}-${scheme}.png`) })
    }
    const checks = await page.evaluate(() => {
      const ids = [...document.querySelectorAll('[id]')].map(node => node.id)
      return { overflow: document.documentElement.scrollWidth - document.documentElement.clientWidth, duplicateIds: [...new Set(ids.filter((id, index) => ids.indexOf(id) !== index))] }
    })
    console.log(width, scheme, 'errors', errors.length, errors.slice(0, 2), checks)
    await context.close()
  }
}
await browser.close()
