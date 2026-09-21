---
title: "Dynamic forensic query planning"
---

The dynamic planner maps an analyst question to the existing forensic query
capability and operation contracts. It does not execute model-generated SQL,
tools, URLs, or shell commands.

Planning uses three bounded layers:

1. Deterministic extraction preserves quoted literals, phone/IP/plate and other
   identifiers, explicit calendar and recording times, selected/workspace scope,
   direction, and result limits.
2. When deterministic routing cannot resolve an English question, a configured
   planning model receives the meanings of every scope-compatible,
   non-engineering operation in the 79-operation registry. The
   `forensics.semantic-operation-proposal/v1` response contains only one
   server-enumerated decision. This supports paraphrases by intent, requested
   measure, grouping, result shape, and family instead of requiring a memorized
   sentence template.
3. The existing 22-capability hybrid residual chooser remains a compatibility
   fallback, including for the currently covered multilingual cases. The server
   binds all deterministic facts and then runs the existing governed plan,
   validation, and execution path.

At the agent boundary, only explicit or provably recognized forensic operations
use the deterministic fast path. Other wording reaches the assistant request
router. Investigation, follow-up, evidence/RAG, case-state, processing,
model-availability, and computed-finding claims require governed forensic tools;
greetings, product help, and relevant general knowledge may be answered without
a tool but may not assert facts about the current case. This prevents a general
question from being silently coerced into case analytics while preserving
open-ended investigative tool use.

The semantic operation proposal cannot contain tenant, user, case, collection,
evidence scope, identifiers, literals, calendar/media time, filters, limits,
tools, URLs, SQL, or facts. Deterministic extraction distinguishes calendar time
from media source time and preserves exact literal/identifier authority. A
proposal is rejected when it selects a non-issued or out-of-scope operation,
contains an extra field or trailing content, or cannot be bound to valid server
facts and parameters.

Typed `forensics.query-plan/v1` requests are also checked against the selected
executor before lowering. Non-empty `measures` and `group_by` values are
assertions and must match the registered operation. Filters and sorts are
accepted only when that executor consumes them: canonical/generic source-row
operations support the allowlisted JSONB predicate set and one allowlisted
deterministic sort; frequent contacts accepts only an exact direction filter;
retained-video timelines accept only their declared inclusive source-time
bounds. Unsupported or silently ignored algebra is rejected instead of being
approximated.

Filters and sorts must be arrays containing only typed objects. Measures and
grouping must be arrays of non-empty strings. Malformed entries and additional
filter/sort controls are rejected before compatibility conversion. A scalar
direction or video-bound parameter cannot consume repeated predicates, including
aliases for the same bound; such requests are rejected. Canonical row predicates
retain conjunctions, including lower and upper bounds on the same field.
Ad-hoc projection, time buckets, comparisons, and step composition remain
unsupported by this plan version and cannot be silently requested through their
corresponding control fields.

Explicit phrases such as `this recording` request selected-evidence scope;
`across the whole investigation` requests the already-authorized workspace.
Contradictory scope wording requires clarification. Query wording never changes
the tenant, user, case, or collection authorization.

Unsupported formats and unavailable processing requests return a typed
unavailable result. This includes TTS generation while TTS is disabled, identity
recognition through face similarity, arbitrary execution, and formats without a
qualified analytical decoder.

The source contract and tests prove schema, registry, authority, scope, and
fallback behavior. They do not prove open-ended English or multilingual model
accuracy, deployment parity, or real-world analytical result correctness. Those
require a fresh representative model qualification, live activation acceptance,
and the independent operation certification program.
