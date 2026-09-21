package localai

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/labstack/echo/v4"
	coreTypes "github.com/mudler/LocalAGI/core/types"
	agiServices "github.com/mudler/LocalAGI/services"
	"github.com/mudler/LocalAI/core/application"
	"github.com/mudler/LocalAI/core/http/auth"
	"github.com/mudler/LocalAI/core/services/agentpool"
	"github.com/mudler/LocalAI/core/services/agents"
	"github.com/mudler/LocalAI/pkg/utils"
	"github.com/mudler/xlog"
)

// getUserID extracts the scoped user ID from the request context.
// Returns empty string when auth is not active (backward compat).
func getUserID(c echo.Context) string {
	user := auth.GetUser(c)
	if user == nil {
		return ""
	}
	return user.ID
}

// decodedParam returns the named path parameter, URL-decoding it.
//
// Echo routes a request via URL.RawPath whenever the path contains
// percent-encoded characters (e.g. %3A for ':'), and in that case stores the
// matched path-param value raw/escaped. Agent and collection names carry a
// "legacy-api-key:" prefix, so the ':' arrives as %3A and the raw param no
// longer matches the stored name. Callers must unescape before lookups.
// Falls back to the raw value if it isn't valid percent-encoding.
func decodedParam(c echo.Context, name string) string {
	raw := c.Param(name)
	if decoded, err := url.PathUnescape(raw); err == nil {
		return decoded
	}
	return raw
}

// isAdminUser returns true if the authenticated user has admin role.
func isAdminUser(c echo.Context) bool {
	user := auth.GetUser(c)
	return user != nil && user.Role == auth.RoleAdmin
}

// forensicActorRole preserves the caller's accountable service role when
// forwarding evidence operations to the forensic sidecar.
func forensicActorRole(c echo.Context) string {
	user := auth.GetUser(c)
	if user == nil {
		return ""
	}
	if user.Provider == auth.ProviderAgentWorker {
		return "agent-worker"
	}
	if user.Role == auth.RoleAdmin {
		return "admin"
	}
	return "user"
}

// forensicForwardIdentity binds sidecar requests to an authenticated LocalAI
// user. Local, intentionally unauthenticated deployments may opt into a fixed
// operator identity through explicit environment configuration; without that
// opt-in, the proxy fails closed instead of forwarding an anonymous admin.
func forensicForwardIdentity(c echo.Context) (actorID, subjectID, actorRole string, err error) {
	if user := auth.GetUser(c); user != nil {
		return user.ID, effectiveUserID(c), forensicActorRole(c), nil
	}
	actorID = strings.TrimSpace(os.Getenv("FORENSIC_RECORDS_PROXY_ACTOR_ID"))
	actorRole = strings.ToLower(strings.TrimSpace(os.Getenv("FORENSIC_RECORDS_PROXY_ACTOR_ROLE")))
	if actorID == "" || actorRole == "" {
		return "", "", "", fmt.Errorf("forensic proxy identity is not configured for unauthenticated LocalAI")
	}
	if actorRole != "user" && actorRole != "admin" && actorRole != "agent-worker" {
		return "", "", "", fmt.Errorf("FORENSIC_RECORDS_PROXY_ACTOR_ROLE must be user, admin, or agent-worker")
	}
	return actorID, actorID, actorRole, nil
}

// wantsAllUsers returns true if the request has ?all_users=true and the user is admin.
func wantsAllUsers(c echo.Context) bool {
	return c.QueryParam("all_users") == "true" && isAdminUser(c)
}

// effectiveUserID returns the user ID to scope operations to.
// SECURITY: Only admins and agent-worker service accounts may supply
// ?user_id=<id> to operate on another user's resources. Agent-worker users are
// created exclusively server-side during node registration and need to access
// collections on behalf of the user whose agent they are executing.
// Regular callers always get their own ID regardless of query params.
func effectiveUserID(c echo.Context) string {
	if targetUID := c.QueryParam("user_id"); targetUID != "" && canImpersonateUser(c) {
		if callerID := getUserID(c); callerID != targetUID {
			xlog.Info("User impersonation", "caller", callerID, "target", targetUID, "path", c.Path())
		}
		return targetUID
	}
	return getUserID(c)
}

