package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mudler/LocalAI/pkg/httpclient"
)

// Config says where the tiles come from and how politely they are fetched. The upstream is a tile server shared with other applications of
// the organisation, so every setting that follows is there to keep the load on it small: tiles are kept on disk and fetched once, the same
// tile asked for by many browsers is fetched once, only a few requests are in flight at a time, a server that is failing is left alone for a
// while, and nothing outside the sets, zoom levels and bounds the upstream declares is ever requested.
type Config struct {
	Upstream        string        // base URL under which every set is served, for example http://HOST:PORT/data (no trailing slash)
	Sets            []string      // the tile sets this proxy may serve
	CacheDir        string        // tiles are kept here, one file each
	MaxCacheBytes   int64         // the oldest tiles are removed when the cache grows beyond this
	TTL             time.Duration // how long a cached tile counts as fresh; a stale one is still served if the upstream fails
	NegativeTTL     time.Duration // how long "this tile does not exist" is remembered
	Concurrency     int           // most upstream requests at the same time
	Timeout         time.Duration // one upstream request
	UserAgent       string        // names us to the upstream's operators
	AllowOrigin     string        // Access-Control-Allow-Origin; empty sends none (same-origin use)
	BreakerFailures int           // this many failures in a row stop requests to the upstream for BreakerOpen
	BreakerOpen     time.Duration
	MetaTTL         time.Duration // how long a set's TileJSON is trusted
	DefaultMinZoom  int           // used until the TileJSON has been read
	DefaultMaxZoom  int
}

// DefaultConfig returns the settings used when nothing else is said.
func DefaultConfig() Config {
	return Config{
		MaxCacheBytes: 2 << 30, TTL: 30 * 24 * time.Hour, NegativeTTL: 5 * time.Minute, Concurrency: 6, Timeout: 15 * time.Second,
		UserAgent: "NexusAI-tilecache/1.0", BreakerFailures: 5, BreakerOpen: 30 * time.Second, MetaTTL: time.Hour, DefaultMinZoom: 0, DefaultMaxZoom: 22,
	}
}

const (
	maxZoomLimit = 22
	maxTileBytes = 4 << 20
	maxMetaBytes = 1 << 20
)

type setMeta struct {
	minZoom, maxZoom int
	bounds           [4]float64 // west, south, east, north
	hasBounds        bool
	format           string
	raw              map[string]any
	fetched          time.Time
	tried            time.Time
}

type flight struct {
	done chan struct{}
	res  tileResult
}

type tileResult struct {
	data       []byte
	status     int
	stale      bool
	source     string // hit, miss, coalesced or stale: what the answer came from
	modTime    time.Time
	retryAfter time.Duration
	err        error
}

// Stats are counters of what the proxy did, served at /stats.
type Stats struct {
	Requests, Hits, Fetches, Coalesced, StaleServed, NegativeHits atomic.Int64
	UpstreamOK, UpstreamErrors, BreakerRejected, InvalidUpstream  atomic.Int64
	Refused, NotModified, BytesServed                             atomic.Int64
	CacheBytes                                                    atomic.Int64
}

// Server is the caching proxy.
type Server struct {
	cfg    Config
	client *http.Client
	sem    chan struct{}
	now    func() time.Time
	sets   map[string]bool

	mu        sync.Mutex
	flights   map[string]*flight
	negative  map[string]time.Time
	meta      map[string]*setMeta
	failures  int
	openUntil time.Time

	Stats Stats
}

