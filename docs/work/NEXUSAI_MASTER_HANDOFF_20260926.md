# NexusAI — MASTER HANDOFF, 2026-09-26

**Paste this whole file as the first message of a new session.** It is the
authoritative state: everything below was measured, not assumed, and every
number can be reproduced with the commands in §12.

Repository: `C:\Users\sheik\Workspace\Office\Projects\NexusAI`
Branch: `codex/forensic-hybrid-checkpoint-20260723`
Two agents share this worktree: **you own `api/**`**, Codex owns
`semantic_layer/**` and the UI. Neither edits the other's.

---

# 1. THE GOAL, IN ONE SENTENCE

**An analyst asks a question in their own words about any evidence in the case —
structured records or media — and the system either answers it from a plan it can
verify against the real database, or says why it cannot.**

No template written in advance for that question. No predefined operation list.
The answer is computed by translating natural language into a typed, verified
plan over the ACTUAL schema, executed as parameterized SQL.

Secondary, required, and currently ABSENT: greetings, chit-chat, "what can you
do", product help, concept explanation and out-of-scope handling — the
conversational roles an LLM should serve. A real user hits these in the first
minute. See §9.

**What must be demonstrable to a team lead:** every evidence family answering
real questions nobody wrote in advance, with citations, with abstention instead
of guessing, and with media observations never presented as source records.

---

# 2. BINDING CONSTRAINTS — these are absolute

Carried verbatim in intent from the product owner's MASTER_EXECUTION_PROMPT.
Violating any of these is worse than making no progress.

## Never run without being explicitly asked in the session

`git reset --hard` · `git clean` · `git checkout --` · `git restore` ·
`git stash` · `git merge` · `git rebase` · `git cherry-pick` · `git pull` ·
`git push` · any force operation. **Never delete an untracked file you did not
create** — `semantic_layer/**` is entirely untracked and is Codex's work.

## Docker

Never `docker compose down`, `down -v`, `--remove-orphans` (it would delete
`nexusai-api-1`), `system prune`, `volume prune`. **Never reprocess, backfill,
migrate or delete retained evidence.** An orphan-container warning on every
compose command is expected; ignore it.

## Ask the product owner first, always

Container rebuild or redeploy · any `docker compose` action beyond `ps`/`logs` ·
any database write, migration or backfill · downloading a model or installing a
backend · any git operation that writes · long builds · running the live
evaluation suites · anything touching retained evidence.

## Absolute product rules

- **ANTI-TEMPLATE:** do not add operation templates. The catalog is FROZEN. Do
  not extend `query.go`'s keyword ladder.
- **ANTI-MODEL-ROULETTE:** no evaluating, benchmarking, downloading or swapping
  LLMs except under a decision rule written down BEFORE the run and surfaced.
- **ANTI-EMBEDDING:** embeddings are for semantic TEXT retrieval only. They must
  never drive a structural decision — family, grouping, measure, field role.
- **ANTI-REPORT:** write a report only when it carries a measurement.
- **NEVER FABRICATE** an analytical result, a citation, OCR text, a transcript,
  an ANPR sighting, a face identity, a location, a timeline, or a cross-family
  relationship.
- **Abstention is a success. A confident wrong answer never is.**
- CNIC and PII masking is enforced **server-side at projection**, never in the UI.
- Never merge upstream LocalAI into this repo. Clone to a SIBLING directory.
- `NEXUSAI_CONTINUATION.md` is the current-state file: **rewrite it, never
  append, hard cap 300 lines.** It is at 299.

## Auth fails closed

`forensic-records-api` requires `FORENSIC_RECORDS_API_KEY` and
`FORENSIC_API_AUTH_REQUIRED=true`. **Never "fix" a crash-loop by disabling auth.**
On 2026-09-17 a whole session served the forensic API unauthenticated on real
evidence.

---

# 3. WHERE WE STAND — measured 2026-09-26

