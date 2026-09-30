# CODEX — Full visual and interaction redesign of the NexusAI workspace

**Date:** 2026-09-29 · **Requested by:** the product owner, on the team lead's direction ·
**Scope:** `apps/investigation-workspace/**` (yours). Backend needs go to Claude (§8).

---

## 0. What changed, and what this document overrides

The team lead reviewed the workspace and found it **too plain and ordinary**. The direction now:

- **Theme:** richer, more distinctive colours and backgrounds. Stop using flat, near-white and
  near-grey surfaces everywhere.
- **Navbar and sidebar:** redesigned to a modern, premium standard.
- **Visualisation:** more charts, figures and interactive components, especially on the dashboard and
  overview pages.
- **Coverage:** every page, every section and every element redesigned to current, world-class
  enterprise standards, while staying easy to use and understand.
- **Responsiveness:** fully responsive.
- **Missing functionality:** where a page needs data or a feature the backend lacks, the backend is
  built too (§8).

**Precedence.** Where this document conflicts with the *visual* rules in
`docs/ux/NEXUSAI_PRODUCT_UX.md` (§2.2 aesthetic rejections, §8.1–§8.4) or with
`docs/work/CODEX_UI_ELEVATION_BRIEF_20260928.md` §0 (the narrow reading of "decorative"), **this
document wins.** Tasteful gradients, a richer brand palette, visible depth, illustrated empty states,
purposeful motion and many more visualisations are now **required**, not forbidden.

**What does not change.** These rules exist because this is forensic evidence that may be read in
court. They are about truth and safety, not style:

1. **Every number, chart, count and label comes from the API.** No sample data, placeholder figures,
   "demo" series or estimates in any shipped view. An empty or partial dataset renders as an honest
   empty or partial state, beautifully designed.
2. **Privacy is enforced by the server.** Never display plate text, identity numbers or transcript
   text unless the API returned it for that request. Never try to unmask anything.
3. **Accessibility:** WCAG 2.2 AA in both themes. Richer colour must still give text at least 4.5:1 and
   UI and focus at least 3:1 against the actual background, gradients included.
4. **No engine jargon for analysts:** no model, template, SQL, operation or token names in the UI.
5. **Every visible control works.** Nothing is shown disabled "for later".
6. **Honest states.** A refusal or clarification is never styled as a finding, and a withheld answer
   never shows a number (see `CODEX_UI_ELEVATION_BRIEF_20260928.md` §4 and
   `CODEX_UI_DEFECTS_20260928.md`).

---

## 1. Research first — deliverable required before redesigning pages

Study current (2025–2026) world-class enterprise products and design systems, and write up the
findings in `docs/work/UI_REDESIGN_RESEARCH_20260929.md`. Cite each reference and say what you are
adopting from it.

- **Enterprise product UI:** Linear, Vercel, Stripe Dashboard, Datadog, Grafana, Retool, Notion, Raycast.
- **Design systems:** Microsoft Fluent 2, IBM Carbon (especially data visualisation and data tables),
  Atlassian Design System, Google Material 3, GitHub Primer, Vercel Geist.
- **Investigation and analytics domain:** Magnet AXIOM (Connections, Timeline), Cellebrite
  (Guardian, Inspector), Palantir Foundry and Gotham (object views, graph), Splunk and Elastic
  Kibana (dashboards, drill-down), Maltego (link analysis).
- **Patterns to evaluate:** collapsible icon-and-label sidebar with sections and live badges; top
  command bar with a ⌘K command palette; bento-grid dashboards; KPI cards with trend context;
  drill-down charts (click a segment to filter the page); cross-filtering between charts and tables;
  side panels and split views; saved filters; skeleton loading matched to real latency; toasts;
  contextual empty states; data-dense tables with sticky headers, column control and density.

For each page (§4), record the two or three best reference patterns and the design decision you took.

---

## 2. Visual identity — the first implementation slice

