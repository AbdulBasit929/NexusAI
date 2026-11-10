package caselayer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Export of answers and findings to the report builder (backend request BR-4, the part that takes what the analyst has found out of the conversation and the findings). An export is a bundle
// of exhibits: for each answer or finding the question, the sentence, the table, the files cited, how it was produced (engine, model, query, data version) and a SHA-256 of all of that. The
// bundle has its own hash over its exhibits in order, comes as JSON for the builder and as Markdown to read or print, and is written to the audit trail of every case it draws on (the hashes, never
// the text), so a printed exhibit can be matched to what the system produced and a change to it can be seen. Nothing is stored: the same call made again gives the same exhibit hashes while the
// stored answers are the same, because they cannot change.

// ExportFormat names the layout of a bundle; a reader checks it before reading on.
const ExportFormat = "nexusai.exhibits/1"

const (
	defaultExportRows = 50
	maxExportRows     = 500
	maxExportItems    = 100
)

// CanExport is who may take an answer out of the system: those who work on a case, not someone who only reads it (viewer) or audits it. A global administrator is a lead everywhere.
func (r Role) CanExport() bool {
	return r == RoleOwner || r == RoleLead || r == RoleAnalyst || r == RoleReviewer || r == RoleOpen
}

// ExhibitTable is the result table of an answer as printed: the columns, the first rows (at most what the caller asked for, never more than the layer kept), the size of the whole result and the
// digest of all of it, so a table cut for the page can still be tied to the full result.
type ExhibitTable struct {
	Columns    []Column `json:"columns"`
	Rows       [][]any  `json:"rows"`
	RowCount   int      `json:"row_count"`
	RowsShown  int      `json:"rows_shown"`
	Truncated  bool     `json:"truncated"`
	RowsDigest string   `json:"rows_digest,omitempty"`
}

// ExhibitAnswer is the answer of an exhibit.
type ExhibitAnswer struct {
	Outcome  string        `json:"outcome"`
	Headline string        `json:"headline"`
	Text     string        `json:"text,omitempty"`
	Shape    string        `json:"shape,omitempty"`
	Figures  []Metric      `json:"figures"`
	Table    *ExhibitTable `json:"table,omitempty"`
}

