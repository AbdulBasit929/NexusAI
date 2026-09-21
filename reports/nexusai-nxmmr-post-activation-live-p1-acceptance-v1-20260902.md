# NX-MMR Post-Activation Live P1 Acceptance V1 — 2026-09-02

## Final video/Activity replay — authoritative overlay, 2026-09-03

**PASS WITH DISCLOSED LIMITATIONS: the two remaining replay-scope P1 defects are closed.**
This supersedes older current-status text below. Deployed receipt
`20260903T063719719Z` remains VERIFIED; do not rerun its activation command.
The immutable operator receipt records its then-pending functional acceptance;
this later report supplies that acceptance without rewriting the receipt.

Four fresh, selected-source Ask requests ran at 11:53–11:55 Asia/Karachi, followed
by actual Data citation and Activity reopens. Tenant `default`, case
`nexusai-multimodal-product-acceptance`, evidence
`a3c75ee1-957d-4476-867b-6c3bf0b6cecd`, version
`b33200e0-df9e-4219-921b-893a46fc85e6`. STIM guidance required an independent
retained-artifact oracle; computer-use guidance required actual UI interaction.

| Replay | Analysis ID | Measured outcome |
|---|---|---|
| Exact raw MM51VSU | `d395b197-96ec-44be-ba78-6b5ef850c6ae` | PASS: 2 rows, 2 citations, 222 ms server elapsed; raw MM51VSU distinct from group MW51VSU; 1s/frame60 and 2s/frame120 |
| Absent QZ99XYZ | `9818a434-2f2a-44f1-91ad-c8230f9daba9` | PASS: explicit plate-named no-match, 0 rows/citations, 288 ms; no evidence UUID substituted as target |
| Exact selected text MW51VSU | `92a2c795-7c1c-4d76-affc-fb94c762670e` | PASS: 5 raw observations at 0, 0.5, 1.5, 1.75, 2.25s; 5 citations, 336 ms; exact-target route intentionally returns readings |
| Grouped video plates | `14531d53-870b-45a3-a15f-be461e5a364d` | PASS bounded projection: 20 rows/citations, 642 ms; MW51VSU group interval 0–2.25s, sightings5, selected_group_candidate, manual review required |

Independent PostgreSQL artifact reads confirmed the two raw MM51VSU artifacts
`89fd5927-3aeb-52ba-9ac3-e82f1eadffed` and
`7f2cf88b-d631-5113-911b-810805fb2a75`, their timestamps/frame numbers/bboxes,
and group `53c8723e-fba7-50ae-834e-a8c7283beed9` with selected MW51VSU,
range 0–2.25s and sightings5. A version-scoped metadata search found zero
QZ99XYZ artifacts. This is a retained-observation oracle, not proof that OCR
readings identify the physical plate correctly or population accuracy.

Ask rendered typed top-level columns, not duplicated bbox rows. Both raw source
buttons opened the correct evidence/artifact; DOM video currentTime was 1 and 2
seconds respectively. Fresh positive, no-match and grouped Activity entries
reopened with the same values. The Activity positive citation also opened at 1s.
Desktop screenshots and DOM inspection confirmed readable result panels.
Historical malformed entry `7611fa6c-9402-4d35-a502-2f91beb0ed82` reopened
without a crash and explicitly reported unavailable historical result details;
its persisted columns remain null and four old bbox rows remain untouched.

Final health PASS, active jobs 0, required model inventory PASS, all five
container IDs/images/restart counts unchanged, no OOMKilled flags. API and
forensic API restarts0; protected worker remains9; PostgreSQL/NATS0.
Retained tuple before/after `63|63|76|22507|827|61`; source seal unchanged
`0b11a1b6cf1b27dc890dd17ceb306cbc255a00ef3f7ba16b5e216d293fcf3127`.
Available host RAM measured 3.493 GiB before and 3.114 GiB after; no RAM failure
occurred in this read/query replay. These measurements do not pass or waive the
6-GiB build gate and cannot guarantee future headroom. No build, cleanup,
service mutation, model load/unload request, migration, upload, reprocessing,
deletion or historical rewrite occurred. Four normal Ask/Activity entries were
added; a full-history count was not measured in this replay.

