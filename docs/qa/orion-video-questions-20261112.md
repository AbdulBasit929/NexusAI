# Orion Lahore Reference: questions about the five real videos

Collection `orion-lahore-reference-20261030`. Every question below was put to the live engine on 2026-11-12 in these words and answered correctly: the answer matches a key computed from the stored readings by a query that does not use the analyst views.
The readings are the models' (plates, speech, text): they are candidates that a person must check, never an identification. Times are seconds from the start of the video.

## One video

| Question | The answer it gives | Seconds to answer |
|---|---|---|
| How many different number plates were read in highway-4k.mp4? | 3 | 50 |
| At what second of highway-4k.mp4 does the first plate appear? | 0.84 | 18 |
| Which plates were read between 0:10 and 0:20 of highway-4k.mp4? | 1 plate (read from the footage: see the private sheet) | 28 |
| Which plate stays in view the longest in highway-4k.mp4? | 1 plate (read from the footage: see the private sheet) | 21 |
| On how many frames of highway-4k.mp4 was a plate read? | 46 | 32 |
| How long is the video highway-4k.mp4? | 21.52 | 34 |
| How many different number plates were read in highway-overpass-hd.mp4? | 9 | 48 |
| At what second of highway-overpass-hd.mp4 does the first plate appear? | 0.47 | 28 |
| Which plate stays in view the longest in highway-overpass-hd.mp4? | 1 plate (read from the footage: see the private sheet) | 38 |
| On how many frames of highway-overpass-hd.mp4 was a plate read? | 78 | 50 |
| How long is the video highway-overpass-hd.mp4? | 15.02 | 10 |
| How many different number plates were read in street-junction-hd.mp4? | 1 | 44 |
| At what second of street-junction-hd.mp4 does the first plate appear? | 2 | 20 |
| Which plate stays in view the longest in street-junction-hd.mp4? | 1 plate (read from the footage: see the private sheet) | 24 |
| On how many frames of street-junction-hd.mp4 was a plate read? | 29 | 40 |
| How long is the video street-junction-hd.mp4? | 19.02 | 10 |
| How many different number plates were read in street-night-hd.mp4? | 5 | 54 |
| At what second of street-night-hd.mp4 does the first plate appear? | 13.68 | 27 |
| Which plates were read between 0:10 and 0:20 of street-night-hd.mp4? | 2 plates (read from the footage: see the private sheet) | 31 |
| Which plate stays in view the longest in street-night-hd.mp4? | 1 plate (read from the footage: see the private sheet) | 23 |
| On how many frames of street-night-hd.mp4 was a plate read? | 208 | 38 |
| How long is the video street-night-hd.mp4? | 36.97 | 9 |
| How many different number plates were read in traffic-control-cctv-4k.mp4? | 29 | 72 |
| At what second of traffic-control-cctv-4k.mp4 does the first plate appear? | 0 | 36 |
| Which plates were read between 0:10 and 0:20 of traffic-control-cctv-4k.mp4? | 8 plates (read from the footage: see the private sheet) | 52 |
| Which plate stays in view the longest in traffic-control-cctv-4k.mp4? | 1 plate (read from the footage: see the private sheet) | 40 |
| On how many frames of traffic-control-cctv-4k.mp4 was a plate read? | 330 | 50 |
| How long is the video traffic-control-cctv-4k.mp4? | 60 | 14 |

## Across the videos

| Question | The answer it gives | Seconds to answer |
|---|---|---|
| Which video shows the most different plates? | traffic-control-cctv-4k.mp4 | 89 |
| Which plates were read in more than one video? | none | 59 |
| How many of the videos have an audio track? | 5 | 100 |
| Which video is the longest? | traffic-control-cctv-4k.mp4 | 111 |
| How many different plates were read across all the videos? | 47 | 171 |

## Speech and text read from the videos

| Question | The answer it gives | Seconds to answer |
|---|---|---|
| Which videos have speech in them? | highway-4k.mp4, highway-overpass-hd.mp4, street-junction-hd.mp4, street-night-hd.mp4, traffic-control-cctv-4k.mp4 | 91 |
| Which video has the most text visible in it? | street-night-hd.mp4 | 136 |

## More wordings that were put to the live engine on this case and answered correctly

| Question | The answer it gives |
|---|---|
| How many minutes of footage are there altogether in the videos? | 2.5 |
| What is the total length of all the videos in seconds? | 152.53 |

## Typed plate search (the questions an investigator asks first)

Ask: **Which photos or videos show the plate &lt;a plate&gt;?** The answer names each clip that holds the reading, the moments, and lists a reading that differs by one or two characters apart (O and 0, I and T) as "check by eye". It never prints a stored
reading: the only plate in the answer is the one you typed. Plates are matched exactly. The plates read from the real footage are in a private file on this machine (`C:\NexusAI-Showcase\orion\video-private-20261112.md`), which is not part of the repository.
For each of the five clips, the plate that stays in view longest was typed in these words on 2026-11-12 and the right clip came back with openable moments in 13 to 22 seconds.

## What to do in the workspace (port 4182)

1. Open the case **Orion Lahore Reference**, then **Ask**. The header says how many sources are ready; sources that are still being read are not searched.
2. Type a question from the tables above, in the same words. A question takes 20 to 60 seconds on this machine; the page says so while it works, and it keeps running if you leave the page.
3. The answer carries the label **Model observation**. Under it, **Behind this answer** says how many readings are behind it and that they are not recomputed from the records.
4. Open **Behind this answer**: the readings are listed with their file and second; under **Sources** each reading is a **Place** (for example "25.49 seconds in source"). Opening a place shows the video at that second with the banner
   "Exact source location" (the start and the end second of the stay).
5. A new video can be added from **Evidence, Add evidence**: it appears as queued, then ready, and is searched once it is ready. About 8 to 12 times its length for HD and about 20 times for 4K on this machine.

## What does not work yet (stated, not hidden)

- "How many minutes of video does the case hold in total?" makes the engine ask a clarifying question on this case ("how many minutes should the co-location window be"): the case also holds call records, whose analysis takes a window in minutes.
  Use one of the two other wordings above.
- A plate is matched exactly; a plate read with a letter that looks like a digit is listed as a similar reading, not as the same plate.
- A reading is a model's candidate: a plate, a piece of speech or a piece of text on a frame can be wrong and must be checked by eye. Urdu, Punjabi and Pashto speech is less reliable than English.
- One clip (`highway-overpass-hd.mp4`) has 16 speech segments, not 3: it was read twice (the first time lost its plate readings to a defect fixed the same day) and a reprocess keeps the earlier outputs. A question about its speech segments counts both.
- Questions that name a video by a description ("the overpass clip") and not by its file name are not mapped to a file.

## How it was verified

The key of each question is computed from the stored readings of the case by queries that do not use the analyst views the engine reads (`tools/video_plates/video_question_bank.py`, `build`). 36 questions were run on the live engine on 2026-11-12:
35 correct (the one that was declined on the first run was a defect found by it and fixed the same day), none wrong; the 36th is the clarifying question above. Median 37 s, slowest 171 s. The sheet is rebuilt by `tools/video_plates/build_video_sheet.py` from a key and the results of a run.

