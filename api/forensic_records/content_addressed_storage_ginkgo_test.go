package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("content-addressed evidence retention", func() {
	var root string
	var store contentAddressedStore

	BeforeEach(func() {
		root = filepath.Join(GinkgoT().TempDir(), "evidence-store")
		var err error
		store, err = newContentAddressedStore(root)
		Expect(err).NotTo(HaveOccurred())
	})

	It("keeps the storage scope key stable across API, worker, and migration fixtures", func() {
		Expect(evidenceStorageScopeKey("phase3-smoke", "phase3-control-plane-smoke")).To(Equal(
			"2dbfa9b88e4316f8cac0ee0094b8d25e65651bcf2ecc1fc385edfb255d07bb42",
		))
	})

	It("retains exact bytes behind a scoped create-only address and receipt", func() {
		payload := []byte("timestamp,source,target\n2026-07-29T08:00:00+05:00,923001111111,923002222222\n")
		stored, err := store.Retain(bytes.NewReader(payload), "tenant-a", "case-a")
		Expect(err).NotTo(HaveOccurred())

		digest := sha256.Sum256(payload)
		expectedHash := hex.EncodeToString(digest[:])
		expectedScope := evidenceStorageScopeKey("tenant-a", "case-a")
		Expect(stored.SHA256).To(Equal(expectedHash))
		Expect(stored.ScopeKey).To(Equal(expectedScope))
		Expect(stored.StorageURI).To(Equal(evidenceStorageURI(expectedScope, expectedHash)))
		Expect(stored.ReceiptURI).To(Equal(evidenceReceiptURI(expectedScope, expectedHash)))
		Expect(stored.SizeBytes).To(Equal(int64(len(payload))))
		Expect(stored.WriteOnce).To(BeTrue())
		Expect(stored.Reused).To(BeFalse())
		Expect(stored.Path).To(Equal(filepath.Join(root, "objects", expectedScope, expectedHash[:2], expectedHash)))
		Expect(os.ReadFile(stored.Path)).To(Equal(payload))
		info, err := os.Stat(stored.Path)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm() & 0o222).To(BeZero())

		verified, err := store.Verify(stored.StorageURI, int64(len(payload)))
		Expect(err).NotTo(HaveOccurred())
		Expect(verified.Path).To(Equal(stored.Path))
		Expect(verified.SHA256).To(Equal(stored.SHA256))
	})

	It("hashes scope values so traversal-like names cannot shape storage paths", func() {
		stored, err := store.Retain(bytes.NewReader([]byte("evidence")), "../../tenant", `..\\..\\case/../../escape`)
		Expect(err).NotTo(HaveOccurred())
		relative, err := filepath.Rel(root, stored.Path)
		Expect(err).NotTo(HaveOccurred())
		Expect(relative).NotTo(HavePrefix(".." + string(filepath.Separator)))
		Expect(relative).NotTo(ContainSubstring("tenant"))
		Expect(relative).NotTo(ContainSubstring("escape"))
		Expect(strings.Split(filepath.ToSlash(relative), "/")).To(HaveLen(4))
	})

	It("reuses one verified object and preserves its first retention receipt", func() {
		payload := []byte("same bytes")
		first, err := store.Retain(bytes.NewReader(payload), "tenant-a", "case-a")
		Expect(err).NotTo(HaveOccurred())
		second, err := store.Retain(bytes.NewReader(payload), "tenant-a", "case-a")
		Expect(err).NotTo(HaveOccurred())
		Expect(second.Path).To(Equal(first.Path))
		Expect(second.StorageURI).To(Equal(first.StorageURI))
		Expect(second.Reused).To(BeTrue())
		Expect(second.RetainedAt).To(Equal(first.RetainedAt))
		entries, err := os.ReadDir(filepath.Dir(first.Path))
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(HaveLen(1))
	})

	It("converges concurrent identical uploads without overwriting", func() {
		payload := []byte("concurrent immutable evidence")
		const workers = 12
		results := make(chan storedEvidenceObject, workers)
		errorsFound := make(chan error, workers)
		var group sync.WaitGroup
		for range workers {
			group.Add(1)
			go func() {
				defer GinkgoRecover()
				defer group.Done()
				stored, err := store.Retain(bytes.NewReader(payload), "tenant-a", "case-a")
				if err != nil {
					errorsFound <- err
					return
				}
				results <- stored
			}()
		}
		group.Wait()
		close(results)
		close(errorsFound)
		Expect(errorsFound).To(BeEmpty())
		created := 0
		paths := map[string]struct{}{}
		for stored := range results {
			if !stored.Reused {
				created++
			}
			paths[stored.Path] = struct{}{}
		}
		Expect(created).To(Equal(1))
		Expect(paths).To(HaveLen(1))
	})

	It("never replaces a corrupt object and quarantines the new good bytes", func() {
		payload := []byte("original evidence")
		stored, err := store.Retain(bytes.NewReader(payload), "tenant-a", "case-a")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chmod(stored.Path, 0o600)).To(Succeed())
		Expect(os.WriteFile(stored.Path, []byte("corrupt evidence"), 0o600)).To(Succeed())
		Expect(os.Chmod(stored.Path, 0o440)).To(Succeed())

		_, err = store.Retain(bytes.NewReader(payload), "tenant-a", "case-a")
		var retentionErr *evidenceRetentionError
		Expect(err).To(MatchError(ContainSubstring("incoming bytes retained for integrity review")))
		Expect(errors.As(err, &retentionErr)).To(BeTrue())
		Expect(os.ReadFile(stored.Path)).To(Equal([]byte("corrupt evidence")))
		Expect(retentionErr.quarantinePath).To(BeAnExistingFile())
		Expect(os.ReadFile(retentionErr.quarantinePath)).To(Equal(payload))
	})

	It("detects post-retention byte, size, receipt, and URI tampering", func() {
		payload := []byte("verified evidence")
		stored, err := store.Retain(bytes.NewReader(payload), "tenant-a", "case-a")
		Expect(err).NotTo(HaveOccurred())
		_, err = store.Verify(stored.StorageURI, int64(len(payload)+1))
		Expect(err).To(MatchError(ContainSubstring("size mismatch")))
		_, err = store.Verify(strings.Replace(stored.StorageURI, stored.ScopeKey, strings.Repeat("0", 64), 1), int64(len(payload)))
		Expect(err).To(HaveOccurred())

		receiptPath := store.receiptPath(stored.ScopeKey, stored.SHA256)
		Expect(os.Chmod(receiptPath, 0o600)).To(Succeed())
		_, err = store.Verify(stored.StorageURI, int64(len(payload)))
		Expect(err).To(MatchError(ContainSubstring("receipt is writable")))
		Expect(os.WriteFile(receiptPath, []byte(`{"layout_version":"tampered"}`), 0o600)).To(Succeed())
		Expect(os.Chmod(receiptPath, 0o440)).To(Succeed())
		_, err = store.Verify(stored.StorageURI, int64(len(payload)))
		Expect(err).To(HaveOccurred())
	})

	It("rejects symlink objects and non-canonical storage URIs", func() {
		payload := []byte("symlink target")
		digest := sha256.Sum256(payload)
		contentHash := hex.EncodeToString(digest[:])
		scopeKey := evidenceStorageScopeKey("tenant-a", "case-a")
		objectPath := store.objectPath(scopeKey, contentHash)
		Expect(os.MkdirAll(filepath.Dir(objectPath), 0o750)).To(Succeed())
		target := filepath.Join(root, "target")
		Expect(os.WriteFile(target, payload, 0o440)).To(Succeed())
		if err := os.Symlink(target, objectPath); err != nil {
			Skip("symlink creation is unavailable on this Windows account")
		}
		Expect(verifyEvidenceFile(objectPath, contentHash, int64(len(payload)))).To(MatchError(ContainSubstring("not a regular file")))
		_, _, err := parseEvidenceStorageURI("forensic-spool://sha256-scope-v1/../" + contentHash)
		Expect(err).To(HaveOccurred())
	})
})
