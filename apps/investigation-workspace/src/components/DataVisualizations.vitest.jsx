import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { MeterBar, ProportionBar, ReadinessDonut, StackedBar } from './DataVisualizations.jsx'

describe('data visualization primitives', () => {
  it('gives every readiness segment an exact text equivalent', () => {
    render(<ProportionBar total={12} ready={8} processing={3} failed={1} />)
    expect(screen.getByRole('img', { name: 'Evidence readiness: ready 8, processing 3, failed 1, other 0' })).toBeInTheDocument()
  })

  it('keeps family and ingestion quantities accessible without colour', () => {
    render(<><StackedBar items={[{ id: 'cdr', label: 'CDR', value: 8642 }, { id: 'anpr', label: 'ANPR', value: 420 }]} /><MeterBar accepted={8642} duplicate={299} rejected={7} /></>)
    expect(screen.getByRole('img', { name: 'Accepted rows by evidence family: CDR 8,642, ANPR 420' })).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Ingestion accounting: accepted 8,642, duplicate 299, rejected 7' })).toBeInTheDocument()
  })

  it('renders a real multi-segment readiness donut with the reported total', () => {
    render(<ReadinessDonut total={8} ready={5} processing={2} failed={1} label="Workspace readiness" />)
    expect(screen.getByRole('img', { name: 'Workspace readiness: ready 5, processing 2, failed 1, other 0' })).toBeInTheDocument()
    expect(screen.getByText('8')).toBeInTheDocument()
  })
})
