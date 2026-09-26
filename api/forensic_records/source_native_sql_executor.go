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
	// WHERE THIS PLAN'S ROWS LIVE, resolved once from the curated layer. A plan
	// that mixes an ingested record with a model observation is REFUSED here
	// rather than answered from one table or silently joined: either would
	// present evidence the analyst did not ask about, or assert a relationship
	// the data does not establish.
	binding, err := sourceNativeBindingForPlan(req.Query, plan, catalog)
	if err != nil {
		return nil, err
	}
	if binding.Derived && !derivedArtifactExecutionEnabled() {
		return nil, fmt.Errorf("reading %s is not enabled", binding.Table)
	}

	var args []any
	var where []string
	if binding.Derived {
		// A DERIVED PLAN MAY NOT PROJECT ROWS YET. forensic.derived_artifacts has
		// no record_id, row_number, batch_id or row_hash, and the projection path
		// cites every one of them. Listing observations without their provenance
		// would hand the analyst rows it cannot attribute, so it is refused until
		// derived provenance is mapped -- an explicit refusal, not a silent
		// omission of the citation columns.
		if len(plan.Project) > 0 {
			return nil, fmt.Errorf("listing individual %s rows is not supported yet; ask for a count, a total or a breakdown", binding.Table)
		}
		where, args, err = sourceNativeDerivedScopeWhere(req, args)
		if err != nil {
			return nil, err
		}
		// THE CONTRACT FILTER. Without it the entity matches every artifact
		// contract in the table -- an OCR entity counting face detections -- and
		// the count looks entirely authoritative.
		args = append(args, binding.ArtifactType)
		where = append(where, fmt.Sprintf("d.artifact_type=$%d", len(args)))
		// THE DISCRIMINATOR, when the contract carries more than one kind of row.
		// forensics.image-observation/v1 holds 20 technical observations and 6
		// sampled video frames whose fields differ; without this the entity would
		// aggregate two populations and describe fields most of its rows lack.
		if binding.ObservationType != "" {
			args = append(args, binding.ObservationType)
			where = append(where, fmt.Sprintf("d.metadata->>'observation_type'=$%d", len(args)))
		}
		// sourceNativeRequestConstraintsSQL is deliberately NOT applied: it
		// constrains on r.primary_target and the records timestamp column, and a
		// derived artifact has neither. Applying a target here is exactly the H2
		// defect -- a constraint the row cannot satisfy, matching nothing, and
		// reporting a false absence over evidence that is present.
	} else {
		where, args, err = sourceNativeScopeWhere(req, args)
		if err != nil {
			return nil, err
		}
		where, args = sourceNativeRequestConstraintsSQL(req, where, args)
	}
	for _, filter := range plan.Filters {
		clause, nextArgs, err := sourceNativeFilterSQL(binding, filter, fields, args)
		if err != nil {
			return nil, err
		}
		args = nextArgs
		where = append(where, clause)
	}
	whereSQL := strings.Join(where, " AND ")

	if len(plan.Project) > 0 {
		return executeSourceNativeProjectionSQL(ctx, db, req, binding, plan, fields, whereSQL, args)
	}
	return executeSourceNativeAggregateSQL(ctx, db, req, binding, plan, fields, whereSQL, args)
}

