# NexusAI Phase 7.1 Populated Subscriber Runtime Acceptance

Date: 2026-08-04  
Verdict: **PASS — populated runtime, privacy, provenance, export, Agent Chat and UI accepted**

## Accepted boundary

Phase 7.1 is complete on the isolated synthetic collection
`phase7-subscriber-populated-acceptance-v1`. The protected governed collection
`records-demo-verified` was not modified. The two pre-existing legacy
subscriber-name collections were preserved and not cleaned up.

The accepted fixture SHA-256 is
`a5edc8f2658b12e8564b49560fe40959be397de6e086eaa1bfe2aa11bad912b0`.
Its retained lineage is:

- evidence: `1b1f27f7-d8cc-4dc1-b33e-758885e1ca8a`
- version: `cc83458d-e17d-4d66-98a6-a670d40467ab`
- batch/job: `d57c9b68-d34d-4134-9953-bcd2fc185d68`
- accounting: 15 input, 11 accepted, 1 duplicate, 3 rejected

## Deterministic acceptance

All six subscriber operations returned the exact populated oracle:

| Operation/template | Accepted result |
|---|---:|
| `subscriber.identity_lookup` / `subscriber_identity_lookup` | 2 cited rows |
| `subscriber.validity_timeline` / `subscriber_validity_timeline` | 2 cited rows |
| `subscriber.device_links` / `subscriber_device_links` | 2 cited rows |
| `subscriber.status_summary` / `subscriber_status_summary` | 6 groups totaling 11 rows |
| `subscriber.conflict_audit` / `subscriber_conflict_audit` | 2 conflict groups |
| `subscriber.reuse_candidates` / `subscriber_reuse_candidates` | 3 bounded review candidates |

Exact target normalization, validity ordering, conflict grouping, reuse review,
bounded pagination, source/evidence/row hashes, and no-results behavior passed.
Raw CNIC and subscriber-name targets fail closed before SQL/model execution.
Canonical responses and exports mask or omit protected subscriber fields while
the protected raw evidence remains intact for chain of custody.

## Runtime, export and interface gates

- Evidence preview redaction passed after hardening `evidence.go` and adding the
  focused accounting/preview regression coverage.
- CSV export passed:
  `C:\Users\sheik\Downloads\forensic-grid-2026-08-04-07-52-38.csv`.
- Audit JSON export passed:
  `C:\Users\sheik\Downloads\forensic-audit-2026-08-04-07-54-21.json`.
- Agent Chat returned the exact deterministic result; message ID
  `1785829727421117664`.
- The case-analysis UI passed at 390, 820, 1024 and 1440 px with no page-level
  horizontal overflow and no browser console warnings/errors.
- Runtime activation marker:
  `reports/runtime-activation-20260804/phase7.1-populated-acceptance.json`.

## Optional model acceptance

The Qwen 4B subscriber-status narrative was rejected because it added an
unsupported validity-overlap statement. The system failed closed and returned
the deterministic explanation. Request ID:
`e7ca74fd-ddc1-4f6a-9a3b-10717f22afc6`. This is a successful policy-fallback
gate, not an accepted model narrative.

Two additional model audits established the operating distinction:

- `data_quality`, request `07d1160d-3475-4498-a03b-1acf20b707e6`: accepted
  after factual review.
- `frequent_contacts`, request `ec79433d-c137-47dd-85de-d43d85e12bf1`:
  runtime validation passed but analyst factual review rejected the prose
  because its stated count range excluded two deterministic top-five rows.

The deterministic tables remained correct in all three cases.

## Verification

- combined family-adapter Python tests: 26 passed
- forensic API package: 222 specs registered, 220 passed, 2 skipped
- `go vet ./api/forensic_records`: passed
- deterministic direct-agent routing: passed
- React production build: passed (658 modules; only existing warnings)

The complete valid/accepted per-agent question list, reusable templates, target
rules, limitations and LLM classifications is in
`reports/nexusai-agent-query-catalog-phase7.1-20260804.md`.

## Result

Phase 7.1 is data-accepted and closed. Phase 7.2 Tower/Site Intelligence may
proceed as the next bounded evidence-family slice.

