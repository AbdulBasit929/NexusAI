# Licence ledger (roadmap Phase 0, principle P7: licence before download)

Status: started 2026-10-12. `docs/compliance/licence-ledger.csv` lists every library, system package, model, dataset and infrastructure component the stack runs, has configured, or the roadmap plans to fetch, with the licence found, **how well that licence is evidenced**, and whether it blocks shipping. It is an engineering record, not legal advice; counsel decides what the ship-blocking rows need.

## Reading a row

| Column | Meaning |
|---|---|
| `evidence_tier` | **A** the licence was read from the artifact itself (package metadata in the running worker image, a LICENSE file). **B** reported by a project page, a model card, a search or an earlier review in this repository; the file still has to be opened. **C** unknown. |
| `status` | `approved` (tier A and a plainly permissive licence), `conditional` (usable, with the condition stated, or the file still to be read), `blocked` (known restrictive: copyleft that reaches us, non-commercial, research-only), `unverified` (unknown or unresolved). |
| `ship_blocking` | `yes` exactly for `blocked` and `unverified`. Such a component is not to be shipped, enabled or downloaded until the row changes. |
| `usage` | `running` (present in the live runtime), `configured` (named in the compose file or requirements but not present or not enabled), `candidate` (roadmap), `evaluation-only`. |
| `config_ref` | `pkg:<name>` for a package pinned in `ingestion/forensic_records/requirements.txt`, or the compose setting that names the model, detector or OCR engine. The checker uses it. |

## The check

```bash
python -X utf8 tools/licence_ledger/check_ledger.py --summary
python -X utf8 tools/licence_ledger/test_check_ledger.py
```

It fails when a package pinned in `requirements.txt` has no row, when a model, detector or OCR setting added to `docker-compose.forensic-records.yaml` has no row, when a row is `approved` without tier-A evidence or with a licence that is not plainly permissive, when a non-approved row has no condition, or when `ship_blocking` disagrees with the status. 13 tests, each rule also broken on purpose. It cannot tell whether a stated licence is true. A proposal, not yet wired: run it in the pre-commit hook next to the real-data scan.

## What the first pass found (2026-10-12)

75 rows: 15 approved, 29 conditional, 8 blocked, 23 unverified. Read from the running worker image, not from memory: the Python packages. Everything about models and datasets is tier B until someone opens the files.

1. **Three components that run today have unresolved terms** (ship-blocking): the YOLOv9-t plate detector weights, the CCT-XS v2 plate reader weights (an earlier review, `configuration/nxmmr_anpr_ocr_license_admission_v1.json`, found no checkpoint-specific statement for either), and the SFace face recogniser (training-data provenance; decision D5). The packages around them are MIT.
2. **Video plate reading (v2) is configured on an AGPL stack and is off.** `yolov8n.pt`, `license_plate_detector.pt` (weights trained with Ultralytics YOLOv8) and the `ultralytics` package are blocked. None of them is present in the worker image or in `/models/media` today (checked), and `FORENSIC_VIDEO_ANPR_V2_ENABLED` defaults to false. Enabling it needs an enterprise licence, an approved AGPL posture, or a different detector.
3. **The running worker image does not match `requirements.txt`**: `torch` 2.6.0 against the pinned 2.5.1, `safetensors` 0.5.3 against 0.6.0, and `torchvision`, `paddleocr` and `paddlepaddle` are not installed at all. Settle which is intended at the next worker build; the rows say so.
4. **Copyleft obligations on distribution.** `psycopg` (LGPL-3.0), FFmpeg and poppler (LGPL and GPL parts) are used as unmodified libraries or separate executables, which is workable, but distributing an image brings source-offer duties. Conditional, not blocked.
5. **No model file in `/models/media` carries a LICENSE file.** Every model row asks for the licence file and hash of the exact artifact to be recorded.
6. Candidates the roadmap names are entered with what is known; SeamlessM4T, ArcFace/InsightFace weights, AdaFace, EdgeFace, UFPR-ALPR and RodoSol-ALPR are blocked for product use.

## Not covered yet

* Go modules and npm packages (the API and the UI): hundreds of transitive dependencies; a scanner in CI (for example `go-licenses` and `license-checker`) is the right tool, to be added with the owner's approval.
* Base container images, fonts, and licences of libraries bundled inside wheels (the OpenCV wheel bundles FFmpeg and others).
* Whether a tier-B licence statement is true: someone has to open each file.

## Adding a row

Add a line to the CSV with the evidence you actually have (tier A only for something you read), set the status, give a condition unless it is `approved`, and run the check. A new package or model setting fails the check until it has a row, which is the point.