// NewServer checks the configuration and builds a server. It does not touch the network.
func NewServer(cfg Config) (*Server, error) {
	up, err := url.Parse(cfg.Upstream)
	if err != nil || (up.Scheme != "http" && up.Scheme != "https") || up.Host == "" {
		return nil, fmt.Errorf("upstream %q is not an http or https URL", cfg.Upstream)
	}
	if len(cfg.Sets) == 0 {
		return nil, errors.New("at least one tile set is needed")
	}
	if cfg.CacheDir == "" {
		return nil, errors.New("a cache directory is needed")
	}
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}
	s := &Server{cfg: cfg, client: httpclient.NewWithTimeout(cfg.Timeout), sem: make(chan struct{}, cfg.Concurrency), now: time.Now,
		sets: map[string]bool{}, flights: map[string]*flight{}, negative: map[string]time.Time{}, meta: map[string]*setMeta{}}
	s.cfg.Upstream = strings.TrimRight(cfg.Upstream, "/")
	for _, set := range cfg.Sets {
		if !validName(set) {
			return nil, fmt.Errorf("tile set name %q is not allowed", set)
		}
		s.sets[set] = true
	}
	return s, os.MkdirAll(cfg.CacheDir, 0o755)
}

func validName(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// Handler returns the HTTP handler: /tiles/{set}.json, /tiles/{set}/{z}/{x}/{y}.png, /healthz and /stats.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tiles/{set}/{z}/{x}/{file}", s.handleTile)
	mux.HandleFunc("GET /tiles/{file}", s.handleTileJSON)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok\n") })
	mux.HandleFunc("GET /stats", s.handleStats)
	return mux
}

// ---- requests --------------------------------------------------------------------------------------------------------------------------------

func parseCoord(text string) (int, bool) {
	if text == "" || len(text) > 9 {
		return 0, false
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(text)
	return n, err == nil
}

func contentTypeOf(ext string) string {
	switch ext {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	}
	return "image/png"
}

func (s *Server) cors(w http.ResponseWriter) {
	if s.cfg.AllowOrigin != "" {
		w.Header().Set("Access-Control-Allow-Origin", s.cfg.AllowOrigin)
	}
}

func (s *Server) refuse(w http.ResponseWriter, code int, why string) {
	s.Stats.Refused.Add(1)
	s.cors(w)
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.Error(w, why, code)
}

func (s *Server) handleTile(w http.ResponseWriter, r *http.Request) {
	s.Stats.Requests.Add(1)
	set := r.PathValue("set")
	if !s.sets[set] {
		s.refuse(w, http.StatusNotFound, "unknown tile set")
		return
	}
	file := r.PathValue("file")
	dot := strings.LastIndexByte(file, '.')
	if dot < 1 {
		s.refuse(w, http.StatusBadRequest, "a tile is named y.png")
		return
	}
	ext := strings.ToLower(file[dot+1:])
	if ext != "png" && ext != "jpg" && ext != "jpeg" && ext != "webp" {
		s.refuse(w, http.StatusBadRequest, "unsupported tile format")
		return
	}
	z, okZ := parseCoord(r.PathValue("z"))
	x, okX := parseCoord(r.PathValue("x"))
	y, okY := parseCoord(file[:dot])
	if !okZ || !okX || !okY || z > maxZoomLimit || x >= 1<<z || y >= 1<<z {
		s.refuse(w, http.StatusBadRequest, "tile coordinates out of range")
		return
	}
	meta := s.metaFor(r.Context(), set)
	if z < meta.minZoom || z > meta.maxZoom {
		s.refuse(w, http.StatusNotFound, "zoom level not served")
		return
	}
	if meta.hasBounds && !tileIntersects(z, x, y, meta.bounds) {
		s.refuse(w, http.StatusNotFound, "outside the area covered")
		return
	}
	res := s.getTile(r.Context(), set, z, x, y, ext)
	s.cors(w)
	if res.status == http.StatusOK {
		etag := fmt.Sprintf(`"%x-%x"`, len(res.data), res.modTime.UnixNano())
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "public, max-age=604800, stale-while-revalidate=86400")
		w.Header().Set("X-Cache", strings.ToUpper(res.source))
		if res.stale {
			w.Header().Set("Cache-Control", "public, max-age=60")
			s.Stats.StaleServed.Add(1)
		}
		if r.Header.Get("If-None-Match") == etag {
			s.Stats.NotModified.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", contentTypeOf(ext))
		w.Header().Set("Content-Length", strconv.Itoa(len(res.data)))
		n, _ := w.Write(res.data)
		s.Stats.BytesServed.Add(int64(n))
		return
	}
	if res.retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(res.retryAfter.Seconds()))))
	}
	if res.status == http.StatusNotFound {
		w.Header().Set("Cache-Control", "public, max-age=300")
	} else {
		w.Header().Set("Cache-Control", "no-store")
	}
	http.Error(w, http.StatusText(res.status), res.status)
}