**Gate decision:** video result-contract and Activity P1 closure is complete
for this bounded scope (open P0=0/P1=0 here, not repository-wide). Earlier OCR,
similarity, Urdu/timing and cross-evidence passes are carried forward, not newly
recertified. Off-page filename/Target label polish, broader language/accuracy
coverage, TTS and all-family certification remain limited/deferred. Historical
bad answers are not repaired retroactively. Next is NX-B2.1 scope/preflight
planning; its deployment/runtime boundary still requires separate approval.
No NX-B2 activation or product-certification promotion was performed.

## Four-P1 deployed replay — authoritative overlay, 2026-09-03

**Replay complete; acceptance PARTIAL / NOT READY.** This overlay supersedes
older current-status and next-action text below. The user completed activation
receipt `20260903T050747951Z`; deployment verification PASS is independently
confirmed. The used activation must not be rerun. This turn performed only
browser acceptance, read-only diagnostics and these checkpoint updates.

Six fresh Ask requests ran at 10:24–10:28 Asia/Karachi in tenant `default`, case
`nexusai-multimodal-product-acceptance`, followed by citation/Activity reopens.
STIM guidance kept independent retained evidence separate from presentation
claims; computer-use guidance required actual Ask/Data/Activity interaction.
No source fix, build, service recreation, model download, evidence upload,
reprocessing, deletion, migration, history rewrite or NX-B2 activation occurred.

| Fresh cell | Analysis ID | Result |
|---|---|---|
| Selected-video raw `MM51VSU` | `7611fa6c-9402-4d35-a502-2f91beb0ed82` | SQL now completes and executive answer correctly counts 2; FAIL presentation/citations: table contains 4 bbox rows, no plate/time/source columns, no evidence links |
| Selected-video absent `QZ99XYZ` | `7439d286-d688-453b-86d5-72d747102ec7` | Zero/no-match terminal outcome works; PARTIAL wording incorrectly names evidence UUID as target instead of plate |
| Selected-image OCR `Investigation Workspace` | `1ac54012-4077-433d-b802-c7a9419c94ff` | PASS exact positive; correct retained image citation opens; Activity reopens same finding |
| OCR absent `NEXUSAI ABSENT OCR PHRASE 9Q7` | `a683e589-bbc4-407d-a3f3-a725a5e39755` | PASS exact no-match with no unrelated citation; Activity list agrees |
| Selected subject-b image similarity | `eecfb42b-4892-4619-b992-e288d2b4f39a` | PASS Ask/Activity: 10 ranked candidates, scores and friendly query/candidate filenames; candidate citation opens subject-a-frontal |
| Selected subject-b face similarity | `af878921-0875-430a-bc50-fe6742c608a9` | PASS Ask/Activity: 3 candidates, exact scores, filenames, candidate-only limitations and source opening; no identity conclusion |

### Independent evidence and persisted contracts

Read-only raw-artifact inspection reconfirmed video version
`b33200e0-df9e-4219-921b-893a46fc85e6`: raw `MM51VSU` artifacts
`89fd5927-3aeb-52ba-9ac3-e82f1eadffed` at 1 second/frame 60 and
`7f2cf88b-d631-5113-911b-810805fb2a75` at 2 seconds/frame 120.
Group `53c8723e-fba7-50ae-834e-a8c7283beed9` selects `MW51VSU`; no
`QZ99XYZ` artifact is present in the bounded selection. The new raw/group
distinction is not yet usable in Ask because those fields are lost downstream.
An additional selected-group query was not needed to establish this failed gate.

OCR artifact `480638b2-f3cd-5d02-8704-4b58c09d9156` contains raw and normalized
`Investigation Workspace`, bbox x87/y112/w645/h62. Fresh retained presentation
preserves image evidence/version and that locator. The browser opens the correct
image with its two text regions; automatic region focus was not separately
certified. Similarity scores agree with the previously independently checked
retained oracle: image top three 0.785177674 / 0.765967065 / 0.742119207;
face 0.309298360 / 0.291637892 / 0.175236154. No scoring/model change was made.

All six new records are `completed` with nonempty answers; both negative
controls remain visibly distinct from positive results. Historical empty records
`f29f4754-e950-4393-b208-6ce11c439e1b` and
`d8866db9-a9de-4ca4-a8ba-009c76ac01bd` remain `completed`/empty in the database
(not rewritten), but Activity now labels them Processing failed. The former
reopened safely after loading the older Activity page. Fresh error/late-event
injection was not performed against production; that state-machine guarantee
retains its prior DB-backed source-test proof, not a newly claimed live test.

### Remaining bounded defects

