# NexusAI Breadth Retained Acceptance Pack — Preparation Gate

Status: `EXACT_INPUTS_PREPARED_ACCEPTED_ASR_DEPLOYED_OPERATOR_APPROVAL_REQUIRED`

`ASRModelExpected=faster-whisper-small-ur`  
`KnownUrduLanguagePolicy=explicit_ur_when_known`  
`IdentifierSpeechLimitation=PENDING_M2`

This document defines one isolated, approval-gated breadth pack. It does not
authorize upload, registration, processing, reprocessing, deletion, or cleanup.
No listed file has been copied into NexusAI retention by this preparation.

## Scope

- tenant: `default`
- collection: `nexusai-breadth-acceptance-20260820`
- case: `nexusai-breadth-acceptance-20260820`
- actor/subject: `nexusai-breadth-acceptance-operator`
- jurisdiction: `PK`
- source timezone: `Asia/Karachi`
- expected mandatory inputs/evidence registrations: 7
- expected queued media jobs: 6 (ANPR image, general image, three audio, video)
- expected document state: registered/KB-mirrored without a media-worker job;
  `document_extraction_pending` remains truthful until Documents basic M1
- retention: keep until the breadth verdict and export/checksum manifest are
  reviewed; deletion requires a separate explicit cleanup approval

The isolated scope prevents acceptance material from being mixed into
`nexusai-forensic-demo` or `nexusai-runtime-acceptance-20260730`.

## Exact candidates

### ANPR / image

- path: `C:\Users\sheik\Workspace\Personal\FastPlateOCR\test_plate.jpg`
- size: `235572` bytes
- SHA-256: `a15a7ec947fdb2c5a08a70243a62ed66e4dddd89defaba9d135f6c5ac67559f6`
- declared type: `auto` (content classification must select raster image)
- visual truth to confirm manually before approval: plate `MN1367`
- expected processor: image technical observation plus FastALPR
- expected persistence: one evidence/version/job; technical and ANPR artifacts;
  one canonical, manual-review-required ANPR observation
- expected Ask: `Show the evidence and cited source location for plate MN1367.`
- limitation: integration reference only; not Pakistan plate-accuracy certification
- approval prerequisite: the operator must confirm this external-workspace image
  is user-owned or otherwise lawfully usable; it was not copied or modified

### General Pakistan image

- path: `C:\Users\sheik\Workspace\Office\Projects\NexusAI\local-acceptance-inputs\downloads\wikimedia-road-cars-islamabad.jpg`
- size: `1430057` bytes
- SHA-256: `ba1aa5a7e28e18bdd78b2cba132a6fe1cd87612d930be91fa6db22194cae0038`
- source: `Road and cars in Islamabad.jpg`, own work by Executioner
- license: CC BY-SA 3.0 Unported; attribution and change indication required
- expected processor: image technical observation plus optional FastALPR
- limitation: visibly meaningful Pakistan image, not a plate-accuracy golden

### Document

- path: `C:\Users\sheik\Workspace\Office\Projects\NexusAI\HP_EliteDesk_LocalAI_Remote_Setup_Guide.docx`
- size: `34352` bytes
- SHA-256: `85b1ab53150a556f84391a222b7fb4dbf69aa80d1c4d7d7a7b54d5f283ea4865`
- declared type: `auto`
- expected processor: current document/KB intake path, not the media worker
- expected persistence: one evidence item and a cited KB source entry where the
  deployed document path supports this format; no ingest job is expected
- expected Ask: `Summarize the remote setup guide and cite the source section.`
- limitation: English functional document; not Urdu/scanned-document acceptance

## Acquired ignored media inputs

The exact acquired files, provenance, component licenses, hashes, source
metadata, and conversion commands are in
`local-acceptance-inputs/ACQUISITION_MANIFEST.json`. The operator must review
the media and source-provided/synthetic truth before approving retained use.

### Clear Urdu

- audio: `C:\Users\sheik\Workspace\Office\Projects\NexusAI\local-acceptance-inputs\pakistan-audio\clear-urdu.wav`
- size/SHA-256: `199758` bytes / `c499e06e26b8cbf37386b88bba26f37045439a775e964bdad8634376435a056e`
- source transcript: same directory, `clear-urdu.ground-truth.txt`
- duration/license: 6.24 seconds / Google FLEURS CC BY 4.0
- expected processor: FFprobe then the accepted
  `faster-whisper-small-ur` deployment
- expected persistence: evidence/version/job, technical observation, timestamped
  transcript artifacts, KB source/asset when mirroring succeeds
- expected Ask: one Urdu-script query and one Roman-Urdu equivalent based on the
  human-confirmed major phrase

### Conversational Urdu

- audio: `...\pakistan-audio\conversational-urdu.wav`
- size/SHA-256: `336078` bytes / `f016076b153c2ed6be21fa5f7a4bbc22b3e391d40080b9fa3de3b8a2dd517550`
- source transcript: `...\pakistan-audio\conversational-urdu.ground-truth.txt`
- duration/license: 10.50 seconds / Google FLEURS CC BY 4.0
- expected path/artifacts: same governed Audio processor contract
- classification: second-speaker scripted/read speech; not spontaneous conversation

