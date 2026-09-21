# NexusAI Agent Chat question catalog

Date: 2026-08-05  
Runtime collection: `nexusai-forensic-demo`  
Contract: 6 specialist agents, 65 accepted operations

## How to use Agent Chat

Type the question normally. You do **not** need to know a template name, add a
`limit`, select a model, or write API syntax. Agent Chat infers the operation,
extracts an identifier when present, applies bounded defaults, and sends the
query to the appropriate deterministic forensic engine. The template column
below is diagnostic information for testers; it is not text the user must type.

Ordinary Agent Chat questions now request bounded local-model explanation after
deterministic results are produced. Counts, rows, dates, citations, hashes, and
limitations remain authoritative; the model cannot add evidence or change
them. If generated prose fails a forensic guard, Agent Chat shows an explicit
policy fallback and still returns the complete professionally formatted factual
answer. A valid query can return `no results` when the retained evidence has no
match.

Useful identifiers in the full demo are:

- CDR: `923001110001`, `923001110002`, tower-linked `923221110001`
- ANPR plate: `ABC-123`
- Subscriber: `923000000001`, reference `PK-SUB-SYN-ALPHA`
- Tower/site: `PK-LHR-SYN-001`

## Forensic Records Analyst (23 questions)

This is the case-wide and cross-family agent. It can also answer every
specialist question in the later sections.

| # | Accepted natural-language question | Inferred operation |
|---:|---|---|
| 1 | Show me an overview of this case | `collection_overview` |
| 2 | What activity exists for 923001110001? | `entity_activity` |
| 3 | What evidence-backed relationships exist for 923001110001? | `relationship_network` |
| 4 | Correlate 923001110001 across the available evidence families | `cross_family_correlation` |
| 5 | What happened involving 923001110001 over time? | `entity_timeline` |
| 6 | Show the source rows involving 923001110001 | `source_records` |
| 7 | Show normalized CDR records | `canonical_records` |
| 8 | What schemas and fields were detected? | `schema_profile` |
| 9 | What data quality, rejection or duplicate issues exist? | `data_quality` |
| 10 | Show the evidence inventory and lineage | `evidence` |
| 11 | When was 923001110001 first and last observed? | `first_seen_last_seen` |
| 12 | Show daily activity for 923001110001 | `activity_by_day` |
| 13 | Is there cross-family co-presence involving 923001110001? | `co_travel_or_co_presence` |
| 14 | Which deterministic patterns should an analyst review? | `suspicious_patterns` |
| 15 | Summarize the deterministic anomalies | `anomaly_summary` |
| 16 | Summarize 923001110001 across datasets | `cross_dataset_entity_summary` |
| 17 | Which files were ingested and did their row counts reconcile? | `source_file_audit` |
| 18 | Were any source files uploaded more than once? | `duplicate_upload_audit` |
| 19 | Is this case ready for analyst demonstration? | `case_readiness` |
| 20 | What is included in the evidence package? | `evidence_package_summary` |
| 21 | Prepare an executive case brief from deterministic findings | `executive_case_brief` |
| 22 | Prepare a court-ready source and provenance summary | `court_ready_source_summary` |
| 23 | What limitations and missing data affect this case? | `limitations_and_data_quality` |

## Communications CDR Analyst (16 questions)

| # | Accepted natural-language question | Inferred operation |
|---:|---|---|
| 24 | Who are the most frequent contacts? | `frequent_contacts` |
| 25 | How are the CDR events divided by call type? | `call_type_breakdown` |
| 26 | What services were used by 923001110001? | `service_usage` |
| 27 | Did 923001110001 change IMEI or IMSI? | `device_identity_changes` |
| 28 | When was 923001110001 active in the CDR data? | `temporal_activity` |
| 29 | Which supplied locations appear most often for 923001110001? | `top_locations` |
| 30 | Show the chronological supplied locations for 923001110001 | `geospatial_movement` |
| 31 | What is the shortest call? | `shortest_call` |
| 32 | What is the longest call? | `longest_call` |
| 33 | Show both the shortest and longest calls | `duration_extremes` |
| 34 | Show hourly activity for 923001110001 | `activity_by_hour` |
| 35 | Show night-time activity for 923001110001 | `night_activity` |
| 36 | Which supplied locations recur for 923001110001? | `repeated_location_visits` |
| 37 | What identifiers does the CDR show for 923001110001? | `subscriber_profile` |
| 38 | How were IMEI and IMSI identifiers used by 923001110001? | `imei_imsi_usage` |
| 39 | Which CDR cells or sites observed 923221110001? | `tower_activity` |

CDR observations do not prove phone ownership, speaker identity, subscriber
location, tower coverage, communication content, motive, or guilt.

## Network IPDR Capture Analyst (7 questions)

