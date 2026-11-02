package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensicrequest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A word of the question that is a value of a column the query filters on is not answered with another value of that column (found by the verified bank run of 2026-11-02, VB-INVE-003:
// "How many tabular files are in this case?" was answered 4, with modality = 'document', where the case holds 3 tabular files). The description of v_evidence_inventory.modality lists five
// values; the case's files also carry text and tabular. The values are the case's own, read with the coverage.
var _ = Describe("Ask engine: the values a column holds in this case", func() {
	// the Orion reference case: text 4, tabular 3, structured_records 42, document 4 (53 files)
	domains := map[string][]string{
		"v_evidence_inventory.modality":      {"document", "structured_records", "tabular", "text"},
		"v_evidence_inventory.detected_type": {"cdr", "document", "ipdr", "pdf", "spreadsheet", "text", "transaction"},
	}
	const (
		tabular   = "How many tabular files are in this case?"
		documents = "How many document files are in this case?"
	)
	var cat *askCatalogue
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})

	// kinds are the obligations a query leaves unmet for a question, with the case's domains in the frame (nil: not known)
	kinds := func(question, sql string, known map[string][]string) map[string]string {
		valid, rej := askValidate(sql, cat, nil)
		Expect(rej).To(BeNil(), sql)
		f := askExtractFrame(question)
		f.Domains = known
		out := map[string]string{}
		for _, o := range askCheckFrame(f, valid) {
			out[o.Kind] = o.Message
		}
		return out
	}
	count := func(where string) string {
		return "SELECT count(*) AS n FROM v_evidence_inventory WHERE " + where
	}

	It("refuses the value the live engine wrote for tabular files, and names the value the question says", func() {
		got := kinds(tabular, count("modality = 'document'"), domains)
		Expect(got).To(HaveKey("VALUE_DOMAIN"))
		Expect(got["VALUE_DOMAIN"]).To(ContainSubstring("'tabular'"))
		Expect(got["VALUE_DOMAIN"]).To(ContainSubstring("'document'"))
		Expect(got["VALUE_DOMAIN"]).To(ContainSubstring("document, structured_records, tabular, text"), "the values present are shown")
		Expect(got["VALUE_DOMAIN"]).NotTo(ContainSubstring("%!"))
		Expect(kinds(tabular, count("modality = 'tabular'"), domains)).NotTo(HaveKey("VALUE_DOMAIN"))
	})

	It("accepts a query that uses a value the question names, whatever else it uses", func() {
		Expect(kinds(tabular, count("modality IN ('tabular', 'text')"), domains)).NotTo(HaveKey("VALUE_DOMAIN"))
		Expect(kinds(tabular, count("modality = ANY (ARRAY['tabular'])"), domains)).NotTo(HaveKey("VALUE_DOMAIN"))
		Expect(kinds(tabular, count("'tabular' = modality"), domains)).NotTo(HaveKey("VALUE_DOMAIN"))
		Expect(kinds(documents, count("modality = 'document'"), domains)).NotTo(HaveKey("VALUE_DOMAIN"))
		// the same word for the other value is the same defect the other way round
		Expect(kinds(documents, count("modality = 'tabular'"), domains)).To(HaveKey("VALUE_DOMAIN"))
		Expect(kinds("How many structured record files are in this case?", count("modality = 'document'"), domains)).To(HaveKey("VALUE_DOMAIN"))
		Expect(kinds("How many structured record files are in this case?", count("modality = 'structured_records'"), domains)).NotTo(HaveKey("VALUE_DOMAIN"))
	})

	It("leaves alone what is not a filter on a value of the column, so that a fair reading is not forced (VB-INVE-004)", func() {
		// "text files" read as the extension txt is another column: the check does not say the modality should be used (the hint and the description are what steer that)
		Expect(kinds("How many text files are in this case?", count("extension = 'txt'"), domains)).NotTo(HaveKey("VALUE_DOMAIN"))
		Expect(kinds("How many text files are in this case?", count("modality = 'text'"), domains)).NotTo(HaveKey("VALUE_DOMAIN"))
		// a filter that is not an equality says nothing about a value
		Expect(kinds(tabular, count("modality <> 'document'"), domains)).NotTo(HaveKey("VALUE_DOMAIN"))
		// a word that is no value of this case's column (there are no images in the Orion case) is not concerned
		Expect(kinds("How many image files are in this case?", count("modality = 'image'"), domains)).NotTo(HaveKey("VALUE_DOMAIN"))
		// a question that names no value
		Expect(kinds("How many evidence files are in this case?", count("modality = 'document'"), domains)).NotTo(HaveKey("VALUE_DOMAIN"))
		// the values of the case are not known (a stub, an older service): nothing is asserted
		Expect(kinds(tabular, count("modality = 'document'"), nil)).NotTo(HaveKey("VALUE_DOMAIN"))
		// a view that the query does not read
		Expect(kinds("How many tabular events did 923000000101 have?", "SELECT count(*) AS n FROM v_comm_events WHERE src = '923000000101'", domains)).NotTo(HaveKey("VALUE_DOMAIN"))
	})

	It("holds the detected type to the same rule", func() {
		Expect(kinds("How many CDR files are in this case?", count("detected_type = 'ipdr'"), domains)).To(HaveKey("VALUE_DOMAIN"))
		Expect(kinds("How many CDR files are in this case?", count("detected_type = 'cdr'"), domains)).NotTo(HaveKey("VALUE_DOMAIN"))
	})

	It("reads a value in the plural and with an underscore written as a space, as a word and not as a part of one", func() {
		values := []string{"document", "structured_records", "tabular", "text"}
		Expect(askDomainNamed("How many tabular files?", values)).To(Equal([]string{"tabular"}))
		Expect(askDomainNamed("How many documents and texts?", values)).To(Equal([]string{"document", "text"}))
		Expect(askDomainNamed("How many structured records are there?", values)).To(Equal([]string{"structured_records"}))
		Expect(askDomainNamed("How many structured record files?", values)).To(Equal([]string{"structured_records"}))
		Expect(askDomainNamed("How many contextual files?", values)).To(BeEmpty(), "text inside another word is not the word")
		Expect(askDomainNamed("How many files are there?", values)).To(BeEmpty())
	})

	It("keeps only the domains of the views the question reads", func() {
		cover := askCoverage{Domains: domains}
		Expect(askDomainsFor(cover, []string{"v_evidence_inventory"})).To(HaveLen(2))
		Expect(askDomainsFor(cover, []string{"v_comm_events"})).To(BeNil())
		Expect(askDomainsFor(askCoverage{}, []string{"v_evidence_inventory"})).To(BeNil(), "a coverage that read no domains")
	})

	It("tells the model, in the hints, the values the case holds and the one the question names", func() {
		f := askExtractFrame(tabular)
		f.Domains = domains
		prompt := askUserPrompt(f)
		Expect(prompt).To(ContainSubstring("The column modality holds these values in this case: document, structured_records, tabular, text. The question names tabular"))
		Expect(prompt).NotTo(ContainSubstring("detected_type"), "no word of the question is a detected type")
		f.Domains = nil
		Expect(askUserPrompt(f)).NotTo(ContainSubstring("holds these values"))
		f = askExtractFrame("How many evidence files are in this case?")
		f.Domains = domains
		Expect(askUserPrompt(f)).NotTo(ContainSubstring("holds these values"), "nothing named, nothing to say")
	})

	Describe("through the run", func() {
		var (
			prompts []string
			execs   []string
			cfg     config
			db      *pgxpool.Pool

			savedModel    askModelFunc
			savedLoader   = askCatalogueLoader
			savedExec     = askExec
			savedProbeFn  = askProbeFn
			savedSink     = askAuditSink
			savedCover    = askCoverageRead
			savedCaseSet  = askCaseSettingsFn
			savedEvidence = askCollectEvidenceFn
		)
		BeforeEach(func() {
			savedModel = askModel
			prompts, execs = nil, nil
			db = &pgxpool.Pool{}
			cfg = config{SynthesisModel: "test-model"}
			askCatalogueLoader = func(context.Context, *pgxpool.Pool) (*askCatalogue, error) { return cat, nil }
			askExec = func(_ context.Context, _ *pgxpool.Pool, _, _ string, v *askValidated, _ askLimits) (*govSQLResult, error) {
				execs = append(execs, v.SQL)
				return &govSQLResult{Columns: []string{"n"}, Texts: []bool{false}, Rows: [][]any{{int64(3)}}}, nil
			}
			askProbeFn = func(context.Context, *pgxpool.Pool, string, string, *askCatalogue, string, askLimits) ([]string, error) {
				return nil, nil
			}
			askCaseSettingsFn = func(context.Context, *pgxpool.Pool, string, string) (askCaseSetting, error) { return askCaseSetting{}, nil }
			askCollectEvidenceFn = func(context.Context, *pgxpool.Pool, string, string, *askCatalogue, *askValidated, *govSQLResult, askLimits) *askEvidence {
				return nil
			}
			askCoverageCache.entries = nil
			askCoverageRead = func(context.Context, *pgxpool.Pool, string, string) (askCoverage, error) {
				return askCoverage{Files: 3, Typed: 3, AnyComm: true, AnySession: true, AnyUsage: true, AnyVolume: true, Domains: domains}, nil
			}
			askAuditSink = func(context.Context, *pgxpool.Pool, hybridQueryRequest, AskAuditV2, askFrame, string) {}
		})
		AfterEach(func() {
			askModel, askCatalogueLoader, askExec, askProbeFn, askAuditSink, askCoverageRead = savedModel, savedLoader, savedExec, savedProbeFn, savedSink, savedCover
			askCaseSettingsFn, askCollectEvidenceFn = savedCaseSet, savedEvidence
		})

		script := func(sqls ...string) {
			i := 0
			askModel = func(_ context.Context, _ config, _ string, turns []govSQLTurn) (string, string) {
				prompts = append(prompts, turns[len(turns)-1].Content)
				if i >= len(sqls) {
					return "", "SCRIPT_ENDED"
				}
				raw, _ := json.Marshal(map[string]any{"answerable": true, "sql": sqls[i]})
				i++
				return string(raw), ""
			}
		}
		ask := func(question string) AskAuditV2 {
			req := hybridQueryRequest{TenantID: "t", CollectionID: "c", Query: question, RequestClass: forensicrequest.GovernedAnalysis}
			_, audit := runAskEngineV2(context.Background(), cfg, db, req, time.Now())
			return audit
		}

		It("puts the case's values in the first draft's hints, repairs a draft that uses another value, and runs only the repaired one", func() {
			script(count("modality = 'document'"), count("modality = 'tabular'"))
			audit := ask(tabular)
			Expect(audit.State).To(Equal("answered"), audit.Reason)
			Expect(audit.Attempts).To(Equal(2))
			Expect(audit.Repairs).To(Equal([]string{"OBLIGATION:VALUE_DOMAIN"}))
			Expect(prompts[0]).To(ContainSubstring("The column modality holds these values in this case: document, structured_records, tabular, text"))
			Expect(prompts[1]).To(ContainSubstring("Compare modality with 'tabular'"))
			Expect(execs).To(Equal([]string{count("modality = 'tabular'")}), "the draft with the other value never ran")
		})

		It("answers at once when the first draft already uses the value the question names", func() {
			script(count("modality = 'tabular'"))
			audit := ask(tabular)
			Expect(audit.State).To(Equal("answered"), audit.Reason)
			Expect(audit.Attempts).To(Equal(1))
		})

		It("does not add a hint or a check for a case whose values were not read", func() {
			askCoverageCache.entries = nil
			askCoverageRead = func(context.Context, *pgxpool.Pool, string, string) (askCoverage, error) {
				return askCoverage{Files: 3, Typed: 3, AnyComm: true, AnySession: true, AnyUsage: true, AnyVolume: true}, nil
			}
			script(count("modality = 'document'"))
			audit := ask(tabular)
			Expect(audit.Attempts).To(Equal(1))
			Expect(prompts[0]).NotTo(ContainSubstring("holds these values"))
		})
	})
})
