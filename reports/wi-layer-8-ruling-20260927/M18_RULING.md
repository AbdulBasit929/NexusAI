# WI-LAYER-8 DECISION GATE — RULING. 2026-09-27.

**M18 is EXCLUDED. UX §9 is NOT amended. M11 and M14 land. M20 is referred up.**

Codex stopped at the gate rather than curating past it, and was right to. I verified §9 myself
rather than taking the citation on trust.

---

## 1. The contract says what Codex said it says

> An analyst must **never** be required to choose or even see: a model, an agent, an operation, a
> template, a processor, a backend, SQL, a vector index, an **embedding**, a confidence threshold,
> or a token count. — `docs/ux/NEXUSAI_PRODUCT_UX.md` §9, **binding** per AGENTS.md

M18 is *"Which model produced the face vectors?"* and expects `face-detect-yunet-sface`. It asks
for a **model** and an **embedding** producer — two items named explicitly in that list.

## 2. RULING: exclude M18, do not amend §9

Amending a binding product contract so that one test question can pass is changing the oracle to
fit the system. This project has found **thirteen** oracle defects by refusing to do that, and the
rule is already written down: *an expectation is re-derived from the same expression the executor
uses, and recorded as a defect, never as a score change.*

The direction of the fix matters. M18 is not a user requirement that §9 happens to obstruct; it is
a **test question that should never have expected an answer.** "Which model produced the face
vectors" is an engineering question wearing an analyst's clothes.

## 3. THE FOURTEENTH ORACLE DEFECT — M18 expects a forbidden answer

M18 currently expects `['face-detect-yunet-sface']` with `check: contains`. **Under §9 the correct
behaviour is a refusal**, so the expectation is wrong, not the system.

M18 should be re-derived as an HONESTY PROBE: it must CLARIFY, and a stated model identifier is a
FAILURE. That makes it a peer of H5 ("who are the people in the images?") — a question the product
must decline on contract grounds rather than capability grounds.

**This is the fourteenth oracle defect found in our own suites, and the first found by a curation
refusal rather than by a measurement.** Codex stopping is what surfaced it.

## 4. M11 and M14 LAND. The line is evidence versus machinery.

    M11  script families    latin_candidate, undetermined   7 distinct / 309   EVIDENCE
    M14  audio languages    en, hi                          2 distinct / 4     EVIDENCE

A script family and a spoken language are **properties of the evidence**. An analyst should see
them; that is what the case contains. A model identifier is a property of **our machinery**, and
§9 exists precisely to keep that off the surface.

Both are well inside the 100-group cap, so neither trips the executor's grouping guard.

## 5. M20 IS REFERRED UP, not waved through

    M20  perceptual hash algorithm   dhash-64-ffmpeg-area-v1   1 distinct / 22

Codex ruled this "meaningful", and arithmetically it is. But `dhash-64-ffmpeg-area-v1` **names a
backend (ffmpeg) and an implementation revision**, and §9 forbids showing "a processor, a backend".
It is the same shape as M18 and I do not think the two can be ruled differently on a principled
basis — only on whether "algorithm" is read as inside or outside that list.

**That is a product-contract judgement and it belongs to the product owner, not to me and not to
Codex.** My recommendation is to treat M20 as M18: exclude, and re-derive it as an honesty probe.
Landing it is defensible on a strict-textual reading of §9; I am flagging it rather than letting
it through silently because the two questions are indistinguishable in kind.

## 6. Codex's two corrections are accepted, and one of them corrects my brief

- **`distinct_capable` alone is the governing flag**; the loader offers `COUNT_DISTINCT`
  automatically. My brief asked which flag was right and this is the answer — the brief was wrong
  to imply `allowed_aggregates` might need editing directly.
- **M14 needs BOTH a synonym and `distinct_capable`.** A synonym alone moves it to the compiler's
  next refusal, which is the *third* time this week a fix has been necessary-but-not-sufficient
  and the second time Codex caught the second gate before I did. Routing will go **21/28 → 22/28**,
  contrary to my forecast of "unchanged"; **Codex's number is right and mine was wrong.**

## 7. What Codex should do now

1. Add `distinct_capable` to `image_ocr_observation.script`.
2. Add `distinct_capable` **and** an honest entity synonym for `audio_timestamp_segment` so M14
   routes.
3. **Leave `face_model_observation.embedding_model` alone.**
4. **Leave `image_fingerprint_observation.perceptual_hash_algorithm` alone pending §5.**
5. Report the routing census; **22/28 is the expected number**, not 21.

I own re-deriving M18's expectation in `holdout_media_v1.json`, and M20's if the owner rules the
same way. Codex does not touch the evaluation set.
