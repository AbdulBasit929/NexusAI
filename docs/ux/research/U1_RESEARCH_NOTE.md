# U1 foundations research note

**Stage:** U1 Foundations · **Date:** 2026-10-28 · **Status:** research complete; options not yet decided · **Scope:** colour, type, spacing, depth, motion, density, CSS architecture, iconography, accessibility and measurement

## Question and starting point

U1 has one job: give NexusAI a stable visual grammar without changing the working application. The grammar has to support a dense forensic instrument used for hours, in light, dark and high-contrast modes, with English, Urdu, mixed-direction identifiers, tables, charts, maps and evidence-strength states. It also has to let the existing pages continue to render while later stages replace their markup and CSS.

The current stylesheet inventory makes that migration constraint concrete: eight CSS files total **618,453 raw bytes**, with **150 distinct `--analyst-*` custom properties** referenced 6,485 times. The styles have no cascade layers and contain three generations of visual rules. U1 therefore cannot be a palette swap. It needs one generated token source, an explicit cascade order, an exhaustive legacy alias layer and automated checks that prevent a fourth generation from appearing.

## Findings and U1 decisions

| Area | Evidence read | Finding | U1 application |
|---|---|---|---|
| Token format | DTCG Format and Color Modules 2025.10 (R45, **V**) | The first stable Community Group report defines typed `$value` tokens, inherited `$type`, descriptions, complete-token aliases, JSON Pointer references and OKLCH values. It is stable for implementation but is not a W3C Recommendation. | `design/tokens.json` uses the 2025.10 schema and typed tokens. Primitive values are explicit; semantic roles alias primitives. The generator validates missing types, broken aliases and cycles rather than guessing. Generated CSS is committed and is never hand-edited. |
| Colour ramps | DTCG Color Module; Radix scale; Linear 2024 and 2026 redesign reports (R45–R47, **V**) | OKLCH is appropriate for authored ramps; a 12-step scale works when every step has a job; neutral chrome lets working content dominate. Linear's 2026 refresh further reduced sidebar prominence, separators and icon treatments, and moved its cool base toward a warmer neutral. | Generate both directions from explicit OKLCH coordinates, clamp to sRGB, and test resolved sRGB pairs. Direction A keeps a low-chroma teal bias; B uses a near-neutral paper/ink base. Both use the same semantic roles and the same status/data palettes. Chrome uses surface tokens, never a separate brand gradient. |
| Contrast and focus | WCAG 2.2 Understanding 1.4.11 and 2.4.13 (R48, **V**) | Required UI boundaries, states and graphical objects need 3:1 against adjacent colours. A custom focus indicator must satisfy adjacent contrast; the AAA focus-appearance criterion also describes a 3:1 state change. Testing must use the actual adjacent colours, not a single assumed background. | Every declared text/surface, control/surface, chart-mark/surface and focus/surface pairing is generated into a contrast table. Text targets 4.5:1, large text and non-text 3:1. The focus token is tested on all four surface levels and on accent-subtle. |
| Colour-vision deficiency | Machado, Oliveira and Fernandes 2009; Datawrapper guidance (R49–R50, **V**) | A simulation is a diagnostic, not proof of accessibility. Red/green confusion is common; varying lightness, reducing category count and double-encoding with shape, pattern or labels is more robust than trying to find six magical hues. | The gallery provides protanopia, deuteranopia and tritanopia simulations using published Machado severity-1 matrices. The validator reports simulated pair distances and greyscale lightness separation, but the acceptance rule is structural: statuses and evidence strength retain words/shapes; charts retain labels/decals. No pass/fail claim rests on simulation alone. |
| Typography and Urdu | Official Noto usage and Noto Sans Arabic specimen (R51, **V**) | Noto recommends Arabic UI variants where vertical space is constrained, Naskh for long text, and identifies Nastaliq as the style predominantly used for Urdu. Noto Sans Arabic supports Urdu shaping. The source does not establish a universal Nastaliq line-height. | Keep the installed Noto Sans Variable, Noto Sans Mono and Noto Sans Arabic Variable. U1 proves Urdu shaping, mixed direction and generous line boxes with the installed sans face. It does **not** add a Nastaliq dependency or settle O-6; evidence-text typography remains an owner/native-reader decision. Numeric samples use tabular figures; identifiers stay LTR and mono. |
| Spacing and density | Carbon spacing, grid and data-table guidance updated September 2026 (R52, **V**) | A constrained 2/4/8-derived spacing scale creates rhythm; density is a coordinated system, not a row-height toggle. Header, row, toolbar and pagination sizes must align. Carbon's five row heights are 24/32/40/48/64 px. | Use the directive's 2, 4, 8, 12, 16, 20, 24, 32, 40, 48, 64 scale. Compact/default/comfortable table rows are 32/40/48 px and their related controls/padding move together. The gallery shows the same content in all three modes, with compact as the table default. |
| Depth | Carbon role-based themes; Linear 2026 (R46, R53, **V**) | Roles must not change meaning across themes. Hierarchy can be made by tonal layers and reduced separator contrast; a border should clarify a relationship, not decorate every region. | Four visibly distinct surfaces carry depth. Working regions use tone or one hairline, not tone + border + shadow. Shadows are reserved for overlays and dragging. Direction A and B must remain equally legible when all shadows are disabled. |
| Motion | Carbon motion guidance updated August 2026; reduced-motion guidance (R54–R55, **V**) | Productive motion should be efficient and unobtrusive; expressive motion is rare. Carbon's six static durations are 70/110/150/240/400/700 ms with separate standard/enter/exit curves. Reduced-motion removes non-essential movement rather than merely shortening everything. | Adopt the six named duration tokens and productive curves. U1 examples cover tone feedback, disclosure, panel entry and one source-row emphasis. The gallery switch emulates reduced motion; the media query disables transforms, animated scrolling and chart animation while preserving instant state changes. No loop, hover lift or press-scale is allowed. |
| CSS architecture | CSS Cascade Level 5 and CSS Logical Properties Level 1 (R56–R57, **V**) | Layer order outranks selector specificity for normal rules; unlayered normal rules outrank all named layers. Layer order must therefore be declared before imports and every migrated rule must actually live in a layer. Logical properties map layout to writing mode and direction. | `styles/index.css` declares `reset, tokens, base, layout, components, patterns, pages, utilities` before importing them. U1 also wraps the untouched legacy files in a controlled compatibility layer so unlayered legacy rules cannot silently beat the new architecture. New U1 CSS uses logical properties and low-specificity `.nx-*` classes; tests reject `!important`, IDs, raw colours and duration literals in migrated component CSS. |
| Iconography | Lucide React and accessibility guidance (R58, **V**) | Per-icon ESM imports are tree-shakable. Lucide v1 hides decorative icons from assistive technology by default; meaningful icon-only controls still need an accessible name. Stroke weight is configurable and scales unless non-scaling stroke is requested. | One family only. U1 specimens standardise 16/18/20 px with 1.75 stroke, `currentColor`, labels in primary navigation, and accessible names/tooltips for icon-only controls. Icons never substitute for status words and are not placed in coloured decorative tiles. |
| Charts | Official Apache ECharts ARIA and theme guidance (R59, **V**) | Themes centralise visual styling. ARIA descriptions and decal patterns are opt-in, and `AriaComponent` must be imported. A generated description may be too verbose for large scatterplots, so authors must be able to supply a concise description. | The `nx` theme reads resolved CSS tokens at runtime. Data colours are shared with the gallery; patterns are on in forced/high-contrast modes. Chart examples include a text/table twin. U1 checks that the ARIA component is present instead of assuming `aria` config alone works. |
| Performance | web.dev INP, updated September 2025 (R60, **V**) | INP is the latency through the next painted frame; ≤200 ms at p75 is “good”, but 100 ms remains the better target for direct feedback. Measurement must include clicks, taps and keyboard interactions after load. | Record a production-build baseline for three repeatable interactions: direction/theme switch, density switch, and foundation specimen/filter interaction. Report median and worst observed sample plus the test machine; do not call a lab run a p75 field result. U1 blocks on any interaction above 200 ms and aims below 100 ms for the two simple switches. |

