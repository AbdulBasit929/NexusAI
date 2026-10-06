package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	pg "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GOVERNED SQL: VERIFICATION, THE PART WHERE "ACCURATE" IS WON.
//
// A valid, executed query is not a correct answer. Measured on this product
// (CONTINUATION §7): a plan can be bound to the right table and verified and still
// answer 309 where the truth is 46, because "verification proves a plan against
// ITSELF, never against the question", and "an INVENTED filter is a symptom this
// system watches for; a MISSING one has none".
//
// So the lane reads the QUESTION first, deterministically, and writes down what the
// question demands; then it reads the QUERY'S parse tree and checks that each demand
// is visibly there. Anything unmet is repaired once (the model is told exactly what
// is missing) or the lane ABSTAINS and says which condition it could not apply.
// There is no third outcome in which the query is run and a number is shown anyway.
//
// What the question can demand, each from curated or syntactic evidence, never from
// a model and never from an embedding:
//
//	identifiers   a phone number, plate, IP, host, code or quoted string it names
//	values        a value the curated layer declares ("VoLTE" is cdr.call_type = 'VOLTE')
//	magnitude     "more than 10", via the existing questionStatesMagnitudeCondition
//	time          a month, year, date, range, "last week", a time of day
//	names         a capitalised name the layer does not know; the evidence has no name
//	              field, so it must be tied to something or the question is not answerable
//	shape         "how many" is ONE number; "which" is a list
type govSQLValueFact struct {
	View, Column, Value, Phrase string
}

type govSQLQuestionFacts struct {
	Identifiers  []string
	ValueFacts   []govSQLValueFact // strong matches: enforced
	WeakValues   []govSQLValueFact // everyday words the layer maps to a value: shown to the model, never enforced
	Magnitude    string
	Numbers      []string
	TimeCues     []string
	TimeOfDay    bool
	Names        []string
	WantsSingle  bool
	WithheldAttr string
	Fields       []govSQLFieldFact // columns the question names by a word only that column answers to
	NamedViews   map[string]bool   // evidence families the question names outright
	Question     string
}

var (
	govSQLRxISODate   = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`)
	govSQLRxTimeOfDay = regexp.MustCompile(`\b\d{1,2}:\d{2}(?::\d{2})?\b`)
	govSQLRxPhone     = regexp.MustCompile(`\+?\d[\d\s\-()]{5,}\d`)
	govSQLRxIPv4      = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	govSQLRxCode      = regexp.MustCompile(`\b[A-Za-z0-9]{2,}(?:-[A-Za-z0-9]+)+\b`)
	govSQLRxPlate     = regexp.MustCompile(`\b[A-Z]{2,3}\d{3,4}\b`)
	govSQLRxHost      = regexp.MustCompile(`\b[a-z0-9][a-z0-9\-]*(?:\.[a-z0-9\-]+)+\b`)
	govSQLRxQuoted    = regexp.MustCompile(`["“”]([^"“”]{2,60})["“”]|(?:^|\s)'([^']{2,60})'(?:\s|$|[?.,;:])`)
	govSQLRxYear      = regexp.MustCompile(`\b(?:19|20)\d{2}\b`)
	govSQLRxSlashDate = regexp.MustCompile(`\b\d{1,2}/\d{1,2}/\d{2,4}\b`)
	govSQLRxNumber    = regexp.MustCompile(`\d+(?:\.\d+)?`)
	govSQLRxWord      = regexp.MustCompile(`[A-Za-z][A-Za-z'’\-]*`)
	govSQLRxRelTime   = regexp.MustCompile(`(?i)\b(?:yesterday|today|tonight|last\s+(?:week|month|year|quarter|\d+\s+(?:days?|weeks?|months?|years?|hours?))|this\s+(?:week|month|year|morning|evening)|past\s+\d+\s+(?:days?|weeks?|months?|years?|hours?)|weekends?|weekdays?|before|after|since|until|between)\b`)
	govSQLRxDayName   = regexp.MustCompile(`(?i)\b(?:monday|tuesday|wednesday|thursday|friday|saturday|sunday)s?\b`)
	govSQLRxMonth     = regexp.MustCompile(`\b(?:January|February|March|April|June|July|August|September|October|November|December|Jan|Feb|Mar|Apr|Jun|Jul|Aug|Sept?|Oct|Nov|Dec)\b|(?i:\bmay\b)\s+\d|\bMay\b`)
	govSQLRxTimeWord  = regexp.MustCompile(`(?i)\b(?:night|nights|nighttime|overnight|midnight|morning|afternoon|evening|noon|dawn|dusk|\d{1,2}\s?(?:am|pm))\b`)
	govSQLRxSingle    = regexp.MustCompile(`(?i)\b(?:how\s+many|how\s+much|number\s+of|count\s+of|count\s+the|total|average|mean|sum|largest|smallest|biggest|highest|lowest|maximum|minimum|earliest|latest|first|last|oldest|newest|longest|shortest)\b`)
	govSQLRxGroupCue  = regexp.MustCompile(`(?i)\b(?:per|each|every|by\s+(?:type|day|month|year|hour|week|status|city|protocol|number)|breakdown|group|which|top\s+\d+|most|least|list|show|rank|distribution)\b`)
)

