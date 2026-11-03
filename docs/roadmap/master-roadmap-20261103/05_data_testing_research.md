# 05. Data, testing and research

**Owner's request, 2026-11-03:** for testing and use, fetch and research the best, most appropriate, realistic and relevant data from online sources until real data is provided; do not get stuck, repeat tasks or run long test cycles.

Marks as in file 01. Everything in the dataset register below was **found today by search and, where marked [V], read on the primary page**; licence statements marked [S] come from a search summary or a mirror and **must be read on the primary page before the download is logged**.

---

## 1. Rules for acquiring and using data

1. **Standing permission, with the ledger.** The owner's message is taken as a standing yes to *search for and fetch public, licence-clean datasets for evaluation and fine-tuning*, subject to the existing process, which is not relaxed: one **licence-ledger row per dataset** (source URL, licence text read, version, size, SHA-256, intended use, partition, who fetched, when); a **written decision rule** before the first measurement; hash manifest and short-read guard (`tools/media_data/` has the fetch/verify kit); the **size stated before any download above 2 GB**. A non-commercial or unclear licence means **evaluation-only** and is recorded as such; unknown means do not download.
2. **Synthetic first, public second, real last.** Tiers: **T0** exact fixtures; **T1** synthetic with planted truth (the case generator, the reference cases); **T2-V** controlled virtual scenes with exact truth; **T2-P** physical captures (owner); **T3** public licensed; **T4** authorised real. Every report states which tier it used.
3. **Real data stays local.** The supplied real files live in the vault (`docs/data/real-data-handling-policy.md`): counts and booleans only in reports; never in the repository, a screenshot, a cloud tool, a prompt or a design tool. The two RAR archives from the team lead are vault samples.
4. **Cloud compute is allowed for public/synthetic data only** (Kaggle/Colab/rented GPU), with the owner's approval per run, never with evidence or labels derived from evidence. Weights and metrics come home with their manifests.
5. **Synthetic identity documents carry a visible watermark** ("SYNTHETIC SPECIMEN") and numbers outside real ranges; no realistic forgeries are produced.
6. **Partitioning:** the last hex digit of `sha256(id)`: 0 to b development, c to f sealed. Sealed sets are never tuned on and are touched once per gate.

## 2. Dataset register (search results of 2026-11-03)

### 2.1 Speech
| Dataset | Content | Licence | Size (as read) | Use | Notes |
|---|---|---|---|---|---|
| Common Voice Scripted Speech 24.0, **Urdu** | read speech | CC0-1.0 [S] | 252,899 clips, 302 h, 81.5 h validated, 498 speakers [S] | train + test | `datacollective.mozillafoundation.org/datasets/cmj8u3pz600t9nxxbz9l2ck2n`; the site moved to `mozilladatacollective.com` and renders its pages with script, so read the datasheet in a browser |
| Common Voice, **Pashto** | read speech | CC0 [S] | 113 h in v24 [V arXiv 2604.06507]; 82 validated h in v23 [S] | train + test | the v25 page's headline figures (5,313 h) look wrong in the summary; verify before use |
| Common Voice 24.0, **Saraiki** | read speech | CC0-1.0 [S] | 5,837 clips, 6.75 h (4.33 validated), 60 speakers [S] | train + test | the only open Saraiki ASR data found; small |
| Saraiki 10-hour dataset (TTS-oriented) | studio speech | CC BY-NC-SA 4.0 [S] | about 10 h | **research only** | not for the product |
| FLEURS (`ur_pk`, `ps_af`, `pa_in`, `sd_in`) | read speech | CC BY 4.0 (ledger DAT-004) [M] | about 12 h per language | test + train | `pa_in` is Gurmukhi (Indian Punjabi), not Shahmukhi |
| Omnilingual ASR corpus | 350 under-served languages | CC BY 4.0 [V README] | large | train + test | check which of `pnb_Arab`, `skr_Arab`, `pus_Arab`, `hno_Arab` it holds |
| Noise and room-response sets for augmentation | noise, RIRs | MUSAN / OpenSLR sets [K] | small | augmentation | read each licence |
| Telephone-band audio | none public for these languages | | | | **simulate**: G.711 mu-law 8 kHz, AMR-NB, Opus low rate, noise, packet loss on clean copies; real calls only from consented captures (D26) |
| TTS-made test audio | | MMS-TTS is non-commercial [K] | | avoid | use public read speech with exact transcripts instead |

