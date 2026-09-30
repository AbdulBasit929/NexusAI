# NexusAI Investigation Workspace

Standalone React 18 case workspace for governed collection-backed routes:

```text
/cases
/cases/:id/overview
/cases/:id/evidence
/cases/:id/evidence/:evidenceId
/cases/:id/investigate
/cases/:id/timeline
/cases/:id/activity
```

## Install, verify, and build

```powershell
npm ci
npm run test:ported
npm run test:unit
npm run test:e2e
npm run build
```

The deployable static site is written to `dist/`. To inspect that production build
locally (including SPA route fallback and the same-origin API proxy), run:

```powershell
npm run preview
```

For development only:

```powershell
npm run dev
```

## Runtime API configuration

`public/runtime-config.js` is copied to `dist/runtime-config.js` and loads before the
application bundle. Its checked-in default points to the same-origin `/api` proxy and
contains no credential. The Vite development and preview servers attach the backend key
server-side and forward to `http://localhost:8091` by default. Configure them before
starting the server:

```powershell
$env:FORENSIC_RECORDS_API_KEY = '<local key>'
$env:NEXUSAI_API_URL = 'http://localhost:8091'
npm run dev
```

Open `http://127.0.0.1:4181/cases`. A serving shell or deployment may replace the runtime
file without rebuilding; its safe default remains:

```js
  window.__INVESTIGATION_WORKSPACE_CONFIG__ = {
    apiBaseUrl: "/api",
    capabilities: { hybrid_query: true },
  };
```

The workspace calls these collection-scoped API surfaces:

```text
GET  /collections/status
GET  /evidence
GET  /evidence/:evidenceId
GET  /evidence/:evidenceId/content
POST /webhooks/records/upload
POST /query/hybrid
```

Requests send `X-Forensic-Tenant-ID`,
`X-Forensic-Collection-ID`, `X-Forensic-Actor-ID`, `X-Forensic-Subject-ID`, and
`X-Forensic-Actor-Role`. In proxy mode, `Authorization: Bearer …` is added by the server,
not the browser. `FORENSIC_RECORDS_API_KEY` and `FORENSIC_RECORDS_TENANT_ID` are
server-side configuration only: do not copy the API key into the repository, static
bundle, or browser storage. The client rejects absolute and protocol-relative
`apiBaseUrl` overrides; browser-direct credential mode is not supported.

The bundled proxy is a local development/review convenience, not authentication. A real
deployment still needs an owned identity/session layer in front of the same-origin proxy;
until that exists, the static build must not be exposed as an authenticated service.

## Fixtures and visual review

The current live capture is labeled under `fixtures/live-20260924/`; transient failures
and every retry remain in separately named directories. Older fixtures under `reports/`
remain immutable provenance and are explicitly legacy—not the visual-review source.
The Playwright suite serves the app with captured live API responses and writes the
current WI-UI-6 review set to `design-review/wi-ui-6/`.
