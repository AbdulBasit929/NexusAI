# NexusAI R8.1 and R8.2 source acceptance

Date: 2026-08-12  
Disposition: R8.1 complete; R8.2 source accepted; R8.3 active  
Mutation boundary: source, tests, and documentation only; no evidence ingest,
model download, migration, profile change, or deployment

## R8.1 capability and reuse matrix

| Required capability | Reused authority | Decision |
| --- | --- | --- |
| Case and tenant scope | authenticated forensic scope binding and URL-bound Case Workspace | reuse; never accept UI-only scope |
| Immutable source bytes | SHA-256 scoped content-addressed store and full-read verification receipt | reuse unchanged |
| Evidence identity | evidence item, version ID, raw storage reference, source hash, custody events | reuse unchanged |
| Image type detection | byte-signature detector plus declared extension/content-type comparison | deepen through the R8 image-intake contract |
| Image technical metadata | bounded PNG/JPEG/GIF/BMP/TIFF/WebP header readers | reuse and publish admission decision |
| EXIF | bounded JPEG/TIFF orientation only | retain orientation; exclude GPS and free-text metadata from the analyst projection |
| Derived evidence | processing runs, derived artifacts, parent IDs, hashes and citation locators | reuse in R8.3; no crop is created in R8.2 |
| Structured ANPR | `nexusai.adapter.anpr` and exact ANPR operations | reuse; structured rows are not image/OCR proof |
| Specialist | existing Vehicle ANPR Geospatial Analyst | extend later; do not create competing authority |
| Model roles | evidence model catalog and modality evaluation matrix | benchmark in R8.4; no model selected or downloaded now |
| Analyst UI | Case Workspace Evidence Operations and Media inventory | reuse; add readable image-admission truth |
| Shared chat reliability | Agent Chat correlated lifecycle and bounded scrolling | add explicit jump-to-latest recovery |

## R8.1 privacy and threat model

| Threat | Boundary and control | Residual disposition |
| --- | --- | --- |
| forged extension or MIME | detect supported raster type from bytes; mismatch blocks automatic decode | preserve source, manual review |
| malformed/truncated header | bounded parser with explicit failure metadata | preserve source, manual review |
| decompression/pixel bomb | header-only inspection; 100,000,000-pixel and 32,768-pixel-per-axis gates | automatic decode prohibited |
| oversized image | 64 MiB image admission limit enforced before retention when declared by image extension and recorded in metadata policy | reject declared oversized image; disguised input remains non-executable and review-bound |
| active/vector content | SVG/HEIC/RAW and other non-enabled codecs are preserved but not decoded | manual review; no embedded execution |
| metadata privacy leakage | expose orientation and bounded technical fields only; no EXIF GPS/free text | original bytes remain governed evidence |
| parser resource exhaustion | 4 MiB bounded configuration read and 512 chunk/IFD-entry limits | later decoders require separate sandbox/resource gates |
| cross-case access | server-authoritative tenant/case binding on registration and inspection | deny conflict; UI cannot grant scope |
| silent evidence rewrite | content hash, immutable storage receipt, version and custody chain | corrections/derivations must be new artifacts/events |
| premature OCR claim | capability remains detection/OCR pending; UI states no OCR/detection claim | R8.3/R8.4 gates remain mandatory |
| inaccessible long answer | contained scroll region, stable gutter and explicit keyboard-focusable jump-to-latest control | live deployment acceptance remains required |

Supported bounded raster headers are PNG, JPEG, GIF, BMP, TIFF and WebP. Other
image formats remain preservable evidence but are not automatically decoded.
Archives are never treated as image intake and embedded content is never
executed.

## R8.2 delivered contract

- `forensics.image-intake/v1` records validation state, detected format,
  extension/signature agreement, source bytes, dimensions, pixel count, color
  model where deterministically available, orientation, inspection mode, policy
  limits and downstream-decode eligibility.
- Valid headers become `accepted_metadata_only`; this does not authorize OCR,
  detection or model inference.
- Malformed, unsupported, mismatched or resource-amplifying images become
  `manual_review_required`, use `image_intake_manual_review`, and cannot be
  promoted to a downstream decoder.
- PNG, BMP and WebP enforce exact terminal/container boundaries; an appended
  archive-payload PNG fixture is forced to manual review.
- Evidence inspection presents the decision and technical facts without
  displaying private EXIF free text/GPS or implying OCR.
- Staging warns for non-enabled codecs and blocks declared images over 64 MiB;
  the server independently enforces that declared-image byte limit.

## Verification

- `go test ./api/forensic_records`: PASS.
- Focused ESLint for `AgentChat.jsx` and `EvidenceWorkspace.jsx`: zero errors;
  pre-existing file-level warnings remain.
- React production build: PASS, 669 modules transformed.
- Complete Agent Chat and Case Workspace Chromium production-bundle suites:
  38/38 PASS. The two new focused scenarios cover:
  - bounded image-admission metadata and no OCR/detection claim;
  - long-answer scroll recovery with Jump to latest.
- No new model, backend, migration, evidence item, case data, container, volume,
  profile or deployed service was changed.

## Exit decision

R8.1 is complete and R8.2 is source accepted. Runtime acceptance is deferred to
the guarded R8 deployment slice because the source change does not justify an
early deployment. R8.3 plate-region detection and exact crop provenance is the
sole active work item.
