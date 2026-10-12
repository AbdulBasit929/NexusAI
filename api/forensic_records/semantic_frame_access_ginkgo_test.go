package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// "What remote access setup does the HP guide describe?" (rehearsal item DOC-05, 2026-10-09) is a question about a document. Its one
// family word was "access": the frame hinted the access-log family, that family's operation outranked every other, and the
// question was withheld as "matched System access logs" instead of being treated as the document question it is.
var _ = Describe("Semantic frame: the word access beside a document", func() {
	const doc05 = "What remote access setup does the HP guide describe?"
	workspace := func(question string) hybridQueryRequest {
		return hybridQueryRequest{Query: question, TenantID: "t", CollectionID: "c", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	}
	familyOf := func(question string) string { return extractSemanticFrame(workspace(question)).FamilyHint }

	DescribeTable("is not the access-log family when the question names a document, an image or a recording",
		func(question, family string) {
			Expect(familyOf(question)).To(Equal(family))
		},
		Entry("the rehearsal's DOC-05", doc05, "document_intelligence"),
		Entry("the same guide, asked another way", "What does the HP guide say about remote access?", "document_intelligence"),
		Entry("documents", "Search the documents for remote access instructions", "document_intelligence"),
		Entry("a PDF", "Which PDF explains how to get access to the server room?", "document_intelligence"),
		Entry("an image", "Which image shows the access control panel?", "image_intelligence"),
		Entry("a recording", "Which recording mentions access to the building?", "audio_intelligence"),
	)

	DescribeTable("is still the access-log family for a real access-log question",
		func(question string) {
			Expect(familyOf(question)).To(Equal("access_security_logs"))
		},
		Entry("failed access events", "Show failed access events"),
		Entry("the corpus phrasing of the same", "Show explicit failed access events"),
		Entry("status codes in the access logs", "Show the breakdown of HTTP status codes in the access logs"),
		Entry("entries in the access log", "How many access log entries are there?"),
		Entry("authentication in the access logs", "Find explicit failed authentication events in the access logs."),
		Entry("access denied", "How many times was access denied?"),
		Entry("login events", "failed login events"),
	)

	It("keeps the other family words of the access-log family working beside a document noun", func() {
		// Only the bare word "access" gives way. "login", "authentication", "http" and "security" still select the family, so a question
		// with one of them and a document noun is a multi-family question exactly as it was before (and not this one's hijack).
		for _, question := range []string{
			"Which documents mention failed login attempts?",
			"Which PDF describes the authentication events?",
		} {
			Expect(familyOf(question)).To(Equal("cross_family"), question)
		}
	})

	Describe("in the deterministic compiler", func() {
		It("does not pick the access-log operation for the document question", func() {
			result, err := compileDeterministicSemanticRequest(context.Background(), config{}, workspace(doc05), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.OperationID).NotTo(Equal("access.failed_events"))
			Expect(result.Executor).NotTo(Equal(semanticExecutorRegistered), "reason=%s", result.ReasonCode)
			planned, state := applyDeterministicSemanticCompiler(context.Background(), config{}, workspace(doc05))
			Expect(planned.Template).NotTo(Equal("access_failed_events"))
			Expect(state).NotTo(Equal("semantic_deterministic_registered"))
		})

		DescribeTable("still picks the access-log operation, with no model, for a real access-log question",
			func(question string) {
				result, err := compileDeterministicSemanticRequest(context.Background(), config{}, workspace(question), nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Executor).To(Equal(semanticExecutorRegistered), "reason=%s", result.ReasonCode)
				Expect(result.OperationID).To(Equal("access.failed_events"))
				planned, state := applyDeterministicSemanticCompiler(context.Background(), config{}, workspace(question))
				Expect(state).To(Equal("semantic_deterministic_registered"))
				Expect(planned.Template).To(Equal("access_failed_events"))
			},
			Entry("the status codes of the access logs", "Show the breakdown of HTTP status codes in the access logs"),
			Entry("failed access events", "Show failed access events"),
		)
	})

	Describe("in the lane's shortlist", func() {
		It("offers the access log for an access-log question and no view at all for the document question", func() {
			layer, _ := defaultSemanticLayer()
			Expect(layer).NotTo(BeNil())
			offer := func(question string) []string {
				req := workspace(question)
				names := []string{}
				for _, view := range govSQLOffer(govSQLViews(layer, req), question) {
					names = append(names, view.Name)
				}
				return names
			}
			Expect(offer("How many access log entries are there?")).To(Equal([]string{"v_access_log"}))
			Expect(offer("Show the breakdown of HTTP status codes in the access logs")).To(Equal([]string{"v_access_log"}))
			Expect(offer(doc05)).To(BeEmpty(), "the lane has nothing to ask the model about the guide")
		})
	})

	Describe("through the handler", func() {
		answerOf := func(question string) map[string]any {
			body, _ := json.Marshal(map[string]string{"tenant_id": "default", "collection_id": "records-demo", "query": question})
			recorder := httptest.NewRecorder()
			// The model name is what lets an unresolved question reach the deterministic compiler; the compiler itself asks no model.
			hybridQueryHandler(config{SynthesisModel: "scripted-model"}, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/query/hybrid", strings.NewReader(string(body))))
			Expect(recorder.Code).To(Equal(http.StatusOK))
			var reply map[string]any
			Expect(json.Unmarshal(recorder.Body.Bytes(), &reply)).To(Succeed())
			return reply
		}

		It("no longer tells the analyst the document question matched the access logs", func() {
			reply := answerOf(doc05)
			raw, _ := json.Marshal(reply)
			Expect(string(raw)).NotTo(ContainSubstring("System access logs"))
			clarification, _ := reply["clarification"].(map[string]any)
			Expect(clarification["reason_code"]).NotTo(Equal(withholdMediaMisroute))
			Expect(reply["template"]).NotTo(Equal("access_failed_events"))
		})

		It("still sends a real access-log question to the access-log operation", func() {
			reply := answerOf("Show the breakdown of HTTP status codes in the access logs")
			Expect(reply["template"]).To(Equal("access_failed_events"))
		})
	})
})
