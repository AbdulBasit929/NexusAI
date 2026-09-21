# NexusAI Phase 2B.2 — Pakistan Structured Goldens Acceptance

Date: 2026-07-27  
Scope: synthetic Pakistan ANPR and PKR financial transactions  
Result: PASS

## Outcome

The first Pakistan-specific messy structured-data pack is implemented, tested,
deployed and reconciled live. It preserves exact source rows while generating
separate searchable and quality-controlled fields. No real sensitive evidence,
database migration, model change or GitHub action was used.

| Measure | ANPR | Transactions | Total |
| --- | ---: | ---: | ---: |
| Source rows | 10 | 11 | 21 |
| Unique accepted records | 5 | 6 | 11 |
| Exact duplicates | 1 | 1 | 2 |
| Explicit rejects | 4 | 4 | 8 |
| Completed jobs/evidence/KB assets/KB entries | 1 | 1 | 2 |

Collection: `forensic-pakistan-structured-goldens-20260727`  
Case: `PK-SYNTHETIC-STRUCTURED-20260727`  
Failures or missing KB assets: 0

## Implemented behavior

ANPR recognizes common registration, camera, capture-time and location aliases.
It retains the exact Urdu or Latin plate token, builds a separate NFKC search
key, labels script, normalizes fractional/percent confidence, validates supplied
coordinates and crop SHA-256, preserves province/rule metadata, and flags
low/missing confidence for review. A plate is not rejected solely because it
does not match one current province-series example.

Transactions recognize transaction/debit/credit/local amount and source/
beneficiary account aliases. PKR amounts remain exact decimal strings and integer
minor units; accounting parentheses remain negative reversals. Invalid grouping,
more than two minor-unit digits, and multiple populated amount fields are
rejected instead of guessed. PK IBAN structure and MOD-97 are recorded separately;
an invalid source IBAN is preserved and flagged rather than silently corrected.

For both types, the raw JSON/CSV row, row hash and lineage remain unchanged.
Naive timestamps use the row or upload source timezone and canonical UTC is stored
separately. Missing timestamps remain visible and review-required.

## Verification evidence

- Python adapter suite: 17/17 passed in 0.063 s.
- Combined Python worker/adapter suite: 21/21 passed in 0.157 s.
- Complete `go test ./api/forensic_records`: PASS, package time 10.033 s.
- Pakistan profile and modality matrix: valid JSON.
- Live row accounting: exact 11 accepted + 2 duplicate + 8 rejected = 21.
- Direct read-only SQL matched Urdu text, UTC conversion, confidence, one-paisa
  value, `-250.00` reversal, valid/invalid IBAN flags and all eight row-numbered
  rejection reasons.
- Protected `records-demo-verified`: unchanged at 9,250 accepted rows, four
  completed jobs, zero rejected/duplicate/failed rows.
- LocalAI mirrored exactly two KB entries. Recent API and worker logs contained
  zero `error`, `panic`, or `fatal` lines.

Fixture SHA-256 values:

- ANPR CSV: `bfdefc0159566a2f16a5332b1efeb4805e354b6f39eab42e9473e4e28e2f65a6`
- Transaction JSONL: `d74bbdf24b733cdb6fef959ac1d0ba0a6cf2f9bf363d8ae572b2d7315ef19f88`
- Golden manifest: `7ee8667e34a081be82e55063be8e4ba86ad7adc89f6e00b38f6c679a9053996e`

## Deployment and rollback

Targeted API and worker build: 43.7 s. No LocalAI rebuild was needed.

- API: `sha256:39a1bc8d9af8f3bb43352ac75ac36dff26281932abbe34856d302328f39d4869`
- Worker: `sha256:c2e7fe083355f2bd1a031ccd24c44883dbb9a25253a553ecc079b0703eee73c9`
- API rollback: `nexusai-forensic-records-api:rollback-20260727-mpeg-v1.5`
  at `sha256:f24d90c5c7941165d137492420d486fd82fbb0c95af62c173f15bb0ec1a6e772`
- Worker rollback: `nexusai-forensic-records-worker:rollback-20260727-pre-pk-goldens`
  at `sha256:faeb65590b1f6a61e7458a9c73c763b5c82dc772df29f20fd8843691c9615268`

Post-deploy health was HTTP 200 for the API, worker metrics, NATS and LocalAI;
PostgreSQL and NATS were healthy and all service restart counts were zero. Single
diagnostic snapshots were API 5.484 MiB, worker 57.89 MiB, PostgreSQL 57.91 MiB
and NATS 8.566 MiB. Health response was 7.246 ms and collection status 18.836 ms;
these are not load-test percentiles.

## Sources and limits

Engineering rules were checked against State Bank of Pakistan IBAN guidance and
its remittance transaction-field format, plus Sindh and Punjab excise plate
examples/policies. Those sources support exact IBAN structure/checksum fields and
province/series-aware ANPR review. They do not justify one permanent national
plate regex or prove that every bank, wallet, camera or legacy export is covered.

Remaining gates include authorized shape-derived bank/wallet and province-series
packs; plate-image/OCR goldens; fees, chargebacks, split payments, reconciliation,
OFX/MT940/spreadsheets; and the next Pakistan subscriber, tower and access-log
pack. No model accuracy claim is made by this deterministic slice.

## Git state

The worktree remains intentionally dirty, unstaged, uncommitted and unpushed.
Nothing has been published to GitHub.