// ExhibitFinding is the finding of an exhibit.
type ExhibitFinding struct {
	FindingID   string     `json:"finding_id"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	Basis       string     `json:"basis"`
	ReviewState string     `json:"review_state"`
	ReviewedBy  *string    `json:"reviewed_by"`
	ReviewedAt  *time.Time `json:"reviewed_at"`
	ReviewNote  *string    `json:"review_note"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ExhibitProvenance is how an answer was produced.
type ExhibitProvenance struct {
	TurnID          string     `json:"turn_id,omitempty"`
	ThreadID        string     `json:"thread_id,omitempty"`
	RunKind         string     `json:"run_kind,omitempty"`
	AskedBy         string     `json:"asked_by,omitempty"`
	AskedAt         *time.Time `json:"asked_at,omitempty"`
	AnsweredAt      *time.Time `json:"answered_at,omitempty"`
	EngineVersion   string     `json:"engine_version,omitempty"`
	Model           string     `json:"model,omitempty"`
	PromptID        string     `json:"prompt_id,omitempty"`
	Query           string     `json:"query,omitempty"`
	QueryHash       string     `json:"query_hash,omitempty"`
	ResultDigest    string     `json:"result_digest,omitempty"`
	SnapshotState   string     `json:"snapshot_state,omitempty"`
	DataVersionHash string     `json:"data_version_hash,omitempty"`
}

// ExhibitData is what the case held when the answer was computed. Recorded is false when the document of that version is not stored (the hash then still names it).
type ExhibitData struct {
	Hash     string         `json:"hash,omitempty"`
	Recorded bool           `json:"recorded"`
	Evidence []DVEvidence   `json:"evidence"`
	Families []DVFamily     `json:"families"`
	Settings map[string]any `json:"settings"`
}

// ExhibitReproduce says how to run the question again.
type ExhibitReproduce struct {
	Rerun string `json:"rerun"`
	Note  string `json:"note"`
}

// ExhibitContext is a fact of the day of the export; it is not covered by the hash of the exhibit (the answer did not change because the case did).
type ExhibitContext struct {
	DataCurrent *bool `json:"data_current"` // true when the case still holds the data version the answer was computed on; nil when that was not read
}

// Exhibit is one answer or one finding, ready for a report.
type Exhibit struct {
	ID         string            `json:"exhibit_id"`
	Kind       string            `json:"kind"` // answer | finding
	CaseID     string            `json:"case_id"`
	Title      string            `json:"title"`
	Question   string            `json:"question,omitempty"`
	Answer     *ExhibitAnswer    `json:"answer,omitempty"`
	Finding    *ExhibitFinding   `json:"finding,omitempty"`
	Evidence   []EvidenceRef     `json:"evidence"`
	Provenance ExhibitProvenance `json:"provenance"`
	Data       ExhibitData       `json:"data"`
	Reproduce  *ExhibitReproduce `json:"reproduce,omitempty"`
	SHA256     string            `json:"sha256,omitempty"`
	Context    *ExhibitContext   `json:"context,omitempty"`
}

// Skipped is something that was asked for or would have been taken and was not, with the reason in words.
type Skipped struct {
	Ref    string `json:"ref"`
	Reason string `json:"reason"`
}

// Bundle is an export. To check an exhibit from the JSON alone: remove its sha256 and context, write the rest with the keys of every object in order and no spaces, and take the SHA-256.
type Bundle struct {
	Format       string    `json:"format"`
	Title        string    `json:"title"`
	ThreadID     string    `json:"thread_id,omitempty"`
	CaseIDs      []string  `json:"case_ids"`
	ExportedBy   string    `json:"exported_by"`
	ExportedAt   time.Time `json:"exported_at"`
	Exhibits     []Exhibit `json:"exhibits"`
	Skipped      []Skipped `json:"skipped"`
	BundleSHA256 string    `json:"bundle_sha256"`
	Markdown     string    `json:"markdown"`
}

// ExportOptions say what goes into an export. TurnIDs names answers (none: the latest answered run of each question of the conversation); MaxRows is how many rows of each table are printed.
type ExportOptions struct {
	TurnIDs       []string
	FindingIDs    []string
	OnlyConfirmed bool
	MaxRows       int
	Title         string
}

// ---- canonical form and hashes -------------------------------------------------------------------------------------------------------------------------------------------

// canonicalJSON writes a value as JSON with the keys of every object in order, no spaces and no escaping of < > &, and numbers exactly as they were written: the form a hash is taken over, which
// a reader in any language can reproduce.
func canonicalJSON(v any) ([]byte, error) {
	var first bytes.Buffer
	enc := json.NewEncoder(&first)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(&first)
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	enc = json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(tree); err != nil { // maps are written with their keys sorted
		return nil, err
	}
	return bytes.TrimRight(out.Bytes(), "\n"), nil
}

// ComputeHash is the SHA-256 of the exhibit's canonical JSON without its sha256 and context.
func (e Exhibit) ComputeHash() (string, error) {
	e.SHA256, e.Context = "", nil
	raw, err := canonicalJSON(e)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// bundleHash covers the format, the cases and the hashes of the exhibits in the order they are printed.
func bundleHash(caseIDs, exhibitHashes []string) string {
	raw, _ := canonicalJSON(map[string]any{"format": ExportFormat, "case_ids": caseIDs, "exhibits": exhibitHashes})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// ---- building exhibits ----------------------------------------------------------------------------------------------------------------------------------------------------

// exhibitTable prints the first maxRows of a result and says what was cut. It is nil when the answer has no columns.
func exhibitTable(columns []Column, rows [][]any, rowCount int, digest string, maxRows int) *ExhibitTable {
	if len(columns) == 0 {
		return nil
	}
	switch {
	case maxRows <= 0:
		maxRows = defaultExportRows
	case maxRows > maxExportRows:
		maxRows = maxExportRows
	}
	shown := rows
	if len(shown) > maxRows {
		shown = shown[:maxRows]
	}
	if shown == nil {
		shown = [][]any{}
	}
	if rowCount < len(rows) {
		rowCount = len(rows)
	}
	return &ExhibitTable{Columns: columns, Rows: shown, RowCount: rowCount, RowsShown: len(shown), Truncated: rowCount > len(shown), RowsDigest: digest}
}

func decodeEvidence(raw json.RawMessage) []EvidenceRef {
	out := []EvidenceRef{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	if out == nil {
		out = []EvidenceRef{}
	}
	return out
}

func exhibitData(hash string, dv *DataVersion) ExhibitData {
	d := ExhibitData{Hash: hash, Evidence: []DVEvidence{}, Families: []DVFamily{}, Settings: map[string]any{}}
	if dv != nil {
		d.Recorded = true
		if dv.Evidence != nil {
			d.Evidence = dv.Evidence
		}
		if dv.Families != nil {
			d.Families = dv.Families
		}
		if dv.Settings != nil {
			d.Settings = dv.Settings
		}
	}
	return d
}

func exhibitContext(dataHash, currentHash string) *ExhibitContext {
	if currentHash == "" || dataHash == "" {
		return &ExhibitContext{}
	}
	same := dataHash == currentHash
	return &ExhibitContext{DataCurrent: &same}
}

// turnExport is what an exhibit of an answer is built from.
type turnExport struct {
	Turn        Turn
	AnswerText  string
	Query       string
	Figures     *Figures
	Data        *DataVersion
	CurrentHash string // the hash of the case's data now; empty when it was not read
}

func answerFigures(f *Figures, maxRows int) ([]Metric, *ExhibitTable) {
	if f == nil {
		return []Metric{}, nil
	}
	metrics := f.Metrics
	if metrics == nil {
		metrics = []Metric{}
	}
	return metrics, exhibitTable(f.Columns, f.Rows, f.RowCount, f.RowsDigest, maxRows)
}

// answerExhibit builds the exhibit of one answered turn.
func answerExhibit(x turnExport, maxRows int) Exhibit {
	t := x.Turn
	metrics, table := answerFigures(x.Figures, maxRows)
	headline := derefString(t.Headline)
	title := clip(headline, 120)
	if title == "" {
		title = clip(t.Question, 120)
	}
	asked := t.CreatedAt
	ex := Exhibit{
		ID: "turn:" + t.ID, Kind: "answer", CaseID: t.CaseID, Title: title, Question: t.Question,
		Answer:   &ExhibitAnswer{Outcome: derefString(t.Outcome), Headline: headline, Text: x.AnswerText, Shape: derefString(t.Shape), Figures: metrics, Table: table},
		Evidence: decodeEvidence(t.Evidence),
		Provenance: ExhibitProvenance{TurnID: t.ID, ThreadID: t.ThreadID, RunKind: t.Kind, AskedBy: t.CreatedBy, AskedAt: &asked, AnsweredAt: t.CompletedAt,
			EngineVersion: derefString(t.EngineVersion), Model: derefString(t.ModelID), PromptID: derefString(t.PromptID), Query: x.Query, QueryHash: derefString(t.QueryHash),
			ResultDigest: derefString(t.ResultDigest), SnapshotState: t.SnapshotState, DataVersionHash: derefString(t.DataVersionHash)},
		Data:      exhibitData(derefString(t.DataVersionHash), x.Data),
		Reproduce: &ExhibitReproduce{Rerun: "POST /turns/" + t.ID + "/rerun", Note: "Runs the question again on the data as it is now and compares the new answer with this one."},
		Context:   exhibitContext(derefString(t.DataVersionHash), x.CurrentHash),
	}
	return ex
}

// findingRow is a finding as the export reads it.
type findingRow struct {
	ID              string
	CaseID          string
	Title           string
	Body            string
	Basis           string
	ReviewState     string
	ReviewedBy      *string
	ReviewedAt      *time.Time
	ReviewNote      *string
	SourceTurnID    *string
	Pinned          json.RawMessage
	Evidence        json.RawMessage
	DataVersionHash *string
	CreatedBy       string
	CreatedAt       time.Time
}

// findingExhibit builds the exhibit of a finding: its own text and review, and the answer it was pinned from as it was when pinned.
func findingExhibit(f findingRow, dv *DataVersion, currentHash string, maxRows int) Exhibit {
	ex := Exhibit{
		ID: "finding:" + f.ID, Kind: "finding", CaseID: f.CaseID, Title: f.Title,
		Finding: &ExhibitFinding{FindingID: f.ID, Title: f.Title, Body: f.Body, Basis: f.Basis, ReviewState: f.ReviewState, ReviewedBy: f.ReviewedBy,
			ReviewedAt: utcPtr(f.ReviewedAt), ReviewNote: f.ReviewNote, CreatedBy: f.CreatedBy, CreatedAt: utc(f.CreatedAt)},
		Evidence: decodeEvidence(f.Evidence),
	}
	dataHash := derefString(f.DataVersionHash)
	var pinned struct {
		TurnID        string          `json:"turn_id"`
		ThreadID      string          `json:"thread_id"`
		Question      string          `json:"question"`
		Headline      string          `json:"headline"`
		Outcome       string          `json:"outcome"`
		Shape         string          `json:"shape"`
		ResultDigest  string          `json:"result_digest"`
		QueryHash     string          `json:"query_hash"`
		EngineVersion string          `json:"engine_version"`
		Model         string          `json:"model"`
		DataVersion   string          `json:"data_version_hash"`
		AnsweredAt    *time.Time      `json:"answered_at"`
		Figures       json.RawMessage `json:"figures"`
	}
	if len(f.Pinned) > 0 {
		_ = json.Unmarshal(f.Pinned, &pinned)
	}
	if f.SourceTurnID != nil && pinned.Outcome != "" {
		var figs *Figures
		if len(pinned.Figures) > 0 {
			dec := json.NewDecoder(bytes.NewReader(pinned.Figures))
			dec.UseNumber() // a long number in a cell keeps its digits
			var decoded Figures
			if dec.Decode(&decoded) == nil {
				figs = &decoded
			}
		}
		metrics, table := answerFigures(figs, maxRows)
		ex.Question = pinned.Question
		ex.Answer = &ExhibitAnswer{Outcome: pinned.Outcome, Headline: pinned.Headline, Shape: pinned.Shape, Figures: metrics, Table: table}
		ex.Provenance = ExhibitProvenance{TurnID: pinned.TurnID, ThreadID: pinned.ThreadID, AnsweredAt: pinned.AnsweredAt, EngineVersion: pinned.EngineVersion, Model: pinned.Model,
			QueryHash: pinned.QueryHash, ResultDigest: pinned.ResultDigest, DataVersionHash: pinned.DataVersion}
		if dataHash == "" {
			dataHash = pinned.DataVersion
		}
	}
	if f.SourceTurnID != nil {
		ex.Reproduce = &ExhibitReproduce{Rerun: "POST /turns/" + *f.SourceTurnID + "/rerun", Note: "Runs the question the finding was made from again on the data as it is now."}
	}
	ex.Data = exhibitData(dataHash, dv)
	ex.Context = exhibitContext(dataHash, currentHash)
	return ex
}

// selectDefaultTurns picks, from the turns of a conversation in the order they were asked, the latest answered run of each question (a re-run replaces the run it re-ran; an edited question is
// another question) and lists the questions that have no answered run, with the reason. The questions come in the order they were first asked.
func selectDefaultTurns(turns []Turn) ([]Turn, []Skipped) {
	var order []string
	byRoot := map[string][]Turn{}
	for _, t := range turns {
		if _, ok := byRoot[t.RootID]; !ok {
			order = append(order, t.RootID)
		}
		byRoot[t.RootID] = append(byRoot[t.RootID], t)
	}
	var selected []Turn
	skipped := []Skipped{}
	for _, root := range order {
		runs := byRoot[root]
		var best *Turn
		for i := range runs {
			if isAnswer(runs[i]) && (best == nil || runs[i].Index > best.Index) {
				best = &runs[i]
			}
		}
		if best != nil {
			selected = append(selected, *best)
			continue
		}
		skipped = append(skipped, notExported(runs[len(runs)-1]))
	}
	return selected, skipped
}

func isAnswer(t Turn) bool {
	return t.State == "completed" && t.Outcome != nil && *t.Outcome == OutcomeAnswered
}

func notExported(t Turn) Skipped {
	return Skipped{Ref: "turn:" + t.ID, Reason: fmt.Sprintf("The latest run of this question is %s, not an answer.", describeTurnState(t))}
}

// ---- reading what an export needs ---------------------------------------------------------------------------------------------------------------------------------------

// loadTurnExport reads what an exhibit of an answer needs besides the turn: the text of the answer, the query, the figures and the stored data version.
func loadTurnExport(ctx context.Context, tx pgx.Tx, sc Scope, t Turn, current map[string]string) (turnExport, error) {
	x := turnExport{Turn: t}
	var answer, query *string
	var figures []byte
	if err := tx.QueryRow(ctx, `SELECT answer_text, query_text, figures_json FROM forensic.conversation_turn WHERE tenant_id = $1 AND turn_id = $2::uuid`, sc.TenantID, t.ID).Scan(&answer, &query, &figures); err != nil {
		return x, err
	}
	x.AnswerText, x.Query = derefString(answer), derefString(query)
	if len(figures) > 0 {
		dec := json.NewDecoder(bytes.NewReader(figures))
		dec.UseNumber() // a long number in a cell keeps its digits
		var f Figures
		if err := dec.Decode(&f); err == nil {
			x.Figures = &f
		}
	}
	var err error
	if x.Data, err = loadDataVersion(ctx, tx, sc.TenantID, t.CaseID, derefString(t.DataVersionHash)); err != nil {
		return x, err
	}
	if x.CurrentHash, err = currentDataHash(ctx, tx, sc, t.CaseID, current); err != nil {
		return x, err
	}
	return x, nil
}

// currentDataHash is the hash of what a case holds now, read once per case for an export.
func currentDataHash(ctx context.Context, tx pgx.Tx, sc Scope, caseID string, cache map[string]string) (string, error) {
	if h, ok := cache[caseID]; ok {
		return h, nil
	}
	v, err := readDataVersion(ctx, tx, sc.TenantID, caseID)
	if err != nil {
		return "", err
	}
	cache[caseID] = v.Hash()
	return cache[caseID], nil
}

func loadFindingRow(ctx context.Context, tx pgx.Tx, sc Scope, caseID, id string) (findingRow, error) {
	var f findingRow
	var source *string
	var pinned, evidence []byte
	err := tx.QueryRow(ctx, `SELECT finding_id::text, collection_id, title, body, basis, review_state, reviewed_by, reviewed_at, review_note, source_turn_id::text, pinned_json, evidence_refs, data_version_hash, created_by, created_at
		FROM forensic.finding WHERE tenant_id = $1 AND collection_id = $2 AND finding_id = $3::uuid AND deleted_at IS NULL`, sc.TenantID, caseID, id).
		Scan(&f.ID, &f.CaseID, &f.Title, &f.Body, &f.Basis, &f.ReviewState, &f.ReviewedBy, &f.ReviewedAt, &f.ReviewNote, &source, &pinned, &evidence, &f.DataVersionHash, &f.CreatedBy, &f.CreatedAt)
	if err == pgx.ErrNoRows {
		return f, NotFound("There is no such finding in this case, or you may not see it.")
	}
	f.SourceTurnID, f.Pinned, f.Evidence = source, pinned, evidence
	return f, err
}

// ---- the export calls -----------------------------------------------------------------------------------------------------------------------------------------------------

func clampRows(n int) int {
	switch {
	case n <= 0:
		return defaultExportRows
	case n > maxExportRows:
		return maxExportRows
	}
	return n
}

func cleanIDs(name string, ids []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if err := checkUUID(name, id); err != nil {
			return nil, err
		}
		id = strings.ToLower(id)
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if len(out) > maxExportItems {
		return nil, BadRequest("At most %d items can be exported at once.", maxExportItems)
	}
	return out, nil
}

func requireExportRoles(t Thread) error {
	for _, c := range t.CaseIDs {
		if r := t.roles[c]; !r.CanExport() {
			if t.CrossCase {
				return Forbidden("Your role in case %q (%s) does not allow exporting answers.", c, r)
			}
			return Forbidden("Your role in this case (%s) does not allow exporting answers.", r)
		}
	}
	return nil
}

// finishBundle numbers nothing and stores nothing: it hashes the exhibits, the bundle, writes the Markdown.
func finishBundle(b *Bundle) error {
	hashes := make([]string, len(b.Exhibits))
	for i := range b.Exhibits {
		h, err := b.Exhibits[i].ComputeHash()
		if err != nil {
			return err
		}
		b.Exhibits[i].SHA256 = h
		hashes[i] = h
	}
	b.BundleSHA256 = bundleHash(b.CaseIDs, hashes)
	b.Markdown = renderMarkdown(b)
	return nil
}

// ExportThread exports the answers of a conversation. By default it takes the latest answered run of each question, in the order they were asked; with opt.TurnIDs, the answers named. What was
// left out is listed with the reason. The caller needs a role that allows exporting in every case the conversation covers; the export is written to the audit trail of each of them.
func (s *Service) ExportThread(ctx context.Context, sc Scope, threadID string, opt ExportOptions) (*Bundle, error) {
	if err := checkUUID("The conversation id", threadID); err != nil {
		return nil, err
	}
	want, err := cleanIDs("turn_ids", opt.TurnIDs)
	if err != nil {
		return nil, err
	}
	rows := clampRows(opt.MaxRows)
	title := ""
	if strings.TrimSpace(opt.Title) != "" {
		if title, err = cleanLine("The title", opt.Title, 1, 200); err != nil {
			return nil, err
		}
	}
	var out *Bundle
	err = s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		t, _, err := loadThread(ctx, tx, sc, threadID)
		if err != nil {
			return err
		}
		if t.Hidden {
			return Conflict("hidden", "This conversation is hidden; restore it first.")
		}
		if err := requireExportRoles(t); err != nil {
			return err
		}
		turns, err := loadTurns(ctx, tx, sc, `t.thread_id = $2::uuid`, []any{threadID}, "t.idx", 0)
		if err != nil {
			return err
		}
		var selected []Turn
		skipped := []Skipped{}
		if len(want) == 0 {
			selected, skipped = selectDefaultTurns(turns)
		} else {
			byID := map[string]Turn{}
			for _, tn := range turns {
				byID[tn.ID] = tn
			}
			for _, id := range want {
				tn, ok := byID[id]
				if !ok {
					return NotFound("There is no such answer in this conversation, or you may not see it.")
				}
				if !isAnswer(tn) {
					skipped = append(skipped, notExported(tn))
					continue
				}
				selected = append(selected, tn)
			}
		}
		if len(selected) == 0 {
			return Conflict("nothing_to_export", "This conversation has no answer to export yet%s.", skippedHint(skipped))
		}
		if title == "" {
			title = t.DisplayTitle
		}
		current := map[string]string{}
		b := &Bundle{Format: ExportFormat, Title: title, ThreadID: threadID, ExportedBy: sc.Subject, ExportedAt: time.Now().UTC(), Skipped: skipped}
		for _, tn := range selected {
			x, err := loadTurnExport(ctx, tx, sc, tn, current)
			if err != nil {
				return err
			}
			b.Exhibits = append(b.Exhibits, answerExhibit(x, rows))
		}
		b.CaseIDs = t.CaseIDs
		if err := finishBundle(b); err != nil {
			return err
		}
		for _, c := range t.CaseIDs {
			n := 0
			for _, e := range b.Exhibits {
				if e.CaseID == c {
					n++
				}
			}
			details := map[string]any{"bundle_sha256": b.BundleSHA256, "format": ExportFormat, "exhibits": n, "skipped": len(skipped)}
			if t.CrossCase {
				details["cross_case"], details["cases"] = true, len(t.CaseIDs)
			}
			if err := audit(ctx, tx, sc, t.roles[c], c, "thread.exported", "thread", threadID, details); err != nil {
				return err
			}
		}
		out = b
		return nil
	})
	return out, err
}

func skippedHint(skipped []Skipped) string {
	if len(skipped) == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d question(s) were not answered; see skipped)", len(skipped))
}

// ExportExhibits exports findings and answers of ONE case, in the order asked for (findings first). A rejected finding is not exported; with opt.OnlyConfirmed an unreviewed one is not either. An
// answer must be of this case and in a conversation the caller may read. The export is written to the audit trail of the case.
func (s *Service) ExportExhibits(ctx context.Context, sc Scope, caseID string, opt ExportOptions) (*Bundle, error) {
	caseID = strings.TrimSpace(caseID)
	if caseID == "" {
		return nil, BadRequest("case_id is required.")
	}
	findings, err := cleanIDs("finding_ids", opt.FindingIDs)
	if err != nil {
		return nil, err
	}
	answers, err := cleanIDs("turn_ids", opt.TurnIDs)
	if err != nil {
		return nil, err
	}
	if len(findings)+len(answers) == 0 {
		return nil, BadRequest("Name what to export: finding_ids, turn_ids or both.")
	}
	if len(findings)+len(answers) > maxExportItems {
		return nil, BadRequest("At most %d items can be exported at once.", maxExportItems)
	}
	rows := clampRows(opt.MaxRows)
	title := ""
	if strings.TrimSpace(opt.Title) != "" {
		if title, err = cleanLine("The title", opt.Title, 1, 200); err != nil {
			return nil, err
		}
	}
	var out *Bundle
	err = s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		role, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if !role.CanExport() {
			return Forbidden("Your role in this case (%s) does not allow exporting.", role)
		}
		current := map[string]string{}
		b := &Bundle{Format: ExportFormat, Title: title, CaseIDs: []string{caseID}, ExportedBy: sc.Subject, ExportedAt: time.Now().UTC(), Skipped: []Skipped{}}
		if b.Title == "" {
			b.Title = "Exhibits of case " + caseID
		}
		for _, id := range findings {
			f, err := loadFindingRow(ctx, tx, sc, caseID, id)
			if err != nil {
				return err
			}
			switch {
			case f.ReviewState == "rejected":
				b.Skipped = append(b.Skipped, Skipped{Ref: "finding:" + id, Reason: "The finding was rejected on review."})
				continue
			case opt.OnlyConfirmed && f.ReviewState != "confirmed":
				b.Skipped = append(b.Skipped, Skipped{Ref: "finding:" + id, Reason: "The finding is not confirmed yet (only confirmed findings were asked for)."})
				continue
			}
			dv, err := loadDataVersion(ctx, tx, sc.TenantID, caseID, derefString(f.DataVersionHash))
			if err != nil {
				return err
			}
			cur, err := currentDataHash(ctx, tx, sc, caseID, current)
			if err != nil {
				return err
			}
			b.Exhibits = append(b.Exhibits, findingExhibit(f, dv, cur, rows))
		}
		for _, id := range answers {
			turns, err := loadTurns(ctx, tx, sc, `t.turn_id = $2::uuid`, []any{id}, "t.idx", 1)
			if err != nil {
				return err
			}
			if len(turns) == 0 || turns[0].CaseID != caseID {
				return NotFound("There is no such answer in this case, or you may not see it.")
			}
			if _, _, err := loadThread(ctx, tx, sc, turns[0].ThreadID); err != nil { // a conversation the caller may not read does not give up its answers
				return err
			}
			if !isAnswer(turns[0]) {
				b.Skipped = append(b.Skipped, notExported(turns[0]))
				continue
			}
			x, err := loadTurnExport(ctx, tx, sc, turns[0], current)
			if err != nil {
				return err
			}
			b.Exhibits = append(b.Exhibits, answerExhibit(x, rows))
		}
		if len(b.Exhibits) == 0 {
			return Conflict("nothing_to_export", "None of what was asked for can be exported%s.", skippedHint(b.Skipped))
		}
		if err := finishBundle(b); err != nil {
			return err
		}
		details := map[string]any{"bundle_sha256": b.BundleSHA256, "format": ExportFormat, "exhibits": len(b.Exhibits), "skipped": len(b.Skipped), "findings": len(findings), "answers": len(answers)}
		if err := audit(ctx, tx, sc, role, caseID, "exhibits.exported", "case", caseID, details); err != nil {
			return err
		}
		out = b
		return nil
	})
	return out, err
}

