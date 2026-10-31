#!/usr/bin/env python3
"""Renders the UI query pack (tools/showcase/build_query_pack.py, <out>.json) as ONE self-contained HTML page for a tester: filter by family, copy a question, read the expected answer and what the live engine
answered, mark pass / fail / skip with a note (kept in the browser), and copy the marks as text.

    python -X utf8 tools/showcase/render_pack_html.py docs/qa/ui-query-pack-20261031.json docs/qa/ui-query-pack-20261031.html

The page needs no server and no network except the font stylesheet (it falls back to system fonts).
"""
import json
import sys

TEMPLATE = r"""<title>Orion Query Pack</title>
<meta name="claude:viewport-fit-sides" content="cover">
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500&family=IBM+Plex+Sans:wght@400;500;600&display=swap">
<style>
/* Layout: one column of cards under a sticky filter bar; a family is a folder, a question is a card with its expected answer beside what the live engine said. */
:root {
  --bg: #f4f6f8; --panel: #ffffff; --ink: #14202b; --muted: #5a6b7a; --line: #d8e0e7; --accent: #0f766e; --accent-ink: #ffffff;
  --good: #1a7f4b; --good-bg: #e3f4ea; --bad: #b42318; --bad-bg: #fde8e6; --warn: #8a5a00; --warn-bg: #fdf1d6; --off: #5a6b7a; --off-bg: #e8edf1; --code: #eef2f5;
  --sans: "IBM Plex Sans", system-ui, -apple-system, "Segoe UI", sans-serif; --mono: "IBM Plex Mono", ui-monospace, "Cascadia Mono", Consolas, monospace;
}
@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) {
  --bg: #0d1419; --panel: #141d24; --ink: #e5edf3; --muted: #93a4b3; --line: #243340; --accent: #3cc4b4; --accent-ink: #06201c;
  --good: #5fd08f; --good-bg: #12301f; --bad: #ff8a80; --bad-bg: #3a1714; --warn: #f0c060; --warn-bg: #33270b; --off: #93a4b3; --off-bg: #1c2833; --code: #0f171d; color-scheme: dark; } }
:root[data-theme="dark"] {
  --bg: #0d1419; --panel: #141d24; --ink: #e5edf3; --muted: #93a4b3; --line: #243340; --accent: #3cc4b4; --accent-ink: #06201c;
  --good: #5fd08f; --good-bg: #12301f; --bad: #ff8a80; --bad-bg: #3a1714; --warn: #f0c060; --warn-bg: #33270b; --off: #93a4b3; --off-bg: #1c2833; --code: #0f171d; color-scheme: dark; }
* { box-sizing: border-box; }
body { background: var(--bg); color: var(--ink); font-family: var(--sans); font-size: 15px; line-height: 1.5; padding-block: 0 48px;
  padding-left: max(16px, env(safe-area-inset-left, 0px)); padding-right: max(16px, env(safe-area-inset-right, 0px)); }
.wrap { max-width: 1040px; margin: 0 auto; }
header { padding-block: 28px 8px; }
h1 { font-size: 1.7rem; line-height: 1.2; margin: 0 0 6px; text-wrap: balance; font-weight: 600; }
.sub { color: var(--muted); margin: 0; max-width: 70ch; }
.how { margin: 16px 0 0; padding: 14px 16px; background: var(--panel); border: 1px solid var(--line); border-radius: 10px; }
.how ol { margin: 8px 0 0; padding-left: 20px; } .how li { margin: 3px 0; max-width: 78ch; }
.how b { font-weight: 600; }
code, .q, .mono { font-family: var(--mono); font-size: 0.86rem; }
.bar { position: sticky; top: env(safe-area-inset-top, 0px); z-index: 5; background: var(--bg); padding-block: 10px; border-bottom: 1px solid var(--line); margin-top: 16px; }
body::before { content: ""; position: fixed; inset: 0 0 auto; height: env(safe-area-inset-top, 0px); z-index: 10; background: var(--bg); }
.row { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
.stat { display: flex; gap: 14px; flex-wrap: wrap; font-variant-numeric: tabular-nums; color: var(--muted); font-size: 0.9rem; }
.stat b { color: var(--ink); font-weight: 600; }
input[type=search], select, input.note { font: inherit; color: var(--ink); background: var(--panel); border: 1px solid var(--line); border-radius: 8px; padding: 7px 10px; min-width: 0; }
input[type=search] { flex: 1 1 220px; }
button { font: inherit; cursor: pointer; border: 1px solid var(--line); background: var(--panel); color: var(--ink); border-radius: 8px; padding: 6px 12px; }
button:hover { border-color: var(--accent); }
button:focus-visible, input:focus-visible, select:focus-visible, summary:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.chips { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px; }
.chip { padding: 4px 10px; font-size: 0.85rem; border-radius: 999px; }
.chip[aria-pressed=true] { background: var(--accent); color: var(--accent-ink); border-color: var(--accent); }
details.area { margin-top: 22px; }
details.area > summary { cursor: pointer; list-style: none; display: flex; gap: 10px; align-items: baseline; flex-wrap: wrap; padding: 6px 0; border-bottom: 2px solid var(--ink); }
details.area > summary::-webkit-details-marker { display: none; }
details.area h2 { margin: 0; font-size: 1.15rem; font-weight: 600; }
.count { color: var(--muted); font-size: 0.85rem; font-variant-numeric: tabular-nums; }
.intro { color: var(--muted); margin: 8px 0 4px; max-width: 80ch; } .where { margin: 0 0 8px; max-width: 80ch; font-size: 0.92rem; }
.card { background: var(--panel); border: 1px solid var(--line); border-radius: 10px; padding: 12px 14px; margin-top: 10px; display: grid; gap: 8px; }
.card[data-mark=pass] { border-left: 4px solid var(--good); } .card[data-mark=fail] { border-left: 4px solid var(--bad); } .card[data-mark=skip] { border-left: 4px solid var(--off); }
.head { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; justify-content: space-between; }
.id { font-family: var(--mono); font-size: 0.8rem; color: var(--muted); }
.tag { font-size: 0.75rem; padding: 1px 8px; border-radius: 999px; background: var(--off-bg); color: var(--off); }
.tag.honesty { background: var(--warn-bg); color: var(--warn); }
.qline { display: flex; gap: 8px; align-items: flex-start; }
.q { flex: 1 1 auto; min-width: 0; background: var(--code); padding: 8px 10px; border-radius: 8px; overflow-wrap: anywhere; user-select: all; }
.cols { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
@media (max-width: 720px) { .cols { grid-template-columns: 1fr; } }
.lab { font-size: 0.72rem; letter-spacing: 0.06em; text-transform: uppercase; color: var(--muted); margin-bottom: 2px; }
.val { overflow-wrap: anywhere; min-width: 0; }
.pill { display: inline-block; font-size: 0.78rem; font-weight: 500; padding: 1px 9px; border-radius: 999px; margin-right: 6px; }
.pill.correct { background: var(--good-bg); color: var(--good); } .pill.wrong { background: var(--bad-bg); color: var(--bad); }
.pill.declined, .pill.abstained, .pill.check { background: var(--warn-bg); color: var(--warn); } .pill.none { background: var(--off-bg); color: var(--off); }
.marks { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; }
.marks button[aria-pressed=true].pass { background: var(--good-bg); border-color: var(--good); color: var(--good); }
.marks button[aria-pressed=true].fail { background: var(--bad-bg); border-color: var(--bad); color: var(--bad); }
.marks button[aria-pressed=true].skip { background: var(--off-bg); border-color: var(--off); color: var(--off); }
input.note { flex: 1 1 200px; font-size: 0.9rem; }
details.sql summary { cursor: pointer; color: var(--muted); font-size: 0.85rem; } details.sql pre { margin: 6px 0 0; padding: 8px 10px; background: var(--code); border-radius: 8px; overflow-x: auto; font-size: 0.8rem; white-space: pre-wrap; overflow-wrap: anywhere; }
.empty { color: var(--muted); padding: 24px 0; }
#toast { position: fixed; left: 50%; transform: translateX(-50%); bottom: calc(18px + env(safe-area-inset-bottom, 0px)); background: var(--ink); color: var(--bg); padding: 8px 14px; border-radius: 8px; font-size: 0.9rem; opacity: 0; pointer-events: none; transition: opacity .2s; }
#toast.on { opacity: 1; }
@media (prefers-reduced-motion: reduce) { #toast { transition: none; } }
</style>
<div class="wrap">
<header>
  <h1>Orion Query Pack</h1>
  <p class="sub" id="sub"></p>
  <div class="how">
    <b>How to test</b>
    <ol>
      <li>Open <code>http://localhost:4181</code>, choose <b>Cases</b>, then <code>orion-lahore-reference-20261030</code>, then the tab <b>Ask this case</b>.</li>
      <li>Copy a question with its button and paste it into the box. An answer takes 15 to 120 seconds.</li>
      <li>Read the first sentence, the table, the notes, then <b>How was this answered</b>. Compare with <b>Expected</b>, mark Pass or Fail, and write what you saw.</li>
      <li><b>Honesty checks</b> matter as much as answers: a question the case cannot answer must be declined or answered "nothing found", never guessed.</li>
      <li>Connection questions expect the case's <b>hub threshold of 10</b>, which is set on this case; each such answer says "Hub threshold used: 10 subscribers (the setting of this case...)". To see the setting at work, put your own number in a question ("...leaving out numbers in contact with more than 5 subscribers") and the answer says "read from your question". Change the case's value with <code>python -X utf8 tools/showcase/set_case_setting.py --case orion-lahore-reference-20261030 --hub-threshold 20 --apply</code> (and set it back to 10).</li>
    </ol>
  </div>
</header>
<div class="bar">
  <div class="row">
    <input type="search" id="q" placeholder="Search a question, an id or a number" aria-label="Search">
    <select id="st" aria-label="Filter by mark"><option value="">All marks</option><option value="none">Not marked</option><option value="pass">Pass</option><option value="fail">Fail</option><option value="skip">Skipped</option></select>
    <select id="lv" aria-label="Filter by live result"><option value="">Any live result</option><option value="correct">Live correct</option><option value="wrong">Live wrong</option><option value="declined">Live declined</option><option value="none">Not run live</option></select>
    <button id="hon" aria-pressed="false">Honesty checks only</button>
    <button id="exp">Copy my marks</button>
  </div>
  <div class="chips" id="chips"></div>
  <div class="stat" id="stat" style="margin-top:8px"></div>
</div>
<main id="main"></main>
</div>
<div id="toast" role="status"></div>
<script>
const PACK = __DATA__;
const KEY = "orion-query-pack-marks-v1";
let marks = {};
try { marks = JSON.parse(localStorage.getItem(KEY) || "{}"); } catch (e) { marks = {}; }
const save = () => { try { localStorage.setItem(KEY, JSON.stringify(marks)); } catch (e) { /* the page works without storage */ } };
const esc = s => String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
const state = { text: "", area: "", mark: "", live: "", honesty: false };
const liveOf = q => (q.live ? (q.live.verdict || q.live.state || "check") : "none");
const markOf = q => (marks[q.id] && marks[q.id].m) || "none";

function toast(msg) { const t = document.getElementById("toast"); t.textContent = msg; t.classList.add("on"); clearTimeout(toast.h); toast.h = setTimeout(() => t.classList.remove("on"), 1800); }
async function copy(text, done) {
  try { await navigator.clipboard.writeText(text); toast(done); }
  catch (e) { const ta = document.createElement("textarea"); ta.value = text; document.body.appendChild(ta); ta.select(); try { document.execCommand("copy"); toast(done); } catch (e2) { toast("Select the text and copy it"); } ta.remove(); }
}
function visible(q) {
  if (state.area && q.area !== state.area) return false;
  if (state.honesty && q.kind_of_check !== "honesty / limits") return false;
  if (state.mark && markOf(q) !== state.mark) return false;
  if (state.live) { const l = liveOf(q); if (state.live === "declined" ? !(l === "declined" || l === "abstained") : l !== state.live) return false; }
  if (state.text) { const hay = (q.id + " " + q.question + " " + q.expected_text + " " + q.type).toLowerCase(); if (!hay.includes(state.text.toLowerCase())) return false; }
  return true;
}
function card(q) {
  const m = markOf(q), n = (marks[q.id] && marks[q.id].n) || "";
  const l = q.live, v = liveOf(q);
  const live = l
    ? '<span class="pill ' + esc(v.split(" ")[0]) + '">' + esc(v) + '</span>' + esc(l.answer || "(no answer text)") + ' <span class="id">' + esc(l.seconds) + ' s</span>' +
      (l.sql ? '<details class="sql"><summary>The query it wrote</summary><pre>' + esc(l.sql) + '</pre></details>' : "")
    : '<span class="pill none">not run live</span>';
  return '<article class="card" data-id="' + esc(q.id) + '" data-mark="' + m + '">' +
    '<div class="head"><span class="id">' + esc(q.id.replace("ORL-", "")) + '</span><span><span class="tag">' + esc(q.type.replace(/_/g, " ")) + '</span> ' +
    (q.kind_of_check === "honesty / limits" ? '<span class="tag honesty">honesty check</span>' : "") + '</span></div>' +
    '<div class="qline"><div class="q">' + esc(q.question) + '</div><button data-copy="' + esc(q.id) + '" aria-label="Copy question ' + esc(q.id) + '">Copy</button></div>' +
    '<div class="cols"><div><div class="lab">Expected</div><div class="val">' + esc(q.expected_text) + (q.note ? ' <span class="id">(' + esc(q.note) + ')</span>' : "") + '</div></div>' +
    '<div><div class="lab">Live engine</div><div class="val">' + live + '</div></div></div>' +
    '<div class="marks" data-id="' + esc(q.id) + '"><button class="pass" data-mark="pass" aria-pressed="' + (m === "pass") + '">Pass</button><button class="fail" data-mark="fail" aria-pressed="' + (m === "fail") + '">Fail</button>' +
    '<button class="skip" data-mark="skip" aria-pressed="' + (m === "skip") + '">Skip</button><input class="note" data-note="' + esc(q.id) + '" placeholder="What you saw" value="' + esc(n) + '" aria-label="Note for ' + esc(q.id) + '"></div></article>';
}
function render() {
  const main = document.getElementById("main");
  let html = "", shown = 0;
  for (const a of PACK.areas) {
    const qs = PACK.questions.filter(q => q.area === a.key && visible(q));
    const all = PACK.questions.filter(q => q.area === a.key);
    if (!all.length) continue;
    shown += qs.length;
    if (!qs.length) continue;
    const done = all.filter(q => markOf(q) !== "none").length;
    html += '<details class="area" open><summary><h2>' + esc(a.title) + '</h2><span class="count">' + qs.length + ' shown of ' + all.length + ' · ' + done + ' marked</span></summary>' +
      '<p class="intro">' + esc(a.exercises) + '</p><p class="where"><b>Where to read it:</b> ' + esc(a.where) + '</p>' + qs.map(card).join("") + '</details>';
  }
  main.innerHTML = html || '<p class="empty">No question matches these filters.</p>';
  stats();
}
function stats() {
  const all = PACK.questions, c = { pass: 0, fail: 0, skip: 0 };
  all.forEach(q => { const m = markOf(q); if (c[m] !== undefined) c[m]++; });
  const live = { correct: 0, wrong: 0, other: 0, none: 0 };
  all.forEach(q => { const l = liveOf(q); if (l === "correct") live.correct++; else if (l === "wrong") live.wrong++; else if (l === "none") live.none++; else live.other++; });
  document.getElementById("stat").innerHTML = '<span>Marked <b>' + (c.pass + c.fail + c.skip) + '</b> of ' + all.length + '</span><span>Pass <b>' + c.pass + '</b></span><span>Fail <b>' + c.fail + '</b></span><span>Skipped <b>' + c.skip +
    '</b></span><span>Live engine: correct <b>' + live.correct + '</b>, wrong <b>' + live.wrong + '</b>, declined or abstained <b>' + live.other + '</b>, not run <b>' + live.none + '</b></span>';
}
function chips() {
  const el = document.getElementById("chips");
  el.innerHTML = '<button class="chip" data-area="" aria-pressed="' + (state.area === "") + '">All families</button>' +
    PACK.areas.filter(a => PACK.questions.some(q => q.area === a.key)).map(a => '<button class="chip" data-area="' + esc(a.key) + '" aria-pressed="' + (state.area === a.key) + '">' + esc(a.title) + '</button>').join("");
}
document.getElementById("sub").textContent = PACK.questions.length + " questions for the synthetic Orion Lahore case, " + PACK.areas.filter(a => PACK.questions.some(q => q.area === a.key)).length +
  " families. Expected answers come from the case's own truth or from SQL on the stored rows, never from the engine.";
document.addEventListener("click", e => {
  const b = e.target.closest("button"); if (!b) return;
  if (b.dataset.copy) { const q = PACK.questions.find(x => x.id === b.dataset.copy); copy(q.question, "Question copied"); return; }
  if (b.dataset.area !== undefined) { state.area = b.dataset.area; chips(); render(); return; }
  if (b.dataset.mark) { const id = b.parentElement.dataset.id; const cur = marks[id] || {}; cur.m = cur.m === b.dataset.mark ? "none" : b.dataset.mark; marks[id] = cur; save(); render(); return; }
  if (b.id === "hon") { state.honesty = !state.honesty; b.setAttribute("aria-pressed", state.honesty); render(); return; }
  if (b.id === "exp") {
    const lines = PACK.questions.filter(q => markOf(q) !== "none").map(q => q.id + " | " + markOf(q) + " | live " + liveOf(q) + " | " + ((marks[q.id] && marks[q.id].n) || ""));
    copy(lines.length ? lines.join("\n") : "(nothing marked yet)", "Marks copied"); return;
  }
});
document.addEventListener("input", e => {
  if (e.target.id === "q") { state.text = e.target.value; render(); return; }
  if (e.target.dataset && e.target.dataset.note) { const id = e.target.dataset.note; const cur = marks[id] || {}; cur.n = e.target.value; marks[id] = cur; save(); }
});
document.getElementById("st").addEventListener("change", e => { state.mark = e.target.value; render(); });
document.getElementById("lv").addEventListener("change", e => { state.live = e.target.value; render(); });
chips(); render();
</script>
"""


def main(argv):
    if len(argv) != 2:
        raise SystemExit(__doc__)
    pack = json.load(open(argv[0], encoding="utf-8"))
    slim = {"areas": pack["areas"], "questions": [{k: q.get(k) for k in ("id", "area", "type", "question", "expected_text", "note", "kind_of_check", "live")} for q in pack["questions"]]}
    html = TEMPLATE.replace("__DATA__", json.dumps(slim, ensure_ascii=False).replace("</", "<\\/"))
    with open(argv[1], "w", encoding="utf-8", newline="\n") as handle:
        handle.write(html)
    print("%d questions -> %s (%d bytes)" % (len(slim["questions"]), argv[1], len(html.encode("utf-8"))))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
