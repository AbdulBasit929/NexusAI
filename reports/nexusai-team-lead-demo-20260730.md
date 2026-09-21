# NexusAI Team-Lead Demonstration

> Superseded by the current deployed-runtime brief:
> `reports/nexusai-team-lead-phase4-demo-20260730.md`.

Date: 2026-07-30  
Audience: team lead and technical reviewers  
Recommended duration: 12–15 minutes

Runtime audit at 2026-07-29: `nexusai-api` is running and healthy on port 8080,
but PostgreSQL, NATS, forensic API and forensic worker are stopped. All four
stopped together with exit code 255 at the same Docker-engine timestamp and were
not OOM-killed, consistent with an engine/host stop rather than four independent
application crashes. Therefore use the offline demo unless the controlled
activation/rebuild smoke is completed before the meeting.

## Opening statement

> In the last development cycle I converted the LocalAI Knowledge Base into the
> foundation of a governed forensic-intelligence platform. It can register all
> declared evidence families safely, process supported structured evidence with
> deterministic adapters, preserve hashes and source locators, combine exact
> database results with cited Knowledge Base context, and refuse unsupported
> analysis instead of allowing the language model to invent it. Phase 3 custody
> and queue controls and Phase 4 structured depth are source-tested; the retained
> environment is still behind the separately controlled migration/rebuild gate.

## Before the meeting

1. Keep the supplied CDR at
   `C:\Users\sheik\Downloads\923461678183.csv`.
2. Open PowerShell in
   `C:\Users\sheik\Workspace\Office\Projects\NexusAI`.
3. Do not upload the real CDR during the demonstration. Use the privacy-safe
   offline audit, which emits aggregate/schema facts only.
4. Close unnecessary applications because this laptop has limited free RAM.
5. If the approved rebuild has not happened, describe the UI as a tested source
   preview and use the offline demonstration. Do not claim the new UI or Phase
   3/4 code is deployed.

## One-command safe demonstration

```powershell
& "C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe" `
  -NoProfile `
  -ExecutionPolicy Bypass `
  -File ".\scripts\demo_phase4_structured.ps1" `
  -CdrPath "C:\Users\sheik\Downloads\923461678183.csv"
```

This command is read-only. It neither uploads evidence nor changes services or
the database. Expected real-CDR result:

| Check | Result |
| --- | ---: |
| Format / adapter | `.csv` / `cdr` |
| Source rows | 3,931 |
| Accepted / rejected | 3,931 / 0 |
| Exact duplicate rows | 297 |
| Unique normalized rows | 3,634 |
| Overflow rows | 0 |
| Encoding / delimiter | UTF-8-compatible / comma |
| Declared source timezone | Asia/Karachi |
| Raw rows or identifiers emitted | No |

The same command then verifies CDR, IPDR, ANPR, transactions, subscriber,
tower/location, and access/security fixtures with exact accepted, rejected, and
duplicate accounting.

## Demonstration flow

### 1. Explain the architecture — 2 minutes

```text
User/KB upload
  -> authenticated evidence registration
  -> immutable source hash + evidence/version lineage
  -> capability-aware routing
       -> structured adapter + PostgreSQL exact analytics
       -> Knowledge Base retrieval for documents/context
       -> pending/manual review for unaccepted operations
  -> deterministic or bounded hybrid answer
  -> citations, limitations, audit history and reprocessing generation
```

Say: “Exact counts, durations, money, identifiers, timelines, duplicates and
correlations come from adapters and SQL. The model is allowed to explain bounded
returned evidence, not manufacture exact forensic facts.”

### 2. Demonstrate the supplied CSV — 3 minutes

Run the command above and point to:

- automatic CDR adapter selection from its 16 headers;
- complete 3,931-row accounting;
- 297 exact duplicates retained as a visible quality fact;
- Pakistan timezone conversion to canonical UTC while preserving the original
  source text;
- source SHA-256 and zero raw-row/identifier output from the audit.

Explain that CSV is a first-class production path. XLSX support was added for
multi-sheet workbooks; it did not replace CSV.

### 3. Demonstrate structured-family depth — 3 minutes

Use the acceptance matrix printed by the command:

- CDR: calls/SMS/GPRS/VoLTE, duration, direction, devices, cells and location;
- IPDR: IPv4/IPv6, NAT, ports, protocol, exact bytes, session duration, domain,
  subscriber and timezone;
