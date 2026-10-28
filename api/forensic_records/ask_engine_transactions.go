package main

import (
	"fmt"
	"regexp"
	"strings"
)

// ASK ENGINE V2: TRANSACTIONS (docs/data/transactions-family-20261028.md, piece g). What is said when a question about an account finds no row, and the correction put in front of a question that
// asks for an offence. A transaction is a line of a statement or a wallet list; a pattern in such lines (money in and out within minutes, amounts just under a limit, a counterparty in common) is a
// pattern, never a finding, and the engine says nothing of laundering, structuring or fraud as a conclusion.

// askTxnPremiseCue is a question that names an offence, which no pattern of rows states.
var askTxnPremiseCue = regexp.MustCompile(`\b(?:launder\w*|structur\w*|smurf\w*|fraud\w*|embezzl\w*|illegal\w*|criminal\w*|guilty|suspicious)\b`)

var askTransactionViews = []string{"v_bank_transactions", "v_account_counterparties", "v_account_activity", "v_account_flows", "v_account_limit_bands"}

// askReadsTransactionView is whether the query reads one of the transaction views.
func askReadsTransactionView(v *askValidated) bool {
	for _, view := range askTransactionViews {
		if askReadsView(v, view) {
			return true
		}
	}
	return false
}

// askTransactionAbsence says what was searched when a transaction view returns no row: the account or number of the question, the parameter values used, or the pattern asked for.
func askTransactionAbsence(v *askValidated, f askFrame) string {
	of := ""
	if len(f.Accounts) == 1 {
		of = " of " + f.Accounts[0]
	}
	switch {
	case askReadsView(v, "v_account_flows"):
		return fmt.Sprintf("no credit%s was followed by a debit of at least %d percent of it within %d minutes%s", of, askParamValue(f, "flow_share_percent"), askParamValue(f, "flow_window_minutes"), askPeriodPhrase(f))
	case askReadsView(v, "v_account_limit_bands"):
		return fmt.Sprintf("no debit%s is at or above %d percent of the limit %d and below it%s", of, askParamValue(f, "limit_share_percent"), askParamValue(f, "limit_amount"), askPeriodPhrase(f))
	case askReadsView(v, "v_account_counterparties"):
		switch {
		case len(f.Accounts) >= 2:
			return "the accounts " + strings.Join(f.Accounts, " and ") + " have no counterparty in common in the files of this case"
		case len(f.Accounts) == 1:
			return "no counterparty of " + f.Accounts[0] + " was found in the files of this case"
		}
		return "no counterparty matching your question was found in the files of this case"
	}
	switch {
	case len(f.Accounts) == 1:
		return "no transaction of " + f.Accounts[0] + askPeriodPhrase(f) + " was found in the files of this case"
	case len(f.Accounts) > 1:
		return "no transaction of " + strings.Join(f.Accounts, ", ") + askPeriodPhrase(f) + " was found in the files of this case"
	case len(f.Numbers) == 1:
		return "no transaction of the wallet or number " + f.Numbers[0] + askPeriodPhrase(f) + " was found in the files of this case"
	}
	return "no transaction matching your question was found in the files of this case"
}

// askTransactionLead is the correction put in front of a question that names an offence or states a conclusion the rows cannot support. "" when it names none.
func askTransactionLead(f askFrame, stated bool) string {
	if stated || askTxnPremiseCue.MatchString(strings.ToLower(f.Question)) {
		return "A pattern in the rows is not a finding of laundering, structuring or fraud: the records do not say why money moved."
	}
	return ""
}
