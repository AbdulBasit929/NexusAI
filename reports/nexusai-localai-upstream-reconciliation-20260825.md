# NexusAI / LocalAI upstream reconciliation — 2026-08-25

## Verified upstream baseline

Official LocalAI v4.9.0 was released on 2026-08-20. Material additions include
deny-by-default auth, context compression, canonical model/backend pages,
vllm-cpp video modality, Qwen3 TTS, KNN routing, admission controls, backend
traces, safer PII pseudonyms, and parallel Hugging Face downloads. v4.8.0 added
vllm-cpp alpha, VRAM budgets, artifact materialization, activity/tracing,
Valkey stores, TTS additions, distributed hardening, and security fixes.

Primary official sources:

- https://github.com/mudler/LocalAI/discussions/11628
- https://github.com/mudler/LocalAI/discussions/11371
- https://localai.io/docs/
- https://localai.io/docs/features/api-discovery/
- https://localai.io/docs/features/openai-functions/
- https://localai.io/docs/features/agents/
- https://localai.io/docs/features/mcp/
- https://localai.io/docs/basics/try/

## Local runtime evidence

The runtime reports snapshot `40717b8`, not a LocalAI semantic release. Its
well-known document advertises agents, MCP, records, tracing, configuration,
stores, and OpenAI-compatible features. `/v1/responses` exists, while
`/v1/models/capabilities` returns 404: a concrete discovery gap. Installed
backends are llama.cpp, Whisper, Faster-Whisper, and face detection; gallery
entries are not admissions.

## Ownership boundary

**Reuse:** inference, Open Responses transport after certification, function
parsing, MCP transport, discovery, embeddings, reranking, tracing, stores, and
admitted audio/vision primitives.

**Nexus-owned:** evidence/case/tenant authorization, provenance, adapters,
deterministic analytics, operation allowlists, typed plans, Fact Packets,
result states, citations/history, candidate semantics, and certification.

**Hybrid:** LocalAI Agents/Open Responses/MCP may execute a Nexus-governed plan,
but may not become the policy engine, create unrestricted SQL, bypass evidence
scope, or allow specialist peer calls.

## Upgrade posture

Do not pull, merge, rebuild, or upgrade during NX-R0. Establish source
provenance, semantic version exposure, route deltas, auth-default impact,
configuration migration, backend compatibility, resource budgets, and rollback
images first. Admit one capability at a time with independent oracles.
