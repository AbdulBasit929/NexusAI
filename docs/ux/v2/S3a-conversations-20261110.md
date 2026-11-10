# Stage S3a: conversations on the Ask page (the case layer's first screens)

**Why.** The case layer (migration 034 and `internal/caselayer`) went live on 2026-11-10. Until now the Ask page kept its history in the browser: no search, no re-run, no fork, nothing that survives a browser change or can be audited. The owner asked for the case backend's screens to be built and deployed before the media work. This stage is the first of them; notes, findings, hypotheses, tasks, members and audit follow, one at a time.

## Research behind it (done before building; sources are mostly design guides and vendor documentation, so they are patterns, not proof)

| Question | What the sources say | Source |
|---|---|---|
| A history rail | Grouped by date, search at the top, favourites first, an automatic name from the first message that the person can rename. A scroll-for-ever list with no way to organise it is the common complaint. | [Cloudscape: conversational history](https://cloudscape.design/gen-ai/patterns/conversational-history/) (its text was not readable in the search result), [Setproduct: AI chat interfaces](https://www.setproduct.com/blog/ai-chat-interface-ui-design) |
| "What changed since the last run" | A badge on the item that opens a diff; separate cosmetic changes from substantive ones; one run differing can be sampling noise, so say when the data did not move. I found no published pattern that combines a data-version badge with a per-cell table diff; that combination is designed here from the parts. | [Plannotator: plan diff](https://plannotator.ai/blog/plan-diff-see-what-changed/), [Modern Analyst: provenance UX](https://www.modernanalyst.com/Resources/Articles/tabid/115/ID/7146/Provenance-UX-How-to-Specify-Why-Should-I-Believe-This-in-Product-Requirements.aspx) |
| Waiting minutes for an answer | Name the step instead of a spinner, show elapsed time after about ten seconds, keep the work alive if the person leaves, end every run in a result, a retry or an explanation. The service sends only real phases and a heartbeat, so the page shows a clock and never a percentage. | [Long-running AI task UI patterns](https://particula.tech/blog/long-running-ai-tasks-user-interface-patterns), [The waiting problem in AI products](https://uxdesign.cc/the-waiting-problem-in-ai-products-e7c11fd5a825) |

## What was built

- `lib/conversationsApi.js`: every call of the contract, and the question as a stream (lines that arrive cut in two are joined; an error before the first event is a sentence; a stream that closes without an answer says so).
- `useConversationThread`: the same shape as the old thread hook, plus run again, fork, feedback and pin. A conversation is a tree; the page draws the path to the newest turn, so a run again takes the place of the run it repeats and carries its comparison. "Stop" means stop waiting: the question keeps running on the server.
- `useConversationRail` and `ConversationRail`: pinned first, then by day; search on the server (it folds Urdu script, Roman Urdu and Urdu digits) with the question that matched shown; a "Changed" mark; pin, rename in place, archive, delete (it hides; the line under the list offers Undo).
- Turn: the wait names the phase and shows the service's own clock; "Changed since the earlier run · Compare" opens `RunDiff`, which leads with whether nothing changed, the figures changed, or the data did NOT change but the answer did; Run again, Pin to findings, Fork, and a right/wrong verdict.
- The open conversation is kept in the address (`?thread=`), read once when the page opens.
- A reusable `Dialog` (focus in, Tab kept inside, Escape and outside click, focus returned) now also carries the chart tables.
- When the service is not installed (migration 034 absent) the page falls back to the browser history exactly as before; questions limited to one kind of record also stay in the browser, because the service takes the question only (a request for the case-layer owner: scope on a turn).
- `lib/caseObjectsApi.js`: the data layer for the screens still to come (notes, findings, hypotheses, tasks, annotations, views, tags, members, authority, audit). Not used by a screen yet.

## Checked

- **Live, in a browser, on the deployed service and a synthetic case: 13 of 13.** The rail comes from the service and has a search box; the wait is narrated with a clock; the address keeps the conversation; run again, pin, fork and verdict buttons are there; a reload returns to the same conversation; pin and verdict are kept; a run again carries its comparison mark; rename is kept; search finds the conversation; delete offers Undo. The first live run found a bug the unit tests could not: the page re-opened the conversation it had just created and cancelled the question being sent (fixed: the address is read once).
- 22 new tests (stream parsing, the path of a tree, the hook's flows with the service mocked, the rail, the comparison dialog, the turn) and 115 in the interface suite pass.
- The comparison dialog was **not** opened live: the run again of that question gave the same answer ("Same as the earlier run"). Its content is covered by unit tests only.
- Not run: axe, the Playwright suite, a screen-reader pass of the rail menu and dialogs.

## Open

- Edit and resubmit is offered only for the last question of a conversation (the page draws one path; an edit of an earlier question would hide the turns after it).
- Cancel: the contract has none (a question runs to the end), so the page offers "stop waiting" only.
- The history rail's menu sits in a scrolling list and can be clipped near the bottom edge.
- Next screens, with their research notes: findings and hypotheses as a hypothesis-by-evidence matrix (the established form for competing hypotheses; the API's `supports` and `contradicts` fit it), notes and tasks, then members, authority and the audit trail.
