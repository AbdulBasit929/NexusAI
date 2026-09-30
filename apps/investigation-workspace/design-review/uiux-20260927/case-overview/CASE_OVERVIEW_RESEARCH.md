# Case Overview slice research — 2026-09-27

## NexusAI contract and endpoint audit

- `GET /collections/status` supplies authoritative collection-wide counts for evidence total, completed, failed and in-flight; ingest totals for accepted, rejected and duplicate structured rows; missing knowledge-base assets; structured record-family totals; and explicitly bounded `recent_jobs` / `recent_evidence` samples.
- The previous surface summed `recent_jobs` for quality metrics. That sample is limited by the endpoint and cannot support collection-wide claims. Quality statements now use the top-level summary only.
- There is no case entity, curated case name, case status, severity, assignee, classification, comment, membership or retry/reprocess endpoint. These fields and controls remain absent.
- Evidence modality composition is not available as an aggregate in this response. The overview does not infer it from `recent_evidence`; the authoritative Evidence catalog remains the route for file-level review.

## Design systems and investigation products reviewed

- Elastic Security case summary: summary metrics are tied to attached case objects and response data; editable attributes appear only because Elastic has persisted case fields and APIs. NexusAI applies the summary-first hierarchy but omits title/status/severity/assignee UI: https://www.elastic.co/guide/en/security/current/cases-open-manage.html
- Elastic case details: related information is organised into purposeful sections and collapsible attributes, preserving room for the primary case content. NexusAI uses one readiness panel, one structured-record section and one first-class quality section rather than equal decorative cards: https://www.elastic.co/docs/explore-analyze/cases/manage-cases
- Microsoft Defender incident overview and Evidence/Response: evidence state is actionable and pending/failed work remains visible. NexusAI keeps processing, failed and not-processed evidence distinct and routes review to the actual Evidence catalog; it does not show remediation actions because no such endpoint exists: https://learn.microsoft.com/en-us/defender-office-365/step-by-step-guides/how-to-prioritize-manage-investigate-and-respond-to-incidents-in-microsoft-365-defender
- Carbon dashboard guidance: order information by importance and remove distractions that do not aid interpretation. Applied as readiness → quality caveats → structured composition → next action, without charts or four equal KPI cards: https://carbon-website-git-pictograms-docs.carbon-design-system.vercel.app/data-visualization/dashboards
- W3C WAI: state remains expressed in labels and descriptions, not colour alone, with semantic headings and definition lists preserving relationships: https://www.w3.org/WAI/WCAG22/Understanding/use-of-color and https://www.w3.org/WAI/WCAG21/Understanding/info-and-relationships

## Resulting design decisions

- The dominant object is the evidence-readiness strip, not the collection slug or a slide-sized title.
- Counts remain in their native units: evidence files are separate from structured rows.
- `not processed` is derived only as the non-negative remainder of the four authoritative evidence counts; it is not merged into processing or failure.
- Data-quality sentences use collection-wide summary counts. Recent-N job rows are not aggregated.
- The two productive actions are backed by existing routes: Ask about this case and Add evidence. No retry, assignment, severity, export or collaboration action is displayed.

