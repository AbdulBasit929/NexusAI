# NexusAI — capability log

**What the product can do that it could not do before, and the measurement that proves each line.**

One entry per shipped capability. Every entry carries the evidence that backs it. A capability with
no measurement does not get an entry — that rule is what makes this document worth showing to
somebody who was not in the room.

Read top-down for the newest work.

---

## 2026-09-27 · The analyst workspace runs on live evidence

**Status: SHIPPED · verified end to end in a browser against the running API**

An analyst can open a case, ask a question in a thread, and read the answer with its citations,
provenance markers and coverage limits — against real retained evidence, not fixtures.

| Verified live | Result |
|---|---|
| `How many CDR records do we have in this case?` | **8,642**, 2 sources cited |
| `How many ANPR sightings are in this case?` | **1,057** |
| `How many plate groups were tracked across the video frames?` | **24**, marked *Candidate observation* |
| `How many text regions were read from the images?` | **309**, 9 sources, three provenance tiers |
| `Which recording mentions coconut sugar and at what time?` | filename + **11.28s** + quoted phrase + stated coverage limit |
| `Who are the people in the images?` | **refuses — 0 sources** |

**67 questions measured CORRECT** across every data type: structured (CDR, IPDR, ANPR, access log,
transaction, subscriber, tower), audio, images, OCR, video, ANPR-from-media, faces, documents.
Evidence: `reports/goal-taxonomy-20260927/control/`.

**Conversation, not a single answer.** The Investigate surface previously held one answer and
replaced it on every new question. It is now a thread: questions and answers persist, each with its
own result grid, citations and follow-ups.

**Every number on screen is traceable.** Dashboard and case cards read live
`/collections/status` — evidence counts, readiness, structured rows, family composition. Nothing on
screen is computed by the interface.

### Two defects found and closed in the same pass

