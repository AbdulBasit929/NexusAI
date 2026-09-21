# NexusAI multimodal pipeline productization acceptance

Recorded: 2026-08-23  
Verdict: **ACCEPTED WITH RECORDED LIMITATIONS**  
Scope: `default/nexusai-multimodal-product-acceptance`  
Actor: `nexusai-breadth-acceptance-operator`

# Verified Starting State

The authoritative accepted baseline was 21 evidence, 21 versions, 34 jobs
(`33 completed | 1 historical dead-letter | 0 active`), 9,274 canonical
records, 372 derived artifacts and 19 KB assets. This phase performed no
retained mutation. Final live API reads again returned 21 ready evidence, 34
jobs with the same status split, 372 artifacts and 19 KB assets.

# Services

Only `nexusai-api-1` was rebuilt/recreated. Active LocalAI is container
`e04dae4cbd44`, image
`sha256:afc6ac795e0c16eca293c765c23caf08fa768b961c774409324664ad320fd7b6`.
Rollback tag:
`nexusai/localai-forensic:rollback-before-media-productization-20260823`.

Preserved without recreation: forensic API `a97e545b8356`, worker
`0b2418d90cbd`, PostgreSQL `f66e05a3b179`, NATS `5c49e70d132a`, models,
volumes, evidence, versions and jobs.

# Unified Workspace

The portal selects **Multimodal Product Acceptance**, shows 21 ready sources,
and provides Data, KB-backed Ask, citations and History in the same case-bound
analyst experience.

# Existing Media Artifacts

Authoritative counts remain: image OCR 219, document passages 114, audio
timestamp segments 6, image observations 5, image fingerprints 7, image
embeddings 9, ANPR observations 1, Roman Urdu segments 6 and face observations
5.

# Data Detail Audit

## Image

PASS. Authorized source preview, typed plate/face/OCR region controls,
confidence, dynamic source-pixel crops, candidate actions and immutable
`nexusai://` citations render from retained artifacts.

## Video

PASS WITH LIMITATION. The detail shows a truthful complete-zero plate timeline,
0 s/5 s frame observations, 40 timestamped OCR observations, one authoritative
Urdu transcript and one Roman Urdu derivative. The in-app browser lacks the
codec for playback; authenticated source delivery remains operational.

## Face

PASS. Detection confidence, bbox/crop, embedding eligibility, bounded ranking,
citations and “not identity” semantics render. Empty embedding and no-face
states do not offer an invalid face rank action.

## OCR

PASS WITH REVIEW REQUIREMENT. Only whitespace-only output is removed. Real
low-confidence and mixed/unusual-script output remains visible with
confidence, script candidate, bbox/timestamp and review language.

# ANPR Image Productization

## Result Contract

The UI recognizes the retained `forensics.anpr-observation/v1` contract and
loads the bounded 250-artifact evidence-detail maximum so the plate result is
not hidden by generic preview limits.

## Plate Cards

PASS: one `MN1367` card on evidence
`a0ff7b18-129f-4a8a-82cf-48887f98bd9e`.

## Crop

PASS: reconstructed in browser from authorized source pixels and bbox
`x=396, y=568, w=157, h=60`. No crop artifact is newly retained.

## Overlay

PASS: plate, face and OCR typed overlay controls are independently focusable.

## Confidence

PASS: detector `0.8897411` and plate OCR `0.9997855` are separately labeled.

## Quick Actions

PASS: **Find MN1367** and **Find this recognized text** are built from actual
retained values and preserve the case query parameter.

## Ask

PASS: exact deterministic `MN1367` query returns
`answered_with_limitations`, one row and one source.

## Citation

PASS: plate artifact
`nexusai://evidence/a0ff7b18-129f-4a8a-82cf-48887f98bd9e/artifacts/65bff181-acb2-52d3-b950-88ccecedc8df`.

## History

PASS: analysis `95a61f9d-a03b-4d96-8802-16f8fe987d6e` reopens with the
vehicle table and source action; the source action reopens the cited Data
detail.

# ANPR Video Productization

## Frame Observations

PASS: `0:00 — 0 plate · 20 text` and `0:05 — 0 plate · 20 text`.

## Timeline

PASS: `Plate processing complete — 0 detections`. This is not a positive
sighting.

## Plate Grouping

PASS AS COMPLETE-ZERO: no empty plate group/card is fabricated.

