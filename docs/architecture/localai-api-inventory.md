# LocalAI and NexusAI API inventory

Status: R1 source accepted; route expressions and material middleware groups reconciled.  
Verified: 2026-08-06

## Static registration summary

Bounded source extraction found 371 non-test registrations in
`core/http/routes`, 10 cross-service/static registrations in `core/http/app.go`,
and 17 forensic-sidecar registrations. The exact 381 LocalAI registration
expressions are in
`reports/nexusai-r1-http-route-registration-inventory-20260806.json`. Largest
route groups are `localai.go` (96), `agents.go` (52), `auth.go` (41),
`ui_api.go` (36), `nodes.go` (33), `openai.go` (28), and
`records.go` (25).

| Group | Examples | Classification | Access posture |
| --- | --- | --- | --- |
| OpenAI compatibility | `/v1/models`, chat/completions, embeddings, audio/images | compatibility API | model/request middleware |
| Anthropic/ElevenLabs/Ollama/Jina | vendor-compatible routes | compatibility API | feature/model middleware |
| LocalAI engine/admin | models, backends, monitor, traces, settings | admin API | admin middleware |
| Agents/collections/skills/jobs | `/api/agents/*` | engine plus NexusAI reuse | feature and pool-ready middleware |
| Auth/users/usage | `/api/auth/*`, users, quotas | security/admin API | auth/admin middleware |
| Distributed/nodes/P2P | node registration, scheduling, monitoring | admin/internal | admin and distributed policy |
| NexusAI records | `/api/records/*` | compatibility product API | records feature middleware |
| NexusAI forensic v1 | `/api/v1/forensics/*` | public product API | records middleware plus case checks |
| Forensic sidecar | port 8091 internal routes | internal service API | static API key required except health |

## NexusAI LocalAI-facing routes

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/api/records/ingest` | local records ingestion |
| GET | `/api/records/batches` | batch list |
| GET/DELETE | `/api/records/batches/:id` | batch inspect/delete compatibility surface |
| POST | `/api/records/query` | exact records query |
| POST | `/api/records/aggregate` | parameterized aggregation |
| POST | `/api/records/correlate` | exact correlation |
| GET | `/api/records/schema/:record_type` | schema discovery |
| GET | `/api/records/forensic/status` | governed collection status |
| GET | `/api/records/forensic/templates` | deterministic templates |
| GET | `/api/records/forensic/capabilities` | live family availability |
| GET | `/api/records/forensic/evidence[/:id]` | evidence list/detail |
| POST | `/api/records/forensic/evidence/:id/reprocess` | approval-sensitive reprocess |
| POST | `/api/records/forensic/query` | compatibility forensic query |
| GET | `/api/v1/forensics/adapters` | adapter discovery |
| GET | `/api/v1/forensics/operations` | operation discovery |
| GET | `/api/v1/forensics/agents` | specialist discovery |
| GET | `/api/v1/forensics/contracts` | query/response contracts |
| GET | `/api/v1/forensics/cases` | governed case list |
| GET | `/api/v1/forensics/cases/:case_id` | governed case metadata |
| GET | `/api/v1/forensics/cases/:case_id/manifest` | read-only resource manifest |
| GET | `/api/v1/forensics/cases/:case_id/evidence` | case evidence |
| POST | `/api/v1/forensics/cases/:case_id/query` | case-bound query |
| POST | `/api/v1/forensics/cases/:case_id/reports` | synchronous report generation |

## Internal forensic sidecar routes

`GET /healthz`; upload webhook; collection status/asset repair; evidence
list/detail/reprocess; query templates/capabilities/hybrid; v1 adapter/operation/
agent/contract discovery; case manifest/query; and report generation. All
non-health direct calls require the sidecar API key. Ordinary consumers should
use the LocalAI-facing v1 contract.

## Confirmed gaps

- Immutable report persistence is absent.
- Endpoint changes still require their route-specific auth, feature, quota,
  streaming, storage/model side effects and UI/MCP caller checks; the exhaustive
  source registration inventory makes that implementation diligence traceable.
- Public v1 cases/evidence/query/report exists, but entities, relations, jobs,
  webhooks, pagination, and service-account scopes remain roadmap work.
- Any new admin forensic endpoint must also be mapped into MCP and the route
  parity test required by repository policy.

## Source-grounded route-group disposition

| Route group | Source anchor | Streaming/state characteristic | NexusAI disposition |
| --- | --- | --- | --- |
| OpenAI chat/completions/embeddings/audio/images | `core/http/routes/openai.go` | Request/stream middleware; model capability dispatch | Retain compatibility; wrap case work in forensic v1 APIs. |
| Open Responses and realtime | `routes/openresponses.go`, `endpoints/openresponses/websocket.go`, `endpoints/openai/realtime*.go` | Stored responses, WebSocket, WebRTC and coordinated streaming state | Capability-gate; not an R6 acceptance claim. |
| Anthropic, Ollama, ElevenLabs and Jina | matching files in `core/http/routes` | Vendor-shaped compatibility | Retain for engine customers; hide from ordinary forensic navigation. |
| LocalAI system/model/backend/gallery | `routes/localai.go`, `ui_backend_gallery.go`, `ui_api.go` | Includes mutating install/config/delete/load operations and status streams | Administrator only; mutation requires approval and audit. |
| Agents, collections, skills and jobs | `routes/agents.go` and agent endpoints | Persisted jobs, collection state and SSE | Reuse behind role/case policy; raw mechanics live in administration. |
| MCP | `endpoints/mcp/*`, `endpoints/localai/mcp*.go`, `pkg/mcp/localaitools` | Tools/prompts/resources and in-process admin assistant | Administrator/specialist allowlist with REST/MCP parity. |
| Authentication/users/usage | `routes/auth.go`, `core/http/auth/*`, `routes/usage.go` | Session/key/OIDC state, roles, quotas and usage | Reuse as R16 foundation; current local profile is not enterprise acceptance. |
| Nodes/distributed/P2P | `routes/nodes.go`, `endpoints/localai/p2p.go` | Registration, scheduling and distributed backend state | Hide/disable now; R16 security and scale gate. |
| Records and forensic v1 | `routes/records.go`, forensic proxy handlers | Sidecar proxy, deterministic query, case-bound contracts | Primary NexusAI product API; expand additively with pagination/idempotency. |

## Middleware and authority contract

`core/http/app.go` applies authentication before
`RequireRouteFeature`, constructs explicit middleware for agents, skills,
collections, records, MCP jobs, MCP, fine-tuning and quantization, and registers
admin middleware separately. `core/http/auth/features.go` is the route-feature
registry used by feature enforcement and quota accounting. A UI route guard is
not a security boundary; server middleware and case/tenant checks remain
authoritative.

R1 source acceptance covers the complete registration inventory and material
middleware/product dispositions. It does not pre-approve the behavior or
security of future endpoint changes; each owning phase must run route-specific
tests and preserve REST/MCP/UI capability parity.
