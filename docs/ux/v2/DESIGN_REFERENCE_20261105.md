# NexusAI interface: the design reference (v2, 2026-11-05)

**Status:** working reference for the whole interface recreation, written before any further build · **Owner of the decisions:** Abdul Basit · **Executor:** Claude
**Binding companions (not replaced):** [`MASTER_UI_UX_DIRECTIVE.md`](../MASTER_UI_UX_DIRECTIVE.md) (rules of truth, accessibility, motion, states, content, performance), [`NEXUSAI_PRODUCT_UX.md`](../NEXUSAI_PRODUCT_UX.md) (the contract: provenance, screen states). **This document adds:** the owner's instructions in one place, the layout system (grid, responsiveness, spacing, sizing, density, layering), the page archetypes, the specification of every dashboard section, the per-section research-then-build loop, and the front-end architecture. Where this document and the directive differ, this one is newer and the difference is listed in section 11.

How to read the evidence marks: **[V]** a primary or official source read in full, **[S]** a secondary source (design blog, vendor documentation, community guide) read as search results or a page summary, **[K]** established practice stated without a single source, **[D]** a decision of this document. Most of the research reached here is [S]: several searches did not return the design systems' own pages (Carbon spacing tokens, Polaris, Atlassian, Primer, Material 3 breakpoints). Those are listed in section 12 as things to verify against the primary pages when their section is built.

---

## 1. What the owner asked for, in the owner's words, and what it requires

Every line below is from this chat (2026-11-03 to 2026-11-05). The right column is the requirement it becomes. A section or page is not finished until each line that applies to it is met.

| # | The owner said | Requirement |
|---|---|---|
| R1 | "recreate and redesign entire application, every page, every section, every area ... best of the best, modern, up to date, elegant, clean, sophisticated, professional, enterprise grade, accurate and correct" | Nothing from the earlier interface is kept for its own sake. Each page is rebuilt on the system of sections 3 to 7. |
| R2 | "for each section or area conduct research ... then implement ... then research for the next and implement ... one by one" | The loop of section 9 is binding. Research is **per section, immediately before building it**, not in one batch. |
| R3 | "start from the dashboard, first section first area, recreate its best, then the next section ... then the next page" | Order of work: dashboard (section by section), then Cases, Evidence (list, file), Ask and its evidence panel, Case overview, Connections, Timeline, Activity, Data, Settings. |
| R4 | "best visualisations, graphs, charts, figures"; "modern, up-to-date, **distinct and appropriate in various sections**" | No two neighbouring sections use the same visual form without a reason. Section 5.3 assigns a form to each section and says why. |
| R5 | "it is not a must that existing sections are recreated; you can add best new things": stats, numbers, productive information, **usage** from the application's data, DB, user preferences | New sections are allowed and encouraged when real data supports them (section 5.4 lists what is available today and what needs the backend). |
| R6 | "also adjust the colour of the main area where pages appear with respect to the sidebar in all themes, and the header rail" | The frame (sidebar and top bar) and the work area are told apart by tone in light, dark and high contrast (section 6.2). |
| R7 | "don't add so much text" | Text budget per section (section 5.2): title of at most three words, a note of at most one short line, no explanatory paragraphs. The rest is in the table twin and in titles on hover **and** on focus. |
| R8 | "best layout of all sections: sizing, positioning, relevant to each other"; "responsiveness, spacing, sizing" | Section 4 (grid, spacing, sizing, breakpoints, density) and the wireframes of section 5.5 at four widths. |
| R9 | "see the most recent and compiled UI/UX docs, which were also provided to Codex" | The master directive binds. Section 11 lists where this reference is newer. |
| R10 | "add best things from the existing backend; if something should be added for usage, productivity, functionality, add it in the UI and **write the backend implementation to be done in future**" | A section may be built over existing data. Anything it wants and the service cannot give is a row in [`docs/work/BACKEND_REQUESTS.md`](../../work/BACKEND_REQUESTS.md) and a designed "not available yet" state in the UI, never invented numbers (directive rule 1). |
| R11 | "I dislike the **Needs you** section: design, position" | Redesigned (section 5.3, S2) and moved to the position the research supports (section 5.5). |
| R12 | "Ask across data types and families must answer, not take a long time"; "the section on the right of an answer (evidence) does not fit" | Fixed in the engine path (cold model: see section 10) and in the evidence panel (rebuilt, `Proof.jsx`); both are acceptance checks of the Ask page. |
| R13 | "run engine round 9e, case backend, speech lab one at a time ... UI work here will continue for long" | Background sessions run one at a time; this work does not hold the heavy lock. |
| R14 | "real, accurate, correct" | Directive rules 1 and 2: every figure is a count a service reported, a share of two such counts, or a median of dated jobs; a case that could not report is named, never zero; a capped read says it is capped. |
| R15 | "firstly create a new design reference ... then start again from the dashboard" | This document; the dashboard is rebuilt from it (section 5). |
| R16 | 2026-11-05: "first the KPIs and indicators ... diverse, many modern, up-to-date charts, figures, visualisations, **not few as in the doc** ... Needs you has too much text ... clean, elegant, easy to use, less text, proper layout, sizing, positioning, spacing ... a hero or single viewport with several visualisations, more below ... interactive, the best animations, each section one at a time with its own research" | **Supersedes the order of 5.1 and the "five figures" row of 5.3.** The first thing on the page is a band of **six indicators**, each a different form (stage S2a). A single 1440 × 900 viewport shows the band, a short Needs you list and the timeline; the remaining sections follow below and each is rebuilt in its turn with the research done first. The count of visual forms on the dashboard grows beyond the table of 5.3; each new one needs real data and a table twin. Needs you is now a short list (one line per cause: count, label, arrow), not a tiered queue; the things themselves are in the line's title. |

