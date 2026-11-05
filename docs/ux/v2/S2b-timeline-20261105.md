# Stage S2b: the evidence timeline (dashboard, section 2)

**Owner's direction (2026-11-05):** one section at a time, each with its own research; many distinct, interactive visuals; clean, little text. Section 1 (indicators) is S2a. This is the section beside it in the first viewport.

## Research behind this section

| Question | Finding | Source |
|---|---|---|
| Which form for activity over time across many entities? | Calendar heatmap for daily rhythm across many entities; **small multiples** for per-category trends that must be compared precisely; a streamgraph only for a few categories where the overall shape is the point, and not where values are read. | [Domo, stream graphs](https://www.domo.com/learn/charts/stream-graphs) (vendor guidance; the category counts are rules of thumb) |
| Stacked area or streamgraph? | Stacked area keeps a zero baseline (totals, exact comparison) but distorts the layers on top; a streamgraph's shifting baseline means a layer can only be compared with itself. The 2016 study found that less distortion improves readability, with streamgraphs best for value comparison, and that the interactive version gave more correct answers but slower ones. | [Thudt et al., Assessing the Readability of Stacked Graphs, Graphics Interface 2016](https://graphicsinterface.org/wp-content/uploads/gi2016-21.pdf) |
| What do investigation tools do with dense events? | Overview strip plus a detailed window, zoom and pan, grouping of events that sit too close, staged loading. | [Cambridge Intelligence KronoGraph](https://cambridge-intelligence.com/?p=43250), [ServiceNow Investigation Canvas timeline](https://www.servicenow.com/docs/r/zwMwt0JEh8FTHmrqXNjCAw/3yq~D5lXy97dHWyhjt1bDA) |

**What I took from it, and what I did not.** The cases cover different periods and some are read only in part, so a stacked area or streamgraph over the sum of the cases would show a false decline where the read stops. Per-case lanes (small multiples) are the honest form. What the first version lacked was a **shared calendar**, so the lanes could not be compared in time; the investigation tools' overview-and-detail idea is kept as a hover/keyboard reading of one month (the detail), not as a second chart, because the data holds at most 100 rows per case and a zoomed day view would mostly be empty. A calendar heatmap needs a full daily read; that is [BACKEND_REQUESTS](../../work/BACKEND_REQUESTS.md) row 19.

## What was built

- **Shared calendar axis** (`alignTimelines` in `model.js`): from the earliest month any case has to the current month; equal groups of months when it would be more than 96 cells, the same groups for every lane; year ticks (at most seven).
- **Lanes** with own scale / shared scale. A cell is drawn only for months inside the case's own span.
- **Not read is not empty.** The service returns at most 100 rows for a case (one for each day and kind of record, oldest first). After the last row a capped case ends in a dotted run, and the figure for such a month reads "not read". The old label "first 100 days only" was wrong (the read of Nexusai Forensic Demo spans Jan 2025 to Jul 2026 in 100 rows); it now says "earliest rows only" with the rule in its title and in the design reference.
- **Crosshair reading:** pointing at the plot, or arrow keys / Home / End on the focused plot (Escape clears), marks the month in every lane, dims the other months, names the month in the panel title, and turns each lane's figure into that month's figure. A visually hidden status line says the same for screen readers. No floating card, so nothing hides data; touch scrolls vertically as usual.
- **The column beside it:** Needs you plus "pick up where you left off" (up to three questions) now share the left column, so the two columns end together; with no question history Needs you stretches instead.
- The range column ("Jun 2021 – Jul 2026") is gone; the shared axis says it. "Latest evidence" moved to the foot of the page at full width, in columns, until its own turn.
- The bars grow once (460 ms) like the indicators, and not at all under reduced motion.

## Checked

- 46 tests in the dashboard file pass (new: calendar alignment, grouping, not-read cells, future-only lane, keyboard reading, totals with a plus for capped cases, scale toggle).
- Screenshots looked at: 1440 × 900 light and dark with the crosshair, with and without question history; 1024; 390 dark.
- Not run: `vite build`, Playwright suite, axe, pointer events in jsdom (the pointer path is checked in the browser only).

## Open

- `@container dm` rules and the option-B rail/queue CSS are dead code (the rail layout was retired); remove in the cleanup pass.
- The "Time to ready" panel repeats the indicator of the same name; its section comes later.
- A case whose activity could not be read at all is named only when no timeline can be drawn; naming it beside the lanes is open.
