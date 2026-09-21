# NexusAI R8 T2 one-session controlled capture runbook

Status: tooling source accepted; evidence deferred_environment_unavailable; optional future T2-P only  
Target: 32 controlled images in one 60–90 minute session  
Contract: `forensics.controlled-demo-pack/v1`

This physical T2-P workflow is preserved for a future suitable environment. It
is not the active Boundary-A dependency and the product owner should not resume
it now. The existing empty pack remains an unused workspace and must never be
mixed with generated T2-V evidence.

## Safety boundary

Use only owned or explicitly consented scenes and synthetic, non-road-use test
identifiers. Never mount a deceptive test target on a vehicle used on a public
road. Exclude unrelated people, real third-party plates, addresses, house
numbers, sensitive documents and private screen content. This pack is T2
controlled evidence, never T4 operational evidence.

## Before capture

1. Print or display clearly marked `NEXUSAI TEST — NOT VALID FOR ROAD USE`
   Pakistan-shaped targets using invented identifiers. Include Latin and Urdu
   script targets, but do not copy a real registration.
   A printable repository-owned sheet is provided at
   `tools/r8_t2_capture/printable-test-targets.html`.
2. Prepare one owned/consented vehicle or a vehicle-like board/background.
3. Prepare harmless negative objects: logo, sign, rectangular label, blank
   vehicle body, document-shaped blank page, non-private test screen, random
   text and background structure.
4. Use landscape camera orientation. Disable cloud synchronization if the pack
   must remain local. Do not apply beauty/AI enhancement filters.
5. Ask two people to act as annotator and independent reviewer. Neither may see
   model output before the manifest is sealed.

## Create the local workspace

Use a path outside Git and outside production evidence storage:

```powershell
$PackRoot = 'C:\NexusAI-Evaluation\R8\T2\1.0.0'
python .\scripts\prepare_nexusai_r8_t2_capture_pack.py --output-dir $PackRoot
```

Expected marker:

```text
"status": "pass"
"planned_images": 32
```

The generated `capture-plan.json` gives every exact filename and condition.

### Optional local camera assistant

The assistant automates plan guidance, exact JPEG naming, progress and direct
folder writes. It cannot automate physical scene arrangement or independent
ground truth; replacing real camera optics with generated screenshots would
remain T1, not T2.

Start a local-only server from the repository:

```powershell
python -m http.server 8778 --bind 127.0.0.1
```

Then open in Chrome or Edge:

```text
http://127.0.0.1:8778/tools/r8_t2_capture/capture-assistant.html
```

Load `$PackRoot\capture-plan.json`, select `$PackRoot\images`, allow camera
access, and use **Capture and save current slot**. Stop the server with `Ctrl+C`
after all 32 files are saved. The assistant performs no upload, inference,
normalization or annotation.

For every slot, follow the assistant's **How to arrange this slot** card from
top to bottom. Positive slots also show the exact synthetic target to use. A
target may be printed/cut out or displayed full-screen on a separate phone,
tablet or monitor facing the camera. The saved evidence must still be a live
camera observation; do not save a browser screenshot or copy the target image
directly into the pack.

For slot 1, use `NXA-T01`, keep the target and camera parallel, center the full
target border, make it occupy roughly 55–75% of the camera-frame width, use
indirect daylight and wait for focus. Then capture once. The assistant advances
to slot 2 and changes the arrangement card automatically.

The camera diagnostic must show a device label and non-zero resolution before
capture is enabled. If the preview remains gray, uncover the privacy shutter,
close Windows Camera/Teams/Zoom and other camera users, verify Chrome/Edge site
permission for `127.0.0.1`, then press **Start camera** again. A missing
`favicon.ico` request in the local server console is harmless.

## Capture matrix

Capture exactly 32 images:

- 18 positive/controlled-target images: frontal, horizontal/vertical angle,
  near/medium/far, daylight/shadow/glare/low light, sharp/slight blur/motion
  blur, small/large/edge targets, multiple targets, Latin and Urdu targets.
- 10 detector negatives: vehicle body, logo, sign, rectangular label, blank
  document, non-private test screen, random text, vehicle without target,
  background structure and plain scene.
- 4 sealed holdout images: daylight target, angled target, owned-vehicle low
  light and a text-like negative. Do not tune against these four images.

Name each file exactly as shown in `capture-plan.json`, for example:

```text
images/t2-001-single-frontal-day-near.jpg
images/t2-019-negative-vehicle-body.jpg
images/t2-029-holdout-single-day.jpg
```

Preserve the camera original. If EXIF/GPS or privacy content must be removed,
create a sanitized derivative, retain the original separately, and record its
parent SHA-256 in `parent_original_sha256`.

Before annotation, verify that all exact planned files exist and that none is
empty, signature-mismatched, unexpected or byte-for-byte duplicated:

```powershell
python .\scripts\check_nexusai_r8_t2_capture_files.py `
  --pack-root $PackRoot `
  --output "$PackRoot\capture-status.json"
```

Continue to annotation only when the command reports:

```text
"status": "pass"
"capture_complete": true
"planned_count": 32
"accepted_file_count": 32
"development_file_count": 28
"holdout_file_count": 4
```

## Independent ground truth

For each image, populate `manifest.json` with dimensions, orientation,
condition fields, plate presence/count, original-pixel polygons, raw and
normalized transcription, script/style, negative class, ownership and privacy
review. Record:

```json
"ground_truth_review": {
  "annotator": "person-a",
  "annotated_at": "ISO-8601 timestamp",
  "reviewer": "person-b",
  "reviewed_at": "ISO-8601 timestamp",
  "reviewer_independent": true,
  "model_output_consulted": false,
  "disagreements_resolved": true
}
```

Slots 1–28 use `evaluation_partition: development`. Slots 29–32 use
`evaluation_partition: holdout`. Do not reveal holdout model results during
tuning.

## Validate and seal

```powershell
python .\scripts\validate_nexusai_r8_t2_pack.py `
  "$PackRoot\manifest.json" `
  --asset-root $PackRoot
```

Acceptance requires:

```text
"status": "pass"
"image_count": 32
"acceptance_ready": true
"development": 28
"holdout": 4
"errors": []
```

Any hash mismatch, unsafe path, missing asset, privacy failure, model-influenced
ground truth or inconsistent plate count is a failure. Do not run candidates
until the pack passes and the holdout partition is sealed.

## Reuse boundary

After acceptance, the immutable pack may support R8 detector/OCR comparison,
R8 regressions, the non-production team-lead demonstration, relevant R9 image
regressions and R17 release regression. Any content change creates a new pack
version and manifest hash.
