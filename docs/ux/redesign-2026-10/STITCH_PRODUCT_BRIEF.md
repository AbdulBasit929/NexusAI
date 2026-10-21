# NexusAI — Product brief for Google Stitch

**Purpose:** give Stitch (and any designer) the full picture of what NexusAI is, who uses it, what every screen holds, and what is real versus planned, so that every screen it generates fits the product.
**Status:** v1, 2026-10-20. Companion to [`README.md`](README.md) (the plan) and the binding [`../NEXUSAI_PRODUCT_UX.md`](../NEXUSAI_PRODUCT_UX.md).
**Data rule:** everything below is synthetic. Never paste real case data, real phone numbers, real file names or screenshots of the private/real cases into Stitch (it is a cloud service). Section 10 lists what is safe.

How to use it: **Part A** is short enough to paste at the top of a Stitch prompt (and later becomes the Overview of `DESIGN.md`). **Parts B–F** are the reference; paste only the section that belongs to the screen being designed.

---

## Part A — Master context (paste this first)

Design the web application **NexusAI**: a private, case-centred investigation workspace for forensic and intelligence analysts working in law-enforcement and regulatory teams.

Analysts open a **case**, add evidence — phone-call records (CDR), internet-session records (IPDR), vehicle-plate sightings (ANPR), subscriber details, cell towers, bank transactions, access logs, plus documents, images, audio and video — and then **ask questions in plain English**. Every answer is computed from the source records and cites the exact row, page, timestamp or image region it came from, so an analyst can defend it in court. The product also shows **connections** between phone numbers (a link chart), **places** (cell sites on a map) and a **timeline** of activity.

**It is an evidence system that happens to use AI — not an AI chat app.** Provenance is the main thing on screen; the answer comes second. The product says "I can't answer that from this evidence" or asks one clarifying question rather than guess. That honesty is the feature.

**Users:** analysts (not engineers), working for hours at a time, mostly on a desktop (1440–1920 px) and sometimes a tablet in the field. They scan dense tables and need to trust every number.

**Feel:** calm, confident, precise, human. Think a well-made professional reference or instrument: editorial clarity in the typography, the density of a financial terminal in tables, none of the sci-fi control-room look. Quiet colour, one restrained accent, colour only where it carries meaning (status, confidence, chart series). Light theme first, with a properly designed dark theme (not an inverted one). Typography and spacing do the hierarchy, not borders and shadows.

**Avoid:** glassmorphism, neon or glow, teal/purple gradients, "AI sparkle" iconography, a chat-bubble-centred layout, hero banners, illustrated empty states, every element in its own bordered card, emoji, decorative or looping motion, a "hacker" dark-only look.

**Structure:** the product is organised by **case → evidence → question**, never by file type. Global navigation: Dashboard, Ask, Cases, Activity, Settings. Inside a case: Overview, Evidence, Ask, Connections, Timeline, Activity, Data. Evidence types (CDR, IPDR, ANPR, Subscriber, Tower, Financial, Access log, Documents, Images, Audio, Video) are **filter chips, never menu items**, so a new type adds a chip, not a page.

**Hard rules:** show only controls that really work; never show model names, SQL, tokens or "confidence thresholds" to analysts; status is never conveyed by colour alone; WCAG 2.2 AA in both themes; full keyboard path; no horizontal page scroll at 375 px; motion under 200 ms and optional.

**Content:** English interface; evidence content can contain Urdu (right-to-left) text, so design for mixed-direction text. Phone numbers look like `923001110001` and must use tabular figures. Use realistic content, never lorem ipsum.

---

## Part B — People and jobs

| Who | What they need from the screen |
|---|---|
| **Analyst** (primary) | Understand one case; ask evidence-grounded questions; verify any figure in one click; find the same answer again later |
| Case supervisor | See readiness and quality at a glance; review decisions; reports |
| Evidence operator | Add files safely; see exactly what was accepted, processed, failed; retry |
| Auditor | Read who asked what and who added what; never alter evidence |
| Administrator (separate, not in analyst navigation) | Users, models, health. Out of scope for this redesign |

