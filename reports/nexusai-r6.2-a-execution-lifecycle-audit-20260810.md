# NexusAI R6.2-A execution lifecycle and server capability audit

Date: 2026-08-10  
Status: audit accepted; R6.2-B selected; no runtime change

## Decision

The first implementation slice is `R6.2-B — Correlated agent lifecycle event
contract`. Every processing, streaming, terminal and error event must carry the
originating request `message_id`, and the browser must update lifecycle state
only for that request and governed case scope.

This precedes cancellation, timeout recovery and retry. A cancel or retry action
cannot be made trustworthy while status, reasoning and tool events are not
consistently attributable to one request.

## Current request path

1. `POST /api/agents/:name/chat` validates the message and authorized forensic
   case/collection scope, returns HTTP 202 and a generated `message_id`.
2. The browser stores that ID in an in-memory pending-request map associated
   with the initiating conversation and case scope.
3. `GET /api/agents/:name/sse` delivers asynchronous status, stream, message and
   error events for the agent/user channel.
4. Final agent messages normally carry `<message_id>-agent`; the browser removes
   the suffix and routes the result to the initiating conversation.
5. Case-scoped browser state rejects events after a governed case switch, and
   conversation history remains isolated in case-keyed local storage.

## Execution-mode capability matrix

| Mode | Correlation | Deadline | Cancellation | Disconnect recovery |
|---|---|---|---|---|
| Local legacy LocalAGI | final message ID only; status/error uncorrelated | no bounded request deadline found | no chat endpoint or reachable registry | EventSource reconnect only; no replay |
| Local native forensic/model | native events normally add message ID | 210-second model path; deterministic route runs outside that outer deadline | no registered per-chat cancel | EventSource reconnect only; no replay |
| Distributed worker/model | final message correlated; status and stream events omit message ID | worker-lifetime context, no per-chat deadline | internal EventBridge cancel registry and NATS cancel subject exist, but no public chat route | reconnect resubscribes only; no replay |
| Distributed deterministic direct route | final message correlated; status and stream events omit message ID | bounded downstream calls, no whole-execution deadline | execution is not registered for cancellation | reconnect resubscribes only; no replay |

## Verified strengths

- The submission response supplies a stable request ID.
- Final agent messages are correlated through the executor's
  `responseMessageID` convention.
- The browser pins final responses to the initiating conversation.
- Forensic submissions carry explicit provider-validated case and collection
  identity and reject late prior-case events.
- Distributed workers already own a cancellation registry and cross-instance
  NATS cancellation subject.
- The native local model path has a 210-second outer timeout; records and
  knowledge HTTP helpers use bounded downstream timeouts.
- Generic Agent Chat compatibility remains distinct from forensic case scope.

## Material gaps and risk ranking

### P0 — distributed live events are not request-correlated

`EventBridge.PublishStatus` and `PublishStreamEvent` do not accept a message ID.
The NATS dispatcher and distributed deterministic route therefore publish
processing, reasoning, content and tool events without request identity. The
browser keeps only one visible stream buffer per Agent Chat page and currently
accepts stream events by case scope, not request. Two conversations in the same
case can consequently mix live reasoning/tool state even though final messages
are routed correctly.

### P1 — cancellation capability is internal but unreachable

`EventBridge.CancelExecution`, the NATS cancel listener and the worker cancel
registry exist. There is no `/api/agents/:name/chat/:message_id/cancel` route,
no AgentPool service abstraction covering every execution mode, and no UI stop
action. Local native and distributed deterministic executions are not
registered in the same cancellation lifecycle.

### P1 — terminal and timeout behavior is inconsistent

Some failure events omit request identity. The local legacy path has no bounded
deadline, the distributed worker inherits only its service context, and the
distributed deterministic path has downstream call timeouts rather than one
whole-execution deadline. A lost terminal event can leave browser processing
state indefinitely active.

### P1 — reconnect has no recovery contract

Native `EventSource` reconnects automatically, but neither SSE implementation
emits event IDs, accepts `Last-Event-ID`, replays missed events or exposes
request status. Events produced during a disconnect are lost and completion
cannot be reconciled after reconnect.

### P2 — retry semantics are undefined

There is no idempotency key or durable request record for chat submission.
Automatically repeating POST could duplicate tool work. Retry must remain an
explicit new request until a durable/idempotent contract exists.

### P2 — configured cancel-previous behavior is not enforced here

`cancel_previous_on_new_message` is present in agent configuration metadata but
no execution-path use was found. It must not be presented as an active product
guarantee.

## R6.2-B bounded implementation contract

1. Extend distributed status and stream publication with `message_id` while
   preserving existing JSON event names and generic clients.
2. Ensure local legacy errors/statuses and deterministic paths use the same
   request identity wherever a request ID exists.
3. Make browser status, stream and error handlers reject mismatched request IDs
   as well as mismatched forensic scope.
4. Preserve the pending map until a correlated terminal message/error, and do
   not let an older request clear a newer request's spinner.
5. Add focused Go event/dispatcher tests and Playwright coverage with two
   conversations in the same case emitting interleaved events.
6. Do not add cancellation, automatic retry, durable history, schema changes or
   deployment in R6.2-B.

## Following slices

- `R6.2-C`: authorized cancellation endpoint plus UI Stop action across local,
  distributed and deterministic execution modes.
- `R6.2-D`: consistent whole-request deadlines and truthful timed-out terminal
  state.
- `R6.2-E`: reconnect reconciliation/status lookup; replay only if a durable
  event contract is deliberately approved.
- `R6.2-F`: explicit safe retry contract, with idempotency before any automatic
  resubmission.

## Governance

This was a read-only source audit plus documentation update. No application
source, API, route, database, model, agent configuration, evidence, collection,
container, image, volume or runtime state changed. No build was required. No
staging, commit, push or publication occurred.
