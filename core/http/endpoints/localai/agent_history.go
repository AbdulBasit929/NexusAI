package localai

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/mudler/LocalAI/core/application"
	"github.com/mudler/LocalAI/core/services/agents"
	"gorm.io/gorm"
)

const maxBrowserHistoryImportBytes = 5 << 20

func authorizedAgentHistoryScopeValues(c echo.Context, app *application.Application, caseValue, collectionValue string) (string, error) {
	caseID := strings.TrimSpace(caseValue)
	collectionID := strings.TrimSpace(collectionValue)
	if caseID == "" || collectionID == "" || caseID != collectionID {
		return "", echo.NewHTTPError(http.StatusConflict, "matching case_id and collection_id are required")
	}
	if err := requireForensicCollectionAccess(c, app, caseID); err != nil {
		return "", err
	}
	cfg := app.AgentPoolService().GetNativeAgentConfigForUser(effectiveUserID(c), decodedParam(c, "name"))
	if cfg == nil || !cfg.EnableForensicRecords {
		return "", echo.NewHTTPError(http.StatusConflict, "agent is not enabled for forensic case history")
	}
	return caseID, nil
}

func authorizedAgentHistoryScope(c echo.Context, app *application.Application) (string, error) {
	return authorizedAgentHistoryScopeValues(c, app, c.QueryParam("case_id"), c.QueryParam("collection_id"))
}

func historyServiceError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, agents.ErrAnalysisHistoryUnavailable):
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
	case errors.Is(err, gorm.ErrRecordNotFound):
		return c.JSON(http.StatusNotFound, map[string]string{"error": "analysis history entry not found"})
	case strings.Contains(err.Error(), "cursor"):
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "analysis history operation failed"})
	}
}

// ListAgentAnalysisHistoryEndpoint lists retained analysis for one governed case.
// @Summary List case-scoped forensic analysis history
// @Tags Agents
// @Produce json
// @Param name path string true "Agent name"
// @Param case_id query string true "Authorized forensic case ID"
// @Param collection_id query string true "Authorized forensic collection ID"
// @Param cursor query string false "Opaque pagination cursor"
// @Param limit query int false "Page size (1-100)"
// @Param saved query bool false "Return only saved analyses"
// @Success 200 {object} agents.AnalysisHistoryPage
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /api/agents/{name}/history [get]
func ListAgentAnalysisHistoryEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		caseID, err := authorizedAgentHistoryScope(c, app)
		if err != nil {
			return err
		}
		limit, err := strconv.Atoi(defaultString(c.QueryParam("limit"), "25"))
		if err != nil || limit < 1 || limit > 100 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "limit must be between 1 and 100"})
		}
		page, err := app.AgentPoolService().AnalysisHistoryForUser(effectiveUserID(c), decodedParam(c, "name"), caseID, agents.AnalysisHistoryListOptions{
			Limit: limit, Cursor: strings.TrimSpace(c.QueryParam("cursor")), SavedOnly: strings.EqualFold(c.QueryParam("saved"), "true"),
		})
		if err != nil {
			return historyServiceError(c, err)
		}
		return c.JSON(http.StatusOK, page)
	}
}

// GetAgentAnalysisHistoryEndpoint returns one retained analysis.
// @Summary Get a case-scoped forensic analysis
// @Tags Agents
// @Produce json
// @Param name path string true "Agent name"
// @Param analysis_id path string true "Analysis ID"
// @Param case_id query string true "Authorized forensic case ID"
// @Param collection_id query string true "Authorized forensic collection ID"
// @Success 200 {object} agents.AnalysisHistoryEntry
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/agents/{name}/history/{analysis_id} [get]
func GetAgentAnalysisHistoryEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		caseID, err := authorizedAgentHistoryScope(c, app)
		if err != nil {
			return err
		}
		entry, err := app.AgentPoolService().AnalysisForUser(effectiveUserID(c), decodedParam(c, "name"), caseID, strings.TrimSpace(decodedParam(c, "analysis_id")))
		if err != nil {
			return historyServiceError(c, err)
		}
		return c.JSON(http.StatusOK, entry)
	}
}

