#!/usr/bin/env python3
"""Builds roadmap-pack.html (the Artifact page) from the markdown files of this folder. Run:  python -I -X utf8 build_page.py"""
import json
import os
import re

HERE = os.path.dirname(os.path.abspath(__file__))
FILES = [
    ("start", "00_START_HERE.md", "Start here"),
    ("state", "01_state_of_the_project.md", "State of the project"),
    ("media", "02_media_programme.md", "Media programme"),
    ("architecture", "03_architecture.md", "Architecture"),
    ("ui", "04_ui_ux_programme.md", "UI/UX programme"),
    ("data", "05_data_testing_research.md", "Data, testing, research"),
    ("roadmap", "06_roadmap_phases.md", "Roadmap and phases"),
    ("decisions", "07_decisions_and_risks.md", "Decisions and risks"),
]

TEMPLATE = r"""<title>NexusAI Roadmap Pack</title>
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500&family=IBM+Plex+Sans:wght@400;500;600&display=swap">
<style>
:root {
  --bg: #f5f7f9; --panel: #ffffff; --ink: #15202b; --muted: #55677a; --line: #d7dfe7; --accent: #0f6b66; --accent-ink: #ffffff; --soft: #e6f2f1;
  --code: #edf1f5; --th: #eaf0f5; --zebra: #f8fafc; --warn: #8a5a00; --warn-bg: #fdf1d6;
  --sans: "IBM Plex Sans", system-ui, -apple-system, "Segoe UI", sans-serif; --mono: "IBM Plex Mono", ui-monospace, "Cascadia Mono", Consolas, monospace;
}
@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) {
  --bg: #0e1418; --panel: #151e25; --ink: #e6edf3; --muted: #93a5b5; --line: #26343f; --accent: #46c2b6; --accent-ink: #05201d; --soft: #12302e;
  --code: #1b2630; --th: #1c2832; --zebra: #121b22; --warn: #f0c36a; --warn-bg: #3a2e12; color-scheme: dark; } }
:root[data-theme="dark"] {
  --bg: #0e1418; --panel: #151e25; --ink: #e6edf3; --muted: #93a5b5; --line: #26343f; --accent: #46c2b6; --accent-ink: #05201d; --soft: #12302e;
  --code: #1b2630; --th: #1c2832; --zebra: #121b22; --warn: #f0c36a; --warn-bg: #3a2e12; color-scheme: dark; }
body { background: var(--bg); color: var(--ink); font: 15px/1.6 var(--sans); }
* { box-sizing: border-box; }
.frame { display: grid; grid-template-columns: 17rem minmax(0, 1fr); gap: 0; min-height: 100vh; }
nav.side { position: sticky; top: env(safe-area-inset-top, 0px); align-self: start; height: 100vh; overflow-y: auto; padding: 1.25rem 1rem; border-right: 1px solid var(--line); background: var(--panel); }
nav.side h1 { font-size: 1rem; margin: 0 0 .15rem; letter-spacing: .01em; }
nav.side .sub { color: var(--muted); font-size: .8rem; margin-bottom: 1rem; }
nav.side a { display: block; padding: .38rem .6rem; border-radius: 6px; color: var(--ink); text-decoration: none; font-size: .9rem; }
nav.side a:hover { background: var(--soft); }
nav.side a[aria-current="page"] { background: var(--accent); color: var(--accent-ink); }
nav.side .toc { margin: .25rem 0 .75rem 0; padding-left: .5rem; border-left: 2px solid var(--line); }
nav.side .toc a { font-size: .8rem; color: var(--muted); padding: .2rem .5rem; }
nav.side button { margin-top: .75rem; width: 100%; padding: .5rem .6rem; border: 1px solid var(--accent); background: transparent; color: var(--accent); border-radius: 6px; font: inherit; font-size: .85rem; cursor: pointer; }
nav.side button:hover { background: var(--soft); }
nav.side .legend { margin-top: 1rem; font-size: .75rem; color: var(--muted); line-height: 1.5; }
main { min-width: 0; padding: 1.5rem 2rem 4rem; }
article { max-width: 62rem; margin: 0 auto; min-width: 0; }
article h1 { font-size: 1.9rem; line-height: 1.2; margin: 0 0 1rem; text-wrap: balance; }
article h2 { font-size: 1.3rem; margin: 2.2rem 0 .6rem; padding-top: .8rem; border-top: 1px solid var(--line); text-wrap: balance; }
article h3 { font-size: 1.05rem; margin: 1.5rem 0 .4rem; }
article p, article li { max-width: 72ch; }
article td p, article td li { max-width: none; }
article a { color: var(--accent); }
article code { font-family: var(--mono); font-size: .86em; background: var(--code); padding: .08em .35em; border-radius: 4px; word-break: break-word; }
article pre { background: var(--code); padding: .9rem 1rem; border-radius: 8px; overflow-x: auto; }
article pre code { background: none; padding: 0; }
article blockquote { margin: 1rem 0; padding: .6rem 1rem; border-left: 4px solid var(--accent); background: var(--soft); border-radius: 0 8px 8px 0; }
article blockquote p { max-width: none; margin: .5rem 0; }
article hr { border: 0; border-top: 1px solid var(--line); margin: 2rem 0; }
.tablewrap { overflow-x: auto; margin: 1rem 0; border: 1px solid var(--line); border-radius: 8px; background: var(--panel); }
table { border-collapse: collapse; width: 100%; font-size: .86rem; }
th, td { padding: .5rem .7rem; text-align: left; vertical-align: top; border-bottom: 1px solid var(--line); min-width: 6rem; }
th { background: var(--th); font-weight: 600; position: sticky; top: 0; }
tbody tr:nth-child(even) td { background: var(--zebra); }
tbody tr:last-child td { border-bottom: 0; }
.mk { display: inline-block; font: 500 .72em/1.5 var(--mono); padding: 0 .35em; border-radius: 4px; background: var(--soft); color: var(--accent); vertical-align: baseline; }
.mk-k, .mk-s { background: var(--warn-bg); color: var(--warn); }
.topbar { display: none; }
@media (max-width: 900px) {
  .frame { display: block; }
  nav.side { position: static; height: auto; border-right: 0; border-bottom: 1px solid var(--line); }
  nav.side .toc, nav.side .legend { display: none; }
  main { padding: 1rem 1rem 3rem; }
  article h1 { font-size: 1.5rem; }
}
</style>
<div class="frame">
  <nav class="side" aria-label="Pack contents">
    <h1>NexusAI roadmap pack</h1>
    <div class="sub">Written 2026-11-03 &middot; eight parts</div>
    <div id="links"></div>
    <button id="copy" type="button">Copy the new-chat prompt</button>
    <button id="copyall" type="button">Copy the whole pack (Markdown)</button>
    <div class="legend"><span class="mk">M</span> measured by us &middot; <span class="mk">V</span> primary page read &middot; <span class="mk mk-s">S</span> search summary only &middot; <span class="mk mk-k">K</span> prior knowledge &middot; <span class="mk">R</span> read from the repository or live system</div>
  </nav>
  <main><article id="doc" aria-live="polite"></article></main>
</div>
<script src="https://cdn.jsdelivr.net/npm/marked@12.0.2/marked.min.js"></script>
<script>
const PACK = /*DATA*/;
const PROMPT = /*PROMPT*/;
const links = document.getElementById('links');
const doc = document.getElementById('doc');
function currentId() {
  const h = (location.hash || '').replace('#', '');
  return PACK.some(p => p.id === h) ? h : 'start';
}
function mark(root) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, { acceptNode(n) {
    for (let p = n.parentNode; p && p !== root; p = p.parentNode) { if (p.nodeName === 'CODE' || p.nodeName === 'PRE') return NodeFilter.FILTER_REJECT; }
    return /\[(M|V|S|K|R)(?:[ ,\]][^\]]{0,60})?\]/.test(n.nodeValue) ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT; } });
  const nodes = []; while (walker.nextNode()) nodes.push(walker.currentNode);
  for (const n of nodes) {
    const frag = document.createDocumentFragment();
    let rest = n.nodeValue; const re = /\[(M|V|S|K|R)((?:[ ,][^\]]{0,60})?)\]/;
    while (true) {
      const m = re.exec(rest); if (!m) { frag.append(rest); break; }
      frag.append(rest.slice(0, m.index));
      const s = document.createElement('span'); s.className = 'mk mk-' + m[1].toLowerCase(); s.textContent = m[1] + (m[2] || ''); frag.append(s);
      rest = rest.slice(m.index + m[0].length);
    }
    n.replaceWith(frag);
  }
}
function render() {
  const id = currentId(); const item = PACK.find(p => p.id === id);
  doc.innerHTML = marked.parse(item.md, { gfm: true });
  doc.querySelectorAll('table').forEach(t => { const w = document.createElement('div'); w.className = 'tablewrap'; t.replaceWith(w); w.append(t); });
  doc.querySelectorAll('a[href^="http"]').forEach(a => { a.target = '_blank'; a.rel = 'noopener'; });
  mark(doc);
  links.innerHTML = '';
  for (const p of PACK) {
    const a = document.createElement('a'); a.href = '#' + p.id; a.textContent = p.title;
    if (p.id === id) { a.setAttribute('aria-current', 'page'); }
    links.append(a);
    if (p.id === id) {
      const toc = document.createElement('div'); toc.className = 'toc';
      doc.querySelectorAll('h2').forEach((h, i) => { h.id = 'sec-' + i; const t = document.createElement('a'); t.href = '#' + p.id; t.textContent = h.textContent.slice(0, 48); t.addEventListener('click', e => { e.preventDefault(); h.scrollIntoView({ behavior: 'smooth', block: 'start' }); }); toc.append(t); });
      links.append(toc);
    }
  }
  document.title = 'NexusAI Roadmap Pack';
  window.scrollTo(0, 0);
}
window.addEventListener('hashchange', render);
function wireCopy(id, label, getText) {
  document.getElementById(id).addEventListener('click', async (e) => {
    const b = e.currentTarget; const text = getText(); const done = (t) => { b.textContent = t; setTimeout(() => { b.textContent = label; }, 2200); };
    try { await navigator.clipboard.writeText(text); done('Copied'); }
    catch (err) { const ta = document.createElement('textarea'); ta.value = text; document.body.append(ta); ta.select(); try { document.execCommand('copy'); done('Copied'); } catch (e2) { done('Select the text on the page instead'); } ta.remove(); }
  });
}
wireCopy('copy', 'Copy the new-chat prompt', () => PROMPT);
wireCopy('copyall', 'Copy the whole pack (Markdown)', () => PACK.map(p => p.md).join('\n\n---\n\n'));
render();
</script>
"""


