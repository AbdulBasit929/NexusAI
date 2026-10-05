package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestSwitchStateSplitsEveryRegisteredSwitch(t *testing.T) {
	env := map[string]string{
		"FORENSIC_VERIFIED_ONLY":           "true",
		"FORENSIC_LADDER_ROUTING":          " TRUE ",
		"FORENSIC_IDENTIFIER_BINDING":      "false",
		"FORENSIC_FRAME_FAMILY_PRECEDENCE": "",
		"FORENSIC_IR_SHADOW":               "yes",
	}
	lookup := func(name string) (string, bool) { value, ok := env[name]; return value, ok }
	on, off, other, unset := switchState(lookup)
	has := func(list []string, name string) bool {
		for _, item := range list {
			if item == name {
				return true
			}
		}
		return false
	}
	if !has(on, "FORENSIC_VERIFIED_ONLY") || !has(on, "FORENSIC_LADDER_ROUTING") {
		t.Fatalf("true values must be on: %v", on) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if !has(off, "FORENSIC_IDENTIFIER_BINDING") || !has(off, "FORENSIC_FRAME_FAMILY_PRECEDENCE") {
		t.Fatalf("false and empty values must be off: %v", off) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if !has(other, "FORENSIC_IR_SHADOW") {
		t.Fatalf("a non-boolean value must be reported, not silently read as off: %v", other) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if !has(unset, "FORENSIC_ROW_COUNT_HEADLINE_GUARD") {
		t.Fatalf("an absent switch must be reported as unset: %v", unset) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if total := len(on) + len(off) + len(other) + len(unset); total != len(behaviourSwitches) {
		t.Fatalf("every registered switch must land in exactly one list: %d of %d", total, len(behaviourSwitches)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

// The registry must name only switches compose actually declares, or the
// startup line would report a switch the deployment can never set.
func TestBehaviourSwitchRegistryMatchesCompose(t *testing.T) {
	raw, err := os.ReadFile("../../docker-compose.forensic-records.yaml")
	if err != nil {
		t.Skipf("compose file not readable from the test directory: %v", err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	compose := string(raw)
	for _, name := range behaviourSwitches {
		if !strings.Contains(compose, name+": ${"+name+":-") {
			t.Errorf("%s is in the startup registry but not declared in docker-compose", name) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}

	// The reverse direction, which the first version missed: FORENSIC_EVIDENCE_SEARCH_FIRST
	// shipped 2026-09-28 declared in compose and read by the code, but absent from the
	// registry, so the startup line never reported it. Every switch this package reads
	// that compose declares as a true/false default must be in the registry.
	registered := map[string]bool{}
	for _, name := range behaviourSwitches {
		registered[name] = true
	}
	declared := regexp.MustCompile(`(FORENSIC_[A-Z0-9_]+): \$\{FORENSIC_[A-Z0-9_]+:-(?:true|false)\}`)
	sources, _ := filepath.Glob("*.go")
	code := strings.Builder{}
	for _, path := range sources {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		body, _ := os.ReadFile(path)
		code.Write(body)
	}
	for _, match := range declared.FindAllStringSubmatch(compose, -1) {
		name := match[1]
		if strings.Contains(code.String(), `"`+name+`"`) && !registered[name] {
			t.Errorf("%s is read by this service and declared in compose, but missing from behaviourSwitches", name) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}