**The sentence every screen serves:** an analyst with no technical training opens a case, adds evidence, asks a question in plain English, and receives an answer they can defend in a courtroom, with every number traceable to a source row they can open.

---

## Part C — Vocabulary (use these words on screen)

| Term | Meaning |
|---|---|
| **Case** | A body of evidence for one investigation (today identified by a case ID; display names and status are planned) |
| **Source** | One uploaded file or evidence item. States: `queued · processing · ready · failed · excluded`, and **"retained copy missing"** (record kept but the original file is gone) |
| **Record family** | The kind of structured data: call detail records, internet session records, vehicle plate sightings, subscribers, towers, transactions, system access logs |
| **Question → Answer** | A question in plain English; an answer that is one sentence with a headline figure, then a table/chart/map, then citations |
| **Citation** | A pointer to the exact source: file, row number, page + text span, audio time range, video frame time, image region. A citation that cannot be opened is not shown |
| **Evidence strength** | How much to trust a claim: a source row (strong) vs a machine-read fragment such as OCR or transcript with a stated confidence (weaker). They must never look identical |
| **Derived artifact** | Something a model produced from media (an OCR text, a transcript, a plate read). Always labelled as derived, never presented as a camera sighting or a record |
| **Clarification** | Success state: "I can answer this, I just need one detail", with 2–4 one-click options |
| **Coverage** | What a question actually searched ("11 of 12 sources ready"); failed sources are named, never silently dropped |
| **Link** | A recorded contact between two numbers. Not a finding about the people |
| **Cell site** | A coverage area with a stated radius. Not the position of a person |

Capability states used by the product: `operational · processing · processing required · no case data · manual review · disabled by policy · administrator only · unavailable · unknown`.

---

## Part D — Screen inventory

For each screen: **purpose · what it shows · main interactions · states to design**. "Real" means backed by the live service today; "Planned" means design it, but it needs backend work (see Part E).

### D0. App shell (every screen)
- **Header strip:** brand mark, global search / command palette (find a case, source or question; `Ctrl K`), primary "Ask a question" action, theme control.
- **Navigation rail:** Dashboard, Investigate (ask across cases), Cases (with the open cases listed beneath, each with an attention badge), Activity, Settings. Collapsible to icons.
- **Case header** (inside a case): case ID with copy, record count and date span, a status chip ("1 source needs review"), quick actions (Ask, Add evidence, Switch case), and the **case tab strip**: Overview · Evidence · Ask this case · Connections · Timeline · Activity (· Data).
- Skip link, visible focus, keyboard-roving tabs.
- States: collapsed rail, mobile drawer, API unreachable (say which part is unavailable and what still works).

### D1. Dashboard `/`
- **Purpose:** answer "what should I do next?" in five seconds.
- **Shows (Real):** four headline figures (Needs review, Ready x of y with %, Processing, Structured rows); **activity over time** by record family (chart/table toggle, scope by case, 30 d / 7 d, days/weeks, zoomable with a mini-map); **what needs review** (failed sources, retained-copy gaps, grouped by cause and case, each openable); what the evidence is made of (family breakdown); a cases table; suggested questions; "Ask your own".
- **Interactions:** filter the review list, open an item, scope by case, switch chart/table, add evidence.
- **States:** no cases (one action: create your first case), loading, one case's figures failed to load (say so, never zeros), updated-just-now / refresh.

### D2. Cases `/cases` and New case `/cases/new`
- **Cases (Real):** a directory of cases as cards or a table: state first, then name, readiness, the figures that decide whether to open it, and record families. Search, sort, filter. A case that could not be read says so and offers no link.
- **New case (Real, 3 steps):** choose the case identifier → add the first evidence → watch processing. A live preview of the case stays "not created yet" until the first file is accepted. Planned: display name, reference number, description, members.
- **States:** empty, identifier already taken, upload failed with reason.

