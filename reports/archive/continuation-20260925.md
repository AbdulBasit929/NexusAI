# NexusAI — Current State

> **CURRENT STATE ONLY. Rewrite it; never append. Hard cap 300 lines.** History:
> [`reports/archive/continuation-20260921.md`](reports/archive/continuation-20260921.md).

**Last updated:** 2026-09-24 (WI-14 · PHASE 1 COMPLETE, latency in progress) · **Branch:** `codex/forensic-hybrid-checkpoint-20260723` · **HEAD:** `efbbd84`

> **TWO AGENTS WRITE THIS FILE.** Everything above `## CODEX_UI_TRACK` belongs to the
> backend track; that section belongs to Codex. Neither edits the other's.

---

## 1. Active work items

| ID | Owner | State | Files owned (exclusive while open) |
|---|---|---|---|
| **WI-0** | claude | **COMPLETE — resolved: PROCEED, REINFORCED (Arm A)** | `reports/ir-spike-20260921/**`, `scripts/ir_spike/**` |
| WI-1 | claude | **COMPLETE — verified live** | `api/forensic_records/deterministic_semantic_compiler.go`, `p1_correctness_test.go` |
| WI-2 | claude | **COMPLETE — verified live** | `api/forensic_records/stim_fact_packet.go`, `answer_presentation.go` |
| WI-3 | claude | **COMPLETE — verified live, one regression (CDR-13)** | `api/forensic_records/operation_applicability.go`, `deterministic_semantic_compiler.go` (measure block) |
| WI-5 | claude | **items 0-5 DONE · item 6 (D8) open** | `api/forensic_records/query.go` (routing + executive answer only), `p1_correctness_test.go` |
| WI-4 | **claude** (taken from codex) | **COMPLETE — layer curated, loaded, validated; not yet wired into the compiler** | `semantic_layer/**`, `api/forensic_records/semantic_layer*.go` |

At most **two** active items, one per agent, disjoint file sets. Definitions: [`MASTER_EXECUTION_PROMPT.md`](docs/work/MASTER_EXECUTION_PROMPT.md) §9.

### WI-0 to WI-8 — complete, archived

Full detail: [`continuation-wi0-wi8-20260923.md`](reports/archive/continuation-wi0-wi8-20260923.md).
**WI-0** IR spike, 86 live calls: Arm A 74.3% execution-equivalent, and the **KEYSTONE HOLDS**
— 0 plans named a field outside the issued enum, so a hallucinated field is structurally
impossible. **WI-1..WI-5** compiler correctness, the curated layer (7 entities, 83
source_names SQL-verified), and the ladder family guard. **WI-6** fixed the INSTRUMENT first
— three runs were mis-measured; the harness needs opt-in `--analyst-text` and **anything
graded without it is NOT comparable**. **WI-7** plan-exact 16% -> 68%; six defects, all ours,
found OFFLINE by `family_catalog_audit_test.go` (~25 s, no model calls) — **run it before any
live run**. The one worth remembering: curated NUMBER never mapped to DECIMAL, so `MAX` was
lexicographic and 9,200 beat 75,000, misdiagnosed for weeks as model instability. **WI-8**
fallback + arbitration ON: IMPROVED 3, REGRESSED 0.

**Rejected after measuring in those items — do not retry:** a layer-vocabulary family
resolver (fixed 3, misrouted 6); arbitrating CONFIDENT answers on a shape mismatch (3 wrong /
2 CORRECT).

### WI-9 — verified-only + rescue: structured side to ZERO, 2026-09-23

Evidence: [`RESULTS.md`](reports/golden-ir-20260923-final/RESULTS.md); detail archived with
WI-0..WI-8. **confident-wrong 23 -> 5 · correct-or-clarified 61% -> 90%.** STRUCTURED
analytics reached 0 (was 13). A structured question is answered from a plan that passed
S6/S9/CONSTRAINT_APPLIED or not at all; when an unverified route is about to answer,
arbitration first tries to EARN a verified plan. **14 questions answer from a GENERATED
PLAN, not a template.**

**Rejected after measuring, do not retry:** cross-verification (cannot judge template
answers — they expose no single comparable value; `FORENSIC_IR_CROSSCHECK` is inert);
overriding confident answers on a shape mismatch (3 wrong / 2 CORRECT); withholding every
unverified structured answer (4 wrong / 3 CORRECT lost).

**The lesson that generalised:** COUNT_DISTINCT made a WRONG plan possible where the question
used to clarify safely — **a new capability must arrive with the check that bounds it**, now
an S9 obligation. And an HTTP-500 survived because the test asserted the SQL STRING and never
ran it: **if a test claims execution, execute.**

### PHASE 1 GATE — MET, 4 of 4, measured 2026-09-23

Evidence: [`RESULTS.md`](reports/golden-ir-20260923-wi12/RESULTS.md). Full 62-question run
with `--analyst-text`.

    CORRECT 43 · CLARIFIED 18 · MANUAL 1 · WRONG 0 · ERROR 0
    confident-wrong 5 -> 0      correct-or-clarified 90% -> 98.4%
    IMPROVED 5 · REGRESSED 0 · unchanged 57

HTTP 500 = 0 **PASS** · correct-or-clarified 98.4% **PASS** · confident-wrong 0 **PASS** ·
answer stated **PASS**. **Phase 1 is complete.**

**The 18 clarifications are COVERAGE, not safety.** Most sit in the audit's `no_family` set,
where the generator never receives an enum so no verified plan is possible. Each point of
family coverage converts one back into an answer at no safety cost. **P3 deletes the ladder**,
removing the competitor rather than out-voting it.

**Not ready for P3:** latency. CDR-11 273 s, CDR-02 114 s, DOC-01 103 s; the P3 gate is
p95 < 15 s.

### WI-10 / WI-11 — the last five, and options, 2026-09-23 · LIVE

Detail archived; full rationale lives in the code comments in `ir_crosscheck.go` and
`clarification_options.go`, which is where the next agent will actually be reading.
Reports: [`WI-10`](reports/golden-ir-20260923-wi10/RESULTS.md) ·
[`WI-11`](reports/clarification-options-20260923/RESULTS.md).

