package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTileCache(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "tilecache suite")
}

var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), []byte("pretend this is a tile")...)

// fakeUpstream stands in for the shared tile server and counts what is asked of it.
type fakeUpstream struct {
	srv         *httptest.Server
	tileCalls   atomic.Int64
	jsonCalls   atomic.Int64
	mu          sync.Mutex
	inFlight    int
	maxInFlight int
	delay       time.Duration
	status      int
	body        []byte
	retryAfter  string
	lastAgent   atomic.Value
}

func newFakeUpstream() *fakeUpstream {
	u := &fakeUpstream{status: http.StatusOK, body: pngBytes}
	u.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/data/pakistan.json" {
			u.jsonCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tiles":["http://private.example/data/pakistan/{z}/{x}/{y}.png"],"name":"Unnamed map","format":"png","basename":"pakistan_master.mbtiles","id":"pakistan_master","bounds":[45,21.94304553,90,40.97989807],"minzoom":4,"maxzoom":18,"tilejson":"3.0.0"}`))
			return
		}
		u.tileCalls.Add(1)
		u.lastAgent.Store(r.Header.Get("User-Agent"))
		u.mu.Lock()
		u.inFlight++
		u.maxInFlight = max(u.maxInFlight, u.inFlight)
		delay, status, body, retry := u.delay, u.status, u.body, u.retryAfter
		u.mu.Unlock()
		time.Sleep(delay)
		u.mu.Lock()
		u.inFlight--
		u.mu.Unlock()
		if retry != "" {
			w.Header().Set("Retry-After", retry)
		}
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	return u
}

func (u *fakeUpstream) set(f func(u *fakeUpstream)) {
	u.mu.Lock()
	defer u.mu.Unlock()
	f(u)
}

func newTestServer(u *fakeUpstream, tweak func(*Config)) *Server {
	cfg := DefaultConfig()
	cfg.Upstream = u.srv.URL + "/data"
	cfg.Sets = []string{"pakistan"}
	cfg.CacheDir = GinkgoT().TempDir()
	cfg.Concurrency = 3
	cfg.Timeout = 3 * time.Second
	cfg.BreakerOpen = 400 * time.Millisecond
	if tweak != nil {
		tweak(&cfg)
	}
	s, err := NewServer(cfg)
	Expect(err).NotTo(HaveOccurred())
	return s
}