## The two directions to build

The options differ only where a real owner decision is needed. They do **not** compete on accessibility, density, motion, component shape or information architecture.

### A — Teal instrument

- Cool paper and slate surfaces with a restrained teal bias; continuity with the accepted product identity without the current cobalt glow, radial field or gradient chrome.
- Teal is the only interactive accent. The working surface stays quieter and less saturated than the evidence and answer content.
- Best case: the product still feels recognisably NexusAI after the migration. Risk: even low-chroma teal chrome may preserve too much of the “AI/techy” impression.

### B — Ink and signal

- Near-neutral paper, ink and slate surfaces, with a blue-teal signal accent used only for action/selection.
- Slightly warmer light backdrop and neutral dark chrome make records and data colours more prominent.
- Best case: closest to a professional record/instrument. Risk: if separation is too subtle, the interface can become generic or muddy; surface and border checks must guard against that.

Both directions will render the same synthetic Dashboard, Ask answer with workbench, and Case Overview in light and dark at 1440 and 390 px. High contrast, all densities, type, icon, motion and CVD specimens are shared foundation proofs rather than a third visual direction.

## What was rejected

| Rejected | Reason |
|---|---|
| A third, “expressive” palette | It would delay the owner decision without testing a materially different product principle. The directive asks for A and B; both already span continuity versus a neutral record-like instrument. |
| Pure white/pure black foundations | They flatten the surface hierarchy and make prolonged dark-mode reading harsher. Four tonal surfaces are a functional navigation aid. |
| Purple, neon, coloured shadows, glass/blur and gradient data panels | They turn evidence into decoration, reduce contrast predictability and repeat the exact aesthetic the owner rejected. |
| Status colours as the categorical data palette | Status and data have different meanings. Reusing colour roles creates false semantics and makes future charts harder to read. |
| Colour alone for Observed/Inferred/Candidate | It fails the product truth contract. Word + shape remains mandatory even if every simulated palette pair is distinguishable. |
| A CSS-in-JS system, UI kit, Storybook or Style Dictionary | Each would create a second styling or gallery system and add dependency/runtime cost. Plain CSS layers plus the in-product gallery match the repository and U1 scope. |
| Hand-authored CSS variables as the source of truth | It cannot prove DTCG conformance, alias integrity or keep contrast metadata beside the values. CSS is generated output. |
| Dynamic `color-mix()` inside components | It makes resolved states difficult to audit. Hover, selected, disabled and tint roles are generated and tested as first-class tokens. |
| New font dependency or a premature Nastaliq choice | O-6 needs a native reader. The installed Arabic face can prove shaping and direction without expanding U1 or pretending typography preference is settled. |
| Animation used to make the gallery feel premium | NexusAI is a repeated-work instrument. Motion is kept only where it explains state, continuity or provenance. |
| A simulator-only colour-blind “pass” | Simulations are approximations and people vary. Shape, pattern, labels and lightness remain the acceptance mechanism. |

