# S2e — Data quality (dashboard hero, row 2, right cell) — 2026-11-16

Status: built on `ui/v2-redesign` (`:4182` only), **not committed** until the owner approves.

## What the panel answers
"Can I trust the rows?" Two different causes were mixed in one share before: an **exact repeat** is a duplicate left out of the counts (harmless); a **rejected** row is one the reader could not accept (needs a look). The panel now separates them.

## Design (owner picked option A, "quality map")
- Figure: share set aside (2.2 %) and "of 96,860 rows set aside"; the sentence becomes the hover readout (case, rows, rejected, repeats, share), wrapped to two lines, never cut.
- Map: one bubble per case. Across = rows read (log scale, decade ticks); up = share set aside (axis from 0); bubble **area** = rows set aside; a bubble with both causes is split in two slices (rejected `--nx-danger`, repeats grey). A case with nothing set aside is a small flat dot on the floor. Top three bubbles are named beside them (collision checked). Bubbles are focusable links to the case overview (Enter / click).
- Legend: two causes with counts; hover or focus lights one cause, click pins it, the other bubbles dim.
- "How the times are known": heading beside one thin segmented bar (analyst-confirmed, stated in file, source-declared, assumed) with shares.
- Table twin (dialog from the panel header): case, rows read, exact repeats, rejected, share.
- Short cells give up the times first, then the legend (container queries on the panel body); the map keeps the rest. States: skeleton, "Every row was kept", "No rows read yet".
- No target is defined for any of these figures, so none is drawn.

## Code
- `model.js`: `qualityMap()` (pure geometry, tested), `setAsideCauses()`.
- `insights.jsx`: `DataQualityPanel`; `styles/quality.css`.
- Tests: 69 pass in `dashboard.vitest.jsx` (3 new for `qualityMap`).

## Research marks
[S] data-quality dashboards (DataKitchen and similar): one trust question, causes separated. [S] DFRWS work on timestamp reliability: why "how the time is known" matters. [S] bubble / small-multiples guidance: area, not radius, for magnitude; log axes need labelled decades. Not read: sources that returned 403/404/429 earlier in the stage series.

## Checked
Real data on `:4182` (counts only): 8 cases, 96,860 rows, 1,811 rejected, 357 repeats. 1366×800 light: cell 347×340, nothing overflows (content 266/266 px). 1100×1000 dark: fits, hover readout complete.

## Not checked
1900 px and 390 px by eye; keyboard path by hand; measured contrast; right-to-left; the layout script (needs a headless browser, asks first). The pane was hidden during the check, so the ResizeObserver never fired; the panel now also measures its box once on mount.

## Known weaknesses
Cases with few set-aside rows are small bubbles close together; their names appear only on hover. Data is clustered between 1–3 %.
