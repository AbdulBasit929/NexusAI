# Face and image search: what to build on (research, 2026-11-19)

Owner direction, 2026-11-19: image search, face search, face detection and matching are to be researched from the best online sources and then implemented and measured. This is the first of the media-track research documents (the audio, video and image-text ones follow). Method: primary pages read in full where it matters (model cards, licence files, READMEs), search-result summaries for the rest; **every claim is marked P (read on the primary page) or S (secondary source, to be verified before it is relied on)**. Licence statements are what the pages say, not legal advice.

## 0. What the bake-off found (added 2026-11-20; details and gates in `docs/data/face-image-search-results-20261119.md`)

Measured on public data, CPU laptop. **Detection:** YuNet finds 82.5% of clear faces (40 px or wider) at 0.11 false boxes per image but only 15 to 66% of small, blurred, turned or occluded ones; BlazeFace finds 16% (its card puts such scenes out of scope) and is not adopted. **Recognition:** SFace (the current model) is equal or better than AuraFace on four of five public verification sets and 13 times faster, so AuraFace is not adopted; neither model's accuracy on real South Asian people is measured (no consented set), only the false-match differential, where Indian faces are not the hardest group. The product's default threshold (cosine 0.363) is too permissive (about 1 false match per 62 pairs of Indian faces). **Quality:** OFIQ matches its reference exactly and its score tracks matching reliability. **Index:** per-case partitions give exact answers; global HNSW with filters tops out at recall 0.86. **Image retrieval:** SigLIP 2 beats SigLIP v1 in English (+3.6 points), neither handles Urdu (15% and 4% in the top 10), and DINOv2 base is the best near-duplicate model (100% recall and precision on the sealed set). The tables below are the pre-measurement research; where they say "candidate" the results note holds the verdict.

## 1. Rules this research works under

* Evidence stays on the premises: no hosted face or image service sees a case file.
* A model is usable in the product only if its **licence and the provenance of its training data** allow it. Face models are the hard case: the code can be MIT while the weights are non-commercial and the training set was retracted.
* It must run on the 16 GB CPU laptop beside the stack (a model of more than about 2 GB is not safe beside the language model); a GPU host is a later option.
* Face matching is a **lead, never an identity**: every result carries a score, a quality figure and a "needs analyst review" flag; thresholds come from a measured false-match rate on our own labelled data; nothing is shown as a name.
* Real faces and voices are used only with the owner's consent; synthetic and public data come first. National identity numbers stay counted, not shown (existing rule).
* A search returns **all** matches above a calibrated bar, not the best one, with a stated completeness (see `feedback_search_runtime_llm_exhaustive_20261119`).

## 2. What the product already has

Read from the worker's configuration and the project notes: face detection and embedding by **YuNet and SFace** (`FORENSIC_FACE_MODEL=face-detect-yunet-sface`, the governed "L1" level); image embeddings by **SigLIP base** (`google/siglip-base-patch16-224`, `FORENSIC_IMAGE_EMBEDDING_MODEL`); **pgvector** is in the database image; perceptual hashes of images; AuraFace downloaded but never measured. So the question is not "what exists" but "is there a better, licence-clean choice, and is what we have good enough on South Asian faces".

## 3. Face detection

