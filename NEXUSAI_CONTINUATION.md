# NexusAI — Current State

> **CURRENT STATE ONLY. Rewrite it; never append. Hard cap 300 lines.** History in
> `reports/archive/` and `docs/work/HANDOFF_LOCAL_20261007.md` (live log). **TWO AGENTS WRITE THIS FILE:**
> everything above `## CODEX_UI_TRACK` is the backend track, that section is the interface session's;
> neither edits the other's. **What to do next: [`docs/roadmap/reconciled-plan-20261124.md`](docs/roadmap/reconciled-plan-20261124.md)** (per-family status, what the older plans got stale, the cross-case track X0 to X3, the media order, the owner's decisions). Behind it: the pack [`docs/roadmap/master-roadmap-20261103/`](docs/roadmap/master-roadmap-20261103/00_START_HERE.md) (media programme, architecture, interface programme, data and tests, decisions D34 to D43) and the ledger of older items [`docs/roadmap/master-plan-20261021.md`](docs/roadmap/master-plan-20261021.md). Where they differ, the reconciled plan wins (migration numbers, order of phases).

**Last updated:** 2026-11-24 · **Lane tree:** `lane-arms` (worktree `NexusAI-localwork`), pushed to `origin claude/gifted-pasteur-4kpx0r` (tip `cef8221`) · **Interface tree:** `ui/v2-redesign` (worktree `NexusAI-ui`, tip `a10ce43`), pushed 2026-11-24 as a new remote branch.

---

## 1. Where the product is (measured)

    STRUCTURED + LINK ANALYSIS (strong)   the Ask engine v2 answers call, internet, app, device, cell, bank, plate, subscriber, document and connection
        questions over 41 analyst views on typed cases; verified bank (Orion Lahore, SQL oracles, 565 questions, rebuilt from the live database 2026-11-24): 562 correct, 0 wrong, 3 abstained (VB-MESS-019, VB-VIDE-031, VB-VIDE-037), every verdict equal to the 2026-11-13 sheet (558 answers carried, 53 re-asked live: `docs/qa/verified-bank-20261124-sheet.md`);
        a 64-question sample re-asked 2026-11-23 on the live code: 62 correct (the two differences are newer data in the staging copy, proved by a switch-off control); p50 ~30 s on the CPU laptop.
    TEXT SEARCH (new, live 2026-11-24)   a search for a number, name, plate or word reads the documents, image text and transcripts together, matches as typed, written differently
        (digit scripts, separators, 0/92/0092 phone forms, case) or as a labelled close reading, and lists EVERY file; an empty records answer gets the files that hold its number or name.
        Fresh banks: committed code 69% fully answered (bank-C), fixed build 70 to 86% (A, B, C); live engine before: 20%. Gates (90% / 0.95) NOT met: the 4B intent model sends about
        a third of "where is X written" questions to the table engines. Record: docs/data/ask-exhaustive-search-results-20261119.md.
    CASE LAYER (live since 2026-11-10)   threads and turns with re-run and diff, notes, findings, members, authority, a hash-chained audit, export (migration 034, internal/caselayer).
        CROSS-CASE IS ONLY A FAN-OUT: a thread puts a question to each of 2 to 10 cases in turn; nothing joins one case to another (no registry, no overlaps, no cross-case findings).
    MEDIA   OCR (EasyOCR, Latin and digits good, Urdu Nastaliq 70% character error); speech (Omnilingual routes with automatic language detection; Urdu, English, Pashto read; Punjabi
        and Saraiki not measured); plates (images: sealed gate 83% against 90%, not met; video V4 live); IMAGE SEARCH (copies, parts of pictures, window vectors, duplicate groups, word
        search, draw-a-box; gates N1 to N3 not met as registered, the owner is testing); video overview, faces: not built / OFF (face bake-off done, no governance code yet).
    INTERFACE   :4182 = the new design (branch ui/v2-redesign: Ask with conversations, dashboard, evidence gallery, media viewers, image search); :4181 = the OLD design with all functions
        (owner rule 2026-10-28); the cut-over is not decided; nothing merges into lane-arms without the owner's separate word.
    ENTERPRISE (open)   member and authority tables exist (034, mostly empty); one shared actor, no identity, no MFA; read-only role and row-level security are live.

