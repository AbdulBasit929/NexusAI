package main

type TimeRange struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

type RecordFamilyCoverage struct {
	RecordType string `json:"record_type"`
	Count      int64  `json:"count"`
}

type NearestActivity struct {
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

type CoverageSummary struct {
	CollectionMinTimestamp string                 `json:"collection_min_timestamp,omitempty"`
	CollectionMaxTimestamp string                 `json:"collection_max_timestamp,omitempty"`
	TotalIndexedRecords    int64                  `json:"total_indexed_records"`
	RecordFamiliesPresent  []RecordFamilyCoverage `json:"record_families_present"`
	NearestActivity        NearestActivity        `json:"nearest_activity,omitempty"`
	ValidTargetExamples    []string               `json:"valid_target_examples"`
}

type QueryPlan struct {
	InterpretedIntent string          `json:"interpreted_intent"`
	SelectedTemplate  string          `json:"selected_template"`
	TargetIdentifiers []string        `json:"target_identifiers"`
	DateBounds        TimeRange       `json:"date_bounds"`
	AppliedFilters    map[string]any  `json:"applied_filters"`
	Confidence        float64         `json:"confidence"`
	CoverageCheck     CoverageSummary `json:"coverage_check"`
	ExecutionStrategy string          `json:"execution_strategy"`
	FallbackReason    string          `json:"fallback_reason,omitempty"`
}

type QueryTelemetry struct {
	RequestID         string  `json:"request_id"`
	DBLatencyMS       int64   `json:"db_latency_ms"`
	KBLatencyMS       int64   `json:"kb_latency_ms"`
	LLMLatencyMS      int64   `json:"llm_latency_ms"`
	TotalLatencyMS    int64   `json:"total_latency_ms"`
	PlannerConfidence float64 `json:"planner_confidence"`
	ExecutionPath     string  `json:"execution_path"`
}
