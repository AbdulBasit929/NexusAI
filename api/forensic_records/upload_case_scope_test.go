package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// THE 403 AN ANALYST SEES WHEN ADDING EVIDENCE, reproduced without uploading
// anything.
//
// Reported 2026-09-27 from the analyst workspace: "This workspace is not
// authorized to add evidence to this case. Reference: HTTP-403".
//
// `uploadHandler` (main.go) binds `case_id` FROM THE MULTIPART FORM, while the
// principal's authorised case comes only from the `X-Forensic-Case-ID` HEADER.
// The workspace sends the form value and no such header, so `bindForensicScope`
// compares a non-empty supplied case against an empty authorised case and
// refuses.
//
// The refusal is CORRECT -- headers carry the authorisation and the body may
// only echo it, which is what keeps a body value from widening scope. The
// defect is that the caller declared a scope it had not been granted. This test
// pins both halves so neither side can drift.
func principalRequest(principal forensicPrincipal) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/webhooks/records/upload", nil)
	return request.WithContext(context.WithValue(request.Context(), forensicPrincipalContextKey{}, principal))
}

func TestUploadCaseScopeRequiresADeclaredCaseHeader(t *testing.T) {
	authorized := forensicPrincipal{
		Authenticated: true,
		TenantID:      "default",
		ActorID:       "investigation-workspace",
		SubjectID:     "investigation-workspace",
		ActorRole:     "user",
		CollectionID:  "nexusai-forensic-demo",
		// No CaseID: the workspace sends no X-Forensic-Case-ID header.
	}

	t.Run("supplying case_id without the header is refused", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		_, err := bindForensicScope(
			principalRequest(authorized),
			"default", "nexusai-forensic-demo", "nexusai-forensic-demo", "investigation-workspace")
		if err == nil {
			t.Fatal("expected a forbidden scope error; the analyst-visible 403 would not reproduce") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if !errors.Is(err, errForensicForbidden) {
			t.Fatalf("expected errForensicForbidden, got %v", err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	// The fix, from the caller's side: declare the case in the header too. The
	// server is unchanged, so nothing about the authorisation boundary moves.
	t.Run("declaring the case in the header is accepted", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		declared := authorized
		declared.CaseID = "nexusai-forensic-demo"
		scope, err := bindForensicScope(
			principalRequest(declared),
			"default", "nexusai-forensic-demo", "nexusai-forensic-demo", "investigation-workspace")
		if err != nil {
			t.Fatalf("expected the scope to bind once the case is declared, got %v", err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if scope.CaseID != "nexusai-forensic-demo" || scope.CollectionID != "nexusai-forensic-demo" {
			t.Fatalf("scope did not carry the declared case and collection: %+v", scope) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	// Omitting the body value is the other way through, and must stay working:
	// an absent supplied value is not a mismatch, it is simply unscoped.
	t.Run("omitting case_id entirely is accepted", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		if _, err := bindForensicScope(
			principalRequest(authorized),
			"default", "nexusai-forensic-demo", "", "investigation-workspace"); err != nil {
			t.Fatalf("an absent case must not be treated as a mismatch, got %v", err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}
