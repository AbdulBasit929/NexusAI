package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	contentAddressedLayoutVersion = "sha256-scope-v1"
	storageVerificationMethod     = "sha256-full-read-after-retain"
)

var (
	errEvidenceObjectWritable  = errors.New("retained evidence object is writable")
	errEvidenceReceiptWritable = errors.New("retained evidence receipt is writable")
)

type contentAddressedStore struct {
	root string
	now  func() time.Time
}

type storedEvidenceObject struct {
	Path               string
	StorageURI         string
	ReceiptURI         string
	ScopeKey           string
	SHA256             string
	SizeBytes          int64
	RetainedAt         time.Time
	VerifiedAt         time.Time
	Reused             bool
	WriteOnce          bool
	LayoutVersion      string
	VerificationMethod string
}

type evidenceStorageReceipt struct {
	LayoutVersion string    `json:"layout_version"`
	ScopeKey      string    `json:"scope_key"`
	TenantID      string    `json:"tenant_id"`
	CollectionID  string    `json:"collection_id"`
	SHA256        string    `json:"sha256"`
	SizeBytes     int64     `json:"size_bytes"`
	StorageURI    string    `json:"storage_uri"`
	RetainedAt    time.Time `json:"retained_at"`
}

type evidenceRetentionError struct {
	operation      string
	cause          error
	quarantinePath string
}

func (e *evidenceRetentionError) Error() string {
	return fmt.Sprintf("%s: %v; incoming bytes retained for integrity review", e.operation, e.cause)
}

func (e *evidenceRetentionError) Unwrap() error {
	return e.cause
}

