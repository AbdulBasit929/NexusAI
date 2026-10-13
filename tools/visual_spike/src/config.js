// Run-time configuration. The offline basemap comes from the organisation tile service through our tile cache (tools/tilecache); the address of the shared
// server is never written into the repository. The browser is told only the cache's TileJSON address, from /config.local.json (git-ignored) or ?tilejson=URL.
export async function loadConfig() {
  let file = {};
  try { const r = await fetch('/config.local.json', { cache: 'no-store' }); if (r.ok && (r.headers.get('content-type') || '').includes('json')) file = await r.json(); } catch { /* no local file: no basemap */ }
  const params = new URLSearchParams(location.search);
  // Preferred: the address of the tile cache's TileJSON (tools/tilecache), which the browser then uses for everything: zoom range, bounds and tile addresses
  // come from it, and the shared tile server is never asked by a browser. A tile address template is accepted too (?tiles=URL), for a quick look.
  const tilejson = params.get('tilejson') || file.tiles?.tilejson || null;
  const url = params.get('tiles') || file.tiles?.url || null;
  const t = file.tiles || {};
  return {
    tiles: tilejson || url ? { tilejson, url: tilejson ? null : url, minzoom: t.minzoom ?? 4, maxzoom: t.maxzoom ?? 18, tileSize: t.tileSize ?? 256, attribution: t.attribution || 'Basemap: organisation tile service' } : null,
  };
}

// Where a generated case is drawn on the map. A generated case has fictional cell sites on a fictional grid in open ocean; for a demonstration
// the grid can be laid over a city: the offsets in metres from the grid centre are kept, so every distance and speed stays what the case says.
export const PLACEMENTS = [
  { id: 'ocean', label: 'As generated (open ocean)', lat: 0.43, lon: -30.57 },
  { id: 'islamabad', label: 'Demo: Islamabad', lat: 33.6844, lon: 73.0479 },
  { id: 'lahore', label: 'Demo: Lahore', lat: 31.5204, lon: 74.3587 },
  { id: 'karachi', label: 'Demo: Karachi', lat: 24.8607, lon: 67.0011 },
  { id: 'peshawar', label: 'Demo: Peshawar', lat: 34.0151, lon: 71.5249 },
  { id: 'quetta', label: 'Demo: Quetta', lat: 30.1798, lon: 66.975 },
];

export function placer(origin, target) {
  const kx0 = 111320 * Math.cos(origin.lat * Math.PI / 180), kx1 = 111320 * Math.cos(target.lat * Math.PI / 180), ky = 110574;
  return (lat, lon) => [target.lon + ((lon - origin.lon) * kx0) / kx1, target.lat + ((lat - origin.lat) * ky) / ky];   // returns [lon, lat]
}
