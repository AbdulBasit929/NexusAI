# NexusAI APF-3 Final Source Closure — 2026-08-18

## 2026-08-19 final runtime acceptance

The final guarded API/UI-only activation passed with API build 4.2 seconds and
LocalAI/UI build 97.1 seconds. Running image IDs matched the newly built tags,
and corrected source hashes remained stable. Worker, PostgreSQL, NATS, named
volumes, retained evidence, KB data, models, profiles and rollback images were
preserved. LocalAI readiness, forensic health and Analyst HTML returned 200.

The displayed Analyst conversation alone was cleared and the complete 18-case
deck was replayed from Case 1. Cases 1–3 remained together. All 18 cases passed
with zero P0/P1. Corrected typo routing, IPDR representative provenance, typed
composition dependency/no-result suppression and governed step presentation
all passed. Deterministic cases reported zero LLM latency.

- `APF-3SourceStatus=ACCEPTED`
- `APF-3.7SourceStatus=ACCEPTED`
- `APF-3BrowserAcceptance=ACCEPTED`
- `APF-3.7RuntimeStatus=ACCEPTED`
- `APF-3RuntimeStatus=ACCEPTED`
- `APF-3FinalStatus=CLOSED`
- `DeploymentNeeded=NO`
- `P0Open=0`
- `P1Open=0`
- `BrowserCases=18/18 PASS`
- `SourceRuntimeParity=PASS`
- `Health=PASS`
- `SecurityScope=PASS`

## Historical post-source deployment update

The superseded earlier activation marker was:

```text
R8UIAPIActivation=PASS APIBuildSeconds=25.1 LocalAIBuildSeconds=406.1 WorkerRebuilt=false ModelsChanged=false ProfilesChanged=false VolumesPreserved=true RollbackImages=preserved
```

LocalAI readiness, forensic health and Analyst HTML returned HTTP 200. The
records worker, NATS and PostgreSQL were preserved and healthy. This was the
infrastructure state before the later browser rejection recorded above; it is
not the current acceptance status.

## Final decision

- `APF-3SourceStatus=ACCEPTED`
- `APF-3.7SourceStatus=ACCEPTED`
- `APF-3BrowserAcceptance=ACCEPTED`
- `APF-3.7RuntimeStatus=ACCEPTED`
- `APF-3RuntimeStatus=ACCEPTED`
- `APF-3FinalStatus=CLOSED`
- `DeploymentNeeded=NO`
- `P0Open=0`
- `P1Open=0`
- `UnsafeExecutionAccepted=0`

APF-3 is closed. Deferred Case 18 capability-guard latency, Case 15 bounded-
synthesis latency and IPDR representative-citation labeling/version-field
clarity remain P2/P3 and do not reopen APF-3. `NextProgram=STIM` and
`NextPhase=STIM-0`; substantive STIM implementation has not started.

## APF-3.7 implementation

The existing orchestrator now supports a second governed plan shape,
`bounded_composition`, without replacing the single-capability fast path. A
composition contains two or three registered read-only steps, typed
dependencies and exact input bindings. It is bounded to 300 intermediate rows,
12 retrieval chunks, one model call and 60 seconds.

Validation rejects unknown or duplicate capabilities, arbitrary SQL/URLs/tool
IDs, scope widening, unauthorized or unavailable data, disabled citations,
self/unknown dependencies, cycles, invalid typed bindings, excessive steps or
resource budgets, and factual completed results without citations. Execution
uses only existing deterministic, KB and hybrid executors. Partial failure is
reported as partial analysis and cannot produce a complete-correlation claim.
Cross-family claim lineage is explicitly `candidate_correlation`; it never
asserts identity or relationship truth.

The enterprise response includes each step's finding, typed inputs, tool ID,
citations and claim lineage. Composition responses do not receive misleading
single-family enrichment or synthetic zero-row citations.

## Risk-based operation certification

The authoritative generated ledger is
`api/forensic_records/contracts/operation-certification-v1.json`.

| Tier | Purpose | Total | Certified | Bounded uncertified | Exposure |
| --- | --- | ---: | ---: | ---: | --- |
| A | Daily/demo truth surface | 4 | 4 | 0 | queryable/suggestion eligible |
| B | Supported specialist operations | 50 | 0 | 50 | explicit-request limited |
| C | Legacy/engineering operations | 11 | 0 | 11 | engineering only, not queryable |
| **Total** |  | **65** | **4** | **61** |  |

Tier A comprises `cdr.frequent_contacts`, `cdr.temporal_activity`,
`forensics.evidence`, and `forensics.evidence_package_summary`. Each has a
fixture/oracle/result/routing/parameter/calculation/citation/presentation and
security basis. Tier B/C correctness debt is recorded rather than hidden:
there are no known correctness defects, but none of those 61 operations is
advertised as fully certified. Suggestion eligibility is restricted to
certified operations. Engineering-only operations are excluded from the
queryable capability projection.

