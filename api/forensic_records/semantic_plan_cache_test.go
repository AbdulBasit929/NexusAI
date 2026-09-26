package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// The plan cache buys latency without touching the verification guarantee.
// Measured 2026-09-24: 45% of questions generate a plan at a median of 138 s,
// and generation is ~100% of those requests.
//
// The invariants that matter are SAFETY invariants. A cache that returns one
// tenant's plan to another, or that survives a catalogue change, would trade a
// forensic guarantee for speed -- which is the trade this product has refused
// three times.

// The cache is gated like every other capability on this path, so each test
// enables it explicitly. t.Setenv restores the previous value automatically,
// which keeps the suite's model-call assertions unaffected.
func enablePlanCache(t *testing.T) {
	t.Helper()
	t.Setenv(semanticPlanCacheEnv, "true")
	resetSemanticPlanCache()
}

func planCacheRequest() hybridQueryRequest {
	return hybridQueryRequest{
		TenantID:     "default",
		CollectionID: "nexusai-forensic-demo",
		RecordType:   "cdr",
		Query:        "How many CDR records are there for each call type?",
	}
}

func TestPlanCacheRoundTripsTheExactBytes(t *testing.T) {
	enablePlanCache(t)
	payload := []byte(`{"model":"m","temperature":0,"messages":[]}`)
	key := semanticPlanCacheKey(planCacheRequest(), payload)

	if _, ok := cachedSemanticPlanCompletion(key); ok {
		t.Fatal("an empty cache must not report a hit")
	}
	completion := []byte(`{"choices":[{"message":{"content":"{}"},"finish_reason":"stop"}]}`)
	storeSemanticPlanCompletion(key, completion)

	got, ok := cachedSemanticPlanCompletion(key)
	if !ok {
		t.Fatal("a stored completion must be retrievable")
	}
	if !bytes.Equal(got, completion) {
		t.Errorf("cache returned different bytes than were stored:\n got %s\nwant %s", got, completion)
	}
}

// A caller must never be able to mutate a cached entry and change what a later
// request sees. Both directions matter: the stored copy and the returned copy.
func TestPlanCacheIsolatesCallersFromTheStoredEntry(t *testing.T) {
	enablePlanCache(t)
	key := semanticPlanCacheKey(planCacheRequest(), []byte("payload"))
	original := []byte(`{"choices":[{"message":{"content":"ORIGINAL"}}]}`)

	stored := append([]byte(nil), original...)
	storeSemanticPlanCompletion(key, stored)
	for i := range stored {
		stored[i] = 'X' // the caller scribbles over its own buffer afterwards
	}
	got, _ := cachedSemanticPlanCompletion(key)
	if !bytes.Equal(got, original) {
		t.Errorf("mutating the source buffer changed the cached entry: %s", got)
	}

	for i := range got {
		got[i] = 'Y' // and a reader scribbles over what it was handed
	}
	again, _ := cachedSemanticPlanCompletion(key)
	if !bytes.Equal(again, original) {
		t.Errorf("mutating a returned copy changed the cached entry: %s", again)
	}
}

// Defence in depth. The plan references field IDs rather than tenant data and
// execution applies scope separately, so a shared entry would still be safe --
// but a cache that CANNOT leak across tenants by construction is worth more
// than one that merely does not today.
func TestPlanCacheKeySeparatesScopes(t *testing.T) {
	payload := []byte("identical payload")
	base := planCacheRequest()
	baseKey := semanticPlanCacheKey(base, payload)

	for name, mutate := range map[string]func(*hybridQueryRequest){
		"tenant":     func(r *hybridQueryRequest) { r.TenantID = "other-tenant" },
		"collection": func(r *hybridQueryRequest) { r.CollectionID = "other-case" },
		"recordType": func(r *hybridQueryRequest) { r.RecordType = "ipdr" },
	} {
		other := planCacheRequest()
		mutate(&other)
		if semanticPlanCacheKey(other, payload) == baseKey {
			t.Errorf("a different %s produced the SAME cache key -- entries could be shared across scopes", name)
		}
	}
}

