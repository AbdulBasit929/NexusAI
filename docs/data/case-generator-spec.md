# Case generator with planted truth: specification v0.5 (roadmap Phase 1, section 7.5 and 8.3)

Status: **v0.5, 2026-10-13.** The owner approved the specification with the default answers of section 13. Steps 2, 3 and (in parts) 4 of section 12 are built (`tools/case_generator/`, section 14); the changes the builds forced are marked *(v0.2)* to *(v0.5)*. Nothing here reads a real file, touches a database or changes the product.

## 1. Why, and what it is

Link analysis cannot be checked against real data: nobody knows the true answers there. The case generator builds **synthetic, multi-provider cases in which the truth is planted and written down**, so that "did the system find the right people, meetings and swaps?" becomes a measured number.

A case is a folder of files shaped exactly like the 13 provider layouts of the layout registry (`docs/data/layout-registry.md`), with their mess, plus an **answer key**. The product ingests the files as it would real ones; the answer key says what it must find.

It serves three uses:

1. **Oracles for the link analyses** (roadmap A1 to A18): exact analyses must equal the key at 100%; inferred ones report precision and recall.
2. **A source of questions with known answers** for the question factory (multi-provider, IP attribution, common contacts, paths, co-location).
3. **Test and later training data** for the query-writer bake-off (never mixing sealed test seeds into training).

Not in scope: modelling real behaviour, real places or real people; learning from the real samples; media files (documents, speech, images, video) in v1 (plant P9, section 4).

## 2. Principles

| # | Principle |
|---|---|
| G1 | **Truth first.** The generator creates the world and its planted facts, then renders files from it. The key is written from the world, never read back from the files. |
| G2 | **Nothing real.** Every number, identifier, address and coordinate comes from a reserved range (section 8). The generator never reads the real-data vault or any real file. |
| G3 | **Same bytes for the same seed.** A seed, the generator version and the specification version reproduce a case byte for byte. |
| G4 | **Plants have decoys.** Every plant ships with near-misses that must not satisfy it (a co-location just outside the window, a common contact that is a service number). A system that finds everything is as wrong as one that finds nothing. |
| G5 | **Mess is part of the truth.** Defects (blank rows, unparsable times, duplicate rows, truncated exports) are injected on purpose, and the key lists what each one must become (quarantined, merged, flagged). |
| G6 | **Independent oracles.** The expected answers in the key are computed by a plain-Python reference over the canonical truth, not by SQL and not by the product. |
| G7 | **Fresh seeds for judgement.** Development seeds are used while building; judgement seeds are generated before the code they judge is measured; sealed seeds stay unopened until a gate. |
| G8 | **Small in the repository.** The generator and a tiny case live in the repository; large cases are regenerated locally and ignored by git. |

## 3. The world model (the canonical truth)

All plants are made of these objects. The canonical truth is the single source for both files and key.

| Object | Fields | Notes |
|---|---|---|
| Person | `person_id`, role (`subject`, `associate`, `bystander`, `service`) | Labels like `P-017`; no names |
| Subscription | `msisdn`, `imsi`, `operator`, `active_from`, `active_to`, `person_id` | One person may hold several over time; numbers come from the project block |
| Device | `imei`, `tac`, `model_label` | Test TAC only; a device may move between subscriptions (device swap) |
| Cell site | `cell_key`, `operator`, `lat`, `lon`, `radius_m`, `sector`, `rat` | A fictional grid (section 8); the same cell may appear in several operators' dumps under different cell columns |
| Service numbers | short codes, sender IDs, helpline-style labels | Reached by many; must be excluded by hub rules |
| Public address pool | documentation IPv4 and IPv6, port ranges | For address-translation collisions |
| Calendar | period (default 90 days), local zone `Asia/Karachi`, daily rhythm per person | Local time is what files show; UTC is the key's clock |
| Events | `event_uid`, kind (`call`, `sms`, `data_session`, `app_usage`), subject subscription, counterparty, start, end, cell, device, volume | The canonical stream; every row of every file derives from it |

