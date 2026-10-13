// Run-time configuration. The offline basemap is a tile server on the local network; its address is never written into the repository:
// it is read from /config.local.json (git-ignored; see config.example.json) or from ?tiles=URL in the address bar.
export async function loadConfig() {
  let file = {};
  try { const r = await fetch('/config.local.json', { cache: 'no-store' }); if (r.ok && (r.headers.get('content-type') || '').includes('json')) file = await r.json(); } catch { /* no local file: no basemap */ }
  const q = new URLSearchParams(location.search).get('tiles');
  const url = q || file.tiles?.url || null;
  return {
    tiles: url ? { url, minzoom: file.tiles?.minzoom ?? 4, maxzoom: file.tiles?.maxzoom ?? 18, tileSize: file.tiles?.tileSize ?? 256, attribution: file.tiles?.attribution || 'Offline basemap (local tile server)' } : null,
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
