package agents

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("forensic presentation contract", func() {
	It("renders aggregate contribution proof as an evidence source", func() {
		citations := presentationCitations([]any{map[string]any{
			"source": "records_aggregate", "contribution_count": int64(68), "source_group_count": int64(2),
			"lineage_complete": true, "result_key": "923009998887", "lineage_digest": "sha256:proof",
		}})
		Expect(citations).To(HaveLen(1))
		citation := citations[0].(map[string]any)
		Expect(citation["label"]).To(Equal("Aggregate contribution lineage"))
		Expect(citation["detail"]).To(ContainSubstring("68 contributing rows"))
		Expect(citation["detail"]).To(ContainSubstring("complete lineage"))
		Expect(citation["lineage_digest"]).To(Equal("sha256:proof"))
	})

	It("decodes structured source-time locators for analyst navigation", func() {
		citations := presentationCitations([]any{map[string]any{
			"citation_id": "C1", "source_file": "recording.wav", "evidence_id": "evidence-1",
			"locator": `{"start_seconds":4.5,"end_seconds":10.25,"artifact_id":"segment-1"}`,
		}})
		Expect(citations).To(HaveLen(1))
		locator := citations[0].(map[string]any)["locator"].(map[string]any)
		Expect(locator).To(HaveKeyWithValue("start_seconds", 4.5))
		Expect(locator).To(HaveKeyWithValue("end_seconds", 10.25))
		Expect(locator).To(HaveKeyWithValue("artifact_id", "segment-1"))
	})

	It("emits bounded typed sections with explicit execution authority", func() {
		rows := make([]any, 25)
		for index := range rows {
			rows[index] = map[string]any{"target": "synthetic", "rank": index + 1}
		}
		metadata := buildForensicPresentationMetadata(forensicDirectResult{
			ToolName: "forensic_hybrid_query",
			Elapsed:  1250 * time.Millisecond,
			Raw: map[string]any{"enterprise": map[string]any{
				"summary":              "Exact synthetic result",
				"operation":            map[string]any{"operation_id": "cdr.frequent_contacts", "family_id": "communications_cdr", "source_access": "records"},
				"metrics":              []any{map[string]any{"label": "Records", "value": 25}},
				"data_grid":            map[string]any{"columns": []any{"target", "rank"}, "rows": rows, "count": 25},
				"limitations":          []any{"Synthetic acceptance data only"},
				"recommended_actions":  []any{"show source evidence"},
				"conversation_context": map[string]any{"target": "923001110001", "template": "frequent_contacts"},
			}},
		})
		presentation := metadata["presentation"].(map[string]any)
		Expect(presentation["contract_version"]).To(Equal(forensicPresentationContract))
		Expect(presentation["execution_authority"]).To(Equal("deterministic_records"))
		Expect(presentation["model_status"]).To(Equal("not_used"))
		Expect(presentation["elapsed_ms"]).To(Equal(int64(1250)))
		Expect(presentation["executive_answer"]).To(Equal("Exact synthetic result"))
		Expect(presentation["operation_id"]).To(Equal("cdr.frequent_contacts"))
		Expect(presentation["specialist"]).To(Equal("Communications_CDR_Analyst"))
		Expect(presentation["source_access"]).To(Equal("records"))
		Expect(presentation["table"].(map[string]any)["rows"]).To(HaveLen(20))
		Expect(presentation["next_actions"]).To(ConsistOf("show source evidence"))
		Expect(presentation["context"]).To(Equal(map[string]any{"target": "923001110001", "template": "frequent_contacts"}))
	})

	It("preserves typed result state for live display and retained History", func() {
		metadata := buildForensicPresentationMetadata(forensicDirectResult{
			ToolName: "forensic_hybrid_query",
			Raw: map[string]any{"enterprise": map[string]any{
				"status": "answered", "result_state": "complete_zero_results", "processing_state": "completed", "row_count": 0,
				"summary":   "ANPR processing complete, 0 plate groups detected.",
				"operation": map[string]any{"operation_id": "video.anpr_grouped_timeline", "family_id": "anpr_vehicles"},
			}},
		})
		presentation := metadata["presentation"].(map[string]any)
		Expect(presentation).To(HaveKeyWithValue("result_state", "complete_zero_results"))
		Expect(presentation).To(HaveKeyWithValue("processing_state", "completed"))
		Expect(presentation).To(HaveKeyWithValue("row_count", 0))
		Expect(presentation).To(HaveKeyWithValue("executive_answer", "ANPR processing complete, 0 plate groups detected."))
	})

	It("prefers the metadata-aware callback without duplicating the message", func() {
		plainCalls, metadataCalls := 0, 0
		callbacks := Callbacks{
			OnMessage: func(_, _, _ string) { plainCalls++ },
			OnMessageMetadata: func(_, _, _ string, metadata map[string]any) {
				metadataCalls++
				Expect(metadata).To(HaveKey("presentation"))
			},
		}
		_, err := finishDeterministicForensicRoute(nil, forensicDirectResult{ToolName: "forensic_hybrid_query", Text: "result"}, nil, callbacks, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(metadataCalls).To(Equal(1))
		Expect(plainCalls).To(Equal(0))
	})

	It("keeps technical metrics and raw provenance out of the analyst view", func() {
		metadata := buildForensicPresentationMetadata(forensicDirectResult{
			ToolName: "forensic_hybrid_query",
			Raw: map[string]any{
				"template": "source_file_audit",
				"enterprise": map[string]any{
					"status":           "answered",
					"executive_answer": "**Deterministic Findings** 10 source files are registered.",
					"metrics": []any{
						map[string]any{"label": "Template", "value": "source_file_audit"},
						map[string]any{"label": "Evidence Count"},
						map[string]any{"label": "Records Row Count", "value": 10},
					},
					"provenance": []any{map[string]any{
						"source": "records_sql", "source_file": "cdr.tsv", "record_type": "cdr", "row_number": 7,
					}},
				},
			},
		})
		presentation := metadata["presentation"].(map[string]any)
		Expect(presentation["executive_answer"]).To(Equal("10 source files are registered."))
		Expect(presentation["metrics"]).To(ConsistOf(map[string]any{"label": "Source files", "value": 10}))
		Expect(presentation["citations"]).To(ConsistOf(map[string]any{
			"label": "cdr.tsv", "detail": "CDR evidence · row 7", "source_file": "cdr.tsv", "evidence_id": nil,
		}))
		trace := presentation["trace"].(map[string]any)
		Expect(trace).To(HaveKey("raw_metrics"))
		Expect(trace).To(HaveKey("raw_provenance"))
	})

	It("retains enterprise semantic evidence as history-ready citations", func() {
		metadata := buildForensicPresentationMetadata(forensicDirectResult{
			ToolName: "forensic_hybrid_query",
			Raw: map[string]any{"enterprise": map[string]any{
				"executive_answer": "A cited document passage matched.",
				"semantic_evidence": []any{map[string]any{
					"id": "citation-document-1", "source_name": "policy.pdf",
					"evidence_id": "evidence-document-1", "version_id": "version-document-1",
					"locator": "page:2 passage:1",
				}},
			}},
		})
		presentation := metadata["presentation"].(map[string]any)
		Expect(presentation["citations"]).To(ConsistOf(map[string]any{
			"label": "policy.pdf", "detail": "page:2 passage:1", "source_file": "policy.pdf",
			"evidence_id": "evidence-document-1", "version_id": "version-document-1",
			"citation_id": "citation-document-1", "proof_role": "", "completeness": "",
			"locator": "page:2 passage:1",
		}))
	})

	It("preserves bounded scalar values inside telecom visualization rows", func() {
		metadata := buildForensicPresentationMetadata(forensicDirectResult{
			ToolName: "forensic_hybrid_query",
			Raw: map[string]any{"enterprise": map[string]any{
				"visualizations": []any{map[string]any{
					"id": "tower-reference-map", "type": "map", "title": "Tower references",
					"citation_ids": []any{"citation-1"},
					"spec": map[string]any{"uncertainty_field": "uncertainty_radius_m", "rows": []any{map[string]any{
						"latitude": 31.5204, "longitude": 74.3587, "uncertainty_radius_m": 125,
						"match_status": "ambiguous_overlapping_references", "source_file": "synthetic-towers.csv", "row_number": 2,
						"row_hash": "sha256:synthetic-row", "evidence_id": "evidence-synthetic", "coordinate_datum": "WGS84",
					}}},
				}},
			}},
		})
		visualizations := metadata["presentation"].(map[string]any)["visualizations"].([]any)
		row := visualizations[0].(map[string]any)["spec"].(map[string]any)["rows"].([]any)[0].(map[string]any)
		Expect(row).To(HaveKeyWithValue("latitude", 31.5204))
		Expect(row).To(HaveKeyWithValue("match_status", "ambiguous_overlapping_references"))
		Expect(row).To(HaveKeyWithValue("source_file", "synthetic-towers.csv"))
		Expect(row).To(HaveKeyWithValue("row_number", 2))
		Expect(row).To(HaveKeyWithValue("row_hash", "sha256:synthetic-row"))
		Expect(row).To(HaveKeyWithValue("evidence_id", "evidence-synthetic"))
		Expect(row).To(HaveKeyWithValue("coordinate_datum", "WGS84"))
	})

	It("labels temporal event totals separately from calculation components", func() {
		metadata := buildForensicPresentationMetadata(forensicDirectResult{
			ToolName: "forensic_hybrid_query",
			Raw: map[string]any{
				"template": "temporal_activity",
				"enterprise": map[string]any{
					"metrics": []any{map[string]any{"label": "Records Row Count", "value": 4}},
					"data_grid": map[string]any{
						"title": "Temporal calculation components", "count": 6, "count_label": "calculation components",
						"rows": []any{
							map[string]any{"section": "hourly_activity", "hour_of_day": 3, "event_count": 4},
							map[string]any{"section": "duration_extremes", "metric": "shortest_nonzero_call", "duration_seconds": 10, "observed_at": "2026-07-10T03:00:00Z"},
						},
					},
				},
			},
		})
		presentation := metadata["presentation"].(map[string]any)
		Expect(presentation["metrics"]).To(ConsistOf(map[string]any{"label": "Matched CDR events", "value": 4}))
		table := presentation["table"].(map[string]any)
		Expect(table).To(HaveKeyWithValue("title", "Temporal calculation components"))
		Expect(table).To(HaveKeyWithValue("count_label", "calculation components"))
		Expect(table["columns"]).To(ContainElements("section", "hour_of_day", "event_count", "metric", "duration_seconds", "observed_at"))
	})

	It("preserves Fact Packet proof semantics and exposes all governed table columns progressively", func() {
		row := map[string]any{"target": "923001110001", "event_count": 3, "direction": "outgoing", "duration_seconds": 60, "observed_at": "2026-08-17T08:30:00+05:00", "source_file": "cdr.csv", "row_number": 7, "evidence_id": "evidence-1", "version_id": "version-1"}
		metadata := buildForensicPresentationMetadata(forensicDirectResult{ToolName: "forensic_hybrid_query", Raw: map[string]any{"enterprise": map[string]any{
			"status": "answered_with_limitations", "executive_answer": "Three exact outgoing events were returned.",
			"data_grid": map[string]any{"rows": []any{row}, "columns": []any{"target", "event_count", "direction", "duration_seconds", "observed_at", "source_file", "row_number", "evidence_id", "version_id"}},
			"fact_packet": map[string]any{
				"language": map[string]any{"tag": "ur"}, "presentation_hints": map[string]any{"direction": "rtl", "priority_columns": []any{"target", "event_count"}},
				"relationships": []any{map[string]any{"relationship_id": "R1", "type": "observed_association"}},
				"citations":     []any{map[string]any{"citation_id": "C1", "source_file": "cdr.csv", "proof_role": "aggregate_contribution_lineage", "proof_completeness": "complete", "evidence_id": "evidence-1", "version_id": "version-1"}},
			},
			"narrative": map[string]any{"status": "model_unavailable_fallback", "fallback": true, "key_findings": []any{map[string]any{"text": "Three exact outgoing events were returned.", "claim_type": "deterministic_fact"}}},
		}}})
		presentation := metadata["presentation"].(map[string]any)
		Expect(presentation).To(HaveKeyWithValue("language", "ur"))
		Expect(presentation).To(HaveKeyWithValue("text_direction", "rtl"))
		Expect(presentation).To(HaveKeyWithValue("model_status", "fallback"))
		Expect(presentation["relationships"]).To(HaveLen(1))
		table := presentation["table"].(map[string]any)
		Expect(table["columns"]).To(HaveLen(9))
		Expect(table["priority_columns"]).To(ConsistOf("target", "event_count"))
		citation := presentation["citations"].([]any)[0].(map[string]any)
		Expect(citation).To(HaveKeyWithValue("citation_id", "C1"))
		Expect(citation).To(HaveKeyWithValue("proof_role", "aggregate_contribution_lineage"))
		Expect(citation).To(HaveKeyWithValue("completeness", "complete"))
	})
})
