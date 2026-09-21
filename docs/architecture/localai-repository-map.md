# LocalAI repository map for NexusAI

Status: R1 source accepted at the material architecture boundary  
Verified: 2026-08-06

| Directory | Primary responsibility | Languages | Important entry points | Runtime dependencies | NexusAI modifications | Tests | Risk/ownership |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `cmd/` | CLI and server entry points | Go | LocalAI commands | core application/config | product launched through LocalAI compatibility engine | Go suites | engine team; medium |
| `core/application` | dependency graph and services | Go | `Application` | model loader, auth, DB, agents | forensic sidecar and agent-pool access | Go/Ginkgo | platform team; high |
| `core/http/routes` | Echo route registration and middleware binding | Go | `localai.go`, `openai.go`, `agents.go`, `records.go` | auth, features, endpoints | 25 forensic proxy/public v1 routes | route and endpoint tests | API/security; critical |
| `core/http/endpoints` | LocalAI/OpenAI/admin handlers | Go | `localai/*`, `openai/*` | application services | governed cases, forensic proxy, collections, agents | Go/Ginkgo | API teams; high |
| `core/http/react-ui` | active NexusAI/LocalAI React SPA | React/JS/CSS | `src/router.jsx`, `src/App.jsx` | LocalAI HTTP APIs | Case Workspace, Records, Agent Chat, branding | 43 Playwright specs | product UI; high |
| `core/services/agents` | agent execution, direct routing, forensic tools | Go | dispatcher/executor/direct router | agent pool, sidecar, models | deterministic forensic routing and presentation | focused Ginkgo | agent platform; critical |
| `core/services/agentpool` | persisted agents, collections, chat runtime | Go | `agent_pool.go` | DB/KB/SSE | case-bound specialist configuration | Go/Ginkgo | agent platform; critical |
| `api/forensic_records` | forensic control/query/report sidecar | Go | `main.go`, `query.go`, contracts | PostgreSQL, NATS, LocalAI | evidence, capabilities, v1 case/query/report APIs | 221-test accepted baseline | forensic API; critical |
| `ingestion/forensic_records` | queue worker, format parsing, normalization | Python | `worker.py`, `forensic_contracts.py` | PostgreSQL, NATS, spool | five extracted family adapters plus compatibility worker | pytest/goldens | ingestion team; critical |
| `db/forensic_records` | schema, migrations, RLS, rollback/verification | SQL | migrations 001-010 | PostgreSQL/TimescaleDB | evidence control plane and queue lifecycle | SQL smoke/verify | data/security; critical |
| `configuration` | family, model, modality, country, agent policy | JSON | three forensic registries and Pakistan profile | loaded by worker/API/scripts | role and lifecycle governance | JSON/schema tests | governance; high |
| `backend` | native/Python/Rust inference backends | mixed | per-backend Dockerfiles/services | CPU/GPU libraries | underlying engine capability pool | backend-specific | backend team; high |
| `pkg` | shared model, HTTP, stores, MCP, utilities | Go | `model`, `httpclient`, `mcp` | core libraries | reused rather than forked | Go/Ginkgo | engine team; high |
| `scripts` | guarded builds, runtime gates, smoke and benchmark runners | PowerShell/Python | `build_deploy_*`, `smoke_*` | Docker, local runtime | rollback-aware NexusAI activation | AST/smoke | operations; critical |
| `reports` | immutable-ish acceptance evidence and runbooks | Markdown/JSON/logs | dated phase reports | source/runtime outputs | living forensic acceptance ledger | manual verification | product/QA; high |
| `docs` | product, design, architecture, operator docs | Markdown | content/design/architecture | Hugo/docs build | NexusAI product boundary | docs checks | documentation; medium |
| `.github` | build matrices and CI/CD | YAML/JS | backend matrix/workflows | registries, runners | inherited LocalAI release pipeline | CI | release team; high |

## Scale baseline

- Go: 1,362 source files in 146 directories.
- Python: 140 source files excluding caches.
- React/JS/CSS: 192 source files, about 50,969 lines.
- Backends: 64 unique matrix identifiers.
- HTTP: 371 bounded non-test Echo registrations plus 17 sidecar registrations.
- Tests: 593 Go test files, 16 Python test files, 43 Playwright specs.

## Runtime data flow

