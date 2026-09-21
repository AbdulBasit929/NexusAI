# NexusAI R6.3-B Presentation and R6.4-A1 Corpus — Source Acceptance

Date: 2026-08-11  
Presentation contract: `forensics.agent-presentation/v1`  
Corpus contract: `forensics.family-query-answer-corpus/v1`  
Status: R6.3-B source accepted; R6.4-A started at A1; guarded live activation pending

## Accepted outcome

R6.3-B now transports a bounded typed answer contract from deterministic
forensic execution through local SSE, distributed NATS delivery and retained
case history. The contract declares deterministic execution authority, model
status and role, elapsed time, exact findings, metrics, bounded records,
citations, visualization descriptors, limitations, next actions and a compact
execution trace. It does not expose hidden reasoning or the unbounded raw tool
payload.

Agent Chat renders the contract as a clean analyst result instead of scraping
markdown: assurance and latency badges, metric cards, exact findings, a
horizontally-contained evidence table, media-aware visualization descriptors,
provenance, limitations, actionable follow-up questions and a collapsed “How
this was computed” trace. Live work exposes a truthful deterministic-workflow
badge, elapsed time and human-readable governed stages; raw tool details remain
collapsed. The same renderer is used for retained analyses.

R6.4-A1 begins the governed query/answer corpus with an embedded, versioned JSON
contract spanning CDR, IPDR, ANPR, subscriber, tower, financial and cross-family
questions, including one Roman-Urdu subscriber example. The capabilities API
publishes this corpus, and Agent Chat adds only entries whose family is actually
queryable in the active collection. Existing essential case-level actions stay
available, and suggestions are deduplicated and bounded.

## Acceptance evidence

- Attached operator rebuild: R6.3-A live accepted; API/worker/LocalAI build
  times 2.7/3.1/57.9 seconds; one LocalAI attempt; 6.41 GiB free RAM; rollback
  images and named volumes preserved; zero mutating activation requests.
- Focused presentation contract and callback test: PASS.
- Query corpus and capability handler tests: PASS.
- Affected agentpool package compile gate: PASS.
- Forensic sidecar package suite: PASS.
- Vite 8.0.16 production build: PASS, 669 modules.
- In-app browser against the new Vite source and live read-only API: governed
  case ready, active scope and query input usable, zero console errors. At
  390×844, document width equals viewport width and the suggestion grid is one
  column.
- Broad Go suites compile and execute, but PostgreSQL-backed specs remain
  unavailable on this Windows host because their test-container provider
  rejects rootless Docker. This environmental failure is not reported as a
  product regression.

## State mutation statement

No Docker rebuild, deployment, migration, upload, agent query, history import,
save, reprocess, model change, staging, commit, push or publication was
performed for this source slice. Browser acceptance used GET-only application
and capability traffic.

## Next gates

The first operator invocation failed safely during preflight because the
retained history URL was not exported to the host or duplicated in the runtime
env file. No service was stopped or rebuilt. The gate was hardened to reuse the
running Compose API container's already-deployed value in process memory only,
without printing or persisting it; explicit configuration remains authoritative.

1. Run the guarded `scripts/build_deploy_nexusai_r6_3_gate.ps1`. It now verifies
   the retained-history contract and versioned R6.4-A corpus after the proven
   sequential rebuild, while sending zero mutating requests.
2. After `R6.3BActivation=PASS`, run one separately authorized synthetic analyst
   question to prove the deployed typed presentation end to end and retain its
   request as explicit acceptance evidence.
3. Continue R6.4-A2: expand multilingual, paraphrase, ambiguity, no-data and
   unsupported-operation corpus coverage and add golden answer assertions per
   operational family.
