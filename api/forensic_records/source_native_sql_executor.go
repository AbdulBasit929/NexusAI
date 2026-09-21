package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// sourceNativeSQLLineageLimit bounds how many per-row citations are carried
// per group/result in the response payload. The aggregate/count itself is
// always computed over every authorized row via SQL, never over this bound;
// only the *cited example rows* shown to the analyst are capped, exactly
// like every other bounded_records_table presentation in this system.
const sourceNativeSQLLineageLimit = 50

// executeSourceNativePlanSQL compiles the already-validated SourceNativePlanV1
// (same contract, same validateSourceNativePlan gate used by the in-memory
// executor) into one parameterized SQL statement and runs it against the
// FULL authorized scope in Postgres, instead of the bounded in-memory row
// sample. Every field reference is resolved through the server-derived
// catalog (FieldID -> NormalizedName/SourceNames/EffectiveType); no analyst-
// or model-supplied string is ever concatenated into the SQL text — field
// names and literal values are always passed as bind parameters.
func executeSourceNativePlanSQL(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, plan *SourceNativePlanV1, catalog []FieldDescriptorV1) (map[string]any, error) {
	if err := validateSourceNativePlan(plan, catalog); err != nil {
		return nil, err
	}
	if db == nil {
		return nil, errors.New("source-native execution requires an authorized database scope")
	}
	fields := sourceNativeFieldMap(catalog)
	for _, id := range plan.Project {
		if _, ok := fields[id]; !ok {
			return nil, fmt.Errorf("unknown project field %q", id)
		}
	}
	var args []any
	where, args, err := sourceNativeScopeWhere(req, args)
	if err != nil {
		return nil, err
	}
	where, args = sourceNativeRequestConstraintsSQL(req, where, args)
	for _, filter := range plan.Filters {
		clause, nextArgs, err := sourceNativeFilterSQL(filter, fields, args)
		if err != nil {
			return nil, err
		}
		args = nextArgs
		where = append(where, clause)
	}
	whereSQL := strings.Join(where, " AND ")

	if len(plan.Project) > 0 {
		return executeSourceNativeProjectionSQL(ctx, db, req, plan, fields, whereSQL, args)
	}
	return executeSourceNativeAggregateSQL(ctx, db, req, plan, fields, whereSQL, args)
}

func executeSourceNativeProjectionSQL(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, plan *SourceNativePlanV1, fields map[string]FieldDescriptorV1, whereSQL string, args []any) (map[string]any, error) {
	// The COUNT query references only the WHERE placeholders. Projection and
	// sort expressions append further args below, so the count must be bound
	// to this prefix; passing the full slice fails pgx's argument-count check
	// and surfaced as an HTTP 500 for every dynamic row-listing plan.
	whereArgs := append([]any(nil), args...)
	selects := make([]string, 0, len(plan.Project))
	for _, id := range plan.Project {
		expr := sourceNativeFieldRawExpr(fields[id], &args)
		selects = append(selects, expr+" AS "+sourceNativeSQLAlias(fields[id].FieldID))
	}
	orderSQL, args, err := sourceNativeSortSQL(plan.Sort, fields, nil, args)
	if err != nil {
		return nil, err
	}
	limit := plan.Limit
	if limit == 0 {
		limit = defaultHybridLimit
	}
	args = append(args, limit)
	limitPlaceholder := fmt.Sprintf("$%d", len(args))
	countSQL := `SELECT COUNT(*) FROM forensic.records r WHERE ` + whereSQL
	total, err := scanScalarInt64(ctx, db, req.TenantID, countSQL, whereArgs...)
	if err != nil {
		return nil, err
	}
	sql := `SELECT ` + strings.Join(selects, ",") +
		`,r.record_id::text AS __record_id,r.record_type::text AS __record_type,r.source_file AS __source_file,r.row_number AS __row_number,r.batch_id::text AS __batch_id,r.file_id AS __file_id,r.row_hash AS __row_hash,r.timestamp AS __timestamp,e.evidence_id AS __evidence_id,e.evidence_version_id AS __evidence_version_id` +
		` FROM forensic.records r LEFT JOIN LATERAL (SELECT item.evidence_id::text AS evidence_id,item.current_version_id::text AS evidence_version_id FROM forensic.evidence_items item WHERE item.tenant_id=r.tenant_id AND item.collection_id=r.collection_id AND item.records_batch_id=r.batch_id ORDER BY item.evidence_id LIMIT 1) e ON true` +
		` WHERE ` + whereSQL + orderSQL + ` LIMIT ` + limitPlaceholder
	rows, err := queryRows(ctx, db, req.TenantID, sql, args...)
	if err != nil {
		return nil, err
	}
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out := map[string]any{}
		for _, id := range plan.Project {
			out[fields[id].NormalizedName] = row[sourceNativeSQLAlias(fields[id].FieldID)]
		}
		out["metadata"] = map[string]any{"source_rows": []map[string]any{{
			"source": "records_sql", "source_table": "forensic.records", "collection_id": req.CollectionID,
			"record_id": row["__record_id"], "record_type": row["__record_type"], "source_file": row["__source_file"],
			"row_number": row["__row_number"], "batch_id": row["__batch_id"], "file_id": row["__file_id"],
			"row_hash": row["__row_hash"], "timestamp": row["__timestamp"],
			"evidence_id": row["__evidence_id"], "version_id": row["__evidence_version_id"],
		}}}
		results = append(results, out)
	}
	return map[string]any{
		"contract_version": sourceNativeResultV1, "source_native_results": results,
		"row_count": len(results), "total_result_count": int(total), "total_count": int(total),
		"scanned_source_rows": int(total), "limit": limit, "complete": true,
		"field_catalog_contract": fieldDescriptorContractV1, "field_catalog": catalogForFields(fields, plan),
		"plan": plan, "input_row_limit": sourceNativeInputRowLimit, "group_limit": sourceNativeGroupLimit,
		"provenance": sourceNativeProjectionProvenance(results),
	}, nil
}

