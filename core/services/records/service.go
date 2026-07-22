package records

import (
	"bytes"
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	store *Store
}

func DefaultBaseDir(dataPath, agentStateDir, dynamicConfigsDir string) string {
	base := cmp.Or(dataPath, agentStateDir, dynamicConfigsDir, "data")
	return filepath.Join(base, "records")
}

func NewService(baseDir string) *Service {
	return &Service{store: NewStore(baseDir)}
}

func (s *Service) Ingest(data []byte, opts IngestOptions) (IngestResult, error) {
	sourceFile := opts.SourceFile
	if sourceFile == "" {
		sourceFile = "records"
	}
	parsed, err := Parse(sourceFile, bytes.NewReader(data))
	if err != nil {
		return IngestResult{}, err
	}

	recordType := opts.RecordType
	if recordType == "" || recordType == "auto" {
		recordType = parsed.RecordType
	}
	if !validRecordType(recordType) {
		return IngestResult{}, fmt.Errorf("unsupported record_type %q", recordType)
	}
	if recordType == "" {
		recordType = RecordTypeGeneric
	}

	now := time.Now().UTC()
	batchID := uuid.NewString()
	batch := Batch{
		ID:             batchID,
		UserID:         opts.UserID,
		CollectionName: opts.CollectionName,
		RecordType:     recordType,
		SourceFile:     sourceFile,
		SourceEntry:    opts.SourceEntry,
		RowCount:       len(parsed.Rows),
		ErrorCount:     len(parsed.Errors),
		Fields:         parsed.Fields,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	records := make([]Record, 0, len(parsed.Rows))
	for _, row := range parsed.Rows {
		normalized := normalizeRecordFields(row.Fields)
		record := Record{
			ID:               fmt.Sprintf("%s:%d", batchID, row.RowNumber),
			UserID:           opts.UserID,
			BatchID:          batchID,
			CollectionName:   opts.CollectionName,
			RecordType:       recordType,
			SourceFile:       sourceFile,
			SourceEntry:      opts.SourceEntry,
			RowNumber:        row.RowNumber,
			RawFields:        row.Fields,
			NormalizedFields: normalized,
			Timestamp:        firstTimestamp(normalized),
			IngestedAt:       now,
		}
		records = append(records, record)
	}

	if err := s.store.SaveBatch(batch, records); err != nil {
		return IngestResult{}, err
	}

	warnings := []string{}
	if recordType == RecordTypeGeneric && opts.RecordType == "" {
		warnings = append(warnings, "record type could not be inferred confidently; stored as generic")
	}
	return IngestResult{Batch: batch, Errors: parsed.Errors, Warnings: warnings}, nil
}

func (s *Service) ListBatches(userID string) ([]Batch, error) {
	return s.store.ListBatches(userID)
}

func (s *Service) GetBatch(userID, batchID string) (Batch, error) {
	return s.store.GetBatch(userID, batchID)
}

func (s *Service) DeleteBatch(userID, batchID string) error {
	return s.store.DeleteBatch(userID, batchID)
}

func (s *Service) Schema(userID, recordType string) (SchemaResponse, error) {
	resp := SchemaResponse{
		RecordType: recordType,
		Aliases:    AliasesForRecordType(recordType),
	}
	if recordType == "" {
		return resp, nil
	}
	batches, err := s.store.ListBatches(userID)
	if err != nil {
		return resp, err
	}
	seen := map[string]bool{}
	for _, batch := range batches {
		if batch.RecordType != recordType {
			continue
		}
		for _, field := range batch.Fields {
			key := field.OriginalName + "|" + field.CanonicalName
			if seen[key] {
				continue
			}
			seen[key] = true
			resp.Fields = append(resp.Fields, field)
		}
	}
	slices.SortFunc(resp.Fields, func(a, b FieldSchema) int {
		if a.CanonicalName == b.CanonicalName {
			return cmp.Compare(a.OriginalName, b.OriginalName)
		}
		return cmp.Compare(a.CanonicalName, b.CanonicalName)
	})
	return resp, nil
}

func validRecordType(recordType string) bool {
	return slices.Contains(SupportedRecordTypes, recordType)
}

func normalizeRecordFields(fields map[string]any) map[string]any {
	out := map[string]any{}
	for name, value := range fields {
		normalizedName := NormalizeFieldName(name)
		canonicalName := CanonicalFieldName(name)
		normalizedValue := normalizeValue(value)
		out[normalizedName] = normalizedValue
		if canonicalName != "" {
			if _, exists := out[canonicalName]; !exists {
				out[canonicalName] = normalizedValue
			}
		}
	}
	return out
}

func firstTimestamp(fields map[string]any) string {
	for _, key := range []string{"timestamp", "date"} {
		if value, ok := fields[key]; ok {
			if text := valueToString(value); text != "" {
				if t, ok := parseTimeValue(text); ok {
					return t.Format(time.RFC3339)
				}
				return text
			}
		}
	}
	return ""
}