- **Case scoping** — a runtime-config value pinned *every* case to one collection. A CDR count
  inside `nexusai-forensic-demo` answered **5,000** (the other collection's) instead of **8,642**,
  while the page still displayed the correct case name. Caught because the number did not match a
  known anchor.
- **Evidence upload returned 403** — *"not authorized to add evidence to this case"*. The workspace
  sent `case_id` in the form but no `X-Forensic-Case-ID` header, so the authorised scope was empty
  while the supplied one was not. **The server was right**; the client now declares the header.
  Pinned by `api/forensic_records/upload_case_scope_test.go`. **No role was widened and no check
  was relaxed.**

---

## 2026-09-27 · A stated condition can no longer be silently dropped  *(A1a)*

**Status: SHIPPED, default ON · measured across two arms · evidence:
`reports/constraint-obligations-20260927/A1A_RESULT.md`**

**Measured result:**

| Threshold | Outcome |
|---|---|
| Arm A reproduces the shipped posture | **exact** — 67/28/4/2/1/1 |
| Corpus movement with the guard on | **0 of 103**, as the census predicted |
| The defect question | **withheld**, naming its condition, `reason_code=condition_not_applied` |
| Control question | still **8,642** |

    asked    "Show me all the calls that lasted longer than ten minutes"
    before   "20 CDR records matched this question."     plan filters: []
    after    "I did not apply the condition "longer than ten minutes" to a curated field,
              so this result counts every record of its kind rather than only those you
              asked for. Name the field the condition applies to and I will compute it."

Shipped **default ON** — the documented exception to the project's switch discipline, because here
the off state is the unsafe one. Rollback image tagged
`rollback-before-constraint-20260927`.

**The capability:** when an analyst states a numeric condition — *"calls longer than ten minutes"*,
*"more than 1000 bytes"*, *"at least 30 seconds"* — and the executed plan compares against nothing,
the product **withholds the answer and names the condition back** instead of stating a count of
everything.

**The defect it closes**, measured 2026-09-27 on freely-typed questions
(`reports/adhoc-runtime-20260927/`):

    asked     "Show me all the calls that lasted longer than ten minutes"
    answered  "20 CDR records matched this question."
    plan      "filters": []          <- the duration condition never reached the SQL

Those were the first twenty rows, presented as the answer to a filtered question.

**Why it is safe:** censused across all three corpora **before** being wired —
**0 of 103 questions captured**, so it cannot disturb a single existing answer. It exists for
freely-typed questions, which is exactly where the defect lives and exactly what the corpora do not
contain.

**What it deliberately does not do:** it adds no capability. Parsing the condition into a real SQL
filter is the next item (A1b) and is measured separately. It covers comparisons against a number
only; relational conditions such as *"do any subscribers share the same handset?"* are out of scope
and recorded as still-uncovered rather than half-handled.

**Engineering note worth stating:** the guard sits at `preExecutionWithhold`, where every planner
converges before execution — not inside the compiler. There are at least two producers of a typed
plan, and a check inside one of them would have been silently absent for questions that took the
other route.

Threshold pre-registered at `reports/constraint-obligations-20260927/THRESHOLD_PREREGISTRATION.md`.

---

## 2026-09-27 · Numeric conditions now compile to real SQL filters  *(A1b)*

**Status: SHIPPED, enabled · two measured rounds · evidence:
`reports/range-filters-20260927/A1B_RESULT.md`**

**The capability:** an analyst states a bound and names a curated field, and the query carries a
real `GT/GTE/LT/LTE` filter into SQL.

Verified against an **independent SQL oracle** — derived by mirroring the executor's own
expression, never from the product's own output:

| Question | Oracle | Measured |
|---|---|---|
| `How many transactions have an amount above 50000?` | **1** of 4 | **1** |
| `How many IPDR sessions have bytes above 1000000?` | **2,189** of 2,500 | **2,189** |

Corpus movement: **0 of 103**. Arm A reproduced the shipped posture exactly.

**It took two rounds, and the first one was held back deliberately.** Round one proved the filter
but the headline read *"There is 1 transaction in this case"* while the case holds four — a true
number inside a sentence that claimed more than was computed. That is the exact defect this product
exists to refuse, so it was not shipped.

Round two (A1b.1) made the headline name the bound, as it already did for a target, and
re-measured — corpus still flat, oracle still matched:

    before   "There is 1 transaction in this case."
    after    "There is 1 transaction with amount above 50000 in this case."

    before   "There are 2,189 IPDR sessions in this case."
    after    "There are 2,189 IPDR sessions with bytes above 1000000 in this case."

**Also settled by this work:** *"calls longer than ten minutes"* cannot be answered and must keep
abstaining. No range-filterable field in the curated layer declares a `unit`, and call duration is
a derived metric (`timestamp_diff_seconds(...)`) with no filterable column. Converting "ten
minutes" to 600 would be an invented number. Duration filtering is its own work item.

---

## 2026-09-27 · Compiler-first routing — tried, measured, reverted  *(A2)*

**Status: MEASURED AND REVERTED · nothing regressed · evidence:
`reports/compiler-first-20260927/A2_RESULT.md`**

**The hypothesis:** questions the keyword ladder does not recognise reach the clarification having
never been planned, so running the compiler there would make them answerable.

**The measurement killed it.** With the change switched **off**, an unrecognised question already
carries ~3.1 s of real compiler latency and 66 ranked candidates — the compiler was always running,
via a path whose gate does not block in this deployment. The change added a **second, redundant
compile** for no change in outcome, so it was removed.

**Why the hypothesis looked right:** a pre-build trace reported no planner audit on those
questions. That field is `json:"-"` and is never serialised, so the probe could not have seen it in
either arm. It measured nothing and was read as evidence of absence — the sixteenth instrument
defect on record here, and the second in one day.

**What it bought.** The routing explanation is now dead with evidence, and the real blocker is
located exactly: the compiler runs, then refuses at its own structured-competence guard
(`semanticQuestionFamily == "" && extractCanonicalRecordType == ""`). That guard is correct — it
exists because a typed plan once answered *"which image says Stay Positive Work Hard"* from
`canonical_records`. Making those questions answerable is the A3 capability work, not a routing
change.

---

## 2026-09-27 · Identifier binding — built and clean; its routing reverted  *(A3.3)*

**Status: binding MEASURED CLEAN, held off · routing MEASURED AND REVERTED · evidence:
`reports/identifier-binding-20260927/A3_3_RESULT.md`**

**What was built:** the deterministic compiler can now bind a named identifier — a tower site
code, a plate — to the **one** curated field that holds it, verified over the full data with the
executor's own expression. It refuses to bind when the value sits in two fields (a phone number is
both caller and callee), when it exists nowhere, and when a "who" question targets an identity the
product withholds as PII. Measured: **0 of 103 corpus questions moved.**

**What was tried and reverted:** letting arbitration run that compiler *before* the LLM rescue that
already exists. It cost three correct answers, one of them to a confident wrong
(*"4 records"* where the answer is 2). The LLM rescue was already answering them; the deterministic
compiler is weaker on those shapes, and going first replaced good plans with bad ones. Reverted
within minutes; posture restored and asserted before analysis began.

**Why this entry matters:** the safety argument written before the run — *"replacing a
clarification cannot make a right answer wrong"* — was **false**, because the baseline was not the
clarification but the next rescuer in line. The pre-registered outcome thresholds caught it anyway.
That is the case for writing thresholds against measured outcomes rather than against reasoning.

