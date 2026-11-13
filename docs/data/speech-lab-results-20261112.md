# Speech lab results, 2026-11-13 (WP-M1)

Plan, gates and every change before measuring: `speech-lab-preregistration-20261112.md` (amendments A1 to A9). Tools: `tools/media_data/speech_lab.py`, `omni_ctc.py`, `speech_report.py`, `speech_gate.py`. Result files: `C:/tmp/nexusai-media-data/results/asr-lab/` (outside the repository; they hold public speech only). Evidence tier **T3, public data**: nothing here measures the owner's own audio, telephone-band speech or noise.

## 1. The outcome

| Language | Route | Status earned | Why |
|---|---|---|---|
| **Pashto** (Central, Southern, Northern) | Omnilingual CTC 300M, converted and run in the worker's own process, audio cut at quiet points | **SEARCH_LEAD** | sealed, three spontaneous sets pooled (120 clips): word error 30.3%, character error 10.7%, word recall 64.9%, loops 0.8%, real-time factor 0.31. Not MACHINE_TRANSCRIPT: the read-speech set (FLEURS) scores 44.9% word error against a gate of 30% |
| **Pakistani Punjabi** (Shahmukhi) | the same model | **LOW_RESOURCE_LANGUAGE** | development: character error 39.8% against 20%, word recall 36.1% against 50%; a sealed read could only confirm a miss of that size, so it was not made (A9) |
| **Urdu** | stays on the live Whisper large-v3-turbo | unchanged (machine transcript, review required) | 23.4% word error on 40 development clips; the Omnilingual model writes Urdu in Devanagari (100.3%) and, held to the Arabic script, leaves 15 of 40 clips empty (92.7%): no challenger |
| **English** | stays on the live Whisper model | unchanged | October figure 4.7% word error; not re-run (A2) |
| **Saraiki** | none | not supported | no Saraiki audio on this machine; the public Omnilingual corpus has none |

A "search lead" is text good enough to find a recording by a spoken word, **not a transcript**: about two spoken words in three are found in the text (64.9%) and about three words in ten are wrong (30.3%). The interface says so; every segment is a model observation that a person reviews.

## 2. Development runs (decisions were made on these)

| Set | Part | Model | Told | Clips | Minutes | Word error [95%] | Char error | Word recall | Script | Loops | Empty | RTF |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| pbt_Arab | dev | omni-ctc-300m-hf | ps, 38 s pieces | 20 | 9.7 | 32.6% [28.5, 36.8] (N2) | 12.5% | 62.5% | 100% | 0 | 0 | 0.34 |
| pbt_Arab | dev | ps-small-cv24-ct2-int8 | ps (no timestamps) | 40 | 19.5 | 75.6% [73.5, 77.8] (N2) | 67.5% | 27.4% | 100% | 0 | 0 | 0.13 |
| pbu_Arab | dev | omni-ctc-300m-hf | ps, 38 s pieces | 20 | 7.7 | 27.8% [23.9, 31.6] (N2) | 9.9% | 68.3% | 100% | 0 | 0 | 0.36 |
| pbu_Arab | dev | ps-small-cv24-ct2-int8 | ps (no timestamps) | 40 | 16.0 | 74.3% [69.6, 78.6] (N2) | 66.1% | 26.9% | 100% | 0 | 0 | 0.15 |
| pnb_Arab | dev | omni-ctc-300m-hf | pnb, 38 s pieces | 20 | 19.5 | 67.0% [60.7, 72.8] (N2) | 39.8% | 36.1% | 100% | 0 | 0 | 0.31 |
| ps_af | dev | omni-ctc-300m-hf | ps, 38 s pieces | 40 | 7.9 | 44.3% [40.1, 48.9] (N2) | 17.8% | 49.3% | 97% | 0 | 0 | 0.32 |
| ps_af | dev | ps-small-cv24-ct2-int8 | ps (no timestamps) | 40 | 7.9 | 52.6% [45.6, 59.0] (N2) | 34.9% | 48.6% | 100% | 0 | 0 | 0.29 |
| ps_af | dev | ps-small-cv24-ct2-int8 | ps (no timestamps), 12 s pieces | 40 | 7.9 | 45.6% [40.4, 50.8] (N2) | 24.0% | 56.8% | 100% | 0 | 0 | 0.40 |
| pst_Arab | dev | omni-ctc-300m-hf | ps, 38 s pieces | 40 | 20.5 | 26.2% [23.0, 29.7] (N2) | 8.6% | 69.8% | 100% | 0 | 0 | 0.35 |
| pst_Arab | dev | ps-small-cv24-ct2-int8 | ps (no timestamps) | 40 | 20.5 | 74.9% [71.2, 78.5] (N2) | 66.8% | 27.4% | 100% | 0 | 0 | 0.14 |
| pst_Arab | dev | ps-small-cv24-ct2-int8 | ps (no timestamps), 12 s pieces | 40 | 20.5 | 30.8% [25.5, 36.7] (N2) | 16.1% | 71.9% | 100% | 1 | 0 | 0.36 |
| ur_pk | dev | faster-whisper-large-v3-turbo-ct2 | ur | 40 | 7.0 | 23.4% [19.5, 27.1] (N1) | 9.0% | 75.2% | 100% | 0 | 0 | 0.88 |
| ur_pk | dev | omni-ctc-300m-hf | ur, 38 s pieces | 40 | 7.0 | 100.3% [100.0, 100.8] (N1) | 104.5% | 0.0% | 0% | 0 | 0 | 0.27 |
| ur_pk | dev | omni-ctc-300m-hf | ur, 38 s pieces | 40 | 7.0 | 92.7% [86.7, 97.8] (N1) | 84.8% | 6.1% | 62% | 0 | 15 | 0.29 |

