# tilecache: a caching proxy for the organisation's shared tile server

The map of the interface uses a tile server that other applications of the organisation share. This small program stands between our browsers and that server: a tile is fetched **once**, kept on disk, and served from disk afterwards, so that our use costs the shared server as little as possible. Full explanation, what to tell the server's operators, and what is still open: `docs/data/offline-basemap-usage.md`.

```bash
go build -o tilecache ./tools/tilecache
./tilecache -upstream http://HOST:PORT/data -sets pakistan -cache-dir ./tilecache-data -listen 127.0.0.1:8099
```

Routes: `/tiles/{set}.json` (the set's TileJSON, rewritten to point at this cache), `/tiles/{set}/{z}/{x}/{y}.png`, `/healthz`, `/stats`.

What it does for the shared server: keeps tiles on disk (30 days; an older tile is still served if the server is down), fetches a tile asked for by several browsers at once a single time, has at most 6 requests in flight, leaves a server that fails five times in a row alone for 30 seconds, honours `Retry-After`, remembers missing tiles for five minutes, never asks for a zoom level, area, tile set or address the set does not declare, stores only image responses, refuses redirects, and names itself in `User-Agent` (`-user-agent` to add a contact). The size of the cache is limited (`-max-cache-mb`, oldest first).

Tests (Ginkgo, 30 specs, a fake upstream; 17 deliberate breakages are each caught): `go test ./tools/tilecache`. The race detector could not be run on the development laptop (no cgo). Not deployed anywhere: running it as a service next to the stack is a change to the live environment and needs the owner's go-ahead. The upstream's address is given on the command line and is never written into the repository.
