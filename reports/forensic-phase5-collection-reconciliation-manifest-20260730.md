# Phase 5 Collection Reconciliation Manifest

Generated read-only at `2026-07-30T11:46:52Z` from the running local stack.

## Safety declaration

- This inventory performed GET-only reads against LocalAI collection, source,
  records-batch, agent-config, and forensic-status endpoints.
- Collections merged: **0**.
- Collections archived: **0**.
- Collections deleted: **0**.
- Database rows or schemas changed: **0**.
- Model or agent configuration changed: **0**.

The source implementation added in Phase 5 exposes a richer read-only manifest
at `GET /api/v1/forensics/cases/{case_id}/manifest`. That source was not deployed
during this audit, so canonical `forensic.records`, entity, audit-event, and
vector cardinalities remain unresolved in this live snapshot. Reports are
currently generated synchronously and have no immutable persistence table.

## Current binding truth

- Governed source default: `records-demo-verified`.
- Running `Forensic_Records_Analyst` stored collection:
  `nexusai-structured-demo-v2-20260730`.
- Phase 5 Agent Chat now adds an explicit URL/request case override and defaults
  the UI to the governed case. The stored binding was not mutated because that
  is retained runtime state and requires operator approval.
- All 20 collections report zero configured collection sources.
- All 20 collections report zero LocalAI legacy record batches. The structured
  row counts below are sidecar accepted-row accounting, not legacy batch counts.

## Per-collection inventory

| Collection | Class | KB entries | KB assets | Evidence | Accepted rows | Jobs | Failed jobs | Agent binding | Analyst selector action |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- |
| `records-demo-verified` | candidate pilot | 8 | 8 | 6 | 9,250 | 4 | 0 | no stored binding | default/preserve |
| `records-demo` | legacy demo | 6 | 2 | 8 | 10,000 | 8 | 6 | no | visible; reconciliation warning |
| `nexusai-structured-demo-20260730` | legacy demo | 0 | 8 | 9 | 37 | 8 | 0 | no | visible; reconciliation warning |
| `nexusai-structured-demo-v2-20260730` | legacy demo | 0 | 12 | 12 | 51 | 12 | 0 | **yes** | visible; reconciliation warning |
| `Forensic_Records_Analyst` | accidental/empty candidate | 0 | 0 | 0 | 0 | 0 | 0 | no | hidden; verify references |
| `forensic_records_analyst` | accidental/empty candidate | 0 | 0 | 0 | 0 | 0 | 0 | no | hidden; verify references |
| `forensic-document-archive-acceptance-20260727-v2` | acceptance/system | 5 | 5 | 5 | 0 | 0 | 0 | no | hidden/retain |
| `forensic-document-archive-acceptance-20260727` | acceptance/system | 4 | 3 | 4 | 0 | 0 | 0 | no | hidden/retain |
| `forensic-ingress-audit-20260724` | acceptance/system | 39 | 26 | 27 | 23 | 10 | 1 | no | hidden/retain |
| `forensic-media-headers-acceptance-20260727` | acceptance/system | 4 | 4 | 4 | 0 | 0 | 0 | no | hidden/retain |
| `forensic-media-metadata-acceptance-20260724` | acceptance/system | 2 | 2 | 2 | 0 | 0 | 0 | no | hidden/retain |
| `forensic-mpeg-container-acceptance-20260727` | acceptance/system | 3 | 3 | 3 | 0 | 0 | 0 | no | hidden/retain |
| `forensic-packet-capture-acceptance-20260727` | acceptance/system | 3 | 3 | 3 | 0 | 0 | 0 | no | hidden/retain |
| `forensic-pakistan-cdr-acceptance-20260724` | acceptance/system | 1 | 1 | 1 | 4 | 1 | 0 | no | hidden/retain |
| `forensic-pakistan-cdr-validation-20260724` | acceptance/system | 1 | 1 | 1 | 4 | 1 | 0 | no | hidden/retain |
| `forensic-pakistan-structured-goldens-20260727` | acceptance/system | 2 | 2 | 2 | 11 | 2 | 0 | no | hidden/retain |
| `forensic-phase2-complete-acceptance-20260727` | acceptance/system | 20 | 20 | 20 | 34 | 8 | 0 | no | hidden/retain |
| `forensic-phase2a-acceptance-20260724` | acceptance/system | 21 | 21 | 21 | 20 | 7 | 0 | no | hidden/retain |
| `forensic-sqlite-inventory-acceptance-20260727` | acceptance/system | 2 | 2 | 2 | 0 | 0 | 0 | no | hidden/retain |
| `forensic-warning-propagation-acceptance-20260727` | acceptance/system | 1 | 1 | 1 | 0 | 0 | 0 | no | hidden/retain |

## Reconciliation findings

1. `records-demo-verified` is the strongest current pilot: 9,250 accepted rows,
   eight KB entries, eight KB assets, six evidence items, four jobs, and no
   failed jobs.
2. `records-demo` contains more accepted rows (10,000) but has six failed jobs,
   only two KB assets for eight evidence items, and therefore requires repair
   and provenance review before any canonical selection or merge proposal.
3. Both `nexusai-structured-demo*` collections have structured/evidence state
   but zero current KB entries. Their sidecar assets cannot be treated as proof
   that the LocalAI KB still contains source entries.
4. The running analyst's stored binding targets the v2 legacy demo. Phase 5
   request-bound scope prevents the Case Workspace from silently inheriting it,
   but an operator should review the stored configuration after manifests and
   real-reference scans are approved.
5. The two analyst-named collections are confirmed empty across the surfaces
   visible in this audit. They remain hidden cleanup candidates, not authorized
   deletion targets.
6. Acceptance/system collections retain reproducibility value and are hidden
   from the default analyst selector. No retention expiry is inferred.

## Unresolved surfaces and next approval gates

| Surface | Current status | Required next action |
| --- | --- | --- |
| Vector counts/checksums | unresolved | add an exact count/checksum adapter for the active KB backend |
| Canonical `forensic.records` counts | source endpoint implemented, not live-probed | deploy after approval, then export v1 manifests |
| Entity and audit-event counts | source endpoint implemented, not live-probed | deploy after approval, then compare manifests |
| Immutable reports | not persisted | implement report registry/versioning in a later bounded slice |
| Alias migration | proposed only | obtain operator approval after content/checksum comparison |
| Archive/soft delete | not implemented or invoked | define retention policy and human approval workflow |
| Permanent deletion | prohibited | explicit separate human approval is mandatory |

