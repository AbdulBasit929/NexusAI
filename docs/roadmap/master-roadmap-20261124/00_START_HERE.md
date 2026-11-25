# Start here (2026-11-24, updated 2026-11-25)

**2026-11-25: the owner approved Wave 1 and ordered the media data types first.** The order of work for media is in `03_media_first_programme.md` (steps MF-0 to MF-8); it replaces the order of section 6 of the roadmap for the media rows, and the media prompt below follows it. The non-media families continue in parallel only where they need no heavy-job slot.

## The five files of this request

| File | What it is |
|---|---|
| `docs/roadmap/master-roadmap-20261124/03_media_first_programme.md` | **Media first**: per media type the use cases, what is built, the online research of 2026-11-25 with marks, the best implementation, gates, needs from the owner, and the order MF-0 to MF-8 |
| `docs/roadmap/master-roadmap-20261124/01_roadmap_flow_architecture.md` | The roadmap, the flow and the architecture: every data type, remaining media, breadth first and depth after |
| `docs/roadmap/master-roadmap-20261124/02_research_sources.md` | The online and repository research behind it, each source marked primary or secondary |
| `docs/status/complete-status-20261124.md` | What has been done and what is live, as of today |
| `docs/qa/question-catalogue-20261124.md` and `.csv` | Every question the application has been measured on (920 rows) and the questions it cannot answer yet |

## Should you start a new main chat?

**Yes.** This conversation has been compacted several times and carries two sessions' worth of detail; everything that matters is now in the four files above, in `NEXUSAI_CONTINUATION.md` and in the memory index, which a new chat loads by itself. A new chat starts clean, reads those files and does not carry the old wording experiments or tool traps. Keep the other sessions as they are: **one main (lane) chat** for Ask, text search, the registry, cross-case and the new data families; **the media session** for speech, images, plates, video and faces; **the interface session** for `NexusAI-ui`. Only one heavy job (model run, indexing, restore, image build) at a time on the laptop.

Start the new main chat in `C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork` (branch `lane-arms`) and paste the first prompt below.

### Prompt for the new main (lane) chat

```
You are continuing NexusAI, a local case-based forensic analysis workspace, in C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork (branch lane-arms).
Read in this order before doing anything: AGENTS.md, NEXUSAI_CONTINUATION.md, docs/roadmap/master-roadmap-20261124/00_START_HERE.md,
docs/roadmap/master-roadmap-20261124/01_roadmap_flow_architecture.md, docs/status/complete-status-20261124.md, docs/roadmap/reconciled-plan-20261124.md.
Then reply in at most 12 lines: what is live, what the next three steps are, and which of my words you need.

Standing rules (owner): questions are answered by a model-written query at run time, no templates or keyword rules; a search returns every occurrence across
documents, pictures, plates, recordings and video; a model's reading is a candidate, never an identification; every capability is behind a switch with a
pre-registered gate, a fresh bank for certification, a rollback tag, and my chat word for every deploy, migration, push, database write or --no-verify commit;
commits use explicit paths and end with Assisted-by and Claude-Session lines, never Co-Authored-By of an AI; real data never enters the repository;
CPU only (no GPU); one heavy job at a time on the laptop; build the API from HEAD and compare it with the live binary; never run docker compose down,
never stop the model container, recreate the API only with tools/dev/live-window/3-recreate-api.ps1.

Your lane: Ask, the text search, the identifier registry, cross-case, and the new typed families (messages, email, phone extractions, tower positions).
The media session owns speech, images, plates, video and faces; the interface session owns NexusAI-ui. Do not edit their files.
First work, in this order: W1-1 Ask Phase 2 (files beside records answers, headline unchanged), W1-2 reference cases Meridian and Control (X0), W1-3 the registry (X1, migration 040).
Ask me for: the push, the disk for docker save, the move of the 60 imgsearch test photographs.
```

### Prompt for the media session

```
Continue the media track of NexusAI in C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork. Read NEXUSAI_CONTINUATION.md, docs/roadmap/master-roadmap-20261124/03_media_first_programme.md (the order of work, MF-0 to MF-7),
01_roadmap_flow_architecture.md (sections 5.7 to 5.11) and docs/roadmap/media-modules-reconciled-plan-20261120.md. Build every image from the repository HEAD plus your own committed files
and compare it with the live one (docs/roadmap/worker-image-reproduction-20261124.md). Order (owner, 2026-11-25: media first, each best of the best): MF-0 foundations (worker rebuild rehearsal, reading contract, moments view),
MF-1 video overview and clip export, MF-2 image EXIF, Content Credentials, scale, MF-3 audio product (speaker turns, spoken numbers, call linking; Punjabi and Saraiki bake-off in the background),
MF-4 face governance (migration 042) then within-case grouping, MF-5 Urdu OCR bake-off, MF-6 video detection and tracking, MF-7 plates. Every step: pre-registration first (the gates in the programme are proposals until you write them),
a switch, a fresh bank, my word to deploy. Close the licence debts in section 10 of the programme (read each LICENSE file) before anything ships.
Commit the watch-list Go files only with my yes for the --no-verify commit.
```

### Prompt for the interface session

```
Continue the new interface in C:\Users\sheik\Workspace\Office\Projects\NexusAI-ui (branch ui/v2-redesign, served on :4182; :4181 keeps the old design and nothing merges into lane-arms without my word).
Read docs/ux/v2/DESIGN_REFERENCE_20261105.md and docs/roadmap/master-roadmap-20261124/01_roadmap_flow_architecture.md (sections 4, 5.13 and step W1-14).
One page at a time, fresh research per page, fewer words, distinct visuals, real data, backend gaps shown as rows; run apps/investigation-workspace/scripts/ui-layout-check.mjs before calling a layout done;
I look at each page before the next starts. Next: the "also appears in" chip once the registry exists, the files of a records answer (Ask Phase 2), then connections and places.
```

## What to do first, in your own words

1. Say yes or no to the three Wave 0 items: **push** the unpushed commits, **choose a disk** for `docker save` of the live images (the only copy today), **move the 60 test photographs** out of the reference case.
2. ~~Approve Wave 1~~ **Done 2026-11-25: Wave 1 approved, media first** (`03_media_first_programme.md`). Still owed: the decisions and clicks in its section 11.
3. Paste the prompts above into a new main chat and into the two other sessions.

## Honest limits of this pack

The research was done on 2026-11-24: seven tools were read at their own pages; others are marked secondary and must be verified before they are adopted. Productivity features for Wave 2 come from the interface programme and from your use of Wave 1, not from a new survey. All estimates are working days at this laptop's pace, not commitments. The question catalogue covers the Orion reference case; questions about families that are not built (messages, email, extractions, towers, faces, cross-case) are listed as "cannot answer yet" and should be declined honestly today.
