import test from 'node:test'
import assert from 'node:assert/strict'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import ActivityResultTable from './ActivityResultTable.js'
import { activityItemPresentation, activityTable } from './analystActivityPresentation.js'

test('actual Activity table renders malformed historical data without inventing plate facts', () => {
  for (const rows of [[{ x: 10, y: 20, width: 30, height: 40, section: 'bbox' }], [null], 'wrong-shape']) {
    const table = { rows, columns: null, priority_columns: null }
    const html = renderToStaticMarkup(createElement(ActivityResultTable, { table, operation: 'video.anpr_grouped_timeline' }))
    assert.match(html, /Result details are unavailable/)
    assert.doesNotMatch(html, /<table|<td|nexusai:\/\//)
  }
  assert.doesNotThrow(() => renderToStaticMarkup(createElement(ActivityResultTable, { table: null })))
  assert.deepEqual(activityTable({ rows: [{ count: 2 }], columns: null }).columns, ['count'])
})

test('actual enterprise -> agent -> retained metadata reopens with plate, time, frame and source actions', { skip: !process.env.NX_VIDEO_CONTRACT_DIR }, () => {
  const cases = JSON.parse(readFileSync(join(process.env.NX_VIDEO_CONTRACT_DIR, 'retained.json'), 'utf8'))
  for (const kind of ['positive', 'group', 'negative']) {
    const view = activityItemPresentation(cases[kind])
    const html = renderToStaticMarkup(createElement(ActivityResultTable, { table: view.presentation.table, operation: view.presentation.operation_id }))
    assert.doesNotMatch(html, /nexusai:\/\/|bbox|synthetic-version|11111111/)
    if (kind === 'negative') {
      assert.match(view.answer, /XYZ999/)
      assert.equal(view.sources.length, 0)
      assert.equal(view.result.id, 'no_match_for_filter')
    } else {
      assert.match(html, /ABC128/)
      assert.match(html, /synthetic-video.mp4/)
      assert.match(html, /1 second/)
      assert.equal(view.sources.length, kind === 'positive' ? 2 : 1)
      assert.equal(view.sources[0].evidenceId, '11111111-1111-4111-8111-111111111111')
      assert.match(view.sources[0].navigation.toString(), /evidence=/)
      assert.equal(view.sources[0].navigation.get('source_time'), '1')
      assert.match(view.sources[0].navigation.get('finding'), /synthetic-(observation|group)/)
      if (kind === 'positive') { assert.match(html, /ABC123/); assert.match(html, />60</); assert.match(html, />120</) }
      assert.equal(html, renderToStaticMarkup(createElement(ActivityResultTable, { table: activityItemPresentation(JSON.parse(JSON.stringify(cases[kind]))).presentation.table })))
    }
  }
})
