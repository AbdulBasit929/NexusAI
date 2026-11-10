package caselayer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// The knowledge objects of a case (notes, hypotheses, tasks, saved views, annotations, authority records and the plain part of findings) share one shape: create, list, read, patch, hide. They are
// described by a spec and served by one engine, so that the rules (who may write, what is validated, what is audited) are the same for all of them and are read in one place. The result is returned as the
// row's own JSON with the tenant removed and collection_id renamed case_id.

type fieldKind int

const (
	fText fieldKind = iota
	fBody
	fEnum
	fBool
	fInt
	fTime
	fDate
	fUUID
	fUUIDs
	fStrings
	fJSONObject
	fJSONArray
)

type field struct {
	Name     string
	Kind     fieldKind
	Min, Max int // characters of a text, the range of an integer, the bytes of a JSON value
	Enum     []string
	Required bool
	Create   bool // accepted when the object is created
	Patch    bool // accepted when it is changed
	Nullable bool // null clears the value
	Default  any
	Pattern  *regexp.Regexp
}

func (f field) cast() string {
	switch f.Kind {
	case fTime:
		return "::timestamptz"
	case fDate:
		return "::date"
	case fUUID:
		return "::uuid"
	case fUUIDs:
		return "::uuid[]"
	case fStrings:
		return "::text[]"
	case fJSONObject, fJSONArray:
		return "::jsonb"
	}
	return ""
}

type filterSpec struct {
	Param  string
	Column string
	Kind   fieldKind // fEnum, fBool or fText (equal), fUUID
}

type objectSpec struct {
	Kind       string // note
	Plural     string // notes
	Table      string // case_note
	IDCol      string // note_id
	TagKind    string // the object_kind of its tags ("" if it has none)
	AuthorCol  string // created_by, recorded_by or owner_subject
	OwnerOnly  bool   // only the author (or an admin) may change or hide it
	SoftDelete bool
	ManageOnly bool // writes need a role that may manage the case
	Fields     []field
	SearchCols []string
	Filters    []filterSpec
	OrderBy    string
	ExtraJSON  string // extra pairs for jsonb_build_object, for computed fields
	// Complete returns extra assignments for a patch or a create given the values being set (for example the completion time of a task).
	Complete func(vals map[string]any, subject string, args *[]any) []string
	Validate func(vals map[string]any) error
}

func (o objectSpec) field(name string) (field, bool) {
	for _, f := range o.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return field{}, false
}

// parsed is one validated field value ready to be bound.
type parsed struct {
	name string
	val  any
	cast string
}

