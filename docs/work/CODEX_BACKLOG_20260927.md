# CODEX BACKLOG — template-free question answering, phase by phase

Self-contained. You do not need another document to start. Written 2026-09-27 after reading
`api/forensic_records/**` and measuring the running system.

**Your scope is `api/**` and `semantic_layer/**`. Claude owns `apps/**` and is working there in
parallel — do not edit it.** The only shared contract is the JSON `POST /query/hybrid` returns:
**you may add fields freely; you may not remove or rename one.**

Work the phases in order. **One phase, one switch, one measurement, one report.** Do not start a
phase until the previous one is measured and reported.

---

## 0. THE SITUATION, AND WHY THIS IS SMALLER THAN IT LOOKS

The product answers 67 of 103 corpus questions correctly. It does this through a **keyword ladder**
of ~250 lines (`choosePreciseTemplate` + `chooseGenericFallbackTemplate`, `query.go`) that maps
phrasings to registered templates. Questions the ladder was not written for either abstain or, in
four measured cases, **answer the wrong question confidently**.

**The replacement engine already exists and is already used** — 39 of 62 golden questions are
answered by the compiler today, not the ladder. What is missing is narrower than "build NL→SQL":

| Capability | State | Where |
|---|---|---|
| Layer declares `GT, GTE, LT, LTE, BETWEEN` and `COUNT, SUM, AVG, MIN, MAX` on numeric fields | **exists** | `semantic_layer/cdr.yaml` and siblings |
| Plan contract carries filters | **exists** | `SemanticFrameV1.Filters` — `semantic_frame.go:57` |
| Frame operator → SQL operator mapping | **exists** | `sourceNativeFilterOpForFrame` — `deterministic_semantic_compiler.go:532` |
| Typed SQL executor | **exists** | `source_native_sql_executor.go`, `source_native_algebra.go` |
| Compiler that sets its own template | **exists** | `applyDeterministicSemanticCompiler` — `deterministic_semantic_compiler.go:967` |
| **NL extraction of a comparative predicate** | **MISSING** | `semanticFrameFilters` (`semantic_frame.go:397`) only copies `facts.Filters`. No `GT`/`LT` literal appears anywhere in `semantic_frame.go`. |
| **CONSTRAINT_APPLIED covering one** | **MISSING** | `deterministicUnboundConstraints` (`:650`) checks only `frame.Identifiers` and `frame.TimeIntent.Kind == "explicit_range"` |

**The stack can already express `duration_seconds > 600`. Nothing extracts it, and nothing notices
it went missing.**

### The two measurements that define the work

**(a) Free-typed questions drop constraints silently.** `reports/adhoc-runtime-20260927/`. Fourteen
questions in ordinary words, none from any suite. Zero fully correct. Four stated a count for a
question nobody asked, each with `"filters": []` in the plan:

    "Show me all the calls that lasted longer than ten minutes"      -> "20 CDR records matched this question."
    "Do any subscribers share the same handset?"                     -> "11 subscriber records matched this question."
    "Is there any link between the plate sightings and the call records?" -> "There are 750 ANPR sightings in this case."

Controls from the same run prove an empty filter list is abnormal: *"How many times was plate
LHR-2026 seen?"* carries `anpr.plate_number EQ LHR-2026` and answers **"87 ANPR sightings involving
LHR-2026"**. **A bound constraint is repeated back in the sentence; a dropped one says "N records
matched this question" and names nothing.**

**(b) Turning the ladder off today costs 3 right answers and creates 3 wrong ones.**
`reports/ladder-deletion-20260925/`, ladder-off vs the 47 CORRECT / 0 WRONG baseline in
`SLICE_RULE.md`. **Nothing else in 62 moved. These six questions are the complete specification of
the compiler's gaps** — Phase 4 is exactly this list.

---

## PHASE 1 — A1a · THE CONSTRAINT GUARD · ~1-2 days · **START HERE**