## Acceptance measurements set before implementation

1. **Token integrity:** every token has a valid DTCG type; aliases resolve without cycles; the generator is deterministic; generated CSS and test JSON are current.
2. **Contrast:** all declared used pairs pass 4.5:1 text or 3:1 non-text/large text; focus passes every adjacent-surface check. Results are visible in `/design/foundations` and machine-readable.
3. **CVD:** the gallery offers three Machado simulations and greyscale. Status/strength/data examples remain understandable without hue. Pair-distance output is diagnostic and no semantic distinction relies on a numeric threshold alone.
4. **Architecture:** one declared layer order; no migrated U1 rule is unlayered; no raw colour, `!important`, ID selector or duration literal in new component/page CSS; every one of the 150 legacy names is defined in the alias layer.
5. **Budgets:** generated foundation CSS is reported separately; total legacy + foundation CSS is baselined truthfully. The final ≤150 KB target belongs to staged retirement, so U1 must not claim the existing 618 KB debt is solved merely by layering it. No U1 route chunk exceeds 30 KB gzip of newly shipped foundation CSS.
6. **Responsive/accessibility:** no page-level horizontal scroll at 390 or 320 CSS px; visible keyboard focus; `dir="auto"`/`bdi` specimens; reduced motion, forced colours and high contrast preserve meaning.
7. **Performance:** three production-build interactions measured with next-paint timing; simple switches target ≤100 ms and all must remain ≤200 ms on the reference laptop.

## Sources added by U1

The primary URLs and confidence marks are recorded as R45–R60 in the shared research register. Every **V** row was fetched and read during this stage on 2026-10-28. The directive's older **S/K** findings were not silently promoted; where U1 depended on one, a primary source was read and a new row was added.
