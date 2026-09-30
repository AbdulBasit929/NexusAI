# Dashboard v2 — research and design ruling

Date: 2026-09-28

## Measured problem

The prior Dashboard was accurate but table-led. At 1440px, the analyst had to scan four full rows to discover the first valid action, while the service-boundary explanation occupied a visually equal side panel. The row action and its reason were separated, and selecting a case could only navigate away; the Dashboard could not help the analyst compare readiness and inspect one case in place.

The backend audit confirmed that `GET /collections/status` supplies per-configured-collection evidence totals, ready/in-flight/failed counts, retained-asset gaps, accepted/rejected/duplicate row accounting, evidence families, and recent evidence/job timestamps. It does **not** supply assignment, investigative priority, severity, deadlines, global activity, or trend series. Browser-local question history is the only resumable question source.

## Primary-source research

- [Carbon: Dashboards](https://carbondesignsystem.com/data-visualization/dashboards/) says to prioritize information by importance, build a clear hierarchy, remove metrics that distract from interpretation, and make exploration dashboards searchable, filterable, and drillable.
- [Carbon: Data table usage](https://carbondesignsystem.com/components/data-table/usage/) positions tables for organizing resources, but recommends another component when the display or interaction is more complex. This supported replacing the wide passive table with a selectable queue and detail workspace.
- [Microsoft Defender XDR: Incident queue](https://learn.microsoft.com/defender-xdr/incident-queue) links search and filters to an operational queue and opens selected-item properties as the starting point for investigation. NexusAI adopts that queue-to-detail rhythm without copying unsupported severity or assignment fields.
- [Elastic Security: Detection & Response dashboard](https://www.elastic.co/guide/en/security/current/detection-response-dashboard.html) treats dashboard elements as direct routes into filtered operational work, rather than decorative totals.
- [GOV.UK: Complete multiple tasks](https://design-system.service.gov.uk/patterns/complete-multiple-tasks/) recommends verb-led task names, a small status vocabulary, related-action grouping, and emphasis on incomplete work for returning users.
- [Carbon: 2x Grid](https://carbondesignsystem.com/elements/2x-grid/usage/) emphasizes that complex product layouts must help users form a mental model, identify relevant content, and pursue the intended action. It also warns that equal visual scale destroys hierarchy.

## Design ruling

The Dashboard is an operational readiness workbench, not a KPI or executive reporting page.

1. A compact readiness band provides linked, count-bearing filters over configured cases. Counts are browser aggregates of the returned per-case status responses and are labelled accordingly.
2. A selectable queue is ordered by evidence readiness: review, processing, no evidence, ready, unavailable. The copy explicitly states that this is not investigative priority.
3. Selection updates a detail inspector in place. The inspector explains the suggested action, presents source/ready/processing/failed accounting, curated evidence-family labels, row-quality accounting, latest evidence activity, and only real navigation actions.
4. Recent questions are a secondary resume strip, explicitly labelled browser-local.
5. Unsupported assignment, severity, and investigative priority are disclosed under a collapsed “About this dashboard” control instead of occupying primary work space.
6. Visual craft remains semantic: state icons, solid state bands, clear selection, real proportion/composition/quality bars, deliberate surface elevation, and restrained blue/teal accents. There are no gradients, invented charts, percentages, priorities, decorative animation, or marketing hero treatments.
7. Selected-case detail is progressively disclosed through keyboard-operable Overview, Attention, Processing, and Questions tabs. This keeps the first view concise while preserving real failure reasons, job duration, and catalogue-authored questions one action away.

## Interaction and accessibility contract

- Every status has icon + text; colour is redundant.
- Queue items are real 44px-or-larger buttons with `aria-pressed` and a linked detail region.
- Search, readiness filters, clear, refresh, queue selection, exact recent-question links, and all case actions are functional.
- The mobile layout preserves the sequence readiness → queue → selected action and has no horizontal page scroll at 375px. The readiness filter itself is an intentional contained horizontal scroller.
- Motion is limited to 160ms hover feedback and only runs when reduced motion is not requested.
- Raw case identifiers remain raw and isolated; evidence families use curated catalogue labels.

## Measured result

- Full verification: 101 unit tests and 46 ported tests passed; the production build completed with 170 modules.
- Focused browser verification: two Dashboard contracts passed, including metric filtering, queue/detail selection, arrow-key tab navigation, refresh, exact recent-question resume, duplicate-ID and console-error checks, and zero page overflow at 375px.
- Review evidence: ten screenshots cover 1440, 1280, 1024, 768 and 375px in both designed themes.
- Live API on 2026-09-28: `nexusai-forensic-demo` reported 12 evidence items, 11 completed, 1 failed, 12,912 accepted rows, 301 duplicates, 7 rejected rows, seven record families and 12 recent jobs. The capability response reported 20 families and 80 corpus entries.
- Live browser: the Dashboard loaded both configured collections, selected the demo case without navigation, rendered its exact 12/11/0/1 readiness, and showed four catalogue-authored analyst-safe questions. One internal template-like corpus entry was caught during the live pass, filtered at presentation, and locked down by a unit test. No browser console errors were observed.
- No runtime service, container, database, API, semantic layer or deployment state was changed.
