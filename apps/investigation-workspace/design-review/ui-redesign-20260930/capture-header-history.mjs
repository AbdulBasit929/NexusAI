// Review harness only: seeds browser-local question history (never product data) to review the header controls,
// the Investigate question history and the command palette's recent questions.
import { chromium } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const out = join(dirname(fileURLToPath(import.meta.url)), 'header-history')
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
  const errors = []
  page.on('console', message => { if (message.type() === 'error') errors.push(message.text()) })
  await page.goto('http://127.0.0.1:4181/', { waitUntil: 'networkidle' })
  await page.waitForTimeout(500)
  const labelColour = await page.evaluate(() => { const link = document.querySelector('.header-ask'); return [getComputedStyle(link).color, getComputedStyle(link.querySelector('span')).color] })
  if (labelColour[0] !== labelColour[1]) throw new Error('Header label colour differs from its button: ' + labelColour.join(' vs '))
  await page.screenshot({ path: join(out, `header-${scheme}.png`), clip: { x: 0, y: 0, width: 1440, height: 72 } })
  await page.getByRole('button', { name: /^Appearance/ }).click()
  await page.screenshot({ path: join(out, `appearance-${scheme}.png`), clip: { x: 900, y: 0, width: 540, height: 330 } })
  await page.keyboard.press('Escape')
  await page.screenshot({ path: join(out, `sidebar-${scheme}.png`), clip: { x: 0, y: 0, width: 300, height: 900 } })
  await page.goto('http://127.0.0.1:4181/investigate', { waitUntil: 'networkidle' })
  await page.waitForTimeout(500)
  await page.screenshot({ path: join(out, `investigate-${scheme}.png`), fullPage: true })
  await page.keyboard.press('Control+k')
  await page.waitForTimeout(400)
  await page.screenshot({ path: join(out, `palette-${scheme}.png`) })
  console.log(scheme, 'console errors', errors.length)
  await context.close()
}
const mobile = await browser.newContext({ viewport: { width: 375, height: 812 }, colorScheme: 'dark', reducedMotion: 'reduce' })
await mobile.addInitScript(entries => { for (const [key, value] of Object.entries(entries)) localStorage.setItem(key, value) }, seed)
const phone = await mobile.newPage()
await phone.goto('http://127.0.0.1:4181/investigate', { waitUntil: 'networkidle' })
await phone.waitForTimeout(500)
await phone.getByRole('button', { name: /^Appearance/ }).click()
await phone.screenshot({ path: join(out, 'appearance-375-dark.png') })
await phone.keyboard.press('Escape')
await phone.screenshot({ path: join(out, 'investigate-375-dark.png'), fullPage: true })
console.log('overflow', await phone.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth))
await browser.close()
