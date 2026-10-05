package main

import (
	"fmt"
	"sort"
	"strings"

	pg "github.com/pganalyze/pg_query_go/v6"
	pgq "github.com/wasilibs/go-pgquery"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GOVERNED SQL: THE VALIDATOR IS THE SECURITY GATE.
//
// A model writes SQL; nothing it writes is trusted. This file turns the text into
// PostgreSQL's own parse tree (libpg_query, compiled to WebAssembly and run in
// pure Go, so the distroless CGO_ENABLED=0 image keeps working) and accepts it
// only if EVERY node is on an allowlist. Anything the walker has not been told
// about is rejected, so a construct nobody thought of fails closed.
//
// The rules, in the order an attacker would try them:
//
//   - exactly one statement, and it is a SELECT (a second statement, DDL, DML,
//     COPY, SET, EXPLAIN, DO, ... are not on the list);
//   - no INTO, no FOR UPDATE/SHARE, no VALUES, no TABLESAMPLE, no recursion,
//     no joins, no function in FROM, no parameters;
//   - only the views the catalogue issued for THIS question, unqualified, and
//     exactly one of them (a join is not attempted; see "one view" below);
//   - functions only from a fixed list, unqualified or pg_catalog-qualified for
//     the handful of SQL-standard forms that parse that way (EXTRACT, AT TIME ZONE);
//   - operators only from a fixed list, never schema-qualified;
//   - casts only to plain scalar types (never regclass, oid, arrays);
//   - bounded size, depth and constant length.
//
// ONE VIEW. docs/architecture/RECONCILIATION_20260921.md §G.4: "the join graph is
// declared, not inferred; an undeclared join is not attempted". Nothing declares a
// working join today (CONTINUATION §7: cdr.msisdn -> subscriber.msisdn matches 0
// rows), and a join that matches nothing manufactures a confident empty answer.
// So a query reads one view; a question that needs two is declined, not guessed.
//
// This gate is one of four layers. The others are the read-only transaction and
// timeout in governed_sql_exec.go, the server-built view that carries the tenant
// and case scope, and (when the owner approves the database change) a role that
// can read nothing but the analyst views. The validator never relies on the
// others and they never rely on it.
const (
	govSQLMaxChars      = 4000
	govSQLMaxNodes      = 1500
	govSQLMaxDepth      = 24
	govSQLMaxConstChars = 400
)

// govSQLMessages is the allowlist of parse-tree message types. pg_query.Node is
// only the wrapper around one of the others and is walked through.
var govSQLMessages = map[string]bool{
	"pg_query.Node": true, "pg_query.RawStmt": true, "pg_query.SelectStmt": true, "pg_query.ResTarget": true,
	"pg_query.ColumnRef": true, "pg_query.A_Star": true, "pg_query.A_Const": true, "pg_query.A_Expr": true,
	"pg_query.A_ArrayExpr": true, "pg_query.BoolExpr": true, "pg_query.BooleanTest": true, "pg_query.NullTest": true,
	"pg_query.CaseExpr": true, "pg_query.CaseWhen": true, "pg_query.CoalesceExpr": true, "pg_query.MinMaxExpr": true,
	"pg_query.FuncCall": true, "pg_query.WindowDef": true, "pg_query.TypeCast": true, "pg_query.TypeName": true,
	"pg_query.SubLink": true, "pg_query.SortBy": true, "pg_query.CommonTableExpr": true, "pg_query.WithClause": true,
	"pg_query.RangeVar": true, "pg_query.RangeSubselect": true, "pg_query.Alias": true, "pg_query.SQLValueFunction": true,
	"pg_query.String": true, "pg_query.Integer": true, "pg_query.Float": true, "pg_query.Boolean": true, "pg_query.List": true,
}

// govSQLFunctions is every function a query may call. Aggregates, rounding, string
// and date helpers an analyst question needs, and nothing that touches the server:
// no file, network, sleep, random, setting, catalogue or system-information
// function is here, and none can be added by the model.
var govSQLFunctions = map[string]bool{
	"count": true, "sum": true, "avg": true, "min": true, "max": true, "stddev": true, "variance": true,
	"percentile_cont": true, "percentile_disc": true, "string_agg": true, "bool_and": true, "bool_or": true,
	"round": true, "floor": true, "ceil": true, "ceiling": true, "abs": true, "trunc": true, "mod": true, "sign": true,
	"lower": true, "upper": true, "length": true, "char_length": true, "left": true, "right": true,
	"substr": true, "substring": true, "trim": true, "btrim": true, "ltrim": true, "rtrim": true, "replace": true,
	"concat": true, "concat_ws": true, "split_part": true, "regexp_replace": true, "position": true,
	"to_char": true, "date_trunc": true, "date_part": true, "extract": true, "age": true, "timezone": true, "date": true,
	"now": true, "row_number": true, "rank": true, "dense_rank": true, "lag": true, "lead": true, "first_value": true, "last_value": true,
	"ntile": true,
}

// govSQLPgCatalogForms are the functions the SQL grammar itself rewrites into
// pg_catalog.<name> (EXTRACT, AT TIME ZONE, TRIM, SUBSTRING, POSITION, ...). Only
// these may be schema-qualified, and only by pg_catalog.
var govSQLPgCatalogForms = map[string]bool{
	"extract": true, "date_part": true, "timezone": true, "btrim": true, "ltrim": true, "rtrim": true,
	"substring": true, "position": true, "overlay": true, "char_length": true, "length": true, "date": true,
}

var govSQLOperators = map[string]bool{
	"=": true, "<>": true, "!=": true, "<": true, "<=": true, ">": true, ">=": true,
	"+": true, "-": true, "*": true, "/": true, "%": true, "||": true,
	"~": true, "~*": true, "!~": true, "!~*": true, "~~": true, "~~*": true, "!~~": true, "!~~*": true,
	"BETWEEN": true, "NOT BETWEEN": true, "BETWEEN SYMMETRIC": true, "NOT BETWEEN SYMMETRIC": true,
}

var govSQLCastTypes = map[string]bool{
	"text": true, "varchar": true, "bpchar": true, "int2": true, "int4": true, "int8": true, "numeric": true,
	"float4": true, "float8": true, "bool": true, "date": true, "time": true, "timestamp": true, "timestamptz": true, "interval": true,
}

// govSQLValueFunctions: the clock is allowed; who is running the query is not.
var govSQLValueFunctions = map[string]bool{
	"SVFOP_CURRENT_DATE": true, "SVFOP_CURRENT_TIME": true, "SVFOP_CURRENT_TIMESTAMP": true,
	"SVFOP_LOCALTIME": true, "SVFOP_LOCALTIMESTAMP": true,
}

var govSQLSubLinks = map[string]bool{
	"EXISTS_SUBLINK": true, "ALL_SUBLINK": true, "ANY_SUBLINK": true, "EXPR_SUBLINK": true,
}

// govSQLRejection is what the model is told when its query is refused. The text is
// written to be fed back for the one correction a question gets, so it names the
// problem and what is allowed, and never repeats attacker-controlled text at length.
type govSQLRejection struct {
	Code    string
	Message string
}

func (r *govSQLRejection) Error() string { return r.Code + ": " + r.Message }

func govSQLReject(code, format string, args ...any) *govSQLRejection {
	return &govSQLRejection{Code: code, Message: fmt.Sprintf(format, args...)}
}

// govSQLConst is a literal in the query. The verifier reads these to confirm the
// question's own values reached the query.
type govSQLConst struct {
	Text    string
	Numeric bool
}

// govSQLValidated is a query that passed. SQL is the parser's own canonical text
// (comments and odd quoting gone), and it is the only text that is ever executed.
type govSQLValidated struct {
	SQL     string
	View    *govSQLView
	Consts  []govSQLConst
	Tree    *pg.ParseResult
	Columns []string // view columns the query references
}

type govSQLWalk struct {
	nodes, maxDepth int
	err             *govSQLRejection
	rangeVars       []string
	cteNames        map[string]bool
	aliases         map[string]bool
	colRefs         []string
	consts          []govSQLConst
}

func (w *govSQLWalk) fail(code, format string, args ...any) {
	if w.err == nil {
		w.err = govSQLReject(code, format, args...)
	}
}

// govSQLValidate parses sql and applies every rule above. views are the views the
// catalogue issued for this question, by name.
func govSQLValidate(sql string, views map[string]*govSQLView) (*govSQLValidated, *govSQLRejection) {
	text := strings.TrimSpace(sql)
	if text == "" {
		return nil, govSQLReject("EMPTY", "the query is empty")
	}
	if len(text) > govSQLMaxChars {
		return nil, govSQLReject("TOO_LONG", "the query is longer than %d characters; write a shorter one", govSQLMaxChars)
	}
	if strings.ContainsRune(text, 0) {
		return nil, govSQLReject("BAD_TEXT", "the query contains a NUL character")
	}
	tree, err := pgq.Parse(text)
	if err != nil {
		return nil, govSQLReject("SYNTAX", "the query is not valid PostgreSQL: %s", govSQLClip(err.Error(), 160))
	}
	if len(tree.Stmts) != 1 {
		return nil, govSQLReject("ONE_STATEMENT", "exactly one SELECT statement is allowed, found %d", len(tree.Stmts))
	}
	root := tree.Stmts[0].GetStmt()
	if root == nil || root.GetSelectStmt() == nil {
		return nil, govSQLReject("SELECT_ONLY", "only a SELECT statement is allowed")
	}

	w := &govSQLWalk{cteNames: map[string]bool{}, aliases: map[string]bool{}}
	w.walk(tree.Stmts[0].ProtoReflect(), 0)
	if w.err != nil {
		return nil, w.err
	}

	// RELATIONS. Resolved after the walk because a WITH may be declared after the
	// field that uses it in the parse tree's field order.
	used := map[string]bool{}
	for _, name := range w.rangeVars {
		switch {
		case views[name] != nil:
			used[name] = true
		case w.cteNames[name]:
		default:
			return nil, govSQLReject("UNKNOWN_RELATION", "%q is not an available view; the only views are: %s", govSQLClip(name, 60), govSQLViewNames(views))
		}
	}
	for name := range w.cteNames {
		if views[name] != nil || strings.HasPrefix(name, "pg_") {
			return nil, govSQLReject("CTE_NAME", "a WITH name may not reuse a view name or start with pg_")
		}
	}
	if len(used) != 1 {
		return nil, govSQLReject("ONE_VIEW", "the query must read exactly one view (it reads %d); the views are: %s", len(used), govSQLViewNames(views))
	}
	var view *govSQLView
	for name := range used {
		view = views[name]
	}

	// COLUMNS. A name that is not a column of the view and not an alias the query
	// itself defines is rejected here, with the real list, so the correction does
	// not need a database round trip to learn it.
	valid := map[string]bool{}
	for _, c := range view.Columns {
		valid[c.Name] = true
	}
	for alias := range w.aliases {
		valid[alias] = true
	}
	referenced := map[string]bool{}
	for _, name := range w.colRefs {
		if !valid[name] {
			return nil, govSQLReject("UNKNOWN_COLUMN", "%q is not a column of %s; its columns are: %s", govSQLClip(name, 60), view.Name, view.columnNames(40))
		}
		for _, c := range view.Columns {
			if c.Name == name {
				referenced[name] = true
			}
		}
	}

	canonical, err := pgq.Deparse(tree)
	if err != nil || strings.TrimSpace(canonical) == "" {
		return nil, govSQLReject("DEPARSE", "the query could not be normalised")
	}
	cols := make([]string, 0, len(referenced))
	for name := range referenced {
		cols = append(cols, name)
	}
	sort.Strings(cols)
	return &govSQLValidated{SQL: strings.TrimSpace(canonical), View: view, Consts: w.consts, Tree: tree, Columns: cols}, nil
}

func (w *govSQLWalk) walk(m protoreflect.Message, depth int) {
	if w.err != nil {
		return
	}
	name := string(m.Descriptor().FullName())
	if !govSQLMessages[name] {
		w.fail("NOT_ALLOWED", "%s is not allowed; use only SELECT with filters, grouping, ordering and the listed functions", govSQLHumanName(name))
		return
	}
	w.nodes++
	if depth > w.maxDepth {
		w.maxDepth = depth
	}
	if w.nodes > govSQLMaxNodes || depth > govSQLMaxDepth {
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
func (w *govSQLWalk) check(m protoreflect.Message) {
	switch n := m.Interface().(type) {
	case *pg.SelectStmt:
		if n.IntoClause != nil {
			w.fail("NO_INTO", "SELECT INTO is not allowed")
		}
		if len(n.LockingClause) > 0 {
			w.fail("NO_LOCKING", "FOR UPDATE and FOR SHARE are not allowed")
		}
		if len(n.ValuesLists) > 0 {
			w.fail("NO_VALUES", "VALUES lists are not allowed")
		}
		if len(n.FromClause) > 1 {
			w.fail("NO_JOIN", "list one view only, with no comma joins")
		}
		if n.WithClause != nil && n.WithClause.Recursive {
			w.fail("NO_RECURSION", "recursive queries are not allowed")
		}
	case *pg.RangeVar:
		if n.Schemaname != "" || n.Catalogname != "" {
			w.fail("NO_SCHEMA", "name a view without a schema; schema-qualified names are not allowed")
			return
		}
		w.rangeVars = append(w.rangeVars, n.Relname)
	case *pg.CommonTableExpr:
		w.cteNames[n.Ctename] = true
		for _, c := range n.Aliascolnames {
			if s := c.GetString_(); s != nil {
				w.aliases[s.Sval] = true
			}
		}
	case *pg.ResTarget:
		if n.Name != "" {
			w.aliases[n.Name] = true
		}
	case *pg.Alias:
		for _, c := range n.Colnames {
			if s := c.GetString_(); s != nil {
				w.aliases[s.Sval] = true
			}
		}
	case *pg.ColumnRef:
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
	case *pg.FuncCall:
		w.checkFunc(n)
	case *pg.A_Expr:
		if len(n.Name) != 1 {
			w.fail("OPERATOR", "schema-qualified operators are not allowed")
			return
		}
		op := ""
		if s := n.Name[0].GetString_(); s != nil {
			op = s.Sval
		}
		if !govSQLOperators[strings.ToUpper(op)] {
			w.fail("OPERATOR", "the operator %q is not allowed", govSQLClip(op, 20))
		}
	case *pg.TypeCast:
		w.checkType(n.TypeName)
	case *pg.SubLink:
		kind := strings.TrimPrefix(n.SubLinkType.String(), "SUB_LINK_TYPE_")
		kind = strings.TrimPrefix(kind, "SubLinkType_")
		if !govSQLSubLinks[kind] && !govSQLSubLinks[n.SubLinkType.String()] {
			w.fail("SUBLINK", "that kind of subquery is not allowed")
		}
	case *pg.SQLValueFunction:
		if !govSQLValueFunctions[n.Op.String()] {
			w.fail("VALUE_FUNCTION", "that function (who is running the query) is not allowed")
		}
	case *pg.A_Const:
		switch {
		case n.GetBsval() != nil:
			w.fail("CONSTANT", "bit-string constants are not allowed")
		case n.GetSval() != nil:
			text := n.GetSval().Sval
			if len(text) > govSQLMaxConstChars {
				w.fail("CONSTANT", "a text constant is longer than %d characters", govSQLMaxConstChars)
				return
			}
			w.consts = append(w.consts, govSQLConst{Text: text})
		case n.GetIval() != nil:
			w.consts = append(w.consts, govSQLConst{Text: fmt.Sprint(n.GetIval().Ival), Numeric: true})
		case n.GetFval() != nil:
			w.consts = append(w.consts, govSQLConst{Text: n.GetFval().Fval, Numeric: true})
		}
	case *pg.TypeName:
		if len(n.ArrayBounds) > 0 {
			w.fail("TYPE", "array types are not allowed")
		}
	}
}

func (w *govSQLWalk) checkFunc(n *pg.FuncCall) {
	names := make([]string, 0, 2)
	for _, f := range n.Funcname {
		if s := f.GetString_(); s != nil {
			names = append(names, strings.ToLower(s.Sval))
		}
	}
	switch len(names) {
	case 1:
		if !govSQLFunctions[names[0]] {
			w.fail("FUNCTION", "the function %q is not allowed; allowed functions: %s", govSQLClip(names[0], 40), govSQLFunctionList())
		}
	case 2:
		if names[0] != "pg_catalog" || !govSQLPgCatalogForms[names[1]] {
			w.fail("FUNCTION", "schema-qualified function %q is not allowed", govSQLClip(strings.Join(names, "."), 60))
		}
	default:
		w.fail("FUNCTION", "that function name is not allowed")
	}
	if n.FuncVariadic {
		w.fail("FUNCTION", "VARIADIC calls are not allowed")
	}
}

func (w *govSQLWalk) checkType(t *pg.TypeName) {
	if t == nil {
		return
	}
	names := make([]string, 0, 2)
	for _, f := range t.Names {
		if s := f.GetString_(); s != nil {
			names = append(names, strings.ToLower(s.Sval))
		}
	}
	if len(names) == 0 || len(names) > 2 || (len(names) == 2 && names[0] != "pg_catalog") || !govSQLCastTypes[names[len(names)-1]] {
		w.fail("TYPE", "casting to %q is not allowed; allowed types: text, integer, bigint, numeric, double precision, boolean, date, time, timestamp, timestamptz, interval", govSQLClip(strings.Join(names, "."), 40))
	}
}

func govSQLClip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// govSQLHumanNames turns a rejected parse-tree type into words the model can act on.
var govSQLHumanNames = map[string]string{
	"InsertStmt": "INSERT", "UpdateStmt": "UPDATE", "DeleteStmt": "DELETE", "DropStmt": "DROP", "CreateStmt": "CREATE",
	"AlterTableStmt": "ALTER", "TruncateStmt": "TRUNCATE", "CopyStmt": "COPY", "VariableSetStmt": "SET", "VariableShowStmt": "SHOW",
	"ExplainStmt": "EXPLAIN", "JoinExpr": "a JOIN", "RangeFunction": "a function in FROM", "ParamRef": "a parameter",
	"IntoClause": "INTO", "LockingClause": "FOR UPDATE", "RangeTableSample": "TABLESAMPLE", "RowExpr": "a row constructor",
	"A_Indirection": "indexing", "CollateClause": "COLLATE", "GroupingSet": "GROUPING SETS", "NamedArgExpr": "a named argument",
}

func govSQLHumanName(full string) string {
	name := strings.TrimPrefix(full, "pg_query.")
	if human, ok := govSQLHumanNames[name]; ok {
		return human
	}
	return name
}

func govSQLViewNames(views map[string]*govSQLView) string {
	names := make([]string, 0, len(views))
	for name := range views {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func govSQLFunctionList() string {
	names := make([]string, 0, len(govSQLFunctions))
	for name := range govSQLFunctions {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
