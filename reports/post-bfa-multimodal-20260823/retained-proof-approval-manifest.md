# Proposed post-breadth retained-proof approval manifest

Status: `NOT_AUTHORIZED_NOT_EXECUTED`  
Tenant: `default`  
Collection/case: `nexusai-breadth-acceptance-20260820`  
Required actor: must be named by the operator  
Retention: inherit the existing review hold; cleanup requires separate approval

This manifest is a review artifact only. It does not authorize upload,
reprocess, job resume, deletion, cleanup, schema migration, model download or
deployment. Existing source bytes, versions, jobs and artifacts remain
immutable.

## Proposed bounded reprocess set

| # | Evidence ID | Source | Proposed new proof roles | Expected retained contracts | Required review oracle |
| --- | --- | --- | --- | --- | --- |
| 1 | `343b1e62-9f24-488c-a9a0-8d79ff91ee1c` | `test_plate.jpg` | FastALPR, dHash, SigLIP, general OCR; face may truthfully return zero | ANPR/image fingerprint/image embedding/OCR observations | Plate `MN1367`; general OCR and semantic scores remain review-required; no face identity |
| 2 | `22c74bf9-0fc1-479d-816b-311106ea707b` | `wikimedia-road-cars-islamabad.jpg` | dHash, SigLIP, general OCR; ANPR/face may truthfully return zero | image fingerprint/image embedding/OCR plus any bounded zero/positive specialist observation | Islamabad road/cars; no positive plate or face is asserted |
| 3 | `fb025351-f86b-405f-bbf0-1b5287d7bf8b` | `clear-urdu.wav` | Roman Urdu over existing explicit-Urdu transcript | `forensics.audio-roman-urdu-segment/v1` | Raw Urdu remains authoritative; source ground truth reviewed |
| 4 | `8c49950b-a91d-49cb-813b-3ff639aa9462` | `conversational-urdu.wav` | Roman Urdu over existing explicit-Urdu transcript | `forensics.audio-roman-urdu-segment/v1` | This is read Urdu, not spontaneous conversation |
| 5 | `88a9962d-24a0-4e3a-86be-c2a6db6e244f` | `urdu-english-identifier.wav` | Roman Urdu identifier-safety proof | `forensics.audio-roman-urdu-segment/v1` | Synthetic only; phone `03001234567`, time `10:35`, plate `MN1367`; spoken plate ASR remains M2 |
| 6 | `0dcb49a7-bd47-4b63-b4a2-7d67e2caffb0` | `pakistan-short-video.mp4` | Video-wrapped Roman Urdu and existing bounded media roles | video/audio timestamp and Roman derivative contracts | Derived composition, never natural video evidence |
| 7 | `2291d02e-e383-41d1-99dd-87d698a38210` | `HP_EliteDesk_LocalAI_Remote_Setup_Guide.docx` | Native DOCX passages and KB mirror | `forensics.document-passage/v1` plus governed KB mirror | Source SHA `85b1ab53150a556f84391a222b7fb4dbf69aa80d1c4d7d7a7b54d5f283ea4865`; disposable oracle 112 passages/5,371 chars |

## Proposed execution ceiling

- Maximum seven immutable reprocess jobs, one per named evidence ID.
- No re-upload, replacement evidence, in-place artifact update or broad case
  replay.
- Use only the normal immutable reprocessing endpoint.
- Stop on any scope mismatch, new dependency/model request, database error,
  model checksum mismatch, or unexpected job fan-out.
- Preserve every old dead-letter/current job and artifact for provenance.

## Required post-run acceptance

Record before/after counts, new job IDs, artifact IDs/contracts, source/version
hash binding, wall time/RSS, operation output, browser Data/Ask/History proof,
all negative checks, and an export/checksum manifest. Do not cleanup after the
verdict.

## Approval phrase template

`APPROVE POST-BREADTH RETAINED PROOF: tenant=default; collection=case=nexusai-breadth-acceptance-20260820; actor=<exact actor>; authorize at most seven immutable reprocess jobs for exactly the seven evidence IDs in reports/post-bfa-multimodal-20260823/retained-proof-approval-manifest.md; no upload, deletion, cleanup, migration, model download, unrelated deployment, broad replay or evidence replacement; retain all old and new evidence, versions, jobs, artifacts, KB mirrors and manifests until separately reviewed.`