**WI-10** three guards, each scored OFFLINE against the 62 saved responses BEFORE being
written: an answer naming a target the executed plan never filtered on (DOC-04 counted all
218 ANPR rows and said "involving ABC-123") · a page size reported as a count (CASE-01 said
"20" where its own payload reported `total_count` 12,912; IMG-04's true count is 22) · a
single-family search asserting case-wide absence (X-01, a false negative, where the gold
shows the number DOES appear in a PDF and a WAV).

**The generalisable lesson: S9 verifies a plan against ITSELF, not against the question it
answers.** DOC-04's plan was arithmetically sound and passed verification while counting
everything and attributing it to one plate.

**WI-11** `clarification.options` was always `[]` while 18 of 62 responses clarify. Contract
bumped to **`forensics.clarification-request/v2`**: options are now `[{label, query}]`, and
`query` is the exact question to re-send so the UI owns no phrasing. Labels come only from
curated `display_name`s; only `sensitivity: NONE` fields are offered; only `groupable: true`
fields become breakdowns; media scope labels come from UX §5's chip vocabulary and each media
query shape is one the corpus PROVES answers today. X-01's first option returns the PDF its
gold expects, in one click.

**Rejected on the numbers, do not retry:** dropping the `SourceNative` exemption from the
media guard (fires on DOC-04 WRONG *and* DOC-01 CORRECT — a 1:1 trade); withholding when a
quantity question resolves to `bounded_records_table` (15 fire — 13 CORRECT, 2 WRONG).

**Gold-set caution — FALSE CORRECTs exist.** The oracle's `contains` check searches the whole
response blob, so a filename in a coverage listing scores CORRECT while the analyst is shown
nothing. DOC-01 is one (`contains ["MN1367"]`, answered about ANPR). AUD-01 and AUD-03 were
two until WI-12. Read any filename-expectation check against the analyst-facing text.

### WI-12 — audio was never broken, 2026-09-23 · LIVE

Detail: [`NOTES.md`](reports/golden-ir-20260923-wi12/NOTES.md). **D5 was misdiagnosed as a
retrieval defect.** Retrieval found, scored, cited and located every match. Two PRESENTATION
defects threw it away:

1. **The completeness caveat REPLACED the answer.** "The text search is incomplete. Any
   listed observations are supported matches..." was returned as the ENTIRE answer whenever
   `result_state` was SEARCH_INCOMPLETE — while listing none, because returning it discarded
   the finding. Now it is the whole answer only when there is nothing to report; otherwise
   the finding is stated and the caveat appended as its limit. **The first attempt dropped
   the disclosure entirely and a ginkgo spec caught it — it must be BOTH.**
2. **Chronology outranked relevance in the result sort.** Segments of one recording share an
   evidence and run, so they sorted by start time and score never broke the tie; truncation
   then kept the EARLIEST. AUD-02 kept "peanut chilies" at 5.56 s and discarded "especially
   Japanese coconut sugar" at 11.28 s. Score now outranks chronology; a genuine browse is
   unaffected because every score is then equal.

`transcriptExecutiveAnswer` states WHICH recording and WHEN from the citation locator of the
segment that actually carried the match. No usable start time means no time claim; no citable
source means no answer at all.

**Still weak:** IMG-01 answers "Retrieved 1 cited evidence result from the selected case
scope" without naming the image. `image_ocr_search` deserves the same citation-naming answer.

**Known flake:** `TestForensicRecordsSynthesis` failed twice while a container redeploy had
the synthesis model under load, then passed 4 consecutive runs in isolation. Load-correlated,
not reproducible standalone.

### WI-13 — curated VALUE display names in answers, 2026-09-24 · LIVE

The UI review surfaced it: the answer said `GPRS: 5,863 ; SMS: 1,108 ; CALL: 930` above a
table saying `Data session`, `SMS`, `Call` — the same values in two vocabularies on one
screen. The DIMENSION was curated ("Call type"); its VALUES were not.
`curatedValueLabel` now resolves enumerated values through the layer in both the breakdown
and the ranked-top headline. An uncurated value returns UNCHANGED — a raw identifier is a
truthful fallback; inventing a label would be worse.

Measured: the 8 affected questions (CDR-12/13/16, SUB-02, ACC-03, ANPR-04, IPDR-05, TXN-02)
scored **identically before and after** — same verdicts, same `stated` flags. Zero cost.

**Oracle caution (third instance):** CDR-13's gold expects RAW `INCOMING`/`OUTGOING`, which
contradicts the product rule that analysts read display names. It still scores CORRECT only
because the raw values survive in the response blob. Together with the filename checks
(DOC-01, AUD-01/03), the gold rewards machine vocabulary in three places. **Do not "fix" the
compiler to satisfy an oracle that contradicts the UX contract** — fix the oracle.

### WI-14 — LATENCY: wrong serving path, blind instrument, 2026-09-24 · LIVE

Evidence: [`RESULTS.md`](reports/latency-20260924/RESULTS.md).

**ALL INFERENCE WAS GOING TO A STALE BINARY.** `nexusai-api-1` (rebuilt 2026-09-22 for the
GBNF fix) had NO network and could not bind 8080 because the rollback container held it. The
API calls `host.docker.internal:8080`, so the pre-fix 2026-09-03 binary served everything —
while holding **5.0 of 7.6 GiB**. Fixed by stopping the stale container (RETAINED, not
removed); the current one then bound 8080 on its own mapping.
**Correction:** WI-7 credited part of 16% -> 68% to the GBNF `maxItems` fix. That fix was
never in the serving path; the gain came from the five Go-side defects.

**The instrument was blind.** `llm_latency_ms` is set only inside bounded synthesis, so
CDR-02 reported `total=119,835 ms · db=988 ms · llm=0` — 119 s attributed to nothing.
`ir_generation_ms` now times generation inside `resolveSemanticDynamicPlan`, the one place it
is spent. Timing the WRAPPER first was wrong and the measurement said so (3 of 4 probes read
0 ms while taking 29-75 s). Where generation runs it is **~93% of the request**.

**`semanticEmbeddingCatalogBatch` was 1** — a 66-descriptor catalogue issued 66 sequential
`/v1/embeddings` calls, ~17 s per question, invisible to every telemetry field. LocalAI
batches correctly (3 inputs, 3 order-preserved vectors, 0.9 s vs ~6 s singly). Now 32:
dozens of calls per request became **1 in four minutes of probing**.

    DETERMINISTIC path  0.3-4.2 s   INSIDE P3's p95 < 15 s
    MODEL path          74-152 s    OUTSIDE it by 5-10x

**Latency measurements in this environment are NOT trustworthy to within an order of
magnitude.** The same query measured 16.8 s, then 150/91/74 s on three consecutive runs.
LocalAI sat at 791% CPU with memory healthy and the host at 23% of 16 cores, so it is not
starvation; its logs show `proxy error: context canceled` — calls abandoned after the time
was already spent. **Benchmark with a fixed methodology (N repetitions, warm model, recorded
host load) before drawing any latency conclusion.**

**MEASURED 2026-09-24** ([`RESULTS.md`](reports/latency-bench-20260924/RESULTS.md)), 20
questions, harness `scripts/nexusai_latency_bench.py`:

    GENERATE a plan     9 of 20 (45%)   median 138.4s   ir_generation ~100% of it
    deterministic only 11 of 20 (55%)   median   3.4s
    p95 across sample 173.7s            P3 gate p95 < 15s

**This is a POLICY question, not a tuning one.** SEVEN of the nine generating questions are
answers the ladder ALREADY produces correctly (CDR-02/16, SUB-01, TWR-01, IPDR-04, ACC-03,
ANPR-04). They generate because verified-only triggers arbitration to EARN a verified plan
for an answer that would otherwise be stated on no authority. So the measured price of the
verification guarantee is **~45% of questions paying ~138 s**. That guarantee is what took
confident-wrong 23 -> 0. **Do not trade it away for latency without the owner's decision** —
that trade has been refused three times already.

Options costed in the report: (1) plan cache keyed on question shape — preserves the
guarantee entirely, highest value; (2) async generation — needs a UX decision because the
answer would change after being shown; (3) shrink the enum/prompt — bounded by WI-7, which
showed a starved enum makes plans worse; (4) narrow when arbitration fires — needs an
"at risk" signal, and shape-mismatch was measured and REJECTED as one (3 wrong / 2 CORRECT).
**Not** an option: accepting unverified answers, which is the state Phase 1 removed.

**The instrument is now COMPLETE.** The audit is seeded at request entry in `query.go`
BEFORE any planning, so cost recording no longer depends on which path happens to create one.
Measured after the fix: **unaccounted fell from 74-154 s to 0.1-0.2 s** — 99.8% of every slow
request is explained, and it is ALL generation.

    CDR-16   ir_gen 30.9s / 90.9s   unaccounted 0.1s   spread 2.89x
    ANPR-04  ir_gen 66.0s / 73.3s   unaccounted 0.2s   spread 1.11x

**Generation time itself varies up to 2.89x on IDENTICAL input**, which is why single samples
misled twice. `scripts/nexusai_latency_bench.py` exists so this cannot happen again: it
discards a warm-up, repeats N times, reports min/median/max with the spread ratio, samples
container load around every run, and prints the UNACCOUNTED remainder — an unaccounted
majority means the instrument is incomplete and no tuning conclusion may be drawn.

Correctness unchanged: 14-question sample = 11 CORRECT · 3 CLARIFIED · 0 WRONG.

**NEXT LEVER:** five questions made five `/v1/chat/completions` calls while reporting
`llm=0` AND `ir_generation_ms=0` — ~16 s each, timed nowhere. Candidates:
`query_hybrid_planner.go:497`, `open_ended_semantic_planner.go:361`, `semantic_slot_assist.go`.
**Instrument before optimising** — that order paid off twice in one session.

### WI-15 — PLAN CACHE: P3 latency gate MET on the cached path, 2026-09-24

Evidence: [`PLAN_CACHE.md`](reports/latency-bench-20260924/PLAN_CACHE.md).
Switch **`FORENSIC_PLAN_CACHE`, default false**.

**Exact, not heuristic.** The completion is requested at `temperature: 0`, so it is a
deterministic function of its payload; the cache keys on a hash of that exact payload and
memoizes a pure function. Cached bytes re-enter the SAME decode/bind/re-verify path — **the
cache removes an HTTP call, never a check.** Self-invalidating: the payload carries the
schema, prompt and issued field enum, so a changed catalogue is a DIFFERENT KEY, not a stale
entry. Tenant/collection/record type are in the key as defence in depth.

    ACC-03 86.3s -> 1.2s (74x) · IPDR-04 90.9s -> 1.5s (60x) · TXN-01 89.2s -> 2.8s (32x)
    CDR-05 105.6s -> 3.7s (29x) · CDR-02 110.0s -> 5.4s (20x)
    cached pass: max 5,400ms · p95 4,492ms      P3 gate p95 < 15,000ms -> PASS

**16 CORRECT · 4 CLARIFIED · 0 WRONG on both passes — ZERO verdict differences and ZERO
`stated` differences across all 20 questions.**

**Shipped OFF deliberately.** The cache changes an OBSERVABLE property — how many times the
model is called — and the spec "uses DYNAMIC_TYPED_PLAN exactly once and never retries after
unsupported" caught exactly that, failing with 1 call where it asserts 2. **That was a real
design signal, not a flake**, and the gate is the answer to it.

**PERSISTED** via `FORENSIC_PLAN_CACHE_DIR` (empty by default). Measured across a real
`docker restart`: ANPR-04 55s -> **0s**, CDR-16 54s -> **1s**, startup logging
`restored semantic plan cache entries=2 skipped_unusable=0`. Atomic writes; validated on load
(a corrupt file is skipped AND removed, since a malformed completion from disk would be
permanent rather than transient); eviction removes the file too, so the bound holds on disk.

**Opt-in deliberately: a persisted plan carries the question's literal values** — a plan for
"how many calls did 03001234567 make" holds that number as a filter. Not evidence, and
already in every response audit, but writing it to a volume is a different act.

**Startup announces what it restored**, which immediately caught a real deployment bug: Git
Bash MSYS path translation rewrote `/data/forensic/spool/plan-cache` to
`C:/Program Files/Git/data/...`. **Set that variable from PowerShell or an env file, never
from Git Bash.**

**What it still does NOT solve:** the FIRST ask of any question costs 60-150 s. The cache
flattens repetition; it does not make generation fast.

### WI-16 — evidence intake and the processing wait, 2026-09-24 · CLAUDE BUILT UI

**CROSS-TRACK NOTICE — CODEX MUST READ THIS BEFORE EDITING THE UI.** Codex hit its usage
limit, so the backend track built three UI items in `apps/investigation-workspace/**`. These
files are Codex's to own from here; **do not clobber them, build on them.**

    NEW   src/pages/NewCasePage.jsx            /cases/new
    NEW   src/components/AddDataDropzone.jsx   real multipart intake
    NEW   src/components/ProcessingWait.jsx    polls /collections/status
    NEW   src/components/AddDataDropzone.vitest.jsx · src/pages/NewCasePage.vitest.jsx
    EDIT  src/lib/apiClient.js (uploadEvidence) · src/router.jsx · src/pages/CasesPage.jsx
          src/pages/EvidenceListPage.jsx · src/styles/workspace.css · .claude/launch.json

**What is REAL, and what could not be built.** The backend exposes `POST
/webhooks/records/upload` (multipart, `file` + `collection_id` required) and `GET
/collections/status`, so intake and the wait state are genuine. **There is NO create-case
endpoint and no case entity** — `case_id` appears 3x against `collection_id` 224x — and a
collection comes into existence when its first file is accepted. So `/cases/new` does not
pretend to create anything: it names the case and takes its first evidence, and says so.
Nothing was simulated.

**The honesty split that governs intake:** TRANSFER has a real measurable percentage and
gets the only progress bar in the product (XHR, because fetch cannot report upload progress).
PROCESSING has NO honest percentage and is reported as a STATE — queued/running/completed/
failed/not-processed — never as a bar. `ProcessingWait` keeps those four truths distinct and
never calls unprocessed evidence "no results". Polling continues through a failed status
READ, because failing to read the status is not a failure of the processing.

Tests 55/55 (4 new files), production build clean, rendered and checked in-browser with zero
console errors. Styles compose existing tokens only — no new palette, logical properties
throughout so WI-UI-3's RTL mirroring still holds.

**Still blocked, backend decision:** live browser evidence needs auth/session ownership — a
long-lived API key must never reach browser storage.

### WI-17 — PHASE 2 AUTH UNBLOCKED: same-origin proxy, 2026-09-24 · CLAUDE BUILT UI

**The blocker is cleared: the browser now holds NO credential.**

**Decision — same-origin proxy, not short-lived tokens.** Session tokens need an identity
system that does not exist (architecture §6: no users/roles/membership tables). The proxy
unblocks now and is exactly where session auth attaches when identity lands.

    PROXY MODE (default, the only deployable shape)  browser sends no credential;
                                                     proxy attaches the key server-side
    DIRECT MODE (explicit opt-in, local dev only)    `apiToken` in page config

`requestContext` no longer THROWS without a token — absence now means "the proxy holds it".
That is safe because the API still enforces auth: a misconfigured proxy yields a clean 401,
never silent unauthenticated access. **This was observed, not assumed** — the first live test
returned exactly that 401 before the key was in the proxy's environment.

**Two real defects found by testing it end to end:**
- **`X-Forensic-Actor-Role: analyst` was rejected with 403 on EVERY request.** `auth_scope.go`
  authorizes `user`, `admin`, `agent-worker` — `analyst` was never valid. Fixed in the CLIENT
  (now `user`); widening the server's role list to match the UI's wording would have been a
  security change made for a copy reason.
- **`public/runtime-config.js` hardcoded `apiBaseUrl: http://localhost:8091`**, forcing a
  cross-origin call and CORS and bypassing the proxy entirely. Default is now `/api`.

Verified live: `/api/collections/status` → **200** with no client credential; the Evidence and
Processing surfaces render real data (12 items, Completed 11 · Failed 1). Tests 59/59, build
clean. `vite.config.js` proxy reads the key from the dev-server environment only — it never
enters the bundle, the page or storage.

**The proxy is NOT authentication.** It forwards with a privileged key, so anyone who reaches
it has that access; it binds to 127.0.0.1 and is a development/review convenience. Real
multi-user auth needs the identity system and attaches at this same seam.

### WI-18 — media answers NAME their source; gold set corrected to v2, 2026-09-24

**IMG-01 and DOC-02: answers that cited a source without naming it.**
`image_ocr_search` answered "Retrieved 1 cited evidence result from the selected case scope"
and `document_search` answered "across 1 document" — sentences that name no file, could
describe any result of any kind, and leave the analyst with a count wearing a citation's
clothes. The image, the matched text and the region were all in the locator and none reached
the analyst. Now: `printed-english.png contains the text "Investigation Workspace"`, and
documents are named the same way (one or two outright, first-plus-count beyond that). The
bounding box is deliberately NOT narrated — a region is something the viewer draws, not a
sentence, and it stays in the locator where the citation opens it. `claimCarryingResult` is
now shared by transcript, image and document answers.

### GOLD SET v2 — `evaluation/golden_questions_v2.json`

Detail: [`ORACLE_CORRECTIONS.md`](reports/golden-v2-20260924/ORACLE_CORRECTIONS.md).
**v1 is retained unchanged**: every measurement before today was taken against it and
overwriting it would make those numbers incomparable — the WI-6 rule.

- **CDR-13 / CDR-05** expected RAW `INCOMING`/`GPRS`/`VOLTE`. UX §9 forbids showing an
  analyst raw values and the layer names them `Incoming`/`Data session`/`VoLTE call`. The v1
  expectations passed only because raw values survive in the payload, so they rewarded
  machine vocabulary and could not detect a regression back to it.
- **DOC-01** expected only `MN1367`, which "There are no ANPR sightings involving MN1367"
  satisfies by echoing the question back while answering about ANPR. **Verified against the
  data first:** MN1367 appears in exactly ONE document passage, in
  `nexusai-multimodal-acceptance-brief.pdf`. That is now the expectation.
- **Harness:** filename expectations must appear in the ANALYST TEXT, not anywhere in the
  response blob — the defect that made AUD-01 and AUD-03 false CORRECTs. **Scoped to
  filenames only, and that scoping was itself a correction:** requiring every element
  verbatim failed CDR-16, whose answer reads `5,000` where the oracle writes `5000`.

**What the corrections revealed:** DOC-02's missing document name (fixed above) and DOC-01,
which genuinely answers about ANPR rather than documents (**not fixed; now visible**).

**THE HONEST CONSEQUENCE: Phase 1's "confident-wrong = 0" was measured partly on a flawed
oracle.** On v2, DOC-01 is a confident-wrong answer. The defect was always there; v1 could
not see it. Correcting the instrument and losing a number is the right trade — this project
has already paid once for the opposite.

### WI-19 — DOC-01: a SOUND plan over the WRONG evidence, 2026-09-24

"Search the case documents for mentions of plate MN1367" compiled a perfectly sound ANPR
plan — correctly filtered on the plate — and answered "There are no ANPR sightings involving
MN1367". **Sound arithmetic over the wrong evidence.**

**The generalisable lesson, and it is the second time this exact shape has appeared:
VERIFICATION PROVES A PLAN AGAINST ITSELF, NEVER AGAINST THE QUESTION IT ANSWERS.** DOC-04
taught it through a missing filter; DOC-01 teaches it with every filter present.

**Fix: the media guard's `SourceNative` exemption is REMOVED.** Being plan-backed no longer
buys an exemption from a misroute guard. DOC-01 WRONG -> CLARIFIED, no regressions.

**This same change was measured and REJECTED on 2026-09-23** at 1 wrong removed against 1
CORRECT lost — the "correct" one being DOC-01 itself, which passed only because its v1 gold
expectation was `MN1367`, a string its wrong answer echoed back. On the v2 oracle that false
CORRECT is gone and the identical change measures **1 removed, 0 lost**. *The fix did not
change; the evidence about it did.* A rejected candidate is worth re-measuring whenever the
instrument that rejected it has been corrected.

**A test asserted the belief this disproved** — "a verified plan must never be withheld by
this guard". Updated to the correct invariant with its reason, not deleted.

**REVERTED, and worth recording so nobody retries it:** an explicit-search-scope rule in
`hybridExplicitCapability` (an analyst who names "the case documents" has said where to
look). It passed its unit tests and measured well (7 fire, 5 already on the named template),
but DOC-01 **never reaches that function** — it routes through `planRuntimeQuery`, the
keyword ladder, which is FROZEN and deleted at end of P3. Keeping an unmeasured routing
change that does not fix the thing it was written for is exactly the speculation this
project rejects.

### WI-20 — PHASE 3 OPENS: the 18 clarifications, classified, 2026-09-24 · NOT YET DEPLOYED

**The premise Phase 3 was written on is wrong.** The Phase 1 gate note says the 18
clarifications "mostly sit in the audit's `no_family` set". Measured 2026-09-24 by crossing
the offline audit against the v2 run: **only 8 of the 18 do, and 12 of the 20 `no_family`
questions already answer CORRECTLY** through the media/text path. `no_family` is not a
failure state — it is what a question that needs no typed plan looks like.

The real distribution of the 18:

    A  generation fails DESPITE a full enum   7   CDR-05/08/11/14, ANPR-05, SUB-03, TWR-02
    B  family MISRESOLUTION                   3   DOC-01, DOC-04, VID-01 -> anpr_vehicles
    C  no family and genuinely needs one      5   CASE-01, CASE-03, IMG-02, IMG-03, IMG-04
    D  correct clarifications                 2   CDR-07 (no subject), X-01 (cross-family)
    E  gold asks for a masked field           1   SUB-03 also wants `subscriber.cnic`

**Bucket A is the lever, not coverage.** Those seven resolve a family and are issued a full
curated enum — `communications_cdr` is 14/14 curated — and still produce no verifiable plan.
Bucket B is a RESOLVER defect: all three say "plate" and get vehicle records. Neither is
fixed by adding families.

**Bucket E is the FOURTH oracle defect.** The audit flags SUB-03 UNANSWERABLE because its
gold needs `subscriber.cnic`, and CNIC is masked server-side at projection by product rule.
Check whether the gold demands an unmasked value before treating SUB-03 as coverage.

**WI-20 fixes the INSTRUMENT first, as WI-6 and WI-14 both had to.** Five withholding causes
shared one machine code (`no_verified_plan`) and three shared one sentence, so **12 of the 18
were indistinguishable in the recorded response** — "which gap causes this" was answerable
only by re-running each question and reading prose. Each cause now carries its own
`reason_code`, its own analyst-facing sentence and a `withhold_detail` of planner state
(never evidence values). `verified_only_withheld` stays as the route label; harnesses and
recorded runs key on it.

**It is also a product fix.** DOC-04 — "What does the case notes document say about plate
ABC-123?" — was told to "name the field, the grouping or the time range". No field, grouping
or time range fixes DOC-04. It now says the question matched vehicle records, which is the
thing that actually went wrong. Specific causes are tested BEFORE the general one; the SET of
withheld requests is unchanged, since every branch already withheld.

New: `withhold_reason.go`, `withhold_reason_test.go` (8 offline cases). Full suite green,
658 specs. One test's expectation narrowed from the blanket code to the specific one.
**Not deployed, not measured live** — expected 0 verdict changes, clarification TEXT changes.

### WI-21 — BUCKET A RESOLVED OFFLINE: the P3 gate's one failing criterion, 2026-09-24

**Against the P3 gate, exactly one criterion fails.** correct-or-clarified 98.4% PASS ·
confident-wrong 0 PASS · p95 5.2 s (cached) PASS · **abstention 18/62 = 29% vs the ≤20%
gate — FAIL.** Six clarifications must become answers. Nothing else is open.

`bucket_a_conformance_test.go` settles where those six are, **offline, no model calls**, by
putting each hand-written gold plan through the real catalog and the real S6 validator. Seven
live probes would have cost an hour and could not have distinguished "our defect" from "the
model's".

    CDR-05 CDR-11 CDR-14 TWR-02   gold plan VALIDATES today -> generation is the gap
    CDR-08 ANPR-05                GOLD SET DEFECT -- see below
    SUB-03                        PRIVACY BOUNDARY, working as designed

**Four of the six are a generation gap, not a coverage gap.** A correct plan is expressible
against the issued enum right now — 14/14 curated for CDR. Adding families or fields fixes
none of them. The fix is S4 narrowing, the prompt, or the model tier, **measured against a
pre-declared rule** (ANTI-MODEL-ROULETTE), never guessed.

**CDR-08 and ANPR-05 are the FIFTH oracle defect.** Both carry `gold: null` with
`inexpressible: "No distinct-count aggregate in the IR."` **COUNT_DISTINCT has existed since
WI-9.** The reconstructed one-measure plan validates against the issued enum for both. The
gold set has been scoring the product for a limitation that expired. Same class as the
DOC-01/AUD-01/AUD-03 filename checks and CDR-13's raw values — **the instrument recording a
constraint that no longer holds.**

**SUB-03 must NOT be "fixed".** `CatalogFields` excludes `PII`/`RESTRICTED` from the dynamic
enum deliberately, with its reasoning in the code. SUB-03's gold expects the literal CNIC
`00000-1000001-1`, and CNIC is masked server-side at projection by product rule. **Satisfying
this gold means leaking a CNIC.** The gold is wrong; the clarification is correct behaviour.
Whether masked PII should be dynamically projectable at all is an owner decision, not a
defect to close.

**Corrected in flight:** the probe first reported CDR-08/ANPR-05 as `GOLD_UNPARSEABLE` and
counted them as pipeline rejections. They are neither — `gold: null` is a recorded claim, not
malformed data. The tally now separates generation gap · gold defect · withheld-by-design,
because collapsing them pointed the next fix at the wrong layer.

**Next, in order:** correct the gold for CDR-08/ANPR-05 and re-adjudicate (they may already
be answered); deploy WI-20 and read `withhold_detail` for the four generation-gap questions;
then one pre-declared measurement on S4/prompt before any tier change.

### WI-22 — WI-20 DEPLOYED: the 18 classify themselves, 2026-09-24 · LIVE

Evidence: [`reports/wi20-withhold-20260924/`](reports/wi20-withhold-20260924/). Rollback
image `rollback-before-wi20-20260924`. Switches unchanged (`FORENSIC_PLAN_CACHE=true`,
`FORENSIC_VERIFIED_ONLY=true`, arbitration+fallback on, shadow+crosscheck off). Startup
restored **31 plan-cache entries**, auth enabled.

    43 CORRECT · 18 CLARIFIED · 1 MANUAL · 0 WRONG      ZERO verdict changes, as predicted

The 18, now self-classifying from the recorded response alone:

    no_verified_plan                     7   CDR-05/08/11/14, ANPR-05, SUB-03, TWR-02
    media_question_structured_template   5   DOC-01/04/05, IMG-02, VID-01
    uncomputed_quantity                  2   CASE-01, IMG-04
    single_family_negative               1   X-01
    (no code -- never reached the gate)  3   CDR-07, CASE-03, IMG-03

**FOUR of the seven fail on ONE identical reason**, and it is ours, not the model's:

    generator_dynamic_validation_rejected: "source-native filter value was not
    supplied by the analyst"        CDR-11 · CDR-14 · SUB-03 · TWR-02

The generator PRODUCES a plan; `semanticSourceNativeLiteralValues` — the CONSTRAINT_APPLIED
allowlist — then rejects its filter value as un-supplied. **This allowlist has already been
fixed twice for exactly this**: the `-2026` hyphen bug read a plate's hyphen as a minus sign,
and curated enum literals like `ACTIVE` were invisible. Its own comment says it "has to see
every shape of literal a question can carry, or it silently turns correct plans into
refusals." This is the third instance.

**Probed offline (pure function, no model, seconds) — and it splits the four:**

    "...923001110001 make?"              -> 923001110001            value IS present
    "...from August 2026?"               -> 2026 | CALL             NO date bound
    "...for PK-SUB-SYN-ALPHA?"           -> (EMPTY)                 no digit -> token skipped
    "...tower PK-LHR-SYN-001 located?"   -> 001 | PK-LHR-SYN-001    value IS present

**Confirmed for 2, DISCONFIRMED for 2 — and the disconfirmation is the more useful half.**
SUB-03 (an all-alphabetic identifier is invisible to a digit-gated token rule) and CDR-14 (a
month NAME supplies no date bound) are extraction gaps. CDR-11 and TWR-02 carry their value
in the allowlist and are rejected anyway, so for them the defect is in the COMPARISON, not
the extraction. Two causes wearing one error string. **Do not fix the extraction and assume
all four move.**

Remaining two: CDR-05/CDR-08 `rejected_shape`; ANPR-05 `generator_malformed_dynamic_proposal`
(the only malformed output in the corpus).

**Instrument limitation, stated so nobody misreads the log:** `withhold_detail` prints
`issued_fields=0` for every question, and no rejected plan is recorded. Both read
`IRShadowIssuedFields` / `IRShadowRejectedPlan`, which are populated **only under
`FORENSIC_IR_SHADOW=true`**. `issued_fields=0` therefore means "not recorded", NOT "the enum
was empty" — the offline audit shows CDR issuing 14/14 curated. Fix the detail to read a
field that exists on this path before drawing any conclusion from it.

**Next:** the allowlist comparison (CDR-11, TWR-02) is the highest-value unknown — two
questions of the six the P3 gate needs, on a defect already proven to be ours.

### WI-23 — two fixes, measured separately: one kept, one REJECTED, 2026-09-24 · LIVE

Evidence: [`fixA`](reports/fixA-literal-20260924/) · [`fixB`](reports/fixB-invented-20260924/).
Rollback `rollback-before-literalfix-20260924`. **Live state: 44 CORRECT · 17 CLARIFIED ·
1 MANUAL · 0 WRONG.**

**Characterisation first, and it OVERTURNED the previous entry.** WI-22 concluded that
CDR-11/TWR-02 were a defect "in the COMPARISON, not the extraction". **That was wrong.** The
persisted plan cache holds the actual model completions, so the plans were read directly
rather than inferred — no redeploy, no model call:

    CDR-11  cdr.originating_number EQ "923001110001"   correct
            cdr.direction          EQ "outbound"       INVENTED
    TWR-02  tower.site_code        EQ "PK-LHR-SYN-001" correct
            tower.site_location    CONTAINS "where"    INVENTED from the interrogative
            tower.district         CONTAINS "area"     INVENTED
    SUB-03  subscriber.subscriber_id EQ "PK-SUB-SYN-ALPHA"   the ONLY filter, exactly right
    CDR-14  cdr.event_time BETWEEN 2026-08-01T.. .. 2026-08-31T..   exactly right

The comparison was CORRECT. Two unrelated defects wore one error string.

**FIX A — the allowlist was refusing correct plans. KEPT. +1 CORRECT, −0.**
`literal_supply.go`. An identifier is a SHAPE, not a digit: `PK-SUB-SYN-ALPHA` holds no digit
and was skipped, so the token rule never saw it. And a DATE BOUND is legitimately absent from
the question's text — the analyst types a month NAME — so a filter on a TIMESTAMP/DATE field
is now validated by parsing the instant against the range S2 already extracted, not by string
equality. That also survives the generator rendering the same month as `2026-08-31T23:59:59Z`
on one run and `2026-09-01T00:00:00Z` on another. **This allowlist has now produced FOUR
false rejections** (the `-2026` minus-sign read, invisible curated enum literals, and these
two). Measured: CDR-14 CLARIFIED → CORRECT. SUB-03's allowlist blocker cleared — it now fails
later, on a malformed proposal, and its gold still demands an unmasked CNIC.

**FIX B — pruning invented filters. MEASURED AND REJECTED. 0 gained, 2 CONFIDENT-WRONG
created.** Switch `FORENSIC_DROP_INVENTED_FILTERS`, **stays false**; code retained with this
measurement, as `FORENSIC_IR_CROSSCHECK` is.

    CDR-11  CLARIFIED -> WRONG  "There are no CDR records in this case."  (gold 872)
    TWR-02  CLARIFIED -> WRONG  "There is 1 tower record in this case."   (asked WHERE)

**The generalisable lesson, and it is worth more than the fix would have been: an invented
filter is a SYMPTOM of a plan the model did not understand, not a blemish on a correct one.**
CDR-11's surviving filter was on `cdr.originating_number` — the WRONG column; the right one
is `cdr.msisdn` — so pruning produced a plan matching 0 rows and a **false negative**, the
worst answer class this product can give. TWR-02's surviving plan COUNTed in answer to
"where". The guard was doing more than it appeared: it was rejecting PLANS, using the
invented value as the signal. Remove the signal and the rest of a bad plan walks through.

**Rejected after measuring — do not retry:** dropping generator-invented filters to rescue
the rest of the plan.

**Cross-track, reconciled not reverted:** Codex edited `semantic_layer/cdr.yaml` and
`anpr.yaml` mid-session (its files), acting on WI-UI-7 §7 — it wrote the CDR-08/ANPR-05 gold
plans and deleted the inert `cdr.distinct_subscribers` / `anpr.distinct_plates` metrics.
`TestWI4DistinctCountMetricsAreDeclared` failed. **The test was the stale part**, not Codex's
change: those declarations carried no `expression:`, so `CatalogFields` skipped them and they
reached nothing — the capability comes from `distinct_capable` on the FIELD. Verified
empirically: `bucket_a_conformance_test.go` now reports **gold defects 0, generation gap 6 of
7** with both metrics gone. Test rewritten to assert the invariant that still carries weight.

**Gate: abstention 17/62 = 27.4% vs ≤20%.** Four more conversions needed, and all four
remaining bucket-A questions are now a pure GENERATION problem — CDR-05/CDR-08
`rejected_shape`, ANPR-05/SUB-03 `generator_malformed_dynamic_proposal`. S4 narrowing or the
prompt, measured against a pre-declared rule. **Not a coverage gap, and not fixable by
loosening a guard** — Fix B is the proof.

### WI-24 — `rejected_shape` characterised: two unrelated rules, 2026-09-24 · OFFLINE

`shape_rejection_probe_test.go`, replaying the VERBATIM cached completions. No model call.
The audit records only `rejected_shape`, which named neither the rule nor the stage; the two
questions fail in different files at different stages.

**CDR-05 — S9 REFUSES A CORRECT PLAN. The defect is ours, and it is not the generator.**

    question   "How many calls of each type are there?"
    plan       GROUP BY cdr.call_type + COUNT          <- exactly right
    frame      goal="aggregate"  group_by_hints=[]     <- "of each type" not read as grouping
    s9 shape   "scalar"
    verdict    S9_SHAPE_VIOLATION: a scalar question compiled a grouping by cdr.call_type

The rule is "a plain count must not acquire a grouping it was never asked for" — D2's
signature — and here it fires on a grouping that **was** asked for. The comment directly
above that rule already warns of exactly this: *"S9 must read the same signal the compiler
used, or it refuses the very grouping the compiler correctly derived."* It was written for
"incoming versus outgoing". The same mismatch is live for "of each type". **A verifier that
reads the question differently from the compiler rejects correct work** — the third distinct
way this codebase has produced a false refusal.

**CDR-08 — THE SCHEMA PERMITS WHAT THE VALIDATOR FORBIDS.**

    question   "How many unique phone numbers appear as callers in the CDRs?"
    frame      goal="distinct"  group_by_hints=[counterparty]      <- correct
    plan       COUNT_DISTINCT with field_id ""                     <- counts nothing
    verdict    rejected AT DECODE by the strict parser:
               "COUNT_DISTINCT requires the field whose values are counted"

`semanticSourceNativeSchema` issues `field_id` as an enum that INCLUDES `""`, because a plain
COUNT legitimately omits a field. The grammar therefore also permits `COUNT_DISTINCT` of
nothing, and the generator took it. **This inverts the architecture's keystone**: an invalid
plan is supposed to be structurally impossible to emit, not rejected afterwards. A flat
measure object cannot express "field_id is required when op is COUNT_DISTINCT"; two measure
variants in a `oneOf` can, and GBNF alternation supports it.

**Neither is a model-tier problem, and neither is coverage.** Both plans came from the 4B
model and both were right or nearly right. Escalating a model would fix neither.

**Candidate fixes, each to be measured on its own before being kept:**
(1) S9 reads the grouping signal the compiler used, rather than re-deriving it lexically;
(2) the measure schema becomes a `oneOf` of COUNT-without-field and aggregate-with-field.

**Correction made during the probe:** it first asserted every cached completion decodes, and
CDR-08 does not. *Where* a plan is rejected is part of what was being characterised, so the
probe now reports the stage — decode, structural validation, or S9 — rather than assuming.

### WI-25 — CDR-05: S9 stops refusing a grouping that WAS asked for, 2026-09-24 · LIVE

Evidence: [`reports/s9-grouping-20260924/`](reports/s9-grouping-20260924/). Rollback
`rollback-before-s9grouping-20260924`. **+1 CORRECT, −0. CDR-05 CLARIFIED → CORRECT.**

    45 CORRECT · 16 CLARIFIED · 1 MANUAL · 0 WRONG
    latency median 2.7s · p95 5.3s · MAX 8.8s   (warm cache; max now inside the 15s gate)

The scalar guard — *"a plain count must not acquire a grouping it was never asked for"*, D2's
guard — was refusing CDR-05's correct `GROUP BY cdr.call_type`. "It was never asked for" is
the entire content of the rule, and the shape classifier could not establish it: extraction
does not read "of each type" as a grouping signal, so `GroupByHints` was empty and the
question classified as scalar.

**The fix asks the CURATED LAYER, not a second lexical rule.** A grouping survives only when
the question names the grouped field by its display name or a declared synonym — "type" is a
declared synonym of "Call type". The layer is what the COMPILER resolves fields through, so
verifier and compiler now read one shared vocabulary instead of two independent guesses about
the same sentence. Adding another regex would have been the third guess.

**D2 keeps its teeth, and the tests are mostly controls.** A spurious grouping comes from
scoring the whole question against the catalogue, on a field the analyst never mentioned — by
definition not named. Refused still: a grouping nobody named, a PARTIALLY named
multi-dimension grouping, a grouping on a hashed uncurated field (no display name to have
been named by), and every rank/breakdown rule unchanged.

**Two characterisation tests were inverted, not deleted.** They asserted the DEFECT, because
that was what was being characterised; they now assert the property. `rejected_shape`'s probe
records the live-verified STAGE per question — CDR-05 `accepted`, CDR-08 `decode` — so it
doubles as the regression record for both.

**P3 gate:** correct-or-clarified 98.4% PASS · confident-wrong 0 PASS · p95 5.3 s PASS ·
**abstention 16/62 = 25.8% vs ≤20% — FAIL, 4 conversions short.**

**Next, and the only bucket-A item left with a known root cause: CDR-08.** The issued schema
lists `""` among `field_id`'s enum members so a plain COUNT may omit a field, which also
permits `COUNT_DISTINCT` of nothing — the generator took it and the strict parser refuses it
at decode. Fix is two measure variants in a `oneOf`. **It changes the issued grammar, so every
payload changes and the plan cache invalidates by design (different payload, different key)**
— budget a cold run.

### WI-26 — CDR-08: schema fixed, guard vindicated, model is the limit, 2026-09-24 · LIVE

Evidence: [`measure-variants`](reports/measure-variants-20260924/) ·
[`grammar-keystone`](reports/grammar-keystone-20260924/). Rollbacks
`rollback-before-measurevariants-20260924`, `rollback-before-grammarenum-20260924`.

    45 CORRECT · 16 CLARIFIED · 1 MANUAL · 0 WRONG    unchanged across BOTH runs

**The schema fix landed and works.** A measure is now issued as two variants —
`{op: COUNT, field_id: ""}` and `{op: any allowed, field_id: an issued id}` — so
`COUNT_DISTINCT` of nothing is unrepresentable. Verified in the SERVING grammar:
`root-0-measures-item-0-op ::= "COUNT"`, `root-0-measures-item-0-field-id ::= "\"\""`.

**And it changed what the model emits.** Captured under `FORENSIC_IR_SHADOW=true`, CDR-08 no
longer produces the invalid `COUNT_DISTINCT` with an empty field; it retreats to a plain
`COUNT` of rows. S9's distinct obligation then refuses it — *"a question asking for unique
values compiled a plain count, which counts rows and not distinct values"*. **The guard is
right.** A plain COUNT answers 8,642 where the truth is 10, and that is a confident wrong
answer, not a near miss.

**CDR-08 is therefore a GENERATION-QUALITY limit, not a schema or guard defect.** The 4B
model will not select `COUNT_DISTINCT` for "how many unique phone numbers appear as callers",
even with the aggregate enumerated in the grammar and the field marked distinct-capable.
Under ANTI-MODEL-ROULETTE a tier change needs a pre-declared measurement; do not start trying
models.

**A WRONG CONCLUSION I REACHED AND RETRACTED — the most useful thing in this entry.**
Mid-investigation I dumped the GBNF by calling `NewJSONSchemaConverter` on the in-memory Go
map, saw every `field_id` and `op` compile to the generic `string` rule, and concluded the
keystone had never been enforced. **That was false.** The converter reads
`schema["enum"].([]any)`; these schemas built `[]string`; the assertion fails only for a
LOCAL conversion. The product never converts locally — the schema is marshalled into the
request and the SERVER unmarshals it into `functions.Item` first, and `[]string` and `[]any`
marshal identically. The server has always received `[]any` and has always produced the
alternation. **The keystone held, and WI-0's 86-call measurement means what it said.**

**The lesson: a probe must reproduce the PATH, not just the function.** Calling the right
converter with the wrong input type produced a confident, plausible, entirely wrong picture
of a safety property. `grammar_keystone_test.go` now builds the grammar through the JSON
round-trip exactly as `core/http/endpoints/openai/chat.go` does, and
`TestEnumRepresentationsAreIdenticalOnTheWire` records why the shortcut is invalid.
`grammarEnum` is kept only so a future local probe gets the server's answer; its comment says
so plainly.

**Newly documented hazard:** if grammar generation fails server-side, chat.go logs
`Failed generating grammar` and **proceeds with NO grammar at all** — every constraint lost,
silently, with a 200 response. Worth a startup assertion.

**Two cache facts, both measured.** The plan cache keys on the payload, so a change to
grammar-conversion BEHAVIOUR is invisible to it — the measure-variant run had to be re-run
with `FORENSIC_PLAN_CACHE=false` to be honest. And cold latency without the cache: median
5.4 s, p95 89.5 s, max 170 s, against warm median 2.7 s / p95 5.3 s.

**P3 gate:** correct-or-clarified 98.4% PASS · confident-wrong 0 PASS · p95 (warm) 5.3 s PASS
· **abstention 16/62 = 25.8% vs ≤20% — FAIL, 4 short.** Every remaining bucket-A question is
now generation quality, with the schema, the guards and the grammar all verified correct.

### WI-27 — TIER MEASUREMENT: it was never the model. Prompt change REVERTED, 2026-09-24

Rule written BEFORE the work: [`DECISION_RULE.md`](reports/tier-decision-20260924/DECISION_RULE.md).
Evidence: [`step1`](reports/tier-decision-20260924/step1/). Rollback
`rollback-before-promptaggregates-20260924`. **Live state restored: 45 CORRECT · 16
CLARIFIED · 1 MANUAL · 0 WRONG**, confirmed on the seven affected questions.

**The tier question was answered without touching a model.** The IR system prompt explains
COUNT, SUM, MAX and MIN and **never mentions COUNT_DISTINCT** — the one aggregate CDR-08 and
ANPR-05 need. The captured shadow plan showed the model doing exactly what the prompt
describes. Escalating a tier to recover an untaught aggregate is the roulette the absolute
rule forbids.

**Two sentences were added and MEASURED. The declared threshold fired.**

    45 CORRECT · 16 CLARIFIED · 0 WRONG   ->   43 · 14 · 4 WRONG
    in-scope conversions (CDR-08, ANPR-05, CDR-11, TWR-02): ZERO

    CDR-11  CLARIFIED -> WRONG   answered 447, truth 872
    TWR-02  CLARIFIED -> WRONG   counted records for a "where" question
    SUB-03  CLARIFIED -> WRONG   "2 subscriber records involving PK-SUB-SYN-ALPHA"
    NEG-02  CORRECT   -> WRONG   "1 ANPR sighting matched" for a plate that does NOT EXIST
    ANPR-04 CORRECT   -> CLARIFIED

Reverted on the rule, not on judgement. **NEG-02 is the one that matters most**: a negative
control turned into a false positive — the exact failure class this project exists to remove.

**THE SAME LESSON, NOW TWICE, FROM OPPOSITE DIRECTIONS.** WI-23's Fix B DROPPED
generator-invented filters after the fact: 2 confident-wrong. This change told the model not
to PRODUCE them: 4 confident-wrong. CDR-11 makes the mechanism plain — with the invented
`cdr.direction = "outbound"` gone, the model emitted one clean filter on the WRONG FIELD
(`cdr.originating_number`, not `cdr.msisdn`), which passes every guard and answers 447.

**The invented filter is LOAD-BEARING AS A SIGNAL.** It marks a plan the model did not
understand; the wrong-field error underneath was always there. Suppress the symptom and the
disease reaches the analyst. **Do not attempt to remove generator-invented filters by any
means — post-hoc, by prompt, or otherwise.** They are diagnostic.

**Never measured in isolation, still separable:** the COUNT_DISTINCT sentence converted
nothing, but it ran alongside the filter sentence that did the damage. Retrying it requires
its own measurement against the same threshold.

**CORRECTION to WI-21 and WI-26, and to the Codex prompt §7.** Both said SUB-03's gold
"demands a raw CNIC". That is true of `scripts/ir_spike/gold_plans.json` (`00000-1000001-1`)
and **FALSE of the scoring oracle**: `golden_questions_v2.json` expects `*********0011` with
the note *"CNIC is masked server-side; the masked form is the correct answer."* The SCORING
gold is correct. What stands is narrower and still decisive: PII is excluded from the dynamic
enum by design (`CatalogFields`), so SUB-03 cannot be answered from a typed plan, and the
step-1 run proved the alternative — it produced a confident wrong answer instead.

**STEP 2 (a larger model tier) is NOT justified and was not started.** Zero of the four
in-scope failures were attributable to model capacity: two are an untaught aggregate, two are
a diagnostic signal that must not be removed. The DECISION_RULE explicitly bars escalating a
failure already attributed to the prompt, schema, a guard, the catalogue or the gold set.

### WI-28 — S4 NARROWING: +2 CORRECT, 0 lost, 0 WRONG, 2026-09-24 · LIVE · KEPT

Rule written first: [`S4_NARROWING_RULE.md`](reports/tier-decision-20260924/S4_NARROWING_RULE.md).
Evidence: [`s4-narrowing-20260924`](reports/s4-narrowing-20260924/). Rollback
`rollback-before-s4narrowing-20260924`. Threshold **"both convert, 0 lost → KEEP"** fired.

    45 CORRECT · 16 CLARIFIED · 0 WRONG   ->   47 · 14 · 0 WRONG
    CDR-08   CLARIFIED -> CORRECT   10 distinct caller numbers
    ANPR-05  CLARIFIED -> CORRECT   6 distinct plates
    nothing else moved · latency median 2.1s · p95 6.3s

**The change: when the frame's goal is `distinct`, the schema WITHDRAWS the row-count
variant and offers only `COUNT_DISTINCT` over distinct-capable fields.** The grammar makes
the correct plan the only expressible one.

**Why this was safe where four earlier attempts were not.** Everything the narrowing forbids,
**S9 already refused** — the schema and the verifier were disagreeing, one offering a plan the
other would always reject while the model spent 60-150 s discovering it. This is the keystone
applied to the aggregate instead of the field, not a new judgement about the question. S9's
obligation is untouched and a test asserts it still fires.

**The signal was measured before it was used:** `frame.Goal == "distinct"` fires on exactly
CDR-08 and ANPR-05 across the full 62, zero false positives on the other sixty.

**Empty-enum killer guarded:** a distinct question over a catalogue with nothing
distinct-capable falls back to the UNNARROWED schema. Narrowing to empty would compile to an
alternation with no alternatives — HTTP 500 before inference.

**Deliberately not bundled, each with its own reason and its own future threshold:** CDR-11's
FIELD selection (`cdr.originating_number` for `cdr.msisdn`; 21 questions share its
`aggregate` goal, several correct today) and TWR-02's SHAPE (a measure where a projection is
required; 16 questions share its `lookup` goal). Bundling would have made the result
unattributable, which is what made the STEP 1 regressions expensive to read.

