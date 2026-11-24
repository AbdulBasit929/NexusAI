package main

// Known sets for images, the watch lists (design and gates: docs/data/image-known-sets-preregistration-20261124.md; migration 039).
//
// A known set is a named list of pictures known in advance, kept as HASHES ONLY (the SHA-256 of the file and the 256-bit PDQ-style hash of migration 038), of one of two kinds:
// of_interest (a watch list) and known_harmless (noise to push out of the way). A picture of a case is FLAGGED when it is the identical file of a set hash (tier identical) or its hash is within the
// calibrated distance of one (tier same_image, the same 56 bits as the copy search). A flag changes nothing on the evidence; hiding flagged pictures is a choice of the viewer.
// The endpoints exist only when FORENSIC_IMAGE_KNOWN_SETS is on (and migration 039 is applied):
//
//	GET  /images/known-sets[?include_retired=1]      the sets of the tenant with their hash counts
//	POST /images/known-sets                          {"name","kind","description"} creates a set
//	POST /images/known-sets/{id}/hashes              {"csv": "..."} or {"hashes":[{sha256,pdq256,label}]} or {"evidence_ids":[...]} (with collection_id) adds hashes
//	POST /images/known-sets/{id}/retire              stops its flags (the hashes stay)
//	GET  /images/known?collection_id=&case_id=&sets= which pictures of the case are flagged, with the coverage line
//
// Every create, add, retire and check leaves a row in forensic.image_known_event. No picture is stored; nothing here identifies a person.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	imageKnownContract       = "forensics.image-known-sets/v1"
	knownSetMaxHashesPerCall = 10000
	knownSetMaxBody          = 4 << 20
	knownSetMaxLabelRunes    = 512
	knownSetRejectedListed   = 50
)

func imageKnownSetsEnabled() bool {
	return envBoolValue(os.Getenv("FORENSIC_IMAGE_KNOWN_SETS"), false) //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

var (
	knownSHA256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	knownBitsPattern   = regexp.MustCompile(`^[01]{256}$`)
)

type knownSet struct {
	SetID       string     `json:"set_id"`
	Name        string     `json:"name"`
	Kind        string     `json:"kind"`
	Description string     `json:"description"`
	Hashes      int        `json:"hashes"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	RetiredAt   *time.Time `json:"retired_at,omitempty"`
}

type knownHashInput struct {
	SHA256 string `json:"sha256"`
	PDQ256 string `json:"pdq256"` // 64 hexadecimal characters
	Label  string `json:"label"`
}

type knownHashRow struct {
	SHA256, PDQBits, Label, Source, SourceRef string
}

type knownReject struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

type knownFlag struct {
	EvidenceID string `json:"evidence_id"`
	SetID      string `json:"set_id"`
	SetName    string `json:"set_name"`
	SetKind    string `json:"set_kind"`
	Tier       string `json:"tier"`
	Distance   int    `json:"distance"`
	Label      string `json:"label,omitempty"`
}

type knownCoverage struct {
	ActiveSets      int64  `json:"active_sets"`
	KnownHashes     int64  `json:"known_hashes"`
	ImagesInCase    int64  `json:"images_in_case"`
	ImagesWithHash  int64  `json:"images_with_hash"`
	HashMaxDistance int    `json:"hash_max_distance"`
	Statement       string `json:"statement"`
}

type knownSetsDeps struct {
	db      *pgxpool.Pool
	hashMax int
}

func newKnownSetsDeps(db *pgxpool.Pool) knownSetsDeps {
	return knownSetsDeps{db: db, hashMax: imageSearchSettingsFromEnv().HashMaxDistance}
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------------------- pure parts

func pluralWord(n int64, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// knownCoverageStatement is the sentence that goes with every flag answer: how much was checked, against how much.
func knownCoverageStatement(c knownCoverage) string {
	if c.ActiveSets == 0 {
		return "No known set is active, so nothing was checked."
	}
	text := fmt.Sprintf("Checked %d of %d %s against %d %s in %d %s.", c.ImagesWithHash, c.ImagesInCase, pluralWord(c.ImagesInCase, "image", "images"),
		c.KnownHashes, pluralWord(c.KnownHashes, "hash", "hashes"), c.ActiveSets, pluralWord(c.ActiveSets, "set", "sets"))
	if c.ImagesWithHash < c.ImagesInCase {
		missing := c.ImagesInCase - c.ImagesWithHash
		text += fmt.Sprintf(" %d %s no perceptual hash yet and %s checked by file hash only.", missing, pluralWord(missing, "image has", "images have"), pluralWord(missing, "was", "were"))
	}
	return text
}

// normaliseKnownHash validates one row and returns the stored forms: the lower-case SHA-256 (or ""), the perceptual hash as a 256-character bit string (or ""), and a label cut to its limit.
func normaliseKnownHash(in knownHashInput) (sha, bits, label string, err error) {
	sha = strings.ToLower(strings.TrimSpace(in.SHA256))
	if sha != "" && !knownSHA256Pattern.MatchString(sha) {
		return "", "", "", errors.New("the file hash must be 64 hexadecimal characters")
	}
	if pdq := strings.TrimSpace(in.PDQ256); pdq != "" {
		if bits, err = bitString(strings.ToLower(pdq)); err != nil {
			return "", "", "", err
		}
	}
	if sha == "" && bits == "" {
		return "", "", "", errors.New("a row needs a file hash (sha256) or a perceptual hash (pdq256)")
	}
	label = strings.TrimSpace(in.Label)
	if utf8.RuneCountInString(label) > knownSetMaxLabelRunes {
		label = string([]rune(label)[:knownSetMaxLabelRunes])
	}
	return sha, bits, label, nil
}

func knownCSVColumn(cell string) string {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(cell), `"'`)) {
	case "sha256", "sha-256", "sha_256", "file_hash", "filehash":
		return "sha256"
	case "pdq", "pdq256", "phash", "perceptual_hash", "perceptual":
		return "pdq256"
	case "label", "name", "note", "notes", "filename", "file_name", "file":
		return "label"
	}
	return ""
}

