# NexusAI information architecture

Status: superseded for ordinary-user navigation by the APF rebase, 2026-08-17.
The advanced Case Workspace structure below remains accepted. See
`nexusai-analyst-portal-product-architecture.md` for the controlling two-surface
product boundary.

## Advanced investigation navigation

1. Home
2. Cases
3. Ask NexusAI
4. Evidence
5. Relationships
6. Timeline
7. Media
8. Reports

The current reference UI implements Cases with Overview, Evidence, Analyze,
Relationships, Jobs, and Settings. Timeline, Media, and immutable Reports are
shown only after backend capability and data gates exist.

## Ordinary analyst portal

The default ordinary-user product is the separate `/analyst` shell:

1. Home
2. Data
3. Ask NexusAI
4. History

It reuses the same authenticated case/collection authority, APIs, citations and
audit history. Agent, model, backend and collection implementation details are
not primary navigation.

## Administration

Agents, Models, Backends, Knowledge/Collections, Audit, Users/Roles, System
Health, and deployment controls belong under explicit technical administration.
Ordinary analysts should not navigate raw LocalAI engine surfaces or agent-name
collections.

## Context contract

Every case route carries tenant, case/collection, user/role, classification,
permissions, evidence coverage, jobs/readiness, and model-role availability.
The URL case, authenticated scope, request body, specialist binding, report, and
export must agree; conflicts fail closed.

## Task flow

```text
select governed case
  -> inspect readiness/evidence
  -> ask natural question or choose capability-aware shortcut
  -> deterministic/retrieval plan
  -> professional typed answer
  -> open citation/evidence
  -> save/export/report with audit
```

Compatibility routes may redirect into this flow but must not create a second
case selector or silently use a default collection.

The top-level “Ask NexusAI” label is a product destination only after R6 begins.
Before then, navigation must not imply the complete Ask workflow is operational.
R3 establishes the shell, R4 the case sections, R5 the Evidence workflow and R6
the central Ask experience in that order.
