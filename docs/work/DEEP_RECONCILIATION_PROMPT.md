# DEEP RECONCILIATION & ROADMAP — paste this to start the work

You are the lead engineer on **NexusAI**, a forensic intelligence product at
`C:\Users\sheik\Workspace\Office\Projects\NexusAI` (LocalAI fork, branch
`codex/forensic-hybrid-checkpoint-20260723`).

**Your output for this first task is a RECONCILIATION and a ROADMAP, not code.**
Do not start implementing until the analysis is delivered and I approve the
plan. Then we implement it in measured slices.

---

## 1. THE GOAL, stated plainly

An analyst must be able to type **any** question and get an appropriate,
accurate, safe answer — or an honest refusal. Not just the questions someone
wrote a template for.

That includes, and none of these may be forgotten:

| Question kind | Example | Expected behaviour |
|---|---|---|
| **Data-driven analytical** | "how many calls did 03001234567 make in August" | computed from evidence, cited, verifiable |
| **Data-driven open-ended** | "where was this number seen", "what's unusual here" | answered from data, or an honest gap |
| **Media / derived** | "which image says X", "what was said at 5:32 in the recording" | from OCR / transcript / ANPR with exact locators |
| **Cross-family** | "where does this number appear across all evidence" | joined across families, or scoped refusal |
| **Product help** | "how do I add evidence", "what can this system do" | grounded in the real product, never invented |
| **Domain concept** | "what is an IMSI", "what does VoLTE mean" | general knowledge, clearly marked as NOT evidence |
| **Greeting / chit-chat** | "hi", "thanks", "who are you" | brief, professional, no fabricated capability |
| **Out of scope** | "what is the suspect's blood type" | refuse, say why, suggest what IS available |
| **Ambiguous** | "show me the totals" | one specific clarifying question with real options |

**The priority for the FIRST implementation phase is the data-driven ones** —
runtime, real-world questions answered correctly and well-reasoned, with the LLM
used for what it is good at and never as the source of truth.

## 2. WHAT IS ALREADY TRUE — verify each before relying on it

Read these first. They are current and measured:

    NEXUSAI_CONTINUATION.md                        state, hard rules, hazards (<=300 lines)
    docs/work/MASTER_EXECUTION_PROMPT.md           binding constraints and phase plan
    docs/ux/NEXUSAI_PRODUCT_UX.md                  BINDING UI contract
    docs/architecture/RECONCILIATION_20260921.md   the architecture decision
    docs/work/CURATION_GAP_INVENTORY.md            every column, curated or not
    reports/tier-decision-20260924/                five decision rules and outcomes
    reports/ladder-deletion-20260925/SLICE_RULE.md why the ladder is still there

Headline numbers, all measured, all reproducible:

    golden suite v2   47 CORRECT · 14 CLARIFIED · 1 MANUAL · 0 WRONG · p95 5.2s warm
    from a baseline of 26% correct and 26 confident-wrong (2026-09-18)
    semantic layer    88/148 columns curated across all collections (59%)
                      81/94 on the demo collection (86%)
    query.go          9,041 lines; the keyword ladder is NOT deleted

**Do not take any of this on trust. Re-verify in source or by running the
harnesses before you build on it.** Several documented claims have already
turned out to be stale.

## 3. WHAT TO ANALYSE — leave nothing out

Go through every one of these, in the code and against the live system. For each,
state what works, what does not, and **how you know**.

**3.1 Query answering — the whole path.** Request classification (S1) — the four
classes in `pkg/forensicrequest/classification.go` and whether greetings and
out-of-scope are actually handled. Literal extraction, scope resolution, catalog
narrowing, enum-constrained IR generation, validation, self-correction,
compilation, S9 verification, abstention, Fact Packets, the deterministic answer,
optional narration, presentation.

**3.2 The non-analytical classes.** PRODUCT_HELP and GENERAL_DOMAIN_KNOWLEDGE
exist as classes — are they wired, grounded and safe? What happens to "hi"? To
"who are you"? To "what is an IMSI"? Test them live; do not assume.

**3.3 Media and derived artifacts — search exhaustively.** OCR
(`multilingual_ocr.py`, `image_intelligence.py`), audio/ASR and transcripts,
video and ANPR (`video_anpr_product_v2/v21/v22.py`, `video_anpr_onnx_v3.py` —
**four versions; which is live?**), documents (`document_pipeline.py`), face.
For each: what it produces, its confidence semantics, whether the confidence is
calibrated or noise, whether a citation can deep-link to its exact locator, and
whether a question about it can actually be answered today.

**3.4 Evidence lifecycle.** Upload, classification, ingestion, the adapters,
job/DLQ handling, custody, versions, lineage, retention, failure and retry,
reprocessing. What happens when a file fails? When a format is unknown?

**3.5 The semantic layer.** Coverage per family (the inventory above), type
correctness, sensitivity, joins, metrics, synonyms — and the cross-family join
graph, which scored 0/5 and is the weakest capability in the product.

**3.6 Architecture and boundaries.** What belongs to NexusAI vs upstream
LocalAI; the ~330-line core delta; the dead `core/services/records/` store; the
path to a pinned unmodified upstream image.