type analysisSavedRequest struct {
	CaseID       string `json:"case_id"`
	CollectionID string `json:"collection_id"`
	Saved        bool   `json:"saved"`
	Title        string `json:"title"`
}

// SetAgentAnalysisSavedEndpoint idempotently saves or unsaves one analysis.
// @Summary Save or unsave a forensic analysis
// @Tags Agents
// @Accept json
// @Produce json
// @Param name path string true "Agent name"
// @Param analysis_id path string true "Analysis ID"
// @Param request body analysisSavedRequest true "Saved state"
// @Success 200 {object} agents.AnalysisHistoryEntry
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/agents/{name}/history/{analysis_id}/saved [put]
func SetAgentAnalysisSavedEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		var payload analysisSavedRequest
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request format"})
		}
		if len(strings.TrimSpace(payload.Title)) > 160 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "title must not exceed 160 characters"})
		}
		caseID, err := authorizedAgentHistoryScopeValues(c, app, payload.CaseID, payload.CollectionID)
		if err != nil {
			return err
		}
		entry, err := app.AgentPoolService().SetAnalysisSavedForUser(effectiveUserID(c), decodedParam(c, "name"), caseID, strings.TrimSpace(decodedParam(c, "analysis_id")), payload.Saved, payload.Title)
		if err != nil {
			return historyServiceError(c, err)
		}
		return c.JSON(http.StatusOK, entry)
	}
}

type browserHistoryImportRequest struct {
	ContractVersion string                              `json:"contract_version"`
	CaseID          string                              `json:"case_id"`
	CollectionID    string                              `json:"collection_id"`
	Conversations   []agents.BrowserHistoryConversation `json:"conversations"`
}

// ImportAgentAnalysisHistoryEndpoint explicitly imports browser-local history.
// @Summary Import browser-local forensic analysis history
// @Tags Agents
// @Accept json
// @Produce json
// @Param name path string true "Agent name"
// @Param request body browserHistoryImportRequest true "Versioned browser history"
// @Success 201 {object} agents.BrowserHistoryImportResult
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/agents/{name}/history/import [post]
func ImportAgentAnalysisHistoryEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		var payload browserHistoryImportRequest
		decoder := json.NewDecoder(io.LimitReader(c.Request().Body, maxBrowserHistoryImportBytes+1))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid browser history payload"})
		}
		if payload.ContractVersion != "browser-agent-history/v1" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "unsupported browser history contract_version"})
		}
		if len(payload.Conversations) > 500 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "at most 500 conversations may be imported"})
		}
		totalMessages := 0
		for _, conversation := range payload.Conversations {
			totalMessages += len(conversation.Messages)
		}
		if totalMessages > 5000 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "at most 5000 messages may be imported"})
		}
		caseID, err := authorizedAgentHistoryScopeValues(c, app, payload.CaseID, payload.CollectionID)
		if err != nil {
			return err
		}
		result, err := app.AgentPoolService().ImportBrowserAnalysisHistory(effectiveUserID(c), decodedParam(c, "name"), caseID, payload.Conversations)
		if err != nil {
			return historyServiceError(c, err)
		}
		return c.JSON(http.StatusCreated, result)
	}
}

// RollbackAgentAnalysisImportEndpoint removes unsaved, non-held rows from one import.
// @Summary Roll back a browser history import
// @Tags Agents
// @Produce json
// @Param name path string true "Agent name"
// @Param import_id path string true "Import ID"
// @Param case_id query string true "Authorized forensic case ID"
// @Param collection_id query string true "Authorized forensic collection ID"
// @Success 200 {object} agents.AnalysisImportRollbackResult
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/agents/{name}/history/imports/{import_id} [delete]
func RollbackAgentAnalysisImportEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		caseID, err := authorizedAgentHistoryScope(c, app)
		if err != nil {
			return err
		}
		result, err := app.AgentPoolService().RollbackBrowserAnalysisImport(effectiveUserID(c), decodedParam(c, "name"), caseID, strings.TrimSpace(decodedParam(c, "import_id")))
		if err != nil {
			return historyServiceError(c, err)
		}
		return c.JSON(http.StatusOK, result)
	}
}
