# 04. The UI/UX programme: recreate the whole application, enterprise grade

**Owner's request, 2026-11-03.** Recreate and redesign the entire interface following the instructions already given (the master UI/UX directive), because Codex is not available now. Specifically: the **Ask** page does not show its sections and answers completely, wastes side space and padding and is too compact, so tables and answers do not appear appropriately; **every evidence file must open previewed best, accurate and complete, whatever its type**; the best stats, visualisations, charts, figures and graphs from what the system computes, in dashboards and pages; a professional, up-to-date, enterprise-grade, clean, elegant, sophisticated, modern, easy-to-use, productive interface. Standing rule since 2026-10-13: **every capability is unfinished until it has its view, a Playwright test on a generated case and an owner demonstration.**

---

## 0. Who does what, where, and under which rules

| Fact | Detail |
|---|---|
| Directive of record | `docs/ux/MASTER_UI_UX_DIRECTIVE.md` (19 sections, stages U1 to U10, merge ledger, research register R1-R60) with the UX contract `docs/ux/NEXUSAI_PRODUCT_UX.md`. **Read both in full before touching the interface.** |
| `:4181` | The lane tree through Vite = **the old design with every working function and engine** (owner rule 2026-10-28). It already carries the 2026-11-02 Ask and Evidence redo (frameless 1200 px Ask column, answer cards, proof panel, paged file viewer, cited-row card). **Keep it working; never merge an interface stage into `lane-arms` on the strength of "accepted"; say what 4181 will look like afterwards and ask once more.** |
| `:4182` | The Codex worktree `C:\Users\sheik\Workspace\Office\Projects\NexusAI-ui`, branch `ui/u2-shell-navigation` (77 commits ahead of `lane-arms`, 8 behind). **U1 accepted** (colour direction **B, Ink and signal**; DTCG tokens, 160/160 contrast pairs, 150 aliases for old names). **U2 built** (shell "B · Case atlas": 56 px header, 256/64 px rail, 48 px case band, command palette, appearance menu, 12/12 viewport-theme combinations, 49/49 focused tests) and **awaiting the owner's review**. U3 to U10 not started. |
| Executor from now | **Claude, in that worktree**, under the same directive and the same per-stage loop (research note, two or three options the owner can look at on `:4182`, the owner decides, build over the existing tested data hooks, prove, close with a stage note and evidence). The worktree needs the owner's env file copied by the owner (never read or printed by Claude) and `npm ci`. |
| Forbidden to the interface executor | `api/**`, `db/**`, `ingestion/**`, `tools/**`, `evaluation/**`, `docs/data/**` (file backend requests in `docs/work/BACKEND_REQUESTS.md` instead). The backend executor edits no interface path while a stage is open and says so in `NEXUSAI_CONTINUATION.md`. |
| Real data | Screenshots, fixtures and design tools receive **synthetic cases only** (Orion, the wave-1 cases). O-8 (offline tile licence) blocks any screenshot leaving the machine. |
| Dependencies | Approved: react, react-dom, react-router-dom, lucide-react, echarts, cytoscape, maplibre-gl, yaml, Noto fonts, vite/vitest/Testing Library/Playwright. **Everything else needs the owner's yes** with the trade-off written; the media viewers need six proposals (section 7). |

## 1. What the redesign must fix first (acceptance list taken from the owner's words)

1. **Ask: use the width.** No decorative side gutters beyond 16 to 24 px; the answer column fills the space to a readable maximum for prose and goes full width for tables and media; **tables, charts, maps and graphs are never squeezed into a bordered card with their own padding**; sections do not hide: every section of an answer (headline, interpretation, table, chart/map/graph, evidence, "how this was computed", actions) is reachable and **visibly complete** (row counts "50 of 408", no silent truncation).
2. **Evidence: open anything, show everything.** The viewer rule of `03_architecture.md` 5.3: if it is stored it opens; completeness stated; the file's own headings and order; the cited row and its neighbours; documents with all passages in order; media with synchronised transcripts and boxes.
3. **Dashboards that answer questions.** Every tile and mark opens the rows behind it. No decorative numbers, no fabricated trends, no empty chart: a state the service cannot speak for is a designed state.
4. **One visual system**, light and dark and high contrast, English and Urdu (right-to-left) deliberately handled, keyboard complete, WCAG 2.2 AA.
5. **Speed that is visible:** the interface never waits silently; the Ask page narrates phases ("understanding, writing the query, checking it, running it, reading the result").

