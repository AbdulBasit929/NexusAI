package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/LocalAI/core/schema"
)

var _ = Describe("transcription language contract", func() {
	It("prefers the explicit multipart form language", func() {
		request := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", strings.NewReader("language=ur"))
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		ctx := echo.New().NewContext(request, httptest.NewRecorder())

		input := &schema.OpenAIRequest{}
		input.Language = "en"
		Expect(requestedTranscriptionLanguage(ctx, input)).To(Equal("ur"))
	})

	It("preserves the parsed request language when the form field is absent", func() {
		request := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)
		ctx := echo.New().NewContext(request, httptest.NewRecorder())

		input := &schema.OpenAIRequest{}
		input.Language = " en "
		Expect(requestedTranscriptionLanguage(ctx, input)).To(Equal("en"))
	})

	It("preserves automatic detection when no language is supplied", func() {
		request := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)
		ctx := echo.New().NewContext(request, httptest.NewRecorder())

		Expect(requestedTranscriptionLanguage(ctx, &schema.OpenAIRequest{})).To(BeEmpty())
	})
})
