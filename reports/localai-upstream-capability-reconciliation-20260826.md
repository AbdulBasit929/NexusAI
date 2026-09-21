# LocalAI upstream capability reconciliation — 2026-08-26

## Scope and evidence boundary

This is a read-only source and official-documentation review for NX-UX1. It
does not install, enable, deploy, or certify a LocalAI feature. The comparison
uses the current repository and the official LocalAI documentation and source
available on 2026-08-26:

- https://localai.io/docs/index.html
- https://localai.io/docs/features/index.html
- https://localai.io/docs/features/agents/index.html
- https://github.com/mudler/LocalAI/blob/master/docs/content/features/mcp.md
- https://localai.io/docs/features/openai-realtime/
- https://localai.io/features/openai-functions/index.html
- https://github.com/mudler/LocalAI

## Capability reconciliation

| Capability | Latest official LocalAI surface | Current local equivalent | NexusAI-specific layer | Disposition | Integration risk / future phase |
|---|---|---|---|---|---|
| OpenAI-compatible inference | Chat, Responses, embeddings, reranking and multimodal APIs | Corresponding `/v1` routes are present in current source | Forensic API calls bounded model roles only after deterministic work | Reuse runtime and API compatibility | Do not let a generic response route become forensic execution authority; NX-A1/NX-B1 |
| Multimodal backends | OCI backends span text, image, audio, video and embeddings | Gallery/backend infrastructure and modality routes are present | Evidence admission, provenance, artifacts and truthful result states remain custom | Reuse backend lifecycle; preserve forensic contracts | Backend availability is not operation certification; modality phases NX-M2/NX-D2 |
| Functions and tools | OpenAI-compatible functions/tools and constrained output | Function/tool parsing is present | Certified forensic operations and typed parameters are maintained by the forensic API | Reuse transport and invocation primitives later | Never expose arbitrary tools as certified analyst operations; NX-A1 design gate |
| Built-in agents | In-process agents, tools/actions, knowledge bases, skills, SSE and import/export | Agent routes, configuration and streaming are present | `Forensic_Records_Analyst`, deterministic planning and governed presentation are custom | Reuse agent hosting and streaming | Generic agent autonomy must not replace deterministic authority or case scope; NX-A1 |
| MCP | Model- and agent-scoped remote/stdio MCP, tools, prompts and resources | MCP servers, apps and local administrative tools are present | Forensic evidence access and operation contracts are separate | Reuse for bounded integrations after policy review | Tool permission, tenant scope and prompt-injection boundaries; NX-A1/NX-X2 |
| Retrieval / knowledge bases | Retrieval and agent knowledge-base support | KB and retrieval infrastructure is present | Evidence membership, source citations, Fact Packets and claim validation are custom | Reuse retrieval plumbing | Retrieval relevance is not evidentiary proof; NX-B1/NX-B2 |
| Realtime voice | WebSocket/WebRTC realtime APIs | Realtime routes and voice surfaces are present | Evidence ASR, retained audio provenance and analyst results remain governed separately | Defer; potential interaction layer | Live audio capture, consent and custody require a separate product/security phase; NX-M2/NX-X2 |
| React application shell | Current upstream includes a React UI and newer canvas/tool-streaming surfaces | The active local UI is `core/http/react-ui` | Investigation Workspace and forensic case surfaces are local extensions | Extend the current React app; do not fork a second frontend stack | Upstream merge drift and shared global branding are the main shell risks; NX-UX1 |
| Backend/gallery integrity | OCI gallery, acceleration variants and signature/integrity support | Backend index, galleries and integrity source are present | Approved forensic model-role catalogue is separate | Reuse admission and integrity controls | Installed does not mean approved or qualified; NX-Q2/NX-X2 |
| Distributed workers | LocalAI exposes distributed and worker-oriented primitives | Current repo also has forensic worker/NATS orchestration | Retained processing jobs, custody and artifact persistence are custom | Keep boundaries separate | No NX-UX1 worker change; future scale review in NX-ADV |

## Conclusions

1. The local tree already contains the important upstream runtime primitives;
   NX-UX1 does not need a new inference, agent, MCP, or frontend framework.
2. The Investigation Workspace should reuse the active React app, auth,
   workspace/case provider, forensic APIs, governed presentation adapter and
   existing media/detail components.
3. LocalAI supplies infrastructure, not forensic truth authority. Typed query
   understanding, certified operations, deterministic execution, retained
   evidence scope, citations and truthful result states remain NexusAI-owned.
4. The bounded NX-UX1A change is therefore an information-architecture and
   presentation shell correction. Upstream agent/tool/realtime expansion is
   explicitly deferred to its own phase and acceptance gates.

## Revisit triggers

Repeat this review before adopting a new upstream agent loop, MCP permission
surface, realtime evidence workflow, backend signing policy, or major React UI
merge. Each revisit must distinguish source presence from configured,
qualified, deployed and live-accepted capability.
