// Captures the "Page header, cards and states" gallery section in both themes at 1440 and 375.
import { chromium } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const out = join(dirname(fileURLToPath(import.meta.url)), 'canvas')
mkdirSync(out, { recursive: true })
const browser = await chromium.launch()
for (const [width, height] of [[1440, 900], [375, 812]]) {
  for (const scheme of ['light', 'dark']) {
    const context = await browser.newContext({ viewport: { width, height }, colorScheme: scheme, reducedMotion: 'reduce' })
    const page = await context.newPage()
    const errors = []
    page.on('console', message => { if (message.type() === 'error') errors.push(message.text()) })
    await page.goto('http://127.0.0.1:4181/design/type-proof', { waitUntil: 'networkidle' })
    await page.locator('#gallery-page-title').locator('xpath=ancestor::section[1]').screenshot({ path: join(out, `states-${width}-${scheme}.png`) })
    console.log(width, scheme, 'errors', errors.length, await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth))
    await context.close()
  }
}
await browser.close()