## The three suites

    golden 62 (regression only)   45 CORRECT · 2 NOT_STATED · 14 CLARIFIED · 1 MANUAL
    structured held-out (13)      10 CORRECT · 2 CLARIFIED · 1 WRONG
    media held-out (28)            6 CORRECT · 6 WRONG · 14 CLARIFIED · 1 ROWCOUNT · 1 ERROR

**The golden 62 measure REGRESSION, not capability.** They were written around
the system. The held-out sets measure capability: written against the DATA, in
phrasings nobody wrote for the system. Every capability claim must cite held-out
evidence.

**The 47 you may see in older documents was never real.** The harness graded
`contains` against the whole response including raw rows, so a value the analyst
was never shown still scored. Eight questions passed that way. The honest
baseline was 39, and Stage 0 took it to 45.

## Configuration, currently persisted in `.env.forensic-runtime.local`

    FORENSIC_VERIFIED_ONLY=true          the S6/S9 gate; abstain over guess
    FORENSIC_IR_ARBITRATION=true         re-earn a refused plan
    FORENSIC_IR_FALLBACK=true
    FORENSIC_PLAN_CACHE=true             + DIR=/data/forensic/spool/plan-cache
    FORENSIC_ANSWER_STATES_VALUES=true   the answer states the value it computed
    FORENSIC_MEASURE_DENOMINATOR=true    an average states what it averaged over

    OFF: FORENSIC_LADDER_ROUTING=true is the COMPOSE default (gates EXISTING
    behaviour, so default-on reproduces today). BREAKDOWN_GOAL, MEDIA_FAMILY_
    ROUTING, DERIVED_ARTIFACT_EXECUTION, IR_SHADOW, IR_CROSSCHECK,
    DROP_INVENTED_FILTERS all default false.

**ASSERT SWITCHES FROM THE CONTAINER BEFORE SCORING ANY RUN.** An env file that
OMITS a switch is indistinguishable from one setting it false, and nothing logs
the gate state at startup. Five settings ran wrong for five hours before anyone
noticed, and a golden suite was scored in that state.

    docker inspect nexusai-forensic-records-api-1 --format '{{range .Config.Env}}{{println .}}{{end}}'

## The data

Postgres on host port **5433**, db `localrecall`, user `localrecall`/`localrecall`
(the `forensic_runtime` role is subject to RLS and returns nothing for audits).

    forensic.records            STRUCTURED, 7 families
    forensic.derived_artifacts  MODEL OBSERVATIONS from media
    forensic.evidence_items     the files

Collections:

    nexusai-forensic-demo                  12,912 structured rows, no media
    nexusai-multimodal-product-acceptance   9,580 structured + 865 derived
                                            artifacts across 12 contracts
                                            (22 images, 7 audio, 2 video, 1 pdf,
                                             1 document)

Curated layer: **17 entities** — 7 structured + 10 derived — 62 media fields and
8 metrics, 183 field/metric ids overall. Demo-collection column coverage
**93/94 (99%)**.

---

# 4. THE ARCHITECTURE — decided, do not relitigate

1. **Stop defining operations.** 79 templates, 104 operations and the keyword
   ladder are a PER-QUESTION abstraction: they can never cover a phrasing nobody
   wrote in advance, and every miss is a confident wrong answer. Frozen; deleted
   at the end of Phase 3.
2. **The Governed Semantic Compiler** is the answer path:
   classify → literals → scope → narrow → **enum-constrained IR** → validation →
   one self-correction → parameterized SQL → verify → **abstain, never guess**.
3. **The keystone, verified on the SERVING path:** a JSON-Schema `enum` becomes a
   GBNF alternation, so the model CANNOT emit a field or value that is not
   issued. It travels as JSON and the server converts it
   (`core/http/endpoints/openai/chat.go`). `grammar_keystone_test.go` asserts it
   through the JSON round-trip — **calling the converter directly gives a
   misleading answer**, which once produced a confident, entirely wrong
   conclusion that the keystone was broken.
