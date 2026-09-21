# NX-MMR evidence auto-routing design

## Decision

Auto-routing is a deterministic evidence classifier with model-assisted
fallback, not an LLM guessing the processor from a filename. It produces a
versioned route plan and never silently substitutes a weaker role.

## Routing sequence

1. Authorize tenant, case, collection and upload permission before inspection.
2. Stream to bounded content-addressed storage while calculating SHA-256 and
   enforcing size/bomb limits.
3. Identify container and MIME from magic bytes; treat extension and browser
   MIME as hints only.
4. Extract bounded container metadata without executing embedded content.
5. Select one primary evidence family and zero or more explicit composed roles.
6. Resolve only installed, admitted processors whose capability/version/health
   match the route requirements.
7. If confidence is below the auto threshold, return review-required choices;
   do not process speculatively.
8. Persist the route decision, classifier version, input signals, processor
   role/version and limitations before queueing.
9. Each derived artifact retains parent evidence/version, source locator,
   processor/model hash and observation-versus-fact authority.

## Core mappings

| Detected evidence | Primary route | Optional composed roles |
|---|---|---|
| CSV/TSV/JSONL/XLSX | structured profiling/adapter selection | KB preview only after canonical rows |
| PDF/DOCX/PPTX/TXT | document parser | OCR for raster pages; KB after cited passages |
| raster image | image metadata | ANPR, OCR, face candidate, embedding only when admitted |
| audio | audio metadata/ASR | language guidance, VAD/diarization only when admitted |
| video | video metadata/timeline | bounded frames → image/ANPR/OCR; audio track → ASR |
| PCAP/PCAPNG | capture inventory | supported structured extraction; otherwise manual review |
| SQLite/database export | schema/inventory | approved adapter only; never execute untrusted SQL |
| archive | safe archive inventory | recurse only within count/depth/ratio limits and create child evidence versions |
| unknown/polyglot/encrypted | manual review | no model invocation |

## Contract

`EvidenceRoutePlan` contains route-plan ID/version, evidence/version IDs,
detected MIME/container, signals and confidences, selected family, composed
roles, required processor capabilities, resolved processor/model identities,
resource estimate, security checks, fallback state, and human-review reason.

The following states remain distinct:

- `ROUTABLE_READY` — admitted installed processor is healthy;
- `ROUTABLE_DEGRADED` — explicitly accepted limited processor;
- `MODEL_REQUIRED` — software route exists but model is absent;
- `PROCESSOR_UNAVAILABLE` — role is admitted but runtime is unhealthy;
- `MANUAL_REVIEW` — ambiguous/unsupported/sensitive;
- `REJECTED_SECURITY` — hostile or policy-disallowed input.

## Confidence and fallback

Automatic queueing requires route precision at least 0.98 on the sealed matrix,
all mandatory security checks passing, and a single role above its threshold.
Ties, polyglots, password protection, nested archives, malformed containers,
or unavailable required models become manual review. A general VLM is never a
fallback for deterministic parser, plate text, exact OCR, identity, count, or
timestamp authority.

## Certification

Routing tests cover extension/MIME mismatch, renamed media, empty files,
truncation, polyglots, zip bombs, encrypted archives, malformed PDFs, huge
dimensions, duplicate content, Urdu filenames, mixed RTL/LTR, unsupported
codecs, absent models, worker outage, and retry idempotency. Product acceptance
also proves Data status, Activity event, retry/cancel, source navigation and
truthful unavailable states.

Current source already contains bounded evidence classification and queue
contracts. NX-MMR does not replace them; this design is the target for the next
vertical routing slice after model/data approval.

