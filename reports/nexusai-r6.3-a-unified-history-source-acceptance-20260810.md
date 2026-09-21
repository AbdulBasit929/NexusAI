# NexusAI R6.3-A Unified Case History — Source Acceptance

Date: 2026-08-10  
Contract: `forensics.case-analysis-history/v1`  
Status: source accepted; guarded live activation pending

## Accepted outcome

R6.3-A1 through A5 are complete at the source and local production-preview
boundary. NexusAI now has one retained case-analysis authority instead of
presenting browser bookmarks as enterprise history.

- `agent_analysis_history` is additive and uniquely scopes requests by tenant,
  user, agent, case and message ID. The v1 boundary requires matching case and
  collection IDs.
- Requests are retained before dispatch. Local callbacks and the API-side
  distributed event listener persist correlated terminal state and final
  answers. Hidden reasoning and transient tool streams are excluded.
- Five authorized routes provide stable cursor pagination, detail, idempotent
  save/unsave, explicit bounded browser import and import rollback.
- Browser imports use deterministic case-scoped identities. Rollback deletes
  only unsaved, non-held rows from the selected import and cannot delete runtime
  analyses.
- The React workspace uses server history as its saved-analysis authority.
  Browser sessions remain separate and can be imported only after confirmation.
- The history rail is an accessible overlay drawer below 1024px, so retained
  history remains usable at phone and tablet widths without page overflow.

## Governance and rollback

Retention expiry and legal-hold mutation remain intentionally unautomated until
an approved governance duration and privileged hold workflow exist. Application
rollback restores preserved images and keeps the additive table and retained
rows. Dropping schema is not an application rollback and requires separate
backup, legal and operator approval.

The activation script validates all generated routes and the explicit
PostgreSQL URL, invokes the proven sequential rebuild/health gate, preserves
rollback images and named volumes, and performs only readiness, shell and
history GETs. It sends zero history mutations.

## Acceptance evidence

- Focused analysis-history contract tests: PASS.
- Affected Go compile-only packages: PASS. The Windows toolchain emitted a
  post-test temporary-executable cleanup warning, after all package results were
  successful.
- Pinned Swag v1.16.6 generation: PASS; 5/5 history routes present.
- Focused UI lint: 0 errors (18 pre-existing warnings in touched files).
- Vite production build: PASS, 669 modules.
- Agent production-preview suite: PASS, 16/16.
- Independent in-app browser: desktop and 390px governed case ready; mobile
  history drawer usable; zero warning/error logs; zero horizontal overflow.
- PowerShell activation-script parse: PASS.

An attempted optional SQLite-backed store isolation test was not accepted as
evidence because this Windows build has `CGO_ENABLED=0`; it was removed rather
than skipped or misreported. The existing PostgreSQL-backed broad suite remains
host-constrained by unavailable rootless Docker test containers. Scope is still
enforced in every query by tenant, user, agent, case and collection, and the
live activation gate performs a read-only authorized contract check.

## State mutation statement

No Docker rebuild, deployment, database migration, retained browser import,
history save mutation, model change, staging, commit, push or publication was
performed during this source slice.

## Next gate

Run `scripts/build_deploy_nexusai_r6_3_gate.ps1` with the explicit local retained
database URL. R6.3-A becomes live accepted only when
`reports/runtime-activation-20260810/r6.3-live-acceptance.json` reports
`status: live_accepted` and `mutating_requests_sent: 0`. R6.3-B follows.