func executeSourceNativeAggregateSQL(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, plan *SourceNativePlanV1, fields map[string]FieldDescriptorV1, whereSQL string, args []any) (map[string]any, error) {
	groupExprs := make([]string, 0, len(plan.GroupFields))
	groupAliases := make([]string, 0, len(plan.GroupFields))
	selects := []string{}
	for _, id := range plan.GroupFields {
		field := fields[id]
		expr := sourceNativeFieldRawExpr(field, &args)
		alias := sourceNativeSQLAlias(field.FieldID)
		groupExprs = append(groupExprs, expr)
		groupAliases = append(groupAliases, alias)
		selects = append(selects, expr+" AS "+alias)
	}
	var bucketAlias, bucketEndAlias string
	if plan.TimeBucket != nil {
		field := fields[plan.TimeBucket.FieldID]
		expr := sourceNativeFieldTypedExpr(field, &args)
		tz := plan.TimeBucket.Timezone
		if tz == "" {
			tz = "UTC"
		}
		args = append(args, plan.TimeBucket.Bucket)
		unitPlaceholder := fmt.Sprintf("$%d", len(args))
		args = append(args, tz)
		tzPlaceholder := fmt.Sprintf("$%d", len(args))
		localExpr := "(" + expr + ") AT TIME ZONE " + tzPlaceholder
		bucketExpr := "date_trunc(" + unitPlaceholder + "::text, " + localExpr + ")"
		bucketAlias = sourceNativeSQLAlias(field.FieldID) + "_bucket"
		bucketEndAlias = bucketAlias + "_end"
		groupExprs = append(groupExprs, bucketExpr)
		selects = append(selects, bucketExpr+" AS "+bucketAlias)
		selects = append(selects, sourceNativeBucketEndSQL(bucketExpr, plan.TimeBucket.Bucket)+" AS "+bucketEndAlias)
	}
	measureAliases := make(map[string]string, len(plan.Measures))
	for _, measure := range plan.Measures {
		alias := "measure_" + sourceNativeSQLIdentSafe(measure.MeasureID)
		measureAliases[measure.MeasureID] = alias
		sqlExpr, nextArgs, err := sourceNativeMeasureSQL(measure, fields, args)
		if err != nil {
			return nil, err
		}
		args = nextArgs
		selects = append(selects, sqlExpr+" AS "+alias)
	}
	selects = append(selects, "COUNT(*) AS __contributing_row_count")
	selects = append(selects, sourceNativeLineageAggExpr()+" AS __lineage")

	havingClauses := make([]string, 0, len(plan.Having))
	for _, having := range plan.Having {
		alias, ok := measureAliases[having.MeasureID]
		if !ok {
			return nil, fmt.Errorf("having references unknown measure %q", having.MeasureID)
		}
		clause, nextArgs, err := sourceNativeHavingSQL(alias, having, args)
		if err != nil {
			return nil, err
		}
		args = nextArgs
		havingClauses = append(havingClauses, clause)
	}

	sql := `SELECT ` + strings.Join(selects, ",") + ` FROM forensic.records r` +
		` LEFT JOIN LATERAL (SELECT item.evidence_id::text AS evidence_id,item.current_version_id::text AS evidence_version_id FROM forensic.evidence_items item WHERE item.tenant_id=r.tenant_id AND item.collection_id=r.collection_id AND item.records_batch_id=r.batch_id ORDER BY item.evidence_id LIMIT 1) e ON true` +
		` WHERE ` + whereSQL
	if len(groupExprs) > 0 {
		sql += " GROUP BY " + strings.Join(groupExprs, ",")
	}
	if len(havingClauses) > 0 {
		sql += " HAVING " + strings.Join(havingClauses, " AND ")
	}
	orderSQL, args, err := sourceNativeSortSQL(plan.Sort, fields, measureAliases, args)
	if err != nil {
		return nil, err
	}
	sql += orderSQL

	groupLimitArg := sourceNativeGroupLimit + 1
	if len(plan.GroupFields) == 0 && plan.TimeBucket == nil {
		groupLimitArg = 1
	}
	args = append(args, groupLimitArg)
	sql += fmt.Sprintf(" LIMIT $%d", len(args))

	rows, err := queryRows(ctx, db, req.TenantID, sql, args...)
	if err != nil {
		return nil, err
	}
	if len(plan.GroupFields) > 0 || plan.TimeBucket != nil {
		if len(rows) > sourceNativeGroupLimit {
			return nil, fmt.Errorf("source-native group cardinality exceeds %d", sourceNativeGroupLimit)
		}
	}
	limit := plan.Limit
	if limit == 0 {
		limit = defaultHybridLimit
	}
	results := make([]map[string]any, 0, len(rows))
	scannedTotal := int64(0)
	for _, row := range rows {
		out := map[string]any{}
		for i, id := range plan.GroupFields {
			out[fields[id].NormalizedName] = row[groupAliases[i]]
		}
		if plan.TimeBucket != nil {
			field := fields[plan.TimeBucket.FieldID]
			out[field.NormalizedName] = row[bucketAlias]
			out[field.NormalizedName+"_bucket_end_exclusive"] = row[bucketEndAlias]
			out["timezone"] = plan.TimeBucket.Timezone
		}
		for _, measure := range plan.Measures {
			out[measure.MeasureID] = row[measureAliases[measure.MeasureID]]
		}
		contributing := int64FromAny(row["__contributing_row_count"])
		scannedTotal += contributing
		lineage := decodeSourceNativeLineage(row["__lineage"], req.CollectionID)
		out["metadata"] = map[string]any{"source_rows": lineage, "contributing_row_count": int(contributing)}
		results = append(results, out)
	}
	totalResults := len(results)
	if totalResults > limit {
		results = results[:limit]
	}
	return map[string]any{
		"contract_version": sourceNativeResultV1, "source_native_results": results,
		"row_count": len(results), "total_result_count": totalResults, "total_count": len(rows),
		"scanned_source_rows": int(scannedTotal), "limit": limit, "complete": true,
		"field_catalog_contract": fieldDescriptorContractV1, "field_catalog": catalogForFields(fields, plan),
		"plan": plan, "input_row_limit": sourceNativeInputRowLimit, "group_limit": sourceNativeGroupLimit,
		"provenance": aggregateProvenance(results),
	}, nil
}

