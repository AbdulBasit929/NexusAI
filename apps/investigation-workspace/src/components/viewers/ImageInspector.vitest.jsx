import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { ImageInspector } from './ImageInspector.jsx'

// jsdom has no layout and, in some versions, no PointerEvent that carries coordinates
beforeEach(() => {
  if (!globalThis.PointerEvent) globalThis.PointerEvent = class extends MouseEvent {}
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({ left: 100, top: 100, width: 400, height: 200, right: 500, bottom: 300, x: 100, y: 100, toJSON() {} })
})
afterEach(() => vi.restoreAllMocks())

function Landing() {
  const location = useLocation()
  return <p data-testid="landing">{location.pathname}{location.search}</p>
}

function show(path = '/cases/case-9/evidence/ev-1') {
  const detail = { item: { evidence_id: 'ev-1', original_filename: 'laptop.jpg' }, records: [] }
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/cases/:id/evidence/:eid" element={<ImageInspector detail={detail} objectUrl="blob:x" />} />
        <Route path="/cases/:id/investigate" element={<Landing />} />
      </Routes>
    </MemoryRouter>
  )
}

describe('searching for a part of a picture', () => {
  test('a box drawn on the picture is taken to the case Ask page, whose route names the case with :id', () => {
    show()
    fireEvent.click(screen.getByRole('button', { name: /find this part elsewhere/i }))
    expect(screen.queryByRole('button', { name: /search for this part/i })).toBeNull()
    const stage = screen.getByRole('group', { name: /image/i })
    fireEvent.pointerDown(stage, { clientX: 140, clientY: 120 })
    fireEvent.pointerMove(stage, { clientX: 300, clientY: 220 })
    fireEvent.pointerUp(stage)
    const go = screen.getByRole('button', { name: /search for this part/i })
    expect(go).not.toBeDisabled()
    fireEvent.click(go)
    expect(screen.getByTestId('landing')).toHaveTextContent('/cases/case-9/investigate?regionOf=ev-1&box=0.1,0.1,0.5,0.6&run=1')
  })

  test('a drag too small to be a part does not offer the search', () => {
    show()
    fireEvent.click(screen.getByRole('button', { name: /find this part elsewhere/i }))
    const stage = screen.getByRole('group', { name: /image/i })
    fireEvent.pointerDown(stage, { clientX: 140, clientY: 120 })
    fireEvent.pointerMove(stage, { clientX: 141, clientY: 121 })
    fireEvent.pointerUp(stage)
    expect(screen.queryByRole('button', { name: /search for this part/i })).toBeNull()
  })

  test('outside a router the picture is only inspected, with no part search', () => {
    const detail = { item: { evidence_id: 'ev-1', original_filename: 'laptop.jpg' }, records: [] }
    render(<ImageInspector detail={detail} objectUrl="blob:x" />)
    expect(screen.queryByRole('button', { name: /find this part elsewhere/i })).toBeNull()
  })
})
