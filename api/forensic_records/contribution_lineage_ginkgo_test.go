package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("APF-3 aggregate contribution lineage", func() {
	It("builds complete, version-aware, bounded contribution proof", func() {
		rows := []map[string]any{
			{
				"result_key": "923009998887", "evidence_id": "evidence-1", "version_id": "version-1", "source_file": "calls-a.csv",
				"contribution_count": int64(40), "first_row": int64(2), "last_row": int64(99), "row_group_digest": strings.Repeat("a", 64),
				"source_group_count": int64(2), "total_contribution_count": int64(68),
			},
			{
				"result_key": "923009998887", "evidence_id": "evidence-2", "version_id": "version-2", "source_file": "calls-b.csv",
				"contribution_count": int64(28), "first_row": int64(1), "last_row": int64(71), "row_group_digest": strings.Repeat("b", 64),
				"source_group_count": int64(2), "total_contribution_count": int64(68),
			},
		}
		lineages := buildContributionLineages("cdr.frequent_contacts", map[string]string{"target": "923001110001"}, rows)
		Expect(lineages).To(HaveLen(1))
		lineage := lineages[0]
		Expect(lineage.ContributionCount).To(Equal(int64(68)))
		Expect(lineage.SourceGroupCount).To(Equal(int64(2)))
		Expect(lineage.SourcesComplete).To(BeTrue())
		Expect(lineage.LineageComplete).To(BeTrue())
		Expect(lineage.LineageDigest).To(HavePrefix("sha256:"))
		Expect(validateContributionLineage(lineage)).To(Succeed())
	})

	It("fails closed when evidence/version identity is missing", func() {
		lineages := buildContributionLineages("cdr.temporal_activity", nil, []map[string]any{{
			"result_key": "", "source_file": "legacy.csv", "contribution_count": int64(4),
			"first_row": int64(2), "last_row": int64(5), "row_group_digest": strings.Repeat("c", 64),
			"source_group_count": int64(1), "total_contribution_count": int64(4),
		}})
		Expect(lineages).To(HaveLen(1))
		Expect(lineages[0].SourcesComplete).To(BeTrue())
		Expect(lineages[0].LineageComplete).To(BeFalse())
		Expect(contributionLineageIncomplete(lineages)).To(BeTrue())
	})

	It("caps source groups without claiming completeness", func() {
		rows := make([]map[string]any, 0, maxContributionSourceGroups)
		for index := 0; index < maxContributionSourceGroups; index++ {
			rows = append(rows, map[string]any{
				"result_key": "", "evidence_id": "e", "version_id": "v", "source_file": "source.csv",
				"contribution_count": int64(1), "first_row": int64(index + 1), "last_row": int64(index + 1),
				"row_group_digest": strings.Repeat("d", 64), "source_group_count": int64(maxContributionSourceGroups + 1),
				"total_contribution_count": int64(maxContributionSourceGroups + 1),
			})
		}
		lineage := buildContributionLineages("cdr.temporal_activity", nil, rows)[0]
		Expect(lineage.Sources).To(HaveLen(maxContributionSourceGroups))
		Expect(lineage.SourcesComplete).To(BeFalse())
		Expect(lineage.LineageComplete).To(BeFalse())
	})

	It("uses grouped SQL rather than transferring raw contribution rows", func() {
		Expect(frequentContactContributionLineageSQL).To(ContainSubstring("GROUP BY"))
		Expect(frequentContactContributionLineageSQL).To(ContainSubstring("source_group_position <= 50"))
		Expect(temporalContributionLineageSQL).To(ContainSubstring("source_group_position <= 50"))
		Expect(frequentContactContributionLineageSQL).ToNot(ContainSubstring("raw_record"))
		Expect(temporalContributionLineageSQL).ToNot(ContainSubstring("raw_record"))
	})
})
