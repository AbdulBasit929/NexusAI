package caselayer

import (
	"encoding/json"
	"regexp"
	"strconv"
)

var kindPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,40}$`)

var noteSpec = objectSpec{
	Kind: "note", Plural: "notes", Table: "case_note", IDCol: "note_id", TagKind: "note", AuthorCol: "created_by", SoftDelete: true,
	Fields: []field{
		{Name: "category", Kind: fEnum, Enum: []string{"note", "lead", "hypothesis", "finding", "question"}, Create: true, Patch: true, Default: "note"},
		{Name: "title", Kind: fText, Min: 1, Max: 200, Create: true, Patch: true, Nullable: true},
		{Name: "body", Kind: fBody, Min: 1, Max: 20000, Required: true, Create: true, Patch: true},
		{Name: "pinned", Kind: fBool, Create: true, Patch: true},
		{Name: "source_turn_id", Kind: fUUID, Create: true, Nullable: true},
	},
	SearchCols: []string{"title", "body"},
	Filters:    []filterSpec{{Param: "category", Column: "category"}, {Param: "pinned", Column: "pinned", Kind: fBool}, {Param: "source_turn_id", Column: "source_turn_id", Kind: fUUID}},
	OrderBy:    "x.pinned DESC, x.updated_at DESC, x.note_id DESC",
}

var hypothesisSpec = objectSpec{
	Kind: "hypothesis", Plural: "hypotheses", Table: "hypothesis", IDCol: "hypothesis_id", TagKind: "hypothesis", AuthorCol: "created_by", SoftDelete: true,
	Fields: []field{
		{Name: "statement", Kind: fBody, Min: 1, Max: 2000, Required: true, Create: true, Patch: true},
		{Name: "rationale", Kind: fBody, Max: 20000, Create: true, Patch: true, Nullable: true},
		{Name: "status", Kind: fEnum, Enum: []string{"open", "supported", "refuted", "parked"}, Create: true, Patch: true, Default: "open"},
		{Name: "confidence", Kind: fEnum, Enum: []string{"low", "medium", "high"}, Create: true, Patch: true, Nullable: true},
		{Name: "supports", Kind: fUUIDs, Create: true, Patch: true},
		{Name: "contradicts", Kind: fUUIDs, Create: true, Patch: true},
	},
	SearchCols: []string{"statement", "rationale"},
	Filters:    []filterSpec{{Param: "status", Column: "status"}, {Param: "confidence", Column: "confidence"}},
}

// taskComplete keeps the completion time and the person in step with the status.
func taskComplete(vals map[string]any, subject string, args *[]any) []string {
	status, ok := vals["status"].(string)
	if !ok {
		return nil
	}
	if status == "done" {
		*args = append(*args, subject)
		return []string{"completed_at = now()", "completed_by = $" + strconv.Itoa(len(*args))}
	}
	return []string{"completed_at = NULL", "completed_by = NULL"}
}

var taskSpec = objectSpec{
	Kind: "task", Plural: "tasks", Table: "task", IDCol: "task_id", TagKind: "task", AuthorCol: "created_by", SoftDelete: true,
	Fields: []field{
		{Name: "title", Kind: fText, Min: 1, Max: 200, Required: true, Create: true, Patch: true},
		{Name: "detail", Kind: fBody, Max: 20000, Create: true, Patch: true, Nullable: true},
		{Name: "status", Kind: fEnum, Enum: []string{"open", "in_progress", "done", "cancelled"}, Create: true, Patch: true, Default: "open"},
		{Name: "assignee_subject", Kind: fText, Min: 1, Max: 256, Create: true, Patch: true, Nullable: true},
		{Name: "due_at", Kind: fTime, Create: true, Patch: true, Nullable: true},
		{Name: "source_kind", Kind: fEnum, Enum: []string{"turn", "finding", "hypothesis", "note", "evidence"}, Create: true, Nullable: true},
		{Name: "source_id", Kind: fUUID, Create: true, Nullable: true},
	},
	SearchCols: []string{"title", "detail"},
	Filters:    []filterSpec{{Param: "status", Column: "status"}, {Param: "assignee", Column: "assignee_subject"}},
	OrderBy:    "(x.status IN ('done', 'cancelled')), x.due_at NULLS LAST, x.created_at DESC, x.task_id DESC",
	Complete:   taskComplete,
}

var savedViewSpec = objectSpec{
	Kind: "view", Plural: "views", Table: "saved_view", IDCol: "view_id", TagKind: "view", AuthorCol: "owner_subject", OwnerOnly: true, SoftDelete: true,
	Fields: []field{
		{Name: "name", Kind: fText, Min: 1, Max: 120, Required: true, Create: true, Patch: true},
		{Name: "kind", Kind: fText, Min: 2, Max: 41, Required: true, Create: true, Pattern: kindPattern},
		{Name: "params_json", Kind: fJSONObject, Max: 262144, Create: true, Patch: true, Default: "{}"},
		{Name: "visibility", Kind: fEnum, Enum: []string{"private", "case"}, Create: true, Patch: true, Default: "private"},
	},
	SearchCols: []string{"name"},
	Filters:    []filterSpec{{Param: "kind", Column: "kind"}, {Param: "visibility", Column: "visibility"}},
}

var annotationSpec = objectSpec{
	Kind: "annotation", Plural: "annotations", Table: "annotation", IDCol: "annotation_id", AuthorCol: "created_by", SoftDelete: true,
	Fields: []field{
		{Name: "target_kind", Kind: fText, Min: 2, Max: 41, Required: true, Create: true, Pattern: kindPattern},
		{Name: "target_key", Kind: fText, Min: 1, Max: 300, Required: true, Create: true},
		{Name: "locator", Kind: fJSONObject, Max: 8192, Create: true, Patch: true, Default: "{}"},
		{Name: "body", Kind: fBody, Min: 1, Max: 4000, Required: true, Create: true, Patch: true},
	},
	SearchCols: []string{"body", "target_key"},
	Filters:    []filterSpec{{Param: "target_kind", Column: "target_kind"}, {Param: "target_key", Column: "target_key"}},
	OrderBy:    "x.target_key, x.created_at, x.annotation_id",
}

func validateAuthority(vals map[string]any) error {
	if raw, ok := vals["attachments"].(string); ok {
		var list []map[string]any
		if err := json.Unmarshal([]byte(raw), &list); err != nil {
			return BadRequest("attachments must be a list of objects that each carry an evidence_id.")
		}
		for _, a := range list {
			id, _ := a["evidence_id"].(string)
			if checkUUID("An attachment's evidence_id", id) != nil {
				return BadRequest("Each attachment needs the evidence_id of a file of the case.")
			}
		}
	}
	from, _ := vals["valid_from"].(string)
	to, _ := vals["valid_to"].(string)
	if from != "" && to != "" && to < from {
		return BadRequest("valid_to is before valid_from.")
	}
	return nil
}

var authoritySpec = objectSpec{
	Kind: "authority", Plural: "authority", Table: "authority_record", IDCol: "authority_id", AuthorCol: "recorded_by", ManageOnly: true,
	Fields: []field{
		{Name: "kind", Kind: fEnum, Enum: []string{"warrant", "court_order", "consent", "internal_approval", "other"}, Required: true, Create: true, Patch: true},
		{Name: "reference", Kind: fText, Min: 1, Max: 200, Required: true, Create: true, Patch: true},
		{Name: "issuing_body", Kind: fText, Min: 1, Max: 200, Required: true, Create: true, Patch: true},
		{Name: "scope", Kind: fBody, Max: 4000, Create: true, Patch: true, Nullable: true},
		{Name: "valid_from", Kind: fDate, Create: true, Patch: true, Nullable: true},
		{Name: "valid_to", Kind: fDate, Create: true, Patch: true, Nullable: true},
		{Name: "notes", Kind: fBody, Max: 20000, Create: true, Patch: true, Nullable: true},
		{Name: "attachments", Kind: fJSONArray, Max: 65536, Create: true, Patch: true, Default: "[]"},
	},
	SearchCols: []string{"reference", "issuing_body", "scope"},
	Filters:    []filterSpec{{Param: "kind", Column: "kind"}},
	OrderBy:    "x.recorded_at DESC, x.authority_id DESC",
	ExtraJSON: `'status', CASE WHEN x.revoked_at IS NOT NULL THEN 'revoked' WHEN x.valid_to IS NOT NULL AND x.valid_to < current_date THEN 'expired'
		WHEN x.valid_from IS NOT NULL AND x.valid_from > current_date THEN 'not_yet_valid' ELSE 'active' END`,
	Validate: validateAuthority,
}

// findingSpec is the plain part of a finding: the title, the text, what kind of claim it is, the order and the evidence it cites. Pinning an answer and reviewing are their own calls (findings.go).
var findingSpec = objectSpec{
	Kind: "finding", Plural: "findings", Table: "finding", IDCol: "finding_id", TagKind: "finding", AuthorCol: "created_by", SoftDelete: true,
	Fields: []field{
		{Name: "title", Kind: fText, Min: 1, Max: 200, Required: true, Create: true, Patch: true},
		{Name: "body", Kind: fBody, Max: 20000, Create: true, Patch: true, Default: ""},
		{Name: "basis", Kind: fEnum, Enum: []string{"observed", "inferred"}, Create: true, Patch: true, Default: "observed"},
		{Name: "sort_order", Kind: fInt, Min: 0, Max: 1000000, Create: true, Patch: true},
		{Name: "source_turn_id", Kind: fUUID, Create: true, Nullable: true},
		{Name: "evidence_refs", Kind: fJSONArray, Max: 262144, Create: true, Patch: true, Default: "[]"},
	},
	SearchCols: []string{"title", "body"},
	Filters:    []filterSpec{{Param: "review_state", Column: "review_state"}, {Param: "basis", Column: "basis"}, {Param: "source_turn_id", Column: "source_turn_id", Kind: fUUID}},
	OrderBy:    "x.sort_order, x.created_at, x.finding_id",
}

// objectSpecs are the knowledge objects by the plural used in the routes.
var objectSpecs = map[string]objectSpec{
	"notes":       noteSpec,
	"hypotheses":  hypothesisSpec,
	"tasks":       taskSpec,
	"views":       savedViewSpec,
	"annotations": annotationSpec,
	"authority":   authoritySpec,
	"findings":    findingSpec,
}