// sourceNativeFieldRawExpr extracts a field's value as text from whichever
// storage location the catalog observed it in. Special top-level columns are
// referenced directly (already correctly typed by Postgres); every other
// field is a COALESCE across raw_payload and metadata.normalized_fields,
// tried under every source-name alias the catalog observed for it. Every
// name is bound as a parameter, never inlined as a literal.
func sourceNativeFieldRawExpr(field FieldDescriptorV1, args *[]any) string {
	switch field.NormalizedName {
	case "record_type":
		return "r.record_type::text"
	case "source_file":
		return "r.source_file"
	case "timestamp":
		return "r.timestamp::text"
	case "row_number":
		return "r.row_number::text"
	}
	names := field.SourceNames
	if len(names) == 0 {
		names = []string{field.NormalizedName}
	}
	parts := make([]string, 0, len(names)*2)
	for _, name := range names {
		*args = append(*args, name)
		parts = append(parts, fmt.Sprintf("r.raw_payload->>$%d", len(*args)))
	}
	for _, name := range names {
		*args = append(*args, name)
		parts = append(parts, fmt.Sprintf("r.metadata->'normalized_fields'->>$%d", len(*args)))
	}
	return "COALESCE(" + strings.Join(parts, ",") + ")"
}

// sourceNativeFieldTypedExpr wraps the raw text extraction in a guarded cast
// matching the catalog's observed EffectiveType, so comparisons/aggregates
// use real numeric/timestamp semantics rather than text ordering. A value
// that fails the guard becomes NULL rather than erroring the whole query,
// mirroring the in-memory executor's "unparseable value does not match"
// behavior.
func sourceNativeFieldTypedExpr(field FieldDescriptorV1, args *[]any) string {
	if field.NormalizedName == "row_number" {
		return "r.row_number"
	}
	if field.NormalizedName == "timestamp" {
		return "r.timestamp"
	}
	raw := sourceNativeFieldRawExpr(field, args)
	switch field.EffectiveType {
	case fieldTypeInteger, fieldTypeDecimal:
		return "(CASE WHEN (" + raw + `) ~ '^-?[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?$' THEN (` + raw + ")::numeric ELSE NULL END)"
	case fieldTypeTimestamp:
		return "(CASE WHEN (" + raw + `) ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}' THEN NULLIF(` + raw + ",'')::timestamptz ELSE NULL END)"
	case fieldTypeDate:
		return "(CASE WHEN (" + raw + `) ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}' THEN NULLIF(` + raw + ",'')::date ELSE NULL END)"
	case fieldTypeBoolean:
		return "(CASE WHEN lower(" + raw + ") IN ('true','false') THEN lower(" + raw + ")::boolean ELSE NULL END)"
	default:
		return raw
	}
}

