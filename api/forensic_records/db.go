package main

import (
	"context"
	"regexp"
	"strings"
)

type requestIDContextKey struct{}

func contextWithRequestID(ctx context.Context, requestID string) context.Context {
	requestID = sanitizeRequestID(requestID)
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return sanitizeRequestID(value)
}

func sqlWithRequestID(ctx context.Context, query string) string {
	requestID := requestIDFromContext(ctx)
	if requestID == "" {
		return query
	}
	return "/* request_id: " + requestID + " */\n" + query
}

func sanitizeRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return regexp.MustCompile(`[^a-zA-Z0-9_.:-]`).ReplaceAllString(value, "")
}