1. **P1 — video result projection/provenance.** Persisted presentation for the
   positive contains four bbox-only rows, `columns: null`, `priority_columns:
   null`, and `citations: []`, despite row_count 2. Source inspection explains
   the mismatch: `enterpriseDisplayRows` special-cases similarity but sends
   video through `flattenEnterpriseRows`; `enterpriseIsRow` rejects nested
   locator maps and descends into bbox children. Video no-match also presents
   the evidence UUID as the requested target. Fix the typed video projection
   and provenance through the complete enterprise -> agent -> retained contract,
   preserving raw plate versus group-selected plate and exact frame locators.
2. **P1 — Activity crash on malformed video table.** Opening the fresh video
   result causes `Cannot read properties of null (reading 'map')` in
   `AnalystHistory`. Its table render calls `columns.map` without an array guard.
   A normal navigation back to the Activity list recovers the UI; no history
   deletion is necessary. This is a presentation robustness failure, not proof
   that the terminal-status repair failed.

Additional limited UI findings: Data's separate image ranking panel still
labels off-page candidates by UUID (top three in this check), although Ask and
Activity correctly hydrate those names. Face Ask still shows the query UUID in
its Target metric, and selected-image Ask can say Current evidence instead of
the filename. These are P2 polish/consistency gaps; do not describe the whole
Data/Ask experience as fully closed. Existing date-query `AND-3` parsing in
unrelated retained history was observed but not replayed or certified here.

### Runtime / accounting / next gate

Final read-only health checks PASS, jobs 0; retained tuple unchanged:
`63|63|76|22507|827|61`. All five container IDs/images/restart counts match the
turn baseline; worker restart count 2 is pre-existing. Live API image
`sha256:9a7aa295c09cbbed77563fa7f6b95de2ebf57a3fd235c375935ae32b639e288d`;
forensic API image
`sha256:4965aa8054345f0a929be0a79a6ab17cbff7d52b3ceed235581c482bba9445bc`.
Analysis history was 278 initially and 290 at the accounting snapshot: six
explicit replay entries above plus six concurrent entries not submitted by this
replay. Do not attribute that concurrent activity to this agent or delete it.

Activation succeeded with the unchanged 6-GiB gate. These failures are not RAM
failures; cleanup or rerunning the same image cannot fix them. Next is a bounded
video enterprise/presentation/Activity correction with full-shape contract and
render tests, then a separately authorized new sealed deployment and only the
remaining failed-cell replay. Do not reopen OCR or similarity algorithms. Keep
NX-MMR active, NX-B2 paused, TTS blocked and all broader certification claims
unchanged. Two demonstrated P1 defects remain within this replay scope; this is
not a count of every possible repository defect.

## R2 failed-cell replay — authoritative current overlay

**Replay complete; acceptance PARTIAL / NOT READY.** This overlay supersedes
the historical live outcomes and next-action instructions below, not their
preserved evidence. Runtime receipt: `20260902T154901947Z`. Fresh browser queries
were run on 2026-09-02, 21:01–21:13 Asia/Karachi, followed by Activity/citation
reopens and independent read-only API/SQL checks. Case:
`nexusai-multimodal-product-acceptance`, tenant `default`.

The STIM skill governed scope/oracle/status separation; the computer-use skill
required actual browser interaction. No source corrections, activation,
uploads, reprocessing, model acquisition, pruning, evidence rewrite, history
clear, schema change or protected-service mutation occurred in this replay.
Eleven fresh Ask queries created their ordinary retained Activity entries.

