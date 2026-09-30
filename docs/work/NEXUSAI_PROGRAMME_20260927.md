# NexusAI — execution programme, UI first, Claude and Codex in parallel

Written 2026-09-27. Supersedes the ordering in `WI-NL2SQL-UX_MASTER_PROMPT_20260927.md`; that
document's **evidence and work-item definitions still stand** and are referenced throughout. Read
it for the measurements behind every claim here.

**Product-owner decision, recorded:** the interface is built first, so progress is visible and
demonstrable while the engine work proceeds behind it. I raised the risk once and it was decided.
This document makes that ordering *safe* rather than merely faster — see §2.

---

## 1. THE SPLIT THAT MAKES PARALLEL WORK ACTUALLY PARALLEL

    CLAUDE owns    apps/**                    the analyst interface
    CODEX owns     api/**  semantic_layer/**  the engine and the vocabulary
    NEITHER owns   evaluation/**  reports/**  shared evidence; append, never rewrite

**These sets do not intersect, so both can work at full speed with no merge conflict and no
coordination overhead.** That is the whole point of the split. The only shared contract is the JSON
that `POST /query/hybrid` returns, and §2.3 fixes how it changes.

One rule, absolute: **Claude never edits `api/**`. Codex never edits `apps/**`.** If either needs
the other side to change, it is written down and handed over, not reached across.

---

## 2. WHY UI-FIRST IS SAFE, AND THE ONE THING THAT MUST RUN BESIDE IT

### 2.1 The risk, stated plainly

A polished interface makes a **wrong** answer more convincing, not less. Measured 2026-09-27
(`reports/adhoc-runtime-20260927/`): *"Show me all the calls that lasted longer than ten minutes"*
returns **"20 CDR records matched this question"** with the duration filter silently dropped —
`"filters": []` in the plan. Rendered in a beautiful answer card with citations, that becomes a
finding somebody acts on.

### 2.2 The mitigation — Codex ships A1a in week 1, in parallel

**A1a is the safety half of A1 and nothing else.** It does not add capability. It makes a dropped
constraint refuse instead of answer. It is small, it is independent of the UI, and it removes
exactly the failure mode that UI-first would amplify. Full spec: `WI-NL2SQL-UX_MASTER_PROMPT` §A1.

**This is the only backend item with a hard deadline: it must land before the interface is shown
to anyone outside the team.**

### 2.3 The contract between the two tracks

The UI renders whatever `/query/hybrid` returns. **Codex must not remove or rename a response field
the UI reads.** Additive changes are free. If a field must change shape, Codex writes it in
`reports/OPEN_ITEMS.md` and it is agreed before the change ships.

The UI must degrade honestly when a field is absent — a missing block renders as nothing, never as
a zero, an empty state that implies "none", or a placeholder.

---

## 3. PHASE 1 — THE INTERFACE (Claude) · ~2 weeks · demoable from day 3

Current state, read before changing anything: `apps/investigation-workspace`, Vite 6 + React,
**75 passing vitest tests + Playwright e2e**, wired to the live API and verified end-to-end.

**The design system already exists and is good.** `src/styles/tokens.css` with `tokens.vitest.js`
enforcing: a **7-step type scale**, **4 weights**, spacing `8/12/16/20/24/32/48`, radii `8/12/16`,
control height `42px`, content widths `1280px` / `1060px`, two themes kept identical between media
and forced dark, **WCAG AA contrast asserted by test** (4.5:1 text, 3:1 lines), and Latin / Urdu /
mono font roles because evidence contains mixed strings.

> **So this is not a design-system rebuild. It is information architecture, three new surfaces, and
> layout craft on a sound base.** Extend the tokens; do not replace the vocabulary, and do not
> weaken a contrast ratio to achieve a look. `tokens.vitest.js` will fail, and it is right to.

### 1.1 — Application shell and information architecture · 2 days

A persistent three-region shell: **left rail** (navigation + case + threads), **main canvas**,
**contextual right panel** (citations, evidence preview, comparison) that collapses under 1280px.

- Left rail: case switcher, thread history (§1.3), primary navigation, collapsible to icons.
- Main canvas respects `--analyst-content-reading` (1060px) for text and may go to
  `--analyst-content-wide` (1280px) for result grids.
- Breakpoints: **1440 / 1280 / 1024 / 768**. At 768 the rail becomes a drawer and the right panel
  moves inline below the answer. No horizontal page scroll at any width.
- Keep the existing skip-link and landmark structure. They already pass; do not regress them.

### 1.2 — The conversation surface · 4 days · **the centrepiece**

A threaded transcript: question → answer → follow-up, with a sticky composer. `InvestigatePage.jsx`
(121 lines), `AskInput.jsx`, `InvestigationResult.jsx`, `QuestionTrail.jsx` and `Citations.jsx`
already carry most of the data; this is composition, not new plumbing.

**Answer card anatomy — fixed order, every time:**

    1  STATE        answered · answered-with-limits · abstained · clarification-needed
    2  HEADLINE     the stated sentence, the number typographically dominant
    3  RESULT       the grid (VirtualizedTable) when there is one, with filter/density/export
    4  CITATIONS    per source: filename, contributing rows, PROVENANCE MARKER, open-at-locator
    5  LIMITATIONS  coverage limits, verbatim, never collapsed away by default
    6  FOLLOW-UPS   suggested next questions

**Do not throw away the evidence affordances to make it feel like a chat app.** The grid, the
provenance markers, the limitations block and export are the product. A chat bubble that hides them
is a downgrade wearing a nicer coat.

**Provenance markers are a primary visual element, not a footnote.** Three states, each a glyph
*plus* a text label — never colour alone:

    ●  Source record              an ingested row: what was collected
    ◐  Candidate observation      a model's inference from pixels or audio
    ○  Low-confidence observation an inference the model itself is unsure of

**Abstention gets a first-class, composed treatment** — its own card style, calm not alarming, with
the reason and a concrete "try asking…" affordance. It is a success in this product. If it renders
like an error, the product's whole character is misread. This is the single highest-leverage
visual decision in Phase 1.

**Progress states matter**: answers take 0.1s to ~100s on CPU-only inference. Show a determinate-
feeling progress treatment with the stage being executed, not a spinner.

### 1.3 — Thread history in the rail · 2 days

**Decision required from the product owner before this starts. Default recommendation: option 1.**

1. **Local-only, labelled.** `localStorage` (today's `src/lib/workspaceState.js`), with the rail
   saying **"this browser only"**. 2 days. Honest and shippable now.
2. **Server-backed.** Needs a sessions table, `GET/POST/DELETE /sessions`, *and* an identity system
   — none of which exist. Two weeks minimum and it is Codex's work, not Claude's.
3. **Defer.** Keep the in-case question trail only.

**Option 4 is forbidden: a rail that looks server-backed and is not.** The standing rule is the UI
never simulates a backend contract. The Dashboard already honours this — it says *"Attention data
is not connected"* instead of inventing a number, and that refusal is the product's character.

Per thread: title from the first question, timestamp, case, pin, rename, delete-with-confirm.

### 1.4 — Dashboard · 2 days

Built from `GET /collections/status`, which is genuinely rich: evidence counts, ready vs
processing, accepted rows per family, data-quality exceptions. Verified live: 43 evidence items, 42
ready, 10,168 accepted rows, 8 families.

**Per-case readiness is real. Cross-case aggregation is not** — there is no case entity. Anything
implying a portfolio view must say it is unavailable, as it does today.

### 1.5 — Craft pass and accessibility proof · 2 days

Spacing rhythm, optical alignment, table density, empty states, loading skeletons, focus order,
motion (respect `prefers-reduced-motion`).

**Non-negotiable, because they are obligations and several already pass:**
- Every control keyboard-reachable with a visible focus ring (`--analyst-focus-ring`).
- `aria-live` on the answer region so a result is announced.
- **A real `<table>` for results.** `VirtualizedTable.jsx` windows ten thousand rows *while keeping
  table semantics*. Do not replace it with divs.
- Never colour as the sole carrier of meaning.
- Both themes pass `tokens.vitest.js` contrast assertions.

### Phase 1 definition of done

An analyst opens a case, asks in a thread, reads the answer with citations and provenance markers,
opens a cited source at its exact locator, exports the result, sees an abstention rendered as a
legitimate outcome, and browses prior threads — **at 1440, 1280, 1024 and 768, by keyboard alone,
in both themes**, with all 75 vitest tests plus e2e green.

**Demo checkpoints:** shell + conversation surface at **day 3**; provenance and abstention
treatment at **day 7**; dashboard and history at **day 10**; craft pass at **day 12**.

---

## 4. PHASE 1 (PARALLEL) — CODEX BRIEF: A1a, THE CONSTRAINT GUARD

**Hand this to Codex on day 1, at the same time as Phase 1 starts.**

### The ask

`deterministicUnboundConstraints` (`api/forensic_records/deterministic_semantic_compiler.go:650`)
is the S9 CONSTRAINT_APPLIED check. It currently recognises exactly two kinds of obligation:
`frame.Identifiers`, and `frame.TimeIntent.Kind == "explicit_range"`.

**A magnitude or relational condition the analyst wrote is not among them**, so a plan that drops
it verifies perfectly and answers confidently. Measured, three times over, in
`reports/adhoc-runtime-20260927/ADHOC_RUNTIME_PROBE.md` §3.

**Extend the guard so such a condition is an obligation exactly as an identifier is.** When the
question carries one and the compiled plan binds no filter to it, **refuse and name the condition**:

> *"I did not apply the condition 'longer than ten minutes' to a curated field, so I did not answer."*

**A1a adds no capability. It must not make a single question answer that does not answer today.**
Parsing the predicate into a filter is A1b and is a separate item with its own measurement.

### Switch, and the trap that has bitten twice

`FORENSIC_CONSTRAINT_OBLIGATIONS`, default **off**, and **declared in
`docker-compose.forensic-records.yaml` in the same commit**. Compose forwards only the variables it
names; a switch set in the shell but undeclared reaches nothing and the arm silently measures the
feature OFF. That near-miss has happened twice on this project and was caught both times only by
asserting container env before scoring.

### Census first, before wiring

Follow the pattern in `goal_taxonomy_slice_test.go`: run the detector **offline** across all three
corpora, print every question it would capture and every one it would disturb, and commit that
list. Slice B of the goal taxonomy moved 5 and disturbed 0 because it was censused first. WI-31
skipped this and cost a correct answer.

### Thresholds — write them down before the first arm runs

- Golden-62 and held-out-13: **any movement at all reverts.** A1a must be inert on questions whose
  constraints already bind.
- **Any question becomes WRONG → revert the whole arm**, not the half that looks guilty.
- **PASS condition**, measured directly:

        "Show me all the calls that lasted longer than ten minutes"   -> ABSTAINS, names the condition
        "Do any subscribers share the same handset?"                  -> ABSTAINS, names the condition
        "Is there any link between the plate sightings and the call records?" -> ABSTAINS

- Control arm first, same container, **switch state asserted from `docker inspect` before scoring.**

### Stop and report — do not proceed — if

- `api/**` does not compile. Say so **with the wall-clock time** and stop. **Never fix it.**
- You would need to change a `sensitivity` value, widen `allowed_filters`, or touch
  `analyticalIntent`. Re-enabling `earliest|latest` there caused a real PII disclosure on
  2026-09-26; `TestClassifierSuperlativeBoundary` asserts they stay absent. **Say so and stop.**
- The detector cannot distinguish a condition from ordinary phrasing without guessing. **A refusal
  to guess is a finding, not a failure** — three of your reports have been worth more than the work
  they cancelled.

### Verify

```bash
go test -C api/forensic_records -count=1 ./...
```

Report the census list, the threshold outcomes, and the container-asserted switch state. **Do not
run the live evaluation suites without the product owner's go-ahead.**

---

## 5. PHASE 2 — THE ENGINE (Codex) · after A1a lands

Full specifications in `WI-NL2SQL-UX_MASTER_PROMPT_20260927.md` §A. Order is not negotiable and
each item is **one switch, one measurement, one report**. Do not bundle: a bundled red result is
un-attributable, and this project has paid for that.

    A1b  parse comparatives into frame filters         2 days   capability half of A1
    A2   compiler path for template-less questions     1-2 days THE blocker; query.go:607
    A3.1 CDR-16  GROUP BY a provenance column          1-1.5
    A3.2 CDR-12  rank over all rows, no target         1-1.5
    A3.3 TWR-02  attribute projection                  1-1.5   fixes a confident-wrong
    A3.4 CASE-01 grouping across families              1-1.5   fixes a confident-wrong
    A3.6 X-01    cross-family presence search          1-2     fixes a FALSE NEGATIVE
    A3.5 DOC-02  passage retrieval via the compiler    3-5     largest; spike before committing
    A4   ladder deletion, slices 2-4                   1-2     SLICE_RULE.md already defines these

**The six questions in A3 are the complete measured specification of the compiler's gaps** —
derived from `reports/ladder-deletion-20260925/`, where turning the ladder off cost 3 correct
answers and created 3 wrong ones, and nothing else in 62 moved.

For **A3.3, A3.4 and A3.6 the bar is higher**: they are currently confident-wrong with the ladder
off, so *"it now abstains"* is a PASS, and *"it now answers"* requires the value to be verified by
independent SQL that mirrors the executor's own expression.

---

## 6. PHASE 3 — THE LLM'S OTHER ROLES (Codex) · 2-4 days

The classifier **already has the classes** (`pkg/forensicrequest/`): `PRODUCT_HELP`,
`GENERAL_DOMAIN_KNOWLEDGE`, `CLARIFY`, `UNSUPPORTED`, `CONTEXTUAL_FOLLOW_UP`. What is missing is
routing and content. Measured: `hello` classifies as **GOVERNED_EVIDENCE_ANALYSIS** and returns
*"I could not map that request to a deterministic forensic workflow."*

    B1  route greetings and small talk to a terminal class (rule on whether PRODUCT_HELP fits)
    B2  PRODUCT_HELP content: what it answers, what it refuses, HOW TO ASK   <- highest value
    B3  GENERAL_DOMAIN_KNOWLEDGE: a bounded glossary (CDR, IPDR, ANPR, IMEI, IMSI).
        An unknown term ABSTAINS. The model never free-generates a definition.

**Binding rule:** a conversational answer must be structurally distinct from an evidence answer —
no citations, no result grid, and it states plainly that it makes no claim about the case. Claude
gives it a distinct card treatment in the UI. **H2 stays blocked**; it is the PII probe.

---

## 7. WORKING DISCIPLINE — both tracks

1. One behavioural change, one switch, default off, **declared in compose in the same commit**.
2. Threshold written **before** the run, in `reports/<item>-<date>/THRESHOLD_PREREGISTRATION.md`.
   Not editable once the first arm starts.
3. Control arm first, same container, **switch asserted from the container** before scoring.
4. **Census offline before wiring** anything that matches on language.
5. Verify an expectation with **SQL mirroring the executor's own expression**, never the API's own
   output. **Fourteen oracle defects** are on record in this project's own instruments — the most
   recent found on 2026-09-27 in a probe that reported a working system as broken. **Any new
   instrument must run a known-good control first and refuse to report unless it passes.**
6. Only write a report that carries a measurement.
7. `NEXUSAI_CONTINUATION.md` is the state file: **rewrite it, never append, hard cap 300 lines.**

### Never

`git reset --hard` · `clean` · `checkout --` · `restore` · `stash` · `merge` · `rebase` ·
`cherry-pick` · `pull` · `push` · any force operation. `docker compose down` · `down -v` ·
`--remove-orphans` · `system prune` · `volume prune`. Reprocess, backfill, migrate or delete
retained evidence. Disable auth to fix a crash-loop. Bypass hooks with `--no-verify`. Lower a
coverage baseline. Merge upstream LocalAI into this repo. Delete an untracked file you did not
create.

### Ask the product owner first

Container rebuild or redeploy · any `docker compose` action beyond `ps`/`logs` · any database
write, migration or backfill · model downloads or backend installs · any git operation that writes
· long builds · **running a live evaluation suite** · anything touching retained evidence.

### Environment notes that cost hours when forgotten

Set env from **PowerShell, never Git Bash** (MSYS rewrites `/data/...`). Postgres is host port
**5433**, db `localrecall`. Bash heredocs break on apostrophes — write a file instead. An empty
JSON-Schema `enum` is an unparseable grammar and 500s before inference.

### The rule that outranks everything above

**Abstention is a success. A confident wrong answer never is.** Never fabricate a result, citation,
OCR text, transcript, sighting, face identity, location, timeline or cross-family relationship.
PII masking is enforced server-side at projection, never in the UI.

---

## 8. TIMELINE

    Week 1   Claude  shell + conversation surface          DEMO day 3
             Codex   A1a constraint guard                  <- must land before external demos
    Week 2   Claude  provenance, abstention, dashboard,
                     history, craft pass                   DEMO day 10 and day 12
             Codex   A1b + A2                              free-typed questions start working
    Week 3   Codex   A3.1 - A3.4                           common analytical shapes, no templates
             Claude  UI polish against real new answers
    Week 4   Codex   A3.6 + B1-B3                          false negative fixed, conversation
    Week 5-6 Codex   A3.5 + A4                             passage retrieval, ladder deleted

**End of week 2: a complete interface over a system that never states an answer it did not
compute.** That is the demo that matters, and it is two weeks away.

**End of week 4: template-free answering for the shapes analysts actually ask.**

Weeks 5–6 are the long tail and can be deferred without losing the story.

### Still owed, and not substitutable

**The held-out set must reach ≥40 questions.** It is the primary gate. A capability measured only
on questions we wrote for ourselves is a capability we have not measured. Re-run
`reports/adhoc-runtime-20260927/adhoc_probe.py` with fresh questions at the end of each phase — the
selftest is already wired.
