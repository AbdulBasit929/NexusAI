package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("R7.9 telecom performance, scope, and provenance hardening", func() {
	It("clamps large result windows and hostile offsets", func() {
		Expect(clampLimit(0)).To(Equal(defaultHybridLimit))
		Expect(clampLimit(maxHybridLimit + 1)).To(Equal(maxHybridLimit))
		Expect(clampOffset(-1)).To(Equal(0))
		Expect(clampOffset(100001)).To(Equal(100000))
	})

	It("keeps tower joins tenant and collection scoped with stable pagination", func() {
		scope := strings.Join(strings.Fields(towerReferenceScopeSQL), " ")
		join := strings.Join(strings.Fields(towerCDRJoinSQL), " ")
		Expect(scope).To(ContainSubstring("tenant_id = $1"))
		Expect(scope).To(ContainSubstring("collection_id = $2"))
		Expect(join).To(ContainSubstring("r.tenant_id = $1"))
		Expect(join).To(ContainSubstring("r.collection_id = $2"))
		Expect(join).To(ContainSubstring("ORDER BY c.cdr_observed_at, c.cdr_source_file, c.cdr_row_number"))
		Expect(join).To(ContainSubstring("LIMIT $6 OFFSET $7"))
	})

	It("publishes exact CDR and reference locators for every tower join row", func() {
		join := strings.Join(strings.Fields(towerCDRJoinSQL), " ")
		for _, field := range []string{
			"cdr_evidence_id", "cdr_source_file", "cdr_row_number", "cdr_row_hash",
			"reference_evidence_id", "reference_source_file", "reference_row_number", "reference_row_hash",
			"coordinate_datum", "uncertainty_radius_m", "match_status",
		} {
			Expect(join).To(ContainSubstring(field), field)
		}
	})

	It("bridges typed tower visualizations into the legacy Ask payload", func() {
		row := map[string]any{
			"cdr_observed_at": "2026-07-21T03:00:00Z", "cell_site_id": "PK-LHR-SYN-001",
			"latitude": 31.5204, "longitude": 74.3587, "coordinate_datum": "WGS84",
			"uncertainty_radius_m": 125, "match_status": "matched",
			"cdr_source_file": "synthetic-cdr.tsv", "cdr_row_number": 2, "cdr_row_hash": "sha256:cdr",
			"reference_source_file": "synthetic-towers.csv", "reference_row_number": 3, "reference_row_hash": "sha256:tower",
		}
		resp := hybridQueryResponse{
			CollectionID: "case-alpha",
			Template:     "tower_cdr_join",
			Records:      map[string]any{"tower_cdr_join": []map[string]any{row}},
			Enterprise: map[string]any{
				"status": "answered_with_limitations",
				"data_grid": map[string]any{
					"columns": []map[string]any{{"key": "cell_site_id", "label": "Cell/site"}},
					"rows":    []map[string]any{row},
				},
				"provenance": []map[string]any{{"source_file": "synthetic-cdr.tsv", "row_number": 2, "row_hash": "sha256:cdr"}},
			},
		}
		attachTypedVisualizationsToLegacyEnterprise(hybridQueryRequest{
			TenantID: "tenant-alpha", CollectionID: "case-alpha", Limit: 20,
		}, "request-r7.9", &resp)

		visualizations, ok := resp.Enterprise["visualizations"].([]VisualizationV1)
		Expect(ok).To(BeTrue())
		Expect(visualizations).To(HaveLen(2))
		Expect(visualizations[0].Type).To(Equal("map"))
		Expect(visualizations[0].Spec).To(HaveKeyWithValue("route_inference", false))
		Expect(visualizations[0].Spec).To(HaveKey("rows"))
		Expect(visualizations[1].Type).To(Equal("timeline"))
	})
})
