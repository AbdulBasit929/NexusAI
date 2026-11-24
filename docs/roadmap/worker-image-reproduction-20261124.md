# The live worker image: what it is, how it differs from the repository, and how to reproduce it (2026-11-24)

Written to close the last gap of step A0 (everything live is committed): the repository now holds the worker sources, but **a worker image built from HEAD would not equal the live one**. This note says why, shows exactly what differs, and gives the safe ways to reproduce or to move to HEAD. Nothing was rebuilt, started or changed to write it; the checks only read (`docker cp` out of the running containers, `git show`).

## 1. Verdict

* The live worker is image `nexusai-forensic-records-worker:image-search-20261123f` (id `40f9bbfbd3cd`, also `:latest`). It is **the end of a chain of more than twenty overlay images**, each built `FROM` the previous one by patching a few files, on top of a full build made on 2026-08-23 (`before-ingest-v2`, id `376bbf162777`).
* Its `/app` files are **not** the repository's. Of 32 files, 23 equal HEAD (line endings ignored), 8 differ, 1 exists only in the image (`forensic-platform-v1.json`), and 5 files of HEAD are absent from the image.
* The differences go **both ways**: HEAD has features the live worker lacks, and the live worker has lines HEAD lacks (the overlays patched an older file). So rebuilding from HEAD would both **add** untested-in-this-shape behaviour and **drop** live behaviour.
* The images themselves are the master copy. They exist only as local Docker tags; they are not stored in git and have not been exported.

## 2. What differs from HEAD (measured with `tools/media_data/worker_image_manifest.py compare nexusai-forensic-records-worker-1`)

| File | HEAD has (lines the image lacks) | Image has (lines HEAD lacks) | What the difference is |
|---|---|---|---|
| `media_pipeline.py` | 527 | 117 | The largest drift. HEAD's file includes the wiring of the plates programme (`dfb762c`, behind `FORENSIC_ANPR_ENGINE`) and of the speech lab (`6f3867d`); the live file is an older file patched by the overlays `plates-*`, `video-v4`, `speech-routes-*`, `easyocr-*`. I did not map each of the 527 and 117 differing lines to a commit or an overlay |
| `worker.py` | 23 | 4 | HEAD adds the evidence route plan (`requested_analysis_roles`), a `media_result_states` column in an insert, `typed_headers` on the evidence (`bcad76c`), and a version id; the 4 live-only lines are the older forms of lines HEAD rewrote |
| `typed_ingest.py` | 12 | 0 | HEAD keeps the file's own header list (`bcad76c`, engine round 9e, ingest side) |
| `roman_urdu.py` | 6 | 1 | HEAD adds a Devanagari check and an optional start time |
| `requirements.txt` | 6 | 2 | Different pins |
| `image_search.py` | 15 | 4 | Window vectors computed in small locked slices (the memory fix). **Not used by the worker**: it sends pictures to the search service (`search_processor_from_environment()` returns the remote processor when `FORENSIC_IMAGE_SEARCH_URL` is set, which it is) |
| `image_search_service.py`, `image_region.py` | 26 / 32 | 5 / 5 | The crop endpoint (draw-a-box) and settings added after the worker image was built. The worker does not run the service; the **service container has HEAD's versions** (its four image files equal HEAD exactly) |
| `forensic-platform-v1.json` | - | present | Only in the image |
| `plate_reader.py`, `processor_readiness.py`, `reference_loader.py`, `video_anpr_product_v21.py`, `video_anpr_product_v22.py` | present | absent | In HEAD only |

My own image-search hook lines in `worker.py` and `media_pipeline.py` (`persist_image_search_rows`, `ImageSearchAugmentedProcessor`) are in **both** the image and HEAD (3 occurrences each), so image search itself is the same live and committed.

The search service container (`image-search-svc-20261123f`, id `3eef9ecbb83f`) is the live worker image plus two later layers: its four image files equal HEAD; its other files are the same older ones as the worker's.

A fingerprint of every file of the live worker is committed: `docs/roadmap/worker-image-live-manifest-20261124.json` (SHA-256 and size of the 32 `/app` files). Any rebuilt image can be proven equal to it with `python tools/media_data/worker_image_manifest.py check <container> docs/roadmap/worker-image-live-manifest-20261124.json`.

## 3. The chain (newest first; each `before-X` tag names the image before step X)