- ANPR: plate/camera/time/location, confidence, script and crop lineage;
- financial: exact decimal/minor units, currency separation, reversal and IBAN
  review;
- subscriber: phone/identity/device validity and conflict flags;
- tower/location: coordinates, datum, sector, azimuth, uncertainty and history;
- access/security: IP/port/user/action/outcome/status and timestamp validation;
- generic/TSV/XLSX: source-preserving schema discovery and per-sheet mapping.

Rejects are expected test evidence, not failures: malformed rows are counted,
reason-coded, and retain source-row provenance for review.

### 4. Show analyst functionality — 2 minutes

If the new UI has been rebuilt and the activation smoke passed, show:

1. **Ask and analyze** — collection, natural-language query and Analyze.
2. **Advanced options** — deterministic template/model/limit only when needed.
3. **Review evidence** — capability map, evidence status and reprocessing lineage.
4. **Manage data** — upload/batch/reject and operational controls.

Suggested synthetic queries:

- `shortest call duration of 923461678183`
- `show frequent contacts for 923461678183`
- `show records from this source file with row citations`
- `correlate 923461678183 across record families`
- `where was plate ABC-123 seen?`
- `show failed access events`
- `show transaction totals by currency`
- `what evidence or adapter is missing for this question?`

If not rebuilt, show the source-build screenshot/static preview only and say so.

### 5. Close with truthful status — 2 minutes

Completed in source:

- universal evidence registration and 20-family capability contract;
- Pakistan-oriented structured adapters and exact goldens;
- CSV/TSV/JSON/JSONL/Parquet routing and bounded read-only XLSX;
- per-sheet XLSX typed mapping with generic preservation;
- deterministic queries and exact cross-family cited correlation;
- privacy-safe provider onboarding audit;
- Phase 3 authenticated scope, custody, retention and durable queue contracts;
- first analyst-first UI simplification.

Not yet claimed live:

- migrations 008/009 and Phase 3 runtime configuration;
- rebuilt API/worker/LocalAI UI/proxy containing all current source;
- OCR, STT, TTS, deep document/video/capture/database/archive adapters;
- new model promotion;
- production object-lock/legal-retention infrastructure;
- GitHub publication.

## Full source verification option

Use only if time and laptop memory allow:

```powershell
& "C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe" `
  -NoProfile `
  -ExecutionPolicy Bypass `
  -File ".\scripts\demo_phase4_structured.ps1" `
  -CdrPath "C:\Users\sheik\Downloads\923461678183.csv" `
  -FullVerification
```

This also runs Python Phase 4 regression, the complete forensic Go package, and
the React production build. It does not migrate, deploy, or upload evidence.

## Rebuild and live-demo gate

The live demonstration becomes valid only after this order passes:

1. higher-memory/CI LocalAI wrapper validation;
2. read-only retained-database and migration preflight;
3. fresh hashed backup;
4. separately approved migrations 008/009 and runtime role/auth/NATS settings;
5. bounded API/worker/LocalAI rebuild with visible logs and rollback tags;
6. health, retry, DLQ, redelivery, tenant, reprocessing, mixed-XLSX, query and UI
   smoke using synthetic evidence.

Do not combine “source complete” with “deployed” in the team update.

## Likely questions and short answers

**Why not use AI for all calculations?**  
Forensic counts, money, duration, identity matching and timelines must be exact
and reproducible. Models explain cited facts; they do not replace exact code/SQL.

**Does CSV work?**  
Yes. The supplied CSV auto-routes to CDR and all 3,931 rows pass production
normalization with complete duplicate and source accounting.

**Why keep duplicates?**  
The source is evidence. We preserve it, identify exact duplicates, and prevent
them from silently inflating unique-result claims.

**Is every registered format fully analyzed?**  
No. Registered, inventoried, parsed, and model-analyzed are different states.
Unsupported derived processing remains pending/manual until its adapter and
acceptance gates pass.

**Why has the full rebuild not happened?**  
The current source depends on controlled Phase 3 migrations/runtime settings,
and the previous LocalAI linker run exceeded the laptop’s bounded resource
window. Backup, migration, build, smoke and rollback must happen in the safe
order instead of risking the retained environment before the demonstration.
