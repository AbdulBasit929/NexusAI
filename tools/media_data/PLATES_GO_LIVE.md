# Plates: going live (runbook; EXECUTED 2026-11-10, see Amendment 7 of the plates pre-registration)

State after the switch-on of 2026-11-10: the live worker runs `nexusai-forensic-records-worker:latest` = `plates-20261110d` (`e9915ad9b98f`) with the plate role on and engine `nexus`; the image that ran before is `before-plates-20261110d` (`47e5e829a3d4`). Before the switch-on the role was off (`FORENSIC_ANPR_ENABLED=false`). The engine stays behind `FORENSIC_ANPR_ENGINE` (code default `fastalpr`).
Evidence for the engine: `docs/data/plates-m5-preregistration-20261110.md` (Amendments 1 to 6). It is a **flagged, provisional** option: the detector's training data is not stated, the reader weights have no separate licence statement, and the pre-registered exact-read gate (17 of 22) was missed by one image (16 of 22; the current models 13 of 22).

## Before you start (check, do not skip)
1. `docker ps`: Postgres and NATS healthy, the worker up. Free memory above 3 GB (`wmic OS get FreePhysicalMemory`).
2. The overlay image exists: `docker images nexusai-forensic-records-worker` shows `plates-20261110d` (built on top of `latest`; `latest` is unchanged). If not: `.\tools\media_data\Build-PlatesOverlay.ps1 -Tag plates-20261110d`.
3. The three model files are in `models/media/nexus-plates/` (the worker mounts `models/media` at `/models/media`); `PROVENANCE.json` there holds their SHA-256 values.
4. Staging proof passes with the overlay's own code (engine `nexus`, a few real photos: readiness READY, one observation per plate; engine unset: the current processor).

## Switch on (done 2026-11-10; the one-command form is tools/dev/live-window/5-recreate-worker-plates.ps1 [-Apply|-Rollback], dry run by default)
```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork
# the script does this for you: it tags the running image as before-plates-20261110d, retags plates-20261110d as latest and recreates only the worker
.	ools\dev\live-window-recreate-worker-plates.ps1            # dry run
.	ools\dev\live-window-recreate-worker-plates.ps1 -Apply     # applied 2026-11-10
# set in the worker's environment (compose or the helper you use to recreate it): FORENSIC_ANPR_ENABLED=true  FORENSIC_ANPR_ENGINE=nexus  FORENSIC_ANPR_ONNX_PROVIDERS=CPUExecutionProvider
# recreate ONLY the worker (the command is in docker-compose.forensic-records.media.yaml)
```
Then: `docker logs nexusai-forensic-records-worker-1 | tail`; upload one synthetic image with a plate to a throwaway collection and confirm the observation says `processor: nexus-plates-onnx-cpu`, carries `plate_format`, `plate_format_flags` and `model_provenance`, and is `manual_review_required`.

## Roll back
```powershell
docker tag nexusai-forensic-records-worker:before-plates-20261110d nexusai-forensic-records-worker:latest   # done by the script's -Rollback
.	ools\dev\live-window-recreate-worker-plates.ps1 -Rollback   # retags the rollback image and recreates the worker without the plates file: the role is off again
```
Observations already written keep their `processor` and `detector_version`; nothing is overwritten, and a re-read with the other engine produces a new version beside them.

## What an analyst will see (state it in the hand-over)
One candidate per plate-like region, each "model observation, review required"; a read that fits no registered format is flagged `format_unknown` and shown as read, never edited. Plates under about 40 px wide are found less often (9 of 35 in the Open Images sealed photos for the new detector, 1 of 35 for the current one). Accuracy on real Pakistani plates: 16 of 22 exact on the owner-labelled sealed set (small sample; 52 to 87% interval). Not certified for field use.
