# Punjabi (Shahmukhi) and Saraiki speech to text: pre-registration (2026-11-19)

**Written before any measurement of the models and data named here.** It extends the speech-lab pre-registration (`speech-lab-preregistration-20261112.md`, amendments A1 to A9) and keeps its partition rule, normalisations (`N1`, `N2`), metrics, status tiers (`MACHINE_TRANSCRIPT` above `SEARCH_LEAD` above `LOW_RESOURCE_LANGUAGE`) and the rule "only sealed decides, read once". Changes are dated amendments at the end. Results: `punjabi-saraiki-stt-results-<date>.md`. Research behind it: `docs/research/PUNJABI_SARAIKI_STT_RESEARCH_20261119.md`. **No measurement of this registration has been run yet** (this session produced the research and the registration only; the face bake-off runs first).

## 1. Questions

* **Q-P (Pakistani Punjabi, Shahmukhi).** Is there a model that runs on this CPU laptop and is clearly better than the live Omnilingual CTC 300M copy (character error 39.8% on 20 development clips, status `LOW_RESOURCE_LANGUAGE`)? Candidates: the **v2 300M** and the **1B CTC** checkpoints of the same family.
* **Q-S (Saraiki).** Does any route reach `SEARCH_LEAD` or better on Saraiki speech? Cannot start until audio exists (section 3).
* **Q-T (telephone).** What happens to every promoted route on telephone-band audio? (Never inferred from a clean-speech figure.)
* **Q-F (search).** Can approximate search over the readings find the recording when a distinctive word is spoken?

## 2. Models (each with its licence and a hash recorded at fetch)

| Id | Model | Licence | Status of the file |
|---|---|---|---|
| `omni300-v1` | `omniASR_CTC_300M`, our converted copy (live route) | Apache-2.0 (hub card) | on disk |
| `omni300-v2` | `omniASR_CTC_300M_v2` (Meta download URL in the README) | Apache-2.0 (README) | to fetch, convert with `tools/media_data/omni_ctc.py`, strict tensor mapping must place every tensor |
| `omni1b` | `omniASR_CTC_1B` (hub `facebook/omniASR-CTC-1B`, 3.7 GiB) | Apache-2.0 (hub tag) | to fetch and convert; runs only with the language model unloaded, one container, memory-capped |
| `turbo` | faster-whisper large-v3-turbo (incumbent for Urdu; told `pa` it writes Gurmukhi) | MIT | on disk; run only to confirm the script and as the Urdu-told baseline |

