# WI-NL2SQL-UX — template-free question answering, the LLM's other roles, and the analyst interface

Written 2026-09-27 after reading the code and measuring the live system, not from the roadmap.
Every claim below carries the file, line or run it came from. **Correct me where I am wrong before
you build — two of the three most expensive mistakes on this project came from acting on a
plausible inference nobody checked.**

This brief has three tracks. **Track A is the product.** Track B and Track C do not start until
A1 and A2 are measured and shipped, because an interface over a system that answers the wrong
question is worse than no interface.

---

## 0. THE FINDING THAT CHANGES THE ESTIMATE

**The NL→SQL engine is already built.** It is not a rewrite. The gap is narrower and much more
specific than "we rely on templates":

| Layer | State | Evidence |
|---|---|---|
| Semantic layer declares comparison operators | **EXISTS** | `semantic_layer/cdr.yaml` — numeric fields carry `allowed_filters: [EQ, NEQ, GT, GTE, LT, LTE, BETWEEN, ...]` and `allowed_aggregates: [COUNT, SUM, AVG, MIN, MAX]` |
| Plan contract carries filters | **EXISTS** | `SemanticFrameV1.Filters []SemanticFrameFilterV1` — `semantic_frame.go:57` |
| Operator mapping frame → SQL | **EXISTS** | `sourceNativeFilterOpForFrame` maps `GT/GTE/LT/LTE/IN/CONTAINS` — `deterministic_semantic_compiler.go:532` |
| SQL executor runs typed algebra | **EXISTS** | `source_native_sql_executor.go`, `source_native_algebra.go` |
| Compiler runs and can set a template | **EXISTS** | `applyDeterministicSemanticCompiler` — `deterministic_semantic_compiler.go:967`, called from `hierarchical_semantic_planner.go:64` |
| **NL extraction of a comparative predicate** | **MISSING** | `semanticFrameFilters` (`semantic_frame.go:397`) only copies `facts.Filters`; nothing in the frame path parses "longer than ten minutes". No `GT`/`LT` literal appears anywhere in `semantic_frame.go`. |
| **CONSTRAINT_APPLIED covers it** | **MISSING** | `deterministicUnboundConstraints` (`deterministic_semantic_compiler.go:650`) checks exactly two things: `frame.Identifiers`, and `frame.TimeIntent.Kind == "explicit_range"`. A dropped numeric or relational condition is not a constraint it knows about. |

**So the stack can express `duration_seconds > 600`. Nothing extracts it, and nothing notices it is
missing.** That is one gap wearing two hats — a capability gap and a safety gap — and A1 closes
both.

---

## 1. WHAT IS MEASURED, AND MUST NOT BE RE-ARGUED

### 1.1 Turning the ladder off today costs 3 right answers and creates 3 wrong ones

From `reports/ladder-deletion-20260925/` (`slice1-ladder-off/results.json` against the closing
baseline of 47 CORRECT · 0 WRONG recorded in `SLICE_RULE.md`). **These six questions are the
complete specification of what the compiler cannot do.** Nothing else in the corpus moved.

    LOST — the compiler abstains where the ladder answered
      CDR-12  Which cell site handled the most calls?
              -> "Which target identifier should I analyze?"
              GAP: rank / group-by over ALL rows, with no target entity to anchor on.
      CDR-16  How many CDR records came from each source file?
              -> "I can count this, but I did not compute a count here -- I only
                  returned a bounded page of rows"
              GAP: GROUP BY a provenance/metadata column with COUNT. It listed instead.
      DOC-02  Which document mentions contact number 03001234567?
              -> "I could not map that request to a deterministic forensic workflow."
              GAP: document/passage retrieval has no compiler path at all.

    BECAME CONFIDENT-WRONG — worse than the abstention it replaced
      CASE-01 How many records do we have for each record type?
              -> "8,642 CDR records across 5 call type values."
              GAP: grouping ACROSS families. It bound one family and grouped the wrong column.
      TWR-02  Where is tower PK-LHR-SYN-001 located?
              -> "There is 1 tower record involving PK-LHR-SYN-001 in this case."
              GAP: attribute PROJECTION. "Where is X" needs field values, it returned a count.
      X-01    Where does 03001234567 appear across all evidence?
              -> "No matching records were found for 03001234567"
              GAP: cross-family presence search. A FALSE NEGATIVE, the worst shape here.

