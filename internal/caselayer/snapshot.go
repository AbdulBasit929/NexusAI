package caselayer

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
)

// Snapshot states: the whole response, a copy with the long lists cut, or none (the turn can only be run again).
const (
	SnapshotFull    = "full"
	SnapshotTrimmed = "trimmed"
	SnapshotNone    = "none"
)

// SnapshotLimits bound what a turn stores of an answer. RawMax is the size of the JSON, StoredMax the size after gzip; TrimRows and TrimEvidenceRows are what a trimmed copy keeps of the
// result rows and of the rows of the evidence list.
type SnapshotLimits struct {
	RawMax           int
	StoredMax        int
	TrimRows         int
	TrimEvidenceRows int
}

// DefaultSnapshotLimits: 8 MiB of JSON, 1 MiB stored, 200 result rows and 100 evidence rows when trimmed.
var DefaultSnapshotLimits = SnapshotLimits{RawMax: 8 << 20, StoredMax: 1 << 20, TrimRows: 200, TrimEvidenceRows: 100}

// Snapshot is the stored form of an engine response.
type Snapshot struct {
	Blob   []byte // gzip
	State  string
	Bytes  int    // size of the JSON that was stored
	SHA256 string // of that JSON
}

func gzipBytes(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BuildSnapshot stores the response whole when it fits the limits, otherwise with the long lists cut (and a note saying so), otherwise not at all. The SHA-256 is of the JSON that was stored.
func BuildSnapshot(raw []byte, lim SnapshotLimits) (Snapshot, error) {
	if len(raw) == 0 {
		return Snapshot{State: SnapshotNone}, nil
	}
	if len(raw) <= lim.RawMax {
		blob, err := gzipBytes(raw)
		if err != nil {
			return Snapshot{}, err
		}
		if len(blob) <= lim.StoredMax {
			return Snapshot{Blob: blob, State: SnapshotFull, Bytes: len(raw), SHA256: sha256Hex(raw)}, nil
		}
	}
	trimmed, ok := trimResponse(raw, lim)
	if !ok {
		return Snapshot{State: SnapshotNone}, nil
	}
	blob, err := gzipBytes(trimmed)
	if err != nil {
		return Snapshot{}, err
	}
	if len(blob) > lim.StoredMax || len(trimmed) > lim.RawMax {
		return Snapshot{State: SnapshotNone}, nil
	}
	return Snapshot{Blob: blob, State: SnapshotTrimmed, Bytes: len(trimmed), SHA256: sha256Hex(trimmed)}, nil
}

// trimResponse cuts the result rows and the evidence rows of a response and adds "snapshot_trimmed" with what was kept. It reports false if the response is not a JSON object.
func trimResponse(raw []byte, lim SnapshotLimits) ([]byte, bool) {
	m, err := decodeJSON(raw)
	if err != nil {
		return nil, false
	}
	note := map[string]any{"rows_kept": lim.TrimRows, "evidence_rows_kept": lim.TrimEvidenceRows}
	if rows, ok := dig(m, "enterprise", "data_grid", "rows").([]any); ok && len(rows) > lim.TrimRows {
		note["rows_total"] = len(rows)
		dig(m, "enterprise", "data_grid").(map[string]any)["rows"] = rows[:lim.TrimRows]
	}
	if rows, ok := dig(m, "enterprise", "evidence", "rows").([]any); ok && len(rows) > lim.TrimEvidenceRows {
		note["evidence_rows_total"] = len(rows)
		dig(m, "enterprise", "evidence").(map[string]any)["rows"] = rows[:lim.TrimEvidenceRows]
	}
	if recs, ok := m["records"].([]any); ok && len(recs) > lim.TrimRows {
		note["records_total"] = len(recs)
		m["records"] = recs[:lim.TrimRows]
	}
	m["snapshot_trimmed"] = note
	out, err := json.Marshal(m)
	if err != nil {
		return nil, false
	}
	return out, true
}

// DecodeSnapshot unzips a stored snapshot, refusing one that would expand past the limit (a snapshot is written by this service, but the column is data like any other).
func DecodeSnapshot(blob []byte, lim SnapshotLimits) ([]byte, error) {
	if len(blob) == 0 {
		return nil, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(blob))
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	raw, err := io.ReadAll(io.LimitReader(zr, int64(lim.RawMax)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > lim.RawMax {
		return nil, BadRequest("The stored answer is larger than the limit.")
	}
	return raw, nil
}
