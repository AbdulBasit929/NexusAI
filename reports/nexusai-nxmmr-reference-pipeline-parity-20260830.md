# NX-MMR reference pipeline parity and baseline recovery

Status: `REFERENCE_PIPELINE_MATERIALLY_BETTER_FOR_VIDEO_IMAGE_PARITY_EXACT`.

This is the bounded NX-MMR reference-parity slice requested after
`CURRENT_NXMMR_BASELINE_V1`. It does not rewrite the locked baseline, use model
output as truth, download a model/dataset, fine-tune, deploy, process retained
evidence or start NX-B2.1.

## Verified Starting State

- Independent gold remains locked at
  `88c6827c23fbed467f097843404ed09c947bf7f27432e5cb5fa8427be9f48567`.
- `CURRENT_NXMMR_BASELINE_V1` remains immutable: image development 5/10 exact,
  image holdout 13/22 exact, video 3/41 exact.
- The five live NexusAI containers were healthy before the slice and were not
  recreated, restarted or reconfigured.
- Host free RAM was 3.765 GiB before the final reference runs. This admitted
  the one-model image reproduction under the 3.5 GiB MEDIUM floor but blocked
  a full dual-YOLO-plus-EasyOCR video rerun under the 4.5 GiB HEAVY floor.

## Reference Directories

Both references were inspected and used read-only:

- `C:\Users\sheik\Workspace\Personal\FastPlateOCR`
- `C:\Users\sheik\Workspace\Personal\Vehicle-License-Plate-Detection\src`

New receipts and diagnostics were written only below the Git-ignored NexusAI
`local-acceptance-models/nxmmr/private-benchmarks/reference-parity` directory.

Private receipt SHA-256 values:

- image reference development:
  `35a3b6240f049e018eec1854eb3f16a2f6cd568813a7fb757d3747b747e4b573`;
- image reference sealed holdout:
  `5b1544dfcf58d78eaf37847b4415f37ad14c10cd004347ca2fca53518bb3cdec`;
- historical video output score:
  `f1732b7b2d87c4e845f67d1413ecd2465514c2cf7a50bae70781b9091fd9689f`.

## Reference Source Hashes

| Reference file | Modified UTC | Bytes | SHA-256 |
|---|---|---:|---|
| FastPlateOCR `batch_alpr.py` | 2026-08-17T09:10:18Z | 11,459 | `6a700f949cf408e2a1ebd8d7b737317d0432e19e91958ddc8c6c3bda4ffc920b` |
| `evaluate_results.py` | 2026-08-17T09:51:24Z | 1,204 | `35ac0dce2c73783f54b77b62a611afc6114567ba3fb1d7c72f87371709eaa391` |
| `test_alpr.py` | 2026-08-17T08:46:22Z | 308 | `5738ea2fe6e78ac44c148850b6777db60390e84b10dd28324ef80132deb37e28` |
| `test_alpr_draw.py` | 2026-08-17T08:47:47Z | 912 | `2085186104af483fe96775f7d3a3411a8e8a6512b2d272ab68f6419d2421eb14` |
| `test_ocr.py` | 2026-08-17T08:36:34Z | 214 | `3aa492c53948d946031907a1f76f62ccfd87c033a769543197d5774a06037c0c` |
| video `main.py` | 2026-08-17T04:36:21Z | 2,448 | `e4e6ad1e7ba0ff100e3cc508b05d93a1f95f03f5bc3d1d50c09807c2b1f91a65` |
| video `util.py` | 2026-08-17T04:36:21Z | 6,374 | `de12e427d925188b9f9fb75903588047a5adc8c4c3eebc20ec6472b0c3477f5d` |
| video `visualize.py` | 2026-08-17T04:36:21Z | 4,779 | `929765446021fdacbeccfa8d69455790e5347ab94c22251d9eadba4028034ee1` |
| video `add_missing_data.py` | 2026-08-17T04:36:21Z | 4,563 | `504d54eacf7baf5fb14ce58894cb3446e4434af2f89eb6b14d187c2b1a8a3977` |
| video `sort/sort.py` | 2026-08-17T04:37:12Z | 12,069 | `41d30e3c3cf94079fad57aa67152704a73837cce27257be2890554b5921652b9` |
| video `requirements.txt` | 2026-08-17T04:36:21Z | 76 | `764045032580aafc587473254c5a47b07f0768b3f1824dc999a25ede54de1d0d` |

