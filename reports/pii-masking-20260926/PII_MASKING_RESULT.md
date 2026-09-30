# PII MASKING — MEASURED 2026-09-27. **SAFE, AND INERT. DO NOT TURN IT ON YET.**

    A  CONTROL        PII_MASKED_PROJECTION=false
    B  MASKING        PII_MASKED_PROJECTION=true + a 256-bit alias secret
    OUTCOME           every suite IDENTICAL. The predicted movement did not happen.
    RECOMMENDATION    keep the switch OFF until the goal taxonomy lands.

Thresholds: [`THRESHOLD_PREREGISTRATION.md`](THRESHOLD_PREREGISTRATION.md), written before the build.

---

## 1. The numbers

| Suite | control | masking |
|---|---|---|
| golden 62 | 45 CORRECT · 14 CLARIFIED · 2 NOT_STATED · 1 MANUAL | **identical** |
| held-out 13 | 10 CORRECT · 2 CLARIFIED · 1 WRONG | **identical** |
| media 28 | 12 CORRECT · 12 CLARIFIED · 3 WRONG · 1 ROWCOUNT | **identical** |

**Zero transitions in any suite.** Including H1, the one movement that was predicted.

## 2. WHY H1 DID NOT MOVE — and my own census had already said so

H1 "Which plates were read from the videos?" was predicted to go CLARIFIED → CORRECT, answering in
aliases. It did not move, in either arm:

    template = canonical_records      route = [verified_only_withheld]

**H1 never reaches the compiler.** Its goal classifies as `lookup`, the ALGEBRA GOAL GATE refuses
it, and it is answered by the ladder instead. The PII exclusion was never H1's binding constraint.

The census printed this days ago and I generated it myself:

    H1-MEDIA-WHICH-PLATES   GOAL GATE   [anpr_model_observation] matches,
                                        but goal=lookup is not algebraic

I even wrote it into the continuation — *"H1 and H2 now MATCH a media phrase and are held ONLY by
the goal gate"* — and then predicted H1 would move when masking shipped. **I had the information
and did not connect it.** That is the §6 lesson for the third time in this stretch: a correct,
well-tested slice measured at zero because the binding constraint was somewhere else, and the
instrument had already said where.

## 3. The safety bar — and an instrument defect in my own check

**No plate is disclosed through the masked field, in either arm.** That is the property that
matters and it holds.

Getting there took two corrections to my own scanner, both worth recording:

**First version: 40 false positives.** It stripped all separators from both the plate and the
response, then substring-matched. The oracle contains OCR noise — values of 1 to 4 characters —
so tokens like `1111` and `CE61` matched inside UUIDs and hashes in completely unrelated
structured questions (ACC-02, CDR-06, CASE-01). **A PII leak reported on a broken instrument is
worse than no check**, because the next real one is disbelieved.

Corrected: only plausible registrations (≥6 chars, letters AND digits — 105 of 137 values; the
shapes are `AA99AAA` ×62 and `AA99AA` ×19), matched on word boundaries against the text as
written rather than a flattened blob.

**Second version: 6 hits, all explained, none from masking.**

    DOC-01  the analyst supplied the plate in the question    echo, not disclosure
    DOC-02  MN1367 inside a returned DOCUMENT PASSAGE         see below
    DOC-03  MN1367 inside a returned DOCUMENT PASSAGE         see below

All six appear **identically in both arms**, so masking causes none of them. DOC-02 asks *"Which
document mentions contact number 03001234567?"*; the system returns the matching passage, and that
passage — a synthetic acceptance fixture that says of itself "not natural evidence" — contains a
plate among its text.

**This is the accepted targeted-retrieval path, not the masked field.** The WI-LAYER-7 ruling
preserves it explicitly: *"An analyst who supplies a term, identifier or time range may still
receive the bounded matching snippet as cited evidence."* The masked field is
`observation.normalized_plate_text` on ANPR and video artifacts; this is free text inside a PDF.

**Worth recording as its own item:** document and OCR passages can carry a plate incidentally, and
the plate-masking ruling does not reach them because there a plate is free text rather than the
curated field. That is the same shape as the WITHHELD ruling for transcripts, and the same
targeted/untargeted boundary applies.

## 4. My threshold was written as an absolute again

§3.2 said *"No raw plate value may appear anywhere in any response, in either arm."* Read
literally it fires on the DOC passages — which are identical in both arms and predate this work.

**I recorded exactly this lesson one day ago** — *"write a threshold against what the change
CAUSES, not an absolute count"* — and then wrote another absolute. Stating it rather than quietly
applying the causal reading, because the whole value of a pre-registered threshold is that it is
not reinterpreted after the fact.

Under the causal reading the bar is met: masking introduces no disclosure. Under the literal
reading it fires on a pre-existing path. **Both are stated; the ship decision is the product
owner's, and it is moot for now because of §5.**

## 5. RECOMMENDATION: leave it OFF

Not because it is unsafe — it is safe, and the offline and database tests prove the alias holds.
**Because it currently delivers nothing.** Every question that would use a masked plate is blocked
upstream by the algebra goal gate, so turning the switch on ships an unmeasurable change and adds
a live PII surface for no capability.

**The order should be: goal taxonomy first, then re-measure masking.** The taxonomy work is already
queued and already known to be worth ~7 media questions; it is what makes H1, M8, M11, M14, M18 and
M20 reachable at all. Masking becomes measurable the moment it lands, and then this measurement is
worth repeating with the same thresholds — rewritten causally.

## 6. What is kept

The implementation, the fail-closed secret handling, the scheme whitelist that keeps
`subscriber.cnic` out, the non-sortable rule, and both test suites — all committed and green in
four switch configurations. Nothing is reverted; the switch simply stays off.

**Also kept: a harness finding.** An earlier pair of runs overlapped after a session boundary, so
one measurement was taken while two evaluations competed for one CPU-bound model. That run was
discarded and re-taken in a quiet state rather than reasoned about. Verdicts proved identical
between the disturbed and clean runs, so on this evidence concurrency costs latency and not
correctness — but the clean run is the one scored.
