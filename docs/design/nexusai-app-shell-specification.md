# NexusAI app-shell specification

Status: R2 source accepted; implementation belongs to R3  
Updated: 2026-08-06

## Shell contract

The shell establishes identity, authenticated user, role/capabilities, optional
active case, global operations and navigation. It never invents evidence state
and never substitutes a default collection when case context is absent.

```text
document metadata and skip link
  └─ authenticated application shell
      ├─ primary navigation / responsive drawer
      ├─ global header: page, active case, classification, user
      ├─ operation/status region
      ├─ route content and route-level state
      └─ restrained footer: version, API/docs where role allows
```

## Context bands

- **Global:** NexusAI identity, tenant, authenticated user, role, policy,
  notifications and long-running operations.
- **Case:** case ID/name, classification, status, permissions, evidence
  readiness and processing summary. This band appears only on case routes.
- **Task:** page title, task actions, filters and scoped status.

Case switching is an explicit navigation event. It clears incompatible
case-scoped drafts/results and changes the URL before any new request begins.

## Route states

Every route supplies dedicated `loading`, `ready`, `empty`, `partial`,
`error`, `forbidden` and `unavailable` presentations. Loading uses neutral
language and unknown-value placeholders. Empty is legal only after the
authoritative request resolves. Partial state retains completed artifacts and
identifies the failed scope.

## Responsive behavior

| Width | Navigation | Case context | Content behavior |
| --- | --- | --- | --- |
| 390 | modal drawer; focus trap and return | compact, expandable | one-column; actions stay reachable; bounded table scroll |
| 820 | drawer or compact rail | concise two-line band | single/two-column by task |
| 1024 | persistent collapsible rail | full essential fields | progressive detail; no page overflow |
| 1440 | persistent rail | full context and actions | bounded readable widths plus analytical canvases |

## Accessibility

Landmarks, skip navigation, visible focus, keyboard drawer behavior, named
navigation, status/live regions and touch targets are mandatory. Classification,
warning and capability states use text and semantics in addition to color.
Urdu/bidirectional evidence content preserves source text and direction.

## Source reuse and R3 boundary

Reuse the current provider tree, lazy router, route guards, responsive drawer,
branding endpoint and operations bar where they meet this contract. R3 may
change shell components, tokens, metadata and product-facing copy. It must not
redesign the R4 case desks or implement the R6 Ask lifecycle.

## R3 acceptance handoff

R2 source acceptance freezes the shell contract, not pixels. R3 acceptance
requires login/loading/error/main-shell checks, no broken assets, correct legal
attribution, role/capability navigation, and 390/820/1024/1440 browser coverage.
