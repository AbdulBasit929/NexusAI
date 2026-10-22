package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A question bank of stage 3 (tools/ask_eval/stage3a_bank.py, stage3bc_bank.py) carries, with every question, the view it must be routed to and the values of the runtime parameters the
// engine must read from it. This checks both for every question of the banks named by ASK_BANK_PARSE (file paths separated by ";"), with no database and no model: a wording the engine cannot
// read is found in a second, not in a model run. It runs only when the variable is set (the harness sets it).
var _ = Describe("Ask engine reading of a question bank", func() {
	type bankQuestion struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Question string `json:"question"`
		Expect   struct {
			Params map[string]int `json:"params"`
			View   *string        `json:"view"`
		} `json:"expect"`
	}

	It("reads the parameters and routes the view of every question", func() {
		files := os.Getenv("ASK_BANK_PARSE") //nolint:forbidigo // set by the harness only
		if files == "" {
			Skip("ASK_BANK_PARSE names no bank")
		}
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		var misses []string
		checked := 0
		for _, file := range strings.Split(files, ";") {
			raw, err := os.ReadFile(file) //nolint:gosec // a path the harness gives
			Expect(err).NotTo(HaveOccurred())
			var questions []bankQuestion
			Expect(json.Unmarshal(raw, &questions)).To(Succeed())
			for _, q := range questions {
				checked++
				frame := askExtractFrame(q.Question)
				short := cat.Shortlist(q.Question, 0)
				if frame.Relative != "" {
					misses = append(misses, fmt.Sprintf("%s %q: the engine declines a relative time with no reference day (%q)", q.ID, q.Question, frame.Relative))
				}
				if q.Expect.View != nil && (len(short.Views) != 1 || short.Views[0] != *q.Expect.View) {
					misses = append(misses, fmt.Sprintf("%s %q: wanted the view %s alone, got %v", q.ID, q.Question, *q.Expect.View, short.Views))
				}
				for _, p := range askParams {
					read := frame.Params[p.Name]
					want, named := q.Expect.Params[p.Name]
					switch {
					case read.Ambiguous:
						misses = append(misses, fmt.Sprintf("%s %q: %s is read as more than one value %v", q.ID, q.Question, p.Name, read.Values))
					case named && (!read.Found || read.Value != want):
						misses = append(misses, fmt.Sprintf("%s %q: %s should read %d, read %v (found %v)", q.ID, q.Question, p.Name, want, read.Values, read.Found))
					case !named && read.Found:
						misses = append(misses, fmt.Sprintf("%s %q: %s is read as %v but the question names none", q.ID, q.Question, p.Name, read.Values))
					}
				}
				if _, issue := askResolveParams(frame, short.Views); issue != nil {
					misses = append(misses, fmt.Sprintf("%s %q: the parameters cannot be resolved: %s", q.ID, q.Question, issue.Message))
				}
			}
		}
		AddReportEntry("bank questions read", fmt.Sprintf("%d questions", checked))
		Expect(misses).To(BeEmpty(), strings.Join(misses, "\n"))
	})
})