Build it once, as tokens and components, and show it on a design-system gallery page (the existing
`TypeProofPage` route) in both themes before redesigning any page. Share screenshots of the gallery
with the product owner, then continue unless they object.

- **Brand palette:** a distinctive, trustworthy identity for an investigation product. For example, a
  deep indigo or navy for the chrome, one confident accent (a teal, cyan or electric blue) for
  interaction, and a separate set of status and data colours. Propose two options in the gallery and
  pick one.
- **Backgrounds with depth.**
  - Light theme: a softly tinted canvas, not white; elevated white or near-white surfaces; subtle
    gradients or mesh on the app header, hero bands and empty states only.
  - Dark theme: a deep, blue-tinted slate with visibly distinct elevation levels.
  - Add fine texture (a subtle grid or noise) only where it reads as premium, and never behind dense
    tables.
- **Navbar:** a slim top bar with case switcher, global search that opens the command palette,
  notifications for real events only (processing finished or failed), theme and profile. It stays
  pinned, with a subtle blur only where contrast allows.
- **Sidebar:** grouped sections (Workspace, Case), icon plus label, collapsible to icons with tooltips,
  and a strong active state. Live badges show real counts (for example, processing or failed items).
  Case context sits at the top of the case section. On mobile it becomes a drawer.
- **Typography:** keep the installed Noto Sans and Noto Sans Mono variable fonts. Set a crisp type scale
  (12, 13, 14, 16, 20, 24, 32, 40) with tabular numerals for all figures.
- **Components:** cards with considered radius and shadow per elevation, buttons, chips, pills, tabs,
  segmented controls, tooltips, popovers, drawers, modals, toasts and skeletons, each with hover,
  focus-visible, active, selected, disabled and loading states.
- **Motion:** purposeful and quick (150–250 ms): page and panel transitions, chart entry, hover lift,
  number count-up on first load. Everything respects `prefers-reduced-motion`. Nothing loops forever.
- **Illustration:** simple, on-brand SVG spot illustrations for empty and first-run states. No stock
  art and no emoji as status.

**Dependencies approved by the product owner for this redesign:**

- `lucide-react` (ISC) for icons.
- **Apache ECharts** (`echarts`, Apache-2.0) with a thin React wrapper for charts. It covers bar,
  line, area, donut, treemap, heatmap, calendar, sankey, gauge and **graph (link analysis)**, with
  zoom, brush and cross-filtering.

Add nothing else without asking. Keep the bundle lean with code-splitting per route.

---

## 3. Data you can use (verified against the live API)

- **`GET /collections/status?collection_id=…`** (per case):
  - `summary`: evidence total, completed, failed and in flight; accepted, duplicate, rejected and
    total rows; job counts.
  - `record_families[]`: accepted, duplicate and rejected rows per record type.
  - `recent_jobs[]`: status, record type, rows, `queued_at`, `started_at`, `completed_at`, attempts,
    error.
  - `recent_evidence[]`: file, modality, detected type, status, size, `created_at`.
- **`GET /query/capabilities?collection_id=…`**: evidence families with support levels, `summary`
  (queryable, manual review, no data, unavailable) and `query_corpus` entries. **Suggested questions
  must come only from `query_corpus`.**
- **`GET /evidence`, `GET /evidence/{id}`, `/content`:** the evidence inventory and detail.
- **`POST /query/hybrid`:** answers. The response includes `enterprise.data_grid` (columns and rows),
  `fact_packet.citations[]` with locators (page, passage, time, bounding box) and `provenance[]` with
  `source_truth_state` and `artifact_type`.
- **`GET /images/similar`, `GET /faces/similar`.**
- **Question history:** browser-local, labelled *this browser only*, until a server endpoint exists
  (§8).

Verified numbers to test against:
- `nexusai-forensic-demo`: 12 evidence items, 12,912 accepted rows, CDR 8,642.
- `nexusai-multimodal-product-acceptance`: 43 evidence items; 1,057 camera ANPR sightings; 307 plate
  reads; 20 detected faces.

---