| Tag | Id | Built |
|---|---|---|
| `image-search-20261123f` (= `latest`, **live worker**) | `40f9bbfbd3cd` | 2026-11-23 14:38 |
| `image-search-20261123` | `b7527d3371a3` | 2026-11-23 09:29 |
| `easyocr-rows-20261119` | `71f5ca33b9c2` | 2026-11-19 |
| `easyocr-reread-20261117` | `c2010550ebe5` | 2026-11-17 |
| `easyocr-20261117` | `cd29640eee55` | 2026-11-17 |
| `speech-routes-20261113c` / `b` / (none) | `117386968580` / `d728de641290` / `f32bd367b972` | 2026-11-13 |
| `artifact-scope-20261112` | `e70bf7e28b6e` | 2026-11-12 |
| `video-v4-20261111` | `4dcc14ec5901` | 2026-11-11 |
| `plates-20261110e` / `d` | `e2b930e29041` / `e9915ad9b98f` | 2026-11-10 |
| `families-20261030b` / (none) | `47e5e829a3d4` / `71e370c09b4d` | 2026-10-30 |
| three untagged steps (media ASR, media OCR v4, media OCR) | `037b7b720365`, `e7f47e81d59c`, `2911c965c8a0` | 2026-10-21 / 22 |
| `s5-layouts-20261021` | `db43bb68c042` | 2026-10-21 |
| `c4v2-20261020` | `0651ab31e9c1` | 2026-10-20 |
| `seed-layout-20261020` | `40a9a7195685` | 2026-10-20 |
| `ingest-v2-match-20261016` | `b136e8f2d5e1` | 2026-10-16 |
| `ingest-v2-xls-20261013` / `text` / `shadow` | `ac6388c57cbc` / `a3537e19c021` / `5682f75a18a3` | 2026-10-13 |
| `before-ingest-v2` (**base, a full build**) | `376bbf162777` | 2026-08-23 |

The builders are the overlay scripts in the repository: `tools/media_data/Build-WorkerOverlay.ps1`, `Build-PlatesOverlay.ps1`, `Build-VideoV4Overlay.ps1`, `Build-ArtifactScopeOverlay.ps1`, `Build-SpeechRoutesOverlay.ps1`, `Build-EasyOcrOverlay.ps1` and `build_image_search_overlay.py`. The matching of each tag to its script is by name and date; I did not verify it one by one. The repository commit of the 2026-08-23 base is **not recorded** anywhere I could find.

How the two image-search steps were made:

1. `image-search-20261123`: `python tools/media_data/build_image_search_overlay.py --tag image-search-20261123 --base-tag latest`, run when `latest` was `easyocr-rows-20261119`. It reads `worker.py` and `media_pipeline.py` out of the running image, applies the image-search lines (each anchor must match exactly once, so it refuses to run on a different base), and adds `image_search.py` and `image_search_service.py`.
2. `image-search-20261123f` (tiles): a Dockerfile of three `COPY` lines (`image_search.py`, `image_search_service.py`, `image_region.py`) from the repository of that day onto step 1. The build context was not kept; those three files are the versions HEAD had at about 14:00 on 2026-11-23, i.e. before the memory fix and the crop endpoint.

## 4. How to reproduce, safest first

**A. Keep the images (recommended now).** The live image is the only exact copy. Export it so it cannot be lost with the Docker VM: `docker save nexusai-forensic-records-worker:image-search-20261123f | gzip > <folder>\worker-image-search-20261123f.tar.gz` (several gigabytes: it carries the OCR and ONNX stacks; choose the disk) and record its SHA-256 next to it. Not done: it needs the owner's choice of disk and a quiet machine (the Docker VM is short of memory).

**B. Replay the overlays.** Starting from `before-ingest-v2`, run the overlay scripts in the order of section 3, each on the previous tag. Each script refuses to run when its anchors do not match exactly once, so a wrong base is detected, not silently built. Then check the result against the committed manifest (`worker_image_manifest.py check`); it must say EQUAL. Not tested end to end: it needs the base image, the model folders, and several hours.

**C. Build from HEAD (a change, not a reproduction).** Not equal to live; see section 2. Do not do this without:
1. a review by the sessions that own HEAD's `worker.py`, `media_pipeline.py` and `typed_ingest.py` changes (the `media_result_states` insert and `typed_headers` write may need migrations and API support that are not live as one set, and HEAD's `media_pipeline.py` lacks 117 lines the live file has);
2. a rehearsal on a throwaway database with all migrations up to 038 and the typed and media proofs re-run;
3. `worker_image_manifest.py compare` on the rebuilt container, with every difference from live listed and accepted;
4. the owner's word, a rollback tag (`before-<tag>` on the current image), and no in-flight jobs.

**If only the three image files should match HEAD** (harmless for the worker, since it calls the remote service): a three-line overlay `FROM nexusai-forensic-records-worker:image-search-20261123f` with `COPY` of HEAD's `image_search.py`, `image_search_service.py` and `image_region.py`. Not needed for any behaviour; it only removes the drift.

## 5. What I did and did not verify

* Verified (read-only, 2026-11-24): the file-by-file comparison above for the worker and the search service; the layer history of both images; the committed hook lines; that the worker selects the remote processor when the service URL is set; the tag chain from `docker images`.
* Not verified: the commit of the 2026-08-23 base; that replaying the overlay scripts reproduces the manifest; that each script in the list built exactly the tag I matched it to; the effect of HEAD's extra worker lines on a live database.
* Not done: any rebuild, recreate, tag move or export.
