# Draft layouts that are not loaded yet

Files here are NOT read by the registry (`load_registry` reads `layouts/` only) and are not registered in `forensic.layout_versions`.

There is no file here now. `ipdr-seed-s8.v1.json` (the 8-column internet-record export of the repository's synthetic seed data) was parked on 2026-10-19 and moved to `layouts/` on 2026-10-20, after `tools/real_data/make_golden.py` learned to build an internet-record golden for an entry without an IMSI, ports or a public address and the registry tests were updated (14 entries). See `docs/data/layout-registry.md`, section 6a. A new draft that cannot be loaded yet goes here with a note of why.