**This one has a deadline: it must land before the interface is shown outside the team.** A
polished answer card makes a wrong answer more convincing, and Claude is building that card now.

### The ask

`deterministicUnboundConstraints` (`deterministic_semantic_compiler.go:650`) is the S9
CONSTRAINT_APPLIED check. Extend it so a **magnitude or relational condition the analyst wrote** is
an obligation exactly as an identifier already is. When the question carries one and the compiled
plan binds no filter to it, **refuse, and name the condition**:

> *"I did not apply the condition 'longer than ten minutes' to a curated field, so I did not answer."*

**A1a adds NO capability. It must not make a single question answer that does not answer today.**
Parsing the predicate into a filter is Phase 2. If you find yourself making something newly
answerable, stop — that is Phase 2 leaking into Phase 1 and it destroys the attribution.

### Switch

`FORENSIC_CONSTRAINT_OBLIGATIONS`, default **off**, **declared in
`docker-compose.forensic-records.yaml` in the same commit**. Compose forwards only the variables it
names; a switch set in the shell but undeclared reaches nothing and the arm silently measures the
feature OFF. **This near-miss has happened twice on this project** and was caught both times only
by asserting container env before scoring.

### Census before wiring

Follow `goal_taxonomy_slice_test.go`. Run the detector **offline** across all three corpora
(`evaluation/golden_questions_v2.json`, `holdout_questions_v1.json`, `holdout_media_v1.json`),
print every question it captures and every one it disturbs, and commit that list. Slice B of the
goal taxonomy moved 5 and disturbed 0 because it was censused first; WI-31 skipped this and cost a
correct answer.

### Thresholds — write to `reports/constraint-obligations-<date>/THRESHOLD_PREREGISTRATION.md` BEFORE the first arm

- Golden-62 and held-out-13: **any movement at all reverts.** A1a must be inert where constraints
  already bind.
- **Any question becomes WRONG → revert the whole arm**, not the half that looks guilty.
- **PASS**, measured directly: the three questions in §0(a) each ABSTAIN and name the condition.
- Control arm first, same container, **switch asserted from `docker inspect` before scoring.**

---

## PHASE 2 — A1b · PARSE THE PREDICATE · ~2 days

Extract the condition into `SemanticFrameFilterV1{FieldHint, Operator, Literal}` with `Operator` in
`GT|GTE|LT|LTE|BETWEEN|EQ|NEQ`.

- Resolve the field through `sourceNativeFieldByExactName` (`:561`) — **exact name and declared
  alias only, no embedding signal**, for the reason written in that function. A filter bound to a
  field the analyst did not name answers a different question confidently.
- Honour `allowed_filters` from the layer. If the field does not declare the operator, **abstain.
  Do not widen the layer.**
- **Units are a correctness problem, not a parsing problem.** "ten minutes" against
  `duration_seconds` is `600`; "more than 1 GB" against a bytes field is `1073741824`. **If the
  curated field declares no unit, abstain.** An invented conversion is a fabricated number.

**Threshold:** structured corpora unchanged; *"calls that lasted longer than ten minutes"* returns
rows whose duration exceeds 600 seconds, **verified by independent SQL — never by the API's own
count.**

---

## PHASE 3 — A2 · A COMPILER PATH FOR A TEMPLATE-LESS QUESTION · ~1-2 days · **the real blocker**

The code already says this. `query.go:2102`:

> *"The real blocker is therefore NOT the template value: it is that no compiler path exists for a
> template-less question. The handler guard at the `template == ""` check ends the request before
> planning."*

That guard is **`query.go:607`**. An empty template short-circuits to a clarification **before any
planning runs**.

**Switch:** `FORENSIC_COMPILER_FIRST`, default **off**.

