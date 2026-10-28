package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Transactions (docs/data/transactions-family-20261028.md, piece g): the pass-through window and share and the limit are runtime parameters read from the question by code (and tested at several
// values), an account is an identifier of the question, a question about minutes must not read a row that states only a day, a pattern is never called an offence, and the coverage guard declines the
// views for a case with no transactions.
var _ = Describe("Ask engine transactions", func() {
	const account = "PK00SYNT0000000000001001"

	DescribeTable("reads the runtime parameters of the transaction views from the question",
		func(question, name string, want ...int) {
			read := askReadParams(askExtractFrame(question).Question)
			got := read[name]
			if len(want) == 0 {
				Expect(got.Found).To(BeFalse(), "%s", question)
				return
			}
			Expect(got.Values).To(Equal(want), "%s", question)
			Expect(got.Ambiguous).To(BeFalse(), "%s", question)
		},
		Entry("a window of 15 minutes", "which accounts received money that went out again within 15 minutes?", "flow_window_minutes", 15),
		Entry("a window of one hour is not read as a number of minutes here", "which accounts received money that went out again within an hour?", "flow_window_minutes"),
		Entry("a share of 95 percent of it", "which accounts passed on at least 95% of what came in within 5 minutes?", "flow_share_percent", 95),
		Entry("a share of 80 percent of the credit", "debits of 80 percent of the credit within 20 minutes", "flow_share_percent", 80),
		Entry("a limit with a thousands comma", "which accounts made debits just under a limit of 500,000?", "limit_amount", 500000),
		Entry("a limit written under", "debits under 400000", "limit_amount", 400000),
		Entry("a limit in rupees", "debits below the limit of rs. 250,000", "limit_amount", 250000),
		Entry("no limit named", "which accounts made many debits?", "limit_amount"),
		Entry("a band that starts at 80 percent of the limit", "debits of 80 percent of the limit of 100000", "limit_share_percent", 80),
		Entry("within 5 percent of the limit is a share of 95", "debits within 5% of the limit of 100000", "limit_share_percent", 95),
		Entry("no share named", "debits under the limit of 100000", "limit_share_percent"))

	It("asks for the limit when the question names none, uses the defaults of the rest, and states what it used", func() {
		views := []string{"v_account_limit_bands"}
		_, issue := askResolveParams(askExtractFrame("Which accounts made debits just under the limit?"), views)
		Expect(issue).NotTo(BeNil())
		Expect(issue.Kind).To(Equal("clarify"))
		Expect(issue.Message).To(ContainSubstring("does not say the limit"))
		uses, issue := askResolveParams(askExtractFrame("Which accounts made debits just under a limit of 500,000?"), views)
		Expect(issue).To(BeNil())
		settings := askParamSettings(uses)
		Expect(settings).To(HaveKeyWithValue("app.limit_amount", "500000"))
		Expect(settings).To(HaveKeyWithValue("app.limit_share_percent", "95"))
		notes := askParamNotes(uses)
		Expect(notes).To(ContainElement(ContainSubstring("Limit used: 500000 rupees (read from your question)")))
		Expect(notes).To(ContainElement(ContainSubstring("Share of the limit where the band starts used: 95 percent (the default")))
		_, issue = askResolveParams(askExtractFrame("debits under a limit of 500000 and under a limit of 400000"), views)
		Expect(issue).NotTo(BeNil())
		Expect(issue.Kind).To(Equal("clarify"))
		uses, issue = askResolveParams(askExtractFrame("debits of at least 90 percent of the limit of 100000"), views)
		Expect(issue).To(BeNil())
		Expect(askParamSettings(uses)).To(HaveKeyWithValue("app.limit_share_percent", "90"))
	})

	It("reads the pass-through settings at several values and leaves the defaults when the question gives none", func() {
		views := []string{"v_account_flows"}
		for question, want := range map[string][2]string{
			"which accounts received money that went out again within 10 minutes?":    {"10", "90"},
			"which accounts received money that went out again within 45 minutes?":    {"45", "90"},
			"which accounts passed on at least 99% of what came in within 3 minutes?": {"3", "99"},
			"which accounts passed on money after it came in?":                        {"10", "90"},
		} {
			uses, issue := askResolveParams(askExtractFrame(question), views)
			Expect(issue).To(BeNil(), "%s", question)
			settings := askParamSettings(uses)
			Expect(settings["app.flow_window_minutes"]).To(Equal(want[0]), "%s", question)
			Expect(settings["app.flow_share_percent"]).To(Equal(want[1]), "%s", question)
		}
		_, issue := askResolveParams(askExtractFrame("which accounts passed on at least 150% of what came in?"), views)
		Expect(issue).NotTo(BeNil(), "a share outside the bounds is refused with the bounds, never clipped")
		Expect(issue.Kind).To(Equal("range"))
		Expect(issue.Message).To(ContainSubstring("from 1 to 100 percent"))
	})

	DescribeTable("finds an account in a question as its search key",
		func(question string, want ...string) {
			f := askExtractFrame(question)
			if len(want) == 0 {
				Expect(f.Accounts).To(BeEmpty(), "%s", question)
				return
			}
			Expect(f.Accounts).To(Equal(want), "%s", question)
			Expect(f.Identifiers()).To(ContainElement(want[0]))
			Expect(f.Numbers).To(BeEmpty(), "the digits of an account are not a phone number")
		},
		Entry("written compactly", "Who paid the account PK00SYNT0000000000001001?", account),
		Entry("written with spaces", "Who paid the account PK00 SYNT 0000 0000 0000 1001?", account),
		Entry("in lower case", "who paid pk00synt0000000000001001", account),
		Entry("two accounts", "Which counterparties do PK00SYNT0000000000001001 and PK00SYNT0000000000001002 share?", account, "PK00SYNT0000000000001002"),
		Entry("the same account twice is one", "Did PK00SYNT0000000000001001 pay PK00 SYNT 0000 0000 0000 1001?", account),
		Entry("a phone number is not an account", "Did 923001110001 send money to 923001110002?"),
		Entry("a plate is not an account", "How many times was the plate SYNT-0420 seen?"))

	It("keeps a phone number and an account apart in one question", func() {
		f := askExtractFrame("Did 923001110001 send money to the account PK00SYNT0000000000001001?")
		Expect(f.Numbers).To(Equal([]string{"923001110001"}))
		Expect(f.Accounts).To(Equal([]string{account}))
	})

	Describe("the query's duty to the account and to the time of day", func() {
		var cat *askCatalogue
		BeforeEach(func() {
			var err error
			cat, err = askGoldenCatalogue()
			Expect(err).NotTo(HaveOccurred())
		})
		check := func(question, sql string, views ...string) []askObligation {
			v, rej := askValidate(sql, cat, views)
			Expect(rej).To(BeNil(), sql)
			return askCheckFrame(askExtractFrame(question), v)
		}
		kinds := func(list []askObligation) []string {
			out := []string{}
			for _, o := range list {
				out = append(out, o.Kind)
			}
			return out
		}

		It("asks for the account when the query leaves it out, and refuses an account the question did not name", func() {
			q := "What is the largest single debit of the account " + account + "?"
			Expect(check(q, "SELECT amount, txn_utc, reference FROM v_bank_transactions WHERE account = '"+account+"' AND direction = 'debit' ORDER BY amount DESC LIMIT 1", "v_bank_transactions")).To(BeEmpty())
			Expect(kinds(check(q, "SELECT amount, txn_utc, reference FROM v_bank_transactions WHERE direction = 'debit' ORDER BY amount DESC LIMIT 1", "v_bank_transactions"))).To(ContainElement("IDENTIFIER"))
		})

		It("treats credit and debit as the direction of money, never asking for incoming or outgoing", func() {
			received := "How much did the account " + account + " receive in March 2026?"
			credit := "SELECT sum(amount) AS total FROM v_bank_transactions WHERE account = '" + account + "' AND direction = 'credit' AND txn_utc >= '2026-03-01' AND txn_utc < '2026-04-01'"
			Expect(kinds(check(received, credit, "v_bank_transactions"))).NotTo(ContainElement("DIRECTION"))
			sent := "How much did the account " + account + " send out in March 2026?"
			debit := "SELECT sum(amount) AS total FROM v_bank_transactions WHERE account = '" + account + "' AND direction = 'debit' AND txn_utc >= '2026-03-01' AND txn_utc < '2026-04-01'"
			Expect(kinds(check(sent, debit, "v_bank_transactions"))).NotTo(ContainElement("DIRECTION"))
			// a direction filter on a question that names no direction is still refused
			Expect(kinds(check("How many transactions did the account "+account+" have?", "SELECT count(*) AS n FROM v_bank_transactions WHERE account = '"+account+"' AND direction = 'credit'", "v_bank_transactions"))).To(ContainElement("DIRECTION"))
		})

		It("keeps a question about minutes to the rows that state a time of day", func() {
			q := "Which transactions of the account " + account + " were made within 10 minutes of each other?"
			bad := "SELECT reference, txn_utc FROM v_bank_transactions WHERE account = '" + account + "' ORDER BY txn_utc LIMIT 200"
			unmet := check(q, bad, "v_bank_transactions")
			Expect(kinds(unmet)).To(ContainElement("TIME_PRECISION"))
			good := "SELECT reference, txn_utc FROM v_bank_transactions WHERE account = '" + account + "' AND time_precision <> 'date' ORDER BY txn_utc LIMIT 200"
			Expect(kinds(check(q, good, "v_bank_transactions"))).NotTo(ContainElement("TIME_PRECISION"))
			// a question about a day or a total is left alone
			Expect(kinds(check("How many transactions did the account "+account+" have?", "SELECT count(*) AS n FROM v_bank_transactions WHERE account = '"+account+"'", "v_bank_transactions"))).NotTo(ContainElement("TIME_PRECISION"))
		})

		It("says what was searched when a transaction view finds nothing, with the values used, and never calls a pattern an offence", func() {
			flows, rej := askValidate("SELECT account, credit_utc FROM v_account_flows LIMIT 200", cat, []string{"v_account_flows"})
			Expect(rej).To(BeNil())
			f := askExtractFrame("Which accounts passed on at least 95% of what came in within 5 minutes?")
			uses, _ := askResolveParams(f, []string{"v_account_flows"})
			f.ParamUses = uses
			Expect(askTransactionAbsence(flows, f)).To(Equal("no credit was followed by a debit of at least 95 percent of it within 5 minutes"))
			bands, rej := askValidate("SELECT account, transactions FROM v_account_limit_bands LIMIT 200", cat, []string{"v_account_limit_bands"})
			Expect(rej).To(BeNil())
			g := askExtractFrame("Which accounts made debits just under a limit of 500,000?")
			guses, _ := askResolveParams(g, []string{"v_account_limit_bands"})
			g.ParamUses = guses
			Expect(askTransactionAbsence(bands, g)).To(Equal("no debit is at or above 95 percent of the limit 500000 and below it"))
			txns, rej := askValidate("SELECT reference FROM v_bank_transactions WHERE account = '"+account+"' LIMIT 200", cat, []string{"v_bank_transactions"})
			Expect(rej).To(BeNil())
			Expect(askTransactionAbsence(txns, askExtractFrame("List the transactions of "+account))).To(Equal("no transaction of " + account + " was found in the files of this case"))
			shared, rej := askValidate("SELECT counterparty FROM v_account_counterparties WHERE account = '"+account+"' LIMIT 200", cat, []string{"v_account_counterparties"})
			Expect(rej).To(BeNil())
			Expect(askTransactionAbsence(shared, askExtractFrame("Which counterparties do "+account+" and PK00SYNT0000000000001002 share?"))).To(ContainSubstring("have no counterparty in common"))
			// the correction in front of a question that names an offence
			Expect(askPremiseLead(bands, askExtractFrame("Is this structuring? Which accounts made debits just under a limit of 500000?"))).To(Equal("A pattern in the rows is not a finding of laundering, structuring or fraud: the records do not say why money moved."))
			Expect(askPremiseLead(bands, g)).To(Equal(""))
		})
	})

	It("adds the caveats of the transaction views: amounts as stated, and a pattern is not a finding", func() {
		for _, name := range []string{"v_bank_transactions", "v_account_activity"} {
			Expect(askViewCaveatText[name]).To(ContainSubstring("Amounts are as the source states them"), name)
		}
		for _, name := range []string{"v_account_counterparties", "v_account_flows", "v_account_limit_bands"} {
			Expect(askViewCaveatText[name]).To(ContainSubstring("a pattern, not a finding of laundering, structuring or any offence"), name)
		}
	})

	It("declines the transaction views for a case with no typed transactions", func() {
		for _, view := range []string{"v_bank_transactions", "v_account_counterparties", "v_account_activity", "v_account_flows", "v_account_limit_bands"} {
			Expect(askCoverageGap(askCoverage{AnyTransaction: false, AnyComm: true}, []string{view})).To(ContainSubstring("no typed bank or wallet transactions"), view)
			Expect(askCoverageGap(askCoverage{AnyTransaction: true}, []string{view})).To(Equal(""), view)
		}
		Expect(askCoverageGap(askCoverage{AnyTransaction: true, Files: 4, Typed: 1}, []string{"v_bank_transactions"})).To(Equal(""))
	})

	It("probes the columns that can hold an account, and tells a statement's own account from the other side of a transaction", func() {
		for _, name := range []string{"account", "counterparty", "counterparty_account", "counterparty_msisdn", "received_from", "sent_to"} {
			Expect(askProbeColumn(name)).To(BeTrue(), name)
		}
		Expect(askProbeColumn("channel")).To(BeFalse())
		hint := askRoleHint(account, map[string]bool{"account": true}, []string{"v_bank_transactions.counterparty_account"})
		Expect(hint).To(ContainSubstring("never the account of a statement"))
		Expect(hint).To(ContainSubstring("counterparty_account"))
		hint = askRoleHint(account, map[string]bool{"counterparty_account": true}, []string{"v_bank_transactions.account"})
		Expect(hint).To(ContainSubstring("never the other side of a transaction"))
		Expect(askRoleHint(account, map[string]bool{"account": true}, []string{"v_bank_transactions.account"})).To(Equal(""))
	})
})

// Found by the transactions judgement run on case 119 (2026-10-28).
var _ = Describe("Ask engine transactions after the first measurement", func() {
	const account = "PK00SYNT0000000000001001"
	var cat *askCatalogue
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})
	check := func(question, sql string, views ...string) []string {
		v, rej := askValidate(sql, cat, views)
		Expect(rej).To(BeNil(), sql)
		out := []string{}
		for _, o := range askCheckFrame(askExtractFrame(question), v) {
			out = append(out, o.Kind)
		}
		return out
	}

	It("reads the limit that a share is of: debits between 95 percent of 400,000 and the limit", func() {
		Expect(askReadParams("which accounts have debits between 95 percent of 400,000 and the limit, and how many debits?")["limit_amount"].Values).To(Equal([]int{400000}))
		Expect(askReadParams("which accounts made many debits just under the limit?")["limit_amount"].Found).To(BeFalse())
		Expect(askReadParams("what is 95 percent of 400,000?")["limit_amount"].Found).To(BeFalse())
	})

	It("asks for a count of the rows when the question asks how many transactions, and for the direction of money", func() {
		q := "How many credits did the account " + account + " receive from the account PK00SYNT0000000000009002?"
		sum := "SELECT sum(t.signed_amount) AS credits FROM v_bank_transactions t WHERE t.account = '" + account + "' AND t.counterparty_account = 'PK00SYNT0000000000009002' AND t.direction = 'credit'"
		Expect(check(q, sum, "v_bank_transactions")).To(ContainElement("AGGREGATE"))
		count := "SELECT count(*) AS n FROM v_bank_transactions t WHERE t.account = '" + account + "' AND t.counterparty_account = 'PK00SYNT0000000000009002' AND t.direction = 'credit'"
		Expect(check(q, count, "v_bank_transactions")).NotTo(ContainElement("AGGREGATE"))
		out := "How much did the account " + account + " pay out in March 2026?"
		credit := "SELECT sum(total_amount) AS total FROM v_account_counterparties WHERE account = '" + account + "' AND direction = 'credit'"
		debit := "SELECT sum(amount) AS total FROM v_bank_transactions WHERE account = '" + account + "' AND direction = 'debit' AND txn_utc >= '2026-03-01' AND txn_utc < '2026-04-01'"
		Expect(check("How much was debited from the account "+account+" in March 2026?", debit, "v_bank_transactions")).To(BeEmpty())
		_ = out
		Expect(check("How much was credited to the account "+account+" in March 2026?", debit, "v_bank_transactions")).To(ContainElement("DIRECTION"))
		_ = credit
	})

	It("does not ask for a direction filter on the views of pass-through pairs and of debits under a limit", func() {
		flows := "SELECT account, credit_amount, debit_amount FROM v_account_flows ORDER BY account"
		Expect(check("Which accounts received money that went out again within 80 minutes, at least 90 percent of it?", flows, "v_account_flows")).NotTo(ContainElement("DIRECTION"))
		bands := "SELECT account, transactions, total_amount FROM v_account_limit_bands ORDER BY account"
		Expect(check("Which accounts made debits just under a limit of 500,000, and how many?", bands, "v_account_limit_bands")).NotTo(ContainElement("DIRECTION"))
	})

	DescribeTable("shortlists the right bank view",
		func(question, want string) {
			Expect(cat.Shortlist(question, 0).Views).To(ContainElement(want), "%s", question)
		},
		Entry("a credit followed by a debit is a pass-through pair", "Find the credits that were followed by a debit of at least 90% of the credit within 10 minutes.", "v_account_flows"),
		Entry("accounts two accounts both transact with", "Which accounts did both PK00SYNT0000000000002011 and PK00SYNT0000000000002005 transact with?", "v_account_counterparties"),
		Entry("how much an account paid out in a month", "How much did the account PK00SYNT0000000000002016 pay out in March 2026?", "v_bank_transactions"))

	It("keeps the camera views out of questions about accounts", func() {
		Expect(cat.Shortlist("Which accounts did both PK00SYNT0000000000002011 and PK00SYNT0000000000002005 transact with?", 0).Views).NotTo(ContainElement("v_plate_visits"))
	})
})
