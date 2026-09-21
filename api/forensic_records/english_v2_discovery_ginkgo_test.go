package main

import (
	"github.com/mudler/LocalAI/core/services/agents"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
)

var _ = Describe("English V2 real discovery contract", func() {
	It("grounds product help using the registered sidecar handlers", func() {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v1/forensics/adapters", forensicPlatformDiscoveryHandler("adapters"))
		mux.HandleFunc("GET /query/templates", queryTemplatesHandler())
		server := httptest.NewServer(mux)
		defer server.Close()
		got := agents.ForensicProductHelpContext(agents.ForensicRecordsToolConfig{APIURL: server.URL})
		Expect(got).To(ContainSubstring(`"adapter_status":"REGISTERED_ONLY"`))
		Expect(got).To(ContainSubstring(`"template_status":"REGISTERED_ONLY"`))
		Expect(got).To(ContainSubstring(`"runtime_availability":"UNKNOWN"`))
	})
})
