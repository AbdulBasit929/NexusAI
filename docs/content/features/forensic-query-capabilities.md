---
title: Forensic query capabilities and readiness
---

The existing `GET /api/records/forensic/capabilities` endpoint adds semantic
references and scoped readiness to its family inventory. Supply an authorized
`collection_id`, optional `case_id`, and optional `evidence_id` with
`evidence_version_id` to inspect a selected current source. Omitting evidence
selection evaluates the authorized workspace.

Five responsibilities remain separate:

| Contract | Authority |
| --- | --- |
| Operation registry | Existing executable operations, arguments and executors |
| Query capability | Semantic reference to an operation or existing direct descriptor |
| Readiness | Current scoped records/artifacts and independent processor receipts |
| Certification | Existing canonical operation certification ledger |
| Suggestions | Existing product certification and suggestion policy |

`query_capability_references` describes supported semantics, artifact contracts,
input boundaries, authority, scope and proof requirements. It contains no SQL,
executor implementation or duplicate argument schema. Required arguments are
resolved from the operation registry. Image and face similarity reference their
existing direct descriptors and do not increase the executable query count.

`query_capability_readiness` separates `query_existing_results`,
`process_new_input` and `generate_new_output`. A retained transcript may remain
queryable while fresh ASR processing is disabled. A completed zero receipt is
different from NOT_RUN, FAILED or INCOMPLETE. Positive current observations
survive a later failed job; a failure cannot manufacture zero results. Workspace
complete-zero readiness requires every eligible source to have a current zero
receipt. Actual search completeness still requires operation execution.

Evidence checks use tenant, collection, optional case, selection and current
version predicates. Artifact run IDs bind processor zero receipts to current
evidence. Structured readiness requires records from the source's current batch.
The API does not cache evidence readiness or execute models to render discovery.

The current API has no fresh worker readiness-receipt transport. It reports
fresh-processing availability as unverified/unavailable until that authority is
provided; configured roles or installed model names cannot substitute for it.
Similarity discovery requires the existing executor's authorized query/candidate
validation. It does not run vector comparisons during discovery. KB runtime
availability and composed component prerequisites also remain separate checks.

Ordinary audio does not establish speech-generation availability. TTS has no
registered generation descriptor in this reference baseline and stays unavailable.
There are no new model, voice or processor controls.

Advertised extensions are classified separately from qualified analytical
paths. Native text extraction covers TXT/PDF/DOCX. MD/YAML, standalone subtitles,
EVTX, OFX/MT940 and HEIC/SVG/raw image claims remain unqualified. Capture,
SQLite and ZIP/TAR inventory does not imply session extraction, arbitrary SQL
or child-ingest analytics. Media qualification refers to existing bounded paths,
not every possible codec. Scanned PDFs require a separately supported OCR path.

The B1 text semantic contract preserves raw versus Roman-derived authority,
bounded 2–4 segment transcript windows and EXHAUSTIVE-only absence claims.
Language matching proof does not certify natural-language planning or every
paraphrase. Citation schemas do not certify navigation or Activity reopening.

This C baseline is SOURCE_CANDIDATE and is not deployed by adding these files.
The endpoint reports an un-attested source identity, not the prior accepted
container's identity. The report under `reports/nxb21/` records source/live drift;
future activation and product certification retain their separate gates.


## Bounded canonical projection and grouping (source candidate)

The existing `forensics.canonical_records` typed query operation accepts optional
`projection` (unique canonical field names) or `group` controls. Projection permits
`record_type`, `timestamp`, `primary_target`, `secondary_target`, `source_file`,
`row_number`, and `ingested_at`. Source provenance is always retained after privacy
redaction. Payload expressions and unknown fields are rejected.

A group is an object such as `{"field":"source_file","measure":"count"}`.
String group fields are `record_type` and `source_file`. It counts
all authorized matching rows only when the complete input has at most 1000 rows
and at most 100 distinct groups. Larger inputs fail with a request to narrow the
filters; they never yield a partial aggregate labeled complete. Null is a separate
bucket from the empty string. Results sort by count descending, null first on ties,
then exact string value ascending. Each group retains exact source-row lineage.

Grouping is a single operation and cannot combine projection, custom sort,
pagination limit/offset or arbitrary composition. Its governed plan carries the
100-group output ceiling. The result distinguishes source-row total from returned
group count. Tenant, collection, source and batch filters use the existing governed
canonical query builder. No raw SQL, expressions or arbitrary schema fields are
accepted. This source extension does not certify deployed API/UI behavior or new
family capabilities.

The source candidate also supports `group.field=timestamp` with `bucket` equal to
`hour`, `day`, `week` (Monday start), or `month`, and an explicit IANA `timezone`.
An increasing RFC3339 `time_range.from`/`to` is required; the query interval is
half-open. Buckets retain offset-bearing boundaries and `bucket_end_exclusive`.
UTC, Pakistan calendar boundaries, month/leap-year changes, and New York DST
transitions have deterministic fixture coverage. Empty buckets are not invented.
The same 1000-input-row and 100-group ceilings apply.

Optional `group.top_k` (1–100) runs after filtering, complete grouping, exact count
and deterministic sorting. It selects displayed groups, never truncates the input
before aggregation. The summary states the full group count and displayed subset.

Optional `compare` supports an exact `count` measure over `source_file`,
`record_type`, or `timestamp`. For string fields, operands `a` and `b` each contain
one exact `value`. For timestamp comparisons, each has explicit increasing
RFC3339 `from` and `to` bounds. Both operands use one bounded SQL snapshot under
the existing scope and filters. Output includes A/B counts, B-minus-A difference,
overlapping contribution count and exact source-row lineage. Missing source values
fail closed; a complete empty selection counts zero. This counts source records,
not distinct people, vehicles or documents. It cannot combine grouping,
projection, pagination or custom ordering. Camera/direction comparisons and
semantic document comparisons are not provided by this canonical operator.

Example typed plan:

```json
{"contract_version":"forensics.query-plan/v1","intent":"forensics.canonical_records","compare":{"field":"source_file","measure":"count","a":{"value":"source-a.csv"},"b":{"value":"source-b.csv"}}}
```

For canonical typed plans, `record_type` and `source_file` filters are exact
nonempty string equality against canonical columns; repeated scalar predicates
are rejected. Other payload filters retain their established behavior. Free-form
SQL, arbitrary expressions and arbitrary caller-authored `steps` remain rejected.
Existing registered bounded compositions remain available. These source and
disposable SQL/API checks do not establish new natural-language model competence.

General forensic assistant turns obtain a bounded read-only adapter/template
snapshot from the existing server discovery endpoints. Implementation, exposure
and certification states must remain distinct from runtime or processor readiness.
Unknown discovery, licensing and model availability must remain unknown. Registry
presence alone does not establish a format can be processed in the current runtime.
