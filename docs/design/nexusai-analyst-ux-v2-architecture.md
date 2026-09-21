# Standalone Investigation Workspace UX architecture

> **Authoritative naming/sequencing update — 2026-08-25:** this is a separate
> analyst utility backed by NexusAI services. Normal UI uses the neutral temporary
> name **Investigation Workspace**, not NexusAI or “Ask NexusAI”. Prefer
> Overview/Evidence/Ask/Activity. Existing `/analyst/*` routes may remain as
> compatibility paths until route change is justified.

## Product principle

The analyst sees evidence, findings, uncertainty, and actions—not internal
routes, templates, model plumbing, or orchestration vocabulary. Technical detail
remains available on demand for audit and engineering users.

## Information architecture

- **Overview:** current case, readiness, recent evidence, recent analyses, suggested
  governed actions, and alerts requiring analyst review.
- **Evidence:** source inventory, ingest/processing state, quality, searchable fields,
  provenance, and family-appropriate actions.
- **Ask:** plain-language question, scope preview, typed answer state, findings,
  citations, limits, and suggested follow-ups.
- **Activity:** analyst-friendly analysis titles, family/state filters, exact
  scope, reopening with context, and export/audit access.

The common four-area task flow remains, but the utility receives an independent
visual hierarchy, interaction model, terminology, and design system rather than
inheriting the NexusAI or LocalAI dashboard shell.

## Ask result composition

1. State banner: findings, complete with zero results, no match, processing,
   unavailable, unauthorized, failed, or invalid.
2. Direct answer in analyst language.
3. Key findings with no duplicated fact cards.
4. Evidence citations that open the exact source/artifact/record.
5. Quality, contradictions, and limitations.
6. Suggested governed next questions.
7. Collapsed technical details: operation ID/version, route, plan, model, packet,
   timings, and audit ID.

Completed-zero is successful analysis, not generic completion or failure.
Unavailable is not no-match. Candidate observations are visually and verbally
distinct from certified facts.

## Data-source detail

Use Overview, Findings, Fields, Quality, Capabilities, and Technical sections,
but derive all readiness wording from a single status contract. Media replaces
misleading generic record counts with artifact-appropriate measures: duration,
frames/samples, transcript segments, OCR observations, plate candidates, and
accepted/rejected observations. Historical immutable notes are labeled and
collapsed when they conflict with current readiness.

## Responsive and multilingual behavior

Primary acceptance widths are 390, 820, 1024, and 1440 px. Tables become cards
or controlled horizontal regions; the document never overflows. Urdu uses
component-level `dir=rtl`, mirrored spatial layout where appropriate, correct
number/identifier isolation, and preserved evidence strings. Mixed Urdu/English
and Roman Urdu receive explicit tests. Keyboard, focus, screen-reader names,
contrast, zoom, empty/error/loading states, and reduced motion are gates.

## Permissions and safety

The client receives already-filtered capabilities. Hidden/disabled actions are
explained without revealing inaccessible evidence. Destructive or retained
analysis actions show scope and consequences and require appropriate approval.
No admin MCP/model/backend controls appear in analyst navigation.
