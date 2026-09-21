# Investigation Workspace investigation model

Date: 2026-09-07  
Scope: current source plus read-only live API reconciliation

## Decision

The header's **Investigation** selector is truthful for governed scope switching,
but it is not a full investigation lifecycle manager.

- `GET /api/v1/forensics/cases` returns authorized, selectable case views.
- `ActiveCaseContext` verifies a requested case against that registry and binds
  Ask, Evidence and Activity to its case/collection identifiers.
- The public forensic route set has no create, rename, archive or delete case
  operation. The case view is projected from an existing agent collection by
  `classifyForensicCase`; by default `case_id == collection_id`.
- Therefore the premium UI keeps the selector and does not invent lifecycle
  controls or imply that an analyst can manage investigation records.

## Current live registry

The read-only live registry returned three analyst-visible selectable cases:

1. `nexusai-breadth-acceptance-20260820`
2. `nexusai-forensic-demo` (current registry default)
3. `nexusai-multimodal-product-acceptance`

The registry also reported six hidden legacy/system/test collections. Visibility
and selection remain server-authoritative.

## Origin of Multimodal Product Acceptance

`nexusai-multimodal-product-acceptance` was created as the explicitly approved
retained acceptance collection for the unified multimodal program. The living
checkpoint records that it began with zero evidence on 2026-08-23 and was
populated by the guarded acceptance executor under the accountable actor
`nexusai-breadth-acceptance-operator`. Later directives explicitly required
reuse of this existing collection. It now appears in the analyst registry
because the case endpoint projects authorized collections into selectable case
views; it is not a separately persisted investigation object.

## Conversation and Activity semantics

**New conversation** creates and selects a new case-scoped browser-local
conversation. It does not delete the prior local conversation and does not call
any governed-history mutation endpoint. **History** continues to show the
server-authoritative case-analysis journal. Clearing or starting a local view
must never be represented as deleting Activity.

## Clarification semantics

The retained `show frequent contacts` result asks for a number or subscriber,
but carries no candidate list of its own. The same governed history endpoint
contains a completed `forensics.entity_activity` result. The UI may offer only
the distinct phone/subscriber values in that retained result as convenience
choices; locations and unrelated entity types are excluded. If that governed
result is absent, the clarification remains text plus its existing safe follow-
up guidance—no identifier is fabricated and no background query is submitted.

## Product boundary

The truthful model is:

`authorized collection -> governed case projection -> active investigation scope`

Full investigation lifecycle management remains a future API/data-model slice,
not a UI-only feature.
