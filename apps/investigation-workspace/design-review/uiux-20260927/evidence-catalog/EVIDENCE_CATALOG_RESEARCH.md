# Evidence Catalog — research and implementation ruling

Date: 2026-09-27  
Slice: 4.5 only  
Contract: `docs/ux/NEXUSAI_PRODUCT_UX.md` §5, §6.4, §10–13

## Research question

How should an investigator find, assess, and open a source item quickly without
the interface overstating completeness, hiding failed processing, or weakening
evidentiary identity?

This is an inventory and triage surface, not a gallery. The dominant object is
the source item. Search, family, processing state, pagination, and intake are
controls over that inventory. Detailed lineage belongs on the item route.

## Live backend contract — inspected before design

The existing `GET /evidence` endpoint already supports the useful catalog
operations the old UI ignored:

- authoritative server search `q` across filename, source file, SHA-256, KB
  reference, and metadata;
- exact `modality`, `detected_type`, and `processing_status` filters;
- offset pagination with a clamped maximum page size of 250;
- `pagination.returned`, `has_next`, `has_previous`, `offset`, `limit`, and an
  authoritative filtered `evidence_total`;
- a filtered summary containing completed, processing, queued, failed,
  structured/media totals and total bytes;
- per-item original/source filename, family/modality, byte size, SHA-256,
  processing status, timestamps, accepted/total rows, and retained asset state.

The previous page fetched 100 items once, filtered them in the browser, and
showed family-chip counts derived from that bounded page. On a case with more
than 100 items those numbers could be mistaken for collection totals and a
"no match" could be false. It also fetched `/collections/status` through
`ProcessingWait` and repeated a second, bounded evidence list below the catalog.

Backend gaps that constrain this slice:

- no family-count rollup for every exact `detected_type`; therefore family
  chips must not display invented or page-local totals;
- no catalog sort parameter; preserve the endpoint's authoritative newest-first
  order and do not ship a cosmetic client sort over one page;
- no retry/reprocess endpoint; failed rows can be opened for diagnosis but must
  not show a fake Retry control;
- no mutation endpoint for correcting a classifier decision;
- no server-projected `added_by` display identity; the raw internal `user_id`
  is not promoted to a human label;
- no derived-artifact count in the list payload; asset presence is distinct
  from a count and is not presented as one;
- upload accepts one multipart file per request. Multi-file orchestration is a
  future UI slice, not something this catalog can claim today.

## Standards and design-system evidence

### W3C WAI

