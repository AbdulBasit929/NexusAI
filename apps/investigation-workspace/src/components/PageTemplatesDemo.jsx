import { Plus } from 'lucide-react'
import { Card } from './Card.jsx'
import { EmptyState, RouteState } from './AnalystComponents.jsx'
import { PageHeader } from './PageHeader.jsx'
import { Skeleton, SkeletonCards } from './Skeleton.jsx'

// Design-system demonstration content only. The words describe the component; no figure here is case data.
export function PageTemplatesDemo() {
  return (
    <section className="gallery-section" aria-labelledby="gallery-page-title">
      <div className="gallery-section__heading"><div><span className="eyebrow">Canvas</span><h2 id="gallery-page-title">Page header, cards and states</h2></div><p>One header anatomy, one card, one state-panel system, and skeletons that hold the layout.</p></div>

      <div className="template-demo">
        <span className="gallery-label">Page header: trail, title and description, action, facts</span>
        <PageHeader
          breadcrumbs={[{ label: 'Section', to: '/' }, { label: 'This page' }]}
          title="Page title says what the page is"
          description="One short sentence on what the analyst can do here."
          actions={<a className="page-header__cta" href="#gallery-page-title"><Plus aria-hidden="true" />Primary action</a>}
          meta={[{ label: 'Scope', value: 'One case' }, { label: 'Updated', value: 'Just now' }]}
        />
      </div>

      <div className="template-demo template-demo--grid">
        <Card title="A card names its question" description="Optional one-line description." actions={<a className="page-header__secondary" href="#gallery-page-title">Action</a>} footer="Footer holds the source and coverage note.">
          <p>The body holds the content: a figure, a chart or a table.</p>
        </Card>
        <Card title="Attention tone" tone="critical" description="Tone changes the border only; the words still say it."><p>Never colour alone.</p></Card>
      </div>

      <div className="template-demo">
        <span className="gallery-label">States: first use, no result, error, forbidden, loading</span>
        <EmptyState kind="not-processed" label="Add evidence to begin" description="Nothing has been processed yet, so there is nothing to search."><a href="#gallery-page-title">Add evidence</a></EmptyState>
        <EmptyState kind="no-match" label="No match in this view" description="Nothing matches these filters. The scope was not widened."><button type="button">Clear filters</button></EmptyState>
        <RouteState state="error" label="This view could not load" reference="EXAMPLE-REF"><button type="button">Try again</button></RouteState>
        <RouteState state="forbidden" />
        <RouteState state="loading" label="Reading the case status" description="This usually takes a few seconds." />
      </div>

      <div className="template-demo template-demo--grid">
        <div><span className="gallery-label">Skeleton lines</span><Skeleton lines={4} label="Loading example" /></div>
        <div><span className="gallery-label">Skeleton cards</span><SkeletonCards count={2} label="Loading cards example" /></div>
      </div>
    </section>
  )
}