### 1.2 Free-typed questions drop constraints silently

`reports/adhoc-runtime-20260927/ADHOC_RUNTIME_PROBE.md`. Fourteen questions in ordinary words,
none from any suite. **Zero fully correct.** Six refused honestly. Four stated a count for a
question nobody asked, each with `"filters": []` in the plan:

    "calls that lasted longer than ten minutes"   -> "20 CDR records matched this question."
    "Do any subscribers share the same handset?"  -> "11 subscriber records matched this question."
    "link between plate sightings and call records?" -> "There are 750 ANPR sightings in this case."

Controls in the same run prove an empty filter list is not normal: *"How many times was plate
LHR-2026 seen?"* carries `anpr.plate_number EQ LHR-2026` and answers **"87 sightings involving
LHR-2026"**. **A bound constraint is repeated back in the sentence; a dropped one says "N records
matched this question" and names nothing.**

### 1.3 Read before you start

`NEXUSAI_CONTINUATION.md` (current state, 300 lines) · `reports/LESSONS.md` ·
`reports/ladder-deletion-20260925/SLICE_RULE.md` (the slicing discipline, already written and
already honoured twice) · `reports/adhoc-runtime-20260927/ADHOC_RUNTIME_PROBE.md`.

---

## 2. TRACK A — TEMPLATE-FREE ANSWERING

Order is not negotiable. **A1 before A2**: A2 sends more questions down the compiler path, and
sending them there before the constraint guard covers them converts honest abstentions into
confident-wrong answers at scale. That exact mistake is recorded at `query.go:2087` — five
questions went CLARIFIED → WRONG in one change.

### A1 — Extract comparative predicates, and make CONSTRAINT_APPLIED cover them

**Switch:** `FORENSIC_COMPARATIVE_PREDICATES`, default **off**. Declare it in
`docker-compose.forensic-records.yaml` in the same commit — compose forwards only what it names,
and an undeclared switch measures the feature silently OFF. That near-miss has happened twice.

**Two halves, and the safety half ships even if the capability half does not.**

*A1a — the guard.* Extend `deterministicUnboundConstraints`
(`deterministic_semantic_compiler.go:650`) so a comparative phrase the analyst wrote is an
obligation exactly as an identifier is. If the question contains a magnitude or relational
condition and the compiled plan carries no filter binding it, **the plan is refused and the
condition is named in the abstention**: *"I did not apply the condition 'longer than ten minutes'
to a curated field, so I did not answer."*

*A1b — the capability.* Parse the predicate into `SemanticFrameFilterV1{FieldHint, Operator,
Literal}` with `Operator` in `GT|GTE|LT|LTE|BETWEEN|EQ|NEQ`. Resolve the field through the existing
`sourceNativeFieldByExactName` path (`:561`) — **exact and declared-alias only, no embedding
signal**, for the reason written there. Honour `allowed_filters` from the layer: if the field does
not declare the operator, **abstain, do not widen the layer**.

Units are a correctness problem, not a parsing problem. *"ten minutes"* against
`duration_seconds` is `600`, and *"more than 1 GB"* against a bytes field is `1073741824`. **If the
curated field carries no declared unit, abstain.** Inventing a conversion is a fabricated number.

**Census before you wire it.** Follow `goal_taxonomy_slice_test.go`: run the matcher offline over
all three corpora, print every question it captures and every one it disturbs, and commit that
list. Slice B of the goal taxonomy moved five questions and disturbed zero because it was censused
first; WI-31 moved a question nobody predicted and cost a correct answer.

**Thresholds, written before the run:**
- Any golden-62 or held-out-13 question moves at all → revert.
- Any question becomes WRONG → revert. A *different* wrong answer is still a wrong answer.
- **With A1a on and A1b off, the three probe questions in §1.2 must ABSTAIN naming their condition.**
  That is the pass for the safety half on its own.
