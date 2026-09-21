# NX-MMR evidence routing implementation and certification

## Implementation

The bounded source slice extends the existing classification, queue and
registration path; it does not create a second ingestion system. Every upload
can now produce `nexusai-evidence-route-plan/v1` with:

- evidence/version references, deterministic plan ID and shadow-only admission;
- bounded magic/signature and MIME signals, with extension/client MIME retained
  only as untrusted hints;
- primary family, explicitly requested family-applicable composed roles,
  required capabilities and resolved processor/model readiness;
- LIGHT/MEDIUM/HEAVY resource estimate using the approved floors;
- scope/security result, confidence, fallback/manual-review reason and
  limitations.

The persisted receipt uses existing evidence JSON metadata. Existing queue
behavior is unchanged; automatic execution is not activated or deployed.

## Safe states

`ROUTABLE_READY`, `ROUTABLE_DEGRADED`, `MODEL_REQUIRED`,
`PROCESSOR_UNAVAILABLE`, `MANUAL_REVIEW` and `REJECTED_SECURITY` remain
distinct. A missing model/worker never becomes a completed zero-result.

## Sealed source certification

The 16-case routing decision matrix passes 16/16 (1.000 fixture decision
accuracy, above the 0.98 target). It covers renamed image/video, MIME mismatch,
extension conflict, empty/truncated/malformed inputs, password protection,
polyglot, archive-bomb signal, huge dimensions, unsupported codec, unknown
binary, Urdu/mixed-RTL filename, unauthorized scope and duplicate content.
Additional tests prove:

- a generic image selects only image metadata unless applicable roles are
  explicitly requested;
- image requests cannot silently select ASR;
- model missing, processor unavailable and worker unhealthy remain truthful;
- deterministic structured-text corroboration crosses the 0.98 threshold;
- route receipts are deterministic and round-trip safely.

Some hostile-container facts (password, expansion ratio, codec support) enter
from the existing bounded inspectors; this slice tests their route contract and
does not claim a new archive/codec parser. Certification is therefore
`FIXTURE_CERTIFIED_SHADOW_ONLY`, not product activation.

