package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/mudler/LocalAI/internal/caselayer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Case layer glue", func() {
	withPrincipal := func(p forensicPrincipal) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/conversations", nil)
		return r.WithContext(context.WithValue(r.Context(), forensicPrincipalContextKey{}, p))
	}

	Describe("who is asking", func() {
		It("takes the verified tenant, subject and role of an authenticated request, and the case a credential is bound to", func() {
			r := withPrincipal(forensicPrincipal{Authenticated: true, TenantID: "default", ActorID: "workspace", SubjectID: "analyst-7", ActorRole: "user", CollectionID: "orion"})
			sc, err := caseLayerScope(r, "orion")
			Expect(err).NotTo(HaveOccurred())
			Expect(sc).To(Equal(caselayer.Scope{TenantID: "default", Subject: "analyst-7", ActorRole: "user", BoundCase: "orion"}))
		})

		It("refuses a case other than the one the credential is bound to, with a 403", func() {
			r := withPrincipal(forensicPrincipal{Authenticated: true, TenantID: "default", ActorID: "w", SubjectID: "s", ActorRole: "user", CollectionID: "orion"})
			_, err := caseLayerScope(r, "meridian")
			var typed *caselayer.Error
			Expect(err).To(HaveOccurred())
			Expect(asCaseLayerError(err, &typed)).To(BeTrue())
			Expect(typed.Status).To(Equal(http.StatusForbidden))
		})

		It("keeps the agent worker out", func() {
			r := withPrincipal(forensicPrincipal{Authenticated: true, TenantID: "default", ActorID: "w", SubjectID: "s", ActorRole: "agent-worker"})
			_, err := caseLayerScope(r, "")
			Expect(err).To(HaveOccurred())
			var typed *caselayer.Error
			Expect(asCaseLayerError(err, &typed)).To(BeTrue())
			Expect(typed.Status).To(Equal(http.StatusForbidden))
		})

		It("reads the workspace's headers when authentication is off (a development setup), and says anonymous when there are none", func() {
			r := httptest.NewRequest(http.MethodGet, "/conversations?tenant_id=acme", nil)
			r.Header.Set(forensicSubjectHeader, "dev-analyst")
			sc, err := caseLayerScope(r, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(sc.TenantID).To(Equal("acme"))
			Expect(sc.Subject).To(Equal("dev-analyst"))
			bare, err := caseLayerScope(httptest.NewRequest(http.MethodGet, "/conversations", nil), "")
			Expect(err).NotTo(HaveOccurred())
			Expect(bare.TenantID).To(Equal("default"))
			Expect(bare.Subject).To(Equal("anonymous"))
		})
	})

	Describe("the frame of a question", func() {
		It("records identifiers, dates and limits for a later turn to be read against", func() {
			var frame map[string]any
			Expect(json.Unmarshal(caseLayerFrame("Which are the top 5 numbers that 923001234567 called on 2026-02-03?"), &frame)).To(Succeed())
			Expect(frame["numbers"]).To(ConsistOf("923001234567"))
			Expect(frame["dates"]).To(ConsistOf("2026-02-03"))
			Expect(frame["limit"]).To(BeEquivalentTo(5))
			Expect(frame["national_id_count"]).To(BeEquivalentTo(0))
		})

		It("counts a national ID number and never keeps it", func() {
			raw := caseLayerFrame("Who is 35202-1234567-1?")
			Expect(string(raw)).NotTo(ContainSubstring("35202"))
			var frame map[string]any
			Expect(json.Unmarshal(raw, &frame)).To(Succeed())
			Expect(frame["national_id_count"]).To(BeEquivalentTo(1))
		})
	})

	Describe("the engine adapter", func() {
		It("posts the question to the engine's handler as /query/hybrid would receive it, under the caller's own principal, and reports the engine's phase", func() {
			var seen struct {
				method, path, requestID string
				body                    map[string]any
				principal               forensicPrincipal
			}
			handler := func(w http.ResponseWriter, r *http.Request) {
				seen.method, seen.path, seen.requestID = r.Method, r.URL.Path, r.Header.Get("X-Request-ID")
				raw, _ := io.ReadAll(r.Body)
				Expect(json.Unmarshal(raw, &seen.body)).To(Succeed())
				seen.principal, _ = r.Context().Value(forensicPrincipalContextKey{}).(forensicPrincipal)
				w.Header().Set("X-Request-ID", "engine-request-1")
				writeJSON(w, http.StatusOK, map[string]any{"answer": map[string]any{"answer": "14 calls", "result_state": "COMPLETED"}})
			}
			engine := &caseLayerEngine{handler: handler, version: "round-test"}
			var phases []string
			ctx := context.WithValue(context.Background(), forensicPrincipalContextKey{}, forensicPrincipal{Authenticated: true, TenantID: "default", SubjectID: "analyst-7"})
			ctx = caselayer.WithPhaseEmitter(ctx, func(phase, label string) { phases = append(phases, phase+"|"+label) })
			res, err := engine.Ask(ctx, caselayer.EngineRequest{Scope: caselayer.Scope{TenantID: "default", Subject: "analyst-7"}, CaseID: "orion", Question: "How many calls did 923001234567 make?", RequestID: "turn-1"})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Status).To(Equal(http.StatusOK))
			Expect(res.RequestID).To(Equal("engine-request-1"))
			Expect(res.EngineVersion).To(Equal("round-test"))
			Expect(string(res.Response)).To(ContainSubstring("14 calls"))
			Expect(string(res.Frame)).To(ContainSubstring("923001234567"))
			Expect(seen.method).To(Equal(http.MethodPost))
			Expect(seen.path).To(Equal("/query/hybrid"))
			Expect(seen.requestID).To(Equal("turn-1"))
			Expect(seen.body).To(Equal(map[string]any{"tenant_id": "default", "user_id": "analyst-7", "collection_id": "orion", "query": "How many calls did 923001234567 make?"}))
			Expect(seen.principal.SubjectID).To(Equal("analyst-7"), "the engine sees the same principal as for a direct call")
			Expect(phases).To(Equal([]string{"understanding|Reading the question"}))
		})

		It("passes an engine error through as the engine's own status and body, for the layer to record", func() {
			engine := &caseLayerEngine{handler: func(w http.ResponseWriter, _ *http.Request) {
				writeError(w, http.StatusBadRequest, io.ErrUnexpectedEOF)
			}}
			res, err := engine.Ask(context.Background(), caselayer.EngineRequest{Scope: caselayer.Scope{TenantID: "default", Subject: "s"}, CaseID: "c", Question: "q", RequestID: "t"})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Status).To(Equal(http.StatusBadRequest))
			Expect(string(res.Response)).To(ContainSubstring("unexpected EOF"))
		})
	})

	It("names the build from the environment first", func() {
		GinkgoT().Setenv("FORENSIC_ENGINE_VERSION", "round9e-test")
		Expect(caseLayerEngineVersion()).To(Equal("round9e-test"))
	})

	It("registers every route without a conflict, with no database (they answer 503 then)", func() {
		mux := http.NewServeMux()
		Expect(func() { registerCaseLayer(mux, config{}, nil) }).NotTo(Panic())
		for _, probe := range []struct{ method, path string }{
			{"GET", "/conversations"}, {"GET", "/cases/orion/notes"}, {"GET", "/cases/orion/members"}, {"GET", "/turns/00000000-0000-4000-8000-000000000000"},
		} {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(probe.method, probe.path, nil))
			Expect(rec.Code).To(Equal(http.StatusServiceUnavailable), probe.method+" "+probe.path+" "+strings.TrimSpace(rec.Body.String()))
		}
	})
})

// asCaseLayerError is errors.As for the layer's error type.
func asCaseLayerError(err error, target **caselayer.Error) bool {
	for err != nil {
		if e, ok := err.(*caselayer.Error); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
