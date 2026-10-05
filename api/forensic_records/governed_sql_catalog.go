package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	pgq "github.com/wasilibs/go-pgquery"
)

// GOVERNED SQL LANE (Phase 2/3 of docs/work/RUNTIME_QUERY_PLAN_20260929.md; the
// decision record is docs/architecture/QUERY_ANSWERING_DECISION_20261005.md).
//
// The typed lane (the Governed Semantic Compiler) is exact but can only say what
// its plan algebra can say: it has no earliest/latest, no time-of-day, no "who
// called both". The ladder in front of it answers first and drops what it cannot
// bind (CONTINUATION §3 D1/D4). This lane lets the model WRITE the query for the
// long tail, against analyst VIEWS that this file builds from the curated semantic
// layer, and then proves the query against the question before anything is shown.
//
// Two switches, both default off, both declared in compose and the startup
// registry:
//
//	FORENSIC_GOVERNED_SQL         the lane may run, AFTER the existing path declines
//	FORENSIC_GOVERNED_SQL_FIRST   the lane runs BEFORE the existing path (measurement
//	                              and, if it earns it, the route to retiring the ladder)
//
// WHAT A VIEW IS. One per curated entity: the entity's VISIBLE, non-sensitive
// fields as typed columns, built from the compiler's own expression builders
// (sourceNativeFieldTypedExpr, sourceNativeDerivedExpr) and its own scope
// functions, so the lane and the compiler cannot disagree about what a field
// means. The view is a CTE the SERVER prepends; the model never sees, names or
// can reach forensic.records. Tenant, case and family scope are part of the CTE,
// not of anything the model writes.
//
// WHAT IS NOT IN A VIEW, on purpose: PII and RESTRICTED fields, masked fields,
// and filter-only fields. CatalogFields already refuses to issue them to a typed
// plan; this lane takes exactly what a typed plan would be given, and then drops
// anything whose redaction state is not VISIBLE, so a model cannot read through
// SQL what it could not name in a plan.
const (
	governedSQLEnv      = "FORENSIC_GOVERNED_SQL"
	governedSQLFirstEnv = "FORENSIC_GOVERNED_SQL_FIRST"
)

func governedSQLEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(governedSQLEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

func governedSQLFirstEnabled() bool {
	return governedSQLEnabled() && strings.EqualFold(strings.TrimSpace(os.Getenv(governedSQLFirstEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

type govSQLColumn struct {
	Name        string
	FieldID     string
	Type        string // text | numeric | timestamptz | date | boolean
	Display     string
	Description string
	Identifier  bool
	Derived     bool
	Values      []string // declared canonical values, for a text column
	Synonyms    []string
	field       FieldDescriptorV1
}

type govSQLView struct {
	Name        string
	Family      string
	RecordType  string
	Display     string
	Description string
	Synonyms    []string
	Columns     []govSQLColumn
	// Provenance columns every view carries so a listing can be cited.
	Provenance []string
	binding    sourceNativeBinding
	entity     SemanticLayerEntityV1
}

func (v *govSQLView) columnNames(limit int) string {
	names := make([]string, 0, len(v.Columns)+len(v.Provenance))
	for _, c := range v.Columns {
		names = append(names, c.Name)
	}
	names = append(names, v.Provenance...)
	if limit > 0 && len(names) > limit {
		names = names[:limit]
	}
	return strings.Join(names, ", ")
}

// govSQLColumnType is the SQL type a column is declared with. It follows the
// effective type the compiler reasons about, so a number is a number (the
// "9,200 is bigger than 75,000" defect was a type mapping, CONTINUATION §1).
func govSQLColumnType(effective string) string {
	switch effective {
	case fieldTypeInteger, fieldTypeDecimal:
		return "numeric"
	case fieldTypeTimestamp:
		return "timestamptz"
	case fieldTypeDate:
		return "date"
	case fieldTypeBoolean:
		return "boolean"
	default:
		return "text"
	}
}

var (
	govSQLNameCache   sync.Map
	govSQLNameCacheMu sync.Mutex
)

// govSQLSafeName makes a field id's column part a plain lower-case identifier
// that PostgreSQL accepts WITHOUT quoting, so the model never has to quote one.
// The parser itself decides (SELECT 1 AS name): a word that does not parse bare
// is reserved and gets a suffix.
func govSQLSafeName(part string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(part) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	name := strings.Trim(b.String(), "_")
	if name == "" {
		name = "value"
	}
	if name[0] >= '0' && name[0] <= '9' {
		name = "c_" + name
	}
	if cached, ok := govSQLNameCache.Load(name); ok {
		return cached.(string)
	}
	govSQLNameCacheMu.Lock()
	defer govSQLNameCacheMu.Unlock()
	safe := name
	if _, err := pgq.Parse("SELECT 1 AS " + name); err != nil {
		safe = name + "_value"
	}
	govSQLNameCache.Store(name, safe)
	return safe
}

// govSQLViews builds a view for every curated entity. It reads the curated layer
// only; nothing here touches the database.
func govSQLViews(layer *SemanticLayerV1, req hybridQueryRequest) []*govSQLView {
	if layer == nil {
		return nil
	}
	views := make([]*govSQLView, 0, len(layer.Entities))
	taken := map[string]bool{}
	for _, entity := range layer.Entities {
		if len(entity.Fields) == 0 {
			continue
		}
		prefix := entity.Family
		if i := strings.Index(entity.Fields[0].ID, "."); i > 0 {
			prefix = entity.Fields[0].ID[:i]
		}
		name := "v_" + govSQLSafeName(prefix)
		if taken[name] {
			name += "_" + govSQLSafeName(entity.RecordType)
		}
		if taken[name] {
			continue
		}
		taken[name] = true
		values := map[string][]string{}
		for _, field := range entity.Fields {
			for _, value := range field.Values {
				values[field.ID] = append(values[field.ID], value.Value)
			}
		}
		scoped := req
		scoped.RecordType = entity.RecordType
		view := &govSQLView{
			Name: name, Family: entity.Family, RecordType: entity.RecordType, Display: entity.DisplayName,
			Description: govSQLFirstSentence(entity.Description, 200), Synonyms: append([]string(nil), entity.Synonyms...),
			entity: entity,
			binding: sourceNativeBinding{
				Table: semanticSourceTables[entity.Source.resolvedTable()], Payload: entity.Source.resolvedPayload(),
				ArtifactType: strings.TrimSpace(entity.Source.ArtifactType), ObservationType: strings.TrimSpace(entity.Source.ObservationType),
				Derived: entity.Source.IsDerived(),
			},
		}
		seen := map[string]bool{}
		for _, field := range entity.CatalogFields(scoped) {
			// Exactly what a typed plan would be issued, minus anything not plainly visible.
			if field.RedactionState != "VISIBLE" || (field.Sensitivity != "STANDARD" && field.Sensitivity != "IDENTIFIER") {
				continue
			}
			colName := field.FieldID
			if i := strings.Index(colName, "."); i >= 0 {
				colName = colName[i+1:]
			}
			colName = govSQLSafeName(colName)
			if seen[colName] {
				continue
			}
			seen[colName] = true
			view.Columns = append(view.Columns, govSQLColumn{
				Name: colName, FieldID: field.FieldID, Type: govSQLColumnType(field.EffectiveType), Display: field.DisplayName,
				Description: field.Description, Identifier: field.Sensitivity == "IDENTIFIER", Derived: field.DerivedOp != "",
				Values: values[field.FieldID], Synonyms: field.Synonyms, field: field,
			})
		}
		if len(view.Columns) == 0 {
			continue
		}
		if view.binding.Derived {
			view.Provenance = []string{"artifact_id", "evidence_id"}
		} else {
			view.Provenance = []string{"record_id", "source_file", "row_number"}
		}
		views = append(views, view)
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Name < views[j].Name })
	return views
}

func govSQLFirstSentence(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	if i := strings.Index(text, ". "); i > 0 {
		text = text[:i+1]
	}
	if len(text) > limit {
		text = text[:limit-3] + "..."
	}
	return text
}

// govSQLShortlist picks the views a question is about, deterministically and from
// the curated vocabulary only. NOT an embedding ranker: CONTINUATION §5 forbids
// embeddings for structural decisions, and two live defects came from them.
//
// A view scores on the phrases the curation declares for its entity (heavily),
// then on its fields' and values' phrases (lightly, capped). At most two views are
// returned, and the second only when it is close to the first, because a question
// that really spans families is declined later and a vague one is asked about.
func govSQLShortlist(views []*govSQLView, question string) []*govSQLView {
	stems := _semanticLayerQuestionTokens(question)
	type scored struct {
		view  *govSQLView
		score int
	}
	ranked := make([]scored, 0, len(views))
	for _, view := range views {
		score := 0
		for _, phrase := range append([]string{view.Display}, view.Synonyms...) {
			if _semanticLayerPhraseInQuestion(phrase, stems) {
				score += 4
			}
		}
		fieldHits, valueHits := 0, 0
		for _, column := range view.Columns {
			matched := false
			for _, phrase := range append([]string{column.Display}, column.Synonyms...) {
				if _semanticLayerPhraseInQuestion(phrase, stems) {
					matched = true
					break
				}
			}
			if matched {
				fieldHits++
			}
			for _, value := range column.Values {
				if _semanticLayerPhraseInQuestion(value, stems) {
					valueHits++
					break
				}
			}
		}
		score += minInt(fieldHits, 6) + 2*minInt(valueHits, 3)
		if score > 0 {
			ranked = append(ranked, scored{view, score})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].view.Name < ranked[j].view.Name
	})
	out := []*govSQLView{}
	for i, item := range ranked {
		if i == 0 || (i == 1 && item.score*2 >= ranked[0].score) {
			out = append(out, item.view)
		}
	}
	return out
}

var govSQLRxPlaceholder = regexp.MustCompile(`\$(\d+)`)

// govSQLShiftPlaceholders renumbers $n placeholders by offset.
func govSQLShiftPlaceholders(clauses []string, offset int) []string {
	if offset == 0 {
		return clauses
	}
	out := make([]string, len(clauses))
	for i, clause := range clauses {
		out[i] = govSQLRxPlaceholder.ReplaceAllStringFunc(clause, func(match string) string {
			n, _ := strconv.Atoi(match[1:])
			return "$" + strconv.Itoa(n+offset)
		})
	}
	return out
}

// govSQLCTE builds the view's definition for one request. The scope predicate is
// the compiler's own: tenant, case and (for a derived view) completed artifacts of
// exactly this contract. The request's target and date bounds are NOT applied;
// those are the question's conditions, and the model's WHERE carries them, which
// is what the verifier checks.
func govSQLCTE(view *govSQLView, req hybridQueryRequest, args []any) (string, []any, error) {
	scoped := req
	scoped.RecordType = view.RecordType
	var where []string
	var err error
	alias := "r"
	if view.binding.Derived {
		alias = "d"
		where, args, err = sourceNativeDerivedScopeWhere(scoped, args)
		if err != nil {
			return "", nil, err
		}
		args = append(args, view.binding.ArtifactType)
		where = append(where, fmt.Sprintf("d.artifact_type=$%d", len(args)))
		if view.binding.ObservationType != "" {
			args = append(args, view.binding.ObservationType)
			where = append(where, fmt.Sprintf("d.metadata->>'observation_type'=$%d", len(args)))
		}
	} else {
		// sourceNativeScopeWhere writes tenant and case as the literal placeholders $1
		// and $2, which is right for one query and wrong for the second view's CTE in a
		// query that carries several. Build the scope on its own and shift it past the
		// arguments already in use.
		local, localArgs, scopeErr := sourceNativeScopeWhere(scoped, nil)
		if scopeErr != nil {
			return "", nil, scopeErr
		}
		where = govSQLShiftPlaceholders(local, len(args))
		args = append(args, localArgs...)
	}
	fields := map[string]FieldDescriptorV1{}
	for _, column := range view.Columns {
		fields[column.field.FieldID] = column.field
	}
	// A computed metric needs its operand fields even when the operand is not a
	// visible column; the entity's own catalogue supplies them.
	for _, field := range view.entity.CatalogFields(scoped) {
		if _, ok := fields[field.FieldID]; !ok {
			fields[field.FieldID] = field
		}
	}
	selects := make([]string, 0, len(view.Columns)+len(view.Provenance))
	for _, column := range view.Columns {
		var expr string
		if column.Derived {
			expr, err = sourceNativeDerivedExpr(view.binding, column.field, fields, &args)
			if err != nil {
				return "", nil, err
			}
		} else {
			expr = sourceNativeFieldTypedExpr(view.binding, column.field, &args)
		}
		selects = append(selects, expr+" AS "+column.Name)
	}
	if view.binding.Derived {
		selects = append(selects, "d.artifact_id::text AS artifact_id", "d.evidence_id::text AS evidence_id")
	} else {
		selects = append(selects, "r.record_id::text AS record_id", "r.source_file AS source_file", "r.row_number AS row_number")
	}
	return fmt.Sprintf("%s AS (SELECT %s FROM %s %s WHERE %s)", view.Name, strings.Join(selects, ", "), view.binding.Table, alias, strings.Join(where, " AND ")), args, nil
}