### 2.2 Text, translation, OCR and documents
| Dataset | Content | Licence | Use | Notes |
|---|---|---|---|---|
| Saraiki-English parallel corpus (51,447 pairs) and a Saraiki literature corpus (about 1 M tokens) | text | per page [S] | translation and language-model checks | Mozilla Data Collective |
| Urdu Newspaper Benchmark | 829 paragraph images, annotated | read the page [S LREC 2026] | OCR evaluation | confirm licence before use |
| Synthetic Nastaliq/Naskh lines | rendered from OFL fonts (for example Noto Nastaliq Urdu [K]) | ours | OCR train + test | generator in `tools/media_data/` |
| Receipts/forms sets (CORD, SROIE and similar) | scans | vary [K] | layout/OCR evaluation | read each licence |
| PuristanLabs Urdu crops | training crops | | **not valid test data** [M] | |
| Generated documents (wave-1 and Orion) | FIR, witness statements, memos, bank requests, utility bill, chat export | ours | T1 | exact truth |

### 2.3 Faces, plates, images, video
| Dataset | Content | Licence | Use | Notes |
|---|---|---|---|---|
| Open Images (face boxes) | photos | CC BY 2.0 photos [M] | detection evaluation (done) | labels incomplete: false-box rate unmeasurable |
| WIDER FACE, LFW, IJB-C and similar | faces | research terms [K] | baseline evaluation only after reading terms | no training for the product |
| **Consented South Asian face set** | volunteers, CCTV-like degradations | consent form | recognition evaluation, bias analysis | to build (D26, D34) |
| CCPD (Chinese plates) | plates in scenes | MIT [K] | backgrounds, robustness | not Pakistani |
| Roboflow "Pakistani-Number-plates" | 336 images | not read [S] | candidate small set | `universe.roboflow.com/liciece-plate` |
| PLPD (Yousaf et al., 2021) | Pakistani plates | ask the authors [S] | candidate | `ncbi.nlm.nih.gov/pmc/articles/PMC8622297` |
| Roboflow LPR v13 | plates | CC BY 4.0 [S] | detector pretraining | dataset terms apply to fine-tunes |
| **T2-V plate pack (ours)** | 96 virtual scenes, exact truth | ours | plates | [R] |
| **Synthetic Pakistani plates (to build)** | province templates, styles, dirt, blur, perspective | ours | recogniser training | grammar registry feeds the generator |
| MEVA | multi-camera surveillance, vehicles and people | CC BY 4.0 [S] | about 328 h public (516 GB): sample only | tracking and event recall; check the licence text on `mevadata.org` |
| UCF-Crime, UA-DETRAC, AI City | surveillance/traffic | research/NC [K] | evaluation only if terms allow | |
| COCO / Open Images | objects | CC BY mix [K] | detector checks | |

### 2.4 Telecom, financial and network (shape and scale tests; none is Pakistani)
| Dataset | Licence | Use |
|---|---|---|
| Telecom Italia Big Data Challenge (Milan, Trentino) | ODbL [S Nature Sci Data] | volume/shape tests for CDR-like activity |
| MIT Reality Mining (call logs, Bluetooth, towers) | academic [S] | structure checks |
| Orange D4D (Ivory Coast, Senegal) | by application [S] | antenna traffic |
| CRAWDAD traces | per trace [S] | various |
| IBM AMLworld (HI/LI, Small/Medium/Large) | CDLA-Sharing-1.0 for the data via a mirror [S]; code Apache-2.0 | transaction-graph tests with labelled laundering patterns |
| CTU-13, CIC-IDS | research [K] | flow/PCAP tests |
| OpenStreetMap | ODbL [K] | basemap, offline tiles; the owner's offline tile source is unverified (O-8) |

**Why these are not enough and what fills the gap.** No public dataset has Pakistani operator layouts, Pakistani plates at scale, Punjabi/Pashto/Saraiki telephone speech or consented South Asian faces. The case generator (20 cases, planted truth), the reference cases (Orion built; Meridian and Control to build), the T2-V scenes, synthetic plates, codec-simulated telephony and a small consented capture campaign fill the gap; the vault holds the real layouts.

## 3. Synthetic programmes (T1 and T2-V)

