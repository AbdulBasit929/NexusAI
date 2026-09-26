package main

import "testing"

// H2: a verified plan asserting FALSE ABSENCE.
//
// "How many VoLTE calls are there?" produced the correct plan
// (`cdr.call_type EQ "VOLTE"`) and was then intersected with
// `(primary_target='VOLTE' OR secondary_target='VOLTE')`, matching 0 of 739
// rows. The analyst was told the calls do not exist.

func TestCallTypeValueIsNotASubjectIdentifier(t *testing.T) {
	for _, value := range []string{"VOLTE", "GPRS", "SMS"} {
		if classifyTargetType(value) != "call_type" {
			t.Fatalf("%q must classify as call_type; the fix keys on it", value)
		}
		if targetIsSubjectIdentifier(value) {
			t.Errorf("%q is a value of cdr.call_type, never the subject of a record; "+
				"applying it to primary_target matches nothing and asserts false absence", value)
		}
	}
}

// The control, and it is the half that matters: every REAL subject must still
// constrain. Widening the scope is how confident wrong answers are made.
func TestRealSubjectsStillConstrain(t *testing.T) {
	for _, target := range []string{
		"923001110001",      // phone
		"03001234567",       // phone
		"192.168.1.10",      // ip
		"someone@example.com", // email
		"PK-LHR-SYN-001",    // identifier
		"ABC-123",           // plate-shaped identifier
		"AcmeCorp",          // entity fallback
	} {
		if !targetIsSubjectIdentifier(target) {
			t.Errorf("%q is a subject; dropping its constraint would widen the answer", target)
		}
	}
}

// An unrecognised target must keep today's behaviour. `classifyTargetType`
// falls back to "entity", and treating the unknown as a non-subject would
// silently widen results -- the failure mode this project exists to prevent.
func TestUnknownTargetsKeepTodaysBehaviour(t *testing.T) {
	for _, target := range []string{"zzz", "?", "Ünïcôdé", "12"} {
		if !targetIsSubjectIdentifier(target) {
			t.Errorf("%q is unrecognised; it must keep constraining, not widen", target)
		}
	}
}
