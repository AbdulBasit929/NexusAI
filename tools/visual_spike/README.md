# Visual spike (roadmap 7.7, Phase 0)

Throwaway code that draws one generated case as a link chart, a map and timeline cards, to settle the library choices and show the first views.
Not part of the product build. Findings and recommendation: `docs/roadmap/visual-spike-findings-20261013.md`.

```bash
npm install --ignore-scripts
python -X utf8 ../case_generator/export_views.py --seed 1 --out public/views.json   # the data the views read (synthetic)
npm run dev                                                                          # or: npm run build && npm run preview
```

Offline basemap (optional): copy `config.example.json` to `public/config.local.json` (ignored by git) and put the address of your tile server
in it, or open the page with `?tiles=http://HOST:PORT/data/NAME/{z}/{x}/{y}.png`. Without it the map shows a neutral grid. Never commit tiles or the address.

`node scripts/measure.mjs URL OUT_DIR [--skip-bench]` drives an installed Chrome, exercises every view, writes screenshots and (without
`--skip-bench`) the measurements. `node scripts/size.mjs` prints the size of each library. All data on screen is generated; the numbers are not real.
