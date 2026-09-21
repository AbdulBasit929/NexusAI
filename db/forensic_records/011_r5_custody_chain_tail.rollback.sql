-- Restores the pre-011 trigger function. This is schema-only and does not
-- rewrite custody events. Use only if 011 verification fails.
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
  SELECT event_sha256
    INTO prior_hash
  FROM forensic.evidence_custody_events
  WHERE tenant_id = NEW.tenant_id
    AND collection_id = NEW.collection_id
    AND evidence_id = NEW.evidence_id
  ORDER BY occurred_at DESC, custody_event_id DESC
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