4. **The curated semantic layer is the vocabulary.** A column the layer does not
   describe is never issued, so no verified plan can reference it. Coverage of
   the layer IS coverage of the product.
5. **LocalAI becomes a pinned, unmodified upstream image.** Fork baseline v4.5.6;
   divergence ~330 lines across 8 files.
6. **NexusAI owns analyst identity and cases.** No case entity exists (`case_id`
   3× vs `collection_id` 224×); every analyst is one shared principal. **This
   blocks real deployment.**
7. **UI contract:** `docs/ux/NEXUSAI_PRODUCT_UX.md`. IA is CASE → EVIDENCE →
   QUESTION; families are filters; standalone app.

## Deterministic SQL, narration-only LLM

The model's ONLY job is to emit an enum-constrained plan and to narrate a result
it did not compute. Every number comes from SQL. Live models: synthesis
`qwen3-4b-instruct-2507-q4km-nxb21d-dev` (Q4_K_M), embeddings
`qwen3-embedding-0.6b`. CPU only, ~4 tok/s, 15.7 GiB RAM.

---

# 5. WHAT IS BUILT — and what each piece cost to learn

## The structured answer path — WORKS

Four fixes took the structured held-out set from 7 WRONG to 1:

- **H2** the executor applied ANY extracted target as a SUBJECT predicate, so a
  verified plan over 739 rows returned 0 — a FALSE NEGATIVE, the worst answer
  available, because the analyst stops looking.
