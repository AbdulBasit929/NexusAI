package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the case record request", func() {
	It("accepts a name, a status or both, and trims the name", func() {
		patch, err := parseCasePatch(strings.NewReader(`{"display_name": "  Harbour Road  ", "status": "on_hold"}`))
		Expect(err).NotTo(HaveOccurred())
		Expect(patch.HasDisplayName).To(BeTrue())
		Expect(*patch.DisplayName).To(Equal("Harbour Road"))
		Expect(*patch.Status).To(Equal("on_hold"))
	})

	It("treats null and a blank name as clearing the name", func() {
		for _, body := range []string{`{"display_name": null}`, `{"display_name": "   "}`} {
			patch, err := parseCasePatch(strings.NewReader(body))
			Expect(err).NotTo(HaveOccurred())
			Expect(patch.HasDisplayName).To(BeTrue())
			Expect(patch.ClearName).To(BeTrue())
			Expect(patch.DisplayName).To(BeNil())
		}
	})

	It("accepts a hub threshold on its own, at both bounds, and null clears it", func() {
		for _, n := range []int{2, 10, 1000} {
			patch, err := parseCasePatch(strings.NewReader(fmt.Sprintf(`{"hub_threshold": %d}`, n)))
			Expect(err).NotTo(HaveOccurred())
			Expect(patch.HasHubThreshold).To(BeTrue())
			Expect(*patch.HubThreshold).To(Equal(n))
			Expect(patch.HasDisplayName).To(BeFalse())
		}
		patch, err := parseCasePatch(strings.NewReader(`{"hub_threshold": null}`))
		Expect(err).NotTo(HaveOccurred())
		Expect(patch.HasHubThreshold).To(BeTrue())
		Expect(patch.HubThreshold).To(BeNil())
		both, err := parseCasePatch(strings.NewReader(`{"status": "open", "hub_threshold": 10}`))
		Expect(err).NotTo(HaveOccurred())
		Expect(*both.HubThreshold).To(Equal(10))
		Expect(*both.Status).To(Equal("open"))
	})

	It("leaves the name alone when the body does not mention it", func() {
		patch, err := parseCasePatch(strings.NewReader(`{"status": "closed"}`))
		Expect(err).NotTo(HaveOccurred())
		Expect(patch.HasDisplayName).To(BeFalse())
	})

	DescribeTable("refuses what the table would refuse, with a sentence",
		func(body, fragment string) {
			_, err := parseCasePatch(strings.NewReader(body))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fragment))
		},
		Entry("an empty object", `{}`, "nothing to change"),
		Entry("a status that does not exist", `{"status": "archived"}`, "open, on_hold, closed"),
		Entry("a name that is too long", `{"display_name": "`+strings.Repeat("x", 121)+`"}`, "longer than 120"),
		Entry("a name with a line break", `{"display_name": "two\nlines"}`, "control characters"),
		Entry("a field that cannot be changed", `{"created_by": "someone"}`, "only display_name, status and hub_threshold"),
		Entry("a hub threshold below the bound", `{"hub_threshold": 1}`, "from 2 to 1000"),
		Entry("a hub threshold above the bound", `{"hub_threshold": 1001}`, "from 2 to 1000"),
		Entry("a hub threshold that is not a whole number", `{"hub_threshold": 12.5}`, "whole number"),
		Entry("a hub threshold that is text", `{"hub_threshold": "ten"}`, "whole number"),
		Entry("a name that is not text", `{"display_name": 5}`, "text or null"),
		Entry("a body that is not an object", `[1]`, "JSON object"),
	)
})

