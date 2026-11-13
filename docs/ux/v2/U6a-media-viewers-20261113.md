# U6a: the media viewers (audio, video, photo), 2026-11-13

Owner brief, same day: an audio viewer with the status labels of the speech routes; the plates read in photos and in video listed beside the file when it is opened; the text on the video clean and readable, and the detected region **left original, never overwritten**; the design to be up to date, elegant and clean.

## What was wrong first (a data fault, not a style one)

The classic viewers looked for the transcript, the regions and the video events as a **list inside one derived artifact** (`metadata.segments`, `metadata.regions`, `metadata.events`). The worker stores **one artifact per reading** (`forensics.audio-timestamp-segment/v1`, `forensics.anpr-observation/v1`, `forensics.video-anpr-plate-group/v1`, `forensics.image-ocr-observation/v1`), with the box or the time in `citation_locator` and the reading in `metadata.observation`. So the audio viewer said "No transcript was supplied" for every recording the worker had read (Whisper's too), the photo viewer said "No OCR or plate regions", and the video viewer listed none of its plates. Fixed at the source in two pure modules with tests: `lib/audioSegments.js` and `lib/mediaReadings.js`. (A second old fault, found on the way: "no cited time" became second 0, so every audio file showed a cited first segment and a "Source evidence" badge.)

## What a person sees

* **Audio.** One quiet note above the player says what the text is: *Machine transcript* (check it), *Search lead* (find the recording by a spoken word, not a transcript) or *Pointer to listen* (the language is read poorly), with the language and only the figures that were measured and recorded with the route (about N words in 10 wrong, N% of spoken words found, public speech). A thin strip under the waveform marks every spoken segment (click to seek; the segment being played is brighter). The transcript beside it shows the span of each segment, follows the playback, searches, and sets right-to-left text in its own language and font. A reading that stopped at its time budget says where.
* **Video.** Two columns when the column is wide (the plates beside the player), one when it is narrow. On the picture: **an outline only** around each plate and each word read at the playing second, with its label **outside** the box on a solid chip (above, or below at the top edge). Nothing is painted inside a box: the detected pixels stay as they are. At the top of the list: the **original region cut from the frame** (not sharpened, not redrawn, only fitted to a height), the reading and its confidence. Under it, one row per plate: the seconds it is seen, the frames that agree, a confidence bar; a click goes to the first second; the readings behind a plate open, each its own click, with the characters that differ from the settled plate marked (a look-alike O/0, I/T is seen at once). The words read in the frames are listed.
* **Photo.** The same rows beside the picture: each plate and each piece of text with the **original crop** of its region next to the reading, so that the eye can compare what was read with what is there (on the Orion photo the plate reader said `SYNI2905`, the crop and the text reader say `SYNT2905`, and the flag says the format is not a known one). Boxes are outlines; labels stay upright and the same size in a zoomed or turned image.
* **Everywhere.** A plate is a model reading to check by eye, not an identification; it says so once, in one line, above the list.

## Design rules used (the new language, `--nx-*` tokens, layer `nx-patterns`)

Hairline borders and one surface per thing, no card in a card; ink for the reading and one signal colour for state (amber: a lead to check; red: a pointer only; the accent: the place being played); monospaced, spaced plates; times in tabular figures; confidence as a short bar and a figure, coloured by the product's three bands; focus rings and keyboard operation kept (Space, arrows, M); two columns only when the **container** is wide (container query), so the layout follows the room the details pane leaves.

## Where

`src/lib/audioSegments.js`, `src/lib/mediaReadings.js` (+ tests); `src/components/viewers/` `SpeechBanner.jsx`, `TranscriptPanel.jsx`, `VideoPlates.jsx`, `MediaMarks.jsx` (plate chip, confidence bar, original-region crop, outline box, video boxes), `MediaPlayer.jsx` (segment bands, a stage for overlays), `ImageInspector.jsx`; `src/components/EvidenceViewers.jsx`; `src/v2/styles/media.css`.

## Checked

Unit tests for the two modules and the components (36 new and updated); the three pages on real data in the browser at 1600, 1280, 768 and 390 px (no sideways scrolling; one column when narrow); click-to-seek on the segment strip and in the list; the original crop is drawn from the frame itself. The whole suite: 86 of 88 files pass; the two that fail (`investigationPresentation`, `InvestigationResult`) read the 62 retained legacy responses, which are not in this worktree, and do not touch the viewers.

## Not done (said plainly)

* The page frame is still the classic evidence page: the page scrolls on a small screen (the design reference wants the artefact to scroll inside one frame); that belongs to the evidence-page redo (U6).
* The waveform is read from the file in the browser (up to 40 MB) and is absent for a bigger file; a server-made waveform is part of the media foundation.
* No word-level timing for the transcript (the route reads in pieces of up to 38 s; frame-level times exist in the model but are not stored).
* Plates of a photo are not compared with the text reader automatically; the crop beside the reading lets the eye do it.
