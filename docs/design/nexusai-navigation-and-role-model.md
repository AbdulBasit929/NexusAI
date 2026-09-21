# NexusAI navigation and role model

Status: R2 source accepted  
Updated: 2026-08-06

## Target navigation

| Area | Primary destinations | Default audience |
| --- | --- | --- |
| Work | Home, Cases | all authenticated users |
| Active case | Overview, Ask NexusAI, Evidence, Relationships, Timeline, Media, Reports | users with case access and feature permission |
| Governance | Audit, review queues | supervisors/auditors |
| System Administration | Users/Roles, Evidence Processing, Agents/Skills, Models/Backends, Knowledge, Nodes/Storage, Usage/Health/Traces, Settings | authorized administrators |
| Developer | API explorer, instructions, compatibility details | authorized integrators/admins |

Raw collections, backend galleries, model editors, P2P, node scheduling and
engine traces are not ordinary analyst destinations.

## Target roles

| Role | Case data | Mutations | Administration |
| --- | --- | --- | --- |
| Analyst | assigned cases; query and cite | save analysis; approved report draft | none |
| Case supervisor | governed cases; review | case membership/status and report approval within policy | case governance only |
| Evidence operator | assigned evidence and jobs | register/reprocess through approval-aware controls | processing operations only |
| Auditor | read-only case/audit/report scope | none | read-only audit/export as policy permits |
| System administrator | infrastructure metadata; evidence access only if separately granted | system configuration under audit | users, models, backends, agents, storage and health |
| Integrator/service account | scoped API resources | idempotent contract-defined actions | none unless explicitly granted |

These are target product roles. Current source primarily exposes user/admin plus
feature flags; R16 must implement and prove the finer permissions. UI labels
must not imply that unimplemented roles are already enforced.

For R3, an authenticated non-admin is presented as **Analyst** and an enforced
admin as **System Administrator**. Case supervisor, Evidence operator, Auditor,
and Integrator/service account remain target R16 roles and are not shown as
enforced identities until their server authorization exists.

## Authorization composition

Access is the intersection of authenticated tenant, role permission, route
feature, case membership, evidence classification, capability state and
operation policy. Denial at any layer fails closed. Hiding navigation is not
authorization; the server repeats all checks.

## Active-case navigation

The canonical route is `/app/cases/:caseId/:section`. Within it, section links
retain `:caseId`. Deep citations and saved analyses carry stable case-bound
identifiers. A compatibility route may redirect to a governed case selector but
may not choose a hidden default case or collection.

## Current-source disposition

- Reuse `RequireAuth`, `RequireAdmin`, `RequireFeature` and server feature
  middleware as the compatibility baseline.
- Move the current Build/Operate engine consoles under explicit System
  Administration during R3/R4 migration.
- Use **Analyst Tools**, **Intelligence Tools**, **Cases**, **Evidence**,
  **Knowledge**, and **System Administration** as the default English product
  terminology. Existing generic Chat remains **Chat** until R6 implements the
  governed Ask NexusAI lifecycle.
- Keep account/theme/language available to the signed-in user.
- Do not expose Timeline, Media or Reports as operational until their backend
  capability states are truthful.
