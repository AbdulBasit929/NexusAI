# NX-MMR ground-truth manifest protocol

Status: `COMPLETE_LOCKED_HUMAN_GOLD` (2026-08-30).

No ANPR, OCR, ASR, embedding, reranker or other model was called while these
packs were prepared. Earlier candidate outputs were neither read nor copied
into a label. Private derivatives are under the Git-ignored
`local-acceptance-models/nxmmr/private-ground-truth` tree and must not be
committed or published.

## Pakistani image pack

- source: exact authorized directory, 108 JPGs, 50,377,210 bytes;
- template: `private-ground-truth/images/image-ground-truth-template.csv`;
- contact sheets: six pages in `private-ground-truth/images/image-contact-sheets`;
- original split: 86 development candidates plus 22 sealed holdout
  (20.370%); bounded active slice: 10 frozen development plus all 22 holdout;
- split seed: `NXMMR-20260829-SEALED-SPLIT-V1`;
- split digest: `16b950bbf79780a227239d790a9e1a7d9192555db4958d99278e2110b8206d2d`;
- pack inventory digest: `784225be3a265d14c752fa22d69f913eaa3cb9d911e98947cc8d74625c05613a`.

Every row contains sample ID, relative path, byte size, SHA-256, dimensions,
split, plate presence/count/text/readability/province, angle, blur, occlusion,
lighting, multiple vehicles, hard negative, privacy, review status, reviewer,
review timestamp, label source, whether model output was seen, and notes.

## sample.mp4 pack

- source: 184,407,144 bytes; SHA-256
  `d470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9fafc83762d137ee`;
- media: 60.010 seconds, H.264 3840×2160 at 60 fps, AAC 48 kHz stereo;
- template: `private-ground-truth/video/video-ground-truth-template.csv`;
- review frames: 30 frames at two-second coarse intervals;
- contact sheets: two pages in `private-ground-truth/video/video-contact-sheets`.
- review audio: `private-ground-truth/video/review-audio.m4a` (1,039,829
  bytes, SHA-256
  `2d03e74d896316837e7fba6ca35b95a171573b0814cbec8c047529ad323558a6`),
  copied losslessly from the source audio stream for private transcription.

Every interval contains start/end source seconds, review frame, plate
visibility/count/text/readability, multiple vehicles, hard-negative and
explicit-negative flags, speech presence, verbatim raw transcript, transcript
language, review status/reviewer/time, label source and no-model-output flag.
Positive plate or speech intervals must be refined against the original video
to exact first/last source times; the coarse frame time is not automatic truth.

## Completed human actions

The independent reviewer completed and froze 10 development images before the
22-image sealed holdout opened, attested that model output was unseen for every
active image, watched the full original video, recorded 19 plate events, and
reviewed all six speech windows. All speech windows were negative, so no
transcript was invented. Finish locked the state at 2026-08-30T06:20:27Z and
wrote `GroundTruthValidation=PASS` with no validation errors.

Implementation and test detail is in
`reports/nexusai-nxmmr-human-verification-workflow-20260829.md`.

`LocalImageGroundTruth=PASS_LOCKED_32`

`SampleVideoGroundTruth=PASS_LOCKED_19_EVENTS`

`SampleAudioTranscriptEvidence=INSUFFICIENT_ALL_6_WINDOWS_NEGATIVE`

`NoSelfOracle=PASS`
