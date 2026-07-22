package agentpool

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mudler/LocalAGI/core/state"
	"github.com/mudler/LocalAI/core/services/agents"
)

const localNativeAgentConfigDir = "native-agent-configs"

type staticSkillProvider struct {
	skills []agents.SkillInfo
}

func (p staticSkillProvider) ListSkills() ([]agents.SkillInfo, error) {
	return p.skills, nil
}

func nativeConfigFromState(cfg *state.AgentConfig) (*agents.AgentConfig, error) {
	if cfg == nil {
		return nil, nil
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var native agents.AgentConfig
	if err := json.Unmarshal(data, &native); err != nil {
		return nil, err
	}
	normalizeNativeAgentConfig(&native)
	return &native, nil
}

func stateConfigFromNative(cfg *agents.AgentConfig) (*state.AgentConfig, error) {
	if cfg == nil {
		return nil, nil
	}
	normalizeNativeAgentConfig(cfg)
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var local state.AgentConfig
	if err := json.Unmarshal(data, &local); err != nil {
		return nil, err
	}
	for i, action := range cfg.Actions {
		if i >= len(local.Actions) {
			break
		}
		if local.Actions[i].Name == "" {
			local.Actions[i].Name = action.Name
		}
		if local.Actions[i].Name == "" {
			local.Actions[i].Name = action.Type
		}
	}
	return &local, nil
}

func normalizeNativeAgentConfig(cfg *agents.AgentConfig) {
	if cfg == nil {
		return
	}
	if cfg.KnowledgeBaseResults <= 0 {
		cfg.KnowledgeBaseResults = 5
	}
	if cfg.KBMode == "" {
		switch {
		case cfg.KBAutoSearch && cfg.KBAsTools:
			cfg.KBMode = agents.KBModeBoth
		case cfg.KBAsTools:
			cfg.KBMode = agents.KBModeTools
		default:
			cfg.KBMode = agents.KBModeAutoSearch
		}
	}
	for i := range cfg.Actions {
		if cfg.Actions[i].Type == "" {
			cfg.Actions[i].Type = cfg.Actions[i].Name
		}
		if cfg.Actions[i].Name == "" {
			cfg.Actions[i].Name = cfg.Actions[i].Type
		}
	}
	if cfg.EnableForensicRecords {
		cfg.EnableKnowledgeBase = true
		if cfg.ForensicTenantID == "" {
			cfg.ForensicTenantID = "default"
		}
		if cfg.ForensicCollectionID == "" {
			cfg.ForensicCollectionID = cfg.Name
		}
		if cfg.MaxIterations < 4 {
			cfg.MaxIterations = 4
		}
	}
}

func (s *AgentPoolService) nativeAgentConfigPath(userID, name string) string {
	key := agents.AgentKey(userID, name)
	encoded := base64.RawURLEncoding.EncodeToString([]byte(key))
	return filepath.Join(s.stateDir, localNativeAgentConfigDir, encoded+".json")
}

func (s *AgentPoolService) saveNativeAgentConfig(userID string, cfg *agents.AgentConfig) error {
	if cfg == nil || cfg.Name == "" {
		return nil
	}
	normalizeNativeAgentConfig(cfg)
	dir := filepath.Join(s.stateDir, localNativeAgentConfigDir)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create native agent config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal native agent config: %w", err)
	}
	if err := os.WriteFile(s.nativeAgentConfigPath(userID, cfg.Name), data, 0600); err != nil {
		return fmt.Errorf("write native agent config: %w", err)
	}
	return nil
}

func (s *AgentPoolService) loadNativeAgentConfig(userID, name string) (*agents.AgentConfig, error) {
	data, err := os.ReadFile(s.nativeAgentConfigPath(userID, name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cfg agents.AgentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Name == "" {
		cfg.Name = name
	}
	normalizeNativeAgentConfig(&cfg)
	return &cfg, nil
}

func (s *AgentPoolService) removeNativeAgentConfig(userID, name string) error {
	err := os.Remove(s.nativeAgentConfigPath(userID, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *AgentPoolService) nativeAgentConfigForUser(userID, name string) (*agents.AgentConfig, error) {
	if cfg, err := s.loadNativeAgentConfig(userID, name); err != nil || cfg != nil {
		return cfg, err
	}
	if userID != "" {
		if cfg, err := s.loadNativeAgentConfig("", name); err != nil || cfg != nil {
			return cfg, err
		}
	}
	if s.distributed.agentStore != nil {
		rec, err := s.distributed.agentStore.GetConfig(userID, name)
		if err == nil && rec != nil {
			var cfg agents.AgentConfig
			if err := json.Unmarshal([]byte(rec.ConfigJSON), &cfg); err != nil {
				return nil, err
			}
			if cfg.Name == "" {
				cfg.Name = name
			}
			normalizeNativeAgentConfig(&cfg)
			return &cfg, nil
		}
		// Local desktop/demo deployments may have a distributed store for
		// events while the full native LocalAI-only agent config is persisted
		// on disk. Fall through so forensic_records and other native-only
		// fields are not silently lost.
	}
	return nil, nil
}