| Candidate | Licence | Evidence | Verdict |
|---|---|---|---|
| **YuNet** (OpenCV Zoo) | **MIT (P: LICENSE file read)** | WIDER Face validation AP 0.8844 / 0.8656 / 0.7503 (easy / medium / hard; P, README table; the README's opening line gives lower figures and does not say why); about 230 KB ONNX and 1.45 ms on a CPU (S); new default model is dynamic-shape (P); training data not named in the README, training code in `ShiqiYu/libfacedetection.train` (P) | **keep as the baseline**; measure on small, blurred and angled faces |
| SCRFD (InsightFace) | code MIT, **pretrained weights non-commercial research only (S, consistent with InsightFace's own pages)** | strong accuracy/speed | **blocked** unless a commercial licence is bought |
| RetinaFace | code MIT in several ports (S); original weights may carry the InsightFace terms (S) | | check the exact weight source before use; low priority |
| BlazeFace (MediaPipe) | Apache-2.0 on a mirror (S, not confirmed on Google's repository) | short-range, small faces weaker | candidate for video frames; verify |
| WIDER Face dataset | non-commercial research (S) | | models trained on it may inherit the restriction: ask counsel |

## 4. Face recognition (matching)

| Candidate | Licence | Evidence | Verdict |
|---|---|---|---|
| **SFace** (OpenCV Zoo, MobileFaceNet with SFace loss) | **Apache-2.0 (P: LICENSE read)** | accuracy 0.9940 in the zoo's table without naming the dataset (P); **training data not named in the README** (P); small, CPU-friendly (8.65 ms on a CPU, S); needs 5-landmark alignment (P) | **baseline**; find the training data in the SFace paper (arXiv 2205.12010) before relying on it |
| **AuraFace-v1** (fal) | **Apache-2.0 (P: card metadata and LICENSE.md read)** | ResNet100, 261 MB recogniser file `glintr100.onnx` (P); card gives LFW 0.99650, CFP-FP 0.95186, AgeDB 0.96100, CALFW 0.94700, CPLFW 0.90933 (P); "trained on a commercial dataset ... from various sources" and "**may not extensively cover all ethnicities**" (P, quotes of the card); embedding size not on the card (512 per third parties, S) | **strongest licence-clean candidate; measure on South Asian faces before adopting.** Caveat found: the repository also contains `1k3d68.onnx`, `2d106det.onnx`, `genderage.onnx` and `scrfd_10g_bnkps.onnx`, the same names as InsightFace's `antelopev2` package (P: file listing); the card does not say they are fal's own, and InsightFace's pretrained models are non-commercial (S). **Use only the recogniser file, with YuNet for detection and alignment, until the others are cleared** |
| InsightFace `buffalo_l` / `antelopev2` | code MIT, **models non-commercial research only; commercial licence from InsightFace (S, InsightFace's own pages)** | the common reference | **blocked** for the product; usable only as an offline accuracy reference in the lab |
| EdgeFace (Idiap) | **CC BY-NC-SA 4.0 on the model card (S); paper CC BY-NC-ND (S)** | 1.77 M parameters; LFW 99.73, IJB-B 92.67, IJB-C 94.85 (S) | **blocked** (non-commercial); the architecture is a useful reference |
| AdaFace, MagFace, others | code MIT in several repositories (S) | | to research next: weights are usually trained on MS1MV2 or WebFace, i.e. the dataset problem below |

**Training-data provenance (S, one NeurIPS 2021 study and dataset guides).** MS-Celeb-1M and VGGFace2 were licensed for non-commercial research and retracted by their owners in 2019, but mirrors and models derived from them circulate; most repositories with pre-trained models dropped the restriction; whether a model trained on such data may be used commercially is an open legal question. WebFace260M and Glint360K terms were not found. **Action for the owner: ask counsel about training-data provenance before any face model is called "cleared for product use"; until then the status is "measured for development only".**

## 5. Face quality and what the scores mean

**OFIQ** (BSI, the reference implementation of ISO/IEC 29794-5 face image quality): C/C++, code MIT per Busch's slides and a PyPI wrapper (S), **its model files have their own terms** (S: the wrapper says to read the licence files before redistribution). Use: refuse to enrol or search with a face whose quality is too low, and report the quality next to every score, because most false matches and misses of face systems come from low-quality captures. Verify the licence on the BSI repository before use.

## 6. How well do these work on South Asian faces?

* **No Pakistani-specific public verification benchmark was found.** Related sets exist (IJB-C with its regional mix; Indian-subset benchmarks), none checked for licence here.
* **RFW** (Racial Faces in the Wild) has an Indian subset of about 3 K identities; in its paper an ArcFace ResNet-34 trained on CASIA-WebFace scored 88.00% on Indian, 92.15% on Caucasian, 83.98% on Asian, 84.93% on African (S, via a survey table): a dated reference point, not our model.
* **NIST FRTE** reports demographic differentials by region of birth including South Asia (S: NIST slides list the groups; the figures are in NISTIR 8280 and 8429, not read here).
* AuraFace's own card says its training data may not cover all ethnicities (P). **So the accuracy figures above are on mostly Western sets and must be re-measured on faces like the ones in our cases** (consented or synthetic), by subgroup, before any threshold is set.

## 7. Index and search architecture

* **pgvector 0.8.7 (P: README read, truncated after about 40 K characters):** HNSW supports `vector` up to 2,000 dimensions and `halfvec` up to 4,000; **filtering is applied after the index scan** (with the default `ef_search` of 40 a condition matching 10% of rows returns about 4 rows), so per-case filters need the iterative scans of 0.8.0 (`hnsw.iterative_scan`) or partitioning by case; building is fast only when the graph fits `maintenance_work_mem`; recall is checked against exact search. 512-d face embeddings and 768-d or 1152-d image embeddings are within the limits. No benchmark for millions of 512-d vectors was found (S): measure on our embeddings.
* **Face search design:** detect, align, quality-check, embed; store the embedding with the box, the quality figure and the source citation; search returns **every candidate above a threshold chosen from a measured false-match rate**, sorted, with scores and quality, plus the count of candidates just below it; an analyst confirms; enrolment is explicit and audited. Clustering of unknown faces (same person across files) is a second step.
* **Image search design:** three complementary indexes behind one question: (1) text read inside the image (done: OCR), (2) a semantic embedding (text-to-image and image-to-image), (3) object and scene labels from a detector or a vision-language model written at ingest as derived artifacts with confidence. The runtime language model decides which to use for a question; no template per phrasing.

## 8. Image and scene retrieval models

| Candidate | Licence | Evidence | Verdict |
|---|---|---|---|
| SigLIP (v1 base, in the product) | Apache-2.0 | | baseline |
| **SigLIP 2** (`google/siglip2-base-patch16-224`, `so400m-patch14-384`) | **Apache-2.0 (P: card metadata)** | 0.4 B and 1 B parameters (P); multilingual (Gemma tokenizer, 256 k vocabulary) and trained on 90% English and 10% non-English pairs (S, paper summaries); retrieval figures are in a table image on the card that could not be read | **candidate**: measure Urdu-caption retrieval (Crossmodal-3600 has Urdu? not confirmed) against v1 |
| DINOv2 | Apache-2.0 since Aug 2023 (S) | good for instance-level and near-duplicate retrieval (general knowledge, not verified here) | candidate for "find this object or the same photo" |
| DINOv3 | custom licence, commercial use allowed, needs legal review (S) | | later |
| Grounding DINO / OWLv2 | Apache-2.0 (Grounding DINO S; OWLv2 not confirmed on a primary page) | text-prompted detection; slow | candidate for scene and object labels at ingest, GPU host likely |
| RF-DETR (N to L) | Apache-2.0 (S); XL and 2XL use another licence | closed-set detector | candidate for fixed classes (vehicles, weapons, bags) |
| Vision-language models (Qwen2-VL, PaddleOCR-VL, Florence-2) | Apache-2.0 / MIT (to verify) | captions and labels at ingest | CPU-heavy; GPU host |

## 9. Proposed bake-off (to be pre-registered before any measurement)

1. **Detection**: YuNet (and BlazeFace if verified) on public sets with small and rotated faces; measure recall at a fixed false-positive rate, CPU milliseconds, memory.
2. **Recognition**: SFace against AuraFace (recogniser file only, YuNet alignment) on public verification sets (LFW, CFP-FP, AgeDB, CALFW, CPLFW: public benchmarks named on AuraFace's card) **and on a South Asian set** (consented, or synthetic with a stated limit): verification accuracy, true accept rate at false accept rates of 1e-3 and 1e-4, per subgroup, with OFIQ quality bins. Gates fixed first, sealed rows read once.
3. **Index**: pgvector HNSW with per-case filtering at 10 K, 100 K and 1 M face vectors: recall against exact search, latency, memory.
4. **Image retrieval**: SigLIP v1 against SigLIP 2 against DINOv2 on a public retrieval set plus a small Urdu-caption set of our own; and the three-index design on synthetic cases.

## 9a. Additions from primary pages read later on 2026-11-19 (while pre-registering the bake-off)

* **BlazeFace short range (Google MediaPipe) is verified Apache-2.0 on Google's own model card (P: card PDF, "LICENSED UNDER Apache License, Version 2.0", dated 2021-06-09, 224 KB model, input 128x128).** The same card lists as **out of scope**: "any form of surveillance or identity recognition", counting faces in a crowd, faces looking away or strongly inclined, and people more than 2 m away; its stated domain is faces whose box sides are at least 20% of the image side, tilt up to 45 degrees (P). It was trained on consented smartphone AR images (P). So it is a licence-clean detector for prominent faces in handheld footage, **not** a detector for CCTV scenes, and its card tells forensic users it is not meant for them. The bake-off measures how far that matters (gate D2).
* **OFIQ**: code is **MIT** (P: `LICENSE.md` of the BSI repository); the model files are **not** in the repository: they are downloaded from the ISO portal (`OFIQ-MODELS.zip`, 422,400,019 bytes) or from eu-LISA, and "the license situation of the model files is documented separately in license files placed in the sub-directories after download" (P). The model licence files are therefore read after the download and copied into the results. The Python package `ofiqpy` 0.2.2 is a **third-party reimplementation** that claims 787/787 conformance with the C++ reference (P: its PyPI page); gate Q1 checks that on the BSI conformance images instead of trusting it. The canonical `ofiq_config.jaxn` hash given by `ofiqpy` was reproduced from the v1.1.0 tag (P: e1172867...).
* **The product's SFace is not the OpenCV Zoo ONNX file.** The configured face model is `face-detect-yunet-sface.gguf` from the LocalAI gallery, a GGUF packaging from the repository `mudler/face-detect-gguf` with sha256 `9ce78d4b...` and a default verification threshold of 0.363 (P: `gallery/index.yaml` of this repository). The OpenCV Zoo ONNX file has sha256 `0ba9fbfa...` (fetched and hashed today). The bake-off measures the ONNX file; **the GGUF conversion is not measured and its equivalence to the ONNX file is not verified**. The face and image-embedding roles are both **off** in the live worker (`FORENSIC_FACE_ENABLED=false`, `FORENSIC_IMAGE_EMBEDDING_ENABLED=false`, read from the container environment today), so the earlier sentence "already in the product" meant "configured", not "running".
* **SFace's paper** (P: arXiv 2205.12010 abstract) trains on CASIA-WebFace, VGGFace2 and MS-Celeb-1M and does not say which one the OpenCV file uses; the ledger's "CASIA-WebFace reported" stays unverified.
* **FairFace** (labels race / gender / age, 108,501 images, validation split 10,954) is **CC BY 4.0** for images and labels (P: the authors' README and the hub metadata); its "Indian" class is the closest public handle on South Asian appearance, but it has **no identity labels**, so it can measure false-match rates and detection by group, not genuine-pair accuracy. **WIDER FACE** is **cc-by-nc-nd-4.0** on the hub (P): development evaluation only.
* **Verification-set mirror:** the five `.bin` sets exist on an unofficial hub mirror without a card; the files are Python pickles, so they are read only with a restricted reader (only numpy array reconstruction allowed; confirmed: nothing else was in them).
* **Where else South Asian genuine pairs might come from (not fetched, S: my recollection, not looked up today):** RFW and IJB-C need a request to their owners; Meta's Casual Conversations (consented participants; I believe its second version adds South Asian countries) is gated behind an agreement. The owner's decision is whether to request one; until then genuine-pair accuracy on South Asian people is not measured.

## 10. Needs from the owner

* Counsel's view on **training-data provenance** for any face model (SFace, AuraFace, others).
* **Consented real faces** (a small set) or approval to use synthetic faces only, with the limit stated in every result.
* The accepted **false-match rate** the analysts can work with (it sets the threshold).

## Sources

Primary pages read: [AuraFace-v1 model card](https://huggingface.co/fal/AuraFace-v1), its [LICENSE.md](https://huggingface.co/fal/AuraFace-v1/blob/main/LICENSE.md) and [file listing](https://huggingface.co/fal/AuraFace-v1/tree/main); [OpenCV Zoo YuNet](https://github.com/opencv/opencv_zoo/tree/main/models/face_detection_yunet) and [SFace](https://github.com/opencv/opencv_zoo/tree/main/models/face_recognition_sface) (README and LICENSE); [SigLIP 2 base](https://huggingface.co/google/siglip2-base-patch16-224) and [so400m](https://huggingface.co/google/siglip2-so400m-patch14-384); [pgvector README](https://github.com/pgvector/pgvector).
Secondary (to verify): [InsightFace licensing](https://www.insightface.ai/solutions/face-recognition-licensing) and [model zoo README](https://github.com/deepinsight/insightface/blob/e68b8b076fb11e9f75c87244d234385f0f318181/model_zoo/README.md); [EdgeFace paper](https://arxiv.org/abs/2307.01838v2) and [model card](https://huggingface.co/Idiap/EdgeFace-Base); [SigLIP 2 paper](https://arxiv.org/pdf/2502.14786); [RFW paper](https://arxiv.org/pdf/1812.00194); [NIST FRTE 1:1](https://pages.nist.gov/frvt/html/frvt11.html) and [demographics slides](https://pages.nist.gov/frvt/html/slides/demographics_1N.pdf); [OFIQ at BSI](https://www.bsi.bund.de/dok/OFIQ1-e) and the [ofiqpy package](https://pypi.org/project/ofiqpy/); [dataset licensing study](https://par.nsf.gov/servlets/purl/10316366); [Grounding DINO vs RF-DETR](https://playground.roboflow.com/models/compare/grounding-dino-vs-rf-detr); [DINOv3 licence discussion](https://github.com/facebookresearch/dinov3/issues/31); [Indian-face datasets and bias](https://arxiv.org/pdf/2407.15810).
