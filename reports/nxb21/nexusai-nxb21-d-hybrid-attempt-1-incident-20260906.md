# NX-B2.1D — first hybrid development incident

Attempt 1 is INCOMPLETE_FAILURE: five cases completed, four deterministic passes, first residual rejected. Runtime postcheck PASS. The remaining 27 cases were not resumed. All run files, timestamps, sidecars and the dispatch lock were left untouched; complete before/after hashes are in `d-hybrid-attempt-1-evidence-manifest.json`. No new inference, corpus, holdout, qualification, activation or service change was performed during diagnosis.

## Receipt discrepancy: confirmed ordering defect

Current receipt SHA256 and sidecar value are both `38684c2f1e9b1cb128ded3693c1778498480034eec560daf34c0e37fa06f36be`. This is the authoritative current-byte version. The earlier printed `c68bdb939e936453348b023804fbb365ac58d0c767d38048f40e23100c6b3321` refers to an earlier serialization at the same path. The earlier bytes were reconstructed in `tmp/hybrid-incident-first-receipt-reconstructed.json` using Windows PowerShell 5.1 and reproduced that exact hash. Reconstruction did not write into the original run.

Write trace in the dispatched implementation:

1. Go evaluator `seal()` wrote the aggregate after each completed case and again on return through deferred `seal()`. It returned normally on malformed residual; Ginkgo PASS means evaluator process success, not development acceptance.
2. The runner read that aggregate, unloaded its Q4 candidate, completed runtime postchecks, and appended runtime fields.
3. The normal PowerShell path wrote the aggregate and sidecar, selected exit code 22 for the failed gate, and printed the first hash.
4. `finally` saw nonzero exitCode. It repeated baseline checks, read the same file, and used `Add-Member -Force` for development_gate, runtime_postcheck and activation. Those properties moved to the end of the object's serialization order even though their values were unchanged.
5. It rewrote the receipt and sidecar and printed the later hash. Final semantic content was unchanged; bytes and SHA256 changed. This is a real receipt immutability violation, not model-output corruption. Timestamps in the evidence manifest preserve the final write times.

The repaired evaluator writes only `hybrid-development-intermediate-v1.json`. The runner completes cleanup/postchecks before its single finalization path. `Seal-NxHybridReceipt` uses CreateNew for final receipt and sidecar, writes/flushed bytes once, computes their hash once, and returns a seal. Only then is the path/hash emitted. Re-sealing cannot overwrite either file. Repeated output verifies the bytes still match and prints the same seal hash. Failures use the same finalization point. No code path removes the dispatch lock. A crash before sidecar completion leaves an unsealed artifact and fails closed; it is not announced as a sealed result.

## Exact residual evidence

Query: **Give me a roll-up of network use associated with 198.51.100.211.**

Expected tuple `ipdr.endpoint/AGGREGATE/authorized_workspace`; expected ambiguity state RESOLVED, empty clarification code; target `198.51.100.211` remains deterministic.

The fact packet has tenant `hybrid-eval`, user `operator`, collection `synthetic`, workspace scope, no selected evidence, one raw/canonical IP identifier at byte offsets 49–63, no explicit dates/direction/top-k/source time/filters, no prior context, transition NONE. The full exact packet and candidates follow.

```json
{
  "contract_version": "forensics.hybrid-facts/v1",
  "tenant_id": "hybrid-eval",
  "user_id": "operator",
  "collection_id": "synthetic",
  "evidence_id": "",
  "scope": "authorized_workspace",
  "identifiers": [
    {
      "raw_span": "198.51.100.211",
      "canonical": "198.51.100.211",
      "identifier_type": "ip",
      "start_byte": 49,
      "end_byte": 63
    }
  ],
  "target": "198.51.100.211",
  "targets": [
    "198.51.100.211"
  ],
  "family": "",
  "date_from": "",
  "date_to": "",
  "direction": "",
  "top_k": 0,
  "start_seconds": null,
  "end_seconds": null,
  "filters": null,
  "prior_capability": "",
  "prior_semantic": "",
  "prior_targets": null,
  "inherited_fields": [],
  "transition": "NONE",
  "explicit_capability": "",
  "state": ""
}
```

