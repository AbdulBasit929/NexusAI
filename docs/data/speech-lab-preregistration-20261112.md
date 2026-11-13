# Speech lab (WP-M1), pre-registration, 2026-11-12

Owner direction, 2026-11-12: start the speech lab "in the best, accurate, correct, up-to-date way", following the rules given earlier. This file fixes the data, the models, the measurements and the gates **before the first measurement** and is not edited afterwards; changes are made as dated amendments at the end. Results go in `speech-lab-results-20261112.md`. It applies sections 4 and 11 of `docs/roadmap/master-roadmap-20261103/02_media_programme.md`, decisions D35 (voice: no identification), D36 (hardware: no cloud for evidence; public-data training only with the owner's approval per run), D38 (data: licence-clean, one ledger row each, gated sets are the owner's click) and D39 (languages: Urdu and English, Pashto, Saraiki, Pakistani Punjabi), and the pre-registration of 2026-10-21 (`media-activation-programme-20261021.md`, track A) whose rules still hold.

## 1. What is known going in (measured by us on 2026-10-21/22, FLEURS read speech, live configuration)

| Language | Word error | Status in the product |
|---|---|---|
| English | 4.7% | machine transcript |
| Urdu | 20.9% (small model 33.8%) | machine transcript, review required |
| Pakistani Punjabi (Shahmukhi) | not measured (FLEURS `pa_in` is Gurmukhi, 91% word error, wrong script) | `LOW_RESOURCE_LANGUAGE`: a pointer to listen, not a transcript |
| Pashto | 96.3% (Whisper writes the wrong script) | `LOW_RESOURCE_LANGUAGE` |
| Saraiki | no model, **no audio on this machine** | not supported |

## 2. Evidence read on 2026-11-12 (what the lab stands on)

* The public Omnilingual ASR corpus (`facebook/omnilingual-asr-corpus`, CC BY 4.0, 348 languages) holds, for our list: `pbt_Arab`, `pbu_Arab`, `pst_Arab` (Southern, Northern and Central Pashto), `pnb_Arab` (Western Punjabi in Shahmukhi), plus `hno_Arab` (Hindko) and `brh_Arab` (Brahui). It holds **no Saraiki (`skr_Arab`), no Urdu and no Sindhi** (directory listing of the dataset, read today). Its clips are **spontaneous spoken answers to written prompts**, not read sentences: this is the closest public thing to conversational speech we have, and the first open Shahmukhi test set. Slices already on this machine since 2026-11-03 (hashes in `manifest.jsonl`; clip lengths read from the files on 2026-11-12): Pashto 300 development and 300 test clips per variety, median length 24 to 28 seconds (range 4 to 84 s, 3% to 9% over 40 s), 4.3 to 5.0 hours per variety; Punjabi 77 and 84 clips, 2.7 hours, **median 60 seconds (47 to 90 s): every Punjabi clip is longer than the 40-second limit of the Omnilingual models**.
* The Omnilingual models were trained on the train split of this same corpus; the corpus's own development split may have been used to choose their checkpoints. A measurement of an Omnilingual model on these clips is therefore **optimistic** (it is at home), and the report says so. A Whisper-family model has not seen this corpus.
* Meta's own inference route needs `fairseq2` (package `omnilingual-asr`; CTC and LLM models accept audio shorter than 40 seconds). There is no official `transformers` integration; a community conversion of the 300M and 1B CTC checkpoints to `Wav2Vec2ForCTC` exists and is unverified. Either route is a **new dependency** and waits for the owner's word (section 8).
* Whisper zero-shot on Pashto fails on script, not only on words: at most 0.8% of outputs are in Pashto script for any Whisper size, and SeamlessM4T (non-commercial licence, evaluation only) reaches 39.7% word error on Common Voice 24 [arXiv 2604.04598]. Fine-tuning fixes it: the Pashto Common Voice paper reports a Whisper base fine-tuned on Common Voice 20 at 13.4% word error on that corpus's own test split, 147 hours, 1,483 speakers [arXiv 2603.27021]; the model on this machine (`exp_009_small_cv24_vanilla`, Whisper small, Apache-2.0 on its card, 24.89% word error on the Common Voice 24 test split by its card) is such a fine-tune. These are **read-speech** numbers on the corpus the model was trained on.
* Shahmukhi Punjabi: no public set was known before the Omnilingual slice; a 2025 CHIPSAL study (broadcast audio, 15 hours) fine-tuned Whisper small to about 38% word error, large zero-shot about 55%.
* Saraiki: no ASR model, dataset or paper found in today's search. The only known audio is Common Voice Saraiki (6.75 hours, CC0) behind an account and terms click that only the owner can make (D38).
* Urdu: no new open model that beats the Whisper family on FLEURS with a confirmed permissive licence was found; the 2026 fine-tunes found report relative gains without absolute figures.

