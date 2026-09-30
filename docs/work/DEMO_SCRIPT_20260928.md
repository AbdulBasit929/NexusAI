# NexusAI — demo script, 2026-09-28

**Every question in this script was measured on the posture that is running now**
(`reports/goal-taxonomy-20260927/control`, 103 questions: 67 CORRECT, 28 abstentions, 4 wrong).
Nothing here is a claim written from memory. The four wrong answers are named in §5 so they are
not discovered in front of an audience.

---

## 1. Start it

Two processes. The API is already running; only the workspace needs starting.

```bash
docker ps --filter name=nexusai-forensic-records-api --format "{{.Names}} {{.Status}}"
```

```bash
npm --prefix apps/investigation-workspace run dev
```

Then open **http://127.0.0.1:4181**.

The dev server proxies `/api` to the forensic API on `localhost:8091` and attaches the API key
server-side. The browser never holds a credential. If every panel reads "not connected", the key
did not load — check `.env.forensic-runtime.local` is present at the repo root.

**Two cases are configured**, and they are real retained collections, not fixtures:

| Case | Contents |
|---|---|
| `nexusai-forensic-demo` | 12,912 structured records — CDR, IPDR, access logs, ANPR, towers, subscribers, transactions |
| `nexusai-multimodal-product-acceptance` | 9,580 records + 43 evidence items — images, audio, video, documents, and the model observations derived from them |

Start on **Cases → nexusai-multimodal-product-acceptance → Overview**: 43 evidence items,
42 ready, 10,168 accepted rows, broken out by family. Then **Investigate** to ask.

---

## 2. The one sentence to open with

> Every number in this product comes from SQL. The language model chooses a plan from a fixed
> vocabulary and then narrates a result it did not compute. It cannot invent a number, because it
> never produces one.

That is the architecture, and §4 is how you show it rather than assert it.

---

## 2b. READ THIS BEFORE LETTING ANYONE TYPE THEIR OWN QUESTION

**Free-form question answering is not finished, and its failure mode is not always an
abstention.** Measured 2026-09-27 on fourteen questions written fresh, in ordinary words, none of
them from any suite: **zero produced a correct, fully stated answer.** Six refused honestly. The
rest are the problem:

| Typed question | What came back | What is wrong |
|---|---|---|
| *"Show me all the calls that lasted longer than ten minutes"* | "20 CDR records matched this question." | The duration filter was **dropped**. Those are just the first 20 rows. |
| *"Do any subscribers share the same handset?"* | "11 subscriber records matched this question." | That is simply every subscriber. The question was not answered. |
| *"Is there any link between the plate sightings and the call records?"* | "There are 750 ANPR sightings in this case." | A count, in place of a correlation nobody computed. |

The plans behind all three carry **`"filters": []`** — the constraint never reached the SQL. This
is the dropped-filter defect recorded in `NEXUSAI_CONTINUATION.md` §7; the guard that catches it
(`FORENSIC_BOOLEAN_RESTRICTION`) covers boolean fields on **derived** plans only, and these are
structured plans with numeric and relational constraints.

**The tell, and it is reliable.** When a constraint really binds, the answer *repeats it back*:

> *"How many times was plate LHR-2026 seen?"* → **"There are 87 ANPR sightings involving
> LHR-2026 in this case."** — plan carries `anpr.plate_number EQ LHR-2026`.

When the answer says **"N records matched this question"** and names no constraint, the constraint
was not applied. Say so in the room rather than reading the number out.

**Recommendation for tomorrow: drive the demo from §3.** If someone wants to type their own, invite
it on *counting and breakdown* questions, which are the shapes that work, and read the sentence
before reading the number.

**Also not built: conversational handling.** *"hello"*, *"What can this system do?"* and *"What is a
CDR?"* all return a forensic error or a blank definition. There is no greeting, no capability
answer and no concept explanation — that work (UX §9 A) has plumbing but no content. Do not open
the demo by typing a greeting.

---

## 3. The demo, by data type

Ask these in the **Investigate** tab of the case named in each row. Every one of these scored
CORRECT on the running posture.

<!-- GENERATED FROM reports/goal-taxonomy-20260927/control — do not hand-edit -->

### 1. Structured records - the everyday work