// canImpersonateUser returns true if the caller is allowed to use ?user_id= to
// scope operations to another user. Allowed for admins and agent-worker service
// accounts (ProviderAgentWorker is set server-side during node registration and
// cannot be self-assigned).
func canImpersonateUser(c echo.Context) bool {
	user := auth.GetUser(c)
	if user == nil {
		return false
	}
	return user.Role == auth.RoleAdmin || user.Provider == auth.ProviderAgentWorker
}

func ListAgentsEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := getUserID(c)
		statuses := svc.ListAgentsForUser(userID)
		agents := slices.Sorted(maps.Keys(statuses))
		resp := map[string]any{
			"agents":     agents,
			"agentCount": len(agents),
			"actions":    len(agiServices.AvailableActions),
			"connectors": len(agiServices.AvailableConnectors),
			"statuses":   statuses,
		}
		if hubURL := svc.AgentHubURL(); hubURL != "" {
			resp["agent_hub_url"] = hubURL
		}

		// Admin cross-user aggregation
		if wantsAllUsers(c) {
			grouped := svc.ListAllAgentsGrouped()
			userGroups := map[string]any{}
			for uid, agentList := range grouped {
				if uid == userID || uid == "" {
					continue
				}
				userGroups[uid] = map[string]any{"agents": agentList}
			}
			if len(userGroups) > 0 {
				resp["user_groups"] = userGroups
			}
		}

		return c.JSON(http.StatusOK, resp)
	}
}

func CreateAgentEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := getUserID(c)
		var cfg agents.AgentConfig
		if err := c.Bind(&cfg); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := svc.CreateNativeAgentForUser(userID, &cfg); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusCreated, map[string]string{"status": "ok"})
	}
}

func GetAgentEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		name := decodedParam(c, "name")

		statuses := svc.ListAgentsForUser(userID)
		active, exists := statuses[name]
		if !exists {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Agent not found"})
		}
		return c.JSON(http.StatusOK, map[string]any{"active": active})
	}
}

func UpdateAgentEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		name := decodedParam(c, "name")
		var cfg agents.AgentConfig
		if err := c.Bind(&cfg); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := svc.UpdateNativeAgentForUser(userID, name, &cfg); err != nil {
			if strings.Contains(err.Error(), "not found") {
				return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

func DeleteAgentEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		name := decodedParam(c, "name")
		if err := svc.DeleteAgentForUser(userID, name); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

func GetAgentConfigEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		name := decodedParam(c, "name")
		if cfg := svc.GetNativeAgentConfigForUser(userID, name); cfg != nil {
			return c.JSON(http.StatusOK, cfg)
		}
		cfg := svc.GetAgentConfigForUser(userID, name)
		if cfg == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Agent not found"})
		}
		return c.JSON(http.StatusOK, cfg)
	}
}

func PauseAgentEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		if err := svc.PauseAgentForUser(userID, decodedParam(c, "name")); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

func ResumeAgentEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		if err := svc.ResumeAgentForUser(userID, decodedParam(c, "name")); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

func GetAgentStatusEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		name := decodedParam(c, "name")

		history := svc.GetAgentStatusForUser(userID, name)
		if history == nil {
			return c.JSON(http.StatusOK, map[string]any{
				"Name":    name,
				"History": []string{},
			})
		}
		entries := []string{}
		for i := len(history.Results()) - 1; i >= 0; i-- {
			h := history.Results()[i]
			actionName := ""
			if h.ActionCurrentState.Action != nil {
				actionName = h.ActionCurrentState.Action.Definition().Name.String()
			}
			entries = append(entries, fmt.Sprintf("Reasoning: %s\nAction taken: %s\nParameters: %+v\nResult: %s",
				h.Reasoning,
				actionName,
				h.ActionCurrentState.Params,
				h.Result))
		}
		return c.JSON(http.StatusOK, map[string]any{
			"Name":    name,
			"History": entries,
		})
	}
}

func GetAgentObservablesEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		name := decodedParam(c, "name")

		history, err := svc.GetAgentObservablesForUser(userID, name)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		}
		if history == nil {
			history = []json.RawMessage{}
		}
		return c.JSON(http.StatusOK, map[string]any{
			"Name":    name,
			"History": history,
		})
	}
}

func ClearAgentObservablesEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		name := decodedParam(c, "name")
		if err := svc.ClearAgentObservablesForUser(userID, name); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"Name": name, "cleared": true})
	}
}

func ChatWithAgentEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		name := decodedParam(c, "name")
		var payload struct {
			Message             string                              `json:"message"`
			CaseID              string                              `json:"case_id"`
			CollectionID        string                              `json:"collection_id"`
			ConversationContext *agents.ForensicConversationContext `json:"conversation_context"`
			QueryScope          *agents.ForensicQueryScope          `json:"query_scope"`
		}
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request format"})
		}
		message := strings.TrimSpace(payload.Message)
		if message == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Message cannot be empty"})
		}
		caseID := strings.TrimSpace(payload.CaseID)
		collectionID := strings.TrimSpace(payload.CollectionID)
		if caseID != "" && collectionID != "" && caseID != collectionID {
			return c.JSON(http.StatusConflict, map[string]string{"error": "case_id and collection_id must match"})
		}
		forensicScope := defaultString(caseID, collectionID)
		if forensicScope == "" {
			if cfg := svc.GetNativeAgentConfigForUser(userID, name); cfg != nil && cfg.EnableForensicRecords {
				forensicScope = strings.TrimSpace(cfg.ForensicCollectionID)
				if forensicScope == "" {
					return c.JSON(http.StatusConflict, map[string]string{"error": "forensic agent has no case scope; case_id is required"})
				}
			}
		}
		var messageID string
		var err error
		if forensicScope != "" {
			if err = requireForensicCollectionAccess(c, app, forensicScope); err != nil {
				return err
			}
			if payload.QueryScope != nil {
				if err = payload.QueryScope.Validate(); err != nil {
					return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
				}
			}
			if payload.ConversationContext != nil {
				if len(payload.ConversationContext.Targets) > 8 || len(payload.ConversationContext.Target) > 128 || len(payload.ConversationContext.Template) > 128 || len(payload.ConversationContext.DateFrom) > 64 || len(payload.ConversationContext.DateTo) > 64 || len(payload.ConversationContext.Direction) > 16 || len(payload.ConversationContext.ConversationID) > 128 || len(payload.ConversationContext.AnalysisID) > 128 || len(payload.ConversationContext.TurnID) > 128 || len(payload.ConversationContext.CollectionID) > 128 || len(payload.ConversationContext.CurrentQuestion) > 4096 {
					return c.JSON(http.StatusBadRequest, map[string]string{"error": "conversation_context exceeds bounded forensic limits"})
				}
				messageID, err = svc.ChatForUserWithForensicScope(userID, name, message, forensicScope, payload.QueryScope, *payload.ConversationContext)
			} else {
				messageID, err = svc.ChatForUserWithForensicScope(userID, name, message, forensicScope, payload.QueryScope)
			}
		} else {
			messageID, err = svc.ChatForUser(userID, name, message)
		}
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusAccepted, map[string]any{
			"status":     "message_received",
			"message_id": messageID,
			"case_id":    forensicScope,
		})
	}
}

// CancelAgentChatEndpoint cancels one request owned by the authenticated user.
// @Summary Cancel an active agent chat request
// @Tags Agents
// @Accept json
// @Produce json
// @Param name path string true "Agent name"
// @Param message_id path string true "Chat request message ID"
// @Success 202 {object} map[string]any
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/agents/{name}/chat/{message_id}/cancel [post]
func CancelAgentChatEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		name := decodedParam(c, "name")
		messageID := strings.TrimSpace(decodedParam(c, "message_id"))
		if messageID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "message_id is required"})
		}
		var payload struct {
			CaseID       string `json:"case_id"`
			CollectionID string `json:"collection_id"`
		}
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request format"})
		}
		caseID := strings.TrimSpace(payload.CaseID)
		collectionID := strings.TrimSpace(payload.CollectionID)
		if caseID == "" || collectionID == "" || caseID != collectionID {
			return c.JSON(http.StatusConflict, map[string]string{"error": "matching case_id and collection_id are required"})
		}
		if err := requireForensicCollectionAccess(c, app, caseID); err != nil {
			return err
		}
		cfg := svc.GetNativeAgentConfigForUser(userID, name)
		if cfg == nil || !cfg.EnableForensicRecords {
			return c.JSON(http.StatusConflict, map[string]string{"error": "agent is not enabled for forensic case-scoped chat"})
		}
		if err := svc.CancelChatForUser(userID, name, caseID, messageID); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusAccepted, map[string]any{"status": "cancellation_requested", "message_id": messageID, "case_id": caseID})
	}
}

