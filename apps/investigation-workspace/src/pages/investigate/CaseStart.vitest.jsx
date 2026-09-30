import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { CaseStart, coverageLine } from './CaseStart.jsx'

const overview = over => ({ data: { summary: { evidence_total: 10, evidence_completed: 8, evidence_in_flight: 2, accepted_rows: 5200, ...over } } })

describe('coverageLine', () => {
  it('says how much is ready, the rows, and that what is not ready is not searched', () => {
    expect(coverageLine(overview())).toBe('8 of 10 sources ready · 5,200 structured rows. The 2 not ready are not searched.')
    expect(coverageLine(overview({ evidence_in_flight: 0, evidence_completed: 10 }))).toBe('10 of 10 sources ready · 5,200 structured rows')
    expect(coverageLine(overview({ evidence_total: 3, evidence_completed: 2, accepted_rows: 0 }))).toBe('2 of 3 sources ready. The 1 not ready is not searched.')
  })

  it('is honest about an empty case and says nothing before the status is known', () => {
    expect(coverageLine(overview({ evidence_total: 0, evidence_completed: 0 }))).toBe('This case has no evidence yet, so there is nothing to ask.')
    expect(coverageLine({ loading: true })).toBe('')
  })
})

describe('CaseStart', () => {
  const starters = [{ query: 'Who called most often?', family: 'Call detail records', scope: 'cdr' }]
  const recent = [{ id: 'r1', query: 'Where was the phone on 2 March?', label: '' }, { id: 'r2', query: 'Long text', label: 'Short name' }]

  it('offers suggested questions, the analyst’s last questions here, and a way to ask every case', async () => {
    const onPick = vi.fn()
    render(<MemoryRouter><CaseStart caseId="case-alpha" overview={overview()} starters={starters} recent={recent} onPick={onPick} /></MemoryRouter>)
    const user = userEvent.setup()
    expect(screen.getByRole('heading', { name: 'What do you want to verify in case-alpha ?' })).toBeTruthy()
    await user.click(screen.getByRole('button', { name: /Who called most often/ }))
    expect(onPick).toHaveBeenCalledWith(starters[0])
    await user.click(screen.getByRole('button', { name: 'Short name' }))
    expect(onPick).toHaveBeenLastCalledWith({ query: 'Long text', scope: 'all' })
    expect(screen.getByRole('link', { name: /Ask across all cases/ }).getAttribute('href')).toBe('/investigate')
  })

  it('leaves out the blocks it has nothing for', () => {
    render(<MemoryRouter><CaseStart caseId="c" overview={{ loading: true }} starters={[]} recent={[]} onPick={() => {}} /></MemoryRouter>)
    expect(screen.getByText('Checking what this case holds…')).toBeTruthy()
    expect(screen.queryByText(/Suggested from the evidence/)).toBeNull()
    expect(screen.queryByText(/Your last questions/)).toBeNull()
  })
})
