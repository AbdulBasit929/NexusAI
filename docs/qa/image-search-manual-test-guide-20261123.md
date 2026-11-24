# Image search: what it does, what is live, and how to test it yourself (2026-11-23)

Case used for every test below: **Orion Lahore Reference** (`orion-lahore-reference-20261030`).
Interface: the new workspace on **http://localhost:4182** (the old design on :4181 does not have the image button).
Everything below is evidence you can check; where a result is a limit, it says so.

## 1. What image search does (plain words)

Inside one case, an analyst can do these things with the pictures:

| # | Function | How you ask | What you get | What it may claim |
|---|---|---|---|---|
| 1 | **Find copies of a picture I have** | Attach the picture in the Ask box (button **Image**, or drag it in) and press Ask | The case's pictures that are the same file, the same image re-saved or resized, or the same photograph edited, cropped or mirrored, each under the strongest label it earned | Completeness only against the stated distances and only for pictures that were processed (the answer says "Searched N of M images") |
| 2 | **Find a small part of a picture inside the other pictures** | Attach a crop, a detail or a screenshot of a region of a picture | "Your picture appears inside N other pictures", each with a **box showing where**, and how many matching points agree (12 or more = verified; 8 to 11 = probable) | Proven by many local features agreeing on one placement: the same picture or the same fixed object (sign, logo, vehicle, building) |
| 3 | **A part of another photograph that looks like it** | Same attach | Pictures whose *window* (halves, thirds, quarters of the picture) looks like your picture, outlined, labelled "not verified" | Candidates only |
| 4 | **Find copies of a picture already in the case** | Ask "find copies of imgsearch-truck-558137.jpg" (name it exactly), or click **Find copies** on any picture | The same labels, for that stored picture | Same as 1 |
| 5 | **Photos by words, in any language** | Ask "show me photos of a laptop" (English, Roman Urdu or Urdu script) | "Closest pictures to "a laptop": 3 best matches, and 2 more also possible", best first | **Ranked candidates.** A search by words always returns the closest pictures even if nothing in the case shows the thing: look at them |
| 6 | **More like this picture** | Ask "pictures that look like imgsearch-bus-124952.jpg", or click **More like this** on any picture | The nearest-looking pictures, ranked | Ranked candidates only |
| 7 | **Draw a box on a picture and find that part elsewhere** | Open any picture in the evidence viewer, press **Find this part elsewhere**, drag a box on the picture, press **Search for this part** | The same answer as 2 and 3 for the part you boxed (the picture it was cut from is left out): the pictures that contain it, outlined, and look-alike parts | Same as 2 and 3; the part is cut from the stored original, so it works without any file on your computer |
| 8 | **Duplicate and near-duplicate groups of the whole case** | Ask "show me the duplicate pictures in this case" | Stacks: one picture stands for each group, every member shows how it is linked (identical, saved again, edited) and opens as its own file; the answer says how many could be set aside | Nothing is hidden, merged or removed |

Every answer says how many pictures were searched, keeps its limitations in a folded "What to keep in mind", and each picture opens as the evidence file itself. No face or person is ever named by this feature (faces are a separate, governed module). The picture you attach is **not** kept in the case.

### The labels for a copy (strongest first)

1. **identical file (same SHA-256)**: the very same bytes, under any name.
2. **the same image, re-saved or resized**: a 256-bit perceptual hash within 56 bits.
3. **the same photograph, re-compressed or lightly edited**: a DINOv2 vector within cosine distance 0.108 (this is what catches most crops and mirrors).
4. **contains it (verified) / probably contains it**: the attached picture is a *part* of this one (local features, placement shown by the box).
5. **a part looks like it (not verified)**, **visually similar (not a copy)**, **best matches / also possible** (words): weaker, ranked kinds.

## 2. What is deployed and switched on (checked 2026-11-24)

