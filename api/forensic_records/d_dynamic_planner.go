package main

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/mudler/LocalAI/pkg/forensictext"
)

var unsafePlannerRequest = regexp.MustCompile(`(?i)(?:\bselect\s+.+\bfrom\b|\binsert\s+into\b|\bupdate\s+\w+\s+set\b|\bdelete\s+from\b|\bdrop\s+(?:table|database)\b|\brm\s+-|\bcurl\s+https?://|\bhttps?://\S+/(?:tool|execute|shell))`)

// applyExplicitQueryScopeIntent changes scope only inside the already-authorized
// case. Terms inside quoted evidence literals cannot influence it.
func applyExplicitQueryScopeIntent(req hybridQueryRequest) (hybridQueryRequest, string) {
	switch forensictext.ExplicitScope(req.Query) {
	case "ambiguous":
		return req, "AMBIGUOUS_SCOPE"
	case "authorized_workspace":
		req.QueryScope = queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}
		req.EvidenceID, req.EvidenceVersionID = "", ""
		return req, "authorized_workspace"
	case "selected_evidence":
		if strings.TrimSpace(req.EvidenceID) == "" && strings.TrimSpace(req.QueryScope.EvidenceID) == "" {
			return req, "SELECTED_EVIDENCE_REQUIRED"
		}
		return req, "selected_evidence"
	}
	return req, ""
}

func assessDynamicQuery(query, template string) queryCapabilityAssessment {
	assessment := assessQueryCapability(query, template)
	normalized := normalizeAnalystSemantics(query)
	if unsafePlannerRequest.MatchString(query) || containsAny(normalized, []string{"ignore restrictions", "run shell", "execute shell"}) {
		assessment.Status = "unavailable"
		assessment.Explanation = "The planner accepts only registered forensic capabilities and typed parameters; arbitrary SQL, shell, URL, and tool execution are rejected."
		assessment.MissingCapabilities = []string{"ARBITRARY_EXECUTION_REJECTED"}
	}
	if regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(evtx|ofx|mt940|heic|srt|vtt|svg)(?:$|[^a-z0-9])`).MatchString(query) {
		assessment.Status = "unavailable"
		assessment.Explanation = "The requested input format has no qualified analytical decoder for this capability."
		assessment.MissingCapabilities = []string{"UNQUALIFIED_INPUT_FORMAT"}
	}
	if containsAny(normalized, []string{"another case", "different case", "other tenant", "switch tenant", "switch collection", "invented tenant"}) {
		assessment.Status = "unavailable"
		assessment.Explanation = "Query text cannot change the authorized tenant, case, or collection."
		assessment.MissingCapabilities = []string{"SCOPE_CONFLICT"}
	}
	for _, match := range regexp.MustCompile(`\b\d{4,}\b`).FindAllString(query, -1) {
		value, _ := strconv.Atoi(match)
		if value > maxHybridLimit && containsAny(normalized, []string{"top " + match, "closest " + match}) {
			assessment.Status = "unavailable"
			assessment.Explanation = "The requested result count exceeds the execution budget."
			assessment.MissingCapabilities = []string{"TOP_K_BUDGET_EXCEEDED"}
		}
	}
	return assessment
}
