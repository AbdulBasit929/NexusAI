import { describe, expect, it } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { CasesTable } from './CasesTable.jsx'
import { ContinueCards } from './ContinueCards.jsx'
import { caseSparks, mergeActivity, parseActivity } from '../../lib/caseActivity.js'
import { curatedQuestions, dashboardNextAction, latestEvidenceActivity, relativeAge } from '../../lib/dashboardCases.js'

const show = ui => render(<MemoryRouter>{ui}</MemoryRouter>)
const summary = over => ({ total: 43, ready: 36, inFlight: 1, failed: 6, missingAssets: 2, acceptedRows: 22660, rejectedRows: 8, duplicateRows: 40, processingState: 'attention', families: [], ...over })
const now = Date.parse('2026-09-30T12:00:00Z')
const rows = [
  { caseId: 'ready-case', index: 0, status: 'complete', summary: summary({ total: 10, ready: 10, inFlight: 0, failed: 0, missingAssets: 0, rejectedRows: 0, duplicateRows: 0, processingState: 'complete' }), activity: null, state: {} },
  { caseId: 'alpha', index: 1, status: 'attention', summary: summary(), activity: { date: new Date('2026-09-30T09:00:00Z') }, state: {} },
  { caseId: 'down', index: 2, status: 'unavailable', summary: null, activity: null, state: { error: { status: 500 } } },
]
const row = name => screen.getByRole('link', { name }).closest('tr')

describe('Cases table', () => {
  it('lists the case that needs review first, then ready, then one that could not be read', () => {
    show(<CasesTable rows={rows} sparks={{}} now={now} />)
    expect(screen.getByRole('heading', { name: 'Which case needs me next?' })).toBeTruthy()
    expect([...document.querySelectorAll('tbody th a')].map(link => link.textContent)).toEqual(['alpha', 'ready-case', 'down'])
  })

  it('shows exact figures, the review count as a link to the failed sources, and a floored age', () => {
    show(<CasesTable rows={rows} sparks={{}} now={now} />)
    const alpha = within(row('alpha'))
    expect(alpha.getByText('36 of 43')).toBeTruthy()
    expect(alpha.getByRole('link', { name: '8' }).getAttribute('href')).toBe('/cases/alpha/evidence?status=failed')
    expect(alpha.getByText('22,660')).toBeTruthy()
    expect(alpha.getByText('8 rejected · 40 duplicate')).toBeTruthy()
    expect(alpha.getByText('3 h ago')).toBeTruthy()
    expect(alpha.getByRole('link', { name: /Review evidence/ }).getAttribute('href')).toBe('/cases/alpha/evidence')
    expect(within(row('ready-case')).getByText('None')).toBeTruthy()
  })

  it('says a case could not be read instead of showing zeros, and "Not read" where activity was not read', () => {
    show(<CasesTable rows={rows} sparks={{}} now={now} />)
    const down = within(row('down'))
    expect(down.getByText('Could not be read')).toBeTruthy()
    expect(down.getAllByText('—').length).toBeGreaterThan(0)
    expect(within(row('alpha')).getByText('Not read')).toBeTruthy()
  })

  it('draws a real trend when the case activity was read', () => {
    const activity = mergeActivity([parseActivity({ records: { activity_by_day: [{ activity_date: '2026-09-01', record_type: 'cdr', event_count: 5 }, { activity_date: '2026-09-02', record_type: 'cdr', event_count: 7 }] } }, 'alpha')])
    show(<CasesTable rows={rows} sparks={caseSparks(activity)} now={now} />)
    expect(within(row('alpha')).getByRole('img', { name: '12 events in the 30 days to 2026-09-02' })).toBeTruthy()
  })

  it('filters by state with exact counts, and clears a family filter', async () => {
    let cleared = 0
    show(<CasesTable rows={rows} sparks={{}} now={now} familyLabel="Call detail records" onClearFamily={() => { cleared += 1 }} />)
    const user = userEvent.setup()
    expect(screen.getByRole('button', { name: /Needs review\s*1/ })).toBeTruthy()
    await user.click(screen.getByRole('button', { name: /Needs review/ }))
    expect([...document.querySelectorAll('tbody th a')].map(link => link.textContent)).toEqual(['alpha'])
    await user.click(screen.getByRole('button', { name: /Clear/ }))
    expect(cleared).toBe(1)
  })

  it('keeps long lists short until asked', async () => {
    const many = Array.from({ length: 12 }, (_, index) => ({ caseId: `case-${index}`, index, status: 'complete', summary: summary({ processingState: 'complete', failed: 0, missingAssets: 0 }), activity: null, state: {} }))
    show(<CasesTable rows={many} sparks={{}} now={now} />)
    expect(document.querySelectorAll('tbody tr')).toHaveLength(8)
    await userEvent.setup().click(screen.getByRole('button', { name: 'Show all 12 cases' }))
    expect(document.querySelectorAll('tbody tr')).toHaveLength(12)
  })
})

