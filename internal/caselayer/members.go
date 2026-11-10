package caselayer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Member is one person's place in a case.
type Member struct {
	Subject     string    `json:"subject_id"`
	Role        string    `json:"role"`
	DisplayName *string   `json:"display_name"`
	AddedBy     string    `json:"added_by"`
	AddedAt     time.Time `json:"added_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// You is the caller's own place in the case, as the policies see it.
type You struct {
	Subject string `json:"subject_id"`
	Role    string `json:"role"`
	Admin   bool   `json:"admin"`
}

// Members is the member list of a case. Mode is "open" while the case has no members (everyone signed in acts as an analyst) and "controlled" once it has one.
type Members struct {
	CaseID  string   `json:"case_id"`
	Mode    string   `json:"mode"`
	Members []Member `json:"members"`
	You     You      `json:"you"`
}

// ListMembers reads the member list of a case. Every participant may read it.
func (s *Service) ListMembers(ctx context.Context, sc Scope, caseID string) (*Members, error) {
	out := Members{CaseID: caseID, Members: []Member{}}
	err := s.Store.InTx(ctx, sc, false, func(ctx context.Context, tx pgx.Tx) error {
		role, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		out.You = You{Subject: sc.Subject, Role: string(role), Admin: sc.IsAdmin()}
		rows, err := tx.Query(ctx, `SELECT subject_id, role, display_name, added_by, added_at, updated_at FROM forensic.case_member WHERE tenant_id = $1 AND collection_id = $2
			ORDER BY (role = 'owner') DESC, (role = 'lead') DESC, subject_id`, sc.TenantID, caseID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var m Member
			if err := rows.Scan(&m.Subject, &m.Role, &m.DisplayName, &m.AddedBy, &m.AddedAt, &m.UpdatedAt); err != nil {
				return err
			}
			m.AddedAt, m.UpdatedAt = utc(m.AddedAt), utc(m.UpdatedAt)
			out.Members = append(out.Members, m)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	out.Mode = "controlled"
	if len(out.Members) == 0 {
		out.Mode = "open"
	}
	return &out, nil
}

// PutMember adds a member or changes a member's role. The first member of an open case must be its owner and, unless an administrator does it, must be the caller; afterwards an owner, a lead or
// an administrator manages the list, and only an owner or an administrator grants or changes the owner role.
func (s *Service) PutMember(ctx context.Context, sc Scope, caseID, subject, role, displayName string) (*Member, error) {
	subject, err := cleanLine("The member's name", subject, 1, 256)
	if err != nil {
		return nil, err
	}
	if !validMemberRole(role) {
		return nil, BadRequest("role must be one of %s.", strings.Join(MemberRoles, ", "))
	}
	var name *string
	if strings.TrimSpace(displayName) != "" {
		n, err := cleanLine("The display name", displayName, 1, 120)
		if err != nil {
			return nil, err
		}
		name = &n
	}
	var out Member
	err = s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		mine, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if !mine.CanManage() {
			return Forbidden("Your role in this case (%s) does not allow changing its members.", mine)
		}
		known, err := caseKnown(ctx, tx, sc.TenantID, caseID)
		if err != nil {
			return err
		}
		if !known {
			return NotFound("There is no such case.")
		}
		var count, owners int
		var existing *string
		if err := tx.QueryRow(ctx, `SELECT count(*)::int, count(*) FILTER (WHERE role = 'owner')::int, max(role) FILTER (WHERE subject_id = $3) FROM forensic.case_member WHERE tenant_id = $1 AND collection_id = $2`,
			sc.TenantID, caseID, subject).Scan(&count, &owners, &existing); err != nil {
			return err
		}
		isOwnerOrAdmin := mine == RoleOwner || sc.IsAdmin()
		switch {
		case count == 0:
			if role != "owner" {
				return BadRequest("The first member of a case must be its owner. A case without members is open to everyone signed in.")
			}
			if !sc.IsAdmin() && subject != sc.Subject {
				return BadRequest("To make a case controlled, make yourself its owner first; then add the others.")
			}
		case (role == "owner" || (existing != nil && *existing == "owner")) && !isOwnerOrAdmin:
			return Forbidden("Only an owner of the case or an administrator can grant or change the owner role.")
		}
		if existing != nil && *existing == "owner" && role != "owner" && owners <= 1 {
			return Conflict("last_owner", "A case must keep at least one owner; make another member the owner first.")
		}
		action := "member.added"
		if existing != nil {
			action = "member.role_changed"
		}
		if err := audit(ctx, tx, sc, mine, caseID, action, "member", subject, map[string]any{"role": role, "previous_role": existing}); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `INSERT INTO forensic.case_member (tenant_id, collection_id, subject_id, role, display_name, added_by) VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (tenant_id, collection_id, subject_id) DO UPDATE SET role = EXCLUDED.role, display_name = coalesce(EXCLUDED.display_name, forensic.case_member.display_name), updated_by = $6, updated_at = now()
			RETURNING subject_id, role, display_name, added_by, added_at, updated_at`, sc.TenantID, caseID, subject, role, name, sc.Subject).
			Scan(&out.Subject, &out.Role, &out.DisplayName, &out.AddedBy, &out.AddedAt, &out.UpdatedAt)
	})
	if err != nil {
		return nil, err
	}
	out.AddedAt, out.UpdatedAt = utc(out.AddedAt), utc(out.UpdatedAt)
	return &out, nil
}

// RemoveMember takes a person out of a case. The last owner cannot be removed.
func (s *Service) RemoveMember(ctx context.Context, sc Scope, caseID, subject string) error {
	return s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		mine, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if !mine.CanManage() {
			return Forbidden("Your role in this case (%s) does not allow changing its members.", mine)
		}
		var role string
		var owners int
		if err := tx.QueryRow(ctx, `SELECT m.role, (SELECT count(*)::int FROM forensic.case_member o WHERE o.tenant_id = m.tenant_id AND o.collection_id = m.collection_id AND o.role = 'owner')
			FROM forensic.case_member m WHERE m.tenant_id = $1 AND m.collection_id = $2 AND m.subject_id = $3`, sc.TenantID, caseID, subject).Scan(&role, &owners); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return NotFound("%q is not a member of this case.", subject)
			}
			return err
		}
		if role == "owner" {
			if mine != RoleOwner && !sc.IsAdmin() {
				return Forbidden("Only an owner of the case or an administrator can remove an owner.")
			}
			if owners <= 1 {
				return Conflict("last_owner", "A case must keep at least one owner; make another member the owner first.")
			}
		}
		// the audit row first: after the row is deleted the caller may no longer be allowed to write it (a person removing themselves)
		if err := audit(ctx, tx, sc, mine, caseID, "member.removed", "member", subject, map[string]any{"role": role}); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM forensic.case_member WHERE tenant_id = $1 AND collection_id = $2 AND subject_id = $3`, sc.TenantID, caseID, subject)
		return err
	})
}

