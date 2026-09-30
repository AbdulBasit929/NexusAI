# Dashboard slice research — 2026-09-27

## Reopened design review after product-owner feedback

The first implementation was visually coherent but not operationally useful.
Measured from its own review captures, it rendered four equal-weight case cards,
repeated the same four counters in every card, placed the work action at each
card's foot, and ended with a large implementation-limit notice. At 390px the
page was 2,248px tall before any case detail was opened. It answered "what
exists?" but made "what can I do now?" slow to scan.

The replacement treats the Dashboard as a bounded command surface over
configured collections: compact readiness filters, a searchable work queue,
one state-derived action per case, explicit refresh, and browser-local recent
questions in a secondary rail. It does not add generic KPI cards or charts.

## Product and backend evidence inspected

- `GET /collections/status` is the only authoritative per-collection readiness source. It reports evidence totals, completed/in-flight/failed counts when available, accepted structured rows, families, recent jobs and recent evidence.
- There is no cross-case aggregate, assignment, priority, notification or team-activity endpoint. Independently fetched case summaries must remain visibly scoped to their case and must not be summed into portfolio claims.
- A collection begins when its first file is accepted. The primary new-work action therefore opens the existing first-evidence workflow; it does not imply a create-case endpoint.
- Recent questions and their optional display names are browser-local navigation aids. The exact submitted query is retained and is the value sent when reopened.

## External standards reviewed

- IBM Carbon, **Dashboards**: put the most important content first and remove content that distracts from interpretation. Applied as `Resume work` before browser-local history, without decorative charts or generic KPI tiles: https://carbon-website-git-pictograms-docs.carbon-design-system.vercel.app/data-visualization/dashboards
- Fluent 2, **Layout**: responsive layouts rearrange and reduce content at narrower widths while keeping a consistent spacing system. Applied through the existing token grid and a one-column card layout below 768px: https://fluent2.microsoft.design/layout
- GOV.UK Design System, **Complete multiple tasks**: expose task state in words and allow people to return to in-progress work. Applied as explicit `Ready`, `Processing`, `Needs review`, and `No evidence yet` states with state-derived next actions: https://design-system.service.gov.uk/patterns/complete-multiple-tasks/
- W3C WAI, **Use of Color** and design tips: status and interaction cannot depend on colour alone; controls and focus must remain identifiable. Applied as visible text state plus a redundant indicator, descriptive next-action links and existing focus tokens: https://www.w3.org/WAI/WCAG22/Understanding/use-of-color and https://www.w3.org/WAI/tips/designing/

## Comparable investigation products reviewed

- Elastic Security, **Detection & Response dashboard**: prioritises recent operational objects and makes each count/name a direct route to its working view. NexusAI adopts direct per-case next actions, but not alert/case totals because it lacks the corresponding aggregate endpoint: https://www.elastic.co/guide/en/security/current/detection-response-dashboard.html
- Elastic Security, **Investigate security events**: keeps investigation tools connected to evidence, timelines, cases and notes so context survives navigation. NexusAI therefore resumes at the exact case/question route and retains the submitted question, but does not claim collaborative notes or saved server history: https://www.elastic.co/docs/solutions/security/investigate
- Microsoft Defender XDR, **Incident queue**: the queue supports triage because its backend supplies correlation, severity, assignment, time range and a documented priority model. This is a useful negative control: NexusAI must not reproduce a priority queue, severity ranking or “assigned to me” panel without those data contracts: https://learn.microsoft.com/defender-xdr/incident-queue
- Splunk Enterprise Security, **Start investigations** and **analyst/team queues**: the working queue is backed by persisted investigations and findings. NexusAI can expose configured collection work and browser-local questions, but must label the latter local and cannot call them an analyst/team queue: https://help.splunk.com/en/splunk-enterprise-security-8/user-guide/8.1/mission-control/start-investigations-in-splunk-enterprise-security and https://help.splunk.com/en/splunk-enterprise-security-8/administer/8.6/mission-control/analyst-and-team-based-queues-in-splunk-enterprise-security
- Palantir Foundry, **Observability dashboards**: operators choose workflow-relevant signals and arrange them around how the team works. NexusAI uses only the four status counts returned for each configured collection and avoids generic visualisations that do not change an analyst decision: https://www.palantir.com/docs/foundry/observability/dashboards

## Deep review used for the rebuild

- IBM Carbon's current **Dashboard** guidance distinguishes presentation from
  exploration dashboards. It requires a strong importance hierarchy, fewer
  metrics, and search/sort/filter/drill-down for exploratory work. Applied:
  evidence-readiness filters operate the case queue, and only values that alter
  the next action survive. https://carbondesignsystem.com/data-visualization/dashboards/
