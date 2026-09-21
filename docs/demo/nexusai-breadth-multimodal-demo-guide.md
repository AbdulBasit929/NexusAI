# NexusAI unified multimodal product demo guide

## MMV-2 final closure note — 2026-08-24

Do not demo the new Urdu or grouped-video ANPR paths as live until the narrow
forensic API plus LocalAI/UI activation and post-activation certification pass.
After activation, demonstrate the current retained video first as the truthful
completed-zero case, then certify English, Roman Urdu and Urdu routing,
citations, History reopen and 390/820/1024/1440 viewports. Do not upload
`sample.mp4` until ownership/retention attestation, visual candidate review and
separate retained-mutation approval are complete. Grouped OCR observations are
not tracking or identity.

## MMV-1 team-lead demonstration addendum — 2026-08-24

The retained 21-source script below is a historical accepted checkpoint; live
read-only state now contains 24 sources in this collection. Do not represent
the approved non-retained `sample.mp4` as retained evidence.

1. Show the MMV-1 report pack and verify the persisted video report hash
   `da880be4ddd87422a25a562f8b6a8d71ec6366c1281d3758c6de2dd3500a3fba`.
2. Explain the unchanged current-pipeline positive result: `LN15ZZC`, 25
   seconds, bbox `(1045,1818,176,60)`, detector `0.781484`, OCR `0.999857`.
3. Show the Urdu OCR baseline: exact heading, aggregate CER `0.436842`, severe
   paragraph/mixed-script limitations, and truthful challenger gate.
4. Show the bounded audio matrix: two lawful natural Urdu speakers plus one
   synthetic identifier control; never generalize the three samples to
   population accuracy.
5. In a source build, demonstrate MIME/family-derived audio icon and summary.
   The active deployed UI remains unchanged until separately authorized.
6. Close with P0=0, open foundational P1=0, MMV-2 next, and the explicit
   retained/deployment/model gates. Do not upload, reprocess, download, migrate,
   deploy or clean up during this demonstration.

Updated: 2026-08-23  
Workspace: `nexusai-multimodal-product-acceptance`  
Tenant: `default`  
Retention: review hold; do not delete or clean up without separate approval

## Demo outcome

This is the single product-demo workspace. It contains the exact approved 21
sources and supports real, case-scoped Data, Knowledge Base, Ask, citations and
History across:

- CDR, IPDR, subscriber, tower, structured ANPR, access logs and transactions;
- retained ANPR image observation, image OCR, image comparison, dHash and
  bounded SigLIP similarity;
- synthetic face detection and explicitly enumerated candidate similarity;
- Urdu audio, authoritative Urdu transcripts, Roman Urdu derivatives and
  timestamped video observations;
- native TXT, text-bearing PDF and DOCX passages plus KB retrieval.

The live retained state is `21 evidence | 21 versions | 34 jobs (33 completed,
1 historical dead-letter, 0 active) | 9,274 canonical records | 372 derived
artifacts | 19 KB assets`. All 21 latest evidence states are completed.

## Five-minute demonstration

1. Open `http://localhost:8080/analyst/home?case=nexusai-multimodal-product-acceptance`.
   Confirm **Multimodal Product Acceptance**, **21 ready sources**, zero
   processing/attention items, and 15 available intelligence areas.
2. Open **Data**, select **Load more sources**, and show all 21 sources. The
   catalog includes the structured families, two general/ANPR images, five
   synthetic face controls, three Urdu audio files, one derived video and
   TXT/PDF/DOCX.
3. Open `Image test plate test plate.jpg`. Show the authenticated source image,
   original-pixel OCR/region overlays, derived cards and stable
   `nexusai://evidence/.../artifacts/...` citations. Ask:
   `Show sightings for plate MN1367 with source citations. Use deterministic only mode.`
   Expected: one cited `MN1367` observation, review-required, never upgraded
   into a certified real-world sighting claim.
