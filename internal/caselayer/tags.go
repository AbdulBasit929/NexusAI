package caselayer

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// taggable is where each kind of object that can carry a tag lives: its table and its identifier column. Tags are the case's own labels; one table serves them all.
var taggable = map[string]struct{ table, idCol string }{
	"thread":     {"conversation_thread", "thread_id"},
	"turn":       {"conversation_turn", "turn_id"},
	"note":       {"case_note", "note_id"},
	"finding":    {"finding", "finding_id"},
	"hypothesis": {"hypothesis", "hypothesis_id"},
	"task":       {"task", "task_id"},
	"view":       {"saved_view", "view_id"},
	"evidence":   {"evidence_items", "evidence_id"},
}

// Tag is a label of a case with the number of objects that carry it.
type Tag struct {
	ID    string  `json:"tag_id"`
	Name  string  `json:"name"`
	Color *string `json:"color"`
	Uses  int     `json:"uses"`
}

// ListTags lists the tags of a case, in alphabetical order, with how many objects carry each.
func (s *Service) ListTags(ctx context.Context, sc Scope, caseID string) ([]Tag, error) {
	out := []Tag{}
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := requireRole(ctx, tx, sc, caseID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT g.tag_id::text, g.name, g.color, count(a.tag_id)::int FROM forensic.tag g LEFT JOIN forensic.tag_assignment a ON a.tag_id = g.tag_id
			WHERE g.tenant_id = $1 AND g.collection_id = $2 GROUP BY g.tag_id ORDER BY g.name_key`, sc.TenantID, caseID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t Tag
			if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.Uses); err != nil {
				return err
			}
			out = append(out, t)
		}
		return rows.Err()
	})
	return out, err
}

// PatchTag renames or recolours a tag. A rename that meets another tag's name is refused (the tags are one word then).
func (s *Service) PatchTag(ctx context.Context, sc Scope, tagID string, name, color *string, clearColor bool) (*Tag, error) {
	if err := checkUUID("The tag id", tagID); err != nil {
		return nil, err
	}
	if name == nil && color == nil && !clearColor {
		return nil, BadRequest("Nothing to change: send name or color.")
	}
	var cleanName *string
	if name != nil {
		n, err := cleanLine("The name", *name, 1, 60)
		if err != nil {
			return nil, err
		}
		cleanName = &n
	}
	if color != nil && !validColor(*color) {
		return nil, BadRequest("color must look like #1a2b3c.")
	}
	var out Tag
	err := s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		var caseID string
		if err := tx.QueryRow(ctx, `SELECT collection_id FROM forensic.tag WHERE tenant_id = $1 AND tag_id = $2::uuid`, sc.TenantID, tagID).Scan(&caseID); err != nil {
			return asError(err)
		}
		role, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if !role.CanContribute() {
			return Forbidden("Your role in this case (%s) does not allow changing tags.", role)
		}
		if _, err := tx.Exec(ctx, `UPDATE forensic.tag SET name = coalesce($3, name), color = CASE WHEN $5 THEN NULL WHEN $4::text IS NOT NULL THEN $4 ELSE color END WHERE tenant_id = $1 AND tag_id = $2::uuid`,
			sc.TenantID, tagID, cleanName, color, clearColor); err != nil {
			return err
		}
		if err := audit(ctx, tx, sc, role, caseID, "tag.updated", "tag", tagID, map[string]any{"renamed": cleanName != nil, "recoloured": color != nil || clearColor}); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT g.tag_id::text, g.name, g.color, (SELECT count(*)::int FROM forensic.tag_assignment a WHERE a.tag_id = g.tag_id) FROM forensic.tag g WHERE g.tag_id = $1::uuid`, tagID).
			Scan(&out.ID, &out.Name, &out.Color, &out.Uses)
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func validColor(c string) bool {
	if len(c) != 7 || c[0] != '#' {
		return false
	}
	for _, r := range c[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// ObjectTags reads the tags on one object of a case.
func (s *Service) ObjectTags(ctx context.Context, sc Scope, caseID, kind, objectID string) ([]string, error) {
	var out []string
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := requireRole(ctx, tx, sc, caseID); err != nil {
			return err
		}
		if err := checkTaggable(ctx, tx, sc, caseID, kind, objectID); err != nil {
			return err
		}
		var err error
		out, err = objectTags(ctx, tx, kind, objectID)
		return err
	})
	return out, err
}

// checkTaggable confirms that the object exists in the case and that the caller may see it (the row-level policies decide what the caller sees).
func checkTaggable(ctx context.Context, tx pgx.Tx, sc Scope, caseID, kind, objectID string) error {
	t, ok := taggable[kind]
	if !ok {
		kinds := make([]string, 0, len(taggable))
		for k := range taggable {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		return BadRequest("Objects of kind %q cannot be tagged; the kinds are %s.", kind, strings.Join(kinds, ", "))
	}
	if err := checkUUID("The object id", objectID); err != nil {
		return err
	}
	var found *string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT collection_id FROM forensic.%s WHERE tenant_id = $1 AND %s = $2::uuid`, t.table, t.idCol), sc.TenantID, objectID).Scan(&found); err != nil {
		return NotFound("There is no such %s in this case, or you may not see it.", kind)
	}
	if found == nil {
		return BadRequest("A conversation across cases cannot be tagged: a tag belongs to one case. Tag its findings, notes or answers in the case instead.")
	}
	if *found != caseID {
		return NotFound("There is no such %s in this case, or you may not see it.", kind)
	}
	return nil
}

// SetObjectTags makes the tags of an object exactly the named ones, creating tags that do not exist yet. Two spellings of one word are one tag.
func (s *Service) SetObjectTags(ctx context.Context, sc Scope, caseID, kind, objectID string, names []string) ([]string, error) {
	clean := []string{}
	seen := map[string]bool{}
	for _, n := range names {
		n, err := cleanLine("A tag", n, 1, 60)
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(n)
		if !seen[key] {
			seen[key] = true
			clean = append(clean, n)
		}
	}
	if len(clean) > 20 {
		return nil, BadRequest("An object can carry at most 20 tags.")
	}
	var out []string
	err := s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		role, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if !role.CanContribute() {
			return Forbidden("Your role in this case (%s) does not allow tagging.", role)
		}
		if err := checkTaggable(ctx, tx, sc, caseID, kind, objectID); err != nil {
			return err
		}
		want := map[string]string{} // tag id -> name
		for _, n := range clean {
			if _, err := tx.Exec(ctx, `INSERT INTO forensic.tag (tenant_id, collection_id, name, created_by) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, sc.TenantID, caseID, n, sc.Subject); err != nil {
				return err
			}
			var id, name string
			if err := tx.QueryRow(ctx, `SELECT tag_id::text, name FROM forensic.tag WHERE tenant_id = $1 AND collection_id = $2 AND name_key = forensic.search_normalize(btrim($3))`, sc.TenantID, caseID, n).Scan(&id, &name); err != nil {
				return err
			}
			want[id] = name
		}
		have := map[string]string{}
		rows, err := tx.Query(ctx, `SELECT a.tag_id::text, g.name FROM forensic.tag_assignment a JOIN forensic.tag g ON g.tag_id = a.tag_id WHERE a.object_kind = $1 AND a.object_id = $2::uuid`, kind, objectID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return err
			}
			have[id] = name
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		var added, removed []string
		for id, name := range want {
			if _, ok := have[id]; !ok {
				added = append(added, name)
			}
		}
		for id, name := range have {
			if _, ok := want[id]; !ok {
				removed = append(removed, name)
			}
		}
		sort.Strings(added)
		sort.Strings(removed)
		if len(added)+len(removed) > 0 {
			// the audit row first: it names the tags, and the assignments follow
			if err := audit(ctx, tx, sc, role, caseID, "tags.changed", kind, objectID, map[string]any{"added": added, "removed": removed}); err != nil {
				return err
			}
		}
		for id := range want {
			if _, ok := have[id]; !ok {
				if _, err := tx.Exec(ctx, `INSERT INTO forensic.tag_assignment (tag_id, tenant_id, collection_id, object_kind, object_id, assigned_by) VALUES ($1::uuid, $2, $3, $4, $5::uuid, $6)
					ON CONFLICT DO NOTHING`, id, sc.TenantID, caseID, kind, objectID, sc.Subject); err != nil {
					return err
				}
			}
		}
		for id := range have {
			if _, ok := want[id]; !ok {
				if _, err := tx.Exec(ctx, `DELETE FROM forensic.tag_assignment WHERE tag_id = $1::uuid AND object_kind = $2 AND object_id = $3::uuid`, id, kind, objectID); err != nil {
					return err
				}
			}
		}
		out, err = objectTags(ctx, tx, kind, objectID)
		return err
	})
	return out, err
}