func (s *Server) handleTileJSON(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	set, ok := strings.CutSuffix(file, ".json")
	if !ok || !s.sets[set] {
		s.refuse(w, http.StatusNotFound, "unknown tile set")
		return
	}
	meta := s.metaFor(r.Context(), set)
	if meta.raw == nil {
		s.refuse(w, http.StatusBadGateway, "the tile server's description is not available")
		return
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	out := make(map[string]any, len(meta.raw)+2)
	for k, v := range meta.raw {
		if k != "basename" && k != "tiles" {
			out[k] = v
		}
	}
	out["tiles"] = []string{fmt.Sprintf("%s://%s/tiles/%s/{z}/{x}/{y}.%s", scheme, r.Host, set, meta.format)}
	out["scheme"] = "xyz"
	s.cors(w)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *Server) handleStats(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	open := s.now().Before(s.openUntil)
	failures := s.failures
	s.mu.Unlock()
	st := &s.Stats
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"requests": st.Requests.Load(), "cache_hits": st.Hits.Load(), "upstream_fetches": st.Fetches.Load(), "coalesced": st.Coalesced.Load(),
		"stale_served": st.StaleServed.Load(), "negative_hits": st.NegativeHits.Load(), "not_modified": st.NotModified.Load(),
		"upstream_ok": st.UpstreamOK.Load(), "upstream_errors": st.UpstreamErrors.Load(), "breaker_rejected": st.BreakerRejected.Load(),
		"invalid_upstream": st.InvalidUpstream.Load(), "refused": st.Refused.Load(), "bytes_served": st.BytesServed.Load(), "cache_bytes": st.CacheBytes.Load(),
		"breaker_open": open, "consecutive_failures": failures,
	})
}

// ---- the tile set's own description -----------------------------------------------------------------------------------------------------

// metaFor returns what the upstream says about a set (zoom range, bounds, format), reading it at most once per MetaTTL and at most once per
// half minute when it cannot be read; until it has been read the configured defaults apply.
func (s *Server) metaFor(ctx context.Context, set string) *setMeta {
	s.mu.Lock()
	m := s.meta[set]
	if m == nil {
		m = &setMeta{minZoom: s.cfg.DefaultMinZoom, maxZoom: s.cfg.DefaultMaxZoom, format: "png"}
		s.meta[set] = m
	}
	now := s.now()
	due := (m.fetched.IsZero() || now.Sub(m.fetched) > s.cfg.MetaTTL) && now.Sub(m.tried) > 30*time.Second && now.After(s.openUntil)
	if due {
		m.tried = now
	}
	s.mu.Unlock()
	if !due {
		return s.copyMeta(set)
	}
	raw, err := s.fetchJSON(ctx, s.cfg.Upstream+"/"+set+".json")
	if err == nil {
		s.mu.Lock()
		m.raw, m.fetched = raw, now
		if v, ok := raw["minzoom"].(float64); ok {
			m.minZoom = int(v)
		}
		if v, ok := raw["maxzoom"].(float64); ok && int(v) <= maxZoomLimit {
			m.maxZoom = int(v)
		}
		if b, ok := raw["bounds"].([]any); ok && len(b) == 4 {
			var bb [4]float64
			good := true
			for i := range bb {
				f, isNum := b[i].(float64)
				good = good && isNum
				bb[i] = f
			}
			m.bounds, m.hasBounds = bb, good && bb[0] < bb[2] && bb[1] < bb[3]
		}
		if f, ok := raw["format"].(string); ok && (f == "png" || f == "jpg" || f == "jpeg" || f == "webp") {
			m.format = f
		}
		s.mu.Unlock()
	}
	return s.copyMeta(set)
}