**What is now exactly located:** routing as a *last* resort after the LLM rescue, plus a lookup
sentence that states the attributes it found (*"1 tower record matched"* is not an answer to
*"where is it"*).

---

## 2026-09-27 · A misspelt family can no longer be reported as "no records"

**Status: SHIPPED, default ON · evidence: `reports/record-type-validation-20260927/RESULT.md`**

**Found by testing the redesigned interface in a browser, not by any suite.** Selecting the Tower
scope and asking *"How many cell towers are in this case?"* answered **"There are no tower records
in this case"** — about a case holding five. The interface sent `tower`; the data calls the family
`tower_location`. The question suites never send a family scope, so they could not have caught it.

    before   "There are no tower records in this case."
    after    "There are 5 tower records in this case."

**Fixed twice, on purpose.** The interface now sends the right name. And the service now validates
any family name before using it — a known alias is resolved, and an unknown one is **refused**
("no scope was applied and nothing was counted") instead of being narrated as absence. So the same
mistake from any future client cannot produce a false "none". Corpus: **0 of 103 moved.**

---

## 2026-09-27 · Compiled family scope — tried, measured, reverted  *(CDR-14)*

**Status: MEASURED AND REVERTED · evidence: `reports/compiled-family-scope-20260927/RESULT.md`**

**The aim:** stop a compiled count from spanning every evidence type — *"How many call records are
from August 2026?"* had answered "4 records" (2 call records + 2 subscriber rows) instead of 2.

**What the measurement showed:** that defect only occurred on a routing path already reverted
earlier today. In every shipped path, the question is scoped correctly and answers **2**. The fix
instead broke a working answer — *"Which domain was accessed most often?"* went from correct to
"1 access log entry", because the word "accessed" misled the new scoping, which ran ahead of the
existing scoping that reads the question correctly.

**Reverted within minutes;** both questions re-verified correct against the running service.

**Why it belongs in this log:** it is the clearest case today for measuring before shipping. The
change had a live-database test that passed — but that test skipped the pipeline stage that already
handled the problem, so it proved a fix for a defect production did not have. Only the full-corpus
run exposed it.

---

## 2026-09-27 · Counts grouped by source file now compile  *(A3.1)*

**Status: SHIPPED, default ON · evidence: `reports/compiler-gaps-20260927/A3_1_A3_2_RESULT.md`**

**The capability:** *"How many CDR records came from each source file?"* — through the question
engine itself, without the keyword templates — now answers:

    "8,642 CDR records across 5 source file values.
     seed_cdr_large.csv: 5,000; 923461678183.csv: 3,634; ..."

Before, the engine read "source" + "records" as *"show me the raw rows"* and returned twenty rows.

**How it was proven:** a new measurement mode that turns the keyword templates off, so every
question travels the full engine path. That mode is now the progress meter for retiring the
templates: **64 correct today with templates off, against 67 with them on.** This change closes one
of the gaps. Shipped setting measured on its own: **0 of 103 moved**.

**What was tried alongside and held back:** a fix for *"Which cell site handled the most calls?"*.
Its first version would have misfiled eight document and media questions as camera sightings — the
pre-deploy check caught that. The narrowed version was safe but did not fix the question, because
the real cause turned out to be different. That cause is now located and is the next item.

---

## 2026-09-28 · "Which cell site handled the most calls?" answers without templates  *(CDR-12)*

**Status: SHIPPED, default ON · evidence: `reports/arbitration-supersedes-20260928/RESULT.md`**

**The capability:** through the question engine itself, with the keyword templates off:

    "The cell site with the highest count is 149631808 (1,114)."

Before, it asked *"Which target identifier should I analyze?"*, even though the right query had
already been built and checked.

**The cause, and why the fix is small:** the engine first considered a ready-made operation that
needs a target number. It then built a better query that needs none. But the target requirement of
the rejected operation was still being checked, so the engine asked for a number nothing would use.
The fix drops that stale check, and only for a query that actually replaced the operation. An
earlier plan would have changed which operation is chosen for five questions, two of them privacy
test questions. This fix changes exactly one.

**How it was proven:** four runs of all 103 questions, with the pass criteria written first.
Templates on: **0 of 103 moved.** Templates off: exactly CDR-12 moved, from a clarification to
correct. The privacy test questions were unchanged in every run.

**Progress toward retiring the templates:** with the templates off, **66 correct** (64 yesterday,
67 with them on).

---

## 2026-09-28 · Relationship and "where" questions no longer get a count for an answer  *(A1c, A1d)*

