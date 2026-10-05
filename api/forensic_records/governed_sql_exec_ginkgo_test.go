package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// These specs run the governed SQL lane's executor against a real PostgreSQL holding
// the demo records. They need GOVSQL_TEST_DB (a DSN); without it they are skipped,
// so CI that has no database is unaffected.
//
//	GOVSQL_TEST_DB='postgresql://postgres@localhost/nexusai_replica?host=/var/run/postgresql' \
//	  go test ./api/forensic_records/ -run TestForensicRecordsSynthesis -ginkgo.focus "Governed SQL execution"
//
// Every expectation is computed by a DIFFERENT query, over the raw payload keys, so
// the lane is checked against an independent oracle and not against itself.
var _ = Describe("Governed SQL execution", func() {
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

	run := func(sql string, scope hybridQueryRequest) (*govSQLResult, error) {
		validated, rejection := govSQLValidate(sql, views)
		Expect(rejection).To(BeNil(), "validator rejected: %v", rejection)
		return govSQLExecute(ctx, db, scope, validated, 0, 0)
	}
	oracle := func(sql string, args ...any) string {
		var out string
		Expect(db.QueryRow(ctx, "SELECT ("+sql+")::text", args...).Scan(&out)).To(Succeed())
		return out
	}
	scalar := func(result *govSQLResult) string {
		Expect(result.Rows).To(HaveLen(1))
		Expect(result.Rows[0]).To(HaveLen(1))
		return strings.TrimSpace(toText(result.Rows[0][0]))
	}

	It("counts a view exactly as an independent query over the raw rows does", func() {
		result, err := run("SELECT COUNT(*) AS total FROM v_cdr", req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Columns).To(Equal([]string{"total"}))
		Expect(scalar(result)).To(Equal(oracle("SELECT count(*) FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type='cdr'")))
	})

	It("filters on a declared value and on a literal the way the raw rows do", func() {
		result, err := run("SELECT COUNT(*) FROM v_cdr WHERE call_type = 'GPRS' AND msisdn = '923001110001'", req)
		Expect(err).NotTo(HaveOccurred())
		Expect(scalar(result)).To(Equal(oracle("SELECT count(*) FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type='cdr' AND raw_payload->>'CALL_TYPE'='GPRS' AND raw_payload->>'MSISDN'='923001110001'")))
	})

	It("answers earliest and latest from typed timestamps, not text order", func() {
		result, err := run("SELECT MIN(call_start) AS earliest, MAX(call_start) AS latest FROM v_cdr", req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Rows).To(HaveLen(1))
		earliest := oracle("SELECT min((raw_payload->>'CALL_START_DT_TM')::timestamptz) AT TIME ZONE 'UTC' FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type='cdr'")
		Expect(strings.ReplaceAll(toText(result.Rows[0][0]), "T", " ")).To(HavePrefix(strings.TrimSuffix(earliest, "+00")[:10]))
	})

	It("groups like the raw rows do", func() {
		result, err := run("SELECT call_type, COUNT(*) AS calls FROM v_cdr GROUP BY call_type ORDER BY call_type", req)
		Expect(err).NotTo(HaveOccurred())
		got := map[string]string{}
		for _, row := range result.Rows {
			got[toText(row[0])] = toText(row[1])
		}
		rows, err := db.Query(ctx, "SELECT raw_payload->>'CALL_TYPE', count(*)::text FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type='cdr' GROUP BY 1")
		Expect(err).NotTo(HaveOccurred())
		defer rows.Close()
		want := map[string]string{}
		for rows.Next() {
			var key, count string
			Expect(rows.Scan(&key, &count)).To(Succeed())
			want[key] = count
		}
		Expect(got).To(Equal(want))
	})

	It("computes the curated duration metric the way the raw timestamps do", func() {
		result, err := run("SELECT MAX(call_duration_seconds) AS longest FROM v_cdr", req)
		Expect(err).NotTo(HaveOccurred())
		want := oracle("SELECT max(extract(epoch from (raw_payload->>'CALL_END_DT_TM')::timestamptz - (raw_payload->>'CALL_START_DT_TM')::timestamptz)) FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type='cdr'")
		Expect(strings.TrimRight(strings.TrimRight(scalar(result), "0"), ".")).To(Equal(strings.TrimRight(strings.TrimRight(want, "0"), ".")))
	})

	It("reads each family from its own view", func() {
		subscribers, err := run("SELECT COUNT(*) FROM v_subscriber WHERE status = 'active'", req)
		Expect(err).NotTo(HaveOccurred())
		Expect(scalar(subscribers)).To(Equal(oracle("SELECT count(*) FROM forensic.records WHERE tenant_id='default' AND collection_id='records-demo' AND record_type='subscriber' AND lower(raw_payload->>'status')='active'")))
	})

	It("cannot see another case or another tenant", func() {
		for _, scope := range []hybridQueryRequest{
			{TenantID: "default", CollectionID: "some-other-case"},
			{TenantID: "another-tenant", CollectionID: "records-demo"},
		} {
			result, err := run("SELECT COUNT(*) FROM v_cdr", scope)
			Expect(err).NotTo(HaveOccurred())
			Expect(scalar(result)).To(Equal("0"), "scope %+v leaked rows", scope)
		}
	})

	It("caps the rows it returns and says so", func() {
		result, err := run("SELECT msisdn, call_start FROM v_cdr", req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Rows).To(HaveLen(govSQLMaxRows))
		Expect(result.Truncated).To(BeTrue())
	})

	It("turns a runtime SQL mistake into a message the model can correct itself from", func() {
		_, err := run("SELECT CAST(location AS integer) FROM v_cdr", req)
		var failure *govSQLRunError
		Expect(err).To(HaveOccurred())
		Expect(err).To(BeAssignableToTypeOf(failure))
		Expect(err.(*govSQLRunError).Code).To(Equal("QUERY_ERROR"))
		Expect(err.(*govSQLRunError).Message).To(ContainSubstring("integer"))
	})

	It("refuses a query that was never validated", func() {
		_, err := govSQLExecute(ctx, db, req, nil, 0, 0)
		Expect(err).To(HaveOccurred())
	})

	Describe("the database walls, attacked directly with SQL the validator would never pass", func() {
		It("is read-only: a write fails inside the database", func() {
			_, err := govSQLRun(ctx, db, "default", "DELETE FROM forensic.records", nil, 10, 0)
			Expect(err).To(HaveOccurred())
			Expect(err.(*govSQLRunError).Code).To(Equal("READ_ONLY"))
			Expect(oracle("SELECT count(*) FROM forensic.records")).NotTo(Equal("0"))
		})
		It("is read-only for DDL as well", func() {
			_, err := govSQLRun(ctx, db, "default", "CREATE TABLE forensic.should_not_exist (x int)", nil, 10, 0)
			Expect(err).To(HaveOccurred())
			Expect(oracle("SELECT to_regclass('forensic.should_not_exist') IS NULL")).To(Equal("true"))
		})
		It("prices a runaway query before running it", func() {
			started := time.Now()
			_, err := govSQLRun(ctx, db, "default", "SELECT count(*) FROM forensic.records a, forensic.records b, forensic.records c, forensic.records d", nil, 10, 0)
			Expect(err).To(HaveOccurred())
			Expect(err.(*govSQLRunError).Code).To(Equal("TOO_EXPENSIVE"))
			Expect(time.Since(started)).To(BeNumerically("<", 5*time.Second), "the planner refused it without running it")
		})
		It("stops a query that runs past the timeout", func() {
			_, err := govSQLRun(ctx, db, "default", "SELECT count(*) FROM forensic.records a, forensic.records b WHERE a.raw_payload::text ~ b.raw_payload::text", nil, 10, 300*time.Millisecond)
			Expect(err).To(HaveOccurred())
			code := err.(*govSQLRunError).Code
			Expect(code).To(BeElementOf("TIMEOUT", "TOO_EXPENSIVE"))
		})
	})
})

func toText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		return strings.TrimSpace(strings.Trim(strings.TrimSpace(sprintAny(v)), "\""))
	}
}

func sprintAny(value any) string { return strings.TrimSpace(fmt.Sprint(value)) }