func sourceNativeCastLiteral(field FieldDescriptorV1, placeholder string) string {
	switch field.EffectiveType {
	case fieldTypeInteger, fieldTypeDecimal:
		return placeholder + "::numeric"
	case fieldTypeTimestamp:
		return placeholder + "::timestamptz"
	case fieldTypeDate:
		return placeholder + "::date"
	case fieldTypeBoolean:
		return placeholder + "::boolean"
	default:
		return placeholder
	}
}

func sourceNativeFilterSQL(filter SourceNativeFilterV1, fields map[string]FieldDescriptorV1, args []any) (string, []any, error) {
	field, ok := fields[filter.FieldID]
	if !ok {
		return "", args, fmt.Errorf("unknown filter field %q", filter.FieldID)
	}
	expr := sourceNativeFieldTypedExpr(field, &args)
	op := strings.ToUpper(filter.Op)
	switch op {
	case "IS_NULL":
		return "(" + expr + ") IS NULL", args, nil
	case "IS_NOT_NULL":
		return "(" + expr + ") IS NOT NULL", args, nil
	case "EQ", "NEQ", "GT", "GTE", "LT", "LTE":
		symbol := map[string]string{"EQ": "=", "NEQ": "<>", "GT": ">", "GTE": ">=", "LT": "<", "LTE": "<="}[op]
		args = append(args, filter.Value)
		return "(" + expr + ") " + symbol + " " + sourceNativeCastLiteral(field, fmt.Sprintf("$%d", len(args))), args, nil
	case "BETWEEN":
		if len(filter.Values) != 2 {
			return "", args, errors.New("BETWEEN requires exactly two values")
		}
		args = append(args, filter.Values[0])
		lo := sourceNativeCastLiteral(field, fmt.Sprintf("$%d", len(args)))
		args = append(args, filter.Values[1])
		hi := sourceNativeCastLiteral(field, fmt.Sprintf("$%d", len(args)))
		return "(" + expr + ") BETWEEN " + lo + " AND " + hi, args, nil
	case "IN":
		if len(filter.Values) == 0 {
			return "FALSE", args, nil
		}
		placeholders := make([]string, len(filter.Values))
		for i, v := range filter.Values {
			args = append(args, v)
			placeholders[i] = sourceNativeCastLiteral(field, fmt.Sprintf("$%d", len(args)))
		}
		return "(" + expr + ") IN (" + strings.Join(placeholders, ",") + ")", args, nil
	case "CONTAINS":
		args = append(args, "%"+filter.Value+"%")
		return "(" + expr + ") ILIKE $" + strconv.Itoa(len(args)), args, nil
	}
	return "", args, fmt.Errorf("unsupported filter operator %q", filter.Op)
}

