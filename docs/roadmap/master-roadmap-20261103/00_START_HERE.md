# NexusAI roadmap pack of 2026-11-03: start here

**What this is.** The reconciled state of the project, the media programme (images, video, ANPR, audio and speech in English, Urdu, Pakistani Punjabi, Pashto and Saraiki, faces), the architecture (cases, cross-case, evidence, the Ask engine, history, structured families, reports, security), the interface programme (a full redesign by Claude now that Codex is not available), the data and testing policy with the researched dataset register, and the phased roadmap with gates and the decisions the owner must make. Written 2026-11-03 after a full read of the repository, the live system, the memory of earlier sessions and today's web research.

| File | Read it for |
|---|---|
| `01_state_of_the_project.md` | what exists, what is live, what was measured today, what is missing |
| `02_media_programme.md` | **the main focus now**: every media type, language and operation, with models, licences, gates and work packages |
| `03_architecture.md` | case, cross-case, evidence viewers, the runtime-LLM Ask engine and its roles, history, families, reports, security |
| `04_ui_ux_programme.md` | the redesign: rules, pages, Ask and Evidence specs, chart catalogue, stages, cut-over |
| `05_data_testing_research.md` | datasets (with licences), synthetic programmes, tests without repetition, banks, research log |
| `06_roadmap_phases.md` | Phase 0 to 4, lanes, gates, the first 14 days, what to protect first |
| `07_decisions_and_risks.md` | decisions D34 to D43, risks, what is not promised, what is superseded |

Binding documents that this pack does **not** override: `AGENTS.md`, `NEXUSAI_CONTINUATION.md`, `docs/architecture/RECONCILIATION_20260921.md` (section G), `docs/architecture/QUERY_ANSWERING_DECISION_20261005.md`, `docs/ux/MASTER_UI_UX_DIRECTIVE.md`, `docs/ux/NEXUSAI_PRODUCT_UX.md`, `docs/data/real-data-handling-policy.md`, and the master plan `docs/roadmap/master-plan-20261021.md` for the ledger of older items.

---

## The prompt to paste into the new chat

> You are continuing the NexusAI forensic-intelligence project for its owner (a developer who must defend the work to a team lead; they want short, concrete, verified, honest answers and PowerShell-ready commands, and they do not want long repeated test cycles).
>
> **First read, in this order and in full:** `AGENTS.md`, `NEXUSAI_CONTINUATION.md`, then the roadmap pack `docs/roadmap/master-roadmap-20261103/00_START_HERE.md` and the files it lists, then `docs/ux/MASTER_UI_UX_DIRECTIVE.md` (only when working on the interface). The lane tree is `C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork` (branch `lane-arms`); the interface worktree is `C:\Users\sheik\Workspace\Office\Projects\NexusAI-ui` (branch `ui/u2-shell-navigation`, served on `:4182`); `:4181` serves the lane tree with the OLD design and must keep working; the main checkout `...\NexusAI` holds `.env.forensic-runtime.local`, which you never read, print or copy (scripts read it themselves).
>
> **What the owner wants now (2026-11-03):** (1) implement the media data types best of the best: images, video, ANPR images and video, audio, speech-to-text in English, Urdu, Pakistani Punjabi, Pashto and Saraiki, face detection, recognition and matching, and every relevant operation, with working viewers; (2) the best architecture for cases, cross-case, evidence use, the Ask chatbot, complete chat-style history with re-work, and every structured family and file type; (3) recreate and redesign the whole interface to the directive (Codex is not available, so you do it), fixing first the Ask page (sections not shown completely, wasted side space, too compact) and making every evidence file open complete and accurate in a proper viewer; best dashboards, charts and visualisations; (4) real, relevant datasets from online sources until real data is supplied; (5) **no predefined question templates or operations**: questions are understood and answered at run time by the language model with code validating (the model roles are in `03_architecture.md`), including a measured greeting/help/clarification role. The owner approved the decisions listed in section 2 of the pack's file 01 and the plan's recommendations; the new decisions D34 to D43 are in file 07: ask them **once, as one list**, in your first message.
>
> **Live state (verify first):** API image `nexusai-forensic-records-api:round9d-20261102` = `c4508a3f7714` (rollback tag `before-round9d-20261102` = `da91800b051a`); worker `47e5e829a3d4`; LocalAI container `nexusai-api-1-prev-gbnf` (never start `nexusai-api-1`); API on `127.0.0.1:8091`; verified bank 473/484 on round 9c. After **any** API recreate restart the workspace at once (`tools\dev\Restart-Workspace.ps1 -Port 4181 -Tree lane -Collection orion-lahore-reference-20261030`), then check `/healthz`.
>
> **Rules that stand:** synthetic or public data only outside the vault; no evidence, crop, transcript or embedding to any third party; model output is an observation never a fact; no identification claims from faces, voices or plates; licence-clean components only (no AGPL, non-commercial weights are evaluation-only, every component has a ledger row); one heavy model at a time and one evaluation container at a time (`--cpus 8 -m 5g`, the Docker VM is 7.6 GB); roles switch on one at a time behind a pre-registered gate; judgement and sealed cases are spent when used; never merge interface work into `lane-arms` without the owner's separate word; **ask before** a deploy, a migration, a database write, a model download, anything touching retained evidence, a long build; the owner's chat word is needed per deploy.
>
> **Commits:** explicit paths only (never `git add -A`; do not stage `tests/e2e-ui/Dockerfile`, `tests/fixtures/backend-image/Dockerfile`, `tools/visual_spike/package-lock.json`); message from a file with `git commit -F`; end with `Assisted-by: Claude Code:<model-id> [tools]` and `Claude-Session: <id>`, **never** `Co-Authored-By` for an AI; the pre-commit hook runs the whole Go package when Go files change (about 4 minutes: commit in the background); no force-push.
>
> **Work method:** follow the pack's phases; finish a capability only with a view, a Playwright test on a generated case, a bank with exact truth and an owner demonstration; test with the tiers (unit, typing report, live smoke, re-ask only what a change touches; full banks only at milestones, in the background); write a stage note and refresh `NEXUSAI_CONTINUATION.md` (rewrite, never append, under 300 lines) and the memory at every gate; report in plain words with the page to open. Tool traps: PowerShell 5.1 splits unquoted dotted flags (quote them); Git Bash has no `python3` (use PowerShell `python -I -X utf8`); the Edit tool changes line endings of mixed CRLF/LF files (check `git diff --stat`); Windows psql needs `SET app.tenant_id='default';`; start Docker via `explorer.exe`.
>
> **First actions:** (1) confirm the live state; (2) send the owner the decision list D34 to D43 and the question "have you reviewed U2 on :4182?"; (3) run Phase 0 of `06_roadmap_phases.md`: engine round 9e, data fetch under ledger rows, the Omnilingual/Whisper baselines on the development partitions, drafts of migrations 033 and 034; (4) split Phase 1 into parallel sessions with file ownership (engine, speech lab, conversations backend, U4 Ask) and start them.

