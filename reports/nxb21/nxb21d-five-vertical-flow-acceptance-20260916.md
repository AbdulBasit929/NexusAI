# NX-B2.1D Five-Vertical-Flow Acceptance — 2026-09-16

## Scope and authority

This is a fixture/disposable functional acceptance receipt, not product
certification and not a retained-data acceptance run. It records executed
request/response paths across deterministic backend contracts and the analyst
UI. The generic source-native flow used an owned, randomly named PostgreSQL
database with forced RLS; the harness deleted the database and role and proved
the retained tuple and Activity count unchanged. No model inference, model
download, deployment, migration, retained-evidence write, or Git operation was
performed.

## Executed acceptance

- Full backend package after WP5: `go test ./api/forensic_records -count=1
  -timeout 180s` — PASS in 74.349 seconds.
- Focused backend paths: CDR specialist response, ANPR specialist response,
  document passage answer, and raw-first audio transcript result — PASS.
- Auditable CDR follow-up inheritance and explicit-current-turn precedence —
  PASS.
- Full current analyst portal suite against a fresh production bundle — 31/31
  PASS in 1.7 minutes. Its broad matrix covers 5 routes × 5 widths × 2 themes
  with no horizontal overflow and no captured console/page errors.
- Focused ESLint for every changed frontend source/spec file — 0 errors. The
  non-quiet run reports 34 existing warnings; the closing slice introduced no
  new warning or error.

## Flow receipts

### A. CDR / structured — PASS

- Request: `Who did this number contact most?`
- Executed response trace: operation `cdr.frequent_contacts`; exact CDR ranking;
  deterministic citation `calls.csv`, rows `1-5004`; Activity reopened the
  stored result and exposed `Continue in Ask` with the bounded follow-up
  `Follow up on: Who did this number contact most?`.
- Backend evidence: `api/forensic_records/platform_contracts_ginkgo_test.go`
  (`adds a CDR specialist trace, deterministic citation, finding, and typed
  visualization`) and `api/forensic_records/query_followup_contracts_test.go`
  (`TestAPF34AuditableFollowUpInheritanceAndExplicitPrecedence`).
- Browser evidence: `core/http/react-ui/e2e/analyst-portal.spec.js`
  (`presents governed Activity as a searchable journal and reopens the stored
  result`). Screenshot:
  `core/http/react-ui/reports/nx-ux1g-final-acceptance/activity-reopened-result-1440.png`,
  SHA-256 `cb6adc59af4488a1b1a590363afb2378c4a054c1a399c1f325c676760fe5ce39`.

### B. Document / RAG — PASS

- Request: `Find the exact phrase "retention schedule" in the document.`
- Executed response trace: retrieval mode `exact_phrase` with
  `exact_is_semantic=false`; answer `The cited policy states that the retention
  schedule applies for seven years.`; retained passage `The retention schedule
  applies for seven years.`; source `policy.pdf`; locator `page 4 · paragraph
  2`; source drawer opened at page 4; follow-up request captured as `Compare
  this passage with the other selected policy.` with case and collection both
  bound to `workspace-alpha`.
- Backend evidence: `api/forensic_records/document_retrieval_ginkgo_test.go`
  (`presents document passages as grounded answers, findings, tables,
  citations, and methodology`).
- Browser evidence: `core/http/react-ui/e2e/analyst-portal.spec.js` (`walks a
  grounded document answer through its source locator and follow-up`).
  Screenshot:
  `core/http/react-ui/reports/nxb21-five-flows/document-grounded-answer-1440.png`,
  SHA-256 `53c577634ccf4be7e9c3ff74731d38ddb1675592f5a66b3679bb2d8c3b7fbba8`.

### C. ANPR / media — PASS

- Request: `Show plate observations from 10 to 30 seconds.`
- Executed response trace: operation `anpr.sightings`; applied source-time
  interval 10–30 seconds; one `LEH5003` sighting; exact source locator `row 42`;
  source drawer opened with that citation context; limitation `A plate
  observation does not establish ownership.` remained visible on demand.
- Backend evidence: `api/forensic_records/platform_contracts_ginkgo_test.go`
  (`adds an ANPR specialist trace, bounded limitation, citation, and typed
  visualization`).
- Browser evidence: `core/http/react-ui/e2e/analyst-portal.spec.js` (`presents a
  one-result ANPR answer as compact facts with provenance on demand`).
  Screenshot:
  `core/http/react-ui/reports/nx-ux1g-final-acceptance/ask-governed-result-1440.png`,
  SHA-256 `9cb166d28a7e6b416eb18383126f34b2ca271995825740b9d0650b70ad104b71`.

### D. Generic source-native — PASS

- Request: `Which department has the highest average invoice total?`
- Executed request trace: authorized field discovery returned `department` and
  `invoice_total`; the dynamic typed plan grouped by the issued department
  field, computed `AVG` over the issued invoice field, sorted measure `m1`
  descending, and limited the result to one row.
- Executed public API response trace: department `Sales`, `m1=400`, lineage
  from `hash-3` and `hash-4`, evidence
  `20000000-0000-4000-8000-000000000001`, version
  `30000000-0000-4000-8000-000000000001`; foreign-tenant row and
  `secret_token` were absent.
- Backend evidence: `api/forensic_records/source_native_sql_api_ginkgo_test.go`
  (`discovers fields from the authorized SQL scope and executes the
  representative AST through the public API`). Receipt:
  `reports/nxb21/source-native-sql-api-20260916T163944208Z.json`, SHA-256
  `b095ec3658f5abf1075329bb0d88c657209935260e513bc3795679236e61bbc7`.
  The receipt records `passed=true`, `cleanup=true`, and
  `retained_unchanged=true`; retained tuple remained
  `69|69|82|22512|876|69`, Activity remained `354`, and active jobs remained
  `0`.

### E. Audio / text — PASS

- Request: `آڈیو متن میں تلاش کریں "مجھے معلوم نہیں آیا آپ نے محسوس کیا یا
  نہیں"`
- Executed response trace: operation `audio.transcript_search`; the exact raw
  transcript phrase was primary; the answer retained the phrase and exposed
  source timestamp `0 seconds` (bounded segment 0–10 seconds) with source
  `interview.wav`; normalized internal target remained hidden.
- Backend evidence: `api/forensic_records/governed_transcript_query_ginkgo_test.go`
  (`returns raw transcript evidence first and preserves Roman Urdu parent
  lineage`).
- Browser evidence: `core/http/react-ui/e2e/analyst-portal.spec.js` (`keeps a
  natural Urdu transcript phrase primary and normalized targets hidden`).
  Screenshot:
  `core/http/react-ui/reports/nx-ux1g-final-acceptance/final-urdu-result-390.png`,
  SHA-256 `f4d6a9f0d3e5813dd407b4239a9705e212937d136b274172c1796a434ecf3eff`.

## Adjudication

`FIVE_FLOWS_RESULT=PASS_A_PASS_B_PASS_C_PASS_D_PASS_E_PASS`.

These proofs close the NX-B2.1D functional flow baseline only. They do not
certify every provider export, language, codec, accelerator, model, or retained
production dataset.