---

## 2. Principles (short, testable)

1. **The page's job comes first** [S: SaaSFrame home-dashboard guidance]. A page is for *resuming work*, *monitoring*, or *investigating*; one job leads, the others follow. The dashboard's job: **"Where do I stand, what needs me, where do I pick up?"** in five seconds.
2. **Reading order follows the eye.** Eye-tracking on dashboards found the upper-left to lower-right path, and big numbers drew the eye before anything else; the item that matters most is not put on the right or at the bottom [S: Tableau, eye-tracking study]. Hence: the queue of what needs you is **top-left**, the figures **top-right of it**, and a viewer can name the state of the workspace after a five-second look [D].
3. **Sort by decision, not by time.** A list of things to do is sorted by what blocks the work, and a stated rule says so; every item has one primary action [S: Cockpit UX principles, Slack triage, saasui inbox patterns].
4. **Situational awareness in three steps** [S: command-centre guides]: *perception* (the figures and the state pills: what is going on), *comprehension* (the breakdowns: why), *projection* (the queue and the next action: what happens if I do nothing / what to do next).
5. **Progressive disclosure.** Overview first, the table twin and the page behind each figure on demand [S: forensic dashboard case studies; Magnet AXIOM's "places to start" idea of a starting point that decides where to look first].
6. **One evidence rule.** A number that can be opened is a link; a number that cannot is plain text; a number with no source is not drawn (directive rule 2).
7. **Calm instrument.** Quiet chrome, loud evidence; one accent for what can be acted on; hierarchy by type and space, not by borders and shadows; no looping motion (directive 4.1, 8).
8. **No card soup.** Regions and rows, not a bordered card around every sentence. A panel exists to group a *question and its answer*; inside it there are rows, not more cards.

---

## 3. The page archetypes

Every page is one of five shapes. A new page picks a shape; it does not invent a layout.

| Archetype | Pages | Regions (desktop ≥1280) | Notes |
|---|---|---|---|
| **A. Overview** | Dashboard, Case overview | header band, a focus row (what needs me + the figures), then bands of equal-height panels on a 12-column grid | Panels answer questions; the table twin is under each chart. |
| **B. Collection** | Cases, Evidence list, Activity | page title row, filter bar (chips, applied filters visible), one table filling the width, footer with count and paging | One table; sticky header; the first column pinned; density control. |
| **C. Workspace** | Ask, Connections | optional left list (history), the working column, a supporting pane on the right opened on demand | The supporting pane has its own grid column (never relies on order in the DOM) and a "wider" control. |
| **D. Viewer** | Evidence file, document, image, audio, video | header (name, state, actions), the artefact filling the remaining height, details as a pane | The artefact scrolls inside one frame; the page does not. |
| **E. Settings** | Settings, New case | a narrow column of labelled groups | Forms only. |

---

## 4. The layout system

### 4.1 Grid, margins, gutters [S: Carbon 2x grid v10; K: 12-column practice; D]

- **12 columns**, fluid. Gutter **16 px** (24 px from 1536). Outer margin **24 px** (16 px under 768). Max content width **1440**; prose at most **72 ch**.
- The grid is built on a **4 px base with an 8 px mini-unit** (Carbon's 2x grid is built on the 8 px mini unit [S]; Cloudscape's tokens use a 4 px base [S]). Every dimension is a multiple of 4; most are multiples of 8.
- The grid is **fluid between breakpoints and changes only at them**; a sidebar shrinks the content area and the *content width* (not the window width) decides the column behaviour. Carbon's own kit notes that when a sidebar makes the content narrower than a breakpoint the next breakpoint's settings apply [S]; this reference does the same with container queries (4.3).

### 4.2 Spacing scale [K/D]

`4 · 8 · 12 · 16 · 20 · 24 · 32 · 48 · 64` (tokens `--nx-s-1 … --nx-s-16`). Rules: gap between sibling panels **16**; inside a panel: padding **16** (12 compact, 20 comfortable: `--nx-pad-panel`); between a panel title and its body **8**; between rows of a list **2 to 4** with a **44 px** minimum row height only where a row is an action target; between a figure and its small picture **8**.

### 4.3 Responsiveness [S: Material 3 window classes and canonical layouts; K: container queries]

- **Window classes** (the viewport), after Material 3's compact / medium / expanded [S] and the directive's breakpoints (480 · 768 · 1024 · 1280 · 1536): **Compact < 768** (one column, rail becomes a drawer), **Medium 768 to 1279** (one main column; a supporting pane becomes a drawer or stacks), **Expanded ≥ 1280** (main plus supporting pane). More than two panes are used only from 1536 [S: SAP guidance mirroring Material].
- **Container classes** (the width a *panel* is given, not the window): **S < 560 · M 560 to 859 · L 860 to 1179 · XL ≥ 1180**. Media queries decide page layout; **container queries decide what a panel shows** (columns of a table, how many bars, whether a legend sits beside or under) [S: container-query guides; the guidance to use both and not replace media queries].
- A layout is **never stretched from mobile to desktop**; each class says how many panes, what moves, what hides [S: Android adaptive codelab].
- No horizontal scroll of the page at 320 px; tables scroll inside their own frame with the **first column pinned** (directive 4.6; Visa/Pajamas table practice [S]).

### 4.4 Sizing and targets

- Control heights **24 / 28 / 32 / 36** (compact / default / comfortable; `--nx-ctl-*`). **Default 32.** A pointer target is **at least 24 × 24 CSS px** or has enough space around it (WCAG 2.2 SC 2.5.8, AA) [V: W3C Understanding]; the stricter **44 × 44** (SC 2.5.5, AAA) applies to the primary action of a row and to everything on a touch screen (< 768) [V/S].
- Table rows **32 / 40 / 48** (density attribute `data-density`); **compact is the default for tables** (owner, 2026-10-12). Compact never replaces comfortable; the analyst can always switch (Cloudscape: compact can hinder readability, so it sits beside comfortable) [S].
- A panel has a **minimum useful width of 280 px**; below that it stacks.

### 4.5 Type scale [S: enterprise type guidance; directive 4.4]

`12 caption, axis, notes · 13 table cell, label, list row · 14 body · 16 lead · 20 section title · 28 to 32 page title · 32 figure`. Ratio about 1.2 (minor third) above 14 [S]. Line height **1.2 to 1.4** for rows and labels, **1.5** for prose [S]. Figures that stand alone are **proportional**; numbers in columns, axes and tooltips are **tabular** (directive 7.7, dataviz rules). Identifiers (IMEI, plate, hash) monospace, `dir="ltr"`.

### 4.6 Density

Three modes through `data-density` on `<html>` (directive 4.6); panel padding, row height, control height and tile padding all follow it (`--nx-pad-panel`, `--nx-row-h`, `--nx-ctl-*`). No component hard-codes a padding.

---

## 5. The dashboard, from scratch

### 5.1 Job and reading order

**Job:** "Where do I stand across my cases, what needs me, and where do I pick up?" in five seconds. **Reading order (F-pattern):** state of the workspace (header) → what needs me (top-left) → the figures (top-right of it) → the cases (the working list) → the evidence drawn (timeline, answers, quality, mix, records, speed). **Test:** a person who has seen the page for five seconds can say how many cases need review and what to open first.

### 5.2 Text budget (owner R7)

Panel title ≤ 3 words. The note beside a title ≤ 6 words and says what is drawn and its unit ("Events per month"). One note line under a chart ≤ 12 words. No paragraph on the dashboard. Explanations live in the tooltip *and* the focus description, and in the table twin's caption.

### 5.3 Sections: the question each answers, its form, its size

Form follows the question (directive 7.2) and **no two neighbours share a form**.

| # | Section | Question | Data today (real) | Form (and why) | Spans XL / L / M / S |
|---|---|---|---|---|---|
| 0 | **Header band** | What is the state? | case status service; case records | title, three state pills, 2 actions | 12 / 12 / 12 / 12 |
| 1 | **Needs you** | What blocks my work, in what order? | failed and dead jobs, missing copies, stalled processing (`/collections/status`), notes (`/analysis/typing-report`), unsent drafts and "needs your choice" (browser) | **Action queue**, three tiers (needs me now / can wait / clear right away), one primary action per row, the sort rule printed [S: queue and triage guidance] | **5 / 12 / 12 / 12** (top-left) |
| 2 | **Indicators** (R16: first on the page) | How big is it, how healthy, what waits? | status summary; case records; engine health (7 per-day reads); `recent_jobs` timestamps; attention groups | **Six indicators, six forms:** Cases (state bar), Files ready (segmented meter), Records (columns by case, one scale), Answered · 7 days (stacked columns by day), Needs you (a dot for each thing, coloured by cause: a unit chart), Time to ready (histogram on fixed bins with the median bin marked). The figure counts up once (420 ms), the bars grow once (460 ms, staggered), neither under reduced motion (system setting, page, or the viewer's stored choice); hovering a part dims the others; the whole tile opens the list behind it. No target exists, so no bullet bar and no good/bad arrow. | **12 / 12 / 12 / 12** (6 columns at ≥ 1020 px of band width, 3 at ≥ 560, 2 below) |
| 3 | **Cases** | Which case, and what next? | status; case records | **Table**, key column pinned, status badge, files-ready meter, **in-cell volume bar** (cases on one scale), one primary action | 12 / 12 / 12 / 12 |
| 4 | **Evidence timeline** | When did things happen, per case? | `activity_by_day` (capped at 100 rows per case: labelled) | **Small multiples**, one row per case, **own scale / shared scale toggle** [S: Cube, ApexCharts: shared compares size, own shows shape] | 8 / 12 / 12 / 12 |
| 5 | **Pick up + Latest** | Where did I stop, what just changed? | browser question history; recent evidence | **List** (the last question of up to three cases; the six latest evidence changes, each leading to its file) [S: resume-first, label the selection rule] | 4 / 12 / 12 / 12 |
| 6 | **Answers** | How are my questions answered? | `/analysis/engine-health` (7 days, and the 7 windows for a per-day split) | **Stacked columns per day** plus a **time ladder** (median, nine in ten, slowest); every share shown with its count n [S: LLM-observability and groundedness guidance] | 4 / 6 / 12 / 12 |
| 7 | **Data quality** | Can I trust the data? | status rows read/duplicate/rejected; typing report layouts' time basis | **Dumbbell** (rejected vs rejected + repeats, per case, one axis from zero) and **one stacked bar** (how the times are known) [S: dumbbell for exactly two values; Microsoft data-quality metrics] | 4 / 6 / 12 / 12 |
| 8 | **Evidence mix** | What do I hold? | typing report `evidence` by modality | **Stacked bar plus legend** with counts and shares (≤ 6 hues + "Other"; colour follows the kind, never its rank) | 4 / 6 / 12 / 12 |
| 9 | **Records** | Which kinds, in which case? | typing report calls/sessions/usage; accepted rows | **Heat matrix**: case × kind, the number in every cell, **log-scale shading, blank means none** [S: heat-matrix guidance: log or quantile buckets, state the scale, empty ≠ zero] | 6 / 12 / 12 / 12 |
| 10 | **Pipeline speed** | How long do files take, and where does the time go? | `recent_jobs` timestamps | **Range strips** on one axis (fastest to slowest, middle half, median) with a median-of-all reference, and the **wait vs processing split** [S: Kipu queue definitions: waiting = total − processing] | 6 / 12 / 12 / 12 |

### 5.4 What can be added from the existing backend, and what is future

**Available now (built or buildable over existing reads):** processing stalled longer than 30 minutes, jobs that gave up (dead letter), files read as plain tables (warnings), the basis of the times (analyst-confirmed, stated in the file, declared), items indexed for search, questions per 24-hour period (seven engine-health reads), case state (open / on hold / closed), time from upload to ready split into waiting and processing.

**Future (each is a row in `docs/work/BACKEND_REQUESTS.md`; the UI already shows the designed state or the capped read):**
full-range activity per case (existing row 19; today capped at 100 rows), engine health by day in one call, a workspace summary and attention feed (existing rows 12, 13), a per-user usage log and preferences on the server, processing history with durations beyond the latest 20 jobs, entities indexed and bytes stored per case, evidence sites for a map, a weekday-by-hour rhythm matrix (existing row 19), shared identifiers across cases (counts only). Section 12 lists the new rows.

### 5.5 Wireframes (the layout, at four widths)

**Expanded ≥ 1280 (content 1176 at 1440).** Left-to-right, top-to-bottom is the reading order; heights in a band are equal.

```
┌ header band ─────────────────────────────────────────────────────────────────┐
│ Your cases  ● 2 need review ● 1 processing ● 3 ready    [Refresh] [Ask across cases] [New case] │
├ A: focus band ───────────────────────────────────────────────────────────────┤
│ ┌ 1 NEEDS YOU (5/12) ───────┐ ┌ 2 FIGURES (7/12) ─────────────────────────┐   │
│ │ needs me now        (5)   │ │ Cases      Files ready   Records           │   │
│ │  ● Did not finish    2    │ │ 6          147 / 150     91,869            │   │
│ │    file …      [Review →] │ │ ▇▇▇        ▇▇▇▇▇▇▁       ▇▇▇▇▇▇▇▁          │   │
│ │  ● Still processing  1    │ │ Answered·7d  Needs review  Median to ready │   │
│ │ can wait                  │ │ 95.7% ╱╲╱╲   2            32 s             │   │
│ │ clear right away          │ └────────────────────────────────────────────┘   │
│ │ ordered by what blocks you│                                                  │
│ └───────────────────────────┘                                                  │
├ B ───────────────────────────────────────────────────────────────────────────┤
│ 3 CASES (12/12)  name · status · files ready ▇▇▁ · records ▇▇ · updated · [action] │
├ C ───────────────────────────────────────────────────────────────────────────┤
│ ┌ 4 EVIDENCE TIMELINE (8/12) ──────────────┐ ┌ 5 PICK UP + LATEST (4/12) ──┐   │
│ │ rows of monthly bars  [own | shared]     │ │ last question per case      │   │
│ └──────────────────────────────────────────┘ │ latest evidence (6)         │   │
├ D ───────────────────────────────────────────────────────────────────────────┤
│ ┌ 6 ANSWERS (4) ───┐ ┌ 7 DATA QUALITY (4) ┐ ┌ 8 EVIDENCE MIX (4) ─────────┐    │
├ E ───────────────────────────────────────────────────────────────────────────┤
│ ┌ 9 RECORDS: heat matrix (6) ──────────────┐ ┌ 10 PIPELINE SPEED (6) ─────┐    │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Medium 768 to 1279.** One main column. Band A: **Needs you** full width (compact: counts by cause, top 4 rows), then the **figures** as 3 + 2 tiles; then cases; then timeline (full width); then pick-up/latest as two columns; then answers, data quality, mix, records, speed in 6/6 pairs.

**Compact < 768.** One column in this order: header (pills wrap), needs you (top 3 rows, "See all"), figures (2 columns, the fifth full width), cases as cards (name, status, one action), then the charts one under another; tables become cards or scroll in their own frame with the first column pinned; every target at least 44 px; no horizontal page scroll at 320.

**Why not the right rail I built first (option B).** It put the queue beside a narrow main column (about 780 px), so the cases table lost columns and the figures lost their captions; it also read as a sidebar, not as the first thing. It stays as a documented alternative: if the owner prefers it, only the grid of the page changes, because every section is a self-contained panel.

### 5.6 Section states (every section has all five)

Loading: a skeleton of the final shape, shown only while reading, never longer than the read. Empty: one line and the next action ("No questions in the last 7 days"). Error: one page-level message with Retry when nothing can be read; a partial failure names the case. Capped: says so in the row ("first 100 days only"). Refusal does not occur on the dashboard.

---

## 6. Visual system: the parts this reference fixes

### 6.1 Charts (directive 7 applies; additions from research)

- **Small multiples:** one row per case; the same size; an **own-scale / shared-scale** control, because shared answers "how big" and own answers "what shape" [S: Cube, ApexCharts]; a cap on rows with "N more in the table".
- **Heat matrix:** at most five or six shades, on a **log scale for counts**, the **number always in the cell**, empty ≠ zero, a real table with row and column headers, shades checked in greyscale [S: heatmap guidance].
- **Dumbbell:** exactly two values per category, one axis from zero, the numbers beside the plot; with more than two values or different units use bars [S: Domo, Highcharts, amCharts].
- **Distribution of a duration:** range strips (min to max, middle half, median) on one shared axis with a reference line; the wait and the processing time are told apart [S: Kipu queue definitions, WorkFusion cycle-time box plots].
- **Answers (model quality):** every share shown with its count, a per-period trend, latency as percentiles, refusals as a share; no single "trust score" is invented (nothing in the sources defines one) [S: groundedness and observability guidance].
- **Always:** a table twin with Copy as CSV, a text summary, 3:1 contrast for every mark (WCAG 1.4.11) [V/S], no colour-only encoding (a legend with values, and shapes where the strength matters), chart colours validated for colour-blind separation (the six data colours were re-derived on 2026-11-04 and are re-checked on every build).
- **Metric cards:** label, one figure, one context line; a sparkline only over at least seven real points, marked decorative with the numbers as text; no colour judgement on direction ("up is not good" for volume) [S: saasui KPI card guidance, shadcn/Boldbi notes].

### 6.2 Layering and the frame

Four tones, always in this order from outermost to the work: **chrome** (sidebar and top bar, one family, the darkest in light mode and the deepest in dark), **canvas** (the page behind the panels), **surface** (panels, tables), **raised** (menus, drawers). Dark mode lifts surfaces by lightness, not by shadow; no pure black; text not pure white; shadows only for overlays [S: dark-mode design-system guides]. The chrome and the canvas never share a tone in any theme (owner R6).

---

## 7. Other pages: shape and first decisions (each gets its own research when its turn comes)

| Page | Archetype | First decisions (to be confirmed by that page's research) |
|---|---|---|
| Cases | B | filter chips with counts, sort, cards/table switch (built); add last-activity column and per-case figures with the dashboard's visual forms |
| Evidence list | B | one dense table first; add evidence in a collapsed region; type chips; row opens the file |
| Evidence file (rows) | D | the table fills the height; the cited row is one card above it; details as a pane over the table, not beside it |
| Ask | C | the evidence panel has its own grid column, a "wider" control, lines as selectable rows, records as a table with a link to the file at the row (built) |
| Case overview | A | the dashboard's panels, scoped to one case |
| Connections, Timeline | C | coordinated views with a working set (directive 6.6, 6.7) |
| Activity | B | feed with filters; reviewed state |
| Data | B/A | typing report: layouts, time basis, repeats |
| Settings | E | grouped forms; storage inventory; appearance |

---

## 8. Front-end architecture

- **Layers:** `nx-reset, nx-tokens, nx-base, nx-components, nx-layout, nx-patterns, nx-pages, nx-utilities`; legacy CSS demoted into `legacy`; element rules apply only inside `.nx-ui`.
- **Tokens:** generated (`design/tokens-v2.mjs`) from OKLCH; every pair used together is checked on build; the six data colours are checked for colour-blind separation on build.
- **Layout primitives:** `.nx-page`, `.nx-grid` (12 columns, `nx-span-*`), `container-type: inline-size` on a band, never on a panel's own contents unless the panel changes its parts.
- **Data:** one hook per read (status, case records, engine health, typing report, activity), results cached for minutes, heavy reads lazy and few at a time; **pure selectors** in `model.js` (tested) turn reads into what is drawn; components receive the result only.
- **Micro charts** are plain elements and CSS variables (`SegBar`, `BarStrip`, `RangeStrip`, `ColumnChart`, `Dumbbell`, `Sparkline`), logical properties only, so right-to-left mirrors without script; `TableTwin` pairs every chart with its table.
- **Page = composition of panels:** a page file only arranges panels and passes data; a panel file owns one question.
- **Performance:** first paint on the cheap reads; each panel shows its skeleton and fills as its read arrives; no chart blocks the main thread over 50 ms.
- **Tests:** a selector test for every number's rule; a page test per state (loading, empty, partial, error, first run); the evidence of both themes at 1440 and 390.

---

## 9. The loop, per section (binding; owner R2)

1. **Research that section only** (two to four targeted searches, one primary source if reachable). Write three lines in the stage note: what the sources say, what was decided, what is unsupported.
2. **Specify:** the question, the data and its endpoint, the form and why, the spans at four widths, the five states, the text budget, the backend gap.
3. **Build it** over the existing hooks; add the selector, the panel, the table twin, the tests.
4. **Verify:** screenshots at 1440, 1280, 1024 and 390, light and dark (and contrast); keyboard path; the table twin opens; the numbers equal the service's (spot-check against the API); the five-second test for the page.
5. **Commit** the section (explicit paths, `Assisted-by` and `Claude-Session` trailers, no `Co-Authored-By`).
6. **Report in four lines** and go to the next section. Nothing else is built until then.

**Definition of done for a section:** meets the owner lines R1 to R15 that apply; every number is real or absent; all five states; the twin; contrast and keyboard; both themes at four widths; tests green; the research lines written.

---

## 10. Product checks that sit beside the interface (from the 2026-11-04 demo preparation)

- **Ask latency:** warm, a question takes 25 to 65 seconds; a model that has been idle takes about a minute longer to reload, and with the machine busy the first question can fail. Before a demonstration: send one tiny completion to the model, then one real question (`docs/ux/v2/demo-warmup.ps1`). Background work (Go builds, mutation runs, rehearsals) is not run during a demonstration.
- **The evidence panel** did not fit because the three-column grid placed the conversation in the history's column when the history folded away; each panel now names its own column.

---

## 11. Where this reference is newer than the master directive

| Directive | This reference |
|---|---|
| 6.1: Attention queue **before** the metric row, full width | Attention queue **top-left** beside the figures (reading order, the queue is the first thing and the figures are not hidden) |
| 6.1: metric row ≤ 4 cards | Five figures, because records and the answers rate are both asked for by the owner; each stays one link to its list |
| 6.1: hero sentence (computed) | State pills (shorter; the owner asked for less text); the sentence is the pills' accessible name |
| 7.2: part-to-whole is a stacked bar | Kept; the heat matrix is added for case × kind; the dumbbell for exactly two values |
| 4.6: breakpoints 480 / 768 / 1024 / 1280 / 1536 | Kept; container classes S / M / L / XL are added for panels |
| 5: "no more than three panes" | Two panes below 1536 |

---

## 12. Research notes, sources and what is still unverified

**Read (marked [S] unless said):** Tableau, how design thinking will affect today's analysts (dashboard eye-tracking): <https://www.tableau.com/blog/how-design-thinking-will-affect-todays-analysts-93507> · SaaSFrame dashboard patterns (resume first, label the selection rule): <https://www.saasframe.io/patterns/dashboard> · ServiceTitan Anvil home screens: <https://anvil.servicetitan.com/docs/blocks/home-screens> · Magnet AXIOM case dashboard (starting points): <https://magnetforensics.com/blog/better-case-starting-points-with-magnet-axioms-new-case-dashboard> · Cockpit UX principles (every blocked item needs a primary action): <https://evref-bl.github.io/DevNexus/user/cockpit/ux-principles> · Slack triage (what needs you, what can wait, what you can clear): <https://slack.com/intl/en-gb/blog/news/slack-activity-triage-for-notifications> · saasui KPI card patterns: <https://www.saasui.design/blog/saas-metric-kpi-card-ux-patterns> · Visa table usage: <https://design.visa.com/components/table/usage/> · Cube small multiples: <https://docs.cube.dev/docs/explore-analyze/charts/configuration/small-multiples> · ApexCharts trellis: <https://apexcharts.com/docs/trellis/> · Domo dumbbell chart: <https://domo-webflow.domo.com/learn/charts/dumbbell-plot-chart> · Microsoft ingestion data-quality metrics: <https://learn.microsoft.com/en-ie/azure/operator-insights/concept-data-quality-monitoring> · Kipu queue definitions: <https://kipuhealth.zendesk.com/hc/en-us/articles/4402412009869> · deepset groundedness: <https://www.deepset.ai/blog/rag-llm-evaluation-groundedness> · OneUptime percentiles: <https://oneuptime.com/blog/post/2025-09-15-p50-vs-p95-vs-p99-latency-percentiles/view> · Chartability (Elavsky, Bennett, Moritz 2022): <https://diglib.eg.org/handle/10.1111/cgf14522> · Android adaptive layout codelab (window size classes): <https://developer.android.com/codelabs/adaptive-material-guidance> · SAP Fiori canonical layouts (panes by size class): <https://www.sap.com/design-system/fiori-design-android/v25-8/foundations/adaptive-layout/canonical-layouts> · Carbon 2x grid (v10): <https://v10.carbondesignsystem.com/guidelines/2x-grid/implementation> · W3C WCAG 2.2 target size (minimum): **[V]** <https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html> · Cloudscape content density: <https://cloudscape.design/foundation/visual-foundation/content-density/> · Dynatrace data table usage: <https://developer.dynatrace.com/design/components/tables/DataTable/usage/> · Lollypop enterprise SaaS typography: <https://lollypop.design/blog/2026/july/enterprise-saas-typography-rules/> · Guidewire Jutro typography: <https://docs.guidewire.com/jutro/documentation/10.10/typography> · command-centre design guide: <https://www.activu.com/the-command-center-design-guide-building-for-operational-intelligence-in-2026/> · container-query guide: <https://devtoolbox.dedyn.io/blog/css-container-queries-guide>.

**Not found, to verify on the primary pages when the section is built:** the exact Material 3 breakpoint values; Carbon's spacing tokens and the current 2x grid margins; Polaris and Atlassian dashboard and table guidance; Primer layout; the browser-support figure for container queries (check MDN and caniuse); any source that defines a single trust score for an AI answer (none was found, so none is drawn); any source on right-rail placement of an attention queue (none was found; the placement here follows the reading-order evidence, not a rail precedent); guidance specific to Cellebrite, Magnet, Nuix or Relativity dashboards beyond Magnet's case dashboard.

**Backend rows added with this reference** (in `docs/work/BACKEND_REQUESTS.md`, rows 27 to 33): engine health by day; workspace data-basis summary; processing history; per-user usage and preferences; attention feed extensions; evidence sites for a map; cross-case overlaps (counts only).

**Open decisions for the owner:** the dashboard layout (option A in 5.5, recommended; option B, the right rail, kept as an alternative); whether a map of evidence sites belongs on the dashboard (needs the offline tile service running and a places summary from the service); whether the five-figure row stays at five.
