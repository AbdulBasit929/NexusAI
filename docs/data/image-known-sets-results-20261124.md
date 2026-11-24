# Watch lists and known sets for images: results (2026-11-24)

Pre-registration (written before the matching code): `docs/data/image-known-sets-preregistration-20261124.md`. Everything below is built and tested, and **it went live on 2026-11-24 14:40 PKT on the owner's word** (section 4a: migration 039 applied after a backup, API image `known-sets-20261124`, `FORENSIC_IMAGE_KNOWN_SETS=true`). The gates were measured before that, on the throwaway database and on the stored Lahore hashes; section 4a repeats the check on the live stack. A gate that is missed stays missed.

## 1. Gates

| Gate | Result | Evidence |
|---|---|---|
| **K1 recall** (the six originals as a set, hashes only: 12 of 12 originals and re-saved copies flagged) | **MET**: 12 of 12 | `tools/media_data/known_sets_gate_check.py` on the stored hashes of the 97 Lahore pictures (read-only query of the live database; the function's own rule: same SHA-256, or hash within 56 bits). Re-saved copies were 12, 12, 12, 12, 16 and 20 bits from their original (limit 56) |
| **K2 no false flags** (the other 79 pictures) | **MET**: 0 of 79 | Same run. The nearest unrelated picture to any member is 108 bits away (limit 56): 52 bits of room |
| Reported only: the six `-cropped` copies | 0 of 6 flagged | A crop beyond 56 bits is the stated limit of a hash set. The copy search with the picture attached (DINOv2 and parts) finds crops; a known set does not |
| **K3 coverage line** | **MET** | Go database test: "Checked 5 of 6 images against 2 hashes in 1 set. 1 image has no perceptual hash yet and was checked by file hash only." with the counts equal to the database; the statement function is unit-tested for the singular and plural forms and for "No known set is active" |
| **K4 isolation and retirement** | **MET** | SQL smoke test (migration rehearsal) and Go database test: another tenant sees no set and no flag and gets 404 when it tries to retire or extend this tenant's set; a retired set produces no flags, cannot be extended (409), keeps its hashes, and frees its name; the runtime role can create sets, add hashes and write events but cannot rename a set, change its kind, delete anything, or update a hash or an event; row-level security leaves a tenant its own rows only |
| **K5 hash-only** | **MET** | No `bytea` column in the three tables (checked in `verify.sql`, `smoke.sql` and the Go test); labels and source references are limited to 512 characters by constraints (label over 512 refused, tested); the only way to add a picture is its hashes (a stored picture's hashes are copied; a table of hashes is read) |
| **K6 audit** | **MET** | One event row per set created, per add, per retirement and per check, every one with an actor (the Go test counts 2 / 3 / 1 / 3 for the sequence it ran, and checks that no event lacks an actor) |
| **K7 speed** | **MET** | 97 pictures against 1,000 hashes: 34 ms; against 10,000 hashes: 245 ms (limit 1 s), exact scan, on the throwaway database. Larger sizes not measured, by the owner's decision |
| **U1 interface** | **MET** (see amendment A1) | A set is made from selected pictures in 3 clicks after picking them (Select, Add to a set…, Create and add N selected); a flagged card carries the set name on the picture; the Known sets button counts the flagged pictures of the case at the top of the page; the "Flagged" chip puts the hits first; "Hide known harmless" removes only pictures that no watch list flags, keeps a visible "N hidden" with a one-click "Show", and is remembered; every hidden picture is one click away; evidence still opens |

## 2. What was run

* **Migration 039** rehearsed on a throwaway PostgreSQL: preflight, apply, verify, smoke, a second apply (idempotency), the rollback refusing without its setting, the gated rollback, re-apply, verify: all passed on the first run (`db/forensic_records/ops/039_image_known_sets/Rehearse-039.ps1`).
* **Go**: unit tests (table parsing, validation, coverage sentence, handlers refusing bad input before touching the database, the switch off by default) and database tests (the full flow through the handlers against the rehearsed database, and the speed run): all pass.
* **Interface**: 90 tests pass in the affected suites (the model of flags and hiding, the client calls, the Evidence page with flags, the Flagged chip, hide, picking, create-and-add, table import, retire-with-confirmation, the feature absent when the server has no known sets, and the earlier viewer and client tests).
* **Looked at in the real gallery** (Lahore case, port 4182) with the known-sets calls answered by a stand-in in the page, because the live API does not have them yet: the flags on the picture, the count on the button, the pick marks, the panel, creating a set from two pictures, hiding. Spacing was corrected once (the flag first landed on the card body instead of the picture).

## 3. Limits stated

* Two tiers only: the identical file and the same image saved or resized (hash within 56 bits). A heavy crop, a frame or a screenshot around a picture does not match a set hash.
* The perceptual hash is a PDQ-style re-implementation, not verified byte-compatible with Meta's PDQ, so shared hash lists in that format may not interoperate; PhotoDNA is not available to us.
* "Flagged" and "Hide known harmless" work on the page that is listed (48 pictures), not on the whole case: the case-wide numbers are on the Known sets button and in the answer of `GET /images/known`. A case-wide filter needs the list to be asked for the flagged ids (next step).
* Selecting works on pictures in the card view; a file that is not a picture cannot be picked.
* Who may retire a set: its creator or an administrator (with authentication off, anyone).
* Not built (in the pre-registered order): adding hashes from attached pictures or a zip in one step, an Ask question ("which pictures here are known?"), flags on image answers in Ask, sharing sets between tenants.

## 4a. LIVE since 2026-11-24 14:40 PKT (on the owner's "Yes, go live")

* **Backup** before anything: `live-20261124T093702Z.dump` in the main checkout's `.phase2-backups`, 533,548,942 bytes, 7,039 table-of-contents entries, SHA-256 `b9bd85da79e39095f7b192054acaf8f96d68ddb50c490ee2ee0e483da6b3fdc3`.
* **Migration 039** applied to the live database: preflight passed, apply committed, `verify.sql` passed; three new tables (`image_known_set`, `image_known_hash`, `image_known_event`), two functions. No existing table was touched.
* **API image** `known-sets-20261124` (id `9e3fca1e37a0`) = the live image `text-search-all-20261124` (`3b20b23d9043`) with the Go binary rebuilt from the repository working tree (HEAD's Go code equals the live code: no Go commit since 729e018 apart from the image-search files already live; plus the known-set files). Rolled out with `3-recreate-api.ps1`: the dry run showed exactly one variable added and none changed, and the after-check reported the same: `FORENSIC_IMAGE_KNOWN_SETS=true` (83 to 84 variables). Both workspace servers (4181, 4182) were restarted afterwards.
* **Rollback**: `.\tools\dev\live-window\3-recreate-api.ps1 -Apply -To before-known-sets-20261124 -RollbackTo before-known-sets-20261124` puts the API back on `3b20b23d9043`; or recreate with `-Set FORENSIC_IMAGE_KNOWN_SETS=false` (the interface then leaves the feature out). The migration is additive and is left in place; its destructive rollback is gated and exports nothing by itself.
* **Live check on the Lahore case** (through the workspace proxy): before any set, "No known set is active, so nothing was checked." (97 of 97 images with a hash). A real test watch list **"Test watch list: six originals"** (the six pictures that have copies, hashes only) was created and the case checked: **12 flagged pictures (6 identical, 6 same image at 12, 12, 12, 12, 16 and 20 bits), 0 others; "Checked 97 of 97 images against 6 hashes in 1 set."** The same numbers as the offline measurement (K1, K2). The Evidence page shows the amber badge on the flagged pictures, the count on the Known sets button (12), and the Flagged chip (9 on the first page of 48). The audit trail holds one `set_created`, one `hashes_added` and the checks, each with an actor.
* **After the deploy** the quick live smoke test passed 5 of 5 (142 s; engine-health 24 questions answered, no declines), and the photos-by-words route still answers ("a laptop": 4 results, searched 97 of 97). The other image routes were not re-run after the deploy; their code is unchanged.
* **The test set stays active** so the flags can be seen; retire it from the Known sets panel (Retire, then Retire it) when finished: its hashes are kept, it stops flagging, evidence is untouched.

## 4. What goes live, and what it needs (owner's word)

1. Migration 039 on the live database (backup first; `ops/039_image_known_sets/preflight.sql`, then apply, `verify.sql`).
2. An API image built from the repository HEAD plus these files (the live image is `text-search-all-20261124`), with `FORENSIC_IMAGE_KNOWN_SETS=true`.
3. Nothing for the worker or the search service. The interface already hides the feature when the server answers 404.
Rollback: switch the variable off (the interface leaves the feature out); the migration's rollback is destructive and gated, and the audit trail is exported first.

## 5. Amendments

* **A1 (2026-11-24, after the measurement, before any deploy).** The registered U1 clause "the flagged cards show their set name in the first screen" cannot hold for every case, because the cards are listed newest first and a flagged picture may be anywhere on the page (in Lahore the flagged laptop pictures were below the first screen). It was met instead by the count on the Known sets button (top of the page) and a "Flagged" chip that shows only the flagged pictures of the page. The gates K1 to K7 are unchanged.
