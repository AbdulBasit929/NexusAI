# Migration 034: conversations and the case layer (DRAFT, never applied)

Design: [`docs/architecture/CONVERSATIONS_AND_CASE_LAYER_20261103.md`](../../../../docs/architecture/CONVERSATIONS_AND_CASE_LAYER_20261103.md). Contract for the interface: [`docs/work/API_CONTRACT_CONVERSATIONS_20261103.md`](../../../../docs/work/API_CONTRACT_CONVERSATIONS_20261103.md).

| File | What it does | Changes data |
|---|---|---|
| `../../034_conversations_and_case_layer.sql` | 15 tables, 8 functions, 5 triggers, row-level security, grants. Additive and idempotent. Includes conversations across two to ten cases (nullable `collection_id`, `group_id` on a turn, a turn-scope trigger, policies that apply the case rule to every case of a thread) | adds tables only; no existing row is touched |
| `preflight.sql` | read-only gate: schema, evidence registry, append-only function, `pg_trgm`, encoding, no table of the same name with another shape | no |
| `verify.sql` | read-only check of tables, constraints, every policy, functions, triggers, search columns and indexes, grants of the application role, and that the engine's read-only role sees nothing | no |
| `behaviour.sql` | 11 steps as the application role in one transaction that is rolled back: private/shared/selected threads, immutable turns, tenant isolation, fail-closed, members and roles, the audit chain (including a tampered row being found), search folding, and conversations across cases (the shape of the list of cases, a turn only in a case of its conversation, a conversation seen only by someone who may open every one of its cases, an auditor of every case and not of some) | no (ROLLBACK) |
| `rollback.sql` | drops the 15 tables and 8 functions; refuses unless `app.caselayer_allow_destructive_rollback = 'true'`; prints the rows it drops | **drops the case layer** |

## Rehearsal (throwaway database, never the live one)

```bash
bash tools/dev/live-window/rehearse-034.sh                      # a fresh database built from the live migrations; KEEP=1 keeps it for the Go specs
SOURCE=dump DUMP=/path/to/live.dump bash tools/dev/live-window/rehearse-034.sh   # a restored copy of a live dump (about 9 minutes)
```

HEAVY: take `C:\tmp\nexusai-locks\heavy.lock` and check 3 GB of free RAM first.

## Applying it to the live database (needs the owner's chat word; this is not done)

1. Backup first (`tools/dev/live-window/1-backup.sh`), and the rehearsal on a restored copy of that dump.
2. `preflight.sql`, then `034_conversations_and_case_layer.sql`, then `verify.sql`, then `behaviour.sql` (it rolls back), then `verify.sql` again. From PowerShell the files are pure ASCII, so piping is safe:

```powershell
foreach ($f in 'ops/034_conversations/preflight.sql','034_conversations_and_case_layer.sql','ops/034_conversations/verify.sql','ops/034_conversations/behaviour.sql','ops/034_conversations/verify.sql') {
  Get-Content -Raw -Encoding ascii "db/forensic_records/$f" | docker exec -i nexusai-forensic-postgres-1 psql -v ON_ERROR_STOP=1 -U localrecall -d localrecall -f -
}
```

3. On a **fresh** database the role `forensic_runtime` does not exist while the numbered files run, so its grants are skipped (as for 021 and 031). After `runtime_role.grants.sql`, run `034_conversations_and_case_layer.sql` once more: it is idempotent and grants then.
4. Deploy the API image that registers the routes only after the migration is applied and verified; the routes answer `503` with a sentence while the tables are missing. Restart the workspace at once after the recreate (`tools/dev/Restart-Workspace.ps1`).
5. Rollback: the image first (`before-case-layer-...` tag), then `SET app.caselayer_allow_destructive_rollback = 'true';` and `rollback.sql`, only after exporting the 15 tables.
