# S2a (redo) — the key-indicator band, 2026-11-13

**Question it answers:** how big is the workspace, how healthy, and what waits for me? Six tiles, each one link to the list behind its figure.

## Research (this section only)

Marks: **[V]** page read in full, **[P]** read in part, **[S]** search result only.

- **Carbon, dashboards and chart anatomy [V]:** the most important data gets the highest contrast and the largest area, at the top and first in the reading order; limit the number of metrics; prefer direct labels to legends; use one colour for one data set across the dashboard; tooltips repeat the values. A KPI is a number with a short description. No numeric guidance on density or grid.
- **Stripe Apps, chart layout [V]:** a metric is a caption label, then a bold value, then a sparkline with no axes or labels; at most three charts in a row; charts get hard to read under about 200 px; use the same height for loading, empty and filled states. No guidance on words, colour, delta or accessibility.
- **Grafana, stat panel [V]:** the picture sits behind or beside the value and hides itself when the panel is small; a missing value is a hyphen; colour meaning comes from thresholds, not from the panel.
- **Tufte, sparkline theory [P, about half verbatim]:** a small graphic beside the number, no frames or backgrounds; mark the latest value; pair a bare line with start and end values.
- **Magnet AXIOM case dashboard (2018) [V]:** case context, then evidence, then places to start; it describes no figures or charts.
- **Not found or not read:** Linear, Vercel and PostHog describe no headline-metric band; Plausible's dashboard could not be read; Carbon's meter-chart page returned 404; no source supports the readout-replaces-the-line interaction (it is a decision of this section, also used by the timeline panel).

## Decisions

- Six separate tiles in the panels' own look, six across at 1020 px of band width and above, three across at 560 to 1020, two below. At 1440 each tile is about 181 px wide, under Stripe's 200 px figure for full charts; the pictures here are small marks, not charts.
- One visual language for all six pictures: the same height on one baseline, rounded marks with a 2 px gap, grey for context, the accent only for what matters, the state colours only where a mark is a state.
- The picture follows the question: a bar for each case, coloured by state (a whole made of cases); a share bar (files); columns by case (where the volume is); stacked columns by day with the two ends named (a rhythm and its split); a bar split by cause (what waits); a histogram with its median (how long).
- Words: a label, a figure, at most one short suffix, one line under the picture. That line also says what a mark is while the pointer is on it. No legends.
- No target exists for any figure, so none is drawn against a target and no arrow or colour says that up is good.

## What it reads

Case status (`/collections/status` through `useConfiguredCaseOverviews`), engine health for the answered share and its seven daily reads, `recent_jobs` for the time to ready, the attention groups for what waits. Selectors: `caseStateMix`, `durationHistogram`, `timeToReady`, `answerQuality`, `dailyQuestions`, `needsYou`. No new endpoint.

## States

Loading: skeleton figure and a skeleton bar at the final height. Empty: a dash for the figure and seven empty ticks (answered), or the "All clear" line (needs you). Partial: a case that could not report is counted with the unread. Error and first run: the page's own states.

## Checked and not checked

Checked: 1440 px light and dark, 390 px dark, no text cut off, no sideways scroll, hover readouts for every tile, 50 dashboard tests. Not checked: 1280 and 1024 px by eye, keyboard path, contrast ratios by measurement, a right-to-left example, a taller window than 900 px. The band is 174 px high, so at a 900 px window the next band is about 16 px under its minimum.
