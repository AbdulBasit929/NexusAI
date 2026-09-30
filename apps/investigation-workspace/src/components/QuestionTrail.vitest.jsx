import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test } from 'vitest'
import { QuestionTrail } from './QuestionTrail.jsx'

describe('question trail', () => {
  test('retains the original question, selected label, and exact sent query', () => {
    render(<QuestionTrail entries={[{
      originalQuestion: 'Where does 03001234567 appear across all evidence?',
      label: 'Documents mentioning 03001234567',
      query: 'Which document mentions 03001234567?',
    }]} />)
    expect(screen.getByText((_, element) => element?.tagName === 'P' && element.textContent.includes('Where does 03001234567 appear across all evidence'))).toBeInTheDocument()
    expect(screen.getByText((_, element) => element?.classList.contains('language-text') && element.textContent === 'Documents mentioning 03001234567')).toBeInTheDocument()
    expect(screen.getByText((_, element) => element?.tagName === 'P' && element.textContent.includes('Which document mentions 03001234567'))).toBeInTheDocument()
  })

  test('keeps prior questions collapsed until the analyst opens them', async () => {
    const user = userEvent.setup()
    render(<QuestionTrail history={[{ id: 'q-1', query: 'Where was this number seen?', state: 'answered', pinned: false }]} />)
    const summary = screen.getByText('Question history').closest('summary')
    expect(summary.closest('details')).not.toHaveAttribute('open')
    expect(screen.getByRole('button', { name: 'Re-run' })).not.toBeVisible()
    await user.click(summary)
    expect(summary.closest('details')).toHaveAttribute('open')
    expect(screen.getByRole('button', { name: 'Re-run' })).toBeVisible()
  })
})