## Environment Inventory

### FastPlateOCR reference

- interpreter: Python 3.13.14, Windows 11;
- virtual environment: `FastPlateOCR\.venv`;
- available ONNX providers: OpenVINO and CPU;
- actual reproduction providers: CPU for detector and OCR because the local
  OpenVINO provider could not load `openvino.dll` and failed over explicitly;
- frozen package inventory:

```text
colorama==0.4.6
fast-alpr==0.4.0
fast-plate-ocr==1.1.0
flatbuffers==25.12.19
markdown-it-py==4.2.0
mdurl==0.1.2
mpmath==1.3.0
numpy==2.3.5
onnxruntime-openvino==1.24.1
open-image-models==0.6.0
opencv-python-headless==5.0.0.93
openvino==2025.4.1
openvino-telemetry==2025.2.0
packaging==26.3
pip==26.2.1
protobuf==7.35.1
Pygments==2.20.0
PyYAML==6.0.3
rich==15.0.0
setuptools==84.0.0
sympy==1.14.0
tqdm==4.70.0
wheel==0.48.0
```

The NX-MMR worker uses Python 3.11.15/Linux but exactly matches the relevant
FastALPR 0.4.0, FastPlateOCR 1.1.0, OpenImageModels 0.6.0,
onnxruntime-openvino 1.24.1, OpenVINO 2025.4.1, OpenCV 5.0.0.93 and NumPy 2.3.5
versions. It requested OpenVINO then CPU.

### Vehicle-License-Plate-Detection reference

- interpreter: Python 3.10.11, Windows 11, CPU only;
- virtual environment: `Vehicle-License-Plate-Detection\.venv`;
- PyTorch threads: 12; CUDA unavailable;
- frozen package inventory:

```text
certifi==2026.7.22
charset-normalizer==3.5.1
colorama==0.4.6
contourpy==1.3.2
cycler==0.12.1
easyocr==1.7.2
filelock==3.32.3
filterpy==1.4.5
fonttools==4.63.0
fsspec==2026.7.0
idna==3.18
ImageIO==2.37.4
Jinja2==3.1.6
kiwisolver==1.5.0
lazy-loader==0.5
MarkupSafe==3.0.3
matplotlib==3.10.9
mpmath==1.3.0
networkx==3.4.2
ninja==1.13.0
numpy==2.2.6
opencv-python==5.0.0.93
opencv-python-headless==5.0.0.93
packaging==26.3
pandas==2.3.3
pillow==12.3.0
pip==26.2.1
psutil==7.2.2
pyclipper==1.4.0
pyparsing==3.3.2
python-bidi==0.6.11
python-dateutil==2.9.0.post0
pytz==2026.3.post1
PyYAML==6.0.3
requests==2.34.2
scikit-image==0.25.2
scipy==1.15.3
seaborn==0.13.2
setuptools==80.10.2
shapely==2.1.2
six==1.17.0
sympy==1.13.1
tifffile==2025.5.10
torch==2.5.1+cpu
torchvision==0.20.1+cpu
tqdm==4.70.0
typing_extensions==4.16.0
tzdata==2026.3.post1
ultralytics==8.0.114
urllib3==2.7.0
wheel==0.48.0
```

## Model Inventory

| Role | Bytes | SHA-256 | Runtime details |
|---|---:|---|---|
| FastALPR YOLOv9-t 384 detector cache | 7,771,218 | `888397b96d761c89db40bc9c305838e8652660f5e282c2cadebbe8d2951a77a8` | ONNX; 384×384; OpenVINO/CPU available |
| CCT-XS-v2 global OCR cache | 3,344,292 | `8031afb5fdc6b4d80462c9d542f1284ebd2cfddf5dbacd62609848d7e2855f44` | ONNX; 64×128 RGB |
| CCT OCR config | 1,725 | `0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6` | global Latin alphabet; no Pakistan region entry |
| historical COCO vehicle detector | 6,534,387 | `31e20dde3def09e2cf938c7be6fe23d9150bbbe503982af13345706515f2ef95` | YOLOv8n, 640, classes car/motorcycle/bus/truck selected |
| historical plate detector | 6,241,454 | `8ec3b254a6c87610f037a90957462cafa11a9c03224e33a28c6a1d1ac2ac51b0` | custom YOLOv8n, one `license_plate` class, 640 |
| EasyOCR CRAFT detector | 83,152,330 | `4a5efbfb48b4081100544e75e1e2b57f8de3d84f213004b14b85fd4b3748db17` | CPU, cached before gold |
| EasyOCR English G2 recognizer | 15,143,997 | `e2272681d9d67a04e2dff396b6e95077bc19001f8f6d3593c307b9852e1c29e8` | CPU, cached before gold |

