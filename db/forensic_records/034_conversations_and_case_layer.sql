-- Migration 034 (DRAFT, never applied): conversations with history, and the case layer the interface needs: notes, findings, hypotheses, tasks, tags, saved views, annotations,
-- case members, authority records and a hash-chained audit trail. Phase 1 lane C of docs/roadmap/master-roadmap-20261103 (backend requests BR-2, BR-3 and the member and authority
-- part of BR-1 of docs/work/BACKEND_REQUESTS.md). Design: docs/architecture/CONVERSATIONS_AND_CASE_LAYER_20261103.md. Contract: docs/work/API_CONTRACT_CONVERSATIONS_20261103.md.
--
-- Why. Today the Ask page keeps its questions, the notes and the timeline pins in the browser only. A question asked on one machine is gone on another, an answer cannot be re-run and compared
-- ("changed since"), a finding cannot cite the answer it came from, and there is no member or authority record to say who may see a case and under what legal basis. This stores all of that on
-- the server, tenant by tenant and case by case, under row-level security, and writes one audit row, chained to the one before it, for every change.
--
-- What it adds (all in schema forensic):
--   conversations   conversation_thread, conversation_turn, turn_feedback, case_data_version
--   knowledge       case_note, finding, hypothesis, task, tag, tag_assignment, saved_view, annotation
--   authority       case_member, authority_record
--   audit           case_audit_event (append-only, one hash chain per case, written by a trigger) and forensic.case_audit_chain_status()
--   functions       forensic.search_normalize() (Urdu and Roman-Urdu aware text folding for search), forensic.case_role() (the caller's role in a case, read by the policies)
--
-- Access. A case with NO member rows is "open": every signed-in user acts as an analyst, which is how the workspace behaves today (one shared actor). The first member row turns the case into a
-- controlled one and the member list decides. Policies read three settings that the API sets per transaction: app.tenant_id, app.subject_id and app.actor_role; a missing setting fails closed.
-- A thread is visible to its owner, to the whole case when visibility = 'case', to the listed subjects when 'selected', and always to an auditor member or a global admin.
--
-- Nothing is deleted by the application: threads, notes, findings and the like are hidden (hidden_at / deleted_at with who and why) and the audit row keeps the fact. A finished turn cannot be changed.
-- National ID numbers typed into a question are masked by the API before they are stored; this file stores no identity data.
--
-- Additive and idempotent; it references only forensic.evidence tables by value (no foreign key to them) so it applies on any database that has 001 to 021. Requires pg_trgm (installed by 001 and 012).
-- Apply by hand on a retained database: ops/034_conversations/preflight.sql first, verify.sql and behaviour.sql after; rollback.sql drops it all (gated).
BEGIN;
SET LOCAL lock_timeout = '10s';

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
-- A. Members (the policies and forensic.case_role() read this table, so it comes first)
-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS forensic.case_member (
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  subject_id text NOT NULL,
  role text NOT NULL,
  display_name text,
  added_by text NOT NULL DEFAULT '',
  added_at timestamptz NOT NULL DEFAULT now(),
  updated_by text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, collection_id, subject_id),
  CONSTRAINT case_member_role_chk CHECK (role IN ('owner', 'lead', 'analyst', 'reviewer', 'viewer', 'auditor')),
  CONSTRAINT case_member_collection_chk CHECK (collection_id <> '' AND length(collection_id) <= 128),
  CONSTRAINT case_member_subject_chk CHECK (length(btrim(subject_id)) BETWEEN 1 AND 256 AND subject_id !~ '[[:cntrl:]]'),
  CONSTRAINT case_member_name_chk CHECK (display_name IS NULL OR (length(btrim(display_name)) BETWEEN 1 AND 120 AND display_name !~ '[[:cntrl:]]'))
);

COMMENT ON TABLE forensic.case_member IS
  'Who may work in a case and in which role (owner, lead, analyst, reviewer, viewer, auditor). A case with no row here is open: every signed-in user acts as an analyst. The first row makes the case controlled.';

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
-- B. Functions
-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

-- Text folding for search. Arabic-script text is typed with several code points for one Urdu letter and with or without marks: the Arabic yeh (U+064A) and alef maksura (U+0649) become the
-- Urdu yeh (U+06CC), the Arabic kaf (U+0643) the Urdu kaf (U+06A9), the Arabic heh (U+0647) the heh goal (U+06C1), the hamza-carrying alefs (U+0623, U+0625, U+0671) a plain alef; Arabic-Indic and
-- extended Arabic-Indic digits become ASCII digits (a number typed in Urdu digits finds the same number in Latin digits); short vowels and other marks (U+064B to U+065F, U+0670), the tatweel
-- (U+0640) and the zero-width and direction marks (U+200B to U+200F) are removed; Latin letters are lower-cased. Roman Urdu needs nothing more than lower-casing: its spelling variants are met by
-- the trigram index of the search, not by this function. IMMUTABLE so that the search columns can be generated from it.
CREATE OR REPLACE FUNCTION forensic.search_normalize(input text)
RETURNS text
LANGUAGE sql
IMMUTABLE
PARALLEL SAFE
AS $f$
  SELECT lower(regexp_replace(
    translate(coalesce(input, ''),
      U&'\064A\0649\0643\0647\0623\0625\0671\0660\0661\0662\0663\0664\0665\0666\0667\0668\0669\06F0\06F1\06F2\06F3\06F4\06F5\06F6\06F7\06F8\06F9',
      U&'\06CC\06CC\06A9\06C1\0627\0627\0627' || '01234567890123456789'),
    U&'[\064B-\065F\0670\0640\200B-\200F]', '', 'g'))
$f$;

-- The caller's role in a case: owner, lead, analyst, reviewer, viewer or auditor (a member row); 'open' when the case has no member rows; 'none' for a non-member of a controlled case, for a case of
-- another tenant and when no subject is set. A global admin is a lead everywhere. SECURITY DEFINER so that reading the member table does not itself run through the member table's policy (which
-- calls this function); the search path is fixed and the tenant is checked against the transaction's own setting, so the function answers only for the caller's tenant.
CREATE OR REPLACE FUNCTION forensic.case_role(p_tenant text, p_case text, p_subject text)
RETURNS text
LANGUAGE plpgsql
STABLE
SECURITY DEFINER
SET search_path = forensic, pg_catalog, pg_temp
AS $f$
DECLARE
  found text;
