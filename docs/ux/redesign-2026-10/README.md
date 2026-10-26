> **SUPERSEDED as a working instruction on 2026-10-26 by [`../MASTER_UI_UX_DIRECTIVE.md`](../MASTER_UI_UX_DIRECTIVE.md).** Kept for history; section 17 of the directive says what was kept and what was rejected from this document.

# NexusAI UI redesign (October 2026) — plan

**Goal:** a clean, modern, elegant, enterprise-grade interface that feels natural and human — not "AI" and not "techy" — covering colour, type, theme, shell, every page, every section, charts and motion, built one thing at a time.
**Status:** Stage 0 (brief) done 2026-10-20. Nothing has been designed or changed in the app yet.
**Files:** [`STITCH_PRODUCT_BRIEF.md`](STITCH_PRODUCT_BRIEF.md) (what Stitch needs to know) · this plan.
**Binding contracts:** [`../NEXUSAI_PRODUCT_UX.md`](../NEXUSAI_PRODUCT_UX.md) (information architecture, provenance, states, accessibility). The redesign changes the *look and feel*; it extends this contract and amends §8 (visual system) where it changes — it does not contradict §1–§7 or §9–§13.

---

## 1. Where the app stands today

Observed on the live app (`:4181`), synthetic case only:
- A working React 18 + Vite app, 16 routes, about 9,100 lines of CSS, ECharts (charts), Cytoscape (link chart), MapLibre (offline map), Lucide icons, Noto Sans family.
- The look is a **dark teal "Mineral Signal" theme**: gradient shell, tinted card borders, bento tiles, glow on status. That is the "AI dashboard / techy" feel you want to leave. Structure and logic underneath are solid and tested (vitest + Playwright), and the information architecture follows the UX contract.
- Real capability behind it is **narrower than the UX contract**: no login, no case record, no server-side history or annotations, timeline only by day. Those are tracked in [`docs/work/BACKEND_REQUESTS.md`](../../work/BACKEND_REQUESTS.md) and listed in brief Part E.

**What this means:** the redesign is a **new visual and interaction layer over existing, working data hooks and presentation logic**. We keep `analystPrimaryAnswer`, `analystAskPresentation`, `analystDataPresentation`, the result-shape and connection models and their tests (contract §12), and replace markup and CSS stage by stage.

---

## 2. What Stitch is for here — and what it is not

Researched 2026-10-20 (third-party reviews plus Google's open DESIGN.md spec; Stitch's own docs site would not render for me, so verify limits in the product):

| Stitch does well | Stitch does not do |
|---|---|
| Gemini-based; generates high-fidelity screens from text, and from screenshots/sketches of an existing UI | Produce your production code: output is HTML/Tailwind scaffolding, not React components wired to your data (reviewers say so consistently) |
| Several connected screens at once (reported: up to ~5), conversational refinement | Hold your brand as a hard contract; `DESIGN.md` conditions output, it is not enforced |
| `DESIGN.md`: a portable design-system file (tokens + rationale) that Stitch and coding agents read | Be reliable on dense, data-heavy specifics (tables, graphs, maps) without very explicit prompts; it falls back to generic patterns |
| Exports (reported: HTML/CSS, Tailwind, Figma paste, MCP/SDK to coding agents) | Know what is real in your backend: it will invent features unless told |

**So Stitch is our exploration and decision tool.** It gives us options to react to quickly. The truth of what ships comes from: the brief (what exists), the UX contract (what must hold), and a real-data check in the running app. Every Stitch screen gets ported by hand into the React app against the real data, then tested.

**DESIGN.md (open spec, version "alpha"):** YAML front matter (`name`, `colors`, `typography`, `rounded`, `spacing`, `components`) plus markdown sections in a fixed order: Overview · Colors · Typography · Layout · Elevation & Depth · Shapes · Components · Do's and Don'ts. A CLI lints it (`npx @google/design.md lint DESIGN.md`) including **WCAG contrast checks on component colour pairs**, and exports to Tailwind or W3C design tokens. We will use the linter as a gate and map the tokens onto our existing CSS custom properties.

**Privacy:** Stitch is a cloud service. Only synthetic content goes in (brief Part I).

---

## 3. Method: one thing at a time

Each stage runs the same loop and finishes before the next begins:

1. **Research** — a short, sourced note on best practice for just this thing (no more than a page).
2. **Options** — 2–3 distinct directions from Stitch (or from me as swatches/specimens for foundations), each judged against the contract and the brief.
3. **Decide** — you pick; the choice and the reason are written down here.
4. **Build** — port into the React app over real data, replacing the old markup/CSS for that area only; nothing half-migrated.
5. **Prove** — vitest + Playwright, light and dark, keyboard path, 375 px, contrast check, screenshots of the synthetic case, reviewed by you.
6. **Close** — update this file and the contract; commit.

**Stage gate rule:** a stage is done only when you have seen it running on `:4181` (or the dev port) and accepted it.

---

## 4. Stages