// RevokeAuthority marks an authority record revoked, with the reason. The record stays; its status becomes "revoked".
func (s *Service) RevokeAuthority(ctx context.Context, sc Scope, id, reason string) (json.RawMessage, error) {
	reason, err := cleanLine("The reason", reason, 3, 500)
	if err != nil {
		return nil, err
	}
	var out json.RawMessage
	err = s.Store.InTx(ctx, sc, true, func(ctx context.Context, tx pgx.Tx) error {
		_, caseID, _, err := authoritySpec.readObject(ctx, tx, sc, id)
		if err != nil {
			return err
		}
		role, err := requireRole(ctx, tx, sc, caseID)
		if err != nil {
			return err
		}
		if err := authoritySpec.writerRole(role); err != nil {
			return err
		}
		if err := audit(ctx, tx, sc, role, caseID, "authority.revoked", "authority", id, map[string]any{"reason": reason}); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE forensic.authority_record SET revoked_at = now(), revoked_by = $3, revoke_reason = $4, updated_by = $3, updated_at = now()
			WHERE tenant_id = $1 AND authority_id = $2::uuid AND revoked_at IS NULL`, sc.TenantID, id, sc.Subject, reason)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return Conflict("already_revoked", "That authority record is already revoked.")
		}
		out, _, _, err = authoritySpec.readObject(ctx, tx, sc, id)
		return err
	})
	return out, err
}