Not run in this registration, with the reason: Omnilingual LLM variants and 3B/7B (do not fit the CPU gate; a GPU reference run is the owner's decision), MMS (CC-BY-NC, no Saraiki), any model whose weights cannot be inspected (`themohal/saraiki-whisper-medium` returns a login error), hosted services (evidence stays on premises).

## 3. Data and partitions

| Set | Source | Licence | State |
|---|---|---|---|
| `pnb_Arab` | Omnilingual corpus, 161 clips, dev + test pooled, spontaneous Shahmukhi, median 60 s | CC BY 4.0 | on disk; partition by the speech-lab rule (last hex digit of SHA-256 of `<set>-<speaker_id>-<prompt_id>-<segment_id>`: `0` to `b` dev, `c` to `f` sealed); **the sealed half has never been read** |
| `skr-cv` | Common Voice Saraiki 24.0 (CC0), its own `test` split for sealed, a hash-chosen part of `dev`/`validated` for development | CC0 | **not on disk**: needs the owner's Mozilla Data Collective account and terms click (D38). Until then Q-S is "not measurable" |
| `skr-team` / `pnb-team` | recordings by the owner's team from the kit | consent | none yet |

Clips over 40 s are read in pieces cut at the quietest point (speech-lab rule). Speakers may fall in dev and sealed for ranking models; any training uses speaker-disjoint splits. All of this is tier T3 (public data) or T4 (consented); no figure is a statement about the owner's real recordings unless it is on T4.

## 4. Protocol

Q-P: each candidate reads `pnb_Arab` **dev** (20 clips as before, then the whole dev part if time allows, nothing else running, one container, memory-capped, no network); decision on dev: lowest character error (`N2`) among models with a real-time factor at most 1.5, ties to the smaller model. The chosen model and `omni300-v1` read **sealed once**. Metrics as in the speech lab: corpus word and character error with 95% bootstrap intervals over clips, word recall (distinct reference words of three letters or more found in the reading), Arabic-script share, loops, empty outputs, real-time factor, peak memory. `turbo` told `pa` on 5 clips only to report the script share.
Q-S: when audio exists, the same protocol on `skr-cv` with `omni300-v1` and the Q-P winner; Whisper `turbo` told `ur` is the "reads it as Urdu" baseline.
Q-T: for every route that reaches `SEARCH_LEAD` or better, the sealed clips are degraded to telephone band (resample to 8 kHz, G.711 mu-law encode and decode with ffmpeg, additive noise at 20, 10 and 5 dB signal-to-noise from the point-source noise folder already on disk) and re-read **once**; the status is recomputed on the degraded clips and reported beside the clean one.
Q-F: for each sealed clip, up to three distinctive reference words (three letters or more, after `N2`) are searched over the readings of **all** sealed clips with an approximate word match (character 3-gram Dice similarity at least 0.6 on the normalised words); recording-level recall and precision are reported with the matching rule printed. Exact match is reported alongside.

## 5. Gates (fixed now; the speech lab's tiers apply on the sealed partition)

* **G-P1 (switch the Punjabi route).** A challenger replaces `omni300-v1` only if its dev character error (`N2`) is at least **5 points (absolute) lower** than `omni300-v1`'s on the same clips **and** it is not higher on sealed; real-time factor at most 1.5; peak memory at most 2.5 GB beside the stack (or the language model is shown to stay loaded).
* **G-P2 (status of the route that stays or wins).** On sealed: `MACHINE_TRANSCRIPT` if character error (`N2`) at most 20%, script share at least 99%, loops at most 1%, real-time factor at most 1.5; else `SEARCH_LEAD` if pooled word recall at least 50% with the same script, loop and speed limits; else `LOW_RESOURCE_LANGUAGE`.
* **G-S1 (Saraiki).** Same tiers on `skr-cv` sealed. Without audio the status is "not supported, not measured"; no number is quoted.
* **G-T1 (telephone).** A route keeps its status for telephone use only if the degraded sealed result meets the same tier at 10 dB; otherwise the interface wording for telephone recordings is the next tier down, whatever the clean result.
* **G-F1 (search).** Approximate search finds the right recording for at least 70% of the searched words at precision at least 80% on sealed readings of a route at `SEARCH_LEAD` or better; if not, the search feature for that language says "unreliable".
* **Decision rule.** Nothing is deployed or enabled by this work. A passed gate is a recommendation to the owner; the speech route table (`speech_routes.json`) changes only on the owner's word and a new sealed measurement, as its own note says.

## 6. Order, budget, stopping rule

1. Fetch and hash `omni300-v2` and `omni1b`; convert; fidelity check (character error on the 5 `pst_Arab` clips in the range of the model family, as in A7 of the speech lab).
2. Q-P development runs, one model per container, nothing else running (language model container unloaded or the host memory shown to be sufficient; the host's lowest available memory is written into every result).
3. Sealed read once (chosen model + `omni300-v1`), then Q-T once, then Q-F.
4. **At most two rounds** per question (a baseline and one correction round), then the language is closed with its numbers stated. No round is repeated to reach a gate.
5. If Q-P ends below `SEARCH_LEAD`, or Q-S (once audio exists) does, the **fine-tuning round** is proposed to the owner: Omnilingual CTC 300M (or 1B) fine-tuned on public Shahmukhi and Saraiki data in a cloud GPU run, speaker-disjoint, with the telephone simulation, accepted on fresh clips. That run is not part of this registration and needs the owner's approval per run (D36) and a legal view on the share-alike dataset.

## 7. What this cannot show (stated in advance)

* `pnb_Arab` is Omnilingual's home corpus: an Omnilingual model's figure is optimistic. A Whisper-family model has not seen it.
* Every clip is longer than 40 s and is read in pieces; a cut through a word costs about one word per piece.
* Read or spontaneous studio-grade speech, not telephone or noisy audio (Q-T tests a simulation of the telephone band, not real lines).
* Saraiki is unmeasurable until audio exists; the 17.0% character error Meta reports for its 7B LLM model on its own test set is not our model and not our data.
* Shahmukhi spelling is not standardised; `N2` removes known letter-form differences but a native speaker's review (D10) is still needed to call a reference right.

## 8. Needs from the owner

Common Voice account and terms click (Saraiki); team recordings (Punjabi, Saraiki, including some on a telephone line); approval of a GPU run only if section 6 step 5 is reached; names of native-speaker reviewers.

## 9. Amendments

(none yet)