**Status: SHIPPED, default ON · evidence: `reports/relational-conditions-20260928/RESULT.md`**

**The capability:** a question asking whether records are related is withheld when the query would
only count one kind of record, and the relationship is named back:

    "Do any subscribers share the same handset?"
    before  "11 subscriber records matched this question."
    after   "I did not compute the relationship you asked about ("share the same handset")..."

A question opening with "where" can no longer be answered with a bare count: *"Where was
923001110001 seen according to the call records?"* used to say "447 CDR records". It now says it
could not compute the answer.

**How it was proven:** 103 questions: only H11 moved, from wrong to withheld. **Wrong answers in
the shipped system: 4 → 3.** Plus 16 dedicated questions, because none of the 103 ask about
relationships. All 6 misleading relationship answers are now withheld, and all 8 lookalike questions
("share of SMS", "Can you share a summary") are unchanged. The deployed build reproduced the measured
result exactly.

---

## 2026-09-28 · An answer never reports its own row count as a finding  *(S1)*

**Status: SHIPPED, default ON · evidence: `reports/rowcount-headline-20260928/RESULT.md`**

**The capability:**

    "Show me everything you have about plate ABC-123"
    before  "1 ANPR sighting matched this question."
    after   "There are 218 ANPR sightings involving ABC-123 in this case."

The query was right all along. The sentence reported the number of result rows (1) instead of the
count the query computed (218). Summaries now say what they contain rather than "14 records matched".

**How it was proven:** 103 questions: 0 moved. Of the 14 everyday questions, exactly the 2 targeted
ones changed. All 38 demo checks and all 16 relationship questions were identical.

---

## 2026-09-28 · Media answers name what they counted  *(S4)*

**Status: SHIPPED, default ON · evidence: `reports/derived-labels-20260928/RESULT.md`**

**The capability:** "How many plate reads were produced from the images?" now answers "307 **plate
reads**" instead of "307 ANPR sightings", which is what the case's 1,057 camera sightings are called.
Faces, image text regions, audio segments, video plate groups and image fingerprints are named the
same way.

**How it was proven:** exactly the 8 predicted answers changed their label, every number is
unchanged, 0 of 103 verdicts moved, and camera sightings keep their name.

---

## 2026-09-28 · "Nothing found" is only said when it was actually established  *(S2, S3)*

**Status: SHIPPED, default ON · evidence: `reports/absence-text-20260928/RESULT.md`**

**The capability:** *"Were any of the transcribed audio segments taken from video?"* used to answer
"No transcript segment intersects the requested source-time range". That was false: one segment did
come from video, and no time range was asked for. A keyword search that finds nothing is now reported
as exactly that, never as absence. A request for more detail now shows its question instead of a
"no events were found" sentence.

**How it was proven:** exactly the 2 predicted answers changed. The correct "phrase not found" answer
was untouched, and nothing else moved. **Wrong answers in the shipped system: 3 → 2.**

---

## 2026-09-28 · Searching images and documents for a plate number works  *(U2a)*

**Status: SHIPPED, default ON · evidence: `reports/evidence-search-first-20260928/RESULT.md`**

**The capability:** *"Find OCR text mentioning BCX-567"* now answers "DSC_1105.JPG contains the text
'BCX-567'". Before, the plate-shaped number sent the question to the camera-records search, and it was
refused. *"Search the case documents for mentions of plate MN1367"* now finds the PDF. A search aimed
at images, documents or transcripts now stays there.

**How it was proven:** exactly the predicted answers changed. One test question went from refused to
correct, and 3 demo checks now pass (36 of 38). Nothing else moved, and the plate-privacy tests still
refuse.

---

## 2026-09-29 · Result tables say what each column holds  *(A1)*

**Status: SHIPPED, default ON · evidence: `reports/column-labels-20260929/RESULT.md`**

**The capability:** result tables no longer show a column called "M1". They say "CDR records",
"Detected faces", "Total bytes transferred" or "Count of HTTP status code". Grouped fields use their
curated names ("Subscriber number", "Call direction"), and the internal "Metadata" column is now
"Source rows".

**How it was proven:** 54 answers relabelled with keys and values byte-identical and 0 answers changed.
All demo, everyday, relationship and plate checks were identical. Two slips were caught and fixed
before shipping: a broken acronym ("hTTP"), and a build that silently didn't run. Every build is now
checked by its image timestamp.

---

## 2026-09-29 · Find which image or video frame a plate was read in  *(U2b, search-only)*

**Status: SHIPPED, default ON · evidence: `reports/plate-read-search-20260928/RESULT.md`**

