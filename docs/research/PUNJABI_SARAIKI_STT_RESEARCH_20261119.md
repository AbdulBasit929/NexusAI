# Speech to text for Pakistani Punjabi (Shahmukhi) and Saraiki: what to build on (research, 2026-11-19)

Second of the media-track research documents (first: `FACE_IMAGE_SEARCH_RESEARCH_20261119.md`; video follows). Owner direction, 2026-11-19: research the best available options online, then implement and measure. Starting point: `docs/data/speech-lab-results-20261112.md`, `docs/data/speech-lab-preregistration-20261112.md` and `ingestion/forensic_records/speech_routes.json`. **Every claim is marked P (read on the primary page or file, today) or S (secondary: search-result summary, to be verified before it is relied on).** Licence statements are what the pages say, not legal advice. Pre-registration for the bake-off: `docs/data/punjabi-saraiki-stt-preregistration-20261119.md`.

## 1. Rules this research works under

* Evidence audio stays on the premises: no hosted speech service (including the commercial ones that advertise Punjabi) sees a case recording.
* Model and weight licences are not a blocker for development (owner, 2026-11-17) but each one is recorded; the training-data provenance of a model is recorded when it is known.
* It must run on the 16 GB CPU laptop beside the stack (real-time factor at most 1.5 with int8 on this machine, the gate of the speech lab), or be a clearly labelled GPU-host option.
* Public data first; the owner's team's recordings (the kit of `docs/qa/punjabi-saraiki-recording-script-20261116.md`) only with their consent. Gated datasets are the owner's click (decision D38).
* A reading of a low-resource language is shown as "a pointer to listen" or "machine reading, a lead" with its measured error, never as a transcript, until its gate is met. Search over readings must tolerate reading errors and say how close each match is (`feedback_search_runtime_llm_exhaustive_20261119`).

## 2. Where the product stands (our own measurements, speech lab 2026-11-13)

| Language | Route today | Measured |
|---|---|---|
| Urdu | Whisper large-v3-turbo (CTranslate2 int8) | 23.4% word error on 40 FLEURS dev clips; 20.9% in October |
| Pashto | Omnilingual CTC 300M (our converted copy) | `SEARCH_LEAD`: sealed, 120 spontaneous clips pooled, word error 30.3%, character error 10.7%, word recall 64.9% |
| Pakistani Punjabi (Shahmukhi) | Omnilingual CTC 300M | `LOW_RESOURCE_LANGUAGE`: 20 development clips, character error 39.8% (gate 20%), word error 67%, word recall 36.1% (gate 50%); Whisper told `pa` writes Gurmukhi (wrong script) |
| Saraiki | none | not supported; no audio on this machine |

All of these are public data (tier T3), read or spontaneous speech from the models' home corpora, **not** telephone or noisy audio. The sealed partition of the Punjabi set (`pnb_Arab`, 161 clips pooled, every clip over 40 s) was **never read** (closed on development evidence, amendment A9 of the speech-lab pre-registration).

## 3. The two languages (facts that decide the design)

* **Pakistani Punjabi** is written in **Shahmukhi** (Perso-Arabic script, language code `pnb`, "Western Punjabi"); Indian Punjabi is written in **Gurmukhi** (`pan_Guru`). Nearly all open Punjabi speech resources are Gurmukhi (Indian). A Gurmukhi model is therefore not a Shahmukhi model: the script differs, the dialect (Majhi/Lahori vs Doabi/Malwai) differs, and the accepted spellings differ. Whisper told `pa` writes Gurmukhi (our run 2026-11-13, P-local).
* **Saraiki** (`skr`, written in an Arabic-script orthography, spoken mainly in southern Punjab and the Sindh border) has very little speech data anywhere.
* Telephone audio is the owner's real condition. The only Pakistani telephone figure found: Whisper small fine-tuned on 15 h of Urdu telephonic speech reaches 62.0% word error (against 22.3% on Urdu broadcast with the same recipe), CHiPSAL 2025 Table 3 (P: paper text read). A model's studio or broadcast figure must not be quoted for telephone audio.

## 4. Candidate models