**PRESENTATION DEFECT FOUND, NOT FIXED — worth a look before it reaches an analyst.** Both
new answers read *"The count across CDR records is 10"* / *"The count across ANPR sightings
is 6"*. The number is right and stated, but the sentence does not say WHAT was counted
distinctly, and "the count across CDR records is 10" can be read as *10 CDR records* when
there are 8,642. Same class as WI-13: presentation not matching the computation.

**P3 gate:** correct-or-clarified 98.4% PASS · confident-wrong 0 PASS · p95 6.3 s PASS ·
**abstention 14/62 = 22.6% vs ≤20% — FAIL, 2 short.** The two nearest candidates are exactly
the two S4 items deferred above.

### WI-29 — FILTER VARIANTS: a structural defect closed, 0 conversions, 2026-09-25 · KEPT

Rule written first: [`FILTER_VARIANTS_RULE.md`](reports/tier-decision-20260924/FILTER_VARIANTS_RULE.md).
Evidence: [`filter-variants-20260925`](reports/filter-variants-20260925/) plus `-warm` and
`-steady`. Rollback `rollback-before-filtervariants-20260925`.

**THE ASKED-FOR FIX WAS NOT BUILT, for two measured reasons.** CDR-11 was on record as a
FIELD-SELECTION failure. (1) Re-reading the current run showed the diagnosis had gone stale —
it now fails on `null filters cannot carry values`, a different defect entirely. (2)
`retrieveSourceNativeFields` deliberately issues EVERY curated field, and its comment records
that a fixed top-12 cut `cdr.msisdn` and made **this same question unanswerable at any model
quality** (2026-09-22). Field narrowing is a lever already tried here and abandoned.

