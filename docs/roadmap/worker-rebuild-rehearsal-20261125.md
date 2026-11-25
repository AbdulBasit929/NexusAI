# MF-0, part 1: rehearsal of a worker rebuild from the repository (2026-11-25)

Programme: `docs/roadmap/master-roadmap-20261124/03_media_first_programme.md`, step MF-0. Background: `docs/roadmap/worker-image-reproduction-20261124.md` (the live worker is the end of a chain of overlays; a worker built from HEAD would not equal it). **Status 2026-11-25: the rehearsal was run on the owner's word ("Yes, run it"); the results are in section 7. Nothing was deployed or changed: the rebuilt image is a new tag (`rebuild-rehearsal-20261125`), the live worker was not touched.** Sections 1 to 6 are the plan as written before the run.

## 1. What the live worker really contains (read-only, 2026-11-25)

`pip freeze` inside `nexusai-forensic-records-worker-1` (image `image-search-20261123f`, 3.68 GB) lists **57 packages**. HEAD's `requirements.txt` pins 19. Against HEAD:

| Difference | HEAD says | The live worker has |
|---|---|---|
| `paddleocr`, `paddlepaddle` | pinned (3.7.0, 3.3.1) | **not installed**: a rebuild from HEAD would add them (hundreds of MB) for a role the live worker does not run |
| `torch` | 2.5.1+cpu | 2.6.0+cpu |
| `torchvision` | 0.20.1+cpu | 0.21.0+cpu (installed by the EasyOCR overlay) |
| `safetensors` | 0.6.2 | 0.5.3 |
| 38 packages not pinned at all | - | `easyocr` 1.7.2 and its dependencies, `opencv-python-headless` 5.0.0.93, `scipy`, `scikit-image`, `shapely`, `pyclipper`, `python-bidi`, `huggingface_hub`, `tokenizers`, `sympy`, ... |

So `docker build` of HEAD's `Dockerfile` today would give a worker with a different torch, no EasyOCR, and without six modules the overlays added (`ocr_router`, `easyocr_ocr_processor`, `onnx_ocr_processor`, `onnx_text_ocr`, `speech_routes`, `media_artifact_scope`, `video_anpr_nexus_v4` and the plate modules) because its file list is older than the code.

## 2. The drafts (new files, nothing replaced)

* `ingestion/forensic_records/requirements.worker-live-20261124.lock`: the 57 exact pins of the live image, with the PyTorch CPU index.
* `ingestion/forensic_records/Dockerfile.rebuild`: installs that lock; copies **every** module of `ingestion/forensic_records` with a pattern (so a new module cannot be forgotten), the two JSON files, the three folders and the contract; checks at build time that every module parses. `Dockerfile` (what compose builds) stays as it is until the rehearsal has passed and the owner says so.

## 3. The rehearsal (when the owner says yes)

A clean context from git (so only committed sources are used; the two draft files are copied in until they are committed), a new tag, nothing moved:

```bash
CTX=/c/tmp/worker-rebuild-ctx; rm -rf "$CTX"; mkdir -p "$CTX"
git archive HEAD ingestion/forensic_records api/forensic_records/contracts | tar -x -C "$CTX"
cp ingestion/forensic_records/Dockerfile.rebuild ingestion/forensic_records/requirements.worker-live-20261124.lock "$CTX/ingestion/forensic_records/"
docker build -f "$(cygpath -m "$CTX")/ingestion/forensic_records/Dockerfile.rebuild" -t nexusai-forensic-records-worker:rebuild-rehearsal-20261125 "$(cygpath -m "$CTX")"
```

Then, with no network and no change to the stack:

```bash
T=nexusai-forensic-records-worker:rebuild-rehearsal-20261125
docker run --rm --entrypoint python $T -m pip freeze > rebuilt-freeze.txt          # P1
docker run --rm --network none --entrypoint python $T -B -c "import worker, media_pipeline, image_search, ocr_router, speech_routes, plate_processor_nexus; print('ok')"   # P2
docker create --name nx-rebuild-rehearsal $T
python tools/media_data/worker_image_manifest.py compare nx-rebuild-rehearsal      # P3 (read-only)
python tools/media_data/worker_image_manifest.py check nx-rebuild-rehearsal docs/roadmap/worker-image-live-manifest-20261124.json
docker rm nx-rebuild-rehearsal
```

**Pass conditions (written before the run):**

| # | Condition |
|---|---|
| P1 | `pip freeze` of the rebuilt image equals the live list: 57 of 57 packages and versions |
| P2 | The import smoke test prints `ok` with no network |
| P3 | The files that differ from the live manifest are exactly those already documented (`media_pipeline.py`, `worker.py`, `typed_ingest.py`, `roman_urdu.py`, `requirements.txt`, the three image files, plus the files only HEAD has); no other file differs or is missing |
| P4 | The image size is within 10% of the live worker's 3.68 GB |

A pass proves the repository can rebuild a worker with the same packages and the current code. It does **not** prove HEAD's worker code is safe to run against the live database: HEAD's `worker.py` writes fields (a `media_result_states` column, `typed_headers`, route-plan roles) that other sessions added and that have not been rehearsed as a set with the live schema. That is a separate rehearsal (all migrations, the typed and media proofs) before any change of the live worker.

