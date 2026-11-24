# Image search (M1): hand-off contract for the Ask engine, the interface and the deploy (2026-11-20)

Status: **built and measured on throwaway databases and public data; not deployed, not switched on.** Evidence and numbers: `docs/data/image-search-m1-results-20261120.md`. Gates and amendments: `docs/data/image-search-m1-preregistration-20261120.md`. Plan: `docs/roadmap/media-modules-reconciled-plan-20261120.md`.

## 1. What the module is

Inside one case an analyst can ask four things of the images:

| Function | Kind | How it is answered | What the answer may claim |
|---|---|---|---|
| Find copies of this image (a stored image, or a file the analyst drops in) | `copies` / `copies_of_upload` | three tiers, each image shown under the strongest it reached: **identical file** (same SHA-256), **the same image, re-saved or resized** (256-bit perceptual hash within a calibrated distance), **the same photograph, edited or cropped** (DINOv2 vector within a calibrated cosine distance) | completeness is claimed **against the stated distances** and only for images that were processed (the coverage line says how many) |
| Find photos by words | `by_words` | the phrase (translated to English by the language model first if it is not in Latin script) is turned into a SigLIP 2 vector and compared with every image vector of the case; ranked | **ranked candidates only**; never "all photos of X" (a description has no sharp edge) |
| More like this image | `more_like_this` | SigLIP 2 neighbours of a stored image, ranked | ranked candidates only; accuracy beyond near-duplicates is **not measured** |
| (later) seen in another case | not built | needs the D41 visibility rules, built with module M2 | - |

Every answer carries: `coverage` (images in the case, indexed, failed, identical re-uploads, and a sentence), `limitations` (sentences to be shown), `search_id` (the row in `forensic.image_search_log`), `results` (no raw vector, ever), and for `copies` the count of images that fell **just outside** the match distance.

## 2. API (all inside the authorised collection; the existing scope headers apply)

`GET /images/search?collection_id=<c>[&case_id=<k>]&kind=copies&evidence_id=<uuid>`
`GET /images/search?collection_id=<c>[&case_id=<k>]&kind=more_like_this&evidence_id=<uuid>[&top_k=20]`
`GET /images/search?collection_id=<c>[&case_id=<k>]&kind=by_words&q=<phrase, up to 512 characters>[&lang=ur][&top_k=20]` (`top_k` 1 to 100)
`POST /images/search?collection_id=<c>[&case_id=<k>]` with a multipart field `image` (at most 32 MB): `copies_of_upload`

Results: `evidence_id`, `original_filename`, `content_type`, `case_id`, `tier` (`identical_file`, `same_image`, `same_photograph`, `ranked`), `tier_label` (the sentence to show), `rank`, `hash_distance_bits`, `cosine_distance`, `similarity`, `citation_ref` (`nexusai://evidence/<id>/artifacts/<artifact>`), `content_path` (`/evidence/<id>/content`, the image itself, for the thumbnail).

Errors: 400 bad parameter or not an image; 403 a collection/tenant/case other than the one authorised; 404 the probe image is not in this case; 409 the probe has not been processed yet; 503 the image-search service, the language model (for a non-Latin phrase) or the database is not available.

## 3. For the Ask engine session (the Ask files are theirs; nothing here edits them)

* The engine decides **at run time, by the language model**, that a question is about photographs (no keyword template; owner's direction 2026-11-19) and calls `by_words` with the phrase it extracted, or `copies` with an evidence id it resolved from the question.
* Always say the coverage sentence in the answer (it is why "no results" can be trusted or not). For `by_words` and `more_like_this` say "most likely matches", never "all".
* Cite each image by `citation_ref`; show `tier_label` as written. A `same_photograph` or `ranked` result is a candidate for the analyst to look at.
* Do not use these results to name a person, a place or an event. Faces are a separate, governed module (M2).
* The functions are plain SQL (`forensic.image_index_state`, `image_search_hash`, `image_search_vector`, `image_exact_matches`) for any other caller; the vector functions need a probe vector, which the engine cannot write itself, so the HTTP endpoint is the contract.

## 4. For the interface (design first, the owner's go before any build; UI rules of 2026-11-03/05)

Where it lives: the **Evidence** section. Three entry points, one result view:
1. On any image: **Find copies** (and **More like this**). Result: a grid of thumbnails grouped under the three labels, each with its distance in words ("3 of 256 hash bits differ", "very close"), the coverage sentence on top, and "N more came close but were not counted" at the end.
2. In the Evidence search box, a switch **Photos**: type words (any language); the translated English phrase is shown under the box. Result: a ranked grid with similarity shown as a bar, headed "most likely matches".
3. Drop a file on the search area: **Find copies of this file** (POST). The file is not kept in the case.
Show the limitations sentences under the grid. Empty states: "no processed images yet" (coverage 0) must read differently from "none found". Everything is a candidate; no confirm-as-duplicate button until the review events of migration 033 exist.

## 5. Deploy checklist (each step needs the owner's word; none has been done)

1. Backup, then `ops/038_image_search/preflight.sql`, `038_image_search.sql`, `verify.sql` on the retained database (rehearsed on a restored copy with `Rehearse-038.ps1`: preflight, apply, verify, smoke, re-apply, gated rollback, re-apply all pass).
2. `tools/media_data/Install-ImageSearchModels.ps1` (copies the two models under `models/media/image-search/` after checking their SHA-256).
3. Rebuild the worker image (the Dockerfile now copies `image_search.py` and `image_search_service.py`) and the API image; add `docker-compose.forensic-records.image-search.yaml` AFTER the files of the running stack; set `FORENSIC_IMAGE_SEARCH_SERVICE_KEY`; restart the workspace afterwards.
4. Switch `FORENSIC_IMAGE_SEARCH_ENABLED=true` for the worker (new uploads only); backfill older images by reprocessing them (the coverage line shows progress).
5. Watch the worker's memory with the EasyOCR role loaded (cap 3600m) before running both; measured figures are in the results note.
Rollback: switch the flag off (nothing else changes), drop the overlay, `ops/038_image_search/rollback.sql` with its setting (drops the two tables; vectors in `content_embedding` stay and are harmless).