**Time basis:** every typed row is `declared`, `explicit_offset` or `confirmed_by_analyst`; none is assumed. **Collections:** 25 live, the reference case is `orion-lahore-reference-20261030` (165 evidence items, 60 of them image-search test photographs named `imgsearch-*`).

## 2. Lessons — the lists that bind. Reasoning: [`reports/LESSONS.md`](reports/LESSONS.md)

**Every capability arrives with the check that bounds it, behind its own switch, measurable and withdrawable.**
Verification proves a plan against ITSELF, never against the question · an invented filter is a symptom and **a missing one has none** · fix the instrument first · a probe must reproduce the PATH ·
**write a threshold against what a change CAUSES** · **a switch must be DECLARED in compose and in the startup registry** and asserted from the container before scoring · **a timeout is never a verdict** ·
re-derive every expectation when a definition changes · **gates are written before a run; a fix is measured on another fresh seed; judgement and sealed cases are never tuned on; a bank whose misses
were read cannot certify the fix** · the oracle's and the judge's own defects are reported · **thick views, thin SQL** (a 4B model on a CPU writes ~5.5 tokens a second) · run the typing report BEFORE
any question after a data change · no spread after each fix: use the tiers (`tools/dev/README.md`).
**New 2026-11:** the 4B intent model cannot hold "where is X written" apart from "how many records does X have" by wording, an extra field or a second question (4-5 false routes of 50, or a 65% yes
on plain questions): do not change the deployed intent prompt without the three intent banks · **live must equal committed**: a build from a shared, dirty tree ships other sessions' unfinished code and
a build from an old commit drops newer commits; build from HEAD and compare the binary with the live one (function names and size) · a Docker build's `go mod download` hangs on this network: cross-compile
on the host with `GOPROXY=off` · **one heavy job at a time on the laptop** (model run, window indexing, database restore, image build never overlap: the Docker VM is 7.6 GB).

**Rejected after measuring, do not retry:** removing generator-invented filters by any means · switching the ladder off without replacing its scope · agreement of two drafts · a fixed all-in prompt
prefix · withholding uncurated fields · embeddings on structural decisions · a wider intent prompt, a `where_written` field, a second model question for lookups · a bigger intent model (no GPU).

## 3. Open work — what to do next is in the reconciled plan (section 5 there)