**What was built:** the filter item is issued as two variants — a null test
(`IS_NULL`/`IS_NOT_NULL`, empty value, NO `values` key) and a valued comparison (every other
operator). `IS_NULL` carrying a value is now unrepresentable. Same class as CDR-08, same fix,
verified through the SERVING path. The null variant OMITS `values` rather than bounding it to
zero items: `maxItems: 0` compiles to an alternation with no alternatives — the empty-enum
killer wearing a different hat.

**Result: 0 WRONG · 0 CORRECT lost · 0 conversions.** CDR-11 stays CLARIFIED, and its reason
moved from `null filters cannot carry values` to `filter value was not supplied by the
analyst` — the invented-filter signal, which WI-23 and WI-27 both measured must NOT be
removed. **The structural defect was masking a semantic one**, which the rule pre-declared as
a plausible outcome.

Kept under the pre-declared structural-safety clause: an invalid plan became impossible to
emit. That clause was written in advance precisely so keeping a null result could not be a
post-hoc rationalisation.

**A THRESHOLD NEARLY FIRED ON AN INSTRUMENT ARTEFACT.** The cold run showed IPDR-05 and
ANPR-06 CORRECT -> CLARIFIED, which triggers "any CORRECT lost -> REVERT". Both failed with
`generator_timeout_or_unavailable` — a TIMEOUT, not a wrong plan. Re-measured:

    IPDR-05   s4 warm 4s (cache hit) -> 88s (first gen, new key) -> 7s steady
    ANPR-06   s4 warm 1s (cache hit) -> 138s (first gen, new key) -> 3s steady

