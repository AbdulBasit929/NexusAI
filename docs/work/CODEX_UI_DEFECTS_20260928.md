# CODEX — two Investigate defects found in a live demo check, 2026-09-28

Found while running verified questions through `http://localhost:4181` in case
`nexusai-multimodal-product-acceptance`. In both cases the API response is correct and the UI renders
it wrongly. Fix in `apps/investigation-workspace` only.

## 1. A correct document answer is shown as "No verified answer is available" — HIGH (demo blocker)

**Ask:** *Which document mentions contact number 03001234567?*

**The UI shows:** "Analysis complete · SUPPORTED FINDING · **0 SOURCES** · No verified answer is
available. This statement did not include an openable source location." It then shows the correct
passage in the result table, and "No openable source locator accompanied this response" under
Citations.

**The API returned** (verified with a direct `POST /query/hybrid`):

    enterprise.executive_answer        "nexusai-multimodal-acceptance-brief.pdf contains 1 cited passage
                                        matching \"contact number 03001234567\". First matching passage: ..."
    enterprise.narrative.direct_answer (same sentence)
    enterprise.status                  answered_with_limitations
    enterprise.result_state            results_present
    enterprise.fact_packet.citations[0] citation_id C1, source_file nexusai-multimodal-acceptance-brief.pdf,
                                        locator { page: 1, passage: 1, evidence_id, source_file ... },
                                        keys: citation_id, completeness, evidence_id, locator, proof_role,
                                        source_file, version_id
    enterprise.provenance[0]           artifact_type forensics.document-native-text-passage/v1,
                                        citation nexusai://evidence/.../artifacts/..., citation_locator {...}

Showing "No verified answer is available" over a correct, cited answer is a false statement about the
evidence. The answer and citation C1 (page 1, passage 1) must render. A page + passage locator is an
openable locator (UX contract §4.2).

Audio and image-text answers in the same session rendered correctly, so compare the path that reads
their citations with the one used for `document_search`.

## 2. A refusal is labelled "SUPPORTED FINDING"

**Ask:** *Who are the people in the images?*

**The UI shows:** "Analysis unavailable · **SUPPORTED FINDING** · 0 SOURCES · This question is outside
the evidence analysis currently available for this case." The refusal is correct, but the label
claims a finding. Unsupported, clarify and withheld states must never carry a "finding" label (see
`CODEX_UI_ELEVATION_BRIEF_20260928.md` §4.1).

## Backend items found in the same check (Claude's, not yours)

- The result table header for a COUNT reads **"M1"** (and "Sightings" for plate reads). The API's
  `enterprise.data_grid.columns` sends `header: "M1"`. The backend will send a proper label.
- Transcript and image-text citations don't carry `source_truth_state`, so the UI can only show
  "Source record" for what is really a model observation. The backend will add it. Until then, don't
  infer it.