## Model Hash Parity

The image reference detector, OCR model and OCR configuration are byte-for-byte
identical to the NX-MMR acquired assets. Relevant package versions also match.
`ModelHashParity=EXACT_IMAGE`.

The video reference is not a differently wrapped copy of the NX-MMR model. It
uses distinct PyTorch vehicle/plate detectors and EasyOCR assets. Names and
architecture cannot be treated as interchangeable.

## FastPlateOCR Reference Architecture

`batch_alpr.py` sorts files, limits the historical batch to 50, decodes BGR with
OpenCV, constructs package-default FastALPR with detector threshold 0.40, runs
detection/OCR on every image, then discards detections below 0.75. It clips the
integer detector box to the image, uses no padding/rotation/multi-crop step,
normalizes to uppercase A–Z/0–9, saves annotated images and raw crops, and writes
one CSV row per accepted result. Batch size is one and no explicit thread limit
is set.

NX-MMR instead constructs the identical detector directly at threshold 0.75.
Since higher-score boxes dominate NMS, the development and holdout reproduction
is the empirical authority for whether this ordering matters.

## FastPlateOCR Historical Outputs

- `output_baseline_50/results.csv`: 50 images, 55 accepted rows, 54 non-empty
  OCR outputs and one empty OCR output; SHA-256
  `56ad58d2c4a8564f7f9430ea7b42a0091db8687faeda49e3b53c4b118eb54268`.
- 50 annotated images and 55 saved crops exist. Their image bytes match the
  later `output` annotated/crop directories.
- The later `output/results.csv` contains manual review fields and fewer rows;
  it is retained as `REFERENCE_OUTPUT`, never used as truth.
- Twelve historical inputs overlap the new active gold: three development and
  nine holdout. Historical normalized exact results were 2/3 and 5/9,
  respectively—consistent with, not superior to, the full parity run.

## Vehicle-License-Plate-Detection Reference Architecture

The source processes every decoded frame. It runs YOLOv8n vehicle detection at
the Ultralytics default 640 input and default inference confidence, keeps COCO
vehicle classes 2/3/5/7, and feeds their boxes to SORT. A separate custom
YOLOv8n plate detector runs on the full frame at 640. Only plate boxes contained
inside a tracked vehicle survive. Each plate crop becomes grayscale and then
binary inverse at threshold 64. EasyOCR runs on every surviving crop.

OCR is accepted only when it fits a hardcoded seven-character UK positional
format. Deterministic O/I/J/A/G/S substitutions are applied by position.
`test.csv` stores only vehicle-associated, format-valid OCR observations. The
post-process linearly interpolates car and plate boxes across missing frames.
The renderer selects the single maximum OCR-confidence value per SORT car ID
and displays that value across the interpolated span.

This supplies dense detection, a vehicle ROI constraint, repeated crops and a
pre-existing form of multi-frame candidate selection. It also introduces
UK-only normalization and temporally extended track labels that cannot become
Pakistan-wide or user-visible identity/tracking semantics.

## Historical Video Outputs

| Artifact | Observed extent | SHA-256 |
|---|---|---|
| `test.csv` | 2,248 OCR observations; 1,849 unique frames; 45 car IDs; frames 0–3599 | `40be951dc3093d066c81d442d77953f1fd71e03ef73686c24053969c4f836107` |
| `test_interpolated.csv` | 4,536 rows; 2,929 unique frames; 2,248 original OCR values | `456a6fdeca4845be912d80ee1340ac18c5ada5d5009f51b8e5e66fbf450f1af4` |
| `out.mp4` | 425,676,825-byte annotated render | `4af01d651edc4f31d527df618219fb087f076956692fefc345f11136d72b8b2d` |

## sample.mp4 Identity Check

