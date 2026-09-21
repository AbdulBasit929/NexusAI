package agentpool

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mudler/LocalAI/core/config"
	"github.com/mudler/LocalAI/core/http/auth"
	"github.com/mudler/LocalAI/core/services/agents"
	"github.com/mudler/LocalAI/core/services/distributed"
	"github.com/mudler/LocalAI/core/services/messaging"
	skillsManager "github.com/mudler/LocalAI/core/services/skills"

	"github.com/mudler/LocalAGI/core/agent"
	"github.com/mudler/LocalAGI/core/sse"
	"github.com/mudler/LocalAGI/core/state"
	coreTypes "github.com/mudler/LocalAGI/core/types"
	agiServices "github.com/mudler/LocalAGI/services"
	"github.com/mudler/LocalAGI/services/skills"
	"github.com/mudler/LocalAGI/webui/collections"
	"github.com/mudler/xlog"

	"gorm.io/gorm"
)

const maxPlainTextCollectionPreviewBytes = 2 * 1024 * 1024

// localAGICore manages the in-process LocalAGI agent pool (standalone mode only).
type localAGICore struct {
	pool          *state.AgentPool
	skillsService *skills.Service
	configMeta    state.AgentConfigMeta
	sharedState   *coreTypes.AgentSharedState
	actionsConfig map[string]string
}

// distributedBridge connects to the NATS-based distributed agent system.
type distributedBridge struct {
	natsClient  messaging.Publisher     // NATS client for distributed agent execution
	agentStore  *agents.AgentStore      // PostgreSQL agent config store
	eventBridge AgentEventBridge        // Event bridge for SSE + persistence
	skillStore  *distributed.SkillStore // PostgreSQL skill metadata (distributed mode)
	dispatcher  agents.Dispatcher       // Native dispatcher (distributed or local)
}

// userManager handles per-user services, storage, and auth.
type userManager struct {
	userServices *UserServicesManager
	userStorage  *UserScopedStorage
	authDB       *gorm.DB
}

// AgentPoolService wraps LocalAGI's AgentPool, Skills service, and collections backend
// to provide agentic capabilities integrated directly into LocalAI.
type AgentPoolService struct {
	appConfig          *config.ApplicationConfig
	collectionsBackend collections.Backend
	configBackend      AgentConfigBackend // Abstracts local vs distributed agent operations
	localAGI           localAGICore
	distributed        distributedBridge
	users              userManager
	stateDir           string
	outputsDir         string
	apiURL             string // Resolved API URL for agent execution
	apiKey             string // Resolved API key for agent execution
	chatCancels        messaging.CancelRegistry
	chatStatuses       agents.RequestStatusRegistry
	chatRetries        agents.ChatRetryRegistry
	mu                 sync.Mutex
}

// AgentEventBridge is the interface for event publishing needed by AgentPoolService.
type AgentEventBridge interface {
	PublishMessage(agentName, userID, sender, content, messageID string, answerMetadata ...map[string]any) error
	PublishStatus(agentName, userID, messageID, status string, caseIDs ...string) error
	PublishStreamEvent(agentName, userID, messageID string, data map[string]any) error
	RequestStatus(agentName, userID, caseID, messageID string) (agents.AgentRequestStatus, bool)
	CancelExecution(agentName, userID, caseID, messageID string) error
	RegisterCancel(key string, cancel context.CancelFunc)
	DeregisterCancel(key string)
}

// AgentConfigStore is the interface for agent config persistence.
type AgentConfigStore interface {
	SaveConfig(cfg *agents.AgentConfigRecord) error
	GetConfig(userID, name string) (*agents.AgentConfigRecord, error)
	ListConfigs(userID string) ([]agents.AgentConfigRecord, error)
	DeleteConfig(userID, name string) error
	UpdateStatus(userID, name, status string) error
	UpdateLastRun(userID, name string) error
}

// AgentPoolOptions holds optional dependencies for AgentPoolService.
// Zero values are fine — the service degrades gracefully without them.
type AgentPoolOptions struct {
	AuthDB      *gorm.DB
	SkillStore  *distributed.SkillStore
	NATSClient  messaging.Publisher
	EventBridge AgentEventBridge
	AgentStore  *agents.AgentStore
}

func NewAgentPoolService(appConfig *config.ApplicationConfig, opts ...AgentPoolOptions) (*AgentPoolService, error) {
	svc := &AgentPoolService{
		appConfig: appConfig,
	}
	if len(opts) > 0 {
		o := opts[0]
		if o.AuthDB != nil {
			svc.users.authDB = o.AuthDB
		}
		if o.SkillStore != nil {
			svc.distributed.skillStore = o.SkillStore
		}
		if o.NATSClient != nil {
			svc.distributed.natsClient = o.NATSClient
		}
		if o.EventBridge != nil {
			svc.distributed.eventBridge = o.EventBridge
		}
		if o.AgentStore != nil {
			svc.distributed.agentStore = o.AgentStore
		}
	}
	if svc.distributed.agentStore == nil && isPostgresAgentDatabaseURL(appConfig.AgentPool.DatabaseURL) {
		store, err := agents.NewAgentStoreFromURL(appConfig.AgentPool.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("initializing standalone agent history store: %w", err)
		}
		svc.distributed.agentStore = store
	}
	return svc, nil
}

func isPostgresAgentDatabaseURL(value string) bool {
	return strings.HasPrefix(value, "postgres://") || strings.HasPrefix(value, "postgresql://")
}

func (s *AgentPoolService) Start(ctx context.Context) error {
	cfg := s.appConfig.AgentPool

	// API URL: use configured value, or derive self-referencing URL from LocalAI's address
	apiURL := cfg.APIURL
	if apiURL == "" {
		_, port, err := net.SplitHostPort(s.appConfig.APIAddress)
		if err != nil {
			port = strings.TrimPrefix(s.appConfig.APIAddress, ":")
		}
		apiURL = "http://127.0.0.1:" + port
	}
	apiKey := cfg.APIKey
	if apiKey == "" && len(s.appConfig.ApiKeys) > 0 {
		apiKey = s.appConfig.ApiKeys[0]
	}

	s.apiURL = apiURL
	s.apiKey = apiKey

	// Distributed mode: use native executor + NATSDispatcher.
	// No LocalAGI pool, no collections, no skills service — all stateless.
	if s.distributed.natsClient != nil {
		return s.startDistributed(ctx, apiURL, apiKey)
	}

	// Standalone mode: use LocalAGI pool (backward compat)
	return s.startLocalAGI(ctx, cfg, apiURL, apiKey)
}

func (s *AgentPoolService) buildCollectionsConfig(apiURL, apiKey, collectionDBPath, fileAssets string) *collections.Config {
	cfg := s.appConfig.AgentPool
	return &collections.Config{
		LLMAPIURL:        apiURL,
		LLMAPIKey:        apiKey,
		LLMModel:         cfg.DefaultModel,
		CollectionDBPath: collectionDBPath,
		FileAssets:       fileAssets,
		VectorEngine:     cfg.VectorEngine,
		EmbeddingModel:   cfg.EmbeddingModel,
		MaxChunkingSize:  cfg.MaxChunkingSize,
		ChunkOverlap:     cfg.ChunkOverlap,
		DatabaseURL:      cfg.DatabaseURL,
	}
}

// startDistributed initializes the native agent executor with NATS dispatcher.
// No LocalAGI pool is created — agent execution is stateless.
// Skills and collections are still initialized for the frontend UI.
func (s *AgentPoolService) startDistributed(ctx context.Context, apiURL, apiKey string) error {
	cfg := s.appConfig.AgentPool

	// State dir for skills and outputs
	stateDir := cmp.Or(cfg.StateDir, s.appConfig.DataPath, s.appConfig.DynamicConfigsDir, "agents")
	if err := os.MkdirAll(stateDir, 0750); err != nil {
		xlog.Warn("Failed to create agent state dir", "error", err)
	}
	s.stateDir = stateDir

	// Outputs directory
	outputsDir := filepath.Join(stateDir, "outputs")
	if err := os.MkdirAll(outputsDir, 0750); err != nil {
		xlog.Warn("Failed to create outputs directory", "error", err)
	}
	s.outputsDir = outputsDir

	// Skills service — same as standalone, filesystem-based
	skillsSvc, err := skills.NewService(stateDir)
	if err != nil {
		xlog.Warn("Failed to create skills service in distributed mode", "error", err)
	} else {
		s.localAGI.skillsService = skillsSvc
	}

	// Collections backend — same as standalone, in-process
	collectionDBPath := cfg.CollectionDBPath
	if collectionDBPath == "" {
		collectionDBPath = filepath.Join(stateDir, "collections")
	}
	fileAssets := filepath.Join(stateDir, "assets")

	collectionsBackend, _ := collections.NewInProcessBackend(s.buildCollectionsConfig(apiURL, apiKey, collectionDBPath, fileAssets))
	s.collectionsBackend = collectionsBackend

	// User-scoped storage
	dataDir := cmp.Or(s.appConfig.DataPath, s.appConfig.DynamicConfigsDir)
	s.users.userStorage = NewUserScopedStorage(stateDir, dataDir)

	// Start the background agent scheduler on the frontend.
	// It needs DB access to list configs and update LastRunAt — the worker doesn't have DB.
	// The advisory lock ensures only one frontend instance runs the scheduler.
	if s.users.authDB != nil && s.distributed.natsClient != nil && s.distributed.agentStore != nil {
		var schedulerOpts []agents.AgentSchedulerOpt
		if s.distributed.skillStore != nil {
			schedulerOpts = append(schedulerOpts, agents.WithSchedulerSkillProvider(s.buildSkillProvider()))
		}
		scheduler := agents.NewAgentScheduler(
			s.users.authDB,
			s.distributed.natsClient,
			s.distributed.agentStore,
			messaging.SubjectAgentExecute,
			schedulerOpts...,
		)
		go scheduler.Start(ctx)
	}

	// Wire the distributed config backend
	s.configBackend = newDistributedAgentConfigBackend(s, s.distributed.agentStore)

	xlog.Info("Agent pool started in distributed mode (frontend dispatcher only)", "apiURL", apiURL, "stateDir", stateDir)
	return nil
}

