package main

import (
	"fmt"
	"sort"
	"strings"

	pg "github.com/pganalyze/pg_query_go/v6"
	pgq "github.com/wasilibs/go-pgquery"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ASK ENGINE V2: THE VALIDATOR FOR MODEL-WRITTEN SQL OVER THE ANALYST VIEWS (roadmap 6.8.4 and appendix D.2).
//
// The governed lane's validator (governed_sql_validate.go) allows one view, no joins and no recursion, and it is left exactly as it is. The Ask engine
// lets the model write joins, WITH clauses, set operations, array walks and bounded recursion over the reviewed views of schema analyst, so it needs its
// own gate, built the same way: the text becomes PostgreSQL's own parse tree and is accepted only if EVERY node is on an allowlist. Whatever the
// walker has not been told about fails closed.
//
// What it adds to the lane's rules, and what it keeps:
//   - one statement, a SELECT (or WITH ... SELECT); no INTO, no locking, no VALUES; set operations are allowed (common contacts is an INTERSECT);
//   - relations: only the analyst views the catalogue lists (and the shortlist given for this question), unqualified or as analyst.<name>, at most six,
//     plus the query's own WITH names; no other schema, no system catalogue, no function-valued relation except generate_series and unnest;
//   - joins need a predicate: a JOIN without ON or USING, or a comma list of relations without a WHERE, is a cross product and is refused (a function
//     such as generate_series may be joined without one);
//   - WITH RECURSIVE only with (a) a depth column that the recursive term increments and that is bounded by a literal so that at most four hops are
//     taken, (b) a cycle guard (the visited path kept in an array and tested with ANY or ALL), and (c) an outer LIMIT of at most 5,000;
//   - functions and operators only from the lists below; casts only to plain scalar types and inet or uuid; arrays only as ARRAY[...], subscripts and
//     ANY or ALL tests; bounded size, depth and constant length; an outer LIMIT, when present, of at most 5,000.
//
// This gate is one of several layers. The others are the read-only transaction, the role forensic_analyst_ro that can read only the views, the scope
// the views carry, the statement timeout, the EXPLAIN cost ceiling and the row cap in the executor. The validator never relies on them and they never
// rely on it. It also cannot judge meaning: that the recursion really walks, that a join relates the right columns, that the answer is the answer to the
// question are the jobs of the checks of roadmap 6.8.6 and of the database's own errors.
const (
	askMaxChars          = 6000
	askMaxNodes          = 4000
	askMaxDepth          = 40
	askMaxConstChars     = 400
	askMaxRelations      = 6
	askMaxRecursionHops  = 4
	askMaxLimit          = 5000
	askMaxSeriesConstant = 100000
)

// askMessages is the allowlist of parse-tree message types.
var askMessages = map[string]bool{
	"pg_query.Node": true, "pg_query.RawStmt": true, "pg_query.SelectStmt": true, "pg_query.ResTarget": true,
	"pg_query.ColumnRef": true, "pg_query.A_Star": true, "pg_query.A_Const": true, "pg_query.A_Expr": true,
	"pg_query.A_ArrayExpr": true, "pg_query.A_Indirection": true, "pg_query.A_Indices": true,
	"pg_query.BoolExpr": true, "pg_query.BooleanTest": true, "pg_query.NullTest": true,
	"pg_query.CaseExpr": true, "pg_query.CaseWhen": true, "pg_query.CoalesceExpr": true, "pg_query.MinMaxExpr": true,
	"pg_query.FuncCall": true, "pg_query.WindowDef": true, "pg_query.TypeCast": true, "pg_query.TypeName": true,
	"pg_query.SubLink": true, "pg_query.SortBy": true, "pg_query.CommonTableExpr": true, "pg_query.WithClause": true,
	"pg_query.RangeVar": true, "pg_query.RangeSubselect": true, "pg_query.RangeFunction": true, "pg_query.JoinExpr": true,
	"pg_query.Alias": true, "pg_query.SQLValueFunction": true,
	"pg_query.String": true, "pg_query.Integer": true, "pg_query.Float": true, "pg_query.Boolean": true, "pg_query.List": true,
}

// askFunctions is every function a query may call: the lane's list plus the few the graph and array work needs. No file, network, sleep, random,
// setting, catalogue or system-information function is here, and none can be added by the model. generate_series and unnest are FROM-only.
var askFunctions = map[string]bool{
	"count": true, "sum": true, "avg": true, "min": true, "max": true, "stddev": true, "variance": true,
	"percentile_cont": true, "percentile_disc": true, "string_agg": true, "bool_and": true, "bool_or": true, "array_agg": true,
	"round": true, "floor": true, "ceil": true, "ceiling": true, "abs": true, "trunc": true, "mod": true, "sign": true,
	"greatest": true, "least": true,
	"lower": true, "upper": true, "length": true, "char_length": true, "left": true, "right": true,
	"substr": true, "substring": true, "trim": true, "btrim": true, "ltrim": true, "rtrim": true, "replace": true,
	"concat": true, "concat_ws": true, "split_part": true, "regexp_replace": true, "position": true,
	"to_char": true, "date_trunc": true, "date_part": true, "extract": true, "age": true, "timezone": true, "date": true,
	"now": true, "row_number": true, "rank": true, "dense_rank": true, "lag": true, "lead": true, "first_value": true, "last_value": true,
	"ntile":       true,
	"cardinality": true, "array_length": true, "array_position": true, "array_append": true, "array_cat": true, "array_remove": true,
	"make_interval": true, "host": true, "family": true,
}

// askRangeFunctions are the only functions that may stand in FROM.
var askRangeFunctions = map[string]bool{"generate_series": true, "unnest": true}

var askPgCatalogForms = map[string]bool{
	"extract": true, "date_part": true, "timezone": true, "btrim": true, "ltrim": true, "rtrim": true,
	"substring": true, "position": true, "overlay": true, "char_length": true, "length": true, "date": true,
}

var askOperators = map[string]bool{
	"=": true, "<>": true, "!=": true, "<": true, "<=": true, ">": true, ">=": true,
	"+": true, "-": true, "*": true, "/": true, "%": true, "||": true,
	"~": true, "~*": true, "!~": true, "!~*": true, "~~": true, "~~*": true, "!~~": true, "!~~*": true,
	"BETWEEN": true, "NOT BETWEEN": true, "BETWEEN SYMMETRIC": true, "NOT BETWEEN SYMMETRIC": true,
	"<<": true, "<<=": true, ">>": true, ">>=": true, "@>": true, "<@": true, "&&": true,
}

var askCastTypes = map[string]bool{
	"text": true, "varchar": true, "bpchar": true, "int2": true, "int4": true, "int8": true, "numeric": true,
	"float4": true, "float8": true, "bool": true, "date": true, "time": true, "timestamp": true, "timestamptz": true, "interval": true,
	"inet": true, "uuid": true,
}

// askSQLValueFunctions: the clock is allowed; who is running the query is not.
var askSQLValueFunctions = map[string]bool{
	"SVFOP_CURRENT_DATE": true, "SVFOP_CURRENT_TIME": true, "SVFOP_CURRENT_TIMESTAMP": true,
	"SVFOP_LOCALTIME": true, "SVFOP_LOCALTIMESTAMP": true,
}

var askSubLinks = map[string]bool{"EXISTS_SUBLINK": true, "ALL_SUBLINK": true, "ANY_SUBLINK": true, "EXPR_SUBLINK": true}

// askValidated is a query that passed. SQL is the parser's own canonical text, the only text that is ever executed.
type askValidated struct {
	SQL       string
	Views     []string // analyst views read, sorted
	Columns   []string // view columns referenced, sorted
	Consts    []govSQLConst
	Recursive bool
	MaxHops   int // the most hops a recursive query can take; 0 when it is not recursive
	Limit     int // the outer LIMIT; 0 when none
	Tree      *pg.ParseResult
}

type askWalk struct {
	nodes     int
	err       *govSQLRejection
	rangeVars []string
	ctes      map[string]bool
	cteCols   map[string]bool
	aliases   map[string]bool
	colRefs   []string
	consts    []govSQLConst
	rangeFunc map[*pg.FuncCall]bool
	recursive []*pg.CommonTableExpr
	maxHops   int
}

func (w *askWalk) fail(code, format string, args ...any) {
	if w.err == nil {
		w.err = govSQLReject(code, format, args...)
	}
}

// askValidate parses sql and applies every rule above. allowed names the views offered for this question (the shortlist); nil means every view of the
// catalogue.
func askValidate(sql string, cat *askCatalogue, allowed []string) (*askValidated, *govSQLRejection) {
	text := strings.TrimSpace(sql)
	switch {
	case text == "":
		return nil, govSQLReject("EMPTY", "the query is empty")
	case len(text) > askMaxChars:
		return nil, govSQLReject("TOO_LONG", "the query is longer than %d characters; write a shorter one", askMaxChars)
	case strings.ContainsRune(text, 0):
		return nil, govSQLReject("BAD_TEXT", "the query contains a NUL character")
	}
	tree, err := pgq.Parse(text)
	if err != nil {
		return nil, govSQLReject("SYNTAX", "the query is not valid PostgreSQL: %s", govSQLClip(err.Error(), 160))
	}
	if len(tree.Stmts) != 1 {
		return nil, govSQLReject("ONE_STATEMENT", "exactly one SELECT statement is allowed, found %d", len(tree.Stmts))
	}
	root := tree.Stmts[0].GetStmt().GetSelectStmt()
	if root == nil {
		return nil, govSQLReject("SELECT_ONLY", "only a SELECT statement is allowed")
	}

	w := &askWalk{ctes: map[string]bool{}, cteCols: map[string]bool{}, aliases: map[string]bool{}, rangeFunc: map[*pg.FuncCall]bool{}}
	w.walk(tree.Stmts[0].ProtoReflect(), 0)
	if w.err != nil {
		return nil, w.err
	}

	offered := askOffered(cat, allowed)
	used, rej := askResolveRelations(w, offered)
	if rej != nil {
		return nil, rej
	}
	referenced, rej := askResolveColumns(w, offered, used)
	if rej != nil {
		return nil, rej
	}
	limit, rej := askOuterLimit(root, len(w.recursive) > 0)
	if rej != nil {
		return nil, rej
	}

	canonical, err := pgq.Deparse(tree)
	if err != nil || strings.TrimSpace(canonical) == "" {
		return nil, govSQLReject("DEPARSE", "the query could not be normalised")
	}
	return &askValidated{SQL: strings.TrimSpace(canonical), Views: used, Columns: referenced, Consts: w.consts, Recursive: len(w.recursive) > 0,
		MaxHops: w.maxHops, Limit: limit, Tree: tree}, nil
}

func askOffered(cat *askCatalogue, allowed []string) map[string]*askView {
	out := map[string]*askView{}
	if cat == nil {
		return out
	}
	for i := range cat.Views {
		view := &cat.Views[i]
		if len(allowed) == 0 {
			out[view.Name] = view
			continue
		}
		for _, name := range allowed {
			if name == view.Name {
				out[view.Name] = view
			}
		}
	}
	return out
}

func askViewNames(views map[string]*askView) string {
	names := make([]string, 0, len(views))
	for name := range views {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func askResolveRelations(w *askWalk, offered map[string]*askView) ([]string, *govSQLRejection) {
	for name := range w.ctes {
		if offered[name] != nil || strings.HasPrefix(name, "pg_") || strings.HasPrefix(name, "v_") {
			return nil, govSQLReject("CTE_NAME", "a WITH name may not reuse a view name or start with pg_ or v_")
		}
	}
	seen := map[string]bool{}
	for _, name := range w.rangeVars {
		switch {
		case w.ctes[name]:
		case offered[name] != nil:
			seen[name] = true
		default:
			return nil, govSQLReject("UNKNOWN_RELATION", "%q is not an available view; the views are: %s", govSQLClip(name, 60), askViewNames(offered))
		}
	}
	used := make([]string, 0, len(seen))
	for name := range seen {
		used = append(used, name)
	}
	sort.Strings(used)
	if len(used) == 0 {
		return nil, govSQLReject("NO_VIEW", "the query must read at least one of the views: %s", askViewNames(offered))
	}
	if len(used) > askMaxRelations {
		return nil, govSQLReject("TOO_MANY_VIEWS", "the query reads %d views; at most %d are allowed", len(used), askMaxRelations)
	}
	return used, nil
}

func askResolveColumns(w *askWalk, offered map[string]*askView, used []string) ([]string, *govSQLRejection) {
	valid := map[string]bool{}
	byView := map[string]bool{}
	for _, name := range used {
		for _, c := range offered[name].Columns {
			valid[c.Name] = true
			byView[c.Name] = true
		}
	}
	for name := range w.aliases {
		valid[name] = true
	}
	for name := range w.cteCols {
		valid[name] = true
	}
	referenced := map[string]bool{}
	for _, name := range w.colRefs {
		if !valid[name] {
			cols := make([]string, 0, len(byView))
			for c := range byView {
				cols = append(cols, c)
			}
			sort.Strings(cols)
			return nil, govSQLReject("UNKNOWN_COLUMN", "%q is not a column of the views used (%s) nor a name the query defines; their columns are: %s",
				govSQLClip(name, 60), strings.Join(used, ", "), govSQLClip(strings.Join(cols, ", "), 400))
		}
		if byView[name] {
			referenced[name] = true
		}
	}
	out := make([]string, 0, len(referenced))
	for name := range referenced {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// askOuterLimit reads the outermost LIMIT. A recursive query must have one.
func askOuterLimit(root *pg.SelectStmt, recursive bool) (int, *govSQLRejection) {
	if root.LimitCount == nil {
		if recursive {
			return 0, govSQLReject("REC_LIMIT", "a recursive query needs an outer LIMIT of at most %d", askMaxLimit)
		}
		return 0, nil
	}
	c := root.LimitCount.GetAConst()
	if c == nil || c.GetIval() == nil {
		return 0, govSQLReject("LIMIT", "LIMIT must be a whole number written in the query, at most %d", askMaxLimit)
	}
	n := int(c.GetIval().Ival)
	if n < 1 || n > askMaxLimit {
		return 0, govSQLReject("LIMIT", "LIMIT must be between 1 and %d", askMaxLimit)
	}
	return n, nil
}

func (w *askWalk) walk(m protoreflect.Message, depth int) {
	if w.err != nil {
		return
	}
	name := string(m.Descriptor().FullName())
	if !askMessages[name] {
		w.fail("NOT_ALLOWED", "%s is not allowed; use only SELECT with joins, WITH, filters, grouping, ordering and the listed functions", govSQLHumanName(name))
		return
	}
	w.nodes++
	if w.nodes > askMaxNodes || depth > askMaxDepth {
		w.fail("TOO_COMPLEX", "the query is too large or too deeply nested")
		return
	}
	w.check(m)
	if w.err != nil {
		return
	}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
		case fd.IsList() && fd.Message() != nil:
			list := v.List()
			for i := 0; i < list.Len(); i++ {
				w.walk(list.Get(i).Message(), depth+1)
			}
		case fd.Message() != nil:
			w.walk(v.Message(), depth+1)
		}
		return w.err == nil
	})
}

// check applies the rules specific to one message type.
func (w *askWalk) check(m protoreflect.Message) {
	switch n := m.Interface().(type) {
	case *pg.SelectStmt:
		w.checkSelect(n)
	case *pg.JoinExpr:
		w.checkJoin(n)
	case *pg.RangeFunction:
		w.checkRangeFunction(n)
	case *pg.RangeVar:
		if n.Catalogname != "" || (n.Schemaname != "" && n.Schemaname != "analyst") {
			w.fail("NO_SCHEMA", "name a view without a schema, or as analyst.<view>; other schemas are not allowed")
			return
		}
		w.rangeVars = append(w.rangeVars, n.Relname)
		if n.Alias != nil && n.Alias.Aliasname != "" {
			w.aliases[n.Alias.Aliasname] = true
		}
	case *pg.CommonTableExpr:
		w.ctes[n.Ctename] = true
		for _, c := range n.Aliascolnames {
			if s := c.GetString_(); s != nil {
				w.cteCols[s.Sval] = true
			}
		}
		w.noteTargetNames(n.Ctequery.GetSelectStmt())
	case *pg.ResTarget:
		if n.Name != "" {
			w.aliases[n.Name] = true
		}
	case *pg.Alias:
		if n.Aliasname != "" {
			w.aliases[n.Aliasname] = true
		}
		for _, c := range n.Colnames {
			if s := c.GetString_(); s != nil {
				w.aliases[s.Sval] = true
			}
		}
	case *pg.ColumnRef:
		w.checkColumnRef(n)
	case *pg.FuncCall:
		w.checkFunc(n)
	case *pg.A_Expr:
		w.checkOperator(n)
	case *pg.TypeCast:
		w.checkType(n.TypeName)
	case *pg.SubLink:
		kind := strings.TrimPrefix(n.SubLinkType.String(), "SUB_LINK_TYPE_")
		kind = strings.TrimPrefix(kind, "SubLinkType_")
		if !askSubLinks[kind] && !askSubLinks[n.SubLinkType.String()] {
			w.fail("SUBLINK", "that kind of subquery is not allowed")
		}
	case *pg.SQLValueFunction:
		if !askSQLValueFunctions[n.Op.String()] {
			w.fail("VALUE_FUNCTION", "that function (who is running the query) is not allowed")
		}
	case *pg.A_Const:
		w.checkConst(n)
	case *pg.TypeName:
		if len(n.ArrayBounds) > 0 {
			w.fail("TYPE", "array types are not allowed; build arrays with ARRAY[...]")
		}
	}
}

// noteTargetNames records the output column names of a WITH query, so that the query's later references to them are not mistaken for unknown columns.
func (w *askWalk) noteTargetNames(sel *pg.SelectStmt) {
	for sel != nil && sel.Larg != nil {
		sel = sel.Larg
	}
	if sel == nil {
		return
	}
	for _, t := range sel.TargetList {
		rt := t.GetResTarget()
		if rt == nil {
			continue
		}
		switch {
		case rt.Name != "":
			w.cteCols[rt.Name] = true
		case rt.Val.GetColumnRef() != nil:
			fields := rt.Val.GetColumnRef().Fields
			if s := fields[len(fields)-1].GetString_(); s != nil {
				w.cteCols[s.Sval] = true
			}
		case rt.Val.GetFuncCall() != nil:
			funcname := rt.Val.GetFuncCall().Funcname
			if s := funcname[len(funcname)-1].GetString_(); s != nil {
				w.cteCols[strings.ToLower(s.Sval)] = true
			}
		}
	}
}

func (w *askWalk) checkSelect(n *pg.SelectStmt) {
	switch {
	case n.IntoClause != nil:
		w.fail("NO_INTO", "SELECT INTO is not allowed")
	case len(n.LockingClause) > 0:
		w.fail("NO_LOCKING", "FOR UPDATE and FOR SHARE are not allowed")
	case len(n.ValuesLists) > 0:
		w.fail("NO_VALUES", "VALUES lists are not allowed")
	}
	if w.err != nil {
		return
	}
	if len(n.FromClause) > 1 {
		plain := 0
		for _, item := range n.FromClause {
			if item.GetRangeFunction() == nil {
				plain++
			}
		}
		if plain > 1 && askTrivialPredicate(n.WhereClause) {
			w.fail("CROSS_PRODUCT", "relations listed with commas need a WHERE that relates them; prefer JOIN ... ON")
			return
		}
	}
	if n.WithClause != nil && n.WithClause.Recursive {
		for _, c := range n.WithClause.Ctes {
			cte := c.GetCommonTableExpr()
			if cte != nil && w.checkRecursiveCTE(cte) {
				w.recursive = append(w.recursive, cte)
			}
		}
	}
}

func (w *askWalk) checkJoin(n *pg.JoinExpr) {
	if n.IsNatural {
		w.fail("CROSS_PRODUCT", "NATURAL JOIN is not allowed; write the join condition")
		return
	}
	if askTrivialPredicate(n.Quals) && len(n.UsingClause) == 0 && n.Rarg.GetRangeFunction() == nil {
		w.fail("CROSS_PRODUCT", "a JOIN needs an ON or USING condition; a join without one is a cross product")
	}
}

// askTrivialPredicate reports whether a join or WHERE condition is missing or relates nothing: absent, a bare constant (ON true) or a comparison of two
// constants (ON 1 = 1). A cross product written that way is still a cross product. It is a syntactic check; the EXPLAIN cost ceiling is the real one.
func askTrivialPredicate(n *pg.Node) bool {
	switch {
	case n == nil:
		return true
	case n.GetAConst() != nil:
		return true
	case n.GetTypeCast() != nil:
		return askTrivialPredicate(n.GetTypeCast().Arg)
	case n.GetAExpr() != nil:
		e := n.GetAExpr()
		return askTrivialPredicate(e.Lexpr) && askTrivialPredicate(e.Rexpr)
	}
	return false
}

func (w *askWalk) checkRangeFunction(n *pg.RangeFunction) {
	if n.IsRowsfrom || len(n.Functions) != 1 {
		w.fail("RANGE_FUNCTION", "only generate_series or unnest may be used in FROM, one at a time")
		return
	}
	var call *pg.FuncCall
	if list := n.Functions[0].GetList(); list != nil && len(list.Items) > 0 {
		call = list.Items[0].GetFuncCall()
	}
	if call == nil || len(call.Funcname) != 1 || !askRangeFunctions[strings.ToLower(call.Funcname[0].GetString_().GetSval())] {
		w.fail("RANGE_FUNCTION", "only generate_series or unnest may be used in FROM")
		return
	}
	if strings.EqualFold(call.Funcname[0].GetString_().GetSval(), "generate_series") {
		if len(call.Args) < 2 || len(call.Args) > 3 {
			w.fail("RANGE_FUNCTION", "generate_series takes two or three arguments")
			return
		}
		if hi := call.Args[1].GetAConst(); hi != nil && hi.GetIval() != nil && hi.GetIval().Ival > askMaxSeriesConstant {
			w.fail("RANGE_FUNCTION", "generate_series may not run past %d", askMaxSeriesConstant)
			return
		}
	}
	w.rangeFunc[call] = true
}

func (w *askWalk) checkColumnRef(n *pg.ColumnRef) {
	parts := make([]string, 0, 2)
	star := false
	for _, f := range n.Fields {
		if s := f.GetString_(); s != nil {
			parts = append(parts, s.Sval)
		} else if f.GetAStar() != nil {
			star = true
		}
	}
	if len(n.Fields) > 2 {
		w.fail("NO_QUALIFIED_COLUMN", "a column may be written as name or alias.name only")
		return
	}
	if !star && len(parts) > 0 {
		w.colRefs = append(w.colRefs, parts[len(parts)-1])
	}
}

func (w *askWalk) checkOperator(n *pg.A_Expr) {
	if len(n.Name) != 1 {
		w.fail("OPERATOR", "schema-qualified operators are not allowed")
		return
	}
	op := ""
	if s := n.Name[0].GetString_(); s != nil {
		op = s.Sval
	}
	if !askOperators[strings.ToUpper(op)] {
		w.fail("OPERATOR", "the operator %q is not allowed", govSQLClip(op, 20))
	}
}

func (w *askWalk) checkConst(n *pg.A_Const) {
	switch {
	case n.GetBsval() != nil:
		w.fail("CONSTANT", "bit-string constants are not allowed")
	case n.GetSval() != nil:
		text := n.GetSval().Sval
		if len(text) > askMaxConstChars {
			w.fail("CONSTANT", "a text constant is longer than %d characters", askMaxConstChars)
			return
		}
		w.consts = append(w.consts, govSQLConst{Text: text})
	case n.GetIval() != nil:
		w.consts = append(w.consts, govSQLConst{Text: fmt.Sprint(n.GetIval().Ival), Numeric: true})
	case n.GetFval() != nil:
		w.consts = append(w.consts, govSQLConst{Text: n.GetFval().Fval, Numeric: true})
	}
}

func (w *askWalk) checkFunc(n *pg.FuncCall) {
	if w.rangeFunc[n] {
		return
	}
	names := make([]string, 0, 2)
	for _, f := range n.Funcname {
		if s := f.GetString_(); s != nil {
			names = append(names, strings.ToLower(s.Sval))
		}
	}
	switch len(names) {
	case 1:
		if !askFunctions[names[0]] {
			w.fail("FUNCTION", "the function %q is not allowed here; allowed functions: %s", govSQLClip(names[0], 40), askFunctionList())
		}
	case 2:
		if names[0] != "pg_catalog" || !askPgCatalogForms[names[1]] {
			w.fail("FUNCTION", "schema-qualified function %q is not allowed", govSQLClip(strings.Join(names, "."), 60))
		}
	default:
		w.fail("FUNCTION", "that function name is not allowed")
	}
	if n.FuncVariadic {
		w.fail("FUNCTION", "VARIADIC calls are not allowed")
	}
}

func askFunctionList() string {
	names := make([]string, 0, len(askFunctions))
	for name := range askFunctions {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func (w *askWalk) checkType(t *pg.TypeName) {
	if t == nil {
		return
	}
	names := make([]string, 0, 2)
	for _, part := range t.Names {
		if s := part.GetString_(); s != nil {
			names = append(names, strings.ToLower(s.Sval))
		}
	}
	if len(names) == 0 || len(names) > 2 || (len(names) == 2 && names[0] != "pg_catalog") || !askCastTypes[names[len(names)-1]] {
		w.fail("TYPE", "casts are allowed to text, integer, numeric, float, boolean, date, time, timestamp, interval, inet and uuid only")
	}
}

// checkRecursiveCTE applies the recursion rules to one WITH RECURSIVE entry and reports whether it really recurses (refers to itself). A WITH entry
// under RECURSIVE that does not refer to itself is an ordinary one.
func (w *askWalk) checkRecursiveCTE(cte *pg.CommonTableExpr) bool {
	q := cte.Ctequery.GetSelectStmt()
	if q == nil || q.Op != pg.SetOperation_SETOP_UNION || q.Rarg == nil || !askReferences(q.Rarg, cte.Ctename) {
		return false
	}
	if askReferences(q.Larg, cte.Ctename) {
		w.fail("REC_SHAPE", "in a recursive query only the second part of the UNION may refer to the query itself; the first part is the starting rows")
		return true
	}
	anchor := q.Larg
	for anchor != nil && anchor.Larg != nil {
		anchor = anchor.Larg
	}
	hops, column := askDepthBound(q.Rarg.WhereClause)
	if column == "" {
		w.fail("REC_NO_DEPTH_BOUND", "a recursive query needs a depth column bounded in the recursive part's WHERE by a literal, for example depth < 3 (at most %d hops)", askMaxRecursionHops)
		return true
	}
	if hops > askMaxRecursionHops {
		w.fail("REC_DEPTH", "the recursion may take at most %d hops; this bound allows %d", askMaxRecursionHops, hops)
		return true
	}
	if !askDepthIncremented(cte, anchor, q.Rarg, column) {
		w.fail("REC_DEPTH_COLUMN", "the depth column %q must start in the first part and grow by a constant in the recursive part (for example depth + 1)", govSQLClip(column, 40))
		return true
	}
	if !askHasPathTest(q.Rarg.WhereClause) {
		w.fail("REC_CYCLE_GUARD", "a recursive query needs a cycle guard: keep the visited numbers in an array and test with NOT x = ANY (path)")
		return true
	}
	if hops > w.maxHops {
		w.maxHops = hops
	}
	return true
}

// askReferences reports whether a SELECT reads the named relation (in FROM or a subquery of it).
func askReferences(sel *pg.SelectStmt, name string) bool {
	if sel == nil {
		return false
	}
	found := false
	var visit func(m protoreflect.Message)
	visit = func(m protoreflect.Message) {
		if found {
			return
		}
		if rv, ok := m.Interface().(*pg.RangeVar); ok && rv.Relname == name && rv.Schemaname == "" {
			found = true
			return
		}
		m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			switch {
			case fd.IsMap():
			case fd.IsList() && fd.Message() != nil:
				list := v.List()
				for i := 0; i < list.Len(); i++ {
					visit(list.Get(i).Message())
				}
			case fd.Message() != nil:
				visit(v.Message())
			}
			return !found
		})
	}
	visit(sel.ProtoReflect())
	return found
}

// askConjuncts flattens the top-level AND of a WHERE.
func askConjuncts(n *pg.Node) []*pg.Node {
	if n == nil {
		return nil
	}
	if b := n.GetBoolExpr(); b != nil && b.Boolop == pg.BoolExprType_AND_EXPR {
		var out []*pg.Node
		for _, a := range b.Args {
			out = append(out, askConjuncts(a)...)
		}
		return out
	}
	return []*pg.Node{n}
}

func askColumnName(n *pg.Node) string {
	ref := n.GetColumnRef()
	if ref == nil || len(ref.Fields) == 0 {
		return ""
	}
	return ref.Fields[len(ref.Fields)-1].GetString_().GetSval()
}

// askDepthBound finds a top-level conjunct of the form depth < N or depth <= N (N a whole-number literal) and returns the most hops it allows and the
// column's name. depth < 3 allows 3 hops (depth starts at 1); depth <= 3 allows 4.
func askDepthBound(where *pg.Node) (int, string) {
	for _, c := range askConjuncts(where) {
		e := c.GetAExpr()
		if e == nil || len(e.Name) != 1 || e.Rexpr.GetAConst() == nil || e.Rexpr.GetAConst().GetIval() == nil {
			continue
		}
		column := askColumnName(e.Lexpr)
		if column == "" {
			continue
		}
		n := int(e.Rexpr.GetAConst().GetIval().Ival)
		switch e.Name[0].GetString_().GetSval() {
		case "<":
			return n, column
		case "<=":
			return n + 1, column
		}
	}
	return 0, ""
}

// askDepthIncremented checks that column is a column of the WITH entry, set in the first part, and that the recursive part's value for the same
// position is column + a positive whole-number literal.
func askDepthIncremented(cte *pg.CommonTableExpr, anchor, recursive *pg.SelectStmt, column string) bool {
	if anchor == nil {
		return false
	}
	position := -1
	if len(cte.Aliascolnames) > 0 {
		for i, c := range cte.Aliascolnames {
			if c.GetString_().GetSval() == column {
				position = i
			}
		}
	} else {
		for i, t := range anchor.TargetList {
			if t.GetResTarget() != nil && t.GetResTarget().Name == column {
				position = i
			}
		}
	}
	if position < 0 || position >= len(recursive.TargetList) {
		return false
	}
	e := recursive.TargetList[position].GetResTarget().GetVal().GetAExpr()
	if e == nil || len(e.Name) != 1 || e.Name[0].GetString_().GetSval() != "+" {
		return false
	}
	for _, pair := range [][2]*pg.Node{{e.Lexpr, e.Rexpr}, {e.Rexpr, e.Lexpr}} {
		step := pair[1].GetAConst()
		if askColumnName(pair[0]) == column && step != nil && step.GetIval() != nil && step.GetIval().Ival >= 1 {
			return true
		}
	}
	return false
}

// askHasPathTest reports whether the WHERE contains an array membership test (= ANY, <> ALL, and so on) anywhere: the shape of a cycle guard.
func askHasPathTest(where *pg.Node) bool {
	if where == nil {
		return false
	}
	found := false
	var visit func(m protoreflect.Message)
	visit = func(m protoreflect.Message) {
		if found {
			return
		}
		if e, ok := m.Interface().(*pg.A_Expr); ok && (e.Kind == pg.A_Expr_Kind_AEXPR_OP_ANY || e.Kind == pg.A_Expr_Kind_AEXPR_OP_ALL) {
			found = true
			return
		}
		m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			switch {
			case fd.IsMap():
			case fd.IsList() && fd.Message() != nil:
				list := v.List()
				for i := 0; i < list.Len(); i++ {
					visit(list.Get(i).Message())
				}
			case fd.Message() != nil:
				visit(v.Message())
			}
			return !found
		})
	}
	visit(where.ProtoReflect())
	return found
}
