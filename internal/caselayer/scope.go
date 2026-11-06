package caselayer

// Scope is who is asking. The API fills it from the authenticated principal (never from the request body): the tenant, the subject (the person, or the one shared actor of today's workspace)
// and the global role, which is "admin" or anything else. Every database transaction of the layer sets these three as app.tenant_id, app.subject_id and app.actor_role, which the
// row-level policies of migration 034 read.
type Scope struct {
	TenantID  string
	Subject   string
	ActorRole string
	// BoundCase is the one case a credential is limited to (the collection header of a key bound to a case); empty for a credential that is not. A bound credential sees nothing of any other case.
	BoundCase string
}

// IsAdmin is the global administrator, who is a lead in every case and can read every thread.
func (s Scope) IsAdmin() bool { return s.ActorRole == "admin" }

// Role is what a person is in one case, as forensic.case_role() computes it: a member's role, "open" when the case has no members (everyone is an analyst), or "none".
type Role string

const (
	RoleOwner    Role = "owner"
	RoleLead     Role = "lead"
	RoleAnalyst  Role = "analyst"
	RoleReviewer Role = "reviewer"
	RoleViewer   Role = "viewer"
	RoleAuditor  Role = "auditor"
	RoleOpen     Role = "open"
	RoleNone     Role = "none"
)

// Valid member roles, in the order they are offered.
var MemberRoles = []string{"owner", "lead", "analyst", "reviewer", "viewer", "auditor"}

// CanRead is any role but none.
func (r Role) CanRead() bool { return r != RoleNone && r != "" }

// CanContribute may create threads, notes, findings and the like.
func (r Role) CanContribute() bool {
	return r == RoleOwner || r == RoleLead || r == RoleAnalyst || r == RoleOpen
}

// CanReview may confirm or reject a finding and give feedback on an answer.
func (r Role) CanReview() bool {
	return r == RoleOwner || r == RoleLead || r == RoleReviewer || r == RoleAnalyst || r == RoleOpen
}

// CanGiveFeedback is every role that reads and is not an auditor.
func (r Role) CanGiveFeedback() bool { return r.CanRead() && r != RoleAuditor }

// CanManage may change members, authority records and the case audit trail's visibility. While a case is open anyone may start managing it (the first member).
func (r Role) CanManage() bool { return r == RoleOwner || r == RoleLead || r == RoleOpen }

// CanReadAudit may read the case's audit trail.
func (r Role) CanReadAudit() bool {
	return r == RoleOwner || r == RoleLead || r == RoleAuditor || r == RoleOpen
}

// SeesPrivateThreads is the role that reads other people's private threads (auditors, as the roadmap requires; a global admin is added by the caller).
func (r Role) SeesPrivateThreads() bool { return r == RoleAuditor }

func validMemberRole(role string) bool {
	for _, r := range MemberRoles {
		if r == role {
			return true
		}
	}
	return false
}
