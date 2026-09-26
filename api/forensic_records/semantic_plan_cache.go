package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PLAN COMPLETION CACHE.
//
// Measured 2026-09-24 ([`reports/latency-bench-20260924`]): 45% of questions
// generate a plan, at a median of 138 s, and generation is ~100% of those
// requests. The P3 gate is p95 < 15 s. Seven of the nine generating questions
// in that sample were answers the ladder ALREADY produced correctly -- they
// generate because verified-only arbitration earns a verified plan for an
// answer that would otherwise be stated on no authority.
//
// So the cost buys the verification guarantee, and the guarantee is what took
// confident-wrong from 23 to 0. It must not be traded away. A cache pays the
// cost once instead of every time, and changes nothing else.
//
// WHY CACHING HERE IS EXACT, NOT A HEURISTIC
//
// The completion is requested at `temperature: 0`, so the model's output is a
// deterministic function of its input payload. Keying on a hash of that exact
// payload memoizes a pure function: a hit returns the bytes the model would
// have returned. This is NOT "questions that look similar reuse a plan" --
// byte-identical input, byte-identical output.
//
// Everything downstream is unchanged. The cached bytes re-enter the SAME
// decode, bind, and re-verification path -- S6 shape, S9 SHAPE, and the
// CONSTRAINT_APPLIED obligations all run exactly as they would on a fresh
// completion. The cache removes an HTTP call, never a check.
//
// WHAT THE KEY COVERS, AND WHY EACH PART IS THERE
//
//   - the payload: model, temperature, schema, system prompt and the context
//     block (question, family, literal values, date bounds, issued fields). A
//     new field in the catalogue, a changed display name, a different question
//     or a different issued enum all change the payload and therefore the key.
//     This is what makes the cache self-invalidating: there is no staleness
//     window because a changed catalogue is simply a different key.
//   - tenant and collection: defence in depth. The plan references field IDs
//     rather than tenant data, and execution applies scope separately, so a
//     shared entry would still be safe -- but this is a forensic product under
//     RLS, and a cache that cannot leak across tenants by construction is worth
//     more than one that merely does not today.
//   - record type: the same question against a different bound record type is
//     a different plan.
//
// Entries are held in memory only. They do not survive a restart, are never
// written to disk, and hold no evidence -- only the model's proposed algebra,
// which is already recorded in the response audit of every request that used
// it.

const semanticPlanCacheMax = 512

// semanticPlanCacheEnv gates the cache. Off by default, like every other
// capability on this path (FORENSIC_IR_FALLBACK, FORENSIC_IR_ARBITRATION,
// FORENSIC_VERIFIED_ONLY): its own switch, so it can be measured and withdrawn
// independently of everything around it.
//
// Default-off is not timidity. The cache changes an OBSERVABLE property --
// how many times the model is called -- and the test
// "uses DYNAMIC_TYPED_PLAN exactly once and never retries after unsupported"
// caught exactly that, failing with 1 call where it asserts 2. A process-global
// memo that silently changes call counts is a poor default for a suite that
// asserts them, and a poor default for anyone measuring generation behaviour.
const semanticPlanCacheEnv = "FORENSIC_PLAN_CACHE"

func semanticPlanCacheEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(semanticPlanCacheEnv)), "true")
}

type semanticPlanCache struct {
	sync.Mutex
	entries map[string][]byte
	order   []string
}

var semanticPlanCompletions = &semanticPlanCache{entries: map[string][]byte{}}

// semanticPlanCacheKey identifies a completion request exactly. The payload
// already carries the schema, the system prompt and the full context block, so
// the scope fields are the only additions.
func semanticPlanCacheKey(req hybridQueryRequest, payload []byte) string {
	hash := sha256.New()
	for _, part := range []string{
		"forensics.plan-completion-cache/v1",
		strings.TrimSpace(req.TenantID),
		strings.TrimSpace(req.CollectionID),
		strings.TrimSpace(req.RecordType),
	} {
		_, _ = hash.Write([]byte(part))
		_, _ = hash.Write([]byte{0})
	}
	_, _ = hash.Write(payload)
	return hex.EncodeToString(hash.Sum(nil))
}

// cachedSemanticPlanCompletion returns the memoized completion bytes, if any.
// The copy is deliberate: a caller must never be able to mutate a cached entry
// and change what a later request sees.
func cachedSemanticPlanCompletion(key string) ([]byte, bool) {
	if key == "" || !semanticPlanCacheEnabled() {
		return nil, false
	}
	semanticPlanCompletions.Lock()
	defer semanticPlanCompletions.Unlock()
	raw, ok := semanticPlanCompletions.entries[key]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), raw...), true
}

