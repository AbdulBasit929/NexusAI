package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// GOVERNED SQL: THE COLUMNS AND VALUES A QUESTION NAMES, AND THE ONES IT DOES NOT.
//
// Measured 2026-10-06 (arm C of the lane's first run): of the five answers the lane
// added that were wrong, three were the model reading the schema loosely.
//
//	"smallest position"       -> min(longitude); the layer declares "position" a name of latitude
//	"most recent call start"  -> WHERE call_type = 'CALL'; the question names no call type and
//	                             'CALL' is not a value the layer declares for it
//	"first activation date"   -> answered from the plate view, the label copied from the question
//
// Two checks, both read from the curated layer and neither a template:
//
//	FIELD     a column the question names by a word only that column answers to must be
//	          used by the query;
//	FILTER    an equality filter on an ENUMERATED column must name a value the layer declares
//	          AND the question must say it (or a word the layer maps to it). A filter the
//	          question never asked for changes the question.
type govSQLFieldFact struct {
	View, Column, Phrase string
}

// govSQLGenericWords are too common in questions to say "this column": they are names
// of many columns, or of none in particular.
var govSQLGenericWords = map[string]bool{
	"time": true, "date": true, "when": true, "period": true, "month": true, "year": true, "number": true, "numbers": true, "type": true,
	"kind": true, "count": true, "total": true, "name": true, "value": true, "id": true, "record": true, "records": true, "call": true,
	"calls": true, "data": true, "event": true, "events": true, "start": true, "end": true, "timestamp": true, "status": true, "code": true,
}

var govSQLProvenanceColumns = map[string]bool{"record_id": true, "source_file": true, "row_number": true, "artifact_id": true, "evidence_id": true}

// govSQLSingular folds a plural so "plate reads" finds the curated phrase "plate read".
func govSQLSingular(word string) string {
	switch {
	case len(word) > 4 && strings.HasSuffix(word, "ies"):
		return word[:len(word)-3] + "y"
	case len(word) > 3 && strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") && !strings.HasSuffix(word, "us") && !strings.HasSuffix(word, "is"):
		return word[:len(word)-1]
	}
	return word
}

// govSQLPhraseWords are the words of a phrase, lower-cased and singular.
func govSQLPhraseWords(phrase string) []string {
	words := semanticWords.FindAllString(strings.ToLower(strings.TrimSpace(phrase)), -1)
	for i, w := range words {
		words[i] = govSQLSingular(w)
	}
	return words
}

// govSQLStems is the set of singular words of a question.
func govSQLStems(question string) map[string]bool {
	out := map[string]bool{}
	for _, w := range govSQLPhraseWords(question) {
		out[w] = true
	}
	return out
}

// govSQLPhraseIn is true when every word of the phrase is in the question. A one-word phrase of
// fewer than three letters never matches ("in", "no").
func govSQLPhraseIn(phrase string, stems map[string]bool) bool {
	words := govSQLPhraseWords(phrase)
	if len(words) == 0 || (len(words) == 1 && len(words[0]) < 3) {
		return false
	}
	for _, w := range words {
		if !stems[w] {
			return false
		}
	}
	return true
}

