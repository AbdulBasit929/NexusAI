# NexusAI interface foundations

This file is the short, agent-readable contract for the interface system. The
product is a calm forensic instrument: evidence and provenance dominate; chrome
is quiet; colour has a job; no visual treatment may imply certainty the records
do not provide.

## Source and generated outputs

- `design/tokens.json` is the only authored token source. It follows the DTCG
  2025.10 format and keeps primitive OKLCH values separate from semantic roles.
- `design/build-tokens.mjs` validates token types and aliases, rejects cycles,
  resolves colours to sRGB, verifies declared contrast pairs, and writes
  `src/styles/tokens.generated.css` plus `design/tokens.generated.json`.
- Generated files are committed but never hand-edited. Run `npm run
  tokens:build` after changing the source.
- `src/styles/legacy-aliases.css` is temporary migration infrastructure. Every
  old `--analyst-*` name maps to an `--nx-*` role so untouched pages keep
  working. It is removed only in U10 after every page has migrated.

## Theme and role contract

The selected product direction is `b`, **Ink and signal**: near-neutral
record-like surfaces with one blue-teal action signal. `a`, **Teal instrument**,
remains only in `/design/foundations` as the comparison record that led to the
decision. Both share the same semantics, status meanings, evidence strengths,
data palette, density, motion and component geometry. Light, dark and
high-contrast themes change values, never roles.

Core roles are:

- surfaces: `--nx-surface-0..3` and `--nx-surface-ask`;
- text: `--nx-text-1..3` and `--nx-text-on-accent`;
- action: `--nx-accent`, its state roles, and `--nx-focus-ring`;
- truth: `--nx-status-*` and `--nx-strength-*`, always accompanied by words and
  shapes;
- visualisation: `--nx-data-1..6`, `--nx-data-gray`, `--nx-seq-1..7` and
  `--nx-div-1..7`.

Components consume semantic roles only. They do not compute colours or reach
into a primitive ramp. The ECharts `nx` theme resolves the same CSS roles at
runtime, so HTML and charts cannot drift into separate palettes.

## Contrast checks

`design/tokens.json` declares every supported foreground/background use. The
generator maps each use to the custom properties below and fails when a resolved
pair misses its target.

| Use | Foreground properties | Background properties | Target |
|---|---|---|---|
| Body and actionable text | `--nx-text-1`, `--nx-text-2`, `--nx-text-3`, `--nx-accent-text` | `--nx-surface-0..3`, `--nx-surface-ask`, status tints | 4.5:1 |
| Accent controls | `--nx-text-on-accent` | `--nx-accent`, hover and pressed roles | 4.5:1 |
| UI boundaries and chart marks | `--nx-border-default`, `--nx-border-strong`, `--nx-data-*`, `--nx-seq-*`, `--nx-div-*` | their declared adjacent surface | 3:1 |
| Keyboard focus | `--nx-focus-ring` | every surface and accent-subtle | 3:1 |

The generated report and `/design/foundations` show the resolved ratios. A CVD
simulation is diagnostic only; labels, shapes, patterns and evidence words are
the actual guarantee.

## CSS architecture

`src/styles/index.css` declares this order exactly once:

`reset, tokens, base, layout, components, patterns, pages, utilities`

Every import is assigned to a layer. Untouched styles are bounded in
`pages.legacy`; new styles use low-specificity `.nx-*` classes, logical
properties and state attributes. New page/component CSS contains no raw colour,
`!important`, ID selector or literal duration. Motion uses `--nx-duration-*`
tokens and disappears under reduced-motion preferences.

## Geometry, type and interaction

- Noto Sans Variable is the UI face; Noto Sans Arabic handles Arabic-script UI;
  Noto Sans Mono isolates identifiers. User text uses `dir="auto"`; identifiers
  remain LTR and tabular.
- Space uses the named 2/4/8-derived scale. Radius, shadow, z-index, control
  height and row height are tokens.
- Density is coordinated through `compact`, `default` and `comfortable`; it is
  not a one-off table override.
- Lucide is the only icon family: 16/18/20 px, 1.75 stroke, `currentColor`.
- Focus is visible, controls retain names, reduced motion preserves state, and
  forced colours preserve borders and selection.

## Gallery and migration

`/design/foundations` is the visual gate. It uses synthetic values only and
renders the same Dashboard, Ask/workbench and Case Overview for both directions,
both themes and 1440/390 previews. Existing production pages intentionally keep
their markup and legacy CSS in U1. A later stage may retire a legacy file only
when its whole page slice has moved to these foundations and its tests pass.

Verification commands:

```text
npm run tokens:build
npm run lint:styles
npm test
npm run build
npm run measure:foundations
```