```json
[
  {
    "tuple_id": "cross_family.identifiers/COMPOSE/authorized_workspace",
    "capability_id": "cross_family.identifiers",
    "semantic": "COMPOSE",
    "allowed_scope": "authorized_workspace",
    "family": "case_cross_family",
    "operation_ref": "forensics.cross_family_correlation",
    "text_representation_rule": "",
    "required_fact_types": [
      "scope"
    ],
    "optional_fact_types": [
      "identifiers",
      "dates",
      "direction",
      "top_k",
      "source_time",
      "filters"
    ]
  },
  {
    "tuple_id": "ipdr.endpoint/AGGREGATE/authorized_workspace",
    "capability_id": "ipdr.endpoint",
    "semantic": "AGGREGATE",
    "allowed_scope": "authorized_workspace",
    "family": "network_ipdr",
    "operation_ref": "ipdr.endpoint_summary",
    "text_representation_rule": "",
    "required_fact_types": [
      "scope",
      "identifiers"
    ],
    "optional_fact_types": [
      "identifiers",
      "dates",
      "direction",
      "top_k",
      "source_time",
      "filters"
    ]
  },
  {
    "tuple_id": "ipdr.sessions/FILTER/authorized_workspace",
    "capability_id": "ipdr.sessions",
    "semantic": "FILTER",
    "allowed_scope": "authorized_workspace",
    "family": "network_ipdr",
    "operation_ref": "ipdr.subscriber_sessions",
    "text_representation_rule": "",
    "required_fact_types": [
      "scope",
      "identifiers"
    ],
    "optional_fact_types": [
      "identifiers",
      "dates",
      "direction",
      "top_k",
      "source_time",
      "filters"
    ]
  }
]
```

Exact system prompt:

```text
Choose one server-issued tuple matching the analyst's residual intent. Return only the five-field residual JSON. Never generate facts, parameters, scope, tools or operations. Use NEEDS_CLARIFICATION and an empty tuple if intent is ambiguous.
```

Exact user/model prompt:

```text
Give me a roll-up of network use associated with 198.51.100.211.
Valid tuples:
[{"tuple_id":"cross_family.identifiers/COMPOSE/authorized_workspace","capability_id":"cross_family.identifiers","semantic":"COMPOSE","allowed_scope":"authorized_workspace","family":"case_cross_family","operation_ref":"forensics.cross_family_correlation","text_representation_rule":"","required_fact_types":["scope"],"optional_fact_types":["identifiers","dates","direction","top_k","source_time","filters"]},{"tuple_id":"ipdr.endpoint/AGGREGATE/authorized_workspace","capability_id":"ipdr.endpoint","semantic":"AGGREGATE","allowed_scope":"authorized_workspace","family":"network_ipdr","operation_ref":"ipdr.endpoint_summary","text_representation_rule":"","required_fact_types":["scope","identifiers"],"optional_fact_types":["identifiers","dates","direction","top_k","source_time","filters"]},{"tuple_id":"ipdr.sessions/FILTER/authorized_workspace","capability_id":"ipdr.sessions","semantic":"FILTER","allowed_scope":"authorized_workspace","family":"network_ipdr","operation_ref":"ipdr.subscriber_sessions","text_representation_rule":"","required_fact_types":["scope","identifiers"],"optional_fact_types":["identifiers","dates","direction","top_k","source_time","filters"]}]
```

Exact residual schema:

```json
{
  "additionalProperties": false,
  "properties": {
    "ambiguity_state": {
      "enum": [
        "RESOLVED",
        "NEEDS_CLARIFICATION"
      ],
      "type": "string"
    },
    "clarification_code": {
      "enum": [
        "",
        "AMBIGUOUS_INTENT",
        "INSUFFICIENT_FACTS"
      ],
      "type": "string"
    },
    "confidence": {
      "maximum": 1,
      "minimum": 0,
      "type": "number"
    },
    "contract_version": {
      "const": "forensics.hybrid-residual/v1",
      "type": "string"
    },
    "selected_tuple_id": {
      "enum": [
        "",
        "cross_family.identifiers/COMPOSE/authorized_workspace",
        "ipdr.endpoint/AGGREGATE/authorized_workspace",
        "ipdr.sessions/FILTER/authorized_workspace"
      ],
      "type": "string"
    }
  },
  "required": [
    "contract_version",
    "selected_tuple_id",
    "ambiguity_state",
    "clarification_code",
    "confidence"
  ],
  "type": "object"
}
```

Request: 2569 UTF-8 bytes, SHA256 `172a2e3c391668f594db78b6bfd200297b2897b2faf25358f8143c6a2a6d2394`. Response: 521 bytes, SHA256 `1a58d33650a629ff3bd3f286bd9e94457d9927c210adffc1f701bc46902a0d12`. Exact serialized byte sequences are preserved in the original files and losslessly included as base64 in `d-hybrid-attempt-1-diagnosis.json`; the request UTF-8 text is also included there. HTTP 200 is established by saved `transport_pass=true` and the evaluator's exact status check; a separate raw header capture was not saved.

Strict UTF-8 decoded response:

