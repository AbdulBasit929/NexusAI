# NX-MMR acceptance correction: deployed and runtime-verified

Deployment update: operator receipt
`local-acceptance-models/nxmmr/private-activation/20260831T135259631Z`
records `CORRECTION_DEPLOYMENT=PASS`. Live LocalAI/UI and worker match the
intended immutable candidate image IDs, are healthy with zero restarts, and the
three protected services retained their identities. Runtime upload variables
both equal 320; OCR revision equals `nxmmr-anpr-ocr-vertical-v1-inputfix1`.
Active jobs are zero and retained counts remain `54|54|67|22208|446|50`.
Model-load readiness and a non-body 300 MiB Content-Length admission probe PASS.
No actual evidence upload/inference was performed by deployment verification;
product acceptance and certification gates remain pending.

Date: 2026-08-31. Scope: user-authorized source preparation and isolated tests
for the negative-image OCR failure, at least 300 MB uploads, and upload UX.
No live deployment, retained upload/retry/reprocessing, model download,
package installation, database/volume mutation or certification promotion.

## Changes

- Paddle detector receives decoded BGR pixels rather than the extensionless
  spool path. Revision `nxmmr-anpr-ocr-vertical-v1-inputfix1`. Recognition,
  thresholds, models and original source bytes remain unchanged.
- Add data permits general/video files through 300 MiB (314,572,800 bytes),
  rejecting one byte over. Existing image safety ceiling remains 64 MiB.
- A prepared-only Compose fragment sets LocalAI `LOCALAI_UPLOAD_LIMIT=320`:
  335,544,320 bytes for the whole HTTP request, including multipart metadata.
  Direct API requests do not gain a new strict 300 MiB per-file backend cap.
- HTTP 413 gets an explicit size message. Network failure retains uncertainty
  and instructs checking Data before retrying. Waiting/registration/closing
  guidance is visible. Existing two-slot concurrency is unchanged.

The supplied `video-v3.mp4` is 184,407,144 bytes (175.9 MiB), below the prepared
file ceiling. No actual 300 MiB payload or retained video was uploaded in these
checks, and no claim of live large-upload acceptance is made.

## Verification

- Dependency-free ANPR/OCR vertical self-tests: **16 PASS**.
- Node upload-policy/error/config tests: **7 PASS**, including exact-limit and
  one-byte-over checks, unchanged image cap and the supplied video's size.
- Focused Chromium intake checks: **4 PASS**, including existing duplicate/
  classification-failure flow, waiting/selection discard, HTTP 413 versus
  connection reset, and close-disabled during registration / enabled after
  server acceptance. All HTTP activity used mocks at an isolated Vite server
  on 127.0.0.1:5187, whose backend proxy targeted unused port 9, not live 8080.
- Full analyst-portal Chromium suite: **24 PASS** (1.1 minutes). The initial
  broader run had 22 passes and two console-cleanliness failures because the
  existing fixture did not mock `/api/operations`; adding the empty-operations
  response fixed the isolated fixture without relaxing any assertion. Live
  backend access remained disabled. Focused ESLint and API syntax check PASS.
- Real cached Paddle runtime with candidate adapter: **PASS**, generated blank
  and printed control, comparing `.png` versus identical extensionless bytes.
  Blank: COMPLETE_ZERO_RESULTS, zero regions/observations. Printed control:
  COMPLETE_RESULTS, one region/observation. Both suffix parity and source
  unchanged assertions passed. This is a functional check, not accuracy scoring.
- Smoke resource measurements: **12.156 s wall**, **694.473 MiB peak process RSS**.
  Ephemeral container: network disabled, read-only root, 2 CPUs, 2 GiB cap,
  temporary scratch and private receipts only; no retained-evidence or live
  database/volume mounts. Existing staged models were hash-checked.
- Before/after snapshots confirm unchanged IDs/images/restarts for LocalAI,
  worker, forensic API, PostgreSQL and NATS; active jobs zero. Post-check free
  RAM was 2.960 GiB, below the unchanged 6 GiB deployment gate. This does not
  authorize lowering that gate.

Private receipts: `local-acceptance-models/nxmmr/private-activation/acceptance-correction-20260831/`
(`before.json`, `after.json`, `ocr-smoke.log`, `ocr-smoke.json`).

## Candidate identity and historical seal

Candidate OCR SHA-256:
`35eae2fe5b6f5713b9c713856b79d0c34cb484741e61e2aba569e64bdf886577`.
Upload fragment SHA-256:
`9366702a71092c66b5b2eb4e66353ea49e23463e52319c5ea932f716f412e768`.

Current live worker image remains
`sha256:e495788b85f07d9a4552c07796f8b810fa4f600ecd96a08acfa6c2afbb02ce73`.
The original activation manifest and operator integrity seal are preserved as
historical artifacts. The changed OCR source intentionally no longer matches
that old source seal. Do not bypass, blindly refresh the seal, or rerun the old
worker-only activation script to deploy this correction. Its authority does
not cover protected LocalAI changes, and its original rollback assumptions are
not a current deployment snapshot.

## Remaining gates and exact next action

The live negative evidence `04db33ae-feea-49e3-8c9a-0011cf1bcd7f` remains failed;
no retained retry has occurred. The video's previous connection failure is not
an inference result. Positive image Ready is not completion of all product
gates. No PRODUCT_CERTIFIED promotion or NX-B2.1.

Next: obtain explicit approval for a bounded correction deployment plan that
captures current service configuration and rollback identities, stages/tests
the worker correction and embedded UI build, applies the 320 MiB LocalAI
request cap without dropping unrelated configuration, and enforces existing
health, zero-job and 6 GiB memory gates. Do not recreate forensic API, database,
NATS or other services. Long builds require approval; none were run here.

After that deployment is separately approved and verified, obtain/confirm
authority for a single retained negative-evidence retry through supported
application controls (not direct database edits) and the user's video upload
into the existing **Multimodal Product Acceptance** collection. Preserve
deduplication and distinguish reused results from fresh processing. Then run
the required API/Ask/Data/citation/Activity/UI/manual gates. No sealed-holdout
benchmark rerun or model-output oracle is part of this correction.
