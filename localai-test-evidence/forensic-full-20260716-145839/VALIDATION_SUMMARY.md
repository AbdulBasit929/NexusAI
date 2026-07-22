# Forensic Full Validation Summary

Date: 2026-07-16
Evidence folder: D:\Projects\LocalAI\localai-test-evidence\forensic-full-20260716-145839
Validation collection: records-validation-20260716-145839-fix

## Passed

- Focused Go tests: api/forensic_records, core/services/agents
- Worker adapter tests: unittest discovery under ingestion/forensic_records/tests
- Docker stack status: Postgres healthy, NATS healthy, records API up, records worker up
- Records API health: /healthz
- Template registry includes case_readiness and all deterministic templates
- Isolated diverse ingestion: 8 completed jobs, 1,781 accepted rows, 0 rejected rows, 0 failed jobs
- Record families validated: CDR, ANPR, IPDR, transaction, subscriber, tower_location, access_log, generic
- KB collection create and document upload
- KB collection search with auto-cap metadata
- Hybrid analytics matrix: overview, source files, data quality, readiness, shortest call, call type, top locations, ANPR, entity activity, timeline, source records, evidence, evidence package, clarification
- Report generation with Case Readiness and Knowledge Base Evidence
- Final forensic smoke script against isolated validation collection
- Embeddings endpoint: granite-embedding-107m-multilingual returned 3 vectors, 384 dimensions

## Fixed During Validation

- Records API schema profile SQL used jsonb_object_length; fixed for this Postgres stack using jsonb_object_keys.
- Worker subscriber adapter did not accept subscriber_name/cnic_last4/home_city aliases; fixed.
- Worker CDR parser rejected ISO timestamps with Z; fixed by using multi-format timestamp parsing.
- Worker failed adapter-resolution jobs could be hidden from collection status; fixed by ensuring job row exists before marking failure.

## Blocked / Failed

- Direct /v1/chat/completions with qwen3-0.6b timed out at 240s and tiny warmed generation timed out at 90s. Logs show heavy CPU/memory use and delayed HTTP 200 around the timeout boundary, so this is a LocalAI/model runtime performance issue under current hardware/config, not a forensic records failure.
- Streaming chat depends on the same qwen3-0.6b runtime and timed out.
- Vision, image generation, audio, VAD, detection, face recognition, and diarization were not run because only qwen3-0.6b and granite-embedding-107m-multilingual are installed.

## Notes

- No volumes were deleted.
- No model files were removed.
- API and worker were rebuilt/restarted only after targeted fixes.
- Validation used an isolated collection rather than modifying records-demo.