## 2. Design language (decided; do not reopen without the owner)

* **Colour direction B "Ink and signal"** in light, dark and high contrast; one accent; colour only for meaning; evidence strength by **word and shape** (observed, inferred, candidate, confirmed), never colour alone. The owner liked the old teal themes and rejected the "Paper" palette on the whole app (D23); direction B was chosen by looking (O-3).
* **Type:** Noto Sans Variable (Latin), Noto Sans Arabic for Urdu UI strings, Noto Sans Mono for identifiers and tabular figures. Urdu **body text** in Nastaliq (a Noto Nastaliq Urdu face) needs a native reader to approve (O-6); until then Naskh via Noto Sans Arabic.
* **Bidirectional text:** `dir="auto"` on user content, `<bdi>` around identifiers, **phone numbers, plates and account numbers always left-to-right**, logical CSS properties, mirrored layout only for fully Urdu UI.
* **Space and density:** the space tokens 8/12/16/20/24/32/48; density modes comfortable, default, compact (a global setting); tables default to compact with 40 px rows and sticky headers.
* **Depth and motion:** quiet; motion only where it explains (arrival of a result, selection, source-row tint); zero motion when the user prefers reduced motion.
* **Forbidden looks:** flat white or black canvas, purple "AI" gradients, neon, glassmorphism, card soup (every element in its own bordered card), emoji as status, skeleton shimmer that outlasts the request, fake progress, looping motion, tooltips holding the only copy of important text, icon-only controls without a name, more than one accent.
* **Language of the interface:** no jargon for analysts (never model, agent, operation, template, processor, backend, SQL, vector, embedding, threshold, token count); one level down for administrators.

## 3. Information architecture

**Global (rail):** Dashboard · Ask across cases · Cases · Activity · (Library: reference lists such as handset types, operators, cell sites, plate formats, for administrators) · Settings. **Header:** case switcher with the full case name, command palette, appearance, user. **Case band (sections):** Overview · Evidence · Ask · Analysis (Connections, Places, Timeline, Money, Vehicles, Media) · Entities · Findings · Reports · Activity/Audit · Access.

| Page | Purpose (one idea) | Primary actions |
|---|---|---|
| Dashboard | what needs attention across my cases | open a case, resolve an attention item, continue a thread |
| Cases / New case | find and create | create (wizard), open, filter |
| Case Overview | the dossier: what this case holds and what to do next | ask, add evidence, open a finding |
| **Ask** | question to defensible answer, with history | ask, refine, pin, export |
| Evidence | what is in the case and its state | add, open, reprocess, tag |
| Evidence viewer | the file itself, complete | read, search, cite, review |
| Connections / Places / Timeline | how things link, where, when | filter, select, ask about this |
| Money | where money came from and went | trace, compare, flag |
| Vehicles | plate sightings and routes | search a plate, replay a route |
| **Media** | gallery of images, audio, video, faces, plates, text found | filter, review, compare |
| Entities | people, numbers, accounts, vehicles, devices | merge candidates, notes |
| Findings | claims with evidence | write, reorder, export |
| Reports | build and sign | assemble, preview, sign |
| Activity | processing and history | retry, inspect |
| Audit / Access | who did what; members | review, grant |

## 4. The pages in specification form

### 4.1 Ask (stage U4; the owner's most urgent page)
**Layout (wide screens):** three zones in one frame: a collapsible **history rail** (left, 280 px), the **conversation** (centre, fills the rest to a 1280 px maximum for text blocks, full width for tables/media), and a collapsible **evidence drawer** (right, 420 px) that shows "behind this answer" for the selected turn or mark. On narrow screens the rails become sheets. The composer is pinned at the bottom with a scope line ("Case Orion · all families · 1 Feb to 3 Mar") and suggestion chips **generated from the case's coverage** (not canned).

