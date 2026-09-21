# LocalAI/NexusAI React route inventory

Verified: 2026-08-06

| Surface | Routes | Audience | NexusAI disposition |
| --- | --- | --- | --- |
| Home/chat | `/app`, `/app/chat[/:model]` | general | retain; evolve Ask NexusAI entry |
| Case workspace | `/app/cases/:caseId/:section` | analyst | primary governed product route |
| Records compatibility | `/app/records` | analyst | redirect into governed case workspace |
| Specialist agents | `/app/agents/:name/chat` and status/edit | analyst/admin split | retain governed chat; admin-gate edit/status |
| Collections | `/app/collections[/:name]` | advanced/admin | hide raw KB mechanics from normal case flow |
| Models | `/app/models`, editor/import | administrator | admin-only role-aware registry |
| Backend operations | `/app/backends`, settings, traces, nodes, scheduling | administrator | keep as NexusAI system administration |
| Skills/jobs | `/app/skills`, `/app/agent-jobs/*` | administrator/advanced | capability-aware exposure |
| Media generation | image/video/TTS/sound/talk/transform/studio | mixed | forensic media evidence must use separate governed workspaces |
| Recognition | face/voice | administrator/policy gated | hide unless approved |
| Auth/account/users/usage | login, invite, account, users, usage | user/admin | extend for enterprise RBAC |
| Explorer/API compatibility | `/explorer`, compatibility redirects | developer | developer/admin only |

## Source structure and risks

The SPA has 46 JSX page modules. Largest pages include
`RecordsIntelligence.jsx` (2,905 lines), `Chat.jsx` (1,470), and
`AgentChat.jsx` (1,050). `CaseWorkspace.jsx` is the governed shell and embeds
Records Intelligence only for Analyze.

Current source uses `/api/v1/forensics/cases` as authoritative case discovery.
The deployed image still displays four cleanup candidates; the source preview
shows only `nexusai-forensic-demo`. Collection administration remains separate.

## Browser acceptance baseline

- Deployed Overview: no broken images, no console warnings/errors, and zero
  page-level horizontal overflow at 390/820/1024/1440.
- Source preview: governed single case and correct 9,272-row state.
- Source regression: loading state does not claim zero data; focused spec 5/5.

## Migration sequence

1. Complete R2 shell/navigation/state contracts, then implement the R3 shell.
2. Keep one URL-bound case contract across the R4 workspace.
3. Build the R5 Evidence and ingestion experience before beginning Ask NexusAI.
4. In R6, extract the Analyze desk into query composer, capability shortcuts,
   response, and advanced data-management modules. Treat the deferred typed
   Agent Chat lifecycle as a bounded later slice such as R6.2.
5. Add typed timeline/map/graph/media result components only as their evidence
   families become operational.
6. Move engine-centric LocalAI pages under explicit NexusAI administration.

## Shared-state audit

| State | Current owner | Persistence | Required target |
| --- | --- | --- | --- |
| Auth/user/feature access | `AuthContext` plus server APIs | server/session | Server authoritative; UI guards only improve navigation. |
| Branding | `BrandingContext` and public branding API | server configuration | R2 contract, R3 shell consumer, safe pre-auth fallback. |
| Theme | `ThemeContext` | browser local storage | User preference only; must not encode evidence state. |
| Long operations | `OperationsContext` | server polling plus in-memory UI | Truthful operation state with reconnect-safe identifiers. |
| Active case | route parameter in Case Workspace | URL/server request | One explicit case-context contract across every case route and request. |
| Agent/chat histories | page hooks | browser local storage | R6 must separate convenience drafts from server/audit-backed saved analyses. |
| Media histories and selectors | page hooks/local storage | browser | Non-authoritative convenience state; governed evidence must come from case APIs. |

The provider tree is branding → auth → operations → router. There is no global
case provider today; that is acceptable only while the URL remains the single
case selector and children receive explicit case identifiers. R2 must prohibit a
second default collection or local-storage case fallback.
