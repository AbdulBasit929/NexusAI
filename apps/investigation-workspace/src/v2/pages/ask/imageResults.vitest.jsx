import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'

vi.mock('../../../lib/apiClient.js', async importOriginal => {
  const actual = await importOriginal()
  return { ...actual, getEvidenceContent: vi.fn(async () => new Blob(['x'], { type: 'image/jpeg' })) }
})
import { searchImages } from '../../../lib/apiClient.js'
import { boxStyle, copyDetail, ImageResults, imageResultGroups, isImageSearchAnswer, relativeShare } from './ImageResults.jsx'
import { Composer } from './Composer.jsx'

const row = (id, kind, score, over = {}) => ({ id, preview: '', content: '', score, metadata: { evidence_id: id, source_file: `${id}.jpg`, match_kind: kind, source_family: 'image', ...over } })
const answer = rows => ({ route: ['image_search'], evidence: { results: rows } })

beforeEach(() => {
  globalThis.URL.createObjectURL = vi.fn(() => 'blob:thumb')
  globalThis.URL.revokeObjectURL = vi.fn()
})
afterEach(() => cleanup())

describe('the groups of an image answer', () => {
  it('recognises an image search answer by its route and nothing else', () => {
    expect(isImageSearchAnswer({ original: answer([]) })).toBe(true)
    expect(isImageSearchAnswer({ original: { route: ['plate_image_search'] } })).toBe(false)
    expect(isImageSearchAnswer(null)).toBe(false)
  })
  it('keeps copies, similar pictures and ranked matches apart, in the order the service gave them', () => {
    const groups = imageResultGroups(answer([row('a', 'identical_file', 1), row('b', 'same_image', 1), row('c', 'same_photograph', 0.9), row('d', 'similar', 0.8), row('e', 'similar', 0.5), row('f', 'ranked', 0.3)]))
    expect(groups.copies.map(r => r.id)).toEqual(['a', 'b', 'c'])
    expect(groups.similar.map(r => r.id)).toEqual(['d', 'e'])
    expect(groups.ranked.map(r => r.id)).toEqual(['f'])
  })
  it('says how a copy matched in words, and draws a bar only relative to the best of its list', () => {
    expect(copyDetail({ kind: 'identical_file', metadata: {} })).toMatch(/very same file/)
    expect(copyDetail({ kind: 'same_image', metadata: { hash_distance_bits: 7 } })).toMatch(/differs by 7 of 256 bits/)
    expect(relativeShare(0.5, 1)).toBe(50)
    expect(relativeShare(1, 1)).toBe(100)
    expect(relativeShare(0, 1)).toBe(0)
    expect(relativeShare(0.001, 1)).toBe(4)
  })
})

describe('the grid', () => {
  it('shows each group under its own heading with the real thumbnails, labels and a link to the evidence', async () => {
    const presentation = { original: answer([row('e1', 'identical_file', 1), row('e2', 'same_photograph', 0.9), row('e3', 'similar', 0.7), row('e4', 'similar', 0.35)]) }
    render(<MemoryRouter><ImageResults presentation={presentation} caseId="case-a" /></MemoryRouter>)
    const copies = screen.getByRole('region', { name: 'Exact and near copies' })
    expect(within(copies).getByText('Identical file')).toBeTruthy()
    expect(within(copies).getByText('Same photograph, lightly edited')).toBeTruthy()
    const similar = screen.getByRole('region', { name: 'Visually similar' })
    expect(within(similar).getAllByText('Visually similar').length).toBeGreaterThan(0)
    expect(screen.queryByRole('region', { name: 'Most likely matches' })).toBeNull()
    await waitFor(() => expect(screen.getAllByRole('img', { name: /\.jpg$/ }).length).toBe(4))
    const link = screen.getByRole('link', { name: /e1\.jpg: Identical file/ })
    expect(link.getAttribute('href')).toBe('/cases/case-a/evidence/e1')
    // the second similar picture is shown at 50 percent of the best one, not as a made-up score
    expect(within(similar).getByRole('img', { name: '50 percent of the best match in this list' })).toBeTruthy()
  })
  it('says so when nothing matched', () => {
    render(<MemoryRouter><ImageResults presentation={{ original: answer([]) }} caseId="case-a" /></MemoryRouter>)
    expect(screen.getByText('No picture matched.')).toBeTruthy()
  })
})