// govSQLFieldFacts finds the columns of the offered views that the question names by
// a phrase no other column of that view answers to.
func govSQLFieldFacts(question string, views []*govSQLView) []govSQLFieldFact {
	stems := govSQLStems(question)
	sequence := " " + strings.Join(govSQLPhraseWords(question), " ") + " "
	var out []govSQLFieldFact
	for _, view := range views {
		entity := map[string]bool{}
		for _, phrase := range append([]string{view.Display}, view.Synonyms...) {
			for _, w := range govSQLPhraseWords(phrase) {
				entity[w] = true
			}
		}
		owners := map[string]map[string]bool{} // phrase -> columns that answer to it
		for _, column := range view.Columns {
			if govSQLProvenanceColumns[column.Name] {
				continue
			}
			for _, phrase := range append([]string{column.Display}, column.Synonyms...) {
				key := strings.Join(govSQLPhraseWords(phrase), " ")
				if key == "" {
					continue
				}
				if owners[key] == nil {
					owners[key] = map[string]bool{}
				}
				owners[key][column.Name] = true
			}
		}
		type match struct {
			column, phrase string
			words          []string
		}
		var matches []match
		for key, cols := range owners {
			if len(cols) != 1 || !govSQLPhraseIn(key, stems) {
				continue
			}
			// The words must stand together in the question: "which" and "tower" somewhere in a
			// sentence about an address do not say "which tower".
			if !strings.Contains(sequence, " "+key+" ") {
				continue
			}
			words := strings.Fields(key)
			if len(words) == 1 && (govSQLGenericWords[words[0]] || govSQLStopWords[words[0]] || len(words[0]) < 5) {
				continue
			}
			allEntity := true
			for _, w := range words {
				if !entity[w] {
					allEntity = false
				}
			}
			if allEntity {
				continue
			}
			for column := range cols {
				matches = append(matches, match{column, key, words})
			}
		}
		// The longest name wins: "call start time" is the call-start column, and the
		// shorter "time" of another column does not also become a demand.
		subset := func(a, b []string) bool {
			if len(a) >= len(b) {
				return false
			}
			set := map[string]bool{}
			for _, w := range b {
				set[w] = true
			}
			for _, w := range a {
				if !set[w] {
					return false
				}
			}
			return true
		}
		for _, m := range matches {
			dominated := false
			for _, other := range matches {
				if other.column != m.column && subset(m.words, other.words) {
					dominated = true
				}
			}
			if !dominated {
				out = append(out, govSQLFieldFact{View: view.Name, Column: m.column, Phrase: m.phrase})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].View != out[j].View {
			return out[i].View < out[j].View
		}
		return out[i].Column < out[j].Column
	})
	return out
}

func govSQLCheckFields(facts govSQLQuestionFacts, validated *govSQLValidated) []govSQLObligation {
	view := validated.View
	used := map[string]bool{}
	for _, name := range validated.Columns {
		used[name] = true
	}
	var unmet []govSQLObligation
	for _, fact := range facts.Fields {
		if fact.View != view.Name || used[fact.Column] {
			continue
		}
		unmet = append(unmet, govSQLObligation{
			Kind: "FIELD", Subject: fact.Column,
			Message: fmt.Sprintf("The question says %q, which is the column %s of %s. The query must use %s.", fact.Phrase, fact.Column, view.Name, fact.Column),
			Reason:  fmt.Sprintf("the query did not use the field the question names (%s)", fact.Phrase),
		})
	}
	return unmet
}

