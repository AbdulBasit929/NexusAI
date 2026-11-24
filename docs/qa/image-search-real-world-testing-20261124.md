# Image search: real-world testing from the interface, and how word search works (2026-11-24)

Interface: **http://localhost:4182**, case **Orion Lahore Reference** (`orion-lahore-reference-20261030`, 97 pictures). The older step-by-step tests A to O are in `docs/qa/image-search-manual-test-guide-20261123.md`; this note adds the real-world view: analyst stories, a scorecard, what the test pack does **not** cover, and how to test with your own pictures.

Honesty rule used below: every "expect" line says whether it was **measured** (with the date) or is an **expectation to check** (not yet run). Write what you see in the scorecard (section 4); a result that disagrees with an expectation is the useful kind.

## 1. Eight analyst stories (do them in this order, about 30 minutes)

| # | Story | Steps in the interface | Expect | Status |
|---|---|---|---|---|
| R1 | **A picture was forwarded in a chat. Is it one of ours?** | Ask page, **Image** button, attach `reference\ref-laptop-toned.jpg`, press Ask. Repeat with `ref-laptop-mirror.jpg`, `ref-laptop-framed.jpg`, `ref-laptop-exact-copy-renamed.jpg` (the pack folder is `C:\NexusAI-Showcase\orion\image-search-test-pack\`) | The laptop picture and its two saved copies, each under its own label (identical / saved again / same photograph edited). Searched 97 of 97 | Measured 2026-11-23: exact 6 of 6, toned 6 of 6, mirrored 6 of 6, framed 4 of 6 (truck and bus framed missed) |
| R2 | **A screenshot of a phone showing the picture** | Attach `reference\ref-laptop-phone-screenshot.jpg`. Then crop the picture out of it (Windows: Win+Shift+S) and attach the crop | The phone screenshot: **no copy found** (stated limit). The crop: found | Screenshot miss measured 0 of 6; the crop step is an expectation to check |
| R3 | **What is this small detail, and which picture is it from?** | Attach `region-probes\imgsearch-street-people-267802__detail_small.jpg` | "Your picture appears inside 1 other picture", the street scene, "81 matching points agree", an **outline** on the thumbnail around the part | Measured 2026-11-23: 24 of 24 small details found; 193 of 216 probes as "contains it", 215 of 216 by any route, 0 false alarms |
| R4 | **Draw on a picture instead of taking a screenshot** | Evidence, open `imgsearch-laptop-502852.jpg`, **Find this part elsewhere**, drag a box on the laptop, **Search for this part** | "The selected part appears inside 2 other pictures", cropped (299 points) and saved copy (133), outlined; the boxed picture itself not listed | Measured live 2026-11-24 |
| R5 | **Where else does this kind of object appear?** | Open `imgsearch-motorcycle-540806.jpg`, draw a box on the motorcycle, search | The copies of that picture as "contains it" (verified). Other motorcycles can only appear as "a part looks like it" or "visually similar", marked **not verified** | Expectation to check. Different pictures of the same kind of thing are not matched by feature matching, only proposed by appearance |
| R6 | **Show me photos of X** | Type each sentence from section 3 below | The right pictures near the top; weaker ones under "Also possible"; things not in the case still return the closest pictures (stated limit) | Measured 2026-11-24 (table in section 3) |
| R7 | **Clean up before review: duplicates** | Type `show me the duplicate pictures in this case` | "7 groups of duplicate or near-duplicate pictures, 18 pictures in all; 11 could be set aside", one stack per group, every member opens | Measured live 2026-11-24 |
| R8 | **The whole flow in one sitting** | Type `photos of a laptop` , open the first result, draw a box on it, search the part, press **More like this** on a result, then **Find copies** | Every step opens from the previous answer with no retyping; each answer is one sentence plus pictures | Judge the flow with the checklist in section 5 |

## 2. What the test pack does not cover (real life has more) and what to try

The pack varies a picture in nine controlled ways. Real pictures arrive damaged in other ways. These were **not measured**; try each on a picture you know is in the case, and record found / not found:

| Real-life case | How to make it | What I expect |
|---|---|---|
| Sent through a chat app (re-compressed, shrunk) | Send a picture to yourself in a chat, save it, attach it | Found as "same image re-saved" or "same photograph edited" (the saved-copy and toned tests pass) |
| Photograph of a computer screen showing the picture | Phone photo of the Evidence viewer | Probably not a copy (reflections, moire); may show as "a part looks like it" |
| Photograph of a printed picture | Print, photograph | Probably not a copy; the part search may still find it if the print is sharp |
| Black and white, or a strong colour filter | Any editor or phone filter | Perceptual hash may fail; the DINOv2 route often holds (the toned test passes) |
| Meme or collage with text over the picture | Add text and a border | Like the framed picture: sometimes found; the part search should find the picture inside |
| Thumbnail or very small copy (under 300 px) | Shrink | Likely a copy match if the whole picture is shown; a tiny part will not match |
| Burst shots of one scene | Several phone shots a second apart | Appear as near-duplicates or "visually similar", not as one stack unless almost identical |
| A different photograph of the same place or object | Photograph the same sign from another angle | Not a copy; may appear as "visually similar" only |

## 3. Photos by words: what it does today (measured 2026-11-24, instant English path, Lahore case)

The pictures in the case are named for what they were chosen to show (`imgsearch-airplane-...`, `imgsearch-cat-...`); use the names as your answer key. In the case there are 2 airplane, 4 motorcycle, 3 bus, 3 market, 3 truck, 2 laptop, 2 phone, 1 cat and 1 clock original pictures (six pictures also have a saved and a cropped copy).

| Sentence | What came back first | Reading |
|---|---|---|
| a baby watching something on a laptop | the laptop picture and its two copies | Right |
| an airplane on a runway | both airplane pictures, then a bus as "also possible" | Right, one weak extra, labelled weaker |
| a person riding a motorcycle on a road | motorcycle pictures (including the copies of 540806 and 356623), but a street picture sits inside "best", and one motorcycle (394240) is only "also possible" | Mostly right, one wrong in the top group |
| a cat | the cat first, then a suitcase in the same top group | #1 right, #2 wrong |
| a clock | the clock first; a plate photograph as an "in a part of a picture" candidate | Right |
| a man selling vegetables at a market | the market picture and its saved copy, then a truck picture inside "best" | Partly right |
| a person holding a baby | a phone picture first, the baby-at-laptop pictures only "also possible" | Weak: a person doing something is harder than a plain object |
| an elephant (not in the case) | seven pictures as candidates | Known limit: it always returns its closest pictures; no firm "nothing here" |
| a swimming pool (not in the case) | five pictures as candidates | Same limit |

Reading it honestly: **single clear objects work well** (airplane, cat, clock, laptop, motorcycle); **people doing something, crowded scenes and things that are not in the case do not**. A words search is a ranked shortlist for a person to look at, never a complete or final list. The answer says so in its folded "What to keep in mind".

## 4. Scorecard (copy this table; one row per try)

| Story / sentence | Right pictures in the first 5 (of how many exist) | Wrong pictures in the first 5 | Seconds to answer | Would an analyst trust it? (1 to 5) | What was unclear |
|---|---|---|---|---|---|
| | | | | | |

How to judge words searches: count the true pictures of that kind that appear in **Best matches + Also possible** (recall), and the wrong ones among them (precision). The measured cut-off on 16 queries was precision 0.97 / recall 0.71 for "best", precision 0.78 / recall 0.79 for the wider band.

## 5. User-experience checklist for every answer

1. Is the first sentence enough to know what was found and how many? (Today: one sentence plus a coverage line.)
2. Are the strongest results visibly separated from the weak ones, and is "not verified" clear?
3. Does every picture open as the evidence file, and does the browser Back button return to the answer?
4. Is the outline on a "contains it" result around the right part?
5. Is the time acceptable (a few seconds for an attached picture; 20 to 60 s for a typed sentence, because a language model reads it first; about 45 s for the first search after a restart)?
6. Did it ever name or identify a person? (It must not.)
7. Could you act on the answer without reading the folded limitations? If not, say what is missing.

## 6. Test with your own pictures (30 minutes, recommended before any further build)

The Lahore pictures are everyday photographs (COCO); real casework pictures (phone photographs, screenshots, CCTV stills, documents) behave differently, so this is the most valuable test.

1. Create a **new case** for it (do not use Lahore: extra pictures change its counts and the verified question bank).
2. Add 15 to 30 of your own pictures through **Add evidence**. Include a few of these on purpose: one picture twice (original and a chat-forwarded copy), one picture and a screenshot of it, one cropped version, three pictures with text in them (signs, documents), two near-identical burst shots.
3. Wait until the case says all sources are ready. Each picture is described and cut into 34 windows when it arrives; with many pictures at once this competes with the language model for memory, so upload in batches of about 20 and do not ask questions until the batch is ready.
4. Run stories R1 to R8 with your own files and sentences (in English, Roman Urdu and Urdu script).
5. Everything stays on this machine; do not use pictures you are not allowed to hold.

## 7. How a words search works

1. **Ask** sees a sentence about photos and a language model reads it at run time (no keyword templates): it decides it is a photo search, extracts the description and, for Urdu or Roman Urdu, translates it (the answer shows the English it searched for). The server validates the result.
2. The description becomes a vector with the **text half of SigLIP 2**. Every picture already has a whole-picture SigLIP vector and 34 window vectors (halves, thirds, quarters), stored when it was uploaded.
3. The system compares the description with each picture's vector by cosine similarity, **exactly** (every picture of the case, 97 of 97; no index, no sampling). The comparison itself takes a fraction of a second; the language model step is what takes the seconds.
4. Window matches never compete with whole-picture matches: they are shown separately as **In a part of a picture** (outlined, marked not verified), so a small object inside a big scene is proposed without crowding out the real hits.
5. **Cut-offs are relative to the best score**, not fixed numbers: best matches are at least 0.88 of the top score, also possible at least 0.75. A fixed floor does not work: the top score for something that is not in the case (0.06 to 0.12) overlaps the top score for something that is (0.085 to 0.134).
6. The answer is one sentence, grouped pictures, the coverage line ("Searched 97 of 97") and a folded limitations note. Nothing is claimed as complete.

## 8. How it can be improved, ordered by value and cost

| # | Improvement | Why it helps | Needs | How we would prove it |
|---|---|---|---|---|
| 1 | **Prompt variants and splitting the sentence** (several phrasings of the description such as "a photo of a ...", and the main object separated from the action) | The wrong top hits above (suitcase for cat, street picture for motorcycle) are the model matching loosely; averaging several phrasings is a known cheap gain | No GPU; code in the Ask step | Re-run the 16-query calibration and the nine sentences above; register the gate first |
| 2 | **Mark relevant / not relevant, then refine** | Turns a shortlist into a search an analyst can steer in two clicks; uses stored vectors only | No GPU; one UI control, one search call | Count clicks and right pictures in the first 5 before and after on the scorecard |
| 3 | **Blend in the text read from the picture (OCR)** | Signs, documents and screens are found by their words, not their look; the OCR text already exists for the case | No GPU; the written-words and image routes combined | Sentences about text in pictures against the OCR ground truth |
| 4 | **A weaker, honest "no strong match" statement** | Today an absent thing still returns seven candidates. A case-relative test (how far the top score stands above the case's usual score for that sentence) might separate "present" from "absent" better than a fixed number | No GPU; an experiment first, it may not work | Present vs absent sentences on the case; adopt only if it separates them |
| 5 | **Categories to browse by** (vehicle, document, screenshot, animal, building, text-heavy) | Browse without typing; combine with words | No GPU; calibrated on captions | Counts against the answer key |
| 6 | **Captions and a vision-language check of the top candidates** | The real fix for "not in this case" and for people doing something | A GPU machine (parked) | Gate on present/absent sentences and on action sentences |
| 7 | **A larger vision model** | Better on crowded scenes | More CPU and memory per picture; re-index the case | Same calibration, plus index time per picture |
| 8 | **Calibrate on real casework pictures** | Every threshold above was set on COCO photographs | Your pictures (section 6) under the data rules already agreed | Recompute precision and recall per threshold on them |

Recommended order: 8 first (it decides everything else), then 1 and 2 (cheap, no GPU, visible), then 3, 4 and 5.
