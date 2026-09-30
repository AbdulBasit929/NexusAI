# Settings — research and implementation ruling

Date: 2026-09-28  
Scope: `/settings`, shared appearance control, browser-local working data only

## Evidence reviewed

- Microsoft, [Guidelines for app settings](https://learn.microsoft.com/en-us/windows/apps/design/app-settings/guidelines-for-app-settings): keep settings few, group related choices, use radio controls for up to five mutually exclusive options, apply changes immediately, keep routine workflow commands out of Settings, and constrain the readable content width.
- W3C, [Media Queries Level 5](https://www.w3.org/TR/mediaqueries-5/): `prefers-color-scheme` represents a user preference and sites may respect it while allowing an explicit override. A system-following option must continue reacting when that preference changes.
- W3C, [CSS Color Adjustment Module Level 1](https://www.w3.org/TR/css-color-adjust-1/): declare supported colour schemes so browser-owned controls render coherently with the chosen theme.
- W3C, [WCAG 2.2](https://www.w3.org/TR/WCAG22/), especially 3.3.4 and 4.1.3: deletion of user-controlled stored data needs review/confirmation or reversibility, and completion feedback must be exposed as a status message.
- GOV.UK Design System, [Button](https://design-system.service.gov.uk/components/button/): reserve the destructive warning treatment for the final irreversible confirmation, not the initial request.
- WHATWG, [HTML Standard — Web storage](https://html.spec.whatwg.org/multipage/webstorage.html): local storage belongs to the browser origin. It is not an account, case record, or server preference store.

## Measured problems in the current surface

1. `tenantId || 'default'` and `actorRole || 'user'` manufacture identity facts when no login/session/role endpoint exists. The UX contract explicitly forbids these placeholders.
2. The first `ThemeControl` render resolves the system theme and immediately persists that concrete value. The UI therefore stops following later device changes without telling the analyst.
3. Header and page theme controls own separate React state, so changing one can leave the other visually stale.
4. “Stored keys” and character count describe an implementation, not what the analyst can recover or remove.
5. “Clear stored questions” also clears drafts, so its label understates the destructive scope.
6. Connection path, proxy shape, and missing authentication are deployment details rather than ordinary analyst preferences.

## Design ruling

- Settings is a narrow preference centre, not an administrator page and not a connection diagnostic.
- Expose only working controls backed by this browser: appearance, question history, and drafts.
- Appearance has three immediate, mutually exclusive options: **Use device setting**, **Light**, and **Dark**. Device mode stores no override and follows `prefers-color-scheme` changes. Explicit modes persist when storage is available.
- All mounted appearance controls share one external-store state so their selected option and resolved theme agree.
- Summarise local working data in analyst language: saved questions, non-empty drafts, and affected cases. Do not present storage keys, token-like identifiers, or a misleading byte estimate.
- Name the destructive scope exactly: **Clear question history and drafts**. The first control opens an inline review; only the final confirmation uses the destructive treatment. Announce completion without moving focus.
- Preserve appearance and navigation preferences when clearing working data. State this boundary before confirmation.
- Remove tenant, role, and connection fields entirely until authoritative session/configuration endpoints exist. Do not replace them with unavailable-looking fake settings.

## Acceptance

- System mode is the default when no valid override exists and responds to media-query changes.
- Light/dark choices persist across remount; every storage read/write/remove remains guarded.
- Two mounted controls stay synchronized.
- Browser-local counts are exact; clear removes only question histories and drafts and reports completion.
- Storage-denied mode remains usable and truthful.
- Both themes pass at 1440 and 375 px with no horizontal page scroll; controls remain at least 44 px.
- No tenant, role, API path, credential, model, agent, operation, SQL, or other internal implementation term appears.