// govSQLCheckFilters refuses a filter on an enumerated column that the question did not ask for,
// or that names a value the layer does not declare. "=", "<>", IN, NOT IN and LIKE all count:
// "status NOT IN ('200','201')" and "status LIKE '4%'" are filters too (measured 2026-10-07).
func govSQLCheckFilters(question string, facts govSQLQuestionFacts, validated *govSQLValidated) []govSQLObligation {
	view := validated.View
	enumerated := map[string][]string{}
	for _, column := range view.Columns {
		if len(column.Values) > 0 {
			enumerated[column.Name] = column.Values
		}
	}
	// The phrases the layer gives each declared value: its code, its display name, its synonyms.
	valuePhrases := map[string][]string{}
	valueKeys := map[string]bool{}
	for _, column := range view.Columns {
		for _, field := range view.entity.Fields {
			if field.ID != column.FieldID {
				continue
			}
			for _, v := range field.Values {
				key := column.Name + "\x00" + strings.ToLower(v.Value)
				phrases := append([]string{v.Value, v.DisplayName}, v.Synonyms...)
				valuePhrases[key] = phrases
				for _, phrase := range phrases {
					valueKeys[strings.Join(govSQLPhraseWords(phrase), " ")] = true
				}
			}
		}
	}
	stems := govSQLStems(question)
	// Words spent on naming something else are not also a request for a value: the words of a
	// multi-word column name ("call start time"), and of the evidence's own name ("call records",
	// where "call" is also a value of call_type). A phrase the layer itself gives to a value
	// ("server error") is never spent.
	spend := func(phrase string) {
		words := govSQLPhraseWords(phrase)
		if len(words) == 0 || !govSQLPhraseIn(phrase, stems) || valueKeys[strings.Join(words, " ")] {
			return
		}
		for _, w := range words {
			delete(stems, w)
		}
	}
	for _, column := range view.Columns {
		for _, phrase := range append([]string{column.Display}, column.Synonyms...) {
			if len(govSQLPhraseWords(phrase)) > 1 {
				spend(phrase)
			}
		}
	}
	for _, phrase := range append([]string{view.Display}, view.Synonyms...) {
		spend(phrase)
	}
	grounded := func(column, value string) bool {
		for _, phrase := range valuePhrases[column+"\x00"+strings.ToLower(value)] {
			if govSQLPhraseIn(phrase, stems) {
				return true
			}
		}
		for _, v := range append(append([]govSQLValueFact{}, facts.ValueFacts...), facts.WeakValues...) {
			if v.View == view.Name && v.Column == column && strings.EqualFold(v.Value, value) && (v.Phrase == "" || govSQLPhraseIn(v.Phrase, stems)) {
				return true
			}
		}
		return false
	}
	// A value the layer does not declare is only acceptable when the question itself carries
	// it as a literal (an identifier or a number); an ordinary word of the question is not enough.
	named := func(value string) bool {
		for _, id := range facts.Identifiers {
			if strings.EqualFold(id, value) {
				return true
			}
		}
		for _, n := range facts.Numbers {
			if n == value {
				return true
			}
		}
		return false
	}
	says := func(value string) bool { return govSQLPhraseIn(value, stems) }
	filterOps := map[string]bool{"=": true, "<>": true, "!=": true, "~~": true, "~~*": true, "!~~": true, "!~~*": true}
	var unmet []govSQLObligation
	for _, cmp := range govSQLComparisons(validated.Tree) {
		declared, ok := enumerated[cmp.Column]
		if !ok || !filterOps[cmp.Op] {
			continue
		}
		for _, c := range cmp.Consts {
			isDeclared := false
			for _, d := range declared {
				if strings.EqualFold(d, c) {
					isDeclared = true
				}
			}
			switch {
			case !isDeclared && !named(c) && !named(strings.Trim(c, "%")):
				unmet = append(unmet, govSQLObligation{
					Kind: "FILTER", Subject: c,
					Message: fmt.Sprintf("%s %s '%s' is not a value of %s (its values are %s) and the question does not say it. Remove that filter.", cmp.Column, cmp.Op, c, cmp.Column, strings.Join(declared, ", ")),
					Reason:  fmt.Sprintf("the query filtered %s on %q, which the question does not ask for", cmp.Column, c),
				})
			case isDeclared && !grounded(cmp.Column, c) && !says(c):
				unmet = append(unmet, govSQLObligation{
					Kind: "FILTER", Subject: c,
					Message: fmt.Sprintf("The question does not ask for %s %s '%s'. Remove that filter: count everything unless the question restricts it.", cmp.Column, cmp.Op, c),
					Reason:  fmt.Sprintf("the query filtered %s on %q, which the question does not ask for", cmp.Column, c),
				})
			}
		}
	}
	return unmet
}

// govSQLNamedViews are the views whose own name the question uses ("data sessions" is
// v_ipdr). When the question names an evidence family and the query reads a different
// one, the answer is about something else (measured 2026-10-06: "highest data volume in
// the data sessions" was answered from the call records). The longest name wins:
// "plate read" names the plate-read observations, and the shorter "plate" of the
// camera sightings does not also name them.
func govSQLNamedViews(question string, views []*govSQLView) map[string]bool {
	sequence := " " + strings.Join(govSQLPhraseWords(question), " ") + " "
	type match struct {
		view  string
		words []string
	}
	var matches []match
	for _, view := range views {
		for _, phrase := range append([]string{view.Display}, view.Synonyms...) {
			words := govSQLPhraseWords(phrase)
			key := strings.Join(words, " ")
			if key != "" && strings.Contains(sequence, " "+key+" ") {
				matches = append(matches, match{view.Name, words})
			}
		}
	}
	subset := func(a, b []string) bool {
		if len(a) >= len(b) {
			return false
		}
		set := map[string]bool{}
		for _, w := range b {
			set[w] = true
		}
		for _, w := range a {
			if !set[w] {
				return false
			}
		}
		return true
	}
	out := map[string]bool{}
	for _, m := range matches {
		dominated := false
		for _, other := range matches {
			if other.view != m.view && subset(m.words, other.words) {
				dominated = true
			}
		}
		if !dominated {
			out[m.view] = true
		}
	}
	return out
}

