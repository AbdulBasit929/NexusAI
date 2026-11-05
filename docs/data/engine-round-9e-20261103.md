# Engine round 9e (2026-11-03): the faults the full verified-bank run left

**Status: built and tested on the lane (`lane-arms`); NOT deployed.** The live engine is still round 9d (image `round9d-20261102` = `c4508a3f7714`). Deploy needs the owner's word (section 7).
Evidence: the saved bank results (`C:/tmp/nexusai-g4/vb_final.jsonl`, `vb_recheck_9d.jsonl`), the audit rows of the live database for the Orion case (`forensic.records_audit_log`, synthetic data), and the sheet
`docs/qa/verified-bank-20261102-sheet.*`. Plan item: `docs/roadmap/master-roadmap-20261103/06_roadmap_phases.md`, Phase 0 task 0.1.

## 1. What was wrong, found from the audit rows and not guessed

| Question | Saved outcome | Cause | Fix (file) |
|---|---|---|---|
| VB-BANK-050, 091, 112, 123: "which accounts sent money **to** the account X" | abstained after three attempts | The direction cue came from the verb ("sent ... to" = outgoing). The check demanded `'outgoing'` or `'debit'`, the round-8 check `DIRECTION_ROLE` demanded `'credit'`, and the prompt said "Outgoing events only." The two checks contradicted each other, so the **right query** (the credits of the account: the text of the verified example `txn-sent-money-to-account`) was refused every time. The audit rows of both runs (2026-11-02 17:29 and 2026-11-03 05:38 UTC) show the repairs `DIRECTION_ROLE`, `DIRECTION`, `DIRECTION`: the first draft read the debits, the second the credits (refused as "not outgoing"), and the third put the account in the **counterparty** column to satisfy "outgoing", which is a wrong reading that the new check also refuses | The direction is **relative to the identifier the question names**, read from its place in the sentence: after "to" it is the receiving end (incoming), after "from" the sending end (outgoing), before the verb the doer (the verbs decide, as before). The whole-question pattern `askSentToAccountRE` is gone. `ask_engine_roles.go`, `ask_engine_frame.go`, `ask_engine_run.go` |
| VB-BANK-004: "largest single debit" | timed out twice (420 s, no answer) | **Not a model problem first.** Audit rows 2026-11-02 11:06 and 14:12 UTC: three attempts, repairs `OBLIGATION:ORDER` twice, model time 299,984 ms. The query was right (`SELECT max(amount) ...`, the verified example `txn-largest-debit`) but the ORDER obligation demanded an ORDER BY for every ranking word ("largest"). The third call met the 300 s budget of commit 1947ed2, the engine **declined**, and the lane and the older path ran on until the client gave up | ORDER is waived when the question asks for a **value** (not "which/who") and the query takes it with MAX or MIN (`askExtremeValue`, `ask_engine_rankings.go`). A model call that ran out of time (its deadline, or at least `askMinModelCall` seconds) or a spent budget now ends with its own honest response, "I could not finish this question in time ... This says nothing about what the records hold", audit `timed_out`, header `timed_out=true`, no hand-over to the slower paths (`ask_engine_timeout.go`). Token cap 480 to 320: the longest query ever written is 662 characters (about 240 tokens) over 1,185 answered questions |
| VB-MESS-019 (and 009) | abstained: "about one kind of event" | The attempts were `UNKNOWN_COLUMN`, `UNKNOWN_COLUMN`, then a ranking over `v_contact_pairs`, which counts every kind of event together and has **no `event_type` column**; the repairs only listed that view's columns, and the EVENT_TYPE repair said "filter event_type = 'sms'", which that view cannot do. (MESS-009 was answered correctly by round 9d because its first draft happened to be right.) | The `UNKNOWN_COLUMN` message names the offered view that has the column; the `EVENT_TYPE` message names the view with `event_type` when the query reads views without it (`ask_engine_kinds.go`; `askValidated` carries `Cat` and `Allowed`). The shortlist was not changed (blast radius) |
| VB-PLAT-051: "which plate was seen the most" | wrong: one plate shown, SYNT4024 (50); SYNT2905 also has 50 | `ORDER BY value DESC LIMIT 1` hides a tie, and the model cannot see the data | After a ranking that keeps one row has run, the same canonical query is run again with `LIMIT 51` (validated like any query) and every row with the first row's figure is kept: "2 plates tie for the top place at 50: SYNT2905, SYNT4024." `ask_engine_ties.go`; audit `tied`. The judge and the bank read a tie as a **set** (`ties` in the expectation) |
| BOM in the first CSV heading | the IPDR CSV's first column read `BOM + "UFDR start time"`, its cells blank | **Not the Python reader** (it decodes `utf-8-sig`). The mark was stored in `evidence_items.metadata.headers` by the **Go upload reader** (`validateDetectedHeaders`, `encoding/csv` keeps it): 48 of 161 evidence items | Go: `fileRowsCleanNames` (read time, covers the stored data), `validateDetectedHeaders` (new uploads). Python: `header_keys`, `disambiguate_headers` |
| `.xls` and `.txt` column order in `file-rows` | database key order (`app, sid, url, host, imei ...`) | The typed rows keep their cells in a JSON object, which has no order, and the Go upload reader's list is a guess from the first line (`line` for a text file) | `SheetSummary.headers` (names in the file's order, from the same `header_keys` that keys the payload), `typed_ingest.file_headers`, written by the worker to `evidence_items.metadata.typed_headers` for typed workbooks and typed text exports; `loadFileRows` prefers it in the typed fallback. Backfill of stored evidence: `tools/dev/backfill_typed_headers.py`, **dry run only** (section 6) |
| VB-INVE-004: "text files" | 14 by extension, 4 by kind | A fair reading; decided: **4**, the files of the text kind | The answer now says which reading was used ("... the files counted are those the system recognised with modality = 'text'. Another reading, such as the ending of a file's name, can give another number."): `askDomainNotes`, one line, no number |