Both answer CORRECTLY. The loss was the one-time cold cost of changed cache keys crossing the
planner timeout, not a regression. This is the standing warning applied — *latency in this
environment is not trustworthy to within an order of magnitude* — and the first comparison
was itself wrong: s4's 4s/1s were CACHE HITS being compared against first generations.

**REAL EXPOSURE, NOT CLOSED:** any schema change invalidates every cache key, and on the cold
pass a generating question can cross the planner timeout and lose an answer it would
otherwise give. That is a property of schema changes in general, not of this one. Worth
either raising the planner timeout or warming the cache after a schema change.

**P3 gate unchanged:** 47 CORRECT · 14 CLARIFIED · 1 MANUAL · 0 WRONG · abstention 22.6% vs
≤20% — **FAIL, 2 short.** CDR-11 is not reachable without removing the invented-filter
signal, which is forbidden on two independent measurements. TWR-02's shape narrowing remains
the one untried S4 item.

### WI-30 — TWR-02 shape narrowing: MEASURED DEAD, NOT BUILT, 2026-09-25

Detail: [`TWR02_SHAPE_NOT_BUILT.md`](reports/tier-decision-20260924/TWR02_SHAPE_NOT_BUILT.md).
**No code changed, nothing deployed.** State unchanged: 47 CORRECT · 14 CLARIFIED · 1 MANUAL
· 0 WRONG.

