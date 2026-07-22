package records

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type Store struct {
	baseDir string
	mu      sync.RWMutex
}

func NewStore(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

func (s *Store) SaveBatch(batch Batch, records []Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if batch.ID == "" {
		return fmt.Errorf("batch id is required")
	}
	if err := s.ensureUserDirs(batch.UserID); err != nil {
		return err
	}
	if err := writeJSONFileAtomic(s.batchPath(batch.UserID, batch.ID), batch); err != nil {
		return err
	}
	return s.writeRecords(batch.UserID, batch.ID, records)
}

func (s *Store) ListBatches(userID string) ([]Batch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := s.batchesDir(userID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Batch{}, nil
		}
		return nil, err
	}
	batches := []Batch{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var batch Batch
		if err := readJSONFile(filepath.Join(dir, entry.Name()), &batch); err != nil {
			continue
		}
		batches = append(batches, batch)
	}
	slices.SortFunc(batches, func(a, b Batch) int {
		return b.CreatedAt.Compare(a.CreatedAt)
	})
	return batches, nil
}

func (s *Store) GetBatch(userID, batchID string) (Batch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var batch Batch
	if err := validateID(batchID); err != nil {
		return batch, err
	}
	err := readJSONFile(s.batchPath(userID, batchID), &batch)
	return batch, err
}

func (s *Store) DeleteBatch(userID, batchID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := validateID(batchID); err != nil {
		return err
	}
	batchPath := s.batchPath(userID, batchID)
	recordsPath := s.recordsPath(userID, batchID)
	if err := os.Remove(batchPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(recordsPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Store) LoadRecords(userID string, req QueryRequest) ([]Record, []Batch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	batches, err := s.matchingBatches(userID, req)
	if err != nil {
		return nil, nil, err
	}
	records := []Record{}
	for _, batch := range batches {
		batchRecords, err := s.readRecords(userID, batch.ID)
		if err != nil {
			return nil, nil, err
		}
		records = append(records, batchRecords...)
	}
	return records, batches, nil
}

func (s *Store) matchingBatches(userID string, req QueryRequest) ([]Batch, error) {
	if len(req.BatchIDs) > 0 {
		batches := make([]Batch, 0, len(req.BatchIDs))
		for _, id := range req.BatchIDs {
			if err := validateID(id); err != nil {
				return nil, err
			}
			batch, err := s.getBatchUnlocked(userID, id)
			if err != nil {
				return nil, err
			}
			if req.CollectionName != "" && batch.CollectionName != req.CollectionName {
				continue
			}
			if req.RecordType != "" && batch.RecordType != req.RecordType {
				continue
			}
			batches = append(batches, batch)
		}
		return batches, nil
	}

	batches, err := s.listBatchesUnlocked(userID)
	if err != nil {
		return nil, err
	}
	filtered := batches[:0]
	for _, batch := range batches {
		if req.CollectionName != "" && batch.CollectionName != req.CollectionName {
			continue
		}
		if req.RecordType != "" && batch.RecordType != req.RecordType {
			continue
		}
		filtered = append(filtered, batch)
	}
	return filtered, nil
}

func (s *Store) getBatchUnlocked(userID, batchID string) (Batch, error) {
	var batch Batch
	err := readJSONFile(s.batchPath(userID, batchID), &batch)
	return batch, err
}

func (s *Store) listBatchesUnlocked(userID string) ([]Batch, error) {
	dir := s.batchesDir(userID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Batch{}, nil
		}
		return nil, err
	}
	batches := []Batch{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var batch Batch
		if err := readJSONFile(filepath.Join(dir, entry.Name()), &batch); err != nil {
			continue
		}
		batches = append(batches, batch)
	}
	return batches, nil
}

func (s *Store) writeRecords(userID, batchID string, records []Record) error {
	path := s.recordsPath(userID, batchID)
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	for _, record := range records {
		if err := enc.Encode(record); err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return err
		}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) readRecords(userID, batchID string) ([]Record, error) {
	path := s.recordsPath(userID, batchID)
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return []Record{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var records []Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), maxJSONLLineBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record Record
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) ensureUserDirs(userID string) error {
	for _, dir := range []string{s.batchesDir(userID), s.recordsDir(userID)} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) userDir(userID string) string {
	if userID == "" {
		return filepath.Join(s.baseDir, "local")
	}
	return filepath.Join(s.baseDir, "users", userID)
}

func (s *Store) batchesDir(userID string) string {
	return filepath.Join(s.userDir(userID), "batches")
}

func (s *Store) recordsDir(userID string) string {
	return filepath.Join(s.userDir(userID), "records")
}

func (s *Store) batchPath(userID, batchID string) string {
	return filepath.Join(s.batchesDir(userID), batchID+".json")
}

func (s *Store) recordsPath(userID, batchID string) string {
	return filepath.Join(s.recordsDir(userID), batchID+".jsonl")
}

func validateID(id string) error {
	if id == "" || strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return fmt.Errorf("invalid id")
	}
	return nil
}

func readJSONFile(path string, dst any) error {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func writeJSONFileAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
