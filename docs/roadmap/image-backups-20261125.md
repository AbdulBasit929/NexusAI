# Saved copies of the live images (2026-11-25)

Programme step MF-0, "docker save the live images". Until today the only copies of the images the live system runs were the local Docker tags. Now each has a file outside Docker. **The files are not in the repository** (they are 1.2 GB and are build products); this note holds the proof of what they are.

Owner's word: "Yes, run it" (chat, 2026-11-25, answering the question "docker save, ~1 GB rebuild download, OFIQ model download?"). The save only **read** images and **wrote new files**; nothing was loaded, tagged, stopped or removed.

## What was saved

Folder: `C:\NexusAI-Backups\images` (same drive as the other backups; for a copy that survives the laptop, copy this folder to external storage: owner's call).

| File | Size (bytes) | SHA-256 | Tags inside (image id) |
|---|---|---|---|
| `worker-image-search-20261123f.tar` | 869,998,592 | `a17f5a8cbcbd2c559c38fbff3cdf64bf8a0dc81440f5eb9fcce9cc049455bedb` | `nexusai-forensic-records-worker:image-search-20261123f` (`40f9bbfbd3cd`, the live worker), `...:image-search-svc-20261123f` (`3eef9ecbb83f`, the worker plus the search service layers) |
| `api-known-sets-20261124.tar` | 313,733,120 | `3c38a2a9ce88c19e5e1bc47dec7dc62d83e65c9d4f52a26cc8388661509ccfcb` | `nexusai-forensic-records-api:known-sets-20261124` (`9e3fca1e37a0`, the live API), `...:text-search-all-20261124` (`3b20b23d9043`, the build it replaced = the API rollback) |

The same two lines are in `C:\NexusAI-Backups\images\SHA256SUMS.txt`.

## What was checked

* `sha256sum -c SHA256SUMS.txt`: both files OK.
* Each tar's `manifest.json` lists the two tags above; every layer and config blob it names is present in the tar: worker 151 of 151, API 77 of 77, none missing (checked by reading the tar, `scratchpad/check_tars.py`).
* The index digests inside the tars equal the image ids Docker shows for the four tags.
* **Not done, on purpose:** `docker load`. Loading would re-create tags that are live; the restore below is for a recovery machine or after the live tag is gone.

## What is not covered (so nobody assumes it is)

* The worker file is the live image only (`image-search-20261123f`). Its rollback, `before-image-search-20261123f` (`b7527d3371a3`), is **not** saved here; the earlier overlay chain is described in `worker-image-reproduction-20261124.md`. Saving it is one more command if wanted.
* The model container image (LocalAI) and the database image are not saved: they are public images that can be pulled, and the model files live in volumes (not touched here).
* The database and the evidence files are backed up by the migration backup scripts (`tools/dev/live-window/1-backup.sh`), not by this.

## Restore (not run)

```powershell
# on a recovery machine, or after the live tag is gone:
docker load -i C:\NexusAI-Backups\images\worker-image-search-20261123f.tar
docker load -i C:\NexusAI-Backups\images\api-known-sets-20261124.tar
docker images --format '{{.Repository}}:{{.Tag}} {{.ID}}' | Select-String 'image-search-20261123f|known-sets-20261124'
```

Check the ids against the table before starting anything. To keep a loaded copy beside a running tag, retag first; do not load over a tag that a container uses.