### D3. Case Overview `/cases/:id/overview`
- **Purpose:** a briefing on one case: a verdict first, then what to do next.
- **Shows (Real):** verdict ("1 source needs review. Answers cover the 11 ready sources") with a **readiness ring**; four figures (Sources, Structured records, Ingestion yield, Last ingested); **what is in this case** by record family with share bars; **next steps** (review failed source, ask a question); **event horizon** calendar heat-strip of activity by day (opens the timeline); **data-quality notes** as plain sentences; a question composer; analyst notes and leads (browser-only today).
- **States:** processing, partial, failed sources, no evidence yet.

### D4. Evidence `/cases/:id/evidence`
- **Shows (Real):** inventory table: name, record family, size, added when, **status**, derived artifacts; filter chips by family and status; **Add data** drop zone (drag anywhere, multi-file, per-file progress); the system states what it decided about each file with one-click correction; failed rows are visible with a reason and **Retry**.
- **Never:** ask the analyst to choose a parser; silently drop failures from counts.
- **States:** empty (drop target + one sentence), mixed processing, all failed, upload in progress, retained copy missing.

### D5. Evidence detail and viewers `/cases/:id/evidence/:eid`
- **Shows (Real):** only facts the service reports about the file; a **lineage panel** (source file → version → derived artifacts → what produced each, with confidence); the viewer for the type:

| Type | Viewer | Must support |
|---|---|---|
| Document | paginated reader | page navigation, find, **deep-link to page + text span**, highlight on arrival |
| Structured | virtualised table | sort, filter, row number and row hash on demand, jump to row |
| Image | image with overlays | zoom, OCR regions and plate boxes as toggleable overlays with confidence (Planned: overlay data), original always viewable |
| Audio | player + transcript | waveform, transcript synced to playback, click a line to seek, deep-link to start time, speaker turns when available |
| Video | player + event timeline | derived events pinned on the scrubber, deep-link to timestamp |

- Arriving from a citation must highlight the exact span, pulsing once.
- **States:** loading, preview unavailable, file too large, retained copy missing, derived text withheld (privacy).

### D6. Ask this case `/cases/:id/investigate` — the core screen
Fixed vertical order, never reordered by answer type:
1. **The answer** — one sentence, largest type on the screen, inline claim-level citation markers. e.g. "There are 8,642 call records in this case."
2. **The result** — the number, table, chart, link chart or map it came from; column headers are plain display names ("Call duration"); table always one click away.
3. **Citations** — numbered to match the markers, hover preview (file, row/page, time, ≤200-character excerpt), click opens the source at the exact locator; evidence-strength badge per citation.
4. **How this was derived** — collapsed by default, plain language ("Counted rows in Call Detail Records for this case").
5. **Limitations** — only when real: excluded failed sources, low confidence, partial coverage.
6. **Follow-ups** — 2–4 concrete next questions from the current result.

Around it: the **question input** (Enter to ask, Shift+Enter newline), **scope chips** (all evidence / family / date range; filters fail loudly and never silently widen), the **thread** of questions asked so far (an investigation is a sequence; a new question never replaces the answer being read), question history, comparison of two answers.
**All states are first-class:** answered · **clarify** · zero results (state the filters applied, offer to widen explicitly) · unsupported (say what is possible) · partial · processing ("3 files still processing; this answer covers the 12 ready") · failed (plain words, an action, a reference code). A deterministic answer appears first and complete; optional narration streams below, visibly marked as interpretation.

### D7. Ask across cases `/investigate`
- **Shows (Real):** one input; a statement of what will be searched (every case with evidence and how many of its sources are ready); suggested questions drawn from what the cases say they can answer; the answer comes back **grouped per case, best first, with each case's coverage**; optionally narrow to chosen cases.

### D8. Connections `/cases/:id/connections`
- **Shows (Real):** a **link chart** of phone numbers and the contacts between them: carrier badge per number, edge weight = number of contacts, selection panel with the number's details, a table alternative of the same data, "overview mode" that groups contacts into communities above about 1,500 numbers (click a group to open it). Caption on honesty: a link is a record of contact, not a finding about the people.
- **Interactions:** select, expand neighbours, filter by family or date, open the records behind an edge.
- **States:** one number with no contacts, huge graph, no data, loading, keyboard alternative to the canvas.

