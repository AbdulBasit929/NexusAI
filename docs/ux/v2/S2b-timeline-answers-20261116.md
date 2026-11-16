# S2b (redo) — the evidence timeline and the answers panel, 2026-11-16

**Questions they answer:** when did the events of each case happen, on one calendar (timeline); were my questions answered, and if not, why (answers).

## Research (these sections only)

Marks: **[V]** page read in full, **[P]** read in part, **[S]** search results only, **[X]** could not be read.

- **Grafana state timeline, status history and heatmap docs [V]:** a timeline of entities is rows of states over time; drag to zoom, double-click to zoom out, drag the axis to pan; a heatmap is the form for a distribution over time (it needs buckets, which our service does not give for answer times); a colour scale can be logarithmic; equal consecutive values can be merged.
- **KronoGraph (Cambridge Intelligence) [P, vendor guidance]:** crowded events become a heatmap and zooming in shows individual records; a "lens" keeps the whole in view while a part is examined; pair the timeline with another linked view. No numbers on thresholds.
- **Small multiples versus horizon charts [S]:** a controlled study (Javed, McDonnel, Elmqvist) found juxtaposed lines and small multiples more robust than horizon graphs as the number of series grows; horizon graphs trade accuracy for space. Only summaries were read.
- **Unit charts [S]:** one mark for one unit; the legend must say what a mark is; highlighted marks should be grouped; avoid them for large counts; give a table as a fallback. No study found that unit charts beat bars for part-to-whole, and one found pictographs and bars equally accurate for simple part-to-whole judgements.
- **LLM-answer monitoring guidance [S, vendor blogs]:** track refusal rate as a time series, show the median and the 90th and 99th percentile together, and do not trust latency alone as a quality signal.
- **Could not be read [X]:** Honeycomb heatmap docs (404), the Cockburn–Karlson–Bederson overview-plus-detail survey (403), Observable Plot (rate-limited). Nothing is claimed from them.

## Decisions

- **Timeline:** keep one lane per case on one shared calendar. Add drag-to-zoom on the lanes (double-click or Escape to go back; plus and minus keys; shift with an arrow moves the view), 1 y / 3 y / All buttons, a strip of all cases together with the part in view marked (drawn only when the cell is tall enough), the busiest month named for each lane, a colour for each case (the six data colours) with a dot beside its name, and a card that names the month and every case's count while the pointer is on the plot. A case read only in part still ends in a dotted run; an event dated in the future is still named and not drawn.
- **Answers (direction A, chosen by the owner):** a half-circle gauge cut into answered, declined and abstained; the week as a stacked area (one value for each rolling 24-hour period); the reasons for declines ranked, each with its count and its share of all questions; and one line for the median, 90% and slowest time. One readout serves the gauge, the legend, the trend and the reasons (pointing lights a part and dims the rest; a click on an outcome keeps it lit).
- **Responsive:** the cell decides, by container query. From about 400 px of cell width the gauge stands beside the week, the reasons and the times; narrower, one column. Parts give way from the bottom as the cell gets shorter, reasons before the week, so a row is never cut in half. Measured heights in one column: gauge (150 px wide) with its one-line legend 106 px, week 54, reasons 29 plus 36 for each row, times 17, gaps 10. The smallest cell the board ever gives the panel is about 220 px (a laptop), and that shows the gauge, one reason and the times; the week appears from about 272 px. An earlier version hid the reasons below 240 px and left only the gauge on a laptop; it was found by the owner and fixed.
- **Not drawn, by design:** a heatmap of how long answers take (needs a distribution by day: backend request 34), and the days of the week as separate columns in Answers (they are in the Answered indicator and in the table).

## What it reads

The case activity read (`activity_by_day`, capped at 100 rows for a case, oldest first), the engine-health reads (7 days, and the 7 windows for the per-day split), the decline reasons and the latency of the case with the most questions. No new endpoint. Pure selectors: `alignTimelines`, `viewWindow`, `zoomWindow`, `panWindow`, `windowTicks`, `answerQuality`, `exceptionRows`, `gaugePath`.

## Checked and not checked

Checked: the timeline at 1440 px (light) and 1900 px (dark) including drag-zoom, minus and Escape, the hover card and the range buttons; the answers panel in light and dark at 1100 px (one column) and 700 px (side by side), including the hover readouts and that no row is cut; 59 dashboard tests. Not checked: 1280 and 390 px by eye, the keyboard path of the gauge, measured contrast, a right-to-left example, and the layout script (it needs a headless browser).