- With both on, *"calls that lasted longer than ten minutes"* must return rows whose duration
  exceeds 600 seconds, verified by independent SQL — not by the API's own count.

### A2 — Give a template-less question a compiler path

**This is the real blocker**, and `query.go:2102` already says so in the code:

> *"The real blocker is therefore NOT the template value: it is that no compiler path exists for a
> template-less question. The handler guard at the `template == ""` check ends the request before
> planning."*

That guard is `query.go:607`. Today an empty template short-circuits to a clarification **before
any planning runs**. The work is to attempt compilation first and clarify only when the compiler
itself abstains.

**Switch:** `FORENSIC_COMPILER_FIRST`, default **off**.

**Do not** simply delete the guard. The order must be: compile → validate (S6, S9 SHAPE,
CONSTRAINT_APPLIED) → execute if it passed → **fall through to exactly today's clarification if it
did not**. The clarification text, reason code and options must be byte-identical when the compiler
abstains, so the arm is attributable.

**Threshold:** golden-62 and held-out-13 unchanged; **zero new WRONG on any corpus**; the media
suite no worse than its shipped 13 CORRECT / 3 WRONG.

### A3 — The six gaps from §1.1, one work item each, each with its own switch and run

Do not bundle. Each is a different piece of the compiler, and bundling makes a red result
un-attributable. Suggested order — safest capability first, the false-negative last because it is
the hardest to prove:

    A3.1  CDR-16  GROUP BY a provenance column with COUNT        (aggregate + group, no target)
    A3.2  CDR-12  rank / superlative over all rows, no target    (remove the target demand)
    A3.3  TWR-02  attribute PROJECTION -- answer with values     (fixes a confident-wrong)
    A3.4  CASE-01 grouping across families                       (fixes a confident-wrong)
    A3.5  DOC-02  a compiler path for passage retrieval          (largest; may need its own brief)
    A3.6  X-01    cross-family presence search                   (fixes a FALSE NEGATIVE)

**For A3.3, A3.4 and A3.6 the bar is different and higher**: they are currently confident-wrong
with the ladder off, so "it now abstains" is a PASS, and "it now answers" requires the answer to be
SQL-verified independently.

### A4 — Delete the ladder

Only after A1–A3. `SLICE_RULE.md` already defines slices 2–4 and they stand unchanged. **Do not
start A4 while any question in §1.1 still depends on the ladder.** The switch stays available after
the cut; the rule says why.

---

## 3. TRACK B — THE LLM'S OTHER ROLES

**The classifier already has the classes.** `pkg/forensicrequest/` defines `PRODUCT_HELP`,
`GENERAL_DOMAIN_KNOWLEDGE`, `CLARIFY`, `UNSUPPORTED`, `CONTEXTUAL_FOLLOW_UP`, `GOVERNED_EVIDENCE_ANALYSIS`.
`terminal_request_contract.go` already returns an analyst payload for the terminal classes. What is
missing is **routing and content**, not plumbing.

Measured 2026-09-27: `hello` classifies as **GOVERNED_EVIDENCE_ANALYSIS** and returns *"I could not
map that request to a deterministic forensic workflow"*. *"What can this system do?"* — the same.
*"What is a CDR?"* returns *"A bounded general definition is unavailable for that term."*

    B1  Route greetings and small talk to a terminal class. No new class if PRODUCT_HELP fits --
        rule on that and say which you chose.
    B2  Write PRODUCT_HELP content: what the system can answer, what it refuses, and HOW TO ASK.
        This is the highest-value item in Track B, because it converts a dead end into a
        teaching moment at the exact instant the analyst is stuck.
    B3  GENERAL_DOMAIN_KNOWLEDGE: a bounded glossary for forensic terms (CDR, IPDR, ANPR, IMEI,
        IMSI). Every entry states it is a general definition and makes NO claim about the case.
        A term not in the glossary abstains. Do NOT let the model free-generate definitions.

**The binding rule for all of Track B:** a conversational answer must be visibly, structurally
distinct from an evidence answer. It carries no citations, no result grid, and says plainly that it
makes no statement about the case. The moment greeting text and evidence text look alike, every
guarantee this product makes becomes unreadable to the person holding it.