| Candidate | Licence (as stated) | What the primary page says | Fit and verdict |
|---|---|---|---|
| **Omnilingual ASR** (Meta; CTC and LLM variants, 300M to 7B, "v2" checkpoints) | **Apache-2.0** for code and models (P: README "License" section; hub card `facebook/omniASR-CTC-1B` tag `apache-2.0`) | Lists **`skr_Arab` (Saraiki) and `pnb_Arab` (Western Punjabi)** among 1,682 language ids (P: `lang_ids.py`). Per-language results of the **7B LLM** model (P: `per_language_results_table_7B_llm_asr.csv`): `skr_Arab` **1.5 training hours, character error 17.0%**; `pnb_Arab` 11.9 h, **31.2%**; `urd_Arab` 237.5 h, 3.5%; `pan_Guru` 207.5 h, 4.3%; `snd_Arab` 12.6 h, 6.6%; `hno_Arab` 12.2 h, 17.1%; `pbt_Arab` 5.6 h, 8.8%; `pst_Arab` 5.3 h, 3.5%. CTC models: 300M (1.3 GiB), 1B (3.7 GiB), 3B, 7B; "v2" CTC checkpoints "of improved accuracy" exist (P: README). LLM variants take **optional language conditioning** (P) but need 5 to 17 GiB and run at about real time on a datacentre GPU (P: README table) | **Best candidate family.** The live route is the 300M CTC copy. Open questions, all measurable: is the **v2 300M** or the **1B CTC** (3.7 GiB; fits only with the language model unloaded) clearly better on `pnb_Arab`; how good is any CTC model on Saraiki (the 17.0% is the 7B LLM's number on Meta's own test set, not ours). The 7B and the LLM variants do not fit this laptop; a GPU host or a cloud run for an **accuracy reference only** is the owner's decision |
| **Fine-tuning Omnilingual** (recipes ship in the repository) | Apache-2.0 | "Training Recipes: pre-configured workflows for CTC and LLM model training" (P: README list) | The way to close the Punjabi gap if zero-shot misses: public Shahmukhi (`pnb_Arab` train split, CC BY 4.0) and Saraiki data, speaker-disjoint splits, telephone simulation. Needs a GPU run, approved per run (D36) |
| **w2v-BERT 2.0** (Meta, 600M, self-supervised) | **MIT** (P: hub metadata) | Pre-training languages include `pa`, `ps`, `sd`, `ur` but **not** `skr` (P: hub tags) | Fallback base for a CTC fine-tune if Omnilingual's licence or size becomes a problem. Gurmukhi fine-tunes exist (`kdcyberdude/w2v-bert-punjabi`: Common Voice 10.18%, FLEURS 6.96% word error, S) but they write Gurmukhi |
| **Whisper family** | MIT | Zero-shot on Pakistani Punjabi (Shahmukhi, after Gurmukhi to Shahmukhi conversion of the output): small 86.83%, medium 86.04%, **large 54.73%** word error (P: CHiPSAL 2025, Table 1). Fine-tuned small: 1 h 45.18%, 5 h 41.57%, 10 h 41.80%, **15 h 38.01%** (P: Table 3) | Whisper has no Saraiki code (S). Fine-tuning Whisper to about 38% word error needs 15 h of in-domain Shahmukhi that the authors **did not release** (P: the paper says no public set existed and theirs is in-house, Bulekha TV vlogs) |
| **MMS-1b-all** (Meta) | **CC-BY-NC 4.0** (P: card) | The card's language list shows `pa`; no `skr` (P: grep of the card) | Non-commercial: lab reference at most. Not recommended |
| Gurmukhi models (IndicConformer, IndicWav2Vec, w2v-BERT-Punjabi) plus **Gurmukhi to Shahmukhi transliteration** | various; IndicConformer gated (P, earlier note) | Rule-based transliteration is reported by its authors at over 98.6% word accuracy (Lehal et al., S, own figures); other tools "make spelling mistakes that preclude automating large datasets" (S) | Possible only as a **pseudo-labelling or lexicon bootstrap**, never as the product route: dialect and spelling differ, and errors stack on ASR errors |
| `themohal/saraiki-whisper-medium` | not visible | Reported 19.4% word error, training data "unknown" (S); **the repository returns "Invalid username or password" to anonymous requests (P: tried today)**, so it cannot be checked | Unverifiable. Do not use. Mention to the owner only as a lead to ask its author about |
| Hosted services that advertise Punjabi (cloud speech APIs) | commercial | e.g. a vendor reports 92.9% word error for Whisper large-v3 on FLEURS Punjabi against 11.6% for its own model (S, vendor page) | **Excluded**: evidence leaves the premises |

## 5. Data that exists (and what each is good for)