```json
{"created":1788692163,"object":"chat.completion","id":"2f549b45-58e6-4609-abaf-a6174720e01c","model":"qwen3-4b-instruct-2507-q4km-nxb21d-dev","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"{\"ambiguity_state\":\"NEEDS_CLARIFICATION\",\"clarification_code\":\"\",\"confidence\":0,\"contract_version\":\"forensics.hybrid-residual/v1\",\"selected_tuple_id\":\"ipdr.endpoint/AGGREGATE/authorized_workspace\"}"}}],"usage":{"prompt_tokens":331,"completion_tokens":66,"total_tokens":397}}

```

Model message:

```json
{"ambiguity_state":"NEEDS_CLARIFICATION","clarification_code":"","confidence":0,"contract_version":"forensics.hybrid-residual/v1","selected_tuple_id":"ipdr.endpoint/AGGREGATE/authorized_workspace"}
```

Envelope and inner JSON both parse successfully. All five fields are present, with no extra fields; all transmitted enums, const and numeric bounds are satisfied. `finish_reason=stop`, prompt tokens 331, completion tokens 66, total 397, latency 32,466 ms. No truncation, empty output, surrounding prose, legacy 12-field object, missing field or unknown tuple occurred.

Exact rejection: `decodeHybridResidual` returned **contradictory residual state**. Its clarification branch required NEEDS_CLARIFICATION + empty selected_tuple_id + AMBIGUOUS_INTENT or INSUFFICIENT_FACTS. The response instead paired NEEDS_CLARIFICATION with a valid nonempty endpoint tuple and empty clarification_code. It also failed the RESOLVED branch. The final plan is null. This is a schema-valid but semantically contradictory residual, not invalid JSON.

## Comparison with the latest successful structured path

Reference: v2 `run-20260905T173444320Z/if24b-en-001`, StructuredValid/ModelProposalPassed/Passed=true. Exact old request, response and result are embedded in diagnosis JSON.

| Dimension | Successful v2 path versus first hybrid residual |
|---|---|
| Model | Same qwen3-4b-instruct-2507-q4km-nxb21d-dev, Q4_K_M artifact. |
| Profile | Same frozen SHA 30d3bcd8c92684e191277dad992354323b7e46c7d30d3c69149010e90f117129; 4096 context, eight threads, CPU, tokenizer template/use_jinja. No profile change. |
| Endpoint | Same local /v1/chat/completions (127.0.0.1:8080). |
| Sampling | Both temperature 0 and max_tokens 512. No evidence of context pressure in the failed response. |
| response_format | Both type=json_schema, strict=true. |
| Schema naming | forensic_dynamic_capability_plan became forensic_hybrid_residual. |
| Schema contract | 12-field source-bound plan became five independent fields; the latter did not encode state/tuple/code mutual dependence. |
| Prompts | Old system instructs capability/semantic selection with copied source constants; new system instructs residual tuple choice and no fact generation. Exact prompts in diagnosis JSON. |
| User construction | Both original query plus server allowlist. Old compact capability/semantic descriptions; new JSON tuples include family, operation and required/optional facts. |
| Grammar | Explicit response_format=json_schema path exists in LocalAI chat handling. No grammar compiler error is established by the saved run. Current source Item conversion retains Type/Properties and drops root conditional fields; blindly adding root oneOf is therefore unsuitable. Actual generated runtime grammar was not captured. |
| Transport | Both save byte-preserving UTF-8 request/raw response; hybrid evaluator uses an HTTP proxy for capture. This response passed transport checks. |
| Response decoding | Both decode one completion envelope and JSON content. Hybrid additionally requires exactly five fields and cross-field coherence. |
| Parser | Old proposal decoder/production validator versus decodeHybridResidual. The latter correctly rejects the observed contradiction; it was stricter than the sent schema. |

Primary classification: **B — prompt/schema integration defect (contract mismatch)**. Q4 emitted the contradictory choice and did not obey the empty-tuple instruction, so it is implicated in that observed output. But its tuple value is the expected semantic choice, and one schema-permitted contradiction does not prove inability to understand the query or emit compact JSON. There is no affirmative evidence here of a LocalAI transport/JSON-generation failure or evaluator parsing defect. The deterministic architecture remains valid; four observed deterministic cases passed without model calls. Broader live success is not established.

## Bounded corrections and source tests

The model-facing residual is now one object field, `decision`, with an enum containing only server-issued tuple IDs plus `CLARIFY:AMBIGUOUS_INTENT` and `CLARIFY:INSUFFICIENT_FACTS`. Server decoding derives mutually consistent tuple/state/code fields in the audit under `forensics.hybrid-decision/v2`; model confidence is no longer solicited. Tuple construction, deterministic fact extraction and final assembly are unchanged. The schema name is forensic_hybrid_decision_v2. This generic correction removes contradictory combinations by construction, avoids unsupported root conditionals, and is not tuned to the failed wording. No model, profile, budget or transport change.

