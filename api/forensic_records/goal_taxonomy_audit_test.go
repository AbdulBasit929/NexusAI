package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"
)

// OFFLINE CENSUS of the goal taxonomy over every question we measure against.
// No model, no database, no deploy -- `semanticFrameGoal` and `s9QuestionShape`
// are both deterministic and lexical.
//
// This exists because the taxonomy change it informs has been attempted twice
// and produced a confident-wrong answer both times (WI-31 "breakdown" ->
// aggregate, ACC-02 CORRECT -> WRONG). The instrument comes first: before
// adding a goal or an S9 `rows` case, we need to know EXACTLY which questions
// currently fall to the `lookup` default, because that default means
// "unrecognised" and skips shape verification.
//
// Run: go test -run TestGoalTaxonomyCensus -v .
func TestGoalTaxonomyCensus(t *testing.T) {
	type question struct {
		ID string `json:"id"`
		Q  string `json:"q"`
	}
	load := func(path string) []question {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var doc struct {
			Questions []question `json:"questions"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		return doc.Questions
	}

	corpora := []struct {
		name string
		path string
	}{
		{"golden62", "../../evaluation/golden_questions_v2.json"},
		{"holdout", "../../evaluation/holdout_questions_v1.json"},
	}

	goalCounts := map[string]int{}
	shapeByGoal := map[string]map[string]int{}
	var unrecognised []string

	for _, corpus := range corpora {
		for _, q := range load(corpus.path) {
			terms := semanticFrameTerms(q.Q)
			goal := semanticFrameGoal(terms, q.Q)
			frame := SemanticFrameV1{
				Goal:         goal,
				Measure:      semanticFrameMeasure(terms),
				GroupByHints: semanticFrameGroupHints(q.Q, terms),
			}
			shape := s9QuestionShape(frame, q.Q)
			if shape == "" {
				shape = "(none)"
			}
			goalCounts[goal]++
			if shapeByGoal[goal] == nil {
				shapeByGoal[goal] = map[string]int{}
			}
			shapeByGoal[goal][shape]++
			if goal == "lookup" {
				unrecognised = append(unrecognised,
					fmt.Sprintf("  %-10s %-22s shape=%-9s groups=%d  %s",
						corpus.name, q.ID, shape, len(frame.GroupByHints), q.Q))
			}
		}
	}

	goals := make([]string, 0, len(goalCounts))
	total := 0
	for goal, n := range goalCounts {
		goals = append(goals, goal)
		total += n
	}
	sort.Slice(goals, func(i, j int) bool { return goalCounts[goals[i]] > goalCounts[goals[j]] })

	t.Logf("GOAL DISTRIBUTION over %d questions (golden62 + holdout):", total)
	for _, goal := range goals {
		shapes := make([]string, 0, len(shapeByGoal[goal]))
		for shape, n := range shapeByGoal[goal] {
			shapes = append(shapes, fmt.Sprintf("%s=%d", shape, n))
		}
		sort.Strings(shapes)
		t.Logf("  %-12s %3d  (%.0f%%)  shapes: %v", goal, goalCounts[goal],
			100*float64(goalCounts[goal])/float64(total), shapes)
	}

	t.Logf("")
	t.Logf("THE %d QUESTIONS ON THE `lookup` DEFAULT -- these are what an S9 `rows`", len(unrecognised))
	t.Logf("case would start asserting a shape for, and why one cannot be added yet:")
	for _, line := range unrecognised {
		t.Log(line)
	}
}