### D9. Timeline and Places `/cases/:id/timeline`
- **Chronology (Real, by day):** a rail of family filters, weekly rhythm and pinned days on the left; one chart with two views on a shared time axis (stacked volume; a lane per family) with zoom and a live summary of the window; an **inspector** for the chosen day (busiest day pre-selected); events inside a day open in Ask. Planned: hours, individual events, clusters, shared annotations.
- **Places (Real):** cell sites on an offline-capable map with each site's radius; "a cell site is a coverage area, not a person's position".
- **States:** no dated records, a single day, sparse data, offline map tiles.

### D10. Activity `/cases/:id/activity` and `/activity`
- **Case activity (Real, browser-based):** a journal of questions and evidence updates grouped by day, a reading pane for the chosen entry, summary cards (outcomes, last two weeks, what this record does and does not cover). Says plainly it is a working record, not an audit history.
- **Workspace activity (Real, browser-based):** an inbox across all cases: views (Needs attention, All, Questions, Evidence), the cases, mark reviewed (J/K to move, E to mark). Planned: true audit and chain-of-custody history.

### D11. Data `/cases/:id/data`
- **Shows (Real, owner-only):** the inventory of every file, number, tower and internet record as typed tables, searchable and sortable, plus towers on a plain offline map; extractable.

### D12. Settings `/settings`
- **Shows (Real):** Appearance (preview each theme), browser data (what is stored, clear each kind, export), shortcuts reference, what these settings do and do not cover. No identity or roles (none are real yet).

### D13. Cross-cutting screens and patterns
Command palette · keyboard shortcut sheet · toasts · confirmation dialogs · error boundary page · 404 · "API unreachable" state · loading skeletons (never longer than the request) · mobile drawers.

---

## Part E — What is real, and what to design for later

**Real today (design with confidence):** case list and per-case counts; upload and processing status; reprocessing a failed source; viewing evidence content; plain-English questions with answers, tables, citations, clarifications, limitations; link chart, map and table result views; Connections and Places data; activity by day and family; document/structured/image/audio/video viewers; a report-generation endpoint exists but has **no screen yet**.

**Stored only in this browser today:** pins, notes and leads, question history and drafts, reviewed flags, theme.

**Planned, needs backend (design it, mark "Planned" in your notes, never fake it in the shipped UI):**

| Backend request | Unlocks |
|---|---|
| Real login and identity; case record with name and status; membership and roles (requests 18, 23, 24, 25) | Sign-in, "My cases", case status, sharing |
| Server-side question history (5) | History that follows the analyst across devices |
| Audit and chain-of-custody history; who registered what (10, 26) | True Activity and custody views |
| Unified timeline events, hourly buckets, event clusters (1, 20, 22) | Event-level timeline, "a call and a plate read within five minutes" |
| Shared timeline annotations: pin, note, tag, star (21) | Team-shared notes on the timeline |
| Image overlay data: OCR regions and plate boxes with confidence (4) | Image viewer overlays |
| Report history: list, open, download (17) | A Reports surface |
| Workspace summary, needs-attention feed, live events, data-quality per family (12, 13, 15, 16) | Faster, live Dashboard and richer Overview |
| Multi-case question scope handled by the server (11) | Faster, exact "ask across cases" |

Source of truth: [`docs/work/BACKEND_REQUESTS.md`](../../work/BACKEND_REQUESTS.md).

---

## Part F — Components and visualisations

**Signature components:** AnswerBlock · CitationMarker · CitationPreview (hover) · SourcePanel · EvidenceStrengthBadge · ClarificationPrompt · ScopeChips · AskInput · FollowUpChips · DerivationDisclosure · LimitationsNote · ResultTable (virtualised) · EvidenceInventory · EvidenceStatusPill · AddDataDropzone · LineagePanel · Document/Table/Image/Audio/Video viewers · CaseHeader · CaseSwitcher · ActivityFeed · DataQualityNotes · ReadinessRing · EmptyState · FailureState.

