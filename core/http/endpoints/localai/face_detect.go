package localai

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mudler/LocalAI/core/backend"
	"github.com/mudler/LocalAI/core/config"
	"github.com/mudler/LocalAI/core/http/middleware"
	"github.com/mudler/LocalAI/core/schema"
	"github.com/mudler/LocalAI/pkg/model"
	"github.com/mudler/xlog"
)

// FaceDetectEndpoint returns face regions and detector confidence only.
// @Summary Detect faces without demographic analysis.
// @Tags face-recognition
// @Param request body schema.FaceDetectRequest true "query params"
// @Success 200 {object} schema.FaceDetectResponse "Response"
// @Router /v1/face/detect [post]
func FaceDetectEndpoint(cl *config.ModelConfigLoader, ml *model.ModelLoader, appConfig *config.ApplicationConfig) echo.HandlerFunc {
	return func(c echo.Context) error {
		input, ok := c.Get(middleware.CONTEXT_LOCALS_KEY_LOCALAI_REQUEST).(*schema.FaceDetectRequest)
		if !ok || input.Model == "" {
			return echo.ErrBadRequest
		}
		cfg, ok := c.Get(middleware.CONTEXT_LOCALS_KEY_MODEL_CONFIG).(*config.ModelConfig)
		if !ok || cfg == nil {
			return echo.ErrBadRequest
		}

		img, err := decodeImageInput(input.Img)
		if err != nil {
			return err
		}

		xlog.Debug("FaceDetect", "model", cfg.Name, "backend", cfg.Backend)
		res, err := backend.FaceAnalyze(c.Request().Context(), img, nil, false, ml, appConfig, *cfg)
		if err != nil {
			return mapBackendError(err)
		}

		response := schema.FaceDetectResponse{
			Faces: make([]schema.FaceDetection, len(res.GetFaces())),
		}
		for i, face := range res.GetFaces() {
			response.Faces[i] = schema.FaceDetection{
				Region: schema.FacialArea{
					X: face.GetRegion().GetX(),
					Y: face.GetRegion().GetY(),
					W: face.GetRegion().GetW(),
					H: face.GetRegion().GetH(),
				},
				Confidence: face.GetFaceConfidence(),
			}
		}
		return c.JSON(http.StatusOK, response)
	}
}