- **H1** "different" implies distinct ONLY without a superlative ("the MOST
  different people" is a ranking).
- **H7** multi-word declared synonyms never bound, because that one matcher
  compared raw words while everything else stemmed.
- **H4** "What is the largest network volume?" was classified
  GENERAL_DOMAIN_KNOWLEDGE and never reached the compiler.

Plus two S9 obligations (MEASURE, RESTRICTION), each shipped with the control
measurement that bounds it.

## Stage 0 — the stated answer contains the answer

`FORENSIC_ANSWER_STATES_VALUES`. Ranges state both ends; a complete breakdown of
≤12 groups states every group. `"1 CDR record matched this question"` became
`"The CDR records span event time from 2026-04-01 to 2026-08-10."`

**A trap worth knowing:** `shortDate` truncates ANY string of ten characters or
more, so rendering a numeric range through it would silently chop `1730127963000`
to ten digits and state a wrong quantity confidently. It is now applied only to
values that parse as timestamps.

## Stage 1 — media becomes algebra

`derived_artifact_source.go` + executor binding. An entity declares where its rows
live:

```yaml
source:
  table: derived_artifacts            # omitted means records
  artifact_type: forensics.anpr-observation/v1
  payload: metadata
  observation_type: video_sampled_frame_observation   # optional discriminator
```

Proven against the real database: `COUNT` over video plate groups = **24**;
nested `AVG` over `observation.crop_quality` = **0.266055**; the discriminator
partitions `image-observation` into **20 + 6 = 26** exactly.

Refusals built in: a plan mixing an ingested record with a model observation is
**REFUSED, never joined**. Derived PROJECTION is refused — `derived_artifacts` has
no `record_id`/`row_hash`, and citing rows without provenance hands the analyst
evidence they cannot attribute. Only `processing_status='completed'` artifacts
count.

**THE 9,580 DEFECT, found by a database test and not by reading code.** A plain
`COUNT(*)` names no field, so the binding had nothing to resolve, fell back to
records, and answered **9,580** — every structured row in the collection — where
the truth is **24**. Binding precedence is now explicit: the plan's fields, then
the issued catalogue (it BOUNDED the plan), then the question's family as a
TIEBREAKER ONLY, then records — never an error, because erroring was an HTTP 500.

## `FORENSIC_MEASURE_DENOMINATOR` — an average says what it averaged over

`image-ocr-observation` carries THREE confidence fields from two disjoint
producers: 263 of 309 rows, and 46 each. Live on real data:

> *"The maximum detection evidence strength across ANPR sightings is 0.92.
> Computed over the 307 of 1,057 ANPR sightings that carry a value; the remaining
> 750 do not."*

750 of 1,057 records carry none. Before this, that read as a claim about all
1,057. Threshold 95%, deliberately not 100%: a qualifier on every answer is noise
analysts learn to skip, which devalues it where it matters.

## `FORENSIC_MEDIA_FAMILY_ROUTING` — media questions can name a media family

Family resolution went through `extractCanonicalRecordType`, which knows only
STRUCTURED record types, so a media family was unreachable. `"How many plate
groups used persistent object tracking?"` resolved `anpr_vehicles` — because
"plate" names the ANPR RECORD TYPE — and answered **"There are 1,057 ANPR
sightings in this case"** to a question about 24 model plate groups.

`media_family_routing.go` asks the CURATED LAYER instead, reusing the phrase
matcher that already binds curated values, so there is one vocabulary rather than
two that can disagree. **Coverage is a curation number, not a code number:**
adding a synonym moves a question from unanswerable to answered with no code
change.

Three safety properties, each load-bearing: MULTI-WORD phrases only (structured
families own the single words "plate", "sighting", "camera", and single words are
dropped in CODE because the layer is curated by another track); EVERY word must
appear; AMBIGUITY CLARIFIES rather than guessing.

**Stemming was the difference between 4% and useful** — exact-token matching
reached 1 of 28 because "faces were detected" cannot match "face detection".
Confined to multi-word phrases, because stemming a single word once made "calls"
bind `CALL` and would have supplied a filter to five correct questions.

**ALGEBRA ONLY.** A media entity claims a question only when the goal is
`aggregate`/`breakdown`/`rank`/`distinct`/`range`. `lookup` is excluded because it
is ALSO the unrecognised default. This resolved four collisions with working
retrieval templates (`image_ocr_search`, `face_candidate_observations`) without
touching Codex's synonyms — those phrases are right for both surfaces; the real
distinction is whether the question computes or retrieves.

## Just landed, UNMEASURED

The media-artifact guard now **stands down for derived families**. It existed
because a STRUCTURED template once answered "what does the HP guide describe?"
from `access_failed_events`. It was withholding FIVE correctly-routed media
questions with *"you asked about images, but this question matched a different
kind of evidence"* — the evidence was exactly right and the guard could not tell.
Suite green both ways; **not yet measured live.**

---

# 6. THE CENTRAL FINDING — read this before planning anything

**Four correct, measured, well-tested slices delivered 1 of 28 media answers,
because the binding constraint was never where the work was.**

Every media question that routed correctly was then withheld by ONE guard. The
contract, the executor, the discriminator and the denominator were all right and
all unreachable.

**The lesson: trace ONE question end-to-end through every gate before building
anything.** List every place it can be refused — routing, family resolution,
catalogue issue, grammar, S6, S9, verified-only, the media guard, presentation —
and find which one actually stops it. Bottom-up plumbing work feels productive
and measures at zero.

---

# 7. THE IMMEDIATE PLAN — four steps to demonstrable

## STEP 1 — DONE, unmeasured
Media guard stands down for derived families. Suite green both ways.

## STEP 2 — the provenance label. THE LAST SAFETY BLOCKER.

A derived observation is currently cited as `"source": "records_sql"` with
`record_type`, `row_hash`, `row_number`, `source_file` all NULL — the records
columns a derived row does not have. Hardcoded in ~8 places in `query.go`,
independent of the lineage expression Stage 1 added
(`sourceNativeLineageAggExpr` already emits `derived_artifacts_sql` correctly).

**The entire "a model's plate read is not a camera's sighting" property depends on
this citation being right, and `source_truth_state` is not surfaced at all.** Do
not demo media answers until this is fixed.

## STEP 3 — synonym coverage. CODEX, in parallel. Pure YAML, zero code risk.

Routing is 7 of 28. The census test names every unclaimed question. Needed
synonyms: "plate group" without "video", "audio segment", "fingerprint",
"perceptual hash", "script family", "text region", "plate read".

## STEP 4 — ONE measurement, then stop.

Deploy with a control on the SAME container, both switches together
(`MEDIA_FAMILY_ROUTING` + `DERIVED_ARTIFACT_EXECUTION` — routing without
execution only routes a question to be refused, and execution alone is provably
inert). Score all three suites.

**Threshold, written before the run:**

    structured suites  ANY movement to WRONG or NOT_STATED reverts
    media              ZERO confident-wrong. A low CORRECT count is an honest
                       starting point, not a failure.
    P1 must not answer 307 to "how many ANPR sightings" (truth 1,057) --
                       that is a FABRICATED SIGHTING and reverts on its own
    H1-H5 must all CLARIFY -- they cover PII-excluded fields and face identity

---

# 8. WHAT NOT TO DO — the traps, each one already paid for

- **Do NOT delete the ladder to unblock media.** 9,041 lines; measured with
  routing OFF: **3 confident-wrong and 3 correct lost**, because
  `chooseTemplate` also SCOPES family and record type. Step 1 proves media gets
  through by making guards YIELD, not by removing the router.
- **Do NOT narrow the issued FIELD set.** A top-12 cut made CDR-11 unanswerable
  at any model quality. **The enum is not a menu:** changing the issued field set
  changes plans for questions that already work — proven twice, a false negative
  both times. The PLAN CACHE is how you prove it did not: a cache HIT means the
  payload was byte-identical.
- **Do NOT remove generator-invented filters** by any means, post-hoc or by
  prompt. Measured twice from opposite directions: they are a load-bearing
  DIAGNOSTIC that marks a plan the model did not understand. Removing them
  produces confident-wrong.
- **Do NOT change a field's `sensitivity` to work around the PII exclusion.**
- **Do NOT bundle two changes into one measurement.**
- **Do NOT fix `api/**` compile errors from the Codex side, or `semantic_layer/**`
  from the backend side.**

Also rejected after measurement — do not retry: withholding uncurated fields
(cost TWR-02 a false negative, *"there are no tower records in this case"*) ·
withholding measures for a `lookup` goal · "breakdown" → aggregate goal ·
cross-verification of template answers · arbitrating CONFIDENT answers on a shape
mismatch.

---

# 9. AFTER STEP 4 — the remaining roadmap, in order

**A. The conversational and non-database roles — UNWIRED, and the product owner
has asked for them explicitly.** Greetings, chit-chat, "what can you do",
product help, concept explanation, out-of-scope. A real user hits these in the
first minute and gets nothing. D7 ("Explain …" treated as a dictionary
definition) is the same area. Cheap, highly visible, and the right thing to do
immediately after media works. These are the LLM's legitimate role: no SQL, no
evidence, no fabrication — and they must NEVER be allowed to answer a question
about data.

**B. PII masking at projection — the largest capability gap.** `CatalogFields`
drops PII/RESTRICTED outright, so 7 curated fields cannot be referenced by any
plan. Costs: "WHICH plates were read", "what the transcript says", "which plate
was visible longest". Codex's recommendation is ACCEPTED and is what ships: a
case-scoped stable alias (`Plate candidate ••••-A7C2`) rather than last-four,
because plate strings are LOW ENTROPY so last-four both reveals too much and
collides; typed placeholders (`[phone masked]`) in OCR and transcript text with
synchronised redaction across the Urdu and Roman-Urdu forms; **and if token-level
redaction cannot be proven, withhold the whole text while keeping counts,
timestamps, language, review state and locators.**

**C. Numeric-offset subtraction.** "Which plate was visible longest?" needs
`last_seen_seconds - first_seen_seconds`; the expression allowlist supports
timestamp subtraction only.

**D. The two template-path NOT_STATED questions.** CDR-10 and ANPR-03 are
answered by registered templates, so `sourceNativeResultAnswer` never runs;
`tabularResultAnswer` needs the Stage 0 treatment.

**E. Derived-row projection.** Needs a derived provenance mapping
(`artifact_id` + `citation_locator`).

**F. Goal taxonomy, re-measured.** `breakdown` is built and measured NEUTRAL —
but against a control that over-credited four of its target questions, and the
instrument is fixed now. Re-measure WITH the S9 `rows` case, "longest/shortest" →
`rank`, and ANPR-03 "first and last seen" → `range`.

**G. Cross-modal.** 0 of 5. `Joins` is validated and **nothing consumes it at
runtime**. WI-LAYER-2 proved the joins we assumed existed do not:
`cdr.msisdn → subscriber.msisdn` matches **ZERO rows**. Needs duplicate-safe join
execution and a decision on `many_to_many`, which the v1 validator rejects and
CDR↔IPDR requires.

**H. Ladder deletion** — Phase 3's remaining deliverable, `query.go` at 9,061
lines. Media curation was its precondition and is done. Derive scope from the
curated layer, then delete in slices.

**I. Identity and cases** — blocks real deployment (§4.6).

---

# 10. THE CODEX TRACK — protocol and next item

Codex has done excellent, genuinely independent work: demo coverage 81/94 → 93/94,
10 media entities, and it **removed two joins it had been asked to declare**
because they match ZERO rows — a declared join that matches nothing manufactures a
confident empty answer.

## Protocol

- Codex owns `semantic_layer/**` and the UI; it must NEVER touch `api/**`.
- **A rebuild BAKES the YAML into the image.** Verify `TestWI4` passes and record
  a `sha256sum semantic_layer/*.yaml` snapshot IMMEDIATELY before building.
- **Codex's reports about `api/**` compile state have twice been STALE**, because
  it tested inside a window while the backend was mid-refactor. Verify with your
  own `go build` before believing a red report.
- Give it measured evidence, name what it got right specifically, and rule
  explicitly on anything it flagged. It pushes back well when the brief is wrong,
  and that is valuable — preserve it.
- Prompts live in `docs/work/WI-LAYER-*_CODEX_PROMPT.md`.

## Its next item

Synonym coverage (§7 Step 3), plus its own open question: **three audio contracts
carry two `observation_type` values each** (standalone vs video-embedded). The
FIELDS are identical, so splitting is probably wrong, but "how many audio
segments were transcribed?" returns 11 with one from a video and nobody is told.
It has already curated `source_modality` as a groupable field on two of them —
confirm that is the ruling for all three.

---

# 11. MEASUREMENT DISCIPLINE — non-negotiable, all of it paid for

- **Fix the instrument first.** It has applied five times: three runs
  mis-measured; 119s attributed to nothing; a coverage audit that read field IDs
  as column names; a TIMEOUT scored as a regression twice; a five-setting config
  drift nobody could see.
- **TWELVE oracle defects have been found in our OWN suites.** H8 expected 5 when
  6 is correct (`subscriber.status` lives under TWO headers and the executor
  COALESCEs). CDR-04/CASE-04 expected the raw `GPRS` where the layer declares
  "Data session". M14 expected `"en"` — two characters — which matched inside
  *"evidence"*. M21 used `check: number` with `19`, which collided with a camera
  row value. **Derive an expectation from the SAME expression the executor uses,
  and re-derive every expectation when curation lands.**
- **A TIMEOUT (`http: -1`) is never a verdict.** Re-measure the mover in steady
  state, both ways.
- **A probe must reproduce the PATH, not just the function.**
- **Every behavioural change: its own switch, default off, a control on the SAME
  container, and a threshold written BEFORE the run.**
- **Prove the mechanism against the DATABASE, not against a SQL string.** The
  9,580 defect was invisible to code review and obvious to one database test.

---

# 12. HOW TO VERIFY EVERYTHING — copy-paste

```bash
# build + full suite (all switch states must be green)
go build ./api/forensic_records/
go test -C api/forensic_records -count=1 .
FORENSIC_MEDIA_FAMILY_ROUTING=true FORENSIC_DERIVED_ARTIFACT_EXECUTION=true \
  go test -C api/forensic_records -count=1 .

# the layer loads, 17 entities, discriminators distinct
go test -C api/forensic_records -count=1 -run TestWI4 -v .

# media routing coverage census (offline, no model, no deploy)
go test -C api/forensic_records -count=1 -run TestMediaFamilyRoutingCensus -v .

# database-backed proofs of the media path
DERIVED_EXEC_DB="postgresql://localrecall:localrecall@localhost:5433/localrecall" \
  go test -C api/forensic_records -count=1 -run "TestDerived|TestMeasureDenominator" -v .

# curated coverage vs columns actually present
FAMILY_AUDIT_DB="postgresql://localrecall:localrecall@localhost:5433/localrecall" \
  go test -C api/forensic_records -count=1 -run TestSemanticLayerCoverage -v .

# live suites (ASK FIRST). Load env in PowerShell, never Git Bash --
# MSYS rewrites /data/... to C:/Program Files/Git/data/...
python scripts\nexusai_live_eval.py --golden evaluation\golden_questions_v2.json \
  --out reports\<name>\golden --analyst-text --timeout 400
python scripts\nexusai_live_eval.py --golden evaluation\holdout_questions_v1.json ...
python scripts\nexusai_live_eval.py --golden evaluation\holdout_media_v1.json ...
```

Environment quirks that have each cost time: **`docker compose build` is unusable
for LocalAI** (uploads the repo, wedges the daemon); the forensic API builds in
~20s to distroless. **`make` is not on PATH.** **Bash heredocs break on
apostrophes** — use the Write tool. **An empty JSON-Schema `enum` is an
unparseable grammar** and an HTTP 500 before inference.

---

# 13. WHAT TO SHOW THE TEAM LEAD

After Step 4, this is defensible and reproducible:

1. **Structured families answering unseen phrasings** — held-out set, 10 of 13,
   written against the data by someone who did not write the system. Was 5 of 13
   with 7 confident-wrong.
2. **Media families answering real questions** from a typed plan over 865 model
   observations, proven to read `derived_artifacts` and not the records table.
3. **Honest abstention** — every honesty probe clarifies rather than guessing,
   including "who are the people in the images?" over 20 detected faces.
4. **No fabricated sightings** — a model's plate read is never counted as a
   camera sighting; 1,057 ingested rows and 307 model reads stay distinct.
5. **Averages that state their denominator** — "307 of 1,057 records carry a
   value; the remaining 750 do not."
6. **Zero HTTP 500s, zero confident-wrong on the golden 62.**

**State the gaps plainly rather than hiding them:** "which plates were read"
needs PII masking; cross-family is 0 of 5; the conversational classes are
unwired; 9,061 lines of template ladder still await deletion. A demo that names
its limits is worth more than one that gets caught by a question.

---

# 14. START HERE

1. Read `NEXUSAI_CONTINUATION.md` (299 lines, current state), then
   `reports/stage2-media-routing-20260926/MEDIA_ROUTING.md` and
   `reports/answer-not-stated-20260925/ANSWER_NOT_STATED.md`.
2. Run the verification block in §12. Everything should be green.
3. Do **Step 2** (provenance label). It is the last safety blocker.
4. Write the Codex synonym brief (**Step 3**) so it works in parallel.
5. Ask the product owner before deploying, then do **Step 4** — one measurement,
   three suites, control first, threshold written before the run.
6. Then **§9 A** — the conversational roles. That is the next thing the product
   owner is waiting on.
