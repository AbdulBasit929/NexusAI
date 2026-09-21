# NX-B2.1D selector fairness audit

This one-time audit uses only the saved request bytes from the consumed Qwen and
Phi runs. It performed no inference, rerun, tuning, model load, or runtime
mutation.

RENDERED_PROMPT_TOKENS_APPROX=583–647, using
`ceil(rendered UTF-8 prompt characters / 4)` across the saved selector requests.

CANDIDATE_RENDERING=JSON array of five compact server-issued descriptors:
`id`, `family`, `intent`, `meaning`, optional `measures`/`group_by`, and
`result_kind`.

DYNAMIC_OPTION_POSITION=1 of 8 enum values. The remaining first-level choices
are CLARIFY, UNSUPPORTED, and the five issued registered operations.

INSTRUCTION_BIAS_TOWARD_DYNAMIC=yes. The enum lists `DYNAMIC_TYPED_PLAN` first
and the system message gives it an explicit positive instruction: “Choose
DYNAMIC_TYPED_PLAN when the request is analytically valid … but none of the
issued named operations fully represents it.” Registered operations receive the
more general choose-the-best instruction, without the symmetric “prefer a
specifically designed registered operation” rule.

DECODING_CONSTRAINT=strict `json_schema`.

OUTPUT_SCHEMA_SHAPE=one required flat semantic field (`decision`), eight enum
values, no semantic nesting, 96 output tokens. The JSON Schema wrapper has four
object levels. The 12-field nested/512-token failure observed later belongs to
the secondary dynamic typed-plan request; it is not the selector schema.

CONTRACT_DEFECT_FOUND=yes. Enum ordering and asymmetric instructions create an
avoidable dynamic bias, consistent with Qwen selecting dynamic for all five
registered cases. This finding does not reopen the model-selector workstream:
the 13–180 second CPU latency and failed selection results remain decisive.

The machine-readable receipt is `selector-fairness-audit-v1.json`. Saved input
file hashes and the first rendered prompt hash are recorded there.