| # | Stage | Decides / delivers | Stitch role | Needs backend? |
|---|---|---|---|---|
| 0 | **Brief** (done) | `STITCH_PRODUCT_BRIEF.md`, this plan | — | no |
| 1 | **Foundations** | Colour palette (neutrals, one accent, status, evidence strength, chart series), light and dark themes, type (Latin UI + tabular figures + Urdu), spacing/radius/elevation, motion, icon style → `DESIGN.md` and the token layer (`tokens.css`) | Generate a "style tile" per option; extract `DESIGN.md` | no |
| 2 | **Shell** | Header strip, navigation rail (expanded/collapsed/mobile), case header and tabs, command palette, global states (offline, error, 404) | Screens | no |
| 3 | **Dashboard** | Headline figures, activity chart, needs-review, cases table, suggested questions | Screens | later: summary, live events |
| 4 | **Cases and Overview** | Cases directory, New case, Case Overview (verdict, readiness, data quality, event horizon, notes) | Screens | later: names, status, members |
| 5 | **Evidence** | Inventory, Add data, status and retry, evidence detail and viewers (document, table, image, audio, video, lineage) | Screens + viewer details | later: image overlays |
| 6 | **Ask (the core)** | Answer anatomy, citations and hover preview, clarification, every state, thread, scope chips, history | Screens, most iteration here | later: server history |
| 7 | **Visualisation system** | Chart theme for ECharts, result charts and tables, link chart styling, map styling, sparklines, readiness ring, calendar strip, motion | Reference visuals; final styling done in code | no |
| 8 | **Connections and Timeline/Places** | Link chart page, chronology workbench, places | Screens | later: events, clusters, annotations |
| 9 | **Activity, Data, Settings** | Case and workspace journals, data inventory, settings | Screens | later: audit |
| 10 | **Polish** | Motion pass, empty/error states sweep, responsive and accessibility audit, density tuning, dark-theme audit | — | no |
| 11 | **Later surfaces** (optional) | Reports, sharing/members, audit view, admin | Screens designed ahead of backend, marked "Planned" | yes |

**Why this order:** foundations first so every later screen inherits one system; shell next because every page lives in it; Ask before the visualisations because it is the product; the visual system is stage 7 so the chart theme is built once, after real answers exist to test it against.

---

## 5. Decisions needed before Stage 1

I will bring research and options for each; your steer on direction saves rounds.

1. **Theme direction.** My lean: **light-first**, a warm-neutral "paper" working surface with deep ink text and a single restrained accent; a designed dark theme as the second theme (deep slate, never black). This moves away from the current dark-teal shell. Dark-first is possible but reads "techy" faster.
2. **Personality.** My lean: *editorial calm* (clear type, generous hierarchy, quiet colour) over *product-SaaS playful*. Reference points to react to: Linear (restraint, hairline structure), Stripe Dashboard (dense, sortable tables), Palantir Blueprint (enterprise density), Cambridge Intelligence tools (investigative graph styling), Kibana/Elastic Security and Binalyze timelines (investigation workflows).
3. **Typeface.** Candidates: **Inter** (safest for dense tables, tabular figures), **Geist**, **IBM Plex Sans** (more "corporate"). Urdu needs a deliberate choice: Noto Nastaliq Urdu (traditional, needs taller line-height, loads separately) versus Noto Sans Arabic (what the app uses now, cleaner but less traditional). Do your analysts expect Nastaliq?
4. **Brand.** Keep the current NexusAI mark and name, or evolve them? (The mark is code-native today.)
5. **Where the work happens.** A parallel session edits the workspace CSS and a Codex UI track owns the `CODEX_UI_TRACK` section of `NEXUSAI_CONTINUATION.md`. I recommend the redesign runs in its own branch/worktree so we do not collide, and the two tracks are told. Your call.
6. **Density.** Comfortable (more air) or compact (more rows) as the default? Investigators lean compact; I would ship a density toggle for tables.

---

## 6. Risks and how this plan handles them

- **Stitch invents features.** Every prompt names what is real and what is planned (brief Part E); planned things are designed but not wired.
- **Look and logic drift apart.** Ported screens use the existing hooks and tests; nothing simulates a missing backend (contract §9).
- **Dual styling during migration.** Each stage replaces one area completely; old and new tokens coexist only behind an alias layer, deleted at Stage 10.
- **Real data leaking to a cloud tool.** Brief Part I; screenshots only from the synthetic case.
- **Accessibility regressions in a prettier UI.** DESIGN.md contrast lint plus the existing accessibility checks at each stage gate.

---

## 7. Sources

- Google DESIGN.md specification and linter: <https://github.com/google-labs-code/design.md>
- Stitch prompting guidance (zoom out, then zoom in; screenshot input; iterate): <https://blog.openreplay.com/prompt-ui-google-stitch/> · Stitch's own guide lives at `stitch.withgoogle.com/docs/learn/prompting/` (did not render for me; read it in the product)
- Stitch capabilities and limits (reviews): <https://stevengonsalvez.com/tools-tips/google-stitch> · <https://www.nxcode.io/resources/news/google-stitch-vs-figma-ai-design-comparison-2026> · <https://designproject.io/blog/google-stitch-review/>
- Link-chart styling guidance: <https://cambridge-intelligence.com/link-visualization-styles/> · <https://developers.cambridge-intelligence.com/docs/regraph/styling>
- Investigation timelines: <https://www.binalyze.com/air-release-notes/binalyze-air-v5.0> (mini-map, zoom, annotate, promote to finding)
- Palantir Blueprint (dense enterprise design system): <https://blog.palantir.com/scaling-product-design-with-blueprint-25492827bb4a>
- Dense dashboard practice (Stripe-style tables, five-second rule, not colour alone): <https://think.design/?p=2597> · <https://www.designrush.com/agency/ui-ux-design/trends/dashboard-design-principles>
- Bilingual type (Inter + Noto Nastaliq Urdu, line-height caveat): <https://open-design.ai/plugins/design-system-urdu/>
