# Known sets (watch lists): the interface test I ran, and how to test it yourself (2026-11-24)

Run on the live stack (port 4182, case Orion Lahore Reference, the real API and database, no stand-ins), with real clicks and keyboard, plus read-only checks of the database. Feature description and limits: `docs/data/image-known-sets-results-20261124.md`.

## 1. What I tested, and what happened

| # | Check | Result |
|---|---|---|
| T1 | The page loads with the feature: Known sets button with a count, Select, Flagged and Hide chips | **Pass**: "Known sets 12", "Flagged 9" on page 1 |
| T2 | Flags are on the right pictures, on every page | **Pass**: page 1 has the 6 saved copies and 3 originals, page 2 the other 3 originals, page 3 none; 12 in all; **no cropped copy is flagged**; no badge on an unflagged picture |
| T3 | The badge and its tooltip | **Pass**: amber badge "Test watch list: six originals"; tooltip "Watch list: … (the same picture, saved or resized)" |
| T4 | Make a set from picked pictures (3 clicks after picking) | **Pass**: Select, picked the cat picture and the clock picture **through two different searches** (the choice is kept), "Add to a set…", name and kind "Known harmless", "Create and add 2 selected": "Set … created. 2 hashes added." |
| T5 | Hide known harmless | **Pass**: 48 cards became 45 with "3 pictures are hidden because a known-harmless set flags them. Show them"; the 9 watch-list pictures stayed; the choice survives a reload |
| T6 | Flagged chip | **Pass**: shows only the flagged pictures of the page (9) |
| T7 | Import a table | **Pass**: a table with one real file hash and one bad row: "1 hash added, 1 row could not be used."; the street sign became flagged at once (count 12 to 15); the pasted text is cleared |
| T8 | Retire needs a second click, "Keep" is the default, flags stop | **Pass**: "Retire" shows "Retire it / Keep" with **Keep focused**; Enter cancels; "Retire it" retires: "… retired. Its hashes are kept; it no longer flags anything."; the flags and the hidden pictures came back |
| T9 | List view | **Pass**: 9 flagged rows carry the badge (on a narrow pane the badge squeezes the file name, fine when wide); Select is cards-only |
| T10 | Keyboard | **Fixed** (see F1, F4), then **pass**: Escape closes the panel after a change, focus stays inside the dialog and returns to the Known sets button |
| T11 | Clicking a picture in Select mode picks it, does not open it | **Pass**: the address did not change, the card was marked picked, the bar said "1 selected" |
| T12 | Reload keeps everything | **Pass**: flags, counts and the hide choice came back from the server and the browser |
| T13 | The same name twice | **Pass**: "an active set with this name already exists" (capitals and spaces ignored), nothing created |
| T14 | Narrow screen (375 px) | **Pass**: no horizontal scroll; the controls wrap; the panel fits (16 to 359 px) |
| T15 | New pictures are checked by themselves | **Pass**: another session added a picture while I tested; the coverage line went from "97 of 97" to "98 of 98" with no action. It was a noisy, heavily compressed copy of the laptop picture: its hash is **128 bits** from the original (limit 56), so it is correctly **not** flagged |

Database, read-only, afterwards: the evidence is untouched (98 pictures); every audit row has an actor; the sets are the test watch list (6 hashes, active), my harmless set (3 hashes, retired) and a set "Baby sitting" that someone else made and retired while I was testing.

## 2. What the test found, and what I fixed

| # | Finding | Fix |
|---|---|---|
| F1 | After a submit (create, add, import) the keyboard focus fell to the page body, so **Escape no longer closed the panel** | Focus is put back inside the dialog after every change; covered by a test |
| F2 | An import said "1 row could not be used" without saying which | The panel now lists the rows ("Line 3: the file hash must be 64 hexadecimal characters"), up to eight, then "and N more" |
| F3 | The Flagged chip counted pictures that "Hide known harmless" had hidden (chip 12, result 9) | The chip counts what it would show |
| F4 | Closing the panel did not return focus to the Known sets button | It does now (the hidden browser pane delays this by about a second; a real window does not) |

Also seen, not changed: every visit of the page writes one audit row for the check (71 so far), which is noisy but complete; the Flagged chip and Hide work on the page you are viewing, not the whole case.
Tests after the fixes: 56 pass in the evidence and client suites (17 on the page, including the keyboard, the rejected rows and the chip count).

## 3. Test it yourself (about 10 minutes)

Open `http://localhost:4182/cases/orion-lahore-reference-20261030/evidence?kind=image`. The test watch list "Test watch list: six originals" is active.

1. **See the flags.** Expect "Known sets 12". Scroll: amber badges on the saved copies (`-resaved`) and the originals of the laptop, backpack, market, bus, truck and motorcycle pictures. The `-cropped` copies have **no** badge (a crop is beyond a hash set; use Image in Ask for crops). Hover a badge for how it matched.
2. **Flagged chip.** Click it: only the flagged pictures of the page remain. Click again to go back.
3. **Make your own set.** Click Select, tick two or three pictures (also try clicking the picture itself: it ticks, it does not open), click "Add to a set…", name it, choose Known harmless, click "Create and add N selected". Expect the message and the new set in "Your sets".
4. **Hide the noise.** A "Hide known harmless" chip appears with the count. Click it: those pictures disappear and a line says how many are hidden, with "Show them". Reload the page: your choice is remembered. Pictures on the watch list never hide.
5. **Import a table.** Known sets, "Or a table of hashes": paste
   `sha256,label` on the first line, then a line with a real 64-character file hash and a label, then a line `not-a-hash,bad`. Click "Import the table". Expect "1 hash added, 1 row could not be used." and "Line 3: …". (A file hash is on every evidence page, and the Quick look shows its first 16 characters.)
6. **Retire.** Click Retire next to your set: it asks "Retire it / Keep" (Keep is focused). Click Keep: nothing changes. Retire again and confirm: the flags of that set disappear; nothing is deleted.
7. **Keyboard.** Open the panel, make a change, press Escape: it closes and focus is back on the Known sets button. Tab goes through the panel without leaving it.
8. **Try to break it.** Create a set with the same name again (it is refused); try an empty name; paste a table with only bad rows; resize the window to phone width.
9. **What should not happen.** No picture is ever deleted or changed; a set never shows a picture's content; nothing names a person.

If something looks wrong, note the picture name, what you clicked, and the message you saw; the audit trail records who did what (`forensic.image_known_event`).