Rows with "no timestamps" are the Pashto fine-tune of Whisper small (see section 5). Rows with "38 s pieces" are the Omnilingual model (audio cut at the quietest point so that no piece is over its 40-second limit). The two `ur_pk` Omnilingual rows are the un-constrained (Devanagari) run and the run held to the Arabic script; the first constrained run, which had a bug (Devanagari vowel signs are Unicode marks, not letters), is kept as a `.faulty` file and is not in the table.

## 3. Sealed runs (read once)

| Set | Part | Model | Told | Clips | Minutes | Word error [95%] | Char error | Word recall | Script | Loops | Empty | RTF |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| pbt_Arab | sealed | omni-ctc-300m-hf | ps, 38 s pieces | 40 | 20.3 | 33.9% [30.9, 37.0] (N2) | 13.0% | 60.3% | 100% | 1 | 0 | 0.32 |
| pbu_Arab | sealed | omni-ctc-300m-hf | ps, 38 s pieces | 40 | 17.6 | 28.8% [25.0, 32.4] (N2) | 11.2% | 66.9% | 100% | 0 | 0 | 0.30 |
| ps_af | sealed | omni-ctc-300m-hf | ps, 38 s pieces | 40 | 9.1 | 44.9% [41.0, 49.0] (N2) | 21.1% | 48.4% | 100% | 0 | 0 | 0.28 |
| pst_Arab | sealed | omni-ctc-300m-hf | ps, 38 s pieces | 40 | 19.5 | 27.4% [24.1, 30.6] (N2) | 7.7% | 68.3% | 100% | 0 | 0 | 0.32 |

Gate evaluation by `speech_gate.py` (every line printed, none judged by hand):

```
model omni-ctc-300m-hf, sealed runs: ['pbt_Arab', 'pbu_Arab', 'ps_af', 'pst_Arab']
  read set word error (N2) <= 30%                      44.9%            NOT MET
  pooled spontaneous word error (N2) <= 45%            30.3%            met
  pooled spontaneous character error (N2) <= 20%       10.7%            met
  pooled spontaneous word recall >= 50%                64.9%            met
  script share >= 99% (pooled spontaneous and read)    100.0% / 99.7%   met
  loops <= 1% of clips (pooled spontaneous)            0.8%             met
  real-time factor <= 1.5                              0.31             met
STATUS for Pashto: SEARCH_LEAD
```

## 4. Runs that were NOT made, and why (owner's direction, 2026-11-13: no run that cannot change a decision)

* The live model (`turbo`) on Pashto sets and on `pnb_Arab` (A2, A6): its Pashto failure, 96% word error in the wrong script, was measured on 2026-10-22; on Punjabi it writes Gurmukhi when told `pa`.
* `turbo` on English (A2): the October figure stands.
* The Pashto fine-tune on `pbt_Arab` and `pbu_Arab` in pieces (A6), and any sealed run of it: Omnilingual was better on every spontaneous set (character error 8.6%, 12.5%, 9.9% against 16.1% for the fine-tune in pieces on the one set where both ran) and is one model for every language.
* Sealed reads for Urdu and Punjabi (A9): the development miss is decisive.
* The full baseline of the registered plan: about 1.5 hours were run instead of about 4, then the Omnilingual development and sealed runs.

