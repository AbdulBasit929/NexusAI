package main

import (
	"bytes"
	"context"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mudler/LocalAI/pkg/forensicrequest"
)

// Run 15 (pre-registration, "Run 15 plan"): the classes the fresh-seed check and the shape probes found. Each spec
// pairs a question with a query that ignores part of it (which must be flagged) and a query that honours it
// (which must pass), in every form the model is likely to write, and then runs the loop with a scripted model.

var _ = Describe("Governed SQL run 15 checks", func() {
	var (
		views map[string]*govSQLView
		all   []*govSQLView
	)
	BeforeEach(func() {
		views = govSQLTestViews()
		all = nil
		for _, v := range views {
			all = append(all, v)
		}
	})
	factsFor := func(question string, names ...string) govSQLQuestionFacts {
		picked := []*govSQLView{}
		for _, name := range names {
			picked = append(picked, views[name])
		}
		return govSQLExtractFacts(question, picked, all)
	}
	kinds := func(facts govSQLQuestionFacts, sql string) []string {
		validated, rejection := govSQLValidate(sql, views)
		Expect(rejection).To(BeNil(), "validator rejected %q: %v", sql, rejection)
		out := []string{}
		for _, item := range govSQLCheck(facts, validated) {
			out = append(out, item.Kind)
		}
		return out
	}

	Describe("a window of the day", func() {
		const question = "How many calls were made between 11 pm and 3 am?"

		It("reads the window as the hours from the first up to, not including, the last", func() {
			cases := map[string][]int{
				question: {23, 0, 1, 2},
				"How many access log entries were recorded between 9 am and 5 pm?": {9, 10, 11, 12, 13, 14, 15, 16},
				"How many calls started from 10 pm to 4 am?":                       {22, 23, 0, 1, 2, 3},
				"How many calls were made between 23:00 and 03:00?":                {23, 0, 1, 2},
				"How many calls were made between 11pm and 3am?":                   {23, 0, 1, 2},
				"How many calls were made between 12 am and 6 am?":                 {0, 1, 2, 3, 4, 5},
				"How many calls were made between 8 pm and 12 am?":                 {20, 21, 22, 23},
				"How many calls were made between 12 pm and 1 pm?":                 {12},
				"How many calls were made from 9:00 to 17:00?":                     {9, 10, 11, 12, 13, 14, 15, 16},
				"How many calls were made 10 pm - 2 am?":                           {22, 23, 0, 1},
			}
			for q, hours := range cases {
				w := govSQLReadClockWindow(q)
				Expect(w).NotTo(BeNil(), q)
				Expect(w.hours()).To(Equal(hours), q)
			}
		})

		It("does not read a pair of numbers, dates or a half hour as a window of whole hours", func() {
			for _, q := range []string{
				"How many calls were made between 9 and 5?",
				"Which numbers made between 10 and 20 calls?",
				"How many calls were made between 2026-04-01 and 2026-04-30?",
				"How many calls lasted 3 to 5 minutes?",
				"How many calls were made between 9:30 am and 5 pm?",
				"How many calls were made between 11 pm and 11 pm?",
				"How many calls were made on 2026-04-02 from 10-12?",
			} {
				Expect(govSQLReadClockWindow(q)).To(BeNil(), q)
			}
		})

		It("takes a window out of the question before a quantity is read from it", func() {
			facts := factsFor(question, "v_cdr")
			Expect(facts.Magnitude).To(BeEmpty())
			Expect(facts.Window).NotTo(BeNil())
			Expect(facts.TimeOfDay).To(BeTrue())
			// the window and a real threshold in one question: the threshold stays
			both := factsFor("How many calls lasting more than 10 minutes were made between 11 pm and 3 am?", "v_cdr")
			Expect(both.Magnitude).To(ContainSubstring("more than 10"))
			Expect(both.Numbers).To(ConsistOf("10"))
			// a half-hour window is not whole hours, and still not a quantity
			half := factsFor("How many calls were made between 9:30 am and 5 pm?", "v_cdr")
			Expect(half.Window).To(BeNil())
			Expect(half.Magnitude).To(BeEmpty())
			// numbers that are numbers are read as before
			Expect(factsFor("Which numbers made between 10 and 20 calls?", "v_cdr").Numbers).To(ConsistOf("10"))
		})

		It("accepts the window in every form that keeps the hours 23, 0, 1 and 2", func() {
			facts := factsFor(question, "v_cdr")
			for _, where := range []string{
				"EXTRACT(HOUR FROM event_time) >= 23 OR EXTRACT(HOUR FROM event_time) < 3",
				"EXTRACT(HOUR FROM event_time) >= 23 OR EXTRACT(HOUR FROM event_time) <= 2",
				"EXTRACT(HOUR FROM event_time) IN (23, 0, 1, 2)",
				"EXTRACT(HOUR FROM event_time) = 23 OR EXTRACT(HOUR FROM event_time) BETWEEN 0 AND 2",
				"EXTRACT(HOUR FROM event_time) NOT BETWEEN 3 AND 22",
				"EXTRACT(HOUR FROM event_time) NOT IN (3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22)",
				"date_part('hour', event_time) >= 23 OR date_part('hour', event_time) < 3",
				"EXTRACT(HOUR FROM event_time)::int IN (23, 0, 1, 2)",
				"NOT (EXTRACT(HOUR FROM event_time) >= 3 AND EXTRACT(HOUR FROM event_time) < 23)",
				"23 <= EXTRACT(HOUR FROM event_time) OR 3 > EXTRACT(HOUR FROM event_time)",
				"event_time::time >= '23:00' OR event_time::time < '03:00'",
				"event_time::time >= '23:00:00' OR event_time::time <= '03:00:00'",
				"event_time::time >= time '23:00' OR event_time::time < time '03:00'",
				"(EXTRACT(HOUR FROM event_time) >= 23 OR EXTRACT(HOUR FROM event_time) < 3) AND call_duration_seconds > 0",
				"call_duration_seconds > 0 AND (EXTRACT(HOUR FROM event_time) >= 23 OR EXTRACT(HOUR FROM event_time) < 3)",
			} {
				Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE "+where)).To(BeEmpty(), where)
			}
		})

		It("accepts the window as a FILTER on the aggregate", func() {
			facts := factsFor(question, "v_cdr")
			Expect(kinds(facts, "SELECT COUNT(*) FILTER (WHERE EXTRACT(HOUR FROM event_time) IN (23, 0, 1, 2)) AS number_of_calls FROM v_cdr WHERE event_time >= '2026-01-01'")).To(BeEmpty())
			Expect(kinds(facts, "SELECT COUNT(*) FILTER (WHERE EXTRACT(HOUR FROM event_time) IN (22, 23, 0, 1, 2)) AS number_of_calls FROM v_cdr WHERE event_time >= '2026-01-01'")).To(ConsistOf("WINDOW"))
		})

		It("flags a window that keeps another set of hours, and says which", func() {
			facts := factsFor(question, "v_cdr")
			for _, where := range []string{
				"EXTRACT(HOUR FROM event_time) BETWEEN 23 AND 3",                                  // nothing: the first is larger than the second
				"EXTRACT(HOUR FROM event_time) >= 23 OR EXTRACT(HOUR FROM event_time) <= 3",       // the hour 3 is not in the window
				"EXTRACT(HOUR FROM event_time) IN (23, 0, 1, 2, 3)",                               // likewise
				"EXTRACT(HOUR FROM event_time) >= 11 OR EXTRACT(HOUR FROM event_time) < 3",        // 11 am, not 11 pm
				"EXTRACT(HOUR FROM event_time) >= 23",                                             // only before midnight
				"EXTRACT(HOUR FROM event_time) < 3",                                               // only after midnight
				"EXTRACT(HOUR FROM event_time) >= 23 AND EXTRACT(HOUR FROM event_time) < 3",       // both at once: nothing
				"event_time::time BETWEEN '23:00' AND '03:00'",                                    // nothing, as a time
				"event_time::time >= '23:00' OR event_time::time < '04:00'",                       // an hour too many
				"EXTRACT(HOUR FROM event_time) >= 23 OR call_duration_seconds > 0",                // or any call at all
				"NOT (EXTRACT(HOUR FROM event_time) >= 3 AND EXTRACT(HOUR FROM event_time) < 22)", // 22 is kept
			} {
				Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE "+where)).To(ConsistOf("WINDOW"), where)
			}
			validated, _ := govSQLValidate("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE EXTRACT(HOUR FROM event_time) BETWEEN 23 AND 3", views)
			unmet := govSQLCheckWindow(facts, validated)
			Expect(unmet).To(HaveLen(1))
			Expect(unmet[0].Message).To(ContainSubstring("hours 23, 0, 1 and 2"))
			Expect(unmet[0].Message).To(ContainSubstring("keeps the hours none"))
			Expect(unmet[0].Message).To(ContainSubstring("EXTRACT(HOUR FROM column) >= 23 OR EXTRACT(HOUR FROM column) < 3"))
			validated, _ = govSQLValidate("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE EXTRACT(HOUR FROM event_time) IN (22, 23, 0, 1, 2)", views)
			Expect(govSQLCheckWindow(facts, validated)[0].Message).To(ContainSubstring("keeps the hours 22 to 2"))
		})

		It("flags a query with no condition on the hour, but not one it cannot read", func() {
			facts := factsFor(question, "v_cdr")
			Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr")).To(ConsistOf("TIME", "WINDOW"))
			Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE event_time >= '2026-01-01'")).To(ConsistOf("WINDOW"))
			Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE EXTRACT(DOW FROM event_time) IN (0, 6)")).To(ConsistOf("WINDOW"))
			// a form the check does not follow is left to the rest of the verifier: it is never a reason to reject
			Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE CASE WHEN EXTRACT(HOUR FROM event_time) >= 23 THEN 1 ELSE 0 END = 1")).To(BeEmpty())
			Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE EXTRACT(HOUR FROM event_time) % 24 >= 23")).To(BeEmpty())
			Expect(kinds(facts, "SELECT SUM(CASE WHEN EXTRACT(HOUR FROM event_time) IN (23, 0, 1, 2) THEN 1 ELSE 0 END) AS number_of_calls FROM v_cdr WHERE event_time >= '2026-01-01'")).To(BeEmpty())
			Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE to_char(event_time, 'HH24') IN ('23', '00', '01', '02')")).To(BeEmpty())
		})

		It("holds a daytime window to its own end hour", func() {
			facts := factsFor("How many access log entries were recorded between 9 am and 5 pm?", "v_access_log")
			Expect(kinds(facts, "SELECT COUNT(*) AS n FROM v_access_log WHERE EXTRACT(HOUR FROM event_time) >= 9 AND EXTRACT(HOUR FROM event_time) < 17")).To(BeEmpty())
			Expect(kinds(facts, "SELECT COUNT(*) AS n FROM v_access_log WHERE EXTRACT(HOUR FROM event_time) BETWEEN 9 AND 16")).To(BeEmpty())
			Expect(kinds(facts, "SELECT COUNT(*) AS n FROM v_access_log WHERE EXTRACT(HOUR FROM event_time) BETWEEN 9 AND 17")).To(ConsistOf("WINDOW"))
			Expect(kinds(facts, "SELECT COUNT(*) AS n FROM v_access_log WHERE EXTRACT(HOUR FROM event_time) BETWEEN 9 AND 5")).To(ConsistOf("WINDOW"))
			validated, _ := govSQLValidate("SELECT COUNT(*) AS n FROM v_access_log WHERE EXTRACT(HOUR FROM event_time) BETWEEN 9 AND 17", views)
			Expect(govSQLCheckWindow(facts, validated)[0].Message).To(ContainSubstring("Write EXTRACT(HOUR FROM column) >= 9 AND EXTRACT(HOUR FROM column) < 17"))
		})

		It("puts the window into the question's own message, with the hours", func() {
			facts := factsFor(question, "v_cdr")
			prompt := govSQLUserPrompt(question, facts, []*govSQLView{views["v_cdr"]})
			Expect(prompt).To(ContainSubstring("from 23:00 up to, but not including, 03:00"))
			Expect(prompt).To(ContainSubstring("hours 23, 0, 1 and 2"))
			Expect(prompt).To(ContainSubstring("EXTRACT(HOUR FROM column) >= 23 OR EXTRACT(HOUR FROM column) < 3"))
			Expect(prompt).NotTo(ContainSubstring("The question states a condition"))
		})

		It("no longer tells a window question that night is hours 0 to 5", func() {
			facts := factsFor(question, "v_cdr")
			Expect(facts.Window).NotTo(BeNil())
			for _, item := range func() []govSQLObligation {
				validated, _ := govSQLValidate("SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-01-01'", views)
				return govSQLCheck(facts, validated)
			}() {
				Expect(item.Message).NotTo(ContainSubstring("night is hours"))
			}
			// a question about the night keeps its definition
			night := factsFor("How many data sessions were there at night?", "v_ipdr")
			validated, _ := govSQLValidate("SELECT COUNT(*) AS sessions FROM v_ipdr WHERE event_time >= '2026-01-01'", views)
			found := false
			for _, item := range govSQLCheck(night, validated) {
				found = found || (item.Kind == "TIME_OF_DAY" && strings.Contains(item.Message, "night is hours 0 to 5"))
			}
			Expect(found).To(BeTrue())
		})
	})

	Describe("the days of the week", func() {
		It("reads weekends, weekdays and named days, and leaves alone what is a date or a grouping", func() {
			weekend := govSQLReadDays("How many calls were made on weekends?")
			Expect(weekend).NotTo(BeNil())
			Expect(weekend.names()).To(Equal([]string{"Saturday", "Sunday"}))
			Expect(weekend.isodow()).To(Equal([]string{"6", "7"}))
			Expect(govSQLReadDays("How many calls were made on weekdays?").names()).To(Equal([]string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday"}))
			Expect(govSQLReadDays("How many access log entries were recorded on a Saturday or a Sunday?").names()).To(Equal([]string{"Saturday", "Sunday"}))
			Expect(govSQLReadDays("How many calls were made on Fridays?").names()).To(Equal([]string{"Friday"}))
			Expect(govSQLReadDays("How many calls were made on Mondays, Wednesdays and Fridays?").isodow()).To(Equal([]string{"1", "3", "5"}))
			for _, q := range []string{
				"Which weekend had the most calls?",
				"How many calls were made each Friday?",
				"How many calls were made last Friday?",
				"How many calls were made on Friday 2026-04-03?",
				"How many calls were made from Monday to Friday?",
				"How many calls were made between Monday and Friday?",
				"How many calls were there per weekday?",
				"How many calls were made?",
			} {
				Expect(govSQLReadDays(q)).To(BeNil(), q)
			}
		})

		It("accepts weekends in every form that keeps Saturday and Sunday", func() {
			facts := factsFor("How many calls were made on weekends?", "v_cdr")
			for _, where := range []string{
				"EXTRACT(ISODOW FROM event_time) IN (6, 7)",
				"EXTRACT(ISODOW FROM event_time) >= 6",
				"EXTRACT(ISODOW FROM event_time) > 5",
				"EXTRACT(DOW FROM event_time) IN (0, 6)",
				"EXTRACT(DOW FROM event_time) = 0 OR EXTRACT(DOW FROM event_time) = 6",
				"date_part('dow', event_time) IN (6, 0)",
				"EXTRACT(ISODOW FROM event_time) NOT BETWEEN 1 AND 5",
				"EXTRACT(DOW FROM event_time) NOT IN (1, 2, 3, 4, 5)",
				"to_char(event_time, 'Dy') IN ('Sat', 'Sun')",
				"to_char(event_time, 'Day') IN ('Saturday ', 'Sunday   ')",
				"TRIM(to_char(event_time, 'Day')) IN ('Saturday', 'Sunday')",
				"LOWER(to_char(event_time, 'DY')) IN ('sat', 'sun')",
				"EXTRACT(ISODOW FROM event_time)::int IN (6, 7) AND call_duration_seconds > 0",
			} {
				Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE "+where)).To(BeEmpty(), where)
			}
		})

		It("flags the Friday and Saturday of the probe, and says which days the query keeps", func() {
			facts := factsFor("How many calls were made on weekends?", "v_cdr")
			for _, where := range []string{
				"EXTRACT(ISODOW FROM event_time) IN (5, 6)",
				"EXTRACT(DOW FROM event_time) IN (5, 6)",
				"EXTRACT(DOW FROM event_time) IN (6, 7)", // 7 is no day of the week: Saturday only
				"EXTRACT(ISODOW FROM event_time) IN (6, 7, 1)",
				"EXTRACT(ISODOW FROM event_time) = 6",
				"to_char(event_time, 'Dy') IN ('Fri', 'Sat')",
				"EXTRACT(DOW FROM event_time) BETWEEN 1 AND 5",
			} {
				Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE "+where)).To(ConsistOf("DAYS"), where)
			}
			validated, _ := govSQLValidate("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE EXTRACT(ISODOW FROM event_time) IN (5, 6)", views)
			unmet := govSQLCheckWindow(facts, validated)
			Expect(unmet).To(HaveLen(1))
			Expect(unmet[0].Message).To(ContainSubstring("The query keeps Friday and Saturday."))
			Expect(unmet[0].Message).To(ContainSubstring("EXTRACT(ISODOW FROM column) IN (6, 7)"))
			Expect(unmet[0].Message).To(ContainSubstring("EXTRACT(DOW ...) numbers Sunday 0 to Saturday 6"))
		})

		It("flags weekdays kept as other days, and a query with no condition on the day", func() {
			facts := factsFor("How many calls were made on weekdays?", "v_cdr")
			Expect(kinds(facts, "SELECT COUNT(*) AS n FROM v_cdr WHERE EXTRACT(ISODOW FROM event_time) BETWEEN 1 AND 5")).To(BeEmpty())
			Expect(kinds(facts, "SELECT COUNT(*) AS n FROM v_cdr WHERE EXTRACT(DOW FROM event_time) BETWEEN 1 AND 5")).To(BeEmpty())
			Expect(kinds(facts, "SELECT COUNT(*) AS n FROM v_cdr WHERE EXTRACT(DOW FROM event_time) NOT IN (0, 6)")).To(BeEmpty())
			Expect(kinds(facts, "SELECT COUNT(*) AS n FROM v_cdr WHERE EXTRACT(DOW FROM event_time) BETWEEN 1 AND 6")).To(ConsistOf("DAYS"))
			Expect(kinds(facts, "SELECT COUNT(*) AS n FROM v_cdr WHERE EXTRACT(DOW FROM event_time) BETWEEN 0 AND 4")).To(ConsistOf("DAYS"))
			Expect(kinds(facts, "SELECT COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-01'")).To(ConsistOf("DAYS"))
			// grouping by the day of the week keeps every day: that is not what "on weekdays" asks for, and it is flagged
			// only when the condition is read; a grouping alone is left to the rest of the verifier
			Expect(kinds(facts, "SELECT EXTRACT(ISODOW FROM event_time) AS d, COUNT(*) AS n FROM v_cdr WHERE event_time >= '2026-04-01' GROUP BY 1")).To(BeEmpty())
		})

		It("names the days in the question's own message", func() {
			facts := factsFor("How many calls were made on weekends?", "v_cdr")
			prompt := govSQLUserPrompt("How many calls were made on weekends?", facts, []*govSQLView{views["v_cdr"]})
			Expect(prompt).To(ContainSubstring("The question names the days Saturday and Sunday"))
			Expect(prompt).To(ContainSubstring("EXTRACT(ISODOW FROM column) IN (6, 7)"))
		})
	})

	Describe("the highest, the lowest and the average", func() {
		It("flags a sum for the highest total, and takes a maximum however it is written", func() {
			facts := factsFor("What is the highest total in the transactions?", "v_transaction")
			Expect(facts.WantsMax).To(BeTrue())
			Expect(kinds(facts, "SELECT SUM(amount) AS highest_total FROM v_transaction")).To(ConsistOf("AGGREGATE"))
			Expect(kinds(facts, "SELECT COUNT(*) AS highest_total FROM v_transaction")).To(ConsistOf("AGGREGATE"))
			Expect(kinds(facts, "SELECT AVG(amount) AS highest_total FROM v_transaction")).To(ConsistOf("AGGREGATE"))
			Expect(kinds(facts, "SELECT MAX(amount) AS highest_total FROM v_transaction")).To(BeEmpty())
			Expect(kinds(facts, "SELECT amount AS highest_total FROM v_transaction ORDER BY amount DESC LIMIT 1")).To(BeEmpty())
			Expect(kinds(facts, "SELECT amount AS highest_total FROM v_transaction ORDER BY amount DESC FETCH FIRST 1 ROW ONLY")).To(BeEmpty())
			Expect(kinds(facts, "SELECT amount FROM (SELECT amount, RANK() OVER (ORDER BY amount DESC) AS r FROM v_transaction) t WHERE r = 1")).To(BeEmpty())
			Expect(kinds(facts, "SELECT amount AS highest_total FROM v_transaction ORDER BY amount ASC LIMIT 1")).To(ConsistOf("AGGREGATE"))
			Expect(kinds(facts, "SELECT amount AS highest_total FROM v_transaction ORDER BY amount DESC")).To(ConsistOf("AGGREGATE"))
			validated, _ := govSQLValidate("SELECT SUM(amount) AS highest_total FROM v_transaction", views)
			unmet := govSQLCheckAggregate(facts, validated)
			Expect(unmet[0].Message).To(ContainSubstring("MAX of the matching column"))
			Expect(unmet[0].Message).To(ContainSubstring("the query uses SUM"))
		})

		It("reads every word that asks for the highest or the lowest, and no word that asks for a total", func() {
			for _, q := range []string{"What is the highest amount?", "What is the largest amount?", "What is the biggest transaction?", "What is the greatest amount?",
				"What is the maximum amount?", "What is the longest call?", "What is the latest call?", "What is the newest record?"} {
				Expect(govSQLRxWantsMax.MatchString(q)).To(BeTrue(), q)
			}
			for _, q := range []string{"What is the lowest amount?", "What is the smallest amount?", "What is the minimum amount?", "What is the shortest call?",
				"What is the earliest call?", "What is the oldest record?"} {
				Expect(govSQLRxWantsMin.MatchString(q)).To(BeTrue(), q)
			}
			for _, q := range []string{"What is the total amount?", "Which number made the most calls?", "How many calls were made?", "What is the first call?", "What is the last call?", "Show the top 3 numbers"} {
				Expect(govSQLRxWantsMax.MatchString(q) || govSQLRxWantsMin.MatchString(q)).To(BeFalse(), q)
			}
		})

		It("leaves a ranking and the earliest and latest alone when the query ranks or takes the extreme", func() {
			Expect(kinds(factsFor("Which number made the most calls?", "v_cdr"), "SELECT msisdn, COUNT(*) AS calls FROM v_cdr GROUP BY msisdn ORDER BY calls DESC LIMIT 5")).To(BeEmpty())
			Expect(kinds(factsFor("Which call type has the highest number of records?", "v_cdr"), "SELECT call_type, COUNT(*) AS n FROM v_cdr GROUP BY call_type ORDER BY n DESC LIMIT 1")).To(BeEmpty())
			Expect(kinds(factsFor("What is the earliest call in the data?", "v_cdr"), "SELECT MIN(call_start) AS earliest_call FROM v_cdr")).To(BeEmpty())
			Expect(kinds(factsFor("What is the latest call in the data?", "v_cdr"), "SELECT MAX(call_start) AS latest_call FROM v_cdr")).To(BeEmpty())
			Expect(kinds(factsFor("What is the latest call in the data?", "v_cdr"), "SELECT call_start FROM v_cdr ORDER BY call_start DESC LIMIT 1")).To(BeEmpty())
			Expect(kinds(factsFor("What is the earliest call in the data?", "v_cdr"), "SELECT call_start FROM v_cdr ORDER BY call_start LIMIT 1")).To(BeEmpty())
			Expect(kinds(factsFor("What is the earliest call in the data?", "v_cdr"), "SELECT MAX(call_start) AS earliest_call FROM v_cdr")).To(ConsistOf("AGGREGATE"))
			both := factsFor("What are the earliest and the latest calls in the data?", "v_cdr")
			Expect(kinds(both, "SELECT MIN(call_start) AS earliest_call, MAX(call_start) AS latest_call FROM v_cdr")).To(BeEmpty())
			Expect(kinds(both, "SELECT MIN(call_start) AS earliest_call FROM v_cdr")).To(ConsistOf("AGGREGATE"))
		})

		It("takes an average as AVG, or as a sum over a count, and not as a sum", func() {
			facts := factsFor("What is the average duration of the calls?", "v_cdr")
			Expect(facts.WantsAvg).To(BeTrue())
			Expect(kinds(facts, "SELECT AVG(call_duration_seconds) AS average_duration FROM v_cdr")).To(BeEmpty())
			Expect(kinds(facts, "SELECT SUM(call_duration_seconds) / COUNT(*) AS average_duration FROM v_cdr")).To(BeEmpty())
			Expect(kinds(facts, "SELECT SUM(call_duration_seconds)::numeric / NULLIF(COUNT(*), 0) AS average_duration FROM v_cdr")).To(BeEmpty())
			Expect(kinds(facts, "SELECT SUM(call_duration_seconds) AS average_duration FROM v_cdr")).To(ConsistOf("AGGREGATE"))
			Expect(kinds(facts, "SELECT MAX(call_duration_seconds) AS average_duration FROM v_cdr")).To(ConsistOf("AGGREGATE"))
			perDay := factsFor("What is the average number of calls per day?", "v_cdr")
			// a division of two counts is an average too (the grouping check, an older one, has its own opinion of "per day")
			Expect(kinds(perDay, "SELECT COUNT(*)::numeric / COUNT(DISTINCT event_time::date) AS average_calls FROM v_cdr")).NotTo(ContainElement("AGGREGATE"))
			Expect(kinds(perDay, "SELECT AVG(c) AS average_calls FROM (SELECT COUNT(*) AS c FROM v_cdr GROUP BY event_time::date) t")).NotTo(ContainElement("AGGREGATE"))
		})

		It("says what is wanted in the question's own message", func() {
			prompt := govSQLUserPrompt("What is the highest total in the transactions?", factsFor("What is the highest total in the transactions?", "v_transaction"), []*govSQLView{views["v_transaction"]})
			Expect(prompt).To(ContainSubstring("use MAX of the matching column"))
			plain := govSQLUserPrompt("What is the total of the transactions?", factsFor("What is the total of the transactions?", "v_transaction"), []*govSQLView{views["v_transaction"]})
			Expect(plain).NotTo(ContainSubstring("MAX"))
		})
	})

	Describe("which day", func() {
		const question = "On which day does 923001110001 have the most call records?"

		It("asks for a date, unless the question asks for the day of the week or of the month", func() {
			Expect(factsFor(question, "v_cdr").WantsDate).To(BeTrue())
			Expect(factsFor("What date had the most calls?", "v_cdr").WantsDate).To(BeTrue())
			Expect(factsFor("Which day was the busiest?", "v_cdr").WantsDate).To(BeTrue())
			Expect(factsFor("Which day of the week has the most calls?", "v_cdr").WantsDate).To(BeFalse())
			Expect(factsFor("Which day of the month has the most calls?", "v_cdr").WantsDate).To(BeFalse())
			Expect(factsFor("Which weekday has the most calls?", "v_cdr").WantsDate).To(BeFalse())
			Expect(factsFor("How many calls were made each day?", "v_cdr").WantsDate).To(BeFalse())
		})

		It("flags the day of the month, and takes a date however it is written", func() {
			facts := factsFor(question, "v_cdr")
			const tail = " AS number_of_calls FROM v_cdr WHERE msisdn = '923001110001' GROUP BY 1 ORDER BY 2 DESC LIMIT 1"
			Expect(kinds(facts, "SELECT EXTRACT(DAY FROM event_time) AS day, COUNT(*)"+tail)).To(ConsistOf("DATE"))
			Expect(kinds(facts, "SELECT date_part('day', event_time) AS day, COUNT(*)"+tail)).To(ConsistOf("DATE"))
			Expect(kinds(facts, "SELECT to_char(event_time, 'DD') AS day, COUNT(*)"+tail)).To(ConsistOf("DATE"))
			Expect(kinds(facts, "SELECT event_time::date AS day, COUNT(*)"+tail)).To(BeEmpty())
			Expect(kinds(facts, "SELECT DATE(event_time) AS day, COUNT(*)"+tail)).To(BeEmpty())
			Expect(kinds(facts, "SELECT CAST(event_time AS date) AS day, COUNT(*)"+tail)).To(BeEmpty())
			Expect(kinds(facts, "SELECT date_trunc('day', event_time) AS day, COUNT(*)"+tail)).To(BeEmpty())
			Expect(kinds(facts, "SELECT date_trunc('day', event_time)::date AS day, COUNT(*)"+tail)).To(BeEmpty())
			Expect(kinds(facts, "SELECT to_char(event_time, 'YYYY-MM-DD') AS day, COUNT(*)"+tail)).To(BeEmpty())
			Expect(kinds(facts, "SELECT d AS day, c FROM (SELECT event_time::date AS d, COUNT(*) AS c FROM v_cdr WHERE msisdn = '923001110001' GROUP BY 1) t ORDER BY c DESC LIMIT 1")).To(BeEmpty())
		})

		It("leaves the day of the month alone when the question asks for it", func() {
			facts := factsFor("Which day of the month has the most calls?", "v_cdr")
			Expect(kinds(facts, "SELECT EXTRACT(DAY FROM event_time) AS day, COUNT(*) AS n FROM v_cdr GROUP BY 1 ORDER BY 2 DESC LIMIT 1")).To(BeEmpty())
		})
	})

	Describe("a threshold in a unit", func() {
		It("reads the unit that ends the quantity", func() {
			facts := factsFor("How many calls lasted more than 15 minutes?", "v_cdr")
			Expect(facts.Magnitude).To(Equal("more than 15 minutes"))
			Expect(facts.Numbers).To(ConsistOf("15"))
			unit, ok := govSQLUnitOf(facts.Magnitude)
			Expect(ok).To(BeTrue())
			Expect(unit.size).To(BeEquivalentTo(60))
			_, ok = govSQLUnitOf("more than 15")
			Expect(ok).To(BeFalse())
		})

		It("takes 15 minutes as 15, 900 seconds, a quarter of an hour or an interval, and nothing else", func() {
			facts := factsFor("How many calls lasted more than 15 minutes?", "v_cdr")
			for _, where := range []string{
				"call_duration_seconds > 900",
				"call_duration_seconds > 15",
				"call_duration_seconds / 60 > 15",
				"EXTRACT(EPOCH FROM (call_end - call_start)) > 900",
				"call_end - call_start > interval '15 minutes'",
				"call_end - call_start > INTERVAL '15 min'",
				"call_end - call_start > '00:15:00'::interval",
				"call_duration_seconds / 3600.0 > 0.25",
			} {
				Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE "+where)).To(BeEmpty(), where)
			}
			for _, where := range []string{"call_duration_seconds > 600", "call_duration_seconds > 90", "call_duration_seconds = 900", "call_end - call_start > interval '10 minutes'"} {
				Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE "+where)).To(ConsistOf("MAGNITUDE"), where)
			}
			Expect(kinds(facts, "SELECT COUNT(*) AS number_of_calls FROM v_cdr")).To(ConsistOf("MAGNITUDE"))
		})

		It("takes hours and days, sizes in both conventions, and percentages", func() {
			hours := factsFor("How many calls lasted longer than 2 hours?", "v_cdr")
			Expect(kinds(hours, "SELECT COUNT(*) AS n FROM v_cdr WHERE call_duration_seconds > 7200")).To(BeEmpty())
			Expect(kinds(hours, "SELECT COUNT(*) AS n FROM v_cdr WHERE call_duration_seconds > 3600")).To(ConsistOf("MAGNITUDE"))
			size := factsFor("How many data sessions used more than 5 MB?", "v_ipdr")
			for _, bytes := range []string{"5000000", "5242880", "5", "5000", "5120"} {
				Expect(kinds(size, "SELECT COUNT(*) AS n FROM v_ipdr WHERE bytes_down > "+bytes)).To(BeEmpty(), bytes)
			}
			Expect(kinds(size, "SELECT COUNT(*) AS n FROM v_ipdr WHERE bytes_down > 123")).To(ConsistOf("MAGNITUDE"))
			share := factsFor("Which numbers made more than 50 percent of the calls?", "v_cdr")
			Expect(kinds(share, "SELECT msisdn FROM v_cdr GROUP BY msisdn HAVING COUNT(*) * 1.0 / (SELECT COUNT(*) FROM v_cdr) > 0.5")).To(BeEmpty())
			// a quantity with no unit is read exactly as before
			plain := factsFor("Which numbers made more than 10 calls?", "v_cdr")
			Expect(kinds(plain, "SELECT msisdn, COUNT(*) AS calls FROM v_cdr GROUP BY msisdn HAVING COUNT(*) > 10")).To(BeEmpty())
			Expect(kinds(plain, "SELECT msisdn, COUNT(*) AS calls FROM v_cdr GROUP BY msisdn HAVING COUNT(*) > 7")).To(ConsistOf("MAGNITUDE"))
		})

		It("tells the model what the quantity is in the unit of the column", func() {
			facts := factsFor("How many calls lasted more than 15 minutes?", "v_cdr")
			Expect(govSQLUnitAdvice(facts)).To(Equal("A duration column holds seconds: 15 minutes is 900 seconds."))
			Expect(govSQLUserPrompt("How many calls lasted more than 15 minutes?", facts, []*govSQLView{views["v_cdr"]})).To(ContainSubstring("15 minutes is 900 seconds"))
			Expect(govSQLUnitAdvice(factsFor("How many data sessions used more than 5 MB?", "v_ipdr"))).To(Equal("A size column holds bytes: 5 MB is 5000000 bytes."))
			Expect(govSQLUnitAdvice(factsFor("How many calls lasted more than 30 seconds?", "v_cdr"))).To(BeEmpty())
			Expect(govSQLUnitAdvice(factsFor("Which numbers made more than 10 calls?", "v_cdr"))).To(BeEmpty())
			validated, _ := govSQLValidate("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE call_duration_seconds > 600", views)
			unmet := govSQLCheck(facts, validated)
			Expect(unmet).To(HaveLen(1))
			Expect(unmet[0].Message).To(ContainSubstring("15 minutes is 900 seconds"))
		})
	})

	Describe("a comparison is not a column", func() {
		It("does not read versus, vs or compared as the name of a field", func() {
			for _, q := range []string{
				"How many call records were there in May 2026 versus June 2026?",
				"How many call records were there in May 2026 vs June 2026?",
				"How many call records in May 2026 compared with June 2026?",
			} {
				facts := factsFor(q, "v_cdr")
				for _, field := range facts.Fields {
					Expect(field.Phrase).NotTo(BeElementOf("versus", "vs", "compared", "compare", "comparison"), q)
				}
				Expect(facts.WantsCompare).To(BeTrue(), q)
			}
		})

		It("passes a month-by-month comparison that does not use the direction field", func() {
			facts := factsFor("How many call records were there in May 2026 versus June 2026?", "v_cdr")
			Expect(kinds(facts, "SELECT to_char(event_time, 'YYYY-MM') AS month, COUNT(*) AS number_of_call_records FROM v_cdr WHERE event_time >= '2026-05-01' AND event_time < '2026-07-01' GROUP BY 1 ORDER BY 1")).To(BeEmpty())
		})
	})

	Describe("the line the lane writes for each response", func() {
		var buffer *bytes.Buffer
		BeforeEach(func() {
			buffer = &bytes.Buffer{}
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(buffer, nil)))
			DeferCleanup(func() { slog.SetDefault(previous) })
		})

		It("records the kinds of what was unmet once each, and nothing of the question, the query or a value", func() {
			audit := GovSQLAuditV1{State: "abstained", Attempts: 2, Views: []string{"v_cdr"}, ModelMS: 3100, TotalMS: 3120,
				SQL: "SELECT secret_column FROM v_cdr WHERE msisdn = '923001110001'"}
			audit.noteUnmet(govSQLObligation{Kind: "WINDOW", Subject: "between 11 pm and 3 am"}, govSQLObligation{Kind: "TIME"})
			audit.noteUnmet(govSQLObligation{Kind: "WINDOW", Subject: "923001110001"})
			Expect(audit.Unmet).To(Equal([]string{"WINDOW", "TIME"}))

			recorder := httptest.NewRecorder()
			setGovernedSQLHeader(recorder, audit)
			Expect(recorder.Header().Get("X-Governed-SQL")).To(Equal("state=abstained; attempts=2; views=v_cdr; model_ms=3100; exec_ms=0; total_ms=3120"))
			line := buffer.String()
			Expect(line).To(ContainSubstring("governed sql"))
			Expect(line).To(ContainSubstring("state=abstained"))
			Expect(line).To(ContainSubstring("attempts=2"))
			Expect(line).To(ContainSubstring("views=v_cdr"))
			Expect(line).To(ContainSubstring("unmet=WINDOW,TIME"))
			Expect(line).To(ContainSubstring("total_ms=3120"))
			Expect(line).NotTo(ContainSubstring("secret_column"))
			Expect(line).NotTo(ContainSubstring("923001110001"))
			Expect(line).NotTo(ContainSubstring("11 pm"))
		})

		It("writes a line for a decline too, with its reason", func() {
			setGovernedSQLHeader(httptest.NewRecorder(), GovSQLAuditV1{State: "declined", Reason: "no evidence family recognised in the question"})
			Expect(buffer.String()).To(ContainSubstring("state=declined"))
			Expect(buffer.String()).To(ContainSubstring("no evidence family recognised"))
		})

		It("writes nothing when the lane did not look at the request", func() {
			setGovernedSQLHeader(httptest.NewRecorder(), GovSQLAuditV1{})
			Expect(buffer.String()).To(BeEmpty())
		})
	})
})

