# NexusAI R3 source and production-browser acceptance

Date: 2026-08-06  
Verdict: PASS  
Deployment: not started; separately approval-gated

## Accepted evidence

- Production build: Vite 8.0.16, 662 transformed modules, hashed JavaScript and
  CSS assets, successful completion.
- Automated browser: 27/27 focused login, loading, error, application-shell,
  Knowledge and navigation contracts passed in installed Chrome against the
  production preview.
- Responsive: 390, 820, 1024 and 1440 pixel widths passed without settled
  page-level horizontal overflow; mobile and desktop context presentation
  switched at the intended boundary.
- Accessibility: keyboard skip-link/route focus, drawer focus return, semantic
  error/unavailable states and authorization loading contracts passed.
- Presentation: NexusAI title/favicon, role/navigation language, light and dark
  themes, Knowledge state language and Talk unavailable state were verified.
- Assets/console: preview served hashed `dist` assets with no `/@vite/client`;
  the controlled production tab captured no browser warnings or errors.

## Corrections during acceptance

The focused suite initially identified two test-harness issues rather than
product failures: an installed system browser may need one focus-entry Tab
before document traversal, and the branded loading state intentionally contains
a nested spinner with its own `status` role. The contract now tolerates the
focus-entry step and scopes the loading assertion to `.nexus-loading-state`.
Both targeted tests and the complete 27-test set then passed.

## Preservation

The preview was stopped after verification. No Docker image or container was
rebuilt or deployed, no volume/database/case/evidence/model state changed, and
nothing was staged, committed, pushed or published.
