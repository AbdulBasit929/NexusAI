# Stage S2c: the board (dashboard layout foundation)

**Why this stage exists.** The owner's complaint (2026-11-05): sections are not aligned with each other in height and position, "show as table" grows the panel and breaks the design, and the first screen does not show many distinct charts at once. All three had one cause: every panel's height followed its content. This stage replaces that with a fixed grid and measures it.

## Research behind this stage

| Question | What the sources say | Source |
|---|---|---|
| Grid for a dashboard | 12 columns; the columns of a row add up to 12; 3 / 3 / 3 / 3 shows more above the fold than 6 / 6; fixed row heights may be given in CSS units; leave some whitespace; stack to one column on a phone. No source found on equal-height cards or on fitting a whole dashboard into one window (that part is my own engineering, and is checked by measurement below). | [Dashboard layout best practices](https://kindatechnical.com/data-visualization/dashboard-layout-best-practices-and-design-patterns.html), [Wix dashboard UX guidelines](https://dev.wix.com/docs/build-apps/develop-your-app/frameworks/wix-blocks/dashboard-pages/ux-guidelines-for-dashboard-pages-in-blocks) |
| Table beside a chart | Inline disclosure (`<details>`) is simple; a modal keeps the dashboard layout undisturbed and suits wide tables, but must move focus in, keep it inside, announce itself and return focus on close. Neither was tested with users in these sources. One BI product puts "view as table" in the widget's menu, built from the chart's own result set. | [ezud](https://ezud.com/accessible-dashboard-design-patterns/), [Voyant](https://support.planwithvoyant.com/hc/en-us/articles/47369134635547-Accessible-Tabular-Chart-View-UK), [Kodelabs](https://docs.kodelabs.com/products/building-bi/reference/accessibility/view-chart-as-table) |
| Part-to-whole with many kinds | Donut and waffle suit about 2 to 5 categories; with 5 to 8 a sorted bar chart is the safer and more precise choice (treemaps compare poorly). The sources are vendor and agency guides, so these are rules of thumb. | [UNHCR dataviz guide](https://dataviz.unhcr.org/chart-types/part-to-a-whole/), [Highcharts](https://www.highcharts.com/blog/?p=18584), [Tableau](https://tableau.com/blog/viz-variety-show-precise-waffle-charts-68462) |

Decisions that follow: a 12-column board with fixed rows; the table opens in a dialog from a button in the panel header; evidence by type (seven kinds) is drawn as sorted bars, not a donut.

## What was built

- **The board** (`styles/board.css`). The first screen (header, indicator band, hero board) has a definite height: the window height less the top bar, clamped between 780 and 1000 px; it ends 28 px above the bottom edge so the next band shows as an edge. The hero is two rows (8 + 4 columns, then 3 + 5 + 4) and **every panel of a row shares its top and bottom**. Below it: a band of four panels of one height (3 + 3 + 3 + 3), then the cases table. Narrower windows get cells of one fixed height, two to a row, then one.
- **Panels draw into their cell.** Header 48 px for every panel (so titles start at the same place), one-line titles (a long note is cut first, never the title), a body that is as tall as the cell leaves, with the picture taking what is left. Rows and lanes that do not fit are not drawn (a container query on the cell's height decides: data quality shows three, four or five cases), and the rest are in the table.
- **The table is a dialog.** The button sits in the panel header; the dialog moves focus in, keeps Tab inside, closes on Escape, on its button and on a click outside, and returns focus to the button. A panel never changes size when its table is read.
- **Panels rebuilt for a cell:** Answers (columns by day take the height; the figure is in the indicator above, not repeated), Evidence by type (sorted bars), Records by kind (heat matrix with an "all cases" row), Data quality (dumbbells and one stacked bar), Time to ready (strips), Pick up (last questions, and a pinned "Ask across cases"), Needs you and Latest evidence (rows share the cell's height).
- **Measured, not eyeballed.** `scripts/ui-layout-check.mjs` opens the page in a real browser at any sizes and fails on: panels of a row without a common top and bottom, titles at different heights or on two lines, content that overflows its panel or is squeezed into too little room, more than 40 px of empty space at the foot of a panel, a page that scrolls sideways (and which element), tiles of one row at different heights, and a hero that is not inside the first screen on a 860-px-high window or taller. It seeds question history by default (`--history=0` for the empty state). Run: `node scripts/ui-layout-check.mjs --sizes=1440x900,1280x720`.
- Dead CSS from the retired rail layout, queue, old resume cards and key lists was removed from `dashboard.css` (81 rules, 8.5 KB), by a script that keeps any class a component still names.
- The timeline's scale control now reads "Own | Shared" (the full words are its tooltip and accessible name).

## Checked

- The layout checker passes at 320 × 640, 390 × 844, 768 × 1024, 1024 × 768, 1280 × 720, 1366 × 768, 1440 × 900, 1536 × 864, 1920 × 1080 and 2560 × 1440, with and without question history (the first screen test applies at heights of 860 and up).
- 87 tests in `src/v2` pass (new: the dialog's focus behaviour, Escape, click outside, the table not inside the panel; the Pick up panel; the lane note in a tooltip). `vite build` passes (16 s).
- Screenshots looked at: 1440 × 900 light and dark, 1440 × 1650 light, 390 dark.
- Not run: the Playwright suite, axe; no screen-reader test of the dialog (the sources say to validate with screen-reader users).

## Known limits

- On windows shorter than 860 px the hero is taller than the screen and the page scrolls (the cells keep their size, so nothing is squeezed).
- Case names are cut in narrow cells; the full name is in the link's text and the table.
- The cases table (last band) and the two cards "Needs you" and "Latest evidence" still have their original content; each gets its own research in its turn.
