package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	forensicTenantHeader     = "X-Forensic-Tenant-ID"
	forensicActorHeader      = "X-Forensic-Actor-ID"
	forensicSubjectHeader    = "X-Forensic-Subject-ID"
	forensicActorRoleHeader  = "X-Forensic-Actor-Role"
	forensicCollectionHeader = "X-Forensic-Collection-ID"
	forensicCaseHeader       = "X-Forensic-Case-ID"
	maxScopeValueLength      = 256
)

var (
	errForensicUnauthenticated = errors.New("forensic API authentication required")
	errForensicForbidden       = errors.New("forensic request scope is not authorized")
)

type forensicAuthConfig struct {
	APIKey          string
	Required        bool
	TrustedTenantID string
}

type forensicPrincipal struct {
	Authenticated bool
	TenantID      string
	ActorID       string
	SubjectID     string
	ActorRole     string
	CollectionID  string
	CaseID        string
}

type forensicScope struct {
	TenantID     string
	ActorID      string
	SubjectID    string
	ActorRole    string
	CollectionID string
	CaseID       string
}

type forensicPrincipalContextKey struct{}

func validateForensicAuthConfig(cfg forensicAuthConfig) error {
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.TrustedTenantID = strings.TrimSpace(cfg.TrustedTenantID)
	if cfg.Required && cfg.APIKey == "" {
		return errors.New("FORENSIC_API_AUTH_REQUIRED is enabled but FORENSIC_API_KEY is empty")
	}
	if cfg.APIKey != "" {
		if err := validateScopeValue("trusted tenant", cfg.TrustedTenantID, true); err != nil {
			return err
		}
	}
	return nil
}

func forensicAuthMiddleware(cfg forensicAuthConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		if strings.TrimSpace(cfg.APIKey) == "" {
			next.ServeHTTP(w, r.WithContext(context.WithValue(
				r.Context(), forensicPrincipalContextKey{}, forensicPrincipal{},
			)))
			return
		}

		providedKey, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok || !constantTimeTokenEqual(providedKey, cfg.APIKey) {
			writeError(w, http.StatusUnauthorized, errForensicUnauthenticated)
			return
		}

		principal := forensicPrincipal{
			Authenticated: true,
			TenantID:      strings.TrimSpace(r.Header.Get(forensicTenantHeader)),
			ActorID:       strings.TrimSpace(r.Header.Get(forensicActorHeader)),
			SubjectID:     strings.TrimSpace(r.Header.Get(forensicSubjectHeader)),
			ActorRole:     strings.ToLower(strings.TrimSpace(r.Header.Get(forensicActorRoleHeader))),
			CollectionID:  strings.TrimSpace(r.Header.Get(forensicCollectionHeader)),
			CaseID:        strings.TrimSpace(r.Header.Get(forensicCaseHeader)),
		}
		for field, value := range map[string]string{
			"tenant": principal.TenantID, "actor": principal.ActorID,
			"subject": principal.SubjectID, "actor role": principal.ActorRole,
		} {
			if err := validateScopeValue(field, value, true); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
		if err := validateScopeValue("collection", principal.CollectionID, false); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if err := validateScopeValue("case", principal.CaseID, false); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if principal.TenantID != strings.TrimSpace(cfg.TrustedTenantID) {
			writeError(w, http.StatusForbidden, errForensicForbidden)
			return
		}
		if principal.ActorRole != "user" && principal.ActorRole != "admin" && principal.ActorRole != "agent-worker" {
			writeError(w, http.StatusForbidden, errForensicForbidden)
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(
			r.Context(), forensicPrincipalContextKey{}, principal,
		)))
	})
}

func bindForensicScope(r *http.Request, tenantID, collectionID, caseID, subjectID string) (forensicScope, error) {
	principal, _ := r.Context().Value(forensicPrincipalContextKey{}).(forensicPrincipal)
	tenantID = strings.TrimSpace(tenantID)
	collectionID = strings.TrimSpace(collectionID)
	caseID = strings.TrimSpace(caseID)
	subjectID = strings.TrimSpace(subjectID)
	if !principal.Authenticated {
		return forensicScope{
			TenantID: defaultString(tenantID, "default"), SubjectID: subjectID,
			CollectionID: collectionID, CaseID: caseID,
		}, nil
	}

	for field, values := range map[string][2]string{
		"tenant":     {tenantID, principal.TenantID},
		"collection": {collectionID, principal.CollectionID},
		"case":       {caseID, principal.CaseID},
		"subject":    {subjectID, principal.SubjectID},
	} {
		supplied, authorized := values[0], values[1]
		if supplied != "" && supplied != authorized {
			return forensicScope{}, fmt.Errorf("%w: %s mismatch", errForensicForbidden, field)
		}
	}

	return forensicScope{
		TenantID: principal.TenantID, ActorID: principal.ActorID,
		SubjectID: principal.SubjectID, ActorRole: principal.ActorRole,
		CollectionID: principal.CollectionID, CaseID: principal.CaseID,
	}, nil
}

func requireForensicAdmin(r *http.Request) error {
	principal, _ := r.Context().Value(forensicPrincipalContextKey{}).(forensicPrincipal)
	if principal.Authenticated && principal.ActorRole != "admin" {
		return errForensicForbidden
	}
	return nil
}

func writeScopeError(w http.ResponseWriter, err error) {
	status := http.StatusForbidden
	if errors.Is(err, errForensicUnauthenticated) {
		status = http.StatusUnauthorized
	}
	writeError(w, status, err)
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(strings.TrimSpace(header))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func constantTimeTokenEqual(provided, expected string) bool {
	providedBytes := []byte(provided)
	expectedBytes := []byte(strings.TrimSpace(expected))
	return len(providedBytes) == len(expectedBytes) &&
		subtle.ConstantTimeCompare(providedBytes, expectedBytes) == 1
}

func validateScopeValue(field, value string, required bool) error {
	value = strings.TrimSpace(value)
	if required && value == "" {
		return fmt.Errorf("%s scope header is required", field)
	}
	if len(value) > maxScopeValueLength {
		return fmt.Errorf("%s scope header exceeds %d bytes", field, maxScopeValueLength)
	}
	if strings.ContainsAny(value, "\r\n\x00") {
		return fmt.Errorf("%s scope header contains control characters", field)
	}
	return nil
}
