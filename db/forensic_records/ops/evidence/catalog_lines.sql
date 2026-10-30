-- One line per catalog object of the schemas forensic and analyst: tables, columns, constraints, indexes, policies, views (hash of the definition), functions (hash of the definition), grants, comments.
-- Read-only. Used before and after to prove that a migration added lines and changed or removed none.
SELECT line FROM (
  SELECT 'table|' || n.nspname || '.' || c.relname || '|' || c.relkind::text || '|rls=' || c.relrowsecurity::text || '|owner=' || pg_get_userbyid(c.relowner) AS line
    FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname IN ('forensic', 'analyst') AND c.relkind IN ('r', 'p', 'v', 'm', 'f')
  UNION ALL
  SELECT 'column|' || n.nspname || '.' || c.relname || '|' || a.attnum || '|' || a.attname || '|' || format_type(a.atttypid, a.atttypmod) || '|nn=' || a.attnotnull::text || '|' || coalesce(pg_get_expr(d.adbin, d.adrelid), '')
    FROM pg_attribute a JOIN pg_class c ON c.oid = a.attrelid JOIN pg_namespace n ON n.oid = c.relnamespace LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
    WHERE n.nspname IN ('forensic', 'analyst') AND c.relkind IN ('r', 'p', 'v', 'm', 'f') AND a.attnum > 0 AND NOT a.attisdropped
  UNION ALL
  SELECT 'constraint|' || n.nspname || '.' || c.relname || '|' || k.conname || '|' || pg_get_constraintdef(k.oid)
    FROM pg_constraint k JOIN pg_class c ON c.oid = k.conrelid JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname IN ('forensic', 'analyst')
  UNION ALL
  SELECT 'index|' || schemaname || '.' || tablename || '|' || indexname || '|' || indexdef FROM pg_indexes WHERE schemaname IN ('forensic', 'analyst')
  UNION ALL
  SELECT 'policy|' || schemaname || '.' || tablename || '|' || policyname || '|' || cmd || '|' || coalesce(qual, '') || '|' || coalesce(with_check, '') FROM pg_policies WHERE schemaname IN ('forensic', 'analyst')
  UNION ALL
  SELECT 'view|' || schemaname || '.' || viewname || '|' || md5(definition) FROM pg_views WHERE schemaname IN ('forensic', 'analyst')
  UNION ALL
  SELECT 'function|' || n.nspname || '.' || p.proname || '(' || pg_get_function_identity_arguments(p.oid) || ')|' || md5(pg_get_functiondef(p.oid))
    FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname IN ('forensic', 'analyst') AND p.prokind IN ('f', 'p')
  UNION ALL
  SELECT 'grant|' || n.nspname || '.' || c.relname || '|' || coalesce(c.relacl::text, '')
    FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname IN ('forensic', 'analyst') AND c.relkind IN ('r', 'p', 'v', 'm', 'f')
  UNION ALL
  SELECT 'comment|' || n.nspname || '.' || c.relname || '|' || coalesce(d.objsubid::text, '0') || '|' || md5(d.description)
    FROM pg_description d JOIN pg_class c ON c.oid = d.objoid AND d.classoid = 'pg_class'::regclass JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname IN ('forensic', 'analyst')
) x ORDER BY line;