describe('Continue cards', () => {
  const recent = [{ id: 'q1', caseId: 'alpha', query: 'Who called most?', label: '' }, { id: 'q2', caseId: 'bravo', query: 'Where was activity?', label: 'Places', pinned: true }]

  it('lists pinned questions first and links each into its case', () => {
    show(<ContinueCards recent={recent} suggestions={[]} suggestionCase="" suggestionsLoading={false} />)
    const links = within(screen.getByRole('heading', { name: 'Pick up where you left off' }).closest('section')).getAllByRole('link')
    expect(links[0].getAttribute('href')).toBe('/cases/bravo/investigate?question=Where%20was%20activity%3F')
    expect(links[1].getAttribute('href')).toBe('/cases/alpha/investigate?question=Who%20called%20most%3F')
  })

  it('offers suggested questions for the named case, and is honest when there are none', () => {
    const { rerender } = show(<ContinueCards recent={[]} suggestions={[{ query: 'Which identifiers occur most often?' }]} suggestionCase="alpha" suggestionsLoading={false} />)
    expect(screen.getByRole('link', { name: /Which identifiers occur most often/ }).getAttribute('href')).toBe('/cases/alpha/investigate?question=Which%20identifiers%20occur%20most%20often%3F')
    expect(screen.getByText(/Questions you ask here appear/)).toBeTruthy()
    rerender(<MemoryRouter><ContinueCards recent={[]} suggestions={[]} suggestionCase="" suggestionsLoading={false} /></MemoryRouter>)
    expect(screen.getByText(/Suggestions appear once a case has ready evidence/)).toBeTruthy()
  })
})

describe('case helpers', () => {
  it('floors ages and never shows a future time as fresh-looking negative', () => {
    expect(relativeAge(new Date(now - 30_000), now)).toBe('just now')
    expect(relativeAge(new Date(now - 59 * 60_000), now)).toBe('59 min ago')
    expect(relativeAge(new Date(now - 5 * 3_600_000), now)).toBe('5 h ago')
    expect(relativeAge(new Date(now - 5 * 86_400_000), now)).toBe('5 d ago')
    expect(relativeAge(new Date(now + 60_000), now)).toBe('just now')
  })

  it('finds the newest dated evidence or job, and none when nothing is dated', () => {
    expect(latestEvidenceActivity({ recent_evidence: [{ source_file: 'a', updated_at: '2026-09-01T00:00:00Z' }], recent_jobs: [{ source_file: 'b', completed_at: '2026-09-02T00:00:00Z' }] }).label).toBe('b')
    expect(latestEvidenceActivity({ recent_evidence: [{ source_file: 'a' }] })).toBeNull()
  })

  it('routes each state to a real next step and keeps engine talk out of suggested questions', () => {
    expect(dashboardNextAction({ processingState: 'complete' }, 'c')).toEqual({ label: 'Investigate', to: '/cases/c/investigate' })
    expect(curatedQuestions({ families: [{ id: 'cdr', availability: 'queryable' }], query_corpus: { entries: [{ family_id: 'cdr', query: 'Who?' }, { family_id: 'cdr', query: 'Run the model' }] } })).toEqual([{ familyId: 'cdr', query: 'Who?' }])
  })

  it('builds a 30-day series per case and leaves out cases that were not read', () => {
    const activity = mergeActivity([parseActivity({ records: { activity_by_day: [{ activity_date: '2026-09-02', record_type: 'cdr', event_count: 7 }] } }, 'alpha')])
    const sparks = caseSparks(activity)
    expect(Object.keys(sparks)).toEqual(['alpha'])
    expect(sparks.alpha.values).toHaveLength(30)
    expect(sparks.alpha.values.at(-1)).toBe(7)
    expect(caseSparks({ available: false })).toEqual({})
  })
})