## Thumbnail

M1 ACCEPTED SUBSTITUTE: player surface plus selected frame/timestamp controls;
no persisted thumbnail artifact was required or created.

## Seek

Frame buttons are present at 0 s and 5 s. Playback seeking requires a
codec-capable browser because the in-app browser reports this MP4 unsupported.

## Ask

PASS: source-local OCR and transcript quick actions contain real retained text.
No video-plate Ask is offered because the retained video has no plate artifact.

## Citation

PASS: timestamped Urdu and Roman Urdu cards expose stable artifact citations.

# OCR Productization

PASS. Image/video OCR cards expose actual raw text, confidence, script family,
bbox/timestamp, focus action, show-all control and source-local Ask action.
General OCR remains review-required and does not replace FastALPR plate OCR.

# Face Productization

## Detection

PASS: `subject-a-frontal.png` has one candidate at `0.9517`.

## Overlay

PASS: source-pixel face bbox is visible and focusable.

## Crops

PASS: `535 × 694` lossless crop is reconstructed from retained pixels.

## Similarity

PASS: explicit case-scoped SFace candidate similarity.

## Ranking

PASS: blur `0.8774586`, pose `0.8419142`, different subject `0.3092984`.

## Limitation

Scores rank candidates. They do not establish identity, exclusion,
demographics, tracking or fact.

# Image Similarity / Comparison

PASS. SigLIP ranks pose `0.9668381`, blur `0.9357892`, different subject
`0.7851777`. A-vs-pose comparison returns exact duplicate `false`, dHash
similarity `0.78125`, semantic similarity `0.9668381`, near-duplicate candidate
`false`. The manifest has no positive exact/near-duplicate control.

# Empty / Irrelevant Section Handling

PASS. The video shows a single explicit complete-zero plate result. The
no-face image omits an irrelevant face section/ranking action while retaining
its real OCR/image sections. Processing states distinguish not run,
processing, complete zero, complete results, failed and unavailable.

# Query Operations

| Family | Operation | Prompt / parameters | Actual result | Citation | Latency | Verdict |
| --- | --- | --- | --- | --- | ---: | --- |
| ANPR | `anpr.sightings` | plate `MN1367`, case-bound deterministic plan | 1 exact row, `answered_with_limitations` | 1 canonical source citation | 1,296 ms | PASS |
| Face | bounded candidate rank | A-frontal observation; blur/pose/B evidence IDs | `0.8775 / 0.8419 / 0.3093` | 3 artifact citations | 289 ms | PASS |
| SigLIP | bounded image rank | A-frontal embedding; pose/blur/B evidence IDs | `0.9668 / 0.9358 / 0.7852` | 3 artifact citations | 58 ms | PASS |
| Image compare | explicit pair | A-frontal vs A-pose | exact no; dHash `0.7813`; semantic `0.9668`; near-duplicate no | source pair in response | 51 ms | PASS |
| Video | retained frame/timeline read | video evidence `50057921-...` | 0 plates; 40 OCR; frames 0 s/5 s; transcript | artifact citations | included in detail read; not separately timed | PASS WITH CODEC LIMITATION |
| OCR | retained artifact/detail read | actual observation cards and case-bound quick action | whitespace removed; low-confidence truth retained | per-artifact citations | included in detail read; Ask not resubmitted | PASS |

# Local Car/Plate Test Matrix

| Input | Expected | Actual | Verdict |
| --- | --- | --- | --- |
| approved retained copy of `test_plate.jpg` | one review-required plate | `MN1367`, bbox/crop, detector `0.8897411`, OCR `0.9997855` | PASS |
| `ZZZ9999` Ask | bounded no-result | existing History reports no matching evidence | PASS |
| Islamabad road image | no fabricated plate | no positive plate section | PASS |

# Video Test Matrix

| Check | Actual | Verdict |
| --- | --- | --- |
| ANPR | complete zero | PASS |
| frame/timestamp | 0 s and 5 s, 20 OCR each | PASS |
| transcript | timestamped Urdu plus Roman Urdu derivative | PASS |
| playback | in-app codec unsupported; source `200/206` previously accepted | P2 LIMITATION |

# Face Test Matrix

