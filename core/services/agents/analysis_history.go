package agents

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	AnalysisHistoryContractV1   = "forensics.case-analysis-history/v1"
	AnalysisSourceRuntime       = "runtime"
	AnalysisSourceBrowserImport = "browser_import"
)

var ErrAnalysisHistoryUnavailable = errors.New("retained analysis history is unavailable")

// AgentAnalysisRecord is the retained, case-authorized system of record for a
// forensic query and its final answer. Intermediate reasoning and tool streams
// are intentionally excluded.
type AgentAnalysisRecord struct {
	ID                 string     `gorm:"primaryKey;size:36" json:"analysis_id"`
	ContractVersion    string     `gorm:"size:64;not null" json:"contract_version"`
	TenantID           string     `gorm:"size:128;not null;index:idx_analysis_scope,priority:1;uniqueIndex:idx_analysis_request,priority:1" json:"tenant_id"`
	UserID             string     `gorm:"size:128;not null;index:idx_analysis_scope,priority:2;uniqueIndex:idx_analysis_request,priority:2" json:"-"`
	AgentName          string     `gorm:"size:255;not null;index:idx_analysis_scope,priority:3;uniqueIndex:idx_analysis_request,priority:3" json:"agent_name"`
	CaseID             string     `gorm:"size:255;not null;index:idx_analysis_scope,priority:4;uniqueIndex:idx_analysis_request,priority:4" json:"case_id"`
	CollectionID       string     `gorm:"size:255;not null" json:"collection_id"`
	MessageID          string     `gorm:"size:128;not null;uniqueIndex:idx_analysis_request,priority:5" json:"message_id"`
	RetryOf            string     `gorm:"size:128;index" json:"retry_of,omitempty"`
	QueryText          string     `gorm:"type:text;not null" json:"query"`
	AnswerText         string     `gorm:"type:text" json:"answer,omitempty"`
	AnswerMetadata     string     `gorm:"type:text" json:"-"`
	Status             string     `gorm:"size:64;not null;index" json:"status"`
	ExecutionAuthority string     `gorm:"size:64" json:"execution_authority"`
	ModelRole          string     `gorm:"size:64" json:"model_role"`
	Source             string     `gorm:"size:32;not null;index" json:"source"`
	ImportID           string     `gorm:"size:36;index" json:"import_id,omitempty"`
	Saved              bool       `gorm:"not null;default:false;index" json:"saved"`
	SavedTitle         string     `gorm:"size:160" json:"saved_title,omitempty"`
	SavedBy            string     `gorm:"size:128" json:"-"`
	SavedAt            *time.Time `json:"saved_at,omitempty"`
	LegalHold          bool       `gorm:"not null;default:false;index" json:"legal_hold"`
	CreatedAt          time.Time  `gorm:"not null;index:idx_analysis_scope,priority:5,sort:desc" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"not null" json:"updated_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
}

func (AgentAnalysisRecord) TableName() string { return "agent_analysis_history" }

type AnalysisHistoryEntry struct {
	AgentAnalysisRecord
	Metadata map[string]any `json:"metadata,omitempty"`
}

func analysisHistoryEntry(record AgentAnalysisRecord) AnalysisHistoryEntry {
	entry := AnalysisHistoryEntry{AgentAnalysisRecord: record}
	if record.AnswerMetadata != "" {
		_ = json.Unmarshal([]byte(record.AnswerMetadata), &entry.Metadata)
	}
	return entry
}

type AnalysisHistoryScope struct {
	TenantID, UserID, AgentName, CaseID, CollectionID string
}

type AnalysisHistoryListOptions struct {
	Limit     int
	Cursor    string
	SavedOnly bool
}

type AnalysisHistoryPage struct {
	ContractVersion string                 `json:"contract_version"`
	Items           []AnalysisHistoryEntry `json:"items"`
	NextCursor      string                 `json:"next_cursor,omitempty"`
	HasMore         bool                   `json:"has_more"`
}

type analysisCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

func encodeAnalysisCursor(record AgentAnalysisRecord) string {
	payload, _ := json.Marshal(analysisCursor{CreatedAt: record.CreatedAt.UTC(), ID: record.ID})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeAnalysisCursor(value string) (analysisCursor, error) {
	var cursor analysisCursor
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || json.Unmarshal(payload, &cursor) != nil || cursor.CreatedAt.IsZero() || cursor.ID == "" {
		return analysisCursor{}, errors.New("invalid history cursor")
	}
	return cursor, nil
}

func (s *AgentStore) CreateAnalysis(record *AgentAnalysisRecord) (bool, error) {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.ContractVersion == "" {
		record.ContractVersion = AnalysisHistoryContractV1
	}
	if record.Source == "" {
		record.Source = AnalysisSourceRuntime
	}
	if record.Status == "" {
		record.Status = "processing"
	}
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	result := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(record)
	return result.RowsAffected == 1, result.Error
}

func (s *AgentStore) UpdateAnalysisStatus(userID, agentName, caseID, messageID, status string) error {
	status = canonicalAnalysisStatus(status)
	updates := map[string]any{"status": status, "updated_at": time.Now().UTC()}
	if IsAnalysisTerminalStatus(status) {
		updates["completed_at"] = time.Now().UTC()
	}
	q := s.db.Model(&AgentAnalysisRecord{}).Where("user_id = ? AND agent_name = ? AND message_id = ?", userID, agentName, messageID)
	if caseID != "" {
		q = q.Where("case_id = ?", caseID)
	}
	q = q.Where("status NOT IN ('completed','error','timed_out','cancelled','needs_input')")
	if status == "completed" {
		// A late generic completion event cannot erase a failure or manufacture
		// a successful analysis before its answer/outcome has been retained.
		updates["status"] = gorm.Expr("CASE WHEN status IN ('error','timed_out','cancelled','needs_input') THEN status WHEN coalesce(answer_text,'') = '' AND coalesce(answer_metadata,'') = '' THEN 'error' ELSE 'completed' END")
	}
	return q.Updates(updates).Error
}

// canonicalAnalysisStatus keeps transport diagnostics out of the indexed,
// bounded lifecycle field. The full error still reaches the request event.
func canonicalAnalysisStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if strings.HasPrefix(status, "error:") {
		return "error"
	}
	switch status {
	case "queued", "pending", "running", "processing", "completed", "error", "cancelled", "timed_out", "needs_input":
		return status
	case "failed":
		return "error"
	default:
		return "error"
	}
}

func IsAnalysisTerminalStatus(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "completed" || status == "needs_input" || status == "timed_out" || status == "cancelled" || status == "error" || strings.HasPrefix(status, "error:")
}

// AnalysisOutcomeStatus is shared by delivery and persistence. A zero result
// and a clarification are valid typed outcomes, not empty transport responses.
func AnalysisOutcomeStatus(answer string, metadata map[string]any) string {
	presentation, _ := metadata["presentation"].(map[string]any)
	state := firstString(presentation, "result_state", "status")
	switch state {
	case "failed", "error", "execution_failed", "unauthorized", "unavailable":
		return "error"
	case "invalid_request", "needs_input":
		return "needs_input"
	case "results_present", "complete_zero_results", "no_match_for_filter", "answered", "answered_with_limitations", "no_results":
		return "completed"
	}
	if strings.TrimSpace(answer) == "" {
		return "error"
	}
	return "completed"
}

func (s *AgentStore) CompleteAnalysis(userID, agentName, messageID, answer string, metadata map[string]any) error {
	encoded := ""
	if len(metadata) > 0 {
		encoded = string(mustJSON(metadata))
	}
	now := time.Now().UTC()
	return s.db.Model(&AgentAnalysisRecord{}).
		Where("user_id = ? AND agent_name = ? AND message_id = ?", userID, agentName, messageID).
		Where("status NOT IN ('completed','error','timed_out','cancelled','needs_input')").
		Updates(map[string]any{"answer_text": answer, "answer_metadata": encoded, "status": AnalysisOutcomeStatus(answer, metadata), "completed_at": now, "updated_at": now}).Error
}

func mustJSON(value any) []byte { payload, _ := json.Marshal(value); return payload }

// JSONPayload encodes bounded audit metadata. Callers use it only for
// non-sensitive identifiers and counts.
func JSONPayload(value any) []byte { return mustJSON(value) }

func analysisScopeQuery(db *gorm.DB, scope AnalysisHistoryScope) *gorm.DB {
	return db.Where("tenant_id = ? AND user_id = ? AND agent_name = ? AND case_id = ? AND collection_id = ?",
		scope.TenantID, scope.UserID, scope.AgentName, scope.CaseID, scope.CollectionID)
}

func (s *AgentStore) ListAnalysisHistory(scope AnalysisHistoryScope, opts AnalysisHistoryListOptions) (AnalysisHistoryPage, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	q := analysisScopeQuery(s.db.Model(&AgentAnalysisRecord{}), scope)
	if opts.SavedOnly {
		q = q.Where("saved = ?", true)
	}
	if opts.Cursor != "" {
		cursor, err := decodeAnalysisCursor(opts.Cursor)
		if err != nil {
			return AnalysisHistoryPage{}, err
		}
		q = q.Where("created_at < ? OR (created_at = ? AND id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	var records []AgentAnalysisRecord
	if err := q.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return AnalysisHistoryPage{}, err
	}
	page := AnalysisHistoryPage{ContractVersion: AnalysisHistoryContractV1, Items: []AnalysisHistoryEntry{}}
	if len(records) > limit {
		page.HasMore = true
		records = records[:limit]
	}
	for _, record := range records {
		page.Items = append(page.Items, analysisHistoryEntry(record))
	}
	if page.HasMore && len(records) > 0 {
		page.NextCursor = encodeAnalysisCursor(records[len(records)-1])
	}
	return page, nil
}

func (s *AgentStore) GetAnalysis(scope AnalysisHistoryScope, id string) (AnalysisHistoryEntry, error) {
	var record AgentAnalysisRecord
	if err := analysisScopeQuery(s.db, scope).Where("id = ?", id).First(&record).Error; err != nil {
		return AnalysisHistoryEntry{}, err
	}
	return analysisHistoryEntry(record), nil
}

func (s *AgentStore) SetAnalysisSaved(scope AnalysisHistoryScope, id string, saved bool, title, actor string) (AnalysisHistoryEntry, error) {
	updates := map[string]any{"saved": saved, "saved_title": "", "saved_by": "", "saved_at": nil, "updated_at": time.Now().UTC()}
	if saved {
		now := time.Now().UTC()
		updates["saved_title"], updates["saved_by"], updates["saved_at"] = strings.TrimSpace(title), actor, &now
	}
	result := analysisScopeQuery(s.db.Model(&AgentAnalysisRecord{}), scope).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return AnalysisHistoryEntry{}, result.Error
	}
	if result.RowsAffected == 0 {
		return AnalysisHistoryEntry{}, gorm.ErrRecordNotFound
	}
	return s.GetAnalysis(scope, id)
}

type AnalysisImportRollbackResult struct {
	ImportID          string `json:"import_id"`
	Deleted           int64  `json:"deleted"`
	RetainedSaved     int64  `json:"retained_saved"`
	RetainedLegalHold int64  `json:"retained_legal_hold"`
}

type BrowserHistoryMessage struct {
	Sender    string         `json:"sender"`
	Content   string         `json:"content"`
	Timestamp int64          `json:"timestamp"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type BrowserHistoryConversation struct {
	ID        string                  `json:"id"`
	Name      string                  `json:"name"`
	CreatedAt int64                   `json:"created_at"`
	UpdatedAt int64                   `json:"updated_at"`
	Messages  []BrowserHistoryMessage `json:"messages"`
}

type BrowserHistoryImportResult struct {
	ContractVersion   string `json:"contract_version"`
	ImportID          string `json:"import_id"`
	Imported          int    `json:"imported"`
	Skipped           int    `json:"skipped"`
	RollbackAvailable bool   `json:"rollback_available"`
}

func (s *AgentStore) RollbackAnalysisImport(scope AnalysisHistoryScope, importID string) (AnalysisImportRollbackResult, error) {
	base := analysisScopeQuery(s.db.Model(&AgentAnalysisRecord{}), scope).
		Where("source = ? AND import_id = ?", AnalysisSourceBrowserImport, importID)
	result := AnalysisImportRollbackResult{ImportID: importID}
	if err := base.Where("saved = ? AND legal_hold = ?", true, false).Count(&result.RetainedSaved).Error; err != nil {
		return result, err
	}
	if err := base.Where("legal_hold = ?", true).Count(&result.RetainedLegalHold).Error; err != nil {
		return result, err
	}
	deleted := base.Where("saved = ? AND legal_hold = ?", false, false).Delete(&AgentAnalysisRecord{})
	result.Deleted = deleted.RowsAffected
	return result, deleted.Error
}

func BrowserImportMessageID(userID, agentName, caseID, conversationID string, index int, query string) string {
	digest := ChatRetryMessageHash(fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d\x00%s", userID, agentName, caseID, conversationID, index, query))
	return "browser-" + digest[:40]
}
