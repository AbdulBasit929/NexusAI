# Investigation Workspace design system v2

The system targets operational analysts with basic computer skills and little
or no AI/tool architecture knowledge. Backend complexity is hidden by default.

## Semantic foundations

Use design tokens, not literal colors, for background, surface, text, border,
focus, positive, caution, critical, information, candidate, and muted states.
Every state uses icon + label + description; color never carries meaning alone.
Typography prioritizes dense investigative reading and includes Urdu-capable
fonts with stable Latin/Urdu metrics.

The implementation boundary is `.analyst-portal`. Analyst tokens must be
defined there and must not override the advanced `/app` surface. The current
core tokens are:

| Token | Contract |
|---|---|
| `--analyst-content-wide` | 1280 px maximum for evidence and journal work |
| `--analyst-content-reading` | 1060 px maximum for Ask and answer reading |
| `--analyst-line` | one quiet theme-derived separator across all four routes |
| `--analyst-surface` | primary reading surface; no decorative glass treatment |
| `--analyst-surface-subtle` | application background and secondary controls |
| `--analyst-radius-sm/md/lg` | 8/12/16 px control, panel and major-region rhythm |
| `--analyst-control-height` | 42 px search/filter control baseline |

Literal evidence values never become design tokens. Theme colors remain the
upstream LocalAI theme authority; analyst CSS composes them without changing
the global theme.

## Typography and spacing

Page titles use one restrained responsive scale with a 700 weight and compact
negative tracking. Section titles use a single 1 rem level. Eyebrows are short
orientation labels, not a substitute for headings. Body and metadata text must
remain readable at 200% zoom; microcopy is reserved for timestamps, source type
and disclosure descriptions.

The spacing rhythm is 8, 12, 16, 20, 24, 32 and 48 px. Major sections use 20–24
px separation; row internals use 8–14 px. A new bordered container must replace
or consolidate an existing boundary rather than create another nested card.

## Surface and control hierarchy

- Each page has one orientation header and one primary decision point.
- Home has one Ask entry when evidence is ready plus Data and Activity
  summaries; no-ready processing/attention states lead to Data review. Home is
  not a card catalogue or a second copy of Data/Ask.
- Search, select and refresh controls share the 42 px baseline and visible
  focus ring. Primary actions use a filled theme-primary surface; secondary
  actions use the quiet surface and line token.
- Status always combines exact text with an icon or explicit count. Ready,
  processing, attention, failed, complete-zero and filter-no-match never rely
  on hue alone.
- Drawers, dialogs, tables, findings, citations and timelines use the same line
  token and radius family. Elevation is limited to overlays or sticky review
  surfaces where spatial separation is necessary.
- Loading, empty and error states state what happened, whether retained state
  changed, and the next meaningful action. Empty sections disappear when they
  add no information.

## Cross-page component contracts

Evidence identity is always modality icon → human type → human filename →
finding preview → exact readiness. Home uses a compact version of the Data row;
Data owns filtering and full inspection. Activity uses the same human source
labels and exact Ask result states. Ask and Activity share answer, finding,
table, timeline, citation and technical-disclosure semantics.

Stored execution metadata can be retained but must not appear as a primary
finding. Template, route, backend, planner, specialist and execution notes live
under Technical details. Existing stored questions remain authoritative in
Activity; Home previews may replace UUID-shaped identifiers with “this evidence
source” to avoid making technical identifiers the first-glance hierarchy.

Tables remain horizontally scrollable, with explicit accessible names and
mobile detail expansion. Timelines preserve source time. Citations lead with a
human source name and supported page/row/source-time/finding locator. Drawers
restore a single-column flow on small screens and keep technical identifiers
collapsed.

## Core components

| Component | Required semantics |
|---|---|
| ResultStateBanner | exact typed state; one source of truth |
| FindingCard | fact/candidate/contradiction class and citation count |
| CitationLink | source family, record/artifact locator, open action |
| ScopeChip | case/evidence/time/entity constraint; never silently truncated |
| QualityPanel | accepted/rejected/duplicate/missing counts with definitions |
| ProcessingBadge | queued/running/complete/failed/not-processed, not vague availability |
| SourceMetric | family-appropriate units, not generic rows for every medium |
| TechnicalDisclosure | operation/plan/model/packet/timing/audit, collapsed by default |
| EmptyState | distinguishes complete-zero, no-match, unavailable, and not-processed |
| LanguageText | explicit direction, script, identifier isolation, copy fidelity |

## Content language

Prefer “analysis”, “evidence”, “finding”, “source”, “time range”, and “could not
be processed”. Hide “template”, `records_sql`, raw slugs, and model/backend names
from default copy. Never say “no results” for unprocessed or unavailable data.
Never turn candidate similarity into “same person/object”.

## Density and disclosure

Default density is calm and task-oriented. Advanced filters, quality accounting,
provenance, and technical telemetry are progressively disclosed. Desktop may
use split panes; mobile uses a single reading column and sheets/drawers with
restored focus. Long IDs are copyable and middle-truncated visually only.

## Accessibility and validation

Target WCAG 2.2 AA. Validate keyboard-only flow, visible focus, landmarks,
headings, error association, 200% zoom, high contrast, reduced motion, screen
reader announcements for async states, Urdu RTL, mixed direction, and every
typed result state at the four reference widths.

All user or evidence text uses `dir="auto"`; UUIDs, hashes, plates, phones and
source-time values use an isolated `bdi`, with `dir="ltr"` when their contract
is inherently LTR. The whole shell must not flip because one Urdu result is
open. Canonical validation widths are 390, 820, 1024 and 1440 px for Home, Data,
Ask and Activity (16 views), with zero horizontal page overflow and zero
console/page errors. Keyboard checks include skip link, top and bottom
navigation, account/theme control, composer, filters, evidence rows, citations,
drawers and disclosure summaries.

## NX-UX1G closure rules

Every analyst route has exactly one stable semantic H1, including Ask when its
visible answer content changes. Execution telemetry such as route, template,
planner confidence and display-row accounting is not a primary finding; it is
available only through Technical details. Advanced `/app` may continue to show
its established observability.

Home Activity previews may replace a complete UUID and segments derived from
that same UUID with a human evidence reference. The masking source is the
authoritative stored question so an independent plate, phone or other typed
identifier is not removed merely because it is alphanumeric. The underlying
stored question and Activity review remain exact.

Final acceptance requires all four analyst routes at 390, 820, 1024 and 1440
px to have zero page overflow, one H1, named controls and no console errors or
warnings. NX-UX1 satisfies this rule and is closed.
