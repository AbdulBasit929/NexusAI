import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'
import { liveFixture } from './live-fixtures.js'

const caseId = 'nexusai-forensic-demo'
const reviewDirectory = fileURLToPath(new URL('../design-review/wi-ui-8/', import.meta.url))
const priorReviewDirectory = fileURLToPath(new URL('../design-review/wi-ui-7/', import.meta.url))
const widths = [375, 820, 1024, 1440]
const themeNames = { light: 'daylight-ledger', dark: 'operations-slate' }
const firstAnswer = liveFixture('CDR-04.json')
const secondAnswer = liveFixture('CDR-01.json')
const firstQuestion = firstAnswer.query_understanding.original_question
const secondQuestion = secondAnswer.query_understanding.original_question

const structuredRows = Array.from({ length: 10_000 }, (_, index) => ({
  row_number: index + 1,
  row_hash: `sha256:${String(index + 1).padStart(8, '0')}`,
  msisdn: `0300${String(index).padStart(7, '0')}`,
  call_type: index % 3 === 0 ? 'GPRS' : index % 3 === 1 ? 'SMS' : 'CALL',
  duration_seconds: index % 240,
}))

const details = {
  'structured-1': {
    item: { evidence_id: 'structured-1', original_filename: 'ten-thousand-cdr.csv', modality: 'structured_records', detected_type: 'cdr', processing_status: 'completed', size_bytes: 1_640_000, version_id: 'v-structured-1' },
    records_preview: structuredRows,
    derived_artifacts: [],
  },
  'document-1': {
    item: { evidence_id: 'document-1', original_filename: 'interview-notes.pdf', modality: 'document', detected_type: 'document', processing_status: 'completed', size_bytes: 48_000, version_id: 'v-document-1', metadata: { pages: [{ page: 1, text: 'Opening notes.' }, { page: 2, text: 'The witness arrived at 08:42 and signed the register.' }] } },
    processing_runs: [{ run_id: 'run-document-ocr', model_id: 'recorded-ocr-reader', model_revision: '2026.09' }],
    derived_artifacts: [{ artifact_id: 'ocr-document', artifact_type: 'ocr', run_id: 'run-document-ocr', confidence: 0.94, metadata: {} }],
  },
  'image-1': {
    item: { evidence_id: 'image-1', original_filename: 'gate-camera.jpg', modality: 'image', detected_type: 'image', processing_status: 'completed', size_bytes: 21_000, version_id: 'v-image-1' },
    derived_artifacts: [{ artifact_id: 'ocr-region-1', artifact_type: 'ocr_region', confidence: 0.31, metadata: { regions: [{ id: 'region-1', bbox: [0.12, 0.2, 0.36, 0.18], text: 'PK-123', confidence: 0.31 }] } }],
  },
  'audio-1': {
    item: { evidence_id: 'audio-1', original_filename: 'interview.wav', modality: 'audio', detected_type: 'audio', processing_status: 'completed', size_bytes: 120_000, version_id: 'v-audio-1' },
    derived_artifacts: [{ artifact_id: 'transcript-1', artifact_type: 'transcript', confidence: 0.91, metadata: { segments: [{ id: 'cue-1', start: 4.2, end: 7.5, speaker: 'Speaker 1', text: 'The vehicle arrived after the call.' }, { id: 'cue-2', start: 8, end: 11, speaker: 'Speaker 2', text: 'I wrote the number in the register.' }] } }],
  },
  'video-1': {
    item: { evidence_id: 'video-1', original_filename: 'checkpoint.mp4', modality: 'video', detected_type: 'video', processing_status: 'completed', size_bytes: 920_000, version_id: 'v-video-1', metadata: { duration_seconds: 60 } },
    derived_artifacts: [{ artifact_id: 'plate-1', artifact_type: 'plate_observation', confidence: 0.74, metadata: { events: [{ id: 'event-1', frame_ts: 12.5, plate: 'ABC-123', confidence: 0.74 }, { id: 'event-2', frame_ts: 42, label: 'Vehicle exits', confidence: 0.83 }] } }],
  },
}

const catalog = {
  summary: { evidence_total: Object.keys(details).length },
  items: Object.values(details).map(detail => detail.item),
}