// startLocalAGI initializes the full LocalAGI pool for standalone mode.
func (s *AgentPoolService) startLocalAGI(_ context.Context, cfg config.AgentPoolConfig, apiURL, apiKey string) error {
	// State dir: explicit config > DataPath > DynamicConfigsDir > fallback
	stateDir := cmp.Or(cfg.StateDir, s.appConfig.DataPath, s.appConfig.DynamicConfigsDir, "agents")
	if err := os.MkdirAll(stateDir, 0750); err != nil {
		return fmt.Errorf("failed to create agent pool state dir: %w", err)
	}

	// Collections paths
	collectionDBPath := cfg.CollectionDBPath
	if collectionDBPath == "" {
		collectionDBPath = filepath.Join(stateDir, "collections")
	}
	fileAssets := filepath.Join(stateDir, "assets")

	// Skills service
	skillsSvc, err := skills.NewService(stateDir)
	if err != nil {
		xlog.Error("Failed to create skills service", "error", err)
	}
	s.localAGI.skillsService = skillsSvc

	// Actions config map
	actionsConfig := map[string]string{
		agiServices.ConfigStateDir: stateDir,
	}
	if cfg.CustomActionsDir != "" {
		actionsConfig[agiServices.CustomActionsDir] = cfg.CustomActionsDir
	}

	// Create outputs subdirectory
	outputsDir := filepath.Join(stateDir, "outputs")
	if err := os.MkdirAll(outputsDir, 0750); err != nil {
		xlog.Error("Failed to create outputs directory", "path", outputsDir, "error", err)
	}

	s.localAGI.actionsConfig = actionsConfig
	s.stateDir = stateDir
	s.outputsDir = outputsDir
	s.localAGI.sharedState = coreTypes.NewAgentSharedState(5 * time.Minute)

	// Initialize user-scoped storage
	dataDir := cmp.Or(s.appConfig.DataPath, s.appConfig.DynamicConfigsDir)
	s.users.userStorage = NewUserScopedStorage(stateDir, dataDir)

	// Create the agent pool
	pool, err := state.NewAgentPool(
		cfg.DefaultModel,
		cfg.MultimodalModel,
		cfg.TranscriptionModel,
		cfg.TranscriptionLanguage,
		cfg.TTSModel,
		apiURL,
		apiKey,
		stateDir,
		agiServices.Actions(actionsConfig),
		agiServices.Connectors,
		agiServices.DynamicPrompts(actionsConfig),
		agiServices.Filters,
		cfg.Timeout,
		cfg.EnableLogs,
		skillsSvc,
	)
	if err != nil {
		return fmt.Errorf("failed to create agent pool: %w", err)
	}
	s.localAGI.pool = pool

	// Create in-process collections backend and RAG provider
	collectionsCfg := s.buildCollectionsConfig(apiURL, apiKey, collectionDBPath, fileAssets)
	collectionsBackend, collectionsState := collections.NewInProcessBackend(collectionsCfg)
	s.collectionsBackend = collectionsBackend

	embedded := collections.RAGProviderFromState(collectionsState)
	pool.SetRAGProvider(func(collectionName, _, _ string) (agent.RAGDB, state.KBCompactionClient, bool) {
		return embedded(collectionName)
	})

	// Build config metadata for UI
	s.localAGI.configMeta = state.NewAgentConfigMeta(
		agiServices.ActionsConfigMeta(cfg.CustomActionsDir),
		agiServices.ConnectorsConfigMeta(),
		agiServices.DynamicPromptsConfigMeta(cfg.CustomActionsDir),
		agiServices.FiltersConfigMeta(),
	)

	// Start all agents
	if err := pool.StartAll(); err != nil {
		xlog.Error("Failed to start agent pool", "error", err)
	}

	// Wire the local config backend
	s.configBackend = newLocalAgentConfigBackend(s)

	xlog.Info("Agent pool started (standalone/LocalAGI mode)", "stateDir", stateDir, "apiURL", apiURL)
	return nil
}

func (s *AgentPoolService) Stop() {
	if s.configBackend != nil {
		s.configBackend.Stop()
	}
}

// ConfigBackend returns the underlying AgentConfigBackend.
func (s *AgentPoolService) ConfigBackend() AgentConfigBackend {
	return s.configBackend
}

// APIURL returns the resolved API URL for agent execution.
func (s *AgentPoolService) APIURL() string {
	return s.apiURL
}

// APIKey returns the resolved API key for agent execution.
func (s *AgentPoolService) APIKey() string {
	return s.apiKey
}

// Pool returns the underlying AgentPool.
func (s *AgentPoolService) Pool() *state.AgentPool {
	return s.localAGI.pool
}

// SetNATSClient sets the NATS client for distributed agent execution.
// Deprecated: prefer passing NATSClient via AgentPoolOptions at construction time.
func (s *AgentPoolService) SetNATSClient(nc messaging.Publisher) {
	s.distributed.natsClient = nc
}

// SetEventBridge sets the event bridge for distributed SSE + persistence.
// Deprecated: prefer passing EventBridge via AgentPoolOptions at construction time.
func (s *AgentPoolService) SetEventBridge(eb AgentEventBridge) {
	s.distributed.eventBridge = eb
}

// SetAgentStore sets the PostgreSQL agent config store.
// Deprecated: prefer passing AgentStore via AgentPoolOptions at construction time.
func (s *AgentPoolService) SetAgentStore(store *agents.AgentStore) {
	s.distributed.agentStore = store
}

// Agent execution in distributed mode is handled by the dedicated agent-worker process
// using the NATSDispatcher from core/services/agents/dispatcher.go.
// The frontend only dispatches chat events to NATS via dispatchChat().

// --- Agent CRUD ---

func (s *AgentPoolService) GetAgent(name string) *agent.Agent {
	// GetAgent is used by the responses interceptor to check if a model name
	// is an agent. It uses the raw pool key (no userID prefix).
	return s.configBackend.GetAgent("", name)
}

// Chat sends a message to an agent and returns immediately. Responses come via SSE.
func (s *AgentPoolService) Chat(name, message string) (string, error) {
	ag := s.localAGI.pool.GetAgent(name)
	if ag == nil {
		return "", fmt.Errorf("%w: %s", ErrAgentNotFound, name)
	}
	manager := s.localAGI.pool.GetManager(name)
	if manager == nil {
		return "", fmt.Errorf("SSE manager not found for agent: %s", name)
	}

	messageID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Send user message via SSE
	userMsg, _ := json.Marshal(map[string]any{
		"id":         messageID + "-user",
		"message_id": messageID + "-user",
		"sender":     "user",
		"content":    message,
		"timestamp":  time.Now().Format(time.RFC3339),
	})
	manager.Send(sse.NewMessage(string(userMsg)).WithEvent("json_message"))

	// Send processing status
	statusMsg, _ := json.Marshal(map[string]any{
		"status":     "processing",
		"message_id": messageID,
		"timestamp":  time.Now().Format(time.RFC3339),
	})
	manager.Send(sse.NewMessage(string(statusMsg)).WithEvent("json_message_status"))

	if nativeCfg, err := s.nativeAgentConfigFromKey(name); err != nil {
		xlog.Warn("Failed to load native agent config for chat", "agent", name, "error", err)
	} else if shouldUseNativeAgentExecutor(nativeCfg) {
		userID, agentName := splitAgentKey(name)
		xlog.Info("Using native local agent executor", "agent", agentName, "user", userID, "forensic_records", nativeCfg.EnableForensicRecords, "forensic_api_url", nativeCfg.ForensicRecordsAPIURL, "forensic_collection", nativeCfg.ForensicCollectionID)
		s.startNativeLocalChat(name, message, messageID, nativeCfg, manager)
		return messageID, nil
	} else {
		xlog.Debug("Native local agent executor not selected", "agent", name, "native_config", nativeCfg != nil)
	}

	// Process asynchronously
	go func() {
		response := ag.Ask(coreTypes.WithText(message))

		if response == nil {
			errMsg, _ := json.Marshal(map[string]any{
				"error":      "agent request failed or was cancelled",
				"message_id": messageID,
				"timestamp":  time.Now().Format(time.RFC3339),
			})
			manager.Send(sse.NewMessage(string(errMsg)).WithEvent("json_error"))
		} else if response.Error != nil {
			errMsg, _ := json.Marshal(map[string]any{
				"error":      response.Error.Error(),
				"message_id": messageID,
				"timestamp":  time.Now().Format(time.RFC3339),
			})
			manager.Send(sse.NewMessage(string(errMsg)).WithEvent("json_error"))
		} else {
			// Collect metadata from all action states
			metadata := map[string]any{}
			for _, state := range response.State {
				for k, v := range state.Metadata {
					if existing, ok := metadata[k]; ok {
						if existList, ok := existing.([]string); ok {
							if newList, ok := v.([]string); ok {
								metadata[k] = append(existList, newList...)
								continue
							}
						}
					}
					metadata[k] = v
				}
			}

			if len(metadata) > 0 {
				// Extract userID from the agent key (format: "userID:agentName")
				var chatUserID string
				if uid, _, ok := strings.Cut(name, ":"); ok {
					chatUserID = uid
				}
				s.collectAndCopyMetadata(metadata, chatUserID)
			}

			content := s.appendLocalAGIKBCitations(response.Response, name, message, response.State)
			msg := map[string]any{
				"id":         messageID + "-agent",
				"message_id": messageID + "-agent",
				"sender":     "agent",
				"content":    content,
				"timestamp":  time.Now().Format(time.RFC3339),
			}
			if len(metadata) > 0 {
				msg["metadata"] = metadata
			}
			respMsg, _ := json.Marshal(msg)
			manager.Send(sse.NewMessage(string(respMsg)).WithEvent("json_message"))
		}

		completedMsg, _ := json.Marshal(map[string]any{
			"status":     "completed",
			"message_id": messageID,
			"timestamp":  time.Now().Format(time.RFC3339),
		})
		manager.Send(sse.NewMessage(string(completedMsg)).WithEvent("json_message_status"))
	}()

	return messageID, nil
}

