# LocalAI capability utilization matrix

Verified: 2026-08-06  
Legend: `unknown` means not proven in this reconciliation, not absent.

| Capability | Source | Configured | Live | Tested | NexusAI use | Analyst exposure | Security disposition | Action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Text generation / llama.cpp | yes | yes | yes | Phase 7.3 | bounded explanations | through governed workflows | no exact-fact authority | reuse |
| vLLM/vllm-omni | yes | no | no | no | future GPU profile | hidden | admin/benchmark only | defer |
| Transformers/SGLang | yes | no | no | no | candidate engines | hidden | admin only | defer |
| Embeddings | yes | yes | yes | accepted | case KB retrieval | implicit | never selectable as chat | reuse |
| Reranking | yes | no | no | no | retrieval candidate | hidden | benchmark gate | defer |
| Knowledge Base/RAG | yes | yes | yes | accepted | cited case context | Case Workspace/Agent Chat | case/tenant filters required | wrap |
| Built-in agents | yes | yes | yes | 65-operation matrix | specialist UI/runtime | governed subset | allowlisted tools | extend |
| Agent Hub | yes | unknown | not proven | no | admin provisioning | hidden from analysts | supply-chain review | admin only |
| Skills | yes | unknown | unknown | no | future specialist guidance | admin only | tool permission audit | audit |
| MCP/tools | yes | yes | unknown this session | prior source tests | admin and forensic tool parity | indirect | REST/MCP parity and auth | audit |
| Constrained grammars | yes | no | no | no | typed planner output | hidden | engine internal | benchmark |
| Vision/object/depth/face | yes | no | no | no | future visual forensics | unavailable | policy/benchmark gated | defer |
| Image generation | yes | no | no | no | no forensic fact role | hide | derived content only | disable for analysts |
| STT/VAD/realtime audio | yes | no | no | no | Phase 9 | unavailable | consent/resource gate | benchmark later |
| TTS | yes | no | no | no | disclosed derived reports | unavailable | never evidence fact | benchmark later |
| Video | yes | no | no | header inventory only | Phase 10 | inventory status only | bounded frame/clip processing | defer |
| Backend Gallery/OCI | yes | yes | yes | source/live admin surface | controlled installation | administrator | approval, signing, rollback | retain admin only |
| Hugging Face import | yes | no current action | no new download | no | candidate acquisition | administrator | approval/license/hash | retain admin only |
| P2P | yes | no | no | no | no current role | hidden | external-peer risk | disable |
| Distributed mode | yes | no | no | no | future enterprise scale | administrator | NATS/TLS/tenant design | defer |
| PostgreSQL/TimescaleDB | yes | yes | yes | schema verified | canonical structured facts | indirect | non-owner RLS | reuse |
| NATS JetStream | yes | yes | yes | health accepted | durable ingest dispatch | hidden | auth/TLS production gate | reuse |
| Multi-user/API keys/quotas/RBAC | yes | partial | local profile no | partial | enterprise access control | incomplete | critical release gate | extend |
| Integrated React WebUI | yes | yes | yes | responsive accepted | NexusAI reference UI | yes | capability-aware routes | migrate incrementally |
| Open Responses / WebSocket / WebRTC realtime | yes | unknown | not proven | source tests exist | future bounded realtime interaction | hidden | capability, resource and case gates | defer to R6/R10 |
| Fine-tuning and quantization | yes | no current action | no | no NexusAI acceptance | possible admin workflow | administrator only | supply-chain/resource/data approval | defer |
| Middleware routing, PII and cloud proxy | yes | partial | not reconciled | source tests | privacy/routing foundation | administrator | fail-closed policy and secret redaction | audit |
| Usage, traces, metrics and system state | yes | yes | partial | source/live health evidence | operations and audit inputs | administrator | sensitive telemetry and tenant scoping | extend in R16 |
| Object storage / S3 abstraction | yes | current forensic filesystem spool | filesystem live | retention tests | future deployment portability | hidden | custody, immutability and tenant scope | benchmark in R16 |
| Training/evaluation/stores | yes | no current NexusAI role | no | no | possible future admin/release tooling | hidden | dataset governance | defer |
| Biometrics (face/speaker/voice) | yes | no approved model | no | engine source tests only | policy-gated future candidate | hidden | legal/privacy/bias/human review | disabled |
| API explorer/instructions | yes | yes | live admin/developer surface | source tests | developer integration | developer/admin | must describe only actual capability | retain |

No source presence alone is an operational claim. The machine has exactly one
approved chat model and one approved embedding model; all other model/backend
roles remain approval- and benchmark-gated.
