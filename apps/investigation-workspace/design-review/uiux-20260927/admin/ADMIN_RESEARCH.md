# Admin — authorization ruling

Date: 2026-09-28  
Scope: `/admin`, analyst navigation, client capability flags

## Evidence reviewed

- NexusAI product UX §6.9: Admin is authorization-gated and absent from analyst navigation entirely, not shown disabled.
- OWASP, [Authorization Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html): deny by default, validate permission on every request, and never rely on client-side access-control checks.
- NIST SP 800-207, [Zero Trust Architecture](https://www.nist.gov/publications/zero-trust-architecture): network location does not establish trust; authenticate and authorize the subject before establishing access to an enterprise resource.

## Measured current state

- No login/session endpoint, user entity, role membership, or server-issued permission document exists.
- `/admin` is always registered and directly reachable.
- A browser-served `capabilities.admin === true` value can reveal an Admin navigation link. This is presentation configuration, not authorization.
- The route contains no admin capability. It renders only an unavailable placeholder, so exposing it is both misleading and contrary to the contract.

## Ruling

- Remove the Admin route, lazy chunk, placeholder page, icon and runtime-config-driven navigation branch.
- Treat `/admin` as an unknown analyst route and return to the dashboard. Do not render a role badge, a fake access-denied decision, or an unavailable admin console when no authority made that decision.
- Do not add users, roles, health, queue, model, backend, or capability controls. Their endpoints and policy enforcement do not exist in this application contract.
- Restore Admin only when the backend provides an authenticated session/identity contract plus server-authoritative authorization for the page and every administration request. Client-side hiding remains defence-in-depth, never the gate.

## Acceptance

- Admin is absent even when browser runtime configuration claims `admin: true`.
- Direct `/admin` navigation exposes no Admin heading or placeholder and resolves to the analyst dashboard.
- No `AdminPage` production chunk is emitted.
- The remaining route audit stays clean.

## Missing backend contract

At minimum: authenticated session identity; tenant membership; authoritative role/permission claims; an authorization decision for the Admin surface; server-enforced authorization on every admin endpoint; and typed, analyst-safe admin summary endpoints. None exists today.