### Urdu-English mixed with identifier

- audio: `...\pakistan-audio\urdu-english-identifier.wav`
- size/SHA-256: `248992` bytes / `606ffc7e281940b1b175fff98169a83a6a6ab7744ffd49f05849ac94318fd314`
- script truth: `...\pakistan-audio\urdu-english-identifier.ground-truth.txt`
- duration: 7.779 seconds
- classification: synthetic sequential pipeline test, not natural code-switch
- required truth annotation: exact spoken identifier, number, date, time, or
  plate separated from the prose transcript
- expected Ask: Urdu, Roman Urdu, and English queries containing the exact
  identifier; no fuzzy identifier rewriting

### Pakistani English — deferred

`PakistaniEnglishNaturalSample=PENDING`. No bounded source with documented
speaker locale was found, and no nationality or accent was inferred. This slot
is not part of the proposed seven-input retained pack.

### Video

- video: `C:\Users\sheik\Workspace\Office\Projects\NexusAI\local-acceptance-inputs\pakistan-video\pakistan-short-video.mp4`
- size/SHA-256: `477174` bytes / `94be8b97d62f28eb0349b1eaea02e8073a669d60ffbd3da6511f7f7d27004a26`
- source speech transcript:
  `...\pakistan-video\pakistan-short-video.ground-truth.txt`
- duration: 10.50 seconds
- classification: derived composition; Islamabad CC BY-SA 3.0 image plus
  FLEURS CC BY 4.0 audio; preserve both attributions
- expected processor: FFprobe, bounded frames, image observations, ANPR where
  present, embedded-audio extraction, shared Audio/ASR processor
- expected persistence: one evidence/version/job plus video, frame, ANPR,
  embedded-audio and transcript artifacts as applicable
- expected Ask: cite a frame timestamp and a transcript time range without
  claiming continuous tracking or perfect transcription

## Mandatory pre-approval checks

For every supplied audio/video input record the exact path, byte size, SHA-256,
duration, language profile, lawful-use confirmation, and human-reviewed truth.
The operator must listen to each sample. Whisper output, an LLM transcript, or
generated subtitles cannot serve as ground truth.

The non-retained `whisper-tiny` run was operational but failed practical Urdu.
The authorized local `Systran/faster-whisper-small` challenger passed M1: with
forced `ur`, clear/second-read WER/CER improved to 0.6000/0.1605 and
0.2917/0.1325, meaning was preserved, no major omission/hallucination was found,
and timestamps remained usable. The synthetic fixture preserved phone and time
but not plate, so identifier maturity remains M2. Automatic detection chose
Hindi; the accepted deployment must preserve explicit Urdu guidance. Full raw
evidence is in the challenger acceptance report and ignored result JSON.

For each Pakistan audio sample the acceptance sheet must record:

- `TranscriptUsability=USABLE|PARTIAL|UNUSABLE`
- `MeaningPreserved=YES|PARTIAL|NO`
- `MajorContentOmitted=YES|NO`
- `MajorHallucinatedContent=YES|NO`
- `IdentifiersPreserved=YES|PARTIAL|NO|N/A`
- `TimestampUsable=YES|NO`
- `Searchable=YES|NO`

## Expected mutation

On later explicit approval, seven uploads are expected to create up to seven
evidence identities and six media processing jobs in the isolated case. The
document is expected to remain a registered/KB-mirrored document pending the
later basic Documents M1 extraction slice. Media processing
will write derived artifacts; ANPR detections may also write governed canonical
ANPR rows. KB mirroring may create collection entries/assets. Exact counts must
be derived from returned evidence IDs and reconciled before/after rather than
assumed from model output.

## Pass boundary

- all jobs terminate without dead-letter/P1 failure;
- case/tenant/collection scope is exact;
- raw inputs remain immutable and cited;
- all model observations require manual review;
- ANPR/image/audio/video details are visible through Data;
- KB assets and Ask citations resolve where promised;
- clear Urdu quality is graded against human-reviewed truth and must meet the
  accepted small-model, explicit-Urdu envelope rather than the tiny baseline;
- identifiers are explicitly compared with human truth;
- no unsupported identity, ownership, presence, intent, guilt, route, or
  continuous-tracking claim is made.

## Current gate

`RetainedDataMutationNeeded=YES`

`AcceptedASRDeploymentNeeded=NO`

`OperatorApprovalRequired=YES_AFTER_MEDIA_TRUTH_LICENSE_AND_TEST_PLATE_OWNERSHIP_REVIEW`

Do not execute uploads yet. The bounded accepted-ASR deployment is verified.
The operator must now confirm review of
the three audio clips, video, their truth sidecars and license notices; confirm
that the external `test_plate.jpg` is lawfully usable; and explicitly approve
the seven-input retained upload/processing in the isolated scope.