// Run against the throwaway database of tools/case_generator/check_ask_engine.py as the application role forensic_runtime, so the grants and the tenant policy are exercised.
var _ = Describe("Ask engine executor on a case database, the case record", Ordered, func() {
	var (
		pool *pgxpool.Pool
		ctx  = context.Background()
	)

	BeforeAll(func() {
		dsn := os.Getenv("ASK_ENGINE_CASE_DSN") //nolint:forbidigo // opt-in switch for a throwaway-database spec
		if dsn == "" {
			Skip("requires the throwaway database of tools/case_generator/check_ask_engine.py")
		}
		cfg, err := pgxpool.ParseConfig(dsn)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.ConnConfig.Host).To(Equal("127.0.0.1"))
		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		Expect(err).NotTo(HaveOccurred())
		var marker bool
		Expect(pool.QueryRow(ctx, "SELECT to_regclass('public.nx_throwaway_marker') IS NOT NULL").Scan(&marker)).To(Succeed())
		Expect(marker).To(BeTrue(), "this is not a throwaway database")
	})

	AfterAll(func() {
		if pool != nil {
			pool.Close()
		}
	})

	var caseID, tenant string
	BeforeAll(func() {
		if pool == nil {
			return
		}
		raw, err := os.ReadFile(os.Getenv("ASK_ENGINE_CASE_TRUTH")) //nolint:forbidigo // set by the harness only
		Expect(err).NotTo(HaveOccurred())
		var truth askTruth
		Expect(json.Unmarshal(raw, &truth)).To(Succeed())
		tenant, caseID = truth.Tenant, truth.CaseID
		Expect(tenant).NotTo(BeEmpty())
		Expect(caseID).NotTo(BeEmpty())
	})

	It("lists the collection of the case as an open case that has no record yet", func() {
		list, err := listCaseRecords(ctx, pool, tenant, caseID)
		Expect(err).NotTo(HaveOccurred())
		Expect(list).To(HaveLen(1))
		Expect(list[0].CaseID).To(Equal(caseID))
		Expect(list[0].Recorded).To(BeFalse())
		Expect(list[0].Status).To(Equal("open"))
		Expect(list[0].DisplayName).To(BeNil())
		Expect(list[0].EvidenceCount).To(BeNumerically(">", 0))
	})

	It("records a name and a status, keeps the creator unguessed, and writes one audit row", func() {
		name := "Harbour Road"
		status := "on_hold"
		rec, err := saveCaseRecord(ctx, pool, tenant, caseID, "", casePatch{DisplayName: &name, HasDisplayName: true, Status: &status})
		Expect(err).NotTo(HaveOccurred())
		Expect(rec.Recorded).To(BeTrue())
		Expect(*rec.DisplayName).To(Equal("Harbour Road"))
		Expect(rec.Status).To(Equal("on_hold"))
		Expect(rec.CreatedBy).To(BeNil(), "an unknown creator stays unknown")
		tx, err := pool.Begin(ctx)
		Expect(err).NotTo(HaveOccurred())
		defer tx.Rollback(ctx) //nolint:errcheck // read only
		_, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenant)
		Expect(err).NotTo(HaveOccurred())
		var audits int
		Expect(tx.QueryRow(ctx, "SELECT count(*) FROM forensic.records_audit_log WHERE action = $1 AND collection_id = $2", caseAuditAction, caseID).Scan(&audits)).To(Succeed())
		Expect(audits).To(Equal(1))
	})

	It("changes only what the patch states, and clears a name", func() {
		closed := "closed"
		rec, err := saveCaseRecord(ctx, pool, tenant, caseID, "analyst-1", casePatch{Status: &closed})
		Expect(err).NotTo(HaveOccurred())
		Expect(rec.Status).To(Equal("closed"))
		Expect(*rec.DisplayName).To(Equal("Harbour Road"), "the name was not part of the patch")
		Expect(*rec.UpdatedBy).To(Equal("analyst-1"))
		rec, err = saveCaseRecord(ctx, pool, tenant, caseID, "analyst-1", casePatch{HasDisplayName: true, ClearName: true})
		Expect(err).NotTo(HaveOccurred())
		Expect(rec.DisplayName).To(BeNil())
		Expect(rec.Status).To(Equal("closed"))
	})

	// Migration 032: the case's own hub threshold. The engine reads it per question (readAskCaseSettings) and the answer says where the number came from.
	It("records a hub threshold, reads it back for the engine, leaves it alone when the patch does not mention it, and clears it", func() {
		ten := 10
		rec, err := saveCaseRecord(ctx, pool, tenant, caseID, "analyst-1", casePatch{HubThreshold: &ten, HasHubThreshold: true})
		Expect(err).NotTo(HaveOccurred())
		Expect(*rec.HubThreshold).To(Equal(10))
		setting, err := readAskCaseSettings(ctx, pool, tenant, caseID)
		Expect(err).NotTo(HaveOccurred())
		Expect(setting.HubThreshold).To(Equal(10))
		open := "open"
		rec, err = saveCaseRecord(ctx, pool, tenant, caseID, "analyst-1", casePatch{Status: &open})
		Expect(err).NotTo(HaveOccurred())
		Expect(*rec.HubThreshold).To(Equal(10), "a patch that does not mention it leaves it")
		other, err := readAskCaseSettings(ctx, pool, "another-tenant", caseID)
		Expect(err).NotTo(HaveOccurred())
		Expect(other.HubThreshold).To(Equal(0), "another tenant's setting is not seen")
		rec, err = saveCaseRecord(ctx, pool, tenant, caseID, "analyst-1", casePatch{HasHubThreshold: true})
		Expect(err).NotTo(HaveOccurred())
		Expect(rec.HubThreshold).To(BeNil())
		setting, err = readAskCaseSettings(ctx, pool, tenant, caseID)
		Expect(err).NotTo(HaveOccurred())
		Expect(setting.HubThreshold).To(Equal(0))
		_, err = pool.Exec(ctx, "UPDATE forensic.case_record SET hub_threshold = 1 WHERE tenant_id = $1 AND collection_id = $2", tenant, caseID)
		Expect(err).To(MatchError(ContainSubstring("case_record_hub_threshold_chk")), "the table refuses a threshold outside 2 to 1000")
	})

	It("refuses a case that has no evidence and no record, and never shows another tenant's record", func() {
		name := "Nobody's"
		_, err := saveCaseRecord(ctx, pool, tenant, "no-such-case-anywhere", "", casePatch{DisplayName: &name, HasDisplayName: true})
		Expect(err).To(MatchError(errCaseNotFound))
		other, err := listCaseRecords(ctx, pool, "another-tenant", caseID)
		Expect(err).NotTo(HaveOccurred())
		Expect(other).To(BeEmpty())
	})

	It("cannot delete a case record as the application role", func() {
		_, err := pool.Exec(ctx, "DELETE FROM forensic.case_record WHERE collection_id = $1", caseID)
		Expect(err).To(HaveOccurred())
	})
})
