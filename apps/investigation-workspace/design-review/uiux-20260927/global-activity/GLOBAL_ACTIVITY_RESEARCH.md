# Global Activity — research and implementation ruling

Date: 2026-09-28  
Scope: global `/activity`. Case-scoped `/cases/:id/activity` is already a deliberately bounded working record and is not changed by this slice.

## Sources reviewed

1. [NIST SP 800-171r3 — Audit and Accountability](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/800-171r3/NIST.SP.800-171r3.html)
   - An audit record establishes the event type, time, location/source, outcome and identity associated with the event.
   - Audit information and audit tooling must be protected from unauthorized access, modification and deletion.
2. [OWASP Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html)
   - Audit/transaction trails and security event logs serve different purposes and should not be conflated.
   - Access controls on event-log data must be verified; log readers should have restricted, reviewed privileges.
3. [GOV.UK Design System — There is a problem with the service pages](https://design-system.service.gov.uk/patterns/problem-with-the-service-pages/)
   - Say clearly what is unavailable and provide another route that lets the user complete their task when one exists.
   - Avoid internal error jargon and unsupported promises about availability.
4. [IBM Carbon — Empty states](https://carbondesignsystem.com/patterns/empty-states-pattern/)
   - Explain what the space would contain and give a concrete next action; an empty state should teach the recovery path.

## Endpoint and data audit

- The global navigation contract requires Activity.
- `forensic.records_audit_log` exists and the case manifest can return only its **count** as one resource line.
- No route returns audit event rows. A count cannot establish who acted, what happened, when it happened, its source or its outcome.
- No authorized cross-case activity endpoint, viewer/session identity endpoint, workspace membership source, custody-event endpoint or export-history endpoint exists.
- `/collections/status` supplies only a recent evidence/job sample within one collection.
- Browser-local saved questions are not an audit log and may be incomplete, cleared or unavailable.

## Measured defect in the existing surface

The page was a single dead empty-state sentence. It was truthful, but it offered no productive recovery even though every configured collection has a bounded case working-record route. Its page description also promised “events across cases and viewers,” which the product cannot supply.

## Ruling

- Keep global Activity unavailable as an authoritative workspace record. Do not aggregate browser-local question history or recent collection samples into a feed that looks authoritative.
- Replace the dead end with an explicit unavailable-state gateway to each configured case working record.
- Explain the distinction in plain language:
  - case working record: browser-saved questions plus the recent evidence sample returned for that collection;
  - missing workspace audit history: viewer identity, custody events, exports and team-wide chronology.
- List configured collection IDs only. Do not fetch status, invent event counts, display last-activity timestamps, or imply that a zero local history count means no activity occurred.
- State the minimum contract the future endpoint must provide: authorized, paginated event rows with event type, trustworthy timestamp, source, outcome, actor identity and exact case scope.

## Missing backend contract

Required before this surface can become a real feed: an authorization-enforced, paginated workspace activity endpoint plus server-issued viewer identity and membership. The endpoint must return event type, authoritative timestamp, source, outcome, actor identity, case/collection scope and an openable locator where the event refers to evidence. Until then, a gateway is more useful and more honest than a fabricated roll-up.
