# NexusAI Phase 7.2 Agent Chat reliability closure

Date: 2026-08-05  
Status: PASS

## Closed defects

The supplied screenshots exposed two distinct failures: some natural questions
could remain on `Working...`, and broad wording could select a plausible but
wrong template or even the wrong collection. The closure adds complete natural
intent coverage, governed fallback routing, end-to-end request correlation, and
bounded execution.

- `Show me an overview of this case` now selects `collection_overview`.
- `How are the CDR events divided by call type?` selects
  `call_type_breakdown` and completes in Agent Chat.
- Cross-family correlation, entity timeline, evidence inventory, and executive
  report paths return the selected collection and template explicitly.
- Status, stream, error, and final SSE events carry the originating
  `message_id`; the React UI ignores stale events and clears progress only for
  the active request.
- Forensic agents cannot fall through to legacy generic records tools.
- Unknown natural forensic wording is handled by the governed hybrid planner.
- Native chat execution has a 210-second ceiling, so a failed model cannot leave
  the UI indefinitely busy.

## Acceptance evidence

| Check | Accepted result |
|---|---:|
| Natural operations through records API | 65/65 |
| Real Agent Chat + correlated SSE | 65/65 |
| Agent Chat route mismatches | 0 |
| Agent Chat incomplete/stuck requests | 0 |
| Agent Chat p95 / maximum | 9,815 / 22,916 ms |
| Final screenshot-path regression | 6/6 |
| Model-assisted specialist agents | 6/6 |
| Compliant model interpretations | 3 |
| Explicit safe deterministic fallbacks | 3 |
| Model run minimum / maximum | 38,219 / 49,474 ms |

The model is not an authority for exact facts. SQL findings are rendered first
and independently. Model input excludes raw tables, exact rows, identifiers,
dates, counts, filenames, and locators. Returned synthesis must contain only
bounded interpretation and limitations; numeric claims and family-specific
unsupported inferences are rejected. A rejection is visible as a policy
fallback and never suppresses the deterministic result.

## Verification

- Forensic API suite: 221 executed and passed; 2 intentionally skipped.
- Focused Go deterministic routing suite: PASS.
- React ESLint: zero errors (14 pre-existing warnings).
- React production build: PASS, 658 modules.
- In-app browser: `call_type_breakdown` completed for
  `nexusai-forensic-demo`, nine rows, no `Working...` state.
- Guarded rebuild: PASS; named volumes and rollback images preserved.

Artifacts:

- `reports/runtime-activation-20260805/phase7.2-agent-chat-full-matrix.json`
- `reports/runtime-activation-20260805/phase7.2-agent-chat-regression-matrix.json`
- `reports/runtime-activation-20260805/phase7.2-agent-chat-llm-matrix.json`
- `scripts/smoke_forensic_agent_chat_full_matrix.ps1`
- `scripts/smoke_forensic_agent_llm_matrix.ps1`

Runtime images:

- LocalAI/UI: `sha256:fcb3355210d78235c677fc3b6f86fa13122c4271c54518cef3beace2b47ecf29`
- Forensic API: `sha256:3a6c39db71ca6a47e30538a01e9a865fbdbd5ef0130edd4e5420f4ab190976ec`
- Worker: `sha256:b8db7e3b430cc59b49a66d8983769427f95258546011f325df52c193db1b203d`

All five runtime containers were running at closure; LocalAI, the worker, NATS,
and PostgreSQL reported healthy.
