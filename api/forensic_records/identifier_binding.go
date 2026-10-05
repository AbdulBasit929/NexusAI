package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// A3.3 — BIND A NAMED IDENTIFIER TO THE ONE FIELD THAT HOLDS IT.
//
// Measured 2026-09-27 by running the deterministic compiler directly against the
// live catalogue (projection_compiler_probe_test.go). Every identifier-bearing
// lookup extracted its identifier and then REFUSED:
//
//	SUB-03  CONSTRAINT_UNBOUND:identifier PK-SUB-SYN-ALPHA
//	H11     CONSTRAINT_UNBOUND:phone 923001110001
//	NEG-02  CONSTRAINT_UNBOUND:identifier ZZZ-0000
//
// with `identifiers=1 filters=0` in every frame. The compiler has NO path from
// an extracted identifier to a plan filter: `compileSourceNativeFrameFilters`
// reads only `frame.Filters`, and an identifier lives in `frame.Identifiers`.
// The CONSTRAINT_APPLIED guard then correctly refuses a plan that ignores it.
// The guard is right; the binding was missing.
//
// THE RULE: an identifier binds to a field ONLY when it occurs in EXACTLY ONE
// curated string field of the family, checked against the FULL authorized scope.
//
//   - Zero fields hold it: nothing binds. The entity is not in this case, and
//     whether to say "none found" or to clarify is not this function's call.
//   - Two or more hold it: nothing binds. A phone number on a CDR row sits in
//     BOTH the calling and the called field; binding it to one would silently
//     answer "calls FROM this number" when "calls involving it" was asked. That
//     is a confident wrong answer, and ambiguity is the analyst's to resolve.
//   - Exactly one: bind EQ. The binding is a fact read from the data, never
//     inferred from a field name or an embedding.
//
// The check reuses the executor's own scope builder (`sourceNativeScopeWhere`),
// its own field expression (`sourceNativeFieldRawExpr`) and its own
// tenant-aware query path (`queryRows`), so it sees precisely the rows that
// execution will see.
//
// RECORDS FAMILIES ONLY. A derived family reads another table through nested
// JSON paths and has its own provenance rules; it is out of scope here.
const identifierBindingEnv = "FORENSIC_IDENTIFIER_BINDING"

func identifierBindingEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(identifierBindingEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// STATUS 2026-09-27: MEASURED CLEAN, HELD OFF. Across 103 corpus questions the
// binding moved nothing -- as predicted, because it only reaches questions the
// keyword ladder abstains on, and none of those names a single-field
// identifier. It is correct and safe, but it has no route to TWR-02 on its own:
// the ladder claims that question, so the deterministic compiler never runs for
// it. The routing that would reach it was tried BEFORE the IR arbitration and
// reverted for pre-empting correct IR rescues (query.go, at the arbitration
// site; reports/identifier-binding-20260927/A3_3_RESULT.md).

// identifierCandidateFields are the curated, non-sensitive string fields that
// accept an equality filter -- the only fields an identifier could be a value
// of. Inferred fields are excluded: nothing the layer does not describe is ever
// bound.
func identifierCandidateFields(catalog []FieldDescriptorV1) []FieldDescriptorV1 {
	out := []FieldDescriptorV1{}
	for _, field := range catalog {
		if !field.Curated || !strings.EqualFold(field.EffectiveType, "STRING") {
			continue
		}
		if !containsString(field.AllowedFilters, "EQ") {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(field.Sensitivity)) {
		case "PII", "RESTRICTED":
			continue
		}
		out = append(out, field)
	}
	return out
}

// fieldsHoldingValue returns every candidate field in which `value` occurs at
// least once across the authorized scope. One query, one scan of the family:
// each field is tested with bool_or over the same expression the executor
// uses, so presence here means presence at execution.
func fieldsHoldingValue(ctx context.Context, db *pgxpool.Pool, scoped hybridQueryRequest, candidates []FieldDescriptorV1, value string) ([]FieldDescriptorV1, error) {
	where, args, err := sourceNativeScopeWhere(scoped, nil)
	if err != nil {
		return nil, err
	}
	// EXACT PARITY WITH EXECUTION. The executor renders EQ as
	// `(typedExpr) = castLiteral($v)` -- case-sensitive, untrimmed
	// (source_native_sql_executor.go, the EQ branch). The first version of this
	// check compared lower(btrim(...)) instead, which would have found
	// "pk-lhr-syn-001" here and then matched ZERO rows at execution, stating a
	// confident "none found" about a tower that exists. Presence is therefore
	// tested with the executor's own typed expression and literal cast, so a
	// value that binds here is a value execution will match.
	args = append(args, value)
	valueParam := fmt.Sprintf("$%d", len(args))
	binding := sourceNativeRecordsBinding()
	selects := make([]string, 0, len(candidates))
	for index, field := range candidates {
		expression := sourceNativeFieldTypedExpr(binding, field, &args)
		literal := sourceNativeCastLiteral(field, valueParam)
		selects = append(selects, fmt.Sprintf("COALESCE(bool_or((%s) = %s),false) AS f%d", expression, literal, index))
	}
	query := "SELECT " + strings.Join(selects, ",") + " FROM forensic.records r WHERE " + strings.Join(where, " AND ")
	rows, err := queryRows(ctx, db, scoped.TenantID, query, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, fmt.Errorf("identifier presence query returned %d rows", len(rows))
	}
	holding := []FieldDescriptorV1{}
	for index, field := range candidates {
		if present, _ := rows[0][fmt.Sprintf("f%d", index)].(bool); present {
			holding = append(holding, field)
		}
	}
	return holding, nil
}

// asksForWithheldIdentity reports a WHO question about an entity whose identity
// is withheld as PII.
//
// Measured 2026-09-27 on SUB-03, "Who is the registered subscriber for
// PK-SUB-SYN-ALPHA?": with binding on, the compiler bound `subscriber_id` and
// projected the subscriber row -- MSISDN, IMEI, city, status -- while the name
// and CNIC, the actual answer to "who", are PII and are excluded from the
// catalogue by design. A record with the "who" removed, presented as the answer
// to "who", is a non-answer that reads like one. Today SUB-03 refuses, which is
// correct until masked CNIC projection exists.
//
// Keyed on the LAYER'S OWN declarations, not on a list of families: any entity
// that declares a PII or RESTRICTED field has an identity this workspace does
// not disclose. The guard only ever WITHHOLDS a binding, so it can return a
// question to today's behaviour and can never make one answer.
func asksForWithheldIdentity(question string, entity SemanticLayerEntityV1) bool {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(question)))
	if len(words) == 0 || (words[0] != "who" && words[0] != "whose") {
		return false
	}
	for _, field := range entity.Fields {
		switch strings.ToUpper(strings.TrimSpace(field.Sensitivity)) {
		case "PII", "RESTRICTED":
			return true
		}
	}
	return false
}

// bindIdentifiersByPresence returns one EQ filter per identifier that occurs in
// exactly one candidate field. An identifier that cannot be bound is simply
// omitted: CONSTRAINT_APPLIED will then refuse the plan and name it, which is
// the honest outcome and today's behaviour.
func bindIdentifiersByPresence(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, frame SemanticFrameV1, catalog []FieldDescriptorV1) []SourceNativeFilterV1 {
	if !identifierBindingEnabled() || db == nil || len(frame.Identifiers) == 0 || len(catalog) == 0 {
		return nil
	}
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return nil
	}
	entity, ok := layer.EntityByFamily(frame.FamilyHint)
	if !ok || entity.Source.IsDerived() || strings.TrimSpace(entity.RecordType) == "" {
		return nil
	}
	if asksForWithheldIdentity(req.Query, entity) {
		return nil
	}
	candidates := identifierCandidateFields(catalog)
	if len(candidates) == 0 {
		return nil
	}
	scoped := req
	scoped.RecordType = entity.RecordType
	filters := []SourceNativeFilterV1{}
	bound := map[string]bool{}
	for _, identifier := range frame.Identifiers {
		value := strings.TrimSpace(identifier.Canonical)
		if value == "" {
			value = strings.TrimSpace(identifier.Raw)
		}
		if value == "" {
			continue
		}
		holding, err := fieldsHoldingValue(ctx, db, scoped, candidates, value)
		if err != nil || len(holding) != 1 || bound[holding[0].FieldID] {
			continue
		}
		filters = append(filters, SourceNativeFilterV1{FieldID: holding[0].FieldID, Op: "EQ", Value: value, Values: []string{}})
		bound[holding[0].FieldID] = true
	}
	return filters
}
