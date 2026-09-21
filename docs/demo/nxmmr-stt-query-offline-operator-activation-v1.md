# NX-MMR STT and governed-query offline operator activation v1

Status: source prepared; live activation not performed.

This bundle activates one reviewed source delta across exactly three services:
`api`, `forensic-records-api`, and `forensic-records-worker`. It preserves the
running PostgreSQL and NATS containers, every named volume, existing LocalAI
models, the forensic spool, ANPR, Paddle OCR, and Video ANPR V3. It enables the
already installed `faster-whisper-small-ur` alias, the existing YuNet/SFace
candidate-only role, and the existing pinned SigLIP revision. It does not enable
TTS, Video ANPR V2, unrestricted SQL, model downloads, bulk reprocessing, or
product certification.

## Operator boundary

Close memory-heavy applications manually. From the repository root, run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\test_nxmmr_stt_query_operator_bundle.ps1 -LiveReadOnly
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_nxmmr_stt_query_activation.ps1
```

The runner repeats every gate and requires the exact confirmation `YES`. It
requires at least 6 GiB available RAM, 20 GiB workspace free disk, zero active
forensic jobs, healthy services, exact source and SigLIP hashes, both existing
LocalAI model aliases, and a renderable exact rollback snapshot. A failure
before recreation leaves the three live services untouched. It may leave newly
copied, read-only SigLIP files inactive; no retained evidence is changed.

The build is sequential and may fetch ordinary build dependencies. If a build
fills Docker Desktop's clean Linux page cache, the bundle first verifies both
`Dirty` and `Writeback` are exactly zero, drops only clean page cache with
mode `1`, and waits up to 30 minutes for the unchanged 6 GiB floor. It never
runs `sync` or mode `3`. Every completed candidate image is recorded in a
source-sealed build-set receipt, so a later RAM failure can resume verified
images instead of rebuilding them. Runtime
model acquisition is disabled with `HF_HUB_OFFLINE=1` and
`TRANSFORMERS_OFFLINE=1`; this bundle copies only the seven already present,
hash-pinned SigLIP files. It never runs `compose down`, `--remove-orphans`,
prune, volume deletion, or evidence cleanup.

## Verification and rollback

Activation writes ACL-restricted receipts under
`local-acceptance-models/nxmmr/private-activation-stt-query/`. Before mutation,
the receipt contains immutable image IDs and the exact environment, mounts,
ports, commands, healthchecks, network, and volume references for all three
changed services. Verification requires healthy stable containers, zero
restarts, exact candidate images, exact worker roles, the two existing LocalAI
models, exact SigLIP assets, zero active jobs, and unchanged PostgreSQL/NATS
identity, image, and restart count.

If verification fails after recreation, automatic rollback is attempted. A
manual retry is:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\rollback_nxmmr_stt_query_activation.ps1
```

Rollback recreates only the same three services from their immutable original
image IDs and snapshotted configuration. Copied SigLIP files remain inert and
recoverable; no volume or evidence is deleted.

Live acceptance after a successful activation is a separate phase. Use fresh,
bounded evidence copies and prove the 16-item critical query matrix before
claiming demo readiness.