func decodeField(f field, raw json.RawMessage) (any, error) {
	if string(raw) == "null" {
		if f.Nullable {
			return nil, nil
		}
		return nil, BadRequest("%s cannot be null.", f.Name)
	}
	switch f.Kind {
	case fText, fBody:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, BadRequest("%s must be text.", f.Name)
		}
		var err error
		if f.Kind == fText {
			s, err = cleanLine(f.Name, s, max(f.Min, 1), f.Max)
		} else {
			if f.Min == 0 && strings.TrimSpace(s) == "" {
				return "", nil
			}
			s, err = cleanBody(f.Name, s, max(f.Min, 1), f.Max)
		}
		if err != nil {
			return nil, err
		}
		if f.Pattern != nil && !f.Pattern.MatchString(s) {
			return nil, BadRequest("%s is not in the form allowed.", f.Name)
		}
		return s, nil
	case fEnum:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, BadRequest("%s must be text.", f.Name)
		}
		for _, e := range f.Enum {
			if e == s {
				return s, nil
			}
		}
		return nil, BadRequest("%s must be one of %s.", f.Name, strings.Join(f.Enum, ", "))
	case fBool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, BadRequest("%s must be true or false.", f.Name)
		}
		return b, nil
	case fInt:
		var n int
		if err := json.Unmarshal(raw, &n); err != nil || n < f.Min || n > f.Max {
			return nil, BadRequest("%s must be a whole number from %d to %d.", f.Name, f.Min, f.Max)
		}
		return n, nil
	case fTime:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, BadRequest("%s must be a time such as 2026-11-03T09:30:00Z.", f.Name)
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, BadRequest("%s must be a time such as 2026-11-03T09:30:00Z.", f.Name)
		}
		return t.UTC().Format(time.RFC3339Nano), nil
	case fDate:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, BadRequest("%s must be a date such as 2026-11-03.", f.Name)
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return nil, BadRequest("%s must be a date such as 2026-11-03.", f.Name)
		}
		return s, nil
	case fUUID:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil || checkUUID(f.Name, s) != nil {
			return nil, BadRequest("%s is not a valid identifier.", f.Name)
		}
		return strings.ToLower(s), nil
	case fUUIDs:
		var list []string
		if err := json.Unmarshal(raw, &list); err != nil || len(list) > 100 {
			return nil, BadRequest("%s must be a list of at most 100 identifiers.", f.Name)
		}
		for i := range list {
			if checkUUID(f.Name, list[i]) != nil {
				return nil, BadRequest("%s holds an identifier that is not valid.", f.Name)
			}
			list[i] = strings.ToLower(list[i])
		}
		if list == nil {
			list = []string{}
		}
		return list, nil
	case fStrings:
		var list []string
		if err := json.Unmarshal(raw, &list); err != nil || len(list) > 100 {
			return nil, BadRequest("%s must be a list of at most 100 texts.", f.Name)
		}
		if list == nil {
			list = []string{}
		}
		return list, nil
	case fJSONObject, fJSONArray:
		if f.Max > 0 && len(raw) > f.Max {
			return nil, BadRequest("%s is larger than %d bytes.", f.Name, f.Max)
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, BadRequest("%s must be JSON.", f.Name)
		}
		_, isObj := v.(map[string]any)
		_, isArr := v.([]any)
		if (f.Kind == fJSONObject && !isObj) || (f.Kind == fJSONArray && !isArr) {
			return nil, BadRequest("%s must be a JSON %s.", f.Name, map[bool]string{true: "object", false: "list"}[f.Kind == fJSONObject])
		}
		return string(raw), nil
	}
	return nil, BadRequest("%s is not supported.", f.Name)
}

// parseBody validates a request body against the spec. On create every Required field must be present and defaults fill the rest.
func (o objectSpec) parseBody(body map[string]json.RawMessage, creating bool) ([]parsed, map[string]any, error) {
	var out []parsed
	vals := map[string]any{}
	names := make([]string, 0, len(body))
	for k := range body {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		f, ok := o.field(k)
		if !ok {
			return nil, nil, BadRequest("%q is not a field of a %s.", k, o.Kind)
		}
		if creating && !f.Create {
			return nil, nil, BadRequest("%s cannot be set when a %s is created.", k, o.Kind)
		}
		if !creating && !f.Patch {
			return nil, nil, BadRequest("%s cannot be changed.", k)
		}
		v, err := decodeField(f, body[k])
		if err != nil {
			return nil, nil, err
		}
		out = append(out, parsed{name: k, val: v, cast: f.cast()})
		vals[k] = v
	}
	if creating {
		for _, f := range o.Fields {
			if _, present := body[f.Name]; present {
				continue
			}
			if f.Required {
				return nil, nil, BadRequest("%s is required.", f.Name)
			}
			if f.Default != nil && f.Create {
				out = append(out, parsed{name: f.Name, val: f.Default, cast: f.cast()})
				vals[f.Name] = f.Default
			}
		}
	} else if len(out) == 0 {
		return nil, nil, BadRequest("Nothing to change.")
	}
	if o.Validate != nil {
		if err := o.Validate(vals); err != nil {
			return nil, nil, err
		}
	}
	return out, vals, nil
}