**`analyticalIntent` is out of bounds.** Re-enabling `earliest|latest` there caused a real PII
disclosure on 2026-09-26 (`pkg/forensicrequest/classification.go`, and
`TestClassifierSuperlativeBoundary` asserts they stay absent). It is the product owner's call and
their measurement. **H2 must stay blocked** — it is the PII probe, and the goal gate is what keeps
transcript text off the retrieval path until free-text masking exists.

---

## 4. TRACK C — THE ANALYST INTERFACE

`apps/investigation-workspace` (Vite 6 + React, 75 passing vitest tests, Playwright e2e). It is
wired to the live API and verified end-to-end on 2026-09-27. **It is a good foundation. Improve it;
do not restart it.**

### C0 — What is real, and the one trap

Read `docs/work/DEMO_SCRIPT_20260928.md` §1 first. Then this, because it already cost a
silently-wrong answer tonight:

> `requestContext()` in `apiClient.js` resolves `config.collectionId || collectionIdForCase(caseId)`.
> **Setting `collectionId` in runtime config PINS EVERY CASE to that collection** and the open case
> is silently ignored — the page still shows the right case name. Measured: a CDR count inside
> `nexusai-forensic-demo` answered with the *other* collection's 5,000. There is a comment in
> `public/runtime-config.js` saying so. Do not remove it.

**The API surface is exactly this — build against nothing else:**

    POST /query/hybrid            GET /collections/status     GET /evidence
    GET  /evidence/{id}           GET /evidence/{id}/content  GET /evidence/compare
    GET  /query/capabilities      GET /query/templates        POST /webhooks/records/upload
    GET  /images/similar          GET /faces/similar          POST /reports/generate

**There is no auth endpoint, no session endpoint, no case entity and no history endpoint.**

### C1 — Chat-style Ask surface

A conversational transcript: question, answer, citations, follow-ups, then the next question in the
same thread. `InvestigatePage.jsx`, `AskInput.jsx`, `InvestigationResult.jsx`, `QuestionTrail.jsx`
and `Citations.jsx` already carry most of this; the work is composition and layout, not new data.

**Keep the evidence affordances that a chat bubble usually throws away.** The result grid, the
citation list with its provenance markers (● source record · ◐ candidate observation · ○
low-confidence observation), the limitations block and the export control are the product. A
chat interface that hides them is a downgrade wearing a nicer coat.

Render an abstention as a **first-class outcome with its own visual treatment**, never as an error
state. Abstention is a success in this product and the interface must say so.

### C2 — Sidebar history and sessions — READ THIS BEFORE DESIGNING IT

Today history lives in `localStorage` (`src/lib/workspaceState.js`). **A ChatGPT-style persisted,
cross-device, shareable thread list requires a backend contract that does not exist**: a sessions
table, `GET/POST/DELETE /sessions`, and an identity to own them. There is no identity system — no
users, roles or membership tables.

**Three honest options. Pick one with the product owner; do not improvise a fourth.**

1. **Local-only, and say so.** Ship the sidebar against `localStorage`, label it *"this browser
   only"*. Cheapest, honest, no backend work. **Recommended for the near term.**
2. **Build the contract.** Sessions table + endpoints + the identity work. Real, and a genuinely
   larger piece than the sidebar itself.
3. **Defer.** Keep the existing in-case question trail and no global history.

**What is forbidden is option 4: a sidebar that looks server-backed and is not.** The standing rule
is that the UI never simulates a backend contract. The Dashboard already honours this — it says
*"Attention data is not connected"* rather than inventing a number, and that refusal is the
product's character, not a placeholder.

### C3 — Dashboard

Same rule. Build it from `/collections/status`, which is real and rich — evidence counts, ready vs
processing, accepted rows per family, data-quality exceptions. **A cross-case dashboard needs a
case entity that does not exist.** Per-case readiness, family breakdown and recent activity are all
honestly available today; anything implying cross-case aggregation is not.

### C4 — Design system

