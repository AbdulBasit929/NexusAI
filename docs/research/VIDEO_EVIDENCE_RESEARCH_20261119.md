# Video evidence beyond plate reading: what to build on (research, 2026-11-19)

Third of the media-track research documents (faces and image search: `FACE_IMAGE_SEARCH_RESEARCH_20261119.md`; Punjabi and Saraiki speech: `PUNJABI_SARAIKI_STT_RESEARCH_20261119.md`). Owner direction, 2026-11-19: research the best available options, then implement and measure video evidence use cases **beyond plate reading**. The roadmap (`docs/roadmap/master-roadmap-20261103/02_media_programme.md` section 6.2) already lists the stages; this note checks the choices against primary pages, adds the forensic-practice view, puts CPU cost and evaluation data next to each stage, and ranks what to do first. **Every claim is marked P (read on the primary page or API today) or S (secondary: a search-result summary or my own recollection, to be verified before it is relied on).** Licence statements are what the pages say, not legal advice. Pre-registration: `docs/data/video-evidence-preregistration-20261119.md`.

## 1. Rules

* Evidence stays on the premises; originals are immutable; every derived artifact carries model, version, hash, parameters and a confidence, and says "model observation, review required".
* A model licence is not a blocker for development (owner, 2026-11-17) but is recorded; AGPL code or weights (Ultralytics, BoxMOT) never ship (roadmap decision, confirmed below).
* CPU first: the laptop has 16 GB beside the stack; a GPU host is a later, labelled option.
* Faces in video are governed (levels L0 to L3 of the roadmap section 7); a face match is a lead, never an identity. Nothing here identifies a person or a vehicle's owner; a track is "bounded adjacent-frame readings, not a journey".
* Indicators of tampering are indicators, never proof. Deepfake classifiers are not used to say "real" or "fake" (section 4, U8).
* Graphic material opens blurred by default (analyst wellbeing, roadmap rule 9).

## 2. What the product does with video today

* **Plates in video (V4)**: live since 2026-11-11 on the owner's word; frames at 4 per second, two plate engines, grouping of reads of one plate, a coverage receipt; cost 4 to 6 times the video length at 1080p and 10 to 13 times at 4K (this CPU), job stops at a wall-clock budget (`tools/media_data/VIDEO_GO_LIVE.md`, P).
* The older path samples up to 12 frames, one every 5 s, and reads each as an image; the soundtrack is sent through the speech route; face detection exists as a switch (YuNet, level L0, **off**).
* Not built: shots and motion index, clock OCR, detection and tracking, appearance search, captions, integrity indicators, clip export, re-identification.

## 3. What forensic practice asks of video (so the use cases follow the work, not the technology)

SWGDE's current documents (S: read through a search summary, the PDFs are linked below): *Best Practices for Digital Forensic Video Analysis* (18-V-001, v1.1, 2024-03-22) lists metadata collection, structural analysis, macroblock analysis, format conversion, **timeline reconstruction**, frame-level analysis, and **enhancement** operations (noise or pattern removal, frame averaging, levels, stabilisation, interpolation, edge sharpening); *comparison* means extracting frames for photographic comparison; *Best Practices for Digital Video Authentication* (23-V-001, v1.2, 2024-03-07) names three tasks: **contextualisation** (provenance and history of the file), **source identification** (which camera or equipment made it) and **content authentication** (visual changes in the encoded streams). The roadmap's "integrity indicators", "clips and export" and "overlay time OCR" rows come from this practice. Two rules follow: enhancement must be reversible and logged, never generative on evidence crops (it invents detail), and every derived clip carries the source hash, in and out points and the operator.

## 4. Use cases, ranked by value to an analyst and feasibility on this CPU