func sourceNativeHavingSQL(alias string, having SourceNativeHavingV1, args []any) (string, []any, error) {
	op := strings.ToUpper(having.Op)
	symbol, ok := map[string]string{"EQ": "=", "NEQ": "<>", "GT": ">", "GTE": ">=", "LT": "<", "LTE": "<="}[op]
	if !ok {
		return "", args, fmt.Errorf("unsupported having operator %q", having.Op)
	}
	args = append(args, having.Value)
	return alias + " " + symbol + " $" + strconv.Itoa(len(args)) + "::numeric", args, nil
}

func sourceNativeMeasureSQL(measure SourceNativeMeasureV1, fields map[string]FieldDescriptorV1, args []any) (string, []any, error) {
	op := strings.ToUpper(measure.Op)
	if op == "COUNT" && measure.FieldID == "" {
		return "COUNT(*)", args, nil
	}
	field, ok := fields[measure.FieldID]
	if !ok {
		return "", args, fmt.Errorf("unknown measure field %q", measure.FieldID)
	}
	expr := sourceNativeFieldTypedExpr(field, &args)
	switch op {
	case "COUNT":
		return "COUNT(" + expr + ")", args, nil
	case "SUM":
		return "SUM(" + expr + ")", args, nil
	case "AVG":
		return "AVG(" + expr + ")", args, nil
	case "MIN":
		return "MIN(" + expr + ")", args, nil
	case "MAX":
		return "MAX(" + expr + ")", args, nil
	}
	return "", args, fmt.Errorf("unsupported measure operator %q", measure.Op)
}

func sourceNativeSortSQL(sortSpecs []SourceNativeSortV1, fields map[string]FieldDescriptorV1, measureAliases map[string]string, args []any) (string, []any, error) {
	if len(sortSpecs) == 0 {
		return "", args, nil
	}
	parts := make([]string, 0, len(sortSpecs))
	for _, spec := range sortSpecs {
		direction := "ASC"
		if strings.EqualFold(spec.Direction, "DESC") {
			direction = "DESC"
		}
		if measureAliases != nil {
			if alias, ok := measureAliases[spec.Target]; ok {
				parts = append(parts, alias+" "+direction+" NULLS LAST")
				continue
			}
		}
		field, ok := fields[spec.Target]
		if !ok {
			return "", args, fmt.Errorf("unknown sort target %q", spec.Target)
		}
		expr := sourceNativeFieldTypedExpr(field, &args)
		parts = append(parts, expr+" "+direction+" NULLS LAST")
	}
	return " ORDER BY " + strings.Join(parts, ","), args, nil
}

func sourceNativeBucketEndSQL(bucketExpr, unit string) string {
	interval := "1 day"
	switch unit {
	case "hour":
		interval = "1 hour"
	case "week":
		interval = "1 week"
	case "month":
		interval = "1 month"
	}
	return bucketExpr + " + interval '" + interval + "'"
}

func sourceNativeLineageAggExpr() string {
	object := "jsonb_build_object(" +
		"'source','records_sql','source_table','forensic.records'," +
		"'record_id',r.record_id::text,'record_type',r.record_type::text,'source_file',r.source_file," +
		"'row_number',r.row_number,'batch_id',r.batch_id::text,'file_id',r.file_id,'row_hash',r.row_hash," +
		"'timestamp',r.timestamp,'evidence_id',e.evidence_id,'version_id',e.evidence_version_id" +
		")"
	// array_agg (not jsonb_agg) so the bounded [1:N] slice operator applies,
	// then to_jsonb converts the sliced native array back to one JSON array
	// value the caller decodes with json.Unmarshal.
	return "to_jsonb((array_agg(" + object + " ORDER BY r.record_id))[1:" + strconv.Itoa(sourceNativeSQLLineageLimit) + "])"
}

