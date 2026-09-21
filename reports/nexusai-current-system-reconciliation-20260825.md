# NexusAI vNext authoritative reconciliation — 2026-08-25

Status: **NX-R0 complete; implementation not started.** This report is the
controlling evidence summary for the next approval decision. The detailed
program is governed by `docs/roadmap/nexusai-next-generation-roadmap.md` and
its ledger. Earlier STIM/MMV reports remain evidence, not current direction.

> **Continuation update:** the later 2026-08-25 team-lead directive supersedes
> the sequencing described below with breadth-before-depth:
> NX-1 → NX-UX1 → NX-A1 → NX-B1 → NX-B2, then justified depth. NX-1 source is
> now complete and deployment-gated. Use the current roadmap and ledger for
> execution status.

## Executive finding

NexusAI already has a credible governed-forensics core: immutable evidence and
custody, typed structured adapters, deterministic operations, case isolation,
Fact Packets, citations, specialist agents, and a usable four-area analyst UI.
The weakness is incomplete reconciliation between source, deployed runtime,
public result semantics, LocalAI capability discovery, and presentation.

The deployed system has no P0. It has **three P1s**:

1. Positive deterministic rows can still be exposed with a contradictory public
   row count. This is fixed and tested in source, but not activated live.
2. A completed grouped-video analysis with zero plates loses its typed
   `complete_zero_results` state in Ask/History. This is fixed and tested in
   source, but not activated live.
3. A UUID segment (`BAB0`) is extracted as an ANPR plate and routes an
   evidence-specific video question to `anpr_timeline`. This remains open in
   source and live.

The exact next slice is **NX-1 Foundation Truth Closure**: correct typed
identifier extraction, add a regression oracle, then—under separate activation
approval and the 6 GiB free-RAM gate—rebuild only the forensic API and
LocalAI/UI, and live-certify all three P1s. No worker, database, retained
evidence, model, migration, or deployment mutation belongs in the source slice.

## Authority and repository state

- Repository: `C:\Users\sheik\Workspace\Office\Projects\NexusAI`
- Branch: `codex/forensic-hybrid-checkpoint-20260723`
- HEAD: `40717b83510c08db25dc26b9d6674bf46db363ac`
- Git history is a single imported root snapshot. No LocalAI upstream remote or
  ancestry exists locally, so an exact upstream merge-base cannot be proven.
- The worktree contains extensive pre-existing modified and untracked project
  work. NX-R0 preserved it and did not stage, commit, fetch, merge, or clean.
- P1-1 and P1-2 are source-fixed. P1-3 remains in
  `core/services/agents/forensic_direct.go`: regex target extraction precedes
  UUID semantics.

## Deployed runtime baseline

Read-only inspection found five healthy containers: LocalAI/UI API on 8080,
forensic records API on 8091, protected forensic worker on 9109, NATS 2.11, and
PostgreSQL/Timescale PG16. Runtime readiness, worker metrics, and NATS health
returned HTTP 200. The LocalAI version endpoint reports the Nexus snapshot hash,
not a semantic LocalAI release.

Five models are configured: Qwen3 4B Instruct, Qwen3 0.6B embeddings,
Faster-Whisper small Urdu, Whisper tiny, and YuNet/SFace. Installed backend
families are llama.cpp, Whisper, Faster-Whisper, and face detection. Gallery
entries are possibilities, not installed or admitted capability.

| Retained measure | Count |
|---|---:|
| evidence / versions | 51 / 51 |
| jobs / active jobs | 64 / 0 |
| records | 22,207 |
| CDR / generic records | 13,649 / 8,556 |
| artifacts / knowledge assets | 441 / 47 |
| custody events / record entities | 190 / 114,103 |

Jobs were 60 completed and four dead-letter. Records include CDR, IPDR,
subscriber, tower/location, ANPR, access-log, transaction, and generic sets.
NX-R0 made no retained-state changes.

## Capability and family reconciliation

The current operation registry exposes 67 templates: five certified (A), 51
limited (B), and 11 engineering-only (C). Certified operations are the three
core CDR analyses plus evidence lookup and evidence-package summary. IPDR,
subscriber, tower, ANPR, and most cross-family work exist but are not fully
certified. Financial/transaction, log/access-log, and general tabular evidence
are retained but lack first-class family operations and specialist certification.

The authoritative row view is
`reports/nexusai-family-capability-maturity-matrix.json`. Ingestion breadth
exceeds governed query depth. CDR is mature; IPDR/subscriber/tower are
operational but limited; ANPR/media/OCR/ASR are useful but uneven; documents,
logs, financial data, diarization, VAD, audio events, entities, relationship
graphs, and timeline fusion need bounded slices.

## Query, agent, and tool architecture

The deterministic-first contract remains:

`question -> typed intent/identifiers -> authorization -> deterministic plan ->
allowlisted operation -> canonical facts -> Fact Packet -> optional LocalAI
synthesis -> typed answer/citations/history`

LocalAI may host inference, Responses transport, tool parsing, MCP, and an
optional bounded agent loop. NexusAI owns case/evidence policy, operation
authorization, deterministic analytics, canonical facts, result states,
citations, and certification. Specialists do not call one another directly;
the Nexus orchestrator invokes allowlisted tools. Analyst MCP receives neither
administrative tools nor ambient credentials.

## LocalAI upstream finding

Official upstream is **v4.9.0 (2026-08-20)**. The local runtime cannot be
declared equivalent because it lacks a semantic upstream version and
`/v1/models/capabilities` returns 404 despite current upstream documentation.
Upgrade work is an evidence-driven admission slice, not an assumed drop-in.
See the upstream reconciliation and reuse/change matrices.

## Analyst UX finding

Home/Data/Ask/History is a strong base and showed no document-level horizontal
overflow at 390, 820, 1024, or 1440 px. Source detail is rich and positive ANPR
is well cited. Default flow still exposes template/route names, duplicates
facts, uses generic History labels, and inconsistently presents completed-zero
media. Source detail can say processing result unavailable beside available
outputs; historical notes compete with current readiness. Urdu/RTL remains a
component-level gate, not something inferred from LTR responsive checks.

## Security, performance, observability, and testing

- Preserve deny-by-default auth, tenant/case/evidence scope, custody, source
  fields, and candidate-only semantics. Upstream auth complements but does not
  replace forensic policy.
- Audit prompts, plans, tools, operation versions, packet IDs, model/revision,
  latency, token counts, and redacted failures.
- Budget planning, operation, packet, and synthesis stages; bound concurrency;
  support cancellation and deterministic fallback.
- Free memory was below the mandatory 6 GiB activation gate. No build/restart is
  authorized by this report.
- Certify against independent retained-row/database oracles, adversarial typed
  states, auth/isolation, Urdu/Roman-Urdu/RTL, responsive UX, resource budgets,
  and rollback proof. Demo only certified claims and graceful limits.

## Decision

NX-R0 is complete. **Do not deploy, download models, migrate data, reprocess
evidence, or begin broad implementation.** Approve only NX-1 source work first.
Activation is a later explicit decision after source verification, RAM/rollback
preflight, and confirmation that only API plus LocalAI/UI will change.
