# Pakistan-first domain profile

Centralize country-specific behavior in a versioned Pakistan domain profile or the compatible existing registry. Do not scatter Pakistan assumptions through parsers, SQL, query routing, or React.

Cover only supported facts: telephone representations and `+92`/national normalization, allocation/prefix reference metadata, provider metadata, timezone/local-date vocabulary, telecom terms, tower/site conventions where authoritative, Urdu and Roman Urdu terminology, analyst phrases, network identifiers, and future regulatory enrichment.

Always preserve raw phone representation, canonical representation, normalization policy/version, and provenance. Distinguish source-reported operator, allocation/prefix reference, authoritatively resolved current operator, and unknown. Never infer current operator solely from a prefix.

Keep MSISDN, IMSI, IMEI, ICCID/SIM, subscriber reference, IP, port, cell/site/tower, plate, account/reference, evidence, and source IDs semantically distinct. Protect exact spelling and digits through language and presentation layers.

Unknown timezone is not UTC. Pakistan context alone does not prove PKT. Record the source timezone/offset, assumption policy, normalized instant, analysis timezone, and boundary rules.

Search repository schemas, fixtures, domain docs, reports, and team terminology for INPR and INPRS. If no authoritative meaning exists, record `DomainStatus=NeedsDomainDefinition`; never invent the expansion or block unrelated CDR/IPDR work.
