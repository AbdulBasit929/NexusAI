-- Exact row count of every base table of the schemas forensic and public (as a superuser, so row-level security does not hide rows). Read-only.
SELECT table_schema || '.' || table_name || '|' || (xpath('/row/c/text()', query_to_xml(format('select count(*) as c from %I.%I', table_schema, table_name), false, true, '')))[1]::text
FROM information_schema.tables WHERE table_schema IN ('forensic', 'public') AND table_type = 'BASE TABLE' ORDER BY 1;