Both video files are 184,407,144 bytes with SHA-256
`d470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9fafc83762d137ee`.
`ReferenceVideoInputParity=EXACT`.

The common source has two streams: H.264 High, 3840×2160, 60 fps, 3,600 video
frames/60.000 seconds; and AAC-LC, 48 kHz stereo, 2,813 audio frames/60.010
seconds. The container duration is 60.010 seconds.

## Current NX-MMR Image Pipeline

FastALPR uses the same YOLOv9-t 384 detector, CCT-XS-v2 OCR/config, package
versions, raw clipped crop and alphanumeric normalizer. Threshold 0.75 is
applied inside the detector and the worker records bbox, detector confidence,
OCR confidence and lineage. There is no crop padding, rotation, candidate vote
or multi-crop selection.

## Current NX-MMR Video Pipeline

The benchmark applies the image FastALPR processor to 60 frames at source
seconds 0–59. It has no vehicle detector, dense plate detection, adjacent-frame
burst, track/ephemeral association, best-crop selection or OCR vote. It groups
only exact normalized values observed in those isolated samples.

## Stage-Level Differential

| Characteristic | Historical working video reference | NX-MMR baseline V1 | Measured/potential impact |
|---|---|---|---|
| frame cadence | every frame, 3,600/3,600 at 60 fps | 60 frames at 1 Hz | 60× frame opportunity; two NX events were cadence-unsampled |
| vehicle detection | YOLOv8n 640, four vehicle classes | none | vehicle containment removes some unrelated plate boxes and enables association |
| plate detector | custom YOLOv8n 640, default inference confidence | YOLOv9-t ONNX 384, 0.75 | different model, scale and threshold; 14 NX missed events had sampled frames |
| crop | full-frame plate box, then grayscale + inverse threshold 64 | clipped BGR box passed to CCT RGB preprocessor | material OCR input difference |
| OCR | EasyOCR on every accepted frame crop | CCT-XS-v2 once per sampled detection | repeated crops recover values missed at any one instant |
| format normalization | strict UK seven-character positional map | general uppercase alphanumeric | benefits this UK source but is invalid as a Pakistan-wide rule |
| temporal association | SORT car ID, maximum-confidence OCR candidate | none | improves stability but may select a wrong high-confidence value |
| interpolation | boxes/selected candidate extended between observations | none | raises coverage and temporal false positives; must not be imported unchanged |
| source time | frame/60, dense first/last | exact sampled second | dense reference reduces matched-group boundary error |
| compute | two YOLO models + EasyOCR over 3,600 frames | one ONNX detector/OCR over 60 | historical cost is much higher; exact resource rerun remains gated |

## Frame-Cadence Differential

Cadence contributes but is not the whole root cause. NX-MMR left two events and
four plates unsampled. Fourteen further events and 30 plates had one or more
NX-MMR sampled frames but no detection. The historical raw reference detects
18/19 events and 33/41 plate-count opportunities, proving that detector path,
input scale/threshold, crop handling and repeated OCR all contribute.

## Detector Differential

Image detection parity is exact. Video detection is not comparable by model
name: the historical detector is a distinct 640-input YOLOv8 plate model,
preceded by 640-input vehicle detection and followed by vehicle containment;
NX-MMR applies the 384-input image detector directly with a much higher
threshold. The event detection delta is 0.947368 versus 0.157895.

## OCR Differential

For images, the exact same crops/models produce exact prediction agreement on
32/32 samples; no historical OCR recovery exists. For video, repeated EasyOCR
over thresholded crops yields raw exact recall 0.731707 and CER 0.146341 versus
NX-MMR 0.073171 and 0.926829. Raw reference output also emits many conflicting
variants, while per-track max confidence reduces variants but lowers exact
recall to 0.585366. Confidence alone is not a sufficient selector.

## Normalization Differential

The general NX-MMR alphanumeric normalizer is semantically safer. The reference
UK positional formatter materially fits this exact UK video but cannot be
adopted for Pakistan plates. A product adapter needs a declared plate-format
profile and must preserve raw OCR; it must not silently force UK substitutions.

## Temporal Aggregation Differential

