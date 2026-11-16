# S2d (redo) — Records by kind, 2026-11-16

**Question it answers:** which kinds of records does each case hold, and where do the records of each kind come from?

## Research (this section only)

Marks: **[V]** page read in full, **[P]** read in part, **[S]** search results only, **[X]** not found.

- **100% stacked bars [S]:** the form for comparing composition when the totals differ widely; the cost is that the totals disappear, so they must be printed; a source suggests about five segments at most, and beyond that small multiples or a heatmap; middle segments are hard to compare because their baselines differ; a heatmap reads exact values less precisely than bar length (general principle, no study found).
- **Marimekko and mosaic charts [S]:** worth their complexity only when column width carries real information; area comparisons are the weak point (Peltier); no formal study of accuracy found.
- **Telecom record dashboards [S, vendor blogs]:** calls and data records are kept as different record types; per-subscriber drill-downs are pre-aggregated; nothing found on forensic views of calls, internet sessions and app use per case, or on timeline views.
- **Not found [X]:** a study comparing a flow chart with a stacked bar or a heatmap for case-by-kind data. The choice of a flow is the owner's, made from three drawn options (flow, bubble matrix, mosaic) after a first version (a 100% composition bar for each case) was rejected.

## Decisions

- A flow: a node for each case (its height is its records), a node for each kind, and a ribbon between them as thick as the records the case holds of that kind. It answers both questions at once: follow a ribbon from either end. The five largest cases are drawn; the rest are one "N more cases" node, so every record is drawn.
- The layout is exact and tested (`flowModel`, `flowLayout`, `spreadLabels`): one scale for both sides, so a ribbon is as thick where it leaves a case as where it enters a kind; the sums of the ribbons are the totals; labels are spread so none overlaps, with a thin line from a label to its node when it had to move.
- Pointing at a ribbon, a case or a kind lights what belongs to it and dims the rest, and the sentence under the figure says what it is (kind, case, count, share of the case, share of all records of that kind). A kind is a button (focus lights it, a click keeps it lit); a case name is a link to the case. The table lists every case and kind.
- Text is never cut to a single short line: a case's name and its records are one text of at most two lines (the whole name is in the tooltip); a kind shows its name, count and share; in a narrow cell the kind drops the percentage (it is in the sentence and the table) and the names get the larger share of the width.
- Colours: the accent for calls, two of the data hues for internet and app use, grey for the rest; the labels carry every count, so no kind is told by colour alone.

## What it reads

The typing report for each case (`calls`, `sessions`, `usage`, with exact repeats left out) and the accepted rows of each case; `recordKinds` as before. No new endpoint.

## Checked and not checked

Checked: a 1366 × 800 window (a 437 px-wide cell), a 1100 × 1000 window in dark (a 395 px-wide cell), no label overlap or clipping and nothing outside the panel in both, the hover sentence, 65 dashboard tests. Not checked: 1900 and 390 px by eye, the keyboard path by hand, measured contrast of the ribbons, a right-to-left example, the layout script. Known limit: with six case labels in a cell shorter than about 200 px, a long case name is cut on its second line.
