# Testing the new analyst workspace — what to click, and what you should see

Everything below was verified live on 2026-09-27 against the running API. Every number is one I
checked against the database or the API directly, not read off the screen.

## Start it

The API is already running. Start the interface:

```bash
npm --prefix apps/investigation-workspace run dev
```

Open **http://127.0.0.1:4181** in your own browser at full width. (The app's built-in preview pane
is too narrow to show the desktop layout properly.)

---

## 1. Dashboard — `/`

**What you should see:** a readiness table for both cases, with filter chips across the top
(*All · Needs review · Processing · Ready · No evidence · Unavailable*).

| Case | Total | Ready | Failed | Also shown |
|---|---|---|---|---|
| `nexusai-multimodal-product-acceptance` | 43 | 42 | 1 | 3 completed jobs missing a retained asset |
| `nexusai-forensic-demo` | 12 | 11 | 1 | — |

Both show **"Needs review"** because each has one failed evidence item. That is correct and honest —
a case with a failed item is not fully covered.

Below: *"Continue recent questions"*, labelled **"This browser only"**.

**What to show someone:** the "What this view covers" note says the service does not expose
assignment, severity or priority — so the dashboard does not invent them.

## 2. Evidence — `/cases/nexusai-multimodal-product-acceptance/evidence`

**What you should see:** *"Showing 1–25 of 43 evidence items · Page 1 of 2"*, with totals
**43 total · 42 ready · 0 queued · 0 processing · 1 failed · 199.6 MB**, and a searchable table.

**Try each family filter.** Every count below matches the database:

| Filter | Shows |
|---|---|
| All | 43 |
| Images | 23 |
| Audio | 7 |
| Video | 2 |
| Documents | 2 |
| CDR · IPDR · ANPR · Subscriber · Financial · Access log | 1 each |
| **Tower** | **1 — `tower_reference.csv`** |

The URL updates as you filter (`?family=audio`), so a filtered view can be shared.

> **Tower was broken until today.** It returned "No evidence items" because the interface used the
> wrong family name. Fixed and verified — see §4.

## 3. Case overview — `/cases/<case>/overview`

Breadcrumbs, the case's evidence counts and structured-row totals, data-quality notes, and the
evidence families with their row counts. An **"Ask a question"** button goes straight to Investigate.

## 4. Investigate — `/cases/<case>/investigate`

Questions and answers stay in a thread — asking a new question does not erase the previous answer.

**Questions that work and are worth showing** (verified today):

| Case | Ask | Answer |
|---|---|---|
| forensic-demo | How many CDR records do we have in this case? | **8,642**, 2 sources cited |
| forensic-demo | Which cell site handled the most calls? | **149631808** (1,114) |
| forensic-demo | How many transactions have an amount above 50000? | **1** transaction *with amount above 50000* |
| forensic-demo | How many IPDR sessions have bytes above 1000000? | **2,189** sessions *with bytes above 1000000* |
| forensic-demo | **Tower** scope → How many cell towers are in this case? | **5** tower records |
| multimodal | How many ANPR sightings are in this case? | **1,057** |
| multimodal | How many text regions were read from the images? | **309**, three provenance tiers |
| multimodal | Which recording mentions coconut sugar and at what time? | file + **11.28 s** + the quote |

**Questions that should REFUSE — and refusing is the correct answer:**

| Ask | What it does |
|---|---|
| Who are the people in the images? | refuses — faces are counted, never identified |
| Show me all the calls that lasted longer than ten minutes | refuses, **names the condition it could not apply** |
| What is the suspect's blood type? | refuses |

The "longer than ten minutes" refusal is new today. Before, it answered *"20 CDR records matched
this question"* — the first twenty rows, presented as filtered.

**How to read an answer:** the number in the sentence is a link to its source. Each citation carries
a marker — **●** source record · **◐** candidate observation · **○** low-confidence observation.

## 5. Known limits — say these before you're asked

- **Free-typed questions that aren't counts, breakdowns or simple filters often refuse.** That is
  the remaining engine work, and refusing is the safe outcome.
- **Relational questions** (*"Do any subscribers share the same handset?"*) are not yet guarded and
  can return an unfiltered count. Avoid in a demo.
- **There is no sign-in.** Settings says so explicitly.
- **Question history lives in this browser only**, and says so.