| # | Accepted natural-language question | Inferred operation |
|---:|---|---|
| 40 | Which network endpoints appear in the IPDR data? | `ipdr_endpoint_summary` |
| 41 | Which domains appear in the network records? | `ipdr_domain_summary` |
| 42 | How is the network traffic divided by protocol? | `ipdr_protocol_breakdown` |
| 43 | What is the IPDR session and byte volume? | `ipdr_session_volume` |
| 44 | Show network sessions for subscriber 923001110002 | `ipdr_subscriber_sessions` |
| 45 | Were there overlapping sessions for 923001110002? | `ipdr_concurrent_sessions` |
| 46 | Build the network session timeline for 923001110002 | `ipdr_timeline` |

NAT and shared infrastructure can prevent one-to-one attribution. An observed
domain or IP does not establish content, intent, maliciousness, or actor identity.

## Vehicle ANPR Geospatial Analyst (7 questions)

| # | Accepted natural-language question | Inferred operation |
|---:|---|---|
| 47 | Where and when was plate ABC-123 observed? | `anpr_sightings` |
| 48 | Which cameras observed ABC-123 in time order? | `anpr_camera_sequence` |
| 49 | Which ANPR cameras have the most observations? | `anpr_camera_activity` |
| 50 | Which plates were observed at the same camera near ABC-123? | `anpr_co_travel` |
| 51 | What were the time gaps between consecutive sightings of ABC-123? | `anpr_route_timing` |
| 52 | What plate variants were recorded for ABC-123? | `anpr_plate_variants` |
| 53 | Build the ANPR timeline for ABC-123 | `anpr_timeline` |

ANPR rows are observations, not proof of driver, passenger, ownership,
continuous route, association, or unlawful conduct.

## Subscriber Identity Analyst (6 questions)

| # | Accepted natural-language question | Inferred operation |
|---:|---|---|
| 54 | What subscriber observations exist for 923000000001? | `subscriber_identity_lookup` |
| 55 | When was subscriber PK-SUB-SYN-ALPHA valid? | `subscriber_validity_timeline` |
| 56 | Which SIM and device identifiers are linked to 923000000001? | `subscriber_device_links` |
| 57 | How many subscriber records are active, inactive or suspended? | `subscriber_status_summary` |
| 58 | Are there conflicting subscriber identity attributes? | `subscriber_conflict_audit` |
| 59 | Which subscriber identifiers appear with multiple phone numbers? | `subscriber_reuse_candidates` |

Allowed lookup keys are MSISDN, subscriber reference, IMSI, and IMEI. Raw CNIC
and subscriber-name lookup are prohibited. A supplied link does not prove
ownership, fraud, SIM swap, reassignment, or device sharing.

## Tower Location Reference Analyst (6 questions)

| # | Accepted natural-language question | Inferred operation |
|---:|---|---|
| 60 | What reference facts exist for tower PK-LHR-SYN-001? | `tower_site_lookup` |
| 61 | Show the reference history for PK-LHR-SYN-001 | `tower_reference_timeline` |
| 62 | Which tower coordinates, datums or uncertainty values require review? | `tower_coordinate_audit` |
| 63 | How are tower references divided by status and technology? | `tower_status_summary` |
| 64 | Are any tower aliases associated with conflicting supplied facts? | `tower_alias_conflicts` |
| 65 | Join CDR observations to the valid tower reference for PK-LHR-SYN-001 | `tower_cdr_join` |

Tower coordinates are supplied reference facts. They do not prove RF coverage,
device presence, handset position, subscriber location, or service availability.

## Model-assistance acceptance questions (one per agent)

Every ordinary question uses the same bounded explanation policy. These six
prompts deliberately exercise interpretation and family-specific safeguards.
CPU-only Qwen responses can take longer than deterministic computation,
especially on the first request after a restart.

| Agent | Copy-ready question |
|---|---|
| Forensic Records Analyst | Explain the review workflow supported by deterministic data quality analysis. Do not restate values or invent causes. |
| Communications CDR Analyst | Explain how an analyst should use the deterministic frequent contacts analysis. Do not restate values or infer relationships or communication content. |
| Network IPDR Capture Analyst | Explain the review workflow supported by the deterministic network protocol breakdown. Do not restate values or infer payload content or maliciousness. |
| Vehicle ANPR Geospatial Analyst | Explain the review workflow supported by deterministic ANPR camera activity. Do not restate values or infer a driver, owner, or route. |
| Subscriber Identity Analyst | Explain how an analyst should use the deterministic subscriber status summary when reviewing supplied sources. Do not restate values. |
| Tower Location Reference Analyst | Explain the review workflow supported by the deterministic tower status summary. Do not restate values or infer RF coverage or device presence. |

## Accepted runtime result

The complete natural-language matrix passed all 65 routes against
`nexusai-forensic-demo`: 64 returned data and one returned accepted `no results`;
there were no route mismatches, incomplete requests, or stuck deterministic
queries. The records-API route-check p95 was 257 ms. The real Agent Chat plus
request-correlated SSE matrix also passed 65/65 with 9,815 ms p95 and 22,916 ms
maximum. Explicit `template=...` and manual `limit=...` syntax remains available
only as an optional API/debugging mechanism and is not the normal Agent Chat
experience.
