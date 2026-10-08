package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// THE CASE CLOCK, tested.
//
// The defect these specs hold in place (reports/governed-sql-20261005/PREREGISTRATION.md, run 10,
// the correction): the lane counted "night" on the stored UTC instant while saying it used the hours
// as recorded at the source. For CDR, whose source times are Pakistan local time, that was a different
// six hours of the day (1,323 against the true 1,283), and the same shift explained the demo case's
// "June" gap. Every expectation below is computed by an INDEPENDENT oracle over the raw payload text,
// not by the lane's own session setting.

// govSQLTestCaseClock returns SQL for the case clock of a raw text time, read the way the ingest reads
// it: a time with an offset (Z, +05:00, -0500) is converted to the case zone; a time without one is
// already the source's local clock, which the ingest reads in that same zone. The result is local
// wall-clock time, so the hour, day and month of it are what an investigator means.
func govSQLTestCaseClock(rawText string) string {
	return "(CASE WHEN " + rawText + " ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}[T ][0-9]{2}:[0-9]{2}.*(Z|[+-][0-9]{2}(:?[0-9]{2})?)$' " +
		"THEN ((" + rawText + ")::timestamptz AT TIME ZONE 'Asia/Karachi') ELSE (" + rawText + ")::timestamp END)"
}

var _ = Describe("Governed SQL case clock", func() {
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

	Describe("the zone setting", func() {
		sample := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)

		It("is Asia/Karachi unless the environment names another", func() {
			GinkgoT().Setenv(govSQLTimeZoneEnv, "")
			name, loc := govSQLTimeZone()
			Expect(name).To(Equal("Asia/Karachi"))
			Expect(govSQLZoneOffset(loc, sample)).To(Equal("UTC+05:00"))
		})

		It("takes a known IANA zone from the environment", func() {
			GinkgoT().Setenv(govSQLTimeZoneEnv, "Europe/London")
			name, loc := govSQLTimeZone()
			Expect(name).To(Equal("Europe/London"))
			Expect(govSQLZoneOffset(loc, sample)).To(Equal("UTC+01:00"), "British summer time")
		})

		It("does not guess at an unknown zone, at Go's \"Local\", or at a path", func() {
			for _, bad := range []string{"Mars/Olympus_Mons", "Local", "../etc/localtime"} {
				GinkgoT().Setenv(govSQLTimeZoneEnv, bad)
				name, _ := govSQLTimeZone()
				Expect(name).To(Equal("Asia/Karachi"), bad)
			}
		})
	})

	Describe("what the lane shows", func() {
		karachi, err := time.LoadLocation("Asia/Karachi")
		It("loads the zone it formats in", func() { Expect(err).NotTo(HaveOccurred()) })

		It("shows a timestamptz on the case clock with the zone's own abbreviation", func() {
			at := time.Date(2026, 7, 14, 20, 30, 0, 0, time.UTC) // 01:30 on the 15th in Karachi
			Expect(govSQLCellValue(at, pgtype.TimestamptzOID, karachi)).To(Equal("2026-07-15 01:30:00 PKT"))
		})

		It("shows a date as the date: the driver hands it back as midnight UTC and a zone must not move it", func() {
			day := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
			Expect(govSQLCellValue(day, pgtype.DateOID, karachi)).To(Equal("2026-07-15"))
		})

		It("shows a timestamp without a zone as the wall clock it already is", func() {
			wall := time.Date(2026, 7, 15, 1, 30, 0, 0, time.UTC)
			Expect(govSQLCellValue(wall, pgtype.TimestampOID, karachi)).To(Equal("2026-07-15 01:30:00"))
		})

		It("leaves every other value as it always was", func() {
			Expect(govSQLCellValue(int64(7), pgtype.Int8OID, karachi)).To(Equal(int64(7)))
			Expect(govSQLCellValue("PK-1", pgtype.TextOID, karachi)).To(Equal("PK-1"))
			Expect(govSQLCellValue(nil, pgtype.TextOID, karachi)).To(BeNil())
			// A time of a type this code does not know keeps the old, explicit UTC rendering.
			Expect(govSQLCellValue(time.Date(2026, 7, 14, 20, 30, 0, 0, time.UTC), 0, karachi)).To(Equal("2026-07-14T20:30:00Z"))
		})
	})

	Describe("the view definition", func() {
		req := hybridQueryRequest{TenantID: "default", CollectionID: "records-demo"}

		It("makes a time that is not a recording NULL, for the record-time column only", func() {
			cte, _, err := govSQLCTE(views["v_cdr"], req, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(cte).To(ContainSubstring("jsonb_exists(r.metadata, 'derived_from_media')"))
			Expect(cte).To(ContainSubstring("jsonb_exists(r.metadata, 'timestamp_fallback')"))
			Expect(cte).To(ContainSubstring("r.timestamp IS NOT DISTINCT FROM r.ingested_at THEN NULL ELSE r.timestamp END) AS event_time"))
			// call_start and call_end are read from their own text, which has no ingest-time fallback.
			Expect(strings.Count(cte, "jsonb_exists(r.metadata, 'derived_from_media')")).To(Equal(1))
		})

		It("leaves a derived-media view alone: it has no records row and no ingest time to confuse", func() {
			for _, view := range views {
				if !view.binding.Derived {
					continue
				}
				cte, _, err := govSQLCTE(view, req, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(cte).NotTo(ContainSubstring("jsonb_exists(r.metadata"), view.Name)
			}
		})
	})

	Describe("the verifier and the prompt", func() {
		unmetKinds := func(question, sql string) []string {
			facts := govSQLExtractFacts(question, []*govSQLView{views["v_ipdr"]}, all)
			validated, rejection := govSQLValidate(sql, views)
			Expect(rejection).To(BeNil(), "validator rejected: %v", rejection)
			kinds := []string{}
			for _, item := range govSQLCheck(facts, validated) {
				kinds = append(kinds, item.Kind)
			}
			return kinds
		}
		const night = "How many data sessions were there at night?"

		It("flags a conversion to another zone: it would count a different window than the case clock", func() {
			Expect(unmetKinds(night, "SELECT COUNT(*) AS sessions FROM v_ipdr WHERE EXTRACT(HOUR FROM event_time AT TIME ZONE 'UTC') BETWEEN 0 AND 5")).To(ContainElement("TIME_ZONE"))
			Expect(unmetKinds(night, "SELECT COUNT(*) AS sessions FROM v_ipdr WHERE EXTRACT(HOUR FROM event_time AT TIME ZONE 'Europe/London') BETWEEN 0 AND 5")).To(ContainElement("TIME_ZONE"))
		})

		It("allows no conversion, and a redundant conversion to the case zone itself", func() {
			Expect(unmetKinds(night, "SELECT COUNT(*) AS sessions FROM v_ipdr WHERE EXTRACT(HOUR FROM event_time) BETWEEN 0 AND 5")).NotTo(ContainElement("TIME_ZONE"))
			Expect(unmetKinds(night, "SELECT COUNT(*) AS sessions FROM v_ipdr WHERE EXTRACT(HOUR FROM event_time AT TIME ZONE 'Asia/Karachi') BETWEEN 0 AND 5")).NotTo(ContainElement("TIME_ZONE"))
		})

		It("follows a different case zone from the environment", func() {
			GinkgoT().Setenv(govSQLTimeZoneEnv, "Europe/London")
			Expect(unmetKinds(night, "SELECT COUNT(*) AS sessions FROM v_ipdr WHERE EXTRACT(HOUR FROM event_time AT TIME ZONE 'Asia/Karachi') BETWEEN 0 AND 5")).To(ContainElement("TIME_ZONE"))
		})

		It("tells a time question, in its own hint, that times are already on the case clock", func() {
			// The shared rules stay as they were (governed_sql_timecolumn_ginkgo_test.go pins them); only a
			// question with a time cue is told.
			Expect(govSQLRules).NotTo(ContainSubstring("case clock"))
			offered := []*govSQLView{views["v_ipdr"]}
			user := govSQLUserPrompt(night, govSQLExtractFacts(night, offered, all), offered)
			Expect(user).To(ContainSubstring("Times are on the case clock"))
			Expect(user).To(ContainSubstring("never convert time zones"))
		})
	})

	Describe("what the answer says about the clock", func() {
		answer := func(question, sql string, columns []string) *hybridQueryResponse {
			view := views["v_ipdr"]
			facts := govSQLExtractFacts(question, []*govSQLView{view}, all)
			validated := &govSQLValidated{SQL: sql, View: view, Columns: columns}
			result := &govSQLResult{Columns: []string{"n"}, Rows: [][]any{{int64(744)}}}
			return govSQLAnswerResponse(hybridQueryRequest{TenantID: "default", CollectionID: "c"}, validated, result, facts, "", GovSQLAuditV1{})
		}

		It("names the clock a time of day was read on, and what night means on it", func() {
			resp := answer("How many data sessions were there at night?", "SELECT COUNT(*) AS n FROM v_ipdr WHERE EXTRACT(HOUR FROM event_time) BETWEEN 0 AND 5", []string{"event_time"})
			Expect(resp.Enterprise["limitations"]).To(ContainElement("A time of day is read on the case clock (Asia/Karachi, UTC+05:00); 'night' was taken as 00:00 to 05:59."))
			Expect(resp.Enterprise["limitations"]).NotTo(ContainElement(ContainSubstring("as recorded at the source")))
		})

		It("names the clock when a time is shown even though no time condition was asked", func() {
			resp := answer("When was the first data session?", "SELECT MIN(event_time) AS n FROM v_ipdr", []string{"event_time"})
			Expect(resp.Enterprise["limitations"]).To(ContainElement("Times are read and shown on the case clock (Asia/Karachi, UTC+05:00)."))
		})

		It("says nothing about a clock when the answer involves no time", func() {
			resp := answer("How many data sessions are there?", "SELECT COUNT(*) AS n FROM v_ipdr", nil)
			Expect(resp.Enterprise["limitations"]).NotTo(ContainElement(ContainSubstring("case clock")))
		})
	})
})

// With a database (GOVSQL_TEST_DB, see governed_sql_exec_ginkgo_test.go). The stand-in is loaded by
// evaluation/question_factory/load_replica.py, which reads a time without an offset in the source zone
// (as the real ingest does) and gives a row with no time of its own the ingest time, so its stored
// timestamps mean what production's mean. These specs only read.
var _ = Describe("Governed SQL case clock (with a database)", func() {
	var (
		ctx   context.Context
		db    *pgxpool.Pool
		views map[string]*govSQLView
		req   hybridQueryRequest
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
		req = hybridQueryRequest{TenantID: "default", CollectionID: "records-demo"}
		views = govSQLTestViews()
	})
	AfterEach(func() {
		if db != nil {
			db.Close()
		}
	})

	const ipdr = "FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type='ipdr'"
	const cdr = "FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type='cdr'"
	run := func(sql string) *govSQLResult {
		validated, rejection := govSQLValidate(sql, views)
		Expect(rejection).To(BeNil(), "validator rejected: %v", rejection)
		result, err := govSQLExecute(ctx, db, req, validated, 0, 0)
		Expect(err).NotTo(HaveOccurred())
		return result
	}
	oracle := func(sql string) string {
		var out string
		Expect(db.QueryRow(ctx, "SELECT ("+sql+")::text").Scan(&out)).To(Succeed())
		return out
	}
	scalar := func(result *govSQLResult) string {
		Expect(result.Rows).To(HaveLen(1))
		Expect(result.Rows[0]).To(HaveLen(1))
		return strings.TrimSpace(toText(result.Rows[0][0]))
	}

	It("counts the hour on the case clock, as an explicit conversion over the raw text does", func() {
		got := scalar(run("SELECT COUNT(*) AS n FROM v_ipdr WHERE EXTRACT(HOUR FROM event_time) BETWEEN 0 AND 5"))
		want := oracle("SELECT count(*) " + ipdr + " AND extract(hour from " + govSQLTestCaseClock("raw_payload->>'timestamp'") + ") BETWEEN 0 AND 5")
		Expect(got).To(Equal(want))
		// And it is a different window than the same hours of the stored UTC instant, the defect this removes.
		utc := oracle("SELECT count(*) " + ipdr + " AND extract(hour from (\"timestamp\" AT TIME ZONE 'UTC')) BETWEEN 0 AND 5")
		Expect(got).NotTo(Equal(utc), "if these ever agree the data no longer shows the difference between the clocks")
	})

	It("counts the hour of a CDR on the case clock: the source clock is local and the stored instant is 5 hours behind it", func() {
		got := scalar(run("SELECT COUNT(*) AS n FROM v_cdr WHERE EXTRACT(HOUR FROM event_time) BETWEEN 0 AND 5"))
		want := oracle("SELECT count(*) " + cdr + " AND extract(hour from " + govSQLTestCaseClock("raw_payload->>'CALL_START_DT_TM'") + ") BETWEEN 0 AND 5")
		Expect(got).To(Equal(want))
	})

	It("reads a calendar day on the case clock", func() {
		got := scalar(run("SELECT COUNT(*) AS n FROM v_ipdr WHERE event_time >= '2026-07-11' AND event_time < '2026-07-12'"))
		clock := govSQLTestCaseClock("raw_payload->>'timestamp'")
		want := oracle("SELECT count(*) " + ipdr + " AND " + clock + " >= '2026-07-11' AND " + clock + " < '2026-07-12'")
		Expect(got).To(Equal(want))
	})

	It("reads a time written without an offset in the source zone, so a call start is the clock on the page", func() {
		got := scalar(run("SELECT COUNT(*) AS n FROM v_cdr WHERE call_start >= '2026-04-02 00:00:00' AND call_start < '2026-04-03 00:00:00'"))
		want := oracle("SELECT count(*) " + cdr + " AND " + govSQLTestCaseClock("raw_payload->>'CALL_START_DT_TM'") + " >= '2026-04-02' AND " + govSQLTestCaseClock("raw_payload->>'CALL_START_DT_TM'") + " < '2026-04-03'")
		Expect(got).To(Equal(want))
	})

	It("shows a time on the case clock with the zone's name", func() {
		result := run("SELECT MIN(event_time) AS first_session FROM v_ipdr")
		want := oracle("SELECT to_char(min(" + govSQLTestCaseClock("raw_payload->>'timestamp'") + "), 'YYYY-MM-DD HH24:MI:SS') " + ipdr)
		Expect(scalar(result)).To(Equal(want + " PKT"))
	})

	It("shows a date as the date it is on the case clock, never shifted", func() {
		result := run("SELECT MIN(event_time)::date AS first_day FROM v_ipdr")
		want := oracle("SELECT to_char(min(" + govSQLTestCaseClock("raw_payload->>'timestamp'") + ")::date, 'YYYY-MM-DD') " + ipdr)
		Expect(scalar(result)).To(Equal(want))
	})

	It("keeps a record with no time of its own out of a time condition, and still counts it where no time is named", func() {
		// The view's own expression, fed rows by VALUES: nothing is written. Row 1 has a recorded time;
		// row 2 holds its ingest time (the fallback, by the structural signature); row 3 is marked
		// derived-from-media; row 4 is marked as a timestamp fallback.
		sql := `SELECT count(` + govSQLRecordedTime("r.timestamp") + `)::text || '/' || count(*)::text FROM (VALUES
		  ('2026-07-14 20:30:00+00'::timestamptz, '2026-08-01 10:00:00+00'::timestamptz, '{}'::jsonb),
		  ('2026-08-01 10:00:00+00'::timestamptz, '2026-08-01 10:00:00+00'::timestamptz, '{}'::jsonb),
		  ('2026-07-15 01:00:00+00'::timestamptz, '2026-08-01 10:00:00+00'::timestamptz, '{"derived_from_media": true}'::jsonb),
		  ('2026-07-15 02:00:00+00'::timestamptz, '2026-08-01 10:00:00+00'::timestamptz, '{"timestamp_fallback": "ingested_at"}'::jsonb)
		) AS r("timestamp", ingested_at, metadata)`
		var out string
		Expect(db.QueryRow(ctx, sql).Scan(&out)).To(Succeed())
		Expect(out).To(Equal("1/4"), "one recorded time among four rows; all four still count")
	})
})
