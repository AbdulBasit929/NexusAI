package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func acceptedPlateCandidate(id string, bounds imagePixelBounds, confidence float64) plateRegionCandidate {
	parent := sha256.Sum256([]byte("immutable-parent"))
	return plateRegionCandidate{
		ContractVersion: plateRegionContractVersion,
		CandidateID:     id, ParentEvidence: "11111111-1111-4111-8111-111111111111",
		ParentVersion: "22222222-2222-4222-8222-222222222222",
		ParentSHA256:  hex.EncodeToString(parent[:]), DetectorID: "fixture-detector",
		DetectorVersion: "1.0.0", CoordinateSpace: "original_image_pixels",
		Bounds: bounds, Confidence: confidence, Label: "license_plate",
		TransformChain: []string{"source_pixels_unchanged"}, ReviewState: "model_candidate",
	}
}

var _ = Describe("R8 plate-region candidate and crop provenance", func() {
	It("rejects unbounded, display-space, unversioned, or unlabeled candidates", func() {
		candidate := acceptedPlateCandidate("candidate-1", imagePixelBounds{X: 5, Y: 6, Width: 20, Height: 8}, .9)
		Expect(validatePlateRegionCandidate(candidate, 100, 60)).To(Succeed())

		invalid := candidate
		invalid.CoordinateSpace = "display_css_pixels"
		Expect(validatePlateRegionCandidate(invalid, 100, 60)).To(MatchError(ContainSubstring("original_image_pixels")))
		invalid = candidate
		invalid.Bounds.X = 90
		Expect(validatePlateRegionCandidate(invalid, 100, 60)).To(MatchError(ContainSubstring("contained")))
		invalid = candidate
		invalid.DetectorVersion = ""
		Expect(validatePlateRegionCandidate(invalid, 100, 60)).To(MatchError(ContainSubstring("identity and version")))
	})

	It("applies deterministic confidence-first NMS with a stable ID tie break", func() {
		candidates := []plateRegionCandidate{
			acceptedPlateCandidate("candidate-b", imagePixelBounds{X: 10, Y: 10, Width: 30, Height: 10}, .9),
			acceptedPlateCandidate("candidate-a", imagePixelBounds{X: 11, Y: 10, Width: 30, Height: 10}, .9),
			acceptedPlateCandidate("candidate-c", imagePixelBounds{X: 60, Y: 30, Width: 25, Height: 9}, .7),
		}
		kept, err := suppressOverlappingPlateRegions(candidates, defaultPlateRegionNMSIoU)
		Expect(err).NotTo(HaveOccurred())
		Expect(kept).To(HaveLen(2))
		Expect(kept[0].CandidateID).To(Equal("candidate-a"))
		Expect(kept[1].CandidateID).To(Equal("candidate-c"))
	})

	It("caps detector output before NMS to bound review and resource use", func() {
		candidates := make([]plateRegionCandidate, maxPlateRegionCandidates+1)
		_, err := suppressOverlappingPlateRegions(candidates, defaultPlateRegionNMSIoU)
		Expect(err).To(MatchError(ContainSubstring("exceeds limit")))
	})

	It("reconstructs the exact original-coordinate crop with stable bytes and lineage", func() {
		source := image.NewNRGBA(image.Rect(7, 9, 47, 29))
		for y := source.Bounds().Min.Y; y < source.Bounds().Max.Y; y++ {
			for x := source.Bounds().Min.X; x < source.Bounds().Max.X; x++ {
				source.Set(x, y, color.NRGBA{R: uint8(x * 3), G: uint8(y * 5), B: uint8(x + y), A: 255})
			}
		}
		candidate := acceptedPlateCandidate("crop-1", imagePixelBounds{X: 4, Y: 3, Width: 18, Height: 7}, .88)
		first, firstProvenance, err := reconstructPlateCrop(source, candidate)
		Expect(err).NotTo(HaveOccurred())
		second, secondProvenance, err := reconstructPlateCrop(source, candidate)
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(Equal(first))
		Expect(secondProvenance.CropSHA256).To(Equal(firstProvenance.CropSHA256))
		Expect(firstProvenance.ContractVersion).To(Equal(plateCropContractVersion))
		Expect(firstProvenance.Bounds).To(Equal(candidate.Bounds))
		Expect(firstProvenance.TransformChain).To(Equal([]string{"source_pixels_unchanged", "crop_original_bounds", "encode_png_lossless"}))
		decoded, err := pngDecode(first)
		Expect(err).NotTo(HaveOccurred())
		Expect(decoded.Bounds().Dx()).To(Equal(18))
		Expect(decoded.Bounds().Dy()).To(Equal(7))
		Expect(color.NRGBAModel.Convert(decoded.At(0, 0))).To(Equal(color.NRGBAModel.Convert(source.At(11, 12))))
	})

	It("computes one-to-one precision and recall without double matching", func() {
		expected := []imagePixelBounds{{X: 10, Y: 10, Width: 30, Height: 10}, {X: 60, Y: 30, Width: 20, Height: 8}}
		predicted := []imagePixelBounds{{X: 11, Y: 10, Width: 30, Height: 10}, {X: 12, Y: 10, Width: 30, Height: 10}, {X: 90, Y: 40, Width: 5, Height: 5}}
		metrics, err := benchmarkPlateLocalization(predicted, expected, .5)
		Expect(err).NotTo(HaveOccurred())
		Expect(metrics.TruePositive).To(Equal(1))
		Expect(metrics.FalsePositive).To(Equal(2))
		Expect(metrics.FalseNegative).To(Equal(1))
		Expect(metrics.Precision).To(BeNumerically("~", 1.0/3.0))
		Expect(metrics.Recall).To(Equal(.5))
	})
})

func pngDecode(value []byte) (image.Image, error) {
	return png.Decode(bytes.NewReader(value))
}
