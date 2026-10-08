# Demo guide: asking the case database questions in plain words (2026-10-08)

For presenting to a team lead. Every question and answer below was measured in a recorded run (`reports/governed-sql-20261005/PREREGISTRATION.md`); nothing is invented. Where today's behaviour is weaker than the target, this guide says so, because a demo that hides the limits gets caught by the first unscripted question.

## 1. The story in one minute

An analyst asks a question about a case in their own words. The system answers from the database in one of three ways, and never a fourth:

1. **Answered:** a number the database computed, the query that produced it, and what it was checked against.
2. **Abstained:** it names the condition it could not apply and shows no number. This counts as success.
3. **Declined:** it hands the question to the older path, unchanged.

The one defect that must reach zero is a confident answer to a different question (for example, answering "how many calls did Ahmed make?" with the total number of calls). How it works: a small AI model only writes one database query; the server checks that every condition of the question is in it, runs it read-only, and the database produces the number. The model never sees case data and never states a number.

```
question
  1. read it            what it demands: identifiers, values, dates, times of day, names, "one number or a list"
  2. model writes       ONE SELECT over server-built views; it sees the schema and the question, never case data
  3. parse and verify   only that SELECT is allowed; every demanded condition must be in it; one retry, with the reason
  4. run read-only      server-built case scope, timeouts, cost ceiling, row cap, the case clock
        |-> answered   the number from the database + the query + what was checked
        |-> abstained  names the condition it could not apply; no number
        |-> declined   the older path continues unchanged
```

## 2. Before the demo (checklist)

| Step | Command or action | Why |
|---|---|---|
| 1 | Check that no measurement is running (`docker ps`; no `python` in Task Manager) and that the lane-first switch is off: `. .\evaluation\question_factory\Run-Arms.ps1` then `Show-Stack` (`FORENSIC_GOVERNED_SQL true`, `FORENSIC_GOVERNED_SQL_FIRST false`) | a running measurement shares the one model and slows every answer |
| 2 | The stack is up: `nexusai-forensic-records-api-1`, `-worker-1`, `-postgres-1`, `-nats-1` and the model container `nexusai-api-1-prev-gbnf` all `Up` | |
| 3 | **Warm the model**: ask 4 or 5 questions per case before the audience arrives (section 6, questions 1 to 5). The first lane question after a restart took 323 s once; warm lane answers take 9 to 20 s | cold prompt cache |
| 4 | Laptop plugged in, awake, heavy apps closed (the model container uses about 4.3 GB; free RAM is tight) | |
| 5 | The workspace UI, if you show it: from the **main** checkout, `npm --prefix apps/investigation-workspace run dev` (it listens on `http://127.0.0.1:4181`). Check that it contains the white-page fix (commit `eb70a2b7`); without it the Ask page can go blank after an abstention | |
| 6 | Keep secrets off the screen: never open `.env.forensic-runtime.local`, never print the API key. `Ask-Case` reads the key from the container itself and does not print it | public repo, shared screen |
| 7 | Confirm the data is fine to show (the identifiers in the cases look synthetic, for example `PK-LHR-SYN-001`) | |

## 3. Three ways to ask a question

* **The workspace UI** (`http://127.0.0.1:4181`, the Ask page of a case): the answer reads as an answer, with its evidence. Today it does **not yet** show the query and the checked conditions (the decision record promises that); show them with the second way.
* **`Ask-Case` (PowerShell), the one to use for the technical part.** It prints the path, the lane header, the answer, **the SQL the model wrote**, and the notes that say what was checked and on which clock:
  ```powershell
  cd <your NexusAI-localwork checkout, the lane-arms branch>
  . .\evaluation\question_factory\Run-Arms.ps1
  $script:AskedSecrets['LOCALAI_API_KEY'] = $true
  Ask-Case -Collection nexusai-multimodal-product-acceptance "How many CDR records were there at night?"
  Ask-Case -Collection nexusai-forensic-demo "How many calls were there in June 2026?"
  ```
* **`factory.py explain`**, to show an answer next to its independent answer key (a SQL count computed straight from the raw data, not through the product): `python factory.py explain --questions questions-multimodal-clock.json --id CDR-night-01` (needs `FORENSIC_RECORDS_API_KEY` in the environment; the question files are local and gitignored).

