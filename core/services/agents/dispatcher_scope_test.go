package agents

import (
	"encoding/json"
	"testing"
)

func TestAgentChatEventPreservesRequestScopedForensicContext(t *testing.T) {
	event := AgentChatEvent{
		AgentName:                   "Forensic_Records_Analyst",
		ForensicConversationContext: &ForensicConversationContext{Template: "image_metadata", Target: "subject"},
		ForensicQueryScope: &ForensicQueryScope{
			Kind: "selected_evidence", EvidenceID: "11111111-1111-1111-8111-111111111111",
			EvidenceVersionID: "22222222-2222-2222-8222-222222222222", SourceFamily: "image",
			AvailableResultFamilies: []string{"forensics.image-embedding/v1"},
		},
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	var decoded AgentChatEvent
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ForensicQueryScope == nil || decoded.ForensicQueryScope.EvidenceID != event.ForensicQueryScope.EvidenceID || decoded.ForensicQueryScope.AvailableResultFamilies[0] != "forensics.image-embedding/v1" {
		t.Fatalf("query scope lost in distributed transport: %#v", decoded.ForensicQueryScope)
	}
	if decoded.ForensicConversationContext == nil || decoded.ForensicConversationContext.Template != "image_metadata" {
		t.Fatalf("conversation context lost in distributed transport: %#v", decoded.ForensicConversationContext)
	}
}