| Programme | State | Next |
|---|---|---|
| Case generator (P1 to P8, A1 to A13; planted truth; hash register; 98 tests) | 20 cases; any generator change invalidates the register | add media assets with truth |
| Reference cases (`docs/roadmap/reference-cases-programme-20261028.md`) | **Orion built** (53 files, 484 verified questions); Meridian (Islamabad, overlap) and Control (Karachi, decoys) planned | build with media: images (IDs with watermark, chat screenshots, scene photos), audio (read-speech clips with exact transcripts, in the five languages), video (T2-V scenes with plates and faces from consented volunteers only), cross-case plants and decoys |
| Media synthetic kit (`tools/media_data/`) | OCR/ASR/face/plate eval tools exist | speech gold-set builder with telephone simulation; plate generator; chat-screenshot generator; clock-offset scenarios |

## 4. Testing without repetition: the policy in operation

**Tiers (already binding):** T0 unit and live-database specs at commit; T1 `typing_report.py` (seconds, no model) after any layout/typing change; T2 `live_smoke.py [--quick]` after a deploy (3 to 5 minutes); T3 re-ask only the questions the change can touch; T4 full banks only at milestones, in the background (the verified bank is 484 questions at 40 to 130 s each, about 8 hours on the CPU laptop, about 1 hour with a GPU; read the summary, never wait).

**The change-impact matrix** (so that nothing is re-run needlessly):

| What changed | Run | Do not run |
|---|---|---|
| Engine obligation, validator or prompt | T0 for the module; **re-ask the questions whose results list that obligation/view** (the result files record `views`, `attempts`, `repairs` per question: build the impact index from them once); `live_smoke --quick` | the full bank |
| Wording of sentences | the wording specs; re-judge saved answers without a model (`factory.py reeffective`) | any model run |
| Layout or typing | `typing_report.py COLLECTION` | engine runs |
| Migration | rehearsal on a restored copy (about 9 minutes, needs free memory) + the migration's verify script | |
| Interface | affected vitest files; Playwright slice at 3 widths; look at both themes | the whole suite every time (the whole suite once per stage) |
| Media model or route | one evaluation container on the **development** partition; sealed partition once at the gate | parallel evaluations (the Docker VM is 7.6 GB; 2026-10-21 outage) |
| Fine-tune | development partition each epoch; sealed once | |

**Rules that stop repeated work:** (1) every run writes a result file keyed by *(bank hash, engine image id, switch set, data version)* and a run with an unchanged key is refused unless forced; (2) judgement and sealed cases are **spent** when used for a fix (register kept in the state file); (3) a failing question is re-asked alone with the full log kept; (4) a milestone run happens once per milestone, in the background, with a notification when done.

## 5. The banks and the judge

| Bank | Size | Truth | Gate |
|---|---|---|---|
| Verified bank (Orion) | 484 | independent SQL oracle (`build_verified_bank.py`) | 473/484 today; target 100% with honest abstentions counted apart |
| Fresh-case engine tests (G4) | 15 to 20 per family per fresh case | generator oracle | 0 wrong, per family |
| Media bank (new) | at least 20 per family (audio, image, OCR, plate, video, face) | exact truth of synthetic assets | 0 confident-wrong; every answer labelled "model observation" |
| Cross-case bank (new) | 30 | Meridian/Control oracle | zero false merges, planted overlaps found |
| Language bank (new) | Urdu, Roman Urdu, Punjabi, Pashto, Saraiki questions | native-reviewed (D10) | measured per language, reported apart |
| Conversation bank (new) | greetings, help, clarification, follow-ups, refusals | scripted expectations | no invented capability, no invented figure |
| Red team | 100 | scripted | instructions in OCR/transcripts, SQL escape, PII extraction: 0 successes |

**Reporting standard for every run:** n, correct / wrong / abstained / not run, Wilson 95% interval, latency p50 and p90, **cause of every miss** (engine fault, wording, data, infrastructure), the engine image and switch set, and the exact command to repeat it.

## 6. Research log

