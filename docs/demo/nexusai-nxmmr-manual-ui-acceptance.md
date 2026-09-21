# NX-MMR manual Investigation Workspace acceptance

Use this only after the exact model/data/runtime approval and successful
automated gates. It is a manual product acceptance script, not authorization to
upload, reprocess, deploy, or expose sensitive evidence.

## Preconditions

- Record operator, date/time, source commit, live container/image IDs, catalog
  version, model/processor IDs and hashes.
- Confirm the workload-class RAM gate immediately before processing: LIGHT
  3.0 GiB, MEDIUM 3.5 GiB, HEAVY 4.5 GiB, or the higher measured incremental
  peak plus 1.5 GiB headroom. VERY_HEAVY is not admitted by default. Stop near
  1 GiB free or on severe paging/evaluator death/Docker instability.
- Do not confuse this benchmark rule with the unchanged 6 GiB deployment/build
  gate; this script authorizes neither deployment nor a build.
- Confirm rollback tags, healthy PostgreSQL/NATS, and unchanged retained tuple.
- Confirm approved evidence manifest, lawful-use/privacy review and sealed
  ground truth.
- Confirm capability payload contains only `PRODUCT_CERTIFIED` suggestions.
- Open a clean browser session at 390, 820, 1024 and 1440 px widths; record
  console/page errors and horizontal overflow.

## Journey A — Home and Data

1. Open the retained investigation workspace.
2. Verify Home uses evidence readiness, not structured-row count, and offers no
   unsupported or no-data action.
3. Open Data and verify structured, document, image, audio and video evidence
   have truthful type/status/source labels.
4. Open one item per family. Check preview, findings, limitations, technical
   disclosure, source hash/locator, and that model observations are not styled
   as deterministic facts.
5. For a missing-model item, verify `MODEL_REQUIRED`/unavailable presentation;
   no stale retained artifact may imply fresh processor readiness.

## Journey B — certified Ask suggestions

1. Start a fresh Ask view and capture the visible suggestions.
2. Resolve every suggestion to an operation ID and verify it is
   `PRODUCT_CERTIFIED` in the live ledger.
3. Confirm no image OCR, face, similarity, audio transcript, TTS artifact,
   spreadsheet, capture, database, archive, or other limited/no-data prompt is
   shown unless separately product-certified.
4. Run `cdr.frequent_contacts` and `cdr.temporal_activity` through both visible
   action and natural language. Confirm the same typed operation, parameters,
   answer, citations and limitations.
5. Repeat accepted English, Roman Urdu, Urdu and mixed-language variants where
   certified; LTR identifiers must remain intact inside RTL content.

## Journey C — result-state truth

For a certified operation, verify independently prepared cases for:

- positive results;
- authoritative complete-zero;
- filter no-match;
- processing/not processed;
- model/processor unavailable;
- bounded failure/retry.

Counts, labels and calls to action must not collapse these states. A zero result
must have row count zero; unavailable must not be described as “no findings.”

## Journey D — citations and Activity

1. Open each answer citation and verify it reaches the correct evidence source,
   row/page/source-second/finding locator supported by that family.
2. Save the result and open Activity.
3. Reopen the stored result without rerunning analysis; compare operation ID,
   parameters, state, counts, findings, citations and limitations.
4. Navigate back to Data and verify the original source context is preserved.

## Journey E — approved multimodal benchmark result

Only after non-retained benchmark approval:

1. Review independently labeled positive and negative image/ANPR/OCR examples.
2. Verify raw/model-normalized plate strings, confidence, crop/source link and
   review-required state; never claim ownership or identity.
3. Review Urdu OCR with original text, normalized derivative, CER record and
   limitations; no invented missing text.
4. Review audio with Urdu source transcript, Roman Urdu derivative where
   governed, timestamps and identifier limitations.
5. Review video frames/audio/timeline at source seconds. Verify complete-zero
   and no-match separately and that historical candidate outputs are not used
   as current ground truth.
6. Review face results only as detection/candidate similarity; no identity or
   same-person declaration.

## Cross-cutting checks

- keyboard-only navigation, visible focus, labels and accessible names;
- light/dark theme and zoom at 200%;
- no horizontal overflow at all four widths;
- no secret, raw internal path, database ID, model prompt or stack trace in the
  ordinary analyst surface;
- no action that lacks a real backend path;
- no query or upload crosses tenant/case/collection scope;
- no console error or failed request hidden behind a success state.

## Acceptance record

Record each step `PASS`, `FAIL`, `BLOCKED`, or `NOT_APPLICABLE` with screenshot,
request/response receipt, operation ID, evidence locator and reviewer. Any P0
or bounded P1 stops acceptance. P2/P3 items receive an owner and future phase;
they do not silently disappear.

Final declaration must separately state source, build, deployment, runtime,
model, dataset, retained-state and product acceptance. “Demo passed” is invalid
unless all required declarations and the exact certification ledger are
attached.