// RetryAgentChatEndpoint explicitly retries one terminal forensic request.
// @Summary Retry a failed forensic agent chat request idempotently
// @Tags Agents
// @Accept json
// @Produce json
// @Param name path string true "Agent name"
// @Param message_id path string true "Original chat request message ID"
// @Success 202 {object} agentpool.ChatRetryResult
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/agents/{name}/chat/{message_id}/retry [post]
func RetryAgentChatEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		name := decodedParam(c, "name")
		originalMessageID := strings.TrimSpace(decodedParam(c, "message_id"))
		if originalMessageID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "message_id is required"})
		}
		var payload struct {
			Message        string                     `json:"message"`
			CaseID         string                     `json:"case_id"`
			CollectionID   string                     `json:"collection_id"`
			IdempotencyKey string                     `json:"idempotency_key"`
			QueryScope     *agents.ForensicQueryScope `json:"query_scope"`
		}
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request format"})
		}
		message := strings.TrimSpace(payload.Message)
		key := strings.TrimSpace(payload.IdempotencyKey)
		caseID := strings.TrimSpace(payload.CaseID)
		collectionID := strings.TrimSpace(payload.CollectionID)
		if message == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Message cannot be empty"})
		}
		if len(key) < 16 || len(key) > 128 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "idempotency_key must contain 16 to 128 characters"})
		}
		if caseID == "" || collectionID == "" || caseID != collectionID {
			return c.JSON(http.StatusConflict, map[string]string{"error": "matching case_id and collection_id are required"})
		}
		if err := requireForensicCollectionAccess(c, app, caseID); err != nil {
			return err
		}
		if payload.QueryScope != nil {
			if err := payload.QueryScope.Validate(); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
		}
		cfg := svc.GetNativeAgentConfigForUser(userID, name)
		if cfg == nil || !cfg.EnableForensicRecords {
			return c.JSON(http.StatusConflict, map[string]string{"error": "agent is not enabled for forensic case-scoped chat"})
		}
		result, err := svc.RetryChatForUser(userID, name, caseID, originalMessageID, message, key, payload.QueryScope)
		if err != nil {
			switch {
			case errors.Is(err, agents.ErrRetryPayloadConflict), errors.Is(err, agents.ErrRetryNotEligible):
				return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
			case strings.Contains(err.Error(), "not found"):
				return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			default:
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
		}
		return c.JSON(http.StatusAccepted, result)
	}
}

// GetAgentChatStatusEndpoint returns one authorized request lifecycle snapshot.
// @Summary Get an agent chat request status
// @Tags Agents
// @Produce json
// @Param name path string true "Agent name"
// @Param message_id path string true "Chat request message ID"
// @Param case_id query string true "Authorized forensic case ID"
// @Param collection_id query string true "Authorized forensic collection ID"
// @Success 200 {object} agents.AgentRequestStatus
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/agents/{name}/chat/{message_id}/status [get]
func GetAgentChatStatusEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		name := decodedParam(c, "name")
		messageID := strings.TrimSpace(decodedParam(c, "message_id"))
		if messageID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "message_id is required"})
		}
		caseID := strings.TrimSpace(c.QueryParam("case_id"))
		collectionID := strings.TrimSpace(c.QueryParam("collection_id"))
		if caseID == "" || collectionID == "" || caseID != collectionID {
			return c.JSON(http.StatusConflict, map[string]string{"error": "matching case_id and collection_id are required"})
		}
		if err := requireForensicCollectionAccess(c, app, caseID); err != nil {
			return err
		}
		cfg := svc.GetNativeAgentConfigForUser(userID, name)
		if cfg == nil || !cfg.EnableForensicRecords {
			return c.JSON(http.StatusConflict, map[string]string{"error": "agent is not enabled for forensic case-scoped chat"})
		}
		status, ok := svc.ChatStatusForUser(userID, name, caseID, messageID)
		if !ok {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "request status not found"})
		}
		return c.JSON(http.StatusOK, status)
	}
}

func AgentSSEEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		name := decodedParam(c, "name")

		// Try local SSE manager first
		manager := svc.GetSSEManagerForUser(userID, name)
		if manager != nil {
			return agentpool.HandleSSE(c, manager)
		}

		// Fall back to distributed EventBridge SSE
		var bridge *agents.EventBridge
		if d := app.Distributed(); d != nil {
			bridge = d.AgentBridge
		}
		if bridge != nil {
			return bridge.HandleSSE(c, name, userID)
		}

		return c.JSON(http.StatusNotFound, map[string]string{"error": "Agent not found"})
	}
}

func GetAgentConfigMetaEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		return c.JSON(http.StatusOK, svc.GetConfigMetaResult())
	}
}

func ExportAgentEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		name := decodedParam(c, "name")
		data, err := svc.ExportAgentForUser(userID, name)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		}
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.json", name))
		return c.JSONBlob(http.StatusOK, data)
	}
}

func ImportAgentEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := getUserID(c)

		// Try multipart form file first
		file, err := c.FormFile("file")
		if err == nil {
			src, err := file.Open()
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to open file"})
			}
			defer src.Close()
			data, err := io.ReadAll(src)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to read file"})
			}
			if err := svc.ImportAgentForUser(userID, data); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusCreated, map[string]string{"status": "ok"})
		}

		// Try JSON body
		var cfg agents.AgentConfig
		if err := json.NewDecoder(c.Request().Body).Decode(&cfg); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request: provide a file or JSON body"})
		}
		data, err := json.Marshal(&cfg)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := svc.ImportAgentForUser(userID, data); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusCreated, map[string]string{"status": "ok"})
	}
}

// --- Actions ---

func ListActionsEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		return c.JSON(http.StatusOK, map[string]any{
			"actions": svc.ListAvailableActions(),
		})
	}
}

func GetActionDefinitionEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		actionName := c.Param("name")

		var payload struct {
			Config map[string]string `json:"config"`
		}
		if err := json.NewDecoder(c.Request().Body).Decode(&payload); err != nil {
			payload.Config = map[string]string{}
		}

		def, err := svc.GetActionDefinition(actionName, payload.Config)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, def)
	}
}

func ExecuteActionEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		actionName := c.Param("name")

		var payload struct {
			Config map[string]string      `json:"config"`
			Params coreTypes.ActionParams `json:"params"`
		}
		if err := json.NewDecoder(c.Request().Body).Decode(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		}

		result, err := svc.ExecuteAction(c.Request().Context(), actionName, payload.Config, payload.Params)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, result)
	}
}

func AgentFileEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()

		requestedPath := c.QueryParam("path")
		if requestedPath == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "no file path specified"})
		}

		// Resolve the real path (follows symlinks, eliminates ..)
		resolved, err := filepath.EvalSymlinks(filepath.Clean(requestedPath))
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "file not found"})
		}

		// Determine the allowed outputs directory — scoped to the user when auth is active
		allowedDir := svc.OutputsDir()
		user := auth.GetUser(c)
		if user != nil {
			allowedDir = filepath.Join(allowedDir, user.ID)
		}

		allowedDirResolved, _ := filepath.EvalSymlinks(filepath.Clean(allowedDir))

		if utils.InTrustedRoot(resolved, allowedDirResolved) != nil {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "access denied"})
		}

		info, err := os.Stat(resolved)
		if err != nil || info.IsDir() {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "file not found"})
		}

		return c.File(resolved)
	}
}
