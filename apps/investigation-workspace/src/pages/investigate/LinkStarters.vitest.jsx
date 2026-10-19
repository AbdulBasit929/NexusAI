import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { LinkStarters, linkStarters } from './LinkStarters.jsx'
import { buildModel } from '../../lib/connectionsModel.js'
import { getConnections } from '../../lib/apiClient.js'

vi.mock('../../lib/apiClient.js', async importOriginal => ({ ...(await importOriginal()), getConnections: vi.fn() }))

// Synthetic numbers from the reserved range: two subscribers who share a contact (010), and a service that is left out of the starters.
const A = '923000001001'
const B = '923000001002'
const C = '923000001010'
const response = {
  available: true,
  subjects: [{ number: A, events: 25, contacts: 2 }, { number: B, events: 15, contacts: 1 }],
  edges: [
    { source: A, target: C, events: 10, outgoing: 6, incoming: 4, unknown: 0, is_service: false, is_hub: false },
    { source: B, target: C, events: 6, outgoing: 3, incoming: 3, unknown: 0, is_service: false, is_hub: false },
  ],
  totals: {},
}

describe('linkStarters', () => {
  it('writes the questions with the case\'s own numbers: the busiest subscriber, the pair that shares the most, and two hops', () => {
    const items = linkStarters(buildModel(response))
    expect(items.map(item => item.key)).toEqual(['top', 'common', 'path', 'reach'])
    expect(items[0].query).toContain(A)
    expect(items[1].query).toContain(A)
    expect(items[1].query).toContain(B)
    expect(items[2].query).toMatch(/connected within three hops, not through services or hubs\?$/)
    expect(items[3].query).toBe(`Who can be reached from ${B} within two hops, not through services or hubs?`)
  })

  it('writes nothing for a case with no subscriber', () => {
    expect(linkStarters(buildModel({ subjects: [], edges: [] }))).toEqual([])
  })
})

describe('LinkStarters', () => {
  beforeEach(() => { getConnections.mockReset() })
  afterEach(() => { vi.restoreAllMocks() })

  it('offers the questions and hands the chosen one to the page', async () => {
    getConnections.mockResolvedValue(response)
    const onPick = vi.fn()
    render(<LinkStarters caseId="case-a" onPick={onPick} />)
    const buttons = await screen.findAllByRole('button')
    expect(buttons).toHaveLength(4)
    await userEvent.click(buttons[2])
    expect(onPick).toHaveBeenCalledWith(expect.objectContaining({ scope: 'all', family: 'link analysis', query: expect.stringContaining('connected within three hops') }))
  })

  it('shows nothing, and says nothing, for a case that cannot be drawn as connections or when the read fails', async () => {
    getConnections.mockResolvedValueOnce({ available: false, reason: 'no typed call records' })
    const first = render(<LinkStarters caseId="case-a" onPick={() => {}} />)
    await vi.waitFor(() => expect(getConnections).toHaveBeenCalled())
    expect(first.container).toBeEmptyDOMElement()
    first.unmount()
    getConnections.mockRejectedValueOnce(Object.assign(new Error('x'), { reference: 'HTTP-500' }))
    const second = render(<LinkStarters caseId="case-a" onPick={() => {}} />)
    await vi.waitFor(() => expect(getConnections).toHaveBeenCalledTimes(2))
    expect(second.container).toBeEmptyDOMElement()
  })
})
