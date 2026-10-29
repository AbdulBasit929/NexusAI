package main

import (
	"github.com/jackc/pgx/v5/pgconn"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A model-written query that Postgres refuses for its form (an ORDER BY a column that SELECT DISTINCT does not list, a parameter of no type) is a mistake the model can repair, so it is reported as
// a query error and retried; found by the subscribers run on development case 2 (2026-10-29), where it ended the question as a database failure.
var _ = Describe("Database errors of a model-written query", func() {
	DescribeTable("the form errors can be repaired and a failure of the database cannot",
		func(code, want string) {
			err := govSQLMapError(&pgconn.PgError{Code: code, Message: "for SELECT DISTINCT, ORDER BY expressions must appear in select list"})
			run, ok := err.(*govSQLRunError)
			Expect(ok).To(BeTrue())
			Expect(run.Code).To(Equal(want))
			if code == "42P10" {
				Expect(run.Message).To(ContainSubstring("select that column as well, or remove the ORDER BY"))
			}
		},
		Entry("an ORDER BY that SELECT DISTINCT does not list", "42P10", "QUERY_ERROR"),
		Entry("a parameter of no type", "42P18", "QUERY_ERROR"),
		Entry("an unknown column", "42703", "QUERY_ERROR"),
		Entry("a statement timeout", "57014", "TIMEOUT"),
		Entry("a disk failure", "53100", "DATABASE"))
})
