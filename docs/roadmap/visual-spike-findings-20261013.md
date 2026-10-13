# Phase 0 visual spike: findings and recommendation (2026-10-13)

Roadmap section 7.7, Phase 0. Throwaway code in `tools/visual_spike/` (not part of the product build) draws one generated case as a link chart, a map and a set of timeline cards, from data shaped like an API answer (`tools/case_generator/export_views.py`). Nothing real is read: every number, address and place is synthetic, and the case is laid over a city only for demonstration, with a banner saying so.

**Purpose.** Settle the library choices (D15) and the basemap (D14) on evidence, and turn the first views into something the owner can use and judge. **Result: decisions D14 to D16 were made by the owner on 2026-10-13** (the basemap is the organisation's own tile service, used through our tile cache; the libraries and the map style are as recommended in section 3).

## 1. What was built

| View | What an analyst can do | Analyses |
|---|---|---|
| **Link chart** | Search a number; filter by kind of link and by number of calls; mark two numbers A and B and list their **shared contacts** (hubs left out, and the list is checked against the computed answer on screen: "matches the answer (7)"); find the **shortest connection path** (observed and inferred links, at most four hops); click any number or line for its evidence rows; dim everything but the neighbourhood; save PNG; every link as a sortable table; keyboard operation; Urdu labels with right-to-left layout | A1 to A4 (A12 communities later) |
| **Map** | Offline basemap (muted so the analysis stands out); **sleeping and working place per week** as pins, with a move shown as two pins and a distance; **movement trail** with a time slider and play, thicker where a route is used more; teal discs for time spent per cell site; **speed alerts** in red with the speed on the line and in a list; **co-location windows** of a chosen pair with the chance baseline and "possible meeting" candidates; operator filter and layer switches; place the case over Islamabad, Lahore, Karachi, Peshawar or Quetta, or leave it where it was generated | A5, A6, A7, A10 |
| **Timelines** (cards) | **Device and SIM history** swim lanes with change markers and shared handsets joined; **co-location and possible meetings** on aligned lanes with a window list; **where they sleep and work, week by week**; **daily activity** with the detected change of rhythm and the before and after averages; **hour of day by day** heat map with before and after profiles; **who held a public address and port**, with ambiguity never resolved by guessing | A5, A7, A8, A10, A11, A13 |

Every card and panel says in plain words what it shows and how to read it, carries the caveat that belongs to it ("proximity is not association", "candidate, not a finding", "a cell site is not a position"), offers its numbers as a table, and can be saved (charts as PNG or SVG). Cell sites have short names an analyst can say aloud (U-03: operator letter and grid position) instead of keys.

Screenshots without any basemap are in `docs/roadmap/visual-spike/` (the map screenshots are not kept: the tiles' source is unverified, see 4).

## 2. What was measured

Headless Chrome on this laptop (Intel UHD graphics through Direct3D 11), driven over the DevTools protocol (`tools/visual_spike/scripts/measure.mjs`); the in-app browser pane stops animating while hidden, so it could not be used for timing. Synthetic graphs of 1,000, 5,000 and 20,000 nodes with about three links per node; the real case graph has 77 nodes and 125 links. Frame rates are over 90 to 120 pan and zoom steps. One machine and two complete runs of the final build; the slow cases differ between runs by up to a factor of two (Cytoscape at 5,000 nodes drew 10 and 17 frames per second), so each cell gives the range of the two runs. Read the numbers as orders of magnitude, not as promises. Nothing was fetched from outside the page's origin except the tiles from the configured tile server.

**Link chart (first draw / frames per second; range over two runs)**

| Nodes, links | Cytoscape (canvas) | Cytoscape (WebGL mode) | Sigma (WebGL) |
|---|---|---|---|
| 77, 125 (this case) | 52 to 54 ms / 60 | 269 to 902 ms / 60 | 142 to 269 ms / 60 |
| 1,000, 2,979 | 0.5 s / 37 to 43 | 0.4 s / 46 to 60 | 124 to 248 ms / 60 |
| 5,000, 14,903 | 1.7 to 2.7 s / 10 to 17 | 1.8 to 2.7 s / 15 to 28 | 247 to 328 ms / 59 to 60 |
| 20,000, 59,606 | not run | 9.4 to 10.4 s / 4 to 5 | 0.9 to 1.1 s / 41 |

**Map (points drawn, frames per second while panning and zooming; range over two runs)**

| Points | MapLibre's own layer | deck.gl |
|---|---|---|
| 50,000 | 54 to 60 (first draw 0.3 s) | 34 to 51 (a slowest frame of 0.4 to 1.3 s) |
| 300,000 | 57 to 59 (first draw 0.7 to 1.3 s) | 34 to 35 |

**Size on the wire (minified / gzip)**: Cytoscape 425 kB / 134 kB; Sigma with graphology 154 kB / 36 kB; MapLibre 992 kB / 266 kB plus its worker file (about 0.5 MB); deck.gl 754 kB / 212 kB; ECharts, only what is used, about 730 kB / 244 kB with our chart code (the full library is 1,100 kB / 362 kB).

**Other checks.** Keyboard: arrows move between numbers, Enter opens one, Escape clears, slash searches (tested). A text summary for screen readers is generated for each chart (the text is checked; no screen-reader session was run). Exported SVG is byte-identical on repeat once ECharts' instance counters are renumbered (tested; without that step it is not). The shared-contact list the chart builds equals the computed answer for every pair with such a link (a Python test), and on screen for the pair shown. Page errors in the views: none.

## 3. Decisions D14, D15, D16 (recommendation, then what the owner decided)

**D15, libraries. Decided as recommended: Cytoscape.js (canvas) for the link chart; MapLibre GL JS for the map; Apache ECharts for charts, swim lanes and heat maps. Do not adopt deck.gl or Sigma now.**

* *Cytoscape canvas* is the only candidate that draws every encoding the interface programme needs: node shapes, dashed and dotted lines, arrowheads. Its WebGL mode is faster at 1,000 nodes but **draws every line solid** (checked by zooming into a dashed edge in both modes), so the observed, inferred and candidate styles would be lost. It is slow from about 2,000 nodes (37 to 43 fps at 1,000, 10 to 17 at 5,000). That is acceptable because the analyses are bounded by design (a capped number of hops, a bounded subgraph) and a graph of thousands of nodes is unreadable anyway; so above about 1,500 numbers the chart switches to **overview mode** (built and tested): the numbers that are not subjects are grouped by community (label propagation, deterministic), each group is one node with a count, calls between groups are added up (no call is lost: a unit test), a click opens a group, a search or a shared-contact comparison opens the groups it needs, and subjects are never grouped. A 3,000-number synthetic graph is drawn as 817 nodes and opened and closed again without error.
* *Sigma* is far faster (about 60 fps at 5,000 and 41 at 20,000) and has built-in label collision handling, but it draws only plain lines and circles; dashes, dotted lines and shapes need custom WebGL programs that nobody has written or tested here. Keep it as the documented escape hatch if a real case needs an overview of tens of thousands of nodes.
* *MapLibre's own layers* drew 300,000 points at 57 to 59 fps, faster than deck.gl, so deck.gl (212 kB gzip) is not needed. Add it only if arcs, hexagon bins or animated trips at scale are wanted.
* *ECharts* covers every chart shown here, has text summaries for screen readers, and costs about 244 kB gzip tree-shaken.

**D14, basemap. Decided: the organisation's own offline tile service, used through our tile cache.** The service (a TileServer GL instance on the local network, raster tiles of Pakistan from zoom 4 to 18 plus provinces) is run by a colleague for the organisation's applications, so the interface must be a light client of it. Built for that: **`tools/tilecache`**, a small Go proxy (30 tests, 17 deliberate breakages caught) that fetches each tile from the shared server once and keeps it on disk, fetches a tile several browsers ask for at once a single time, allows at most six requests at a time, leaves a failing server alone and honours `Retry-After`, never asks for a zoom level or area the set does not declare, stores only images, and names itself in `User-Agent`. The browser sees only the cache's TileJSON (`/tiles/pakistan.json`), so the shared server's address is not in the page, there is no mixed-content problem over HTTPS, and nothing but the cache talks to the shared server. Measured against the real service: a first view of two cities and the map controls made 101 tile requests from the browser and 74 fetches from the shared server; a second full run made 112 and 40 (72 served from disk); routine use of the same areas is mostly disk. Two things remain the service owner's: the tile images' source and terms (the metadata says only "created with SAS.Planet"), so the tiles are used for display inside the organisation and are never redistributed with a product (ledger DAT-017), and permission to reach the service from the machines that will run NexusAI. Raster limits: labels are baked into the image (they cannot be hidden, restyled or translated and a dark theme can only dim the picture); vector tiles from OpenStreetMap data remain an option if that ever matters. The cache is built, tested and **not deployed**: running it next to the stack needs the owner's go-ahead. How it is used, what to tell the colleague: `docs/data/offline-basemap-usage.md`.

**D16, map style of generated cases. Decided: keep the open-ocean grid** (so no generated place resembles a real one and the registered case hashes stay valid), and lay it over a chosen city only in the interface, with a visible "synthetic case" banner. This is built (offsets in metres are kept, so distances and speeds are unchanged).

## 4. What the spike does not prove

* Integration: these views read a file; the product reads the API, in the React interface, behind the authority gating, with the wrappers and tests of the definition of done in 7.7. None of that exists yet.
* Real data: only generated cases. Real graphs are denser and messier; the level-of-detail rules are a design, not a measurement.
* Hardware: one laptop's integrated graphics. The GPU node and a phone are not measured.
* Accessibility: keyboard and text summaries are tested; a screen-reader session, colour-blind simulation and a full right-to-left audit are not. ECharts does not mirror a chart by itself, so the timelines do it (time runs from the right, lane names stand on the right, the grid is swapped; checked by picture). Known gap: long Urdu labels overlap in the link chart (Cytoscape has no label collision handling).
* Print and export of the whole view with its evidence appendix (rule V8) is not built; single charts save as PNG or SVG.

## 5. Problems found and what they cost (for the next stage)

* MapLibre 6 starts its worker from a file we must serve ourselves, and its default location does not exist after a Vite build: the page showed a blank map with no error. Fixed with a copy step (`scripts/copy-worker.mjs`).
* Text on the map needs a font file served by us; the spike uses HTML pins for labels instead.
* ECharts must not be created in a hidden view; charts now wait until their box has a size.
* Cytoscape draws wrongly if its layout is not normalised; the spike uses one fixed-size layout.
* The Browser pane of this app does not run animation frames while it is hidden; timing was done in headless Chrome.

## 6. Run it

```bash
cd tools/visual_spike
npm install --ignore-scripts
python -X utf8 ../case_generator/export_views.py --seed 1 --out public/views.json
# optional: public/config.local.json from config.example.json, with the address of the offline tile server
npm run dev            # or: npm run build, then npm run preview
node scripts/measure.mjs http://127.0.0.1:PORT/ OUTPUT_DIR [--skip-bench]    # screenshots and measurements, needs Chrome
```

Licences were read from the installed packages (ledger rows LIB-022 to LIB-028, DAT-017); all are permissive except one build-time tool (lightningcss, MPL-2.0, not in the output).
