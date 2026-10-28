# U1 foundations stage note

**Date:** 2026-10-28
**Branch / port:** `ui/u1-foundations` / `http://localhost:4182`
**Status:** gate ready; owner decision O-3 is still open
**Owner:** Codex for the interface paths; no backend or live state changed

## Outcome

U1 now has one generated foundation system and one visual decision surface. The
gallery compares **A · Teal instrument** with **B · Ink and signal** using the
same synthetic Dashboard, Ask/workbench answer and Case Overview. Each direction
can be viewed in light and dark at 1440 and 390 px; high contrast, density,
type/Urdu, icons, motion/reduced motion, contrast and CVD diagnostics are shown
beside the screens.

Nothing was retired. Existing pages continue through an exhaustive alias layer,
as U1 requires.

## What was built

1. A DTCG 2025.10 token source, plain-Node validator/generator and committed CSS
   and JSON outputs. It resolves authored OKLCH to sRGB and fails on broken
   aliases, cycles or contrast misses.
2. The fixed cascade order `reset, tokens, base, layout, components, patterns,
   pages, utilities`, with untouched files bounded in `pages.legacy`.
3. Aliases for all **150** distinct legacy `--analyst-*` names.
4. Base/layout foundations, motion and density tokens, forced/high-contrast
   handling, and the ECharts `nx` theme reading resolved CSS roles.
5. `/design/foundations`, its unit tests, a plain-Node style-contract scanner,
   and a repeatable browser/bundle measurement script.
6. `DESIGN.md`, the U1 research note, R45–R60 in the shared register, and this
   evidence packet.

## Decisions shown to the owner

| Direction | Character | Strength | Watch-out |
|---|---|---|---|
| A · Teal instrument | cool paper and deep teal-slate, restrained teal action | continuity without the prior glow/gradient chrome | may retain too much of the old technical character |
| B · Ink and signal | near-neutral paper/ink and slate, blue-teal action signal | strongest record/instrument character; data stands forward | quiet separation must not become generic |

Recommendation after implementation: **B · Ink and signal**. Its neutral chrome
makes evidence, warnings and charts easier to distinguish, while A remains a
credible continuity choice. The owner decides O-3 by looking at the complete
screen set, not swatches alone.

## Proof

| Check | Result |
|---|---|
| Generated contrast | **160 / 160 pass**, 0 failed; text 4.5:1, large/non-text/focus 3:1 |
| Token/alias/layer/style gallery tests | focused U1 suite: **8 / 8 pass** |
| Full regression suite | **536 / 551 pass** across 77 files; the 15 failures are confined to the two pre-existing legacy-answer suites whose three report/fixture directories are absent in this linked worktree (each loader returns `[]`). U1 did not touch those tests or loaders. |
| Style contract | plain-Node scan passes: no raw colour, `!important`, ID selectors or page duration literals in migrated U1 CSS |
| Production build | **passes**, 2,733 modules transformed; existing ECharts/Cytoscape/MapLibre chunk warnings remain |
| Foundation CSS | **24,881 B raw / 3,846 B gzip** route chunk; under the 30 KB gzip route limit |
| Initial JavaScript | **203,787 B raw / 67,117 B gzip** |
| Foundation JavaScript | **23,262 B raw / 6,464 B gzip** |
| Existing global CSS after production minification | **531,086 B raw / 71,181 B gzip**; the 150 KB raw target is not met because U1 is forbidden to delete legacy CSS |
| Local Event Timing p75 | direction **32 ms**, theme **32 ms**, density **40 ms**; all below the 200 ms INP threshold |
| Next-paint proxy p75 | direction **25.10 ms**, theme **23.00 ms**, density **25.40 ms** |
| Responsive visual check | A light/dark, B dark and the 390 px preview inspected; narrow Case Overview reflows without page clipping |

The interaction numbers are a local Chromium lab baseline, not field INP. The
machine-readable record, environment and method are in
`docs/ux/evidence/U1/metrics.json`.

## Known limits and deliberate deferrals

- Total CSS cannot meet 150 KB raw in U1 without violating the instruction to
  delete nothing. Later page stages must retire legacy selectors; U10 removes
  the alias layer.
- The directive names a Stylelint configuration, but Stylelint was not already
  approved or installed. U1 adds no dependency and enforces the same contract
  with `design/lint-styles.mjs`. Installing Stylelint still needs explicit owner
  approval.
- O-6 (Nastaliq versus Sans/Naskh for evidence text) still needs a native Urdu
  reader. U1 proves shaping/direction with the already installed face only.
- CVD simulation is not labelled as an accessibility pass. Redundant words,
  shapes, labels and chart patterns remain mandatory.
- Field INP, screen-reader walkthroughs and full-page legacy CSS retirement are
  later-stage proof, not claims made here.
- The repository-wide test command is not fully green in this linked worktree:
  `reports/golden-ir-20260923-final/raw`,
  `reports/clarification-options-20260923/fixtures` and every
  `fixtures/live-20260924*/raw` capture are absent, so the legacy fixture loader
  returns no responses. The run still passes 536 tests, including every U1 test.
  No fixture was copied, fabricated or committed to conceal that environment
  gap.

## Owner review script

1. Open `http://localhost:4182/design/foundations`.
2. Compare A and B in light, then dark, without changing any other control.
3. Switch 1440 to 390 and inspect all three synthetic screens.
4. Check high contrast, compact/default/comfortable density and reduced motion.
5. Review the generated contrast and CVD sections.
6. Record O-3 as A or B. That choice closes the colour gate and makes U2 the
   next implementation stage.
