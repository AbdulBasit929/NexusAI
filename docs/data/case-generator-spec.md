# Case generator with planted truth: specification v0.1 (roadmap Phase 1, section 7.5 and 8.3)

Status: **draft for the owner's review, 2026-10-12. No code exists yet.** Nothing here reads a real file, touches a database or changes the product. When the owner approves this document (or sends changes), the generator is built in the order of section 12.

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
| **P2 common contacts** | Pairs (X, Y) sharing a known set of contacts | Both subjects call or text the same third parties, at different times and in different operators' exports | A "common" contact that is only a service number or a hub above the threshold | A2 | Expected shared set per pair with each side's count, and the excluded ones with the reason |
| **P3 co-location** | Pairs at the same cell within a stated window, on known days | Events at the same `cell_key` for both subscriptions within W minutes | Same cell far apart in time; neighbouring sector; a cell that is busy for everyone (chance) | A5 | Windows per pair (start, end, cell, count), the chance baseline per cell and hour, expected lift |
| **P4 possible meeting** | P3 windows preceded by a call or SMS between the pair within a stated gap; some meetings without any call | As P3 plus the contact event | Co-location with no contact; contact with no co-location | A7 | Meetings with the features that make them (gap, window, repeats) and the negatives |
| **P5 device and SIM change** | A person changes handset, or SIM, or both, on a known date; a handset passes between two people | The IMEI under a number changes; the IMSI under an IMEI changes; both overlap or not | Dual-SIM phones (two IMSIs under one IMEI at once); a lost-and-found handset | A8 | Swap events with date, old and new identifiers, shared devices |
| **P6 successor number** | One number goes silent and a new number appears with a similar contact set and cell footprint | Last event of the old number, first of the new, overlapping contacts | A silent number with no successor; a new number with unrelated contacts | A9 | Successor pairs with the planted similarity, and the negatives |
| **P7 address-translation collisions** | Several subscriptions share one public IP; port ranges overlap in time for some pairs and not for others | IPDR rows with public IP, port, NAT start and end, subscription | Same IP, disjoint times; same time, disjoint ports | A13 | Per (public IP, port, instant): the unique subscription, or "ambiguous" with the candidate set |
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

**Injected defects**, each with an expected outcome in the key: a blank row; an unparsable time; a missing time; an unknown label; a service number as counterparty; a duplicate row (same content twice); a row truncated mid-way; a counterparty equal to the subject; a slash date that contradicts the layout's settled order (expected: quarantined as `date_order_conflict`). Rates are parameters; the default injects each class a handful of times so counts stay checkable.

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
| `hub_threshold` (A2) | contacted by more than 30 distinct subscriptions | Same value in oracle and product |
| `defect_rates` | each class 3 to 10 rows per file | |
| `plant_counts` | 5 of each plant per case, each with 2 decoys | |

Seeds: development 1 to 99; judgement seeds 100 to 199 (generated before the code they judge is measured); sealed seeds 900 and up (generated once, hashes recorded, files not opened until a gate). The target set is **20 cases** (roadmap section 8.3).

## 10. The first two oracles, defined

These definitions are fixed before any code and are the contract between generator, oracle and product. They are written here so that the SQL implementation, the Python oracle and the key cannot drift apart.

**A2 common contacts.** For a subject `S`, `contacts(S)` is the set of normalised counterparties of `S`'s calls and SMS (both directions) with class `person_like` or `international`; counterparties of class `short_code`, `sender_id`, `service_label` and `unknown` are excluded, and so is any number in contact with more than `hub_threshold` distinct subscriptions in the case. `common(X, Y) = contacts(X) ∩ contacts(Y)`, minus `X` and `Y` themselves. Output per shared number: the count of events with `X` and with `Y`, first and last time on each side. Excluded numbers are reported with the reason.

**A5 co-location.** Two subscriptions are co-located at time `t` when each has an event with the same `cell_key` and the two event times differ by at most `W`. Co-located events are merged into windows (consecutive pairs within `W` of the previous one). Output per window: start, end, `cell_key`, number of event pairs. The chance baseline for a pair is the expected number of windows if the two subscriptions chose cells independently: for each hour-of-week bucket, the product of each subscription's share of its events in each cell, scaled by the number of event pairs falling in that bucket. `lift = observed windows / expected windows` (undefined when the expected value is below 0.5; the output then says "too few events"). The formula is **pre-registered here**; a change needs a new specification version. Every co-location result carries "proximity is not association".

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
| 2 | Generator skeleton: world model, P1 and P2 (plants and decoys), rendering for the target-export layouts (C1 to C4), key, manifest, determinism and scanner tests | A seeded case renders; round trip equals the truth; the plant audit passes |
| 3 | A2 and A5 oracles in plain Python with hand-checkable cases; P3 | Oracle parity; the deliberate breakages change the output |
| 4 | Tower-dump and IPDR rendering (C5 to C8, I1 to I4, A1) and P7, P4; the defects of section 6 | Round trip for all 13 layouts; expected quarantine matches what the registry produces |
| 5 | Load a case into the **throwaway** database (migrations 012 to 015 applied there only) and check that counts and plants survive; first brute-force SQL check of A2 and A5 against the oracle | Parity on the throwaway database. This is not the live database |
| 6 | P5, P6, P8; 20 cases; judgement and sealed seeds | Gate set after the baseline |

## 13. Questions for the owner

1. **Scale defaults** in section 9 (120 people, 90 days, about 100,000 rows). Smaller is faster to review; larger finds performance problems earlier.
2. **Coordinates:** an open-ocean fictional grid (proposed) or a fictional city layout with invented street-free coordinates. The first never resembles a real place; the second reads more naturally on a map in the demo.
3. **Who holds the sealed seeds** and when they may be opened.
4. **Where cases live:** generator plus one tiny case in the repository, everything else regenerated locally and ignored by git (proposed), or a private store.
5. **Time zone:** every case in `Asia/Karachi` with the zone declared (proposed), or one case in each of two zones to exercise the zone handling.

Out of scope until separately approved: any use of the real samples to tune the generator's statistics (the generator is built from the layout entries and the documented formulas, not from real data), media generators, and loading a generated case into the live database.