## 4. Page-by-page redesign — one page at a time, in this order

For each page: research notes (§1), design, build, verify live (§6), screenshots, tests, a
`CODEX_UI_TRACK` entry, and then the next page.

### 4.1 Shell: navbar, sidebar, command palette, breadcrumbs, theme

Per §2. The command palette jumps to pages, configured cases, recent questions (this browser) and
"Ask in <case>…".

### 4.2 Dashboard — the flagship page

A bento grid. Every element is interactive and clicking it filters or navigates:
- **KPI cards** (configured cases): evidence total, ready, processing, failed, accepted rows,
  duplicate and rejected rows. Each has a mini-visual (for example, a ready/processing/failed
  proportion ring) and filters the case table.
- **Evidence by modality** (donut or treemap from `recent_evidence` modality) and **rows by record
  type** (stacked bar from `record_families`). Clicking a segment opens that case's evidence,
  filtered.
- **Processing activity over time:** jobs completed and failed per day from the
  `recent_jobs` timestamps, as area or bar with a brush to zoom. Plus a **processing duration**
  distribution (`completed_at − started_at`).
- **Ingestion quality:** accepted vs duplicate vs rejected per record type, as a gauge or stacked
  meter with a one-line explanation of each.
- **Needs attention:** failed evidence and jobs with the error in plain words and a direct action.
- **Case readiness table** with inline proportion bars and last activity.
- **Suggested questions** (`query_corpus`) as clickable cards that open Investigate pre-filled, and
  **continue recent questions** (this browser).
- Auto-refresh every 15 s while anything is in flight (paused when the tab is hidden), with a "last
  updated" time.

### 4.3 Cases list and New case

- **Cases list:** a rich table or card toggle, search, sort, status chips, a composition bar per case,
  and last activity.
- **New case:** a guided, attractive intake flow with a drag-and-drop zone, per-file progress and the
  detected family shown with one-click correction.

### 4.4 Case overview

- Composition by record type and modality: treemap or sunburst plus exact counts.
- Accepted, duplicate and rejected rows per family.
- An evidence-added timeline (from `created_at`).
- A capability coverage panel: which evidence families can be queried and which need review, from
  `/query/capabilities`.
- A data-quality panel with drill-down, suggested questions for this case, and recent evidence
  thumbnails.

### 4.5 Evidence catalogue and evidence detail

- **Catalogue:** a modern data table (sticky header, column chooser, density, saved filters, filter
  chips with explained counts) and a preview side panel. Grid view for images.
- **Detail:**
  - Images: the image with **toggleable overlays** of OCR regions and plate boxes, drawn from citation
    `bbox` with confidence. No plate text unless the API supplies it.
  - Audio: a waveform-style timeline with transcript segments (click to seek).
  - Video: a scrubber with markers for derived events.
  - Documents: a paginated reader with passage highlight.
  - An always-available **lineage** panel, drawn as a small flow graph from source file to version to
    derived artifacts.

### 4.6 Investigate — the core screen, with smart charts

- A conversational thread with a history sidebar (this browser), scope chips, an ask box with
  shortcuts, and suggested follow-ups.
- **Answer-first layout:** answer, result, citations, derivation, limitations, follow-ups.
- **Automatic visualisation of results**, chosen from the shape of `data_grid` and the plan:
  - breakdown or group → bar or donut;
  - ranking → horizontal bar with the top item highlighted;
  - time bucket → line or area;
  - two dimensions → heatmap;
  - relationship or network results → ECharts graph.
- Every chart has a table toggle, the exact values, and **click-to-drill**: a bar opens a follow-up
  question that the API actually answers.
- Every withheld or clarify state is a distinct, calm card (`CODEX_UI_ELEVATION_BRIEF` §4.1).
  *Model observation* and *Source record* badges appear on citations.
- An honest progress state for new questions, which can take 2–2.5 minutes on this server:
  cancellable, with elapsed time, and never a fake percentage.

### 4.7 Timeline