## 3. Data and partitions (fixed)

Evaluation sets (all outside the repository in `C:/tmp/nexusai-media-data/`, hashes in `manifest.jsonl`, a hash mismatch blocks use):

| Set | Source | Kind | Clips on disk |
|---|---|---|---|
| `ur_pk` | FLEURS test | read speech, Urdu (median 9 s) | 299 |
| `en_us` | FLEURS test | read speech, English (median 9.5 s) | 647 |
| `ps_af` | FLEURS test | read speech, Pashto (median 11.8 s) | 512 |
| `pst_Arab`, `pbt_Arab`, `pbu_Arab` | Omnilingual corpus, dev + test splits pooled | spontaneous, Pashto (Central, Southern, Northern) | 600 each |
| `pnb_Arab` | Omnilingual corpus, dev + test splits pooled | spontaneous, Western Punjabi, Shahmukhi (median 60 s) | 161 |

**FLEURS `pa_in` (Gurmukhi) is not a test of Pakistani Punjabi and is not used for any gate.**

* **Partition.** A clip belongs to `dev` or `sealed` by the last hex digit of the SHA-256 of its key: `0` to `b` is dev, `c` to `f` is sealed. Key: `<set>-<id>` for FLEURS, `<set>-<speaker_id>-<prompt_id>-<segment_id>` for the Omnilingual corpus. Decisions are made on dev. **Sealed is read once per language, at the end**, for the final report, and never used to choose or tune.
* **Sample.** Within a partition the clips are ordered by the SHA-256 of their key and the first N are taken (deterministic; the list of keys and its SHA-256 are written into every result file). N, dev: `ur_pk` 60, `en_us` 30, `ps_af` 40, each Omnilingual Pashto variety 40, `pnb_Arab` 40. N, sealed: the same, and all of `pnb_Arab` sealed if fewer than 40. Forty Pashto clips are about 17 minutes of audio and 40 Punjabi clips about 40 minutes.
* **Speakers.** Clips of one speaker may fall in dev and in sealed. That is acceptable for ranking existing models; it is **not** acceptable for training, and the fine-tuning round (section 9) will split by speaker.

## 4. Models and routes under test

| Id | Model | Licence | Role |
|---|---|---|---|
| `turbo` | faster-whisper large-v3-turbo CT2 (MOD-043), the live model | MIT | incumbent for every language |
| `small-ur` | faster-whisper small (live fallback for recordings over 600 s) | MIT | reference only |
| `ps-small` | `ihanif/exp_009_small_cv24_vanilla` (Whisper small fine-tuned on Common Voice Pashto 24), converted here to CTranslate2 int8 with `ct2-transformers-converter` (no new dependency: the tool ships with `ctranslate2` in the evaluation image); the converted folder gets its own manifest row | Apache-2.0 on its card (base model MIT; training data CC0); **to verify** against the repository's licence file before any product use | Pashto candidate |
| `omni-ctc-300m` | Omnilingual `omniASR_CTC_300M` (on disk, 1.3 GB) | Apache-2.0 on its card, code and weights to be verified separately | second opinion and candidate for Pashto, Punjabi (Shahmukhi), Urdu |

