# tools/dev: running the product and checking a change without waiting an hour

The policy is in `docs/roadmap/reconciliation-20261020.md`, section 6. In short: **a change is checked by the cheapest tier that can catch it; the full banks and spreads run at a milestone, in the background, and are read, not waited on.**

| Tier | What | Time | Command |
|---|---|---|---|
| T0 | Unit and live-database specs of the paths that changed (Go engine specs, layout registry, typed ingest, judge, factory) | 1 to 3 min | `python -X utf8 tools/dev/check_change.py` (also automatic in pre-commit for Go) |
| T1 | **Typing report**: what the typed tables of a collection hold and where the engine could be misled (exact repeats, event types read as other, directions, populated columns, assumed time zones, unstated volumes, files typed against files the guard counts). Counts and shares only, no model | seconds | `python -X utf8 tools/dev/typing_report.py COLLECTION` |
| T2 | **Live smoke**: engine health, a template request writes no audit row, the IMEI check (the original failure), the cell-site sentence, the connection answers; `--real` adds the six checks on the private real collection (counts and booleans only) | about 5 min (`--quick`: about 3) | `python -X utf8 tools/dev/live_smoke.py [--quick] [--real]` |
| T3 | **Re-ask only what changed**: the questions that failed or that the change can touch. On a typed case, **re-judge saved answers on the effective views with no model**: `factory.py reeffective ARM --questions FILE --psql "..."` (25 s) | 1 to 3 min | see `evaluation/question_factory/factory.py --help` |
| T4 | **Milestone runs**: the full banks (`tools/ask_eval`, `tools/case_generator/check_ask_engine.py --register-case ...`) and the factory spreads on fresh seeds, with gates written first | 20 to 70 min | in the background, once per milestone; read the summary |

**One command for a change:**

```
python -X utf8 tools/dev/check_change.py --typing nexusai-forensic-demo --smoke --quick
```

runs T0 for what changed in `git status`, the typing report for the collection, and the quick live smoke, then prints a table of steps and times. Add `--db-specs` when the executor, the coverage read or the audit changed (it starts a throwaway Postgres, about 2 minutes).

**Rules that keep the cycle short**

* A fix is measured on another unspent seed; a bank is never re-run for a change that cannot touch it; a run that hits a limit (the one-hour Ginkgo suite timeout, a machine's sleep) re-runs only the questions that did not run.
* After any layout, typing or back-fill change run T1 **before** any question: four of the five surprises of 2026-10-20 were visible there in seconds.
* After every deploy run T2, and after every API recreate restart the dev servers (`Restart-Workspace.ps1`), or requests hang.
* Judge a typed case against keys read from the effective views (`--effective-views`), not version 1's table: on forensic-demo six of seven "wrong" verdicts were that difference.

**The other tool here:** `Restart-Workspace.ps1 -Port 4181 -Tree main` / `-Port 4190 -Tree lane` restarts the workspace dev server after an API recreate, reading the key from the env file (never printed) and checking that the proxy reaches the API.
