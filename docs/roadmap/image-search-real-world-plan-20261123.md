# Image search for real casework: research, gap analysis and plan (2026-11-23)

Owner direction (2026-11-23): the first version only proved near-duplicates and one phrase. Image search must work for real investigative use: a detail of a picture, the same object in different photographs, similar pictures, any wording, concise answers, best-of-the-best, researched from primary sources.

## 1. What investigators actually do with pictures (sources)

| Practice | Source | What it means for us |
|---|---|---|
| **Stack duplicates**, show a count on each thumbnail, say where every copy was found; filter out known pictures with hash sets | [Griffeye Analyze DI Pro review](https://www.forensicfocus.com/reviews/analyze-di-pro-from-griffeye/), [Magnet Griffeye](https://magnetforensics.com/products/magnet-griffeye) | A whole-case **duplicate and near-duplicate grouping** is the first thing a reviewer wants: it cuts the pile. We have the descriptors; the grouping view was missing. |
| **Similarity search from a selected picture** (colour, shape, contour, texture) | same | One-click "more like this / find copies" on any picture in any result. Missing in the interface. |
| **Categorise** pictures and filter by what they contain (faces, objects) | same | Browse and filter by content. Needs a vocabulary and calibration; planned. |
| **Perceptual hashes find altered copies**; exact hashes miss them | [PHASER, perceptual hashing evaluation, DFRWS 2024](https://dfrws.org/wp-content/uploads/2024/03/PHASER-Perceptual-hashing-algorithms-evaluati_2024_Forensic-Science-Interna.pdf) | Done (PDQ-style hash + DINOv2). |
| **Small objects need local evidence**: whole-picture vectors work for big objects, local features for small ones; multi-scale windows with a frozen encoder; verify candidates geometrically | [Patch-wise retrieval, multi-scale windows, 2025](https://arxiv.org/pdf/2512.12610.pdf); [Philbin et al., CVPR 2007, spatial verification](https://www.microsoft.com/en-us/research/?p=147076) | Done this session: windows at three scales + SIFT/RANSAC verification with the located box. |
| **Caption + keyword + neural retrieval, then a reranker, beat any single method**; a VLM can verify or filter candidates | [Slide retrieval comparison, 2025](https://arxiv.org/pdf/2509.15211); [OCR + VLM verification](https://arxiv.org/pdf/2505.05666) | Planned: dense captions of every picture (text index) and a verification step for the top candidates. |
| **Composed queries** (a picture plus words: "this car, but at night"), **multi-example** and **multi-round feedback** | [Survey of composed image retrieval, 2025](https://arxiv.org/html/2502.18495v2); [multi-round evaluation, 2026](https://arxiv.org/pdf/2604.12904.pdf) | Planned: several attached pictures, "not like this", refine from marked results. |
| **Thresholds must be calibrated on your own labelled queries** | no off-the-shelf rule exists for a calibrated SigLIP gate (searched 2026-11-23) | Done for the words cut-off (section 3); the region and part thresholds are being calibrated the same way. |

## 2. What exists now, and what is missing

| Capability | State |
|---|---|
| Identical file, re-saved, edited copies (SHA-256, PDQ-style hash, DINOv2) | **Done**, calibrated, in Ask and on attach |
| A detail or region of a picture found inside other pictures, with the box where it sits (SIFT + RANSAC) | **Done** (this session); smoke run 32 of 32, full evaluation in section 4 |
| A part of another photograph that looks like the attached picture (34 windows per picture, DINOv2) | **Built**, indexing of the Lahore case running; threshold provisional |
| Photos by words in any language, scored by whole picture and by best window | **Done**; the cut-off is calibrated below |
| Ask understands photo questions by a model at run time (no templates); plates and written words stay with their own steps | **Done**; 28 of 29 routed as expected, the 29th is caught by the server |
| Concise answers, limitations folded away, coverage line, evidence opens as the file | **Done** |
| Whole-case duplicate and near-duplicate grouping | **Next** |
| One-click "more like this", "find copies", "search this region" on any picture | **Next** |
| Honest "nothing found" for things that are not in the case | **Partly**: whole-picture scores cannot say it (section 3); needs a verification step |
| Dense captions + keyword index; verification of top candidates by a vision-language model | **Planned** (CPU cost: seconds per picture) |
| Several attached pictures, picture + words, relevance feedback | **Planned** |
| Picture metadata (EXIF time, camera, location) as filters and in the answer | **Planned** (needs extraction at ingest) |
| Categories to browse by (vehicle, document, screenshot, animal, building, text) | **Planned** |
| Which of these 50 pictures appear in the case (a watch list) | **Planned**, a loop over the attach flow |
| People and faces | Separate governed module (M2); never mixed into this one |

## 3. The words cut-off, measured (16 queries, 63 pictures with a person's caption as ground truth)

* Best single rule: keep pictures whose similarity is at least **0.88 x the best one**: precision 0.97, recall 0.71, F1 0.82. A wider band at **0.75 x** gives precision 0.78, recall 0.79.
  Product setting: "Best matches" at 0.88 x the best, and "Also possible" between 0.75 and 0.88.
* A fixed score floor does not work: the best score for things that are **not in the case** (0.06 to 0.12) overlaps the best score for things that are (0.085 to 0.134). A whole-picture vector always has a nearest neighbour, so a vector score alone cannot say "nothing here". The honest wording is "the closest pictures; none is a strong match" and a verification step (section 5) for a firm no.

## 4. Region evaluation (live, Lahore case)

Smoke run (4 sources x 8 variants): 32 of 32 found at rank 1 with one region hit each. Full run: see `docs/qa/image-search-manual-test-guide-20261123.md` and `<pack>/region-eval-results.json`. Known weak spots: low-texture regions (plain sky, a bicycle frame), heavy annotation over a plain region.

## 5. Investigator-tool parity and research, item by item (decided 2026-11-23)

| Item | Source | State |
|---|---|---|
| Stack duplicates, a count on each thumbnail | Griffeye | **Stacks done** (Ask: "show the duplicate pictures"). **Next:** the count badge and "where each copy was found" on every thumbnail of the Evidence gallery. |
| Similarity search from any selected picture | Griffeye | **Done**: "More like this" and "Find copies" on every picture. |
| Categorise pictures | Griffeye | **Next**: a closed list of categories (vehicle, document or ID, screenshot, animal, building, text-heavy, map, weapon...) scored from the stored vectors, as a choice among competing categories (not one threshold per category, which the measurements show does not work), calibrated on captions, shown as browse chips with counts. |
| Filter out known pictures by hash | Griffeye | **Next**: named sets of known pictures (SHA-256 and perceptual hash only, never the pictures), flagged or hidden in results and the gallery; the same machinery as the watch list. Interoperability with shared lists is a limit: our hash is a PDQ-style re-implementation, not verified byte-compatible with Meta's PDQ, and Microsoft PhotoDNA is licensed to qualifying organisations and not available to us. |
| Face filters | Griffeye | Separate governed module (M2); not mixed into image search. |
| Multi-scale windows with geometric verification for small objects | patch-wise retrieval, Philbin et al. | **Done** (34 windows per picture, SIFT + RANSAC with the box). Windows indexing finishing on the Lahore case. |
| Captions + keywords + reranker; a vision-language model to verify | slide-retrieval study, OCR + VLM | **Parked** (decision 2026-11-23: no GPU). Until then a words search always returns the closest pictures and says so. Written-words search stays on the OCR text route, which already exists. |
| Composed queries (picture + words), multi-picture, multi-round feedback | composed-retrieval survey | **Planned**: mark results relevant or not and refine; several attached pictures; picture + words. |

## 6. Plan, in order

1. **Calibrate** the window route ("a part looks like it") and re-check the words cut-off with windows; raise recall on plain-area pictures (more features, lower contrast level), re-measure.
2. **Draw a box on any picture and find it elsewhere** (no screenshot step). **DONE and live 2026-11-24** (API `image-search-20261123k`, service `image-search-svc-20261123f`; tested on the Lahore case, see Test O of the manual guide).
3. **Watch lists and known sets**: attach many pictures or a zip, get one table; save a set as hashes to flag or hide known pictures.
4. **Duplicate count on every thumbnail** of the Evidence gallery, with the stack one click away.
5. **Categories** to browse by, calibrated.
6. **Picture metadata** (EXIF time, camera, location) as filters and a timeline.
7. **Mark relevant or not, refine**; several pictures; picture + words.
8. **Scale**: an indexed nearest-window query and a grouped duplicate comparison before cases of tens of thousands of pictures (one migration, rehearsed first).
9. Evaluation on real casework pictures under the data rules already agreed; recalibrate every threshold on them.
10. Parked: captions and the vision-language verification (needs a GPU machine).

## 7. Status and the next two steps (noted 2026-11-24, owner's request to keep these for future work)

**Implemented and live on the Lahore case (97 pictures, all with descriptors and window vectors):** copies of a picture you attach (identical, re-saved, edited, cropped, mirrored), a part of a picture found inside other pictures with the outline (193 of 216 probes as "contains it", 215 of 216 by any route, 0 false alarms), draw a box on any stored picture and search for that part, look-alike parts by window, photos by words (ranked, tiers best / also possible), more like this, find copies, duplicate groups of the whole case, all from Ask with a language-model step that reads the question at run time, concise answers with a folded limitations note.

**Not implemented yet (the triage layer and the proofs):** watch lists and known sets, the duplicate count on gallery thumbnails, categories, EXIF filters and timeline, mark relevant / several pictures / picture + words, a firm "not in this case" (needs a vision-language check, parked: no GPU), indexed nearest-window query for tens of thousands of pictures (not measured, by decision), calibration on real casework pictures.

**Next step 1: watch lists and known sets. BUILT 2026-11-24 (gates K1 to K7 and U1 met on the throwaway database and on the Lahore hashes; not applied live, not deployed, awaiting the owner's word): `docs/data/image-known-sets-preregistration-20261124.md`, `docs/data/image-known-sets-results-20261124.md`.** The design below is what was built.
* Named sets of known pictures stored as hashes only (SHA-256 plus the perceptual hash), never the pictures; kinds such as "of interest" and "known harmless".
* Every picture of a case is checked against the sets from the hashes already stored; a match is flagged ("in set X") or hidden from the gallery and results, never deleted; the flag can be switched off.
* Import: a table of hashes (CSV first). Attach many pictures or a zip, get one table, save as a set.
* Probably one new table, so one migration, rehearsed on a throwaway database first and applied live only on the owner's word. Limits to state: our perceptual hash is a PDQ-style re-implementation (not verified byte-compatible with Meta's PDQ); PhotoDNA is not available to us.
* Gate to register before building: a set of N pictures is flagged in 100% of identical and re-saved members, with 0 false flags on the other pictures of the case.

**Next step 2: duplicate count on every gallery thumbnail.**
* A badge such as "3 copies" on each picture of the Evidence gallery, one click to the group; uses the duplicate-groups answer that is already live, no model, no migration.
* Detail to handle: the groups call writes a search-log row each time, so the gallery must call it once per page with a no-log mode or a cache.

Also owed (small): the results group says "Contains your picture" even for a boxed part; it should say "Contains this part".