Not under test now, with the reason: Omnilingual LLM variants (6 to 8.5 GB, real-time factor about 1 on an A100: not usable on this CPU), SeamlessM4T and MMS (non-commercial: evaluation only, and not worth a download for a number that cannot ship), Whisper large-v3 (not on disk; turbo is its distilled form), any model whose weights licence cannot be established (decision D31 applies to speech as to plates).

## 5. Measurement (fixed)

* **Configuration.** The live one: int8, CPU, 8 threads, beam 1, `condition_on_previous_text=False`, the language **told** (`ur`, `en`, `ps`). Clips longer than 30 seconds are read by the library's own sliding window. **Models with a 40-second input limit (Omnilingual)** read a longer clip in pieces cut at the quietest point (20 ms frames of RMS energy, smoothed over 200 ms, the minimum between 20 s and 36 s of the remaining audio, repeated), and the piece texts are joined with a space; the same cutting is applied to every such model, and a cut through a word costs that model about one word per piece, which the report states. On `pnb_Arab`, `turbo` is run twice, told `pa` and told `ur` (Shahmukhi is written in the Urdu script; `pa` makes Whisper write Gurmukhi); both are reported, the better one is the incumbent figure. Models are run one at a time, in one container (`nexusai-media-eval-asr`, `--network none`, `--cpus 8`, `-m 5g`, data read-only), and **nothing else runs while a model is timed** (the Ask engine's load, a bank run, a build are all "something else").
* **Normalisation, two versions, both reported for every run.** `N1` is the written normalisation of 2026-10-21 (NFKC, lower case, punctuation and symbols removed, Arabic-Indic and Extended Arabic-Indic digits mapped to ASCII, whitespace collapsed). `N2` is `N1` plus: removal of Arabic diacritics (U+064B to U+065F, U+0670), tatweel (U+0640), zero-width joiners and non-joiners (U+200C, U+200D) and the Arabic comma and question mark; mapping of Arabic yeh forms U+064A and U+0649 and Farsi yeh U+06CC to U+06CC, Arabic kaf U+0643 to Keheh U+06A9, and Arabic heh U+0647 and heh goal U+06C1 to U+06C1. It exists because Pashto, Punjabi and Urdu spell the same word with letters that differ only in their Arabic or Persian form, and a reference written in one convention scored against a hypothesis in the other would be counted as a word error. **`N2` is the primary figure for `ps` and `pnb`; `N1` is the primary figure for `ur` and `en`** (comparable with October). The two mappings are in one function with a test, and are not changed after the first run.
* **Metrics.** Corpus word error (sum of word edits over sum of reference words) and corpus character error (spaces ignored), each with a 95% bootstrap interval over clips (2,000 resamples, seed 1); per-clip median word error; share of letters in the expected script (Arabic block U+0600 to U+06FF and supplements for `ur`, `ps`, `pnb`; Latin for `en`); repetition loops (a 3-word sequence five times or more, or an output more than three times the reference length plus ten words); empty outputs; real-time factor (compute seconds per audio second); the language the model detected when it was not told (only on a separate pass, never mixed into the gate figures).
* **Reading of the results.** A difference smaller than the half-width of the interval is **not** a difference. A language is never called "good" or "bad" from one set.

## 6. Gates (fixed now; a route is promoted from `LOW_RESOURCE_LANGUAGE` to "machine transcript, review required" only if all of its line hold on the **sealed** partition)