---

## The same thing in twelve lines (for a short message)
1. Strong: typed structured families and the Ask engine (verified bank 473/484, 0 confident-wrong), link analysis, evidence viewers.
2. Weak: media (OCR and turbo speech on; faces, video, plates off; the engine cannot see media), no server-side case layer (notes, history, members, cross-case), interface split between old `:4181` and new `:4182`.
3. Today: bank run done; fixes cherry-picked and **round 9d deployed and checked on real files**; two small defects found (BOM in CSV headings, `.xls`/`.txt` column order); a deploy gotcha recorded.
4. Media plan: one pipeline (intake, proxies, tasks, versioned artifacts, review, views for the engine); speech with a router per language; Pashto fixable by public-data fine-tuning (about 25% WER read speech), Saraiki and Pakistani Punjabi weakest (Omnilingual CER 17 and 31); plates and video with RF-DETR, tracking and a versioned plate-format registry; faces L1 then L2 under strict governance.
5. Architecture: case profile, authority, members; identifier registry and reversible entity resolution for cross-case; viewers for every type with a safe inspector as the last resort; model-role registry; server-side conversations with re-run and "changed since".
6. Interface: new design on `:4182` (Ink and signal), order U4 Ask, U6 Evidence, U5, U3, U7, U9, U8, U10; the 2026-11-02 Ask/Evidence work is carried over.
7. Data: Common Voice (Urdu, Pashto, Saraiki), FLEURS, Omnilingual corpus, MEVA, CCPD, synthetic Pakistani plates, codec-simulated telephony, consented faces; every one with a ledger row.
8. Tests: tiers and an impact index; no unchanged reruns; banks at milestones in the background.
9. Phases: 0 close-out (week 1), 1 foundations (weeks 1 to 5), 2 media in the product (5 to 11), 3 intelligence across families and cases (11 to 17), 4 enterprise and release (17 to 24).
10. Decisions D34 to D43 wait for the owner; recommendations are given.
11. Biggest limits: no GPU, no consented data yet, low-resource languages, legal exposure of face recognition.
12. Nothing is promised that is not measured on our own gold sets.
