# Handoff to a local session (2026-10-07)

Read this first, then `docs/architecture/QUERY_ANSWERING_DECISION_20261005.md`, then `reports/governed-sql-20261005/PREREGISTRATION.md` (every run, every gate, every fix, as measured) and `AGENTS.md`.

## Where things are
* Branch `claude/gifted-pasteur-4kpx0r` (checkout `NexusAI-localwork`, local branch `lane-arms`). The laptop's own work is on `local-backend-work-20261005` in `NexusAI`; do not mix them. Commit and push only to the lane branch.
* Governed SQL lane: `api/forensic_records/governed_sql_*.go`. Two switches, both default off and registered in `behaviourSwitches`: `FORENSIC_GOVERNED_SQL` (stage 1, after the existing path) and `FORENSIC_GOVERNED_SQL_FIRST` (stage 2, before it).
* **Stage 1 is ON in the running container** (image built from `cb0f3bc`). `Disable-Lane` (in `evaluation/question_factory/Run-Arms.ps1`) rolls it back. It is not persistent across a full stack restart: the switch must be set where the stack starts, and the main checkout's compose file does not declare the switch (the lane branch's does).
* Measured (demo case `nexusai-forensic-demo`): factory set 73/82 correct, 2 wrong (arm B); unseen set 72/82, 3 wrong; corpus and pre-flight read; all gates met. Details and the gate table are in the pre-registration, runs 1-9.
* The main working case is `nexusai-multimodal-product-acceptance` (audio, video, ANPR images, faces). **The lane has not been measured there.** Next step: `New-QuestionSet -Seed 3 -Name questions-multimodal.json -Collection nexusai-multimodal-product-acceptance`, then `Run-Arm A` and `Run-Arm C` with `-Questions questions-multimodal.json -Tag -mm`.
* Open UI problem: the Ask page goes white after a long question and a refresh starts a new chat. Not reproduced offline (the lane's responses render fine in the workspace components, `governed-sql` vitest). Needs the browser console error (F12) and `docker logs nexusai-forensic-records-api-1`. The UI is `apps/investigation-workspace` (port 4181).
* Fixed today: lane answers carried intent `semantic`, which the workspace shows as "Unavailable"; now `records`.

## Tools you now have (PowerShell, `. .\evaluation\question_factory\Run-Arms.ps1`)
`Show-Stack`, `Build-LaneImage`, `Run-Arm A|B|C -Questions <file> -Tag <suffix>`, `Run-Regression A|B|C`, `Rejudge-Arms`, `Explain-Question <ids> -Questions <file>`, `Ask-Case "<question>" -Collection <case>`, `Enable-LaneStage1`, `Disable-Lane`, `Restore-Stack`, `New-QuestionSet`, `Verify-Round`, `Finish-Round`.
Secrets: `Run-Arms.ps1` reads `FORENSIC_RECORDS_API_KEY` and the alias secret from `.env.forensic-runtime.local` (gitignored). Never print or commit them.

## Rules that stand
* The GitHub repo is PUBLIC: never push real data, PII or secrets. Generated factory files (`questions*.json`, `arm-*/`) stay local and gitignored.
* Ask the product owner first for: database writes or migrations, container rebuild or redeploy beyond the lane arms above, compose actions beyond ps/logs, live eval suites, model downloads. Do not modify the case database (read-only SELECTs only).
* Thresholds are written before measuring; no re-running with reworded prompts after a gate fails: write the failure taxonomy and shrink the scope. Every behaviour sits behind its own default-off switch declared in compose AND registered in `behaviourSwitches`.
* No model narrates a number; the model never sees case data; abstention is success; a confident answer to a different question is the one defect that must reach zero.
* Commits: no `Co-Authored-By` AI trailer (AGENTS.md); use `Assisted-by:` and `Claude-Session:` lines only. Do not bypass hooks or lower coverage baselines (one-time `--no-verify` was authorised for the 73-file laptop backend commit only).

## Open decisions for the product owner
`event_time` or `call_start` for "June" and "night" on call records; vocabulary gaps ("registered numbers", "log entries"); the silent drop of a condition in words no layer field maps to (name it as a layer field, or have the model report what it did not apply); merging the lane branch into the branch the stack runs from; the read-only database role for analyst views (R9); stage 2 after stage 1 has run on real questions.