describe('the reference image in the question box', () => {
  const file = new File(['png'], 'reference.png', { type: 'image/png' })
  it('attaches an image from the picker, and refuses a file that is not an image', () => {
    const onAttach = vi.fn()
    const { container } = render(<Composer value="" onChange={() => {}} onSubmit={() => {}} onAttach={onAttach} onClearAttachment={() => {}} scope="all" onScope={() => {}} />)
    const input = container.querySelector('input[type=file]')
    fireEvent.change(input, { target: { files: [new File(['x'], 'notes.txt', { type: 'text/plain' })] } })
    expect(onAttach).not.toHaveBeenCalled()
    fireEvent.change(input, { target: { files: [file] } })
    expect(onAttach).toHaveBeenCalledWith(file)
  })
  it('lets the picture alone be asked: the chip names the file and Ask works with no words', () => {
    const onSubmit = vi.fn()
    const onClear = vi.fn()
    render(<Composer value="" onChange={() => {}} onSubmit={onSubmit} attachment={file} onAttach={() => {}} onClearAttachment={onClear} scope="all" onScope={() => {}} />)
    expect(screen.getByRole('group', { name: 'Reference image' }).textContent).toContain('reference.png')
    const ask = screen.getByRole('button', { name: 'Ask' })
    expect(ask.disabled).toBe(false)
    fireEvent.click(ask)
    expect(onSubmit).toHaveBeenCalledWith('', file)
    fireEvent.click(screen.getByRole('button', { name: 'Remove the reference image' }))
    expect(onClear).toHaveBeenCalled()
  })
  it('without an attachment the box asks as it always did, and shows no attach button when none is offered', () => {
    const onSubmit = vi.fn()
    const { rerender } = render(<Composer value="who called the most" onChange={() => {}} onSubmit={onSubmit} scope="all" onScope={() => {}} />)
    expect(screen.queryByRole('button', { name: /Attach a reference image/ })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Ask' }))
    expect(onSubmit).toHaveBeenCalledWith('who called the most', null)
    rerender(<Composer value="" onChange={() => {}} onSubmit={onSubmit} scope="all" onScope={() => {}} />)
    expect(screen.getByRole('button', { name: 'Ask' }).disabled).toBe(true)
  })
})

describe('the search call', () => {
  const configure = () => { globalThis.window.__INVESTIGATION_WORKSPACE_CONFIG__ = { collectionId: 'col-1', tenantId: 'ten-1', apiBase: '' } }
  afterEach(() => { delete globalThis.window.__INVESTIGATION_WORKSPACE_CONFIG__ })
  const ok = body => ({ ok: true, status: 200, headers: { get: () => null }, json: async () => body })
  it('sends a dropped reference image as multipart with the scope and asks for an Ask-shaped answer', async () => {
    configure()
    const fetchImpl = vi.fn(async () => ok({ route: ['image_search'] }))
    const reply = await searchImages({ caseId: 'col-1', file: new File(['png'], 'ref.png', { type: 'image/png' }), fetchImpl })
    expect(reply.route).toEqual(['image_search'])
    const [url, init] = fetchImpl.mock.calls[0]
    expect(init.method).toBe('POST')
    expect(init.body instanceof FormData).toBe(true)
    expect(init.body.get('image').name).toBe('ref.png')
    expect(init.headers['Content-Type']).toBeUndefined()
    const query = new URL(String(url)).searchParams
    expect(query.get('kind')).toBe('reference_upload')
    expect(query.get('format')).toBe('ask')
    expect(query.get('collection_id')).toBeTruthy()
  })
  it('sends words as a plain GET and reports the service message and status on a refusal', async () => {
    configure()
    const fetchImpl = vi.fn(async () => ({ ok: false, status: 409, headers: { get: () => 'req-1' }, json: async () => ({ error: 'that image has not been processed for copy search yet' }) }))
    await expect(searchImages({ caseId: 'col-1', kind: 'by_words', text: 'a red car', topK: 5, fetchImpl })).rejects.toMatchObject({ status: 409, reference: 'req-1', message: 'that image has not been processed for copy search yet' })
    const query = new URL(String(fetchImpl.mock.calls[0][0])).searchParams
    expect([query.get('kind'), query.get('q'), query.get('top_k')]).toEqual(['by_words', 'a red car', '5'])
    expect(fetchImpl.mock.calls[0][1].method).toBe('GET')
  })
})

describe('where a matched region sits', () => {
  it('places the box inside the picture as it is shown in the 4:3 frame, letterboxed or not', () => {
    // a wide picture fills the width of the frame and is centred vertically
    expect(boxStyle([0, 0, 0.5, 0.5], { w: 800, h: 400 })).toEqual({ left: '0%', top: '16.67%', width: '50%', height: '33.33%' })
    // a tall picture fills the height and is centred horizontally
    expect(boxStyle([0.5, 0, 1, 1], { w: 300, h: 600 })).toEqual({ left: '50%', top: '0%', width: '18.75%', height: '100%' })
    // a picture of exactly the frame's shape
    expect(boxStyle([0.25, 0.25, 0.75, 0.75], { w: 400, h: 300 })).toEqual({ left: '25%', top: '25%', width: '50%', height: '50%' })
  })

  it('draws nothing for a missing, malformed or empty box or an unknown picture size', () => {
    expect(boxStyle(null, { w: 10, h: 10 })).toBeNull()
    expect(boxStyle([0, 0, 1], { w: 10, h: 10 })).toBeNull()
    expect(boxStyle([0.5, 0.5, 0.5, 0.9], { w: 10, h: 10 })).toBeNull()
    expect(boxStyle([0, 0, 1, 1], null)).toBeNull()
  })

  it('keeps the pictures that contain the attached one apart from those with only a similar part', () => {
    const rows = ['region_verified', 'region_probable', 'similar_region', 'similar', 'ranked'].map((kind, i) => ({ id: `r${i}`, score: 1, metadata: { match_kind: kind, evidence_id: `r${i}` } }))
    const groups = imageResultGroups({ evidence: { results: rows } })
    expect(groups.contains.map(r => r.id)).toEqual(['r0', 'r1'])
    expect(groups.parts.map(r => r.id)).toEqual(['r2'])
    expect(groups.similar.map(r => r.id)).toEqual(['r3'])
    expect(groups.ranked.map(r => r.id)).toEqual(['r4'])
    expect(copyDetail({ kind: 'region_verified', metadata: { region_inliers: 31 } })).toMatch(/31 matching points agree/)
    expect(copyDetail({ kind: 'region_probable', metadata: { region_inliers: 9 } })).toMatch(/probable, not certain/)
    expect(copyDetail({ kind: 'similar_region', metadata: {} })).toMatch(/Not verified/)
  })
})

describe('stacks, tiers and one-click actions', () => {
  const stacked = [
    row('a', 'duplicate_lead', 1, { dup_group: 1, dup_group_size: 3 }), row('b', 'same_image', 1, { dup_group: 1, dup_group_size: 3, hash_distance_bits: 12 }), row('c', 'linked_copy', 1, { dup_group: 1, dup_group_size: 3 }),
    row('d', 'duplicate_lead', 1, { dup_group: 2, dup_group_size: 2 }), row('e', 'identical_file', 1, { dup_group: 2, dup_group_size: 2 }),
  ]

  it('collects duplicate groups by their number, each as one stack, and leaves the other rows loose', () => {
    const groups = imageResultGroups(answer([...stacked, row('x', 'ranked', 0.1), row('y', 'possible', 0.09)]))
    expect(groups.stacks.map(s => [s.number, s.size, s.rows.map(r => r.id).join('')])).toEqual([[1, 3, 'abc'], [2, 2, 'de']])
    expect(groups.ranked.map(r => r.id)).toEqual(['x'])
    expect(groups.possible.map(r => r.id)).toEqual(['y'])
    expect(groups.copies).toEqual([])
  })

  it('shows each stack under its own heading and the weaker words matches apart from the best', () => {
    const presentation = { original: answer([...stacked, row('x', 'ranked', 0.12), row('y', 'possible', 0.1)]) }
    render(<MemoryRouter><ImageResults presentation={presentation} caseId="case-a" /></MemoryRouter>)
    const first = screen.getByRole('region', { name: 'Group 1' })
    expect(within(first).getByText('Stands for the group')).toBeTruthy()
    expect(within(first).getByText('Linked through other copies')).toBeTruthy()
    expect(within(screen.getByRole('region', { name: 'Group 2' })).getByText('Identical file')).toBeTruthy()
    expect(within(screen.getByRole('region', { name: 'Best matches' })).getByText('Rank 1')).toBeTruthy()
    expect(within(screen.getByRole('region', { name: 'Also possible' })).getAllByText('Also possible').length).toBe(2) // the heading and the label of the one picture
  })

  it('offers more-like-this and find-copies on a picture and hands its id and name to the page', () => {
    const onAction = vi.fn()
    render(<MemoryRouter><ImageResults presentation={{ original: answer([row('e1', 'ranked', 0.1)]) }} caseId="case-a" onAction={onAction} /></MemoryRouter>)
    fireEvent.click(screen.getByRole('button', { name: 'Pictures that look like e1.jpg' }))
    expect(onAction).toHaveBeenCalledWith('more_like_this', 'e1', 'e1.jpg')
    fireEvent.click(screen.getByRole('button', { name: 'Copies of e1.jpg' }))
    expect(onAction).toHaveBeenCalledWith('reference', 'e1', 'e1.jpg')
  })

  it('shows no action buttons when the page offers none', () => {
    render(<MemoryRouter><ImageResults presentation={{ original: answer([row('e1', 'ranked', 0.1)]) }} caseId="case-a" /></MemoryRouter>)
    expect(screen.queryByRole('button', { name: /Pictures that look like/ })).toBeNull()
  })
})

describe('pictures that show it only in a part', () => {
  it('keeps them in their own group, outlined, apart from the ranked pictures', () => {
    const rows = [row('x', 'ranked', 0.12), row('y', 'in_part', 0.1, { region_box: [0.1, 0.1, 0.6, 0.6] })]
    expect(imageResultGroups(answer(rows)).inPart.map(r => r.id)).toEqual(['y'])
    expect(imageResultGroups(answer(rows)).ranked.map(r => r.id)).toEqual(['x'])
    render(<MemoryRouter><ImageResults presentation={{ original: answer(rows) }} caseId="case-a" /></MemoryRouter>)
    const region = screen.getByRole('region', { name: 'In a part of a picture' })
    expect(within(region).getAllByText('In a part of it').length).toBe(1)
    expect(copyDetail({ kind: 'in_part', metadata: {} })).toMatch(/may show what you described/)
  })
})