// parseKnownHashCSV reads a table of hashes: comma, semicolon or tab separated; an optional header row naming the columns (sha256, pdq256, label) in any order; otherwise the columns are
// sha256, pdq256, label in that order. Blank lines and lines starting with # are skipped. Rows that cannot be used are returned with the reason and their line number.
func parseKnownHashCSV(text string) (rows []knownHashInput, rejected []knownReject) {
	text = strings.TrimPrefix(text, "\ufeff")
	columns := []string{"sha256", "pdq256", "label"}
	headerSeen := false
	for index, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		number := index + 1
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		separator := ","
		switch {
		case strings.Contains(line, "\t"):
			separator = "\t"
		case !strings.Contains(line, ",") && strings.Contains(line, ";"):
			separator = ";"
		}
		cells := strings.Split(line, separator)
		if !headerSeen {
			headerSeen = true
			named := make([]string, len(cells))
			found := false
			for i, cell := range cells {
				named[i] = knownCSVColumn(cell)
				found = found || named[i] != ""
			}
			if found && !knownSHA256Pattern.MatchString(strings.ToLower(strings.TrimSpace(cells[0]))) {
				columns = named
				continue
			}
		}
		var row knownHashInput
		for i, cell := range cells {
			if i >= len(columns) {
				break
			}
			cell = strings.Trim(strings.TrimSpace(cell), `"`)
			switch columns[i] {
			case "sha256":
				row.SHA256 = cell
			case "pdq256":
				row.PDQ256 = cell
			case "label":
				row.Label = cell
			}
		}
		if _, _, _, err := normaliseKnownHash(row); err != nil {
			rejected = append(rejected, knownReject{Line: number, Reason: err.Error()})
			continue
		}
		rows = append(rows, row)
	}
	return rows, rejected
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------------------- plumbing

func (k knownSetsDeps) scope(w http.ResponseWriter, r *http.Request, needCollection bool) (forensicScope, bool) {
	query := r.URL.Query()
	scope, err := bindForensicScope(r, query.Get("tenant_id"), query.Get("collection_id"), query.Get("case_id"), "")
	if err != nil {
		writeScopeError(w, err)
		return scope, false
	}
	if needCollection && scope.CollectionID == "" {
		writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
		return scope, false
	}
	if k.db == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
		return scope, false
	}
	return scope, true
}

func (k knownSetsDeps) begin(ctx context.Context, tenant string) (pgx.Tx, error) {
	tx, err := k.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenant); err != nil {
		_ = tx.Rollback(ctx)
		return nil, fmt.Errorf("set tenant context: %w", err)
	}
	return tx, nil
}

func decodeKnownBody(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, knownSetMaxBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("the request body must be JSON of the documented shape (at most %d MB): %v", knownSetMaxBody>>20, err))
		return false
	}
	return true
}

const knownEventSQL = `
INSERT INTO forensic.image_known_event (tenant_id, set_id, kind, actor, collection_id, case_id, detail)
VALUES ($1, NULLIF($2, '')::uuid, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7::jsonb)`

