# 03. Architecture: cases, cross-case, evidence, the Ask engine, history, structured families, reports and security

**Owner's request, 2026-11-03:** the best architecture, flow, pipeline and connectivity of the application's modules: case creation and the case itself, cross-case, evidence files and their best use, the Ask chatbot, history like a chat application, every structured data family and file type, and the runtime use of language models (no predefined templates or operations).

Marks as in file 01. Binding documents this file respects and does not override: `docs/architecture/RECONCILIATION_20260921.md` (section G), `docs/architecture/QUERY_ANSWERING_DECISION_20261005.md` (two lanes, one verifier), `docs/ux/NEXUSAI_PRODUCT_UX.md` (the contract), AGENTS.md ("three rules that override habit": add no operation templates; no model swaps outside WI-0's written decision rule; no embeddings for structural decisions such as family, group, measure or field role).

---

## 1. Principles

1. **The case is the unit of trust.** Access, audit, retention, time basis, languages and enabled capabilities (faces, translation) are properties of a case. Nothing crosses a case boundary except through the cross-case mechanism of section 4.
2. **Evidence is immutable; everything else is derived and versioned.** Originals are hashed at intake and never edited. Parsed records, typed events, media readings, entities and answers carry their derivation (model or rule, version, input hash) and can be recomputed.
3. **Every figure opens its source.** From an answer, a chart mark, a table cell, a transcript word or a face box, one click reaches the original row, page, second or frame (the contract, section 0 and 4).
4. **Four epistemic states, always visible, word and shape:** *observed* (in the records), *inferred* (by a stated rule), *candidate* (a lead, requires review), *confirmed* (an analyst, with name and time). A model's output is at best *candidate*.
5. **Runtime understanding, not templates.** A question is understood and answered at run time by a language model working over a reviewed schema, checked by code. What code may contain: the schema description, validators and obligations derived at run time from the question's own parse, executors, walls, evidence tracing. What it may not contain: fixed question-to-SQL or question-to-operation maps, keyword routing to canned answers (section 6.4).
6. **Offline and on-premises.** No evidence, crop, transcript or embedding reaches a third party. Public or synthetic data may use rented compute to train, never to process cases.
7. **Honest states over empty ones.** A missing capability, a running job, a withheld cell, a low-confidence reading are designed states with next actions, not blank panels.

## 2. The layers and who talks to whom

```
 Interface (React/Vite, :4181 old, :4182 new)  --HTTP/JSON-->  Read and Ask API (Go modular monolith)
                                                                 | cases | evidence | query (Ask engine) | analysis
                                                                 | media | conversations | identity | cross-case | reports | audit
        reads (read-only role)  <---------- PostgreSQL 16 + TimescaleDB + pgvector (RLS by tenant and case) ----------> writes (typed writers, worker role)
                                                                 ^
        jobs (NATS) ---> Worker (Python): ingest, layouts, typing, media tasks (cpu_light | cpu_heavy | gpu)
                                                                 |
        Model services (LocalAI): language model, embeddings, speech  |  vision services (OCR, detectors, face)  |  object storage on the evidence volume
```

* **Today** most of this exists: one Go package of 402 files (`api/forensic_records`, about 64,000 lines of non-test code), one Python worker, one database, LocalAI. The interface talks only to the Go API through the Vite proxy.
* **Recommended structure** (done gradually, never as a big-bang rewrite): new capabilities go into **internal Go packages** with narrow interfaces (`internal/conversations`, `internal/media`, `internal/identity`, `internal/crosscase`, `internal/reports`), registered from `main.go`; the flat package stops growing; each new package ships with its own migration range, view definitions, specs and a bank.
* **Roles in the database:** `localrecall` (owner, migrations), the runtime role under RLS (writers), the **read-only analyst role** (the engine). Every query path sets `app.tenant_id` and, from this pack on, `app.case_ids` (the cases the user may see).

## 3. The domain model (one vocabulary for all modules)

| Group | Object | Today | Needed |
|---|---|---|---|
| Identity and authority | user, role, permission, **case_member**, **authority_record** (reference, issuing body, scope, validity, attachments) | one shared actor; read-only role; RLS | **new** (migration 034) |
| Case | `case_record` (name, status), collection mapping | name/status, audited | + type/profile, time zone and clock basis, languages, enabled capabilities, classification, retention, owner, team |
| Evidence | `evidence_items`, `evidence_versions`, `evidence_storage_objects`, `evidence_custody_events`, `evidence_source_links` | done | + tags, legal hold, supersession, "used by" index |
| Records | `records`, typed tables (`comm_event`, `data_session`, `app_usage`, `bank_transaction`, `plate_sighting`, `cell_site`, `subscriber_*`, `device_tac`), effective views | done | + message, extraction, location-history, browser families (section 8) |
| Derived | `derived_artifacts`, `derivation`, `content_embedding`, `document_mention` | done | + media tables and `analyst` views (migration 033, `02_media_programme.md` 3.3) |
| Entities | `entity`, `identifier` (**new**: type, normalised value, keyed hash), `alias`, `entity_mention`, `relationship`, `relationship_evidence` | tables exist and are **empty** (except document mentions) | populate through the mention index and resolution (section 4.4) |
| Knowledge capture | **note, finding, hypothesis, task, tag, bookmark, saved view, annotation** | in the browser's `localStorage` only | **new**, server-side, audited |
| Conversations | **thread, turn, turn_feedback** | in the page only | **new** (section 7) |
| Reports | report, section, exhibit, export bundle | `POST /reports/generate` | + versioning, sign-off, bundle |
| Audit | `records_audit_log`, custody events | partial | one append-only, hash-chained log: access, search (including face search), export, role change, model run, cross-case query |
| Cross-case | identifier registry, `link_notice`, `access_request` | none | **new** (section 4) |

**States.** `observed | inferred | candidate | confirmed | rejected` on every relationship, link and mention; `model_candidate | human_reviewed | externally_verified` on every derived reading; both are columns, never free text.

## 4. Cross-case architecture

### 4.1 What it must do
Tell an analyst, without leaking what they may not see, that the thing they are looking at (a number, an account, a plate, a handset, a person's national ID number, a file or image, optionally a face cluster) also appears in other cases; let them ask across the cases they may access; and never merge or assert identity without a person.

### 4.2 Mechanism
1. **Identifier registry.** For each case a job extracts *strong identifiers* from typed tables, document mentions and media mentions: MSISDN (E.164), IMEI/IMSI, bank account/IBAN, plate (normalised, with date range), national ID number, email, file SHA-256, image PDQ hash. Values are stored as **keyed hashes** (HMAC with a tenant key held outside the database dump) so the registry can match without exposing values to roles that should not see them; the plain value stays in the case's own tables.
2. **Overlap detection** after each ingest: for each new hash, find other cases with the same hash; write a `link_notice(case_a, case_b, identifier_type, count_a, count_b, first_seen, last_seen, strength, state)`; `strength` is a word (exact strong key / strong key plus corroboration / weak) from rules, not from a model.
3. **Visibility rules** (configurable per tenant): a user who can open both cases sees the notice with details and a link; a user who can open only one sees "also appears in another case; owner: X; request access" (the owner name optional per policy); a case a user may not see is **never named**. Requests are recorded, answered by the other case's owner or a supervisor, and audited.
4. **Entity resolution** is a separate, reversible layer: `same_as(entity_a, entity_b, rule, evidence, state, decided_by)`. Strong keys give `candidate` automatically; names alone never do. Name matching is **transliteration-aware** (Muhammad/Mohammad/Mohd/Muhammed; Urdu script vs Roman; honorifics; father's name as corroborator) and needs at least two independent corroborators before a name-based `candidate` is even shown. Confirmation is always a person.
5. **Zero false merges** is the gate (the Control case with decoys: same name with another national ID number; a re-issued number held by another person).

### 4.3 What the analyst gets
* The **notice** on the number/account/plate/file/entity chip and in a case-level "Also in other cases" panel.
* **Ask across cases** (scope picker limited to accessible cases): the engine reads `case_id`-carrying views and answers "which numbers appear in more than one of my cases", "list repeated plates", "which SIM was swapped across cases"; the answer states the scope.
* **Link-chart overlay:** nodes coloured by case, cross-case edges dashed (candidate) or solid (confirmed); filter to cross-case only.
* **Case relations:** `related`, `same incident`, `parent/child` set by a person; shared evidence (same hash) keeps **one custody chain and many case references**.
* **Audit:** every cross-case query, notice view and access request.

### 4.4 The mention index (the missing foundation)
One table of *mentions* from every source: structured identifiers, document passages, OCR regions, transcript segments, chat messages (`mention_kind`, normalised value, locator, evidence id, confidence, state). It feeds: the identifier registry, "where else is this number mentioned" in the entity drawer, cross-family search (a number in a bank narration, a recording, a photographed document), and the entity graph. It exists for documents (`document_mention`, migration 031); generalise it to all sources.

## 5. Evidence: intake to answer, and every file must open

### 5.1 Intake and typing
`upload → hash and custody event → safety scan → type detection (extension, magic, content) → layout registry (provider layouts with confidence) → reader → typed writer → effective records (dedupe, overrides) → views → coverage (what the engine can now answer)`. An **unknown layout** does not fail: it opens a **mapping wizard** (columns, units, time basis, party roles) whose result is a draft layout that can be promoted after a real-file confirmation (the existing layout-registry process). Every stage records counts: accepted, duplicate, rejected-with-reason.

### 5.2 The evidence passport (what a person sees when they open a file)
Identity (name, type, size, hash, source, who added it, when), **processing state with per-stage progress and a reason and next action for every failure**, what was extracted (rows by kind, pages, languages, duration, faces/plates found), quality (rejected rows with reasons, repeated rows, time basis, layout confidence), **where it was used** (answers, findings, reports that cite it), custody history, and actions (open, download original with audit, tag, hold, supersede, reprocess with the current models, compare with another version, export).

### 5.3 The viewer rule: *if it is stored, it opens*
A viewer registry maps type to viewer; the last viewer, for anything unknown, is a **safe inspector** (metadata, hex, printable strings, container listing, a note saying which viewer is missing and a request button). The worst case is never a blank panel. Every viewer states completeness: "rows 1 to 50 of 408", "pages 1 to 12 of 12", "decoded as UTF-8", "N cells withheld by the privacy rule", "OCR: 3 low-confidence regions".

| Type | Viewer (state) |
|---|---|
| CSV/TSV, XLSX/XLS (incl. SpreadsheetML), JSON/XML tables | **Paged file-rows viewer with the file's own headings, order and column letters, cited-row card, spreadsheet-date reading (done 2026-11-02/03)**; to do: BOM-safe headings, header order for `.xls`/`.txt`, multi-sheet tabs, hidden sheets/rows flags, merged cells, formulas shown |
| Text, logs, chat exports | virtualised line viewer, encoding detect (UTF-8/16, Windows-1256, Arabic-script legacy), search, message-bubble mode for recognised chat formats |
| PDF | page viewer (PDF.js) with text layer, OCR overlay for scans, page rail, search hits |
| DOCX/ODT/RTF | server-side render to PDF/HTML, passages in order (done for the docx passages 2026-11-02), tracked changes flagged |
| Email (EML/MSG/MBOX/PST) | header table, body, attachment tree, thread view |
| Image | zoom/pan/tiles, EXIF panel, OCR overlay, face/plate boxes (governed), similar images |
| Audio | waveform, synchronised transcript, speakers, language chips, review |
| Video | player with event scrubber, shot list, tracks, plates, faces, speech |
| Archive (ZIP/RAR/7z/TAR) | tree with parent-child lineage, bounded extraction, hostile-input limits |
| SQLite/DB extractions | read-only table browser on a copy, bounded SELECTs |
| PCAP/EVTX | flow summaries / event counts (no payload) |
| Maps (KML/GPX/GeoJSON) | map layer with a table |
| Unknown | safe inspector |

### 5.4 File-type coverage matrix (priority order of work)
| Source type | Examples | Status | Route | Priority |
|---|---|---|---|---|
| Call detail records | operator exports, CSV/XLSX/XLS/TXT | **done** (20+ layouts) | comm_event | maintain |
| Internet session records | IPDR | **done** | data_session, app_usage | maintain |
| Subscriber/registration, handsets | operator lists, TAC | **done** (subscribers gate not met) | subscriber_*, device_tac | close gate |
| Cell sites/tower data | site lists, tower dumps | partial | cell_site | tower dumps P1 |
| Bank/wallet statements | CSV/XLSX | **done** (gate met) | bank_transaction | PDF/image statements via OCR (P1) |
| Vehicle-camera sightings | camera exports | **done** | plate_sighting | + media plates (WP-M5/M6) |
| Documents | DOCX, TXT, PDF | **done** (retrieval, mentions) | passages, mentions | scanned PDFs (WP-M4) |
| **Message exports** | WhatsApp .txt/.zip, SMS backup XML, Telegram JSON | **open** | new `message` family | **P1 (high)** |
| **Phone extraction reports** | UFDR/UFED/Oxygen/XRY CSV/XLSX/HTML | **open** (inventory only) | contacts, calls, messages, locations | **P1 (high)** |
| Email | EML/MSG/MBOX | open | message family | P2 |
| Location history | Google Takeout JSON, app exports | open | location events | P2 |
| Browser/app history | Chrome/Firefox SQLite, Takeout | open | web events | P3 |
| Access/door/VPN logs | CSV | open (migration 022 numbering plan) | access events | P3 |
| Images, audio, video | all | partial (media tracks) | media tables | **P1 (focus)** |
| Archives, SQLite, PCAP, EVTX | | inventory only | child evidence | P2 |
| Financial documents | cheques, FBR/tax forms, invoices | open | OCR + parsers | P3 |

### 5.5 Quality architecture
Per file and per case: accepted/duplicate/rejected counts with reasons; **statement integrity checks** for financial files (opening + credits - debits = closing, running-balance continuity; failures are shown as "gaps or edits in the statement" indicators); time-basis state for every typed row (`declared`, `explicit_offset`, `confirmed_by_analyst`; never assumed); unit and range checks; layout confidence; a **data-quality ledger** surfaced on the Overview and Evidence pages, never a ring without a question it answers.

## 6. The Ask engine: runtime language-model use, end to end

### 6.1 The path (as built, with the additions marked)
```
question (any language) -> [conversation role: greeting/help/clarify/scope (OFF today)]
   -> frame (what is asked: family, entities, time, measure, parameters read from the question by code into runtime settings)
   -> coverage (what this case holds: views, domains of columns, languages)           [exists]
   -> demonstrations (verified examples selected by similarity, as data)              [exists]
   -> query writer (model writes ONE read-only SELECT over reviewed views)            [exists]
   -> validator: parse tree + obligations derived from the question (pair, direction, order, count/sum, value domain, time) [exists]
   -> repair loop (bounded, messages say what is wrong)                               [exists]
   -> executor (read-only role, statement timeout, row/column walls)                  [exists]
   -> evidence tracing (rows behind each figure; records behind each summary view)    [exists: PART A / A2]
   -> answer composition (first sentence from the computed result under a number lock) [exists]
   -> reading (model interprets the computed result in plain words, grounded)         [exists: PART B]
   -> render (figure | table | chart | map | graph | timeline | passages | media moments)
```
Additions: media views and retrieval for transcript/OCR text; the conversation role; cross-case scope; follow-up context; streaming of phases.

### 6.2 The model roles (a registry, each with a contract)
| Role | Job | Model class | Guard | Status |
|---|---|---|---|---|
| R1 Frame reader | what kind of request, which family/case/scope, parameters | small instruct + code parse | schema-constrained output; code reads numbers/dates | exists (IR) |
| R2 Query writer | one SELECT over analyst views | 4B CPU now; 8B to 14B on GPU | validator, obligations, read-only, walls | **live** |
| R3 Verifier | checks the query asks what the question asked | **code first**, model second opinion optional | never lets a failing query run | live (code) |
| R4 Result reader | explains the computed result in plain words | same model | may only restate computed values; number lock | live (PART B) |
| R5 Document answerer | answers "what does it say" from retrieved passages with citations | same | quote-only, passage ids, PII guard | partial |
| R6 Media answerer | media questions: structured via views, text via retrieval, moments as links | same | label "model observation" | **to build** |
| R7 **Conversation role** | greetings, thanks, "what can I ask", clarification, refusal explanations, scope help | small and fast | answers only from case coverage and capability list; never invents data | built, OFF → **activate after measurement** |
| R8 Translator/transliterator | Urdu, Roman Urdu, English, Punjabi/Pashto glosses | local LLM / IndicTrans2 | labelled "machine translation" | partial |
| R9 Summariser/report writer | case summary, timeline narrative, report sections | same | every sentence tied to evidence ids; number lock; human sign-off | partial (reports) |
| R10 Extractor | entities/mentions from documents, OCR, transcripts | schema-constrained | validators (checksums, patterns) | partial |
| R11 Describer (VLM) | image/video captions | Qwen3-VL (GPU) | "model description, unreviewed", never counted | to build |
| R12 Judge | offline evaluation of answers | larger model or human | not in the live path | tooling exists |

**Greetings and small talk (owner request).** R7 is a real model role, not a list of canned strings: it receives the user's message, the case's coverage summary and the capability list, and answers briefly in the user's language ("Hello. This case holds call records for 12 numbers, bank statements and 14 documents. You can ask about contacts, durations, money in and out, or what a document says."). It is measured (does it ever invent a capability or a figure? does it route a real question to the engine?) before it is switched on; a failed measurement keeps it off.

### 6.3 Understanding, context and follow-ups
* **Conversation state per thread:** the last frames (family, entities, time window, parameters) are passed to R1 so "and for the last week?", "what about the second number?", "show them on the map" resolve against the previous turn. The resolved frame is shown to the analyst ("Interpreted as: calls of 923000001002 between 1 and 28 February") with a one-click edit.
* **Clarification is a successful answer:** when the request is ambiguous or a parameter is missing ("long calls": how long?), the engine asks one specific question and offers the options as buttons.
* **Languages:** English, Urdu (Nastaliq/Naskh), Roman Urdu and mixed; Punjabi/Pashto/Saraiki questions are evaluated as a language programme (PL; native reviewers needed, D10). Dictation: the analyst may speak the question; our ASR transcribes it, and the transcript is shown for correction before it runs.
* **Runtime parameters:** numbers an analyst asks for (minutes, days, thresholds) are read from the question by code into settings that the views read (`askParams` registry); never constants in SQL or code.

### 6.4 The "no templates" boundary, stated so it can be tested
Allowed: the schema description and column domains read from the case at run time; validators and obligations computed from the question's own parse; retrieval-selected demonstrations shown to the model; executors; evidence tracing; wording of the **first sentence from the computed result** (a presentation of data, not a canned answer). Not allowed: a table mapping question patterns to SQL or operations; keyword ladders that choose canned answers; per-question special cases. **Test:** an ablation run with the demonstrations removed must show the model still writes correct SQL for the families whose views are well described; the share of answers that depend on a demonstration is reported at each milestone, so examples cannot become templates by the back door. The old keyword ladder and registered operations (11 template-only correct answers on the 2026-09-29 shipped posture) are retired family by family behind switches with "0 correct lost, 0 new wrong".

### 6.5 Performance and the model path
CPU: p50 about 30 s, p90 about 50 s, reading 43 to 57 s (limit 120 s); plan cache of 509 entries; one request at a time. Levers in order: (1) a GPU and an 8B to 14B model (about 10 times faster and more accurate), (2) streaming of phases so the wait is explained, (3) caching of coverage and demonstrations, (4) a small router model for R1/R7. **A model swap needs the written decision rule of WI-0** (AGENTS.md); the rule for the GPU bake-off is drafted in `06_roadmap_phases.md` (Phase 1), measured on the verified bank plus a fresh judgement case, with "0 correct lost, 0 new confident-wrong" as the floor.

### 6.6 Safety
Prompt-injection: media text and documents are data in quoted blocks; the model never sees rows as instructions; tools are read-only. Privacy: subscriber names never returned, national ID numbers masked, plate masking per D28, "withheld" cells marked. Abstention: when the query cannot be shown to ask what was asked, the engine says so (this is why 7 bank questions abstained: honest, but the fix list in section 2 of file 01 closes them).

## 7. Conversations and history (like a chat application, but evidence-grade)

### 7.1 Model
`conversation_thread(id, case_id|null, scope_case_ids[], owner, title, pinned, archived, shared_with[], created_at, updated_at)` · `conversation_turn(id, thread_id, idx, parent_turn_id, question_text, language, frame_json, query_text, query_hash, result_ref, answer_text, evidence_refs, engine_version, model_id, data_version_hash, timings, state, error_reason)` · `turn_feedback(turn_id, user, verdict, note)`. `result_ref` points to a compressed **snapshot of the result rows** (size-bounded; larger results are re-runnable instead); `data_version_hash` is the hash of the evidence versions and effective-record version the answer was computed on.

### 7.2 What the analyst can do
* **History rail** (as in chat apps): grouped by day/case, search (full text over questions and answers), rename, pin, archive, delete-with-audit (deletion hides, the audit keeps the fact).
* **Continue** any thread; **edit and resubmit** a question (creates a branch); **fork** a thread from any turn; **re-run** a turn against current data and see **"changed since"**: a diff of the figures and of the evidence set (this is the "reworking" the owner asked for).
* **Pin to findings:** an answer (with its evidence and snapshot) becomes a finding/exhibit; findings are editable text with live evidence references.
* **Compare** two turns or two runs side by side.
* **Share** a thread to case members (read-only link inside the app); permissions follow case membership.
* **Export** a thread or a selection to the report builder.
* **Reproducibility:** each turn shows engine version, model, data version, and a "reproduce" action; a court exhibit prints these.
* **Retention** follows the case; per-user private threads are allowed but visible to auditors.

### 7.3 API (new)
`POST /conversations`, `GET /conversations?case_id=&q=`, `GET /conversations/{id}`, `POST /conversations/{id}/turns` (runs the engine; streams phases), `POST /turns/{id}/rerun`, `POST /turns/{id}/fork`, `PATCH /conversations/{id}`, `POST /turns/{id}/pin`, `POST /turns/{id}/feedback`, `GET /turns/{id}/diff?against=`.

## 8. Structured data families: how a family is added (so every chat does it the same way)
1. **Layout spec** (provider layouts in the registry: header patterns, column meaning, units, time basis, party roles) with synthetic samples and, when available, a vault-confirmed real sample.
2. **Reader and normaliser** (encoding, BOM, date/time formats incl. Excel serials and time zones, phone numbers to E.164, plate/IBAN/IMEI normalisers, amounts and signs).
3. **Typed writer** to a typed table with `source_row`, `evidence_id`, time-basis state and the idempotency key.
4. **Migration + views** in `analyst` (thick, curated, documented columns; `case_id`-carrying; effective-records variant).
5. **Engine description + one verified example** per family operation (data only).
6. **Generator + oracle** (planted truth) in the case generator or the reference-case builder; **bank** written against the data; **judge**.
7. **Gate** pre-registered (G1 typing, G3 oracle equality, G4 fresh-case engine test with 0 wrong).
8. **Viewer** (file viewer + family view) and **Playwright test on a generated case**.
9. **Documentation** (`docs/data/<family>.md`) and the coverage entry.
Families to add next, in order: message exports, phone-extraction reports, tower dumps, PDF/image bank statements via OCR, email, location history.

## 9. Reports and exports
A report is an **ordered list of references** (findings, pinned answers, charts, media exhibits, timeline excerpts) with an optional narrative written by R9 under the number lock; every sentence carries evidence ids; formats PDF and DOCX; **hash manifest** (originals, derived, report), custody log extract, reproducibility block (engine, model, data version), reviewer sign-off with name and time; versioned; exports of media use clips/crops with burned-in hash and time. `POST /reports/generate` exists; the builder UI, sections, sign-off and bundle are new (stage U8).

## 10. Identity, security, audit and operations
* **Identity:** local accounts first (offline), optional OIDC; MFA; roles `admin | lead | analyst | reviewer | viewer | auditor`; permissions `export`, `cross_case`, `face_search`, `vault_access`, `reprocess`, `delete_with_audit`.
* **Row-level security** by tenant and case membership; the engine's read-only role sees only views; secrets never in images or logs.
* **Audit log:** append-only, hash-chained, with a UI for auditors: logins, access, searches (including every face search with its stated reason), exports, cross-case queries, role changes, model runs, evidence actions.
* **Backup and restore:** nightly dump + evidence volume snapshot; a **tested restore** (a restore on this laptop takes about 9 minutes and needs free memory); restore rehearsal before every migration.
* **Deployment:** compose now; an offline installer with a model registry (hashes), later; observability from `engine-health` and queue depth in the Activity page; no uncontrolled `latest` tags (every live image has a rollback tag).
* **Scale:** benchmark at 10 times the Orion volume before the enterprise phase; partition or hypertable the event tables by case and time; index review.

## 11. Contracts the interface needs (the 30 backend requests, grouped)
BR-1 case record and census (members, authority, classification, readiness, summary, directory); BR-2 conversations (section 7.3); BR-3 knowledge capture (notes, findings, hypotheses, tasks, tags, saved views, annotations); BR-4 activity/audit/reports (global history, report builder, exports); BR-5 media (media list, derived artifacts by locator, review actions, clock offsets); BR-6 cross-case (notices, access requests, registry). `docs/work/BACKEND_REQUESTS.md` is the register; this pack's phases deliver them in the order the interface stages need them.
