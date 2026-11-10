package caselayer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// typedFamilyTables are the tables of records that the analyst views read, per case. The watermark of each is its row count and the time of its latest ingestion.
var typedFamilyTables = []string{"comm_event", "data_session", "app_usage", "plate_sighting", "subscriber_registration", "bank_transaction", "document_mention"}

// DVEvidence is one evidence item at its current version.
type DVEvidence struct {
	EvidenceID string `json:"evidence_id"`
	VersionID  string `json:"version_id"`
	SHA256     string `json:"sha256"`
	Name       string `json:"name"`
}

// DVFamily is the watermark of one family of typed records.
type DVFamily struct {
	Family         string `json:"family"`
	Rows           int64  `json:"rows"`
	LastIngestedAt string `json:"last_ingested_at,omitempty"`
	Unavailable    bool   `json:"unavailable,omitempty"`
}

// DataVersion is the document behind a data_version_hash: what the case held when an answer was computed.
type DataVersion struct {
	Evidence []DVEvidence   `json:"evidence"`
	Families []DVFamily     `json:"families"`
	Settings map[string]any `json:"settings"`
}

// Hash is the SHA-256 of the canonical JSON of the document (slices are sorted before the hash is taken; map keys are sorted by the encoder).
func (v DataVersion) Hash() string {
	raw, _ := json.Marshal(v)
	return sha256Hex(raw)
}

// readDataVersion reads the data version of a case in the caller's transaction. It never fails the turn for one family: a family that cannot be read in time (or the role may not read)
// is recorded as unavailable, which makes the version differ from one where it could be read, and the diff says so.
func readDataVersion(ctx context.Context, tx pgx.Tx, tenantID, caseID string) (DataVersion, error) {
	v := DataVersion{Evidence: []DVEvidence{}, Families: []DVFamily{}, Settings: map[string]any{}}
	rows, err := tx.Query(ctx, `SELECT e.evidence_id::text, coalesce(e.current_version_id::text, ''), coalesce(ver.content_sha256, e.sha256, ''), e.original_filename
		FROM forensic.evidence_items e
		LEFT JOIN forensic.evidence_versions ver ON ver.tenant_id = e.tenant_id AND ver.collection_id = e.collection_id AND ver.evidence_id = e.evidence_id AND ver.version_id = e.current_version_id
		WHERE e.tenant_id = $1 AND e.collection_id = $2
		ORDER BY e.evidence_id`, tenantID, caseID)
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var x DVEvidence
		if err := rows.Scan(&x.EvidenceID, &x.VersionID, &x.SHA256, &x.Name); err != nil {
			rows.Close()
			return v, err
		}
		v.Evidence = append(v.Evidence, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return v, err
	}
	for _, table := range typedFamilyTables {
		var present bool
		if err := tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, "forensic."+table).Scan(&present); err != nil {
			return v, err
		}
		if !present {
			continue
		}
		fam := DVFamily{Family: table}
		sp, err := tx.Begin(ctx) // a savepoint: a family that times out must not end the transaction
		if err != nil {
			return v, err
		}
		var last *time.Time
		qerr := sp.QueryRow(ctx, fmt.Sprintf(`SELECT count(*), max(ingested_at) FROM %s WHERE tenant_id = $1 AND collection_id = $2`, pgx.Identifier{"forensic", table}.Sanitize()), tenantID, caseID).Scan(&fam.Rows, &last)
		if qerr != nil {
			_ = sp.Rollback(ctx)
			fam = DVFamily{Family: table, Unavailable: true}
		} else {
			_ = sp.Commit(ctx)
			if last != nil {
				fam.LastIngestedAt = last.UTC().Format(time.RFC3339Nano)
			}
		}
		v.Families = append(v.Families, fam)
	}
	sort.Slice(v.Families, func(i, j int) bool { return v.Families[i].Family < v.Families[j].Family })
	var status string
	var hub *int
	err = tx.QueryRow(ctx, `SELECT status, hub_threshold FROM forensic.case_record WHERE tenant_id = $1 AND collection_id = $2`, tenantID, caseID).Scan(&status, &hub)
	switch {
	case err == nil:
		v.Settings["status"] = status
		if hub != nil {
			v.Settings["hub_threshold"] = *hub
		}
	case err == pgx.ErrNoRows:
		v.Settings["status"] = "open"
	default:
		return v, err
	}
	return v, nil
}

