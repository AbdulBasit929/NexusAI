package main

import (
	"fmt"
	"regexp"
	"strings"

	pg "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// GOVERNED SQL: A FILTER MADE OF THE EVIDENCE'S OWN NAME, AND A DIRECTION FROM A VERB (RUN 16, CLASSES W AND X).
//
// Two filters the model added that the question never asked for (fresh seed 6, pre-registration "Run 15 results"):
//
//	"Which is the most recent request time among the web requests?"
//	    SELECT max(event_time) ... WHERE path LIKE '/web%'      "web requests" is what the question calls the evidence;
//	                                                            nothing matches, and the answer was "no value"
//	"How many calls were made on weekends?"
//	    ... WHERE direction = 'OUTGOING' AND ...                the layer lists "made" as a word for OUTGOING; the
//	                                                            question counted calls, not outgoing calls (853 for 2,980)
//
// Both are refused once, with a message that says what is wrong, and then the lane abstains. Neither check asks a model
// anything. W reads the string literals of the query; X is a rule about which words of a question ask for a value.

// ---------------------------------------------------------------- W: a literal that is a word of the evidence's name

var govSQLRxLetterRun = regexp.MustCompile(`[A-Za-z]{2,}`)

// govSQLFamilyWords are the words the layer uses to name the evidence the view reads: its display name and synonyms
// ("system access logs", "web log", "request"), and the lane's own (governed_sql_vocabulary.go).
func govSQLFamilyWords(view *govSQLView) map[string]bool {
	out := map[string]bool{}
	for _, phrase := range append([]string{view.Display}, view.Synonyms...) {
		for _, w := range govSQLPhraseWords(phrase) {
			out[w] = true
		}
	}
	return out
}

// govSQLCheckLiterals refuses a string literal on a free-text column that is made only of words that name the
// evidence, when the question names no such column and does not quote the literal. A path "like '/web%'" for "web
// requests" is the evidence's name used as a filter; "the /login endpoint" names the column and is left alone.
func govSQLCheckLiterals(facts govSQLQuestionFacts, validated *govSQLValidated) []govSQLObligation {
	if validated == nil || validated.View == nil || validated.Tree == nil {
		return nil
	}
	view := validated.View
	family := govSQLFamilyWords(view)
	stems := govSQLStems(facts.Question)
	columns := map[string]govSQLColumn{}
	for _, c := range view.Columns {
		if c.Type == "text" && len(c.Values) == 0 && !c.Identifier {
			columns[c.Name] = c
		}
	}
	if len(columns) == 0 {
		return nil
	}
	named := func(c govSQLColumn) bool {
		for _, phrase := range append([]string{c.Display}, c.Synonyms...) {
			if govSQLPhraseIn(phrase, stems) {
				return true
			}
		}
		return false
	}
	var unmet []govSQLObligation
	seen := map[string]bool{}
	for _, cmp := range govSQLComparisons(validated.Tree) {
		column, ok := columns[cmp.Column]
		if !ok {
			continue
		}
		switch cmp.Op {
		case "=", "<>", "!=", "~~", "~~*", "!~~", "!~~*", "~", "~*", "!~", "!~*":
		default:
			continue
		}
		if named(column) {
			continue
		}
		for _, literal := range cmp.Consts {
			if seen[cmp.Column+"\x00"+literal] || strings.ContainsAny(literal, "0123456789") {
				continue
			}
			words := govSQLRxLetterRun.FindAllString(literal, -1)
			if len(words) == 0 {
				continue
			}
			allFamily := true
			for _, w := range words {
				w = strings.ToLower(w)
				if !family[w] && !family[govSQLSingular(w)] {
					allFamily = false
				}
			}
			if !allFamily || govSQLQuoted(facts, literal) {
				continue
			}
			seen[cmp.Column+"\x00"+literal] = true
			unmet = append(unmet, govSQLObligation{
				Kind: "LITERAL", Subject: literal,
				Message: fmt.Sprintf("The question names the evidence (%s) and says nothing about %s, so %s %s '%s' is a filter the question never asked for: the evidence's own name is not a value to look for. Remove that condition.", view.Display, column.Display, cmp.Column, govSQLOpWord(cmp.Op), literal),
				Reason:  fmt.Sprintf("the query filtered %s on %q, a word of the evidence's name that the question does not ask for", cmp.Column, literal),
			})
		}
	}
	return unmet
}

func govSQLOpWord(op string) string {
	switch op {
	case "~~", "~~*":
		return "LIKE"
	case "!~~", "!~~*":
		return "NOT LIKE"
	case "~", "~*":
		return "matches"
	case "!~", "!~*":
		return "does not match"
	}
	return op
}

// govSQLQuoted is true when the question carries the literal as written: inside quotes, or as an identifier.
func govSQLQuoted(facts govSQLQuestionFacts, literal string) bool {
	key := govSQLAlnum(literal)
	if key == "" {
		return false
	}
	for _, id := range facts.Identifiers {
		if govSQLAlnum(id) == key {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- X: a verb is not a value

// govSQLVerbValueWords are verbs that the layer also lists as words for a value: "made" for OUTGOING, "received" for
// INCOMING. "Calls made by 923001110001" asks for the outgoing ones; "calls were made on weekends" counts calls.
var govSQLVerbValueWords = map[string]bool{"made": true, "received": true}

// govSQLVerbPhrase is true for a phrase of the layer that is only such a verb.
func govSQLVerbPhrase(phrase string) bool {
	return govSQLVerbValueWords[strings.ToLower(strings.TrimSpace(phrase))]
}

// govSQLVerbOnlyValue says that the question reaches the layer's value only through such a verb, with no identifier
// for the verb to belong to: it is not a request for that value.
func govSQLVerbOnlyValue(question string, entity SemanticLayerEntityV1, fieldID, value string, hasIdentifier bool) bool {
	if hasIdentifier {
		return false
	}
	for _, field := range entity.Fields {
		if field.ID != fieldID {
			continue
		}
		for _, v := range field.Values {
			if !strings.EqualFold(v.Value, value) {
				continue
			}
			matched := 0
			for _, phrase := range append([]string{v.Value, v.DisplayName}, v.Synonyms...) {
				phrase = strings.TrimSpace(phrase)
				if phrase == "" {
					continue
				}
				if regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(phrase) + `\b`).MatchString(question) {
					if !govSQLVerbPhrase(phrase) {
						return false
					}
					matched++
				}
			}
			return matched > 0
		}
	}
	return false
}

// govSQLReadsTimeByFunction is true when a condition of the query reads a time through EXTRACT, date_part, date_trunc
// or to_char: a month named that way is not a pair of bounds, and the calendar check judges bounds only.
func govSQLReadsTimeByFunction(tree *pg.ParseResult) bool {
	found := false
	for _, root := range govSQLPredicateRoots(tree) {
		govSQLWalkTree(root.ProtoReflect(), func(msg protoreflect.Message) bool {
			if call, ok := msg.Interface().(*pg.FuncCall); ok {
				switch govSQLFuncName(call) {
				case "extract", "date_part", "date_trunc", "to_char":
					found = true
				}
			}
			return !found
		})
	}
	return found
}
