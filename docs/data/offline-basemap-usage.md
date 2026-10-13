# Using the organisation's offline map tiles (decision D14)

Status: 2026-10-13. The map in the interface uses the **offline tile service that the organisation already runs** (a TileServer GL instance on the local network, raster tiles of Pakistan from zoom 4 to 18, plus provinces). It is shared with the other applications of the organisation, so our use of it has two goals: the map must be good, and our application must be a **light and well-behaved client** that does not slow down the others.

The address of the shared server is infrastructure, not source code: it is never written into the repository, the documents or the interface. It is given to the tile cache on its command line (or environment) when it is started.

## 1. How it is used

```
browser (interface)  --/tiles/pakistan.json, /tiles/pakistan/{z}/{x}/{y}.png-->  tilecache (ours)  --one request per tile, once-->  shared tile server
```

* The browser never talks to the shared server. It asks **our tile cache** (`tools/tilecache`, a small Go program with no dependencies beyond the repository's) for the set's TileJSON and its tiles. This keeps the shared server's address out of the browser and out of the page, avoids browser mixed-content blocks when the interface is served over HTTPS, and lets us protect the shared server.
* A tile is fetched from the shared server **once**, kept on disk, and served from disk afterwards (a month by default; a stale tile is still served if the shared server is down). Many analysts looking at the same area cost the shared server one request per tile, not one per view.
* The same tile asked for at the same moment by several browsers is fetched once (request coalescing). At most **six** requests are in flight at a time (configurable). A server that fails five times in a row is left alone for 30 seconds; an answer "busy" with a `Retry-After` is honoured. A tile that does not exist is remembered for five minutes.
* Nothing outside what the set says about itself is ever requested: the cache reads the set's TileJSON (zoom range, bounds, format) and answers locally, without asking, for a zoom level or an area the set does not cover and for any malformed address. The cache names itself in `User-Agent` so the shared server's operators can see which requests are ours. Redirects are refused and only image responses are stored.
* Tiles are not committed, copied into a release or shipped; the cache folder is local to the machine and ignored by git.

## 2. Running it

```bash
go build -o tilecache ./tools/tilecache        # or: go run ./tools/tilecache ...
./tilecache -upstream http://HOST:PORT/data -sets pakistan -cache-dir ./tilecache-data -listen 127.0.0.1:8099
```

| Flag | Default | Meaning |
|---|---|---|
| `-upstream` | none (required) | base URL under which the shared server serves its sets |
| `-sets` | none (required) | tile sets to serve, for example `pakistan` |
| `-cache-dir`, `-max-cache-mb` | `tilecache-data`, 2048 | where tiles are kept, and the size limit (the oldest go first) |
| `-ttl` | 720h | how long a tile is fresh |
| `-concurrency` | 6 | most requests to the shared server at once |
| `-user-agent` | `NexusAI-tilecache/1.0` | add a contact, for example `NexusAI-tilecache/1.0 (maps contact: NAME)` |
| `-allow-origin` | none | CORS header, only if another origin must use the cache |

`/stats` shows what it did (requests, cache hits, upstream fetches, coalesced, stale served, breaker state); `/healthz` says it is up. In the interface the address is only `/tiles/pakistan.json`, configured in `public/config.local.json` of the visual spike (and, in the product, in the interface's own configuration), with the development server forwarding `/tiles` to the cache.

## 3. What to tell the colleague who runs the shared server

* Our traffic: one request per distinct tile **per cache lifetime**, at most six at once, identifiable by `User-Agent`. A first look at one city at zoom 11 to 16 is a few hundred tiles; routine use is mostly cache hits (the spike measured it, see the findings).
* We use only the `pakistan` set at zoom 4 to 18 (others on request), through one cache per deployment, not one per browser.
* We would like to know: the source of the tile images and the terms they were made under (the metadata says only that the tiles were created with SAS.Planet), and whether the server may be reached from the machines that will run NexusAI.

## 4. What is still open

* **Terms of the tile images.** The ledger row (DAT-017) records the service as an organisation-provided internal basemap for use in organisational applications (the owner's statement of 2026-10-13); the original provider's terms are the service owner's to confirm. Until they are, the tiles are used for display inside the organisation and are never redistributed with the product.
* **Raster limits.** The labels are part of the image: they cannot be hidden, restyled or translated, and a dark theme can only dim the picture. Vector tiles from OpenStreetMap data would allow all three, at the cost of producing and hosting them; not needed now.
* **Deployment.** The cache is built and tested but not deployed anywhere: running it next to the stack (for example as one more compose service) is a change to the live environment and needs the owner's go-ahead.
