# NexusAI workspace redesign research

Date: 2026-09-29  
Scope: `apps/investigation-workspace/**`  
Status: design direction for the staged visual and interaction redesign

## Purpose and constraints

This study establishes a visual and interaction system before any live-data page is restyled. The target is a distinctive, modern enterprise forensic workspace: fast to scan, calm under information load, visually rich where the data supports it, and explicit about evidence state and provenance.

The following constraints are non-negotiable:

- Never invent evidence, counts, progress, chronology, identities, classifications, roles, or attention states.
- A visualization must be driven by an existing response field or be absent.
- Colour never carries state alone; icons and labels accompany it.
- Evidence families remain filters, not navigation.
- Zero, unavailable, unprocessed, partial, forbidden, and failed remain distinct states.
- Mixed English, Urdu, Roman Urdu, identifiers, and LTR/RTL evidence retain `dir="auto"` and `<bdi>` isolation.
- Noto Sans, Noto Sans Arabic, and Noto Sans Mono remain the product fonts because Geist does not cover the evidence corpus.
- Existing analyst presentation adapters are ported, not rewritten.

## Research method

The references were assessed for specific, transferable patterns rather than copied visually. Product sources were preferred over galleries and opinion articles. The evaluation questions were:

1. How does the product keep a dense workspace understandable?
2. How does it preserve context while moving from summary to source?
3. Which interactions are keyboard- and touch-complete?
4. How are theme, elevation, status, and data colours governed?
5. Which patterns are appropriate for forensic evidence rather than generic business analytics?

## Reference findings

### Enterprise workspaces

