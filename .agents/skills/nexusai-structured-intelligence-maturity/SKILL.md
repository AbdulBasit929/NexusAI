---
name: nexusai-structured-intelligence-maturity
description: Mature NexusAI structured and textual forensic intelligence through bounded STIM slices. Use for structured forensic ingestion; CDR, IPDR, subscriber/identity, tower/site/cell/location, structured ANPR, and other existing structured evidence families; schema profiling; provider/export adapters; source-field mapping and preservation; canonicalization; dynamic attributes; data-quality accounting; certified operations and independent oracles; multi-file, multi-CDR, or cross-family correlation; English, messy English, Roman Urdu, Urdu, mixed-language, and follow-up query semantics; validated Fact Packets and grounded answer synthesis; structured citations, presentation, UI/UX, planning, testing, or acceptance. Do not use for unrelated NexusAI work, deployment-only or generic DevOps tasks, general frontend work, media model/OCR/ASR/video/image qualification, R8/CCPD, or unrelated LocalAI modules.
---

# NexusAI Structured Intelligence Maturity

## Purpose

Advance the existing NexusAI system through the active STIM phase without reopening APF-3, duplicating accepted architecture, or starting major media work. Optimize for truth, provenance, analytical correctness, Pakistan relevance, analyst clarity, security, and performance.

## Required workflow

1. Read `NEXUSAI_CONTINUATION.md` and `docs/design/nexusai-family-adapter-agent-api-architecture.md` before changing production behavior.
2. Reconcile authority in this order: latest explicit product-owner instruction, `NEXUSAI_MASTER_DIRECTIVE.md`, `AGENTS.md`, relevant `.agents/*.md`, this skill, current roadmap, phase ledger, continuation checkpoint, current maturity matrix, source, and tests. Treat source truth as controlling over stale reports.
3. Identify the active STIM phase and keep the slice bounded. Do not reopen APF-3; classify inherited defects under the active STIM phase unless they are genuine P0 incidents.
4. Trace the affected path end to end before editing: raw source, admission, preservation, detection/classification, profiling, mapping, normalization, validation, duplicate/rejection accounting, persistence, extension attributes, indexing/readiness, query understanding, planning/execution, analytics, relationships, provenance, Fact Packet, optional synthesis, validation, presentation, and UI.
5. Apply the reuse ladder: reuse unchanged, configure, extend an adapter, extend a contract, add compatibility, add a bounded module, and introduce a service only when unavoidable.
6. Define the semantic contract, Pakistan-specific assumption, independent oracle, downstream calculations, potential field/identifier/time loss, LLM boundary, presentation risk, severity, and phase fit before implementation.
7. Preserve raw values, exact identifiers, source locators, versioned mappings, and evidentiary strength. Never treat unknown as zero or co-occurrence as identity/ownership.
8. Add proportionate tests from the STIM pyramid and compare expected versus actual results independently. Production helpers cannot generate their own oracle.
9. Update the single roadmap, phase ledger, maturity matrix, issue register, and `NEXUSAI_CONTINUATION.md` after each meaningful phase. Do not create competing trackers.
10. Stop for approval before model/dataset/backend downloads, database migrations, backfills, retained uploads or reprocessing, destructive cleanup, model/profile changes, deployment/rebuild, staging, commit, push, or PR creation.

## Reference routing

- Read [authority-and-precedence.md](references/authority-and-precedence.md) for governance, safety, reuse, approval, and ledger rules.
- Read [structured-family-semantics.md](references/structured-family-semantics.md) for family and relationship semantics.
- Read [pakistan-domain-profile.md](references/pakistan-domain-profile.md) for Pakistan-first identifier, phone, locale, and INPR/INPRS rules.
- Read [ingestion-normalization-contract.md](references/ingestion-normalization-contract.md) for the structured source-to-readiness path, time semantics, schema profiling, and quality accounting.
- Read [dynamic-attributes-contract.md](references/dynamic-attributes-contract.md) when mapping or preserving source-native fields.
- Read [certification-and-independent-oracles.md](references/certification-and-independent-oracles.md) for operation contracts, goldens, and test levels.
- Read [multi-source-correlation.md](references/multi-source-correlation.md) for multi-file/CDR and cross-family work.
- Read [query-language-quality.md](references/query-language-quality.md) for multilingual, messy-query, follow-up, and clarification work.
- Read [grounded-llm-synthesis.md](references/grounded-llm-synthesis.md) for Fact Packets, model boundaries, validation, fallback, and benchmarks.
- Read [ui-answer-presentation-contract.md](references/ui-answer-presentation-contract.md) for Data, Ask, History, citations, responsive, and accessibility work.
- Read [phase-gates-and-triage.md](references/phase-gates-and-triage.md) for STIM phase boundaries, P0-P3 triage, and acceptance gates.

Load only the references relevant to the current slice, but always load authority, the affected family/contract reference, and phase gates before a production change.

## Permanent boundaries

### vNext product sequencing overlay — 2026-08-25

- After foundation-truth closure, advance **breadth before depth**. Structured,
  document, image, audio, and video families must progress together to a useful
  product baseline before deep operation catalogs or dynamic query intelligence.
- A family baseline is end to end: recognize, ingest/process, preserve
  provenance, produce facts or explicitly typed observations, assign a domain
  agent/tool, expose one or a few useful operations, support positive and
  zero/negative Ask paths, cite/open the source, record Activity, measure basic
  performance, and demonstrate on lawful controlled or representative evidence.
- Build the standalone analyst utility before broad family expansion. It has a
  neutral temporary identity such as **Investigation Workspace**, a low-training
  persona, and one Overview/Evidence/Ask/Activity flow. Do not inherit LocalAI or
  NexusAI dashboard complexity and do not create family mini-applications.
- Backend terms—including models, processors, agents, operations, MCP,
  FactPacket, routes, templates, jobs, hashes, and traces—stay out of the normal
  analyst flow and appear only under progressive Details/Technical disclosure.
- Expose no fake, placeholder, dead, unsupported, or uncertified normal-flow
  control. Every exposed family function must really execute and preserve
  truthful result state, citations, and Activity.
- Domain agents orchestrate multiple typed tools. They never own truth, call
  peers directly, authorize scope, invent IDs, issue unrestricted SQL, or turn
  model confidence into a fact. Deterministic values remain facts; OCR, ANPR,
  ASR, face, similarity, detection, and VLM output remain observations unless
  independently verified.
- Reuse LocalAI selectively for inference and protocol infrastructure. NexusAI
  retains evidence truth, scope, policy, deterministic operations, packets,
  result semantics, citations, Activity, and certification.
- The controlling sequence is NX-1 → NX-UX1 → NX-A1 → NX-B1 → NX-B2, followed
  by justified structured, media, document/language/retrieval, dynamic-query,
  cross-family, enterprise, and advanced depth phases.

- Keep deterministic analytics and server-authoritative identifiers separate from model narrative.
- Keep format decoding separate from evidence-family semantics.
- Keep one authorized case/collection scope through upload, query, evidence, history, and presentation.
- Keep country-specific behavior centralized and versioned; remain extensible beyond Pakistan.
- Do not claim format or language coverage that fixtures and acceptance evidence do not prove.
- Do not perform database migrations or retained-state changes without explicit approval.
- Do not download or qualify new media models during STIM.
