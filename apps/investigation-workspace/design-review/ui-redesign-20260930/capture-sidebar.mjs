// Review harness only: seeds browser-local question history (never product data) so the sidebar's
// pinned and recent rows can be reviewed. Captures the sidebar and the mobile drawer.
import { chromium } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const out = join(dirname(fileURLToPath(import.meta.url)), 'sidebar-review')
mkdirSync(out, { recursive: true })
const now = Date.now()
const iso = minutes => new Date(now - minutes * 60_000).toISOString()
const seed = {
  'nexusai.viewer.questions.nexusai-forensic-demo': JSON.stringify([
    { id: 'p1', query: 'Which numbers called the same tower within ten minutes of each other?', label: 'Tower co-location', state: 'answered', pinned: true, updatedAt: iso(300) },
    { id: 'r1', query: 'Who called 03001234567 most often?', label: '', state: 'answered', pinned: false, updatedAt: iso(12) },
    { id: 'r2', query: 'List every vehicle seen near the border checkpoint after midnight', label: '', state: 'answered', pinned: false, updatedAt: iso(180) },
  ]),
  'nexusai.viewer.questions.nexusai-multimodal-product-acceptance': JSON.stringify([
    { id: 'r3', query: 'کال 03001234567 کب کی گئی؟', label: '', state: 'answered', pinned: false, updatedAt: iso(60 * 30) },
  ]),
}
const browser = await chromium.launch()
for (const scheme of ['light', 'dark']) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, colorScheme: scheme, reducedMotion: 'reduce' })
  await context.addInitScript(entries => { for (const [key, value] of Object.entries(entries)) localStorage.setItem(key, value) }, seed)
  const page = await context.newPage()
  await page.goto('http://127.0.0.1:4181/cases/nexusai-forensic-demo/overview', { waitUntil: 'networkidle' })
  await page.waitForTimeout(700)
  await page.locator('.desktop-rail').screenshot({ path: join(out, `sidebar-${scheme}.png`) })
  await page.locator('.rail-question').first().hover()
  await page.locator('.rail-question__more').first().click()
  await page.locator('.desktop-rail').screenshot({ path: join(out, `sidebar-menu-${scheme}.png`) })
  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: 'Collapse navigation' }).click()
  await page.waitForTimeout(300)
  await page.locator('.desktop-rail').screenshot({ path: join(out, `sidebar-collapsed-${scheme}.png`) })
  await context.close()
}
const mobile = await browser.newContext({ viewport: { width: 375, height: 812 }, colorScheme: 'dark', reducedMotion: 'reduce' })
await mobile.addInitScript(entries => { for (const [key, value] of Object.entries(entries)) localStorage.setItem(key, value) }, seed)
const phone = await mobile.newPage()
await phone.goto('http://127.0.0.1:4181/cases/nexusai-forensic-demo/overview', { waitUntil: 'networkidle' })
await phone.getByRole('button', { name: /Menu/ }).click()
await phone.waitForTimeout(400)
await phone.screenshot({ path: join(out, 'drawer-375-dark.png') })
await browser.close()
console.log('done')
