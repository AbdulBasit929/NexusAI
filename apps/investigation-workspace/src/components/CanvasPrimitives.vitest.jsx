import { describe, expect, it } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { Card } from './Card.jsx'
import { EmptyState, RouteState } from './AnalystComponents.jsx'
import { PageHeader } from './PageHeader.jsx'
import { ShellSkeleton, Skeleton, SkeletonCards } from './Skeleton.jsx'

const inRouter = ui => render(<MemoryRouter>{ui}</MemoryRouter>)

describe('PageHeader', () => {
  it('has one h1 with the description and actions, and never renders an eyebrow', () => {
    inRouter(<PageHeader eyebrow="Should not appear" title="Evidence" description="Find a source." actions={<a href="/x">Add evidence</a>} />)
    expect(screen.getByRole('heading', { level: 1, name: 'Evidence' })).toBeTruthy()
    expect(screen.getByText('Find a source.')).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Add evidence' })).toBeTruthy()
    expect(screen.queryByText('Should not appear')).toBeNull()
  })

  it('offers a labelled page trail that does not clash with the case breadcrumb, marking the current page', () => {
    inRouter(<PageHeader title="Source" breadcrumbs={[{ label: 'Evidence', to: '/cases/a/evidence' }, { label: 'Source' }]} />)
    const trail = screen.getByRole('navigation', { name: 'Page trail' })
    expect(within(trail).getByRole('link', { name: 'Evidence' }).getAttribute('href')).toBe('/cases/a/evidence')
    expect(trail.querySelector('[aria-current="page"]').textContent).toBe('Source')
  })

  it('renders facts as a definition list and skips the trail and facts when there are none', () => {
    const { container } = inRouter(<PageHeader title="Cases" meta={[{ label: 'Scope', value: 'One case' }, null]} />)
    expect(container.querySelectorAll('dl > div')).toHaveLength(1)
    expect(container.querySelector('nav')).toBeNull()
  })
})

describe('Card', () => {
  it('is a region named by its title, with description, actions and footer', () => {
    render(<Card title="Is each case ready?" description="Share of sources." actions={<button type="button">Refresh</button>} footer="53 of 55 ready.">Body</Card>)
    const region = screen.getByRole('region', { name: 'Is each case ready?' })
    expect(within(region).getByText('Share of sources.')).toBeTruthy()
    expect(within(region).getByRole('button', { name: 'Refresh' })).toBeTruthy()
    expect(region.textContent).toContain('53 of 55 ready.')
  })

  it('honours the heading level and keeps tone as a class only', () => {
    render(<Card title="Needs review" tone="critical" level={3}>Body</Card>)
    expect(screen.getByRole('heading', { level: 3, name: 'Needs review' })).toBeTruthy()
    expect(screen.getByRole('region').className).toContain('card--critical')
  })
})

describe('skeletons', () => {
  it('announce one status message and hide the shapes from assistive technology', () => {
    const { container } = render(<Skeleton lines={3} label="Loading evidence" />)
    expect(screen.getByRole('status').textContent).toBe('Loading evidence')
    expect(screen.getByRole('status').getAttribute('aria-busy')).toBe('true')
    expect(container.querySelector('[aria-hidden="true"]').querySelectorAll('.skeleton__line')).toHaveLength(3)
  })

  it('draw the requested number of cards and the shell placeholder without any number', () => {
    const { container } = render(<><SkeletonCards count={4} label="Loading cards" /><ShellSkeleton /></>)
    expect(container.querySelectorAll('.skeleton-cards .skeleton-card')).toHaveLength(4)
    expect(screen.getByText('Opening investigation')).toBeTruthy()
    expect(container.textContent).not.toMatch(/\d/)
  })
})

describe('state panels', () => {
  it('give each state a tone, a heading and a description in words', () => {
    const { container } = inRouter(<><EmptyState kind="not-processed" /><EmptyState kind="complete-zero" /><RouteState state="error" reference="REF-1" /><RouteState state="forbidden" /></>)
    const tones = [...container.querySelectorAll('.state-panel')].map(panel => panel.className.match(/state-panel--(\w+)/)[1])
    expect(tones).toEqual(['caution', 'positive', 'critical', 'neutral'])
    expect(screen.getByRole('heading', { name: 'Evidence not processed' })).toBeTruthy()
    expect(screen.getByRole('heading', { name: 'Access denied' })).toBeTruthy()
    expect(screen.getByRole('alert').textContent).toContain('REF-1')
  })

  it('shows a loading state as a busy status with decorative placeholder lines', () => {
    const { container } = inRouter(<RouteState state="loading" label="Reading the case status" />)
    const panel = screen.getByRole('status')
    expect(panel.getAttribute('aria-busy')).toBe('true')
    expect(container.querySelector('.state-panel__skeleton').getAttribute('aria-hidden')).toBe('true')
  })

  it('places actions inside the panel and renders none when there are none', () => {
    const { container } = inRouter(<><EmptyState kind="no-match"><button type="button">Clear filters</button></EmptyState><EmptyState kind="unavailable" /></>)
    expect(container.querySelectorAll('.state-panel__actions')).toHaveLength(1)
    expect(screen.getByRole('button', { name: 'Clear filters' })).toBeTruthy()
  })
})
