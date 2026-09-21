package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("video full-shape result contract", func() {
	It("preserves raw observations, group context, locators and negative targets through enterprise construction", func() {
		req := hybridQueryRequest{TenantID: "synthetic", CollectionID: "synthetic-video-case", EvidenceID: "11111111-1111-4111-8111-111111111111", Target: "11111111-1111-4111-8111-111111111111", Plate: "ABC123", Limit: 10}
		rows := []map[string]any{}
		for i := 1; i <= 2; i++ {
			rows = append(rows, map[string]any{
				"artifact_id": fmt.Sprintf("synthetic-observation-%d", i), "evidence_id": req.EvidenceID, "version_id": "synthetic-version", "source_file": "synthetic-video.mp4",
				"raw_plate_text": "ABC123", "normalized_plate_text": "ABC123", "group_selected_plate_text": "ABC128", "match_kind": "raw_plate_observation",
				"first_seen_seconds": float64(i), "last_seen_seconds": float64(i), "frame_number": i * 60, "sightings_count": 1, "manual_review_required": true,
				"citation_ref":             fmt.Sprintf("nexusai://evidence/%s/artifacts/synthetic-observation-%d", req.EvidenceID, i),
				"citation_locator":         map[string]any{"timestamp_seconds": float64(i), "frame_number": i * 60, "bbox": map[string]any{"x": 10, "y": 20, "width": 30, "height": 40}},
				"best_observation_locator": map[string]any{"timestamp_seconds": float64(i), "bbox": map[string]any{"x": 10}},
			})
		}
		outputs := map[string]any{}
		for _, kind := range []string{"positive", "negative", "group"} {
			selected := rows
			if kind == "negative" {
				req.Plate = "XYZ999"
				selected = []map[string]any{}
			}
			if kind == "group" {
				req.Plate = "ABC128"
				selected = []map[string]any{{"artifact_id": "synthetic-group", "evidence_id": req.EvidenceID, "version_id": "synthetic-version", "source_file": "synthetic-video.mp4", "normalized_plate_text": "ABC128", "group_selected_plate_text": "ABC128", "match_kind": "selected_group_candidate", "first_seen_seconds": 1.0, "last_seen_seconds": 2.0, "sightings_count": 2, "manual_review_required": true, "citation_ref": "nexusai://evidence/synthetic/artifacts/synthetic-group", "group_locator": map[string]any{"first_seen_seconds": 1.0, "last_seen_seconds": 2.0, "bbox": map[string]any{"x": 10}}, "best_observation_locator": map[string]any{"timestamp_seconds": 2.0, "frame_number": 120}}}
			}
			records := videoANPRGroupedTimelineRecords(req.EvidenceID, map[string]any{"processing_status": "completed", "anpr_result_state": "COMPLETE_RESULTS", "version_id": "synthetic-version"}, selected, req)
			resp := hybridQueryResponse{Template: "video_anpr_grouped_timeline", Records: records, Answer: map[string]any{"records_row_count": len(selected)}}
			enterprise := buildEnterprisePayload(req, resp)
			grid := enterprise["data_grid"].(map[string]any)
			actual := grid["rows"].([]map[string]any)
			Expect(actual).To(HaveLen(len(selected)))
			Expect(grid["columns"]).NotTo(BeNil())
			Expect(grid["priority_columns"]).NotTo(BeNil())
			provenance := enterprise["provenance"].([]map[string]any)
			Expect(provenance).To(HaveLen(len(selected)))
			if kind == "negative" {
				Expect(enterprise["executive_answer"]).To(Equal("No exact observation of XYZ999 was found in this video."))
				Expect(enterprise["result_state"]).To(Equal(EnterpriseResultStateNoMatchForFilter))
			} else {
				Expect(grid["priority_columns"]).To(ContainElements("normalized_plate_text", "group_selected_plate_text", "source_time", "source_file"))
				Expect(actual[0]["normalized_plate_text"]).To(Equal(req.Plate))
				Expect(actual[0]["group_selected_plate_text"]).To(Equal("ABC128"))
				Expect(provenance[0]["source_family"]).To(Equal("video_anpr"))
				Expect(provenance[0]["artifact_id"]).To(Equal(selected[0]["artifact_id"]))
				if kind == "positive" {
					Expect(actual[0]["frame_number"]).To(Equal(60))
					Expect(actual[1]["first_seen_seconds"]).To(Equal(2.0))
					Expect(actual[0]["result_semantics"]).To(Equal("video_anpr_observation"))
				} else {
					Expect(actual[0]["result_semantics"]).To(Equal("video_anpr_group"))
					Expect(actual[0]).NotTo(HaveKey("frame_number"))
				}
			}
			resp.Enterprise = enterprise
			req.Template = resp.Template
			resp.CollectionID = req.CollectionID
			resp.ExecutionPlan = &GovernedExecutionPlanV1{PlanID: "synthetic-plan", Steps: []ExecutionStepV1{{StepID: "video", OperationID: "video.anpr_grouped_timeline", ExpectedResultContract: "forensics.enterprise-response/v1", CitationRequired: true}}, ResourcePolicy: ExecutionResourcePolicyV1{MaximumRows: 10}}
			tools, warnings := buildSharedToolResults(req, resp, nil)
			Expect(warnings).To(BeEmpty())
			Expect(tools).To(HaveLen(1))
			Expect(tools[0].Rows).To(HaveLen(len(selected)))
			if len(selected) > 0 {
				Expect(tools[0].Authority).To(Equal(AuthorityModelObservation))
				Expect(tools[0].Observations[0].Observations).To(HaveLen(len(selected)))
				Expect(tools[0].Facts).To(BeEmpty())
				for index, observation := range tools[0].Observations[0].Observations {
					Expect(observation.CitationIDs).To(Equal([]string{fmt.Sprintf("tool-citation-%d", index+1)}))
				}
			}
			public := legacyHybridToEnterpriseV1(req.CollectionID, "synthetic-request", map[string]any{"tenant_id": req.TenantID}, resp)
			Expect(public.ExecutiveAnswer).To(Equal(enterprise["executive_answer"]))
			Expect(public.SemanticEvidence).To(HaveLen(len(selected)))
			if len(selected) > 0 {
				Expect(public.Tables).To(HaveLen(1))
				Expect(public.Tables[0].Rows).To(HaveLen(len(selected)))
				Expect(public.Tables[0].Columns[0].Key).To(Equal("normalized_plate_text"))
				Expect(public.SemanticEvidence[0].Locator).To(ContainSubstring("synthetic-"))
				Expect(citationsFromEnterprise(enterprise, req.TenantID, req.CollectionID)[0].Locator).To(Equal(public.SemanticEvidence[0].Locator))
			}
			outputs[kind] = map[string]any{"template": resp.Template, "enterprise": enterprise}
		}
		Expect(rows[0]["citation_locator"]).NotTo(HaveKey("artifact_id"))
		Expect(videoResultRows(append(rows, rows...))).To(HaveLen(4))
		bounded := make([]map[string]any, 101)
		for i := range bounded {
			bounded[i] = rows[0]
		}
		Expect(videoResultRows(bounded)).To(HaveLen(100))
		wire, err := json.Marshal(rows)
		Expect(err).NotTo(HaveOccurred())
		var decoded any
		Expect(json.Unmarshal(wire, &decoded)).To(Succeed())
		Expect(videoResultRows(decoded)).To(HaveLen(2))
		if directory := os.Getenv("NX_VIDEO_CONTRACT_DIR"); directory != "" {
			Expect(os.MkdirAll(directory, 0700)).To(Succeed())
			data, err := json.Marshal(outputs)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(filepath.Join(directory, "enterprise.json"), data, 0600)).To(Succeed())
		}
	})
})
