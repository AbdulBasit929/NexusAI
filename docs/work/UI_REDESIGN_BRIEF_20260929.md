# NexusAI workspace: full UI/UX redesign brief (v2)

2026-09-29. Written for a Claude Code session that takes over the UI track while Codex is paused.
It **supersedes** `docs/work/CODEX_UI_REDESIGN_PROMPT_20260929.md` where they conflict, and keeps that
brief's truth, safety and verification rules.

## 1. The owner's verdict on the current state

- Only part of the Dashboard has been redesigned. **Every section of every page** must be redesigned,
  or rebuilt where that is better, to a professional, enterprise-grade standard. Add, change or remove
  sections wherever that makes a page more useful.
- **Colour:** the new header gradient (navy `#111827` to deep teal `#123b43`) looks right. The sidebar
  (`--analyst-navigation-surface: #f3f8fd`) and the main canvas (`--analyst-surface-0: #eaf1f8`) don't
  belong to the same palette. Make the whole shell one coherent identity derived from the header.
- **Charts:** the current donut and pie-style figures don't explain the use case. Every visual must be
  correct, logical and useful for an investigator, and consistent with the charts around it. Use
  different chart types where different questions call for them; don't repeat one shape everywhere.
- **Interaction:** every interactive element must be accurate and productive. Clicking something
  filters, drills down or navigates to the right, real place.
- **Research depth:** research how each page as a whole can be improved, then go deeper, section by
  section, and design from that research.

## 2. Ownership and coordination

- You own `apps/investigation-workspace/**` for this work. Codex built the current version, and its
  limit resets tomorrow. **Don't revert Codex's work blindly**: read it, keep what's good, and replace
  what isn't. Before editing a file, check `git status` and `git diff` for it. The owner decides who
  owns the UI if Codex resumes, so two agents never edit the same files at once.
- The backend is another Claude session: `api/**`, `semantic_layer/**`, the database and Docker. Never
  edit those. When a page needs data or an endpoint that doesn't exist, add a precise request to
  `docs/work/BACKEND_REQUESTS.md` (§7) and design the honest state until it lands.
- Record progress in the `## CODEX_UI_TRACK` section of `NEXUSAI_CONTINUATION.md` (the UI section).
  Don't edit anything above it. The file is capped at 300 lines, so keep entries short and rewrite
  rather than append.

## 3. Read first

1. `AGENTS.md`, then the `## CODEX_UI_TRACK` section of `NEXUSAI_CONTINUATION.md`.
2. `docs/work/CODEX_UI_REDESIGN_PROMPT_20260929.md`: the earlier brief. Its §0 rules, §3 data, §5
   accessibility, §6 verification and §9 never-list still apply.
3. `docs/work/UI_REDESIGN_RESEARCH_20260929.md`: Codex's research. Build on it and extend it.
4. `docs/ux/NEXUSAI_PRODUCT_UX.md`, `docs/work/CODEX_UI_ELEVATION_BRIEF_20260928.md` (§4, honest states)
   and `docs/work/CODEX_UI_DEFECTS_20260928.md`.
5. `docs/work/BACKEND_REQUESTS.md`: what's delivered and what's open.
6. `docs/work/RUNTIME_QUERY_PLAN_20260929.md`: what the backend is building next. Investigate across
   "my cases" is first, BACKEND_REQUESTS row 11.
7. The code: `src/styles/tokens.css`, `src/styles/workspace.css`, `src/components/CaseShell.jsx`,
   `src/components/DataVisualizations.jsx`, `src/pages/*`.

## 4. The rules that never change

These exist because this is forensic evidence that may be read in court.

1. **Real data only.** Every number, chart, count and label comes from the API. No sample data,
   placeholders, "demo" series or estimates in a shipped view. Empty or partial data gets a designed,
   honest empty or partial state.
2. **Privacy is server-side.** Never show plate text, identity numbers or transcript text unless the
   API returned it for that request. Never try to unmask anything.
3. **WCAG 2.2 AA in both themes:** text at least 4.5:1, UI parts and focus at least 3:1, measured
   against the real background, gradients included.
4. **No engine jargon** for analysts: no model, template, SQL, operation or token names.
5. **Every visible control works.** Nothing is disabled "for later".
6. **Honest states.** A refusal or clarification is never styled as a finding, and a withheld answer
   never shows a number. A model observation, such as OCR, a transcript or a plate read, is badged as
   one, using `provenance[].source_truth_state`.

## 5. Method: page, then section

For each page, in the order in §9:

1. **Audit.** Screenshot the current page at 1440 and 375 px in both themes. List every section, the
   investigator's job it serves, the question it answers, and the exact API field behind it. Mark
   each one keep, improve, replace or remove, and list sections the page is missing.
2. **Research the page.** Take 2–3 reference products for this kind of page and note what they do
   better.
   - Enterprise UI: Linear, Vercel, Stripe, Datadog, Grafana, Retool.
   - Forensic and analytics: Magnet AXIOM, Cellebrite, Palantir, Splunk, Kibana, Maltego.
   - Design systems: Carbon (data visualisation and tables), Fluent 2, Atlassian, Material 3,
     Primer, Geist.
3. **Research each section.** For each section: the best pattern for that job, and why.
4. **Write the spec** in `docs/work/UI_PAGE_SPECS/<page>.md`. For each section, record:
   - its purpose and the question it answers
   - its data source (endpoint and field)
   - the visual encoding and why (§6)
   - what clicking does
   - its loading, empty, partial and error states
   - how it behaves at 1440, 1024, 768 and 375 px
