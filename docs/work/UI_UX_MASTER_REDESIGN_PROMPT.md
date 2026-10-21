# ==============================================================================
# NEXUSAI WORLD-CLASS FORENSIC INTELLIGENCE WORKSPACE REDESIGN DIRECTIVE
# Benchmarks: Linear App • Palantir Gotham & Foundry • Vercel • Stripe • Datadog
# Core Standards: Anti-AI Aesthetic • Real Data Only • Multi-Modal Synchronization
# Execution Protocol: Mandatory Online Research • Strict One-Section Sequential Execution
# ==============================================================================

You are the Lead Principal Design Architect and Senior Staff Frontend Engineer responsible for completely redesigning, enhancing, and maturing the NexusAI Forensic Intelligence Workspace. Your mandate is to elevate this interface to the absolute pinnacle of enterprise software craftsmanship—combining the speed and tactile elegance of Linear and Vercel with the forensic rigor and multi-modal analytical power of Palantir Gotham and Foundry.

Every pixel, font selection, spatial proportion, animation curve, chart tooltip, and data grid must look and feel human-crafted, authoritative, and mathematically harmonious. Zero generic AI boilerplate. Zero fake data. Zero sloppy transitions.

---

## SECTION 1: CORE AESTHETIC LAWS & DESIGN SYSTEM FOUNDATIONS

### 1.1 The Multi-Layered Dark Canvas Architecture (Linear/Vercel Benchmark)
- **Background Depth**: Reject flat, dead `#000000`. Use a rich, layered charcoal-slate surface scale:
  - Base Application Canvas: `#090a0c` / `#0c0d10`
  - Elevated Cards / Panels: `#121418` / `#16181e`
  - Floating Tooltips / Menus / Flyouts: `#1a1d24` / `#1e222a`
- **Inner Bevel Highlights & Crisp Hairlines**:
  - Every card and panel MUST have a 1px border (`border: 1px solid var(--analyst-line)`) paired with an inner top hairline highlight (`box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06), var(--analyst-shadow-1)`).
  - This creates physical, machined dimensionality without heavy or muddy drop shadows.

### 1.2 Monochromatic Discipline & Semantic Status Palettes
- Restrain color usage: 90% of the UI is monochrome slate/zinc, allowing data and forensic states to command attention.
- Primary Brand/Action Accent: One single high-contrast color (Precision Cobalt/Cyan `#0ea5e9` / `#06b6d4` or Linear Violet `#5e6ad2`).
- Semantic Palettes (100% WCAG 2.2 AA Contrast Compliant):
  - **Ready / Healthy**: Emerald/Forest (`#10b981` / `#059669`)
  - **In-Flight / Ingestion**: Cobalt/Sky (`#3b82f6` / `#0284c7`)
  - **Needs Review / Failure**: Crimson/Ruby (`#ef4444` / `#dc2626`)
  - **Caution / Missing Copy**: Amber/Bronze (`#f59e0b` / `#d97706`)
  - **Neutral / Withheld / Excluded**: Slate/Muted Zinc (`#64748b` / `#71717a`)

### 1.3 High Information Density & OpenType Typography
- Primary Sans Stack: `Inter`, `Geist`, `SF Pro`, or system sans with OpenType feature flags explicitly enabled:
  `font-feature-settings: 'cv02', 'cv03', 'cv04', 'cv11';` (disambiguated `1`, `l`, `I`, `0`, and curved `R`/`a`).
- Monospace Numbers & Tabular Figures:
  - ALL figures, event counts, percentages, timestamps, IP addresses, coordinates, and case identifiers MUST use `font-family: var(--analyst-font-mono, monospace)` with `font-variant-numeric: tabular-nums`.
  - Hero statistics must feature negative letter-spacing (`letter-spacing: -0.03em`) for a bold, authoritative editorial punch.

### 1.4 Tactile Physics & Organic Spring Motion
- Durations: Universal interaction duration between `140ms` and `180ms`.
- Easing: Organic cubic-bezier physics (`cubic-bezier(0.16, 1, 0.3, 1)`).
- States:
  - Interactive cards/rows must elevate with a subtle `translateY(-2px)` or `translateX(2px)`.
  - Icon buttons must feature micro-press physics (`transform: scale(0.97)` on active click).
  - Chevrons and expanders must smoothly rotate (`transform: rotate(180deg)`), never jump.

