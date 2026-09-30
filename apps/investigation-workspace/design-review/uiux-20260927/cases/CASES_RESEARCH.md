# Cases directory — research and implementation ruling

Date: 2026-09-28  
Scope: `/cases` only. Dashboard remains the action-oriented starting point; Cases is the exhaustive directory of configured collection-backed workspaces.

## Sources reviewed

1. [IBM Carbon — Data table usage](https://carbondesignsystem.com/components/data-table/usage/)
   - Put search and filtering in the table toolbar.
   - Keep the row structure concise and expose visible row actions when the action set is small.
2. [U.S. Web Design System — Table](https://designsystem.digital.gov/components/table/)
   - Tables suit directories and comparison of consistently structured records.
   - Minimize columns, use native row and column headers, announce sort changes, make scrollable regions keyboard-focusable, and stack directory rows at small widths.
3. [W3C APG — Grid pattern](https://www.w3.org/WAI/ARIA/apg/patterns/grid/)
   - Arrow-key navigation can reduce tab stops in interactive tabular data, but a full grid role requires comprehensive focus management.
   - NexusAI retains native table semantics and adds Up/Down/Home/End movement only between the primary case links.
4. [Microsoft Purview — Create and manage eDiscovery cases](https://learn.microsoft.com/en-us/purview/edisc-cases-manage)
   - A forensic case directory prioritizes quick access to status, filtering and meaningful recency.
   - NexusAI cannot copy Purview's name, owner, case type or case-modified columns because its service does not expose those fields.
5. [USWDS — Data visualizations](https://designsystem.digital.gov/components/data-visualizations/)
   - Visual summaries need equivalent accessible data. The readiness and family bars therefore carry explicit text alternatives and are paired with exact counts.

## Measured defects before this pass

- The Cases slice was functionally correct but visually indistinguishable from a basic administration table.
- Evidence readiness and evidence-family composition appeared only as text and pills, making cross-row comparison slow.
- The endpoint already exposed bounded evidence/job timestamps, but the directory did not show the most recent evidence lifecycle activity.
- Restricted collections still rendered navigation links even though their status locator was not openable.
- Status used a coloured dot plus label; the new treatment adds a distinct icon, label and surface.
- Keyboard users could tab through actions but could not move efficiently between primary case rows.
- Persistent explanatory copy competed with the directory; the service boundary is now a compact disclosure.

## Ruling

- Keep one concise command surface: search, readiness filters, result count, refresh and clear.
- Default to latest reported evidence lifecycle activity, with null values last. Label it “Evidence activity”; never call it case activity or source-event time.
- Present five comparable columns: Case, Evidence, Evidence families, Evidence activity and Next action.
- Use exact values plus accessible segmented bars for evidence readiness and accepted rows by curated evidence family.
- Preserve raw collection IDs and curated family labels. Do not manufacture case names or labels.
- A 403 collection remains visible as “Access restricted”, is not counted as zero and does not render an openable case/action link.
- Keep native table semantics. Up/Down/Home/End move between openable primary case links; normal Tab behavior remains available for every action.
- Pin the first column from 768–1279 px; below 768 px, retain the table DOM while visually stacking each directory row.
- Keep every target at least 44 px, no page-level horizontal scroll at 375 px, and no decorative chart unsupported by exact text.

## Backend fields used

- Browser-served configuration: configured collection IDs only.
- `GET /collections/status`: evidence totals, completed/in-flight/failed counts, accepted rows, curated record-family identifiers and bounded `recent_evidence` / `recent_jobs` lifecycle timestamps.

## Missing contracts

There is no authoritative case entity, cross-case list endpoint, case display name, owner/assignee, member list, classification, severity, investigative priority, permission summary or case-updated timestamp. Those columns remain absent rather than populated with plausible values. A collection becomes case-like only when its first evidence file is accepted.
