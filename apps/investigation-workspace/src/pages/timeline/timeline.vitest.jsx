import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, test, vi } from 'vitest'
import { EventStream } from './EventStream.jsx'
import { FamilyStrip, RhythmPanel, SavedPanel } from './FamilyRail.jsx'
import { Inspector } from './Inspector.jsx'

const days = [{ date: '2026-02-01', total: 30, byFamily: { cdr: 30 } }, { date: '2026-02-02', total: 10, byFamily: { cdr: 4, anpr: 6 } }]
const palette = ['#111', '#222', '#333']
const wrap = ui => render(<MemoryRouter>{ui}</MemoryRouter>)

describe('timeline workbench parts', () => {
  test('family filters report the choice and offer a clear', () => {
    const onToggle = vi.fn(); const onClear = vi.fn()
    render(<FamilyStrip all={[{ id: 'cdr', total: 34 }, { id: 'anpr', total: 6 }]} days={days} order={['anpr', 'cdr']} palette={palette} chosen={['cdr']} onToggle={onToggle} onClear={onClear} totals={40} />)
    expect(screen.getByText('1 chosen')).toBeInTheDocument()
    fireEvent.click(screen.getAllByRole('button', { pressed: false })[0])
    expect(onToggle).toHaveBeenCalledWith('anpr')
    fireEvent.click(screen.getByRole('button', { name: 'Clear' }))
    expect(onClear).toHaveBeenCalled()
  })

  test('weekly rhythm and saved days read real values', () => {
    render(<><RhythmPanel days={days} /><SavedPanel pins={[{ date: '2026-02-01', note: 'Key call' }]} onOpenPin={() => {}} onRemovePin={() => {}} /></>)
    expect(screen.getByText(/Sun: 30 events, 75%/)).toBeInTheDocument()
    expect(screen.getByText('Key call')).toBeInTheDocument()
  })

  test('says events inside a day are not available yet, and lists them when supplied', () => {
    const { unmount } = wrap(<EventStream caseId="c1" bucket={{ date: '2026-02-01' }} />)
    expect(screen.getByText(/not available yet/i)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Ask Investigate about this day/ })).toHaveAttribute('href', expect.stringContaining('2026-02-01'))
    unmount()
    wrap(<EventStream caseId="c1" bucket={{ date: '2026-02-01' }} events={[{ id: 'e1', time: '2026-02-01T08:42:10Z', family: 'cdr', summary: 'Call between two numbers' }]} />)
    expect(screen.getByText('08:42:10')).toBeInTheDocument()
    expect(screen.getByText('Call between two numbers')).toBeInTheDocument()
  })

  test('the inspector pins a day, shows its note box only once pinned, and switches tabs by keyboard', () => {
    const onPin = vi.fn()
    const props = { caseId: 'c1', bucket: { ...days[1], topCase: 'c1' }, typical: 20, order: ['anpr', 'cdr'], palette, position: 1, count: 2, onStep: () => {}, questions: [], onPin, onNote: () => {}, link: () => '/x' }
    const { rerender } = wrap(<Inspector {...props} pinned={false} note="" />)
    expect(screen.queryByLabelText(/Note on this day/)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Pin 2026-02-02' }))
    expect(onPin).toHaveBeenCalled()
    rerender(<MemoryRouter><Inspector {...props} pinned note="why" /></MemoryRouter>)
    expect(screen.getByLabelText(/Note on this day/)).toHaveValue('why')
    fireEvent.keyDown(screen.getByRole('tab', { name: 'Summary' }), { key: 'ArrowRight' })
    expect(screen.getByRole('tab', { name: 'Events' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByText(/not available yet/i)).toBeInTheDocument()
  })
})
