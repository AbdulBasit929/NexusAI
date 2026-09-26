# THE LADDER'S PII PATH — found, scoped, guarded. 2026-09-26.

**The PII boundary this product asserts covers the COMPILER, not the LADDER.** `CatalogFields`
drops `sensitivity: PII` and `RESTRICTED`, so no typed plan can name those fields —
`media_pii_boundary_test.go` proves it at every goal, 7 fields across 5 entities. **The registered
retrieval templates never consult it.** `derivedTextEvidenceSQL` reads `observation.raw_text`,
`normalized_text`, `text`, `roman_urdu_text` and `raw_urdu_text` straight out of the JSON.

---

## 1. How it was found, and by whom

**I caused it.** H2 "What does the audio transcript say?" was classified as a dictionary question
and answered "a bounded general definition is unavailable for that term". Fixing that
misclassification — clean census, exactly one question moved, structured suites untouched — sent it
to `audio_transcript_search`, which **returned the actual Urdu transcript text in the
analyst-facing answer.** Reverted the same day.

**The lesson was already written down and I still walked into it: when a gate widens, ask what ELSE
that gate was holding.** A misclassification had been standing in front of a PII path. My census
measured the CLASSIFIER and said nothing about where the reclassified question would LAND.

## 2. The defect is UNTARGETED BROWSE, not search

The first framing — "the ladder ignores curated sensitivity" — was too broad, and the data corrected
it. **AUD-03 returns Urdu transcript text today and that is CORRECT:**

    AUD-03  "Is the number 03001234567 mentioned in any audio?"   expect urdu-english-identifier.wav
            "urdu-english-identifier.wav mentions it at 2.84 seconds: 'کال 03001234567 at 1035'"

The analyst supplied the identifier. **The snippet is the citation that proves the match.** AUD-01,
AUD-02, DOC-02, DOC-03 and IMG-01 all work this way and are CORRECT in the golden 62. Withholding
their snippets would destroy six working answers and protect nothing.

The defect is `derived_text_relevance.go:83`:

    if len(identifiers) == 0 && len(content) == 0 {
        return 0, true // nothing to filter on: a browse request
    }

**No search term means every passage scores as relevant and is returned IN FULL.** The distinction
is SEARCH versus BULK DISCLOSURE, and it is exactly what H2 fails and AUD-03 passes.

## 3. The guard

Censused across all three corpora **before it was written**: exactly **ONE** untargeted question,
H2, and **zero structured questions**.

`FORENSIC_TEXT_BROWSE_GUARD`, **default ON** (`:-true` in compose — and declared there, not left to
the shell, after that near-miss cost an arm of Step 4b). Default-off would ship the bulk-PII path.
The switch exists to take a CONTROL, not to omit the guard.

Per the accepted §9 B ruling it **withholds the text and keeps everything else** — count, files,
languages, timestamps, citation locators — and **says the text was withheld**. Returning fewer
fields silently is the empty-200 defect wearing a different hat: the analyst cannot tell a boundary
from an absence.

Targeting is read from the request, not guessed: an explicit `TextQuery`, a supplied `Target`, a
time window, an identifier or any content term all disqualify it. `derivedTextRequestIsUntargeted`
asks `derivedTextRelevance` itself rather than re-deriving the rule, so there is ONE definition of
"untargeted" and it cannot drift from the filter it guards.

## 4. Evidence — proven on the SERVING path, not just by unit test

**Live probe, same template, same route, opposite outcomes:**

    UNTARGETED (H2 shape)    text_withheld=TRUE    Urdu in answer: NO    Urdu ANYWHERE in payload: NO
    TARGETED  (AUD-03 shape) text_withheld=false   Urdu in answer: YES   (the citation — correct)

The untargeted response has the transcript absent from the **entire payload**, not merely hidden
from the headline.

**Golden 62 with the guard live: 45 CORRECT · 14 CLARIFIED · 2 NOT_STATED · 1 MANUAL · 0 WRONG —
ZERO TRANSITIONS.** All six search answers still CORRECT with their snippets intact.

    go test -C api/forensic_records -count=1 .                          ok  (guard ON, default)
    FORENSIC_TEXT_BROWSE_GUARD=false go test -C api/forensic_records .   ok  (control)
    shipped combo + guard                                                ok

Tests: the census · **the six working searches are never treated as browse** · the untargeted case
IS claimed · any targeting disqualifies it (time window, target, TextQuery) · **withholding removes
the words and keeps source_file, offsets, evidence_id and the citation locator** · the guard
defaults ON and can still be disabled for a control.

## 5. What this does NOT do

It is the §9 B **fallback**, not the answer: "if token-level redaction cannot be proven, withhold
the whole text". H2 now gets a bounded refusal instead of a transcript, which is safe but is not
what an analyst needs.

**The real masking work is owed**, and part of it is a curation question I cannot answer:
`audio_roman_urdu_segment` carries `raw_urdu_text` and `roman_urdu_text` — **the same utterance in
two scripts**. Redacting a phone number in one and not the other discloses it anyway, and whether
the two can be synchronously redacted at all depends on whether the Roman form is a deterministic
transliteration or an independent producer output, and on whether digits appear as Urdu-Indic
numerals in one and ASCII in the other. Asked of Codex, from the DATA, in
[`WI-LAYER-7_CODEX_PROMPT.md`](../../docs/work/WI-LAYER-7_CODEX_PROMPT.md), with "this cannot be
synchronised safely" named as a valid and valuable answer.

**H2 stays on the terminal path until the templates are safe.** The classifier fix is correct and
must not land before the masking does.