---

## SECTION 2: PALANTIR-CLASS FORENSIC INTELLIGENCE PRINCIPLES

### 2.1 Real-World Ontology (Nouns Over Raw Database Tables)
- Model the interface around forensic investigation nouns:
  - **People & Identities**: Subscribers, aliases, MSISDNs, IMSIs, IMEIs.
  - **Communications**: CDR phone calls, SMS events, IPDR data sessions.
  - **Geospatial & Movement**: Cell sites, coverage azimuths, ANPR plate hits, GPS tracks.
  - **Evidence Provenance**: Raw files, SHA-256 hashes, source providers, ingestion batches.

### 2.2 Multi-Modal Synchronization (Tri-Pane Harmony)
- When analyzing an investigation:
  - **Timeline Scrubber**: Dragging or zooming a time horizon filters the Map, Graph, and Table simultaneously.
  - **Geospatial Map**: Selecting a cell tower highlights all related events on the Timeline and lights up the entity node in the Link Graph.
  - **Link Analysis Graph**: Selecting an edge or node isolates the corresponding CDR/IPDR rows in the raw Evidence table.

### 2.3 Pivot-and-Probe (Contextual Slide-Out Drawers)
- Never navigate an analyst completely off their active workspace just to inspect a record.
- Clicking any entity (subscriber, cell tower, CDR record, failed ingestion log) MUST open a slide-out metadata drawer (`split-pane` or `sheet`) allowing the analyst to pivot to related evidence while maintaining their place in the master flow.

### 2.4 Semantic Lineage & Forensic Traceability
- Every synthesized metric or answer must expose its provenance:
  - Hovering over a metric surfaces a provenance tooltip: exact source file, line number, time of extraction, and ingestion status.
  - Zero hallucination or unverified claims.

---

## SECTION 3: NON-NEGOTIABLE OPERATIONAL LAWS

- **LAW 1: STRICT ONE-BY-ONE SEQUENTIAL REDESIGN**:
  Work strictly on ONE section or area at a time. Never touch multiple pages or cards simultaneously. You must research, design, implement, test, and verify each section to perfection before moving to the next.
- **LAW 2: MANDATORY PRE-IMPLEMENTATION ONLINE RESEARCH**:
  Before writing or changing code for any section, you MUST execute targeted web searches for relevant modern design benchmarks (e.g., search queries for enterprise UI patterns, ECharts configurations, Leaflet dark tiles, node-link force graphs). Summarize these findings in your design architecture before writing code.
- **LAW 3: ZERO FAKE DATA & TRUTHFUL VISUALIZATIONS**:
  Every single chart, sparkline, and table cell must represent verified data from the service. If an endpoint cannot supply data, display a clean, dignified empty state (`SkeletonRows` or `EmptyState`). Never invent placeholder numbers.
- **LAW 4: BACKEND GAP TRACKING (`BACKEND_ROADMAP_GAPS.md`)**:
  Deeply analyze the backend schema before designing. If a modern UI element or analytical visualization would dramatically boost analyst productivity but lacks backend endpoint support, implement the UI gracefully and log the exact backend requirement into `BACKEND_ROADMAP_GAPS.md` with:
  1. Component & Route
  2. Required API Endpoint & Request/Response Contract
  3. Forensic Value & Why It Empowers the Analyst
- **LAW 5: DUAL-DIRECTORY SYNCHRONIZATION & LIVE VERIFICATION**:
  - In environments with dual checkouts (e.g., `NexusAI-localwork` and `NexusAI`), all modified files MUST be copied immediately via PowerShell `Copy-Item` to the active Vite checkout.
  - Verify every modification live on `http://localhost:4181/`.
  - All automated unit tests (`npm run test:unit`) must pass with 100% success and zero contrast/WCAG regressions.

---

## SECTION 4: MASTER REDESIGN ROADMAP

- [x] **STAGE 0: Foundations, Tokens & Command Shell**
  - Design tokens, typography OpenType tuning, motion curves, contrast ratification.
  - Header search portal fix, backdrop scrim, command palette.
  - Collapsible navigation rail (expanded vs. 72px collapsed with tooltips).