var govSQLStopWords = func() map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(`how what which who whom whose when where why is are was were do does did can could would should will
		the a an and or of in on at to for from by with without about between into over under per than that this these those there their
		show list tell give find count display get number total average sum maximum minimum earliest latest first last all any each every
		many much most least more less records record data case evidence calls call sessions session users user people numbers`) {
		out[w] = true
	}
	return out
}()

// govSQLVocabulary is every word the curated layer uses for anything: entity,
// field and value names and synonyms. A capitalised word NOT in it is a name.
func govSQLVocabulary(views []*govSQLView) map[string]bool {
	out := map[string]bool{}
	add := func(text string) {
		for _, w := range govSQLRxWord.FindAllString(strings.ToLower(text), -1) {
			out[strings.Trim(w, "-'’")] = true
		}
	}
	for _, view := range views {
		add(view.Display)
		add(view.Name)
		for _, s := range view.Synonyms {
			add(s)
		}
		for _, c := range view.Columns {
			add(c.Display)
			add(c.Name)
			for _, s := range c.Synonyms {
				add(s)
			}
			for _, v := range c.Values {
				add(v)
			}
		}
		for _, f := range view.entity.Fields {
			for _, v := range f.Values {
				add(v.Value)
				add(v.DisplayName)
				for _, s := range v.Synonyms {
					add(s)
				}
			}
		}
	}
	return out
}