An interactive, zoomable multi-lane timeline (one lane per evidence family, events openable at their
source). **This needs a backend endpoint (§8).** Until it exists, show a designed "coming soon"
state that explains why and offers real suggested questions. Never draw a fake axis.

### 4.8 Connections (new page, if the backend supports it)

Link analysis for one identifier the analyst enters: a graph of related numbers, devices,
subscribers, plates and towers, built only from relationships the API returns. Needs a backend
endpoint (§8).

### 4.9 Activity, Settings and error pages

- **Activity:** grouped by day, filters, search, and a small activity chart (this browser).
- **Settings:** polish.
- **404, error and offline:** designed pages with a way back.

---

## 5. Responsive and accessible — every page

- Breakpoints 1440, 1280, 1024, 768 and 375. No horizontal page scroll at 375. Touch targets at
  least 44 px. The sidebar becomes a drawer, and bento grids reflow to one column.
- Charts resize, keep readable labels, have text equivalents (the values table) and do not rely on
  colour alone (patterns or labels).
- Full keyboard path, visible focus, screen-reader labels on every chart and control, and
  `prefers-reduced-motion` respected.

---

## 6. Verification for every page (definition of done per slice)

- `npm --prefix apps/investigation-workspace run test` is green, with new tests for every rendered
  count, chart data mapping and state.
- Verified in a browser against the **live API** at `http://127.0.0.1:4181`. The §3 numbers must
  appear exactly.
- Screenshots in both themes at 1440 and 375, attached to the `CODEX_UI_TRACK` entry.
- Zero console errors, zero horizontal overflow and zero duplicate IDs. Contrast is checked on the
  actual (gradient) backgrounds.
- Performance: route chunks code-split; the dashboard is interactive within about 2 s on this laptop
  once the API has responded.

---

## 7. Order and cadence

Research write-up → identity and gallery (§2) → shell (4.1) → **dashboard (4.2)** →
**investigate (4.6)** → case overview (4.4) → evidence (4.5) → cases and new case (4.3) → activity,
settings and error pages (4.9) → timeline and connections (4.7, 4.8) when the backend endpoints land.
**One page per change**, each reviewable and revertible.

---

## 8. Missing backend — request it, don't build it in `apps/`

Claude owns `api/**` and `semantic_layer/**`. When a page needs data or a feature the API doesn't
provide, add an entry to **`docs/work/BACKEND_REQUESTS.md`**:

- the endpoint and method you need;
- the exact response fields and types;
- the page it's for;
- empty and error behaviour;
- privacy notes.

Then move on to the next page. Claude will build, measure and document each one, and record it in
that file as delivered. Expected first requests:

1. A **timeline** endpoint: events across families for a case with time, family, summary and an
   openable source locator; paginated, filterable by family, identifier and date range.
2. A **connections** endpoint for one identifier: typed relationships across families, only from
   links verified in the data.
3. A **processing history** time series per case (jobs per day by status), if `recent_jobs` is too
   short for a real trend.
4. **Image overlay** data for an evidence item: OCR regions and plate-read boxes with confidence,
   privacy-aware.
5. **Server-side question history** (sessions) to replace browser-local history.
6. **Result presentation hints:** display labels for result columns (the API currently sends
   "M1"), plus a suggested chart type.

Never mock these in shipped UI. Until they land, the page shows its honest designed state.

---

## 9. Never

- Edit `api/**` or `semantic_layer/**`.
- Invent, sample or estimate data in shipped views.
- Display unmasked PII the API didn't return.
- Add dependencies beyond §2's approved two without asking.
- Run git writes (`reset --hard`, `clean`, `checkout --`, `restore`, `stash`, `merge`, `rebase`,
  `cherry-pick`, `pull`, `push`, force) or bypass hooks.
- Delete untracked files you didn't create.
- Write files with Bash heredocs (they have corrupted files here).

**Stop and report** if a design would need data the API doesn't have (write a §8 request instead), if
tests fail for a reason you don't understand, or if a choice would break contrast or accessibility.
