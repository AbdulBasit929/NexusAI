package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type modalityEvaluationMatrix struct {
	RequiredLifecycle  []string                    `json:"required_lifecycle"`
	RequiredProvenance []string                    `json:"required_provenance"`
	RequiredFamilies   []string                    `json:"required_families"`
	MetricRegistry     map[string]string           `json:"metric_registry"`
	Profiles           []modalityEvaluationProfile `json:"profiles"`
}

type modalityEvaluationProfile struct {
	ID                      string                     `json:"id"`
	Formats                 []string                   `json:"formats"`
	SupportLevel            string                     `json:"support_level"`
	Fixture                 modalityEvaluationFixture  `json:"fixture"`
	Classifier              modalityClassifierContract `json:"classifier"`
	Adapter                 string                     `json:"adapter"`
	AuthoritativeStore      string                     `json:"authoritative_store"`
	DeterministicOperations []string                   `json:"deterministic_operations"`
	SemanticOperations      []string                   `json:"semantic_operations"`
	Entities                []string                   `json:"entities"`
	Relationships           []string                   `json:"relationships"`
	Metrics                 []modalityEvaluationMetric `json:"metrics"`
	FailureBehavior         string                     `json:"failure_behavior"`
	NextGate                string                     `json:"next_gate"`
}

type modalityEvaluationFixture struct {
	State  string `json:"state"`
	Path   string `json:"path"`
	Golden string `json:"golden"`
}

type modalityClassifierContract struct {
	Filename     string            `json:"filename"`
	ContentType  string            `json:"content_type"`
	Headers      []string          `json:"headers"`
	Hints        map[string]string `json:"hints"`
	Modality     string            `json:"modality"`
	DetectedType string            `json:"detected_type"`
	Route        string            `json:"route"`
	QueueRecords bool              `json:"queue_records"`
}

type modalityEvaluationMetric struct {
	Name      string  `json:"name"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
}

var _ = Describe("Forensic modality evaluation matrix", func() {
	It("covers every required evidence family with an executable classifier and acceptance contract", func() {
		repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
		matrixPath := filepath.Join(repositoryRoot, "configuration", "forensic_modality_evaluation_matrix.json")
		data, err := os.ReadFile(matrixPath)
		Expect(err).NotTo(HaveOccurred())

		var matrix modalityEvaluationMatrix
		Expect(json.Unmarshal(data, &matrix)).To(Succeed())
		Expect(matrix.RequiredLifecycle).To(ContainElements(
			"register", "classify", "validate", "profile", "extract", "normalize",
			"store", "index", "record_provenance", "surface_status", "query", "benchmark",
		))
		Expect(matrix.RequiredProvenance).To(ContainElements(
			"tenant_id", "collection_id", "evidence_id", "version_id", "source_file",
			"source_entry", "sha256", "processing_route", "processing_status",
			"adapter_identity", "backend_model_version", "warnings", "errors",
		))
		Expect(matrix.Profiles).To(HaveLen(len(matrix.RequiredFamilies)))

		profilesByID := make(map[string]modalityEvaluationProfile, len(matrix.Profiles))
		for _, profile := range matrix.Profiles {
			Expect(profile.ID).NotTo(BeEmpty())
			Expect(profilesByID).NotTo(HaveKey(profile.ID), "duplicate modality profile %q", profile.ID)
			profilesByID[profile.ID] = profile

			Expect(profile.Formats).NotTo(BeEmpty(), profile.ID)
			Expect([]string{"operational", "limited", "foundation", "planned"}).To(ContainElement(profile.SupportLevel), profile.ID)
			Expect(profile.Adapter).NotTo(BeEmpty(), profile.ID)
			Expect(profile.AuthoritativeStore).NotTo(BeEmpty(), profile.ID)
			Expect(profile.DeterministicOperations).NotTo(BeEmpty(), profile.ID)
			Expect(profile.SemanticOperations).NotTo(BeEmpty(), profile.ID)
			Expect(profile.Entities).NotTo(BeEmpty(), profile.ID)
			Expect(profile.Relationships).NotTo(BeEmpty(), profile.ID)
			Expect(profile.FailureBehavior).NotTo(BeEmpty(), profile.ID)
			Expect(profile.NextGate).NotTo(BeEmpty(), profile.ID)

			Expect([]string{"ready", "planned"}).To(ContainElement(profile.Fixture.State), profile.ID)
			Expect(profile.Fixture.Path).NotTo(BeEmpty(), profile.ID)
			Expect(profile.Fixture.Golden).NotTo(BeEmpty(), profile.ID)
			if profile.Fixture.State == "ready" {
				Expect(filepath.Join(repositoryRoot, filepath.FromSlash(profile.Fixture.Path))).To(BeAnExistingFile(), profile.ID)
			}
			if profile.SupportLevel == "operational" {
				Expect(profile.Fixture.State).To(Equal("ready"), profile.ID)
				Expect(profile.Classifier.QueueRecords).To(BeTrue(), profile.ID)
				Expect(profile.Classifier.Route).NotTo(ContainSubstring("pending"), profile.ID)
			}
			if profile.SupportLevel == "planned" {
				Expect(profile.Classifier.QueueRecords).To(BeFalse(), profile.ID)
			}

			contract := profile.Classifier
			classification := classifyEvidenceItem(
				contract.Filename,
				contract.ContentType,
				detectRecordType(contract.Headers),
				contract.Headers,
				contract.Hints,
			)
			Expect(classification.Modality).To(Equal(contract.Modality), profile.ID)
			Expect(classification.DetectedType).To(Equal(contract.DetectedType), profile.ID)
			Expect(classification.ProcessingRoute).To(Equal(contract.Route), profile.ID)
			Expect(classification.QueueRecords).To(Equal(contract.QueueRecords), profile.ID)

			Expect(profile.Metrics).NotTo(BeEmpty(), profile.ID)
			for _, metric := range profile.Metrics {
				Expect(matrix.MetricRegistry).To(HaveKey(metric.Name), profile.ID)
				Expect([]string{"<=", ">="}).To(ContainElement(metric.Operator), profile.ID+":"+metric.Name)
				Expect(metric.Threshold).To(BeNumerically(">=", 0), profile.ID+":"+metric.Name)
			}
		}

		seenRequired := map[string]struct{}{}
		for _, family := range matrix.RequiredFamilies {
			family = strings.TrimSpace(family)
			Expect(family).NotTo(BeEmpty())
			Expect(seenRequired).NotTo(HaveKey(family), "duplicate required family %q", family)
			seenRequired[family] = struct{}{}
			Expect(profilesByID).To(HaveKey(family), "missing required modality profile %q", family)
		}
	})

	It("keeps every Phase 2 family backed by a versioned ready fixture", func() {
		repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
		matrixPath := filepath.Join(repositoryRoot, "configuration", "forensic_modality_evaluation_matrix.json")
		data, err := os.ReadFile(matrixPath)
		Expect(err).NotTo(HaveOccurred())
		var matrix modalityEvaluationMatrix
		Expect(json.Unmarshal(data, &matrix)).To(Succeed())
		for _, profile := range matrix.Profiles {
			Expect(profile.Fixture.State).To(Equal("ready"), profile.ID)
			Expect(filepath.Join(repositoryRoot, filepath.FromSlash(profile.Fixture.Path))).To(BeAnExistingFile(), profile.ID)
		}
	})
})