A second finding came from holding **every verified example to the obligations of its own question** (a new spec): `data-by-host` ("which hosts did X send the most data to") was refused by its own DIRECTION check, because the
direction of a session is carried by `bytes_up` and `bytes_down`, not by a filter. Both are now read as the direction they are.

## 2. Why this is general, not a template

No new operation template, no keyword ladder, no whole-question pattern: the direction comes from the place of each named identifier in the question's own words (`askIdentifierRoles`), the ORDER waiver from "does the question ask which or who",
the kind-of-event hint from the columns of the catalogue the query was validated against, the tie from the parse tree of the query that ran (`askRankKey` finds the sort column by name, position or expression). The one rule that names
views is the list of direction-carrying columns (`bytes_up`, `credits`, ...), which is the columns' own meaning.

## 3. Specs, mutation checks

Run 2026-11-05 under `heavy.lock`, `-p 1`, `GOMEMLIMIT=1500MiB`, a throwaway Postgres for the database specs.

| Tier | Result |
|---|---|
| Focused specs (round 9e, file rows, time-out) | 34 of 34 pass |
| Whole Go package, no database | **1,612 passed, 0 failed, 228 skipped** (98 s; build 113 s) |
| Database specs (`ASKENGINE=PASS`, incl. the 2 tie specs on a case database) | **33 of 33** (758 s) |
| Python (`tests/test_headers_round9e.py` + the existing suite) | 5 new + 196 existing pass |

**Mutation checks** (one text replacement removes one fix; the specs that belong to it must fail; the file is restored by SHA-256 afterwards). All 12 Go and 4 Python mutations are killed.

| # | What the mutation removes | Result |
|---|---|---|
| M1 | direction not read from the place of the identifier | KILLED |
| M2 | a query on the other end is not read from the senders' side | KILLED |
| M3 | the ORDER rule is not waived for MAX and MIN | KILLED |
| M4 | a model call that ran out of time is a decline again | KILLED |
| M4b | a spent question budget is a decline again | KILLED (earlier run) |
| M5 | the run does not look for a tie | KILLED (earlier run) |
| M5b | the tie extension never finds a tie | KILLED |
| M6 | the kind-of-event message does not name the view | KILLED |
| M7 | the unknown-column message does not name the view | KILLED |
| M8 | `file-rows` names keep the byte-order mark | KILLED |
| M9 | the typed header list is not read | KILLED |
| M10 | the answer does not say which value the files were counted by | KILLED |
| M11 | the upload reader keeps the byte-order mark | KILLED |
| M12 | `bytes_up` / `bytes_down` do not carry a direction | KILLED |
| P1, P2, P4, P5 | the Python header-list and byte-order-mark fixes | KILLED |

Groups: A = M1, M3, M6–M12 (one compile, 184 s, 15 specs failed, none unexpected); B = M2, M4, M5b (16 s, 5 specs failed). Two mutation texts (M4, M10) first left an unused variable and did not compile; they were rewritten with `&& false` and rerun. A fifth Python
mutation (a second byte-order-mark strip in `readers.py`) survived because `header_keys` already strips it, so that edit was removed instead of kept as dead code.