BEGIN
  IF p_tenant IS NULL OR p_case IS NULL OR coalesce(p_subject, '') = '' OR p_tenant IS DISTINCT FROM current_setting('app.tenant_id', true) THEN
    RETURN 'none';
  END IF;
  IF current_setting('app.actor_role', true) = 'admin' THEN
    RETURN 'lead';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM forensic.case_member m WHERE m.tenant_id = p_tenant AND m.collection_id = p_case) THEN
    RETURN 'open';
  END IF;
  SELECT m.role INTO found FROM forensic.case_member m WHERE m.tenant_id = p_tenant AND m.collection_id = p_case AND m.subject_id = p_subject;
  RETURN coalesce(found, 'none');
END;
$f$;

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
-- C. Conversations
-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS forensic.conversation_thread (
  thread_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  -- reserved for threads that span cases (Ask across cases); not used until the engine can answer across cases, which needs collection_id to become nullable
  scope_collection_ids text[] NOT NULL DEFAULT '{}',
  owner_subject text NOT NULL,
  title text,
  title_source text NOT NULL DEFAULT 'auto',
  pinned boolean NOT NULL DEFAULT false,
  archived boolean NOT NULL DEFAULT false,
  visibility text NOT NULL DEFAULT 'private',
  shared_with text[] NOT NULL DEFAULT '{}',
  forked_from_thread_id uuid,
  forked_from_turn_id uuid,
  turn_count integer NOT NULL DEFAULT 0,
  first_question text,
  last_question text,
  last_headline text,
  last_outcome text,
  last_turn_at timestamptz,
  hidden_at timestamptz,
  hidden_by text,
  hide_reason text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT conversation_thread_collection_chk CHECK (collection_id <> '' AND length(collection_id) <= 128),
  CONSTRAINT conversation_thread_title_chk CHECK (title IS NULL OR (char_length(btrim(title)) BETWEEN 1 AND 200 AND title !~ '[[:cntrl:]]')),
  CONSTRAINT conversation_thread_title_source_chk CHECK (title_source IN ('auto', 'user')),
  CONSTRAINT conversation_thread_visibility_chk CHECK (visibility IN ('private', 'selected', 'case')),
  CONSTRAINT conversation_thread_shared_chk CHECK (cardinality(shared_with) <= 100),
  CONSTRAINT conversation_thread_scope_chk CHECK (cardinality(scope_collection_ids) = 0),
  CONSTRAINT conversation_thread_turns_chk CHECK (turn_count >= 0),
  CONSTRAINT conversation_thread_hidden_chk CHECK ((hidden_at IS NULL) = (hidden_by IS NULL)),
  CONSTRAINT conversation_thread_owner_chk CHECK (length(btrim(owner_subject)) BETWEEN 1 AND 256),
  UNIQUE (thread_id, tenant_id, collection_id)
);

CREATE INDEX IF NOT EXISTS conversation_thread_rail_idx
  ON forensic.conversation_thread (tenant_id, collection_id, pinned DESC, last_turn_at DESC NULLS LAST, thread_id DESC) WHERE hidden_at IS NULL;
CREATE INDEX IF NOT EXISTS conversation_thread_owner_idx
  ON forensic.conversation_thread (tenant_id, owner_subject, last_turn_at DESC NULLS LAST);

COMMENT ON TABLE forensic.conversation_thread IS
  'A conversation about one case: owner, title, pinned, archived, who may read it (private, selected, case), where it was forked from. Hidden (not deleted) by hidden_at; the audit row keeps who and why.';

CREATE TABLE IF NOT EXISTS forensic.conversation_turn (
  turn_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  thread_id uuid NOT NULL,
  idx integer NOT NULL,
  parent_turn_id uuid REFERENCES forensic.conversation_turn (turn_id),
  kind text NOT NULL DEFAULT 'ask',
  edit_of_turn_id uuid REFERENCES forensic.conversation_turn (turn_id),
  rerun_of_turn_id uuid REFERENCES forensic.conversation_turn (turn_id),
  root_turn_id uuid NOT NULL REFERENCES forensic.conversation_turn (turn_id),
  question_text text NOT NULL,
  resolved_text text,
  language text,
  frame_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  resolution_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  state text NOT NULL DEFAULT 'running',
  outcome text,
  headline text,
  answer_text text,
  shape text,
  row_count integer,
  query_text text,
  query_hash text,
  result_digest text,
  figures_json jsonb,
  evidence_refs jsonb NOT NULL DEFAULT '[]'::jsonb,
  snapshot bytea,
  snapshot_state text NOT NULL DEFAULT 'none',
  snapshot_bytes integer,
  snapshot_sha256 text,
  engine_version text,
  model_id text,
  prompt_id text,
  data_version_hash text,
  timings_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  diff_summary jsonb,
  error_reason text,
  request_id text,
  client_request_id text,
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  -- search: the question and the words of the answer, folded by forensic.search_normalize; a tsvector ranks, a trigram index serves the substring match
  search_text text GENERATED ALWAYS AS (forensic.search_normalize(question_text || ' ' || coalesce(answer_text, ''))) STORED,
  search_tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple', forensic.search_normalize(question_text || ' ' || coalesce(answer_text, '')))) STORED,
  CONSTRAINT conversation_turn_collection_chk CHECK (collection_id <> '' AND length(collection_id) <= 128),
  CONSTRAINT conversation_turn_kind_chk CHECK (kind IN ('ask', 'edit', 'rerun')),
  CONSTRAINT conversation_turn_edit_chk CHECK (kind <> 'edit' OR edit_of_turn_id IS NOT NULL),
  CONSTRAINT conversation_turn_rerun_chk CHECK (kind <> 'rerun' OR rerun_of_turn_id IS NOT NULL),
  CONSTRAINT conversation_turn_state_chk CHECK (state IN ('running', 'completed', 'failed', 'interrupted')),
  CONSTRAINT conversation_turn_outcome_chk CHECK (outcome IS NULL OR outcome IN ('answered', 'abstained', 'clarification', 'withheld', 'unsupported', 'refused', 'failed', 'unknown')),
  CONSTRAINT conversation_turn_question_chk CHECK (char_length(question_text) BETWEEN 1 AND 4000),
  CONSTRAINT conversation_turn_idx_chk CHECK (idx >= 1),
  CONSTRAINT conversation_turn_json_chk CHECK (jsonb_typeof(frame_json) = 'object' AND jsonb_typeof(resolution_json) = 'object' AND jsonb_typeof(timings_json) = 'object' AND jsonb_typeof(evidence_refs) = 'array'),
  CONSTRAINT conversation_turn_snapshot_chk CHECK (snapshot_state IN ('full', 'trimmed', 'none') AND ((snapshot IS NOT NULL) = (snapshot_state IN ('full', 'trimmed')))),
  CONSTRAINT conversation_turn_hash_chk CHECK (
    (query_hash IS NULL OR query_hash ~ '^[0-9a-f]{64}$') AND (result_digest IS NULL OR result_digest ~ '^[0-9a-f]{64}$')
    AND (snapshot_sha256 IS NULL OR snapshot_sha256 ~ '^[0-9a-f]{64}$') AND (data_version_hash IS NULL OR data_version_hash ~ '^[0-9a-f]{64}$')),
  CONSTRAINT conversation_turn_done_chk CHECK (state = 'running' OR completed_at IS NOT NULL),
  UNIQUE (thread_id, idx),
  -- a turn belongs to the case of its thread
  CONSTRAINT conversation_turn_thread_fk FOREIGN KEY (thread_id, tenant_id, collection_id) REFERENCES forensic.conversation_thread (thread_id, tenant_id, collection_id)
);

