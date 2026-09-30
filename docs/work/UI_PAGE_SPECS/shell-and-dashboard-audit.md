# Audit: application shell and Dashboard (2026-09-29)

Evidence: `apps/investigation-workspace/design-review/ui-redesign-20260930/before/` (1440 and 375 px, light and dark, real API: 2 cases, 55 sources, 53 ready, 2 failed). Zero console errors, zero overflow and zero duplicate IDs in all eight captures. The defects below are design and information-design defects, not runtime faults.

Verdicts: **keep**, **improve**, **replace**, **remove**, **add**.

## Shell

| Part | Job | Finding | Verdict |
|---|---|---|---|
| Header gradient bar | Identity, search, appearance | The colour anchor the owner likes. Search is scoped and good. No global "Investigate" entry, no live status. | keep the gradient; add Investigate entry and a live processing indicator |
| Sidebar | Navigation and case context | Light `#f3f8fd` rail (dark `#0b1929`) is a different hue family from the header. Only four global links; "Investigate" only exists inside a case. No badges. The rail ends short of the viewport bottom in the 1440 capture (visible cut-off under Settings). | replace: palette, information, height |
| Canvas | Page background | `#eaf1f8` is a generic blue-grey unrelated to the teal header, so header, rail and page read as three products. | replace with the chosen palette |
| Case strip | Active case, readiness | Fine idea. Depends on the same status call. | improve with breadcrumbs |
| Breadcrumbs | Location | Present only on the gallery. | add everywhere |
| Mobile drawer | Navigation < 1024 | Works and traps focus. | keep, restyle |
| Command palette | Find case, source, question | Works, scoped, keyboard-first. | keep, restyle |

## Dashboard sections

| # | Section | Question it answers | Source | Finding | Verdict |
|---|---|---|---|---|---|
| 1 | Page header | Where am I? | none | Title "Case readiness" is a status word, not the job. "Add evidence" links to /cases/new. | improve |
| 2 | Evidence status band (two tiles) | How big is my workspace? | `caseIds` config, `summary.evidence_total` | Tiles repeat the same figure as the donut centre (55). "Configured cases only" eyebrow is scope jargon. | replace with one KPI strip; each KPI links to its rows |
| 3 | Evidence readiness donut | Is my evidence usable? | `GET /collections/status` summary | Donut with 3 parts is legitimate, but the ring is 96% one colour, so it says "fine" and hides the 2 failures; the count buttons beside it look like cards inside cards. | replace with a stacked bar plus a "needs attention" list that names the failed sources |
| 4 | Ingestion quality bar | Did ingestion drop rows? | summary accepted / duplicate / rejected rows | Bar is 98.7% accepted; duplicate 302 and rejected 7 are invisible. Three filter buttons filter *cases*, which is not what the labels promise. | replace with a per-case stacked breakdown plus counts |
| 5 | Structured evidence mix | Which record families dominate? | `record_families[].accepted_rows` | Ranked bars are the right encoding. Ranks 5–8 render as empty tracks (values 16, 10, 8 against 13,642). Colour per bar is arbitrary and inconsistent with other pages. Clicking filters the case list, not the records. | improve: sorted bars with a minimum visible mark, table toggle, click-through to the records |
| 6 | Case list | Which case needs me next? | same status responses | Readiness ordering is honest. Rows repeat a mini bar plus three counts. | improve |
| 7 | Selected-case inspector | What is wrong here and what next? | status, `recent_jobs`, `recent_evidence`, `missing_kb_assets` | Best section. Tabs clip at 375 ("Processing 4" cut off). "Evidence represented" pills duplicate the bars. Latest activity shows a raw numeric filename. | improve |
| 8 | Recent questions | Resume my work | browser-local history | Empty here, honest wording. | keep, move up beside Investigate |
| 9 | About this dashboard | Scope note | none | Explains scope in a disclosure. | replace with a persistent scope chip |

## Missing from the page
1. A global **Investigate** entry point (ask a question without first choosing a case).
2. A **needs attention** list across cases (failed sources, retained-asset gaps), each row linking to the exact evidence item.
3. A first-class **processing** state: honest and cancellable, never a fake percentage.
4. Real time series and a real evidence census: blocked on BACKEND_REQUESTS 3 and 8.

## Chart rulings for this page (brief section 6)

| Question | Encoding | Data | Drill-through |
|---|---|---|---|
| Is my evidence usable? | 100% stacked bar (ready, processing, failed) with totals | summary evidence counts | segment opens the Evidence list filtered by state |
| Which sources need attention? | table, not a chart | failed and asset-gap rows | evidence detail |
| Which record families dominate? | sorted horizontal bar | `record_families` | Overview families / Evidence filter |
| Did ingestion drop rows? | stacked bar per case (accepted, duplicate, rejected) | per-case summary | case overview |
| What changed over time? | withheld | BACKEND_REQUESTS 3 | designed empty state |

## Next steps, blocked on the owner's palette pick
Shell rebuild (tokens, rail, Investigate entry, breadcrumbs, live badges), then the Dashboard rebuild on ECharts (code-split), then the spec `dashboard.md`.
