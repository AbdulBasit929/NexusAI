package agents

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("video full-shape pipeline bridge", func() {
	It("persists actual enterprise output through agent presentation and retained metadata serialization", func() {
		directory := os.Getenv("NX_VIDEO_CONTRACT_DIR")
		if directory == "" {
			Skip("Run API contract producer with NX_VIDEO_CONTRACT_DIR first")
		}
		data, err := os.ReadFile(filepath.Join(directory, "enterprise.json"))
		Expect(err).NotTo(HaveOccurred())
		var cases map[string]map[string]any
		Expect(json.Unmarshal(data, &cases)).To(Succeed())
		outputs := map[string]any{}
		for _, kind := range []string{"positive", "negative", "group"} {
			metadata := buildForensicPresentationMetadata(forensicDirectResult{Raw: cases[kind], ToolName: "forensic_hybrid_query"})
			serialized, err := json.Marshal(metadata)
			Expect(err).NotTo(HaveOccurred())
			entry := analysisHistoryEntry(AgentAnalysisRecord{ID: "synthetic-" + kind, Status: "completed", AnswerText: "Synthetic retained result", AnswerMetadata: string(serialized)})
			presentation := entry.Metadata["presentation"].(map[string]any)
			table := presentation["table"].(map[string]any)
			citations := presentation["citations"].([]any)
			if kind == "negative" {
				Expect(citations).To(BeEmpty())
				Expect(presentation["executive_answer"]).To(Equal("No exact observation of XYZ999 was found in this video."))
			} else {
				expected := 2
				if kind == "group" {
					expected = 1
				}
				Expect(table["rows"]).To(HaveLen(expected))
				Expect(table["columns"]).To(ContainElements("normalized_plate_text", "source_time", "source_file"))
				Expect(table["priority_columns"]).NotTo(BeNil())
				Expect(citations).To(HaveLen(expected))
				citation := citations[0].(map[string]any)
				Expect(citation["source_file"]).To(Equal("synthetic-video.mp4"))
				Expect(citation["version_id"]).To(Equal("synthetic-version"))
				locator := citation["locator"].(map[string]any)
				Expect(locator["artifact_id"]).NotTo(BeNil())
				if kind == "positive" {
					Expect(locator["timestamp_seconds"]).To(Equal(1.0))
					Expect(locator["frame_number"]).To(Equal(60.0))
				}
			}
			outputs[kind] = entry
		}
		data, err = json.Marshal(outputs)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(directory, "retained.json"), data, 0600)).To(Succeed())
	})
})