| # | Use case | What it gives | Build on (licence) | CPU cost | Verdict |
|---|---|---|---|---|---|
| U1 | **Time-indexed overview**: probe, proxy, thumbnails, **motion-only index** (a 12-hour static camera becomes a short list of intervals), shot boundaries, burned-in **clock OCR** with drift and jump detection | the biggest time saver for CCTV; feeds the clock-offset table and the timeline | ffprobe/ffmpeg (licence depends on the build: LGPL or GPL, the GitHub API reports "other", P), PySceneDetect (BSD-3-Clause, P), PP-OCR already in the worker | low | **first** |
| U2 | **Detect and track persons and vehicles**, count timeline, best crops | "how many people entered between 22:00 and 23:00", feeds plates, faces, appearance | **RF-DETR** (Apache-2.0, P; its COCO paper figures are on a GPU, CPU latency not found, S), RT-DETR (Apache-2.0, P), YOLOX (Apache-2.0, P) as challengers; trackers **ByteTrack** (MIT, P), **BoT-SORT** (MIT, P; for moving cameras), Roboflow `trackers` (Apache-2.0, P); evaluation code TrackEval/HOTA (MIT, S). **Not**: Ultralytics and BoxMOT (AGPL-3.0, P) | medium: the detector at 2 to 5 frames per second dominates; the tracker is negligible (S) | **second** |
| U3 | **Search by appearance**: colour, type, clothing words ("red motorcycle") | words to crops, with every match listed | SigLIP 2 (Apache-2.0, P) embeddings of person and vehicle crops, stored in pgvector; DINOv2 (Apache-2.0, P) for "the same object" | low per crop | **third**; shares the image-retrieval bake-off |
| U4 | **Find the moment by words**: text to frame embeddings with temporal smoothing; optional **captions** written at ingest ("model description, unreviewed", searchable, never counted) | "when does the white van arrive" | SigLIP 2 frames; captions: **SmolVLM2** 256M/500M/2.2B (Apache-2.0, P; Video-MME 33.7/42.2/52.1 per the cards, S; the 500M card quotes about 1.8 GB for video, S), Florence-2 (MIT, P), Qwen2.5-VL-7B (Apache-2.0, P; the 3B card carries no licence tag, P: check), InternVL3-1B (Apache-2.0, P) | captions are slow on CPU and hallucinate (to be measured); frame embeddings are cheap | embeddings now; captions measured on the smallest model first, larger models on a GPU host |
| U5 | **Faces in video** (governed) | person clusters inside one case, the best face per track | the face bake-off (YuNet, SFace/AuraFace, OFIQ) | medium | after the face gates; level L1 first |
| U6 | **Plates with track voting** | character voting across the track | V4 (live) | high | extend what exists |
| U7 | **Cross-camera re-identification** (people, vehicles) | candidate "may be the same" across cameras | torchreid (code MIT, P), FastReID (Apache-2.0, P); the pretrained **weights are trained on Market-1501, DukeMTMC (withdrawn in 2019, S) and similar research-only sets** | medium | **candidate-only, later**; needs a consented multi-camera set to measure |
| U8 | **Integrity indicators**: container and metadata anomalies, duplicated or dropped frames, timestamp gaps, **double encoding** and GOP analysis, source clues | the SWGDE authentication tasks as indicators | ffprobe; double-compression methods exist (VPF, S: commercial implementation; **CoFFEE** H.264 feature extractor, free for research, S); no open-source video tamper toolkit found (S). Deepfake classifiers: 2026 benchmarks report detectors near chance on newer generators (S) | low | indicators only; **no real/fake verdict** |
| U9 | **Same footage across files**: exact hash, then perceptual video hash | "this clip is a re-encoding of that one" | file hashes (exist); PDQ-family hashes live in a repository the GitHub API labels "other" (P), licence file to read before use; our own frame-hash method is measurable | low | after U1 |
| U10 | **Clip export and redaction**: cut with burned-in hash/time watermark and a manifest; blur of bystanders for sharing | court-ready outputs, privacy | ffmpeg, masks from U2 | low | with U1 |
| U11 | Audio track to speech and moments | already routed | speech routes | existing | exists |
| U12 | Anomaly or "suspicious behaviour" alerts | | | | **not recommended**: cannot be defended as evidence, false-alarm rate unacceptable (judgement, no source) |

## 5. Evaluation data (and what each licence allows)

* **Own synthetic scenes with exact truth** (the plate-scene generator `tools/showcase/build_plate_scenes.py` and the Orion showcase videos): objects, tracks, clocks and edits are known, so motion intervals, counts, planted frame drops and re-encodes can be scored exactly. The limit: synthetic footage is easier than real CCTV.
* **The five real public clips** already used for plates (`C:/tmp/video-plates`, checked by eye): a few real scenes with human-checked truth.
* **Public tracking benchmarks** (MOT17 and MOT20 train sequences, DanceTrack, UA-DETRAC): licences were **not found on any primary page today** (S: they are generally research or non-commercial). They are candidates for development scoring only, with the licence read before fetching.
* **Re-identification sets** (Market-1501 and similar): research-only (S). Not used for anything that ships.
* **Authenticity**: no licensed forensic-grade tampered-video benchmark was verified; planted edits on our own clips are the plan.
* **Real casework footage**: the only true measure; consented/authorised clips (D26) remain **not measured**.