// govSQLExtractFacts reads the question and writes down what it demands.
func govSQLExtractFacts(question string, views []*govSQLView, all []*govSQLView) govSQLQuestionFacts {
	facts := govSQLQuestionFacts{Question: question}
	facts.Fields = govSQLFieldFacts(question, views)
	facts.NamedViews = govSQLNamedViews(question, views)
	text := strings.TrimSpace(question)

	// Identifiers. Dates and clock times are removed first so "2026-04-02" and
	// "10:30" are not read as account numbers.
	scrub := govSQLRxISODate.ReplaceAllString(text, " ")
	scrub = govSQLRxTimeOfDay.ReplaceAllString(scrub, " ")
	seen := map[string]bool{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		key := govSQLAlnum(id)
		if id == "" || key == "" || seen[key] {
			return
		}
		seen[key] = true
		facts.Identifiers = append(facts.Identifiers, id)
	}
	for _, m := range govSQLRxPhone.FindAllString(scrub, -1) {
		if digits := govSQLDigits(m); len(digits) >= 7 {
			add(strings.TrimSpace(m))
		}
	}
	for _, m := range govSQLRxIPv4.FindAllString(scrub, -1) {
		add(m)
	}
	for _, m := range govSQLRxCode.FindAllString(scrub, -1) {
		if strings.ContainsAny(m, "0123456789") || m == strings.ToUpper(m) {
			add(m)
		}
	}
	for _, m := range govSQLRxPlate.FindAllString(scrub, -1) {
		add(m)
	}
	for _, m := range govSQLRxHost.FindAllString(scrub, -1) {
		if len(m) >= 5 && strings.IndexFunc(m, unicode.IsLetter) >= 0 {
			add(m)
		}
	}
	for _, m := range govSQLRxQuoted.FindAllStringSubmatch(scrub, -1) {
		if m[1] != "" {
			add(m[1])
		} else {
			add(m[2])
		}
	}

	// Values the curated layer declares, using the same matcher the compiler uses.
	// A match is STRONG when the question says the value as a code or says which
	// column it belongs to; a plain lower-case word that merely IS a synonym ("made",
	// "data", "text") is WEAK. Strong matches are enforced; weak ones only inform the
	// model, because enforcing "calls were made in April" as direction = OUTGOING
	// would refuse or retry questions that never asked for it.
	for _, view := range views {
		catalog := make([]FieldDescriptorV1, 0, len(view.Columns))
		for _, c := range view.Columns {
			catalog = append(catalog, c.field)
		}
		for _, filter := range semanticLayerValueFilters(text, view.entity, catalog) {
			column := ""
			for _, c := range view.Columns {
				if c.FieldID == filter.FieldID {
					column = c.Name
				}
			}
			if column == "" || strings.TrimSpace(filter.Value) == "" {
				continue
			}
			fact := govSQLValueFact{View: view.Name, Column: column, Value: filter.Value, Phrase: filter.Value}
			if govSQLValueIsStrong(text, view.entity, filter.FieldID, filter.Value) {
				facts.ValueFacts = append(facts.ValueFacts, fact)
			} else {
				facts.WeakValues = append(facts.WeakValues, fact)
			}
		}
	}

	// A stated magnitude, with the numbers it names.
	if phrase := questionStatesMagnitudeCondition(text); phrase != "" {
		facts.Magnitude = phrase
		for _, n := range govSQLRxNumber.FindAllString(phrase, -1) {
			facts.Numbers = append(facts.Numbers, n)
		}
	}

	// Time. A constraint cue is a month, a year, a date, a range word, a relative
	// period or a time of day; a bare "month" in "per month" is not one.
	for _, rx := range []*regexp.Regexp{govSQLRxMonth, govSQLRxYear, govSQLRxSlashDate, govSQLRxRelTime, govSQLRxDayName} {
		for _, m := range rx.FindAllString(text, -1) {
			facts.TimeCues = append(facts.TimeCues, strings.TrimSpace(m))
		}
	}
	if govSQLRxISODate.MatchString(text) {
		facts.TimeCues = append(facts.TimeCues, govSQLRxISODate.FindString(text))
	}
	if govSQLRxTimeOfDay.MatchString(scrubDates(text)) || govSQLRxTimeWord.MatchString(text) {
		facts.TimeOfDay = true
		facts.TimeCues = append(facts.TimeCues, strings.TrimSpace(govSQLFirstNonEmpty(govSQLRxTimeWord.FindString(text), govSQLRxTimeOfDay.FindString(scrubDates(text)))))
	}

	// Names the layer does not know.
	vocabulary := govSQLVocabulary(all)
	facts.Names = govSQLUnknownNames(text, vocabulary, seen)

	// Shape.
	facts.WantsSingle = govSQLRxSingle.MatchString(text) && !govSQLRxGroupCue.MatchString(text)

	// An attribute the evidence withholds (a person's name, a national ID): the
	// curated layer says which fields are PII or RESTRICTED and what they are called.
	stems := _semanticLayerQuestionTokens(text)
	for _, view := range views {
		for _, field := range view.entity.Fields {
			if field.Sensitivity != "PII" && field.Sensitivity != "RESTRICTED" {
				continue
			}
			for _, phrase := range append([]string{field.DisplayName}, field.Synonyms...) {
				if _semanticLayerPhraseInQuestion(phrase, stems) {
					facts.WithheldAttr = phrase
				}
			}
		}
	}
	return facts
}

