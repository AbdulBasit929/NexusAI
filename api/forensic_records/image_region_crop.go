// "Draw a box on a picture and find it elsewhere": the box is in fractions of the picture as it is displayed; the service cuts exactly that part out of the stored picture (orientation applied, lossless)
// and the existing attach flow runs on the cut-out: copies, pictures that contain it (with the box), pictures with a part that looks like it, look-alikes. The picture the box was drawn on is left out of
// its own results.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

type imageCropClient interface {
	Crop(ctx context.Context, picture []byte, box [4]float64) ([]byte, error)
}

func (c httpImageEmbedClient) Crop(ctx context.Context, picture []byte, box [4]float64) ([]byte, error) {
	payload, err := json.Marshal(map[string]any{"b64": base64.StdEncoding.EncodeToString(picture), "box": box})
	if err != nil {
		return nil, err
	}
	var out struct {
		PNG string `json:"png_b64"`
	}
	if err := c.post(ctx, "/crop", "application/json", payload, &out); err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(out.PNG)
	if err != nil || len(data) == 0 {
		return nil, errors.New("the image search service returned no part")
	}
	return data, nil
}

// parseImageBox reads "x0,y0,x1,y1" (fractions of the picture): four numbers in 0..1 with x1 > x0 and y1 > y0.
func parseImageBox(text string) ([4]float64, error) {
	var box [4]float64
	parts := strings.Split(strings.TrimSpace(text), ",")
	if len(parts) != 4 {
		return box, errors.New("box must be four numbers: x0,y0,x1,y1 as fractions of the picture")
	}
	for i, part := range parts {
		value, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil || value < 0 || value > 1 {
			return box, errors.New("every number of the box must be between 0 and 1")
		}
		box[i] = value
	}
	if box[2] <= box[0] || box[3] <= box[1] {
		return box, errors.New("the box must have x1 greater than x0 and y1 greater than y0")
	}
	return box, nil
}

// cropStoredPicture reads the stored bytes of one image of the authorised scope and asks the service for the part inside the box.
func cropStoredPicture(ctx context.Context, deps imageSearchDeps, scope forensicScope, evidenceID string, box [4]float64) ([]byte, error) {
	if deps.crop == nil || deps.db == nil || deps.store.root == "" {
		return nil, imageSearchError{http.StatusServiceUnavailable, "cutting a part out of a picture is not available here"}
	}
	descriptor, err := resolveEvidenceContent(ctx, deps.db, scope, evidenceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, imageSearchError{http.StatusNotFound, "that picture was not found in this case"}
	}
	if err != nil {
		return nil, fmt.Errorf("resolve the picture: %w", err)
	}
	if descriptor.Modality != "image" {
		return nil, imageSearchError{http.StatusBadRequest, "a part can only be cut out of a picture"}
	}
	if descriptor.SizeBytes <= 0 || descriptor.SizeBytes > imageSearchMaxUpload {
		return nil, imageSearchError{http.StatusRequestEntityTooLarge, "the picture is too large to search inside"}
	}
	verified, err := deps.store.Verify(descriptor.StorageURI, descriptor.SizeBytes)
	if err != nil {
		return nil, imageSearchError{http.StatusConflict, "the retained picture could not be verified"}
	}
	data, err := os.ReadFile(verified.Path)
	if err != nil {
		return nil, fmt.Errorf("read the picture: %w", err)
	}
	part, err := deps.crop.Crop(ctx, data, box)
	if err != nil {
		if errors.Is(err, errImageSearchServiceDown) {
			return nil, imageSearchError{http.StatusServiceUnavailable, err.Error()}
		}
		return nil, imageSearchError{http.StatusBadRequest, err.Error()}
	}
	return part, nil
}

// withoutPicture removes one picture (the one the box was drawn on) from a list of hits.
func withoutPicture(hits []imageSearchHit, evidenceID string) []imageSearchHit {
	if evidenceID == "" {
		return hits
	}
	kept := make([]imageSearchHit, 0, len(hits))
	for _, h := range hits {
		if h.EvidenceID != evidenceID {
			kept = append(kept, h)
		}
	}
	return kept
}
