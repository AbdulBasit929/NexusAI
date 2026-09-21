# NexusAI / LocalAI integration architecture vNext

## Boundary

LocalAI is the local inference and protocol substrate. NexusAI is the forensic
system of record and policy authority. This boundary avoids duplicating mature
serving capabilities without outsourcing forensic truth.

| Layer | LocalAI role | NexusAI role |
|---|---|---|
| Inference | model/backend lifecycle, OpenAI APIs | admission, role policy, evidence-safe prompts |
| Responses | streaming/background/tool transport | typed plan, scope, state, audit, citations |
| Agents | optional bounded execution loop | orchestrator policy and specialist allowlists |
| MCP | protocol/server/client | privilege-separated forensic tools |
| Retrieval | embeddings/rerank/store primitives | evidence corpus, chunk/citation truth |
| Audio/vision | admitted primitive endpoints | artifact provenance, candidates, certification |
| Auth | service/API authentication | tenant/case/evidence authorization |
| Observability | backend/model traces and admission | forensic audit correlation and redaction |

## Discovery and provenance

The integration records semantic LocalAI version, source revision, image digest,
backend digests, configured model immutable revisions, discovery document, and
route contract. Startup fails closed for required capability mismatches. Gallery
presence is never interpreted as installed or admitted capability.

The current runtime's 404 on `/v1/models/capabilities` blocks reliance on that
surface. Existing well-known and instructions endpoints remain informative but
are verified against the required-route manifest.

## Open Responses

Responses may provide streaming, background execution, continuation, and tool
events after bounded certification. Nexus wraps it with a forensic execution ID
and persists only policy-approved state. Cancel and retry semantics must not
duplicate operations. Previous-response context cannot widen evidence scope.

## Functions, agents, and MCP

Tool definitions are generated from the Nexus operation registry with typed
schemas, versions, risk class, and scope claims. Parallel tool calls are off
until ordering, concurrency, and audit tests pass. LocalAI skill loading is off
by default for forensic agents unless a reviewed Nexus skill is explicitly
admitted. MCP tools are separated into analyst, service, and admin trust zones.

## Model and primitive admission

Every candidate requires source, license, immutable revision, size, hardware,
benefit hypothesis, benchmark pack, thresholds, rollback, and privacy review.
Model output is `candidate` unless an operation contract promotes a measurement
under an independent oracle. Face, semantic similarity, captioning, diarization,
audio events, and video-language output never become identity or event facts by
mere model confidence.

## Upgrade sequence

1. Establish exact current LocalAI provenance and a route/capability manifest.
2. Compare v4.9 config, auth defaults, backend/model compatibility, and image
   changes without altering runtime.
3. Build a candidate image only under approval; retain current immutable rollback.
4. Run contract, security, model, resource, and Nexus regression suites.
5. Canary the minimum service set, certify live, then promote or roll back.

No upgrade is bundled with NX-1 truth closure.
