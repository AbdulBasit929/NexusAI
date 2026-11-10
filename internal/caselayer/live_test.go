package caselayer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// testScope is the stand-in for the API's principal: who is asking comes from headers the specs set.
func testScope(r *http.Request, _ string) (Scope, error) {
	sc := Scope{TenantID: r.Header.Get("X-Test-Tenant"), Subject: r.Header.Get("X-Test-Subject"), ActorRole: r.Header.Get("X-Test-Role"), BoundCase: r.Header.Get("X-Test-Bound")}
	if sc.TenantID == "" || sc.Subject == "" {
		return Scope{}, &Error{Status: http.StatusUnauthorized, Code: "unauthenticated", Message: "no tenant or subject"}
	}
	return sc, nil
}

// These specs run against the throwaway database of tools/dev/live-window/rehearse-034.sh (KEEP=1): migrations 001 to 032 and 034 applied, the application role forensic_runtime for the
// layer itself (so the row-level policies are in force) and the owner for fixtures. They are skipped when CASELAYER_TEST_DSN is not set.
var _ = Describe("the case layer on a throwaway database", Ordered, func() {
	var (
		ctx      = context.Background()
		pool     *pgxpool.Pool
		owner    *pgxpool.Pool
		svc      *Service
		eng      *scriptedEngine
		srv      *httptest.Server
		tenant   = "t-" + uuid.NewString()[:8]
		caseOpen = "open-" + uuid.NewString()[:8]
		caseCtl  = "ctl-" + uuid.NewString()[:8]
		caseData = "data-" + uuid.NewString()[:8]
		caseLog  = "audit-" + uuid.NewString()[:8]
	)

	call := func(c caller, method, path string, body any) (int, map[string]any) {
		resp, raw := c.do(srv.URL, tenant, method, path, body, "")
		return resp.StatusCode, jsonMap(raw)
	}
	newThread := func(c caller, caseID, title string) string {
		status, out := call(c, "POST", "/conversations", map[string]any{"case_id": caseID, "title": title})
		Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
		return jstr(out, "thread_id")
	}
	runQ := func(c caller, threadID, question string) (int, map[string]any) {
		return call(c, "POST", "/conversations/"+threadID+"/turns", map[string]any{"question": question})
	}
	turnID := func(out map[string]any) string { return jstr(out, "turn", "turn_id") }
	addEvidence := func(caseID, name string) {
		sum := sha256.Sum256([]byte(name + uuid.NewString()))
		_, err := owner.Exec(ctx, `INSERT INTO forensic.evidence_items (tenant_id, collection_id, user_id, original_filename, source_file, sha256, size_bytes) VALUES ($1, $2, 'test', $3, $3, $4, 10)`,
			tenant, caseID, name, hex.EncodeToString(sum[:]))
		Expect(err).NotTo(HaveOccurred())
	}
	makeCase := func(caseID string) {
		_, err := owner.Exec(ctx, `INSERT INTO forensic.case_record (tenant_id, collection_id, display_name) VALUES ($1, $2, $3)`, tenant, caseID, "Spec case "+caseID)
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeAll(func() {
		dsn := os.Getenv("CASELAYER_TEST_DSN")            //nolint:forbidigo // opt-in switch for a throwaway-database spec
		ownerDSN := os.Getenv("CASELAYER_TEST_OWNER_DSN") //nolint:forbidigo // set by the rehearsal script only
		if dsn == "" || ownerDSN == "" {
			Skip("needs the throwaway database of tools/dev/live-window/rehearse-034.sh (KEEP=1)")
		}
		cfg, err := pgxpool.ParseConfig(dsn)
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.ConnConfig.Host).To(Equal("127.0.0.1"))
		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		Expect(err).NotTo(HaveOccurred())
		ocfg, err := pgxpool.ParseConfig(ownerDSN)
		Expect(err).NotTo(HaveOccurred())
		Expect(ocfg.ConnConfig.Host).To(Equal("127.0.0.1"))
		owner, err = pgxpool.NewWithConfig(ctx, ocfg)
		Expect(err).NotTo(HaveOccurred())
		var marker bool
		Expect(owner.QueryRow(ctx, "SELECT to_regclass('public.nx_throwaway_marker') IS NOT NULL").Scan(&marker)).To(Succeed())
		Expect(marker).To(BeTrue(), "this is not a throwaway database")
		var current string
		Expect(pool.QueryRow(ctx, "SELECT current_user").Scan(&current)).To(Succeed())
		Expect(current).To(Equal("forensic_runtime"), "the layer must be tested as the application role, under row-level security")

		eng = &scriptedEngine{}
		eng.configure(ranked1, nil, 0)
		svc = NewService(NewStore(pool), eng, Config{Heartbeat: 20 * time.Millisecond, TurnTimeout: time.Minute, EngineVersion: "spec-build"})
		mux := http.NewServeMux()
		(&Handler{Svc: svc, Scope: testScope}).Register(mux)
		srv = httptest.NewServer(mux)
		for _, c := range []string{caseOpen, caseCtl, caseLog} {
			makeCase(c)
		}
		addEvidence(caseData, "calls.csv")
	})

	AfterAll(func() {
		if srv != nil {
			srv.Close()
		}
		if pool != nil {
			pool.Close()
		}
		if owner != nil {
			owner.Close()
		}
	})

	BeforeEach(func() {
		if eng != nil {
			eng.configure(ranked1, nil, 0)
		}
	})

	// ---- the rail -------------------------------------------------------------------------------------------------------------------------------------------------------------

	Describe("conversations as a rail", func() {
		It("refuses to start a conversation in a case that does not exist", func() {
			status, out := call(alice, "POST", "/conversations", map[string]any{"case_id": "no-such-case-" + uuid.NewString()[:6]})
			Expect(status).To(Equal(http.StatusNotFound), fmt.Sprint(out))
			Expect(out["code"]).To(Equal("not_found"))
			status, _ = call(alice, "POST", "/conversations", map[string]any{"title": "no case"})
			Expect(status).To(Equal(http.StatusBadRequest))
		})

		It("starts, names, pins, archives and pages conversations, pinned first", func() {
			ids := map[string]string{}
			for _, n := range []string{"t1", "t2", "t3", "t4", "t5"} {
				status, out := call(alice, "POST", "/conversations", map[string]any{"case_id": caseOpen, "title": n})
				Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
				Expect(out["is_owner"]).To(BeTrue())
				Expect(out["display_title"]).To(Equal(n))
				ids[n] = jstr(out, "thread_id")
				status, tags := call(alice, "PUT", "/cases/"+caseOpen+"/objects/thread/"+ids[n]+"/tags", map[string]any{"tags": []string{"railtest"}})
				Expect(status).To(Equal(http.StatusOK), fmt.Sprint(tags))
			}
			status, out := call(alice, "PATCH", "/conversations/"+ids["t3"], map[string]any{"pinned": true})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			Expect(out["pinned"]).To(BeTrue())
			status, out = call(alice, "PATCH", "/conversations/"+ids["t5"], map[string]any{"archived": true})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))

			var seen []string
			path := "/conversations?case_id=" + caseOpen + "&tag=railtest&limit=2"
			next := path
			for page := 0; page < 6; page++ {
				status, out = call(alice, "GET", next, nil)
				Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
				for _, it := range out["items"].([]any) {
					seen = append(seen, jstr(it, "thread_id"))
				}
				cursor, _ := out["next_cursor"].(string)
				if cursor == "" {
					break
				}
				next = path + "&cursor=" + cursor
			}
			Expect(seen).To(HaveLen(4), "the archived one is left out")
			Expect(seen[0]).To(Equal(ids["t3"]), "pinned first")
			unique := map[string]bool{}
			for _, id := range seen {
				Expect(unique[id]).To(BeFalse(), "no thread appears on two pages")
				unique[id] = true
			}
			_, out = call(alice, "GET", path+"&archived=true", nil)
			Expect(jlen(out, "items")).To(Equal(1))
			Expect(jstr(out, "items", "0", "thread_id")).To(Equal(ids["t5"]))
			_, out = call(alice, "GET", "/conversations?case_id="+caseOpen+"&tag=railtest&archived=all&limit=10", nil)
			Expect(jlen(out, "items")).To(Equal(5))
			_, out = call(alice, "GET", path+"&pinned=true", nil)
			Expect(jlen(out, "items")).To(Equal(1))
			status, out = call(alice, "PATCH", "/conversations/"+ids["t1"], map[string]any{"title": nil})
			Expect(status).To(Equal(http.StatusOK))
			Expect(out["title_source"]).To(Equal("auto"))
			Expect(out["display_title"]).To(Equal("New conversation"))
			status, _ = call(alice, "PATCH", "/conversations/"+ids["t1"], map[string]any{})
			Expect(status).To(Equal(http.StatusBadRequest))
			status, _ = call(alice, "PATCH", "/conversations/"+ids["t1"], map[string]any{"owner": "bob"})
			Expect(status).To(Equal(http.StatusBadRequest))
		})

		It("hides a conversation instead of deleting it, keeps its turns, and the audit says who and why", func() {
			tid := newThread(alice, caseOpen, "to hide")
			status, out := runQ(alice, tid, "Which numbers made the most calls?")
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
			status, _ = call(alice, "PATCH", "/conversations/"+tid, map[string]any{"visibility": "case"})
			Expect(status).To(Equal(http.StatusOK))
			status, _ = call(bob, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusOK), "shared with the case, bob reads it, until it is hidden")
			status, _ = call(alice, "DELETE", "/conversations/"+tid+"?reason=duplicate+of+another+question", nil)
			Expect(status).To(Equal(http.StatusNoContent))
			_, list := call(alice, "GET", "/conversations?case_id="+caseOpen+"&limit=100", nil)
			for _, it := range list["items"].([]any) {
				Expect(jstr(it, "thread_id")).NotTo(Equal(tid), "a hidden conversation is not in the list")
			}
			_, list = call(alice, "GET", "/conversations?case_id="+caseOpen+"&limit=100&include_hidden=true", nil)
			found := false
			for _, it := range list["items"].([]any) {
				if jstr(it, "thread_id") == tid {
					found = true
					Expect(it.(map[string]any)["hidden"]).To(BeTrue())
				}
			}
			Expect(found).To(BeTrue(), "its owner can still list it")
			status, th := call(alice, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(jlen(th, "turns")).To(Equal(1), "the turns are kept")
			status, _ = call(bob, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusNotFound), "a hidden conversation is gone for everyone but its owner, an administrator and the auditors")
			status, _ = call(bob, "GET", "/turns/"+jstr(th, "turns", "0", "turn_id"), nil)
			Expect(status).To(Equal(http.StatusNotFound), "and so are its turns")
			status, _ = call(root, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusOK))
			_, aud := call(alice, "GET", "/cases/"+caseOpen+"/audit?action=thread.hidden&object_id="+tid, nil)
			Expect(jlen(aud, "items")).To(Equal(1))
			Expect(jstr(aud, "items", "0", "details", "reason")).To(Equal("duplicate of another question"))
			Expect(jstr(aud, "items", "0", "actor")).To(Equal("alice"))
			status, back := call(alice, "POST", "/conversations/"+tid+"/restore", nil)
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(back))
			Expect(back["hidden"]).To(BeFalse())
			status, _ = runQ(alice, tid, "Which numbers made the most calls?")
			Expect(status).To(Equal(http.StatusCreated))
		})
	})

	// ---- a turn ----------------------------------------------------------------------------------------------------------------------------------------------------------------

	Describe("running a question", func() {
		It("runs it, keeps the answer, the digest and the data version, and a retry returns the same turn", func() {
			tid := newThread(alice, caseOpen, "first")
			before := eng.callCount()
			status, out := call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "Which numbers made the most calls?", "client_request_id": "req-run-1"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
			turn := out["turn"].(map[string]any)
			Expect(turn["state"]).To(Equal("completed"))
			Expect(turn["outcome"]).To(Equal("answered"))
			Expect(turn["idx"]).To(BeEquivalentTo(1))
			Expect(jstr(turn, "headline")).To(HavePrefix("3 numbers"))
			Expect(turn["snapshot_state"]).To(Equal("full"))
			Expect(jstr(turn, "data_version_hash")).To(MatchRegexp(`^[0-9a-f]{64}$`))
			Expect(jstr(turn, "query_hash")).To(MatchRegexp(`^[0-9a-f]{64}$`))
			Expect(jstr(turn, "answer", "answer", "answer")).To(HavePrefix("3 numbers"))
			Expect(turn["live_tokens"]).To(BeTrue())
			Expect(jstr(turn, "engine_version")).To(Equal("test-1"))
			Expect(jstr(turn, "model")).To(Equal("test-model"))
			Expect(jlen(turn, "evidence_refs")).To(Equal(1))
			Expect(jget(turn, "frame", "numbers", "0")).To(Equal("923001234567"))
			Expect(jget(turn, "resolution", "method")).To(Equal("none"))
			for _, key := range []string{"created_at", "started_at", "completed_at"} {
				at, err := time.Parse(time.RFC3339Nano, jstr(turn, key))
				Expect(err).NotTo(HaveOccurred(), key)
				_, offset := at.Zone()
				Expect(offset).To(Equal(0), key+" is in UTC")
			}
			Expect(eng.callCount()).To(Equal(before + 1))
			id := turnID(out)

			status, th := call(alice, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(th["turn_count"]).To(BeEquivalentTo(1))
			Expect(jstr(th, "last_headline")).To(HavePrefix("3 numbers"))
			Expect(jstr(th, "first_question")).To(Equal("Which numbers made the most calls?"))
			Expect(jstr(th, "last_outcome")).To(Equal("answered"))
			Expect(jstr(th, "active_turn_id")).To(Equal(id))
			Expect(jlen(th, "turns")).To(Equal(1))
			Expect(jstr(th, "turns", "0", "answer", "answer", "answer")).To(HavePrefix("3 numbers"), "the latest answer comes with the thread")

			status, one := call(alice, "GET", "/turns/"+id, nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(jstr(one, "query")).To(ContainSubstring("SELECT src AS label"))
			Expect(one["pinned_as"]).To(BeNil())
			_, light := call(alice, "GET", "/turns/"+id+"?answer=false", nil)
			Expect(light["answer"]).To(BeNil())

			status, again := call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "Which numbers made the most calls?", "client_request_id": "req-run-1"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(again))
			Expect(again["existing"]).To(BeTrue())
			Expect(turnID(again)).To(Equal(id))
			Expect(eng.callCount()).To(Equal(before+1), "the engine was not asked again")
			status, _ = call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "x", "client_request_id": "bad id!"})
			Expect(status).To(Equal(http.StatusBadRequest))
			status, _ = call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "   "})
			Expect(status).To(Equal(http.StatusBadRequest))
			status, _ = call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "x", "parent_turn_id": id, "edit_of_turn_id": id})
			Expect(status).To(Equal(http.StatusBadRequest))
		})

		It("streams the phases in order as NDJSON, with a heartbeat while the engine works", func() {
			tid := newThread(alice, caseOpen, "stream")
			eng.configure(ranked1, []string{"writing_query", "running"}, 150*time.Millisecond)
			resp, raw := alice.do(srv.URL, tenant, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "Which numbers made the most calls?"}, "application/x-ndjson")
			Expect(resp.StatusCode).To(Equal(http.StatusOK), string(raw))
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/x-ndjson"))
			events := ndjsonEvents(raw)
			var phases []string
			beats := 0
			prev := -1.0
			for _, e := range events {
				switch e["event"] {
				case "phase":
					phases = append(phases, e["phase"].(string))
				case "heartbeat":
					beats++
				}
				at := e["at_ms"].(float64)
				Expect(at).To(BeNumerically(">=", prev), "the clock does not run backwards")
				prev = at
			}
			Expect(phases).To(Equal([]string{"accepted", "engine", "engine.writing_query", "engine.running", "saving"}))
			Expect(beats).To(BeNumerically(">=", 2))
			Expect(events[len(events)-2]["event"]).To(Equal("turn"))
			Expect(events[len(events)-1]["event"]).To(Equal("done"))
			Expect(jstr(events[len(events)-2], "turn", "state")).To(Equal("completed"))
			Expect(jstr(events[len(events)-2], "turn", "answer", "answer", "answer")).To(HavePrefix("3 numbers"))
		})

		It("streams the same events as server-sent events", func() {
			tid := newThread(alice, caseOpen, "sse")
			resp, raw := alice.do(srv.URL, tenant, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "Which numbers made the most calls?"}, "text/event-stream")
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(resp.Header.Get("Content-Type")).To(Equal("text/event-stream"))
			body := string(raw)
			Expect(body).To(ContainSubstring("event: phase\ndata: {"))
			Expect(body).To(ContainSubstring("event: turn\ndata: {"))
			Expect(body).To(HaveSuffix("event: done\ndata: " + body[strings.LastIndex(body, "data: ")+6:]))
		})

		It("records a failed engine call as a failed turn, in a plain call and in a stream", func() {
			tid := newThread(alice, caseOpen, "failing")
			eng.configure(func(EngineRequest) (int, []byte) {
				return http.StatusBadRequest, []byte(`{"error":"collection_id is required"}`)
			}, nil, 0)
			status, out := runQ(alice, tid, "Which numbers made the most calls?")
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
			Expect(jstr(out, "turn", "state")).To(Equal("failed"))
			Expect(jstr(out, "turn", "outcome")).To(Equal("failed"))
			Expect(jstr(out, "turn", "error")).To(ContainSubstring("collection_id is required"))
			resp, raw := alice.do(srv.URL, tenant, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "And again?"}, "application/x-ndjson")
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			events := ndjsonEvents(raw)
			Expect(events[len(events)-2]["event"]).To(Equal("error"))
			Expect(events[len(events)-2]["code"]).To(Equal("engine_failed"))
			Expect(jstr(events[len(events)-2], "turn", "state")).To(Equal("failed"))
			_, th := call(alice, "GET", "/conversations/"+tid, nil)
			Expect(jlen(th, "turns")).To(Equal(2), "a failure is part of the history")
			status, _ = call(alice, "POST", "/turns/"+turnID(out)+"/pin", nil)
			Expect(status).To(Equal(http.StatusConflict), "a failed question cannot be pinned")
		})

		It("passes the earlier turns to the engine, and resolves a follow-up only when the role is on and the number lock holds", func() {
			tid := newThread(alice, caseOpen, "follow-ups")
			_, a := runQ(alice, tid, "How many calls did 923001234567 make in February 2026?")
			q1 := turnID(a)
			status, b := runQ(alice, tid, "and for March?")
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(b))
			Expect(jstr(b, "turn", "parent_turn_id")).To(Equal(q1))
			last := eng.lastCall()
			Expect(last.History).To(HaveLen(1))
			Expect(last.History[0].Question).To(ContainSubstring("923001234567"))
			Expect(string(last.History[0].Frame)).To(ContainSubstring("question_seen"))
			Expect(last.Question).To(Equal("and for March?"), "with the resolver off the message goes as typed")

			svc.FollowUp = resolverFunc(func(context.Context, FollowUpInput) (FollowUpResult, error) {
				return FollowUpResult{Question: "How many calls did 923001234567 make in March 2026?", Inherited: []string{"number"}, Model: "m"}, nil
			})
			DeferCleanup(func() { svc.FollowUp = nil })
			_, c := runQ(alice, tid, "and for March?")
			Expect(eng.lastCall().Question).To(Equal("How many calls did 923001234567 make in March 2026?"))
			Expect(eng.lastCall().Original).To(Equal("and for March?"))
			Expect(jstr(c, "turn", "resolved_question")).To(Equal("How many calls did 923001234567 make in March 2026?"))
			Expect(jstr(c, "turn", "resolution", "method")).To(Equal("model"))
			Expect(jstr(c, "turn", "question")).To(Equal("and for March?"), "the question as typed is kept")

			svc.FollowUp = resolverFunc(func(context.Context, FollowUpInput) (FollowUpResult, error) {
				return FollowUpResult{Question: "How many calls did 923009999999 make in March 2026?"}, nil
			})
			_, d := runQ(alice, tid, "and for March?")
			Expect(eng.lastCall().Question).To(Equal("and for March?"), "a question that adds a number is not believed")
			Expect(jstr(d, "turn", "resolution", "method")).To(Equal("rejected"))
			Expect(d["turn"].(map[string]any)["resolved_question"]).To(BeNil())
		})

		It("edits a question as a sibling, forks a conversation, and walks the ancestry", func() {
			tid := newThread(alice, caseOpen, "branches")
			_, a := runQ(alice, tid, "Which numbers made the most calls?")
			_, b := runQ(alice, tid, "And in March?")
			id1, id2 := turnID(a), turnID(b)
			status, e := call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "And in April?", "edit_of_turn_id": id2})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(e))
			Expect(jstr(e, "turn", "kind")).To(Equal("edit"))
			Expect(jstr(e, "turn", "parent_turn_id")).To(Equal(id1), "an edit is a sibling of the question it replaces")
			Expect(jstr(e, "turn", "edit_of_turn_id")).To(Equal(id2))
			_, th := call(alice, "GET", "/conversations/"+tid, nil)
			Expect(jlen(th, "turns")).To(Equal(3))

			status, f := call(alice, "POST", "/turns/"+id1+"/fork", map[string]any{"title": "Another line"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(f))
			Expect(jstr(f, "forked_from", "turn_id")).To(Equal(id1))
			Expect(f["turn_count"]).To(BeEquivalentTo(0))
			fid := jstr(f, "thread_id")
			_, c := runQ(alice, fid, "And in May?")
			Expect(jstr(c, "turn", "parent_turn_id")).To(Equal(id1), "the first turn of a fork continues from the turn that was forked")
			Expect(eng.lastCall().History).To(HaveLen(1))
			status, anc := call(alice, "GET", "/turns/"+turnID(c)+"/ancestry", nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(jlen(anc, "items")).To(Equal(2))
			Expect(jstr(anc, "items", "0", "turn_id")).To(Equal(id1))

			status, _ = runQ(bob, tid, "Can I add to this?")
			Expect(status).To(Equal(http.StatusNotFound), "a private conversation is not there for anyone else")
			status, _ = call(alice, "PATCH", "/conversations/"+tid, map[string]any{"visibility": "case"})
			Expect(status).To(Equal(http.StatusOK))
			status, out := runQ(bob, tid, "Can I add to this?")
			Expect(status).To(Equal(http.StatusForbidden), fmt.Sprint(out))
			Expect(jstr(out, "error")).To(ContainSubstring("fork it"))
			status, bf := call(bob, "POST", "/turns/"+id1+"/fork", nil)
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(bf))
			Expect(bf["owner"]).To(Equal("bob"))
		})

		It("keeps a national ID number out of what it stores, and will not re-run such a question", func() {
			tid := newThread(alice, caseOpen, "identity")
			status, out := runQ(alice, tid, "Who is 35202-1234567-1 and what did they call?")
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
			Expect(jstr(out, "turn", "question")).To(Equal("Who is #####-#######-# and what did they call?"))
			Expect(eng.lastCall().Question).To(ContainSubstring("35202-1234567-1"), "the engine receives what was typed, and withholds it")
			Expect(jget(out, "turn", "frame", "national_id_count")).To(BeEquivalentTo(1))
			var stored int
			Expect(owner.QueryRow(ctx, `SELECT count(*) FROM forensic.conversation_turn WHERE tenant_id = $1 AND (question_text LIKE '%35202%' OR coalesce(resolved_text, '') LIKE '%35202%' OR coalesce(answer_text, '') LIKE '%35202%' OR frame_json::text LIKE '%35202%')`, tenant).Scan(&stored)).To(Succeed())
			Expect(stored).To(Equal(0))
			status, r := call(alice, "POST", "/turns/"+turnID(out)+"/rerun", nil)
			Expect(status).To(Equal(http.StatusConflict), fmt.Sprint(r))
			Expect(r["code"]).To(Equal("withheld"))
		})

		It("reads a turn lost with its process as interrupted and sweeps it when the next question comes", func() {
			tid := newThread(alice, caseOpen, "lost")
			lost := uuid.NewString()
			_, err := owner.Exec(ctx, `INSERT INTO forensic.conversation_turn (turn_id, tenant_id, collection_id, thread_id, idx, root_turn_id, question_text, state, created_by, started_at)
				VALUES ($1::uuid, $2, $3, $4::uuid, 1, $1::uuid, 'a question that was lost', 'running', 'alice', now() - interval '1 hour')`, lost, tenant, caseOpen, tid)
			Expect(err).NotTo(HaveOccurred())
			_, err = owner.Exec(ctx, `UPDATE forensic.conversation_thread SET turn_count = 1 WHERE thread_id = $1::uuid`, tid)
			Expect(err).NotTo(HaveOccurred())
			_, th := call(alice, "GET", "/conversations/"+tid, nil)
			Expect(jstr(th, "turns", "0", "state")).To(Equal("interrupted"))
			Expect(jstr(th, "turns", "0", "error")).To(ContainSubstring("Run the question again"))
			status, out := runQ(alice, tid, "The next question")
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
			Expect(jget(out, "turn", "idx")).To(BeEquivalentTo(2))
			var state string
			Expect(owner.QueryRow(ctx, `SELECT state FROM forensic.conversation_turn WHERE turn_id = $1::uuid`, lost).Scan(&state)).To(Succeed())
			Expect(state).To(Equal("interrupted"), "the sweep is written, not only read")
		})
	})

	// ---- re-run and compare ----------------------------------------------------------------------------------------------------------------------------------------------

	Describe("re-running and comparing", func() {
		It("says nothing changed when nothing did", func() {
			tid := newThread(alice, caseOpen, "rerun same")
			_, a := runQ(alice, tid, "Which numbers made the most calls?")
			id1 := turnID(a)
			status, r := call(alice, "POST", "/turns/"+id1+"/rerun", nil)
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(r))
			Expect(jstr(r, "turn", "kind")).To(Equal("rerun"))
			Expect(jstr(r, "turn", "rerun_of_turn_id")).To(Equal(id1))
			Expect(jstr(r, "turn", "root_turn_id")).To(Equal(id1))
			Expect(jget(r, "diff", "changed")).To(BeFalse())
			Expect(jstr(r, "diff", "against")).To(Equal(id1))
			Expect(jget(r, "diff", "data_version", "known")).To(BeTrue())
			Expect(jget(r, "diff", "data_version", "changed")).To(BeFalse())
			Expect(jget(r, "diff", "same_data_different_answer")).To(BeFalse())
			Expect(jget(r, "turn", "diff_summary", "changed")).To(BeFalse())
			_, one := call(alice, "GET", "/turns/"+id1+"?answer=false", nil)
			Expect(jget(one, "reruns", "count")).To(BeEquivalentTo(1))
			Expect(jget(one, "reruns", "latest_changed")).To(BeFalse())
		})

		It("says what changed when the figures changed, and that the data did not", func() {
			tid := newThread(alice, caseOpen, "rerun changed")
			_, a := runQ(alice, tid, "Which numbers made the most calls?")
			id1 := turnID(a)
			eng.configure(ranked2, nil, 0)
			status, r := call(alice, "POST", "/turns/"+id1+"/rerun", nil)
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(r))
			Expect(jget(r, "diff", "changed")).To(BeTrue())
			Expect(jget(r, "diff", "figures_changed")).To(BeTrue())
			Expect(jget(r, "diff", "headline", "changed")).To(BeTrue())
			Expect(jget(r, "diff", "rows", "changed_total")).To(BeEquivalentTo(1))
			Expect(jstr(r, "diff", "rows", "changed", "0", "key")).To(Equal("923000000202"))
			Expect(jget(r, "diff", "rows", "changed", "0", "cells", "0", "before")).To(BeEquivalentTo(9))
			Expect(jget(r, "diff", "rows", "changed", "0", "cells", "0", "after")).To(BeEquivalentTo(10))
			Expect(jget(r, "diff", "rows", "added_total")).To(BeEquivalentTo(1))
			Expect(jget(r, "diff", "rows", "removed_total")).To(BeEquivalentTo(0))
			Expect(jget(r, "diff", "rows", "before_count")).To(BeEquivalentTo(3))
			Expect(jget(r, "diff", "rows", "after_count")).To(BeEquivalentTo(4))
			Expect(jget(r, "diff", "same_data_different_answer")).To(BeTrue(), "the data did not move, the answer did")
			Expect(jget(r, "diff", "query", "changed")).To(BeFalse())
			rid := turnID(r)

			_, list := call(alice, "GET", "/conversations?case_id="+caseOpen+"&limit=100", nil)
			var flagged bool
			for _, it := range list["items"].([]any) {
				if jstr(it, "thread_id") == tid {
					flagged = it.(map[string]any)["changed_since"] == true
				}
			}
			Expect(flagged).To(BeTrue(), "the rail shows that this conversation has an answer that changed since")
			_, one := call(alice, "GET", "/turns/"+id1+"?answer=false", nil)
			Expect(jget(one, "reruns", "latest_changed")).To(BeTrue())
			Expect(jstr(one, "reruns", "latest_turn_id")).To(Equal(rid))

			status, d := call(alice, "GET", "/turns/"+rid+"/diff?against="+id1, nil)
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(d))
			Expect(d["changed"]).To(BeTrue())
			status, d = call(alice, "GET", "/turns/"+rid+"/diff", nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(d["against"]).To(Equal(id1), "by default a re-run is compared with the run it re-ran")
			status, d = call(alice, "GET", "/turns/"+id1+"/diff?against="+rid, nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(jget(d, "rows", "removed_total")).To(BeEquivalentTo(1), "the other way round, the added row is a removed one")
			status, _ = call(alice, "GET", "/turns/"+id1+"/diff", nil)
			Expect(status).To(Equal(http.StatusBadRequest), "a first run has nothing earlier to compare with")
		})

		It("names the file and the setting that moved when the case's data changed", func() {
			tid := newThread(alice, caseData, "data moves")
			_, a := runQ(alice, tid, "Which numbers made the most calls?")
			id1 := turnID(a)
			addEvidence(caseData, "new-calls.csv")
			_, err := owner.Exec(ctx, `INSERT INTO forensic.case_record (tenant_id, collection_id, hub_threshold) VALUES ($1, $2, 7) ON CONFLICT (tenant_id, collection_id) DO UPDATE SET hub_threshold = 7`, tenant, caseData)
			Expect(err).NotTo(HaveOccurred())
			status, r := call(alice, "POST", "/turns/"+id1+"/rerun", nil)
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(r))
			Expect(jget(r, "diff", "data_version", "changed")).To(BeTrue())
			Expect(jlen(r, "diff", "data_version", "detail", "evidence_added")).To(Equal(1))
			Expect(jstr(r, "diff", "data_version", "detail", "evidence_added", "0", "name")).To(Equal("new-calls.csv"))
			Expect(fmt.Sprint(jget(r, "diff", "data_version", "detail", "settings_changed"))).To(ContainSubstring("hub_threshold"))
			Expect(jget(r, "diff", "same_data_different_answer")).To(BeFalse())
			Expect(jget(r, "diff", "changed")).To(BeFalse(), "the data moved and the answer did not")
		})
	})

	// ---- who sees what ---------------------------------------------------------------------------------------------------------------------------------------------------

	Describe("who may see and do what", func() {
		It("keeps a private conversation private, shares it with the case or with chosen people, and lets an administrator read it", func() {
			tid := newThread(alice, caseOpen, "private one")
			_, out := runQ(alice, tid, "Which numbers made the most calls?")
			id := turnID(out)
			status, _ := call(bob, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusNotFound))
			status, _ = call(bob, "GET", "/turns/"+id, nil)
			Expect(status).To(Equal(http.StatusNotFound))
			_, list := call(bob, "GET", "/conversations?case_id="+caseOpen+"&limit=100", nil)
			for _, it := range list["items"].([]any) {
				Expect(jstr(it, "thread_id")).NotTo(Equal(tid))
			}
			status, _ = call(root, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusOK), "a global administrator reads private conversations")

			status, sh := call(alice, "PATCH", "/conversations/"+tid, map[string]any{"visibility": "selected", "shared_with": []string{"carol"}})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(sh))
			status, _ = call(carol, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusOK))
			status, _ = call(bob, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusNotFound))
			status, _ = call(alice, "PATCH", "/conversations/"+tid, map[string]any{"visibility": "selected", "shared_with": []string{}})
			Expect(status).To(Equal(http.StatusBadRequest), "sharing with nobody is not sharing")

			status, _ = call(alice, "PATCH", "/conversations/"+tid, map[string]any{"visibility": "case"})
			Expect(status).To(Equal(http.StatusOK))
			status, th := call(bob, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(th["is_owner"]).To(BeFalse())
			status, _ = call(bob, "PATCH", "/conversations/"+tid, map[string]any{"title": "mine now"})
			Expect(status).To(Equal(http.StatusForbidden))
			status, _ = call(bob, "DELETE", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusForbidden))
			_, aud := call(alice, "GET", "/cases/"+caseOpen+"/audit?action=thread.shared&object_id="+tid, nil)
			Expect(jlen(aud, "items")).To(BeNumerically(">=", 2))
		})

		It("shows a credential bound to one case nothing of another", func() {
			bound := caller{subject: "alice", role: "user", bound: caseOpen}
			status, _ := call(bound, "GET", "/cases/"+caseCtl+"/notes", nil)
			Expect(status).To(Equal(http.StatusNotFound))
			status, out := call(bound, "GET", "/conversations?limit=100", nil)
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			Expect(jlen(out, "items")).To(BeNumerically(">", 0))
			for _, it := range out["items"].([]any) {
				Expect(jstr(it, "case_id")).To(Equal(caseOpen))
			}
		})

		It("turns a case into a controlled one with its first member, and roles decide from then on", func() {
			status, m := call(bob, "GET", "/cases/"+caseCtl+"/members", nil)
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(m))
			Expect(m["mode"]).To(Equal("open"))
			Expect(jstr(m, "you", "role")).To(Equal("open"))
			status, _ = call(bob, "PUT", "/cases/"+caseCtl+"/members/alice", map[string]any{"role": "owner"})
			Expect(status).To(Equal(http.StatusBadRequest), "nobody makes someone else the owner of an open case")
			status, _ = call(bob, "PUT", "/cases/"+caseCtl+"/members/bob", map[string]any{"role": "analyst"})
			Expect(status).To(Equal(http.StatusBadRequest), "the first member must be the owner")
			status, mem := call(alice, "PUT", "/cases/"+caseCtl+"/members/alice", map[string]any{"role": "owner", "display_name": "Alice Khan"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(mem))
			Expect(mem["role"]).To(Equal("owner"))

			status, _ = call(bob, "GET", "/cases/"+caseCtl+"/members", nil)
			Expect(status).To(Equal(http.StatusNotFound), "a non-member of a controlled case does not see it")
			status, _ = call(bob, "POST", "/conversations", map[string]any{"case_id": caseCtl})
			Expect(status).To(Equal(http.StatusNotFound))
			for who, role := range map[string]string{"bob": "viewer", "dave": "auditor", "erin": "analyst"} {
				status, mem = call(alice, "PUT", "/cases/"+caseCtl+"/members/"+who, map[string]any{"role": role})
				Expect(status).To(Equal(http.StatusOK), fmt.Sprint(mem))
			}
			status, m = call(bob, "GET", "/cases/"+caseCtl+"/members", nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(m["mode"]).To(Equal("controlled"))
			Expect(jstr(m, "you", "role")).To(Equal("viewer"))
			Expect(jlen(m, "members")).To(Equal(4))
			Expect(jstr(m, "members", "0", "subject_id")).To(Equal("alice"), "the owner is listed first")

			status, out := call(erin, "PUT", "/cases/"+caseCtl+"/members/mallory", map[string]any{"role": "owner"})
			Expect(status).To(Equal(http.StatusForbidden), fmt.Sprint(out))
			status, _ = call(bob, "POST", "/conversations", map[string]any{"case_id": caseCtl})
			Expect(status).To(Equal(http.StatusForbidden), "a viewer cannot start a conversation")
			status, _ = call(bob, "POST", "/cases/"+caseCtl+"/notes", map[string]any{"body": "a viewer cannot write"})
			Expect(status).To(Equal(http.StatusForbidden))
			status, _ = call(dave, "POST", "/cases/"+caseCtl+"/notes", map[string]any{"body": "an auditor does not write"})
			Expect(status).To(Equal(http.StatusForbidden))
			status, _ = call(erin, "POST", "/cases/"+caseCtl+"/authority", map[string]any{"kind": "consent", "reference": "r", "issuing_body": "b"})
			Expect(status).To(Equal(http.StatusForbidden), "an analyst does not record the authority for the case")
			status, note := call(erin, "POST", "/cases/"+caseCtl+"/notes", map[string]any{"body": "an analyst writes", "category": "lead"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(note))

			eid := newThread(erin, caseCtl, "erin private")
			status, _ = call(alice, "GET", "/conversations/"+eid, nil)
			Expect(status).To(Equal(http.StatusNotFound), "even the owner of the case does not read an analyst's private conversation")
			status, _ = call(dave, "GET", "/conversations/"+eid, nil)
			Expect(status).To(Equal(http.StatusOK), "an auditor does")
			status, _ = call(root, "GET", "/conversations/"+eid, nil)
			Expect(status).To(Equal(http.StatusOK))

			status, out = call(alice, "DELETE", "/cases/"+caseCtl+"/members/alice", nil)
			Expect(status).To(Equal(http.StatusConflict), fmt.Sprint(out))
			Expect(out["code"]).To(Equal("last_owner"))
			status, out = call(alice, "PUT", "/cases/"+caseCtl+"/members/alice", map[string]any{"role": "lead"})
			Expect(status).To(Equal(http.StatusConflict), fmt.Sprint(out))
			status, _ = call(alice, "PUT", "/cases/"+caseCtl+"/members/bob", map[string]any{"role": "superuser"})
			Expect(status).To(Equal(http.StatusBadRequest))

			status, _ = call(erin, "GET", "/cases/"+caseCtl+"/audit", nil)
			Expect(status).To(Equal(http.StatusForbidden), "an analyst does not read the audit trail")
			status, aud := call(alice, "GET", "/cases/"+caseCtl+"/audit?action=member.", nil)
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(aud))
			Expect(jlen(aud, "items")).To(BeNumerically(">=", 4))
			Expect(jstr(aud, "items", "0", "object_kind")).To(Equal("member"))
			status, v := call(dave, "GET", "/cases/"+caseCtl+"/audit/verify", nil)
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(v))
			Expect(v["ok"]).To(BeTrue())
			Expect(v["events"]).To(BeNumerically(">=", 6))

			// the second-person rule: erin pins an answer and cannot review it herself; the owner can
			_, out = runQ(erin, eid, "Which numbers made the most calls?")
			status, pin := call(erin, "POST", "/turns/"+turnID(out)+"/pin", nil)
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(pin))
			fid := jstr(pin, "finding", "finding_id")
			status, rv := call(erin, "POST", "/findings/"+fid+"/review", map[string]any{"state": "confirmed"})
			Expect(status).To(Equal(http.StatusForbidden), fmt.Sprint(rv))
			status, _ = call(bob, "POST", "/findings/"+fid+"/review", map[string]any{"state": "confirmed"})
			Expect(status).To(Equal(http.StatusForbidden), "a finding belongs to the case, so a viewer reads it, but a viewer may not review it")
			status, rv = call(alice, "POST", "/findings/"+fid+"/review", map[string]any{"state": "confirmed", "note": "checked against the file"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(rv))
			Expect(rv["review_state"]).To(Equal("confirmed"))
			Expect(rv["reviewed_by"]).To(Equal("alice"))
		})
	})

	// ---- feedback and findings -------------------------------------------------------------------------------------------------------------------------------------------

	Describe("feedback and findings", func() {
		It("keeps one verdict per person and shows the counts", func() {
			tid := newThread(alice, caseOpen, "feedback")
			_, out := runQ(alice, tid, "Which numbers made the most calls?")
			id := turnID(out)
			call(alice, "PATCH", "/conversations/"+tid, map[string]any{"visibility": "case"})
			status, fb := call(bob, "POST", "/turns/"+id+"/feedback", map[string]any{"verdict": "wrong", "note": "the figure is for the other number"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(fb))
			Expect(jget(fb, "counts", "wrong")).To(BeEquivalentTo(1))
			Expect(jstr(fb, "mine", "verdict")).To(Equal("wrong"))
			status, fb = call(bob, "POST", "/turns/"+id+"/feedback", map[string]any{"verdict": "incomplete"})
			Expect(status).To(Equal(http.StatusOK))
			Expect(jget(fb, "counts", "incomplete")).To(BeEquivalentTo(1))
			Expect(jget(fb, "counts", "wrong")).To(BeNil())
			status, _ = call(bob, "POST", "/turns/"+id+"/feedback", map[string]any{"verdict": "maybe"})
			Expect(status).To(Equal(http.StatusBadRequest))
			status, fb = call(alice, "POST", "/turns/"+id+"/feedback", map[string]any{"verdict": "correct"})
			Expect(status).To(Equal(http.StatusOK))
			Expect(jget(fb, "counts", "correct")).To(BeEquivalentTo(1))
			_, one := call(alice, "GET", "/turns/"+id+"?answer=false", nil)
			Expect(jstr(one, "feedback", "mine", "verdict")).To(Equal("correct"))
			status, all := call(alice, "GET", "/turns/"+id+"/feedback", nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(jlen(all, "items")).To(Equal(2), "the owner of the conversation reads everyone's feedback")
			status, _ = call(bob, "GET", "/turns/"+id+"/feedback", nil)
			Expect(status).To(Equal(http.StatusOK), "an open case has no roles: everyone who may read the conversation reads its feedback")
		})

		It("pins an answer to the findings once, refuses what is not an answer, and lets a finding be edited, reviewed and removed", func() {
			tid := newThread(alice, caseOpen, "pins")
			_, out := runQ(alice, tid, "Which numbers made the most calls?")
			id := turnID(out)
			status, pin := call(alice, "POST", "/turns/"+id+"/pin", map[string]any{"note": "the three numbers to look at first"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(pin))
			f := pin["finding"].(map[string]any)
			Expect(f["title"]).To(HavePrefix("3 numbers made calls"))
			Expect(f["basis"]).To(Equal("observed"))
			Expect(f["review_state"]).To(Equal("unreviewed"))
			Expect(f["source_turn_id"]).To(Equal(id))
			Expect(f["case_id"]).To(Equal(caseOpen))
			Expect(jstr(f, "pinned_json", "headline")).To(HavePrefix("3 numbers"))
			Expect(jstr(f, "pinned_json", "query_hash")).To(MatchRegexp(`^[0-9a-f]{64}$`))
			Expect(jlen(f, "pinned_json", "figures", "rows")).To(Equal(3))
			Expect(jlen(f, "evidence_refs")).To(Equal(1))
			fid := jstr(f, "finding_id")
			status, again := call(alice, "POST", "/turns/"+id+"/pin", nil)
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(again))
			Expect(again["already_pinned"]).To(BeTrue())
			Expect(jstr(again, "finding", "finding_id")).To(Equal(fid))
			_, one := call(alice, "GET", "/turns/"+id+"?answer=false", nil)
			Expect(fmt.Sprint(one["pinned_as"])).To(ContainSubstring(fid))

			eng.configure(func(EngineRequest) (int, []byte) { return http.StatusOK, abstainBody() }, nil, 0)
			_, ab := runQ(alice, tid, "Something it cannot show")
			Expect(jstr(ab, "turn", "outcome")).To(Equal("abstained"))
			status, no := call(alice, "POST", "/turns/"+turnID(ab)+"/pin", nil)
			Expect(status).To(Equal(http.StatusConflict), fmt.Sprint(no))
			Expect(no["code"]).To(Equal("not_an_answer"))

			status, got := call(alice, "GET", "/findings/"+fid, nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(got["body"]).To(Equal("the three numbers to look at first"))
			status, up := call(alice, "PATCH", "/findings/"+fid, map[string]any{"title": "The three busiest numbers", "basis": "inferred", "sort_order": 3})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(up))
			Expect(up["basis"]).To(Equal("inferred"))
			status, _ = call(alice, "PATCH", "/findings/"+fid, map[string]any{"review_state": "confirmed"})
			Expect(status).To(Equal(http.StatusBadRequest), "a review is not a patch")
			status, _ = call(alice, "PATCH", "/findings/"+fid, map[string]any{"source_turn_id": uuid.NewString()})
			Expect(status).To(Equal(http.StatusBadRequest))
			status, rv := call(alice, "POST", "/findings/"+fid+"/review", map[string]any{"state": "rejected", "note": "double counted"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(rv))
			Expect(rv["review_state"]).To(Equal("rejected"))
			status, rv = call(alice, "POST", "/findings/"+fid+"/review", map[string]any{"state": "unreviewed"})
			Expect(status).To(Equal(http.StatusOK))
			Expect(rv["reviewed_by"]).To(BeNil())
			_, list := call(alice, "GET", "/cases/"+caseOpen+"/findings?review_state=unreviewed&source_turn_id="+id, nil)
			Expect(jlen(list, "items")).To(Equal(1))
			status, _ = call(alice, "DELETE", "/findings/"+fid+"?reason=not+relevant", nil)
			Expect(status).To(Equal(http.StatusNoContent))
			status, _ = call(alice, "GET", "/findings/"+fid, nil)
			Expect(status).To(Equal(http.StatusNotFound))
			status, pin = call(alice, "POST", "/turns/"+id+"/pin", nil)
			Expect(status).To(Equal(http.StatusCreated), "after the finding is removed the answer can be pinned again")
			Expect(jstr(pin, "finding", "finding_id")).NotTo(Equal(fid))
		})
	})

	// ---- notes, tags, tasks, views, annotations, authority ---------------------------------------------------------------------------------------------------------------

	Describe("knowledge capture", func() {
		It("keeps notes with tags and search, and a reason for every delete", func() {
			base := "/cases/" + caseOpen + "/notes"
			status, n := call(alice, "POST", base, map[string]any{"category": "lead", "title": "Handset", "body": "Check the second handset of 923001234567"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(n))
			Expect(n["case_id"]).To(Equal(caseOpen))
			Expect(n["created_by"]).To(Equal("alice"))
			Expect(n["tenant_id"]).To(BeNil(), "the tenant is not part of the answer")
			at, err := time.Parse(time.RFC3339Nano, jstr(n, "created_at"))
			Expect(err).NotTo(HaveOccurred())
			_, offset := at.Zone()
			Expect(offset).To(Equal(0), "times of objects are in UTC too")
			Expect(jlen(n, "tags")).To(Equal(0))
			id := jstr(n, "note_id")
			for _, bad := range []map[string]any{{"body": ""}, {"body": "x", "category": "gossip"}, {"body": "x", "created_by": "bob"}, {"category": "lead"}} {
				status, _ = call(alice, "POST", base, bad)
				Expect(status).To(Equal(http.StatusBadRequest), fmt.Sprint(bad))
			}
			status, _ = call(alice, "POST", "/cases/no-such-case/notes", map[string]any{"body": "x"})
			Expect(status).To(Equal(http.StatusNotFound))
			status, p := call(alice, "PATCH", "/notes/"+id, map[string]any{"pinned": true})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(p))
			Expect(p["pinned"]).To(BeTrue())
			Expect(p["updated_by"]).To(Equal("alice"))
			_, l := call(alice, "GET", base+"?q=HANDSET&category=lead", nil)
			Expect(jlen(l, "items")).To(Equal(1))
			_, l = call(alice, "GET", base+"?q=nothing+matches+this", nil)
			Expect(jlen(l, "items")).To(Equal(0))

			status, tg := call(alice, "PUT", "/cases/"+caseOpen+"/objects/note/"+id+"/tags", map[string]any{"tags": []string{"Urgent", "bank"}})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(tg))
			Expect(jlen(tg, "tags")).To(Equal(2))
			status, tg = call(alice, "PUT", "/cases/"+caseOpen+"/objects/note/"+id+"/tags", map[string]any{"tags": []string{" URGENT "}})
			Expect(status).To(Equal(http.StatusOK))
			Expect(jlen(tg, "tags")).To(Equal(1))
			Expect(jstr(tg, "tags", "0")).To(Equal("Urgent"), "two spellings are one tag")
			_, l = call(alice, "GET", base+"?tag=urgent", nil)
			Expect(jlen(l, "items")).To(Equal(1))
			Expect(jstr(l, "items", "0", "tags", "0")).To(Equal("Urgent"))
			status, _ = call(alice, "PUT", "/cases/"+caseOpen+"/objects/note/"+uuid.NewString()+"/tags", map[string]any{"tags": []string{"x"}})
			Expect(status).To(Equal(http.StatusNotFound))
			status, _ = call(alice, "PUT", "/cases/"+caseOpen+"/objects/spaceship/"+id+"/tags", map[string]any{"tags": []string{"x"}})
			Expect(status).To(Equal(http.StatusBadRequest))
			_, tags := call(alice, "GET", "/cases/"+caseOpen+"/tags", nil)
			var urgentID string
			for _, t := range tags["items"].([]any) {
				if jstr(t, "name") == "Urgent" {
					urgentID = jstr(t, "tag_id")
					Expect(jget(t, "uses")).To(BeEquivalentTo(1))
				}
			}
			Expect(urgentID).NotTo(BeEmpty())
			status, rn := call(alice, "PATCH", "/tags/"+urgentID, map[string]any{"name": "Very urgent", "color": "#c0392b"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(rn))
			Expect(rn["name"]).To(Equal("Very urgent"))
			status, _ = call(alice, "PATCH", "/tags/"+urgentID, map[string]any{"color": "red"})
			Expect(status).To(Equal(http.StatusBadRequest))

			status, _ = call(alice, "DELETE", "/notes/"+id+"?reason=entered+in+error", nil)
			Expect(status).To(Equal(http.StatusNoContent))
			status, _ = call(alice, "GET", "/notes/"+id, nil)
			Expect(status).To(Equal(http.StatusNotFound))
			_, l = call(alice, "GET", base+"?q=handset", nil)
			Expect(jlen(l, "items")).To(Equal(0))
			_, l = call(alice, "GET", base+"?q=handset&include_deleted=true", nil)
			Expect(jlen(l, "items")).To(Equal(1))
			_, aud := call(alice, "GET", "/cases/"+caseOpen+"/audit?action=note.deleted&object_id="+id, nil)
			Expect(jstr(aud, "items", "0", "details", "reason")).To(Equal("entered in error"))
			_, aud = call(alice, "GET", "/cases/"+caseOpen+"/audit?action=note.created&object_id="+id, nil)
			Expect(fmt.Sprint(jget(aud, "items", "0", "details", "fields"))).NotTo(ContainSubstring("handset"), "the audit names the fields, never the text")
		})

		It("keeps hypotheses, tasks, annotations and saved views", func() {
			c := "/cases/" + caseOpen
			status, h := call(alice, "POST", c+"/hypotheses", map[string]any{"statement": "The second handset belongs to the courier", "confidence": "low"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(h))
			Expect(h["status"]).To(Equal("open"))
			hid := jstr(h, "hypothesis_id")
			status, h = call(alice, "PATCH", "/hypotheses/"+hid, map[string]any{"status": "supported", "supports": []string{uuid.NewString()}})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(h))
			_, l := call(alice, "GET", c+"/hypotheses?status=supported", nil)
			Expect(jlen(l, "items")).To(BeNumerically(">=", 1))
			status, _ = call(alice, "PATCH", "/hypotheses/"+hid, map[string]any{"status": "proven"})
			Expect(status).To(Equal(http.StatusBadRequest))

			status, tk := call(alice, "POST", c+"/tasks", map[string]any{"title": "Ask the operator for the IMEI history", "due_at": "2026-11-10T09:00:00Z"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(tk))
			Expect(tk["status"]).To(Equal("open"))
			Expect(tk["completed_at"]).To(BeNil())
			tid := jstr(tk, "task_id")
			status, tk = call(alice, "PATCH", "/tasks/"+tid, map[string]any{"status": "done"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(tk))
			Expect(tk["completed_by"]).To(Equal("alice"))
			Expect(tk["completed_at"]).NotTo(BeNil())
			status, tk = call(alice, "PATCH", "/tasks/"+tid, map[string]any{"status": "open"})
			Expect(status).To(Equal(http.StatusOK))
			Expect(tk["completed_at"]).To(BeNil())
			status, _ = call(alice, "POST", c+"/tasks", map[string]any{"title": "x", "due_at": "next week"})
			Expect(status).To(Equal(http.StatusBadRequest))

			status, an := call(alice, "POST", c+"/annotations", map[string]any{"target_kind": "timeline_day", "target_key": "2026-02-03", "body": "the day of the meeting"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(an))
			_, l = call(alice, "GET", c+"/annotations?target_kind=timeline_day&target_key=2026-02-03", nil)
			Expect(jlen(l, "items")).To(Equal(1))
			status, _ = call(alice, "PATCH", "/annotations/"+jstr(an, "annotation_id"), map[string]any{"target_key": "2026-02-04"})
			Expect(status).To(Equal(http.StatusBadRequest), "an annotation stays where it was put")

			status, v := call(alice, "POST", c+"/views", map[string]any{"name": "Links in February", "kind": "connections", "params_json": map[string]any{"from": "2026-02-01"}})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(v))
			Expect(v["visibility"]).To(Equal("private"))
			vid := jstr(v, "view_id")
			status, _ = call(bob, "GET", "/views/"+vid, nil)
			Expect(status).To(Equal(http.StatusNotFound))
			_, l = call(bob, "GET", c+"/views?kind=connections", nil)
			for _, it := range l["items"].([]any) {
				Expect(jstr(it, "view_id")).NotTo(Equal(vid))
			}
			status, _ = call(alice, "PATCH", "/views/"+vid, map[string]any{"visibility": "case"})
			Expect(status).To(Equal(http.StatusOK))
			status, _ = call(bob, "GET", "/views/"+vid, nil)
			Expect(status).To(Equal(http.StatusOK))
			status, _ = call(bob, "PATCH", "/views/"+vid, map[string]any{"name": "Bob's now"})
			Expect(status).To(Equal(http.StatusForbidden), "only the person who made a view changes it")
			status, _ = call(alice, "PATCH", "/views/"+vid, map[string]any{"kind": "places"})
			Expect(status).To(Equal(http.StatusBadRequest))
			status, _ = call(alice, "DELETE", "/views/"+vid, nil)
			Expect(status).To(Equal(http.StatusNoContent))
		})

		It("records the authority for working a case, computes whether it holds, and revokes it", func() {
			c := "/cases/" + caseOpen + "/authority"
			status, a := call(alice, "POST", c, map[string]any{"kind": "court_order", "reference": "ATC-2026/114", "issuing_body": "Anti-Terrorism Court Lahore", "valid_from": "2026-01-01", "valid_to": "2099-12-31", "scope": "Call records of two numbers"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(a))
			Expect(a["status"]).To(Equal("active"))
			aid := jstr(a, "authority_id")
			status, e := call(alice, "POST", c, map[string]any{"kind": "warrant", "reference": "W-1", "issuing_body": "Magistrate", "valid_from": "2019-01-01", "valid_to": "2020-01-01"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(e))
			Expect(e["status"]).To(Equal("expired"))
			status, f := call(alice, "POST", c, map[string]any{"kind": "consent", "reference": "C-9", "issuing_body": "Owner of the handset", "valid_from": "2099-01-01"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(f))
			Expect(f["status"]).To(Equal("not_yet_valid"))
			status, _ = call(alice, "POST", c, map[string]any{"kind": "warrant", "reference": "W-2", "issuing_body": "x", "valid_from": "2026-05-01", "valid_to": "2026-04-01"})
			Expect(status).To(Equal(http.StatusBadRequest))
			status, rv := call(alice, "POST", "/authority/"+aid+"/revoke", map[string]any{"reason": "Order withdrawn by the court"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(rv))
			Expect(rv["status"]).To(Equal("revoked"))
			Expect(rv["revoked_by"]).To(Equal("alice"))
			status, rv = call(alice, "POST", "/authority/"+aid+"/revoke", map[string]any{"reason": "Again"})
			Expect(status).To(Equal(http.StatusConflict), fmt.Sprint(rv))
			status, _ = call(alice, "PATCH", "/authority/"+aid, map[string]any{"notes": "Copy attached"})
			Expect(status).To(Equal(http.StatusOK))
			_, l := call(alice, "GET", c, nil)
			Expect(jlen(l, "items")).To(Equal(3))
			status, _ = call(alice, "DELETE", "/authority/"+aid, nil)
			Expect(status).To(BeElementOf(http.StatusNotFound, http.StatusMethodNotAllowed), "an authority record is revoked, never deleted")
		})
	})

	// ---- search ----------------------------------------------------------------------------------------------------------------------------------------------------------------

	Describe("searching the history", func() {
		It("finds Urdu and Roman-Urdu questions however the letters were typed", func() {
			tid := newThread(alice, caseOpen, "search")
			runQ(alice, tid, "كيا نمبر 923001234567 نے کال کی؟") // Arabic kaf and yeh in the first word
			runQ(alice, tid, "kitni calls hui hain 0300-1234567 se?")
			find := func(q string) (map[string]any, bool) {
				_, out := call(alice, "GET", "/conversations?case_id="+caseOpen+"&limit=100&q="+urlQuery(q), nil)
				for _, it := range out["items"].([]any) {
					if jstr(it, "thread_id") == tid {
						return it.(map[string]any), true
					}
				}
				return nil, false
			}
			item, ok := find("کیا") // the same word with the Urdu kaf and yeh
			Expect(ok).To(BeTrue(), "the Urdu spelling finds the Arabic-keyboard spelling")
			Expect(jlen(item, "matches")).To(BeNumerically(">=", 1))
			Expect(jstr(item, "matches", "0", "question")).To(ContainSubstring("923001234567"))
			_, ok = find("٩٢٣٠٠١٢٣٤٥٦٧") // the number in Arabic-Indic digits
			Expect(ok).To(BeTrue(), "a number typed in Urdu digits finds the same number in Latin digits")
			_, ok = find("CALLS")
			Expect(ok).To(BeTrue(), "case does not matter")
			_, ok = find("kitni calls")
			Expect(ok).To(BeTrue(), "every word must be there")
			_, ok = find("kitni zebra")
			Expect(ok).To(BeFalse())
			_, ok = find("0300-1234567")
			Expect(ok).To(BeTrue())
			_, ok = find("100%")
			Expect(ok).To(BeFalse(), "a percent sign is searched as written, not as a wildcard")
		})
	})

	// ---- the audit chain -------------------------------------------------------------------------------------------------------------------------------------------------

	Describe("the audit chain", func() {
		It("verifies, and finds a row changed behind the application's back", func() {
			tid := newThread(alice, caseLog, "audited")
			runQ(alice, tid, "Which numbers made the most calls?")
			call(alice, "POST", "/cases/"+caseLog+"/notes", map[string]any{"body": "an audited note"})
			status, v := call(alice, "GET", "/cases/"+caseLog+"/audit/verify", nil)
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(v))
			Expect(v["ok"]).To(BeTrue())
			Expect(v["events"]).To(BeNumerically(">=", 4))
			status, page := call(alice, "GET", "/cases/"+caseLog+"/audit?limit=2", nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(jlen(page, "items")).To(Equal(2))
			Expect(page["next_cursor"]).NotTo(BeEmpty())
			Expect(jstr(page, "items", "0", "previous_sha256")).To(Equal(jstr(page, "items", "1", "event_sha256")), "each row names the hash of the one before it")

			_, err := owner.Exec(ctx, `ALTER TABLE forensic.case_audit_event DISABLE TRIGGER case_audit_event_append_only`)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				_, _ = owner.Exec(ctx, `ALTER TABLE forensic.case_audit_event ENABLE TRIGGER case_audit_event_append_only`)
			})
			_, err = owner.Exec(ctx, `UPDATE forensic.case_audit_event SET details = '{"tampered":true}' WHERE tenant_id = $1 AND collection_id = $2 AND seq = 2`, tenant, caseLog)
			Expect(err).NotTo(HaveOccurred())
			_, err = owner.Exec(ctx, `ALTER TABLE forensic.case_audit_event ENABLE TRIGGER case_audit_event_append_only`)
			Expect(err).NotTo(HaveOccurred())
			status, v = call(alice, "GET", "/cases/"+caseLog+"/audit/verify", nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(v["ok"]).To(BeFalse())
			Expect(v["first_broken_seq"]).To(BeEquivalentTo(2))
		})
	})
})

// urlQuery escapes a search text for a query string.
func urlQuery(s string) string {
	return strings.NewReplacer("%", "%25", " ", "+", "&", "%26", "#", "%23", "?", "%3F").Replace(s)
}
