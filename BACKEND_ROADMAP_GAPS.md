# Backend Roadmap & UI/UX Feature Gaps Tracker

This document records high-value UI/UX capabilities, advanced forensic visualizations, and interaction models identified during frontend design that require future backend endpoint or schema extensions.

## Tracking Schema
For each gap:
1. **Target Feature & Route**: Which UI page, modal, or component would display this?
2. **Current Workaround**: How the UI gracefully degrades with the existing backend today.
3. **Proposed Backend API Contract**: Suggested endpoint URL, query params, and JSON response shape.
4. **Analyst Value**: Why this capability empowers intelligence work (e.g., speed, clarity, cross-case linkage).

---

## Logged Gaps

### 1. Multi-Case Entity Co-Occurrence Matrix
* **Target Feature & Route**: `/cases/:id/connections` and `/cases/:id/overview`
* **Current Workaround**: Connections graph currently computes links between subscribers within the single active case's local evidence.
* **Proposed Backend API Contract**:
  - `GET /api/v1/analytics/co-occurrence?entity={msisdn_or_imei}`
  - Response: `{ entity: string, occurrences: [ { caseId: string, firstSeen: string, lastSeen: string, eventCount: number } ] }`
* **Analyst Value**: Instantly detects if a suspect phone number or vehicle plate has appeared in prior or concurrent investigations without manual cross-case querying.

### 2. Micro-Timeline Anomaly Density Heatmap Stream
* **Target Feature & Route**: `/cases/:id/timeline` (Global scrubber header)
* **Current Workaround**: Aggregated into daily/weekly buckets via `activity.days`.
* **Proposed Backend API Contract**:
  - `GET /api/v1/cases/:id/timeline/density?resolution=hour&start={iso}&end={iso}`
  - Response: `{ buckets: [ { timestamp: string, count: number, anomalyScore: number } ] }`
* **Analyst Value**: Allows sub-hour burst detection (e.g. 50 calls in 10 minutes) directly visible in the timeline mini-scrubber.

### 3. Authoritative Case Metadata & Cross-Modality Event Horizon
* **Target Feature & Route**: `/cases/:id/*` (Case Context Header & Dossier Overview)
* **Current Workaround**: UI derives record counts from `GET /collections/status` and event dates from calling the `activity_by_day` template. Case title defaults to raw identifier (e.g., `nexusai-forensic-demo`).
* **Proposed Backend API Contract**:
  - `GET /api/v1/collections/{id}/metadata` (or include in `/collections/status` response)
  - Proposed schema:
    ```json
    {
      "collection_id": "nexusai-forensic-demo",
      "display_name": "Operation Falcon Sentinel",
      "classification": "RESTRICTED // LAW ENFORCEMENT SENSITIVE",
      "created_at": "2026-08-01T08:00:00Z",
      "lead_analyst": "Investigator S. Khan",
      "horizon": {
        "earliest_event_at": "2026-06-01T04:12:00Z",
        "latest_event_at": "2026-08-10T19:45:00Z",
        "span_days": 71
      }
    }
    ```
* **Analyst Value**: Gives investigators an immediate, authoritative time envelope across all evidence modalities (CDR, IPDR, ANPR, Access Logs) and enterprise classification markers without triggering expensive template aggregation queries on every header mount.

### 4. Authoritative Case Annotations & Investigative Hypotheses Store
* **Target Feature & Route**: `/cases/:id/overview` (Analyst Notes & Lead Tracker Bento Tile)
* **Current Workaround**: Case notes and operational hypotheses are managed locally in browser storage via `nexusai.casenotes.${caseId}`.
* **Proposed Backend API Contract**:
  - `GET /api/v1/cases/:id/notes`
  - `POST /api/v1/cases/:id/notes` with body: `{ "content": string, "category": "lead" | "hypothesis" | "finding", "pinned": boolean }`
  - `DELETE /api/v1/cases/:id/notes/:noteId`
  - Response shape:
    ```json
    {
      "notes": [
        {
          "id": "note-9812",
          "case_id": "nexusai-forensic-demo",
          "author": "analyst@nexusai.local",
          "content": "Primary target device swapped SIM at 14:22:01 UTC near Tower 404.",
          "category": "hypothesis",
          "pinned": true,
          "created_at": "2026-08-10T15:30:00Z"
        }
      ]
    }
    ```
* **Analyst Value**: Allows multi-analyst joint forensic teams to share corroborated leads, test conflicting operational hypotheses, and preserve human analytical context directly on the case dossier instead of losing insights in disconnected spreadsheets.