func get(s *Server, path string, header ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// Tiles of the 7th zoom level that cover Pakistan: x 87 to 89, y 52 to 54.
func tileURL(x, y int) string { return fmt.Sprintf("/tiles/pakistan/7/%d/%d.png", x, y) }

var _ = Describe("tilecache", func() {
	var up *fakeUpstream
	BeforeEach(func() { up = newFakeUpstream() })
	AfterEach(func() { up.srv.Close() })

	It("fetches a tile once and then serves it from disk", func() {
		s := newTestServer(up, nil)
		first := get(s, tileURL(88, 53))
		second := get(s, tileURL(88, 53))
		Expect(first.Code).To(Equal(http.StatusOK))
		Expect(first.Header().Get("X-Cache")).To(Equal("MISS"))
		Expect(second.Code).To(Equal(http.StatusOK))
		Expect(second.Header().Get("X-Cache")).To(Equal("HIT"))
		Expect(second.Body.Bytes()).To(Equal(pngBytes))
		Expect(second.Header().Get("Content-Type")).To(Equal("image/png"))
		Expect(second.Header().Get("Cache-Control")).To(ContainSubstring("max-age"))
		Expect(second.Header().Get("ETag")).NotTo(BeEmpty())
		Expect(up.tileCalls.Load()).To(BeEquivalentTo(1))
	})

	It("asks the upstream once when many browsers ask for the same tile at the same moment", func() {
		up.set(func(u *fakeUpstream) { u.delay = 150 * time.Millisecond })
		s := newTestServer(up, nil)
		var wg sync.WaitGroup
		codes := make([]int, 20)
		for i := range codes {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes[i] = get(s, tileURL(88, 53)).Code
			}()
		}
		wg.Wait()
		for _, c := range codes {
			Expect(c).To(Equal(http.StatusOK))
		}
		Expect(up.tileCalls.Load()).To(BeEquivalentTo(1))
		Expect(s.Stats.Coalesced.Load()).To(BeNumerically(">=", 1))
	})

	It("never has more requests in flight at the upstream than its limit", func() {
		up.set(func(u *fakeUpstream) { u.delay = 80 * time.Millisecond })
		s := newTestServer(up, nil) // a limit of three
		var wg sync.WaitGroup
		var failed atomic.Int64
		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if get(s, tileURL(87+i%3, 52+i/3%3)).Code != http.StatusOK {
					failed.Add(1)
				}
			}()
		}
		wg.Wait()
		Expect(failed.Load()).To(BeZero())
		up.mu.Lock()
		defer up.mu.Unlock()
		Expect(up.maxInFlight).To(BeNumerically("<=", 3))
		Expect(up.maxInFlight).To(BeNumerically(">=", 2), "the test really did run requests side by side")
	})

	It("remembers that a tile does not exist and does not ask again", func() {
		up.set(func(u *fakeUpstream) { u.status = http.StatusNotFound })
		s := newTestServer(up, nil)
		Expect(get(s, tileURL(88, 53)).Code).To(Equal(http.StatusNotFound))
		Expect(get(s, tileURL(88, 53)).Code).To(Equal(http.StatusNotFound))
		Expect(up.tileCalls.Load()).To(BeEquivalentTo(1))
		Expect(s.Stats.NegativeHits.Load()).To(BeEquivalentTo(1))
	})

	DescribeTable("refuses what the upstream does not serve without asking it",
		func(path string, wanted int) {
			s := newTestServer(up, nil)
			Expect(get(s, "/tiles/pakistan/7/88/53.png").Code).To(Equal(http.StatusOK)) // the description of the set is read here
			calls := up.tileCalls.Load()
			Expect(get(s, path).Code).To(Equal(wanted))
			Expect(up.tileCalls.Load()).To(Equal(calls))
		},
		Entry("a set that is not allowed", "/tiles/other/7/88/53.png", http.StatusNotFound),
		Entry("a zoom level below the set's, over Pakistan", "/tiles/pakistan/3/5/3.png", http.StatusNotFound),
		Entry("a zoom level above the set's, over Pakistan", "/tiles/pakistan/19/361000/217500.png", http.StatusNotFound),
		Entry("a column that does not exist at that zoom level", "/tiles/pakistan/7/900/53.png", http.StatusBadRequest),
		Entry("a row that is not a number", "/tiles/pakistan/7/88/abc.png", http.StatusBadRequest),
		Entry("a negative column", "/tiles/pakistan/7/-1/53.png", http.StatusBadRequest),
		Entry("a file type that is not an image", "/tiles/pakistan/7/88/53.gif", http.StatusBadRequest),
		Entry("a tile outside the area the set covers", "/tiles/pakistan/7/10/10.png", http.StatusNotFound),
	)

	It("does not let a request leave the cache folder", func() {
		s := newTestServer(up, nil)
		Expect(get(s, "/tiles/pakistan/7/88/53.png").Code).To(Equal(http.StatusOK))
		calls := up.tileCalls.Load()
		for _, bad := range []string{"/tiles/pakistan/7/../../88/53.png", "/tiles/pakistan/7/88/%2e%2e%2f53.png", "/tiles/%2e%2e/7/88/53.png"} {
			Expect(get(s, bad).Code).NotTo(Equal(http.StatusOK), bad)
		}
		Expect(up.tileCalls.Load()).To(Equal(calls))
	})

	It("serves an old tile when the upstream fails, and refreshes it when the upstream is back", func() {
		s := newTestServer(up, nil)
		Expect(get(s, tileURL(88, 53)).Code).To(Equal(http.StatusOK))
		old := time.Now().Add(-40 * 24 * time.Hour)
		Expect(os.Chtimes(s.tilePath("pakistan", 7, 88, 53, "png"), old, old)).To(Succeed())

		up.set(func(u *fakeUpstream) { u.status = http.StatusInternalServerError })
		stale := get(s, tileURL(88, 53))
		Expect(stale.Code).To(Equal(http.StatusOK))
		Expect(stale.Header().Get("X-Cache")).To(Equal("STALE"))
		Expect(stale.Body.Bytes()).To(Equal(pngBytes))

		fresh := append([]byte("\x89PNG\r\n\x1a\n"), []byte("a newer tile")...)
		up.set(func(u *fakeUpstream) { u.status, u.body = http.StatusOK, fresh })
		Eventually(func() string { return get(s, tileURL(88, 53)).Header().Get("X-Cache") }, 2*time.Second, 50*time.Millisecond).Should(Equal("HIT"))
		Expect(get(s, tileURL(88, 53)).Body.Bytes()).To(Equal(fresh))
	})

	It("leaves an upstream that keeps failing alone for a while, then tries again", func() {
		up.set(func(u *fakeUpstream) { u.status = http.StatusInternalServerError })
		s := newTestServer(up, nil) // five failures in a row open the breaker for 400 ms
		for i := 0; i < 5; i++ {
			Expect(get(s, tileURL(87+i%3, 52+i/3)).Code).To(Equal(http.StatusBadGateway))
		}
		Expect(up.tileCalls.Load()).To(BeEquivalentTo(5))
		rejected := get(s, tileURL(89, 54))
		Expect(rejected.Code).To(Equal(http.StatusServiceUnavailable))
		Expect(rejected.Header().Get("Retry-After")).NotTo(BeEmpty())
		Expect(up.tileCalls.Load()).To(BeEquivalentTo(5), "the open breaker sent nothing to the upstream")
		up.set(func(u *fakeUpstream) { u.status = http.StatusOK })
		Eventually(func() int { return get(s, tileURL(89, 54)).Code }, 3*time.Second, 100*time.Millisecond).Should(Equal(http.StatusOK))
	})

	It("honours a Retry-After from an upstream that says it is busy", func() {
		up.set(func(u *fakeUpstream) { u.status, u.retryAfter = http.StatusTooManyRequests, "2" })
		s := newTestServer(up, nil)
		first := get(s, tileURL(88, 53))
		Expect(first.Code).To(Equal(http.StatusServiceUnavailable))
		Expect(first.Header().Get("Retry-After")).To(Equal("2"))
		Expect(get(s, tileURL(88, 54)).Code).To(Equal(http.StatusServiceUnavailable))
		Expect(up.tileCalls.Load()).To(BeEquivalentTo(1), "one busy answer was enough to stop asking")
	})

	It("does not keep what is not an image", func() {
		up.set(func(u *fakeUpstream) { u.body = []byte("<html>please sign in</html>") })
		s := newTestServer(up, nil)
		Expect(get(s, tileURL(88, 53)).Code).To(Equal(http.StatusBadGateway))
		Expect(s.tilePath("pakistan", 7, 88, 53, "png")).NotTo(BeAnExistingFile())
		Expect(get(s, tileURL(88, 53)).Code).To(Equal(http.StatusBadGateway))
		Expect(up.tileCalls.Load()).To(BeEquivalentTo(2))
		Expect(s.Stats.InvalidUpstream.Load()).To(BeEquivalentTo(2))
	})

	It("answers 304 when the browser already has the tile", func() {
		s := newTestServer(up, nil)
		first := get(s, tileURL(88, 53))
		again := get(s, tileURL(88, 53), "If-None-Match", first.Header().Get("ETag"))
		Expect(again.Code).To(Equal(http.StatusNotModified))
		Expect(again.Body.Len()).To(BeZero())
	})

	It("keeps the cache inside its size limit by removing the tiles fetched longest ago", func() {
		clock := time.Now()
		s := newTestServer(up, func(c *Config) { c.MaxCacheBytes = 3 * int64(len(pngBytes)) })
		s.now = func() time.Time { clock = clock.Add(time.Minute); return clock }
		for x := 87; x <= 89; x++ {
			for y := 52; y <= 53; y++ {
				Expect(get(s, tileURL(x, y)).Code).To(Equal(http.StatusOK))
			}
		}
		removed, size := s.Sweep()
		Expect(removed).To(BeNumerically(">=", 3))
		Expect(size).To(BeNumerically("<=", 3*int64(len(pngBytes))))
		Expect(s.tilePath("pakistan", 7, 87, 52, "png")).NotTo(BeAnExistingFile(), "the oldest went first")
		Expect(s.tilePath("pakistan", 7, 89, 53, "png")).To(BeAnExistingFile(), "the newest stayed")
	})

	It("describes a set with its own addresses, so that browsers never ask the shared server", func() {
		s := newTestServer(up, nil)
		req := httptest.NewRequest(http.MethodGet, "/tiles/pakistan.json", nil)
		req.Host = "maps.internal.example"
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusOK))
		var doc map[string]any
		Expect(json.Unmarshal(rec.Body.Bytes(), &doc)).To(Succeed())
		Expect(doc["tiles"]).To(ConsistOf("http://maps.internal.example/tiles/pakistan/{z}/{x}/{y}.png"))
		Expect(doc).To(HaveKey("bounds"))
		Expect(doc["minzoom"]).To(BeEquivalentTo(4))
		Expect(doc["maxzoom"]).To(BeEquivalentTo(18))
		Expect(doc).NotTo(HaveKey("basename"))
		Expect(rec.Body.String()).NotTo(ContainSubstring("private.example"))
	})

	It("reads the description of a set once while it is fresh", func() {
		s := newTestServer(up, nil)
		for x := 87; x <= 89; x++ {
			Expect(get(s, tileURL(x, 53)).Code).To(Equal(http.StatusOK))
		}
		Expect(get(s, "/tiles/pakistan.json").Code).To(Equal(http.StatusOK))
		Expect(up.jsonCalls.Load()).To(BeEquivalentTo(1))
	})

	It("names itself to the operators of the upstream", func() {
		s := newTestServer(up, func(c *Config) { c.UserAgent = "NexusAI-tilecache/test (contact: maps team)" })
		Expect(get(s, tileURL(88, 53)).Code).To(Equal(http.StatusOK))
		Expect(up.lastAgent.Load()).To(Equal("NexusAI-tilecache/test (contact: maps team)"))
	})

	It("only answers GET", func() {
		s := newTestServer(up, nil)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, tileURL(88, 53), nil))
		Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
		Expect(up.tileCalls.Load()).To(BeZero())
	})

	It("reports what it did", func() {
		s := newTestServer(up, nil)
		get(s, tileURL(88, 53))
		get(s, tileURL(88, 53))
		var stats map[string]any
		Expect(json.Unmarshal(get(s, "/stats").Body.Bytes(), &stats)).To(Succeed())
		Expect(stats["requests"]).To(BeEquivalentTo(2))
		Expect(stats["cache_hits"]).To(BeEquivalentTo(1))
		Expect(stats["upstream_fetches"]).To(BeEquivalentTo(1))
		Expect(stats["breaker_open"]).To(BeFalse())
	})

	DescribeTable("refuses a configuration that could be dangerous or useless",
		func(tweak func(*Config)) {
			cfg := DefaultConfig()
			cfg.Upstream, cfg.Sets, cfg.CacheDir = up.srv.URL+"/data", []string{"pakistan"}, GinkgoT().TempDir()
			tweak(&cfg)
			_, err := NewServer(cfg)
			Expect(err).To(HaveOccurred())
		},
		Entry("an upstream that is not http", func(c *Config) { c.Upstream = "ftp://tiles.example/data" }),
		Entry("no upstream", func(c *Config) { c.Upstream = "" }),
		Entry("no sets", func(c *Config) { c.Sets = nil }),
		Entry("a set name that could leave the cache folder", func(c *Config) { c.Sets = []string{"../x"} }),
		Entry("no cache folder", func(c *Config) { c.CacheDir = "" }),
	)

	It("keeps the files of different tiles apart", func() {
		s := newTestServer(up, nil)
		get(s, tileURL(87, 52))
		get(s, tileURL(88, 53))
		var files []string
		Expect(filepath.WalkDir(s.cfg.CacheDir, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				files = append(files, strings.TrimPrefix(filepath.ToSlash(p), filepath.ToSlash(s.cfg.CacheDir)+"/"))
			}
			return nil
		})).To(Succeed())
		Expect(files).To(ConsistOf("pakistan/7/87/52.png", "pakistan/7/88/53.png"))
		Expect(strconv.Itoa(len(files))).To(Equal("2"))
	})
})
