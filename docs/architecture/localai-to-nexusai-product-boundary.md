# LocalAI to NexusAI product boundary

Status: approved implementation boundary reconciled 2026-08-06

## LocalAI remains the engine

Retain model loading/backends, compatibility APIs, embeddings/KB primitives,
agent runtime, MCP/tool infrastructure, auth/feature middleware, distributed
building blocks, backend gallery, telemetry, and the React delivery shell where
they are technically useful.

## NexusAI owns the product

NexusAI owns tenants/cases, evidence/version/custody identity, family adapters,
deterministic operations, specialist policies, model-role selection, professional
responses, case-centered UI, reports/audit, capability truth, security posture,
and customer-facing `/api/v1/forensics` contracts.

```text
NexusAI UI / customer API
  -> governed tenant + case contract
  -> capability-aware planner
  -> deterministic family tools / cited retrieval
  -> optional bounded LocalAI model explanation
  -> validated NexusAI response, audit, export

LocalAI engine
  -> model/backend lifecycle, embeddings, agent/MCP primitives,
     compatibility APIs, distributed/runtime infrastructure
```

## Rename policy

- Rebrand product-facing copy, titles, assets, primary navigation, reports, and
  analyst terminology.
- Preserve `LocalAI`, `local-ai`, package/module names, API compatibility names,
  container/image identifiers, configuration keys, and legal attribution where
  renaming would break compatibility or licensing truth.
- Keep engine administration accessible only to authorized technical users.

## Enforcement rules

The boundary applies to state as well as naming:

- LocalAI owns inference compatibility, backend/model lifecycle, vendor APIs,
  distributed primitives, generic agent/KB/MCP machinery, and engine telemetry.
- NexusAI owns tenant/case scope, evidence authority, deterministic operations,
  specialist policy, citations, analyst workflows, reports, custody and
  capability truth.
- Browser-local state may hold preferences and unsaved drafts only. It may not
  select a case, collection, tenant, evidence version, or authoritative result.
- A LocalAI capability can be retained in administration while absent from the
  analyst product. Source presence, installation, configuration, live status,
  testing and acceptance are six separate claims.
- Compatibility identifiers may remain `localai` in packages, routes, storage
  keys, environment variables, images and legal references. Product-facing
  identity is governed by R2/R3 and must never be changed through blind search
  and replace.

1. Exact facts and locators come from deterministic stores/tools.
2. Every case route and request uses one authoritative case ID.
3. Unsupported modalities return typed unavailable/manual-review states.
4. Models cannot grant permission, change scope, or rewrite citations.
5. New admin REST controls require MCP parity.
6. Destructive evidence, collection, model, backend, database, or runtime changes
   remain explicit approval gates.
