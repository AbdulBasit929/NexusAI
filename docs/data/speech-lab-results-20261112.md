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
