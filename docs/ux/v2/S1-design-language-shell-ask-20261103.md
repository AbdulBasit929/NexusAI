# V2 stage S1: design language, shell and Ask (first slice), 2026-11-03

**Status:** built on branch `ui/v2-redesign`, served on `:4182`, **not accepted by the owner yet** (the owner accepts a page by looking at it). `:4181` and `lane-arms` are untouched.

## Why
The owner (2026-11-03): everything Codex built (U1 foundations, U2 shell, the pages) is redone, every page and section, one at a time, best of the best. The colour direction B "Ink and signal" and the shell "Case atlas" are reopened; the rules of `docs/ux/MASTER_UI_UX_DIRECTIVE.md` still bind.

## What was wrong (read from screenshots of `:4182` on 2026-11-03)
Wide letter-spacing on small text, monospace display titles that lead with the raw collection id, coloured left borders and tinted blocks on every card (card soup), a bubble pack, a progress ring and a Sankey on the dashboard that answer no question, an Ask start that is a centred poster, an Evidence page whose table starts below the fold, and two competing colour/space systems (150 aliases over 618 KB of old CSS).

## What was built
| Part | Where | Notes |
|---|---|---|
| Colour tokens | `design/tokens-v2.mjs`, `design/make-tokens.mjs` -> `src/v2/styles/tokens.generated.css` | OKLCH, three themes (light, dark, high contrast). 114 pairs are checked when the file is built (text 4.5:1, controls, focus and marks 3:1); the build fails below. Dark canvas `#111418` (not near-black), accent a deep blue-teal. |
| Cascade layers | `src/v2/styles/index.css`, `src/styles/legacy.css` | The old CSS is demoted to the lowest layer; the new layers always win. Element-level base styles apply only inside `.nx-ui`, so a page that has not been rebuilt keeps its look inside the new frame. |
| Shell | `src/v2/shell/*` (`components/CaseShell.jsx` re-exports it) | Sidebar (workspace, the case you are in with its readable name and status, recent cases), breadcrumb top bar, Ctrl+K search (cases, evidence of the case, pages, earlier questions, "ask this"), activity menu, appearance menu (theme incl. high contrast, density, reduce motion). Collapsible rail, remembered per kind of page; drawer on a phone. |
| Ask | `src/v2/pages/ask/*`, route `/cases/:id/investigate` | History of this browser, a conversation that uses the width it is given, the question box (Enter sends, scope one click away, Stop), an answer in a fixed order (strength, headline with citations, how the question was read, the result, the evidence bar, the model's reading, limits, how it was worked out, next questions, actions), an evidence drawer, a table that says "Showing 1 to 50 of 408". The engine call, the presentation mapping and the history hooks are the existing ones. |
| Table | `src/v2/ui/DataTable.jsx` | Numbers right-aligned and sorted as numbers; a long run of digits (a phone number, an account) is an identifier and stays text; filter, copy as CSV, export; never hides a row silently. |
| Design language page | `/design/language` | Colour of the three themes with the contrast of every pair, type incl. Urdu, components in their states, space, shape. |
| Case names | `src/v2/lib/caseLabel.js` | A case with no name an analyst gave it is shown as a title and a date ("Orion Lahore Reference, 30 Oct 2026"); the identifier stays as the title attribute. |

## Proof (2026-11-03)
* New tests: `src/v2` has 4 files, 32 tests, all pass (tokens and contrast, shell, table, case label).
* Whole suite on this branch: 542 passed, 29 failed. 14 of the 29 tested the old shell's markup (`CaseShell.vitest.jsx`, deleted and replaced by `AppShell.vitest.jsx`); the other 15 are in `investigationPresentation.vitest.js` and `InvestigationResult.vitest.jsx`, which read retained response captures that exist only in the lane tree (untracked there), so they fail in any separate worktree and were not touched by this stage.
* Looked at on the live Orion case at 1440 and 390 wide, light and dark: the empty Ask start, a real answer (a table with its row count, the evidence bar "32 records in 1 file, recomputed from the records: matches", the evidence drawer with the cited rows), the design language page, an old page (Overview, Dashboard) inside the new frame. Screenshots: `docs/ux/evidence/v2-s1/`. No browser errors except the known 503 of the capabilities route.

## Not done (so do not read this as finished)
* `vite build`, a Playwright test on a generated case, axe, a keyboard walk by hand, right-to-left and colour-blind checks.
* Every other page still shows its old content inside the new frame: Dashboard, Cases, New case, Overview, Evidence list and viewers, Connections, Timeline, Activity, Data, Settings, Ask across cases.
* History is the browser's own (the conversation service, backend request BR-2, is being built on its own branch). "Pin to findings" and "compare" are not shown because nothing stores them yet.
* The model's reading takes 43 to 57 seconds; the page says it is being read.

## Look at it (owner's demonstration script)
1. Open `http://127.0.0.1:4182/design/language`: the colours, type and components. Change the theme in the top bar's appearance menu (Light, Dark, High contrast) and the density.
2. Open `http://127.0.0.1:4182/cases/orion-lahore-reference-20261030/investigate`: click "Who 0300 0001006 talks to most", press Enter, wait about 30 seconds. Then click "Open the evidence".
3. Press Ctrl+K and type `evidence` or a three-word question.
4. Resize the window to a phone width.
Tell me what you do not like, in your words: a colour, the density, the order of the answer, the sidebar. Each is a change in one place.