The query-variant ledger remains 155 accepted occurrences, 143 unique variants
and 12 duplicate occurrences; all 155 pass routing, parameter and semantic
expectations. The executable inventory remains 65 operations: 62 records, one
KB and two hybrid.

## Defects found and closed

| Severity | Finding | Closure |
| --- | --- | --- |
| P1 | Dependency-count sorting was not a true topological ordering | Replaced by validated topological ordering; dependency and exact-binding regression passes. |
| P1 | Composition response received misleading primary-family enrichment and a synthetic zero-row citation | Composition now preserves only step/claim lineage and real citations. |
| P0 | None | No open P0. |

## Verification

- `go test ./api/forensic_records -run '^TestAPF37' -count=1` — PASS.
- Risk-tier exposure, operation-registration and query-ledger drift gates — PASS.
- `go test ./api/forensic_records -count=1` — PASS (`31.833s` on final run).
- `go vet ./api/forensic_records` — PASS.
- Focused agent presentation/history/tools/lifecycle/retry Ginkgo suite — PASS.
- Standalone agent regression selection — PASS.
- APF-3.7 tests cover fast-path preservation, valid two-step and structured+KB
  plans, typed bindings, true dependency ordering, citations, partial failure,
  claim lineage, presentation, unsafe plans and bounded planning performance.

The earlier parallel Windows run failed before compilation because concurrent
processes collided on the shared Go build cache. The same gates passed using an
isolated repository-local Go cache; this is runner evidence, not a product
defect.

## Manual acceptance deck after deployment

Run these in the `nexusai-forensic-demo` workspace. Verify the operation/plan,
parameters, truthful empty/unsupported state, cited evidence and completed
History entry. Follow-up cases must be entered in the same conversation.

| # | Analyst input | Expected result |
| ---: | --- | --- |
| 1 | `Who did 923001110001 contact most?` | `cdr.frequent_contacts`; eight ranked cited contacts. |
| 2 | `Only outgoing.` | Retains target/operation; direction becomes OUTGOING. |
| 3 | `For 923001234567 instead.` | Retains operation/direction and replaces only the target. |
| 4 | `Show temporal CDR activity for 923001234567 on 2026-07-10.` | `cdr.temporal_activity`; four events and independently accepted duration aggregates. |
| 5 | `Who are the frequent contacts?` | Minimal target clarification; no query executes. |
| 6 | `Show exact ANPR sightings for ZZZ-SYNTHETIC-NO-MATCH.` | Truthful no-results state; no fabricated sighting. |
| 7 | `923001234567 cdr actvty 10 july 2026 pls` | Validated language assistance or deterministic normalization; same target/day. |
| 8 | `10 july 2026 ko 923001234567 ki CDR activity dikhao` | Roman Urdu equivalence to case 4. |
| 9 | `10 جولائی 2026 کو 923001234567 کی CDR سرگرمی دکھائیں` | Urdu equivalence to case 4. |
| 10 | `Show IPDR endpoint activity for 10.20.1.7.` | Registered IPDR operation with exact endpoint parameter and cited rows or truthful no results. |
| 11 | `Look up subscriber identity observations for 923001234567.` | Registered subscriber operation; observations only, no ownership inference. |
| 12 | `Look up tower site PK-LHR-HIST-001.` | Registered tower operation; cited site data or truthful no results. |
| 13 | `Show exact ANPR sightings for ABC-123.` | Registered ANPR operation; cited sightings or truthful no results. |
| 14 | `Show forensic evidence lineage.` | Tier-A `forensics.evidence`; cited KB results. |
| 15 | `Prepare an evidence package summary.` | Tier-A hybrid; deterministic rows plus cited KB retrieval. |
| 16 | `Show subscriber identity and CDR activity for 923001234567.` | APF-3.7 two-step bounded composition with typed binding and candidate-correlation limitation. |
| 17 | `Show subscriber identity and CDR activity for 999999999999.` | Empty/partial result is explicit; no complete correlation claim. |
| 18 | `Transcribe and diarize this audio.` | Unsupported/missing-input response; no invented execution. |

## Final 18-case runtime result