| Fresh cell | Observed result | Disposition |
|---|---|---|
| Urdu `اس آڈیو میں کیا کہا گیا؟` | Raw Arabic-derived Urdu plus separately derived Roman text; correct source | PASS, not verbatim-accuracy certification |
| `انہوں نے یہ کب کہا؟` | Retained 0–10-second range, correct audio citations | PASS |
| `When did they say that?` | Same source range; not literal search for “that”; Activity reopen agrees | PASS |
| Audio citation seek | Correct native player, duration 10.5, readyState 4; after manual scrub to 4.979461, reopening citation sets currentTime 0 | PASS for this zero-second locator only |
| Selected-audio `Find plate ZZ99ZZZ. Return no exact match if absent from this selected evidence.` | Zero results, no transcript substitution | PASS for intent isolation; no general cross-case certification |
| Selected-video exact `MM51VSU` | No Ask answer; Activity “Results available” then “No retained answer text was reported.” | FAIL |
| Selected-video absent `QZ99XYZ` | No Ask answer; history completed with no result | FAIL; absence control not accepted |
| Data-generated OCR `Investigation Workspace` | False exact no-match, no PDF substitution | FAIL positive; source isolation improved |
| OCR `NEXUSAI ABSENT OCR PHRASE 9Q7` | Exact no-match, no unrelated citations | Negative control only; cannot close a path that fails its positive |
| Selected subject-b image similarity | 10 ranked persisted candidates; first three scores 0.785177674, 0.765967065, 0.742119207 | Routing/result PASS; presentation FAIL |
| `Find candidate faces visually similar to this face.` | 3 candidates, scores 0.309298360, 0.291637892, 0.175236154; no identity claim; candidate citation opens subject-a-frontal | Routing/result/citation PASS; presentation FAIL |
| Phone `923001110001` + plate `MN1367` | 1,283 structured observations and 1 ANPR sighting; both source families in first two of 20 citation cards and Activity reopen | PASS for bounded composition/citation fairness |

### Fresh retained analysis IDs

| Question/cell | Analysis ID |
|---|---|
| Urdu transcript | `939897d5-bb5e-4b96-af15-a1a42918b5ee` |
| Urdu timing | `12bf01ba-ccbe-49e1-a665-a677c69c9a22` |
| English timing | `94d0d212-6a5f-439a-a76d-9d3c631ae553` |
| Audio-scope absent plate | `abb5a786-5e3a-4594-a4ab-69d683455b94` |
| Video positive | `f29f4754-e950-4393-b208-6ce11c439e1b` |
| Video negative | `d8866db9-a9de-4ca4-a8ba-009c76ac01bd` |
| OCR positive | `7f8d89f6-20bc-453c-8992-8358e9a4ed89` |
| OCR negative | `2d571840-5b95-41ce-abd9-3e571cf2c8c9` |
| Image similarity | `167c3b74-7e4e-49e5-bb9d-786acd70152f` |
| Face similarity | `33f175b0-5bff-44cf-9870-043eb6217216` |
| Cross-family | `460c2ad1-4c93-461a-89e5-f4b0540ea35a` |

### Independent oracles and four remaining P1 areas

1. **Video execution:** existing current-version raw observations contain two
   `MM51VSU` readings at 1s/frame 60 and 2s/frame 120; no raw `QZ99XYZ` reading.
   Evidence `a3c75ee1-957d-4476-867b-6c3bf0b6cecd`, version
   `b33200e0-df9e-4219-921b-893a46fc85e6`. A direct analytical case-query POST
   with the same selected evidence and explicit video template returned HTTP 400:
   `iterate analytical rows: ERROR: unsupported subplan type for SkipScan: Result
   (SQLSTATE XX000)`. No database configuration was changed. Source inspection
   additionally shows the grouped route filters the group's selected normalized
   plate, while `MM51VSU` is a retained alternative/raw reading under selected
   group `MW51VSU`; exact-reading versus selected-group semantics need explicit
   tests. Neither SQL execution nor exact/no-match can be promoted.
2. **Failure status:** video positive message `1788365343835223300` status GET
   returns `completed` with no answer/error; history and browser agree on this
   incorrect terminal success. Activity must not infer available results from
   that envelope. This is separate from fixing the database query itself.
3. **OCR contract projection:** evidence `cd8c840f-404f-49d9-920a-c57f1bb15fab`,
   version `6925d443-9aa5-48aa-95b9-dd24b77670b4`, artifact
   `480638b2-f3cd-5d02-8704-4b58c09d9156` retains raw and normalized text
   `Investigation Workspace`, bbox x87/y112/w645/h62. SQL in
   `api/forensic_records/derived_text_query.go` only coalesces top-level text and
   transcript-oriented observation fields, not OCR raw/normalized text. A
   read-only SQL oracle returned the correct raw text and NULL for that existing
   coalesce expression. Parser-only/post-SQL fixture tests do not cover this gap.
4. **Similarity presentation:** source evidence
   `273bc132-74b5-4633-b8f9-bf0a812ef3ff`, version
   `7603fd46-a242-4a84-b081-23e4b1d225f1`. Ask defaults to 8 alphabetically
   selected technical columns, hiding Rank/Score; enabling them reveals correct
   candidate values. Image table still says “Image processing inventory” and
   repeats metadata-summary wording. Ask/Activity citations use “Source 1” and
   UUIDs for evidence outside the first loaded catalog page. Activity tables
   display raw `nexusai://` URIs and `[object Object]`. Reopen works, but the
   simple analyst presentation contract does not pass. Rankings remain model
   candidates, not identity or event findings; no new calibration claim is made.