## 4. The impact index and the re-ask

**Impact index** (`tools/ask_eval/impact_index.py`, built once from the saved bank results): 17 targets (6 misses: BANK-004/050/091/112/123 and MESS-019, plus 11 `largest_debit` answers that passed only after ORDER repairs), 47 risks, 18 neighbours, 402 untouched.
Re-reading the role rule over all 692 saved questions (bank + query pack) moves exactly 5: BANK-050/091/112/123 and ORL-TXN-008. The replay of the 484 saved queries against the new checks is in `docs/data/engine-round-9e-impact-index-20261103.md`.

**Re-ask**: 15 questions, one at a time, on a staging API (`127.0.0.1:8191`) built from this tree over a restored copy of the live database (dump of 2026-10-31, migration 032 applied to the copy), the real local model, judged by `judge_verified_bank.py`.
Result: **14 correct, 0 wrong, 1 abstained (VB-MESS-019), 0 timed out.**

| ID | Fault | Engine said | Latency | Judge |
|---|---|---|---|---|
| VB-BANK-050 | (a) sent TO the account | 7 rows | 105 s | correct |
| VB-BANK-091 | (a) | 7 rows | 31 s | correct |
| VB-BANK-112 | (a) | 5 rows | 29 s | correct |
| VB-BANK-123 | (a) | 7 rows | 33 s | correct |
| VB-MESS-019 | (b) number sent the most texts | abstained: EVENT_TYPE after two UNKNOWN_COLUMN repairs | 68 s | abstained |
| VB-MESS-009 | (b) must stay correct | 5 results (16, 13, 12, ...) | 50 s | correct |
| VB-PLAT-051 | (c) tie at the top | "2 plates tie for the top place at 50: SYNT4024, SYNT2905." | 76 s | correct |
| VB-BANK-004 | (d) used to time out | Largest: 490,500 | 66 s | correct |
| VB-INVE-004 | (g) stays 4 | N: 4 | 80 s | correct |
| VB-INVE-003 | neighbour | N: 3 | 25 s | correct |
| VB-BANK-014 | neighbour (ORDER waiver) | Largest: 494,500 | 39 s | correct |
| VB-BANK-008 | neighbour | top payee, 1,312,676.9 | 86 s | correct |
| VB-CELL-001 | neighbour | 1 cell site | 79 s | correct |
| VB-PAIR-011 | neighbour | 6,183 seconds | 48 s | correct |
| VB-CALL-012 | neighbour | busiest day | 116 s | correct |

**VB-MESS-019 is not fixed.** The repair now says where `event_type` lives (v_comm_events), and the model reached that view in its second draft, but the third query still did not filter on the kind and was refused. It stays an abstention (never a wrong answer). The cause is the model's draft, not the check; making this reliable
would mean a better shortlist for "rank by kind of event", which was not done here.

**File rows (faults e and f), on the copy:** the IPDR CSV's first column is `UFDR start time` with its cells filled and no byte-order mark. For the `.xls` (18 columns) and `.txt` (27 columns) the columns were in the database's key order before (`app, sid, url, ...` / `APN, No., URL, ...`); after the backfill was applied to the COPY
(7 items, header row read) they are in the file's own order (`Public IP, Public Port Start, ...` / `No., Public IP, Public Port, ...`). The live database was not touched.

## 5. What is not done, and what this does not say

* **Not deployed, and not a full bank.** The re-asks of section 4 are about a dozen questions on a restored copy of the live database with the real model; the other ~470 questions were **not** asked again (the tiers of `tools/dev/README.md`; a bank is eight hours). What replaces the
  full run is the replay: every saved query of the bank, held to the NEW checks with no model, says which previously accepted query would now be refused (section 4). That bounds the change in the checks; it does not measure how the model behaves under the changed hints.
  The hints change only for a question whose direction was read from the place of its identifier (5 of the 692 saved questions of the bank and the query pack), the repair messages only on a repair turn, the answer only for a one-row ranking.
* **The time-out path is proved by specs, not provoked live.** A scripted model that stalls until its deadline gives the honest response and no hand-over; a real stalled model service on the staging API was not produced.
* **`typed_headers` is written only by a worker that carries round 9e** (a worker overlay image, a separate rebuild, not part of the API image). Until then new `.xls` and `.txt` ingests keep the database's key order, as before, and so does the stored evidence until the backfill is applied (the owner's go; section 6).
  The byte-order mark needs none of that: the read path of the API strips it from every stored list.
