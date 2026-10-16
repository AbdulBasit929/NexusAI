// SIM carriers: names, colours, badges, and how a number, an IMSI or a file layout is tied to one.
//
// Lifted from tools/visual_spike/src/carriers.js into the workspace (roadmap 7.8.5): pure functions, no DOM; the badge is drawn by components/CarrierBadge.jsx.
//
// LOGOS. The badges drawn here are ORIGINAL placeholder marks (a rounded square with the carrier's initial in a colour near its brand colour), not the carriers' logos:
// a logo is a trademark and may not be fetched or shipped without a licence-ledger entry (roadmap P7). To use the official artwork, put one file per carrier in
// public/carriers/<id>.svg (or .png); `carrierIcon` uses it when the file exists and falls back to the placeholder otherwise. Nothing is loaded from the network.
//
// HOW A CARRIER IS FOUND. (1) From the IMSI: the first five digits are the mobile country code and network code (410 and 01 for Jazz): the most reliable, because it is
// the SIM's own network. (2) From the file's layout when it names a carrier. (3) From the number's range: Pakistan's number ranges are issued per carrier, but a ported
// number keeps its number and changes carrier, so a range is a good guess and not a fact. Every result carries how it was found.

export const CARRIERS = {
  jazz: { id: 'jazz', name: 'Jazz', initial: 'J', colour: '#D6192A', ink: '#fff', mccmnc: ['41001', '41007'], note: 'Mobilink and Warid, now Jazz' },
  zong: { id: 'zong', name: 'Zong', initial: 'Z', colour: '#6CB33F', ink: '#102a05', mccmnc: ['41004'], note: 'China Mobile Pakistan' },
  ufone: { id: 'ufone', name: 'Ufone', initial: 'U', colour: '#F58220', ink: '#2b1500', mccmnc: ['41003'], note: 'PTCL group' },
  telenor: { id: 'telenor', name: 'Telenor', initial: 'T', colour: '#0A9BD8', ink: '#fff', mccmnc: ['41006'], note: 'Telenor Pakistan' },
  scom: { id: 'scom', name: 'SCOM', initial: 'S', colour: '#2E7D32', ink: '#fff', mccmnc: ['41005'], note: 'Special Communications Organization' },
  other: { id: 'other', name: 'Other or unknown', initial: '?', colour: '#7a8494', ink: '#fff', mccmnc: [], note: 'Not a Pakistani mobile range, an international number, or not known' },
};
export const CARRIER_ORDER = ['jazz', 'zong', 'ufone', 'telenor', 'scom', 'other'];

// Pakistani mobile ranges after the country code (92) and the leading 3: 300-309 and 320-325 Jazz, 310-319 Zong, 330-337 Ufone, 340-349 Telenor, 355 SCOM.
const RANGES = [[300, 309, 'jazz'], [320, 325, 'jazz'], [310, 319, 'zong'], [330, 337, 'ufone'], [340, 349, 'telenor'], [355, 355, 'scom']];

const digits = value => String(value ?? '').replace(/\D/g, '');

// A Pakistani mobile number in any common written form (92300..., 0300..., +92 300...) as the three digits after the 3, or null.
export function pkMobile(number) {
  const d = digits(number);
  const m = /^(?:92|0)?(3\d{2})(\d{7})$/.exec(d);
  return m ? { prefix: +m[1], rest: m[2] } : null;
}

export function carrierOfNumber(number) {
  const p = pkMobile(number);
  if (!p) return { id: 'other', how: 'not a Pakistani mobile number' };
  const hit = RANGES.find(([lo, hi]) => p.prefix >= lo && p.prefix <= hi);
  return hit ? { id: hit[2], how: 'number range (a ported number may differ)' } : { id: 'other', how: 'a mobile range without a known carrier' };
}

export function carrierOfImsi(imsi) {
  const d = digits(imsi);
  if (d.length < 5) return null;
  const five = d.slice(0, 5);
  const hit = Object.values(CARRIERS).find(c => c.mccmnc.includes(five));
  return hit ? { id: hit.id, how: 'IMSI network code ' + five.slice(0, 3) + '-' + five.slice(3) } : null;
}

// The operator a view carries is a carrier name in the generated case (ufone, zong, mobilink, telenor) and a layout name in real files (company, strt-tm, ...).
export function carrierOfOperator(operator) {
  const key = String(operator || '').toLowerCase();
  const direct = { ufone: 'ufone', zong: 'zong', mobilink: 'jazz', jazz: 'jazz', warid: 'jazz', telenor: 'telenor', scom: 'scom' }[key];
  return direct ? { id: direct, how: 'the file format names it' } : null;
}

// The best carrier for a number the analysis holds: the SIM's network, else the file's operator, else the number range.
export function carrierFor({ number, imsi, operator } = {}) {
  return carrierOfImsi(imsi) || carrierOfOperator(operator) || carrierOfNumber(number);
}

// 923001234567 -> 0300 1234567 (how a person says it); anything else is returned as written.
export function formatMobile(number) {
  const p = pkMobile(number);
  return p ? `0${p.prefix} ${p.rest}` : String(number ?? '');
}

// ---- badges -----------------------------------------------------------------------------------------------------------------------------------------------------------
export function badgeSvg(id, size = 64) {
  const c = CARRIERS[id] || CARRIERS.other;
  const r = Math.round(size * 0.24);
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 ${size} ${size}" role="img" aria-label="${c.name}"><rect width="${size}" height="${size}" rx="${r}" fill="${c.colour}"/>`
    + `<text x="50%" y="53%" text-anchor="middle" dominant-baseline="middle" font-family="Segoe UI, Arial, sans-serif" font-weight="800" font-size="${Math.round(size * 0.62)}" fill="${c.ink}">${c.initial}</text></svg>`;
}

const official = new Map();   // id -> url or null, after one probe of /carriers/<id>.svg (or .png) in the served files
let probed = null;
export function probeOfficialIcons(ids = CARRIER_ORDER, fetchImpl = globalThis.fetch) {
  if (probed) return probed;
  probed = Promise.all(ids.map(async id => {
    for (const ext of ['svg', 'png']) {
      try {
        const r = await fetchImpl(`/carriers/${id}.${ext}`, { method: 'HEAD', cache: 'no-store' });
        if (r.ok && /image/.test(r.headers.get('content-type') || '')) { official.set(id, `/carriers/${id}.${ext}`); return; }
      } catch { /* no network or no file */ }
    }
    official.set(id, null);
  })).then(() => true);
  return probed;
}

// A URL that an <img> or Cytoscape can use: the official file when there is one, else the placeholder badge as a data URI.
export function carrierIconUrl(id, size = 64) {
  const c = CARRIERS[id] || CARRIERS.other;
  return official.get(c.id) || 'data:image/svg+xml;utf8,' + encodeURIComponent(badgeSvg(c.id, size));
}

export const carrierName = id => (CARRIERS[id] || CARRIERS.other).name;
