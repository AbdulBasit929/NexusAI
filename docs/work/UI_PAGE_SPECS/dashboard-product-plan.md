# Dashboard: product plan (2026-09-30)

The Dashboard is the front door of the product, so it is planned as an investigator's **command centre**, not a set of restyled panels. Owner direction: cover the whole product, its usefulness and productivity, and add backend functionality where that makes the page better. Every section obeys `UI_REDESIGN_BRIEF_20260929.md` §11 (metric cards, little text, interaction) and §6 (chart chosen by the question). Section-level specs stay in `dashboard.md`; this file decides **what the page is for** and which backend features it needs.

## What the research says
- A forensic investigation moves through acquisition or ingestion, analysis, then review and reporting ([IBM](https://www.ibm.com/think/topics/digital-forensics), [Graylog](https://graylog.org/post/the-phases-of-the-digital-forensics-investigation-process/), [Open University](https://www.open.edu/openlearn/science-maths-technology/digital-forensics/content-section-4.1)). A home page is most useful when it follows that flow: what is still coming in, what can I trust, what do I analyse next, what can I report.
- Case-management dashboards for analysts put status, ownership, timeline context and real-time visibility in one place so triage does not stop for lookups, and give different roles their own views connected by drill-downs ([Datadog Cloud SIEM cases](https://www.datadoghq.com/blog/cloud-siem-cases/), [Sumo Logic](https://www.sumologic.com/blog/how-using-cloud-siem-dashboards-and-metrics-for-daily-standups-improves-soc-efficiency)). NexusAI has no ownership or role model yet, so those parts stay absent rather than invented.
- Productivity home pages combine a command or ask bar, recent items and quick actions, ranked by current context first ([Setproduct command palette teardown](https://www.setproduct.com/blog/command-palette-ui-design-guide)). "Jump back in" beats a static list.
- Exception-first and honest freshness were already applied to D1 to D3.
Sources are search summaries only (primary sites were unreachable from the build environment).

## The four questions, in order
1. **What needs me now?** D1 briefing, D2 cards, **D3 Needs review** (done). Next: act on it, not only see it (retry a failed source).
2. **Can I trust the evidence?** D4 readiness, D5 what the evidence contains, D6 ingestion and data quality (rows kept, duplicated, rejected, time coverage).
3. **Where do I continue?** D7 case portfolio, D8 continue (recent questions, recent cases), recent reports.
4. **What can I do right now?** An **Ask** bar (the product's core action) and quick actions: add evidence, ask, generate a case brief, open a case.

## Target layout (top to bottom)
| # | Section | Answers | Data today | Needs backend |
|---|---|---|---|---|
| D1 | Briefing header | what needs me, how fresh | yes (done) | 12 for one atomic read |
| D2 | Metric cards | how big is the problem | yes (done) | 12 |
| **D0** | **Ask bar**: one input, a case scope chip, suggested questions from the case's curated corpus, opens Investigate with the question | what can I do now | yes: routes to `/cases/:id/investigate?question=`, curated questions from `GET /query/capabilities` | 11 (all cases) |
| D3 | Needs review hero (done) + **Retry** on failed sources | act on problems | yes (view) | **14** (retry), 13 (complete list) |
| D4 | Readiness by case | can I search it | yes (done) | 12 |
| D5 | What the evidence contains: ranked bars now, treemap by modality then type when a census exists | what is in here | families only (bounded to structured records) | 8 (census) |
| **D5b** | **Ingestion over time**: area or bars of jobs per day by status, brush to zoom, click a day to open those jobs | is intake healthy, what changed | no (only the last few jobs) | 3 (history), 15 (live) |
| D6 | **Data quality**: per family, kept, duplicate, rejected rates, first and last event time covered, link to the rejected rows | can I trust it | totals only | **16** |
| D7 | Case portfolio: sortable cards with readiness, last activity, pinned first, friendly names | which case next | readiness only | 9, **18** (names, status) |
| D8 | Continue: recent questions and cases, recent reports | where was I | browser-local questions | 5, **17** (reports) |
| **D9** | **Activity feed**: uploads, completions, failures, questions, exports (a custody trail) | what changed | no | 10, **15** |
| Quick actions | Add evidence (drag-drop target), Ask, Generate case brief (`POST /reports/generate` exists) | do it now | brief: yes | 17 (history) |

D0 and the case-brief action need no new backend and can ship first. D5b, D6, D9 are the charts the owner asked for: they become real only when the backend provides history, quality and events, so until then they are absent or show their honest "not available yet" state, never a sample.

## Chart choices (brief §6)
- D3: ranked stacked bars (magnitude across cases, composition inside). Done.
- D4: 100% stacked bar per case (parts of a whole). Done.
- D5: sorted horizontal bars now; treemap (hierarchy: modality, then type) once a census exists.
- D5b: area or stacked bars over real timestamps, brush to zoom, click drills to the jobs.
- D6: per-family table with inline rate bars (tiny rates are invisible on a shared baseline, so the rate is a number first).
- D9: an event list, not a chart; a small volume strip only if the volume is meaningful.

## Build order
1. **D0 Ask bar and quick actions** (no backend). 2. Redo D4, D5, D6 to the §11 language with what exists. 3. D7 portfolio and D8 continue. 4. As each backend request lands: D3 Retry (14), D5b (3), D6 quality (16), D9 feed (10 and 15), reports (17), names (18).

## Rules kept
Real API data only; a missing feature is absent or shows an honest state; no owner, priority or assignment until an authoritative source exists; privacy is server-side.
