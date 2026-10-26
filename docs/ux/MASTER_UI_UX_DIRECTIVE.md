# NexusAI: the master UI/UX directive

**Status:** the one interface directive of record · **Date:** 2026-10-26 · **Owner of the decisions:** Abdul Basit · **Executor:** Codex (interface), Claude (data, API, evidence)
**Replaces, as the working instruction for every interface stage:** the redesign master guide, the Codex briefs and prompts of September, the October Stitch plan and the dark-directive notes (the ledger in section 17 says, for each, what was kept and what is gone). **Does not replace:** [`NEXUSAI_PRODUCT_UX.md`](NEXUSAI_PRODUCT_UX.md) (the contract: information architecture, provenance, screen states, the clarification surface). This directive **proposes amendments** to sections 8 (visual system) and 11 (responsive) of the contract (listed in 4.9); **they take effect when the owner accepts them (decision O-1)**, and until then the contract binds on those sections too and a stage that needs a difference asks first (master-plan ruling C8). On everything else the contract binds.

> The owner's words, 2026-10-26: *"the best of the best, world class, clean, modern, elegant, up to date, professional, enterprise grade, accurate, functional, correct and productive ... starting from the main theme, colours, CSS, shell, desktop rail, sidebar, header, navbar, main dashboard, hero section ... then for every section and area deep research and the best layout, spacing and implementation ... charts, graphs, figures, maps ... transitions, effects ... every minor and major thing."*

**Contents**

