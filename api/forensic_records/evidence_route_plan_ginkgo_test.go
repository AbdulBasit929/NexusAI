package main

import (
	"encoding/binary"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func validRoutePNG(width, height uint32) []byte {
	data := make([]byte, 40)
	copy(data, []byte("\x89PNG\r\n\x1a\n"))
	copy(data[12:], []byte("IHDR"))
	binary.BigEndian.PutUint32(data[16:20], width)
	binary.BigEndian.PutUint32(data[20:24], height)
	copy(data[32:], []byte("IEND"))
	return data
}

func validRoutePDF() []byte {
	return []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n%%EOF")
}

func validRouteMP4() []byte {
	return []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0}
}

func validRouteM4A() []byte {
	return []byte{0, 0, 0, 28, 'f', 't', 'y', 'p', 'M', '4', 'A', ' ', 0, 0, 2, 0}
}

var _ = Describe("NX-MMR evidence route plan", func() {
	base := func(filename, declared, detected string, payload []byte) evidenceRouteInput {
		return evidenceRouteInput{
			EvidenceID: "evidence-1", VersionID: "version-1", SourceFilename: filename,
			DeclaredMIME: declared, DetectedMIME: detected, Head: payload, Tail: payload,
			SizeBytes: int64(len(payload)), ScopeAuthorized: true,
		}
	}

	DescribeTable("fails closed on the sealed routing negative matrix",
		func(input evidenceRouteInput, state evidenceRouteState, family string) {
			plan := buildEvidenceRoutePlan(input)
			Expect(plan.PlanVersion).To(Equal(evidenceRoutePlanVersion))
			Expect(plan.PlanID).To(HavePrefix("erp-"))
			Expect(plan.ExecutionAdmission).To(Equal("SHADOW_ONLY"))
			Expect(plan.State).To(Equal(state))
			Expect(plan.PrimaryFamily).To(Equal(family))
		},
		Entry("renamed image uses signature", base("تصویر.bin", "application/octet-stream", "image/png", validRoutePNG(1280, 720)), routeStateReady, "image"),
		Entry("renamed video uses signature", base("ویڈیو.dat", "application/octet-stream", "video/mp4", validRouteMP4()), routeStateReady, "video"),
		Entry("declared MIME mismatch", base("scene.jpg", "application/pdf", "image/png", validRoutePNG(800, 600)), routeStateManualReview, "image"),
		Entry("extension mismatch stays a hint", base("renamed.pdf", "image/png", "image/png", validRoutePNG(800, 600)), routeStateReady, "image"),
		Entry("empty file", base("empty.bin", "application/octet-stream", "application/octet-stream", nil), routeStateRejectedSecurity, "unknown"),
		Entry("truncated image", base("broken.png", "image/png", "image/png", []byte("\x89PNG\r\n\x1a\n")), routeStateManualReview, "image"),
		Entry("malformed PDF", base("broken.pdf", "application/pdf", "application/pdf", []byte("%PDF-1.7\n1 0 obj")), routeStateManualReview, "document"),
		Entry("password protected", func() evidenceRouteInput {
			v := base("locked.pdf", "application/pdf", "application/pdf", validRoutePDF())
			v.PasswordProtected = true
			return v
		}(), routeStateManualReview, "document"),
		Entry("polyglot", base("polyglot.png", "image/png", "image/png", append(validRoutePNG(10, 10), validRoutePDF()...)), routeStateRejectedSecurity, "image"),
		Entry("archive bomb", func() evidenceRouteInput {
			v := base("bomb.zip", "application/zip", "application/zip", []byte{'P', 'K', 3, 4, 0, 0, 0, 0})
			v.ArchiveBombSuspected = true
			return v
		}(), routeStateRejectedSecurity, "container"),
		Entry("huge dimensions", base("huge.png", "image/png", "image/png", validRoutePNG(100000, 100000)), routeStateRejectedSecurity, "image"),
		Entry("unsupported codec", func() evidenceRouteInput {
			v := base("camera.mp4", "video/mp4", "video/mp4", validRouteMP4())
			v.UnsupportedCodec = true
			return v
		}(), routeStateManualReview, "video"),
		Entry("unknown binary", base("device.dump", "application/octet-stream", "application/octet-stream", []byte{0, 1, 2, 3, 4}), routeStateManualReview, "unknown"),
		Entry("mixed RTL LTR filename", base("case-گاڑی-42.png", "image/png", "image/png", validRoutePNG(640, 480)), routeStateReady, "image"),
		Entry("scope unauthorized", func() evidenceRouteInput {
			v := base("scene.png", "image/png", "image/png", validRoutePNG(640, 480))
			v.ScopeAuthorized = false
			return v
		}(), routeStateRejectedSecurity, "image"),
		Entry("duplicate content", func() evidenceRouteInput {
			v := base("copy.png", "image/png", "image/png", validRoutePNG(640, 480))
			v.DuplicateContent = true
			return v
		}(), routeStateManualReview, "image"),
	)

	It("does not invoke unrelated image processors without explicit roles", func() {
		input := base("vehicle.png", "image/png", "image/png", validRoutePNG(1280, 720))
		plan := buildEvidenceRoutePlan(input)
		Expect(plan.RequiredCapabilities).To(Equal([]string{"image_metadata"}))
		Expect(plan.ComposedRoles).To(BeEmpty())
		Expect(plan.ResolvedProcessors).To(HaveLen(1))
	})

	It("selects only family-applicable explicitly requested composed roles", func() {
		input := base("vehicle.png", "image/png", "image/png", validRoutePNG(1280, 720))
		input.RequestedRoles = []string{"anpr", "ocr", "asr", "anpr"}
		input.Processors = map[string]evidenceRouteProcessorState{
			"anpr": {ProcessorID: "anpr-worker", ModelID: "plate-detector", Readiness: "READY"},
			"ocr":  {ProcessorID: "ocr-worker", ModelID: "printed-ocr", Readiness: "READY"},
		}
		plan := buildEvidenceRoutePlan(input)
		Expect(plan.ComposedRoles).To(Equal([]string{"anpr", "ocr"}))
		Expect(plan.RequiredCapabilities).To(Equal([]string{"image_metadata", "anpr", "ocr"}))
		Expect(plan.State).To(Equal(routeStateReady))
	})

	It("distinguishes an M4A audio brand from a video MP4 container", func() {
		input := base("review-audio.m4a", "audio/mp4", "audio/mp4", validRouteM4A())
		input.RequestedRoles = []string{"asr", "anpr"}
		plan := buildEvidenceRoutePlan(input)
		Expect(plan.DetectedFormat).To(Equal("m4a"))
		Expect(plan.DetectedMIME).To(Equal("audio/mp4"))
		Expect(plan.PrimaryFamily).To(Equal("audio"))
		Expect(plan.ComposedRoles).To(Equal([]string{"asr"}))
	})

	It("routes actual local files across the five shadow families when explicitly supplied", func() {
		paths := map[string]string{
			"image": os.Getenv("NXMMR_ROUTE_IMAGE"), "video": os.Getenv("NXMMR_ROUTE_VIDEO"),
			"audio": os.Getenv("NXMMR_ROUTE_AUDIO"), "document": os.Getenv("NXMMR_ROUTE_DOCUMENT"),
			"structured_records": os.Getenv("NXMMR_ROUTE_STRUCTURED"),
		}
		for family, path := range paths {
			if strings.TrimSpace(path) == "" {
				Skip("actual local route inputs are opt-in and were not supplied")
			}
			input := evidenceRouteInput{
				EvidenceID: "actual-" + family, VersionID: "source-v1", SourceFilename: path,
				ScopeAuthorized: true,
			}
			switch family {
			case "image":
				input.DetectedMIME, input.RequestedRoles = "image/jpeg", []string{"anpr", "ocr"}
			case "video":
				input.DetectedMIME, input.RequestedRoles = "video/mp4", []string{"anpr", "ocr", "asr", "bounded_frames"}
			case "audio":
				input.DetectedMIME, input.RequestedRoles = "audio/mp4", []string{"asr"}
			case "document":
				input.DetectedMIME, input.RequestedRoles = "application/pdf", []string{"ocr"}
			case "structured_records":
				input.DetectedMIME = "text/plain"
				input.Classification = evidenceClassification{Modality: "structured_records", DetectedType: "cdr"}
			}
			plan, err := buildEvidenceRoutePlanFromPath(path, input)
			Expect(err).NotTo(HaveOccurred(), family)
			Expect(plan.ExecutionAdmission).To(Equal("SHADOW_ONLY"), family)
			Expect(plan.PrimaryFamily).To(Equal(family), family)
			Expect(plan.Security.Result).To(Equal("PASS"), family)
			Expect(plan.RequiredCapabilities).NotTo(BeEmpty(), family)
			GinkgoWriter.Printf("actual route %s: format=%s roles=%v state=%s admission=%s\n", family, plan.DetectedFormat, plan.ComposedRoles, plan.State, plan.ExecutionAdmission)
		}
	})

	DescribeTable("keeps processor and model absence distinct from zero findings",
		func(readiness string, expected evidenceRouteState) {
			input := base("vehicle.png", "image/png", "image/png", validRoutePNG(1280, 720))
			input.RequestedRoles = []string{"anpr"}
			input.Processors = map[string]evidenceRouteProcessorState{
				"anpr": {ProcessorID: "anpr-worker", Readiness: readiness},
			}
			plan := buildEvidenceRoutePlan(input)
			Expect(plan.State).To(Equal(expected))
			Expect(plan.Fallback).To(Equal("manual_review"))
			Expect(strings.ToLower(plan.HumanReviewReason)).NotTo(ContainSubstring("zero"))
		},
		Entry("model missing", "MODEL_REQUIRED", routeStateModelRequired),
		Entry("processor unavailable", "UNAVAILABLE", routeStateProcessorUnavailable),
		Entry("worker unhealthy", "UNHEALTHY", routeStateProcessorUnavailable),
	)

	It("corroborates a bounded structured text classification above the auto threshold", func() {
		input := base("calls.csv", "text/csv", "text/plain", []byte("timestamp,source_number,target_number\n"))
		input.Classification = evidenceClassification{Modality: "structured_records", DetectedType: "cdr"}
		plan := buildEvidenceRoutePlan(input)
		Expect(plan.PrimaryFamily).To(Equal("structured_records"))
		Expect(plan.DetectedFormat).To(Equal("cdr"))
		Expect(plan.Confidence).To(BeNumerically(">=", autoRouteThreshold))
		Expect(plan.State).To(Equal(routeStateReady))
	})

	It("persists a deterministic round-trippable plan receipt", func() {
		plan := buildEvidenceRoutePlan(base("scene.png", "image/png", "image/png", validRoutePNG(640, 480)))
		metadata := map[string]string{}
		recordEvidenceRoutePlanMetadata(metadata, plan)
		restored := evidenceRoutePlanFromMetadata(metadata)
		Expect(restored).To(Equal(plan))
		Expect(evidenceRoutePlanID(restored)).To(Equal(plan.PlanID))
	})
})