func (o objectSpec) jsonSelect(alias string) string {
	extra := ""
	if o.ExtraJSON != "" {
		extra = ", " + o.ExtraJSON
	}
	tags := ""
	if o.TagKind != "" {
		tags = fmt.Sprintf(`, 'tags', coalesce((SELECT jsonb_agg(g.name ORDER BY g.name_key) FROM forensic.tag_assignment a JOIN forensic.tag g ON g.tag_id = a.tag_id
			WHERE a.object_kind = '%s' AND a.object_id = %s.%s), '[]'::jsonb)`, o.TagKind, alias, o.IDCol)
	}
	return fmt.Sprintf(`(to_jsonb(%s) - 'tenant_id' - 'collection_id' - 'deleted_at' - 'deleted_by' - 'delete_reason') || jsonb_build_object('case_id', %s.collection_id%s%s)`, alias, alias, tags, extra)
}

func (o objectSpec) alive(alias string) string {
	switch {
	case !o.SoftDelete:
		return ""
	case alias == "":
		return " AND deleted_at IS NULL"
	default:
		return " AND " + alias + ".deleted_at IS NULL"
	}
}

// objectCursor pages a list by offset: these lists are a case's notes, findings and the like, a few thousand at most.
type objectCursor struct {
	Offset int `json:"o"`
}

// ListObjects lists the objects of a case, newest first (or in the order of the spec), with the filters the spec names, a word search and a tag filter.
func (s *Service) ListObjects(ctx context.Context, sc Scope, o objectSpec, caseID string, q url.Values) (Page[json.RawMessage], error) {
	page := Page[json.RawMessage]{Items: []json.RawMessage{}}
	limit := clampLimit(atoiDefault(q.Get("limit"), 0))
	offset := 0
	if c := q.Get("cursor"); c != "" {
		var cur objectCursor
		if err := decodeCursor(c, &cur); err != nil {
			return page, err
		}
		offset = cur.Offset
	}
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := requireRole(ctx, tx, sc, caseID); err != nil {
			return err
		}
		args := []any{sc.TenantID, caseID}
		arg := func(v any) string { args = append(args, v); return "$" + strconv.Itoa(len(args)) }
		where := []string{"x.tenant_id = $1", "x.collection_id = $2"}
		if o.SoftDelete && q.Get("include_deleted") != "true" {
			where = append(where, "x.deleted_at IS NULL")
		}
		for _, fl := range o.Filters {
			v := strings.TrimSpace(q.Get(fl.Param))
			if v == "" {
				continue
			}
			switch fl.Kind {
			case fBool:
				where = append(where, fmt.Sprintf("x.%s = %s", fl.Column, arg(v == "true")))
			case fUUID:
				if checkUUID(fl.Param, v) != nil {
					return BadRequest("%s is not a valid identifier.", fl.Param)
				}
				where = append(where, fmt.Sprintf("x.%s = %s::uuid", fl.Column, arg(v)))
			default:
				where = append(where, fmt.Sprintf("x.%s = %s", fl.Column, arg(v)))
			}
		}
		if tag := strings.TrimSpace(q.Get("tag")); tag != "" && o.TagKind != "" {
			where = append(where, fmt.Sprintf(`EXISTS (SELECT 1 FROM forensic.tag_assignment a JOIN forensic.tag g ON g.tag_id = a.tag_id
				WHERE a.object_kind = '%s' AND a.object_id = x.%s AND g.name_key = forensic.search_normalize(btrim(%s)))`, o.TagKind, o.IDCol, arg(tag)))
		}
		if words := searchWords(q.Get("q")); len(words) > 0 && len(o.SearchCols) > 0 {
			cols := make([]string, len(o.SearchCols))
			for i, c := range o.SearchCols {
				cols[i] = "coalesce(x." + c + "::text, '')"
			}
			hay := "forensic.search_normalize(concat_ws(' ', " + strings.Join(cols, ", ") + "))"
			for _, w := range words {
				where = append(where, hay+" LIKE '%' || forensic.search_normalize("+arg(w)+") || '%'")
			}
		}
		order := o.OrderBy
		if order == "" {
			order = "x.updated_at DESC, x." + o.IDCol + " DESC"
		}
		query := fmt.Sprintf(`SELECT %s FROM forensic.%s x WHERE %s ORDER BY %s LIMIT %d OFFSET %d`, o.jsonSelect("x"), o.Table, strings.Join(where, " AND "), order, limit+1, offset)
		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				return err
			}
			page.Items = append(page.Items, json.RawMessage(raw))
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(page.Items) > limit {
			page.Items = page.Items[:limit]
			page.NextCursor = encodeCursor(objectCursor{Offset: offset + limit})
		}
		return nil
	})
	return page, err
}

