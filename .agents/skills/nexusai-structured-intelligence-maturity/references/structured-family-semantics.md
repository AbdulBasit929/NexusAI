# Structured family semantics

Treat CSV/XLSX/JSON/TSV/Parquet as containers and CDR, IPDR, subscriber, tower/location, structured ANPR, financial, access/security, and generic tabular data as semantic families.

## Family boundaries

- CDR: communications events between typed parties with direction, service/event, time, duration, device/SIM, and cell/site attributes where supplied. Separate provider/export schemas from the canonical CDR contract.
- IPDR: network/session events with source/destination IP, ports, protocol/service, identifiers, volume, timestamps, IPv4/IPv6, and NAT attributes only when supplied.
- Subscriber/identity: source-declared or observed links among MSISDN, IMSI, ICCID, IMEI, subscriber reference/name, provider, effective dates, and status. Observation is not ownership.
- Tower/location: time-valid cell/site/tower/sector references with coordinates, datum, uncertainty, source version, and explicit unmatched/ambiguous states. An observation is not continuous movement or RF presence.
- Structured ANPR: existing exact plate observations remain eligible; do not begin image/model expansion.
- Other existing structured families: inventory and mature only those demonstrably implemented in current source.

## Relationship strength

Use explicit labels such as `exact_match`, `source_declared_relationship`, `observed_association`, `shared_contact`, `shared_device_observation`, `temporal_overlap`, `spatial_overlap`, `candidate_correlation`, `conflict`, and `no_evidence`. Never let aggregation, graph weight, UI language, or model prose strengthen certainty.