CREATE INDEX IF NOT EXISTS conversation_turn_case_idx ON forensic.conversation_turn (tenant_id, collection_id, created_at DESC);
CREATE INDEX IF NOT EXISTS conversation_turn_parent_idx ON forensic.conversation_turn (parent_turn_id) WHERE parent_turn_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS conversation_turn_root_idx ON forensic.conversation_turn (root_turn_id, idx);
CREATE INDEX IF NOT EXISTS conversation_turn_rerun_idx ON forensic.conversation_turn (rerun_of_turn_id) WHERE rerun_of_turn_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS conversation_turn_client_request_idx ON forensic.conversation_turn (thread_id, client_request_id) WHERE client_request_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS conversation_turn_search_tsv_idx ON forensic.conversation_turn USING gin (search_tsv);
CREATE INDEX IF NOT EXISTS conversation_turn_search_trgm_idx ON forensic.conversation_turn USING gin (search_text gin_trgm_ops);

COMMENT ON TABLE forensic.conversation_turn IS
  'One question and the answer the engine gave: the question as typed (dashed national ID numbers masked), the question actually sent (resolved_text), the frame the engine read, the outcome, the headline, the query, a digest of the figures, the evidence it cited, a gzip snapshot of the whole response (size-bounded), the engine, model and data version it ran on. Immutable once it is no longer running.';
COMMENT ON COLUMN forensic.conversation_turn.snapshot IS
  'gzip of the engine response exactly as returned, or a trimmed copy (result rows cut to 200, evidence rows to 100) when the whole is over the limit; snapshot_state says which. A turn with no snapshot can still be re-run.';
COMMENT ON COLUMN forensic.conversation_turn.data_version_hash IS
  'SHA-256 of the case data the answer was computed on (evidence versions, typed-family watermarks, the settings that change figures); the document is in case_data_version.';

CREATE TABLE IF NOT EXISTS forensic.turn_feedback (
  turn_id uuid NOT NULL REFERENCES forensic.conversation_turn (turn_id),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  subject_id text NOT NULL,
  verdict text NOT NULL,
  note text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (turn_id, subject_id),
  CONSTRAINT turn_feedback_verdict_chk CHECK (verdict IN ('correct', 'wrong', 'incomplete', 'unclear')),
  CONSTRAINT turn_feedback_note_chk CHECK (note IS NULL OR char_length(note) <= 2000)
);

COMMENT ON TABLE forensic.turn_feedback IS
  'One reviewer''s verdict on one answer (correct, wrong, incomplete, unclear) with an optional note. "This answer is wrong" feeds the triage bank; changing a verdict is a new audit row.';

CREATE TABLE IF NOT EXISTS forensic.case_data_version (
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  data_version_hash text NOT NULL,
  detail jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, collection_id, data_version_hash),
  CONSTRAINT case_data_version_hash_chk CHECK (data_version_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT case_data_version_detail_chk CHECK (jsonb_typeof(detail) = 'object')
);

COMMENT ON TABLE forensic.case_data_version IS
  'The document behind a data_version_hash: every evidence item with its version and content hash, the row count and latest ingestion time of each typed family, the settings that change figures. One row per distinct version, so a diff can name what moved.';

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
-- D. Knowledge capture
-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS forensic.case_note (
  note_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  category text NOT NULL DEFAULT 'note',
  title text,
  body text NOT NULL,
  pinned boolean NOT NULL DEFAULT false,
  source_turn_id uuid REFERENCES forensic.conversation_turn (turn_id),
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_by text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by text,
  delete_reason text,
  CONSTRAINT case_note_collection_chk CHECK (collection_id <> '' AND length(collection_id) <= 128),
  CONSTRAINT case_note_category_chk CHECK (category IN ('note', 'lead', 'hypothesis', 'finding', 'question')),
  CONSTRAINT case_note_title_chk CHECK (title IS NULL OR (char_length(btrim(title)) BETWEEN 1 AND 200 AND title !~ '[[:cntrl:]]')),
  CONSTRAINT case_note_body_chk CHECK (char_length(btrim(body)) BETWEEN 1 AND 20000),
  CONSTRAINT case_note_deleted_chk CHECK ((deleted_at IS NULL) = (deleted_by IS NULL))
);
CREATE INDEX IF NOT EXISTS case_note_case_idx ON forensic.case_note (tenant_id, collection_id, pinned DESC, updated_at DESC) WHERE deleted_at IS NULL;
COMMENT ON TABLE forensic.case_note IS
  'A free note on a case (a lead, a hypothesis, a question, a finding in the making), pinned or not, optionally taken from an answer. Hidden by deleted_at; the audit row keeps who and why.';

