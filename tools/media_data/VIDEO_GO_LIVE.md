# Plates in video: going live (runbook; EXECUTED 2026-11-11 on the owner's word "Go live")

**State 2026-11-11 16:38 to 16:45 PKT:** the API image `video-search-20261111` (dce27afe9556; rollback `plate-search-20261110c` = dcf5ad448b5c) and the video worker overlay (`latest` = 4dcc14ec5901; rollback `before-video-v4-20261111` = e2b930e29041) are live; both workspaces restarted. Live check on a throwaway collection `video-plates-live-test-20261111` (a 10 s synthetic clip with one known plate): the job completed, 87 frame reads (`timestamp_seconds` 0 to 9.96), one plate group (RLX902, 86 sightings, 0 to 9.96 s, engines nexus-plates-onnx-cpu and fastalpr-onnx-cpu), the plate question named the video and its moments (0:00 to 0:09), an unknown plate was declined with the "reader may have misread" wording, a one-character-different plate (O for 0) listed the video as a similar reading; the UI on :4182 opened the video at the cited moment with the box. The synthetic gates VG1, VG2 and VG5 stay NOT MET (see the pre-registration); the owner switched it on knowing that. The steps below are kept as the record and as the recipe for a rollback or a re-apply.

Plan and evidence: `docs/data/video-plates-research-and-preregistration-20261111.md` (research, gates, the sealed result, the real-footage benchmark, Amendments 1 to 5).
**Not certified.** The frozen V4 missed three of the five pre-registered gates on the synthetic sealed set (75% of clips read exactly against 85%; 0.46 stray groups per clip against 0.3; 4 points below today's path against a required gain of 10) and met two (time sanity, cost). On real clips, checked crop by crop, it read 9 of 9, 7 of 7 and 13 of 15 plates exactly where today's path confirms 0 to 4. The configuration in the image (group cap 1024, support floor 0.6, tiles on 4K frames) comes after the sealed run and is not covered by it.

## What changes when the switch is on
Today a video is handled by the worker's older path: up to 12 frames, one every 5 s, each read as an image. With V4 the worker reads the video at 4 frames per second with two engines (the M5 plate engine on every second frame, the licence-clean default engine on every frame), reads extra frames around each plate, groups reads of one plate (one group per plate seen, never an identity or a journey), and stores every frame read (`anpr_ocr_observation`, with `timestamp_seconds`, `frame_number`, bounding box) and every group (`video_anpr_plate_group_observation`, first and last seen). The plate search then answers "which photos or videos show this plate" with the video, the moments and an openable citation. Frame reads of images are unchanged.

## Cost to expect
About 4 to 6 times the video's length for 1080p, 10 to 13 times for 4K without tiles (this laptop's CPU), more when tiles are used on 4K. A job has a lease; the video overlay raises it to 2400 s and V4 stops by itself at a wall-clock budget of 1800 s (coverage receipt `resource_stopped`, the reads made so far are kept and the answer says the video was only partly read). A video longer than 900 s is refused by V4's bound. A GPU would remove most of the cost (the EliteDesk RTX 2080 is the candidate).

## Before you start (check, do not skip)
1. `docker ps`: Postgres and NATS healthy, the worker up and idle (the script refuses if jobs are in flight). Free memory above 3 GB (`wmic OS get FreePhysicalMemory`).
2. The overlay image exists: `docker images nexusai-forensic-records-worker` shows `video-v4-20261111` (built on `latest`; `latest` is unchanged). If not: `.\tools\media_data\Build-VideoV4Overlay.ps1 -Tag video-v4-20261111` (its smoke test imports the pipeline and V4).
3. Offline proof of the overlay image on a video (switch on and off): `docker run --rm --network none --entrypoint python -v <models>:/models/media:ro -v <clips>:/clips <overlay image> /clips/staging_proof.py <clip> true` (and `false`): switch on gives the frame reads, a group with its time range and `COMPLETE_RESULTS`; switch off gives the earlier behaviour. Done 2026-11-11 on a synthetic clip.
4. The API image with the video-aware plate search is deployed (the answer names the moments): `tools/dev/live-window/3-recreate-api.ps1 -Apply -To <tag> -RollbackTo <previous tag>`, then `Restart-Workspace.ps1` for 4181 and 4182.

## Switch on (one command; dry run by default)
```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork
.\tools\dev\live-window\6-recreate-worker-video.ps1            # dry run: the compose hash of the five running files must be reproduced
.\tools\dev\live-window\6-recreate-worker-video.ps1 -Apply     # tags the running :latest as :before-video-v4-20261111, tags the overlay as :latest, recreates only the worker with docker-compose.forensic-records.video.yaml
```
Then `docker logs nexusai-forensic-records-worker-1 | tail` and upload one video (a short synthetic clip) to a throwaway collection: the evidence must reach `READY`, show `anpr_ocr_observation` rows with `timestamp_seconds`, and one `video_anpr_plate_group_observation`.

## Roll back
```powershell
.\tools\dev\live-window\6-recreate-worker-video.ps1 -Rollback   # tags :before-video-v4-20261111 as :latest and recreates the worker without the video file: V4 is off again
```
Observations already written keep their processor identity; nothing is overwritten.

## What an analyst will see (say it in the hand-over)
Each plate is a "model observation, review required" with the moments it was seen; a group is "bounded adjacent-frame readings, not tracking". A similar reading is listed apart and labelled "check by eye". Plates smaller than about 40 px in a frame are found less often; blur, glare and night lower the rate; a plate with a letter that looks like a digit (O and 0, T and I, 0 and 8) is often read as the digit and is then found as a "similar reading". Not certified for field use.
