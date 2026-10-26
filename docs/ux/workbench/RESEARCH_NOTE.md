# The link-analysis workbench: research note and direction proof

Written 2026-10-26 for the owner's decision. It is the first step of the interface programme's workbench stage (master plan, sections 7.3 and 7.6): **research, then two options on the application's own themes, then you choose, then it is built over live data.** Nothing here reads a live case: the proof runs on a synthetic fixture of the generator's development case 2 (12 numbers, 60 days), and every figure on screen is checkable against the oracle that scored stage 3.

**Where to look:** `http://localhost:4181/design/workbench` (layout A, side by side) and `?layout=b` (layout B, one pane at a time). Questions to try are in the Examples menu on the page; `?q=together|meeting|successor|move|speed|rhythm|connected` opens one, `?state=empty|broad|abstained|partial` shows the four designed states. Both themes follow the device setting or the appearance menu.

## Decided 2026-10-26: layout B, and where the workbench lives

The owner chose **layout B (one pane at a time: links, places or table, the timeline always underneath, the evidence of the selected mark on the right)** and asked for it to be built over the live reads. Then they decided **where it belongs**:

* **Ask is where questions are asked.** An answer of the Ask engine that can be drawn (a window of two numbers at one cell, a possible meeting, a candidate successor, a move, a speed alert, a change of rhythm, a connection, a list of contacts, cell sites) carries the workbench inside it, drawn from the rows the engine returned (`ResultViews` -> `WorkbenchResult`). Each mark opens its row; the strength of what the answer shows (Observed, Inferred, Candidate) and the values the engine took from the question are shown above it. The evidence sits below the picture in the conversation and beside it once the picture is widened.
* **Connections is the whole case.** Every subscriber and the contacts they share, the cell sites they used, the days they were active, and the evidence of whatever is selected. It has **no question field**: "Ask about this case" and the evidence panel's questions take the numbers on screen to the Ask page, which runs them (`?question=...&run=1`).
* **One implementation** (`pages/workbench/WorkbenchCore.jsx`), two homes; the state is in the address on Connections and in the answer inside a conversation.
* The design proof (`/design/workbench`) stays as the reference for the states, on its synthetic fixture.

What is real now: the connections and places reads, the Ask engine's answers, the mapping of an answer's rows to marks (tested against the real answers recorded on the synthetic development case 2), the lanes (events at a cell site a day, from the hourly visits), the period brush, the evidence panel, the table twin with CSV, the picture as PNG. What is not yet: events a day by number for events without a cell, the density of a place and the rows behind a mark as their own reads (BR-2), device and handset links, saved views, annotation.

## 1. Why the first draft was wrong

You said the first draft looked odd, messy and childish. Reading it against the guidance below, it broke the same rules the sources agree on: a rounded, shadowed box around every pane ("card soup"), pill buttons and a bright yellow alert for a caveat, large type and large radii, three fonts of emphasis, a coloured legend that repeated the toggles, and a diamond for every mark. The second draft keeps **one frame with hairline dividers**, one accent used only for what is selected or primary, small-capital labels, one weight for emphasis, 6 px radii, no shadow except on a menu, and tabular figures everywhere.

## 2. What was studied, and the rules taken from it

