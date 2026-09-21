# NexusAI R7.8 protected CDR acceptance and R7 closure

Date: 2026-08-12  
Disposition: R7.8 read-only accepted; R7 complete

## Authorized scope

- Operator supplied a team-lead CDR dataset for testing and requested R7
  completion.
- Validation mode was ephemeral and read-only against the existing logical
  scope `tenant=default`, `case/collection=nexusai-forensic-demo`.
- The source remained in place. It was not copied, uploaded, retained,
  reprocessed, indexed, added to a model prompt or written to a database.
- Reported evidence is limited to source hash, byte/column counts, accounting,
  classifications, compatibility, performance and privacy assertions.

## Protected source identity

- SHA-256: `3EEE8CB613D2DEE9A5EE10600EC361AD2537C0B95D5601FE8D5C8D135A39C8F5`
- Bytes: 828,259
- Header columns: 16
- Encoding: UTF-8 with BOM
- Source timezone supplied to the adapter: `Asia/Karachi`

No raw header, row, subscriber identifier, device identifier, location value or
communication target is included in this report.

## Acceptance results

| Check | Result |
| --- | --- |
| CDR routing | PASS |
| Total row accounting | 3,931 |
| Accepted/normalized rows | 3,931 |
| Normalization errors | 0 |
| Unique normalized row hashes | 3,634 |
| Exact duplicate hashes | 297 |
| Complete file/row/hash locators | 3,931 |
| Explicit originating/called roles | 3,931 |
| Source/canonical timezone provenance | 3,931 |
| Rows with supplied location context | 3,931 |
| Rows with supplied coordinates | 3,885 |
| Manual-review quality flags | 0 |
| Direction classes | DATA 3,410; INCOMING 404; OUTGOING 117 |
| Service classes | PACKET_DATA 3,410; SMS 341; USSD 3; VOICE 177 |
| Timestamp basis | 3,931 assumed from supplied Asia/Karachi source timezone |
| Full privacy-safe normalization pass | 473.31 ms |
| Repeated benchmark throughput | 1,753.64 rows/second |
| Peak benchmark Python allocation | 42.59 MiB |
| Raw rows or identifiers emitted | 0 |
| Evidence/database/model writes or calls | 0 |

## R7 exit decision

The protected read-only result agrees with the synthetic R7 contracts: schema
routing, canonical party roles, Pakistan service/direction semantics, explicit
timezone basis, deterministic hashes, exact locators and missing-coordinate
behavior all hold. R7.1 through R7.10 are accepted at their governed boundaries.
R7 is complete. Retained ingestion of this protected source was not required and
remains unperformed.