CREATE TABLE IF NOT EXISTS forensic.finding (
  finding_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  title text NOT NULL,
  body text NOT NULL DEFAULT '',
  basis text NOT NULL DEFAULT 'observed',
  review_state text NOT NULL DEFAULT 'unreviewed',
  reviewed_by text,
  reviewed_at timestamptz,
  review_note text,
  sort_order integer NOT NULL DEFAULT 0,
  source_turn_id uuid REFERENCES forensic.conversation_turn (turn_id),
  pinned_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  evidence_refs jsonb NOT NULL DEFAULT '[]'::jsonb,
  data_version_hash text,
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_by text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by text,
  delete_reason text,
  CONSTRAINT finding_collection_chk CHECK (collection_id <> '' AND length(collection_id) <= 128),
  CONSTRAINT finding_title_chk CHECK (char_length(btrim(title)) BETWEEN 1 AND 200 AND title !~ '[[:cntrl:]]'),
  CONSTRAINT finding_body_chk CHECK (char_length(body) <= 20000),
  CONSTRAINT finding_basis_chk CHECK (basis IN ('observed', 'inferred')),
  CONSTRAINT finding_review_chk CHECK (review_state IN ('unreviewed', 'confirmed', 'rejected') AND ((review_state = 'unreviewed') = (reviewed_by IS NULL))),
  CONSTRAINT finding_json_chk CHECK (jsonb_typeof(pinned_json) = 'object' AND jsonb_typeof(evidence_refs) = 'array'),
  CONSTRAINT finding_hash_chk CHECK (data_version_hash IS NULL OR data_version_hash ~ '^[0-9a-f]{64}$'),
  CONSTRAINT finding_deleted_chk CHECK ((deleted_at IS NULL) = (deleted_by IS NULL))
);
CREATE INDEX IF NOT EXISTS finding_case_idx ON forensic.finding (tenant_id, collection_id, sort_order, created_at) WHERE deleted_at IS NULL;
-- pinning the same answer twice returns the finding that exists
CREATE UNIQUE INDEX IF NOT EXISTS finding_source_turn_idx ON forensic.finding (tenant_id, collection_id, source_turn_id) WHERE source_turn_id IS NOT NULL AND deleted_at IS NULL;

COMMENT ON TABLE forensic.finding IS
  'A claim the analyst stands behind, usually an answer pinned from a turn. basis says what kind of claim it is (observed in the records, or inferred by a stated rule); review_state says whether a second person confirmed or rejected it, with name and time. pinned_json keeps the digest of the answer as it was when pinned, so the finding stays reproducible if the thread is hidden.';

CREATE TABLE IF NOT EXISTS forensic.hypothesis (
  hypothesis_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  statement text NOT NULL,
  rationale text,
  status text NOT NULL DEFAULT 'open',
  confidence text,
  supports uuid[] NOT NULL DEFAULT '{}',
  contradicts uuid[] NOT NULL DEFAULT '{}',
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_by text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by text,
  delete_reason text,
  CONSTRAINT hypothesis_collection_chk CHECK (collection_id <> '' AND length(collection_id) <= 128),
  CONSTRAINT hypothesis_statement_chk CHECK (char_length(btrim(statement)) BETWEEN 1 AND 2000),
  CONSTRAINT hypothesis_rationale_chk CHECK (rationale IS NULL OR char_length(rationale) <= 20000),
  CONSTRAINT hypothesis_status_chk CHECK (status IN ('open', 'supported', 'refuted', 'parked')),
  CONSTRAINT hypothesis_confidence_chk CHECK (confidence IS NULL OR confidence IN ('low', 'medium', 'high')),
  CONSTRAINT hypothesis_links_chk CHECK (cardinality(supports) <= 100 AND cardinality(contradicts) <= 100),
  CONSTRAINT hypothesis_deleted_chk CHECK ((deleted_at IS NULL) = (deleted_by IS NULL))
);
CREATE INDEX IF NOT EXISTS hypothesis_case_idx ON forensic.hypothesis (tenant_id, collection_id, status, updated_at DESC) WHERE deleted_at IS NULL;
COMMENT ON TABLE forensic.hypothesis IS
  'A statement the analyst is testing, with a status (open, supported, refuted, parked), the findings that support it and the findings that contradict it.';

CREATE TABLE IF NOT EXISTS forensic.task (
  task_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  title text NOT NULL,
  detail text,
  status text NOT NULL DEFAULT 'open',
  assignee_subject text,
  due_at timestamptz,
  source_kind text,
  source_id uuid,
  completed_at timestamptz,
  completed_by text,
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_by text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by text,
  delete_reason text,
  CONSTRAINT task_collection_chk CHECK (collection_id <> '' AND length(collection_id) <= 128),
  CONSTRAINT task_title_chk CHECK (char_length(btrim(title)) BETWEEN 1 AND 200 AND title !~ '[[:cntrl:]]'),
  CONSTRAINT task_detail_chk CHECK (detail IS NULL OR char_length(detail) <= 20000),
  CONSTRAINT task_status_chk CHECK (status IN ('open', 'in_progress', 'done', 'cancelled')),
  CONSTRAINT task_source_chk CHECK (source_kind IS NULL OR source_kind IN ('turn', 'finding', 'hypothesis', 'note', 'evidence')),
  CONSTRAINT task_deleted_chk CHECK ((deleted_at IS NULL) = (deleted_by IS NULL))
);
CREATE INDEX IF NOT EXISTS task_case_idx ON forensic.task (tenant_id, collection_id, status, due_at) WHERE deleted_at IS NULL;
COMMENT ON TABLE forensic.task IS
  'Something to do in a case: a title, a status, an assignee and a due time, optionally tied to the answer, finding, hypothesis, note or evidence it came from.';

CREATE TABLE IF NOT EXISTS forensic.tag (
  tag_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  name text NOT NULL,
  name_key text GENERATED ALWAYS AS (forensic.search_normalize(btrim(name))) STORED,
  color text,
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT tag_collection_chk CHECK (collection_id <> '' AND length(collection_id) <= 128),
  CONSTRAINT tag_name_chk CHECK (char_length(btrim(name)) BETWEEN 1 AND 60 AND name !~ '[[:cntrl:]]'),
  CONSTRAINT tag_color_chk CHECK (color IS NULL OR color ~ '^#[0-9a-fA-F]{6}$'),
  UNIQUE (tenant_id, collection_id, name_key)
);

COMMENT ON TABLE forensic.tag IS
  'A label of a case. name_key is the name folded for comparison, so that two spellings of one word are one tag.';

CREATE TABLE IF NOT EXISTS forensic.tag_assignment (
  tag_id uuid NOT NULL REFERENCES forensic.tag (tag_id),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  object_kind text NOT NULL,
  object_id uuid NOT NULL,
  assigned_by text NOT NULL,
  assigned_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tag_id, object_kind, object_id),
  CONSTRAINT tag_assignment_kind_chk CHECK (object_kind IN ('thread', 'turn', 'note', 'finding', 'hypothesis', 'task', 'view', 'evidence'))
);
CREATE INDEX IF NOT EXISTS tag_assignment_object_idx ON forensic.tag_assignment (tenant_id, collection_id, object_kind, object_id);

COMMENT ON TABLE forensic.tag_assignment IS
  'One tag on one object of a case. A single table serves threads, turns, notes, findings, hypotheses, tasks, saved views and evidence items; removing a tag from an object deletes the row and writes an audit row.';