func atoiDefault(s string, d int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return d
	}
	return n
}

// readObject reads one object by id for the given tenant; it also returns the case it belongs to and the author column's value.
func (o objectSpec) readObject(ctx context.Context, tx pgx.Tx, sc Scope, id string) (json.RawMessage, string, string, error) {
	if err := checkUUID("The identifier", id); err != nil {
		return nil, "", "", err
	}
	var raw []byte
	var caseID, author string
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT %s, x.collection_id, x.%s FROM forensic.%s x WHERE x.tenant_id = $1 AND x.%s = $2::uuid%s`,
		o.jsonSelect("x"), o.AuthorCol, o.Table, o.IDCol, o.alive("x")), sc.TenantID, id).Scan(&raw, &caseID, &author)
	if err != nil {
		return nil, "", "", asError(err)
	}
	return raw, caseID, author, nil
}

// GetObject reads one object.
func (s *Service) GetObject(ctx context.Context, sc Scope, o objectSpec, id string) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		raw, caseID, _, err := o.readObject(ctx, tx, sc, id)
		if err != nil {
			return err
		}
		if _, err := requireRole(ctx, tx, sc, caseID); err != nil {
			return err
		}
		out = raw
		return nil
	})
	return out, err
}

func (o objectSpec) writerRole(role Role) error {
	if o.ManageOnly {
		if !role.CanManage() {
			return Forbidden("Your role in this case (%s) does not allow changing %s.", role, o.Plural)
		}
		return nil
	}
	if !role.CanContribute() {
		return Forbidden("Your role in this case (%s) does not allow changing %s.", role, o.Plural)
	}
	return nil
}

// CreateObject creates an object in a case from a request body.
func (s *Service) CreateObject(ctx context.Context, sc Scope, o objectSpec, caseID string, body map[string]json.RawMessage) (json.RawMessage, error) {
	cols, vals, err := o.parseBody(body, true)
	if err != nil {
		return nil, err
	}
	var out json.RawMessage
	err = s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		role, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if err := o.writerRole(role); err != nil {
			return err
		}
		known, err := caseKnown(ctx, tx, sc.TenantID, caseID)
		if err != nil {
			return err
		}
		if !known {
			return NotFound("There is no such case.")
		}
		names := []string{"tenant_id", "collection_id", o.AuthorCol}
		args := []any{sc.TenantID, caseID, sc.Subject}
		holders := []string{"$1", "$2", "$3"}
		for _, c := range cols {
			args = append(args, c.val)
			names = append(names, c.name)
			holders = append(holders, "$"+strconv.Itoa(len(args))+c.cast)
		}
		if o.Complete != nil {
			for _, extra := range o.Complete(vals, sc.Subject, &args) {
				parts := strings.SplitN(extra, "=", 2)
				names = append(names, strings.TrimSpace(parts[0]))
				holders = append(holders, strings.TrimSpace(parts[1]))
			}
		}
		var id string
		if err := tx.QueryRow(ctx, fmt.Sprintf(`INSERT INTO forensic.%s (%s) VALUES (%s) RETURNING %s::text`, o.Table, strings.Join(names, ", "), strings.Join(holders, ", "), o.IDCol), args...).Scan(&id); err != nil {
			return err
		}
		fieldNames := make([]string, 0, len(cols))
		for _, c := range cols {
			fieldNames = append(fieldNames, c.name)
		}
		if err := audit(ctx, tx, sc, role, caseID, o.Kind+".created", o.Kind, id, map[string]any{"fields": fieldNames}); err != nil {
			return err
		}
		out, _, _, err = o.readObject(ctx, tx, sc, id)
		return err
	})
	return out, err
}

// PatchObject changes the named fields of an object.
func (s *Service) PatchObject(ctx context.Context, sc Scope, o objectSpec, id string, body map[string]json.RawMessage) (json.RawMessage, error) {
	cols, vals, err := o.parseBody(body, false)
	if err != nil {
		return nil, err
	}
	var out json.RawMessage
	err = s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		_, caseID, author, err := o.readObject(ctx, tx, sc, id)
		if err != nil {
			return err
		}
		role, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if err := o.writerRole(role); err != nil {
			return err
		}
		if o.OwnerOnly && author != sc.Subject && !sc.IsAdmin() {
			return Forbidden("Only the person who made this %s can change it.", o.Kind)
		}
		args := []any{sc.TenantID, id}
		var sets []string
		names := []string{}
		for _, c := range cols {
			args = append(args, c.val)
			sets = append(sets, c.name+" = $"+strconv.Itoa(len(args))+c.cast)
			names = append(names, c.name)
		}
		if o.Complete != nil {
			sets = append(sets, o.Complete(vals, sc.Subject, &args)...)
		}
		args = append(args, sc.Subject)
		sets = append(sets, "updated_by = $"+strconv.Itoa(len(args)), "updated_at = now()")
		tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE forensic.%s SET %s WHERE tenant_id = $1 AND %s = $2::uuid%s`, o.Table, strings.Join(sets, ", "), o.IDCol, o.alive("")), args...)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return NotFound("There is no such %s, or you may not change it.", o.Kind)
		}
		if err := audit(ctx, tx, sc, role, caseID, o.Kind+".updated", o.Kind, id, map[string]any{"fields": names}); err != nil {
			return err
		}
		out, _, _, err = o.readObject(ctx, tx, sc, id)
		return err
	})
	return out, err
}

