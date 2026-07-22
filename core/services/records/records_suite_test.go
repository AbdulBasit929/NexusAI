package records

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRecords(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Records service suite")
}
