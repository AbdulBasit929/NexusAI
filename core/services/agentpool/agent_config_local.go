package agentpool

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mudler/LocalAGI/core/agent"
	"github.com/mudler/LocalAGI/core/sse"
	"github.com/mudler/LocalAGI/core/state"
	agiConfig "github.com/mudler/LocalAGI/pkg/config"
	agiServices "github.com/mudler/LocalAGI/services"
	"github.com/mudler/LocalAI/core/services/agents"
)

// localAgentConfigBackend wraps the in-memory LocalAGI AgentPool for standalone mode.
type localAgentConfigBackend struct {
	svc *AgentPoolService // back-reference for shared fields (pool, configMeta, outputsDir, etc.)
}

func newLocalAgentConfigBackend(svc *AgentPoolService) *localAgentConfigBackend {
	return &localAgentConfigBackend{svc: svc}
}

func (b *localAgentConfigBackend) ListAgents(userID string) map[string]bool {
	statuses := map[string]bool{}
	agents := b.svc.localAGI.pool.List()
	prefix := ""
	if userID != "" {
		prefix = userID + ":"
	}
	for _, a := range agents {
		if userID != "" && !strings.HasPrefix(a, prefix) {
			continue
		}
		ag := b.svc.localAGI.pool.GetAgent(a)
		if ag == nil {
			continue
		}
		displayName := a
		if prefix != "" {
			displayName = strings.TrimPrefix(a, prefix)
		}
		statuses[displayName] = !ag.Paused()
	}
	return statuses
}

func (b *localAgentConfigBackend) GetConfig(userID, name string) *state.AgentConfig {
	cfg := b.svc.localAGI.pool.GetConfig(agents.AgentKey(userID, name))
	if cfg == nil {
		return nil
	}
	// Return a copy with the original name (strip userID: prefix)
	result := *cfg
	result.Name = name
	return &result
}

func (b *localAgentConfigBackend) SaveConfig(userID string, cfg *state.AgentConfig) error {
	key := agents.AgentKey(userID, cfg.Name)
	cfg.Name = key
	return b.svc.localAGI.pool.CreateAgent(key, cfg)
}

func (b *localAgentConfigBackend) UpdateConfig(userID, name string, cfg *state.AgentConfig) error {
	key := agents.AgentKey(userID, name)
	if old := b.svc.localAGI.pool.GetConfig(key); old == nil {
		return fmt.Errorf("%w: %s", ErrAgentNotFound, name)
	}
	cfg.Name = key
	return b.svc.localAGI.pool.RecreateAgent(key, cfg)
}

func (b *localAgentConfigBackend) DeleteConfig(userID, name string) error {
	return b.svc.localAGI.pool.Remove(agents.AgentKey(userID, name))
}

func (b *localAgentConfigBackend) ImportConfig(userID string, cfg *state.AgentConfig) error {
	key := agents.AgentKey(userID, cfg.Name)
	cfg.Name = key
	return b.svc.localAGI.pool.CreateAgent(key, cfg)
}

