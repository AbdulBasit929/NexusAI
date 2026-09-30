# Plan cache — the P3 latency gate is met, with the guarantee intact

**Date:** 2026-09-24 · Switch: `FORENSIC_PLAN_CACHE` (default **false**)
Rollback: `nexusai-forensic-records-api:rollback-before-plancache-20260924`

## Why this is exact rather than a heuristic

The plan completion is requested at `temperature: 0`, so the model's output is a
DETERMINISTIC function of its input payload. The cache keys on a hash of that exact payload,
which makes it memoization of a pure function — byte-identical input, byte-identical output.
It is not "similar questions reuse a plan".

**The cache removes an HTTP call, never a check.** Cached bytes re-enter the same decode,
bind and re-verification path: S6 shape, S9 SHAPE and the CONSTRAINT_APPLIED obligations all
run exactly as they would on a fresh completion.

**Self-invalidating.** The payload carries the schema, the system prompt and the context
block — question, family, literal values, date bounds and the issued field enum. A new field,
a changed display name or a different question is a DIFFERENT KEY, not a stale entry. There
is no staleness window. Tenant, collection and record type are also in the key as defence in
depth, so entries cannot be shared across scopes by construction.

Only a completion that decoded and finished cleanly is stored: caching a transient
malformation would make a momentary model hiccup permanent for that question.

## Result — measured, both passes scored

20 questions, run cold then repeated against the populated cache:

| | Cold | Cached | |
|---|---:|---:|---:|
| ACC-03 | 86.3 s | **1.2 s** | 74x |
| IPDR-04 | 90.9 s | **1.5 s** | 60x |
| TXN-01 | 89.2 s | **2.8 s** | 32x |
| CDR-05 | 105.6 s | **3.7 s** | 29x |
| CDR-02 | 110.0 s | **5.4 s** | 20x |

    cached pass: max 5,400 ms · p95 4,492 ms      P3 gate p95 < 15,000 ms -> PASS

**Correctness: 16 CORRECT · 4 CLARIFIED · 0 WRONG on both passes, with ZERO verdict
differences and ZERO `stated` flag differences across all 20 questions.**

Caveat on the cold column: several questions (SUB-01, TWR-01, ANPR-04) were already warm
from an earlier benchmark, so their "cold" figures understate the true first-run cost. The
86-110 s rows are the genuinely cold ones.

## Shipped OFF, like every other capability on this path

`FORENSIC_PLAN_CACHE` defaults to false, matching `FORENSIC_IR_FALLBACK`,
`FORENSIC_IR_ARBITRATION` and `FORENSIC_VERIFIED_ONLY`: its own switch, so it can be measured
and withdrawn independently.

Default-off is not timidity here. The cache changes an OBSERVABLE property — how many times
the model is called — and the existing spec "uses DYNAMIC_TYPED_PLAN exactly once and never
retries after unsupported" caught exactly that, failing with 1 call where it asserts 2. A
process-global memo that silently changes call counts is a poor default for a suite that
asserts them. That failure was a real design signal, not a flake, and the gate is the answer
to it.

## What this does and does not solve

**Does:** a repeated question costs nothing. Analysts re-ask, refine and re-run constantly,
and every clarification option in the UI is a pre-written question that will be asked many
times across cases with the same catalogue.

**Does not:** the FIRST time any question is asked still costs 60-150 s. The cache flattens
repetition; it does not make generation fast. A genuinely novel question on a cold cache is
still outside the gate, and that remains open — options 2-4 in
[`RESULTS.md`](RESULTS.md) are unchanged.

**Does not survive a restart.** Entries are in memory only. A redeploy re-pays the cold cost
once per distinct question. Persisting them is possible — the key is a content hash and the
value is the model's own proposal, no evidence — but that is a separate decision.

## Verification

- `semantic_plan_cache_test.go` — 7 tests: exact byte round-trip, caller isolation in BOTH
  directions, scope separation (tenant/collection/record type), payload-driven
  self-invalidation, bounded eviction, refusal of empty entries, and inert-when-disabled.
- Full package suite green.
- Two scored live passes, verdict-by-verdict comparison, 0 differences.

---

## Persistence — the latency property now survives a redeploy

**Switch:** `FORENSIC_PLAN_CACHE_DIR`, **empty by default**. Nothing is written unless an
operator sets it deliberately, and it is inert whenever `FORENSIC_PLAN_CACHE` is false.

### What is written, stated plainly

Each file holds ONE model completion: the proposed typed algebra for one question. **That
plan can carry the question's literal values** — a plan for "How many calls did 03001234567
make?" contains that number as a filter.

It is not evidence, and the same plan is already returned in the response audit of every
request that used it. But writing it to a volume is a different act from returning it in a
response, so this is opt-in rather than on by default. Nothing else is written: no rows, no
citations, no results.

### Measured across a real restart

    populate (cold)      ANPR-04  55 s     CDR-16  54 s
    docker restart       "restored semantic plan cache entries=2 skipped_unusable=0"
    after restart        ANPR-04   0 s     CDR-16   1 s

Answers unchanged and correct: "The camera with the highest count is CAM-12 (192)" and the
CDR source-file breakdown.

### Properties

- **Atomic writes** — temp file plus rename, so a partially written file can never be read
  as a completion and served as though the model had produced it.
- **Validated on load** — exactly one choice carrying content, mirroring what the generator
  requires of a live response. A truncated or corrupt file is skipped AND removed, because a
  malformed completion from disk would look identical to one from the model but be permanent
  instead of transient.
- **Bounded on disk** — eviction removes the file as well as the memory entry, or the bound
  would hold only in memory and the directory would grow across restarts.
- **Announced at startup** — `restored semantic plan cache … entries=N skipped_unusable=M`.
  A cache that silently fails to load is indistinguishable from one that is working, and its
  whole purpose is a latency property someone will check. This caught a real deployment bug
  immediately: Git Bash MSYS path translation rewrote `/data/forensic/spool/plan-cache` into
  `C:/Program Files/Git/data/...`, which the log line exposed at once. Set this variable from
  PowerShell or an env file, never from Git Bash.
- **No staleness window** — the key is a hash of the exact model payload, which carries the
  schema, the prompt and the issued field enum. A changed catalogue is a different file, not
  a stale one; orphans are never read and are evicted by the bound.

### Correctness after every change today

12-question scored sample: **9 CORRECT · 3 CLARIFIED · 0 WRONG**, with CASE-01, IMG-04 and
X-01 clarifying by design exactly as before.
