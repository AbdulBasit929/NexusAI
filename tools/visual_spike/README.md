# Visual spike (roadmap 7.7, Phase 0)

Throwaway code that draws one generated case as a link chart, a map and timeline cards, to settle the library choices and show the first views.
Not part of the product build. Findings and recommendation: `docs/roadmap/visual-spike-findings-20261013.md`.

```bash
npm install --ignore-scripts
python -X utf8 ../case_generator/export_views.py --seed 1 --out public/views.json   # the data the views read (synthetic)
npm run dev                                                                          # or: npm run build && npm run preview
```

Offline basemap (optional): the organisation's tile service is reached **through our tile cache** (`tools/tilecache`, see `docs/data/offline-basemap-usage.md`),
never directly from the browser. Start the cache (`go run ./tools/tilecache -upstream http://HOST:PORT/data -sets pakistan -listen 127.0.0.1:8099`), copy
`config.example.json` to `public/config.local.json` (ignored by git), and the development and preview servers forward `/tiles` to it (`TILE_CACHE_URL`
changes where). Without it the map shows a plain background and says why (a badge and a notice: no basemap configured, tile cache not reachable, shared server not reachable, or outside the basemap's coverage). Never commit tiles or the address of the shared server.

**One command:** `npm run dev:tiles` starts the tile cache and the development server together. Put the shared server's address in the environment (`TILE_UPSTREAM=http://HOST:PORT/data`) or in `tiles.local.json` (`{ "upstream": "http://HOST:PORT/data", "sets": "pakistan" }`, ignored by git, never served to a browser); the cache keeps its tiles in `%LOCALAPPDATA%\NexusAI\tilecache-data` (`TILE_CACHE_DIR` changes it). With a basemap configured the case is first drawn over Islamabad, because the Pakistan basemap has no tiles in the open ocean where a generated case sits.

`?stress=3000` replaces the link chart's graph by a synthetic one of that many numbers, to see overview mode (groups of numbers). `node --test scripts/test-overview.mjs`
tests the grouping; `node scripts/check-overview.mjs BASE_URL` checks it in the browser.

`node scripts/measure.mjs URL OUT_DIR [--skip-bench]` drives an installed Chrome, exercises every view, writes screenshots and (without
`--skip-bench`) the measurements. `node scripts/size.mjs` prints the size of each library. All data on screen is generated; the numbers are not real.

## Targets, filters and SIM carriers (ready to lift into the workspace)

Two files hold everything the views share, with no dependency on the rest of the spike except `ui.js` helpers, so that they can be moved into the workspace as they are (roadmap 7.8.5):

| File | What it gives |
|---|---|
| `src/carriers.js` | The carrier registry (Jazz, Zong, Ufone, Telenor, SCOM, other): names, colours, IMSI network codes. `carrierFor({ number, imsi, operator })` finds the carrier from the SIM's own network code first, then from a file layout that names a carrier, then from the number's range (a ported number may differ, and the result says how it was found). `formatNumber` writes 923001234567 as 0300 1234567. `carrierIcon(id, px)` / `carrierIconUrl(id)` / `carrierLabel(id)` draw the badge. |
| `src/filters.js` | `decorate(views)` adds the carrier to every number and puts the same number format in every view; `FilterState` keeps targets, carriers and period in the address (`?t=...&c=jazz,zong&from=2026-03-01&to=2026-03-31`, so a view can be shared or reopened); `filterViews(views, state)` is pure and returns the views for one set of filters; `mountFilterBar(...)` draws the bar (target picker with search, All, None and Top 5; carrier chips with counts; period with presets; Reset; Copy link; a line saying how much is shown). |

What the filters mean: **Targets** are the subjects to look at (the link chart shows each target and the numbers in contact with it; the map and the timelines show the targets). **Carrier** keeps the subjects whose SIM is on the chosen carriers and, around them, the other numbers on those carriers. **Period** keeps a link when its first and last contact overlap the period and cuts the movement trail to it; shared-contact and successor links are computed on the whole record, so they are hidden while a period is chosen, and the bar says so.

**Logos.** The badges are original placeholder marks (a rounded square and the carrier's initial in a colour near its brand colour), not the carriers' logos: a logo is a trademark and may not be fetched or shipped without a licence-ledger entry (roadmap P7). To use the official artwork, add one file per carrier as `public/carriers/<id>.svg` (or `.png`; ids `jazz`, `zong`, `ufone`, `telenor`, `scom`) once the entry exists. The spike uses the file when it is there and the placeholder otherwise; nothing is fetched from the network.

**Numbers.** Real numbers are shown in full, written the way a person says them (0300 1234567), in every view; there are no short labels. Cell sites keep their short names (J-03, U-12), given on the whole case so that a filter does not rename them.

`node scripts/check-filters.mjs BASE_URL [synthetic]` drives an installed Chrome through the targets, carriers and period filters, the empty state, the address round trip and the number format, and prints counts and true/false only (no numbers, no screenshots), so that it is safe to run on the real data. In the page, `window.__app.filters.set({ targets: [...], carriers: [...], from, to })` and `window.__app.views` show the same state.