A **projection** says who sees what: a target export holds the events of one subject (for the subject's operator), a tower dump holds the events of all subscriptions of one operator at a set of cells in a window, an IPDR export holds the data sessions of one subscription or one public address. The same call appears in the caller's and the callee's operator views with the small differences real exports show (section 6).

## 4. The plant catalogue

Each plant states what is planted, the traces it leaves, the decoys, the analysis that checks it and what the key records. Parameters are per case (section 9) with defaults.

| Plant | Planted | Traces in the files | Decoys | Checked by | Key records |
|---|---|---|---|---|---|
| **P1 contact structure** | Communities (dense groups), chains (A to B to C), hubs, bridges between communities, one-way and reciprocal contacts | Calls and SMS with realistic rhythm (day and night, weekday and weekend), durations, missed calls, short-code traffic | Services contacted by many people; dormant ties | A1, A4, A3, A12 | Edges with counts, first and last, per-direction counts; community membership; hubs |
| **P2 common contacts** | Pairs (X, Y) sharing a known set of contacts *(v0.2: persons that no subject is linked to in the background graph, reached by planted calls only, so no third subject can turn one into a hub by chance; calls because not every layout can express a message)* | Both subjects call the same third parties, at different times and in different operators' exports | A "common" contact that is only a service number or a hub above the threshold | A2 | Expected shared set per pair with each side's count, and the excluded ones with the reason |
| **P3 co-location** | Pairs of subjects *(v0.3: on the same operator, because cell identities are per operator)* at the same cell within W minutes, on several days, at a meeting cell neither uses in the background | Events at the same `cell_key` for both subscriptions within W minutes (one to three consecutive pairs per window) | Same cell three hours apart; same cell W + 1 minute apart; the same moment at a different cell. Chance co-location of habitual cells is what the baseline measures | A5 | Windows per pair (start, end, cell, pairs), the chance baseline and lift, the planted windows and the decoy events |
| **P4 possible meeting** | *(v0.4)* On each P3 pair, the first two co-location windows are preceded by a direct call between the pair 10 to 40 minutes earlier; the pair has no other direct contact in the background | The call appears in both subjects' exports, at two different cells (a contact is never itself a co-location) | A window with no contact; a contact G + 120 minutes before a window (too early); a contact on a day with no co-location | A7 | Planted meetings with their contacts, and the three kinds of negative |
| **P5 device and SIM change** | *(v0.5)* On a known local date a subject's handset, SIM or both change (the number stays); two subjects on one operator share a handset for the whole period (a dual-SIM phone). Only subjects whose layout shows the identifiers (C1, C3, C4 show IMSI and IMEI; C2 only the IMEI) | The IMEI under a number changes; the IMSI under it changes; the same IMEI sits under two numbers at once | The dual-SIM phone: one device under two numbers at the same time, nothing swapped | A8 | Each change with its date, old and new identity, and the shared device |
| **P6 successor number** | *(v0.5)* An old number goes silent on a known day; a new number on the same operator starts one to three days later with the old one's contacts (their edges are copied) and cell footprint (same home and work cells); in the first pair the new number also reuses the old handset | The old number's events stop, the new one's begin; the contact sets and cells match; the IMEI continues (first pair) | A number that goes silent with no successor; a number that starts late with contacts of its own; the two pairs against each other (similar timing, different contacts) | A9 | Successor pairs with the planted features, and the decoys that must score below the threshold |
| **P7 address-translation collisions** | *(v0.4)* On a public address of its own, three subscriptions of one operator hold ports: two overlap in time and port (a collision), one shares the time but not the port, one repeats the port later, two hold one port back to back one second apart; seven probes (address, port, instant) each state the intended answer | A *public-address query export*: every session on that address in the day, in the operator's internet-record layout (with the query preamble where the layout has one) | Same time, other port; same port, later; one second apart; a port nobody held | A13 | Per probe: the intended and the computed answer and holders |
| **P8 movement and rhythm** | Anchor cells (home, work) and trips; a change point in behaviour on a known date | Cell sequences, night and day dwell, a step change in daily activity | Commuting noise; a one-day outlier | A10, A11 | Anchors, trips, change dates |
| **P9 convoys and mentions** (later) | Plates seen together on camera sequences; numbers mentioned in documents and speech | Needs the media generators | Look-alike plates; misread digits | A14, A15 | Mention index, convoys |

Version 1 builds P1 to P4 and P7 with the defects of section 6. P5, P6 and P8 follow in v1.1; P9 waits for the media phase.

## 5. What a rendered case contains

```
case-<seed>/
  manifest.json                 case id, seed, generator and specification versions, parameters, SHA-256 of every file
  files/                        one file per projection, in the real container (xlsx, SpreadsheetML, CSV, UTF-16 text)
  key/key.json                  the answer key (section 7)
  key/world.json                the canonical truth: people, subscriptions, devices, cells, events
  README.txt                    "synthetic; every value is from a reserved range"
```

Files carry the provider's header names, header position, preamble, time formats, enumerations, typos and padding exactly as the layout entry describes. The generator reuses the container writers and value renderers of `tools/real_data/make_golden.py` and the reserved-range generators of `tools/real_data/reserved.py`.

## 6. Rendering and mess

**Renderer.** For each layout entry the generator turns a canonical event into a row: it is the inverse of the registry's reference interpreter. Direction and event-type labels use the entry's mapped labels; where an entry's meaning is unconfirmed (for example C6's numeric direction codes) the generator uses the entry's `proposed` meaning and the key says so, so the case can never be mistaken for evidence about the real meaning.

