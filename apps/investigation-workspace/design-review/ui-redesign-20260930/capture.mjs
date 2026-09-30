// Usage: node design-review/ui-redesign-20260930/capture.mjs <label> <route> [route...]
// Captures full-page screenshots at 1440 and 375 in light and dark into <label>/, and
// reports console errors, horizontal overflow at 375 and duplicate element IDs.
import { chromium } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const [label, ...routes] = process.argv.slice(2)
const base = process.env.BASE_URL || 'http://127.0.0.1:4181'
const out = join(dirname(fileURLToPath(import.meta.url)), label)
mkdirSync(out, { recursive: true })

const browser = await chromium.launch()
const report = []
for (const route of routes) {
  for (const [width, height] of [[1440, 900], [375, 812]]) {
    for (const scheme of ['light', 'dark']) {
      const context = await browser.newContext({ viewport: { width, height }, colorScheme: scheme, reducedMotion: 'reduce' })
      const page = await context.newPage()
      const errors = []
      page.on('console', message => { if (message.type() === 'error') errors.push(message.text()) })
      page.on('pageerror', error => errors.push(String(error)))
      await page.goto(`${base}${route}`, { waitUntil: 'networkidle' })
      await page.waitForTimeout(600)
      const checks = await page.evaluate(() => {
        const ids = [...document.querySelectorAll('[id]')].map(node => node.id)
        return {
          overflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
          duplicateIds: [...new Set(ids.filter((id, index) => ids.indexOf(id) !== index))],
        }
      })
      const name = `${route.replace(/[^a-z0-9]+/gi, '_').replace(/^_|_$/g, '') || 'dashboard'}-${width}-${scheme}.png`
      await page.screenshot({ path: join(out, name), fullPage: true })
      report.push({ name, ...checks, consoleErrors: errors.length })
      if (errors.length) console.log(name, [...new Set(errors)].map(text => text.slice(0, 300)))
      await context.close()
    }
  }
}
await browser.close()
console.table(report)