**The `lookup` goal is not the signal the narrowing needed.** Probed directly:

    CDR-04   goal=lookup shape=rows  "Show the call type breakdown"       CORRECT, needs measures
    CDR-15   goal=lookup shape=rows  "What was the longest call?"         CORRECT, needs MAX
    IPDR-03  goal=lookup shape=rows  "Break down IPDR sessions by proto"  CORRECT, needs measures
    ANPR-03  goal=lookup shape=rows  "When was LHR-2026 first/last seen"  CORRECT, needs MIN/MAX
    ACC-02   goal=lookup shape=rows  "Show the breakdown of HTTP codes"   CORRECT, needs measures
    TWR-02   goal=lookup shape=rows  "Where is tower PK-LHR-SYN-001"      the target

Withholding measures for this goal would break **six correct answers to convert one
clarification**. Same failure pattern as the prompt attempt: a signal assumed clean and used
without being measured first. Measuring it cost minutes; the prompt attempt cost four
confident-wrong and a 36-minute run.

**And TWR-02 does not fail on shape anyway.** Its reason is `filter value was not supplied by
the analyst` — the model appends `tower.site_location CONTAINS "where"` and
`tower.district CONTAINS "area"` to a correct `site_code` filter. **TWR-02 and CDR-11 are the
SAME blocked case**: both were recorded as distinct defects (shape, field selection) and both
are the invented-filter signal working. Removing it is forbidden on two independent
measurements (WI-23: 2 wrong · WI-27: 4 wrong).