| Part | State |
|---|---|
| Database | Migration 038 applied (tables `image_hash`, `image_search_log`, four search functions). No new migration for regions, windows or duplicate groups. |
| Worker | Image `image-search-20261123f` (unchanged since 2026-11-23): every new picture gets its descriptors and its 34 window vectors. Switches `FORENSIC_IMAGE_SEARCH_ENABLED=true`, `FORENSIC_IMAGE_SEARCH_TILES=true`. |
| Search service | `nexusai-forensic-image-search-1` on `image-search-svc-20261123f`: a crop endpoint for draw-a-box; models and their memory are released after 45 s idle; region matching with OpenCV at the settings chosen on measurements (contrast 0.006, 3000 features, 128 MB feature cache); windows computed in small slices so searches are not made to wait. |
| API | Image `text-search-all-20261124` (= `image-search-20261123k` plus the Ask-engine session's any-source text search, switched on 2026-11-24 about 11:17; rollback tag `before-text-search-all-20261124` = k; image search unchanged and re-checked): `/images/search` (copies, region, windows, words, more like this, duplicate groups, **the part you boxed**) and the Ask step, which now runs **before** the plate and written-words steps for sentences about photos. Switches `FORENSIC_IMAGE_SEARCH_ASK=true`, `FORENSIC_IMAGE_REGION_SEARCH=true`. |
| Lahore case | **97 of 97 pictures** have descriptors **and window vectors**. |
| Interface (:4182) | **Find this part elsewhere** in the picture viewer (draw a box, search for that part), image button and drop zone, attachment chip, grouped results with thumbnails, the outline where a region sits, duplicate stacks, **More like this** and **Find copies** on every picture, folded limitations. |

Rollback (nothing is deleted): `.\tools\dev\live-window\10-image-search.ps1 -Rollback` puts the worker and API back on the previous images (the API tag `before-image-search-20261123k` is the one before draw-a-box); the service is removed; evidence is never deleted.

## 3. The test files

Folder: `C:\NexusAI-Showcase\orion\image-search-test-pack\`

* `in-case\` (60 files, **already uploaded** to the Lahore case, so you do not need to upload them): 48 varied everyday photographs named `imgsearch-<what>-<id>.jpg`, plus 12 deliberate copies of six of them (`-resaved` and `-cropped`).
  The six that have copies: `imgsearch-motorcycle-540806`, `imgsearch-truck-558137`, `imgsearch-bus-124952`, `imgsearch-market-106909`, `imgsearch-laptop-502852`, `imgsearch-backpack-312878`.
* `reference\` (36 files, **the ones you attach**): for each of the six pictures above, five versions: `-mirror`, `-framed` (white border and caption strip, like a picture shared in a chat), `-toned` (brightness and contrast changed), `-phone-screenshot` (a phone screen with the picture inside it) and `-exact-copy-renamed` (byte-for-byte the same file, new name); plus six pictures that are **not** in the case (`ref-not-in-case-1…6`).
* `region-probes\` (27 files, **the ones you attach to find a part inside a picture**): for three pictures of the case (`imgsearch-street-people-267802`, `imgsearch-phone-205720`, `imgsearch-car-street-366111`), nine cut-outs each: `__detail_medium`, `__detail_small`, `__detail_tiny` (a region of 18-32%, 5-12% and 1.5-4% of the picture), `__rotated`, `__perspective` (as if photographed at an angle), `__low_resolution`, `__noisy_compressed`, `__annotated_screenshot` (a screenshot with a bar, text and a red arrow over it) and `__mirrored`. Make your own: open any picture in the evidence viewer, take a screenshot of a part of it (Windows: Win+Shift+S) and attach that.
* `manifest.json`: what each file is. `measured-results.json`: what the live system answered for each reference file on 2026-11-23.

Source of the photographs: COCO 2014 (Flickr, Creative Commons licences), evaluation use only.

**Important for the verified question bank:** the 60 `imgsearch-*` files are now evidence in the Lahore case, so questions that count photos or images in that case now see 60 more pictures than before. The files are named so they can be told apart.

## 4. Step-by-step tests in the interface

Open **http://localhost:4182/cases/orion-lahore-reference-20261030/investigate**. The question box is at the bottom.

### Test A: attach a reference picture, exact copy (about 10 seconds)
1. Click **Image** in the question box and choose `reference\ref-truck-exact-copy-renamed.jpg` (or drag it in). A small chip with the thumbnail appears.
2. Leave the text empty (or type "find copies of this") and press Ask.
3. Expect: "Found N copies or near-copies of … in this case, strongest match first", `imgsearch-truck-558137.jpg` first with the label **identical file (same SHA-256)**, then its re-saved copy as **the same image, re-saved or resized**. The line "Searched 97 of 97 images in this case." is shown. Each thumbnail opens the file.

### Test B: mirrored picture
Attach `reference\ref-motorcycle-mirror.jpg`. Expect three hits, all **the same photograph, re-compressed or lightly edited** (the original, its re-saved copy and its cropped copy), then at most a short "visually similar, not copies" list.

### Test C: brightness and contrast changed
Attach `reference\ref-bus-toned.jpg`. Expect `imgsearch-bus-124952.jpg` and its re-saved copy as **the same image, re-saved or resized**.

### Test D: a picture that is not in the case
Attach `reference\ref-not-in-case-4.jpg` (a jet in the sky). Expect "No copy of … was found in this case: no identical file, no re-saved or resized version, and no lightly edited version." plus the coverage line. Nothing is shown as a copy.

### Test E: the stated limits (these are expected NOT to be found)
* `reference\ref-*-phone-screenshot.jpg`: expected **no copy** (the interface around the picture changes it too much). Crop the picture out of the screenshot first and attach that.
* `reference\ref-*-framed.jpg`: **sometimes** found, sometimes not. A white border with a caption is the registered weak spot. See the table in section 5 for what happened.

### Test F: photos by words (typed question)
Type: `show me photos of a person riding a motorcycle on a road` and press Ask.
Expect: one short sentence, "Closest pictures to "a person riding a motorcycle on a road": N best matches, and M more also possible", then the pictures in groups (**Best matches**, **Also possible**, and, when a small thing may be in a part of another picture, **In a part of a picture**, outlined and marked not verified), motorcycles first. The cut-off between the groups was measured: best matches are those scoring at least 0.88 x the best one (precision 0.97, recall 0.71 on 16 queries against COCO captions), also possible from 0.75 x. A language model reads the sentence first, so on a busy machine this takes tens of seconds.
Other sentences to try: `photos of a bus on a street`, `a laptop on a desk`, `pictures of a market`, `رات کو سڑک پر موٹر سائیکل کی تصاویر` (Urdu: the system translates it and shows the English it searched for).

### Test G: a picture already in the case, by name
Type: `find copies of imgsearch-truck-558137.jpg` (the file name does not need the word "photo" or "image"). A language model reads the sentence first (about 20 seconds on this laptop).
Measured 2026-11-23: "Found 1 copy or near-copy of imgsearch-truck-558137.jpg in this case, strongest match first. Searched 97 of 97 images in this case." The one copy is `imgsearch-truck-558137-resaved.jpg` as **the same image, re-saved or resized**. The stored file itself is not listed as its own copy.
Honest limit seen here: `imgsearch-truck-558137-cropped.jpg` (a strong crop) is **not** found, and neither is the crop of the backpack picture. The crops of the motorcycle, bus, market and laptop pictures are found. A crop that removes too much of the picture is beyond the calibrated distance.

### Test H: opening a result
Click any thumbnail. The evidence file opens (the picture itself with its details).

### Test J: find a part of a picture inside the other pictures
Attach `region-probes\imgsearch-street-people-267802__detail_small.jpg` and press Ask.
Expect: "Your picture appears inside 1 other picture.", group **Contains your picture** with the street scene, label **Contains it**, "81 matching points agree. Outlined where it sits." and an **outline on the thumbnail** around the part you attached (here about 5% of the picture, right of centre). Time: about 45 s the very first time after a restart (the service has to read all 97 pictures once), a few seconds afterwards.

### Test K: the harder variants of the same
Attach, one at a time, the `__rotated`, `__perspective`, `__low_resolution`, `__noisy_compressed`, `__annotated_screenshot`, `__mirrored` and `__detail_tiny` files of the same picture. Measured on all 24 sources they come back as "contains it" in 23, 21, 21, 23, 20, 22 and 20 of 24 cases (section 5), and almost every other one comes back as a whole-picture copy or as "a part looks like it"; on these three sources all are found. The outline sits around the part for details; for a rotated one it is the box around the rotated part.

### Test L: duplicates in the whole case
Ask `show me the duplicate pictures in this case`.
Expect: "7 groups of duplicate or near-duplicate pictures, 18 pictures in all; 11 could be set aside." Seven stacks: laptop, motorcycle, bus and market (original, re-saved, cropped), truck and backpack (original and re-saved), and one pair of real plate photographs (`real-photo-PKPLATE-0074` and `-0064`) that look like the same scene taken moments apart. The first picture of each stack stands for it; the others say how they are linked ("hash differs by 12 of 256 bits", "same photograph, lightly edited"). Nothing is hidden: every member opens.

### Test M: one click from any picture
Under every picture are two buttons. **More like this** adds an answer with the pictures that look most like it (for the laptop picture: its crop and re-save, the other laptop picture, then the man holding a baby). **Find copies** adds the copies and look-alikes of that picture.

### Test N: a part the search should not claim
Attach a screenshot of a **plain** area (blue sky, a wall). Expect "no other picture contains it" or only a "part looks like it (not verified)" candidate: a plain area has no features to match, and the search never calls it verified.

### Test O: draw a box on a picture in the case
Open `imgsearch-laptop-502852.jpg` from Evidence (case Orion Lahore Reference). Press **Find this part elsewhere** (the button says "Drawing: drag on the picture"), drag a box around the laptop, press **Search for this part**. The Ask page opens with "Find the selected part of a picture elsewhere in the case".
Expect (measured live on 2026-11-24): "The selected part appears inside 2 other pictures. 4 look-alike pictures below (not copies). Searched 97 of 97 images in this case." The two are `imgsearch-laptop-502852-cropped.jpg` (299 matching points) and `imgsearch-laptop-502852-resaved.jpg` (133), each with an outline where the part sits; the picture you boxed is not listed. About 45 s the first time after a restart, a few seconds afterwards. Draw on a fitted, unrotated picture (the button is off while it is rotated).

### Test I: things it must refuse or hand on
* `which photos show the plate SYNT9150` goes to plate search, not image search.
* `who is the person in imgsearch-market-106909.jpg` is not answered by image search (no person is named by this feature).

## 5. What the live system answered for each reference file

Measured on the live system 2026-11-23 against the 97 images of the Lahore case (`measured-results.json` in the pack holds the distances).

| Picture | exact copy, new name | brightness/contrast changed | mirrored | white frame + caption strip | phone screenshot around it |
|---|---|---|---|---|---|
| motorcycle | found: identical file;<br>plus 2 of its saved copies | found: same image, re-saved;<br>plus 1 of its saved copies | found: same photograph, edited;<br>plus 2 of its saved copies | found: same photograph, edited;<br>plus 2 of its saved copies | **not found** |
| truck | found: identical file;<br>plus 1 of its saved copies | found: same image, re-saved;<br>plus 1 of its saved copies | found: same photograph, edited | **not found** | **not found** |
| bus | found: identical file;<br>plus 2 of its saved copies | found: same image, re-saved;<br>plus 1 of its saved copies | found: same photograph, edited;<br>plus 1 of its saved copies | **not found** | **not found** |
| market | found: identical file;<br>plus 2 of its saved copies | found: same image, re-saved;<br>plus 2 of its saved copies | found: same photograph, edited;<br>plus 2 of its saved copies | found: same photograph, edited;<br>plus 2 of its saved copies | **not found** |
| laptop | found: identical file;<br>plus 2 of its saved copies | found: same image, re-saved;<br>plus 2 of its saved copies | found: same photograph, edited;<br>plus 2 of its saved copies | found: same photograph, edited;<br>plus 2 of its saved copies | **not found** |
| backpack | found: identical file;<br>plus 1 of its saved copies | found: same image, re-saved;<br>plus 1 of its saved copies | found: same photograph, edited;<br>plus 1 of its saved copies | found: same photograph, edited;<br>plus 1 of its saved copies | **not found** |

Pictures that are **not** in the case:

| File | Result |
|---|---|
| `ref-not-in-case-1.jpg` | no copy found |
| `ref-not-in-case-2.jpg` | no copy found |
| `ref-not-in-case-3.jpg` | no copy found |
| `ref-not-in-case-4.jpg` | no copy found |
| `ref-not-in-case-5.jpg` | no copy found |
| `ref-not-in-case-6.jpg` | no copy found |

**Totals:** exact copy, new name: 6 of 6, brightness/contrast changed: 6 of 6, mirrored: 6 of 6, white frame + caption strip: 4 of 6, phone screenshot around it: 0 of 6. Pictures not in the case wrongly reported as a copy: 0 of 6.

Reading this honestly: the frame-and-caption version was missed for the truck and the bus (it is the registered weak spot), and a phone screenshot is never found. Crop the picture out of the screenshot or the framed share and attach that instead.

### Region evaluation (live, final settings, 2026-11-23): 24 pictures of the case x 9 variants = 216 attached pictures

| Variant | Came back as "contains it" | Verified (12+ points) | Box overlap with the true part (mean) |
|---|---|---|---|
| small detail (5-12% of the picture) | 24 of 24 | 24 | 1.00 |
| noisy and heavily compressed | 23 of 24 | 23 | 1.00 |
| rotated 8-25 degrees | 23 of 24 | 23 | 0.58 (scored against the unrotated rectangle) |
| mirrored | 22 of 24 | 22 | 1.00 |
| low resolution (shrunk to 35% and enlarged) | 21 of 24 | 21 | 0.99 |
| perspective-warped | 21 of 24 | 21 | 0.77 |
| tiny detail (1.5-4%) | 20 of 24 | 19 | 0.99 |
| annotated screenshot (bar, text, red arrow) | 20 of 24 | 20 | 0.70 |
| medium detail (18-32%) | 19 of 24 | 19 | 1.00 |
| **All** | **193 of 216 (89%)** | | |

The picture the part was cut from came back in **215 of 216** cases: 193 as "contains it" (with the box), **17 more as a whole-picture copy** (a large part is the same photograph, so it is listed with the copies, and the region pass skips copies), **5 more as "a part looks like it"**; **1 missed** (a tiny detail of the cat picture). An earlier figure of 86% in this guide counted only the first kind, on less sensitive settings.

False alarms: **none** on 14 probes cut from 7 pictures that are not in the case. (Two probes were cut from `contact.jpg`, a contact sheet of the case's own pictures; the search correctly found the case pictures inside it, 126 matching points each.) Chance matches between unrelated pictures never exceeded 6 matching points in 1,080 pairs, against the 12 needed for "verified".

The "a part looks like it" level is 0.25 (largest DINOv2 distance): at 0.15 / 0.20 / 0.25 / 0.30 the source came back as a part in 1 / 3 / 4 / 5 probes and 1 / 2 / 11 / 28 other pictures were listed.

## 6. Checking the same things without the interface (no language model involved)

From the project folder, in PowerShell (the key is read from the stack's secrets file and never shown):

```powershell
# a reference file: copies and look-alikes
python tools\media_data\image_search_live_check.py --collection orion-lahore-reference-20261030 file "C:\NexusAI-Showcase\orion\image-search-test-pack\reference\ref-bus-mirror.jpg"
# photos by words (English, instant path)
python tools\media_data\image_search_live_check.py --collection orion-lahore-reference-20261030 words "a person riding a motorcycle on a road"
# the region evaluation (needs OpenCV: run it inside the service image, see the script header)
#   python tools\media_data\image_region_eval.py --collection orion-lahore-reference-20261030 --pack C:\NexusAI-Showcase\orion\image-search-test-pack
# the words cut-off calibration
python tools\media_data\image_words_calibration.py --collection orion-lahore-reference-20261030 --pack C:\NexusAI-Showcase\orion\image-search-test-pack
# run all 36 reference files and write measured-results.json
python tools\media_data\image_search_reference_run.py --collection orion-lahore-reference-20261030 --pack C:\NexusAI-Showcase\orion\image-search-test-pack
```

## 7. Limits you should know (all stated in the answers too)

* Copies are found **against stated distances** (56 hash bits; 0.108 DINOv2), and only among pictures that were processed. The answer always says "Searched N of M".
* A picture shared in a chat with a border, or a screenshot of a phone showing it, can be missed. Crop first.
* A strong crop of a stored picture can be missed (4 of the 6 crops in the test pack are found; the truck and backpack crops are not).
* Photos-by-words are **ranked candidates**: the order is useful, a complete list is not claimed. Urdu and other non-Latin sentences are translated by the language model first; that route is not yet measured on reviewed captions.
* Pictures with no processed descriptors (uploaded after the switch was turned off, or failed) are not searched; the coverage line says how many.
* Exact search, no index: fast for a case of thousands of pictures; the time at 30,000 pictures was not measured (by decision).
* The language model that reads a typed question is slow on this laptop CPU (tens of seconds per read when the Docker VM is short of memory). The attach-a-picture route does not use it.
* A part of a picture is found when many local features agree. **Plain areas** (sky, water, a blank wall) and **very small parts** (under about 4% of the picture) are the weak spot: 20 of 24 tiny details came back as "contains it", the others mostly as a copy or "a part looks like it"; 1 of 216 was missed. A found region is verified; a missed one is not proof of absence.
* "A part looks like it" and "in a part of a picture" are **candidates by appearance, not verified**. They need the picture's window vectors; all 97 pictures of the Lahore case have them.
* A search by words always returns the closest pictures even if nothing in the case shows the thing: look at them as candidates, not as proof. Whole-picture scores for things that are not in the case (0.06 to 0.12) overlap those for things that are (0.085 to 0.134), so a firm "not in this case" needs a verification step (parked: it needs a GPU).
* A box drawn on a plain area or on a very small part has the same weak spot as an attached crop (see above). The box is cut from the stored original, so its quality is the original's.
* Nothing here identifies a person. Face search is a separate module with its own governance.
