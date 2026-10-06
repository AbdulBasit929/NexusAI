package main

import (
	"fmt"
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

func govSQLPhraseWords(phrase string) []string {
	return semanticWords.FindAllString(strings.ToLower(strings.TrimSpace(phrase)), -1)
}

// govSQLFieldFacts finds the columns of the offered views that the question names by
// a phrase no other column of that view answers to.
func govSQLFieldFacts(question string, views []*govSQLView) []govSQLFieldFact {
	stems := _semanticLayerQuestionTokens(question)
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
			if len(cols) != 1 || !_semanticLayerPhraseInQuestion(key, stems) {
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

// govSQLCheckFilters refuses an equality filter on an enumerated column that the
// question did not ask for, or that names a value the layer does not declare.
func govSQLCheckFilters(question string, facts govSQLQuestionFacts, validated *govSQLValidated) []govSQLObligation {
	view := validated.View
	enumerated := map[string][]string{}
	for _, column := range view.Columns {
		if len(column.Values) > 0 {
			enumerated[column.Name] = column.Values
		}
	}
	grounded := map[string]bool{}
	for _, v := range append(append([]govSQLValueFact{}, facts.ValueFacts...), facts.WeakValues...) {
		if v.View == view.Name {
			grounded[v.Column+"\x00"+strings.ToLower(v.Value)] = true
		}
	}
	stems := _semanticLayerQuestionTokens(question)
	// The words of a multi-word column name the question uses ("call start time") are spent on
	// naming that column; they are not also a request for a value ("call" is a value of call_type).
	for _, column := range view.Columns {
		for _, phrase := range append([]string{column.Display}, column.Synonyms...) {
			if words := govSQLPhraseWords(phrase); len(words) > 1 && _semanticLayerPhraseInQuestion(phrase, stems) {
				for _, w := range words {
					delete(stems, w)
				}
			}
		}
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
	says := func(value string) bool { return _semanticLayerPhraseInQuestion(value, stems) }
	var unmet []govSQLObligation
	for _, cmp := range govSQLComparisons(validated.Tree) {
		declared, ok := enumerated[cmp.Column]
		if !ok || cmp.Op != "=" {
			continue
		}
		for _, c := range cmp.Consts {
			value := strings.ToLower(strings.TrimSpace(c))
			isDeclared := false
			for _, d := range declared {
				if strings.EqualFold(d, c) {
					isDeclared = true
				}
			}
			switch {
			case !isDeclared && !named(c):
				unmet = append(unmet, govSQLObligation{
					Kind: "FILTER", Subject: c,
					Message: fmt.Sprintf("%s = '%s' is not a value of %s (its values are %s) and the question does not say it. Remove that filter.", cmp.Column, c, cmp.Column, strings.Join(declared, ", ")),
					Reason:  fmt.Sprintf("the query filtered %s on %q, which the question does not ask for", cmp.Column, c),
				})
			case isDeclared && !grounded[cmp.Column+"\x00"+value] && !says(c):
				unmet = append(unmet, govSQLObligation{
					Kind: "FILTER", Subject: c,
					Message: fmt.Sprintf("The question does not ask for %s = '%s'. Remove that filter: count everything unless the question restricts it.", cmp.Column, c),
					Reason:  fmt.Sprintf("the query filtered %s on %q, which the question does not ask for", cmp.Column, c),
				})
			}
		}
	}
	return unmet
}
