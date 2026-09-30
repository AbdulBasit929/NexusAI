# Spec: Dashboard (Stage 6, rebuilt section by section from 2026-09-30)

Route `/`. The investigator's job on this page: **"What needs me now, can I trust the evidence, and where do I continue?"** Every section answers one of those three, in that order.

Data today: one `GET /collections/status` per configured case (`useConfiguredCaseOverviews`) plus `GET /query/capabilities` for the selected case. Nothing is estimated. A case that did not report is left out and named, never zero-shaped. Requests 12 (workspace summary) and 13 (needs-attention feed) are filed in `BACKEND_REQUESTS.md`; this page will move to them when they land.

## Build status
| Section | Status |
|---|---|
| D1 briefing header, D2 summary strip | redesigned 2026-09-30 after owner review (first version rejected); earlier research pass also applied (freshness age and stale state, oldest-read freshness, partial coverage named, no fabricated zeros) with unit tests and a live check of the all-down, one-down and empty cases |
| D3 Needs review, D4 Readiness by case | built and verified 2026-09-30: unit tests (`dashboard/AttentionAndReadiness.vitest.jsx`), e2e `dashboard-slice.spec.js`, screenshots at 1440 and 375 in both themes. The workbench's duplicate Refresh was removed (Refresh lives in D1 only). |
| D5 to D8 | still the first-pass versions; rebuilt next, one at a time |

## D1 and D2 research pass (2026-09-30)
Sources read as search summaries only (the primary pages of Primer, Grafana and W3C were not reachable from the build environment, so treat the wording as secondary): Primer and Carbon PageHeader, Grafana stat panel, Nielsen Norman Group dashboard guidance, WCAG 2.2 (2.5.8 target size, 4.1.3 status messages), and dashboard KPI write-ups on zero versus missing data.

| Finding | Applied |
|---|---|
| A freshness signal is owed: last-updated marker, manual refresh, and stale as its own state. | D1 shows time and age ("05:15 · 3 min ago"). Older than 5 minutes adds ", may be out of date" in words and an amber value. The age stays out of the live region so it is not announced every minute. |
| Freshness must be honest. | "Updated" is the age of the **oldest** successful read behind the figures shown. The newest would hide a case whose refresh failed. |
| Missing is not zero; loading, unavailable, no data and zero must look and read differently. | Loading is an ellipsis. Nothing reported is an em dash with "Status unavailable". A case that could not report is named in Scope ("3 of 4 cases reporting") and on every tile ("Across 3 of 4 cases"). Zero sources says "No sources added yet" and draws no empty bar. "No successful read yet" is distinct from "Reading…". |
| A sparkline or delta needs a real time series; a fabricated trend misleads. | None drawn. The API has no history, so trends stay out and are filed as backend request 3. |
| Drill through instead of packing detail into a card. | Every tile links to the section behind it. |
| One h1, h2 card titles, status announced without taking focus, targets at least 24 px. | Unchanged and verified: one h1, `role="status"`, 44 px buttons, reduced motion respected. |

## Research applied to the page
- **Nielsen Norman Group** on operational dashboards: they exist for fast decisions, so the most critical three to five figures go top-left, colour coding stays consistent, and detail is disclosed progressively.
- **Grafana stat panel**: the value is prominent, the label says what it counts, colour appears only when a threshold matters, and the tile is a link into the data behind it.
- **W3C complex images**: a chart needs a short description and a long description available to everyone. Each chart card has a labelled figure and a Table view holding every value and real links.
- **Grafana, Datadog**: one question per panel; links preserve context. **Microsoft Defender queues**: filter, pick one item, see the reason and next action beside it.
- Codex's earlier research (`UI_REDESIGN_RESEARCH_20260929.md`) still applies for the drill-through and filter rulings.

## Layout (1440, 1024, 768, 375)
1. D1 page header, with scope and freshness.
2. D2 four KPI tiles (one row at 1440, two by two at 768 and 375).
3. D3 Needs review (7 columns) beside D4 Readiness by case (5 columns), stacked below 1024.
4. D5 Record families (7) beside D6 Ingestion (5).
5. D7 Case workbench (queue and inspector), full width; the inspector stacks under the queue below 1024.
6. D8 Continue (resume questions).
7. Scope line.

## D1 Briefing header (third version, 2026-09-30; rules: UI_REDESIGN_BRIEF §11)
Question: what needs me now, and how fresh is this? History: the first version (generic description, Scope/Updated facts, two big buttons, about 180 px) said nothing about the case; the second put a full sentence with a wrapping link under the title. This one obeys the copy budget: the title (1.5 rem) and **one short line**, about 110 px in all.
- The line, computed only from figures already on the page: a tone dot, "3 sources need review" (red), a **Review** pill that jumps to D3, then "· 53 of 55 ready". Variants: "5 sources processing" (amber, pill "Progress"), "All 55 sources ready" (green), "No evidence yet", "Reading status…", "Status unavailable" (amber), and an appended "3 of 4 cases reporting" when coverage is partial. It never asserts a state the data does not show, and words carry the meaning as well as colour.
- Right side: "Updated just now" (tooltip has the clock time; age is floored; older than 5 minutes adds "· out of date" in amber; "No successful read yet" is distinct from "Reading…"), a 44 px icon Refresh with an accessible name, then the one primary action, Add evidence. "Refreshing every 15 s" appears only while polling is real. The live region announces the clock time, never the ticking age.
- Phone: the line wraps without a dangling separator; freshness and Refresh share a row; Add evidence is full width.