// ---- Markdown ---------------------------------------------------------------------------------------------------------------------------------------------------------------------

// mdText makes a text safe to put in a line of Markdown that may be rendered as HTML: markup is written as text, line breaks become spaces.
func mdText(s string) string {
	s = strings.Join(strings.Fields(strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ")), " ")
	return strings.NewReplacer("<", "&lt;", ">", "&gt;").Replace(s)
}

// mdCell is mdText for a cell of a table: a pipe stays in its cell.
func mdCell(s string) string {
	return strings.ReplaceAll(mdText(s), "|", `\|`)
}

func mdBlock(s string) string {
	// a body keeps its lines: each is its own line of a quote, markup written as text
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(s), "\r", ""), "\n")
	for i, l := range lines {
		lines[i] = "> " + strings.NewReplacer("<", "&lt;", ">", "&gt;").Replace(l)
	}
	return strings.Join(lines, "\n")
}

// renderMarkdown writes a bundle as a document: a title, the hash of the bundle, and for each exhibit its question, its sentence, its table, the files it cites, how it was produced and its hash.
func renderMarkdown(b *Bundle) string {
	var w strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&w, format+"\n", args...) }
	line("# %s", mdText(b.Title))
	line("")
	line("Exported by %s at %s. %d exhibit(s) from %s.", mdText(b.ExportedBy), b.ExportedAt.UTC().Format(time.RFC3339), len(b.Exhibits), mdText(strings.Join(b.CaseIDs, ", ")))
	line("")
	line("Format: %s. Bundle SHA-256: %s", b.Format, b.BundleSHA256)
	for i, e := range b.Exhibits {
		line("")
		line("## Exhibit %d. %s", i+1, mdText(e.Title))
		line("")
		line("Case: %s. Kind: %s.", mdText(e.CaseID), e.Kind)
		if e.Finding != nil {
			f := e.Finding
			line("")
			line("Basis: %s", f.Basis)
			switch {
			case f.ReviewState == "unreviewed" || f.ReviewedBy == nil:
				line("Review: not yet reviewed")
			default:
				at := ""
				if f.ReviewedAt != nil {
					at = " on " + f.ReviewedAt.UTC().Format("2006-01-02")
				}
				line("Review: %s by %s%s", f.ReviewState, mdText(*f.ReviewedBy), at)
				if f.ReviewNote != nil && *f.ReviewNote != "" {
					line("")
					line("%s", mdBlock(*f.ReviewNote))
				}
			}
			if strings.TrimSpace(f.Body) != "" {
				line("")
				line("%s", mdBlock(f.Body))
			}
		}
		if e.Question != "" {
			line("")
			line("**Question.**")
			line("")
			line("%s", mdBlock(e.Question))
		}
		if a := e.Answer; a != nil {
			line("")
			line("**Answer** (%s). %s", mdText(a.Outcome), mdText(a.Headline))
			if a.Text != "" && a.Text != a.Headline {
				line("")
				line("%s", mdBlock(a.Text))
			}
			for _, m := range a.Figures {
				line("")
				line("- %s: %s", mdText(m.Label), mdText(m.Value))
			}
			if t := a.Table; t != nil {
				line("")
				heads := make([]string, len(t.Columns))
				rule := make([]string, len(t.Columns))
				for j, c := range t.Columns {
					heads[j], rule[j] = mdCell(c.Header), "---"
				}
				line("| %s |", strings.Join(heads, " | "))
				line("| %s |", strings.Join(rule, " | "))
				for _, row := range t.Rows {
					cells := make([]string, len(t.Columns))
					for j := range cells {
						if j < len(row) {
							cells[j] = mdCell(cellText(row[j]))
						}
					}
					line("| %s |", strings.Join(cells, " | "))
				}
				if t.Truncated {
					line("")
					line("Showing %d of %d rows.", t.RowsShown, t.RowCount)
				}
			}
		}
		if len(e.Evidence) > 0 {
			line("")
			line("**Files cited.**")
			for _, ev := range e.Evidence {
				name := ev.SourceFile
				if name == "" {
					name = ev.EvidenceID
				}
				line("")
				line("- %s (evidence %s)", mdText(name), ev.EvidenceID)
			}
		}
		line("")
		line("**How it was produced.**")
		p := e.Provenance
		for _, kv := range [][2]string{{"Asked by", p.AskedBy}, {"Engine", p.EngineVersion}, {"Model", p.Model}, {"Prompt", p.PromptID}, {"Query hash", p.QueryHash},
			{"Result digest", p.ResultDigest}, {"Data version", p.DataVersionHash}} {
			if kv[1] != "" {
				line("")
				line("%s: %s", kv[0], mdText(kv[1]))
			}
		}
		if p.AnsweredAt != nil {
			line("")
			line("Answered at: %s", p.AnsweredAt.UTC().Format(time.RFC3339))
		}
		if e.Context != nil && e.Context.DataCurrent != nil && !*e.Context.DataCurrent {
			line("")
			line("The case's data has changed since this answer was computed.")
		}
		line("")
		line("SHA-256 of this exhibit: %s", e.SHA256)
	}
	if len(b.Skipped) > 0 {
		line("")
		line("## Not exported")
		for _, s := range b.Skipped {
			line("")
			line("- %s: %s", mdText(s.Ref), mdText(s.Reason))
		}
	}
	return w.String()
}
