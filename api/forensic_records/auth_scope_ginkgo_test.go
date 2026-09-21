package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Phase 3 authenticated forensic request scope", func() {
	const serviceKey = "phase3-test-service-key"

	validRequest := func(method, target string) *http.Request {
		req := httptest.NewRequest(method, target, nil)
		req.Header.Set("Authorization", "Bearer "+serviceKey)
		req.Header.Set(forensicTenantHeader, "tenant-a")
		req.Header.Set(forensicActorHeader, "actor-a")
		req.Header.Set(forensicSubjectHeader, "subject-a")
		req.Header.Set(forensicActorRoleHeader, "user")
		req.Header.Set(forensicCollectionHeader, "case-collection-a")
		req.Header.Set(forensicCaseHeader, "case-a")
		return req
	}

	It("fails configuration closed when authentication is required without a key", func() {
		Expect(validateForensicAuthConfig(forensicAuthConfig{
			Required: true, TrustedTenantID: "tenant-a",
		})).To(MatchError(ContainSubstring("FORENSIC_API_KEY is empty")))
		Expect(validateForensicAuthConfig(forensicAuthConfig{
			APIKey: serviceKey, Required: true, TrustedTenantID: "tenant-a",
		})).To(Succeed())
	})

	It("keeps health public but rejects missing and incorrect bearer credentials", func() {
		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
		handler := forensicAuthMiddleware(forensicAuthConfig{
			APIKey: serviceKey, Required: true, TrustedTenantID: "tenant-a",
		}, next)

		health := httptest.NewRecorder()
		handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		Expect(health.Code).To(Equal(http.StatusNoContent))

		missing := httptest.NewRecorder()
		handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/query/templates", nil))
		Expect(missing.Code).To(Equal(http.StatusUnauthorized))
		Expect(missing.Body.String()).To(ContainSubstring("authentication required"))

		wrongReq := validRequest(http.MethodGet, "/query/templates")
		wrongReq.Header.Set("Authorization", "Bearer incorrect")
		wrong := httptest.NewRecorder()
		handler.ServeHTTP(wrong, wrongReq)
		Expect(wrong.Code).To(Equal(http.StatusUnauthorized))
	})

	It("binds authenticated actor, subject, tenant, collection, and case claims", func() {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scope, err := bindForensicScope(r, "tenant-a", "case-collection-a", "case-a", "subject-a")
			Expect(err).NotTo(HaveOccurred())
			Expect(scope).To(Equal(forensicScope{
				TenantID: "tenant-a", ActorID: "actor-a", SubjectID: "subject-a",
				ActorRole: "user", CollectionID: "case-collection-a", CaseID: "case-a",
			}))
			w.WriteHeader(http.StatusNoContent)
		})
		handler := forensicAuthMiddleware(forensicAuthConfig{
			APIKey: serviceKey, Required: true, TrustedTenantID: "tenant-a",
		}, next)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, validRequest(http.MethodGet, "/evidence"))
		Expect(recorder.Code).To(Equal(http.StatusNoContent))
	})

	It("rejects a tenant claim outside the service token's configured boundary", func() {
		handler := forensicAuthMiddleware(forensicAuthConfig{
			APIKey: serviceKey, Required: true, TrustedTenantID: "tenant-a",
		}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
		req := validRequest(http.MethodGet, "/evidence")
		req.Header.Set(forensicTenantHeader, "tenant-b")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		Expect(recorder.Code).To(Equal(http.StatusForbidden))
	})

	It("rejects caller payload attempts to switch any authenticated scope", func() {
		principal := forensicPrincipal{
			Authenticated: true, TenantID: "tenant-a", ActorID: "actor-a",
			SubjectID: "subject-a", ActorRole: "user",
			CollectionID: "collection-a", CaseID: "case-a",
		}
		req := httptest.NewRequest(http.MethodPost, "/query/hybrid", nil)
		req = req.WithContext(context.WithValue(req.Context(), forensicPrincipalContextKey{}, principal))

		for _, values := range [][4]string{
			{"tenant-b", "collection-a", "case-a", "subject-a"},
			{"tenant-a", "collection-b", "case-a", "subject-a"},
			{"tenant-a", "collection-a", "case-b", "subject-a"},
			{"tenant-a", "collection-a", "case-a", "subject-b"},
		} {
			_, err := bindForensicScope(req, values[0], values[1], values[2], values[3])
			Expect(err).To(MatchError(ContainSubstring("scope is not authorized")))
		}
	})

	It("allows only an authenticated admin to run collection repair", func() {
		for role, allowed := range map[string]bool{"user": false, "agent-worker": false, "admin": true} {
			req := httptest.NewRequest(http.MethodPost, "/collections/repair-assets", nil)
			req = req.WithContext(context.WithValue(req.Context(), forensicPrincipalContextKey{}, forensicPrincipal{
				Authenticated: true, ActorRole: role,
			}))
			err := requireForensicAdmin(req)
			if allowed {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(MatchError(errForensicForbidden))
			}
		}
	})

	It("rejects a hybrid-query body that conflicts with authenticated headers", func() {
		body := []byte(`{"tenant_id":"tenant-b","user_id":"subject-a","collection_id":"case-collection-a","query":"status"}`)
		req := validRequest(http.MethodPost, "/query/hybrid")
		req.Body = http.NoBody
		req.ContentLength = 0
		req = req.WithContext(context.WithValue(req.Context(), forensicPrincipalContextKey{}, forensicPrincipal{
			Authenticated: true, TenantID: "tenant-a", ActorID: "actor-a",
			SubjectID: "subject-a", ActorRole: "user", CollectionID: "case-collection-a",
		}))
		req.Body = ioNopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		recorder := httptest.NewRecorder()
		hybridQueryHandler(config{}, nil).ServeHTTP(recorder, req)
		Expect(recorder.Code).To(Equal(http.StatusForbidden))
	})

	It("wires scope binding into every scoped handler and the main middleware", func() {
		root := filepath.Clean(filepath.Join("..", ".."))
		for _, relative := range []string{
			"api/forensic_records/main.go", "api/forensic_records/collections.go",
			"api/forensic_records/capabilities.go",
			"api/forensic_records/evidence.go", "api/forensic_records/query.go",
			"api/forensic_records/report.go",
		} {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
			Expect(err).NotTo(HaveOccurred())
			text := strings.ReplaceAll(string(data), "\r\n", "\n")
			Expect(text).To(ContainSubstring("bindForensicScope"), relative)
		}
		mainData, err := os.ReadFile(filepath.Join(root, "api", "forensic_records", "main.go"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(mainData)).To(ContainSubstring("forensicAuthMiddleware(authConfig, mux)"))
	})
})

// ioNopCloser keeps this test independent from request-construction details.
func ioNopCloser(reader *bytes.Reader) httpBodyReadCloser {
	return httpBodyReadCloser{Reader: reader}
}

type httpBodyReadCloser struct {
	*bytes.Reader
}

func (httpBodyReadCloser) Close() error { return nil }
