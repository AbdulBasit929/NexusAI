# Forensic Authenticated Scope Boundary

Status: source-complete and isolated-test clean on 2026-07-28. The retained
LocalAI and forensic sidecar have not been rebuilt or reconfigured with this
boundary, and migration 008 has not been applied to the retained database.

## Objective

Prevent an authenticated caller, delegated agent, or internal request body from
selecting another tenant, user, collection, or case. The design separates who is
performing an action from whose evidence is being accessed and carries that
scope across LocalAI, the forensic sidecar, and PostgreSQL.

## Trust flow

```text
human or agent
    |
    | LocalAI authentication and effective-user resolution
    v
LocalAI forensic proxy / upload forwarder
    |-- verifies exact collection ownership
    |-- sets actor ID and actor role
    |-- sets subject user ID
    |-- sets configured tenant and requested collection/case
    |-- authenticates the internal call with a shared bearer secret
    v
forensic sidecar middleware
    |-- permits public health only
    |-- validates bearer secret in constant time
    |-- validates bounded trusted headers
    |-- rejects untrusted tenant and body/query scope switching
    |-- restricts repair to an administrator
    v
tenant- and collection-filtered SQL / queued ingest metadata
    |
    v
PostgreSQL tenant RLS as a second boundary after migration/deployment
```

The trusted context is bearer-authenticated; the individual headers are not
separately HMAC-signed. Edge proxies must strip inbound `X-Forensic-*` headers,
and external callers must use LocalAI instead of direct sidecar access.

## Identity model

| Field | Meaning | Example |
| --- | --- | --- |
| actor ID | Identity that initiated the operation | user ID or agent identity |
| actor role | `user`, `admin`, or `agent-worker` | `agent-worker` |
| subject ID | User whose collection/evidence is being accessed | collection owner |
| tenant ID | Deployment-configured tenant boundary | `default` |
| collection ID | Exact LocalAI KB/case collection | `records-demo-verified` |
| case ID | Optional narrower case/workspace hint | case UUID or slug |

Actor and subject are intentionally distinct. A delegated worker can perform an
action for a user without becoming that user, preserving attributable custody and
audit metadata.

## Configuration

LocalAI reads:

- `FORENSIC_RECORDS_API_KEY`
- `FORENSIC_RECORDS_TENANT_ID`

The sidecar reads:

- `FORENSIC_API_KEY` (with `FORENSIC_RECORDS_API_KEY` compatibility fallback)
- `FORENSIC_API_AUTH_REQUIRED`
- `FORENSIC_TRUSTED_TENANT_ID`

Compose maps the shared key and tenant into the sidecar without committing a
secret. In the retained backward-compatible state, an empty key leaves auth
disabled and emits a warning. With `FORENSIC_API_AUTH_REQUIRED=true`, startup
fails if the key is absent. Production promotion must use required mode.

## Route rules

| Route family | Required authenticated scope |
| --- | --- |
| `/healthz` | public; no evidence access |
| upload | tenant, subject, actor, role, collection; optional case |
| collection status | tenant, subject, actor, role, collection |
| evidence list | tenant, subject, actor, role, collection; optional case |
| evidence detail | tenant, subject, actor, role, collection |
| hybrid query/report | tenant, subject, actor, role, collection |
| query templates | tenant, subject, actor, role; no collection data read |
| repair | all collection scope plus `admin` role |

Authenticated body or query values may match the trusted context, but cannot
change it. Evidence detail also filters linked jobs, KB assets, record previews,
and entity rollups by collection, preventing a guessed evidence UUID from
crossing a collection boundary.

## LocalAI ownership enforcement

Before proxying collection-scoped operations, LocalAI resolves the effective
user and compares the requested collection with the exact set returned by
`ListCollectionsForUser`. Missing or foreign collections are denied before the
sidecar call. The sidecar then rebinds the request to its trusted headers rather
than trusting client payload fields.

## PostgreSQL RLS acceptance

The disposable acceptance script creates a temporary role with:

- `NOLOGIN`
- `NOSUPERUSER`
- `NOINHERIT`
- `NOBYPASSRLS`

After granting only the required schema/table permissions and setting
`app.tenant_id=phase3-smoke`, it proves expected same-tenant visibility across
all seven Phase 3 tables, proves a same-tenant insert, and requires a
cross-tenant source-link insert to fail with `insufficient_privilege`. The whole
test rolls back, including the temporary role and insert. This validates policy
behavior under a genuine non-owner role; it does not authorize changing runtime
database ownership or grants.

## Security invariants

1. A request body never establishes authenticated identity or tenant.
2. A user cannot select a collection they do not own.
3. Actor and subject remain separately attributable.
4. Case scope can narrow authorization but cannot widen collection scope.
5. Repair is administrative when authenticated mode is active.
6. SQL retains explicit tenant/collection filters even when RLS is present.
7. RLS is tested with a role that cannot bypass it.
8. Health checks reveal no evidence data.
9. Auth-required configuration fails closed when its secret is missing.
10. Deployment and database migration remain separate approval gates.

## Remaining production work

- Store and rotate the shared secret through the deployment's secret manager.
- Require TLS or a protected service network between LocalAI and the sidecar.
- Strip trusted headers at every external ingress.
- Decide runtime PostgreSQL owner/grant separation before applying migration 008.
- Add token-to-tenant mapping before hosting more than one tenant per sidecar.
- Build/recreate only the affected services under explicit approval, then run
  negative live authorization tests without using sensitive evidence.
- Deploy the completed source-only content-addressed retention/hash-verification
  slice only after its storage gates, and complete JetStream acknowledgement,
  retry, dead-letter, and safe reprocessing.