| Data | Language / script | Licence | Size | Access | Use |
|---|---|---|---|---|---|
| Omnilingual ASR corpus `pnb_Arab` | Western Punjabi, **Shahmukhi**, spontaneous answers to written prompts | CC BY 4.0 (P: corpus card) | on disk: 161 clips (dev + test pooled), 2.7 h, median 60 s | open, already fetched | the only open Shahmukhi test set; **optimistic for Omnilingual models** (trained on its train split); sealed half unread |
| Omnilingual corpus, `train` split of `pnb_Arab` (11.9 h per the results table, P) | same | CC BY 4.0 | | open, not fetched | fine-tuning data (speaker-disjoint from our evaluation clips, to be checked) |
| **Common Voice Saraiki 24.0** | Saraiki | **CC0** | 5,837 clips, 6.75 h recorded, 4.33 h validated, 60 speakers (S: dataset-page summary; the page is script-rendered and was not readable here) | Mozilla Data Collective **account and terms click: only the owner** (D38). The page also says it must not be re-hosted (S) | the only open Saraiki audio we know; read speech |
| **MirasAI "Saraiki 10 Hours TTS Dataset"** | Saraiki | **CC BY-NC-SA 4.0** (S) | about 10 h with transcripts, released 2026-04-01 (S) | same platform and click | studio-style TTS recordings; non-commercial and share-alike: development evaluation only, legal view needed before any training |
| FLEURS | Punjabi `pa_in` is **Gurmukhi**; no Saraiki | CC BY 4.0 | | open | not a Pakistani Punjabi test; no gate uses it |
| CHiPSAL Punjabi (Bulekha TV vlogs) | Pakistani Punjabi, Shahmukhi | not released (P: paper) | 15 h + 4.2 h | none | evidence of what fine-tuning buys, not a resource |
| Owner's team recordings (the kit) | Punjabi, Saraiki, Urdu, mixed | consent needed | as recorded | needs the owner's people | the only way to quote a conversational or telephone-band figure (D26) |

## 6. What the research says to do

1. **Measure before building.** The cheapest high-value step is a pre-registered comparison of the Omnilingual CTC family on Shahmukhi Punjabi: the live 300M copy, the **v2 300M** and the **1B** (same converter route, new hashes), on the unread `pnb_Arab` sealed half, with the speech lab's status tiers (`MACHINE_TRANSCRIPT`, `SEARCH_LEAD`, `LOW_RESOURCE_LANGUAGE`). Whisper `pa` is run once to confirm it writes Gurmukhi (already known) and is not otherwise pursued.
2. **Saraiki cannot be measured until the owner clicks through Common Voice (or the team records it).** The research gives the expectation to test, not a result: Meta reports 17.0% character error for the 7B LLM model with 1.5 training hours; our 300M CTC copy will be worse, and **no number for it is quoted**.
3. **If zero-shot misses, fine-tune, don't switch families.** Fine-tune the Omnilingual CTC 300M (or 1B) on public Shahmukhi and Saraiki data in a cloud GPU run approved by the owner (D36), speaker-disjoint, with a telephone-band simulation, then accept on fresh clips. CHiPSAL's curve (45% to 38% word error from 1 h to 15 h of in-domain data) is the order of magnitude to expect for read broadcast speech, and its telephone Urdu figure (62%) is the warning for the owner's real audio.
4. **Search must be built for reading errors.** With character error of 17% to 40% the useful product is approximate search over the reading (character n-grams, normalised spelling variants, phonetic keys), returning every recording above a stated closeness with the audio position, as the exhaustive-search rule requires.
5. **Interface and Ask** show the status tier and measured error next to every reading and never a plain transcript for these two languages until a gate is met.

## 7. Needs from the owner

* Common Voice account and terms click (or a decision to buy/arrange Saraiki recordings): Saraiki cannot be measured without it.
* The team recordings from the kit (any speakers who consent), including some on a telephone line.
* A GPU run for fine-tuning and for the 7B/LLM accuracy reference, approved per run (D36), and which public sets may be used for training (the NC-SA set needs a legal view).
* Native-speaker reviewers (D10) to check references and any machine-translated or transliterated text.

## Sources

Primary pages read today: [Omnilingual ASR README](https://github.com/facebookresearch/omnilingual-asr), its `lang_ids.py` and `per_language_results_table_7B_llm_asr.csv`; hub cards [facebook/omniASR-CTC-1B](https://huggingface.co/facebook/omniASR-CTC-1B), [facebook/w2v-bert-2.0](https://huggingface.co/facebook/w2v-bert-2.0), [facebook/mms-1b-all](https://huggingface.co/facebook/mms-1b-all); [CHiPSAL 2025 paper, Whisper on Pashto, Punjabi and Urdu](https://aclanthology.org/2025.chipsal-1.20.pdf) (tables 1 to 3 read from the PDF text).
Secondary (to verify): Common Voice Saraiki [24.0](https://datacollective.mozillafoundation.org/datasets/cmj8u3pta00phnxxbsw61jb2r) and [23.0](https://datacollective.mozillafoundation.org/datasets/cmflnuzw7rb0xy8f8yozu9aed) dataset listings; [Saraiki 10 Hours TTS Dataset](https://datacollective.mozillafoundation.org/datasets/cmnggqr8z0082mh07vbsbm6t5); [kdcyberdude/w2v-bert-punjabi](https://huggingface.co/kdcyberdude/w2v-bert-punjabi); [themohal/saraiki-whisper-medium](https://huggingface.co/themohal/saraiki-whisper-medium) (login wall); [Gurmukhi to Shahmukhi transliteration system](https://learnpunjabi.org/pdf/GurmukhiToShahmukhiTransliteration.pdf); [Omnilingual ASR paper](https://arxiv.org/abs/2511.09690).
