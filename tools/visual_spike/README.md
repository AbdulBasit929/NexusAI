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
changes where). Without it the map shows a neutral grid. Never commit tiles or the address of the shared server.

`?stress=3000` replaces the link chart's graph by a synthetic one of that many numbers, to see overview mode (groups of numbers). `node --test scripts/test-overview.mjs`
tests the grouping; `node scripts/check-overview.mjs BASE_URL` checks it in the browser.

`node scripts/measure.mjs URL OUT_DIR [--skip-bench]` drives an installed Chrome, exercises every view, writes screenshots and (without
`--skip-bench`) the measurements. `node scripts/size.mjs` prints the size of each library. All data on screen is generated; the numbers are not real.