// With a database: the whole loop, with a scripted model, on the questions of the fresh-seed check and the probes.
var _ = Describe("Governed SQL run 15 (with a database)", func() {
	var (
		ctx    context.Context
		db     *pgxpool.Pool
		model  *scriptedModel
		oracle func(sql string) string
	)
	BeforeEach(func() {
		dsn := os.Getenv("GOVSQL_TEST_DB") //nolint:forbidigo // tests gate live-database specs on an environment variable
		if dsn == "" {
			Skip("needs a live database; set GOVSQL_TEST_DB")
		}
		ctx = context.Background()
		var err error
		db, err = pgxpool.New(ctx, dsn)
		Expect(err).NotTo(HaveOccurred())
		model = &scriptedModel{}
		previous := govSQLModel
		govSQLModel = model.ask
		DeferCleanup(func() {
			govSQLModel = previous
			db.Close()
		})
		oracle = func(sql string) string {
			var out string
			Expect(db.QueryRow(ctx, "SELECT ("+sql+")::text").Scan(&out)).To(Succeed())
			return out
		}
	})

	const cdr = "FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type='cdr'"
	started := govSQLTestCaseClock("raw_payload->>'CALL_START_DT_TM'")
	ask := func(question string) (*hybridQueryResponse, GovSQLAuditV1) {
		req := hybridQueryRequest{TenantID: "default", CollectionID: "records-demo", Query: question, RequestClass: forensicrequest.GovernedAnalysis, SynthesisModel: "scripted"}
		return runGovernedSQLLane(ctx, config{}, db, req, timeNow())
	}
	headline := func(resp *hybridQueryResponse) string { return resp.Enterprise["executive_answer"].(string) }
	lastPrompt := func(call int) string {
		turns := model.calls[call]
		return turns[len(turns)-1].Content
	}

	Describe("a window across midnight", func() {
		const question = "How many calls were made between 11 pm and 3 am?"
		var want string
		BeforeEach(func() {
			want = govSQLFormatNumber(oracle("SELECT count(*) " + cdr + " AND extract(hour from " + started + ") IN (23, 0, 1, 2)"))
			Expect(want).NotTo(Equal("0"))
		})

		It("is told the hours in the first message, and repairs a BETWEEN that keeps nothing", func() {
			model.replies = []string{
				sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE EXTRACT(HOUR FROM event_time) BETWEEN 23 AND 3"),
				sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE EXTRACT(HOUR FROM event_time) >= 23 OR EXTRACT(HOUR FROM event_time) < 3"),
			}
			resp, audit := ask(question)
			Expect(resp).NotTo(BeNil(), audit.Reason)
			Expect(audit.State).To(Equal("answered"))
			Expect(audit.Attempts).To(Equal(2))
			Expect(audit.Unmet).To(Equal([]string{"WINDOW"}))
			Expect(headline(resp)).To(Equal("Number of calls: " + want + "."))
			Expect(model.calls[0][len(model.calls[0])-1].Content).To(ContainSubstring("hours 23, 0, 1 and 2"))
			Expect(lastPrompt(1)).To(ContainSubstring("keeps the hours none"))
			Expect(resp.Enterprise["limitations"]).To(ContainElement(ContainSubstring("hours: 23:00 up to 03:00")))
		})

		It("answers a window written correctly at once, in the form of a time", func() {
			model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE event_time::time >= '23:00' OR event_time::time < '03:00'")}
			resp, audit := ask(question)
			Expect(audit.State).To(Equal("answered"), audit.Reason)
			Expect(audit.Attempts).To(Equal(1))
			Expect(headline(resp)).To(Equal("Number of calls: " + want + "."))
		})

		It("abstains, with no number, when the model keeps the hour after the window too", func() {
			model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE EXTRACT(HOUR FROM event_time) >= 23 OR EXTRACT(HOUR FROM event_time) <= 3")}
			resp, audit := ask(question)
			Expect(resp).NotTo(BeNil())
			Expect(audit.State).To(Equal("abstained"))
			text := resp.Answer["clarification"].(string)
			Expect(text).To(ContainSubstring("did not keep exactly the hours of the window the question gives"))
			Expect(text).NotTo(ContainSubstring("Number of calls"))
		})
	})

	Describe("weekends", func() {
		const question = "How many calls were made on weekends?"
		It("repairs Friday and Saturday into Saturday and Sunday, and the count is the key's", func() {
			want := govSQLFormatNumber(oracle("SELECT count(*) " + cdr + " AND extract(isodow from " + started + ") IN (6, 7)"))
			Expect(want).NotTo(Equal("0"))
			friSat := govSQLFormatNumber(oracle("SELECT count(*) " + cdr + " AND extract(isodow from " + started + ") IN (5, 6)"))
			Expect(friSat).NotTo(Equal(want))
			model.replies = []string{
				sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE EXTRACT(ISODOW FROM event_time) IN (5, 6)"),
				sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE EXTRACT(ISODOW FROM event_time) IN (6, 7)"),
			}
			resp, audit := ask(question)
			Expect(resp).NotTo(BeNil(), audit.Reason)
			Expect(audit.State).To(Equal("answered"))
			Expect(audit.Attempts).To(Equal(2))
			Expect(headline(resp)).To(Equal("Number of calls: " + want + "."))
			Expect(lastPrompt(1)).To(ContainSubstring("The query keeps Friday and Saturday."))
			Expect(resp.Enterprise["limitations"]).To(ContainElement(ContainSubstring("days: Saturday and Sunday")))
		})
	})

	Describe("the highest value", func() {
		It("repairs an average given for the longest call", func() {
			want := govSQLFormatNumber(oracle("SELECT max(extract(epoch from (raw_payload->>'CALL_END_DT_TM')::timestamptz - (raw_payload->>'CALL_START_DT_TM')::timestamptz)) " + cdr))
			model.replies = []string{
				sqlReply("SELECT AVG(call_duration_seconds) AS longest_call FROM v_cdr"),
				sqlReply("SELECT MAX(call_duration_seconds) AS longest_call FROM v_cdr"),
			}
			resp, audit := ask("What is the longest call duration?")
			Expect(resp).NotTo(BeNil(), audit.Reason)
			Expect(audit.State).To(Equal("answered"))
			Expect(audit.Attempts).To(Equal(2))
			Expect(audit.Unmet).To(Equal([]string{"AGGREGATE"}))
			Expect(headline(resp)).To(ContainSubstring(want))
		})
	})

	Describe("which day", func() {
		It("repairs the day of the month into the date", func() {
			want := oracle("SELECT to_char(d, 'YYYY-MM-DD') FROM (SELECT (" + started + ")::date AS d, count(*) AS c " + cdr + " AND raw_payload->>'MSISDN'='923001110001' GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT 1) t")
			model.replies = []string{
				sqlReply("SELECT EXTRACT(DAY FROM event_time) AS day, COUNT(*) AS number_of_calls FROM v_cdr WHERE msisdn = '923001110001' GROUP BY 1 ORDER BY 2 DESC LIMIT 1"),
				sqlReply("SELECT event_time::date AS day, COUNT(*) AS number_of_calls FROM v_cdr WHERE msisdn = '923001110001' GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT 1"),
			}
			resp, audit := ask("On which day does 923001110001 have the most call records?")
			Expect(resp).NotTo(BeNil(), audit.Reason)
			Expect(audit.State).To(Equal("answered"))
			Expect(audit.Attempts).To(Equal(2))
			Expect(audit.Unmet).To(Equal([]string{"DATE"}))
			Expect(headline(resp)).To(ContainSubstring(want))
		})
	})

	Describe("a threshold in minutes", func() {
		It("takes 900 seconds for 15 minutes at once, and the count is the key's", func() {
			want := govSQLFormatNumber(oracle("SELECT count(*) " + cdr + " AND extract(epoch from (raw_payload->>'CALL_END_DT_TM')::timestamptz - (raw_payload->>'CALL_START_DT_TM')::timestamptz) > 900"))
			model.replies = []string{sqlReply("SELECT COUNT(*) AS number_of_calls FROM v_cdr WHERE call_duration_seconds > 900")}
			resp, audit := ask("How many calls lasted more than 15 minutes?")
			Expect(audit.State).To(Equal("answered"), audit.Reason)
			Expect(audit.Attempts).To(Equal(1))
			Expect(headline(resp)).To(Equal("Number of calls: " + want + "."))
			Expect(model.calls[0][len(model.calls[0])-1].Content).To(ContainSubstring("15 minutes is 900 seconds"))
		})
	})

	Describe("two months compared", func() {
		It("is not asked for the direction field, and returns one row per month", func() {
			model.replies = []string{sqlReply("SELECT to_char(event_time, 'YYYY-MM') AS month, COUNT(*) AS number_of_call_records FROM v_cdr WHERE event_time >= '2026-04-01' AND event_time < '2026-08-01' GROUP BY 1 ORDER BY 1")}
			resp, audit := ask("How many call records were there in April 2026 versus July 2026?")
			Expect(resp).NotTo(BeNil(), audit.Reason)
			Expect(audit.State).To(Equal("answered"))
			Expect(audit.Attempts).To(Equal(1))
			april := govSQLFormatNumber(oracle("SELECT count(*) " + cdr + " AND to_char(" + started + ", 'YYYY-MM') = '2026-04'"))
			july := govSQLFormatNumber(oracle("SELECT count(*) " + cdr + " AND to_char(" + started + ", 'YYYY-MM') = '2026-07'"))
			text := headline(resp)
			Expect(text).To(ContainSubstring(april))
			Expect(text).To(ContainSubstring(july))
		})
	})
})