// govSQLValueIsStrong decides whether the question really names a declared value.
func govSQLValueIsStrong(question string, entity SemanticLayerEntityV1, fieldID, canonical string) bool {
	var field SemanticLayerFieldV1
	var value SemanticLayerValueV1
	for _, f := range entity.Fields {
		if f.ID != fieldID {
			continue
		}
		field = f
		for _, v := range f.Values {
			if v.Value == canonical {
				value = v
			}
		}
	}
	if value.Value == "" {
		return false
	}
	// The generic words that name "which column" in an analyst's sentence, and the
	// column's own display name and synonyms taken as WHOLE PHRASES: the word "call"
	// alone is not evidence of cdr.call_type, but "call direction" is evidence of
	// cdr.direction.
	columnWords := map[string]bool{"type": true, "kind": true, "status": true, "direction": true, "code": true, "state": true, "protocol": true, "method": true, "class": true}
	columnPhrases := []*regexp.Regexp{}
	for _, part := range append([]string{field.DisplayName}, field.Synonyms...) {
		part = strings.ToLower(strings.TrimSpace(part))
		if strings.Contains(part, " ") {
			columnPhrases = append(columnPhrases, regexp.MustCompile(`\b`+regexp.QuoteMeta(part)+`\b`))
		}
	}
	for _, phrase := range append([]string{value.Value}, value.Synonyms...) {
		phrase = strings.TrimSpace(phrase)
		if phrase == "" || _semanticLayerValueEchoesFieldName(field, phrase) {
			continue
		}
		rx := regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9])(` + regexp.QuoteMeta(phrase) + `)(?:$|[^A-Za-z0-9])`)
		for _, loc := range rx.FindAllStringSubmatchIndex(question, -1) {
			start, end := loc[2], loc[3]
			original := question[start:end]
			switch {
			case len(strings.Fields(phrase)) >= 2:
				return true
			case govSQLRxNumber.MatchString(original) && govSQLDigits(original) == strings.TrimSpace(original):
				return true
			case govSQLWrittenAsCode(original):
				return true
			}
			// Named by its column: "call direction is incoming", "status 403".
			before := strings.Fields(strings.ToLower(question[:start]))
			if len(before) > 5 {
				before = before[len(before)-5:]
			}
			window := strings.Join(before, " ")
			for _, w := range before {
				if columnWords[strings.Trim(w, "?,.;:")] {
					return true
				}
			}
			for _, rx := range columnPhrases {
				if rx.MatchString(window) {
					return true
				}
			}
		}
	}
	return false
}

// govSQLWrittenAsCode: "VoLTE", "SMS", "INCOMING" are codes; "voice" and "Data" are words.
func govSQLWrittenAsCode(token string) bool {
	runes := []rune(token)
	if len(runes) < 2 {
		return false
	}
	for _, r := range runes[1:] {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func scrubDates(text string) string { return govSQLRxISODate.ReplaceAllString(text, " ") }

func govSQLFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// govSQLUnknownNames finds runs of capitalised words that are not the first word of
// the question, not an acronym, not a month or day, not part of an identifier, and
// not a word the curated layer uses.
func govSQLUnknownNames(text string, vocabulary map[string]bool, identifiers map[string]bool) []string {
	locs := govSQLRxWord.FindAllStringIndex(text, -1)
	names := []string{}
	run := []string{}
	flush := func() {
		if len(run) > 0 {
			names = append(names, strings.Join(run, " "))
			run = run[:0]
		}
	}
	for i, loc := range locs {
		word := text[loc[0]:loc[1]]
		lower := strings.ToLower(strings.Trim(word, "-'’"))
		runes := []rune(word)
		capitalised := unicode.IsUpper(runes[0]) && strings.IndexFunc(string(runes[1:]), unicode.IsLower) >= 0
		firstWord := i == 0 || govSQLStartsSentence(text, loc[0])
		switch {
		case !capitalised, firstWord, vocabulary[lower], govSQLStopWords[lower], govSQLIsCalendarWord(lower), identifiers[govSQLAlnum(word)]:
			flush()
		default:
			run = append(run, word)
		}
	}
	flush()
	return names
}

// govSQLStartsSentence reports whether the word at index starts a sentence, where a
// capital letter says nothing about whether the word is a name.
func govSQLStartsSentence(text string, index int) bool {
	before := strings.TrimRightFunc(text[:index], unicode.IsSpace)
	if before == "" {
		return true
	}
	switch before[len(before)-1] {
	case '.', '?', '!', ':':
		return true
	}
	return false
}

func govSQLIsCalendarWord(lower string) bool {
	switch lower {
	case "january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december",
		"jan", "feb", "mar", "apr", "jun", "jul", "aug", "sep", "sept", "oct", "nov", "dec",
		"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday", "i", "ok", "okay", "pakistan", "english", "urdu":
		return true
	}
	return false
}

func govSQLAlnum(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func govSQLDigits(text string) string {
	var b strings.Builder
	for _, r := range text {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ---------------------------------------------------------------- reading the query

// govSQLComparison is "this column is compared with these constants".
type govSQLComparison struct {
	Column string
	Consts []string
	Op     string
}

func govSQLWalkTree(root protoreflect.Message, visit func(msg protoreflect.Message) bool) {
	if !visit(root) {
		return
	}
	root.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
		case fd.IsList() && fd.Message() != nil:
			list := v.List()
			for i := 0; i < list.Len(); i++ {
				govSQLWalkTree(list.Get(i).Message(), visit)
			}
		case fd.Message() != nil:
			govSQLWalkTree(v.Message(), visit)
		}
		return true
	})
}

func govSQLColumnOf(node *pg.Node) string {
	if node == nil {
		return ""
	}
	switch {
	case node.GetColumnRef() != nil:
		fields := node.GetColumnRef().Fields
		if len(fields) > 0 {
			if s := fields[len(fields)-1].GetString_(); s != nil {
				return s.Sval
			}
		}
	case node.GetTypeCast() != nil:
		return govSQLColumnOf(node.GetTypeCast().Arg)
	case node.GetFuncCall() != nil:
		for _, arg := range node.GetFuncCall().Args {
			if name := govSQLColumnOf(arg); name != "" {
				return name
			}
		}
	}
	return ""
}

func govSQLConstsOf(node *pg.Node) []string {
	if node == nil {
		return nil
	}
	var out []string
	var walk func(n *pg.Node)
	walk = func(n *pg.Node) {
		if n == nil {
			return
		}
		switch {
		case n.GetAConst() != nil:
			c := n.GetAConst()
			switch {
			case c.GetSval() != nil:
				out = append(out, c.GetSval().Sval)
			case c.GetIval() != nil:
				out = append(out, fmt.Sprint(c.GetIval().Ival))
			case c.GetFval() != nil:
				out = append(out, c.GetFval().Fval)
			}
		case n.GetTypeCast() != nil:
			walk(n.GetTypeCast().Arg)
		case n.GetList() != nil:
			for _, item := range n.GetList().Items {
				walk(item)
			}
		case n.GetAArrayExpr() != nil:
			for _, item := range n.GetAArrayExpr().Elements {
				walk(item)
			}
		}
	}
	walk(node)
	return out
}

// govSQLComparisons lists every column-versus-constant comparison in the query.
func govSQLComparisons(tree *pg.ParseResult) []govSQLComparison {
	var out []govSQLComparison
	for _, stmt := range tree.Stmts {
		govSQLWalkTree(stmt.ProtoReflect(), func(msg protoreflect.Message) bool {
			expr, ok := msg.Interface().(*pg.A_Expr)
			if !ok {
				return true
			}
			op := ""
			if len(expr.Name) == 1 && expr.Name[0].GetString_() != nil {
				op = expr.Name[0].GetString_().Sval
			}
			if col, consts := govSQLColumnOf(expr.Lexpr), govSQLConstsOf(expr.Rexpr); col != "" && len(consts) > 0 {
				out = append(out, govSQLComparison{Column: col, Consts: consts, Op: op})
			}
			if col, consts := govSQLColumnOf(expr.Rexpr), govSQLConstsOf(expr.Lexpr); col != "" && len(consts) > 0 {
				out = append(out, govSQLComparison{Column: col, Consts: consts, Op: op})
			}
			return true
		})
	}
	return out
}

// govSQLTimeConstraint reports whether a time column appears in a WHERE or HAVING,
// and whether a time-of-day construct (EXTRACT(HOUR ...), a clock-time literal)
// does. Looking only at predicates is the point: MIN(call_start) in a SELECT list
// asks for a time, it does not constrain one.
func govSQLTimeConstraint(tree *pg.ParseResult, view *govSQLView) (onTimeColumn, timeOfDay bool) {
	timeColumns := map[string]bool{}
	for _, c := range view.Columns {
		if c.Type == "timestamptz" || c.Type == "date" {
			timeColumns[c.Name] = true
		}
	}
	scan := func(node *pg.Node) {
		if node == nil {
			return
		}
		govSQLWalkTree(node.ProtoReflect(), func(msg protoreflect.Message) bool {
			switch n := msg.Interface().(type) {
			case *pg.ColumnRef:
				if len(n.Fields) > 0 {
					if s := n.Fields[len(n.Fields)-1].GetString_(); s != nil && timeColumns[s.Sval] {
						onTimeColumn = true
					}
				}
			case *pg.FuncCall:
				names := []string{}
				for _, f := range n.Funcname {
					if s := f.GetString_(); s != nil {
						names = append(names, strings.ToLower(s.Sval))
					}
				}
				switch names[len(names)-1] {
				case "extract", "date_part", "to_char", "timezone":
					timeOfDay = true
				}
			case *pg.TypeName:
				for _, f := range n.Names {
					if s := f.GetString_(); s != nil && strings.EqualFold(s.Sval, "time") {
						timeOfDay = true
					}
				}
			case *pg.A_Const:
				if s := n.GetSval(); s != nil && govSQLRxTimeOfDay.MatchString(s.Sval) {
					timeOfDay = true
				}
			}
			return true
		})
	}
	for _, stmt := range tree.Stmts {
		govSQLWalkTree(stmt.ProtoReflect(), func(msg protoreflect.Message) bool {
			if sel, ok := msg.Interface().(*pg.SelectStmt); ok {
				scan(sel.WhereClause)
				scan(sel.HavingClause)
			}
			return true
		})
	}
	return onTimeColumn, timeOfDay
}

// ---------------------------------------------------------------- the checks

// govSQLObligation is one thing the question demands that the query lacks.
type govSQLObligation struct {
	Kind    string // IDENTIFIER, VALUE, MAGNITUDE, TIME, TIME_OF_DAY, NAME, WITHHELD
	Subject string
	Message string // written for the model
	Reason  string // written for the analyst, when the lane abstains
}

// govSQLCheck reads the query against what the question demands and returns every
// unmet demand. views are the views the model was offered.
func govSQLCheck(facts govSQLQuestionFacts, validated *govSQLValidated) []govSQLObligation {
	var unmet []govSQLObligation
	view := validated.View
	comparisons := govSQLComparisons(validated.Tree)

	identifierColumns := []string{}
	for _, c := range view.Columns {
		if c.Identifier && c.Type == "text" {
			identifierColumns = append(identifierColumns, c.Name)
		}
	}
	isIdentifierColumn := map[string]bool{}
	for _, name := range identifierColumns {
		isIdentifierColumn[name] = true
	}

	// IDENTIFIERS must appear as a constant compared with a column.
	for _, id := range facts.Identifiers {
		if !govSQLAppliedTo(id, comparisons, nil) {
			where := "a column of the view"
			if len(identifierColumns) > 0 {
				where = "one or more of: " + strings.Join(identifierColumns, ", ")
			}
			unmet = append(unmet, govSQLObligation{
				Kind: "IDENTIFIER", Subject: id,
				Message: fmt.Sprintf("The question names %q but the query never compares a column with it. Add it as a text literal on %s (a number that can be either party needs an OR across those columns).", id, where),
				Reason:  fmt.Sprintf("the query did not apply %q", id),
			})
		}
	}

	// VALUES the curated layer declares.
	for _, fact := range facts.ValueFacts {
		if fact.View != view.Name {
			continue
		}
		found := false
		for _, cmp := range comparisons {
			if cmp.Column != fact.Column {
				continue
			}
			for _, c := range cmp.Consts {
				if strings.Contains(strings.ToLower(c), strings.ToLower(fact.Value)) {
					found = true
				}
			}
		}
		if !found {
			unmet = append(unmet, govSQLObligation{
				Kind: "VALUE", Subject: fact.Value,
				Message: fmt.Sprintf("The question mentions the value %q, which is a value of %s. Filter on it exactly: %s = '%s'.", fact.Value, fact.Column, fact.Column, fact.Value),
				Reason:  fmt.Sprintf("the query did not filter %s on %q", fact.Column, fact.Value),
			})
		}
	}

	// MAGNITUDE: a comparison with the number the question states.
	if facts.Magnitude != "" {
		numeric := map[string]bool{}
		for _, c := range validated.Consts {
			if c.Numeric {
				numeric[strings.TrimRight(strings.TrimRight(c.Text, "0"), ".")] = true
				numeric[c.Text] = true
			}
		}
		hasNumber := len(facts.Numbers) == 0
		for _, n := range facts.Numbers {
			if numeric[strings.TrimRight(strings.TrimRight(n, "0"), ".")] || numeric[n] {
				hasNumber = true
			}
		}
		if !hasNumber || !govSQLHasRangeOperator(validated.Tree) {
			unmet = append(unmet, govSQLObligation{
				Kind: "MAGNITUDE", Subject: facts.Magnitude,
				Message: fmt.Sprintf("The question states a condition (%q). The query must compare the matching numeric column or an aggregate with %s using > or < (not =).", facts.Magnitude, strings.Join(facts.Numbers, " / ")),
				Reason:  fmt.Sprintf("the query did not apply the condition %q", facts.Magnitude),
			})
		}
	}

	// TIME.
	if len(facts.TimeCues) > 0 {
		onTime, timeOfDay := govSQLTimeConstraint(validated.Tree, view)
		if !onTime {
			unmet = append(unmet, govSQLObligation{
				Kind: "TIME", Subject: strings.Join(facts.TimeCues, ", "),
				Message: fmt.Sprintf("The question restricts time (%s). Add a WHERE condition on a time column (%s).", strings.Join(facts.TimeCues, ", "), govSQLTimeColumns(view)),
				Reason:  fmt.Sprintf("the query did not apply the time condition (%s)", strings.Join(facts.TimeCues, ", ")),
			})
		} else if facts.TimeOfDay && !timeOfDay {
			unmet = append(unmet, govSQLObligation{
				Kind: "TIME_OF_DAY", Subject: strings.Join(facts.TimeCues, ", "),
				Message: "The question names a time of day. Filter on the hour with EXTRACT(HOUR FROM column) (night is hours 0 to 5).",
				Reason:  "the query did not apply the time-of-day condition",
			})
		}
	}

	// NAMES: a capitalised name must be compared with a column that is not an
	// identifier column, or it is unbound. The evidence holds no personal-name field.
	for _, name := range facts.Names {
		bound := false
		for _, cmp := range comparisons {
			if isIdentifierColumn[cmp.Column] {
				continue
			}
			for _, c := range cmp.Consts {
				if strings.Contains(govSQLAlnum(c), govSQLAlnum(name)) || strings.Contains(govSQLAlnum(name), govSQLAlnum(c)) && len(govSQLAlnum(c)) >= 3 {
					bound = true
				}
			}
		}
		if !bound {
			unmet = append(unmet, govSQLObligation{
				Kind: "NAME", Subject: name,
				Message: fmt.Sprintf("The question names %q, which is not a value of any column. Do not invent a filter for it: set answerable to false.", name),
				Reason:  fmt.Sprintf("I could not tie %q to any field: these records hold numbers, plates, addresses and codes, not personal names", name),
			})
		}
	}
	unmet = append(unmet, govSQLCheckView(facts, validated)...)
	unmet = append(unmet, govSQLCheckGrouping(facts.Question, []*govSQLView{validated.View})...)
	unmet = append(unmet, govSQLCheckFields(facts, validated)...)
	unmet = append(unmet, govSQLCheckFilters(facts.Question, facts, validated)...)
	return unmet
}

func govSQLTimeColumns(view *govSQLView) string {
	names := []string{}
	for _, c := range view.Columns {
		if c.Type == "timestamptz" || c.Type == "date" {
			names = append(names, c.Name)
		}
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// govSQLAppliedTo reports whether any comparison's constant corresponds to the
// identifier. A phone number matches on its last ten digits so 0300... and 92300...
// are the same number; anything else matches on its letters and digits.
func govSQLAppliedTo(id string, comparisons []govSQLComparison, columns map[string]bool) bool {
	idDigits, idKey := govSQLDigits(id), govSQLAlnum(id)
	for _, cmp := range comparisons {
		if columns != nil && !columns[cmp.Column] {
			continue
		}
		for _, c := range cmp.Consts {
			cKey, cDigits := govSQLAlnum(c), govSQLDigits(c)
			if cKey == "" {
				continue
			}
			if strings.Contains(cKey, idKey) || strings.Contains(idKey, cKey) && len(cKey) >= 4 {
				return true
			}
			if len(idDigits) >= 7 && len(cDigits) >= 7 {
				a, b := idDigits, cDigits
				if len(a) > 10 {
					a = a[len(a)-10:]
				}
				if len(b) > 10 {
					b = b[len(b)-10:]
				}
				if a == b {
					return true
				}
			}
		}
	}
	return false
}

func govSQLHasRangeOperator(tree *pg.ParseResult) bool {
	found := false
	for _, stmt := range tree.Stmts {
		govSQLWalkTree(stmt.ProtoReflect(), func(msg protoreflect.Message) bool {
			if expr, ok := msg.Interface().(*pg.A_Expr); ok && len(expr.Name) == 1 && expr.Name[0].GetString_() != nil {
				switch strings.ToUpper(expr.Name[0].GetString_().Sval) {
				case ">", ">=", "<", "<=", "BETWEEN", "NOT BETWEEN":
					found = true
				}
			}
			return true
		})
	}
	return found
}

// govSQLCheckShape compares the result with what the question asked for. "How many"
// and "the average" are ONE value; a result of many rows means the query did not
// aggregate over everything that matched.
func govSQLCheckShape(facts govSQLQuestionFacts, result *govSQLResult) *govSQLObligation {
	if facts.WantsSingle && len(result.Rows) > 1 {
		return &govSQLObligation{
			Kind: "SHAPE", Subject: "single value",
			Message: fmt.Sprintf("The question asks for one value but the query returned %d rows. Aggregate over all matching rows with no GROUP BY.", len(result.Rows)),
			Reason:  "the query returned a list where one value was asked for",
		}
	}
	return nil
}
