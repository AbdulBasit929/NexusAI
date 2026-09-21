# NexusAI Records Analyst UX Source Acceptance

Date: 2026-07-31  
Status: source accepted; operator rebuild and live UI acceptance pending

## Outcome

The Records experience has been reorganized around an analyst decision rather
than backend response sections. A completed query now opens directly on one
summary surface containing a human-readable result title, concise answer, exact
row/evidence/finding/latency indicators, typed visuals, cited findings, exact
row preview, next checks, and collapsed evidence limits. Full rows, timeline,
evidence, and execution audit remain available through four secondary controls.

The previous Executive Brief/Data Grid/Entity Timeline/Typed Visuals/Provenance
& Sources/Coverage & Limitations tab set, duplicate case-health sidebar,
repeated cautions, and empty `Value: {}` presentation are removed. Technical
trace and ingest-health detail remains available in Audit details and JSON
export instead of competing with the answer.

## Query productivity

- Exact target is a separate field and recent evidence targets are suggested.
- Templates are grouped into CDR, IPDR, ANPR, Operations, Timeline & Movement,
  Entities & Links, Evidence & Briefing, and Records Analytics.
- Target-required chips prepare the query and request the identifier instead of
  sending an unsafe ambiguous operation.
- Target-free camera activity remains target-free.
- Existing Phase 6 deterministic operations are exposed across all families:
  eight dedicated CDR analysis templates plus the target-bound evidence
  timeline, seven IPDR templates, and seven ANPR templates, plus case readiness,
  source, entity, evidence, and briefing workflows.
- Results automatically move into view; preview is deliberately capped while
  full exact rows remain searchable, sortable, and exportable.

## Contract and API correction

LocalAI now registers public authenticated proxy routes for the platform
adapter, operation, specialist, and contract catalogs. Trusted tenant/actor and
sidecar bearer identity are forwarded by the shared discovery handler. Records
feature authorization includes all four routes.

Operation-only v1 plans now humanize dots and underscores before legacy
natural-query fallback. This prevents `anpr.camera_activity` from becoming the
false plate target `CAMERA_ACTIVITY`; a regression specification verifies that
the target remains empty.

## Collection presentation

The governed analyst case remains `records-demo-verified`. Acceptance, legacy,
system, specialist-name, and superseded demo collections are hidden from normal
case selection and Collections-page browsing. They remain available through an
explicit retained-system audit toggle. No collection or evidence was deleted.
The complete live inventory and exact zero-entry candidates are recorded in
`forensic-collection-cleanup-plan-20260731.md`.

## Verification evidence

- `go test ./api/forensic_records -count=1`: PASS in 18.535 s; 183 passed, two
  intentionally skipped, zero failed.
- `go vet ./api/forensic_records`: PASS.
- Focused LocalAI discovery-proxy Ginkgo: PASS, one selected assertion.
- `go test ./core/http/auth -count=1`: PASS.
- Targeted Records, Collections, and Records E2E ESLint: zero errors; eight
  repository-config JSX-use warnings.
- React production build: PASS; 658 modules transformed.
- Catalog JSON parse and Python bytecode compilation: PASS.
- Final `git diff --check`: PASS; line-ending notices only.
- The broader routes suite compiled and ran 18/19 specs; its unrelated backend
  upgrade fixture failed after sandbox-blocked OCI network access and a missing
  temporary `run.sh`. No Records route or discovery assertion failed.
- Swagger generation was attempted with the pinned `swag` version but checksum
  network access required escalation; that approval could not be granted due to
  the current tool-usage limit. Source annotations are present; generated
  Swagger artifacts remain pending the operator command in the runbook.
- Standalone Playwright remains pending because the repository-pinned Chromium
  headless-shell executable is not installed. The updated specification checks
  the analyst-first layout, typed family visuals, evidence citations, absence of
  empty object output, target-free camera activity, and exact ANPR target body.

## Deployment boundary

The currently running Phase 6.3 image predates these final source refinements.
No further Docker rebuild was performed by Codex, per operator instruction. Use
`nexusai-records-ui-acceptance-runbook-20260731.md` after restarting Windows.
Only then can the redesigned UI, public discovery routes, and collection
visibility be marked live accepted.