```text
React/customer API
  -> LocalAI Echo auth/feature middleware
  -> /api/v1/forensics compatibility/public contract
  -> forensic sidecar
  -> PostgreSQL/TimescaleDB + NATS JetStream + content spool
  -> Python family adapter worker
  -> canonical records/entities/KB assets/provenance
  -> deterministic query
  -> optional bounded Qwen explanation
  -> typed response and Agent Chat/Case Workspace presentation
```

## Known topology risks

The current accepted forensic implementation is spread across a very large dirty
worktree. `RecordsIntelligence.jsx` (2,905 lines), `worker.py`, `query.go`, and
agent execution files remain high-coupling areas. Incremental extraction behind
existing contracts is required; no big-bang rewrite is authorized.

## Material subsystem audit

| Subsystem | Source anchors | Verified source behavior | NexusAI boundary |
| --- | --- | --- | --- |
| Authentication, authorization and quotas | `core/http/auth/*`, `core/http/app.go`, `core/http/middleware/usage*.go` | Static keys, sessions, local auth, OAuth/OIDC, roles, feature access, route-feature quota accounting and API-key lifecycle exist. | Reuse engine controls; R16 must define tenant/case roles and prove production isolation. |
| OpenAI/vendor compatibility | `core/http/routes/{openai,openresponses,anthropic,ollama,elevenlabs,jina}.go` | Chat/completions/responses, embeddings, image/audio/media and vendor-compatible groups are separately registered. | Preserve compatibility APIs; ordinary analysts use governed forensic contracts. |
| Streaming and realtime | `core/http/endpoints/openai/realtime*.go`, `chat_stream*.go`, `openresponses/websocket.go`, `core/services/agentpool/agent_pool_sse.go` | SSE, streaming inference, WebSocket/WebRTC realtime transports and agent-pool SSE are material independent lifecycles. | Keep engine streaming; expose only capability-gated, case-bound streams. Agent Chat reliability is deferred to R6.2. |
| RAG, KB and agents | `core/services/agents/{knowledge,executor,dispatcher}.go`, `core/services/agentpool/*`, LocalAI agent/collection routes | Collections, knowledge retrieval, persisted agents/jobs and direct forensic routing coexist. | Case manifest and specialist bindings wrap raw collections; models never become exact-fact authority. |
| MCP and skills | `pkg/mcp/localaitools/*`, `core/services/{mcp,skills}/*`, `core/http/endpoints/{mcp,localai}/mcp*.go` | In-process and HTTP MCP clients, tools/prompts/resources, skill management and REST/MCP parity tests exist. | Retain for administrator automation and approved specialists; every admin endpoint must preserve route/tool parity and auth. |
| Model and backend lifecycle | `pkg/model/*`, `core/services/modeladmin/*`, `core/services/galleryop/*` | Gallery sourcing is separate from installed-model config; loader handles aliases, concurrent load coalescing, LRU/group eviction, pinning, preload, shutdown and logs. | R16 admin-only. Install/download/delete/upgrade/config/load actions remain explicit approval boundaries. |
| Distributed operation | `core/services/nodes/*`, `core/services/distributed/*`, `pkg/clusterrouting/*`, `docker-compose.distributed.yaml` | Node registration, model routing, gallery sync, distributed workers and NATS subjects are first-class upstream surfaces. | Disabled for the current local profile; R16 must add secure tenant-aware topology and acceptance. |
| React shared state | `src/main.jsx`, branding/theme/auth/operations contexts, router params and local-storage hooks | Global providers own branding/theme/auth/operations, while case selection is URL-bound and several chat/media preferences remain browser-local. | R2 defines state ownership; case/evidence/job truth must be server/URL scoped, never inferred from browser storage. |
| Deployment modes | root and forensic Compose files, `Dockerfile`, backend Dockerfiles, Kubernetes docs, `Makefile`, guarded PowerShell scripts | Single-node, forensic sidecars, distributed workers, backend images and Kubernetes guidance coexist. | Current acceptance covers the guarded local Compose profile only; all other modes require explicit R16/R17 acceptance. |

## R1 closure position

Repository ownership, all material technical boundaries, all 371 route-file
registration expressions and all 64 backend identifiers now have traceable
inventories/dispositions. R1 is source accepted. Endpoint-specific authorization
tests and backend dependency-license/promotion reviews remain mandatory in the
owning implementation phase; they are not deployment/runtime acceptance.
