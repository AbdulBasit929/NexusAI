# WI-UI-9 — the curation, and the contract ruling you asked for

WI-UI-8 is **accepted**. The viewers, the virtualised table, the command palette,
the question history and the finding rebalance all stand. The before/after on
the finding surface is exactly what was asked for: claim-level attribution that
no longer shreds the sentence.

**Your next item is NOT more UI.** Read §2 before planning anything.

---

## 1. THE CONTRACT CONFLICT — RULED, so you can stop holding it open

You flagged that UX §6.5 requires the lineage panel to show "the model/backend
and version that produced each" derived artifact, while §9 forbids showing an
analyst "a model, an agent, an operation, a template, a processor, a backend".
You were right to flag it rather than guess. **The ruling:**

> **§9 governs CONTROL. §6.5 governs PROVENANCE. Both stand.**
>
> An analyst must never be asked to *choose, configure or reason about* a model
> or backend — no pickers, no thresholds, no engine names in the answer surface,
> the ask surface, navigation or settings. That is §9 and it is absolute.
>
> But a derived artifact's producer is part of **what the evidence is**. An OCR
> line produced by one model at one version is not the same evidence as the same
> line produced by another, and in a courtroom that difference is the whole
> question. Withholding it would make the lineage panel a decoration.

**So: show the producer and version in the lineage panel, as a recorded fact
about that artifact, alongside its confidence. Never anywhere else, and never as
something to act on.** Where the payload does not carry it, say so explicitly —
"producer not recorded" — rather than omitting the row, because a missing
provenance field is itself a finding.

## 2. YOUR NEXT ITEM — WI-LAYER-1, the curation

**It is now the highest-value work available anywhere on this project, and it is
yours.** Brief: [`WI-LAYER-1_CODEX_CURATION_BRIEF.md`](WI-LAYER-1_CODEX_CURATION_BRIEF.md)
· inventory: [`CURATION_GAP_INVENTORY.md`](CURATION_GAP_INVENTORY.md).

The backend track spent this session fixing the answer path against questions
nobody wrote for the system. Measured on a held-out set (SQL-verified, unseen
phrasings):

    5 CORRECT · 7 WRONG   ->   8 CORRECT · 4 WRONG · 1 CLARIFIED

**Two of the four remaining failures are YOURS, and neither is fixable in code:**

    "what is the average BEAM WIDTH of the towers?"
        -> "the average AZIMUTH is 209.97"   a DIFFERENT FIELD, presented as the answer
    "where was 923001110001 seen?"
        -> answers about the wrong thing; cdr `location` is uncurated on 8,642 rows

The first one matters most for understanding why this is curation and not a
guard. The plan now emits a **correct `AVG`** — over an **undescribed** field.
The operation is right; only a description makes the field choosable correctly.
No verifier can catch it, because nothing in the plan is malformed.

**This was proven the hard way.** Withholding undescribed columns from the
issued enum was implemented, measured, and REVERTED: it did stop the
substitution, and it cost a different question a false negative ("there are no
tower records in this case") because changing the issued field set changes plans
for questions that already worked. **The enum is not a menu. Describing the
columns is the route; withholding them is not.**

Start with `cdr` — `location`, `Site_Id`, `Lac_Id`, `lat`, `longitude`, each on
~8,640 rows — then `anpr`, `tower_location`, `ipdr`. Follow the brief's field
contract exactly; every key in it was paid for by a measured defect.

## 3. UI follow-ups — after the curation, and smaller than they look

**U1 · The finding is below the fold at 1440.** The ask panel plus the new
Question history panel push "SUPPORTED FINDING" past 700px. The finding is what
the analyst came for. History is a working set, not a headline — consider it
collapsed by default, or in the rail where "Recent questions" already lives.

**U2 · Dead space right of the result.** The citations rail ends around 1300px
and the canvas below it is empty while the result table is boxed at ~840px. UX
§11 at 1440: "bounded readable widths **plus analytical canvases**."

**U3 · The ask panel is still the heaviest element on the page.** It was D2 in
WI-UI-8 and it has improved, but a large tinted block still outweighs the
finding beneath it.

**U4 · Lineage producer row** — per the ruling in §1.

## 4. BLOCKED ON ENDPOINTS — do not re-attempt, and do not simulate

Your report named these correctly. Recording them so they are not rediscovered:
no login/session endpoint · no cursor-paginated full structured-record endpoint
· no guaranteed normalized document-page, diarized-transcript, image-region or
video-event endpoint · no case entity or server-side pin/history persistence.

**Leaving these absent and explicitly unavailable was the right call.** Keep
doing that. An invented row is a forensic defect, not a placeholder.

## 5. CONSTRAINTS — unchanged

Never invent evidence, chronology, a locator, a label for an uncurated value, or
a producer that is not recorded. Zero / unavailable / unprocessed stay distinct.
Every state uses icon + label + description. CNIC masking is server-side at
projection; the UI never unmasks. The browser holds no credential. Motion
≤200 ms, respects `prefers-reduced-motion`. **Do not touch `api/**`.**

## 6. REPORT BACK

For the curation: per family, columns curated, columns deliberately excluded and
why, and the before/after coverage from
`go test -C api/forensic_records -run TestSemanticLayerCoverage -v .` (needs
`FAMILY_AUDIT_DB`). Target: no family below 90% on the demo collection.

**Name anything whose meaning you cannot recover from the data.** An honest "I
do not know what `TAC` holds here" is worth more than a guessed description — a
wrong description is exactly how the beam-width answer happened.