func govSQLCheckView(facts govSQLQuestionFacts, validated *govSQLValidated) []govSQLObligation {
	if len(facts.NamedViews) == 0 || facts.NamedViews[validated.View.Name] {
		return nil
	}
	names := make([]string, 0, len(facts.NamedViews))
	for name := range facts.NamedViews {
		names = append(names, name)
	}
	sort.Strings(names)
	return []govSQLObligation{{
		Kind: "VIEW", Subject: validated.View.Name,
		Message: fmt.Sprintf("The question is about %s, but the query reads %s. Use %s.", strings.Join(names, " / "), validated.View.Name, strings.Join(names, " or ")),
		Reason:  fmt.Sprintf("the query read %s, which is not the evidence the question names", validated.View.Display),
	}}
}

var govSQLRxGroupNoun = regexp.MustCompile(`(?i)\b(?:for\s+each|of\s+each|in\s+each|by\s+each|per|each)\s+([a-z][a-z-]*(?:\s+[a-z][a-z-]*)?)`)

var govSQLFunctionWords = map[string]bool{
	"the": true, "a": true, "an": true, "of": true, "in": true, "on": true, "are": true, "is": true, "there": true, "do": true, "does": true,
	"did": true, "have": true, "has": true, "were": true, "was": true, "and": true, "with": true, "for": true, "from": true, "how": true,
	"many": true, "what": true, "which": true, "that": true, "this": true, "to": true, "by": true, "at": true, "be": true, "case": true,
}

// govSQLGroupNouns are the nouns the question groups by ("for each record type" -> record, type).
func govSQLGroupNouns(question string) [][]string {
	var out [][]string
	for _, m := range govSQLRxGroupNoun.FindAllStringSubmatch(question, -1) {
		words := []string{}
		for _, w := range govSQLPhraseWords(m[1]) {
			if !govSQLFunctionWords[w] {
				words = append(words, w)
			} else {
				break
			}
		}
		if len(words) > 0 {
			out = append(out, words)
		}
	}
	return out
}

// govSQLCheckGrouping holds the query to a grouping that is a field of the evidence. "How many
// records for each record type" names no field of the call records; answering it by call
// type is an answer to a different question (measured 2026-10-06).
func govSQLCheckGrouping(question string, views []*govSQLView) []govSQLObligation {
	var unmet []govSQLObligation
	for _, words := range govSQLGroupNouns(question) {
		found := false
		for _, view := range views {
			sets := []map[string]bool{{"source": true, "file": true}, {"record": true, "id": true}, {"row": true, "number": true}}
			for _, column := range view.Columns {
				tokens := map[string]bool{}
				for _, phrase := range append([]string{column.Display, column.Name}, column.Synonyms...) {
					for _, w := range govSQLPhraseWords(strings.ReplaceAll(phrase, "_", " ")) {
						tokens[w] = true
					}
				}
				sets = append(sets, tokens)
			}
			for _, set := range sets {
				covered := true
				for _, w := range words {
					if !set[w] {
						covered = false
					}
				}
				if covered {
					found = true
				}
			}
		}
		if !found {
			unmet = append(unmet, govSQLObligation{
				Kind: "GROUPING", Subject: strings.Join(words, " "),
				Message: fmt.Sprintf("The question groups by %q, which is not a column of the evidence. Do not substitute another column: set answerable to false.", strings.Join(words, " ")),
				Reason:  fmt.Sprintf("I could not tie %q to a field of this evidence", strings.Join(words, " ")),
			})
		}
	}
	return unmet
}
