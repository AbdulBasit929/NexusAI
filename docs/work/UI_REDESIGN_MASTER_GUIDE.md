# NexusAI workspace: UI/UX redesign master guide

Owner directive, 2026-09-29. This file is the working instruction for the redesign. It sits under `UI_REDESIGN_BRIEF_20260929.md` (binding rules, chart table, definition of done) and above the per-stage specs in `docs/work/UI_PAGE_SPECS/`. Where they conflict, the brief's truth, privacy and accessibility rules win.

## 1. What the owner asked for

1. Redesign and recreate every section of every page to a world-class, enterprise, modern, elegant and interactive standard. Carry an existing section over only if it is genuinely the best solution; otherwise improve it or replace it.
2. Add new sections wherever they make the product more useful, easier and more productive, including sections that need new backend capability.
3. Work in stages, one thing at a time, in this order: **shell, header, top navigation, sidebar, main canvas, then the Dashboard, section by section**, then every other page the same way.
4. Research before every stage: the whole thing first, then each component in depth. Design from the research, not from habit.
5. Charts, figures and graphs must be modern and each must answer a named investigator question (brief section 6).
6. The header no longer matches the palette. Fix it in the shell stages (section 5).
7. Improve the backend where it blocks a better UI. Route: section 8.

## 2. Rules that never change

Real API data only. Privacy is enforced by the server. WCAG 2.2 AA in both themes, measured on real backgrounds. No engine jargon. Every control works. Refusal, clarification and model-observation states are honest and never styled as findings. Approved dependencies: `lucide-react`, Apache ECharts with the thin wrapper, code-split per route. Anything else needs the owner's yes. No edits to `semantic_layer/**`, Docker or the database. No git writes.

## 3. Design principles (apply to every stage)

> Section 11 (modern visual and interaction language, owner direction of 2026-09-30) refines these and wins where they differ, notably principle 3 (a flat status tint is now allowed) and principle 9 (motion is richer but still functional).

1. **Answer first.** Each page opens with what the investigator most needs, then supporting detail, then raw records.
2. **One accent, one meaning.** Teal is interaction. Status (ready, processing, failed, withheld) and categorical data colours are separate sets. Never reuse a status colour as a category.
3. **Elevation over decoration.** Four surfaces: canvas, card, raised, overlay. Gradients live on chrome, hero bands and empty states only, never behind tables or charts.
4. **Progressive density.** Summary by default, detail on selection, raw data one click away. Compact and comfortable density options where tables dominate.
5. **Context survives.** Case, filters, selection and scroll persist across drill-downs and browser back.
6. **Every visual is a route.** Click filters, drills down or opens the real records. Every chart has a table path and a text equivalent.
7. **Honest waiting.** Skeletons hold layout. Long work shows real state and is cancellable. No fake percentage.
8. **Keyboard first, touch clear.** One tab stop per composite widget, arrow keys inside, 44 px touch targets, focus visible at 3:1, focus returns after overlays.
9. **Motion is functional.** 120 to 200 ms, ease-out, off under reduced motion.
10. **One component, one place.** Build shared primitives once (button, chip, badge, tabs, table, card, chart frame, empty state, toast, drawer) and use them everywhere.

## 4. Identity and tokens (the palette that governs everything)

Identity A, continuous dark chrome, is applied: header gradient navy `#111827` to deep teal `#123b43` (dark: `#070b12` to `#10343a`), teal-tinted canvas, near-white cards, teal accent `#0e7490` (dark `#4fd1e5`). Tokens live in `src/styles/tokens.css` and `src/styles/identity-a.css`. The gallery at `/design/type-proof` keeps options A, B and C for reference. All contrast is asserted in `tokens.vitest.js` and `paletteOptions.vitest.js`.

