package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

//go:embed ask_engine_retrieval_cases.json
var askRetrievalCasesJSON []byte

type askRetrievalCase struct {
	Q       string   `json:"q"`
	Views   []string `json:"views"`
	Example string   `json:"example"`
	Heldout bool     `json:"heldout"`
}

// The gates below were written before the first measurement (roadmap 6.8.10): the set was labelled by hand first, the lexicon was tuned against the
// non-held-out questions only, and the held-out ones were measured once. The set is the author's, written in English; it is a regression and development
// instrument, not evidence of accuracy on real analysts' questions. The analysts' and native speakers' sets of roadmap 6.4 replace it.
const (
	askGateRecall        = 0.95 // every view the question needs is in the shortlist
	askGateRecallHeldout = 0.90
	askGateMeanSize      = 1.8
	askGateExampleHit    = 0.85 // the labelled example is among the first two retrieved
)

var _ = Describe("Ask engine shortlist and example retrieval", func() {
	var (
		cat      *askCatalogue
		examples []askExample
		cases    []askRetrievalCase
	)
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		examples, err = askExamples()
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Unmarshal(askRetrievalCasesJSON, &cases)).To(Succeed())
	})

	It("replaces identifiers by placeholders and counts them", func() {
		text, f := askNormalise("Who held 203.0.113.7 port 40000 at 2026-03-04 10:15, and who used 359000000000001 or +923001110001 on cell 410-01:1a2b:3c4d?")
		Expect(f.IPs).To(Equal(1))
		Expect(f.Dates).To(Equal(1))
		Expect(f.IMEIs).To(Equal(1))
		Expect(f.Phones).To(Equal(1))
		Expect(text).To(ContainSubstring("ipaddr"))
		Expect(text).To(ContainSubstring("phonenumber"))
		Expect(text).NotTo(ContainSubstring("203.0"))
		_, f = askNormalise("Which numbers did 92300111000 and 923001110002 contact?")
		Expect(f.Phones).To(Equal(2))
	})

	It("stems the few endings that matter", func() {
		for _, w := range []string{"calls", "called", "calling"} {
			Expect(askStem(w)).To(Equal(askStem("call")), w)
		}
		Expect(askStem("cell")).To(Equal("cell"))
	})

	It("is deterministic and never returns a view twice or an unknown one", func() {
		for _, c := range cases {
			a := cat.Shortlist(c.Q, 0)
			b := cat.Shortlist(c.Q, 0)
			Expect(a.Views).To(Equal(b.Views), c.Q)
			seen := map[string]bool{}
			for _, v := range a.Views {
				Expect(cat.View(v)).NotTo(BeNil(), c.Q)
				Expect(seen[v]).To(BeFalse(), c.Q)
				seen[v] = true
			}
			Expect(len(a.Views)).To(BeNumerically("<=", askShortlistMax), c.Q)
		}
	})

	It("says when it is not confident, and still offers the best views", func() {
		sl := cat.Shortlist("tell me something interesting about the weather", 0)
		Expect(sl.Confident).To(BeFalse())
		sl = cat.Shortlist("Who does 923001110001 talk to most?", 0)
		Expect(sl.Confident).To(BeTrue())
		Expect(sl.Views[0]).To(Equal("v_contact_pairs"))
		Expect(sl.Reasons["v_contact_pairs"]).NotTo(BeEmpty(), "every choice can be explained")
	})

	It("points an address at internet sessions, a 15-digit number at devices and two numbers with a linking word at contact pairs", func() {
		Expect(cat.Shortlist("what happened at 198.51.100.4", 0).Views[0]).To(Equal("v_data_sessions"))
		Expect(cat.Shortlist("anything about 359000000000001", 0).Views[0]).To(Equal("v_device_pairings"))
		Expect(cat.Shortlist("923001110001 and 923001110002 mutual", 0).Views[0]).To(Equal("v_contact_pairs"))
	})

	It("meets the gates on the labelled set", func() {
		var total, hit, heldTotal, heldHit, size, withExample, exampleHit int
		var misses []string
		for _, c := range cases {
			sl := cat.Shortlist(c.Q, 0)
			ok := true
			for _, v := range c.Views {
				if !askContains(sl.Views, v) {
					ok = false
				}
			}
			total++
			size += len(sl.Views)
			if c.Heldout {
				heldTotal++
			}
			if ok {
				hit++
				if c.Heldout {
					heldHit++
				}
			} else {
				misses = append(misses, fmt.Sprintf("%q wanted %v got %v", c.Q, c.Views, sl.Views))
			}
			if c.Example != "" {
				withExample++
				got := askRetrieveExamples(c.Q, sl.Views, examples, 2)
				found := false
				for _, e := range got {
					if e.ID == c.Example {
						found = true
					}
				}
				if found {
					exampleHit++
				} else {
					misses = append(misses, fmt.Sprintf("%q wanted example %s got %v", c.Q, c.Example, askExampleIDs(got)))
				}
			}
		}
		recall := float64(hit) / float64(total)
		heldRecall := float64(heldHit) / float64(heldTotal)
		mean := float64(size) / float64(total)
		exampleRate := float64(exampleHit) / float64(withExample)
		AddReportEntry("shortlist and example retrieval", fmt.Sprintf("shortlist recall %.3f (held-out %.3f), mean size %.2f, example top-2 %.3f over %d questions", recall, heldRecall, mean, exampleRate, total))
		detail := strings.Join(misses, "\n")
		Expect(recall).To(BeNumerically(">=", askGateRecall), detail)
		Expect(heldRecall).To(BeNumerically(">=", askGateRecallHeldout), detail)
		Expect(mean).To(BeNumerically("<=", askGateMeanSize), detail)
		Expect(exampleRate).To(BeNumerically(">=", askGateExampleHit), detail)
	})

	It("uses the view's own description when the lexicon has no word for it", func() {
		Expect(cat.Shortlist("pairing", 0).Views).To(ContainElement("v_device_pairings"))
		Expect(askStem("pairings")).To(Equal(askStem("pairing")))
	})

	It("does not read a spatial question about two numbers as a contact question", func() {
		sl := cat.Shortlist("Did they meet? Show common places of 923001110001 and 923001110002", 0)
		Expect(sl.Views[0]).To(Equal("v_cell_visits"))
		Expect(cat.Shortlist("Who do 923001110001 and 923001110002 both contact?", 0).Views[0]).To(Equal("v_contact_pairs"))
	})

	It("honours the number of views asked for", func() {
		q := "How are 923001110001 and 923001110002 connected, and where were they?"
		Expect(len(cat.Shortlist(q, 1).Views)).To(Equal(1))
		Expect(len(cat.Shortlist(q, 2).Views)).To(BeNumerically("<=", 2))
		Expect(len(cat.Shortlist(q, 0).Views)).To(BeNumerically("<=", askShortlistMax))
	})

	It("does not drag in views the question does not need", func() {
		for _, q := range []string{"Which IMEI numbers were used with 923001110001?", "Show the coverage radius and position of site Gulberg", "Which apps does 923001110001 use most?"} {
			sl := cat.Shortlist(q, 0)
			Expect(sl.Views).To(HaveLen(1), fmt.Sprintf("%s scores %v reasons %v", q, sl.Scores, sl.Reasons))
		}
	})

	It("reports confidence from the best score and the threshold", func() {
		for _, c := range cases {
			sl := cat.Shortlist(c.Q, 0)
			best := 0.0
			for _, v := range sl.Views {
				if sl.Scores[v] > best {
					best = sl.Scores[v]
				}
			}
			Expect(sl.Confident).To(Equal(best >= askShortlistConfident), c.Q)
		}
		Expect(cat.Shortlist("hello there", 0).Confident).To(BeFalse())
	})

	It("lets the result shape the question asks for, and the best view, decide when no word is shared", func() {
		Expect(askExampleIDs(askRetrieveExamples("highest", []string{"v_contact_pairs"}, examples, 1))).To(Equal([]string{"top-contacts"}))
		Expect(askExampleIDs(askRetrieveExamples("facebook sessions", []string{"v_app_usage"}, examples, 1))).To(Equal([]string{"top-apps"}))
	})

	It("treats month names and similar words as parameters, not as intent", func() {
		a, _ := askNormalise("cells used in March 2026 local time")
		b, _ := askNormalise("cells used in April 2026 local time")
		Expect(askQuestionTokens(a)).To(Equal(askQuestionTokens(b)))
	})

	It("returns only examples whose views were offered, at most n, never twice", func() {
		for _, c := range cases {
			sl := cat.Shortlist(c.Q, 0)
			got := askRetrieveExamples(c.Q, sl.Views, examples, 2)
			Expect(len(got)).To(BeNumerically("<=", 2), c.Q)
			seen := map[string]bool{}
			for _, e := range got {
				Expect(seen[e.ID]).To(BeFalse(), c.Q)
				seen[e.ID] = true
				for _, v := range e.Views {
					Expect(sl.Views).To(ContainElement(v), c.Q+" "+e.ID)
				}
			}
		}
		Expect(askRetrieveExamples("anything", []string{"v_cells"}, examples, 2)).To(BeEmpty(), "no example uses v_cells alone")
		Expect(askRetrieveExamples("who talks most", []string{"v_contact_pairs"}, examples, 0)).To(BeEmpty())
	})

	It("matches an example even when its numbers differ, and prefers the one whose result shape the question asks for", func() {
		got := askRetrieveExamples("How are 923009990001 and 923009990002 connected?", []string{"v_connection_edges"}, examples, 1)
		Expect(askExampleIDs(got)).To(Or(Equal([]string{"connection-path"}), Equal([]string{"connection-path-any"})), "an example on the view the question was offered")
		got = askRetrieveExamples("How are 923009990001 and 923009990002 connected within four hops?", []string{"v_contact_pairs"}, examples, 1)
		Expect(askExampleIDs(got)).To(Equal([]string{"connection-path-long"}), "more than three hops is the recursive example over the contact pairs")
		got = askRetrieveExamples("Who do 923009990001 and 923009990002 both contact?", []string{"v_contact_pairs"}, examples, 1)
		Expect(askExampleIDs(got)).To(Equal([]string{"common-contacts"}))
	})

	It("keeps the prompt short: three views and two examples stay well under the whole catalogue", func() {
		q := "How are 923001110001 and 923001110002 connected within three hops?"
		sl := cat.Shortlist(q, 0)
		short := askPrompt(cat, sl.Views, askRetrieveExamples(q, sl.Views, examples, 2))
		full := askPrompt(cat, nil, examples)
		Expect(len(short)).To(BeNumerically("<", len(full)/2))
	})
})

func askExampleIDs(list []askExample) []string {
	out := make([]string, 0, len(list))
	for _, e := range list {
		out = append(out, e.ID)
	}
	return out
}
