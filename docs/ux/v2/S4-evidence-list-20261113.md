# S4: the evidence list as a gallery of openable files (2026-11-13)

**Owner, 2026-11-13:** the first version (a table with a coloured tile and a row of figures) "looks like a list"; the old design at least made each evidence file look like something that opens. Asked for the best, cleanest, most modern page, every section world class, researched.

## Research (read for this section, marks as in the design reference)

* **Gallery over list for mixed media [S]**: Windows File Explorer's Gallery, Notion's gallery view and Magnet AXIOM's thumbnail view all rest on the same idea: a file is recognised by its face, not by its name. AXIOM's own reviewers say the thumbnail view is the one to use when there is a lot to look at. Notion's guidance: one preview rule for every card, so the grid reads evenly. A fixed aspect ratio keeps the layout still while pictures arrive [S: web.dev, Cloudinary].
* **Preview on hover and a quick look [S]**: Frame.io scrubs a video on hover; macOS Quick Look and its copies (Eagle, Directory Opus) open a larger look on the space bar, and a docked preview pane suits people who browse for long. Decision: a quick look (eye on the card, Space on the keyboard) over the same face; no docked pane (the owner dislikes right rails).
* **Remember the view [S]**: the most common complaint in file browsers (Blender, Figma forum) is that the grid/list choice is forgotten. Decision: the view, card size and row height are stored per browser.
* **Cards [S]**: one link per card (the name), made to cover the card with a stretched `::after`, so the card is one target and one tab stop; a card with several actions keeps them as separate buttons above the link (the eye).
* **Large galleries [S]**: load pictures lazily with an `IntersectionObserver` and a margin, cap how many requests run together, drop the ones whose card left, cache, give memory back. Decision: four at a time, a cache of 400, object URLs revoked on eviction.
* **Filters with counts [S]**: counts that describe the whole case, an option with no files dimmed, the filter in the address so the view can be shared. Row grouping: no source gave guidance; decision: the day a file was added (as in photo galleries), a quiet line, not a card.
* **Not found:** guidance specific to evidence-management products' gallery design (Cellebrite, Relativity); none is claimed.

## What was built

| Section | Decision |
|---|---|
| Head | Title, one line that counts the case (files, size, ready, failed, processing), Refresh, **Add evidence** (a dialog, not an inline fold). |
| The bar | One stacked bar of the case by kind (seven kinds, the six data colours plus a muted one); its legend is the filter, with count and share, and lights the same kind as the bar. |
| Tools | Search, state chips with case-wide counts (an empty state is dimmed), record type (only where it applies), card size (small, medium, large) or row height, **Cards / List**. |
| Cards | A face of a fixed 4:3 shape. **Photo:** a 480 px copy of the file. **Recording:** the file's real waveform, decoded in the browser. **Table file:** the first three rows and columns. **Document or text:** the first three passages, right-to-left when they are. **Video:** a designed face with its length and a sound mark, because a video is too big to read for a card (poster frames are a backend request). Chips: the kind and the one fact worth knowing (length, pixels, rows, passages). Hover lifts the card and shows the way in and the eye; a failed or processing file has its own border. |
| Quick look | A dialog with the same face, large, the file's facts and **Open evidence**. |
| List | The earlier table, kept for the analyst who wants rows; it gives up columns by its own width. |
| States | Skeleton cards of the final shape, "No file matches" with the way back, an error with Try again, and "No evidence yet". |

Nothing on a face is made up: where a file cannot be read cheaply the card shows its facts and an icon, never a guessed picture.

## Checks

* 34 tests (model 15, preview store 10, page 9) pass: counts, kinds, facts, grouping, the first rows and passages, the cache, the line of work, filters, the legacy `family=` link, quick look by eye and by space, failed files, list view and its memory, empty and error states.
* Looked at on :4182 in both themes: photos, waveforms, sheets, passages (Urdu and Pashto right-to-left), the video face, quick look, the list at the width of the pane, the add dialog, arrow keys across the grid.
* `scripts/ui-evidence-check.mjs` measures the page in a headless browser at six sizes in both themes (not run in this session: the launch was declined).

## Backend rows this needs (for `docs/work/BACKEND_REQUESTS.md`)

1. A poster frame for each video and a small copy of each photo, made at processing time (today the browser makes the copy of a photo and a video has none).
2. A page of the list that returns the first rows or passages with each item, so that a card does not need one more request.
3. The language of each recording and its reading status in the list item (they are in the artifacts, not in the list).