**Answer anatomy (fixed order, every part optional but never hidden):**
1. *Interpretation chip* ("Interpreted as: ..."), editable.
2. *Headline* (the engine's first sentence, number-locked) with the evidence-strength word.
3. *Result block*, by shape: figure card; **table** (sticky header, resizable and pinnable columns, sort, filter, copy, CSV, "open source row"; virtualised beyond 500 rows; always "showing 1 to 50 of 408"); chart; map; graph; timeline; passages; **media moments** (thumbnail strip, audio snippets with transcript lines, "open at 00:12:34").
4. *Reading* (the model's plain-words interpretation, labelled, with the computed values only).
5. *Evidence tabs:* rows behind the figure (with the file and row in words), records behind each summary view, per-line checks, source row on demand.
6. *How this was computed* (plain-language steps; the technical query one level down for administrators).
7. *Actions:* pin to findings, ask a follow-up, re-run, compare with earlier, export, "this answer is wrong" (feeds the triage bank).

**States designed as results, not errors:** clarification, abstention ("I could not show that the query asks everything you asked; here is what is missing and the options"), refusal, too broad, no data in this case, waiting (phases), partial, failed with next step.

**History rail:** threads grouped by day, search, pin, rename, archive, share; opening a thread restores the page; "changed since" badges on re-runs (`03_architecture.md` section 7).

**Acceptance (proved by screenshot at 4 widths × 3 themes on Orion):** an answer with a 408-row table, a chart and a map shows all of them, with no horizontal page scroll, no clipped section, no empty side bands wider than 24 px, keyboard path from composer to evidence drawer, and the same page in Urdu.

### 4.2 Evidence and the viewers (stage U6)
* **Library:** table with type, state (per-stage progress), rows/pages/duration, languages, quality, added by/when, tags; filters by family/state/tag; bulk reprocess/tag; "add evidence" with drag and drop, folder and archive upload, per-file progress, type detection preview, the mapping wizard for unknown layouts.
* **Evidence passport** (first tab of every file; `03_architecture.md` 5.2).
* **Viewers:** the registry of `03_architecture.md` 5.3 in the new design system, carrying over **FileRowsViewer, SourceRowCard and the document passage assembly** built on 4181. Specific requirements: spreadsheets show the **cell address** and the file's order, dates read as dates with the raw serial one click away; PDFs show a page rail and OCR overlay; text exports show lines with encodings; each viewer has search, "cite this" (creates a citation chip usable in findings), and the completeness line.
* **Media viewers (with WP-M0/M2/M3/M6):**
  * *Image:* zoom/pan with tiles, EXIF panel, OCR overlay toggles, face/plate boxes (governed), "similar images", compare two images side by side, blur-by-default option.
  * *Audio:* waveform with region selection, **synchronised transcript** (click a word to play, follow-along highlighting), speaker lanes, per-segment language chips, low-resource banner, spoken-number highlights, translation toggle, edit-and-verify with history, link to the matching call record.
  * *Video:* player (HLS/MP4 proxy), **event scrubber** (shots, motion, tracks, plates, faces, speech as lanes), frame step, clip cutter with hash watermark, detections overlay with confidence, transcript pane, clock-offset control showing both stamps.
  * *Faces (when enabled):* person-cluster grid, face detail with every appearance, **compare** two faces (side by side with the "this is not an identification" notice), search with a candidate list and the error-rate statement, review queue.

### 4.3 Dashboard, Cases, New case, Case Overview (stage U5)
* **Dashboard (hero):** a band answering "what needs me today": attention queue (failed ingests, items awaiting review, new cross-case notices, long-running jobs), KPI tiles over **real** values (cases, evidence by state, questions asked, findings), per-family coverage, model/queue health, continue cards (last threads, last viewed evidence). Every tile opens its rows.
* **New case wizard:** name and number; case profile (type, jurisdiction, time zone and **clock basis**, languages expected, capabilities to enable such as faces, with a place for the authority reference); members; evidence upload with live detection; first-readiness summary with suggested questions.
* **Case Overview (dossier):** header with state and owner; **what this case holds** (family cards with counts and quality), activity calendar, key entities and numbers (top contacts, accounts, plates) as real mini-visuals, data-quality ledger, next steps, recent findings and threads, cross-case notices. No ring without a question it answers.

### 4.4 Analysis pages
* **Connections (workbench, stage U3):** links, places, table, timeline coordinated; layout B was chosen by the owner; restyle in the new system plus the research items already specified (table twin and marks listbox, time-bar brushing with a reset chip, level-of-detail labels, hidden-count banner, uncertainty circles for cell sites, muted basemap, stable layout seed).
* **Places:** cell sites with **uncertainty** (a coverage area, not a position), visits, dwell, movement anchors, speed alerts, co-location; offline basemap; heat and cluster layers.
* **Timeline:** one lane per family (calls, messages, data, bank, plates, media moments, documents), brush to filter, annotations stored server-side.
* **Money (new, with the transactions family):** running-balance line, in/out waterfall, counterparty bars and **Sankey** of flows, structuring bands (just below limits), velocity, circular flows; every mark opens the statement rows.
* **Vehicles (new):** plate search, sightings table, camera map, route replay, convoy candidates.
* **Media (new):** gallery with filters (type, language, date, camera, review state, has faces/plates/text), search by words (OCR, transcript, caption) and by example image, face clusters, plate hits, review queue shortcut.
* **Entities and drawer (stage U7):** "Ask about this" from any chip; mentions across families; cross-case notices; notes.

### 4.5 Findings, Reports, Activity, Settings (stages U7, U8)
Findings editor with live evidence references; report builder (sections, exhibits, number lock, sign-off, bundle); Activity with real job history (backend request 27); Settings (appearance, languages, density, shortcuts, model roles status for administrators); Admin (users, roles, audit viewer).

## 5. The visualisation catalogue (what to draw, from which view, with which library)

All charts use the token-driven ECharts theme; the graph uses Cytoscape; maps use MapLibre with the offline raster basemap. Every chart has a **table twin** and an **honest empty state**, and every mark opens its rows.

| Question the analyst has | Visual | Source | Notes |
|---|---|---|---|
| Who does this number talk to most? | ranked bars with call count and talk time | `v_contact_pairs` | click a bar: the pair's events |
| When is this number active? | **hour × weekday heatmap** and a daily calendar | `v_comm_events` | night/odd-hour highlight |
| How long are calls? | distribution (histogram) and the longest-call list | `v_comm_events` | parameters (minutes) read from the question |
| First and last contact, bursts | event strip with burst detection | `v_comm_events` | |
| Who connects whom? | **link chart** with communities, degree/betweenness sizing, time slider, path finding, ego network | `v_connection_edges` | cap and level-of-detail rules; candidate edges dashed |
| Where was the phone? | map of cell visits with **uncertainty circles**, dwell, movement trail | `v_cell_visits`, `v_movement_anchors` | "may have been near", never "was at" |
| Who met? | co-location windows matrix and map | `v_colocation_windows`, `v_meeting_candidates` | proximity is not association |
| Did the rhythm change? | before/after bands | `v_rhythm_changes` | |
| Which handsets/SIMs? | pairing timeline, SIM-swap markers | `v_device_pairings`, `v_successor_candidates` | |
| What did the device do online? | app/host treemap, bytes up/down area, session timeline | `v_data_sessions`, `v_app_usage` | |
| Where did money go? | running balance, waterfall, **Sankey** counterparties, velocity, limit bands | `v_bank_transactions`, `v_account_flows`, `v_account_limit_bands` | statement integrity banner |
| Where was the vehicle? | camera map, sightings timeline per plate, route replay | `v_plate_sightings`, `v_plate_visits` | plate masking per D28 |
| What do documents mention? | mention network, term timeline | `v_document_mentions` | |
| What is in the media? | counts by type/language, face-cluster grid, appearance timeline, speech language lanes | media views (033) | "model observation" label |
| Is the evidence healthy? | stacked accepted/duplicate/rejected, layout confidence | `v_evidence_inventory`, typing report | |
| What is the portfolio doing? | cases by state, evidence by state, queue depth, questions per day | case/evidence reads | real series only |

**Runtime visual selection (consistent with "no templates"):** the Ask result block chooses a chart **by the shape of the returned data** (the existing `resultShapes` logic), never by the question text; the model may *suggest* follow-up visuals as validated queries over the same views.

## 6. Quality, accessibility and performance (per stage, binding)
* Tests: vitest for every state/mapping/count a stage adds (the existing 549 stay green); **Playwright screenshot tests on generated cases at 1440, 1280, 1023 and 390 px × light, dark and high contrast**; axe checks; a keyboard-only path per page.
* Budgets: INP p75 under 200 ms; initial JS/CSS gzip budgets from the U1 baseline (foundation CSS 3.8 KB gzip); virtualised tables beyond 500 rows; no layout shift on result arrival.
* Right-to-left audit of every page at U10; native screen-reader pass of the core loop ("go to a case, open Ask, open a source row").
* **Usability test** with real analysts at U10 (names owed: D26): unaided "add data, ask, open a citation, pin a finding".

## 7. Dependencies the media interface will need (proposals; each needs the owner's yes and a licence-ledger row)
| Proposal | For | Licence [K] |
|---|---|---|
| wavesurfer.js | audio waveform and regions | BSD-3-Clause |
| hls.js | HLS video proxies | Apache-2.0 |
| PDF.js | PDF pages with text layer | Apache-2.0 |
| OpenSeadragon (or tiles in MapLibre) | very large images | BSD-3-Clause |
| a virtual-table helper (TanStack Table/Virtual) | 10k-row tables | MIT |
| docx-preview or server-side render | Word documents | MIT / server |
Rendering Word, Excel and PDF **on the server to a safe PDF/HTML** is the alternative that avoids shipping parsers to the browser; decide in the U6 research note.

## 8. The stage plan (revised order, a proposal for the owner's yes: D37)

The owner-set order was U2 review, U5 (section by section), U3, U4, U6. The owner's new message puts Ask and Evidence first, and U4/U6 are the pages with the most backend work waiting (conversations, media). Proposed order, with parallel backend work:

| Stage | Content | Needs from the backend | Proof the owner looks at | Weeks |
|---|---|---|---|---|
| U2 close | owner reviews the case-atlas shell on `:4182` | none | the review | 0.5 |
| **U4 Ask + History** | section 4.1 | BR-2 conversations (Phase 1); until then the page uses the current client-side turns and says so | the answer anatomy on Orion, every state, history rail, keyboard loop, Urdu | 3 |
| **U6 Evidence + viewers v1** | section 4.2 (structured, documents, text, images, audio v1, video v1 with WP-M0) | file-rows (live), media proxies (WP-M0) | open every one of the 53 Orion files plus the media samples; "unaided add data and open a citation" | 3 |
| U5 Dashboard, Cases, New case, Overview | section 4.3 | BR-1 (honest states until it lands) | no fabricated number; every tile opens rows | 3 |
| U3 Workbench restyle | section 4.4 | none | the same walkthrough in the new system | 1.5 |
| U7 Entity drawer, findings, saved views | 4.4 and 4.5 | BR-3 | "Ask about this" from any chip | 2 |
| U9 Media workbench and review desk | media pages, review desk | WP-M2..M8 | review with reason and undo | with media |
| U8 Activity, Settings, Admin, Reports | 4.5 | BR-4 | honest states; roles | 2 |
| U10 Polish, sweep, usability test | everything | none | the unaided test; alias layer and classic Connections deleted | 2 |

## 9. Cut-over: when `:4182` replaces `:4181`
Criteria, all required: (1) **route and function parity** with the checklist generated from `:4181`'s 17 routes and every action in them; (2) all vitest and Playwright suites green on `:4182`; (3) the owner accepts each page by looking; (4) the Ask and Evidence carry-overs are done; (5) a rollback tag `ui-old-final` on the last old-design commit. Then, and only then and with the owner's word, merge the UI branch into `lane-arms`, restart the workspace, and keep `:4181` as the old design on a different port for one week. Until the owner says so, **`:4181` keeps the old design.**

## 10. What to do first in the interface lane
1. Owner reviews U2 on `:4182` (nothing to build; remind them).
2. Port the 2026-11-02 Ask/Evidence components into the new system as the starting point of U4/U6 (they hold the learned fixes: completeness lines, file headings, cell addresses, passage assembly).
3. Write the U4 research note and two Ask layouts as **real routes on `:4182`** (`/design/ask-a`, `/design/ask-b`) over the same data hooks, so the owner can look at both on Orion before a line of production markup is final.
4. In parallel the backend lane delivers BR-2 (conversations) so the history rail is real.
