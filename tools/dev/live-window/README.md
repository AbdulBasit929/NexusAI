# The live window for the new families (2026-10-30)

Scripts for applying migrations 027 to 031, the engine and the worker overlay to the LIVE stack, in the order of `docs/data/migrations-027-031-runbook.md` section 7. Everything was rehearsed first: the
migrations on a restored copy of the live database, the worker image end to end on throwaway databases (`tools/case_generator/check_worker_overlay.py`), the engine on development banks and the red team,
and every `-Apply` script has a dry run that changes nothing.

| Step | Script | What it changes | Rollback |
|---|---|---|---|
| 1 | `bash tools/dev/live-window/1-backup.sh` | nothing (baseline and a fresh `pg_dump`) | none needed |
| 2+3 | `bash tools/dev/live-window/2-migrations.sh` | the live database: 407 catalog lines added, 3 changed, the 4 layouts recorded | `ops/<migration>/rollback.sql` in reverse order (runbook section 4), or restore the dump |
| 4 | `.\tools\dev\live-window\3-recreate-api.ps1 -Apply` (dry run first, without `-Apply`) | the API container, image `families-20261030` | `-Rollback` |
| 5 | `.\tools\dev\live-window\4-recreate-worker.ps1 -Apply` (dry run first) | the worker container, image `families-20261030`, two new switches | `-Rollback` |
| 6 | `.\tools\dev\Restart-Workspace.ps1 -Port 4181 -Tree lane` | the interface dev proxy (it keeps a dead connection to the old API container) | none needed |
| 7 | the live smoke (runbook section 7, step 7) | adds new synthetic collections | the collections stay; they are the demonstration |

Steps 2+3 and 4 follow each other within minutes: the engine that runs today reads the analyst views from the database and would list the 13 new, empty views.
Images are built beside the running ones and tagged (`before-families-20261030` is the rollback of each).
