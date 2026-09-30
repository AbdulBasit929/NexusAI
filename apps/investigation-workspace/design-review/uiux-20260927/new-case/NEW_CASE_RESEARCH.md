# New case — research and implementation ruling

Date: 2026-09-27  
Surface: global New case / first evidence intake  
Authority: `docs/ux/NEXUSAI_PRODUCT_UX.md`, live upload behavior and the no-case-entity architecture

## Analyst job and product truth

The analyst is not creating an empty case record. They choose a stable collection identifier and submit the first evidence. **The case begins only when the first file is accepted by `POST /webhooks/records/upload`.** Naming alone changes no server state.

The page must therefore make three phases unmistakable:

1. choose a valid immutable case identifier;
2. select and transfer the first evidence;
3. after acceptance, monitor processing and open the case.

Transfer and processing are separate truths. Transfer has measurable byte progress. Processing exposes states and counts, but no percentage.

## Endpoint and state audit

- There is no case entity and no create-case endpoint.
- `POST /webhooks/records/upload` accepts multipart evidence scoped to the chosen collection identifier. A successful response is the creation boundary.
- `GET /collections/status` supplies subsequent evidence processing state.
- The upload client can report real XHR byte progress. It can also safely support browser cancellation by aborting the in-flight XHR; cancellation does not claim that a server-accepted file was removed.
- The service owns file-type/size/content acceptance. The browser must not claim support from an `accept` list that could drift from the backend.

## External research applied

1. [GOV.UK file upload](https://design-system.service.gov.uk/components/file-upload/) supports both a visible choose-files control and a persistently visible drop target, uses short translatable instructions, announces entry/exit from the drop zone, and recommends precise refusal messages. Ruling: keep the native file input as the foundation, provide a real button, visible drop affordance, drag-state announcement and exact server refusal.
2. [U.S. Web Design System file input](https://designsystem.digital.gov/components/file-input/) treats enhancement as progressive, requires an associated label/hint, reports selected file names/counts to screen readers, and removes drag language on coarse-pointer mobile. Ruling: the NexusAI chooser remains backed by `input[type=file]`, exposes selected files as a named list, and does not make dragging the only path.
3. [W3C ARIA25](https://www.w3.org/WAI/WCAG22/Techniques/aria/ARIA25) shows file-transfer progress paired with a polite live status. Ruling: only measured transfer gets a progressbar and visible/live percentage; processing never receives a synthetic bar.
4. [GOV.UK text input](https://design-system.service.gov.uk/components/text-input/) requires a visible short label, persistent hint, `aria-describedby`, and error styling only with a specific message. Ruling: call this field **Case identifier**, not a friendly “name,” and keep its immutable format rule visible.
5. [GOV.UK error message](https://design-system.service.gov.uk/components/error-message/) recommends distinct messages for empty, length, forbidden characters and format errors. [USWDS text input](https://designsystem.digital.gov/components/text-input/) recommends showing validation after interaction. Ruling: retain touched validation but split the current catch-all into actionable reasons.

## Information architecture and interaction

- Page title: **Start a case with evidence**.
- A compact three-step lifecycle explains the creation boundary before the form.
- Step 1 is a single visible `Case identifier` field. A read-only confirmation line repeats the exact identifier; nothing is silently normalized.
- Step 2 appears only when the identifier is valid. The target says **Choose evidence files** first; drag-and-drop is secondary.
- Each selected file row shows filename, byte size, state, real transfer percentage when available, and Cancel only while queued/sending. Refused or cancelled local rows can be removed. Accepted evidence cannot be “removed” from this UI because there is no delete contract.
- Duplicate file selections receive distinct client row IDs; one update can never mutate another row accidentally.
- After the first acceptance, a clear ready-state confirms that the case now exists, starts processing-status polling, and offers **Open case**. Questions are explicitly limited to ready evidence.

## Accessibility and responsive acceptance

- File input remains programmatically labelled; drop status and transfer text are polite live regions.
- Every button is at least 44 CSS pixels and cancellation is available from the keyboard.
- State is conveyed by icon/text/description, never colour alone.
- On coarse-pointer/mobile layouts, the primary instruction remains choose-files; no workflow depends on drag-and-drop.
- 375px has no page-level horizontal overflow; file names and identifiers wrap safely.

## Deliberately not doing

- No empty case is claimed before first-file acceptance.
- No separate display title, classification, owner, role or tenant field is invented because there is nowhere to persist it.
- No client-side file-type promise is added; the service remains the acceptance authority.
- No processing percentage, estimated duration, or fake completion time.
- No accepted evidence delete/remove control without an authorized deletion endpoint.