## 5. Findings worth keeping

1. **The Pashto fine-tune of Whisper small (Common Voice 24) was trained without timestamps.** In faster-whisper's default mode it returned an empty text for 2 of 3 clips; read with `without_timestamps=True` it reads. It also writes only about 38% of the words of an utterance longer than the 3 to 6 seconds it was trained on (word error 74.9% to 75.6% whole-clip on spontaneous Pashto); cut into 12-second pieces at quiet points it reads 30.8% (Central Pashto) and 45.6% (FLEURS read). So it needs the sound-based cutter anyway, and then it is still beaten by the Omnilingual model.
2. **The Omnilingual CTC 300M checkpoint runs on this CPU with the packages already in the worker image** (torch, transformers `Wav2Vec2ForCTC`), after a conversion written for this project: 423 of 423 tensors placed, a vocabulary of 9,812 pieces equal to the CTC outputs, read at 0.3 of real time. The one public converter needs fairseq2 and has no licence file, so it was read and not used.
3. **A CTC model takes no language.** Spoken Urdu comes out in Devanagari (Hindi script); holding the output to Arabic-script letters does not give Urdu, it empties the readings. Pashto and Punjabi are unaffected. Urdu stays on Whisper, which the worker already re-reads as Urdu when it hears Hindi.
4. **Read speech is harder than conversation for these models** (Pashto FLEURS 44.9% word error; spontaneous Pashto 30.3%): the FLEURS sentences are technical and written. A gate set on read speech alone would have been the wrong gate for this product; both are reported.
5. **The loop rule (at most 1% of clips) cannot be met by a set of 40 clips with a single loop (2.5%).** On the pooled 120 clips one loop is 0.8%, which is why the gate is evaluated pooled.
6. **Pakistani Punjabi (Shahmukhi) is out of reach for a 300M CPU model**: 39.8% character error (the published figure for Meta's 7B model with an LLM decoder is 31%). It needs fine-tuning on Shahmukhi speech or a much larger model.
7. **Two environment traps:** Docker lost its connection once during a run ("unexpected EOF"); the laptop slept 16 hours during the bank run and recorded 57,729 s for one answer (that question was asked again).

## 6. Limits (said plainly)

Public speech only (T3). The Omnilingual figures are on the Omnilingual corpus, which the model was trained on (its train split), so they are optimistic; the Whisper fine-tune was trained on Common Voice, not on this corpus. Sets of 20 to 40 clips (intervals are given). Not measured: telephone-band or noisy audio, code-switching, long recordings, the owner's own speech. Normalisation `N2` (Arabic-script letter unification) is the primary figure for Pashto and Punjabi, `N1` for Urdu and English; both are in the result files.

## 7. What changed in the repository

The route table `ingestion/forensic_records/speech_routes.json` now carries the measured status of each route. The worker wiring (`speech_routes.py`, the one-line patch of `media_pipeline.py`, the overlay build, the compose file and the recreate script) is written and tested and **not applied to the live worker**. Applying it needs the owner's word and the model folder installed (`tools/media_data/Install-SpeechRouteModel.ps1`).

## 8. Applied to the live worker (2026-11-13, on the owner's word "Yes, apply it")

* **Image:** `nexusai-forensic-records-worker:speech-routes-20261113` (`f32bd367b972`), built on top of the running worker image (`e70bf7e28b6e`, the artifact-scope overlay); **rollback tag `before-speech-routes-20261113` = `e70bf7e28b6e`**; one command: `tools/dev/live-window/8-recreate-worker-speech-routes.ps1 -Rollback`. Four settings added (`FORENSIC_ASR_ROUTES_ENABLED=true`, route table path, 4 threads, 1500 s budget); nothing else changed (video plate step V4 and the 2400 s lease are as before). The dry run reproduced the running worker from six compose files before the switch; the build smoke test (compile, import, table load) passed; the model folder was installed with a verified SHA-256.
* **Live check on a throwaway collection `speech-routes-live-test-20261113`** (three public clips through the normal upload path, one per path, all three jobs finished in under two minutes, no error in the worker log):

| Clip | Requested language | Stored segments | Route and label |
|---|---|---|---|
| Central Pashto, 39 s | `ps` | 2: 0.0 to 33.13 s and 33.13 to 38.96 s (cut at a quiet point), text in Pashto script matching the lab's reading | Omnilingual route, **SEARCH_LEAD**, measured recall 0.649 carried on each segment, timing "reported", review required |
| Western Punjabi (Shahmukhi), 47 s | `pa` | 2: 0.0 to 34.81 s and 34.81 to 47.11 s, Shahmukhi text | Omnilingual route, **LOW_RESOURCE_LANGUAGE**, recall 0.361 carried |
| Urdu (FLEURS), 9 s | `ur` | 1, 0.0 to 8.88 s | unchanged: Whisper turbo through the local service, no route fields |

* **Ask engine:** "How many speech segments does ps_pst.wav have?" answered 2 (83 s, the first question of a cold model); "which recordings have speech segments, and in which language?" answered with 3 rows.
* **Not done:** the interface does not yet draw the three labels as badges or play from a segment (the audio viewer of the interface programme); the label is in every segment's data. The worker holds about 1.1 GB with the model loaded lazily; the route stops by itself at its time budget and says so on the segments it did read. Not tested live: a recording longer than the 1500 s budget, and a recording requested without a language that Whisper then hears as Pashto (the re-read rule is covered by a unit test).
* The throwaway collection holds public clips only and can be retired with the other two throwaway collections.


## 9. Automatic language identification for recordings uploaded without a language (2026-11-13)

**Why.** An analyst should not have to know the language of a recording. The live Whisper service does not know Pashto as such: on 2026-11-13 it called the Pashto test clip Persian (`fa`), Punjabi Gujarati (`gu`) and Urdu Hindi (`hi`, which the worker already corrects to Urdu). So an upload with no language was read by Whisper and the Pashto route was never reached.

**What it does (`speech_routes.py`, rule in the `signatures` block of `speech_routes.json`).** When no language was requested, Whisper has read the recording and did not hear English or a routed language, the first 30 seconds are read by the Pashto model and the reading is checked for the letters only Pashto uses (ټ ډ ړ ږ ښ څ ځ ڼ ې ۍ ګ). If they are at least 2% of the reading's Arabic-script letters (and there are at least 20 letters) the whole recording is read by the Pashto route, and every segment carries `first_detected_language` and a `language_resolution` sentence saying what the service heard and why the route was taken. A probe that cannot run never fails the job: the Whisper reading stands. The model is given back after each recording (`FORENSIC_ASR_ROUTE_KEEP_LOADED`, default off), because the Docker VM is small.

**Measured before it was written, on the saved lab readings (no new run):**

| Readings | Called Pashto |
|---|---|
| Pashto, three varieties, development and sealed, 280 clips | 278 (the two misses: one empty reading, one of 6.3 s with 41 letters and no Pashto-only letter) |
| of those with at least 20 letters, 279 clips | 278 |
| Punjabi Shahmukhi, 20 clips | 0 |
| Urdu, 40 clips unconstrained (Devanagari, no Arabic letter) and the same 40 held to the Arabic script | 0 and 0 |

**Checked in the built image (`speech-routes-20261113b`) on the three real clips**, with the speech service replaced by a stub that says what the live service said: the Pashto clip (service heard `fa`) was read by the Pashto route, 5.2% Pashto-only letters, status SEARCH_LEAD, 2 segments, 33 s; the Punjabi clip (service heard `gu`) and the Urdu clip stayed with the Whisper reading, no route fields; the Pashto clip with the service hearing `en` was not probed; the model was released after every recording.

**What this does not cover (said plainly).** (1) Other Arabic-script languages were not measured: Persian, Sindhi, Arabic, Dari. A Persian recording might carry some of these letters in a reading by a Pashto model; the 2% rate was set from Pashto, Punjabi and Urdu only, so a few Persian or Dari recordings could be sent to the Pashto route, in which case the segment says SEARCH_LEAD and carries the reason sentence. (2) Punjabi (Shahmukhi) is not identified automatically: it has no letters of its own that separate it from Urdu, so a Punjabi upload is read by Whisper unless its language is given; its route stays LOW_RESOURCE_LANGUAGE. (3) Short clips: a reading with fewer than 20 Arabic letters, a recording with more than 30 seconds of silence at the start, or heavy noise, may not be identified. (4) Telephone-quality speech was not measured. The next check is a fresh set of clips, including Persian, when the machine has the memory for it.