func (s *AgentPoolService) nativeAgentConfigFromKey(agentKey string) (*agents.AgentConfig, error) {
	userID, name := splitAgentKey(agentKey)
	return s.nativeAgentConfigForUser(userID, name)
}

func shouldUseNativeAgentExecutor(cfg *agents.AgentConfig) bool {
	return cfg != nil && cfg.EnableForensicRecords
}

func (s *AgentPoolService) startNativeLocalChat(agentKey, message, messageID string, cfg *agents.AgentConfig, manager sse.Manager) {
	userID, agentName := splitAgentKey(agentKey)
	ctx, cancel := agents.WithAgentChatDeadline(context.Background(), 0)
	key := agents.AgentCancellationKey(agentName, userID, cfg.ForensicCollectionID, messageID)
	s.chatCancels.Register(key, cancel)
	go func() {
		defer s.chatCancels.Deregister(key)
		defer cancel()
		s.executeNativeLocalChat(ctx, agentKey, message, messageID, cfg, manager)
	}()
}

func (s *AgentPoolService) executeNativeLocalChat(ctx context.Context, agentKey, message, messageID string, cfg *agents.AgentConfig, manager sse.Manager) {
	userID, agentName := splitAgentKey(agentKey)
	cfgCopy := *cfg
	cfgCopy.Name = agentName
	normalizeNativeAgentConfig(&cfgCopy)

	sendJSON := func(event string, payload map[string]any) {
		if _, ok := payload["message_id"]; !ok {
			payload["message_id"] = messageID
		}
		if _, ok := payload["timestamp"]; !ok {
			payload["timestamp"] = time.Now().Format(time.RFC3339)
		}
		data, _ := json.Marshal(payload)
		manager.Send(sse.NewMessage(string(data)).WithEvent(event))
	}
	sendStatus := func(status string) {
		s.chatStatuses.Record(agentName, userID, cfgCopy.ForensicCollectionID, messageID, status)
		if s.distributed.agentStore != nil {
			if err := s.distributed.agentStore.UpdateAnalysisStatus(userID, agentName, cfgCopy.ForensicCollectionID, messageID, status); err != nil {
				xlog.Warn("Failed to retain local analysis status", "agent", agentName, "message_id", messageID, "error", err)
			}
		}
		sendJSON("json_message_status", map[string]any{"status": status})
	}

	opts := agents.ExecuteChatOpts{
		APIURL:    s.apiURL,
		APIKey:    s.apiKey,
		UserID:    userID,
		MessageID: messageID,
	}
	if cfgCopy.EnableSkills {
		if loaded, err := s.loadSkillsForUser(userID); err == nil {
			opts.SkillProvider = staticSkillProvider{skills: loaded}
		} else {
			xlog.Warn("Failed to load skills for native local agent chat", "agent", agentName, "user", userID, "error", err)
		}
	}

	messageSent := false
	callbacks := agents.Callbacks{
		OnReasoning: func(text string) {
			if strings.TrimSpace(text) == "" {
				return
			}
			sendJSON("stream_event", map[string]any{
				"type":    "reasoning",
				"content": text,
			})
		},
		OnToolCall: func(toolName, args string) {
			sendJSON("stream_event", map[string]any{
				"type":      "tool_call",
				"tool_name": toolName,
				"tool_args": args,
			})
		},
		OnToolResult: func(toolName, result string) {
			sendJSON("stream_event", map[string]any{
				"type":        "tool_result",
				"tool_name":   toolName,
				"tool_result": result,
			})
		},
		OnStatus: func(status string) {
			sendStatus(status)
		},
		OnMessage: func(sender, content, id string) {
			messageSent = true
			if sender == "agent" && s.distributed.agentStore != nil {
				if err := s.distributed.agentStore.CompleteAnalysis(userID, agentName, messageID, content, nil); err != nil {
					xlog.Warn("Failed to retain local final analysis", "agent", agentName, "message_id", messageID, "error", err)
				}
			}
			sendJSON("json_message", map[string]any{
				"id":         id,
				"message_id": id,
				"sender":     sender,
				"content":    content,
			})
		},
		OnMessageMetadata: func(sender, content, id string, metadata map[string]any) {
			messageSent = true
			if sender == "agent" && s.distributed.agentStore != nil {
				if err := s.distributed.agentStore.CompleteAnalysis(userID, agentName, messageID, content, metadata); err != nil {
					xlog.Warn("Failed to retain local final analysis metadata", "agent", agentName, "message_id", messageID, "error", err)
				}
			}
			sendJSON("json_message", map[string]any{
				"id": id, "message_id": id, "sender": sender, "content": content, "answer_metadata": metadata,
			})
		},
	}

	if agents.CanRunDeterministicForensicRoute(&cfgCopy, message, userID) {
		if _, handled, err := agents.TryDeterministicForensicRouteContext(ctx, &cfgCopy, message, userID, callbacks, opts); handled {
			if err != nil {
				switch agents.ExecutionTerminalStatus(err) {
				case "timed_out":
					sendStatus("timed_out")
					return
				case "cancelled":
					sendStatus("cancelled")
					return
				}
				sendJSON("json_error", map[string]any{
					"error": "The analysis could not be completed. Please try again or contact your administrator.",
				})
				xlog.Warn("Forensic analysis failed", "agent", agentName, "message_id", messageID, "error", err)
				sendStatus("error")
			}
			return
		}
	}

	response, err := agents.ExecuteChat(ctx, s.apiURL, s.apiKey, &cfgCopy, message, callbacks, opts)
	if err != nil {
		switch agents.ExecutionTerminalStatus(err) {
		case "timed_out":
			sendStatus("timed_out")
			return
		case "cancelled":
			sendStatus("cancelled")
			return
		}
		sendJSON("json_error", map[string]any{
			"error": "The analysis could not be completed. Please try again or contact your administrator.",
		})
		xlog.Warn("Agent analysis failed", "agent", agentName, "message_id", messageID, "error", err)
		sendStatus("error")
		return
	}
	if !messageSent {
		sendJSON("json_message", map[string]any{
			"id":      messageID + "-agent",
			"sender":  "agent",
			"content": response,
		})
	}
}

func (s *AgentPoolService) appendLocalAGIKBCitations(response, agentKey, message string, states []coreTypes.ActionState) string {
	if strings.TrimSpace(response) == "" {
		return response
	}

	userID, collection := splitAgentKey(agentKey)
	cfg := s.localAGI.pool.GetConfig(agentKey)
	if cfg == nil || !cfg.EnableKnowledgeBase {
		return response
	}

	citations := kbCitationsFromActionStates(states)
	if len(citations) == 0 && cfg.KBAutoSearch {
		maxResults := cfg.KnowledgeBaseResults
		if maxResults <= 0 {
			maxResults = 5
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		kbResult := agents.KBAutoSearchPrompt(ctx, s.apiURL, s.apiKey, collection, message, maxResults, userID)
		citations = kbResult.Citations
	}

	return agents.AppendKBCitations(response, collection, userID, citations)
}

func splitAgentKey(agentKey string) (userID, name string) {
	if uid, n, ok := strings.Cut(agentKey, ":"); ok {
		return uid, n
	}
	return "", agentKey
}

func kbCitationsFromActionStates(states []coreTypes.ActionState) []agents.KBCitation {
	var citations []agents.KBCitation
	for _, state := range states {
		citations = append(citations, kbCitationsFromMetadata(state.Metadata)...)
	}
	return citations
}

func kbCitationsFromMetadata(metadata map[string]any) []agents.KBCitation {
	if len(metadata) == 0 {
		return nil
	}

	fileName := metadata["file_name"]
	source := metadata["source"]
	if fileName == nil && source == nil {
		return nil
	}

	citation := agents.KBCitation{
		FileName: metadataString(fileName),
		EntryKey: metadataString(source),
	}
	if citation.FileName == "" && citation.EntryKey == "" {
		return nil
	}
	return []agents.KBCitation{citation}
}

func metadataString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

// userOutputsDir returns the per-user outputs directory, creating it if needed.
// If userID is empty, falls back to the shared outputs directory.
func (s *AgentPoolService) userOutputsDir(userID string) string {
	if userID == "" {
		return s.outputsDir
	}
	dir := filepath.Join(s.outputsDir, userID)
	os.MkdirAll(dir, 0750)
	return dir
}

// copyToOutputs copies a file into the per-user outputs directory and returns the new path.
// If the file is already inside the target dir, it returns the original path unchanged.
func (s *AgentPoolService) copyToOutputs(srcPath, userID string) (string, error) {
	targetDir := s.userOutputsDir(userID)
	srcClean := filepath.Clean(srcPath)
	absTarget, _ := filepath.Abs(targetDir)
	absSrc, _ := filepath.Abs(srcClean)
	if strings.HasPrefix(absSrc, absTarget+string(os.PathSeparator)) {
		return srcPath, nil
	}

	src, err := os.Open(srcClean)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dstPath := filepath.Join(targetDir, filepath.Base(srcClean))
	dst, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	return dstPath, nil
}

// collectAndCopyMetadata iterates all metadata keys and, for any value that is
// a []string of local file paths, copies those files into the per-user outputs
// directory so the file endpoint can serve them from a single confined location.
// Entries that are URLs (http/https) are left unchanged.
func (s *AgentPoolService) collectAndCopyMetadata(metadata map[string]any, userID string) {
	for key, val := range metadata {
		list, ok := val.([]string)
		if !ok {
			continue
		}
		updated := make([]string, 0, len(list))
		for _, p := range list {
			if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
				updated = append(updated, p)
				continue
			}
			newPath, err := s.copyToOutputs(p, userID)
			if err != nil {
				xlog.Error("Failed to copy file to outputs", "src", p, "error", err)
				updated = append(updated, p)
				continue
			}
			updated = append(updated, newPath)
		}
		metadata[key] = updated
	}
}