| Ask | Case | Expected |
|---|---|---|
| How many CDR records do we have in this case? | forensic-demo | 8642 |
| What's the total number of call records? | forensic-demo | 8642 |
| count the cdrs | forensic-demo | 8642 |
| Show the call type breakdown | forensic-demo | Data session, 5863, SMS, 1108 |
| How many calls of each type are there? | forensic-demo | Data session, 5863, VoLTE call, 739 |
| Which phone number made the most calls? | forensic-demo | 923461678183 |
| How many unique phone numbers appear as callers in the CDRs? | forensic-demo | 10 |
| What date range do the CDR records cover? | forensic-demo | 2026-04-01, 2026-08-10 |
| Which cell site handled the most calls? | forensic-demo | 149631808 |
| How many incoming versus outgoing calls are there? | forensic-demo | Incoming, 2906, Outgoing, 2592 |
| How many call records are from August 2026? | forensic-demo | 2 |
| What was the longest call? | forensic-demo | 1798 |
| How many CDR records came from each source file? | forensic-demo | seed_cdr_large.csv, 5000, 923461678183.csv, 3634 |
| How many different handsets show up in the call records? | forensic-demo | 4997 |
| How many VoLTE calls are there? | forensic-demo | 739 |
| How many incoming calls are in the CDRs? | forensic-demo | 2906 |
| What is the largest network volume on any call record? | forensic-demo | 1730127963 |
| How many internet sessions were captured? | forensic-demo | 2500 |
| How many IPDR sessions are there? | forensic-demo | 2500 |
| Which domain was accessed most often? | forensic-demo | chat.example.test |
| Break down IPDR sessions by protocol | forensic-demo | HTTPS, 644, DNS, 613 |
| What is the total number of bytes transferred in IPDR sessions? | forensic-demo | 9764124834 |
| Which subscriber has the most internet sessions? | forensic-demo | 923001110002 |
| How many cell towers are in this case? | forensic-demo | 5 |
| How many cell towers are in the case? | forensic-demo | 5 |
| How many subscribers are currently active? | forensic-demo | 6 |
| How many subscribers are there? | forensic-demo | 11 |
| How many subscribers are active? | forensic-demo | 6 |

### 2. ANPR from structured sightings

| Ask | Case | Expected |
|---|---|---|
| How many ANPR sightings are there in total? | forensic-demo | 750 |
| How many times was plate LHR-2026 seen? | forensic-demo | 87 |
| Which camera recorded the most sightings? | forensic-demo | CAM-12 |
| How many distinct license plates were captured? | forensic-demo | 6 |
| Which vehicle was seen most often? | forensic-demo | ABC-123 |
| How many sightings involve plate QQQ-9999? | forensic-demo | 0 |
| How many ANPR sightings are in this case? | multimodal | 1057 |

### 3. Access logs and financial records

| Ask | Case | Expected |
|---|---|---|
| How many access log entries do we have? | forensic-demo | 1000 |
| Show the breakdown of HTTP status codes in the access logs | forensic-demo | 200, 697, 500, 46 |
| Which IP address made the most requests? | forensic-demo | 10.20.1.7 |
| How many requests failed with a server error (status 500)? | forensic-demo | 46 |
| How many server errors are in the access log? | forensic-demo | 46 |
| How many financial transactions were recorded? | forensic-demo | 4 |
| What is the total transaction amount? | forensic-demo | 129700 |
| What was the largest transaction? | forensic-demo | 75000 |
| Which account has the most transactions? | forensic-demo | ACCT-778812 |

### 4. Documents - targeted search

| Ask | Case | Expected |
|---|---|---|
| Which document mentions contact number 03001234567? | multimodal | nexusai-multimodal-acceptance-brief.pdf |
| Find the exact phrase "Scanned-only PDF OCR" in the documents | multimodal | Scanned-only PDF OCR |
| Search documents for the phrase "quantum encryption key" | multimodal | **abstains** - states nothing (this is the pass) |

### 5. Images and OCR

| Ask | Case | Expected |
|---|---|---|
| Find OCR text mentioning Investigation Workspace | multimodal | printed-english.png |
| How many text regions were read from the images? | multimodal | 309 |
| How many image fingerprints were computed? | multimodal | 22 |

### 6. Audio - transcripts and segments

| Ask | Case | Expected |
|---|---|---|
| Search the audio transcripts for Japanese cuisine | multimodal | fleurs-en_us-validation-row-01.wav |
| Which recording mentions coconut sugar and at what time? | multimodal | coconut sugar, 11.28 |
| Is the number 03001234567 mentioned in any audio? | multimodal | urdu-english-identifier.wav |
| How many audio segments were transcribed? | multimodal | 11 |

### 7. Video and ANPR read from media

| Ask | Case | Expected |
|---|---|---|
| Which plate was visible longest in the video? | multimodal | **abstains** - states nothing (this is the pass) |
| How many plate groups were tracked across the video frames? | multimodal | 24 |
| How many plate reads match QQQ-0000? | multimodal | **abstains** - states nothing (this is the pass) |
| How many plate reads were produced from the images? | multimodal | 307 |
| What is the average OCR confidence of the plate reads? | multimodal | 0.955573 |

### 8. Faces - detected, never identified

| Ask | Case | Expected |
|---|---|---|
| Who are the people in the images? | multimodal | **abstains** - states nothing (this is the pass) |
| How many faces were detected in the evidence? | multimodal | 20 |
| What is the average face detection confidence? | multimodal | 0.78958 |

### 9. Cross-family

| Ask | Case | Expected |
|---|---|---|
| Explain the call type breakdown | forensic-demo | Data session, 5863 |

### 10. What it REFUSES to answer