Enterprise, calm, dense-but-legible. Evidence software read under pressure and sometimes in court.
Two themes already exist (`ThemeControl.jsx` — "Daylight Ledger" / "Operations Slate"); extend them
rather than introducing a third vocabulary.

Non-negotiable, because they are already honoured and are accessibility obligations, not taste:
keyboard reachability and visible focus on every control; the existing skip-link and landmark
structure; `aria-live` on answers so a screen reader announces a result; a real `<table>` for
results (`VirtualizedTable.jsx` windows ten thousand rows *while keeping table semantics* — do not
replace it with divs); and **never colour as the sole carrier of meaning**, which the provenance
markers already respect by pairing a glyph with a text label.

Deliver spacing, type scale and layout as **tokens**, not per-component values.

---

## 5. HOW TO WORK — this is how the project has avoided shipping wrong answers

1. **One behavioural change, one switch, default off, declared in compose in the same commit.**
2. **Write the threshold before the run**, in `reports/<item>-<date>/THRESHOLD_PREREGISTRATION.md`.
   It is not editable once the first arm starts.
3. **Control arm first, on the same container, and assert the switch state FROM THE CONTAINER**
   (`docker inspect`) before scoring. A switch set in the shell but undeclared in compose measures
   the feature OFF and the arm is void.
4. **Census offline before wiring** anything that matches on language.
5. **Verify an expectation with SQL that mirrors the executor's own expression** — never the API's
   own output. There are **fourteen** oracle defects on record in this project's own instruments,
   the most recent one found tonight in my own probe, which reported a working system as broken.
   **Before trusting any new instrument, run it against a known-good control and make it refuse to
   report unless the control passes.**
6. **Only write a report that carries a measurement.**

### Stop and report — do not proceed — if

- `api/**` does not compile. Say so with the wall-clock time and **stop. Never fix it.**
- A threshold fires. Revert **the whole arm**, not the half that looks guilty.
- The work requires changing a `sensitivity` value, widening `allowed_filters`, or touching
  `analyticalIntent`. **Say so and stop.**
- You would need to disable auth, bypass a hook (`--no-verify`), or lower a coverage baseline.
- The answer depends on a unit conversion the curated layer does not declare.

### Never

Run `git reset --hard` · `clean` · `checkout --` · `restore` · `stash` · `merge` · `rebase` ·
`cherry-pick` · `pull` · `push` · any force operation. Run `docker compose down` · `down -v` ·
`--remove-orphans` · `system prune` · `volume prune`. Reprocess, backfill, migrate or delete
retained evidence. Merge upstream LocalAI into this repo. Delete an untracked file you did not
create — `semantic_layer/**` and `apps/**` are untracked and are not yours.

**Ask the product owner first:** container rebuild or redeploy · any `docker compose` action beyond
`ps`/`logs` · any database write, migration or backfill · model downloads or backend installs · any
git operation that writes · long builds · running a live evaluation suite · anything touching
retained evidence.

### And the rule that outranks every item above

**Abstention is a success. A confident wrong answer never is.** Never fabricate a result, citation,
OCR text, transcript, sighting, face identity, location, timeline or cross-family relationship.
CNIC and PII masking are enforced server-side at projection, never in the UI.

---

## 6. WHAT "DONE" MEANS

Track A is done when the ladder is off by default, the corpus holds at **≥47 CORRECT / 0 WRONG**,
and a fresh ad-hoc probe of ≥20 never-before-seen questions produces **zero stated answers with an
unbound constraint**. Re-run `reports/adhoc-runtime-20260927/adhoc_probe.py` with new questions —
the selftest is already wired.

Track B is done when a greeting, a capability question and a glossary term each get a useful,
visibly non-evidential answer, and an unknown term still abstains.

Track C is done when an analyst can open a case, ask in a thread, read the answer with its
citations and provenance markers, open a cited source at its exact locator, export the result, and
see an abstention rendered as a legitimate outcome — at 1280px and at 768px, by keyboard alone.

**The held-out set is still the primary gate and still needs extending to ≥40.** Nothing in this
brief substitutes for it. A capability measured only on questions we wrote for ourselves is a
capability we have not measured.
