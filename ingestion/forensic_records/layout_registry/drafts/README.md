# Draft layouts that are not loaded yet

Files here are NOT read by the registry (`load_registry` reads `layouts/` only) and are not registered in `forensic.layout_versions`.

* `ipdr-seed-s8.v1.json` (2026-10-19): the 8-column internet-record export of the repository's synthetic seed data (`timestamp, source_ip, destination_ip, domain, protocol, bytes, subscriber_id, session_id`). Checked offline on the stored file of the multimodal acceptance collection: the registry matches it at score 1.0 and all 2,500 rows are accepted (0 quarantined; the `Z` instants are stored as UTC; one volume column with no direction, not mapped).
  **Why it is parked:** the golden-file tool (`tools/real_data/make_golden.py`) assumes every internet-record entry has an `imsi` field (7 tests fail with `KeyError: 'imsi'`) and `test_layout_registry.py` pins exactly 13 ids. To load it: let `make_golden` build an entry without an IMSI, update the id set and the counts in the tests, move the file to `layouts/`, then (on the owner's go) rebuild the worker overlay and register the entry. The roadmap row "Type the four older collections" (section 11) has the order.
