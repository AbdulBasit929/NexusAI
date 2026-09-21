# Unified UI Deployment Ownership and Isolation Map

Date: 2026-09-07

## Runtime composition

| Layer | Exact owner | Deployment treatment |
|---|---|---|
| Backend binary and its embedded prior UI | Accepted live image `sha256:feb935b42a93c1810394cb270a809189ca240a225253159a47b8daa72a799171` | Used as an opaque base; not rebuilt |
| Unified React application | Sealed production `dist/` from the validated current source | Copied to `/opt/nexusai/unified-ui` |
| Same-origin UI gateway | `scripts/nexusai-unified-ui-wrapper` | Serves the sealed UI on container port 8088 and proxies current backend routes to `127.0.0.1:8080` |
| Host route | Compose UI-only override | Host `8080` → UI gateway `8088`; the accepted backend continues internally on `8080` |
| Protected services | forensic API, worker, NATS, PostgreSQL | Container and image identities must remain unchanged |
| Retained data | Existing volumes/database | Before/after seven-field tuple must match; no migration or evidence write |

The gateway launches the image's existing `/entrypoint.sh phi-2` contract,
forwards signals, proxies streaming/WebSocket-capable HTTP through Go's reverse
proxy, and serves SPA routes only for browser HTML navigation. The image's
existing health check continues to test the actual backend on internal port
8080.

## UI task source ownership

The deployment manifest records exact SHA-256 values for the bounded unified
workspace files: routing, portal layout, unified workspace, evidence and
history integration, Ask presentation, workspace identity, responsive styles,
the analyst Playwright suite, preview configuration, UI gateway and activation
script. The deployed artifact is the compiled UI, not a dirty repository build.

## D exclusion proof

- Build-context production source files: **0**.
- Files from the frozen 460-file D activation source manifest present as source
  in the context: **0**.
- Forbidden D marker matches (`nxb21d`, Decision8, Q4 planner and Qwen
  acquisition identifiers): **0**.
- Backend compilation during this activation: **none**.
- D-unqualified directories and patterns explicitly excluded:
  `api/forensic_records/d_*`, `api/forensic_records/nxb21d_*`,
  `scripts/nxb21d*`, `reports/nxb21/d-*`, Q4/Qwen configuration, and every
  other repository production-source directory.
- The Docker image carries
  `nexusai.d-unqualified-source-included=false`; the activation script verifies
  that label before recreation.

The machine-readable authority is
`reports/nxb21/investigation-workspace-unified-ui-deployment-manifest-20260907.json`
with its adjacent SHA-256 sidecar. The no-mutation script preflight validates
all 216 context files and rejects missing, changed or extra files.

## Route reconciliation

| Check | Live result |
|---|---|
| `SOURCE_EXISTS` | YES — unified source and tests are present |
| `ROUTES_REGISTERED` | YES — `/analyst` canonical plus compatibility redirects |
| `BUILD_CONTAINS_UI` | YES — production build completed and is sealed |
| `EMBED_CONTAINS_UI` | YES — the UI-only overlay image contains the sealed production build |
| `RUNNING_IMAGE_CONTAINS_UI` | YES — API image `sha256:5e056478ae0f1823cfe0fc453f1c26a60c2eb86cc173958c6537fc064b0420d9` |
| `PROXY_ROUTE_STATUS` | PASS — live same-origin gateway serves UI and current backend routes |
| `CURRENT_RUNTIME_ROUTE_STATUS` | `LIVE_VERIFIED` — canonical `/analyst`, backend and browser acceptance pass |
