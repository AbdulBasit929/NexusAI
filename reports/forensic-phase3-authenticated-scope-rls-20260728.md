# Phase 3 Authenticated Scope and Non-Owner RLS Acceptance

Date: 2026-07-28  
Status: source and isolated acceptance complete; retained deployment unchanged

1. **Objective:** bind forensic tenant, case, collection, actor and subject to an
   authenticated LocalAI-to-sidecar context; deny collection crossover and prove
   Phase 3 row-level security through a genuine non-owner role.
2. **Assumptions verified:** LocalAI remains the external authentication and
   collection-ownership boundary; one configured tenant is supported per sidecar;
   the active database remains migration 007; retained services/database require
   explicit approval before auth enablement or migration 008.
3. **Files changed:** added sidecar auth middleware/tests, Phase 3 RLS smoke SQL
   and this design/report; updated sidecar handlers, LocalAI proxy/upload paths,
   agent tools, Compose source configuration, routes, tests and project docs.
4. **Schema/API/config:** no new route or active schema. Existing forensic routes
   now accept bearer-authenticated internal scope headers in source. LocalAI
   checks exact collection ownership; sidecar body/query fields cannot override
   tenant/subject/collection/case; repair requires `admin`. Compose maps
   `FORENSIC_API_KEY`, `FORENSIC_API_AUTH_REQUIRED`, and the trusted tenant.
5. **Tests:** forensic API passed 110/110 Ginkgo specs plus legacy tests (specs
   1.322 s, package 10.049 s); focused LocalAI forwarding/ownership suite passed
   in 33.574 s, and its final explicit-binary rerun passed 21/21 focused specs in
   0.016 s; agent services passed in 126.976 s; route compilation passed in
   15.774 s; `go vet` passed for API,
   agents, LocalAI endpoints and routes; Python passed 24/24 in 0.541 s; Python
   compilation and Compose validation exited zero.
6. **Live runtime:** no LocalAI/sidecar rebuild or recreate, secret activation,
   migration, retained-database write, model change, or real-data ingest. The
   source security boundary is therefore not claimed as deployed.
7. **Dataset/evidence:** disposable synthetic migration fixture only. The RLS
   role saw exactly 1 storage object, 1 version, 2 source links, 1 run, 3 events,
   0 artifacts and 3 custody events for `phase3-smoke`.
8. **Models:** existing Qwen chat and embedding baseline unchanged; no model was
   downloaded, installed, promoted, or invoked for this slice.
9. **Accuracy/retrieval:** no analytical behavior or model baseline changed.
   Phase 2's 20/20 fixture, 18/18 chat and 27/27 deterministic-route gates remain
   the fixed regression baseline.
10. **Latency/memory:** no live inference measurement was required. The longest
    relevant package validation was agent services at 126.976 s. No service or
    model memory state was changed.
11. **Failures/fallbacks:** Windows file locking reported access denied while Go
    tried to delete generated test executables after the package printed `ok`.
    Compiling to an explicit workspace binary and running it separately exited
    zero with 21 passed and 0 failed; all generated binaries/caches were then
    removed. Compose emitted read-only access warnings for the user's Docker
    client config but returned success. An offline Swagger preview stopped before
    generation because the pinned CLI's command-only `urfave/cli/v2` dependency
    is not cached; no dependency was downloaded and no tracked Swagger file was
    changed. The generated bundle already omitted the pre-existing forensic proxy
    routes, so source annotations are correct but regeneration remains gated.
12. **Security/provenance:** bearer comparison is constant-time; health remains
    public; bounded trusted headers carry separate actor/subject identities;
    untrusted tenants and scope switches fail closed; evidence details filter all
    linked data by collection; repair is admin-only. A temporary `NOSUPERUSER`,
    `NOINHERIT`, `NOBYPASSRLS` role proved same-tenant reads/write and required a
    cross-tenant insert to fail. RLS is defense in depth, not a substitute for
    reviewed runtime ownership/grants or protected transport.
13. **Git:** all work remains unstaged, uncommitted and unpushed; no branch
    publication or PR was performed. Existing unrelated/user changes were
    preserved.
14. **Rollback:** authenticated mode is source-configurable and not active;
    retained services can remain on their current images/config. The RLS test ran
    inside a transaction and rolled back its role and same-tenant insert; the
    disposable database container was removed. Migration 008 rollback remains
    separately gated and has not been run on retained data.
15. **Next action/approval:** proceed in source with immutable raw retention/hash
    re-verification and reliable JetStream acknowledgement/retry/DLQ/reprocessing.
    Before deployment, obtain explicit approval to generate/store a service key,
    rebuild/recreate affected services, establish runtime DB grants, back up the
    retained registry, and separately apply migration 008.

## Tooling disclosure

The route-only compile populated missing modules in the existing Go cache:
`echo-swagger`, `flock`, `swag`, `openapi`, `yaml`, and `swaggo/files`. This did
not modify `go.mod`, `go.sum`, source data, containers, models, or services. All
subsequent Go validation ran with `GOPROXY=off`.

Generated Swagger regeneration still needs a separately approved pinned CLI
dependency download or an approved pinned generator container. This is an API
discovery/documentation gate, not a claim that the existing proxy routes are
absent from runtime source.