4. Open `Subject a frontal.png`. Explain that face results are candidate visual
   similarity—not identity. The retained bounded ranking is blur `0.87746`,
   pose `0.84191`, different subject `0.30930`.
5. Explain the SigLIP candidate ranking from the same query image: pose
   `0.96684`, blur `0.93579`, different subject `0.78518`, no-face street
   `0.46303`, Islamabad road `0.43841`. It is semantic similarity, not fact or
   identity.
6. Open `Clear urdu.wav` and `Pakistan short video.mp4`. Show the source-bound
   timestamped Urdu and Roman-Urdu cards. The embedded in-app browser reports
   audio/video codec playback unsupported, but authenticated source delivery is
   operational (`200` full and `206` range for both); use a codec-capable browser
   for playback during the demo.
7. Open `Case notes records demo.txt`, the PDF brief and the DOCX guide. Show
   the native passage cards. TXT/PDF source actions are inline; DOCX is a
   governed attachment. Ask:
   `Which retained source mentions NEXUS-DEMO-2026, MN1367, 03001234567 and 10:35, and what synthetic-evidence limitation applies? Use deterministic only mode.`
   Expected: `answered_with_limitations`, three citations, execution route
   `derived_text_lexical + kb_rag`.
8. Open **History**. Reopen the successful `MN1367` analysis, the grounded
   document analysis, and the truthful negative `ZZZ9999` analysis. Confirm
   questions, case context, citations, limitations and Continue actions.

## Core media pipeline demonstration

Use exactly tenant `default`, workspace/case
`nexusai-multimodal-product-acceptance`, and actor
`nexusai-breadth-acceptance-operator`. Do not upload, reprocess, download a
model, or clean up evidence during this demonstration.

### Start and health

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'
Invoke-WebRequest 'http://localhost:8080/readyz' -UseBasicParsing
docker compose -f docker-compose.forensic-records.yaml -f docker-compose.forensic-records.runtime.yaml ps
```

Expected: LocalAI `200`, and LocalAI, forensic API, worker, PostgreSQL and NATS
healthy in Compose. An unauthenticated direct request to forensic API port 8091
returns `401`; this is the expected credential boundary, not a health failure.
Open:

`http://localhost:8080/analyst/data?case=nexusai-multimodal-product-acceptance`

### Exact retained media

| Role | Retained source | Evidence ID | Local acceptance asset |
| --- | --- | --- | --- |
| ANPR positive | `image-test-plate-test_plate.jpg` | `a0ff7b18-129f-4a8a-82cf-48887f98bd9e` | `C:\Users\sheik\Workspace\Office\Projects\NexusAI\local-acceptance-inputs\unified-source-copies\image-test-plate-test_plate.jpg` |
| Video complete-zero ANPR | `pakistan-short-video.mp4` | `50057921-4f1f-4ab8-bab0-47bcdc957822` | `C:\Users\sheik\Workspace\Office\Projects\NexusAI\local-acceptance-inputs\pakistan-video\pakistan-short-video.mp4` |
| Face query | `subject-a-frontal.png` | `c23c3e10-a7a6-435a-ae64-c411a7cd0e14` | `C:\Users\sheik\Workspace\Office\Projects\NexusAI\tests\fixtures\faces\nexusai-synthetic\subject-a-frontal.png` |
| Same subject, pose | `subject-a-pose.png` | `3fff8296-ee69-4283-a498-7a0000887072` | `C:\Users\sheik\Workspace\Office\Projects\NexusAI\tests\fixtures\faces\nexusai-synthetic\subject-a-pose.png` |
| Same subject, blur | `subject-a-blur.png` | `769f6124-5b06-4c73-9122-39ef4423e0c4` | `C:\Users\sheik\Workspace\Office\Projects\NexusAI\tests\fixtures\faces\nexusai-synthetic\subject-a-blur.png` |
| Different subject | `subject-b-frontal.png` | `273bc132-74b5-4633-b8f9-bf0a812ef3ff` | `C:\Users\sheik\Workspace\Office\Projects\NexusAI\tests\fixtures\faces\nexusai-synthetic\subject-b-frontal.png` |
| No-face negative | `no-face-street.png` | `3ffc0eae-f030-4e62-835f-100682b99b1c` | `C:\Users\sheik\Workspace\Office\Projects\NexusAI\tests\fixtures\faces\nexusai-synthetic\no-face-street.png` |

