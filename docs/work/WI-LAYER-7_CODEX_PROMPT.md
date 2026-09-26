# WI-LAYER-7 — the PII surface, and it is a curation question I cannot answer alone

WI-LAYER-6 is **accepted and SHIPPED**. Routing went 7/28 → 15/28 exactly as you reported, I
verified every number myself, and media answering is live in production as of 2026-09-26:

    media 28   5 CORRECT · 7 WRONG   ->   13 CORRECT · 3 WRONG
    golden 62 and held-out 13: IDENTICAL, 0 transitions across 75 structured questions

**Your ten synonyms are in the shipped image.** `plate group` alone claimed three questions. H4's
accepted risk did not materialise — it clarified, so `plate read` stands and nothing was reverted.

---

## 1. Why you are being asked, and what I got wrong first

I found a PII path in the ladder's text-retrieval templates and I want your ruling on the curation
side before I build the rest of it.

I also want to be straight about how I found it: **I caused it.** H2 "What does the audio transcript
say?" was being classified as a dictionary question and answered "a bounded general definition is
unavailable for that term". I fixed that misclassification — a clean census, exactly one question
moved, the structured suites untouched — and the question then reached `audio_transcript_search`,
which **returned the actual Urdu transcript text in the analyst-facing answer.**

Reverted the same day. The lesson is one this project already had written down and I still walked
into: **when a gate widens, ask what ELSE that gate was holding.** A misclassification had been
standing in front of a PII path, and my census measured the classifier rather than where the
reclassified question would land.

## 2. The finding, stated precisely

**The PII boundary covers the COMPILER, not the LADDER.**

`CatalogFields` drops `sensitivity: PII` and `RESTRICTED` outright, so no typed plan can name those
fields. I proved it holds at every goal — `media_pii_boundary_test.go`, 7 fields across 5 entities:

    anpr_model_observation.plate_text          video_anpr_plate_group.plate_text
    image_ocr_observation.raw_text             image_ocr_observation.normalized_text
    audio_timestamp_segment.text               audio_roman_urdu_segment.raw_urdu_text
    audio_roman_urdu_segment.roman_urdu_text

The registered retrieval templates never consult that sensitivity. `derivedTextEvidenceSQL` reads
`observation.raw_text`, `normalized_text`, `text`, `roman_urdu_text` and `raw_urdu_text` straight
out of the JSON.

**Search with a term is NOT the defect and I have not touched it.** AUD-01, AUD-02, AUD-03, DOC-02,
DOC-03 and IMG-01 are CORRECT in the golden 62, and every one shows a snippet — the analyst supplies
an identifier they already hold ("is 03001234567 mentioned in any audio?") and **the snippet is the
citation that proves the match.** Withholding it would destroy six working answers and protect
nothing.

The defect is the **untargeted** case: a question naming no term, no identifier and no time range
returns every passage in full. Censused across all three corpora: **exactly one question, H2, and
zero structured questions.** I have shipped a guard for it (`FORENSIC_TEXT_BROWSE_GUARD`, default
ON) that withholds the text and keeps the count, files, languages, timestamps and locators.

**That guard is a floor, not the answer.** It is the accepted §9 B fallback — "if token-level
redaction cannot be proven, withhold the whole text" — and it means H2 gets a bounded refusal rather
than a transcript. It does not give the analyst what they actually need.

---

## 3. WHAT I NEED FROM YOU

### 3.1 The masking ruling you already made, applied field by field

Your §9 B recommendation was **accepted** and is what ships: a case-scoped stable alias
(`Plate candidate ••••-A7C2`) rather than last-four, because plate strings are LOW ENTROPY so
last-four both reveals too much and collides; typed placeholders (`[phone masked]`) inside OCR and
transcript text; and **synchronised redaction across the Urdu and Roman-Urdu forms**.

What I do not have is the per-field decision. For each of the seven fields above, tell me:

1. **Which masking applies** — stable alias, typed placeholder in-text, or withhold entirely.
2. **What a placeholder must cover.** In `image_ocr_observation.raw_text` and the three transcript
   fields, what classes of token are PII? Phone numbers and CNICs obviously. Plates? Names? Account
   numbers? Addresses? I need the list the redactor implements, and I would rather it be short and
   defensible than complete and unprovable.
3. **Whether the field is safe to show at all once masked.** If a transcript's non-PII remainder is
   still identifying in context, say so and rule "withhold".

### 3.2 THE ONE I CANNOT RULE ON: synchronised Urdu / Roman-Urdu redaction

`audio_roman_urdu_segment` carries `raw_urdu_text` and `roman_urdu_text` — **the same utterance in
two scripts.** Redacting a phone number in one and not the other discloses it anyway.

I cannot verify this myself and will not guess at it:

- Is the Roman-Urdu form a deterministic transliteration of the Urdu, or an independent producer
  output? If independent, a token can appear in one and not the other, and position-based
  synchronisation is unsound.
- Do digits appear in Urdu-Indic numerals (۰۱۲۳۴۵۶۷۸۹) in `raw_urdu_text` and ASCII in
  `roman_urdu_text`? A redactor matching ASCII digits would miss every Urdu-script number.
- Are the two rows linked by an id I can join on, or only by artifact/segment position?

**Please answer this from the DATA, with a read-only query, the way you did the audio shape
finding.** That finding — that `forensics.audio-observation/v1` is not one field shape — is exactly
the kind of thing I need here, and you found it by looking rather than assuming.

### 3.3 A sensitivity audit of the fields the TEMPLATES read

The compiler is safe because it only sees curated fields. The templates read raw JSON paths. So:

**Is there any field the retrieval templates expose that the layer does NOT mark PII but should?**
The SQL reads `observation.language`, `observation_id`, `citation_locator`, `source_file` and the
five text fields. `citation_locator` carries bboxes and time offsets; `source_file` carries original
filenames.

A filename like `urdu-english-identifier.wav` is arguably identifying. I do not think it is, but
**it is a curation judgement and it is yours.** If any of these should be marked, mark them and say
why; if not, say so explicitly so the decision is recorded rather than assumed.

---

## 4. WHAT IS NOT YOURS HERE

- **The redactor itself, the guard, and the template changes** — `api/**`, mine.
- **The classifier.** H2 stays on the terminal path until the templates are safe. Do not try to fix
  it with a synonym; the layer already names `audio transcript` correctly and that is precisely what
  made the question reach the template.
- **Deployment and live evaluation.**

## 5. VERIFY, AND WHERE TO STOP

```bash
go test -C api/forensic_records -count=1 -run TestWI4 -v .
go test -C api/forensic_records -count=1 -run "TestMediaPII|TestTextBrowse|TestTargetedSearches" -v .
FAMILY_AUDIT_DB="postgresql://localrecall:localrecall@localhost:5433/localrecall" \
  go test -C api/forensic_records -count=1 -run TestSemanticLayerCoverage -v .
```

`TestMediaPIIFieldsAreNeverIssuedInTheCatalogue` derives its list FROM THE LAYER, so a field you newly
mark PII is covered by it the day it lands — and a field you UNMARK silently leaves that protection.
**If you change any `sensitivity:` value, say so explicitly in your report**; I will re-run the
boundary tests against it rather than discover it later.

### Stop and report, do not proceed, if

- `api/**` does not compile. Say so **with the wall-clock time** and stop. Never fix it.
- The Urdu/Roman-Urdu relationship turns out not to be reliably synchronisable. **That is a finding,
  not a failure** — it decides between "redact" and "withhold" and I need the honest answer.
- A masking rule you would have to invent rather than derive. Say what you cannot establish.

## 6. Why you are being asked this way

You removed two joins you had been asked to declare because they matched zero rows, and you refused
to flatten `forensics.audio-observation/v1` into one entity when it is two field shapes. Both times
the refusal was worth more than the work. **§3.2 is the same kind of question**, and if the answer is
"this cannot be synchronised safely", saying so is the most valuable thing you can return.
