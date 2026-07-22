# Records Handling Policy

Collection records-demo is a controlled evidence workspace. Structured CDR, ANPR, IPDR,
subscriber, tower, transaction, and access-log files must be ingested through the Records
pipeline for exact analytics. Knowledge Base retrieval is used for policy context, source
previews, case notes, and citations. Exact counts, durations, relationships, and timelines
must come from deterministic records queries.

Retention rule: raw uploads are retained as evidence; derived analytics may be regenerated
from source files. Duplicate uploads should be flagged by SHA-256 and should not create a new
batch unless an analyst explicitly forces a re-ingest for audit testing.