Urdu source/version and raw/Roman lineage are unchanged from the historical
fresh-upload section. The browser transcript matches retained observations;
no new audio transcription or word-error-rate evaluation was performed.
Cross-family count was independently confirmed with read-only primary/secondary
target SQL (1,283). The ANPR citation opens retained
`a0ff7b18-129f-4a8a-82cf-48887f98bd9e`; the answer explicitly rejects identity,
ownership, causation and intent inference. No unrelated-case source was observed
in these named cells; adversarial tenant/RBAC certification was not performed.

### Runtime preservation and next boundary

All five container IDs/images and restart counts match the activation-verification
report. API remains healthy with zero restarts; forensic API running with zero;
worker healthy with its pre-existing two restarts; PostgreSQL/NATS healthy with
zero. SQL retained tuple remains `63|63|76|22507|827|61`, active jobs 0. Current
acceptance case is 36 sources / 35 ready / 1 failed, unchanged during this replay.
Free host RAM at final resource check: 4.950 GiB with Codex/browser open. No
OOM/build/resource failure was observed during these analytical queries. Deleting
data or reducing the RAM gate would not fix the demonstrated SQL/contract bugs.

`OpenP0Observed=0`; `OpenP1Areas=4`; `LiveAcceptance=PARTIAL_NOT_READY`;
`ProductCertificationPerformed=false`; `NXB2ActivationPerformed=false`.
English/Urdu TTS remain blocked and were not activated. Historical passed cells
not listed here were not broadly rerun or recertified.

**Next:** bounded source corrections with database-backed positive/negative
regressions and visible Ask/Activity presentation tests, then a new separately
approved source-sealed activation and only those failed-cell replays. Preserve
the healthy R2 runtime meanwhile. Do not rerun the already-used R2 activation,
bulk-reprocess evidence, rewrite history, tune/download models, or enter NX-B2.

---

## Historical first activation acceptance (superseded where noted above)

# Verified Activated Starting State

`ActivationAlreadyComplete=true`. The accepted starting runtime is the independently verified three-service activation under receipt `20260902T025246574Z`. This acceptance did not rebuild, recreate, restart, migrate, prune, download a model, or reprocess retained evidence.

# Activation Receipt

`ActivationReceipt=20260902T025246574Z`

The receipt remains the authority for the activated images and protected PostgreSQL/NATS identities. It must not be rerun for the new source-only corrections documented below.

# Runtime Health

Final live state: `nexusai-api-1` healthy, worker healthy, forensic API running, NATS healthy, PostgreSQL healthy. The acceptance observed no service restart or protected-service recreation.

# Fresh Urdu Upload

One fresh controlled file, `fleurs-ur_pk-validation-row4.wav`, was uploaded with explicit Urdu. Evidence `c80595c4-dd54-4604-a6ad-a687319eb03a`, current version `0ae1a63c-a6ac-4790-b221-89de60b2c4a6`, job `199ee...`, SHA-256 `dc6adf...`. The job completed with results and retained a 0–10 second observation over a 10.5-second source. Manual review remains required.

# Urdu Language Hint Transport

The Add Data selector offered `Automatic detection` and `Urdu — اردو`; Urdu was explicitly selected. Durable job metadata reported `asr_language=ur`, processor/model `faster-whisper-small-ur`.

# Urdu Script Acceptance

The authoritative transcript is Arabic-derived Urdu script, not Devanagari. Accuracy is limited and review is required.

# Trusted-Urdu Fail-Closed Behavior

The activated worker policy and activation checks reject Devanagari-only or Urdu-script-absent output on a trusted `ur` path instead of reporting successful Urdu STT. The fresh source satisfied the Urdu-script postcondition. Historical no-hint Devanagari evidence was preserved and not rewritten.

# Urdu Transcript

`مجھے معلوم نہیں آیا آپ نے محسوس کیا یا نہیں اس مرک میں سینٹر لمڈیکہ سے آئی زیادہ ترشیا دیوٹی فری ہے`

This is a model observation, not a guaranteed verbatim transcript.

# Urdu RTL