func (s *Server) copyMeta(set string) *setMeta {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := *s.meta[set]
	return &c
}

func (s *Server) fetchJSON(ctx context.Context, target string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream answered %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxMetaBytes)).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// tileIntersects says whether tile z/x/y overlaps the bounds (west, south, east, north in degrees).
func tileIntersects(z, x, y int, b [4]float64) bool {
	n := float64(int(1) << z)
	west := float64(x)/n*360 - 180
	east := float64(x+1)/n*360 - 180
	lat := func(row float64) float64 { return math.Atan(math.Sinh(math.Pi*(1-2*row/n))) * 180 / math.Pi }
	north, south := lat(float64(y)), lat(float64(y+1))
	return east > b[0] && west < b[2] && north > b[1] && south < b[3]
}

// ---- the cache and the upstream -----------------------------------------------------------------------------------------------------------

func (s *Server) tilePath(set string, z, x, y int, ext string) string {
	return filepath.Join(s.cfg.CacheDir, set, strconv.Itoa(z), strconv.Itoa(x), strconv.Itoa(y)+"."+ext)
}

func (s *Server) getTile(ctx context.Context, set string, z, x, y int, ext string) tileResult {
	key := fmt.Sprintf("%s/%d/%d/%d.%s", set, z, x, y, ext)
	path := s.tilePath(set, z, x, y, ext)
	var stale *tileResult
	if fi, err := os.Stat(path); err == nil {
		if data, rerr := os.ReadFile(path); rerr == nil {
			res := tileResult{data: data, status: http.StatusOK, source: "hit", modTime: fi.ModTime()}
			if s.now().Sub(fi.ModTime()) <= s.cfg.TTL {
				s.Stats.Hits.Add(1)
				return res
			}
			res.stale, res.source = true, "stale"
			stale = &res
		}
	}
	s.mu.Lock()
	if until, ok := s.negative[key]; ok {
		if s.now().Before(until) {
			s.mu.Unlock()
			s.Stats.NegativeHits.Add(1)
			return tileResult{status: http.StatusNotFound}
		}
		delete(s.negative, key)
	}
	if f, ok := s.flights[key]; ok {
		s.mu.Unlock()
		s.Stats.Coalesced.Add(1)
		select {
		case <-f.done:
			shared := f.res
			if shared.source == "miss" {
				shared.source = "coalesced"
			}
			return shared
		case <-ctx.Done():
			return tileResult{status: http.StatusGatewayTimeout, err: ctx.Err()}
		}
	}
	f := &flight{done: make(chan struct{})}
	s.flights[key] = f
	s.mu.Unlock()

	f.res = s.fetchTile(context.WithoutCancel(ctx), key, path, ext, stale)
	s.mu.Lock()
	delete(s.flights, key)
	s.mu.Unlock()
	close(f.done)
	return f.res
}

func (s *Server) breakerOpen() (bool, time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if left := s.openUntil.Sub(s.now()); left > 0 {
		return true, left
	}
	return false, 0
}

func (s *Server) recordFailure(openFor time.Duration) {
	s.Stats.UpstreamErrors.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures++
	if openFor > 0 {
		s.openUntil = s.now().Add(openFor)
	} else if s.failures >= s.cfg.BreakerFailures {
		s.openUntil = s.now().Add(s.cfg.BreakerOpen)
	}
}

func (s *Server) recordSuccess() {
	s.mu.Lock()
	s.failures = 0
	s.mu.Unlock()
}

func looksLikeImage(b []byte) bool {
	switch {
	case len(b) >= 8 && bytes.Equal(b[:8], []byte("\x89PNG\r\n\x1a\n")):
		return true
	case len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff:
		return true
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return true
	}
	return false
}

