# NexusAI Forensic Intelligence — Living Continuation Checkpoint

## 2026-09-18 TL directive P0 baseline — 34% correct-or-clarified, 26 confident-wrong; no behavior changed

Controlling directive: `NEXUSAI_TL_DIRECTIVE_PROMPT_20260918.md` (team-lead instruction:
English-first, LLM-planned querying, reasoned answers, premium UI, re-certify everything).
Full evidence: `reports/nexusai-tl-audit-20260918/P0-baseline-report.md`.

- 62-question English golden set with **SQL-derived oracles** (`golden_questions_v0.json`),
  repeatable harness `scripts/nexusai_live_eval.py`, every live response read by hand.
- Result: CORRECT 16, PARTIAL 9, SAFE_FAIL 5, WRONG 29 (26 confident, 3 false-negative),
  ERROR 3. Answer stated in analyst text for only 5/20 correct-or-partial factual answers.
- Live synthesis model is `qwen3-4b-instruct-2507-q4km-nxb21d-dev` (Q4), not Q8 as recorded
  on 2026-09-17.
- **Supersedes 2026-09-17 claims:** "How many CDR records" is **nondeterministic**: 8,642
  in one call and a spurious 3-row GROUP BY `manual_review_required` 16 minutes later.
  Root cause is `sourceNativeFieldByHint` adding whole-question embedding similarity when the
  group hint is empty.
- New P0 root causes (source-verified, not yet fixed):
  - `executeSourceNativeProjectionSQL` passes projection args to the COUNT query, so every
    dynamic row-listing plan returns HTTP 500 (3/3 reproducible).
  - Dynamic counts silently drop target/date constraints. A nonexistent number returns
    counts of all 12,912 case rows.
  - Cross-family misroutes (IPDR/tower/access log → CDR or ingest templates).
  - Audio search returns an unrelated segment as a match.
  - "Explain …" is classified as a general-domain definition.
- Role A: this is not a hidden queue. Warm narration takes 37 s (1,030 prompt tokens, 145
  output tokens, ~4 tok/s); in-product it took 79 s. Both were **correctly rejected** by
  `validateNarrative`, because Fact Packets contain plumbing metrics ("Template: …", "Route: …",
  "Planner Confidence") instead of result values and passages. Fact Packet v2 is the
  prerequisite for any reasoned LLM answer.
- 33 templates are `bounded_uncertified` in `/query/templates` but `CERTIFIED` in
  `/api/v1/forensics/operations`. Treat none as certified until the golden suite passes.
- Exact next action: P1 in the order in report §8. It starts with the projection-SQL argument
  fix and the empty-hint embedding group fix. Each fix needs rebuild + redeploy approval for
  live verification.

## 2026-09-17 CRITICAL: every container rebuild this session silently ran the forensic API without authentication — found, root-caused, and fixed structurally

While instrumenting the narration diagnostic above, its startup log line
(`auth_enabled=false`) revealed that the live `nexusai-forensic-records-api-1`
container was serving requests to **completely unauthenticated callers** —
verified directly: a request with no `Authorization` header at all returned
HTTP 200, not 401.

**Root cause:** `docker-compose.forensic-records.yaml` defaulted
`FORENSIC_API_AUTH_REQUIRED` to `false` and `FORENSIC_API_KEY` to empty
(`${FORENSIC_RECORDS_API_KEY:-}`) whenever those shell variables aren't
explicitly exported before `docker compose up`. The real values
(`FORENSIC_RECORDS_API_KEY=8d6b8...`) live only in the gitignored
`.env.forensic-runtime.local`, which is meant to be sourced/exported before
running compose — every `docker compose ... up -d --no-deps forensic-records-api`
run during this session's many rebuild-and-verify cycles was run from a
shell that never sourced that file, so **every single redeploy this session
silently fell back to no authentication** on a system handling forensic
evidence (CNIC masking, chain-of-custody, tenant/case scoping — all of which
depend on the auth middleware actually running).

**Fixed structurally, not just for this session:** flipped both compose
defaults to fail-**closed**: `FORENSIC_API_AUTH_REQUIRED` now defaults to
`true`. The Go binary already had a real, tested startup guard
(`validateForensicAuthConfig` in `auth_scope.go`) that calls `os.Exit(1)`
when `Required=true` and the key is empty — that guard was already correct
and already had test coverage; the compose *default* was the actual bug.
Verified live, twice: (1) redeployed with the key intentionally unset —
container now correctly **refuses to start** and crash-loops with a loud,
unmistakable `"FORENSIC_API_AUTH_REQUIRED is enabled but FORENSIC_API_KEY is
empty"` error, instead of silently serving unauthenticated traffic; (2)
redeployed with the correct key exported — unauthenticated requests now
correctly get `401`, authenticated requests still work correctly (verified
against a real query, exact CDR count 8,642 still correct).

**Operational note for this session and any future one:** `FORENSIC_RECORDS_API_KEY`
and `FORENSIC_API_AUTH_REQUIRED` must be exported in the shell (or sourced
from `.env.forensic-runtime.local`) before every `docker compose up` for
`forensic-records-api` — they do not persist between separate shell
invocations. The compose-default fix above means forgetting this now fails
loudly (container won't start) instead of silently (container runs open),
which is the real fix; but the correct values should still always be
supplied for the container to actually come up.

## 2026-09-17 Role A narration: two real bugs found and fixed; the remaining latency issue is NOT resolved — documented honestly rather than claimed fixed

Investigated why Role A synthesis (the LLM narration path) never actually
worked in practice, since it's the one production-wired LLM role in this
system. Found and fixed two genuine, permanent bugs; the underlying latency
problem that blocks a live demo remains open despite extensive diagnosis —
recorded precisely so the next pass doesn't have to re-derive any of this.

**Bug 1 — fixed, high confidence:** `narrativeJSONSchema` built `fact_refs`/
`citation_refs` inside the repeated `claim` object schema with a raw
`{"enum": factIDs}` literal instead of going through the existing
`narrativeStringChoices` helper (which already handled this correctly
everywhere else). When a deterministic-only template has no KB/document
citations (e.g. `call_type_breakdown`), `citationIDs` is empty, producing
`"enum": []` — an empty JSON Schema enum, which crashes llama.cpp's grammar
sampler outright (`rpc error: ... Failed to initialize samplers: failed to
parse grammar`, surfaced as an HTTP 500). Reproduced directly against
LocalAI to confirm before fixing. This was causing Role A to **crash
instantly, every time**, for an entire class of ordinary explain/hybrid
questions. Fixed by routing `claim`'s fact_refs/citation_refs through
`narrativeStringChoices` like everything else. This fix is unconditionally
correct and should be kept regardless of the latency finding below.

**Bug 2 — fixed, high confidence, real but insufficient on its own:**
`limitations` and `suggested_questions` used full free-text sentences
(measured up to 126 characters each) directly as JSON Schema `enum` values
with `uniqueItems: true`. An isolated test proved this exact pattern is
disproportionately expensive for llama.cpp's grammar sampler on this CPU
backend: the identical 3-array-of-claim-objects schema shape generated at
the hardware's normal ~4.3 tokens/sec when given short IDs, but the
production schema (same shape, real free-text limitations) consistently
hit the full 120s synthesis timeout. Fixed by having the model select short
IDs (`L1`, `L2`, ... / `Q1`, `Q2`, ...) constrained to the real list, then
translating IDs back to the real text server-side (`narrativeIndexedTranslate`,
new) before `validateNarrative` runs — the exact same safety guarantee (model
can only select from the server-supplied list, never invent text), just a
trivial grammar instead of one built from full sentences.

**What was also tried and measured, for the record:**
- Raising `threads` in the model YAML from 8 to 16 (all 16 host CPUs were
  available, only 8 configured): negligible difference (4.13 → 4.38 tok/s).
  This confirmed the workload is memory-bandwidth-bound, not thread-bound, on
  this CPU-only (`n_gpu_layers: 0`) deployment — more threads is not the lever.
- Reducing `max_tokens` 512 → 250: did not change the outcome (still hit the
  full 120s ceiling) once combined with Bug 2's fix being absent; kept as a
  reasonable additional safety margin regardless.
- Reducing per-claim ref limits 50 → 4 and claim-array `maxItems` 8 → 3: a
  reasonable independent improvement (smaller grammar, cheaper regardless),
  kept, but an isolated test proved this was **not** the dominant cost —
  the same reduced-bound schema with real free-text limitations still hit
  120s; only substituting short IDs for the free text (Bug 2) fixed the
  isolated case.

**Unresolved, reported honestly rather than claimed fixed:** after fixing
both bugs above, the real `/query/hybrid` "explain call type breakdown"
request *still* hit the exact 120.0–120.3 second timeout, identically
across every configuration change attempted (before and after Bug 2's fix).
This exact, unvarying 120s ceiling — never 90s, never 140s, never
completing — despite isolated tests of the same schema shapes completing
correctly at expected speed in 9–46s, is itself the most important clue:
if raw generation speed were the limiting factor, changing the schema
should have changed the completion time, and it did not, even once, across
four independent attempts. This points to something in the **real
production request path** (as opposed to a direct curl against LocalAI)
never actually starting real generation at all, or hanging on something
unrelated to the model/schema — not yet isolated. Do not assume Bug 2's fix
resolved the user-visible symptom; it did not, verified live, twice.

**Recommendation for whoever picks this up:** do not keep tuning
schema/token/thread parameters further — three rounds of that produced no
timing variation at all, which is itself evidence the bottleneck isn't
there. Instead, instrument the actual production code path directly (log
timestamps immediately before and after the `httpclient.NewWithTimeout(timeout).Do(httpReq)`
call in `synthesizeFactPacketNarrativeWithTrace`, stim_fact_packet.go) to
see whether the HTTP call to LocalAI is even dispatched promptly from
inside the real request-handling goroutine, versus queued behind something
else (case/collection resolution, KB retrieval, or another lock) — that is
the one diagnostic this session did not yet do, because it requires a
rebuild+redeploy+log-instrumented live request cycle that was not reached
before this investigation's time was better spent stopping and reporting.

**Demo guidance:** do not rely on live, synchronous Role A narration for a
demo right now — it is not reliably fast. The dynamic runtime query
pipeline (extensively verified working this session, fast, across CDR
counts, IPDR counts, ANPR counts, access-log counts, document search, image
OCR, audio transcript search including Roman-Urdu, and video timelines) is
the genuinely demo-ready story. When Role A does succeed (or once the
above is diagnosed and fixed), it now can never crash on the empty-enum
bug and never compiles an unnecessarily expensive grammar — both real,
permanent, verified improvements — but "sufficiently fast for a live demo"
is not yet true and should not be presented as such.

## 2026-09-17 Phase 3 attempted a second time — rolled back again, but the honest story is more nuanced than "the LLM is unsafe"

Re-attempted the LLM operation-chooser fallback with an added safety layer:
`resolveSemanticOperationInCandidates` now rejects a model-chosen operation
whose declared `GroupBy` doesn't overlap the question's grouping dimension
(reusing `semanticGroupingCompatible`, the same signal the deterministic
compiler already scores with — no new validator). First fix attempt broke
`TestConvergenceRetrievalBoundary` (a legitimate rank-shaped match got
rejected because the hint-extraction regex captures noisy free text like
"event frequency" as a fake grouping hint); corrected by restricting the
grounding check to only `semanticFrameGroupHints`' curated,
controlled-vocabulary hints (`curatedGroupHints`, new), not its raw
regex-captured text. Suite back to 711/711.

**Rebuilt, redeployed, and ran a deliberately broad live batch this time —
not just the one case that broke it before.** The exact original bug case
was safe. A NEW question in the batch, "Which number talked to the most
other people in this case?", resolved to `forensics.top_locations` again —
the same wrong-answer shape. **Rolled the fallback back off immediately**
(same as the first time) rather than investigate with it live.

**Investigating the rollback's root cause matters here, and it wasn't what
it looked like:** re-tested the exact same "talked to the most other
people" question with the LLM fallback *fully disabled* — it still returned
`top_locations`, with `llm_latency_ms: 0`. This is a **pre-existing,
purely deterministic-compiler bug**, the same root cause family as the
original "phone numbers" bug (fixed earlier this session via a grouping-hint
synonym), just triggered by different vocabulary ("number"/"talked"/"people"
instead of "phone"/"number") that the curated hint list didn't cover. It was
not actually caused or exposed by Phase 3's changes — the rollback was
still the correct conservative call given what was known at the time, but
the LLM fallback itself was not to blame for this specific instance.

**Fixed the actual deterministic bug:** added "talked"/"talk"/"spoke"/
"speak"/"communicated" to `semanticFrameGroupHints`' counterparty synonym
list. Verified live: the question now correctly resolves to
`frequent_contacts` (asks for a target, safe) instead of confidently
returning cell towers. Full suite 711/711; `call_type_breakdown` and CDR
count re-confirmed unaffected.

**Where Phase 3 actually stands:** the LLM operation-chooser fallback
remains **disabled**. Two live reproductions across two sessions both
resolved to be about the deterministic compiler's narrow, curated-keyword-
list approach to grounding — not about the LLM being inherently unreliable.
This suggests the productive next step isn't "try re-enabling the LLM a
third time" but **replacing the curated keyword list itself with a more
general grounding signal** (e.g. embedding similarity between the
question's likely subject and each candidate's declared Measures/GroupBy,
already computed elsewhere in this file for other purposes) that doesn't
require enumerating every synonym for "who talked to whom" by hand — that
approach will keep finding new gaps one live-tested phrasing at a time
otherwise. Both LLM-fallback attempts and both reproductions are preserved
here as evidence for whoever picks this up next; do not re-attempt
re-enabling the fallback without first addressing the general grounding
signal, or expect a third live-tested phrasing to find a third gap.

## 2026-09-17 Phase 1.5 spot-check + Phase 2 complete: remaining media/family gaps live-tested, no new bugs found

**Spot-checked the other generic-fallback templates** for the same
count-vs-listing interaction fixed for ANPR/access-log in Phase 1.5, rather
than assuming they had the same bug: "how many subscribers/towers/
transactions are there" all resolve to `subscriber_status_summary`/
`tower_status_summary`/`financial_transaction_summary` — genuine per-status
breakdowns whose row counts sum to the correct total (verified: 4+2+2+1+1+1
= 11 subscribers, matching the database), not empty/wrong listings. These
are legitimate aggregate answers, not the same bug — no fix needed. IPDR's
count question already correctly went through the dynamic SQL path (2,500,
matching the database). Phase 1.5's gate is holding correctly without
needing to be extended to these templates.

**Phase 2 — previously-untested media types, now live-verified:**
- Roman-Urdu audio segment search: "Search the roman urdu transcript for
  agoa shdgan" correctly found and returned the exact matching segment
  ("fil payni forto grafron ke sath chhe agoa shdgan ko jl rha kr dia gia
  tha") with its required review-state limitation disclosed. Confirms the
  earlier `source`/`SOURCE` relevance-filtering fix (media/RAG search depth
  audit, above) generalizes correctly to this modality too.
- Video observation timeline: correctly bounded, scoped to the exact
  evidence_id, proper limitations.
- Video ANPR grouped timeline ("what plates appear in this video?"):
  correctly bounded, proper limitations.

No new defects found in this pass — Phase 2 closes clean.

## 2026-09-17 Phase 1.5: deterministic compiler no longer confidently picks a wrong-shaped operation

Root-caused why "how many access log entries do we have?" / "how many ANPR
sightings are there in total?" still returned raw row listings even after
Phase 1's routing fix. Diagnosed with a throwaway instrumented test
(printing each candidate's score breakdown) rather than guessing:

`deterministicRegisteredMatch` (`deterministic_semantic_compiler.go`) let a
candidate win via **family match or exact-name match alone**, with zero
requirement that the operation's own intent class actually matches what the
question asked for. Concretely: `access.failed_events` won "how many access
log entries" with `IntentCompatibility=0` (its intent is "lookup", the
question's goal is "aggregate" — they don't match at all) purely because its
family score (6) cleared the configured `structural_score_minimum: 0` —
a threshold so permissive it was gating on nothing. `anpr.sightings` won
"how many ANPR sightings" the same way via a different branch: its
`ExactMetadata` name-match score (12) alone bypasses every other check
in the three-way OR, regardless of intent alignment.

**Fixed** by adding a hard gate to `deterministicRegisteredMatch`: when
`frame.Goal` has a defined intent mapping in `semanticGoalMatchesIntent`
(aggregate/rank/distinct/summarize/compare/timeline/search/extract/
source_rows/existence — i.e. not "lookup"/"none", which have no such
mapping and are correctly exempt), the winning candidate's
`IntentCompatibility` must be nonzero, or the registered match is rejected
outright regardless of family/exact-name score — deferring to the dynamic
SQL count path instead. One test regressed on the first pass
(`TestConvergenceRetrievalBoundary`: "summarize repeated communications
between counterparties" expects `cdr.frequent_contacts`, a "rank"-intent
op) — traced it to `semanticGoalMatchesIntent`'s "summarize" case missing
the same rank↔aggregate tolerance its own "rank" case already documents;
added `intent == "rank"` there too (symmetric, not a weakening) and the
suite passed clean.

A second, separate interaction was found live even after this: "how many
ANPR sightings" still returned a listing because the **legacy lexical
ladder** (`chooseTemplate`, restructured in Phase 1) matched the bare word
"sightings" and returned a template *before the semantic compiler ever ran*
— that ladder has no concept of aggregate-vs-listing intent at all, so
Phase 1.5's gate never got a chance. Fixed narrowly: added a
`looksLikeCountQuestion` guard (checks for "how many"/"count of"/"total
number of"/"number of") to the one confirmed-buggy generic-fallback case
(`anpr_sightings`'s bare sighting/plate/camera match), deferring
count-shaped questions to the semantic compiler instead. Not applied
speculatively to the other generic-fallback listing templates
(subscriber/imei_imsi_usage/entity_activity/tower_activity/top_locations) —
only this one was live-confirmed broken; the others are plausible future
instances of the same interaction but unverified.

**Verified, live, against real data, all in one pass:**
- "How many access log entries do we have?" → **1000** (exact match to
  direct DB count).
- "How many ANPR sightings are there in total?" → **750** (exact match).
- "How many CDR records do we have in this case?" → still 8,642 (unaffected).
- `call_type_breakdown`, plate lookup, and "show me ANPR sightings" (the
  legitimate listing request) all re-confirmed unaffected — regression
  checked, not assumed.
- Full suite 711/711 both before finding the regression and after fixing it.

**What Phase 1.5 does not cover, named rather than assumed:** the same
family+exact-name-without-intent-check pattern could plausibly affect other
registered operations this session didn't specifically construct a "how
many X" question for. The gate is now general (any future aggregate/rank/
etc. question benefits from it automatically), but the legacy-ladder
short-circuit interaction was only closed for the one instance found live.

## 2026-09-17 Phase 1: structural fix to the keyword routing ladder (query.go `chooseTemplate`)

Restructured the ~280-line literal keyword switch into two ordered passes
instead of patching individual collisions as they're found:

- `choosePreciseTemplate(q, target, dateFrom)` — every multi-word or
  otherwise distinctive phrase match, in original relative order, unchanged.
- `chooseGenericFallbackTemplate(q, target)` — every bare single/generic
  word catch (schema/headers/status/subscriber/tower/plate/camera/entity/
  network/imei/policy/document/etc.) demoted here, in the same relative
  order they had among each other before, so generic-vs-generic collisions
  resolve identically to before — only generic-vs-specific precedence
  changed.
- `chooseTemplate` now calls precise first; only if nothing specific matched
  does it fall through to the generic pass.

This means a generic word can never again silently outrank a specific
phrase meant for a different template purely because of switch position —
the actual mechanism behind three bugs found and patched individually
earlier this session (document-vs-plate, count-vs-listing, face-vs-schema).

**Verified:** full suite still 711/711 with zero test changes required (the
split preserved every existing precise-vs-precise and generic-vs-generic
ordering exactly). Live-retested all previously-fixed cases after rebuild/
redeploy: document-vs-plate, face-vs-schema, CDR count, call-type breakdown,
and the phone-numbers safety fix all still correct.

**Important, honest limit of this fix — do not overclaim it:** re-testing
also confirmed "how many access log entries do we have?" and "how many ANPR
sightings are there in total?" **still** return a 20-row raw listing instead
of a count. Traced this: neither phrase matches anything in `chooseTemplate`
at all (verified `chooseTemplate` correctly returns `""` for both now), so
they are NOT an instance of the keyword-ladder bug this phase fixed. They
reach `access_failed_events`/`anpr_sightings` via the **separate**
deterministic semantic compiler's own embedding-based registered-match
scoring (`deterministic_semantic_compiler.go`) — the same underlying
mechanism that produced the earlier "top 5 phone numbers" → `top_locations`
wrong-answer bug, just a different pair of operations this time. That
specific instance was fixed via a targeted grouping-hint synonym; the
underlying scoring weakness that lets a confidently-wrong registered match
beat the correct dynamic-count path was not fixed generally. **This is
Phase 1's natural next-door problem, not solved by it**, and is the honest
reason "how many X" still doesn't work uniformly across every family yet.

## 2026-09-17 Media/RAG search depth audit — two real defects found and fixed with live media evidence (documents, images, audio)

Per the explicit priority to also cover media data types, tested the actual
media evidence already present in `nexusai-multimodal-product-acceptance`
(24 images, 10 audio files, 3 videos, 2 documents/PDF — 309 image-OCR
observations, 115 document-text passages, 10 audio observations, 19 face
observations, 307 ANPR observations already derived and stored) with real
natural-language questions, matching this project's own embedded acceptance
fixture (`case_notes_records_demo.txt`, which states its own expected
question and expected grounded answer).

1. **Document-search misrouting when an identifier is also present, fixed.**
   *"Search the case documents for mentions of plate MN1367"* routed to
   `anpr_sightings` (a structured plate lookup), not `document_search` —
   despite explicitly saying "documents". Root cause: the legacy literal
   keyword ladder in `query.go` only recognized rigid exact phrases like
   "search documents"/"which document mentions", so "search **the case**
   documents for mentions of" fell through and was caught later by the
   generic "plate" keyword instead. Fixed by adding a looser, word-level
   combination check (`document(s)/pdf/case notes` + `search/find/mention/
   passage/cite/...`) positioned at the same priority as the existing rigid
   phrases. Verified live: now correctly returns `document_search` with the
   exact cited passage, matching the fixture's own expected answer, even
   with "plate MN1367" in the same sentence.
2. **A much deeper defect: image OCR / audio transcript / document search
   silently return irrelevant results for ordinary phrasing, found and
   fixed.** Even with routing fixed, *"Find OCR text mentioning Investigation
   Workspace"* returned a single unrelated OCR fragment (`"J"`, from an
   unrelated image) as if it were a match. Root cause in
   `governedDerivedTextEvidence` (`derived_text_query.go`), the single shared
   retrieval function behind document/image-OCR/audio-transcript search:
   whenever the analyst's phrasing didn't hit one of the rigid regexes for
   `exact_term`/`time_range` extraction, the mode fell back to
   `normalizeTranscriptMode`'s default, which forced `score = 1`
   **unconditionally for every candidate — no lexical relevance filtering was
   ever applied for plain natural-language search.** The one real
   term-filtering branch in the code (`default: if len(terms) > 0 && score ==
   0 { continue }`) was **unreachable** — no code path could ever produce a
   mode value that landed there. This affected all three derived-text search
   templates equally (it is the same shared function), not just images.
   Distinguished this blind fallback from the legitimate, deliberate
   `forensictext`-classified `"SOURCE"` mode (uppercase; used for explicit
   multi-document comparison across a supplied source set, which correctly
   wants an unfiltered exhaustive browse) — only the blind lowercase
   `"source"` fallback now gets relevance filtering; the deliberate
   uppercase `"SOURCE"` browse-all path is untouched. Verified: full test
   suite 711/711 (the fix initially broke one comparison test by conflating
   these two modes; corrected and re-verified). Live: the OCR question now
   correctly returns "Investigation Workspace" from `printed-english.png`
   with `result_state: COMPLETE_RESULTS`; the original document-search
   passage-citation test re-confirmed unaffected.

**Follow-up same pass:** live-tested ANPR ("Where was plate MN1367 seen?" —
worked correctly, 1 exact result) and face candidates ("Show face candidates
detected in this image") — the face question **misrouted to `schema_profile`**
because of the word "detected", which the legacy keyword ladder also treats
as an unconditional trigger for schema/header inspection
(`{"schema","headers","columns","fields","adapter","detected"}`), checked
earlier in the same switch than the face-candidate keywords. This is the
**same systemic pattern** as the document/ANPR misrouting fixed above, not a
new class of bug: a generic single word, meant for one template, silently
steals traffic from a more specific but later-checked template. Fixed by
removing "detected" from that specific trigger list (the other five words —
schema/headers/columns/fields/adapter — are unambiguous enough to keep).
Verified live: face query now correctly returns `face_candidate_observations`
with a real result; schema_profile's own existing phrasing ("show detected
headers and schema") still routes correctly since it also contains
"headers"/"schema". Full suite still 711/711.

**Still not tested this pass, explicitly deferred rather than assumed fine:**
video timeline/ANPR-grouped-timeline and audio Roman-Urdu segment search —
real derived data exists (2 video observations, 7 Roman-Urdu audio segments)
but were not live-tested. **More importantly:** three independent instances
of the same root cause were found and patched individually this session
(document-vs-plate, count-vs-listing, face-vs-schema) — this is very likely
a **systemic weakness in `query.go`'s ~500-line literal keyword ladder**,
where single generic words checked early can out-rank more specific
multi-word matches checked later, for ANY pair of templates, not just the
three found. Patching instances one at a time as they're discovered is not
a substitute for a structural fix (e.g. checking multi-word/specific phrases
before any single generic word, across the whole ladder) or a full audit of
every generic single-word trigger in it. Named as its own follow-up phase
rather than continuing to whack-a-mole individual collisions.

## 2026-09-17 Dynamic-query fix hardened across families; a real 500 found and fixed; one separate routing gap identified and left open

Immediately after the dynamic SQL executor above landed, tested it across
other families (not just CDR) as the next most valuable check, per the
standing priority on making non-templated questions actually work:

- **IPDR: found and fixed a real HTTP 500.** *"How many IPDR sessions do we
  have in this case?"* returned `500 {"error":"source-native group field is
  unavailable, repeated, or restricted: fld_..."}` — a raw error surfaced to
  the analyst, violating this project's own "never a raw error" invariant.
  Root cause: the field catalog was built **twice** from two independent
  row-sample queries — once at compile time (to decide the plan) and again
  at execution time inside `sourceNativeRecords` — and the two samples could
  disagree on a field's type/groupability, causing a validly-compiled plan
  to fail re-validation against a different catalog. Fixed by carrying the
  exact compile-time catalog forward on the request
  (`hybridQueryRequest.SourceNativeCatalog`, new field, `json:"-"` so it can
  never be supplied by a client) and using it at execution instead of
  re-sampling. `sourceNativeRecords` now only falls back to a fresh sample
  when no compile-time catalog was carried (e.g. a stored/replayed follow-up
  plan). Verified: 3/3 repeated live runs now return HTTP 200 with the exact
  correct count (2,500 IPDR sessions, matching the database directly);
  `go test ./api/forensic_records/...` still 0 failures.
- **CDR/breakdown/safety regression re-checked after this fix**: "how many
  CDR records" still 8,642, `call_type_breakdown` still 11 rows, the earlier
  phone-numbers-vs-cell-towers safety fix still holds (still clarifies,
  never guesses).
- **Separate, NOT fixed this pass — a different subsystem:** "How many
  access log entries do we have?" and "How many ANPR sightings are there in
  total?" never reached the dynamic compiler at all — they matched an
  older, literal keyword-phrase route (`access_failed_events`,
  `anpr_sightings`) that returns a **list of 20 raw rows**, not a count.
  Same underlying pattern also affects a plain-language group-by count
  ("how many records for each record type?"), which resolved to the generic
  case-wide `forensics.canonical_records` browse capability (accurate but
  unaggregated — not wrong, just not what was asked) rather than a grouped
  count. Root cause is upstream of tonight's compiler: the legacy literal
  keyword matcher (`planRuntimeQuery` in `query.go`) and/or the earlier
  capability-resolution stage claim these questions before the semantic
  compiler / dynamic SQL path ever gets a chance to run. This is a real,
  separate, and likely broad gap (it plausibly affects many "how many X"
  phrasings across families that happen to collide with an existing literal
  keyword pattern) — flagged for a dedicated investigation rather than
  patched hastily at the end of this one, since fixing routing precedence
  between the legacy matcher and the semantic compiler is a different, wider
  surface than the execution-layer bugs fixed above.

## 2026-09-17 Dynamic/ad-hoc query execution rebuilt as real SQL — the actual root cause of "questions not in predefined templates don't work"

Following the query-understanding investigation below, traced why the
dynamic/ad-hoc query path (`forensics.source-native-plan/v1`, this system's
mechanism for answering questions that don't match any of the ~66 registered
operations) never actually worked on real data:

1. `semanticFrameGoal` in `semantic_frame.go` didn't recognize "many" (as in
   "how **many** CDR records") as an aggregate/count goal — only literal
   "count/sum/total/minimum/maximum" — so "how many X" questions were
   misclassified as `lookup` and never even attempted a dynamic plan. Fixed:
   added "many" to the aggregate-goal keyword set.
2. With that fixed, the dynamic path was correctly attempted but failed with
   `field_catalog_unavailable`. Root cause in `source_native_algebra.go`:
   `sourceNativeScopeRows` hard-errors when the authorized scope has more
   than `sourceNativeInputRowLimit` (1000) rows — and **the actual
   computation itself** (`executeSourceNativePlan`) ran entirely **in memory
   over that same capped sample**, not as a real database aggregate. Your
   `nexusai-forensic-demo` case has 8,642 CDR rows alone, so this mechanism
   was structurally incapable of answering any dynamic question correctly at
   real forensic scale — it would either fail closed (safe) or, had the cap
   simply been raised, have silently computed wrong/truncated aggregates
   (unsafe). This is almost certainly the actual root of "LLM/dynamic
   queries for anything not in a predefined template" not working.

**Fixed with a genuine SQL execution path**, added in the new
`source_native_sql_executor.go`, reusing the existing, unchanged, already-
validated `SourceNativePlanV1` contract and `validateSourceNativePlan` gate
(no second router/validator, per the standing constraint):
- Field-shape/type discovery still uses a bounded sample (renamed
  `sourceNativeCatalogSampleRows`) — appropriate, since it only needs to
  learn what fields exist and their type, never used to compute a result.
- The actual filter/group/aggregate/having/time-bucket/sort/limit
  computation now compiles the plan into one parameterized SQL statement
  (`executeSourceNativePlanSQL`) executed by Postgres over the **full**
  authorized scope — accurate regardless of collection size. Every field
  name and literal value is bound as a parameter; no request- or model-
  derived string is ever concatenated into SQL text. Per-row citations are
  captured via a bounded (50-row) `array_agg`/`jsonb` lineage per group,
  while the aggregate/count itself is always computed over every row.
- Wired into the sole live call site, `sourceNativeRecords`
  (`source_native_algebra.go`), and into the two catalog-discovery call
  sites that previously hard-failed on large collections
  (`applyDeterministicSemanticCompiler` in `deterministic_semantic_compiler.go`,
  `resolveSemanticDynamicPlan` in `semantic_dynamic_plan.go`).
- The pre-existing in-memory `executeSourceNativePlan` is untouched and still
  used by its existing direct callers/tests — this is a new, additive
  execution path, not a rewrite of the validated contract.

**Verified, not just built:**
- `go test ./api/forensic_records/...`: 0 failures, full suite, both before
  and after (including the pre-existing `source_native_algebra_ginkgo_test.go`
  and `source_native_sql_api_ginkgo_test.go` suites that exercise the
  contract this reuses).
- Live, against the real 8,642-row CDR collection: *"How many CDR records do
  we have in this case?"* now returns the exact correct count (8,642,
  independently verified against `SELECT count(*)` in Postgres) with full
  source-row citation lineage, `clarify=false`, executed in under 300ms.
- Regression-checked the two previously-fixed cases: `call_type_breakdown`
  still works (11 rows), and "list the top 5 phone numbers by number of
  calls made" still safely asks for a target rather than confidently
  guessing — the SQL rebuild did not reopen the earlier wrong-answer defect.

**Newly observed, not yet investigated (separate subsystem):** "How many
records do we have for each record type?" (a group-by variant) routed through
`canonical_records` but returned raw field-role/confidence schema-detection
output instead of a per-record-type count — this did not go through the
SourceNativePlanV1 path fixed above; it appears to hit the older, separate
`AD_HOC_GROUP`/`canonical_group.go` mechanism this file's history describes
as a distinct, earlier work item. Not fixed this pass — flagged for a
dedicated follow-up rather than rushed at the end of this one, since it is a
different code path requiring its own investigation.

## 2026-09-17 Query-understanding investigation: a real, live-reproduced wrong-answer defect in the deterministic compiler — attempted LLM-fallback fix rolled back

Following the TL-8 narration fix below, live-tested the actual query-understanding/
routing path against the running `nexusai-forensic-records-api-1` (real
`nexusai-forensic-demo` case data, not fixtures) with plain natural-language
questions never seen in any corpus. Findings, in order of discovery:

1. **The live-configured model is `qwen_qwen3-4b-instruct-2507` (Q8_0, 4B)** —
   confirmed via `docker inspect` env vars. The other four registered models
   (`qwen3-4b-instruct-2507-q4km-nxb21d-dev`, two `qwen3-8b-q4km...-selection-dev`
   variants, `phi4-mini-instruct-nxb21d-semantic`) are loaded in LocalAI but not
   referenced by `FORENSIC_SYNTHESIS_MODEL` anywhere in the live compose config.
   Model selection is not the blocker.

2. **The LLM is never consulted for query understanding/planning in the live
   path**, only for post-hoc narration (the TL-8 path). Traced this to
   `resolveSemanticOperationInCandidates` in `open_ended_semantic_planner.go` —
   a fully built, schema-validated, strictly-decoded model-based operation
   chooser with its own test coverage — having **zero non-test callers**.
   `hierarchical_semantic_planner_ginkgo_test.go` explicitly asserts this via
   `Fail("the deterministic compiler must not call the ... selector")`,
   confirming this was a **deliberate retirement**, not an oversight — it
   matches this file's own repeated `Q4_FULL_PLAN_ROLE=RETIRED` / Decision8
   entries (`MODEL_CLARIFICATION_WEAKNESS`, `MODEL_MULTILINGUAL_INTENT_WEAKNESS`).

3. Wired a **bounded** version of the retired fallback into
   `resolveOpenEndedSemanticPlanner` (`hierarchical_semantic_planner.go`): only
   triggered when the deterministic compiler itself returns `AMBIGUOUS_INTENT`
   or `UNSUPPORTED_REQUEST_CLASS` (never overriding a confident deterministic
   match), reusing the exact same schema validation
   (`decodeSemanticOperationProposal`/`applySemanticOperation`) that re-derives
   every fact server-side and rejects unissued operations/foreign scope by
   construction. Full `go test ./api/forensic_records/...` passed (0 failures)
   with this change.

4. **Rebuilding and redeploying this to the live container to verify it end to
   end reproduced a real, confidently-wrong answer**: the question *"List the
   top 5 phone numbers by number of calls made"* resolved to
   `forensics.top_locations` (a cell-tower/GPS report) and was presented as a
   successful, non-clarifying answer (`clarify=false`, 5 rows of cell sites and
   coordinates — nothing about phone numbers or call counts).

5. **Root-cause isolation after rollback proved this specific wrong answer is
   NOT caused by the re-enabled fallback** — with the fallback fully disabled
   again and redeployed, the exact same question still resolves to
   `top_locations` via the purely deterministic path (`llm_latency_ms:0`,
   `route:["records_sql"]`, **planner confidence: 1** — i.e. the deterministic,
   supposedly "0 wrong confident executions" compiler is itself confidently
   wrong here). This is a pre-existing defect in
   `compileDeterministicSemanticRequest`'s embedding/ranking scoring
   (`deterministic_semantic_compiler.go`), already live before any change made
   this session, and it directly contradicts the WP2 "0 wrong confident
   executions" claim recorded above in this file. Not yet root-caused past this
   point (which embedding signal or ranking term is responsible) — flagged
   here rather than guessed at.

**Current live state:** the LLM-fallback re-enablement is **rolled back and
disabled** (`hierarchical_semantic_planner.go` returns the deterministic
result unchanged; the guard test in `hierarchical_semantic_planner_ginkgo_test.go`
is restored to asserting zero model calls). The forensic-records-api container
has been rebuilt and redeployed at each step of this investigation; it is
currently running the rolled-back, fallback-disabled build. `api/forensic_records/Dockerfile`
was also fixed in passing — it only copied `pkg/httpclient`, but the working
tree's current source needs `pkg/forensicpolicy`, `pkg/forensicrequest`, and
`pkg/forensictext` too, so **the container that was running for the prior 45
hours predated a large slice of the uncommitted working tree** and had never
actually run most of it; this investigation's rebuild was the first real live
test of that pile of source. The Dockerfile now copies all of `pkg/`.

**Root-caused and fixed (source-verified AND live-verified):** the
"top 5 phone numbers by number of calls made" → `forensics.top_locations`
defect (finding 5) traced to `semanticFrameGroupHints` in `semantic_frame.go`:
it recognizes "contact/contacts/caller/callee/counterparty" as the
`counterparty` grouping dimension, but not "phone number(s)"/"msisdn" — so
this question's `GroupByHints` came back empty, the structural
grouping-compatibility signal that should have favored `frequent_contacts`
never fired, and the two CDR-family "rank" operations (`frequent_contacts`
vs `top_locations`) were left to be decided by raw lexical overlap alone,
which `top_locations` happened to win. Added "phone"+"number" (both stemmed
tokens present) and "msisdn" as counterparty-grouping synonyms. Verified:
`go test ./api/forensic_records/...` still 0 failures; rebuilt and redeployed
to the live container; the exact failing question now correctly routes to
`frequent_contacts` and asks for a target number (safe clarification) instead
of confidently returning cell-tower data. Re-checked `call_type_breakdown`
(the working case) for regression — still correct, 11 rows, no clarification.

**Still open, deliberately not attempted this session:** there is no
registered operation for "rank phone numbers/subscribers case-wide by call
volume without a specific target" — `frequent_contacts` only ranks a GIVEN
number's counterparties. The fix above makes the system fail safely closed
(ask for a target) for this class of question instead of guessing wrong, but
a genuinely new "top talkers" operation would still need to be designed,
implemented (SQL + template catalog + tests), and registered as separate,
larger scoped work if case-wide top-N-by-caller becomes a priority. The
LLM-based fallback re-enablement remains disabled/rolled back pending a
confidence/margin or grounding gate, as recorded above — this session
deliberately fixed the deterministic layer first since it was the more
fundamental and higher-severity defect.

## 2026-09-17 TL-8 live LLM-narration wiring fix (source-verified, not deployed)

Direct code inspection of the actual live analyst-facing answer pipeline
(`api/forensic_records/query.go` + `stim_fact_packet.go`, and the Ask/Chat
rendering paths that consume it) found that Role A synthesis was already
correctly implemented at the backend synthesis layer — `synthesizeFactPacketNarrative`
is called unconditionally whenever `shouldRunBoundedSynthesis` selects a
hybrid/interpretive question, is gated by the existing `validateNarrative`
grounding check, and any failure/timeout path only ever produces a
`llm_fallback_reason` string (never a raw error, never a crash) — but the
validated result was never actually rendered to the analyst:

- `core/services/agents/forensic_direct.go: formatEnterpriseForensicSummary`
  (the function that renders every "Ask NexusAI" chat answer) read only the
  deterministic `enterprise["summary"]` and never looked at
  `answer["llm_summary"]`/`answer["llm_fallback_reason"]` at all, even though
  a `"**Model Interpretation**" -> "### Plain-language interpretation"` label
  already existed unused in `polishForensicSummary`.
- `core/http/react-ui/src/pages/RecordsIntelligence.jsx` computed
  `forensicResult.answer.llm_summary` from the typed `model_interpretation`
  field but the page's `primaryBrief` (the headline analyst-facing text) never
  read it, always showing the deterministic `enterprise.summary` instead.

Both were fixed as pure connection/read-path changes (no new validator, no
new timeout, no new router): each now prefers the validated Role A narrative
when `llm_summary` is present with no `llm_fallback_reason`, and falls back
to the existing deterministic Fact Packet text otherwise. A new focused test,
`TestForensicSummaryAttemptsRoleAThenFallsBackToDeterministicFactPacket` in
`core/services/agents/forensic_direct_test.go`, proves attempt-then-fallback
end to end: a validated case surfaces the LLM prose, a
timed-out/rejected case surfaces the deterministic Fact Packet text (and never
leaks the unvalidated attempt), and a case with no attempt at all still
returns a governed message rather than nothing. `go test ./api/forensic_records/...`
and `go test ./core/services/agents/...` both pass (real runs this session,
91.0s and 55.6s respectively); this does not reproduce or supersede any
previously reported count from an earlier session.

Role A is not certified as unconditionally reliable and is not the default
without a safety check; it IS live and attempted on every synthesis request,
with automatic fallback to the deterministic answer on failure or timeout.
LLM narration is not disabled.

No other item from this session's requested full-family/TL-1..11 audit was
independently re-verified: this repository's history is a single git commit
with a very large uncommitted working tree, and the extensive prior-session
narrative in `NEXUSAI_MASTER_DIRECTIVE.md` (RAM-admission gates, r3 live
model-adjudication numbers, WP0-WP7/R0-R17/APF/NX-MMR/NX-B2.1 closures) was
not re-derived or contradicted here — it was simply out of scope for what
could be verified through source-code inspection and offline `go test` in
this session, since it requires an actually running LocalAI/model runtime
this session did not start or observe. Treat those numbers as this session's
unverified inherited claims, not as findings of this pass.

## 2026-09-17 NX-B2.1D premium analyst UI activation prepared

The current NX-B2.1D analyst UI/UX source is built and sealed for a UI-only
activation. The live runtime is not changed yet. Wrapper
`scripts/activate_nexusai_nxb21d_premium_ui_20260917.ps1` has SHA-256
`598a67410a2ced4984dbbf3d001258ac1f0fa55f6910975601448b78bf0fbd05`;
PowerShell parser validation, the Linux gateway binary check, gateway tests,
Vite production build, both Compose overlays, and the full live
`-ValidateOnly` preflight pass. Deployment manifest SHA-256 is
`e307629dd69bb2cf345d4035b8f3e6662ad9b06e8892a656c7f8885eda341c8c`.

The candidate is layered on exact live API image
`sha256:91620e4c932764c46931dd4eecb47f9f72408e2387da3189f8fcd4ab4d7506be`
and can recreate only Compose service `api`. Forensic API, worker, PostgreSQL,
and NATS are identity/image/restart protected. The retained tuple is
`69|69|82|0|22512|876|69`, Activity is `354`, active jobs are `0`, the resident
model is only `qwen3-embedding-0.6b`, and Q8 is unloaded. No database migration,
retained-data change, backend-source activation, model/profile change, image
tag, build, or container recreation occurred during preparation.

Live execution retains the unchanged 6 GiB deployment floor and requires three
consecutive qualifying samples. Optional governed recovery temporarily unloads
only the embedding runtime, drops only clean Docker-WSL file cache when
writeback is zero, and restores the embedding runtime afterward. The exact
pre-activation image is tagged for automatic rollback before recreation. Any
post-recreation verification failure restores that image. Current read-only
RAM was 4.814 GiB, so the operator must close high-memory applications before
running the prepared command. WP3/model-role closure remains deferred until
the UI activation result is returned; this preparation does not alter its r3
evidence.

## 2026-09-16 NX-B2.1D functional baseline closure

NX-B2.1D is **FUNCTIONAL_BASELINE_COMPLETE**, not product-certified. WP0,
WP1, WP2, WP4, WP5, WP6, and WP7 are closed. WP3 is
**R3_AUTHORIZED_PREPARED_NOT_EXECUTED**. The original r1
resource admission passed but its command aborted before inference because
PowerShell split the Ginkgo focus flag. The authorized r2 command then failed
the unchanged five-sample RAM gate before inference. Neither attempt produced
model-quality evidence, so no model verdict is claimed.

The final read-only WP3 operator preflight also passes. All five required
containers are running; LocalAI, the worker, Postgres, and NATS report healthy,
none is OOM-killed, and the target model is registered but not loaded (only the
embedding model is resident). The installed Q8_0 artifact is exactly
`4,280,405,216` bytes with SHA-256
`260b5b5b6ad73e44df81a43ea1f5c11c37007b6bac18eb3cd2016e8667c19662`,
and its profile SHA-256 is
`ef036e0a02cc5e8331dc4dff41c0e0cb68e6e0bda71d92b92f47f673472e666f`.
The frozen characterization runner still matches SHA-256
`ec0387052e0de7041799f869341000469211ccaaeda53dbf6bbcbbba53b2470b`.
Both no-inference characterization self-tests and the focused offline
model-role/corpus contract pass. No live model verdict exists yet.

The original nested `powershell.exe -Command "..."` handoff proved unsafe to
copy from rendered chat: the parent shell expanded `$` variables and escaped
underscores were pasted literally. It failed during parsing before RAM sampling
or inference, so it did not consume the attempt. The replacement operator
wrapper used for the r1 attempt was
`scripts/nxb21-english-functional-qualification/run_model_role_development_adjudication.ps1`
at SHA-256
`7afd9f88535cfb70bf748fa3154143b983522754dd9d694a59a946f232e66044`.
Its parser check and full `-ValidateOnly` preflight pass. It pins all frozen
host-file hashes, container state, model registration, model byte count and
in-container artifact/profile hashes; refuses existing output artifacts;
admits only five stable samples at or above 8.627 GiB; and runs only the
opt-in model-role adjudication. Optional RAM recovery gracefully closes an
explicit application allowlist, temporarily unloads the resident embedding
model, and uses the existing guarded Docker-WSL clean-cache reclaim. The live
unload/restore probe passed (LocalAI fell from about 1.21 GiB to 189 MiB), and
the clean-cache guard passed with zero writeback; the embedding model was
restored. The wrapper restores it in `finally` after the operator run as well.

The operator run admitted five consecutive samples of `8.819`, `8.820`,
`8.817`, `8.818`, and `8.825` GiB against the unchanged 8.627 GiB floor. The
immutable RAM observation artifact is
`reports/nxb21/model-role-ram-admission-20260916.ndjson` at SHA-256
`e97d6082d6a46c66f168c31e9a7f2ceca8ad1eca14af2dfb19dd5e3ed0d6a374`.
Immediately afterward, unquoted `-ginkgo.focus` was passed by Windows
PowerShell as the invalid `-ginkgo` flag. `go test` exited before selecting or
executing the live spec. The target Q8 model stayed unloaded, no evidence
directory or adjudication receipt was created, and the embedding model was
restored. The wrapper now passes each Go flag as one literal `name=value`
argument; the exact corrected focused command passes offline in 0.320 seconds,
and the final `-ValidateOnly` preflight passes. This is a runner invocation
defect with **NO_INFERENCE**, not a resource or model-quality failure.
The sealed incident receipt is
`reports/nxb21/model-role-development-attempt-1-no-inference-20260916.json`
at SHA-256
`df7dd663aaf164e41ba7298ffadaa6bc66712e8f3dd9bb1c7f5609d451b2d338`.

On 2026-09-17, the product owner authorized exactly one r2 Q8 execution and
authorized adding only an `-AttemptId` parameter to the existing wrapper. With
`-AttemptId r2`, the three outputs are isolated as
`model-role-ram-admission-r2-20260916.ndjson`,
`model-role-development-adjudication-r2-20260916.json`, and
`model-role-development-evidence-r2-20260916`. No admission, sampling, RAM
floor, five-sample stability, application closure, embedding unload/restore,
cache reclaim, or Go invocation logic changed. The current wrapper SHA-256 is
`487e07b3c43104488d92cf7e207d508fd59079b9ec35a5820915da7dd1f431a6`.
Its parser check passes and its full `-ValidateOnly -AttemptId r2` preflight
passes with the target Q8 model registered/unloaded and the embedding model
loaded. All three r2 outputs were absent at preparation. The r1 RAM and incident
receipt retain their prior exact byte counts and SHA-256 values.

The authorized r2 run produced 24 RAM observations. Its maximum was 8.666 GiB;
samples 9-12 supplied four consecutive qualifying values (`8.661`, `8.663`,
`8.665`, `8.666` GiB), but sample 13 fell to 8.553 GiB. The unchanged contract
requires five consecutive samples at or above 8.627 GiB, so admission failed
and no Go test or model request ran. The immutable r2 RAM artifact is
`reports/nxb21/model-role-ram-admission-r2-20260916.ndjson` at SHA-256
`73213cbe45979eaf0489f41b0d888fadc2a6f35e7fc71bd42fb15a059862c0fc`.
After the operator stopped OneDrive sync and Phone Link, a second r2 launch was
correctly refused by the existing-artifact guard; it added no sample and made no
model call. Post-state verification shows the Q8 target unloaded, the embedding
model restored, all five required containers running, and no OOM kill. The
sealed incident receipt is
`reports/nxb21/model-role-development-attempt-r2-resource-admission-failed-20260917.json`
at SHA-256
`e497c0177832cf2660acd5a20132cb0c133ef1a0e1f4257f9c67609a258db433`.
The product owner subsequently authorized one Q8 r3 execution. The
`-AttemptId r3` path is collision-free; parser validation and the full read-only
validate-only preflight pass, and all three r3 outputs are absent. The wrapper
and r1/r2 artifact hashes remain unchanged. r3 is authorized and prepared but
has not executed.

The post-WP5 full `api/forensic_records` regression passes in 74.349 seconds.
The fresh Vite production build passes. Focused ESLint over every changed
frontend source/spec file reports zero errors; its 34 warnings are existing
configuration/source warnings and no new warning was introduced. The complete
current analyst suite passes 31/31 in 1.7 minutes. The embedded broad matrix
passes all 50 combinations (light/dark × 390/820/1024/1440/1600 ×
home/ask/data/activity/history) with no horizontal overflow, no captured
console/page errors, and invariant navigation geometry between themes. The IA
is unchanged: no route, page, drawer, or button location moved.

WP7 is closed by the executed receipt
`reports/nxb21/nxb21d-five-vertical-flow-acceptance-20260916.md`: CDR with
deterministic citation, auditable follow-up, and Activity/History; document/RAG
with exact retrieval, passage, locator, source open, and follow-up; ANPR with
source evidence and ownership limitation; generic source-native planning
through the public API against a disposable forced-RLS database; and audio
transcript retrieval with source time and grounded answer all pass. The
source-native receipt records cleanup and retained-state parity (retained tuple
`69|69|82|22512|876|69`, Activity `354`, active jobs `0`).

`go build ./...` remains red only at declared repository-wide Windows/local
boundaries: Windows-excluded `go-piper` and `go-gl`, a local acceptance fixture
missing its embedded contract, an unreadable private-activation directory, and
the sandboxed module-cache stat write. The forensic package itself compiles and
tests cleanly. No production backend fix is indicated by those errors.

Forward plan: execute the single authorized Q8 r3 attempt after the operator's
additional host-memory preparation, then adjudicate any generated receipt
against the frozen Role A/B/C criteria. Do not weaken the 8.627 GiB/five-sample
gate or infer model quality from RAM admission. Deterministic fallback remains
the production path and WP0-WP2 remain closed.
Multilingual breadth, GPU qualification, fine-tuning, wider provider/format
fixtures, deployment, and retained-runtime certification remain deferred to
the next roadmap phase. No new blocking defect was discovered by the broad UI
matrix or five-flow walk.

## 2026-09-16 NX-B2.1D stabilization, document depth, and planner-contract closure

The in-flight retrieval edits are classified **CORRECT_INCOMPLETE** and have
now been completed. `SemanticFrame.retrieval_mode` is bound into the real
document execution path with explicit `EXACT_TERM`, `EXACT_PHRASE`,
`FULL_TEXT`, `SEMANTIC`, `HYBRID`, `METADATA_FILTERED`, and `SOURCE_SCOPED`
modes. Exact and phrase retrieval do not call the knowledge-base semantic
path. Narrative/section/passage requests use bounded KB plus derived-text
evidence, retain extractor-supplied page/paragraph/section locators, and never
promote a scanned PDF to searchable text. Source-scoped document comparison is
bounded, fair across selected sources, and clarifies when the source set is
missing.

The focused document retrieval contract passes 12/12 specifications; the
native TXT/PDF/DOCX extraction pipeline passes 6/6 unit tests. The full
`api/forensic_records` package passes in 59.122 seconds. The 11 previously open
Ginkgo planner-contract failures were all **STALE_EXPECTATION** after the
accepted deterministic semantic compiler replaced the model selector on the
production path: five registered-operation expectations, four malicious or
nonissued selector-response expectations, the legacy dynamic selector entry,
and the model timeout entry. Registered and ambiguous production-path tests
now assert zero retired-selector calls. Dynamic proposal and timeout coverage
remain exercised directly at their bounded legacy/model helper boundary.
Two overlapping standard convergence tests were aligned to the same contract.
No production safety rule or confidence threshold was relaxed.

`go vet ./api/forensic_records/...` passes. The repository-wide `go build
./...` remains non-portable on this Windows checkout because existing
third-party `go-piper`/`go-gl` build constraints exclude Windows, a local
acceptance fixture lacks its embedded contract artifact, and one private
activation artifact directory is unreadable. These are distinct from the
forensic package, which compiles and tests successfully. No deployment,
database, retained-evidence, model, container, or Git-state mutation occurred.

Current state: `WP0=CLOSED`, `WP1=CLOSED`, `WP2=CLOSED`,
`WP4=SOURCE_COMPLETE_FOCUSED_GREEN`, `PLANNER_FAILURES=0`,
`NX_B21_D=OPEN`. Exact next action: bring document/RAG answer composition to
structured-result parity, then complete the existing-IA UI polish and E2E
acceptance.

## 2026-09-16 NX-B2.1D WP2 — installed embedding assistance closed

WP2 is complete at the bounded development-ledger boundary. The production
semantic compiler now calls the already-installed `qwen3-embedding-0.6b`
through LocalAI `POST /v1/embeddings`. Operation and source-field vectors are
ranking signals only: registered-operation authorization, issued-field rules,
type compatibility, scope enforcement, and execution confidence remain
deterministic. Operation descriptors are operation-specific, normalized, and
capped at 1,536 runes; the 66-vector catalog is keyed by model plus descriptor
hash, populated once in an independently bounded background task, shared by
concurrent requests, and evicted on failure. Foreground requests retain a hard
two-second ceiling and explicitly fall back to lexical/frame ranking.

The frozen 137-query development ledger improved from 130/137 to 131/137
top-1, correcting `qv-083`, with zero regressions. Top-3 remained 132/137 and
top-5 remained 134/137. The accepted operation-embedding weight is 4.5 in
`api/forensic_records/contracts/semantic-compiler-thresholds-v1.json`.
Installed-runtime availability was 133/137; four bounded timeouts used the
explicit lexical fallback. Confidence remained 122/137 (coverage
0.8905109489051095), with 122 correct, zero wrong, and precision 1.0. The
confidence margin excludes embedding score, so an embedding-only lead cannot
authorize execution.

The reusable receipt is
`reports/nxb21/embedding-assisted-semantic-evaluation-v1.json`, SHA-256
`dcb218a3b6d3007cadde8ff42d4542371bfd47b72d6d6b16d9402718ca49478e`.
The consolidated offline WP2 bundle passed in 14.889 seconds, covering the
frozen BM25 baseline, semantic frame ranking, zero-wrong confidence
calibration, the independent 20/20 dynamic corpus, descriptor caching,
ranking-only policy, field type safety, anti-escape rules, and timeout
fallback. The opt-in installed-runtime evaluation passed in 149.020 seconds.

No model was downloaded, loaded, unloaded, or reconfigured; no holdout was
created or consumed; and no deployment, database, retained evidence,
migration, container, volume, Git staging, commit, push, or PR action occurred.
`WP0=CLOSED`, `WP1=CLOSED`, `WP2=CLOSED`, `NX_B21_D=OPEN`. Exact next action:
run the single bounded current-4B synthesis-role adjudication over frozen,
validated Fact Packets; do not reopen selector replacement or consume a new
holdout.

## 2026-09-15 NX-B2.1D Phi semantic qualification — completed FAIL; stop before candidate 2

The authorized standalone qualification completed under run
`run-20260915T111523979Z-9fb7248b`. Pre-run RAM was 8.851 GiB; the unchanged
8.627 GiB admission gate passed with five stable samples at 8.723–8.728 GiB.
Docker, all five required services, artifact/profile identities, LocalAI
profile registration, prepared-binary reuse, source regression, 5/5 retrieval,
96.35% (132/137) variant recall, two model loads, unload/reload admission, and
runtime integrity all passed. The complete frozen suite ran exactly five
registered, two dynamic, and three terminal cases with zero synthesis cases.

The controlling adjudication is `PHI_SEMANTIC_QUALIFICATION_FAIL`:
registered 0/5, dynamic 0/2, and terminal 3/3. All authority and scope safety
counters remained zero: authority violations, unissued operation IDs,
invented fields, invented values, SQL emission, and scope violations. Two
registered requests incorrectly chose `DYNAMIC_TYPED_PLAN` and their follow-up
typed plans reached the 512-token limit with malformed repetitive output;
three registered requests incorrectly chose clarification. One dynamic request
incorrectly chose clarification and the other selected the typed-plan path but
timed out. Semantic p50 was 13,884 ms, p95/max were 180,019 ms, exceeding the
30,000 ms p95 and 180,000 ms individual limits. No infrastructure failure or
runtime mutation invalidates this result.

The report is
`reports/nxb21/phi4-mini-semantic-qualification-20260915.json`, 23,050 bytes,
SHA-256
`88926b347ff594442207ef190eb3848de7c3cd9707bdce9eb89d8eb72a121c2a`.
The run-local adjudication has the same hash. The runtime receipt SHA-256 is
`e128e99a29a4581a2accbaa0a8e5473a746fdc65be2271f65cf27b5e70bcd8f5`;
development-results SHA-256 is
`21742849025bb5427b621696b5dd9cbcc8d091f6d3ea7dc0b39fe8e1571eb0d5`.
`CURRENT_PHI_SEMANTIC_ROLE=FAIL`,
`CURRENT_4B_SYNTHESIS_ROLE=UNADJUDICATED`, and `NX_B21_D=OPEN` are binding.

Exact next action: STOP and return this evidence. Do not tune or rerun Phi and
do not download candidate 2 without explicit approval. No second model was
downloaded, no formal proof was created, and no deployment, database, retained
evidence, synthesis-role, or Git operation occurred.

## 2026-09-15 NX-B2.1D Phi semantic qualification — RAM admitted; LocalAI profile 404 corrected and verified

Standalone run `run-20260915T105954772Z-6dbb77ea` passed the unchanged
8.627 GiB admission gate with five stable samples at 8.705–8.720 GiB. Its
first governed model-load request then received HTTP 404, before any semantic
quality case ran. The request was dispatched, but the model remained unloaded;
cleanup reported `MODEL_UNLOAD=ALREADY_UNLOADED`, runtime integrity PASS, and
the run ended `QUALIFICATION_DRIVER_ERROR_AFTER_INFERENCE`. No candidate
quality result was consumed.

The NexusAI API log identified the exact cause: LocalAI rejected profile lines
16–17 because `options` contained YAML maps where its `[]string` contract was
required. The two entries are now exact strings, `use_jinja:true` and
`parallel:1`. The corrected 472-byte source and Docker profile SHA-256 is
`c4a409b6d456cf86731c3f204f193871139260e9f36cc6a15c93a765ff2a1a74`;
the known faulty Docker profile was replaced only after its previous SHA-256
`88dd8cf698e26b5e4531a263277e25c547186993aae9ba59d8a05ba2e1731fec`
matched. The 2,491,874,688-byte GGUF remains unchanged at SHA-256
`01999f17c39cc3074afae5e9c539bc82d45f2dd7faa3917c66cbef76fce8c0c2`.

LocalAI's configuration reload endpoint accepted the corrected file. Its
configuration API returns the expected name, `llama-cpp` backend, artifact,
and both required string options; `/v1/models` now registers
`phi4-mini-instruct-nxb21d-semantic`. This validation did not load a model or
run inference. The standalone wrapper now performs the same registration
check, with one configuration-reload fallback, before invoking the governed
runner. Its full `-VerifyOnly` path passed Docker engine, all five services,
artifact identity, profile identity, and profile registration with
`LIVE_INFERENCE=false`.

The prepared binary remains exactly 48,183,808 bytes with SHA-256
`c2b1b21e007253db12658066e0320e62ce884c54922fcc5ee744b025b4e6ce63`.
The exact reuse validation passed again against source-manifest SHA-256
`2d7583354e80ab5ded41fdb51b25a9f78fcb6c7269701e66dc041c57c1bc2113`,
5/5 retrieval, and 96.35% (132/137) variant recall; runtime integrity passed
and `LIVE_INFERENCE=false`. Drift exclusions are limited to the orchestration
runner and its profile-identity freeze, neither of which is compiled into the
verified Go test binary. Current runner SHA-256 is
`09a05bbdeb57d8f08e328e2dcb2a97c743f9c3244af560dcbf6e0f2ada136e5f`;
standalone wrapper SHA-256 is
`aed5ccc67e811341389618f74a59b741f40b14494fb7fd29e43ecfa168ce94bc`.

Exact next action: close Codex, keep Docker Desktop running, open a new
PowerShell window, and run:

`powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\sheik\Workspace\Office\Projects\NexusAI\scripts\nxb21-english-functional-qualification\run_phi4_mini_semantic_qualification_standalone.ps1"`

## 2026-09-15 NX-B2.1D Phi semantic qualification — artifact verified; standalone admission handoff required

The corrected standalone wrapper then started with 8.933 GiB and passed Docker,
service, artifact, profile, source-regression, 5/5 retrieval, and 96.35% recall
checks. Repeating the already-passed Go regression/build immediately before
admission left host file cache resident, and the unchanged gate plateaued at
8.389–8.422 GiB. Run
`run-20260915T105152240Z-dd155bb5` therefore ended
`RESOURCE_ADMISSION_FAILURE_NO_INFERENCE` with runtime integrity PASS.

That no-inference run froze source-manifest SHA-256
`2d7583354e80ab5ded41fdb51b25a9f78fcb6c7269701e66dc041c57c1bc2113`
and the already-regression-passed 48,183,808-byte binary SHA-256
`c2b1b21e007253db12658066e0320e62ce884c54922fcc5ee744b025b4e6ce63`.
The standalone retry now uses a strict prepared-binary path: it verifies branch,
HEAD, every compiled-source manifest entry, binary size/hash, Docker baseline,
and model identities while allowing drift only in the orchestration runner and
its profile-identity freeze.
Its complete validation-only execution passed without rebuilding and without
inference. The 8.627 GiB admission gate is unchanged. Current runner SHA-256 is
`09a05bbdeb57d8f08e328e2dcb2a97c743f9c3244af560dcbf6e0f2ada136e5f`;
standalone wrapper SHA-256 is
`aed5ccc67e811341389618f74a59b741f40b14494fb7fd29e43ecfa168ce94bc`.

The corrected wrapper SHA-256 is
`aed5ccc67e811341389618f74a59b741f40b14494fb7fd29e43ecfa168ce94bc`.
Its complete `-VerifyOnly` path passes Docker engine, five-service health,
artifact/profile identities, profile registration, and cleanup with
`LIVE_INFERENCE=false`.

The first standalone launch exposed a Windows PowerShell 5.1 wrapper defect,
not a missing-container condition: `ConvertFrom-Json` preserved Docker's
top-level inspect array as one pipeline object. The wrapper now splats the five
container names and explicitly enumerates the decoded array; a native Windows
PowerShell check returns all five running containers. Before the retry, the
on-demand `qwen3-embedding-0.6b` backend (1,165,360 KiB RSS) was cleanly
unloaded, and disposable OneDrive, Windows shell companion, and PowerToys
processes were stopped. Free RAM increased to 6.533 GiB while Codex remained
open; measured Codex-family working set was 2.407 GiB. The standalone wrapper
repeats this bounded cleanup, never targets Codex, Chrome, VS Code, Explorer,
Docker, or `vmmemWSL`, prints pre-run available RAM, and retains the unchanged
8.627 GiB admission guard. Direct optional-property access was also replaced
with strict-mode-safe inspection for the records API container, which has no
health-check object.

The explicitly approved `microsoft/Phi-4-mini-instruct` semantic-only
qualification is prepared. The pinned Bartowski Q4_K_M artifact from snapshot
`faffc28d86d0c0781b4ec92d30e400a6d350a53b` was downloaded once and verified
at exactly 2,491,874,688 bytes with SHA-256
`01999f17c39cc3074afae5e9c539bc82d45f2dd7faa3917c66cbef76fce8c0c2`.
It and the isolated profile
`phi4-mini-instruct-nxb21d-semantic.yaml` (SHA-256
`c4a409b6d456cf86731c3f204f193871139260e9f36cc6a15c93a765ff2a1a74`)
were installed under new names in the Docker model volume. The existing Qwen
profile, synthesis role, and service configuration were not changed.

Docker Desktop engine 29.6.2 and the five required NexusAI services were
recovered in place after the Windows restart. Source regression, the frozen
5/5 retrieval precheck, 96.35% (132/137) variant-ledger recall, server-side
malformed/unissued proposal rejection, exact model/profile identities, and
runtime integrity passed. The bounded suite is exactly five registered, two
dynamic, and three terminal cases; semantic-only mode suppresses all synthesis
cases.

The Codex-hosted live attempt
`local-acceptance-models/nxb21-phi4-mini-instruct-semantic/run-20260915T100807120Z-f8849fd1`
stopped at the unchanged 8.627 GiB RAM gate. Available RAM plateaued near
4.8 GiB after governed clean-cache recovery. `LIVE_INFERENCE=false`,
`MODEL_LOAD=NOT_ATTEMPTED`, `MODEL_UNLOAD=NOT_REQUIRED`, and
`RUNTIME_INTEGRITY=PASS`; therefore no candidate result was consumed.
The evidence report is
`reports/nxb21/phi4-mini-semantic-qualification-20260915.json` (SHA-256
`a6c63c34c98f117e5e1f0d89e8f7b7e1b2291cc0161ab005caea84d252820a8c`).
No second model, synthesis adjudication, formal proof, deployment, retained
evidence mutation, database migration, or Git operation occurred. NX-B2.1D
remains OPEN and the current 4B synthesis role remains UNADJUDICATED.

Exact next action: close Codex, keep Docker Desktop running, open a new
PowerShell window, and run:

`powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\sheik\Workspace\Office\Projects\NexusAI\scripts\nxb21-english-functional-qualification\run_phi4_mini_semantic_qualification_standalone.ps1"`

## 2026-09-15 NX-B2.1D semantic-model replacement research — recommendation frozen; no acquisition

The bounded three-candidate research pass is complete. No model bytes were
downloaded, no model was installed or loaded, no inference ran, and no runtime
profile, service, database, or retained state changed. The current runtime's
5/5 retrieval, 96.35% TOP5 variant recall, passed RAM/runtime gates, and 0/5
registered semantic result remain controlling. Additional RAM cleanup is not a
remedy for the active failure because the last run passed admission and failed
semantic selection.

`microsoft/Phi-4-mini-instruct` is ranked first for a semantic-only candidate
qualification. The exact proposed artifact is Bartowski Q4_K_M snapshot
`faffc28d86d0c0781b4ec92d30e400a6d350a53b`, 2,491,874,688 bytes, SHA-256
`01999f17c39cc3074afae5e9c539bc82d45f2dd7faa3917c66cbef76fce8c0c2`.
Its native JSON tool format, non-thinking behavior, smaller CPU/RAM envelope,
and MIT license make it the best match for the one-issued-enum selector.
Google's official Gemma 3 4B IT QAT Q4_0 GGUF is second; Qwen3-4B-Thinking-2507
Q4_K_M is third because mandatory long thinking is a poor latency fit despite
strong generic tool benchmarks. All compatibility and latency conclusions are
research estimates pending an approved, single-variable qualification.

The proposed non-formal candidate suite has 13 cases per model: five existing
registered, two existing dynamic, three existing terminal, and three new blind
bounded diagnostics, with identical frozen retrieval candidates and server
oracles recorded before inference. Passing requires 5/5 registered, 2/2
dynamic, 3/3 terminal, 3/3 independent, 100% schema/authority safety, zero
invented operations/fields/values, semantic p95 <=30 seconds, and runtime
integrity. Consumed V1/V2 proof cases are excluded and no tuning loop is
allowed.

Detailed research is in
`reports/nxb21/nxb21d-semantic-model-replacement-research-20260915.md` with a
machine-readable companion JSON. `CURRENT_4B_SEMANTIC_ROLE=REPLACE_REQUIRED`,
`CURRENT_4B_SYNTHESIS_ROLE=UNADJUDICATED`, and
`RECOMMENDED_ROLE_CHANGE=SEMANTIC_ONLY` remain binding. NX-B2.1D remains OPEN.

Exact next action: `Wait for explicit approval before downloading or installing
the selected candidate.`

## 2026-09-15 NX-B2.1D Work Item 4 — current-4B semantic replacement required; run consumed

Standalone run `run-20260915T064053142Z-77a39c6d` passed the unchanged
8.627 GiB RAM admission gate with five stable samples at 8.658–8.661 GiB.
Source regression, the 5/5 registered-operation retrieval precheck, and the
96.35% (132/137) variant-ledger TOP5 recall passed. The exact run identities
remain source-manifest SHA-256
`d5e454a5230650b98bcb7b3317c7d7e7dd632f037b09d67288bbdfe31e2717e2`,
binary SHA-256
`aea31f373d6562549dbdc9a6aafe6f43a75e002db44c88a9bd32f9428bfcb076`,
and runner SHA-256
`6259f63fbb600220a221d90614ea1d4bf8ed98edb0a5dd26294396180ad39bb4`.

Five registered semantic cases completed before the driver abort. Every
expected operation was present in the issued TOP5 and every result preserved
authority, but the current 4B model chose `DYNAMIC_TYPED_PLAN` for all five
instead of `cdr.frequent_contacts`, `anpr.camera_activity`, `document.search`,
`audio.transcript_search`, and `generic.filter_records`. The valid registered
result is therefore 0/5, with semantic latency p50 22,827 ms, p95/max 31,373
ms. Under the frozen Work Item 4 threshold, this independently decides
`CURRENT_4B_SEMANTIC_ROLE=REPLACE_REQUIRED` and
`MODEL_REPLACEMENT_REQUIRED=true`; no prompt-tuning loop or rerun is allowed.

The later groups did not run because the harness emitted literal
`${script:scratchName}`/`${script:scratchPassword}` text in the disposable
PostgreSQL DSN. The child exited 1 after ten captured child model requests, so
the preserved final state remains `DEVELOPMENT_DRIVER_ERROR_AFTER_INFERENCE`.
Dynamic, terminal, and synthesis results are absent, and
`CURRENT_4B_SYNTHESIS_ROLE=UNADJUDICATED`; the harness defect does not erase the
five completed semantic observations. The DSN interpolation is corrected, and
the runner now loads partial result files and records runner/child request
counts on every exit. PowerShell parse and state-accounting tests pass. No
second live run was performed.

The reconciled report is
`reports/nxb21/product-convergence-development-20260915.json`. Its run-local
amendment is
`local-acceptance-models/nxb21-product-convergence-development/run-20260915T064053142Z-77a39c6d/development-adjudication-amendment.json`;
partial-result SHA-256 is
`61df5744dd9b0377f7b104d6b8aa674751371d9a47031083d5112ff4a8ca5ff8` and
stdout SHA-256 is
`0aaf4076c9bb52645ac0979681de6024d9dbc7dc61ec3fafa90df8aaa564a8f9`.
Model unload and runtime integrity both passed; retained runtime state was
unchanged. No V3, formal proof, download, deployment, retained-data mutation,
profile/envelope change, or Git stage/commit/push/PR occurred. NX-B2.1D remains
OPEN.

Exact next action: `STOP. Prepare model-replacement candidate research only.
Do not download anything without explicit approval.`

## 2026-09-15 NX-B2.1D Work Item 4 — harness rebuilt; RAM-blocked before inference

The reusable development adjudication harness now rebuilds
`development.test.exe` from the current working tree on every run, emits a
deterministic `nexusai.development-source-manifest/v1` containing branch, HEAD,
dirty-status digest, Go/toolchain identity, and SHA-256/size for repository
files contributing to the test binary, then records the binary and runner
digests. The runner distinguishes preflight, resource admission, completed
functional pass/failure, driver error after inference, and runtime-integrity
failure; records the native child PID/exit code and dispatch state; and never
labels a post-dispatch failure as no-inference. Its report includes the required
registered, dynamic, terminal, synthesis, safety, latency, model-role, and exact
next-action fields. Runner state tests cover native exit 0/7 and the
no-inference invariant.

The development suite now contains five registered analytical cases
(`cdr.frequent_contacts`, `anpr.camera_activity`, `document.search`,
`audio.transcript_search`, and `generic.filter_records`), two source-native
dynamic typed-plan cases, three terminal request classes, and three existing
Fact Packet narrative cases. The required dynamic question is exactly
`Which department has the highest average invoice total?`; it must select
`DYNAMIC_TYPED_PLAN`, use only the issued `department` and `invoice_total`
field IDs, produce GROUP + AVG + DESC + LIMIT 1, and match the independent
Sales/400 fixture oracle. The second case requires issued device/invoice fields,
GROUP + SUM + HAVING > 600 + DESC + LIMIT 1. Both execute against a random
NOSUPERUSER/NOBYPASSRLS disposable PostgreSQL database removed by the runner.

The final source preparation passed `go test ./api/forensic_records -count=1`,
`go test ./core/services/agents -count=1`, `go vet` for both packages,
PowerShell runner-state tests, and the retrieval precheck. The five development
operations remain 5/5 in TOP5; the preserved variant-ledger recall is 96.35%
(132/137). A fresh binary was built with SHA-256
`aea31f373d6562549dbdc9a6aafe6f43a75e002db44c88a9bd32f9428bfcb076`.
The source-gated run receipt is under
`local-acceptance-models/nxb21-product-convergence-development/run-20260915T061151171Z-a33b8d59/`;
its source manifest SHA-256 is
`537ca2659f616cb36d4554c1eb30659ec31acc7d3bc2586ef36bcc484f51cbbb`.

Docker Desktop engine 29.6.2 was running, but an earlier host interruption had
left the five existing NexusAI containers stopped or dependency-looping. The
same containers were started in dependency order without rebuild/recreate;
their IDs and images were preserved. API, records API, worker, PostgreSQL, and
NATS were running; health was healthy where defined. The current retained
baseline is `69|69|82|22512|876|69`, Activity 354, active jobs 0, and the
unrelated `qwen3-embedding-0.6b` loaded model was preserved. Before/after
container, image, restart-count, loaded-model, retained tuple, Activity, job,
and UI hash checks passed.

The governed attempt stopped at the pre-admission experimental safety check:
available RAM was 1.015 GiB, below the unchanged 1.5 GiB emergency reserve and
well below the unchanged 8.627 GiB admission floor. Commit reserve was
6.228 GiB, paging was quiet, WSL swap use was zero, and no container was OOM
killed. No current-4B load, adjudication request, child process, disposable
database, formal proof, deployment, retained evidence mutation, model/profile/
envelope change, or git stage/commit/push/PR occurred. Final state is
`PREFLIGHT_FAILURE_NO_INFERENCE`; runtime integrity is PASS; both current-4B
roles and replacement need remain UNADJUDICATED/UNDETERMINED. NX-B2.1D remains
OPEN.

The first operator-standalone retry on run
`run-20260915T062026625Z-1cd439ea` again passed the complete source
regression, 5/5 retrieval precheck, 96.35% variant-ledger recall, and fresh
binary rebuild. Safe cache recovery preserved dirty pages and observed zero
writeback. The unchanged 8.627 GiB gate was not admitted: its best sample was
8.324 GiB, so it ended
`RESOURCE_ADMISSION_FAILURE_NO_INFERENCE`. Runtime integrity passed and no
current-4B request was dispatched.

Subsequent safe RAM recovery unloaded only the idle
`qwen3-embedding-0.6b` in-memory backend and repeated clean-cache recovery
with zero writeback. No model artifact/profile or service changed. All five
required containers remain running and not OOM-killed; defined health checks
are healthy. `vmmemWSL` contracted from approximately 1.755 GiB to
1.334 GiB, LocalAI now reports zero loaded models, and available RAM is
6.640 GiB while Codex remains open. Closing Codex is expected to release the
remaining host memory needed for the unchanged admission gate.

The next standalone retry again passed all source/retrieval gates and reached
8.521 GiB at best, 0.106 GiB below admission; it ended
`RESOURCE_ADMISSION_FAILURE_NO_INFERENCE` with runtime integrity PASS. No
optional Docker container was running, so the required stack was left intact.
Non-terminating working-set trimming was applied to idle Windows shell and
OneDrive pages. Windows service-control access was unavailable, so WSearch and
SysMain were not changed; Defender was idle and remained enabled. OneDrive was
then shut down through its own `/shutdown` interface. Available RAM rose from
6.537 GiB to 6.814 GiB with Codex open, a measured 0.277 GiB gain. Combined
with the last closed-Codex peak, expected admission headroom is approximately
0.171 GiB. OneDrive should remain closed until adjudication finishes, then may
be restarted normally.

Exact next action: close Codex and unused applications, keep Docker running,
then execute:
`powershell.exe -NoProfile -ExecutionPolicy Bypass -File "C:\Users\sheik\Workspace\Office\Projects\NexusAI\scripts\nxb21-english-functional-qualification\run_product_convergence_development.ps1"`.
Do not create V3.

## 2026-09-15 NX-B2.1D Work Item 3 — conversational contract complete

`forensics.request-class/v1` is now the single terminal semantic contract used
by the query handler, retrieval-first semantic resolver, and agent router. It
distinguishes GENERAL_DOMAIN_KNOWLEDGE, PRODUCT_HELP,
GOVERNED_EVIDENCE_ANALYSIS, CONTEXTUAL_FOLLOW_UP, CLARIFY, and UNSUPPORTED.
Case/evidence wording and explicit evidence bindings remain governed analysis;
domain definitions cannot claim case facts. Product help is rendered from the
current family capability definitions and exposed operation registry, including
their explicit support, certification, and exposure states. Missing registry
truth yields a bounded unavailable state.

`forensics.context-mutation/v1` records field-level INHERIT, REPLACE, REMOVE,
and CLARIFY decisions with the issuing analysis/audit identity. The bounded
conversation envelope carries either a registered operation or the prior
validated `forensics.source-native-plan/v1` AST, issued field descriptors,
targets, time and direction, source/evidence/version scope, result/entity/fact/
citation handles, expiry, and inherited-field audit. Dynamic continuations now
support numeric or worded top limits, one issued grouping-field filter, issued
group replacement, AVG-to-SUM replacement when the field permits SUM,
previous-calendar-month replacement, removal of direction/date constraints,
and transition to exact source-row view. Every dynamic mutation is revalidated
against the typed AST and issued field catalog. Registered continuations retain
the prior operation and omitted authorized filters. Expired, cross-user,
cross-tenant, cross-collection, cross-evidence/version, oversized, malformed,
or ambiguous context fails closed to CLARIFY. Source/result/citation transitions
are rebound through the authoritative selected-evidence path before execution.

The Fact Packet remains the truth authority. The deterministic narrative guard
now admits bounded reporting-verb weakening, the required active/passive camera
form, and the issued counterparties paraphrase while retaining exact critical
tokens, fact/citation linkage, polarity, uncertainty, endpoint order, and
source-supported rank terms. Claims may combine at most three separately
validated fact clauses. Unsupported causality, certainty, identity, ownership,
location, relationship, value, fact/citation, and follow-up changes reject.
Narrative failure still returns the deterministic server answer with facts,
citations, limitations, and source affordances; suggested questions remain an
exact selection from the server-issued allowlist.

Source acceptance: focused request-class, product-help, registered/dynamic
follow-up, handle, expiry/scope, paraphrase, critical-value, polarity/order,
Fact Packet, allowlist, fallback, and agent-router tests PASS. Full
`go test ./api/forensic_records -count=1` PASS in 61.231 seconds. Full
`go test ./core/services/agents -count=1` PASS in 55.655 seconds with the healthy
Docker Desktop `desktop-linux` 29.6.2 engine supplying PostgreSQL testcontainers.
Focused forensic proxy tests PASS; the agentpool Ask-path package compiles (no
matching focused tests); `go vet` passes for `pkg/forensicrequest`,
`api/forensic_records`, `core/services/agents`, and
`core/http/endpoints/localai`. No Work Item 3 UI source changed, so UI execution
was not required. No live inference, new proof, deployment, model/profile,
retained database/evidence, or git stage/commit/push/PR action occurred.
NX-B2.1D remains OPEN.

Exact next action: proceed to Work Item 4: rebuild the reusable development
harness from current source, verify source/binary identity and run the single
fair current-4B development adjudication with registered, dynamic, terminal and
synthesis cases.

## 2026-09-14 NX-B2.1D Work Item 2 — source-native typed algebra complete

The canonical records path now issues `forensics.field-descriptor/v1` catalogs
from the current authorized tenant/collection/evidence/version scope. Stable
opaque `fld_<24 hex>` IDs bind source names on the server. Descriptors record
normalized and observed source names, STRING/INTEGER/DECIMAL/BOOLEAN/TIMESTAMP/
DATE/UNKNOWN type, presence/null/incompatible counts, privacy state, allowed
filters and aggregates, project/group/sort policy, family/source provenance, and
selected evidence/version provenance. Sensitive name classes are omitted before
catalog issuance, and mixed or unparseable fields cannot be grouped, sorted, or
aggregated unsafely. The independent six-row fixture issues 13 visible fields;
`secret_token` is withheld and `mixed_metric` records one incompatible value.

Deterministic lexical field retrieval issues at most 12 descriptors and records
rank and score in semantic planner audit. The strict
`forensics.source-native-plan/v1` AST supports type-aware filters, up to 12
projected fields, up to two group fields, four ordered measures (`m1` through
`m4`), COUNT/SUM/AVG/MIN/MAX, source-native hour/day/Monday-week/month buckets
through the existing calendar implementation, numeric HAVING, two deterministic
sort keys, and a bounded limit. Input is capped at 1,000 authorized rows, result
groups at 100, and total AST complexity at 16. Unknown values remain null and do
not become zero. The server rejects unknown JSON controls, unissued/reused field
IDs, disallowed type/operator combinations, invented filter/HAVING literals,
cross-scope IDs, injection-shaped IDs, and cap overflow. The model-facing schema
contains issued field IDs rather than source names, JSON paths, tables, SQL, or
authorization identifiers.

The representative query, `Which department has the highest average invoice
total?`, retrieves `department` and `invoice_total`, binds GROUP + AVG + descending
sort + LIMIT 1, and returns `Sales` with average `400`, matching the independent
oracle. Project; one/two-key group; COUNT/SUM/AVG/MIN/MAX; HAVING; aggregate sort;
limit; null group; zero result; and source timestamp bucket cases pass. Comparing
aggregate results by an issued source field is expressible as a bounded group;
the existing registered two-operand compare primitive remains unchanged. The
execution reuses the canonical records route: a parameterized, RLS-bound source
scan feeds the validated bounded typed executor. Each result retains exact
contributing record, evidence/version, file, row, hash, timestamp, batch, and file
locators. Those locators now flow through the existing enterprise response, tool
result, Fact Packet, and citation pipeline rather than creating a second result
ecosystem.

Disposable PostgreSQL/API acceptance passed with a random
NOSUPERUSER/NOBYPASSRLS database and the public case-query handler. Receipt:
`reports/nxb21/source-native-sql-api-20260914T102724987Z.json`. The scratch
database and role were removed. Retained tuple remained
`69|69|82|22512|876|69`, Activity count remained `354`, active jobs remained `0`,
and PostgreSQL container/image identity remained unchanged. Final source
acceptance: focused source-native contracts PASS; disposable SQL/API PASS; full
`go test ./api/forensic_records -count=1` PASS in 66.685 seconds; full
`go test ./core/services/agents -count=1` PASS in 101.931 seconds; focused
forensic proxy identity/ownership/timeout checks PASS; and
`go vet ./api/forensic_records ./core/services/agents` PASS. No live inference,
formal proof, model/profile change, deployment, retained migration/evidence
mutation, or git stage/commit/push/PR occurred. NX-B2.1D remains OPEN.

Exact next action: proceed to Work Item 3: unify help/analysis terminal contract,
typed INHERIT/REPLACE/REMOVE/CLARIFY follow-up mutation, and finish bounded
narrative grounding regression closure.

## 2026-09-14 NX-B2.1D Work Item 1 — retrieval-first semantic resolver complete

The English open-ended semantic path now classifies the request, applies the
server-owned exposure and evidence-scope filter, ranks the complete eligible
operation pool with deterministic BM25 descriptor retrieval, and issues the top
five registered operations plus one `DYNAMIC_TYPED_PLAN` entry to one constrained
model decision. The mandatory family model call is bypassed. Family taxonomy and
metadata remain available for registry, authorization, capability help, audit,
and diversity uses. Registered-operation binding, dynamic typed planning,
execution authority, Fact Packets, and provenance remain server-owned.

The workspace pool contains 66 eligible operations after engineering-only
exclusion. The independent eligible English analytical variant-ledger slice is
137 entries: top-1 117/137 (85.40%), top-3 126/137 (91.97%), and top-5 132/137
(96.35%). All five required CDR, ANPR, document, audio, and generic development
operations are in the top five. Ranking and score stability passed ten repeated
runs. Audit now records eligible count, retrieved candidates and scores, top-k,
selected decision, dynamic selection, request class, scope-filter reason, and
binding state. `DYNAMIC_TYPED_PLAN` is issued once; `UNSUPPORTED` is terminal and
does not trigger a hidden retry. Parent cancellation remains authoritative.

Source acceptance: full `go test ./api/forensic_records -count=1` PASS; full
`go test ./core/services/agents -count=1` PASS; focused forensic proxy timeout,
retrieval, binding, and cancellation regressions PASS. No Ask UI source was
changed. No live inference, proof, deployment, model/profile, database, retained
evidence, or git staging/commit action occurred. NX-B2.1D remains OPEN. Exact next
action: proceed to Work Item 2, server-issued source-native `FieldDescriptorV1`
and bounded project/group/aggregate dynamic algebra.

## 2026-09-14 convergence source acceptance — development inference not admitted

Current report: `reports/nxb21/product-convergence-development-20260914.json`.
Hierarchical family/operation selection and conservative grounded paraphrases are
source-tested. Canonical calendar buckets, source/period count comparisons and
filter/group/count/sort/top-k passed 11 disposable SQL/API specifications.
The outer typed comparison guard, comparison presentation, and canonical typed
record-type/source-file filter conversion were fixed during these checks.
Full forensic and agent regressions PASS; scratch cleanup and retained tuple
`68|68|81|22511|870|68` unchanged. Running services were not deployed or modified.

The latest governed development attempt stopped before inference: peak recovered
RAM 6.602 GiB versus the unchanged 8.627 GiB admission gate. Runtime integrity PASS;
no model loaded, no semantic/synthesis scores measured, no formal proof created.
The six-question/three-synthesis development driver is built and digest-checked.
Operator next: save work, close Codex/unused applications, keep Docker running,
and run `scripts/nxb21-english-functional-qualification/run_product_convergence_development.ps1`.
Its service/model checks, admission gate, telemetry and unload guards remain active.
Model replacement is UNDETERMINED, not justified by a failed RAM preflight.

Full natural-English CDR/document/ANPR/generic/text-audio flows, remaining general
help/unsupported response integration, broader grounding and comparison coverage
remain OPEN. These SQL/API fixtures do not certify those analyst flows. V2 is
consumed and immutable; no rerun or V3. NX-B2.1D OPEN, strict PENDING.

## 2026-09-11 product convergence development — implementation in progress

Latest owner directive: hierarchical semantic selection and safe paraphrase
grounding, then disposable execution and a six-question development smoke.
V2 remains consumed; its 202 frozen source files were hash-verified and copied to
`local-acceptance-models/nxb21-english-product-proof-v2-20260911/consumed-source-snapshot`.
No new formal proof, holdout, model/profile change or deployment is authorized by
this checkpoint. Runtime admission and abort guards remain unchanged.

Source now partitions the scoped registry into 14 semantic families (66 total
operations; mean 4.71 per family, maximum 18), selects a family then its operation
under one planner deadline, and binds facts on the server. Negative English
family decisions cannot fall through to the old question-pruned chooser.
Synthesis permits conservative grammatical paraphrases with exact critical
tokens, single-proposition mapping, citation-to-fact linkage, polarity checks,
entity/value ordering and server-issued follow-up choices. This is a conservative
deterministic guard, not a general natural-language entailment oracle.
Initial full forensic regression PASS after adapting stage-specific mocks;
calendar bucket extension subsequently passed focused UTC/Pakistan/DST tests.
SQL/API, development model measurements and broader end-to-end work remain open.
NX-B2.1D OPEN; resource track CLOSED; strict certification PENDING.

## 2026-09-11 V2 consumed — runtime duration abort and functional gate failures

Current authority: `reports/nxb21/english-product-proof-v2-consumed-adjudication.json`.
V2 is CONSUMED_DO_NOT_RERUN: 40/48 complete (A20/B6/C14/D0); D01 started,
no durable D answer. The unchanged 1200-second runner wall-time guard aborted.
Cold RAM admission PASS, owned-model unload PASS, runtime/retained integrity PASS.
A supported direct operation selection 0/14; B routing 5/6 (B05 deterministic
fast-path error); C validated narratives 1/14, thirteen safe grounding fallbacks.
All fourteen C HTTP/model outputs completed; no C timeout. Rejection includes
verbatim-sentence constraints and unissued follow-ups, not thirteen proven
hallucinations. C05 retains values but fails punctuation-sensitive exact-sentence
coverage. Frozen scores remain unchanged; C09 stored-output review passes.
Current integrated configuration is insufficient for English functional
qualification; model artifact remains runtime baseline only, strict PENDING.
No resume, lock deletion, duration increase or immediate V3. Next work is generic
planner, routing and synthesis-contract diagnosis with independent development
fixtures. NX-B2.1D OPEN; resource track remains CLOSED. Earlier handoff below
was valid before consumption and must not be used as a new run authorization.


## 2026-09-11 fresh English product proof V2 — frozen operator handoff

Authoritative handoff: `reports/nxb21/english-product-proof-v2-handoff.json`.
Proof ID: `nxb21-english-product-proof-v2-20260911`; 48 fresh cases: A20/B6/C14/D8.
Disposable PostgreSQL + actual typed API grouping acceptance PASS, including
source-row/group counts, ties, exact lineage, scope, filtering, zero results,
100-group/1000-row boundaries and fail-closed controls. The NULL bucket uses an
explicitly nullable adversarial scratch fixture; production columns stay NOT NULL.
Owned disposable databases/roles removed; retained tuple unchanged.
Fixed the help adapter discovery URL and rejection of ignored group sort aliases.
Full forensic and agents regressions, proxy, Ask presentation (8/8), evaluator
self-tests and PowerShell 5.1/7 checks PASS. ValidateOnly PASS in both shells:
identities/runtime integrity pass, no inference, no dispatch or consumption.
Memory state: OPERATOR_MEMORY_CLEANUP_REQUIRED; cold gate remains 8.627 GiB.
Current 4B model/profile/envelope unchanged; resource track CLOSED.
Freshness: 15,845 files scanned; zero normalized exact/prior-corpus overlap.
The consumed V1 artifacts and all 177 snapshot files remain preserved.
Next action is the single handoff command after manual app closure; no additional
build, corpus edit, source tweak, tuning, or automatic proof dispatch.
Postrun content review must use immutable captured outputs and separate hash-bound
adjudication. Safe fallback does not count as model narrative success (12/14
validated model narratives required); supported direct selection requires 14/14.
NX-B2.1D remains OPEN; strict certification, general TIME_BUCKET/COMPARE/MULTI_STEP,
broader family acceptance, browser/deployment and model-effectiveness closure are
not established. Historical entries below describe their earlier checkpoint.


## 2026-09-11 V2 preparation — disposable group acceptance

Disposable PostgreSQL SQL/API acceptance PASS (8 focused specs), receipt
reports/nxb21/group-sql-api-v2-20260911T044654141Z.json.
Owned scratch database and role removed; retained tuple unchanged.
Grouping scope, lineage, counts, bounds, NULL defensive fixture and malformed
controls passed. Fixed silently ignored typed group sort_by/sort_direction.
Product help now uses the actual registered adapter discovery route; tested
with real discovery handlers. Fresh V2 package preparation IN PROGRESS.
No live inference or deployment. NX-B2.1D remains OPEN.


## 2026-09-11 NX-B2.1D post-proof source remediation

Final source checks: full forensic package PASS (53.556 s), full agents PASS
(50.747 s), proxy timeout PASS, Ask presentation 8/8 PASS. Grouping passes
governed plan validation, executor dispatch and presentation helpers with the
100-group output ceiling. SQL/API live acceptance remains pending.

45_CASE_PROOF=CONSUMED_FAILED; RESOURCE=PASS; current 4B KEEP as the English
functional runtime baseline. English model effectiveness and strict certification
remain unproven/pending; NX-B2.1D remains OPEN. No live inference, deployment,
service restart, model/profile change, RAM-gate change or new proof in this pass.

PLANNER=GENERIC_REMEDIATION: the selection prompt now separates operation meaning
from execution requirements. The model still emits only an issued enum; the
server binds scope/facts and records READY, SERVER_BINDABLE or USER_FACT_REQUIRED, rechecking after
final request binding. All 66 workspace descriptors are audited in
`reports/nxb21/semantic-binding-descriptor-audit-v1.json`; no question-based candidate
pruning or benchmark phrase patches. This repairs a contract defect; it does not
prove the cause of every historical model rejection or improved model behavior.

SYNTHESIS_TIMEOUT=SOURCE_DEFECT: removed the internal eight-second cap. Both
synthesis paths default to configured 120 seconds, clamp to 180 seconds and honor
earlier parent deadlines/cancellation. The agent and proxy share a 405-second
transport ceiling (180 planning + 30 execution + 180 synthesis + 15 delivery).
Individual model calls and frozen runtime guards are unchanged. Tests cover an
actual five-second cancellation, valid output after eight seconds, parent cancel,
malformed responses, invented values/foreign references and exact fallback retention.

GENERAL_HELP=GROUNDING_REMEDIATION: non-deterministic forensic assistant turns
receive question-independent adapter/template discovery with authenticated scope,
a five-second deadline, bounded response bytes and bounded model context. Catalog
status is preserved; registry presence cannot establish runtime availability,
licensing or successful processing. Domain policy distinguishes subscription,
SIM/profile, equipment and phone identifiers; IPDR fields vary by provider;
similarity is not calibrated probability and universal thresholds are forbidden.
Model compliance requires fresh evidence and is not certified by source tests.

PROJECT retains its existing seven-field source-validated implementation.
AD_HOC_GROUP now has a bounded canonical count implementation for record_type or
source_file: at most 1000 complete input rows and 100 groups, explicit null bucket,
count-descending deterministic ties, exact row lineage, scope/type checks and no
SQL or expressions from the caller. Overflow fails closed. Projection, custom
sort and pagination cannot be silently combined. Typed lowering, agent argument,
catalog input and executor are wired; live SQL/API acceptance remains pending.
Source-native grouping, general TIME_BUCKET/COMPARE/MULTI_STEP remain OPEN.
Document literal/source/version correctness remains source tested; broad OCR/RAG,
all-family acceptance and deployed Ask interaction qualification remain OPEN.

Historical consumed runner, corpus, freeze, evaluator binary and receipts are
preserved. Original frozen API/agent source is retained under
`local-acceptance-models/nxb21-small-english-product-proof-v1/consumed-source-snapshot`.
Current source intentionally differs from that historical freeze; do not rebuild
or rerun the consumed package. Final source checks and exact next actions are in
`reports/nxb21/english-functional-remediation-source-20260910.json`.


## 2026-09-10 English proof completed — functional gate failed, resource fit retained

Run `run-20260910T100919072Z` completed all 45 cases. Receipt hash verified;
runtime_before equals runtime_after, unload PASS, retained tuple/Activity/jobs and
service identities/restarts unchanged. Corpus is CONSUMED_DO_NOT_RERUN. The runner's
combined INCOMPLETE_OR_FAILED label means completed-but-failed here: all rows exist,
evaluator completed normally with 11 automated failures. No RAM-admission issue
remains for this run and no further resource tuning is called for.

Stage A: schema-valid 16/16, zero operation selections. Twelve insufficient-facts
clarifications and four unsupported responses; 11 frozen-oracle failures (nine
supported-operation clarifications and two ambiguity/follow-up rejections). Five
accepted enum outcomes are not five successful analytical operations. Planner
root cause is not proven: task framing/binding context needs source diagnosis.
Stage B: 8/8 router outcomes pass (three deterministic, five model proposals), not
live end-to-end tool execution. Stage C: 0/13 model narratives, 13/13 deterministic
fallbacks. Every call hit the internal 8-second maxNarrativeWait cap despite config
SynthesisTimeout=120s. Earlier statements that this path used 120s were incorrect.
Exact fallback facts, identifiers, limitations and references pass independent
comparison; this does not establish model synthesis quality.
Stage D: eight responses captured; content review finds 3 acceptable, 3 qualified,
2 failures. OCR answer invents universal confidence thresholds (90%/70%) without
calibration; file-type answer invents licensing dependence and an unsupported claim
that no forensic capability lookup exists. Other caveats include IMSI/device wording,
IPDR provider variation, and treating face similarity as a probability.

Runtime: physical minimum 3.683 GiB; minimum commit reserve 10.526 GiB; WSL swap
growth 48652 KiB (~47.5 MiB); four isolated paging spikes, maximum 4604 pages/s, no
recorded sustained guard breach. Evaluator duration 571.618 s. KEEP runtime baseline;
English functional qualification FAILED, strict PENDING, historical multilingual
insufficiency preserved, resource track CLOSED, D OPEN. No deployment or new inference.

Machine adjudication with artifact hashes and per-answer review:
`reports/nxb21/small-english-product-proof-adjudication-v1.json`.
Next: repair narrative-timeout integration with cancellation tests, diagnose generic
planner task boundaries without phrase patches, ground help in capability discovery,
and continue bounded algebra/family work. Do not rerun this consumed proof or silently
change its oracles, historical receipts, runner, binary or freeze.


## Manual proof execution selected

Owner requested a manual PowerShell command after closing Codex. Process inventory
confirmed the deferred helper is absent; its old WAITING state file was stale and
is now marked INACTIVE_MANUAL_RUN_SELECTED. Both proof locks are absent and frozen
identity verification PASS. Docker engine 29.6.2 reports all five containers running;
health checks healthy where configured, no OOM, restart counts unchanged (0/6/4/0/0).
Do not start another background launcher. Use the original frozen proof command.


## 2026-09-10 authorized RAM cleanup and deferred proof launch

Owner requested safe cleanup and completion without changing the gate. Closed the
Docker dashboard and PowerToys Quick Access windows gracefully; stopped the verified
idle Command Palette background process. No files deleted, no protected service
stopped, no model unloaded. All five NexusAI containers remained running. Available
physical RAM sampled 6.483 GiB while Codex remained open; no gate-pass claim.

One-time helper PID 21068 is armed in
`local-acceptance-models/nxb21-small-english-product-proof-v1/launch-20260910T100236548Z`.
It waits up to 15 minutes for exact Codex app PID 27640/start-time identity to exit,
then invokes the unchanged frozen proof once. It does not close Codex itself,
change admission, or retry. Read launcher-state.json and proof-console.log on
return, then latest product receipt and all answers. English proof remains pending
until actual execution and independent review; do not launch a concurrent runner.


## 2026-09-10 standalone English proof — cold admission not met

Receipt `run-20260910T095549213Z/product-proof-receipt.json` and its SHA-256
verified. All 24 cold-admission samples were below the frozen 8.627 GiB floor;
maximum and last sample 8.473 GiB, shortfall 0.154 GiB (157.696 MiB). The recovered
plateau was approximately 8.403–8.473 GiB. No model request was dispatched;
`dispatch_created=false`, both dispatch locks absent, corpus remains UNCONSUMED.
Before/after runtime state is identical: retained tuple 68|68|81|22511|870|68,
Activity 353, jobs 0, service identities/restarts and loaded-model set unchanged.
Unload NOT_REQUIRED; runtime integrity PASS.

This is PRELOAD_ADMISSION_NOT_MET, not model execution or semantic failure.
KEEP / ENGLISH_FUNCTIONAL_BASELINE and the completed tested-shape runtime PASS
remain unchanged. English product proof and D remain pending. No lowering of cold
admission, no reuse of the 8.5 GiB reload gate for cold loading, no replacement
corpus, and no repeated resource-tuning loop. The existing proof can run only when
its unchanged admission prerequisites are met. Source-only work may continue;
editing frozen evaluator dependencies requires explicit package reconciliation
before any eventual dispatch. Earlier immediate-run instructions are superseded
by this admission status.


## 2026-09-10 completed characterization — runtime frozen; English proof prepared

Read-only proof preflight on 2026-09-10 verified frozen/source/model identities,
accepted service IDs and retained baseline, then stopped at the unchanged emergency
physical guard: available 1.181 GiB, commit reserve 7.453 GiB, paging 0, current 4B
unloaded. State PREFLIGHT_FAILURE_NO_INFERENCE. Both proof dispatch locks remain
absent. This is current-host admission state with applications open, not a reversal
of the completed model characterization. No threshold change or resource rerun.
Operator action remains manual app closure and one standalone product-proof run.

Final run `run-20260910T052648583Z` completed all 14 requests with clean unload
and runtime/retained integrity PASS. Current 4B decision KEEP, runtime stage
ENGLISH_FUNCTIONAL_BASELINE, sustained fit PASS_FOR_TESTED_PRODUCT_SHAPES.
Strict certification PENDING; historical multilingual insufficiency and consumed
r1/r2/Decision8/8B receipts are preserved. The current-host 8B resource path and
4B resource-tuning track are CLOSED.

`configuration/current4b-runtime-envelope-v1.json` freezes the measured policy:
cold 8.627 GiB; reload 8.5 GiB with five stable multi-signal samples; emergency
physical >1.5 GiB, commit >2 GiB, swap growth <=256 MiB; sustained paging guard
1024 pages/s for 15 s; request cap 180 s and session cap 1200 s. Build/recreate
remains 6 GiB. The 4.191 GiB observed minimum is not proof of sustained operation
at the emergency floor. One transient paging spike and incomplete immediate WSL
recovery are disclosed. Large identical-body timings 131.100/6.093 s are consistent
with prompt/context reuse; cache-hit causality is unproven and tiny priming already
preceded the slow request. No additional latency experiment is planned.

A single fresh 45-case English proof covers 16 planner, 8 router, 13 synthetic
Fact Packet synthesis and 8 general-assistant cases. Its offline freshness check
scanned 5426 text files without normalized exact-question overlap. Independent
content review is mandatory; fallback is reported separately and never counts as
model synthesis success. This is source-component evidence, not deployed end-to-end
or family certification. Runner:
`scripts/nxb21-english-functional-qualification/run_small_english_product_proof.ps1`.
Run once after manually closing nonessential apps; keep Docker/WSL and a terminal.
No inference has been dispatched by this work package.

Source progress: typed canonical PROJECT now retains provenance, exact identifiers
and nulls after privacy redaction; seven allowlisted fields, no payload expressions,
no ignored controls. Full forensic regression passed in 41.120 s; later offline
fixture/projection/deadline checks passed. The planner has a cancellable 180 s
maximum, and Ask displays actual elapsed time and cancellation acknowledgement.
AD_HOC_GROUP, TIME_BUCKET, general COMPARE and multistep composition remain OPEN.
P1 family and live acceptance remain OPEN. NX-B2.1D remains OPEN pending English
proof and remaining functional requirements, not future multilingual optimization.
Next sequence remains D → E proof framework → F family certification → G Ask UX
→ H live demo. Deployment/recreation and retained mutation require explicit approval.
Earlier next-run instructions below are historical where superseded.


## 2026-09-10 final current-4B reload admission adjudication

Completed v2 run `run-20260910T050248809Z` is preserved as
INCOMPLETE_PRE_RELOAD_ADMISSION. Load plus four tiny calls passed; loaded minimum
4.095 GiB, minimum commit reserve 10.975 GiB, zero WSL swap growth, post-load
backend RSS growth 4.890 MiB, unload and runtime integrity PASS. This is safe and
stable for that tested interval; larger requests remain unproven.

The 8.627 GiB reload gate is predictive, not a measured failure limit. One final
owner-authorized correction uses 8.5 GiB reload admission, derived from 4.677 GiB
observed peak draw + 3.5 GiB selected reserve + 0.25 GiB observed-variation margin,
rounded up, with commit/paging/swap/unload/service/state checks. Initial admission
8.627 GiB and build/recreate gate 6 GiB are unchanged. This is a candidate final
experiment envelope, not yet a qualified sustained product operating envelope.

Runner `scripts/nxb21-english-functional-qualification/run_current4b_final_characterization.ps1`
is frozen; PowerShell 5.1/7 tests and live read-only ValidateOnly PASS. No inference
was dispatched. Full forensic-records suite with six new document literal/source
acceptance cases passes in 66.603 seconds. Production behavior and model/profile/
prompts remain unchanged in this package.

Next: manually close nonessential apps, keep Docker/WSL and one terminal, run the
final runner ONCE per owner directive sections 13/22. On acceptable completed
runtime proof keep 4B runtime baseline, strict certification pending, then English
product proof. On failure reject sustained local role and select one smaller
artifact; no further 4B profile/threshold/soak loop. No r3 corpus, deployment or
retained mutation. NX-B2.1D remains OPEN.

The current section of `reports/nxb21/nexusai-product-functional-all-family-gap-analysis-20260909.md`
contains all metrics, margins, exact command and hashes. Machine telemetry
adjudication is `reports/nxb21/current4b-v2-runtime-adjudication.json`; the existing
product matrix remains authoritative. Earlier next-run instructions below are
historical where superseded.

## 2026-09-10 product-completion v2 — source fix; runtime characterization prepared

The latest owner directive prioritizes English functional breadth while preserving
historical strict/consumed evaluations. NX-B2.1D remains OPEN. The original
lifecycle soak failed in PowerShell telemetry; its receipt is not semantic model
evidence. No consumed corpus was rerun and no inference or deployment occurred.

Typed algebra now rejects malformed/dropped controls and repeated scalar filters;
the full forensic-records package passes (39.778 s). The separately versioned
characterization runner passes tests in PowerShell 5.1/7 and live read-only
ValidateOnly. The original soak runner, freeze and receipts are preserved.

One owner-authorized standalone runtime experiment is ready after manual closing
of nonessential apps, per directive section 92. The 6 GiB build/recreate gate is
unchanged; 4 GiB is a historical qualification guard with no demonstrated OOM or
paging boundary. The new experiment records it diagnostically with explicit
resource/integrity abort guards. No lower operating floor or model role has been
adopted. Current model decision and English functional proof remain pending.

Authoritative current report, exact command, abort guards, limitations and next
phases: `reports/nxb21/nexusai-product-functional-all-family-gap-analysis-20260909.md`
(2026-09-10 section). Machine product matrix remains
`reports/nxb21/product-query-capability-matrix-v1.json`. Actual next phases after
D: E proof framework, F family certification, G Ask UX, H live demo. No phase is
closed by this source-only package. Earlier next-action instructions below are
historical where they conflict with this overlay.

## 2026-09-09 r2 consumed resource failure — disposable lifecycle soak frozen

Final run `run-20260909T110254961Z` passed stable preload (minimum 8.733 GiB),
loaded RAM (minimum 4.201 GiB), three generic requests, and post-soak RAM
(minimum 4.177 GiB). It then created the r2 dispatch lock. The first semantic
request, `r2-sem-001`, was still in flight when available RAM reached 3.966 GiB
after 46.584 seconds. Zero semantic, router, or synthesis cases completed. The
owned model unloaded and runtime integrity passed. R2 is
`CONSUMED_DO_NOT_RERUN`; current-4B semantic quality is
`INSUFFICIENT_EVIDENCE`.

Host RAM declined from the first qualification sample of 4.160 GiB through a
last above-floor reading of 4.027 GiB to 3.966 GiB. Evaluator working set peaked
at only 22.137 MiB and stayed flat. R2 had no per-call API/backend/vmmem series,
so KV/context accumulation, API/backend growth, WSL growth, and runtime leakage
remain unproven. Exact non-inference measurement shows each semantic request
carried 66 candidates and approximately 2,778–2,800 tokens (11,111–11,197
prompt bytes; 14,798–14,884 request bytes). The evidence supports observed host
drift plus the large first-request shape as the likely trigger, compounded by
only 0.177 GiB post-soak headroom.

Disposable soak `nxb21-current4b-disposable-resource-lifecycle-soak-v1-20260909`
is frozen, statically validated, and not executed. It runs 20 generic tiny calls
in batches `1,2,4,8,5` with owned-model resets, then three non-forensic
payload-shaped calls, recording per-call host RAM, API memory, corrected backend
RSS, vmmemWSL, and latency. Runner SHA-256 is
`18f27c767982e4e078ad3280b69aa44f7af830a7d7642378e2a70e71df3c95a2`;
freeze SHA-256 is
`5e206207dc49737056481eb3f4c510d417e2540271340e075ba1dcc62a66c0c2`.
`-ValidateOnly` passes without inference. R3 is not created and is prohibited
until this soak passes. Full adjudication:
`reports/nxb21/nexusai-r2-consumed-resource-failure-and-lifecycle-diagnosis-20260909.md`.

## 2026-09-09 final current-4B stable-state resource harness frozen

The two r2 standalone attempts (`run-20260909T102228770Z` and
`run-20260909T103428085Z`) stopped before model load because the old runner
treated rising recovery observations as the formal preload window. They created
no resource or qualification lock, made no inference request, preserved runtime
integrity, and left corpus
`nxb21-english-functional-current-4b-fresh-r2-20260909` fresh and unconsumed.
They are not current-4B runtime or semantic evidence.

The runner-only correction is frozen. Diagnostic recovery samples are now
separate from the five-sample qualifying window. Recovery is observed every
five seconds for at most 120 seconds; formal preload remains 6 GiB, loaded and
post-soak floors remain 4 GiB, and a first-to-last decline greater than 0.5 GiB
rejects a nominally qualifying window. The authoritative r1 measurements are
6.720 GiB preload and 2.093 GiB loaded, yielding a 4.627 GiB observed delta.
Preferred empirical admission is 9.127 GiB with the existing 0.5 GiB margin;
the no-margin credible predicted-loaded threshold is 8.627 GiB. If neither can
be sustained, the final run stops before model load as
`CURRENT_4B_PREDICTED_RESOURCE_INSUFFICIENT` and permits no further 4B tuning
cycle.

Old runner SHA-256:
`356e45013fc22da10e8ae4d88248de240f671087d4fe482732187d0f0202b70b`.
Corrected runner SHA-256:
`8b4b6147175d3d662673f44295d21e95a59a897b4437001a156f7e7d3d789458`.
Updated freeze SHA-256:
`4b804681d71c1b4f0b6f86fef229d95dd42cef0f0bf0e177b1cd7c4deaae1ea5`.
Static state-machine, hash, tamper, gate-order, process-safety, and dispatch-lock
checks pass. The corrected `-ValidateOnly` pass reverified model/profile identity,
runtime baseline, and `LIVE_INFERENCE=false`. Corpus, oracle, evaluator, model,
profile, semantic contracts, and questions are byte-identical.

Current state: `RESOURCE_HARNESS_CORRECTION=PASS`,
`R2_CORPUS_CONSUMED=false`, `MODEL_STATE=UNLOADED`,
`NX-PFC-P1-001=FINAL_STABLE_RESOURCE_ADMISSION_PENDING`, and
`NX-PFC-P1-002=FINAL_STABLE_RESOURCE_ADMISSION_PENDING`. The operator must close
Codex/ChatGPT and other nonessential applications manually, keep Docker Desktop
and exactly one PowerShell terminal, wait 20–30 seconds, then run the corrected
standalone command exactly once. This is the final current-4B resource-admission
adjudication.

## 2026-09-09 first standalone attempt — preload gate failed, R2 unconsumed

Standalone run `run-20260909T102228770Z` failed closed at the formal preload
gate. Its five available-RAM samples were 4.304, 4.301, 5.441, 7.233, and
8.418 GiB; the minimum was 4.301 GiB against the unchanged 6 GiB floor. The
late rise does not retroactively satisfy the requirement that every sample be
at least 6 GiB, and the 9.127 GiB empirical advisory target was not met.

`RESOURCE_SMOKE=NOT_STARTED`, the resource lock was not created, the current
4B model was not loaded, transport soak and qualification were not started,
and the fresh r2 corpus remains unconsumed. Runtime integrity passed with
retained tuple `68|68|81|22511|870|68`, Activity 353, zero active jobs, and
unchanged container identities/restarts and analyst-index hash. Receipt
SHA-256:
`3bf8fde5c2465910b285651a8fe12a8f0c678b4200b667e5e86ccc622ad039eb`.

This is `PENDING_OPERATOR_MEMORY`, not current-4B runtime or semantic-quality
evidence. Because no model request occurred and the last two samples recovered
above the formal floor, one bounded manual-close retry of the unchanged frozen
runner is allowed. If that retry fails the preload gate again, stop without
another retry or tuning loop and request separate approval for smaller-model
selection. Codex/ChatGPT and other nonessential applications must again be
closed manually before that retry; the runner must not terminate them.

## 2026-09-09 current-4B standalone resource proof prepared — R2 unconsumed

The prior English corpus `nxb21-english-functional-current-4b-fresh-r1-20260909`
is consumed and must not be rerun. It is formally
`INCOMPLETE_RESOURCE_RUNTIME_FAILURE`, not a semantic-model failure. The run
admitted at 6.72 GiB, its first semantic request timed out after 60,056 ms, and
loaded-state RAM was 2.093 GiB against the frozen 4 GiB floor. Only one of 149
cases was attempted, so `CURRENT_4B_SEMANTIC_QUALITY=INSUFFICIENT_EVIDENCE_FROM_THIS_RUN`.
The model was unloaded; retained tuple `68|68|81|22511|870|68`, Activity 353,
active jobs zero, all container identities/restarts, and the accepted analyst
index remained unchanged. Immutable adjudication:
`reports/nxb21/nexusai-english-functional-model-qualification-20260909.json`.

The corrected workflow is frozen but not executed. New corpus
`nxb21-english-functional-current-4b-fresh-r2-20260909` contains 149 cases
(101 semantic, 32 router, 16 synthesis), covers all 66 workspace operations,
and passes freshness across 3,877 files with zero exact, normalized, or
prior-corpus overlap. Corpus SHA-256 is
`df0554164bd6aae54525a3329f3e570cbc37880c2d89eff7cdb58c513d7c74e5`;
oracle SHA-256 is
`80b01fa1c46535d49bfc7a74ce4a244dedb24479b997328b072d6ee75a863ed9`.
The prebuilt evaluator SHA-256 is
`462084d7a76c35dc3d295b8a43139a18e5bb040d28d80a9a6d933996a13c448a`.

The standalone runner SHA-256 is
`356e45013fc22da10e8ae4d88248de240f671087d4fe482732187d0f0202b70b`.
It performs no compilation or repository-wide scan and kills no user/system
application. It requires five preload samples >=6 GiB, then a generic
non-benchmark model-load request, five loaded samples >=4 GiB, two additional
generic transport requests, and five post-soak samples >=4 GiB before the
evaluator may create the qualification dispatch lock. The empirical advisory
preload target is 9.127 GiB, calculated as
`4.0 + (6.72 - 2.093) + 0.5`; it is not a formal threshold. Static hash,
tamper, ordering, process-safety and lock-semantics validation pass;
`-ValidateOnly` passes; the complete forensic-records package passes in
55.181 seconds. The current 4B model is unloaded, both R2 live locks are absent,
and `LIVE_INFERENCE=false`.

Current state: `NX-PFC-P1-001=STANDALONE_RESOURCE_PROOF_PENDING`,
`NX-PFC-P1-002=STANDALONE_RESOURCE_PROOF_PENDING`, `NX-PFC-P1-003=BLOCKED_BY_P1_001_002`,
`NX-PFC-P1-004=BLOCKED_BY_P1_001_003`, `NX-PFC-P1-005=SEPARATE_APPROVAL_REQUIRED`,
`NX-B2.1D=OPEN`, and Qwen3-8B remains `CLOSED_RESOURCE_INSUFFICIENT`.
The operator must save this result, close Codex/ChatGPT, browsers, IDEs and
other nonessential applications manually, keep Docker Desktop and one
PowerShell terminal running, wait 20-30 seconds, then run the one frozen
standalone command documented in
`reports/nxb21/nexusai-current4b-standalone-resource-then-english-preparation-20260909.md`.



## 2026-09-09 English-first full-registry semantic planner foundation — source validated

The product-completion reconciliation found a capability-selection mismatch:
the governed runtime registers 79 executable operations, but unresolved language
was reduced to 22 residual capability tuples and phrase-shaped candidate
filtering. Source now includes `forensics.semantic-operation-proposal/v1`.
For English questions it projects every scope-compatible non-engineering
operation with family, intent, meaning, required/optional parameters, measures,
grouping, result kind, presentation, scope, and exposure metadata. The model can
return only one server-enumerated decision; scope, identifiers, dates, filters,
limits, tools, SQL, and facts remain server-authoritative. Strict decoding,
registry/scope validation, deterministic fact binding, audit publication, and
the existing residual fallback are wired. The complete forensic-records Go
package passes.

The next request-class slice is also source validated. Only explicit or
provably recognized forensic operations take the deterministic fast path;
product help, greetings, relevant general knowledge, and unfamiliar wording
reach the configured assistant/tool router. Its policy requires governed tools
for case/evidence/processing/model-state/computed-finding claims. The complete
agents package passes. Agent-pool integration passed 58/59; the sole failure is
an unrelated repeatable Windows file-lock race in the existing
`agent_jobs_test.go` concurrent persister test.

The bounded typed-query algebra slice is source validated as well. Typed plans
now assert the registered operation's measures and grouping, lower only
executor-consumed filters and sorts, and reject mismatches or ignored controls.
Canonical/generic source rows support allowlisted payload predicates, one
allowlisted sort, bounded top-K, and provenance; frequent-contact direction and
retained-video inclusive source-time bounds are explicitly lowered. The full
forensic-records package passes (527 specs passed, 33 intentionally skipped).
Arbitrary ad-hoc projection/group/time-bucket/compare composition remains
unexposed and uncertified.

The all-family and machine-readable gap/capability matrices are
`reports/nxb21/nexusai-product-functional-all-family-gap-analysis-20260909.md`
and `reports/nxb21/product-query-capability-matrix-v1.json`. This is source
validation only: no deployment, activation, retained-data mutation, new model
gate, or product certification occurred. Read-only live observation is
`68|68|81|22511|870|68`, Activity 353, active jobs zero; the delta from the
last accepted tuple is not attributed.

The final-local Qwen3-8B selection-12 attempt is consumed and closed as
`CLOSED_RESOURCE_INSUFFICIENT`; quality remains `INSUFFICIENT_EVIDENCE`.
Its receipt SHA-256 is
`430f40db43950981d064f2e52b5cc96290f038f28c33e14771a6e34f4583f4db`.
Do not rerun it. `NX-B2.1D=OPEN`, `FINAL_D_CANDIDATE=NOT_YET`, and
activation remains blocked. Next: create a fresh representative English
qualification corpus covering both the one-decision operation contract and the
assistant/tool request classes, then independently oracle the remaining ad-hoc
projection/group/time-bucket/compare slices before any planner exposure;
approval is required before inference, build/deploy, or live acceptance.


## 2026-09-08 conversational investigation UI final — live verified

The final bounded conversation-first refinement is live on canonical `/analyst`.
It preserves `Add data -> Ask -> Answer -> Evidence` and all backend, citation,
Activity, retained-data, and model-selection semantics. Questions are distinct
right-aligned turns; answers are structured cards; safe row/page/time locators
are visible; the composer, header, Evidence, History, Add data, dark mode, RTL,
focus, reduced motion, and 390/820/1024/1440/1600 px layouts passed. A live
browser review caught and closed one horizontal Evidence-drawer overflow before
the final reseal.

Final source digest:
`17b49764f4e8d4225a18ab95b4577595d0ea84daa0e764c83e901cb1d2b89344`.
Manifest SHA-256:
`6f142a28622e430a26e1451ba191ce2735132a15d8ba186fbd6940a198707f25`.
Live image:
`sha256:91620e4c932764c46931dd4eecb47f9f72408e2387da3189f8fcd4ab4d7506be`.
Activation receipt SHA-256:
`0288b3ebdabfd84f75bb553c9355a130a48c915eec24fd8f9a4b7831c4c5f0bc`.
Retained tuple remained `67|67|80|22510|863|66`, Activity remained 344,
active jobs remained zero, all four protected container IDs/images/restart
counts were preserved, and `D_UNQUALIFIED_SOURCE_INCLUDED=NO`.

Verification: focused ESLint zero errors; analyst unit tests 50 pass with one
intentional skip; Playwright 29/29; production build 687 modules; wrapper test
PASS; Go HTTP compile PASS; final real-browser light/dark and drawer/modal review
PASS. Reports:
`reports/nxb21/nexusai-investigation-workspace-conversation-ui-final-20260908.md`
and
`reports/nxb21/nexusai-investigation-workspace-conversation-ui-final-activation-20260908.md`.

The consumed Qwen3-8B fresh selection12 resource failure is unchanged:
`FINAL_D_CANDIDATE=NOT_YET`, `NX-B2.1D=OPEN`, and `D_ACTIVATION=BLOCKED`.

## 2026-09-08 NX-B2.1D fresh low-memory selection12 consumed by resource failure

The one-shot gate
`nxb21d-qwen3-8b-lowmem-fresh-selection12-20260908T071710Z` dispatched its
first model request and is consumed. It must not be resumed or rerun. The run
stopped before semantic adjudication when the loaded-state RAM gate remained
below 4 GiB after the full bounded recovery window (initially 2.396 GiB,
finally 3.195 GiB). Its immutable receipt SHA-256 is
`346a519e8e83ee18ade6ffb9a3a740ed7203f5fe07d5b0c102fcc6d32330f107`.
The receipt records one dispatched model call, `MODEL_UNLOAD=PASS`,
`RUNTIME_POSTCHECK=PASS`, retained tuple `67|67|80|22510|863|66`,
Activity 344, and zero active jobs. This is
`INCOMPLETE_RESOURCE_FAILURE`; Qwen3-8B model-selection quality remains
`INSUFFICIENT_EVIDENCE`.

Only rebuildable workspace caches were removed afterward, recovering about
3.9 GiB of disk space. Governed clean-page reclamation preserved dirty pages
and raised available host RAM from 4.870 GiB to a stable 5.790 GiB while Codex
and ChatGPT remained open; `vmmemWSL` fell to about 1.426 GiB. The exact
model profile, RAM floors, protected services, retained data, consumed lock,
run evidence, and receipt were not changed.

Current state: `QWEN3_8B_MODEL_SELECTION=FAIL_INCOMPLETE_RESOURCE`,
`FRESH_SELECTION12=CONSUMED_DO_NOT_RERUN`,
`FINAL_D_CANDIDATE=NOT_YET`, `FINAL_HOLDOUT=NOT_CREATED`,
`FINAL_QUALIFICATION=NOT_STARTED`, `NX-B2.1D=OPEN`, and
`D_ACTIVATION=BLOCKED`. Stop for owner adjudication; do not create another
corpus, profile, qualification, or activation.



## 2026-09-08 NX-B2.1D fresh low-memory Qwen3 8B selection12 frozen

The sealed post-close resource recheck passed at receipt SHA-256
`f148e40e5dc6a69fa23700c9332bb14c543001625a607be5d217c51b67af917e`;
the unchanged profile sustained three loaded-state readings at 4.422, 4.413,
and 4.304 GiB after entering with 6.767 GiB available. The requested batch is
128 while LocalAI's measured effective `n_batch` remains 512.

One new residual-only development gate is frozen as
`nxb21d-qwen3-8b-lowmem-fresh-selection12-20260908T071710Z`. Its corpus
SHA-256 is `a56f9b8690f5debc6034a682496ffff004bfe89ac1971d5908bcfeba0244512c`;
it contains exactly three English, Urdu, Roman Urdu, and mixed cases, with one
resolved, one ambiguous, and one insufficient-facts oracle per language. All 12
are production-source-admitted model calls with frozen fact packets, candidate
sets, decisions, and final typed outcomes. Freshness passed across 2,020
historical files with zero question, target-value, or normalized-literal
overlap. Schema conversion, strict decoding/rejection, runner syntax, source
tamper checks, one-shot dispatch semantics, and `-ValidateOnly` passed. The
model remained unloaded and no live inference ran.

Current state: `LOWMEM_RESOURCE_RECHECK=PASS`,
`QWEN3_8B_MODEL_SELECTION_GATE=FROZEN_NOT_EXECUTED`,
`FINAL_D_CANDIDATE=NOT_YET`, `FINAL_HOLDOUT=NOT_CREATED`,
`FINAL_QUALIFICATION=NOT_STARTED`, `NX-B2.1D=OPEN`, and
`D_ACTIVATION=BLOCKED`.

After manually closing nonessential applications while leaving Docker and
protected services running, execute exactly:

`powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\nxb21d-qwen3-8b-lowmem-selection\fresh-selection12\run_fresh_selection12_development.ps1`



## 2026-09-08 NX-B2.1D post-close Qwen3 8B resource-only recheck prepared

The owner authorized one standalone post-close resource admission recheck of the
unchanged `qwen3-8b-q4km-nxb21d-selection-lowmem-dev` profile. The exact
profile remains SHA-256
`672f8e2c29f162cdc74a8a8dfe7129955436c771db454b0a3ebb20a3bf9b560a`;
the pinned 5,027,783,488-byte artifact remains
`d98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785`.
The resource-only script is
`scripts/nxb21d-qwen3-8b-lowmem-selection/run_post_close_resource_recheck.ps1`
at SHA-256
`b66fa7d98d246d7b379db84406428ad27862026814c17c1d1ef7d1b0be2a2f14`.
Its `-ValidateOnly` path passed with the model unloaded, retained tuple
`67|67|80|22510|863|66`, Activity count 344, and zero active jobs; it loaded
no model and performed no inference. The live path uses tokenizer-only loading,
records the requested batch 128 and actual LocalAI effective `n_batch`, waits
within the unchanged 180-second settling ceiling, requires three consecutive
loaded readings at or above the frozen 4 GiB floor, unloads only the owned
development model, verifies all container identities and retained/runtime state,
and seals one immutable receipt plus SHA sidecar. The prior effective
`n_batch` observation remains 512; no claim that 128 is active is permitted.
No fresh corpus, qualification, or activation exists. After closing Codex,
Chrome, IDEs, and other nonessential applications while leaving Docker Desktop
running, execute exactly:

`powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\nxb21d-qwen3-8b-lowmem-selection\run_post_close_resource_recheck.ps1`


## 2026-09-08 NX-B2.1D Qwen3 8B selection12 attempt 1 stopped on RAM floor

The one-shot Qwen3 8B selection gate is consumed and must not be rerun. Its
immutable receipt SHA-256 is
`bf45069a8c58f2020a3a1c85c56f02194ca00f7b63bda1f8c3d093cb157dc55f`.
It contains two result entries: `sel12-en-resolved` made the only model call
and passed exactly in 54,405 ms; `sel12-en-ambiguous` made no model call
because available RAM remained below the frozen 4 GiB between-call floor after
bounded recovery. Resource samples measured 2.640–7.907 GiB available RAM and a
6,048.8 MiB API-container peak. The model was unloaded; runtime postcheck,
retained tuple `67|67|80|22510|863|66`, activity count 343, and zero active
jobs passed. This is `INCOMPLETE_RESOURCE_FAILURE`, not a model-quality
failure: Qwen3 8B quality is `INSUFFICIENT_EVIDENCE` from 1/12 model calls.
The inner Ginkgo SUCCESS is a harness-presentation defect caused by returning
after recording the RAM failure; the outer runner and sealed receipt correctly
failed the gate. `FINAL_D_CANDIDATE=NOT_YET`,
`FINAL_HOLDOUT=NOT_CREATED`, `FINAL_QUALIFICATION=NOT_STARTED`,
`NX-B2.1D=OPEN`, and `ACTIVATION=BLOCKED`. The next action is an explicit
resource/model architecture decision before any fresh corpus or freeze.

Incident adjudication:
`reports/nxb21/nexusai-nxb21-d-qwen3-8b-selection12-attempt-1-resource-failure-20260908.md`.


## 2026-09-08 NX-B2.1D Qwen3 8B selection12 frozen; standalone execution pending

The exact authorized `Qwen/Qwen3-8B-GGUF` artifact at revision
`7c41481f57cb95916b40956ab2f0b139b296d974` is verified at 5,027,783,488
bytes and SHA-256 `d98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785`.
Its isolated CPU-only LocalAI profile is registered and unloaded. A fresh
12-case selection gate is frozen at corpus SHA-256
`6376e5d18352edd6319562e9799d773bedb67f54bbedc901f06256dd53b226cd`:
3 cases per English/Urdu/Roman Urdu/mixed and, within every language, one
resolved, one ambiguous, and one insufficient-facts oracle. Freshness PASS
scanned 1,124 historical files with zero question/value overlap. All 12 are
source-admitted residual calls. LocalAI JSON-schema grammar conversion and
strict one-field decoding/rejection tests pass. Freeze identity is
`e0aa163fdf62e12ee3b416bf6fa05061355d16f2ce616566578673ae926e9771`;
the standalone runner's `-ValidateOnly` framework and tamper tests pass. No live
inference ran in Codex. `PREMIUM_UI=LIVE_VERIFIED`,
`Q4_FULL_PLAN_ROLE=RETIRED`,
`Q4_RESIDUAL_ROLE=INSUFFICIENT_FOR_RESIDUAL_ROLE`,
`QWEN3_8B_ARTIFACT=VERIFIED`, `QWEN3_8B_PROFILE=FROZEN`,
`QWEN3_8B_DEVELOPMENT_GATE=FROZEN_NOT_EXECUTED`,
`FINAL_D_CANDIDATE=NOT_YET`, `FINAL_HOLDOUT=NOT_CREATED`,
`FINAL_QUALIFICATION=NOT_STARTED`, `NX-B2.1D=OPEN`, and
`ACTIVATION=BLOCKED`.

Pre-run freeze and exact standalone command:
`reports/nxb21/nexusai-nxb21-d-qwen3-8b-selection12-pre-run-freeze-20260908.md`.

## 2026-09-07 premium Investigation Workspace final drawer live-verified

The premium UI/UX pass and final History-drawer correction are live on image
`sha256:bed2eec364ca8c65151625685f79f5ab7a606a8223af88d1696504751b4a03a1`.
It provides a single-row 1480 px shell, bounded 760 px reading measure, Geist
typography, restrained light surfaces, compact factual answers, icon-led facts,
source cards, responsive 390/820/1024/1440/1600 behavior, real case switching,
browser-local New conversation semantics, and clarification choices sourced
only from retained governed entity activity. The investigation-model decision
is recorded in
`reports/nxb21/investigation-workspace-investigation-model-20260907.md`.

Validation: 50 analyst unit tests pass with one intentional skip; the full
29/29 Playwright suite passes; focused lint has zero errors; production build
passes with 687 modules; wrapper test and HTTP compile-only checks pass. The
final activation passed at 7.322 GiB available RAM, recreated only `api`,
preserved protected services, and preserved retained tuple
`66|66|79|0|22509|850|63`. Final receipt SHA-256:
`8729a485ffabf659ae54547006580dc720930acdbd4145b13cc6c6c1a37d7d6e`.

Direct live review had found one visual-only issue: the 500 px History drawer
inherited a wide four-column journal layout. Its one-column correction passed
focused 3/3 browser checks and is included in the live final image. The sealed
216-file final overlay remains bound to manifest SHA-256
`f961ee03ea2ee3328d2ebc4c0598f046db4417d8efce8ca00b6098ae14732366`.
To satisfy the governed gate without deleting retained data or restarting
protected services, the two disposable in-memory inference backends were
unloaded and WSL memory was compacted in place. Windows available RAM rose from
3.231 GiB to 7.466 GiB. Direct post-activation browser inspection confirms the
corrected one-column History drawer and the primary premium workspace.

Current state: `PREMIUM_UI=LIVE_VERIFIED`,
`PREMIUM_UI_FINAL_DRAWER=LIVE_VERIFIED`, `UI_ACTIVATION=PASS`,
`RETAINED_DATA_MUTATED=false`, `NX-B2.1D=OPEN`,
`Q4_FULL_PLAN_ROLE=RETIRED`,
`Q4_RESIDUAL_ROLE=INSUFFICIENT_FOR_RESIDUAL_ROLE`,
`MODEL_REPLACEMENT=APPROVAL_REQUIRED`, and `D_ACTIVATION=BLOCKED`.

## 2026-09-07 final UX simplification source validated; final visual correction pending RAM

The team-lead simplification pass is complete in source. The ordinary analyst
hierarchy is now Add data → Ask → Answer → Evidence: the permanent evidence
strip and repeated case context are removed; History is under the workspace
menu; submitted questions are plain conversation text; one-result ANPR answers
use compact facts; natural Urdu remains primary; tables, normalization,
technical labels and detailed provenance are behind disclosure; and the Ask
composer is one-row compact. Focused lint has zero errors, 56 unit assertions
pass with one explicit historical-data skip, all 27 Playwright scenarios pass,
the 687-module production build passes, wrapper tests pass, and all HTTP packages
compile.

The first simplification overlay was safely activated as image
`sha256:0e47f2d689e796cf231e08b33f23e470999d3a80000ef852d49d8c93eb05827d`;
live review then caught that the submitted question still inherited an
input-like rounded chat background. That visual defect is fixed and guarded by
a computed-style Playwright assertion. The final 216-file UI-only context is
sealed with zero production source, zero D markers, and zero D-unqualified
source. Its manifest SHA-256 is
`f272370da31b98163bbfd869d724298cab4bb565a2f6c77c51bb9f3e52628057` and
sealed index SHA-256 is
`1f6d36ee9371d919c97357ed02c7b98b40aa613295e84b2ef819d4d46aac5285`.

Final activation preflight passed with zero active jobs and retained tuple
`66|66|79|0|22509|850|63`, but the required immediate pre-mutation RAM gate
measured 4.859 GiB (< 6 GiB). The script stopped before image tagging, building,
or container recreation; no service or retained state changed in that attempt.
Do not try to manage processes from Codex. After the owner closes applications
manually and at least 6 GiB is available, run exactly:

`powershell -NoProfile -ExecutionPolicy Bypass -File scripts\activate_nexusai_ux_simplification_final_20260907.ps1`

Current state: `UX_SIMPLIFICATION=SOURCE_VALIDATED_ACTIVATION_PENDING_RAM`,
`UI_ACTIVATION=PENDING_RAM`, `NX-B2.1D=OPEN`,
`Q4_FULL_PLAN_ROLE=RETIRED`,
`Q4_RESIDUAL_ROLE=INSUFFICIENT_FOR_RESIDUAL_ROLE`,
`MODEL_REPLACEMENT=APPROVAL_REQUIRED`, and `D_ACTIVATION=BLOCKED`.

## 2026-09-07 unified Investigation Workspace activated and live-verified

The ordinary analyst flow is now one canonical `/analyst` workspace: Add
evidence → Ask → Answer → Verify evidence. Home/Data/Ask/Activity compatibility
routes resolve to unified states; evidence and History are secondary drawers;
the real governed intake and query/citation contracts remain unchanged. Focused
lint passes, 45 analyst unit tests pass with one explicit fixture skip, all 25
Playwright scenarios pass, the 686-module production build passes, and a
live-backed preview passes 390/820/1024/1440 responsive, accessibility-isolation
and clean-console checks. A 216-file sealed UI-only context was bound to accepted
live image `sha256:feb935b42a93c1810394cb270a809189ca240a225253159a47b8daa72a799171`;
it contains zero production source, zero files from the frozen D manifest as
source, and zero D markers. The standalone activation script passed its
no-mutation preflight with active jobs zero and recreated only the API container
on UI overlay image
`sha256:5e056478ae0f1823cfe0fc453f1c26a60c2eb86cc173958c6537fc064b0420d9`.
The protected forensic API, worker, NATS and PostgreSQL container/image
identities were preserved; the retained tuple remained
`66|66|79|0|22509|850|63`; no migration, evidence write, model change or D
activation occurred. Live `/analyst` serves the sealed index, the API is
healthy, its backend acceptance passed, and a final real-browser pass confirmed
the canonical workspace, evidence and History drawers, actual Add data input,
modal isolation, clean console, and no horizontal overflow at
390/820/1024/1440. Activation receipt SHA-256:
`d386e78a651f4b658835970d71c034963d3badf3c3814eac9b987b71a42199c3`.
Program state: `UNIFIED_ANALYST_UX=LIVE_VERIFIED`,
`UNIFIED_UI_ACTIVATION=PASS`, `INVESTIGATION_WORKSPACE_UI=ACTIVE`,
`NX-B2.1D=OPEN`,
`Q4_FULL_PLAN_ROLE=RETIRED`,
`Q4_RESIDUAL_ROLE=INSUFFICIENT_FOR_RESIDUAL_ROLE`,
`MODEL_REPLACEMENT=APPROVAL_REQUIRED`, and `D_ACTIVATION=BLOCKED`.

## 2026-09-07 NX-B2.1D Decision8 adjudicated; replacement-model approval required

Decision8 is complete and immutable: 8/8 cases completed, 5/8 passed, eight model calls, runtime postcheck PASS, and receipt SHA-256 `0680fe8c706f2eda40b1352a22103a03d28f0e395f6b6835790e7d174dffce95`. The three failures are `dec8-ur-clarify=MODEL_CLARIFICATION_WEAKNESS`, `dec8-roman_ur-resolved=MODEL_MULTILINGUAL_INTENT_WEAKNESS`, and `dec8-mixed-resolved=MODEL_MULTILINGUAL_INTENT_WEAKNESS`; contract, transport, candidates, and admitted oracles were valid. Q4_FULL_PLAN_ROLE=RETIRED; Q4_RESIDUAL_ROLE=INSUFFICIENT_FOR_RESIDUAL_ROLE. No installed candidate is suitable for another development benchmark. The selected acquisition candidate is the exact pinned official `Qwen3-8B-Q4_K_M.gguf` artifact (5,027,783,488 bytes; expected SHA-256 `d98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785`) at revision `7c41481f57cb95916b40956ab2f0b139b296d974`; download requires explicit owner approval. FINAL_DEVELOPMENT=NOT_STARTED_APPROVAL_REQUIRED; FINAL_D_CANDIDATE=NOT_YET; FINAL_HOLDOUT=NOT_CREATED; FINAL_QUALIFICATION=NOT_STARTED; D_STATUS=OPEN; ACTIVATION=BLOCKED. Do not rerun consumed corpora or include the compile-safe but incomplete Ask UI edits in a D image.

Full adjudication and exact next action: `reports/nxb21/nexusai-nxb21-d-decision8-adjudication-and-model-selection-20260907.md`.

## 2026-09-06 NX-B2.1D one-field residual-only 8-case development frozen

Current preparation state: RESIDUAL_V2_ONE_FIELD_DECISION=SOURCE_VALIDATED; RESIDUAL_ONLY_8CASE_GATE=FROZEN_NOT_EXECUTED. Fresh gate has 8 expected residual calls, 2/language, 4 resolved across network/CDR/logs/knowledge and 4 clarification (2 ambiguous, 2 insufficient). Corpus SHA `7bf616e0c146128dcf4fd9e4d470512ae49f48d3e8feb7497f1ba1ccdda9fbe3`. Freeze `nxb21d-q4-hybrid-decision-dev-r1`; identity `2467e8a85b0517fcfd5f7580825ddc55611db20ba4bdec073692c6d6facea4e3`. Freshness PASS against 590 historical files, zero question/value overlap. Full API PASS (527 passed, 31 skipped; 36.994 s); focused, schema conversion/enum, rejection, receipt, transport, guards and ValidateOnly PASS. No live inference. HYBRID_ATTEMPT_1=INCOMPLETE_FAILURE_RETIRED; RESIDUAL_V1_CONTRACT=RETIRED; old 32-case lock and evidence preserved. Q4_FULL_PLAN_ROLE=RETIRED; Q4_HYBRID_ROLE=HYBRID_PROMISING; FINAL_REPLACEMENT_QUALIFICATION_CANDIDATE=NOT_YET; REPLACEMENT_HOLDOUT=NOT_CREATED; D_STATUS=OPEN; ACTIVATION=BLOCKED.

Full handoff: `reports/nxb21/nexusai-nxb21-d-decision8-development-freeze-handoff-20260906.md`. New development freeze receipt: `reports/nxb21/d-decision8-development-freeze-v1.json`. Only the new standalone command is prepared; no gate was dispatched in Codex.


## 2026-09-06 NX-B2.1D first hybrid live attempt incomplete; residual and receipt source corrections

Supersedes the frozen-not-executed status below. HYBRID_DEVELOPMENT_ATTEMPT_1=INCOMPLETE_FAILURE; CASES_COMPLETED=5; DETERMINISTIC_PASS=4_OF_4; FIRST_RESIDUAL_RESULT=MALFORMED_RESIDUAL; RUNTIME_INTEGRITY=PASS. Current receipt and sidecar: `38684c2f1e9b1cb128ded3693c1778498480034eec560daf34c0e37fa06f36be`. Receipt post-hash rewrite reproduced and fixed in source. Residual schema allowed contradictory state fields; replaced with exclusive decision enum and server-derived audit state. Full API source tests PASS (42.242 s); receipt immutability tests PASS. All original run evidence and dispatch lock preserved. CURRENT_32CASE_CORPUS=RETIRED_AFTER_DISPATCH; no rerun/resume permitted. Q4_FULL_PLAN_ROLE=RETIRED; Q4_HYBRID_ROLE=HYBRID_PROMISING; DETERMINISTIC_FIRST_HYBRID=SOURCE_VALIDATED. Fresh 8-case residual gate recommended but NOT_CREATED; no new inference performed here. FINAL_REPLACEMENT_QUALIFICATION_CANDIDATE=NOT_YET; REPLACEMENT_HOLDOUT=NOT_CREATED; D_STATUS=OPEN; ACTIVATION=BLOCKED.

Incident evidence and exact next action: `reports/nxb21/nexusai-nxb21-d-hybrid-attempt-1-incident-20260906.md`; lossless diagnosis: `reports/nxb21/d-hybrid-attempt-1-diagnosis.json`. Historical freeze remains immutable and no longer matches corrected source.


## 2026-09-06 NX-B2.1D deterministic-first hybrid source validated; development frozen

Current status supersedes the earlier interface-development checkpoint below. Q4_FULL_PLAN_ROLE=RETIRED; Q4_HYBRID_ROLE=HYBRID_PROMISING; DETERMINISTIC_FIRST_HYBRID=SOURCE_VALIDATED; HYBRID_DEVELOPMENT_GATE=FROZEN_NOT_EXECUTED. The unchanged fresh corpus has 32 cases (8/language; 26 deterministic, 6 residual), SHA256 `caec70a2bbd5023358afffe24c00a51805b7539f5eb1c77c0e827325194407c8`. Freeze `nxb21d-q4-deterministic-first-hybrid-dev-r1`, identity `85566bcfcf0681fed5df649d442d25b975106e03e9d44476f88e85776547bad2`. Full API source tests PASS (526 Ginkgo passed, 29 skipped; 40.333 s); oracle admission, relevant routing, transport and guards PASS. DB/Testcontainers-dependent agent checks remain environment-blocked. No inference, replacement holdout, activation or runtime mutation. FINAL_REPLACEMENT_QUALIFICATION_CANDIDATE=NOT_YET; REPLACEMENT_HOLDOUT=NOT_CREATED; D_STATUS=OPEN; ACTIVATION=BLOCKED.

Handoff: [reports/nxb21/nexusai-nxb21-d-hybrid-source-freeze-handoff-v1-20260906.md](reports/nxb21/nexusai-nxb21-d-hybrid-source-freeze-handoff-v1-20260906.md); immutable development freeze receipt: `reports/nxb21/d-hybrid-development-freeze-v1.json`. Acceptance is frozen before operator execution. Existing evidence and consumed holdouts remain preserved.


## 2026-09-05 NX-B2.1D interface development attempt 1 retired; schema-bound v2 frozen

The development-only run `run-20260905T161347181Z` ended after four of 24
cases when the PowerShell pipeline stopped. No aggregate result exists. Its
partial checkpoint and exact request/response evidence are preserved by
`reports/nxb21/d-q4-production-interface-development-attempt-1-incomplete-v1.json`
at SHA-256 `e37b222a43487e13e2e1c3b69e23448446ee297d89520ded0bf381117740b110`.
The v1 corpus is retired and must not be resumed or rerun.

All four observed calls returned HTTP 200, strict UTF-8 JSON, parseable planner
objects and `finish_reason=stop` at 174-266 of 512 completion tokens. This
proves the completion-budget remediation for the observed sample. The printed
`SCHEMA_INVALID` classification was inaccurate: the responses were
schema-shaped but rejected by the production validator. The schema permitted
unused `start_seconds` and `end_seconds` as numeric zero although validation
treats every non-null number as an asserted recording bound. It also permitted
`text_query` on non-text capabilities. Separately, top-k validation trusted
only digits despite an explicit word-number request, and the v1 English exact
phrase oracle contradicted the shared deterministic parser.

The production schema now binds literal, top-k, calendar dates, source time,
direction and nested text fields to deterministic request-derived constraints.
Unused source time is forced to JSON null; non-text requests cannot emit
`text_query`; explicit quoted text is bound to the exact source literal and
shared semantic. Bounded English, Roman Urdu and Urdu top-k word forms are
server-derived. Focused source tests pass, all 24 fresh deterministic corpus
oracles pass, and the replacement framework passes 70 assertions. The full
forensic Ginkgo suite passed 485 specs with 27 expected skips; the concurrent
package command's separate timing test measured 2.667 seconds under three
parallel Go jobs, then passed alone in 0.601 package seconds.

Replacement development freeze
`nxb21d-q4-production-interface-schema-bound-dev-r2` has identity SHA-256
`ae5ff08e090a4c23b10f6dca2d1e6a8c4549ab13059fc51dcf34d70926b0c4db`.
Its 24-case corpus SHA-256 is
`3eb69ae6284e38cfc4e3f4c68514d0ad2ef5ed5081908faf389fc4a60122dbb0`;
validation proves four languages, 20 model cases, four deterministic critical
cases, and no overlap with seven earlier corpora or 87 captured probes. The
runner stops after four consecutive model failures while still sealing a
receipt and unloading Q4. No live inference, deployment, retained write,
download, model/profile change or container recreation was performed by Codex.
Q4 is unloaded. D remains OPEN, Q4 suitability is UNCLASSIFIED, replacement
qualification is not frozen, and activation remains BLOCKED. Only the v2
standalone development runner may execute next.


## 2026-09-05 NX-B2.1D Q4 development passed; independent qualification frozen

The resumed explicit-scope development run `run-20260905T081643297Z` completed
all 16 cases. Raw model plans and final typed plans both passed 16/16 with zero
deterministic corrections; transport, schema, UTF-8 and runtime-integrity gates
passed. The immutable receipt SHA-256 is
`921229a78719a5594271b4ede2fd5aadabc6f3a52d50c3893e4dd5f5badfafe1`.

The new independent Q4 qualification candidate is
`nxb21d-q4-final-qualification-r1`, identity SHA-256
`11c7ce2a5c0ed1d6bb83a8d2a9b26274dbcedf6c96b014a4c7e0404160c26371`.
Its 168 unique questions cover all 22 capabilities with English 65, Urdu 37,
Roman Urdu 36 and mixed 30; 80 cases are critical. Corpus SHA-256 is
`aaa8517983ea4523e235c4b3de31e5562bfaeada21ef24a7b23065a567d31d13`.
Validation proves UTF-8 without BOM, exact distributions, zero prior-question
overlap and zero prior-oracle-value overlap. The deterministic source oracle
passes 168/168 and the sealed qualification/activation framework passes 537
assertions.

The final preparation-only run `qualification-20260905T152339897Z` passed the
frozen Docker/model/database baseline without loading or unloading a model and
without consuming the holdout. Its receipt SHA-256 is
`01d28017aab8755147cd848dc4c1546c9e12f59b68a5810a1f024246406592cd`.
D remains OPEN until the one-shot qualification produces a reviewed SUITABLE
receipt. E is not accepted and activation remains blocked pending that receipt
and explicit owner approval. A Q4-specific activation and rollback pair is
sealed but cannot pass its gate until a suitable receipt and source seal exist.
Never use the consumed Q8 holdout or any completed development corpus as
qualification evidence.

## 2026-09-04 NX-B2.1D offline preflight repaired — holdout still unconsumed

The third standalone invocation passed the 7.37 GiB RAM gate and stopped during
read-only baseline SQL because the bundle queried nonexistent ingest enum value
`retrying`. The query now uses the repository-authoritative active predicate
`status NOT IN ('completed','dead_letter')`. It stopped before contract freeze,
Qwen load, inference, or holdout access; the 168-case holdout remains unconsumed.

The fourth invocation passed the 7.321 GiB initial gate and froze the contract,
then artifact hashing populated WSL clean file cache and the immediate pre-load
gate stopped at 3.41 GiB. It stopped before `HOLDOUT_CONSUMPTION=BEGIN`, Qwen
load, or inference. The evaluator now applies the repository's guarded cache
recovery after artifact verification.

The fifth invocation passed the 8.099 GiB initial gate, but harmless Docker
background writes kept Dirty at 4–56 KiB while Writeback stayed zero. The overly
strict zero-Dirty condition skipped recovery and the pre-load gate stopped at
4.228 GiB, again before holdout consumption or model load. Recovery now follows
kernel semantics: with Writeback zero it drops reclaimable clean cache, preserves
dirty pages, performs no `sync`, waits up to 60 seconds for Windows RAM release,
and then enforces the unchanged 6 GiB gate.

The final end-to-end preparation dry run passed at `2026-09-04T05:36:04Z`:
initial RAM 6.411 GiB, exact baseline and 4.28 GB artifact verification passed,
guarded recovery ran with Dirty 8 KiB and Writeback zero, and Windows RAM moved
from a transient 2.106 GiB minimum to 6.166 GiB before the identical immediate
pre-load gate passed. It loaded no model and did not access the holdout. A
Docker/Windows restart is not required.

The complete evaluation/activation/rollback path was re-audited. Baseline and
postchecks now cover all five HTTP services; activation proves both changed
containers were recreated on the newly built image IDs; activation and rollback
preserve protected identities, retained tuple, Activity count, zero jobs, and
unloaded Qwen state. Rollback validates the restored image IDs without requiring
impossible preservation of recreated container IDs. Twenty-six preparation tests
pass. D remains `SOURCE_IMPLEMENTED_MODEL_EVALUATION_PENDING`; activation remains
separate and gated on a reviewed `SUITABLE` D receipt plus owner approval.

## 2026-09-03 NX-B2.1D offline operator bundle prepared — no heavy execution

The self-contained PowerShell bundle is prepared under `scripts/nxb21d-offline/`.
It separates the one-shot Qwen evaluation from a future owner-approved B1+C+D
activation. Both phases enforce a 6 GiB initial gate and an immediate pre-mutation
recheck. Preparation loaded no model, performed no inference, build, recreation,
deployment, retained write or migration. The 168-case holdout remains unconsumed.

The first offline invocation passed the 7.231 GiB RAM gate and stopped before Docker/model access because Windows PowerShell culture ordering did not reproduce the Python aggregate source digest. Ordinal key sorting now matches the frozen digest, and a real-manifest regression test passes. Qwen was not loaded and the holdout remains unconsumed.

A second invocation passed the 7.366 GiB RAM gate and stopped at baseline inspection because strict mode assumed every running container exposed a Docker Health object. Health validation now accepts running containers without a configured healthcheck while still rejecting unhealthy, stopped or OOM states; both shapes have regression coverage.

The source-bound D digest remains `34c69067628a58373d471aa8e75d7a17a2c4a7f789b521f62c5baa38f8a5478d`;
the operator and reporting files are outside its 460 bound members. Eighteen
preparation tests pass, including PowerShell syntax, evaluator compilation,
RAM/source/receipt/critical-gate refusal, exact LocalAI unload path, durable logs
and stable exit codes. All 22 capabilities and all 79 operations are reconciled
in practical runbooks. Worker activation is not required. D remains
`SOURCE_IMPLEMENTED_MODEL_EVALUATION_PENDING` until the offline one-shot run.

## 2026-09-03 NX-B2.1D pre-model gate — source only

D source contract is implemented but D exit has NOT passed. C digest verified.
The model-facing vocabulary is 22 C references; the server derives operations.
All 79 operations are mapped for E/F; all 214 historical variants are classified
COMPATIBLE_PLAN_PARITY. The independent 168-case corpus is frozen before model
evaluation. Source validator, APF compatibility and integration gates pass.

Current qwen_qwen3-4b-instruct-2507 is installed (Q8_0, 4,280,405,216 bytes,
llama-cpp CPU) but not loaded. Actual English/Urdu/Roman/mixed accuracy, critical
safety, latency and memory remain NOT_MEASURED; suitability is not classifiable.
Only 3,372,187,648 bytes host RAM were free at inspection. Per the owner gate,
do not load it or mutate model state without a separate decision.

No deployment, activation rerun, retained query/write, migration, model/volume
change, certification or Git publication. Receipt 20260903T063719719Z remains.
D digest: 34c69067628a58373d471aa8e75d7a17a2c4a7f789b521f62c5baa38f8a5478d. Review: reports/nxb21/nexusai-nxb21-d-dynamic-planner-and-operation-verification-alignment-v1-20260903.md
Next: remain in D and resolve the controlled current-model evaluation gate.


## 2026-09-03 NX-B2.1C source closure — not deployed

C is SOURCE_VALIDATED_NOT_DEPLOYED: 22 thin references, unchanged 79 executable
queries / 104 descriptors / 214 variants, 29 catalog-only dispositions and 159
category/format boundaries. Final C gate: 74 passed, including 14 isolated DB
cases. Full forensic and agent regressions pass; public API/auth checks pass.
B1 frozen explicit-semantic replay remains 14/14. Source P0/C P1=0/0.

TTS false readiness is fixed in source. Retained-query and fresh-processing
readiness are separate; missing processor receipts fail closed. Scope, current
version, partial/zero/failure and image-ANPR authority boundaries are tested.
The retained read-only projection is 134.969ms after transaction-local JIT
suppression; its two-second bound is preserved. No global DB setting changed.

Receipt 20260903T063719719Z, five runtime identities/images/restarts and installed
models remain unchanged; active jobs=0. Final retained tuple is
64|64|77|22507|828|61 versus starting 63|63|76|22507|827|61. One TXT ingestion
created at 11:56:30Z added evidence/version/job/artifact; origin is unverified.
C performed no retained write; the added document was left untouched. Live still
has the old B1/C defects. C performed no deployment, retained mutation, migration, model or
volume change, certification promotion, Git publication or activation rerun.
Test fixtures, role and empty isolated database were removed.

C source-review digest (443 files): 9985d71d894631e6c730c6c7ffa9caf85215427ca076e12e8bb0edc545a91c02
This is not a deployable seal. The used activation seal remains unchanged.
Future minimum services remain api + forensic-records-api; worker/DB/NATS stay
outside that scope. Keep the 6 GiB future activation gate. No bundle prepared.

Option 1 is supported: B+C may remain source-only before D; no live dependency
blocks D source tests. D is NOT_STARTED and must retain source/live separation.
Next: explicitly select D planner robustness; E/F certification and G polish
remain pending. Review: reports/nxb21/nexusai-nxb21-c-query-capability-readiness-closure-v1-20260903.md

## 2026-09-03 NX-B2.1B-1 source closure — not deployed

B-1 shared literal semantics, pre-routing extraction, bounded adjacent-segment
matching, scoped result states and paginated-search completeness are SOURCE
VALIDATED. Forty required matrix cells pass source/DB/artifact proof; the frozen
14-case corpus passes explicit PHRASE_CONTAINS with unchanged literals. Legacy
exact retains its historical policy. One workspace artifact replay truthfully
remains SEARCH_INCOMPLETE because some adjacency is unproven.

Full forensic package: 391 Ginkgo passed, five unrelated optional/environment
skips; final B-1/DB/artifact/video gate: 63 passed. Full agent package: 145 passed,
one optional bridge skip; that bridge subsequently passed in a ten-spec run.
Two registry generators remain deliberately disabled. Synthetic DB fixtures,
test role and the empty isolated database were removed. No retained writes.

Receipt 20260903T063719719Z, all five container IDs/images/restarts, health,
installed models and tuple 63|63|76|22507|827|61 remain unchanged; active jobs=0.
The accepted 428-file seal is historical: 11 members intentionally changed.
A new 435-file source-review digest is recorded in b1-source-diff-manifest-v1.json;
it is not an activation seal or deployable operator bundle.

Source P0/B-1 P1=0/0. B source exit passes; no evidenced B-2/B-3 work is needed.
Deployed B-1 defects remain pending a future approved activation. B2.1C is NOT
started; its exact next source scope is capability references/readiness, including
NXB21-P1-004 TTS availability. P2 coverage and disclosure work remains C/E/F/G.
Review this closure before selecting the next phase or a separate activation.
Future minimum services: api and forensic-records-api; worker/DB/NATS excluded.
Keep the 6 GiB future activation gate. No deployment, migration, model/volume
change, certification promotion, Git publication or used-activation rerun.

Report: `reports/nxb21/nexusai-nxb21-b1-shared-derived-text-semantics-closure-v1-20260903.md`.
Earlier B-source-next and pending acceptance notes below are historical.


## 2026-09-03 NX-B2.1A reconciliation complete — B2.1B source slice next

Latest owner continuation authorizes NX-B2.1 source/query productization. The
sole vNext roadmap/ledger are updated; NX-MMR bounded closure stays accepted.
Receipt `20260903T063719719Z` and five container identities/images/restarts match,
health PASS, jobs0; tuple `63|63|76|22507|827|61` unchanged. Do not rerun activation.
The old receipt's live-replay PENDING field predates the final PASS overlay.

Current truth: 79 executable queries live/source, 104 discovery descriptors,
214 source variants (12 duplicates), 80 live family-corpus entries versus ten
static originals; nine adapter manifests, 16 agent manifests, six running agents.
Canonical certification unchanged: REGISTERED62/SOURCE12/FIXTURE1/PRODUCT4.
Live OCR uses Paddle; ASR is faster-whisper-small-ur; only Qwen embedding was
loaded at inspection. TTS disabled; discovery's queryable TTS category is a
new B2.1 readiness P1 caused by extension-based counting, not synthesis support.

Four grouped B2.1 P1 areas: literal-mode/default and token/substring semantics;
cross-segment ASR search; whole-workspace transcript result state; misleading
family readiness. Fourteen read-only public-query diagnostics: eight contract
passes, six gap observations. Urdu/English touching-segment false negatives have
independent DB proof of current version, same run and literal text occurrence.
Within-segment Urdu/English/Roman and sampled OCR searches pass. Read-only Activity
inspection then recovered two short quoted Urdu failures (no operation selected,
~60s) and a full-transcript success. Two fresh no-synthesis probes confirm the
short prompt fails routing while its typed literal matches. Original browser
selection is unrecorded; routing is the primary recovered incident mechanism,
cross-segment matching a separate gap. Nine focused Ginkgo specs and
selected legacy ledger/contract tests PASS. No fresh browser/Activity acceptance.

Report: `reports/nxb21/nexusai-nxb21-all-family-query-productization-reconciliation-v1-20260903.md`.
Adjacent JSON holds full inventories, sanitized probes and 25 capability designs
with the 20-family gap matrix. Raw snapshots/targets/responses/tests are ignored
under `local-acceptance-models/nxb21-reconciliation/`.

Next: implement NX-B2.1B-1 shared derived-text semantic/result-state contract and
independent tests under existing source authority. No production source changed
in A; no build/deployment, DB migration, upload/reprocessing, model change,
volume change, staging/commit or certification. Source correction must precede
a new source seal/operator bundle and separate deployment authority; keep 6 GiB.

## 2026-09-03 final video/Activity acceptance PASS — bounded P1 closure

Receipt `20260903T063719719Z` is deployed VERIFIED and its later failed-cell
replay now PASS WITH LIMITATIONS. Four fresh Ask results verified exact raw
MM51VSU (2 rows, frames60/120 at1s/2s), absent QZ99XYZ (0), raw MW51VSU (5),
and grouped MW51VSU (0–2.25s, sightings5). Ask and Activity preserve typed rows,
raw/group distinction and citations; source clicks seek correctly. Historical
null-column entry safely reports unavailable details without rewriting history.
Open replay-scope P0/P1=0/0; broader certification is not implied.
Health/models PASS, jobs0, tuple `63|63|76|22507|827|61`, all container identities,
images/restarts and 428-file seal unchanged. No rebuild, cleanup, unload,
reprocessing or deployment; four ordinary Ask entries added. RAM3.493→3.114GiB
with no replay failure; 6GiB build gate unchanged. DO NOT RERUN used activation.
Authority: final overlay in
`reports/nexusai-nxmmr-post-activation-live-p1-acceptance-v1-20260902.md`.
Next: NX-B2.1 scope/preflight planning, with separate deployment/runtime approval
before activation. TTS, broader accuracy/certification and P2 polish remain
limited/deferred. Older pending/failure statements below are historical.

## 2026-09-03 video-result activation VERIFIED — failed-cell replay next

User completed the sealed operator: receipt `20260903T063719719Z` is VERIFIED,
activation_verification PASS, live_failed_cells_acceptance PENDING. Read-only
receipt/live checks independently confirm both expected candidate images,
health, zero jobs, required installed models, protected identities/restarts
and retained tuple `63|63|76|22507|827|61`. Source seal unchanged:
`0b11a1b6cf1b27dc890dd17ceb306cbc255a00ef3f7ba16b5e216d293fcf3127`.
Current API container `d62021a0a4fc258ff3ce6fc4140090c6d9862dcc04863a065e6e842f62d769de`,
image `sha256:feb935b42a93c1810394cb270a809189ca240a225253159a47b8daa72a799171`.
Forensic API container `ffd93bf51ecad0bc61271dfad0576b852057cd85f85aa8221534e3250ef1c11b`,
image `sha256:828e4b03e02979c3fc25dd6b6896e74fb97a2e59b8c17300a3efadce1236ace8`.
Both restart counts 0. Worker remains 9, PostgreSQL/NATS 0, identities unchanged.

RAM recovery passed the 6 GiB checkpoints; both images built sequentially and
only the two admitted services were recreated. No guarded drain was needed in
the supplied log. Compose orphan warning names intentionally excluded services;
never apply --remove-orphans to clean up the protected worker/database/broker.
DO NOT RERUN the now-used activation bundle or rebaseline it to these new IDs.
No deployment or evidence writes by this verification turn; documentation only.
Next: bounded video positive/no-match/group/time/frame/source-citation and
new/historical Activity reopen acceptance. Two live P1s remain unclosed until
that replay passes; no broader product certification or NX-B2 promotion yet.

## 2026-09-03 post-reboot baseline reconciled and resealed — operator handoff

User approved rebasing only the verified post-reboot starting baseline and wants
to close optional apps before running the operator. IDs/images remain identical
to receipt `20260903T050747951Z`; API startup retries were logged while NATS/DNS
and PostgreSQL were not ready, and worker retries while PostgreSQL DNS failed.
Stable restart counters now api=0, forensic-records-api=8, worker=9, postgres=0,
nats=0. Manifest records the prior 0/2 counters and explicit reconciliation
`post-reboot-20260903T063334Z`. Exact checks remain; future drift is rejected.

Only manifest/audit and two operator selftests changed among sealed files;
analytical source unchanged. 52 operator checks PASS; 428-file new seal SHA256
`0b11a1b6cf1b27dc890dd17ceb306cbc255a00ef3f7ba16b5e216d293fcf3127`.
At 06:35:00Z preflight is RAM-only BLOCKED (4.728/6 GiB), no errors; health,
models, rollback, Compose, integrity PASS; jobs 0, retained tuple unchanged.
Only embedding model is loaded after the user's reboot; no model load/unload,
app termination, cleanup, build, deployment or service mutation in this phase.

Next: save work, close optional apps including this agent app, keep Docker and
PowerShell running; do not reboot/restart Docker again before this activation.
Run scripts/run_nxmmr_video_result_activation.ps1 -WaitForRamMinutes 120 and
confirm YES. It waits for 6 GiB, builds sequentially, preserves completed
source-bound images, guards post-build drain and verifies/rolls back. RAM success
is not guaranteed; no gate bypass or file pruning. Return the final PASS/receipt
or blocker for remaining-video/Activity replay. NX-B2/live P1 status unchanged.

## 2026-09-03 requested model reload and guarded cache cleanup — RAM blocked

Supersedes the unloaded-model state below: user explicitly requested both models
reloaded and unused WSL cache cleanup. `qwen_qwen3-4b-instruct-2507` passed a
one-token synthetic local chat request; `qwen3-embedding-0.6b` passed one synthetic
embedding request (1024 dimensions). Both are verified in /system loaded_models.
No evidence/Activity query, model download, deletion or configuration edit.
Windows free RAM had reached 6.473 GiB before reload; afterward 1.596–1.695 GiB.

Two bounded attempts of the existing clean-page-cache helper were SKIPPED:
Dirty stayed nonzero (12–88 KiB observed), Writeback zero. No cache drop occurred,
no sync/guard bypass, no service stop/recreation or activation. At 06:23:18Z
preflight was RAM-only BLOCKED (1.596/6 GiB), all other checks PASS; jobs zero,
installed models present, identities/images/restarts and retained tuple
`63|63|76|22507|827|61` unchanged. Last RAM sample 1.695 GiB. Keep both models
loaded per user's direction; do not repeat unloading or claim cleanup succeeded.
Source candidate remains sealed; live P1s and next-phase gate unchanged.

## 2026-09-03 authorized idle-model unload — activation still RAM-blocked

User approved unloading idle LocalAI backends only. The instruction model
`qwen_qwen3-4b-instruct-2507` and embedding model `qwen3-embedding-0.6b` were
unloaded via POST /backend/shutdown (both HTTP 200), using the server's busy-wait
guard with forced shutdown disabled and analytical jobs zero. Backend monitor
RPC was unsupported, so it was not treated as proof of idleness. The shutdown
guard supplies the idle boundary. Loaded-model list is now empty; installed
models remain present. No files/models deleted, containers stopped/recreated,
cache dropped, configuration changed, or activation started in this operation.

At 06:18:37Z read-only preflight remains RAM-only BLOCKED: Windows available
2.488 GiB / required 6 GiB. WSL reports about 6.16 GiB MemAvailable and 4.55 GiB
Cached, which is not the same as Windows free RAM. Health, model inventory,
source seal, rollback, Compose and jobs PASS; all starting service identities/
images/restart counts and retained tuple `63|63|76|22507|827|61` preserved.
Next requires Windows-visible memory recovery; clean WSL page-cache reclamation
was not included in this unload-only approval. Do not lower the RAM gate or
claim activation/P1 live acceptance complete. The 428-file candidate is unchanged.

## 2026-09-03 video/Activity two-P1 source correction — SEALED, NOT DEPLOYED

NX-MMR-VIDEO-RESULT-CONTRACT-AND-ACTIVITY-P1-CLOSURE-V1 is source-tested and
sealed across 428 files. Video raw/group rows retain plate, source time/frame,
source file and per-artifact citations through enterprise/public/tool/agent
presentation. Activity renders null/malformed historical tables without crashing
or inventing plate facts. Focused Go tests/vet, 45 frontend tests and 50 operator
checks PASS. See reports/nexusai-nxmmr-video-result-contract-activity-p1-closure-v1-20260903.md.

Current runtime remains receipt `20260903T050747951Z`; five identities/images/
restart counts and tuple `63|63|76|22507|827|61` unchanged, jobs 0, health PASS.
New read-only preflight: RAM-only BLOCKED (1.415 GiB / 6 GiB at 06:01:49Z).
No build, deployment, restart, cache cleanup, migration or retained writes.
New manifest/seal: configuration/nxmmr_video_result_activation_v1.json and
configuration/nxmmr_video_result_operator_integrity_v1.json.
Future runner: scripts/run_nxmmr_video_result_activation.ps1 -WaitForRamMinutes 120.
Only api + forensic-records-api may later change; worker/DB/NATS protected.
STOP here. Separate deployment authority and failed-cell replay are next.
The two live P1s below remain open pending that replay; NX-B2 remains paused.
Air-gapped build readiness is not claimed; required build-dependency networking
is retained, while runtime pulls/model downloads remain prohibited.

## 2026-09-03 four-P1 activation/replay — current handoff, NOT READY

Activation `20260903T050747951Z` is VERIFIED/PASS; do not rerun the used bundle.
Six fresh bounded browser queries now pass OCR positive/negative and image/face
Ask ranking, friendly filenames, source opening and Activity retention. Video
SQL completes (2 raw readings / 0 absent matches), but enterprise flattening
loses plate/time/provenance fields: four bbox rows, null columns, no citations.
Reopening that fresh video result crashes Activity at `columns.map`. Two P1
defects remain in this replay scope; full demo and next-phase gate NOT READY.
Historical empty completions now render as failures without rewriting history;
fresh failure/late-event injection remains source-tested, not live-injected.
Data off-page candidate labels and Ask Target/source labels retain P2 gaps.

No source/runtime/evidence mutation in this replay; only six ordinary retained
Ask entries plus documentation. Six additional concurrent entries were observed
at the final 278 -> 290 history snapshot and are not this replay's writes.
Health PASS, jobs 0, tuple `63|63|76|22507|827|61`; all five identities/images/
restart counts unchanged from the newly activated baseline (worker 2 preexisting).
The existing 415-file source seal is unchanged. These are not RAM failures.

Next: bounded video enterprise projection/provenance, negative target wording
and null-safe Activity correction with complete-contract/render tests; then
separate new sealed deployment authorization and remaining-failure replay only.
No broad rebuild, cleanup, model search, NX-B2 activation or TTS promotion.
Authoritative evidence/IDs: four-P1 overlay in
`reports/nexusai-nxmmr-post-activation-live-p1-acceptance-v1-20260902.md`.
All older current-status/activation instructions below are historical.

## 2026-09-03 NX-MMR four-P1 source correction — sealed, STOP before deployment

`NX-MMR-FOUR-P1-SOFTWARE-DEFECT-CLOSURE-V1`: the four bounded corrections
passed source/DB-backed validation. Timescale 2.28.3 with the restricted RLS
role reproduced `DISTINCT artifact_type` selecting SkipScan over a policy
`Result`; equivalent `GROUP BY` executes without server configuration changes.
Exact raw video plate readings preserve the different group-selected candidate.
Failed/empty operations cannot become completed, including late terminal events;
valid zero matches and clarification retain separate outcomes. OCR reads actual
`observation.raw_text` with normalized-text fallback. Similarity Ask/Activity
prioritize rank, score and authorized source filenames with safe citation labels.

New separate bundle: `scripts/run_nxmmr_four_p1_activation.ps1`, manifest
`configuration/nxmmr_four_p1_activation_v1.json`, 415-file integrity seal
`configuration/nxmmr_four_p1_operator_integrity_v1.json` (SHA-256
`2d20b3cce2ebc581d884f0430cff1069861dc36884019d622f1cff6969d75340`).
50 operator self-tests pass. Read-only preflight passes every non-RAM gate;
4.126 GiB available after test cleanup versus the unchanged 6 GiB requirement. No build, deployment,
live replay, retained mutation, migration, model download or NX-B2 activation.
Disposable Timescale fixtures were removed; no retained evidence was deleted.

Live truth remains R2 receipt `20260902T154901947Z`, PARTIAL/NOT READY,
four live P1 areas pending deployment/replay. Five service identities, images and
restart counts remain unchanged; tuple `63|63|76|22507|827|61`, active jobs 0.
Next: separately authorize one later new-bundle activation (api + forensic API
only), then replay only the four failed areas. Do not rerun the used R2 bundle.
Source report: `reports/nexusai-nxmmr-four-p1-software-defect-closure-v1-20260903.md`.

## 2026-09-02 NX-MMR R2 failed-cell browser replay — current handoff

The authorized live replay is complete, but the acceptance gate is **PARTIAL / NOT
READY**, not a next-phase pass. Activation receipt `20260902T154901947Z` remains
VERIFIED/PASS and must not be rerun. No source/service/model/evidence mutation
was performed during this replay; only expected Ask/Activity entries were added.

Fresh browser passes: real Urdu-script transcript; Urdu and English timing
anaphora (0–10 seconds); source citation opens the correct 10.5-second audio and
resets a manually scrubbed 4.979461-second position to zero; explicit plate intent
does not repeat the selected audio transcript; phone/plate composition retains
both CDR and ANPR citations within the 20-card limit, including Activity reopen.
Image/face Ask now execute ranked candidate searches (10/3), with no identity
claim, and a candidate citation opens the correct retained image.

Four P1 areas remain: (1) selected-video exact/no-match execution, (2) missing
video failure/answer status in Ask/Activity, (3) selected-image OCR false negative,
and (4) image/face analyst presentation. Video diagnostic query returns HTTP 400
`unsupported subplan type for SkipScan: Result (SQLSTATE XX000)` while its agent
history incorrectly says completed with no answer. The OCR SQL projects neither
`observation.raw_text` nor `observation.normalized_text`; a read-only database
oracle confirms `Investigation Workspace` exists but that projection is NULL.
Similarity scores/ranks exist but default Ask columns hide them behind UUIDs;
Activity renders raw URIs and `[object Object]`. These are not RAM failures.

Final retained tuple is unchanged `63|63|76|22507|827|61`, active jobs zero,
all five container/image identities and restart counts unchanged. API is healthy;
forensic API is running with zero restarts. Free host RAM was 4.950 GiB with
Codex/browser open; no build or cleanup was attempted. Current acceptance-case
inventory is 36 sources / 35 ready / 1 failed (not the historical 35/34/1).

Exact next boundary: a bounded source correction and independent DB-backed tests
for those four P1s, then a **new**, separately approved sealed deployment and
failed-cells replay. Do not lower the RAM gate, rerun the used activation,
reprocess evidence, rewrite history, download models, or start NX-B2. Detailed
fresh analysis IDs and oracle evidence are in the R2 overlay of
`reports/nexusai-nxmmr-post-activation-live-p1-acceptance-v1-20260902.md`.

## 2026-09-02 NX-MMR P1 correction R2 activation — superseded handoff

R2 activation completed and independently verified under receipt
`20260902T154901947Z`. Live API image
`sha256:91616844039217196b9f30e2478dc2e5e91525e0e8a4fc74d6a7887a7e3eb85a`
and forensic API image
`sha256:254649dcb4d032669d9f9dc46f21fddb91ccddba86a00631ea70c245b1f3e9d3`
match the sealed build set, are running, and have zero restarts. API health is
PASS. The receipt state and activation verification are both `VERIFIED/PASS`.

Worker, PostgreSQL and NATS retained their exact pre-R2 container and image
identities. Required `faster-whisper-small-ur` and
`face-detect-yunet-sface` models remain present. Active jobs are zero and
retained accounting remains `63|63|76|22507|827|61`. No migration, retained
evidence rewrite, volume deletion, model download or bulk reprocessing occurred.
The orphan warning was informational; no orphan removal ran.

Do not rerun this activation. Runtime activation is complete, but product/live
acceptance remains pending. Exact next action: execute only the previously
failed live cells for transcript timing/seek, selected-audio intent isolation,
selected-video exact/no-match, selected-image OCR exact/no-match, image/face
Ask and Activity presentation, and cross-family citation fairness. Preserve
case/evidence scope and real Urdu-script requirements. Report:
`reports/nexusai-nxmmr-p1-correction-r2-activation-verified-20260902.md`.

## 2026-09-02 NX-MMR P1 snapshot-reader repair R2 — superseded handoff

R1 receipt `20260902T152519865Z` built both candidate images and recreated the
two admitted services. Verification then failed because the private snapshot's
PowerShell JSON form stores each inspect object as a one-element `value` array;
the reader expected `Config` directly on `value`. The automatic rollback
recreated both original immutable images, then reported the same parser error.

An independent read-only rollback oracle correctly unwrapped the saved snapshot
and passed: original image IDs, environment/mount/network/restart topology,
protected worker/PostgreSQL/NATS identities, health, models, zero active jobs,
and retained tuple `63|63|76|22507|827|61`. The two recreated original-service
container IDs are now the R2 preflight baseline. The failed R1 receipt remains
preserved; do not rerun or treat its status field as activation success.

R2 accepts only a direct inspect object or the observed single-element wrapper
and rejects ambiguous arrays. Direct/wrapped/rejection cases raise the operator
self-test to 45/45 PASS. Manifest and tags are new, so R1 images cannot be
silently resumed. Generated policy/rollback and every non-RAM live preflight
gate pass. Current free RAM is 5.004/6 GiB with Codex open. No build or service
mutation was performed during R2 preparation.

Exact next action: close Codex and other optional apps, keep Docker Desktop,
internet and standalone PowerShell running, then execute
`scripts/run_nxmmr_p1_correction_activation.ps1 -WaitForRamMinutes 120`.
Full details: `reports/nexusai-nxmmr-p1-snapshot-reader-repair-r2-20260902.md`.

## 2026-09-02 NX-MMR P1 build-dependency repair R1 — superseded handoff

Operator receipt `20260902T145238258Z` passed the 6-GiB gate but failed
building the API at Ubuntu apt installation because `build.network=none`
could not supply an uncached dependency layer. No candidate image completed;
receipt mutation/recreation flags were false and the accepted container
identities remained unchanged. This was not an out-of-memory failure.

The user approved necessary Dockerfile build-dependency acquisition. The
resealed revision `NX-MMR-SHARED-P1-CORRECTION-ACTIVATION-V1-BUILD-DEPS-R1`
uses build-only `network=default`, no forced base refresh, bounded API Go
compilation (`GOMAXPROCS=4`, `GOFLAGS=-p=2`), and unchanged no-build/no-pull
runtime startup. Model downloads, runtime package installation, volume/data
deletion and worker/PostgreSQL/NATS recreation remain prohibited. A receipt
outcome-property bug is also fixed and tested under Windows PowerShell 5.1.

All 42 static/behavioral operator checks and generated Compose validation pass.
Read-only preflight passes every non-RAM gate; current free RAM is 5.233/6 GiB,
active jobs zero. No build, dependency download or service mutation occurred
during this repair. No actual rebuilt-image or rollback success is claimed.
The old failed receipt is preserved and cannot resume under the revised seal.

Exact next action: close nonessential apps, keep Docker Desktop, internet and
standalone PowerShell available, then run
`scripts/run_nxmmr_p1_correction_activation.ps1 -WaitForRamMinutes 120`.
The command path is unchanged but now invokes the repaired bundle. Wait for
verification PASS before failed-cells-only browser acceptance. Details and
current hashes: `reports/nexusai-nxmmr-p1-build-dependency-repair-r1-20260902.md`.

## 2026-09-02 NX-MMR consolidated P1 correction candidate — superseded build-policy handoff

The correction source is reconciled, tested and sealed under
`NX-MMR-SHARED-P1-CORRECTION-ACTIVATION-V1`. Actual Dockerfile/source ownership
proves the minimum deployment scope is `api` plus `forensic-records-api`; the
worker has no new production change and is now protected with PostgreSQL and
NATS instead of being rebuilt. The accepted live receipt
`20260902T025246574Z` and retained tuple `63|63|76|22507|827|61` remain
unchanged.

Focused agent/API/UI routing, timing, locator, OCR exact/no-match,
selected-video ANPR, suffix parsing, similarity/face, citation fairness,
scope/authorization, vet, ESLint, anti-hardcode, privacy and diff checks pass.
The offline operator bundle passes 30 static checks. Its read-only live
preflight passed every non-RAM gate and blocked only at 2.919 GiB versus the
unchanged 6-GiB floor; active jobs were zero. No build, image, restart,
recreation, migration, retained mutation or model acquisition occurred.

Exact next action: close Codex/ChatGPT/browser/editor processes, keep Docker
Desktop and standalone PowerShell running, then execute
`scripts/run_nxmmr_p1_correction_activation.ps1 -WaitForRamMinutes 120`. The
runner builds sequentially with network disabled, resumes exact source-bound
images, and may drain only the two candidate services after both candidates and
rollback are sealed. After a new verification PASS, rerun only the previously
failed live cells. Consolidated report:
`reports/nexusai-nxmmr-consolidated-p1-correction-activation-candidate-v1-20260902.md`.

## 2026-09-02 NX-MMR post-activation live P1 acceptance — current handoff

Activation receipt `20260902T025246574Z` remains verified and was not rerun.
Fresh explicit-Urdu evidence `c80595c4-dd54-4604-a6ad-a687319eb03a`
retained real Urdu script, correct `asr_language=ur`, a 0–10 second raw
observation, and a parent-linked Roman-Urdu derivative. Urdu generic Ask,
exact-found, exact-no-match, raw RTL, English exact STT, general exact ANPR,
structured CDR, Data SigLIP/face rankings, backend-authoritative pagination,
and two-step phone+plate planning passed live. No cross-case leakage or P0 was
observed.

Live P1 failures remain on the deployed images: transcript timing anaphora and
seek presentation; explicit plate intent under selected audio; selected-video
exact/no-match routing plus oversized retained error status; Data-generated
exact OCR routing; SigLIP/face Ask presentation; and fair cross-step citations.
The phone+plate composition completed correctly, but the 20-citation UI cap
showed only CDR sources and hid the ANPR source. Activity also mislabeled the
failed selected-video and image-similarity results.

One compatible source-only correction set now passes focused tests. It preserves
typed source-time locators, bounds transport status, recognizes explicit plate,
OCR and natural face/similarity wording, promotes selected video to the retained
video-ANPR operation, carries request-scoped evidence/context through NATS,
presents similarity candidates/scores/citations instead of metadata, and
prioritizes one citation per completed composition step. UI presentation tests
pass 13/13; focused agent/API/serialization/composition tests pass. No rebuild,
deployment, restart, migration, model download or reprocess was performed.

Final runtime is healthy. Host free RAM is only 1.993 GiB; API uses 4.169 GiB
and worker 510.9 MiB. Do not attempt activation in this state and do not rerun
the old receipt. Exact next action: prepare and independently verify one new
source-sealed, rollback-capable three-service correction candidate using the
existing sequential/resumable clean-page-cache recovery rules; then perform one
separately approved activation and rerun only failed live cells. Full report:
`reports/nexusai-nxmmr-post-activation-live-p1-acceptance-v1-20260902.md`.

## 2026-09-02 NX-MMR shared evidence query P1 activation — current handoff

The single bounded activation completed and independently verified under receipt
`local-acceptance-models/nxmmr/private-activation-shared-query/20260902T025246574Z`.
The live `api`, `forensic-records-api`, and `forensic-records-worker` images match
the receipt, are running with zero restarts, and active jobs are zero. PostgreSQL
container `f66e05a3b179...` and NATS container `5c49e70d132a...` retained their
pre-activation identities. No migration, named-volume change, retained-evidence
rewrite, prune, pull, model download, or bulk reprocess was performed. Do not
rerun the activation command for this candidate.

Exact selected-image OCR/no-match, transcript time locators and
media seek, phone+plate(+document) composition, ANPR trailing suffixes, scoped
follow-ups, and backend-populated image/face similarity Ask/citations pass
source tests. Add Data now exposes explicit Urdu; the durable `ur` hint reaches
ASR and the worker rejects Devanagari or no-Urdu-script output. AUTO remains the
default and the retained no-hint sample is not rewritten. The sealed offline
bundle builds/recreates only `api`, `forensic-records-api`, and
`forensic-records-worker`; PostgreSQL and NATS identities are protected. Its mandatory 6-GiB preflight,
sequential/resumable builds, exact rollback and no-prune/no-download rules pass
25 static checks. The runner passed the unchanged 6-GiB gate after bounded waits
and used only verified-clean mode-1 page-cache recovery.

Runtime activation is complete, but product/live acceptance is still pending.
The exact next action is one fresh controlled Urdu audio upload through Add Data
with `Urdu — اردو` selected, followed by raw Urdu-script, Roman-Urdu lineage,
timed-citation/seek, Activity reopen, and cross-scope checks. Fail the test rather
than accept Devanagari or Latin-only output as Urdu. Then complete the remaining
21-point real-browser matrix; do not promote PRODUCT_CERTIFIED or NX-B2 before
that evidence exists. See
`reports/nexusai-nxmmr-shared-evidence-query-p1-closure-v1-20260901.md`.

## 2026-09-01 NX-MMR live multimodal product acceptance V1 — current handoff

Activation `20260901T154458190Z` remains complete and verified; no rebuild,
redeploy, restart, migration, prune or bulk reprocess occurred. Two approved
fresh FLEURS sources brought the workspace to 35 total / 34 ready / one
preserved failed. English ASR stored three timed segments; Urdu ASR ran but
returned one Devanagari `hi` segment with no raw Urdu or Roman derivative.

Live positives: current-version selected English transcript exact lookup,
exact image ANPR `[REDACTED PLATE]`, CDR frequent contacts, and a 193-event bounded CDR
time/aggregate result. Responsive 390/820/1024/1440 passed. Retained SigLIP and
SFace rankings worked with correct non-identity semantics. Live P1 failures:
transcript citations omit time/seek; OCR selected exact lookup cites unrelated
PDFs; phone+plate composition omits structured evidence; UI context crosses
scope/explicit targets; absent `ZZ99ZZZ` returns 20 unrelated ANPR rows.

Source-only fixes now bind answer context to identical portal scope, inherit it
only for anaphoric follow-ups, and recognize generic plates with up to three
trailing letters. Analyst unit 7/7 and focused Go extractor test pass. Full
agent suite reached 111/125; 14 environment-only Windows testcontainer failures.
No live claim is upgraded by these undeployed fixes. TTS English and Urdu remain
not acquisition-ready; no download. Exact report:
`reports/nexusai-nxmmr-live-multimodal-product-acceptance-v1-20260901.md`.

Next: source-fix/test OCR scope, transcript locator time, and cross-family
composition, then one separately approved bounded activation containing all P1
fixes. Preserve the 6-GiB deployment gate, rollback, current models and retained
data. Product certification=false; NX-B2 remains paused.

## 2026-08-31 NX-MMR speech acquisition / live closure V1 — current handoff

Continues the existing breadth work item; supersedes the handoff immediately
below. Owner authorized exactly bounded FLEURS acquisition and representative
demo uploads/Ask checks, not deployment, new models, migration or NX-B2.

- Acquired ten fixed `google/fleurs` en_us validation rows 0–9 at revision
  `70bb2e84b976b7e960aa89f1c648e09c59f894dd`, CC-BY-4.0; 80 seconds,
  5,120,580 audio bytes. Publisher transcripts/private sealed manifest only.
- Actual installed ASR, no tuning: English WER 0.107955/CER 0.032294 (10 clips);
  existing Urdu WER 0.454545/CER 0.164634 (2 clips). Zero failures, all timed;
  independent score recomputation PASS. English data acquisition gap CLOSED.
- One fresh English Add Data source `22a595a9-bd51-4da0-8cde-dd892b55842e`:
  technical inspection complete, **zero transcripts** because worker ASR=false.
  Demo now 33 sources/32 completed/one failed; completion is not STT readiness.
  Five fresh Ask histories: two wrong document-retrieval routes, two capability
  unavailable, one time-bound clarification. No STT semantic PASS.
- English/Urdu playback ended normally; raw Urdu RTL and two exact derivative
  parent/time links verified. Fresh Activity result reopen verified. Source
  fixes remove false transcript-ready labels and support duration_ms without
  coercing unknown to zero. Prior timing/parent and new fixes are not deployed.
- Worker face/image embedding roles also false. SigLIP configured directory
  missing inside mounted models/media; Python dependencies present. No new model
  required. Fresh stored three-candidate image/face rankings PASS; direct face
  detector one-face/zero-face checks PASS, not worker ingestion acceptance.
- TTS research pinned Piper voice/runtime (33,737,429-byte backend layer) but
  packaged dependency SBOM/installed size/GPL notice review incomplete. Matcha
  acoustic ONNX pinned; documented Vocos repo returns 401. Neither pack is ready
  for acquisition approval. No TTS/new model/backend/dependency download.
- Focused tests: 22 Python (9 acquisition/scoring + 9 timing + 4 V3), 19 ANPR/OCR,
  31 Data/Media/Ask/Activity, focused Go regression PASS; ESLint zero errors and
  nine pre-existing warnings. Acquisition checks also pass optimized Python;
  public-manifest equality, private-text/anti-hardcode scans, JSON and tracked
  diff checks PASS. No full build, deployment or certification.

Next: bounded typed transcript evidence-scope/retrieval/exact/time source slice,
then separately approved precise deployment and one-source immutable reprocess.
Preserve 6-GiB gate, ANPR/OCR/V3 configuration and NX-B2 pause. Do not run old
bulk activation/recovery scripts. Details and safety boundaries:
`reports/nexusai-nxmmr-speech-live-closure-v1-20260831.md` and sanitized JSON;
existing dated team-lead readiness matrix/runbook remain authoritative.
RetainedStateMutated=true only for one source and five queries; no migration,
volume change, new certification, staging, commit or push.

## 2026-08-31 NX-MMR-NEXT-MULTIMODAL-BREADTH-V1 — current handoff

The latest owner directive freezes functional ANPR/OCR and advances breadth.
This entry supersedes older activation/tuning next steps below; NX-B2.1 remains
paused. Overall PARTIAL, not all-modality demo closure or product certification.

- Fresh inventory: five installed LocalAI models, no TTS backend/model; healthy
  existing services. Multimodal Product Acceptance has 32 sources, 31 completed,
  one failed, including three completed Urdu/mixed audio files. Do not reuse
  historical 21-source counts as current state.
- Reused existing Faster-Whisper small: fresh synthetic English transcription
  works (natural-English quality unproven); real FLEURS Urdu 6.24-second clip
  returned two segments in 3.783 seconds. Independent publisher-transcript
  scoring WER 0.6 / CER 0.1852. Read speech is not conversational validation.
- Source-fixed NEXT-P1-ASR-TIME: no fabricated clip-wide segment timing,
  unknown/invalid time remains unknown, Roman Urdu pairs by parent ID and
  preserves raw Urdu/identifiers. NEXT-P1-DATA-PAGE discloses loaded-page
  filtering instead of implying workspace-wide zero. Neither fix is deployed.
- Read-only live browser: Home/Data/Ask/Activity, raw Urdu plus derivative and
  stored audio result reopen verified. Playback/seek and fresh Ask unrun;
  legacy stored Markdown rendering remains imperfect. Local media quick-action
  suggestion certification still needs reconciliation; do not claim whole-UI
  PRODUCT_CERTIFIED-only PASS.
- Existing SigLIP seven-file host cache found; 812,672,320-byte weight SHA-256
  verified. Current worker mount/fresh ranking not verified. Face/native-document/
  structured historical integration evidence remains valid but not freshly
  recertified. Baseline regressions: 19 ANPR/OCR and four Video V3 checks pass.
- New ASR tests 9; analyst presentation tests 35. No dependency install, full
  build, deployment, source reprocessing or retained evidence/query mutation.
  Direct inference can warm model memory; no current RAM-admission PASS.
- Next boundary: lawful natural English clips with independent transcripts;
  exact pinned English TTS backend and Urdu acoustic/vocoder/license/size/hash
  manifest before any isolated acquisition approval. No blanket downloads.
  Later worker/UI deployment requires separate approval and unchanged 6 GiB gate.

Read `reports/nexusai-next-multimodal-breadth-v1-20260831.md`, the matching dated
team-lead readiness Markdown/JSON, current roadmap/phase ledger/backlog, and
`docs/demo/nexusai-team-lead-multimodal-demo-v3.md`. Certification remains
62 REGISTERED / 12 SOURCE_VALIDATED / 1 FIXTURE_CERTIFIED / 4 PRODUCT_CERTIFIED.
No product promotion or NX-B2 activation occurred.

## 2026-08-31 OCR / 300 MiB correction DEPLOYED and VERIFIED

Operator run private-activation/20260831T135259631Z completed with
CORRECTION_DEPLOYMENT=PASS. It used the pinned completed images, skipped build
and downloads, waited until all ChatGPT/Codex processes closed, drained only old
worker/LocalAI, passed the unchanged 6-GiB gate (6.162 GiB, then 6.147 GiB), and
recreated only those two targets. Live worker c0e65cbc8e87 runs immutable image
sha256:2755e0ffbf3774d722b12cc9a5fbfc2241aad6e876e9891ca8b1c13d24afd146;
LocalAI ede93502d542 runs
sha256:54683ddd2c2302632b25c2619d13254f7ef685d46f556ea3854fdfcd5463f8c1.
Both healthy, restart_count zero. Forensic API b5c13f96e67f, PostgreSQL
f66e05a3b179 and NATS 5c49e70d132a identities/images were preserved.

Independent receipt/live verification PASS: correction state VERIFIED; jobs
zero; retained counts unchanged 54|54|67|22208|446|50; runtime
LOCALAI_UPLOAD_LIMIT=320 and UPLOAD_LIMIT=320; worker processor revision
nxmmr-anpr-ocr-vertical-v1-inputfix1. Readiness probe: image ANPR, Video V3 and
Paddle OCR MODEL_LOAD_READY, Ultralytics absent, downloads/inference false.
Non-body GET header probe with Content-Length 314572800 returned HTTP 200.
This proves admission-gate configuration, not an actual upload/processing PASS.

Exact next action: operator hard-refreshes the existing Multimodal Product
Acceptance UI and visually confirms new 300 MiB general/video and 64 MiB image
copy. Then perform separately controlled negative-image recovery/freshness check
and 175.9 MiB video upload one at a time, preserving dedup truth and waiting
through upload/registration. Do not delete/rename evidence to evade dedup or
claim reused output as fresh. API/Ask/Data/citation/Activity/UI/manual gates
remain pending; no PRODUCT_CERTIFIED, holdout rerun, retained bulk reprocessing
or NX-B2.1.

## 2026-08-31 correction candidates BUILT; guarded drain/resume prepared

Subsequent run private-activation/20260831T134939144Z stopped before any
mutation because the exact-current identity still named pre-rollback container
IDs. This was a correct fail-closed stop. Restored live IDs are now LocalAI
d015424c9d74 and worker baec952b54bc on the exact original image IDs, healthy,
restart_count zero, jobs zero and retained counts unchanged. LocalAI host-config
hash changed only because Compose reordered the same five volume bind strings;
semantic mount verification already passed and worker host hash is identical.
Expected-current identity/fingerprint renewed and sealed. 25 operator tests,
source seal and live renewed-runtime identity all PASS. No build/deployment was
performed in the identity-stop run. Exact next action remains the same pinned
completed-image drain/resume command; app-close gate now precedes target drain.

Operator drain run private-activation/20260831T133534230Z verified pinned
candidates/tests, stopped old worker/LocalAI, then was externally interrupted
after about two minutes (`pipeline has been stopped`) while ChatGPT/Codex were
reopened. It did not wait the requested 30 minutes or start candidates.
Rollback was armed. First wrapper rollback attempt exposed two verifier defects:
historical api containers also matched a label-only lookup, and the protected
PowerShell snapshot wraps Docker objects under `.value`. Lookup is now restricted
to exact current container names and snapshot unwrapping is deterministic.
Exact old images/config/mounts were restored; direct verification PASS: all five
services healthy, jobs zero, retained counts unchanged 54|54|67|22208|446|50.
Receipt rollback-result.json records PASS_DIRECT_VERIFICATION.

No deletion/prune is appropriate: C: still has ~1.64 TiB free. During drain,
Windows was missing ~0.64 GiB while five ChatGPT processes plus Codex used
~1.65 GiB working set. A second clean-cache attempt refused because Linux Dirty
was nonzero (24 KiB on recheck); it made no change. The corrected resume now
waits for all ChatGPT/Codex processes to close *before* stopping targets, then
uses the existing 6-GiB drain gate. Operator checks 25 PASS; seal PASS. Run the
same pinned drain/resume command externally, close apps when the explicit
OPERATOR_APPS_CLOSED=WAITING marker appears, and do not reopen/interact until a
final PASS or FAILED/rollback marker.

Operator run private-activation/20260831T115207942Z passed 16 OCR and 7 upload
tests, built both candidates and passed worker source, LocalAI version and
embedded-UI smokes. Immutable images: worker
sha256:2755e0ffbf3774d722b12cc9a5fbfc2241aad6e876e9891ca8b1c13d24afd146;
LocalAI/UI sha256:54683ddd2c2302632b25c2619d13254f7ef685d46f556ea3854fdfcd5463f8c1.
Post-build free RAM fell to 3.701 GiB and never reached 6 in the wait; run
stopped with touched=[], so no live replacement or retained processing.

Diagnosis: disk is not constrained (C: ~1.64 TiB free). vmmemWSL ~4.46 GiB;
Docker containers use ~2.74 GiB (LocalAI 1.718 GiB, worker 848 MiB); Linux clean
file cache ~2.97 GiB, Dirty/Writeback zero. One guarded clean cache release at
private-activation/20260831T132712838Z recovered only 0.140 GiB (3.267 to
3.407), proving cache-only recovery insufficient; no disk build cache, images,
models or volumes deleted and service identities/counts/jobs preserved.

New resume path pins the completed receipt/image hashes and skips all builds.
With -DrainTargetsForRam it verifies candidates/rollback/zero jobs first, marks
both target services rollback-required, stops only old worker and LocalAI, then
waits for 6 GiB and starts the candidates. Forensic API/PostgreSQL/NATS remain
running. Any post-drain failure attempts exact two-target rollback; no gate
bypass or Docker/WSL shutdown. Exact standalone PowerShell command is in
docs/demo/nxmmr-acceptance-correction-operator-20260831.md. Operator must close
Codex/ChatGPT/browsers before running and report final marker/receipt. Product
acceptance, retained retry and certification remain pending.

Final no-mutation resume preflight private-activation/20260831T133310529Z
reported COMPLETED_IMAGES=VERIFIED, BuildSkipped=true, Downloads=false, then
safely blocked at 3.746 GiB while Codex/ChatGPT remained open. No services were
stopped/replaced. Operator self-tests now 23 PASS and correction source seal
PASS. This proves the standalone resume reaches the intended immutable-image
boundary; the actual drain/replacement still must be run from external
PowerShell after closing Codex/ChatGPT/browsers.

## 2026-08-31 standalone correction deployment script PREPARED for operator

User requested complete deploy/rebuild commands to run with Codex/other apps
closed. New scripts/deploy_nxmmr_acceptance_correction.ps1 provides read-only
preflight, bounded RAM waiting, sequential OCR-only cached worker and LocalAI/UI
builds, terminal plus durable logs, exact current-runtime rollback snapshots,
only worker/api replacement and guarded automatic/manual rollback. Six-GiB RAM
floor remains unchanged; no app/service stopping, cache eviction or gate bypass.
Forensic API/PostgreSQL/NATS remain protected. Worker Dockerfile replaces only
multilingual_ocr.py on the existing activated image; no dependency reinstall.
LocalAI builds embedded UI with GOMAXPROCS=4/GOFLAGS=-p=2; build-only dependency
network may be required. Original manifest/seal untouched; new correction seal
binds the reviewed delta and operator files.

17 synthetic operator checks PASS under Windows PowerShell 5.1. Actual
preflight validated live identities/health/model hashes, zero jobs and generated
Compose/rollback configuration, then safely stopped at 4.624 GiB free RAM.
Receipt: private-activation/20260831T114659710Z. No build, image tagging,
service replacement or retained processing occurred in this preparation turn.
Final sealed-script preflight repeated all checks and safely blocked at
4.830 GiB; receipt private-activation/20260831T115035240Z. Source seal PASS.
Full deployment/rollback execution is not yet empirically verified. Runbook:
docs/demo/nxmmr-acceptance-correction-operator-20260831.md. Exact next action:
operator runs new script from standalone PowerShell after saving work/closing
nonessential apps, with -WaitForRamMinutes 30; keep Docker running. Product
acceptance/negative retained retry remain separate and pending. No promotion.

## 2026-08-31 OCR input and 300 MiB upload correction PREPARED, not deployed

User approved preparing/testing the diagnosed fixes and requested at least
300 MB data uploads. Candidate Paddle detector now receives the already-decoded
BGR frame; revision nxmmr-anpr-ocr-vertical-v1-inputfix1. UI general/video
ceiling is 300 MiB; image guard stays 64 MiB. Prepared-only Compose fragment
docker-compose.nxmmr-upload-300m.yaml sets LOCALAI_UPLOAD_LIMIT=320 for the
whole request including multipart metadata. Live cap remains unchanged.
Explicit HTTP 413 versus uncertain-network messages and Waiting/closing
instructions added. No concurrency increase or model/threshold change.

16 ANPR/OCR self-tests and 7 upload-policy tests PASS. Four focused mocked
Chromium intake checks and the full 24-test portal suite PASS; missing isolated
operations-poll mock repaired without relaxing assertions. Focused ESLint/API
syntax checks PASS. Network-disabled, cached-runtime Paddle smoke
using generated pixels PASS: blank COMPLETE_ZERO_RESULTS, printed control
COMPLETE_RESULTS; named/extensionless parity and source preservation confirmed.
12.156 s wall, 694.473 MiB peak process RSS. Live container IDs/images/restarts
unchanged, jobs zero. Receipts in private-activation/acceptance-correction-20260831.
No retained upload/retry, DB/volume write, model/package install, service
recreation or certification. No production build run. Old activation seal is
historical and intentionally mismatches the changed OCR source: do not bypass
or blindly reseal/reuse the old worker-only activation plan.

Next: explicit authorization for guarded worker/UI correction deployment and
protected LocalAI request-limit update with a fresh current-runtime snapshot,
rollback and unchanged 6 GiB/zero-job gates (post-smoke RAM 2.960 GiB, so memory
gate not met). Negative retained retry and video upload remain separate gates;
use existing Multimodal Product Acceptance collection, never dedup evasion.
API/Ask/Data/citation/Activity/UI/manual acceptance still pending; no
PRODUCT_CERTIFIED or NX-B2.1. Details:
reports/nexusai-nxmmr-acceptance-correction-300m-20260831.md.

## 2026-08-31 first live acceptance findings: positive works; OCR path and upload limit fail

User reports image-positive.JPG worked; screenshot shows Ready and worker log
records completion at 11:16:55 UTC. This is not confirmation of every Ask/citation/
Activity/manual gate. Negative image registered (HTTP 200; ingest job
9847dea1-001d-48df-9f7f-3fc2a3856e0b) but failed processing: contextual routing
falls back from zero ANPR observations to Paddle OCR, whose detector receives
the extensionless content-addressed spool path via input=str(path). Paddle logs
unsupported suffix and raises 'The number of inputs does not match the model:
1 vs 0'. Source already decodes the frame with cv2 but does not pass that frame
to detection. Further printed OCR acceptance is paused pending a tested adapter
correction; no retained job retry/reprocessing was performed.

Video screenshot reports upload connection failure for 175.9 MiB video-v3.mp4.
Read-only GET /readyz with its declared Content-Length (184407144), but no body,
returned HTTP 413; ordinary GET returned 200. The HTTP body-size gate therefore
rejects this size. Repository default upload limit is 15 MiB; no upload-limit
environment override was found on LocalAI. No video ingest queue entry was found
in the inspected interval; this is not evidence of Video V3 inference failure.
The UI's generic XMLHttpRequest error obscures the size rejection. A bounded
upload-limit correction would affect the protected LocalAI HTTP service and
needs separately authorized deployment, not the original worker-only command.

Instructions clarified: Waiting before submission means click Add selected data;
Waiting during an active batch is queued. Keep the page/dialog open during
Uploading/Registering; after all uploads are accepted, Processing continues
server-side and the dialog can be closed. No source/runtime settings, services,
models or retained state changed during diagnosis. All current services healthy;
product acceptance blocked, not PRODUCT_CERTIFIED. Next: obtain authority to
prepare/test the two fixes without deployment or retained reprocessing.

## 2026-08-31 manual acceptance uses the EXISTING Multimodal Product Acceptance collection

User explicitly selected the already-present Multimodal Product Acceptance
collection instead of a new collection, and requested detailed manual upload/test
instructions. This supersedes the earlier new-case/collection recommendation.
The prepared six-file inventory and exact paths were read from
`private-activation/20260831T110755253Z/fresh-acceptance-20260831T111030836Z/inventory.json`.
UI-source review confirmed Add Data > Browse files > Add selected data,
Open source, Technical details (Evidence ID, SHA-256, Processing route),
media finding actions and evidence-linked Ask. Existing evidence is to remain
untouched. A matching-content response/old result must be recorded as reused,
not passed as fresh processor acceptance; no renaming/editing to evade dedup,
bulk reprocessing, deletion or collection reset is authorized by the guide.
No upload/inference/browser test was executed in this instruction-only turn.
All six tests, Activity/reopen, multilingual/citation/UI/manual checks remain
PENDING. No direct PRODUCT_CERTIFIED promotion or NX-B2.1.

## 2026-08-31 fresh acceptance pack READY; first image UI check pending

Operator output confirms ACCEPTANCE_PACK=READY at
`private-activation/20260831T110755253Z/fresh-acceptance-20260831T111030836Z`.
Preparation creates six source copies and inventory only: no upload, benchmark,
inference or product-acceptance PASS is established. Activation remains VERIFIED.
Next is a new controlled case/collection at localhost:8080, adding only
`image-positive.JPG` first. Check fresh processing identity, Image classification,
COMPLETE_RESULTS, overlay/crop/confidences, Ask and region citation, Activity and
reopen before the negative image, video and printed OCR cases. Stop if old
results are reused; no retained bulk reprocessing. The historical runbook's
initial activation-gate wording is superseded by the completed activation receipt;
do not rebuild or require the build RAM floor for this browser phase.

## 2026-08-31 operator activation VERIFIED; fresh product acceptance next

Operator receipt `private-activation/20260831T110755253Z` records ACTIVATION=PASS
and ACTIVATION_VERIFICATION=PASS. Preflight RAM was 6.648010 GiB; final
pre-recreation RAM was 6.653824 GiB. Every build layer was reused from cache;
no package download was needed. Receipt state is VERIFIED with expected image
`sha256:e495788b85f07d9a4552c07796f8b810fa4f600ecd96a08acfa6c2afbb02ce73`.
The new worker `4866d8fc0ea3` is recorded healthy with restart_count=0; all four
protected service identities/images remain unchanged. Active jobs zero.

Read-only receipt confirmation: Image ANPR, Video V3 and Paddle OCR are all
MODEL_LOAD_READY; Ultralytics unavailable; model_downloads=false;
inference_performed=false; product_certified=false; live_product_acceptance=PENDING.
The deployment RAM gate is complete; browser acceptance uses normal workload-aware
resource policy, not the 6-GiB build floor. Do not rerun activation/preflight as
the next step: its original-worker rollback identity intentionally no longer matches.

Exact next action: `scripts/prepare_nxmmr_demo_acceptance.ps1` verifies the live
image/health and creates six hash-verified source copies under this receipt;
it does not upload, infer, or certify. Then follow
`docs/demo/nexusai-team-lead-multimodal-demo-v3.md` in a fresh controlled
case/collection, one file at a time, checking fresh processing identity and
API/Ask/Data/citations/Activity/UI/manual acceptance. Stop on deduplication reuse;
do not bulk reprocess retained evidence. No new benchmark/holdout access or
NX-B2.1; STT/TTS progression remains after actual product acceptance.

## 2026-08-31 one-time clean page-cache recovery completed; no activation

User approved one cache-only recovery. Exactly one clean page-cache release
(`drop_caches=1`) succeeded in docker-desktop, without sync, service restart,
model unload, file deletion, package installation or activation. A preceding
generic-wrapper read-only WSL memory query failed before reaching the release;
direct WSL inspection succeeded and the release was not repeated.

Receipt: `private-activation/20260831T110618358Z`. Windows available RAM rose
from 2.717930 to 5.056976 GiB (+2.339046 GiB). Linux Cached after release was
1,332,352 KiB; later Windows vmmemWSL working set was 2.317 GiB. All five service
identities/images/restarts were unchanged; health PASS; active ingestion jobs 0.
The completed candidate image remains present at
`sha256:0ee45fb25d4440ba24bac82f6bc080f3f5a0cbbd38eaccf657b325853dce8612`.

Full post-recovery preflight is BLOCKED only on RAM (4.996563 GiB versus 6),
while ChatGPT processes occupy 1.674 GiB aggregate working set. This is not a
guarantee that all process working set becomes available upon closing. Exact
next action: operator closes the app fully, retains Docker Desktop/PowerShell,
runs preflight, and only on PASS runs cached activation. Do not clear cache
again, bypass the gate, stop WSL/services, or run verification before activation
PASS. Receipt contains before/after metrics and full post-recovery preflight.
Product acceptance remains pending; no PRODUCT_CERTIFIED promotion or NX-B2.1.

## 2026-08-31 RAM diagnosis; cache-only recovery proposed, not executed

After closing applications, operator preflight still measured 4.480614 GiB
available versus 6 GiB. Read-only diagnosis found 15.713 GiB physical RAM,
vmmemWSL working set 4.929 GiB, and Linux Cached=4,870,332 KiB (~4.645 GiB),
MemAvailable=5,215,184 KiB (~4.974 GiB), AnonPages=1,957,980 KiB (~1.867 GiB).
These host/guest views overlap and are not additive; not all guest cache is
guaranteed to return to Windows. Linux Dirty/Writeback and current PSI averages
were zero. Docker reports LocalAI 2.15 GiB, other four containers ~208 MiB.
Only the five expected containers run; Ubuntu is stopped. All protected service
identities/restarts and health checks pass; active ingestion jobs zero.

ChatGPT processes were 1.593 GiB aggregate working set after reopening for this
diagnosis; Windows services/Defender are not targets for forced termination.
WSL 2.7.12 is installed; no user .wslconfig exists. Current Microsoft docs list
autoMemoryReclaim=dropCache as default, so absence of the file does not prove
reclamation is disabled. Post-build cache retention is a plausible substantial
contributor, not evidence of a model leak or a proven amount recoverable on host.

Recommended next step requires explicit authorization: one cache-only Linux
page-cache reclamation attempt with before/after RAM, service identity/health,
and jobs checks, without stopping WSL/containers, uninstalling anything, deleting
image/package/model files, or lowering the 6 GiB gate. This is a one-time debug
recovery, not periodic cache flushing; subsequent disk I/O can be slower. Then
operator preflight and cached activation only if resource gates pass. No recovery,
build, deployment, live-model unload or system-setting change was executed here.

## 2026-08-31 operator image built; post-build RAM gate blocks recreation

Receipt `private-activation/20260831T101559131Z` completed worker package
installation on attempt one (package layer 2304.8 seconds), image export, the
network-disabled import smoke and built-source hash checks. Candidate image is
locally present as `sha256:0ee45fb25d4440ba24bac82f6bc080f3f5a0cbbd38eaccf657b325853dce8612`.
The final pre-recreation RAM check measured 3.779896 GiB against the unchanged
6 GiB floor and stopped activation. The original receipt remains unchanged:
BUILD_STARTED / recreation_started=false. That coarse state does not negate
the successful build recorded by build.log and the post-build receipts.

Read-only follow-up confirmed original worker `296e0881d891` healthy, all five
original service IDs still present, and the candidate image retained locally.
Current free RAM was 2.847885 GiB; vmmemWSL working set was approximately
4.938 GiB (not a measurement of reclaimable cache). No agent build, installation,
activation, rollback, Docker/WSL restart, runtime/model or retained-state mutation
was performed. No rollback is needed for this pre-recreation stop.

Next action: close unnecessary user applications manually, retain Docker Desktop
and PowerShell, and rerun preflight. Do not retry activation until preflight PASS.
The existing activation path can reuse unchanged cached build layers; do not
prune caches or delete the built image/models. It still revalidates imports,
hashes, services/jobs and RAM before recreation. If RAM remains below 6 GiB,
stop and report that result rather than bypassing the gate or restarting WSL.
Only ACTIVATION=PASS admits verification and then fresh product acceptance.
ProductCertification=false; consumed holdout untouched; NX-B2.1 unstarted.

## 2026-08-31 download recovery and live terminal logs (no build/activation)

Operator receipt `private-activation/20260831T100208816Z` failed on a PyPI socket
read timeout at 125.9/194.8 MB of PaddlePaddle, after downloading the corrected
safetensors pin. Its state remains BUILD_STARTED / recreation_started=false;
the original failed receipt is preserved and needs no rollback.

The user authorized bounded download protection plus terminal logs. The worker
Dockerfile now uses a 120-second pip socket timeout, five connection retries,
at most three whole pip attempts with five-second pauses, and a BuildKit package
cache. The manifest's one-hour build deadline and all resource/model gates are
unchanged. Build stdout/stderr are streamed to the operator terminal and flushed
continuously to build.log; inspection/config/environment output remains private.

Eleven focused tests PASS, including live-before-exit log delivery, stderr,
nonzero exit, timeout, private-output isolation, 4,000-line dual-pipe drainage,
and Linux fake-pip success/recovery/three-attempt-exhaustion. Forty-six bundle
checks PASS, including integrity and read-only actual health/Compose/rollback
and unchanged service identities/images/restarts. Receipts:
`private-activation/20260831T101224407Z` and `20260831T101327182Z`.
Available RAM 5.025 GiB correctly blocks activation below 6 GiB; active jobs 0.
No package installation/download, image build, model access, activation,
retained processing, Activity/database/volume mutation or benchmark rerun occurred.

Next: operator frees RAM by closing unnecessary apps, keeps Docker Desktop and
PowerShell, runs preflight, and only on PASS runs activation with visible logs.
Only ACTIVATION=PASS admits verification. Runtime readiness and fresh product
acceptance remain pending; ProductCertification=false; NX-B2.1 unstarted.
Report: `reports/nexusai-nxmmr-download-recovery-terminal-logs-20260831.md`.

## 2026-08-31 operator build dependency correction (no agent build/activation)

The operator's receipt `private-activation/20260831T094201498Z` passed the 6 GiB
RAM gate and copied the admitted models, but pip stopped before recreation:
`safetensors==0.5.3` conflicts with PaddlePaddle 3.3.1's `safetensors>=0.6.0`.
The original worker `296e0881d891` and all protected services remain unchanged;
rollback is not needed for this pre-recreation failure. Old benchmark receipts,
the original failed build log, and its BUILD_STARTED state are preserved.

The user authorized correcting the pin and revalidating without image building
or service activation. Only safetensors changes to exact `0.6.2`; model assets,
PaddlePaddle, Transformers, V3 source and consumed holdout remain unchanged.
Three bundle regression checks cover this conflict. The standalone disposable
Linux CPython 3.11 resolver uses pip dry-run with build isolation disabled;
packages are not installed and no models or live data are accessed. Full
resolution PASS completed in 477.352 seconds; installed distributions were
unchanged. Receipt: `private-activation/dependency-repair-20260831/attempt2/validation.json`.
All 43 bundle checks PASS, including read-only live checks; integrity seal PASS.
RAM during validation was 3.715 GiB, correctly below the unchanged 6 GiB gate.
No image build, activation, retained processing or certification occurred.

Exact next action: operator closes unnecessary apps, keeps Docker Desktop and
PowerShell, and runs preflight again. Only on preflight PASS run activation;
only on ACTIVATION=PASS run verification. Build/import/model-health and fresh
API/Ask/Data/citation/Activity/UI/manual acceptance gates remain outstanding.
Correction report and copy-ready commands:
`reports/nexusai-nxmmr-operator-dependency-correction-20260831.md`.

## 2026-08-31 offline operator bundle prepared; DO NOT activate from this preparation turn

The operator now has dedicated preflight, activation, verification, rollback,
optional YES/NO orchestration, and fresh-acceptance preparation scripts under
`scripts/*nxmmr*demo*.ps1`, sharing `nxmmr_demo_operator_common.ps1`. The single
authority remains `configuration/nxmmr_demo_activation_v1.json`; its corrections
explicitly disable ASR/face/image embeddings/TTS for demo activation and name all
Paddle paths/hashes. `nxmmr_demo_operator_integrity.json` seals source/scripts/
manifest/notices; it is not competing activation policy. V3/frozen benchmark
source and all benchmark receipts remain unchanged.

Existing admitted Paddle trees were copied from the read-only cache to private
operator staging with identical hashes. No live model/service/retained/Activity/
database/volume state changed. The bundle has no model downloads. The existing
Dockerfile can need Internet for uncached apt/pip dependencies: the workflow is
Codex-independent, not guaranteed air-gapped. Build/recreate is worker-only,
with exact actual-config rollback and protected LocalAI/UI/API/PostgreSQL/NATS.
Rollback restores prior ASR=true, not the new disabled activation default.

Forty PowerShell parser/safety/simulation/read-only live checks passed. Current
model/source/license/Compose/health/rollback gates pass; latest full validation
RAM was 4.963467 GiB, correctly BLOCKED below 6 GiB. Six existing fresh-acceptance
source samples are enumerated and hash-verified. No activation, build, recreate,
rollback execution, benchmark inference or browser acceptance was performed.

Exact next action belongs to the operator: close Codex/Chrome/IDE manually, keep
Docker Desktop + PowerShell, run `preflight_nxmmr_demo_activation.ps1`, then only
on PASS run `activate_nxmmr_demo.ps1` and `verify_nxmmr_demo_activation.ps1`.
Return the durable receipt/output on failure. After verification PASS reopen
Chrome/Codex and run `prepare_nxmmr_demo_acceptance.ps1`; the browser phase uses
normal workload-aware policy, not the build RAM floor. No STT/TTS before live
browser acceptance. Full copy-ready instructions:
`docs/demo/nxmmr-offline-operator-activation-v1.md`; preparation report:
`reports/nexusai-nxmmr-offline-operator-activation-bundle-v1-20260831.md`.

## 2026-08-31 product-first demo continuation: Video V3 selected; activation RAM-gated

The product-first continuation closed the Ultralytics dependency for the demo
path without reopening the consumed V2.2 holdout. A preregistered, plate-only
`NX-MMR-VIDEO-ANPR-ONNX-DEMO-V3` now reuses the existing hash-pinned 384 ONNX
detector and FastPlate OCR at actual-frame 4 FPS with RAW_RGB, one-second local
association, observed-string-only grouping, no vehicle detector, no interpolation
and no tracking. A network-disabled synthetic smoke proved the ONNX providers
while an import guard prohibited Ultralytics.

The single fixed development run used only the 11-event development projection;
zero consumed-reserved rows were accessed. It analyzed 133 frames / 210 OCR crops
with zero failure, 198.19 seconds wall, 486.11 seconds CPU and 550.20 MiB peak
RSS. Event recall is 1.0; exact precision/recall/F1 is
0.777778/0.583333/0.666667; CER is 0.315476; group precision/recall/F1 is
0.866667/0.764706/0.8125; final false groups are 2; boundary MAE/median/p95 is
0.894385/0.5845/2.67675 seconds. All preregistered demo utility gates pass, so
the 384 candidate is selected and no 640 acquisition is needed. This remains
development/demo evidence, not real-world or product certification.

Fresh authoritative project-license plus published hub/release evidence admits
the local open-image/FastPlate/Paddle payloads for the bounded internal demo with
MIT/Apache notices; external redistribution/customer delivery remains separate.
Ultralytics and both V2 `.pt` assets remain blocked and disabled. Worker source
now supports V3 and drops the direct Ultralytics dependency after rebuild. The
existing Data/Ask/citation/UI contracts already consume the same ANPR/OCR/group
packets.

Live activation stopped safely before asset placement/build/recreation because
available RAM was 4.790039 GiB versus the unchanged 6 GiB gate. Disk was
1648.20 GiB, active jobs zero, protected services healthy and identities
unchanged. No live model, retained evidence, Activity, database, volume or
service state changed. Exact next boundary: restore at least 6 GiB available RAM,
then perform the prepared worker-only activation and fresh controlled UI journey;
after manual acceptance, start English/Urdu STT rather than more ANPR research.
Authority: `configuration/nxmmr_demo_activation_v1.json` and
`configuration/nxmmr_video_anpr_onnx_demo_v3_selection.json`.

Closure audit: a separate receipt-only recomputation matched the saved scorer
and stage metrics exactly without inference or reserved access. It reconstructed
176 raw observations and 15 groups; error decomposition is 14/24 exact selected,
eight occurrences without any exact raw reading, and two exact raw readings not
retained by grouping. Registered candidate hashes remain unchanged. A literal
truth scan found no development/reserved plate value in the V3 candidate,
factory, test, evaluator, auditor, or public configuration; older continuation
history still preserves previously documented sample outputs. Final available
RAM was 5.030689 GiB, so the safe stop remains binding. Full disposition:
`reports/nexusai-nxmmr-product-demo-activation-multimodal-progression-v1-20260831.md`.

## 2026-08-31 V2.2 reserved authorization consumed; pre-inference harness FAIL

The user authorized exactly one sealed-reserved run of frozen V2.2 digest
`b0caae6d646e9d188ec5bc387585ac43aa7f1e22504d5b2d741699bab8d9f394`,
followed by read-only license/NOTICE and modular activation consolidation. A
separate one-time harness was preregistered without modifying any frozen input.
Canonical/config/freeze, all 17 candidate sources, five harness sources, scorer,
runtime/interpreter/lock/distribution map, four assets, split/projection and all
policies verified; 10 no-model contracts passed. Pre-run RAM was 4.803860 GiB.

The immutable consumption marker was written and only the eight locked reserved
rows were lexically projected. The partition contains 17 truth occurrences and
14 unique normalized truth strings. `ReservedEvaluationCount=1`; the one-time
authorization is consumed and repeat is forbidden. The evaluator then failed
closed before model load at the 77-entry effective-distribution assertion: an
early metadata scan exposed 64 top-level distributions, while the frozen map
also contains 13 setuptools-vendored distributions registered only after later
imports. A no-model diagnostic found no common-version mismatch. Model loads,
reserved decoded/scored frames and OCR calls are all zero. No reserved metric,
development/reserved delta or generalization result exists. This is a harness
fingerprint-order defect, not a candidate quality result. The harness was not
repaired or rerun; `PostHoldoutTuning=false`.

V2.2 state is now
`RESERVED_AUTHORIZATION_CONSUMED_TECHNICALLY_UNASSESSED`. None of ACCEPTED,
ACCEPTED_WITH_LIMITATION or REJECTED_RESERVED_GENERALIZATION can be asserted
without a score. Original V2/V2.1 and V2.2 development evidence remain intact.
Image ANPR stays `SOURCE_READY_LIVE_MODEL_REQUIRED`; printed English remains
fixture PASS, Urdu/mixed remain fixture-measured with limitation, identifier
preservation stays 14/15, and handwriting still needs benchmark data.

Read-only license review admits FastALPR/FastPlate/open-image code under MIT and
PaddleOCR/PaddlePaddle/Tesseract code under Apache-2.0 with notices. Production
remains conditional for the open-image and CCT model release assets and the
three PP-OCRv5 checkpoint directories because checkpoint-specific/source
receipts are incomplete. Video is `LEGAL_REVIEW_REQUIRED`: local plate-detector
revision is unrecoverable, repository MIT is not checkpoint proof, and
Ultralytics documents AGPL-3.0 or an applicable commercial license for its code
and trained models. No enterprise agreement or approved AGPL posture is recorded.

Combined and partial activation packages are prepared but not activation-ready;
no asset copy/build/recreation occurred. Exact next boundary is checkpoint-rights
closure, then explicit partial Image ANPR + printed OCR activation with Video
disabled. The consumed holdout must never be rerun; a future Video candidate
needs a new independent population, not more preprocessing. Authority:
`reports/nexusai-nxmmr-video-v22-reserved-license-activation-readiness-20260831.md`.

## 2026-08-31 Raw-color parity complete; RAW_RGB selected and V2.2 frozen

The user approved exactly RAW_BGR_PARITY (diagnostic declared-RGB deviation)
and RAW_RGB (channel-permuted contract-correct color), with no third arm or
other model/detector/frame/association/aggregation/normalization policy change.
Original V2 and V2.1 source, input locks and evidence are preserved byte-for-byte.
The new versioned processor records upstream detection/containment/crop traces
so the arms can prove input parity before attributing differences to OCR.
Selection order and a conservative pre-result BGR materiality convention are
recorded in the private `raw-color-parity-20260831/registration.json`: unchanged
utility gates first; prefer eligible RGB unless BGR gains >=0.05 exact F1 and
>=2 exact matches distributed across >=2 events, with upstream parity and
source semantics review. This is not a model threshold or significance claim.
Tests precede both offline smokes; both smokes must pass before either sequential
development arm. Reuse the same validated development-only projection/runtime.
Reserved evaluation, deployment, live/retained/Activity/DB/volume work, downloads,
image/Paddle changes and NX-B2.1 remain unauthorized. Freeze at most one V2.2
only after development admission, then request approval naming that new digest.

All 68 contract/regression tests pass. Runtime imports match all 77 effective
distributions and the admitted interpreter hash. Both sequential offline
synthetic smokes passed through the actual processor with the same source-crop
hash, complete resource receipts and no ONNX shape error. BGR supplied bytes
equal the source; RGB supplies the exact channel permutation. No arm was
repaired or repeated. Development now proceeds BGR then RGB on the same locked
133-frame development schedule; no selection before both complete.

Both sequential development runs completed without failed frames or safety
stops. Source frames, detector boxes/confidences/classes, vehicle containment,
crop bounds/shapes and source BGR crop hashes are identical; deterministic
rescoring and stage-FPR recomputation pass. Each arm analyzed 133 frames / 222
OCR calls. Both score event recall 1.0, exact TP/FP/FN 19/2/5 (precision
0.904762, recall 0.791667, F1 0.844445, CER 0.178571), and group TP/FP/FN
15/1/2 (precision 0.9375, recall 0.882353, F1 0.909091). Raw/OCR/group-selected
negative-window FPR is 61/72, 59/72 and 47/72. Fifteen matched groups have
boundary MAE/median/p95 0.786367/0.512/2.87705 seconds; no catastrophic
source-time regression. Both numeric utility gates pass.

Raw exact OCR appears in both arms for all 11 events and 20/24 truth
occurrences; no event or truth occurrence is BGR-only or RGB-only. For each arm,
19 truth occurrences are exactly selected, four never generate an exact raw
reading and one generates an exact reading that aggregation does not retain.
Raw-BGR has no exact-F1, TP-count or distributed-event advantage, so it fails
every pre-registered materiality component. `RAW_RGB_SELECTED` follows the
declared preference and records `OCRInputContract=PASS`; no installed-source
semantics inspection was needed because no BGR win was proposed.

The selected development-only configuration is
`configuration/nxmmr_video_anpr_product_baseline_v22.json`. V2.2 is frozen as
`development_frozen_reserved_unevaluated` with canonical digest
`b0caae6d646e9d188ec5bc387585ac43aa7f1e22504d5b2d741699bab8d9f394`.
The freeze binds the 17 registered sources, config, comparison/audit receipts,
models, CPython 3.11.15 interpreter, immutable base, top-level lock, all 77
effective distributions, policies and decision limits. Original V2 remains
`FUNCTIONAL_OCR_INPUT_CONTRACT_FAIL`; V2.1 remains
`FUNCTIONAL_PASS_DEVELOPMENT_UTILITY_FAIL` and unfrozen.

Reserved evaluation count is zero. No tuning, model/data/package acquisition,
repeat, third arm, deployment, live/retained/Activity/database/volume work,
image/Paddle change or NX-B2.1 occurred. V2.2 is not `PRODUCT_CERTIFIED`;
license and live API/Ask/Data/citation/Activity/UI/manual acceptance gates remain
open. Exact next action is to request explicit one-time sealed-reserved authority
for only the new V2.2 digest, forbidding tuning, repeats and substitution.
Authority: `reports/nexusai-nxmmr-raw-color-parity-20260831.md`.

## 2026-08-31 V2.1 correction complete; development utility FAIL; reserved preserved

The user approved only the Video V2.1 channel-contract correction and evaluator
failure-resource finalization. Reserved evaluation is explicitly unauthorized,
even after a development PASS. V2 source/config/freeze and its failed receipts
remain intact. A versioned processor now wraps the same V2 threshold-64 path
with validated, pixel-identical RGB replication and explicit transform
provenance. Models, cadence, thresholds, grouping and normalization are unchanged.
The evaluator uses try/finally resource persistence without masking the original
error and separately refuses V2.1 reserved launches. No image/Urdu/Paddle or
live processing path is changed. Focused synthetic tests precede offline model
smoke; development alone follows only if smoke passes. New candidate freeze is
permitted only after passing development; license/NOTICE and all product gates
remain deferred. `ReservedEvaluationCount=0`; no new download or live mutation.

All 12 new pixel/shape/provenance/resource tests and 48 existing evaluator,
scorer, aggregation, source-time and vertical tests pass. The offline import
fingerprint matches all 77 effective distributions and the exact interpreter.
Synthetic smoke loaded both detectors, ran a zero frame, then forced synthetic
boxes to exercise the actual crop/threshold/adapter/recognizer path; FastPlate
received contiguous 64x128x3 uint8 and completed without a shape error. No OCR
text correctness was scored. Success-path resource finalization passed; host
minimum available RAM was 4.986732 GiB.

The one development-only run completed 133 frames / 222 OCR calls / 215 raw
observations with no failed frames. Event TP/FN 8/3 (recall 0.727273), exact
TP/FP/FN 9/2/15 (precision 0.818182, recall 0.375, F1 0.514286, CER 0.559524),
and unique group TP/FP/FN 8/2/9 (precision 0.8, recall 0.470588, F1 0.592592).
Event recall, exact F1 and group F1 fail the pre-registered utility gates.
`VideoV21Development=FAIL`; functional success is not quality admission.

Raw detector / raw OCR / group-selected negative-frame FPR is respectively
61/72=0.847222, 58/72=0.805556 and 17/72=0.236111. Two false unique strings
and three string-plus-time-false packets remain. Eight matched groups have
boundary MAE/median/p95 0.857438/0.6385/2.05925 seconds. Of 15 exact misses,
10 lack any exact raw observation and 5 had an exact raw reading not selected.
All 11 events had raw OCR, but 3 had no selected group output. No tuning ran.

Process peak RSS was 801.968750 MiB, host minimum available RAM 4.396755 GiB,
processing wall 363.506403 seconds / CPU 1028.834615 seconds. Resource receipts
finalized, no safety stop occurred, and incremental peak plus 1.5-GiB headroom
is 2.256527 GiB for this run; the 3-GiB/1.5-GiB/6-GiB policy stays unchanged.
The completed-frame audit found zero reserved-exclusion timestamps; saved-output
recomputation exactly matched the unchanged primary scorer and FPR receipts.

`VideoV21Frozen=false`; `VideoV21FreezeDigest=NONE`; reserved count remains zero.
No V2.1 candidate config/freeze was created because development failed. Original
V2 source/config/freeze and failure evidence are unchanged. All five protected
live IDs remain unchanged and temporary containers exited. No live, retained,
Activity, database, volume or image/general-OCR changes occurred.

Exact next action: request one separately versioned raw-BGR development parity
candidate with all other pins fixed, comparing against these saved V2.1 results.
This proposed preprocessing change is NOT authorized or implemented here; no
new model, tuning, reserved evaluation or NX-B2.1. Alternatively retain the
thresholded policy's FAIL status pending a different bounded investigation.
Authority: `reports/nexusai-nxmmr-video-v21-development-20260831.md`.

## 2026-08-31 Isolated exact runtime admitted; frozen OCR functional gate failed

The user explicitly approved one private Python 3.11 CPU evaluator and exact
package acquisition from official PyTorch CPU/PyPI sources, without Docker
builds, model downloads, personal-venv changes or live mutations. The existing
read-only worker image verifies Linux x86_64 / CPython 3.11.15. Resolution kept
all frozen pins and the existing base dependencies constrained. Sixteen
compatible wheels (315,352,106 bytes) were acquired and hash-verified; exact
filenames, tags, URLs, sizes and hashes are recorded in the private wheelhouse
manifest. Offline installation and pip-check passed in the one private venv at
`local-acceptance-models/nxmmr/private-runtime/video-v2-cp311-20260831`.

The added fixed-candidate evaluator uses the actual frozen product factory,
threshold-64 frame processor and unchanged scorer; it has no selection grid.
The host monitor enforces 3.0-GiB initial admission and 1.5-GiB available-RAM
stop. Ten non-oracle tests pass. Exact import/full-lock/hash admission passed,
with CPU-only Torch and no models loaded in that phase; import RSS peak was
351.141 MiB and host minimum available RAM 5.253571 GiB. Fontconfig emitted
cache-directory warnings without import failure or download.

One network-disabled synthetic smoke loaded both pinned YOLO detectors and the
FastPlate model and completed the zero-frame processor call. The supplemental
OCR-interface call failed: the frozen threshold-64 processor emits a 2-D crop,
but the pinned RGB model expects 64x128x3. ONNX rejected the dimensions. No
candidate change or retry occurred. `FrozenRuntimeDevelopmentGate=FAIL` records
this mandatory functional prerequisite failure; development accuracy is
`NOT_RUN`, not an observed accuracy drop. `MATERIAL_RUNTIME_DELTA` is functional.
No real oracle projection or development/reserved scoring ran;
`ReservedEvaluationCount=0` and `CandidateFreezeUnchanged=true`.

Smoke host RAM was 5.332481 GiB before / 5.011143 GiB minimum, with 59.062876
seconds including container startup and no safety stop. Exception-path process
RSS/CPU finalization is missing and must not be replaced with import-only RSS.
All temporary containers exited; all five protected live IDs are unchanged.
No live/retained/Activity/database/volume mutation or model download occurred.

Exact next action: request one V2.1 development-only OCR input-contract fix
(preserve thresholded pixels, adapt channels for the RGB model), preserving
all model/policy pins and the current failed freeze. Include exception-resource
persistence, synthetic smoke and development comparability; new candidate
reserved admission needs explicit approval. This correction is proposed, not
implemented. License review remains after technical scoring. All live/product
acceptance gates and the separate 6-GiB deployment policy are unchanged.
Authority: `reports/nexusai-nxmmr-video-v2-isolated-runtime-provisioning-20260831.md`.

## 2026-08-31 Exact-runtime comparability: RAM override recorded; runtime missing

The new directive supersedes only the isolated Video V2 evaluator's provisional
4.5-GiB floor: `VIDEO_V2_LOCAL_EVALUATOR` now requires 3.0 GiB before model load,
one evaluator, and a 1.5-GiB absolute available-RAM stop. Actual host checks
were 4.624435 and 4.960541 GiB; RAM is not the current blocker. The separate
6-GiB build/deployment policy is unchanged. Scoped policy:
`configuration/nxmmr_video_v2_local_evaluator_policy.json`.

All 24 frozen hash/policy checks pass, but no inspected local runtime matches
the frozen Python 3.11 / Torch 2.5.1+cpu / Torchvision 0.20.1+cpu /
Ultralytics 8.0.114 / NumPy 2.3.5 combination. The existing worker image has
Torch 2.6.0 and lacks Torchvision/Ultralytics. Existing evaluator images also
fail the pins. Matching Torch/Torchvision in the personal venv and pip cache
are CPython 3.10 Windows binaries, not compatible Python 3.11 dependencies.
Do not substitute versions, stitch incompatible runtimes, or change the freeze.

No model or video inference ran; no reserved oracle/result row was read.
`ReservedEvaluationCount=0`, `CandidateFreezeUnchanged=true`, and
`PostHocTuning=false`. Development quality/parity remains unmeasured, not FAIL.
Three temporary network-disabled read-only metadata-only inventory containers
exited and were removed; all five live container IDs remain unchanged. No
downloads/installations/builds/live/retained/Activity/DB/volume mutation.

Exact next action: obtain an already-prepared matching runtime or explicit
approval for isolated pinned dependency acquisition, with no model downloads,
Docker build or live changes. Then run the exact thresholded development path;
only a passing development gate may consume the reserved evaluation. Preserve
the raw-BGR versus threshold-64 distinction and label 0.652778 as group-selected
negative-frame FPR. License review remains ordered after scoring/decision.
Image/OCR states are preserved and NX-B2.1 remains paused. Authority:
`reports/nexusai-nxmmr-video-v2-exact-runtime-comparability-20260831.md`.

## 2026-08-31 Video V2 reserved authorization recorded; preflight stopped

The user authorized one private/non-retained evaluation of frozen V2 digest
`212c00970d7bed1c40612102c11f1a0dd24c43435c406df83d80bd8f40117b20`
on only eight reserved events. All 24 hash/policy checks pass, but host free RAM
was 3.539 GiB, below the unchanged 4.5-GiB HEAVY benchmark floor. No model was
loaded, no reserved frame decoded/scored, and no reserved label opened.
`ReservedEvaluationCount=0`: authorization remains unconsumed. Runtime versions
must still be reverified before inference. No candidate decision or promotion.

Pre-scoring source audit corrects two earlier interpretations: V2 development
re-OCR used raw BGR crops, whereas the frozen integrated processor/config uses
grayscale binary-inverse threshold 64; the development 0.652778 negative-frame
FPR was computed after `group_frame_rows`, not on raw detector output. Preserve
the freeze, disclose these distinctions, and never execute the development
policy-selection loop on reserved data. No source/config/threshold was changed.

Private preflight receipt records prospective user decision gates. Resume only
after the operator restores sufficient RAM; verify exact runtime and run the
fixed frozen candidate with the unchanged scorer once. Post-evaluation license/
NOTICE review, any activation approval, and live product gates remain pending.
Image/OCR source/local states are preserved; no live/retained/Activity/DB/volume
mutation occurred and NX-B2.1 remains paused. Authority:
`reports/nexusai-nxmmr-video-v2-reserved-preflight-20260831.md`.

## 2026-08-30 NX-MMR ANPR/OCR vertical source/local closure complete

`NX-MMR-ANPR-OCR-VERTICAL-CLOSURE-V1` is source/local complete and stopped
before live activation. The recent fresh image failure was not an ingestion or
queue failure: live ANPR/OCR roles are disabled and `/models/media` is empty,
so model execution was `MODEL_REQUIRED`/`NOT_RUN`. The hash-pinned image path
passes two non-holdout positives plus a generated zero-result negative; the
sealed image holdout was not rescored.

Video V2 development uses fixed 4 FPS actual frames, the dedicated 640 plate
detector, required vehicle containment, FastPlateOCR repeated crops, and
confidence-weighted support-3 selection of an actually observed string. On 11
development events it records event recall 1.0, normalized exact F1 0.844445,
group F1 0.909091 and group precision 0.9375. Negative-frame FPR remains high
at 0.652778 and is an explicit P1 limitation. The authoritative product-source
freeze digest is
`212c00970d7bed1c40612102c11f1a0dd24c43435c406df83d80bd8f40117b20`;
the eight reserved event components remain untouched.

Paddle detector plus English and Arabic-multilingual recognizers pass a
generated 5/5/5 printed English/Urdu/mixed pack: English CER/WER 0/0, Urdu
CER/WER 0.118012/0.258065, mixed CER/WER 0.143791/0.318182. Identifier exact
is 10/10 English+mixed and 4/5 Urdu; all Urdu regions are RTL. This is fixture
evidence only, handwriting remains `BENCHMARK_DATA_REQUIRED`, and a small
independent real-world printed pack is still required.

The full forensic API suite, focused source contracts, Python compilation,
dependency-free 12-test runner and Go vet pass. No service/model/mount/retained
evidence/Activity/database/volume was changed; no operation was promoted and
NX-B2.1 remains paused. Exact next action: obtain explicit approval for one
private, local, non-retained evaluation of only the exact V2 freeze on the
locked eight reserved events, with no tuning. Production checkpoint licensing,
Paddle NOTICE packaging, consolidated activation, and all live API/Ask/Data/
citation/Activity/UI/manual gates remain separate later approvals. Authority:
`reports/nexusai-nxmmr-anpr-ocr-vertical-closure-v1-20260830.md`.

## 2026-08-30 NX-MMR parity adapter V1 frozen; evaluation approval required

`NX-MMR-REFERENCE-PARITY-ADAPTER-V1` source development is complete. A
model-output-independent interval split was frozen before inference with 11
development and 8 reserved event components; no reserved-candidate metric was
computed. The selected candidate is fixed 4 FPS, dedicated hash-pinned 640
plate detection, EasyOCR, no vehicle model, neutral majority aggregation,
minimum support 2, and one vote per actual crop. Freeze digest is
`1dcfabf17a8b66165b6c1390e65a8784eb875d3650b1b7f27d328c9ad2f0bb13`.

Development event recall improves from the incumbent's 0.090909 to 0.818182,
and exact F1 from 0.080001 to 0.105263, but exact TP/FP/FN remains 2/12/22 and
group TP/FP/FN 1/17/16. The adapter is therefore `PARTIAL`, not promotable.
Vehicle context is optional, persistent tracking/interpolation/UK correction
are absent, raw OCR and source-time provenance are preserved, and the live
processor factory/routing remains unchanged. Model-license admission and
development precision/group quality remain P1.

No model/data was downloaded, no service was built/deployed/recreated, no
retained evidence/Activity/database/volume was mutated, and NX-B2.1 remains
paused. Exact next action: obtain explicit approval for one one-time local,
non-retained evaluation of the exact frozen candidate on the 8 reserved event
components. Do not tune, alter the freeze, rescore the image holdout, activate
routing, deploy, promote, or start NX-B2.1.

## 2026-08-30 NX-MMR reference parity complete; video adapter is next

Read-only inspection located both pre-existing personal reference pipelines and
confirmed exact byte identity between their `sample.mp4` and the locked-oracle
input. The image reference has the same FastALPR model assets, relevant package
versions and effective inference semantics as `CURRENT_NXMMR_BASELINE_V1`.
Frozen reference predictions agree exactly on all 10 development and all 22
one-time holdout images. Keep the current image baseline; remaining image errors
are OCR capability failures, not a pipeline regression.

The historical dense video reference is materially better than the sparse
incumbent: 18/19 versus 3/19 event detection and 24/41 versus 3/41 final exact
plate matches. Its useful components are dense/repeated observations, the
existing dedicated 640-input plate detector and optional vehicle ROI. UK-only
global normalization, persistent SORT identity and interpolated observations
are excluded from product authority. Historical output was evaluated against
the locked human oracle and was never used as truth. The exact rerun was stopped
before model load at the 4.5 GiB HEAVY memory gate.

No model/dataset was downloaded, no service was built/deployed/recreated, and
live models, retained evidence, Activity, database, volumes and NX-B2.1 are
unchanged. No certification state was promoted. Exact next action: implement
only the non-retained, source-level `NX-MMR-REFERENCE-PARITY-ADAPTER-V1`
development slice with already-local hash-pinned assets, preserve raw
observations, use bounded ephemeral adjacent-frame aggregation, freeze it on
development evidence, and return for explicit approval before candidate
evaluation or activation.

## 2026-08-30 NX-MMR human-gold ANPR scoring complete; no promotion

The independently labeled pack is locked with `GroundTruthValidation=PASS` and
gold digest
`88c6827c23fbed467f097843404ed09c947bf7f27432e5cb5fa8427be9f48567`.
The frozen incumbent ran development first and the sealed holdout once with no
configuration change. Development detected 10/10 and recognized 5/10 exactly;
holdout detected 20/22 and recognized 13/22 exactly. Both image splits contain
only positives, so image specificity remains unmeasured.

The fixed one-second `sample.mp4` policy detected 3/19 human events and exactly
recognized 3/41 plate occurrences. It produced one false-positive temporal gap
among 26 negative sampled frames, 31 misses among 34 positive sampled frames,
and 4/29 exact-value groups with 1.178250-second mean absolute boundary error.
This blocks video-ANPR promotion. Two events/four plates were cadence-unsampled,
but 14 further events/30 plates had sampled frames and no detection.

All six human speech windows are negative with no transcripts, so
`faster-whisper-small-ur` was not called and WER remains inadmissible. The
isolated runs peaked at 279.691 MiB RSS; host free RAM remained above the
3.5 GiB MEDIUM floor. Live container identities, services, models, retained
state, Activity, database and volumes are unchanged. No operation or model was
promoted, NX-B2.1 remains paused, and all live API/Ask/Data/citation/Activity/
UI/manual product-acceptance gates remain mandatory.

## 2026-08-29 NX-MMR pre-approval program complete; consequential work stopped

NX-B2.0 remains complete and NX-B2.1 is paused. NX-MMR is the active bounded
pre-approval program. Fresh read-only reconciliation confirms source at 79
operations/214 variants versus live at 67/68 and catalog
`2026-08-23.post-bfa-runtime.1`. The live worker media-model mount is empty,
so retained multimedia artifacts remain queryable but fresh ANPR/OCR/SigLIP
processing is not model-ready. LocalAI exposes five installed models and eight
backend registrations; `/v1/models/capabilities` returns 404 live.

The required eight NX-MMR reports, manual UI acceptance guide and
`NEXUSAI_MULTIMODAL_BENCHMARK_MANIFEST_V1` are complete. The exact proposed
isolated seven-file model bundle is 663,933,265 bytes. No public dataset
download is proposed at the first boundary; P-LPCD (1,235,177,299 bytes), full
Common Voice Urdu (5.79 GB), Mendeley Urdu handwriting and research-only UTRNet
remain deferred for explicit reasons. The two exact local user-controlled
sources total 234,784,354 bytes and were detected but not processed.

NX-MMR also closes a source-level product-truth defect: ordinary visible
suggestions now require `PRODUCT_CERTIFIED`. The 79-entry generated ledger is
62 `REGISTERED`, 12 `SOURCE_VALIDATED`, one `FIXTURE_CERTIFIED`, and four
`PRODUCT_CERTIFIED`; only the four product-certified operations are ledger-
eligible, and capability materialization conservatively exposes the two CDR
prompts with explicit mappings. Focused Go gates pass in 10.916 seconds. This
source change is not deployed.

No model/data download, install, build, deployment, restart, upload,
reprocessing, database/profile/backend change, retained mutation, stage,
commit, push or PR occurred. Final free physical RAM measured 3,733,495,808
bytes, below the 6 GiB heavy-run/deployment gate. Exact next action: wait for
explicit approval of the consolidated NX-MMR acquisition, local non-retained
processing and isolated benchmark boundary. NX-B2.1 remains paused.

## 2026-08-28 NX-B2.0 live/source reconciliation complete; activation gate closed

NX-B2 is active at the read-only preflight gate. Live forensic API introspection
returns 67 executable templates, platform catalogue
`2026-08-23.post-bfa-runtime.1`, 98 platform operation descriptors and 68 query-
corpus entries. None of the 12 NX-B1 operations is live. Current source remains
79 executable operations, reconciled 104-operation projection and 214 governed
query variants. B1 runtime activation is therefore required before retained
NX-B2 certification.

Current healthy runtime identities were re-detected: LocalAI/UI
`321ba681774b...` on `sha256:628f56af541a...`; forensic API
`b5c13f96e67f...` on `sha256:d2992062b8f5...`; worker
`296e0881d891...` on `sha256:be13dbec4bd9...`; PostgreSQL
`f66e05a3b179...`; NATS `5c49e70d132a...`. API and worker use
`unless-stopped`; all service health/readiness probes pass. The retained tuple
is unchanged at `51|51|64|0|22207|441|47`; the primary acceptance workspace
contains `24|24|9275|426|22` evidence, versions, rows, artifacts and KB assets,
and its current Activity count is 20.

Source ownership requires a bounded three-service activation: forensic API for
the 79-operation catalogue/executors, LocalAI/UI `api` for the updated agent
routing/presentation adapter, and worker for its embedded
`2026-08-28.nxb1.1` platform catalogue. PostgreSQL, NATS, models and named
volumes remain protected. The runtime env/hash and History-URL presence resolve
without secret disclosure, and the accepted Compose topology passes.

No activation was attempted. Fresh free physical RAM is 3.201 GiB versus the
mandatory 6 GiB gate, and this invocation contains no separate deployment
approval. `DeploymentGate=CLOSED`, `B1RuntimeActivated=false`,
`RetainedStateMutated=false`, `ModelsChanged=false`,
`DatabaseMigration=false`, `VolumesChanged=false`, `OpenP0=0`, and
`OpenNXB2P1=1` (`NXB2-P1-001`: NX-B1 operations unavailable live). Exact next
action: restore at least 6 GiB free RAM, obtain explicit approval for the
rollback-tagged three-service activation, then begin NX-B2.1. Preflight record:
`reports/nexusai-nxb2-activation-preflight-20260828.md`.

## 2026-08-28 NX-B1 all-major-family breadth baseline source-complete

NX-B1 is source-complete. The required gap inventory was recorded first at
`reports/nexusai-nxb1-family-gap-inventory-20260828.md`. Twelve bounded
operations were then added across financial, access/security logs, generic
structured, documents, images, audio and video, increasing the authoritative
executable catalogue from 67 to 79 operations. Exposure remains truthful:
A=5 certified/queryable, B=63 limited, C=11 engineering-only. The reconciled
platform catalogue is version `2026-08-28.nxb1.1` with 104 descriptors.

All operations use the existing NX-A1 plan, validation, capability/readiness,
Tool Result and Answer Envelope foundation. Ask and typed case-query actions
resolve the same operation. Structured and metadata results retain
deterministic authority; document/image/audio derived search retains semantic-
retrieval authority; video timeline rows are cited model observations in an
Observation Packet. Complete-zero, no-match, processing/not-processed and
scope boundaries remain distinct. No unrestricted SQL surface was introduced.

Focused tests and the final full `api/forensic_records` suite pass (85.579s).
The standalone agent routing/presentation gate passes; the full agent suite
reaches 105 passing specs and 14 environment-only failures before assertions
because rootless Docker is unavailable on this Windows host. New
operations remain Tier B limited and suggestion-ineligible until NX-B2 runtime
and integrated certification. Financial is explicitly partial: currency-
separated transaction aggregation is supported, but exchange conversion,
ownership, fraud, motive and criminality are not inferred.

No service, image, container, model, profile, backend, database schema,
retained evidence, retained Activity, named volume or runtime state was
changed. No deployment, restart, migration, upload, reprocessing, stage,
commit, push or PR occurred. `NXB1=COMPLETE`, `OpenP0=0`,
`OpenNXB1SourceP1=0`. Exact next phase: NX-B2; do not start it without a new
explicit directive.

## 2026-08-25 MMV-2 enterprise-closure activation passed; live closure remains partial

The approved narrow activation passed with LocalAI/UI image
`sha256:f255f1889757...` in container `c4495d82b714...` and forensic API image
`sha256:c0f07382ab7d...` in container `4aad36646278...`. Worker
`1846f558941f...`, NATS `5c49e70d132a...`, and PostgreSQL
`f66e05a3b179...` preserved their exact IDs. All health checks pass, required
volumes and models remain present, and retained truth remains
`51 | 51 | 64 | 0 | 22,207 | 441 | 47`.

Live certification proves the positive enterprise-summary fix across accepted
English, natural-English, Roman Urdu, and Urdu queries: each returns one exact
`MN1367` result, operation `anpr.sightings`, and the preserved source citation.
Positive History save/reopen, responsive layout at 390/820/1024/1440 with zero
horizontal overflow or console errors, video Data complete-zero presentation,
and the live audio row/player all pass.

MMV-2 cannot close. The public case adapter reports `row_count: 1` for
authoritative zero-row no-match and complete-zero results; the grouped-video
Data quick action loses its typed complete-zero state in Ask and History; and a
natural grouped-video prompt containing the retained UUID can misroute UUID
segment `BAB0` as a plate target. These are three newly exposed foundational
P1s requiring separate source and deployment approval. No upload, retained
reprocessing, model change, migration, cleanup, or MMV-3 work occurred.
Evidence is
`reports/mmv2-anpr-image-video-20260824/enterprise-closure-live-certification-20260825.json`.
Current markers: `MMV2EnterpriseClosureActivation=PASS`,
`PositiveEnterpriseSummary=PASS`, `CitationCertification=PASS`,
`AudioPresentation=PASS`, `MediaQueryCertification=PARTIAL`,
`HistoryCertification=PARTIAL`, `BrowserAcceptance=PARTIAL`, `OpenP0=0`,
`OpenFoundationalP1=3`. Do not begin MMV-3.

## 2026-08-25 MMV-2 final P1 activation passed; live certification found two presentation P1s

The guarded final P1 activation passed. LocalAI/UI now runs image
`sha256:13b37779c63c...` in container `0898847fda28...`; forensic API runs
`sha256:abce6567f187...` in container `38a5040f82f3...`. Worker
`1846f558941f...`, NATS `5c49e70d132a...` and PostgreSQL
`f66e05a3b179...` retained their exact IDs. All health checks pass and retained
truth remains `51 | 51 | 64 | 0 | 22,207 | 441 | 47`.

Live routing proves the Urdu-script fix: English, natural English, Roman Urdu
and Urdu all route to `anpr.sightings` for exact plate `MN1367` with one exact
row and citation. The new `video.anpr_grouped_timeline` operation is publicly
executable; its raw governed operation correctly returns `complete_zero` and
`ANPR processing complete, 0 plate groups detected.` History reopen, citation
detail and 390/820/1024/1440 browser acceptance pass; completed audio playback
acceptance remains reusable and its focused presentation regression passes.

Final closure is nevertheless PARTIAL with two newly exposed foundational P1s.
The enterprise summary passes `nil` rows to its no-results test, so every
positive ANPR enterprise answer falsely says no matching records despite the
rendered exact row and citation. The public grouped-video enterprise adapter
also collapses raw `complete_zero` into generic `no_results` and omits the
operation tool identity. No source correction or deployment is authorized in
this phase; request a separate narrow source/deployment approval. Evidence is
`reports/mmv2-anpr-image-video-20260824/final-p1-live-certification-20260825.json`.
Current markers: `MMV2Activation=PASS`, `PublicMediaOperations=PARTIAL`,
`MediaQueryCertification=PARTIAL`, `CitationCertification=PASS`,
`HistoryCertification=PASS`, `BrowserAcceptance=PASS`,
`AudioPresentation=PASS`, `OpenP0=0`, `OpenFoundationalP1=2`. Do not begin
MMV-3.

## 2026-08-25 MMV-2 final P1 offline activation procedure prepared; not executed

The approved final P1 activation remains deliberately unexecuted in this
session. The PowerShell 5.1 procedure is
`scripts/activate_nexusai_mmv2_final_p1.ps1`; its read-only preflight enforces
the exact branch, HEAD, source/checkpoint hashes, current container/image IDs,
zero-active-job gate, health, retained counts, mounts, volumes and mandatory
6 GiB free-physical-RAM threshold. The activation path creates new exact
rollback tags for current forensic API and LocalAI/UI images, then builds and
recreates only `forensic-records-api` and `api` with `--no-deps`. Worker,
PostgreSQL and NATS IDs are protected, and rollback is armed only after service
recreation begins. Windows PowerShell 5.1 AST validation and prohibited-command
and mutation-scope scans pass. No build, tag, restart, recreation, retained
mutation, model change, migration, upload, reprocess or Git publication action
was performed. Resume after reboot with the script preflight, then activation;
do not begin MMV-3.

The operator's first post-reboot run passed the full read-only preflight at
6.01 GiB and reconfirmed all protected IDs, health checks, zero active jobs,
mounts, volumes and retained truth (`51 | 51 | 64 | 0 | 22,207 | 441 | 47`).
The activation then failed safely before tag creation, build or recreation
because Windows PowerShell 5.1 promoted expected stderr from probing a missing
rollback tag with `docker image inspect` into a terminating error. The script
now uses the exact-reference, empty-success `docker image ls --quiet
--no-trunc` probe. That replacement passes under Windows PowerShell 5.1; the
revised script SHA-256 is
`fbb02146e6ff20ee044dc9383a18d63f7d6b8b7d7e5eba67e77f85c2f71ac246`.
No runtime or retained state changed.

The next retry passed preflight, created both approved exact rollback tags and
passed all focused source smokes, then stopped during the forensic API build
because normal BuildKit stderr was promoted by the outer Windows PowerShell
5.1 `Tee-Object` pipeline. Independent read-only reconciliation proves the
build did not replace either runtime image and no container was recreated: all
five exact IDs and running images remain unchanged, all health endpoints return
200, and retained truth remains `51 | 51 | 64 | 0 | 22,207 | 441 | 47`. Only
the two valid rollback tags now exist. The native-command wrapper now merges
and streams native stdout/stderr within the child process while continuing to
fail on nonzero exit status. Windows PowerShell 5.1 syntax, simulated native
stderr streaming and forbidden-operation scans pass. The revised script hash
is `4a6f5f108045baff20d3c28689d169aee0cf5e466556f8dc5b50b673b10593fe`.

The following attempt failed closed at the mandatory RAM gate: read-only
preflight observed 5.38 GiB and the separately entered activation command
observed 5.36 GiB. Both stopped before source/runtime inspection, tag changes,
build or recreation; the activation emitted `FAILED_NO_RECREATION` and
`RuntimeServiceMutation=false`. The operator should next use one guarded
PowerShell block whose activation branch is unreachable unless preflight exits
zero. Existing verified rollback tags remain available; script content and
SHA-256 are unchanged.

## 2026-08-24 MMV-2 final P1 source closure complete; narrow live activation gated

The two remaining foundational MMV-2 gaps are now fixed in source. Generic
Urdu-script ANPR intent routes through the same governed operations as English
and Roman Urdu while ASCII identifiers remain unchanged. Exactly one public
grouped-video operation, `video.anpr_grouped_timeline`, is registered through
the case-scoped Ask/Data path with explicit evidence UUID, optional plate and
source-second bounds, citation locators, review state and non-tracking
semantics. A completed retained video with no group artifacts returns `ANPR
processing complete, 0 plate groups detected.`

Full forensic API tests, focused agent tests, Go vet, 10/10 media-presentation
tests, focused ESLint and the React production build pass. Production source
contains none of the forbidden demo plate identifiers. The live runtime was
not changed: `51 evidence | 51 versions | 64 jobs | 0 active | 22,207 records
| 441 artifacts | 47 KB assets`; worker, PostgreSQL and NATS remain unaffected.

The required activation set is forensic API plus LocalAI/UI only. Existing
rollback tags do not point to the current running images, so deployment must
first create new rollback tags for API image `sha256:9814cdf0...` and LocalAI/UI
image `sha256:f08bb832...`, then pass the mandatory 6 GiB free-physical-RAM
gate. Deployment, live multilingual/citation/History/four-viewport browser
certification, and any retained positive-video upload remain unapproved.
`sample.mp4` still matches the expected hash and size but ownership/retention
attestation and visual candidate review remain gates. Do not begin MMV-3.

## 2026-08-24 MMV-2 second activation attempt passed product gates; display-only failure rolled back cleanly

The retry passed preflight at 6.67 GiB and repeated the exact two cached builds,
pre-replacement smokes, narrow recreation, protected-ID verification, complete
health checks (including Analyst Portal HTML 200), mount preservation and
retained-count reconciliation. The deployed MMV-2 product state was healthy and
retained truth was unchanged. A final evidence-display-only `docker image
inspect --format` expression used a nested Go-template string literal that
Windows PowerShell 5.1 stripped at the native-command boundary; Docker returned
template exit 64, and the conservative procedure rolled back both affected
services.

This rollback completed fully with `MMV2NarrowActivation=FAILED_ROLLED_BACK`,
unchanged counts and exact rollback images. Current verified affected IDs are
LocalAI `d4207db67d15` and worker `4aaf9b56ca22`; unaffected API/NATS/PostgreSQL
IDs remain unchanged. The display uses Docker's quote-free `json` helper now,
and the exact five-image invocation passes under Windows PowerShell 5.1. The
current build tags remain available. No retained mutation, model change,
migration, upload or reprocessing occurred. One final guarded retry remains in
MMV-2; do not begin MMV-3.

## 2026-08-24 MMV-2 activation attempt built successfully; verifier-triggered rollback recovered

The operator's guarded preflight passed at 6.97 GiB free RAM with every source
hash, protected ID, health endpoint, zero-active-job gate, retained count and
rollback image verified. The exact worker and LocalAI/UI builds passed and
produced `sha256:1adc9880176ae6c4b1999b35ca7009cbda7a81dd8ca9bdae2f9ab05c357a4740`
and `sha256:951e4f3f5df790555da492d64b3c4f4a881217bc5802e21cbe229f4b79244786`.
Both pre-replacement smokes passed and only `api` plus
`forensic-records-worker` were recreated; PostgreSQL, NATS and forensic API
retained their exact IDs.

The deployed product was ready/healthy, but the verifier requested the Analyst
SPA route without `Accept: text/html`; the intentional HTML-only SPA fallback
therefore returned the API 404 representation and triggered guarded rollback.
Rollback recreated both affected services on their exact preserved image IDs.
Its final mount check then falsely classified Docker Desktop's equivalent
`/run/desktop/mnt/host/c/...` and `C:\...` bind-source spellings as different.
Independent read-only recovery reconciliation proves LocalAI is running
`sha256:afc6ac...`, worker is running `sha256:4b345f...`, both are healthy,
all public health endpoints return 200, the Analyst route returns 200 with an
HTML Accept header, and retained truth remains `51 | 51 | 64 | 0 | 22,207 |
441 | 47`. Unaffected IDs remain forensic API `a97e545b8356`, NATS
`5c49e70d132a`, and PostgreSQL `f66e05a3b179`.

The retry procedure now sends the correct SPA Accept header, canonicalizes
equivalent Docker Desktop bind paths, and expects the verified post-rollback
LocalAI `0f13e3115fe5` and worker `36f564778af2` container IDs. Both fixes pass
under Windows PowerShell 5.1. No retained mutation, migration, model change,
upload or reprocessing occurred. MMV-2 activation remains pending one guarded
retry; do not begin MMV-3.

## 2026-08-24 MMV-2 manual post-reboot activation procedure prepared; activation not run

At the product owner's direction, no MMV-2 build, restart, recreation or
deployment was performed. The exact guarded Windows procedure is now
`scripts/activate_nexusai_mmv2_narrow.ps1`; it has a read-only
`-PreflightOnly` mode, embeds the verified source/checkpoint hashes, rebuilds
only `forensic-records-worker` and `api`, reuses the two existing MMV-2
rollback tags, preserves the three unaffected protected container IDs, and
automatically restores the exact rollback images if a failure occurs after
recreation.

PowerShell AST parsing, prohibited-operation scanning, 44 readable embedded
checkpoint hashes plus the protected runtime-env hash, both exact Compose
merges, current rollback IDs, and current runtime state were revalidated.
Retained truth is still `51 evidence | 51 versions | 64 jobs | 0 active |
22,207 canonical records | 441 artifacts | 47 KB assets`; LocalAI, forensic
API, worker, NATS and PostgreSQL are healthy. The read-only procedure check
failed closed as designed at `2.24 GiB` free physical RAM versus the mandatory
`6.00 GiB` threshold, with marker `MMV2NarrowActivation=SAFE_STOP_RAM_GATE`.
No container/image/data/model/database state changed. Resume after reboot by
running the script locally; remain in MMV-2 for post-activation operation,
query, citation, History and four-viewport browser certification.

The operator subsequently stopped only the two unbound historical LocalAI
containers (`nexusai-api-pre-phase3-20260730t055216z` and
`nexusai-api-rollback-20260724`) reversibly, releasing enough RAM for the source
gate. The next read-only preflight reached the exact protected-container IDs
and exposed a Windows PowerShell 5.1 JSON-array enumeration incompatibility
before health/count/build/recreation. The procedure now parses environment
entries line-by-line and explicitly enumerates mount arrays; both corrected
paths passed read-only PowerShell 5.1 validation without emitting secrets. A
second read-only run passed all service health and retained-count gates, then
exposed Docker's omitted optional `Name` field on bind mounts under StrictMode;
the mount normalizer now handles unnamed bind mounts explicitly and passed for
the five LocalAI mounts and two worker mounts. No MMV-2 build or activation
occurred during either correction.

## 2026-08-24 MMV-2 narrow activation preflight failed closed at RAM gate

The product owner approved a narrow worker plus LocalAI/UI MMV-2 activation.
Source-diff reconciliation found no MMV-2 forensic API delta, so the smallest
rebuild set is `forensic-records-worker` and `api` (LocalAI/UI). Fresh rollback
tags preserve their exact active images:
`nexusai/forensic-records-worker:rollback-before-mmv2-activation-20260824` and
`nexusai/localai-forensic:rollback-before-mmv2-activation-20260824`.

The mandatory 6 GiB guarded build threshold failed before any build. Initial
free RAM was 2.68 GiB. A reversible stop of the five runtime containers, with
no removal or Compose down, observed at most 2.56 GiB and ended at 2.08 GiB.
No image was built/replaced and no container was recreated. The exact original
containers were restarted: LocalAI `e04dae4cbd44`, forensic API
`a97e545b8356`, worker `0b2418d90cbd`, NATS `5c49e70d132a`, PostgreSQL
`f66e05a3b179`. LocalAI `/readyz`, forensic API `/healthz`, worker metrics and
NATS health all return 200; PostgreSQL is healthy.

Retained truth remains `51 evidence | 51 versions | 64 jobs (60 completed, 4
dead-letter, 0 active) | 22,207 canonical records | 441 artifacts | 47 KB
assets`. No retained mutation, reprocess, upload, model download, migration,
cleanup, stage, commit or deployment occurred. Resume only after the host has at
least 6 GiB free RAM and Docker is ready. Evidence is
`reports/mmv2-anpr-image-video-20260824/activation-preflight-20260824.json`.

## 2026-08-24 MMV-2 bounded sampling and image maturity slice source-complete

MMV-2 corrected the foundational video citation error discovered by independent
sampling: the prior FFmpeg `fps` path selected interval-internal frames, rewrote
timestamps, and labeled them as `index * interval`. Consequently the historical
MMV-1 `LN15ZZC at 25s` result is preserved as historical model output but is not
an exact source-time citation. Source now uses `select` plus `showinfo`, records
best-effort source timestamps, source-frame SHA-256 and explicitly estimated
frame numbers. An isolated worker smoke produced exact 0/5/10/15/20/25 source
timestamps and cleaned every temporary frame.

The non-retained 60-second positive benchmark selected bounded 1-second ANPR
sampling: 60 frames, 41.676 seconds, 23.28 CPU seconds, 278,632 KiB peak RSS and
523.50 MiB temporary PNGs, all cleaned. It produced review-required candidates
`MW51VSU` at 0s, `AK64DMV` at 19s, `EF10DZT` at 20s and `WG65ZFX` at 43s. The
2-second path found two candidates; the adaptive path found three. The lawful
10.5-second negative video produced zero candidates at every cadence.

Production source splits FastALPR to the 1-second/full-duration bounded cadence
(60 default, 120 hard maximum) while face/OCR/SigLIP remain on the heavier
5-second cadence. Equal normalized plate model outputs are grouped for UI
presentation with sightings count, first/last time and best/raw observation IDs;
all raw observations remain preserved and the contract explicitly says this is
not tracking. Analyst Data prefers those groups when present. Focused UI tests
pass 9/9; focused image comparison Go tests pass; the broader forensic package
has 282 passes, two skips and one unrelated existing deterministic-synthesis
expectation failure.

The 13-image Pakistan ANPR matrix is 7 PASS, 2 PARTIAL, 4 FAIL. Exact duplicate
controls pass; six same-source dHash transforms are distance 1–7 and six
unrelated controls 19–35, so threshold 10 is unchanged and remains a candidate
signal. The 16-image pinned SigLIP matrix is accepted-limited (same-scene resize
0.9741; portrait-vs-street 0.4630). No retained upload/reprocess, model download,
database migration, deployment, cleanup, or service restart occurred. Runtime
remains `51 evidence | 51 versions | 64 jobs (60 completed, 4 dead-letter, 0
active) | 22,207 canonical records | 441 artifacts | 47 KB assets`.

Artifacts are under `reports/mmv2-anpr-image-video-20260824/`; the master report
is `reports/nexusai-mmv2-anpr-image-video-20260824.md`. Retained positive proof,
deployment/live browser acceptance, and full media Ask/History operation
certification remain explicit gates. `OpenP0=0`; `OpenFoundationalP1=0` in
source; `ModelDownloadNeeded=NO`; `DatabaseMigrationNeeded=NO`.

## 2026-08-24 MMV-1 real-world validation source complete; gated work remains

MMV-1 reconciled every current evidence family without reopening accepted STIM
or mutating retained evidence. The unchanged deployed video processor passed
the approved non-retained `sample.mp4` positive control: six frames sampled at
0/5/10/15/20/25 seconds, one plate `LN15ZZC` at 25 seconds, detector confidence
`0.781484`, OCR confidence `0.999857`, and 73.244 seconds runtime. The approved
worker report is preserved at
`reports/mmv1-real-world-validation-20260824/sample-video-anpr-nonretained.json`
with SHA-256 `da880be4ddd87422a25a562f8b6a8d71ec6366c1281d3758c6de2dd3500a3fba`.

Current Tesseract `eng+urd` measured aggregate non-negative Urdu CER
`0.436842`; paragraph and mixed-script omissions make it inadequate for a
practical Urdu maturity claim. PaddleOCR
`arabic_PP-OCRv5_mobile_rec` is the official Urdu-capable MMV-3 challenger, but
no model was downloaded and its admission record remains incomplete. The
three-fixture audio pack measured natural Urdu WER/CER `0.6000/0.1852` and
`0.3333/0.1446`; the synthetic control preserved phone/time but not `MN1367`.

Two foundational presentation/correctness defects are fixed in source: audio
typing/readiness now derives from family/MIME/artifacts, and a positive later
video frame suppresses contradictory aggregate zero-result limitations. No
deployment was performed. Live read-only database reconciliation is now `51
evidence | 51 versions | 64 jobs (60 completed, 4 dead-letter, 0 active) |
22,207 canonical records | 441 artifacts | 47 KB assets`; the product
collection contains 24 evidence. The three newest product-collection images
postdate the accepted 21-source checkpoint and were not created by MMV-1.

`RetainedMutationNeeded=YES` for a future positive-video retained proof;
`DeploymentNeeded=YES` for the two source corrections;
`ModelDownloadNeeded=YES_FOR_FUTURE_URDU_OCR_CHALLENGER_ONLY`;
`DatabaseMigrationNeeded=NO`. None is authorized by the report-persistence
approval. The next phase is MMV-2 after the explicit gates are decided.

## 2026-08-23 core media pipeline productization complete; narrow activation and acceptance passed

The bounded Data-detail productization is source-complete against the existing
read-only `default/nexusai-multimodal-product-acceptance` workspace. The active
retained artifact contract `forensics.anpr-observation/v1` is now recognized;
the browser requests the bounded 250-artifact evidence-detail maximum so late
plate/face observations are not hidden by a generic preview limit. Data renders
real plate text, detector/OCR confidence, artifact citation, typed bbox overlay
and a non-retained canvas crop reconstructed from authorized source pixels.

Image/video/audio presentation now has explicit processing states, result-only
sections, whitespace-only OCR filtering without hiding low-confidence real
model output, face crops with embedding eligibility, video frame selectors and
a truthful complete-zero ANPR timeline for the approved video (retained ANPR
observations: zero). Source-local Ask actions use actual plate/OCR/transcript
values. Existing face and SigLIP ranking plus deterministic image comparison
are exposed through URL-case-bound, read-only LocalAI proxies; candidate IDs are
explicit and the sidecar credential never reaches the browser. No new model,
processor, database contract or retained artifact was introduced.

Verification: five pure presentation tests pass; the focused case-scope Go spec
passes; auth tests pass; React production build passes; and the focused mocked
browser test passes ANPR detail/crop/overlay, face ranking, zero-result filtering
and page-overflow checks at 390/820/1024/1440. The broader routes suite reports
18 pass and one unrelated existing Windows/sandbox backend-upgrade fixture
failure. Live browser acceptance then passed the retained plate result and crop,
face/SigLIP candidate ranking, image comparison, video complete-zero plate
timeline, timestamped OCR/transcript, clean no-face section handling, History
reopen and evidence citation navigation. Live 390/820/1024/1440 viewport checks
showed no page or detail-dialog horizontal overflow and the browser console had
zero errors.

Only `nexusai-api-1` was rebuilt/recreated. Rollback tag
`nexusai/localai-forensic:rollback-before-media-productization-20260823`
preserves the prior image. Active LocalAI is container `e04dae4cbd44` on image
`sha256:afc6ac795e0c16eca293c765c23caf08fa768b961c774409324664ad320fd7b6`;
`/readyz` returns 200. Forensic API `a97e545b8356`, worker `0b2418d90cbd`,
PostgreSQL `f66e05a3b179`, NATS `5c49e70d132a`, models, named volumes and all
retained evidence/jobs were preserved. Live read-only API reconciliation remains
21 ready evidence, 34 jobs (`33 completed | 1 historical dead-letter | 0
active`), 372 derived artifacts and 19 KB assets; the authoritative accepted
baseline remains 21 versions and 9,274 canonical records because this phase did
not mutate retained state. `RetainedMutationNeeded=NO`,
`DatabaseMigrationNeeded=NO`, `ModelDownloadNeeded=NO`, `OpenP0=0`,
`OpenFoundationalP1=0`, and cleanup remains forbidden.

## 2026-08-23 unified multimodal product acceptance complete

The exact approved 21-source workspace
`default/nexusai-multimodal-product-acceptance` is accepted with recorded
limitations. Exactly 12 approved sequential immutable normal-endpoint
reprocess jobs completed in manifest order with no failure and no second
reprocess. Final read-only reconciliation is `21 evidence | 21 versions | 34
jobs (33 completed, 1 historical dead-letter, 0 active) | 9,274 canonical
records | 372 derived artifacts | 19 KB assets`. No new evidence, force upload,
database mutation/migration, model download, broad replay, deletion or cleanup
occurred.

The worker role/TXT-authority recovery is complete. A bounded related forensic
API correction restored extension-safe native TXT source access, routed
retrieval-only Ask operations through KB/derived-text instead of records SQL,
and stopped evidence-only cited answers from being mislabeled as no-results.
Focused Go tests pass. Only the forensic API was rebuilt/recreated for these
corrections; LocalAI `7f41a8378012`, worker `0b2418d90cbd`, PostgreSQL
`f66e05a3b179`, NATS `5c49e70d132a`, models, volumes and retained state were
preserved. API rollback tag is
`nexusai/forensic-records-api:rollback-before-unified-txt-retrieval-p1-20260823`.

The 65-operation structured matrix passed `64 answered | 1 accepted
no-result`, p95 904.3 ms. Retained ANPR `MN1367`, image OCR, dHash comparison,
five-candidate SigLIP, bounded synthetic face ranking, Urdu/Roman Urdu,
timestamped video, TXT/PDF/DOCX passages, KB retrieval, cited Ask, truthful
negative Ask and History reopen all pass at their published maturity. Security
denials are `403/400/403/404/404`; full/range source access is `200/206` for
image, audio, video, TXT, PDF and DOCX.

Recorded limitations: no positive exact/near-duplicate pair exists in the
approved manifest; face/SigLIP are candidate similarity only; OCR/ANPR/ASR
remain review-required; spoken plate formatting is M2; the in-app browser lacks
audio/video codec playback despite working source endpoints; and the audio
detail readiness label conflicts with its visible completed artifacts.

Authoritative artifacts are
`reports/nexusai-unified-multimodal-product-acceptance-20260823.md`,
`reports/unified-multimodal-product-acceptance-20260823/product-acceptance-api.json`,
`reports/unified-multimodal-product-acceptance-20260823/export-checksum-manifest.json`
and the single demo guide
`docs/demo/nexusai-breadth-multimodal-demo-guide.md`. `OpenP0=0`, `OpenP1=0`,
`Verdict=ACCEPTED_WITH_RECORDED_LIMITATIONS`, `CleanupAuthorized=NO`.

## 2026-08-23 unified multimodal source admission complete; media-role recovery approval gate

The exact approved 21-source manifest is fully admitted in tenant `default`,
collection/case `nexusai-multimodal-product-acceptance`, with 21 evidence,
21 versions and all 21 latest source jobs completed. The one separately
approved ANPR recovery job is immutable generation 1
`92d81704-c291-4598-a0ee-4d145c900bf4`; its dead-letter parent remains
preserved. No source was force-uploaded, no unmanifested evidence entered the
case, and no cleanup or deletion occurred.

Final guarded-ingest accounting is 9,407 accepted job outputs, one duplicate
TXT row and zero rejected rows. Read-only PostgreSQL reconciliation returned
22 total job rows (`21 completed | 1 dead_letter`), 9,273 canonical records,
125 derived artifacts, 19 KB-asset rows and zero active jobs. The 65-operation
deterministic structured query matrix passed `64 answered | 1 accepted no
results`, with no routing/completion failure and p95 904.3 ms.

Product reconciliation found that the recovery rebuild had recreated the
worker with ANPR, face, SigLIP, OCR and ASR disabled and mounted the empty
repository media directory. This made the completed image/audio/video jobs
metadata-only. The existing checksum-verified local assets were intact; no
model download was needed. A rollback tag was created, then only the worker was
rebuilt/recreated with `C:\Users\sheik\.cache` mounted read-only and all five
approved roles enabled. LocalAI `7f41a8378012`, forensic API
`62d9d475ad0a`, PostgreSQL `f66e05a3b179` and NATS `5c49e70d132a` were
preserved. Active worker `0b2418d90cbd` uses image
`sha256:4b345f8f3a503a9e54c32a07089a2313b19a368ab4120edb610648d592f8764f`.

A disposable non-retained production-processor probe proved FastALPR ANPR at
OCR confidence 0.9998, one synthetic face at confidence 0.9517 with a
128-dimensional candidate embedding, SigLIP 768-dimensional output, and
explicit-Urdu timestamped ASR plus Roman Urdu derivatives. It also exposed a
TXT route-authority defect: the API selected `native_document_worker`, but
stale `structured_records` modality caused generic-row parsing. The worker now
treats the authenticated native-document route as authoritative; nine focused,
network-disabled tests pass and the correction is deployed in the same narrow
worker generation.

The current approval authorizes exactly one reprocess job and forbids broad
replay, so no further retained job was issued. The exact minimal recovery is 12
sequential normal-endpoint reprocess jobs: seven approved images, three Urdu
audio files, the one approved video and the TXT source. PDF/DOCX and all seven
structured sources require no replay. Exact evidence IDs, hashes, generations,
order, ceilings and stop-on-failure rules are frozen in
`reports/unified-multimodal-product-acceptance-20260823/P1_MEDIA_ROLE_RECOVERY_APPROVAL_MANIFEST.json`.
`OpenP0=0`, `OpenP1=1`, `ModelDownloadNeeded=NO`,
`DatabaseMigrationNeeded=NO`, `NewEvidenceNeeded=NO`,
`RetainedMutationBlocked=YES_PENDING_EXACT_12_JOB_APPROVAL`, and cleanup remains
forbidden.

## 2026-08-23 post-BF-A breadth source completion; activation and retained proof gated

The bounded post-BF-A source slice is complete without changing runtime or
retained state. Image ingestion now emits deterministic 64-bit dHash artifacts;
the forensic API exposes a read-only, tenant/collection/case-scoped two-image
comparison with exact hash, dimensions, dHash distance, OCR/ANPR overlap, face
counts when present, and stable evidence/artifact citations. Existing FastALPR
integration remains opt-in and local-path-only. The active worker was verified
with `FORENSIC_ANPR_ENABLED=false`, while the accepted detector/OCR assets still
exist in the user caches. This is configuration/source-image drift, not an ANPR
pipeline rewrite, and no replacement ANPR model is needed.

Urdu audio/video processing now preserves raw Urdu as authoritative and adds a
deterministic, identifier-safe `ur-Latn` derivative with lineage and review
metadata. Video wrappers preserve child timestamp/Roman-Urdu contracts. The
worker adds bounded native TXT, DOCX and text-bearing PDF extraction with stable
section/page/paragraph citations, KB/query integration, a 64 MiB source limit,
16 MiB expanded DOCX XML limit and truthful scanned-PDF abstention. No OCR is
invoked for scanned documents.

History presentation now consumes enterprise semantic evidence, fact-packet
citations and stored provenance. Home/Data source uses live readiness,
capability and recent-evidence state, unified modality filters, shrinkable grids
and content wrapping without horizontal clipping. The platform catalog is now
`2026-08-23.post-bfa-source.1`, with 9 adapters, 97 operations and explicit
`PLANNED`, `ENGINEERING_ONLY`, `LIMITED` or `CERTIFIED` maturity. Python catalog
binding safely ignores non-structured adapters when constructing the legacy
structured compatibility registry.

Verification passes: the full forensic API synthesis suite, 45 focused Python
image/Roman-Urdu/document/media/worker tests, the focused agent History
presentation contract and targeted Analyst UI lint with zero errors. The
earlier broad agent suite remains unsuitable on this Windows host because
unrelated database tests require rootless testcontainers; the relevant focused
contract passes. No long UI build was run without the required separate build
approval, and no deployed/browser claim is made.

The sole guide and mutable acceptance matrix is
`docs/demo/nexusai-breadth-multimodal-demo-guide.md`. The active gates are:
`WorkerDeploymentNeeded=YES_NOT_AUTHORIZED`,
`RetainedMutationNeeded=YES_FOR_NEW_PROOF_NOT_AUTHORIZED`,
`DatabaseMigrationNeeded=NO`, `ANPRModelDownloadNeeded=NO`,
`FaceDetection=MODEL_APPROVAL_REQUIRED`,
`FaceSimilarity=MODEL_APPROVAL_REQUIRED`, `OpenP0=0`, `OpenP1=2`, and
`CleanupAuthorized=NO`. One proposed face role is
`face-detect-yunet-sface`/`yunet-sface.gguf` (26.1 MB, Apache-2.0, SHA-256
`9ce78d4ba0ae9d5e8c91a0e145d511558d1d90f5d9c1f4131cca9bb4bce60902`),
restricted to detection and authorized candidate similarity. It has not been
downloaded. Exact next action is a separately approved rollback-tagged,
worker-only activation with existing local ANPR assets and non-mutating health
checks; BF-A evidence remains on review hold and must not be reprocessed,
uploaded, deleted or cleaned implicitly.

## 2026-08-23 BF-A media recovery complete; retained breadth accepted with limitations

The approved worker-only recovery is complete. The old worker image is tagged
`rollback-before-bfa-media-completion-20260821`; only the worker was rebuilt and
recreated, and the forensic API, LocalAI, PostgreSQL and NATS container IDs were
preserved. The tested PostgreSQL text casts are live. Exactly one immutable
Islamabad-image reprocess job was created as
`831c9b62-4872-4302-a8e5-c2cf7f27042b`; the existing plate generation and all
failed/queued audio/video jobs resumed without upload or deletion.

The isolated scope now contains seven evidence items, eight job rows, fifteen
derived artifacts, seven KB mirrors and zero canonical structured records. Six
latest media generations completed; the two original dead-letter jobs remain
as immutable history. Data/KB/History and the hybrid citation contract pass.
Agent History retained the analysis but its presentation citation array is
empty. Browser inspection has no console errors and Data/Ask/History do not
overflow, while Home reproducibly overflows at 820/1024/1440 pixels because of
long analysis cards. Image jobs truthfully report that no approved local
vision/ANPR role is configured, so retained ANPR extraction is not certified.

The authoritative result is
`reports/nexusai-bf-a-retained-breadth-acceptance-20260821.md`; evidence hashes
and versions are frozen in
`reports/nexusai-bf-a-retained-evidence-checksum-manifest-20260823.md`.
`BreadthVerdict=ACCEPTED_WITH_RECORDED_LIMITATIONS`, `OpenP0=0`, `OpenP1=0`,
`OpenP2=2`, `WorkerDeploymentNeeded=NO`, `DatabaseStateChangeNeeded=NO`, and
cleanup remains forbidden without separate explicit approval.

## 2026-08-21 BF-A recovery frozen on media-completion SQL typing P1

The approved one-table `derived_artifacts` INSERT grant passed a rolled-back
same-tenant write/cross-tenant denial probe. The approved worker-only activation
also deployed the durable ASR-language handoff. Immutable ANPR reprocessing was
queued as job `88df3883-6c4e-44a2-ac54-704e10d3d26f`.

The next processor transaction exposed `mark_media_job_completed` passing
untyped string parameters to `jsonb_build_object`; PostgreSQL failed with
`could not determine data type of parameter $3` and rolled back artifact writes.
Islamabad image is dead-lettered, clear Urdu is failed at attempt four, and the
remaining jobs plus ANPR generation 1 are queued. The worker was stopped again.
Source now explicitly casts readiness/processor/revision parameters to text and
focused tests pass. Scope artifacts/canonical rows remain zero.

Current counts are
`jobs=26|evidence=27|artifacts=0|canonical_records=12932|kb_assets=19`.
`OpenP0=0`, `OpenP1=1`, `DatabaseStateChangeNeeded=NO_GRANT_ACCEPTED`,
`WorkerDeploymentNeeded=YES_APPROVAL_REQUIRED`, and cleanup remains forbidden.

## 2026-08-21 BF-A retained run frozen on derived-artifact grant P1

The approved BF-A run registered all seven immutable inputs and six media jobs
inside `default/nexusai-breadth-acceptance-20260820`. The API-only language
admission activation passed and durable job metadata contains `ur` for both
natural Urdu clips and the video; the mixed synthetic clip correctly remains
automatic. The DOCX registered one KB asset and truthfully remains
`document_extraction_pending`.

Processing was frozen when the worker hit `permission denied for table
derived_artifacts`. ANPR dead-lettered after five rapid retries, the Islamabad
image stopped failed after four, and the remaining jobs remain queued. The
worker container was stopped without recreation or deletion. Current global
counts are `jobs=25|evidence=27|artifacts=0|canonical_records=12932|kb_assets=19`;
the approved scope contains seven evidence/version pairs, six jobs, no artifact
or canonical row, and one document KB asset.

Source now includes the missing one-table INSERT grant and a focused worker
handoff that copies only durable `asr_language` into processor metadata. Focused
Go/Python tests pass. Neither the database grant nor worker deployment has been
applied because both are new approval boundaries. The authoritative frozen
result and all IDs are in
`reports/nexusai-bf-a-retained-breadth-acceptance-20260821.md`.

`OpenP0=0`, `OpenP1=1`, `RetainedDataMutated=true_authorized_scope_only`,
`DatabaseStateChangeNeeded=YES_APPROVAL_REQUIRED`, and
`WorkerDeploymentNeeded=YES_APPROVAL_REQUIRED`.

## 2026-08-21 BF-A retained breadth preflight — API language-admission deployment gate

The operator explicitly approved the isolated seven-input retained breadth run
for tenant/collection/case `default/nexusai-breadth-acceptance-20260820`.
Preflight verified all seven exact files and hashes, healthy runtime services,
zero active jobs, an empty target scope, and unchanged global counts
`jobs=19|evidence=20|artifacts=0|canonical_records=12932|kb_assets=18`.

No upload was started because source inspection proved the normal sidecar upload
path did not admit `asr_language` into durable job metadata. Proceeding would
silently violate `KnownUrduLanguagePolicy=explicit_ur_when_known`. The narrow
source correction now normalizes and validates a two/three-letter
`asr_language`, persists it with the job, and preserves empty-language automatic
detection. Focused Go tests pass. `RetainedDataMutated=false` and target-scope
evidence remains zero.

`OpenP0=0`, `OpenP1=1` (`BF-P1-ASR-UPLOAD-LANGUAGE-003`, source corrected,
deployment pending). The next action is separately authorized, API-only guarded
build/recreate of `nexusai-forensic-records-api-1`; LocalAI, worker, PostgreSQL,
NATS, models and volumes must remain unchanged. After deployment and a
non-retained/request-contract probe, resume the already-approved BF-A upload.

## 2026-08-21 Pakistan Urdu ASR M1 runtime accepted — retained breadth approval gate

The language-safe `faster-whisper-small-ur` path is live. Source now validates
and forwards explicit `ur`/`en`, preserves absent-language automatic detection,
maps the verified local model directory without startup download, and keeps
requested and detected language metadata distinct. The HTTP multipart adapter
was the final drop point and now preserves the form language through gRPC.
Focused Python media tests passed 12/12, faster-whisper adapter tests passed
6/6, and the focused Go multipart-language contract passed.

The pinned runtime uses `Systran/faster-whisper-small` revision
`536b0662742c02347bc0e980a01041f333bce120`, faster-whisper 1.2.1,
CTranslate2 4.7.1, CPU INT8, and model hash
`3e305921506d8872816023e4c273e75d2419fb89b24da97b4fe7bce14170d671`.
Direct and deployed-worker non-retained smokes returned Urdu script with
explicit `ur`; clear/second-read WER/CER were 0.6000/0.1852 and
0.3333/0.1446. The worker preserved timestamps, provenance, manual-review
state and video composition. The no-language control still selected Hindi and
Devanagari, proving why the governed explicit-Urdu policy is required.

Only LocalAI (architecturally required) and the worker were rebuilt/recreated.
The forensic API, PostgreSQL and NATS IDs were preserved; rollback image tags
exist for both changed services, and `whisper-tiny` remains installed.
Before/after retained counts are exactly
`jobs=19|evidence=20|artifacts=0|canonical_records=12932|kb_assets=18`, with
zero active jobs. `PakistanUrduASRRuntime=PASS`, `AudioM1PakistanReady=YES`,
`OpenP0=0`, `OpenP1=0`, `DeploymentNeeded=NO`,
`DatabaseMigrationNeeded=NO`, `ModelDownloadNeeded=NO`, and
`RetainedDataMutated=false`. The exact next phase is BF-5/6 isolated retained
breadth diagnostic E2E, still blocked on the separate operator approval in the
retained pack. Do not upload it implicitly.

## 2026-08-20 Pakistan Urdu ASR challenger accepted for M1 — deployment approval gate

The one authorized `Systran/faster-whisper-small` challenger was verified,
downloaded at revision `536b0662742c02347bc0e980a01041f333bce120`
(486214370 model bytes, MIT), and benchmarked locally/offline against the exact
three canonical WAVs. No remote model code ran. Model/runtime/results remain
under ignored `local-acceptance-models/`; the full acceptance record is
`reports/nexusai-pakistan-urdu-asr-challenger-acceptance-20260820.md`.

With explicit `ur` guidance, clear and second-speaker read Urdu improved from
tiny WER/CER 1.0000/1.0617 and 1.4583/1.3012 to small 0.6000/0.1605 and
0.2917/0.1325. The first is PARTIAL but meaning-preserving; the second is
USABLE; neither has a major omission or hallucination, and timestamps remain
usable. The synthetic fixture preserved phone `03001234567` and time `1035`
but not plate `MN1367`, so identifier maturity remains M2. Model load was
2.193 seconds, warm RTF was 0.8574/0.4809/1.1251, and peak RSS was 918.27 MiB.

Automatic detection mislabeled both Urdu clips as Hindi and emitted
Devanagari. The existing LocalAI Python faster-whisper backend receives a gRPC
language field but does not forward it to `WhisperModel.transcribe`; deployment
therefore requires a focused, reviewed backend correction and test in addition
to model/worker configuration. Nothing was deployed. Before/after retained
counts stayed exactly `jobs=19|evidence=20|artifacts=0|canonical_records=12932|kb_assets=18`.

`PakistanUrduASRPracticalBaseline=PASS_FOR_M1`,
`WhisperTinyPakistanM1=REJECTED`,
`WhisperSmallPakistanM1=ACCEPTED_CHALLENGER`, `AudioM1PakistanReady=YES`,
`IdentifierASRMaturity=PENDING_M2`, `OpenP0=0`, `OpenP1=1` (accepted ASR
deployment compatibility/activation), `DatabaseMigrationNeeded=NO`,
`ModelDeploymentNeeded=YES_PENDING_APPROVAL`, and `RetainedDataMutated=false`.
The exact next action is approval for that bounded ASR-only activation gate;
the retained seven-input pack remains unuploaded.

## 2026-08-20 lawful inputs tested; retained breadth diagnostic approval gate

The product owner activated the frame-identity correction with
`BFVideoFrameIdentityActivation=PASS WorkerRunning=true
ProtectedContainersPreserved=true RetainedDataMutated=false`. Final composed
non-retained runtime acceptance passed in 10.796 seconds: metadata available,
two bounded frames at 0/5 seconds, two distinct `MN1367` observations, embedded
audio, three timestamped ASR segments, nine unique observation IDs, preserved
parent/citation locators, manual-review transcript state, and exact temporary
cleanup. Retained counts remained `jobs=19|evidence=20|artifacts=0`.

The family processor boundary is accepted for ANPR, Image, Audio and Video.
This does not certify retained product E2E. Two lawful FLEURS `ur_pk` clips and
a clearly synthetic sequential identifier fixture were tested without
persistence. The pipeline emitted timestamps, but clear/second-read Urdu scored
indicative WER 1.0000/1.4583 and the fixture did not preserve exact identifiers.
`PakistanUrduASRPracticalBaseline=FAIL`; this is a model-quality block, not an
input or runtime block. The approval-gated diagnostic pack is defined in
`reports/nexusai-breadth-retained-acceptance-pack-20260820.md` with isolated
tenant/collection/case scope. Exact licensed inputs, hashes, source transcripts
and attribution are preserved under ignored `local-acceptance-inputs/`. A
lawful derived Islamabad video separately passed frame/audio/timestamp/citation/
cleanup processing in 14.246 seconds. Database counts remain `19|20|0`.

`OpenP0=0`, `OpenP1=1` (Pakistan Urdu ASR model quality),
`ANPRProcessorRuntimeStatus=ACCEPTED`,
`ImageProcessorRuntimeStatus=ACCEPTED`, `AudioProcessorRuntimeStatus=ACCEPTED`,
`VideoCompositionRuntimeStatus=ACCEPTED`,
`PakistanUrduASRPracticalBaseline=FAIL`,
`AudioMVPStatus=BLOCKED_ASR_MODEL_QUALITY`,
`RetainedDataMutationNeeded=YES_AFTER_REVIEW_AND_APPROVAL`,
`DatabaseMigrationNeeded=NO`, `ModelDownloadNeeded=NO_CURRENTLY`, and
`DeploymentNeeded=NO`. One compatible 486 MB
`Systran/faster-whisper-small` challenger is researched but not downloaded.
The next operator action is to review the media/truth/licenses, confirm the
external ANPR fixture is lawfully usable, and explicitly approve or decline the
seven-input retained diagnostic pack. Do not upload anything yet.

## 2026-08-20 composed video runtime acceptance — frame identity P1 pending activation

The product owner activated the first video-composition correction and preserved
`BFVideoCorrectionActivation=PASS WorkerRunning=true
ProtectedContainersPreserved=true ModelsChanged=false VolumesChanged=false
RetainedDataMutated=false`. The worker remained healthy and subscribed. A
reusable non-retained composed-video verifier now asserts metadata, bounded
frame times, ANPR, embedded-audio ASR, timestamp citations, provenance, unique
IDs, cleanup, performance, and unchanged retained counts.

Two generated-fixture attempts failed safely and cleaned all API/worker/host
temporary files. The first six-second fixture produced only one frame at the
five-second sampling boundary, so the final fixture was correctly lengthened to
11 seconds. That run exposed a genuine P1: identical plate bounds in separate
sampled frames reused one candidate/observation UUID because ANPR identity did
not include the frame locator. Candidate identity now includes a stable JSON
encoding of `locator_prefix`, making the same detection at 0 and 5 seconds
distinct while retaining deterministic replay. The focused regression returned
`VideoFrameANPRIdentityRegression=PASS`; compilation and diff checks passed.
Retained counts remained exactly `jobs=19|evidence=20|artifacts=0` before and
after, and cleanup returned `DuplicateIdentityAttemptCleanup=PASS`.

`OpenP0=0`, `OpenP1=1`, `VideoRuntimeStatus=BLOCKED_PENDING_FRAME_IDENTITY_ACTIVATION`,
`DatabaseMigrationNeeded=NO`, `ModelDownloadNeeded=NO`, and
`RetainedDataMutationNeeded=NO_FOR_NEXT_GATE`. The exact next action is the
smallest worker-only guarded rebuild/recreate. Do not prepare or execute the
retained acceptance pack until the composed non-retained video verifier passes.

## 2026-08-20 breadth worker activation and non-retained media runtime — video correction pending activation

The product owner completed the worker-only guarded activation. Preserve the
marker `BFWorkerPackagingActivation=PASS WorkerRunning=true ASREnabled=true
ANPREnabled=true ProtectedContainersPreserved=true ModelsChanged=false
VolumesChanged=false RetainedDataMutated=false`. The corrected worker is healthy
and subscribed to its durable JetStream consumer. The image import command's
`NameError: PASS` was native PowerShell quote loss in the diagnostic `print`,
not an import failure; successful service startup proves the import path.

Read-only/non-retained worker acceptance then passed. FFmpeg and ffprobe are
installed; all explicit ANPR assets are readable; and
`MediaProcessorConfig=PASS ANPR=FastALPRImageProcessor
ASR=LocalAIASRProcessor`. The actual worker ASR client returned two timestamped
observations through the LocalAI service with `language=None` and
`manual_review_required=true`. FastALPR processed the existing read-only
reference image as `MN1367` with detector confidence `0.8897411227226257`, OCR
confidence `0.9997855305671692`, and active OpenVINO plus CPU providers. All
temporary API/worker/host files were verified removed. No queue job, retained
evidence, derived artifact, or database row was created.

Video composition review found one P1 source defect before runtime acceptance:
configured ASR still emitted an "unavailable" limitation, embedded technical
and transcript observations reused one ID, and segment citation times were not
carried into the video locator. The source now emits the limitation only when
ASR is absent, derives unique IDs from each child observation, and preserves
segment start/end times; focused regression coverage was added. `OpenP0=0`,
`OpenP1=1`, `WorkerRuntimeStatus=ACCEPTED_PRE_VIDEO_CORRECTION`,
`AudioNonRetainedRuntimeStatus=ACCEPTED`,
`ANPRNonRetainedRuntimeStatus=ACCEPTED`, `VideoRuntimeStatus=PENDING_CORRECTION_ACTIVATION`,
`DatabaseMigrationNeeded=NO`, `ModelDownloadNeeded=NO`, and
`RetainedDataMutationNeeded=NO_FOR_NEXT_GATE`. The exact next action is one
more worker-only guarded rebuild/recreate followed by a non-retained composed
video smoke.

## 2026-08-20 breadth runtime activation reconciliation — P1 worker packaging correction pending activation

The product owner completed the guarded LocalAI/UI and forensic-worker builds.
The LocalAI image is running healthy as
`nexusai/localai-forensic:phase3-runtime`, `/readyz` returned HTTP 200, and the
installed inventory now includes `whisper-tiny` alongside the accepted Qwen
general and embedding models. The worker image also built successfully, but
`nexusai-forensic-records-worker-1` exited with code 1 immediately after start.

Read-only container inspection proved the exact cause: `/app/worker.py` imports
`structured_maturity`, but the standalone worker Dockerfile omitted that source
module. The Dockerfile now copies `structured_maturity.py`, and each extracted
structured adapter has an explicit standalone-image fallback matching the
existing worker import contract. Python compilation passed and a forced
standalone import (with repository-package imports blocked) returned
`StandaloneWorkerImport=PASS`. Pytest is unavailable in the host Python 3.10
environment.

`OpenP0=0`, `OpenP1=1`, `WorkerRuntimeStatus=BLOCKED_PENDING_PACKAGING_FIX_ACTIVATION`,
`DatabaseMigrationNeeded=NO`, `ModelDownloadNeeded=NO`, and
`RetainedDataMutationNeeded=NO_FOR_NEXT_GATE`. The exact next action is a
worker-only guarded rebuild/import smoke/recreate, preserving PostgreSQL, NATS,
LocalAI/UI, forensic API, models, profiles, volumes, and rollback images. After
worker liveness passes, continue with worker-path acceptance before requesting
any retained media intake.

The installed `whisper-tiny` backend contract is already runtime-proven through
a non-retained temporary `espeak-ng` WAV submitted directly inside the LocalAI
container. `verbose_json` returned one segment (`start=0`, `end=4.84`), top-level
text, and `duration=5.25906229019165` in 5.57 seconds (RTF approximately 1.06).
The response did not include a language field. The synthetic-speech wording was
imperfect, so this proves endpoint/model/timestamp execution only—not Urdu,
mixed-language, representative-evidence accuracy, or Audio M1. The stopped
worker's effective environment has `FORENSIC_ANPR_ENABLED=true` and correct
read-only model paths, but `FORENSIC_ASR_ENABLED=false` despite
`FORENSIC_ASR_MODEL=whisper-tiny`; worker-path ASR therefore remains pending an
explicitly enabled worker recreation plus an approval-gated evidence input.

## 2026-08-20 breadth-first multimodal source implementation — ACTIVE controlling checkpoint

The latest team-lead directive supersedes the post-STIM design-only handoff
without reopening STIM-0 through STIM-7. Structured Intelligence remains
closed. BF-0 reconciliation was followed immediately by bounded ANPR, image,
audio and video source implementation.

Supported raster/audio/video intake now targets the existing queue and shared
media processor. The source preserves immutable evidence and existing scope,
run/event/artifact contracts; provides an opt-in, no-download FastALPR adapter;
persists review-required OCR into derived artifacts and canonical ANPR records;
adds ffprobe audio observations; composes metadata, bounded frame and embedded-
audio extraction with optional ANPR for video; and exposes authorized preview/
playback. No migration was needed. Both external tutorial projects remained
read-only.

The read-only FastALPR smoke recognized `MN1367` at detector confidence
`0.8897413611412048`; ONNX Runtime fell back to CPU because OpenVINO could not
load `openvino.dll`. This is a smoke, not an accuracy gate. ANPR/Image/Video
source activation requires a guarded rebuild and runtime acceptance. Audio
source foundations are ready, but Audio M1 is `BLOCKED_MODEL_APPROVAL` because
no approved ASR model is configured. No deployment, model/dataset download,
retained ingest, training, migration, staging, commit, push or PR occurred. See
`reports/nexusai-breadth-first-multimodal-source-progress-20260820.md`.

Exact next action: decide whether to approve the single ASR proposal;
independently, approve the guarded multimodal deployment/runtime deck. Advanced
Query Intelligence is `DEFERRED_UNTIL_BREADTH_BASELINE`.

## 2026-08-20 STIM-7 final structured intelligence acceptance — CLOSED controlling checkpoint

STIM-0 through STIM-6 remained frozen at their accepted boundaries while the
bounded STIM-7 certification replayed the complete structured-intelligence
surface. No production source, database, retained evidence, model, profile,
worker, service, or deployment state was changed. Source/runtime parity passed
against the running LocalAI and forensic-records API images; PostgreSQL, NATS,
named volumes, model inventory, profiles, rollback images, and the retained
seven-fixture acceptance pack remained preserved.

The source gate passed 46/46 structured Python goldens, the forensic Go package,
Go vet, and the focused presentation-contract suite. The live 65-operation
matrix passed with 64 answered and one accepted no-result operation at 898.5 ms
p95. The independent STIM-7 runtime oracle passed 11/11: exact evidence/version
inventory and 21/20/1/0 row accounting, the seven-event three-source CDR oracle,
stable tampered-membership rejection, five cross-family positive/negative
cases, East-to-West ANPR sequence and 480-second timing, and the plate-lookalike
non-match. The repeat STIM-5/6 deck also passed 14/14.

The 677-module production UI build passed. ESLint returned zero errors and 220
warnings across the intentionally broad dirty UI surface. The controlled
browser deck passed 13/13, and an independent deployed-product sweep verified
Home, Data, Ask, History, and a real source-detail drawer at 390, 820, 1024, and
1440 px with no horizontal overflow or console errors.

`STIM-7SourceStatus=ACCEPTED`, `STIM-7RuntimeStatus=ACCEPTED`,
`STIM-7FinalStatus=CLOSED`, `StructuredIntelligenceMaturityProgram=CLOSED`,
`StructuredIntelligenceFinalAcceptance=PASS`, `OpenP0=0`, `OpenP1=0`,
`DatabaseMigrationNeeded=NO`, and `DeploymentNeeded=NO`. The remaining four P2
items are non-blocking: installed-model raw narrative quality/latency,
capability registration versus indexed readiness, the still-undefined
INPR/INPRS domain terms, and future representative large-case query/index
measurement. No P3 remains open.

The exact next phase is **Post-STIM architecture reconciliation**. Its immediate
priority is the design and approval of **Governed Runtime Query Intelligence**:
a typed, policy-bounded planning/execution layer over certified operations and
allowlisted analytical primitives. Do not implement generalized runtime SQL,
resume R8/media work, or begin document/RAG expansion under this closure.

## 2026-08-20 STIM-5/STIM-6 guarded runtime acceptance — superseded closed checkpoint

The product-owner preflight and guarded activation passed with source/runtime
parity and preserved the worker, model inventory, profiles, named volumes and
rollback images. The activation markers were
`R8UIAPIActivationPreflight=PASS API=d29a3376c0ee LocalAI=9ef8805eebba Mutation=false`
and `R8UIAPIActivation=PASS APIBuildSeconds=21.5 LocalAIBuildSeconds=225.5
WorkerRebuilt=false ModelsChanged=false ProfilesChanged=false
VolumesPreserved=true RollbackImages=preserved`.

The corrected Windows PowerShell 5.1 runtime deck passed 14/14 checks across
readiness, model inventory, English, messy English, Roman Urdu, UTF-8 Urdu,
mixed language, clarification, scoped follow-up direction, unsupported fast
path, validator-protected narrative fallback and the three deployed Analyst
Portal routes. Its evidence is
`reports/runtime-activation-stim56/stim56-runtime-acceptance-20260820T070738Z.json`
and its marker is `STIM56RuntimeAcceptance=PASS Checks=14 Failures=0
DatabaseMutated=false ModelsChanged=false`. The independent 65-operation demo
matrix also passed: `FullDemoQueryMatrix=PASS Queries=65 Answered=64
AcceptedNoResults=1 P95MS=454.9`.

The Windows-safe browser wrapper passed the Analyst Portal deck 11/11 plus the
rich governed-answer and Urdu 390px cases. A separate in-app inspection of the
deployed `/analyst/data`, `/analyst/ask` and `/analyst/history` routes confirmed
real retained workspace content, mobile navigation, no horizontal overflow at
390px, and zero browser warnings/errors. The HTTP verifier now checks those
canonical `/analyst/*` routes rather than treating the `/app` shell's HTTP-200
404 page as route acceptance.

`STIM-5RuntimeStatus=ACCEPTED`, `STIM-5FinalStatus=CLOSED`,
`STIM-6RuntimeStatus=ACCEPTED`, `STIM-6FinalStatus=CLOSED`, `OpenP0=0`,
`OpenP1=0`, `DatabaseMigrationNeeded=NO`, `DeploymentNeeded=NO`. The installed
Qwen model remains optional and validator/fallback protected; no challenger was
downloaded or approved. At this historical checkpoint STIM-7 remained not
started; the controlling STIM-7 closure above supersedes that handoff.

## 2026-08-20 STIM-5/STIM-6 source accepted; guarded runtime activation pending — superseded source checkpoint

STIM-3 and STIM-4 remain closed at their accepted source/runtime boundary.
STIM-5 adds deterministic multilingual normalization for English, messy English,
Roman Urdu, Urdu and mixed script; exact scoped follow-up inheritance;
clarification and unsupported-capability preflight; bounded
`forensics.fact-packet/v1`; strict `forensics.narrative/v1`; claim/reference,
identifier, number, relationship, limitation and suggestion validation; and
deterministic fallback. The regenerated language ledger has 172 accepted
occurrences (160 unique, 12 duplicate occurrences) and the full forensic Go
package passes 285 specs with the same two intentional skips. `go vet` passes.

Direct probes of installed `qwen_qwen3-4b-instruct-2507` produced schema-valid
but factually rejected exact-number answers in English, Roman Urdu and Urdu,
with roughly 21--45 second latency. The model is therefore retained only as an
optional explainer behind the strict eight-second validator/fallback boundary;
raw model prose is not accepted as factual output. No model download, profile
change or challenger activation is authorized.

STIM-6 removes the hidden server-side table-column truncation and adds governed
priority/progressive columns, source overview/mapping/quality/readiness,
explicit answer/fallback states, relationships, proof-role citations,
limitations, how-determined trace, full History restoration, and Urdu RTL/BiDi
identifier isolation. The Analyst Portal deck passes 11/11, the rich governed
answer case passes, the Urdu 390px mobile case passes, and the 677-module
production build passes. Real-source interactive QA at desktop and 390px found
no horizontal overflow; retained older source metadata is shown as not reported
instead of inferred. Focused ESLint has zero errors and 22 deferred warnings.

One consolidated source evidence artifact is
`reports/nexusai-stim56-source-acceptance-20260820.md`. No build/deployment,
restart, migration, retained ingest/reprocess/delete, worker/model/profile
change, stage, commit, push or PR occurred. Both the forensic API and LocalAI/UI
source changed, so the product owner must run the existing guarded R8 UI/API
preflight and activation, then `scripts/verify_nexusai_stim56_live.ps1` and the
browser deck. `STIM-5SourceStatus=ACCEPTED`,
`STIM-6SourceStatus=ACCEPTED`, both runtime/final statuses remain pending,
`OpenP0=0`, `OpenP1=0`, `DatabaseMigrationNeeded=NO`, `DeploymentNeeded=YES`.
Do not begin STIM-7 until runtime acceptance passes.

## 2026-08-20 STIM-3/STIM-4 runtime closure accepted — controlling checkpoint

The product-owner ran the guarded forensic-API-only activation and preserved
the marker `R7.10APIActivation=PASS BuildSeconds=21.2
ServicesRecreated=forensic-records-api ProfilesChanged=false
VolumesPreserved=true RollbackImage=preserved`. The API is healthy; no worker,
PostgreSQL, NATS, LocalAI/UI, model, profile, or named-volume change was made.
The orphan-container warning remained warning-only and no cleanup occurred.

The retained seven-fixture pack remains accepted at 21 total rows, 20 accepted,
one duplicate and zero rejected. The exact three-CDR query passed the independent
oracle: three sources, seven accepted events, common contact `923110000001`, the
three expected per-source unique contacts, shared IMEI/IMSI/tower, one Alpha+
Charlie duplicate candidate, one Alpha+Bravo+Charlie conflict, and three
pairwise overlaps at `2026-08-18T05:00:00Z`. It completed in 95 ms total / 76 ms
database time. A tampered version failed closed with HTTP 400 and stable
`invalid_source_membership`.

Cross-family phone correlation returned cited CDR, IPDR and subscriber coverage
in 42 ms total / 25 ms database time. The valid IPDR/subscriber pair is time
eligible, the future pair is time-ineligible, and each one-sided no-match target
returns zero in the opposite family. Cross-source ANPR returned the cited East
then West sequence exactly 480 seconds apart in 35 ms total / 19 ms database
time. Results retain explicit limits against identity, ownership, presence,
association, causation, intent, guilt, route, or continuous-movement inference.

The compact runtime deck is
`reports/runtime-activation-stim34/closure-20260820.json`. All 279 forensic API
specs pass with two intentional skips, `go vet ./api/forensic_records` passes,
and the closure JSON plus STIM matrix parse. `STIM-3RuntimeStatus=ACCEPTED`,
`STIM-4RuntimeStatus=ACCEPTED`, `STIM-3FinalStatus=CLOSED`,
`STIM-4FinalStatus=CLOSED`, `OpenP0=0`, `OpenP1=0`,
`DatabaseMigrationNeeded=NO`, `DeploymentNeeded=NO`,
`AcceptanceDataIngestNeeded=NO`, and `RuntimeClosureDeckNeeded=NO_COMPLETE`.
STIM-5 remains not started and requires a separately bounded continuation.

## 2026-08-20 STIM-3/STIM-4 final source corrections pending activation — controlling checkpoint

The product-owner ran the next guarded forensic-API-only activation. Preserve
the marker `R7.10APIActivation=PASS BuildSeconds=14.8
ServicesRecreated=forensic-records-api ProfilesChanged=false
VolumesPreserved=true RollbackImage=preserved`. The orphan-container warning
remains warning-only; no cleanup was performed. The API is healthy and the
accepted seven-fixture pack remains retained, so no re-ingest is needed.

Closure replay now returns the exact seven accepted multi-CDR events, the common
and per-source unique contacts, shared IMEI/IMSI/tower, duplicate candidate and
three pairwise exact-instant overlaps required by the independent oracle. The
tampered source version still fails closed with HTTP 400
`invalid_source_membership`. The remaining missing conflict row was traced to
raw provider service tokens: `VOICE` and `CALL` were semantically equivalent in
the accepted adapter but distinct in the comparison key. The comparison now
reuses that canonical service-class contract, with a focused regression.

Cross-family phone and plate probes advanced past the optional-filter cast but
returned SQLSTATE `42804` because the bounded candidate `UNION` combined
`forensic_record_type` with varchar. Both branches and the final join now use a
text-normalized record-type boundary. All 279 forensic API specs pass with two
intentional skips, `go vet ./api/forensic_records` passes, and `git diff --check`
passes.

`STIM-3RuntimeStatus=BLOCKED_P1_PENDING_SERVICE_CLASS_ACTIVATION` and
`STIM-4RuntimeStatus=BLOCKED_P1_PENDING_UNION_TYPE_ACTIVATION`.
`STIM-3FinalStatus=OPEN`, `STIM-4FinalStatus=OPEN`, `OpenP0=0`, `OpenP1=2`,
`DatabaseMigrationNeeded=NO`, `DeploymentNeeded=YES_FOR_FINAL_SOURCE_CORRECTIONS`,
and `AcceptanceDataIngestNeeded=NO`. The exact next action is one guarded
forensic-API-only activation followed by the same read-only closure probes. Do
not begin STIM-5.

## 2026-08-20 STIM-3/STIM-4 second closure corrections pending activation — controlling checkpoint

The product-owner ran the second guarded forensic-API-only activation. Preserve
the marker `R7.10APIActivation=PASS BuildSeconds=25.2
ServicesRecreated=forensic-records-api ProfilesChanged=false
VolumesPreserved=true RollbackImage=preserved`. `/healthz` is healthy, the
retained seven-fixture acceptance pack remains present, and no re-ingest is
needed.

Read-only closure replay proved the earlier UUID/text source-membership repair:
the exact three-source request reached `records_sql`, while a tampered version
returned HTTP 400 with stable `invalid_source_membership` and the generic
non-enumerating message. The valid comparison completed in 79 ms total / 62 ms
database time, but returned only the two Bravo events rather than the seven-event
independent oracle. The SQL candidate prefilter was still comparing raw digit
forms and excluded Pakistan-equivalent `03...` and `0092...` source rows before
the correct Go canonicalizer could see them. Source now applies the same bounded
Pakistan phone equivalence at that prefilter.

Both cross-family acceptance probes reached analytical execution but returned
HTTP 500 with PostgreSQL SQLSTATE `42883` because optional record-family filters
compared the `forensic_record_type` enum to a text parameter. Source now casts
only those enum columns to text at the parameter boundary. All 278 forensic API
specs pass with two intentional skips, `go vet ./api/forensic_records` passes,
and `git diff --check` passes for the correction files.

`STIM-3SourceStatus=ACCEPTED`,
`STIM-3RuntimeCorrectionSourceStatus=SOURCE_CORRECTED_PENDING_SECOND_ACTIVATION`,
`STIM-3RuntimeStatus=BLOCKED_P1_PENDING_PK_PHONE_PREFILTER_ACTIVATION`,
`STIM-3FinalStatus=OPEN`, `STIM-4SourceStatus=ACCEPTED`,
`STIM-4RuntimeCorrectionSourceStatus=SOURCE_CORRECTED_PENDING_SECOND_ACTIVATION`,
`STIM-4RuntimeStatus=BLOCKED_P1_PENDING_ENUM_TEXT_ACTIVATION`, and
`STIM-4FinalStatus=OPEN`. `OpenP0=0`, `OpenP1=2`,
`DatabaseMigrationNeeded=NO`, `DeploymentNeeded=YES_FOR_SECOND_CLOSURE_CORRECTIONS`,
`AcceptanceDataIngestNeeded=NO`, and `RuntimeClosureDeckNeeded=YES_AFTER_ACTIVATION`.
The exact next action is one product-owner approved guarded API-only activation,
then replay the same four read-only closure probes. Do not begin STIM-5.

## 2026-08-19 STIM-3/STIM-4 acceptance ingest passed; UUID/text source correction pending activation — controlling checkpoint

The product-owner executed the guarded API-only activation with
`scripts/build_deploy_nexusai_r7_10_api_gate.ps1`. The forensic records API
image `nexusai/forensic-records-api:phase3-runtime` built successfully, only
`forensic-records-api` was recreated, `/healthz` passed, profiles were
unchanged, named volumes were preserved, and rollback image preservation was
reported. The retained warning about orphan container `nexusai-api-1` was
warning-only; no orphan cleanup was performed. Preserve the operator marker:
`R7.10APIActivation=PASS BuildSeconds=26.4 ServicesRecreated=forensic-records-api ProfilesChanged=false VolumesPreserved=true RollbackImage=preserved`.

The approved normal-path STIM-4 synthetic acceptance ingest is now accepted
after two runner corrections. The runner now formats and URI-escapes evidence
detail URLs, submits the explicit expected family for each synthetic fixture,
emits upload/poll progress, bounds each curl upload to 60 seconds, handles the
earlier validation-only `cdr_charlie.tsv` duplicate with one forced replacement,
and reports version IDs from `item.metadata.version_id`. The accepted retained
pack contains seven completed fixtures with exact accounting: 21 total source
rows, 20 accepted canonical rows, one duplicate row, and zero rejected rows.

Accepted fixture identities are: `cdr_alpha.csv`
`e6397cf8-68e4-4cc4-a3e6-d393003edb68` /
`78fc038b-b2c4-4a40-85f5-77f07fa2db5e`; `cdr_bravo.csv`
`5eea84ae-fa94-4300-8863-49dad4f11e9b` /
`762afcb9-291a-4033-9678-c2609ec7f02b`; `cdr_charlie.tsv`
`3e09a86b-1e82-4226-ab31-2e9dd8901a12` /
`bbaa6878-1135-4045-929b-691b0e4680fa`; `ipdr.csv`
`aab440fe-3d03-4f30-a894-4b169b449c39` /
`fdbce0ec-3f41-4989-9854-923972eaac2e`; `subscriber.jsonl`
`5fdbc11c-c5dd-4b08-9d8a-397db861ca9e` /
`af1e2f57-d696-474b-92b2-fd493fe43897`; `anpr_east.csv`
`4c679639-f3e3-4d97-b830-990af202382e` /
`600aa04c-7876-4e59-90f9-8a5f7172c6cc`; and `anpr_west.csv`
`15762aea-3db3-4a96-950c-667714fed474` /
`06f1fdcf-f830-4353-9f29-688bcb79e62c`.

The first read-only closure probe against the activated API then exposed one
new P1 source/runtime defect: the corrected source-membership SQL compared UUID
columns (`file_id`, `evidence_id`) to text-array source-set values, returning
HTTP 500 with PostgreSQL SQLSTATE `42883` before multi-CDR or cross-family
acceptance could be proven. Source is now corrected by text-normalizing those
exact UUID boundary comparisons in both the membership preflight and the
selected CDR analytical row binding. `go test ./api/forensic_records` passes in
25.589 seconds with the workspace Go cache, `go vet ./api/forensic_records`
passes, PowerShell parser validation passes for the ingest runner, and the STIM
matrix JSON parses.

`STIM-3SourceStatus=ACCEPTED`,
`STIM-3RuntimeCorrectionSourceStatus=SOURCE_CORRECTED_PENDING_ACTIVATION`,
`STIM-3RuntimeStatus=BLOCKED_P1_PENDING_UUID_CAST_ACTIVATION`, `STIM-3FinalStatus=OPEN`,
`STIM-4SourceStatus=ACCEPTED`,
`STIM-4RuntimeCorrectionSourceStatus=SOURCE_CORRECTED_PENDING_ACTIVATION`,
`STIM-4RuntimeStatus=BLOCKED_P1_PENDING_UUID_CAST_ACTIVATION`, and
`STIM-4FinalStatus=OPEN`. `OpenP0=0`, `OpenP1=2`,
`DatabaseMigrationNeeded=NO`, `DeploymentNeeded=YES_FOR_UUID_CAST_CORRECTION`,
`AcceptanceDataIngestNeeded=NO`, and `RuntimeClosureDeckNeeded=YES_AFTER_ACTIVATION`.
The exact next action is another product-owner approved guarded API-only
activation, followed by read-only closure probes using the retained accepted
fixture identities above. Do not begin STIM-5.

## 2026-08-19 STIM-3/STIM-4 runtime correction source acceptance — controlling checkpoint

The two runtime P1 root causes described in the prior checkpoint are now
source-corrected. Exact retained source/file/CDR-family/evidence/version
membership is validated inside tenant and collection before analytical SQL;
invalid membership returns HTTP 400 with stable code
`invalid_source_membership` and a non-enumerating message. Injection tests
prove rejected membership executes zero analytical queries.

Cross-family execution now uses one bounded `forensic.record_entities`
candidate lookup plus one non-CDR normalized-field parity expansion instead of
three repeated full expansions. Exact record/evidence/version provenance,
allowlisted entity types, tenant/collection/source/batch/date/family scopes,
unresolved-time nullability, typed validity validators and the 5,000/5,001
fail-closed bound are preserved. The controlled 5,000-row compiler measured
63.3869 ms; corrected deployed DB/API timing remains unclaimed.

The seven-file synthetic acceptance pack and hand-authored oracle are source-
accepted but were not ingested. Structured ingestion passes 102 tests with two
intentional skips; 14 focused STIM specs, the complete forensic Go package and
`go vet` pass. No build, deployment, restart, migration, backfill, retained
mutation, model/profile change, download, staging, commit or push occurred.

`STIM-3SourceStatus=ACCEPTED`,
`STIM-3RuntimeCorrectionSourceStatus=ACCEPTED_PENDING_ACTIVATION`,
`STIM-3RuntimeStatus=BLOCKED_P1_PENDING_CORRECTED_ACTIVATION`,
`STIM-3FinalStatus=OPEN`, `STIM-4SourceStatus=ACCEPTED`,
`STIM-4RuntimeCorrectionSourceStatus=ACCEPTED_PENDING_ACTIVATION`,
`STIM-4RuntimeStatus=BLOCKED_P1_PENDING_CORRECTED_ACTIVATION`, and
`STIM-4FinalStatus=OPEN`. `OpenP0=0`, `OpenP1=2`,
`DatabaseMigrationNeeded=NO`, `DeploymentNeeded=YES`, and
`AcceptanceDataIngestNeeded=YES`. The exact next action requires one approval
for the guarded API-only activation, normal-path controlled ingest in the
existing runtime-acceptance case, and read-only closure deck. Do not begin
STIM-5.

## 2026-08-19 STIM-3 and STIM-4 guarded runtime acceptance — superseded runtime evidence

The operator-executed single forensic-API-only activation passed. The accepted
source hash `9161a5b18214dc06b4e0802af872b4854f808f1003fa2bbd409e0929ff4eadbd`
matches the running API image, `/healthz` is healthy, and the runtime registry
contains 66 operations: five certified/queryable, 50 limited, and 11
engineering-only. Silently uncertified ordinary-user exposure remains zero.
The worker, PostgreSQL, NATS, LocalAI/UI, named volumes, two-model inventory,
profiles, rollback image and retained accounting were preserved. Post-deck
accounting remains 11 evidence items / 1,304,742 bytes, 9,278 canonical rows,
5,008 CDR rows, 11 jobs (10 completed, one dead-letter), and zero due work.

Certified `cdr.frequent_contacts` and `cdr.temporal_activity` returned exact
deterministic results with complete aggregate contribution lineage. Limited
IPDR, subscriber, tower and structured-ANPR operations remained visibly
`bounded_uncertified`, `limited`, and not suggestion eligible. Exact
CDR-to-tower runtime correlation matched two time-eligible observations with
both source rows preserved. Exact CDR-to-IPDR correlation covered 822 CDR and
453 IPDR observations but required 19.4 seconds. A legitimate retained
CDR-to-subscriber IMEI correlation exceeded the server context deadline. No
retained IPDR-to-subscriber identifier overlap or cross-source ANPR plate
overlap exists; those runtime cases remain
`NOT_EXERCISABLE_WITH_CURRENT_RETAINED_DATA`, with accepted source goldens as
the bounded evidence.

STIM-4 ran against two exact retained CDR sources and preserved both selected
source identities. The nominated target produced one matching event in only
one selected source; the live database has no phone-role value shared by two
retained CDR source files. Empty common/shared/overlap/duplicate/conflict sets
are therefore truthful, but the generated eligibility artifact that claimed a
two-source target is stale. Multi-CDR latency was 84 ms total / 68 ms database;
the representative SQL shape executed in 8.5 ms. Its citation carries the exact
file ID, filename, evidence/version IDs, row number/hash and observation ID.

Runtime closure is blocked by two P1 classes. First, syntactically valid unknown
sources and mismatched evidence/version identities return HTTP 200 and are
silently filtered instead of being rejected, although no unauthorized row
leaked; malformed, duplicate, one-source, nine-source, wrong-tenant and wrong-
collection requests do fail closed. Second, the broad cross-family
materializer scans/expands canonical JSON three times and its related-entity
join is operationally slow: one representative query took 27.4 seconds, the
CDR/IPDR overlap took 19.4 seconds, and the legitimate CDR/subscriber overlap
timed out. No index, schema, source or deployment correction was made.

Deferred P2 items are the stale CDR eligibility artifact; inconsistent
enterprise provenance presentation for some aggregate/limited answers despite
underlying row or contribution lineage; and family capability rows that merit
reconciliation when indexed-record count is zero. The likely smallest
performance correction is to reuse `forensic.record_entities` for correlation
instead of repeatedly expanding `metadata.normalized_fields`, then measure
whether a collection-aware entity lookup index is still needed. Any index is a
separately approved migration.

`STIM-3SourceStatus=ACCEPTED`, `STIM-3RuntimeStatus=BLOCKED_P1`,
`STIM-3FinalStatus=OPEN`, `STIM-4SourceStatus=ACCEPTED`,
`STIM-4RuntimeStatus=BLOCKED_P1`, and `STIM-4FinalStatus=OPEN`.
`DatabaseMigrationNeeded=NO_FOR_CURRENT_RUNTIME`; a later approved index may
change that. `DeploymentNeeded=YES_FOR_CORRECTION`. Do not begin STIM-5. The
exact next action is a bounded source correction proposal for source identity
resolution and cross-family execution, followed by focused source acceptance
and a separately approved single API-only refresh/runtime recheck.

## 2026-08-19 STIM-3 and STIM-4 source acceptance — superseded checkpoint

STIM-3 and STIM-4 are source accepted and await guarded runtime acceptance.
`STIMSkillLoaded=true`. The derived operation ledger was reconciled from
`supportedQueryTemplates()` and contains 66 operations: five independently
certified/queryable, 50 explicitly limited, and 11 engineering-only. Structured
scope excludes the KB-only evidence operation and is 65 total: four certified,
50 limited, and 11 engineering-only. Every ordinary-user structured operation
is independently certified or visibly limited; silently uncertified exposure
is zero. Engineering-only operations remain non-queryable.

STIM-4 adds `forensics.structured-source-set/v1` and
`forensics.multi-cdr-comparison/v1`. An exact two-to-eight-source, tenant/case/
source/evidence/version-scoped, read-only query compiles common and unique
contacts; incoming/outgoing, reciprocal/one-way direction; exact frequency and
duration; shared IMEI/IMSI/tower observations; exact normalized-instant
overlaps; duplicate candidates; explicit conflicts; per-source citations; and
bounded comparison/graph/timeline/conflict/limitation/provenance presentation.
It reads at most 5,000 selected-source observations and creates no persistent
graph or parallel relationship store.

The independent cross-family golden proves only typed and time-valid
CDR↔subscriber, CDR↔tower, IPDR↔subscriber, CDR↔IPDR, and exact-plate
ANPR↔ANPR primitives. Anti-correlation cases reject type collisions, invalid or
out-of-window time, same-source same-family pairs, and unsupported family
combinations. These relationships do not prove identity, ownership, guilt,
intent, causation, or continuous presence. Existing broad public cross-family
operations remain explicitly limited; financial, access/security, generic,
unsupported ANPR/telecom, and undefined INPR/INPRS relationships were not
invented or advertised.

Validation: the complete `api/forensic_records` Go package passes; `go vet
./api/forensic_records` passes; all nine focused STIM-4 specs pass; 1,000 pure
multi-CDR compilations complete in approximately 80 ms on the source host; all 99
structured-ingestion tests pass with two intentional dependency skips; the
operation and query ledgers parse and report 66 operations plus 156/156 passing
query occurrences (144 unique, 12 duplicate). Open issues remain P0=0, P1=0,
P2=5, P3=1. The new P2 is activation-time query-plan/latency measurement; it
does not weaken the exact 5,000-row bound.

No deployment, restart, database migration, retained-data mutation, model or
profile change, worker/UI/LocalAI rebuild, staging, commit, or push occurred.
The pre-existing materially dirty worktree remains preserved.
`DatabaseMigrationNeeded=NO`, `DeploymentNeeded=YES`,
`STIM-3SourceStatus=ACCEPTED`, `STIM-4SourceStatus=ACCEPTED`, and both runtime
statuses are `PENDING_ACTIVATION`. The only proposed operational action is the
existing guarded API-only gate, followed by a read-only runtime acceptance deck
covering source/runtime image parity, health, source-set rejection, scoped
three-CDR truth, conflicts, provenance, cross-family limitations, query plan,
latency, preservation, and rollback evidence. Stop before STIM-5 Natural-
Language Semantics and Grounded Fact Packets; no STIM-5 source work began.

## 2026-08-19 STIM-1 and STIM-2 source acceptance — controlling checkpoint

STIM-1 and STIM-2 are source accepted. `STIMSkillLoaded=true`. The two
foundational time P1s are closed by `forensics.time-policy/v1`: unresolved
slash-date ambiguity is rejected, naive timestamps require a governed source
timezone, explicit offsets remain self-describing, profile defaults are opt-in,
and Pakistan jurisdiction alone is not time proof. Independent goldens cover
unambiguous DMY/MDY, governed ambiguous DMY/MDY, explicit offset, unknown
timezone, allowed profile default, and cross-midnight conversion. The three CDR
layouts remain canonically equivalent.

STIM-2 adds one ingestion-side `forensics.schema-registry/v1`, versioned family
mapping profiles, `forensics.dynamic-attribute/v1`, structured quality and
capability readiness, protected subscriber attribute presentation, record/evidence
provenance, and the bounded Analyst Data field-mapping/quality/extensions/time/
readiness surface. Existing `raw_record`, `normalized_record`, evidence metadata,
legacy family tables and `forensic.records` remain the only persistence path;
no parallel raw store or migration was introduced. CDR, IPDR, subscriber,
tower/location, and structured ANPR have governed mappings; generic tabular has
common infrastructure; financial and access/security remain truthfully partial;
INPR/INPRS remain `NeedsDomainDefinition`.

Validation: all 99 structured-ingestion tests pass with two intentional
dependency skips; the complete `api/forensic_records` Go package passes; the
677-module production UI build passes; focused browser checks pass for source
maturity and unknown/unresolved intake policy. Existing CDR frequent-contact
and temporal-operation coverage remains green. Open issues are P0=0, P1=0,
P2=5, P3=1.

No Docker deployment, database migration, retained-data mutation, model/profile
change, staging, commit or push occurred. The pre-existing materially dirty
worktree remains preserved. `DatabaseMigrationNeeded=NO` and
`DeploymentNeeded=NO`. Exact next phase and action: STIM-3 Structured Analytical
Truth Certification, beginning with a bounded high-value operation pack and
independent result, citation, presentation and scope oracles. STIM-3 has not
started; stop at this boundary.

## 2026-08-19 STIM-0 baseline complete; STIM-1 CDR slice in progress (superseded)

STIM is now the active explicitly authorized program. APF-3 remains closed and
R8 remains valid/paused. Read-only checks returned HTTP 200 from LocalAI
`/readyz`, forensic `/healthz`, and `/app`; the installed chat and embedding
models are unchanged. No container, volume, retained evidence, database, model,
or profile was mutated. `DeploymentNeeded=NO`.

STIM-0 source work is complete. The repository-scoped governance skill lives at
`.agents/skills/nexusai-structured-intelligence-maturity/SKILL.md`; the living
architecture is `docs/design/nexusai-structured-intelligence-maturity.md`; and
the family-by-family matrix plus issue register is
`configuration/nexusai_stim_maturity_matrix.json`. INPR/INPRS are explicitly
`NeedsDomainDefinition`, not guessed product families. The reusable grounded
narrative corpus is
`api/forensic_records/contracts/stim-answer-narrative-benchmark-v1.json`; it is
defined but not yet executed under the future Fact Packet contract.

STIM-1's first bounded vertical slice is implemented. CDR adapter 1.3 emits
`forensics.schema-profile/v1` with per-column candidates, selected canonical
field, mapping state, inferred type, sampled completeness/patterns, required
group coverage, review state, and raw-preservation policy. Three independent
CSV/TSV CDR layouts encode the same two events with distinct headers, timestamp
representations, and provider extensions. A hand-authored golden proves common
canonical truth, explainable mappings, and raw extension preservation. No
database migration was introduced.

Validation: focused CDR/family tests pass 23/23; full structured-ingestion tests
pass 83 with two intentional skips; the matrix and benchmark JSON parse. The
bundled skill validator could not run because PyYAML is absent; manual
frontmatter, naming, link, trigger, and agent-metadata checks passed instead,
without downloading a dependency implicitly.

Open issues are P0=0, P1=2, P2=7, P3=1. The active P1s are: ambiguous slash
dates can be interpreted D/M/Y before M/D/Y without a declared order; and
`Asia/Karachi` may be used as a source-timezone default even when the source
does not establish it. Exact next action: add a versioned source date-order and
timezone provenance policy, fail safely on unresolved ambiguity, add
equivalence/rejection goldens, and surface review state through existing
metadata before extending schema profiles to another family.

## 2026-08-19 APF-3 final runtime acceptance — CLOSED

The operator completed the final guarded API/UI-only activation. The genuine
marker was `R8UIAPIActivation=PASS APIBuildSeconds=4.2
LocalAIBuildSeconds=97.1 WorkerRebuilt=false ModelsChanged=false
ProfilesChanged=false VolumesPreserved=true RollbackImages=preserved`. Running
API/UI image identities matched the newly built tags; corrected source hashes
remained stable. Worker, PostgreSQL, NATS, named volumes, retained evidence,
KB data, models, profiles and rollback images were preserved. LocalAI
`/readyz`, forensic `/healthz` and `/app` returned HTTP 200.

After clearing only the displayed Analyst conversation, the complete 18-case
deck passed from Case 1. Cases 1–3 passed in one conversation. Corrected Cases
7, 10, 16 and 17 passed semantically and operationally. Deterministic cases
used zero LLM latency; unauthorized execution, scope leakage, invented
capabilities, arbitrary SQL/tools and model scope widening were all zero.

`BrowserCases=18/18 PASS`, `OpenP0=0`, `OpenP1=0`,
`SourceRuntimeParity=PASS`, `Health=PASS`, and `SecurityScope=PASS`.

`APF-3SourceStatus=ACCEPTED`, `APF-3.7SourceStatus=ACCEPTED`,
`APF-3BrowserAcceptance=ACCEPTED`, `APF-3.7RuntimeStatus=ACCEPTED`,
`APF-3RuntimeStatus=ACCEPTED`, `APF-3FinalStatus=CLOSED`, and
`DeploymentNeeded=NO`.

Case 18's approximately 60-second capability-guard latency and Case 15's
approximately 47.6-second bounded synthesis latency remain deferred P2
performance work. IPDR representative-citation labeling/version-field clarity
is deferred P3; evidence ID, source-file SHA-256, source row and row hash were
available and no full-contribution proof was claimed. These findings do not
reopen APF-3. At this historical handoff, `NextProgram=STIM` and
`NextPhase=STIM-0`; the controlling STIM checkpoint above has now consumed that
boundary.

## 2026-08-19 APF-3.7 guarded correction deployment passed — later browser gate failed

The operator completed the authorized guarded activation. Preserve the marker:

```text
R8UIAPIActivation=PASS APIBuildSeconds=25.1 LocalAIBuildSeconds=406.1 WorkerRebuilt=false ModelsChanged=false ProfilesChanged=false VolumesPreserved=true RollbackImages=preserved
```

Post-activation LocalAI `/readyz`, forensic `/healthz`, and Analyst HTML each
returned HTTP 200. `nexusai-api-1`, the records worker, NATS and PostgreSQL are
healthy; the records API is running. Existing-volume and orphan-container text
was warning-only and no cleanup was performed.

`APF-3.7RuntimeStatus=DEPLOYED_PENDING_BROWSER_ACCEPTANCE` and
`APF-3RuntimeStatus=DEPLOYED_PENDING_BROWSER_ACCEPTANCE`. `DeploymentNeeded=NO`.
Do not mark either runtime accepted or close APF-3 until the required live deck
is supplied and shows no P0/P1. Do not rebuild, remove orphans, mutate retained
state, or start APF-4/R13.

## 2026-08-18 APF-3 final source closure — controlling override

APF-3.7 and overall APF-3 are source accepted. The bounded-composition planner
supports two or three registered read-only steps with typed dependencies,
exact bindings, strict scope/security/citation validation, bounded resources,
partial-failure semantics and candidate-correlation claim lineage. The existing
single-capability path remains unchanged.

The risk-tier ledger contains 65 operations: Tier A 4/4 fully certified and
queryable; Tier B 50 bounded uncertified and limited to explicit requests; Tier
C 11 bounded uncertified and engineering-only. There are zero known correctness
defects, zero open P0 and zero open P1. The query ledger remains 155/155 passing.
Full forensic API tests, focused agent contracts and static analysis pass.

The source-closure deployment boundary below is superseded by the successful
guarded activation above. Browser runtime acceptance is still pending. Do not
rebuild implicitly, mutate volumes/data/models/profiles, or claim APF-3.7
runtime acceptance before the deck passes. Authority:
`reports/nexusai-apf3-final-source-closure-20260818.md`. APF-4/R13 has not
started.

## 2026-08-18 team-lead demo runtime restart

At the product owner's request, the existing NexusAI containers were restarted
without rebuild, recreation, image change, volume mutation, model/profile
change, or retained-data mutation. The forensic API had exited after losing
its NATS connection; PostgreSQL and NATS were restarted first, followed by the
forensic API/worker and LocalAI/UI. Final state: LocalAI `/readyz` HTTP 200,
forensic `/healthz` HTTP 200, PostgreSQL/NATS/worker healthy, forensic API
running, and LocalAI/UI healthy. The governed template proxy returns all 65
current templates (62 records, one KB, two hybrid), and the demo collection
capability catalog returns HTTP 200. The Analyst Ask page was opened and left
ready at `http://localhost:8080/analyst/ask?case=nexusai-forensic-demo`.

That restarted runtime remains healthy, but APF-3.7 is not deployed into it.

## 2026-08-18 APF-3.4–3.6 live closure and strict APF-3.7 gate

The final guarded API/UI activation passed with API build 19.5 seconds and
LocalAI/UI build 49.5 seconds. Worker, model, profile and named-volume state was
unchanged and rollback images were preserved. The existing-volume/orphan
messages were warnings only; no orphan removal or volume mutation occurred.

APF-3.4 is live accepted. The exact three-turn chain returned eight contacts,
retained OUTGOING with counts 68/67/61/61/52/51/49/38, then replaced only the
target with `923001234567` and returned its one outgoing contact. Fresh
deterministic queries complete in approximately 0.1–0.5 seconds instead of the
previous 40-second synthesis path.

APF-3.5 is live accepted. A deliberately misspelled query invoked the actual
installed schema-bound model and returned `validated_model_proposal` in 43.7
seconds with operation `cdr.temporal_activity`, exact target, exclusive UTC day
bounds, no invented direction and four matched rows. English, Roman Urdu and
Urdu named-month equivalents use the deterministic fast path in 300/115/104
ms and return identical operation, target, day bounds and row count. Unsafe or
invalid proposals remain fail-closed; the model cannot control family, scope,
authorization, arbitrary operations, targets or unrequested directions.

APF-3.6 is live accepted. KB retrieval returns three cited results. Both
catalog-declared hybrid operations now return `records_sql + kb_rag`, 17
deterministic rows and three KB results instead of HTTP 500. The temporal UI
renders all 12 columns, including 10/45-second extremes and source locators.
Frequent-contact and temporal answers render complete aggregate contribution
lineage; the independent SQL oracle ties 68 and four contributing rows to the
correct evidence/version/source groups and digests.

The operation-certification ledger is now 2 `certified` and 63
`blocked_missing_fixture`; only the two independently proven CDR operations
were promoted. Registry/routing/parameter parity remains 65/65, but it is not a
substitute for independent calculation, citation, presentation and scope
proof for the remaining operations.

The authoritative derived query-variant ledger is now implemented and checked
against every retained source corpus. Current-source counts are 155
occurrences, 143 unique cases and 12 duplicate occurrences (not the historical
141/11): 65 canonical plus 78 additional unique variants. Unique language
counts are 124 English, 10 Roman Urdu, 7 Urdu and 2 mixed. All 155 occurrences
pass routing, parameter and semantic-equivalence checks; eight are contextual
follow-ups and four require clarification.

Verification: complete `api/forensic_records` passes; final certification-gate
tests pass; scoped UI lint has zero errors (18 existing warnings); the
677-module production build passes. Windows denied cleanup of one temporary Go
test executable after the test returned `ok`; this is not a test failure. No
upload, retry, reprocess, evidence deletion, migration, model/profile change,
worker rebuild, staging, commit or push occurred.

`APF-3.7Ready = NO`. APF-3.4, APF-3.5 and APF-3.6 are live accepted and current
foundational P1s are closed, but 63 executable operations still lack independent
goldens and full certification. `DeploymentNeeded: NO` for the accepted
APF-3.4–3.6 state. APF-3.7 remains not started under the directive's strict
gate; do not waive the remaining analytical-truth debt.

## 2026-08-18 APF-3.4-3.6 activation attempt failed; runtime recovered

The operator-run APF-3.4-3.6 preflight passed with no mutation. Two guarded
activation attempts built the new forensic API image but both failed before a
PASS marker when `http://localhost:8091/healthz` closed unexpectedly. Automatic
rollback restored the prior forensic API and LocalAI image tags. The activation
is not accepted and the APF-3.4-3.6 source delta is not live.

Read-only diagnosis found PostgreSQL, NATS and the records worker exited with
code 255. The rolled-back forensic API then exited because `forensic-nats`
could not resolve, and LocalAI initially returned case API 503 because its
agent-history pool started while `forensic-postgres` was unavailable. Recovery
started only the existing PostgreSQL, NATS, worker, rolled-back forensic API and
rolled-back LocalAI containers. No image was rebuilt during recovery and named
volumes were preserved.

Recovered state: PostgreSQL, NATS, worker and LocalAI are healthy; forensic API
`/healthz`, LocalAI `/readyz`, `/analyst/ask`, and `/api/v1/forensics/cases`
all return 200. Do not use `down`, `--remove-orphans`, prune or volume deletion.
The exact next action is one operator-run guarded gate attempt from this healthy
baseline; do not treat either failed attempt as deployment acceptance.

## 2026-08-18 APF-3.4 through APF-3.6 second tranche source accepted

APF-3.4, APF-3.5 and APF-3.6 are source accepted sequentially. APF-3.4
replaces implicit filter reuse with `forensics.follow-up-context/v1`: inherited
fields carry source turn/analysis, reason and expiry; tenant, user and case
scope are rechecked; explicit current-turn values win; stale, cross-scope and
standalone questions do not inherit. The Analyst Portal consumes inbound URL
prompts once and scrolls only its conversation pane, eliminating the reproduced
page-shift/blank-canvas defect while preserving scroll-up and Jump-to-latest.

APF-3.5 preserves the deterministic fast path and permits the installed text
model only to propose a strict `forensics.language-assistance-proposal/v1` for
an otherwise unresolved question. Unknown JSON fields, operations, families,
outputs, filters and scope are rejected. A live non-retained runtime probe of
`qwen_qwen3-4b-instruct-2507` proved JSON-schema response support. APF-3.6 adds
typed knowledge execution and model-capability contracts with authorized
evidence/chunk/hash/version/locator citations, no-result abstention, optional
reranker role, model-role/runtime/maturity/artifact/resource validation, and
explicit 62 deterministic + 1 KB + 2 existing-hybrid compatibility.

The authoritative operation certification ledger contains exactly 65 rows.
Routing and parameter registry parity are recorded, but independent computation
certification is truthfully 0/65; all 65 remain `blocked_missing_fixture` until
hand-verifiable or independent reference goldens prove result, citation and
presentation. A registration invariant rejects an executable operation with no
ledger row and rejects a certified claim without independent proof.

Verification: complete `api/forensic_records` passes; combined APF-3.1-3.6 and
certification tests pass; affected `core/services/agents` passes; focused Ask
Playwright passes 3/3; scoped UI lint passes. The full endpoint package ran all
239 internal specs successfully but the unfiltered package command then failed
because the repository contains two `RunSpecs` suites in one Go package; the
affected internal suite is verified separately. Full lint retains six unrelated
baseline violations in `Chat.jsx` and `coverage-fixtures.js`.

No database migration, retained upload/query/reprocess, model/data download,
worker rebuild, Docker deployment, staging, commit or push occurred. APF-3.7
remains not started. Backend and UI source changed, so one operator-authorized
guarded API/UI rebuild is required before live acceptance.

## 2026-08-17 APF-3 guarded activation halted and rolled back

The operator-run R8 UI/API gate passed read-only preflight with Docker 29.6.2
and 7.59 GiB free RAM. The APF-3 forensic API image built successfully, but the
activation halted when the API health request closed unexpectedly. Automatic
rollback restored the prior API and LocalAI image tags. Runtime acceptance did
not pass and no PASS marker was emitted.

Post-failure inspection found the preserved PostgreSQL, NATS and records-worker
containers exited with code 255. The rolled-back forensic API consequently
exited with code 1 after Docker DNS could not resolve `forensic-nats`; LocalAI
is healthy but logged an analogous inability to resolve `forensic-postgres`.
This is dependency-service availability after the activation attempt, not an
APF-3 compile/test failure. Named volumes remain attached and no destructive
cleanup is authorized. Exact next action: start only PostgreSQL, NATS and the
existing worker through the accepted sidecar Compose files, verify their
health, then rerun gate preflight and the guarded gate. Do not use
`--remove-orphans`, `down`, prune or volume deletion.

## 2026-08-17 APF-3.1 through APF-3.3 source accepted

The first APF-3 implementation tranche is source accepted. APF-3.1 adds
versioned typed query-understanding and read-only execution-plan contracts and
adapts the existing deterministic router without replacing it. APF-3.2 derives
one 65-operation resolved-capability projection from the executable template
catalog and joins workspace evidence readiness, authorization, runtime/model
availability, maturity, specialist ownership, result shape, presentation and
citation requirements. APF-3.3 validates and dispatches exactly one registered
capability through the existing deterministic/KB/hybrid execution paths.

The parity blocker is closed without deleting the older non-query platform
operations: all 65 executable query operations have stable descriptors and
reverse resolution, while the reconciled platform registry contains 80 total
descriptors (50 retained platform descriptors plus 30 previously missing query
descriptors). Unknown IDs, scope widening, unavailable data/runtime/model,
unauthorized capability, invalid required parameters, arbitrary SQL/URL/tool
IDs, multiple steps and disabled citations fail before execution. English,
Urdu and Roman Urdu legacy routing remains compatible.

Verification: 26 APF Go test events passed, 0 failed, 0 skipped; the complete
forensic package passed; its Ginkgo suite ran 258/260 specs with 258 passed,
0 failed and 2 intentionally skipped; `go vet ./api/forensic_records` passed.
One thousand deterministic understanding passes took 971.0366 ms in the
recorded focused run. Race mode was not available because the Windows Go
environment has CGO disabled. No schema, data, UI, model, worker, upload,
reprocess, Docker, R8/CCPD, staging, commit or push action occurred.

Live/runtime acceptance is pending. Backend source changed, so the guarded
deployment is required before browser acceptance; it was not run. APF-3.4 is
the sole next eligible work item and remains not started.

## 2026-08-17 APF-3 architecture/dependency reconciliation complete

At the architecture-reconciliation checkpoint, production implementation had
not begun. The newer controlling block above records the subsequent source
acceptance of APF-3.1 through APF-3.3.
The authoritative decision is recorded in
`docs/design/nexusai-apf3-unified-query-intelligence-architecture.md`.
NexusAI's forensic service owns scope, capability resolution, governed planning,
execution policy, fact/citation validation and response compilation; LocalAI
agents/models remain bounded language, tool, retrieval and inference runtimes.
The existing Ask/SSE/history APIs and both portal surfaces are reused.

Source truth is 65 executable query templates (62 records, 1 KB, 2 hybrid), 19
evidence-family capability definitions, 50 older platform operation descriptors,
16 specialist manifests and 10 model roles. The 65/50 catalog drift is the first
implementation dependency: APF-3 must create one derived, parity-tested
capability projection rather than another router or registry. APF-3.1 is the
sole next source slice: typed query-understanding/execution-plan contracts plus
an adapter around the existing deterministic router, with no API, database, UI,
model or retained-data change.

This session changed documentation only. No query, upload, reprocess, model
download, Docker build/deploy, migration, CCPD/R8 action, staging, commit or push
occurred. Deployment remains unchanged; R8 remains
`valid_but_paused_non_blocking`.

## 2026-08-17 APF-2 retained-runtime acceptance complete

APF-2 is live/runtime accepted through the existing Vite preview and deployed
backend contracts. The source-detail defect was corrected without changing the
backend: the drawer now resolves the selected evidence ID to the authoritative
catalog item, displays 4 verified records from 5 input rows with 1 duplicate
and 0 rejected, preserves factual zero, and renders missing accounting as
`Not available`. A second bounded P1 found during resumed acceptance was also
closed: citations without an embedded evidence ID resolve only when their
source filename uniquely matches the loaded catalog, so both Ask and History
can open the exact retained evidence.

The single authorized query, `Show temporal CDR activity for 923001234567 on
2026-07-10.`, created completed analysis
`c540c875-0cd7-410d-bd6b-3f2be87b0336`. Deterministic operation
`cdr.temporal_activity` returned 4 exact rows and cited rows 5 and 2 of
`pakistan_cdr_messy_synthetic.csv`; the citation opened evidence
`4320772b-febe-4af2-b9e3-92cea114da0b`. History advanced 50 to 51 and Continue
in Ask populated, but did not submit, the follow-up. Focused Playwright is
10/10, scoped ESLint has zero errors, the 677-module build passes, and real
390/820/1024/1440 dark/light inspection has no overflow or console errors.

No second upload, retry, reprocess, evidence repair, migration, Docker rebuild,
deployment, model/profile/worker change, staging, commit or push occurred.
APF-3 — Unified Query Intelligence / Capability Routing — is the reconciled
next item and has not started. R8 remains `valid_but_paused_non_blocking`.

## 2026-08-17 APF-2 retained-runtime acceptance halted on UI accounting truth

The user explicitly approved one retained upload of
`ingestion/forensic_records/tests/fixtures/pakistan_cdr_messy_synthetic.csv`
into the governed `nexusai-forensic-demo` case/collection. Preflight confirmed
the repository test references, 1,020-byte synthetic fixture, unchanged
`cf6792d8339f3a3cd79c2845eef2642d8fef480cc237062df30a20d6e17561e1`
SHA-256, unambiguous scope and no existing name/hash match.

The single authorized Add Data submission succeeded and is preserved:
evidence `4320772b-febe-4af2-b9e3-92cea114da0b`, version
`ad4b97c9-54eb-4657-afa7-5f06f52ba695`, job/run
`605b53d8-195f-4dde-aae6-081fbef6b500`, CDR/structured classification,
completed first attempt, 4 canonical rows from 5 input rows with 1 duplicate,
KB asset `8d772259-6d03-4fa8-923a-1f9c153d331d`, verified write-once hash
retention, and a valid 3-event custody chain with zero broken links. The Data
catalog advanced from 10/9 Ready/1 Failed and 9,274 accepted records to
11/10 Ready/1 Failed and 9,278 accepted records; Processing and Needs attention
remain zero. CDR indexed records advanced from 5,004 to 5,008.

Runtime acceptance is **halted, not passed**. The exact modal-to-source-detail
path rendered `0 verified rows` although backend evidence accounting and the
four-record preview prove 4 accepted canonical rows. Diagnosis: the case detail
response does not include `item.accepted_rows`; the modal callback opens the
drawer with only evidence ID and filename; the drawer therefore falls through
to zero at `AnalystData.jsx:247`. Per the approved failure rule, no Ask query or
history entry was created, no retry/reprocess/cleanup/repair was attempted, and
the retained evidence remains preserved. APF-2 stays active until this bounded
source-detail accounting defect is fixed and the acceptance path is resumed
without another upload. APF-3 must not start yet. No Docker rebuild is required.

## 2026-08-17 APF-2 unified Data intake source accepted

APF-2 is complete in source and remains active only at its retained-runtime
approval gate. The permanent ordinary-analyst Data journey now composes the
existing workspace collection upload, content-addressed retention, evidence
classification, scope-local duplicate lookup, transactional processing queue,
case evidence catalog/detail pagination, capability registry and read-only
reprocess-plan contracts. No second intake backend, queue or evidence store was
introduced.

The Data page supports click or page-drop entry, a multi-file modal, actual
upload progress, maximum two concurrent registrations, a durable per-file
ledger, partial success/failure, **Already added** duplicate communication, and
central **Ready / Processing / Needs attention / Failed** mapping. Ready source
actions are derived from live capability operations and `suggested_queries`.
Failed sources disclose original preservation and approval-gated immutable
reprocess planning; there is no Retry or execution control. The original
filename is visible while raw storage paths are not exposed.

Scoped ESLint reports zero errors; the 677-module Vite production build passes;
focused Analyst Portal Playwright passes 9/9. Real source-preview inspection at
`http://127.0.0.1:3000/analyst/data` verified the 10-source catalog (9 Ready,
1 Failed), Add Data in both themes and the failed-source recovery plan. No file
was submitted, no evidence/job/record/custody state was retained, and no Docker
image, backend, schema, model, worker, profile or R8 artifact changed.

Exact next action: obtain explicit user approval before uploading the bounded
repository-owned synthetic fixture
`ingestion/forensic_records/tests/fixtures/pakistan_cdr_messy_synthetic.csv`.
That action will create retained evidence, job, record and custody state. The
already deployed backend can perform it; a Docker rebuild is not required.
Frontend production activation remains a separate guarded deployment decision.
R8 stays `valid_but_paused_non_blocking`.

## 2026-08-17 APF-UX-1 analyst experience V2 source accepted

APF-UX-1 is complete in source. The simple `/analyst` product keeps only Home,
Data, Ask NexusAI and History, but now presents an action-oriented investigation
workspace rather than a dashboard or family inventory. Home leads with truthful
Add Data/Ask actions, attention state, capability-derived questions and recent
analyses. Data adds search, type/date/status filters and a human-readable source
desk with technical identifiers disclosed only on demand. Ask suggestions come
from the backend capability registry rather than a React prompt catalog. History
is a full-width searchable/filterable analysis list with cursor pagination,
reopen detail and Continue in Ask.

Acceptance is source-preview only. Scoped ESLint has zero errors; the Vite
production build passes; the focused portal/navigation/login suite passes 21/21.
Real-backend browser QA passed Home, Data, source detail, Ask, History and reopened
analysis detail at 390, 820, 1024 and 1440 pixels, with no horizontal overflow,
clean console, keyboard-operable account access and legible light/dark themes.
No Docker image was rebuilt or deployed, and no schema, evidence, case,
collection, model, worker, profile or external R8 artifact changed.

The deployment gate remains valid but was not invoked: APF-UX-1 can be reviewed
through Vite and should remain source-only while the product is iterative. APF-2
is now the one active work item: bind the existing governed evidence admission
and job contracts to simple multi-file upload, evidence-based or ambiguous
classification, progress, failure and retry. R8 remains
`valid_but_paused_non_blocking`; preserve every existing artifact and do not
resume transfer, evaluation or promotion during APF.

## 2026-08-17 analyst portal architecture rebase and APF-1 source acceptance

The latest team-lead directive establishes a separate simple ordinary-analyst
product without discarding accepted work. `/analyst` is now a distinct shell
with Home, Data, Ask NexusAI and History; `/app/cases/...` remains the advanced
Case Workspace. Both reuse one server-authoritative case/collection scope,
authentication, records/evidence APIs, governed agent lifecycle, typed result
presentation, citations and analysis history. Root and successful login enter
the analyst portal. Data is deliberately read-only in APF-1; upload is APF-2.

Read-only live inspection found one selectable default demo workspace with 10
evidence items, 9 ready sources, 1 failed source, 9 knowledge assets, 14
knowledge entries, 9,274 canonical records and 6 active agents. A real portal
query for the most frequent contacts of `923001110001` completed with eight
ranked contacts, exact counts/timestamps and visible deterministic-policy
fallback. Scoped ESLint has zero errors, Vite production build passes and
the focused portal/navigation set passes 16/16, including 4/4 dedicated portal
flows and 390-pixel overflow coverage. The full legacy lint command still has
six pre-existing hook-rule errors in
`e2e/coverage-fixtures.js`; that baseline was not altered.

No build was deployed and no schema, evidence, case/collection, container,
model, or external evaluation artifact changed. The required real query created
the normal governed history row `f7e12210-a7f2-42ed-a4ad-8144388ac72b`; it was
not deleted or rewritten. R0-R7 acceptance and the advanced Case Workspace
remain frozen. R8 is `valid_but_paused_non_blocking`; preserve its
T2-V, model packages, receipts, rejection evidence and stopped CCPD partial,
and do not resume transfer/evaluation/promotion or start R8.5 during APF. The
one active work item is APF-2: reuse existing evidence admission and job
contracts for simple upload, ambiguity-aware classification, progress, failure
and retry. A running-product activation is a separate explicit guarded
build/redeploy approval. The controlling design is
`docs/design/nexusai-analyst-portal-product-architecture.md`.

## 2026-08-13 R8 hardware tiers and exact next acquisition gate

NexusAI now has a versioned D0/D1/P1/P2 hardware contract while preserving
hardware-neutral forensic API and agent roles. The current Windows/i7-1260P/
15.7-GiB/Intel-UHD workstation is measured as CPU-first D0; Docker exposes 16
CPUs and 7.61 GiB RAM, and no CUDA/NVIDIA capability exists. Existing OMZ,
PARSeq and EasyOCR failures were audited as fair and remain reusable rejection
evidence. OMZ 0123 is deferred because its shared barrier-camera domain and
conversion burden provide no material improvement hypothesis.

FastPlateOCR `cct-s-v2-global` v1.1.0 is the sole selected next small
evaluation candidate. Its 5,263,955-byte package is acquired in the isolated
cache and independently matches both publisher SHA-256 values. A pinned
network-disabled CPU evaluator now preserves official RGB/uint8/64x128
preprocessing, raw character probabilities, explicit geometric-mean confidence,
latency/RSS/hardware provenance and unchanged partition scoring. Source tests
and the read-only T2-V/Docker/model-integrity preflight pass; build/inference is
deferred until the active CCPD transfer finishes, so the model is not measured,
accepted or promoted. CCPD2019 is authorized only for isolated offline
T3 distribution-shift evaluation: exact 13,164,924,944-byte Zenodo artifact,
publisher MD5, privacy controls, resumable acquisition and hostile-archive
validation are pinned. It is not downloaded. Boundary A/B remain blocked,
R8.5 is not started and production is unchanged. See
`reports/nexusai-r8-hardware-tier-and-boundary-a-next-acquisition-20260813.md`.

The first operator-run CCPD attempt exposed a curl retry defect: its displayed
118 MiB later reset and stopped with a preserved 28.35-MiB partial. A one-byte
publisher probe confirms correct HTTP 206/`Content-Range` support. The shared
downloader now re-reads the on-disk byte count for every attempt, appends only
when the exact range and total match, refuses an ignored/mismatched range, emits
periodic progress and supports long bounded retry. Fifteen focused tests pass.
Re-running the same acquisition command safely resumes the preserved partial.

The operator requested useful real-image progress before the full archive
finishes. NexusAI now has a five-image, 354,853-byte diagnostic pilot from the
official CCPD repository `rpnet/demo` at verified commit
`02aaea15137c4d2fe662e57d257c6822356e9304`. Every image matches its publisher
Git blob identity and a local SHA-256 receipt. The images are real and privacy
restricted, remain outside Git and are not demo assets. Because the publisher
provides no ground-truth file for them, this pilot can validate decode/crop/OCR
compatibility and failure behavior only: it grants no accuracy, tuning,
Boundary-A or promotion authority. A privacy-minimized FastPlateOCR diagnostic
retains neither crops nor recognized strings and emits only hashed/aggregate
observations. Its source tests and preflight pass; its isolated image build is
complete while the full CCPD transfer continues. The real pilot produced
nonempty, Latin/digit-charset-valid output for 5/5 images at 30.097 ms mean CPU
latency without retaining strings or crops. This is compatibility evidence, not
accuracy. Full offline T2-V evaluation is also complete: FastPlateOCR reaches
0.7949 exactness/0.1566 CER on development and 0.7857/0.1683 on both validation
and sealed holdout. Brier, latency and 106.484-MiB RSS pass, but exactness, CER
and abstention fail unchanged gates. It remains the best efficient Latin
reference and is `blocked_no_promotion`.

## 2026-08-13 R8 replacement-model acquisition and sealed T2-V evaluation

The operator approved bounded evaluation acquisition. Four candidate packages
were downloaded only to `C:\NexusAI-Evaluation\R8\Artifacts\1.0.0`, validated
against publisher checksums or exact release metadata, assigned NexusAI SHA-256
receipts and paired with license/NOTICE evidence. CCPD and Artificial Mercosur
were not downloaded because their immutable artifact/privacy gates remain
incomplete. Production roles, retained evidence and services were unchanged.

Network-disabled evaluation on the unchanged T2-V development, validation and
sealed holdout partitions is complete. OMZ 0106 produced zero true positives at
the predeclared 0.50 threshold. EasyOCR `arabic_g1` reaches 0.1795 development
and 0.0714 validation/holdout exactness, with CER 0.4057-0.4950. PARSeq-tiny is
strongest but still reaches only 0.7436 development and 0.7857
validation/holdout exactness with CER 0.2028-0.2079. Every candidate is
`blocked_no_promotion`. OMZ 0123 remains unevaluated because its artifact needs
a pinned TensorFlow-to-IR conversion lane. Boundary A/B remain blocked, R8.5 is
not started and production is unchanged. See
`reports/nexusai-r8-replacement-model-acquisition-and-t2v-evaluation-20260813.md`.

## 2026-08-13 R8 feasibility correction; T2-V generated and sealed

Physical T2-P is now truthfully `deferred_environment_unavailable`, not failed.
All source-accepted capture tooling and the empty external workspace are
preserved, but the operator must not be asked to continue manual capture and the
workspace is not evidence. Boundary A is explicitly amended without reducing
any threshold: it requires accepted T0/T1, controlled virtual T2-V, suitable
licensed real-image T3-D/T3-O, and license/privacy, hostile-input, accuracy,
calibration, abstention, resource and reproducibility passes. T2-P is optional
for Boundary A only under this stronger replacement. Boundary B is unchanged.

T2-V 1.0.0 is materialized separately under
`C:\NexusAI-Evaluation\R8\T2V\1.0.0`. Its 96 procedural, non-personal scenes
contain exact perspective polygons/crops and hashes, Pakistan-oriented Latin/
Urdu synthetic styles, difficult positives and 14 hard-negative classes across
56 development, 20 validation and 20 sealed-holdout images. Validation passes;
manifest SHA-256 is
`180889ad2dc78736cba6617ab49b3b910ddfd29e8dfb77a143fc9ce26f50e510`.
Offline incumbent inference is complete and all candidates fail every partition.
On development, the Paddle generic detector reaches only 0.1852 precision and
0.8889 recall; Paddle English OCR reaches 0.5897 exactness/0.1779 CER, Paddle
Arabic 0.2564/0.2384 and Tesseract 0.2564/0.5445. Validation and sealed holdout
also fail unchanged gates. T2-V is therefore not accuracy accepted.

CCPD is the primary T3-D/T3-O authorization target pending exact artifact and
privacy integrity. Artificial Mercosur is supplemental mixed-real evidence, not
sufficient alone for real OCR. No dataset/model was downloaded. Existing OMZ,
PARSeq and EasyOCR shortlists remain research-only. Boundary A/B remain blocked,
R8.5 is not started and production remains unchanged. See
`reports/nexusai-r8-feasibility-corrected-t2v-t3-boundary-a-20260813.md`.

## 2026-08-13 T2 capture mechanics automated; physical evidence boundary preserved

A local-only camera assistant now automates the time-consuming T2 mechanics:
loading the governed 32-slot plan, showing each condition, selecting the exact
pack `images` directory, capturing JPEG frames with deterministic filenames,
advancing progress and allowing controlled retakes. It has no external assets,
uploads, inference, normalization or annotation behavior. Localhost browser
acceptance passed with capture disabled before plan/camera setup and a clean
console. The assistant deliberately does not automate physical scene changes or
independent ground truth; generated screenshots would remain T1 and cannot close
T2. Production remains unchanged.

Operator feedback exposed two UX gaps, now corrected: every slot provides a
concrete arrangement card covering target choice, frame coverage, camera angle,
lighting, optical blur/occlusion and exact negative-scene construction; and the
assistant no longer reports camera readiness until live metadata, playback and a
non-zero frame resolution are verified. Gray/no-frame states fail closed with
privacy-shutter and camera-contention guidance. The runbook now gives the exact
slot-1 setup and explains separate-device display as an allowed physical target.

## 2026-08-13 T2 workspace verified; physical capture remains next

The operator successfully created `C:\NexusAI-Evaluation\R8\T2\1.0.0`.
Read-only verification confirms the correct capture plan and manifest contracts,
32 planned slots, 28 development/four holdout slots, zero manifest images and
zero captured files. This is successful preparation, not T2 acceptance.

To reduce capture mistakes, source now includes an unmistakably non-road-use
Latin/Urdu printable target sheet and a pre-annotation checker that rejects
missing, unexpected, empty, signature-mismatched or byte-duplicate captures.
The operator runbook includes its exact PASS marker. No external pack content,
model output, production evidence or production runtime was changed.

## 2026-08-13 R8 Boundary-A no-stall preparation; capture and approvals pending

R8.4 is now managed through parallel internal work items A–F without starting
R8.5. Safe engineering preparation is complete for the current blockers. T2 has
a reusable 32-slot one-session plan (28 development, four sealed holdout), a
workspace generator, stricter capture-condition/privacy/independent-review
validation and an exact operator runbook. Physical images remain user-operable
and absent, so T2 is not accepted.

Primary-source T3 research no longer depends on P-LPCD. Artificial Mercosur v1
is the preferred detector authorization candidate after privacy/source-rights
review; CCPD remains privacy/integrity gated; SYNLIP v2 is clear-license but
supplemental synthetic evidence only; INDO-ALPR is deferred; UFPR and RodoSol
remain non-commercial reference-only. P-LPCD remains blocked. Detector research
retains OMZ 0123/0106 for future approved evaluation and Paddle as baseline.
OCR research retains Paddle English/Arabic controls and identifies PARSeq Latin
and EasyOCR `arabic_g1` challengers. No dataset/model was downloaded.

New evaluator results attest candidate revision, exact fixture-manifest hash,
source tier, partition, preprocessing and environment; scoring adds detector F1
and incorrect high-confidence OCR acceptance. Boundary A remains blocked by
absent accepted T2/T3 evidence and failing candidates; Boundary B remains
blocked. Production is unchanged and R8.5 has not started.

## 2026-08-12 R8 two-boundary gate, T2 controls and bounded tuning; no promotion

The approved two-boundary policy is now applied. Boundary A permits only
non-production R8.5 source integration after T0-T3 evidence, license/privacy,
hostile-input, accuracy, calibration, resource and reproducibility gates pass;
T4 is not an engineering prerequisite. Boundary B additionally requires an
authorized T4 pack, live-case agreement, overlay/citation acceptance,
operational resources and explicit promotion approval. Both boundaries remain
blocked. The new T2 manifest and fail-closed validator are ready, but physical
assets and independent ground truth are not yet captured. Exact P-LPCD
publisher metadata conflicts with the authors' repository license (CC BY 4.0
versus CC BY-NC 4.0), so no T3 download is approved.

Bounded offline tuning used only the existing 29-fixture cache and left
production containers unchanged. OCR scale/contrast variants did not improve
exactness or CER. Detector long-side 640 improved precision to 0.85 while
retaining 0.9444 recall, but still fails the unchanged 0.95/0.95 gate; 1280
regressed. Error categorization, confidence/NMS diagnostics and a license-first
specialized detector shortlist are recorded without downloading alternatives.
R8.5 has not started. See
`reports/nexusai-r8-two-boundary-t2-t3-and-bounded-tuning-20260812.md`.

Last reconciled: 2026-08-12
Repository: `C:\Users\sheik\Workspace\Office\Projects\NexusAI`
Working branch: `codex/forensic-hybrid-checkpoint-20260723`
Base commit: `40717b83510c08db25dc26b9d6674bf46db363ac`

## 2026-08-12 R8 independent evidence program and expanded evaluation; no promotion

The permanent T0-T4 evidence contract is now established through one registry
and one supporting program document. R8's deterministic generator now produces
29 reproducible non-personal fixtures (6 T0, 23 T1), covers all 14 required
classes, exercises challenging positives, detector negatives and OCR abstention,
and prevents four hostile admission fixtures from entering model code. The
expanded offline run kept production containers unchanged. It rejects the
Paddle text-detector adapter at 0.6071 precision/0.9444 recall and all OCR
candidates: Paddle Arabic/English reach 0.875 normalized exactness with CER
0.06863/0.14706, while Tesseract reaches 0.5 with CER 0.31373. T2 capture
protocol and T3 authoritative research are recorded without downloading public
data; T4 operational agreement remains pending. Model notices are verified for
evaluation. No model is promoted and R8.5 remains blocked under the unchanged
T4 gate. See
`reports/nexusai-r8-evidence-program-t0-t3-and-expanded-evaluation-20260812.md`.

## 2026-08-12 R8.3/R8.4 first measured evaluation complete; superseded by expanded pack

The operator-approved isolated evaluator completed. Official Paddle detector,
English recognizer and Arabic-script recognizer assets were downloaded into the
dedicated evaluation cache, then all Paddle and Tesseract runs executed offline.
Production API, worker and LocalAI container identities were unchanged and no
production model role was assigned. The corrected role-aware scorer records a
promising Paddle detector synthetic baseline (precision/recall 1.0 at IoU 0.5)
but blocks acceptance because only 8/14 taxonomy classes are present, packaged
license notices remain to verify and no authorized real-image agreement pack exists. Paddle
Arabic and English each reach only 0.7143 normalized exact plate accuracy;
Tesseract reaches 0.4286. All OCR candidates fail the normalized accuracy/CER
and calibration gate. R8.5 is therefore not started. See
`reports/nexusai-r8.3-r8.4-measured-evaluation-decision-20260812.md`.

## 2026-08-12 R8 UI/API activation accepted; model evaluation execution superseded

The reported `forensic-records-api container is missing` failure was a false
negative in the activation script's container discovery. It occurred before an
image tag, build or service mutation. The gate now resolves the two exact
Compose containers, verifies their `nexusai` project/service labels from parsed
`docker inspect` JSON and exposes a read-only `-PreflightOnly` mode. Preflight
passed, then the guarded activation rebuilt and recreated only
`forensic-records-api` and `api`. Health/ready/UI checks passed, named volumes
and rollback images were preserved, and the worker, models and profiles were
unchanged. Live Evidence acceptance passed at desktop and 390x844 with no
horizontal overflow or browser warnings/errors. The active case reports 10
registered sources, 9 ready, one failed and no image evidence; therefore the
new candidate overlay is deployed but is not claimed live-image accepted.

The operator explicitly approved the non-personal synthetic fixture pack and
local OCR/model evaluation installation. Source includes a pinned isolated
PaddleOCR 3.7.0/PaddlePaddle 3.3.1 and Tesseract `urd+eng` evaluator plus a
deterministic fixture generator. Its initial execution blockage was later
resolved and the completed measured decision is recorded in the controlling
entry above. See
`reports/nexusai-r8-ui-api-activation-and-model-progress-20260812.md`.

## 2026-08-12 R8.3 platform source acceptance and R8.4 approval gate

R8.3 now has a strict `forensics.plate-region-candidate/v1` contract with
original-pixel bounds, immutable parent/version/hash identity, versioned
detector provenance, bounded confidence-first NMS, exact lossless crop
reconstruction/hash lineage and one-to-one localization metrics. Evidence
Operations renders recorded candidates in a responsive read-only overlay and
states that candidates are not proof. Go tests, JSON gates, the 669-module
production build and the 38/38 Agent Chat/Case Workspace Chromium matrix pass.

R8.4's read-only installed-inventory gate is complete. The runtime contains the
accepted Qwen text explanation and embedding models but no detector, PaddleOCR,
Tesseract, OpenCV, Ultralytics or Torch capability. The approved fixture
taxonomy contains no visual files. Therefore no localization/OCR accuracy,
license acceptance, model download or production-role claim is made. R8.3
accuracy acceptance and any R8.4 model promotion are explicitly blocked on
approval of a non-personal visual pack and local detector/OCR evaluation
installation. No service, evidence, model, profile, migration or retained data
changed. See `reports/nexusai-r8.3-r8.4-source-gate-20260812.md`.

## 2026-08-12 R8.1 complete and R8.2 source accepted

R8.1 reconciles the reusable case/tenant scope, content-addressed evidence,
custody/version, bounded media-header, structured ANPR, specialist,
model-governance, API and Case Workspace surfaces. Its threat model explicitly
covers forged MIME/extensions, malformed headers, decompression bombs,
oversized dimensions/bytes, active/vector formats, EXIF privacy, parser
resource bounds, cross-case access, immutable lineage and premature OCR claims.

R8.2 introduces `forensics.image-intake/v1`. Supported PNG/JPEG/GIF/BMP/TIFF/
WebP sources receive bounded signature, format, dimension, orientation,
color-model, byte and pixel decisions. Malformed, unsupported, mismatched or
resource-amplifying images remain preserved/manual-review evidence and cannot
enter downstream decoding. The Evidence desk exposes this admission truth and
staging boundary without claiming detection or OCR. The shared Ask UI also has
a tested Jump to latest recovery plus stable contained scrolling for the
backlogged inaccessible-content defect. Go tests, focused lint, the 669-module
production build and 2/2 focused Chromium scenarios pass. No data, model,
configuration, migration or deployed runtime changed. R8.3 is active. See
`reports/nexusai-r8.1-r8.2-source-acceptance-20260812.md`.

## 2026-08-12 R7.8 protected CDR acceptance and R7 closure

The explicitly supplied team-lead CDR passed an ephemeral read-only R7.8 audit.
All 3,931 rows normalized without error; 3,634 hashes were unique and 297 exact
duplicate hashes remained visible. Every row retained file/row/hash provenance,
explicit originating/called roles and Asia/Karachi-to-UTC provenance. All rows
carried supplied location context and 3,885 carried supplied coordinates; the
remaining coordinate absence was not filled. Direction/service classifications
reconciled, no manual-review quality flag was raised, and no raw row, identifier,
write, database query, model call, upload or retained copy occurred. R7.1-R7.10
are accepted at their governed boundaries and R7 is complete. R8 is active at
R8.1 entry reconciliation. See
`reports/nexusai-r7.8-protected-cdr-and-r7-closure-20260812.md` and
`docs/design/nexusai-r8-anpr-image-ocr-intelligence-contract.md`.

## 2026-08-12 R7.9 source and R7.10 live acceptance

R7.9 is source accepted and R7.10 is live accepted. The user-run guarded full
activation passed, all six version 1.2 profiles were applied, all 65
deterministic Agent Chat operations passed at 4,149.7 ms P95, and the corrected
CDR model-assisted regression completes six-family synthesis/fallback coverage.
The legacy Ask response now carries the governed typed map/timeline descriptors
instead of dropping them. A rollback-preserving API-only activation passed in
33 seconds without changing profiles or named volumes. The deployed response
publishes map and timeline descriptors with bounded rows, no route inference
and CDR/reference locators. Live desktop and 390 px UI acceptance confirms
synchronized map, timeline and selected-evidence detail with zero horizontal
overflow and zero captured console errors. R7.8 protected-CDR validation is the
sole remaining R7 gate and remains approval-bound; R8 has not started. See
`reports/nexusai-r7.9-r7.10-runtime-acceptance-20260812.md`.

## 2026-08-12 R7 activation preflight correction

The first user-run guarded rebuild stopped during read-only Compose
interpolation because the combined Phase 6 activation gate loaded but did not
export `NEXUSAI_AGENT_HISTORY_DATABASE_URL`. It therefore made no image,
container, volume, database, model, profile or case-data change. The shared
runtime helper now resolves the retained database URL from the current process,
the protected runtime environment or the newest retained Compose API container,
validates its PostgreSQL shape and exports it only to the gate process without
printing it. The combined gate calls that helper before its first Compose
command. PowerShell parsing, a redacted resolver unit check and read-only
two-file Compose interpolation pass. The guarded rebuild remains user-run and
approval-bound; R7.8 protected-data validation was not started.

## 2026-08-12 R7.6 and R7.7 source acceptance

R7.6 and R7.7 are source accepted. The existing
`forensics.enterprise-response/v1` visualization descriptors now reach Ask
NexusAI without losing nested scalar row values. Telecom results render an
accessible synchronized table, chronological event list, supplied-coordinate
plot and selected-observation evidence drawer. Tower CDR joins expose matched,
ambiguous and unmatched states with datum, uncertainty and exact source
locators. Empty coordinates stay empty; the specs and analyst text explicitly
disable inferred routes, RF coverage and device/subscriber presence.

The catalog is `2026-08-12.r7.7-source`. The primary analyst and operational
CDR, subscriber and tower profiles are version 1.2; the primary analyst exposes
the governed telecom operation surface while specialists remain case-bound,
cannot peer-delegate, require citations and fall back safely to the primary
analyst. The installed Qwen explanation role is unchanged and never becomes
the authority for facts or visualization coordinates.

The forensic API suite passes 248 specs (246 executed, two intentional skips),
the ingestion suite passes 80 tests with two intentional skips, the focused
presentation metadata suite passes, changed UI files lint clean,
and the React production build transforms 669 modules successfully. The full
agent package additionally passed 97 non-container specs but cannot start its
13 testcontainer-backed scheduler/store specs in this Windows sandbox because
rootless Docker is unsupported; this is an environment limitation, not an
assertion failure. The source preview loads at the application shell without
horizontal overflow. The deterministic Chromium telecom presentation scenario
passes 1/1 against the production source preview, including map/timeline
visibility, selection synchronization, uncertainty/source detail and 390 px
horizontal-overflow acceptance.

No Docker deployment, protected-CDR/data mutation, migration, model download or
change, profile application, staging, commit, push or publication occurred.
R7.8 protected-CDR validation is now active but approval-gated. R7.9 remains the
next fully synthetic/read-only executable hardening slice. See
`reports/nexusai-r7.6-r7.7-source-acceptance-20260812.md`.

## 2026-08-12 R7.4 and R7.5 source acceptance

R7.4 and R7.5 are source accepted. Tower adapter 1.2 /
`tower-location/v2` preserves raw provider/site/sector/radio aliases,
reference/version identifiers, coordinates, datum, method/source, uncertainty
and validity basis. Its deterministic history key separates provider, site and
sector. Half-open validity overlaps, coordinate/technology/status/datum and
uncertainty conflicts remain visible review candidates and are never silently
reconciled. Missing bounds remain labeled open/unknown and RF coverage, handset
position and movement inference remain prohibited.

The existing 65-operation query catalog remains stable. R7.5 adds exercised
natural variants for provider/sector history, overlap review, ICCID/service
links, packet/USSD usage and SIM/device history. Target-required requests
clarify before SQL; no-result responses remain bounded negatives. Focused tower
tests pass 6/6, the full ingestion suite passes 80 with two intentional skips,
and `go test ./api/forensic_records` passes. No deployment, protected-CDR/data
mutation, migration, model/configuration change, staging, commit, push or
publication occurred. R7.6 telecom map/timeline presentation is active. See
`reports/nexusai-r7.4-r7.5-source-acceptance-20260812.md`.

## 2026-08-12 R7.2 and R7.3 source acceptance

R7.2 and R7.3 are source accepted. CDR adapter 1.2 /
`cdr-canonical/v2` keeps legacy CDR columns and row hashes compatible while an
additive canonical projection preserves raw and Pakistan-normalized subscriber,
originating and called roles, deterministic direction/service classification,
source-timezone assumptions, UTC output provenance, provider/reference aliases,
sentinels and quality flags. The projection is mirrored into canonical record
metadata through the existing temporary stage; no database migration is needed.

Subscriber adapter 1.2 / `subscriber-identity/v2` now separates subscriber
MSISDN/reference, SIM IMSI/ICCID, device IMEI and service reference/type/plan/
provider. Links are labeled as explicit source-row co-observations with
open/closed/inverted-review/unknown validity; no ownership, physical use or
entitlement is inferred. Existing privacy policy remains: CNIC is masked and
names are omitted from public query/export rows.

The synthetic role golden is
`tests/fixtures/forensic_modalities/pakistan_cdr_subscriber_role_goldens_v1.json`.
Focused Python tests pass 10/10, the full ingestion suite passes 79 with two
intentional skips, and `go test ./api/forensic_records` passes. The Python
catalog loader was also reconciled with optional operation-input fields already
present in the shared platform contract, eliminating pre-existing suite errors.
No deployment, migration, data/model/configuration mutation, staging, commit,
push or publication occurred. R7.4 tower reference history and validity
deepening is active. See
`reports/nexusai-r7.2-r7.3-source-acceptance-20260812.md`.

## 2026-08-12 R6 final closure and R7.1 source acceptance

The product owner accepted the roadmap-governance directive: R6/R6.6 is closed
at its verified live boundary and R7 is active. The R6.6 marker reports
`live_accepted`, 65 templates, 66 corpus entries covering all 65 templates,
eight governance scenarios, zero mutating requests, and preserved rollback
images/volumes. The prior read-only live verification found all five services
running, required health/application endpoints at HTTP 200, and correct
clarification/deterministic query behavior.

One authoritative backlog now lives at
`docs/roadmap/nexusai-product-engineering-backlog.md`. The reported Ask NexusAI
blank-content/scroll issue did not reproduce during live R7-entry inspection:
the governed case, retained history, active answer, tables, provenance,
suggestions and input loaded. It is P2 monitoring work, not an R7 blocker.
Retained-history progressive loading, open-world analytical planning, further
natural-query coverage and future family renderers are scheduled without
reopening R6.

R7 phase-entry reconciliation is recorded in
`docs/design/nexusai-r7-telecom-intelligence-contract.md`. Existing CDR,
subscriber and tower adapters, the public forensic query API, operational
specialists, deterministic authority, provenance and current Qwen explanation
role are reused. The first bounded R7.1 source slice deepens `tower_cdr_join`:
every requested CDR observation now exposes exact and timestamp-eligible
reference candidate counts plus `matched`, `ambiguous_overlapping_references`,
`unmatched_no_reference`, or `unmatched_outside_validity_window`. Enterprise
metrics expose the outcome totals, and ambiguity remains visible rather than
silently reconciled. `go test ./api/forensic_records` passes. No Docker build,
deployment, migration, retained-data mutation, protected-CDR ingest, model
change, staging, commit, push or publication occurred. R7.1 is source accepted;
R7.2 Pakistan CDR normalization reconciliation is next.

## 2026-08-11 R6.6-B2 exhaustive predeployment acceptance

R6.6-A and B1 remain source accepted. Safe R6.6-B2 acceptance is now complete:
all 65 deterministic operations executed against the governed predeployment
runtime with exact template agreement (64 answered, one intentionally bounded
KB no-result); all six operational specialists completed a deterministic Agent
Chat path; the exhaustive source catalog test verifies input, scope,
calculation, output, limitation and presentation metadata; and runtime target
enforcement now consumes that catalog authority. Evidence-ID citations open the
governed Evidence workspace, and compact retained-history selection closes its
drawer while preserving the answer. The 669-module build and Agent Chat 17/17
pass. In-app 390/820/1024/1440 light/dark inspection has zero overflow and zero
console warnings/errors. R6.6 remains not deployed; the accepted prior R6 image
is live. Next: explicit approval for the guarded rebuild and postdeployment
runtime/model/manual acceptance. R7 remains blocked. See
`reports/nexusai-r6.6-b2-exhaustive-predeployment-acceptance-20260811.md`.

## 2026-08-11 R6.6-A Ask quality source acceptance

The user's R6.6 continuation directive reactivates R6 for a corrective quality
extension and blocks R7. R6.6-A is source accepted. Targetless frequent-contact
questions now return a specific clarification before SQL; the ranked query
requires one participant, accepts only phone-like counterparties, and excludes
the selected participant and service labels such as `INTERNET`. Source-file
inventory and case-readiness results receive direct, scoped executive answers.
The typed presentation contract now separates clarification, executive answer,
model interpretation and audit trace; primary metrics, columns and citations
are analyst-facing. The React layer also normalizes retained pre-R6.6 payloads,
so old saved analyses lose raw Markdown, placeholder metrics, raw provenance,
wide default tables and always-visible diagnostics without rewriting history.

Ask NexusAI is the visible analyst identity, the context ribbon is compact,
technical state is disclosed on demand, and result tables show five rows before
expansion. Verification: `go test ./api/forensic_records` passes; the focused
forensic presentation contract passes; targeted ESLint reports zero errors;
the Vite production build passes; Agent Chat Playwright passes 17/17; and an
interactive source-UI/live-API smoke verified the cleaned retained inventory
answer and readable sources. The broader agents package still has 13 unrelated
Windows rootless-testcontainers failures when run in full.

No Docker rebuild, retained-data mutation, model change, migration, staging,
commit, push or publication occurred. The deployed `:8080` backend remains the
accepted pre-R6.6 runtime. R6.6-B1 is also source accepted: a prior authorized
answer may return only a bounded target/template/date/direction filter packet;
the same-case client may send it back, and the planner reuses it only for
explicit follow-up language. It is request-scoped, non-persistent and cannot
grant access. “Only outgoing” is covered, while a fresh targetless frequent-
contact question still clarifies. The corpus now verifies all 65 catalog
examples plus 30 realistic natural/mixed-language routes. Next: R6.6-B2 must
run the representative data/browser matrix, then obtain explicit approval for
a guarded rebuild and live acceptance. Do not start R7.

## 2026-08-06 R3.1 acceptance override

Deployment update: the user executed the existing guarded combined Phase 6
gate. It reported PASS with 6.3 GiB free RAM, one LocalAI build attempt,
preserved rollback images and preserved volumes. Live verification found the
LocalAI, worker, PostgreSQL and NATS containers healthy; both readiness
endpoints returned 200. The deployed UI serves the accepted R3.1 asset hashes,
the governed demo case resolves its matching active collection and 9,272 rows,
390/1440 layouts have no page overflow, Settings requests no legacy logo, and
the browser console is clean. R3.1 is fully deployed/runtime accepted. R4-ARCH-01
is now source accepted: the complete case-context inventory and single
active-case contract are recorded in
`docs/design/nexusai-r4-active-case-contract.md` and
`reports/nexusai-r4-arch-01-case-context-inventory-20260806.md`. R4-ARCH-02 is
source accepted: one provider now validates and supplies App Shell, Records
redirect and Case Workspace identity; focused ESLint has zero errors, the
664-module production build passes and the protected browser matrix passes
35/35. Acceptance is recorded in
`reports/nexusai-r4-arch-02-active-case-provider-source-acceptance-20260806.md`.
R4-ARCH-03 is source accepted: embedded Records Intelligence now derives case
and collection scope only from the provider, removes alternate collection
authority, scopes query/upload/list/evidence/delete/export behavior, clears
case-derived state on switches and discards late prior-case answers. The
664-module build and 46/46 protected browser matrix pass. Acceptance is in
`reports/nexusai-r4-arch-03-bound-records-intelligence-source-acceptance-20260807.md`.
R4-ARCH-04 is source accepted: forensic Agent Chat now consumes provider-
validated case/collection identity, sends explicit scope, isolates case-keyed
conversation state and rejects late prior-case SSE/HTTP activity while generic
chat remains compatible. The 664-module build and 53/53 protected browser
matrix pass. Acceptance is in
`reports/nexusai-r4-arch-04-case-aware-agent-chat-source-acceptance-20260807.md`.
R4-ARCH-05 is source accepted: generic Collections and Collection Details are
explicit knowledge-administration surfaces, and case entry appears only for an
exact selectable registry mapping. Reports is now an on-demand Case Workspace
module whose browser adapter locks distinct provider case/collection IDs and
whose V1 compatibility endpoint rejects either body ID when it conflicts with
the URL case. Focused ESLint has zero errors, the 664-module production build
passes, the focused Go scope test passes, and live read-only production-preview
acceptance verified the explicit transition, a real 13-section non-retained
report, clean console and zero overflow at 820/390. Acceptance is recorded in
`reports/nexusai-r4-arch-05-administrative-report-source-acceptance-20260807.md`.
R4-ARCH-06 is source accepted: Case Workspace is now an elite eight-module
investigation desk covering Overview, Ask, Evidence, Relationships, Timeline,
Media, Reports and Admin. A reusable command-center shell presents one calm
case header, distinct locked case/collection scope, a descriptive desktop rail
and a contained mobile navigator. Ask reuses deterministic Records
Intelligence; Timeline exposes evidence-backed workflows; Media remains a
truthful registered-inventory boundary; Admin consolidates processing and
governance. Legacy Analyze/Jobs/Settings URLs redirect explicitly without
losing query context. Focused ESLint has zero errors, Vite 8.0.16 builds 667
modules, and the complete protected Chrome matrix passes 61/61. Interactive
production-preview inspection found zero console errors and zero desktop/mobile
page overflow. Acceptance is recorded in
`reports/nexusai-r4-arch-06-modular-workspace-source-acceptance-20260807.md`.
The user then ran both guarded combined gates. The corrected second gate passed
with 7.22 GiB free RAM, one LocalAI attempt, rollback images preserved and
volumes preserved. LocalAI `/readyz`, records `/healthz`, NATS `/healthz` and
`/app` returned 200; the corrected deployed skip-focus contract passed 1/1 and
the protected runtime matrix passed 61/61. R4 is complete. R5-EVID-01 is now
source accepted: Case Workspace has a case-scoped Evidence Operations desk and
detail API with exact processing accounting, lineage, artifacts, integrity,
immutable run history, filters and safe staged intake. Its 669-module build,
focused Go/auth checks, Case Workspace 12/12 and protected production-preview
matrix 62/62 pass. R5-EVID-02 is also source accepted: the same authorized
detail contract now exposes canonical runs/events, stable derived-artifact
citations and hash-chain-verified append-only custody history. Its focused Go
contracts, 669-module build, 12/12 workspace suite, 62/62 protected matrix and
1440/390 detailed-overflow checks pass. No R5 deployment or retained evidence
mutation occurred.
Acceptance is recorded in
`reports/nexusai-r5-evid-02-custody-citation-source-acceptance-20260807.md`.
R5-EVID-03 is source accepted and remains source-only: the catalog
returns exact look-ahead pagination metadata; collection status exposes recent
queue jobs; the Evidence desk supports server-backed navigation, bounded
two-at-a-time registration and durable per-file success/failure outcomes.
Focused Go contracts, the 669-module production build and the complete Case
Workspace browser suite pass 13/13, including pagination, queue visibility and
mixed bulk-intake outcomes. No deployment, upload, reprocess, migration or
retained-data mutation occurred. See
`reports/nexusai-r5-evid-03-safe-intake-pagination-source-progress-20260810.md`.
R5-EVID-04 is source accepted and remains source-only: the public contract now
publishes a case-bound evidence reprocess-plan workflow, while Evidence
Operations presents terminal-job eligibility, immutable generation context and
an explicit approval boundary. The review route is read-only and the UI never
posts a queue action. Focused forensic Go tests, the 669-module production
build and the complete Case Workspace browser suite pass 14/14. No deployment,
upload, reprocess, migration or retained-data mutation occurred. See
`reports/nexusai-r5-evid-04-governed-reprocess-contract-source-acceptance-20260810.md`.
R5-EVID-05 source-closes R5: pinned Swag v1.16.6 refreshed all generated API
artifacts; public documentation records the read-only approval boundary; and a
guarded R5 activation gate reuses the proven rollback-safe combined rebuild
before GET-only health, application, catalog, workflow and plan checks. Focused
forensic Go, generated-contract parsing, PowerShell syntax and the 669-module
production build pass; the unchanged Case Workspace acceptance remains 14/14.
No deployment or retained-data mutation occurred. R5 remains program-in-progress
at `R5-DEPLOY-01` until the operator-run rebuild and separately acknowledged
synthetic upload-to-ready gate pass. See
`reports/nexusai-r5-source-closure-and-live-rebuild-runbook-20260810.md`.
The operator-run R5 deployment then passed with API/worker/LocalAI build times
19.0/5.8/417.3 seconds, one LocalAI attempt, 7.37 GiB free RAM, rollback images
and volumes preserved. Both health endpoints returned 200 and the exact
read-only reprocess-plan contract passed with zero mutating gate requests. The
explicitly authorized live intake proved independent `1 registered, 1 failed`
outcomes. A corrected CDR fixture completed 2/2 rows and reconciled case rows
from 9,272 to 9,274. Same-second custody events then exposed false API broken-
link reporting and nondeterministic trigger-tail selection. Source corrections
now verify predecessor topology and backend-declared HTTP-200 upload failures;
Go, the 669-module build and Case Workspace 14/14 pass. The explicitly approved
migration 011 is now verified from a 5,707,090-byte rollback archive with zero
retained rows rewritten; API and worker health returned 200 afterward. At that
checkpoint R5 remained active at `R5-LIVE-02` pending one final corrected
refresh and live zero-broken-link acceptance. See
`reports/nexusai-r5-live-intake-and-custody-correction-progress-20260810.md`.

The operator-run corrected refresh then passed in one attempt with API/worker/
LocalAI build times 11.6/3.6/58.1 seconds, 6.42 GiB free RAM, rollback images
and named volumes preserved. Independent reconciliation found all five runtime
services running, PostgreSQL and NATS healthy, one governed case with zero
deletions, 10 evidence items and 9,274 canonical rows. The retained three-event
same-second custody chain has one root, deterministic predecessor topology,
zero broken links and `chain_valid: true`; the live Evidence UI reports `Hash
chain verified` at 390/820/1024/1440 with no page overflow or browser errors.
R5 is complete and live-runtime accepted. See
`reports/nexusai-r5-final-runtime-acceptance-20260810.md`.

R6 is now active. The capability audit confirmed 65 deterministic templates,
20 registered families (nine currently queryable), six operational specialist
agents and two unchanged approved models. R6.1-A is source accepted: Ask NexusAI
now verifies the live agent registry before exposing a specialist handoff,
renders unavailable specialists as non-interactive status, and keeps
deterministic analysis usable when agent discovery fails. Focused ESLint has
zero errors, Vite builds 669 modules and the Case Workspace suite passes 17/17.
R6.1-B is also source accepted: specialist continuation now returns to the
canonical Ask NexusAI route with the active governed case and the current or
latest analyst question preserved through send and case switching. Generic
Agent Chat remains unscoped and unchanged. The combined Agent Chat and Case
Workspace production-preview suite passes 25/25. R6.1 is source accepted.
R6.2-A is audit accepted. It found that final responses are request-correlated
but distributed status and stream events omit `message_id`; internal worker
cancellation exists without a chat cancellation route; execution deadlines
vary by mode; SSE reconnect has no replay or status reconciliation; and retry
has no idempotency contract. The active item is `R6.2-B — Correlated agent
lifecycle event contract`. It must correlate every status, stream and error
event before cancellation is exposed. See
`reports/nexusai-r6.2-a-execution-lifecycle-audit-20260810.md`.

R6.2-B is source accepted. Distributed and local lifecycle events now carry the
originating `message_id`; the browser requires request and governed-case matches
before updating live reasoning, tools, errors or spinner state; and `-error`
terminal messages resolve to their base request. Interleaved requests in two
conversations within the same case remain isolated. Focused Go correlation
specs and the agentpool compile gate pass, Vite builds 669 modules, and the
combined production-preview matrix passes 26/26. The broad agents suite's 13
database specs remain unavailable because rootless Docker test containers are
unsupported on this Windows host; 69 non-database specs ran. The active item is
`R6.2-C — Authorized cancellation endpoint and Stop action`. See
`reports/nexusai-r6.2-b-correlated-lifecycle-source-acceptance-20260810.md`.

R6.2-C is source accepted. A documented, authorized cancellation endpoint and
forensic Stop action bind cancellation to user, agent, case and request. Local
native, distributed worker and deterministic execution contexts are cancellable,
downstream forensic HTTP inherits cancellation, and the UI clears only after a
correlated `cancelled` status. Focused Go/compile gates, pinned Swagger
generation, the 669-module build and production-preview matrix 27/27 pass. The
active item is `R6.2-D — Consistent whole-request deadlines and timeout state`.
See `reports/nexusai-r6.2-c-authorized-cancellation-source-acceptance-20260810.md`.

R6.2-D is source accepted. Local native, distributed worker and distributed
deterministic execution now share one 210-second whole-request deadline and a
central terminal classifier. An elapsed deadline emits correlated `timed_out`;
analyst Stop remains `cancelled`; deterministic routes do not emit an
intermediate generic error for either context outcome; and the UI releases only
the matching request with an explicit no-retry message. Focused Go and compile
gates pass, Vite builds 669 modules, the Agent suite passes 11/11, the combined
production-preview matrix passes 28/28, and in-app preview inspection reports
the governed case ready with zero console errors. The active item is `R6.2-E —
Reconnect reconciliation and status lookup`. See
`reports/nexusai-r6.2-d-timeout-state-source-acceptance-20260810.md`.

R6.2-E is source accepted. A read-only, authorized status endpoint and bounded
30-minute/4,096-entry lifecycle registry isolate state by user, agent, case and
request. Distributed frontends observe scoped status independently of connected
SSE clients; local execution records the same contract. On reconnect, the UI
looks up only the current request, releases every terminal or unavailable state,
and never replays or retries work. Focused Go and compile gates, pinned Swagger
generation, the 669-module build, Agent suite 12/12 and deterministic combined
matrix 29/29 pass; in-app preview is clean. Two-worker diagnostics remain timing-
unstable at 28/29 in different unchanged five-second assertions and are not
claimed as passing. The active item is `R6.2-F — Idempotent explicit retry
semantics`. See
`reports/nexusai-r6.2-e-reconnect-reconciliation-source-acceptance-20260810.md`.

R6.2-F is source accepted. Explicit retry is limited to failed, timed-out or
cancelled forensic requests and is scoped by user, agent, case, original
request, message hash and caller idempotency key. Distributed frontends reserve
atomically in the existing observability table without schema migration;
standalone mode uses equivalent process-local semantics. Concurrent duplicates
return the winning message ID and do not dispatch twice; changed payloads
conflict; `retry_of` is preserved; no automatic retry exists. R6.3-A has
started at A1 with versioned, explicitly browser-local saved-analysis metadata
and a truthful **Saved analyses · This browser** UI. The enterprise history
contract is documented but no retained schema or server authority is claimed.
Focused retry tests, affected Go compile gates, zero-error focused lint, the
669-module build and pinned Swagger route validation pass. The active item is
R6.3-A2, retained history schema and governance design. See
`reports/nexusai-r6.2-f-idempotent-retry-and-r6.3-a1-foundation-20260810.md`.

R6.3-A is now source accepted across A1-A5. The additive
`agent_analysis_history` contract scopes every row by tenant, user, agent, case,
collection and request; records the accepted request before dispatch; and
retains terminal status plus the final answer while excluding hidden reasoning
and transient tool streams. The API-facing event bridge persists distributed
worker lifecycle events, while standalone mode uses the same store. Five
case-authorized endpoints provide opaque-cursor list, detail, idempotent saved
state, explicit bounded browser import and rollback that preserves saved and
legal-hold rows. The React workspace now treats the server as history authority,
keeps local sessions clearly separate and exposes an accessible mobile history
drawer. Pinned Swagger publishes 5/5 routes; focused history tests and affected
Go compile gates pass; focused ESLint has zero errors; Vite builds 669 modules;
the Agent production-preview suite passes 16/16; and independent in-app desktop
and 390px inspection found no console warnings/errors or horizontal overflow.
No deployment, schema activation, retained import or history mutation occurred.
The operator then ran the rollback-safe gate. R6.3-A is live accepted: API,
worker and LocalAI built in 2.7/3.1/57.9 seconds in one LocalAI attempt with
6.41 GiB free RAM; rollback images and named volumes were preserved; readiness,
shell and retained-history GETs passed; and the marker records zero mutating
requests.

R6.3-B is source accepted. Deterministic execution now emits the bounded
`forensics.agent-presentation/v1` contract through local/distributed delivery
and retained history with explicit authority, model status/role, elapsed time,
typed findings, records, citations, visualization descriptors, limitations,
next actions and trace. Agent Chat renders the same clean component for live
and retained answers, exposes elapsed governed stages during work and keeps raw
details collapsed. R6.4-A has begun at A1 with embedded corpus version
`2026-08-11.r6.4-a1`; the capabilities API publishes its CDR/IPDR/ANPR/
subscriber/tower/financial/cross-family entries and the UI offers only entries
backed by queryable families. Focused presentation and corpus tests, affected
compile gates, the forensic sidecar suite and the 669-module production build
pass. Read-only in-app acceptance found the governed case usable, zero console
errors and zero 390px overflow. No deployment or retained analysis mutation
occurred. The active item is `R6.3-B-LIVE`, followed by `R6.4-A2` corpus
expansion. See
`reports/nexusai-r6.3-b-presentation-and-r6.4-a1-corpus-source-acceptance-20260811.md`.
The first operator activation attempt then failed closed before any service
mutation because the retained history URL was present in neither the host
session nor the runtime env file. The gate now safely falls back to the running
Compose `api` container's existing URL, keeps it process-scoped and never
prints or persists the credential; explicit host and env-file values still
take precedence. The operator rerun passed: API/worker/LocalAI built in
14.6/4.5/139.5 seconds in one LocalAI attempt with 6.26 GiB free RAM, rollback
images and named volumes preserved. The `R6.3-B+R6.4-A1` marker is
`live_accepted`; retained-history and corpus GETs passed, both health endpoints
returned 200, and zero mutating requests were sent.

R6.4 is source accepted. The embedded `2026-08-11.r6.4` corpus now covers all
65 authoritative deterministic templates while marking only the curated set as
UI suggestions. Seven governed scenarios cover ambiguity, no data, unsupported
audio/OCR, incompatible model rejection and records/KB source access. The
versioned template catalog publishes operation, family, source, calculation,
output and limitation metadata for every operation; enterprise responses carry
the selected metadata; and typed Agent Chat results name the responsible
specialist and source boundary. Accessible table captions/column scopes,
keyboard-scrollable result tables and polite processing status begin R6.5-A.
Focused sidecar catalog/corpus/model-policy tests, the full forensic sidecar
package, all-65 Agent routing/title tests and typed presentation tests pass.
Vite builds 669 modules in 4.84 seconds, focused ESLint has zero errors, and the
complete Agent Chat browser suite passes 17/17. Independent in-app 1440×900 and
390×844 acceptance found the governed case usable with zero horizontal overflow
and zero console warnings/errors. Final route/auth and deployed-marker
reconciliation remain for R6.5-A. The
new `scripts/build_deploy_nexusai_r6_4_gate.ps1` is operator-only: it reuses the
proven rollback-safe build, requires exact 65-operation and corpus coverage,
sends GET requests only, preserves volumes/rollback images and writes
`r6.4-live-acceptance.json`. No deployment or retained-data mutation occurred
during this source slice.

The first R6.4 operator attempt failed closed before invoking the combined
rebuild because a new terminal had no host `NEXUSAI_AGENT_HISTORY_DATABASE_URL`
and Docker Desktop had stopped the complete Compose stack with exit code 255.
The absent marker and refused readiness connections were therefore expected;
the gate made no service or data mutation. A fallback defect was corrected:
Compose exposes the retained URL inside LocalAI as
`LOCALAI_AGENT_POOL_DATABASE_URL`, and the most recent container can be stopped.
The R6.3 base gate now inspects the newest running-or-stopped Compose API
container and accepts either the host-facing or mapped variable name while
keeping the value process-only. Credential-safe inspection confirms the mapped
PostgreSQL URL is present; both R6.3/R6.4 scripts parse successfully. R6.4 live
activation remains pending an operator rerun.

The next rerun exposed a second resolver-only defect before any build: Windows
PowerShell retained `$LASTEXITCODE = -1` after a Docker-output pipeline even
though Docker returned the correct current container ID. The fallback therefore
discarded valid output. Native exit-code checks were removed from these two
pipelines; the gate now validates the returned container ID and matching
environment entry directly. An exact credential-safe replay reports container,
mapped entry and assigned URL present, PostgreSQL validation true and
placeholder false. Live activation remains pending another operator rerun.

The final operator rerun passed. API/worker/LocalAI built in 12.6/3.3/81.5
seconds in one LocalAI attempt with 6.33 GiB free RAM; rollback images and named
volumes were preserved. R6.3-B and R6.4 activation passed with presentation
`forensics.agent-presentation/v1`, exact 65-template coverage, corpus
`2026-08-11.r6.4`, seven governance scenarios and zero mutating requests. The
R6.4 marker records 66 entries, nine curated suggestions, no schema-drop
attempt and both health endpoints at HTTP 200.

R6.5 is complete and R6 is fully source/live-runtime accepted. The complete
forensic sidecar package, isolated LocalAI forensic proxy/case/user boundary,
Agent lifecycle/history/retry/presentation suites, full auth package and
focused auth/usage route suite pass. The final GET-only R6.5 gate verified
eight live surfaces, six required specialist identities, catalog/corpus/history
contracts, `/assets/index-4Nowm5K2.js`, 231.99 ms local GET p95, zero mutations
and preserved rollback assets/volumes; it wrote
`reports/runtime-activation-20260810/r6.5-live-acceptance.json`. The deployed
browser shows the corrected corpus, no obsolete A1 template names, zero console
warnings/errors and zero page overflow at desktop and 390×844. A broad routes
diagnostic passed 18/19; its unrelated backend-upgrade fixture failed after a
blocked external OCI lookup and missing temporary `run.sh`, while the relevant
focused route suite passes. R6 has no remaining bounded deliverable. Preserve
this runtime; R7 is next but not started. See
`reports/nexusai-r6-final-runtime-acceptance-20260811.md`.

## Purpose and update protocol

This is the living factual handoff for NexusAI forensic-intelligence
development. `NEXUSAI_MASTER_DIRECTIVE.md` is the repository-level product and
program authority; the current roadmap and phase ledger govern active work.

Every future coding session working on this feature must:

1. Read `AGENTS.md`, `NEXUSAI_MASTER_DIRECTIVE.md`, the current roadmap/ledger,
   this file, and the task-relevant `.agents/*.md` guides.
2. Reconcile this document with the actual repository, Git state, services,
   database, model configuration, and test results before making assumptions.
3. Preserve all existing modifications and untracked files unless the user
   explicitly authorizes removal or replacement.
4. Ask before downloads, installations, full builds, database migrations,
   destructive operations, model changes, or external publication.
5. Update the Current status, Test ledger, Decision log, and Next action after
   every meaningful phase. Record facts and commands, not optimistic guesses.
6. Do not stage, commit, push, or open a pull request without explicit user
   authorization. When authorized, follow the AI-contribution policy in
   `.agents/ai-coding-assistants.md`; never add AI `Signed-off-by` or
   `Co-Authored-By` trailers.

## Mission

Build a local-first, CPU-capable multimodal forensic evidence platform on top
of LocalAI. The platform must ingest heterogeneous evidence, preserve source
provenance and integrity, normalize structured records, retrieve unstructured
knowledge, correlate entities and events, and return evidence-grounded answers
with stable citations and auditable processing history.

The current laptop is the reference development target. Improvements must be
measured against the verified local baseline before any model or architecture
replacement is proposed.

## Unified Knowledge Fabric contract

Every evidence item must participate in one logical fabric even when its bytes,
metadata, extracted text, vectors, structured rows, or derived artifacts live in
different physical stores.

Required identity and provenance fields:

- `tenant_id`: security and data-isolation boundary.
- `collection_id`: user-visible case or evidence collection.
- `evidence_id`: immutable logical identity for the source evidence.
- `version_id`: immutable identity for a particular source version.
- `content_hash`: hash of the exact source bytes.
- `source_uri` and `source_name`: recoverable source reference and display name.
- `media_type`, `modality`, and `classification`: detected type and routing facts.
- `ingested_at`, `created_by`, and `chain_of_custody`: provenance timestamps and actors.
- `processing_run_id`: the versioned pipeline execution that produced an output.
- `model_id`, `model_revision`, and `parameters`: reproducibility for model outputs.
- `artifact_id` and `parent_artifact_id`: lineage for extracted or derived material.
- `citation_locator`: stable pointer to a row, page, time range, bounding box, or text span.

Lifecycle invariants:

1. Source bytes are immutable after registration; corrections create a new
   version rather than silently overwriting evidence.
2. Every structured row, vector chunk, entity, relation, summary, transcript,
   thumbnail, and report traces back to a source `evidence_id` and version.
3. Reprocessing creates a new processing run and never erases prior provenance.
4. Retrieval results carry tenant, collection, evidence, version, and locator
   information through synthesis to the final citation.
5. Access control is enforced at storage and query boundaries, not solely in the UI.
6. Hash verification, deduplication decisions, failures, retries, and operator
   actions are append-only audit events.

## Verified reference environment

### Hardware and host

- Lenovo `21BVS0QX00`, Windows, PowerShell.
- Intel Core i7-1260P: 12 physical cores / 16 logical processors.
- 15.71 GiB installed RAM; Intel UHD integrated graphics; no CUDA GPU.
- Free physical memory measured by the user on 2026-07-23: 2.21 GiB.
- C: has roughly 1.92 TB decimal free space.

### Toolchain

- Go: `C:\Program Files\Go\bin\go.exe`.
- Python: `C:\Users\sheik\AppData\Local\Programs\Python\Python313\python.exe`.
- Docker Compose is available and the forensic compose file validates.
- GNU Make is not installed or not on `PATH`.
- WSL currently reports that no Linux distribution is installed.
- Host `protoc`, `buf`, `protoc-gen-go`, and `protoc-gen-go-grpc` are unavailable.
- Python `grpc_tools` is unavailable.
- Protobuf generation is available through the approved ephemeral image
  `golang@sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651`.
  The image uses Go 1.26.5 and runs the repository-pinned Protobuf 31.1,
  `protoc-gen-go` v1.34.2, and `protoc-gen-go-grpc` v1.4.0 toolchain.
- Generated `protoc`, `pkg/grpc/proto/backend.pb.go`, and
  `pkg/grpc/proto/backend_grpc.pb.go` are local ignored build artifacts. No
  host-wide tool installation was performed.

### Running services verified on 2026-07-27

- LocalAI API: `http://localhost:8080`, healthy, running image
  `nexusai-localai:universal-20260724` at
  `sha256:0731ab05fa080fae9bc90ddbcc26f74fcb554389ee82aa0282e5373a27144c9b`.
- Forensic records API: `http://localhost:8091`, healthy.
- Deployed forensic API image:
  `sha256:2fc0425044baeda5339cc4f06b8e242ef4a5734c70254e3d58861418dcec9473`.
- Deployed forensic worker image:
  `sha256:b0b66c87d11c556bcdee1a184806c270c0a393b57280436778fc2b7f4222e233`.
- LocalAI rollback is preserved both as the stopped container
  `nexusai-api-rollback-20260724` and tag
  `nexusai-localai:rollback-20260724-pre-universal` at image
  `sha256:033a78423049dae159821b2ee3e88801b8e41fd4cc7dcda65d81a1568beb20f5`.
- Forensic rollback tags include
  `nexusai-forensic-records-api:rollback-20260724-before-binary-header-fix`
  (`sha256:6b2d04b65f5e8b5742c00452b4ab52e3bf3fb7d4825780a2eebba3ecaff5076b`),
  `nexusai-forensic-records-worker:rollback-20260724-before-client-ip-fix`
  (`sha256:5b1753a6d72f7c11cd06a2b7d214224521dac46628177ad9359520199e6b9d88`),
  `nexusai-forensic-records-api:rollback-20260724-before-json-header-fix`
  (`sha256:69b5000a5b262de3cd15a1d03ec45020f9b598e66633a71266f42f8f37607723`)
  and `nexusai-forensic-records-worker:rollback-20260724-before-cdr-alias-fix`
  (`sha256:46b87426f887173c704e71c6807d0fa58e81a688fceb1e363729b861caace081`).
- PostgreSQL/TimescaleDB: host port `5433`, healthy.
- NATS: client `4222`, monitoring `8222`, healthy.
- Worker metrics: `http://localhost:9109`.
- `docker compose -f .\docker-compose.forensic-records.yaml config --quiet`
  succeeds.

### Active models and agent configuration

- Chat: `qwen_qwen3-4b-instruct-2507` using
  `Qwen_Qwen3-4B-Instruct-2507-Q8_0.gguf` (4,280,405,216 bytes).
- Embeddings: `qwen3-embedding-0.6b` using
  `Qwen3-Embedding-0.6B-Q8_0.gguf` (639,150,592 bytes), 1024 dimensions observed.
- Both models use `llama-cpp`, CPU mode (`gpu_layers: 0`), eight threads, and mmap.
- Chat parallelism is one. Embeddings are enabled.
- Configuration has a context-size ambiguity: top-level 4096 and nested
  `parameters.context_size` 8192. Resolve and test before increasing workload.
- Mean-pooling behavior for the embedding model is not explicit and should be verified.
- Agent `Forensic_Records_Analyst` uses the chat model, collection
  `records-demo-verified`, tenant `default`, Knowledge Base retrieval, and
  forensic-record querying.
- Runtime vector engine is `chromem`; agent-pool embedding model is
  `qwen3-embedding-0.6b`.

## Current evidence and behavior

Verified structured records in `records-demo-verified`:

| Type | Rows |
| --- | ---: |
| CDR | 5,000 |
| IPDR | 2,500 |
| ANPR | 750 |
| Access log | 1,000 |
| Total | 9,250 |

Verified processing state:

- Four completed ingestion jobs.
- Four completed evidence-registry entries.
- Four mirrored assets.
- No recorded failures, duplicates, or rejections in this dataset.
- The Knowledge Base collection contains eight entries, while the evidence
  registry contains four. Four direct Knowledge Base documents are therefore
  outside the unified evidence registry and must be reconciled.
- Read-only reconciliation on 2026-07-24 proved that those four source entries
  are two byte-identical content pairs:
  - `case_notes_records_demo.txt`: two 429-byte entries, SHA-256
    `11c370e9f4564786ebee8d76d894ffba52ea1a26918e3470f0a9e096cac47216`.
  - `policy_records_handling.md`: two 702-byte entries, SHA-256
    `7af6e64c1673b7115c7d10b40cd1e03e2b096fbec9b0d45fcaffa58e1bc62374`.
- The reviewed contract preserved four distinct KB source-entry links while
  deduplicating them into two content-addressed evidence records. It was applied
  on 2026-07-27 only after the scoped registry backup was verified.

Verified live hybrid query:

- Intent: `hybrid`.
- Route: `records_sql` plus `kb_rag`.
- Template: `evidence`.
- Structured rows returned: 12.
- Knowledge Base evidence results: 3.
- Planner confidence: 0.8.
- Database latency: 8 ms.
- Knowledge Base latency: 883 ms.
- LLM latency: 19,823 ms.
- Total latency: 20,720 ms.
- A grounded synthesis was returned without fallback.

Deployed source status: the bounded synthesis payload now includes up to four
retrieved Knowledge Base excerpts, each capped at 1,200 characters and scoped
with tenant, collection, evidence/version/chunk locators where available. The
system prompt treats excerpts as untrusted text and requires supplied locators
for citations. This change is package-tested and live-validated in the running
forensic API container.

Post-deployment live validation used the policy-plus-deterministic-records query
against `records-demo-verified`:

- Cold attempt: route `records_sql+kb_rag`, 12 structured rows, three vector
  results, 81 ms DB, 1,741 ms KB, 60,041 ms LLM, 61,867 ms total. The model hit
  the configured 60-second limit and safely returned the deterministic fallback.
- Warm attempt: same route/counts, 4 ms DB, 444 ms KB, 29,205 ms LLM, 29,655 ms
  total with a grounded summary and no warnings or fallback.
- The summary cited locator `2`. Direct KB inspection confirmed that result ID
  `2` was supplied by retrieval (`policy_records_handling.md`, similarity
  0.7756932), so the locator was not invented.
- Direct KB results IDs `1`, `2`, and `3` expose source paths and filenames but
  currently have no `evidence_id`, `version_id`, or `chunk_id`. This verifies the
  registry/lineage gap for direct KB-only documents.

## Implemented architecture

- NATS-driven forensic ingestion worker with health/metrics surface.
- Source-only scoped content-addressed raw retention with actual-byte hashing,
  create-only publication, read-only receipt/quarantine and worker re-verification;
  retained deployment still uses its prior storage behavior.
- Evidence registry, jobs, mirrored assets, and audit-oriented metadata.
- Header/value-based classification and structured adapters for CDR, IPDR,
  ANPR, subscriber, tower, transaction, access-log, and generic records.
- Canonical SQL-backed structured records and deterministic record queries.
- Entity observations.
- Rule-based query planner with structured, Knowledge Base, and hybrid routes.
- Bounded local-LLM synthesis with timeout and fallback behavior.
- React structured-evidence views and collection-aware assistant integration.
- Source-tested universal evidence registration separates preservation from
  processing: every valid KB upload is forwarded for registration, while only
  file formats supported by the current records worker are queued.
- The registration catalog covers structured records, documents, images, audio,
  video, STT transcripts, TTS outputs, packet/system captures, databases,
  archives, and unknown binary evidence.
- Non-record KB evidence receives an evidence ID, metadata-held version ID,
  SHA-256, modality/route, KB asset link, and transactional audit event without
  being misparsed by the records worker.
- KB-to-sidecar forwarding now streams multipart content and defaults to a
  configurable five-minute upload timeout for large media evidence.
- Hybrid retrieval can enrich KB results from the evidence/asset registry with
  tenant, collection, evidence/version IDs, retrieval chunk ID, and chunk hash.
- Phase 5A source now embeds a versioned adapter/operation/agent/model-role
  catalog, routes current CDR and generic worker selection/normalization through
  compatibility bindings, defines typed v1 query/enterprise response contracts, and exposes
  authenticated read-only sidecar discovery. It is not deployed and does not
  create additional runtime agents.

## Universal evidence format catalog

| Family | Current registration behavior | Current processing state |
| --- | --- | --- |
| CDR, IPDR, ANPR, subscriber, tower, financial transaction, access log | Detect record family from headers and preserve original hash/lineage | Queue CSV/JSON/JSONL/NDJSON/Parquet and recognized log/text inputs to records worker |
| Generic CSV/JSON/Parquet | Register as generic structured evidence with warning | Queue only when a parseable schema is available |
| TSV and XLSX | Register, queue and preserve source/dialect or workbook lineage | Streaming TSV plus bounded read-only XLSX row extraction and conservative per-sheet typed mapping operational in source; not deployed |
| XLS, XLSM, ODS, Arrow, Feather, Avro, ORC, XML | Register and retain recognizable record family | `structured_adapter_pending` or `tabular_adapter_pending` |
| PDF, Word, PowerPoint, OpenDocument, HTML, email, EPUB, Markdown/text | Register as document/text evidence | Existing KB indexing where supported; extraction/index route otherwise pending |
| PNG/JPEG/WebP/GIF/BMP/TIFF/HEIC/RAW/SVG | Register as image evidence | `image_ocr_vision_pending` |
| WAV/MP3/M4A/FLAC/OGG/AAC/Opus/WMA/AMR | Register as audio evidence | `audio_stt_pending` |
| STT transcript artifact | Register text artifact with optional parent evidence | `transcript_index_pending` |
| TTS output artifact | Register audio artifact with optional parent evidence | `tts_artifact_registry` |
| MP4/MOV/MKV/AVI/WebM/MPEG/MTS/3GP | Register as video evidence | `video_analysis_pending` |
| PCAP/PCAPNG/CAP/EVTX | Register as capture evidence | `capture_adapter_pending` |
| SQLite/database/SQL export | Register as database evidence | `database_adapter_pending` |
| ZIP/7z/RAR/TAR/GZ/BZ2/XZ | Register as container evidence | `archive_inventory_pending` |
| Unknown binary | Register without guessing its contents | `manual_review` |

“Pending” is an intentional safety state, not a claim that the modality-specific
extractor already exists. Each pending route must receive its own adapter,
fixtures, accuracy metrics, resource budget, provenance rules, and rollback gate
in the later phases.

## Current physical Knowledge Fabric inventory

| Logical stage | Current physical representation | Identity/lineage carried | Gap or next control |
| --- | --- | --- | --- |
| Source bytes | Scoped `sha256-scope-v1` content objects and receipts under `FORENSIC_SPOOL_DIR` in source; retained deployment still uses its prior layout | Scope hash, content SHA-256, actual byte size, stable object/receipt URI, source filename, retained/verified time and method | Deploy only after backup/build approval; select infrastructure WORM/object lock, legal hold and retention policy |
| Evidence registry | `forensic.evidence_items` | Evidence ID, tenant, collection, hashes, classification, metadata and extracted entities | Register direct KB-only documents and make version lineage explicit |
| Ingestion execution | `forensic.records_ingest_jobs`, `forensic.records_ingest_errors`; Phase 3 source adds `processing_runs/events` and queue lifecycle fields | Job, evidence, tenant, collection, file/batch, attempts, leases, retry/DLQ, reprocess lineage and row counters | Apply only through approved 008/009 migration and non-owner runtime rollout; adapter/model revision depth continues in Phase 4+ |
| Structured facts | `forensic.records` plus legacy/specialized `cdr_records` and `generic_records` | Evidence, tenant, collection, batch, source file/row, timestamp and normalized payload | Consolidate canonical use and retain exact source locators through every query |
| Collection routing metadata | `forensic.kb_active_metadata`, `forensic.kb_collection_assets` | Evidence where registered, tenant, collection, source entry, structured/RAG state | Reconcile the current eight KB entries with four registered evidence items |
| Vector chunks | LocalAI Knowledge Base collection using `chromem` | Collection, result ID, filename and source path returned by current KB APIs | Current direct KB results lack tenant/evidence/version/chunk fields; require them at indexing and retrieval boundaries |
| Entity observations | `forensic.record_entities` | Tenant, collection, batch, record family and source row | Add canonical entity resolution, confidence, aliases and persisted relations |
| Audit events | `forensic.records_audit_log` | Tenant, user, action, collection, file, batch and JSON details | Make processing/version/model events append-only and cover every derived artifact |
| Query planning | Typed `QueryPlan` and rule-based SQL/KB/hybrid routing in `query.go` | Query filters, target/date hints, confidence and execution strategy | Bind tenant to authenticated identity and evolve toward capability-aware modality routing |
| Synthesis | Bounded local chat-completion payload | Tenant, collection, SQL sample, up to four KB excerpts and available locators | Validate citation correctness and unsupported-claim rate on a gold set before deployment |
| Derived media/artifacts | Not implemented | None | Add artifact IDs, parent lineage, processing run, model revision and modality locators |
| Response citations | Enterprise provenance assembled from structured rows and KB results | Collection, source file/entry, row/timestamp or KB citation/score/preview | Standardize a stable citation-locator schema across all modalities |

## Known gaps and risks

Prioritize these gaps by evidence integrity, security, retrieval quality, then
new modality breadth:

1. Universal all-format forwarding is deployed in the main LocalAI image and
   universal registration plus corrected modern-CDR processing are deployed in
   the forensic API/worker. Multipart MIME preservation is test-passing in the
   current source but not deployed because its second full LocalAI rebuild
   stalled; the deployed path safely falls back to server-side content sniffing.
   The four existing KB-only source entries are now closed into two evidence
   objects and four source links; retain the verified pre-closure dump.
2. Scoped content-addressed retention, create-only publication, receipts,
   corruption quarantine and worker re-verification pass source/disposable tests
   but are not deployed. This local application contract is not regulatory WORM;
   production object lock/ACL, legal hold, retention, monitoring and recovery
   policy remain explicit gates.
3. Authenticated actor/subject/tenant/collection/case binding and non-owner RLS
   pass source/disposable tests but are not deployed. Secret rotation, protected
   service transport, edge-header stripping, runtime DB grants/ownership, and
   multi-tenant token mapping remain production gates.
4. File-backed JetStream acknowledgement, lease, bounded retry/DLQ and immutable
   reprocessing pass source and disposable restart acceptance but are not
   deployed. Persistent-volume backup/retention, queue monitoring and activation
   smoke remain production gates.
5. Classification has bounded HTTP signature sniffing and safe binary fallback,
   but still lacks a comprehensive magic-signature library, deep encoding
   diagnostics, and calibrated confidence/reasons.
6. Streaming TSV, bounded read-only XLSX and conservative per-sheet typed mapping
   pass source acceptance; legacy XLS, XLSM, ODS, deeper columnar formats and
   multi-provider schema-drift depth remain open.
7. Canonical entity resolution and persisted relation/graph layers are absent.
8. Image, audio, video, OCR, transcript, thumbnail, and other derived-artifact
   pipelines are absent.
9. Phase 3 processing runs/events and job lineage are normalized in the
   unapplied migration candidate; per-adapter/model revision capture and deeper
   artifact locators remain later vertical-slice work.
10. The planner is strong for present record queries but is not a general
    modality/capability planner.
11. The React UI now has a first progressive-disclosure simplification slice,
    but still needs separate analyst/evidence/ingestion/admin workspaces, media,
    artifact-lineage, processing-version, model views, and measured accessibility.
12. Existing `api/forensic_records` Go tests use standard-library style rather
    than the repository's current Ginkgo convention; treat this as technical debt.
13. The generated Swagger bundle omits the pre-existing forensic proxy routes.
    Source annotations are present, but offline regeneration is blocked on the
    pinned CLI's uncached `urfave/cli/v2`; download/container use needs approval.

## Git safety checkpoint

- Base `main` was at `40717b83510c08db25dc26b9d6674bf46db363ac` and matched
  the locally known `origin/main` at reconciliation time.
- Work was isolated on `codex/forensic-hybrid-checkpoint-20260723`.
- Local Git config uses `core.whitespace=cr-at-eol` to avoid treating CRLF as
  trailing whitespace and `core.hooksPath=.githooks` so project hooks run.
- Nothing has been staged, committed, pushed, or published.
- `Dockerfile` has a whole-file line-ending-only diff; do not normalize or mix it
  into semantic work without an explicit decision.
- Preserve the existing `.gitattributes` and all pre-existing edits.
- The pre-rebuild API image is preserved under the rollback tag above. To roll
  back, retag it as `nexusai-forensic-records-api:latest`, then run:

  ```powershell
  docker compose -f .\docker-compose.forensic-records.yaml up -d `
    --no-deps --force-recreate forensic-records-api
  ```

- The pre-universal LocalAI container is retained stopped as
  `nexusai-api-rollback-20260724`; do not remove it until the deployed image has
  passed the wider Phase 2A modality smoke matrix.

Pre-existing modified files at reconciliation:

- `.gitattributes`
- `Dockerfile`
- `api/forensic_records/main.go`
- `api/forensic_records/query.go`
- `api/forensic_records/query_test.go`
- `core/http/endpoints/localai/agent_collections.go`
- `core/http/endpoints/localai/agent_collections_param_test.go`
- `docker-compose.forensic-records.yaml`
- `docs/content/features/forensic-intelligence-phase1.md`
- `ingestion/forensic_records/worker.py`

Pre-existing untracked benchmark reports:

- `reports/forensic-model-benchmark.json`
- `reports/forensic-model-benchmark-qwen3-4b-cpu.json`
- `reports/forensic-model-benchmark-qwen3-4b-cpu-3rounds.json`
- `reports/forensic-model-benchmark-qwen3-combined.json`
- `reports/forensic-model-benchmark-qwen3-postfix.json`

Files added or intentionally updated for this living checkpoint:

- `NEXUSAI_CONTINUATION.md`
- `AGENTS.md` (link to this checkpoint only)
- `api/forensic_records/query.go` (bounded, scoped KB excerpts for synthesis)
- `api/forensic_records/query_synthesis_test.go` (new Ginkgo regression suite)
- `api/forensic_records/evidence_classification.go` (data-driven universal format catalog and safe processing gates)
- `api/forensic_records/evidence_classification_ginkgo_test.go` (cross-modality and lineage regression matrix)
- `api/forensic_records/modality_evaluation_matrix_ginkgo_test.go` (machine-readable modality contract validation)
- `configuration/forensic_modality_evaluation_matrix.json` (20-family fixtures, metrics, thresholds, failure behavior, and rollout gates)
- `core/http/endpoints/localai/agent_collections.go` (universal streaming evidence forwarding and lineage hints)
- `reports/forensic-phase2a-ingress-acceptance-20260724.md` (team-lead acceptance record with hashes, evidence IDs, routes, counts, lineage, defects and rollback state)

## Test ledger

| Date | Check | Result | Notes |
| --- | --- | --- | --- |
| 2026-08-10 | R5-EVID-04 governed reprocess controls and contract publication | PASS | Read-only case-bound reprocess plan; approval-required contract publication; Evidence Operations has no queue action; forensic Go PASS; Vite 669-module build PASS; Case Workspace browser suite 14/14 PASS; source-only |
| 2026-08-10 | R5-EVID-03 safe intake, pagination and queue observability | PASS | Focused forensic Go contracts PASS; ESLint zero errors (repository JSX false-positive warnings only); Vite 669-module production build PASS; Case Workspace browser suite 13/13 including pagination, queue state and 2-success/1-failure bounded intake; source-only |
| 2026-08-07 | R5-EVID-02 custody, citations and processing depth | PASS | Tenant/collection/evidence-bound queries; stable nexusai artifact refs; whole-chain custody verification; focused Go PASS; 669-module build; Case Workspace 12/12; protected matrix 62/62; detailed 1440/390 overflow 0; source-only |
| 2026-08-07 | R5-EVID-01 evidence operations source acceptance | PASS | Focused ESLint zero errors; Vite built 669 modules; endpoint fail-closed and auth package pass; Case Workspace 12/12; protected production-preview matrix 62/62; 1440/390 overflow 0 and console errors 0; no deployment or retained evidence mutation |
| 2026-08-07 | R4-DEPLOY-01 corrected guarded deployment and live acceptance | PASS | Second gate PASS with 7.22 GiB RAM, one LocalAI attempt, rollback/volumes preserved; corrected image and UI asset deployed; health checks pass; isolated focus 1/1 and deployed protected matrix 61/61 |
| 2026-08-07 | R4-ARCH-06 elite modular workspace source/browser acceptance | PASS | Focused ESLint zero errors; Vite 8.0.16 built 667 modules; eight-module Case Workspace; protected Chrome matrix 61/61; interactive 1440/390 overflow 0 and console errors 0; source-only, no deployment or retained-state change |
| 2026-08-07 | R4-ARCH-05 administrative transitions/report boundary source and live-preview acceptance | PASS | Focused ESLint zero errors; Vite 8.0.16 built 664 modules; Go body-scope assertion 1/1; explicit mapped transition and live 13-section non-retained report verified; 820/390 overflow 0; console errors 0; Playwright CLI unavailable because pinned browser binary is not installed |
| 2026-08-07 | R4-ARCH-04 case-aware Agent Chat source/browser acceptance | PASS | Focused ESLint zero errors; Vite 8.0.16 built 664 modules; focused Agents/Chat suite 7/7; protected production-browser matrix 53/53; no deployment or retained-state change |
| 2026-08-07 | R4-ARCH-03 bound Records Intelligence source/browser acceptance | PASS | Focused ESLint zero errors; Vite 8.0.16 built 664 modules; focused Records suite 11/11; protected R3/R3.1/R4 production-browser matrix 46/46; no deployment or retained-state change |
| 2026-07-23 | Python worker and Phase 2 adapter tests | PASS | 10 tests; user run 0.071 s, verification run 0.248 s |
| 2026-07-23 | `go test ./api/forensic_records` | PASS | User saw cached pass; verification run passed in 7.219 s |
| 2026-07-23 | Forensic Compose config | PASS | `config --quiet` |
| 2026-07-23 | Live critical hybrid query | PASS | Structured + KB + LLM; 20.720 s total |
| 2026-07-23 | Ephemeral `make protogen-go` | PASS | Repository target ran inside pinned Go 1.26 Docker image; generated files are Git-ignored |
| 2026-07-23 | Focused LocalAI endpoint test | PASS | Package result 6.466 s; command wall time 88.3 s after cold compilation |
| 2026-07-23 | `go test ./api/forensic_records -count=1` | PASS | Package result 8.699 s; includes legacy tests and new Ginkgo suite |
| 2026-07-23 | Focused bounded-synthesis Ginkgo test | PASS | Package result 8.292 s; verifies scope, four-result cap, untrusted-text instruction and citation locators |
| 2026-07-23 | Targeted `forensic-records-api` rebuild | PASS | 35.6 s; only API container recreated; DB, NATS and worker retained |
| 2026-07-23 | Cold live bounded synthesis | SAFE FALLBACK | 61.867 s total; configured 60 s LLM timeout triggered deterministic fallback |
| 2026-07-23 | Warm live bounded synthesis | PASS | 29.655 s total; three KB results, 12 structured rows, cited retrieved locator `2`, no warnings |
| 2026-07-23 | Post-rebuild API/NATS/worker health | PASS | API JSON health OK; NATS and worker metrics HTTP 200; API log contains no error |
| 2026-07-24 | Universal forensic API package tests | PASS | `go test ./api/forensic_records -count=1`; package result 10.890 s; 27 Ginkgo modality/lineage specs plus legacy tests |
| 2026-07-24 | Focused LocalAI universal upload forwarding | PASS | Package result 11.398 s using workspace `GOTMPDIR`; covers all-format forwarding, loop marker, streaming fields and timeout config |
| 2026-07-24 | Windows default-temp cleanup | ENVIRONMENT NOTE | First focused run reported `ok` in 29.911 s but Go could not unlink the temporary EXE; rerun with workspace `GOTMPDIR` exited 0 |
| 2026-07-24 | KB-only evidence reconciliation dry run | PASS | Four unregistered source entries hashed read-only; two exact duplicate pairs identified; proposed result is two evidence records plus four source links; no database write |
| 2026-07-24 | Targeted universal forensic API build/deploy | PASS | Corrected image `sha256:a4281a8c427f07fa458a978a00d1e07ffaa45ce91523e7fc940c3edfd3e043ba`; API `/healthz` OK, NATS health 200, worker metrics 200; only API container recreated |
| 2026-07-24 | Duplicate-source and truthful-state correction | PASS | Duplicate content now preserves each distinct KB source-entry link; adapter-pending media remains `registered` instead of being mislabeled `completed` |
| 2026-07-24 | Twenty-family modality matrix validation | PASS | `go test ./api/forensic_records -count=1`; package result 9.510 s; 20/20 required profiles, 8 ready fixtures, 12 planned fixture packs, classifier/metric/rollout gates validated |
| 2026-07-24 | First full LocalAI universal-ingress build | PASS | Approved CPU/RAM-bounded Docker build completed in 671.7 s; deployed image `sha256:0731ab05fa080fae9bc90ddbcc26f74fcb554389ee82aa0282e5373a27144c9b`; original container/image preserved |
| 2026-07-24 | Advisory-lock regression | PASS | Replaced PostgreSQL-invalid NUL-delimited advisory-lock text with an unambiguous JSON tuple; forensic Go suite passed in 18.870 s with workspace `GOTMPDIR` |
| 2026-07-24 | Modern CDR alias adapter | PASS | Python worker/adapter suite: 11 tests in 0.050 s; accepts `call_time`, source/target aliases, provided duration and `area` location |
| 2026-07-24 | Pretty-JSON and MIME forwarding regressions | PASS | Forensic Go suite passed in 15.337 s; compiled focused LocalAI suite ran 18 selected Ginkgo specs with 18 passed, 0 failed, 198 skipped |
| 2026-07-24 | Corrected forensic API/worker build and deployment | PASS | API `sha256:6b2d04b65f5e8b5742c00452b4ab52e3bf3fb7d4825780a2eebba3ecaff5076b`; worker `sha256:5b1753a6d72f7c11cd06a2b7d214224521dac46628177ad9359520199e6b9d88`; targeted build 33.1 s |
| 2026-07-24 | Second LocalAI rebuild for MIME preservation | TERMINATED | No output/progress for roughly 45 minutes and no `universal-20260724-r2` image was produced; only that build was terminated, then the successful first image was restarted and verified healthy |
| 2026-07-24 | Isolated universal-ingress live smoke | PASS WITH AUDIT HISTORY | In `forensic-ingress-audit-20260724`, text and pretty JSON completed; fresh modern CDR completed with 4/4 accepted rows and 0 rejected/duplicates. One earlier pre-fix CDR failure remains visible; `records-demo-verified` was untouched |
| 2026-07-24 | Binary-header/JSONB and access-log alias regressions | PASS | Python worker/adapter suite passed 12 tests in 0.194 s; forensic Go suite passed in 5.512 s. Non-worker binaries skip tabular header parsing; CSV headers reject NUL/invalid UTF-8/excessive size; `client_ip` maps to access-log |
| 2026-07-24 | Hardened API/worker targeted deployment | PASS | API `sha256:03a55612567e25fb04d9a8c1cec67bd2d35a7e4de30f6c67fd081e1582471bbe`; worker `sha256:d2ec7cf626d21b6be399017e87b660f5a679ef22962583afd91e4ff43c8f8d18`; prior images preserved under binary-header/client-IP rollback tags |
| 2026-07-24 | Twenty-family clean live acceptance matrix | PASS | `forensic-phase2a-acceptance-20260724`: 21 KB entries, 21 evidence items, 21 KB assets, 7/7 completed jobs, 20/20 rows accepted, 0 rejected/duplicates/failures/warnings; 13 pending/manual items had no records batch; STT/TTS parent lineage verified |
| 2026-07-24 | Orphaned BuildKit diagnosis and cancellation | PASS | History showed exact second-build ref `khk58bwg259mz3ck70wsdjidc` still `Running` with 0/0 steps since 05:54 after client termination. Removed only that record; running build-record count is now 0. No image/cache prune was performed |
| 2026-07-27 | SQLite raw inventory package validation | PASS | 69 Ginkgo specs; final `go test ./api/forensic_records -count=1` package time 29.579 s; `go vet` passed; evaluation matrix JSON parsed |
| 2026-07-27 | SQLite API-only build/deploy | PASS | Build/recreate 47.6 s, compile 33.6 s; deployed `sha256:9dc1ec79...`; API/LocalAI/PostgreSQL/NATS healthy, restart counts zero |
| 2026-07-27 | SQLite live acceptance | PASS | Two real synthetic SQLite files = 2 evidence = 2 KB assets = 2 KB entries; schema 4/4 per file and table counts `3+2` per file matched; WAL warning propagated; zero failures/jobs |
| 2026-07-27 | PCAP/PCAPNG inventory package validation | PASS | 77 Ginkgo specs; final package test 29.418 s; `go vet` passed; matrix JSON parsed |
| 2026-07-27 | Capture API-only build/deploy | PASS | Build/recreate 46.5 s, compile 34.7 s; deployed `sha256:bff1fd081...`; all services healthy and restart counts zero |
| 2026-07-27 | PCAP/PCAPNG live acceptance | PASS | 3 synthetic captures = 3 evidence = 3 KB assets = 3 KB entries; exact packet/interface/byte/time counts; secret warning propagated; zero jobs/failures |
| 2026-07-27 | MPEG media inventory package validation | PASS | 88/88 Ginkgo specs plus legacy tests; final package time 6.427 s; `go vet` passed; matrix JSON parsed |
| 2026-07-27 | MPEG media API-only build/deploy | PASS | Build/recreate 46.7 s; deployed `sha256:f24d90c5...`; API/LocalAI/PostgreSQL/NATS healthy, API restart count zero |
| 2026-07-27 | MP3/MP4/MOV live acceptance | PASS | 3 synthetic files = 3 evidence = 3 KB assets = 3 KB entries; exact frames/tracks/codecs/duration/rotation; mismatch warning propagated; zero jobs/failures |
| 2026-07-27 | Pakistan ANPR/PKR worker regressions | PASS | 21 Python worker/adapter tests in 0.157 s; 17 structured-adapter tests in 0.063 s; exact golden accounting, Urdu, timezones, PKR decimals and IBAN checks |
| 2026-07-27 | Pakistan vendor-header API regressions | PASS | Complete `go test ./api/forensic_records` package passed in 10.033 s after cold compile; ANPR and PKR JSONL aliases route to the records worker |
| 2026-07-27 | Pakistan structured live acceptance | PASS | 21 synthetic source rows = 11 unique accepted + 2 duplicates + 8 explicit rejects; 2/2 jobs/evidence/KB assets/KB entries completed; zero failures; protected 9,250-row baseline unchanged |
| 2026-07-27 | Phase 2 fixture-contract closure | PASS | 20/20 versioned ready profiles; 12 remaining modality generators reproduce exact SHA-256, classifier, metadata, warning and abstention goldens |
| 2026-07-27 | Final Python worker/adapter suite | PASS | 24/24 tests in 0.669 s |
| 2026-07-27 | Final forensic Go package and vet | PASS | 97/97 Ginkgo specs plus legacy tests; package 19.884 s; `go vet` passed; Compose, Python compilation, JSON contracts and diff whitespace passed |
| 2026-07-27 | Twenty-source Phase 2 live acceptance | PASS | 20 evidence/KB assets/KB entries; 8 completed jobs; 59 rows = 34 accepted + 6 duplicate + 19 rejected; 12 truthful pending/manual items; zero failures/missing assets |
| 2026-07-27 | Cross-family query acceptance | PASS | Subscriber 2 rows, relationship network 13, entity timeline 5, and corrected tower activity 1 cited tower-sector row |
| 2026-07-27 | Production registry closure | PASS | Verified 102,065-byte custom dump before write; two unique text evidence objects preserve four source links; final 8 KB entries/6 evidence/8 assets; 9,250 rows unchanged |
| 2026-07-27 | Unchanged three-round CPU baseline | PASS | 20/20 matrix; chat 18/18, lexical term agreement 1.0 and unsupported-claim proxy 0.0; embeddings 3 x 8 x 1024; fail-closed deterministic routes 27/27 |
| 2026-07-27 | Final services/resource snapshot | PASS | LocalAI/API/NATS/worker HTTP healthy; free RAM 1.38 GiB; API 13.53 MiB, worker 56.57 MiB, PostgreSQL 133.7 MiB, NATS 17.47 MiB, LocalAI 4.637 GiB |
| 2026-07-27 | Phase 3 migration contract regressions | PASS | 102/102 Ginkgo specs plus legacy tests in 3.152 s; focused Phase 3 package 19.321 s; `go vet`, Compose, JSON, Python compilation and diff whitespace passed; Python 24/24 in 0.297 s |
| 2026-07-27 | Phase 3 disposable database acceptance | PASS | Legacy backfill 1/1/3/1/1/1; native path added 1/1/2/1/3/3; second forward apply had zero drift; invalid preflight and forced-forward failure were atomic; gated rollback retained 2 Phase 2 evidence rows |
| 2026-07-27 | Running-registry Phase 3 preflight | PASS, READ ONLY | 110 evidence, 104 linked KB assets, 41 linked jobs; no compatibility failures; Phase 3 tables and pointer column confirmed absent afterward |
| 2026-07-28 | Phase 3 authenticated sidecar scope | PASS | 110/110 Ginkgo specs plus legacy tests; specs 1.322 s, package 10.049 s; missing/wrong credentials, untrusted tenant, scope switching and non-admin repair fail closed |
| 2026-07-28 | LocalAI ownership/forwarding and agent tools | PASS | Focused LocalAI suite passed in 33.574 s; final explicit-binary run passed 21/21 focused specs in 0.016 s, including delegated `agent-worker` provenance; complete agent-services package passed in 126.976 s |
| 2026-07-28 | Phase 3 non-owner RLS behavior | PASS, DISPOSABLE | Real `NOSUPERUSER NOINHERIT NOBYPASSRLS` role saw exact same-tenant rows, inserted same-tenant link, rejected cross-tenant link, and rolled back role/data; retained database untouched |
| 2026-07-28 | Auth slice offline regression | PASS | `go vet` clean for API/agents/LocalAI endpoints/routes; routes compiled in 15.774 s; Python 24/24 in 0.541 s; Python compilation and Compose config exited zero |
| 2026-07-28 | Route compile dependency cache | DISCLOSED | Compile populated missing Go-cache modules (`echo-swagger`, `flock`, `swag`, `openapi`, `yaml`, `swaggo/files`); no `go.mod`, `go.sum`, source, container, model or service changed; remaining Go checks used `GOPROXY=off` |
| 2026-07-28 | Swagger isolated offline preview | BLOCKED, NO CHANGE | Existing generated bundle lacks forensic proxy routes; pinned `swag` CLI requires uncached `urfave/cli/v2`; no download occurred, preview artifacts were removed, tracked Swagger stayed unchanged |
| 2026-07-29 | Phase 3 content-addressed storage contracts | PASS | Scoped path/URI, exact bytes, read-only receipt, traversal/symlink/tamper/corruption rejection, duplicate reuse and 12-writer convergence; five independent focused storage runs passed |
| 2026-07-29 | Phase 3 retention full offline regression | PASS | 119/119 forensic Ginkgo specs plus legacy tests; package test 24.397 s (99.6 s cold wall); `go vet` clean; Python 27/27 in 2.116 s; Python compile and Compose config passed |
| 2026-07-29 | Phase 3 storage migration acceptance | PASS, DISPOSABLE | Full 001–008 chain, legacy fixture, preflight, verify, native storage smoke, second forward apply, idempotency and non-owner RLS smoke passed; exact result `forensic_spool_content_addressed|verified|true|sha256-full-read-after-retain`; container removed and retained DB untouched |
| 2026-07-29 | Phase 3 queue/offline regression | PASS | 125 forensic Ginkgo specs registered: 124 passed and one live-NATS spec skipped offline; package 28.815 s; `go vet` clean; Python compile and 32/32 tests passed in 1.122 s; Compose config exited zero |
| 2026-07-29 | Phase 3 queue migration acceptance | PASS, DISPOSABLE | Full 001–009 chain plus 008 fixture/preflight/verify/smoke/idempotency/RLS, 009 preflight/forward/verify/smoke and second 009 apply passed; final assertion `phase3-db-ok|4`; container removed |
| 2026-07-29 | JetStream restart/dedupe/AckSync | PASS, DISPOSABLE | Two full forensic package runs passed in 8.018 s and 9.381 s; duplicate stable message stored once, survived file-backed NATS restart, recovered exact payload/message ID and drained after `AckSync`; container and volume removed |
| 2026-07-29 | Focused LocalAI queue/reprocess proxy package | RESOURCE TIMEOUT, NO PASS | Workspace and populated-cache runs exceeded bounded 300 s without compiler diagnostic on the low-memory laptop; no source failure observed, but CI/higher-memory validation remains required before deployment |
| 2026-07-29 | Phase 4A capability/TSV regression | PASS | Python 33/33 in 1.117 s; full forensic Go package 15.035 s; capability contracts, JSON/compile and React lint passed with zero errors and six existing warnings |
| 2026-07-29 | Phase 4B bounded XLSX goldens | PASS | 8/8 focused tests in 0.249 s: multisheet, hidden content, merged range, exact PKR/Urdu/raw values, 1900/1904 dates, formula/cache/error separation and corrupt/unsafe-package rejection |
| 2026-07-29 | Phase 4B combined source regression | PASS | Python worker/Phase 2/XLSX 41/41 in 1.277 s; full forensic Go package passed in 4.303 s; Python compile and three changed JSON contracts passed |
| 2026-07-29 | Phase 4C exact correlation goldens | PASS | Six focused Ginkgo specs passed in 20.838 s; 4/4 planner cases, 3/3 expected relation aggregates and 4/4 cited occurrences; exact SQL contract prohibits substring `ILIKE` matching |
| 2026-07-29 | Phase 4C combined source regression | PASS | Final full forensic Go package passed in 11.309 s; focused forensic agent-tools suite passed in 31.065 s package time (117.4 s wall); API+agents `go vet` passed in 15.9 s; JSON and diff checks passed |
| 2026-07-29 | Phase 4D per-sheet XLSX mapping | PASS | Focused XLSX 10/10 in 0.826 s; combined Python worker/adapter/XLSX 43/43 in 0.677 s; mixed CDR/ANPR/PKR transaction/notes mapping agrees 4/4 with exact sheet/row provenance |
| 2026-07-29 | Phase 4D API/UI source regression | PASS | Final full forensic Go package 31.249 s after capability/matrix sync; Python compile, golden JSON and diff check passed; page ESLint zero errors/four warnings; Vite production build passed in 2.52 s; static progressive-disclosure render inspected; no live API claim |
| 2026-07-29 | Phase 4E real CDR and typed-family matrix | PASS | Supplied CSV read-only audit: 3,931/3,931 accepted, zero rejected, 297 exact duplicates, 3,634 unique normalized and zero overflow; seven typed family matrix passes exact row accounting; audit emits no raw rows/identifiers |
| 2026-07-29 | Phase 4 structured v1 final source regression | PASS | Final Python 50/50 in 3.116 s; full forensic Go package 30.520 s; React production build 2.70 s; Python compile, JSON and diff checks passed; safe demo passed in 15 s; no live/deployment claim |
| 2026-07-29 | Phase 4/activation read-only runtime audit | DISCLOSED, NO CHANGE | LocalAI healthy on 8080; forensic PostgreSQL/NATS/API/worker stopped together exit 255 at identical engine timestamp, not OOM; Compose valid, rollback images present, 4.07 GiB free RAM; no service mutation |
| 2026-07-29 | Phase 3 activation preparation | READY FOR REVIEW, NO MIGRATION | PostgreSQL/NATS healthy while API/worker stayed stopped; 008 read-only preflight passed for 110 evidence/104 assets/41 jobs; 009 correctly blocked because 008 is absent and aggregate inventory found 7 failed jobs; 13,018,415-byte custom backup round-trip hash/catalog/full-read verified |
| 2026-07-29 | Retained migration 008 activation | PASS, MIGRATION 008 LIVE | Verified backup/hash and 110/104/41 preflight; transactional migration committed; official verifier returned 110 storage objects, 110 versions, 214 source links, 41 runs/events, 0 derived artifacts and 110 custody events; 7/7 RLS tables/policies, zero missing version pointers/invalid custody hashes; API/worker stayed stopped |
| 2026-07-29 | Legacy failed-job recovery diagnosis | PASS, READ ONLY | Seven unique historical jobs/evidence across two anonymous scopes: CDR 1, ANPR 2, IPDR 2, access-log 2; all attempt 1/5, zero rows/outputs/KB/audit/error payloads, two technical diagnostic signatures, seven legacy unverified spool objects, processing runs failed, evidence states 1 failed/6 stale queued; no identifiers or raw text emitted |
| 2026-07-29 | Retained Phase 3 recovery and migration 009 | PASS, DATABASE PHASE COMPLETE | Verified 13,161,166-byte post-008 backup; guarded recovery committed 34 completed/7 dead-letter/0 blockers plus 7 audit, 7 processing and 6 custody appends; 008 verify and 009 preflight passed; 009 committed and official 009+008 verifies passed; final 41/41 message IDs, zero leases, 3 constraints/1 history trigger; verified 13,169,864-byte post-009 backup |
| 2026-07-30 | Phase 3 non-owner runtime source preparation | SOURCE READY, NOT DEPLOYED | Worker owner-DDL removed; migration 010 closes three policy gaps and makes aggregate views security-invoker; fixed NOBYPASSRLS runtime grants, authenticated Compose overrides and resumable PowerShell activation/rollback runbook added; final Python 49/49, full forensic Go package, Compose/PowerShell/diff checks passed; no live mutation |
| 2026-07-30 | Local activation Gates 0–1 | PASS, NO LIVE MUTATION | Host: 3.8 GiB free RAM/1,774.09 GiB disk, Docker 29.6.2, Compose 5.3.1, Python 3.13.14, Go 1.26.5; final Python 49/49 in 1.164 s and Go forensic package in 18.178 s; PowerShell 5 stderr false-failure corrected with reusable runner; secret file remains absent and no service/database mutation occurred |
| 2026-07-30 | Local activation Gate 2 | PASS, LOCAL SECRET CREATED | Ignored runtime env created with cryptographic 64-hex DB/API secrets, tenant `default` and restricted ACL; operator manually regenerated both values once after the protected runner passed, then revalidated both shapes; Codex cannot read the file and Git confirms it is ignored; no value printed and no service/database mutation |
| 2026-07-30 | Local activation Gate 5 | PASS, CONFIG ONLY | Both authenticated runtime Compose merges passed through the quiet one-command verifier; configuration was not expanded, secret values were not printed, and no image/service/database state changed |
| 2026-07-30 | Local activation Gate 6 | PASS | Higher-memory focused LocalAI forensic forwarding package passed in 43.827 s with 6.29 GiB free RAM and generated protobuf ready; result persisted in the activation log |
| 2026-07-30 | Local activation Gate 7 attempts | SAFE STOPS, NO BUILD | First run stopped at 4.89 GiB before tagging/building. After reboot, four retries tagged the unchanged API/worker source images but stopped before LocalAI tagging or any build because implicit Compose discovery omitted healthy `nexusai-api`; inventory proved retained image `nexusai-localai:universal-20260724`, and the guard now pins project `nexusai` with a running-label fallback |
| 2026-07-30 | Phase 5A contracts/registry foundation | PASS, SOURCE ONLY | Python Phase 5A 3/3; complete worker discovery 56 passed/2 historical external-sample skips; focused Go 9/9; full forensic Go package passed in 8.143 s; `go vet`, JSON parse and Python compile passed; no retained/runtime/model/agent change |
| 2026-08-04 | Phase 7.1 populated subscriber safe source gate | PASS, SOURCE ONLY / DATA PENDING | 15-row hashed fixture reconciles 12 normalizable/1 duplicate/11 unique/3 rejected; six-operation oracle exact; Phase 2+7 Python 26/26, focused Ginkgo 23/23 and full forensic Go package, vet/compile/JSON/diff pass; no retained write/model call/deploy |
| 2026-08-04 | Phase 7.1 populated subscriber runtime acceptance | PASS, DATA ACCEPTED | Isolated hashed fixture retained as 15 input/11 accepted/1 duplicate/3 rejected; all six operations, privacy, citations, pagination, preview, CSV/audit exports, model fail-closed behavior, Agent Chat and responsive UI accepted; governed demo and legacy collections preserved |
| 2026-08-04 | Phase 7.2 Tower/Site bounded vertical slice | PASS, API/WORKER RUNTIME; LOCALAI REFRESH PENDING | Dedicated adapter/profile and six deterministic operations; registry 6 adapters/50 operations/65 templates; live preserved synthetic collection returns 5 tower references and explicit unmatched CDR joins; source route/UI await one guarded 6 GiB LocalAI refresh |

The focused test command is:

```powershell
& "C:\Program Files\Go\bin\go.exe" test `
  ./core/http/endpoints/localai `
  -run TestLocalAIInternalEndpoints `
  '--ginkgo.focus=forensic records KB upload forwarding' `
  -count=1
```

Do not use `go get github.com/mudler/LocalAI/pkg/grpc/proto`; this is an internal
generated package, not a missing external dependency. Re-run the approved
ephemeral code-generation recipe when `backend/backend.proto` changes or when a
clean local checkout lacks the ignored generated files. On PowerShell, use the
documented double-dash Ginkgo flag; the inherited single-dash form is parsed as
an unknown `-ginkgo` flag after compilation.

Approved PowerShell code-generation recipe:

```powershell
docker run --rm `
  --mount "type=bind,source=$((Get-Location).Path),target=/src" `
  -w /src `
  golang@sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651 `
  bash -c 'apt-get update && apt-get install -y --no-install-recommends unzip=6.0-28 && make protogen-go'
```

The container is removed after generation. The observed `unzip` package version
was 6.0-28; if the pinned package becomes unavailable, stop and update this
recipe deliberately instead of silently selecting a different toolchain.

## Authoritative CPU baseline

Existing postfix benchmark report:

- Chat: 6/6 successful, average 11,031.95 ms, p95 23,699.77 ms.
- Embeddings: 3/3 successful, average 568.49 ms, 1024 dimensions.
- Sidecar report contains 11 checks in total (nine query checks plus health and
  status), average 283.33 ms, zero recorded route mismatches.

The current runner is not an acceptance-grade forensic benchmark. It samples
only two chat prompts, three sidecar queries, and a single three-text embedding
batch. Before selecting or downloading another model, add a versioned gold set
covering:

- classification accuracy and abstention;
- structured extraction precision/recall;
- entity and relation resolution;
- retrieval recall@k, MRR/nDCG, reranking quality, and citation correctness;
- grounded-answer correctness and unsupported-claim rate;
- p50/p95 latency, peak RAM, model load time, and throughput;
- deterministic failure, timeout, and rollback behavior.

`Qwen3-Reranker-0.6B` is a provisional reranker candidate only. It is not yet
download-ready in this project because the local catalog/backend path, CPU
profile, quantization, size/RAM budget, acceptance threshold, and rollback plan
are incomplete. Do not download it until those items are recorded and approved.

## Required next phase

Phase: Phase 3/4 runtime activation gate, then Phase 5 documents/OCR source work.

Acceptance criteria:

1. Run the focused LocalAI forensic forwarding/reprocessing package on CI or a
   higher-memory runner; do not record the laptop's bounded linker timeout as a
   pass.
2. Retained migrations 008/009 and the invariant-guarded historical-failure
   disposition were applied and officially verified on 2026-07-29. Pre-008,
   post-008/pre-recovery and post-009 custom dumps passed round-trip hash, catalog
   and full-read checks. This database activation gate is complete.
3. Provision persistent NATS storage/retention, a non-owner database runtime role
   and grants, required sidecar authentication and matching queue configuration;
   these runtime changes require separate approval.
4. Deploy API, worker and LocalAI proxy as one rollback-protected activation and
   smoke synthetic success, transient retry, permanent DLQ, crash/redelivery,
   tenant isolation and idempotent reprocessing before admitting real evidence.
5. Treat Phase 4 structured-data v1 as source-complete at the versioned acceptance
   boundary. Keep new private-provider aliases/schema drift as additive compatibility
   packs that cannot weaken raw values, exact money/time semantics, visible rejects,
   cited source rows, privacy-safe audits or the 20/20 unchanged baseline.
6. Preserve the verified `records-demo-verified` registry closure and its scoped
   pre-closure dump; do not restore, replace or ingest real sensitive state
   without approval.
7. Keep OCR, STT, deep documents/video, sessions, archive children, database
   queries and unknown inputs truthful pending/manual routes until their later
   bounded vertical phases pass.

## Ordered upcoming phase roadmap

The sequence below is dependency-driven. A later modality may receive registration
and fixtures early, but it is not promoted to operational until its entire bounded
vertical slice passes.

### Phase 2A — Universal ingress deployment and registry closure

Scope:

- Keep the successfully rebuilt LocalAI universal-forwarding image live; diagnose
  the stalled second build and deploy MIME preservation without sacrificing the
  verified rollback point.
- Live-smoke one small representative structured, text, image, audio, video,
  transcript, capture, database, archive, TTS, and unknown upload.
- Verify no unsupported binary/media item reaches NATS or the records worker.
- Back up the forensic registry tables, then reconcile the two unique document
  hashes into two evidence objects while preserving all four KB source links.

Exit criteria:

- LocalAI `/readyz`, forensic `/healthz`, NATS, and worker metrics are healthy.
- Upload responses contain evidence ID/version/hash/route; duplicate source links
  produce `evidence.kb.source_linked` audit events.
- Counts reconcile to eight KB entries, six unique evidence objects, and eight KB
  asset/source links without changing the existing 9,250 structured rows.
- Rollback images and a database backup are recorded.

Approvals: the main LocalAI image build was approved and completed once. The
user's explicit 2026-07-27 request to complete every remaining Phase 2 subphase
authorized the documented registry-closure step. A scoped backup was created and
verified before the write.

Status: complete. Universal ingress, worker isolation, health and rollback passed.
`records-demo-verified` now has eight KB entries, six evidence objects and eight
source links while its 9,250 structured rows remain unchanged. The custom-format
pre-closure dump is retained under `.phase2-backups/` with SHA-256
`8194527272738f4feeb0440217dc989d043d9506708dfd04138218b5cb2795ff`.

### Phase 2B — Golden fixtures and unchanged-baseline evaluation

Scope:

- Keep `configuration/forensic_modality_evaluation_matrix.json` authoritative.
- Build the twelve planned fixture packs: schema drift/spreadsheets, mixed native
  and scanned documents, images/OCR, noisy multilingual audio, TTS, timestamped
  transcripts, short video, PCAP/EVTX, SQLite, hostile archives, and unknown/mixed.
- Include clean, messy, corrupt, duplicate, ambiguous, unsupported, and adversarial
  cases; store expected outputs and provenance locators beside each fixture.
- Extend the benchmark runner to emit classification, extraction, exact-answer,
  retrieval, citation, unsupported-claim, abstention, latency, RAM, CPU, load-time,
  throughput, and disk metrics.

Exit criteria:

- All 20 profiles have real versioned fixtures and machine-verifiable goldens.
- Current Qwen chat/embedding plus deterministic adapters have a reproducible
  unchanged-baseline report; no new model is installed during this phase.
- Every failed threshold blocks promotion and reports the exact failed fixture.

Status: complete. All 20 profiles have versioned ready fixture contracts. The
unchanged three-round CPU baseline passes 18/18 chat guardrail checks, 27/27
fail-closed deterministic routes, and three 8-vector, 1,024-dimensional embedding
runs. No model was downloaded or promoted. Detailed result:
`reports/forensic-phase2-completion-acceptance-20260727.md`.

### Phase 3 — Evidence control plane, versions, and chain of custody

Scope:

- Design an additive migration for normalized evidence versions, source links,
  processing runs/events, typed derived artifacts, and adapter/model revisions.
- Preserve and later deploy the verified scoped content-addressed raw-storage
  contract without weakening its receipt/hash/concurrency controls.
- Bind tenant/case scope to authenticated identity, review RLS enforcement, and
  add permission checks for evidence, artifacts, reports, and admin operations.
- Add idempotent retry, JetStream acknowledgement, dead-letter, and reprocessing
  rules without destroying earlier outputs.

Exit criteria:

- Backup, forward migration, compatibility test, and rollback procedure pass.
- Concurrent upload/retry/reprocessing tests prove no silent loss or overwrite.
- Every material transition is append-only auditable and queryable by version.

Status: source development and isolated acceptance are complete. The read-only
preflight, additive migration/backfill, append-only protection, hash-chain,
authenticated scope, non-owner RLS, scoped content addressing, receipts,
corruption quarantine, read-only worker verification, atomic queue outbox,
file-backed JetStream, lease/concurrency control, bounded retry/DLQ,
commit-before-ack and immutable idempotent reprocessing contracts pass. A full
disposable 001-009 database chain and real NATS deduplication/restart/AckSync
acceptance passed. The retained registry remains unmigrated and the source is not
deployed. The focused LocalAI wrapper package exceeded the bounded linker time on
this constrained laptop without a compiler diagnostic; CI/higher-memory passage
is required before the separately approved activation gate.

### Phase 4 — Structured-data depth and real financial/log adapters

Scope:

- Harden CDR, IPDR, ANPR, subscriber, tower/location, transaction, access-log,
  and generic adapters for CSV, JSON/JSONL, and Parquet.
- Add TSV and XLSX first, then ODS/Arrow/Feather/Avro/ORC/XML based on fixture value.
- Preserve unknown columns, exact decimal/currency semantics, raw timestamps,
  timezone/late-arrival behavior, duplicate-row decisions, and schema drift.
- Expand deterministic query templates, cross-family correlations, UI profiles,
  rejection views, and record-level provenance.

Exit criteria:

- 100% row accounting; exact-answer agreement 1.0 on gold queries; required schema
  mapping thresholds pass; all rejects are visible with source rows and reasons.
- Financial calculations never use floating-point coercion or combine currencies
  silently; CDR/IPDR/ANPR joins retain source and time uncertainty.

Status: source-complete for structured-data v1 across Phase 4A capability-aware
querying/TSV, Phase 4B bounded read-only XLSX, Phase 4C exact cross-family
correlation, Phase 4D per-sheet typed XLSX/UI simplification, and Phase 4E
privacy-safe real-format CSV auditing plus typed IPDR depth. XLSX keeps
sheet/row/cell lineage, exact raw and cached values, formula text, hidden state,
dates/errors/merges, and blocks active content. Exact correlation spans CDR,
IPDR, ANPR, subscriber, tower/location, transaction, access-log and generic
records with bounded normalization, family coverage and stable source-row
citations. Mixed workbooks independently map uniquely recognized sheets and
preserve ambiguous/unrelated sheets generically. The seven typed families have a
versioned privacy-safe audit matrix; generic/TSV/XLSX keep their existing goldens.
Private-provider long-tail packs, ODS and deeper columnar adapters remain
additive future compatibility work, not blockers to the defined v1 boundary.

### Phase 5 — Documents, native extraction, OCR, and citation geometry

Scope:

- Implement deterministic native extraction and metadata before model OCR.
- Add PDF, DOCX/ODT/RTF, PPTX/ODP, HTML/EPUB, EML/MSG, tables, attachments,
  reading order, page/slide coordinates, encryption, and corruption handling.
- Benchmark one CPU-compatible OCR path only after fixture baselines, resource
  estimates, license review, download approval, and rollback plan.
- Index versioned extracted text/layout while preserving page/region citations.

Exit criteria:

- OCR CER/WER, table-cell accuracy, retrieval Recall@5, citation correctness,
  unsupported-claim rate, warm/cold latency, and peak RAM meet the matrix.
- Partial OCR failure never invalidates the registered source or successful pages.

### Phase 6 — Media metadata and derived-artifact foundation

Scope:

- Add typed artifact/media tables and APIs for codec, dimensions, duration, EXIF,
  streams, checksums, coordinates, time ranges, confidence, parent/version links,
  processing configuration, adapter/model revision, and warnings.
- Implement deterministic image/audio/video metadata adapters before large models.
- Add artifact lineage and status UI, query locators, audit events, and retention.

Exit criteria:

- Metadata accuracy is 1.0 on goldens and every artifact resolves to immutable
  evidence/version plus spatial or temporal coordinates.
- Corrupt/unsupported media remains registered and visibly partial/pending.

### Phase 7 — Audio, STT, diarization, transcript search, and TTS

Scope:

- Benchmark one small CPU-compatible ASR candidate against clean/noisy Urdu-English
  fixtures before considering larger Whisper/Qwen ASR variants.
- Store word/cue timestamps, language, confidence, gaps, and optional speaker turns;
  index transcript chunks with audio/video time citations.
- Treat speaker identity only as a candidate relationship unless corroborated.
- Treat TTS as a derived accessibility/report artifact with source-text hash,
  parent evidence/report, voice/backend revision, consent policy, and round-trip
  intelligibility checks; TTS output is never a new evidence fact.

Exit criteria:

- Speech WER, timestamp p95 error, diarization error (when enabled), retrieval,
  citation, latency, RAM, and abstention thresholds pass.
- STT/TTS failures preserve original evidence/report and prior artifact versions.

### Phase 8 — Images, OCR-on-image, ANPR, objects, and policy-gated faces

Scope:

- Add OCR regions, object/plate boxes, crops, coordinates, confidence, detector
  version, and source-image hash.
- Benchmark specialist deterministic detections before a general VLM explanation.
- Add rotated/blurred/night/rain/screenshot/Urdu-English fixtures and false-positive
  analysis; face capability remains disabled until legal/policy controls exist.

Exit criteria:

- Precision/recall, OCR CER, box provenance, abstention, and CPU budgets pass.
- Candidate detections never become exact identities without threshold, policy,
  provenance, and corroborating evidence.

### Phase 9 — Video timelines and multimodal correlation

Scope:

- Implement metadata, bounded frame/scene sampling, artifact time mapping, audio
  extraction, STT linkage, object persistence, and cross-frame event timelines.
- Route analyst queries to stored detections/transcripts first and use a VLM only
  for bounded, cited interpretation of selected frames/clips.

Exit criteria:

- Frame/event recall, timestamp alignment, object persistence, transcript linkage,
  query latency, unsupported-claim rate, and resource limits pass on short and
  long-video fixtures.

### Phase 10 — Captures, databases, archives, and hostile inputs

Scope:

- PCAP/PCAPNG session extraction and separate EVTX event normalization.
- Read-only SQLite inventory/query with disabled extensions and strict limits.
- Inventory-only ZIP/TAR first, followed by other archives; child files become
  separately registered evidence with parent/member lineage.
- Defend against traversal, links, decompression bombs, nesting, encrypted members,
  polyglots, corrupt signatures, macros, and executable content.

Exit criteria:

- Safe inventory and row accounting are 1.0; malicious fixtures cannot escape
  limits or execute; every unsupported member is preserved or explicitly rejected.

### Phase 11 — Entity/relation graph and capability-aware query planner

Scope:

- Normalize aliases for people, phones, IMEI/IMSI, IP/domain, accounts, vehicles,
  plates, locations, files, organizations, speakers, and candidate detections.
- Store relationship evidence, time validity, resolution confidence, conflicts,
  and all supporting locators instead of asserting model-only identity.
- Evolve the planner to inspect available modalities/adapters, schema, entities,
  ambiguity, permissions, cost, and resource state before selecting SQL, KB,
  entity, media, or hybrid tools.

Exit criteria:

- Entity/link goldens, contradiction handling, tenant isolation, clarification,
  execution trace, and citation completeness pass with zero cross-case leakage.

### Phase 12 — Analyst UI, observability, reports, and production hardening

Scope:

- Unified case evidence inventory, processing/version timeline, schema/media views,
  artifact lineage, confidence/limitations, retries, DLQ, and explicit partial state.
- Professional reports that separate deterministic facts, semantic context,
  inferred relationships, and model interpretation with stable citations.
- Metrics/traces/logs for queue lag, adapter errors, row rejection, OCR/STT quality,
  model resource usage, audit gaps, and SLOs; security and load testing.

Exit criteria:

- Accessibility, permission, recovery, backup/restore, concurrency, performance,
  report reproducibility, and analyst acceptance tests pass.
- Release documentation lists operational versus pending capabilities accurately.

### Phase 13 — Candidate promotion and continuous regression

Scope:

- Recommend only one candidate model/backend change at a time from the catalog.
- Record license/source, exact version/quantization, download/disk/RAM/CPU estimates,
  benchmark command, acceptance deltas, activation configuration, and rollback.
- Run the same fixed gold set against baseline and candidate; retain the baseline.

Exit criteria:

- Candidate improves its declared task without violating accuracy, unsupported-
  claim, latency, memory, provenance, security, or stability gates.
- Explicit approval precedes download/install and separate approval precedes
  production activation.

## Decision log

| Date | Decision | Reason | Status |
| --- | --- | --- | --- |
| 2026-08-10 | Publish reprocess eligibility without exposing execution in Evidence Operations | An analyst can review exact terminal-job conditions and approval requirements without mutating retained evidence or accidentally enqueueing work | R5-EVID-04 source accepted |
| 2026-08-10 | Accept R5-EVID-03 after installing its pinned local browser runtime | The focused browser suite verifies the changed evidence surface 13/13; two unrelated legacy full-suite checks remain separately visible | R5-EVID-03 source accepted |
| 2026-08-07 | Publish the retained control-plane custody/artifact model through the existing evidence detail API | Reusing append-only schema truth avoids parallel provenance, unstable citations and invented timelines | R5-EVID-02 source accepted |
| 2026-08-07 | Open R5 only after corrected R4 runtime acceptance, then begin with read-only evidence truth | Preserves phase governance and delivers investigator value without inventing custody history or mutating retained evidence | R4 complete; R5-EVID-01 source accepted |
| 2026-07-23 | Preserve all existing edits and reports | They are user work from the prior laptop/session | Active |
| 2026-07-23 | Isolate continuation work on a `codex/` branch | Establish a recoverable checkpoint without altering `main` | Active |
| 2026-07-23 | Do not stage, commit, or push yet | User explicitly reported no Git publication actions and did not authorize them | Active |
| 2026-07-23 | Treat current Qwen chat/embedding pair as baseline | It is already installed, configured, and measured on target hardware | Active |
| 2026-07-23 | Defer reranker/model download | Acceptance criteria and reproducible CPU deployment path are incomplete | Active |
| 2026-07-23 | Use pinned temporary Docker codegen | Avoids an untracked host-wide tool installation; approved and verified with an immutable image digest | Complete |
| 2026-07-23 | Preserve the old API image before targeted rebuild | Provides a local recovery point without altering database state | Complete |
| 2026-07-24 | Register every evidence format but queue only supported parsers | Preserves all evidence while preventing binary/media/unsupported tabular files from being misparsed as CSV | Source complete |
| 2026-07-24 | Keep version IDs in metadata during Phase 2 | Delivers schema-neutral lineage without applying an unapproved database migration | Interim; normalize in Phase 3 |
| 2026-07-24 | Do not backfill the four KB-only documents until explicitly approved and backed up | Reconciliation writes forensic registry and asset rows and required a reviewed execution step | Complete 2026-07-27; scoped backup verified before four-link closure |
| 2026-07-24 | Deduplicate content, not source lineage | Byte-identical uploads share one evidence identity while every distinct KB source entry remains linked and audited | Deployed in forensic API |
| 2026-07-24 | Never equate raw registration with adapter completion | Pending image/audio/video/document/capture/database/archive routes must remain visibly registered or pending | Deployed in forensic API |
| 2026-07-24 | Make the 20-family modality matrix executable | Support claims now require fixtures, classifier contracts, metrics, failure behavior, provenance, and rollout gates | Active |
| 2026-07-24 | Stop the unproductive second LocalAI rebuild | It exceeded the successful build duration by a wide margin, emitted no useful progress, and produced no image; service continuity took priority | Complete; first rebuilt image restored |
| 2026-07-24 | Keep MIME preservation source-only until a bounded rebuild succeeds | Focused tests pass, while the deployed API still safely identifies octet-stream uploads through signature/content sniffing | Active |
| 2026-07-24 | Keep the live retry isolated | The `forensic-ingress-audit-20260724` collection validates new routing without changing the 9,250-row `records-demo-verified` baseline | Active |
| 2026-07-24 | Never parse arbitrary binary payloads as CSV headers | Binary header bytes can contain PostgreSQL-unsupported NUL escapes; header detection is now allowlisted to worker-readable formats with UTF-8/NUL/width bounds | Deployed |
| 2026-07-24 | Use a clean matrix collection for acceptance | The first smoke collection preserves failures and retry artifacts; `forensic-phase2a-acceptance-20260724` provides one reconcilable evidence path per declared family | Complete |
| 2026-07-24 | Cancel only the orphaned BuildKit record and retain cache | Build cache is 20.21 GB with 11.23 GB reclaimable, but disk is not constrained; broad pruning would discard useful cache and was not required | Complete |
| 2026-07-27 | Inventory SQLite through raw bounded pages, not a database engine | Preserves CGO-free deployment and prevents SQL/schema execution, extension loading, sidecar following, and evidence-value reads | Deployed |
| 2026-07-27 | Warn on every WAL-mode main-file-only inventory | A separate `-wal` may contain uncheckpointed evidence that cannot be inferred from the uploaded main file | Deployed |
| 2026-07-27 | Separate capture structure from packet content | Technical inventory can be made safe and exact without exposing payloads, endpoints, filters, name-resolution values, or secrets | Deployed |
| 2026-07-27 | Keep PCAP/PCAPNG at foundation support | Packet/session/protocol/EVTX derivatives lack real versioned goldens and packet-level lineage, so registration inventory must not be described as full capture analysis | Active |
| 2026-07-27 | Inventory MPEG media without decoding payloads | Exact MP3 frame and ISO box/track facts do not require reading ID3 values, compressed samples, `mdat`, frames, or embedded audio | Deployed |
| 2026-07-27 | Keep MP3/MP4/MOV at foundation support | Real-codec goldens, fragmented/edited containers, frame/audio derivatives, STT, objects and timelines remain pending | Active |
| 2026-07-27 | Preserve raw ANPR/transaction rows and store derived fields separately | Search keys, UTC times, confidence, exact money and IBAN results must never replace evidential source tokens | Deployed |
| 2026-07-27 | Reject ambiguous financial parsing instead of guessing | Invalid grouping, excess minor-unit precision and multiple populated amount columns would otherwise create false exact financial facts | Deployed |
| 2026-07-27 | Treat plate formats and invalid PK IBANs as review signals | Pakistan formats vary by province, series and period; a source identifier must remain visible without being silently corrected or promoted | Deployed |
| 2026-07-27 | Close Phase 2 only at the documented 20-family boundary | Fixture readiness proves exact classification/inventory/abstention contracts but does not falsely promote pending OCR/STT/deep adapters | Complete |
| 2026-07-27 | Score deterministic benchmark routes fail-closed | HTTP failures must reduce route agreement instead of disappearing from the denominator | Deployed and verified 27/27 |
| 2026-07-27 | Route tower activity through the canonical CDR+tower store | The advertised tower template must return tower-sector evidence, not silently query only legacy CDR rows | Deployed and live-verified |
| 2026-07-27 | Make migration 008 preflighted and transactional | Compatibility failures must be visible before DDL and any forward/rollback error must leave no partial schema | Retained migration applied and officially verified 2026-07-29 from verified backup; API/worker remained stopped |
| 2026-07-27 | Distinguish native Phase 3 registration from legacy backfill | Reapplying migration 008 must not invent a second legacy source link for evidence registered after Phase 3 | Implemented and zero-drift verified |
| 2026-07-28 | Separate forensic actor from subject user | Delegated agent work must remain attributable without granting the agent a different user's scope | Implemented in source; deployment gated |
| 2026-07-28 | Trust scope only after LocalAI collection ownership and sidecar bearer validation | Caller-controlled tenant/user/collection/case fields are an authorization-confusion risk | Implemented and negative-test clean; deployment gated |
| 2026-07-28 | Keep explicit tenant/collection SQL filters in addition to RLS | RLS is defense in depth and table owners can bypass it | Implemented; non-owner isolated acceptance passed |
| 2026-07-28 | Preserve auth-disabled compatibility only for development | Existing tests and undeployed stacks need a transition path, while production must fail closed | Empty key warns; required mode refuses startup without a key |
| 2026-07-29 | Address raw bytes by scope hash plus content SHA-256 | Caller tenant/collection/filename values must not shape paths, while identical bytes in different scopes must remain isolated | Implemented in source; deployment gated |
| 2026-07-29 | Publish objects and receipts create-only and quarantine conflicts | Retries/concurrent uploads may reuse only after full verification; corrupt hash addresses must never be silently overwritten | Implemented; concurrency/corruption tests pass |
| 2026-07-29 | Mount retained bytes read-only in the worker and verify independently | A queued path/URI/receipt is untrusted until containment, scope, size and full SHA-256 agree | Implemented in Compose/source; undeployed |
| 2026-07-29 | Do not describe local read-only files as regulatory WORM | Storage administrators/API ownership can still alter local filesystem state | Production object lock, legal hold and retention policy remain approval gates |
| 2026-07-29 | Make PostgreSQL the queue idempotency authority | JetStream is at-least-once and a worker may commit before its acknowledgement is observed | Implemented; completed redelivery is ack-only |
| 2026-07-29 | Commit evidence registration and queue outbox atomically | A separate insert leaves a crash window where evidence exists with no recoverable job | Implemented in one transaction |
| 2026-07-29 | Publish terminal diagnostics before terminating the source message | A DLQ outage must not silently discard the only recoverable delivery | Implemented; failed DLQ publish leaves the source unacknowledged |
| 2026-07-29 | Reprocess by immutable linked job, never terminal-state reset | Investigative history and earlier outputs must remain attributable and reproducible | Implemented with evidence lock, idempotency key and generation lineage |
| 2026-07-29 | Treat the LocalAI linker timeout as an open gate | No diagnostic is not evidence of a passing package, even when smaller Phase 3 suites are green | CI/higher-memory check required before activation |
| 2026-07-29 | Parse XLSX as inert OOXML evidence | Formula text, cached values, hidden state and external references are evidence; executing or following them would cross the trust boundary | Implemented in source; macros/DTD/unsafe packages fail closed |
| 2026-07-29 | Classify XLSX per sheet and preserve ambiguity generically | Workbook-wide union headers can misclassify unrelated schemas, while defaulting the whole workbook to generic loses safe typed value | Each non-empty sheet promotes only on one typed match; zero/multiple matches and explicit non-matches remain generic with review/provenance |
| 2026-07-29 | Add a separate exact cross-family correlation route | Legacy relationship-network SQL uses exploratory substring matching and aggregate filenames, which cannot satisfy exact forensic citation requirements | `cross_family_correlation` uses canonical exact keys, bounded normalization, source-row citations and explicit truncation; legacy behavior remains backward compatible |
| 2026-07-29 | Count canonical matched records, not nested display rows | A response contains match, coverage and relation arrays whose combined display length would overstate the number of facts matched | Answer count is sourced from exact family coverage; display/scan caps are visible limitations |
| 2026-07-29 | Make the Records landing page analyst-first | Query, evidence admin, batch operations, diagnostics and developer controls at one visual level made the workflow difficult to understand | Default to collection/question/analyze and progressively disclose advanced, evidence and management panels; deeper workspace split remains Phase 4E+ |
| 2026-07-29 | Keep CSV a first-class structured source | XLSX depth is additive; most supplied CDR/provider exports remain delimited files and must not be treated as a legacy path | Supplied 3,931-row CSV auto-routes to CDR with complete read-only production normalization accounting |
| 2026-07-29 | Add a privacy-safe offline provider audit | Provider onboarding and demonstrations need production adapter proof before authorization to ingest sensitive data | Audit returns hash/schema/aggregate quality only; no raw rows, identifiers, rejected values, DB, queue, KB, model or DNS activity |
| 2026-07-29 | Close Phase 4 structured-data v1 at versioned acceptance, not universal vendor exhaustiveness | No finite project can claim every private export schema; correctness requires a stable boundary and additive compatibility packs | Seven typed families plus generic/TSV/XLSX and exact query/correlation contracts pass; new provider packs may not weaken existing gates |
| 2026-07-29 | Split retained activation at the migration 008/009 dependency boundary | The 009 read-only gate proves 008 must exist first, while seven failed legacy jobs require an explicit audit-preserving recovery decision | Complete: 008, guarded recovery and 009 applied/verified sequentially from independent recovery points |
| 2026-07-29 | Preserve legacy failures as dead-letter history before migration 009 | Seven historical jobs produced no outputs, have no successful sibling jobs, and cannot truthfully become completed; deletion/reset would erase or reuse evidence history | Executed and verified: 7 dead-letter jobs/runs, 7 audit + 7 processing + 6 custody appends, exact evidence pointers, no history/count drift |
| 2026-07-30 | Keep schema-owner DDL out of worker job transactions | A non-owner runtime must verify migrated schema and fail closed; creating tables, indexes, hypertables or policy during evidence processing defeats least privilege | Worker now validates canonical records/policy at startup and performs no owner DDL; deployment pending |
| 2026-07-30 | Treat aggregate views as caller-security objects | Owner-context views can bypass base-table RLS even when the application role is non-owner | Migration 010 sets both forensic aggregate views `security_invoker=true`; retained application pending |
| 2026-07-30 | Run long activation commands in local PowerShell | Clean Go/LocalAI compilation and Docker builds can exceed agent frontend timeouts and need operator-visible logs | Resumable secret/backup/migration/test/build/deploy/smoke/rollback runbook added; execution pending |
| 2026-07-30 | Use one embedded cross-runtime Phase 5A catalog | Adapter, operation, specialist, model-role and contract discovery must not drift or depend on mutable retained state | CDR/generic compatibility bindings and source-only authenticated discovery implemented; other specialists remain contract-only/pending |
| 2026-08-04 | Reject and redact prohibited subscriber targets before response construction | A CNIC/name target must not reach SQL, planner/audit export, or an optional model merely because it fails to match allowed columns | Source-tested; deploy before any populated upload |
| 2026-08-04 | Keep raw subscriber evidence protected while redacting generic/canonical projections | Chain of custody requires source preservation, but default API/entity/export fields must never disclose full CNIC or subscriber names | Source-tested; worker indexes safe secondary identity and canonical response removes protected fields |
| 2026-08-04 | Prepare populated acceptance without inferring upload authority | The request permits safe source/read-only work but retains a separate explicit authorization gate for evidence writes | Fixture/oracle/benchmark complete; retained data remains unchanged |

## Phase status ledger

### 2026-07-23 — Laptop continuation reconciliation

- Re-read repository contribution, build/test, coding-style, endpoint/auth,
  assistant-MCP, backend-debugging, and backend-addition instructions.
- Verified Git base, dirty worktree, service health, models, agent settings,
  evidence counts, benchmark reports, and the critical live hybrid path.
- Re-ran focused Python and Go forensic tests successfully.
- Identified protobuf generation as the blocker for the focused core endpoint test.
- Created the isolated branch and enabled the repository's local hooks path.
- Created this living checkpoint and linked it from `AGENTS.md`.
- Generated the ignored Go protobuf bindings with the approved pinned temporary
  Docker toolchain; no host-wide install was made.
- Corrected the Windows Ginkgo focus syntax and passed the focused LocalAI
  endpoint test in 6.466 seconds of reported package test time.
- Mapped the current physical evidence-to-citation flow and its lineage gaps.
- Added a bounded, tenant/collection-scoped KB evidence sample to LLM synthesis,
  with untrusted-text prompt guidance and available evidence citation locators.
- Added and passed a focused Ginkgo regression; the running container was not
  rebuilt until explicit approval was received.
- Preserved the prior API image under a rollback tag, rebuilt/restarted only the
  API service, and verified API, NATS, PostgreSQL, and worker continuity.
- Confirmed safe deterministic fallback on the cold 60-second timeout and a
  successful grounded warm synthesis citing an actual retrieved result locator.
- Confirmed that direct KB result metadata lacks evidence/version/chunk IDs,
  making first-class registration of KB-only documents the next lineage task.

Status at that checkpoint: continuation documentation, physical flow inventory,
protobuf tooling, bounded synthesis, universal forensic registration,
cross-modality tests, rollback images, and the 20-family evaluation contract were
ready. The main LocalAI build and live smoke were still pending then; see the
later 2026-07-24 deployment/recovery entry for the current state.

### 2026-07-24 — Universal evidence registration foundation

- Reconciled the KB upload, raw-entry storage, forensic webhook, evidence table,
  KB asset table, worker parser, and hybrid retrieval paths.
- Replaced extension-limited forwarding with universal evidence registration,
  preserving the existing `skip_forensic_records` loop-prevention marker.
- Replaced in-memory forwarding buffers with streaming multipart upload and a
  configurable `FORENSIC_RECORDS_UPLOAD_TIMEOUT` (default `5m`).
- Added a data-driven catalog for structured data, documents, images, audio,
  video, STT/TTS artifacts, captures, databases, archives, and unknown formats.
- Added hard processing gates so only formats understood by the current worker
  can reach NATS; pending modalities remain registered and auditable.
- Added schema-neutral version IDs, SHA-256 idempotency checks, a transaction-
  scoped advisory lock, KB asset linkage, and `evidence.kb.registered` audit events.
- Added hybrid KB result enrichment with evidence/version/retrieval/chunk hashes.
- Added and passed the cross-modality Ginkgo matrix and focused LocalAI upload
  forwarding suite.
- Corrected duplicate handling so one content hash can preserve multiple KB
  source-entry links, and kept adapter-pending evidence visibly `registered`.
- Added a machine-readable 20-family evaluation contract: eight profiles have
  real fixtures now and twelve are explicit planned fixture packs with measurable
  acceptance, abstention, provenance, resource, and rollback gates.
- Built and deployed only the forensic API as image
  `sha256:a4281a8c427f07fa458a978a00d1e07ffaa45ce91523e7fc940c3edfd3e043ba`;
  retained `rollback-20260724-universal-registration` and
  `rollback-20260724-before-source-link-fix` recovery tags.
- Completed the read-only `records-demo-verified` reconciliation: four missing KB
  entries reduce to two unique content hashes, so the future backfill must create
  two evidence records while retaining four source-entry links.
- Did not rebuild images, mutate existing evidence rows, migrate the database,
  stage files, commit, push, or download any new model.

### 2026-07-24 — Universal ingress deployment, defect closure, and recovery

- Received explicit approval for the main LocalAI rebuild. Captured 3.77 GiB
  free RAM and 1,783.2 GB free disk, stopped LocalAI during compilation to reduce
  memory pressure, and preserved the original image/container for rollback.
- Built `nexusai-localai:universal-20260724` successfully in 671.7 seconds and
  deployed digest
  `sha256:0731ab05fa080fae9bc90ddbcc26f74fcb554389ee82aa0282e5373a27144c9b`
  with the original port, volumes, environment, restart policy, models and data.
- Created the isolated `forensic-ingress-audit-20260724` collection. The first live
  pass exposed three integration defects without touching the verified demo:
  PostgreSQL rejected a NUL byte in the advisory-lock text key; the worker still
  required legacy CDR headers; and the deployed forwarding layer declared file
  parts as `application/octet-stream`. Pretty JSON header extraction was also
  hardened after this live boundary test.
- Replaced the lock key with an unambiguous JSON tuple, added pretty top-level JSON
  field extraction, aligned CDR validation/normalization with modern aliases, and
  preserved incoming multipart content type in current LocalAI source. Added
  focused regressions for each behavior.
- Rebuilt and deployed only the corrected forensic API and worker. A fresh modern
  CDR then completed through the real LocalAI → forensic API → NATS → worker path
  with four accepted rows, zero rejected rows and zero duplicates. Text and JSON
  evidence also completed; the earlier failed pre-fix CDR remains auditable.
- Started a second full LocalAI build solely to deploy MIME preservation. It made
  no useful progress for roughly 45 minutes, produced no `r2` image, and was
  terminated. The successful first rebuilt container was restarted immediately;
  Docker health is `healthy`, `/v1/models` returns both baseline models, and the
  forensic `/healthz` response is `{"status":"ok"}`. Server-side sniffing keeps
  current deployments functionally safe until the MIME-only source delta is
  deployed through a diagnosed bounded build.
- No database backfill, schema migration, model download, staging, commit, push,
  pull request, or GitHub publication was performed.

Current status: universal registration and forwarding are live; modern CDR, text,
and pretty-JSON smoke paths are verified; rollback images/containers are intact.
Multipart MIME preservation is source-complete and test-passing but not yet in the
running LocalAI image. The next safe work is the remaining small-format Phase 2A
smoke matrix plus diagnosis of the second build. Database reconciliation remains
blocked on its separate explicit write approval.

### 2026-07-24 — Twenty-family Phase 2A acceptance closure

- Generated and format-validated a bounded local ingress pack without downloads:
  WAV audio/TTS, WebVTT transcript, one-frame AVI, DNS PCAP, read-only-test SQLite,
  deterministic XLSX/ZIP, minimal PDF, TSV schema drift, and unknown binary. Used
  existing repository fixtures for CDR, IPDR, ANPR, subscriber, tower, financial
  transactions, access logs, Markdown, DOCX and PNG.
- The first broad live pass exposed a fourth boundary defect: binary bytes from
  XLSX/DOCX/WAV/AVI/SQLite/ZIP/unknown files could be accepted as a CSV header,
  then PostgreSQL JSONB rejected the embedded NUL escape with SQLSTATE `22P05`.
- Restricted header inspection to worker-readable structured formats and added
  maximum-column/name, UTF-8 and NUL validation. Binary sources remain immutable
  and are classified from extension plus bounded content sniffing. Added Ginkgo
  regressions for binary workbooks and malicious CSV headers.
- Aligned both API classification and the Python access-log adapter with the real
  `client_ip` fixture alias. A fresh live access-log ingest completed as
  `access_log` with 2/2 rows accepted.
- Deployed API image
  `sha256:03a55612567e25fb04d9a8c1cec67bd2d35a7e4de30f6c67fd081e1582471bbe`
  and worker image
  `sha256:d2ec7cf626d21b6be399017e87b660f5a679ef22962583afd91e4ff43c8f8d18`;
  retained the previous images under explicit rollback tags.
- Created clean collection `forensic-phase2a-acceptance-20260724` and uploaded one case
  for every one of the 20 matrix families, plus a second Office-document case.
  Final reconciliation: 21 KB entries = 21 evidence objects = 21 KB assets;
  7/7 deterministic jobs completed; 20/20 rows accepted; zero rejected,
  duplicates, failed jobs, failed evidence or warnings. All 13 adapter-pending or
  manual-review items had no records batch, proving they did not reach the worker.
- Verified derived lineage in the evidence detail API: TTS metadata points to its
  PDF parent and STT transcript metadata points to its audio parent, with distinct
  evidence/version IDs, hashes and KB source entries.
- Diagnosed the second LocalAI build after client termination: BuildKit retained
  ref `khk58bwg259mz3ck70wsdjidc` as `Running` with zero discovered/completed
  steps since 05:54. Removed only that exact orphan record and verified zero
  running build records. Build cache remains intact; no prune or new full build.
- Production collection `records-demo-verified` remains unchanged at 9,250 rows.
  No reconciliation backfill, schema migration, model download, Git staging,
  commit, push, pull request, or publication occurred.

Current status: Phase 2A universal ingress registration/routing is live and cleanly
validated across all 20 declared evidence families. This is not a claim that the
13 pending modality adapters perform extraction: spreadsheets, documents/OCR,
audio/STT, video, captures, databases, archives and unknowns remain explicit
pending/manual routes. Next implementation priority is Phase 2B golden fixture
packs and the first deterministic media-metadata vertical slice. Multipart MIME
preservation remains source-tested but not deployed; server-side content sniffing
is verified as the active fallback. Database backfill still requires separate
explicit approval.

### 2026-07-24 — Pakistan real-format readiness and neutral naming

- Profiled the user-supplied CDR locally and read-only. The 828,259-byte source
  has SHA-256
  `3eee8cb613d2dee9a5ee10600ec361ad2537c0b95d5601fe8d5c8d135a39c8f5`,
  16 headers, 3,931 data rows, no malformed-width rows, and 297 exact duplicate
  rows. No real values were uploaded, copied into the repository, disclosed in
  output, or sent to an online service.
- Observed provider-format cases now represented by synthetic goldens: 12-digit
  Pakistan phone form, short/service identifiers, USSD, blank originators,
  15-digit IMSI with MCC 410, 14-digit IMEI, spreadsheet `.0` location IDs,
  missing tower coordinates, negative cell sentinels, CALL/SMS/GPRS/VOLTE, and
  INCOMING/OUTGOING/DATA semantics. All 3,885 supplied coordinate pairs passed a
  coarse Pakistan screening bound; this is not treated as proof of location.
- Added versioned country profile
  `configuration/forensic_country_profiles/pakistan.json` with dated official
  ITU, NADRA, SBP, provincial excise, Pakistan Code, PTA/NTCERT, and MoITT source
  references. The MoITT personal-data material is explicitly labeled draft, not
  enacted law. The profile is extensible and does not claim to enumerate every
  historic or proprietary source format.
- Added `jurisdiction` and `source_timezone` upload metadata. Pakistan deployment
  defaults are `PK` and `Asia/Karachi`; explicit timestamp offsets take priority,
  and naïve timestamps are interpreted in the declared zone before canonical UTC
  storage. Added pinned `tzdata==2026.3` for consistent IANA timezone behavior.
- Added a wholly synthetic UTF-8 Pakistan CDR fixture and adapter regressions for
  Urdu, quoted commas, 14-digit IMEI, service labels, USSD, blank tower fields,
  negative cell sentinels and exact duplicates. Final Python worker/adapter suite:
  17 tests passed in 0.128 seconds. The Go forensic API package passed (15.793
  seconds reported after cache warm-up).
- Renamed the isolated live collections by supported copy/verify/reconcile/retire
  migration: `forensic-phase2a-acceptance-20260724` now has 21 entries and
  `forensic-ingress-audit-20260724` has 39. Each copied file passed byte SHA-256
  comparison. All writable forensic tables and KB entry references were updated
  transactionally; old database references are zero; the old collection stores
  were retired only after verification. LocalAI and forensic API stayed healthy.
- Added detailed team-lead handoff and phased roadmap in
  `reports/forensic-pakistan-data-readiness-20260724.md` covering structured
  records, ANPR, finance, logs, images/documents/video, audio/STT/TTS,
  databases/archives/captures, entity resolution, retrieval/UI, scale and release.
- The focused LocalAI forwarding package did not complete compilation within two
  bounded five-minute Windows attempts and emitted no compiler or test failure.
  Generated protobuf files are present, so this is not the earlier missing-proto
  setup failure. Re-run this focused Ginkgo check in CI or a less constrained
  build environment before merge.
- The real CDR has not been ingested. It remains a separate sensitive-data write
  decision requiring explicit authorization plus case, retention and access
  metadata. No backfill, schema migration, model download, Git stage, commit,
  push, pull request, or publication occurred. `records-demo-verified` remains
  unchanged at 9,250 rows.

Current status: Pakistan localization and real-format CDR regression foundations
are source-complete. The next implementation slice is Phase 2B.1 deterministic,
model-free metadata extraction for images, audio, video, PDF/Office, archives,
SQLite and network captures, followed by Pakistan structured golden expansion.
Model downloads and sensitive real-data ingest retain separate approval gates.

### 2026-07-24 — Phase 2B.1 Pakistan deployment and deterministic media v1

- Preserved pre-change rollback images for the forensic API and worker, then
  rebuilt and redeployed only those two sidecars. Current API image is
  `sha256:d32e5118d18849fd4aae024944e547573c9643189a7b5bb2a9f72cbecde64980`;
  worker is
  `sha256:faeb65590b1f6a61e7458a9c73c763b5c82dc772df29f20fd8843691c9615268`.
  LocalAI remained unchanged and healthy on
  `sha256:0731ab05fa080fae9bc90ddbcc26f74fcb554389ee82aa0282e5373a27144c9b`.
- Verified the worker contains `tzdata=2026.3`, resolves `Asia/Karachi`, and
  subscribed to NATS after recreation. API health returned OK.
- Ran the synthetic messy Pakistan CDR through the real LocalAI → API → NATS →
  worker → PostgreSQL pipeline. Final acceptance collection
  `forensic-pakistan-cdr-acceptance-20260724` has one KB entry, one evidence item,
  one completed job, 5 input rows, 4 accepted, 1 exact duplicate and 0 rejected.
  All accepted IMEIs remained 14 digits, the negative cell sentinel survived,
  and naïve 08:00 Pakistan time canonicalized to 03:00 UTC.
- The first diagnostic run exposed missing `jurisdiction` and `source_timezone`
  keys on evidence metadata even though job metadata was correct. Added a pure
  evidence metadata builder and Ginkgo regression, rebuilt, redeployed, and
  verified the final evidence carries `PK` and `Asia/Karachi`.
- Implemented deterministic media metadata extractor version 1.0.0 during
  evidence registration. PNG/JPEG/GIF use a bounded header-only dimension parse;
  WAV exposes format code, channels, sample rate, byte rate, bit depth, data bytes
  and duration. Corrupt supported media remains registered with a warning.
- Live collection `forensic-media-metadata-acceptance-20260724` verified a 16×16
  PNG and a mono 8 kHz, 16-bit, 250 ms PCM WAV. Both have complete Pakistan
  provenance and zero warnings/errors; OCR and STT routes correctly remain
  pending.
- Final Go forensic API tests passed after provenance (29.733 s), media extractor
  (16.829 s), and PNG/JPEG/GIF coverage (30.937 s). Earlier Python worker/adapter
  total remains 17/17 passing. Detailed acceptance report:
  `reports/forensic-phase2b1-pakistan-media-acceptance-20260724.md`.
- `records-demo-verified` remains unchanged at 9,250 records. No real CDR ingest,
  database backfill, schema migration, model download, Git staging, commit, push,
  pull request, or publication occurred.

Current status: Pakistan defaults and evidence provenance are live. Deterministic
PNG/JPEG/GIF and WAV metadata are live and separately identified from pending
OCR/STT. Continue Phase 2B.1 with EXIF and additional bounded image/audio headers,
then PDF/archive/SQLite/capture/video inventories. Model downloads and real-data
ingest remain separate approval gates.

### 2026-07-27 — Phase 2B.1 deterministic media headers v1.1

1. **Objective:** extend model-free technical metadata to BMP, TIFF/EXIF,
   WebP, and FLAC; harden WAV; keep OCR/STT routes explicitly pending.
2. **Assumptions verified:** the prior API digest was `sha256:d32e5118...`;
   LocalAI was healthy; the laptop restart had left the forensic sidecars exited
   with persistent volumes intact; pre-build capacity was 4.45 GiB RAM and
   1,781.04 GiB disk.
3. **Files changed:** added `api/forensic_records/media_metadata_image.go` and
   `reports/forensic-phase2b1-media-headers-acceptance-20260727.md`; updated the
   media extractor, Ginkgo tests, forensic feature documentation, and this
   checkpoint. Unrelated worktree changes were preserved.
4. **Schema/API/config:** no schema, migration, endpoint, auth, RLS, Compose, or
   environment change. Existing `media_metadata` now carries extractor `1.1.0`.
5. **Tests:** `go test ./api/forensic_records -count=1` passed in 9.741 seconds;
   `go vet ./api/forensic_records` passed. Positive coverage includes BMP, TIFF
   orientation, WebP, JPEG EXIF, FLAC, and extension mismatch. Bounded rejection
   covers BMP size overflow, a 513-entry TIFF IFD, WebP/WAV chunk overflow, and
   zero-rate FLAC.
6. **Live runtime:** built only the forensic API in 43.5 seconds and deployed
   `sha256:43489c192a7d24982ad8361f3aa84d048b2aaf9c379869c8a866ce7b11974aba`.
   PostgreSQL/NATS are healthy; API/worker are running; LocalAI is healthy and
   unchanged on `sha256:0731ab05...` with both baseline models visible.
7. **Dataset/evidence:** neutral collection
   `forensic-media-headers-acceptance-20260727` contains four synthetic sources,
   four KB entries, four evidence items, and four KB assets. BMP 7x5, TIFF 9x6
   orientation 6/display 6x9, WebP 11x8, and FLAC stereo 48 kHz/24-bit/2,000 ms
   matched exactly. All carry `PK`/`Asia/Karachi`; warnings/errors are zero.
8. **Models:** no model call/download/change. Visible IDs remain
   `qwen3-embedding-0.6b` and `qwen_qwen3-4b-instruct-2507`.
9. **Accuracy/retrieval:** 4/4 live deterministic field agreements, 5/5 targeted
   malformed-header rejections, 4/4 KB mirrors, and 4/4 evidence-asset links.
   No OCR/ASR/vision/TTS accuracy is claimed.
10. **Latency/memory:** health 226.05 ms; four-item evidence catalog 30.94 ms;
    API 7.555 MiB, worker 51.94 MiB, PostgreSQL 51.56 MiB, NATS 8.23 MiB,
    LocalAI 644.5 MiB; post-run free RAM 3.22 GiB. These are local snapshots.
11. **Failures/fallbacks:** the first acceptance client stopped before HTTP
    because Windows PowerShell had not loaded `System.Net.Http`; loading the
    standard assembly fixed it without partial evidence state. Explicit LocalAI
    forwarding of caller jurisdiction/timezone remains source-only; deployed API
    Pakistan defaults remain the verified fallback.
12. **Security/provenance:** 4 MiB header, 512 chunk/segment, 512 IFD-entry, and
    100-million-pixel review limits are explicit. Parsers execute no codecs or
    models and retain no EXIF GPS/free text. Hash, evidence/version, KB entry,
    case, jurisdiction, timezone, warning, and extractor lineage remain intact.
13. **Git:** dirty worktree remains unstaged, uncommitted, and unpushed; no
    GitHub or external publication action occurred.
14. **Rollback:** `nexusai-forensic-records-api:rollback-20260727-media-v1.0`
    preserves `sha256:d32e5118...`; persistent volumes and earlier rollback
    images remain intact.
15. **Next action/approval:** continue bounded PDF/archive/SQLite/PCAP/video and
    Pakistan ANPR/finance/log goldens. Model downloads, real sensitive ingest,
    database backfill/migrations, heavy LocalAI rebuild, and Git publication
    remain separate approval gates. `records-demo-verified` remains exactly
    9,250 accepted rows with zero rejected/duplicate rows.

### 2026-07-27 — Phase 2B.1 PDF and archive inventory v1.2

1. **Objective:** add model-free bounded structural inventory for PDF,
   ZIP/ZIP64, and TAR; surface archive/document risks consistently without
   claiming text extraction or archive unpacking.
2. **Assumptions verified:** previous API was `sha256:43489c192...`; LocalAI,
   worker, PostgreSQL, and NATS were healthy; pre-build capacity was about 5.2
   GiB RAM and 1,781 GiB disk; `records-demo-verified` remained untouched.
3. **Files changed:** added PDF/archive inventory implementations and Ginkgo
   adversarial tests; updated extractor dispatch/version, registration warning
   propagation, evaluation matrix, feature docs, the detailed acceptance report,
   and this checkpoint. Unrelated worktree changes were preserved.
4. **Schema/API/config:** no migration, backfill, endpoint, auth/RLS, Compose, or
   environment change. Existing `media_metadata` is version 1.2.0. The existing
   upload warning field now includes deduplicated registration-time warnings.
5. **Tests:** final `go test ./api/forensic_records -count=1` passed in 10.320
   seconds; `go vet ./api/forensic_records` passed; evaluation matrix JSON
   parsed; diff check found no whitespace errors.
6. **Live runtime:** after a first v1.2 candidate exposed warning-return drift,
   rebuilt and deployed API `sha256:12a5448490e3456cf65b16ded18e70d573e08bc9f0073fd7b3fac7ba028bf701`.
   LocalAI/worker were not rebuilt; all services remained healthy with zero
   API/worker restarts.
7. **Dataset/evidence:** clean collection
   `forensic-document-archive-acceptance-20260727-v2` has 5 sources, 5 evidence
   items, 5 KB assets, 5 KB entries, 0 failures/errors, and 0 expected worker
   jobs. The malformed first diagnostic remains preserved. Isolated warning
   acceptance has 1 hostile ZIP, 1 evidence, 1 asset, and 1 KB entry.
8. **Models:** no call/download/change. Visible IDs remain
   `qwen3-embedding-0.6b` and `qwen_qwen3-4b-instruct-2507`.
9. **Accuracy/retrieval:** 5/5 clean technical inventories and 5/5 KB/evidence/
   asset links agreed. All 5 expected warning strings appeared. In the isolated
   test, all 3 hostile-ZIP warnings matched upload, evidence, and KB asset
   `quality_report` values. No OCR or semantic accuracy is claimed.
10. **Latency/memory:** health 3.83 ms; five-item catalog 6.63 ms; API 5.215 MiB,
    worker 47.02 MiB, PostgreSQL 54.52 MiB, NATS 8.367 MiB, LocalAI 671.8 MiB;
    free RAM 3.77 GiB. These are local single snapshots.
11. **Failures/fallbacks:** a PowerShell fixture bug produced a preserved
    malformed 90-byte diagnostic PDF; corrected v2 fixtures passed. Warning
    response drift was fixed and live-verified. PDF remains bounded lexical
    inventory; ZIP is central-directory inventory; TAR PAX/GNU is incomplete;
    RAR/7z/compressed TAR/recursive extraction remain pending.
12. **Security/provenance:** PDF scan is capped at first 64 MiB plus final 128
    KiB and 500,000 tokens. Archives are capped at 10,000 members, 64 MiB ZIP
    directory, 2 GiB/member, 10 GiB total, and 100:1 review ratio. No actions,
    general decompression, PDF text, or member-name list is persisted. Existing
    tenant, hash, evidence/version, KB, audit, Pakistan timezone, and jurisdiction
    lineage remains intact.
13. **Git:** dirty worktree remains unstaged, uncommitted, and unpushed; no
    GitHub or publication action occurred.
14. **Rollback:** v1.1 is tagged
    `nexusai-forensic-records-api:rollback-20260727-media-v1.1`; the first v1.2
    candidate is tagged
    `nexusai-forensic-records-api:rollback-20260727-file-inventory-pre-warning-fix`;
    persistent volumes and diagnostic evidence remain intact.
15. **Next action/approval:** implement bounded read-only SQLite inventory next,
    then PCAP/PCAPNG and MP3/MP4/video headers, while expanding Pakistan ANPR,
    finance, subscriber/tower, log, and bilingual goldens. Models, real sensitive
    ingest, migrations/backfills, heavy LocalAI rebuild, destructive cleanup, and
    Git publication retain separate approval gates. Detailed report:
    `reports/forensic-phase2b1-pdf-archive-acceptance-20260727.md`.

### 2026-07-27 — Phase 2B.1 deterministic SQLite inventory v1.3

1. **Objective:** add bounded raw read-only SQLite header, schema-object, and
   table-row-count inventory without SQL execution, extensions, sidecars, or
   evidence data-value reads.
2. **Assumptions verified:** prior API was `sha256:12a544849...`; LocalAI,
   worker, PostgreSQL, and NATS were healthy; pre-build capacity was 4.30 GiB
   RAM and 1,780.79 GiB disk; the API remains CGO-free.
3. **Files changed:** added `media_metadata_sqlite.go` and seven Ginkgo SQLite
   cases; updated extractor v1.3 dispatch, prior version assertion, evaluation
   matrix, feature docs, detailed report, and this checkpoint. Unrelated edits
   were preserved.
4. **Schema/API/config:** no migration, backfill, endpoint, auth/RLS, Compose, or
   environment change. SQLite metadata uses the existing JSON field; the route
   remains `database_adapter_pending` and never queues the records worker.
5. **Tests:** 69 Ginkgo specs; final package test passed in 29.579 seconds;
   `go vet` passed; matrix JSON parsed. Windows default Go cache ACL failed, then
   the documented workspace-cache fallback passed and was removed afterward.
6. **Live runtime:** API-only build/recreate took 47.6 seconds, compile 33.6
   seconds. Deployed `sha256:9dc1ec79b34aa69913230f2d76e7e4b89c1c1795a7031cad146f381eb8032772`;
   API/worker restart counts are zero; LocalAI is unchanged and healthy.
7. **Dataset/evidence:** neutral collection
   `forensic-sqlite-inventory-acceptance-20260727` has 2 synthetic real SQLite
   files, 2 evidence, 2 KB assets, 2 KB entries, 32,768 bytes, 0 jobs/failures.
8. **Models:** no call/download/change. Visible IDs remain
   `qwen_qwen3-4b-instruct-2507` and `qwen3-embedding-0.6b`.
9. **Accuracy/retrieval:** both sources matched page/header fields and 4/4 schema
   objects; exact table counts `calls=3`, `accounts=2` matched on both, including
   `WITHOUT ROWID`. The one WAL warning matched upload/evidence/KB asset. No
   column, foreign-key, selected-row, SQL, recovery, or semantic claim is made.
10. **Latency/memory:** health 400.55 ms; two-item catalog 134.92 ms; API 8.168
    MiB, worker 47.02 MiB, PostgreSQL 52.4 MiB, NATS 8.805 MiB, LocalAI 667.2
    MiB; free RAM 3.34 GiB. These are single snapshots.
11. **Failures/fallbacks:** default Go cache ACL required the documented local
    cache; one PowerShell read-only URL needed `${id}` delimiting. WAL sidecars,
    columns, foreign keys, selected rows, recovery, SQL dumps, and other engines
    remain pending.
12. **Security/provenance:** raw `ReadAt` only; caps are 4,096 pages/64 MiB,
    2,048 schema objects, 1 MiB/record, 8 MiB schema payload, 512-byte names,
    depth 64, and 256 tables. Cycles/corruption fail closed; existing tenant,
    evidence/version/hash/KB/Pakistan/audit lineage remains intact.
13. **Git:** dirty worktree remains unstaged, uncommitted, and unpushed; no
    GitHub/publication action occurred.
14. **Rollback:** pre-SQLite v1.2 is tagged
    `nexusai-forensic-records-api:rollback-20260727-file-inventory-v1.2` at
    `sha256:12a544849...`; no schema/data reversal is needed.
15. **Next action/approval:** implement bounded PCAP/PCAPNG metadata next, then
    MP3/MP4/video headers and Pakistan structured goldens. Models, real sensitive
    ingest, migrations/backfills, heavy LocalAI rebuild, destructive evidence
    cleanup, and Git publication retain separate approval gates. Detailed report:
    `reports/forensic-phase2b1-sqlite-acceptance-20260727.md`.

Current status: deterministic image/audio headers, PDF structure, ZIP/TAR safety,
and SQLite header/schema/table-count inventory are live. All are technical
inventory foundations, not OCR/STT/document extraction/archive extraction or
database query adapters. The next bounded implementation slice is PCAP/PCAPNG.

### 2026-07-27 — Phase 2B.1 deterministic PCAP/PCAPNG inventory v1.4

1. **Objective:** inventory classic PCAP and PCAPNG structure, interfaces,
   packet lengths/counts and timestamps without reading payloads or extracting
   endpoints/protocols/sessions/secrets.
2. **Assumptions verified:** previous API was `sha256:9dc1ec79...`; all services
   healthy; 4.34 GiB RAM and 1,780.75 GiB disk free; API remains CGO-free.
3. **Files changed:** capture parser and eight Ginkgo cases added; extractor,
   capture modality, evaluation matrix, docs, acceptance report, plain-language
   team brief, and checkpoint updated. Unrelated edits preserved.
4. **Schema/API/config:** no migration, endpoint, auth/RLS, Compose, dependency,
   or environment change. Metadata uses the existing JSON field; capture route
   stays pending and never queues the worker.
5. **Tests:** 77 Ginkgo specs; final package test passed in 29.418 seconds;
   `go vet` passed; matrix JSON parsed. Initial compile found only two unused
   imports, removed before the green run.
6. **Live runtime:** API-only build/recreate 46.5 seconds, compile 34.7 seconds;
   deployed `sha256:bff1fd08162918504ba3d503b03ce1ef964c7165bdedeaf4a7de8673e43d68f5`.
   LocalAI/worker unchanged; services healthy, restart counts zero.
7. **Dataset/evidence:** `forensic-packet-capture-acceptance-20260727` contains
   3 synthetic captures, 3 evidence, 3 KB assets, 3 KB entries, 522 bytes, zero
   jobs/failures.
8. **Models:** no call/download/change; baseline chat and embedding IDs unchanged.
9. **Accuracy/retrieval:** exact classic PCAP 2 packets/102 captured/124 original;
   PCAPNG 1 section/2 interfaces/6 blocks/3 packets/84 captured/104 original;
   secret-marker 1 secrets block with no retained secret. Warning matched upload,
   evidence, and asset. No protocol/session claim.
10. **Latency/memory:** health 275.26 ms; catalog 228.08 ms; API 7.18 MiB,
    worker 47.02 MiB, PostgreSQL 52.78 MiB, NATS 9.797 MiB, LocalAI 675.8 MiB;
    free RAM 3.79 GiB. Single snapshots.
11. **Failures/fallbacks:** two unused imports were corrected before tests. Deep
    protocols/endpoints/flows/sessions, long-capture benchmarks, common link-type
    expansion, packet locators and EVTX remain pending.
12. **Security/provenance:** caps: 1M packets, 1.25M blocks, 1,024 sections,
    10,000 interfaces, 64 MiB headers, 16 MiB packet, 32 MiB block, 64 KiB
    interface options. Payloads/names/addresses/filters/resolution values/secrets
    are not retained. Existing evidence/version/hash/KB/Pakistan lineage intact.
13. **Git:** unstaged, uncommitted, unpushed; no GitHub/publication action.
14. **Rollback:** SQLite v1.3 preserved as
    `nexusai-forensic-records-api:rollback-20260727-sqlite-v1.3` at
    `sha256:9dc1ec79...`; no schema/data reversal required.
15. **Next action/approval:** implement MP3/MP4/MOV/video container metadata,
    then Pakistan structured goldens. Deep capture sessions remain separate.
    Models, sensitive data, migrations, heavy LocalAI rebuild and Git publication
    retain approval gates. Reports:
    `reports/forensic-phase2b1-pcap-pcapng-acceptance-20260727.md` and
    `reports/forensic-team-lead-brief-20260727.md`.

Current status: universal governed registration, exact structured records,
Pakistan defaults/profile, hybrid SQL+KB answers, and bounded image/audio/PDF/
archive/SQLite/PCAP technical inventories are live and rollback-protected. The
next small deterministic slice is MP3 plus MP4/MOV/video container metadata.

### 2026-07-27 — Phase 2B.1 deterministic MPEG media inventory v1.5

1. **Objective:** inventory MP3 frames/ID3 boundaries and MP4/M4A/M4V/MOV/3GP
   boxes, tracks, codecs, duration, audio fields, dimensions and rotation without
   decoding or reading compressed media payloads/tag values.
2. **Assumptions verified:** prior API `sha256:bff1fd081...`, restart zero;
   LocalAI/PostgreSQL/NATS/worker healthy; existing metadata JSON needs no schema.
3. **Files changed:** MPEG parser and eleven Ginkgo specs added; extractor v1.5,
   version assertions, matrix, docs, team brief, report and checkpoint updated.
4. **Schema/API/config:** no migration, endpoint, auth/RLS, Compose, dependency,
   environment, queue, model or worker change; audio/video routes remain pending.
5. **Tests:** focused 11/11 passed; final package 88/88 Ginkgo plus legacy tests,
   6.427 seconds; `go vet` passed; matrix JSON parsed.
6. **Live runtime:** API-only build/recreate 46.7 seconds; deployed
   `sha256:f24d90c5c7941165d137492420d486fd82fbb0c95af62c173f15bb0ec1a6e772`;
   API/LocalAI/PostgreSQL/NATS healthy and API restart count zero.
7. **Dataset/evidence:** `forensic-mpeg-container-acceptance-20260727` has three
   synthetic sources, 2,595 bytes, 3 evidence, 3 KB assets, 3 KB entries, zero
   records jobs/failures; case `PK-SYNTHETIC-MPEG-20260727`.
8. **Models:** no call/download/change; Qwen chat and embedding baseline unchanged.
9. **Accuracy/retrieval:** MP3 exact 3 frames/3,456 samples/78 ms/128-160 kbps;
   MP4 exact 2 tracks/`avc1`+`mp4a`/5,000 ms/1280x720/90 degrees/48 kHz;
   zero compressed/`mdat` bytes read. QuickTime warning matched upload, evidence,
   status and asset. Protected baseline remains 9,250 accepted, zero failures.
10. **Latency/memory:** health 287.04 ms; catalog 66.90 ms; API 7.203 MiB,
    worker 47.02 MiB, PostgreSQL 53.7 MiB, NATS 9.18 MiB; single snapshots.
11. **Failures/fallbacks:** workspace Go cache, PowerShell Ginkgo quoting and one
    read-only URL interpolation issue were corrected; no product/acceptance failure.
    Real codecs/fragmentation, other audio/video containers, frames/STT/vision pending.
12. **Security/provenance:** MP3 1M frames/4 KiB frame/8 MiB headers; ISO 100k
    boxes/depth 12/4,096 tracks/256 sample entries/64 brands/8 MiB headers.
    Tags, compressed audio, `mdat`, metadata values and frames remain unread;
    tenant/evidence/version/hash/KB/Pakistan/audit lineage intact.
13. **Git:** dirty worktree remains unstaged, uncommitted, unpushed; no GitHub action.
14. **Rollback:** capture v1.4 tagged
    `nexusai-forensic-records-api:rollback-20260727-capture-v1.4` at
    `sha256:bff1fd081...`; no schema/data reversal needed.
15. **Next action/approval:** expand Pakistan structured goldens, beginning with
    messy ANPR and PKR financial transactions, then subscriber/tower/access logs.
    Models, real sensitive data, migrations, heavy LocalAI builds, destructive
    cleanup and Git publication retain separate approval gates. Report:
    `reports/forensic-phase2b1-mpeg-media-acceptance-20260727.md`.

### 2026-07-27 — Phase 2B.2 Pakistan ANPR and PKR structured goldens v1.1

1. **Objective:** harden Pakistan ANPR and financial-transaction ingestion with
   realistic messy synthetic formats, exact expected outputs and live row
   accounting without ingesting the supplied real CDR or any sensitive evidence.
2. **Assumptions verified:** the previous API/worker containers and named data
   volumes survived a Docker Desktop restart; LocalAI remained healthy; exact
   rollback image IDs were captured before rebuilding.
3. **Files changed:** worker normalization, API header aliases, Ginkgo/Python
   tests, two synthetic fixtures plus manifest, Pakistan profile v1.1, evaluation
   matrix, product docs, team-lead brief, phase report and checkpoint.
4. **Schema/API/config:** no migration, endpoint, auth/RLS, Compose, dependency,
   queue or model change. Derived values merge into existing
   `metadata.normalized_fields`; immutable raw rows and the legacy store remain.
5. **Tests:** 17 adapter tests passed in 0.063 s; combined worker/adapter suite
   21/21 passed in 0.157 s; complete forensic Go package passed in 10.033 s;
   Python compile and both JSON contracts passed.
6. **Live runtime:** targeted API+worker build completed in 43.7 s. Deployed API
   `sha256:39a1bc8d9af8f3bb43352ac75ac36dff26281932abbe34856d302328f39d4869`
   and worker `sha256:c2e7fe083355f2bd1a031ccd24c44883dbb9a25253a553ecc079b0703eee73c9`;
   API/worker running, PostgreSQL/NATS healthy, all restart counts zero.
7. **Dataset/evidence:** neutral collection
   `forensic-pakistan-structured-goldens-20260727`, case
   `PK-SYNTHETIC-STRUCTURED-20260727`; 2 synthetic files, 5,110 bytes, 21 rows,
   2 jobs, 2 evidence, 2 KB assets and 2 KB entries; no missing asset/failure.
8. **Models:** no call, download, install or configuration change; Qwen chat and
   1024-dimensional embedding baseline unchanged.
9. **Accuracy/retrieval:** exact 11 unique accepted + 2 duplicates + 8 rejected;
   ANPR 5+1+4 and transaction 6+1+4. Stored Urdu/search/script/confidence/time,
   exact decimal/minor units/reversal and IBAN flags matched goldens. This slice
   tests deterministic extraction, not semantic retrieval or model accuracy.
10. **Latency/memory:** health 7.246 ms; collection status 18.836 ms; single
    snapshots API 5.484 MiB, worker 57.89 MiB, PostgreSQL 57.91 MiB, NATS
    8.566 MiB. These are diagnostic snapshots, not load-test percentiles.
11. **Failures/fallbacks:** missing Windows tzdata initially blocked local tests;
    UTC and Pakistan's current fixed UTC+05:00 received deterministic fallbacks
    while `ZoneInfo` remains authoritative when available. Docker Desktop had
    stopped the forensic stack with exit 255; existing volumes were reused and
    no data recovery was required. One read-only SQL quoting attempt was corrected.
12. **Security/provenance:** fixtures are synthetic. Raw rows, row hashes,
    tenant/collection/case/evidence/version/KB lineage and row-numbered rejects
    remain. No real CDR, account, plate, image, subscriber or identity data was
    ingested or committed.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
    action.
14. **Rollback:** API tag
    `nexusai-forensic-records-api:rollback-20260727-mpeg-v1.5` points to
    `sha256:f24d90c5...`; worker tag
    `nexusai-forensic-records-worker:rollback-20260727-pre-pk-goldens` points to
    `sha256:faeb65590...`; no schema/data reversal is needed.
15. **Next action/approval:** add Pakistan subscriber/history, tower/sector and
    access/security-log messy goldens, then deterministic cross-family joins.
    Models, real sensitive data, migrations, heavy LocalAI builds, destructive
    cleanup and Git publication retain separate approval gates. Report:
    `reports/forensic-phase2b2-pakistan-structured-goldens-acceptance-20260727.md`.

Status at that checkpoint: governed registration; exact CDR/IPDR/ANPR/transaction/log
records; Pakistan profile v1.1 and live messy ANPR/PKR acceptance; hybrid SQL+KB;
and bounded image/audio/PDF/archive/SQLite/PCAP/MP3/MP4 inventories are live and
rollback-protected. The then-next bounded slice was Pakistan subscriber, tower and
access/security-log goldens followed by cross-family joins.

### 2026-07-27 — Phase 2 complete: all subphases and acceptance gates

1. **Objective:** close Phase 2A registry work and Phase 2B fixtures/evaluation
   across all 20 evidence families without promoting unimplemented deep adapters.
2. **Assumptions verified:** the attached real CDR remained read-only; all final
   acceptance inputs are synthetic; dirty worktree and volumes preserved; current
   Qwen model pair unchanged.
3. **Files changed:** subscriber/tower/access worker aliases and normalization;
   API classifiers and queries; 15 fixture/golden contracts; Pakistan profile
   v1.2; 20-profile matrix; fail-closed benchmark; product docs, team brief,
   completion report and checkpoint.
4. **Schema/API/config:** no migration, new endpoint, dependency, subject or model
   config. Canonical JSON metadata stores derived values; raw rows remain. Tower
   activity now spans CDR+tower rows; schema profile safely handles JSON null.
5. **Tests:** Python 24/24 in 0.669 s; final Go 97/97 Ginkgo plus legacy tests;
   `go vet`, Compose config, Python compilation, four JSON contracts and diff
   whitespace passed.
6. **Live runtime:** targeted API/worker builds only. Final API
   `sha256:2fc0425044baeda5339cc4f06b8e242ef4a5734c70254e3d58861418dcec9473`;
   worker `sha256:b0b66c87d11c556bcdee1a184806c270c0a393b57280436778fc2b7f4222e233`;
   LocalAI/API/NATS/worker healthy.
7. **Dataset/evidence:** final neutral collection has 20 evidence, 20 KB assets,
   20 KB entries, 8 completed jobs, zero failures/missing assets, and 59 structured
   rows = 34 accepted + 6 duplicate + 19 rejected. Twelve pending/manual items
   were not queued.
8. **Models:** unchanged `qwen_qwen3-4b-instruct-2507` and
   `qwen3-embedding-0.6b`; CPU, three rounds, temperature 0, max 256 chat tokens;
   no download/promotion.
9. **Accuracy/retrieval:** matrix 20/20 ready and SHA-matched; chat 18/18 with
   term agreement 1.0 and lexical unsupported-claim proxy 0.0; embeddings three
   runs of 8 x 1,024; deterministic routes 27/27 fail-closed. Structured row and
   normalization goldens matched exactly.
10. **Latency/memory:** chat avg/p50/p95 13,663.93/10,672.12/20,274.93 ms;
    embeddings avg/p95 2,185.99/2,584.72 ms; sidecar avg/p95 191.71/1,102.20 ms;
    free RAM 1.38 GiB; API 13.53 MiB, worker 56.57 MiB, PostgreSQL 133.7 MiB,
    NATS 17.47 MiB, LocalAI 4.637 GiB.
11. **Failures/fallbacks:** acceptance found and fixed CDR-only tower activity,
    JSON-null schema profiling, and benchmark route-error undercounting. Final
    rerun has zero route errors. OCR/STT/deep media/session/archive/database work
    remains truthful future scope.
12. **Security/provenance:** synthetic fixtures only; raw/source/row/hash/tenant/
    collection/case/evidence/version/KB/rejection/audit lineage preserved. Invalid
    or ambiguous Pakistan identifiers are flagged, not silently inferred.
13. **Git:** unstaged, uncommitted, unpushed; no PR/publication.
14. **Rollback:** scoped pre-closure PostgreSQL custom dump is 102,065 bytes,
    SHA-256 `8194527272738f4feeb0440217dc989d043d9506708dfd04138218b5cb2795ff`;
    API/worker rollback tags retained. Production registry is 8 KB entries/6
    evidence/8 links and protected structured rows remain exactly 9,250.
15. **Next action/approval:** Phase 3 control-plane design/tests may start. Apply
    no migration, real-data ingest, model change, full LocalAI build, policy-
    sensitive identity capability, database restore/replacement, or Git action
    without separate approval. Detailed report:
    `reports/forensic-phase2-completion-acceptance-20260727.md`.

### 2026-07-27 — Phase 3 migration candidate: evidence control plane v1

1. **Objective:** implement and isolate-test normalized storage objects, evidence
   versions/source links, processing runs/events, typed artifacts and custody.
2. **Assumptions verified:** synthetic migration fixtures only; migrations 001-007
   are the compatibility baseline; retained registry application remains gated.
3. **Files changed:** migration 008 plus preflight/verify/fixture/smoke/idempotency/
   rollback SQL, Go contracts, design/product/DB docs, report and team brief.
4. **Schema/API/config:** seven additive RLS tables, current-version pointer,
   tenant-scoped FKs, append-only/identity/custody triggers and legacy sync. No
   endpoint, queue subject, model or active config changed.
5. **Tests:** 102/102 Ginkgo plus legacy tests in 3.152 s; Phase 3 focus passed;
   `go vet`; Python 24/24 in 0.297 s; Compose/JSON/compile/diff checks passed.
6. **Live runtime:** no build/recreate/migration. Read-only active-registry
   preflight passed 110 evidence, 104 linked KB assets and 41 linked jobs.
7. **Dataset/evidence:** isolated legacy totals 1 storage/1 version/3 links/1
   run/1 processing/1 custody; native added 1/1/2/1/3/3; second apply retained
   combined 2/2/5/2/4/4 exactly.
8. **Models:** current Qwen chat/embedding pair unchanged; no download/promotion.
9. **Accuracy/retrieval:** not applicable; Phase 2 20/20, 18/18 and 27/27 gates
   remain fixed regressions.
10. **Latency/memory:** final Go package 3.152 s; Python 0.297 s; no active model
    or service memory change.
11. **Failures/fallbacks:** fixed source-link reapply drift, timestamp fixture,
    partial-DDL risk and deferred-FK/RLS ordering. Injected failure rolled back
    completely.
12. **Security/provenance:** tenant FKs/RLS, immutable identities, append-only
    events and serialized SHA-256 custody chain added. Auth binding remains open.
13. **Git:** unstaged, uncommitted, unpushed; no PR/publication.
14. **Rollback:** disabled by default; explicit disposable rollback removed Phase
    3 atomically and preserved 2 Phase 2 evidence rows.
15. **Next action/approval:** source-only auth/RLS and queue-lifecycle work may
    continue. Before applying 008, create/verify a fresh scoped backup and obtain
    explicit database-migration approval. Detailed report:
    `reports/forensic-phase3-control-plane-migration-candidate-20260727.md`.

### 2026-07-28 — Phase 3 authenticated scope and non-owner RLS

1. **Objective:** bind tenant/case/collection/actor/subject to authenticated
   service context and prove Phase 3 RLS using a genuine non-owner role.
2. **Assumptions verified:** LocalAI owns external authentication and collection
   authorization; retained deployment/database changes remain separately gated.
3. **Files changed:** sidecar auth middleware/tests, LocalAI proxy/upload/routes,
   agent tool scope, Compose source config, RLS smoke SQL, design/product docs,
   team brief and acceptance report.
4. **Schema/API/config:** no new route or applied schema. LocalAI exact-ownership
   checks and bearer-authenticated trusted headers added; body/query scope cannot
   override them; repair is admin-only; auth-required startup fails without key.
5. **Tests:** 110/110 forensic Ginkgo specs plus legacy tests (package 10.049 s);
   LocalAI focus 33.574 s plus final 21/21 explicit-binary specs in 0.016 s;
   agent services 126.976 s; routes 15.774 s; `go vet` clean; Python 24/24 in
   0.541 s; Compose and Python compile passed.
6. **Live runtime:** no build/recreate, secret activation, migration, retained DB
   write, real-data ingest, model change, or service-state change.
7. **Dataset/evidence:** disposable synthetic Phase 3 tenant only; same-tenant
   counts 1/1/2/1/3/0/3 across storage/version/link/run/event/artifact/custody.
8. **Models:** current Qwen chat/embedding baseline unchanged; no download or
   promotion.
9. **Accuracy/retrieval:** Phase 2 20/20 fixture, 18/18 chat and 27/27 route
   baselines remain fixed and unaffected.
10. **Latency/memory:** longest relevant validation was agent services 126.976 s;
    no live model or service memory change.
11. **Failures/fallbacks:** Windows test-EXE cleanup locks occurred after package
    `ok`; explicit-binary execution exited zero and generated artifacts were
    removed. Compose Docker-config warning was non-fatal. Route compile filled
    disclosed Go-cache modules; later Go checks were offline with `GOPROXY=off`.
    Swagger preview stopped on an uncached command-only dependency; no download
    or tracked Swagger change occurred, so regeneration remains a tooling gate.
12. **Security/provenance:** constant-time bearer check; public health only;
    distinct actor/subject; exact collection ownership; tenant/case rebinding;
    collection-filtered evidence joins; admin-only repair; non-bypass RLS role
    accepted same-tenant and rejected cross-tenant insert.
13. **Git:** unstaged, uncommitted and unpushed; no PR/publication.
14. **Rollback:** source auth remains undeployed; isolated RLS role/insert rolled
    back and its temporary container was removed; retained database untouched.
15. **Next action/approval:** implement immutable retention/hash re-verification
    and reliable JetStream acknowledgement/retry/DLQ/reprocessing in source.
    Deployment needs separate secret, build/recreate, runtime-grant, backup and
    migration approvals; generated Swagger also needs an approved pinned CLI
    dependency or generator container. Detailed report:
    `reports/forensic-phase3-authenticated-scope-rls-20260728.md`.

### 2026-07-29 — Phase 3 scoped content-addressed retention

1. **Objective:** replace mutable job-named spool assumptions with scoped,
   create-only content objects, read-only receipts and full hash re-verification.
2. **Assumptions verified:** API is the spool writer; worker supports read-only
   consumption; source filename must remain parser metadata; runtime remains gated.
3. **Files changed:** storage/API/worker code and tests, Compose source config,
   migration 008 contracts, design/operator docs, team brief and phase report.
4. **Schema/API/config:** no new route or applied schema. Upload/job metadata now
   carries stable object/receipt URI, layout, scope, actual size, verified state,
   time/method, write-once flag and reuse result; worker mount becomes read-only.
5. **Tests:** full forensic package passed with 119/119 Ginkgo specs plus legacy
   tests (24.397 s test, 99.6 s cold wall); `go vet` clean; Python 27/27 in
   2.116 s; compile and Compose passed; five independent storage focus runs passed.
6. **Live runtime:** no build/recreate, migration, retained DB write, evidence
   ingest, model/secret change or volume rewrite.
7. **Dataset/evidence:** synthetic storage bytes and disposable database rows
   only; retained registry and supplied Pakistan CDR were untouched.
8. **Models:** current Qwen chat/embedding baseline unchanged; no download or
   promotion.
9. **Accuracy/retrieval:** actual streamed size/hash are authoritative;
   extensionless objects use preserved source filename; Phase 2 claims unchanged.
10. **Latency/memory:** no live performance/RAM state changed; bounded storage
    retry handles only short cross-process publish windows.
11. **Failures/fallbacks:** cold compile timeout later passed; Windows root
    symlink evaluation was made platform-aware; disposable harness readiness,
    filename and result-query mistakes were corrected. The harness found and the
    source fixed a real nullable legacy `write_once` backfill defect.
12. **Security/provenance:** hashed scope paths, atomic no-overwrite publication,
    read-only object/receipt, full verification, traversal/symlink/tamper checks,
    12-writer convergence and conflict quarantine pass. Not regulatory WORM.
13. **Git:** unstaged, uncommitted and unpushed; no PR/publication.
14. **Rollback:** source remains undeployed. Full disposable 001–008/preflight/
    verify/smoke/reapply/idempotency/RLS chain passed with exact verified storage
    assertion; container removed; retained database untouched.
15. **Next action/approval:** implement reliable JetStream durable acknowledgement,
    bounded retry/backoff, dead-letter, lease/concurrency and explicit
    reprocessing without erasing outputs. Deployment/migration/backup/runtime
    grants/infrastructure WORM remain separately gated. Detailed report:
    `reports/forensic-phase3-content-addressed-retention-20260729.md`.

### 2026-07-29 — Phase 3 durable queue lifecycle and source completion

1. **Objective:** close Phase 3 with crash-safe publication, concurrent worker
   leases, bounded retry/DLQ and immutable explicit reprocessing.
2. **Assumptions verified:** JetStream is at-least-once; PostgreSQL is the job
   authority; the existing content-addressed object is read-only worker input;
   retained runtime/database/Git remain outside source-only acceptance.
3. **Files changed:** migration 009 family; API outbox/JetStream/reprocess source
   and tests; worker lifecycle/tests; LocalAI proxy/route/auth/client; Compose;
   design/operator/product docs, team brief, completion report and checkpoint.
4. **Schema/API/config:** additive queue/outbox/lease/DLQ/reprocess fields,
   constraints/indexes/history trigger; new collection-scoped evidence reprocess
   route; persistent NATS volume and queue settings. No retained application.
5. **Tests:** offline forensic suite 124 passed/one environment-gated skip of 125
   in 28.815 s; live NATS full runs 8.018/9.381 s; Python 32/32 in 1.122 s;
   Python compile, Go vet and Compose passed; full disposable database chain
   passed. Focused LocalAI package remains a resource-timeout gate, not a pass.
6. **Live runtime:** no retained build/recreate/restart, migration, secret, grant,
   model or evidence-state change. Disposable TimescaleDB/NATS resources only;
   all were removed.
7. **Dataset/evidence:** synthetic migration and queue fixtures only. Supplied
   real Pakistan CDR and retained registry were untouched.
8. **Models:** Qwen chat/embedding baseline unchanged; no model call, download,
   install, promotion or parameter change.
9. **Accuracy/retrieval:** queue lifecycle correctness only; Phase 2 20/20,
   18/18, 27/27 and exact-row baselines remain unchanged.
10. **Latency/memory:** real NATS package runs 8.018/9.381 s; offline package
    28.815 s; Python 1.122 s. No new live p95/RAM claim. LocalAI linking exceeded
    300 s on the constrained laptop.
11. **Failures/fallbacks:** fixed transient enum misuse, non-atomic job insert,
    tenant-setting transaction lifetime, heartbeat error masking, nullable scan,
    reprocess generation race, Windows Ginkgo flag parsing and Docker random-port
    restart handling. No unresolved core queue failure remains.
12. **Security/provenance:** tenant context per transaction, exact collection and
    case binding, immutable evidence/job/reprocess identity, monotonic attempts,
    terminal immutability and bounded hash-referenced DLQ diagnostics.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
    action.
14. **Rollback:** source is undeployed. Migration 009 rollback is explicitly
    destructive/gated; retained activation requires a fresh verified backup.
15. **Next action/approval:** run LocalAI focus in CI/high-memory, then separately
    approve backup, migrations 008/009, runtime role/grants, persistent NATS,
    secrets and targeted activation. After activation acceptance, begin Phase 4.
    Detailed report:
    `reports/forensic-phase3-queue-lifecycle-completion-20260729.md`.

Current status: Phase 2A and 2B are complete at the documented boundary. All 20
evidence families have executable versioned fixtures; supported structured
families have exact live row accounting; unsupported deeper analysis remains
explicitly pending/manual. Phase 3 source development and isolated acceptance are
complete across migration/control plane, authenticated scope/RLS,
content-addressed retention and durable queue/reprocessing. Retained migration 008
and migration 009 are now applied and officially verified; the guarded legacy
failure disposition preserved 34 completed and 7 dead-letter jobs with zero queue
blockers. Runtime roles/grants, API, worker and proxy source remain undeployed.
Phase 4A through 4E structured-data v1 source development now provides a 20-family
collection-aware capability contract, pre-runtime unsupported-operation guard,
Records Intelligence capability/reprocess UI, accepted streaming TSV and bounded
read-only XLSX ingestion with complete workbook/cell provenance, conservative
per-sheet typed mapping with generic preservation, plus exact source-cited
correlation over all eight canonical structured record families. The first UI
simplification makes the default workflow analyst-first and progressively reveals
advanced, evidence and management controls. A privacy-safe offline audit proves
the supplied 3,931-row CDR CSV and a versioned seven-family typed matrix without
uploading or emitting identifiers; typed IPDR now validates IPv4/IPv6, NAT, ports,
bytes, time/duration and domains.
The deployment gate now consists of higher-memory LocalAI validation and the
remaining explicitly approved Phase 3/4 runtime activation: non-owner role/grants,
auth/JetStream settings, rollback-tagged build/recreate and synthetic live smokes.
Phase 5 source work may begin without claiming that undeployed Phase 3/4 runtime
code is live.

Current runtime boundary after retained migrations 008/009: `nexusai-api` remains on
the earlier universal image and was not rebuilt or restarted by activation work.
Forensic PostgreSQL and persistent NATS are healthy; forensic API and worker remain
stopped. The evidence control plane and queue lifecycle schema are live and
verified, while runtime grants, sidecar deployment and live queue processing remain
gated. The safe offline demo remains the only approved/current demonstration until
activation and rebuild smoke pass.

### 2026-07-29 — Phase 4A capability-aware query and streaming TSV slice

1. **Objective:** make arbitrary analyst questions capability-aware and begin
   Phase 4 format depth without pretending pending OCR/STT/media adapters exist.
2. **Assumptions verified:** exact facts remain SQL-only; KB synthesis is cited;
   collection absence is not a negative real-world finding; no retained rebuild
   or model change is authorized by source work.
3. **Files changed:** sidecar capability API/tests and query guard; LocalAI proxy,
   route and instructions; React API/UI; worker TSV dialect/encoding source and
   tests; classifier, modality/model catalogs, reports and checkpoint/team brief.
4. **Schema/API/config:** new read-only collection-scoped capability endpoint;
   hybrid response gains capability assessment; TSV queues to the existing worker.
   No database schema or active configuration was applied.
5. **Tests:** Python worker/adapters 33/33 in 1.117 s; full forensic Go package
   passed in 15.035 s (27.8 s wall); six focused capability contracts passed;
   React ESLint zero errors/six existing warnings; compile/JSON/diff checks pass.
6. **Live runtime:** no build/recreate/restart, migration, retained write, queue
   activation or live endpoint claim. LocalAI wrapper remains CI/high-memory gate.
7. **Dataset/evidence:** synthetic UTF-16 Pakistan-style TSV only; Urdu, duplicate
   headers, extra columns, leading zeros and exact decimals preserved. Supplied
   real Pakistan CDR remained untouched.
8. **Models:** active Qwen3 4B Instruct 2507 and Qwen3 Embedding 0.6B unchanged;
   primary-source shortlist recorded; zero download/install/call/promotion.
9. **Accuracy/retrieval:** capability matrix remains exactly 20 families; pending
   direct processing is blocked before DB/KB/LLM; fixed TSV raw values match.
10. **Latency/memory:** focused capability guard reports zero DB/KB/LLM latency;
    no new live model/RAM claim. TSV full ingestion is streaming.
11. **Failures/fallbacks:** Go cache sandbox permission was required; a helper
    name collision and one test indentation error were corrected. No unresolved
    Phase 4A source failure remains; XLSX/deeper columnar stay pending.
12. **Security/provenance:** existing collection authorization/trusted scope is
    reused; raw values and source dialect are explicit; immutable reprocessing UI
    preserves earlier generations; unsupported processing cannot be synthesized.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
    action.
14. **Rollback:** source-only files can be human-reviewed before activation; no
    retained state needs rollback.
15. **Next action/approval:** implement XLSX source adapter/goldens and deepen
    cross-family structured query tests. Separate approval remains required for
    rebuild/deployment, migrations, model download/promotion and Git publication.
    Detailed reports: `reports/forensic-phase4a-capability-tsv-acceptance-20260729.md`
    and `reports/forensic-model-candidate-shortlist-20260729.md`.

### 2026-07-29 — Phase 4B bounded read-only XLSX slice

1. **Objective:** turn real `.xlsx` evidence into queryable normalized rows while
   preserving workbook semantics and never executing active content.
2. **Assumptions verified:** OOXML packages are untrusted; formula/cache/raw
   representations must coexist; mixed-sheet auto-detection must not force one
   typed schema over unrelated sheets.
3. **Files changed:** new XLSX reader/tests/versioned Pakistan golden; worker
   routing/metadata; classifier/capability tests; modality matrix, docs and reports.
4. **Schema/API/config:** `.xlsx` queues to the existing worker and accepted row
   queries pass the capability guard. No schema, endpoint or live config change.
5. **Tests:** focused XLSX 8/8 in 0.249 s; combined Python 41/41 in 1.277 s;
   complete forensic Go package passed in 4.303 s; compile and JSON checks pass.
6. **Live runtime:** no build/recreate/restart, migration, retained write, queue
   activation or live upload; source is not claimed deployed.
7. **Dataset/evidence:** synthetic PKR/Urdu two-sheet workbook only; four data
   rows with hidden sheet/row/column, merge, formulas, error and inert external link.
8. **Models:** active Qwen chat/embedding pair unchanged; no model call/download/
   install/promotion.
9. **Accuracy/retrieval:** exact leading zero, decimal token, raw date serial,
   1904 date, PKR, Urdu, formula text, cached value and error state match golden.
10. **Latency/memory:** no live claim; ZIP/XML sizes/counts and samples are
    bounded, full structural inventory is complete and row output streams per sheet.
11. **Failures/fallbacks:** macros, DTD/entities, traversal and invalid shared
    strings reject; Excel serial 60 is review-flagged; no active content runs.
12. **Security/provenance:** raw workbook unchanged; sheet/row/cell, hidden state,
    style, formula/cache/error and workbook metadata survive normalization.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub action.
14. **Rollback:** source-only; retained state requires no rollback.
15. **Next action/approval:** Phase 4C exact cross-family query depth and
    per-sheet typed mapping, then ODS/columnar evaluation. Rebuild/deploy,
    migrations, model changes, real sensitive ingest and Git remain gated.
    Detailed report: `reports/forensic-phase4b-xlsx-acceptance-20260729.md`.

### 2026-07-29 — Phase 4C exact cross-family correlation slice

1. **Objective:** correlate bounded analyst-supplied identifiers across all
   eight canonical structured record families and return exact, source-cited
   matches and related entities without model-generated facts.
2. **Assumptions verified:** canonical records are the source of exact facts;
   identifier normalization must be narrow and disclosed; nested response arrays
   must not inflate answer counts; legacy exploratory relationships remain separate.
3. **Files changed:** query planner/SQL/aggregation, Phase 4C Ginkgo tests,
   versioned Pakistan-oriented golden, capability/agent policy text, docs,
   acceptance report, team-lead brief and this checkpoint.
4. **Schema/API/config:** no schema, migration, endpoint or runtime-config change;
   one new advertised query template and explicit aliases use the existing
   `/query/hybrid` contract.
5. **Tests:** six focused specs passed in 20.838 s; final full forensic API
   package passed in 11.309 s; focused forensic agent-tools suite passed in 31.065 s
   package time (117.4 s wall); API+agents `go vet` passed in 15.9 s; final JSON
   and whitespace checks passed.
6. **Live runtime:** no rebuild/recreate/restart, migration, retained query/write,
   upload, queue activity or deployment; SQL was contract-tested in source only.
7. **Dataset/evidence:** synthetic/de-identified version 1.0.0 golden with four
   planner cases, four occurrences and three relation aggregates; no real data.
8. **Models:** active Qwen chat/embedding pair unchanged; no model call, download,
   installation, benchmark or promotion.
9. **Accuracy/retrieval:** planner 4/4, aggregate 3/3 and citations 4/4; exact SQL
   assertions enforce canonical scope, lineage fields and no `ILIKE` substring.
10. **Latency/memory:** no live claim; maximum eight targets, 1,000 scanned
    related occurrences and five retained citations per related entity.
11. **Failures/fallbacks:** missing comparison targets clarify; more than eight
    targets reject before DB; short phone compaction is disabled; caps warn that
    displayed relationships may not be exhaustive.
12. **Security/provenance:** tenant/collection and optional date/type scopes are
    parameterized; matches retain evidence/version/record/file/row/hash/time and
    available XLSX sheet/row locators.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
    action.
14. **Rollback:** source-only; no retained state or deployed image needs rollback.
15. **Next action/approval:** Phase 4D per-sheet typed XLSX mapping and messy
    multi-provider schema/timezone/duplicate goldens, then family-specific
    finance/log analytics. Rebuild/deploy, migrations, model changes, sensitive
    ingest and Git publication remain separately gated. Detailed report:
    `reports/forensic-phase4c-cross-family-correlation-acceptance-20260729.md`.

### 2026-07-29 — Phase 4D per-sheet XLSX mapping and UI simplification slice

1. **Objective:** safely type mixed XLSX worksheets independently and reduce the
   Records Intelligence page's default cognitive load.
2. **Assumptions verified:** workbook-wide union headers are unsafe for typed
   routing; ambiguous/non-matching sheets must survive; advanced and admin
   controls should not compete with the primary analyst question.
3. **Files changed:** upload API/test; XLSX reader; worker/tests; versioned mixed
   Pakistan-oriented golden; capability/UI/docs; acceptance report, team brief,
   and this checkpoint.
4. **Schema/API/config:** no schema, endpoint, migration, or active config change;
   upload metadata now retains auto versus explicit record-type intent and worker
   rows retain `per_sheet_schema_v1` decisions.
5. **Tests:** focused XLSX 10/10 in 0.826 s; combined Python 43/43 in 0.677 s;
   final full forensic Go package 31.249 s after capability/matrix sync; Python
   compile, JSON and diff checks passed;
   page ESLint zero errors/four warnings; Vite build 2.52 s.
6. **Live runtime:** no rebuild/recreate/restart, migration, retained write,
   upload, queue activity, or deployment. Static UI render only; preview stopped.
7. **Dataset/evidence:** synthetic/de-identified mixed workbook: CDR, ANPR, PKR
   transaction and case notes; no supplied real evidence modified or ingested.
8. **Models:** configured Qwen baseline unchanged; no call, search, download,
   installation, benchmark, or promotion.
9. **Accuracy/retrieval:** auto mapping agrees 4/4; exact values and sheet/row
   locators agree; explicit transaction types only its matching sheet and retains
   three non-matches generically with review required.
10. **Latency/memory:** no deployed performance claim; existing XLSX resource
    bounds/streaming retained. Build timing is diagnostic only.
11. **Failures/fallbacks:** test setup frozen-dataclass and money-scale fixture
    issues were corrected; full-repo React lint retains seven unrelated errors
    and 571 warnings; no gate weakened.
12. **Security/provenance:** unique per-sheet match is required for promotion;
    ambiguity falls back safely; every normalized row retains sheet/header/source
    mapping provenance; active workbook content remains prohibited.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
    action.
14. **Rollback:** source-only; no retained state or image needs rollback.
15. **Next action/approval:** Phase 4E multi-provider structured-family depth and
    clearer analyst/evidence/ingestion/admin workspaces. Deployment sequence is a
    read-only activation audit, separately approved backup/migrations 008/009,
    then a bounded rebuild/smoke/rollback. Models, sensitive evidence and Git stay
    separately gated. Detailed report:
    `reports/forensic-phase4d-xlsx-sheet-mapping-ui-acceptance-20260729.md`.

### 2026-07-29 — Phase 4E CSV/IPDR/demo readiness and structured v1 completion

1. **Objective:** prove the supplied real-format CDR CSV without ingesting or
   exposing it, add missing typed IPDR depth, and make Phase 4 demonstrable.
2. **Assumptions verified:** CSV remains a primary source; timezone is declared;
   complete accounting includes duplicates/rejects; provider audit output must
   not contain identifiers.
3. **Files changed:** worker audit/performance/IPDR; tests and synthetic fixture/
   golden; capability matrix/docs; safe demo script, team-lead guide/report and
   this checkpoint.
4. **Schema/API/config:** no schema, endpoint, migration or active config change;
   new offline `audit-source` CLI only.
5. **Tests:** final Python 50/50 in 3.116 s; full forensic Go package 30.520 s;
   React production build 2.70 s; compile, JSON and diff checks passed; demo 15 s.
6. **Live runtime:** no build/recreate/restart, migration, upload, queue, retained
   DB write, model call or live query.
7. **Dataset/evidence:** real supplied CSV read-only: 828,259 bytes, 3,931 total/
   accepted, zero rejects, 297 exact duplicates, 3,634 unique, zero overflow.
   Synthetic IPDR: six total, four accepted, two rejects, one duplicate.
8. **Models:** configured Qwen baseline unchanged; no search/download/call/
   benchmark/install/promotion.
9. **Accuracy/retrieval:** real CSV auto-routes CDR with 100% row accounting;
   seven-family typed matrix matches all expected totals; IPDR exact IPv4/IPv6,
   NAT, port, byte, duration, domain and timezone checks pass.
10. **Latency/memory:** real CSV audit improved from measured 35–55 s to 2.94 s
    by caching immutable timezone resolution and normalizing CDR aliases once;
    exact output remained unchanged.
11. **Failures/fallbacks:** Windows direct script execution policy blocked; the
    documented process-scoped bypass passed without policy mutation. IPDR stable
    rejection category golden was synchronized.
12. **Security/provenance:** source SHA/schema/accounting emitted; raw rows,
    identifiers and rejected values not emitted; no DNS resolution; real file
    read only and never copied to fixtures.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
    action.
14. **Rollback:** source-only; temporary profiler file removed; no retained state
    needs rollback.
15. **Next action/approval:** Phase 4 structured-data v1 is source-complete. Run
    activation readiness, then separately approve backup, migrations 008/009,
    runtime settings and bounded rebuild before a live UI/API demo. Begin Phase 5
    source work afterward; private vendor packs remain additive compatibility.
    Detailed report: `reports/forensic-phase4e-csv-ipdr-demo-acceptance-20260729.md`.

### 2026-07-29 - Phase 3 activation preparation and recovery point

1. **Objective:** prepare retained infrastructure for a separately approved
   Phase 3 activation without applying schema or starting processors.
2. **Assumptions verified:** only PostgreSQL/NATS were required; API/worker had
   to remain stopped; 009 depends on installed 008 and a drained legacy queue.
3. **Files changed:** activation report, this checkpoint, and one ignored custom
   backup. No application, migration, adapter, UI, model or config source changed.
4. **Schema/API/config:** none applied; existing Compose started PostgreSQL/NATS.
5. **Checks:** both infrastructure services healthy; 008 read-only preflight
   passed; 009 exited 3 at its expected 008 dependency gate; aggregate inventory
   found 34 completed and 7 failed jobs out of 41.
6. **Live runtime:** forensic API and worker stayed stopped; no upload, queue
   consumption, rebuild, restart of LocalAI, or live query claim.
7. **Dataset/evidence:** aggregate-only 110 evidence, 104 linked KB assets and 41
   linked jobs; no identifier/raw value output and no retained row changed.
8. **Models:** configured Qwen baseline unchanged; no model activity.
9. **Accuracy/retrieval:** not applicable; exact database compatibility counts
   only.
10. **Latency/memory:** no application/model claim; bounded health check passed.
11. **Failures/fallbacks:** 009 remains blocked by absent 008 and seven failed
    legacy jobs. Full-dump TimescaleDB circular-FK warning is disclosed; archive
    round-trip hash, 396-line catalog and full read passed.
12. **Security/provenance:** read-only aggregate inspection; processors stopped;
    backup retained inside the sensitive project backup area.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
    action.
14. **Rollback:** 13,018,415-byte custom dump at
    `.phase2-backups/phase3-activation-preparation-pre-migration-20260729T155543.dump`;
    SHA-256 `0760ef0613a074d2ace96639f0a3d85fb2019f93339cc79732c17ab103b47cea`;
    round-trip match and full archive read passed.
15. **Next action/approval:** separately approve apply+verify 008, then an
    audit-preserving recovery decision for seven failed jobs, rerun 009 preflight,
    and separately approve apply+verify 009. API/worker/runtime grants, rebuild,
    smoke, models, sensitive ingestion and Git remain gated. Detailed report:
    `reports/forensic-phase3-activation-preparation-20260729.md`.

### 2026-07-29 - Retained migration 008 activation

1. **Objective:** apply and verify the Phase 3 evidence control plane against the
   retained registry from the verified recovery point.
2. **Assumptions verified:** dump/hash and 008 source hash matched; PostgreSQL/NATS
   healthy; API/worker stopped; disposable fixture/smoke scripts prohibited here.
3. **Files changed:** retained-activation report and this checkpoint only; database
   schema/state changed through the approved migration.
4. **Schema/API/config:** 008 added seven Phase 3 relations, current-version
   pointers, constraints/indexes, synchronization/immutability functions/triggers,
   and seven tenant policies. No API/config/009 change.
5. **Checks:** final 110/104/41 preflight passed; migration exited zero at COMMIT;
   official verify and aggregate integrity transaction passed.
6. **Live runtime:** PostgreSQL/NATS healthy; API/worker stayed stopped; LocalAI
   was not rebuilt or restarted.
7. **Dataset/evidence:** 110 storage objects, 110 versions, 214 source links, 41
   runs, 41 processing events, zero derived artifacts and 110 custody events;
   job outcomes remain 34 completed and seven failed.
8. **Models:** configured Qwen baseline unchanged; no model activity.
9. **Accuracy/retrieval:** not applicable; exact lineage/backfill integrity only.
10. **Latency/memory:** forward command approximately two seconds; verifier under
    one second after execution began; no production benchmark claim.
11. **Failures/fallbacks:** no SQL failure; expected first-application notices
    only. Seven failed jobs remain the intentional 009 blocker.
12. **Security/provenance:** 7/7 Phase 3 tables have RLS and exactly seven policies;
    zero missing current versions or invalid custody hashes; aggregate output only.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
    action.
14. **Rollback:** pre-008 custom dump remains 13,018,415 bytes with SHA-256
    `0760ef0613a074d2ace96639f0a3d85fb2019f93339cc79732c17ab103b47cea`;
    destructive rollback was not run.
15. **Next action/approval:** separately approve privacy-safe failed-job diagnosis
    and an audit-preserving recovery disposition, then rerun 009 preflight. Applying
    009, service activation, rebuild, models, sensitive ingestion and Git remain
    gated. Detailed report:
    `reports/forensic-phase3-migration008-retained-activation-20260729.md`.

### 2026-07-29 - Legacy failed-job recovery diagnosis and disposition

1. **Objective:** diagnose the seven retained 009 blockers and prepare a truthful,
   audit-preserving disposition without changing state.
2. **Assumptions verified:** 009 rejects failed jobs; dead-letter is the truthful
   legacy terminal state; originals must never be deleted/reset/mislabeled success.
3. **Files changed:** recovery-disposition report and checkpoint only; no code,
   migration source, configuration or retained row changed.
4. **Schema/API/config:** none; read-only transactions only; 009 preflight/forward
   not run; API/worker not started.
5. **Checks:** seven unique jobs/evidence, one tenant/two anonymous scopes, all
   attempt 1/5 with terminal timestamps and two diagnostic signatures.
6. **Live runtime:** database reads only; no producer, consumer, retry or file
   processing activity.
7. **Dataset/evidence:** CDR 1, ANPR 2, IPDR 2, access-log 2; zero row counters,
   canonical/generic/CDR/KB/audit/error outputs; no sibling jobs or KB assets;
   seven legacy unverified storage objects; evidence states 1 failed/6 stale queued.
8. **Models:** no model activity.
9. **Accuracy/retrieval:** not applicable; exact lineage/state audit only.
10. **Latency/memory:** bounded aggregate queries; no production benchmark claim.
11. **Failures/fallbacks:** first JSON-array query hit legacy scalar JSON and its
    read transaction aborted; shape-safe rerun passed. Physical legacy bytes remain
    unverified and cannot be reprocessed yet.
12. **Security/provenance:** only aggregate/anonymous/signature output; no raw
    evidence, identifiers, filenames, collection names or diagnostic text emitted.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
    action.
14. **Rollback/recovery:** recommended guarded transaction preserves original jobs,
    errors/attempts/times and appends 7 audit + 7 processing transitions + 6 custody
    corrections; it must follow a fresh post-008 backup and separate approval.
15. **Next action/approval:** approve post-008 backup plus reviewed failed-to-
    dead-letter/evidence queued-to-failed recovery. Then rerun official 008 verify
    and 009 read-only preflight and stop again before 009 application. Detailed
    report:
    `reports/forensic-phase3-legacy-failed-job-recovery-disposition-20260729.md`.

### 2026-07-29 - Phase 3 retained queue activation completion

1. **Objective:** complete retained Phase 3 database activation through guarded
   failure preservation, migration 009 and independent recovery points.
2. **Assumptions verified:** API/worker drained/stopped; seven failures had zero
   outputs/siblings; dead-letter is truthful; runtime deployment remains separate.
3. **Files changed:** one-time gated recovery SQL, DB/design docs, reports, team
   brief and checkpoint; two ignored custom dumps. No API/worker/UI/model source.
4. **Schema/API/config:** migration 009 adds message/outbox/lease/retry/DLQ/ack/
   reprocess fields, constraints/indexes and history trigger. No runtime config.
5. **Checks:** post-008 backup verified; recovery committed; 008 verify and 009
   preflight passed; 009 committed; official 009 and final 008 verifies passed;
   final aggregate lifecycle snapshot passed.
6. **Live runtime:** PostgreSQL/NATS healthy; API/worker stopped; LocalAI not
   rebuilt/restarted.
7. **Dataset/evidence:** 110 evidence/storage/version, 214 source links, 41 jobs/
   runs (34 completed/succeeded, 7 dead-letter), 48 processing events, 116 custody
   events, 0 derived artifacts and 7 recovery audit rows.
8. **Models:** configured Qwen baseline unchanged; no model activity.
9. **Accuracy/retrieval:** not applicable; exact lineage/queue invariants only.
10. **Latency/memory:** bounded database commands; no production benchmark claim.
11. **Failures/fallbacks:** no activation failure; full-dump Timescale warning
    disclosed, while every backup passed round-trip hash/catalog/full read. Runtime
    security/build/live-smoke gates remain open.
12. **Security/provenance:** original failure identity/errors/attempts/times
    preserved; audit/processing/custody history appended; 41/41 stable queue IDs,
    zero leases, 3 constraints and 1 history trigger.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
    action.
14. **Rollback:** post-008/pre-recovery dump 13,161,166 bytes SHA-256
    `15d2188c680dae310baacba609cdb44479db388b76fd89ecdd05b3b6ea96ae10`;
    post-009 dump 13,169,864 bytes SHA-256
    `442a9d9f2d55eec958a33968d3fb54d7e4732f4574e16af69f8de22b5db1fc41`;
    both round-trip/catalog/full-read verified; pre-008 dump also retained.
15. **Next action/approval:** Phase 3 retained database/control-plane/queue
    migration is complete. Separately approve higher-memory LocalAI validation,
    non-owner role/grants and RLS proof, auth/JetStream settings, rollback-tagged
    API/worker deployment and synthetic success/retry/DLQ/redelivery/reprocess
    smokes. Models, sensitive ingestion and Git remain gated. Detailed report:
    `reports/forensic-phase3-retained-queue-activation-completion-20260729.md`.

### 2026-07-30 - Phase 3 non-owner runtime source preparation

1. **Objective:** prepare the complete operator-controlled runtime activation
   path while keeping long builds and retained mutations out of the Codex window.
2. **Assumptions verified:** 008/009 are live and verified; API/worker remain a
   stopped gate; the user requested local PowerShell execution and visible logs.
3. **Files changed:** worker/test; migration 010 preflight/forward/verify; runtime
   grants/RLS proof; auth/runtime Compose overrides/env example; authenticated
   smoke support; DB docs, activation runbook/report and this checkpoint.
4. **Schema/API/config:** source-only 010 closes three Phase 1 tenant-policy gaps
   and makes two aggregate views security-invoker; runtime override requires auth
   and the fixed non-owner `forensic_runtime` DB identity. Nothing applied live.
5. **Tests:** final four-module Python activation suite 49/49 passed in 1.164 s;
   operator Go forensic package passed in 18.178 s with repository-local cache;
   both Compose merges, PowerShell parse and selected diff check passed. Windows
   PowerShell 5 initially promoted expected diagnostic stderr to a terminating
   error; the exit-code-authoritative runner passed without weakening a gate.
6. **Live runtime:** no build, recreate, restart, role, migration, upload, queue
   processing, model call or retained DB change. Gate 2 created the ignored local
   secret file; final values are ACL-protected and were not printed.
7. **Dataset/evidence:** none read or changed. First activation write is restricted
   to the five-row synthetic Pakistan CDR fixture in a neutral collection; real
   supplied CDR and `records-demo-verified` are prohibited from the write smoke.
8. **Models:** Qwen baseline unchanged; no model activity.
9. **Accuracy/retrieval:** no new metric claim; existing structured v1 acceptance
   remains unchanged.
10. **Latency/memory:** operator preflight found 3.8 GiB free RAM and 1,774.09 GiB
    disk; final Python 1.164 s and Go package 18.178 s; focused LocalAI passed in
    43.827 s with 6.29 GiB free. Gate 7 later observed 4.89 GiB and stopped before
    building because the 6 GiB floor remains enforced; post-reboot retries reached
    rollback discovery but performed no build.
11. **Failures/fallbacks:** corrected worker owner-DDL, three missing policies and
    owner-context views and PowerShell 5 stderr handling. Gates 1, 2, 5 and 6 are
    complete; rebuild/deploy/live acceptance remains open. Gate 7 stopped at the
    memory/discovery guards; four duplicate API/worker rollback tag sets are
    harmless and no image layers or containers changed;
    failure-path injection must use a disposable stack, never retained corruption.
12. **Security/provenance:** runtime role is NOSUPERUSER/NOBYPASSRLS without
    create/delete/truncate; secrets stay in an ACL-restricted ignored file; RLS
    probe is synthetic/rolled back; Compose expansion is quiet to avoid leakage.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
    action.
14. **Rollback:** no service/database change. Preserve the ignored runtime secret
    file; the operator runbook later creates a verified pre-010 dump and rollback
    image tags; existing verified post-009 dump remains valid.
15. **Next action/approval:** reboot/close nonessential applications, keep Docker
    available, and rerun only the rollback-tagged complete rebuild in Gate 7;
    Gate 6 is persisted and must not be repeated. Continue the runbook and
    return only sanitized summaries. Do not mark Phase 3 runtime complete before
    migration/role/auth/JetStream, synthetic success and disposable retry/DLQ/
    redelivery/reprocess acceptance. Detailed report:
    `reports/forensic-phase3-runtime-source-preparation-20260730.md`.

### 2026-07-30 - Gate 7 complete rebuild and guarded activation runners

1. **Objective:** finish the approved complete API/worker/LocalAI build and make
   the remaining retained activation steps safe, ordered and resumable.
2. **Assumptions verified:** Gate 6 passed; retained LocalAI was healthy and
   rollback-tagged; secrets remain ignored/ACL-protected; named volumes and real
   evidence remain outside synthetic acceptance.
3. **Files changed:** PowerShell common guard and one-command Gates 3, 4, 8, 9
   and 10; corrected smoke-header initialization; runbook and checkpoint.
4. **Schema/API/config:** no retained schema/runtime mutation in this entry. Gate
   4 applies 010/non-owner grants only after Gate 3 backup verification; Gate 8
   performs image-only cutover with named volumes preserved.
5. **Tests:** Gate 7 passed: sidecars 18.4 s and full LocalAI/React UI 515.5 s.
   New PowerShell scripts parse under Windows PowerShell; diff check is clean.
6. **Live runtime:** images built, not deployed: API `sha256:ced34f28...`, worker
   `sha256:8284695b...`, LocalAI `sha256:07283a2f...`. Gates 3-4/8-10 remain.
7. **Dataset/evidence:** unchanged. Gate 10 is hard-bound to the five-row
   synthetic Pakistan CDR and excludes the real sample and retained demo scope.
8. **Models:** Qwen baseline/model volumes unchanged; no model activity.
9. **Accuracy/retrieval:** no new claim; Gate 10 will enforce golden accounting
   and deterministic authenticated query routing.
10. **Latency/memory:** successful build timings recorded above; no new runtime
    latency claim.
11. **Failures/fallbacks:** memory and rollback-discovery guards stopped safely;
    explicit project discovery fixed Gate 7. Gate 8 preserves/restarts prior
    LocalAI on detected cutover failure.
12. **Security/provenance:** no secret logging; backup hash gates migration; RLS
    proof rolls back; 401, wrong-tenant 403 and idempotency checks are mandatory.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
    action.
14. **Rollback:** canonical image tags are in
    `reports/runtime-activation-20260730/rollback-images.txt`; Gate 3 creates the
    verified pre-010 host/container dump.
15. **Next action/approval:** activation/rebuild is already approved. Execute
    Gates 3, 4, 8, 9 and 10 in order and return only sanitized PASS summaries.
    Then finish isolated failure-path acceptance before declaring runtime Phase 3
    complete.

### 2026-07-30 - Phase 3 retained runtime activation complete

1. **Objective:** close Phase 3 by activating and proving authentication,
   non-owner RLS, persistent queue delivery, retries/DLQ, reprocessing and
   redelivery against the retained runtime.
2. **Assumptions verified:** pre-010 backup/hash, rollback images, synthetic-only
   write scope and volume preservation all held.
3. **Files changed:** runtime/API/worker fixes, tests, Gates 8b/8c/11, completion
   report and checkpoint. No Git publication.
4. **Schema/API/config:** 010 live; `forensic_runtime` all privileged flags false;
   auth required; transaction-scoped tenant context; file-backed explicit-ack
   JetStream with compatible durable queue identity.
5. **Tests:** Python 51/51; forensic Go package PASS (14.272 s); LocalAI focus
   PASS (43.827 s); build, Compose, PowerShell and whitespace gates PASS.
6. **Live runtime:** PostgreSQL/NATS healthy; API running/health PASS; worker
   healthy/metrics PASS; LocalAI healthy/ready PASS.
7. **Dataset/evidence:** one synthetic evidence, 5 total/4 unique/1 duplicate/0
   rejected; one 5/5 dead-letter parent plus one completed generation-1 linked
   reprocess. Real supplied CDR and retained demo evidence unchanged.
8. **Models:** established Qwen models/configuration unchanged; no model activity.
9. **Accuracy/retrieval:** exact accounting, idempotency, seven deterministic
   queries and 20-family capability contract passed; no new model claim.
10. **Latency/memory:** sidecars 18.4 s, LocalAI/UI 515.5 s, final Go 14.272 s;
    no production load percentile claim.
11. **Failures/fallbacks:** fixed worker XLSX packaging, JetStream identity,
    non-owner transactional tenant context, CDR canonical SQL and mixed legacy
    metadata. Failure history was preserved and reprocessed immutably.
12. **Security/provenance:** 401 and cross-tenant 403 passed; secrets not printed;
    Gate 11 synchronously acknowledged completed redelivery with zero duplicate
    effects and an empty ingest stream.
13. **Git:** unstaged, uncommitted and unpushed; no GitHub action.
14. **Rollback:** verified 13,169,864-byte pre-010 dump SHA-256
    `f690323d0c2b16f6f4df12bc0b72891a26b053c23f836fa47287181880e543c5`;
    image/container rollback artifacts retained; volumes preserved.
15. **Next action:** Phase 3 is complete. Begin Phase 4 retained structured-data
    operational depth and UI workflow acceptance, followed by separately gated
    document/image/audio/video model pipelines. Detailed report:
    `reports/forensic-phase3-runtime-activation-completion-20260730.md`.

### 2026-07-30 - Phase 4 structured runtime deployment and final closure gate

1. **Objective:** deploy and prove operational structured depth plus the
   analyst-first Records UI against the retained Phase 3 runtime.
2. **Assumptions verified:** Phase 3 remained healthy; only synthetic/de-identified
   retained writes were authorized; the real supplied CDR and existing verified
   demo scope remained untouched.
3. **Files changed:** API classification/capability/evidence-detail source and
   tests; LocalAI proxy identity/KB forwarding; React upload workflow; TSV/XLSX
   fixture/generator; Gates 12/13/13b; Phase 4 report and current team-lead brief.
4. **Schema/API/config:** no database migration. Added explicit local proxy
   identity, record-type forwarding, evidence `view=accounting`, preview opt-out,
   and canonical evidence-extension capability materialization.
5. **Tests:** Python 51/51; focused LocalAI forwarding PASS 13.934 s; forensic Go
   package repeatedly PASS after repairs; React build PASS, 656 modules/1.81 s;
   PowerShell parsers PASS.
6. **Live runtime:** Gate 13 deployed API and LocalAI/UI; API build 34.3 s,
   LocalAI 474.9 s, both ready; Gate 13b API repair passed. Final canonical
   extension fix is source-tested but not deployed because privileged Docker
   execution reached the product usage limit.
7. **Dataset/evidence:** clean v2 scope has nine synthetic file paths with golden
   68 total/39 accepted unique/8 duplicate/21 rejected accounting. The first
   incomplete TSV registration remains preserved as audit history. No real
   evidence uploaded.
8. **Models:** unchanged active CPU baseline: `qwen_qwen3-4b-instruct-2507`
   Q8_0/8 threads and `qwen3-embedding-0.6b` Q8_0/1024d/8 threads; no new model
   download, promotion or model-derived exact claim.
9. **Accuracy/retrieval:** all nine per-file accounting checks passed before
   Gate 12 reached capability assertion. Capability false-zero defect was fixed;
   final queryability, cross-family and source-audit closure awaits deploy/rerun.
10. **Latency/memory:** stored chat benchmark 6/6, average 8565.53 ms, p50
    4781.66 ms, p95 12843.57 ms; deployment timings above; no production SLA.
11. **Failures/fallbacks:** live acceptance found blank no-auth proxy identity,
    UI/sidecar upload drift, explicit TSV classification drift, PowerShell
    case-sensitive JSON incompatibility and a silently swallowed capability SQL
    error. All are source-fixed/tested; only the last API image deployment is open.
12. **Security/provenance:** local proxy identity is explicit/accountable and
    fails closed when absent; accounting view excludes raw previews and IDs;
    secrets/raw rows were not printed; rollback images/volumes preserved.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
    action.
14. **Rollback:** Phase 4 API/LocalAI rollback tags remain; Gate 13 preserved
    named volumes; the verified Phase 3 pre-010 database backup remains valid.
15. **Next action:** locally run `repair_forensic_phase4_api_gate13b.ps1`, then
    `smoke_forensic_phase4_structured_gate12.ps1`. Require both PASS summaries
    before declaring Phase 4 closed or beginning the separately gated document/
    OCR model phase. Detailed report:
    `reports/forensic-phase4-structured-runtime-status-20260730.md`.

## Required phase report format

At each meaningful checkpoint, append a compact report containing:

1. Objective.
2. Assumptions verified.
3. Files changed.
4. Schema/API/config changes.
5. Tests and exact results.
6. Live-runtime checks.
7. Dataset/evidence counts.
8. Model IDs, revisions, and parameters.
9. Accuracy/retrieval metrics.
10. Latency and memory metrics.
11. Failures, fallbacks, and unresolved blockers.
12. Security, tenant-isolation, and provenance impact.
13. Git state (unstaged/staged/committed/pushed).
14. Rollback or recovery point.
15. Next action and approval required.

### 2026-07-30 - Enterprise Agent Chat production rebuild and timeout handoff

1. **Objective:** replace the temporary port-3000 preview with the production
   enterprise forensic Agent Chat on port 8080 and prove exact/model routes.
2. **Assumptions verified:** retained volumes and rollback images existed; no
   real-evidence write or Git publication was authorized.
3. **Files changed:** enterprise Agent Chat and Records handoff UI, direct
   forensic routing/identity/timeout source and tests, records-only explicit
   synthesis, bounded Docker build configuration, hardened Gate 13 and runtime
   status report.
4. **Schema/API/config:** no schema migration. Explicit no-auth local proxy
   identity now reaches Agent forensic tools; exact queries omit synthesis;
   explicit explanation requests carry the configured Qwen model.
5. **Tests:** focused Agent identity/routing/synthesis PASS; React production
   build PASS (656 modules); focused forensic API synthesis PASS; PowerShell
   gate parser and Compose configuration PASS.
6. **Live runtime:** Gate 13 PASS with 5.5 s API and 454.7 s bounded LocalAI
   build; Gate 13b PASS; rollback recovery subsequently health-checked at
   LocalAI `/readyz` 200 and forensic API `/healthz` 200.
7. **Dataset/evidence:** active synthetic collection remains
   `nexusai-structured-demo-v2-20260730`; retained evidence and named volumes
   unchanged; no real source uploaded or altered.
8. **Models:** `qwen_qwen3-4b-instruct-2507` and `qwen3-embedding-0.6b` remain
   active; no model download or replacement.
9. **Accuracy/retrieval:** production `frequent_contacts` exact query returned
   `records_sql`, confidence 1 and six rows; no subject-scope failure. Warm
   bounded synthesis returned a 535-character grounded summary without fallback.
10. **Latency/memory:** warm synthesis 43.4 s; cold synthesis safely fell back at
    the 60 s sidecar ceiling. Bounded LocalAI compile uses 4 runtime threads and
    two package workers; last timeout-only rebuild was stopped at 1.19 GiB free.
11. **Failures/fallbacks:** fixed blank local Agent subject identity and
    records-only synthesis suppression. The source-tested 75 s Agent HTTP timeout
    is not yet deployed; the 45 s deployed client can still time out a visible
    model explanation even though exact queries work.
12. **Security/provenance:** unauthenticated local use requires the explicit
    configured proxy identity; missing identity fails closed. Raw full tables are
    excluded from LLM synthesis; rollback and audit history were preserved.
13. **Git:** unstaged, uncommitted and unpushed; no GitHub action.
14. **Rollback:** pre-attempt LocalAI/API snapshots were automatically restored
    and all five services returned healthy. Port 3000 preview remains stopped.
15. **Next action:** after a clean reboot with Codex/browser closed, run only
    `scripts/build_deploy_forensic_phase4_gate13.ps1`; require
    `Phase4Gate13=PASS`, then repeat the exact and explicit-explanation UI checks.
    Detailed status: `reports/forensic-enterprise-agent-chat-runtime-status-20260730.md`.

### 2026-07-30 - Evidence-family adapter, specialist-agent, API, and UX architecture reset

1. **Objective:** convert the team-lead direction into an implementation-grade
   target architecture and a durable next-chat handoff without deleting data,
   downloading models, or changing the retained runtime.
2. **Assumptions verified:** one logical Case Knowledge Fabric remains the correct
   product boundary; file formats and evidence-family semantics require separate
   contracts; exact evidence facts remain deterministic.
3. **Files changed:** added the approved evidence-family architecture/roadmap and
   `NEXUSAI_NEXT_CHAT_PROMPT.md`; updated `AGENTS.md` and this checkpoint so future
   sessions must read and follow the target design.
4. **Schema/API/config:** no runtime schema or configuration change. The target
   specifies versioned adapter/agent/model registries, typed query/result contracts,
   and a future `/api/v1/forensics` customer integration surface.
5. **Tests/inspection:** read-only Git/source/API/browser audit; `git diff --check`
   remains the documentation acceptance gate. No product code or retained build
   was changed by this architecture slice.
6. **Live runtime:** production Records and Agent Chat were inspected on port 8080;
   no query, upload, collection mutation, restart, or build was performed.
7. **Dataset/evidence:** the KB exposes 20 collections: two empty agent-name
   collections, fourteen acceptance/validation collections, two zero-KB-entry
   structured demos, `records-demo` with six entries, and
   `records-demo-verified` with eight. Nothing was removed or renamed.
8. **Models:** installed Qwen chat/embedding baselines remain unchanged. Official
   LocalAI, PaddleOCR, Docling, Whisper, and Tesseract materials were reviewed only
   to define benchmark candidates; no candidate is promoted by documentation.
9. **Architecture findings:** seven structured family definitions and generic
   processing remain concentrated in one worker; CDR is the deepest dedicated
   path; only one forensic agent exists; the Records UI is a 2,355-line monolith;
   model choices are not role-filtered.
10. **UI finding:** Records hardcodes `records-demo`, while Agent Chat is bound to
    `nexusai-structured-demo-v2-20260730`; the free-text case field, independent
    states, horizontal overflow, and duplicated controls make case scope and
    workflow unclear.
11. **Decision:** use one supervising Case Intelligence Orchestrator plus
    independently configured evidence-domain specialists. Each family receives a
    versioned adapter, operations, tools, goldens, model-role policy, API, UI result
    components, metrics, and rollback.
12. **Security/provenance:** collection cleanup is now explicitly manifest-first,
    soft/archive-first, and approval-gated. Test collections will be hidden by
    metadata before any permanent deletion is considered.
13. **Git:** all changes remain unstaged, uncommitted, and unpushed; no GitHub action.
14. **Rollback:** documentation-only changes can be reviewed independently; no
    database, evidence, model, container, volume, or service rollback is required.
15. **Next action:** begin Phase 5A source-only contracts and registry extraction,
    wrapping current CDR/generic behavior without output drift. Phase 5B then fixes
    the single active-case contract and produces the collection reconciliation
    manifest before the Case Workspace UI is rebuilt. Target design:
    `docs/design/nexusai-family-adapter-agent-api-architecture.md`.

### 2026-07-30 - Phase 5A contracts and registry foundation

1. **Objective:** establish the smallest versioned family-adapter, operation,
   specialist-agent, model-role, query-plan and enterprise-response foundation
   while preserving accepted CDR/generic behavior.
2. **Assumptions verified:** branch/base and dirty worktree matched; service
   endpoints were healthy; the two Qwen baselines, 20 collections and one
   configured agent matched the architecture handoff. Docker pipe access was
   denied, so no container-inventory claim was made.
3. **Files changed:** shared embedded platform catalog; Go typed contracts,
   discovery handlers and Ginkgo tests; Python lifecycle/registry, compatibility
   hook and goldens; API/worker/product docs; Phase 5A report and checkpoint.
4. **Schema/API/config:** no database or active configuration change. Source adds
   authenticated read-only `/api/v1/forensics/{adapters,operations,agents,contracts}`
   sidecar routes. Existing `/query/hybrid` JSON remains unchanged.
5. **Tests:** final Python Phase 5A 3/3 in 0.019 s; complete worker discovery 56 passed
   and two absent historical external-sample tests skipped in 1.290 s; focused Go
   9/9 in 10.142 s; full forensic package passed in 8.143 s; `go vet`, Python
   compile and JSON parse passed.
6. **Live runtime:** read-only health/model/collection/agent reconciliation only;
   no build, image, restart, deployment or live discovery-route claim.
7. **Dataset/evidence:** no evidence bytes, rows, collections, vectors, jobs,
   reports or audit state read beyond public aggregate/catalog APIs or changed.
8. **Models:** configured baseline references remain
   `qwen_qwen3-4b-instruct-2507` and `qwen3-embedding-0.6b`; exact structured role
   is explicitly no-model; pending roles remain benchmark-required.
9. **Accuracy/retrieval:** compatibility selection matches the pre-contract
   algorithm; CDR/generic normalized golden fields match exactly; accepted legacy
   `QueryPlan` JSON has an exact golden. No new retrieval/model accuracy claim.
10. **Latency/memory:** source tests only; no live latency/RAM/SLA claim.
11. **Failures/fallbacks:** first Go attempt hit the known Windows default-cache
   ACL; repository-local cache/GOTMPDIR with `GOPROXY=off` passed. One initial
   golden incorrectly equated MSISDN with originating number and was corrected to
   the accepted canonical behavior; product code was not changed for the test.
12. **Security/provenance:** discovery reuses sidecar bearer/trusted-scope auth;
   specialist peers cannot delegate; case scope comes from the orchestrator
   contract; no exact fact is assigned to a model.
13. **Git:** dirty worktree remains unstaged, uncommitted and unpushed; no GitHub
   or external publication action.
14. **Rollback:** source-only review rollback; no database, evidence, model,
   container or volume recovery action is required.
15. **Next action:** add the bounded legacy-to-v1 query/response compatibility
   mapper and accept the source discovery contract. Then begin Phase 5B active-case
   truth and read-only collection reconciliation; LocalAI proxy/OpenAPI publication,
   deployment, models, retained migrations, cleanup and Git remain separately gated.
   Detailed report:
   `reports/forensic-phase5a-contracts-registry-foundation-20260730.md`.

### 2026-07-30 - Phase 5B/5C case truth, governance, and Case Workspace completion

1. **Objective:** complete Phase 5 source work by adding the legacy-to-v1 query
   adapter, one enforceable case scope, zero-deletion collection governance,
   and the six-route responsive Case Workspace.
2. **Assumptions verified:** the new attachment and repository target design are
   line-for-line identical (589 lines); branch/HEAD remain
   `codex/forensic-hybrid-checkpoint-20260723` at `40717b83510c`; the worktree
   remains intentionally dirty; no retained-state authority was inferred.
3. **Files changed:** case/query compatibility and manifest sidecar source/tests;
   LocalAI case APIs, auth registry, discovery, docs and agent scope; Case
   Workspace/redirect/Records/Agent Chat UI and E2E; live reconciliation and
   completion reports.
4. **Schema/API/config:** no schema or active config change. Source adds
   `/api/v1/forensics/cases` list/read/manifest/evidence/query/report routes.
   The URL case equals the collection in v1 and conflicting body/header scope
   fails. Raw and typed query plans return `enterprise-response/v1`.
5. **Tests:** complete forensic Go package PASS (27.706 s); focused Phase 5
   package PASS (10.838 s after typed-plan tests); LocalAI governance 6/6 PASS;
   agent-pool/routes compile PASS; four-package `go vet` PASS; ESLint 0 errors;
   React production build PASS (658 modules, 1.67 s); Playwright 7/7 PASS in
   22.6 s across desktop/tablet/mobile, keyboard/focus and three legacy Records
   workflows.
6. **Live runtime:** GET-only checks returned LocalAI 200, forensic API 200 and
   NATS 200. New source was not deployed; no rebuild, restart, upload or query
   was performed against retained evidence.
7. **Dataset/evidence:** all 20 collections remain. The canonical candidate has
   8 KB entries, 8 KB assets, 6 evidence items, 9,250 accepted rows and 0 failed
   jobs. The legacy `records-demo` has 6 entries, 2 assets, 8 evidence items,
   10,000 accepted rows and 6 failed jobs. Full live table is in the manifest.
8. **Models:** no model change/download. Qwen chat/embedding baselines remain;
   exact structured operations remain no-model and fallback model output is not
   represented as successful interpretation.
9. **Accuracy/retrieval:** accepted legacy handler remains the execution core;
   old sidecar tests/goldens pass. Typed operation mapping, unsupported work,
   clarification, scope conflict, citation sections, and fallback separation
   have contract tests. No new model-quality claim.
10. **Latency/memory:** source tests/UI build only; no production SLA/RAM claim.
    Playwright completed 7 cases in 22.6 s. No model inference was run.
11. **Failures/fallbacks:** Playwright's bundled Chromium was absent; installed
    system Chrome was used with no download. Windows intermittently denies Go
    temp executable cleanup after successful results; the correct isolated
    Ginkgo suite and vet pass. Existing ESLint warnings remain, with zero errors.
12. **Security/provenance:** LocalAI verifies collection access; authenticated
    sidecar binds tenant, subject, collection and case; Agent Chat copies config
    per request; manifests are read-only with `deletion_count: 0`; unknown vector
    and report counts remain explicit, not guessed.
13. **Git:** all work is unstaged, uncommitted and unpushed; no branch, commit,
    PR, plugin or external publication action.
14. **Rollback:** source-only file review/revert. No database, evidence, vector,
    model, container, volume or agent-config recovery point is needed.
15. **Next action:** Phase 5 is source-complete. Deployment/rebuild and changing
    the stored analyst binding from `nexusai-structured-demo-v2-20260730` to the
    governed pilot require separate approval. After that, begin Phase 6.1 CDR as
    a bounded adapter/agent/API/UI/golden vertical slice. Detailed reports:
    `reports/forensic-phase5-case-governance-workspace-completion-20260730.md`
    and `reports/forensic-phase5-collection-reconciliation-manifest-20260730.md`.

### 2026-07-31 - Phase 6.1 Communications CDR source acceptance and governed binding

1. **Objective:** activate the approved governed analyst binding and implement
   the first bounded Phase 6 CDR adapter/specialist/API/UI/golden vertical slice.
2. **Assumptions verified:** Phase 5 source/checkpoint and the 589-line approved
   architecture remain authoritative; branch/HEAD and intentionally dirty
   worktree are unchanged; retained evidence and unrelated host processes are
   outside mutation scope.
3. **Files changed:** dedicated Python CDR adapter/manifest/tests and Docker copy;
   platform catalog; new CDR SQL/templates and typed response enrichment; CDR
   visuals/E2E; benchmark; product/worker/API docs; Phase 6.1 report/checkpoint.
4. **Schema/API/config:** no schema change. Source catalog advances to
   `2026-07-31.phase6.1`, adapter/specialist 1.1.0, and nine case-query CDR
   operations. The approved compatibility-agent binding changed from
   `nexusai-structured-demo-v2-20260730` to `records-demo-verified`. The approved
   active `Communications_CDR_Analyst` was created with the accepted Qwen model,
   forensic mode, safety/resource baseline, governed-pilot binding and a
   CDR-specific no-invention/citation prompt. Readback verified both configs.
5. **Tests:** forensic Go package PASS (final 16.623 s); focused Python 27/27;
   full worker 60 PASS with two external skips; API `go vet` PASS; JSON/bytecode
   PASS; ESLint zero errors; final Vite 658-module build PASS in 2.51 s;
   Playwright 8/8 in 23.3 s after the specialist handoff; diff check PASS.
   Read-only PostgreSQL prepare/EXPLAIN passed for
   both new SQL operations.
6. **Live runtime:** approved rebuild attempted after explicit rollback tags but
   stopped before compilation at the 5.25 GiB RAM guard (4.28 GiB free). Rollback
   restored prior images; later free RAM was 3.53 GiB and finally 3.85 GiB.
   LocalAI, forensic API, worker and NATS health endpoints all return 200. New
   Phase 5/6 source is not deployed; both approved agent configs are live.
7. **Dataset/evidence:** 20 collections remain. Governed pilot remains 9,250
   accepted, zero duplicate/rejected/failed jobs, four completed jobs, eight KB
   assets and six evidence items; CDR 5,000, IPDR 2,500, log 1,000, ANPR 750.
   Two active agents both bind to `records-demo-verified` with forensic mode.
   No upload, reprocess, delete, or evidence-returning SQL was executed.
8. **Models:** no model download/change/call. Exact CDR operations are
   deterministic. Optional explanation remains separate and citation-bounded.
9. **Accuracy/retrieval:** accepted normalization/hash behavior remains golden;
   Pakistan timezone/Urdu/provider tokens and one duplicate hash pass. Device
   changes now require a target, exclude dialed parties as device owners, and
   partition changes by originating subscriber. Unknown services are not guessed.
10. **Latency/memory:** read-only 2,000-iteration benchmark measured 36.8521 ms
    cold five-row pass, 770.8514 microseconds/warm row, 1,297.27 rows/s, 352,538
    peak allocated bytes and 9,205 adapter bytes. These are not live SLAs.
11. **Failures/fallbacks:** deployment guard worked and rollback completed; four-
    package vet timed out without diagnostics, while the changed API package vet
    passed. First Playwright path syntax matched no files; portable names passed.
12. **Security/provenance:** case scope remains URL-authoritative; CDR traces name
    exact adapter/tool; findings cite query/row provenance; maps do not infer
    routes; SQL validation used `BEGIN READ ONLY` and returned no evidence rows.
13. **Git:** all work remains unstaged, uncommitted and unpushed; no branch/PR or
    plugin action.
14. **Rollback:** image tags
    `rollback-before-phase5-20260731` preserve API `sha256:62760165...` and
    LocalAI `sha256:7232b000...`; compatibility binding rollback is the prior v2
    collection. Removing the new specialist is destructive and needs separate
    explicit approval. No database/volume rollback is needed.
15. **Next action:** free at least 5.25 GiB host RAM, rerun the approved deployment,
    verify Phase 5/6 discovery/case routes and bounded synthetic CDR queries, and
    record live latency/RAM/disk with unchanged counts. Only then mark Phase 6.1
    live-complete and begin Phase 6.2 IPDR. Detailed report:
    `reports/forensic-phase6-cdr-source-acceptance-20260731.md`.

### 2026-07-31 - Phase 6.2 Network IPDR source acceptance and specialist activation

1. **Objective:** proceed to the second approved Phase 6 structured-family slice
   while the retained image build remains below its mandatory RAM threshold.
2. **Assumptions verified:** the Phase 6.1 checkpoint and approved architecture
   remain authoritative; IPDR/network sessions follow CDR; exact facts remain
   deterministic; capture/payload conclusions remain separately gated.
3. **Files changed:** dedicated IPDR adapter/manifest/tests; worker compatibility
   binding; platform catalog; seven SQL/query-plan/enterprise operations and
   tests; typed UI/specialist handoff/E2E; benchmark; product/worker/API docs;
   Phase 6.2 report and this checkpoint.
4. **Schema/API/config:** no schema change. Source catalog advances to
   `2026-07-31.phase6.2` with three adapters and 25 operations. IPDR adapter and
   `Network_IPDR_Capture_Analyst` advance to 1.1.0 operational-family-slice
   status. The agent was created active with the accepted Qwen/resource/safety
   baseline, forensic mode, governed-pilot binding, and an IPDR no-inference,
   no-payload, citation/abstention prompt.
5. **Tests:** forensic Go package PASS in 16.560 s; API vet PASS; Python focus
   15/15 in 0.074 s; full worker 64 PASS with two external skips in 0.676 s;
   JSON/bytecode PASS; React 658-module build PASS in 1.02 s; ESLint zero errors;
   Playwright 9/9 in 21.4 s; diff check PASS.
6. **Live runtime:** LocalAI, sidecar, worker and NATS health endpoints return
   200. Three agents are active. New Phase 5/6 source remains undeployed because
   free RAM was 4.09 GiB versus the enforced 5.25 GiB build floor.
7. **Dataset/evidence:** governed pilot remains 9,250 accepted rows, zero
   duplicate/rejected/failed jobs, four completed jobs, eight KB assets and six
   evidence items; CDR 5,000, IPDR 2,500, access log 1,000 and ANPR 750. No
   upload, reprocess, evidence-returning SQL, migration or deletion occurred.
8. **Models:** no model download/change/call. Exact IPDR operations are
   deterministic; optional explanation retains the accepted Qwen baseline.
9. **Accuracy/retrieval:** Phase 4 Pakistan IPDR golden remains exact at six
   input, four accepted, two rejected, three unique hashes and one duplicate.
   IPv4/IPv6, explicit NAT, IDNA domain, ports, bytes, session times and source
   locators are verified. Ownership, DNS resolution, route, geography,
   attribution, maliciousness, payload and capture conclusions are prohibited.
10. **Latency/memory:** 2,000-iteration read-only benchmark measured 156.0175 ms
    cold pass, 1,985.4705 microseconds/input row, 503.66 input rows/s, 548,001
    peak allocated bytes and 10,758 adapter bytes. These are not live SLAs.
11. **Failures/fallbacks:** compatibility registry failed closed until the IPDR
    normalizer binding was supplied. The first read-only SQL prepare exposed the
    absent direct `version_id` column; projection was corrected. Final endpoint
    and overlap PREPARE/EXPLAIN passed inside `BEGIN READ ONLY`/`ROLLBACK`.
12. **Security/provenance:** target-bound subscriber/overlap/timeline operations
    clarify before execution; aggregates carry canonical-table provenance;
    row-bearing results carry source file/row/hash/evidence locators. Agent
    creation materialized two empty IPDR name-derived collections; catalog now
    has 24 collections, including four zero-entry CDR/IPDR specialist-name
    cleanup candidates. None was deleted or hidden.
13. **Git:** all work remains unstaged, uncommitted and unpushed; no branch, PR,
    plugin or external publication action.
14. **Rollback:** prior API/LocalAI image tags remain valid. Source can be
    reviewed/reverted as one IPDR slice. Removing the new agent or empty
    generated collections is destructive and separately approval-gated; no
    database/volume rollback is required.
15. **Next action:** free at least 5.25 GiB host RAM, rerun the approved
    memory-safe deployment, live-smoke catalog `2026-07-31.phase6.2`, Phase 5
    case routes, CDR and all seven IPDR operations, counts, latency/RAM/disk and
    specialist handoffs. Only then mark Phase 6.1/6.2 live-complete and begin
    Phase 6.3 ANPR/geospatial. Detailed report:
    `reports/forensic-phase6-ipdr-source-acceptance-20260731.md`.

### 2026-07-31 - Phase 6.1/6.2 clean image activation before operator restart

1. **Objective:** reclaim at least 6 GiB RAM, perform a complete sequential
   API/worker/LocalAI rebuild, deploy with rollback protection, and leave a
   reproducible local PowerShell procedure for the requested restart.
2. **Memory/disk preparation:** all five NexusAI containers were stopped after
   fresh API/worker/LocalAI rollback tags were preserved. Docker Desktop and WSL
   were shut down cleanly, producing 7.96 GiB free RAM. Unused BuildKit cache
   cleanup reclaimed 31.51 GB while preserving every image, container, model,
   named volume and retained data store. Reversible working-set trimming, not
   process termination, crossed the strict 6 GiB gate.
3. **First clean attempt:** preflight passed at 6.16 GiB. API and worker built.
   LocalAI failed closed because Docker DNS temporarily returned `no such host`
   for `proxy.golang.org`; this was not an OOM or code/test failure. Automatic
   rollback restored API, worker and LocalAI, and API/worker/NATS/LocalAI health
   returned 200. Disposable Alpine `nslookup` and HTTPS checks then passed.
4. **Gate hardening:** new
   `scripts/build_deploy_forensic_phase6_gate.ps1` enforces 6 GiB, builds all
   three images sequentially, preserves three rollback images, performs bounded
   LocalAI build retries, deploys dependencies before application services,
   checks four health surfaces, writes an activation marker, and automatically
   restores all three prior images on failure. Stage output now streams to both
   console and per-stage log.
5. **Successful activation:** retry preflight and the gate's internal check
   measured 6.56/6.72 GiB. API built in 6.2 s, worker in 7.8 s and LocalAI/UI in
   851.7 s with one LocalAI attempt. Marker deployment time is
   `2026-07-31T07:21:33.4812095Z`.
6. **Deployed images:** API
   `sha256:eaa48e89507af733043a25d589a4a70db82a2afad4e25a18ff5dac4884a52d11`;
   worker
   `sha256:a5edb8612aff2aaad357ec881f4953257228fc33b08ec454f0503b8a0977a1b0`;
   LocalAI
   `sha256:d63879907f15e9f99a6254ee4b6eead12efe4257b9bec1c8faa55be748036ddd`.
   They differ from the preserved rollback image IDs.
7. **Health:** deployed LocalAI `/readyz`, forensic sidecar `/healthz`, worker
   metrics/health and NATS `/healthz` all returned 200. Named volumes and
   rollback images are preserved. No migration, upload, reprocess, evidence
   query, agent mutation, collection deletion or model download occurred.
8. **Cleanup side effect:** BuildKit cache is intentionally zero immediately
   after the 31.51 GB cleanup, then repopulated only with the successful clean
   builds. No image/container/volume prune was run.
9. **Operator handoff:** the user requested a Windows restart and an operator-run
   rebuild with only Docker Desktop and PowerShell open before deeper live
   acceptance. Complete commands, transcript capture, live logs, memory gate,
   health/image/catalog/count/CDR/IPDR checks, DNS triage and rollback guidance
   are in
   `reports/nexusai-phase6-clean-rebuild-powershell-runbook-20260731.md`.
10. **Next action:** after restart, follow the runbook. Then perform exhaustive
    live catalog `2026-07-31.phase6.2`, 9,250-row reconciliation, all Phase 5
    routes, all CDR and seven IPDR operations, provenance/abstention, specialist
    handoffs and live latency/RAM/disk acceptance. Only after those pass mark
    Phase 6.1/6.2 fully live-complete and start Phase 6.3 ANPR/geospatial.

### 2026-07-31 - Phase 6.3 live probe and Records analyst UX closure source

1. **Objective:** correct the messy Records result workflow, make all existing
   CDR/IPDR/ANPR deterministic operations discoverable and usable, reconcile KB
   collection clutter safely, and provide an operator-owned rebuild/log/UI test
   sequence.
2. **Live evidence:** the guarded Phase 6.3 image activation passed with all four
   health surfaces. Six target-bound ANPR operations passed against `ABC-123`;
   target-free camera activity returned 20 rows. The audit identified a missing
   public discovery proxy, accidental operation-ID target extraction, and a
   duplicate/overloaded result layout.
3. **API corrections:** LocalAI now proxies authenticated `/adapters`,
   `/operations`, `/agents`, and `/contracts` discovery routes and registers
   them under Records feature authorization. Operation-only v1 planning now
   converts dots/underscores to words before natural-query fallback so
   `anpr.camera_activity` cannot become plate `CAMERA_ACTIVITY`.
4. **Records UI:** default output is one analyst summary: human-readable answer,
   four KPIs, typed visual analysis, cited findings, eight-row exact preview,
   next checks, and collapsed cautions. Secondary navigation is limited to All
   records, Timeline, Evidence, and Audit details. Empty `Value: {}`, duplicate
   case-health output, repeated limitation blocks, and the six-tab technical
   sprawl are removed. Query target is an explicit field; target-required chips
   fail before dispatch when blank.
5. **Operations/templates:** all current Phase 6 family workflows are exposed in
   family-aware discovery: CDR contact/call-type/temporal/duration/movement/
   device/service/tower; IPDR endpoint/domain/protocol/volume/subscriber/
   concurrency/timeline; ANPR sightings/camera/sequence/co-observation/timing/
   variants/timeline. Existing deterministic catalog remains phase6.3, four
   adapters and 34 operations; no invented model-only operation was added.
6. **Collections:** read-only live inventory found 26 collections. Ten are
   zero-entry specialist-name or superseded demo candidates; 15 retained
   acceptance/legacy collections contain 1-39 entries; governed pilot has eight
   KB entries. Default UIs hide retained/system collections and expose them only
   through an audit toggle. No reset, delete, upload, reprocess, migration, or
   evidence mutation occurred.
7. **Source verification:** forensic API suite PASS (185 specs: 183 passed, two
   intentionally skipped); API vet PASS; focused LocalAI discovery proxy and
   auth tests PASS; targeted Records/Collections/E2E ESLint has zero errors and
   repository-baseline JSX-use warnings; production React build PASS at 658
   modules; JSON/bytecode and diff checks PASS. The unrelated broad routes suite
   passed 18/19 and failed only its backend-upgrade fixture after sandbox-blocked
   OCI access. Pinned Swagger generation requires operator network access; the
   endpoint annotations are present and the exact command is in the runbook.
8. **Deployment state:** the previous Phase 6.3 image is healthy, but the public
   discovery/target/Records/Collections refinements are source-only until the
   user performs the next rebuild. Codex must not repeat that rebuild.
9. **Operator handoff:** exact restart, 6 GiB gate, transcript, streamed build
   logs, runtime logs, health/discovery assertions, 23-operation CDR/IPDR/ANPR
   matrix, desktop/mobile/keyboard UX sequence, and failure handoff are in
   `reports/nexusai-records-ui-acceptance-runbook-20260731.md`. Collection
   dispositions are in `reports/forensic-collection-cleanup-plan-20260731.md`.
10. **Next action:** user runs the guarded rebuild after restart with only Docker
    Desktop and PowerShell active, then completes the runbook. Mark this closure
    live only when public discovery, all operation traces, analyst result layout,
    collection visibility, exports, responsive behavior, and console/network
    checks pass.

### 2026-08-03 - Phase 5 query guidance and specialist-profile closure source

1. **Objective:** turn family query templates into executable analyst workflows,
   prevent masked coverage examples from becoming invalid query targets, make
   family/time scope transparent, and source-control the four active analyst
   profiles and their model/prompt boundaries.
2. **Operator transcript:** API and worker builds succeeded. LocalAI failed
   before compilation because `.dockerignore` excluded `.git` while Dockerfile
   copies it. After the operator exposed `.git`, the guarded retry stopped at
   5.26 GiB free RAM versus the mandatory 6 GiB. No
   `Phase6Activation=PASS` was produced, so the newest source is not live.
3. **Build contract:** `.git` is intentionally included for LocalAI build
   stamping; host `.tmp`, `.cache`, `.phase2-backups`, and nested `node_modules`
   remain excluded. The deploy gate now validates Dockerfile/ignore compatibility
   before stopping services. The observed valid root context was about 63.5 MB.
4. **Template API:** CDR/IPDR/ANPR catalog entries now expose operation/family
   IDs, required/optional inputs, accepted target kinds, measures, groupings,
   deterministic calculations, materialized examples, output meaning, and
   non-inference limits. Target input metadata directs callers to exact case
   entity discovery, never masked hints.
5. **Query correctness:** CDR frequent contacts now calculate from scoped source
   rows and can return true counterparties for an exact participant. Contact,
   call-type, temporal, duration, tower, and geospatial operations apply explicit
   inclusive/exclusive time bounds and return the applied scope. Existing IPDR
   and ANPR family scopes retain their date filters.
6. **Records UX:** exact authorized identifiers returned by `entity_activity`
   become target chips and datalist values; `entity_value`, subscriber, IP/NAT,
   IMSI/IMEI, plate, and canonical targets are recognized. Masked coverage hints
   are displayed separately as non-executable. Selected workflows show target
   required/optional state, accepted kinds, start/end controls, and expandable
   calculation/input/output documentation.
7. **Agent profiles:** new versioned
   `configuration/forensic_agent_profiles.json` defines separate orchestrator,
   CDR, IPDR, and ANPR/geospatial system prompts, permanent goals, user request
   contracts, family scopes, examples, and prohibited inferences. The preferred
   synthesis role remains `qwen_qwen3-4b-instruct-2507`; deterministic tools are
   authoritative. Bootstrap rejects explicitly selected non-chat models.
8. **Reproducibility:** `bootstrap_forensic_agent.ps1` now loads the exact named
   profile and `bootstrap_forensic_specialists.ps1` applies all four to one
   explicit tenant/case collection. Neither was executed against the live API.
9. **Verification:** forensic Go suite PASS; API vet PASS; JSON and PowerShell AST
   PASS; targeted UI lint zero errors with five pre-existing unused warnings;
   production React build PASS at 658 modules; bounded diff check PASS. Focused
   Playwright could not start its configured web server in the Windows sandbox;
   the new exact-target/template/date scenario remains operator-live-gated.
10. **Data and Git:** no Docker rebuild, model call/download, database/KB change,
    upload, reprocess, migration, collection deletion, agent mutation, staging,
    commit, push, or external publication. Physical empty-collection deletion
    remains separately approval-gated; the reversible UI decluttering remains.
11. **Handoff:** after restart, require at least 6 GiB free RAM, run the guarded
    build once, require `Phase6Activation=PASS`, apply all four profiles to
    `records-demo-verified`, then execute the complete Records UI/API acceptance
    runbook. Detailed evidence:
    `reports/nexusai-phase5-query-guidance-agent-profiles-source-acceptance-20260803.md`.
12. **Next phase:** only after current CDR/IPDR/ANPR operations, identifiers,
    date scopes, profiles, exports, responsive UI, and audit traces pass live,
    continue with the next approved evidence-family vertical slice.
13. **2026-08-03 operator retry:** after restart the machine had 9.86 GiB free,
    but Docker Desktop's Linux named pipe did not exist. No build or service
    mutation started and no activation marker was written. The gate now checks
    `docker info` before compose/image inspection, emits one actionable Docker
    readiness error, tracks whether services were actually mutated, and skips
    rollback when a preflight failure occurred. PowerShell AST and diff checks
    pass. The operator only needs to start Docker Desktop, wait for the Linux
    engine, verify `docker info`, and rerun the same gate; no laptop restart is
    required.

### 2026-08-03 - Phase 6.3 guarded activation and live acceptance

1. **Activation accepted:** the operator-run guarded build ended with
   `Phase6Activation=PASS`. API, worker, and LocalAI build times were 15.1,
   5.7, and 408.6 seconds; LocalAI passed on attempt one with 6.44 GiB free
   before build. Rollback images and named volumes were preserved.
2. **Health and catalog:** LocalAI, forensic API, worker metrics, and NATS passed
   live health checks. The authenticated public proxy exposes 53 templates,
   including 23 enriched CDR/IPDR/ANPR workflows. One governed selectable case,
   `records-demo-verified`, is exposed; 25 retained/system collections remain
   hidden from ordinary analyst selection.
3. **Profiles live:** version 1.2.0 of the orchestrator, CDR, IPDR, and
   ANPR/geospatial profiles was applied to tenant `default` and the governed
   case. Live readback confirmed separate prompts, the approved
   `qwen_qwen3-4b-instruct-2507` synthesis role, KB mode, and forensic mode.
4. **Deterministic acceptance:** exact case discovery returned usable CDR,
   IPDR, and ANPR identifiers. The complete operation matrix passed 23/23 via
   `records_sql`: CDR 9/9, IPDR 7/7, and ANPR 7/7. The fixture's governed
   `no_results` response for IPDR concurrent sessions is correct abstention,
   not a failure.
5. **Live Records UX:** the governed 9,250-row case loaded with zero duplicates
   and eight KB assets. Entity discovery, specialist links, readable analyst
   output, exact preview, visualization, provenance, and limits passed. A CDR
   call-type result returned eight rows and one source reference; the browser
   console had zero errors/warnings.
6. **Browser-driven source fixes:** live interaction exposed cross-family target
   materialization, native datetime dispatch, and responsive flex-overflow
   defects. Source now enforces family/type-compatible target examples and
   chips, reads datetime bounds from input refs at dispatch, and permits the
   Records shell to shrink at mobile/tablet widths. Targeted lint has zero
   errors; production React build passes at 658 modules; diff checks pass.
7. **Deployment boundary:** the backend, profiles, catalog, discovery, and
   deterministic operation matrix are live-accepted. The three final UI fixes
   are source-verified but were made after image digest
   `c98fb6fe74ab196ee22fbd4b56f6bfc99719fc9f8014074da56c9f7be26a30ea`
   was activated, so they are not yet live.
8. **Next bounded gate:** perform one guarded image refresh, then recheck only
   compatible template targets, datetime request bounds, 390/1024/desktop
   overflow, console state, and one deterministic result. Do not repeat the
   full 23-operation matrix unless backend source changes. Detailed evidence:
   `reports/nexusai-phase6.3-live-acceptance-20260803.md`.
9. **UI-refresh retry:** the next operator run stopped before image compilation
   at 5.9 GiB versus the unchanged 6 GiB minimum. Compose service-start output
   was automatic rollback restoring the prior deployment; post-rollback probes
   returned HTTP 200 for LocalAI, forensic API, worker, and NATS. The gate now
   also stops PostgreSQL/NATS during compilation and waits up to 45 seconds for
   Windows/WSL memory reclamation, reporting `RAMGate=WAITING` samples. Named
   volumes remain preserved; the threshold was not reduced. PowerShell AST and
   diff checks pass. The operator may rerun the same command without restarting
   the laptop.
10. **Corrected refresh and focused UI gate:** the retry reached
    `Phase6Activation=PASS` at exactly 6 GiB after the bounded wait. API,
    worker, and LocalAI builds took 3.1, 3.9, and 410.3 seconds; LocalAI passed
    on attempt one as image
    `sha256:9e0e9c47edfead2824d14aa5cd955a112753821362a577d22e6ddec555d5841c`.
    All four health probes return 200. Live browser acceptance confirms ANPR
    templates now offer only compatible exact plates and a bounded one-day
    query retains its dates, returning six exact rows and six citations.
11. **Final responsive finding:** live 390 px measurement improved from 1,234
    to 930 px but remained wider than the viewport; at 1024 px the case root
    extended to x=1178. The residual cause is max-content sizing of the
    horizontal-auto-margin `.case-workspace` flex child. Source now gives that
    root `width: 100%`. Targeted lint has zero errors, the production React
    build passes at 658 modules, and diff checks pass. The existing 390/820/1440
    overflow E2E could not start its configured Windows web server in this
    environment. One final image refresh and focused 390/820/1024 browser
    measurement remain; backend/profile/catalog/23-operation acceptance must
    not be repeated.
12. **Phase 6.3 fully live-accepted:** the final guarded refresh passed with
    6.31 GiB free, API/worker/LocalAI build times of 3.1/3.4/505 seconds, one
    LocalAI attempt, and preserved rollback images and volumes. Activated
    LocalAI image:
    `sha256:a283931706c2e428fa1b813ab29f81e3b2274bb4e290a4d846aafdd6bf39d6c8`.
    All four health surfaces return HTTP 200. Deployed browser measurements at
    390, 820, 1024, and 1440 px report zero horizontal-overflow pixels, and the
    console has zero warnings/errors. Together with the prior live target/date
    checks and 23/23 deterministic matrix, Phase 6.3 has no remaining gate.
    Do not rebuild or repeat Phase 6 acceptance unless its source changes;
    proceed to the next bounded evidence-family slice from the approved
    architecture.

### 2026-08-03 - Phase 6.4 capability-aware analyst workspace source closure

1. **Trigger:** live screenshots showed a broken sidebar logo, a locked-looking
   one-case selector, zero Evidence-tab rows despite six status-counted evidence
   items, development-console density in Analyze, and largely empty
   Relationships/Jobs surfaces. Agent Chat had not been live-tested.
2. **Collection contract:** all collections already authorized by
   `ListCollectionsForUser` can now be explicitly listed with
   `include_system=true`, selected, and inspected. Active, legacy/demo, and
   retained system/test governance classifications and non-deletion warnings are
   preserved. Default catalog calls may still hide system collections.
3. **Evidence correctness:** v1 case and collection are the same governed scope.
   Evidence catalog filtering now includes older rows whose `case_id` is empty
   while continuing to reject a non-empty different case binding. This resolves
   the verified case's status/catalog inconsistency without changing data.
4. **Analyst UX:** the case command bar uses a grouped collection selector and
   live readiness posture (hybrid, records, knowledge-only, processing, empty).
   Evidence registry is primary and upload is collapsed; Analyze removes the
   redundant embedded walkthrough and limits shortcuts; irrelevant specialist
   links are hidden; Relationships, Jobs, and Settings are compact and
   human-readable. The sidebar uses served branding assets rather than a 404
   source-only path.
5. **Agent Chat:** the case selector requests the same accessible full catalog
   and suggested questions are filtered by selected-collection capabilities.
   Exact case-readiness, IPDR protocol, and ANPR camera queries were found to fall
   through deterministic routing; source routing/template recognition now covers
   them and the focused regression passes. Empty evidence results now provide a
   governed analyst no-data summary.
6. **Live finding:** deployed source-audit chat completed through
   `forensic_hybrid_query` in about one second (`responseLen=1845`). The deployed
   case-readiness query loaded the local model and remained processing beyond 55
   seconds, confirming the regression before the source fix. The running image
   is therefore not Phase 6.4 accepted.
7. **Verification:** forensic sidecar evidence tests PASS; focused deterministic
   agent-routing/empty-result tests PASS; changed UI ESLint PASS; production Vite
   build PASS at 658 modules; bounded diff check PASS. Broad repository lint has
   six unrelated pre-existing hook errors. The focused LocalAI Ginkgo endpoint
   package exceeded the bounded Windows link/test timeout and is not claimed as
   passed.
8. **Safety:** no collection/entry/evidence/database/model/container/volume
   deletion, reset, upload, reprocess, migration, staging, commit, or push. The
   build was not run by Codex.
9. **Handoff:** run the existing guarded Phase 6 gate once, require
   `Phase6Activation=PASS`, then execute the focused API/UI/Agent Chat gate in
   `reports/nexusai-phase6.4-analyst-workspace-source-acceptance-20260803.md`.
10. **Next slice:** only after Phase 6.4 live acceptance, implement subscriber
    identity and tower/site intelligence as the next bounded evidence family.
11. **Source-preview UI acceptance:** the updated React workspace was exercised
    at 390, 820, 1024, and 1440 px against the running backend. Overview,
    Evidence, Analyze, Relationships, and Jobs have zero page-level horizontal
    overflow and zero broken images; browser diagnostics contain zero warnings
    or errors. Analyze is reduced to six primary presets with its redundant
    embedded ANPR hero removed. This validates source layout only; the collection
    catalog, legacy evidence inclusion, and deterministic readiness routing still
    require the single guarded rebuild and focused live gate above.

### 2026-08-03 - Phase 6.5 model-governed raw-query closure source

1. **Objective:** audit the downloaded models and all major forensic modules,
   exercise real model-assisted examples, correct agent/model-role integration,
   make unknown natural-language queries fail closed, and define the next
   architecture-approved evidence-family sequence.
2. **Live model truth:** only `qwen_qwen3-4b-instruct-2507` and
   `qwen3-embedding-0.6b` are installed. Qwen 4B is the optional explanation
   model for the four current agents; Qwen 0.6B remains an embedding/KB model.
   No model was downloaded. The embedding model incorrectly advertises a chat
   capability flag, proving model-role filtering cannot depend on flags alone.
3. **Live model probes:** bounded CDR and IPDR fact packets were explained by
   Qwen 4B in about 68.95 and 87.40 seconds. Counts and family prohibitions were
   respected, but the model reworded supplied citation locators. Deterministic
   citation objects therefore remain authoritative. An integrated Records
   explanation failed at the LocalAI proxy's 30-second deadline.
4. **Timeout correction:** source gives optional sidecar synthesis 120 seconds
   and both LocalAI/agent callers 135 seconds, while ordinary endpoints remain
   at 30 seconds. The outer callers can now receive either the model result or
   the deterministic sidecar fallback instead of masking it with a proxy error.
5. **Role correction:** Records Intelligence lists only enabled chat-compatible
   model names and excludes embeddings/reranking/OCR/speech/audio/vision/image/
   video roles. The sidecar enforces the same boundary. Text-only agent
   bootstrap clears `multimodal_model`; the Agent Chat ribbon describes model
   explanation as optional rather than claiming it occurred.
6. **Prompt correction:** sidecar synthesis now selects CDR-, IPDR-, ANPR-, or
   cross-family-specific non-inference policy, separates deterministic findings,
   model interpretation, and limitations, and tells the model not to rewrite
   supplied locators. Exact values and citations still come only from tools.
7. **Raw-query correction:** an unmatched question no longer defaults to
   `frequent_contacts`. It returns typed clarification, suggested operations,
   and runs no SQL, KB retrieval, or model. Capability-unavailable requests take
   priority. Direct agent routing now covers service/device changes, schema,
   collection overview, geospatial movement, and existing CDR/IPDR/ANPR depth.
8. **Capability audit:** governed case reports 20 families: eight queryable, two
   semantic-only, nine no-data, and one manual-review. Its 9,250 indexed rows are
   access/security 1,000, CDR 5,000, IPDR 2,500, and ANPR 750. Public operation
   discovery remains 34 operations: CDR 11, IPDR 9, ANPR 9, generic 5.
   Subscriber/tower/financial/generic adapter availability is not equivalent to
   specialist depth or populated dedicated rows.
9. **Verification:** PowerShell parser and gofmt PASS; forensic API suite PASS
   (188 specs registered, 186 passed, two skipped); focused agent routing PASS;
   changed-page ESLint has zero errors; production React build PASS at 658
   modules. Broad UI lint retains six unrelated baseline errors. The focused
   LocalAI endpoint package twice exceeded a 180-second Windows compile/link cap
   and is not claimed as passed.
10. **Deployment boundary:** no rebuild, runtime config mutation, data mutation,
    deletion, migration, upload, reprocess, staging, commit, or push. Phase 6.4
    and 6.5 remain source-only. Detailed evidence and the focused activation
    sequence are in
    `reports/nexusai-phase6.5-model-governance-query-audit-20260803.md`.
11. **Next phase:** after one guarded rebuild plus focused Phase 6.4/6.5 live
    acceptance, implement Phase 7A subscriber identity and tower/site
    intelligence as bounded vertical slices. Then proceed to financial,
    access/security, generic mapping, documents/OCR, audio/STT, image, video,
    capture/database/archive depth in the approved architecture order.

### 2026-08-03 - Phase 6.4/6.5 guarded activation and live defect closure

1. **Activation passed:** operator transcript contains `Phase6Activation=PASS`;
   API/worker/LocalAI builds were 11.9/3/440.1 seconds on the first attempt with
   7.3 GiB free. Rollback images and named volumes were preserved.
2. **Specialists live:** all four version 1.2.0 forensic profiles were applied
   to `records-demo-verified`/`default` with
   `qwen_qwen3-4b-instruct-2507`; live readback confirms KB and forensic tools,
   text-only model role, and empty multimodal model.
3. **Runtime acceptance:** all four health surfaces pass. Unknown language
   fails closed to clarification; unavailable audio transcription returns a
   capability guard. Exact CDR/IPDR/ANPR probes pass without a model. One
   explicit CDR explanation completed in 93.228 seconds without fallback and
   retained deterministic facts/citations as authority.
4. **Collection/UX acceptance:** 26 authorized collections are selectable.
   Switching to a retained knowledge-only collection adjusts metrics, presets,
   and specialist links truthfully. The chat-model selector excludes the
   embedding role. The professional CDR result renders eight exact rows, chart,
   KPIs, source reference, limits, export controls, and next checks; browser
   diagnostics are clean.
5. **Live defects found:** a typed analytical call-type question was
   incorrectly forced to `canonical_records`; the Evidence catalog multiplied
   rows through one-to-many job/asset joins; and source audit flattened mixed
   jobs/assets/families into 16 apparent records. Agent Chat correctly routed
   the query but displayed that incorrect 16-row inventory, proving the defect
   was shared backend accounting rather than chat model behavior.
6. **Source fixes:** canonical auto-promotion now requires an explicit
   structural predicate and leaves analytical language to the governed planner;
   Evidence uses latest-row lateral joins; `source_file_audit` now reconciles
   jobs, KB assets, and evidence by canonical filename with explicit accounting.
7. **Verification:** the live planner selects `call_type_breakdown` and returns
   eight rows when no template is supplied. Read-only execution of the new
   source audit SQL returns six canonical sources (two documents and four
   structured files totaling 9,250 rows). Forensic API tests pass; targeted UI
   ESLint has zero errors; production React build passes at 658 modules.
8. **Activation boundary:** the new fixes were made after the successful image
   activation and therefore require one final guarded image refresh. Recheck
   only typed natural CDR routing, six-row source audit, six-row Evidence
   registry, Agent Chat source audit, browser console, and
   390/820/1024/1440 overflow. Do not repeat the model benchmark, agent
   bootstrap, or full operation matrix unless related source changes.
9. **Safety:** no evidence, KB entry, collection, model, database row, volume,
   image, or rollback artifact was deleted; no upload, reprocess, migration,
   staging, commit, push, or external publication occurred. The SQL validation
   was read-only.
10. **Next phase:** after the focused post-refresh checks pass, Phase 6.4/6.5 is
    fully accepted and work proceeds directly to Phase 7A subscriber identity
    and tower/site intelligence from the approved architecture.

### 2026-08-04 - Final Phase 6 acceptance and Phase 7.1 subscriber source closure

1. **Phase 6 is closed:** after the operator's final guarded rebuild, live API,
   UI, and Agent Chat checks confirmed natural CDR routing, six reconciled source
   rows, six evidence items, responsive 390/820/1024/1440 layouts, and clean
   browser diagnostics. Do not repeat Phase 6 acceptance unless its source
   changes.
2. **Bounded next slice:** subscriber identity was implemented first. Tower/site
   remains the next separately accepted slice; it was not combined into this
   change.
3. **Dedicated adapter:** `nexusai.adapter.subscriber_identity` version `1.1.0`
   owns subscriber schema detection and normalization. It adds Unicode-aware
   name keys/script metadata, masked CNIC, parsed lifecycle timestamps, status
   normalization, and explicit invalid-window review flags while preserving the
   worker's compatibility entry point.
4. **Operations:** six deterministic public analyst workflows are registered:
   identity lookup, validity timeline, device/SIM observations, status summary,
   conflict audit, and reuse candidates. The platform catalog is
   `2026-08-04.phase7.1` with five adapters and 42 operations.
5. **Privacy/inference boundary:** default results never expose full CNIC or
   subscriber names. Observed links and reuse/conflicts are review candidates,
   not proof of identity, ownership, current control, SIM swap, fraud,
   association, or continuous device use. Exact target workflows accept only an
   authorized MSISDN, subscriber reference, IMSI, or IMEI and clarify when it is
   missing.
6. **Specialist/model integration:** `Subscriber_Identity_Analyst` version
   `1.1.0` is operational in source and added to the specialist bootstrap. The
   Qwen 4B chat model is optional only for an explicit narrative request; exact
   queries use deterministic tools without model latency. Qwen embedding remains
   retrieval-only and no model was downloaded.
7. **Analyst UX:** the governed case Analyze route displays a subscriber command
   center only when both subscriber data and operations exist, offers status and
   conflict calculations plus a prepared exact lookup, states the privacy
   boundary, and links to the specialist. Typed status results render an exact
   chart/table with citations, limitations, and execution trace.
8. **Defect found by UI acceptance:** the first focused browser test proved the
   command surface was hidden in the embedded case route. Source now exposes it
   in the actual analyst path and the rerun passes.
9. **Verification:** Python adapter/contracts PASS (27); forensic API Ginkgo
   PASS (206 specs, 204 passed and two skipped); focused direct-agent routing
   PASS; React production build PASS (658 modules); focused Chromium Playwright
   PASS; targeted ESLint has zero errors and five existing warnings. The broad
   agent package reached 67 passes before 13 unrelated Windows rootless-Docker
   testcontainer failures; this is not represented as a full-package pass.
10. **Deployment boundary:** Phase 7.1 is source-accepted, not live-accepted.
    No rebuild, agent mutation, upload, reprocess, migration, data deletion,
    model download, staging, commit, push, or publication occurred. The governed
    demo case does not yet contain dedicated subscriber rows.
11. **Handoff:** use the existing guarded rebuild once, apply the now five-profile
    specialist bootstrap, load only authorized or synthetic subscriber data,
    then run the focused ten-step live gate in
    `reports/nexusai-phase7.1-subscriber-identity-source-acceptance-20260804.md`.
12. **Next:** after Phase 7.1 live acceptance, implement Phase 7.2 tower/site
    intelligence with time-aware references, validated coordinates, explicit
    joins, provider drift fixtures, RF limitations, a separate specialist, and
    deterministic map/table citations.

### 2026-08-04 - Phase 7.1 runtime activation and hardening audit

1. **Activation reconciled:** the operator rebuild passed with
   `Phase6Activation=PASS`; API/worker/LocalAI builds were 13.9/5/735.1 seconds
   on the first attempt with 6.06 GiB free. Rollback images and named volumes
   were preserved. Five profiles were applied with the approved Qwen 4B model.
2. **Live runtime truth:** all four health surfaces return HTTP 200; model
   discovery returns only Qwen 4B chat and Qwen 0.6B embedding; agent discovery
   returns five active specialists; adapter/operation discovery remains five
   and 42. The governed case remains 9,250 rows with zero subscriber rows.
3. **Records acceptance:** live `subscriber_status_summary` returned the v1
   enterprise contract as a deterministic bounded no-data result with two
   findings, one source reference, explicit privacy limitations, zero model
   roles, and 68 ms observed in the UI. At 1280 px the page has zero overflow
   and browser diagnostics are empty.
4. **Agent Chat acceptance:** case-scoped `Subscriber_Identity_Analyst` chat
   completed in the same second through `forensic_hybrid_query`, selected
   `subscriber.status_summary`, used no model, and returned the same governed
   negative result. Agent/runtime integration is live; populated-data behavior
   is not yet accepted.
5. **Collection defect:** bootstrap increased the authorized catalog from 26 to
   28 with `Subscriber_Identity_Analyst` and
   `subscriber_identity_analyst`. Native forensic create/update/import now
   ensure only the configured `forensic_collection_id`; generic agents retain
   name-based collections. Existing duplicates were not deleted and remain
   manifest-backed cleanup candidates.
6. **Build-context defect:** the successful build transferred a 1.25 GB root
   context. `.codex-tmp` (about 696.4 MiB) and `.codex-cache` (about 368.8 MiB)
   were principal avoidable contributors. Docker/Git ignores now exclude these
   and other host-only test/cache outputs without removing `.git` version
   stamping.
7. **Privacy defect:** cross-family correlation could emit a full CNIC via
   `cnic_raw`/`cnic_digits`. Relation aggregation now groups on the internal raw
   identifier but returns only `*********` plus the last four digits. A focused
   privacy regression and updated golden pass.
8. **Pakistan language hardening:** a 24-question English, Roman Urdu, and Urdu
   subscriber corpus covers lookup, validity, device links, status, conflicts,
   and reuse. Planner and deterministic Agent Chat routing share the governed
   vocabulary. A live Roman Urdu query proved the deployed pre-fix image could
   return 20 unrelated rows; source now selects the correct subscriber workflow
   or target clarification.
9. **Verification:** focused native collection selection and agent routing
   PASS; focused Phase 7.1 plus cross-family API run PASS (26/26 selected
   specs); agent-pool package compile PASS; Docker BuildKit `--check` PASS with
   no warnings. No data, evidence,
   collection, entry, model, image, volume, or
    rollback artifact was deleted; no upload, migration, reprocess, staging,
    commit, push, or publication occurred.
10. **Future-model correction:** official Qwen3-ASR language support does not
    list Urdu, so both Qwen3-ASR candidates are now explicitly ineligible for
    the Pakistan Urdu/English primary ASR role. Whisper remains benchmark-only;
    PaddleOCR PP-OCRv5 remains OCR-eligible because its official Arabic-script
    model lists Urdu, Pashto, Sindhi, Balochi, and English. No model was
    downloaded or activated.
11. **Deployment boundary:** the operator activation predates the hardening
    changes above. One focused guarded refresh is required. Confirm no new
    specialist-name collection is created, then load subscriber data only with
    explicit authorization and complete all six populated-data/privacy/export
    gates before calling Phase 7.1 data-accepted.
12. **Next phase:** Phase 7.2 tower/site remains next because it is a dependency
    for Pakistan CDR location correlation. The approved follow-on sequence is
    tower/site, Pakistan CDR deepening, ANPR image/OCR, image forensics,
    Urdu/English STT and disclosed TTS, then supporting identity/vehicle/
    financial/access/social/report slices. Detailed evidence:
    `reports/nexusai-phase7.1-runtime-hardening-and-platform-audit-20260804.md`.

### 2026-08-04 - Phase 7.1 hardening activation closure

1. **Guarded refresh passed:** the later operator transcript records
   `Phase6Activation=PASS`; API/worker/LocalAI builds were 19.8/4.1/375.4
   seconds, the LocalAI build completed on its first attempt, and free RAM at
   the gate was 6.77 GiB. Rollback images and named volumes were preserved.
2. **Build-context control verified:** the root LocalAI context transferred
   128.95 MB instead of the earlier 1.25 GB. The host Codex cache exclusions
   are therefore active without removing `.git` version stamping.
3. **Runtime inventory:** LocalAI, forensic API, worker metrics, and NATS all
   return HTTP 200. Only Qwen 4B chat and Qwen 0.6B embedding are installed;
   all five specialist profiles are active.
4. **Collection governance:** the catalog remained at 28 after activation.
   No additional specialist-name collection was created. The pre-existing
   `Subscriber_Identity_Analyst` and `subscriber_identity_analyst` collections
   remain preserved cleanup candidates; deletion still requires explicit
   operator approval.
5. **Multilingual live closure:** `subscriber ki maloomat dikhao` now returns
   `needs_input` with `subscriber.identity_lookup` and no SQL/model execution.
   An exact Roman Urdu Pakistan-mobile lookup normalizes the target and returns
   a bounded `no_results` result. The Urdu status aggregate, sent as explicit
   UTF-8 bytes, selects `subscriber.status_summary`, returns `no_results`, and
   uses no model. PowerShell 5.1 Urdu tests must explicitly encode UTF-8 JSON.
6. **UI acceptance:** at 1280 x 720 the Analyze desk has zero horizontal
   overflow and zero browser warnings/errors. The 28 accessible collections
   render in the selector; switching to `Subscriber_Identity_Analyst` and back
   to `records-demo-verified` succeeds.
7. **Honest boundary:** Phase 7.1 hardening and no-data behavior are deployed
   and accepted. The case still contains zero subscriber rows, so identity,
   validity, device-link, conflict, reuse, populated privacy, export, and model
   explanation behavior are not data-accepted. No subscriber data or retained
   state was mutated during this closure.
8. **Next action:** obtain explicit authorization for a clearly synthetic
   Pakistan subscriber fixture, run all six deterministic operations and the
   populated privacy/citation/export/Agent Chat/UI gate, then begin Phase 7.2
   Tower/Site Intelligence. Do not repeat the expensive rebuild unless related
   source changes require deployment.

### 2026-08-04 - Phase 7.1 populated subscriber safe source gate

1. **Objective:** finish every safe populated-data preparation gate without
   inferring authorization for a retained upload or cleanup.
2. **Assumptions verified:** branch/HEAD and intentionally dirty worktree match;
   LocalAI, forensic API, worker metrics, and NATS return HTTP 200; deployed case
   remains zero-subscriber and no-data accepted.
3. **Files changed:** worker/query privacy boundaries, Phase 7 tests, one
   synthetic JSONL fixture, golden/oracle, benchmark, reports, and checkpoint.
4. **Schema/API/config:** no schema/route/config mutation. Existing raw/typed
   query paths now reject prohibited subscriber target types, and canonical
   subscriber results redact protected fields in source.
5. **Tests:** focused Phase 7 Python 6/6 PASS; combined Phase 2/7 Python 26/26
   PASS; final focused Phase 7 Ginkgo 23/23 PASS (package 13.528 s); final full
   forensic API package PASS (5.768 s); Go vet, Python compile, JSON, gofmt, and
   diff checks PASS.
6. **Live runtime:** health-only reads; no build, deploy, upload, query, model
   call, agent change, collection change, reprocess, or cleanup.
7. **Dataset/evidence:** fixture SHA-256
   `a5edc8f2658b12e8564b49560fe40959be397de6e086eaa1bfe2aa11bad912b0`;
   15 total, 12 normalizable, 1 duplicate, 11 unique accepted, 3 rejected.
   Evidence/version/batch/job IDs remain explicitly data-pending.
8. **Models:** established Qwen chat/embedding pair unchanged; zero calls.
9. **Accuracy:** offline oracle passes all six operations exactly: lookup 2,
   timeline 2, device links 2, status rows 11/six groups, conflicts 2, reuse 3;
   public projections mask CNIC and omit names with row/hash provenance.
10. **Latency/memory:** bounded 200-iteration run: 157.8104 ms cold/15 rows,
    3,094.0496 microseconds/warm input row, 323.2 rows/s, 439,373 peak Python
    allocation bytes. The 2,000-iteration attempt hit the 30-second command cap
    and is not counted as a pass.
11. **Failures/fallbacks:** source audit found and fixed CNIC/name target echo
    and generic/canonical export disclosure before upload. The first response
    test exposed a residual normalized CNIC in the nested query plan; the final
    23/23 focused run proves it removed. Live populated SQL,
    export, Agent Chat, model and UI gates remain authorization-blocked.
12. **Security/provenance:** prohibited values execute no SQL/model and are
    removed from response/export fields; raw evidence remains protected while
    generic indexes and canonical outputs use only safe projections.
13. **Git:** unstaged, uncommitted, unpushed; no PR/publication.
14. **Rollback:** source-only; no retained rollback. Legacy subscriber-name
    collections and every named volume/image/evidence row remain untouched.
15. **Next action:** explicitly authorize deployment plus upload of the exact
    hashed isolated fixture, then record generated lineage IDs and run all live
    privacy/citation/pagination/export/audit/model/UI gates. Phase 7.2 remains
    planned only after Phase 7.1 data acceptance.

Detailed report:
`reports/nexusai-phase7.1-populated-subscriber-source-readiness-20260804.md`.

### 2026-08-04 - Phase 7.1 populated subscriber runtime closure

1. **Verdict:** Phase 7.1 is populated-runtime and data accepted on the isolated
   synthetic collection `phase7-subscriber-populated-acceptance-v1`.
2. **Fixture and lineage:** SHA-256
   `a5edc8f2658b12e8564b49560fe40959be397de6e086eaa1bfe2aa11bad912b0`;
   evidence `1b1f27f7-d8cc-4dc1-b33e-758885e1ca8a`; version
   `cc83458d-e17d-4d66-98a6-a670d40467ab`; batch/job
   `d57c9b68-d34d-4134-9953-bcd2fc185d68`.
3. **Accounting:** 15 input, 11 accepted, 1 duplicate, and 3 rejected. All six
   exact oracles pass: identity 2, validity 2, device links 2, status 6 groups
   totaling 11, conflicts 2, and reuse candidates 3.
4. **Privacy and provenance:** raw CNIC/name targets fail closed; protected
   values do not enter SQL, public canonical rows, preview, export, audit, or
   optional model prompts. Source/evidence/row hashes and protected raw evidence
   remain available under their intended controls.
5. **Exports and UI:** CSV grid and JSON audit downloads passed. Agent Chat
   returned the exact result (message `1785829727421117664`). The case workspace
   passed at 390/820/1024/1440 px with no page overflow or browser log errors.
6. **Model gate:** Qwen subscriber-status prose was policy-rejected for an
   unsupported validity-overlap claim and deterministic fallback was returned
   (`e7ca74fd-ddc1-4f6a-9a3b-10717f22afc6`). A separate data-quality narrative
   passed factual review; a frequent-contacts narrative was manually rejected
   because its claimed top-five range omitted two deterministic counts.
7. **Governance:** `records-demo-verified` was not mutated. Both legacy
   subscriber-name collections remain preserved; no cleanup was performed.
8. **Evidence:** activation marker
   `reports/runtime-activation-20260804/phase7.1-populated-acceptance.json`;
   report
   `reports/nexusai-phase7.1-populated-subscriber-runtime-acceptance-20260804.md`;
   full accepted question catalog
   `reports/nexusai-agent-query-catalog-phase7.1-20260804.md`.

### 2026-08-04 - Phase 7.2 Tower/Site bounded source and runtime slice

1. **Delivered:** `nexusai.adapter.tower_location` plus the active
   `Tower_Location_Reference_Analyst` and six operations: exact site lookup,
   reference timeline, coordinate audit, status summary, alias conflicts, and
   time-aware exact CDR join.
2. **Contracts:** registry is now 6 adapters, 50 operations, and 65 accepted
   templates. Exact allowed targets are site/sector ID, LAC, TAC, CGI, ECGI,
   eNodeB ID, and gNodeB ID. All SQL remains parameterized and paginated.
3. **Adapter quality:** the 10-row fixture reconciles to 6 candidates, 5 unique,
   1 duplicate, and 4 rejects. WGS84/EPSG:4326 is preserved, other datums are
   flagged for transformation, coordinate/radio bounds are validated, and
   missing facts are flagged rather than invented.
4. **Live acceptance:** preserved collection
   `forensic-phase2-complete-acceptance-20260727` exposes 5 tower references.
   Lookup/timeline return the exact Lahore synthetic row; coordinate audit
   returns 5; status returns 4 groups totaling 5; conflicts returns accepted
   no-results. The `CELL-01` join returns 2 cited CDR rows, both explicitly
   unmatched because no exact time-eligible reference identifier exists.
5. **Activation:** guarded API/worker sidecars and coverage hotfix passed;
   rollback images, LocalAI image, named volumes, collections, and evidence were
   preserved. Markers are
   `reports/runtime-activation-20260804/phase7.2-sidecar-activation.json` and
   `reports/runtime-activation-20260804/phase7.2-coverage-sidecar-activation.json`.
6. **Verification:** combined family Python 26/26; forensic API 220 passed/2
   skipped of 222 registered; Go vet, deterministic direct routing, and React
   production build pass.
7. **Honest boundary:** available free RAM was about 2.6 GiB, below the 6 GiB
   guarded LocalAI build threshold. Live Agent Chat reached the correct Tower
   specialist/collection and returned the exact cited row via compatible legacy
   `tower_activity`; the new `tower_site_lookup` direct phrase and Tower UI
   controls are source-tested and await the next guarded full LocalAI refresh.
8. **Next:** once memory permits, run one guarded full LocalAI refresh and only
   the focused Tower direct-route/UI smoke. Then deepen Pakistan CDR analysis
   using exact time-aware references, preserving unmatched joins and the ban on
   RF/device-presence inference.

Detailed report:
`reports/nexusai-phase7.2-tower-site-source-and-runtime-acceptance-20260804.md`.

### 2026-08-05 - Phase 7.1 closure and Phase 7.2 Agent Chat acceptance

1. **Single demo collection:** retained only `nexusai-forensic-demo`. It has 8
   evidence items, 8 completed jobs, 9,282 input rows, 9,272 accepted, 3
   duplicates, 7 rejects, 9,272 canonical records, and 44,318 record/entity
   links. There are no failed or in-flight jobs.
2. **Authorized cleanup:** removed 30 pre-consolidation Knowledge Base
   collections and all non-demo forensic database rows. The final rebuild
   recreated six generated/specialist memory collections; the authorized
   consolidation was reapplied after startup and removed all six. Browser
   verification shows only `nexusai-forensic-demo`.
3. **Agent Chat contract:** users type normal questions. All 65 accepted
   operations infer their deterministic route, target, and bounded defaults;
   neither `template=...` nor `limit=...` is required. Explicit template syntax
   remains optional for API/debug use only.
4. **Deterministic acceptance:** final records-API matrix PASS: 65 queries, 64
   answered with data, 1 accepted no-results, 0 route mismatches, 0 failures,
   0 incomplete requests, and 257 ms p95. The later real Agent Chat/SSE matrix
   passed 65/65 with 9,815 ms p95 and 22,916 ms maximum. Recognized summaries
   remain deterministic unless the user explicitly asks to explain or interpret.
5. **Model acceptance:** the final request-correlated Agent Chat POST+SSE matrix
   completed all six agents in 38.2-49.5 seconds. Three model interpretations
   passed forensic policy; three unsafe drafts were rejected and returned an
   explicit complete deterministic fallback.
6. **Stall fixes:** direct forensic routing bypasses the model for recognized
   deterministic questions; analytical timeout is 30 seconds; deterministic
   anomaly/cross-dataset summaries do not silently select synthesis; `briefly`
   no longer selects a forensic brief; the LLM smoke harness requires the exact
   POST acknowledgement ID, uses a 210-second ceiling, and persists each result.
7. **Final guarded activation:** PASS with API 3.0 s, worker 3.9 s, LocalAI/UI
   33.5 s, one successful attempt, and 6.59 GiB free at the unchanged 6 GiB
   gate. LocalAI image is
   `sha256:fcb3355210d78235c677fc3b6f86fa13122c4271c54518cef3beace2b47ecf29`;
   named volumes and rollback images were preserved.
8. **Build reliability:** an earlier activation exhausted bounded retries on
   external Go proxy EOFs and a Docker Hub TLS timeout, then restored services.
   The Dockerfile now uses persistent BuildKit Go module/compile caches; the
   next guarded run passed without weakening gates.
9. **UI acceptance:** six agents active; Forensic Records Agent Chat selected
   the demo case automatically and answered a plain data-quality question with
   exact 9,282/9,272/3/7 accounting, records-SQL route, provenance and stated
   limitations. Browser console logs were empty.
10. **Catalog and evidence:** complete questions and model prompts are in
    `reports/nexusai-agent-query-catalog-phase7.1-20260804.md`; final acceptance
    is `reports/nexusai-phase7.2-agent-chat-acceptance-20260805.md`; activation
    marker is `reports/runtime-activation-20260731/phase6.3-live-activation.json`.

### 2026-08-04 - Agent Chat complete-template routing closure

1. **Agent Chat-first contract:** all 65 accepted templates can now be selected
   unambiguously with `Run deterministic forensic query;
   template=<accepted_template>; target=<exact target when required>; limit=20`.
   Natural-language shortcuts remain supported.
2. **Model selection:** exact deterministic prompts use no model. Model
   assistance is requested by beginning with `Explain`, `Interpret`,
   `Summarize`, `Executive brief`, `Narrative`, `Why`, `What does this mean`, or
   `Model synthesis`; the active agent's configured local model is then used as
   bounded synthesis over deterministic facts.
3. **Defect prevented:** the explicit deterministic prefix overrides model
   words embedded in a template name, so `template=executive_case_brief` does
   not accidentally invoke a model.
4. **Tests:** natural-language route coverage, all 65 explicit templates,
   explicit model assistance, and deterministic override passed in the focused
   agent package (12.980 s). The approved full rebuild script parses cleanly.
5. **Activation:** these Agent Chat routing changes are source-only until the
   next guarded full API/worker/LocalAI/UI rebuild completes with at least 6 GiB
   free RAM. Complete copy-ready questions and limitations are in
   `reports/nexusai-agent-query-catalog-phase7.1-20260804.md`.

### 2026-08-05 - Phase 7.2 Agent Chat reliability and model-grounding closure

1. **Screenshot defects closed:** case overview, CDR call-type breakdown,
   cross-family correlation, entity timeline, evidence inventory, and executive
   brief all complete through the real Agent Chat/SSE path. The CDR screenshot
   regression returned `call_type_breakdown`, collection
   `nexusai-forensic-demo`, 9 rows, and no remaining `Working...` indicator.
2. **Request correlation:** started, status, stream, error, tool-result, and
   final events carry the originating `message_id`. The React UI ignores stale
   events and clears progress only for the active request. Native execution is
   bounded at 210 seconds.
3. **Governed routing:** all nonempty forensic questions remain in the governed
   deterministic/hybrid forensic planner. Specialist agents cannot fall through
   to legacy generic records tools or a stale/default collection.
4. **Full natural acceptance:** 65/65 natural questions completed through the
   actual Agent Chat POST plus exact request-correlated SSE path; 0 route
   mismatches, 0 failures, and 0 incomplete/stuck requests. P95 was 9,815 ms and
   maximum was 22,916 ms. The separate records-API route-check p95 was 257 ms.
5. **Model-grounding acceptance:** all 6 specialist model prompts completed in
   38,219-49,474 ms. Three returned policy-compliant qualitative model
   interpretation; three safely rejected prohibited inference and returned a
   complete deterministic answer with an explicit fallback. Exact SQL findings
   are always rendered independently of model prose.
6. **Verification:** forensic API suite passed 221 executed tests with 2
   intentional skips; focused Go routing, React production build (658 modules),
   and React lint with zero errors passed. Final six-query screenshot-path
   regression passed 6/6.
7. **Activation:** guarded full rebuild passed with API/worker/LocalAI build
   times 3.0/3.9/33.5 seconds, 6.59 GiB free at the unchanged gate, one LocalAI
   attempt, preserved named volumes, and preserved rollback images. Images are
   LocalAI/UI `sha256:fcb3355210d78235c677fc3b6f86fa13122c4271c54518cef3beace2b47ecf29`,
   API `sha256:3a6c39db71ca6a47e30538a01e9a865fbdbd5ef0130edd4e5420f4ab190976ec`,
   and worker `sha256:b8db7e3b430cc59b49a66d8983769427f95258546011f325df52c193db1b203d`.
8. **Handoff:** complete copy-ready questions and safe model prompts are in
   `reports/nexusai-agent-query-catalog-phase7.1-20260804.md`; detailed closure
   is `reports/nexusai-phase7.2-agent-chat-reliability-closure-20260805.md`;
   upcoming phases are `reports/nexusai-upcoming-phase-roadmap-20260805.md`.

### 2026-08-05 - Phase 7.3 professional answer presentation runtime acceptance

1. **User-facing contract:** Agent Chat now formats enterprise results as a
   direct answer, optional validated plain-language interpretation, useful
   totals, a five-row humanized key-results table, important context, and a
   collapsed technical/source section. Internal template, route, planner,
   operating-metric, sourced-claim, and data-grid labels are no longer expanded
   in the main answer.
2. **Specific presentation:** all 65 accepted operations have a unique analyst
   title. CDR, IPDR, ANPR, subscriber, and tower results have preferred readable
   columns; fallback tables are bounded to five columns and common forensic
   acronyms remain correctly capitalized.
3. **Model default:** ordinary natural forensic questions request bounded local
   explanation after deterministic computation. Exact facts are independently
   rendered and authoritative. Unsafe, incomplete, numeric, or unsupported
   prose is rejected without suppressing the professionally formatted factual
   answer. Explicit deterministic-only mode remains available for internal
   acceptance and low-latency diagnostics.
4. **Evidence and reports:** evidence UUIDs, processing routes, report scope,
   and other implementation details are placed in collapsed disclosures rather
   than the primary answer.
5. **Verification:** focused presentation/routing tests and the complete
   forensic-records tool group pass; forensic API passes 221 executed tests with
   2 intentional skips; AgentChat lint has 0 errors and 14 existing warnings;
   React production build passes with 658 modules. Repository-wide UI lint has
   unrelated pre-existing errors in E2E coverage fixtures.
6. **Runtime activation:** guarded full rebuild/deploy PASS. Final cached timings
   were API 3.6 s, worker 4.4 s and LocalAI 40.1 s with one LocalAI attempt,
   6.15 GiB free before build, rollback images preserved and volumes preserved.
   All five services are running; LocalAI, worker, NATS and PostgreSQL report
   healthy.
7. **Complete matrices:** all 65 Agent Chat operations pass computation and
   professional-presentation acceptance (p95 9,655 ms). The separate bounded
   model matrix passes for all six specialist agents, with accepted model
   interpretation and 32,607.2-42,548.3 ms latency (p95 42,548.3 ms).
8. **Browser acceptance:** a normal CDR call-type question completed after the
   final rebuild with no orphaned `Working...`; direct answer, plain-language
   interpretation, humanized key-results table and collapsed traceability were
   visible. Raw template/operating diagnostics and the formerly empty
   `Important context` heading were absent.
9. **Deployed images:** LocalAI/UI `sha256:0aff21e8f09b...`, forensic API
   `sha256:4c415a95e18d...`, forensic worker `sha256:7a62791e40cf...`.
   Report:
   `reports/nexusai-phase7.3-professional-answer-presentation-source-readiness-20260805.md`.

### 2026-08-06 - R0 reconciliation and truthful Case Workspace loading state

1. **Directive reconciliation:** the new enterprise master directive was read
   with this checkpoint, the approved family architecture, AI contribution
   policy, build/test guide, coding style, current reports, and activation
   artifacts. `NEXUSAI_NEXT_CHAT_PROMPT.md` was found stale at Phase 7.1; an
   authoritative override now points to Phase 7.3 lifecycle reliability.
2. **Git safety:** branch and HEAD remain
   `codex/forensic-hybrid-checkpoint-20260723` at `40717b83510c...`. Opening
   worktree state was 56 modified and 192 untracked paths, zero deleted. No
   reset, staging, commit, push, stash, or publication occurred.
3. **Live runtime:** all five containers are running. LocalAI/UI, API, and worker
   image digests match the August 5 Phase 7.3 acceptance. Health probes return
   200. Models remain exactly Qwen 4B chat/explanation and Qwen 0.6B embeddings.
4. **Case truth:** v1 discovery exposes one governed analyst case and hides four
   cleanup/system candidates. Its read-only manifest reports 9,272 canonical
   records across seven families, eight evidence items, eight KB assets, 11 KB
   entries, 44,318 record entities, eight jobs, six agent bindings, eight audit
   events, and zero deletions. The non-owner PostgreSQL role sees the 17 expected
   `forensic` tables. No data mutation occurred.
5. **R0/R1 artifacts:** added current reconciliation, capability JSON, risk/gap
   register, repository/capability/API/UI/backend/product-boundary architecture
   baselines, brand/design/information-architecture/migration documents, current
   roadmap, and team-lead brief. R1 is explicitly partial outside the complete
   forensic route inventory; the remaining 371 compatibility/admin route
   registrations still need per-route middleware classification.
6. **Live browser baseline:** deployed Overview has zero page overflow and zero
   broken images at 390/820/1024/1440 with no console warning/error. Deployed
   selectors still reveal four cleanup candidates, whereas current source shows
   only the governed analyst case; this source/runtime difference is recorded.
7. **Truthful loading fix:** deployed Overview was observed briefly claiming
   `Inventory only`, zero evidence/rows/families, and no queryable coverage while
   APIs loaded. Current source now renders `Verifying case state`, unknown-value
   dashes, and explicit registry/store/capability checks until governed data
   resolves. It never emits a zero-data conclusion from an uninitialized state.
8. **Verification:** targeted ESLint has zero errors (12 pre-existing warnings
   in the untracked page); production Vite build passes with 658 modules; focused
   Case Workspace Playwright passes 5/5 on installed Chrome, including the held
   case-list loading regression and 390/820/1440 overflow checks. Source preview
   loads the sole governed case with the correct 9,272-row state and no browser
   diagnostics.
9. **Deployment boundary:** the selector and truthful-loading improvements are
   source-only. No Docker build/restart, migration/backfill, upload/reprocess,
   collection cleanup, agent/model mutation, model/backend download, or retained
   configuration change occurred. Existing rollback images and volumes remain.
10. **Superseded next objective:** the Phase 7.3 Agent Chat lifecycle proposal
    was not started. The later master-directive enforcement correction below
    reclassifies that work under future R6 (tentatively R6.2).

### 2026-08-06 - Master-directive enforcement and R-program governance correction

1. **Authority adopted:** `NEXUSAI_MASTER_DIRECTIVE.md` contains the complete
   user-supplied master prompt beneath the required precedence and
   historical-versus-active-program notice.
2. **Program ledger:** `docs/roadmap/nexusai-phase-mapping.md` preserves legacy
   evidence while `nexusai-phase-ledger.md` tracks R0-R17 with independent
   source/deployment/runtime/data states. R1 is the sole Active Program Phase;
   R1-AUD-01 is the sole Active Work Item.
3. **R0 evaluation:** complete at the bounded 2026-08-06 criteria. Current live
   functionality/data is documented, existing work is protected, unsupported
   next-plan assumptions were corrected and one bounded R1 objective is named.
4. **R1 evaluation:** remains in progress. The six required architecture
   artifacts now cover auth/quotas, streaming/realtime, RAG/KB, MCP/skills,
   model/backend lifecycle, distributed operation, React state and deployment
   modes. Per-route middleware/caller and backend license/lifecycle matrices
   remain before source acceptance.
5. **R2 reconciliation:** product definition, app-shell specification,
   navigation/role model, UI-state contract and white-label inventory were added;
   the four existing design drafts were corrected to the R2→R3→R4→R5→R6 order.
   R2 is not active or source accepted.
6. **Protected reclassification:** the truthful loading fix is
   `R4-PRE-01 — Truthful Case Workspace initialization`, source accepted and
   deployment/runtime pending. It does not complete R4 or authorize deployment.
7. **Preservation:** no runtime, data, model, agent, collection, container,
   image, volume or retained-configuration mutation occurred; no staging,
   commit, push or publication occurred.
8. **Next:** complete R1-AUD-01. Do not implement R6 Agent Chat lifecycle or
   deploy R4-PRE-01 during this work.

### 2026-08-06 - R1 source acceptance and R2 activation

1. **Exact source inventories:** generated a 371-route-file plus 10
   cross-service/static LocalAI registration inventory and a 64-backend
   platform/capability/disposition inventory from 261 parsed Linux/Darwin matrix
   entries. Both are machine-readable dated JSON reports.
2. **R1 exit criteria:** PASS at the material architecture boundary. Every major
   capability and backend identifier has a NexusAI disposition; R2 no longer
   depends on undocumented engine/auth/state/deployment assumptions; LocalAI and
   NexusAI ownership boundaries are explicit.
3. **Honest limitation:** R1 source acceptance is not endpoint security,
   dependency-license, model/backend promotion, deployment or runtime
   acceptance. Owning R-phases retain those tests and approvals.
4. **Active program:** R2 is now the sole Active Program Phase.
   `R2-DES-01 — Resolve product, asset, role and branding-configuration
   decisions` is the sole Active Work Item.
5. **R2 gap:** final assets, primary-tagline placement, role/administration
   terminology, branding/report configuration and remaining visible LocalAI
   disposition need product-owner decisions before R2 source acceptance.
6. **Preservation:** no runtime/data/model/agent/collection/container/image/
   volume/retained-setting mutation and no stage/commit/push/publication.
   `R4-PRE-01` remains source accepted and undeployed; R6 remains deferred.

### 2026-08-06 - R2 source acceptance and R3 source activation

1. **Execution-focus integration:** reuse-first APIs, focused phase-owned tests,
   just-in-time fixtures, phase-relevant Pakistan behavior, just-in-time model
   evaluation and ready-only PowerShell handoff are now cross-cutting R0-R17
   Definition-of-Done rules, not a separate workstream.
2. **R2 verdict:** source accepted. The code-native/default asset set, exact
   tagline/context placement, Analyst/System Administrator labels, navigation/
   Evidence/Knowledge/administration terminology, existing branding and report
   fallback contract, shell/state/token contract and visible LocalAI disposition
   are resolved. There is no genuine remaining R2 decision.
3. **Active program:** R3 is the sole Active Program Phase. `R3-UI-01 —
   Establish the branded, role-aware application shell` is the sole Active Work
   Item.
4. **R3 source value:** reusable NexusAI mark/lockup/signature; existing
   white-label override; exact metadata/tagline fallback; branded non-blank
   boot/login/route loading; Analyst/System Administrator footer context;
   Analyst Tools/Intelligence Tools/System Administration terminology; initial
   `--nx-*` tokens; focused login/navigation/responsive/focus browser contracts.
5. **API impact:** existing public branding, auth and features APIs are reused.
   No route, schema, service, model, agent, evidence or governed-data change.
6. **Verification:** focused ESLint reports zero errors; English navigation JSON
   parses; changed-file `git diff --check` passes. Browser tests were added for
   identity/metadata, drawer focus and 390/820/1024/1440 overflow but were not
   executed because a current-source build/preview is not authorized.
7. **Preservation:** no build/deploy, database/data/CDR operation, model/backend
   research/download, staging, commit, push or publication. The existing dirty
   worktree and `R4-PRE-01` remain protected.
8. **Next:** continue R3-UI-01 with unified desktop shell context, shared
   empty/error/unavailable states and remaining primary-route visible-brand
   dispositions; request the bounded build/preview gate only when ready.

### 2026-08-06 - R3 unified shell context and shared-state foundation

1. **R3-UI-02 source implementation:** desktop/tablet shell context now shows
   the current route, existing authenticated identity and only the enforced
   Analyst/System Administrator role. No-auth mode is labeled Local workspace.
2. **Truthful case context:** case routes show only the decoded case ID already
   carried by the URL. No name, classification, readiness, count or permission
   is inferred, and no new API is required.
3. **Responsive/accessibility:** mobile now shows compact route/case context;
   the shell adds a visible-on-focus skip link and focusable route-content
   target; authorization loading is branded instead of blank.
4. **R3-UI-03 started:** the existing EmptyState component preserves legacy
   callers and child content while adding empty/error/forbidden/partial/
   unavailable variants. The 404 and governed-case redirect states use it.
5. **Verification:** focused ESLint zero errors (13 warnings from the existing
   JSX/no-unused rule behavior); English nav JSON PASS; App CSS parse PASS;
   shell-route assertions PASS; changed-file diff check PASS; 23 focused
   Playwright tests discover across login/shell/navigation. Browser execution
   remains pending because no current-source build/preview is authorized.
6. **Preservation/API impact:** existing auth, branding, feature, router and
   governed-case contracts are reused. No build/deploy, runtime/data/CDR/model/
   agent/API/schema mutation, staging, commit, push or publication.
7. **Active work item:** `R3-UI-03 — Complete shell states and primary-route
   white-labeling`.
8. **Next:** migrate remaining primary analyst-route ad hoc states and visible
   brand copy, then prepare the bounded browser gate.

### 2026-08-06 - R3 primary-route state pass and acceptance handoff

1. **R3-UI-03 source complete:** Knowledge collection loading, failure and empty
   states now use the shared NexusAI state contract with a durable retry path;
   Talk exposes truthful realtime pipeline loading, failure and unavailable
   states; the non-admin Home no-model path uses the shared unavailable state.
2. **White-label disposition:** Knowledge copy is analyst-facing and visible
   manage-mode copy is contextualized as System administration. Internal
   `localai_*` storage keys, protocol flags, technical commands and legal source
   identity remain unchanged for compatibility and accuracy.
3. **Focused validation:** changed JSX and the Knowledge browser contract pass
   ESLint with zero errors (30 existing-rule warnings); changed English locale
   JSON parses; 12 focused shell/Knowledge Playwright contracts discover;
   changed-file whitespace validation passes.
4. **API/security/preservation:** no API, schema, backend, model, evidence,
   case, retained-state or runtime mutation. No build, preview, deployment,
   staging, commit, push or publication occurred.
5. **Phase status:** R3 remains `in_progress`. `R3-UI-04 — Browser/build
   acceptance and handoff` is the sole Active Work Item. Its production build,
   browser matrix and runtime evidence require explicit approval.
6. **Next:** after approval, run the bounded current-source build/preview and
   browser acceptance at 390/820/1024/1440, light/dark, keyboard/focus and
   asset/network/console gates. Close R3 only if those results pass.

### 2026-08-06 - R3 production-browser acceptance and R4 activation

1. **Production build:** approved `npm run build` completed successfully with
   Vite 8.0.16, 662 transformed modules and hashed production assets.
2. **Browser contracts:** after correcting two environment-sensitive test
   selectors (document focus entry and nested status roles), 27/27 focused
   login/loading/error/shell/Knowledge/navigation contracts passed against the
   production preview in installed Chrome.
3. **Interactive acceptance:** the hashed bundle (no `/@vite/client`) passed
   390/820/1024/1440 responsive checks with no settled horizontal overflow,
   correct mobile/desktop context switching, light/dark theme switching,
   NexusAI title/favicon, Knowledge and Talk route presentation and zero
   captured browser warnings/errors.
4. **Lifecycle:** the local preview was stopped after acceptance. No Docker
   image/container deployment, database or retained-state mutation, staging,
   commit, push or publication occurred.
5. **R3 verdict:** R3 is complete at its source/browser exit boundary.
   Deployment remains a separately approval-gated status and is not implied by
   phase closure.
6. **Program transition:** R4 is now the sole Active Program Phase.
   `R4-ARCH-01 — Reconcile and establish the single active-case contract` is
   the sole Active Work Item. Begin with read-only case-context and collection-
   coupling inventory; preserve R4-PRE-01 until separately deployed.

### 2026-08-07 - R4-ARCH-03 bound Records Intelligence source acceptance

1. **Scope authority:** embedded Records Intelligence derives case and
   collection only from the shared provider and fails closed without valid
   provider identity. Alternate collection selection/listing/creation is absent.
2. **Bound operations:** forensic and structured queries, case evidence,
   evidence upload, legacy ingestion, status, capabilities, batches, deletion
   and export use the active case/collection. Out-of-scope returned batches are
   filtered and out-of-scope deletion is blocked.
3. **No stale case state:** a case switch clears case-derived Records state;
   request version/scope guards discard late structured and forensic responses.
   A completed request also preserves a question edited while it was running.
4. **UI/UX:** embedded analysis visibly states that its active-case boundary is
   enforced, exposes the collection as read-only and retains specialist command
   surfaces. The boundary stacks cleanly at narrow widths.
5. **Verification:** focused ESLint has zero errors; Vite 8.0.16 builds 664
   modules; Records browser tests pass 11/11; the full protected shell/case/
   records/visual matrix passes 46/46 against the production preview in system
   Chrome.
6. **Preservation:** no API/schema/runtime/data/model/agent/container/image/
   volume/configuration mutation and no deployment, staging, commit, push or
   publication.
7. **Program transition:** R4 remains active. `R4-ARCH-04 — Bind Agent Chat to
   the active case` is the sole Active Work Item, bounded to identity/request/
   conversation-state scoping and explicitly excluding future R6 lifecycle work.

### 2026-08-07 - R4-ARCH-04 case-aware Agent Chat source acceptance

1. **Provider authority:** forensic Agent Chat uses the shared active case and
   authorized options. It no longer fetches its own case registry or falls back
   to persisted agent collection configuration for case-aware UI scope.
2. **Explicit request scope:** every forensic send supplies the provider's
   distinct `case_id` and `collection_id`. Invalid or unresolved cases disable
   input and issue zero chat requests.
3. **Case-isolated state:** browser conversations remain keyed by case and old
   messages are withheld during scope loading. Case changes clear transient
   processing, streaming and canvas state.
4. **Response correlation:** pending request mappings retain conversation plus
   case scope. Late HTTP completions and SSE status/stream/error/final events
   from a prior case cannot update the visible case.
5. **Compatibility/UI:** direct forensic entry resolves the governed current or
   default case into the URL; the ribbon displays case and collection; generic
   Agent Chat keeps its unscoped request and does not load the forensic registry.
6. **Verification:** focused ESLint has zero errors; Vite 8.0.16 builds 664
   modules; focused Agents/Chat browser tests pass 7/7; the protected production-
   browser matrix passes 53/53, including 390-pixel Agent Chat overflow.
7. **Preservation:** no endpoint/schema/runtime/data/model/agent/container/image/
   volume/configuration mutation and no deployment, staging, commit, push or
   publication.
8. **Program transition:** R4 remains active. `R4-ARCH-05 — Administrative
   transitions and reports` is the sole Active Work Item; retained report
   generation and data mutation remain outside this source boundary.

### 2026-08-07 - R4-ARCH-05 administrative transitions and reports source acceptance

1. **Administrative separation:** generic Collections and Collection Details
   now identify themselves as knowledge administration. Their detail/search/
   source/lifecycle behavior remains collection-scoped and never redirects
   through the implicit Records default.
2. **Explicit authorized transition:** an Open Case action exists only when the
   shared provider contains a selectable case whose `collectionId` exactly
   matches the current own-user collection. Other-user and unmapped collections
   receive no case action; names alone never establish identity.
3. **Report adapter:** Case Workspace now exposes an on-demand, non-retained
   Reports module with locked provider case/collection display, optional target
   and evidence previews, truthful warnings, Markdown preview and browser-
   session export. A case-keyed mount and request version prevent late prior-
   case results from appearing after navigation.
4. **API boundary:** the browser adapter always supplies both provider IDs and
   ignores caller overrides. The V1 URL-authoritative compatibility endpoint
   validates both body identifiers before applying the current same-ID mapping;
   a mismatch returns conflict without partial scope mutation.
5. **Verification:** focused ESLint reports zero errors; Vite 8.0.16 builds 664
   modules; the focused internal Go governance assertion passes 1/1. The CLI
   Playwright files were authored, but the local pinned Chromium 1208 binary is
   absent, so no CLI browser assertion executed in this session.
6. **Live preview acceptance:** the in-app production preview showed three
   active administrative collections and exactly one authorized Open Case
   transition. It navigated explicitly to `nexusai-forensic-demo`, generated a
   real 13-section deterministic report without retaining an artifact, emitted
   no console error and had zero page overflow at 820 and 390 pixels.
7. **Preservation:** no deployment, database/schema/evidence/model/agent/
   container/image/volume/configuration mutation, retained report, staging,
   commit, push or publication occurred.
8. **Program transition:** R4 remains active. `R4-ARCH-06 — Modular workspace
   acceptance` is the sole Active Work Item, bounded to truthful module
   composition and regression acceptance without implementing later R5/R6/R8+
   product capabilities early.

### 2026-08-07 - R4-ARCH-06 elite modular workspace source acceptance

1. **Complete modular desk:** the case experience now uses the approved
   Overview, Ask, Evidence, Relationships, Timeline, Media, Reports and Admin
   composition. One versioned module registry controls labels, icons,
   descriptions, navigation and valid-route resolution.
2. **Elite visual hierarchy:** a reusable command-center chrome separates the
   active investigation, selectable case, distinct authorized identifiers,
   module navigation and analytical stage. Desktop uses a quiet sticky module
   rail; tablet/mobile use a contained horizontal navigator. Visual density,
   borders, gradients, status accents and typography follow the accepted R3.1
   system without adding decorative noise.
3. **Truthful capability ownership:** Ask embeds the current deterministic
   Records Intelligence boundary rather than claiming future orchestration.
   Timeline links only to recorded-event workflows and forbids interpolated
   chronology. Media reports registered image/audio/video inventory and states
   explicitly that registration is not OCR, transcription, detection or
   validated analysis.
4. **Administrative consolidation:** Admin combines processing history,
   governance metadata, resource accounting and protected advanced controls.
   Destructive controls remain disabled. Reports preserves the R4-ARCH-05
   on-demand/non-retained scope contract.
5. **Compatibility and isolation:** legacy `/analyze`, `/jobs` and `/settings`
   paths redirect to `/ask` or `/admin`, preserving query strings. Case changes
   retain the selected canonical module and all existing stale-response/fail-
   closed protections.
6. **Verification:** focused ESLint reports zero errors; Vite 8.0.16 builds 667
   modules. The complete protected system-Chrome matrix passes 61/61 across
   login, navigation, App Shell, visual system, Case Workspace, Records
   Intelligence, Agent Chat and Collections.
7. **Interactive acceptance:** the production bundle against the current local
   APIs rendered the full governed case and all eight exact accessible module
   names. Desktop at 1440 used the sticky rail; mobile at 390 collapsed cleanly
   with an internally scrollable module navigator, zero page overflow and zero
   captured console errors.
8. **Preservation/program transition:** no API/schema/data/model/agent/container/
   image/volume/configuration mutation, deployment, staging, commit, push or
   publication occurred. R4 is source accepted. `R4-DEPLOY-01 — Guarded
   combined refresh and runtime acceptance` is now the sole active item and is
   blocked on explicit rebuild/deployment approval; R5 does not begin early.

### 2026-08-07 - R4-DEPLOY-01 guarded refresh and fail-closed runtime progress

1. **User-run activation:** the retained Phase 6 gate transcript ends in PASS:
   6.12 GiB free RAM, API 3.7 seconds, worker 7.5 seconds, LocalAI 114.5
   seconds, one LocalAI attempt, rollback images preserved and volumes
   preserved.
2. **Runtime health:** LocalAI `/readyz`, forensic API `/healthz`, NATS
   `/healthz` and LocalAI `/app` return 200. LocalAI, worker, PostgreSQL and
   NATS are running healthy; the API is running with its documented HTTP health
   route.
3. **Live product acceptance:** the deployed `index-BCL3QJ6m.js` bundle renders
   the exact eight-module desk for `nexusai-forensic-demo`, with matching case
   and collection IDs, 9,272 rows, zero 1440/390 page overflow and zero captured
   console errors.
4. **Fail-closed defect:** the deployed protected matrix passed 60/61. The one
   failure—skip navigation not reliably transferring focus—reproduced in an
   isolated run and was not waived.
5. **Bounded correction:** App Shell skip navigation now explicitly focuses and
   scrolls the existing main route target. Focused ESLint has zero errors, Vite
   builds 667 modules, the focused corrected contract passes 1/1, and the full
   corrected production-bundle matrix passes 61/61.
6. **Program status:** R4 remains active and source accepted. The corrected
   bundle requires a second explicitly approved guarded refresh and final live
   acceptance before R4 can become complete or R5 can begin. No evidence,
   report, database, model, configuration or collection state was changed.

### 2026-08-07 - R4 closure and R5-EVID-01 evidence operations source acceptance

1. **R4 runtime closure:** the corrected user-run gate passed with 7.22 GiB free
   RAM, one LocalAI build attempt, rollback images and volumes preserved. The
   deployed `/assets/index-BY8o0UmF.js`, isolated skip-focus contract 1/1 and
   protected matrix 61/61 are accepted; R4 is complete.
2. **Case-scoped API:** `GET /api/v1/forensics/cases/:case_id/evidence/:evidence_id`
   validates the URL case and mapped collection before forwarding trusted scope;
   it fails closed when case authority is unavailable.
3. **Evidence Operations UI:** the new responsive master/detail desk exposes
   truthful catalog state, integrity, exact row accounting, immutable runs,
   lineage, artifacts and actionable notes. Multi-file intake is staged safely.
4. **Truth boundary:** no custody timeline is invented because append-only
   custody events are not yet published by the case evidence API. Upload and
   reprocessing were not executed.
5. **Verification:** focused ESLint has zero errors; Vite built 669 modules;
   focused endpoint and auth tests pass; Case Workspace passes 12/12; the
   protected production-preview matrix passes 62/62. Interactive 1440/390 QA
   reports zero overflow and zero console errors.
6. **Next:** R5 remains in progress. R5-EVID-02 adds append-only custody-event
   publication, stable citation locators, generated Swagger closure and their
   truthful UI flows source-only. Deployment and retained ingestion remain
   separately approval-gated.

### 2026-08-07 - R5-EVID-02 custody, citations and processing-depth source acceptance

1. **Retained schema reuse:** the existing evidence detail transaction now
   reads canonical processing runs/events, derived artifacts and append-only
   custody events; every query is tenant/collection/evidence scoped.
2. **Stable citations:** derived artifacts expose exact JSON locators plus
   `nexusai://evidence/{evidence_id}/artifacts/{artifact_id}` references. No
   artifact content or locator is synthesized.
3. **Custody integrity:** the API compares every recorded previous-event hash
   with its ordered predecessor and returns whole-chain event/broken-link counts
   and first/last times alongside bounded event history.
4. **Elite evidence audit UI:** artifacts use a compact selector and exact
   locator panel; canonical runs/events use a dense pipeline ledger; custody
   uses a verified chronology or an explicit unavailable state.
5. **Verification:** focused provenance/accounting/privacy Go tests pass; ESLint
   has zero errors; Vite builds 669 modules; Case Workspace passes 12/12; the
   protected matrix passes 62/62; detailed desktop/mobile overflow is zero.
6. **Runtime boundary:** the live-data preview accurately reports Resource not
   found because the R5 detail adapter is not deployed. No live custody or
   upload-to-ready acceptance is claimed.
7. **Next:** R5-EVID-03 adds scalable pagination, bounded concurrent bulk
   registration, exact per-file outcomes and queue observability source-only.
   Swagger generation, deployment, uploads and reprocessing remain gated.

### 2026-08-19 - STIM-3/STIM-4 runtime-closure P1 source correction

1. **Preserved authority:** `APF-3FinalStatus=CLOSED`; STIM-0 is accepted and
   STIM-1/STIM-2 remain source accepted. This is a bounded STIM-3/STIM-4
   runtime-closure correction. STIM-5 did not begin.
2. **P1-A root cause and correction:** multi-CDR validation previously checked
   only source-set syntax. The analytical query filtered unknown or mismatched
   evidence/version rows, allowing HTTP 200 with a silently incomplete source
   set. The API now performs a read-only exact tenant/collection/source ID/
   filename/CDR-family/evidence/version membership preflight. Invalid retained
   membership returns HTTP 400 with stable code `invalid_source_membership`
   and a generic non-enumerating message. Focused injection tests prove the
   analytical query call count stays zero for every rejected membership class.
3. **P1-B entity-index audit:** `forensic.record_entities` is written in the
   worker's canonical ingest transaction and has tenant/collection/file/batch/
   family/row/hash/time/entity/value/source-field/source-file linkage plus an
   indexed `(tenant_id, entity_type, entity_value, observed_at)` access path.
   Retained audit found no stale rows. CDR has typed phone/IMSI/IMEI/cell/
   location projection; non-CDR generic projection intentionally carries only
   primary/secondary/location, so index-only execution would lose semantic
   fields such as subscriber IMEI/IMSI.
4. **P1-B correction:** the runtime now executes one bounded, parameterized,
   read-only candidate query. It uses allowlisted `record_entities` first and
   one non-CDR normalized-field fallback expansion for semantic parity, then
   looks up exact retained records/evidence/version and compiles matches,
   coverage, relationships, citations and limitations once in Go. Tenant,
   collection, optional source/batch, family, date and 5,000-row scopes remain;
   5,001 fails closed. Unresolved time remains null. The independent typed
   validator continues to own entity-type, source-distinctness and validity-
   interval semantics for CDR↔subscriber, CDR↔tower, IPDR↔subscriber,
   CDR↔IPDR and ANPR↔ANPR. No graph, cache, table, migration or index was added.
5. **Semantic and performance source evidence:** all 14 focused STIM specs pass.
   The 1,000-iteration multi-CDR compiler took 94.1471 ms and the controlled
   5,000-target-record cross-family compiler took 63.3869 ms. These are source-
   host compiler timings, not p50/p95 or deployed DB/API claims. The existing
   analytical request deadline remains 30 seconds; corrected runtime EXPLAIN,
   DB and API timings are still required.
6. **Acceptance-data pack:** seven new, clearly synthetic and human-auditable
   sources under
   `ingestion/forensic_records/tests/fixtures/stim4_runtime_acceptance/` cover
   three-source CDR intersection/unique/reciprocal/one-way/device/SIM/tower/
   exact-overlap/duplicate/conflict/no-evidence semantics, IPDR↔subscriber
   valid/future/type-collision/no-match cases, and exact/unique/nonmatching
   distinct-source ANPR plates. The independent contract is
   `api/forensic_records/contracts/stim4-runtime-acceptance-oracle-v1.json`.
   Offline auto-routing/accounting and oracle tests pass. No file was uploaded.
7. **Regression:** structured ingestion passes 102 tests with the same two
   intentional missing-reference skips. The complete forensic Go package
   passes in 24.563 seconds; `go vet ./api/forensic_records` passes. The
   66-operation ledger remains 5 queryable/certified, 50 limited, 11
   engineering-only, and zero silently uncertified ordinary-user operations.
   Frequent contacts, temporal activity, tower validity, IPDR time equality,
   ANPR source distinction, anti-correlation, citations and the 200-observation
   provenance bound remain covered by the unchanged full suite.
8. **Prepared future gate:** the existing
   `scripts/build_deploy_nexusai_r7_10_api_gate.ps1` remains the authoritative
   API-only mechanism: it builds/recreates only `forensic-records-api`, keeps
   `--no-deps`, preserves a rollback image and verifies health. The new guarded
   `scripts/ingest_nexusai_stim4_runtime_acceptance.ps1` uses only normal
   authenticated multipart admission and refuses to run without `-Approved`.
   It targets the existing dedicated `nexusai-runtime-acceptance-20260730` /
   `runtime-acceptance-20260730` scope, waits for worker completion and checks
   exact 21 total / 20 accepted / 1 duplicate / 0 rejected accounting. Neither
   script was executed.
9. **Deferred issues:** `STIM-4-P2-STALE-ELIGIBILITY-METADATA` requires read-
   only regeneration after approved retained ingest. Partial aggregate
   provenance presentation stays `STIM-6-P2`; advertised family availability
   with zero indexed records stays `STIM-7-P2`. No source-result provenance
   loss was introduced.
10. **Status:** `STIM-3SourceStatus=ACCEPTED`,
    `STIM-4SourceStatus=ACCEPTED`, both runtime correction source statuses are
    `ACCEPTED_PENDING_ACTIVATION`, and both runtime statuses are
    `BLOCKED_P1_PENDING_CORRECTED_ACTIVATION`. `OpenP0=0`, `OpenP1=2`,
    `DatabaseMigrationNeeded=NO`, `DeploymentNeeded=YES`, and
    `AcceptanceDataIngestNeeded=YES`. No build, deployment, restart, migration,
    backfill, retained ingest/reprocess/delete, model/profile change, download,
    stage, commit or push occurred.

### 2026-08-20 - lawful Pakistan media acquisition and non-retained quality baseline

1. **Bounded lawful acquisition:** selectively retrieved two Google FLEURS
   `ur_pk` WAV rows and source transcripts under CC BY 4.0 plus the Wikimedia
   Commons `Road and cars in Islamabad.jpg` by Executioner under CC BY-SA 3.0.
   External downloads total 2,851,891 bytes. Mozilla Common Voice Urdu was
   rejected because its current scripted release is multi-gigabyte. No archive,
   executable, corpus, model, private recording or random social-media clip was
   downloaded.
2. **Canonical inputs:** prepared ignored 16 kHz mono PCM derivatives at
   `local-acceptance-inputs/pakistan-audio/`, one locally generated and clearly
   synthetic sequential Urdu/English identifier smoke, and a 10.5-second H.264/
   AAC derived Islamabad-image/FLEURS-audio composition. The manifest preserves
   originals, hashes, source URLs, licenses, transcripts and conversion steps.
   Pakistani-English, natural code-switch and spontaneous conversational Urdu
   remain pending rather than receiving fabricated provenance.
3. **Audio runtime evidence:** the deployed `whisper-tiny` worker emitted
   timestamped observations for all three audio clips in 10.122, 13.103 and
   22.829 seconds. Under Unicode NFKC/casefold/punctuation-normalized scoring,
   clear Urdu WER/CER was 1.0000/1.0617 and second-speaker read Urdu was
   1.4583/1.3012. The synthetic identifier fixture scored 1.0000/0.8696 and did
   not preserve its exact identifiers. `AudioPipelineOperational=YES`, but
   `PakistanUrduASRPracticalBaseline=FAIL` and `IdentifierPreservation=FAIL`.
4. **Video runtime evidence:** the lawful derived video passed the non-retained
   composition contract in 14.246 seconds with frames at 0/5 seconds, five
   unique observations, embedded audio, one timestamped ASR segment and exact
   temporary cleanup. ANPR was explicitly optional for this general road image
   and returned zero plate observations. This is composition evidence, not
   real-world video accuracy evidence.
5. **Preservation and gate:** PostgreSQL counts remained exactly `19|20|0`
   before and after. No upload, retained ingest, reprocess, migration, model
   download, deployment, restart, training, staging, commit, push or PR
   occurred. The seven-input isolated retained diagnostic pack is prepared but
   requires operator media/truth/license review, confirmation that the external
   `test_plate.jpg` is lawfully usable, and explicit retained-mutation approval.

### 2026-08-23 - post-BF-A ANPR and face runtime activation

1. **FastALPR preserved and activated:** the existing worker path remains the
   ANPR implementation. A non-retained run against the user-owned
   `test_plate.jpg` returned `MN1367`, detector confidence `0.8897411227` and
   OCR confidence `0.9997855306` with the bounded crop/provenance contract. No
   replacement ANPR model was downloaded.
2. **Face model acquisition:** LocalAI gallery model
   `face-detect-yunet-sface` was acquired from the pinned upstream revision.
   `/models/face-detect-yunet-sface.gguf` is 26,073,536 bytes and its verified
   SHA-256 is
   `9ce78d4ba0ae9d5e8c91a0e145d511558d1d90f5d9c1f4131cca9bb4bce60902`.
   The accompanying CPU backend metadata digest is
   `sha256:8d303ecedb5e6f4dd2c5a0023da806ac3f8ceef369ff83628f24d99d1a0f3f19`.
   The installer warned that the OCI backend lacked signature verification;
   checksum provenance passes, supply-chain signature verification does not.
3. **Lawful synthetic benchmark:** six clearly synthetic fixtures cover clear,
   pose, blur, different subject, four-person group and no-face truth. Counts
   were exactly `1/1/1/1/4/0`; embeddings were 128-dimensional. Same-subject
   cosine was `0.83675894`, blurred same-subject `0.89980385`, and different
   subject `0.28379574`. Cold detection was 3,028 ms, warm detection roughly
   335-622 ms, and process memory rose from 42.38 MiB to 338.3 MiB. Verdict is
   `FaceDetection=M1_AVAILABLE`, `FaceSimilarity=LIMITED`, never identity.
4. **Architecture and security:** LocalAI now publishes detection-only
   `POST /v1/face/detect`. The media worker creates original-pixel bounded
   face observations, lossless-crop hashes, model/backend provenance and
   candidate embeddings. `GET /faces/similar` requires authenticated tenant,
   collection/case scope, `explicit_case_evidence_scope`, and 1-200 explicitly
   enumerated candidate evidence IDs; current-version SQL and RLS remain
   server-owned. No LocalAI 1:N registry, demographics, global gallery or
   cross-tenant scan is used.
5. **Narrow activation:** rollback tags were created for LocalAI, forensic API
   and worker. Only those three changed services were rebuilt/recreated.
   PostgreSQL ID `f66e05a3b179...` and NATS ID `5c49e70d132a...` remained
   unchanged. New LocalAI/API/worker IDs are `13104628593f...`,
   `c33fc7903ef4...`, and `c0838cf0bbbe...`; LocalAI and worker are healthy.
   Worker roles are `ANPR=true`, `FACE=true`, `ASR=true` with
   `faster-whisper-small-ur`; model and spool mounts remain read-only.
6. **Live non-retained proof:** the rebuilt LocalAI returned one face at
   confidence `0.9517` and a 128-dimensional embedding for the synthetic clear
   fixture. A disposable worker run returned `forensics.face-observation/v1`,
   128 dimensions, the exact `candidate visual similarity; not identity`
   semantics, and a truthful zero-observation/no-face result. Authenticated
   negative checks returned 403 without explicit scope, 400 without candidate
   evidence IDs, 403 cross-tenant, and 404 for a fully scoped absent query.
7. **Preservation:** before and after activation the database remained exactly
   27 evidence, 27 jobs, 15 artifacts, 12,932 records and 25 KB assets with
   zero queued/running/failed jobs. No retained upload/reprocess, migration,
   deletion, cleanup, stage, commit, push or PR occurred. Existing BF-A review
   hold remains intact.
8. **Next boundary:** exact/near-duplicate and image-comparison live acceptance
   comes next. General image OCR and semantic image embedding remain distinct
   missing roles pending one recorded, bounded model proposal per role. New
   retained acceptance evidence still requires a separate approval boundary.

### 2026-08-23 - post-BF-A advanced multimodal reconciliation

1. **Bounded acquisitions:** added one Apache-2.0 general image role,
   `google/siglip-base-patch16-224`, revision
   `7fd15f0689c79d79e38b1c2e2e2370a7bf2761ed`, 812,672,320-byte weight SHA
   `2c63cb7d1f2e95ba501893cbb8faeb4ea9a3af295498d35097126228659c2af8`,
   plus official Apache-2.0 Tesseract fast `eng+urd` data at revision
   `65727574dfcd264acbb0c3e07860e4e9e9b22185`. English/Urdu asset hashes are
   `7d4322bd...` and `62e8250c...`. Cumulative acquired model assets stayed
   below the authorized 2 GB ceiling; no duplicate model or generative VLM was
   downloaded.
2. **Image/OCR M1:** SigLIP produces 768-dimensional bounded candidate
   embeddings. Same/blur cosine was `0.9358`, same/pose `0.9668`, different
   fixture `0.7852`, and portrait/street `0.4630`; English text-image top-1 was
   1.0 while the Urdu street query failed top-1. Tesseract synthetic English
   CER was `0.0189`, Urdu CER `0.0741`; blank returned zero observations and
   corrupt/oversized controls failed closed. Verdict for both roles is
   `M1_ACCEPTABLE_LIMITED`.
3. **Scoped APIs and capability truth:** `/images/similar`, two-image comparison
   and `/faces/similar` use authenticated tenant/collection/case scope and
   explicit bounded candidates. Capability catalog
   `2026-08-23.post-bfa-runtime.1` now counts exact persisted artifact types.
   The live BF-A case therefore suggests image metadata/recorded plate
   candidates and timestamped Urdu, but not unretained OCR, SigLIP, face, Roman
   Urdu or document passages.
4. **Roman Urdu and documents:** a disposable network-disabled Roman control
   preserved `03001234567`, `MN1367` and `10:35`, with raw Urdu authoritative.
   The user-owned HP DOCX produced 112 source-bound passages and 5,371
   characters with evidence/version/hash locators; its source SHA remained
   `85b1ab53150a556f84391a222b7fb4dbf69aa80d1c4d7d7a7b54d5f283ea4865`.
   No retained Roman or document artifact was written.
5. **Analyst portal:** Home/Data/Ask/History now render previews and
   contract-specific derived cards, enforce the exact phrase `candidate visual
   similarity; not identity`, and separate old immutable processing notes from
   current live readiness. The final deployed browser matrix passed all four
   routes at 390/820/1024/1440 with main/navigation present, no horizontal
   overflow and no console error. A legacy saved BF-A analysis truthfully
   reports `citation state not reported`; the UI does not fabricate a citation.
6. **Verification:** full `go test ./api/forensic_records -count=1` passed,
   `go vet ./api/forensic_records` passed, focused Phase5A/5B and source suites
   passed, worker media tests passed 20/20, document tests passed 6/6, and the
   production React build passed with 677 modules. Final bundle is
   `AnalystData-TBR-3eWU.js`.
7. **Narrow deployment:** final LocalAI image is
   `sha256:5647f78f271c2218cf26d84eb5e89e0499910d6ca9604e8412493f64a4ee162d`
   and final forensic API image is
   `sha256:501e8342c65239bf329ae61b738c8950e6c7303d026f5e7495284f02d20be56f`.
   LocalAI container is `e5548e0884ad...`; forensic API is `4bf1d01cea79...`.
   Worker `d7605c33967c...`, PostgreSQL `f66e05a3b179...` and NATS
   `5c49e70d132a...` are unchanged and healthy. Rollback tags exist for every
   rebuilt generation.
8. **Final preservation:** a read-only transaction returned exactly `27`
   evidence, `27` versions, `27` jobs, `12,932` records, `15` artifacts and
   `25` KB assets; job states are `24 completed | 3 dead_letter`. No retained
   upload/reprocess, migration, deletion, cleanup, broad replay, model-volume
   replacement, stage, commit, push or PR occurred.
9. **Reconciliation artifacts:** the complete §74 response is recorded at
   `reports/post-bfa-multimodal-20260823/post-breadth-reconciliation.md`; the
   updated operator guide is
   `docs/demo/nexusai-breadth-multimodal-demo-guide.md`; and the exact proposed
   seven-evidence retained gate is
   `reports/post-bfa-multimodal-20260823/retained-proof-approval-manifest.md`.
10. **Stop boundary:** `OpenP0=0`, `OpenP1=0` inside the authorized non-retained
    activation slice; `ModelDownloadNeeded=NO`, `DatabaseMigrationNeeded=NO`,
    `DeploymentNeeded=NO`, `RetainedMutationNeeded=YES_SEPARATE_APPROVAL`.
    Stop at Post-Breadth Reconciliation. Do not run the proposed retained jobs
    or begin P2/P3 capabilities without new authority.

### 2026-08-23 - product-demo media preview P1 correction and unified manifest gate

1. **Root cause and source correction:** valid case-scoped image/audio/video
   content requests returned 404 because the forensic content handler queried
   RLS-protected evidence without setting `app.tenant_id`. The handler now uses
   a read-only transaction, sets the tenant context locally, resolves the exact
   tenant/collection/case/evidence tuple, verifies the immutable content object,
   and serves it with byte-range support. TXT/PDF/DOCX content policies are
   explicit: text and PDF may render inline, DOCX is a governed attachment, and
   unsafe/unsupported document types fail closed.
2. **Analyst Data presentation:** the source drawer now offers truthful native
   TXT and PDF review, a governed DOCX source action plus retained-passage
   summary, and an explicit fallback for unsupported formats. No filesystem
   path is exposed and no browser-native DOCX renderer is claimed.
3. **Verification:** full `go test ./api/forensic_records -count=1` and
   `go vet ./api/forensic_records` passed; the React production build passed
   with 677 modules. Live proxy checks returned 200 for full JPEG content and
   206 with correct `Accept-Ranges`, `Content-Range`, MIME and length for JPEG,
   WAV and MP4. Wrong-case and missing-evidence requests returned 404.
4. **Narrow activation:** rollback tags
   `nexusai/forensic-records-api:rollback-before-demo-preview-scope-20260823`
   and `nexusai/localai-forensic:rollback-before-demo-preview-scope-20260823`
   were created. Only `nexusai-forensic-records-api-1` and `nexusai-api-1`
   were rebuilt/recreated. Current IDs are `62d9d475ad0a...` and
   `7f41a8378012...`. Worker `d7605c33967c...`, PostgreSQL
   `f66e05a3b179...`, and NATS `5c49e70d132a...` remained unchanged.
5. **Browser proof:** the live Analyst Portal visibly rendered the retained
   `test_plate.jpg`, native controls for `clear-urdu.wav`, and a decoded frame
   for `pakistan-short-video.mp4`. The image, audio and video were delivered
   only through the authenticated case-scoped content proxy.
6. **Preservation:** post-activation counts remain exactly 27 evidence,
   27 versions, 27 jobs, 12,932 records, 15 derived artifacts and 25 KB assets.
   No retained upload/reprocess, deletion, cleanup, migration, model download,
   broad replay, stage, commit, push or PR occurred.
7. **Unified acceptance preparation:** created the validated 21-source
   machine-readable proposal at
   `local-acceptance-inputs/NEXUSAI_MULTIMODAL_PRODUCT_ACCEPTANCE_MANIFEST.json`.
   It covers the coherent structured seed, seven image/face/ANPR candidates,
   three Urdu audio sources, one video, TXT, a deterministic text-bearing PDF,
   DOCX and KB expectations in target case
   `nexusai-multimodal-product-acceptance`. All 19 local path hashes match; two
   existing lawful retained sources are bound by evidence ID and SHA-256.
8. **Stop boundary:** both the prior seven-evidence manifest and the new unified
   manifest require an exact accountable actor. No actor was supplied in the
   current directive, so retained mutation remains blocked. Do not infer the
   actor, create the target case, copy/upload evidence, start jobs, or run
   retained Ask/History acceptance until the operator supplies the actor and
   explicitly approves the exact manifest phrase.

### 2026-08-23 - unified multimodal acceptance execution paused at ANPR P1 recovery gate

1. **Approval and baseline:** the product owner approved the exact 21-source
   manifest for tenant `default`, collection/case
   `nexusai-multimodal-product-acceptance`, actor
   `nexusai-breadth-acceptance-operator`. All 19 local paths and two retained
   source identities revalidated against size/SHA-256. The target began with
   zero evidence and zero jobs; the global baseline was 27 evidence, 27
   versions, 27 jobs, 12,932 records, 15 artifacts and 25 KB assets.
2. **Controlled execution:** the guarded executor
   `scripts/ingest_nexusai_unified_multimodal_acceptance.ps1` enforces the
   manifest actor, exact 21 hashes, 21 evidence/job ceilings, zero reprocess
   jobs, no unmanifested source and family-ordered stop-on-failure processing.
   It uses only normal authenticated multipart admission and preserves phase
   reconciliation files.
3. **Structured phase:** seven exact structured sources completed with 9,264
   accepted canonical records: CDR 5,000; IPDR 2,500; ANPR 750; access log
   1,000; subscriber 5; tower/location 5; transaction 4. The target therefore
   has seven completed jobs and source/version provenance for every structured
   family in the approved core.
4. **ANPR image P1:** the first media source, immutable evidence
   `a0ff7b18-129f-4a8a-82cf-48887f98bd9e` (SHA-256
   `a15a7ec947fdb2c5a08a70243a62ed66e4dddd89defaba9d135f6c5ac67559f6`),
   failed in `persist_anpr_media_observation` because PostgreSQL could not infer
   parameter `$12` inside `jsonb_build_object`. The executor stopped before the
   Islamabad image, all face/audio/video sources and all documents.
5. **Bounded correction:** `derived_observation_id` is now explicitly cast as
   PostgreSQL text. Two focused worker regression tests pass. Rollback image
   `sha256:376bbf162777...` was retained; only the worker was rebuilt/recreated.
   Active worker image is `sha256:f338458e0830...`, container
   `b1a695631a3f...`, healthy. LocalAI, forensic API, PostgreSQL, NATS, models,
   volumes and existing evidence were not rebuilt or replaced.
6. **Retry exhaustion and exact partial state:** the old worker exhausted the
   job's final automatic retries before the replacement could claim it. Job
   `2e527cd5-02b4-4745-b847-4f31dad83c57` is retained as `dead_letter`.
   Current target totals are 8 evidence, 8 versions, 8 jobs, 7 completed, 1
   dead-letter, 9,264 records, 0 derived artifacts and 7 KB assets.
7. **Stop boundary:** the original manifest explicitly authorizes zero
   reprocess jobs. No database status edit, duplicate/force upload, reprocess,
   further evidence admission, cleanup or deletion was performed. Exact
   one-job recovery scope is recorded at
   `reports/unified-multimodal-product-acceptance-20260823/P1_RECOVERY_APPROVAL_MANIFEST.json`.
   Resume only after the product owner approves that exact phrase.

### 2026-08-24 - MMV-2 guarded narrow activation and current-data certification

1. **Activation PASS:** the operator executed the guarded post-reboot MMV-2
   procedure. LocalAI is `0405fef3a5c5...` on image
   `sha256:f08bb832d18e...`; the worker is `1846f558941f...` on image
   `sha256:be13dbec4bd9...`. The deployed media-pipeline SHA-256 is the expected
   `76abd76a8d6c...`. Existing rollback tags remain valid.
2. **Protected runtime preserved:** forensic API `a97e545b8356...`, NATS
   `5c49e70d132a...` and PostgreSQL `f66e05a3b179...` did not change. LocalAI,
   forensic API, worker metrics, NATS, UI root and Analyst Portal all return
   HTTP 200; PostgreSQL accepts connections and the worker restart count is 0.
3. **Live public media reads:** bounded case-scoped face similarity returned two
   cited candidates, semantic image similarity three cited candidates, image
   comparison one result, and image/audio/video range delivery returned 206
   with the correct MIME types. The current retained video exposes 44 artifacts
   and 40 OCR observations at retained 0/5-second timestamps, but zero retained
   ANPR groups; no reprocess was authorized or run.
4. **Queries and citations:** retained plate `MN1367` returns one exact cited
   `anpr.sightings` row in English, natural English and Roman Urdu. A typed absent
   plate returns bounded `no_results`. Urdu-script ANPR phrasing routes to
   clarification rather than `anpr.sightings`. The positive citation preserves
   evidence, version, row number/hash/timestamp and source filename.
5. **History/browser/audio:** analysis `95a61f9d-a03b-4d96-8802-16f8fe987d6e`
   was saved and reopened by ID with one citation, then rendered in live History.
   Browser acceptance passes at 390/820/1024/1440 without horizontal overflow.
   Audio presentation passes with its waveform icon, visible loaded native
   controls, successful 6.24-second WAV playback, and source-cited Urdu/Roman
   Urdu cards.
6. **Final preservation:** counts remain exactly
   `51 evidence | 51 versions | 64 jobs | 0 active | 22,207 records | 441 artifacts | 47 KB assets`.
   No sample upload, retained reprocess, model change, migration, cleanup,
   stage, commit, push or PR occurred.
7. **Honest exit status:** `MMV2Activation=PASS`,
   `CitationCertification=PASS`, `HistoryCertification=PASS`,
   `BrowserAcceptance=PASS`, and `AudioPresentation=PASS`.
   `PublicMediaOperations=PARTIAL` because grouped video ANPR is not a public
   executable operation; `MediaQueryCertification=PARTIAL` because Urdu-script
   ANPR routing needs clarification. `OpenP0=0`, `OpenFoundationalP1=2`.
   Evidence is in
   `reports/mmv2-anpr-image-video-20260824/post-activation-certification-20260824.json`.
   Do not begin MMV-3. Any new source correction/deployment or retained positive
   video proof requires a separate approval.

### 2026-08-25 - MMV-2 enterprise result-semantics source closure

1. **Two P1 source defects fixed:** authoritative operation rows/counts now
   drive result presence, so positive ANPR rows cannot be summarized as
   no-match. Grouped-video `complete_zero` is preserved publicly as
   `complete_zero_results` with processing state, row count and operation ID.
2. **Typed taxonomy:** source distinguishes `results_present`,
   `complete_zero_results`, `no_match_for_filter`, `not_processed`,
   `processing`, `failed`, `unavailable`, `unauthorized`, and
   `invalid_request`. Authorization failures remain authorization failures.
3. **Presentation/History:** LocalAI metadata forwards the typed states; Ask and
   History render them separately. Completed-zero and no-match History reopen
   acceptance passes at 390, 820, 1024 and 1440 pixels without overflow.
4. **Verification:** full forensic API tests, focused agent presentation tests,
   Go vet, scoped UI lint, React build and focused Chromium acceptance pass.
   Full agent testing is 105 pass/13 unrelated Windows testcontainer setup
   failures; repository-wide UI lint retains six unrelated pre-existing errors.
5. **Runtime unchanged:** no build, restart, recreation, retained analysis,
   upload, reprocess, model operation, migration, cleanup or Git publication
   occurred. Counts remain `51|51|64|0|22207|441|47`.
6. **Approval boundary:** source-open P1=0; live-open P1=2 until activation and
   certification. Rebuild only `forensic-records-api` and LocalAI/UI `api`.
   Protect worker `1846f558...`, NATS `5c49e70d...`, and PostgreSQL
   `f66e05a3...`. New rollback tags must capture current API image
   `sha256:abce6567...` and LocalAI image `sha256:13b37779...`; older final-P1
   tags resolve to the preceding generation. The 6 GiB RAM gate remains.
   Do not begin MMV-3.

### 2026-08-25 - NX-R0 vNext authoritative reconciliation closure

1. **Authority reset:** the user's vNext reconciliation directive supersedes the
   former immediate MMV-3/post-STIM start. The sole current roadmap and ledger
   are now `docs/roadmap/nexusai-next-generation-roadmap.md` and
   `docs/roadmap/nexusai-next-generation-phase-ledger.md`; older roadmap/ledger
   content remains historical evidence.
2. **Read-only baseline:** branch
   `codex/forensic-hybrid-checkpoint-20260723`, HEAD
   `40717b83510c08db25dc26b9d6674bf46db363ac`. Git contains a single imported
   root snapshot and no LocalAI upstream remote, so an exact upstream merge-base
   is not provable. The extensive pre-existing dirty worktree was preserved.
3. **Runtime truth:** LocalAI/UI API, forensic API, protected worker, NATS and
   PostgreSQL were healthy. Retained counts were
   `51|51|64|0|22207|441|47`; jobs were 60 completed and four dead-letter.
   Five configured models and four installed backend families were observed.
   No runtime, retained data, model, configuration, migration or Git mutation
   occurred.
4. **LocalAI reconciliation:** official upstream v4.9.0 (2026-08-20) is the
   comparator. Local runtime exposes Nexus snapshot `40717b8`, not a semantic
   LocalAI version. Well-known/agent/Responses surfaces exist, but
   `/v1/models/capabilities` returns 404; local v4.9 equivalence is therefore
   unproven and no upgrade was attempted.
5. **Current maturity:** 67 operation templates comprise five A-certified, 51
   B-limited and 11 C-engineering operations. CDR core is mature; structured
   secondary families and media are limited; financial/log/document/advanced
   audio/vision/entity-graph work needs bounded certification slices.
6. **Live defect ledger:** P0=0, live P1=3. Public authoritative-row/count and
   grouped-video typed-zero defects are source-resolved but live-open. UUID
   segment `BAB0` misclassification as an ANPR plate remains source/live-open.
7. **UX evidence:** Home/Data/Ask/History has a sound responsive base and no
   document overflow at 390/820/1024/1440, but default flow leaks internal
   template/route vocabulary and loses typed media state. Urdu/RTL remains an
   explicit component-level certification gate.
8. **Exact next approval:** NX-1 source only—make typed UUID/evidence extraction
   precede family regexes, add the regression oracle, and run focused source
   verification. Activation is separately approval-gated and requires at least
   6 GiB free RAM, new immutable rollback tags/digests, protected worker/NATS/
   PostgreSQL, minimum API + LocalAI/UI rebuild, and live certification of all
   three P1s. Do not deploy, download models, migrate/reprocess data, or begin
   NX-2 without explicit approval.

### 2026-08-25 - NX-1 breadth-first directive ingestion and source closure

1. **Latest strategy ingested:** the authoritative program is now breadth before
   depth. The controlling sequence is NX-1 → NX-UX1 standalone Investigation
   Workspace → NX-A1 shared typed execution contracts → NX-B1 all-major-family
   breadth → NX-B2 demo/baseline certification, then justified depth phases.
   Structured, document, image, audio, video and knowledge families advance
   together; operation count is not the baseline measure.
2. **Authority updates:** the current roadmap/ledger, repository STIM skill and
   phase-gate reference, family architecture, Analyst UX/design system,
   certification/demo guidance and machine-readable family dashboard now carry
   the breadth-first, low-training, neutral-brand, no-fake-controls rules.
3. **Runtime reverified:** LocalAI/UI `c4495d82...` image `sha256:f255f188...`,
   forensic API `4aad3664...` image `sha256:c0f07382...`, worker
   `1846f558...`, NATS `5c49e70d...`, and PostgreSQL `f66e05a3...` are running;
   health-capable services are healthy. Retained counts remain exactly
   `51|51|64|0|22207|441|47`. Local version remains snapshot `40717b8` and
   `/v1/models/capabilities` remains 404.
4. **P1-3 source correction:** `extractForensicTarget` now removes complete UUID
   spans before analytical target scanning. Centralized hybrid-query argument
   construction preserves the full evidence UUID, selects
   `video_anpr_grouped_timeline`, leaves `plate` empty when no plate was supplied,
   and preserves explicit plate `MN1367` separately from the evidence UUID.
5. **Source verification PASS:** focused UUID/explicit-plate parameter tests,
   broader deterministic route/template tests, all MMV2 API state tests, the
   agent presentation contract, and analyst media tests (10/10) pass. Focused
   `go vet` passes. The initial sandbox-cache test attempt was an environment
   denial only; the approved identical test completed successfully.
6. **Current severity:** P0=0; source-open foundation P1=0; live-open foundation
   P1=3 until activation. P1-1 row/count, P1-2 complete-zero Activity, and P1-3
   UUID/plate are all source-resolved/live-open. NX-1 is not live-complete.
7. **Activation preflight:** free physical RAM is 5,674,300 KiB, 617,156 KiB
   below the mandatory 6 GiB gate. No tag, build, restart or recreation ran.
   Before build, create fresh rollback tags for the current `f255f188...` and
   `c0f07382...` images. Rebuild only forensic API and LocalAI/UI; protect
   worker/NATS/PostgreSQL, models, profiles, volumes and retained data.
8. **Stop boundary:** deployment manifest is
   `reports/nexusai-nx1-activation-manifest-20260825.json`. No retained mutation,
   model change, database migration, deployment, staging, commit, push or NX-UX1
   implementation occurred. Resume only with explicit NX-1 activation approval
   and a passing RAM/rollback preflight.

### 2026-08-25 - NX-1 operator activation package prepared

1. **Single operator script:** prepared
   `scripts/activate_nexusai_nx1_foundation_truth.ps1` for Windows PowerShell
   5.1. Its SHA-256 is
   `58b63235ac07b8758ec3015de56c182ddad88a4aa9094927288bb269bfee3699`.
   No second activation helper or deployment wrapper was added.
2. **Bounded behavior:** `-PreflightOnly` is read-only. Normal execution first
   repeats all preflight gates, creates two rollback tags, runs focused tests,
   builds only `forensic-records-api` and LocalAI/UI `api`, verifies the built
   artifacts, and recreates only those services with `--no-deps`. Worker, NATS,
   PostgreSQL, models, profiles, named volumes, and retained state are protected.
3. **Automatic rollback:** a substantive post-recreation failure restores only
   the two changed services from the newly verified rollback tags and then
   rechecks health, protected identities, retained counts, models, profiles,
   and volumes. The script never stops processes to manufacture free RAM.
4. **PowerShell validation:** Windows PowerShell
   `5.1.22621.6133` parsed the final script with zero parser errors.
5. **Safe preflight evidence:** the final zero-mutation preflight validated the
   checkpoint/source hashes, exact five runtime images, compose configs, health,
   retained counts `51|51|64|0|22207|441|47`, zero active jobs, five-model
   inventory, profiles, mounts, volumes, and disk policy. It then stopped at
   4,420,788 KiB (4.216 GiB) free RAM, below 6,291,456 KiB, with
   `NX1Activation=NOT_STARTED_RAM_GATE` and `RuntimeServiceMutation=false`.
6. **Runtime unchanged:** no rollback tag, image build, restart, recreation,
   migration, retained-data change, model change, profile change, volume change,
   cleanup, commit, push, or NX-UX1 work occurred. Evidence is
   `reports/nx1-foundation-truth-20260825/nx1-local-preflight-output.txt`.
7. **Resume boundary:** the operator may run the documented preflight and then
   normal activation only after the RAM gate passes. After
   `NX1Activation=PASS`, reopen Codex and read
   `reports/nx1-foundation-truth-20260825/nx1-local-activation-output.txt`
   before performing the full three-P1 live certification. Do not rebuild a
   successful activation.

### 2026-08-25 - NX-1 activation passed; live certification remains partial

1. **Activation PASS:** operator output
   `reports/nx1-foundation-truth-20260825/nx1-local-activation-output.txt`
   has SHA-256
   `8246e43607e32d8439688b768f505419d6054f04727dc41f3e750b7b4fc8b694`.
   LocalAI/UI now runs image `sha256:11549bad5bf...` in container
   `77d0f172f0ca...`; forensic API runs `sha256:4eb2db0af60d...` in
   `1bdf4f77087e...`. Rollback tags preserve the preceding exact images.
2. **Protection PASS:** worker `1846f558...`, NATS `5c49e70d...`, and
   PostgreSQL `f66e05a3...` retained their exact IDs. All health surfaces pass;
   the five-model contract hash remains `9c7f93d9...`; retained truth remains
   `51|51|64|0|22207|441|47`. Certification performed no rebuild, restart,
   upload, reprocess, model/profile/volume change, migration, or cleanup.
3. **Positive truth PASS:** English, natural English, Roman Urdu and Urdu all
   return `anpr.sightings`, `results_present`, one exact row and one exact
   citation for `MN1367`. An independent read-only PostgreSQL oracle confirms
   the same evidence/version/source row and hash. Ask and Activity reopen pass.
4. **P1-2 live-closed:** canonical grouped-video Ask preserves
   `complete_zero_results`, processing `completed`, operation
   `video.anpr_grouped_timeline`, zero rows, and the exact completed-zero answer.
   Activity list/reopen and 390/820/1024/1440 presentation pass without
   horizontal overflow or console errors.
5. **P1-1 remains live-open:** the raw negative and complete-zero operations
   both have authoritative `row_count: 0`, but the public enterprise adapter
   reports `row_count: 1` with zero table rows. The live response shape exposes
   a gap not covered by the source fixture: top-level metadata arrays are
   flattened as a result row before `enterpriseAuthoritativeRowCount` runs.
6. **P1-3 remains live-open:** canonical `show plates in this video evidence
   UUID` preserves the full UUID with no inferred plate, and the canonical
   explicit-plate form stays on the grouped-video operation. However ordinary
   `Show the grouped ANPR timeline for retained video evidence UUID` still
   routes to `anpr.timeline` with target `50057921`. Representative natural
   wording therefore does not satisfy the typed UUID contract.
7. **Current gate:** `NX1Activation=PASS`, `NX1LiveCertification=PARTIAL`,
   `OpenP0=0`, `OpenFoundationalP1=2`. Evidence is
   `reports/nx1-foundation-truth-20260825/nx1-live-certification-20260825.json`.
   NX-1 is not complete and NX-UX1 remains blocked. Any source correction and
   subsequent API + LocalAI/UI activation require a separate explicit approval;
   do not rebuild the successful activation in the meantime.

### 2026-08-25 - NX-1 final two-P1 source closure passed; activation not approved

1. **Verified start:** the preceding activation remains healthy and immutable:
   LocalAI/UI `77d0f172...` / `sha256:11549bad...`, forensic API
   `1bdf4f77...` / `sha256:4eb2db0...`, protected worker `1846f558...`, NATS
   `5c49e70d...`, PostgreSQL `f66e05a3...`, retained counts
   `51|51|64|0|22207|441|47`, and five-model hash `9c7f93d9...`.
2. **P1-1 final source PASS:** an explicit deterministic result count now has
   authority over flattened display metadata, including explicit zero. The
   public invariant forces complete-zero, no-match, not-processed, processing,
   failed, unavailable, unauthorized, and invalid-request states to zero.
   Positive one-row and three-row counts remain exact. A retained History
   persistence/reopen test preserves `row_count: 0` with
   `complete_zero_results`.
3. **P1-3 final source PASS:** grouped-video intent now precedes generic ANPR
   timeline intent. Complete UUID spans and typed source-second ranges are
   removed before analytical target extraction. Natural English, Roman Urdu,
   and Urdu routes preserve the grouped-video operation, full evidence UUID,
   optional explicit plate, `start_seconds`/`end_seconds`, tenant, user, and
   collection scope as separate parameters.
4. **Validation PASS:** focused public-adapter tests, full
   `go test ./api/forensic_records`, focused agent routing tests, focused agent
   Ginkgo History/presentation contracts, and
   `go vet ./api/forensic_records ./core/services/agents` pass. Production
   anti-hardcode search and `git diff --check` pass. No UI source changed, so
   no UI test was required for this correction.
5. **Living evidence:** source/operation/query matrix is
   `reports/nx1-foundation-truth-20260825/nx1-final-source-closure-matrix.json`.
   The current NX-1 live certification report contains a source-only addendum;
   its historical live observations remain unchanged. Roadmap, phase ledger,
   STIM maturity matrix, family matrix, operation matrix, and activation
   manifest are reconciled.
6. **Exact deployment scope:** source ownership requires rebuilding only
   `forensic-records-api` for P1-1 and LocalAI/UI Compose service `api` for
   P1-3. Worker, NATS, PostgreSQL, models, profiles, volumes, retained evidence,
   and database schema are protected and must not change.
7. **Prepared activation path:** updated the single Windows PowerShell 5.1
   script `scripts/activate_nexusai_nx1_foundation_truth.ps1`, SHA-256
   `b40f530902fda910cc30f76d461186c9e64fb22e2fdbbad51e7de0897e820a88`.
   Windows PowerShell `5.1.22621.6133` parses it with zero errors. It binds the
   exact current containers/images and source hashes, uses fresh rollback tags,
   requires at least 6 GiB free physical RAM, and writes new final-closure
   outputs without overwriting the successful prior activation evidence.
8. **Stop boundary:** no preflight, rollback tag, build, restart, recreation,
   upload, reprocess, model download/change, retained mutation, migration,
   cleanup, NX-UX1 work, commit, or push occurred. `OpenP0Source=0`,
   `OpenFoundationP1Source=0`, `OpenFoundationP1Live=2`. NX-1 remains live
   partial and NX-UX1 remains blocked. Resume only after exact approval to run
   the prepared two-service activation path and only if its fresh 6 GiB RAM and
   rollback preflight passes.

### 2026-08-26 - NX-1 final typed-identifier P1 source closure passed; stop before deployment

1. **Controlling live truth:** the latest bounded activation is healthy. P1-1
   authoritative public row count and P1-2 complete-zero Ask/Activity are live
   closed. P1-3 remains the sole live foundation P1 because deep inspection of
   grouped-video query understanding exposed UUID-internal phone/entity matches
   and an unbound independently written plate.
2. **Exact root cause:** `extractTargets` applied phone and plate-shaped regular
   expressions independently across the raw question. A complete UUID was not
   a first-class protected span, and `normalizeExtractedTarget` could compact a
   digit-heavy UUID into a phone. `adaptLegacyRuntimePlan` then flattened every
   surviving value into a generic `target`. Although
   `videoANPRGroupedTimeline` already supports `evidence_id`, optional `plate`,
   `start_seconds`, and `end_seconds`, the governed execution parameter model
   exposed none of those dedicated fields.
3. **Typed-span rule:** recognize every complete valid forensic UUID first,
   preserve it canonically as one `evidence_id`, and exclude every overlapping
   phone, plate, or generic-entity match. Protect an explicit source-second
   range only when the matched span names seconds, preserving ordinary
   hyphenated phones. Independently written plate text outside the UUID remains
   a `plate` entity and binds the existing grouped-video `plate` constraint.
4. **Source result:** grouped-video query understanding now contains exactly
   one evidence target plus an optional independent plate target. Execution
   keeps `target`/`targets` restricted to the authoritative evidence UUID and
   carries dedicated `evidence_id`, `plate`, `start_seconds`, and `end_seconds`
   parameters. The operation/template remains
   `video.anpr_grouped_timeline` / `video_anpr_grouped_timeline` across English,
   Roman Urdu, Urdu, UUID-only, UUID+plate, UUID+time, and UUID+plate+time cases.
5. **Validation PASS:** 12 deterministically randomized valid UUIDs plus
   adversarial digit-heavy and plate-looking UUID groups pass exact entity,
   target, and execution-plan assertions. Standalone phone and plate controls
   pass. Focused tests pass; full `go test ./api/forensic_records -count=1`
   passes in 68.010 s with all 285 Ginkgo specs; the derived query-variant
   ledger reconciles; `go vet ./api/forensic_records`, production anti-hardcode
   scan, and `git diff --check` pass.
6. **Mutation boundary:** no build, image tag, restart, recreation, upload,
   reprocess, retained mutation, model/profile/volume change, migration,
   cleanup, NX-UX1 work, commit, or push occurred. The running forensic API is
   container `df9d069ad51f...`, image `sha256:74c07cce1225...`, status running.
7. **Next deployment gate:** P1-3 source verdict is PASS, but live closure still
   requires rebuilding/recreating only Compose service
   `forensic-records-api`. LocalAI/UI, worker, NATS, PostgreSQL, models,
   profiles, volumes, retained evidence, and schema are protected. Fresh free
   physical RAM is 4.555 GiB, below the mandatory 6 GiB gate. Before any future
   recreation, preserve an exact rollback tag for image
   `sha256:74c07cce1225d0694ca8342419af9f1994801aeacdb1c048a54ef9af854ed8a7`.
   Current markers: `OpenP0=0`, `OpenFoundationP1Source=0`,
   `OpenFoundationP1Live=1`, `P1-3Source=PASS`, `P1-3Live=OPEN`,
   `NXUX1Started=false`.

### 2026-08-26 - NX-1 source-time and Urdu routing closure passed; stop before API-only activation

1. **Controlling live replay:** the typed-identifier activation succeeded and
   remains healthy in forensic API container `8879e5d6da3e...`, image
   `sha256:0ef44346e744...`. P1-1 and P1-2 remain live closed. P1-3 now passes
   live for protected UUID parsing, explicit plate binding, English and Roman
   Urdu grouped-video routing, and positive/negative ANPR result counts.
2. **Remaining live gaps isolated:** explicit video source-time wording such as
   `from N seconds to M seconds` selected the right operation but lost both
   bounds because the range recognizer required the seconds unit only after the
   final value. Native Urdu failed before routing because evidence, ANPR,
   grouped, timeline/observations, and find/show variants were absent from the
   deterministic semantic normalization vocabulary.
3. **Source-time rule:** recognize only explicit source-time phrases naming
   seconds, carry non-negative bounds through query-understanding filters and
   dedicated execution parameters, and leave `date_from`/`date_to` untouched.
   Reversed bounds reach the existing executor validation and are rejected;
   explicit bounded zero results are reported as `no_match_for_filter`.
   Hyphenated phone numbers without a seconds unit are not ranges.
4. **Urdu rule:** normalize a bounded semantic vocabulary for video evidence,
   ANPR, plate, grouped observations/timeline, and show/find while preserving
   the exact LTR UUID. This is phrase-family normalization, not a literal-query
   exception and not an LLM planner.
5. **Validation PASS:** randomized source-time, UUID, plate, phone, English,
   Roman Urdu, and four native Urdu variants pass query-understanding,
   execution-plan, and final-record assertions. Focused tests pass in 15.285 s;
   full `go test ./api/forensic_records -count=1` passes in 68.503 s; focused
   `go vet`, production anti-hardcode scan, JSON validation, and
   `git diff --check` pass.
6. **Mutation boundary:** no build, tag, restart, recreation, deployment,
   upload, reprocess, retained mutation, model/profile/volume change, database
   migration, NX-A1, NX-UX1, commit, or push occurred in this source turn.
7. **Next gate:** source-open foundation P1 is zero; live foundation P1 remains
   one until a separately approved rebuild/recreation of only
   `forensic-records-api` and exact live replay. Fresh free RAM is 5.13 GiB,
   below the mandatory 6 GiB gate. The next rollback baseline is the current
   image `sha256:0ef44346e74478a8d0775db19350f674a4f7089fefe4f6fd7374cf93d4b3a454`;
   the preceding rollback tag
   `nexusai/forensic-records-api:rollback-before-nx1-p13-final-20260826`
   continues to preserve image `sha256:74c07cce1225...`.

### 2026-08-26 - NX-1 closed live; NX-UX1A standalone workspace source slice passed

1. **NX-1 final live reconciliation:** the separately executed API-only
   activation passed. Forensic API container
   `d8ce6868f51d990ae51f95d48637c5b771411a917076262f58a47263a30096af`
   is healthy on image
   `sha256:d2992062b8f55334d602530eb96cec8289dd714f0ae9523d3b27edebb724b5c5`.
   Source-time bounds, English, Roman Urdu, native Urdu, protected UUID,
   independent plate and positive/negative grouped-ANPR cases pass live.
   Markers: `NX1TimeUrduActivation=PASS`,
   `NX1FinalLiveCertification=PASS`, `OpenP0=0`, foundation `P1=0`.
2. **Protected runtime:** LocalAI/UI remains container
   `ddfa8e0d4d2c0dc1b14ef112c91ac4bfeb6bb3fb4800ff722aa022ba5c7c0f23`,
   image `sha256:71b63e0ef454...`; worker, NATS and PostgreSQL remain healthy
   and unchanged. Retained accounting is unchanged at
   `51|51|64|0|22207|441|47`. The final NX-1 rollback tag is
   `nexusai/forensic-records-api:rollback-before-nx1-time-urdu-final-20260826`
   pointing to `sha256:0ef44346e744...`.
3. **NX-UX1A source implementation:** `/analyst/*` now presents the neutral
   **Investigation Workspace** identity and the canonical Home, Data, Ask and
   Activity information architecture. `/analyst/history` preserves old deep
   links through a query-preserving redirect. The boot state, application title
   and shell are neutral only on analyst routes; the advanced `/app` identity
   remains unchanged.
4. **Reuse boundary:** authentication, active workspace/case state, forensic
   APIs, agent history, governed result presentation, result-state semantics,
   citations, media presentation and unified source detail are reused. No API,
   ingestion, operation, media-processing, model, retained-state or database
   contract changed. Ask remains automatic and hides implementation selectors.
5. **Language and responsive contract:** evidence/result values retain bidi
   isolation and the Ask composer now uses `dir="auto"`, with exact mixed Urdu
   plus LTR UUID/plate preservation asserted. Controlled coverage includes
   390, 820, 1024 and 1440 px, legacy route compatibility, keyboard controls,
   zero/no-match truth states, and zero horizontal overflow.
6. **Validation:** focused Node tests pass 11/11; focused ESLint passes; the
   production UI build passes with 679 modules; the analyst Playwright suite
   passes 16/16. In-app source-preview inspection of the live read-only
   `default / nexusai-multimodal-product-acceptance` workspace confirms the
   neutral shell and settled desktop/mobile layouts. The upstream capability
   matrix is
   `reports/localai-upstream-capability-reconciliation-20260826.md`.
7. **Mutation and next gate:** no container build, image tag, restart,
   recreation, deployment, upload, reprocess, retained mutation, model/profile/
   volume change, database migration, commit or push occurred. Eventual source
   activation scope is LocalAI/UI Compose service `api` only, not approved.
   Preserve the current LocalAI image as rollback baseline and require a fresh
   6 GiB free-physical-RAM pass; the last read-only observation was 5.08 GiB.
   Exact next subphase is NX-UX1B Data and unified source-detail refinement.

### 2026-08-27 - NX-UX1B/C Data, unified detail and media source/browser pass

1. **Source verdict:** NX-UX1B and the bounded NX-UX1C media refinement pass
   source and read-only browser validation. They are not yet represented as a
   new deployed-runtime activation. NX-A1 remains unstarted.
2. **Data hierarchy:** the analyst Data surface is now a quiet evidence library
   with type-specific icons, human filenames, meaningful finding/readiness
   previews, date/size context and relevant-only status filters. Media previews
   no longer expose internal artifact counts as user-facing transcript/timeline
   counts. Dense backend-shaped columns, permanent zero-count filters and a
   large mobile filter wall were removed.
3. **Unified detail:** desktop inspection is a wide evidence surface and mobile
   inspection is full-width. Summary, real source preview, findings and actions
   precede optional Details and Technical details. Empty field, quality,
   capability and derived-output sections disappear; reported zero accounting
   remains visible and distinct from unavailable values.
4. **Media refinement:** images retain aspect ratio and have working zoom,
   fit, reset and plate/face/OCR overlay controls. ANPR uses real plate text,
   confidence, source crop and review state. OCR is bounded to six initial
   observations without deleting genuine low-confidence output. Face and image
   similarity remain candidate-only. Audio puts the player and machine-review
   state first, pairs Urdu source text with Roman Urdu as a secondary aid, and
   hides format/processor data. Video preserves retained source playback,
   source-time seeking, grouped-detection/not-tracking language and completed-
   zero plate truth. PDF/TXT/DOCX share a truthful preview/extraction pattern.
   Raw governed references remain accessible but collapsed by default.
5. **Contract boundary:** existing case/evidence/content, derived-artifact,
   similarity/comparison, Ask, citation, history and result-state contracts are
   unchanged. No API, Go source, ingestion, model, profile, processor, database
   or retained-state contract changed; no evidence was uploaded, reprocessed,
   deleted or otherwise mutated.
6. **Validation:** presentation/media/workspace Node tests pass 15/15; focused
   Data/media Playwright paths and the final full analyst portal suite pass
   16/16. Production Vite build passes with 681 modules.
   Changed-file ESLint has zero errors (the current JSX-use configuration emits
   nine `no-unused-vars` warnings for JSX-used imports/components). Read-only
   in-app validation against
   `default / nexusai-multimodal-product-acceptance` confirms structured,
   document, image, retained ANPR, audio and video presentation, zero visible
   console errors, and no horizontal overflow at 390, 820, 1024 or 1440 px.
   Visual evidence is retained under `reports/nx-ux1b-visual-evidence/`.
7. **Changed production/test source:**
   `core/http/react-ui/src/analyst/AnalystData.jsx`,
   `AnalystEvidenceRow.jsx`, `AnalystPortal.css`,
   `analystDataPresentation.js`, `analystDataPresentation.test.js`, and
   `core/http/react-ui/e2e/analyst-portal.spec.js`. Living roadmap, ledger,
   product architecture, maturity matrix and team-lead demo artifacts were
   reconciled in place; no new status-report document was created.
8. **Protected runtime/RAM:** no build, image tag, restart, recreation or
   deployment occurred. Read-only rediscovery found LocalAI/UI container
   `8fd05d6827aae3923eb998a2f7f27888434b9f5358f171c40120b6797a584d83`
   on image
   `sha256:5d6cf265ae6f0209201339516eb589cc9b38da8490fbb8a40a2343c0d529d4d3`
   healthy; forensic API remains
   `d8ce6868f51d990ae51f95d48637c5b771411a917076262f58a47263a30096af`
   on `sha256:d2992062b8f5...`. Worker, NATS and PostgreSQL remain healthy.
   Fresh free physical RAM was 3.008 GiB, below the mandatory 6 GiB gate.
9. **Activation/rollback/next:** eventual deployment scope is LocalAI/UI Compose
   service `api` only. Preserve current LocalAI image
   `sha256:5d6cf265ae6f...` as rollback baseline. The exact next source subphase is
   NX-UX1D Ask result presentation; do not start NX-A1.

### 2026-08-27 - NX-UX1D Ask and answer workspace source/browser pass

1. **Source verdict:** NX-UX1D passes its bounded source gate. `/analyst/ask`
   is now an investigation question/answer workspace rather than a generic
   agent-chat skin. NX-UX1E is next; NX-A1 remains unstarted.
2. **Scope and composer:** the Ask surface always shows a human workspace scope
   and shows a human filename when entered from Data. Data hand-offs now retain
   the exact evidence ID in the route alongside the capability-derived prompt.
   The multiline composer remains keyboard accessible and `dir="auto"`; no
   model, agent, family, operation, backend or processor selector is exposed.
3. **Answer contract:** the primary order is answer, deterministic findings,
   typed table/timeline output, human citations and contextual next questions.
   Optional interpretation follows verified facts. Limitations and technical
   execution are collapsed. Advanced `/app` presentation and observability are
   preserved.
4. **Truth states:** `results_present`, `complete_zero_results`,
   `no_match_for_filter`, `processing`, `not_processed`, `failed`,
   `unavailable`, `unauthorized` and `invalid_request` remain distinct.
   Complete-zero says processing completed with no detections; filter no-match
   says no results matched the filter. Source-time parameters render as seconds,
   never calendar dates.
5. **Citations and multilingual:** citations prefer filename plus page, row or
   source-time and preserve existing evidence navigation. Urdu, Roman Urdu,
   English and mixed Urdu/LTR identifiers remain direction-aware without a
   global RTL flip.
6. **Validation:** 18/18 focused analyst presentation/data/media tests pass;
   changed-file ESLint has zero errors (29 existing JSX/hook warnings); Vite
   production build passes with 682 modules; the full analyst portal suite is
   19/19 and advanced Agent Chat compatibility is 20/20. Accepted widths
   390/820/1024/1440 have zero horizontal overflow and browser console errors
   are zero. Production anti-hardcode scan, trailing-whitespace scan and
   `git diff --check` pass. Seven screenshots, including the read-only retained
   workspace initial Ask view, are in
   `core/http/react-ui/reports/nx-ux1d-visual-evidence/`.
7. **Runtime boundary:** no Docker image, container, service, API, worker, model,
   profile, volume, database or retained evidence was changed. The protected
   LocalAI/UI image remains `sha256:5d6cf265ae6f...`; forensic API remains
   `sha256:d2992062b8f5...`. Fresh free RAM was 3.800 GiB, below the 6 GiB gate.
   Eventual deployment scope remains LocalAI/UI Compose service `api` only and
   requires separate approval plus the preserved LocalAI rollback image.
8. **Boundaries and next:** `HomeDeepRedesign=NX-UX1F` and
   `AnalystDesignSystemConsolidation=NX-UX1F`. Exact next subphase and action:
   begin NX-UX1E Activity source design; do not deploy and do not start NX-A1.

### 2026-08-27 - NX-UX1E Activity and citation navigation source/browser pass

1. **Source verdict:** NX-UX1E passes its bounded source gate. Activity is now a
   chronological investigation journal rather than a flat technical history
   log. NX-UX1F is next; NX-A1 remains unstarted.
2. **Journal contract:** entries group by human date, lead with the analyst
   question and exact governed outcome, show useful human source context and
   preserve `Load more activity`. Search clearly covers only loaded items and
   filters are shown only when their state group has members.
3. **Truth states:** `results_present`, `complete_zero_results`,
   `no_match_for_filter`, `processing`, `not_processed`, `failed`,
   `unavailable`, `unauthorized` and `invalid_request` remain distinct.
   Complete-zero says processing completed with no detections; filter no-match
   says no results matched the filter.
4. **Review and continuation:** reopening an entry renders the stored governed
   answer, metrics, findings, table/timeline, citations and limitations without
   rerunning the question. Continue in Ask preserves source scope when present.
   Long legacy Markdown is summarized in the journal but remains intact in the
   reopened stored result.
5. **Source navigation:** Activity and Ask citations prefer catalog filename
   and type, and carry only supported source-time/page/row/finding locators into
   Data. Data identifies the citation context without pretending to seek media,
   move a document page or select a row automatically. Structured, document,
   image/ANPR, audio and video matrices pass.
6. **Validation:** 25/25 focused analyst presentation/data/media tests pass;
   changed-file ESLint has zero errors; Vite production source build passes with
   683 modules; the analyst portal suite is 21/21 and advanced Agent Chat is
   20/20 (41/41 combined). Production anti-hardcode scan, trailing-whitespace
   scan and `git diff --check` pass. Read-only retained-workspace and mocked
   screenshots are in
   `core/http/react-ui/reports/nx-ux1e-visual-evidence/`.
7. **Browser evidence:** read-only validation used
   `nexusai-multimodal-product-acceptance`, reviewed an existing stored ANPR
   result and opened its retained source without submitting Ask or changing
   Activity. A desktop long-identifier layout defect found during that pass was
   corrected and rechecked; 390 px retained review has no horizontal overflow.
8. **Runtime boundary and next:** no image, container, service, API, worker,
   model, profile, volume, database or retained evidence changed. The protected
   LocalAI/UI image remains `sha256:5d6cf265ae6f...`; forensic API remains
   `sha256:d2992062b8f5...`. Fresh free RAM was 3.344 GiB, below the 6 GiB gate.
   Eventual scope remains LocalAI/UI Compose service `api` only with separate
   approval and the current LocalAI image preserved for rollback.
   `HomeDeepRedesign=NX-UX1F` and
   `AnalystDesignSystemConsolidation=NX-UX1F`; exact next action is NX-UX1F.

### 2026-08-27 - NX-UX1F Home and analyst design-system source/browser pass

1. **Source verdict:** NX-UX1F passes its bounded source and read-only browser
   gate. Home is now a workspace-orientation surface and the four analyst
   routes use one route-scoped presentation system. NX-UX1G is next; NX-A1 has
   not started.
2. **Home truth and hierarchy:** the primary readiness statement is derived
   from evidence-source state, not `accepted_rows`; the retained multimodal
   workspace therefore reports 24 ready sources truthfully. One Ask entry,
   conditional attention guidance, a compact evidence summary with modality-
   aware recent evidence and exact recent Activity replace the suggestion,
   readiness, capability and coverage card wall. Empty and processing-only
   workspaces have separate useful states. A final reconciliation also routes
   attention-only or processing-only workspaces to Data review rather than
   presenting Ask before any evidence source is ready.
3. **Cross-page system:** analyst-scoped content widths, title scale, spacing,
   surfaces, lines, radii and 42 px controls reconcile Home, Data, Ask and
   Activity. Decorative Home gradients, shell/composer glass, excessive
   elevation and the duplicated legacy Ask block were removed. Ask begins at
   the top of its reading column instead of vertically centering a large empty
   state. Advanced `/app` styles remain isolated.
4. **Disclosure and language:** recent Home Activity simplifies UUID-shaped
   identifiers without changing the stored question. Legacy template, route
   and execution findings remain retained but render only inside Technical
   details. Urdu and mixed-language content use automatic direction; exact LTR
   values remain isolated.
5. **Validation:** 32/32 focused analyst presentation/workspace tests pass. Production
   Vite build passes with 684 modules. The analyst portal suite passes 21/21
   and advanced Agent Chat compatibility passes 20/20. The actual retained
   workspace passes Home/Data/Ask/Activity at 390/820/1024/1440 (16/16) with
   zero horizontal overflow; the automated matrix reports zero console/page
   errors. Before/after and retained detail review evidence is under
   `core/http/react-ui/reports/nx-ux1f-visual-evidence/`.
6. **Runtime boundary:** no image, container, service, API, worker, model,
   profile, backend, volume, database or retained evidence changed. The
   protected LocalAI/UI rollback image remains `sha256:5d6cf265ae6f...` and
   forensic API remains `sha256:d2992062b8f5...`. Eventual deployment scope is
   LocalAI/UI Compose service `api` only, behind separate approval and a fresh
   minimum 6 GiB free-physical-RAM gate. Final read-only free RAM was 3.195
   GiB, so the gate is closed.
7. **Next boundary:** source-open P0=0 and NX-UX1F P1=0. Do not deploy this
   checkpoint. Exact next subphase and action: NX-UX1G; do not start NX-A1.

### 2026-08-27 - NX-UX1G final product acceptance and NX-UX1 closure

1. **Closure verdict:** NX-UX1G passes final source, test, build, retained-
   workspace and read-only browser acceptance. `OpenP0=0`,
   `OpenNXUX1GP1=0`, `NXUX1GAcceptance=PASS` and `NXUX1=CLOSED`. The exact
   next phase is NX-A1, which has not started.
2. **Final P1 reconciliation:** Ask now has one stable semantic level-one
   heading. Stored planner confidence, display-row counts and other execution
   telemetry no longer appear as analyst findings; they remain available under
   Technical details and advanced `/app` remains unchanged. Home previews mask
   both complete UUIDs and UUID-derived fragments without masking independent
   plates or other identifiers in the retained question.
3. **Product acceptance:** Home, Data, Ask and Activity pass one canonical
   evidence -> question -> governed result -> citation -> source -> stored-
   result-review journey. Structured, document, image, ANPR, face-candidate,
   audio, video and KB/retrieval evidence were inspected against the retained
   `nexusai-multimodal-product-acceptance` workspace without submitting Ask or
   changing evidence, Activity or processing state.
4. **Truth and navigation:** complete processing with zero detections remains
   distinct from no match for an applied filter. Citations use only supported
   page, row, source-time or finding locators; opaque legacy locators do not
   cause invented seeking. Data -> Ask scope, Ask/Activity -> Data source
   opening, Activity stored-result reopen, Urdu/Roman-Urdu/mixed-LTR copy,
   dark/light theme, keyboard focus and advanced `/app` compatibility pass.
5. **Validation:** 32/32 focused analyst tests, 21/21 analyst Playwright tests,
   20/20 advanced Agent Chat tests and the 684-module production Vite build
   pass. The 16-view Home/Data/Ask/Activity matrix at 390/820/1024/1440 has
   zero horizontal overflow, one H1 and no unnamed buttons per route. A fresh
   browser pass has zero console errors or warnings. ESLint has zero errors;
   production hardcode, whitespace and diff checks pass. Final screenshots are
   in `core/http/react-ui/reports/nx-ux1g-final-acceptance/`.
6. **Runtime boundary:** no build, deployment, restart, service recreation,
   model/profile/backend/database/volume change, evidence mutation or Ask
   submission occurred. Future activation scope is Compose service `api` only,
   under separate approval, with a fresh minimum 6 GiB free-RAM gate and the
   running LocalAI/UI image preserved as the rollback baseline. Final free RAM
   was 3.437 GiB, so `DeploymentReady=false`.
7. **Handoff:** NX-A1 may begin only as a new bounded phase. It owns the shared
   typed execution foundation, not further NX-UX1 redesign and not deployment
   of this checkpoint.

### 2026-08-28 - NX-UX1 live activation integration recovery passed

1. **Incident:** an operator-run external UI activation script built the NX-UX1
   source successfully but recreated `api` from only `docker-compose.yaml`.
   The replacement therefore omitted `FORENSIC_RECORDS_API_URL`,
   `FORENSIC_RECORDS_API_KEY` and `LOCALAI_AGENT_POOL_DATABASE_URL`. The shell
   rendered, while evidence/status/capability/manifest requests returned
   502/503 and retained History was unavailable. Its protected-ID comparison
   also compared a full API ID with short IDs, incorrectly classified the
   intentionally replaced API as protected and stopped after recreation.
2. **Secondary startup failure:** Docker subsequently restarted the runtime.
   The preserved forensic API and worker started before PostgreSQL/NATS were
   ready, exited with code 1, and remained stopped because their restart policy
   is `no`. They were restarted in place after both dependencies were healthy;
   the exact protected container and image identities were preserved.
3. **Bounded recovery:** only `api` was recreated with the authoritative merged
   `docker-compose.yaml` plus `docker-compose.forensic-runtime.localai.yaml`
   configuration. The accepted secret env file hash remains
   `82bb4660ef9615f821f3ef3157652054d6b338fa5324cb894f53ea6e9e5de726`.
   The missing retained-history URL was reconstructed from the live PostgreSQL
   owner environment without printing or persisting a secret. No image build
   was repeated.
4. **Canonical runtime:** LocalAI/UI now runs container
   `321ba681774b371dfbbf8a85c34ee45596e62f659800e6d647a0f5b7974c4021`
   on canonical image
   `sha256:628f56af541ac739887ddfb6f08d661e1070c4d0a2d343fb902882ab0f5dba9a`.
   The pre-NX-UX1 rollback tag
   `nexusai-ui:rollback-before-nxux1-20260828-091424` still points to
   `sha256:5d6cf265ae6f0209201339516eb589cc9b38da8490fbb8a40a2343c0d529d4d3`.
5. **Protected runtime:** forensic API `d8ce6868f51d...` on
   `sha256:d2992062b8f5...`, worker `1846f558941f...` on
   `sha256:be13dbec4bd...`, NATS `5c49e70d132a...` and PostgreSQL
   `f66e05a3b179...` are running; all health checks pass.
6. **Live acceptance:** Home renders 24 ready evidence sources, Data renders the
   retained multimodal library, and Activity renders 15 retained entries.
   Evidence, manifest, forensic status, capabilities and scoped History return
   HTTP 200. Browser console errors/warnings are zero.
7. **Truth boundary:** retained accounting remains
   `51|51|64|0|22207|441|47`. No evidence, job, model, profile, backend,
   database schema or volume was changed. Final free RAM was 2.544 GiB; this
   recovery performed no build. Marker: `NXUX1RuntimeRecovery=PASS`.

### 2026-08-28 - Post-NX-UX1 forensic runtime hardening source pass

1. **Bounded verdict:** `ForensicRuntimeHardeningSource=PASS` and
   `OpenRuntimeHardeningP1=0`. This is a source/configuration correction only;
   it does not reopen NX-UX1 or start NX-A1.
2. **Exact root cause:** the forensic API and worker intentionally fail fast
   with logged errors when PostgreSQL or NATS is unavailable. Their resolved
   restart policy was `no`, so a dependency race during Docker Desktop/daemon
   restoration left them exited. Existing Compose `service_healthy`
   dependencies protect normal Compose orchestration but cannot restart an
   already failed container.
3. **Source correction:** only `forensic-records-api` and
   `forensic-records-worker` declare `restart: unless-stopped`. This provides
   observable engine-managed retry/backoff for transient startup exits and
   daemon restart recovery while preserving intentional manual-stop semantics.
   No infinite application retry loop was added.
4. **Readiness truth:** PostgreSQL retains its `pg_isready` healthcheck and NATS
   retains its monitoring `/healthz` healthcheck. Both API and worker retain
   `service_healthy` dependencies on PostgreSQL/NATS and the completed spool
   initializer. The API's external `/healthz` is reachable only after its
   fail-fast dependency initialization; the worker process exits if startup
   validation fails, allowing the restart policy to recover it. For a temporary
   post-start outage, existing clients reconnect/reacquire connections, API
   outbox failures are logged and deferred, and worker claim failures are
   logged and NAKed so JetStream retains them for retry.
5. **Resolved topology:** project `nexusai`, the same five forensic services,
   default network, current runtime images, and named volumes
   `forensic_postgres_data`, `forensic_nats_data`, and `forensic_spool` are
   unchanged. The read-only verifier
   `scripts/verify_forensic_runtime_restart_hardening.ps1` passes without
   printing resolved environment values or secrets.
6. **Mutation boundary:** `RetainedStateMutated=false`, `ModelsChanged=false`,
   `DatabaseMigration=false`, `VolumesChanged=false`,
   `DeploymentPerformed=false`, and `SecretLeakage=0`. No build, restart,
   service recreation, reprocessing, migration or volume operation occurred.
7. **Activation boundary:** live containers retain their existing runtime
   metadata until separate approval. Future activation must preflight healthy
   PostgreSQL/NATS and available RAM, then force-recreate only
   `forensic-records-api` and `forensic-records-worker` with the accepted three
   forensic Compose overlays and existing images. Rollback is the inverse
   two-line policy change followed, only if approved, by the same bounded
   two-service recreation. `OpenP0=0`; exact next phase is NX-A1, not started.

### 2026-08-28 — NX-A1 shared execution foundation source progress

1. **Inventory gate complete:** the required reconciliation found that APF-3
   already owns typed query understanding, a governed single/composed plan,
   operation/capability projection, deterministic/retrieval/hybrid execution,
   Fact Packets, enterprise response, citations, Agent transport and scoped
   Activity. NX-A1 therefore extends these authorities and creates no parallel
   router, catalogue, Fact Packet, response, history store or UI engine.
2. **Shared contracts:** source now defines bounded Investigation Context,
   evidence/time/location scopes, readiness states/snapshot, capability
   requirements, plan-validation result, Tool Invocation/Tool Result,
   Observation Packet, Clarification Request, authority classes, execution
   statuses, and the existing public result states. Existing plan/step/
   operation/budget/entity/citation/answer types are reused through aliases.
3. **Execution integration:** after the accepted direct or two/three-step
   composition plan is built, one shared validator rechecks exact authenticated
   scope, query permission, capability readiness, dependencies/cycles, budget
   and the existing governed implementation-key allowlist before execution.
4. **Truth boundary:** deterministic facts and derivations are separate from
   model observations, semantic retrieval, candidate correlations and LLM
   interpretation. Observation Packets require same-scope citations and reject
   deterministic-fact authority. Composition can publish only cited candidate
   correlation observations.
5. **Safety:** the typed case-query adapter recursively rejects raw SQL,
   statements, commands, arbitrary executables/tool URLs and implementation-key
   overrides. Plans remain read-only and bounded to the existing APF-3 limits.
6. **Shared consumers:** a contract test proves a visual typed action and a
   natural-language Ask question resolve the same `cdr.frequent_contacts`
   operation. The same Tool Result/Answer Envelope feeds presentation and
   Activity; React contains no analytical calculation.
7. **Focused validation:** the clean pre-change API baseline passed in 71.860s;
   the focused Ginkgo `NX-A1` suite passes in 3.175s using the repository-local
   Go cache after the known shared Windows cache collision. Full regression,
   build/lint/diff/hardcode gates and the final closure report remain.
8. **Mutation boundary:** no runtime, container, service, retained evidence,
   Activity, model, profile, backend, database, volume, stage, commit, push or PR
   change. `RuntimeMutated=false`, `RetainedStateMutated=false`,
   `ModelsChanged=false`, `DatabaseMigration=false`.
9. **Phase transition:** NX-A1.8 certification is recorded in the closure entry
   below. Do not begin NX-B1 or perform deployment without a new explicit
   directive.

### 2026-08-28 — NX-A1 source closure

1. **Closure:** NX-A1.1 through NX-A1.8 are source-complete. The implementation
   extends APF-3 and does not introduce a parallel router, operation catalogue,
   plan hierarchy, Fact Packet, answer envelope, history store or UI engine.
2. **Regression evidence:** focused NX-A1 Ginkgo passes; the full forensic API
   suite passes in 74.212s; APF-3.1–3.7 passes in 56.073s; focused forensic
   agent routing/presentation and six standalone forensic agent tests pass; all
   32 analyst presentation tests pass; Go vet and the React production build
   pass. Global ESLint has six unrelated pre-existing hook errors in untouched
   files. Of 119 agent specs, 105 pass and 14 database-backed specs are blocked
   before assertions by unsupported Windows rootless-Docker testcontainers.
3. **Runtime reconciliation:** read-only inspection found LocalAI/UI image
   `sha256:628f56af541a...` healthy, forensic API image
   `sha256:d2992062b8f...` running, worker image `sha256:be13dbec4bd9...`
   healthy, and PostgreSQL/NATS healthy. API/worker retain
   `restart: unless-stopped`; no container or service was changed.
4. **Mutation boundary:** `RuntimeMutated=false`,
   `RetainedStateMutated=false`, `ModelsChanged=false`,
   `ProfilesChanged=false`, `BackendsChanged=false`,
   `DatabaseMigration=false`, and `DeploymentPerformed=false`.
5. **Open NX-A1 gates:** `OpenP0=0`; `OpenNXA1SourceP1=0`.
6. **Handoff:** `NXA1=COMPLETE`. Exact next phase is NX-B1, not started. Do not
   deploy or begin NX-B1 without a new explicit directive.

### 2026-08-29 — NX-MMR empirical source slice and human-oracle boundary

1. **Governance and supply chain:** the corrected workload-aware policy is
   recorded. The exact seven-file bundle totals 663,933,265 bytes and passes
   hash/size/container/license review. No public dataset, substitution or live
   model install occurred.
2. **Private truth packs:** the 108-image inventory and 22-image sealed holdout
   plus the 60.010-second video's 30 review frames, contact sheets and lossless
   review audio are prepared. ANPR and ASR inference is stopped until an
   independent human completes the image, interval and transcript templates.
3. **OCR evidence:** on the same eight synthetic samples, Paddle achieved 5/6
   English exact versus Tesseract 3/6. Both were 0/1 Urdu exact; Tesseract had
   lower single-sample Urdu CER/WER. English is fixture-only, Urdu limited and
   handwriting data-required.
4. **Retrieval evidence:** the current Qwen3 embedding baseline achieved
   Recall@1 0.8182, MRR 0.8667 and Recall@5 1.0 in 136.836 seconds, but 0/2
   no-answer abstention. The Qwen3 reranker achieved Recall@1 0.7273, MRR
   0.8636, Recall@3 1.0 and 2/2 abstention in 397.425 seconds after one bounded
   timeout retry. Both reached the 3 GiB container cap. Reranker admission is
   rejected; the current embedding baseline is retained with limitations.
5. **Auto-routing source:** `nexusai-evidence-route-plan/v1` now extends the
   existing classifier/queue/registration metadata in shadow-only mode. Magic
   and MIME lead, extension is only a hint, scope/security/readiness/resource
   states are explicit and missing models/workers cannot become zero findings.
   The 16-case sealed routing matrix passes 16/16; automatic execution was not
   activated.
6. **Certification truth:** no operation or visible suggestion was promoted.
   The ledger remains 62 registered, 12 source-validated, one fixture-certified,
   zero real-world-certified and four product-certified. No self-oracle or
   unrestricted SQL was introduced.
7. **Validation:** the full forensic API suite passes (322 specs, 320 passed,
   two skipped) in 46.934 seconds; route-focused tests and Go vet pass. Six
   NX-MMR Python tests pass. The agents suite remains at the known Windows
   baseline: 105 pass and 14 database-backed specs are environment-blocked by
   unsupported rootless-Windows testcontainers before assertions.
8. **Mutation boundary:** disposable internal-only benchmark containers and
   their temporary network were removed. `RuntimeMutated=false`,
   `LiveModelsChanged=false`, `RetainedStateMutated=false`,
   `ActivityMutated=false`, `DatabaseMigration=false`, `VolumesChanged=false`
   and `DeploymentPerformed=false`.
9. **Exact next action:** the user/reviewer completes and validates the private
   image/video/transcript ground truth. The already-approved non-retained
   ANPR/ASR scoring can then resume. Any live route/model/service activation and
   NX-B2.1 remain separate approval boundaries behind the unchanged 6 GiB
   build/deployment gate.

### 2026-08-29 — NX-MMR guided human-verification utility

1. **Workflow implemented:** a benchmark-only, loopback-bound browser helper
   now replaces manual editing of 108 image rows and 30 coarse video rows. It
   verifies the real private source hashes and writes only to the ignored
   NX-MMR private ground-truth directory.
2. **Bounded image workload:** exactly 10 deterministic metadata-diverse
   development images are reviewed and frozen before the existing 22-image
   sealed holdout opens. Every active sample requires an explicit independence
   declaration. A model-output-seen sample is recorded, excluded and replaced
   by a fixed hash-ranked sample; it is never silently claimed independent.
3. **Video and speech UX:** the original 60.010-second video supports native
   play/pause/seek and direct event/negative interval capture. Browser Web Audio
   uses only signal energy. The prepared audio decodes as digital silence, so
   the honest fallback is six approximately 10-second audit windows for human
   speech/no-speech confirmation; no ASR output or Roman-Urdu pseudo-oracle is
   exposed.
4. **Fail-closed validation:** Finish requires 32 complete independent image
   labels, frozen development labels, full-video attestation and reviewed video
   interval, plus all six speech windows. It then locks the gold state and
   writes a canonical digest with `GroundTruthValidation=PASS`. Current status
   is pending human work, so inference remains blocked.
5. **Verification:** four new workflow tests, Python compile and JavaScript
   syntax pass. Real source binding passes for 108 image paths/hashes and the
   video size/hash. Loopback state reports 10 development plus 22 holdout; HTTP
   range returns 206; browser rendering and 60.010-second video/audio metadata
   pass. Two focused actual-route specs, the complete NX-MMR route focus, Go
   vet and the full forensic API regression pass; the full suite is 324 specs
   (322 passed, two skipped) in 72.950 seconds. The older contact-sheet test
   remains unavailable in host Python only because Pillow is absent.
6. **Certification boundary:** the private gold may later contribute limited
   real-world evidence but cannot alone reach product certification. Routing
   remains shadow-only. An opt-in test routes an actual local JPG, MP4, M4A,
   PDF and Pakistan CDR CSV to image, video, audio, document and structured
   families with only applicable composed roles. M4A brand detection now keeps
   audio distinct from video; absent model roles remain `MODEL_REQUIRED`.
   Public dataset acquisition, live/API/Ask/UI acceptance, model admission,
   deployment and NX-B2.1 remain separate approval gates.
7. **Mutation boundary:** `RuntimeMutated=false`,
   `RetainedStateMutated=false`, `ActivityMutated=false`,
   `DatabaseMigration=false`, `VolumesChanged=false`,
   `DeploymentPerformed=false`, and `NXB2ActivationPerformed=false`.
8. **Exact next action:** from the repository root run
   `.\scripts\run_nexusai_nxmmr_human_verification.ps1`, complete the guided
   review, press Finish, and confirm `GroundTruthValidation=PASS` before any
   ANPR or ASR inference.

### 2026-08-30 — NX-MMR locked-human-gold ANPR/ASR empirical closure

1. **Oracle gate:** the independent human pack is locked with no validation
   errors, 10 frozen development images, 22 sealed-holdout images, 19 video
   plate events and six reviewed speech windows. Gold digest is
   `88c6827c23fbed467f097843404ed09c947bf7f27432e5cb5fa8427be9f48567`;
   model output was not used as truth.
2. **Image development:** the frozen incumbent detected 10/10 plates and
   exactly recognized 5/10; normalized CER is 0.184615. There were five OCR
   errors and no detector misses. Peak RSS was 279.691 MiB.
3. **One-time image holdout:** with the identical threshold/models/providers,
   it detected 20/22 and exactly recognized 13/22; exact precision/recall/F1
   are 0.650000/0.590909/0.619048 and CER is 0.201439. There were seven OCR
   errors and two detector misses. The receipt is immutable and must not be
   reused as an independent tuning oracle.
4. **Video:** the pre-oracle fixed one-second policy sampled 60 frames. It
   detected 3/19 human events and exactly recognized 3/41 event plates. Sampled
   frame presence was TP/FP/FN/TN 3/1/31/25. Exact grouping was 4/29 with F1
   0.242424 and 1.178250-second mean absolute boundary error. The current video
   policy/model combination is not admissible for promotion.
5. **ASR:** all six independently reviewed windows are negative and contain no
   transcript. `faster-whisper-small-ur` was not loaded or called; WER,
   identifier preservation, source-time and resource metrics remain honestly
   unmeasured rather than zero.
6. **Resource/safety:** host free RAM was 4.156 GiB before and 3.683 GiB after;
   each network-disabled worker was capped at four CPU and 2 GiB. Video wall/
   CPU/peak RSS were 63.878912 s/13.380812 s/273.211 MiB. The five live
   container identities were unchanged before and after.
7. **Certification boundary:** no ledger entry, visible suggestion, processor
   or operation was promoted. `PRODUCT_CERTIFIED` still requires representative
   real-world evidence plus live API/Ask/Data/citation/Activity/UI/security/
   performance/manual acceptance. NX-B2.1 remains paused.
8. **Mutation boundary:** `RuntimeMutated=false`,
   `RetainedStateMutated=false`, `ActivityMutated=false`,
   `DatabaseMigration=false`, `VolumesChanged=false`,
   `DeploymentPerformed=false`, `ModelsDownloaded=false`,
   `ModelsInstalled=false`, and `NXB2ActivationPerformed=false`.
9. **Exact next action:** prepare only a new approval pack for negative/boxed
   Pakistan images, separate development/sealed videos with track truth, and
   positive human-transcribed Urdu/Pakistani speech. Do not acquire, retune,
   rescore the completed holdout, activate services or start NX-B2.1 without
   that authorization. This historical next action is superseded by the
   2026-08-30 reference-parity ledger entry below.

### 2026-08-30 — NX-MMR reference-pipeline parity and baseline recovery

1. **Read-only recovery:** located and inspected the pre-existing FastALPR image
   and YOLOv8/SORT/EasyOCR video references without modifying either directory.
   The reference and oracle `sample.mp4` bytes, duration, frame count, frame
   rate, dimensions and audio/video streams match exactly.
2. **Image parity:** detector, OCR and configuration asset hashes match the
   acquired NX-MMR assets; relevant FastALPR/OpenVINO package versions match.
   Frozen development and one-time holdout predictions agree exactly with
   `CURRENT_NXMMR_BASELINE_V1` on 10/10 and 22/22 images. The reference scores
   remain development detection/exact/CER 10/10, 5/10, 0.184615 and holdout
   20/22, 13/22, 0.201439. Keep the current image baseline.
3. **Video reference score:** historical raw observations detect 18/19 events;
   exact counts are TP/FP/FN 30/239/11 because repeated OCR variants are kept.
   Historical final max-confidence-per-track output has exact TP/FP/FN
   24/18/17, F1 0.578314 and CER 0.170732. Its frame-presence F1 is 0.730470
   and exact-group F1 is 0.553846. This materially exceeds the current fixed
   one-second policy's 3/19 events and 3/41 exact plate occurrences.
4. **Root cause:** the current video path sparsely applies the 384-input image
   detector at a 0.75 threshold. The reference uses dense 60-fps frames, an
   existing dedicated 640-input detector, vehicle containment, repeated OCR
   and temporal candidate selection. Cadence contributes but does not explain
   14 events/30 plates that had sampled frames and no current detection.
5. **Selection:** keep `CURRENT_NXMMR_BASELINE_V1` for images. Select the raw
   historical video architecture only as a development adaptation candidate.
   Exclude strict UK normalization, persistent SORT identities and interpolated
   detections from forensic/product authority.
6. **Resource boundary:** the frozen image reference holdout peaked at 168.070
   MiB. Historical video artifacts contain no trustworthy runtime telemetry;
   an exact rerun was stopped before model load because 3.765 GiB free RAM was
   below the 4.5 GiB HEAVY floor.
7. **Oracle/certification:** no model output was an oracle. No operation/model
   was promoted; live API/Ask/Data/citation/Activity/UI/security/performance/
   manual gates remain mandatory before `PRODUCT_CERTIFIED`.
8. **Mutation boundary:** `RuntimeMutated=false`, `LiveModelsChanged=false`,
   `RetainedStateMutated=false`, `ActivityMutated=false`,
   `DatabaseMigration=false`, `VolumesChanged=false`,
   `DeploymentPerformed=false`, `ModelsDownloaded=false`,
   `ModelsInstalled=false`, and `NXB2ActivationPerformed=false`.
9. **Exact next action:** implement only source-level, non-retained
   `NX-MMR-REFERENCE-PARITY-ADAPTER-V1` with existing local hash-pinned assets,
   raw observations and bounded ephemeral adjacent-frame aggregation. Develop
   and freeze against development evidence only, then return for explicit
   approval before candidate evaluation or activation. This historical next
   action is completed and superseded by the ledger entry below.

### 2026-08-30 — NX-MMR reference-parity adapter V1 source-development closure

1. **Split authority:** froze a deterministic, model-output-independent
   overlap-safe partition before candidate inference: 11 development and 8
   reserved event components. Split digest is
   `a759572ba657d3385267abe06f3ae5f756708001759903b040b2d7741ad79a1b`.
   Reserved candidate metrics remain uncomputed.
2. **Implementation:** added a source-only video processor using hash-pinned
   dedicated 640 plate detection and EasyOCR; bounded fixed/adaptive scheduling;
   optional per-frame vehicle containment; actual-frame observations; neutral
   normalization; short-lived geometry/time association; deterministic
   best/majority/weighted aggregation; explicit coverage/failure states; and
   existing observation/group contracts. It is not live-registered.
3. **Development A/B:** compared fixed 2 FPS, fixed 4 FPS and 2+8 FPS adaptive
   bursts; best-confidence, majority and confidence-weighted aggregation;
   support 1/2/3; and plate-only versus vehicle context. Vehicle context gave
   only a marginal false-positive reduction and is optional.
4. **Frozen candidate:** selected fixed 4 FPS, plate-only, neutral majority,
   support >=2 and one vote per actual crop. Freeze digest is
   `1dcfabf17a8b66165b6c1390e65a8784eb875d3650b1b7f27d328c9ad2f0bb13`.
   Frozen adapter/config/development receipt hashes are recorded in
   `reports/nexusai-nxmmr-reference-parity-adapter-v1-20260830.md`.
5. **Measured result:** on 11 development events/24 plates/17 groups, event
   recall is 0.818182; exact TP/FP/FN is 2/12/22, F1 0.105263 and CER 0.583333;
   group TP/FP/FN is 1/17/16, F1 0.057143; negative-frame FPR is 0.333333.
   Wall/CPU/peak RSS are 180.568560 s/184.828125 s/1,154.211 MiB. This is a
   meaningful incumbent recall recovery but only `ParityAdapterV1=PARTIAL`.
6. **Forensic semantics:** `PersistentTracking=false`,
   `InterpolatedObservations=false`, `UKNormalization=false`,
   `RawOCRPreserved=true`, and `SourceTimeProvenance=PASS`. Model output was not
   used as oracle.
7. **Certification/routing:** no operation/model/query/suggestion was promoted;
   the candidate is absent from the live processor factory and default route.
   Live API/Ask/Data/citation/Activity/UI/security/performance/manual acceptance
   remains mandatory. Model-license admission and weak precision/group quality
   are the two open NX-MMR P1 items.
8. **Mutation boundary:** `RuntimeMutated=false`, `LiveModelsChanged=false`,
   `RetainedStateMutated=false`, `ActivityMutated=false`,
   `DatabaseMigration=false`, `VolumesChanged=false`,
   `DeploymentPerformed=false`, and `NXB2ActivationPerformed=false`.
9. **Exact next action:** obtain explicit authorization for exactly one local,
   private, non-retained evaluation of the frozen candidate against the 8
   reserved event components, with the metrics/resource scope in the closure
   report. Do not tune, alter the candidate, evaluate the image holdout,
   deploy, register routing, promote, acquire assets, or start NX-B2.1.

### 2026-08-31 — NX-MMR mixed-script product-acceptance diagnosis

1. **Operator result:** the corrected worker/API activation is operational and
   the operator reports successful video, English OCR and Urdu OCR acceptance.
   The retained `printed-mixed.png` result completed but its visible annotations
   omitted Urdu from the mixed Urdu/English line.
2. **Read-only finding:** evidence
   `54e35fb6-a858-4687-93ec-558e4369793d` contains three detector regions and
   three OCR observations, matching the fixture's three-line layout. For the
   expected `کیس Islamabad` region, the English recognizer returned only
   `Islamabad` at `0.999208`; the Arabic-multilingual recognizer returned
   `Islamabad كيس` at `0.957137`. The processor selected the English result.
3. **Root cause:** `multilingual_ocr.py` emits exactly one recognizer candidate
   per detected crop using maximum model-reported confidence. This
   winner-take-all arbitration loses valid cross-script content when a mixed
   crop gives the partial English result a higher confidence. Detection and
   upload are not the cause in this fixture. The model also rendered Arabic
   `ك` rather than Urdu `ک`, so exact Urdu fidelity remains review-required.
4. **Certification consequence:** English-only annotation of a mixed-script
   line is a reproducible product-acceptance failure. Mixed Urdu/English OCR
   remains not accepted and no OCR capability is promoted to
   `PRODUCT_CERTIFIED`. Standalone English and Urdu operator observations do not
   establish mixed-script accuracy without a locked human oracle.
5. **Mutation boundary:** diagnosis used read-only source and retained metadata/
   artifact inspection only. No evidence was uploaded, retried, reprocessed,
   deleted or edited; no database row, Activity record, model, service, volume
   or runtime configuration was changed.
6. **Exact next action:** authorize a bounded development-only source correction
   that preserves non-duplicate script-complementary candidates for a single
   detected crop, adds mixed-script ordering/annotation tests against the human
   fixture oracle, and performs no retained retry or deployment until an
   independently reviewed build/activation plan is approved.

### 2026-08-31 — NX-MMR mixed-script OCR source correction complete

1. **Bounded selection correction:** processor revision
   `nxmmr-anpr-ocr-vertical-v1-inputfix1-mixedfix1` now preserves a mixed
   Arabic/Latin candidate when it is a strict textual superset of the ordinary
   confidence winner and falls within a fixed `0.05` development confidence
   tolerance. All recognizer candidates, selected recognizer/confidence,
   selection policy/reason, Unicode order and manual-review status remain in the
   governed observation. No model output becomes an oracle.
2. **Observed-failure regression:** the exact retained candidate pair from the
   operator failure (`Islamabad` at `0.9992083311`; `Islamabad كيس` at
   `0.9571371675`) now emits the bilingual candidate as `mixed_arabic_latin`.
   Boundary coverage proves a `0.051` confidence gap does not override the
   winner, and a non-superset Arabic/Latin alternative does not replace a
   higher-confidence result. The model's Arabic `ك` versus Urdu `ک` variation
   remains visible and review-required; the processor does not fabricate a
   correction or reorder model text.
3. **Verification:** dependency-free NX-MMR ANPR/OCR vertical self-tests pass
   `19/19`; Python compilation passes; correction operator tests pass `25/25`;
   refreshed correction source seal passes; and changed-file whitespace checks
   pass. Host Python lacked `pytest`, so no dependency was installed; the
   repository's existing dependency-free runner executed every multilingual OCR
   test instead.
4. **Activation safety:** the worker image smoke now requires the new exact
   processor revision and the correction integrity manifest binds the reviewed
   source, tests, self-test runner and operator deployment script. Previously
   completed image receipt `20260831T115207942Z` contains the old processor and
   must not be reused for this correction.
5. **Mutation boundary:** source/tests/operator seal only. No model/package was
   installed or downloaded; no image was built; no service was stopped,
   recreated or deployed; and no retained evidence, processing job, database,
   Activity record, volume or model configuration was changed.
6. **Certification consequence:** mixed Urdu/English OCR remains
   `SOURCE_CORRECTED_RUNTIME_PENDING`, not product-accepted or
   `PRODUCT_CERTIFIED`. Existing standalone English/Urdu and video operator
   observations are unchanged.
7. **Exact next action:** obtain explicit approval for a fresh guarded
   worker-only build from the current live worker image, run the 19 tests plus a
   network-disabled model smoke, verify zero jobs/current identities/counts and
   rollback material, then recreate only `forensic-records-worker`. Do not reuse
   the old completed-build receipt, rebuild LocalAI/UI, reprocess the retained
   mixed image, or change any model/threshold/database/volume during activation.
   After activation, upload one fresh human-oracle mixed fixture for manual
   Data/annotation/citation/Activity acceptance; no direct promotion.

### 2026-08-31 — mixed OCR worker-only operator activation prepared

1. **Operator path:** added sealed
   `scripts/deploy_nxmmr_mixed_ocr_worker.ps1` and an exact standalone
   PowerShell runbook. The script is pinned to the currently verified five live
   container/image identities and refuses identity drift, nonzero jobs, changed
   retained counts, unhealthy services, source-seal drift or insufficient disk.
2. **RAM solution without gate bypass:** the operator may launch the command
   while Codex is open. After prechecks it explicitly waits for all Codex/
   ChatGPT processes to close. It then durably arms worker rollback, temporarily
   stops the old worker and LocalAI, and waits for the unchanged 6 GiB floor.
   LocalAI is neither rebuilt nor recreated; the exact same container/image is
   restarted after the candidate worker starts. Forensic API, PostgreSQL and
   NATS remain running and identity-protected.
3. **Build/smoke scope:** one network-disabled Docker layer copies only the
   reviewed `multilingual_ocr.py` onto the current worker image. The candidate
   must pass exact source/revision verification and a network-disabled,
   generated-pixel Paddle model smoke mounted only to admitted models and the
   private receipt. No retained spool is mounted or accessed and no package,
   model or dataset download is possible.
4. **Recovery:** any failure after drain first restarts the exact original
   LocalAI container and then invokes the existing worker-only rollback receipt.
   Candidate activation recreates only `forensic-records-worker`; environment,
   mounts, health, readiness, protected identities, zero jobs and retained
   counts are reverified before PASS.
5. **Verification:** OCR vertical tests pass `19/19`; operator static/safety
   checks pass `35/35`; PowerShell parsing and refreshed correction source seal
   pass. A non-deploying live preflight reached exact current identities and old
   processor revision, then correctly blocked at 3.896 GiB with Codex open.
   Receipt `private-activation/20260831T144221361Z`; no service was stopped or
   replaced.
6. **Exact next action:** from standalone Windows PowerShell run
   `powershell.exe -NoProfile -ExecutionPolicy Bypass -File
   .\\scripts\\deploy_nxmmr_mixed_ocr_worker.ps1 -WaitForRamMinutes 30`, close
   Codex/ChatGPT/browsers only when prompted, and wait without interruption for
   `MIXED_OCR_WORKER_DEPLOYMENT=PASS` or the final failure/rollback marker. Do
   not use the old completed-build receipt or any historical activation script.
   Fresh mixed-fixture product acceptance remains a later separate gate.

### 2026-08-31 — first mixed OCR activation safely failed; smoke corrected

1. **Run result:** operator receipt
   `private-activation/20260831T144849406Z` passed 19 tests, exact identity/jobs/
   counts guards, application-close and 6 GiB gates, then built the source-only
   candidate successfully. Candidate manifest-list digest is
   `sha256:711d47eb848b4d93ffa905c13ac158b4edb662862d68a657e65b290c10fd3787`.
2. **Failure cause:** the network-disabled smoke correctly retained a read-only
   candidate filesystem, but the live worker's configured home is
   `/nonexistent`. PaddleX attempted to create `/nonexistent/.paddlex/temp` and
   failed before model inference. This is a smoke sandbox configuration defect,
   not an OCR result/model/source failure. Retained evidence was not mounted or
   accessed and downloads remained false.
3. **Recovery verification:** LocalAI restarted with its exact prior container/
   image. Worker rollback recreated a healthy container
   `8ba318c294fd...` on the exact prior image
   `sha256:2755e0ffbf...`, revision
   `nxmmr-anpr-ocr-vertical-v1-inputfix1`. Forensic API/PostgreSQL/NATS
   identities are unchanged and healthy/running; jobs are zero and counts match
   pre-run exactly `58|58|71|22507|812|54`. The receipt lacks a final rollback
   marker because its health-wait pipeline was interrupted after the old worker
   started; independent live verification establishes recovered state.
4. **Bounded correction:** candidate smoke now keeps the root filesystem
   read-only while setting `HOME=/tmp` and
   `PADDLE_PDX_CACHE_HOME=/tmp/.paddlex` on the existing bounded tmpfs. The
   operator script is renewed to the recovered worker identity and the seal/
   static checks pass `36/36`.
5. **Empirical smoke:** the already-built candidate passed the corrected
   network-disabled generated-pixel smoke: blank
   `COMPLETE_ZERO_RESULTS`, printed control `COMPLETE_RESULTS`, named versus
   extensionless parity, source unchanged, retained access false, downloads
   false, revision `inputfix1-mixedfix1`; wall 9.391 s and peak RSS 696.086 MiB.
6. **Exact next action:** rerun the same standalone worker-only activation
   command, close Codex/ChatGPT/browsers only at its prompt, and do not reopen or
   interrupt the external PowerShell until a final PASS or explicit rollback
   marker. No manual rollback is currently required because the recovered live
   state has been independently verified.

### 2026-08-31 — second mixed OCR run interrupted at RAM gate; bounded recovery prepared

1. **Interrupted run:** receipt `private-activation/20260831T150043817Z`
   passed 19 tests and exact guards, closed operator apps and drained the old
   worker plus unchanged LocalAI, but available RAM stabilized between roughly
   5.73 and 5.84 GiB. The external PowerShell pipeline ended without a PASS,
   FAILED or rollback marker while both targets were still stopped.
2. **Immediate recovery:** independently restarted the exact original worker
   `8ba318c294fd...` and LocalAI `ede93502d542...` containers without recreation.
   Both are now healthy with restart count zero on their prior images. Forensic
   API/PostgreSQL/NATS remained running; jobs are zero and retained counts match
   the run's before-state exactly `58|58|71|22507|812|54`.
3. **Memory evidence:** with Codex reopened, vmmemWSL was 3,861.6 MiB; five
   ChatGPT processes plus Codex used about 1,580 MiB; Phone Link used 161.1 MiB
   and LockApp 72.7 MiB. Defender, Memory Compression, Explorer and Docker are
   not cleanup targets. Disk/model/image deletion is irrelevant to the missing
   physical RAM and remains forbidden.
4. **Explicit recovery mode:** sealed `-RecoverRam` stops only
   `PhoneExperienceHost` and `LockApp` after Codex/ChatGPT close. After guarded
   worker/LocalAI drain, it reads Docker Desktop Linux Dirty/Writeback counters
   and executes `drop_caches=1` only when both are exactly zero. It never invokes
   sync, `drop_caches=3`, Defender/Explorer/Docker termination, file/image/model/
   volume pruning, swap edits or a gate reduction.
5. **Verification:** the Dirty/Writeback parser was exercised read-only and
   returned two governed values; script parsing, renewed source seal and operator
   safety tests pass `40/40`. Current worker and LocalAI health are independently
   reverified PASS.
6. **Exact next action:** run the worker activation with `-RecoverRam
   -WaitForRamMinutes 30` from standalone PowerShell. Do not press Ctrl+C or
   reopen Codex merely because the 10-second RAM readings repeat; wait for the
   full final PASS or failure/rollback marker.

### 2026-09-01 — NX-MMR governed transcript query and live-integration source closure

1. **Shared Ask P1 source resolution:** selected evidence ID/current version,
   source family and completed result families now cross the common UI, chat,
   agent tool and forensic API path. Selected evidence is the default; only
   explicit whole-investigation language broadens scope. Typed-plan validation
   rejects scope/parameter divergence and unauthorized or stale scope.
2. **Transcript authority:** current-version derived transcript lookup supports
   exact NFKC Unicode matching, inclusive source-time intersection, truthful no
   match, English/Urdu/Roman Urdu routing, raw-first ordering and Roman-parent
   lineage. The worker persists ASR role state separately, so completed
   technical metadata cannot masquerade as completed-zero transcription.
3. **Baseline decision:** existing `faster-whisper-small-ur` remains the demo
   baseline. English FLEURS WER/CER is 10.795/3.229%; supplied Urdu two-clip
   WER/CER is 45.45/16.46% with substantial limitation. Fine-tuning/model search
   is deferred.
4. **Breadth preserved:** the source-ready activation enables existing ASR,
   hash-pinned seven-file SigLIP and YuNet/SFace candidate roles while retaining
   ANPR/OCR/VideoV3. TTS stays disabled: English lacks complete installed-size/
   packaged dependency notice closure; Urdu lacks a publicly resolvable exact
   `charactr/vocos-mel-22khz` artifact.
5. **Verification:** critical source query matrix 16/16; full forensic API,
   focused agent/API, 22 worker unittests, Python compile/role-state smoke, 14
   frontend tests and 27 operator source checks pass. Live read-only operator
   checks pass 29/29 and preserve all container identities; preflight blocks
   only because 5.087 GiB is below the corrected 6 GiB floor.
6. **Mutation boundary:** source/tests/docs and read-only runtime inspection
   only. No build, image, restart, recreation, migration, volume, model,
   retained evidence, job, Activity or database mutation occurred.
7. **Exact next action:** manually close unnecessary applications until at
   least 6 GiB is available, then run the sealed three-service bundle from
   standalone PowerShell. It builds sequentially and rechecks RAM before each
   build and before recreation; it never terminates applications itself.

### 2026-09-01 — NX-MMR post-build RAM failure repaired with sealed resume

1. **Observed failure:** the first operator run passed its 6 GiB gates at
   6.884/6.793 GiB and built the API candidate successfully, but available RAM
   fell from 6.198 GiB before that build to 3.504 GiB afterward. The former
   abort-only gate stopped before the next image; no service was recreated.
2. **Root cause:** Docker/BuildKit clean page-cache pressure after a successful
   build made the immediate 6 GiB recheck predictably fail on this host. The
   safety floor itself is correct and remains 6 GiB; an 8 GiB gate is rejected
   as infeasible on the available machine.
3. **Bounded correction:** a failed RAM check now waits up to 30 minutes and
   can release Docker Desktop Linux clean page cache using mode 1 only after
   both Dirty and Writeback are exactly zero. It never invokes sync, mode 3,
   application termination, prune, service drain or a reduced RAM gate.
4. **Progress preservation:** every completed candidate image is immediately
   recorded with immutable image ID, current manifest hash, current source-seal
   hash and the unchanged original container IDs. A later invocation reuses
   only that exact sealed build set and rejects tag drift or any recreation.
5. **Legacy candidate boundary:** the API image from the pre-correction run has
   no per-image source-sealed build receipt and is therefore not auto-admitted;
   the first corrected retry may rebuild it from cached layers. Subsequent
   completed images are resumable.
6. **Verification:** source/safety self-test passes 30/30. Final escalated
   read-only live verification passes 32/32, including integrity and unchanged
   service identities; it measured 3.948 GiB and correctly remained blocked
   only by the 6 GiB RAM floor. No activation or runtime mutation was performed.

### 2026-09-01 — NX-MMR STT/query three-service activation recovered and verified

1. **Initial recovery:** clean Docker Linux page cache was released in mode 1
   only at exact zero Dirty/Writeback. Copilot, Command Palette, OneDrive,
   background Edge, Phone Link and LockApp were closed; reclaimable GUI working
   sets were trimmed without terminating Codex/ChatGPT. RAM preflight passed at
   6.149 GiB with zero jobs and healthy protected services.
2. **Operator defects closed:** WSL memory commands now use a fixed-command
   launcher because quoted `wsl.exe` switches were interpreted as Linux command
   names on this host. Safe recovery retries after dirty writes drain. The
   isolated readiness compose adds absent entrypoint/command properties safely,
   and generated CMD-SHELL healthchecks escape `$` for container-side variable
   expansion.
3. **Build and activation:** API, forensic API and worker rebuilt sequentially.
   Final live immutable images are `sha256:130f3f40686c...`,
   `sha256:5c1d71cd903c...` and `sha256:a7ca5956dcc1...`. No model acquisition,
   migration, volume replacement, retained reprocessing or protected-service
   recreation occurred.
4. **Health reconciliation:** the first API recreation rendered its host-side
   health variable empty. A corrected one-service candidate recreation restored
   the literal container-side endpoint and became healthy. The timed-out
   verifier's concurrent rollback failed before changing the candidate set and
   left one non-running temporary forensic-API container; that exact temporary
   container was inspected and removed.
5. **Final proof:** independent verification PASS at receipt
   `private-activation-stt-query/20260901T154458190Z`. Final operator tests pass
   34/34 at 6.161 GiB, with all candidate images stable, restart count zero,
   exact worker roles/assets/models, zero jobs, and PostgreSQL/NATS identities
   unchanged. Product/live query acceptance remains pending.

### 2026-09-04 — NX-B2.1D one-shot model evaluation terminal failure and admission boundary

1. **Frozen run:** authorized Qwen evaluation run
   `evaluation-20260904T053938055Z` froze source digest
   `34c69067628a58373d471aa8e75d7a17a2c4a7f789b521f62c5baa38f8a5478d`,
   manifest SHA-256
   `9e9507fb174cef2f356c452c793e2905329af5de19c2791d8ae0bb46332e42bb`
   and holdout SHA-256
   `eb102cdd1f1fa1d9da02c5dce4c8c9e216a523ba78d4468ce9802f880170283f`
   before first inference.
2. **Terminal outcome:** 81/168 cases were attempted: 80
   `malformed_proposal`, one `timeout_or_unavailable`, zero passes and zero
   schema-valid outputs. Ginkgo's separate one-hour suite timeout ended the run
   before all 80 critical cases, so critical safety is
   `NOT_REACHED_UNMEASURED`. Every 95% language gate is already mathematically
   unreachable even if every unattempted case passed.
3. **Validity decision:** the live profile did not match the frozen candidate.
   The manifest specified context 8192, while LocalAI reported effective context
   4096. The live YAML default temperature was 0.6, but the request set zero.
   `function.grammar.disable` governs generated function-call grammar; the
   requested `response_format` schema follows a separate LocalAI path. No
   grammar-generation errors were logged. Because malformed raw completions were
   not preserved, their exact cause remains unresolved. The run is
   `INVALID_ABORTED_CONFIGURATION_MISMATCH_AND_SUITE_TIMEOUT`. The deployed Q8
   profile is `INSUFFICIENT`; intrinsic Qwen base-model suitability is not
   classifiable from this invalid run.
4. **Resource result:** median case latency was 42.814 seconds, p95 58.108
   seconds, minimum host available RAM 1.412 GiB, maximum API-container memory
   5,951.488 MiB and maximum observed CPU 810.52%. This is diagnostic evidence,
   not a production SLA.
5. **Post-unload proof:** the documented exact LocalAI shutdown path unloaded
   Qwen. `/system` lists only `qwen3-embedding-0.6b`. All five container IDs,
   images and restart counts match baseline; all are healthy/running, active
   jobs are zero, retained tuple is `64|64|77|22507|828|61`, Activity count is
   303 with newest row predating the evaluation, and no deployment, migration,
   volume change, retained write or model download occurred.
6. **Durable artifacts:** public failure receipt
   `reports/nxb21/d-model-evaluation-receipt-v1.json` has SHA-256
   `eba308ad2a948d615bafa48fb54ef959c3fb303ec278739c3c38bc1d2c33dba5`.
   Private case-level reconstruction remains under the ignored run directory.
   `reports/nxb21/d-model-admission-required-v1.{md,json}` records pinned
   candidates and the separate acquisition boundary.
7. **Safety hardening:** the offline bundle now passes 29/29 checks, blocks the
   consumed holdout, uses LocalAI `/system` for model state, performs
   unconditional safe unload after inference may begin, sets Go and Ginkgo
   four-hour timeouts, streams verbose output and saves a checkpoint after every
   case. The activation runner rejects the failed receipt and no activation seal
   exists.
8. **Exact next action:** obtain separate owner approval for the pinned Qwen3
   4B Instruct 2507 Q4_K_M artifact and isolated non-retained development
   qualification. Fix/validate the profile, schema transport and prompt only on
   development data, then freeze a new independent holdout. Never rerun or tune
   against the consumed corpus; do not activate B1+C+D before a new valid D PASS.

### 2026-09-04 — NX-B2.1D Q4 byte-safe 32-case development framework source validation

1. **Reconciled Q4 candidate:** artifact SHA-256
   `2fde00ce69dd4899c70d020845e2638353015bba0fdf161b3eb965f2bca4464e`,
   isolated profile SHA-256
   `30d3bcd8c92684e191277dad992354323b7e46c7d30d3c69149010e90f117129`,
   prompt SHA-256
   `3ce8225e30beacb51430dc1f377ee58fbb0e5f2746a44660e099dddac9fc67bc`,
   schema SHA-256
   `be217f9e6ddec85131e5b0f60ac8f4050d18b5632e838b1560bc6b1067a3d731`,
   request temperature zero and effective development context 4096 all match the
   candidate tuple. The LocalAI container remains `d62021a0a4fc...` on image
   `sha256:feb935b42a93...`, restart count zero; Q4 is registered and was loaded
   at source reconciliation.
2. **12-case adjudication:** the original mechanical 9/3 summary remains
   preserved. Raw-byte strict-UTF-8 replay proved all three apparent Urdu
   failures were client decoding false negatives, so the adjudicated development
   result is 12/12 PASS. Receipt
   `local-acceptance-models/nxb21-d/challengers/qwen3-4b-instruct-2507-q4km/localai-dev-results/12case-smoke/12case-development-smoke-adjudicated-v1.json`
   has SHA-256
   `75ffdda521a33af075b4e03f95270cf32da64031968306e650774b87eeadc12a`.
3. **Transport defect and fix:** Windows PowerShell
   `Invoke-WebRequest.Content` is prohibited as a multilingual correctness
   oracle, and BOM-bearing saved JSON must not be transmitted byte-for-byte.
   The reusable transport writes deterministic ordered JSON as UTF-8 without
   BOM, sends byte arrays with `HttpClient`, captures raw response bytes, applies
   strict UTF-8 decoding, and only then parses the OpenAI envelope and planner
   object. It preserves request/response hashes and exact codepoint plus
   normalization diagnostics without normalizing the authoritative comparison.
4. **Fresh development corpus:** exactly 32 fresh cases are frozen under
   `scripts/nxb21d-development/`: eight English, eight Urdu-script, eight Roman
   Urdu and eight mixed; 12 `image.plate`, 12 `document.phrase`, eight
   `transcript.phrase`; 12 `EXACT_VALUE`, 20 `PHRASE_CONTAINS`; 16 selected and
   16 workspace. Deterministic scanning found no overlap with the consumed
   168-case holdout or locally accessible earlier development questions. Corpus
   SHA-256 is
   `01647f6240d9bcbabbc758c153ba7eea454458efb9348a57e22f6a0a1ed63183`.
5. **Source validation:** 14/14 focused UTF-8, raw-body, bounded frozen-schema,
   Windows PowerShell 5.1 strict-input
   serialization, schema-shape, corpus, syntax,
   integrity and runner-safety checks pass. The live 32 cases were not executed
   in Codex. No model/profile change, download, build, recreate, deployment,
   database or retained-evidence mutation occurred.
6. **Boundary:** D remains OPEN, Q4 qualification is NOT YET, and activation
   remains BLOCKED. The final independent qualification holdout has not been
   frozen. Next, close Chrome, ChatGPT/Codex, VS Code and other heavy apps, leave
   Docker Desktop running, execute
   `scripts/nxb21d-development/run_nxb21d_q4_32case_development.ps1` from
   standalone PowerShell, and return its receipt for review.
7. **First standalone invocation correction:** run
   `run-20260904T140723387Z` passed corpus, candidate, registration and 4 GiB
   loaded-model gates, then Windows PowerShell 5.1 expanded the depth-50 imported
   schema serialization before request-file creation. No HTTP request or live
   case completed. The process was stopped at an observed 8,952,647,680-byte
   working set. The invalid run and log are preserved; derived abort receipt
   SHA-256 is
   `901ad930ffed12a7cc50a679f5e6f17335f2403c86a2b2f882fee87110334658`.
   Serialization depth is now bounded to 12, with an actual frozen prompt/schema
   regression requiring completion within five seconds, request size below 100
   KB and no BOM.
8. **Second standalone invocation correction:** run
   `run-20260904T150006413Z` also stopped before request-file creation or HTTP.
   Its peak observed working set was 1,214,550,016 bytes. An isolated guarded
   reproduction proved Windows PowerShell 5.1 default text decoding of the
   BOM-less corpus/prompt path triggered the remaining CPU loop, while strict
   UTF-8 decoding serialized the same frozen request in 35 ms at 3,282 bytes.
   Every evaluator JSON/prompt read now uses strict UTF-8; a legitimate input BOM
   is stripped during decoding, the corpus and outgoing request retain their
   no-BOM gates, and explicit serialize/save/post markers expose transport
   progress. Invalid abort receipt SHA-256 is
   `1ddb04bbc255e02a0e06c8a20eb85a3e0e517b2d3ecabec2e426d74d6be8ff0d`.
   Zero live cases have completed across either invalid run, so the development
   corpus remains reusable; corrected rerun is pending.
9. **Corrected 32-case development result:** run
   `run-20260904T151011658Z` completed all 32 byte-safe HTTP requests with HTTP
   200, zero invalid UTF-8 responses, zero malformed planner responses and zero
   schema-incompatible proposals. Result is 31 PASS / 1 FAIL. The sole failure,
   `dev32-en-002`, preserved exact literal `ISB-5907`, capability, semantic,
   scope and clarification, but returned entities `ISB-590` and `5907` instead
   of the required atomic entity `ISB-5907`; this is a real semantic-oracle
   failure. Latency was 8,701 ms minimum, 11,564.5 ms median, 16,106 ms p95 and
   16,883 ms maximum; 27 exceeded 10 seconds, none exceeded 20 or 30 seconds.
   Minimum available host RAM was 6.078 GiB and maximum LocalAI memory was
   4,285.44 MiB. Runtime identity/profile/restart integrity passed.
10. **Summary recovery:** after all cases and runtime integrity completed,
    Windows PowerShell 5.1 rejected direct `@($results)` conversion of the
    generic result list with `Argument types do not match`. The runner now
    enumerates the list through the pipeline and the PS5.1 regression covers
    that behavior. The missing aggregate was recovered without inference from
    all 32 case receipts after rechecking corpus identity, exact questions and
    oracles, raw request/response hashes, strict UTF-8 and no-BOM requests.
    Receipt SHA-256 is
    `bf73696f2fc89ee02d26027818ee9d0b838926d1cbaf67a7276829a9b1321132`.
    D remains OPEN, Q4 is not qualified, no independent holdout is frozen, and
    activation remains BLOCKED. Do not rerun this corpus as qualification.


### 2026-09-05 — NX-B2.1D Q4 atomic-identifier remediation refrozen

1. **Predecessor preserved:** completed development run
   `run-20260904T151011658Z` remains 31/32 FAIL. Its receipt SHA-256 is
   `bf73696f2fc89ee02d26027818ee9d0b838926d1cbaf67a7276829a9b1321132`;
   `dev32-en-002` is the sole failure because model entities split the exact
   source identifier `ISB-5907`. It will not be rerun or treated as an
   independent qualification holdout.
2. **Shared typed-boundary correction:** `image.plate` plus `EXACT_VALUE`
   now derives exact identifier authority from the original question at the
   production proposal-validation boundary. Exactly one bounded source span is
   preserved byte/codepoint exactly and finalized as one literal/entity.
   Missing or multiple spans require clarification. The raw model proposal,
   mismatch reasons, reconciliation decision and final typed plan remain
   separately auditable. Phrase capabilities are unchanged.
3. **Explicit candidate refreeze:** candidate
   `nxb21d-q4-atomic-identifier-authority-r1` has identity SHA-256
   `1ccc03ab44a25cb20ec28053b898c2f975b6373829303e8265833164a89e2ccc`.
   Model artifact, Q4/Q8 profiles, prompt, schema, temperature zero and context
   4096 are unchanged. The identity also binds the production implementation
   SHA-256 `85ad1fdf89a9794193896580595ecd33b054ba206e028467406824beea590828`,
   query bridge `95cbeb0547b616eccc326c27f4c22744638f109b949e1321097a4e7571e08164`,
   PowerShell authority `216b0ec4e2b69c5482c65a3eef306f41b170a11a38a4b2b7a715b81134c7d304`
   and byte-safe transport
   `9deed398e86e0347afc33964c47b8f4e489cc88372a77d16b4ba9ef8ed4a8c64`.
4. **Fresh development evidence frozen:** 16 new questions have corpus SHA-256
   `937f44c241dce1abc6f0ca1781b865b8dc0d49c89ed804411c1ac58e6a329cec`.
   There are four cases per language, eight exact plate cases and eight phrase
   controls. All plate cases are selected scope because the registered
   `anpr.sightings` contract permits only `selected_evidence`; phrase
   controls exercise selected and workspace scope. The validator proves all
   questions and literals are fresh against the consumed holdout and prior
   development material.
5. **Source validation:** production planner tests pass 25 focused specs,
   remediation framework tests pass 94 assertions, the earlier byte-safe
   framework remains 14/14 PASS, and corpus validation passes. No live
   inference, service mutation, database write, model/profile change,
   deployment or activation occurred.
6. **Current gate:** remediation candidate is **REFROZEN** and the fresh
   development corpus is **FROZEN_NOT_EXECUTED**. D remains **OPEN**,
   qualification holdout remains **NOT_FROZEN**, and activation remains
   **BLOCKED**. The consumed 168-case Q8 holdout is permanently retired.


### 2026-09-05 — NX-B2.1D remediation development run reviewed

1. **Valid completed run:** standalone run
   `run-20260905T054458677Z` executed all 16 frozen cases. Aggregate receipt
   SHA-256 is
   `83ac55c7cd9627f0d37e21f33c556245bc6e7e62f35fb267d66f95b6872c23f3`;
   its sidecar and all 16 saved request/response hashes independently match.
   HTTP, strict UTF-8, envelope, planner JSON and schema gates had zero errors.
   Runtime identity, image, profiles and restart count remained unchanged.
2. **Exact-identifier outcome:** all eight `image.plate` cases pass the final
   typed-plan oracle. Six model proposals were already correct. In
   `rem16-en-plate-002` and `rem16-ur-plate-002`, the model omitted the
   entity; deterministic authority recorded that weakness and finalized exact
   atomic identifiers `TRV-2048` and `ZXA-615`. This validates the intended
   exact-identifier remediation on this bounded development corpus.
3. **Control failures:** final result is 13/16 PASS and therefore overall
   DEVELOPMENT FAIL. `rem16-ur-document-001` substituted Cyrillic U+0430 and
   U+043C for Arabic U+0627 and U+0645 inside the Urdu literal, producing text
   absent from the question. `rem16-ur-transcript-001` and
   `rem16-roman-transcript-001` returned empty literals with false
   clarification. These are model phrase-literal preservation failures in the
   unchanged, non-applicable phrase path; they are not transport, schema, scope
   or exact-identifier-authority failures.
4. **Measured result:** raw model proposals are 11/16 PASS; final typed plans
   are 13/16 PASS. By capability: image plate 8/8, document phrase 3/4,
   transcript phrase 2/4. By language: English 4/4, Urdu script 2/4, Roman Urdu
   3/4, mixed 4/4. Median latency is 9,864 ms, p95 16,396 ms, minimum available
   host RAM 4.902 GiB and maximum LocalAI memory 4,313.088 MiB.
5. **Decision:** preserve this run and do not rerun its questions. Candidate
   `nxb21d-q4-atomic-identifier-authority-r1` is
   **DEVELOPMENT_FAIL_DO_NOT_ADVANCE**. D remains **OPEN**, qualification
   holdout remains **NOT_FROZEN**, E is not accepted, and activation remains
   **BLOCKED**. Any phrase-literal correction requires a separate explicitly
   refrozen candidate and fresh development questions.

### 2026-09-05 — NX-B2.1D phrase-literal authority candidate refrozen

1. **Immutable predecessor evidence:** the 32-case receipt remains 31/32 FAIL
   at SHA-256
   `bf73696f2fc89ee02d26027818ee9d0b838926d1cbaf67a7276829a9b1321132`.
   The atomic 16-case receipt remains 13/16 final PASS, including exact plate
   8/8 and phrase controls 5/8, at SHA-256
   `83ac55c7cd9627f0d37e21f33c556245bc6e7e62f35fb267d66f95b6872c23f3`.
   Both corpora and the consumed Q8 holdout are retired and must not be rerun.
2. **Bounded production correction:** phrase authority uses the existing
   forensic-text parser to accept exactly one explicit quoted source span only
   when independently detected document or transcript family context agrees.
   Source codepoints become the final literal and typed text query. Raw model
   literal, clarification, normalized comparison, codepoints, mismatch reasons
   and the Arabic/Cyrillic script diagnostic remain auditable. Multiple quoted
   spans and contradictory family context fail closed. Unquoted requests stay
   on the model-planned path; no substring or homoglyph-conversion heuristic was
   added.
3. **New refreeze:** candidate `nxb21d-q4-phrase-literal-authority-r1` has
   identity SHA-256
   `0129bd4ab0d660573726c744f6ba9874a9a15b9343a87cc5b37a264dcadf675e`.
   It binds the unchanged model/profile/prompt/schema, temperature zero,
   context 4096, unchanged exact authority, byte-safe transport, production
   implementation and new phrase-authority source.
4. **Fresh development corpus:** 20 new questions are frozen at SHA-256
   `7c273c3b45fd3103234f098cd38bce84348c8cfff27b243f6c6987a5dd1555e6`:
   five per language; eight document phrase, eight transcript phrase and four
   exact plate controls. Validation confirms unique questions/literals and no
   overlap with the consumed Q8, completed 32-case or atomic 16-case corpora.
5. **Source validation:** 35 focused planner specs and the full
   `api/forensic_records` package pass; the phrase framework passes 133
   assertions with corpus validation; the prior byte-safe framework remains
   14/14 PASS. The retired Q8 framework correctly retains its historical
   source-manifest drift failure after intentional current source changes.
6. **Current gate:** candidate state is
   **REFROZEN_SOURCE_VALIDATED_DEVELOPMENT_NOT_EXECUTED**. D remains **OPEN**,
   qualification holdout is **NOT_FROZEN**, E is not accepted, and activation
   remains **BLOCKED**. Only the new standalone 20-case bundle may run next.

### 2026-09-05 — NX-B2.1D phrase run reviewed and scope candidate refrozen

1. **Valid immutable run:** `run-20260905T062620032Z` completed all 20 fresh
   requests. Receipt SHA-256 is
   `02a7981aa4d4918255ef056e0008250baa7fd265e72db5e10cf510da934123a0`;
   its sidecar, all request/response hashes, strict UTF-8, schema and runtime
   integrity pass. Result is 19/20 for both raw model and final typed plans.
2. **Phrase objective achieved:** all 16 document/transcript phrase literals
   pass exactly, including Urdu, Roman Urdu and mixed cases; all four exact
   plate controls also pass. No deterministic correction was needed in this
   sample, so model and final results are independently 19/20.
3. **Only failure:** `phrase20-roman-tr-001` preserved capability, semantic,
   literal `pichla rasta band rakho` and clarification, but returned `selected`
   instead of the explicit workspace scope in `tamam transcript files`. Request
   SHA-256 is `d0d74810...6e2d8`; response SHA-256 is
   `19445841...37c72`. This is a model/shared explicit-scope vocabulary miss,
   not a phrase-literal, Unicode, transport, schema or authority failure.
4. **Shared correction:** bounded selected/workspace phrases outside quoted
   evidence text now use one shared forensic-text authority across API and
   agent routing. It adds generic English, Roman Urdu and Urdu transcript and
   document collection forms. Scope can change only inside the authorized case;
   contradictory explicit scopes require clarification, quoted evidence cannot
   steer scope, and implicit scope stays unchanged.
5. **Separate refreeze:** candidate `nxb21d-q4-explicit-scope-authority-r1`
   has identity SHA-256
   `ba17c5c688eaa567ce2d006d8a0a80948a4e01e1b3c62f0765c84aeaa4d9d23a`.
   Model/profile/prompt/schema, exact authority, phrase authority, transport,
   temperature zero and context 4096 are unchanged.
6. **Fresh evidence:** 16 new cases are frozen at SHA-256
   `e034bb3683f23c1aeb7488232a7337c56d5e9eb7cb69f9f280698af3412b1b50`:
   four per language, six document phrases, six transcript phrases, four plate
   controls, eight workspace and eight selected. Freshness and UTF-8 validation
   pass. The corpus is **FROZEN_NOT_EXECUTED**.
7. **Source gates:** focused API and agent scope tests pass; full
   `api/forensic_records` passes; scope framework passes 120 assertions. The
   full agents package reached 130 PASS/1 skipped but 15 unrelated DB-backed
   specs could not start because Docker Desktop was unavailable. D remains
   **OPEN**, qualification is **NOT_FROZEN**, and activation is **BLOCKED**.

### 2026-09-05 — scope-run preflight aborts preserved and health wait hardened

1. Runs `run-20260905T075458031Z` and `run-20260905T075506272Z` both aborted
   before case 1 with `LOCALAI_CONTAINER_UNHEALTHY`. Their receipts contain
   zero executed cases and are preserved at SHA-256
   `b70774be1e521bc1dc33c13df155409868d180f2b6f2c365eafbfb8c40d4f7f8`
   and `af4f7dffaae8b528f8f2d1951cd37d57a33c557d344cf7633d3ae182b884f44a`.
   No development question or literal was sent, so the frozen 16-case corpus
   remains fresh and may be executed once.
2. **Root cause:** the preserved exact LocalAI container was running after a
   Docker engine restart but was still in its startup health interval. It
   reported healthy shortly after both preflight aborts. Container ID, image
   digest and restart-count contract remained unchanged.
3. **Runner hardening:** preflight now waits up to 180 seconds for the existing
   named container, reports status every five seconds, and fails immediately
   for a stopped or dead container. It does not restart or recreate services.
4. **RAM preflight evidence and correction:** after the health correction,
   `run-20260905T080101493Z` reached the RAM gate and stopped before case 1 at
   3.858 GiB available versus the preserved 6 GiB preload floor. Its zero-case
   receipt SHA-256 is
   `a6eea5c45bed615d569dbab89a4302253d0f39ff17d8765fa996c24d5c9fc5a3`.
   Preload now uses the proven safe clean-page-cache recovery and waits up to
   180 seconds without lowering the gate, stopping WSL or restarting Docker.
   Runner SHA-256 is
   `f3a5de9d8b06919f5e8543af202ab43186e0d69621ac5b3a1f99c455d3ca83ca`;
   config SHA-256 is
   `ef167b2d5b5388021dee8ae59c4e8e0daa8ae5b0d840a4b571ecc7166a86cb12`.
5. **Validation:** the refrozen framework passes 125 assertions and corpus
   validation still passes with unchanged corpus SHA-256
   `e034bb3683f23c1aeb7488232a7337c56d5e9eb7cb69f9f280698af3412b1b50`.
   No live inference was run from Codex. D remains **OPEN**, qualification is
   **NOT_FROZEN**, and activation remains **BLOCKED**.

### 2026-09-05 — scope development case 1 preserved and resumable execution sealed

1. `run-20260905T080508522Z` passed health, safe cache recovery, the 6 GiB
   preload gate, registration and candidate refreeze. Case
   `scope16-en-tr-workspace-001` completed with HTTP 200, byte-safe transport,
   raw model PASS and final typed-plan PASS. Request SHA-256 is
   `eecaf4ca61726e964d4665eca8e44f73fab0ffdfcfce1a1787c9e90f932147cb`;
   response SHA-256 is
   `8d0c0c83056016f3192d7215770dd88a0f5a5e47b5b27146f07e0f4656c50a74`.
2. The run then stopped before case 2 because available RAM was 3.525 GiB
   against the unchanged 4 GiB loaded-model floor. Its immutable partial
   receipt SHA-256 is
   `d61e6ffb44513f3448713a0cbab69db126f597fa9426c620b88a73f955a7fa33`.
   Case 1 must not be sent again; cases 2 through 16 remain unconsumed.
3. The runner now discovers completed prefix cases only from aggregate receipts
   whose sidecar, candidate identity, corpus hash, question, oracle, evidence
   paths and raw request/response hashes all verify. It preserves those results
   in the next aggregate receipt and skips their inference. Gaps or conflicting
   duplicate evidence fail closed. The same bounded safe RAM recovery now runs
   before every remaining case without lowering the 4 GiB loaded-model floor.
4. Updated runner SHA-256 is
   `15f1f8fde5b74f6438b863b8660f13c23fd89e53e858de93333ca3fb296087b9`;
   config SHA-256 is
   `39d8a0fd52ad9b671eb0f4f9a357003516e85ffc8d836f54adf5b835ccde1351`.
   Framework validation passes 135 assertions, including a live filesystem
   proof that the sealed case-1 prefix and both raw artifacts are recovered.
   No inference was run from Codex. D remains **OPEN** and activation remains
   **BLOCKED**.

### 2026-09-05 — resume enumeration abort preserved and corrected

1. `run-20260905T081329300Z` stopped during resume discovery before runtime
   preflight or inference. Its zero-case receipt SHA-256 is
   `d5f4c307dfd1f45c12f03040100f16fa9ad084fa7048313147fe78fc7eb86e62`.
   The already-sealed case 1 remains the only consumed case.
2. The cause was unnecessary recursive enumeration descending into case
   evidence directories. Resume discovery now enumerates only immediate run
   directories and their aggregate receipt files; evidence files are opened
   later by exact recorded path and hash. `RESUME_*` failures also map to the
   integrity exit code instead of the generic runner exit.
3. Updated runner SHA-256 is
   `55ee473e6985a030dbe2f23958ea48a17ef36804416e1329ac7d0f8fa4b3fc9f`;
   config SHA-256 is
   `7fe1e0bf20dcf46055e55c2e04e6a2b8e87a7c2a8ebaf34b70523d7c0e5972f0`.
   Framework validation passes 137 assertions and again recovers the exact
   case-1 request and response evidence. Cases 2 through 16 remain authorized.

### 2026-09-05 — Q4 final qualification attempt 1 invalidated and holdout retired

1. The one-shot run `qualification-20260905T152811856Z` is preserved as
   **INVALID_INCOMPLETE_CONSUMED**. Its manifest SHA-256 is
   `5dc11809273d762653fb38eb6084e573031dcfb69d786065551dc4661acd8b2e`.
   Two cases completed and a third request was captured without any response
   or checkpoint result. The exact evidence therefore proves two completed
   model calls and three dispatched requests; it does not support inventing a
   result for DQ2-003.
2. DQ2-001 and DQ2-002 both returned HTTP 200 strict-UTF-8 JSON envelopes, but
   both completions ended with `finish_reason=length` at exactly 192 completion
   tokens. Their inner planner JSON was cut mid-string inside the nested
   parameters object. This is a production-interface completion-budget and
   prompt/schema compactness defect, not HTTP corruption or an envelope/parser
   defect. The earlier passing development path used a six-field schema and a
   50-token stopped response, while qualification exercised the twelve-field
   production schema and nested parameters.
3. The immutable incident receipt is
   `reports/nxb21/d-q4-final-qualification-attempt-1-invalid-consumed-v1.json`
   at SHA-256
   `94f8554098bd8cfb012bd7726644c7d16bae8d08ef6b30d53d52ce86dbcdbd3c`.
   The holdout SHA-256
   `aaa8517983ea4523e235c4b3de31e5562bfaeada21ef24a7b23065a567d31d13`
   is permanently registered as non-reusable, including copied or renamed
   bytes. No remaining case may run.
4. Read-only post-state verification passed: HTTP health is good, active jobs
   are zero, retained tuple is `64|64|77|22507|828|61`, activity count is 314,
   all five container IDs/images/restart counts match the qualification
   baseline, and the Q4 model is unloaded. No deployment, database, retained
   evidence, activity, model-download, container, or volume mutation occurred.
5. Source remediation raises the production dynamic-planner completion budget
   from 192 to 512 and instructs the model to emit compact values for unused
   fields and leave clarification empty unless required. Focused Go tests pass
   using an HTTP stub, with no model inference. The qualification framework
   passes 28 assertions, including same-hash copy rejection and explicit
   activation rejection of the invalid incident object. A direct runner guard
   proof returned exit 11 without creating a run directory.
6. Candidate `nxb21d-q4-final-qualification-r1` is retired. Its suitability is
   **NOT_DETERMINED**, D remains **OPEN**, E is not accepted, and activation is
   **BLOCKED**. The next permissible work is the fresh non-holdout multilingual
   development gate in
   `reports/nxb21/d-q4-interface-remediation-development-plan-v1.md`. A new
   candidate identity and a new independent holdout may be frozen only after
   that development gate passes.
7. Program state: `DEVELOPMENT=COMPLETE_FOR_PREVIOUS_CANDIDATE`,
   `Q4_FINAL_QUALIFICATION_ATTEMPT_1=INVALID_INCOMPLETE_CONSUMED`,
   `CASES_COMPLETED=2/168`, `COMPLETED_MODEL_CALLS=2`,
   `REQUESTS_DISPATCHED=3`,
   `FAILURE=MALFORMED_PROPOSAL_DQ2_001_DQ2_002`,
   `Q4_SUITABILITY=UNCLASSIFIED`, `D_STATUS=OPEN`,
   `E_STATUS=NOT_ACCEPTED`, `ACTIVATION=BLOCKED`, and
   `REPLACEMENT_HOLDOUT=NOT_YET_CREATED`.

### 2026-09-05 — production-interface remediation development gate frozen

1. Production completion behavior remains fixed at 512 tokens. The model-facing
   contract now directs compact defaults for required unused fields, omission
   of optional nested parameter fields, and an empty clarification unless the
   request cannot be resolved. The full twelve-field production schema remains
   unchanged. Source SHA-256 is
   `f869bcb38f255b84de000e32fa01da152c36fd54ee8cf84f3966fa1e1d5287d5`.
2. A fresh development-only corpus contains 24 cases, exactly six each in
   English, Urdu script, Roman Urdu and mixed language. It has 20 exact
   production-interface model cases and four deterministic critical/state
   cases. Corpus SHA-256 is
   `c9b71b7e27669b257a145ad5dd94ab1dd5576e718600f1c90ffefbb5cbca281e`.
   Validation proves unique IDs/questions/oracle values, valid capability,
   semantic and scope contracts, UTF-8 without BOM, different content hash,
   and no exact question/value overlap across six prior corpora or 87 captured
   development probes. Both consumed holdout hashes are explicitly excluded.
3. Development freeze `nxb21d-q4-production-interface-remediation-dev-r1`
   has identity SHA-256
   `032234bf2717965e6afe20135c61e8ff8907f2e3393771c7b7280c1c395cdfcb`.
   It binds the Q4 artifact, Q4/Q8 profiles, production query-language source,
   exact/phrase/scope authorities, contracts, capability references, byte-safe
   transport, corpus, evaluator, runner and completion budget. It is not a
   final replacement qualification candidate.
4. The standalone runner exercises `resolveWithLanguageAssistance` through an
   exact production request proxy. It captures request bytes, raw and strict
   UTF-8 responses, inner proposal, per-case result, model-versus-final audit,
   finish reason, token use/utilization, byte sizes, latency and continuous RAM
   samples. It enforces 6 GiB unloaded/4 GiB loaded pre-inference floors,
   verifies all runtime and retained baselines, unloads Q4, and writes a sealed
   aggregate receipt. No live model call was made while preparing the bundle.
5. Source validation passes: focused `TestNXB21D|TestAPF35` in 8.595 seconds;
   dynamic planner authority regressions in 1.522 seconds; the final fresh
   24-case deterministic capability/state oracle in 0.549 package seconds;
   interface framework
   68 assertions; qualification/activation guard framework 28 assertions; and
   the full `api/forensic_records` package in 44.511 seconds. Corpus validation
   and PowerShell syntax also pass.
6. Program state is
   `PRODUCTION_INTERFACE_REMEDIATION=SOURCE_VALIDATED`,
   `DEVELOPMENT_DIAGNOSTIC_24=FROZEN_NOT_EXECUTED`,
   `FINAL_REPLACEMENT_QUALIFICATION_CANDIDATE=NOT_YET_FROZEN`,
   `REPLACEMENT_INDEPENDENT_HOLDOUT=NOT_CREATED`,
   `Q4_SUITABILITY=UNCLASSIFIED`, `D_STATUS=OPEN`, and
   `ACTIVATION=BLOCKED`.
## 2026-09-08 NX-B2.1D final controlled local Qwen3-8B attempt prepared

The last authorized local Qwen3-8B development gate is frozen but unexecuted as
`nxb21d-qwen3-8b-final-local-selection12-20260908T100500Z`. It preserves the
exact 5,027,783,488-byte artifact
`d98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785`,
the exact low-memory profile
`672f8e2c29f162cdc74a8a8dfe7129955436c771db454b0a3ebb20a3bf9b560a`,
and `forensics.hybrid-decision/v2`. The completely new 12-case corpus is
`1fdd47b3112e910c9a9850a0cdca16db2240a12e8a9209676b0e60035d4f2d6d`;
freshness passed across 2,035 files with zero question, target-value, or
normalized-literal overlap, and production source admission passed 12/12.

The operator runner uses the sealed prebuilt evaluator
`130ffac7fcfeb01c57b713cf171dc27d4b9f54a0fde5a7b69796a1dc9875588d`.
It creates no consumption marker until immediately before the first actual
model request, requires three consecutive >=6 GiB preload samples at five-second
intervals, preserves the >=4 GiB pre-inference loaded-state floor, records
initial and settled loaded RAM separately, and preserves backend/container/host
memory observations. The prebuilt no-inference Ginkgo admission path measured
0.621 seconds; compilation is absent from the live gate. The prior consumed
failure remains a resource-admission failure with insufficient model-quality
evidence and immutable receipt SHA
`346a519e8e83ee18ade6ffb9a3a740ed7203f5fe07d5b0c102fcc6d32330f107`.

Current state: `LOCAL_QWEN3_8B_FINAL_ATTEMPT=PREPARED_NOT_EXECUTED`,
`QWEN3_8B_MODEL_QUALITY=INSUFFICIENT_EVIDENCE`,
`FINAL_HOLDOUT=NOT_CREATED`, `FINAL_QUALIFICATION=NOT_STARTED`,
`NX-B2.1D=OPEN`, and `D_ACTIVATION=BLOCKED`. Execute only after closing
Codex/ChatGPT, Chrome, VS Code/IDE, and other nonessential applications while
keeping Docker Desktop running:

`powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\nxb21d-qwen3-8b-lowmem-selection\final-local-selection12\run_final_local_selection12_development.ps1`

### 2026-09-17 — frontend source-truth correction passed; UI activation stopped at RAM gate

1. **Scope boundary:** this turn is frontend UI/UX plus the browser/API plumbing
   required by those screens only. The temporary grouped-query and semantic-
   planner edits made during the turn were removed. No query, semantic,
   operation, adapter, model-role, database, retained-data or reprocessing
   change remains.
2. **Add Data:** the analyst flow is drop/choose plus a truthful queue. Server
   capabilities describe accepted formats; browser validation is advisory;
   time and language remain unknown unless authoritative metadata resolves
   them; cancellation reaches XHR and failed/cancelled items can be retried.
3. **Evidence and modality:** the evidence top strip is visible, sticky and
   responsive across 390/820/1024/1440/1600 in light and dark themes, with
   real filters/chips/reset and sequenced requests. One authoritative-field-
   first modality resolver now supplies structured/document/text/image/audio/
   video/archive/unknown icons and labels, including the audio waveform.
4. **History:** opening a retained result remains distinct from explicit
   `Use again`/`Use question again`; reuse pre-fills the exact question and does
   not execute it. Search remains explicitly limited to loaded paginated rows.
5. **Validation:** 49 focused unit tests, 31 Analyst Portal Playwright tests,
   20 Case Workspace Playwright tests and the 689-module production build pass.
   Focused ESLint reports zero errors and zero new warnings (55 inherited
   warnings remain in already-modified large frontend files).
6. **Fresh activation candidate:** source digest
   `c82530503b60b7190a2ac6627c83faacccca822598eaa1ca4cf46d4c7eafd4df`,
   dist digest
   `c848483ea2907d69bff75af48b2d03765eadf9bbe0f9878cf2160132b28ca085`,
   and manifest SHA-256
   `0ae4dae2af4f45e2a05cf7f49eb2febeff37533987a2c02655e85a9656f240a6`
   are sealed under
   `reports/nxb21/nxb21d-source-truth-ui-activation-20260917/`. The bundle is
   pinned to the exact live backend base image
   `sha256:327754b7fe958c9d3f33f4ad2188f3c29cac7d9576f775b8eaca4c60b3b10006`
   and includes no backend source.
7. **Activation outcome:** read-only preflight passed at retained tuple
   `69|69|82|0|22512|876|69`, Activity 355 and zero active jobs. The authorized
   live attempt stopped before first mutation after twelve 4.57–4.836 GiB RAM
   samples failed the unchanged 6 GiB gate. No service was recreated, no image
   was built/tagged, no model was unloaded, and retained data was not mutated.
   Current status is `FRONTEND_SOURCE_VERIFIED`,
   `UI_ACTIVATION=NOT_STARTED_RAM_GATE`, `OpenFrontendSourceP0/P1=0/0`.
8. **Next:** free physical RAM until three consecutive samples are at least
   6 GiB, then run
   `scripts/activate_nexusai_source_truth_ui_20260917.ps1` without model-unload
   or cache-reclaim switches. Full evidence and limitations are recorded in
   `reports/nxb21/nxb21d-frontend-source-truth-reconciliation-20260917.md`.