**Do not simply delete the guard.** The order must be: compile → validate (S6, S9 SHAPE,
CONSTRAINT_APPLIED) → execute if it passed → **fall through to exactly today's clarification if it
did not**. The clarification text, reason code and options must be **byte-identical** when the
compiler abstains, or the arm is not attributable.

**Threshold:** golden-62 and held-out-13 unchanged; **zero new WRONG on any corpus**; media suite
no worse than its shipped 13 CORRECT / 3 WRONG.

**Warning from the record:** a change that looked equally well-evidenced — returning
`canonical_records` from `chooseTemplate` for media questions — sent five correctly-routed
questions to the wrong table and turned five honest abstentions into five confident-wrong answers.
The full post-mortem is in the comment at `query.go:2087`. Read it before you start.

---

## PHASE 4 — A3 · THE SIX MEASURED GAPS · ~8-12 days

One work item each, **its own switch, its own run, its own report**. Do not bundle: a bundled red
result is un-attributable, and this project has already paid for that.

    A3.1  CDR-16   "How many CDR records came from each source file?"
                   now: "I can count this, but I did not compute a count here"
                   GAP: GROUP BY a provenance column with COUNT. It listed rows instead.
    A3.2  CDR-12   "Which cell site handled the most calls?"
                   now: "Which target identifier should I analyze?"
                   GAP: rank/superlative over ALL rows, with no target entity to anchor on.
    A3.3  TWR-02   "Where is tower PK-LHR-SYN-001 located?"
                   now: "There is 1 tower record involving PK-LHR-SYN-001"
                   GAP: attribute PROJECTION. "Where is X" needs field values, not a count.
    A3.4  CASE-01  "How many records do we have for each record type?"
                   now: "8,642 CDR records across 5 call type values."
                   GAP: grouping ACROSS families. Bound one family, grouped the wrong column.
    A3.6  X-01     "Where does 03001234567 appear across all evidence?"
                   now: "No matching records were found"
                   GAP: cross-family presence search. A FALSE NEGATIVE — the worst shape here.
    A3.5  DOC-02   "Which document mentions contact number 03001234567?"
                   now: "I could not map that request to a deterministic forensic workflow."
                   GAP: passage retrieval has no compiler path. Largest item — SPIKE FIRST and
                   report an estimate before committing to it.

**A3.3, A3.4 and A3.6 carry a higher bar.** They are currently confident-wrong or a false negative
with the ladder off, so **"it now abstains" is a PASS**, and "it now answers" requires the value to
be verified by independent SQL mirroring the executor's own expression.

---

## PHASE 5 — A4 · DELETE THE LADDER · ~1-2 days

`reports/ladder-deletion-20260925/SLICE_RULE.md` already defines slices 2-4 and they stand
unchanged. **Do not start while any question in Phase 4 still depends on the ladder.** The switch
stays available after the cut; the rule says why.

---

## PHASE 6 — B · THE LLM'S OTHER ROLES · ~2-4 days

The classifier **already has the classes** (`pkg/forensicrequest/`): `PRODUCT_HELP`,
`GENERAL_DOMAIN_KNOWLEDGE`, `CLARIFY`, `UNSUPPORTED`, `CONTEXTUAL_FOLLOW_UP`. Missing is routing and
content. Measured: `hello` classifies as **GOVERNED_EVIDENCE_ANALYSIS** and returns *"I could not
map that request to a deterministic forensic workflow."*

    B1  Route greetings and small talk to a terminal class. Rule on whether PRODUCT_HELP fits
        rather than inventing a class, and say which you chose and why.
    B2  PRODUCT_HELP content: what the system answers, what it refuses, and HOW TO ASK.
        HIGHEST VALUE ITEM IN THIS PHASE -- it turns a dead end into a teaching moment at the
        exact moment the analyst is stuck.
    B3  GENERAL_DOMAIN_KNOWLEDGE: a bounded glossary (CDR, IPDR, ANPR, IMEI, IMSI). Every entry
        states it is a general definition and makes NO claim about the case. A term not in the
        glossary ABSTAINS. The model never free-generates a definition.

