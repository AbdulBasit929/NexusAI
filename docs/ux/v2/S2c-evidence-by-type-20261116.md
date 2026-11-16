# S2c (redo) — Evidence by type, and the hero board's second row, 2026-11-16

**Question it answers:** what kinds of evidence do I hold, how much of each, is it usable, and which cases hold it?

## The layout fault found first

The first screen was a fixed height (780 px at the smallest) while the two hero rows needed 292 + 264 + 16 = 572 px, so on a short window the second row (Evidence by type, Records by kind, Data quality) spilled about 24 px into the band below. Fix: the first screen has a minimum height and grows with its content, and the second row has a floor of 340 px (both rows then share it, so the first row is 340 px at least). Measured at a 780 px-high window: 16 px between the rows and the band below, where there had been a 24 px overlap. On a short window the next band now starts lower; that is the price of never clipping.

## Research (this section only)

Marks: **[V]** page read in full, **[P]** read in part, **[S]** search results only, **[X]** not found.

- **Storage-breakdown screens (iManage, iOS-style storage, shadcn blocks) [S]:** a segmented bar with a labelled breakdown list beneath it; some let the legend hide categories; small segments are hard to read unless the chart is tall enough.
- **Magnet AXIOM case dashboard [P, 2018 article and docs]:** an "evidence overview" per source and "places to start" listing the most common evidence types with counts, broad to specific; media has its own explorer.
- **Treemap versus sorted bars [S]:** one controlled study (Yalçın, Elmqvist, Bederson, 2017) found wrapped bars beat treemaps for dense, sorted, non-hierarchical numeric data; practitioner guidance favours sorted bars for "the top categories". No controlled study of one sorted bar chart against a treemap for part-to-whole was found.
- **Cellebrite dashboards [X]:** nothing found.

## Decisions

- Keep the sorted bar (there are more than five kinds and an exact share is the point) and make each row a small object in the family of the indicator tiles: a chip with the kind's icon in the kind's permanent colour, the name, the bar, the count and the share of all files.
- The files that did not finish are the hatched red end of the bar, so what is unusable is seen where it sits (the share is not hidden in a tooltip).
- Pointing at a kind or focusing it dims the others, says what it is under the figure (name, count, share, files that did not finish) and opens a card that lists the cases that hold it (the card the timeline uses); a click keeps it lit. Each row is one button, so the keyboard path is Tab and Space.
- The figure under the title is the number of files; the old "items indexed for search" note moved to the tooltip of the sentence under the figure so the title never truncates in a narrow cell.
- The table twin lists each kind and each case (the keyboard and screen-reader way to the hover card).

## What it reads

The typing report for each case (`evidence`: modality, files, failed). `modalityMix` now also returns, for each kind, the cases that hold it (largest first, with the files that did not finish). No new endpoint. The per-case cards come from the same reads as before.

## Checked and not checked

Checked: a 1366 × 800 window (a 256 px-wide cell: seven rows of 30 px, none clipped, share column dropped, title not truncated), a 1100 × 1000 window in dark (a 395 px-wide cell, shares shown, hover card), the hover readout, 60 dashboard tests. Not checked: 1900 px and 390 px by eye, the keyboard path by hand, measured contrast of the kind colours on their chips, a right-to-left example, the layout script.