The raw Urdu Data observation uses `bdi dir=auto`, computed RTL, and showed no overflow. A combined Urdu+Roman Ask heading computes LTR as a mixed block; the raw Urdu observation remains correctly isolated and RTL.

# Roman Urdu

Derived text: `mjhe mlom nhin aaia aap ne mhsos kia ia nhin as mrk min sintr lmdikh se aayi ziadh trshia dioti fri he`. It retains the raw Urdu parent/version/time lineage and is not promoted above the raw observation.

# Urdu Ask

Generic Urdu Ask returned the current selected recording's Urdu and Roman observations.

# Urdu Exact Found

PASS. Exact normalized lookup returned the selected current-version transcript and its retained source.

# Urdu No-Match

PASS. The absent phrase `یہ عبارت بالکل موجود نہیں ہے` returned a truthful exact no-match.

# Urdu Time Query

FAIL live. Anaphoric Urdu timing repeated transcript text without the observed 0–10 second range. Source correction now recognizes Urdu/English timing anaphora as the governed `source_time` submode.

# Urdu Citation

FAIL live. The citation identified the correct audio/artifact but exposed the raw `nexusai://` locator and did not offer a visible time/seek action. Source correction preserves the typed source-time locator through Fact Packet and analyst presentation.

# English STT Regression

PASS. Exact lookup for `especially Japanese coconut sugar, and various aromatic spices.` in `fleurs-en_us-validation-row-01.wav` returned `audio.transcript_search`, exact found, one source citation, current-version scope.

# Conversation Context

## Same-Scope Anaphora

FAIL live. `When did they say that?` searched for the literal word `that`; `انہوں نے یہ کب کہا؟` repeated transcript text without timing. Source tests now pass both as `source_time` follow-ups.

## Explicit New Target

FAIL live. A plate query while fresh Urdu audio was selected returned the audio transcript. Source correction preserves explicit plate intent and prevents selected audio from swallowing another-family query.

## Scope Switch

Explicit whole-investigation wording is source-tested to broaden from selected evidence; no new live promotion is claimed before deployment.

# ANPR Exact Regression

General retained exact ANPR remains healthy: `MN1367` returned one cited image observation and `AV08HVF` returned cited structured ANPR rows. Selected-video exact `MM51VSU` failed live because the request remained on the legacy ANPR path and hit the Timescale SkipScan/status-storage defects. Source correction promotes selected-video plate intent to `video_anpr_grouped_timeline` and bounds transport errors to canonical status `error`.

# ANPR Multi-Trailing-Letter No-Match

FAIL live for selected-video `QZ99XYZ`; no truthful no-match answer was retained. The same routing/status source corrections cover one-to-three trailing-letter targets.

# OCR Exact Selected-Evidence

FAIL live. The Data-generated prompt for exact text `Investigation Workspace` cited unrelated PDF evidence through generic retrieval. Source correction now routes the exact product-generated `recognized text ... retained derived observations` wording to selected-image `image_ocr_search`.

# OCR No-Match

FAIL/not safely executable on the deployed Ask route because the exact selected-evidence OCR route is broken. The corrected route has focused no-match coverage but is not live.

# OCR Semantic Regression

Data correctly displays retained OCR candidates and review-required confidence/script labels. Ask semantic/exact authority remains blocked by the live routing defect.

# Transcript Time Locator

FAIL live; correct 0–10 second metadata exists but was not surfaced in the Ask result. Source correction and focused tests pass typed start/end preservation.

# Transcript Citation / Seek

FAIL live; correct source, no analyst-friendly seek. Source-only presentation correction is complete and tested.

# Cross-Family Query

Planning PASS. The live query for phone `923001110001` plus plate `MN1367` ran two governed steps: 1,283 structured observations and one exact ANPR sighting, with explicit non-identity/non-ownership/non-causation language. Citation presentation FAIL: the 20-item cap was filled by CDR rows and omitted the plate source. Source correction now reserves one citation per completed capability before filling the remaining cap.

# Family Authority

Structured records remain deterministic SQL authority; ANPR uses canonical/derived observations; ASR/OCR/SigLIP/face outputs remain review-required model observations. No LLM result was used as analytical authority.

# Structured CDR Regression

PASS. `cdr.temporal_activity` returned the governed 193-component result for `923001110001`, with three cited `seed_cdr_large.csv` sources and deterministic aggregate wording.

# SigLIP

## Fresh/Current Embedding