| Ask | Case | Expected |
|---|---|---|
| How many calls did 03999999999 make? | forensic-demo | **abstains** - states nothing (this is the pass) |
| Where was plate ZZZ-0000 seen? | forensic-demo | **abstains** - states nothing (this is the pass) |
| What is the suspect's blood type? | forensic-demo | **abstains** - states nothing (this is the pass) |
| How many emails are in this case? | forensic-demo | **abstains** - states nothing (this is the pass) |


---

## 4. The three things worth slowing down for

**a. Faces are detected, never identified.** Ask *"How many faces were detected in the evidence?"* →
**20**. Then ask *"Who are the people in the images?"* → it refuses, **0 sources**. The system
counts faces and will not name them. Most demos show the first question. Show both.

**b. A model's reading is not a camera's sighting.** In the multimodal case:

- *"How many ANPR sightings are in this case?"* → **1,057** — the ingested ANPR record family.
- *"How many plate reads were produced from the images?"* → **307** — model observations, held in
  a separate table under a versioned contract (`forensics.anpr-observation/v1`).

**Be precise about what these two numbers are**, because they are not two disjoint piles of events.
The 1,057 ingested rows are 750 from `seed_anpr.csv` plus 299 from `video-v3.mp4` plus 8 from
still images. The 307 model observations are the model's own record of reading those same pixels.
The same read can therefore appear in both tables — **once as an ingested record, once as an
inference** — and the product's job is to never let the second be reported as the first.

So the discipline on show is not arithmetic, it is which contract answers the question. Ask for
*sightings* and you get 1,057, from the ingested family. Answering **307** would be reporting model
guesses as camera sightings, and that is the single failure this product exists to prevent — it has
its own standing probe (`P1-PROVENANCE-SIGHTINGS`). A plan that tries to span both tables is
refused outright.

Open the citations and read the marker on each source. There are three, and they are the whole
argument in one glyph:

| Marker | Label | Means |
|---|---|---|
| ● | **Source record** | an ingested row — what was collected |
| ◐ | **Candidate observation** | a model's inference from pixels or audio |
| ○ | **Low-confidence observation** | an inference the model itself is unsure of |

The OCR answer (*"How many text regions were read from the images?"*) shows all three across its
nine sources. That distinction is the difference between evidence and inference, and it is enforced
in the query compiler, not in the UI.

**c. It says no.** *"What is the suspect's blood type?"*, *"How many emails are in this case?"*,
*"Where was plate ZZZ-0000 seen?"* — all refuse. An abstention is a success here. A confident wrong
answer never is.

Also worth one click: on any answer, expand **Citations** and open a source. The link carries the
exact region — for video, a bounding box and a frame — so a finding can be checked against the
original evidence rather than trusted.

---

## 5. Known limits — say these before you are asked

Four questions out of 103 answer wrongly on this posture. Three are unhelpful-but-safe: they state
plainly that they make no claim about the case. One is genuinely misleading. None fabricate
evidence.

| Ask | What happens | Severity |
|---|---|---|
| *"Where was 923001110001 seen according to the call records?"* | Answers **"There are 447 CDR records involving 923001110001"** — a count, where the honest answer is that call records do not establish location | **Misleading — avoid asking** |
| *"Were any of the transcribed audio segments taken from video?"* | Answers *"No transcript segment intersects the requested source-time range"* — the wrong template, and it reads as a "no" | **Misleading — avoid asking** |
| *"What is the latest end offset in the audio transcripts?"* | *"This response makes no statement about the current case"* | Safe, unhelpful |
| *"What does the audio transcript say?"* | Same. **Deliberate** — transcript text is curated PII and is withheld until free-text masking ships | Safe, by design |

Two more answer partially: *"Who did 923001110001 contact most frequently?"* and *"When was
LHR-2026 first and last seen?"* compute the right rows but do not state the answer in the summary
sentence. The data grid below the answer is correct.

**Two cosmetic defects to expect, both on media answers.** In each case the number, the sources and
the citations are correct — only the wording around them is off.

- **The noun is sometimes generic.** *"How many plate groups were tracked across the video
  frames?"* returns the correct **24** but says "24 ANPR sightings". *"How many text regions were
  read from the images?"* returns the correct **309** but says "309 records". The count and the
  citation are right; the noun is a server-side label.
- **A raw measure alias can leak into the result grid.** The OCR answer's column header reads
  **`m1`** instead of a field name.

Neither is fixed for this demo, because changing narration requires a full three-suite measurement
and an unmeasured change the night before a demo is how the 67 working answers get broken. Say the
number out loud and point at the citations; that is where the substance is.

---

## 6. What is not built, if asked

- **No login.** There is no identity system — no users, roles or membership. The dev proxy holds one
  privileged key, which is why it binds to `127.0.0.1` and is a review convenience, not a
  deployment.
- **No case entity.** A case *is* a collection. Cases are configured, not created in the product;
  "Open a new case" uploads evidence under a new collection name.
- **No cross-case dashboard.** The Dashboard says so rather than showing an invented number — that
  refusal is deliberate and is the same principle as §4c.
- **28 of 103 questions abstain.** Most are honesty probes that are supposed to. The rest are real
  capability gaps, tracked in `reports/OPEN_ITEMS.md`.
