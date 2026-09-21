package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"sort"
	"strings"
)

const (
	plateRegionContractVersion = "forensics.plate-region-candidate/v1"
	plateCropContractVersion   = "forensics.plate-region-crop/v1"
	maxPlateRegionCandidates   = 32
	defaultPlateRegionNMSIoU   = 0.45
)

// imagePixelBounds always uses the unmodified source image coordinate space.
// Keeping this type integer-only prevents display scaling from leaking into
// immutable evidence locators or producing non-reconstructable crops.
type imagePixelBounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type plateRegionCandidate struct {
	ContractVersion string           `json:"contract_version"`
	CandidateID     string           `json:"candidate_id"`
	ParentEvidence  string           `json:"parent_evidence_id"`
	ParentVersion   string           `json:"parent_version_id"`
	ParentSHA256    string           `json:"parent_sha256"`
	DetectorID      string           `json:"detector_id"`
	DetectorVersion string           `json:"detector_version"`
	CoordinateSpace string           `json:"coordinate_space"`
	Bounds          imagePixelBounds `json:"bounds"`
	Confidence      float64          `json:"confidence"`
	Label           string           `json:"label"`
	TransformChain  []string         `json:"transform_chain"`
	ReviewState     string           `json:"review_state"`
}

type plateCropProvenance struct {
	ContractVersion string           `json:"contract_version"`
	CandidateID     string           `json:"candidate_id"`
	ParentEvidence  string           `json:"parent_evidence_id"`
	ParentVersion   string           `json:"parent_version_id"`
	ParentSHA256    string           `json:"parent_sha256"`
	Bounds          imagePixelBounds `json:"bounds"`
	CoordinateSpace string           `json:"coordinate_space"`
	TransformChain  []string         `json:"transform_chain"`
	Encoding        string           `json:"encoding"`
	CropSHA256      string           `json:"crop_sha256"`
	WidthPixels     int              `json:"width_pixels"`
	HeightPixels    int              `json:"height_pixels"`
}

func validatePlateRegionCandidate(candidate plateRegionCandidate, sourceWidth, sourceHeight int) error {
	if candidate.ContractVersion != plateRegionContractVersion {
		return fmt.Errorf("unsupported plate-region contract %q", candidate.ContractVersion)
	}
	if strings.TrimSpace(candidate.CandidateID) == "" {
		return errors.New("candidate_id is required")
	}
	if strings.TrimSpace(candidate.ParentEvidence) == "" || strings.TrimSpace(candidate.ParentVersion) == "" {
		return errors.New("parent evidence and version IDs are required")
	}
	if len(candidate.ParentSHA256) != sha256.Size*2 {
		return errors.New("parent_sha256 must be a lowercase SHA-256 digest")
	}
	if _, err := hex.DecodeString(candidate.ParentSHA256); err != nil || candidate.ParentSHA256 != strings.ToLower(candidate.ParentSHA256) {
		return errors.New("parent_sha256 must be a lowercase SHA-256 digest")
	}
	if strings.TrimSpace(candidate.DetectorID) == "" || strings.TrimSpace(candidate.DetectorVersion) == "" {
		return errors.New("detector identity and version are required")
	}
	if candidate.CoordinateSpace != "original_image_pixels" {
		return errors.New("candidate coordinates must use original_image_pixels")
	}
	if candidate.Label != "license_plate" {
		return errors.New("candidate label must be license_plate")
	}
	if candidate.Confidence < 0 || candidate.Confidence > 1 {
		return errors.New("candidate confidence must be between zero and one")
	}
	if sourceWidth <= 0 || sourceHeight <= 0 {
		return errors.New("source dimensions must be positive")
	}
	if !candidate.Bounds.validWithin(sourceWidth, sourceHeight) {
		return errors.New("candidate bounds must be positive and contained in the original image")
	}
	if len(candidate.TransformChain) == 0 || candidate.TransformChain[0] != "source_pixels_unchanged" {
		return errors.New("transform chain must begin with source_pixels_unchanged")
	}
	switch candidate.ReviewState {
	case "model_candidate", "human_selected", "human_rejected", "manual_region":
	default:
		return fmt.Errorf("unsupported review state %q", candidate.ReviewState)
	}
	return nil
}

func (bounds imagePixelBounds) validWithin(width, height int) bool {
	if bounds.X < 0 || bounds.Y < 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return false
	}
	return bounds.X <= width-bounds.Width && bounds.Y <= height-bounds.Height
}

