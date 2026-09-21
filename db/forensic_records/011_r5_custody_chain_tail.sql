-- Select the actual unreferenced hash-chain tail when appending custody events.
-- UUID order is not insertion order, so timestamp/UUID sorting is insufficient
-- when several events are recorded in the same database timestamp resolution.
BEGIN;

CREATE OR REPLACE FUNCTION forensic.prepare_custody_event()
RETURNS trigger AS $$
DECLARE
  prior_hash text;
BEGIN
  PERFORM pg_advisory_xact_lock(hashtextextended(
    NEW.tenant_id || E'\n' || NEW.collection_id || E'\n' || NEW.evidence_id::text,
    0
  ));
  SELECT event.event_sha256
    INTO prior_hash
  FROM forensic.evidence_custody_events event
  WHERE event.tenant_id = NEW.tenant_id
    AND event.collection_id = NEW.collection_id
    AND event.evidence_id = NEW.evidence_id
    AND NOT EXISTS (
      SELECT 1
      FROM forensic.evidence_custody_events child
      WHERE child.tenant_id = NEW.tenant_id
        AND child.collection_id = NEW.collection_id
        AND child.evidence_id = NEW.evidence_id
        AND child.previous_event_sha256 = event.event_sha256
    )
  ORDER BY event.occurred_at DESC, event.custody_event_id DESC
  LIMIT 1;
  NEW.previous_event_sha256 := prior_hash;
  NEW.event_sha256 := encode(digest(convert_to(concat_ws(E'\n',
    coalesce(prior_hash, ''), NEW.custody_event_id::text, NEW.tenant_id,
    NEW.collection_id, NEW.evidence_id::text, NEW.version_id::text,
    coalesce(NEW.source_link_id::text, ''), coalesce(NEW.run_id::text, ''),
    coalesce(NEW.artifact_id::text, ''), NEW.event_type, NEW.actor_type,
    NEW.actor_id, coalesce(NEW.custody_location, ''), coalesce(NEW.reason, ''),
    NEW.occurred_at::text, NEW.payload::text
  ), 'UTF8'), 'sha256'), 'hex');
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

COMMIT;
