# NexusAI white-label inventory

Status: R2 source accepted; R3 migration active  
Updated: 2026-08-06

## Decision classes

- **Replace:** product-facing upstream identity visible to ordinary users.
- **Contextualize:** accurate engine terminology retained inside authorized
  administration with NexusAI framing.
- **Retain:** compatibility, package, protocol, legal or operator identifier
  whose rename would break accuracy or compatibility.
- **Remove/hide:** capability that is irrelevant, unsafe or not accepted for the
  audience.

## Source inventory

| Surface | Current evidence | Class | R-phase action |
| --- | --- | --- | --- |
| HTML title/favicon | `react-ui/index.html` already says NexusAI and references `/favicon.svg` | replace/verify | R3 verifies built and deployed metadata/assets. |
| Runtime title/favicon/login identity | `BrandingContext` reads public branding, defaults to NexusAI and updates document metadata | reuse | R2 freezes config contract; R3 validates pre-auth/failure behavior. |
| Sidebar/mobile/footer wordmark | `Sidebar.jsx` and `App.jsx` consume branding; NexusAI fallback wordmark exists | reuse/refine | R3 applies approved assets and legal/version presentation. |
| Analyst navigation/copy | Current mixed general AI, Build/Operate and forensic routes | replace/restructure | R3 shell; R4-R6 case/evidence/Ask terminology. |
| Engine administration | Backends, models, Agent Hub, nodes, P2P, traces and CLI/container commands contain LocalAI terms | contextualize/retain | Keep exact technical names under NexusAI System Administration. |
| Compatibility APIs and docs | OpenAI/Ollama/vendor routes, LocalAI API paths and explorer | retain | Developer/admin only; document product-vs-engine boundary. |
| Code/package/config identifiers | Go imports, images, env keys, feature flags, local-storage keys and route internals | retain | No blind rename; migrate only with compatibility plan. |
| Browser-storage keys | many `localai_*` keys | retain initially | Non-visible compatibility; version/migrate only when state contract requires. |
| LocalAI Assistant/manage mode | product-facing term and `localai_assistant` protocol flag coexist | replace visible, retain protocol | Ordinary forensic experience becomes Ask NexusAI in R6; admin protocol remains internal. |
| Legal/upstream attribution | source licenses, notices, dependency links and upstream references | retain | Present accurately without turning them into product branding. |
| Reports/exports/social/install icons | existing branding fields cover horizontal/report fallback; social/install variants are not yet implemented | reuse/extend only if needed | R3 metadata/asset work; report integration in its owning phase. |

## Product-facing search disposition

The bounded React search finds mostly non-visible storage keys, comments,
protocol flags, image names and administrator commands. Those do not justify
mass replacement. Visible analyst copy must be reviewed by route and role.
Administrator instructions may say LocalAI when that is the exact binary,
container, API or upstream service being operated.

## R2 exit gate

Passed on 2026-08-06: the asset/attribution list, existing branding/fallback
contract, surface inventory, replace/contextualize/retain/hide rules and
390/820/1024/1440 R3 browser scenarios are recorded. R3 owns implementation and
verification; this acceptance does not imply build, deployment or runtime
acceptance.