- [Linear board layout](https://linear.app/docs/board-layout), [display options](https://linear.app/docs/display-options), [selection](https://linear.app/docs/select-issues), and [command menu](https://linear.app/now/invisible-details) demonstrate compact list density, saved personal view preferences, contextual actions, and keyboard-first selection. NexusAI should adopt their speed and progressive controls, but not reproduce Linear's issue-tracker information architecture.
- [Stripe dashboard search](https://docs.stripe.com/dashboard/search) combines immediate top results with grouped expanded results and explicit search filters. NexusAI should use the same disclosure model for case/evidence search: quick results first, structured filters when the analyst needs precision.
- [Notion layouts](https://www.notion.com/help/layouts), [sidebar](https://www.notion.com/help/navigate-with-the-sidebar), and [search](https://www.notion.com/help/search) show how a collapsible navigation layer, focused content area, and secondary detail panel can coexist. NexusAI should preserve navigation state while a source opens in context.
- [Raycast list/detail](https://developers.raycast.com/api-reference/user-interface/list) and [Grid](https://developers.raycast.com/api-reference/user-interface/grid) support a useful rule: use list/detail for text- and metadata-dominant evidence, and a visual grid only for image-dominant evidence. Actions remain reachable from a consistent action panel and shortcuts.
- [Retool apps](https://docs.retool.com/apps/quickstart) demonstrate selected-row detail binding and persistent frame/page structure. NexusAI should use this relationship for evidence tables and source details, without exposing builder-like controls.

### Operational dashboards and analytical navigation

- [Datadog dashboard template variables](https://docs.datadoghq.com/dashboards/template_variables/) make scope visible and persist reusable filter views. NexusAI should expose active case/evidence scope as labelled chips and never silently broaden an empty scope.
- [Grafana dashboard guidance](https://grafana.com/docs/grafana/latest/visualizations/dashboards/build-dashboards/best-practices/) recommends that a dashboard answer a focused question and reduce cognitive load. [Data links](https://grafana.com/docs/grafana/latest/visualizations/panels-visualizations/configure-data-links/) preserve context during drill-down, while [transform/table views](https://grafana.com/docs/grafana/latest/visualizations/panels-visualizations/query-transform-data/transform-data/) provide a table when a chart is the wrong representation. NexusAI should make every chart a route into its underlying evidence and always retain a readable tabular path.
- [Elastic dashboard drilldowns](https://www.elastic.co/docs/explore-analyze/dashboards/drilldowns) preserve filters and time context when moving from a summary to detail. NexusAI should preserve case, evidence-family, and selected-time context across every drill-down.

### Design systems

- [Vercel Geist colours](https://vercel.com/geist/colors) use stepped roles for canvas, components, borders, contrast, and text. [Material/elevation](https://vercel.com/geist/material) cautions that the lowest sufficient elevation should be used and that shadows alone should not encode hierarchy. [Badges](https://vercel.com/geist/badge) distinguish static labels from actions and combine semantic colour with text/icon meaning.
- [Microsoft Fluent tokens](https://fluent2.microsoft.design/design-tokens) separate global primitives from semantic aliases, allowing accessible theme changes without component rewrites. [Fluent elevation](https://fluent2.microsoft.design/elevation) accounts for surface luminosity rather than applying the same shadow to every background.
- [Fluent colour](https://fluent2.microsoft.design/color) separates neutral, shared, and brand palettes. Neutrals carry surfaces and hierarchy; brand colours anchor a product but should not be overused across large surfaces.
- [Atlassian design tokens](https://atlassian.design/foundations/design-tokens) treat colour, spacing, typography, and elevation as governed meaning rather than one-off styling. This supports NexusAI's existing `--analyst-*` semantic namespace.
- [Atlassian colour](https://atlassian.design/foundations/color) assigns separate neutral ramps to light and dark themes and maps saturated colours through role, emphasis, and interaction-state tokens.
- [Adobe Spectrum colour fundamentals](https://spectrum.adobe.com/page/color-fundamentals/) generate theme values against target contrast ratios rather than mirroring a hex value between themes. [Using colour](https://spectrum.adobe.com/page/using-color/) uses three background layers for application framing.
- [USWDS colour guidance](https://designsystem.digital.gov/design-tokens/color/overview/) separates project, state, and system colours, limits a project to a small coherent subset, and describes contrast by measurable luminance grades. This supports distinct brand, interaction, status, and data roles.
- [GitHub Primer layout](https://primer.style/product/getting-started/foundations/layout/) and [colour usage](https://primer.style/product/getting-started/foundations/color-usage/) provide responsive rules that change information arrangement, not merely dimensions. Below 768px, NexusAI should become one primary column with secondary content in drawers or sheets; wide screens may use list/detail or summary/detail compositions.
- [Material 3](https://m3.material.io/) provides useful adaptive colour, shape, and motion concepts. NexusAI should borrow tokenized motion and responsive behaviour, not its expressive consumer styling or oversized controls.
- [Carbon data tables](https://carbondesignsystem.com/components/data-table/usage/) support compact tables, hover/selection, sticky context, and a limited toolbar. [Content switchers](https://carbondesignsystem.com/components/content-switcher/usage/) are appropriate for alternate views of the same dataset, with arrow-key navigation.
- [WAI-ARIA grid](https://www.w3.org/WAI/ARIA/apg/patterns/grid/), [tabs](https://www.w3.org/WAI/ARIA/apg/patterns/tabs/), and [keyboard interface](https://www.w3.org/WAI/ARIA/apg/practices/keyboard-interface/) define the interaction floor: visible focus, one tab stop in composite widgets, arrow/Home/End navigation where expected, and focus return after dialogs/drawers.

### Data visualization

- [Apache ECharts dataset](https://echarts.apache.org/handbook/en/concepts/dataset/) separates source data from its visual encoding. [Events](https://echarts.apache.org/handbook/en/concepts/event/) enable click-to-filter and drill-down, and [ARIA guidance](https://echarts.apache.org/handbook/en/best-practices/aria/) requires accessibility to be enabled explicitly and supports decal patterns so series are not distinguished by colour alone. [Carbon's visualization palettes](https://carbondesignsystem.com/data-visualization/color-palettes/) deliberately sequence categorical colours for neighbouring contrast and warn that gradients must not replace a meaningful sequential palette.
- NexusAI should therefore use a thin chart wrapper with: semantic token colours; a visible title and concise interpretation; a table equivalent; keyboard-reachable drill-down controls; labelled empty/unavailable states; and no chart if the API does not provide the necessary values.
- Gauges and progress visuals are permitted only for true fixed ratios. This follows [Geist Gauge](https://vercel.com/geist/gauge): an accessible label and progress semantics are required, and no invented target or threshold may be implied.

### Forensic and investigative products

- [Cellebrite Inspector](https://cellebrite.com/en/inspector/) emphasizes unified search, filtering, OCR, and report-ready review. [Cellebrite Guardian](https://cellebrite.com/en/products/guardian/) emphasizes centralized evidence, chain of custody, case visibility, and event history. NexusAI should make source truth and activity history first-class, not hide them behind decorative summaries.
- [Microsoft Purview review sets](https://learn.microsoft.com/en-us/purview/edisc-review-set-view) use a source/native view, searchable plain text, metadata panel, and previous/next navigation. NexusAI's source viewers should preserve this predictable document-review loop.
- [Magnet AXIOM Timeline Explorer](https://www.magnetforensics.com/resources/using-the-new-timeline-explorer-in-magnet-axiom-3-0/) pairs a time overview with evidence preview/details at the selected point. NexusAI should couple timeline selections to source detail and never treat a visual cluster as proof by itself.
- [Maltego entity selection](https://support.maltego.com/en/support/solutions/articles/15000010432-entity-selection) and [graph sidebar](https://docs.maltego.com/en/support/solutions/articles/15000009615-graph-sidebar) demonstrate synchronized graph/list selection and typed paths. NexusAI connections should keep an accessible list/detail alternative and explicit relationship labels.
- [Palantir Object Views](https://www.palantir.com/docs/foundry/object-views/overview) distinguish compact panel context from a full working view. NexusAI should show only critical source context in a rail/panel and reserve full evidence inspection for a focused view.

## Adopted product principles

### 1. Progressive density

The default view answers “what is this, what changed, and what can I do?” Detail appears on selection or expansion. Dense tables remain available; card layouts are reserved for genuinely distinct summaries or actions, not used as a universal container.

### 2. Context survives every transition

Case, evidence-family scope, time scope, current selection, and the analyst's theme/density preferences persist when possible. Opening a source does not discard the finding or filter state that led to it.

### 3. Visuals are navigation, not decoration

Charts, timelines, distributions, and relationship views must map to real response fields and offer a drill-down or table path. Decorative gradients and illustrations may establish identity or improve an empty state, but never imply data.

### 4. Provenance stays beside the claim

The visual system must strengthen the product differentiator: claim-level markers, evidence-strength labels, openable locators, hover/focus previews, and a source detail path. A model-derived observation must never look identical to a source record when the API distinguishes them.

### 5. Fast by keyboard, clear by touch

Primary actions have stable placement. Composite widgets follow ARIA keyboard patterns. Every pointer target is at least 44px on touch layouts. Motion is short, purposeful, and removed when reduced motion is requested.

### 6. Richness through layers and focus

The interface should feel modern through a tinted canvas, restrained mesh/gradient identity fields, crisp elevated surfaces, strong selected states, purposeful icons, and responsive detail panels. It should not depend on glass effects, continuous animation, giant marketing copy, or a collage of unrelated cards.

## Palette proposals

Both options retain governed brand blue `#2563EB` and teal `#14B8A6`, preserve separate status roles, and use dedicated categorical data colours. Values are design candidates until the automated contrast matrix passes.

### Option A — Navy Signal

Identity: operational, trustworthy, precise, distinctive without becoming cyberpunk.

| Role | Light direction | Dark direction |
|---|---|---|
| Canvas | cool blue-grey `#EEF3F9` | midnight navy `#071321` |
| Primary surface | near-white blue `#FBFDFF` | deep slate `#0D1C2E` |
| Raised surface | white `#FFFFFF` | lifted slate `#14263C` |
| Chrome | deep navy `#0B2442` | near-black navy `#06101C` |
| Primary action | brand blue `#2563EB` | luminous blue `#74AEFF` |
| Secondary accent | brand teal `#14B8A6` | aqua teal `#5EEAD4` |
| Focus | blue ring with offset | bright blue ring with offset |
| Data | blue, teal, cyan, amber, coral, slate | lighter matched series |

The identity field may use a low-contrast navy-to-blue radial mesh in chrome/hero areas. Content surfaces remain solid for legibility. This direction is trustworthy, but it makes blue carry canvas, chrome, brand, selection, and interaction at once. That weakens hierarchy and makes the product resemble many security dashboards.

### Option B — Mineral Signal (selected)

Identity: precise technical workspace, with mineral/graphite framing, cobalt actions, and teal/cyan identity signals.

| Role | Light direction | Dark direction |
|---|---|---|
| Canvas | mineral grey `#F1F4F8` | graphite `#0C111B` |
| Primary surface | cool near-white `#FAFCFE` | carbon slate `#121A27` |
| Raised surface | white `#FFFFFF` | lifted slate `#1C2838` |
| Chrome | ink graphite `#172033` | near-black mineral `#080D15` |
| Primary action | cobalt blue `#2563EB` | clear sky blue `#82B7FF` |
| Secondary accent | teal `#0F9F93` | aqua teal `#66E3D4` |
| Signal accent | cyan `#0788A5` | ice cyan `#67E8F9` |

This option follows the research more closely: neutrals create the workspace hierarchy while a small set of saturated colours creates brand recall and interaction priority. A cobalt-to-teal signal field remains distinctive without making every analytical surface blue.

## Selected direction: Mineral Signal

Mineral Signal is selected after evaluating both options against contrast, forensic trust, data-series separation, light/dark parity, and visual distinctiveness. It provides a calm evidence workspace while reserving chroma for actions, focus, selection, identity, and real visualizations. Its implementation rules are:

- Brand blue means selection, navigation, focus, and primary action—not “good”.
- Teal supports identity and secondary interaction—not “verified”.
- Green, amber, red, and grey remain explicit ready/processing/failed/excluded semantics.
- Categorical chart colours never reuse status meaning without a written label.
- Gradients are confined to identity, hero, selected-feature, and illustrated empty-state fields; not body text backgrounds or evidence tables.
- Surfaces use three deliberate elevations: inset/context, standard work surface, and overlay. Borders and background shifts carry hierarchy before shadow.
- The light theme is mineral-grey rather than flat white. The dark theme is graphite/slate rather than flat black or navy-washed.

## Initial component contract

The Type Proof gallery must exercise the system before it reaches live pages:

- typography in English, Urdu, Roman Urdu, mixed direction, identifiers, and tabular numerals;
- brand lockup, mark, palette, surface/elevation, spacing, radius, and focus tokens;
- primary, secondary, quiet, and destructive buttons in default, hover, focus, pressed, loading, and disabled states;
- static badges/pills, interactive chips, tabs, and segmented controls with correct semantics;
- tooltip, popover, drawer, modal, toast, and skeleton primitives with keyboard behaviour;
- status and evidence-strength treatments that always contain an icon and text;
- light and dark theme parity, reduced-motion behaviour, 44px touch targets, logical properties, and no horizontal overflow at 375px.

The gallery is not a second application or an excuse to place mock forensic data on screen. Its copy is explicitly design-system demonstration content. Live-data pages will consume the approved primitives one surface at a time.

## Rejected patterns

- Card soup: repeated same-weight rectangles erase hierarchy.
- A marketing hero inside analytical routes: it consumes working space and obscures case context.
- Glassmorphism: translucent evidence surfaces reduce predictability and contrast.
- Purple “AI” gradients, neon glow, cyberpunk grids, and looping decorative motion: they weaken the forensic tone.
- Charts without a direct question, data source, drill-down, and table alternative.
- Disabled controls for unimplemented functions; absent functions remain absent.
- Native selects where the interaction requires a governed popover/listbox.
- Colour-only status, emoji status icons, and icons without accessible labels.
- Fabricated dashboard “attention” scores, percentages, trends, or thresholds.

## Delivery sequence

1. Implement Mineral Signal tokens and the Type Proof component gallery in both themes.
2. Measure contrast, keyboard paths, reduced motion, touch targets, and 375px overflow; capture the required responsive screenshots.
3. Apply the approved identity to the global/case/task shell.
4. Redesign Dashboard with only verified backend fields; record missing fields in `docs/work/BACKEND_REQUESTS.md`.
5. Continue surface by surface through Investigate, Overview, Evidence, Cases/New Case, Activity/Settings/Error, Timeline, and Connections.

No downstream surface should be visually rewritten before the identity gallery is reviewed and its tokens are proven.

## Slice 2 decision — application shell

### Primary-source patterns reviewed

- Linear's command menu groups navigation and creation behind a keyboard-first surface, while its search documentation keeps scope explicit instead of silently broadening it: https://linear.app/docs/search
- Linear treats sidebar favorites as personal shortcuts and preserves their order independently of shared workspace structure: https://linear.app/docs/favorites
- Linear's selection model keeps bulk actions adjacent to the selected objects instead of filling global chrome with dormant controls: https://linear.app/docs/select-issues
- GitHub Primer's PageLayout moves a persistent sidebar into a full-screen dialog overlay at narrow widths rather than wrapping desktop navigation into rows: https://primer.style/product/components/page-layout/
- Microsoft Fluent's navigation guidance distinguishes global navigation from local task context and keeps the current location visible: https://fluent2.microsoft.design/components/web/react/core/nav/usage

### NexusAI ruling

The live shell keeps the contract's three bands: a dark Mineral Signal global identity bar, a compact authoritative case strip on case routes, and the existing page header as task context. The global bar contains only identity, bounded workspace search, and appearance preferences. Tenant, user, role, policy, notification, and long-running-operation affordances remain absent because the current UI API exposes no authoritative source for them.

At 1024px and above the navigation rail is persistent and collapsible. Below 1024px it becomes the existing modal drawer with focus trap, Escape dismissal, and focus return. The command palette searches configured cases, active-case evidence, routes, and browser-local question history; every group names its scope and a zero match never widens it. Case switching continues to update the URL and clear incompatible case-scoped session state before navigation.

The shell uses Lucide icons for a consistent enterprise icon grammar. Colour remains secondary: active routes have text, icon, boundary, and position cues; readiness states have an icon, label, and description; every icon-only control carries an accessible name. The graphite-to-mineral gradient is confined to identity chrome, while analytical work surfaces remain solid.

## Slice 3 decision — Dashboard

### Primary-source patterns reviewed

- Grafana's dashboard guidance says a dashboard should answer a question, progress from general to specific, keep graphs focused, reduce cognitive load, and avoid refresh rates that exceed the data's useful cadence: https://grafana.com/docs/grafana/latest/visualizations/dashboards/build-dashboards/best-practices/
- Datadog defines widgets as correlated visual representations and recommends choosing the visualization, unit, grouping, and contextual link that fit the data rather than filling a dashboard with one widget type: https://docs.datadoghq.com/dashboards/widgets/
- Datadog's dashboard variables dynamically filter related widgets and persist the selection in the URL. NexusAI applies the same linked-filter principle to its case queue without claiming fields the service does not expose: https://docs.datadoghq.com/dashboards/template_variables/
- Datadog context links preserve the selected group and time context when moving from a dashboard into corrective work. NexusAI's equivalent is a direct, case-scoped evidence or Investigate route: https://docs.datadoghq.com/dashboards/guide/context-links/

### NexusAI ruling

The Dashboard tells one story: understand evidence readiness, identify the evidence composition behind it, select the cases contributing to a measure, then continue the next real action. Its aggregate band now exposes all backend-reported accounting fields—configured cases, evidence, ready, processing, failed, accepted, duplicate, and rejected—and each measure filters the same case queue. Evidence readiness uses a true multi-segment donut with an exact total and labelled count controls; row accounting stays on a linear composition because tiny duplicate and rejected values compare better on a common baseline. A separate ranked bar view compares accepted structured rows by family and filters the workbench. The earlier one-value rings were rejected because dividing a total by itself communicates no relationship; no synthetic score, trend, percentage, or progress is introduced.

The queue remains readiness-ordered rather than pretending to know investigative priority. The selected-case inspector moves from aggregate to specific: readiness composition, returned record families, ingestion accounting, recent processing, reported failures, curated question suggestions, and exact case-scoped links. Recent questions remain explicitly browser-local. Auto-refresh remains limited to a real in-flight state, every 15 seconds, and pauses while the tab is hidden.

The endpoint still does not expose assignment, owner, severity, classification, investigative priority, a complete historical job series, or an unbounded evidence-modality census. Those fields and visualizations remain absent. Recent evidence and jobs are treated as bounded status-response data, never as a full historical trend.

### Dashboard insight and visualization audit

The follow-up audit applies Grafana's primary rule that a dashboard must answer a question and reduce cognitive load, plus Microsoft Defender's queue pattern of filtering work, selecting one item, then exposing the reason and next action in a summary pane. A chart is admitted only when it has an authoritative source, a named analytical question, a meaningful visual encoding, and a drill-through or filtering action.

| Analyst question | Visual and interaction | Authoritative source | Ruling |
|---|---|---|---|
| How much configured work is in this browser workspace? | Two direct totals that filter the case queue | runtime-configured collection IDs and per-collection status | shipped; totals are not drawn as charts |
| Is evidence ready, processing, or failed? | multi-segment donut with an exact total plus three labelled count filters | `GET /collections/status` summary | shipped; the donut encodes a real part-to-whole relationship |
| Did ingestion retain, duplicate, or reject structured rows? | segmented accounting bar plus three exact count filters | `GET /collections/status` top-level summary | shipped |
| Which structured families contribute the most accepted rows across configured cases? | ranked horizontal bars with exact rows, case coverage and linked workbench filtering | `record_families[].accepted_rows` from each configured case status | shipped; this is structured-row composition, not a complete file or modality census |
| Which case needs the next evidence-readiness action? | readiness-ordered selectable queue and synchronized inspector | the same status responses | shipped; explicitly not investigative priority |
| What structured evidence does the selected case contain? | categorical composition bar, labelled values and Overview drill-through | `record_families[].accepted_rows` | shipped in the selected-case inspector |
| What changed over time and where are failures recurring? | status-over-time chart with failed bucket selection opening the affected jobs | no complete history endpoint | withheld; backend request 3 |
| What evidence media and file families dominate a complete case? | interactive ranked bars with an Evidence-route filter | no complete modality/family evidence-file census; `recent_evidence` is bounded | withheld; backend request 8 |
| Which authorized cases exist beyond served runtime configuration? | searchable server-backed directory and workspace totals | no collection-directory endpoint | withheld; backend request 9 |
| What is the team-wide activity and custody story? | event volume by type with a paginated event table and case drill-through | no authorized audit/activity endpoint | withheld; backend request 10 |

Pie and donut charts are not the default. They are permitted only when proportionality is the analyst's actual question and the segments remain distinguishable. Readiness qualifies because ready, processing and failed are exclusive parts of the reported source total; its donut therefore keeps the exact total in the centre and every segment in the accessible name, with labelled controls beside it. Row accounting remains linear because small failed/rejected values compare more accurately on a common baseline. Ranked family bars answer a magnitude question, while the queue/detail workbench answers an operational selection question. No meaning is hidden behind hover.
