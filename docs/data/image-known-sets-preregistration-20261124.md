# Watch lists and known sets for images: design and pre-registered gates (2026-11-24)

Owner decision 2026-11-24: build watch lists first. This note is written **before** the matching code, so the gates cannot be bent to the result. A gate that is missed stays missed and is reported as missed; changes are recorded as dated amendments at the end.

## 1. What it is

A **known set** is a named list of pictures that are known in advance, stored as **hashes only**: the SHA-256 of the file and the 256-bit perceptual hash (the same PDQ-style hash the image search already stores). Two kinds:

* **of interest** (a watch list): pictures the analyst wants flagged wherever they turn up;
* **known harmless**: pictures that are noise in this work (stock photographs, logos, app screenshots, cartoons) and that the analyst wants out of the way.

Every picture of a case is checked against the active sets using the hashes already stored for it. A match is **flagged** ("in set X") and may be **hidden from the gallery by the viewer's choice**. Nothing is deleted, merged or changed on the evidence; hiding is a view filter, off by default.

## 2. Rules built in

* **Hash-only.** The picture itself is never stored in a set. The set tables have no column that could hold picture bytes; a set built from stored evidence copies hashes, and a set built from attached pictures keeps their hashes and file names, not the pictures.
* **Evidence is never touched.** Flagging reads; hiding filters the view; retiring a set stops its flags, and the hashes stay for the record.
* **Tenant isolation.** Sets belong to a tenant (row-level security as in migration 038); a set of one tenant is invisible to another. Sets are not tied to one case, so a watch list works across cases of the tenant.
* **Audit.** Creating a set, adding hashes, retiring a set and every check of a case against sets write an append-only event row (who, when, what, how many).
* **Honest coverage.** Every answer says how much it checked: "Checked N of M images against K hashes in S sets"; pictures that have no hash yet are counted apart.
* **Not an identification.** A flag says "this picture is, or is a re-save of, a picture in set X", nothing about people or about what the picture shows. Faces stay a separate governed module.
* **Stated limits.** Two tiers only: identical file (SHA-256) and the same image re-saved or resized (hash within 56 bits, the calibrated distance of the copy search). A heavy crop, a frame, or a screenshot around a picture will not match a set hash; the copy search with the picture attached remains the tool for those. The perceptual hash is a PDQ-style re-implementation, not verified byte-compatible with Meta's PDQ, so shared hash lists in that format are not guaranteed to interoperate; Microsoft PhotoDNA is not available to us.

## 3. Scope of version 1

In: migration 039 (three tables, one read function); API to create, list and retire sets, add hashes from a table (CSV) or from stored evidence, and to list which pictures of a case are flagged; the interface (sets panel on the Evidence page, flags on gallery cards, a hide switch, import); tests; a rehearsal on a throwaway database; measurement of the gates below.
Out (later, in this order): adding hashes from attached pictures or a zip in one step; an Ask question ("which pictures here are known?"); flags on image answers in Ask; sharing sets between tenants; a DINOv2 tier (needs stored vectors, which breaks "hash-only").

## 4. Registered gates

Test bed: the Orion Lahore Reference case (97 pictures: 48 different everyday photographs, 12 deliberate saved and cropped copies of six of them, plus 37 earlier pictures), the set built from stored evidence. Ground truth: the file names (`-resaved`, `-cropped` mark copies of the stated original; everything else is a different picture), already used for the duplicate-groups check.

| Gate | Pass condition |
|---|---|
| **K1 recall** | A set made of the six originals that have copies (hashes only): every picture of the case that is the identical file or a re-saved or resized copy of a set member (hash within 56 bits, or same SHA-256) is flagged: 100 % of those members (the six originals and their six `-resaved` copies, 12 of 12). |
| **K2 no false flags** | The other 79 pictures of the case (97 minus the 12 members and re-saved copies, minus the six `-cropped` copies), which are neither a member nor a copy of one: 0 flagged. The six `-cropped` copies may be flagged or not (a crop beyond 56 bits is a stated limit); they are reported separately and are not a failure either way. |
| **K3 coverage line** | Every flag answer carries "Checked N of M images against K hashes in S sets" with N, M, K, S equal to the database counts; pictures without a hash are counted as not checked. |
| **K4 isolation and retirement** | On a throwaway database: a set of tenant A is invisible to tenant B (list, add, check); a retired set produces no flags; a probe collection other than the requested one never leaks; the runtime role can read and insert hashes and events but cannot update or delete a hash or an event, and can only set the retirement columns of a set. |
| **K5 hash-only** | After creating a set from evidence and from a table, the three tables contain no picture bytes: no column of type bytea, no text larger than 512 characters in `label` or `source_ref` (enforced by constraints and checked in the smoke test). |
| **K6 audit** | One event row per create, add, retire and check, with the actor and the counts; no event row without an actor. |
| **K7 speed** | Checking the 97 pictures of the case against sets of 1,000 and of 10,000 hashes takes under 1 second each on this machine (exact scan, no index). Larger sizes are not measured, by the owner's decision on scale tests. |
| **U1 interface** | Starting from the Evidence page, a set is created from selected pictures in at most 3 clicks; the flagged cards show their set name in the first screen without scrolling in the case above; the hide switch removes the flagged cards of known-harmless sets and keeps a visible "N hidden" count; every hidden picture is one click away; evidence still opens. |

Measured and reported in `docs/data/image-known-sets-results-20261124.md` (written when the numbers exist).

## 5. Amendments

* **A1 (2026-11-24, after the measurement, before any deploy):** the U1 clause about the first screen is met by the count on the Known sets button and a "Flagged" chip, not by card order; see `docs/data/image-known-sets-results-20261124.md`. Gates K1 to K7 unchanged.