// decodeSourceNativeLineage accepts whatever shape pgx decoded the jsonb
// array column into. pgx's default codec decodes jsonb directly to Go
// map[string]any/[]any (see sourceNativeMap above for the same pattern with
// raw_payload/metadata), but raw bytes/string are also handled defensively
// in case of a differently configured connection.
func decodeSourceNativeLineage(value any, collectionID string) []map[string]any {
	var elements []any
	switch typed := value.(type) {
	case []any:
		elements = typed
	case []byte:
		_ = json.Unmarshal(typed, &elements)
	case string:
		_ = json.Unmarshal([]byte(typed), &elements)
	default:
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(elements))
	for _, element := range elements {
		item, ok := element.(map[string]any)
		if !ok {
			continue
		}
		item["collection_id"] = collectionID
		out = append(out, item)
	}
	return out
}

func sourceNativeSQLAlias(fieldID string) string {
	return "f_" + sourceNativeSQLIdentSafe(fieldID)
}

func sourceNativeSQLIdentSafe(id string) string {
	var b strings.Builder
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "f"
	}
	return b.String()
}

func catalogForFields(fields map[string]FieldDescriptorV1, plan *SourceNativePlanV1) []FieldDescriptorV1 {
	seen := map[string]bool{}
	out := make([]FieldDescriptorV1, 0, len(fields))
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		if field, ok := fields[id]; ok {
			seen[id] = true
			out = append(out, field)
		}
	}
	for _, id := range plan.Project {
		add(id)
	}
	for _, id := range plan.GroupFields {
		add(id)
	}
	for _, m := range plan.Measures {
		add(m.FieldID)
	}
	for _, f := range plan.Filters {
		add(f.FieldID)
	}
	if plan.TimeBucket != nil {
		add(plan.TimeBucket.FieldID)
	}
	return out
}

func sourceNativeProjectionProvenance(results []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, row := range results {
		metadata, _ := row["metadata"].(map[string]any)
		sourceRows, _ := metadata["source_rows"].([]map[string]any)
		out = append(out, sourceRows...)
	}
	return out
}

func aggregateProvenance(results []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, row := range results {
		metadata, _ := row["metadata"].(map[string]any)
		sourceRows, _ := metadata["source_rows"].([]map[string]any)
		out = append(out, sourceRows...)
	}
	return out
}

func scanScalarInt64(ctx context.Context, db *pgxpool.Pool, tenantID, sql string, args ...any) (int64, error) {
	rows, err := queryRows(ctx, db, tenantID, sql, args...)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	for _, value := range rows[0] {
		return int64FromAny(value), nil
	}
	return 0, nil
}

// sourceNativeRequestConstraintsSQL applies the analyst's extracted target
// identifier and date bounds to dynamic execution. They were previously
// extracted but never reached the SQL, so "how many calls did X make" and
// even a nonexistent number returned counts over the whole case. The target
// matches either party column exactly or by digits (>= 8), so formatting
// differences such as spaces or a leading '+' do not cause false negatives.
// Field discovery deliberately does not use these constraints: the catalog
// must describe the scope's fields even when the constrained result is empty.
func sourceNativeRequestConstraintsSQL(req hybridQueryRequest, where []string, args []any) ([]string, []any) {
	add := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if target := strings.TrimSpace(req.Target); target != "" {
		exact := add(target)
		clause := "(r.primary_target=" + exact + " OR r.secondary_target=" + exact
		if digits := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, target); len(digits) >= 8 {
			d := add(digits)
			clause += ` OR regexp_replace(coalesce(r.primary_target,''),'\D','','g')=` + d +
				` OR regexp_replace(coalesce(r.secondary_target,''),'\D','','g')=` + d
		}
		where = append(where, clause+")")
	}
	if from := strings.TrimSpace(req.DateFrom); from != "" {
		where = append(where, "r.timestamp>="+add(from)+"::timestamptz")
	}
	if to := strings.TrimSpace(req.DateTo); to != "" {
		where = append(where, "r.timestamp<"+add(to)+"::timestamptz")
	}
	return where, args
}
