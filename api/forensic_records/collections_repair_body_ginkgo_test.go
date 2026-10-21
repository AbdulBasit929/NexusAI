package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the repair request body", func() {
	base := func() collectionRepairRequest {
		return collectionRepairRequest{collectionScope: collectionScope{TenantID: "t", CollectionID: "c"}, DryRun: true, Limit: 20}
	}

	It("keeps a dry run from the query string when the body does not mention it", func() {
		req := base()
		Expect(applyRepairBody(&req, strings.NewReader("{}"))).To(Succeed())
		Expect(req.DryRun).To(BeTrue())
		Expect(req.Limit).To(Equal(20))
	})

	It("keeps a dry run when there is no body at all", func() {
		req := base()
		Expect(applyRepairBody(&req, strings.NewReader(""))).To(Succeed())
		Expect(req.DryRun).To(BeTrue())
	})

	It("lets the body choose explicitly, in either direction", func() {
		req := base()
		Expect(applyRepairBody(&req, strings.NewReader(`{"dry_run": false, "limit": 5}`))).To(Succeed())
		Expect(req.DryRun).To(BeFalse())
		Expect(req.Limit).To(Equal(5))
		req = base()
		req.DryRun = false
		Expect(applyRepairBody(&req, strings.NewReader(`{"dry_run": true}`))).To(Succeed())
		Expect(req.DryRun).To(BeTrue())
	})

	It("refuses a body that is not JSON and leaves the request as it was", func() {
		req := base()
		Expect(applyRepairBody(&req, strings.NewReader("not json"))).NotTo(Succeed())
		Expect(req.DryRun).To(BeTrue())
	})
})