The two cases: `nexusai-forensic-demo` (the demo case) and `nexusai-multimodal-product-acceptance` (the main working case: structured records plus audio, video, plate images and faces).

## 4. What it can answer

Evidence families (from the curated semantic layer): call records (CDR), internet sessions (IPDR), vehicle sightings (ANPR), access logs, financial transactions, subscribers (registered numbers), cell towers. It also has views over derived media metadata (OCR confidence, face-candidate counts, plate groups, audio segment timing, sampled video frames, image technical metadata), by their curated non-text fields only.

Phrasing the layer understands: "call records", "calls", "CDR records"; "internet sessions", "IPDR records", "data sessions"; "vehicle sightings", "camera sightings", "ANPR records"; "access log entries", "log entries", "web requests"; "transactions", "payments", "financial transactions"; "subscribers", "registered numbers"; "towers", "cell towers".

| Kind of question | Example that was measured | Answer |
|---|---|---|
| Count everything | How many camera sightings are there in this case? | 1,057 |
| Count with a value or an identifier | How many call records are there where the phone number is 923451112233? | 807 |
| "Linked to" an identifier | How many IPDR records are linked to 172.16.4.22? (demo case) | 497 |
| Honest absence (only after searching every identifier field) | How many calls does 923097431083 have? | "No record in this case contains 923097431083: I searched every identifier field of the structured evidence…" |
| Time of day | How many CDR records were there at night? | 1,283 (multimodal case), 1,768 (demo case); night is 00:00 to 05:59 on the case clock |
| Earliest and latest | What is the earliest transaction time in the transactions? | 2026-07-11 00:42:01 PKT |
| A date range | How many registered numbers have a activation date between 2023-01-07 and 2024-08-21? | 2 |
| Average, total, smallest, largest | What is the average certainty of the camera sightings? | 0.8892; "What is the smallest position among the towers?" gives 24.8138 |
| Most common value | Which account status appears most often in the subscriber records? | active (4) |
| A month ("in June 2026") | How many calls were there in June 2026? (demo case) | 2,463 |

Every time it shows is on one **case clock** (Asia/Karachi, shown as PKT) and the answer says so. UTC stays the stored instant.

## 5. What it will not do, and what it does instead

| Question | What happens | Say this |
|---|---|---|
| A **person's name** ("How many camera sightings did Mr Qureshi have?") | The lane abstains: "I could not tie "Mr Qureshi" to any field: these records hold numbers, plates, addresses and codes, not personal names." **Today only for ANPR and transactions.** For call records, internet sessions and access logs the **older path still answers with a total** (wrong) | "It abstains where the lane runs first; where the older path runs first it can still give a total. That is exactly what the lane-first measurement is about." Do not ask CDR, IPDR or access-log name questions live unless you mean to show this |
| A question spanning two families (a join) | Declined: the older path continues | "No join is declared and verified; it declines on purpose." |
| OCR text, transcripts, documents | The older retrieval path, not the lane | "Text evidence is the next phase, with its own sensitivity rules." |
| Greetings, concepts, help | The front door, off by default | |
| Questions the layer has no field for | Abstains and lists the fields it does have | |

## 6. Live demo script (about 25 minutes)

1. **The problem (2 min).** Show the baseline: of 82 unseen questions the original system answered 34 right, 23 confidently wrong, 25 abstained.
2. **The architecture (3 min).** One question in, one of three outcomes out; the model writes a query, the server checks and runs it (the flow in section 1; the same picture as a diagram is in the chat that produced this guide).
3. **Live questions (10 min), multimodal case, in this order** (all were answered by the lane and judged correct in every recent run):
   1. How many CDR records were there at night? Expect `1,283` and the notes: the time condition was applied to the event time; "A time of day is read on the case clock (Asia/Karachi, UTC+05:00); 'night' was taken as 00:00 to 05:59." **Point at the SQL**, then at the note.
   2. How many camera sightings were there at night? Expect `176`. Say: 3 plate reads derived from video frames have no time of their own and are never counted as night sightings.
   3. What is the earliest transaction time in the transactions? Expect `2026-07-11 00:42:01 PKT` (in UTC it is the 10th: the clock is stated, not hidden).
   4. How many call records are there where the phone number is 923451112233? Expect `807`.
   5. How many calls does 923097431083 have? Expect the honest absence text: it searched every identifier field before saying none.
   6. How many registered numbers have a activation date between 2023-01-07 and 2024-08-21? Expect `2`.
   7. How many camera sightings did Mr Qureshi have? Expect the abstention: no number, a reason, and what to change.
