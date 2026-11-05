package main

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The replay of the saved queries of a bank under the CURRENT frame and obligations, for the impact index (tools/ask_eval/impact_index.py). It needs no model and no database: the saved
// query of each answered question is validated against the pinned catalogue and held to the frame of its question, and the line written says which obligations it now draws, whether the
// frame reads the direction from the place of an identifier, whether the query is a one-row ranking (a tie can surface) and whether the ORDER rule is waived for it. Opt in with
// ASK_REPLAY_IN (lines of {id, question, sql, views}) and ASK_REPLAY_OUT; without them the spec is skipped.
var _ = Describe("Ask engine replay of saved queries", func() {
	It("holds each saved query to the current obligations of its question", func() {
		in, out := os.Getenv("ASK_REPLAY_IN"), os.Getenv("ASK_REPLAY_OUT") //nolint:forbidigo // opt-in switch for a replay tool
		if in == "" || out == "" {
			Skip("opt in with ASK_REPLAY_IN and ASK_REPLAY_OUT (tools/ask_eval/impact_index.py)")
		}
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		source, err := os.Open(in)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = source.Close() }()
		sink, err := os.Create(out)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = sink.Close() }()
		scanner := bufio.NewScanner(source)
		scanner.Buffer(make([]byte, 0, 1<<20), 8<<20)
		writer := json.NewEncoder(sink)
		for scanner.Scan() {
			var item struct {
				ID       string   `json:"id"`
				Question string   `json:"question"`
				SQL      string   `json:"sql"`
				Views    []string `json:"views"`
			}
			Expect(json.Unmarshal(scanner.Bytes(), &item)).To(Succeed())
			frame := askExtractFrameWith(item.Question, cat.View("v_plate_sightings") != nil)
			line := map[string]any{"id": item.ID, "direction_role": frame.DirectionRole, "role_ids": frame.RoleIDs, "incoming": frame.Cues["incoming"], "outgoing": frame.Cues["outgoing"], "rank": frame.Cues["rank"], "limit": frame.Limit}
			if item.SQL != "" {
				valid, rejection := askValidate(item.SQL, cat, item.Views)
				if rejection != nil {
					line["validator"] = rejection.Code
				} else {
					kinds := []string{}
					for _, o := range askCheckFrame(frame, valid) {
						kinds = append(kinds, o.Kind)
					}
					line["obligations"] = kinds
					lower := strings.ToLower(valid.SQL)
					line["tie_eligible"] = valid.Limit == 1 && frame.Cues["rank"] && frame.Limit <= 1
					line["order_waived"] = frame.Cues["rank"] && !frame.Cues["count"] && !strings.Contains(lower, "order by") && askExtremeValue(frame, lower)
				}
			}
			Expect(writer.Encode(line)).To(Succeed())
		}
		Expect(scanner.Err()).NotTo(HaveOccurred())
	})
})