PASS in Data. The selected current-version source exposed the pinned SigLIP model metadata and persisted 768-dimensional observation without exposing raw vectors.

## Candidate Population

PASS. Data returned ten bounded candidates from a backend-authorized population of 12.

## Ask

FAIL as presented. The live Ask executed the similarity submode and retained ten correct candidate sources, but labeled it `image.metadata`, showed metadata/locator rows rather than ranked scores, and answered that no model was invoked.

## Citation

FAIL live as analyst UX. Candidate sources existed, but evidence/version identity and friendly navigation were not presented correctly.

## Activity

FAIL. Activity labeled the mispresented result as ten image-metadata results rather than a similarity comparison.

## Pagination

PASS. With only the first Data page loaded/visible, ranking still returned authorized candidates from later pages, proving backend-authoritative population.

# Face Candidate Comparison

## Ask

FAIL live. Data face ranking returned three bounded candidates, but `Find candidate faces visually similar to this face` ended in clarification. Source correction recognizes the natural wording and transports selected-evidence scope through NATS.

## Citation

FAIL live because Ask did not execute. Data exposed technical source URIs only.

## Activity

FAIL for the expected comparison event; it truthfully retained `Question needs detail`, not a face comparison.

## Safety

PASS. Data uses candidate-only, similarity-only, analyst-review language and explicitly says rankings do not establish identity. No name, demographic, criminality, ownership, relationship, or raw vector was shown.

# Data / Ask Consistency

FAIL overall. Urdu exact/no-match, English STT, general ANPR, and structured CDR are consistent. Selected-video ANPR, selected-image OCR, cross-family citations, and similarity/face Ask are not.

# LLM Dynamic Routing

`LLMIntentUnderstandingLive=PARTIAL_P1`. Governed phone+plate composition succeeds; natural face wording and some selected-source follow-ups fail live. `TypedDynamicPlanningLive=PARTIAL_P1`.

# Simple Ask UX

FAIL overall. Several successful answers are concise, but raw URIs, hidden similarity scores, misleading metadata wording, missing seek actions, and minute-long face clarification violate the required simple product experience.

# Activity UX

PARTIAL/FAIL. Most exact/no-match entries are understandable. Failed selected-video operations were labeled `Results available` without answer text, image similarity was mislabeled as metadata, and face comparison became `Question needs detail`. Source status bounding prevents future oversized error strings but needs deployment.

# Responsive Regression

PASS at 390, 1024, and 1440 pixels: no horizontal body overflow; main content and composer stayed within the viewport. Viewport override was reset afterward.

# Demo Readiness Matrix

| Capability | State |
|---|---|
| Shared Ask | NOT_READY_P1 |
| English STT | LIVE_DEMO_READY_WITH_LIMITATION |
| Urdu STT | LIVE_DEMO_READY_WITH_LIMITATION |
| Roman Urdu | LIVE_DEMO_READY_WITH_LIMITATION |
| ANPR exact | WORKSPACE_READY; SELECTED_VIDEO_NOT_READY_P1 |
| OCR exact | NOT_READY_P1 |
| Structured | LIVE_DEMO_READY |
| Image similarity | DATA_READY; ASK_NOT_READY_P1 |
| Face candidate comparison | DATA_READY_WITH_LIMITATION; ASK_NOT_READY_P1 |
| Cross-evidence | PLANNING_READY; CITATION_NOT_READY_P1 |

# Certification Matrix

No certification state was promoted. Existing registration/source/fixture states remain independent. `ProductCertificationPerformed=false`. `NXB2ActivationPerformed=false`.

# Remaining P0

None observed. `CrossCaseLeakage=NONE`.

# Remaining P1

All observed P1s have bounded source-only corrections and focused passing tests, but remain live until one later deployment: selected-source intent isolation and video ANPR routing; transcript anaphoric time/citation/seek; exact selected-image OCR routing; similarity/face Ask presentation and distributed scope transport; cross-family citation fairness; bounded retained error status.

# Post-Breadth Optimization Backlog

Recognition tuning, improved Urdu WER, OCR/ANPR quality optimization, broad responsive audit, and certification expansion remain deferred. Do not reopen model search during this correction deployment.

# TTS Status

English and Urdu TTS remain blocked/not activated. No package or model was downloaded.

# Files Changed

