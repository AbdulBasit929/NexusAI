# NexusAI vNext certification strategy

## Breadth-first product baseline

Before depth promotion, every targeted structured, document, image, audio,
video, and knowledge family must prove one or a few useful end-to-end paths:
Evidence → Finding → Ask → Source → Activity. Certification includes positive,
zero/negative, truthful unavailable behavior, representative lawful evidence,
basic performance, and the absence of fake controls. Existing deep operations
need not all become A-certified before this baseline.

## Evidence classes

Every promoted capability needs: source tests, independent oracle, API contract,
runtime observation, analyst UI acceptance, security/isolation proof, resource
budget, rollback proof, and an evidence bundle with immutable identifiers.
Source-only success is not live certification.

## Independent oracles

- Structured families: database/retained-row computations independent of the
  production adapter and operation implementation.
- Media: known-source timestamps, labeled plates/text/speech/events, negative
  controls, exact artifact hashes, and accepted/rejected accounting.
- Retrieval: frozen questions, relevant passages, citation correctness, recall,
  rank, and abstention.
- Entity/graph/timeline: explicit ground-truth links, non-links, time zones,
  duplicates, conflicts, and forbidden identity inference.

## Test matrix

| Layer | Minimum gates |
|---|---|
| Contracts | schema/version/backward compatibility; all nine result states |
| Routing | typed IDs before regex; multilingual/paraphrase/follow-up/adversarial inputs |
| Authorization | tenant/case/evidence isolation; unauthorized distinct from empty |
| Operations | deterministic rows/counts/citations/quality; timeout/cancel/idempotency |
| Models | complete model card; quality/calibration/resource/privacy/rollback |
| LocalAI | version/discovery/routes/auth/tools/Responses/MCP/backend compatibility |
| UI | 390/820/1024/1440, RTL/mixed script, keyboard/screen reader/zoom, state reopen |
| Reliability | concurrency, partial failure, restart, retry, degraded deterministic fallback |
| Observability | correlation IDs, redaction, plan/tool/packet/model latency and failures |

## Severity and promotion

P0 is cross-tenant exposure, evidence/custody corruption, or unsafe destructive
action. P1 is wrong fact/routing/state/citation in an ordinary analyst path.
No phase exits with P0/P1. A capability is A-certified only when independent
oracle, API, live runtime, UI, and rollback evidence agree. Otherwise it remains
B limited, C engineering-only, candidate-only, or absent.

## NX-1 regression pack

Use a full video evidence UUID whose compact segment resembles a plate. Assert
typed evidence extraction, grouped-video operation, no plate parameter, and the
same scoped evidence through API, Ask, and History. Also assert positive rows do
not report zero and completed-zero remains `complete_zero_results`. Recheck
retained counts and protected service identities before and after activation.

## NX-1 post-activation live certification

The operator activation output is the first authority after Codex reopens. If
it reports `NX1Activation=PASS`, do not rebuild or recreate services again.
Reconfirm the activated image IDs, protected container identities, retained
counts, five-model inventory, profiles, named volumes, and all health surfaces
from the recorded output and independent read-only observations.

Close the three live P1s as one bounded certification slice:

1. Prove a retained positive operation reports authoritative rows and the same
   non-zero count through the operation endpoint, public API presentation, Ask,
   Activity/History reopen, and the analyst UI.
2. Prove a completed-zero grouped-video result remains
   `complete_zero_results` with its processing state, row count, operation ID,
   citations, Ask response, Activity/History reopen, and analyst UI. Also prove
   a true no-match and an unavailable/not-processed case remain distinct.
3. Use the full regression evidence UUID whose compact segment resembles a
   plate. Prove the UUID is retained as evidence, no plate argument is inferred,
   and an explicit plate remains a separate argument when supplied.
4. Exercise representative English, messy English, Roman Urdu, Urdu script,
   and follow-up wording across positive, no-match, and zero-result paths.
5. Compare all factual rows/counts against an independent retained-state oracle
   and require resolvable source citations at operation, answer, and UI layers.

NX-1 exits only when `OpenP0=0`, `OpenFoundationP1=0`, every runtime and
retained-state preservation check passes, and the immutable certification bundle
records the activated image/container IDs and exact oracle inputs. Any mismatch
keeps NX-1 live-pending and blocks NX-UX1.