- Microsoft Defender's current **Incident queue** makes search, filters, total
  scope, detail preview, and direct investigation routes first-class. Its
  priority score is backed by correlation, severity, asset criticality and
  threat signals. Applied: queue mechanics. Explicitly rejected: priority,
  severity, recommended actions and assignment because NexusAI has none of
  those sources. https://learn.microsoft.com/en-us/defender-xdr/incident-queue
- Splunk Enterprise Security 8.7 **Mission Control** begins with a refreshable,
  filterable analyst queue and then opens investigation detail. Its queue has
  persisted ownership, urgency, domain and workflow state. Applied: refresh,
  filter and direct-open mechanics. Rejected: "my queue", owner, urgency,
  disposition and team workload. https://help.splunk.com/en/splunk-enterprise-security-8/user-guide/8.7/mission-control/overview-of-mission-control-in-splunk-enterprise-security
- Elastic's current **Detection & Response dashboard** routes counts and names
  directly into filtered working views. Applied: readiness summaries are
  controls, not inert decoration. Rejected: trends and cross-case risk because
  NexusAI has no time-series or risk endpoint.
  https://www.elastic.co/docs/solutions/security/dashboards/detection-response-dashboard
- GOV.UK's researched **Complete multiple tasks** pattern says returning-session
  surfaces must group related actions, expose status, keep the status vocabulary
  small, and visually emphasize incomplete work over completed work. Applied:
  Needs review, Processing, Ready, No evidence, and Unavailable are the whole
  queue vocabulary; rows begin with verbs that match the available next step.
  https://design-system.service.gov.uk/patterns/complete-multiple-tasks/
- Fluent 2 **Layout** uses whitespace and grid area to encode importance, then
  reflows and progressively discloses metadata at narrow widths. Applied: a
  12-column desktop composition with the work queue dominant, a narrow recent
  work rail, and compact mobile rows with secondary metrics collapsed.
  https://fluent2.microsoft.design/layout

## Backend re-audit for the rebuild

`GET /collections/status` returns, per configured collection:

- authoritative evidence total, ready, in-flight and failed counts;
- authoritative job, accepted/rejected/duplicate-row, and missing-asset totals;
- record-family rollups;
- bounded recent evidence and recent ingest jobs, including timestamps and
  failure information;
- a server `generated_at` timestamp.

The UI may assemble these independent responses into a view labelled
**Configured case readiness**. It must not call that view a global portfolio,
assignment queue, priority queue, alert feed, or workload. Filters operate only
over the complete runtime-configured case list. Sorting failed evidence before
processing before ready is labelled evidence-readiness order, not investigative
priority.

No case name exists. The collection ID remains the raw identifier. No user,
assignee, role, classification, severity, deadline, last analyst action, shared
question history, or global case/activity endpoint exists. Those fields remain
absent.

## Resulting design decisions

- First question answered: “What should I do next?” Each case chooses one primary next action from returned state: review failed evidence, investigate ready evidence, view processing, or add first evidence.
- No browser-derived readiness percentage. The backend reports counts, not a percentage; the UI presents those counts directly.
- No cross-case totals, ranking, risk, urgency or fabricated attention count.
- Browser-local recent questions follow the authoritative case list and are labelled `This browser only`; a renamed shortcut continues to show and submit the original query.
- Cards remain bounded case objects, not a wall of generic metrics. Evidence state and the next action are primary; family rows and accepted structured rows are secondary context.
- The research products justify priority/severity/assignment views only when those systems have explicit backends for them. Their absence in NexusAI is a contract boundary, not a visual omission to fill with client inference.
- Replace equal card grids with one semantic case-work list so analysts compare
  cases by the same columns and action location.
- Summary values become filter buttons with pressed state and exact counts from
  successfully resolved configured cases. A separate unavailable count prevents
  request failures from becoming zeroes.
- Search filters only configured collection IDs. No hidden fuzzy inference.
- Manual refresh reissues all case-status requests. In-flight cases poll every
  15 seconds while visible; no fabricated progress percentage is shown.
- Each row states why it is in its state, exposes the latest backend-reported
  evidence activity when available, and supplies exactly one primary next step.
- Browser-local recent questions are secondary, visibly local, and reopen the
  exact submitted query. They never masquerade as shared case activity.
- Replace the large "not connected" banner with a compact scope disclosure:
  configured collections only; assignment and priority are not available.
