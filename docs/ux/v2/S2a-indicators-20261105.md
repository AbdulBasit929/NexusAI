# Stage S2a: the indicators (dashboard, section 1)

**Owner's direction (2026-11-05):** KPIs and indicators first; many diverse modern visuals; much less text in Needs you; one viewport with several visualisations and more below; interactive, best animation; one section at a time with its own research. Recorded as R16 in the [design reference](DESIGN_REFERENCE_20261105.md).

## Research behind this section (done before building)

1. **A KPI is a number plus the one picture that explains it.** The guidance read for this stage puts the figure first, a small comparison or distribution beside it, and a caption that adds a fact instead of repeating the figure. No two tiles in a band should use the same picture. (Source links are not recorded in this note; the next section's research will list its sources.)
2. **Do not draw a target that does not exist.** Bullet bars, gauges and "up is good" arrows need a defined target or direction. None of the six figures has one, so none is drawn that way; the colour only marks what blocks work (Needs you).
3. **Motion is feedback, not decoration.** A count-up of well under half a second and a one-time bar growth on first paint are accepted; both must be off for people who ask for less motion, and the real number must be the text from the first frame (screen readers, search, tests).

## What was built

| Indicator | Figure | Picture (form) | Data |
|---|---|---|---|
| Cases | count | state bar (ready, processing, needs review, not reported) | case status service |
| Files ready | ready / all | segmented meter (ready, processing, failed, other) | `/collections/status` |
| Records | accepted rows | columns by case, one scale, largest first | case summaries |
| Answered · 7 days | share answered | stacked columns by day (answered, declined, abstained) | `/analysis/engine-health` ×7 windows |
| Needs you | count | one dot per thing, coloured by cause (unit chart) | attention groups |
| Time to ready | median | histogram on fixed bins, median bin marked | `recent_jobs` timestamps |

- New: `Indicators` (`pages/dashboard/indicators.jsx`), `CountUp` (`ui/CountUp.jsx`), `DotRow` and `Histogram` (`ui/micro.jsx`), `durationHistogram` (`model.js`), the compact `NeedsPanel` (one line per cause).
- The page is now: header, indicator band, then Needs you + timeline, answers, data quality, evidence by type, records by kind, time to ready, cases, pick-up, latest. Those sections are the earlier ones, unchanged except the heat table (now fixed layout so it cannot overflow a half-width panel); each is rebuilt in its own turn.
- Fix found on the way: `CountUp` now also reads the viewer's stored motion choice, because the shell writes `data-motion` to the page only after the first paint.

## Checked

- 77 tests in `src/v2` pass (new: `durationHistogram` bins and median bin, empty case, `DotRow` overflow, `Histogram` scale, `CountUp` with and without reduced motion; the page test now reads the six indicators).
- Screenshots looked at: 1440 × 900 hero light and dark, 1440 full page, 1024, 390 dark. The band and the Needs you / timeline row fit one 1440 × 900 viewport.
- Not run: `vite build`, Playwright suite, axe.

## Open

- Whether to add a map or a sparkline-bearing sixth-to-eighth indicator (needs real sites and a longer activity read: BACKEND_REQUESTS rows 19 and 33).
- The Needs you list leaves empty space under five rows beside the taller timeline; its section is redone next together with the timeline.
