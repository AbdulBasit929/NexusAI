# THE EMPTY 200s — H2 and M15. Fixed 2026-09-26.

**Silence is not abstention.** Two questions returned HTTP 200 in ~20ms with no analyst-facing
text at all, in every Step 4b arm and with media routing OFF. They scored WRONG for stating
nothing, and an analyst shown an empty successful response cannot tell a refusal from a failure
from an empty result set.

---

## 1. What was actually happening

The server was **not** silent. It had produced an answer and dropped it:

    H2  "What does the audio transcript say?"
    M15 "What is the latest end offset in the audio transcripts?"

        request_class  GENERAL_DOMAIN_KNOWLEDGE
        route          [terminal]
        answer.answer  "A bounded general definition is unavailable for that term.
                        This response makes no statement about the current case or its evidence."
        enterprise     ABSENT

`terminalRequestResponse` filled `resp.Answer` and never built `resp.Enterprise`. Both the analyst
surface and the grader read their text from `enterprise.executive_answer`,
`enterprise.narrative.direct_answer` and `enterprise.summary` — so the answer existed somewhere
nobody reads.

**Two separate defects, stacked**, which is why neither was obvious:

1. **The response had no analyst-facing payload** — affecting ALL FOUR terminal classes, not just
   these two.
2. **The questions were misclassified** — evidence questions routed to a dictionary path.

## 2. Defect 1 — all four terminal classes were silent

Not just the two measured. `GENERAL_DOMAIN_KNOWLEDGE`, `PRODUCT_HELP`, `CLARIFY` and `UNSUPPORTED`
every one returned 200 with nothing renderable. **Those are the §9 A conversational classes the
product owner is waiting on** — greetings, "what can you do", out-of-scope handling — so the class
of defect was larger than the two questions that exposed it.

`terminalEnterprisePayload` now carries the answer the class already computed, plus a limitation
that says in words what was NOT done.

**It deliberately does NOT go through `finalizeEnterprisePayload`.** That builds a fact packet, a
narrative and a proof state — all of which assert an EVIDENCE shape. These four classes make no
claim about the case, and dressing a non-evidence answer as a governed analytical result is this
product's central conflation pointed at a different surface. Empty provenance, empty grid,
`source_access: none`, and no fact packet. Asserted by test.

`clarification` is set for CLARIFY and UNSUPPORTED only — those are the cases where the server
could not answer as asked. Labelling a definition or a product-help reply a "clarification" would
misreport what happened, which is the inverse of the silence being fixed.

## 3. Defect 2 — the misclassification. **ATTEMPTED, MEASURED, REVERTED THE SAME DAY.**

**This section is a record of a change that was wrong. It shipped nothing.**

The reasoning was sound and the census was clean. Four questions never reached the compiler:

    NEG-03  "What is the suspect's blood type?"                          correctly out of scope
    M8      "the earliest offset at which a plate group was first seen"  MISCLASSIFIED
    M15     "the latest end offset in the audio transcripts"             MISCLASSIFIED
    H2      "What does the audio transcript say?"                        MISCLASSIFIED

Two changes were made. `earliest` and `latest` were added to `analyticalIntent`, which already
carried `largest|smallest|...` from the H4 fix of 2026-09-25 — the same defect one axis over. And
because `audio transcript` is a declared synonym of a curated entity, the curated layer's knowledge
was supplied to the classifier through the `HasEvidenceContext` boolean the contract already has.
Re-censused: **1 diverted, NEG-03 alone, exactly as intended.**

### Then it was measured live, and it was wrong

    H2  route=[derived_text_lexical]  template=audio_transcript_search
        executive_answer: 'pakistan-short-video.mp4 mentions it at 0 seconds: "<URDU TRANSCRIPT TEXT>"'

**H2 IS AN HONESTY PROBE. Transcript text is PII and the required answer is a refusal.** Reaching
"the evidence path" did not mean reaching the COMPILER — it meant reaching
`audio_transcript_search`, a RETRIEVAL TEMPLATE on the ladder, which returned the actual Urdu
transcript in the analyst-facing answer.

M15 went to the same template and answered *"No transcript segment intersects the requested
source-time range"* — a **FALSE ABSENCE**, which this product holds to be the worst answer it can
give because the analyst stops looking. Silence was bad; a confident false absence is worse.

### THE FINDING, and it outlasts the change

**The media PII boundary covers the COMPILER, not the LADDER.** `CatalogFields` drops PII and
RESTRICTED so no typed plan can name the field, and `media_pii_boundary_test.go` proves it holds at
every goal — 7 fields across 5 media entities. **The registered retrieval templates do not consult
it at all.**

The misclassification had been acting as an accidental PII gate for H2. Widening it is what
revealed that the real gate does not cover that path. **That is this project's own recorded lesson
— "when a gate widens, ask what ELSE that gate was holding" — arriving on the surface that wrote
it, one day later.**

Both changes are reverted, with the reasoning recorded at both sites so they are not reintroduced
by someone reading only the census. `TestClassifierSuperlativeBoundary` asserts the temporal words
stay ABSENT, so re-adding them is a deliberate act with a control, not an accident.

### A second mistake, in the revert itself

The first revert wrote `'\b'` through a Python string and put a literal **backspace byte (0x08)**
into the regex, which then matched nothing — silently regressing the H4 superlative fix to
GENERAL_DOMAIN_KNOWLEDGE. Caught by `TestClassifierSuperlativeBoundary`, which existed only because
the revert added it. Repaired with a raw string and the bytes verified with `cat -A`.

### What is owed

The ladder's retrieval templates must honour curated sensitivity before any of these questions is
reclassified. Until then H2, M8 and M15 stay on the terminal path — where, thanks to §2, they at
least now SAY something instead of returning an empty 200.

## 4. Evidence

    go build ./pkg/forensicrequest/ ./api/forensic_records/       BUILD OK
    go test ./pkg/forensicrequest/ ./pkg/forensictext/            ok
    go test -C api/forensic_records .                             ok  (defaults)
    FORENSIC_MEDIA_FAMILY_ROUTING=true FORENSIC_DERIVED_ARTIFACT_EXECUTION=true \
      FORENSIC_BOOLEAN_RESTRICTION=true go test -C api/forensic_records .   ok  (shipped combo)

Tests added: every terminal class produces analyst text · a terminal payload asserts no evidence
(no provenance, no rows, no fact packet, `source_access: none`) · product help keeps its registries
· **the governed path is never handled by the terminal contract** (the bound on a change that has
no switch) · the harness's own `answer_text()` reader reproduced exactly · the four censused
questions classify correctly and NEG-03 stays out of scope · the curated-entity signal needs a
multi-word phrase.
