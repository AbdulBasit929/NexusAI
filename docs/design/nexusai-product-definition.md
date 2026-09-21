# NexusAI product definition

Status: R2 source accepted  
Updated: 2026-08-06  
Authority: `NEXUSAI_MASTER_DIRECTIVE.md`

## Product statement

NexusAI is an enterprise forensic-intelligence product powered by local and
private AI infrastructure. It gives authorized analysts one governed
case-centered environment to ingest, preserve, normalize, search, correlate,
question, cite, review and report on heterogeneous evidence.

LocalAI is the underlying compatibility and inference engine. It is not the
ordinary analyst product, the authority for forensic facts, or a substitute for
case, evidence, provenance and policy controls.

## Primary users and jobs

| User | Primary job | Product outcome |
| --- | --- | --- |
| Analyst | Understand one case and answer evidence-grounded questions | Exact, cited, scoped findings with visible limitations |
| Case supervisor | Govern case access, readiness, review and reporting | Consistent scope, reviewable decisions and defensible outputs |
| Evidence operator | Register, process, reconcile and reprocess evidence safely | Exact accounting, lineage, custody and actionable failures |
| Auditor | Inspect access, processing, query and export history | Immutable, attributable records without evidence mutation |
| System administrator | Operate users, models, backends, agents, storage and health | Secure, observable, approval-controlled infrastructure |
| Integrator | Use stable APIs and webhooks | Versioned, idempotent, case-bound contracts |

## Product promises

1. **Evidence first.** Original evidence, versions, locators and custody remain
   visible and preserved.
2. **Truthful capability.** Installed, configured, live, tested and accepted are
   never treated as synonyms.
3. **Deterministic authority.** Exact totals, joins, timelines and citations
   originate in validated adapters and operations. Models may plan, retrieve,
   summarize or explain, but cannot silently replace evidence facts.
4. **One case scope.** URL, authenticated tenant, request, collection,
   specialist, result, report and export must agree or fail closed.
5. **Private operation.** The default product assumes local or
   enterprise-controlled infrastructure; any external service is explicit and
   policy-gated.
6. **Professional uncertainty.** Missing, unmatched, ambiguous, low-confidence
   and unsupported states remain visible.

## Product layers

| Layer | NexusAI responsibility | LocalAI responsibility |
| --- | --- | --- |
| Analyst experience | Cases, Evidence, Ask, Relationships, Timeline, Media, Reports | React/runtime primitives reused under NexusAI shell |
| Forensic services | adapters, typed operations, specialists, citations, reports | generic agents, KB/RAG, MCP and model invocation |
| Evidence control plane | custody, lineage, jobs, canonical facts, tenant/case policy | storage/messaging primitives where appropriate |
| AI engine | role policy, benchmark and acceptance | model/backends, compatibility APIs, streaming, loading |
| Operations | NexusAI administration, audit, release gates | engine admin, telemetry, nodes and backend lifecycle |

## Capability state language

Product surfaces may report only: `operational`, `processing`,
`processing_required`, `no_case_data`, `manual_review`,
`disabled_by_policy`, `administrator_only`, `unavailable`, or
`unknown`. A capability is `operational` only when its required source,
deployment, runtime, representative-data and policy gates pass.

## Non-goals

- A generic chat skin over LocalAI.
- Blind white-label replacement of compatibility identifiers.
- Model-generated evidence facts, identities, routes or certainty.
- Placeholder dashboards that imply unavailable processing.
- A big-bang rewrite of the accepted forensic baseline.

## R2 accepted decisions

The default code-native mark/horizontal lockup and served favicon share the
existing branding configuration with login/report/export fallbacks. The primary
tagline is identity-led rather than global; the forensic line is contextual.
R3 presents only currently enforceable Analyst/System Administrator identities,
while the finer target roles remain R16 work. Evidence, Knowledge and System
Administration are the accepted product terms. See the accepted brand and
navigation contracts for the full surface rules.