func (s *Server) fetchTile(ctx context.Context, key, path, ext string, stale *tileResult) tileResult {
	fallback := func(code int, retry time.Duration, err error) tileResult {
		if stale != nil {
			return *stale
		}
		return tileResult{status: code, retryAfter: retry, err: err}
	}
	if open, left := s.breakerOpen(); open {
		s.Stats.BreakerRejected.Add(1)
		return fallback(http.StatusServiceUnavailable, left, errors.New("upstream is being left alone after repeated failures"))
	}
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-time.After(s.cfg.Timeout):
		return fallback(http.StatusServiceUnavailable, time.Second, errors.New("too many requests in flight"))
	}
	s.Stats.Fetches.Add(1)
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.Upstream+"/"+key, nil)
	if err != nil {
		return fallback(http.StatusBadGateway, 0, err)
	}
	req.Header.Set("User-Agent", s.cfg.UserAgent)
	req.Header.Set("Accept", "image/*")
	resp, err := s.client.Do(req)
	if err != nil {
		s.recordFailure(0)
		return fallback(http.StatusBadGateway, 0, err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		data, rerr := io.ReadAll(io.LimitReader(resp.Body, maxTileBytes+1))
		if rerr != nil || len(data) > maxTileBytes || !looksLikeImage(data) {
			s.Stats.InvalidUpstream.Add(1)
			s.recordFailure(0)
			return fallback(http.StatusBadGateway, 0, errors.New("the upstream did not send an image"))
		}
		s.recordSuccess()
		s.Stats.UpstreamOK.Add(1)
		modTime, werr := writeAtomic(path, data, s.now())
		if werr != nil {
			modTime = s.now()
		}
		return tileResult{data: data, status: http.StatusOK, source: "miss", modTime: modTime}
	case http.StatusNotFound, http.StatusNoContent, http.StatusGone:
		s.recordSuccess()
		s.mu.Lock()
		s.negative[key] = s.now().Add(s.cfg.NegativeTTL)
		s.mu.Unlock()
		return tileResult{status: http.StatusNotFound}
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		wait := s.cfg.BreakerOpen
		if secs, perr := strconv.Atoi(resp.Header.Get("Retry-After")); perr == nil && secs > 0 {
			wait = min(time.Duration(secs)*time.Second, 5*time.Minute)
		}
		s.recordFailure(wait)
		return fallback(http.StatusServiceUnavailable, wait, fmt.Errorf("upstream answered %d", resp.StatusCode))
	default:
		s.recordFailure(0)
		return fallback(http.StatusBadGateway, 0, fmt.Errorf("upstream answered %d", resp.StatusCode))
	}
}

func writeAtomic(path string, data []byte, now time.Time) (time.Time, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return time.Time{}, err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return time.Time{}, err
	}
	name := tmp.Name()
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Close()
	} else {
		_ = tmp.Close()
	}
	if err == nil {
		err = os.Chtimes(name, now, now)
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		_ = os.Remove(name)
		return time.Time{}, err
	}
	return now, nil
}

// Sweep keeps the cache inside its size limit by removing the tiles fetched longest ago (and any temporary file left by a crash). It returns
// the number of files removed and the size that remains.
func (s *Server) Sweep() (int, int64) {
	type entry struct {
		path string
		size int64
		mod  time.Time
	}
	var files []entry
	var total int64
	_ = filepath.WalkDir(s.cfg.CacheDir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".tmp-") && s.now().Sub(info.ModTime()) > time.Hour {
			_ = os.Remove(p)
			return nil
		}
		files = append(files, entry{p, info.Size(), info.ModTime()})
		total += info.Size()
		return nil
	})
	removed := 0
	if s.cfg.MaxCacheBytes > 0 && total > s.cfg.MaxCacheBytes {
		sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
		target := s.cfg.MaxCacheBytes * 9 / 10
		for _, f := range files {
			if total <= target {
				break
			}
			if os.Remove(f.path) == nil {
				total -= f.size
				removed++
			}
		}
	}
	s.Stats.CacheBytes.Store(total)
	return removed, total
}
