# Activity — research and implementation ruling

Date: 2026-09-27  
Surface: case-scoped Activity  
Authority: `docs/ux/NEXUSAI_PRODUCT_UX.md`, with the live API and code taking precedence

## Analyst job

Activity answers one bounded question: **what did I recently ask in this browser, and what recent evidence-processing state did this collection report?** It is a working record for resuming work. It is not a custody log, a team audit trail, or an authoritative event history.

## Endpoint and state audit

The current surface can use only:

- `GET /collections/status` — collection summary plus bounded `recent_evidence` and `recent_jobs` samples.
- browser-local question history — up to 50 saved questions per collection; answer/outcome and update time are retained locally.
- current-tab activity — richer transient entries that retain the exact submitted evidence-family scope until the tab is closed.

No read endpoint supplies actor identity, tenant, role, team activity, custody events, exports, policy changes, complete retention, or an authoritative audit sequence. Consequently:

- evidence rows are labelled **Recent collection status**, never “audit events”;
- saved questions are labelled **Saved in this browser**;
- a current-tab question can show its retained scope, while an older browser entry must say **Scope not retained**;
- missing timestamps remain **Time not reported**;
- no user, ownership, priority, chronology, or completion fact is inferred.

## External research applied

1. [NIST SP 800-171 Rev. 3](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/800-171r3/NIST.SP.800-171r3.html) treats audit records as accountable records containing the event type, time, location, source, outcome, and associated identities. Ruling: this surface cannot claim “audit history” without those fields.
2. [NIST SP 800-92, Guide to Computer Security Log Management](https://www.nist.gov/publications/guide-computer-security-log-management) defines logs as records of events and frames collection, storage, analysis, and disposal as a managed lifecycle. Ruling: a bounded recent sample cannot be presented as a complete log.
3. [Microsoft Defender XDR incident investigation](https://learn.microsoft.com/en-us/defender-xdr/investigate-incidents) exposes a unified Activities timeline only because manual and automated actions have recorded origin, category, provider, trigger, and status. Ruling: borrow the fast filtering and resumable-action pattern, not the unsupported promise of a unified timeline.
4. [Microsoft Defender device entity timeline](https://learn.microsoft.com/en-us/defender-xdr/entity-page-device) makes event time, action, user, and associated entities first-class columns. Ruling: show only columns backed by NexusAI data; do not manufacture an actor column.
5. [Carbon Design System — Data table usage](https://carbondesignsystem.com/components/data-table/usage/) recommends a toolbar for search/filter actions and tables when users need to locate a specific item in a larger set. Ruling: use one dense, searchable work list with stable fields rather than a stack of cards.
6. [Microsoft Sentinel incident investigation](https://learn.microsoft.com/en-us/azure/sentinel/incident-investigation) describes its timeline as a diary of logged relevant events and provides search/filtering. Ruling: NexusAI may use the interaction pattern only when every entry visibly declares its actual source and retention boundary.

## Interaction and information architecture

- Compact scope notice first: “Working record, not audit history.”
- Search across question, answer, filename, and identifier.
- Source, evidence-family, and outcome filters use the existing accessible custom listbox.
- A single chronological list exposes Time, Activity, Source, Outcome, Scope, and one next action.
- Question actions reopen Investigate with the exact question. Evidence actions open the evidence detail route.
- Filtered zero is recoverable with **Clear filters** and never confused with an empty collection.
- Loading and API failure retain browser-local items and name only the failed evidence scope.

## Accessibility and responsive rules

- The list has a programmatic name and each item has a heading.
- Filter results are announced with `role="status"`.
- Outcome and source are always text, never colour alone.
- All controls remain at least 44 CSS pixels.
- The row layout collapses to a single-column reading order on narrow screens; no page-level horizontal scroll is introduced at 375 px.
- Dates are explicitly formatted in UTC to avoid an unlabeled browser-timezone interpretation.

## Explicit non-features

Until an authoritative activity endpoint exists, this surface does not show team activity, chain of custody, export history, user identity, ownership, policy decisions, or a “complete” audit count. Internal request IDs, planner states, templates, operations, SQL, and measures are never rendered on the analyst surface.
