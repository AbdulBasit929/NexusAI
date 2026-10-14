package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Typed families in the governed SQL lane", func() {
	var (
		probes   []string
		exists   bool
		clock    time.Time
		original = struct {
			exists func(context.Context, *pgxpool.Pool, string) bool
			now    func() time.Time
		}{typedViewExists, typedViewNow}
	)

	BeforeEach(func() {
		probes, exists, clock = nil, true, time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)
		typedViewExists = func(_ context.Context, _ *pgxpool.Pool, qualified string) bool {
			probes = append(probes, qualified)
			return exists
		}
		typedViewNow = func() time.Time { return clock }
		typedViewMu.Lock()
		typedViewStates = map[string]typedViewState{}
		typedViewMu.Unlock()
	})

	AfterEach(func() {
		typedViewExists, typedViewNow = original.exists, original.now
	})

	// views builds the lane's real views from the real curated layer. The pool is never used: the existence probe is replaced above.
	views := func() map[string]*govSQLView {
		layer, _ := defaultSemanticLayer()
		Expect(layer).NotTo(BeNil())
		built := map[string]*govSQLView{}
		for _, view := range govSQLViews(layer, hybridQueryRequest{TenantID: "t", CollectionID: "c"}) {
			built[view.Family] = view
		}
		return built
	}
	apply := func(built map[string]*govSQLView) {
		list := make([]*govSQLView, 0, len(built))
		for _, view := range built {
			list = append(list, view)
		}
		govSQLApplyTypedFamilies(context.Background(), &pgxpool.Pool{}, list)
	}
	cte := func(view *govSQLView) string {
		sql, _, err := govSQLCTE(view, hybridQueryRequest{TenantID: "t", CollectionID: "c"}, nil)
		Expect(err).NotTo(HaveOccurred())
		return sql
	}

	It("changes nothing while the variable is unset or empty", func() {
		for _, value := range []string{"", "   ", ",,"} {
			GinkgoT().Setenv(typedFamiliesEnv, value)
			built := views()
			apply(built)
			Expect(built).To(HaveKey("communications_cdr"))
			Expect(built["communications_cdr"].binding.Table).To(Equal("forensic.records"))
			Expect(cte(built["communications_cdr"])).To(ContainSubstring("FROM forensic.records r"))
		}
		Expect(probes).To(BeEmpty(), "the database is not even asked")
	})

	It("reads the call records from the effective view when the family is listed and the view exists", func() {
		GinkgoT().Setenv(typedFamiliesEnv, "communications_cdr")
		built := views()
		apply(built)
		Expect(built["communications_cdr"].binding.Table).To(Equal("forensic.records_effective_cdr"))
		sql := cte(built["communications_cdr"])
		Expect(sql).To(ContainSubstring("FROM forensic.records_effective_cdr r WHERE"))
		Expect(sql).To(ContainSubstring("r.tenant_id=$1"))
		Expect(sql).To(ContainSubstring("r.record_type="))
		Expect(sql).To(ContainSubstring("r.record_id::text AS record_id"))
		Expect(sql).NotTo(ContainSubstring("FROM forensic.records r"))
	})

	It("accepts spaces, capitals and other names in the list, and ignores what it does not know", func() {
		GinkgoT().Setenv(typedFamiliesEnv, " Communications_CDR , not_a_family ,, subscriber")
		built := views()
		apply(built)
		Expect(built["communications_cdr"].binding.Table).To(Equal("forensic.records_effective_cdr"))
		for family, view := range built {
			if family != "communications_cdr" {
				Expect(view.binding.Table).NotTo(Equal("forensic.records_effective_cdr"), family)
			}
		}
	})

	It("leaves the call-record family alone when the list names only other families", func() {
		GinkgoT().Setenv(typedFamiliesEnv, "subscriber, ipdr")
		built := views()
		apply(built)
		Expect(built["communications_cdr"].binding.Table).To(Equal("forensic.records"))
		Expect(probes).To(BeEmpty(), "a family that was not asked for is not even probed")
	})

	It("never switches a derived view, even if a table were ever mapped for its family", func() {
		var derived *govSQLView
		built := views()
		for _, view := range built {
			if view.binding.Derived {
				derived = view
				break
			}
		}
		Expect(derived).NotTo(BeNil(), "the layer has derived views")
		typedEffectiveTables[derived.Family] = "forensic.records_effective_test"
		DeferCleanup(func() { delete(typedEffectiveTables, derived.Family) })
		GinkgoT().Setenv(typedFamiliesEnv, derived.Family)
		before := derived.binding.Table
		apply(built)
		Expect(derived.binding.Table).To(Equal(before))
		Expect(probes).To(BeEmpty())
	})

	It("leaves every other family and every derived view on its own table", func() {
		GinkgoT().Setenv(typedFamiliesEnv, "communications_cdr")
		before := views()
		want := map[string]string{}
		for family, view := range before {
			want[family] = view.binding.Table
		}
		built := views()
		apply(built)
		for family, view := range built {
			if family == "communications_cdr" {
				continue
			}
			Expect(view.binding.Table).To(Equal(want[family]), family)
		}
		derived := 0
		for _, view := range built {
			if view.binding.Derived {
				derived++
				Expect(view.binding.Table).To(Equal("forensic.derived_artifacts"))
			}
		}
		Expect(derived).To(BeNumerically(">", 0), "the layer has derived views, and none was switched")
	})

	It("stays on forensic.records, and warns once, while the view does not exist yet", func() {
		GinkgoT().Setenv(typedFamiliesEnv, "communications_cdr")
		exists = false
		built := views()
		apply(built)
		Expect(built["communications_cdr"].binding.Table).To(Equal("forensic.records"))
		Expect(cte(built["communications_cdr"])).To(ContainSubstring("FROM forensic.records r"))
		typedViewMu.Lock()
		Expect(typedViewStates["forensic.records_effective_cdr"].warned).To(BeTrue())
		typedViewMu.Unlock()
	})

	It("asks the database once per half minute, then notices when the view appears", func() {
		GinkgoT().Setenv(typedFamiliesEnv, "communications_cdr")
		exists = false
		for i := 0; i < 5; i++ {
			apply(views())
		}
		Expect(probes).To(HaveLen(1))
		exists = true
		clock = clock.Add(typedViewRecheck - time.Second)
		built := views()
		apply(built)
		Expect(built["communications_cdr"].binding.Table).To(Equal("forensic.records"), "still inside the cached half minute")
		clock = clock.Add(2 * time.Second)
		built = views()
		apply(built)
		Expect(built["communications_cdr"].binding.Table).To(Equal("forensic.records_effective_cdr"))
		Expect(probes).To(HaveLen(2))
		typedViewMu.Lock()
		Expect(typedViewStates["forensic.records_effective_cdr"].warned).To(BeFalse())
		typedViewMu.Unlock()
	})

	It("does nothing without a database", func() {
		GinkgoT().Setenv(typedFamiliesEnv, "communications_cdr")
		built := views()
		list := []*govSQLView{built["communications_cdr"]}
		govSQLApplyTypedFamilies(context.Background(), nil, list)
		Expect(list[0].binding.Table).To(Equal("forensic.records"))
		Expect(probes).To(BeEmpty())
	})

	It("is applied by the lane after the views are built and before the question is matched to them", func() {
		source, err := os.ReadFile("governed_sql_lane.go")
		Expect(err).NotTo(HaveOccurred())
		text := strings.ReplaceAll(string(source), "\r\n", "\n")
		built := strings.Index(text, "all := govSQLViews(layer, req)")
		applied := strings.Index(text, "govSQLApplyTypedFamilies(ctx, db, all)")
		offered := strings.Index(text, "offered := govSQLOffer(all, req.Query)")
		Expect(built).To(BeNumerically(">", 0))
		Expect(applied).To(BeNumerically(">", built))
		Expect(offered).To(BeNumerically(">", applied))
	})

	It("only ever names tables that a migration creates", func() {
		for family, table := range typedEffectiveTables {
			Expect(table).To(MatchRegexp(`^forensic\.records_effective_[a-z_]+$`), family)
		}
	})
})