Source-only correction set: `core/services/agents/analysis_history.go`, `analysis_history_test.go`, `forensic_direct.go`, `governed_transcript_scope_test.go`, `dispatcher.go`, `dispatcher_scope_test.go`, `forensic_presentation.go`, `forensic_presentation_test.go`; `core/services/agentpool/agent_pool.go`; `api/forensic_records/derived_text_query.go`, `derived_text_query_test.go`, `query.go`, `stim_fact_packet.go`, `similarity_query.go`, `image_similarity_test.go`, `query_intelligence_composition_test.go`; `core/http/react-ui/src/analyst/analystAskPresentation.js` and its test.

# Tests

PASS: focused agent status/scope serialization; governed transcript/plate/image/face/OCR routing; structured locator presentation; forensic-record source-time and similarity presentation; bounded composition citation fairness; UI Ask/Activity presentation 13/13. Agentpool compiled successfully; Windows reported a non-failing temporary test-executable cleanup access warning. No full Docker-dependent suite or production build was run.

# Manual Browser Results

Fresh Urdu Data/Ask/exact/no-match/RTL/Roman lineage PASS. English exact STT PASS. Structured CDR PASS. Data SigLIP/face ranking and pagination PASS. Live P1 failures are recorded above without promotion. Cross-family two-step planning PASS with citation-cap defect. No hidden technical trace was treated as an analyst-visible pass.

# Runtime Resources

Final host: 15.713 GiB total RAM, 1.993 GiB free, 1,641.174 GiB free on C:. Container memory: API 4.169 GiB, worker 510.9 MiB, forensic API 20.6 MiB, NATS 19.58 MiB, PostgreSQL 248.8 MiB. This proves that a 6–8 GiB free-RAM gate is not achievable with the activated stack running; any later build must use the existing sequential drain/resume/cache-recovery design, never lower the safety floor or prune evidence/models.

# Retained Impact

Expected acceptance mutations only: one fresh Urdu evidence/current version/job plus retained Ask/Activity entries for Urdu, ANPR, similarity, face, and cross-family checks. No historical evidence rewrite, bulk reprocess, migration, named-volume replacement, protected-service recreation, model download, or database schema change.

# EXACT NEXT ACTION

Prepare and independently verify one new source-sealed three-service P1 correction candidate containing the complete tested set above. Do not rerun receipt `20260902T025246574Z` and do not activate from the current 1.993-GiB state. The later operator run must build sequentially with resumable per-image receipts, drain only the three mutable services at the guarded activation boundary, use clean page-cache mode 1 only when Dirty/Writeback are zero, preserve PostgreSQL/NATS identities and retained counts, and perform one rollback-capable activation. After that single deployment, rerun only the failed live cells; if they pass, stop acceptance and move to the next multimodal breadth.

## Expected Markers

```text
ActivationAlreadyComplete=true
ActivationReceipt=20260902T025246574Z
UrduLanguageSelectorLive=PASS
UrduLanguageHintTransportLive=PASS
TrustedUrduScriptPolicyLive=PASS
FreshUrduSTTLive=PASS_WITH_LIMITATION
FreshUrduScript=URDU
RomanUrduLive=PASS
UrduExactFoundLive=PASS
UrduNoMatchLive=PASS
UrduTimeQueryLive=FAIL
UrduCitationLive=FAIL
EnglishSTTRegression=PASS
SameScopeAnaphoraLive=FAIL
ExplicitTargetContextIsolationLive=FAIL
ANPRExactLookupLive=FAIL
ANPRTrailingLettersNoMatchLive=FAIL
OCRExactSelectedEvidenceLive=FAIL
OCRExactNoMatchLive=FAIL
TranscriptTimeLocatorLive=FAIL
CrossFamilyPlanningLive=PASS
StructuredAuthorityLive=PASS
ImageSimilarityAskLive=FAIL
ImageSimilarityCitationLive=FAIL
ImageSimilarityActivityLive=FAIL
ImageSimilarityPaginationLive=PASS
FaceAskLive=FAIL
FaceCitationLive=FAIL
FaceActivityLive=FAIL
DataAskConsistencyLive=FAIL
LLMIntentUnderstandingLive=PARTIAL_P1
TypedDynamicPlanningLive=PARTIAL_P1
SimpleAskUX=FAIL
CrossCaseLeakage=NONE
NoHardcodedAnalyticalAnswers=PASS
NoSelfOracle=PASS
NoUnrestrictedSQL=PASS
ProductCertificationPerformed=false
NXB2ActivationPerformed=false
DeploymentPerformed=false
```
