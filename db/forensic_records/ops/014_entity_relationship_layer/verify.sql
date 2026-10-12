-- Read-only verification of migration 014. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  problem text;
BEGIN
  SELECT string_agg(t.name, ', ')
    INTO problem
  FROM (VALUES ('derivation'), ('entity'), ('entity_mention'), ('relationship'), ('relationship_evidence'), ('ip_assignment')) AS t(name)
  WHERE to_regclass('forensic.' || t.name) IS NULL;
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'missing tables: %', problem;
  END IF;

  SELECT string_agg(t.name, ', ')
    INTO problem
  FROM (VALUES ('derivation'), ('entity'), ('entity_mention'), ('relationship'), ('relationship_evidence'), ('ip_assignment')) AS t(name)
  WHERE NOT EXISTS (SELECT 1 FROM pg_class c WHERE c.oid = ('forensic.' || t.name)::regclass AND c.relrowsecurity)
     OR NOT EXISTS (
       SELECT 1 FROM pg_policies p
       WHERE p.schemaname = 'forensic' AND p.tablename = t.name
         AND p.policyname = 'tenant_isolation_' || t.name
         AND p.cmd = 'ALL' AND p.permissive = 'PERMISSIVE'
         AND p.qual IS NOT NULL AND p.with_check IS NOT NULL
     );
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'row-level security or tenant policy missing on: %', problem;
  END IF;

  SELECT string_agg(c.rel || '.' || c.conname, ', ')
    INTO problem
  FROM (VALUES
    ('entity', 'entity_type_chk'), ('entity', 'entity_person_chk'), ('entity', 'entity_merged_fk'),
    ('entity_mention', 'entity_mention_entity_fk'), ('entity_mention', 'entity_mention_evidence_fk'),
    ('relationship', 'relationship_src_fk'), ('relationship', 'relationship_dst_fk'), ('relationship', 'relationship_derivation_fk'),
    ('relationship', 'relationship_type_kind_chk'), ('relationship', 'relationship_derivation_required_chk'),
    ('relationship', 'relationship_undirected_order_chk'), ('relationship', 'relationship_distinct_ends_chk'),
    ('relationship_evidence', 'relationship_evidence_rel_fk'), ('relationship_evidence', 'relationship_evidence_source_chk'),
    ('ip_assignment', 'ip_assignment_derivation_fk'), ('ip_assignment', 'ip_assignment_ports_chk'), ('ip_assignment', 'ip_assignment_holder_chk'),
    ('derivation', 'derivation_parameters_chk')
  ) AS c(rel, conname)
  WHERE NOT EXISTS (
    SELECT 1 FROM pg_constraint k
    WHERE k.conrelid = ('forensic.' || c.rel)::regclass AND k.conname = c.conname AND k.convalidated
  );
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'missing or unvalidated constraints: %', problem;
  END IF;

  -- No face comparison relationship exists before the case-authority migration.
  IF EXISTS (
    SELECT 1 FROM pg_constraint k
    WHERE k.conrelid = 'forensic.relationship'::regclass AND k.conname = 'relationship_type_kind_chk'
      AND pg_get_constraintdef(k.oid) LIKE '%face%'
  ) THEN
    RAISE EXCEPTION 'a face relationship type is allowed before the case-authority policy exists';
  END IF;

  SELECT string_agg(t.name, ', ')
    INTO problem
  FROM (VALUES ('entity_touch_updated_at', 'entity'), ('relationship_touch_updated_at', 'relationship')) AS t(name, rel)
  WHERE NOT EXISTS (
    SELECT 1 FROM pg_trigger g
    WHERE g.tgrelid = ('forensic.' || t.rel)::regclass AND g.tgname = t.name AND NOT g.tgisinternal
  );
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'missing updated_at triggers: %', problem;
  END IF;

  -- Data checks that hold at every moment (empty tables pass).
  IF EXISTS (SELECT 1 FROM forensic.relationship r
             JOIN forensic.entity s ON s.entity_id = r.src_entity
             JOIN forensic.entity d ON d.entity_id = r.dst_entity
             WHERE s.tenant_id <> r.tenant_id OR d.tenant_id <> r.tenant_id
                OR s.collection_id <> r.collection_id OR d.collection_id <> r.collection_id) THEN
    RAISE EXCEPTION 'relationship rows connect entities of another tenant or collection';
  END IF;
  IF EXISTS (SELECT 1 FROM forensic.relationship r
             WHERE r.evidence_count <> (SELECT count(*) FROM forensic.relationship_evidence e WHERE e.rel_id = r.rel_id)
               AND EXISTS (SELECT 1 FROM forensic.relationship_evidence e WHERE e.rel_id = r.rel_id)) THEN
    RAISE EXCEPTION 'evidence_count disagrees with the stored evidence rows (relationships with materialised evidence only)';
  END IF;

  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    SELECT string_agg(t.name, ', ')
      INTO problem
    FROM (VALUES ('derivation'), ('entity'), ('entity_mention'), ('relationship'), ('relationship_evidence'), ('ip_assignment')) AS t(name)
    WHERE NOT has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'SELECT')
       OR NOT has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'INSERT')
       OR has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'UPDATE')
       OR has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'DELETE')
       OR has_table_privilege('forensic_runtime', 'forensic.' || t.name, 'TRUNCATE');
    -- Column-level UPDATE grants do not count as table-level UPDATE here; they are checked column by column below.
    IF problem IS NOT NULL THEN
      RAISE EXCEPTION 'runtime role privileges are wrong on: %', problem;
    END IF;

    IF has_column_privilege('forensic_runtime', 'forensic.entity', 'canonical', 'UPDATE')
       OR has_column_privilege('forensic_runtime', 'forensic.entity', 'entity_type', 'UPDATE')
       OR has_column_privilege('forensic_runtime', 'forensic.relationship', 'src_entity', 'UPDATE')
       OR has_column_privilege('forensic_runtime', 'forensic.relationship', 'rel_type', 'UPDATE')
       OR has_column_privilege('forensic_runtime', 'forensic.relationship', 'kind', 'UPDATE')
       OR has_column_privilege('forensic_runtime', 'forensic.relationship', 'derivation_id', 'UPDATE') THEN
      RAISE EXCEPTION 'runtime role can update key columns of entity or relationship';
    END IF;
    IF NOT has_column_privilege('forensic_runtime', 'forensic.relationship', 'review_state', 'UPDATE')
       OR NOT has_column_privilege('forensic_runtime', 'forensic.entity', 'display_label', 'UPDATE') THEN
      RAISE EXCEPTION 'runtime role cannot record analyst review or relabel an entity';
    END IF;
  END IF;
END
$$;

SELECT 'migration_014_verified' AS result;

ROLLBACK;
