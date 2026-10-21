# Capture and labelling kit (decisions D26 and D10, 2026-10-21)

The test sets that decide the media tracks cannot be made by a model. This page is what the named people need in order to start. Each set is **sealed in part**: a development part may be used to tune, a validation part to choose, a sealed part is opened once at the gate and never tuned on. Nothing here is a real-data file; real captures stay in the vault under `docs/data/real-data-handling-policy.md`.

## Who does what

| Role | Does | Time to start | Needed for |
|---|---|---|---|
| Capture lead (one person) | Captures plates and short videos under authority; keeps the capture log | 2 days for the first 100 images | Plates (tracks P, V) |
| Plate annotator and a second reviewer (two different people) | Draw the plate box in original pixels and type the exact visible text, without looking at any model output; the reviewer checks privacy and ownership | 1 minute per plate | Plates |
| Urdu reader (one native reader) | Types the exact text of Urdu lines from images (newspapers, screenshots, social posts, documents), with Latin digits kept as they appear | 500 lines in about 3 days | Track O |
| Speakers and transcribers (per language: Urdu, Punjabi in Shahmukhi, Pashto, Roman Urdu, English with a Pakistani accent) | Record consented read speech and short conversations; transcribe a second person's recording, never their own | 2 hours of audio per language to start | Track A, languages PL |
| Investigator (one analyst) | Writes about 50 real questions about a case, with the answer the analyst expects and which questions the case cannot answer | 1 to 2 days | E1, the honest accuracy number |

## Plates (tracks P and V)

* **Capture protocol:** `docs/data/nexusai-evidence-test-and-demo-program.md`, "R8 T2-P protocol". In short: only owned or consented vehicles and plates, or a deliberately synthetic plate; keep people, homes, documents and screens out of the frame; original bytes copied untouched, SHA-256 recorded before anything is renamed or cropped; EXIF and GPS reviewed before sharing.
* **Diversity to cover:** frontal daylight, angle, near and far, small and large plates, partial occlusion, shadow, glare, low light, controlled motion, edge framing, portrait and landscape, several vehicles. A short 20 to 60 second video of the same vehicles passing a fixed point, for the video track.
* **Split before looking:** about 60% development, 20% validation, 20% sealed, decided by the capture lead by session (not by image), and written in the manifest before annotation starts.
* **Annotation:** original-pixel polygon, exact text, visibility notes (blur, glare, partial), the annotator's name. Template: `configuration/nexusai_r8_t2_manifest_template.json`; validator: `scripts/validate_nexusai_r8_t2_pack.py`.
* **Target:** at least 300 sealed plate tracks across day, night, rain, blur and angle (master plan 8.13).

## Urdu lines (track O)

* **Sources that are allowed:** images we generated ourselves from openly licensed fonts and our own text; screenshots of text the reader wrote; newspaper or document images only where their licence allows reuse (each source is a ledger row first, and the owner approves each download, D4).
* **Transcription rules (written before the first line):** keep the characters as printed; do not normalise letter forms or digits; mark unreadable characters with a fixed symbol, never a guess; Latin digits stay Latin; one line per record with the image name; a second reader re-types a 10% sample, and disagreements are listed, not silently fixed.
* **Size:** 500 to 1,000 lines to start, split development, validation and sealed by source image.

## Speech (track A and languages)

* **Consent (text for the form, to be reviewed by counsel):** I agree that my recording will be used only to test and improve this product's speech recognition on this organisation's own machines; it will not be shared outside the organisation; I may withdraw before the sealed set is built. The speaker's name is kept apart from the audio.
* **Recordings:** a phone and a quiet room for read speech; the same sentences again through a phone call (a call to a second phone, recorded at the far end) so telephone-band audio is real, not simulated; digit strings read aloud (phone numbers, plates, amounts) because exact digits are the evidence; a few two-person conversations.
* **Transcripts:** written by a native speaker who did not record it; the normalisation rules (Urdu letter forms, spaces, digit words, Roman Urdu spelling) are written first and kept with the set; a time stamp per sentence is enough to start.
* **Size:** 2 to 5 hours per language, with a sealed part.

## Investigator questions (E1)

* Write the questions the way an analyst would ask them, about one real or generated case, in English first and then in Urdu or Roman Urdu where natural. For each: the answer expected, where the analyst would look, and **which questions the evidence cannot answer** (these matter as much, because a wrong answer costs more than a refusal).
* Do not look at the engine's answers while writing. About 50 questions; the engine is scored once, afterwards.

## Reviewers (D10)

A reviewer is a native speaker who can read Urdu and, for the second and third languages, Punjabi in Shahmukhi or Pashto script. They write the 200 questions per language, check the synonym lists, label the sets above, and judge the answers' wording. Please send names and the languages each can cover; the lead time of this work is the longest of any item in the plan.
