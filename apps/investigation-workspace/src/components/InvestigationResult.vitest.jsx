import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'
import { InvestigationResult } from './InvestigationResult.jsx'
import { presentInvestigationResponse } from '../lib/investigationPresentation.js'
import { clarificationFixture, goldenFixture, loadClarificationFixtures } from '../test/goldenFixtures.js'

describe('investigation result', () => {
  test.each(loadClarificationFixtures())('renders every server-authored option in $name', ({ response }) => {
    const view = presentInvestigationResponse(response, { caseId: response.collection_id })
    render(
      <InvestigationResult
        presentation={view}
        onAsk={() => {}}
        onClarificationChoice={() => {}}
        onRetry={() => {}}
      />,
    )
    expect(screen.getAllByRole('button')).toHaveLength(response.clarification.options.length)
    response.clarification.options.forEach(option => {
      const button = screen.getByRole('button', { name: new RegExp(option.label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')) })
      expect(button).toHaveTextContent(option.label)
      expect(button).toHaveTextContent(option.query)
    })
  })

  test('renders answer sections in the fixed order with semantic headers', () => {
    const view = presentInvestigationResponse(goldenFixture('CDR-04.json'), { caseId: 'nexusai-forensic-demo' })
    const { container } = render(<InvestigationResult presentation={view} onAsk={() => {}} onClarificationChoice={() => {}} onRetry={() => {}} />)
    expect(screen.getByText((_content, element) => element.classList?.contains('claim-text') && element.textContent.includes('8,642') && element.textContent.includes('CDR records'))).toBeInTheDocument()
    expect(container.querySelector('h1')).toBeNull()
    expect(screen.getByRole('columnheader', { name: 'Call type' })).toBeInTheDocument()
    expect(screen.getByRole('cell', { name: 'Call' })).toBeInTheDocument()
    const headings = [...container.querySelectorAll('.section-heading')].map(node => node.textContent)
    expect(headings.slice(0, 3)).toEqual(['The result', 'Citations', 'How this was derived'])
    expect(container.querySelectorAll('.answer-block .citation-marker')).toHaveLength(5)
    expect(container.querySelectorAll('.source-group').length).toBeGreaterThanOrEqual(2)
    expect(screen.getAllByText(/of 8,642 contributing rows/i).length).toBeGreaterThanOrEqual(2)
  })

  test('opens a rich exact-locator preview and dismisses it with Escape', async () => {
    const user = userEvent.setup()
    const view = presentInvestigationResponse(goldenFixture('CDR-04.json'), { caseId: 'nexusai-forensic-demo' })
    render(<InvestigationResult presentation={view} onAsk={() => {}} onClarificationChoice={() => {}} onRetry={() => {}} />)
    const marker = document.querySelector('.citation-marker')
    await user.tab()
    while (document.activeElement !== marker) await user.tab()
    expect(await screen.findByRole('tooltip')).toHaveTextContent('Exact locator')
    expect(screen.getByRole('tooltip')).toHaveTextContent(/row|hash/i)
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
  })

  test('marks displayed numbers as aligned numeric cells', () => {
    const view = presentInvestigationResponse(goldenFixture('CDR-04.json'), { caseId: 'nexusai-forensic-demo' })
    const { container } = render(<InvestigationResult presentation={view} onAsk={() => {}} onClarificationChoice={() => {}} onRetry={() => {}} />)
    expect(container.querySelectorAll('.table-cell--numeric').length).toBeGreaterThan(0)
  })

  test('renders abstention as a careful result and does not fabricate choices', () => {
    const view = presentInvestigationResponse(goldenFixture('CDR-07.json'), { caseId: 'nexusai-forensic-demo' })
    render(<InvestigationResult presentation={view} onAsk={() => {}} onClarificationChoice={() => {}} onRetry={() => {}} />)
    expect(screen.getByRole('heading', { name: 'I can answer this with one more detail' })).toBeInTheDocument()
    expect(screen.getByText(/required choices were not supplied/i)).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  test('one-click follow-up sends the server-provided query', async () => {
    const user = userEvent.setup()
    const onAsk = vi.fn()
    const view = presentInvestigationResponse(goldenFixture('IMG-01.json'), { caseId: 'nexusai-multimodal-product-acceptance' })
    render(<InvestigationResult presentation={view} onAsk={onAsk} onClarificationChoice={() => {}} onRetry={() => {}} />)
    const button = screen.getByRole('button', { name: /Audit source files/i })
    await user.click(button)
    expect(onAsk).toHaveBeenCalledWith('which files were ingested?')
  })

  test('shows the exact outgoing v2 question and sends the server option unchanged', async () => {
    const user = userEvent.setup()
    const onClarificationChoice = vi.fn()
    const view = presentInvestigationResponse(clarificationFixture('X-01.json'), {
      caseId: 'nexusai-multimodal-product-acceptance',
    })
    render(
      <InvestigationResult
        presentation={view}
        onAsk={() => {}}
        onClarificationChoice={onClarificationChoice}
        onRetry={() => {}}
      />,
    )
    const option = screen.getByRole('button', { name: /Documents mentioning 03001234567/i })
    expect(option).toHaveTextContent('Ask: “Which document mentions 03001234567?”')
    await user.click(option)
    expect(onClarificationChoice).toHaveBeenCalledWith(
      expect.objectContaining({
        label: 'Documents mentioning 03001234567',
        query: 'Which document mentions 03001234567?',
      }),
      view.clarification,
    )
  })

  test('renders a legacy string option as text, never as a dead button', () => {
    const view = presentInvestigationResponse({
      intent: 'clarification',
      query_understanding: { original_question: 'What is the total?' },
      clarification: {
        contract_version: 'forensics.clarification-request/v1',
        reason_code: 'missing_required_parameter',
        options: ['Call duration'],
      },
    })
    render(<InvestigationResult presentation={view} onAsk={() => {}} onClarificationChoice={() => {}} onRetry={() => {}} />)
    expect(screen.getByText('Call duration')).toBeInTheDocument()
    expect(screen.getByText(/does not include a question that can be sent/i)).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  test('renders an unsupported answer as a neutral notice, never as a supported finding', () => {
    const view = presentInvestigationResponse({
      intent: 'semantic',
      answer: { answer: 'This question is outside the evidence analysis currently available for this case.' },
    }, { caseId: 'nexusai-multimodal-product-acceptance' })
    const { container } = render(<InvestigationResult presentation={view} onAsk={() => {}} onClarificationChoice={() => {}} onRetry={() => {}} />)

    expect(screen.getByText('Analysis unavailable')).toBeInTheDocument()
    expect(screen.getByRole('note')).toHaveTextContent('outside the evidence analysis')
    expect(screen.queryByText('Supported finding')).not.toBeInTheDocument()
    expect(screen.queryByText(/0 sources/i)).not.toBeInTheDocument()
    expect(container.querySelector('.finding-card')).toBeNull()
  })

  test('stops after one automatic clarification round', () => {
    const view = presentInvestigationResponse(clarificationFixture('ANPR-02.json'), { caseId: 'nexusai-forensic-demo' })
    render(
      <InvestigationResult
        presentation={view}
        onAsk={() => {}}
        onClarificationChoice={() => {}}
        onRetry={() => {}}
        clarificationRounds={1}
      />,
    )
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.getByText(/No second automatic clarification was sent/i)).toBeInTheDocument()
    expect(screen.getByText(/How many vehicle plate sightings are there for each camera/i)).toBeInTheDocument()
  })
})
