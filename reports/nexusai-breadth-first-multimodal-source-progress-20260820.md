# NexusAI breadth-first multimodal source progress

Date: 2026-08-21  
Program: BF-0 through BF-4 bounded source implementation  
Runtime mutation: none

## Reconciliation

| Family | Existing source/runtime | Reuse decision | MVP gap after this slice |
| --- | --- | --- | --- |
| ANPR | WORKING structured adapter/query; PARTIAL R8 image contracts | Reuse canonical ANPR records, R8 provenance contracts and the MIT FastALPR stack through a Nexus-owned adapter | guarded worker activation and representative review/negative/corrupt goldens |
| Image | PARTIAL immutable intake, metadata and overlay; intake/API/UI WORKING | Queue supported raster evidence to the shared worker; reuse retained bytes and artifact store | general OCR/vision unavailable; HEIC/raw/SVG remain manual review |
| Audio | PARTIAL deterministic inventories; ASR MISSING | Add ffprobe observations, playback, transcript/segment contracts and truthful no-ASR state | one approved CPU ASR model is required for M1 |
| Video | PARTIAL deterministic container inventories | Add ffprobe, bounded ffmpeg frame/audio extraction, reuse image/ANPR processors and timestamp locators | guarded activation; ASR awaits Audio M1; tracking/deduplication deferred |
| Documents | PARTIAL safe inventory and KB/RAG baseline | Preserve current scope | native extraction, OCR fallback and page-region citations remain later breadth work |

Shared evidence registration, content-addressed retention, versions, custody,
processing runs/events, derived artifacts, canonical records, tenant/collection/
case scope, source-file audit, ANPR Ask operations and the analyst evidence
workspace were reused. No schema change was required.

## External reuse and provenance

The external FastPlateOCR project and environment were read only. Reused
package versions are `fast-alpr 0.4.0`, `fast-plate-ocr 1.1.0`,
`open-image-models 0.6.0`, `onnxruntime-openvino 1.24.1`, `openvino 2025.4.1`
and `numpy 2.3.5`. FastALPR, fast-plate-ocr and open-image-models declare MIT;
OpenVINO declares Apache-2.0; ONNX Runtime OpenVINO declares MIT; NumPy uses
BSD terms. NexusAI requires explicit local model paths and disables implicit
downloads.

The Vehicle-License-Plate-Detection tutorial was also read only. Reused
concepts are frame sampling, plate observations, timestamps and later
tracking/deduplication. No source or weights were copied because its
Ultralytics dependency is AGPL-3.0 and its environment is not a product runtime.

## Implemented bounded architecture

- Supported raster, audio and video formats queue through the existing durable
  ingestion lifecycle to one media worker.
- Every observation retains evidence/version/source locators and is stored as a
  derived artifact explicitly distinguished from source truth.
- ANPR OCR observations additionally enter canonical ANPR records so governed
  sightings/timeline/variant queries and citations work.
- FastALPR is opt-in, uses mounted read-only assets, applies the 0.75 threshold,
  preserves raw and formatting-only normalized text and requires review.
- Audio emits technical observations. Transcript and timestamp-segment
  contracts plus an opt-in local-only ASR client exist, but no transcript is
  fabricated without an explicitly configured model.
- Video emits stream metadata, bounded sampled-frame observations, optional
  ANPR observations, and embedded-audio observations. Temporary files are removed.
- The LocalAI route authorizes the case before proxying verified, bounded,
  range-capable image/audio/video content from the sidecar.
- Analyst Data can preview media, inspect artifacts and compare raw/normalized
  plate output with review labeling.

## Validation evidence

The real read-only `DSC_1104.jpg` smoke returned `MN1367`, detector confidence
`0.8897413611412048`, mean OCR confidence `0.999785578250885`, and bounds
`x=396 y=568 width=157 height=60` in 7.993 seconds including cold load.
OpenVINO could not load `openvino.dll` and ONNX Runtime fell back to
`CPUExecutionProvider`; a repeat with the same reference copy completed in
4.917 seconds and returned the same text/confidences. This is integration
evidence, not certification.

Python compilation passes. Pytest is not installed in available runtimes, so no
pytest result is claimed. Final focused Go/UI results belong in the handoff.

## Status and boundaries

`StructuredIntelligence=CLOSED`. `ANPRProcessorRuntime=ACCEPTED`.
`ImageProcessorRuntime=ACCEPTED`. `AudioProcessorRuntime=ACCEPTED`; deployed
`whisper-tiny` is rejected for Pakistan M1, while the pinned and deployed
`Systran/faster-whisper-small` challenger makes
`PakistanUrduASRPracticalBaseline=PASS_FOR_M1`. With forced `ur`, both lawful
FLEURS clips materially improved; the synthetic fixture preserved phone/time
but not plate. Natural
code-switch, spontaneous conversation and documented Pakistani-English remain
pending. `VideoCompositionRuntime=ACCEPTED`. The final deterministic plate smoke returned two
frames at 0/5 seconds, two distinct `MN1367` observations, three timestamped
ASR segments, nine unique observations and exact cleanup in 10.796 seconds;
database counts remained `19|20|0`. `DatabaseMigrationNeeded=NO` and
`ModelDownloadStatus=COMPLETED_AND_ACTIVATED`. A separate lawful Islamabad-image/FLEURS-audio
composition passed non-retained processing with two 0/5-second frames, one
timestamped transcript, unique IDs, cleanup and unchanged `19|20|0` counts;
ANPR was optional and returned no plate. The 2,851,891-byte ignored acquisition
bundle and exact retained diagnostic pack are prepared. Retained breadth E2E
has not run. No retained ingest,
training, fine-tuning, external edit, staging, commit, push or PR occurred.

`AudioMVPStatus=RUNTIME_ACCEPTED_RETAINED_E2E_PENDING`, `OpenP0=0`,
`OpenP1=0`. The deployed worker honors explicit `ur`; automatic detection
remains available and reproduces the known Hindi/Devanagari result. Exact model
provenance, runtime evidence and quality scores are in
`reports/nexusai-pakistan-urdu-asr-challenger-acceptance-20260820.md`.
Only retained processing requires new explicit approval.

Governed Runtime Query Intelligence remains deferred until the breadth baseline.
Accuracy, Urdu specialization, tracking, deduplication, adverse-condition packs
and throughput tuning are non-blocking M2/M3 work.
