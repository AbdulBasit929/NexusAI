# EasyOCR role with the Latin re-read: end-to-end live check (2026-11-19)

Question: does the role work as a product, from an upload to an answer in Ask and a view in the interface, and not only as a function inside the worker? Method: eight synthetic images (development partitions, clean variant; nothing sealed) were uploaded through the live upload endpoint (`tools/showcase/upload_case.py`, the analyst's own path) into a **throwaway collection** `ocr-easyocr-e2e-test-20261119`; the live worker (`easyocr-reread-20261117`) read them; the stored readings were compared with the known truth; nine questions were put to the live Ask engine; the new interface (:4182) was opened on the case. Scripts: `tools/media_data/e2e_live/` (case builder, readings check, questions).

## 1. Upload and reading

All 8 files were accepted (HTTP 202) and read by the worker one after another in about 2 minutes (the Urdu pages about 20 s each). The jobs table shows 0.0 to 0.1 s per job for these image jobs because it records the queue step, not the media reading; the worker log shows eight "media processing completed" lines spaced 10 to 20 s apart.

## 2. Stored readings (`forensic.derived_artifacts`, type `forensics.image-ocr-observation/v1`)

| File | Lines stored | Reader | Latin boxes re-read | Planted identifiers found |
|---|---|---|---|---|
| urpage-naskh-004 | 15 | EasyOCR | 18 | 15/15 |
| urpage-nastaliq-003 | 14 | EasyOCR | 16 | 15/15 |
| mixed-form-001 | 7 | EasyOCR | 5 | 4/4 |
| mixed-statement-001 | 19 | PP-OCRv5 (router: little Arabic script) | 0 | 18/18 |
| mixed-receipt-000 | 8 | EasyOCR | 11 | 7/7 |
| mixed-chat-002 | 8 | EasyOCR | 13 | 3/3 |
| mixed-mixline-000 | 1 | EasyOCR | 1 | 1/1 |
| en-20261021-000 | 30 | PP-OCRv5 | 0 | 27/27 |

**90 of 90 identifiers** are in the stored readings; every line is flagged `manual_review_required`; the observation keeps the original EasyOCR text of each re-read box, the model hashes (detector, recogniser, Latin recogniser) and the box polygon. One extra record exists on the short mixed line: an ANPR observation (confidence 0.32) from the plate role looking at a non-plate image.

## 3. Ask (live engine, `/query/hybrid`)

| Wording | Result |
|---|---|
| "Which **document** mentions ..." (9 questions) | all answered "The requested analysis capability or authorized source is unavailable": the question routes to the document family and this collection holds only images |
| "Which **images** mention Ahmed Raza / contain the phone number ... / mention Rs. 144,665 / contain the word کریانہ" | **5 of 9 answered with the right file and the exact stored line cited** (name on a mixed form, phone on an Urdu page, amount with the "Rs." prefix, phone on a short mixed line, an Urdu word on a receipt); 7 to 8 s each |
| "...the CNIC 24017-..." | withheld by design: national ID numbers are counted, not shown, until the masking design is decided |
| "...the vehicle number LEB-19-2672" and "...contain the number 0344-5990591" | **routed as plate searches** ("No image or video reading matches the plate ...") and so missed text on a form and a chat |
| "...mention IMEI 2147..." | routed to Call detail records (an IMEI is a column there) and declined |
| The same three reworded as "Which images contain the **text** ..." | **all three found**, with the exact line cited |

So the stored text is fully searchable; the misses are Ask routing: words that name the place to look decide the route (images, not documents), and "the number" or "the vehicle number" reads as a plate. This is an Ask-engine item, not an OCR item.

## 4. Interface (:4182)

The case appears on the dashboard (latest evidence, time to ready 1 min 41 s); the evidence list shows eight image cards that load their thumbnails lazily; the file view shows the image with observation overlays and "Text in the picture" with each line and its confidence (99% on the amount line, 41 to 77% on Urdu lines), and the amounts read correctly ("Rs. 144,665"). Gaps seen: the viewer does not say which reader produced a line or that it needs review (only the confidence percentage); at 23% zoom the overlay labels of right-to-left lines run past the right edge of the page.

## 5. What it shows about OCR quality

Urdu text still has character errors that exact search cannot forgive: the stored lines read "گاڑی نمبل" for "گاڑی نمبر" and "نیو لاہول کریانہ" for "نیو لاہور کریانہ", so a search for "لاہور" on that receipt would miss. The identifiers, which carry the forensic value, were all found.

## 6. State left behind

The throwaway collection `ocr-easyocr-e2e-test-20261119` (8 evidence items) stays in the live database, like the two earlier test collections, and is listed in the new interface's runtime config (`NexusAI-ui` tree, uncommitted). Deleting evidence is the owner's decision.