The regression enumerates every allowed decision, passes the schema through the same LocalAI Item/Grammar conversion, verifies each enum survives, and decodes every decision into consistent state. It rejects duplicate keys, null/empty/unknown decisions, extra authority fields and the old contradictory five-field shape. Existing generic HTTP mock tests now use the new wire format and still verify deterministic targets/audit separation. Receipt regressions test success and failure, sidecar equality, unchanged bytes and modification time after sealing, repeated identical output, rejected overwrite, one finalization site, and lock survival.

Files changed:

- `api/forensic_records/query_hybrid_planner.go` (SHA256 `c327ea8b2e63ef18b7bcbee923ce9253ed1ff4ae247902cfaadfa9156f0be459`)
- `api/forensic_records/query_hybrid_planner_ginkgo_test.go` (SHA256 `98f55955ebf0d9009ee389ccd17bee662c700aab7069aca432f9f1853498e4a2`)
- `api/forensic_records/query_language_assistance_test.go` (SHA256 `ec8a42706681108dad4e85b6efa90c0fda2abe26d4ffcf9a0b09be2fe793d0d0`)
- `api/forensic_records/nxb21d_hybrid_development_ginkgo_test.go` (SHA256 `606fbbed487f7a1157ccf65a75c68a3fc302ff00f16dc74c1813d8ac5febce8d`)
- `scripts/nxb21d-hybrid-development/run_nxb21d_q4_hybrid_32case_development.ps1` (SHA256 `05f972b785a5082a2cc96196cef7d7f1b23d307a2b8f9f76638539d304aaecf3`)
- `scripts/nxb21d-hybrid-development/immutable_receipt.ps1` (SHA256 `143d28f7b419da73a032aaa754d4279d7245a69eef3e0949d238fc9049d9b067`)
- `scripts/nxb21d-hybrid-development/test_hybrid_incident.ps1` (SHA256 `d12a543baed1b5ed5c271dc7e9c2e4c750f27a116f2868cec26993d8f5c9ae3d`)

Full API source suite: **PASS, 42.242 seconds** (package-reported 42.241 s); Ginkgo results below. Windows PowerShell receipt regression: PASS. No live evaluation was used for testing. Current source hashes intentionally no longer match the historical development freeze. Its receipt/config/corpus were preserved, not silently re-frozen. Historical ValidateOnly is therefore expected to reject current-source drift; it is not a new runnable candidate.

```text
Ran 527 of 556 Specs in 4.145 seconds
SUCCESS! -- 527 Passed | 0 Failed | 0 Pending | 29 Skipped
```

## Next gate and exact next action

A fresh residual-only gate is required before claiming the correction works with Q4. Recommend **8 fresh cases: 2 per language (English, Urdu, Roman Urdu, mixed)**, one resolved tuple and one clarification per language; distribute the four clarification cases across ambiguous intent and insufficient facts. Vary resolved capabilities and candidate order, verify the one-field serialization, correct decision, deterministic facts unchanged, 512-token completion, runtime integrity, and one immutable final receipt. This tests the specific interface correction; it cannot substitute for later complete planner qualification.

No questions, values, corpus, runner for a new gate, or qualification holdout were generated. Exact next action: review this source-only incident closure and authorize preparation/freezing of that fresh 8-case residual gate. Do not run the old command. The current 32-case corpus is RETIRED_AFTER_DISPATCH; the original one-shot lock remains present and unchanged. No resume from case 6, lock bypass, or re-freeze of old questions is permitted.

NX-B2.1D
Q4_FULL_PLAN_ROLE=RETIRED
Q4_HYBRID_ROLE=HYBRID_PROMISING
DETERMINISTIC_FIRST_HYBRID=SOURCE_VALIDATED
HYBRID_DEVELOPMENT_ATTEMPT_1=INCOMPLETE_FAILURE
CASES_COMPLETED=5
DETERMINISTIC_PASS=4_OF_4
FIRST_RESIDUAL_RESULT=MALFORMED_RESIDUAL
RUNTIME_INTEGRITY=PASS
CURRENT_32CASE_CORPUS=RETIRED_AFTER_DISPATCH
FINAL_REPLACEMENT_QUALIFICATION_CANDIDATE=NOT_YET
REPLACEMENT_HOLDOUT=NOT_CREATED
D_STATUS=OPEN
ACTIVATION=BLOCKED