## 6. Recommended order

1. **U1 + U10 + U9** (overview, clip export, same-footage): cheap, deterministic, testable with exact truth, and the base for everything else.
2. **U2** detection and tracking with the Apache/MIT stack, measured on CPU, with the timeline of counts and the best crops.
3. **U3/U4** appearance search and moment search on frame and crop embeddings, sharing the image-retrieval decision; captions measured on SmolVLM2-500M on this CPU for time and hallucination before anything larger is considered.
4. **U8** integrity indicators with planted-edit truth.
5. **U5/U6/U7** faces in video, plate voting and re-identification, each behind its own gate and governance.

## 7. Needs from the owner

Authorised or consented real clips (CCTV-like, several cameras, day and night) with a person who can say what is in them; a decision on whether a GPU host may be used for captions and larger detectors; the licence stance for research-only tracking datasets used for development scoring; native-language review of any caption wording shown to analysts.

## Sources

Primary pages read today: licence files through the GitHub API for [rf-detr](https://github.com/roboflow/rf-detr), [ByteTrack](https://github.com/FoundationVision/ByteTrack), [BoT-SORT](https://github.com/NirAharon/BoT-SORT), [deep-person-reid](https://github.com/KaiyangZhou/deep-person-reid), [fast-reid](https://github.com/JDAI-CV/fast-reid), [PySceneDetect](https://github.com/Breakthrough/PySceneDetect), [roboflow/trackers](https://github.com/roboflow/trackers), [SAM 2](https://github.com/facebookresearch/sam2), [Grounding DINO](https://github.com/IDEA-Research/GroundingDINO), [RT-DETR](https://github.com/lyuwenyu/RT-DETR), [YOLOX](https://github.com/Megvii-BaseDetection/YOLOX), [boxmot](https://github.com/mikel-brostrom/boxmot) and [ultralytics](https://github.com/ultralytics/ultralytics) (AGPL-3.0); Hugging Face licence metadata for [SmolVLM2-500M](https://huggingface.co/HuggingFaceTB/SmolVLM2-500M-Video-Instruct), [Florence-2](https://huggingface.co/microsoft/Florence-2-base), [X-CLIP](https://huggingface.co/microsoft/xclip-base-patch32), [VideoMAE](https://huggingface.co/MCG-NJU/videomae-base) (CC-BY-NC), [Qwen2.5-VL-7B](https://huggingface.co/Qwen/Qwen2.5-VL-7B-Instruct), [SigLIP 2](https://huggingface.co/google/siglip2-base-patch16-224), [DINOv2](https://huggingface.co/facebook/dinov2-base), [InternVL3-1B](https://huggingface.co/OpenGVLab/InternVL3-1B); the product's own `VIDEO_GO_LIVE.md` and the roadmap section 6.2.
Secondary (to verify): SWGDE [Digital Forensic Video Analysis 18-V-001](https://www.swgde.org/wp-content/uploads/2024/04/2024-03-22-SWGDE-Best-Practices-for-Digital-Forensic-Video-Analysis-18-V-001-1.1.pdf) and [Digital Video Authentication 23-V-001](https://www.swgde.org/wp-content/uploads/2024/03/2024-03-07-SWGDE-Best-Practices-for-Digital-Video-Authentication-23-V-001-1.2.pdf); [RF-DETR paper](https://arxiv.org/pdf/2511.09554); [SmolVLM2 cards](https://huggingface.co/HuggingFaceTB/SmolVLM2-2.2B-Instruct); [VPF double-encoding analysis](https://www.forensicfocus.com/articles/how-to-check-video-integrity-by-detecting-double-encoding-with-vpf-analysis/); [CoFFEE](https://lesc.dinfo.unifi.it/papers/coffee/); [2026 deepfake video benchmarks](https://arxiv.org/abs/2602.04939); [HOTA metrics](https://github.com/JonathonLuiten/HOTA-metrics).