func (s *AgentPoolService) GetConfigMeta() state.AgentConfigMeta {
	return s.localAGI.configMeta
}

// GetConfigMetaResult returns the config metadata via the backend, which handles
// local vs distributed differences (LocalAGI metadata vs native static metadata).
func (s *AgentPoolService) GetConfigMetaResult() AgentConfigMetaResult {
	return s.configBackend.GetConfigMeta()
}

func (s *AgentPoolService) AgentHubURL() string {
	return s.appConfig.AgentPool.AgentHubURL
}

func (s *AgentPoolService) StateDir() string {
	return s.stateDir
}

func (s *AgentPoolService) OutputsDir() string {
	return s.outputsDir
}

// ExportAgent returns the agent config as JSON bytes.
func (s *AgentPoolService) ExportAgent(name string) ([]byte, error) {
	// Extract userID and agent name from the key (format: "userID:agentName")
	userID := ""
	agentName := name
	if u, a, ok := strings.Cut(name, ":"); ok {
		userID = u
		agentName = a
	}
	return s.configBackend.ExportConfig(userID, agentName)
}

// --- User Services ---

// SetUserServicesManager sets the user services manager for per-user scoping.
func (s *AgentPoolService) SetUserServicesManager(usm *UserServicesManager) {
	s.users.userServices = usm
}

// UserStorage returns the user-scoped storage.
func (s *AgentPoolService) UserStorage() *UserScopedStorage {
	return s.users.userStorage
}

// UserServicesManager returns the user services manager.
func (s *AgentPoolService) UserServicesManager() *UserServicesManager {
	return s.users.userServices
}

// SetAuthDB sets the auth database for API key generation.
// Deprecated: prefer passing AuthDB via AgentPoolOptions at construction time.
func (s *AgentPoolService) SetAuthDB(db *gorm.DB) {
	s.users.authDB = db
}

// SetSkillStore sets the distributed skill store for persisting skill metadata to PostgreSQL.
// Deprecated: prefer passing SkillStore via AgentPoolOptions at construction time.
func (s *AgentPoolService) SetSkillStore(store *distributed.SkillStore) {
	s.distributed.skillStore = store
}

// --- Admin Aggregation ---

// UserAgentInfo holds agent info for cross-user listing.
type UserAgentInfo struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// ListAllAgentsGrouped returns all agents grouped by user ID.
// Keys without ":" go into the "" (root) group.
func (s *AgentPoolService) ListAllAgentsGrouped() map[string][]UserAgentInfo {
	return s.configBackend.ListAllGrouped()
}

// --- ForUser Collections ---

// ListCollectionsForUser lists collections for a specific user.
func (s *AgentPoolService) ListCollectionsForUser(userID string) ([]string, error) {
	backend, err := s.CollectionsBackendForUser(userID)
	if err != nil {
		return nil, err
	}
	return backend.ListCollections()
}

// CreateCollectionForUser creates a collection for a specific user.
func (s *AgentPoolService) CreateCollectionForUser(userID, name string) error {
	backend, err := s.CollectionsBackendForUser(userID)
	if err != nil {
		return err
	}
	return backend.CreateCollection(name)
}

// ensureCollectionForUser creates a collection for the user if it doesn't already exist.
func (s *AgentPoolService) ensureCollectionForUser(userID, name string) error {
	backend, err := s.CollectionsBackendForUser(userID)
	if err != nil {
		return err
	}
	collections, err := backend.ListCollections()
	if err != nil {
		return err
	}
	if slices.Contains(collections, name) {
		return nil
	}
	return backend.CreateCollection(name)
}

// UploadToCollectionForUser uploads to a collection for a specific user.
// The filename arrives from a multipart upload; the vendored backend may or
// may not sanitise it, so strip any directory components at the boundary.
func (s *AgentPoolService) UploadToCollectionForUser(userID, collection, filename string, fileBody io.Reader) (string, error) {
	backend, err := s.CollectionsBackendForUser(userID)
	if err != nil {
		return "", err
	}
	base := filepath.Base(filename)
	if base == "." || base == ".." || base == "/" || base == "" {
		return "", fmt.Errorf("invalid filename")
	}
	return backend.Upload(collection, base, fileBody)
}

// CollectionEntryExistsForUser checks if an entry exists in a user's collection.
func (s *AgentPoolService) CollectionEntryExistsForUser(userID, collection, entry string) bool {
	backend, err := s.CollectionsBackendForUser(userID)
	if err != nil {
		return false
	}
	return backend.EntryExists(collection, entry)
}

// ListCollectionEntriesForUser lists entries in a user's collection.
func (s *AgentPoolService) ListCollectionEntriesForUser(userID, collection string) ([]string, error) {
	backend, err := s.CollectionsBackendForUser(userID)
	if err != nil {
		return nil, err
	}
	return backend.ListEntries(collection)
}

// GetCollectionEntryContentForUser gets entry content for a user's collection.
func (s *AgentPoolService) GetCollectionEntryContentForUser(userID, collection, entry string) (string, int, error) {
	backend, err := s.CollectionsBackendForUser(userID)
	if err != nil {
		return "", 0, err
	}
	content, chunks, err := backend.GetEntryContent(collection, entry)
	if err == nil {
		return content, chunks, nil
	}
	if !isPlainTextCollectionEntry(entry) || !strings.Contains(strings.ToLower(err.Error()), "unsupported file type") {
		return "", 0, err
	}
	fpath, pathErr := backend.GetEntryFilePath(collection, entry)
	if pathErr != nil {
		return "", 0, err
	}
	plainText, readErr := readPlainTextCollectionPreview(fpath)
	if readErr != nil {
		return "", 0, readErr
	}
	if plainText == "" {
		return "", 0, nil
	}
	return plainText, 1, nil
}

func isPlainTextCollectionEntry(entry string) bool {
	switch strings.ToLower(filepath.Ext(entry)) {
	case ".csv", ".tsv", ".txt", ".log", ".json", ".jsonl", ".ndjson", ".md", ".yaml", ".yml", ".cdr":
		return true
	default:
		return false
	}
}