**Binding rule:** a conversational answer must be **structurally distinct** from an evidence answer
— no citations, no result grid, and it says plainly that it makes no statement about the case. The
moment greeting text and evidence text look alike, every guarantee this product makes becomes
unreadable.

---

## WORKING DISCIPLINE — every phase

1. One behavioural change, one switch, default off, **declared in compose in the same commit**.
2. Threshold written **before** the run, in `reports/<item>-<date>/THRESHOLD_PREREGISTRATION.md`.
   **Not editable once the first arm starts.**
3. Control arm first, same container, **switch state asserted from the container** before scoring.
4. **Census offline before wiring** anything that matches on language.
5. Verify an expectation with **SQL that mirrors the executor's own expression** — never the API's
   own output. **Fourteen oracle defects** are on record in this project's own instruments; the
   most recent, on 2026-09-27, made a working system look broken. **Any new instrument must run a
   known-good control first and refuse to report unless it passes.**
6. Only write a report that carries a measurement.
7. Revert **the whole arm**, never the half that looks guilty.

### Stop and report — do not proceed — if

- `api/**` does not compile. Say so **with the wall-clock time** and **stop. Never fix it.**
- A threshold fires.
- You would need to change a `sensitivity` value or widen `allowed_filters`.
- You would need to touch `analyticalIntent` in `pkg/forensicrequest/classification.go`.
  Re-enabling `earliest|latest` there caused a **real PII disclosure** on 2026-09-26;
  `TestClassifierSuperlativeBoundary` asserts they stay absent. It is the product owner's call.
- **H2 must stay blocked.** It is the PII probe, and the goal gate keeps transcript text off the
  retrieval path until free-text masking exists.
- A detector cannot tell a condition from ordinary phrasing without guessing. **A refusal to guess
  is a finding, not a failure** — three prior reports were worth more than the work they cancelled.

### Never

`git reset --hard` · `clean` · `checkout --` · `restore` · `stash` · `merge` · `rebase` ·
`cherry-pick` · `pull` · `push` · any force operation. `docker compose down` · `down -v` ·
`--remove-orphans` · `system prune` · `volume prune`. Reprocess, backfill, migrate or delete
retained evidence. Disable auth to fix a crash-loop. Bypass hooks with `--no-verify`. Lower a
coverage baseline. Merge upstream LocalAI into this repo. Edit `apps/**`. Delete an untracked file
you did not create.

### Ask the product owner first

Container rebuild or redeploy · any `docker compose` action beyond `ps`/`logs` · any database
write, migration or backfill · model downloads or backend installs · any git operation that writes
· long builds · **running a live evaluation suite** · anything touching retained evidence.

### Environment notes that cost hours when forgotten

Set env from **PowerShell, never Git Bash** (MSYS rewrites `/data/...`). Postgres is host port
**5433**, db `localrecall`. **Bash heredocs corrupt content** — they wrote a literal `0x08` into a
regex once and two NUL bytes into a source file on 2026-09-27; write a file instead. An empty
JSON-Schema `enum` is an unparseable grammar and 500s before inference.

### Verify

```bash
go test -C api/forensic_records -count=1 ./...
```

### The rule that outranks all of the above

**Abstention is a success. A confident wrong answer never is.** Never fabricate a result, citation,
OCR text, transcript, sighting, face identity, location, timeline or cross-family relationship.

---

## DONE MEANS

The ladder is off by default, the corpus holds at **≥47 CORRECT / 0 WRONG**, and a fresh probe of
≥20 never-before-seen questions produces **zero stated answers with an unbound constraint**. Re-run
`reports/adhoc-runtime-20260927/adhoc_probe.py` with new questions; its selftest is already wired.

**The held-out set must still reach ≥40 questions.** It is the primary gate. A capability measured
only on questions we wrote for ourselves is a capability we have not measured.