**VERIFICATION GAP FOUND, AND IT CANNOT BE CLOSED YET.** `verifySourceNativePlanShape` has
cases for `rank`, `scalar` and `breakdown` and **no case for `rows`** — a question demanding
rows gets no shape check, so a scalar count can answer it silently. A `rows` rule requiring a
projection would refuse the same six correct answers, because the goal feeding it is
mis-assigned: **the frame calls an explicit breakdown a lookup.**

**The defect is in GOAL CLASSIFICATION, not in S4 or S9.** It touches the compiler's core
signal that every downstream check reads, and it is its own work item with its own
measurement.

**Both recorded so neither is retried as an "obvious" improvement.**

### PHASE 3 STATUS — one criterion short, and the remaining gap is characterised

    correct-or-clarified 98.4%   PASS      confident-wrong 0        PASS
    p95 (warm) 6.3s              PASS      abstention 22.6% vs <=20%  FAIL, 2 short

**Every remaining clarification now has a named, measured cause.** The two nearest candidates
(CDR-11, TWR-02) are blocked by a guard that must not be removed. SUB-03 is a deliberate
privacy boundary. The rest are coverage (`no_family` media/cross-family) or correct
clarifications. **There is no known safe change that closes the last two**, and four attempts
to force it produced six confident-wrong answers between them. Closing the gap needs either
the goal-classification work item or an owner decision that 22.6% is where this corpus lands.

### WI-31 — GOAL CLASSIFICATION: taxonomy gap found, vocabulary fix REVERTED, 2026-09-25

Rule written first: [`GOAL_CLASSIFICATION_RULE.md`](reports/tier-decision-20260924/GOAL_CLASSIFICATION_RULE.md).
Evidence: [`goal-classification-20260925`](reports/goal-classification-20260925/). Rollback
`rollback-before-goalclass-20260925`. **State restored: 47 CORRECT · 14 CLARIFIED · 1 MANUAL
· 0 WRONG**, confirmed on all five affected questions.

**THE HOLE IS REAL AND STILL OPEN.** `semanticFrameGoal` returns `lookup` from its
`default:` branch — it means *"we did not recognise this question"*, not *"the analyst wants
a lookup"* — and `s9QuestionShape` maps it to shape `rows`, for which
`verifySourceNativePlanShape` has NO CASE.

    unclassified (default branch):              16 of 62  (26%)
    of those, CORRECT today with NO shape check:  8

**The vocabulary fix was measured and reverted at 1 confident-wrong.** Adding "breakdown" to
the aggregate case and "longest"/"shortest" to rank reclassified four questions, all
predicted offline before the run, no surprises. Result:

    ACC-02  CORRECT -> WRONG  "Show the breakdown of HTTP status codes in the access logs"
                              answered "There are 1,000 access log entries in this case"
                              — a TOTAL where a BREAKDOWN was asked for

**THE REASON IS A TAXONOMY GAP, NOT A BAD TERM CHOICE, and this is the finding worth
keeping. There is no `breakdown` goal.** `aggregate` with no group-by hints resolves to shape
`scalar`, so classifying an explicit breakdown as `aggregate` actively ASSERTS it is a single
number. The `lookup` default is also wrong — it skips shape verification — but it is wrong in
the SAFE direction. **Being unrecognised was protecting these questions.**

A real fix introduces a breakdown goal carrying a grouping obligation. That is a taxonomy
change touching every consumer of `frame.Goal` (operation ranking, plan compilation, S9, S4),
and it is a larger work item than the vocabulary edit that was attempted.

**Separable and never measured alone:** "longest"/"shortest" → rank. CDR-15 stayed CORRECT
throughout and the regression was attributable to "breakdown" — but the declared threshold
reverts the CHANGE, not the half that looks guilty. Retrying it needs its own run.

**Do not add the S9 `rows` case** until the taxonomy is corrected: it would refuse the eight
questions that currently answer correctly under the unrecognised default.

### PHASE 3 — the abstention gate is DEFENDED, and that is now measured, not asserted

    correct-or-clarified 98.4%  PASS     confident-wrong 0          PASS
    p95 (warm) 6.3s             PASS     abstention 22.6% vs <=20%  FAIL, 2 short

**Five attempts to close the last two questions produced SEVEN confident-wrong answers and
zero conversions.** Every one was reverted on a pre-declared threshold.

    WI-23 Fix B          drop invented filters post-hoc      2 wrong
    WI-27 prompt         instruct the model not to invent    4 wrong (incl. a negative control)
    WI-29 filter split   structural, kept                    0 wrong, 0 conversions
    WI-30 TWR-02 shape   measured dead, NOT BUILT            —
    WI-31 goal vocab     reclassify unrecognised questions   1 wrong

**The two remaining conversions are blocked by design, not by absence of effort.** CDR-11 and
TWR-02 both fail on the invented-filter signal, which is diagnostic and must not be removed
(proven twice). SUB-03 is a privacy boundary. The rest are coverage or correct clarifications.
**Closing the gate requires the breakdown-taxonomy work item, or the owner accepting 22.6% as
where this corpus lands.**

## 1a. The inverted gate — ON since 2026-09-22

Detail archived with WI-0..WI-8. **The IR generator was never gated; it was DISCONNECTED** —
`resolveSemanticDynamicPlan` was reachable only from a dead function, so what WI-0 measured
at 74.3% had never run in production. Five separate blockers each silently defeated the whole
path (missing `semantic_ir_fallback` status, unset `SourceNativeCatalog`, zeroed `Limit`, a
185 s legacy schema, and unset `RecordType` which ran every plan across ALL families).

The LLM operation selector stays DISABLED on purpose: it picks one of ~104 opaque operation
IDs that nothing can re-verify, whereas a generated plan is re-checked by S6, S9 SHAPE and
CONSTRAINT_APPLIED and **discarded** on any failure.

**Audit:** `ir_fallback_outcome` records `accepted` or the exact check that discarded the
plan; `ir_arbitrated` marks an answer that replaced a clarification. Both required — "why did
the system answer this way" must be reconstructable months later.

**Cost:** with shadow AND fallback on, an unresolved question pays for TWO generations
(167/174/219 s observed). Keep shadow OFF in production. P3's gate is p95 < 15 s.

---

## 2. Measured truth

Current scorecard is in §1 (Phase 1 gate). Baseline evidence:
[`P0-baseline-report.md`](reports/nexusai-tl-audit-20260918/P0-baseline-report.md).

**Regression anchors** (SQL-verified, case `nexusai-forensic-demo`, re-confirmed after the
2026-09-21 daemon restart): CDR 8,642 · IPDR 2,500 · ANPR 750 · access log 1,000 ·
subscribers 11 · towers 5 · transactions 4 — **12,912 rows**.

**Live models:** synthesis `qwen3-4b-instruct-2507-q4km-nxb21d-dev` (**Q4_K_M**, not Q8),
embeddings `qwen3-embedding-0.6b`. CPU only, ~4 tok/s, ~15.7 GiB RAM, 6 GiB floor.

**Runtime:** forensic-records-api rebuilt on the host (7 s) — `docker compose build` is
unusable, see §6. **LocalAI rebuilt 2026-09-22** for the GBNF `maxItems` fix: binary only,
overlaid so the UI wrapper and model config are inherited. Rollback image
`nexusai/localai-forensic:rollback-before-gbnf-maxitems-20260922`, container
`nexusai-api-1-prev-gbnf` (renamed, NOT removed):

    docker stop nexusai-api-1 && docker rename nexusai-api-1 nexusai-api-1-gbnf && docker rename nexusai-api-1-prev-gbnf nexusai-api-1 && docker start nexusai-api-1

Auth verified failing closed (401 unauthenticated).

---

## 3. Defect status

**FIXED AND CONFIRMED:** **D3** HTTP 500s (0/62, baseline 3) · **D2** spurious GROUP BY ·
Fact Packets carried plumbing → WI-2. **FIXED AT THE COMPILER, STILL LIVE ON THE LADDER**
(27/62 route through `runtime_query` and bypass them): **D1** unbound literals dropped ·
**D4** cross-family misrouting.

**OPEN:** **D8** payload filters discarded unless the template is `canonical_records`
(`query.go:7728`) · **D7**
"Explain …" treated as a dictionary definition · **3 `no_family` structured gaps** (CDR-07,
CDR-10, ANPR-03) · **gold-set defect** — SUB-03's plan gold demands `subscriber.cnic` and
`subscriber.full_name`, which the sensitive-field guard withholds by design.

---

## 4. Architecture — decided, do not relitigate

Rationale: [`docs/architecture/RECONCILIATION_20260921.md`](docs/architecture/RECONCILIATION_20260921.md).

