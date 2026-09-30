# Dashboard: layout architecture (2026-09-30)

Owner verdict on the previous D3: a restyled pipeline-status block is not what a main dashboard is. A world-class dashboard shows **the investigation itself** (what is in the evidence, when things happened, what needs attention) as charts and figures side by side, with interaction, and is built so more can be added without redesigning. This file fixes the architecture. `dashboard-product-plan.md` says what the page is for; `dashboard.md` holds section specs; brief §6 (chart by question) and §11 (modern language) apply throughout.

## 1. What the study found
- Dashboards that work put the few most important figures first and the primary visual beside them, then drill down progressively ([NN/g via UX summaries](https://uxpilot.ai/blogs/dashboard-design-principles)); modern versions use a **bento grid** where tile size reflects importance, not data volume ([Bento Grid Dashboard Design 2026](https://www.orbix.studio/blogs/bento-grid-dashboard-design-aesthetics)).
- Analyst dashboards pair a **ranked breakdown with filters** and keep the chart usable as a control (cross-filtering: a chart never filters itself; active filters are removable chips) ([GitHub security overview](https://docs.github.com/en/code-security/security-overview), [Looker cross-filtering](https://docs.cloud.google.com/looker/docs/cross-filtering-dashboards)).
- A forensic tool's case dashboard is an overview of evidence sources and what was extracted, with per-source ingestion status ([Magnet AXIOM case dashboard](https://www.magnetforensics.com/resources/magnet-axiom-2-0-case-dashboard/)); the analyst's questions follow acquisition, analysis, then reporting ([IBM](https://www.ibm.com/think/topics/digital-forensics)).
- Read as search summaries (primary sites were unreachable from the build environment).

## 2. What the system can honestly show today (audit of the API)
| Visual | Real source | Status |
|---|---|---|
| Activity over time by record family | `POST /query/hybrid` with `template=activity_by_day`, no target: `records.activity_by_day[{activity_date, record_type, event_count}]`, unsampled | **available**, capped at 100 buckets (`maxHybridLimit`), so the UI labels a cap as partial (request 19 for the complete endpoint) |
| Evidence composition (record families) | `GET /collections/status` `record_families[].accepted_rows` | available |
| Readiness, review, ingestion accounting | `GET /collections/status` | available |
| Retry a failed source | `POST /evidence/{id}/reprocess` (needs `reason`, `Idempotency-Key`) | **exists**; UI to add |
| Top entities (contacts, places, plates) | `frequent_contacts`, `top_locations` refuse an empty target | needs request 19 |
| Call mix, peak hours | `call_type_breakdown`, `activity_by_hour` (whole case, CDR only) | available for CDR cases; later widget |
| Intake history, live events, data quality, reports | none | requests 3, 15, 16, 17 |
Nothing is drawn that a source above does not supply. Where a source is missing the widget shows an honest state, never a sample.

## 3. The architecture: shell, page, zones, widgets
```
AppShell
 └─ DashboardPage            owns scope (All cases | one case, in the URL), refresh, and the data hooks
     ├─ Z0 Command header    title, one-line briefing, freshness, Refresh, Add evidence
     ├─ Z1 Metric cards      four cards (done)
     ├─ Z2 Hero              [ Activity over time + highlights (8) ] [ Needs review by cause (4) ]
     ├─ Z3 Evidence          [ Evidence map: bubbles (5) ] [ Sources and rows flow (7) ]
     ├─ Z3b                  [ Readiness by case (7) ] [ Key entities (5, honest until request 19) ]
     ├─ Z4 Quality           ingestion accounting now; data quality when request 16 lands
     ├─ Z5 Case workbench    queue and inspector (redesign next)
     └─ Z6 Continue          recent questions, later recent reports
```
- **Grid:** 12 columns at 1280 px and up, 8 at tablet, 4 on phones; 16 px gaps, 24 px between zones. A widget declares its span per breakpoint; the page never hard-codes layout inside a widget.
- **Widget contract (maintainability):** each widget is one component that receives already-computed props and renders inside one shared frame (title as the question, actions, body, footer) with the four states handled the same way: loading skeleton, empty (one sentence), partial (says what is missing), error (says why, offers retry). Adding a widget is one file plus one line in the page composition; nothing else changes.
- **Data layer, separate from presentation:** fetching lives in hooks (`useConfiguredCaseOverviews`, `useCaseActivity`); shaping lives in pure functions in `lib/` (`caseActivity.js`, `dashboardCharts.js`) that are unit-tested; components only draw. A new backend endpoint touches one hook and one mapper.
- **Scope is global and in the URL** (`?scope=<caseId>`; absent means all cases), so a view can be shared and survives reload. Scope changes Z2 activity and Z3 families; cards and readiness always show the whole workspace.
- **One interaction language:** clicking a mark filters or drills to real records; the chart itself is never filtered by its own selection; active filters are removable chips with Clear; hover and focus are visible; every chart has an exact-values table and a text equivalent; motion is transform and opacity only and off under reduced motion.
- **Chart libraries:** ECharts (approved) for time and comparison charts, code-split; small purpose-built SVG for the bubble map. No other dependency.

## 4. The hero (Z2), in parallel by design
- **Activity over time (8 cols):** stacked bars per day, one colour per record family, interactive legend (toggle families), zoom slider, tooltip with exact counts per family, click a day to open Investigate with that day. States the span (first to last day) and the total. A 100-bucket cap is labelled "partial" with the reason.
- **Attention rail (4 cols):** the redesigned D3. The count and the worst case first, a mini breakdown, the top items with Review and Retry, and "All N" to the full list. It answers "what needs me" without owning the page.
- Cards (Z1) sit above so the figures and the visual read together: figures left-to-right, then the picture and the queue side by side.

## 5. Evidence map (Z3): the bubbles
Record families as packed circles, area proportional to accepted rows, colour from the categorical palette, count and share on the label, click filters the case queue and the activity chart to that family. Exact values are in a Table toggle. Bubbles are used because the question is "what is this case made of, at a glance"; precision lives in the labels and the table.

## 6. Build sequence
1. This turn: data hook and mappers, activity chart, attention rail, evidence map, layout, tests, request 19.
2. Next: Retry dialog on the rail (reprocess exists), scope in the URL, Z4 and Z5 redesign, then Ask bar and quick actions.
3. As backend lands: complete activity and entities (19), history (3), live events (15), data quality (16), reports (17).