Stage 0 finishes the system: named role tokens for surface, border, text, accent, status, data and chrome; a spacing, radius, elevation, motion and z-index scale; one icon grammar (Lucide, 16/18/20 px, 1.75 stroke); a colour-blind-safe categorical set with pattern fallbacks for charts.

## 5. Stage plan

Each stage ends with: spec in `docs/work/UI_PAGE_SPECS/`, live verification, before and after screenshots at 1440 and 375 in light and dark, a log line in `CODEX_UI_TRACK`, and then the next stage starts unless the owner objects.

| Stage | Scope | Key questions to research | Deliverable |
|---|---|---|---|
| S0 | Foundations | Token architecture (Fluent, Atlassian, Primer, Geist); elevation in dark UIs; type scale for mixed English/Urdu | Token set, primitives inventory, gallery update |
| S1 | Shell frame | App-frame layouts (Linear, Vercel, Grafana, Datadog); grid, regions, scroll ownership, sticky rules | `shell.md`, layout grid, responsive rules |
| S2 | Header (top bar) | Global bar contents; palette match; search/command pattern (Linear, Stripe, Raycast); status and notification affordances | Header rebuilt on the teal gradient with search, Investigate shortcut, live activity, appearance and help. **Fix the mismatch: the header still mixes a cobalt glow and mark; retune to navy-teal only** |
| S3 | Top navigation and context bar | Breadcrumb patterns; case switcher; tab bars vs sidebar for case sections (Atlassian, Primer) | Case context bar with breadcrumb, readiness, switcher, section tabs |
| S4 | Sidebar | Collapsible rail patterns (Linear, Notion, Azure); grouping; badges; pinned items; mobile drawer (Primer) | Sidebar with groups, live badges, pins, recent items, collapsed mode |
| S5 | Main canvas and page templates | Page anatomy; content widths; card system; empty, loading, error, offline patterns | Page templates: overview, list, detail, workspace, status pages |
| S6 | Dashboard | Ops dashboards (Grafana, Datadog), queue-plus-detail (Defender, Linear), forensic case summaries (AXIOM, Cellebrite) | Section by section, each with its own spec (below) |
| S7 | Investigate (per case and global) | Answer-first result layouts; citation and provenance UI; chart auto-selection from result grids | Answer card, automatic charts, citations with truth-state badges, refusal and clarify cards, cross-case scope |
| S8 | Case overview | Case dossier patterns | Readiness, families, quality, recent activity, next actions |
| S9 | Evidence list and detail | Review-set UIs (Purview, AXIOM); list/detail; previews | Faceted list, saved views, bulk actions, viewer with provenance |
| S10 | Cases and New case | Directory and onboarding flows | Searchable directory, guided intake |
| S11 | Timeline and Connections | Timeline Explorer, Maltego | Real timeline, relationship graph with list alternative (blocked on backend rows 1, 2). **Timeline v5 layout rule (2026-10-02)**: one spacing system (16px between cards, 20px inside, one 16px radius), rows that stretch so cards in a row share a height and bottom edges line up (research: bento and grid guidance on base-unit rows, uniform gaps, gap-to-radius linkage): toolbar; equal family cards with trend lines; chart and inspector as one aligned row (the inspector's "Around this day" bars absorb spare height); insights, weekly rhythm and saved days as one equal row. **Timeline v4, a workbench (2026-10-02)**, from research into Timesketch (stars, tags, comments), Magnet AXIOM and Autopsy (a counts histogram you brush, filtering by artifact type), Datadog Log Explorer (facets plus a volume histogram plus a details panel) and Palantir Timeline (layers per object type): a record-family filter strip (share and trend per family; filters narrow every number), a "What stands out" row (spike, quiet stretch, leading family, each with its arithmetic stated), one chart with a stacked volume strip and a lane per family on a shared real time axis with a linked crosshair and one zoom, a live "In view" summary, busiest-day jumps, an inspector for the chosen day (total, comparison with typical day, family mix with per-family Investigate links, earlier/later, pin and note kept in this browser, an Events tab that says honestly events inside a day are not available yet and takes real events when the service supplies them), weekly rhythm, saved days, Copy briefing and Export CSV, exact values table. New backend rows 20 (hourly buckets), 21 (shared annotations) and 22 (co-occurrence clusters) extend row 1. Built only on the real `activity_by_day` counts, so it is a per-day chronology, not an event feed: individual events with exact times and openable locators still need row 1 |
| S12 | Activity, Settings, 404, error, offline | Audit and custody UIs | Complete, honest states |

### S6 Dashboard sections, each built and reviewed separately

D1 page header and primary actions. D2 workspace KPI strip. D3 readiness by case. D4 needs-review queue. D5 record families. D6 ingestion quality. D7 case workbench (queue, inspector). D8 recent questions and resume. D9 cross-case insights when the backend allows (processing history, evidence census, activity). Status of the first pass is in `dashboard.md`; every section is revisited under this guide, because the first pass carried structure over rather than designing from research.

## 6. Per-stage protocol

1. **Audit.** Screenshot current state (1440, 375, both themes). List each part, the job it serves, its data source, and mark keep, improve, replace, remove. List what is missing.
2. **Research the whole.** Two or three reference products for this kind of surface. Write what they do better and why. Cite the source.
3. **Research each component.** The best pattern for that job, with reasons, and the accessibility pattern (WAI-ARIA APG) it needs.
4. **Spec.** Purpose, question answered, data source, encoding, interaction, states (loading, empty, partial, error, forbidden), and behaviour at 1440, 1024, 768, 375.
5. **Backend needs.** Anything the UI wants that the API lacks becomes a precise row in `docs/work/BACKEND_REQUESTS.md`, and the UI shows an honest state until it lands.
6. **Build** with shared tokens and components.
7. **Verify.** Unit tests for every count, mapping and state. Live check against the real API with the preview. Zero console errors, zero overflow at 375, zero duplicate IDs, contrast measured, keyboard path walked.
8. **Show and log.** Send screenshots, update `CODEX_UI_TRACK`, move on.

## 7. Research references by component type

| Component | Study |
|---|---|
| Frame, header, sidebar | Linear, Vercel, Stripe Dashboard, Notion, Datadog, Fluent nav, Primer PageLayout |
| Search and command | Linear command menu, Raycast, Stripe search, GitHub command palette |
| Tables and lists | Carbon data table, Linear lists, Retool tables, Purview review sets |
| Charts | Carbon data visualisation, Grafana, Datadog widgets, ECharts docs, Observable Plot guidance |
| Forensic workflows | Magnet AXIOM, Cellebrite, Palantir Object Views, Maltego |
| Accessibility | WAI-ARIA APG (tabs, grid, dialog, disclosure, listbox), WCAG 2.2 |
| Tokens and theming | Fluent tokens, Atlassian tokens, Geist colour, Spectrum colour, USWDS |

Chart choice follows brief section 6: bars for comparison and ranking, 100% stacked bars for composition across groups, line or area only with real timestamps, histogram for distribution, heatmap for two dimensions, treemap for hierarchy, Sankey for lineage, network for verified relationships, and a number or table when that is clearer. Never a pie above four slices. Never a chart without a question.

## 8. Backend improvement route

The backend is built by a separate Claude session working in `api/**`. This track proposes, that session builds. Each proposal is a row in `docs/work/BACKEND_REQUESTS.md` with endpoint, exact response fields, page served, empty and error behaviour, and privacy notes. Open rows today: 1 timeline, 2 connections, 3 processing history, 4 image overlays, 5 question history, 8 evidence census, 9 collection directory, 10 activity, 11 multi-case question scope.

Candidate additions to raise as their stages begin (each needs owner sign-off before it becomes a row):

- A single workspace summary endpoint, so the Dashboard makes one request instead of one per case.
- A needs-attention feed across cases: failed sources, retained-copy gaps, stuck jobs, each with a locator.
- Case metadata the UI can trust: title, created date, last activity, and optional owner, status and priority once real sources exist.
- Saved views and pins stored server-side, so they follow the analyst across browsers.
- Global search across cases and evidence, with grouped results and a completeness flag.
- Evidence thumbnails and previews with privacy applied server-side.
- Export of an answer with its citations for a report.
- Long-running operation status with cancel, so waits are honest.

If the owner wants this session to edit `api/**` directly, they must say so explicitly; until then this track only writes requests.

## 9. Definition of done, every stage

- `npm --prefix apps/investigation-workspace run test` green with new tests for the stage.
- Live check on port 4181 against the real API; figures match the API exactly.
- Screenshots at 1440 and 375, light and dark, before and after.
- Zero console errors, zero horizontal overflow at 375, zero duplicate IDs.
- Contrast measured; keyboard path works; reduced motion respected.
- Route code-split; no unapproved dependency.
- Spec written, backend requests filed, `CODEX_UI_TRACK` updated.

## 11. Modern visual and interaction language (owner direction, 2026-09-30)

The full rules (metric-card contract, copy budget, interaction and motion, layout, sources) live in **`UI_REDESIGN_BRIEF_20260929.md` §11**, the owner's designated reference file. Every stage spec cites it. In short: one card per metric with its own status accent, size follows importance, flat tint not gradient, at most one short line of text per card, hover/focus/press interaction, accessible count-up that respects reduced motion, and no fabricated trends.

## 10. Progress tracker

| Stage | Status |
|---|---|
| Identity A tokens | applied; needs S0 completion |
| S1 shell frame, S2 header | **done 2026-09-29**, spec `UI_PAGE_SPECS/shell.md` |
| S3 top navigation and case bar | **done 2026-09-29**, in `UI_PAGE_SPECS/shell.md` |
| S4 sidebar | **done 2026-09-29** (redesigned after owner review), in `UI_PAGE_SPECS/shell.md` |
| S5 canvas and page templates | **done 2026-09-30**, `UI_PAGE_SPECS/canvas.md` |
| Team-lead revision | **done 2026-09-30**: recent questions moved to the command palette and the Investigate page (not the sidebar); header "Ask a question" and Appearance redesigned; see `UI_PAGE_SPECS/shell.md` |
| S6 Dashboard, section by section | **in progress**: D1 header, D2 KPI tiles, D3 needs review, D4 readiness by case **done 2026-09-30**; **next** D5 families, D6 ingestion, D7 workbench, D8 continue (D9 filed as backend requests) |
| S6 Dashboard | first pass built; to be redone section by section after S1 to S5 |
| S7 Investigate | **in progress (2026-09-30)**: global `/investigate` now needs no case choice; one question is asked of every case with evidence and answered per case (team-lead ruling, stopgap for backend request 11). Answers now show a chart above the exact table when the result is a ranked list or a day series of service-marked measures (`lib/resultChart.js`; anything ambiguous, such as a numeric-looking identifier, gets no chart), and the empty "does not require a result table" section is gone. An answer summary strip now states what it rests on (source files, rows behind it, evidence kind, scope; "None openable" is said plainly) and each cited file shows its floored share of the contributing rows (`lib/answerSummary.js`). The per-case empty thread now says what is ready and searched, offers suggested questions, the analyst's last questions and a link to ask every case (`pages/investigate/CaseStart.jsx`), and follow-ups are "Ask next" chips under the answer. **Per-case thread redone as a chat workspace (2026-09-30, second pass after owner review of real data)**: one framed workspace with a slim header (case, coverage, Conversation outline popover, New conversation), a centred reading column, user questions as right-aligned bubbles, assistant answers with an avatar, the finding, a one-line summary and the result (chart and exact table), a docked composer with a scope popover, a "Latest" jump, a thinking indicator, and per-message actions (Evidence, Copy, Compare, Ask a variation). Proof (sources with row shares, method, limitations) opens in one on-demand Evidence panel on the right, replacing the two always-on boxes. Research basis: 2026 chat and answer-engine patterns (centred 720-800px column, optional collapsible right panel for citations, polite live region, stop button). The all-cases results now use the same vocabulary (compact answers, per-case Evidence / Copy / Continue actions, the same on-demand evidence panel; the progress rail folds to a chip row while the panel is open). Earlier first pass: the same command landing (coverage, composer with a scope popover, suggested and recent questions), then a thread with compact question bubbles and answer cards, the composer docked under it, and a rail with the case coverage and a jumpable outline of the conversation. Live-fixture specs `investigate-slice` and `wi-ui-8` could not be run here . **Global `/investigate` recreated (2026-09-30)**: a centred command-style landing (one composer with a scope popover, suggestion cards, the analyst's recent questions) that becomes a results view with a sticky composer bar, a live progress rail (one row per case: waiting, searching or its outcome, each jumping to its result) and per-case results. Research basis: answer-engine patterns (intermediate progress, visible sources, one focused input) |
| S8 Case overview | **redone 2026-09-30 after owner review** (the first version reused the dashboard widgets and looked the same): now a case briefing. Verdict band (tone-tinted, case id, one sentence on whether the case can be relied on, Ask / Add evidence) with a readiness ring; a main column (what the case holds as ranked rows, an activity calendar of real per-day counts with an exact table, data-quality notes with rows-kept shares) beside a sticky rail (ordered next steps, needs review by cause, an inline ask box with suggested and recent questions). Pure logic in `lib/caseDossier.js` |
| S9 Evidence list | **list done 2026-09-30**: case-wide readiness strip, one-tap state filter with case-wide counts (replaces the dropdown), file-type marks, family pills, floored relative ages with the exact time on hover, accepted-rows mini bars, failed rows accented, compact/comfortable density. **Evidence viewer redone (2026-09-30)**: a review workspace: slim header (file mark, name, family, size, added age, state, Download original), an arrival strip "Exact source location" when opened from a citation, the source filling a scrolling canvas with sticky viewer headers, and one inspector with Details (only the fields the service reported; ID and hash copyable) and Lineage tabs. Unlabelled columns are worded. **Per-type viewers (2026-09-30)**: audio and video share one player (own controls, ±10 s, speed, mute, full screen for video, keyboard, derived-moment markers on the seek bar, real waveform for audio when the file is under 40 MB and decodes, never estimated) with a searchable transcript that follows playback and marks the cited cue; documents open in a reader with a page rail, find-in-document (per-page counts, next/previous across pages), copy page text and the retained-file fallback; images open in an inspector (wheel/keys/drag zoom and pan, rotate, fit, overlay toggle, region list that centres a region, cited region focused on arrival); structured rows select into a Record tab in the inspector (arrow keys move the selection). Helpers in `lib/viewerTools.js`, components in `components/viewers/`. Live-fixture specs `workspace`, `wi-ui-5`, `wi-ui-8` keep the headings they assert (`Exact source location`, `Evidence lineage`) but could not be run here |
| S10 Cases | **directory done 2026-09-30**: cards by default (state, id, readiness bar, sources/rows/review figures, families, next step), table kept as a toggle and remembered; an unreadable case says so and offers no link. **New case redone twice after owner review (2026-09-30)**: now a one-screen, two-pane wizard. Left: a live preview of the case ("Not created yet" until a file is accepted) with the URL and a progress checklist. Right: a three-segment track and one step at a time (identifier with live rules, suggestion and duplicate detection, then intake, then processing); Continue is disabled until the identifier is valid, Enter continues, Change identifier goes back, and the upload stays mounted through acceptance so no queued file is lost. Pure helpers in `lib/caseName.js` |
| S11 to S12 | not started |