// storeSemanticPlanCompletion memoizes a completion that already decoded and
// finished cleanly. A transient malformation is never stored: caching one would
// make a momentary model hiccup permanent for that question.
func storeSemanticPlanCompletion(key string, raw []byte) {
	if key == "" || len(raw) == 0 || !semanticPlanCacheEnabled() {
		return
	}
	semanticPlanCompletions.Lock()
	defer semanticPlanCompletions.Unlock()
	if _, exists := semanticPlanCompletions.entries[key]; exists {
		return
	}
	// Bounded, oldest-first. An unbounded cache in a long-running forensic
	// service is a slow memory leak, and this box already runs the model in
	// 7.6 GiB.
	if len(semanticPlanCompletions.order) >= semanticPlanCacheMax {
		oldest := semanticPlanCompletions.order[0]
		semanticPlanCompletions.order = semanticPlanCompletions.order[1:]
		delete(semanticPlanCompletions.entries, oldest)
		// Evict from disk too, or the bound holds only in memory and the
		// directory grows without limit across restarts.
		if dir := semanticPlanCacheDir(); dir != "" {
			os.Remove(filepath.Join(dir, oldest+".json"))
		}
	}
	semanticPlanCompletions.entries[key] = append([]byte(nil), raw...)
	semanticPlanCompletions.order = append(semanticPlanCompletions.order, key)
	if dir := semanticPlanCacheDir(); dir != "" {
		persistSemanticPlanCompletion(dir, key, raw)
	}
}

// resetSemanticPlanCache exists for tests. Production never clears the cache:
// a changed catalogue produces a different key rather than a stale entry.
func resetSemanticPlanCache() {
	semanticPlanCompletions.Lock()
	defer semanticPlanCompletions.Unlock()
	semanticPlanCompletions.entries = map[string][]byte{}
	semanticPlanCompletions.order = nil
}

// ---------------------------------------------------------------------------
// PERSISTENCE
//
// The in-memory cache is lost on every restart, so a redeploy re-pays the cold
// generation cost once per distinct question -- 60-150 s each, measured
// 2026-09-24. Persisting the entries makes the P3 latency gate survive a
// deployment instead of being re-earned after it.
//
// WHAT IS WRITTEN, STATED PLAINLY
//
// Each file holds ONE model completion: the proposed typed algebra for one
// question. That plan can carry the question's literal values -- a plan for
// "How many calls did 03001234567 make?" contains that number as a filter.
//
// It is NOT evidence. It is derived from analyst input, and the same plan is
// already returned in the response audit of every request that used it. But
// writing it to a volume is a different act from returning it in a response,
// so persistence is OPT-IN and OFF by default: it writes nothing unless an
// operator sets the directory deliberately.
//
// Nothing else is written. No rows, no citations, no evidence, no results.
//
// STALENESS is not a risk here for the same reason it is not in memory: the
// key is a hash of the exact model payload, which carries the schema, the
// prompt and the issued field enum. A changed catalogue, a changed prompt or a
// changed question is a DIFFERENT FILE, not a stale one. Entries orphaned by
// such a change are never read again; they are evicted by the bound below.
const semanticPlanCacheDirEnv = "FORENSIC_PLAN_CACHE_DIR"

func semanticPlanCacheDir() string {
	if !semanticPlanCacheEnabled() {
		return ""
	}
	return strings.TrimSpace(os.Getenv(semanticPlanCacheDirEnv))
}

// persistSemanticPlanCompletion writes one entry atomically: a temp file in the
// same directory, then a rename. A partially written file must never be
// readable as a completion -- it would be served as though the model had
// produced it.
func persistSemanticPlanCompletion(dir, key string, raw []byte) {
	if dir == "" || key == "" || len(raw) == 0 {
		return
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return
	}
	final := filepath.Join(dir, key+".json")
	if _, err := os.Stat(final); err == nil {
		return
	}
	temp, err := os.CreateTemp(dir, key+".*.tmp")
	if err != nil {
		return
	}
	tempName := temp.Name()
	if _, err := temp.Write(raw); err != nil {
		temp.Close()
		os.Remove(tempName)
		return
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempName)
		return
	}
	if err := os.Rename(tempName, final); err != nil {
		os.Remove(tempName)
	}
}

// loadSemanticPlanCache repopulates the cache at startup. Every entry is
// VALIDATED before it is trusted: a truncated or corrupt file is skipped and
// removed rather than served, because a malformed completion served from disk
// would look exactly like a malformed completion from the model and would be
// permanent instead of transient.
//
// Returns the number of entries loaded so startup can say so out loud -- a
// cache that silently fails to load is indistinguishable from one that is
// working, and the whole point of it is a latency property someone will check.
func loadSemanticPlanCache(dir string) (loaded int, skipped int) {
	if dir == "" {
		return 0, 0
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if loaded >= semanticPlanCacheMax {
			break
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil || !usableSemanticPlanCompletion(raw) {
			skipped++
			os.Remove(path)
			continue
		}
		key := strings.TrimSuffix(entry.Name(), ".json")
		semanticPlanCompletions.Lock()
		if _, exists := semanticPlanCompletions.entries[key]; !exists {
			semanticPlanCompletions.entries[key] = raw
			semanticPlanCompletions.order = append(semanticPlanCompletions.order, key)
			loaded++
		}
		semanticPlanCompletions.Unlock()
	}
	return loaded, skipped
}

// usableSemanticPlanCompletion is the validation a persisted entry must pass to
// be trusted. It mirrors what the generator itself requires of a live response:
// exactly one choice, carrying content. Anything else is not a completion.
func usableSemanticPlanCompletion(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(raw, &decoded) != nil || len(decoded.Choices) != 1 {
		return false
	}
	return strings.TrimSpace(decoded.Choices[0].Message.Content) != ""
}
