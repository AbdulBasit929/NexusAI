package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGatewayServesSPAAssetsAndProxiesBackend(t *testing.T) {
	uiRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(uiRoot, "index.html"), []byte("unified-workspace"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(uiRoot, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uiRoot, "assets", "app.js"), []byte("asset-content"), 0o600); err != nil {
		t.Fatal(err)
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "backend:"+r.URL.Path)
	}))
	defer backend.Close()
	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}

	gateway := httptest.NewServer(newGateway(uiRoot, backendURL))
	defer gateway.Close()

	checks := []struct {
		path   string
		accept string
		want   string
	}{
		{path: "/analyst?case=demo", accept: "text/html", want: "unified-workspace"},
		{path: "/assets/app.js", accept: "*/*", want: "asset-content"},
		{path: "/api/forensic/cases", accept: "application/json", want: "backend:/api/forensic/cases"},
		{path: "/unknown-data", accept: "application/json", want: "backend:/unknown-data"},
	}

	for _, check := range checks {
		req, err := http.NewRequest(http.MethodGet, gateway.URL+check.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept", check.accept)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), check.want) {
			t.Fatalf("%s: got %q, want substring %q", check.path, string(body), check.want)
		}
	}
}

func TestRouteToBackendUsesSegmentBoundaries(t *testing.T) {
	if !routeToBackend("/api") || !routeToBackend("/api/forensic") {
		t.Fatal("expected API routes to use the accepted backend")
	}
	if routeToBackend("/apiary") {
		t.Fatal("must not proxy a lookalike SPA route")
	}
}