**3.7 Identity, cases, multi-tenancy.** There is NO case entity — `case_id`
appears 3× in the schema against `collection_id` 224×, and every analyst is one
shared service principal. This blocks real deployment.

**3.8 UI/UX.** Against `docs/ux/NEXUSAI_PRODUCT_UX.md` as the binding contract,
plus the app-shell spec and design system. The analyst app is
`apps/investigation-workspace/**`.

**3.9 Security, privacy, audit.** Auth posture, CNIC masking at projection, RLS,
the audit record, and whether "why did the system answer this way" is
reconstructable months later.

**3.10 Operations.** Deployment, switches, rollback, latency, cold-start,
resource ceilings on the target hardware.

## 4. HOW TO WORK — these are not style preferences

Every one of these was paid for with a measured failure on this project:

- **Measure before building.** Score a candidate offline against recorded runs
  before writing it. Six candidates were rejected this way in one session.
- **Fix the instrument first.** Three runs were mis-measured; 119 s was
  attributed to nothing; a coverage audit indicted every working field. **If a
  measurement surprises you, suspect the instrument before the system.**
- **A probe must reproduce the PATH, not just the function.** Calling the right
  converter with the wrong input type "proved" the architecture's keystone was
  broken. It was not.
- **Pre-declare thresholds in writing before any run**, including what would
  make you revert. Then honour them. This fired twice and was honoured twice.
- **Abstention is a success. A confident wrong answer never is.** Five attempts
  to convert two clarifications produced seven confident-wrong answers.
- **Never fabricate** a result, citation, OCR text, transcript, sighting, face
  identity, location, timeline or relationship. Never turn a similarity score
  into an identity claim.
- **No new operation templates. Do not extend `query.go`'s keyword ladder.**
- **No embeddings on structural decisions** (family, group, measure, field role).
- **No model roulette** — no swapping or benchmarking LLMs except against a
  threshold written down first and surfaced to me.
- **Ask before**: container rebuild or redeploy · any docker action beyond
  ps/logs · any database write or migration · model downloads · any git write ·
  long builds · running the live suites · anything touching retained evidence.
- **Report honestly.** If a test fails, show it. If a number regressed, lead
  with it. "Partially working" is valuable; "working" when it is not is the one
  unacceptable outcome.

**Things already measured and REJECTED — do not retry:** removing
generator-invented filters by any means · switching the ladder off without
replacing its scope contribution · narrowing the issued field set · withholding
measures for a `lookup` goal · "breakdown" → aggregate goal · cross-verification
of template answers.

## 5. RESEARCH — required, and it must be current

Do not design from memory. Research and cite, for at least:

- Natural-language-to-SQL / semantic-layer-mediated querying: what the current
  best architecture is, and what the measured accuracy ceilings are for small
  local models on enterprise schemas.
- Open-ended question answering over structured + unstructured evidence, and how
  production systems route between them.
- Handling non-analytical intents (greeting, help, concept, out-of-scope) in a
  domain tool without letting the model invent capability.
- Provenance and citation UI for high-stakes domains; the post-hoc
  rationalisation failure mode and how to avoid it structurally.
- Forensic/DFIR tooling conventions (AXIOM, Cellebrite, Nuix) for timeline,
  relationship analysis and evidence review.
- Enterprise analyst UX: keyboard-first operation, virtualised dense tables,
  progressive disclosure, accessibility at WCAG 2.2 AA.

State clearly which of your claims come from research, which from this
repository, and which are your judgement.

## 6. WHAT TO DELIVER

**6.1 A reconciliation** covering every item in §3. For each: current state,
evidence for that assessment, the gap, and the risk of leaving it. Separate
**measured fact** from **inference** from **unknown**. An honest "I could not
verify this" is worth more than a confident guess — on this project, guesses have
cost real regressions.

**6.2 A prioritised roadmap.** Phases, each with: goal · exact work · files ·
dependencies · **a measurable exit gate with numbers** · rollback · and the
deployment/activation decision for that stage (what ships, what stays behind a
switch, what needs my approval). **Phase 1 must be runtime data-driven question
answering** — that is what I want working first.

**6.3 The measurement plan.** How we will know each phase worked, on questions
nobody wrote for the system. `evaluation/holdout_questions_v1.json` is the start
of this; extend it. The 62-question golden suite measures regression, not
capability — say so wherever it is used.

**6.4 A named list of everything you found that is missing, half-built, or
duplicated** — including the four ANPR versions, and anything in the media
pipeline that produces output nothing can query.

**6.5 The UI/UX plan** against the binding contract, with the same rigour: what
exists, what the contract requires, and what world-class looks like for a
forensic analyst working this for hours. Modern means keyboard-first, dense,
directly manipulable and fast — not ornament. `docs/ux/NEXUSAI_PRODUCT_UX.md`
§8.3 forbids glassmorphism, neon, gradients, card soup and decorative motion by
name.

## 7. THE STANDARD

The system must answer a real analyst's real question — however they phrase it —
accurately, with its evidence, or tell them honestly why it cannot. Everything
else in this document serves that sentence.

Begin with §3. Show your evidence as you go.
