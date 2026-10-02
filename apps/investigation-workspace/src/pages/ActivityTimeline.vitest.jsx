import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, test, vi } from 'vitest'
import { mergeActivity, parseActivity as parse } from '../lib/caseActivity.js'
import { clearSessionActivityForTests, recordSessionActivity } from '../lib/sessionActivity.js'
import { clearWorkspaceStateForTests, recordQuestionHistory } from '../lib/workspaceState.js'
import ActivityPage from './ActivityPage.jsx'
import TimelinePage, { timelineQuestions } from './TimelinePage.jsx'

const reading = { activity: mergeActivity([]) }
vi.mock('../lib/useCaseActivity.js', async importOriginal => ({ ...(await importOriginal()), useCaseActivity: () => ({ status: 'ready', activity: reading.activity, failures: [] }) }))

afterEach(() => {
  clearSessionActivityForTests()
  clearWorkspaceStateForTests()
  globalThis.localStorage?.clear()
})

function at(path, element) {
  return render(<MemoryRouter initialEntries={[path]}><Routes><Route path="/cases/:id/:view" element={element} /></Routes></MemoryRouter>)
}

describe('case activity', () => {
  test('keeps answered and clarification outcomes distinct, scoped, filterable, and reopenable', async () => {
    recordSessionActivity({
      caseId: 'case-1',
      query: 'Show the call type breakdown',
      presentation: { state: 'answered', answer: 'Three call types.', scope: [{ label: 'Evidence', value: 'CDR records' }], original: { query_plan: { applied_filters: { record_type: 'cdr' } } } },
    })
    recordSessionActivity({
      caseId: 'case-1',
      query: 'How many calls of each type are there?',
      presentation: { state: 'clarify', clarification: { title: 'I can answer this with one more detail', question: 'Choose call records or sessions.' }, original: { query_plan: { applied_filters: { record_type: 'cdr' } } } },
    })
    at('/cases/case-1/activity', <ActivityPage />)
    expect(screen.getAllByText('Answered', { exact: true }).length).toBeGreaterThan(0)
    expect(screen.getAllByText('Clarification requested', { exact: true }).length).toBeGreaterThan(0)
    expect(screen.getAllByText('case-1', { exact: true }).length).toBeGreaterThan(0)
    expect(screen.getAllByRole('link', { name: 'Reopen with this question' })[0]).toHaveAttribute('href', expect.stringContaining('question='))
    await userEvent.click(screen.getByRole('button', { name: /^Answered\s*1/ }))
    expect(screen.getByRole('list', { name: 'Activity' })).not.toHaveTextContent('Clarification requested')
  })

  test('keeps durable browser history distinct and never exposes technical planner details', () => {
    recordQuestionHistory('case-1', 'Find the busiest calling hour', { state: 'answered', answer: 'The saved answer.' })
    at('/cases/case-1/activity', <ActivityPage />)
    expect(screen.getByText('Saved in this browser')).toBeInTheDocument()
    expect(screen.getByText('Not retained with this saved entry')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Working record, not audit history' })).toBeInTheDocument()
    expect(screen.queryByText('Technical details')).not.toBeInTheDocument()
    expect(screen.queryByText('Request reference')).not.toBeInTheDocument()
  })
})

describe('case timeline', () => {
  test('reads the selected day from real record dates, and never uses ingestion dates', async () => {
    const rows = [
      { activity_date: '2026-01-30', record_type: 'cdr', event_count: 6 },
      { activity_date: '2026-01-30', record_type: 'anpr', event_count: 4 },
      { activity_date: '2026-02-01', record_type: 'cdr', event_count: 30 },
    ]
    reading.activity = mergeActivity([parse({ records: { activity_by_day: rows } }, 'case-1')])
    at('/cases/case-1/timeline', <TimelinePage />)
    expect(screen.getByRole('heading', { name: 'Timeline' })).toBeInTheDocument()
    expect(screen.getByRole('complementary', { name: 'Selected day' })).toHaveTextContent('30')
    expect(screen.getByText(/upload and processing dates are never used/i)).toBeInTheDocument()
    await userEvent.click(screen.getAllByRole('button', { name: /^2026-01-30/ })[0])
    expect(screen.getByRole('complementary', { name: 'Selected day' })).toHaveTextContent('10 events')
    expect(screen.getByRole('link', { name: /Investigate this day/ })).toHaveAttribute('href', expect.stringContaining('2026-01-30'))
  })

  test('projects only curated, available chronology questions from capabilities', () => {
    const projected = timelineQuestions({ families: [
      { id: 'cdr', label: 'CDR', availability: 'queryable', record_types: ['cdr'], suggested_queries: ['show communications timeline', 'count calls'] },
      { id: 'anpr', label: 'ANPR', availability: 'no_data', record_types: ['anpr'], suggested_queries: ['show camera sequence'] },
      { id: 'internal', label: 'Internal', availability: 'queryable', adapter: 'secret-adapter', deterministic_operations: ['secret'], suggested_queries: ['show history over time'] },
    ] })
    expect(projected).toEqual([
      { question: 'show communications timeline', family: 'CDR', scope: 'cdr' },
      { question: 'show history over time', family: 'Internal', scope: 'all' },
    ])
    expect(projected).not.toEqual(expect.arrayContaining([expect.objectContaining({ adapter: expect.anything() })]))
  })
})