4. **The evidence (6 min).** Show the tables in section 7 from the pre-registration; stress that the gates were written before the runs and that two misses were reported, not hidden.
5. **Limits and next steps (4 min).** Section 5, then the phases: lane first on every question (the stage 2 decision), text retrieval, retiring the older keyword path, repairing the commit gate.

## 7. The evidence (all from `PREREGISTRATION.md`)

| Measurement | Result |
|---|---|
| Original system, 82 unseen questions (2026-10-05) | 34 correct, 23 confidently wrong, 25 abstained |
| Today (older path first, "stage 1"), multimodal case, 79 questions | control 34 correct, 21 wrong; **now 57 correct, 19 wrong** (all 19 are the older path's) |
| Today, demo case unseen set, 82 questions | control 42 correct, 18 wrong; **now 61 correct, 18 wrong** |
| Answers the lane itself gave in those runs | 23 of 23 and 19 of 19 correct |
| Lane first on the 42 time questions (run 13, part B) | 41 correct, 1 wrong, against 28 correct and 13 wrong with the older path first |
| Safety | the validator rejects 100+ hostile queries and accepts none; read-only transaction, timeouts, cost ceiling, row cap, server-built case scope; the model sees no case data |
| Process | architecture decided twice and re-researched; unseen questions with independent SQL answer keys; every gate written before the run; no prompt wording tuned after a failed gate |

Phase 1 (lane first on every question, on the 103-question corpus and on the 38-question pre-flight) has run (2026-10-08; pre-registration, "Phase 1 results"). Lane first, 161 unseen questions: **146 correct (90.7%), 4 wrong (2.5%), 11 abstained**, against 118 correct and 37 wrong in stage 1 (the rows above), with no right answer lost. The corpus did not pass: lane first answers two questions wrongly that stage 1 answers right ("incoming versus outgoing" given as one total; "who did X contact most" answered with X itself) and abstains on one text search. So the product stays in stage 1 for now; a fix round (run 14) and a repeat of Phase 1 come next, and only the owner flips the default. For the demo: show stage 1 as shipped, and present lane first as the measured next step.

## 8. Honest answers to likely questions

* **"Is it always right?"** No. When the lane answers it has been right in 23 of 23 and 19 of 19 recent answers, and when it cannot be sure it says why. The remaining wrong answers (19 of 79 and 18 of 82 questions today, 22 to 24%) all come from the older keyword path, which still answers first; making the lane first is the measured next step, and the owner decides it. Measured lane first (2026-10-08): 4 wrong of 161 questions (2.5%), plus two more wrong answers on the hand-written corpus (a comparison given as one total; a contact question answered with the number asked about) that the fix round closes before it can become the default.
* **"Why not just let the AI answer?"** It would invent numbers. Here the model only writes a query; the number comes from the database and the query is shown.
* **"What if the model writes something dangerous?"** One `SELECT` over server-built views only, checked by a parse-tree allowlist, run read-only inside the case's scope, with timeouts and a cost ceiling. 100+ hostile queries are rejected in the tests.
* **"How fast is it?"** Warm lane answers take about 9 to 20 s on this CPU-only laptop; the first question after a restart can take minutes; the older path answers in a few seconds.
* **"Can it answer anything?"** It answers structured questions about the seven evidence families; names of people and joins across families it declines or abstains on, and text evidence is a later phase.
* **"How do you know?"** A question factory generates unseen questions with an independent SQL answer key; every run is compared with a control; all numbers are in the pre-registration.
* **"Why PKT and not UTC?"** The stored time is UTC, and "night" or "June" must mean the clock of the place the case is about; the answer states the clock.

## 9. If something goes wrong live

* The page goes blank after an abstention: reload; the fix is commit `eb70a2b7`, to be merged into the branch the UI runs from.
* A question takes minutes: the model is cold or something else is using it; say so and move to the next question.
* An answer looks wrong: say it is measured, show the key with `factory.py explain`, and state that the older path answers first today; do not improvise a cause.
* To put the lane-first switch back to the shipped state: `Enable-LaneStage1`.