func readPlainTextCollectionPreview(fpath string) (string, error) {
	file, err := os.Open(filepath.Clean(fpath))
	if err != nil {
		return "", err
	}
	defer file.Close()

	limited := io.LimitReader(file, maxPlainTextCollectionPreviewBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if len(data) > maxPlainTextCollectionPreviewBytes {
		data = data[:maxPlainTextCollectionPreviewBytes]
		data = append(data, []byte("\n\n[preview truncated]")...)
	}
	return string(data), nil
}

// SearchCollectionForUser searches a user's collection.
func (s *AgentPoolService) SearchCollectionForUser(userID, collection, query string, maxResults int) ([]collections.SearchResult, error) {
	backend, err := s.CollectionsBackendForUser(userID)
	if err != nil {
		return nil, err
	}
	return backend.Search(collection, query, maxResults)
}

// ResetCollectionForUser resets a user's collection.
func (s *AgentPoolService) ResetCollectionForUser(userID, collection string) error {
	backend, err := s.CollectionsBackendForUser(userID)
	if err != nil {
		return err
	}
	return backend.Reset(collection)
}

// DeleteCollectionEntryForUser deletes an entry from a user's collection.
func (s *AgentPoolService) DeleteCollectionEntryForUser(userID, collection, entry string) ([]string, error) {
	backend, err := s.CollectionsBackendForUser(userID)
	if err != nil {
		return nil, err
	}
	return backend.DeleteEntry(collection, entry)
}

// AddCollectionSourceForUser adds a source to a user's collection.
func (s *AgentPoolService) AddCollectionSourceForUser(userID, collection, sourceURL string, intervalMin int) error {
	backend, err := s.CollectionsBackendForUser(userID)
	if err != nil {
		return err
	}
	return backend.AddSource(collection, sourceURL, intervalMin)
}

// RemoveCollectionSourceForUser removes a source from a user's collection.
func (s *AgentPoolService) RemoveCollectionSourceForUser(userID, collection, sourceURL string) error {
	backend, err := s.CollectionsBackendForUser(userID)
	if err != nil {
		return err
	}
	return backend.RemoveSource(collection, sourceURL)
}

// ListCollectionSourcesForUser lists sources for a user's collection.
func (s *AgentPoolService) ListCollectionSourcesForUser(userID, collection string) ([]collections.SourceInfo, error) {
	backend, err := s.CollectionsBackendForUser(userID)
	if err != nil {
		return nil, err
	}
	return backend.ListSources(collection)
}

// GetCollectionEntryFilePathForUser gets the file path for an entry in a user's collection.
func (s *AgentPoolService) GetCollectionEntryFilePathForUser(userID, collection, entry string) (string, error) {
	backend, err := s.CollectionsBackendForUser(userID)
	if err != nil {
		return "", err
	}
	return backend.GetEntryFilePath(collection, entry)
}

// --- ForUser Agent Methods ---

// ListAgentsForUser lists agents belonging to a specific user.
// If userID is empty, returns all agents (backward compat).
func (s *AgentPoolService) ListAgentsForUser(userID string) map[string]bool {
	return s.configBackend.ListAgents(userID)
}

// CreateAgentForUser creates an agent namespaced to a user.
// When auth is enabled and the agent config has no API key, a new user API key
// is auto-generated so the agent can authenticate against LocalAI's own API.
func (s *AgentPoolService) CreateAgentForUser(userID string, config *state.AgentConfig) error {
	return s.createAgentForUserWithCollection(userID, config, config.Name)
}

func (s *AgentPoolService) createAgentForUserWithCollection(userID string, config *state.AgentConfig, collectionName string) error {
	if err := ValidateAgentName(config.Name); err != nil {
		return err
	}

	// Auto-generate a user API key when auth is active and none is specified
	if s.users.authDB != nil && userID != "" && config.APIKey == "" {
		plaintext, _, err := auth.CreateAPIKey(s.users.authDB, userID, "agent:"+config.Name, "user", s.appConfig.Auth.APIKeyHMACSecret, nil)
		if err != nil {
			return fmt.Errorf("failed to create API key for agent: %w", err)
		}
		config.APIKey = plaintext
		xlog.Info("Auto-generated API key for agent", "agent", config.Name, "user", userID)
	}

	if err := s.configBackend.SaveConfig(userID, config); err != nil {
		return err
	}

	// Auto-create collection when knowledge base or long-term memory is enabled
	if config.EnableKnowledgeBase || config.LongTermMemory {
		if err := s.ensureCollectionForUser(userID, collectionName); err != nil {
			xlog.Warn("Failed to ensure agent knowledge collection", "agent", config.Name, "collection", collectionName, "error", err)
		}
	}

	return nil
}

// GetAgentForUser returns the agent for a user.
// Returns nil in distributed mode where agents don't run in-process.
func (s *AgentPoolService) GetAgentForUser(userID, name string) *agent.Agent {
	return s.configBackend.GetAgent(userID, name)
}

// GetAgentConfigForUser returns the agent config for a user's agent.
func (s *AgentPoolService) GetAgentConfigForUser(userID, name string) *state.AgentConfig {
	return s.configBackend.GetConfig(userID, name)
}

// GetNativeAgentConfigForUser returns the full native agent config when one is
// available. Local mode persists this alongside the LocalAGI-compatible config
// so newer LocalAI-only fields are not lost on save/edit/restart.
func (s *AgentPoolService) GetNativeAgentConfigForUser(userID, name string) *agents.AgentConfig {
	cfg, err := s.nativeAgentConfigForUser(userID, name)
	if err != nil {
		xlog.Warn("Failed to load native agent config", "agent", name, "user", userID, "error", err)
		return nil
	}
	return cfg
}

// CreateNativeAgentForUser creates an agent from the native LocalAI config,
// preserving fields that LocalAGI's legacy AgentConfig does not know about.
func (s *AgentPoolService) CreateNativeAgentForUser(userID string, config *agents.AgentConfig) error {
	if config == nil {
		return fmt.Errorf("missing agent config")
	}
	localConfig, err := stateConfigFromNative(config)
	if err != nil {
		return fmt.Errorf("invalid agent config: %w", err)
	}
	if err := s.createAgentForUserWithCollection(userID, localConfig, config.KnowledgeCollectionName()); err != nil {
		return err
	}
	config.APIKey = localConfig.APIKey
	if s.distributed.agentStore != nil {
		return s.saveDistributedNativeAgentConfig(userID, config)
	}
	return s.saveNativeAgentConfig(userID, config)
}

// UpdateAgentForUser updates a user's agent.
func (s *AgentPoolService) UpdateAgentForUser(userID, name string, config *state.AgentConfig) error {
	return s.updateAgentForUserWithCollection(userID, name, config, config.Name)
}

func (s *AgentPoolService) updateAgentForUserWithCollection(userID, name string, config *state.AgentConfig, collectionName string) error {
	// Auto-generate a user API key when auth is active and none is specified
	if s.users.authDB != nil && userID != "" && config.APIKey == "" {
		plaintext, _, err := auth.CreateAPIKey(s.users.authDB, userID, "agent:"+name, "user", s.appConfig.Auth.APIKeyHMACSecret, nil)
		if err != nil {
			return fmt.Errorf("failed to create API key for agent: %w", err)
		}
		config.APIKey = plaintext
	}

	if err := s.configBackend.UpdateConfig(userID, name, config); err != nil {
		return err
	}

	// Auto-create collection when knowledge base or long-term memory is enabled
	if config.EnableKnowledgeBase || config.LongTermMemory {
		if err := s.ensureCollectionForUser(userID, collectionName); err != nil {
			xlog.Warn("Failed to ensure agent knowledge collection", "agent", config.Name, "collection", collectionName, "error", err)
		}
	}

	return nil
}

// UpdateNativeAgentForUser updates an agent from the native LocalAI config.
func (s *AgentPoolService) UpdateNativeAgentForUser(userID, name string, config *agents.AgentConfig) error {
	if config == nil {
		return fmt.Errorf("missing agent config")
	}
	if config.Name == "" {
		config.Name = name
	}
	localConfig, err := stateConfigFromNative(config)
	if err != nil {
		return fmt.Errorf("invalid agent config: %w", err)
	}
	if err := s.updateAgentForUserWithCollection(userID, name, localConfig, config.KnowledgeCollectionName()); err != nil {
		return err
	}
	config.APIKey = localConfig.APIKey
	if s.distributed.agentStore != nil {
		return s.saveDistributedNativeAgentConfig(userID, config)
	}
	return s.saveNativeAgentConfig(userID, config)
}

// DeleteAgentForUser deletes a user's agent.
func (s *AgentPoolService) DeleteAgentForUser(userID, name string) error {
	if err := s.configBackend.DeleteConfig(userID, name); err != nil {
		return err
	}
	if err := s.removeNativeAgentConfig(userID, name); err != nil {
		xlog.Warn("Failed to remove native agent config", "agent", name, "user", userID, "error", err)
	}
	return nil
}

// PauseAgentForUser pauses a user's agent.
func (s *AgentPoolService) PauseAgentForUser(userID, name string) error {
	return s.configBackend.SetStatus(userID, name, "paused")
}

// ResumeAgentForUser resumes a user's agent.
func (s *AgentPoolService) ResumeAgentForUser(userID, name string) error {
	return s.configBackend.SetStatus(userID, name, "active")
}

// GetAgentStatusForUser returns the status of a user's agent.
// Returns nil in distributed mode where status is not tracked in-process.
func (s *AgentPoolService) GetAgentStatusForUser(userID, name string) *state.Status {
	return s.configBackend.GetStatus(userID, name)
}

// GetAgentObservablesForUser returns observables for a user's agent as raw JSON entries.
func (s *AgentPoolService) GetAgentObservablesForUser(userID, name string) ([]json.RawMessage, error) {
	return s.configBackend.GetObservables(userID, name)
}

// ClearAgentObservablesForUser clears observables for a user's agent.
func (s *AgentPoolService) ClearAgentObservablesForUser(userID, name string) error {
	return s.configBackend.ClearObservables(userID, name)
}

// ChatForUser sends a message to a user's agent.
func (s *AgentPoolService) ChatForUser(userID, name, message string) (string, error) {
	if messageID, handled, err := s.tryDeterministicForensicChatForUser(userID, name, message); handled || err != nil {
		return messageID, err
	}
	return s.configBackend.Chat(userID, name, message)
}

// ChatForUserWithForensicScope executes one chat against a request-bound case.
// The stored agent configuration is copied and never mutated. This is the
// enforcement point behind the URL-backed Case Workspace selector: both
// deterministic direct routes and model-selected forensic tools receive the
// same authoritative collection ID.
func (s *AgentPoolService) ChatForUserWithForensicScope(userID, name, message, collectionID string, queryScope *agents.ForensicQueryScope, conversationContext ...agents.ForensicConversationContext) (string, error) {
	messageID := fmt.Sprintf("%d", time.Now().UnixNano())
	s.recordAnalysisAccepted(userID, name, collectionID, messageID, message, "")
	var boundedContext *agents.ForensicConversationContext
	if len(conversationContext) > 0 {
		contextCopy := conversationContext[0]
		boundedContext = &contextCopy
	}
	acceptedID, err := s.chatForUserWithForensicScopeID(userID, name, message, collectionID, messageID, queryScope, boundedContext)
	if err != nil && s.distributed.agentStore != nil {
		_ = s.distributed.agentStore.UpdateAnalysisStatus(userID, name, collectionID, messageID, "error: dispatch failed")
	}
	return acceptedID, err
}

func (s *AgentPoolService) chatForUserWithForensicScopeID(userID, name, message, collectionID, messageID string, queryScope *agents.ForensicQueryScope, conversationContext *agents.ForensicConversationContext) (string, error) {
	collectionID = strings.TrimSpace(collectionID)
	if collectionID == "" {
		return "", errors.New("forensic collection scope is required")
	}
	cfg, err := s.nativeAgentConfigForUser(userID, name)
	if err != nil {
		return "", fmt.Errorf("invalid agent config: %w", err)
	}
	if cfg == nil {
		return "", fmt.Errorf("agent not found: %s", name)
	}
	if !cfg.EnableForensicRecords {
		return "", fmt.Errorf("agent %s is not enabled for forensic case-scoped chat", name)
	}
	cfgCopy := *cfg
	cfgCopy.ForensicCollectionID = collectionID
	cfgCopy.ForensicConversationContext = conversationContext
	cfgCopy.ForensicQueryScope = queryScope

	if s.distributed.natsClient != nil {
		return s.dispatchChatWithConfigID(userID, name, message, &cfgCopy, messageID)
	}
	manager := s.configBackend.GetSSEManager(userID, name)
	if manager == nil {
		return "", fmt.Errorf("SSE manager not found for agent: %s", name)
	}
	if messageID == "" {
		messageID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	s.chatStatuses.Record(name, userID, collectionID, messageID, "processing")
	sendAgentChatStarted(manager, message, messageID)
	s.startNativeLocalChat(agents.AgentKey(userID, name), message, messageID, &cfgCopy, manager)
	return messageID, nil
}

// ChatRetryResult records the retry lineage and whether this call created the
// dispatch or replayed an existing idempotent acknowledgement.
type ChatRetryResult struct {
	MessageID        string `json:"message_id"`
	RetryOf          string `json:"retry_of"`
	CaseID           string `json:"case_id"`
	IdempotentReplay bool   `json:"idempotent_replay"`
}

// RetryChatForUser performs one explicit forensic retry. It never retries
// automatically and reserves the idempotency key before dispatch.
func (s *AgentPoolService) RetryChatForUser(userID, name, caseID, originalMessageID, message, idempotencyKey string, queryScope *agents.ForensicQueryScope) (ChatRetryResult, error) {
	status, ok := s.ChatStatusForUser(userID, name, caseID, originalMessageID)
	if !ok {
		return ChatRetryResult{}, errors.New("original request status not found")
	}
	if !agents.IsRetryEligibleStatus(status.Status) {
		return ChatRetryResult{}, agents.ErrRetryNotEligible
	}

	retryID := fmt.Sprintf("%d", time.Now().UnixNano())
	requested := agents.ChatRetryReservation{
		AgentName: name, UserID: userID, CaseID: caseID,
		OriginalMessageID: originalMessageID, RetryMessageID: retryID,
		IdempotencyKeyHash: agents.ChatRetryMessageHash(idempotencyKey), MessageHash: agents.ChatRetryMessageHash(message),
	}
	reservationID := agents.ChatRetryReservationID(name, userID, caseID, idempotencyKey)
	var reservation agents.ChatRetryReservation
	var created bool
	var err error
	if s.distributed.agentStore != nil {
		reservation, created, err = s.distributed.agentStore.ReserveChatRetry(reservationID, agents.AgentKey(userID, name), requested)
	} else {
		reservation, created, err = s.chatRetries.Reserve(reservationID, requested)
	}
	if err != nil {
		return ChatRetryResult{}, err
	}
	result := ChatRetryResult{
		MessageID: reservation.RetryMessageID, RetryOf: reservation.OriginalMessageID,
		CaseID: reservation.CaseID, IdempotentReplay: !created,
	}
	if !created {
		return result, nil
	}
	s.recordAnalysisAccepted(userID, name, caseID, reservation.RetryMessageID, message, originalMessageID)
	if _, err := s.chatForUserWithForensicScopeID(userID, name, message, caseID, reservation.RetryMessageID, queryScope, nil); err != nil {
		s.chatStatuses.Record(name, userID, caseID, reservation.RetryMessageID, "error: retry dispatch failed")
		if s.distributed.agentStore != nil {
			_ = s.distributed.agentStore.UpdateAnalysisStatus(userID, name, caseID, reservation.RetryMessageID, "error: retry dispatch failed")
		}
		return ChatRetryResult{}, err
	}
	return result, nil
}

func (s *AgentPoolService) analysisScopeForUser(userID, name, caseID string) (agents.AnalysisHistoryScope, error) {
	if s.distributed.agentStore == nil {
		return agents.AnalysisHistoryScope{}, agents.ErrAnalysisHistoryUnavailable
	}
	cfg, err := s.nativeAgentConfigForUser(userID, name)
	if err != nil || cfg == nil {
		return agents.AnalysisHistoryScope{}, fmt.Errorf("agent not found: %s", name)
	}
	if !cfg.EnableForensicRecords {
		return agents.AnalysisHistoryScope{}, fmt.Errorf("agent %s is not enabled for forensic case history", name)
	}
	tenantID := strings.TrimSpace(cfg.ForensicTenantID)
	if tenantID == "" {
		tenantID = "default"
	}
	return agents.AnalysisHistoryScope{
		TenantID: tenantID, UserID: userID, AgentName: name,
		CaseID: caseID, CollectionID: caseID,
	}, nil
}

func (s *AgentPoolService) recordAnalysisAccepted(userID, name, caseID, messageID, message, retryOf string) {
	if messageID == "" || s.distributed.agentStore == nil {
		return
	}
	scope, err := s.analysisScopeForUser(userID, name, caseID)
	if err != nil {
		return
	}
	cfg := s.GetNativeAgentConfigForUser(userID, name)
	authority, modelRole := "specialist_agent", "bounded_specialist_synthesis"
	if agents.CanRunDeterministicForensicRoute(cfg, message, userID) {
		authority, modelRole = "deterministic_records", "explanation_after_exact_analysis"
	}
	_, err = s.distributed.agentStore.CreateAnalysis(&agents.AgentAnalysisRecord{
		TenantID: scope.TenantID, UserID: userID, AgentName: name,
		CaseID: caseID, CollectionID: caseID, MessageID: messageID,
		RetryOf: retryOf, QueryText: message, Status: "processing",
		ExecutionAuthority: authority, ModelRole: modelRole,
	})
	if err != nil {
		xlog.Warn("Failed to retain accepted analysis", "agent", name, "message_id", messageID, "error", err)
	}
}

func (s *AgentPoolService) AnalysisHistoryForUser(userID, name, caseID string, opts agents.AnalysisHistoryListOptions) (agents.AnalysisHistoryPage, error) {
	scope, err := s.analysisScopeForUser(userID, name, caseID)
	if err != nil {
		return agents.AnalysisHistoryPage{}, err
	}
	return s.distributed.agentStore.ListAnalysisHistory(scope, opts)
}

func (s *AgentPoolService) AnalysisForUser(userID, name, caseID, analysisID string) (agents.AnalysisHistoryEntry, error) {
	scope, err := s.analysisScopeForUser(userID, name, caseID)
	if err != nil {
		return agents.AnalysisHistoryEntry{}, err
	}
	return s.distributed.agentStore.GetAnalysis(scope, analysisID)
}

func (s *AgentPoolService) SetAnalysisSavedForUser(userID, name, caseID, analysisID string, saved bool, title string) (agents.AnalysisHistoryEntry, error) {
	scope, err := s.analysisScopeForUser(userID, name, caseID)
	if err != nil {
		return agents.AnalysisHistoryEntry{}, err
	}
	entry, err := s.distributed.agentStore.SetAnalysisSaved(scope, analysisID, saved, title, userID)
	if err == nil {
		s.distributed.agentStore.AppendObservable(&agents.AgentObservableRecord{
			AgentName: agents.AgentKey(userID, name), EventType: "analysis_saved_state",
			PayloadJSON: string(agents.JSONPayload(map[string]any{"analysis_id": analysisID, "case_id": caseID, "saved": saved})),
		})
	}
	return entry, err
}

func (s *AgentPoolService) ImportBrowserAnalysisHistory(userID, name, caseID string, conversations []agents.BrowserHistoryConversation) (agents.BrowserHistoryImportResult, error) {
	scope, err := s.analysisScopeForUser(userID, name, caseID)
	if err != nil {
		return agents.BrowserHistoryImportResult{}, err
	}
	importID := uuid.New().String()
	result := agents.BrowserHistoryImportResult{ContractVersion: agents.AnalysisHistoryContractV1, ImportID: importID, RollbackAvailable: true}
	for _, conversation := range conversations {
		for index, message := range conversation.Messages {
			if message.Sender != "user" || strings.TrimSpace(message.Content) == "" {
				continue
			}
			answer := ""
			metadata := map[string]any{}
			var answerTimestamp int64
			for next := index + 1; next < len(conversation.Messages); next++ {
				candidate := conversation.Messages[next]
				if candidate.Sender == "user" {
					break
				}
				if candidate.Sender == "agent" {
					answer, metadata = candidate.Content, candidate.Metadata
					answerTimestamp = candidate.Timestamp
					break
				}
			}
			createdAt := time.Now().UTC()
			if message.Timestamp > 0 {
				createdAt = time.UnixMilli(message.Timestamp).UTC()
			}
			status := "imported_incomplete"
			var completedAt *time.Time
			if answer != "" {
				status = "completed"
				completionTime := createdAt
				if answerTimestamp > 0 {
					completionTime = time.UnixMilli(answerTimestamp).UTC()
				}
				completedAt = &completionTime
			}
			created, createErr := s.distributed.agentStore.CreateAnalysis(&agents.AgentAnalysisRecord{
				TenantID: scope.TenantID, UserID: userID, AgentName: name,
				CaseID: caseID, CollectionID: caseID,
				MessageID: agents.BrowserImportMessageID(userID, name, caseID, conversation.ID, index, message.Content),
				QueryText: message.Content, AnswerText: answer, AnswerMetadata: string(agents.JSONPayload(metadata)),
				Status: status, ExecutionAuthority: "historical_unknown", ModelRole: "historical_unknown",
				Source: agents.AnalysisSourceBrowserImport, ImportID: importID,
				CreatedAt: createdAt, CompletedAt: completedAt,
			})
			if createErr != nil {
				return result, createErr
			}
			if created {
				result.Imported++
			} else {
				result.Skipped++
			}
		}
	}
	result.RollbackAvailable = result.Imported > 0
	s.distributed.agentStore.AppendObservable(&agents.AgentObservableRecord{
		AgentName: agents.AgentKey(userID, name), EventType: "analysis_history_import",
		PayloadJSON: string(agents.JSONPayload(map[string]any{"import_id": importID, "case_id": caseID, "imported": result.Imported, "skipped": result.Skipped})),
	})
	return result, nil
}

func (s *AgentPoolService) RollbackBrowserAnalysisImport(userID, name, caseID, importID string) (agents.AnalysisImportRollbackResult, error) {
	scope, err := s.analysisScopeForUser(userID, name, caseID)
	if err != nil {
		return agents.AnalysisImportRollbackResult{}, err
	}
	result, err := s.distributed.agentStore.RollbackAnalysisImport(scope, importID)
	if err == nil {
		s.distributed.agentStore.AppendObservable(&agents.AgentObservableRecord{
			AgentName: agents.AgentKey(userID, name), EventType: "analysis_history_import_rollback",
			PayloadJSON: string(agents.JSONPayload(result)),
		})
	}
	return result, err
}

// CancelChatForUser cancels only the exact user/agent/case/request tuple.
func (s *AgentPoolService) CancelChatForUser(userID, name, caseID, messageID string) error {
	key := agents.AgentCancellationKey(name, userID, caseID, messageID)
	if s.chatCancels.Cancel(key) {
		return nil
	}
	if s.distributed.eventBridge != nil {
		return s.distributed.eventBridge.CancelExecution(name, userID, caseID, messageID)
	}
	return nil
}

// ChatStatusForUser returns only the exact user/agent/case/request lifecycle snapshot.
func (s *AgentPoolService) ChatStatusForUser(userID, name, caseID, messageID string) (agents.AgentRequestStatus, bool) {
	if status, ok := s.chatStatuses.Get(name, userID, caseID, messageID); ok {
		return status, true
	}
	if s.distributed.eventBridge != nil {
		return s.distributed.eventBridge.RequestStatus(name, userID, caseID, messageID)
	}
	return agents.AgentRequestStatus{}, false
}

func sendAgentChatStarted(manager sse.Manager, message, messageID string) {
	userMsg, _ := json.Marshal(map[string]any{
		"id": messageID + "-user", "message_id": messageID + "-user", "sender": "user", "content": message, "timestamp": time.Now().Format(time.RFC3339),
	})
	manager.Send(sse.NewMessage(string(userMsg)).WithEvent("json_message"))
	statusMsg, _ := json.Marshal(map[string]any{
		"status": "processing", "message_id": messageID, "timestamp": time.Now().Format(time.RFC3339),
	})
	manager.Send(sse.NewMessage(string(statusMsg)).WithEvent("json_message_status"))
}

func (s *AgentPoolService) tryDeterministicForensicChatForUser(userID, name, message string) (string, bool, error) {
	cfg, err := s.nativeAgentConfigForUser(userID, name)
	if err != nil {
		return "", false, fmt.Errorf("invalid agent config: %w", err)
	}
	if !agents.CanRunDeterministicForensicRoute(cfg, message, userID) {
		return "", false, nil
	}

	messageID := fmt.Sprintf("%d", time.Now().UnixNano())
	xlog.Info("Routing forensic chat directly to deterministic records API", "agent", name, "user", userID, "collection", cfg.ForensicCollectionID)

	if s.distributed.eventBridge != nil {
		_ = s.distributed.eventBridge.PublishMessage(name, userID, "user", message, messageID+"-user")
		_ = s.distributed.eventBridge.PublishStatus(name, userID, messageID, "processing", cfg.ForensicCollectionID)
		go s.runDistributedDeterministicForensicChat(userID, name, message, messageID, cfg)
		return messageID, true, nil
	}

	manager := s.configBackend.GetSSEManager(userID, name)
	if manager == nil {
		return "", false, nil
	}
	userMsg, _ := json.Marshal(map[string]any{
		"id":         messageID + "-user",
		"message_id": messageID + "-user",
		"sender":     "user",
		"content":    message,
		"timestamp":  time.Now().Format(time.RFC3339),
	})
	manager.Send(sse.NewMessage(string(userMsg)).WithEvent("json_message"))
	statusMsg, _ := json.Marshal(map[string]any{
		"status":     "processing",
		"message_id": messageID,
		"timestamp":  time.Now().Format(time.RFC3339),
	})
	manager.Send(sse.NewMessage(string(statusMsg)).WithEvent("json_message_status"))
	s.chatStatuses.Record(name, userID, cfg.ForensicCollectionID, messageID, "processing")
	s.startNativeLocalChat(agents.AgentKey(userID, name), message, messageID, cfg, manager)
	return messageID, true, nil
}

// dispatchChat publishes a chat event to the NATS agent execution queue.
// The event is enriched with the full agent config and resolved skills so that
// the worker does not need direct database access.
func (s *AgentPoolService) dispatchChat(userID, name, message string) (string, error) {
	return s.dispatchChatWithConfig(userID, name, message, nil)
}

func (s *AgentPoolService) dispatchChatWithConfig(userID, name, message string, suppliedCfg *agents.AgentConfig) (string, error) {
	return s.dispatchChatWithConfigID(userID, name, message, suppliedCfg, "")
}

func (s *AgentPoolService) dispatchChatWithConfigID(userID, name, message string, suppliedCfg *agents.AgentConfig, messageID string) (string, error) {
	if messageID == "" {
		messageID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	// Send user message to SSE immediately so the UI shows it right away.
	if s.distributed.eventBridge != nil {
		agentName := name
		s.distributed.eventBridge.PublishMessage(agentName, userID, "user", message, messageID+"-user")
	}

	// Load the native LocalAI config to embed in the NATS payload. This keeps
	// LocalAI-only fields such as enable_forensic_records available to the
	// worker instead of losing them through the legacy LocalAGI config shape.
	cfg := suppliedCfg
	if cfg == nil {
		var err error
		cfg, err = s.nativeAgentConfigForUser(userID, name)
		if err != nil {
			return "", fmt.Errorf("invalid agent config: %w", err)
		}
	}
	if cfg == nil && s.distributed.agentStore != nil {
		rec, err := s.distributed.agentStore.GetConfig(userID, name)
		if err != nil {
			return "", fmt.Errorf("agent config not found: %w", err)
		}
		var c agents.AgentConfig
		if err := agents.ParseConfigJSON(rec.ConfigJSON, &c); err != nil {
			return "", fmt.Errorf("invalid agent config: %w", err)
		}
		cfg = &c
	}
	caseID := ""
	if cfg != nil && cfg.EnableForensicRecords {
		caseID = cfg.ForensicCollectionID
	}
	if s.distributed.eventBridge != nil {
		_ = s.distributed.eventBridge.PublishStatus(name, userID, messageID, "processing", caseID)
	}

	// Load skills if enabled — uses SkillManager which reads from filesystem/PostgreSQL
	if cfg != nil && s.distributed.eventBridge != nil && agents.CanRunDeterministicForensicRoute(cfg, message, userID) {
		go s.runDistributedDeterministicForensicChat(userID, name, message, messageID, cfg)
		return messageID, nil
	}

	var skills []agents.SkillInfo
	if cfg != nil && cfg.EnableSkills {
		if loaded, err := s.loadSkillsForUser(userID); err == nil {
			skills = loaded
		}
	}

	evt := agents.AgentChatEvent{
		AgentName:                   name,
		UserID:                      userID,
		Message:                     message,
		MessageID:                   messageID,
		Role:                        "user",
		CaseID:                      caseID,
		ForensicConversationContext: cfg.ForensicConversationContext,
		ForensicQueryScope:          cfg.ForensicQueryScope,
		DeadlineUnixMilli:           agents.AgentChatDeadlineUnixMilli(),
		Config:                      cfg,
		Skills:                      skills,
	}
	if err := s.distributed.natsClient.Publish(messaging.SubjectAgentExecute, evt); err != nil {
		return "", fmt.Errorf("failed to dispatch agent chat: %w", err)
	}
	return messageID, nil
}

func (s *AgentPoolService) runDistributedDeterministicForensicChat(userID, name, message, messageID string, cfg *agents.AgentConfig) {
	if s.distributed.eventBridge == nil {
		return
	}
	ctx, cancel := agents.WithAgentChatDeadline(context.Background(), 0)
	cancelKey := agents.AgentCancellationKey(name, userID, cfg.ForensicCollectionID, messageID)
	s.distributed.eventBridge.RegisterCancel(cancelKey, cancel)
	defer s.distributed.eventBridge.DeregisterCancel(cancelKey)
	defer cancel()
	opts := agents.ExecuteChatOpts{
		APIURL:    s.apiURL,
		APIKey:    s.apiKey,
		UserID:    userID,
		MessageID: messageID,
	}
	cb := agents.Callbacks{
		OnToolCall: func(toolName, args string) {
			_ = s.distributed.eventBridge.PublishStreamEvent(name, userID, messageID, map[string]any{
				"type":      "tool_call",
				"tool_name": toolName,
				"tool_args": args,
				"timestamp": time.Now().Format(time.RFC3339),
			})
		},
		OnToolResult: func(toolName, result string) {
			_ = s.distributed.eventBridge.PublishStreamEvent(name, userID, messageID, map[string]any{
				"type":        "tool_result",
				"tool_name":   toolName,
				"tool_result": result,
				"timestamp":   time.Now().Format(time.RFC3339),
			})
		},
		OnStatus: func(status string) {
			_ = s.distributed.eventBridge.PublishStatus(name, userID, messageID, status, cfg.ForensicCollectionID)
		},
		OnMessage: func(sender, content, msgID string) {
			_ = s.distributed.eventBridge.PublishMessage(name, userID, sender, content, msgID)
		},
		OnMessageMetadata: func(sender, content, msgID string, metadata map[string]any) {
			_ = s.distributed.eventBridge.PublishMessage(name, userID, sender, content, msgID, metadata)
		},
	}
	if _, handled, err := agents.TryDeterministicForensicRouteContext(ctx, cfg, message, userID, cb, opts); handled {
		if err != nil {
			switch agents.ExecutionTerminalStatus(err) {
			case "timed_out":
				_ = s.distributed.eventBridge.PublishStatus(name, userID, messageID, "timed_out", cfg.ForensicCollectionID)
				return
			case "cancelled":
				_ = s.distributed.eventBridge.PublishStatus(name, userID, messageID, "cancelled", cfg.ForensicCollectionID)
				return
			}
			xlog.Error("deterministic forensic dispatch failed", "agent", name, "error", err)
			_ = s.distributed.eventBridge.PublishStatus(name, userID, messageID, "error: "+err.Error(), cfg.ForensicCollectionID)
			_ = s.distributed.eventBridge.PublishMessage(name, userID, "agent", "Records analysis failed: "+err.Error(), messageID)
		}
		return
	}
	xlog.Warn("deterministic forensic dispatch was requested but no route handled the message", "agent", name)
	_ = s.distributed.eventBridge.PublishStatus(name, userID, messageID, "error: deterministic forensic route unavailable", cfg.ForensicCollectionID)
}

// GetSSEManagerForUser returns the SSE manager for a user's agent.
// Returns nil in distributed mode where SSE is handled by EventBridge.
func (s *AgentPoolService) GetSSEManagerForUser(userID, name string) sse.Manager {
	return s.configBackend.GetSSEManager(userID, name)
}

// ExportAgentForUser exports a user's agent config.
func (s *AgentPoolService) ExportAgentForUser(userID, name string) ([]byte, error) {
	if cfg, err := s.nativeAgentConfigForUser(userID, name); err != nil {
		return nil, err
	} else if cfg != nil {
		return json.MarshalIndent(cfg, "", "  ")
	}
	return s.ExportAgent(agents.AgentKey(userID, name))
}

// ImportAgentForUser imports an agent for a user.
func (s *AgentPoolService) ImportAgentForUser(userID string, data []byte) error {
	var nativeCfg agents.AgentConfig
	if err := json.Unmarshal(data, &nativeCfg); err == nil && nativeCfg.Name != "" {
		return s.ImportNativeAgentForUser(userID, &nativeCfg)
	}

	var cfg state.AgentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("invalid agent config: %w", err)
	}
	if err := ValidateAgentName(cfg.Name); err != nil {
		return err
	}

	// Auto-generate a user API key when auth is active and none is specified
	if s.users.authDB != nil && userID != "" && cfg.APIKey == "" {
		plaintext, _, err := auth.CreateAPIKey(s.users.authDB, userID, "agent:"+cfg.Name, "user", s.appConfig.Auth.APIKeyHMACSecret, nil)
		if err != nil {
			return fmt.Errorf("failed to create API key for agent: %w", err)
		}
		cfg.APIKey = plaintext
	}

	return s.configBackend.ImportConfig(userID, &cfg)
}

// ImportNativeAgentForUser imports an agent from the native LocalAI config.
func (s *AgentPoolService) ImportNativeAgentForUser(userID string, cfg *agents.AgentConfig) error {
	if cfg == nil {
		return fmt.Errorf("missing agent config")
	}
	if err := ValidateAgentName(cfg.Name); err != nil {
		return err
	}
	localConfig, err := stateConfigFromNative(cfg)
	if err != nil {
		return fmt.Errorf("invalid agent config: %w", err)
	}
	if s.users.authDB != nil && userID != "" && localConfig.APIKey == "" {
		plaintext, _, err := auth.CreateAPIKey(s.users.authDB, userID, "agent:"+cfg.Name, "user", s.appConfig.Auth.APIKeyHMACSecret, nil)
		if err != nil {
			return fmt.Errorf("failed to create API key for agent: %w", err)
		}
		localConfig.APIKey = plaintext
		cfg.APIKey = plaintext
	}
	if err := s.configBackend.ImportConfig(userID, localConfig); err != nil {
		return err
	}
	if cfg.EnableKnowledgeBase || cfg.LongTermMemory {
		collectionName := cfg.KnowledgeCollectionName()
		if err := s.ensureCollectionForUser(userID, collectionName); err != nil {
			xlog.Warn("Failed to ensure agent knowledge collection", "agent", cfg.Name, "collection", collectionName, "error", err)
		}
	}
	if s.distributed.agentStore != nil {
		return s.saveDistributedNativeAgentConfig(userID, cfg)
	}
	return s.saveNativeAgentConfig(userID, cfg)
}

func (s *AgentPoolService) saveDistributedNativeAgentConfig(userID string, cfg *agents.AgentConfig) error {
	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal native agent config: %w", err)
	}
	return s.distributed.agentStore.SaveConfig(&agents.AgentConfigRecord{
		UserID:     userID,
		Name:       cfg.Name,
		ConfigJSON: string(configJSON),
		Status:     agents.StatusActive,
	})
}

// --- ForUser Collections ---

// CollectionsBackendForUser returns the collections backend for a user.
func (s *AgentPoolService) CollectionsBackendForUser(userID string) (collections.Backend, error) {
	if s.users.userServices == nil || userID == "" {
		if s.collectionsBackend == nil {
			return nil, fmt.Errorf("collections not available in distributed mode")
		}
		return s.collectionsBackend, nil
	}
	return s.users.userServices.GetCollections(userID)
}

// --- ForUser Skills ---

// SkillsServiceForUser returns the skills service for a user.
func (s *AgentPoolService) SkillsServiceForUser(userID string) (*skills.Service, error) {
	if s.users.userServices == nil || userID == "" {
		if s.localAGI.skillsService == nil {
			return nil, fmt.Errorf("skills service not available")
		}
		return s.localAGI.skillsService, nil
	}
	return s.users.userServices.GetSkills(userID)
}

// SkillManagerForUser returns a SkillManager for a specific user.
// In distributed mode, returns a DistributedManager that syncs to PostgreSQL.
// In standalone mode, returns a FilesystemManager.
func (s *AgentPoolService) SkillManagerForUser(userID string) (skillsManager.Manager, error) {
	svc, err := s.SkillsServiceForUser(userID)
	if err != nil {
		return nil, err
	}
	fs := skillsManager.NewFilesystemManager(svc)

	// In distributed mode, wrap with PostgreSQL sync
	if s.distributed.skillStore != nil {
		return skillsManager.NewDistributedManager(fs, s.distributed.skillStore, userID), nil
	}
	return fs, nil
}

// --- ForUser Jobs ---

// JobServiceForUser returns the agent job service for a user.
func (s *AgentPoolService) JobServiceForUser(userID string) (*AgentJobService, error) {
	if s.users.userServices == nil || userID == "" {
		return nil, fmt.Errorf("no user services manager or empty user ID")
	}
	return s.users.userServices.GetJobs(userID)
}

// --- Actions ---

// ListAvailableActions returns the list of all available action type names.
// In distributed mode, returns an empty list (actions are configured as MCP tools per agent).
func (s *AgentPoolService) ListAvailableActions() []string {
	return s.configBackend.ListAvailableActions()
}

// GetActionDefinition creates an action instance by name with the given config and returns its definition.
func (s *AgentPoolService) GetActionDefinition(actionName string, actionConfig map[string]string) (any, error) {
	if actionConfig == nil {
		actionConfig = map[string]string{}
	}
	a, err := agiServices.Action(actionName, "", actionConfig, s.localAGI.pool, s.localAGI.actionsConfig)
	if err != nil {
		return nil, err
	}
	return a.Definition(), nil
}

// ExecuteAction creates an action instance and runs it with the given params.
func (s *AgentPoolService) ExecuteAction(ctx context.Context, actionName string, actionConfig map[string]string, params coreTypes.ActionParams) (coreTypes.ActionResult, error) {
	if actionConfig == nil {
		actionConfig = map[string]string{}
	}
	a, err := agiServices.Action(actionName, "", actionConfig, s.localAGI.pool, s.localAGI.actionsConfig)
	if err != nil {
		return coreTypes.ActionResult{}, err
	}
	return a.Run(ctx, s.localAGI.sharedState, params)
}

// loadSkillsForUser loads full skill info (name, description, content) for a user.
// Used by dispatchChat and the scheduler to enrich NATS events.
func (s *AgentPoolService) loadSkillsForUser(userID string) ([]agents.SkillInfo, error) {
	mgr, err := s.SkillManagerForUser(userID)
	if err != nil {
		return nil, err
	}
	allSkills, err := mgr.List()
	if err != nil {
		return nil, err
	}
	var skills []agents.SkillInfo
	for _, sk := range allSkills {
		desc := ""
		if sk.Metadata != nil && sk.Metadata.Description != "" {
			desc = sk.Metadata.Description
		}
		if desc == "" {
			d := sk.Content
			if len(d) > 200 {
				d = d[:200] + "..."
			}
			desc = d
		}
		skills = append(skills, agents.SkillInfo{
			Name:        sk.Name,
			Description: desc,
			Content:     sk.Content,
		})
	}
	return skills, nil
}

// buildSkillProvider returns a SkillContentProvider closure for the scheduler.
func (s *AgentPoolService) buildSkillProvider() agents.SkillContentProvider {
	return func(userID string) ([]agents.SkillInfo, error) {
		return s.loadSkillsForUser(userID)
	}
}
