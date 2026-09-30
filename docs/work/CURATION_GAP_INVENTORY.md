# Curation gap inventory — every family, every column

Generated 2026-09-25 from the live database, across ALL collections.
`rows` = records carrying the key. `cols` = collections it appears in.

A column with no curated `source_names` entry is never issued in the enum, so the
generator cannot name it and **no question about it can be answered.**

## access_log — 6 columns, 0 NOT CURATED

Fully curated.

## anpr — 23 columns, 15 NOT CURATED

| column | rows | cols |
|---|---:|---:|
| `detection_confidence` | 307 | 1 |
| `ocr_confidence` | 307 | 1 |
| `plate` | 307 | 1 |
| `observation_id` | 307 | 1 |
| `source_locator` | 307 | 1 |
| `observation_state` | 307 | 1 |
| `OCR Confidence` | 6 | 1 |
| `Plate Script` | 6 | 1 |
| `Registration No.` | 6 | 1 |
| `Province` | 6 | 1 |
| `Camera Code` | 6 | 1 |
| `Camera Location` | 6 | 1 |
| `Camera Timezone` | 6 | 1 |
| `Captured At` | 6 | 1 |
| `Crop SHA256` | 6 | 1 |

## cdr — 32 columns, 15 NOT CURATED

| column | rows | cols |
|---|---:|---:|
| `location` | 13,647 | 3 |
| `lat` | 13,643 | 3 |
| `Lac_Id` | 13,643 | 3 |
| `Site_Id` | 13,643 | 3 |
| `longitude` | 13,643 | 3 |
| `direction` | 4 | 1 |
| `end_time` | 2 | 1 |
| `cell_id` | 2 | 1 |
| `call_time` | 2 | 1 |
| `call_end_time` | 2 | 1 |
| `b_number` | 2 | 1 |
| `a_number` | 2 | 1 |
| `site_id` | 2 | 1 |
| `source_number` | 2 | 1 |
| `target_number` | 2 | 1 |

## generic — 1 columns, 1 NOT CURATED

| column | rows | cols |
|---|---:|---:|
| `Case notes for records-demo` | 9 | 1 |

## ipdr — 19 columns, 11 NOT CURATED

| column | rows | cols |
|---|---:|---:|
| `location` | 4 | 1 |
| `cell_id` | 4 | 1 |
| `bytes_down` | 4 | 1 |
| `bytes_up` | 4 | 1 |
| `url` | 4 | 1 |
| `msisdn` | 4 | 1 |
| `source_port` | 3 | 1 |
| `session_end` | 3 | 1 |
| `source_timezone` | 3 | 1 |
| `session_start` | 3 | 1 |
| `destination_port` | 3 | 1 |

## subscriber — 35 columns, 3 NOT CURATED

| column | rows | cols |
|---|---:|---:|
| `cnic_last4` | 5 | 1 |
| `account_type` | 5 | 1 |
| `location` | 1 | 1 |

## tower_location — 23 columns, 15 NOT CURATED

| column | rows | cols |
|---|---:|---:|
| `cell_site_id` | 10 | 2 |
| `city` | 10 | 2 |
| `coverage_type` | 10 | 2 |
| `latitude` | 10 | 2 |
| `location` | 10 | 2 |
| `longitude` | 10 | 2 |
| `operator` | 10 | 2 |
| `site_name` | 10 | 2 |
| `TAC` | 5 | 1 |
| `Uncertainty Radius M` | 5 | 1 |
| `Updated At` | 5 | 1 |
| `Azimuth` | 5 | 1 |
| `Beam Width` | 5 | 1 |
| `Coordinate Datum` | 5 | 1 |
| `Sector ID` | 5 | 1 |

## transaction — 9 columns, 0 NOT CURATED

Fully curated.

**TOTAL: 148 columns, 88 curated (59%), 60 NOT CURATED.**