const overview = {
  collection_id: caseId,
  summary: { evidence_total: 5, evidence_completed: 5, evidence_in_flight: 0, accepted_rows: 10_000 },
  record_families: [{ record_type: 'cdr', accepted_rows: 10_000 }],
  recent_jobs: [],
  recent_evidence: Object.values(details).map(detail => ({ evidence_id: detail.item.evidence_id, source_file: detail.item.original_filename, modality: detail.item.modality, processing_status: 'completed' })),
}

const sourceBytes = '<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360"><rect width="640" height="360" fill="#d9e7f5"/><path d="M70 260h500" stroke="#2563eb" stroke-width="8"/></svg>'

async function installRoutes(page) {
  await page.addInitScript(() => {
    window.__INVESTIGATION_WORKSPACE_CONFIG__ = { tenantId: 'default', collectionId: 'nexusai-forensic-demo', caseIds: ['nexusai-forensic-demo'], capabilities: { hybrid_query: true } }
  })
  await page.route('**/api/**', route => {
    const url = new URL(route.request().url())
    if (url.pathname.endsWith('/collections/status')) return route.fulfill({ json: overview })
    if (url.pathname.endsWith('/query/hybrid')) {
      const query = JSON.parse(route.request().postData() || '{}').query
      return route.fulfill({ json: query === secondQuestion ? secondAnswer : firstAnswer })
    }
    if (url.pathname.endsWith('/webhooks/records/upload')) return route.fulfill({ status: 202, json: { evidence_id: 'new-evidence' } })
    const detailMatch = url.pathname.match(/\/evidence\/([^/]+)$/)
    if (detailMatch) return route.fulfill({ json: details[decodeURIComponent(detailMatch[1])] || details['structured-1'] })
    if (/\/evidence\/[^/]+\/content$/.test(url.pathname)) {
      return route.fulfill({ status: 200, contentType: 'image/svg+xml', body: sourceBytes })
    }
    if (url.pathname.endsWith('/evidence')) return route.fulfill({ json: catalog })
    return route.fulfill({ status: 404, json: {} })
  })
}

async function setTheme(page, theme) {
  await page.evaluate(value => {
    document.documentElement.dataset.theme = value
    try { localStorage.setItem('nexusai.viewer.theme', value) } catch { /* rendering does not depend on storage */ }
  }, theme)
}

async function assertNoOverflow(page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0)
}

async function openQuestionHistory(page) {
  const history = page.locator('details.question-history')
  if (!await history.evaluate(node => node.open)) await history.locator('summary').click()
}

test.beforeEach(async ({ page }) => installRoutes(page))

