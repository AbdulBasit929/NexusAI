package caselayer

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCaseLayer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Case layer")
}