**The capability:** *"Which image shows plate MN1367?"* now answers "There is 1 plate read involving
MN1367 in this case. Read in: image-test-plate-test_plate.jpg. These are unreviewed model reads, not
camera sightings." *"Which video frames show plate AK64DMV?"* answers 14 reads in video-v3.mp4. When
nothing matches, the answer says the model may have misread the plate rather than claiming absence.

**The privacy rule (product-owner decision):** search-only. A plate the analyst types can be found.
Plates are never listed, grouped or counted, so *"Which plates were read from the videos?"* stays
refused. Tests confirm that no other use of the plate field gets past the query validator.

**How it was proven:** 8 of 8 plate questions matched the database, including 3 privacy questions that
disclosed no plate. The demo checks went from 36 to 38 of 38, and 0 of 103 test questions moved. One
wrong word in the no-match sentence ("ANPR sightings") was found after the checks passed, fixed and
re-verified before shipping.

---

## 2026-09-29 · Table headers read properly; transcripts are marked as model output  *(A1.1, A2)*

**Status: SHIPPED, default ON · evidence: `reports/a1-1-a2-20260929/RESULT.md`**

**The capability:** template tables no longer show shouted fragments of column names. "Call END TS"
now reads "Call end timestamp", "ROW Hash" reads "Row hash", and "Imei" reads "IMEI". Document,
image-text and transcript citations now carry their stored truth state, so the UI can mark a
speech-to-text segment or an OCR read as a model observation instead of "Source record".

**How it was proven:** 69 header cells in 8 answers were recased, and all 55 distinct changes were
read. Keys, values and answers were identical, and 0 of 103 questions moved. 254 fact-packet
citations match their source item, with 0 mismatches, and no structured record gained a truth state.
All demo, everyday, relationship and plate checks were identical. One consistency check fired on a
reference file measured before the A1 "HTTP" fix. It was traced to that fix, recorded, and still
fires on any other difference.

---

## 2026-09-29 · "Who did they contact most?" and "When was it first seen?" are answered in words  *(A3)*

**Status: SHIPPED, default ON · evidence: `reports/template-states-20260929/RESULT.md`**

**The capability:** *"Who did 923001110001 contact most frequently?"* now answers "923009998887 and
923009998883 are tied as the most frequent contacts of 923001110001, with 121 CDR records each."
Before, it said only that 8 contacts were ranked. *"When was LHR-2026 first and last seen?"* now
answers "first seen at 2026-07-14 06:00:00 UTC and last seen at 2026-07-19 07:22:00 UTC, across 87
ANPR sightings". Before, it described the machinery.

**How it was proven:** both questions moved from "computed but not stated" to correct, and both were
re-checked against the database with independent read-only queries. 0 of the other 101 questions
moved, and all tables were identical. A tie is always named in full, because naming one of two tied
contacts would be a choice the data does not make.

---

## 2026-09-29 · A count says what it counted  *(A4)*

**Status: SHIPPED, default ON · evidence: `reports/count-names-field-20260929/RESULT.md`**

**The capability:** *"How many unique phone numbers appear as callers?"* now answers "There are 10
distinct subscriber number values across CDR records". Before, it said "The count across CDR records
is 10." The analyst can now see that subscriber numbers were counted, not originating numbers. A count
of one field also no longer reads as a count of every record.

**How it was proven:** three answers now name their field, and 0 of 103 questions moved. The first build
passed every threshold but made one pre-flight sentence worse, which was found by reading the changed
texts. It was fixed, rebuilt and re-measured before shipping.

---

## 2026-09-28 · The server states its own configuration at startup  *(P3)*

**Status: SHIPPED (logging only, changes no answer)**

On start, the API now writes one line listing every behaviour switch and whether it is on or off
(today: 23 on, 8 off). It warns if a switch is missing or has an unreadable value. On 25 September
five settings ran switched off for five hours without anyone noticing. That is now visible in the
first line of the log. A test keeps the list in step with `docker-compose`. The deployed build passed
the same 38 demo checks as the measured one.

**Also recorded:** one full test-suite run failed under heavy load without printing which test. Three
captured re-runs passed. That's the second such occurrence (gap P10), still unexplained.

---

## How to read a status in this log

    SHIPPED                          measured, thresholds passed, running in the deployed posture
    BUILT AND CENSUSED               code and offline evidence complete; live measurement pending
    MEASURED AND REVERTED            it was tried, it failed its threshold, and it was withdrawn

**A reverted entry is kept, not deleted.** Knowing what was tried and refused is worth as much as
knowing what shipped — three changes on this project were reverted after measurement, and each one
would otherwise have been a confident wrong answer in front of a user.
