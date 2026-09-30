# Timeline — research and implementation ruling

Date: 2026-09-27  
Surface: case-scoped Timeline  
Authority: `docs/ux/NEXUSAI_PRODUCT_UX.md`, with live routes and response contracts taking precedence

## Analyst job

Timeline should answer **what happened, when, across the evidence in this collection**, and let the analyst open the exact source for every event. NexusAI cannot currently answer that question as a first-class view, so this slice must make the limitation legible and route the analyst to narrower supported work.

## Endpoint and capability audit

The registered forensic routes include collection status, evidence inventory/detail/content, query capabilities, hybrid query, case query, and report generation. There is no `GET` route for a normalized case-wide timeline or event feed.

- `GET /collections/status` can report actual evidence totals, readiness and represented structured families. These are inventory facts, not a chronology.
- `GET /query/capabilities` supplies collection-specific, backend-curated family labels, availability and suggested questions. Some supported families offer narrower timeline/history/sequence questions.
- `POST /query/hybrid` can answer those curated questions in Investigate with the existing citation and abstention contract.
- `recent_evidence.created_at`, job timestamps and `generated_at` are lifecycle metadata. They must never be substituted for source-event time.

The frontend therefore projects only safe fields from the capability payload: family label, availability, record type and curated suggested question. It does not render adapter, model, operation, readiness-attestation, corpus, policy, or other internal fields.

## External research applied

1. [Magnet Forensics — A Tale of Two Timelines](https://www.magnetforensics.com/resources/s2e5-a-tale-of-two-timelines/) describes forensic timelines as unified, event-driven chronologies built by artifact extraction and correlation. Ruling: a list of upload or processing dates is not a substitute.
2. [Microsoft Sentinel entity pages](https://learn.microsoft.com/en-us/azure/sentinel/entity-pages) makes the timeline a story of entity-related events within an explicit time frame, with event-type filters. Ruling: the eventual NexusAI view needs actual mapped entities and typed events, not only dates.
3. [Microsoft Sentinel incident investigation](https://learn.microsoft.com/en-us/azure/sentinel/incident-investigation) calls the incident timeline a diary of logged relevant events in the order they happened, with drill-down to underlying details. Ruling: every future row needs an openable source locator.
4. [Microsoft Defender advanced hunting results](https://learn.microsoft.com/en-us/defender-xdr/advanced-hunting-query-results) renders a timeline only when results contain a recognized timestamp field and meet a meaningful event threshold. Ruling: timeline visualization is conditional on trustworthy event data, never decorative.
5. [Microsoft Graph timelineEvent](https://learn.microsoft.com/en-us/graph/api/resources/security-timelineevent?view=graph-rest-beta) separates event time, details, result, source and type. Ruling: the missing NexusAI projection needs these semantic roles plus evidence family, identifiers and locator.
6. [NIST — System Time](https://www.nist.gov/glossary-term/40821) distinguishes system-clock time in computer forensics. Ruling: source time must retain its stated basis; UI display must not silently equate it with ingestion time.
7. [USWDS site alert guidance](https://designsystem.digital.gov/components/site-alert/) supports a static status notice for unavailable service/content. [W3C ARIA22](https://www.w3.org/WAI/WCAG21/Techniques/aria/ARIA22) requires dynamically updated status information to be programmatically announced with its context. Ruling: use a composed unavailable state and polite, atomic status for the asynchronously loaded availability summary.

## Information architecture

1. Page header says **Timeline** and describes the intended source-event chronology without implying it exists.
2. Dominant unavailable state: **A reliable source chronology is not available**. It explains that processing dates were deliberately excluded.
3. **What is available now** uses real collection status for evidence total, ready count, and represented structured families. No total is shown until the request resolves.
4. **Continue the investigation** offers Evidence and Investigate as clear actions.
5. When query capabilities resolve, **Supported chronology questions** shows only backend-curated questions for collection-present capabilities. Each action opens Investigate with the exact question and supported family scope.
6. **What a complete timeline needs** explains analyst-facing requirements: source event time, event meaning, involved identifiers, and an openable evidence location. It does not expose implementation terminology.

## States and accessibility

- Timeline remains `unavailable` until a case-wide event contract exists.
- Collection status independently renders loading, ready, empty, partial/error/forbidden as appropriate; failed status never removes the core unavailable explanation.
- Capability suggestions are progressive enhancement and disappear safely when the endpoint cannot project them.
- No filters are shown because there are no timeline results to filter.
- Actions are real links with 44px targets. No disabled or placeholder control is rendered.
- One `h1`; logical properties; both themes; no page-level overflow at 375px.

## Endpoint still required

An authoritative, paginated case timeline read endpoint must return normalized source-event time (including precision/time basis), curated event label/type, family, involved identifiers, evidence ID, and an openable locator for every event. It also needs explicit ordering behavior for equal, missing, or conflicting timestamps. Until then, the first-class cross-family timeline stays unavailable.