* **Limits of the rules.** The role rule reads the prepositions "to" and "from" only ("who called X" with X as the object is not read; a question that names the doer and the other end keeps the verbs' reading, as it always did). A one-row ranking whose sort key the parse tree cannot match to an output
  column (an expression that is no output) gets no tie look. The shortlist can still offer `v_contact_pairs` for a ranking by kind of event; the repair now says where to go, so the cost of a wrong first draft is one repair, not an abstention. `FORENSIC_ASK_V2_AGREE` stays off.
* **Timeouts are audited as `abstained` with `timed_out: true`**; the engine-health read does not separate them yet.
* **The rows behind a figure (part A of the evidence) are collected for the query as the model wrote it** (one row), so for a tie they cover the first of the tied results; when the counts differ, the check line under the evidence says that the list is not shown as proof of the answer. Widening the evidence to every tied result is a small change in `ask_engine_evidence.go`, not made here.
* **VB-INVE-004 stays 4** (decided: the files of the text kind); the answer says which reading was used, without a number.

## 6. Backfill of the header list (dry run)

`python -X utf8 tools/dev/backfill_typed_headers.py --collection orion-lahore-reference-20261030` lists the evidence that has typed rows, no version-1 records and no `typed_headers`: **7 files in Orion** (3 `.xls`, 4 `.txt` IPDR exports).
With `--files-root C:\NexusAI-Showcase\orion\files --show-names` it read the header row of the synthetic copies: 18 columns for the `.xls` (`Public IP, Public Port Start, Public Port End, Log Date, app, host, imei, imsi, msisdn, sid, url, cell/eci, ...`)
and 27 for the `.txt` (`No., Public IP, Public Port, NAT Begin Time, ...`), in the file's order. Nothing was written. `--apply` needs `NEXUSAI_OWNER_GO_TYPED_HEADERS=yes` and `--files-root`, and adds one metadata key; it never reprocesses a file.
Not needed for the byte-order mark: the Go read path already strips it from the stored list.

## 7. Deploy plan and rollback (nothing here has been run; the owner's word is needed)

**What the API image alone brings:** faults (a) to (d), (g), the byte-order mark (read path and upload reader) and the tie. **What needs a worker overlay as well:** `typed_headers` for new `.xls` and `.txt` ingests (f). No migration, no database write, no backup needed (the engine only reads; the dump of 2026-10-31 stays).

```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork
# 1. the rollback tag is the image that runs now (round 9d, c4508a3f7714)
docker tag nexusai-forensic-records-api:round9d-20261102 nexusai-forensic-records-api:before-round9e-20261103
# 2. build the image from the COMMITTED tree (heavy: take the heavy lock; the model server idle; free memory)
docker build -f api/forensic_records/Dockerfile -t nexusai-forensic-records-api:round9e-20261103 .
# 3. dry run: compose's config hash must equal the running label (74eb879ac9e9), nothing to add or differ
.\tools\dev\live-window\3-recreate-api.ps1 -To round9e-20261103 -RollbackTo before-round9e-20261103
# 4. apply, only after the owner's word
.\tools\dev\live-window\3-recreate-api.ps1 -Apply -To round9e-20261103 -RollbackTo before-round9e-20261103
# 5. restart BOTH dev servers AT ONCE, before anything else (else wslrelay hangs :8091 and the script says unhealthy)
.\tools\dev\Restart-Workspace.ps1 -Port 4181 -Tree lane -Collection orion-lahore-reference-20261030
.\tools\dev\Restart-Workspace.ps1 -Port 4182 -Tree ui -Collection orion-lahore-reference-20261030
# 6. then: /healthz, python -X utf8 tools/dev/live_smoke.py --quick, and the questions of section 4 through the workspace
```

**Rollback:** `.\tools\dev\live-window\3-recreate-api.ps1 -Rollback -RollbackTo before-round9e-20261103`, then restart both dev servers again.
**The word the owner types to approve it:** `Deploy round 9e` (the classifier needs the owner's chat word for every deploy; "round 9e" names this note).

Optional and separate: a worker overlay carrying `typed_ingest.py`, `worker.py` and `layout_registry/interpret.py` (the recipe of `tools/dev/live-window/4-recreate-worker.ps1`; checked with `tools/case_generator/check_worker_overlay.py`) so that new ingests keep `typed_headers`; the backfill of stored evidence (section 6) needs the owner's go on top.
