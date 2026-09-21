# NexusAI UI state contract

Status: R2 source accepted  
Updated: 2026-08-06

## Authority order

1. Server-side authenticated tenant, permissions and governed resources.
2. URL identifiers for the selected case and route section.
3. Server-issued operation/request/resource identifiers.
4. In-memory presentation state.
5. Browser storage for preferences and unsaved convenience drafts only.

Lower-authority state may never override higher-authority state.

## State ownership

| State | Authoritative owner | Allowed client persistence |
| --- | --- | --- |
| User, tenant, roles, feature access | auth server/session | short-lived session mechanics only |
| Active case | governed case API plus URL `:caseId` | none outside the URL |
| Evidence, counts, readiness, jobs | forensic APIs | query cache keyed by tenant + case + resource revision |
| Query/result/report | case-bound API and request ID | unsaved draft text; accepted saved analysis is server-side |
| Branding | public branding API/configuration | bundled safe fallback |
| Theme/language/layout | user/browser preference | local storage permitted |
| Model/backend/agent state | administration APIs | no authoritative local cache |

## Case-context envelope

Every case request must resolve:

`tenant_id`, `case_id`, `case_revision`, `user_id`, effective
permissions, classification, evidence/readiness revision and request ID.
Collection and specialist bindings are server-derived from the case manifest,
not selected independently by ordinary analyst components.

## Fetch and transition rules

- Key requests by all authority-bearing identifiers.
- Abort or ignore superseded responses after case/route changes.
- Clear incompatible results before showing the next case as ready.
- Preserve last accepted content only when explicitly labeled stale/revalidating.
- Treat HTTP success with incomplete suboperations as `partial`, not `ready`.
- Require idempotency keys for future retryable mutations.
- Surface request ID, failed scope and retry eligibility without exposing secrets.

## Empty, loading and error truth

`loading` has unknown values, never zero. `empty` follows a completed
authoritative response. `unavailable` means missing accepted capability, not
missing case data. `forbidden` does not reveal resource existence.

`R4-PRE-01` implements one narrow part of this contract: Case Workspace
initialization no longer presents transient zero evidence/record counts. It is
source accepted, runtime pending and does not constitute R4 acceptance.

## Current-source risks

Chat and media histories, selectors, theme, navigation and other preferences use
browser storage. These keys may remain compatibility identifiers during R3, but
R6 must distinguish unsaved convenience history from audited saved analyses.
There is no global case provider; URL-bound case state remains the only accepted
selector until R4 deliberately introduces a scoped case context.