// DeleteObject hides an object (it is not removed); the audit row keeps who and why.
func (s *Service) DeleteObject(ctx context.Context, sc Scope, o objectSpec, id, reason string) error {
	if !o.SoftDelete {
		return BadRequest("A %s cannot be deleted.", o.Kind)
	}
	reason = strings.TrimSpace(reason)
	if reason != "" {
		var err error
		if reason, err = cleanLine("The reason", reason, 1, 500); err != nil {
			return err
		}
	}
	return s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		_, caseID, author, err := o.readObject(ctx, tx, sc, id)
		if err != nil {
			return err
		}
		role, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if err := o.writerRole(role); err != nil {
			return err
		}
		if o.OwnerOnly && author != sc.Subject && !sc.IsAdmin() {
			return Forbidden("Only the person who made this %s can delete it.", o.Kind)
		}
		if err := audit(ctx, tx, sc, role, caseID, o.Kind+".deleted", o.Kind, id, map[string]any{"reason": reason}); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE forensic.%s SET deleted_at = now(), deleted_by = $3, delete_reason = nullif($4, ''), updated_by = $3, updated_at = now()
			WHERE tenant_id = $1 AND %s = $2::uuid AND deleted_at IS NULL`, o.Table, o.IDCol), sc.TenantID, id, sc.Subject, reason)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return NotFound("There is no such %s, or you may not delete it.", o.Kind)
		}
		return nil
	})
}