**Round trip (the generator's own test).** Render, match the header with the registry, interpret every row, and compare with the canonical events. They must be equal except for the documented losses of each layout (minute-precision times, truncated durations) and the injected defects.

**View differences between the two sides of one event.** Start times differ by a few seconds, durations by rounding, the counterparty form by operator (`923...`, `03...`, `3...`), and each operator sees only its own subscriber's cell. The key records the canonical event and both views.

*(v0.4)* **What a layout can only say vaguely, it says vaguely.** C6's direction codes have no confirmed meaning, so its rows carry the code whose *proposed* meaning is the planted one and read back as `unknown` (the key marks the assumption); C8 has one activity code, so every row reads as `other`; C7 has no call-type column, so its rows read as `other` too. The row keeps `true_direction` and `true_event_type` for the audit, but the oracles use what the file says.

*(v0.2)* **What a layout cannot express is left out, and counted.** C2's labels cover calls only and it has no short-code or sender-id form, so its export has no SMS and no service traffic; the key lists the number of such events per subject (`events_not_representable`) and marks a decoy that is not visible in both files (`visible_in_both_files`).

**Injected defects** *(v0.4)*, each with an expected outcome written from the registry's documented rules (not from running the interpreter) and checked by the audit; never on a planted row; three per class and export by default (`defects_per_class`, 0 switches them off). Call-record exports: blank row (skipped), start time not a time (quarantined `time_unparsed`), start time missing (`time_missing`), a slash date in the other order where the day or month cannot exist (`date_order_conflict`), a direction label the layout does not map (accepted, direction `unknown`, flag `unmapped_label:direction`; where the counterparty column depends on the direction, no counterparty either), counterparty `-1` (accepted, flag `b_party_sentinel`), counterparty equal to the subscriber in C6 (flag `counterparty_equals_subject`), a repeated row (kept with its lineage and flagged `duplicate_row`, as migration 012 requires; the analyses count it once). Internet-record exports: blank row, time not a time, time missing, server address 999.1.1.1 (flag `ip_invalid:server_ip`), server port 70000 (flag `server_port_out_of_range`), a repeated row (flagged, counted once). A row that cannot be an event is left out of the rows the reference answers use. Not built: truncated rows (the registry has no rule for them yet).

## 7. The answer key

`key/key.json` is written from the canonical truth and from the reference oracles. Rows are identified by natural keys, never by file row numbers (the loader assigns its own ids): `(subject msisdn, start_utc, counterparty normalised, event_type, source layout)`.

```json
{
  "case_id": "...", "seed": 101, "generator_version": "0.1", "spec_version": "0.1",
  "clock": {"local_zone": "Asia/Karachi", "key_time": "UTC"},
  "plants": [
    {"id": "P3-007", "type": "colocation", "members": ["923000000012", "923000000047"],
     "windows": [{"cell_key": "...", "start_utc": "...", "end_utc": "...", "events": [["natural key"], ["natural key"]]}],
     "decoys": [{"kind": "same_cell_far_apart_in_time", "events": []}]}
  ],
  "oracle": {
    "A2": [{"x": "...", "y": "...", "shared": [{"number": "...", "x_count": 4, "y_count": 2}], "excluded": [{"number": "...", "reason": "service"}]}],
    "A5": [{"x": "...", "y": "...", "window_minutes": 15, "windows": [], "lift": {"formula": "see 10", "value": 0.0}}],
    "A13": [{"public_ip": "...", "port": 0, "instant": "...", "result": "unique|ambiguous", "candidates": []}]
  },
  "expected_quarantine": [{"file": "...", "natural_key": [], "reason": "time_unparsed"}],
  "expected_merge": [{"file": "...", "natural_key": [], "reason": "duplicate"}],
  "layout_assumptions": [{"layout_id": "geofence-c6-telenor", "item": "direction-codes", "used": "proposed meaning"}]
}
```

## 8. Reserved ranges and their limits

| Value | Source | Capacity and note |
|---|---|---|
| Mobile numbers | `923000000000` to `923000099999` (`reserved.synthetic_msisdn`) | 100,000 numbers; a case needs hundreds to a few thousand; services use short codes and sender IDs that cannot be real numbers |
| IMSI | PLMN `00101` + 10 digits | Test network code |
| IMEI | TAC prefix `00` + serial + Luhn digit | Test TAC |
| IPv4 | documentation blocks (`192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`) | Only 762 addresses; enough for collisions, which need few public addresses and many subscribers |
| IPv6 | `2001:db8::/32` | Large; used when a case needs many distinct addresses |
| CNIC-shaped | `00000-xxxxxxx-x` | Only if a layout needs it |
| Coordinates | A fictional grid anchored in open ocean (proposal: near 0 degrees latitude and 30 degrees west), cells 300 m to 5 km apart | Describes no real place, never trips the scanner's Pakistan-coordinate rule, keeps distances realistic for radius and speed checks |
| Host names, URLs, agents | `*.example.test`, `/path/N`, `ExampleAgent/1.N` | As in the goldens |

Every generated file and the key pass `scan_for_real_data.py` as a test.

## 9. Parameters and scale

| Parameter | Default | Note |
|---|---|---|
| `people` | 120 (subjects 6, associates 24, bystanders 80, services 10) | Range 40 to 2,000 |
| `period_days` | 90 | |
| `events_per_person_per_day` | 8 calls and SMS, 20 sessions | A case of about 40,000 to 150,000 rows |
| `operators` | 4 (the four whose layouts exist) | Each subscription belongs to one |
| `window_minutes` (A5) | 15 | A parameter of the oracle and of the key |
| `subjects` | *(v0.5)* 10 (was 6) | Roles need pairs of subjects on one operator; with 10 subjects (operators 0 to 3, then 0, 1, 2, 3, 0, 1) there are four pairs and two singles, enough for two co-location pairs (P3), two successor pairs (P6), a silent number and a late starter. Bystanders go from 90 to 86 (120 people in all) |
| `p3_pairs`, `windows_per_plant`, `window_minutes` | 2 plants *(v0.5: was 5, but 6 subjects only ever gave two pairs on one operator)*, 4 windows each, W = 15 | *(v0.3)* each plant has three decoys |
| `p6_pairs`, `p5` | 2 successor pairs, 1 (device and SIM changes on) | Roles are allocated once (`allocate_roles`): P3 pairs first, then P6 pairs, then the silent number and the late starter |
| `hub_threshold` (A2) | *(v0.2)* distinct **subjects whose exports are loaded** in contact with a number: more than `subjects - 2` makes it a hub (4 for 6 subjects); 30 for a case with a tower dump | A product can only count subscriptions that appear in the loaded data, so the rule is stated over the loaded exports; same value in oracle and product |
| `defect_rates` | each class 3 to 10 rows per file | |
| `plant_counts` | 5 of each plant per case, each with 2 decoys | |

Seeds: development 1 to 99; judgement seeds 100 to 199 (generated before the code they judge is measured); sealed seeds 900 and up (generated once, hashes recorded, files not opened until a gate). The target set is **20 cases** (roadmap section 8.3).

## 10. The first two oracles, defined

These definitions are fixed before any code and are the contract between generator, oracle and product. They are written here so that the SQL implementation, the Python oracle and the key cannot drift apart.

**A2 common contacts.** For a subject `S`, `contacts(S)` is the set of normalised counterparties of `S`'s calls and SMS (both directions) with class `person_like` or `international`; counterparties of class `short_code`, `sender_id`, `service_label` and `unknown` are excluded, and so is any number in contact with more than `hub_threshold` distinct subjects *(v0.2: counted over the loaded exports)*. `common(X, Y) = contacts(X) ∩ contacts(Y)`, minus `X` and `Y` themselves. Output per shared number: the count of events with `X` and with `Y`, first and last time on each side. Excluded numbers are reported with the reason.

**A5 co-location.** *(v0.3: the baseline is stated on event pairs; the v0.1 text compared windows with an expected number of windows, which had no independent definition, and used hour-of-week buckets, which are too sparse for one subscription.)* Two subscriptions on the same operator are co-located at a time when each has an event with the same `cell_key` and the two event times differ by at most `W` (a difference of exactly `W` counts). Co-located event pairs on one cell are merged into windows: a pair joins the open window when its earlier time is within `W` of the window's latest time. Output per window: start, end, `cell_key`, number of pairs. **Chance baseline:** take every pair of events, one of each subscription, within `W` of each other whatever their cells. For such a pair, the time-of-week class is (weekday or weekend, hour of the day) of the earlier event; each subscription's habit in that class is the share of its own events in each cell (its overall shares when it has no event in the class); the probability that both are at the same cell is the sum over cells of the product of the two shares. `expected = ` the sum of these probabilities over all pairs; `lift = observed pairs / expected`, undefined ("too few events") when `expected` is below 0.5. Two subscriptions on different operators never share a cell key, so they have no A5 answer. The formula is **pre-registered here**; a change needs a new specification version. Every co-location result carries "proximity is not association".

**A6 tower-dump population.** *(v0.4)* A tower dump is the projection of every event in a window in which a party of one operator was at one of a stated set of cells, rendered as that party's row. Output: per number the events and the cells at which it appears, and the numbers seen at two or more of the dump's cells. A layout without a cell column (C5) lists the numbers only and says that the cells are the request, not a column. The result is authority-gated in the product (roadmap section 9); the generator only supplies the truth.

**A7 possible meetings.** *(v0.4)* A co-location window (A5) of two subjects is a candidate meeting when they had a direct call or SMS with each other no more than `G` minutes before the window starts (a contact exactly `G` before, or at the start, counts; one after the start does not). Output per candidate: the window, the contacts in the gap, the gap to the latest of them and a documented score, `score = 0.5 * min(pairs, 3) / 3 + 0.5 * (1 - gap / G)`. A candidate, not a finding: proximity and contact are not proof of a meeting. `G` defaults to 60.

**A8 device and SIM history.** *(v0.5)* Per number, the sequence of (IMSI, 14-digit IMEI) as runs of unchanged identity, and each change between runs (`imei`, `imsi` or `both`) with the last time the old identity and the first time the new one was seen. Across numbers, a device seen under two or more numbers is `concurrent` when their periods of use overlap (a dual-SIM phone, or one phone shared) and `sequential` when one stops before the other starts (a handset passed on). Layouts without an IMSI column contribute the IMEI only.

**A9 successor candidates.** *(v0.5)* A number is silent when its last event is at least 7 days before the end of the case. A candidate successor is another number whose first event falls between 2 days before and 14 days after the silent number's last event and that was not already in use in the first 7 days of the case. Features: the cosine of the two contact-count vectors (person-like counterparties, the silent number's last 30 days against the candidate's first 30 days, the two numbers themselves left out), the Jaccard index of the cells each used at least twice in those windows, and whether they share a 14-digit IMEI at any time. `score = 0.6 * cosine + 0.2 * jaccard + 0.2 * shared device`; a candidate is listed when `score >= 0.35`. Behavioural similarity, not identity.

**A13 address attribution.** *(v0.4)* A session holds its public address and port block from its NAT start (else its start) to its NAT end (else its end), both included. Given an address, a port and an instant, the holders are the distinct subscriptions whose held block contains the port at that instant: one is `unique`, two or more `ambiguous` (never resolved by guessing), none `none`. Plant P7 runs on zong, mobilink and telenor only: ufone's layout (I3) writes a log time and no end, so it cannot support interval attribution and the key does not ask it to.

Later oracles (A4 chaining, A3 paths, A7 meetings, A8, A9, A13) get their definitions added as each plant is built, before the code.

## 11. How the generator itself is verified

1. **Determinism:** the same seed twice gives identical files and identical manifest hashes.
2. **Reserved values only:** the scanner finds nothing in any generated file; a deliberate breakage (a number outside the block) is caught.
3. **Round trip** through the layout registry, per layout (section 6).
4. **Plant audit:** a separate script reads only the rendered files and the key and confirms that every plant is present in the data (the shared contacts exist, the co-locations are within the window) and that every decoy is not a plant (it fails the predicate). The audit does not import the generator.
5. **Oracle parity:** the Python oracles agree with a brute-force reference on small hand-checkable cases kept in the repository.
6. **Metamorphic checks:** relabelling numbers, shifting all times by a whole offset with the zone declared, or adding below-threshold noise must not change the oracle output.
7. **Deliberate breakages:** removing a planted event, moving one outside the window, or turning a service number into a person each changes the oracle output or fails the audit.
8. **Statistics under control:** the chance co-location rate for bystanders is measured and bounded, so a "co-location" among bystanders is rare and the lift baseline is meaningful.

## 12. Build order and acceptance

| Step | Deliverable | Acceptance |
|---|---|---|
| 1 | **This specification**, reviewed by the owner | Approved or amended |
| 2 | Generator skeleton: world model, P1 and P2 (plants and decoys), rendering for the target-export layouts (C1 to C4), key, manifest, determinism and scanner tests | A seeded case renders; round trip equals the truth; the plant audit passes. **Built 2026-10-12** (section 14) |
| 3 | A2 and A5 oracles in plain Python with hand-checkable cases; P3 | Oracle parity; the deliberate breakages change the output. **Built 2026-10-13** (section 14) |
| 4 | Tower-dump and IPDR rendering (C5 to C8, I1 to I4, A1) and P7, P4; the defects of section 6 | Round trip for all 13 layouts; expected quarantine matches what the registry produces |
| 5 | Load a case into the **throwaway** database (migrations 012 to 015 applied there only) and check that counts and plants survive; first brute-force SQL check of A2 and A5 against the oracle | Parity on the throwaway database. This is not the live database. **Built 2026-10-13** (section 14) |
| 6 | P5, P6, P8; 20 cases; judgement and sealed seeds | Gate set after the baseline |

## 13. Questions for the owner

1. **Scale defaults** in section 9 (120 people, 90 days, about 100,000 rows). Smaller is faster to review; larger finds performance problems earlier.
2. **Coordinates:** an open-ocean fictional grid (proposed) or a fictional city layout with invented street-free coordinates. The first never resembles a real place; the second reads more naturally on a map in the demo.
3. **Who holds the sealed seeds** and when they may be opened.
4. **Where cases live:** generator plus one tiny case in the repository, everything else regenerated locally and ignored by git (proposed), or a private store.
5. **Time zone:** every case in `Asia/Karachi` with the zone declared (proposed), or one case in each of two zones to exercise the zone handling.

6. *(v0.3, answered by the build in v0.5)* **More subjects?** With 6 subjects only two pairs share an operator, so a default case has two co-location plants, not five. Eight subjects (two per operator) give four; the cost is two fewer bystanders. Default stays 6 unless you say otherwise.

Out of scope until separately approved: any use of the real samples to tune the generator's statistics (the generator is built from the layout entries and the documented formulas, not from real data), media generators, and loading a generated case into the live database.

## 14. What exists after step 2 (2026-10-12)

`tools/case_generator/` (Python 3.10, standard library only):

| File | Role |
|---|---|
| `world.py` | Parameters, the canonical world (people, numbers, devices, cells), plant P1 (communities, chains, bridges, a hub) and plant P2 (shared contacts with a service decoy and a hub decoy), the event stream |
| `render.py` | Inverse of the registry interpreter for the target exports C1 to C4: labels, number forms, times, durations, cells, coordinates, preamble and blank-header padding from the layout entry |
| `oracle.py` | Reference answers A1 (contact profile) and A2 (common contacts), section 10 |
| `generate.py` | Writes `case-<seed>/` with `files/`, `key/key.json`, `key/world.json`, `manifest.json`, `README.txt` |
| `audit.py` | Independent audit: reads only the files and the key (with the vault tools' reader and the registry), recomputes A2, checks plants, decoys, hashes and reserved values |
| `test_case_generator.py` | 29 tests (18 at step 2, 11 at step 3): same seed same bytes, every export read back equal to the truth, audit passes (small and default size), scanner finds nothing, plants and decoys as keyed, hand-checked oracle, four deliberate breakages of a case noticed |

```bash
python -X utf8 tools/case_generator/generate.py --seed 1 --out CASES_DIR
python -X utf8 tools/case_generator/audit.py CASES_DIR/case-1
python -X utf8 tools/case_generator/test_case_generator.py
```

The default case (120 people, 6 subjects, 90 days) is written in about two seconds: six exports of a few hundred rows each, a key and a world file of a few megabytes (not for the repository). Not yet built: P3 to P9, the tower-dump and IPDR layouts, the injected defects, loading into the throwaway database (steps 3 to 6).

**Step 3 (2026-10-13).** Plant P3 and the A5 oracle are built.

* `world.py` plants co-location for pairs of subjects on one operator: per pair a meeting cell neither uses in the background, four windows of one to three consecutive pairs, and three decoys (same cell three hours apart, same cell W + 1 minutes apart, same moment at another cell); events carry a planted cell (`Event.cells`).
* `oracle.py` `a5_colocation` implements section 10 exactly; `generate.py` writes `oracle.A5` and the P3 plants (windows and decoys by natural key) into the key.
* `audit.py` recomputes A5 with a separate brute-force implementation (every pair of events, no shortcuts) and checks that the key's windows, pair counts and chance baseline equal it, that each planted window is a co-location window in the files, and that no decoy event takes part in a co-located pair.
* Tests: hand-checked windows, the exact boundary (W co-located, W + 1 second not), a chance baseline worked out by hand (lift 1.0), "too few events", a week-shift and cell-rename check, a planted window removed from a file is noticed.
* The default case (6 subjects) has two co-location plants; a background co-location between subjects who share a home or work cell appears in the answer too, with its lift, and is not a plant.

**Step 4, part a (2026-10-13): tower dumps.** `render.py` now renders any list of (event, person) views, so a tower dump is the view list of every party of an operator at a set of cells in a window (`dump_views`), in that operator's dump layout (ufone C7, zong C8, mobilink C5, telenor C6). `generate.py` plans one dump per operator that has a subject (the planted meeting cell first, then subjects' work and home cells; the window starts on the first planted co-location day of that operator), writes `dumps[]` and `oracle.A6` into the key, and `audit.py` reads each dump back, checks its window and cells, and recomputes the population. 37 tests. The default case has four dumps of 77 to 152 rows each; the people in them are mostly not subjects.

**Step 4, part b (2026-10-13): data sessions and the internet-record layouts.** `world.py` now generates data sessions for the subjects (a private address, a public address and port block that nobody else holds at that time, a server, volumes, an application): a documentation-block pool of three public addresses per operator, blocks of 64 ports for the operator whose layout writes a range (ufone, I3) and one port for the others, allocated so that the background never collides. `render_sessions.py` renders them in the internet-record layout of each operator (ufone I3, zong I4, mobilink I1, telenor I2: CSV with BOM, SpreadsheetML, UTF-16 text with the query preamble, CSV) using each layout's most precise time format, and one application-usage export (A1) for the first subjects. `audit.py` reads every container, checks each row is accepted and from reserved addresses, and that no two sessions hold one public address and port at once. 44 tests. Sessions exist for subjects only (and, next, for the subscriptions of plant P7).

**Step 4, part c (2026-10-13): plant P7 and A13.** `world.plant_p7` builds, per plant, six sessions of three bystanders on a dedicated documentation address and seven probes with intended answers; `generate.py` renders the public-address query export in the operator's layout (zong I4 with its query preamble, mobilink I1, telenor I2), writes `ip_queries[]`, the plants with their probes and `oracle.A13`; `audit.py` recomputes every probe from the file and checks it against both the key and the intent (21 probes in a default case). 51 tests.

**Step 4, parts d and e (2026-10-13): possible meetings and injected defects.** Plant P4 is built on P3 (`world.py`: contacts at differing planted cells, the pair's background contacts removed, `meeting_windows` and `contact_gap_minutes`); `oracle.a7_meetings` implements A7; the key gets `oracle.A7`, the P4 plants (planted meetings and three kinds of negative) and `defects` per export with `expected_quarantine` and `expected_merge`. `defects.py` damages background rows of the call-record and internet-record exports; `audit.py` checks every damaged row against its expected outcome, that all other rows are accepted, and merges exact repeats before recomputing the answers. 71 tests (about three minutes on the laptop), and each of the 17 deliberate breakages of the step 4 code is caught by a test. The default case now holds 21 exports: 6 target exports (C1 to C4), 4 tower dumps, 6 internet-record exports, 2 application-usage exports and 3 public-address queries, plus the key, the world and the manifest (about 7.7 MB, mostly the world file).

**Step 5 (2026-10-13): the harness loader.** `load_case.py CASE_DIR` starts a disposable container from the local image (never pulls, no published port, random password, removed with its volume), applies migrations 001 to 015 and the runtime-role grants as a fresh database sees them, registers the 13 layouts, reads every export through the registry and loads the accepted rows into `forensic.comm_event`, `forensic.data_session` and `forensic.app_usage` (each file first becomes an `evidence_items` row, as the composite foreign keys require). Then it answers with **SQL** and compares with the key: reconciliation per export (rows read = loaded + quarantined + skipped; repeats flagged; the quarantine and repeat counts equal the key's), A1 for every subject, A2 for every pair, A5 windows and pairs for every same-operator pair, A6 for every dump and A13 for every probe: 71 checks on a default case, about 75 seconds in all. Two points. First, migration 012 says repeated rows are kept with their lineage and never dropped, so the loader flags later copies `duplicate_row` and the SQL answers exclude flagged rows; the key's `expected_merge` therefore means "flag and count once". Second, the chance baseline (lift) and A7 are not yet written in SQL; the windows and pair counts are. `test_load_case.py` runs the same on a smaller case (9 tests, about 80 seconds): all checks, the typed tables hold exactly the accepted rows, quarantined and blank rows are not loaded, repeats are kept and flagged, lineage and layout foreign keys, numbers inside the reserved block, row-level security hides the case from another tenant, and a deliberately damaged table is noticed. The loader is **test-only**: the product's worker does not write the typed tables until Phase 1 (Ingest v2).

**Step 6, parts a and b (2026-10-13): device and SIM changes, successor numbers.** `world.allocate_roles` gives each subject at most one part (pairs are ordered by subject number). Identities now follow time (`Person.segments`, `identity_at`), and every renderer asks for the identity at the event or session time. Plant P5 (`plant_p5`) puts the changes and the dual-SIM decoy on subjects that show the identifiers; plant P6 rewires the new number's contacts and cells and cuts both numbers' events and sessions to their planted days (planted events respect those days too; P2 pairs exclude P6 subjects). `oracle.a8_device_history` and `oracle.a9_successors` implement the two definitions above; the key gets the plants, `oracle.A8` and `oracle.A9`; `audit.py` recomputes both from the files (A8 from the identities, A9 from contacts, cells and handsets) and checks every planted change, shared device, successor, silent number and late starter. The default case is now 10 subjects (32 files). 87 tests.

