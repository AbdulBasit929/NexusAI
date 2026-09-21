# NexusAI Phase 5A Contracts and Registry Foundation

Date: 2026-07-30  
Scope: source-only; no retained runtime, database, evidence, model, or agent mutation

## Outcome

Phase 5A now has a shared `forensics.platform/v1` catalog, typed Go query/response
contracts, a Python adapter lifecycle protocol, compatibility bindings for the
current CDR and generic-tabular worker paths, and authenticated read-only registry
discovery endpoints.

The legacy worker still performs parsing, normalization, persistence, row
accounting, entity indexing, KB materialization, audit, and completion updates.
The compatibility registry changes only adapter selection and normalization
dispatch ownership; it invokes the same accepted normalizer functions. Existing
`/query/hybrid` request and response JSON is unchanged.

## Completed behavior

- Versioned adapter descriptors for `nexusai.adapter.cdr` and
  `nexusai.adapter.generic_tabular`.
- Ordered lifecycle contract: detect, validate, plan, extract, normalize, derive,
  persist, index, verify, capabilities.
- Eleven deterministic CDR/generic operation descriptors with typed input/output
  contract IDs and citation requirements.
- Model-role references for exact structured work, planner, synthesis, embedding,
  extraction, reranking, OCR, ASR, vision, and TTS.
- Manifests for the case orchestrator, required evidence-domain specialists, and
  the existing `Forensic_Records_Analyst` compatibility alias.
- Typed `forensics.query-plan/v1` Go contract covering scope, families, entities,
  timezone-aware time range, filters, measures, grouping, sorting, limit, tools,
  and clarifications.
- Typed `forensics.enterprise-response/v1` Go contract separating exact findings,
  cited semantic evidence, confidence-labeled inference, model interpretation,
  unsupported work, limitations, actions, tables, visualizations, and trace.
- Authenticated read-only source routes:
  - `GET /api/v1/forensics/adapters`
  - `GET /api/v1/forensics/operations`
  - `GET /api/v1/forensics/agents`
  - `GET /api/v1/forensics/contracts`

## Explicitly pending

- The new discovery routes are source-complete and not deployed.
- Only CDR and generic-tabular are compatibility-wrapped. The other structured
  families remain in the shared worker and await individual extraction slices.
- Specialist manifests do not create configured runtime agents. Only the existing
  `Forensic_Records_Analyst` remains configured in the live system.
- The case orchestrator does not yet delegate work.
- Existing `/query/hybrid` does not yet emit `forensics.enterprise-response/v1`;
  its accepted compatibility output remains unchanged.
- No public LocalAI proxy, generated Swagger update, UI, SDK, database migration,
  model selection UI, case unification, or collection-governance change is part
  of this slice.

## Reconciliation

- Branch: `codex/forensic-hybrid-checkpoint-20260723`.
- HEAD/base: `40717b83510c08db25dc26b9d6674bf46db363ac`.
- Worktree remained intentionally dirty, unstaged, uncommitted, and unpushed.
- Local read-only endpoints showed LocalAI ready, forensic API healthy, NATS
  healthy, and worker metrics available.
- Installed model IDs remained `qwen_qwen3-4b-instruct-2507` and
  `qwen3-embedding-0.6b`.
- The KB still listed 20 collections.
- Exactly one runtime agent remained configured:
  `Forensic_Records_Analyst`, scoped to
  `nexusai-structured-demo-v2-20260730`.
- Docker pipe access was denied to the Codex process, so container inventory was
  not asserted; health came from the running service endpoints.

## Tests

- Final focused Python Phase 5A contracts: 3/3 passed in 0.019 seconds.
- Complete Python worker discovery: 56 tests passed, 2 external real-sample tests
  skipped because their historical attachment path was absent; 1.290 seconds.
- Final focused Go Phase 5A Ginkgo selection: 9 selected specs, 9 passed;
  package 10.142 seconds after repository-local cache setup.
- Full `go test ./api/forensic_records -count=1`: passed; package 8.143 seconds.
- `go vet ./api/forensic_records`: passed with no output.
- Both new JSON contracts parsed successfully and Python compilation passed.
- The first Go invocation was blocked before compilation by the known Windows
  default-cache ACL. Re-running with `.cache/go-build`, a scoped `GOTMPDIR`, and
  `GOPROXY=off` passed.

## Security and provenance

- Discovery uses the existing forensic sidecar bearer and trusted-scope middleware.
- Specialist manifests use orchestrator-bound case scope and do not allow peer
  specialist delegation.
- Exact structured operations declare `structured_exact`, which selects no model.
- Unaccepted model roles have no configured model and remain `benchmark_required`.
- No source bytes, retained rows, evidence references, audit history, secrets, or
  model parameters changed.

## Performance and runtime

No runtime benchmark or live performance claim was added. Contract discovery is
an embedded in-memory catalog read, but it has not been deployed or measured in
the retained service. No image was built, no container was recreated, and no
service was restarted.

## Rollback

This is source-only. Rollback is review-based removal of the new catalog,
contract/registry files, route registrations, and compatibility selection hook.
No database, evidence, container, volume, or model rollback is required.

## Next phase

Phase 5A can continue with a compatibility adapter from the accepted legacy
`QueryPlan`/enterprise payload into the new typed contracts, then expose the
versioned registry through the LocalAI proxy and generated OpenAPI only after the
sidecar contract is accepted. Phase 5B remains the first user-facing outcome:
one validated URL-backed active case shared by Records, Agent Chat, upload,
evidence, queries, reports, and specialist scope.

No deployment, full build, model download, retained migration, collection cleanup,
Git staging, commit, push, or pull request is authorized by this report.
