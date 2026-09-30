# NexusAI analyst interface — UI/UX research and surface plan

**Status:** Phase 0 research deliverable for review  
**Date:** 2026-09-27  
**Scope:** `apps/investigation-workspace/**` only  
**Implementation status:** research only; no component, style, API, runtime, or retained-data change is authorized by this document

## 1. Executive conclusion

NexusAI should feel like a calm evidence instrument: case scope is always visible, source truth is never visually subordinate to generated prose, and the interface makes the difference between **none**, **not known yet**, **partially known**, and **not available** impossible to miss.

The strongest patterns found are consistent across forensic guidance, eDiscovery tools, enterprise design systems, accessible-table standards, and bounded conversational analytics:

1. Preserve and expose the source context. Microsoft Purview keeps review activity inside a case and offers source, extracted-text, annotation, metadata, tagging, and export views rather than replacing evidence with a summary. Its source view aims at the truest available native representation; its text view adds line references and hit highlighting. [Microsoft Purview: group and view review-set items](https://learn.microsoft.com/en-us/purview/edisc-review-set-view)
2. State limitations instead of implying completeness. NIST notes that not all evidence may be discovered and that recovered material can contain extraneous data; SWGDE requires examiners to report only what the data represents and remain mindful of limitations. [NIST digital-investigation foundation review](https://www.nist.gov/publications/digital-investigation-techniques-nist-scientific-foundation-review), [SWGDE guidance for presenting digital evidence](https://www.swgde.org/documents/published-complete-listing/23-q-001-best-practices-for-personnel-presenting-digital-evidence-in-legal-proceedings/)
3. Keep global and local navigation visibly different. Carbon treats the header as global/system navigation and the left panel as local product navigation; persistent local navigation is appropriate when people switch frequently between more than five destinations. [Carbon global-header pattern](https://carbondesignsystem.com/patterns/global-header/), [Carbon UI-shell left panel](https://carbondesignsystem.com/components/UI-shell-left-panel/usage/)
4. Keep conversational analysis attached to analytical artifacts. Power BI returns an insight and a visual, preserves the applied filters under “How … arrived at this,” and links back to the report; Tableau keeps its conversational agent beside the worksheet so the result remains an editable visualization rather than becoming a chat bubble. [Power BI Copilot data-question tutorial](https://learn.microsoft.com/en-us/power-bi/create-reports/tutorial-copilot-power-bi-discover-data), [Tableau Agent data exploration](https://help.tableau.com/current/online/en-us/web_author_einstein.htm)
5. Preserve semantic tables at scale. W3C strongly prefers native HTML tables where possible, defines `aria-rowcount`/`aria-rowindex` for windowed data, and requires `aria-sort` on the active sorted header. USWDS adds practical guidance: short headers, right-aligned comparable numbers, table captions, focusable scroll containers, and live sort announcements. [W3C table pattern](https://www.w3.org/WAI/ARIA/apg/patterns/table/), [W3C grid and table properties](https://www.w3.org/WAI/ARIA/apg/practices/grid-and-table-properties/), [USWDS table guidance](https://designsystem.digital.gov/components/table/)

This does **not** justify a visual restart. The existing tokens, `PageHeader`, case-overview hook, case cards, threaded Investigate surface, and semantic `VirtualizedTable` are the foundation. The redesign should improve hierarchy, state truth, and task flow one reviewable surface at a time.

## 2. Authority and non-negotiable product rules

This plan follows, in order:

- the 2026-09-27 product-owner brief;
- `docs/ux/NEXUSAI_PRODUCT_UX.md`;
- `docs/design/nexusai-app-shell-specification.md`;
- `docs/design/nexusai-brand-system.md`;
- the current API surface and live behavior;
- the tested design tokens and current components.

The following are design inputs, not later acceptance details:

- Case → Evidence → Question remains the information architecture.
- Evidence families are filters, never primary navigation.
- A case is currently a collection; there is no case-entity endpoint.
- No auth, session, membership, tenant/role, classification, or team-history endpoint exists.
- Local question history must say **This browser only**.
- No cross-case aggregation endpoint exists. A list of independently loaded cases must not be described as a portfolio total or organization-wide view.
- No real processing percentage exists. Display state and stated counts, never a decorative progress bar.
- Every number shown must be returned by the API. Client calculations may only format or partition API-stated values without inventing a new total or completeness claim.
- Provenance is primary content: `● Source record`, `◐ Candidate observation`, `○ Low-confidence observation`. Glyph and text accompany colour.
- CNIC masking remains server-side. The UI neither unmasks nor invents masking.
- The runtime `collectionId` must not be set when more than one case is available because it pins every request to one collection while the URL can show another case.

## 3. Research synthesis

### 3.1 Evidence, provenance, and legal defensibility

SWGDE defines digital forensics as collection, examination, analysis, and reporting while preserving information integrity and chain of custody. It warns against overextending a conclusion beyond the data and examiner expertise. [SWGDE legal-proceedings guidance](https://www.swgde.org/documents/published-complete-listing/23-q-001-best-practices-for-personnel-presenting-digital-evidence-in-legal-proceedings/)

SWGDE's error-mitigation guidance says a tool must not report artifacts that do not exist, group unrelated items, or alter meaning, and it recommends describing tool limitations and unsuitable conditions. That supports NexusAI's visible distinction between deterministic source records and model-derived observations. [SWGDE confidence and error-mitigation guidance](https://www.swgde.org/documents/published-complete-listing/12-q-001-swgde-establishing-confidence-in-digital-and-multimedia-evidence-forensic-results-by-error-mitigation-analysis/)

Microsoft Purview's review-set design reinforces four useful patterns:

- the case is the durable scope;
- filters determine the exact working subset;
- the analyst can move from list to native source, extracted text, annotations, and metadata;
- exports state whether they contain selected, filtered, or all review-set documents and can preserve source path structure and item reports. [Purview review sets](https://learn.microsoft.com/en-us/purview/edisc-features-components), [Purview export options](https://learn.microsoft.com/en-us/purview/edisc-review-set-export)

**NexusAI decision:** every result must retain its applied case and evidence scope, and every export action must state the exported scope. A summary, sample, recent-N list, or filtered result never inherits the language of completeness.

### 3.2 Empty, partial, processing, failed, and unavailable are different facts

Carbon separates no-data, user-action/no-results, and error-management empty states. It recommends placing the state exactly where data would have appeared and giving a contextual next action. [Carbon empty-state pattern](https://carbondesignsystem.com/patterns/empty-states-pattern/)

For NexusAI, “empty” is narrower than generic design-system usage. It is legal only after the authoritative request resolves successfully and the returned scope contains zero matching items. The complete state vocabulary is:

| State | What is known | Required treatment | Prohibited wording |
|---|---|---|---|
| Loading | No authoritative response yet | Skeleton or calm loading label; retain known surrounding context | “No evidence” |
| Ready | Authoritative request resolved with usable content | Content plus stated scope | Any broader completeness claim |
| Empty | Authoritative request resolved with zero matches in the stated scope | Name the scope and offer an explicit filter change | “Nothing exists” when only the filter is empty |
| Partial | Some requested artifacts resolved; a named scope failed, is excluded, or is still unavailable | Retain completed artifacts and name the missing scope | “Complete”, “all”, or a silent omission |
| Processing | API says queued/running or gives ready versus in-flight counts | State real counts and offer a refresh/revisit action | Invented percentage or estimated finish time |
| Failed | A request or evidence item failed | Reason when supplied, error reference, and a supported recovery route | “Finished processing” |
| Forbidden | Service refused access | Plain-language access state without leaking hidden content | Empty-state styling |
| Unavailable | Required contract or capability does not exist | Say which capability is not connected and what still works | Disabled fake control or plausible placeholder |
| Abstained | The system deliberately withheld an answer because it could not support one | Calm clarification/limitation treatment and concrete next options | Generic error styling |

Dynamic changes must be announced without moving focus unnecessarily. W3C recommends `role="status"`/polite live regions for status updates that assistive-technology users need to hear. [W3C status-message technique](https://www.w3.org/WAI/WCAG21/Techniques/aria/ARIA22)

### 3.3 Dense enterprise shells

Carbon's UI shell uses a persistent header for global/system context and a left panel for local product navigation. It also warns against unbounded user-created content in shell navigation. [Carbon global-header pattern](https://carbondesignsystem.com/patterns/global-header/)

**NexusAI decision:**

- Header = NexusAI identity and genuinely global utilities.
- Case context band = the active case and case switch.
- Rail = global destinations separated from case destinations.
- Recent questions may remain in the expanded rail only because the list is capped at four, subordinate to navigation, and labelled **This browser only**. The full, unbounded history belongs in Investigate/Activity, not the shell.
- Collapsed icons receive accessible names and a tooltip on both hover and keyboard focus. A pointer-only tooltip is insufficient.

Density comes from short labels, stable alignment, compact rows, and progressive disclosure—not tiny type, low contrast, or stacked cards.

### 3.4 Conversational analysis without the chat downgrade

Microsoft's human-AI guidelines say to make capabilities and limitations clear, support correction, scope services when uncertain, explain behavior, and preserve recent interaction context. [Microsoft Research: Guidelines for Human-AI Interaction](https://www.microsoft.com/en-us/research/project/guidelines-for-human-ai-interaction/)

Power BI returns summaries with footnote references to visuals, supports opening the source report, returns a visual for data questions, and exposes filters under an explanation disclosure. Tableau Agent lives in an analysis worksheet and leaves generated visualizations directly usable after the conversational step. [Power BI Copilot tutorial](https://learn.microsoft.com/en-us/power-bi/create-reports/tutorial-copilot-power-bi-discover-data), [Tableau Agent](https://help.tableau.com/current/online/en-us/web_author_einstein.htm)

**NexusAI decision:** a question and answer form a thread, but each answer is an analytical document with state, finding, result table, claim-level provenance, limitations, follow-ups, and export. Conversation supplies continuity; it does not determine the visual anatomy.

### 3.5 Accessible tables at forensic scale

W3C recommends native `<table>` markup when possible and `aria-sort` on the currently sorted header. When windowing means the DOM contains only part of the result, `aria-rowcount` communicates the full API-stated count and `aria-rowindex` communicates each rendered row's position. [W3C table pattern](https://www.w3.org/WAI/ARIA/apg/patterns/table/), [W3C large-table properties](https://www.w3.org/WAI/ARIA/apg/practices/grid-and-table-properties/)

USWDS recommends predictable columns, right-aligned additive numbers, clear captions/attribution, focusable horizontal-scroll regions, and live announcements for sort changes. [USWDS table](https://designsystem.digital.gov/components/table/)

TanStack's official virtualization guidance makes the same architectural separation already present in NexusAI: the table owns rows, columns, headers, sorting, and filtering; virtualization decides which indexes render. It is not a replacement for server-side pagination when the browser does not hold the full dataset. [TanStack virtualization guidance](https://tanstack.com/table/latest/docs/framework/lit/guide/virtualization)

**NexusAI decision:** retain `VirtualizedTable` and real table semantics. Pin the first column from 768–1279px, keep sort buttons inside `<th>`, announce sort changes, use tabular numerals for quantities, isolate identifiers with `<bdi>`, and never claim a window or sample is the full result. Server pagination remains blocked because no cursor-paginated records endpoint exists.

## 4. Surface-by-surface proposal

### 4.1 Navigation rail

#### What it must let a person do

- Distinguish global workspace destinations from the active case's destinations at a glance.
- Move among Dashboard, Cases, global Activity, and Settings without losing orientation.
- Move among Overview, Evidence, Investigate, Timeline, and case Activity within the current case.
- Switch case without first navigating to the Cases page; the URL changes and incompatible case-scoped state clears before a new request.
- Resume one of the four most recent local questions, pin it, or rename the local thread where that browser-only function already exists.
- Collapse the rail while retaining understandable icon-only navigation, focus-visible tooltips, and active states.

#### What it must never let a person believe

- That recent questions are synchronized, shared, or audit history. They are browser-local.
- That a case switch preserves filters, answers, or source selection from another case.
- That global Activity and case Activity have the same scope.
- That an icon without a visible label/tooltip is safe to infer.

#### Proposed layout

1. Rail header: `Navigation`, collapse control, and—when expanded—a compact active-case switch trigger that invokes the single shared case switcher.
2. `Workspace` group: Dashboard, Cases, Activity, Settings.
3. Strong divider and `Active case` group: Overview, Evidence, Investigate, Timeline, Activity.
4. `Recent questions · This browser only`: maximum four rows, each with question text, pin state, rename overflow action, and a link to the thread.
5. Collapsed state: 72px rail; icons remain in the same order; group dividers remain; tooltip appears on hover and focus; recent-question content is not squeezed into mystery glyphs.
6. Mobile/tablet: the same information order in the existing modal drawer, with focus trap, Escape close, background inertness, and focus return.

#### Deliberately not doing

- No family navigation.
- No unlimited history in the shell; Carbon specifically cautions against unbounded user content in side navigation.
- No team/shared-history language until a session/history endpoint exists.
- No tenant, role, notification count, or policy badge without a source endpoint.
- No new navigation library or shell rewrite.

### 4.2 Header and case context band

#### What it must let a person do

- Identify NexusAI and the current case without reading page content.
- Open Quick find and jump to a configured case, an evidence item returned by `/evidence`, a route, or a recent browser-local question.
- Switch case through one shared control.
- Change the designed theme through Display preferences.
- Understand the difference between global identity, case context, and the page-level `PageHeader`.

#### What it must never let a person believe

- That a displayed case identifier is a curated case name when no case entity exists.
- That the product knows their user, tenant, role, classification, notifications, or policy when no endpoint supplies those values.
- That Quick find searches server-side case history or every record; its searchable domains must be named.
- That a recent-N evidence response is a total.

#### Proposed layout

1. Global header: NexusAI lockup at inline start; Quick find and Display at inline end. No repeated tagline on dense work surfaces.
2. Case context band: `Active case` label, raw case/collection identifier isolated as an identifier, real summary from `/collections/status`, and the case-switch trigger.
3. Quick find dialog: grouped results (`Cases`, `Evidence`, `Routes`, `Recent questions`), each with scope text; loading and unavailable states per group; keyboard arrows/Enter/Escape; focus return.
4. Task band: every page uses `PageHeader` for breadcrumb, eyebrow, one `h1`, description, metadata, and page-specific actions.

#### Deliberately not doing

- No search across data rows; no endpoint supports it.
- No fake command for unsupported actions.
- No avatar/user menu that implies authentication.
- No case title manufactured from a slug.
- No duplicate case switchers competing in header and rail; both entry points invoke one stateful control.

### 4.3 Dashboard

#### What it must let a person do

- Decide what to resume or do next within five seconds.
- Open one configured case and see that case's evidence composition, ready/processing/failed state, and supported next action.
- Start evidence intake through the real first-upload workflow.
- Understand when attention data or cross-case activity is not connected.

#### What it must never let a person believe

- That independently loaded case cards form an authoritative cross-case portfolio aggregate.
- That “needs attention” counts exist when there is no Dashboard aggregation endpoint.
- That all cases are known beyond the configured case identifiers.
- That a processing item is complete or that a failed item is merely finished.

#### Proposed layout

1. `PageHeader`: Dashboard, concise purpose, primary `Add evidence / Start a case` action using the actual upload-created collection workflow.
2. `Resume work`: configured cases rendered through existing `CaseCard`; each card loads its own `/collections/status` and labels its scope.
3. Inside each case card, one compact composition line: API-stated evidence total, completed, in-flight, and failed; family/modality labels only where returned.
4. `What you can do next`: contextual links derived from real state—open failed evidence when failures exist, continue Investigate when evidence is ready, or add evidence when the collection is empty.
5. Explicit dependency note: `Attention data is not connected` with a short explanation that organization-wide prioritization needs a cross-case endpoint.

#### Deliberately not doing

- No hero, welcome marketing, decorative chart, or generic KPI tiles.
- No summed evidence, failure, or activity totals across cards.
- No “urgent” or risk ranking computed in the browser.
- No fake recent-team activity.
- No duplicate status-fetching logic outside `useCaseOverview`/`summariseCase` and `CaseCard`.

### 4.4 Case overview

#### What it must let a person do

- Understand the case's composition, readiness, exclusions/failures, and material data-quality caveats in ten seconds.
- Separate evidence-file counts from accepted structured-row counts.
- Open the Evidence catalog already filtered to a state/family and move directly to Investigate.
- See the failed item rather than accepting a misleading “finished” summary.

#### What it must never let a person believe

- That completed + failed means all evidence succeeded.
- That a recent evidence sample is the whole collection.
- That file count and row count are interchangeable.
- That missing classification, member, or role data exists.

#### Proposed layout

1. `PageHeader`: raw case identifier, readiness summary, `Ask about this case`, and `Add evidence` actions.
2. Primary readiness strip: API `summary.evidence_total`, completed, queued/running, failed, and not-processed as distinct labelled states. Failed always has a route to the failed row; no unsupported Retry button.
3. Evidence composition: two explicitly named sections when data exists—`Evidence files` by modality/classification and `Structured records` by curated family/accepted rows. Do not combine their units in one chart.
4. Ingest quality: accepted, rejected, duplicate, and ambiguous counts only when returned, with concrete explanatory sentences and source scope.
5. Data-quality notes: first-class panel, not a footnote; each note names the affected family/count only when supplied by `/collections/status`.
6. Next action: a restrained route into Investigate, carrying no silent family widening.

#### Deliberately not doing

- No four equal KPI cards with decorative accent rules.
- No percentage readiness unless the API states a percentage.
- No charts that require deriving totals absent from the response.
- No members/roles panel until a case-membership contract exists.
- No retry/reprocess control without an endpoint.

### 4.5 Evidence catalog and filters

#### What it must let a person do

- Browse the authoritative evidence inventory once, filter by family/status, search by available metadata, inspect processing failures, add data, compare supported evidence, and open an item at its viewer.
- Understand whether a count refers to evidence files or structured records.
- See that the table is filtered without interpreting zero as case-wide absence.
- Use the table at 10,000 rows with native semantics, keyboard access, sorting, numeric alignment, and stable first-column context.

#### What it must never let a person believe

- That `recent_evidence` is the complete catalog.
- That `Tower 0` files means there are no tower rows in the case.
- That items shown in both a processing panel and catalog are separate evidence.
- That a windowed DOM is a partial dataset when the API supplied the full set—or conversely that a server sample is complete.

#### Proposed layout

1. `PageHeader`: Evidence, API-stated evidence total, Add data action.
2. Compact readiness summary driven by `/collections/status.summary`; it displays totals and state counts, not item rows. Selecting a state filters/scrolls the authoritative catalog.
3. Filter bar labelled `Evidence files`: search plus exact family chips with explicit `files` units, e.g. `Images · 23 files`. `All · 43 files` uses `summary.evidence_total`, not the length of `recent_evidence`.
4. If a family has zero classified files but `/collections/status.record_families` reports rows for that family, show: `No standalone Tower evidence file; tower records are present in processed structured evidence.` Show this only when both facts are in the responses.
5. One authoritative catalog table from `GET /evidence`: name, file family/modality, size, status, and only metadata the endpoint returns. Failed rows include the returned reason and a supported next route, not a fake Retry.
6. `VirtualizedTable` remains a real `<table>`. Sort buttons live in headers; the sorted header owns `aria-sort`; numeric quantities align to the inline end with tabular figures; the scroll container is keyboard-focusable; sort/filter result changes use a polite live region.
7. Evidence detail/viewer remains the place for source-native content, locators, overlays, transcript cues, and lineage.

#### Deliberately not doing

- Remove the duplicate item enumeration from the processing panel; the catalog is authoritative.
- Do not replace the table with cards or div-based rows.
- Do not use client pagination as if it were cursor pagination.
- Do not hide zero-count families without explaining the active filter vocabulary.
- Do not invent Added by/Added when where the endpoint does not return them.
- Do not expose parser, model, backend, template, operation, or raw filesystem path.

#### Required correction to the measured completeness defect

`ProcessingWait` must derive the collection summary from `summary.evidence_total`, completed, in-flight, failed, and not-processed fields. `recent_evidence` may be used only as a visibly labelled recent sample. The sentence `All 20 evidence items have finished processing` is prohibited for the measured 43-item case and is logically prohibited whenever failures are nonzero. The four states remain distinct: queued/running, completed, failed, not-processed.

### 4.6 Investigate thread

#### What it must let a person do

- Ask within a visible case and evidence-family scope, preserve the thread in this browser, refine scope, inspect the result artifact, verify every cited claim, open the exact source locator, export through the supported report path, and ask a grounded follow-up.
- Understand at a glance whether the turn is answered, zero, partial, processing, clarification/abstention, unsupported, or failed.
- Distinguish deterministic source facts from candidate and low-confidence observations.

#### What it must never let a person believe

- That fluent prose is stronger than the result grid or source record.
- That an observation is a deterministic fact.
- That an answer covering ready evidence covers the entire case.
- That a missing locator is a usable citation.
- That localStorage history is server audit history.
- That clarification or abstention is a system failure.

#### Proposed layout

1. Thread heading and scope chips remain outside individual turns; scope persists and never silently widens.
2. Each analyst question is compact context, not a decorative chat bubble.
3. Each answer is a composed analytical block in fixed order:
   - state label with glyph and plain-language description;
   - finding headline with the measured quantity typographically dominant;
   - result artifact (`VirtualizedTable`, metric, comparison, or timeline);
   - claim-level citation markers and source list/rail at the governed threshold;
   - provenance legend: `● Source record`, `◐ Candidate observation`, `○ Low-confidence observation`;
   - `How this was derived`, collapsed and written for an analyst;
   - limitations, only when real;
   - 2–4 server/semantic-layer-supported follow-ups;
   - export/open-source actions.
4. Citation preview works on hover and keyboard focus and contains source file, exact row/page/time/region locator, timestamp when available, and an excerpt of at most 200 characters. An unopenable locator renders as source context, not a citation control.
5. Abstention receives the most deliberate treatment: calm heading, specific reason, original question, 2–4 concrete choices or a manual scope path, and no red error styling.
6. New answers use a polite live region; keyboard focus remains predictable and is not forced into streaming content.

#### Deliberately not doing

- No assistant avatar, chat-bubble theater, typing dots, decorative streaming, or purple AI treatment.
- No backend/model/agent/operation/SQL/token/confidence-threshold language.
- No fabricated follow-up prompts.
- No source rail for fewer than three distinct sources.
- No auto-widened filter after zero results.
- No claim of export success until `/reports/generate` returns it.

### 4.7 Activity, Timeline, New case, Settings, and Admin

#### What they must let a person do

- **Activity:** inspect the local question/history record with honest scope and reopen a question.
- **Timeline:** understand that chronology is unavailable when the service does not provide a timeline contract, while preserving the intended filter vocabulary for later.
- **New case:** create a collection through first evidence upload, with a valid identifier and clear processing handoff.
- **Settings:** manage appearance and understand this browser's storage footprint and real connection facts.
- **Admin:** show only capabilities that are explicitly enabled and backed; otherwise remain absent from analyst navigation.

#### What they must never let a person believe

- Activity is shared custody/team audit history.
- Timeline events have been inferred from unrelated timestamps or client-side sorting.
- A case entity was created before the first upload was accepted.
- Browser-local settings are server-side account preferences.
- Admin authorization exists merely because a route component exists.

#### Proposed layout

- All pages use `PageHeader`, one `h1`, one primary task, and the seven-state route contract.
- **Activity:** `This browser only` state banner; accessible named list/table of question, outcome, case, time, and reopen action using only stored fields.
- **Timeline:** composed unavailable state naming the missing normalized event endpoint; explain what Evidence and Investigate can do now. No disabled filters.
- **New case:** identifier/name step plus Add-data dropzone; make the lifecycle explicit: `The collection is created when its first file is accepted.` Processing state uses server-stated counts only.
- **Settings:** Appearance, browser-local data with clear/delete controls, and connection facts. Destructive local clear requires precise scope and confirmation.
- **Admin:** gated by an explicit runtime capability; with no supported admin contracts, show a bounded unavailable explanation or omit the route rather than render controls.

#### Deliberately not doing

- No team member, role, classification, custody, or audit events without endpoints.
- No fake timeline, disabled timeline filter, or inferred chronology.
- No pre-upload success state for New case.
- No account/profile editor without auth/session contracts.
- No system health, model, backend, queue, or template controls on analyst surfaces.

## 5. Cross-surface composition rules

### Hierarchy

- Global header: identity and global utilities.
- Case band: active case and real case state.
- Page header: breadcrumb, one `h1`, description, metadata, page actions.
- Main task: evidence and analyst action.
- Secondary disclosure: technical or explanatory detail.

### Visual density

- Extend existing tokens; never hard-code component colour values.
- Preserve Noto Sans / Noto Sans Arabic / Noto Sans Mono roles.
- Use typography, alignment, and spacing before adding containers.
- One dominant surface per page; cards are reserved for independently actionable objects such as cases, not for every metric.
- Keep 44px minimum touch targets even though the base control token is 42px.
- Motion stays at or below 200ms and disappears under reduced motion.

### Provenance and confidence

| Truth state | Required label | Meaning |
|---|---|---|
| Deterministic source | `● Source record` | Direct source row/content or deterministic aggregate with openable contribution provenance |
| Derived candidate | `◐ Candidate observation` | Model/tool-derived observation that must not be stated as source fact |
| Weak derived candidate | `○ Low-confidence observation` | Observation whose supplied confidence band requires stronger caution |

The glyph, label, and description travel together. Colour reinforces but never creates the distinction.

### Responsive behavior

- 1440: persistent expanded rail, wide data surface, optional source rail for 3+ sources.
- 1280: persistent rail; preserve analytical width before secondary prose.
- 1024: persistent collapsible rail; first table column pinned.
- 768/820: modal drawer; tables scroll within a labelled focusable region; first column pinned where the table remains tabular.
- 375/390: drawer, stacked task header, source disclosure below answer, no page-level horizontal scroll.

### Accessibility acceptance

- Skip link reaches the primary task.
- Drawer traps focus, closes with Escape, and returns focus.
- All icon-only controls have accessible names and hover/focus explanations.
- Status changes use polite live regions unless urgent intervention is truly required.
- Sort direction is visible and programmatic.
- Mixed English/Urdu evidence retains `dir="auto"` and identifier isolation.
- Focus rings and state contrast continue to pass the existing token tests.

## 6. Measured defects and pre-fix audit plan

The following defects are accepted as measured input and should not be re-diagnosed before correction:

1. `ProcessingWait` states a complete 20-item result over a recent-N sample while the authoritative summary says 43 total, 42 completed, 1 failed.
2. Family-filter counts are file counts but are presented beside row-based family counts without a unit distinction; `Tower 0` conflicts with tower rows present in status.
3. Processing and catalog surfaces enumerate the same evidence without explaining authority.

After Phase 0 review, and before fixing additional defects, perform the requested browser sweep and write the findings into the first surface change note:

| Width | Routes | Checks |
|---|---|---|
| 1440, 1280, 1024, 768 | Dashboard, Cases/New case, Overview, Evidence, Evidence detail, Investigate states, Activity, Timeline, Settings, Admin when gated | console errors; page overflow; duplicate IDs; exactly one `h1`; landmark order; keyboard reachability; visible focus; sticky/pinned table behavior |

The sweep is observational first. Findings are recorded before fixes so a reviewer can distinguish measured defects from design preference.

## 7. Implementation sequence after review

Each item is a separate, revertible change with focused tests and browser evidence:

1. Navigation rail.
2. Header and case context band.
3. Dashboard.
4. Case overview.
5. Evidence catalog and filters, including the three measured correctness defects.
6. Investigate thread.
7. Activity, Timeline, New case, Settings, Admin.

For every surface:

- identify API fields and state ownership before editing;
- render loading, ready, empty, partial, error, forbidden, unavailable fixtures;
- add behavior tests for each new count/state claim;
- keep `tokens.vitest.js` and semantic-table tests unchanged or stronger;
- run unit and relevant Playwright suites;
- verify against the live API only after fixture tests pass;
- report what changed, why, what was measured, and which endpoint gaps remain;
- do not deploy or rebuild runtime services without product-owner approval.

## 8. Conflicts found and rulings

1. **Carbon warns against unbounded content in side navigation; the brief asks for recent questions in the rail.** Keep only the capped four-row browser-local resume list in the expanded rail. Full history stays out of the shell.
2. **Generic empty-state patterns group unavailable/error with empty.** NexusAI does not. Empty is an authoritative zero; unavailable and failed remain separate states.
3. **Conversational products often center prose/chat.** NexusAI centers the evidence artifact and provenance. Thread continuity is retained, bubble-first presentation is rejected.
4. **Generic dashboards encourage aggregate KPIs.** No cross-case aggregate exists, so Dashboard shows independently scoped case summaries and an explicit missing-contract notice.
5. **Virtualization examples sometimes use div/grid markup.** NexusAI retains real `<table>` semantics and uses ARIA row metadata only to describe windowing.
6. **Forensic products may expose tool/version detail prominently.** NexusAI keeps that off ordinary analyst surfaces under UX §9; analyst-visible provenance names the source and locator, while restricted technical lineage remains progressively disclosed only where authorized.

## 9. Sources reviewed

### Forensic and evidence-management practice

- [NIST — Digital Investigation Techniques: A Scientific Foundation Review](https://www.nist.gov/publications/digital-investigation-techniques-nist-scientific-foundation-review): incompleteness, extraneous recovery, and changing artifact meaning.
- [SWGDE — Presenting Digital Evidence in Legal Proceedings](https://www.swgde.org/documents/published-complete-listing/23-q-001-best-practices-for-personnel-presenting-digital-evidence-in-legal-proceedings/): integrity, chain of custody, clear reporting, and limits.
- [SWGDE — Establishing Confidence through Error Mitigation](https://www.swgde.org/documents/published-complete-listing/12-q-001-swgde-establishing-confidence-in-digital-and-multimedia-evidence-forensic-results-by-error-mitigation-analysis/): error sources, tool limitations, and avoiding meaning-changing output.
- [Microsoft Purview — eDiscovery features and review sets](https://learn.microsoft.com/en-us/purview/edisc-features-components): case-scoped search, filter, view, tag, analysis, and export.
- [Microsoft Purview — review-set viewers](https://learn.microsoft.com/en-us/purview/edisc-review-set-view): native source, text/line references, annotations, metadata, and bounded AI summary limitations.
- [Microsoft Purview — review-set export](https://learn.microsoft.com/en-us/purview/edisc-review-set-export): selected/filtered/all scope and item reports.
- [Magnet AXIOM official overview](https://www.magnetforensics.com/products/magnet-axiom/): timeline and connection analysis as first-class investigation views. Used only as product-pattern evidence, not as a UI specification.
- [Cellebrite Guardian official overview](https://cellebrite.com/en/guardian-how-it-works/): case/evidence lifecycle, chain of custody, status visibility, and collaboration. Used only as product-pattern evidence.

### Enterprise shells and state design

- [Carbon — global header pattern](https://carbondesignsystem.com/patterns/global-header/): global/local hierarchy, persistent shell, responsive movement, and unbounded-content warning.
- [Carbon — UI-shell left panel](https://carbondesignsystem.com/components/UI-shell-left-panel/usage/): persistent local navigation and tested keyboard/screen-reader behavior.
- [Carbon — empty states](https://carbondesignsystem.com/patterns/empty-states-pattern/): no-data, user-action, and error-management distinctions with contextual actions.

### Conversational analysis and human-AI behavior

- [Microsoft Research — Guidelines for Human-AI Interaction](https://www.microsoft.com/en-us/research/project/guidelines-for-human-ai-interaction/): capability boundaries, correction, disambiguation, explanation, and recent context.
- [Power BI — Copilot data-question tutorial](https://learn.microsoft.com/en-us/power-bi/create-reports/tutorial-copilot-power-bi-discover-data): answers paired with visuals, source-report links, filter disclosure, and references.
- [Tableau — Explore data with Tableau Agent](https://help.tableau.com/current/online/en-us/web_author_einstein.htm): conversation embedded in an analytical workspace, with the visualization remaining directly usable.
- [Microsoft Security Copilot application guidance](https://learn.microsoft.com/en-us/copilot/security/rai-faqs-security-copilot-agents): cited sources, human verification, domain limits, and incomplete/outdated response risk.

### Accessible data at scale

- [W3C — Table pattern](https://www.w3.org/WAI/ARIA/apg/patterns/table/): native table preference and semantic requirements.
- [W3C — Grid and table properties](https://www.w3.org/WAI/ARIA/apg/practices/grid-and-table-properties/): total row/column metadata for partially rendered DOM and `aria-sort`.
- [W3C — Sortable table example](https://www.w3.org/WAI/ARIA/apg/patterns/table/examples/sortable-table/): header buttons, sort state, target size, and focus visibility.
- [USWDS — Table component](https://designsystem.digital.gov/components/table/): numerical alignment, captions, responsive scrolling, sort announcements, and accessible structure.
- [TanStack — virtualization guidance](https://tanstack.com/table/latest/docs/framework/lit/guide/virtualization): separation of table state from rendered-window selection and limits of client virtualization.
- [W3C — Status messages](https://www.w3.org/WAI/WCAG21/Understanding/status-messages): announcing dynamic changes without disruptive focus movement.
- [WCAG 2.2](https://www.w3.org/TR/wcag/): focus visibility, target size, structure, contrast, and status-message acceptance baseline.

## 10. Review gate

Phase 0 is complete when this document is reviewed. No implementation surface begins before that review. The first authorized implementation slice is **4.1 Navigation rail**, followed by the required pre-fix browser sweep findings relevant to that surface.
