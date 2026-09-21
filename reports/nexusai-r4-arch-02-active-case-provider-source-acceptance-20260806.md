# NexusAI R4-ARCH-02 active-case provider source acceptance

Date: 2026-08-06  
Verdict: PASS at source and local production-browser boundary  
Deployment: not performed

## Outcome

NexusAI now has one shared, read-only active-case provider at the authenticated
application-route boundary. App Shell context, `/app/records` default routing
and Case Workspace identity consume that provider instead of independently
parsing or fetching case scope.

The provider implements the accepted R4 precedence and safety contract for this
bounded slice:

1. an explicit case path;
2. an explicit `?case=` compatibility parameter;
3. the provider's current authorized case;
4. the governed API default or first selectable case.

Explicit cases are validated against the governed accessible-case registry.
An inaccessible explicit case fails closed before case-detail, status,
capability, manifest or evidence requests are issued. No default/demo fallback
occurs for an invalid explicit case.

## Bounded implementation

- Added `ActiveCaseProvider` and `useActiveCase` in
  `core/http/react-ui/src/contexts/ActiveCaseContext.jsx`.
- Mounted the provider once beneath authenticated routing.
- App Shell desktop and mobile context now display only provider-validated case
  identity; pathname parsing no longer invents an active case.
- Records Redirect consumes the provider's current/default case and offers a
  governed registry retry state.
- Case Workspace consumes provider identity and selector options, removes its
  duplicate case-list request and uses provider navigation for case switches.
- Case Workspace clears old case-derived state immediately on a switch and
  ignores late evidence responses for the previous case.
- Existing detail, status, capability, manifest, evidence and upload API
  behavior remains unchanged.

## Verification

- Focused ESLint: PASS with zero errors. Existing JSX/fast-refresh warning
  behavior remains warnings-only; no rule or baseline was weakened.
- Vite 8.0.16 production build: PASS, 664 transformed modules.
- Production assets included `index-Bw0r5KkB.js` and the accepted
  `index-pJX-gsGx.css` visual-system stylesheet.
- Focused shell and Case Workspace browser matrix: PASS, 16/16.
- Protected R3/R3.1 plus R4 case-context regression matrix: PASS, 35/35 in
  installed system Chrome, with no browser download.
- Changed-file whitespace validation: PASS.

The browser contracts cover explicit URL validation, fail-closed inaccessible
cases with zero case-specific requests, one governed default resolution,
desktop/tablet/mobile overflow, keyboard navigation, evidence URL binding and
clearing prior-case results while a new case is loading.

## Preservation

No API/backend schema, database, case/evidence data, collection, agent, model,
image, container, volume or retained configuration was changed. No Docker
rebuild/redeployment, download, staging, commit, push or publication occurred.
The accepted live R3.1 runtime remains unchanged.

## Next bounded slice

`R4-ARCH-03 — Bind embedded Records Intelligence to the active case` removes
alternate collection authority from embedded mode, locks case-aware query,
upload/list/report actions to provider identity and adds focused no-cross-case
tests. Unbound administrative compatibility remains explicitly separate.
