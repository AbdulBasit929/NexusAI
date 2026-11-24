# Video evidence beyond plates: pre-registration (2026-11-19)

**Written before any measurement of the components named here.** Research: `docs/research/VIDEO_EVIDENCE_RESEARCH_20261119.md` (use cases U1 to U12 and the order). This registration fixes data, protocols and gates for the first four stages (U1, U2, U3/U4, U8) with U9 and U10 riding on U1; faces in video (U5), plate voting (U6) and re-identification (U7) wait for the face gates and a consented multi-camera set. Changes are dated amendments at the end. Results: `video-evidence-results-<date>.md`. **No video measurement of this registration has been run yet.** Rules of every media programme apply: public or synthetic data first, sealed rows read once per method, one heavy job at a time in a memory-capped container without network, medians for timings, nothing deployed or enabled by this work.

## 1. Data

| Id | Set | Truth | Licence / source | Use |
|---|---|---|---|---|
| VD1 | **Synthetic static-camera scenes** from the scene generator (`tools/showcase/build_plate_scenes.py` and a small extension for pedestrians and vehicles crossing, day/night, rain, a burned-in clock, planted edits) | exact: object tracks, motion intervals, clock text, edit positions | own generator; backgrounds permissively licensed | U1, U2, U8 gates |
| VD2 | The five real public clips already used for plates (`C:/tmp/video-plates`) with human-checked counts of people and vehicles per 10 s | by eye, recorded in the results | as fetched for the plate work | sanity on real footage, not a gate by itself |
| VD3 | A public tracking benchmark subset (MOT17 train sequences) **only if its licence is read first and recorded**; otherwise not used | benchmark labels | licence not yet read (S: research use) | development scoring of U2 only |
| VD4 | Crop sets for appearance search: person and vehicle crops cut from VD1 and VD2, each labelled with colour and type words | by construction (VD1) and by eye (VD2) | as above | U3 |
| VD5 | Frames with a stated object list (for captions) from VD1 with exact truth | exact | own | U4 captions |

**Partition.** Scenes and clips are assigned to `dev` or `sealed` by the last hex digit of the SHA-256 of the scene id (`0` to `9`, `a`, `b` dev; `c` to `f` sealed). Thresholds and tracker parameters are chosen on dev; sealed is read once per method.

## 2. Stages, methods and protocols

* **U1 overview.** *Motion index:* frame differencing on a downscaled grey stream (the threshold and minimum interval length chosen on dev), merged into intervals. *Shots:* PySceneDetect content detector. *Clock OCR:* the worker's existing OCR on the burned-in clock at one frame per minute, drift and jump detection. *Same-footage (U9):* exact file hash, then a perceptual frame-hash signature; on VD1 clips re-encoded at three bitrates, cropped by 5% and mirrored, with unrelated clips as negatives. *Clip export (U10):* cut, burn-in, manifest with source hash; the cut is verified by decoding and comparing frame counts and hashes of the unmodified streams.
* **U2 detect and track.** Detector candidates: RF-DETR nano and small (Apache-2.0) and RT-DETR R18 (Apache-2.0); trackers: ByteTrack and BoT-SORT (MIT). Frames sampled at 3 per second, input long side 640. Classes: person, car, motorcycle, bus, truck, bicycle. Scoring on VD1 sealed: recall and precision of objects at least 32 pixels high at IoU 0.5, id switches, HOTA if the evaluation code is available offline; counts per 10-second window against truth; CPU time per video minute and peak memory with 4 threads. On VD2/VD3: reported, not gated.
* **U3 appearance search.** SigLIP 2 base and SigLIP v1 base embeddings of VD4 crops; text queries of the form "<colour> <type>"; recall@10 and precision@10 per query, all matches above a closeness bar returned (not the single best); the same crops with DINOv2 for crop-to-crop search.
* **U4 moment search and captions.** Frame embeddings at 1 frame per second, text query to a ranked list of intervals (interval hit if it overlaps the truth interval); **captions** by SmolVLM2-256M and 500M on 8 frames per clip: time per clip, peak memory, yes/no object-presence accuracy against the object list of VD5, and **hallucination rate** (a caption that names an object absent from the frame).
* **U8 integrity indicators.** On VD1 clips with planted edits (a dropped run of frames, a duplicated run, a timestamp jump in the container, a re-encode at lower quality, a spliced segment from another clip) and untouched copies: the detector of each indicator (frame-difference duplicates, presentation-timestamp gaps from ffprobe, GOP/encoder signature change, clock-OCR jump) is scored for recall of planted edits and false alarms on untouched clips. Output wording is "indicator", never "tampered".

## 3. Gates (fixed now; "met" or "NOT met", never softened)

* **V-A (motion index)** on VD1 sealed static-camera scenes: recall of truth motion intervals at least 95%, at most 1 false interval per hour of video, and the indexed intervals cover at most 10% of the duration where the truth covers at most 5%.
* **V-B1 (detection and tracking quality)** on VD1 sealed: recall of objects at least 32 px high at least 80% and precision at least 80% for the best detector/tracker pair, counts per 10-second window within one object of truth in at least 90% of windows.
* **V-B2 (cost)**: the chosen pair processes a minute of 1080p video at 3 frames per second in at most 2 minutes with 4 threads and at most 2 GB resident memory (so 12 hours of motion-indexed video can be read in a working day). Otherwise only the motion intervals are analysed, and the result says so.
* **V-C (appearance search)**: recall@10 at least 80% for colour+type queries on VD4 sealed, and the all-matches-above-the-bar list contains at least 90% of the true matches; SigLIP 2 is "preferred over v1" only if it is not worse on either figure.
* **V-D1 (moment search)**: the truth moment is inside one of the top 5 intervals for at least 70% of queries on VD1 sealed.
* **V-D2 (captions)**: object-presence accuracy at least 90% and hallucination rate at most 5% on VD5 sealed for a caption to be stored as "model description" at all; time per clip at most 60 s on this CPU. If not met, captions are used only as search hints and never shown as a statement about the video.
* **V-E (integrity indicators)**: recall of each planted edit type at least 95% with at most 2% false alarms on untouched clips, for the indicators that are to be shown; an indicator below this is shown as "weak" or not at all. Double-encoding detection is measured and reported without a gate (the published methods degrade as the last compression gets stronger).
* **V-F (clip export)**: every exported clip decodes, its frame count equals the requested interval, and the source hash and manifest verify; zero failures on 30 exports.
* **Decision rule.** A stage that meets its gates is recommended for a switch behind a flag; nothing is enabled or deployed here, and the owner's word is needed for each.

## 4. What this cannot show (stated in advance)

Synthetic footage is easier than real CCTV; real-footage figures are by eye on a handful of clips; night, rain, compression and fisheye cameras are only represented as far as the generator simulates them; re-identification, faces in video and plate voting are not measured here; all figures are tier T1/T2 (synthetic or public), not T4 (authorised real evidence).

## 5. Needs from the owner

Authorised or consented real CCTV clips with someone who can say what they show; a decision about a GPU host for captions and larger detectors; the licence stance for research-only tracking datasets used only for development scoring.

## 6. Amendments

(none yet)