func newContentAddressedStore(root string) (contentAddressedStore, error) {
	if strings.TrimSpace(root) == "" {
		return contentAddressedStore{}, errors.New("content-addressed storage root is required")
	}
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return contentAddressedStore{}, fmt.Errorf("resolve content-addressed storage root: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o750); err != nil {
		return contentAddressedStore{}, fmt.Errorf("create content-addressed storage root: %w", err)
	}
	resolvedRoot := absRoot
	if runtime.GOOS != "windows" {
		resolvedRoot, err = filepath.EvalSymlinks(absRoot)
		if err != nil {
			return contentAddressedStore{}, fmt.Errorf("resolve content-addressed storage root links: %w", err)
		}
	}
	return contentAddressedStore{root: resolvedRoot, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (s contentAddressedStore) Retain(reader io.Reader, tenantID, collectionID string) (storedEvidenceObject, error) {
	if reader == nil {
		return storedEvidenceObject{}, errors.New("evidence source reader is required")
	}
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(collectionID) == "" {
		return storedEvidenceObject{}, errors.New("tenant and collection are required for evidence retention")
	}

	incomingDir := filepath.Join(s.root, ".incoming")
	if err := os.MkdirAll(incomingDir, 0o750); err != nil {
		return storedEvidenceObject{}, fmt.Errorf("create incoming evidence directory: %w", err)
	}
	tmp, err := os.CreateTemp(incomingDir, ".evidence-*")
	if err != nil {
		return storedEvidenceObject{}, fmt.Errorf("create incoming evidence file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	hasher := sha256.New()
	sizeBytes, copyErr := io.Copy(io.MultiWriter(tmp, hasher), reader)
	if copyErr != nil {
		_ = tmp.Close()
		return storedEvidenceObject{}, fmt.Errorf("retain incoming evidence bytes: %w", copyErr)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return storedEvidenceObject{}, fmt.Errorf("sync incoming evidence bytes: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return storedEvidenceObject{}, fmt.Errorf("close incoming evidence bytes: %w", err)
	}

	contentHash := hex.EncodeToString(hasher.Sum(nil))
	scopeKey := evidenceStorageScopeKey(tenantID, collectionID)
	storageURI := evidenceStorageURI(scopeKey, contentHash)
	objectPath := s.objectPath(scopeKey, contentHash)
	if err := os.MkdirAll(filepath.Dir(objectPath), 0o750); err != nil {
		return storedEvidenceObject{}, fmt.Errorf("create evidence object directory: %w", err)
	}

	created := false
	if err := os.Link(tmpPath, objectPath); err == nil {
		created = true
		if err := os.Remove(tmpPath); err != nil {
			return storedEvidenceObject{}, fmt.Errorf("remove published incoming evidence link: %w", err)
		}
		cleanupTemp = false
		if err := os.Chmod(objectPath, 0o440); err != nil {
			return storedEvidenceObject{}, fmt.Errorf("make retained evidence object read-only: %w", err)
		}
		if err := syncDirectory(filepath.Dir(objectPath)); err != nil {
			return storedEvidenceObject{}, fmt.Errorf("sync evidence object directory: %w", err)
		}
	} else if _, statErr := os.Lstat(objectPath); statErr != nil {
		quarantinePath, quarantineErr := s.quarantine(tmpPath, scopeKey, contentHash)
		cleanupTemp = false
		if quarantineErr != nil {
			quarantinePath = tmpPath
		}
		return storedEvidenceObject{}, &evidenceRetentionError{
			operation: "publish retained evidence object", cause: err, quarantinePath: quarantinePath,
		}
	}

	if err := retryPublishedIntegrity(func() error {
		return verifyEvidenceFile(objectPath, contentHash, sizeBytes)
	}); err != nil {
		if !created {
			quarantinePath, quarantineErr := s.quarantine(tmpPath, scopeKey, contentHash)
			cleanupTemp = false
			if quarantineErr != nil {
				quarantinePath = tmpPath
			}
			return storedEvidenceObject{}, &evidenceRetentionError{
				operation: "verify existing retained evidence object", cause: err, quarantinePath: quarantinePath,
			}
		}
		return storedEvidenceObject{}, fmt.Errorf("verify retained evidence object: %w", err)
	}

	retainedAt := s.now().UTC()
	receipt := evidenceStorageReceipt{
		LayoutVersion: contentAddressedLayoutVersion,
		ScopeKey:      scopeKey,
		TenantID:      tenantID,
		CollectionID:  collectionID,
		SHA256:        contentHash,
		SizeBytes:     sizeBytes,
		StorageURI:    storageURI,
		RetainedAt:    retainedAt,
	}
	existingReceipt, err := s.ensureReceipt(receipt)
	if err != nil {
		if !created {
			quarantinePath, quarantineErr := s.quarantine(tmpPath, scopeKey, contentHash)
			cleanupTemp = false
			if quarantineErr != nil {
				quarantinePath = tmpPath
			}
			return storedEvidenceObject{}, &evidenceRetentionError{
				operation: "verify retained evidence receipt", cause: err, quarantinePath: quarantinePath,
			}
		}
		return storedEvidenceObject{}, fmt.Errorf("record retained evidence receipt: %w", err)
	}
	if !created {
		if err := os.Remove(tmpPath); err != nil {
			return storedEvidenceObject{}, fmt.Errorf("remove duplicate incoming evidence bytes: %w", err)
		}
		cleanupTemp = false
		retainedAt = existingReceipt.RetainedAt
	}

	verifiedAt := s.now().UTC()
	return storedEvidenceObject{
		Path: objectPath, StorageURI: storageURI,
		ReceiptURI: evidenceReceiptURI(scopeKey, contentHash),
		ScopeKey:   scopeKey, SHA256: contentHash, SizeBytes: sizeBytes,
		RetainedAt: retainedAt, VerifiedAt: verifiedAt, Reused: !created,
		WriteOnce: true, LayoutVersion: contentAddressedLayoutVersion,
		VerificationMethod: storageVerificationMethod,
	}, nil
}

func (s contentAddressedStore) Verify(storageURI string, expectedSize int64) (storedEvidenceObject, error) {
	scopeKey, contentHash, err := parseEvidenceStorageURI(storageURI)
	if err != nil {
		return storedEvidenceObject{}, err
	}
	path := s.objectPath(scopeKey, contentHash)
	if err := verifyEvidenceFile(path, contentHash, expectedSize); err != nil {
		return storedEvidenceObject{}, err
	}
	receipt, err := s.readReceipt(scopeKey, contentHash)
	if err != nil {
		return storedEvidenceObject{}, err
	}
	if err := validateEvidenceReceipt(receipt, scopeKey, contentHash, expectedSize, storageURI); err != nil {
		return storedEvidenceObject{}, err
	}
	return storedEvidenceObject{
		Path: path, StorageURI: storageURI,
		ReceiptURI: evidenceReceiptURI(scopeKey, contentHash),
		ScopeKey:   scopeKey, SHA256: contentHash, SizeBytes: expectedSize,
		RetainedAt: receipt.RetainedAt, VerifiedAt: s.now().UTC(),
		Reused: true, WriteOnce: true, LayoutVersion: contentAddressedLayoutVersion,
		VerificationMethod: storageVerificationMethod,
	}, nil
}

func (s contentAddressedStore) objectPath(scopeKey, contentHash string) string {
	return filepath.Join(s.root, "objects", scopeKey, contentHash[:2], contentHash)
}

func (s contentAddressedStore) receiptPath(scopeKey, contentHash string) string {
	return filepath.Join(s.root, "receipts", scopeKey, contentHash[:2], contentHash+".json")
}

func (s contentAddressedStore) quarantine(tmpPath, scopeKey, contentHash string) (string, error) {
	dir := filepath.Join(s.root, "integrity-conflicts", scopeKey, contentHash[:2], contentHash)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	destination := filepath.Join(dir, filepath.Base(tmpPath)+".retained")
	if err := os.Rename(tmpPath, destination); err != nil {
		return "", err
	}
	if err := os.Chmod(destination, 0o440); err != nil {
		return destination, err
	}
	if err := syncDirectory(dir); err != nil {
		return destination, err
	}
	return destination, nil
}

func (s contentAddressedStore) ensureReceipt(receipt evidenceStorageReceipt) (evidenceStorageReceipt, error) {
	path := s.receiptPath(receipt.ScopeKey, receipt.SHA256)
	if existing, err := s.readReceipt(receipt.ScopeKey, receipt.SHA256); err == nil {
		return existing, validateEvidenceReceipt(existing, receipt.ScopeKey, receipt.SHA256, receipt.SizeBytes, receipt.StorageURI)
	} else if errors.Is(err, errEvidenceReceiptWritable) {
		existing, retryErr := s.readReceiptWithRetry(receipt.ScopeKey, receipt.SHA256)
		if retryErr != nil {
			return evidenceStorageReceipt{}, retryErr
		}
		return existing, validateEvidenceReceipt(existing, receipt.ScopeKey, receipt.SHA256, receipt.SizeBytes, receipt.StorageURI)
	} else if !errors.Is(err, os.ErrNotExist) {
		return evidenceStorageReceipt{}, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return evidenceStorageReceipt{}, fmt.Errorf("create evidence receipt directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".receipt-*")
	if err != nil {
		return evidenceStorageReceipt{}, fmt.Errorf("create evidence receipt: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	encoder := json.NewEncoder(tmp)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(receipt); err != nil {
		_ = tmp.Close()
		return evidenceStorageReceipt{}, fmt.Errorf("encode evidence receipt: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return evidenceStorageReceipt{}, fmt.Errorf("sync evidence receipt: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return evidenceStorageReceipt{}, fmt.Errorf("close evidence receipt: %w", err)
	}
	if err := os.Link(tmpPath, path); err != nil {
		if existing, readErr := s.readReceiptWithRetry(receipt.ScopeKey, receipt.SHA256); readErr == nil {
			return existing, validateEvidenceReceipt(existing, receipt.ScopeKey, receipt.SHA256, receipt.SizeBytes, receipt.StorageURI)
		}
		return evidenceStorageReceipt{}, fmt.Errorf("publish evidence receipt: %w", err)
	}
	if err := os.Remove(tmpPath); err != nil {
		return evidenceStorageReceipt{}, fmt.Errorf("remove published receipt link: %w", err)
	}
	if err := os.Chmod(path, 0o440); err != nil {
		return evidenceStorageReceipt{}, fmt.Errorf("make evidence receipt read-only: %w", err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return evidenceStorageReceipt{}, fmt.Errorf("sync evidence receipt directory: %w", err)
	}
	return receipt, nil
}

func (s contentAddressedStore) readReceipt(scopeKey, contentHash string) (evidenceStorageReceipt, error) {
	path := s.receiptPath(scopeKey, contentHash)
	info, err := os.Lstat(path)
	if err != nil {
		return evidenceStorageReceipt{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return evidenceStorageReceipt{}, errors.New("evidence receipt is not a regular file")
	}
	if info.Mode().Perm()&0o222 != 0 {
		return evidenceStorageReceipt{}, errEvidenceReceiptWritable
	}
	file, err := os.Open(path)
	if err != nil {
		return evidenceStorageReceipt{}, fmt.Errorf("open evidence receipt: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 64<<10))
	decoder.DisallowUnknownFields()
	var receipt evidenceStorageReceipt
	if err := decoder.Decode(&receipt); err != nil {
		return evidenceStorageReceipt{}, fmt.Errorf("decode evidence receipt: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return evidenceStorageReceipt{}, errors.New("evidence receipt contains trailing data")
	}
	return receipt, nil
}

func (s contentAddressedStore) readReceiptWithRetry(scopeKey, contentHash string) (evidenceStorageReceipt, error) {
	deadline := time.Now().Add(2 * time.Second)
	for {
		receipt, err := s.readReceipt(scopeKey, contentHash)
		if err == nil {
			return receipt, nil
		}
		if (!errors.Is(err, os.ErrNotExist) && !errors.Is(err, errEvidenceReceiptWritable)) || time.Now().After(deadline) {
			return evidenceStorageReceipt{}, err
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func validateEvidenceReceipt(receipt evidenceStorageReceipt, scopeKey, contentHash string, sizeBytes int64, storageURI string) error {
	if receipt.LayoutVersion != contentAddressedLayoutVersion ||
		receipt.ScopeKey != scopeKey || receipt.SHA256 != contentHash ||
		receipt.SizeBytes != sizeBytes || receipt.StorageURI != storageURI ||
		evidenceStorageScopeKey(receipt.TenantID, receipt.CollectionID) != scopeKey ||
		receipt.RetainedAt.IsZero() {
		return errors.New("evidence receipt does not match retained object identity")
	}
	return nil
}

func verifyEvidenceFile(path, expectedSHA256 string, expectedSize int64) error {
	if !validLowerSHA256(expectedSHA256) {
		return errors.New("expected evidence SHA-256 must be 64 lowercase hexadecimal characters")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat retained evidence object: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("retained evidence object is not a regular file")
	}
	if info.Mode().Perm()&0o222 != 0 {
		return errEvidenceObjectWritable
	}
	if expectedSize < 0 || info.Size() != expectedSize {
		return fmt.Errorf("retained evidence size mismatch: expected %d, got %d", expectedSize, info.Size())
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open retained evidence object: %w", err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("hash retained evidence object: %w", err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expectedSHA256 {
		return fmt.Errorf("retained evidence SHA-256 mismatch: expected %s, got %s", expectedSHA256, actual)
	}
	return nil
}

func retryPublishedIntegrity(check func() error) error {
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := check()
		if err == nil {
			return nil
		}
		if !errors.Is(err, errEvidenceObjectWritable) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func evidenceStorageScopeKey(tenantID, collectionID string) string {
	hasher := sha256.New()
	tenant := []byte(tenantID)
	collection := []byte(collectionID)
	_, _ = fmt.Fprintf(hasher, "%d:", len(tenant))
	_, _ = hasher.Write(tenant)
	_, _ = fmt.Fprintf(hasher, "%d:", len(collection))
	_, _ = hasher.Write(collection)
	return hex.EncodeToString(hasher.Sum(nil))
}

func evidenceStorageURI(scopeKey, contentHash string) string {
	return "forensic-spool://" + contentAddressedLayoutVersion + "/" + scopeKey + "/" + contentHash
}

func evidenceReceiptURI(scopeKey, contentHash string) string {
	return "forensic-spool-receipt://" + contentAddressedLayoutVersion + "/" + scopeKey + "/" + contentHash
}

func parseEvidenceStorageURI(raw string) (string, string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("parse evidence storage URI: %w", err)
	}
	if parsed.Scheme != "forensic-spool" || parsed.Host != contentAddressedLayoutVersion ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.RawPath != "" {
		return "", "", errors.New("unsupported evidence storage URI")
	}
	parts := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
	if len(parts) != 2 || !validLowerSHA256(parts[0]) || !validLowerSHA256(parts[1]) {
		return "", "", errors.New("invalid evidence storage URI identity")
	}
	return parts[0], parts[1], nil
}

func validLowerSHA256(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil && runtime.GOOS != "windows" {
		return err
	}
	return nil
}