func (b *localAgentConfigBackend) ExportConfig(userID, name string) ([]byte, error) {
	cfg := b.svc.localAGI.pool.GetConfig(agents.AgentKey(userID, name))
	if cfg == nil {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, name)
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func (b *localAgentConfigBackend) SetStatus(userID, name, status string) error {
	ag := b.svc.localAGI.pool.GetAgent(agents.AgentKey(userID, name))
	if ag == nil {
		return fmt.Errorf("%w: %s", ErrAgentNotFound, name)
	}
	switch status {
	case "paused":
		ag.Pause()
	case "active":
		ag.Resume()
	default:
		return fmt.Errorf("unknown status: %s", status)
	}
	return nil
}

func (b *localAgentConfigBackend) GetAgent(userID, name string) *agent.Agent {
	return b.svc.localAGI.pool.GetAgent(agents.AgentKey(userID, name))
}

func (b *localAgentConfigBackend) GetSSEManager(userID, name string) sse.Manager {
	return b.svc.localAGI.pool.GetManager(agents.AgentKey(userID, name))
}

func (b *localAgentConfigBackend) GetStatus(userID, name string) *state.Status {
	return b.svc.localAGI.pool.GetStatusHistory(agents.AgentKey(userID, name))
}

func (b *localAgentConfigBackend) GetObservables(userID, name string) ([]json.RawMessage, error) {
	ag := b.svc.localAGI.pool.GetAgent(agents.AgentKey(userID, name))
	if ag == nil {
		return nil, fmt.Errorf("%w: %s", ErrAgentNotFound, name)
	}
	history := ag.Observer().History()
	result := make([]json.RawMessage, 0, len(history))
	for _, obs := range history {
		data, err := json.Marshal(obs)
		if err != nil {
			continue
		}
		result = append(result, data)
	}
	return result, nil
}

func (b *localAgentConfigBackend) ClearObservables(userID, name string) error {
	ag := b.svc.localAGI.pool.GetAgent(agents.AgentKey(userID, name))
	if ag == nil {
		return fmt.Errorf("%w: %s", ErrAgentNotFound, name)
	}
	ag.Observer().ClearHistory()
	return nil
}

func (b *localAgentConfigBackend) ListAllGrouped() map[string][]UserAgentInfo {
	result := map[string][]UserAgentInfo{}
	agents := b.svc.localAGI.pool.List()
	for _, a := range agents {
		ag := b.svc.localAGI.pool.GetAgent(a)
		if ag == nil {
			continue
		}
		userID := ""
		name := a
		if u, n, ok := strings.Cut(a, ":"); ok {
			userID = u
			name = n
		}
		result[userID] = append(result[userID], UserAgentInfo{
			Name:   name,
			Active: !ag.Paused(),
		})
	}
	return result
}

func (b *localAgentConfigBackend) GetConfigMeta() AgentConfigMetaResult {
	meta := b.svc.localAGI.configMeta
	return AgentConfigMetaResult{
		Fields:     appendNativeLocalAgentFields(meta.Fields),
		Actions:    meta.Actions,
		Connectors: meta.Connectors,
		Filters:    meta.Filters,
		OutputsDir: b.svc.outputsDir,
	}
}

func appendNativeLocalAgentFields(fields []agiConfig.Field) []agiConfig.Field {
	out := append([]agiConfig.Field{}, fields...)
	existing := map[string]struct{}{}
	for _, field := range out {
		existing[field.Name] = struct{}{}
	}
	add := func(field agiConfig.Field) {
		if _, ok := existing[field.Name]; ok {
			return
		}
		out = append(out, field)
		existing[field.Name] = struct{}{}
	}

	add(agiConfig.Field{
		Name:         "kb_mode",
		Label:        "Knowledge Base Mode",
		Type:         agiConfig.FieldTypeSelect,
		DefaultValue: agents.KBModeAutoSearch,
		HelpText:     "How the Knowledge Base is used: automatic search, callable tools, or both.",
		Options: []agiConfig.FieldOption{
			{Value: agents.KBModeAutoSearch, Label: "Auto Search"},
			{Value: agents.KBModeTools, Label: "As Tools"},
			{Value: agents.KBModeBoth, Label: "Both"},
		},
		Tags: agiConfig.Tags{Section: "MemorySettings"},
	})
	add(agiConfig.Field{
		Name:         "enable_forensic_records",
		Label:        "Enable Forensic Records",
		Type:         agiConfig.FieldTypeCheckbox,
		DefaultValue: false,
		HelpText:     "Expose deterministic KB + structured records intelligence tools to this agent.",
		Tags:         agiConfig.Tags{Section: "MemorySettings"},
	})
	add(agiConfig.Field{
		Name:        "forensic_records_api_url",
		Label:       "Forensic Records API URL",
		Type:        agiConfig.FieldTypeText,
		Placeholder: "http://localhost:8091",
		HelpText:    "Sidecar URL used by forensic_hybrid_query, forensic_query_templates, and forensic_generate_report.",
		Tags:        agiConfig.Tags{Section: "MemorySettings"},
	})
	add(agiConfig.Field{
		Name:     "forensic_records_api_key",
		Label:    "Forensic Records API Key",
		Type:     agiConfig.FieldTypeText,
		HelpText: "Optional bearer token if the forensic sidecar is protected.",
		Tags:     agiConfig.Tags{Section: "MemorySettings"},
	})
	add(agiConfig.Field{
		Name:         "forensic_tenant_id",
		Label:        "Forensic Tenant ID",
		Type:         agiConfig.FieldTypeText,
		DefaultValue: "default",
		Placeholder:  "default",
		Tags:         agiConfig.Tags{Section: "MemorySettings"},
	})
	add(agiConfig.Field{
		Name:        "forensic_collection_id",
		Label:       "Forensic Collection ID",
		Type:        agiConfig.FieldTypeText,
		Placeholder: "records-demo",
		HelpText:    "Case/KB collection used when the tool call does not pass a collection_id.",
		Tags:        agiConfig.Tags{Section: "MemorySettings"},
	})
	add(agiConfig.Field{
		Name:         "enable_reasoning_for_instruct",
		Label:        "Enable Reasoning for Instruct Models",
		Type:         agiConfig.FieldTypeCheckbox,
		DefaultValue: false,
		HelpText:     "Force structured reasoning before tool selection for instruct-tuned models.",
		Tags:         agiConfig.Tags{Section: "AdvancedSettings"},
	})
	add(agiConfig.Field{
		Name:         "skills_mode",
		Label:        "Skills Injection Mode",
		Type:         agiConfig.FieldTypeSelect,
		DefaultValue: agents.SkillsModePrompt,
		HelpText:     "How selected skills are made available to the agent.",
		Options: []agiConfig.FieldOption{
			{Value: agents.SkillsModePrompt, Label: "Inject as System Prompt"},
			{Value: agents.SkillsModeTools, Label: "Expose as Tools"},
			{Value: agents.SkillsModeBoth, Label: "Both"},
		},
		Tags: agiConfig.Tags{Section: "AdvancedSettings"},
	})
	add(agiConfig.Field{
		Name:         "max_iterations",
		Label:        "Max Tool Iterations",
		Type:         agiConfig.FieldTypeNumber,
		DefaultValue: 4,
		Min:          1,
		Step:         1,
		HelpText:     "Maximum agent tool loop iterations per response.",
		Tags:         agiConfig.Tags{Section: "AdvancedSettings"},
	})
	return out
}

func (b *localAgentConfigBackend) ListAvailableActions() []string {
	return agiServices.AvailableActions
}

func (b *localAgentConfigBackend) Chat(userID, name, message string) (string, error) {
	return b.svc.Chat(agents.AgentKey(userID, name), message)
}

func (b *localAgentConfigBackend) Stop() {
	if b.svc.localAGI.pool != nil {
		b.svc.localAGI.pool.StopAll()
	}
}