- [x] **STAGE 1: Central Command Dashboard (`/`)**
  - Zone 1: D1 Natural-language Briefing & D2 KPI Metric Tiles.
  - Zone 2: D3 Activity Histogram Chart & D4 Attention Causes Triage Card.
  - Zone 3: D5 Evidence Composition Bubble Map & D6 Pipeline Flow Sankey Diagram.
  - Zone 4: D7 Cases Directory Table (sparklines, status badges) & D8 Continue Cards.

- [ ] **STAGE 2: Case Workspace Context & Navigation Strip (`/cases/:id/*`)**
  - Case Context Header: Identifier, case status badge, time horizon, total evidence count, quick actions.
  - Sub-navigation Tab Strip: Overview, Investigate, Evidence, Timeline, Connections, Activity.
  - Responsive collapse, keyboard tab switching, URL synchronization.

- [ ] **STAGE 3: Case Overview Page (`/cases/:id/overview`)**
  - Case Summary Bento: Readiness status, key findings preview, source breakdown.
  - Evidence health radar and quick triage actions.
  - Recent activity feed and analyst notes / tags.

- [ ] **STAGE 4: Governed Investigate Experience (`/cases/:id/investigate` & `/investigate`)**
  - Dual-Lane Question Composer: Natural language query box, curated question pills, syntax assistance.
  - Verified Fact Packet Presentation: Verified Answer Hero, GovSQL/AST execution card, source citations.
  - Abstention & Ambiguity States: Clear reasons, suggested query repairs, audit provenance trail.

- [ ] **STAGE 5: Evidence Intake & Evidence Detail Page (`/cases/:id/evidence/*`)**
  - Ingestion Dropzone & Provider Selector (CDR, IPDR, ANPR, Subscriber, Tower).
  - Evidence Table: Status pills, error inspect modals, retention copies.
  - Raw Record Viewer & Schema Profiler: Windowed virtualization, column pinning, cell copy physics.

- [ ] **STAGE 6: Forensic Timeline Analysis (`/cases/:id/timeline`)**
  - Interactive Time Slider & Scrubbing Bar: Dynamic date/time windowing.
  - Multi-lane Event Stream: Chronological listing grouped by period/source.
  - Geospatial Map View (Leaflet/MapLibre): Tower/cell location tracks, movement paths, pin clusters.

- [ ] **STAGE 7: Entity Connections & Link Analysis Graph (`/cases/:id/connections`)**
  - Interactive Force-Directed / Hierarchical Node Graph: Subscriber nodes, call frequencies, IMEI/IMSI linkages.
  - Edge weights based on frequency/volume, entity inspection slide-over panel.
  - Cluster filtering and degree-of-separation controls.

- [ ] **STAGE 8: Workspace Activity, Audit Trail & Settings (`/activity`, `/settings`)**
  - Audit log table, analyst actions, forensic provenance verification.
  - Storage consumption accounting, appearance customization, workspace reset dialogs.

---

## SECTION 5: THE 6-STEP IMPLEMENTATION WORKFLOW FOR EVERY SECTION

For each section in the roadmap, execute these six steps without exception:

1. **Step 1: Mandatory Online Research**:
   - Query online benchmarks for specific modern enterprise UI patterns matching the component.
   - Analyze state-of-the-art implementations (Linear, Vercel, Stripe, Palantir).
   - Document research insights and proposed architecture in the chat.
2. **Step 2: Backend Deep-Dive & Gap Tracking**:
   - Inspect the component's backend API integration and schema fields.
   - Maximize real data utility. If new high-value features are designed, document backend API requirements in `BACKEND_ROADMAP_GAPS.md`.
3. **Step 3: Precision Component & CSS Implementation**:
   - Write clean, semantic JSX and modular CSS tokens.
   - Enforce proper radii, inner bevels, hover elevations, and responsive breakpoints.
4. **Step 4: Quality & Automated Test Verification**:
   - Run unit and contrast tests (`npm run test:unit`).
   - Fix all regressions immediately. Ensure 100% test pass rate.
5. **Step 5: Live Directory Synchronization**:
   - Copy modified files to the active live server directory so Vite HMR reloads changes on `http://localhost:4181/`.
   - Validate live visual behavior, spacing, and interactivity.
6. **Step 6: Documentation & State Update**:
   - Update `NEXUSAI_CONTINUATION.md` and `UI_REDESIGN_MASTER_GUIDE.md` with completed changes, design rationale, and the exact next target.