1. **Stop defining operations.** 79 templates, 104 operations and the 8,644-line ladder are
   a *per-question* abstraction. **Frozen; deleted at the end of Phase 3.**
2. **Governed Semantic Compiler**: classify → extract literals → scope → narrow →
   enum-constrained IR → hard validation → one self-correction → parameterized SQL →
   verify → **abstain rather than guess**.
3. **The IR already exists.** `SourceNativePlanV1` is richer than the SMQ that scored
   94.15% on Spider2-snow; `source_native_sql_executor.go` compiles it with scope
   predicates and lineage. Build on it.
4. **The keystone is evidenced.** JSON-Schema `enum` compiles to a GBNF alternation
   (`pkg/functions/grammars/json_schema.go:129-138`); WI-0 measured 0 hallucinated fields.
5. **LocalAI becomes a pinned, unmodified upstream image** (`v4.10.x`), internal only.
   Fork baseline v4.5.6; rebase impossible; core divergence ~330 lines across 8 files.
6. **NexusAI owns analyst identity and cases.** No case entity exists today: `case_id`
   appears 3× in the schema vs `collection_id` 224×; no users/roles/membership tables.
7. **UI contract is [`docs/ux/NEXUSAI_PRODUCT_UX.md`](docs/ux/NEXUSAI_PRODUCT_UX.md).**
   IA is CASE → EVIDENCE → QUESTION; families are filters, never navigation. The analyst
   UI is a **standalone app**, not a page in the LocalAI dashboard.

---

## 5. Hard rules

- **No new operation templates**; do not extend `query.go`'s keyword ladder.
- **No model roulette** — no evaluating, benchmarking, downloading or swapping LLMs except
  via a written decision rule surfaced to the user.
- **No embeddings on structural decisions** (family, group, measure, field role). D2 and
  the measure defect both came from exactly this. Embeddings are for *text* retrieval only.
- **No reports without a measurement.**
- **Abstention is a success.** A confident wrong answer never is.
- **Never fabricate** a result, citation, OCR text, transcript, sighting, face identity,
  location, timeline or cross-family relationship. Never turn a similarity score into an
  identity claim.
- **Never merge upstream LocalAI into this repo.** Clone to a sibling directory to compare.

---

## 6. Operational hazards

- **Auth fails closed.** `forensic-records-api` needs `FORENSIC_RECORDS_API_KEY` from
  `.env.forensic-runtime.local`; pass `--env-file` to every compose command. The container
  refuses to start otherwise. **Never "fix" a crash-loop by disabling auth** — on
  2026-09-17 an entire session served the forensic API unauthenticated on real evidence.
- **`docker compose build` is unusable**: `context: .` uploads the whole repo then runs
  `go mod download` in the container. It stalled 65 min and wedged the daemon; recovery
  needed a machine restart. Build on the host (`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go
  build -trimpath -ldflags="-s -w"`) onto `distroless/static-debian12:nonroot` — 7 seconds.
  **Debt:** add `reports/` to `.dockerignore`.
- **Postgres is on host port 5433, database `localrecall`** (not 5432/`forensic_records`).
- **An empty JSON-Schema `enum` is an unparseable grammar.** llama.cpp rejects the whole
  request — `500 "failed to parse grammar"` — before any inference. Proven controlled
  2026-09-22 (same schema: 200 populated, 500 emptied), fixed;
  `semantic_dynamic_schema_test.go` walks every schema. **Seed runtime-built enums with a
  constant.**
- **Never** `docker compose down`, `down -v`, `--remove-orphans` (it would delete
  `nexusai-api-1`, the LocalAI container), `system prune`, `volume prune`. Never
  reprocess, backfill, migrate or delete retained evidence.
- **Git:** never `reset --hard`, `clean`, `checkout --`, `restore`, `stash`, `merge`,
  `rebase`, `cherry-pick`, `pull`, `push` without being asked in the current session.
- **Build gates unrunnable**: `make` is not on PATH, so `make lint` / `make
  test-coverage-check` cannot run and the pre-commit hook fails closed.

---

## 7. Ask before

Container rebuild or redeploy · any `docker compose` action beyond `ps`/`logs` · any
database write, migration or backfill · model downloads or backend installs · any git
operation that writes · long builds · running the live golden suite · anything touching
retained evidence.

---

## 8. Where to read more

Architecture: `docs/architecture/RECONCILIATION_20260921.md` · UI (binding):
`docs/ux/NEXUSAI_PRODUCT_UX.md` · standalone-app decision:
`docs/design/nexusai-analyst-ux-v2-architecture.md` · what to implement:
`docs/work/MASTER_EXECUTION_PROMPT.md` · Codex onboarding: `.agents/CODEX.md` · baseline
defects: `reports/nexusai-tl-audit-20260918/P0-baseline-report.md` · history:
`reports/archive/continuation-20260921.md`. **Superseded where they conflict** (provenance
only): `NEXUSAI_MASTER_DIRECTIVE.md`, `NEXUSAI_NEXT_CHAT_PROMPT.md`,
`NEXUSAI_TL_DIRECTIVE_PROMPT_20260918.md`.

---

## 9. Phase 0 — complete

Six checkpoint commits 2026-09-21 (`d374f3d`..`584c135`) brought 3,481 files under version
control; not pushed. **Debt:** reconstruct the LocalAI v4.5.6 baseline in a sibling clone →
`docs/integration/FORK_DELTA_v4.5.6.md`.

---

## CODEX_UI_TRACK

- **ACTIVE_WORK_ITEM:** WI-UI-7 — UX-contract conformance COMPLETE locally 2026-09-25. Owned changes are in `apps/investigation-workspace/**`, `semantic_layer/{cdr,anpr}.yaml`, `semantic_layer/README.md`, `scripts/ir_spike/gold_plans.json`, and this section. `api/**`, database state, retained evidence, containers, and deployment were not changed.
- **Provenance:** numeric/date/identifier claims receive claim-local exact-locator markers; markers have no redundant brackets. Markers without an openable locator are suppressed. Hover/focus previews expose source file, locator, supplied timestamp, a supplied excerpt capped at 200 characters, proof role, and a text/icon `EvidenceStrengthBadge`. Three or more distinct sources produce a rail; one or two remain inline. Missing timestamps/excerpts stay explicitly unavailable.
- **Shell and identity:** the global, case, and task bands are separate. NexusAI ships with a code-native mark, horizontal lockup, `/favicon.svg`, runtime customer identity overrides, Dashboard/Cases/Activity/Settings navigation, capability-gated Admin, a persistent collapsible rail at 1024/1440, and a trapped modal drawer below 1024 with Escape and focus return. Case switching changes the URL and clears incompatible `nexusai.case.*` session state first. Unsupported tenant/user/role/classification fields are absent.
- **Transport:** all analyst requests use the same-origin `/api` proxy; browser-direct credential mode and every fixture token were removed. Absolute and protocol-relative API overrides fail closed with `CFG-PROXY`. The browser never receives an API credential.
- **Scopes and states:** exact evidence-family chips persist per session on Evidence and Investigate; an empty or unsupported scope fails loudly and is never widened. Curated semantic-layer labels are used when present; otherwise raw identifiers remain unchanged. Every route exposes loading, ready, empty, partial, error, forbidden, and unavailable fixtures. Timeline's inert disabled filters were removed. Partial Activity retains completed artifacts and names the failed scope.
- **Answer and overview:** finding hierarchy emphasizes the measured quantity rather than enlarging the whole sentence. “How this was derived” is a real collapsed disclosure. Overview uses a real case title, one primary evidence metric, and concrete data-quality sentences. Theme selection lives under account/preferences, survives reload, falls back to `prefers-color-scheme`, and remains usable when storage throws.
- **Palette and typography:** governed blue `#2563EB`, teal `#14B8A6`, navy/charcoal canvases, `--analyst-surface-0..3`, `--analyst-surface-ask`, focus, status, and evidence-strength roles replace the undocumented petrol/cyan palette. Noto Sans / Arabic / Mono remain deliberate because Geist lacks Arabic coverage and the corpus contains mixed-script evidence such as `کال 03001234567 at 1035`.
- **Measured contrast:** Daylight minimum text 4.51:1 and component/focus 4.16:1; Operations minimum text 6.35:1 and component/focus 5.20:1. `tokens.vitest.js` enforces all governed roles and thresholds.
- **Semantic correction:** CDR-08 now has `COUNT_DISTINCT(cdr.msisdn)` gold; ANPR-05 has `COUNT_DISTINCT(anpr.plate_number)` gold. The inert `cdr.distinct_subscribers` and `anpr.distinct_plates` declarations were deleted. SUB-03 was intentionally not changed because its literal-CNIC gold conflicts with server-side masking. `go test ./api/forensic_records -run TestBucketAGoldPlanConformance -count=1` passes.
- **Visual evidence:** 68 PNGs in `apps/investigation-workspace/design-review/wi-ui-7/`: the eight requested surfaces at 375/820/1024/1440 in both themes, plus drawer-open, rail-collapsed, claim-level close-up, and all-route-state fixtures. The 375px captures have no horizontal page scroll, retain NexusAI identity, and use 44px mobile targets; the 768–1279 table range pins the first column.
- **Verification:** ported 46/46; unit 67/67 across 10 files; Playwright 46 passed + 30 intentional project/owner skips, covering clarification, zero, unsupported, drawer focus trap/return, and the core loop; production build passes with 162 transformed modules.
- **Missing endpoints kept honest:** no case entity/create-case endpoint (the first accepted file creates the collection); no tenant/user/role/classification source; no cross-case attention/assignment feed; no global/team audit history; no cross-family timeline event feed; no family-scoped Documents/Images/Audio/Video query without an evidence ID; no guaranteed citation excerpt/timestamp; no authorized admin summary. The corresponding fields, counts, events, and controls are absent or explicitly unavailable rather than fabricated.
- **Not deployed/activated:** WI-UI-7 is implemented and reviewable locally only. Production activation still requires an explicit deployment decision and an owned identity/session layer in front of the same-origin proxy. Exact next action: product-owner review of `design-review/wi-ui-7/`, then authorize the deployment/auth work separately.
