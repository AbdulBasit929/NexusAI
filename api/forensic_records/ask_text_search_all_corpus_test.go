package main

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"testing"

	"github.com/mudler/LocalAI/pkg/forensictext"
)

// TestTextSearchOnExportedCorpus runs the any-source search on passages exported from a case (a CSV with the columns original_filename, artifact_type and t) for a list of probes, and writes what it
// found. It reads no database and asserts nothing: tools/showcase compares the output with the true file lists. Skipped unless TEXT_SEARCH_CORPUS_CSV and TEXT_SEARCH_PROBES name the files.
//
//	TEXT_SEARCH_CORPUS_CSV=C:/tmp/nexusai-g4/orion_text.csv TEXT_SEARCH_PROBES=probes.json TEXT_SEARCH_OUT=out.json go test -run TestTextSearchOnExportedCorpus -count=1 ./api/forensic_records/
func TestTextSearchOnExportedCorpus(t *testing.T) {
	csvPath, probesPath := os.Getenv("TEXT_SEARCH_CORPUS_CSV"), os.Getenv("TEXT_SEARCH_PROBES") //nolint:forbidigo // measurement harness reads its inputs from the environment
	if csvPath == "" || probesPath == "" {
		t.Skip("TEXT_SEARCH_CORPUS_CSV and TEXT_SEARCH_PROBES not set") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	handle, err := os.Open(csvPath)
	if err != nil {
		t.Fatal(err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	defer func() { _ = handle.Close() }()
	records, err := csv.NewReader(handle).ReadAll()
	if err != nil || len(records) < 2 {
		t.Fatalf("corpus: %v", err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	rows := []map[string]any{}
	for i, record := range records[1:] {
		rows = append(rows, map[string]any{
			"artifact_id": "row-" + string(rune('a'+i%26)) + record[0], "artifact_type": record[1], "evidence_id": record[0], "version_id": "v", "source_file": record[0], "passage_text": record[2],
			"citation_locator": map[string]any{}, "run_id": "r",
		})
	}
	raw, err := os.ReadFile(probesPath)
	if err != nil {
		t.Fatal(err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	var probes []struct {
		Literal string `json:"literal"`
	}
	if err := json.Unmarshal(raw, &probes); err != nil {
		t.Fatal(err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	type found struct {
		File string `json:"file"`
		Best string `json:"best"`
	}
	type part struct {
		Phrase string  `json:"phrase"`
		Files  []found `json:"files"`
	}
	type result struct {
		Literal  string  `json:"literal"`
		Files    []found `json:"files"`
		Partial  []part  `json:"partial,omitempty"`
		Searched int     `json:"searched"`
	}
	out := []result{}
	for _, probe := range probes {
		query := forensictext.Query{LiteralText: probe.Literal, MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}
		state := newTextSearchState(query, textSearchAllExamples)
		state.keep = true
		for _, row := range rows {
			state.add(row)
		}
		res := result{Literal: probe.Literal, Searched: state.inspected}
		files := state.ordered()
		for _, f := range files {
			res.Files = append(res.Files, found{File: f.File, Best: string(f.Best)})
		}
		already := map[string]bool{}
		for _, f := range files {
			already[f.EvidenceID+"|"+f.File] = true
		}
		if len(files) == 0 || textSearchHasDigit(probe.Literal) {
			for _, p := range state.relaxed(already) {
				entry := part{Phrase: p.Phrase}
				for _, f := range p.Files {
					entry.Files = append(entry.Files, found{File: f.File, Best: string(f.Best)})
				}
				res.Partial = append(res.Partial, entry)
			}
		}
		out = append(out, res)
	}
	encoded, _ := json.MarshalIndent(out, "", " ")
	path := os.Getenv("TEXT_SEARCH_OUT") //nolint:forbidigo // measurement harness
	if path == "" {
		path = "text-search-corpus.json"
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}