CREATE TABLE IF NOT EXISTS forensic.saved_view (
  view_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  owner_subject text NOT NULL,
  name text NOT NULL,
  kind text NOT NULL,
  params_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  visibility text NOT NULL DEFAULT 'private',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_by text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by text,
  delete_reason text,
  CONSTRAINT saved_view_collection_chk CHECK (collection_id <> '' AND length(collection_id) <= 128),
  CONSTRAINT saved_view_name_chk CHECK (char_length(btrim(name)) BETWEEN 1 AND 120 AND name !~ '[[:cntrl:]]'),
  CONSTRAINT saved_view_kind_chk CHECK (kind ~ '^[a-z][a-z0-9_]{1,40}$'),
  CONSTRAINT saved_view_params_chk CHECK (jsonb_typeof(params_json) = 'object' AND octet_length(params_json::text) <= 262144),
  CONSTRAINT saved_view_visibility_chk CHECK (visibility IN ('private', 'case')),
  CONSTRAINT saved_view_deleted_chk CHECK ((deleted_at IS NULL) = (deleted_by IS NULL))
);
CREATE INDEX IF NOT EXISTS saved_view_case_idx ON forensic.saved_view (tenant_id, collection_id, kind, updated_at DESC) WHERE deleted_at IS NULL;
COMMENT ON TABLE forensic.saved_view IS
  'The state of a view the analyst wants back (links, places, timeline, a table, an Ask filter): its kind and its parameters. Private to the owner unless shared with the case.';

CREATE TABLE IF NOT EXISTS forensic.annotation (
  annotation_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  target_kind text NOT NULL,
  target_key text NOT NULL,
  locator jsonb NOT NULL DEFAULT '{}'::jsonb,
  body text NOT NULL,
  created_by text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_by text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  deleted_by text,
  delete_reason text,
  CONSTRAINT annotation_collection_chk CHECK (collection_id <> '' AND length(collection_id) <= 128),
  CONSTRAINT annotation_target_kind_chk CHECK (target_kind ~ '^[a-z][a-z0-9_]{1,40}$'),
  CONSTRAINT annotation_target_key_chk CHECK (char_length(target_key) BETWEEN 1 AND 300),
  CONSTRAINT annotation_locator_chk CHECK (jsonb_typeof(locator) = 'object' AND octet_length(locator::text) <= 8192),
  CONSTRAINT annotation_body_chk CHECK (char_length(btrim(body)) BETWEEN 1 AND 4000),
  CONSTRAINT annotation_deleted_chk CHECK ((deleted_at IS NULL) = (deleted_by IS NULL))
);
CREATE INDEX IF NOT EXISTS annotation_target_idx ON forensic.annotation (tenant_id, collection_id, target_kind, target_key) WHERE deleted_at IS NULL;

COMMENT ON TABLE forensic.annotation IS
  'A note pinned to something in a case: a day on the timeline (target_kind timeline_day, target_key 2026-02-03), an event, a row of a file, an entity, a turn. Shared by every member of the case; replaces the pins kept in the browser.';

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
-- E. Authority and audit
-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS forensic.authority_record (
  authority_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  kind text NOT NULL,
  reference text NOT NULL,
  issuing_body text NOT NULL,
  scope text,
  valid_from date,
  valid_to date,
  notes text,
  attachments jsonb NOT NULL DEFAULT '[]'::jsonb,
  revoked_at timestamptz,
  revoked_by text,
  revoke_reason text,
  recorded_by text NOT NULL,
  recorded_at timestamptz NOT NULL DEFAULT now(),
  updated_by text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT authority_record_collection_chk CHECK (collection_id <> '' AND length(collection_id) <= 128),
  CONSTRAINT authority_record_kind_chk CHECK (kind IN ('warrant', 'court_order', 'consent', 'internal_approval', 'other')),
  CONSTRAINT authority_record_reference_chk CHECK (char_length(btrim(reference)) BETWEEN 1 AND 200 AND reference !~ '[[:cntrl:]]'),
  CONSTRAINT authority_record_body_chk CHECK (char_length(btrim(issuing_body)) BETWEEN 1 AND 200 AND issuing_body !~ '[[:cntrl:]]'),
  CONSTRAINT authority_record_scope_chk CHECK (scope IS NULL OR char_length(scope) <= 4000),
  CONSTRAINT authority_record_notes_chk CHECK (notes IS NULL OR char_length(notes) <= 20000),
  CONSTRAINT authority_record_dates_chk CHECK (valid_from IS NULL OR valid_to IS NULL OR valid_to >= valid_from),
  CONSTRAINT authority_record_attachments_chk CHECK (jsonb_typeof(attachments) = 'array' AND jsonb_array_length(attachments) <= 50),
  CONSTRAINT authority_record_revoked_chk CHECK ((revoked_at IS NULL) = (revoked_by IS NULL))
);
CREATE INDEX IF NOT EXISTS authority_record_case_idx ON forensic.authority_record (tenant_id, collection_id, recorded_at DESC);

COMMENT ON TABLE forensic.authority_record IS
  'The legal or administrative basis for working a case: a warrant, a court order, a consent, an internal approval, with its reference, issuing body, scope, validity dates and attachments (evidence ids). The status (active, expired, not yet valid, revoked) is computed from the dates and the revocation. It informs; it does not yet gate a capability.';

CREATE TABLE IF NOT EXISTS forensic.case_audit_event (
  event_id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  seq bigint NOT NULL DEFAULT 0,
  actor_subject text NOT NULL,
  actor_role text NOT NULL DEFAULT '',
  action text NOT NULL,
  object_kind text NOT NULL,
  object_id text NOT NULL DEFAULT '',
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  occurred_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  previous_sha256 text,
  event_sha256 text NOT NULL DEFAULT repeat('0', 64),
  CONSTRAINT case_audit_event_collection_chk CHECK (collection_id <> '' AND length(collection_id) <= 128),
  CONSTRAINT case_audit_event_action_chk CHECK (action ~ '^[a-z][a-z0-9_.]{1,80}$'),
  CONSTRAINT case_audit_event_details_chk CHECK (jsonb_typeof(details) = 'object'),
  CONSTRAINT case_audit_event_hash_chk CHECK (event_sha256 ~ '^[0-9a-f]{64}$' AND (previous_sha256 IS NULL OR previous_sha256 ~ '^[0-9a-f]{64}$')),
  UNIQUE (tenant_id, collection_id, seq)
);
CREATE INDEX IF NOT EXISTS case_audit_event_object_idx ON forensic.case_audit_event (tenant_id, collection_id, object_kind, object_id);
CREATE INDEX IF NOT EXISTS case_audit_event_action_idx ON forensic.case_audit_event (tenant_id, collection_id, action, seq DESC);

