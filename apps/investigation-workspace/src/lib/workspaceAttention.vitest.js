import { describe, expect, it } from 'vitest'
import { countAttention } from './workspaceAttention.js'

describe('sidebar attention counts', () => {
  it('counts only cases whose status was actually reported', () => {
    expect(countAttention([
      { processingState: 'attention' },
      { processingState: 'processing' },
      { processingState: 'complete' },
      null,
    ])).toEqual({ reported: 3, needReview: 1, processing: 1 })
  })

  it('reports nothing rather than zero-shaped success when every status call failed', () => {
    expect(countAttention([null, null])).toEqual({ reported: 0, needReview: 0, processing: 0 })
  })
})
