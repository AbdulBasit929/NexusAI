package records

import "time"

const (
	RecordTypeCDR           = "cdr"
	RecordTypeANPR          = "anpr"
	RecordTypeIPDR          = "ipdr"
	RecordTypeSubscriber    = "subscriber"
	RecordTypeTowerLocation = "tower_location"
	RecordTypeTransaction   = "transaction"
	RecordTypeAccessLog     = "access_log"
	RecordTypeGeneric       = "generic"
)

var SupportedRecordTypes = []string{
	RecordTypeCDR,
	RecordTypeANPR,
	RecordTypeIPDR,
	RecordTypeSubscriber,
	RecordTypeTowerLocation,
	RecordTypeTransaction,
	RecordTypeAccessLog,
	RecordTypeGeneric,
}

const (
	FieldTypeEmpty  = "empty"
	FieldTypeString = "string"
	FieldTypeNumber = "number"
	FieldTypeBool   = "bool"
	FieldTypeTime   = "time"
	FieldTypeMixed  = "mixed"
)

type FieldSchema struct {
	OriginalName    string   `json:"original_name"`
	NormalizedName  string   `json:"normalized_name"`
	CanonicalName   string   `json:"canonical_name,omitempty"`
	Type            string   `json:"type"`
	NonEmptyCount   int      `json:"non_empty_count"`
	SampleValues    []string `json:"sample_values,omitempty"`
	DetectedAliases []string `json:"detected_aliases,omitempty"`
}

type Batch struct {
	ID             string        `json:"id"`
	UserID         string        `json:"user_id,omitempty"`
	CollectionName string        `json:"collection_name,omitempty"`
	RecordType     string        `json:"record_type"`
	SourceFile     string        `json:"source_file"`
	SourceEntry    string        `json:"source_entry,omitempty"`
	RowCount       int           `json:"row_count"`
	ErrorCount     int           `json:"error_count"`
	Fields         []FieldSchema `json:"fields"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

type Record struct {
	ID               string         `json:"id"`
	UserID           string         `json:"user_id,omitempty"`
	BatchID          string         `json:"batch_id"`
	CollectionName   string         `json:"collection_name,omitempty"`
	RecordType       string         `json:"record_type"`
	SourceFile       string         `json:"source_file"`
	SourceEntry      string         `json:"source_entry,omitempty"`
	RowNumber        int            `json:"row_number"`
	RawFields        map[string]any `json:"raw_fields"`
	NormalizedFields map[string]any `json:"normalized_fields"`
	Timestamp        string         `json:"timestamp,omitempty"`
	IngestedAt       time.Time      `json:"ingested_at"`
}

type IngestError struct {
	RowNumber int    `json:"row_number"`
	Message   string `json:"message"`
	Raw       string `json:"raw,omitempty"`
}

type IngestOptions struct {
	UserID         string
	CollectionName string
	RecordType     string
	SourceFile     string
	SourceEntry    string
}

type IngestResult struct {
	Batch    Batch         `json:"batch"`
	Errors   []IngestError `json:"errors,omitempty"`
	Warnings []string      `json:"warnings,omitempty"`
}

type Filter struct {
	Field  string `json:"field"`
	Op     string `json:"op"`
	Value  any    `json:"value,omitempty"`
	Values []any  `json:"values,omitempty"`
}

type TimeRange struct {
	Field string `json:"field,omitempty"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
}

type SortField struct {
	Field     string `json:"field"`
	Direction string `json:"direction,omitempty"`
}

type QueryRequest struct {
	BatchIDs       []string       `json:"batch_ids,omitempty"`
	CollectionName string         `json:"collection_name,omitempty"`
	RecordType     string         `json:"record_type,omitempty"`
	Filters        []Filter       `json:"filters,omitempty"`
	TimeRange      *TimeRange     `json:"time_range,omitempty"`
	Sort           []SortField    `json:"sort,omitempty"`
	Fields         []string       `json:"fields,omitempty"`
	Limit          int            `json:"limit,omitempty"`
	Offset         int            `json:"offset,omitempty"`
	Helper         string         `json:"helper,omitempty"`
	HelperArgs     map[string]any `json:"helper_args,omitempty"`
}

type QueryResponse struct {
	Records  []map[string]any `json:"records"`
	Count    int              `json:"count"`
	Total    int              `json:"total"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
	BatchIDs []string         `json:"batch_ids,omitempty"`
}

type AggregateRequest struct {
	QueryRequest
	Operation string   `json:"operation"`
	Field     string   `json:"field,omitempty"`
	GroupBy   []string `json:"group_by,omitempty"`
	TopN      int      `json:"top_n,omitempty"`
}

type GroupResult struct {
	Key    map[string]any `json:"key,omitempty"`
	Value  any            `json:"value,omitempty"`
	Count  int            `json:"count,omitempty"`
	Record map[string]any `json:"record,omitempty"`
}

type AggregateResponse struct {
	Operation string         `json:"operation"`
	Field     string         `json:"field,omitempty"`
	Count     int            `json:"count,omitempty"`
	Value     any            `json:"value,omitempty"`
	Values    []any          `json:"values,omitempty"`
	Record    map[string]any `json:"record,omitempty"`
	Groups    []GroupResult  `json:"groups,omitempty"`
	Results   []GroupResult  `json:"results,omitempty"`
}

type CorrelateRequest struct {
	Left              QueryRequest `json:"left"`
	Right             QueryRequest `json:"right"`
	EntityFields      []string     `json:"entity_fields,omitempty"`
	TimeField         string       `json:"time_field,omitempty"`
	TimeWindowSeconds int          `json:"time_window_seconds,omitempty"`
	Limit             int          `json:"limit,omitempty"`
}

type CorrelationMatch struct {
	Entity       map[string]any `json:"entity,omitempty"`
	TimeDeltaSec int64          `json:"time_delta_seconds,omitempty"`
	Left         map[string]any `json:"left"`
	Right        map[string]any `json:"right"`
}

type CorrelateResponse struct {
	Matches []CorrelationMatch `json:"matches"`
	Count   int                `json:"count"`
}

type SchemaResponse struct {
	RecordType string              `json:"record_type"`
	Aliases    map[string][]string `json:"aliases"`
	Fields     []FieldSchema       `json:"fields,omitempty"`
}