func recordKnownEvent(ctx context.Context, tx pgx.Tx, scope forensicScope, setID, kind string, detail map[string]any) error {
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, knownEventSQL, scope.TenantID, setID, kind, imageSearchActor(scope), scope.CollectionID, scope.CaseID, string(raw))
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

const knownSetListSQL = `
SELECT s.set_id::text, s.name, s.kind, s.description, s.created_by, s.created_at, s.retired_at,
       (SELECT count(*) FROM forensic.image_known_hash k WHERE k.tenant_id = s.tenant_id AND k.set_id = s.set_id)::int
FROM forensic.image_known_set s
WHERE s.tenant_id = current_setting('app.tenant_id', true) AND ($1::boolean OR s.retired_at IS NULL)
ORDER BY (s.retired_at IS NOT NULL), lower(s.name), s.created_at`

func listKnownSets(ctx context.Context, tx pgx.Tx, includeRetired bool) ([]knownSet, error) {
	rows, err := tx.Query(ctx, knownSetListSQL, includeRetired)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sets := []knownSet{}
	for rows.Next() {
		var s knownSet
		if err := rows.Scan(&s.SetID, &s.Name, &s.Kind, &s.Description, &s.CreatedBy, &s.CreatedAt, &s.RetiredAt, &s.Hashes); err != nil {
			return nil, err
		}
		sets = append(sets, s)
	}
	return sets, rows.Err()
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------------------- handlers

func (k knownSetsDeps) list(w http.ResponseWriter, r *http.Request) {
	scope, ok := k.scope(w, r, false)
	if !ok {
		return
	}
	tx, err := k.begin(r.Context(), scope.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	sets, err := listKnownSets(r.Context(), tx, r.URL.Query().Get("include_retired") == "1")
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("list known sets: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"contract_version": imageKnownContract, "sets": sets})
}

type knownSetCreateRequest struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

func (k knownSetsDeps) create(w http.ResponseWriter, r *http.Request) {
	var body knownSetCreateRequest
	if !decodeKnownBody(w, r, &body) {
		return
	}
	body.Name, body.Description = strings.TrimSpace(body.Name), strings.TrimSpace(body.Description)
	switch {
	case body.Name == "" || utf8.RuneCountInString(body.Name) > 120:
		writeError(w, http.StatusBadRequest, errors.New("name must be 1 to 120 characters"))
		return
	case body.Kind != "of_interest" && body.Kind != "known_harmless":
		writeError(w, http.StatusBadRequest, errors.New("kind must be of_interest or known_harmless"))
		return
	case utf8.RuneCountInString(body.Description) > 1000:
		writeError(w, http.StatusBadRequest, errors.New("description must be at most 1000 characters"))
		return
	}
	scope, ok := k.scope(w, r, false)
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := k.begin(ctx, scope.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var set knownSet
	err = tx.QueryRow(ctx, `
		INSERT INTO forensic.image_known_set (tenant_id, name, kind, description, created_by) VALUES ($1, $2, $3, $4, $5)
		RETURNING set_id::text, name, kind, description, created_by, created_at`,
		scope.TenantID, body.Name, body.Kind, body.Description, imageSearchActor(scope)).Scan(&set.SetID, &set.Name, &set.Kind, &set.Description, &set.CreatedBy, &set.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, errors.New("an active set with this name already exists"))
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Errorf("create set: %w", err))
		return
	}
	if err := recordKnownEvent(ctx, tx, scope, set.SetID, "set_created", map[string]any{"name": set.Name, "kind": set.Kind}); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("record event: %w", err))
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"contract_version": imageKnownContract, "set": set})
}

