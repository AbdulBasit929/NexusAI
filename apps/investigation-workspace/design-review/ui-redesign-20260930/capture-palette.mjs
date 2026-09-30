// Captures the palette-options gallery section (page theme light and dark) at 1440 and 375.
import { chromium } from '@playwright/test'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const out = join(dirname(fileURLToPath(import.meta.url)), 'palette')
import { mkdirSync } from 'node:fs'
mkdirSync(out, { recursive: true })
const browser = await chromium.launch()
for (const [width, height] of [[1440, 900], [375, 812]]) {
  for (const scheme of ['light', 'dark']) {
    const context = await browser.newContext({ viewport: { width, height }, colorScheme: scheme, reducedMotion: 'reduce' })
    const page = await context.newPage()
    await page.goto('http://127.0.0.1:4181/design/type-proof', { waitUntil: 'networkidle' })
    for (const option of ['A', 'B', 'C']) {
      await page.locator(`#palette-option-${option}`).locator('xpath=ancestor::article').screenshot({ path: join(out, `option-${option}-${width}-page-${scheme}.png`) })
    }
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    console.log(width, scheme, 'overflow', overflow)
    await context.close()
  }
}
await browser.close()
