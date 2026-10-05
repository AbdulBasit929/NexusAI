package main

import "testing"

func TestRegisteredSelectionSuperseded(t *testing.T) {
	selected := &SemanticOperationCandidateV1{OperationID: "cdr.tower_activity", RequiredParameters: []string{"target"}}
	plan := &SourceNativePlanV1{ContractVersion: "forensics.source-native-plan/v1", GroupFields: []string{"cdr.cell_site_id"}}
	arbitrated := hybridQueryRequest{
		Query:                "Which cell site handled the most calls?",
		SourceNative:         plan,
		SemanticPlannerAudit: &SemanticPlannerAuditV1{Selected: selected, IRArbitrated: true},
	}
	notArbitrated := hybridQueryRequest{
		Query:                arbitrated.Query,
		SourceNative:         plan,
		SemanticPlannerAudit: &SemanticPlannerAuditV1{Selected: selected},
	}
	noPlan := hybridQueryRequest{
		Query:                arbitrated.Query,
		SemanticPlannerAudit: &SemanticPlannerAuditV1{Selected: selected, IRArbitrated: true},
	}
	noAudit := hybridQueryRequest{Query: arbitrated.Query, SourceNative: plan}

	t.Run("switch off leaves every request to the readiness check", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(arbitrationSupersedesSelectionEnv, "")
		for name, req := range map[string]hybridQueryRequest{"arbitrated": arbitrated, "not_arbitrated": notArbitrated, "no_plan": noPlan, "no_audit": noAudit} {
			if registeredSelectionSuperseded(req) {
				t.Fatalf("%s: superseded with the switch off", name) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	})

	t.Run("switch on supersedes only the arbitrated typed plan", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		t.Setenv(arbitrationSupersedesSelectionEnv, "true")
		if !registeredSelectionSuperseded(arbitrated) {
			t.Fatal("an arbitrated typed plan must supersede the registered selection") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		// A registered operation that was NOT replaced still executes, so its
		// missing target is a real obligation and must still clarify.
		if registeredSelectionSuperseded(notArbitrated) {
			t.Fatal("a request that was not arbitrated must keep the readiness check") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if registeredSelectionSuperseded(noPlan) {
			t.Fatal("arbitration without an executing typed plan must keep the readiness check") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		if registeredSelectionSuperseded(noAudit) {
			t.Fatal("a request with no planner audit must keep the readiness check") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})

	t.Run("the superseded selection still fails its own readiness check", func(t *testing.T) { //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		// Pins the defect this switch exists for: were the recompute run, it
		// would clarify on a target the executing plan does not use.
		state, missing := semanticBindingReadiness(arbitrated, *selected)
		if state != "USER_FACT_REQUIRED" || len(missing) != 1 || missing[0] != "target" {
			t.Fatalf("readiness = %s %v, want USER_FACT_REQUIRED [target]", state, missing) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	})
}
