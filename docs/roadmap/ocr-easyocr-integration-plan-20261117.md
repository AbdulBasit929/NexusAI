# Urdu and mixed-text OCR in the worker: integration plan (2026-11-17)

Owner direction, 2026-11-17: implement the best OCR for English, Urdu and mixed text. The bake-off (`docs/data/ocr-bakeoff-v2-preregistration-20261117.md`, results `ocr-bakeoff-v2-results-20261117.md`) found EasyOCR the strongest reader we can run on this CPU for printed Urdu and for English inside Urdu pages, and PaddleOCR PP-OCRv5 still the best for English. This plan puts them in the worker as **one routed OCR role**, off by default, with a licence-clean path that does not depend on the pretrained weights.

## 1. Design in one picture

```
image -> script probe (existing: PP-OCRv5 Arabic recogniser on the longest lines -> Arabic-script share)
          |-- share below T_low (Latin page) ---------> PP-OCRv5 ONNX (as now) -> Tesseract second opinion if not confident
          |-- share at or above T_low (Urdu or mixed) -> EasyOCR reader (new)    -> Tesseract urd+eng second opinion if it fails or reads little
every observation: model, revision, file hashes, route and probe share in the metadata; a model observation a person reviews
```

T_low is the **only new number** and is chosen on development rows by a pre-registered measurement (section 4), not by hand.

## 2. What is added or changed

| Item | Change |
|---|---|
| `ingestion/forensic_records/easyocr_ocr_processor.py` (new) | `EasyOcrProcessor`, same shape as `OnnxPaddleOCRProcessor`: `from_environment()` with **pinned model paths and SHA-256** (nothing downloaded, the same rule as every other model), `process_image(path, **context) -> MediaProcessResult` with one `MediaObservation` per line (bounding box, raw text, confidence, script, direction, identifiers) in the reading order of the bake-off assembler (`easyocr_reader.py`: vertical-overlap line grouping, right to left when most letters are Arabic-script), the image-OCR observation contract unchanged |
| `ocr_router.py` (v5) | a new optional engine and threshold `FORENSIC_OCR_ROUTER_URDU_SHARE` (T_low); the Arabic branch uses EasyOCR when configured and keeps Tesseract `urd+eng` as the fallback and second opinion; `ocr_route` metadata names the engine, the share and why |
| Worker settings | `FORENSIC_OCR_EASYOCR_ENABLED` (default **false**), `FORENSIC_EASYOCR_MODEL_DIR`, `FORENSIC_EASYOCR_DETECTOR_SHA256`, `FORENSIC_EASYOCR_RECOGNIZER_SHA256`, `FORENSIC_EASYOCR_THREADS` (default 4), `FORENSIC_EASYOCR_MAX_SIDE` (cap on the long side of an image, to bound time) |
| Worker image | an **overlay** on the running worker image (the pattern of `speech-routes`): `easyocr==1.7.2` without its dependency list, CPU `torchvision==0.21.0`, `scikit-image`, `python-bidi`, `Shapely`, `pyclipper`, `ninja` (about 40 MB; the recipe is `tools/media_data/Dockerfile.eval-easyocr`); models mounted from the models folder; a rollback tag before the recreate |
| Ledger | MOD-047 to MOD-049 stay "unverified, ship-blocking" until the licence question is answered (`docs/compliance/easyocr-weights-licence-request-20261117.md`) or the own-recogniser route replaces them |

## 3. The licence-clean route (parallel, does not wait for the answer)

Train our own recogniser with EasyOCR's open trainer (`None-VGG-BiLSTM-CTC`) on synthetic Urdu and mixed Urdu and English lines (open fonts, CC0 or public-domain text, the bake-off's degradations), on public data only, in the approved cloud training (D36). Pair it with PP-OCRv5's detector (Apache-2.0, already shipped), so no CRAFT or EasyOCR weights are needed. It is measured on the same sealed sets and gates. If it is as good as the pretrained recogniser, it is the one that ships.

## 4. Measurements before any code is trusted (pre-registered where they decide something)

1. **Router threshold (development rows only).** For the development partitions of the English pages (S1), Urdu pages (S2b), the mixed set (S4) and the receipts, compute the probe's Arabic-script share per image; choose T_low as the value that sends at least 98% of English pages and receipts to the Latin route and at least 90% of the mixed pages and 98% of the Urdu pages to EasyOCR. Written down with the distribution before the acceptance run.
2. **Speed and memory.** EasyOCR with 1, 2, 4 and 8 threads on an A4 page and on a newspaper paragraph: seconds, peak resident memory; a cap on the image size that keeps an A4 page under 20 seconds with a loss of at most 2 points of character error on development rows. The bake-off ran one thread: A4 pages took 40 to 48 s, small images 2 to 12 s.
3. **Product-level acceptance (fresh data, never a sealed row twice).** The generators are run again with a new seed for the English, Urdu and mixed sets, and the unread newspaper rows beyond the first 60 of the sealed partition are used for S3; the reading goes through the **worker's router**, not the lab script, and is scored against the same gates (U1 to U6 of the bake-off plan, with the routed English gate: identifiers within 2 points of today's route).

## 5. Rules

* The role is **off** until the owner switches it on; the first deploy ships it disabled. One role is turned on at a time.
* Every EasyOCR reading is a **model observation, review required**; the limitation sentence says that Urdu line order and low-quality Nastaliq are not certified and that a reading of a table row with Urdu text may return its columns in reverse order (the order effect measured in the bake-off).
* Existing evidence is read again only by a governed reprocess with lineage (the earlier reading is kept); nothing is overwritten.
* A reading that fails or comes back almost empty falls back to Tesseract `urd+eng`, as today; no reading is merged with another.
* Real consented Urdu and mixed documents (forms, statements, ID cards: decision D26) repeat the mixed test on real pages before the role is switched on for a real case; until then the evidence is T1 and T3 (synthetic and public), and says so.

## 6. Order of work

1. Router-threshold and speed measurements (development rows): half a day.
2. `EasyOcrProcessor`, router v5 and unit tests with a stub reader (Urdu page routes to EasyOCR; Latin page to PP-OCRv5; EasyOCR error falls back to Tesseract; a model hash mismatch refuses to start): one day.
3. Overlay image, rehearsal on a restored copy of the database (the staging pattern), **no change to the live worker**: half a day.
4. Product-level acceptance on fresh data through the router: half a day.
5. **Deploy disabled** (the owner's word, with a rollback tag), then **enable** (the owner's word) once the gates pass and the weights question is closed or our own recogniser has replaced them.
6. In parallel: the licence message (owner), the own-recogniser training (needs the approved cloud run and the data generators), the consented real documents (team lead).

## 7. Risks, said plainly

* **Memory.** EasyOCR with its Arabic recogniser holds about 0.3 GB of weights; the peak with a large image is unmeasured (the bake-off container had a 3 GB cap and never hit it). It runs one at a time in the worker, and the image size is capped.
* **Speed.** Urdu pages that take 1.5 seconds now take 10 to 40 seconds. Acceptable in the background queue; it should be visible in the interface as a longer "reading" state.
* **Mixed rows with Urdu in a left-to-right table** come back with the columns reversed; the words and identifiers are right. A layout step (column detection) is a later improvement.
* **Nastaliq** is much better than today (about 20 to 25% character error on real newspaper rows in the first rows read, against 68%), but it is not certified; it stays "review required".
