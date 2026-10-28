import fs from 'node:fs'
import path from 'node:path'
import zlib from 'node:zlib'
import { chromium } from '@playwright/test'

const appRoot = process.cwd()
const url = process.env.NX_FOUNDATIONS_URL || 'http://localhost:4182/design/foundations'
const output = path.resolve(appRoot, '../../docs/ux/evidence/U1/metrics.json')

const percentile = (values, proportion) => {
  if (!values.length) return null
  const ordered = [...values].sort((a, b) => a - b)
  return Number(ordered[Math.max(0, Math.ceil(ordered.length * proportion) - 1)].toFixed(2))
}

const sizesFor = pattern => {
  const assets = path.join(appRoot, 'dist/assets')
  const name = fs.readdirSync(assets).find(file => pattern.test(file))
  if (!name) return null
  const bytes = fs.readFileSync(path.join(assets, name))
  return { file: name, rawBytes: bytes.byteLength, gzipBytes: zlib.gzipSync(bytes).byteLength }
}

const browser = await chromium.launch({ headless: true })
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
await page.goto(url, { waitUntil: 'networkidle' })
await page.evaluate(() => {
  window.__nxPaints = []
  window.__nxEvents = []
  document.addEventListener('click', event => {
    const button = event.target.closest('button')
    if (!button) return
    const started = performance.now()
    requestAnimationFrame(() => requestAnimationFrame(() => {
      window.__nxPaints.push({ label: button.textContent.trim(), duration: performance.now() - started })
    }))
  }, { capture: true })
  if ('PerformanceObserver' in window && PerformanceObserver.supportedEntryTypes?.includes('event')) {
    const observer = new PerformanceObserver(list => {
      for (const entry of list.getEntries()) {
        if (entry.name === 'click') window.__nxEvents.push(entry.duration)
      }
    })
    observer.observe({ type: 'event', durationThreshold: 16 })
  }
})

const measure = async (label, choices) => {
  await page.evaluate(() => { window.__nxPaints = []; window.__nxEvents = [] })
  for (let index = 0; index < 8; index += 1) {
    await page.getByRole('button', { name: choices[index % choices.length], exact: true }).click()
    await page.waitForTimeout(80)
  }
  const sample = await page.evaluate(() => ({
    paints: window.__nxPaints.map(item => item.duration),
    events: [...window.__nxEvents],
  }))
  return {
    interaction: label,
    samples: sample.paints.length,
    nextPaintProxyP75Ms: percentile(sample.paints, 0.75),
    nextPaintProxyMaxMs: sample.paints.length ? Number(Math.max(...sample.paints).toFixed(2)) : null,
    eventTimingP75Ms: percentile(sample.events, 0.75),
    eventTimingSamples: sample.events.length,
  }
}

const interactions = [
  await measure('switch colour direction', ['A · Teal instrument', 'B · Ink and signal']),
  await measure('switch theme', ['Light', 'Dark']),
  await measure('switch density', ['compact', 'default']),
]

await browser.close()

const report = {
  measuredAt: new Date().toISOString(),
  route: url,
  environment: 'Chromium headless, 1440 × 1000, local Vite server',
  method: 'Eight trusted clicks per interaction. Event Timing is reported when the browser emits entries; the double-requestAnimationFrame value is a local interaction-to-next-paint proxy, not field INP.',
  interactions,
  bundle: {
    initialJavaScript: sizesFor(/^index-.*\.js$/),
    foundationsJavaScript: sizesFor(/^FoundationsPage-.*\.js$/),
    globalCss: sizesFor(/^index-.*\.css$/),
    foundationsCss: sizesFor(/^FoundationsPage-.*\.css$/),
  },
}

fs.mkdirSync(path.dirname(output), { recursive: true })
fs.writeFileSync(output, `${JSON.stringify(report, null, 2)}\n`)
console.log(JSON.stringify(report, null, 2))
