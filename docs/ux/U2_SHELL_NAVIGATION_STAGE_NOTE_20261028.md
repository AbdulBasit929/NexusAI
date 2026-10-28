# U2 shell and navigation stage note

**Date:** 2026-10-28

**Branch / port:** `ui/u2-shell-navigation` / `http://localhost:4182`

**Status:** implementation complete; owner selected B · Case atlas

**Owner:** Codex for the interface paths; no backend, database, retained evidence or live service changed

## Outcome

NexusAI now has one persistent, case-centric application shell. B · Case atlas
keeps global workspace destinations available while making the current case a
distinct working block. The shell answers scope before content: workspace,
case, section, readiness and available actions are visible without inventing
activity or identity state.

The comparison gallery remains at `/design/components`, with B recorded as the
owner-selected direction and A/C retained as the decision record.

## What was built

1. A 56 px global header, 264/56 px expanded/collapsed rail and 48 px case
   context band, with remembered desktop state and 1024/768 responsive changes.
2. Purpose-labelled Workspace, Case and Preferences navigation, one banner,
   one focusable main, truthful readiness/activity state and route-owned title,
   focus and polite announcement behavior.
3. A single command and shortcut source for `/`, `?`, `g` sequences and
   Ctrl/Cmd+K. The palette keeps focus on its combobox, uses
   `aria-activedescendant`, groups Go to/Do/Ask results and never executes Ask
   merely by opening or typing.
4. System, light, dark and high-contrast appearance; default, comfortable and
   compact density; standard, detailed and concise answer detail; and system or
   reduced motion. Guarded pre-paint storage prevents a wrong-theme flash.
5. Responsive modal navigation, mobile case-section selection, bottom-sheet
   overlays, 44 px primary touch targets, offline treatment, not-found and
   capability-denied states, and explicit next actions.
6. A truthful activity disclosure that reports only current processing.
   Historical completed/failed jobs are not fabricated; backend request 27
   records the missing global history contract.
7. The old shell stylesheet is no longer imported. `shell-u2.css` owns the new
   shell in the normal pages layer while remaining page-era CSS stays bounded.

## Acceptance proof

| Check | Result |
|---|---|
| Focused U2 and adjacent contracts | **44 / 44 pass**: shell, commands, appearance, settings, route effects, theme and gallery selection |
| Foundation/layer plus contention reruns | **15 / 15 pass** across the promoted shell import, workbench rendering and Ask-page abstention behavior |
| Ported presentation contracts | **46 / 46 pass** |
| Full repository accounting | **547 / 562 accounted green**. The first concurrent run reached 544; one stale shell-import expectation was corrected and two resource-contention timeouts passed unchanged in isolation. The remaining 15 failures are the pre-existing capture-fixture gap described below. |
| Style contract | **passes** for all three migrated files; no raw color, `!important`, ID selector or duration-literal regression |
| Production build | **passes**; 2,733 modules transformed and the final quiet optimized build exited 0 |
| Desktop route smoke | 12 global and case routes; correct titles and main focus, no page-level overflow, no console warning/error |
| Mobile route smoke | the same 12 routes at 390 × 844; drawer, section select and bottom-sheet behavior exercised |
| Responsive geometry | header 56 px, case band 48 px, rail 264/56 px; desktop, 1023 px tablet and 390 px phone inspected |
| Appearance and commands | every theme, density, answer-detail and motion option exercised; palette groups, typing, arrows, route sequences, switcher and shortcut sheet exercised |

The machine-readable checklist and exact viewport matrix are in
`docs/ux/evidence/U2/metrics.json`.

## Known limits and deliberate deferrals

- Global completed/failed job history is unavailable in the current API. The
  shell says so and links to Activity; request 27 defines the needed backend
  contract.
- The linked worktree lacks `reports/golden-ir-20260923-final/raw`,
  `reports/clarification-options-20260923/fixtures` and the retained
  `fixtures/live-20260924*/raw` captures. Fifteen legacy-answer tests therefore
  receive empty fixture arrays. No fixture or passing assertion was fabricated.
- Field INP, native screen-reader walkthroughs and job-history UI are later
  evidence. U2 does not claim them.
- The branch is not merged into `lane-arms`; that remains an explicit repository
  action after owner review.

## Close

U2 is ready for owner review. The next interface stage is U3 only after the
owner accepts this shell and explicitly authorizes the repository transition.
