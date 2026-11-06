// Package caselayer is the server-side case layer of NexusAI: conversations with history (threads and turns that run the Ask engine, re-run and compare), knowledge capture
// (notes, findings, hypotheses, tasks, tags, saved views, annotations), case members and authority records, and the hash-chained audit trail that records every change.
//
// It imports nothing from the API's package main. The API hands it a Scope (who is asking), an Engine (how a question is answered) and a database pool, and registers its routes.
// Design: docs/architecture/CONVERSATIONS_AND_CASE_LAYER_20261103.md. Contract: docs/work/API_CONTRACT_CONVERSATIONS_20261103.md. Tables: db/forensic_records/034_conversations_and_case_layer.sql.
package caselayer

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Error is a failure that has an HTTP status and a sentence for the analyst. Everything the package returns to a caller is an *Error or wraps one.
type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"error"`
	Cause   error  `json:"-"`
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }

func newError(status int, code, format string, args ...any) *Error {
	return &Error{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

// BadRequest is a request the server understood and refuses, with the reason.
func BadRequest(format string, args ...any) *Error {
	return newError(http.StatusBadRequest, "invalid_request", format, args...)
}

// NotFound is also what a caller gets for something they may not see: the layer never confirms that a hidden or foreign object exists.
func NotFound(format string, args ...any) *Error {
	return newError(http.StatusNotFound, "not_found", format, args...)
}

func Forbidden(format string, args ...any) *Error {
	return newError(http.StatusForbidden, "forbidden", format, args...)
}

func Conflict(code, format string, args ...any) *Error {
	return newError(http.StatusConflict, code, format, args...)
}

func Unavailable(format string, args ...any) *Error {
	return newError(http.StatusServiceUnavailable, "unavailable", format, args...)
}

// asError turns any error into an *Error: a typed one stays as it is, a database error is mapped to a sentence, anything else is an internal error whose text is not shown.
func asError(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return &Error{Status: http.StatusNotFound, Code: "not_found", Message: "There is no such object, or you may not see it.", Cause: err}
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			return &Error{Status: http.StatusConflict, Code: "duplicate", Message: "That already exists.", Cause: err}
		case "23514", "22001", "22P02", "22007", "22008":
			return &Error{Status: http.StatusBadRequest, Code: "invalid_request", Message: "A value is outside what is allowed (" + pg.ConstraintName + ").", Cause: err}
		case "23503":
			return &Error{Status: http.StatusBadRequest, Code: "invalid_request", Message: "It refers to something that does not exist in this case.", Cause: err}
		case "42501":
			return &Error{Status: http.StatusForbidden, Code: "forbidden", Message: "You do not have permission to do that in this case.", Cause: err}
		case "55000":
			return &Error{Status: http.StatusConflict, Code: "immutable", Message: "A finished record cannot be changed; make a new one instead.", Cause: err}
		case "57014":
			return &Error{Status: http.StatusServiceUnavailable, Code: "unavailable", Message: "The request took too long and was stopped; narrow it and try again.", Cause: err}
		case "40001", "40P01":
			return &Error{Status: http.StatusConflict, Code: "retry", Message: "Another change was made at the same moment; try again.", Cause: err}
		case "42P01":
			return &Error{Status: http.StatusServiceUnavailable, Code: "not_installed", Message: "The case layer is not installed in this database (migration 034).", Cause: err}
		}
	}
	return &Error{Status: http.StatusInternalServerError, Code: "internal", Message: "The request could not be completed.", Cause: err}
}