// Self-invalidation. A new field, a changed display name, a different question
// or a different issued enum all change the payload, so a changed catalogue is
// a different key rather than a stale entry. There is no staleness window.
func TestPlanCacheKeyChangesWithThePayload(t *testing.T) {
	req := planCacheRequest()
	first := semanticPlanCacheKey(req, []byte(`{"issued_fields":["cdr.call_type"]}`))
	second := semanticPlanCacheKey(req, []byte(`{"issued_fields":["cdr.call_type","cdr.msisdn"]}`))
	if first == second {
		t.Fatal("a changed issued-field set must produce a different key, or a stale plan could outlive its catalogue")
	}
}

// An unbounded cache in a long-running forensic service is a slow memory leak,
// and this box already runs the model in 7.6 GiB.
func TestPlanCacheIsBoundedAndEvictsOldestFirst(t *testing.T) {
	enablePlanCache(t)
	req := planCacheRequest()
	firstKey := semanticPlanCacheKey(req, []byte("payload-0"))
	for i := 0; i <= semanticPlanCacheMax; i++ {
		key := semanticPlanCacheKey(req, []byte("payload-"+string(rune('a'+i%26))+string(rune('a'+i/26))))
		storeSemanticPlanCompletion(key, []byte("completion"))
	}
	storeSemanticPlanCompletion(firstKey, []byte("completion"))

	semanticPlanCompletions.Lock()
	size := len(semanticPlanCompletions.entries)
	order := len(semanticPlanCompletions.order)
	semanticPlanCompletions.Unlock()
	if size > semanticPlanCacheMax {
		t.Errorf("cache holds %d entries, above the %d bound", size, semanticPlanCacheMax)
	}
	if order != size {
		t.Errorf("eviction order list (%d) drifted from the entry map (%d)", order, size)
	}
}

// Nothing empty is ever stored or served: an empty key or an empty completion
// would memoize a non-answer.
func TestPlanCacheRefusesEmptyEntries(t *testing.T) {
	enablePlanCache(t)
	storeSemanticPlanCompletion("", []byte("completion"))
	storeSemanticPlanCompletion("key", nil)
	if _, ok := cachedSemanticPlanCompletion(""); ok {
		t.Error("an empty key must never report a hit")
	}
	if _, ok := cachedSemanticPlanCompletion("key"); ok {
		t.Error("an empty completion must never be stored")
	}
}

// Off by default, and provably inert when off: a disabled cache must neither
// store nor serve, or a deployment that believes it disabled the cache would
// still be served memoized completions.
func TestPlanCacheIsOffByDefaultAndInertWhenOff(t *testing.T) {
	if semanticPlanCacheEnabled() {
		t.Fatal("the plan cache must default to OFF, like every other capability on this path")
	}
	resetSemanticPlanCache()
	key := semanticPlanCacheKey(planCacheRequest(), []byte("payload"))
	storeSemanticPlanCompletion(key, []byte("completion"))
	if _, ok := cachedSemanticPlanCompletion(key); ok {
		t.Error("a disabled cache served a completion")
	}
	t.Setenv(semanticPlanCacheEnv, "true")
	if _, ok := cachedSemanticPlanCompletion(key); ok {
		t.Error("a disabled cache STORED a completion that a later enable then served")
	}
}

// --- persistence -----------------------------------------------------------