| Case | Actual operation/result | Provenance | API / UI latency | Result |
| ---: | --- | --- | --- | --- |
| 1 | `cdr.frequent_contacts`; 8 ranked contacts for `923001110001` | 101-row complete aggregate lineage | 124 ms / 2.44 s | PASS |
| 2 | Same operation/target; `OUTGOING`; counts 68/67/61/61/52/51/49/38 | 51-row complete aggregate lineage | 164 ms / 1.18 s | PASS |
| 3 | Target replaced with `923001234567`; one outgoing contact `923111234567` | 1-row complete aggregate lineage | 90 ms / 1.15 s | PASS |
| 4 | `cdr.temporal_activity`; 4 events, 10/45/27.5-second facts | Complete contribution lineage plus cited rows 5/2 | 112 ms / 1.19 s | PASS |
| 5 | Target clarification; no query | No provenance claimed | 3 ms / 1.05 s | PASS |
| 6 | Exact long ANPR target; truthful zero | No provenance claimed | 107 ms / 1.13 s | PASS |
| 7 | Deterministic `cdr.temporal_activity`; same facts as Case 4 | Same complete lineage/citations | 92 ms / 1.15 s | PASS |
| 8 | Roman-Urdu temporal equivalence | Same complete lineage/citations | 129 ms / 1.76 s | PASS |
| 9 | Urdu temporal equivalence | Same complete lineage/citations | 98 ms / 1.61 s | PASS |
| 10 | `ipdr.endpoint_summary`; 20 endpoint aggregates for `10.20.1.7` | 20 representative evidence/source/row/hash citations; source SHA-256 resolvable | 228 ms / 1.98 s | PASS |
| 11 | `subscriber.identity_lookup`; truthful zero | No ownership inference/provenance claim | 58 ms / 1.64 s | PASS |
| 12 | `tower.site_lookup`; truthful zero | No fabricated location/provenance | 56 ms / 1.67 s | PASS |
| 13 | `anpr.sightings`; 20 sightings | 20 evidence/version/row/hash citations | 57 ms / 1.78 s | PASS |
| 14 | `forensics.evidence`; 3 KB results | 3 KB citations; no SQL | 432 ms / 1.66 s | PASS |
| 15 | Hybrid evidence package; 28 rows and 3 KB results | Provenance metric 24 | 47,974 ms / 49.26 s | PASS |
| 16 | Bounded composition; subscriber no-result, dependent CDR skipped | No correlation/provenance claim | 125 ms / 1.35 s | PASS |
| 17 | Exact 12-digit target; subscriber no-result, dependent CDR skipped | No correlation/provenance claim | 129 ms / 1.34 s | PASS |
| 18 | Capability guard; no SQL, KB, model or transcript | No evidence claim | 60,013 ms / 61.40 s | PASS; P2 latency deferred |

## Runtime and deployment boundary

The running demo containers contain the accepted final correction. Named
volumes, retained data, models, profiles and the worker image were unchanged.
APF-3 runtime acceptance is closed; no additional APF deployment is needed.
STIM-0 is the next program boundary but has not started in this execution.

## Historical guarded deployment handoff — completed

The following handoff was used for the completed activation and is retained as
historical evidence, not as a current action. The
script preserves rollback images and does not rebuild the worker or alter
models, profiles or named volumes. Do not add `--remove-orphans`, `down`, prune
or volume deletion.

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

docker info --format 'DockerServer={{.ServerVersion}}'
if ($LASTEXITCODE -ne 0) { throw 'Docker Desktop Linux engine is not ready.' }

$os = Get-CimInstance Win32_OperatingSystem
$freeGiB = [math]::Round($os.FreePhysicalMemory / 1MB, 2)
"FreeRAMGiB=$freeGiB"
if ($freeGiB -lt 4) { throw 'At least 4 GiB free RAM is required for the guarded build.' }

& '.\scripts\build_deploy_nexusai_r8_ui_api_gate.ps1' -PreflightOnly
if ($LASTEXITCODE -ne 0) { throw 'Guarded NexusAI preflight failed.' }

& '.\scripts\build_deploy_nexusai_r8_ui_api_gate.ps1'
if ($LASTEXITCODE -ne 0) { throw 'Guarded NexusAI deployment failed.' }

$checks = @(
    @{ Name = 'LocalAI readiness'; Url = 'http://localhost:8080/readyz'; Headers = @{} },
    @{ Name = 'Forensic API'; Url = 'http://localhost:8091/healthz'; Headers = @{} },
    @{ Name = 'Analyst Portal'; Url = 'http://localhost:8080/analyst/ask?case=nexusai-forensic-demo'; Headers = @{ Accept = 'text/html' } }
)

foreach ($check in $checks) {
    $response = Invoke-WebRequest -UseBasicParsing -TimeoutSec 30 -Uri $check.Url -Headers $check.Headers
    "{0}: HTTP {1}" -f $check.Name, $response.StatusCode
}

docker ps --filter 'name=nexusai-' --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}'
Start-Process 'http://localhost:8080/analyst/ask?case=nexusai-forensic-demo'
```

Successful activation must emit `R8UIAPIActivation=PASS` with
`WorkerRebuilt=false`, `ModelsChanged=false`, `ProfilesChanged=false`,
`VolumesPreserved=true` and `RollbackImages=preserved`. Then execute the 18-case
manual deck above before marking APF-3.7 runtime accepted.