test('opens every evidence family at an exact, openable locator with lineage', async ({ page }) => {
  const checks = [
    [`/cases/${caseId}/evidence/structured-1?version=v-structured-1&row=8421&row_hash=sha256%3A00008421`, 'Structured source', 'sha256:00008421'],
    [`/cases/${caseId}/evidence/document-1?version=v-document-1&page=2&char_span=%5B12%2C19%5D`, 'Document source · page 2', 'arrived'],
    [`/cases/${caseId}/evidence/image-1?version=v-image-1&bbox=%5B0.12%2C0.2%2C0.36%2C0.18%5D`, 'Image source', 'Low-confidence observation · 31%'],
    [`/cases/${caseId}/evidence/audio-1?version=v-audio-1&source_time=4.2&source_end=7.5`, 'Audio source', 'Speaker 1'],
    [`/cases/${caseId}/evidence/video-1?version=v-video-1&frame=12.5`, 'Video source', 'ABC-123'],
  ]
  for (const [url, heading, exactSource] of checks) {
    await page.goto(url)
    await expect(page.getByRole('heading', { name: heading })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Evidence lineage' })).toBeVisible()
    await expect(page.getByText(exactSource, { exact: false }).first()).toBeVisible()
    await expect(page.getByText('v-', { exact: false }).first()).toBeVisible()
  }
  await page.goto(`/cases/${caseId}/evidence/image-1?bbox=%5B0.12%2C0.2%2C0.36%2C0.18%5D`)
  await expect(page.getByText('Producer not recorded')).toBeVisible()
  await expect(page.getByText('Version not recorded')).toBeVisible()
  await page.getByRole('button', { name: 'Hide observation overlays' }).click()
  await expect(page.locator('.evidence-overlay')).toHaveCount(0)

  await page.goto(`/cases/${caseId}/evidence/document-1?page=2`)
  await expect(page.getByText('recorded-ocr-reader')).toBeVisible()
  await expect(page.getByText('2026.09')).toBeVisible()

  await page.goto(`/cases/${caseId}/evidence/audio-1?source_time=4.2`)
  const audio = page.locator('audio')
  await audio.dispatchEvent('loadedmetadata')
  await expect.poll(() => audio.evaluate(node => node.currentTime)).toBe(4.2)
  await page.getByRole('button', { name: /8\.0s.*Speaker 2/ }).click()
  await expect.poll(() => audio.evaluate(node => node.currentTime)).toBe(8)
})

test('keeps ten thousand rows semantic, virtual, and locally interactive', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'One deterministic browser profile owns the measured 10k-row proof.')
  await page.goto(`/cases/${caseId}/evidence/structured-1?row=8421`)
  const table = page.getByRole('table')
  await expect(table).toHaveAttribute('aria-rowcount', '10001')
  await expect(page.getByText('10,000 of 10,000 loaded rows')).toBeVisible()
  await expect(page.getByRole('cell', { name: '8421', exact: true })).toBeVisible()
  expect(await table.getByRole('row').count()).toBeLessThan(40)
  const elapsed = await page.locator('.table-shell--virtual').evaluate(async node => {
    const start = performance.now()
    node.scrollTop = 240_000
    node.dispatchEvent(new Event('scroll', { bubbles: true }))
    await new Promise(resolve => requestAnimationFrame(resolve))
    return performance.now() - start
  })
  expect(elapsed).toBeLessThan(100)
  await page.getByLabel('Filter rows').fill('03000000042')
  await expect(page.getByText('1 of 10,000 loaded rows')).toBeVisible()
  await page.getByRole('button', { name: 'Comfortable' }).click()
  await expect(page.getByRole('button', { name: 'Comfortable' })).toHaveAttribute('aria-pressed', 'true')
  await page.getByLabel('Filter rows').fill('')
  const downloadEvent = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Export current result' }).click()
  const download = await downloadEvent
  const exported = fs.readFileSync(await download.path(), 'utf8')
  expect(exported).toContain('Source row')
  expect(exported).toContain('sha256:00008421')
})

test('opens the command palette and shortcut sheet on every application route', async ({ page }) => {
  const routes = ['/', '/cases', '/activity', '/settings', `/cases/${caseId}/overview`, `/cases/${caseId}/evidence`, `/cases/${caseId}/investigate`, `/cases/${caseId}/timeline`, `/cases/${caseId}/activity`]
  for (const route of routes) {
    await page.goto(route)
    await expect(page.locator('#workspace-main')).toBeVisible()
    await page.keyboard.press('Control+K')
    await expect(page.getByRole('dialog', { name: 'Quick find' })).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(page.getByRole('dialog', { name: 'Quick find' })).toHaveCount(0)
  }
  await page.keyboard.press('?')
  await expect(page.getByRole('dialog', { name: 'Keyboard shortcuts' })).toBeVisible()
  await expect(page.getByText('Focus the question composer')).toBeVisible()
})

test('preserves a draft across reload and remains usable when storage throws', async ({ page }) => {
  await page.goto(`/cases/${caseId}/investigate`)
  const composer = page.getByLabel('Ask a question about this case')
  await composer.fill('Keep this forensic draft')
  await page.reload()
  await expect(page.getByLabel('Ask a question about this case')).toHaveValue('Keep this forensic draft')

  const broken = await page.context().newPage()
  await installRoutes(broken)
  await broken.addInitScript(() => {
    Storage.prototype.getItem = () => { throw new Error('storage denied') }
    Storage.prototype.setItem = () => { throw new Error('storage denied') }
  })
  await broken.goto(`/cases/${caseId}/investigate`)
  await broken.getByLabel('Ask a question about this case').fill('Works without storage')
  await expect(broken.getByLabel('Ask a question about this case')).toHaveValue('Works without storage')
  await broken.close()
})

