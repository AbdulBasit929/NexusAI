package main

import (
	"fmt"
	"math"
	"testing"
)

type semanticRecallCounts struct {
	Total int
	Top1  int
	Top3  int
	Top5  int
}

func (c semanticRecallCounts) percent(value int) float64 {
	if c.Total == 0 {
		return 0
	}
	return 100 * float64(value) / float64(c.Total)
}

func semanticOperationRank(candidates []SemanticOperationCandidateV1, operationID string) int {
	for index, candidate := range candidates {
		if candidate.OperationID == operationID {
			return index + 1
		}
	}
	return 0
}

func semanticRecallRequest(operationID, question string) (hybridQueryRequest, bool) {
	template, ok := queryTemplateByName(queryTemplateNameByOperationID(operationID))
	if !ok || template.ExposureStatus == "engineering_only" {
		return hybridQueryRequest{}, false
	}
	req := hybridQueryRequest{Query: question, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	if template.ScopeMode == "evidence_required" {
		family := ""
		if len(template.RecordTypes) > 0 && template.RecordTypes[0] != "all" {
			family = template.RecordTypes[0]
		}
		req.EvidenceID = "10000000-0000-4000-8000-000000000001"
		req.QueryScope = queryEvidenceScope{Kind: string(EvidenceScopeSelected), EvidenceID: req.EvidenceID, SourceFamily: family}
	}
	if semanticOperationRank(semanticOperationCandidates(req), operationID) == 0 {
		return hybridQueryRequest{}, false
	}
	return req, true
}

func TestRetrievalFirstSemanticRecall(t *testing.T) {
	development := []struct{ question, operation string }{
		{"Rank the counterparties of 923146208975 by how often they exchanged calls.", "cdr.frequent_contacts"},
		{"Which capture points recorded the greatest number of vehicle sightings?", "anpr.camera_activity"},
		{"Find passages discussing damage to the loading dock in the case documents.", "document.search"},
		{"Look through recorded speech for mentions of the missed supply delivery.", "audio.transcript_search"},
		{"Display the imported spreadsheet entries for inspection.", "generic.filter_records"},
	}
	developmentTop5 := 0
	for _, item := range development {
		req := hybridQueryRequest{Query: item.question, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		ranked, _ := rankSemanticOperationsWithScores(item.question, semanticOperationCandidates(req))
		if rank := semanticOperationRank(ranked, item.operation); rank > 0 && rank <= 5 {
			developmentTop5++
		} else {
			t.Logf("development retrieval miss operation=%s candidates=%v", item.operation, semanticOperationIDs(ranked))
			for _, candidate := range semanticOperationCandidates(req) {
				if candidate.OperationID == item.operation {
					t.Logf("expected descriptor=%s", semanticOperationDocument(candidate))
				}
			}
		}
	}
	if developmentTop5 != len(development) {
		t.Fatalf("DEVELOPMENT_EXPECTED_OPS_TOP5=%d/%d", developmentTop5, len(development))
	}

	ledger, err := loadQueryVariantLedger()
	if err != nil {
		t.Fatal(err)
	}
	counts := semanticRecallCounts{}
	misses := []string{}
	for _, entry := range ledger.Entries {
		// The retrieval-first resolver is the English analytical fallback. Contextual
		// follow-ups are rebound before this resolver, clarification rows have no
		// resolvable operation, and engineering-only operations must stay excluded.
		if entry.Language != "english" || entry.ClarificationRequired || len(entry.FollowUpRequirements) > 0 {
			continue
		}
		req, eligible := semanticRecallRequest(entry.CanonicalOperation, entry.Text)
		if !eligible {
			continue
		}
		ranked, _ := rankSemanticOperationsWithScores(entry.Text, semanticOperationCandidates(req))
		rank := semanticOperationRank(ranked, entry.CanonicalOperation)
		counts.Total++
		if rank == 1 {
			counts.Top1++
		}
		if rank > 0 && rank <= 3 {
			counts.Top3++
		}
		if rank > 0 && rank <= 5 {
			counts.Top5++
		} else if len(misses) < 20 {
			misses = append(misses, fmt.Sprintf("%s:%s=>%v", entry.VariantID, entry.CanonicalOperation, semanticOperationIDs(ranked)))
		}
	}
	t.Logf("RETRIEVAL_TOP1=%.2f%% (%d/%d)", counts.percent(counts.Top1), counts.Top1, counts.Total)
	t.Logf("RETRIEVAL_TOP3=%.2f%% (%d/%d)", counts.percent(counts.Top3), counts.Top3, counts.Total)
	t.Logf("RETRIEVAL_TOP5=%.2f%% (%d/%d)", counts.percent(counts.Top5), counts.Top5, counts.Total)
	if counts.Total == 0 || counts.Top5*100 < int(math.Ceil(95*float64(counts.Total))) {
		t.Fatalf("variant ledger top-5 recall below 95%%: %d/%d; misses=%v", counts.Top5, counts.Total, misses)
	}
}

func semanticOperationIDs(candidates []SemanticOperationCandidateV1) []string {
	ids := make([]string, len(candidates))
	for index := range candidates {
		ids[index] = candidates[index].OperationID
	}
	return ids
}

func TestRetrievalFirstScopeAndStability(t *testing.T) {
	workspace := hybridQueryRequest{Query: "Review registered evidence", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
	eligible := semanticOperationCandidates(workspace)
	if len(eligible) != 66 {
		t.Fatalf("workspace eligible operations=%d want=66", len(eligible))
	}
	for _, candidate := range eligible {
		if candidate.ExposureStatus == "engineering_only" {
			t.Fatalf("engineering-only operation retrieved: %s", candidate.OperationID)
		}
	}
	first, firstScores := rankSemanticOperationsWithScores(workspace.Query, eligible)
	second, secondScores := rankSemanticOperationsWithScores(workspace.Query, eligible)
	if fmt.Sprint(semanticOperationIDs(first)) != fmt.Sprint(semanticOperationIDs(second)) || fmt.Sprint(firstScores) != fmt.Sprint(secondScores) {
		t.Fatal("semantic retrieval is not deterministic")
	}
	selected := hybridQueryRequest{EvidenceID: "10000000-0000-4000-8000-000000000001", QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeSelected), SourceFamily: "document"}}
	selectedCandidates := semanticOperationCandidates(selected)
	if len(selectedCandidates) == 0 {
		t.Fatal("selected document evidence lost all operations")
	}
	for _, candidate := range selectedCandidates {
		if candidate.FamilyID == "communications_cdr" || candidate.FamilyID == "audio_intelligence" || candidate.FamilyID == "anpr_vehicles" {
			t.Fatalf("selected document evidence leaked unauthorized family operation %s", candidate.OperationID)
		}
	}
}