The original plate file was separately approved as user-owned or lawfully
usable at `C:\Users\sheik\Workspace\Personal\FastPlateOCR\test_plate.jpg`.
The retained acceptance copy and its recorded SHA-256 are authoritative for
this workspace.

### ANPR image

1. In **Data**, select **Load more sources**, search `plate`, and open
   `Image test plate test plate.jpg`.
2. Confirm the original preview, **Plates** overlay, `MN1367`, detector
   confidence `89%` (raw `0.8897411`), plate-OCR confidence `100%` (raw
   `0.9997855`), and recorded bbox `x=396, y=568, w=157, h=60`.
3. Confirm the crop is dynamically reconstructed from retained source pixels;
   it is not a new retained artifact.
4. Confirm the artifact citation begins
   `nexusai://evidence/a0ff7b18-129f-4a8a-82cf-48887f98bd9e/artifacts/`.
5. Use **Find MN1367**, or ask exactly:
   `Show sightings for plate MN1367 with source citations. Use deterministic only mode.`
6. Expected: `answered_with_limitations`, one exact row, one citation, source
   `image-test-plate-test_plate.jpg`. The accepted live read measured 1,296 ms.

Negative control:
`Show sightings for plate ZZZ9999 with source citations. Use deterministic only mode.`
Expected: bounded no-result, not proof the plate never occurred.

### Video

Open `Pakistan short video.mp4` and confirm:

| Check | Expected retained result |
| --- | --- |
| Duration | 10.5 seconds |
| Plate timeline | `Plate processing complete — 0 detections` |
| Frame observations | `0:00 — 0 plate · 20 text`; `0:05 — 0 plate · 20 text` |
| OCR | 40 review-required observations with frame timestamps |
| Speech | one timestamped Urdu segment at `0:00` |
| Roman Urdu | one derivative at `0:00`; raw Urdu remains authoritative |
| Preview | in-app browser codec unsupported; authenticated full/range source delivery remains `200/206` |

Select the `0:00` and `0:05` frame rows to seek when a codec-capable browser is
used. Do not claim a video plate: the retained video has zero ANPR artifacts.
Use **Find this recognized text** or **Find this transcript phrase** only when a
new History entry is intended.

### Face and candidate intelligence

1. Open `Subject a frontal.png`; confirm one face candidate at `95%`, bbox/crop
   `535 × 694`, an immutable citation, and “candidate similarity available.”
2. Select **Rank face candidates**. Expected bounded SFace ranking:

| Rank | Candidate | Score | Meaning |
| --- | --- | ---: | --- |
| 1 | `Subject a blur.png` | `0.8775` | candidate similarity; not identity |
| 2 | `Subject a pose.png` | `0.8419` | candidate similarity; not identity |
| 3 | `Subject b frontal.png` | `0.3093` | lower candidate score; not exclusion proof |

The accepted live read measured 289 ms. Open `No face street.png` and confirm
that no irrelevant face section or ranking action is shown; its real OCR and
image-intelligence sections remain available.

### SigLIP and image comparison

From `Subject a frontal.png`, select **Rank similar images**. The verified first
three candidates are pose `0.9668`, blur `0.9358`, and different subject
`0.7852`; the accepted live read measured 58 ms. This is semantic candidate
similarity, not identity or fact.

Choose `Subject a pose.png` and select **Compare images**. Expected:
`exact duplicate = No`, perceptual similarity `0.7813`, semantic similarity
`0.9668`, `near-duplicate candidate = No`; the accepted live read measured
51 ms. The approved manifest contains no positive exact/near-duplicate pair.

### OCR and quick actions