test('supports history, editing, pinning, rerunning, and two-result comparison', async ({ page }) => {
  await page.goto(`/cases/${caseId}/investigate`)
  const composer = page.getByLabel('Ask a question about this case')
  await composer.fill(firstQuestion)
  await page.getByRole('button', { name: 'Ask', exact: true }).click()
  await expect(page.locator('.investigation-result').last()).toBeVisible()
  await page.getByRole('button', { name: 'Add result to comparison' }).last().click()
  await openQuestionHistory(page)
  await page.getByRole('button', { name: 'Pin', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Unpin', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Edit', exact: true }).click()
  await expect(composer).toHaveValue(firstQuestion)
  await composer.fill(secondQuestion)
  await page.getByRole('button', { name: 'Ask', exact: true }).click()
  await page.getByRole('button', { name: 'Add result to comparison' }).last().click()
  await expect(page.locator('.result-comparison article')).toHaveCount(2)
  await expect(page.locator('.result-value-link').first()).toBeVisible()
  await openQuestionHistory(page)
  const rerun = page.waitForRequest(request => request.url().includes('/query/hybrid') && request.method() === 'POST')
  await page.getByRole('button', { name: 'Re-run' }).first().click()
  await rerun
  await page.reload()
  await openQuestionHistory(page)
  await expect(page.getByRole('button', { name: 'Unpin', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Re-run' })).toHaveCount(2)
  await expect(page.locator('.rail-questions')).toContainText(firstQuestion)
})

test('provides the keyboard-first evidence-to-answer-to-source loop available without login', async ({ page }) => {
  await page.goto(`/cases/${caseId}/evidence`)
  const intakeSummary = page.locator('summary').filter({ hasText: 'Add evidence to this case' })
  await intakeSummary.focus()
  await page.keyboard.press('Enter')
  await page.getByLabel('Choose evidence files to add to this case').setInputFiles({ name: 'fresh-cdr.csv', mimeType: 'text/csv', buffer: Buffer.from('msisdn,call_type\n03001234567,CALL') })
  await expect(page.getByText('Accepted — processing has started')).toBeVisible()
  await expect(page.getByText('5 of 5 sources ready', { exact: true })).toBeVisible()
  await page.keyboard.press('Control+K')
  await page.getByRole('combobox').fill('Investigate')
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(new RegExp(`/cases/${caseId}/investigate`))
  await expect(page.getByLabel('Ask a question about this case')).toBeVisible()
  await page.keyboard.press('Alt+A')
  await expect(page.getByLabel('Ask a question about this case')).toBeFocused()
  await page.keyboard.type(firstQuestion)
  await page.keyboard.press('Tab')
  await page.keyboard.press('Enter')
  await expect(page.locator('.answer-block .citation-marker').first()).toBeVisible()
  await page.locator('.answer-block .citation-marker').first().click()
  await expect(page).toHaveURL(/\/evidence\/structured-1|\/evidence\//)
  await expect(page.getByRole('heading', { name: 'Exact source location' })).toBeVisible()
  await page.goBack()
  await expect(page.getByRole('heading', { name: 'Question history' })).toBeVisible()
  await openQuestionHistory(page)
  await page.getByRole('button', { name: 'Re-run' }).first().click()
  await expect(page.locator('.follow-ups button').first()).toBeVisible()
  await page.locator('.follow-ups button').first().click()
  await expect(page.locator('.investigation-result').last()).toBeVisible()
  await page.keyboard.press('Control+K')
  await page.getByRole('combobox').fill('Case activity')
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(new RegExp(`/cases/${caseId}/activity`))
  await expect(page.locator('.activity-list').getByText(firstQuestion, { exact: true }).first()).toBeVisible()
})

test('has no horizontal page scroll at the 375px contract width', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 })
  const routes = Object.keys(details).map(id => `/cases/${caseId}/evidence/${id}`)
  routes.push(`/cases/${caseId}/investigate`)
  for (const route of routes) {
    await page.goto(route)
    await expect(page.locator('#workspace-main')).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0)
  }
})

test('keeps new compact controls at least 44px on the touch layout', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 })
  const checks = [
    [`/cases/${caseId}/evidence/structured-1`, '.data-table__toolbar button, .data-table__toolbar input'],
    [`/cases/${caseId}/evidence/video-1?frame=12.5`, '.event-scrubber button, .video-events button'],
    [`/cases/${caseId}/investigate`, '.filter-chips button'],
  ]
  for (const [route, selector] of checks) {
    await page.goto(route)
    await expect(page.locator(selector).first()).toBeVisible()
    const sizes = await page.locator(selector).evaluateAll(nodes => nodes.filter(node => node.getClientRects().length).map(node => {
      const box = node.getBoundingClientRect()
      return { width: box.width, height: box.height, label: node.getAttribute('aria-label') || node.textContent.trim() }
    }))
    for (const size of sizes) {
      expect(size.height, `${size.label} height`).toBeGreaterThanOrEqual(44)
      expect(size.width, `${size.label} width`).toBeGreaterThanOrEqual(44)
    }
  }
})

