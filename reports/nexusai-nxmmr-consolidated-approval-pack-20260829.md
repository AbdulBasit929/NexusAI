# NX-MMR consolidated approval pack

Status: `APPROVED_BOUNDED_EMPIRICAL_EXECUTION`. The 2026-08-29 product-owner
directive authorizes sections A-D exactly as corrected below and confirms
lawful authority for the two named local inputs. NX-B2.1 remains paused.

## Recorded authorization truth

| Authority | Value | Exact boundary |
|---|---|---|
| External download | `true` | seven listed files only, maximum 663,933,265 bytes; no substitutions |
| Live model install | `false` | isolated cache/evaluators only |
| Local evidence processing | `true` | only the two named inputs; private, read-only and non-retained |
| Runtime build/deploy | `false` | no build, pull, restart, recreation or deployment |
| Retained mutation | `false` | no ingest, reprocess, Activity, database, KB or named-volume mutation |

## Approval requested

### A. Isolated model acquisition, maximum 663,933,265 bytes

Authorized. Inspect existing isolated evaluator caches first. Download only missing files
from the seven immutable URLs in
`configuration/nexusai_multimodal_benchmark_manifest_v1.json` into a new
`local-acceptance-models/nxmmr` cache. Verify exact byte size, publisher/local
hash, license and notices before extraction or inference. The bundle is:

- FastALPR detector, OCR and config: 11,117,235 bytes;
- Tesseract `eng+urd`: 5,511,806 bytes;
- PaddleOCR Arabic PP-OCRv5: 8,151,040 bytes;
- Qwen3-Reranker-0.6B Q8: 639,153,184 bytes.

No other model, package, image, dataset, source repository or dependency may be
downloaded. If an existing evaluator cannot run without an unlisted download,
stop and return a new exact manifest.

### B. Local non-retained evidence use, 234,784,354 existing bytes

Authorized after integrity verification and independent ground-truth sealing.
The operator has confirmed lawful authority for read-only model processing of:

- `C:\Users\sheik\Downloads\sample.mp4`, 184,407,144 bytes, SHA-256
  `d470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9fafc83762d137ee`;
- `C:\Users\sheik\Downloads\archive\Pakistani License Number Plates Data`,
  108 JPGs totaling 50,377,210 bytes.

Originals remain untouched. Create only hash/ground-truth manifests and
non-retained benchmark outputs under the controlled workspace. Do not ingest,
upload, copy into Git, retain in PostgreSQL/KB, publish plate strings/faces, or
attempt identity inference.

### C. Isolated benchmark execution

Use the approved workload-aware admission rule and one heavy role at a time:
LIGHT requires 3.0 GiB free, MEDIUM 3.5 GiB, HEAVY 4.5 GiB, and VERY_HEAVY is
not admitted by default. Once measured, require expected incremental peak plus
at least 1.5 GiB headroom, never below the class floor without an explicit
receipt justification. Use sequential 5→10→20 batches and stream/bound video.
Stop near 1 GiB free, severe paging, evaluator death, Docker instability, or
two resource failures. Existing local images may be used—no image build or
pull—including:

- `nexusai/r8-fastplate-eval:1.1.0-cpu` (`7ecb12c53a2d`);
- `nexusai/r8-ocr-eval:3.7.0-cpu` (`614ac87f4f10`);
- the existing LocalAI/worker images only as disposable, network-disabled,
  resource-capped evaluators where required.

Evidence mounts are read-only; output/cache mounts are new and isolated. Do not
attach PostgreSQL, NATS, live model, evidence or KB volumes. Record baseline
before tuning, use independent oracles, run sealed negatives/holdouts, capture
latency/peak RAM/errors, and stop on every manifest stop condition.

### D. Post-benchmark source/report work

Allow bounded source changes and tests required to consume the benchmark
receipts, refine auto-routing, and promote or reject roles/operations. This does
not authorize live installation, deployment or retained processing. Any
promotion that changes live services returns with exact service/image/rollback
and RAM evidence for a second approval.

## Explicitly excluded

- no P-LPCD, Common Voice, Mendeley, UTRNet or other dataset download;
- no package-manager, Git, container-image or backend download;
- no live LocalAI model install/config change;
- no API/UI/worker rebuild, recreation, restart or deployment;
- no database migration, profile/backend change, named-volume mutation;
- no retained upload/reprocess or Activity mutation;
- no cleanup of unrelated files/images/volumes;
- no stage, commit, push or PR.

## Current gate state

The 3,733,495,808-byte inventory value is historical. Remeasure immediately
before each workload and apply its class floor and later measured headroom rule.
It admits LIGHT and MEDIUM at that historical value, but not HEAVY. The separate
6 GiB build/deployment gate remains closed and unchanged.

## Approval disposition

The operator approved A-D and confirmed that the two local evidence sources may
lawfully be processed locally for a private, non-retained benchmark. The
corrected workload-aware benchmark policy supersedes the former fixed 6 GiB
benchmark threshold only; all exclusions and the separate deployment gate
remain in force.