func intersectionOverUnion(left, right imagePixelBounds) float64 {
	leftX2, leftY2 := left.X+left.Width, left.Y+left.Height
	rightX2, rightY2 := right.X+right.Width, right.Y+right.Height
	intersectionWidth := min(leftX2, rightX2) - max(left.X, right.X)
	intersectionHeight := min(leftY2, rightY2) - max(left.Y, right.Y)
	if intersectionWidth <= 0 || intersectionHeight <= 0 {
		return 0
	}
	intersection := intersectionWidth * intersectionHeight
	union := left.Width*left.Height + right.Width*right.Height - intersection
	if union <= 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func suppressOverlappingPlateRegions(candidates []plateRegionCandidate, threshold float64) ([]plateRegionCandidate, error) {
	if threshold <= 0 || threshold >= 1 {
		return nil, errors.New("NMS IoU threshold must be between zero and one")
	}
	if len(candidates) > maxPlateRegionCandidates {
		return nil, fmt.Errorf("candidate count %d exceeds limit %d", len(candidates), maxPlateRegionCandidates)
	}
	ordered := append([]plateRegionCandidate(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Confidence == ordered[j].Confidence {
			return ordered[i].CandidateID < ordered[j].CandidateID
		}
		return ordered[i].Confidence > ordered[j].Confidence
	})
	kept := make([]plateRegionCandidate, 0, len(ordered))
	for _, candidate := range ordered {
		overlaps := false
		for _, accepted := range kept {
			if intersectionOverUnion(candidate.Bounds, accepted.Bounds) > threshold {
				overlaps = true
				break
			}
		}
		if !overlaps {
			kept = append(kept, candidate)
		}
	}
	return kept, nil
}

func reconstructPlateCrop(source image.Image, candidate plateRegionCandidate) ([]byte, plateCropProvenance, error) {
	if source == nil {
		return nil, plateCropProvenance{}, errors.New("source image is required")
	}
	sourceBounds := source.Bounds()
	if err := validatePlateRegionCandidate(candidate, sourceBounds.Dx(), sourceBounds.Dy()); err != nil {
		return nil, plateCropProvenance{}, err
	}
	requested := image.Rect(
		sourceBounds.Min.X+candidate.Bounds.X,
		sourceBounds.Min.Y+candidate.Bounds.Y,
		sourceBounds.Min.X+candidate.Bounds.X+candidate.Bounds.Width,
		sourceBounds.Min.Y+candidate.Bounds.Y+candidate.Bounds.Height,
	)
	crop := image.NewNRGBA(image.Rect(0, 0, candidate.Bounds.Width, candidate.Bounds.Height))
	draw.Draw(crop, crop.Bounds(), source, requested.Min, draw.Src)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, crop); err != nil {
		return nil, plateCropProvenance{}, fmt.Errorf("encode deterministic PNG crop: %w", err)
	}
	cropBytes := encoded.Bytes()
	digest := sha256.Sum256(cropBytes)
	return cropBytes, plateCropProvenance{
		ContractVersion: plateCropContractVersion,
		CandidateID:     candidate.CandidateID,
		ParentEvidence:  candidate.ParentEvidence,
		ParentVersion:   candidate.ParentVersion,
		ParentSHA256:    candidate.ParentSHA256,
		Bounds:          candidate.Bounds,
		CoordinateSpace: candidate.CoordinateSpace,
		TransformChain:  append(append([]string(nil), candidate.TransformChain...), "crop_original_bounds", "encode_png_lossless"),
		Encoding:        "image/png",
		CropSHA256:      hex.EncodeToString(digest[:]),
		WidthPixels:     candidate.Bounds.Width,
		HeightPixels:    candidate.Bounds.Height,
	}, nil
}

type plateLocalizationMetrics struct {
	TruePositive  int     `json:"true_positive"`
	FalsePositive int     `json:"false_positive"`
	FalseNegative int     `json:"false_negative"`
	Precision     float64 `json:"precision"`
	Recall        float64 `json:"recall"`
	IoUThreshold  float64 `json:"iou_threshold"`
}

func benchmarkPlateLocalization(predicted, expected []imagePixelBounds, threshold float64) (plateLocalizationMetrics, error) {
	if threshold <= 0 || threshold > 1 {
		return plateLocalizationMetrics{}, errors.New("benchmark IoU threshold must be between zero and one")
	}
	matched := make([]bool, len(expected))
	truePositive := 0
	for _, prediction := range predicted {
		bestIndex, bestIoU := -1, 0.0
		for index, truth := range expected {
			if matched[index] {
				continue
			}
			if score := intersectionOverUnion(prediction, truth); score > bestIoU {
				bestIndex, bestIoU = index, score
			}
		}
		if bestIndex >= 0 && bestIoU >= threshold {
			matched[bestIndex] = true
			truePositive++
		}
	}
	metrics := plateLocalizationMetrics{
		TruePositive: truePositive, FalsePositive: len(predicted) - truePositive,
		FalseNegative: len(expected) - truePositive, IoUThreshold: threshold,
	}
	if len(predicted) > 0 {
		metrics.Precision = float64(truePositive) / float64(len(predicted))
	}
	if len(expected) > 0 {
		metrics.Recall = float64(truePositive) / float64(len(expected))
	}
	return metrics, nil
}