**A0 done 2026-11-24** (both sessions' work committed and pushed; the API proven equal to HEAD). **A1 done 2026-11-24:** this file, the verified bank rebuilt and re-asked (above). Still open from A1: the image test pack still sits
inside the reference case (moving it needs the owner's word). **A2:** Ask Phase 2 (run the text search beside every value-bearing message, attach the files in a secondary field; headline unchanged).
**B:** cross-case X0 (reference cases Meridian and Control), X1 (identifier registry, migration 040), the audio product. **C:** X2, X3, the message family, video overview. **D:** face governance and
within-case groups, Urdu OCR bake-off, cut-over preparation. Parked for lack of a GPU: a bigger intent model, captions, fine-tuning, faces at scale.

**Known live defects and limits:** the live WORKER image is the end of a chain of 20+ overlay images on the 2026-08-23 base, with files OLDER than the repository's: 23 of 32 `/app` files equal HEAD, 8 differ in
both directions, so a worker built from HEAD would add unreleased wiring and drop live lines (`docs/roadmap/worker-image-reproduction-20261124.md`, manifest `worker-image-live-manifest-20261124.json`, check with
`python tools/media_data/worker_image_manifest.py compare|check <container>`); **the images exist only as local Docker tags (never exported: a `docker save` to a disk the owner chooses is the backup)** · the 13 provider layouts are drafts (no real-file confirmation) · records answers that have rows never name the files that write the value (Phase 2) ·
Postgres shared memory is the 64 MB default · the connection class takes 39 s on the laptop · speech and plate sealed gates not met · the Lahore photo counts include the 60 test photographs.

## 4. Architecture — decided, do not relitigate

1. **Questions are answered by the Ask engine v2 first** (`FORENSIC_ASK_V2`, `_FIRST`): the model writes ONE SELECT over the analyst views (`analyst.v_*`, 41 live), code validates it (parse-tree
   allowlist, red team), checks obligations, runs it read-only as `forensic_analyst_ro` (cost 5,000,000, row cap 1,000, 15 s) and writes one audit row. A decline goes to the governed lane and then
   the older ladder. Three outcomes only: answered, abstained, declined (`docs/architecture/QUERY_ANSWERING_DECISION_20261005.md`, `docs/data/ask-engine-v2.md`).
   **Before it, in this order:** the image step (`FORENSIC_IMAGE_SEARCH_ASK`: only a sentence that names a photograph), the text-intent step (`FORENSIC_TEXT_INTENT`: a model reads the sentence and, for a
   search for words, the request goes to the any-source text search `FORENSIC_TEXT_SEARCH_ALL`; a plate in images goes to the plate search). An answer with NO rows may get the files that hold its number or name.
2. **Typed tables are the source** for calls, internet, apps, plates, subscribers, bank and wallet, document mentions (`FORENSIC_TYPED_FAMILIES`); layouts are data
   (`ingestion/forensic_records/layout_registry/layouts`).
3. **Deterministic SQL, no model narrates a number.**
4. **Derived artifacts are never merged with ingested records.** A model's plate read is not a camera's sighting; `source_truth_state` travels with every derived citation; media views exist (023, 035 to 037).
5. **One case clock:** `FORENSIC_ANALYSIS_TIMEZONE=Asia/Karachi`; evidence times are stored as UTC instants.
6. **The ladder (`query.go`) is frozen,** still on for what the engine declines. **No new templates; questions are understood at run time by the model, code validates.**
7. **The case is the unit of trust.** Nothing crosses a case boundary except through the cross-case mechanism (reconciled plan section 3; D41: a reader who cannot open the other case learns only that an overlap exists).
8. **UI contract:** [`docs/ux/NEXUSAI_PRODUCT_UX.md`](docs/ux/NEXUSAI_PRODUCT_UX.md) is binding (CASE → EVIDENCE → QUESTION; provenance is the primary object; the analyst never sees SQL or a model).

## 5. Hard rules

- **No new operation templates**; do not extend `query.go`'s keyword ladder. **No embeddings on structural decisions.** **No model roulette:** no evaluating, benchmarking, downloading or swapping a model
  except by a decision rule written BEFORE the run and the owner's yes per item (licence ledger row first). Licence status is recorded; it is not a blocker for development.
- **Abstention is a success.** A confident wrong answer never is. **Never fabricate** a result, citation, OCR text, transcript, sighting, face identity, location, timeline or relationship.
- **Real data is a test input, never a repository file.** Counts and booleans only for the real collection; plates read from real footage never enter the repository; the vault, the policy
  (`docs/data/real-data-handling-policy.md`) and the scanner apply; no evidence, crop, transcript or embedding to any third party.
- **Every behavioural change is measured against a pre-declared threshold** and reverted if it fires. **Every deploy needs the owner's chat word**, with a rollback tag first.
- **Commits:** explicit paths only (never `git add -A`); never stage `tests/e2e-ui/Dockerfile`, `tests/fixtures/backend-image/Dockerfile`, `tools/visual_spike/package-lock.json`; message from a file with
  `git commit -F`, ending `Assisted-by: Claude Code:<model-id> [tools]` and `Claude-Session: <id>`, NEVER `Co-Authored-By` of an AI. **The Windows lint hook fails on `backend/go/*` (purego), unrelated to
  this code: a Go commit needs `--no-verify`, and that needs the owner's yes PER COMMIT** (first run `golangci-lint run --new-from-merge-base=master <changed packages>`, `go vet` natively and with
  `GOOS=linux`, and the focused tests). Docs-only and interface-only commits skip the hook. No force-push; a push needs the owner's word.
- **Never merge upstream LocalAI into this repo.**

## 6. Operational hazards

- **Auth fails closed** (`FORENSIC_RECORDS_API_KEY` from `.env.forensic-runtime.local`, in the MAIN checkout, never read, printed or copied). **Never "fix" a crash-loop by disabling auth.**
- **Recreate the API only with `tools/dev/live-window/3-recreate-api.ps1`** (dry run first; `-Apply -To <tag> -RollbackTo <tag> -Set NAME=value`; it copies the running container's environment, about 83
  variables, so a switch you set stays set; the compose default of `FORENSIC_TEXT_SEARCH_ALL` is still false, so a raw `docker compose up` would switch it off). **The image is a host-built Linux binary laid over
  the previous image** (`GOPROXY=off GOFLAGS=-mod=mod CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -p 2 -trimpath`, NOT stripped, like the live one; `FROM <live image>` + `COPY --chmod=0755`). **Build it from
  HEAD** (a clean `git archive` or a snapshot of the tree), tag a rollback first, compare function names with the live binary (`docker create` + `docker cp`), then apply and **restart the workspaces at once**
  (`tools/dev/Restart-Workspace.ps1 -Port 4181 -Tree lane`, `-Port 4182 -Tree ui`; never pipe it through `Select-Object`). The API image is distroless: use `docker inspect` for its environment.
- **Staging without touching the live stack:** restore a dump into a throwaway container (`SKIP_SPEC=1 KEEP=1 bash tools/dev/live-window/rehearse-evidence.sh <stamp>`, about 7 minutes, 2 GB; the container is
  `--rm`, so NEVER `docker stop` it: the restore is lost) and run `tools/dev/live-window/run-staging-api.ps1 -Stamp <stamp> -WithAuth` on :8191 (without `-WithAuth` the text routes lack a workspace scope).
- **Never** `docker compose down`, `down -v`, `--remove-orphans` (would delete `nexusai-api-1-prev-gbnf`, the LocalAI that serves the language model and speech), `system prune`, `volume prune`. Never
  stop the model container. Never reprocess, backfill, migrate or delete retained evidence without the owner's go. **Git:** no `reset --hard`, `clean`, `checkout --`, `restore`, `stash`, `merge`,
  `rebase`, `pull` unless the owner asked this session.
- **Two sessions share the laptop and the working tree.** The media session builds its images from the live commit plus its own files; the live API now contains the text search too, so ITS builds must
  start from HEAD. Heavy work takes the lock `C:\tmp\nexusai-locks\heavy.lock`. The host watchdog kills runs below 600 MB free; compile with `GOMAXPROCS=2 GOMEMLIMIT=700MiB go ... -p 1`.
- Env from PowerShell, not Git Bash (`MSYS_NO_PATHCONV=1` for `docker exec`); Postgres host port 5433, db `localrecall`; `query.go` and `main.go` are CRLF in the working tree (LF in git): edit them
  with CRLF-aware scripts and check `git diff --stat`; the bash tool halves doubled backslashes (build them with `chr(92)`); an empty JSON-Schema `enum` is an unparseable grammar; Ginkgo needs `-ginkgo.timeout 6h`
  for the live specs; the Docker VM wedges after a laptop sleep (start Docker via `explorer.exe`).

## 7. The live system (read 2026-11-24)

| Part | State |
|---|---|
| LocalAI (language model, embeddings, speech) | container **`nexusai-api-1-prev-gbnf`** (host port 8080, restart policy `no`; **`nexusai-api-1` is a detached duplicate: never start it**), Qwen3-4B-Instruct Q4_K_M on CPU (context 4096, one request at a time, ~22 tok/s reading, ~5.5 writing), Qwen3-Embedding-0.6B, `faster-whisper-turbo` and `-small-ur`; 15.7 GB laptop, no GPU |
| API | image **`text-search-all-20261124` = `3b20b23d9043`** (`:latest`), live since 2026-11-24 ~11:17 PKT on the owner's "Deploy this build" = the media session's `image-search-20261123k` + the Go binary rebuilt from HEAD (commits `ad1afb2`, `729e018`); `FORENSIC_TEXT_SEARCH_ALL=true`. Rollback: `3-recreate-api.ps1 -Apply -To text-search-all-20261124 -RollbackTo text-search-all-20261124 -Set FORENSIC_TEXT_SEARCH_ALL=false` (switch off) or the image **`before-text-search-all-20261124` = `517f31ae9a0f`**. Chain below it: `before-image-search-20261123k` = `0e72aad96d6c`, `before-image-search-20261123j` = `b04139ba4a2a`, `before-image-search-20261123b` = `4fd3b23b0021`, **`audio-files-20261117f` = `22d23221aba5`** (audio engine rounds 10 and 10b), **`case-layer-20261110` = `85eeeaf98c26`**, `before-case-layer-20261110` = `b1514079de69` (round 9e). After ANY recreate restart the workspaces at once |
| Worker | image **`image-search-20261123f` = `40f9bbfbd3cd`** (`:latest`): an overlay on files older than the repository's (OCR with EasyOCR and the Latin re-read, speech routes and automatic language detection, plate and video roles, image-search indexing); recreate it only with the media session's scripts (`tools/dev/live-window/4-` to `10-`), when in-flight jobs are 0 |
| Image-search service | container `nexusai-forensic-image-search-1`, image `image-search-svc-20261123f` (`3eef9ecbb83f`): word, copy, region and crop endpoints; `11-image-search-service.ps1 -Tag ...` recreates only it |
| Switches (API) | **39 on** (the Ask v2 pair, governed SQL pair, `TEXT_INTENT`, `TEXT_SEARCH_ALL`, `IMAGE_SEARCH_ASK`, `IMAGE_REGION_SEARCH`, `PLATE_SEARCH_ONLY`, `MEDIA_FAMILY_ROUTING`, `LADDER_ROUTING`, `VERIFIED_ONLY`, `PLAN_CACHE`, `TEXT_BROWSE_GUARD`, `DERIVED_ARTIFACT_EXECUTION`, the presentation and guard switches) · **13 off** (`ASK_V2_AGREE`, `CONVERSATION_FRONT_DOOR`, `CONVERSATION_ROUTER`, `PII_MASKED_PROJECTION`, `IR_CROSSCHECK`, `IR_SHADOW`, `BREAKDOWN_GOAL`, `GOAL_TAXONOMY_SLICES`, `IDENTIFIER_BINDING`, `FRAME_FAMILY_PRECEDENCE`, `DROP_INVENTED_FILTERS`, `EMPTY_AGGREGATE_HONEST`, `HEADLINE_STATES_FILTERS`) |
| Database | PostgreSQL 16 + TimescaleDB, pgvector, `cube`, `earthdistance`; migration files present for 001 to 021, 023 to 032, 034 to 038 (**022 never applied; 033 never used; 039 is a DRAFT of the media session (image known sets, not applied); next free number 040**); there is no migration table: check the catalog. **034 case layer** applied 2026-11-10; **037 audio files** and **038 image search** applied (038 on 2026-11-23 after backup `.phase2-backups/live-20261123T043746Z.dump`, 516 MB); 41 analyst views; backups in the main checkout's `.phase2-backups/` and `C:/tmp/nexusai-backups/` |
| Reads | `GET /analysis/connections`, `/places`, `/engine-health`, `/typing-report`; `GET/PATCH /cases`; the case layer (`/conversations`, findings, members, exhibits); `/images/search` (words, copies, reference, region, crop) |

**Regression anchors:** forensic-demo call records 8,642 in version 1, **6,239 effective** and 2,500 internet sessions; the acceptance case 10,168 structured rows (43 sources); judgement cases 101 to 104, 106 to 108 are spent
(105 unspent), the four sealed cases are never used; the Orion bank figures in section 1.

## 8. Tools and evidence

`tools/dev/check_change.py` (tiers as one command), `typing_report.py COLLECTION`, `live_smoke.py [--quick] [--real]`, `Restart-Workspace.ps1`, `live-window/` (3-recreate-api, 10/11 image search, rehearse-evidence,
run-staging-api); `tools/case_generator/` (cases with planted truth, loader, checks); `tools/ask_eval/` (banks, judge, red team, impact index); **`tools/showcase/`** (`build_verified_bank.py`, `judge_verified_bank.py`,
`carry_results.py`, `ask_case.py`, and the exhaustive-search banks `ask_exhaustive_bank.py` (bank-A, B, C) with `ask_exhaustive_score.py`); `evaluation/text_intent/` (intent banks and `score_tier1.py`; the live test
`TestTextIntentLive`); `tools/media_data/` (face, image-search and speech-data measurements); `tools/real_data/` (vault tools, scanner); `ingestion/forensic_records/layout_registry/` (layouts) and `typed_ingest.py`.

## 9. Next — the exact next action

Follow `docs/roadmap/reconciled-plan-20261124.md` section 5. **A1 done (2026-11-24):** this file; the verified bank rebuilt and re-asked. **Next: A2** (Ask Phase 2) and **B** (cross-case X0 and X1, the audio product). **Waiting for the owner:** where to `docker save` the live worker and API images (backup of the only copy), whether the image-search PDF report goes into the repository (the media session says its four pictures are screenshots of a public COCO photo; not checked here), whether to move the 60 image-search test photographs out of the reference case (a write to retained evidence), images next (watch lists first is
recommended), the cross-case order X0 to X3, reviewer names D10, capture names D26, the plate masking choice D42, the Common Voice click for Saraiki. **Needs the owner every time:** a deploy, a migration, a
database write, a model download, anything touching retained evidence, a push, a `--no-verify` commit.

## CODEX_UI_TRACK

- **2026-11-24 (written by the continuation session during A1; the interface session may rewrite it):** the interface is built by Claude sessions on branch `ui/v2-redesign` (worktree `C:\Users\sheik\Workspace\Office\Projects\NexusAI-ui`, served on :4182, pushed to origin 2026-11-24, tip `a10ce43`). Built and shown: design language and shell, the Ask page with server-side conversations, the dashboard (S2a to S2d), the evidence gallery with openable cards and media viewers, the image-search results and draw-a-box. Next in the reconciled plan: the "also appears in" chip and the cross-case views once the identifier registry exists, the files of a records answer (Ask Phase 2), then the cut-over preparation (usability test, route order). :4181 keeps the old design.
- **2026-11-03: UI_OWNER = Claude (Codex is gone); FULL REDESIGN.** The owner asked that everything Codex built be redesigned and recreated, every page and section one at a time, best of the best (amendment banner at the top of the directive). Branch `ui/v2-redesign` in `C:\Users\sheik\Workspace\Office\Projects\NexusAI-ui`, served on :4182 (restart it after every API recreate: `tools/dev/Restart-Workspace.ps1 -Port 4182 -Tree ui -Collection orion-lahore-reference-20261030`). Order: design language, shell, Ask, Evidence, Dashboard/Cases/Case, workbench, Money/Vehicles/Media, Entities/Findings/Reports, Activity/Settings, polish. The owner accepts each page by looking before the next starts. :4181 keeps the OLD design; nothing merges into `lane-arms` without the owner's separate word.
- **2026-10-26: ONE interface directive.** [`docs/ux/MASTER_UI_UX_DIRECTIVE.md`](docs/ux/MASTER_UI_UX_DIRECTIVE.md) merges every earlier UI brief (ledger in its section 17; the older briefs carry a SUPERSEDED banner), with the research register `docs/ux/research/RESEARCH_REGISTER_20261026.md` and the paste-ready Codex prompt `docs/ux/CODEX_UI_START_PROMPT.md`. The workbench (U3 v1, f7d61fc) is built and live; its restyle is stage U3.
- **2026-10-28 OWNER RULE: :4181 keeps the OLD design with all its working functions and engines; the NEW design is built only in the interface worktree on :4182.** Merge an interface stage into `lane-arms` ONLY when the owner decides the new design replaces the old one, and say what :4181 will look like before doing it. Backend and engine work continues on `lane-arms` and is unaffected.
- (older entries are history in `reports/archive/`; the directive wins where they differ)