COMMENT ON TABLE forensic.case_audit_event IS
  'One row for every change the case layer makes: who (subject and role), what (action, object), when, and the SHA-256 chain that ties each row to the one before it in the same case. Append-only; the hashes are set by a trigger, not by the application.';

-- The chain is per case. SECURITY DEFINER because the trigger must read the tail of the chain whatever the caller's role may read; the search path is fixed. The time is the server's, never the caller's.
CREATE OR REPLACE FUNCTION forensic.prepare_case_audit_event()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = forensic, pg_catalog, pg_temp
AS $f$
DECLARE
  last_seq bigint;
  prior text;
BEGIN
  PERFORM pg_advisory_xact_lock(hashtextextended('case-audit' || E'\n' || NEW.tenant_id || E'\n' || NEW.collection_id, 0));
  SELECT e.seq, e.event_sha256 INTO last_seq, prior
  FROM forensic.case_audit_event e
  WHERE e.tenant_id = NEW.tenant_id AND e.collection_id = NEW.collection_id
  ORDER BY e.seq DESC
  LIMIT 1;
  NEW.seq := coalesce(last_seq, 0) + 1;
  NEW.previous_sha256 := prior;
  NEW.occurred_at := clock_timestamp();
  NEW.event_sha256 := encode(sha256(convert_to(concat_ws(E'\n',
    coalesce(prior, ''), NEW.seq::text, NEW.event_id::text, NEW.tenant_id, NEW.collection_id, NEW.actor_subject, NEW.actor_role,
    NEW.action, NEW.object_kind, NEW.object_id, NEW.details::text,
    to_char(NEW.occurred_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
  ), 'UTF8')), 'hex');
  RETURN NEW;
END;
$f$;

DROP TRIGGER IF EXISTS case_audit_event_prepare ON forensic.case_audit_event;
CREATE TRIGGER case_audit_event_prepare
  BEFORE INSERT ON forensic.case_audit_event
  FOR EACH ROW EXECUTE FUNCTION forensic.prepare_case_audit_event();

-- forensic.reject_append_only_mutation() is migration 008's; it refuses UPDATE and DELETE with SQLSTATE 55000.
DROP TRIGGER IF EXISTS case_audit_event_append_only ON forensic.case_audit_event;
CREATE TRIGGER case_audit_event_append_only
  BEFORE UPDATE OR DELETE ON forensic.case_audit_event
  FOR EACH ROW EXECUTE FUNCTION forensic.reject_append_only_mutation();

-- Recomputes the chain of one case and reports how many events it holds, whether every hash and link is right, and the first sequence number that is not.
CREATE OR REPLACE FUNCTION forensic.case_audit_chain_status(p_tenant text, p_case text)
RETURNS TABLE (events bigint, ok boolean, first_broken_seq bigint)
LANGUAGE plpgsql
STABLE
AS $f$
DECLARE
  e record;
  prior text := NULL;
  expected text;
  n bigint := 0;
  broken bigint := NULL;
BEGIN
  FOR e IN
    SELECT * FROM forensic.case_audit_event x WHERE x.tenant_id = p_tenant AND x.collection_id = p_case ORDER BY x.seq
  LOOP
    n := n + 1;
    expected := encode(sha256(convert_to(concat_ws(E'\n',
      coalesce(prior, ''), e.seq::text, e.event_id::text, e.tenant_id, e.collection_id, e.actor_subject, e.actor_role,
      e.action, e.object_kind, e.object_id, e.details::text,
      to_char(e.occurred_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
    ), 'UTF8')), 'hex');
    IF broken IS NULL AND (e.seq <> n OR e.previous_sha256 IS DISTINCT FROM prior OR e.event_sha256 <> expected) THEN
      broken := e.seq;
    END IF;
    prior := e.event_sha256;
  END LOOP;
  RETURN QUERY SELECT n, broken IS NULL, broken;
END;
$f$;

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
-- F. A finished turn cannot be changed
-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION forensic.guard_conversation_turn()
RETURNS trigger
LANGUAGE plpgsql
AS $f$
BEGIN
  IF OLD.state <> 'running' THEN
    RAISE EXCEPTION 'conversation_turn % is finished (%) and cannot be changed; run the question again to get a new turn', OLD.turn_id, OLD.state
      USING ERRCODE = '55000';
  END IF;
  IF NEW.turn_id IS DISTINCT FROM OLD.turn_id OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id OR NEW.collection_id IS DISTINCT FROM OLD.collection_id
     OR NEW.thread_id IS DISTINCT FROM OLD.thread_id OR NEW.idx IS DISTINCT FROM OLD.idx OR NEW.parent_turn_id IS DISTINCT FROM OLD.parent_turn_id
     OR NEW.kind IS DISTINCT FROM OLD.kind OR NEW.edit_of_turn_id IS DISTINCT FROM OLD.edit_of_turn_id OR NEW.rerun_of_turn_id IS DISTINCT FROM OLD.rerun_of_turn_id
     OR NEW.root_turn_id IS DISTINCT FROM OLD.root_turn_id OR NEW.question_text IS DISTINCT FROM OLD.question_text
     OR NEW.created_by IS DISTINCT FROM OLD.created_by OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
    RAISE EXCEPTION 'the identity and the question of conversation_turn % cannot be changed', OLD.turn_id USING ERRCODE = '55000';
  END IF;
  RETURN NEW;
END;
$f$;

DROP TRIGGER IF EXISTS conversation_turn_guard ON forensic.conversation_turn;
CREATE TRIGGER conversation_turn_guard
  BEFORE UPDATE ON forensic.conversation_turn
  FOR EACH ROW EXECUTE FUNCTION forensic.guard_conversation_turn();

DROP TRIGGER IF EXISTS conversation_turn_no_delete ON forensic.conversation_turn;
CREATE TRIGGER conversation_turn_no_delete
  BEFORE DELETE ON forensic.conversation_turn
  FOR EACH ROW EXECUTE FUNCTION forensic.reject_append_only_mutation();

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
-- G. Row-level security
-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
-- Building blocks, written out in every policy so that each can be read alone:
--   tenant      tenant_id = current_setting('app.tenant_id', true)
--   role        forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true))
--   writes      role NOT IN ('none', 'viewer', 'auditor')

ALTER TABLE forensic.case_member ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.conversation_thread ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.conversation_turn ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.turn_feedback ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.case_data_version ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.case_note ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.finding ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.hypothesis ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.task ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.tag ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.tag_assignment ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.saved_view ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.annotation ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.authority_record ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.case_audit_event ENABLE ROW LEVEL SECURITY;