**Visualisations needed:** activity-over-time chart with zoom and mini-map; stacked volume and per-family lanes; calendar heat-strip; readiness ring; family share bars; link chart with communities; map with radius circles; bar/line/table results from questions; waveform; sparklines in tables.

**Chart honesty rules (design to these):** a distinct, accessible palette for series that also works in dark mode and for colour-blind viewers; direct labels over legends where possible; tabular numerals; axis and units always stated; observed vs inferred vs candidate must look different; uncertainty is drawn; every mark opens the rows behind it; "proximity is not association"; a table alternative for every chart.

---

## Part G — Constraints from the product contract (do not break)

- Layered surfaces, not flat canvases: light is cool/warm neutral, never pure white everywhere; dark is deep slate, never pure black; four elevation levels in both.
- Accent colour = interactive only. Status colours = status only.
- Density is a feature: compact, sortable, filterable tables with sticky headers and right-aligned numbers; no generous padding on a 10,000-row table.
- Forbidden: glass, neon, purple AI gradients, marketing hero blocks, card soup, decorative motion, emoji as status, progress bars that do not reflect real progress.
- Responsive: ≥1280 three zones (navigation, working surface, contextual rail); 768–1279 rail becomes a drawer, tables scroll with a pinned first column; <768 single column, 16 px gutter, 44 px touch targets.
- Accessibility: WCAG 2.2 AA text 4.5:1, UI and focus 3:1 against every adjacent surface in both themes; live regions announce streaming and processing once; real `<table>` semantics.
- Language: never "M1", "template", "route", "planner confidence", stack traces. Say "Call duration", "Something went wrong running this query. Reference: QRY-4821".

---

## Part H — Realistic synthetic content (use instead of lorem ipsum)

**Case:** `CASE-2026-0142` · "Harbour Road" · 12 sources, 12,912 structured records, 2025-01-01 to 2026-07-18, 91% ready (11 of 12), 1 source failed.

**Record families (counts):** Call detail records 8,642 (67%) · Internet session records 2,500 (19%) · System access logs 1,000 (8%) · Vehicle plate sightings 750 (6%) · Subscribers 11 · Cell towers 5 · Financial transactions 4.

**Sources (names):** `cdr_july_2026.csv` (ready) · `ipdr_export_q2.xlsx` (ready) · `anpr_gate_3.csv` (ready) · `tower_list.csv` (ready) · `witness_statement_scan.pdf` (ready, 6 pages) · `interview_audio_0714.m4a` (processing) · `bank_ledger_march.xlsx` (failed: "Header row not found") .

**Numbers:** `923001110001`, `923001110002`, `923005550142` (use only these prefixes).

**Questions and answers:**
- "How many calls did 923001110001 make in July 2026?" → "923001110001 made **312 calls** in July 2026." (table of top counterparties; citation markers [1][2]; follow-ups: "Which numbers did it call most?", "Show the busiest day")
- "Who is connected to 923001110002?" → "923001110002 shares **3 contacts** with 923001110001: …" (link chart)
- "Which towers did 923005550142 use on 2026-07-14?" → map + table
- Clarify: "What's the total for 923001110001 in August?" → "By "total", do you mean: [Call duration] [Data volume] [Transaction amount]"
- Zero result: "No calls from 923001110001 in August 2026." + the filters applied + "Widen to all dates"
- Unsupported: "This deployment cannot analyse video scenes. It can search the transcript and the timestamps of derived events."
- Processing: "1 file is still processing; this answer covers the 11 that are ready."
- Urdu snippet (RTL) in a transcript line: `رات کو گیٹ پر ملاقات ہوئی`

---

## Part I — Safe and unsafe inputs for Stitch

| Safe | Never |
|---|---|
| Part A–H of this file | Real MSISDNs, real file names, real subscriber details |
| Screenshots of the synthetic demo case `nexusai-forensic-demo` | Anything from `nexusai-real-private-demo` or any real-data case |
| Counts and names from Part H | Screenshots of the Data inventory page (it can hold real values) |
| Competitor-style references described in words | Credentials, API keys, internal hostnames |