test('captures the WI-UI-8 viewer and working-loop review matrix', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== 'desktop', 'One deterministic project owns review artifacts.')
  test.setTimeout(360_000)
  fs.mkdirSync(reviewDirectory, { recursive: true })
  const viewerRoutes = {
    'viewer-structured': `/cases/${caseId}/evidence/structured-1?version=v-structured-1&row=8421`,
    'viewer-document': `/cases/${caseId}/evidence/document-1?version=v-document-1&page=2&char_span=%5B12%2C19%5D`,
    'viewer-image': `/cases/${caseId}/evidence/image-1?version=v-image-1&bbox=%5B0.12%2C0.2%2C0.36%2C0.18%5D`,
    'viewer-audio': `/cases/${caseId}/evidence/audio-1?version=v-audio-1&source_time=4.2&source_end=7.5`,
    'viewer-video': `/cases/${caseId}/evidence/video-1?version=v-video-1&frame=12.5`,
  }
  for (const width of widths) {
    await page.setViewportSize({ width, height: width === 375 ? 812 : 1000 })
    for (const theme of ['light', 'dark']) {
      for (const [surface, route] of Object.entries(viewerRoutes)) {
        await page.goto(route)
        await setTheme(page, theme)
        await expect(page.getByRole('heading', { name: 'Evidence lineage' })).toBeVisible()
        await assertNoOverflow(page)
        await page.evaluate(() => scrollTo(0, 0))
        await page.screenshot({ path: path.join(reviewDirectory, `${surface}-${themeNames[theme]}-${width}.png`), fullPage: true })
      }

      await page.goto(`/cases/${caseId}/investigate`)
      await setTheme(page, theme)
      await expect(page.getByLabel('Ask a question about this case')).toBeVisible()
      await page.keyboard.press('Control+K')
      await expect(page.getByRole('dialog', { name: 'Quick find' })).toBeVisible()
      await assertNoOverflow(page)
      await page.evaluate(() => scrollTo(0, 0))
      await page.screenshot({ path: path.join(reviewDirectory, `command-palette-${themeNames[theme]}-${width}.png`), fullPage: true })
      await page.keyboard.press('Escape')
      await page.keyboard.press('?')
      await expect(page.getByRole('dialog', { name: 'Keyboard shortcuts' })).toBeVisible()
      await assertNoOverflow(page)
      await page.evaluate(() => scrollTo(0, 0))
      await page.screenshot({ path: path.join(reviewDirectory, `shortcut-sheet-${themeNames[theme]}-${width}.png`), fullPage: true })
      await page.getByRole('button', { name: 'Close' }).click()

      const composer = page.getByLabel('Ask a question about this case')
      await composer.fill(firstQuestion)
      await page.getByRole('button', { name: 'Ask', exact: true }).click()
      if (width === 1440) {
        await page.evaluate(() => scrollTo(0, 0))
        await page.screenshot({ path: path.join(reviewDirectory, `finding-after-${themeNames[theme]}-1440.png`), fullPage: true })
        const prior = path.join(priorReviewDirectory, `answered-${themeNames[theme]}-1440.png`)
        if (fs.existsSync(prior)) fs.copyFileSync(prior, path.join(reviewDirectory, `finding-before-${themeNames[theme]}-1440.png`))
      }
      await page.getByRole('button', { name: 'Add result to comparison' }).last().click()
      await composer.fill(secondQuestion)
      await page.getByRole('button', { name: 'Ask', exact: true }).click()
      await page.getByRole('button', { name: 'Add result to comparison' }).last().click()
      await expect(page.locator('.result-comparison article')).toHaveCount(2)
      await assertNoOverflow(page)
      await page.evaluate(() => scrollTo(0, 0))
      await page.screenshot({ path: path.join(reviewDirectory, `working-comparison-${themeNames[theme]}-${width}.png`), fullPage: true })
    }
  }
})
