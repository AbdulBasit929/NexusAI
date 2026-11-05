package main

import (
	"regexp"
	"strings"
)

// ASK ENGINE V2: RANKINGS (the verified bank run of 2026-11-02, VB-BANK-004).
//
// "What is the largest single debit of the account X?" asks for a VALUE, and `SELECT max(amount) ...` is the right query for it (it is the text of the verified example
// txn-largest-debit). The ORDER obligation demanded an ORDER BY for every question with a ranking word ("largest"), refused the example's own query three times, and the third
// model call ran into the budget of the question (audit rows of 2026-11-02 11:06 and 14:12 UTC: three attempts, 299,984 ms of model time, repairs OBLIGATION:ORDER twice).

// askAsksEntityRE is a question that asks WHICH one (or who): its answer is the label of the winner, so a ranking is needed and MAX alone would drop the label.
var askAsksEntityRE = regexp.MustCompile(`^\s*(?:which|who|whom|whose|list|name|show|give)\b`)

// askExtremeValue is whether the question asks for the extreme VALUE of a column ("what is the largest single debit", "the longest call") and the query takes it with MAX or MIN: the
// extreme of a column needs no ordering. sql is the validated query in lower case.
func askExtremeValue(f askFrame, sql string) bool {
	return !askAsksEntityRE.MatchString(strings.ToLower(f.Question)) && askSQLMentions(sql, "max(", "min(")
}
