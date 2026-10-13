// Command tilecache is a small caching proxy for a tile server that other applications of the organisation share. The interface asks this
// proxy for tiles; a tile is fetched from the shared server once, kept on disk, and served from there afterwards. See README.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	cfg := DefaultConfig()
	var sets string
	var maxMB int64
	listen := flag.String("listen", "127.0.0.1:8099", "address to listen on (keep it on the loopback unless a reverse proxy is in front)")
	flag.StringVar(&cfg.Upstream, "upstream", "", "base URL under which the shared tile server serves its sets, for example http://HOST:PORT/data")
	flag.StringVar(&sets, "sets", "", "comma separated tile sets to serve, for example pakistan")
	flag.StringVar(&cfg.CacheDir, "cache-dir", "tilecache-data", "where tiles are kept")
	flag.Int64Var(&maxMB, "max-cache-mb", cfg.MaxCacheBytes>>20, "size limit of the cache in MiB; the oldest tiles go first")
	flag.DurationVar(&cfg.TTL, "ttl", cfg.TTL, "how long a cached tile counts as fresh (a stale one is still served if the upstream fails)")
	flag.DurationVar(&cfg.NegativeTTL, "negative-ttl", cfg.NegativeTTL, "how long a missing tile is remembered")
	flag.IntVar(&cfg.Concurrency, "concurrency", cfg.Concurrency, "most requests to the upstream at the same time")
	flag.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "timeout of one request to the upstream")
	flag.StringVar(&cfg.UserAgent, "user-agent", cfg.UserAgent, "how this proxy names itself to the upstream's operators")
	flag.StringVar(&cfg.AllowOrigin, "allow-origin", "", "Access-Control-Allow-Origin to send (empty: none, for same-origin use)")
	flag.Parse()
	cfg.MaxCacheBytes = maxMB << 20
	for _, set := range strings.Split(sets, ",") {
		if set = strings.TrimSpace(set); set != "" {
			cfg.Sets = append(cfg.Sets, set)
		}
	}
	srv, err := NewServer(cfg)
	if err != nil {
		log.Fatalf("tilecache: %v", err)
	}
	removed, size := srv.Sweep()
	log.Printf("tilecache: serving %v on %s, cache %s holds %d MiB (%d removed), concurrency %d, ttl %s", cfg.Sets, *listen, cfg.CacheDir, size>>20, removed, cfg.Concurrency, cfg.TTL)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		tick := time.NewTicker(5 * time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				srv.Sweep()
			}
		}
	}()
	httpSrv := &http.Server{Addr: *listen, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdown)
	}()
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, "tilecache:", err)
		os.Exit(1)
	}
	log.Printf("tilecache: stopped; %d requests, %d from the cache, %d fetched from the upstream", srv.Stats.Requests.Load(), srv.Stats.Hits.Load(), srv.Stats.UpstreamOK.Load())
}