Dense repeated observations are beneficial. Unbounded SORT/interpolation is
not. Raw exact-value group recall rises from 0.137931 to 0.758621 but creates
315 extra exact-value groups. The renderer's max-confidence-per-track candidate
reduces this to 18 extra groups with group recall 0.620690, while interpolated
negative-frame FPR reaches 0.722441. The measured reusable component is bounded
intra-video observation stabilization, not persistent tracking or interpolation.

## Image Reference Development Benchmark

The frozen pre-existing reference produced the same 10 prediction multisets as
NX-MMR: detection 10/10, exact 5/10, CER 0.184615. Exact agreement is 10/10.
Mean/p50/p95/max pipeline latency was
0.248784/0.220540/0.411354/0.482603 seconds. Model load was 0.743641 seconds;
benchmark wall was 3.499372 seconds and process CPU 45.000 seconds. The first
receipt did not recover Windows peak RSS; the holdout run did.

## Image Reference Holdout Benchmark

The configuration hash
`6ce10bccc6a8ccd80a03b251b5e63ac8e6fb8bb1f11c95e86285029c5c67a568`
was frozen before the single candidate holdout run. Results again match all 22
NX-MMR prediction multisets: detection 20/22, exact 13/22, exact
precision/recall/F1 0.650000/0.590909/0.619048, CER 0.201439. Mean/p50/p95/max
latency was 0.157921/0.146351/0.244087/0.447808 seconds. Model load was 0.613505
seconds, benchmark wall 4.481547 seconds, CPU 62.890625 seconds and peak RSS
168.070 MiB.

## Video Reference Benchmark

The pre-existing `test.csv`, `test_interpolated.csv` and `out.mp4` predate the
human gold. Exact source identity and understood output semantics make them
admissible candidate output, not truth.

| Metric | Historical raw observations | Historical final max-confidence/interpolated |
|---|---:|---:|
| events with any observation | 18/19 | 18/19 |
| count-proxy TP / FP / FN | 33 / 0 / 8 | 35 / 3 / 6 |
| count-proxy precision / recall / F1 | 1.000000 / 0.804878 / 0.891892 | 0.921053 / 0.853659 / 0.886076 |
| exact TP / FP / FN | 30 / 239 / 11 | 24 / 18 / 17 |
| exact precision / recall / F1 | 0.111524 / 0.731707 / 0.193548 | 0.571429 / 0.585366 / 0.578314 |
| normalized CER | 0.146341 | 0.170732 |
| frame presence precision / recall | 0.639805 / 0.569846 | 0.624104 / 0.880539 |
| negative-frame FPR | 0.437008 | 0.722441 |
| exact group TP / FP / FN | 22 / 315 / 7 | 18 / 18 / 11 |
| exact group precision / recall / F1 | 0.065282 / 0.758621 / 0.120219 | 0.500000 / 0.620690 / 0.553846 |
| mean absolute boundary error | 0.866030 s | 0.741833 s |

Raw exact precision treats every distinct OCR variant within an event as a
candidate and therefore exposes the variant explosion rather than hiding it.

## Current vs Reference Metrics

| Metric | NX-MMR Baseline V1 | Historical raw reference | Historical final reference | Best measured delta |
|---|---:|---:|---:|---:|
| image dev exact | 5/10 | 5/10 frozen rerun | n/a | 0 |
| image holdout exact | 13/22 | 13/22 frozen rerun | n/a | 0 |
| video event detection | 3/19 | 18/19 | 18/19 | +15 events |
| video exact recall | 3/41 (0.073171) | 30/41 (0.731707) | 24/41 (0.585366) | +0.658536 raw |
| video CER | 0.926829 | 0.146341 | 0.170732 | −0.780488 raw |
| group recall | 4/29 (0.137931) | 22/29 (0.758621) | 18/29 (0.620690) | +0.620690 raw |
| negative-frame FPR | 0.038462 | 0.437008 | 0.722441 | worse by +0.398546 raw |
| mean absolute boundary error | 1.178250 s | 0.866030 s | 0.741833 s | −0.436417 s final |

## Visual Diagnostic Outputs

Private/non-retained diagnostics include reference-annotated image errors and a
five-item visual comparison pack: one exact image, one OCR-error image, one
current-video positive, one historical recovery where an NX one-second sample
still had no detection, and one historical temporal-gap false positive. Each
comparison shows original, CURRENT_NXMMR_BASELINE_V1 and historical reference.
The selected pack is
`local-acceptance-models/nxmmr/private-benchmarks/reference-parity/visual-comparison-v2`.
It is Git-ignored and must not be published.