### 6.1 Established today
| Finding | Source | Mark |
|---|---|---|
| Omnilingual ASR is Apache-2.0 (code and models), CC BY 4.0 (dataset); models CTC 300M to 7B and LLM 300M to 7B plus "Unlimited" variants; inference limit 40 s per clip except Unlimited; CTC 300M about 2 GiB and 1B about 3 GiB VRAM, 3B about 8 GiB, LLM 1B about 6 GiB, 7B about 17 GiB (A100 table) | github.com/facebookresearch/omnilingual-asr | V |
| Its language list contains `urd_Arab`, `pus_Arab`, `pbt_Arab`, `pnb_Arab`, `skr_Arab`, `pan_Guru`, `snd_Arab`, `hno_Arab`, `brh_Arab`; per-language 7B-LLM CER: Urdu 3.5 (237.5 h), Pashto 12.6 (28.7 h), Southern Pashto 8.8 (5.6 h), **Western Punjabi 31.2 (11.9 h)**, **Saraiki 17.0 (1.5 h)**, Punjabi Gurmukhi 4.3, Sindhi 6.6, Hindko 17.1, Brahui 8.2, English 3.0 | `per_language_results_table_7B_llm_asr.csv` in the repo | V (read through a fetch summariser: re-read the file once before quoting) |
| Whisper is unusable zero-shot for Pashto (WER over 100%, script error); full fine-tuning on Common Voice Pashto v24 (113 h) reaches 23.4% (turbo) and 24.9% (small) WER; a base model on v20 reaches 21.2% | arXiv 2604.06507 (+ 2604.04598: at most 0.8% true Pashto script) | V / S |
| SeamlessM4T is best on Urdu (16.3% WER) and Punjabi (22.1%) in a 2026 benchmark; cross-language script contamination affects up to 35% of output characters in lower-resource languages; named entities lag by 5 to 20 points | aghaaliraza.com Interspeech 2026 paper | S |
| Common Voice has Urdu (302 h), Pashto (113 h in v24) and **Saraiki (6.75 h)** under CC0 | Mozilla Data Collective | S |
| AuraFace-v1: Apache-2.0 on its card, ResNet-100 ArcFace-style, 512-D, LFW 99.65% (self-reported), trained on "a commercial dataset ... from various sources"; bundled InsightFace detectors are non-commercial | huggingface.co/fal/AuraFace-v1 | V |
| NIST FRTE reports false-match differentials by region of birth including South Asia; thresholds are algorithm- and version-dependent | pages.nist.gov/frvt | S |
| PaddleOCR-VL (0.9B) lists 109 languages with Urdu, Pashto, Sindhi and Balochi in its Arabic-script group (vendor claim); no independent Urdu figure found | paddleocr.ai | S |
| IndicConformer 600M (22 Indian languages incl. Urdu, Punjabi, Sindhi), MIT | huggingface.co/ai4bharat/indic-conformer-600m-multilingual | S |
| Qwen3-ASR (Apache-2.0, 52 languages): Urdu/Punjabi/Pashto not confirmed in the list | github.com/QwenLM/Qwen3-ASR | S |
| fast-plate-ocr is MIT-licensed; its pretrained weights' terms are not stated | github.com/ankandrew/fast-plate-ocr | V |
| Pakistani plates are changing: Islamabad owner-based plates; Punjab linking registration to the owner's national ID number and proposing new designs; Sindh Ajrak-themed plates with deadlines extended into late 2025 | PakWheels, ProPakistani, Wikipedia | S |
| Pakistan's personal data protection law is still a draft with broad state exemptions; the Interior Ministry directed a unified facial-recognition system in 2025 | GenderIT, Privacy International, ProPakistani | S |
| MEVA is CC BY 4.0, about 328 h public | registry.opendata.aws/mevadata | S |
| IBM AMLworld data under CDLA-Sharing-1.0 (via a mirror of the README) | arXiv 2306.16424 | S |

### 6.2 Open research questions (read these before the dependent work package starts)
1. Omnilingual fine-tuning recipe, CPU latency of the 300M/1B CTC models, quality on 8 kHz audio (WP-M1).
2. Which languages the released Omnilingual **corpus** contains (WP-M1).
3. The Interspeech 2026 paper's per-model tables (is its Punjabi test set Shahmukhi?) (WP-M1).
4. Common Voice: is there a Western Punjabi (Shahmukhi) release? Pashto v25 true size (WP-M1).
5. AuraFace licence file and training-data statement; cost and terms of an InsightFace commercial licence; counsel on Pakistani law (WP-M7, D34).
6. RF-DETR checkpoint licences per size; PDQ licence file; pyannote community-1 terms; Roboflow Trackers licence (WP-M5, M6).
7. NIST FRTE values for the shortlisted algorithm families for the South Asia group (WP-M7).
8. Independent Urdu OCR results for PaddleOCR-VL and Qwen3-VL (WP-M4).
9. DVR container formats common in Pakistan and their ffmpeg support (WP-M6).
10. Official plate formats and dates per province, from the excise departments (WP-M5).
11. Licence of IBM AMLworld on the Kaggle page; Telecom Italia terms on its challenge page.
12. Sources for Urdu/Punjabi/Pashto/Saraiki question sets and native reviewers (D10).