5. **Build it** with shared tokens and components.
6. **Verify it live** (§8), then send the owner before and after screenshots. Continue unless they
   object.

## 6. Charts: every visual answers a named question

The current donut and pie charts fail this test. Choose the encoding from the question and the shape
of the data, never for decoration or variety alone.

| The question is about | Use | Don't use |
|---|---|---|
| Comparing categories ("which record family has the most rows?") | Horizontal bar, sorted, values labelled | Pie with more than 4–5 slices |
| Parts of one whole with few parts ("of 55 items, how many are ready, processing or failed?") | 100% stacked bar, or a donut with the total in the centre when there are 2–4 parts | Several donuts side by side for comparison |
| Composition across groups ("accepted, duplicate and rejected rows per family") | Stacked or 100% stacked bars, one per group | Pie |
| Change over time ("jobs per day") | Line or area, or bars for counts, with a brush to zoom. Only with real timestamps. | A fake axis, or a trend from a sample |
| Distribution ("processing duration") | Histogram or box plot | Average alone |
| Hierarchy or size by category ("evidence by modality, then type") | Treemap or sunburst, with exact counts | Nested pies |
| Flow or lineage ("source file → version → derived artifacts") | Sankey or a small flow graph | Plain lists only |
| Two dimensions ("calls by hour and weekday") | Heatmap | 3D anything |
| Relationships ("numbers linked to this subscriber") | Network graph from API relationships only | Invented links |
| A single figure against a real target | Bullet chart or gauge, only if the target is real | A gauge without a target |
| One number | A KPI tile with its definition and a link to the rows behind it | A chart |

**Every chart has:**
- a title that is the question it answers
- exact values, as a tooltip or labels plus a table toggle
- a text equivalent for screen readers
- a meaning that doesn't rely on colour alone
- consistent colours for the same category on every page
- click-through to the filtered records, or to a follow-up question the API really answers

When a number or a small table is clearer than a chart, use the number or the table.

**Library:** the owner approves Apache ECharts (`echarts`, Apache-2.0) with a thin React wrapper,
code-split per route. `lucide-react` is already installed. Add nothing else without asking.

## 7. Colour and identity: derive it from the header

Build it as tokens first, and show it on the design-system gallery (`TypeProofPage`) in both themes.
Propose **2–3 coherent options**, each measured for contrast, and let the owner pick from screenshots:

- **A. Continuous dark chrome.** The navy-to-teal family of the header carries into a dark sidebar,
  with a clear active state in the teal accent. The main canvas is a softly tinted cool surface, a
  low-chroma relative of the same hues rather than a generic blue-grey, with raised near-white cards.
- **B. Light rail, navy anchors.** The sidebar is a light surface tinted from the header hue, with navy
  section headers and active states. The canvas is one step darker than the cards.
- **C. Full dark workspace.** A dark theme first, in the header family, with distinct elevation levels.

Also: use one accent for interaction, a separate status set (ready, processing, failed, withheld) and
a separate categorical data palette that is colour-blind safe and consistent across pages. Gradients
and fine texture belong on the chrome, hero bands and empty states, never behind dense tables or
charts.

## 8. Definition of done, per page

- `npm --prefix apps/investigation-workspace run test` is green, with new tests for every count, every
  chart's data mapping and every state.
- The page is checked live through the preview (`.claude/launch.json` configuration
  `investigation-workspace`, port 4181) against the running API. Figures match the API exactly. New
  questions take about 70 s on this laptop, so progress states must be honest and cancellable, and
  must never show a fake percentage.
- Screenshots at 1440 and 375 px in both themes.
- Zero console errors, zero horizontal overflow at 375, and zero duplicate IDs.
- Contrast is measured on the real backgrounds, and the keyboard path works throughout.
- Each route is code-split. The dashboard is interactive within about 2 s once the API has answered.

## 9. Order

1. Identity: the palette options in the gallery. The owner picks.
2. The shell: header, sidebar with a global Investigate entry and live badges, command palette,
   breadcrumbs, mobile drawer.
3. Dashboard, completed across **all** sections.
4. Investigate: the answer-first layout, automatic charts from `enterprise.data_grid` chosen by the
   rules in §6, citations with truth-state badges, and honest withheld and clarify cards.
5. Case overview.
6. Evidence list and evidence detail.
7. Cases and New case.
8. Timeline and Connections: designed honest states until the backend endpoints land.
9. Activity, Settings, and the 404, error and offline pages.

A global **Investigate across my cases** page (BACKEND_REQUESTS row 11) is coming from the backend. Design
it now, with results grouped by case, a scope chip and complete-search wording. Ship it when the API
supports it.

## 10. Never

- Edit `api/**` or `semantic_layer/**`, or change Docker or the database.
- Run git writes: commit, reset, clean, checkout --, restore, stash, merge, rebase, cherry-pick, pull,
  push or any force. Don't bypass hooks.
- Delete untracked files you didn't create.
- Write files with Bash heredocs. Use the editor tools.
- Invent data, sample data, or show PII the API didn't return.

Stop and ask the owner when a design needs data the API lacks (and write the backend request), when
tests fail for a reason you don't understand, or when a choice would break accessibility.