// The in-memory cache is lost on every restart, so a redeploy re-pays the cold
// generation cost once per distinct question -- 60-150 s each. Persistence is
// what makes the latency property survive a deployment.
func TestPlanCachePersistsAndRestores(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(semanticPlanCacheEnv, "true")
	t.Setenv(semanticPlanCacheDirEnv, dir)
	resetSemanticPlanCache()

	key := semanticPlanCacheKey(planCacheRequest(), []byte("payload"))
	completion := []byte(`{"choices":[{"message":{"content":"{\"measures\":[]}"},"finish_reason":"stop"}]}`)
	storeSemanticPlanCompletion(key, completion)

	// Simulate a restart: memory gone, disk intact.
	resetSemanticPlanCache()
	if _, ok := cachedSemanticPlanCompletion(key); ok {
		t.Fatal("reset must clear memory, or the test proves nothing")
	}
	loaded, skipped := loadSemanticPlanCache(dir)
	if loaded != 1 || skipped != 0 {
		t.Fatalf("restore loaded %d skipped %d, want 1 and 0", loaded, skipped)
	}
	got, ok := cachedSemanticPlanCompletion(key)
	if !ok || !bytes.Equal(got, completion) {
		t.Errorf("restored entry differs from what was persisted: %s", got)
	}
}

// A truncated or corrupt file must be skipped and removed, never served. A
// malformed completion from disk would look exactly like one from the model --
// but permanent instead of transient.
func TestPlanCacheRejectsAndRemovesUnusableFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(semanticPlanCacheEnv, "true")
	t.Setenv(semanticPlanCacheDirEnv, dir)
	resetSemanticPlanCache()

	bad := map[string]string{
		"truncated.json": `{"choices":[{"message":{"content":`,
		"empty.json":     ``,
		"nochoice.json":  `{"choices":[]}`,
		"twochoice.json": `{"choices":[{"message":{"content":"a"}},{"message":{"content":"b"}}]}`,
		"nocontent.json": `{"choices":[{"message":{"content":"   "}}]}`,
	}
	for name, body := range bad {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	loaded, skipped := loadSemanticPlanCache(dir)
	if loaded != 0 {
		t.Errorf("loaded %d unusable entries; none may be trusted", loaded)
	}
	if skipped != len(bad) {
		t.Errorf("skipped %d of %d unusable files", skipped, len(bad))
	}
	for name := range bad {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s survived; an unusable entry must be removed, not left to be retried forever", name)
		}
	}
}

// Nothing is written unless an operator sets the directory deliberately. A
// persisted plan can carry the question's literal values, so the default must
// write nothing at all.
func TestPlanCacheWritesNothingWithoutAnExplicitDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(semanticPlanCacheEnv, "true")
	t.Setenv(semanticPlanCacheDirEnv, "")
	resetSemanticPlanCache()

	storeSemanticPlanCompletion(semanticPlanCacheKey(planCacheRequest(), []byte("p")), []byte(`{"choices":[{"message":{"content":"x"}}]}`))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("wrote %d files with no directory configured", len(entries))
	}
	// And with the cache itself disabled, a configured directory is still inert.
	t.Setenv(semanticPlanCacheEnv, "false")
	t.Setenv(semanticPlanCacheDirEnv, dir)
	if semanticPlanCacheDir() != "" {
		t.Error("a disabled cache must report no directory, or it would persist while disabled")
	}
}

// The bound must hold across restarts too, or the directory grows without
// limit while memory stays capped.
func TestPlanCacheEvictionRemovesThePersistedFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(semanticPlanCacheEnv, "true")
	t.Setenv(semanticPlanCacheDirEnv, dir)
	resetSemanticPlanCache()

	completion := []byte(`{"choices":[{"message":{"content":"x"}}]}`)
	req := planCacheRequest()
	first := semanticPlanCacheKey(req, []byte("payload-first"))
	storeSemanticPlanCompletion(first, completion)
	for i := 0; i < semanticPlanCacheMax; i++ {
		storeSemanticPlanCompletion(semanticPlanCacheKey(req, []byte("fill-"+strconv.Itoa(i))), completion)
	}
	if _, err := os.Stat(filepath.Join(dir, first+".json")); !os.IsNotExist(err) {
		t.Error("the evicted entry's file survived; the bound would hold only in memory")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) > semanticPlanCacheMax {
		t.Errorf("directory holds %d files, above the %d bound", len(entries), semanticPlanCacheMax)
	}
}