- [0. How to use this document](#0-how-to-use-this-document)
- [1. The product, in one paragraph, and what "best of the best" means here](#1-the-product-in-one-paragraph-and-what-best-of-the-best-means-here)
- [2. The rules that never change (merged from every earlier directive)](#2-the-rules-that-never-change-merged-from-every-earlier-directive)
- [3. Where the interface stands today (read 2026-10-26, from the code)](#3-where-the-interface-stands-today-read-2026-10-26-from-the-code)
- [4. Foundations: theme, colour, type, space, depth, CSS architecture](#4-foundations-theme-colour-type-space-depth-css-architecture)
- [5. The application shell](#5-the-application-shell)
- [6. Every page](#6-every-page)
- [7. The data-visualisation system](#7-the-data-visualisation-system)
- [8. Motion](#8-motion)
- [9. Feedback, loading and states](#9-feedback-loading-and-states)
- [10. Content and language](#10-content-and-language)
- [11. Accessibility](#11-accessibility)
- [12. Performance budgets](#12-performance-budgets)
- [13. Components and dependencies](#13-components-and-dependencies)
- [14. Verification and the definition of done](#14-verification-and-the-definition-of-done)
- [15. The per-stage loop (binding, merged from the master plan 7.2, the Stitch plan and the dark directive's six steps)](#15-the-per-stage-loop-binding-merged-from-the-master-plan-72-the-stitch-plan-and-the-dark-directives-six-steps)
- [16. The stages (aligned with the master plan's U0 to U10)](#16-the-stages-aligned-with-the-master-plans-u0-to-u10)
- [17. The merge ledger: every earlier directive, and what became of it](#17-the-merge-ledger-every-earlier-directive-and-what-became-of-it)
- [18. Sources and open decisions](#18-sources-and-open-decisions)

## 0. How to use this document

1. Read sections 1 to 3 once (the rules and the starting point). They do not change from stage to stage.
2. For the stage you are given, read its card in section 16, then the sections it names (each section is written so that it can be read alone).
3. **Every stage starts with research, not with CSS** (section 15, the loop). The research here is the *floor*, not the ceiling: a stage note must add the sources for its own area and may overrule a rule below only by saying which source and why, and the owner decides by looking.
4. Where a rule says **PROPOSED**, it is a recommendation that needs the owner's yes before it is built (a dependency, a theme, a layout). Where it says **MUST** or **NEVER**, it binds.

**Confidence marks used in the research register ([`research/RESEARCH_REGISTER_20261026.md`](research/RESEARCH_REGISTER_20261026.md)):** **V** = read in a primary source on 2026-10-26; **S** = reported by secondary sources (blogs, summaries) and to be checked in the primary source when a stage depends on it; **K** = established practice that was not re-read this session. A rule that rests on **S** or **K** says so.

---

## 1. The product, in one paragraph, and what "best of the best" means here

NexusAI is a **case-centric investigation workspace** for analysts with no technical training: they open a case, add evidence, ask in plain words, and receive an answer they can defend, in which **every number opens its source row** (contract §0, §4). It is an *instrument*, used for hours, read in courtrooms; it must look like a record, never like a game, a chat toy or a marketing page.

"Best of the best" is therefore **not decoration**. It means, in this order:

| Quality | What it demands of the interface |
|---|---|
| **Correct** | Every figure on screen is the service's figure; a control that does nothing is not shown; a state the service cannot speak for is a designed state, not an empty chart. |
| **Legible** | Hierarchy by type, space and tone, not by boxes; one idea per region; nothing that has to be explained twice. |
| **Fast** | The interface never makes the analyst wait without saying what is happening; interaction latency under 200 ms at the 75th percentile (INP, **V**: web.dev); motion never slows work. |
| **Calm** | Quiet chrome, one accent, colour for meaning only; the analyst's eye is spent on evidence, not on the application. |
| **Inclusive** | WCAG 2.2 AA in both themes, keyboard-complete, screen-reader-true, colour never the only signal, Urdu and mixed-direction text handled deliberately. |
| **Delightful where it earns it** | Motion, depth and craft are used where they *explain* (where something came from, what changed, what is selected), and are absent where the analyst repeats an action a hundred times a day. |

---

## 2. The rules that never change (merged from every earlier directive)

**Truth and data**
1. **Real service data only.** The interface shows what the API returned. It never simulates a missing backend (contract §9). Where a design needs a read that does not exist, it shows a *designed* state ("not available yet", with what is possible) and the need is filed as a row in `docs/work/BACKEND_REQUESTS.md` (endpoint, exact fields, page served, empty and error behaviour, privacy).
2. **No fabricated trends, no decorative numbers.** A sparkline, a delta or a count-up exists only over a real series or a real value.
3. **Evidence strength is always visible:** *Observed* (in the records), *Inferred* (derived by a stated rule), *Candidate* (a lead, not a finding), each a **word and a shape**, never colour alone. A source row and a model guess never look alike.
4. **Refusals, clarifications and "too broad" are successes of the design**, styled as careful results, never as errors or findings.
5. **Real-data policy.** Real case values never enter a screenshot, a repository file, a test fixture, a prompt to a cloud tool or a design tool. Demonstrations, screenshots and fixtures use the **generated (synthetic) cases** (the wave-1 development cases, for example `nexusai-wave1-dev2-20261021`). Cloud design tools receive synthetic content only.
6. **No analyst-facing jargon** (contract §9): never a model, agent, operation, template, processor, backend, SQL (one level down, for administrators), vector, embedding, threshold or token count.

**Work discipline**
7. **Design first.** No stage begins by editing CSS. The loop of section 15 is binding: research → two or three options the owner can *look at* → the owner decides → build over the existing, tested data hooks → prove → close.
8. **One owner per stage, one tree, one branch.** While a stage is open its owner alone edits the shell, tokens and shared styles; anyone else files a request. Claude edits no interface path while a Codex stage is open and says so in `NEXUSAI_CONTINUATION.md`; Codex edits no `api/**`, `db/**`, `ingestion/**`, `tools/**`, `evaluation/**` or `docs/data/**`.
9. **Keep what works:** the data hooks, presentation logic and their tests (`analystPrimaryAnswer`, `analystAskPresentation`, `analystDataPresentation`, `investigationPresentation`, `connectionsModel`, `resultShapes`, `src/lib/workbench/**`, the viewers' logic); the ECharts wrapper, Cytoscape and MapLibre; **replace markup and CSS stage by stage**, never half-migrated. Old and new tokens coexist only behind an alias layer that is deleted in the last stage.
10. **Approved dependencies:** `react`, `react-dom`, `react-router-dom`, `lucide-react`, `echarts`, `cytoscape`, `maplibre-gl`, `yaml`, the Noto fontsource packages, and the dev tools already present (vite, vitest, Testing Library, Playwright). **Anything else needs the owner's yes**, asked with the trade-off written (section 13 lists the proposals).
11. **Tests are part of the work.** `npm --prefix apps/investigation-workspace run test` stays green; a stage adds tests for every state, mapping and count it introduces (section 14).
12. **Commits:** `Assisted-by: <agent>:<model> [tools]` and `Claude-Session`/session trailer per `AGENTS.md`; **never** `Co-Authored-By` for an AI and never `Signed-off-by` from an AI; explicit paths only (`git add <paths>`, never `-A`); no force push, no rewriting published history; plain push to the working branch only. Commits that touch Go, `core/http/react-ui/` or realtime specs run the hook (2 to 3 minutes); never `--no-verify`.
13. **Ask first** before: a new dependency, a container rebuild or deploy, a database write or migration, a model download, anything touching retained evidence, a long build.

**Forbidden looks (contract §8.3, kept and extended)**
Flat white canvas · flat black canvas · purple "AI" gradients · neon or cyberpunk · glassmorphism and blur-behind panels · decorative gradients behind data · marketing hero blocks · **card soup** (every element in its own bordered card) · emoji as status · skeleton shimmer that outlasts the request · progress bars that do not reflect real progress · looping or attention-seeking motion · tooltips that hold the only copy of important information · icon-only controls without an accessible name · more than one accent colour.

---

## 3. Where the interface stands today (read 2026-10-26, from the code)

| Fact | Detail |
|---|---|
| Stack | React 18, Vite 6, React Router 7 (code-split routes), ECharts 6 (SVG renderer through one wrapper `EChart.jsx`), Cytoscape 3.34, MapLibre GL 6.11 with an offline raster basemap (`GeoMap.jsx`), Lucide, Noto Sans Variable / Noto Sans Mono / Noto Sans Arabic. |
| Styles | **About 618 KB of CSS in nine layers of three generations:** `workspace.css` 296 KB, `identity-a.css` 205 KB ("Identity A": navy-teal gradient chrome, cobalt and teal glows, radial "identity field"), `shell.css` 34 KB, `workbench.css` 28 KB (new, 2026-10-26), `connections.css`, `canvas.css`, `palette-options.css`, `tokens.css` (287 custom properties, vocabulary `--analyst-*`). No cascade layers. Legacy aliases everywhere. |
| Look | A dark-teal "Mineral Signal" chrome with gradients and glow, near-white cards; the owner has called it too "AI/techy" and the header out of palette with the page. |
| Routes | `/` Dashboard · `/investigate` (all cases) · `/cases`, `/cases/new` · `/cases/:id/overview, evidence, evidence/:eid, investigate, connections, timeline, activity, data` · `/activity` · `/settings` · `/design/type-proof, route-states, workbench` · `/cases/:id/connections/classic` (the page before the workbench; delete when the workbench is accepted). |
| Tests | 75 vitest files, 549 tests; 26 Playwright slice specs (`apps/investigation-workspace/tests`), some of which need a live fixture. |
| Built since 2026-10-26 | The **link-analysis workbench** (links, places, table, timeline, evidence panel; layout B chosen by the owner), inside Ask answers and on Connections: `pages/workbench/**`, `lib/workbench/**`. Its logic and tests are frozen inputs to this directive; its *look* is in scope (section 6.9). |
| What the service does not give yet | Login and identity, a server-side case record beyond the name and status, annotations, saved views, events-a-day by number for events without a cell, density of a place, the rows behind a mark as their own read. They are `BACKEND_REQUESTS` rows; the interface shows an honest state until they land. |

**The consequence:** this is a **new visual and interaction layer over existing, working data hooks**. The biggest single technical debt is the CSS (three generations in one cascade); stage U1 (section 16) removes it as part of the foundations, not as an afterthought.


---

## 4. Foundations: theme, colour, type, space, depth, CSS architecture

*Research behind this section: R1 to R6, R20, R22, R30, R35, R37, R38 in the register. This is stage U1; nothing else is built before the owner has looked at the options in 4.3.*

### 4.1 Direction: a calm instrument

Reference points and **what is taken from each** (not copied):

| Reference | Taken | Left behind |
|---|---|---|
| **Linear**: reduced its theme to three inputs (base colour, accent, contrast) generated in LCH, limited the chrome's own colour for a "more neutral, timeless" look, aligned labels, icons and buttons on one grid, and gave the sidebar higher hierarchy and density (R6, **V**) | Few inputs, generated scales, neutral chrome, alignment as a quality; the six-week, flag-gated, dogfooded process | Its marketing look |
| **IBM Carbon**: four fixed themes with role-based tokens that never change role between themes; layering (`background`, `layer-01/02/03`); productive vs expressive motion; five table densities (R4, R11, R13, **V**) | The token-role discipline; layering; the density ladder; the motion split | Its blue |
| **Radix Colors**: a 12-step scale in which each step has one job (1 app background, 2 subtle background, 3 to 5 component backgrounds, 6 to 8 borders and focus, 9 solid, 10 solid hover, 11 low-contrast text, 12 high-contrast text) (R3, **V**) | The *job per step* idea for every ramp | Its specific hues |
| **Material 3**: tone-based surface containers instead of shadows for depth; dark themes on a grey base, never pure black; lighter tonal accents on dark to avoid vibration (R5, **S**) | Depth by tone; dark theme rules | Its shapes and motion |
| **Atlassian**: one navigation system, global items in a top bar, project items in a side navigation, sections pinnable and collapsible (R7, **S**) | The global/context split | Its density |
| **Palantir Blueprint, Elastic EUI**: dense, composable, desktop-first enterprise systems (R36, **V** thin; Blueprint **K**) | Density as a feature | — |
| **Cambridge Intelligence** and the investigation tools in the contract §3 (AXIOM, Cellebrite, Nuix, Maltego, i2) | Coordinated views, a small working set, evidence one click from every mark | Their chrome |

**Personality, in five lines.** Quiet chrome, loud evidence. One accent, used for what can be acted on. Hierarchy by type and space, not by borders and shadows. Numbers are always tabular and always honest. The interface never performs: no motion is there to be admired.

### 4.2 Token architecture (the contract between design and code)

1. **Three tiers.** *Primitive* (a colour at a lightness, a size, a duration), *semantic* (what it is for: `--nx-surface-1`, `--nx-text-muted`, `--nx-border-subtle`, `--nx-accent`, `--nx-status-failed`), *component* (`--nx-table-row-hover`). **Components use only semantic and component tokens.** A raw colour, a pixel value for spacing or a hard-coded duration in a component stylesheet is a defect (enforced by a test that scans `src/**/*.css`, see section 14).
2. **One source of truth, in the open format.** A DTCG-format file (`design/tokens.json`; the Design Tokens Community Group's format reached its **first stable version 2025.10** on 2025-10-28, with theming, Display P3 and Oklch support, **S**: designtokens.org; it is a Community Group report, not a W3C standard) holds the primitives and semantics for each theme. A small generator (`design/build-tokens.mjs`, plain Node, no new dependency) writes `styles/tokens.generated.css` and a JSON copy the tests read. The generated file is committed.
3. **Names.** Prefix `--nx-` (replacing the nine ad-hoc families of the old CSS); role-based, never value-based (`--nx-accent`, never `--nx-teal`); a role never changes meaning between themes (Carbon's rule, R4).
4. **Themes by `color-scheme` and `light-dark()`** (`light-dark()` is Baseline newly available since May 2024, R2/R15 **S**; it needs `color-scheme: light dark`; the generator writes each theme's resolved values explicitly and components do not nest `light-dark()` inside other colour functions, to be re-verified on the stage). Keep the three states of the contract: `:root` light; `@media (prefers-color-scheme: dark)` guarded by `:root:not([data-theme="light"])`; `:root[data-theme="dark"]`. A no-flash inline script in `index.html` reads the stored choice before paint. A third, **high-contrast** theme ships with U1 (see 4.3).
5. **Register the tokens that animate** with `@property` (a token that animates must be typed or it jumps, R2 **V**).
6. **Derived values by relative colour syntax** in a theme-generation script, not in components: hover, pressed, selected and disabled are *roles* with their own steps; no component computes `color-mix` on the fly (keeps contrast auditable).

### 4.3 Colour

**Method (MUST).** Ramps are **generated in OKLCH** (perceptually uniform, so equal steps look equal; lightness fixed per step and per theme to meet contrast targets; chroma peaking mid-ramp and muted at the ends; R1/R2 **V/S**), then checked, never eyeballed. Each ramp has 12 steps with the job per step of Radix (R3) for the neutral and accent ramps, and fewer for status.

**Roles (names fixed; values are for U1 to propose).**

| Group | Tokens | Rule |
|---|---|---|
| Surfaces | `surface-0` app backdrop · `surface-1` working surface · `surface-2` raised (panels, drawers) · `surface-3` overlays (menus, popovers, previews) · `surface-ask` the question surface | **Depth is tone first, shadow only for overlays** (R5). Light mode is never `#fff` as the only surface; dark mode is **never pure black** (Material's 15.8:1 body-text guidance, halation, R5 **S**); at least four visibly distinct levels in both. |
| Text | `text-1` primary · `text-2` secondary · `text-3` muted · `text-on-accent` | 4.5:1 on **every** surface it can sit on; muted text is still 4.5:1 (it is read to act). |
| Lines | `border-subtle` (6) · `border-default` (7) · `border-strong` (8, also focus) | 1 px. Subtle borders between regions, strong only on interactive controls. |
| Accent | `accent` · `accent-hover` · `accent-pressed` · `accent-subtle` (tint) · `accent-text` (accent as text) | **One accent** that means "you can act on this / this is selected". Never reused as a status or a data colour. In dark mode use the lighter tone (about the 200 tone of the tonal palette) so it does not vibrate (R5). |
| Status | `status-ready` · `-processing` · `-failed` · `-excluded` · `-info`, each with a `-surface` tint | A dot or icon **and** words. Flat tint at 4 to 8% mixed into the surface (brief §11.1), never a gradient. |
| Evidence strength | `strength-observed` · `-inferred` · `-candidate` | Distinguished by **shape** (solid, hollow, dotted) as well as tone; the existing badge component is the reference. |
| Data | `data-1..6` categorical + `data-gray` for context, missing and "others" | See 7.2: at most six categories; lightness differs between neighbours so they survive greyscale; every categorical series also has a **pattern fallback** (ECharts decals). |
| Scales | `seq-1..7` sequential (one hue family), `div-1..7` diverging (two sequential scales joined at their lightest) | Wide lightness range; hue shifts slightly to widen it (R22 **V**). Never used for categories. |
| Focus | `focus-ring` | 3:1 against **every** adjacent surface, in all three themes; 2 px solid with 2 px offset; never removed. |

**Chrome.** The header and the rail are made **from the surface and text tokens** (a neutral that leans slightly to the accent hue), with the accent only on the active marker. The September "Identity A" gradient chrome, the radial "identity field" and the cobalt/teal glows are **retired** (they are the "AI/techy" look; the header currently mixes a cobalt glow with a teal mark and does not match the page, which is the owner's standing complaint). A gradient may remain in **one** place: an empty state or the first-run hero band, as a flat-to-flat tonal ramp, never behind data.

**The decision for the owner (U1 delivers it; D23 is re-opened only as far as this table).** D23 (2026-10-21) kept the existing teal themes. The owner has now asked for the theme and colours to be redesigned from research. U1 therefore produces **two directions, both inside every rule above**, on a gallery route `/design/foundations`, and the owner chooses by looking:

| | **Direction A, "Teal instrument"** (continuity with D23) | **Direction B, "Ink and signal"** (a step further) |
|---|---|---|
| Neutrals | teal-tinted (hue about 205, chroma at most 0.012) | near-neutral warm-cool grey (chroma at most 0.005) |
| Accent | the current teal family refined (about `#0e7490` light, a lighter tone on dark) | a deeper blue-teal with a stronger step contrast |
| Light | cool paper surface, white working surface | warm paper backdrop, white working surface, deep ink text |
| Dark | deep teal-slate, four tones | deep neutral slate, four tones |
| Both | the same status, strength and data sets; both pass the same contrast table | |

U1 must render **the same three screens in both directions, both themes, at 1440 and 390 wide**: the Dashboard (with its hero and metric cards), an Ask answer with the workbench inside it, and a Case Overview. Each with its contrast table. Synthetic data only.

**Contrast targets (MUST, asserted in tests across every token pair that is used together):** text 4.5:1 (large text 3:1); non-text UI and chart marks 3:1; focus ring 3:1. WCAG 2.2 is the enforceable standard; **APCA/WCAG 3 is informative only** (WCAG 3 is a Working Draft of 2026-03-03, not a Recommendation, with Bronze/Silver/Gold levels; R35 **S**): a stage may record Lc values beside the ratios, never instead of them. Colour-blind safety: red-green pairs are not used alone (blue-orange is the default pair); a simulator check is part of the stage (R22 **V**).

**High-contrast theme and system modes.** `prefers-contrast: more` and `forced-colors: active` are honoured (system colours for borders, focus and selection; no information lost when colours are replaced). A user-facing choice in Appearance: *System / Light / Dark / High contrast*.

### 4.4 Typography

- **Families.** UI: **Noto Sans Variable** (keeps one family for Latin and the Arabic-script fallback; the owner's D23 choice; Inter and Geist were considered: both have tabular figures and are fine for dense UI, but Noto keeps Latin and Urdu metrics consistent, **S/K**). Identifiers, hashes and queries: **Noto Sans Mono**. Urdu: *labels* in Noto Sans Arabic (cleaner, current), *evidence text* shown as written in a Nastaliq face (**decision O-6 for the owner**: do analysts expect Nastaliq?). Nastaliq needs a **line-height of about 1.8 to 1.9** (diacritics collide at 1.5, R37 **S**, single source: verify against a native reviewer, PL programme). Load fonts with `font-display: swap`, subset to used scripts, preload only the UI face.
- **Scale** (rem; the app is dense, so fixed sizes, not fluid, except the page title): caption 0.75 · label 0.8125 · **body 0.875** · body-large 1 · title-3 1.125 · title-2 1.375 · title-1 1.75 · display 2.25 (hero figure only). Text an analyst must read to act is **never below 0.8125 rem**; captions only for secondary metadata.
- **Weights:** 400, 500, 600 only. Hierarchy comes from size, weight and tone together, not from all-caps (small-caps labels with letter-spacing 0.06 to 0.08 em are allowed for *region labels* only).
- **Numbers:** `font-variant-numeric: tabular-nums` on everything numeric (tables, metric cards, axes, timelines); right-aligned in columns; thousands separators by locale; units attached (`6.29 km`, `22 minutes`).
- **Prose:** measure 65 to 80 characters; line-height 1.5 to 1.6 in prose, 1.35 to 1.45 in dense UI; `text-wrap: balance` on headings and `pretty` on paragraphs where supported (progressive enhancement, **K**).
- **Mixed direction:** every user-originated string that can contain Urdu or an identifier is rendered in an element with `dir="auto"` or inside `<bdi>`; identifiers (numbers, IMEIs, IPs, hashes) are always `dir="ltr"` and monospace with a copy affordance.

### 4.5 Space, radius, depth, lines

- **Spacing:** a 4 px base: 2, 4, 8, 12, 16, 20, 24, 32, 40, 48, 64. Layout uses `gap` and grid, never per-element margins that collapse. Inside a card 16 (comfortable 20); between cards 16 (comfortable 24); page gutters 24 desktop, 16 phone.
- **Radius:** 4 (chips, inputs, table cells), 6 (buttons, segmented controls), 8 (cards, panels), 12 (dialogs, popovers). Pills only for badges and counts. One radius per nesting level; an inner radius is never larger than its container's.
- **Depth:** surfaces by tone (4 levels); shadow only on **overlays and things being dragged** (menu, popover, dialog, toast, drawer when it floats); one soft shadow per level; no coloured shadows or glows. Borders are 1 px hairlines at `border-subtle`; a region is separated by a hairline or by tone, never by both plus a shadow.
- **z-index scale** as tokens (`base 0, sticky 10, rail 20, overlay 30, modal 40, toast 50, tooltip 60`); no magic numbers.
- **Focus:** a visible 2 px ring with 2 px offset on `:focus-visible` for everything; the browser's default ring is not removed, it is restyled; `scroll-padding-top` equals the sticky header height so a focused element is never hidden under it (WCAG 2.4.11 Focus Not Obscured, R30 **V**).

### 4.6 Density and the layout grid

- **Density is a feature** (contract §8.1; investigators scan). Three modes through one attribute `data-density="compact|default|comfortable"` on `<html>`, remembered per browser: table row heights **32 / 40 / 48 px** (Carbon's ladder xs 24, sm 32, md 40, lg 48, xl 64; R11 **V**: row height, header row, toolbar and pagination move together), control heights 28 / 32 / 36, card padding 12 / 16 / 20. **Default for tables is compact** (owner, 2026-10-12); default for pages is default.
- **Grid:** content is a 12-column fluid grid, gutters 16 (24 at xl), max content width 1440 (reading text 72 ch, never full width). Pages compose from **regions** (section 5.1), not from nested cards.
- **Breakpoints** (min-width): 480 · 768 · 1024 · 1280 · 1536. Components answer for *their own width* with **container queries** (the workbench already does); media queries are for the shell only.
- **Responsive contract (amends contract §11):** ≥1280 three zones (rail, working surface, contextual panel); 1024 to 1279 the contextual panel becomes a drawer, the rail stays; 768 to 1023 the rail collapses to icons, tables keep a pinned first column and scroll horizontally *inside their own container*; **< 768 single column**, the rail becomes a drawer, tables become cards or scroll in a container, 16 px gutters, targets **≥ 44 px**, **no horizontal page scroll** at 320 (reflow, WCAG 1.4.10). Desktop pointer targets are at least 28 px (WCAG 2.5.8 needs 24; R30 **V**).

### 4.7 Iconography

One family (**Lucide**), outline, sizes 16 / 18 / 20, stroke 1.75, `currentColor`, `aria-hidden` when beside text. Primary navigation uses icon **and** label. An icon-only control has an accessible name and a tooltip that appears on focus as well as hover. Status and strength icons are fixed in a map (one place); no emoji. Brand mark: keep the code-native NexusAI mark (owner decision O-4 open).

### 4.8 CSS architecture (MUST)

1. **Cascade layers:** `@layer reset, tokens, base, layout, components, patterns, pages, utilities;` declared once in `styles/index.css`; later layers win without specificity fights; **no `!important`**, no ID selectors, no inline colours.
2. **Files:** `styles/index.css` (layer order + imports), `tokens.generated.css`, `base.css` (reset, typography, focus, forms), `layout.css` (shell, grid, regions), `components/*.css` (one file per shared component), `patterns/*.css`, `pages/*.css` (only what is unique to a page). **A page stylesheet may not restyle a shared component.**
3. **Logical properties** (`margin-inline`, `inset-block-start`, `padding-inline`) everywhere so right-to-left works without a second stylesheet.
4. **Naming:** `.nx-<component>` and `.nx-<component>--<modifier>`; state by attribute (`[aria-selected]`, `[data-state]`).
5. **Budget:** total shipped CSS **≤ 150 KB raw, ≤ 30 KB gzip per route chunk** (today 618 KB raw across nine files). The legacy files (`identity-a.css`, `workspace.css`, `palette-options.css`, `connections.css`, the old `shell.css`) are **deleted stage by stage** as each page is rebuilt; the alias layer that maps old `--analyst-*` names to `--nx-*` is removed in U10.
6. **No dead styles:** a stage deletes the selectors of what it replaces; a script lists selectors with no match in `src/**/*.jsx` (a dev check, not a gate at first).
7. **Print:** the answer, its derivation and its citations print cleanly (a `@media print` layer in `patterns`); the workbench prints its table twin.

### 4.9 What this section amends in the contract (effective on the owner's yes, decision O-1)

§8.1 "Geist is already installed" is replaced by 4.4 (Noto stays; Geist was never adopted in the build). §8.2 tokens are renamed and extended (4.2, 4.3) with the same three-state theme mechanics. §8.3 forbidden looks are extended (section 2). §8.4 motion becomes section 8 here (the 200 ms ceiling becomes a *token ladder* with a frequency rule). §11 is replaced by 4.6.


---

## 5. The application shell

*Research behind this section: R7 (Atlassian navigation), R6 (Linear), R14 (command-palette pattern), R15 (popover, dialog, anchor positioning), R26 (APG landmarks), R30 (WCAG 2.2). This is stage U2.*

### 5.1 Anatomy (desktop, 1280 and wider)

```
┌───────────────────────────────────────────────────────────────────────────────┐
│ HEADER  (56 px)  mark · case switcher · ⌘K "Search or ask…" · activity · theme · user │
├────────┬──────────────────────────────────────────────────────────────────────┤
│ RAIL   │ CASE BAND (only inside a case, 48 px): name · status · tabs           │
│ 248 /  ├──────────────────────────────────────────────────────┬───────────────┤
│ 56 px  │ WORKING SURFACE  (page title row, then regions)      │ CONTEXT PANEL │
│        │                                                      │ (optional,    │
│        │                                                      │  360–420 px)  │
└────────┴──────────────────────────────────────────────────────┴───────────────┘
```

1. **Header** (global, never changes between pages): brand mark and name (left, 248 px wide so it aligns with the rail), **case switcher** (a combobox: recent cases first, search, "All cases"), the **command palette trigger** (a search-styled button, `⌘K` / `Ctrl+K`), **activity** (running jobs and failures: a count badge, opens a popover), **appearance** (theme and density), **user** (name, role, sign-out when login exists; today a neutral "Analyst" mark). The header is made from surface and text tokens (4.3), **one row, no gradient**, with a 1 px bottom hairline. The header does not carry page titles.
2. **Rail** (global navigation, left): *Workspace* group (Dashboard, Ask across cases, Cases, Activity) and, below a hairline, the *current case* group when inside a case (Overview, Evidence, Ask, Connections, Timeline, Data, Activity), then *Settings* anchored at the bottom. Atlassian's finding that **global items belong in the top bar and project items in the side navigation** (R7 **S**) is adapted: the rail keeps both groups so the analyst never loses either, but the two are visibly separate. Each item: icon + label + optional count; the active item has the accent marker (a 3 px bar) and `aria-current="page"`.
   - Width 248 expanded, 56 collapsed (icons with tooltips shown on focus and hover); collapsed state remembered; **below 1024 it collapses by default; below 768 it is a drawer** opened from the header.
   - Counts are real service numbers only (evidence in progress, needs review).
3. **Case band** (inside a case only): case name (inline-editable, existing `CaseNameEditor`), status chip (Ready, Processing, Needs attention, from the existing readiness model), **case tabs** in the same order as the rail's case group (the rail is the primary control; the tabs repeat them for orientation on wide screens and become the *only* case navigation when the rail is collapsed to a drawer on phones). A tab never has more than a count.
4. **Working surface**: one page title row (title, one-line purpose, primary action right-aligned), then **regions** (5.3). Max readable width for prose; full width for data.
5. **Context panel** (optional, per page): the evidence panel, a row's detail, the source of an answer. 360 to 420 px, resizable by keyboard and pointer (APG window splitter pattern, **K**), closes with `Esc`, at < 1280 it is a drawer; it is a `complementary` landmark with a name. **Only one panel is ever open**; opening another replaces it.

### 5.2 Landmarks, focus, skip links (MUST)

`<header role=banner>`, `<nav aria-label="Workspace">` and `<nav aria-label="Case">`, `<main id="workspace-main" tabindex="-1">`, `<aside aria-label="Details">`; exactly one `<h1>` per route; a **skip link** to main (exists; keep) and, inside Ask, to the answer. On every route change focus moves to the page `<h1>` (or `main`) and the document title updates (`<Case name> · <Page> · NexusAI`); a polite live region announces "Loaded <page>" only when navigation is async. Route transitions never trap focus. Sticky header height is exposed as `--nx-header-h` and used for `scroll-padding-top` (WCAG 2.4.11).

### 5.3 Regions, not card soup

A page is a sequence of **regions**: each has a title (`h2`), an optional one-line explanation, optional actions on the right, and content. Regions are separated by **space (24 to 32 px) and a hairline**, not by individually bordered cards. A *card* (bordered, raised) is used only for **repeatable objects the analyst picks from** (a case, a piece of evidence, a metric) and for **overlays**. A page contains at most **one row of metric cards** (≤ 4, brief §11.1) and no card nested in a card. This is the single biggest change from today's `workspace.css`, where nearly every block is a card.

### 5.4 Command palette (⌘K / Ctrl+K)

An **ARIA combobox with `aria-activedescendant`** in a modal dialog (APG; R14 **S/K**): the input keeps DOM focus, results are a `listbox`, the active option is announced, `↑/↓` moves, `Enter` runs, `Esc` closes and returns focus to the trigger. It has three kinds of results, each with an icon and a label of the kind: **Go to** (cases, evidence by name/number, pages), **Do** (new case, add evidence, switch theme, switch density, copy case link), and **Ask** (typing a sentence ending in `?` or beginning with a verb offers "Ask this case: …", which opens Ask with the question, as the workbench does today). Recent items first, fuzzy match, matched characters marked, results grouped, max 8 per group, a quiet "No match, press Enter to ask" empty state. It never fires a model call by itself. Shortcuts shown beside every command; `?` opens the shortcut sheet. The existing `WorkspaceCommands.jsx` is the data source; its tests stay.

### 5.5 Activity and notifications

The header's **activity** control replaces scattered banners: a popover (native `popover` where available, R15 **S**) listing running processing jobs (with real progress where the service reports it, otherwise an indeterminate bar and elapsed time), recent completions and failures; failures persist until read (toast rules in 9.4). A job's row links to its evidence. Nothing here is decorative.

### 5.6 Appearance menu

Theme (System / Light / Dark / High contrast), density (Compact / Default / Comfortable), **text size** (follows the browser; the control only reminds that the page respects it), reduced motion (follows the system; a manual "Reduce motion" override in the menu, **K**), and the *Answer detail* default (Summary / Show working) that Ask already has. One popover, all stored in `localStorage` behind try/catch, and applied before first paint.

### 5.7 Responsive behaviour of the shell

| Width | Rail | Case band | Context panel | Header |
|---|---|---|---|---|
| ≥ 1280 | expanded (or collapsed by choice) | tabs | beside the surface | full |
| 1024–1279 | collapsed icons | tabs | drawer | full |
| 768–1023 | collapsed icons | tabs scroll | drawer | switcher shortens |
| < 768 | drawer from header | tabs become a segmented *select* ("Overview ▾") | bottom sheet (full height, grab handle, `Esc`/swipe) | mark, switcher icon, ⌘K icon, menu |

Pointer targets on touch ≥ 44 px; the rail drawer traps focus while open and restores it on close; no hover-only controls on touch.

---

## 6. Every page

Each page below has the same five lines: **Job** (what the analyst is here to do), **Anatomy** (regions in order), **Research basis**, **Data** (the existing hooks and what is real), **States** (empty, loading, error, partial, refusal). A page stage (section 16) delivers: the research note for the page, the options viewed by the owner, the build, the tests for each state, the evidence of both themes at 1440 and 390 wide.

### 6.1 Dashboard `/`

- **Job:** "Where do I stand across my cases, and what needs me now?" in under 10 seconds.
- **Anatomy (top to bottom):**
  1. **Hero band (not a marketing hero).** Left: a plain greeting-free title ("Your cases"), and under it **one sentence that is the state of the workspace, computed** ("3 cases need your attention, 2 are still processing, 1 has new answers"), all real. Right: the **primary action** (New case) and a secondary (Ask across cases). Beneath, a **focus strip**: up to three **"Continue" items** (the last question in each of the most recently touched cases, or the evidence that is mid-processing), each one a single line with a time, a status chip and one action. The band is the only place a tonal gradient (flat-to-flat, 3 to 5% difference) may be used; height ≤ 220 px at 1440.
  2. **Attention queue**: the list that answers "what needs me": items grouped by cause with the existing `attentionCauses` model (failed evidence, needs review, ambiguous columns, stale processing), each row: cause icon + sentence + case + one action. If empty, an affirmative empty state ("Nothing needs you") with the last activity time. **This replaces KPI-first dashboards**: NN/g's finding that dashboards fail when they show data without helping decisions (R17 **S**).
  3. **Metric row (≤ 4 cards)** per brief §11.1: *Cases open*, *Evidence ready / total*, *Answers in the last 7 days*, *Needs review*. Each card = label, **one figure** (tabular, 2 rem), one comparison line **only if a real previous period exists** (no fabricated deltas), a 7-day sparkline **only if there are ≥ 7 real points**; the whole card is a link to the filtered list behind it.
  4. **Cases table**: the working list (6.2) trimmed to the 8 most recent with "View all"; row: name, status chip, evidence progress, last activity, owner (when it exists), one primary action. Compact density default.
  5. **Activity over time** (a single quiet area/bar chart of events per day, real; a table twin behind "Show as table") and **where the evidence is** (the offline map of the evidence's sites, only when ≥ 1 site exists, else omitted, not shown empty).
- **Research basis:** R17 (NN/g dashboard guidance: purpose-led, encode with position and length, no pies/gauges/3D, colour secondary), R19 (Carbon dashboard/data-viz patterns **S**), R6 (Linear's focus on a short list of what matters), the owner's "hero section" wording (taken as: the first band must *orient and offer the next action*, not decorate).
- **Data:** `useWorkspaceState`, `dashboardCases`, `dashboardCharts`, `workspaceAttention`, `attentionCauses`, `ContinueCards`. Real. Existing widgets (`PipelineFlow`, bubble pack, `ReadinessRing`) are **not carried over unless they pass the question "what decision does this support?"**; U5 records the verdict on each.
- **States:** first-run (no cases): the hero becomes a three-step start (Create a case → Add evidence → Ask) with the first step primary; loading: skeleton that matches the final layout (≥ 1 s only); service unreachable: a single page-level message with Retry, never six broken widgets.

### 6.2 Cases `/cases` and New case `/cases/new`

- **Job:** find a case, create one. **Anatomy:** page title row (title, search, New case); **filter bar** (status, owner when it exists, date: as chips, applied filters visible and removable); **table** (sortable columns, keyboard row navigation, row click opens, row action menu); empty/first-run state with a one-line explanation and the primary action.
- **Table rules (MUST, from the table research R11, R12):** header row sticky; sort state announced (`aria-sort`); numeric columns right-aligned, tabular; selection by checkbox (24 px target) with a bulk bar that replaces the toolbar; row hover and selected are tone steps (not shadows); **rows virtualized only above 200 rows and with `aria-rowcount`/`aria-rowindex`** (TanStack Virtual does not set them; R12 **S**); a column menu to show/hide; **pagination or "load more" is server-driven when the list is server-driven** (virtualization is not paging); density follows the global control; zebra stripes **off**; the table is `role=table` (grid only if cells are editable).
- **New case:** one screen, **three fields at most** (name, optional description, optional time zone/jurisdiction when the service stores it), name pre-focused, `Enter` creates and goes to Add evidence, validation inline with a message that tells the fix (WCAG 3.3.1, 3.3.7 redundant entry); no wizard.

### 6.3 Case overview `/cases/:id/overview`

- **Job:** "Is this case ready, and what should I do next?" **Anatomy:** (1) status line + **next best action** (one button); (2) **readiness** (evidence by type and stage as a stacked horizontal bar with a table twin, not a ring with a decorative centre; the existing `ReadinessRing` is replaced if it cannot carry the table twin); (3) **what the case contains** (people/numbers/places counts from the typed views, each a link into Connections/Data); (4) **recent questions and answers** (the trail, newest first, each opens in Ask); (5) **notes** (the existing local case notes, marked "stored on this browser only" until the service stores them).
- **Data:** `useCaseOverview`, `caseDossier`, `caseNotes`, `caseActivity`. **States:** processing (live progress, real), needs attention (the cause and the fix), empty (Add evidence is the page).

### 6.4 Evidence list and detail `/cases/:id/evidence`, `/evidence/:eid`

- **Job:** add, see the state of, and open evidence. **List anatomy:** add-data drop zone as the **top region** (large, keyboard-operable "Choose files" button and drag-and-drop, with the **file-upload pattern**: a *card per file* with name, size, type, stage, progress (real), **retry** on failure, remove; a live region announcing completion; R31 **S**); below, the evidence table (type icon, name, stage chip, rows/pages/duration, added, actions) with **type filter chips** (Records, Documents, Images, Audio/Video) and a search.
- **Detail anatomy (viewer):** a two-pane layout: **the artefact** (left, dominant) and **the evidence panel** (right: what the system understood, the entities found, the transcript or OCR text with the matched span highlighted, the *citations that point here*). A toolbar per viewer type from the existing viewers: documents (page nav, zoom, find, highlight spans), images (pan/zoom, rotate, EXIF, *OCR boxes toggle*), audio/video (waveform + transcript, speed, click-a-line-to-seek, speaker labels with "uncertain" marking), records (virtualized table with the schema detected). Every viewer has a keyboard map shown on `?`, a **"Show where the answer came from"** deep link (`?span=…`) that scrolls and highlights the cited span, and **never auto-plays**.
- **Research basis:** forensic review tools (contract §3), R31 (file upload), R32 (media accessibility: captions/transcripts, no auto-play, 1.4.2), R12 (virtualization with aria-rowcount).
- **States:** processing (stage-aware: "Reading pages 14 of 40"), failed (reason in plain words + retry + "exclude"), excluded (greyed, with why and "include again"), unsupported type (what is accepted).

### 6.5 Ask `/cases/:id/investigate` (and `/investigate` across cases)

This page carries the product (contract §4: "every number opens its source row"). It is **one conversation-shaped column plus the evidence panel**, not a chat toy.
- **Anatomy:**
  1. **Question composer** (sticky at the bottom on phone, at the top of the answer area on desktop): a multi-line field (auto-grows to 6 lines), `Enter` sends, `Shift+Enter` newline, **a visible scope line** ("Asking: this case · all evidence · all dates") with chips to narrow (existing `ScopeChips`), **suggested questions drawn from this case's real data** (the `suggestionsFor` logic), the **right to stop** while a question runs.
  2. **The answer, in a fixed anatomy** (order is a rule, so the analyst learns where things are): **(a) the sentence** (what the data says, plain words, with the figure large and in tabular figures) → **(b) strength** (*Observed / Inferred / Candidate*, word + shape) and **scope used** ("March 2026 · numbers 923…003 and …007 · both exports") → **(c) the picture** (workbench: links, places, table, timeline) when the answer is drawable → **(d) how it was worked out** (collapsed by default: steps in plain language; "Show query" one level down for administrators; the checks that ran) → **(e) limitations and "what would change this"** (always visible when non-empty, never buried) → **(f) follow-ups** (3 suggested next questions from the result's own columns) → **(g) copy / export / cite**. Refusals and clarifications use the same anatomy with their own heading and tone (success of the design, rule 4).
  3. **Question trail** (the existing `QuestionTrail`/`QuestionHistory`): earlier questions of the case as a compact list on the left or in a drawer; each re-opens its saved answer without a model call.
- **Waiting is designed (9.2):** model answers take 10 to 120 s. The waiting state shows **what is being done in the analyst's words** (reading your question → looking in the records → checking the result → writing it up), driven by the service's real step events when they exist, otherwise by honest elapsed time with **"Stop"**; **never a fake percentage**; the composer stays usable for the next question; leaving the page does not cancel (and returning finds the answer).
- **Research basis:** R17 (NN/g response-time limits and progress), R10 (error-message guidance), R28 (clear errors and recovery), the owner's brief (non-technical analyst), the engine's own result shape (`readAnswer`).
- **Data:** `InvestigatePage`, `InvestigationResult`, `analystPrimaryAnswer`, `analystAskPresentation`, `investigationPresentation`, `WorkbenchLocal` (inside answers). **Unchanged:** the engine and every fact it states.
- **States:** no evidence (explains and links to Evidence), evidence still processing (can ask, warned that answers may be incomplete, with a count), timeouts (what was tried, **what to try instead**, one-click retry), "too broad" (offers narrower questions), "no such rows" (states the scope searched; see the thin-answer wording programme).

### 6.6 Connections (the workbench) `/cases/:id/connections`

- **Job:** "Show me the whole case: who is connected to whom, where, when." The owner chose **layout B** (one pane at a time: Links, Places, Table; the timeline always underneath; the evidence panel beside). This stage **restyles** it to the master foundations and **adds the research items below**; it does not change the data layer.
- **Restyle (must):** remove the workbench's private tokens in favour of `--nx-*`; one pane toolbar with a **segmented control** (Links / Places / Table) that is keyboard-operable (`role=tablist`, arrow keys); the evidence panel uses the shared context-panel shell; marks and badges use the strength tokens; the table pane uses the shared table.
- **Research items to implement (from Cambridge Intelligence guidance and map accessibility, R23, R24):** (1) **working-set discipline** (the view opens on a *meaningful* subset: subscribers plus contacts shared by ≥ 2; a visible count "showing 38 of 412, change"); (2) **combos** (a community collapses to one node that expands in place, with a count; PROPOSED, needs a stable group in the data, else omit); (3) **level of detail** (labels only on hover/selection or above a zoom threshold; edge weight as width with a legend); (4) **time bar** (the timeline is also the filter; brushing it changes the graph and the map, with a "reset" chip); (5) **a table alternative to every picture** (a real table with the same marks and a "Show as table" control, WCAG 1.1.1 for complex images and the map-accessibility guidance: a keyboard route into map content, a text description); (6) **uncertainty drawn** (cell-site positions as **circles of the stated radius**, not exact points; "inferred" positions dashed); (7) **map base** muted and low-contrast so marks carry colour (R24 **S**), clustering via MapLibre's native cluster layer above ~200 marks, a density/heat layer only for *counts of events*, legend and scale always visible, **no WebGL graph renderer unless the owner approves a dependency** (the 2D/Cytoscape renderer stays to ~500 nodes, with the working-set rule keeping it there).
- **Keyboard (MUST):** the graph and map each have a **listbox of marks** (the same set, in a sensible order) so a keyboard or screen-reader user can reach, select and open every mark; `Esc` clears selection; the selected mark's evidence is always one key away.
- **States:** no connections yet ("Add call records to see who is connected"), a place with no coordinates (placed on a *synthetic* ring and labelled so, as built), the service slow (the existing skeleton), truncated at the cap (a banner with the count and "narrow", never silent).

### 6.7 Timeline `/cases/:id/timeline`

- **Job:** "What happened, in order, across all evidence?" **Anatomy:** a **time bar** (events per day as a bar overview, brushable) → **the event stream** (virtualized, one row per event: time with offset and zone, type icon, one-sentence description, strength, source link) with **group by day and by family** (Calls, Messages, Movements, Documents, Media) and a **family rail** to filter → **the inspector** (context panel: the event, its source row, related events). Time is always shown with **zone and (where it matters) the UTC value** on hover/copy; gaps are shown as gap markers ("12 days with no events"), not hidden.
- **Research basis:** R23 (time bars), Heer–Robertson on animated transitions (R21: object constancy when filtering), forensic timeline tools (contract §3). **Data:** `timelineModel`, `timelineChart`, `timelineInsights`, `timelineAnnotations`, `EventStream`, `FamilyRail`, `Inspector`, `PlacesView`.

### 6.8 Activity `/cases/:id/activity` and `/activity`

- **Job:** audit trail and processing log: "what was done to this case, by whom, when". **Anatomy:** a filterable list (type: evidence added, processing, question asked, export, change) with **who, what, when (relative with absolute on hover), result**; a calendar heat strip (the existing `ActivityCalendar`) **only as a filter**, not decoration; export (CSV) for the audit (administrators). Immutable: no delete. **Data:** `caseActivity`, `activityFeed`, `activityReviewed`.

### 6.9 Data `/cases/:id/data`

- **Job:** "What does the system know about this case's data, and how good is it?" **Anatomy:** **data quality** (the typing report: what was understood, what was not, why, with counts only), the **engine-health** card (real, from the endpoint), the list of **layouts recognised** per evidence file, and the **view catalogue** in plain words for analysts ("Call records: 12,431 rows, 21 numbers"). Technical detail one level down for administrators. **Data:** `DataPage`, `DataQualityCard`, `EngineHealthCard`, `analystDataPresentation`.

### 6.10 Settings and admin `/settings`

- **Job:** appearance, defaults, and (administrators) system facts. **Anatomy:** a left sub-nav (Appearance, Data and privacy, Keyboard shortcuts, About/System) and one section at a time. **Appearance** is the single place the **theme/density/motion** controls live (the header menu mirrors it). **Keyboard shortcuts** lists every shortcut (generated from the same table as the palette). **System** shows the real health facts and versions; nothing editable that the service does not store. **Not yet in the service** (login, roles, users): a designed "coming" state per the BACKEND_REQUESTS ledger, never a fake form.

### 6.11 System pages and states

**Not found, no access, error boundary, offline, session expired, maintenance:** one shared layout (illustration-free: an icon, a title in plain words, one sentence of what happened, the **one next step** as a button, a "Copy details" link for support with an error id). `RouteErrorBoundary` (exists) renders these. Rules from the empty-state/error research (R28, R10): *say what happened, why if known, what to do; never blame the user; never show a stack*; the page keeps the shell so navigation still works. **Design gallery routes** (`/design/*`) are developer-only, linked nowhere in the product, and become the visual regression targets (section 14).

### 6.12 The design gallery

`/design/foundations` (U1): tokens, both directions, both themes, contrast table. `/design/components` (grows each stage): every shared component in every state. `/design/charts` (grows from U1, complete in U10): every chart type with real-shaped synthetic data. `/design/workbench` (exists). A route is added **before** the page that uses a component is rebuilt, so the owner can look at the part, then the whole.


---

## 7. The data-visualisation system

*Research behind this section: R19 (Carbon data visualisation), R20 (Cleveland–McGill, Munzner), R21 (Heer–Robertson), R22 (Datawrapper colour), R23 (Cambridge Intelligence), R24 (maps), R25 (ECharts), R27 (Chartability), R17 (NN/g dashboards). Tokens and the chart theme ship in U1; every stage applies these rules to the visuals on its own pages and adds them to the `/design/charts` gallery, which is completed in U10.*

### 7.1 Principles (kept short; each is testable in review)

1. **Encode with the strongest channel the question allows.** Position on a common scale, then length, then angle/slope, then area, then colour lightness or saturation (Cleveland–McGill; Munzner's expressiveness and effectiveness principles, R20 **S/K**). Therefore: bars, dots and lines carry quantity; colour carries *category or emphasis*, never quantity when position can.
2. **No pies, donuts, gauges, radar charts, 3-D, dual axes, or decorative charts** (NN/g dashboards, R17 **V**; Carbon's guidance on part-to-whole, R19 **V/S**). Part-to-whole is a **stacked horizontal bar** (100% or absolute) with a table twin. The current `ReadinessRing` and bubble-pack are replaced unless the stage note proves they answer a question a bar cannot.
3. **Overview first, zoom and filter, then details on demand** (Shneiderman, R20 **K**). Every visual has an overview state, a way to narrow (brush, filter chip, selection), and a way to the source rows.
4. **Constancy:** when a filter or time range changes, **marks that persist stay in the same place and the same colour** (Heer–Robertson, R21 **S**: animated transitions help when they preserve object congruence and are staged, and hurt when everything moves at once). The same entity (a subscriber, a place) has **the same colour and shape in every view of a case**.
5. **Every chart has its twin:** a data table, a text summary, a CSV copy (7.6). A chart without all three is not finished.
6. **Honest scales:** bars start at zero; time axes show the zone; a log axis is labelled; a truncated axis is never used for bars; gaps in data are drawn as gaps and labelled.
7. **Title states what is drawn, not a conclusion** ("Calls per day, 1 Mar to 30 Jun 2026"), with the unit and the source ("Source: call records, 3 files"). A conclusion lives in the answer sentence, with its strength.

### 7.2 Which visual for which question

| The analyst's question | Use | Rules |
|---|---|---|
| How many / how much *now* | **Figure** (tabular, large) with label and unit | One figure per card; context line only with a real baseline |
| Compare categories (≤ 12) | **Horizontal bar**, sorted descending | Direct value labels; > 12 → top 10 + "Others (n)" in grey, or a table |
| Many categories, precise reading | **Dot plot** (lollipop) | Shared axis; sorted |
| Change over time | **Line** (≤ 5 series) or **column** per day/hour | Column when counts per bucket; line when continuous; area only for a single series |
| Pattern by hour × weekday | **Heatmap** (single-hue sequential, 24 × 7) | Value on hover and focus; table twin; legend with real min/max |
| Distribution of a measure | **Histogram** (or strip plot for < 200 points) | Bin width stated |
| Part of a whole | **Stacked bar** | ≤ 5 parts, ordered, labelled directly |
| Compare to a target or threshold | **Bullet chart** | Target as a mark, ranges as three tones |
| Who is connected to whom | **Node-link** (workbench), **matrix** for dense graphs (> ~200 edges; PROPOSED) | Working-set rule (6.6) |
| Where | **Map** | 7.8 |
| What happened in order | **Timeline** with swimlanes | 7.10 |
| Several comparable series | **Small multiples** with shared axes | ≥ 4 series that would overlap on one chart |
| A trend inside a table row or card | **Sparkline** | ≥ 7 real points; no axes; end dot; 7.7 |
| Flow through stages (pipeline) | **Stage list with counts** | No Sankey unless a real flow dataset exists |

### 7.3 Colour in data

- **Categorical:** at most **six** hues (`data-1` to `data-6`) plus `data-gray` for context and "Others". A seventh category becomes small multiples or a table. Neighbouring hues differ in **lightness** as well as hue, so a greyscale or colour-blind reading still separates them (R22 **V**).
- **Emphasis:** the strongest practice for most charts is **one coloured series, the rest grey** (Datawrapper, R22 **V**). Use it for "this subscriber against the others".
- **Sequential:** one hue family, wide lightness range (`seq-1..7`); **diverging** only where the midpoint is meaningful (change from baseline); never rainbow.
- **Entity colour is stable** across the case: a small `colorOf(entityId)` map (kept in the workbench index) assigns `data-1..6` in order of first appearance and persists in the URL state; entity 7 and later are grey with a shape/label.
- **Contrast:** every mark 3:1 against its background (non-text contrast, WCAG 1.4.11); strokes on light-on-dark may be lighter than their light-theme counterparts (tonal pairs, 4.3).
- **Pattern fallback:** a **"Patterns" switch** in Appearance turns on ECharts `decal` fills for categorical series (R25 **S**); forced-colors and `prefers-contrast: more` turn it on automatically.
- **Colour-blind check** is part of every visual's acceptance: the stage note includes protanopia/deuteranopia/tritanopia simulations of the gallery page (R22/R27 **V**). A palette validator with a stated minimum distance between simulated colours is added in U1 (threshold to be set and justified in the U1 note).

### 7.4 One ECharts theme

`components/charts/chartTheme.js` is rebuilt to **read the tokens at runtime** (so the theme follows light/dark/high-contrast and density) and registers one theme `nx` used by every chart:

| Element | Rule |
|---|---|
| Text | Noto Sans, `tabular-nums` on axes and tooltips, `text-2` for axis labels, `text-1` for titles |
| Axes | Category axis line at `border-default`, no tick marks on value axes; **gridlines horizontal only**, `border-subtle`; units on the axis title or in the first label |
| Marks | Bars: 4 px top radius, 60–70% category width, 2 px gap; lines 2 px with round joins, markers only on hover/focus or ≤ 30 points; area fill 12% of line colour |
| Tooltip | A shared popover look (surface-3, 1 px hairline, 8 px radius, 8 px padding), a header with the category, rows with colour key + name + right-aligned tabular value; pinned on click and reachable by keyboard |
| Legend | **An HTML legend outside the canvas** (buttons with `aria-pressed`) wired to ECharts' `dispatchAction`, because canvas/SVG legends are not keyboard-operable (PROPOSED component `ChartLegend`); omitted when direct labels are used |
| Selection | Selected series full strength, others 25% (150 ms); hover cross-highlights the same entity in other views |
| Accessibility | `aria.enabled` with a generated description, **`decal` on demand** (7.3), keyboard focus on marks through the HTML twin (7.6) |
| Motion | First draw 400 ms `cubicOut`, update 300 ms; **`animation: false` above 1,000 marks and under reduced motion** (8.5) |
| Performance | SVG renderer under ~5,000 marks; `large: true` and `sampling: 'lttb'` for dense lines and scatters (R25 **S**: ECharts' large mode and sampling exist for this); `progressive` rendering for very large; **never a chart that blocks the main thread for > 50 ms on a 15.7 GB laptop** (measured in the stage that builds the chart) |

### 7.5 Annotation, uncertainty, freshness

- **Annotate the one thing that matters** (a peak, a gap, an event) with a label and a leader line; one per chart by default; the analyst can add more only when annotations are stored by the service (BACKEND_REQUESTS; today none).
- **Uncertainty is drawn, not described:** a position known to a radius is a circle; an inferred link is dashed; an estimate is a band with a label. A mark whose precision is unknown is not drawn as precise.
- **Data freshness** line under every chart that can lag ("Records up to 29 Jun 2026, 14:32 PKT").
- **Missing data** is a labelled gap or an explicit "No data for this period", **never zero**.

### 7.6 The table twin (MUST)

Every chart, graph, map and timeline offers **Show as table** (a toggle in the chart header, remembered per chart type). The table has the same filter and sort as the picture, the same entity names, and a **"Copy as CSV"** action. A **text summary** (one or two sentences generated from the data, e.g. "Calls peaked on 14 Mar with 212; 9 of 30 days had none") sits in the chart's accessible name/description. For graphs and maps the twin also **is the keyboard route** to every mark (6.6).

### 7.7 Metric cards and sparklines

Card anatomy: **label** (0.8125 rem, `text-2`) → **figure** (2 rem, tabular, `text-1`) → **context line** (a real previous period or a real denominator: "of 21 files") → optional **sparkline** (only with ≥ 7 real points). The whole card is one link/button to the list behind the figure (a 44 px minimum height, visible focus). **No fabricated deltas, no trend arrows without a baseline, no coloured up/down judgments** (up is not "good" for call volume). A card that cannot be computed shows "Not available yet" and why.

### 7.8 Maps

*(Research: R24, MapLibre documentation, cartographic practice for data maps, accessible-maps guidance.)*

1. **Base map recedes.** The offline raster tiles are muted through MapLibre raster paint properties (`raster-saturation`, `raster-contrast`, `raster-brightness-min/max`, **K**) so that marks, not the map, carry colour; the dark theme lowers brightness rather than inverting.
2. **Marks:** circles with a 1.5 px halo in the surface colour; area scaled with the **square root** of the value; minimum 6 px, maximum 28 px; colour by entity (7.3) or neutral; selection ring in `accent`; **cell sites are circles of the stated radius**, positions with unknown precision are not drawn as points.
3. **Clustering:** above about 200 marks use MapLibre's native clustering (count labels, click to zoom into the cluster); **density/heat** only for *counts of events*, with a legend of real values, never for presence.
4. **Tracks:** lines with direction chevrons and time ticks; a gap larger than the case's gap setting breaks the line.
5. **Always visible:** scale bar, attribution (the tile source's required attribution; **the offline tile source is unverified, O-8**), legend of marks, "Fit all", zoom +/−, an **"Illustrative positions"** label when the placement is synthetic (as built).
6. **Camera:** fly-to 400 to 700 ms only after a user selection; **never re-centres on data refresh**; instant under reduced motion; the camera is in the URL state so a link reproduces the view.
7. **Interaction:** keyboard map keys (arrows pan, `+`/`−` zoom, `Enter` opens the focused mark, `Esc` clears) documented on `?`; **marks listbox** beside the map; **table twin**; marks have names, not only coordinates.
8. **WebGL:** MapLibre is WebGL already; the *graph* stays 2-D (Cytoscape) unless the owner approves a WebGL graph renderer (sigma.js, PROPOSED, only if the working-set rule cannot keep a case readable).

### 7.9 Graphs (link charts)

*(Research: R23, Cambridge Intelligence on design, large networks and time.)*

- **Encoding (one channel per attribute, always with a legend):** node **shape** = kind (subscriber, contact, place, device), node **size** = events (one attribute only), node **colour** = entity colour or community (never both), edge **width** = events, edge **style** = strength (solid observed, dashed inferred, dotted candidate), edge **direction** only when direction is real.
- **Labels:** the person's name, else the last four digits; labels on selection/hover and when zoomed past a threshold; **how many nodes/edges are hidden is always shown** ("38 of 412, change").
- **Layout is stable:** a seeded layout, so reopening the case gives the same picture (constancy); switching layout animates 400 ms; never re-lay-out on selection.
- **Declutter:** working set first (6.6), then collapse communities into combos (PROPOSED, needs real groups), then filters; **never** draw all 400 nodes and call it an overview.
- **Direct manipulation alternatives (WCAG 2.5.7):** anything done by dragging (pinning a node, brushing the time bar, resizing the panel) has a keyboard or single-pointer equivalent.

### 7.10 Timelines

Swimlanes by family (Calls, Messages, Movements, Documents, Media); **semantic zoom** (aggregate bars when dense, individual events when sparse); time axis with zone and a UTC toggle; a brushable overview bar; gap markers; the *last evidence* marker; keyboard: `←/→` previous/next event, `+/−` zoom, `Enter` opens the inspector; the list below the picture is the twin.

---

## 8. Motion

*Research behind this section: R13 (Carbon motion), R16 (View Transitions), R18 (NN/g animation duration), R21 (animated transitions), R40 (prefers-reduced-motion, WCAG 2.3.3, 2.2.2). Motion tokens ship in U1; each stage applies the catalogue to its own pages; the final sweep is U10.*

### 8.1 Principles

1. **Motion explains.** It shows *where something came from* (a source row highlighted when its number is clicked), *what changed* (a chart that morphs instead of replaced), *what is selected* (cross-highlighting), or *that the system is working*. Anything else is removed.
2. **Frequency decides.** What an analyst does a hundred times a day (open a menu, change a sort, move through rows, switch a tab) gets **no animation or the shortest one**; what happens once per session or once per case (answer arrives, first-run complete) may be a little richer.
3. **Never in the way.** Animations are interruptible, never block input, never delay a result; **nothing over 500 ms except a progress indicator or a one-off moment** (NN/g, R18 **V**: roughly 100 to 500 ms reads as responsive; longer feels slow).
4. **Two registers** (Carbon, R13 **V**): **productive** (efficient, the default for everything the analyst works in) and **expressive** (a little more presence, used for at most the few moments listed below).

### 8.2 Tokens (shipped in U1)

```css
--nx-dur-1: 70ms;   /* instant feedback: hover/press tone */
--nx-dur-2: 110ms;  /* small elements, exits, tooltips */
--nx-dur-3: 150ms;  /* menus, popovers, tab indicator, view cross-fade */
--nx-dur-4: 240ms;  /* panels, drawers, rail, toasts in */
--nx-dur-5: 400ms;  /* charts first draw, graph layout, map fly-to (short) */
--nx-dur-6: 700ms;  /* one-off emphasis only (source-row tint fade) */

/* productive (default) */
--nx-ease-standard: cubic-bezier(0.2, 0, 0.38, 0.9);
--nx-ease-enter:    cubic-bezier(0, 0, 0.38, 0.9);
--nx-ease-exit:     cubic-bezier(0.2, 0, 1, 0.9);
/* expressive (rare) */
--nx-ease-x-standard: cubic-bezier(0.4, 0.14, 0.3, 1);
--nx-ease-x-enter:    cubic-bezier(0, 0, 0.3, 1);
--nx-ease-x-exit:     cubic-bezier(0.4, 0.14, 1, 1);
```

The values are **Carbon's published motion tokens** (R13 **V**); the stage may refine them by looking, and records why. **Exits are faster than entrances** (about 60 to 75%). Durations are used by token, never as literals; a test greps component CSS for `ms` literals.

### 8.3 The catalogue (what moves, how, how long)

| Element | Motion | Token | Notes |
|---|---|---|---|
| Hover / press of controls and rows | tone change only | 1 | No lift, no scale |
| Focus ring | **appears instantly** | none | Never animated |
| Menu, popover, tooltip | fade + 4 px move from the anchor | enter 3 / exit 2 | Origin at the trigger |
| Dialog and command palette | scrim fade, panel fade + scale .98→1 | enter 3–4 / exit 2 | Closing is faster |
| Context panel and mobile drawers | slide from the edge | enter 4 / exit 3 | Content does not animate inside |
| Rail collapse/expand | width, labels fade | 4 | Charts resize **after** the transition ends |
| Tabs, segmented controls | indicator slides | 3 | Panel swaps instantly |
| Route change | **View Transition** cross-fade of the main region only | 3 | Shell stays put; off under reduced motion (8.4) |
| Loading → content | skeleton fades to content | 2 | No layout shift (9.6) |
| Table sort / filter / paging | **none** | — | The most repeated actions |
| Row removed | collapse | 3 | Rows added by refresh do not animate |
| Toast | slide + fade in, fade out | enter 4 / exit 2 | 9.4 |
| Dashboard figures | **count-up once per session, first paint only** | ≤ 6 | Real value in the DOM from the start (visually hidden), animated copy `aria-hidden`; none under reduced motion |
| Chart first draw / update | draw-in / morph | 5 / 300 ms | Off above 1,000 marks; **the same marks keep their identity** (R21) |
| Cross-highlighting between views | opacity of others to 25% | 3 | Hover a node: its row, mark and timeline events respond |
| Graph layout change | positions interpolate | 5 | Never on mere selection |
| Map fly-to | camera eases | 5 | After user selection only |
| **Answer arrives** (expressive, once per question) | the answer region rises 8 px and fades in; the leading figure's background tint fades once | x-enter 4 + 6 | The one moment of presence; skipped under reduced motion |
| **Click a number → its source** (the product's signature) | the source row/span is scrolled into view and **tinted for 700 ms** | 6 | Shows where it came from; skipped under reduced motion (the row is still selected) |
| Processing | indeterminate bar loops; steps tick with a check | essential | Only essential moving element; has a text equivalent |

### 8.4 View Transitions (progressive enhancement)

Same-document view transitions reached Baseline "newly available" in October 2025 (R16 **S**; verify on the stage). Use them for **route changes of the main region** and for shared-element continuity where it clarifies (a case card to the Overview header) through React Router's `viewTransition` option on `Link`/`NavLink` (**K**, verify the exact prop in the installed version). Always feature-detect (`document.startViewTransition`), always fall back to no animation, never let a transition delay interaction: the new page is interactive during it. Cross-document transitions are not used (single-page app).

### 8.5 Reduced motion (MUST)

`@media (prefers-reduced-motion: reduce)` and a manual `data-motion="reduce"` switch (Appearance): all transforms, slides, scale, count-up, camera moves, graph interpolation and View Transitions are replaced by **instant changes or opacity fades ≤ 110 ms**; charts `animation: false`; the answer tint and source-row tint become a static outline; indeterminate progress stays (essential) but without parallax or sliding stripes. WCAG 2.3.3 (animation from interactions) is met by this switch even though it is AAA; anything that moves automatically for more than 5 seconds has a pause (2.2.2).

### 8.6 Performance rules for animation

Animate **`opacity` and `transform`** (and `clip-path` sparingly); never animate layout properties of large subtrees (the rail width is the exception, with `contain: layout`); `will-change` only while animating; `content-visibility: auto` for long off-screen regions; no animation that holds a frame over 16 ms on the reference laptop; the INP budget (12) applies during animations.

### 8.7 Forbidden

Bounce or elastic easing · parallax · looping decorative motion · animated gradients · staggered reveals of more than three items · typewriter or streaming text for answers (they are built whole) · hover lift on rows or cards · confetti or celebratory effects · scroll-jacking · auto-playing media · motion that repeats for attention.

---

## 9. Feedback, loading and states

*Research: R17 (NN/g response-time limits, skeleton screens, progress), R10 (error messages), R28 (empty states), R29 (toasts and live regions), R31 (file upload).*

### 9.1 The response-time contract

| Duration | Treatment |
|---|---|
| under 0.1 s | feels instant; no indicator |
| 0.1 to 1 s | no spinner; keep the previous content; the control shows it was pressed |
| 1 to 2 s | a subtle inline indicator on the part that is loading |
| 2 to 10 s | **skeleton that matches the final layout** (for a full region) or a spinner (for a small one) |
| over 10 s | a **progress indicator with real progress** if the service reports it; otherwise elapsed time and an honest description (9.2) |

(NN/g, R17 **V**: skeletons help for 2 to 10 s loads, are not for under 1 s, and give way to a progress bar over 10 s.)

### 9.2 Long answers and long processing

A question may take **10 to 120 seconds**; evidence processing minutes. Rules:
1. **Say what is happening in the analyst's words**, stage by stage, from the service's real step events when they exist; otherwise a single honest line ("Working on your question") plus **elapsed time** after 5 s. **Never a fake percentage.**
2. **Always offer Stop.** Stopping keeps the question and lets the analyst edit and re-ask.
3. **The page stays usable:** the composer remains active for the next question; navigation does not cancel; returning finds the answer.
4. **Announce** the start and the completion to assistive technology (polite live region): "Working on your question" … "Answer ready: <the sentence>".
5. **Beyond the expected time** (over about 120 s): "Taking longer than usual. Keep waiting / Stop." No error until the service says so.
6. If the tab is hidden, the document title carries the state ("Answer ready · NexusAI"); a system notification only if the analyst opted in.

### 9.3 Empty states: what, why, next

Every empty state has an **icon, a title that says what is absent, one sentence on why or what it means, and a next action**. Kinds: *first use* (teach the first step), *nothing found* (state what was searched, offer to widen; never blame), *all clear* ("Nothing needs you", with the time of the last check), *not available yet* (the data is not in the service; what is possible now), *not permitted*. **An empty chart keeps its frame and height** and shows the message inside it (no layout jump).

### 9.4 Toasts, banners, dialogs

- **Toast** (`role="status"`, polite): only a **confirmation of a completed action that needs no follow-up** ("Evidence added: 3 files"), 5 to 8 s, pausable on hover and focus, dismissible, **one at a time**.
- **Errors and anything actionable never live in a toast.** Inline next to the thing, or in a persistent alert region; `role="alert"` only for blocking errors that need immediate attention. A failed long job goes to **Activity** and stays until read.
- **Banners** for page-level conditions (service slow, working offline, truncated result); dismissible only when the condition is informational.
- **Confirmation dialogs** only for **irreversible** actions, naming the consequence and using the verb as the button ("Exclude evidence", not "OK"), safe action focused first. Reversible actions use **Undo** instead.

### 9.5 Forms and validation

Labels always visible (placeholders never replace labels); required fields marked in words; validate **on blur and on submit**, not per keystroke; inline messages tied with `aria-describedby`; on submit failure focus the first error and show a summary with links; **preserve input** on error; never ask for the same information twice (WCAG 3.3.7); `autocomplete` attributes set; no CAPTCHAs (3.3.8); keyboard submit with `Enter`.

### 9.6 Layout stability

No layout shift: skeletons have the final dimensions; charts and media have fixed aspect ratios or heights; fonts swap with metric-compatible fallbacks (`size-adjust`); the page title row does not change height between loading and loaded; banners are inserted without pushing content when possible (reserved slot).

### 9.7 Saving and optimistic updates

Reversible local actions (rename a case, add a note) update immediately and show **"Saved" with the time**; a failure shows an inline retry and keeps the unsaved text. Irreversible or server-computed actions wait for the service.

### 9.8 Connectivity

A single banner ("Can't reach the service. Showing what was loaded at 14:32. Retry"), exponential back-off retry in the background, previously loaded data stays visible and is marked stale; actions that need the service are disabled with the reason.

---

## 10. Content and language

### 10.1 Voice

Plain, direct, calm; **sentence case**; active verbs; numerals; no exclamation marks, no jokes, no "oops"; no blame ("The file could not be read", never "You uploaded a bad file"); **no analyst-facing jargon** (rule 6).

### 10.2 Terms (one list, used everywhere)

| Say | Never say |
|---|---|
| case, evidence, answer, source row, question | project, dataset, query, ingest, payload |
| Reading, Understanding, Ready, Needs review | ETL, pipeline, processor, stage 3 of 7 |
| how it was worked out | SQL, plan, operators, agent, model, prompt |
| Observed, Inferred, Candidate | confidence score, probability, certainty % |
| Add evidence | Upload dataset, ingest files |
| Ask this case | Run query |

(Administrators may open "Show technical detail", one level down; the words above are the analyst's.)

### 10.3 Error messages (NN/g pattern, R10 **V**)

*What happened* → *why, if known* → *what to do*, with a **next action as a button**. Visible, near the cause, polite, constructive, specific. Example: "**Could not read report.pdf.** The file is password-protected. Add an unlocked copy, or exclude this file." Never a code, a stack or a blame sentence; an error id may be copied for support.

### 10.4 Numbers, dates, units, identifiers

`Intl` formatting with a fixed locale for evidence (so a number reads the same on every screen); thousands separators; **tabular figures**; units attached (`6.29 km`, `22 min`, `2 h 14 min`); ranges with an en dash; real minus sign. **Dates and times: `5 Mar 2026, 14:32 PKT` (the case's zone, always shown),** with UTC available on hover and copy; relative times ("3 h ago") only in activity lists and always with the absolute time as the title. **Phone numbers** in international form grouped (`+92 300 1234567`), **IMEI** grouped, **IPs, hashes and plates** monospace, `dir="ltr"`, with a copy button; long hashes are shortened in the middle with the full value copyable.

### 10.5 Script and direction

All user-originated text that may be Urdu (or another right-to-left script) is rendered with `dir="auto"` or in `<bdi>`; identifiers are always `dir="ltr"`; logical CSS properties everywhere; **sentences are never assembled from fragments** (word order differs across languages), they are whole templates; `lang` set on the document and on Urdu spans. The interface stays **English** (an Urdu UI is a separate programme; strings stay in components until the owner asks, then move to a catalogue in one pass).

### 10.6 Microcopy standards

Loading text names the task ("Reading your question…", not "Loading…"); success states the result ("3 files added"); buttons are *verb + object* ("Add evidence", "Ask this case", "Copy as CSV"); navigation items are nouns; **tooltips never hold the only copy** and appear on focus as well as hover (WCAG 1.4.13: dismissible, hoverable, persistent).

---

## 11. Accessibility

*Research: R30 (WCAG 2.2 new criteria), R26 (WAI-ARIA Authoring Practices), R27 (Chartability), R35 (WCAG 3 status), R39 (forced colours, contrast preferences), R41 (automated testing).*

### 11.1 The standard

**WCAG 2.2 level AA, in both themes, at all three densities, at 200% zoom and at 320 CSS px reflow.** WCAG 3.0 is a Working Draft (R35 **S**) and is *not* a target; APCA values may be recorded as information. Some AAA practices are adopted where cheap: focus appearance (2.4.13), animation from interactions (2.3.3), no timing.

### 11.2 Mapping of the criteria that most often fail in this kind of product

| Criterion | How this directive meets it |
|---|---|
| 1.1.1 Non-text content | Charts, graphs, maps: text summary + **table twin** (7.6) |
| 1.3.1 / 4.1.2 Structure, name, role, value | Native elements first; APG patterns for the rest; landmarks (5.2) |
| 1.4.1 Use of colour | Strength, status, series: words + shape/pattern as well as colour |
| 1.4.3 / 1.4.11 Contrast | Token pairs tested (4.3); marks and focus 3:1 |
| 1.4.4 / 1.4.10 / 1.4.12 Resize, reflow, text spacing | Rem units; reflow to 320 px; no clipped text with user spacing |
| 1.4.13 Content on hover or focus | Tooltips dismissible, hoverable, persistent |
| 2.1.1 / 2.1.2 / 2.1.4 Keyboard | Everything reachable; no traps; **single-key shortcuts only when a list has focus**, modifiers otherwise |
| 2.2.2 Pause, stop, hide | Nothing moves automatically > 5 s except essential progress |
| 2.4.1 / 2.4.3 / 2.4.7 Bypass, order, focus visible | Skip links, logical order, restyled focus ring |
| **2.4.11** Focus not obscured (min.) | `scroll-padding-top` = header height; sticky regions measured |
| 2.4.13 Focus appearance (AAA, adopted) | 2 px ring, 2 px offset, 3:1 |
| **2.5.7** Dragging movements | Every drag has a click/keyboard equivalent (splitters, brushing, node pinning) |
| **2.5.8** Target size (min.) | ≥ 24 px by rule; desktop controls ≥ 28 px; touch ≥ 44 px |
| **3.2.6** Consistent help | One help/shortcut entry in the same place on every page |
| **3.3.7** Redundant entry | Never ask twice; carry values forward |
| **3.3.8** Accessible authentication (min.) | When login arrives: no cognitive tests, paste allowed, password managers work |
| 4.1.3 Status messages | Live regions for answers, job completion, counts, toasts |

(The 2.2 additions are as listed on W3C's "What's new in WCAG 2.2", R30 **V**.)

### 11.3 Patterns

Use the **WAI-ARIA Authoring Practices** pattern for each composite widget (tabs, combobox, listbox, menu button, dialog, disclosure, toolbar, tree/grid, window splitter, alert); native HTML (`button`, `dialog`, `details`, `select`, `input`) wherever it suffices; complex widgets from a tested library if the owner approves (13). Never invent a new keyboard model for a known pattern.

### 11.4 Keyboard model

Global: `⌘/Ctrl+K` palette, `?` shortcut sheet, `g` then a letter to go to a page (**only when no field has focus**, and can be switched off in Settings, 2.1.4), `/` focus search, `Esc` close/clear. Lists and tables: `↑/↓` move, `Enter` open, `Space` select. Ask: `Enter` send, `Shift+Enter` newline, `Esc` stop. Every shortcut is listed once, in one table that generates the sheet, the palette hints and the Settings page.

### 11.5 Screen readers

Accessible names for every control and region; counts and status announced (not only drawn); `aria-busy` while loading; **the answer sentence is announced when ready**; charts are described and have a table; map marks are listed; table semantics are real (`aria-sort`, `aria-rowcount` when virtualized); headings form an outline; live regions are quiet and never repeat.

### 11.6 Chartability as the acceptance workbook for visuals

**Chartability** (R27 **V**) is a set of testable heuristics for accessible data visualisation. Each visualisation stage runs it on every chart type it touches and records pass/fail with the fix. A visual that fails the *perceivable* or *operable* checks is not merged.

### 11.7 Colour modes

`prefers-contrast: more` raises border and text tokens; `forced-colors: active` uses system colours (`CanvasText`, `Canvas`, `Highlight`, `ButtonText`) for borders, focus, selection, with `forced-color-adjust` only where a colour *is* the data (and then patterns/shapes carry it); `prefers-color-scheme` follows the system; the manual choice wins. All three are screenshot-tested in the gallery.

### 11.8 Testing and evidence per stage

1. **Automated:** an `axe` pass per route × theme × density in Playwright (PROPOSED dev-only dependency `@axe-core/playwright`); contrast table test over all token pairs; a lint for raw colours/`ms` literals/`!important`.
2. **Keyboard walk:** a scripted Tab/Shift+Tab/arrow/Esc walk of each page's main flow (Playwright), plus a manual one.
3. **Screen reader:** a manual pass of the main flow with **NVDA + Chrome** (and VoiceOver on a Mac if available), recorded as notes in the stage doc. Automated tools find only part of the issues (**K**: commonly cited as roughly a third); the manual pass is mandatory, not optional.
4. **Zoom and reflow:** 200% and 320 px at both themes, recorded as screenshots (synthetic data).
5. **Chartability** for visuals (11.6).


---

## 12. Performance budgets

*Research: R33 (web.dev INP), R25 (ECharts large data), R12 (virtualization). The numbers marked PROPOSED are set from a baseline measured in U1 and may only go down.*

| Budget | Target | Source / status |
|---|---|---|
| **INP** (interaction latency, 75th percentile) | **≤ 200 ms** | web.dev "good" threshold (R33 **V**) |
| LCP / CLS | ≤ 2.5 s / ≤ 0.1 | Standard Core Web Vitals "good" values (**K**) |
| Command palette open; tab or pane switch | ≤ 100 ms; ≤ 150 ms | PROPOSED |
| Sort or filter of 10,000 table rows | ≤ 100 ms to first paint of the result | PROPOSED (virtualized, derived data memoised) |
| Chart update on filter | ≤ 50 ms main thread, ≤ 300 ms to settled | PROPOSED |
| Workbench first view | **≤ 2 s** after the reads return; 60 frames a second to **5,000 nodes and 50,000 points** on the reference laptop | Master plan 7.3 (owner-accepted) |
| Initial JavaScript (shell + dashboard) | **≤ 200 KB gzip**, with ECharts, Cytoscape and MapLibre loaded **only** on the routes that draw with them | PROPOSED, baseline in U1 |
| CSS | **≤ 30 KB gzip per route chunk**, ≤ 150 KB raw total | 4.8 |
| Fonts | Latin UI face subset ≤ 100 KB; Urdu faces load only when a string needs them | PROPOSED |
| Layout shift on route change | none (9.6) | |

**Mechanics (MUST).** Routes are code-split (exists); heavy libraries are imported per use and ECharts is **tree-shaken** (import only the chart types and components in use); long lists are virtualized with correct ARIA; derived data is memoised at the hook; filters use `startTransition`/`useDeferredValue` so typing never waits on a re-render; images and tiles are cached; no layout reads inside scroll handlers; `content-visibility: auto` for off-screen regions.

**Measurement.** Every stage records, in its note, a Playwright trace or the Performance panel result for its two or three heaviest interactions **on the synthetic wave-1 case**, on the reference laptop (15.7 GB RAM, no discrete GPU), in a production build. A regression against the baseline blocks the stage.

---

## 13. Components and dependencies

### 13.1 Component policy (MUST)

1. **Shared components live in one place** (`src/ui/` for primitives, `src/patterns/` for compositions); a page imports from there and never styles them. A new shared component arrives **with** its gallery page (every state: default, hover, focus, active, disabled, loading, empty, error, selected; light, dark, high contrast; compact, default, comfortable; left-to-right with an Urdu string), its tests (render, keyboard, a11y name/role), and its entry in `/design/components`.
2. **Presentational components do not fetch;** data hooks stay in `src/lib/` and pages. The existing hooks and their tests are frozen inputs (rule 9).
3. **No one-off components inside pages** unless used once and under 80 lines; when a second page needs it, it moves.
4. **Props express meaning** (`strength="inferred"`, `status="failed"`), never visual detail (`color="red"`).
5. **Native first:** `button`, `a`, `dialog`, `details`, `select`, `input`, `table`, popover attribute (R15 **S**), before any widget library.

### 13.2 Dependency proposals (each needs the owner's yes, rule 10; the trade-off is stated so the owner can decide)

| # | Proposal | Why | Cost / risk | Recommendation |
|---|---|---|---|---|
| **P-1** | **React Aria Components** (Adobe) for complex widgets: combobox/command palette, listbox, menu, dialog/popover, tabs, tooltip, date and number fields | Accessibility behaviours (keyboard, focus, screen-reader, **internationalisation including right-to-left**) are built and tested; unstyled, so the tokens style them (R34 **S**) | A dependency (~tree-shakeable), a learning cost, API churn | **Yes, for the six composite widgets only**; native elsewhere. The alternative, Radix Primitives, is simpler to style and popular; React Aria is stronger on i18n/RTL, which this product needs for Urdu (**S**, verify on the stage) |
| **P-2** | **TanStack Table** (headless) | Column state (sort, visibility, resizing, pinning) without writing it | Another dependency; the existing `VirtualizedTable.jsx` already virtualizes | **Only if** the table stage shows the existing component cannot meet the table rules (6.2) |
| **P-3** | **`@axe-core/playwright`** (dev only) | Automated accessibility checks on every route × theme × density | Dev dependency; finds only part of the issues | **Yes** |
| **P-4** | **`culori`** (dev only) | OKLCH generation and contrast checks in the token generator and its tests | Dev dependency | **Yes** (the alternative is hand-written colour maths, error-prone) |
| **P-5** | **`stylelint`** (dev only) with a small config | Enforces "no raw colours, no `!important`, no `ms` literals, logical properties" mechanically | Dev dependency; some config | **Yes** |
| **P-6** | **A Nastaliq webfont** (Noto Nastaliq Urdu through fontsource, OFL) | Evidence text in the script analysts expect (decision O-6) | Large font (loaded only when needed) | **Only after O-6** |
| **P-7** | **sigma.js** (WebGL graph) | Beyond ~500 nodes without the working-set rule | Second graph renderer to maintain | **No**, until a real case needs it |
| **P-8** | Storybook, Style Dictionary, a CSS-in-JS library, a full UI kit (MUI, Ant, Chakra, shadcn registry) | — | Weight, duplication of the gallery and generator, a second styling system | **No.** The `/design/*` gallery, the 100-line token generator and plain CSS layers are the system |

---

## 14. Verification and the definition of done

### 14.1 Every stage must pass (merged from the master guide and the contract)

1. `npm --prefix apps/investigation-workspace run test` green (all existing tests; new ones for every state, mapping and count the stage adds). **A test is never weakened to pass**; a test that is wrong is fixed with the reason written in the commit.
2. `vite build` succeeds; **size budgets** (12) checked by a script; no new warnings.
3. **Token tests:** every used token pair passes the contrast table (4.3); no raw colour, `!important`, ID selector or duration literal in component CSS (stylelint, P-5); CSS budget.
4. **Accessibility:** `axe` per route × theme × density (P-3); scripted keyboard walk; the manual NVDA pass recorded; Chartability for every visual (11.6); `prefers-reduced-motion`, `forced-colors` and `prefers-contrast` screenshots.
5. **Visual evidence** (synthetic data only), saved under `docs/ux/evidence/<stage>/`: each changed page at **1440 and 390 wide, light and dark** (+ the high-contrast theme for the shell), each state of each new component (the gallery), before/after for replaced pages.
6. **Playwright slices** relevant to the stage pass; a **visual-regression baseline** (`toHaveScreenshot`) exists for the gallery routes and is updated deliberately, never by default.
7. **Performance:** the stage's two or three heaviest interactions measured (12).
8. **Truth:** every figure on the changed pages equals the service's figure on the synthetic case (a stated spot-check of at least five numbers, recorded); every "not available yet" state has its `BACKEND_REQUESTS.md` row.
9. **Docs:** the stage note (research, options, decision, what was built, what was removed, proof, open items), `NEXUSAI_CONTINUATION.md` updated (UI owner, next action), the gallery page.
10. **Commit hygiene:** trailers per `AGENTS.md`; explicit paths; one stage = a small series of coherent commits on its branch; **no `Co-Authored-By` for an AI**; never `--no-verify`.

### 14.2 What the analyst-facing acceptance looks like

The **owner accepts a stage by looking at it** on the running application (the stage's branch served on its own port, decision O-2), with a **demonstration script** written in the stage note: the exact clicks and keys, the expected outcome, run on a generated case (master plan principle 8). No stage is "done" because its tests pass.

---

## 15. The per-stage loop (binding, merged from the master plan 7.2, the Stitch plan and the dark directive's six steps)

| Step | What | Output | Who decides |
|---|---|---|---|
| **1. Research** | Search for the best relevant, current examples, standards and implementations **for this stage's own areas** (every area, however small: the rail, the empty state, the table header, the legend). Read primary sources first. Add rows to the register (topic, finding, URL, **V/S/K**). Record **what was rejected and why** | A research note (2 to 4 pages) | — |
| **2. Options** | Build **two or three** alternatives **in the gallery** (real components, synthetic data, light and dark, two widths), not images from another tool | `/design/<stage>` pages | — |
| **3. Decide** | The owner chooses by looking; the choice and the reason go in a decision row (`D-U<n>`) | Decision log | **Owner** |
| **4. Build** | Over the existing, tested hooks; delete the CSS and markup it replaces in the same stage; no half-migrated pages | Commits | — |
| **5. Prove** | Section 14 | Evidence folder | — |
| **6. Close** | Stage note, continuation file, gallery, backend requests; the old file list shrinks | Commit + handoff | — |
| **7. Review** | The owner uses it on the stage's port; findings become a short list; fix, then the next stage | Acceptance | **Owner** |

**Stop and ask** when: a rule here conflicts with a real constraint; a design needs a backend that does not exist (file a request, show a designed state, continue); a dependency is needed; a test would need weakening; real case data would be required to look right (never; use the generated cases); a decision in section 18 is still open and the stage depends on it.

**The research duty is real.** The owner asked that *each and every thing* be researched for the best examples before it is built. In practice: no area is built from memory. Where a stage note has no source for an area, the area is not finished. The register of this directive is the floor; stage research extends it.

---

## 16. The stages (aligned with the master plan's U0 to U10)

*The master plan (`docs/roadmap/master-plan-20261021.md`, section 7.4) fixed the stage list and gates; this section keeps its numbering and adds this directive's content. **Visualisation (7), motion (8), feedback (9), content (10) and accessibility (11) are not stages: every stage applies them to its own pages, and U10 sweeps what is left.** U0 (the direction proof) was delivered on 2026-10-21 (D23: structure and layout accepted; colour is reopened here, 4.3).*

| Stage | Name | Depends on | Gate (the owner looks) | Weeks |
|---|---|---|---|---|
| **U1** | **Foundations** (and the colour direction of 4.3) | decision O-1, O-2 | `/design/foundations`: both directions, both themes, three screens, contrast tables; the owner picks A or B | 2 |
| **U2** | **Shell and navigation** | U1 | keyboard path, four widths, three themes | 2 |
| **U3** | **The workbench restyle** (the v1 is built and live) | U1, U2 | the same walkthrough, now in the new system, plus the 6.6 additions that need no new backend | 1.5 |
| **U4** | **Ask** | U1, U2 | the answer anatomy, every state, the core loop by keyboard | 3 |
| **U5** | **Dashboard (hero), Cases, New case, Case overview** | U1, U2, BR-1 (honest states until it lands) | no fabricated numbers; every tile opens its rows | 3 |
| **U6** | **Evidence and viewers** | U1, U2 | unaided add-data and open-a-citation | 3 |
| **U7** | **Entity drawer, saved views, notes** | U3, BR-3 | "Ask about this" from any chip | 2 |
| **U8** | **Activity, Settings, Admin, Reports** | BR-4 | honest states; roles | 2 |
| **U9** | **Media workbench and review desk** | P3 tracks | review with reason and undo | with P3 |
| **U10** | **Polish, sweep and the usability test** | all | the contract's 13.1; alias layer deleted; classic Connections deleted | 2 |

### U1 Foundations

- **Goal:** one token system, one CSS architecture, two colour directions to choose from, and the gallery to prove it. **Nothing else changes yet.**
- **Reads:** 1 to 4, 7.3, 7.4, 8.2, 11.1, 11.7, 12, 13.
- **Delivers:** `design/tokens.json` (DTCG) + generator + `tokens.generated.css`; `styles/index.css` with the layer order; `base.css`; the `nx` ECharts theme reading tokens; motion and density tokens; `DESIGN.md` (a plain statement of the system for agents, with a mapping of contrast checks to custom properties; R38 **S**); the **alias layer** that maps every `--analyst-*` name to `--nx-*` so the old pages keep working; `/design/foundations` with: palette ramps, surfaces, text, status, strength, data, scales, type specimen (Latin, Urdu, tabular, mono), spacing/radius/depth, density, icons, motion samples with reduced-motion switch, the contrast table (generated), the colour-blind simulation, and **the three screens in Directions A and B**; `stylelint` config; the CSS-budget and no-raw-colour tests; baseline measurements (bundle, CSS, INP of three interactions).
- **Retires:** nothing is deleted in U1 (the alias layer keeps the old pages alive); `palette-options.css` and `PaletteOptions.jsx` are deleted once the owner has chosen.
- **Owner decides:** direction A or B (4.3); density default; brand mark (O-4); Urdu face (O-6).

### U2 Shell and navigation

- **Goal:** header, rail, case band, command palette, activity popover, appearance menu, states pages: section 5 complete.
- **Reads:** 5, 8.3 (menus, drawers, route change), 9.4, 11.4.
- **Delivers:** the shell in the new system; skip links, landmarks, focus management on route change, document titles; the command palette (ARIA combobox) over `WorkspaceCommands`; shortcut sheet; responsive behaviour 5.7; the global states (offline, 404, access, error boundary); `/design/components` entries for every shell part.
- **Retires:** `shell.css`, the shell parts of `identity-a.css` and `workspace.css`, `BrandLockup` markup replaced by the chosen mark.
- **Proof:** keyboard path through the whole shell; four widths; three themes; axe; the NVDA pass of "go to a case, open Ask".

### U3 The workbench restyle

- **Goal:** the layout-B workbench moves to the foundations and gains the research items of 6.6 that need no new backend: table twin and marks listbox, time-bar brushing as filter with reset chip, level-of-detail labels, hidden-count banner, uncertainty circles for cell sites, muted basemap, stable layout seed.
- **Reads:** 6.6, 7.8 to 7.10, 8.3 (cross-highlight, layout transitions), 11.5, 11.6.
- **Retires:** `workbench.css`'s private tokens; the `/connections/classic` route and `ConnectionsPage` after the owner accepts (O-9).
- **Keeps:** every file under `src/lib/workbench/` and its tests; `WorkbenchCore`, `WorkbenchResult`, `WbGraph`, `WbTimeline`, `WbDrawer`, `WbTable`, `WbStates` change in markup and CSS only unless a research item needs a logic change, which gets its own tests.

### U4 Ask

- **Goal:** the core screen: the fixed answer anatomy of 6.5, every designed state, honest waiting, and the workbench inside answers.
- **Reads:** 6.5, 9.1 to 9.4, 10, 8.3 (answer arrival, source-row tint), 11.5.
- **Delivers:** composer with scope line and suggestions from the real case; answer anatomy; **every withheld-answer variant by `reason_code` as a calm card (never an error, never a number)**, with the system's sentence verbatim and the options as one-click re-runs; source records and model observations visibly different (badge); the waiting state (9.2); the question trail; keyboard loop; "this answer is wrong" only when the triage endpoint exists (BACKEND_REQUESTS row).
- **Keeps:** `analystPrimaryAnswer`, `analystAskPresentation`, `investigationPresentation`, `readAnswer`; **the engine's wording is not changed by the interface** (the wording of absence sentences is the Ask-engine programme, Step 1 of the owner's list).
- **Retires:** `InvestigationResult` markup and the Ask start's data panel and starters (the master plan 7.3: quality moves to Overview/Evidence).

### U5 Dashboard (with the hero), Cases, New case, Case overview

- **Goal:** sections 6.1 to 6.3 exactly. The hero band and the attention queue are the owner's explicit "hero section" request.
- **Reads:** 6.1 to 6.3, 7.2 (what to draw), 7.7 (metric cards), 9.3.
- **Backend:** reads from BR-1 (case record, census, health, summary, attention feed, directory) arrive in P1; until then the page uses the existing client-side aggregation **and says so**, and designed "not available yet" states mark each missing read.
- **Retires:** `PipelineFlow`, bubble pack and `ReadinessRing` unless the stage note shows the question each answers; `CasesTable` replaced by the shared table; the dashboard parts of `workspace.css`.

### U6 Evidence and viewers

- **Goal:** 6.4: the add-evidence region with the file-upload pattern, the evidence table, and the viewers (document, image, audio/video with transcript, records).
- **Reads:** 6.4, 9.2 (processing), 9.3, 11.5, the media-accessibility rules (no auto-play; captions/transcripts).
- **Keeps:** the viewers' logic, `viewerPresentation`, `viewerStrength`, `viewerTools` and their tests.

### U7 to U9

U7 (entity drawer, saved views, notes) waits for BR-3; U8 (activity, settings, admin, reports) for BR-4; U9 (media workbench, review desk) for the media tracks. Each follows the loop; each card is written when its backend arrives.

### U10 Polish, sweep and the usability test

- The full motion and empty-state sweep, the accessibility and right-to-left audit across **every** page, the `/design/charts` gallery complete, **the alias layer deleted**, `/connections/classic` and every legacy CSS file deleted, performance re-measured against the U1 baseline, and **an unaided core-loop test with real analysts** (task completion and error rates written first; names owed, D26). The usability test is the only acceptance that can say "the analyst finds it easy".

---

## 17. The merge ledger: every earlier directive, and what became of it

*Each document below is **superseded as a working instruction** by this one. Its content is not lost: the table says what was kept (and where it now lives) and what was rejected (and why). Codex reads only this directive, the contract and `AGENTS.md`/`.agents/CODEX.md`; it consults the old documents only for the specific facts named here.*

| Document | Kept (where now) | Rejected or changed (why) |
|---|---|---|
| **`docs/ux/NEXUSAI_PRODUCT_UX.md`** (the contract) | **Binds.** Information architecture, provenance, claim-level citations, state list, clarification surface, 13.1 usability test | §8 (visual system) and §11 (responsive) **proposed amendments** in 4.9, effective on O-1 |
| `docs/work/UIUX_RESEARCH_20260927.md` (Phase 0 research) | Empty / partial / processing / failed / unavailable are different facts (9.3); dense shell; accessible tables (6.2); provenance (rule 3) | Stage sequence (replaced by 16); the measured defects of 2026-09-27 are historical: a stage re-checks any it touches and records the verdict |
| `docs/work/CODEX_UIUX_BRIEF_20260927.md` | "Build on what exists", scope `apps/investigation-workspace/**` only (rules 8, 9) | Ownership statement (the plan's D24 governs); its defect list is historical |
| `docs/work/CODEX_UI_DEFECTS_20260928.md` | Two regression ideas (a document answer must not read "no verified answer"; a correct response must not render wrongly), kept as U4 tests if still reproducible | — |
| `docs/work/CODEX_UI_ELEVATION_BRIEF_20260928.md` | **Withheld answers are a designed state per `reason_code`** (U4); source record vs model observation badge; honest latency (9.2); "does not exist, do not simulate" (rule 1; updated: a case record exists since 2026-10-21 with `GET/PATCH /cases`, while users, auth, sessions and cross-case aggregation do not); the data each page may truthfully show (re-verify against the live API in each stage) | Its "first slice" design-system upgrade (replaced by U1); quality bar prose (replaced by section 1) |
| `docs/work/CODEX_UI_REDESIGN_PROMPT_20260929.md` | Research first (15), page-by-page order of thought, responsive and accessible every page (4.6, 11), verification per slice (14) | "Too plain" verdict resolved by a calm instrument, not decoration (4.1); page order replaced by 16 |
| `docs/work/UI_REDESIGN_BRIEF_20260929.md` (v2) | §4 rules (section 2), §5 method (15), §6 "every visual answers a named question" (7.1, 7.2), §8 definition of done (14), **§11 metric-card contract, copy budget and interaction language (7.7, 10, 8)** | §7 "derive colour from the header" (**reversed:** the chrome is neutral, 4.3); "Never" list kept in section 2 |
| `docs/work/UI_REDESIGN_RESEARCH_20260929.md` | Principles: progressive density, **context survives every transition**, visuals are navigation not decoration, provenance beside the claim, fast by keyboard / clear by touch, richness through layers (sections 1, 5, 6, 7) | "Mineral Signal" palette (reopened by the owner; 4.3 offers two directions); its shell and dashboard rulings (replaced by 5 and 6.1) |
| `docs/work/UI_REDESIGN_MASTER_GUIDE.md` (S0 to S12) | Per-stage protocol (15), definition of done (14), "keep hooks" (rule 9), token vocabulary (renamed `--nx-*` behind an alias layer) | The S-stage list (replaced by U1 to U10); its progress tracker is the history of what was built |
| **`docs/work/UI_UX_MASTER_REDESIGN_PROMPT.md`** (the "dark directive") | Ontology nouns over tables (people, communications, geospatial, provenance; section 1); **multi-modal synchronisation** (selection in one view is selection in all: 6.6, 8.3); **pivot-and-probe** (the context panel and drawers keep the analyst in place: 5.1); provenance one step from every metric (rule 3); real data only; backend gaps logged (in `BACKEND_REQUESTS.md`); the six-step workflow (15); one section at a time | **Near-black bevelled canvas, inner highlights, "machined" look** (contract 8.3 and ruling C8: tonal surfaces without bevels, 4.3, 4.5); **hover lift `translateY(-2px)` and press `scale(0.97)`** (forbidden, 8.7: tone change only); the single `cubic-bezier(0.16, 1, 0.3, 1)` at 140 to 180 ms (replaced by the token ladder, 8.2); negative letter-spacing on hero figures (tabular figures keep default tracking); **Sankey pipeline** (7.2: only for a real flow); **evidence-health radar** (7.1: no radar charts); Leaflet (the app uses MapLibre); **dual-directory sync** (obsolete: one canonical tree since P0); `BACKEND_ROADMAP_GAPS.md` (the ledger is `BACKEND_REQUESTS.md`); Inter-specific OpenType flags (Noto is the face) |
| `docs/work/UI_PAGE_SPECS/*` (shell, dashboard, canvas, dashboard architecture, product plan, audit) | The data mappings and hook names, where the hooks are unchanged (input to U2, U5) | Layouts (replaced by 5 and 6) |
| `docs/work/WI-UI-7/8/9_CODEX_PROMPT.md` | Their **accepted work is the code to keep**: the three-band shell logic, claim-level citations, governed palette, scope chips, route states, the viewers, the virtualised table, the command palette, the question history | The prompts themselves |
| `docs/work/WI-NL2SQL-UX_MASTER_PROMPT_20260927.md`, `CODEX_BACKLOG_20260927.md`, `WI-LAYER-*` | Not interface directives (engine and data); Codex does not act on them | — |
| `docs/ux/redesign-2026-10/README.md`, `STITCH_PRODUCT_BRIEF.md` (Stitch plan) | One stage at a time; the owner decides by looking; **synthetic content only** in any outside tool; metric-card contract and copy budget (merged); decisions 1 to 6 (re-posed as O-decisions) | **Stitch-first method** (replaced by gallery options built in code, 15); Stitch may still be used for ideation with synthetic content, the output a reference never shipped |
| `docs/ux/workbench/RESEARCH_NOTE.md` | Input to U3 (working set, coordinated views, evidence one click away) | — |
| `docs/roadmap/master-plan-20261021.md` §5 (C1 to C12), §7 | **In force**: one programme, one owner per stage, design first, keep what works, honest data, contract binds; the stage numbering and gates | — |
| `AGENTS.md`, `.agents/CODEX.md` | **Bind** (operating rules, commit trailers, scopes, safety) | — |

---

## 18. Sources and open decisions

### 18.1 Sources

The full list, with the finding taken from each and its V/S/K mark, is [`research/RESEARCH_REGISTER_20261026.md`](research/RESEARCH_REGISTER_20261026.md) (R1 to R41). **Stage research adds rows; it never deletes one.**

### 18.2 Decisions owed by the owner (nothing waits on all of them)

| # | Decision | Needed by | Recommendation |
|---|---|---|---|
| **O-1** | Accept the proposed amendments to contract §8 and §11 (4.9) | before U1 builds anything that differs | Yes: they are the redesign the owner asked for; the contract's forbidden looks stay |
| **O-2** | **How Codex works.** (a) In the lane tree `NexusAI-localwork`, path-scoped commits to `apps/investigation-workspace/**` and `docs/ux/**`; or (b) **in its own linked git worktree on its own branch** (for example `ui/u1-foundations`), served by its own dev server on **:4182** (same API proxy), merged into `lane-arms` when the owner accepts the stage, :4181 following | before U1 | **(b).** Two agents committing in one working tree contend for the index and the commit hook; a separate worktree is still the one repository and one history, and the owner compares :4182 with :4181 side by side. Claude stays out of UI paths while a stage is open (rule 8). **Practical catch:** the dev server attaches the API key server-side from the gitignored `.env.forensic-runtime.local` (or the process environment); `git worktree add` does not copy it, and Codex must never read, print or commit it, so the owner places the file in the new worktree or starts the :4182 server |
| **O-3** | Colour direction A or B (4.3) | U1 gate | by looking |
| **O-4** | Brand mark: keep the code-native mark or evolve it | U1 | evolve lightly to match the chosen palette |
| **O-5** | Density defaults: compact tables (recorded 2026-10-12) and default pages | U1 | confirm |
| **O-6** | Urdu: Nastaliq for evidence text or a Naskh/Sans Arabic face; needs a native reader | before U6 | Nastaliq for evidence text, Sans Arabic for labels; the named reviewers (D10) check |
| **O-7** | Dependencies P-1 to P-6 (13.2), each yes or no | before the stage that needs it | P-1 for six widgets, P-3, P-4, P-5 yes; P-2, P-6, P-7, P-8 no for now |
| **O-8** | The offline map tile source: its licence and the attribution it requires (**unverified**) | before any screenshot leaves the machine | verify the source and record it in the register |
| **O-9** | Delete `/connections/classic` and `ConnectionsPage` once the workbench is accepted | U3 close | yes, at acceptance |
| **O-10** | The case time zone shown with every time (PKT assumed) and whether a per-case setting is wanted | U4 | per-case setting later; PKT default now |
| **O-11** | Urdu interface (strings in a catalogue) | not now | defer |
| **O-12** | Named analysts for the unaided usability test (D26) | U10 | owner to name |

### 18.3 How this directive is kept alive

`NEXUSAI_CONTINUATION.md` carries the current stage, its owner and its port. A change to this document is a commit with a reason; a rule is changed by the owner's decision, recorded in the decision log (`D-U<n>`), and the directive is edited the same day. **When this directive and a stage note differ, the later, owner-approved text wins.**
