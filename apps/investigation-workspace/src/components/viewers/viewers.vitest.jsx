import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import { DocumentReader } from './DocumentReader.jsx'
import { TranscriptPanel } from './TranscriptPanel.jsx'
import { MediaPlayer } from './MediaPlayer.jsx'

const detail = { item: { metadata: { pages: [{ page: 1, text: 'Call at noon. The call ended.' }, { page: 2, text: 'Nothing here' }, { page: 3, text: 'A final call' }] } }, derived_artifacts: [] }

describe('document reader', () => {
  test('finds words across pages, counts them and jumps between pages', () => {
    const onPageChange = vi.fn()
    render(<DocumentReader detail={detail} page="1" onPageChange={onPageChange} />)
    fireEvent.change(screen.getByLabelText('Find in document'), { target: { value: 'call' } })
    expect(screen.getByRole('status')).toHaveTextContent('1 of 2 on this page · 3 in document')
    fireEvent.click(screen.getByRole('button', { name: 'Next match' }))
    expect(screen.getByRole('status')).toHaveTextContent('2 of 2')
    fireEvent.click(screen.getByRole('button', { name: /Page 3/ }))
    expect(onPageChange).toHaveBeenCalledWith(3)
  })
  test('says so when nothing matches', () => {
    render(<DocumentReader detail={detail} page="1" onPageChange={() => {}} />)
    fireEvent.change(screen.getByLabelText('Find in document'), { target: { value: 'zzz' } })
    expect(screen.getByRole('status')).toHaveTextContent('No matches')
  })
})

describe('transcript panel', () => {
  const cues = [{ id: 'a', start: 0, end: 2, text: 'Hello there', speaker: 'A' }, { id: 'b', start: 2, end: 4, text: 'Goodbye now', speaker: 'B' }]
  test('marks the spoken cue, seeks on click and filters by search', () => {
    const onSeek = vi.fn()
    render(<TranscriptPanel cues={cues} time={2.5} onSeek={onSeek} />)
    expect(screen.getByText('Goodbye now').closest('li')).toHaveAttribute('aria-current', 'true')
    fireEvent.click(screen.getByText('Hello there'))
    expect(onSeek).toHaveBeenCalledWith(0)
    fireEvent.change(screen.getByLabelText('Search the transcript'), { target: { value: 'good' } })
    expect(screen.getByRole('status')).toHaveTextContent('1 of 2 cues match')
    expect(screen.queryByText('Hello there')).toBeNull()
  })
})

describe('media player', () => {
  test('offers labelled controls and marker buttons', () => {
    render(<MediaPlayer kind="video" src="blob:x" label="Video player" durationHint={100} markers={[{ id: 'm', time: 30, label: 'Vehicle seen' }]} />)
    for (const name of ['Play', 'Back 10 seconds', 'Forward 10 seconds', 'Mute', 'Full screen', 'Seek']) expect(screen.getByLabelText(name)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Vehicle seen at 30 seconds' })).toBeInTheDocument()
  })
})
