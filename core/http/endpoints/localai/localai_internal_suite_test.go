package localai

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLocalAIInternalEndpoints(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LocalAI internal endpoint test suite")
}