## 4. What the rehearsal downloads (an estimate; the build log gives the exact sizes)

The base image `python:3.11-slim` (189 MB) is already on this machine. The build fetches, from Debian mirrors and from PyPI and `download.pytorch.org`: the apt packages `ffmpeg`, `poppler-utils`, `tesseract-ocr` with their dependencies (about 300 MB, estimate) and the 57 wheels of the lock, of which the large ones are `torch` 2.6.0+cpu (about 200 MB), `openvino` and `onnxruntime-openvino`, `opencv-python-headless`, `scipy`, `scikit-image` and `easyocr`. **About 1 GB in all (an estimate from the sizes of these wheels as I know them; not measured).** Disk is not a constraint (Docker holds 58 GB of images and a 95 GB build cache on a drive with more than 1 TB free). It is one heavy job: about 15 to 40 minutes, run alone, with the language model and the workers left running. The build's memory use has not been measured; I will check free memory first (the rule is more than 3 GB free before any build) and watch the Docker VM while it runs.

## 5. What I need from you

1. **Your word for the downloads** above (about 1 GB from Debian, PyPI and PyTorch's wheel index) and for the heavy slot.
2. A disk and your word for `docker save` of the live API and worker images (the only copy today); the Docker data drive has the room, but a copy on the same machine does not protect against losing the machine. Not done.
3. Your word to move the 60 `imgsearch-*` photographs out of the reference case. Not done.

## 6. Still to do in MF-0 (not started)

The one reading contract (tier, model, version, input hash, measured error, candidate label, shown the same way in every viewer and every Ask answer) and the moments view over the typed views. Both are software work with no heavy job and no download; they come after this rehearsal has a yes or a no.

## 7. Results (run 2026-11-25, 04:49 to 05:01 UTC; the build took 12.7 minutes)

Run from a clean `git archive HEAD` (commit `5eb0e37`, 184 files) with `Dockerfile.rebuild` into the new tag `nexusai-forensic-records-worker:rebuild-rehearsal-20261125` (image `d3ae3045581a`). The lowest memory available inside the Docker VM during the build was 2,401 MB (a watchdog would have stopped the build below 450 MB; it did not need to). The build downloaded the Debian packages and the 57 wheels; the rebuilt image was also started once with `--network none` for the import test.

| # | Condition | Result |
|---|---|---|
| P1 | `pip freeze` of the rebuilt image equals the live list | **MET**: 57 of 57 packages and versions identical, none missing, none extra |
| P2 | the import smoke test prints `ok` with no network | **MET** (`worker`, `media_pipeline`, `image_search`, `ocr_router`, `speech_routes`, `plate_processor_nexus`) |
| P3 | the files that differ from the live manifest are exactly the documented ones | **NOT MET as written, by one file.** With line endings normalised, 23 of 32 live files are identical to the rebuilt ones; 8 differ in content: the 7 documented (`media_pipeline.py`, `worker.py`, `typed_ingest.py`, `roman_urdu.py`, `image_region.py`, `image_search.py`, `image_search_service.py`) **and `forensic-platform-v1.json`, which the plan did not list**: HEAD's contract catalogue is newer than the one baked into the live worker (catalog_version `2026-08-28.nxb1.1` against `2026-08-23.post-bfa-source.1`; HEAD adds `financial.transaction_summary`, `access.failed_events` and changes the image operations list and an implementation status). Also as documented: `requirements.txt` exists only in the live image (the rebuild installs from the lock instead), and six files exist only in the rebuild (`plate_reader.py`, `processor_readiness.py`, `reference_loader.py`, `video_anpr_product_v21.py`, `video_anpr_product_v22.py`, `requirements.lock`). Compared with HEAD itself the rebuilt `/app` is identical (35 files, line endings ignored; the 2 extras are the contract and the lock) |
| P4 | the image size is within 10% of the live worker's | **MET**: 3,678,690,393 bytes against 3,683,749,875 (-0.14%) |

**Reading.** The repository can now rebuild a worker with exactly the same packages as the live one and a clean, complete set of the current modules, in about 13 minutes with the downloads. It does not yet equal the live worker, for two reasons that were known (seven modules differ in both directions, listed above) and one that is new (the contract catalogue). Whether HEAD's seven modules are safe against the live database is still the separate rehearsal named in section 3; this result does not change that.

**Findings that changed tools or notes:**
* The raw-byte `check` of `tools/media_data/worker_image_manifest.py` reported 33 differing files, almost all because the live files carry Windows line endings and a `git archive` has Unix ones. The manifest now also stores `sha256_lf` (the hash after line endings are normalised) and `check` compares that when it is present; the live manifest was regenerated from the same live image (`40f9bbfbd3cd`; every raw hash is unchanged, only the new field was added). The check on the rebuilt image now lists exactly the 8 content differences, the 6 extras and `requirements.txt` as missing, nothing else.
* `Dockerfile.rebuild` has no `requirements.txt` (not needed) and runs as `USER nobody`; pip prints a harmless warning about its cache directory at that user.

The rebuilt image stays on this machine as a reference (`docker rmi nexusai-forensic-records-worker:rebuild-rehearsal-20261125` removes it; it shares layers with nothing live). To move to a rebuilt worker, the owner's word is needed after the full rehearsal of HEAD's modules against the live schema.