-- members: readable by every participant of the case; managed by the owner, a lead, an admin, or by anyone while the case is still open (the first row)
DROP POLICY IF EXISTS case_member_select ON forensic.case_member;
CREATE POLICY case_member_select ON forensic.case_member FOR SELECT USING (
  tenant_id = current_setting('app.tenant_id', true)
  AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) <> 'none');
DROP POLICY IF EXISTS case_member_insert ON forensic.case_member;
CREATE POLICY case_member_insert ON forensic.case_member FOR INSERT WITH CHECK (
  tenant_id = current_setting('app.tenant_id', true)
  AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) IN ('owner', 'lead', 'open'));
DROP POLICY IF EXISTS case_member_update ON forensic.case_member;
CREATE POLICY case_member_update ON forensic.case_member FOR UPDATE
  USING (tenant_id = current_setting('app.tenant_id', true)
    AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) IN ('owner', 'lead'))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
DROP POLICY IF EXISTS case_member_delete ON forensic.case_member;
CREATE POLICY case_member_delete ON forensic.case_member FOR DELETE USING (
  tenant_id = current_setting('app.tenant_id', true)
  AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) IN ('owner', 'lead'));

-- threads
DROP POLICY IF EXISTS conversation_thread_select ON forensic.conversation_thread;
CREATE POLICY conversation_thread_select ON forensic.conversation_thread FOR SELECT USING (
  tenant_id = current_setting('app.tenant_id', true)
  AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) <> 'none'
  AND (owner_subject = current_setting('app.subject_id', true)
       OR visibility = 'case'
       OR (visibility = 'selected' AND current_setting('app.subject_id', true) = ANY (shared_with))
       OR forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) = 'auditor'
       OR current_setting('app.actor_role', true) = 'admin'));
DROP POLICY IF EXISTS conversation_thread_insert ON forensic.conversation_thread;
CREATE POLICY conversation_thread_insert ON forensic.conversation_thread FOR INSERT WITH CHECK (
  tenant_id = current_setting('app.tenant_id', true)
  AND owner_subject = current_setting('app.subject_id', true)
  AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) NOT IN ('none', 'viewer', 'auditor'));
DROP POLICY IF EXISTS conversation_thread_update ON forensic.conversation_thread;
CREATE POLICY conversation_thread_update ON forensic.conversation_thread FOR UPDATE
  USING (tenant_id = current_setting('app.tenant_id', true)
    AND (owner_subject = current_setting('app.subject_id', true) OR current_setting('app.actor_role', true) = 'admin'))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

-- turns follow their thread: a turn is visible when its thread is (the thread's own policy runs inside the test), and is written only by the thread's owner
DROP POLICY IF EXISTS conversation_turn_select ON forensic.conversation_turn;
CREATE POLICY conversation_turn_select ON forensic.conversation_turn FOR SELECT USING (
  tenant_id = current_setting('app.tenant_id', true)
  AND EXISTS (SELECT 1 FROM forensic.conversation_thread t WHERE t.thread_id = conversation_turn.thread_id));
DROP POLICY IF EXISTS conversation_turn_insert ON forensic.conversation_turn;
CREATE POLICY conversation_turn_insert ON forensic.conversation_turn FOR INSERT WITH CHECK (
  tenant_id = current_setting('app.tenant_id', true)
  AND created_by = current_setting('app.subject_id', true)
  AND EXISTS (SELECT 1 FROM forensic.conversation_thread t
              WHERE t.thread_id = conversation_turn.thread_id AND t.owner_subject = current_setting('app.subject_id', true) AND t.hidden_at IS NULL));
DROP POLICY IF EXISTS conversation_turn_update ON forensic.conversation_turn;
CREATE POLICY conversation_turn_update ON forensic.conversation_turn FOR UPDATE
  USING (tenant_id = current_setting('app.tenant_id', true)
    AND EXISTS (SELECT 1 FROM forensic.conversation_thread t
                WHERE t.thread_id = conversation_turn.thread_id AND t.owner_subject = current_setting('app.subject_id', true)))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

-- feedback: anyone who can read the turn and is not an auditor, one row each, changeable by its author only
DROP POLICY IF EXISTS turn_feedback_select ON forensic.turn_feedback;
CREATE POLICY turn_feedback_select ON forensic.turn_feedback FOR SELECT USING (
  tenant_id = current_setting('app.tenant_id', true)
  AND EXISTS (SELECT 1 FROM forensic.conversation_turn t WHERE t.turn_id = turn_feedback.turn_id));
DROP POLICY IF EXISTS turn_feedback_insert ON forensic.turn_feedback;
CREATE POLICY turn_feedback_insert ON forensic.turn_feedback FOR INSERT WITH CHECK (
  tenant_id = current_setting('app.tenant_id', true)
  AND subject_id = current_setting('app.subject_id', true)
  AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) NOT IN ('none', 'auditor')
  AND EXISTS (SELECT 1 FROM forensic.conversation_turn t WHERE t.turn_id = turn_feedback.turn_id));
DROP POLICY IF EXISTS turn_feedback_update ON forensic.turn_feedback;
CREATE POLICY turn_feedback_update ON forensic.turn_feedback FOR UPDATE
  USING (tenant_id = current_setting('app.tenant_id', true) AND subject_id = current_setting('app.subject_id', true))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true) AND subject_id = current_setting('app.subject_id', true));

-- case content: every participant reads; a writer role writes
DO $policies$
DECLARE
  t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['case_data_version', 'case_note', 'finding', 'hypothesis', 'task', 'tag', 'tag_assignment', 'annotation', 'authority_record']
  LOOP
    EXECUTE format('DROP POLICY IF EXISTS %I ON forensic.%I', t || '_select', t);
    EXECUTE format($p$CREATE POLICY %I ON forensic.%I FOR SELECT USING (
      tenant_id = current_setting('app.tenant_id', true)
      AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) <> 'none')$p$, t || '_select', t);
    EXECUTE format('DROP POLICY IF EXISTS %I ON forensic.%I', t || '_insert', t);
    EXECUTE format($p$CREATE POLICY %I ON forensic.%I FOR INSERT WITH CHECK (
      tenant_id = current_setting('app.tenant_id', true)
      AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) NOT IN ('none', 'viewer', 'auditor'))$p$, t || '_insert', t);
  END LOOP;
  FOREACH t IN ARRAY ARRAY['case_note', 'finding', 'hypothesis', 'task', 'annotation', 'authority_record']
  LOOP
    EXECUTE format('DROP POLICY IF EXISTS %I ON forensic.%I', t || '_update', t);
    EXECUTE format($p$CREATE POLICY %I ON forensic.%I FOR UPDATE
      USING (tenant_id = current_setting('app.tenant_id', true)
        AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) NOT IN ('none', 'viewer', 'auditor'))
      WITH CHECK (tenant_id = current_setting('app.tenant_id', true))$p$, t || '_update', t);
  END LOOP;