func (k knownSetsDeps) retire(w http.ResponseWriter, r *http.Request) {
	setID := r.PathValue("id")
	if _, err := uuid.Parse(setID); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("the set id must be a UUID"))
		return
	}
	scope, ok := k.scope(w, r, false)
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := k.begin(ctx, scope.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var name, createdBy string
	var retired bool
	err = tx.QueryRow(ctx, `SELECT name, created_by, retired_at IS NOT NULL FROM forensic.image_known_set WHERE tenant_id = current_setting('app.tenant_id', true) AND set_id = $1::uuid`, setID).Scan(&name, &createdBy, &retired)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, errors.New("no such set"))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if retired {
		writeError(w, http.StatusConflict, errors.New("the set is already retired"))
		return
	}
	// who may retire: the creator or an administrator (with authentication off, anyone)
	if scope.ActorID != "" && scope.ActorID != createdBy && requireForensicAdmin(r) != nil {
		writeError(w, http.StatusForbidden, errors.New("only the creator of a set or an administrator may retire it"))
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE forensic.image_known_set SET retired_at = now(), retired_by = $2 WHERE set_id = $1::uuid AND retired_at IS NULL`, setID, imageSearchActor(scope)); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("retire set: %w", err))
		return
	}
	if err := recordKnownEvent(ctx, tx, scope, setID, "set_retired", map[string]any{"name": name}); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("record event: %w", err))
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"contract_version": imageKnownContract, "set_id": setID, "retired": true})
}

type knownHashesRequest struct {
	CSV         string           `json:"csv"`
	Hashes      []knownHashInput `json:"hashes"`
	EvidenceIDs []string         `json:"evidence_ids"`
}

const knownEvidenceHashSQL = `
SELECT e.evidence_id::text, e.sha256, COALESCE(h.pdq256::text, ''), e.original_filename
FROM forensic.evidence_items e
LEFT JOIN forensic.image_hash h ON h.tenant_id = e.tenant_id AND h.collection_id = e.collection_id AND h.evidence_id = e.evidence_id AND h.version_id = e.current_version_id
WHERE e.tenant_id = current_setting('app.tenant_id', true) AND e.collection_id = $1 AND e.evidence_id = ANY ($2::uuid[]) AND e.modality = 'image'
ORDER BY e.evidence_id, h.chunk_ref`

const knownHashInsertSQL = `
INSERT INTO forensic.image_known_hash (tenant_id, set_id, sha256, pdq256, label, source, source_ref, added_by)
VALUES ($1, $2::uuid, NULLIF($3, ''), NULLIF($4, '')::bit(256), $5, $6, NULLIF($7, ''), $8)
ON CONFLICT DO NOTHING`

func (k knownSetsDeps) addHashes(w http.ResponseWriter, r *http.Request) {
	setID := r.PathValue("id")
	if _, err := uuid.Parse(setID); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("the set id must be a UUID"))
		return
	}
	var body knownHashesRequest
	if !decodeKnownBody(w, r, &body) {
		return
	}
	if body.CSV == "" && len(body.Hashes) == 0 && len(body.EvidenceIDs) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("send csv, hashes or evidence_ids"))
		return
	}
	for _, id := range body.EvidenceIDs {
		if _, err := uuid.Parse(id); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("evidence id %q is not a UUID", id))
			return
		}
	}
	// the rows that can be built without the database
	var rows []knownHashRow
	var rejected []knownReject
	csvRows, csvRejected := parseKnownHashCSV(body.CSV)
	rejected = append(rejected, csvRejected...)
	addInputs := func(inputs []knownHashInput, source string, offset int) {
		for i, in := range inputs {
			sha, bits, label, err := normaliseKnownHash(in)
			if err != nil {
				rejected = append(rejected, knownReject{Line: offset + i + 1, Reason: err.Error()})
				continue
			}
			rows = append(rows, knownHashRow{SHA256: sha, PDQBits: bits, Label: label, Source: source})
		}
	}
	addInputs(csvRows, "csv", 0)
	addInputs(body.Hashes, "api", 0)
	if len(rows)+len(rejected)+len(body.EvidenceIDs) > knownSetMaxHashesPerCall {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Errorf("at most %d rows per request", knownSetMaxHashesPerCall))
		return
	}
	scope, ok := k.scope(w, r, len(body.EvidenceIDs) > 0)
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := k.begin(ctx, scope.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var active bool
	err = tx.QueryRow(ctx, `SELECT retired_at IS NULL FROM forensic.image_known_set WHERE tenant_id = current_setting('app.tenant_id', true) AND set_id = $1::uuid`, setID).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, errors.New("no such set"))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !active {
		writeError(w, http.StatusConflict, errors.New("the set is retired; create a new set instead"))
		return
	}
	notPictures := 0
	if len(body.EvidenceIDs) > 0 {
		found := map[string]bool{}
		evidenceRows, err := tx.Query(ctx, knownEvidenceHashSQL, scope.CollectionID, body.EvidenceIDs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("read evidence hashes: %w", err))
			return
		}
		for evidenceRows.Next() {
			var id, sha, bits, filename string
			if err := evidenceRows.Scan(&id, &sha, &bits, &filename); err != nil {
				evidenceRows.Close()
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			found[id] = true
			if !knownSHA256Pattern.MatchString(sha) {
				sha = ""
			}
			if !knownBitsPattern.MatchString(bits) {
				bits = ""
			}
			if sha == "" && bits == "" {
				continue
			}
			label := filename
			if utf8.RuneCountInString(label) > knownSetMaxLabelRunes {
				label = string([]rune(label)[:knownSetMaxLabelRunes])
			}
			rows = append(rows, knownHashRow{SHA256: sha, PDQBits: bits, Label: label, Source: "evidence", SourceRef: id})
		}
		evidenceRows.Close()
		if err := evidenceRows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		for _, id := range body.EvidenceIDs {
			if !found[id] {
				notPictures++
			}
		}
	}
	added, already := 0, 0
	for _, row := range rows {
		tag, err := tx.Exec(ctx, knownHashInsertSQL, scope.TenantID, setID, row.SHA256, row.PDQBits, row.Label, row.Source, row.SourceRef, imageSearchActor(scope))
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("add hash: %w", err))
			return
		}
		if tag.RowsAffected() == 1 {
			added++
		} else {
			already++
		}
	}
	var total int
	if err := tx.QueryRow(ctx, `SELECT count(*)::int FROM forensic.image_known_hash WHERE tenant_id = current_setting('app.tenant_id', true) AND set_id = $1::uuid`, setID).Scan(&total); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	detail := map[string]any{"added": added, "already_in_set": already, "rejected": len(rejected), "evidence_not_found": notPictures, "hashes_in_set": total}
	if err := recordKnownEvent(ctx, tx, scope, setID, "hashes_added", detail); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("record event: %w", err))
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	listed := rejected
	if len(listed) > knownSetRejectedListed {
		listed = listed[:knownSetRejectedListed]
	}
	if listed == nil {
		listed = []knownReject{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"contract_version": imageKnownContract, "set_id": setID, "added": added, "already_in_set": already,
		"rejected_count": len(rejected), "rejected": listed, "evidence_not_found": notPictures, "hashes_in_set": total,
	})
}

const knownMatchesSQL = `
SELECT evidence_id::text, set_id::text, set_name, set_kind, tier, distance, label
FROM forensic.image_known_matches($1, $2, NULLIF($3, ''), $4::uuid[])`

const knownStateSQL = `SELECT active_sets, known_hashes, images_in_case, images_with_hash FROM forensic.image_known_state($1, NULLIF($2, ''))`

func (k knownSetsDeps) flags(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var only []string
	if raw := strings.TrimSpace(query.Get("sets")); raw != "" {
		for _, id := range strings.Split(raw, ",") {
			id = strings.TrimSpace(id)
			if _, err := uuid.Parse(id); err != nil {
				writeError(w, http.StatusBadRequest, fmt.Errorf("set id %q is not a UUID", id))
				return
			}
			only = append(only, id)
		}
	}
	scope, ok := k.scope(w, r, true)
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := k.begin(ctx, scope.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var cover knownCoverage
	if err := tx.QueryRow(ctx, knownStateSQL, scope.CollectionID, scope.CaseID).Scan(&cover.ActiveSets, &cover.KnownHashes, &cover.ImagesInCase, &cover.ImagesWithHash); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("coverage: %w", err))
		return
	}
	cover.HashMaxDistance = k.hashMax
	cover.Statement = knownCoverageStatement(cover)
	flags := []knownFlag{}
	flagged := map[string]bool{}
	byKind := map[string]int{"of_interest": 0, "known_harmless": 0}
	if cover.ActiveSets > 0 {
		rows, err := tx.Query(ctx, knownMatchesSQL, scope.CollectionID, k.hashMax, scope.CaseID, only)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("known matches: %w", err))
			return
		}
		for rows.Next() {
			var f knownFlag
			var label *string
			if err := rows.Scan(&f.EvidenceID, &f.SetID, &f.SetName, &f.SetKind, &f.Tier, &f.Distance, &label); err != nil {
				rows.Close()
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			if label != nil {
				f.Label = *label
			}
			flags = append(flags, f)
			if !flagged[f.EvidenceID] {
				flagged[f.EvidenceID] = true
			}
			byKind[f.SetKind]++
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	sets, err := listKnownSets(ctx, tx, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("list known sets: %w", err))
		return
	}
	detail := map[string]any{"flags": len(flags), "flagged_pictures": len(flagged), "max_distance": k.hashMax, "sets_checked": cover.ActiveSets, "hashes": cover.KnownHashes, "images_in_case": cover.ImagesInCase}
	if len(only) > 0 {
		detail["sets_filter"] = only
	}
	if err := recordKnownEvent(ctx, tx, scope, "", "check", detail); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("record event: %w", err))
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"contract_version": imageKnownContract,
		"scope":            map[string]any{"tenant_id": scope.TenantID, "collection_id": scope.CollectionID, "case_id": optionalString(scope.CaseID)},
		"coverage":         cover,
		"sets":             sets,
		"flags":            flags,
		"flagged_pictures": len(flagged),
		"by_kind":          byKind,
	})
}
