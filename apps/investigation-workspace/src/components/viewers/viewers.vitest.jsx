import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import { DocumentReader } from './DocumentReader.jsx'
import { TranscriptPanel } from './TranscriptPanel.jsx'
import { MediaPlayer } from './MediaPlayer.jsx'
import { SpeechBanner } from './SpeechBanner.jsx'
import { VideoPlates } from './VideoPlates.jsx'

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
    expect(screen.getByRole('status')).toHaveTextContent('1 of 2 segments match')
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

describe('transcript panel of a read recording', () => {
  const cues = [
    { id: 'a', start: 0, end: 33.13, text: 'first', language: 'ps', status: 'SEARCH_LEAD' },
    { id: 'b', start: 33.13, end: 38.96, text: 'second', language: 'ps', status: 'LOW_RESOURCE_LANGUAGE' },
  ]
  test('shows the span of each segment, tags only a segment weaker than the recording and says where a reading stopped', () => {
    render(<TranscriptPanel cues={cues} time={0} onSeek={() => {}} overall="SEARCH_LEAD" stopAt={38.96} />)
    expect(screen.getByText('0:00–0:33')).toBeInTheDocument()
    expect(screen.getByText('0:33–0:38')).toBeInTheDocument()
    expect(screen.getByText('Pointer to listen')).toBeInTheDocument()
    expect(screen.queryByText('Search lead')).toBeNull()
    expect(screen.getByText(/Reading stopped at 0:38/)).toBeInTheDocument()
    expect(screen.getByText('first')).toHaveAttribute('lang', 'ps')
  })
})

describe('segment bands of the player', () => {
  test('are buttons that seek to the segment and carry a spoken label', () => {
    render(<MediaPlayer kind="audio" src="blob:x" label="Audio player" durationHint={40} bands={[{ id: 's1', start: 0, end: 33, tone: 'caution', active: true, label: 'Segment 1, 0:00 to 0:33, Pashto' }]} />)
    expect(screen.getByRole('button', { name: 'Segment 1, 0:00 to 0:33, Pashto' })).toBeInTheDocument()
  })
})

describe('speech banner', () => {
  test('says what the text is and carries only the figures that were measured', () => {
    render(<SpeechBanner model={{ status: 'SEARCH_LEAD', language: 'ps', measured: { word_error: 0.303, word_recall: 0.649, tier: 'public' }, complete: true, readUntil: null }} />)
    expect(screen.getByRole('note')).toHaveTextContent('Search lead')
    expect(screen.getByRole('note')).toHaveTextContent('About 3 words in 10 are wrong · 65% of spoken words are found · Measured on public speech')
    expect(screen.getByRole('note')).toHaveTextContent('Pashto')
  })
})

describe('plates of a video', () => {
  const group = { id: 'g', plate: 'AB1234', first: 2, last: 3, count: 24, confidence: 0.95, reads: [{ id: 'r1', plate: 'AB1234', time: 2, confidence: 0.9, box: [1, 2, 3, 4] }, { id: 'r2', plate: 'A81234', time: 2.5, confidence: 0.6, box: [1, 2, 3, 4] }] }
  test('lists each plate with its span and frames, goes to the first second on click and shows the characters that differ in a reading', () => {
    const onSeek = vi.fn()
    render(<VideoPlates groups={[group]} texts={[]} time={0} onSeek={onSeek} media={{ current: null }} />)
    expect(screen.getByText('0:02–0:03')).toBeInTheDocument()
    expect(screen.getByText('24 frames')).toBeInTheDocument()
    expect(screen.getByText(/Play or step to a plate/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Plate AB1234, seen from 0:02 to 0:03/ }))
    expect(onSeek).toHaveBeenCalledWith(2)
    expect(document.querySelector('.mx-reads b.is-diff').textContent).toBe('8')
  })
  test('marks the plate on the screen at the playing second', () => {
    render(<VideoPlates groups={[group]} texts={[]} time={2.2} onSeek={() => {}} media={{ current: null }} />)
    expect(document.querySelector('.mx-item[aria-current="true"]')).not.toBeNull()
  })
})