// Records only: a derived plan carrying Project is refused before it reaches
// here, because derived artifacts have none of the citation columns this
// function selects.
func executeSourceNativeProjectionSQL(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, binding sourceNativeBinding, plan *SourceNativePlanV1, fields map[string]FieldDescriptorV1, whereSQL string, args []any) (map[string]any, error) {
	// The COUNT query references only the WHERE placeholders. Projection and
	// sort expressions append further args below, so the count must be bound
	// to this prefix; passing the full slice fails pgx's argument-count check
	// and surfaced as an HTTP 500 for every dynamic row-listing plan.
	whereArgs := append([]any(nil), args...)
	selects := make([]string, 0, len(plan.Project))
	for _, id := range plan.Project {
		expr := sourceNativeFieldRawExpr(binding, fields[id], &args)
		selects = append(selects, expr+" AS "+sourceNativeSQLAlias(fields[id].FieldID))
	}
	orderSQL, args, err := sourceNativeSortSQL(binding, plan.Sort, fields, nil, args)
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

func executeSourceNativeAggregateSQL(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, binding sourceNativeBinding, plan *SourceNativePlanV1, fields map[string]FieldDescriptorV1, whereSQL string, args []any) (map[string]any, error) {
	groupExprs := make([]string, 0, len(plan.GroupFields))
	groupAliases := make([]string, 0, len(plan.GroupFields))
	selects := []string{}
	for _, id := range plan.GroupFields {
		field := fields[id]
		expr := sourceNativeFieldRawExpr(binding, field, &args)
		alias := sourceNativeSQLAlias(field.FieldID)
		groupExprs = append(groupExprs, expr)
		groupAliases = append(groupAliases, alias)
		selects = append(selects, expr+" AS "+alias)
	}
	var bucketAlias, bucketEndAlias string
	if plan.TimeBucket != nil {
		field := fields[plan.TimeBucket.FieldID]
		expr := sourceNativeFieldTypedExpr(binding, field, &args)
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
	denominatorAliases := map[string]string{}
	for _, measure := range plan.Measures {
		alias := "measure_" + sourceNativeSQLIdentSafe(measure.MeasureID)
		measureAliases[measure.MeasureID] = alias
		sqlExpr, nextArgs, err := sourceNativeMeasureSQL(binding, measure, fields, args)
		if err != nil {
			return nil, err
		}
		args = nextArgs
		selects = append(selects, sqlExpr+" AS "+alias)
		// HOW MANY ROWS ACTUALLY CARRIED A VALUE. An AVG over a field present on
		// 263 of 309 rows reads as a statement about all 309; this is what lets
		// the answer say otherwise. Selected only when the switch is on, so the
		// SQL is otherwise byte-identical.
		if measureDenominatorEnabled() && measureNeedsDenominator(measure) {
			denominatorAlias := alias + "_valuecount"
			denominatorAliases[measure.MeasureID] = denominatorAlias
			field, ok := fields[measure.FieldID]
			if !ok {
				return nil, fmt.Errorf("unknown measure field %q", measure.FieldID)
			}
			if field.DerivedOp == "" {
				selects = append(selects, "COUNT("+sourceNativeFieldTypedExpr(binding, field, &args)+") AS "+denominatorAlias)
			} else {
				// A computed metric has no column of its own to count. Its
				// coverage is the coverage of the fields it names, which this
				// slice does not model, so it is left unqualified rather than
				// qualified wrongly.
				delete(denominatorAliases, measure.MeasureID)
			}
		}
	}
	selects = append(selects, "COUNT(*) AS __contributing_row_count")
	selects = append(selects, sourceNativeLineageAggExpr(binding)+" AS __lineage")

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

	// A derived artifact already carries its own evidence_id and version_id, so
	// it needs no lateral join to reach them -- and it could not use that join
	// anyway, since it has no records_batch_id to match on.
	from := ` FROM forensic.records r` +
		` LEFT JOIN LATERAL (SELECT item.evidence_id::text AS evidence_id,item.current_version_id::text AS evidence_version_id FROM forensic.evidence_items item WHERE item.tenant_id=r.tenant_id AND item.collection_id=r.collection_id AND item.records_batch_id=r.batch_id ORDER BY item.evidence_id LIMIT 1) e ON true`
	if binding.Derived {
		from = ` FROM ` + binding.Table + ` d`
	}
	sql := `SELECT ` + strings.Join(selects, ",") + from +
		` WHERE ` + whereSQL
	if len(groupExprs) > 0 {
		sql += " GROUP BY " + strings.Join(groupExprs, ",")
	}
	if len(havingClauses) > 0 {
		sql += " HAVING " + strings.Join(havingClauses, " AND ")
	}
	orderSQL, args, err := sourceNativeSortSQL(binding, plan.Sort, fields, measureAliases, args)
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
			if alias, ok := denominatorAliases[measure.MeasureID]; ok {
				out[measureDenominatorKey(measure.MeasureID)] = int(int64FromAny(row[alias]))
			}
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
// A DERIVED entity reads a different table, a different payload column, and
// NESTED paths, so its extraction is built by sourceNativePayloadTextExpr. It
// has no `normalized_fields` mirror and none of the records top-level columns:
// forensic.derived_artifacts has no record_type, source_file, row_number or
// timestamp, so referencing them would be a SQL error rather than a wrong
// answer. The switch below therefore applies to records only.
func sourceNativeFieldRawExpr(binding sourceNativeBinding, field FieldDescriptorV1, args *[]any) string {
	names := field.SourceNames
	if len(names) == 0 {
		names = []string{field.NormalizedName}
	}
	if binding.Derived {
		parts := make([]string, 0, len(names))
		for _, name := range names {
			parts = append(parts, sourceNativePayloadTextExpr(binding, "d", name, args))
		}
		if len(parts) == 1 {
			return parts[0]
		}
		return "COALESCE(" + strings.Join(parts, ",") + ")"
	}
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
func sourceNativeFieldTypedExpr(binding sourceNativeBinding, field FieldDescriptorV1, args *[]any) string {
	if !binding.Derived {
		if field.NormalizedName == "row_number" {
			return "r.row_number"
		}
		if field.NormalizedName == "timestamp" {
			return "r.timestamp"
		}
	}
	raw := sourceNativeFieldRawExpr(binding, field, args)
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

func sourceNativeFilterSQL(binding sourceNativeBinding, filter SourceNativeFilterV1, fields map[string]FieldDescriptorV1, args []any) (string, []any, error) {
	field, ok := fields[filter.FieldID]
	if !ok {
		return "", args, fmt.Errorf("unknown filter field %q", filter.FieldID)
	}
	expr := sourceNativeFieldTypedExpr(binding, field, &args)
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

// sourceNativeDerivedExpr builds the SQL for a curated computed metric. Only
// the allowlisted vocabulary is resolvable, and both operands must be issued
// fields, so a metric can never widen what the plan may touch.
func sourceNativeDerivedExpr(binding sourceNativeBinding, field FieldDescriptorV1, fields map[string]FieldDescriptorV1, args *[]any) (string, error) {
	if field.DerivedOp != "timestamp_diff_seconds" || len(field.DerivedFields) != 2 {
		return "", fmt.Errorf("unsupported derived metric %q", field.FieldID)
	}
	operands := make([]string, 0, 2)
	for _, id := range field.DerivedFields {
		source, ok := fields[id]
		if !ok {
			return "", fmt.Errorf("derived metric %q needs field %q", field.FieldID, id)
		}
		operands = append(operands, sourceNativeFieldTypedExpr(binding, source, args))
	}
	// timestamp_diff_seconds(start, end) is elapsed seconds, so end - start.
	return "EXTRACT(EPOCH FROM ((" + operands[1] + ") - (" + operands[0] + ")))", nil
}

func sourceNativeMeasureSQL(binding sourceNativeBinding, measure SourceNativeMeasureV1, fields map[string]FieldDescriptorV1, args []any) (string, []any, error) {
	op := strings.ToUpper(measure.Op)
	if op == "COUNT" && measure.FieldID == "" {
		return "COUNT(*)", args, nil
	}
	field, ok := fields[measure.FieldID]
	if !ok {
		return "", args, fmt.Errorf("unknown measure field %q", measure.FieldID)
	}
	// Build ONE expression. Building the typed one first and discarding it for a
	// derived metric left its placeholders in args with nothing referencing them,
	// and Postgres refused the whole query: "could not determine data type of
	// parameter $4". A derived pseudo-field has no column of its own to extract,
	// so the typed path must not run for it at all.
	var expr string
	if field.DerivedOp != "" {
		// A computed metric: resolve the allowlisted expression server-side from
		// the fields it names. The plan only ever carries the metric's ID, so no
		// expression text crosses the model boundary.
		derived, err := sourceNativeDerivedExpr(binding, field, fields, &args)
		if err != nil {
			return "", args, err
		}
		expr = derived
	} else {
		expr = sourceNativeFieldTypedExpr(binding, field, &args)
	}
	switch op {
	case "COUNT":
		return "COUNT(" + expr + ")", args, nil
	case "COUNT_DISTINCT":
		// "How many DIFFERENT people did X contact" is a distinct count, not a
		// row count. The curated layer has declared it since WI-4
		// (cdr.distinct_counterparties, distinct_capable) but the typed plan
		// could not express it, so CDR-07/08 and ANPR-05 were unanswerable at
		// any model quality.
		return "COUNT(DISTINCT " + expr + ")", args, nil
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

func sourceNativeSortSQL(binding sourceNativeBinding, sortSpecs []SourceNativeSortV1, fields map[string]FieldDescriptorV1, measureAliases map[string]string, args []any) (string, []any, error) {
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
		expr := sourceNativeFieldTypedExpr(binding, field, &args)
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

func sourceNativeLineageAggExpr(binding sourceNativeBinding) string {
	if binding.Derived {
		// A DERIVED CITATION NAMES WHAT PRODUCED IT. A model observation is
		// evidence about a pixel crop or an audio window, not a row of ingested
		// source data, so its citation carries the artifact and the evidence it
		// was derived FROM -- plus `source_truth_state`, which is the field that
		// stops a model's plate read being read as a camera's sighting.
		object := "jsonb_build_object(" +
			"'source','derived_artifacts_sql','source_table','forensic.derived_artifacts'," +
			"'artifact_id',d.artifact_id::text,'artifact_type',d.artifact_type," +
			"'evidence_id',d.evidence_id::text,'version_id',d.version_id::text," +
			"'run_id',d.run_id::text,'content_sha256',d.content_sha256," +
			"'confidence',d.confidence,'citation_locator',d.citation_locator," +
			"'source_truth_state',d.metadata->>'source_truth_state'," +
			"'timestamp',d.created_at" +
			")"
		return "to_jsonb((array_agg(" + object + " ORDER BY d.artifact_id))[1:" + strconv.Itoa(sourceNativeSQLLineageLimit) + "])"
	}
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
	// A target is applied here as the record's SUBJECT -- `primary_target` and
	// `secondary_target` hold the MSISDN, plate or account the row is ABOUT.
	// Applying a target that is not a subject silently matches nothing.
	//
	// Measured 2026-09-25: "How many VoLTE calls are there?" produced the
	// CORRECT plan (`cdr.call_type EQ "VOLTE"`, a value the layer declares) and
	// then this function added `(primary_target='VOLTE' OR
	// secondary_target='VOLTE')`, so the query matched 0 of 739 rows and the
	// analyst was told "there are no CDR records involving VOLTE".
	//
	// A verified plan asserting false absence is the worst answer this product
	// can give: the analyst stops looking. The extraction was never wrong --
	// `classifyTargetType` already returns "call_type" for GPRS/SMS/VOLTE -- the
	// executor just ignored the type. The plan filters on the right column
	// already, so this predicate was both redundant and destructive.
	if target := strings.TrimSpace(req.Target); target != "" && targetIsSubjectIdentifier(target) {
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

// targetIsSubjectIdentifier reports whether a target names the SUBJECT of a
// record rather than the value of one of its fields.
//
// Deliberately an allowlist of known NON-subject types rather than a list of
// subject types: `classifyTargetType` falls back to "entity" for anything it
// does not recognise, and treating an unrecognised target as a non-subject
// would silently widen results. Widening is the failure mode that produces
// confident wrong answers; narrowing produces refusals. When in doubt this
// keeps today's behaviour.
func targetIsSubjectIdentifier(target string) bool {
	switch classifyTargetType(target) {
	case "call_type":
		// A call type is a value of `cdr.call_type`. The typed plan filters on
		// it directly and correctly; there is nothing for a subject predicate
		// to add.
		return false
	default:
		return true
	}
}