## D2 Metric cards (third version, 2026-09-30; rules: UI_REDESIGN_BRIEF §11)
Question: how big is the problem, and how much can I use? The owner liked the original individual cards with a red accent on review, and rejected the flat strip that replaced them. This version restores one card per metric and makes it modern and interactive, with very little text.
| Card | Tone | Figure | Line (six words or fewer) |
|---|---|---|---|
| Needs review (hero, widest, largest figure) | failed red; green "All clear" when zero | sources to review | "2 failed · 1 missing copy" (tooltip: the full definition) |
| Ready | ready green | "53 of 55" with a floored percentage chip and a slim composition bar | none; "No sources yet" when empty |
| Processing | amber with a live pulse while work is in flight; quiet neutral at zero | sources in flight | "In progress" or "Idle" |
| Structured rows | data teal | accepted rows | "Accepted" |
Each card has a 4 px accent bar, a tinted icon chip and a flat 6% tint (no gradient). Hover lifts it 2 px with a deeper shadow and a tone-coloured border and slides an arrow in; press settles it; keyboard focus shows a 3 px ring. Figures count up over about 700 ms with ease-out (the animated digits are `aria-hidden`; the final value is exposed separately; reduced motion writes the final value at once). A click smooth-scrolls to its section, moves focus there and flashes it once (instant under reduced motion). Unknown is an ellipsis, nothing reported is a dash with "Unavailable", partial coverage is named on each card ("3 of 4 cases"), and zero sources draws no empty bar. Layout: four columns with a wider hero at 1280 px and up; at tablet the hero takes a full row above three cards; on phones the hero and Ready take full rows and the last two share one. No sparklines or deltas: the API has no history (backend request 3).
Known cross-page inconsistency: the sidebar Dashboard badge counts cases needing review (1) while the card counts sources (3). The badge belongs to the shell and is noted for the shell review.

## D3 Needs review
Question: which sources need review before I rely on results? Encoding: a queue (list of named things, not a chart). Row: severity icon plus problem word, source name (link to Evidence detail), reason sentence from the service if it gave one, case (link), and a "Review" link. Worst first (failed, then missing copy). Header states the count. Footer discloses completeness: while the status only lists each case's most recent items it says so and links to each case's Evidence list filtered to failed; when request 13 lands it becomes complete and paginated. Empty: a positive, plain sentence. Loading: skeleton rows.

## D4 Readiness by case
Question: is each case's evidence ready to search? Encoding: 100% stacked horizontal bar per case (composition across groups), worst case first; colour plus stripe (processing) and dot (failed) patterns; exact counts inside wide segments; a right-hand "42 of 43 ready" label per bar; legend. Click a segment: that case's Evidence list filtered to that state. Table view: real links per count. Text equivalent names every count.

## D5 Record families
Question: which kinds of records make up the evidence? Encoding: sorted horizontal bars (magnitude of one measure), single colour, value labels, minimum visible mark. Click a bar or "Show cases": a strip under the chart lists the cases that hold that family with rows per case and a link to that case's Evidence list filtered to the family (where the Evidence list has that filter; "generic" has none and says so). Table view adds each family's share. Footer states these are structured rows, not every file.

## D6 Ingestion
Question: did ingestion keep every row? Encoding: per-case figures with rates (bars were rejected: a 0.03% rejection is invisible on a shared baseline). Accepted, duplicate and rejected with their rates ("<0.1%" rather than rounding a real loss to zero) and a one-line definition for each. A totals row when more than one case. Empty: "No structured rows have been ingested yet."

## D7 Case workbench
Question: which case is next and what do I do? Queue on the left (search, state chips, readiness order, each row status icon plus text, bar, updated time), inspector on the right: the next action as a heading, the reason sentence, readiness metrics, then **Overview** (families and ingestion, plus "Ask next" suggested questions as chips), **Attention** and **Processing** tabs. Suggested questions move out of a tab because they are the natural next step. Family filter chip from D5 appears here. Tabs follow the APG tabs pattern. Below 1024 the inspector stacks under the queue and the selected row scrolls into view.

## D8 Continue
Question: where was I? The three most recent questions (pinned first) across cases, each with case and time, and a link to all questions on Investigate. Browser-local and labelled so. Nothing when there is no history.

## D9 Cross-case insights (not built, filed)
Processing over time, evidence by modality and file type, workspace activity: requests 3, 8, 10. They stay absent until the API can support them; no placeholder is shown.

## Chart rules
Every chart: a title that is the question, exact values (tooltip plus Table view), a text equivalent, meaning not carried by colour alone (patterns, labels), consistent colours for the same category on every page, and click-through to real rows.

## Verification
Unit tests for every count, mapping and state; e2e with four mocked cases; live check against the real API (2 cases, 55 sources, 53 ready, 2 failed); screenshots at 1440 and 375, light and dark, per section.
