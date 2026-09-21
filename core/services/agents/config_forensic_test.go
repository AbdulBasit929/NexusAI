package agents

import "testing"

func TestAgentConfigKnowledgeCollectionName(t *testing.T) {
	tests := []struct {
		name string
		cfg  *AgentConfig
		want string
	}{
		{name: "nil config", cfg: nil, want: ""},
		{
			name: "forensic specialist uses governed case collection",
			cfg: &AgentConfig{
				Name:                  "Subscriber_Identity_Analyst",
				EnableForensicRecords: true,
				ForensicCollectionID:  " records-demo-verified ",
				EnableKnowledgeBase:   true,
			},
			want: "records-demo-verified",
		},
		{
			name: "generic knowledge agent retains agent collection",
			cfg:  &AgentConfig{Name: "Research_Assistant", EnableKnowledgeBase: true},
			want: "Research_Assistant",
		},
		{
			name: "forensic fallback remains agent name until normalization",
			cfg:  &AgentConfig{Name: "Forensic_Records_Analyst", EnableForensicRecords: true},
			want: "Forensic_Records_Analyst",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.KnowledgeCollectionName(); got != tt.want {
				t.Fatalf("KnowledgeCollectionName() = %q, want %q", got, tt.want)
			}
		})
	}
}