def main():
    pack = []
    prompt_lines = []
    for ident, name, title in FILES:
        with open(os.path.join(HERE, name), encoding="utf-8") as handle:
            text = handle.read()
        pack.append({"id": ident, "title": title, "md": text})
        if ident == "start":
            started = False
            for line in text.splitlines():
                if line.startswith("## The prompt to paste"):
                    started = True
                    continue
                if started and line.startswith("---"):
                    break
                if started and line.startswith(">"):
                    prompt_lines.append(line[1:].lstrip() if len(line) > 1 else "")
    data = json.dumps(pack, ensure_ascii=False).replace("</", "<\\/")
    prompt = json.dumps("\n".join(prompt_lines).strip(), ensure_ascii=False).replace("</", "<\\/")
    html = TEMPLATE.replace("/*DATA*/", data).replace("/*PROMPT*/", prompt)
    out = os.path.join(HERE, "roadmap-pack.html")
    with open(out, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(html)
    full = os.path.join(HERE, "ROADMAP_PACK_FULL.md")
    with open(full, "w", encoding="utf-8", newline="\n") as handle:
        handle.write("\n\n---\n\n".join(item["md"] for item in pack))
    print("wrote", out, len(html), "bytes;", full, ";", len(prompt_lines), "prompt lines")


if __name__ == "__main__":
    main()