Optional user verification is deliberately small: open `index.json`, inspect
the five referenced JPEGs, and compare the labeled original/current/reference
panels. No labels are editable and no additional manual labeling is required.

## Root Cause(s)

1. `ImageRootCause=MODEL_OCR_CAPABILITY_NOT_PIPELINE_REGRESSION`. Exact assets,
   packages and 32/32 prediction agreement disprove an image implementation
   parity gap.
2. `VideoRootCause=DISTINCT_SPARSE_IMAGE_PIPELINE_USED_INSTEAD_OF_DENSE_VIDEO_REFERENCE`.
   The current benchmark discarded the known two-stage 640 detector path,
   dense frame opportunities, vehicle containment, thresholded crops and
   repeated OCR.
3. Cadence is contributory, not sufficient: 14 missed events had current
   sampled frames.
4. Historical UK formatting and unrestricted SORT interpolation artificially
   improve some values while producing unacceptable variant and temporal false
   positives. They are measured limitations, not reusable product truth.

`PipelineParityGap=PROVEN` for video and `NOT_PROVEN` for images.

## Recommended Selected Baseline

- Image: keep `CURRENT_NXMMR_BASELINE_V1` FastALPR as the measured baseline.
  The personal reference adds no accuracy.
- Video diagnostic/reference: select
  `PRE_EXISTING_YOLOV8_SORT_EASYOCR_RAW_OBSERVATION_REFERENCE_V1` as the best
  current measured recovery baseline, because it has the highest exact recall
  and lowest CER. It is not product-admissible unchanged.
- Product candidate: reuse the existing dedicated 640 plate detector, optional
  vehicle ROI and repeated crop/OCR architecture behind the NexusAI processor,
  but replace persistent SORT IDs/interpolation with bounded ephemeral
  within-source bursts and deterministic raw-preserving candidate aggregation.
  This adapted candidate must be development-benchmarked before certification.

## Integration Delta

Use the existing media processor/Observation Packet path. Add one versioned
video ANPR processor configuration that performs bounded dense/adaptive plate
detection, optional vehicle containment, short adjacent-frame refinement and
raw-preserving candidate aggregation. Record detector/OCR model hashes,
per-observation boxes/times/confidences and aggregation reasons. Emit no
persistent track ID, cross-video identity, journey, route or ownership claim.
Do not copy the personal repository wholesale or create a separate ANPR app.

## Certification Consequences

The reference benchmark supplies material real-world candidate evidence but
does not itself change the operation ledger. Image remains limited; the current
video baseline remains rejected; the historical raw reference becomes a
versioned pre-existing candidate. No direct `REAL_WORLD_CERTIFIED` or
`PRODUCT_CERTIFIED` promotion is recorded because the adapted bounded processor
has not yet been implemented/scored and the historical false-positive behavior
is too high.

## Query/Ask Consequences

No query code changes now. A later selected processor may feed the existing
typed operations for candidate plates, source-time filters, repeat observations
and uncertainty. Ask may route/explain those cited observations but may not OCR,
repair characters, invent times or compute deterministic counts.

## Data UI Consequences

No UI changes now. A later product gate may show raw-preserving candidate text,
boxes/crops, first/last source time, observation count, separately labeled
detector/OCR confidence and click-to-seek citations. It must not show track,
journey, owner or cross-video identity semantics.

## Auto-Routing Consequences

The shadow planner remains unchanged. ANPR readiness must remain unavailable or
explicit-test-only until the bounded processor is source-validated and
activated separately. Ordinary visible suggestions remain
`PRODUCT_CERTIFIED` only.

## Fine-Tuning Decision

`FineTuningRequired=UNDECIDED`. It is not justified before the existing video
architecture is adapted and development-scored. Image errors remain relevant
future OCR evidence, but this slice proves no configuration recovery for them.

## New-Model Decision

`NewANPRModelRequired=false` for the immediate recovery slice. A materially
better video detector already exists locally with a fixed hash. License/notice,
security and adapter admission remain required before integration.

## New-Dataset Decision

`NewANPRDatasetRequired=false` for the immediate parity-recovery slice. No data
should be downloaded now. Independent generalization/hard-negative evidence
will still be needed later and the completed holdout must not be reused for
tuning.