| Language | Gate |
|---|---|
| Pashto (route `ps-small` or another) | FLEURS `ps_af`: word error (N2) at most **30%**; the three Omnilingual Pashto sets pooled (spontaneous): word error (N2) at most **45%** and character error (N2) at most **20%**; script share at least 99%; loops at most 1%; real-time factor at most 1.5 |
| Pakistani Punjabi, Shahmukhi (route `omni-ctc-300m` or another) | `pnb_Arab` spontaneous: character error (N2) at most **20%**; script share at least 99%; loops at most 1%; real-time factor at most 1.5. Word error is reported, not gated |
| Urdu | the incumbent stays unless a challenger is at least **15% relatively lower** in word error (N1) on dev **and** not higher on sealed; the roadmap target of at most 15% absolute is reported as open, not as a condition on the live model |
| English | no worse than the incumbent (4.7% on the October set) |
| Saraiki | **not measurable now** (no audio). It stays unsupported; no number is quoted |

A language that misses its gate keeps `LOW_RESOURCE_LANGUAGE` and the interface wording "a pointer to listen, not a transcript"; the result file says by how much it missed. A passed gate on read or "at home" speech is **not** a statement about telephone, noisy or conversational audio of the owner's kind; that needs a consented sample (D26, names still owed) and the telephone simulation of the fine-tuning round.

## 7. Order, budget and stopping rule

1. Verify the hashes of every file used (`verify_manifest.py`). Convert `ps-small` to CTranslate2 int8 and record its hash.
2. Baselines on **dev**, one model per container run, nothing else running: `turbo` on every set (it is the incumbent everywhere); `ps-small` on `ps_af` and the three Pashto sets. Wall time, by the October real-time factor of 1.45 for `turbo` and about 0.55 for a small model: dev audio is about 52 minutes on the three Pashto sets, 40 on `pnb_Arab` (twice for `turbo`), 8 on `ps_af`, 9 on `ur_pk` and 5 on `en_us`, so about **3.5 hours** of machine time for the `turbo` runs and 1 hour for `ps-small`. The estimate is a range; the video work ran longer than predicted.
3. `omni-ctc-300m` when the owner approves the dependency (section 8), on the same dev sets.
4. Decide per language from dev by the rule of 2026-10-21 (lowest word error among models with a real-time factor at most 1.5; ties to the smaller model and the simpler licence). Read **sealed once**.
5. **At most two rounds per language** (a baseline and one correction round), then the language is closed with its numbers stated, whatever they are. No round is repeated to reach a gate.

## 8. What needs the owner

* **A new dependency** for `omni-ctc-300m`: either the `omnilingual-asr` package with `fairseq2` (pip, in a separate throwaway evaluation image built from the evaluation image; it is never run with a network and never deployed), or the community `Wav2Vec2ForCTC` conversion (its code read first). One word from the owner picks the route or declines it. Until then the lab runs everything else.
* **Saraiki audio:** Common Voice Saraiki (and Urdu, Pashto at their full size) is behind an account and terms click that only the owner can make. Nothing is created or accepted on the owner's behalf.
* **Names** for native-speaker reviewers (D10) and the audio capture (D26): the only route to a conversational and telephone-band figure we may quote.

## 9. Not in this round (stated so that nothing is implied)

Fine-tuning (it needs a cloud or GPU run on public data, approved per run, split by speaker, with the telephone simulation of section 4.3 of the media programme); diarisation, spoken-number conversion, translation and the transcript viewer (WP-M2); speaker clustering (D35, not identification); Hindko and Brahui (on disk in the corpus, not on the owner's list). The results are **T3** (public data), not a measure of the owner's real audio.

## 10. Amendments

**A1, 2026-11-12, before the first measurement.** "Nothing else runs while a model is timed" means that no work is running: the Ask engine's language-model container, the worker, the database and the queue stay up and idle, as they did on 2026-10-21/22, so that the real-time factors compare with October's. With them resident the Docker VM (7.6 GB) has about 1.4 GB free, so the clip loader reads the audio in small batches instead of loading a whole 400 MB parquet file. The VM's lowest free memory is written into every result file; a run during which it falls under 300 MiB is repeated and the first one is discarded and said so.