Image and video OCR sections remove whitespace-only output. Low-confidence,
mixed-script and unusual-script model output remains visible with confidence,
script candidate, bbox/timestamp and analyst-review language. On the plate
source, **Find this recognized text** opens a case-bound Ask prompt containing
the actual retained text and source name. On the video, the same action includes
the actual OCR text and its retained source; the transcript action includes the
actual Urdu segment. Opening the link does not run a query; submitting Ask
creates normal retained History.

### History and citations

Open **History**, then analysis
`95a61f9d-a03b-4d96-8802-16f8fe987d6e`. Confirm the direct answer, one vehicle
sighting, source `image-test-plate-test_plate.jpg`, and select the source button
to reopen the cited evidence detail. Existing negative analysis
`7f656345-b19a-4e3a-bd52-2a9a581243e4` demonstrates clean no-result behavior.

### Responsive acceptance

The retained live Data detail was checked at 390, 820, 1024 and 1440 px. At all
four widths, `MN1367` and its quick action remained available, document/page and
detail-dialog horizontal overflow were absent, and the browser console contained
zero errors. The focused Playwright productization spec also passes all four
widths.

## Structured operations

The controlled 65-operation matrix covers CDR, IPDR, ANPR, subscriber, tower,
generic structured families, evidence, schema, quality and cross-family
correlation. Result: `64 answered | 1 accepted no-result | 0 routing failures`,
p95 `904.3 ms`. Do not substitute model-generated arithmetic for these
deterministic operations.

## Product truth boundaries

- The approved manifest contains no distinct exact-duplicate pair and no
  threshold-positive near-duplicate pair. Comparison works and returns truthful
  negative results; no positive duplicate claim is fabricated.
- Face similarity is never identity, demographics, tracking or a global face
  search. Candidate evidence IDs are explicit and case-scoped.
- SigLIP is candidate semantic similarity. General OCR, ANPR and ASR outputs
  remain review-required model observations unless separately certified.
- Raw Urdu remains authoritative. Roman Urdu is a derived aid. Spoken plate
  formatting remains `PENDING_M2`.
- The Islamabad road image is licensed real-world media with no asserted plate
  or face positive. Face controls and identifier audio are synthetic. The video
  is a derived composition, not natural evidence.
- One historical dead-letter job is preserved immutably. It is not a current
  failure; latest generations are completed.
- Earlier failed Ask history remains visible as immutable audit history beside
  the corrected successful rerun.

## Acceptance artifacts

- `reports/unified-multimodal-product-acceptance-20260823/product-acceptance-api.json`
- `reports/unified-multimodal-product-acceptance-20260823/export-checksum-manifest.json`
- `reports/unified-multimodal-product-acceptance-20260823/structured-65-operation-matrix.json`
- `reports/unified-multimodal-product-acceptance-20260823/media-role-reprocess-results.json`
- `reports/nexusai-unified-multimodal-product-acceptance-20260823.md`

## Security and source-access expectations

| Check | Expected/live result |
| --- | --- |
| Face search without explicit authorization | `403` |
| Image search without candidate evidence | `400` |
| Cross-tenant query/header mismatch | `403` |
| Wrong case or missing evidence | `404` |
| Image/audio/video/TXT/PDF/DOCX full source | `200` |
| Image/audio/video/TXT/PDF/DOCX byte range | `206` |
| DOCX disposition | attachment |
| Image/audio/video/TXT/PDF disposition | inline |

## Recovery and rollback

Only the worker and the related forensic API were rebuilt during recovery.
Protected PostgreSQL, NATS and LocalAI containers and all retained data remain
preserved. Worker rollback tag:
`nexusai/forensic-records-worker:rollback-before-unified-media-role-reactivation-20260823`.
API rollback tag:
`nexusai/forensic-records-api:rollback-before-unified-txt-retrieval-p1-20260823`.

No new evidence, force upload, second reprocess, database update/migration,
model download, broad replay, unrelated deployment, deletion or cleanup was
performed.