END
$policies$;

-- tags can be renamed or recoloured by a writer; an assignment is removed by a writer
DROP POLICY IF EXISTS tag_update ON forensic.tag;
CREATE POLICY tag_update ON forensic.tag FOR UPDATE
  USING (tenant_id = current_setting('app.tenant_id', true)
    AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) NOT IN ('none', 'viewer', 'auditor'))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));
DROP POLICY IF EXISTS tag_assignment_delete ON forensic.tag_assignment;
CREATE POLICY tag_assignment_delete ON forensic.tag_assignment FOR DELETE USING (
  tenant_id = current_setting('app.tenant_id', true)
  AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) NOT IN ('none', 'viewer', 'auditor'));

-- saved views: private to their owner unless shared with the case; auditors and admins read all
DROP POLICY IF EXISTS saved_view_select ON forensic.saved_view;
CREATE POLICY saved_view_select ON forensic.saved_view FOR SELECT USING (
  tenant_id = current_setting('app.tenant_id', true)
  AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) <> 'none'
  AND (owner_subject = current_setting('app.subject_id', true) OR visibility = 'case'
       OR forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) = 'auditor'
       OR current_setting('app.actor_role', true) = 'admin'));
DROP POLICY IF EXISTS saved_view_insert ON forensic.saved_view;
CREATE POLICY saved_view_insert ON forensic.saved_view FOR INSERT WITH CHECK (
  tenant_id = current_setting('app.tenant_id', true)
  AND owner_subject = current_setting('app.subject_id', true)
  AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) NOT IN ('none', 'viewer', 'auditor'));
DROP POLICY IF EXISTS saved_view_update ON forensic.saved_view;
CREATE POLICY saved_view_update ON forensic.saved_view FOR UPDATE
  USING (tenant_id = current_setting('app.tenant_id', true)
    AND (owner_subject = current_setting('app.subject_id', true) OR current_setting('app.actor_role', true) = 'admin'))
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true));

-- the audit trail is read by the owner, a lead, an auditor and an admin (or by anyone while the case is open); any participant's change may add a row
DROP POLICY IF EXISTS case_audit_event_select ON forensic.case_audit_event;
CREATE POLICY case_audit_event_select ON forensic.case_audit_event FOR SELECT USING (
  tenant_id = current_setting('app.tenant_id', true)
  AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) IN ('owner', 'lead', 'auditor', 'open'));
DROP POLICY IF EXISTS case_audit_event_insert ON forensic.case_audit_event;
CREATE POLICY case_audit_event_insert ON forensic.case_audit_event FOR INSERT WITH CHECK (
  tenant_id = current_setting('app.tenant_id', true)
  AND forensic.case_role(tenant_id, collection_id, current_setting('app.subject_id', true)) <> 'none');

-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
-- H. Grants for the application role: reads and inserts everywhere, column-level updates where a record is edited, deletes only where a row is a link, never on a record.
-- The read-only analyst role gets nothing: these tables are not analyst views and the engine must never see them.
-- ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------

DO $grants$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    GRANT SELECT, INSERT ON
      forensic.case_member, forensic.conversation_thread, forensic.conversation_turn, forensic.turn_feedback, forensic.case_data_version,
      forensic.case_note, forensic.finding, forensic.hypothesis, forensic.task, forensic.tag, forensic.tag_assignment, forensic.saved_view,
      forensic.annotation, forensic.authority_record, forensic.case_audit_event
    TO forensic_runtime;
    GRANT DELETE ON forensic.case_member, forensic.tag_assignment TO forensic_runtime;
    GRANT UPDATE (role, display_name, updated_by, updated_at) ON forensic.case_member TO forensic_runtime;
    GRANT UPDATE (title, title_source, pinned, archived, visibility, shared_with, turn_count, first_question, last_question, last_headline, last_outcome, last_turn_at,
                  hidden_at, hidden_by, hide_reason, updated_at) ON forensic.conversation_thread TO forensic_runtime;
    GRANT UPDATE (resolved_text, language, frame_json, resolution_json, state, outcome, headline, answer_text, shape, row_count, query_text, query_hash, result_digest, figures_json,
                  evidence_refs, snapshot, snapshot_state, snapshot_bytes, snapshot_sha256, engine_version, model_id, prompt_id, data_version_hash, timings_json, diff_summary,
                  error_reason, request_id, completed_at) ON forensic.conversation_turn TO forensic_runtime;
    GRANT UPDATE (verdict, note, updated_at) ON forensic.turn_feedback TO forensic_runtime;
    GRANT UPDATE (category, title, body, pinned, updated_by, updated_at, deleted_at, deleted_by, delete_reason) ON forensic.case_note TO forensic_runtime;
    GRANT UPDATE (title, body, basis, review_state, reviewed_by, reviewed_at, review_note, sort_order, evidence_refs, updated_by, updated_at, deleted_at, deleted_by, delete_reason)
      ON forensic.finding TO forensic_runtime;
    GRANT UPDATE (statement, rationale, status, confidence, supports, contradicts, updated_by, updated_at, deleted_at, deleted_by, delete_reason) ON forensic.hypothesis TO forensic_runtime;
    GRANT UPDATE (title, detail, status, assignee_subject, due_at, completed_at, completed_by, updated_by, updated_at, deleted_at, deleted_by, delete_reason) ON forensic.task TO forensic_runtime;
    GRANT UPDATE (name, color) ON forensic.tag TO forensic_runtime;
    GRANT UPDATE (name, params_json, visibility, updated_by, updated_at, deleted_at, deleted_by, delete_reason) ON forensic.saved_view TO forensic_runtime;
    GRANT UPDATE (body, locator, updated_by, updated_at, deleted_at, deleted_by, delete_reason) ON forensic.annotation TO forensic_runtime;
    GRANT UPDATE (kind, reference, issuing_body, scope, valid_from, valid_to, notes, attachments, revoked_at, revoked_by, revoke_reason, updated_by, updated_at)
      ON forensic.authority_record TO forensic_runtime;
    GRANT EXECUTE ON FUNCTION forensic.case_audit_chain_status(text, text) TO forensic_runtime;
  END IF;
END
$grants$;

COMMIT;