## Resource Measurements

- Current NX-MMR image peak: 279.691 MiB development and 255.539 MiB holdout.
- Frozen image reference holdout peak: 168.070 MiB; exact output parity despite
  CPU fallback. Its latency is slower than the container's OpenVINO-requested
  path but accuracy is identical.
- Current NX-MMR video: 63.878912 seconds wall, 13.380812 seconds CPU,
  273.211 MiB peak RSS for 60 frames.
- Historical video files contain no process telemetry. Artifact chronology is
  not treated as runtime measurement. The exact full rerun was stopped before
  load because 3.765 GiB free was below the 4.5 GiB HEAVY floor.
- Final read-only audit: the same five named NexusAI containers remain up (four
  healthy and the records API running normally), with unchanged two-day
  uptime. Host free RAM was 4.250 GiB, still below the HEAVY rerun floor. Live
  container memory at the snapshot was 2.221 GiB for the primary API and below
  156 MiB for each other service.

## Files Changed

- added the frozen-reference scorer and tests;
- added the private visual comparison generator;
- added this aggregate/redacted report;
- updated the NX-MMR continuation and immediate-priority records.

Neither personal reference directory was modified.

## Tests

- three reference parity scorer unit tests pass;
- existing four NX-MMR scorer and four human-oracle workflow tests remain pass;
- Python compilation passes;
- receipt-to-report assertions pass for all published development, holdout and
  historical-video headline metrics;
- the automated scan of 61 private human plate values across the changed
  public records and scorer sources found zero leaks;
- all eight reported frozen model/reference assets plus the core
  source/input/historical-output hashes remain unchanged after the work, and
  private receipts/visuals remain Git-ignored;
- full video runtime reproduction is resource-blocked, not failed.

## Runtime Impact

`RuntimeMutated=false`; `LiveModelsChanged=false`; `DeploymentPerformed=false`.

## Retained Impact

`RetainedStateMutated=false`; `ActivityMutated=false`;
`DatabaseMigration=false`; `VolumesChanged=false`.

## Open P0

`OpenP0=0`.

## Open NX-MMR P1

`OpenNXMMRP1=1`: implement and development-score the bounded video reference
adapter without persistent tracking semantics, then freeze its configuration.

## Exact Next Action

Prepare and implement only a source-level, non-retained
`NX-MMR-REFERENCE-PARITY-ADAPTER-V1` development slice using the existing local
video detector/OCR assets. Preserve raw observations; use bounded ephemeral
adjacent-frame aggregation; exclude UK-only global normalization and SORT
interpolation from product authority. Measure it on development/private video
evidence, freeze the configuration, and return for an explicit one-time
candidate evaluation/activation decision. Do not download, fine-tune, deploy,
touch retained evidence or start NX-B2.1.

```text
ReferenceImagePipelineLocated=PASS
ReferenceVideoPipelineLocated=PASS
ReferenceVideoInputParity=EXACT
ReferenceEnvironmentReproduced=PARTIAL
ReferenceImageBenchmark=PASS_EQUAL_TO_CURRENT_NXMMR_BASELINE_V1
ReferenceVideoBenchmark=PASS_HISTORICAL_OUTPUT_SCORED_RUNTIME_RESOURCE_BLOCKED
PipelineParityGap=PROVEN
ImageRootCause=MODEL_OCR_CAPABILITY_NOT_PIPELINE_REGRESSION
VideoRootCause=DISTINCT_SPARSE_IMAGE_PIPELINE_USED_INSTEAD_OF_DENSE_VIDEO_REFERENCE
SelectedANPRBaseline=IMAGE_CURRENT_FASTALPR_V1_VIDEO_PREEXISTING_DENSE_RAW_REFERENCE_V1_FOR_ADAPTATION
FineTuningRequired=UNDECIDED
NewANPRModelRequired=false
NewANPRDatasetRequired=false
VisibleSuggestionsProductCertifiedOnly=PASS
NoSelfOracle=PASS
OpenP0=0
RuntimeMutated=false
LiveModelsChanged=false
RetainedStateMutated=false
ActivityMutated=false
DatabaseMigration=false
VolumesChanged=false
DeploymentPerformed=false
NXB2ActivationPerformed=false
```