// storeDataVersion keeps the document once per distinct hash.
func storeDataVersion(ctx context.Context, tx pgx.Tx, tenantID, caseID string, v DataVersion) (string, error) {
	hash := v.Hash()
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `INSERT INTO forensic.case_data_version (tenant_id, collection_id, data_version_hash, detail) VALUES ($1, $2, $3, $4::jsonb) ON CONFLICT DO NOTHING`, tenantID, caseID, hash, string(raw))
	return hash, err
}

// loadDataVersion reads a stored document; nil when the hash is unknown (for instance a turn of a case whose version was never stored).
func loadDataVersion(ctx context.Context, tx pgx.Tx, tenantID, caseID, hash string) (*DataVersion, error) {
	if hash == "" {
		return nil, nil
	}
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT detail FROM forensic.case_data_version WHERE tenant_id = $1 AND collection_id = $2 AND data_version_hash = $3`, tenantID, caseID, hash).Scan(&raw)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var v DataVersion
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// DataVersionChange says what moved between two versions of a case's data.
type DataVersionChange struct {
	EvidenceAdded    []DVEvidence   `json:"evidence_added"`
	EvidenceRemoved  []DVEvidence   `json:"evidence_removed"`
	EvidenceReplaced []DVReplaced   `json:"evidence_replaced"`
	Families         []DVFamilyMove `json:"families"`
	Settings         []string       `json:"settings_changed"`
}

// DVReplaced is an evidence item whose current version changed.
type DVReplaced struct {
	EvidenceID    string `json:"evidence_id"`
	Name          string `json:"name"`
	BeforeVersion string `json:"before_version"`
	AfterVersion  string `json:"after_version"`
}

// DVFamilyMove is a family whose watermark moved.
type DVFamilyMove struct {
	Family       string `json:"family"`
	RowsBefore   int64  `json:"rows_before"`
	RowsAfter    int64  `json:"rows_after"`
	LatestBefore string `json:"latest_ingestion_before,omitempty"`
	LatestAfter  string `json:"latest_ingestion_after,omitempty"`
}

// CompareDataVersions lists what differs between two documents.
func CompareDataVersions(before, after DataVersion) DataVersionChange {
	ch := DataVersionChange{EvidenceAdded: []DVEvidence{}, EvidenceRemoved: []DVEvidence{}, EvidenceReplaced: []DVReplaced{}, Families: []DVFamilyMove{}, Settings: []string{}}
	b := map[string]DVEvidence{}
	for _, e := range before.Evidence {
		b[e.EvidenceID] = e
	}
	a := map[string]DVEvidence{}
	for _, e := range after.Evidence {
		a[e.EvidenceID] = e
		old, ok := b[e.EvidenceID]
		switch {
		case !ok:
			ch.EvidenceAdded = append(ch.EvidenceAdded, e)
		case old.VersionID != e.VersionID || old.SHA256 != e.SHA256:
			ch.EvidenceReplaced = append(ch.EvidenceReplaced, DVReplaced{EvidenceID: e.EvidenceID, Name: e.Name, BeforeVersion: old.VersionID, AfterVersion: e.VersionID})
		}
	}
	for _, e := range before.Evidence {
		if _, ok := a[e.EvidenceID]; !ok {
			ch.EvidenceRemoved = append(ch.EvidenceRemoved, e)
		}
	}
	fb := map[string]DVFamily{}
	for _, f := range before.Families {
		fb[f.Family] = f
	}
	seen := map[string]bool{}
	for _, f := range after.Families {
		seen[f.Family] = true
		old := fb[f.Family]
		if old.Rows != f.Rows || old.LastIngestedAt != f.LastIngestedAt || old.Unavailable != f.Unavailable {
			ch.Families = append(ch.Families, DVFamilyMove{Family: f.Family, RowsBefore: old.Rows, RowsAfter: f.Rows, LatestBefore: old.LastIngestedAt, LatestAfter: f.LastIngestedAt})
		}
	}
	for _, f := range before.Families {
		if !seen[f.Family] {
			ch.Families = append(ch.Families, DVFamilyMove{Family: f.Family, RowsBefore: f.Rows, LatestBefore: f.LastIngestedAt})
		}
	}
	keys := map[string]bool{}
	for k := range before.Settings {
		keys[k] = true
	}
	for k := range after.Settings {
		keys[k] = true
	}
	for k := range keys {
		if fmt.Sprint(before.Settings[k]) != fmt.Sprint(after.Settings[k]) {
			ch.Settings = append(ch.Settings, k)
		}
	}
	sort.Strings(ch.Settings)
	return ch
}