The [WAI tables tutorial](https://www.w3.org/WAI/tutorials/tables/) requires
native table structure with header/data-cell relationships so assistive
technology retains row context. The
[sortable-table pattern](https://www.w3.org/WAI/ARIA/apg/patterns/table/examples/sortable-table/)
also warns that sort controls must be real buttons and `aria-sort` must describe
the active direction. Since the API has no sort contract, this slice uses a
semantic table but does not pretend its headings are sortable.

Applied ruling: desktop/tablet uses a real `<table>` with a caption and scoped
headers. At narrow widths the same source data becomes labelled records rather
than forcing page-level horizontal scroll. Status always has icon and text.

### IBM Carbon

Carbon's current
[data-table guidance](https://carbondesignsystem.com/components/data-table/usage/)
places table-wide search, filters, and utilities in one toolbar; recommends
giving dense tables the main content width; places pagination at the table
bottom; and treats the row as the route to a specific resource. Carbon also
advises open search when search is a frequent operation and simple previous/next
pagination when that is all the product can truthfully support.

Applied ruling: one inventory frame contains title/summary, an always-visible
server search, exact family/status filters, the result set, and endpoint-backed
previous/next controls. There are no unsupported selection, export, display,
batch, or sorting controls.

## Comparable investigation products

### Microsoft Defender XDR

The current
[incident investigation guidance](https://learn.microsoft.com/en-us/defender-xdr/investigate-incidents)
keeps evidence/entities in a dedicated working view, marks analyzed entities
with explicit verdict and remediation state, opens detail in context, and shows
an explicit empty result when filters remove every entity. Its wider priority,
assignee, severity, verdict, and remediation model is supported by Defender
services; NexusAI does not have those sources and must not imitate their badges.

Applied ruling: explicit state + direct detail navigation + loud filtered-empty
state. Rejected: invented severity, owner, verdict, remediation, or priority.

### Elastic Security

Elastic's
[case-management workflow](https://www.elastic.co/guide/en/security/current/cases-open-manage.html)
uses searchable resource tables and a purposeful in-table empty action. Its
investigation flows preserve the source events behind grouped findings instead
of making an aggregate appear to be the complete source set.

Applied ruling: inventory search and filter state stays visible; the empty case
offers intake, while a filtered empty result offers a reset and explicitly says
scope was not widened.

### Magnet AXIOM

Magnet's official
[AXIOM trial guidance](https://www.magnetforensics.com/axiomtrialresources/)
uses the Case Dashboard to orient analysts, then provides evidence-source and
artifact-first paths into focused examination. The user guide also emphasizes
viewing an artifact's source rather than treating a derived result as its own
unrelated item.

Applied ruling: this page remains source-item inventory. Derived artifacts and
lineage are disclosed on evidence detail, not flattened into duplicate source
rows.

### Cellebrite

Cellebrite Reader's current
[investigator workflow](https://cellebrite.com/en/products/cellebrite-inseyets/reader/)
combines cross-artifact search with type/date filtering and persistent analyst
review aids. Cellebrite Guardian's
[evidence-management contract](https://cellebrite.com/en/products/guardian/)
centres case visibility, chain of custody, and event logging. NexusAI does not
have server-persisted tags/bookmarks, so those controls are not copied.

Applied ruling: optimize for find/open/verify. Rejected: browser-only evidence
tags masquerading as collaborative case data.

## Forensic evidence-management evidence

[NIST IR 8387](https://www.nist.gov/publications/digital-evidence-preservation-considerations-evidence-handlers)
identifies original-source documentation and cryptographic hashes as core
digital-preservation concerns. NIST's
[evidence-management overview](https://www.nist.gov/forensic-science/interdisciplinary-topics/evidence-management)
states that integrity and custody must be tracked at every stage. INTERPOL's
[digital-forensics laboratory guidance](https://www.interpol.int/content/download/13501/file/INTERPOL_DFL_GlobalGuidelinesDigitalForensics)
requires unique evidence labels, inventory, custody records, and matching
hashes for original and preserved copies.

Applied ruling: filename is primary, evidence ID remains available as the stable
identifier, size and added time orient the analyst, and the detail route owns
the full hash/custody proof. The catalog never truncates or transforms the
underlying identifier into a friendlier invented label.

## Final design ruling

1. Put `Add evidence` in the task-header action and keep the real drop target in
   a collapsible intake region on the same route.
2. Use one wide inventory surface, not a stack of cards and not a duplicate
   processing panel.
3. Send search, family, status, page size, and offset to `GET /evidence`.
4. Reset offset before every scope/search/status/page-size change.
5. Debounce search, while retaining explicit submit and clear controls.
6. Show authoritative filtered total and an exact displayed range. Never call a
   page "all evidence" when `has_next` is true.
7. Use the exact family vocabulary from the UX contract. Counts on chips are
   omitted because the endpoint does not provide exact per-family totals.
8. Status filter uses only endpoint values that map one-to-one: all, ready
   (`completed`), processing, and failed. Queued remains visible in the
   inventory and summary, but `registered OR queued` cannot be expressed by
   this endpoint as one honest filter. `skipped`/`duplicate` display as
   Excluded, but the service has no single excluded query value.
9. A row exposes only one action, `Open evidence`; the filename also links to
   the same governed detail route.
10. Failed rows remain in totals and results. Without a reprocess endpoint the
    honest action is to open the item and inspect its recorded failure.
11. Empty-unfiltered and empty-filtered are different states. Filtered empty
    explicitly states that scope was not widened and offers `Clear filters`.
12. At 768–1279 the filename column is pinned. Below 768 the catalog becomes
    labelled stacked records with 44px targets and no page-level overflow.

## Acceptance for this slice

- request assertions prove that search/family/status/offset/limit reach the
  existing endpoint;
- pagination wording is derived from response metadata, including a last page;
- a result beyond the old first 100 is reachable using Next;
- filtered empty remains scoped and can be reset;
- failed evidence stays visible with text/icon state and no Retry control;
- table semantics are asserted at desktop and mobile has no horizontal page
  scroll;
- both themes are visually captured at 1440 and 390;
- full unit, ported, build, and route/breakpoint sweeps remain green.