| Source | What it says that this design relies on |
|---|---|
| Shneiderman, "The eyes have it" (1996): overview first, zoom and filter, then details on demand | The workbench starts from **one question and a small working set**, filters by period and by analysis, and opens **details on demand** in the inspector. History is kept in the address (the state is a link). |
| [Cambridge Intelligence, graph visualization UX](https://cambridge-intelligence.com/blog/designing-intuitive-data-experiences-with-graph-visualizations/) and [visualizing large networks](https://cambridge-intelligence.com/blog/visualize-large-networks/) | Start with an empty or small view and grow it deliberately; style differs **only where it adds context**; hide labels too small to read; limit expansion around hubs (here: services and numbers everyone contacts are listed as **excluded, not hidden**); a **time bar** filters activity; level of detail by zoom; keyboard navigation and a text alternative. |
| Cambridge Intelligence's investigation showcase, the HisVA system and a VAST 2014 crime-analysis entry | **Coordinated views**: selecting a call in the timeline lights the callers on the map and the graph, a period set on the timeline filters every pane. The brush here is the workbench's, not a chart's. |
| [NN/g, data tables](https://www.nngroup.com/articles/data-tables/) | A human-readable first column; columns in order of importance; a header that stays in view; make sorting, filtering and hiding discoverable; **edit and inspect in a non-modal side panel, not a modal that covers the neighbouring rows**; batch actions beside the table. Tables compare adjacent values without holding them in memory. |
| [NN/g, dashboards and charts](https://www.nngroup.com/articles/dashboards-preattentive/) | Encode quantity by length and position; avoid pies, gauges and 3-D; **use colour sparingly, as a secondary cue, to make one thing stand out**; never use colour to show magnitude; about 4.5 percent of people are colour-blind. |
| [GitHub Primer, data visualization](https://primer.style/product/ui-patterns/data-visualization/) | Marks need 3:1 against the background and text 4.5:1; **more than colour must tell items apart** (stroke styles and markers); a legend only when there is more than one item, and none where items are labelled directly; tooltips on hover; a table or download for the chart's data. |
| IBM Carbon, [colour palettes](https://carbondesignsystem.com/data-visualization/color-palettes/) and the IBM data-visualization basics | An ordered categorical palette in which neighbouring colours contrast; **reserve red, orange, yellow and green for alerts**; do not fill the frame with too many elements. |
| Stripe, [accessible colour systems](https://stripe.com/blog/accessible-color-systems); Linear, [how the UI was redesigned](https://linear.app/now/how-we-redesigned-the-linear-ui) | Colours that meet contrast **by construction** (the same lightness steps for every hue, in a perceptual colour space); a more neutral chrome; elevation by surface tone, not shadow; aligned labels, icons and controls; reduced noise before added features. |
| W3C WCAG 2.2: 1.4.3, 1.4.11, 2.4.7 and [2.5.8 target size](https://www.w3.org/WAI/WCAG22/Understanding/target-size-minimum.html) | Text 4.5:1, interface marks 3:1, a visible focus, **pointer targets at least 24 px** (every control here is 28 to 30 px). |
| Palantir [Blueprint](https://blog.palantir.com/scaling-product-design-with-blueprint-25492827bb4a) (its blog was not retrievable; the documented intent: composable, desktop-first, consistent keyboard behaviour) | Keyboard consistency across the workbench; a dense, desktop-first layout with a resizable inspector. |
| Tufte on data-ink and small multiples | Erase non-data ink (boxes, heavy grids, fills); show lanes of the same shape side by side so that patterns compare. The timeline's lanes are small multiples; its grid is dotted and faint. |

The references the plan names (i2 Analyst's Notebook, Maltego, Linkurious, Cellebrite Pathfinder, Siren) publish little design guidance of their own; their agreed properties are the ones above (coordinated views, a small working set, evidence one click from every mark), which is why the proof is built on those.

## 3. The design, rule by rule

1. **Question first.** A question field, the answer in one sentence, the **values the engine read from the question** (and where each came from), the **caveat in its fixed words**, an **evidence-strength badge** (Observed, Inferred, Candidate: a word and a shape, never colour alone), a "How this was determined" disclosure (the engine's own steps), and a line of scope: case, evidence scope, events counted, repeats left out, case clock.
2. **One frame, hairlines.** Links and places side by side, the timeline under them, the **inspector** on the right, all in one surface; the working set is a bar across its top.
3. **A small working set, grown deliberately.** The numbers of the question and their strongest neighbours; "Expand one hop" adds one at a time; exclusions are counted and named.
4. **Coordinated panes.** A mark selected in any pane is selected in all (the link, the dot on the timeline, the site on the map, the row of the table); the brush filters every pane; **analysis layers** are toggles whose dot is the colour of their marks.
5. **Evidence one click away, in a side panel.** Values, the caveat, the rows (sortable, numbers on the right in tabular figures, copy as tab-separated text), and "Ask about this". The panel's width is the analyst's: drag the handle, or use the arrow keys on it; a double click restores it.
6. **Honest encoding.** Observed contact is a solid line, a same-place link is dashed, a candidate is dotted with an arrow where it has a direction; the caveat text sits beside every analysis; coverage areas are drawn as areas; the synthetic placement over a city is stated on the map.
7. **Level of detail.** Key labels only (the numbers of the question, the selected and the hovered); all or none on request; zoom, fit and three layouts (focus, circle, tree).
8. **A time bar that works.** Quick periods (all, 30 days, 7 days) and the brush; marks are dots on the lane's top rule.
9. **Keyboard.** `T` table, `F` fit, `[` and `]` step through the marks in time order, `Esc` closes the evidence, `?` lists them; the evidence handle responds to the arrow keys.
10. **Designed states** for empty, too broad, abstained and partial coverage; each says what happened, why and what to do.
11. **Both themes** from the application's tokens (D23 stays: the teal light and dark themes and Noto Sans are untouched).

## 4. The two options, and my recommendation

| | A, side by side | B, one pane |
|---|---|---|
| What you see | Links and places together, the timeline under them | One of links, places or table at a time, the timeline always under it |
| Strength | The coordination is visible: select a window and the link, the site and the dot light together | Calmer; each pane gets room; works on a laptop screen |
| Cost | Needs width (comfortable from about 1,400 px) | A mark in the hidden pane is not seen until you switch |

**Recommendation: build A, and let it fall back to B's behaviour below about 1,200 px**, so it is one design with two widths, not two products. The inspector, the working set, the time bar and the table are the same in both.

## 5. What is real in the proof and what is not

* **Real:** the numbers on screen are the oracle's answers on a synthetic case, the caveats are the engine's own texts, the derivation steps are the engine's wording, the tables' rows are the case's own records, and the layout, themes, keyboard and states are the application's.
* **Fixture, not yet read from the system:** the proof loads one JSON file. To make it live the engine's answer must carry a **structured payload** (the view, the parameters used, the rows as marks with their times and cells), and three backend reads are needed (BR-2: events a day by number, the density of a place, the rows behind a mark). Both are small and are listed in the plan's backend batches.
* **Not in this version:** free drawing and annotation on the chart, cross-case views, merging entities, export with the evidence appendix (the plan puts the last in the stage after this one).

## 6. What I need from you

1. **Open the page** in your browser and tell me what still looks odd, messy or childish, in your own words: I will change it before anything is built.
2. **Choose A or B** (or ask for a third once), and the **density** you want as the default (compact as shown, or more spacious).
3. A **go** to build it over the live reads: the next steps are the structured payload of the engine's answers, the three backend reads, then replacing the Connections and Places pages by this workbench.