| Control | Actual | Verdict |
| --- | --- | --- |
| A frontal | one face, crop, embedding | PASS |
| A blur / A pose | highest two SFace candidates | PASS |
| B frontal | lower relative candidate score | PASS |
| no-face street | no irrelevant face section/action | PASS |

# OCR Test Matrix

| Check | Actual | Verdict |
| --- | --- | --- |
| whitespace-only | removed | PASS |
| low confidence | remains visible and labeled | PASS |
| script family | candidate label, not language fact | PASS |
| video time | 0 s/5 s visible | PASS |
| quick action | actual text/source in case-bound Ask URL | PASS |

# UI Tests

## 390

PASS: 390 px page and 380 px detail dialog had no horizontal overflow; plate
detail remained usable.

## 820

PASS: no page/dialog horizontal overflow; `MN1367` and quick action present.

## 1024

PASS: no page/dialog horizontal overflow; `MN1367` and quick action present.

## 1440

PASS: no page/dialog horizontal overflow; `MN1367` and quick action present.
Browser console errors: zero.

# Services Final Health

LocalAI `/readyz` returned 200 after narrow recreation. A final unauthenticated
direct forensic-API request returned the expected `401` credential boundary;
its authenticated health check and forensic API, worker, PostgreSQL and NATS
container health had already passed after deployment. The three new read-only
media proxy contracts returned live results through LocalAI.

# Demo Guide

## Exact Path

`docs/demo/nexusai-breadth-multimodal-demo-guide.md`

## Exact Workspace

`default/nexusai-multimodal-product-acceptance`

## Exact Evidence

Plate `a0ff7b18-...`, video `50057921-...`, face query `c23c3e10-...`, pose
`3fff8296-...`, blur `769f6124-...`, different subject `273bc132-...`, no-face
`3ffc0eae-...`.

## Exact Steps

Use the guide’s **Core media pipeline demonstration** section.

## Exact Queries

Use only the verified `MN1367`, `ZZZ9999`, face rank, SigLIP rank and explicit
image-pair operations recorded above.

# P0

`0`.

# P1

`0 foundational`. The requested core media presentation and bounded operations
are working.

# P2

- In-app browser codec playback is unsupported for the retained audio/video;
  authenticated full/range delivery is operational.
- The approved video is a truthful zero-ANPR control, not a positive video
  plate demo.
- The manifest contains no positive exact/near-duplicate pair.
- One unrelated pre-existing Windows/sandbox backend-upgrade route fixture fails
  while 18 route specs pass.

# P3

`0 required for the product demo`.

# RetainedMutationNeeded

`NO`.

# DeploymentNeeded

`NO` — the narrow LocalAI/API image is already deployed. Rollback tag exists.

# DatabaseMigrationNeeded

`NO`.

# ModelDownloadNeeded

`NO`.

# Files Changed

- `core/http/react-ui/src/analyst/analystMediaPresentation.js`
- `core/http/react-ui/src/analyst/analystMediaPresentation.test.js`
- `core/http/react-ui/src/analyst/AnalystData.jsx`
- `core/http/react-ui/src/analyst/AnalystPortal.css`
- `core/http/react-ui/e2e/analyst-portal.spec.js`
- `core/http/react-ui/src/utils/api.js`
- `core/http/endpoints/localai/forensic_cases.go`
- `core/http/endpoints/localai/forensic_cases_test.go`
- `core/http/routes/records.go`
- `core/http/routes/localai.go`
- `core/http/auth/features.go`
- `docs/content/features/records-intelligence.md`
- `swagger/docs.go`, `swagger/swagger.json`, `swagger/swagger.yaml`
- `docs/demo/nexusai-breadth-multimodal-demo-guide.md`
- `NEXUSAI_CONTINUATION.md`
- this acceptance report

# Exact Team-Lead Demo Steps

Open the workspace Data page; show the `MN1367` overlay/card/crop/citation and
run the existing cited Ask; show the video complete-zero plate timeline plus
0 s/5 s OCR and Urdu transcript; show A-frontal face crop and bounded SFace
ranking; show SigLIP ranking and A-vs-pose comparison; open the no-face negative;
then reopen `MN1367` in History and navigate its source citation.

# Exact Next Phase

Do not broaden the current demo. If separately approved later, the smallest
next evidence phase is one lawful, ground-truthed positive video-ANPR control
and one lawful positive exact/near-duplicate image pair under a supplemental
retained mutation manifest. Cleanup remains separately gated.
