// Shared helpers: data loading, a seeded generator for stress data, labels (English and Urdu), colours read from the stylesheet.
export async function loadViews() {
  return (await fetch('/views.json')).json();
}

export function rng(seed) {   // mulberry32: the same seed gives the same stress data every time
  let a = seed >>> 0;
  return () => { a |= 0; a = (a + 0x6D2B79F5) | 0; let t = Math.imul(a ^ (a >>> 15), 1 | a); t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t; return ((t ^ (t >>> 14)) >>> 0) / 4294967296; };
}

export const state = { rtl: false };
const URDU_DIGITS = ['٠', '١', '٢', '٣', '٤', '٥', '٦', '٧', '٨', '٩'];
export function label(text, kind) {
  if (!state.rtl) return text;
  const digits = String(text).replace(/\d/g, d => URDU_DIGITS[+d]);
  return (kind === 'subject' ? 'موضوع ' : kind === 'hub' ? 'مرکز ' : 'رابطہ ') + digits;
}

export function css(name) { return getComputedStyle(document.documentElement).getPropertyValue(name).trim(); }
export const palette = () => ({ subject: css('--subject'), contact: css('--contact'), hub: css('--hub'), inferred: css('--inferred'), candidate: css('--candidate'), alert: css('--alert'), ink: css('--ink'), line: css('--line'), ok: css('--ok') });

export function evidenceHtml(title, rows, note) {
  const body = (rows || []).map(r => `<tr><td>${r.time ?? ''}</td><td>${r.type ?? ''}</td><td>${r.direction ?? ''}</td><td>${r.duration_s ?? ''}</td><td>${r.owner ?? ''}</td></tr>`).join('');
  return `<h2>Evidence rows</h2><p><strong>${title}</strong></p>${note ? `<p class="muted">${note}</p>` : ''}
    <table><thead><tr><th>Time</th><th>Type</th><th>Dir</th><th>s</th><th>Of</th></tr></thead><tbody>${body}</tbody></table>`;
}

// Time a render and the next two animation frames, so that the work the browser does after the call is counted.
export async function timed(fn) {
  const t0 = performance.now();
  const out = await fn();
  await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)));
  return { ms: Math.round(performance.now() - t0), out };
}

// Mean frame rate while a function drives an animation for a fixed number of steps.
export function framesPerSecond(step, steps) {
  return new Promise(resolve => {
    let i = 0; const t0 = performance.now(); const gaps = []; let last = t0;
    function tick(now) {
      gaps.push(now - last); last = now;
      step(i++);
      if (i < steps) requestAnimationFrame(tick);
      else { const total = performance.now() - t0; resolve({ fps: Math.round(steps / (total / 1000)), worst_ms: Math.round(Math.max(...gaps)) }); }
    }
    requestAnimationFrame(tick);
  });
}
