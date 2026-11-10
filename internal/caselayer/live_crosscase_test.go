package caselayer

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (e *scriptedEngine) callsFrom(n int) []EngineRequest {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]EngineRequest(nil), e.calls[n:]...)
}

func threadIDs(out map[string]any) []string {
	var ids []string
	items, _ := out["items"].([]any)
	for _, it := range items {
		ids = append(ids, jstr(it, "thread_id"))
	}
	return ids
}

// These specs run against the throwaway database of tools/dev/live-window/rehearse-034.sh (KEEP=1), as the application role, under row-level security.
var _ = Describe("conversations across cases on a throwaway database", Ordered, func() {
	var (
		f     *liveFixture
		id    = uuid.NewString()[:6]
		caseA = "ca-" + id // open
		caseB = "cb-" + id // open
		caseC = "cc-" + id // controlled: alice owner, bob viewer, erin analyst
		caseD = "cd-" + id // controlled: alice owner, erin analyst, dave auditor
		zed   = caller{subject: "zed", role: "user"}
	)
	byCase := func(req EngineRequest) (int, []byte) {
		if req.CaseID == caseB || req.CaseID == caseD {
			return ranked2(req)
		}
		return ranked1(req)
	}

	BeforeAll(func() {
		f = newLiveFixture()
		for _, c := range []string{caseA, caseB, caseC, caseD} {
			f.makeCase(c)
		}
		f.addEvidence(caseA, "a-calls.csv")
		f.control(caseC, alice, map[string]string{"bob": "viewer", "erin": "analyst"})
		f.control(caseD, alice, map[string]string{"erin": "analyst", "dave": "auditor"})
	})
	AfterAll(func() {
		if f != nil {
			f.close()
		}
	})
	BeforeEach(func() {
		if f != nil {
			f.eng.configure(byCase, nil, 0)
		}
	})

	Describe("starting one", func() {
		It("keeps each case once, in byte order, and says it is across cases", func() {
			status, out := f.call(alice, "POST", "/conversations", map[string]any{"case_ids": []string{caseB, caseA, " " + caseB + " "}, "title": "A and B"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
			Expect(out["cross_case"]).To(BeTrue())
			Expect(out["case_id"]).To(Equal(""))
			Expect(out["case_ids"]).To(Equal([]any{caseA, caseB}))
			Expect(out["is_owner"]).To(BeTrue())
			Expect(out["turn_count"]).To(BeEquivalentTo(0))
			status, one := f.call(alice, "POST", "/conversations", map[string]any{"case_id": caseA, "title": "just A"})
			Expect(status).To(Equal(http.StatusCreated))
			Expect(one["cross_case"]).To(BeFalse())
			Expect(one["case_id"]).To(Equal(caseA))
			Expect(one["case_ids"]).To(Equal([]any{caseA}))
		})

		It("refuses a list that is not allowed, and says why", func() {
			status, out := f.call(alice, "POST", "/conversations", map[string]any{"case_ids": []string{caseA}})
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(jstr(out, "error")).To(ContainSubstring("at least 2 different cases"))
			status, out = f.call(alice, "POST", "/conversations", map[string]any{"case_id": caseA, "case_ids": []string{caseA, caseB}})
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(jstr(out, "error")).To(ContainSubstring("not both"))
			var many []string
			for i := 1; i <= 11; i++ {
				many = append(many, fmt.Sprintf("x%02d", i))
			}
			status, out = f.call(alice, "POST", "/conversations", map[string]any{"case_ids": many})
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(jstr(out, "error")).To(ContainSubstring("at most 10"))
			status, out = f.call(alice, "POST", "/conversations", map[string]any{"case_ids": caseA})
			Expect(status).To(Equal(http.StatusBadRequest))
			Expect(jstr(out, "error")).To(ContainSubstring("list of text"))
			missing := "no-such-" + id
			status, out = f.call(alice, "POST", "/conversations", map[string]any{"case_ids": []string{caseA, missing}})
			Expect(status).To(Equal(http.StatusNotFound), fmt.Sprint(out))
			Expect(jstr(out, "error")).To(ContainSubstring(missing))
		})

		It("needs a role that allows asking in every case, and shows a case you may not open as one that does not exist", func() {
			status, out := f.call(bob, "POST", "/conversations", map[string]any{"case_ids": []string{caseA, caseC}})
			Expect(status).To(Equal(http.StatusForbidden), fmt.Sprint(out))
			Expect(jstr(out, "error")).To(ContainSubstring(caseC))
			Expect(jstr(out, "error")).To(ContainSubstring("viewer"))
			status, out = f.call(zed, "POST", "/conversations", map[string]any{"case_ids": []string{caseA, caseC}})
			Expect(status).To(Equal(http.StatusNotFound), fmt.Sprint(out))
			Expect(jstr(out, "error")).To(ContainSubstring("no such case"))
			status, _ = f.call(erin, "POST", "/conversations", map[string]any{"case_ids": []string{caseA, caseC}})
			Expect(status).To(Equal(http.StatusCreated), "an analyst of C and anyone in the open A may")
		})

		It("is refused to a credential bound to one case, which has a role in no other", func() {
			bound := caller{subject: "alice", role: "user", bound: caseA}
			status, out := f.call(bound, "POST", "/conversations", map[string]any{"case_ids": []string{caseA, caseB}})
			Expect(status).To(Equal(http.StatusNotFound), fmt.Sprint(out))
		})
	})

	Describe("asking one", func() {
		var tid string
		BeforeAll(func() { tid = f.newAcross(alice, "scope", caseA, caseB) })

		It("puts the question to each case in turn, one engine call each, and states the scope in a sentence", func() {
			before := f.eng.callCount()
			status, out := f.call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "Which numbers made the most calls?"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
			Expect(out["existing"]).To(BeFalse())
			group := out["group"].(map[string]any)
			Expect(group["answered"]).To(BeEquivalentTo(2))
			Expect(group["not_answered"]).To(BeEquivalentTo(0))
			Expect(group["summary"]).To(Equal(fmt.Sprintf("Asked in 2 cases (%s, %s): answered in 2, not answered in 0.", caseA, caseB)))
			Expect(jstr(group, "question")).To(Equal("Which numbers made the most calls?"))
			Expect(jlen(group, "cases")).To(Equal(2))
			t0, t1 := jget(group, "cases", "0", "turn"), jget(group, "cases", "1", "turn")
			Expect(jstr(group, "cases", "0", "case_id")).To(Equal(caseA))
			Expect(jstr(group, "cases", "1", "case_id")).To(Equal(caseB))
			Expect(jstr(t0, "case_id")).To(Equal(caseA))
			Expect(jstr(t1, "case_id")).To(Equal(caseB))
			Expect(jstr(t0, "group_id")).To(Equal(jstr(group, "group_id")))
			Expect(jstr(t1, "group_id")).To(Equal(jstr(group, "group_id")))
			Expect(jstr(group, "group_id")).To(MatchRegexp(`^[0-9a-f-]{36}$`))
			Expect(jget(t0, "idx")).To(BeEquivalentTo(1))
			Expect(jget(t1, "idx")).To(BeEquivalentTo(2))
			Expect(jstr(t0, "headline")).To(HavePrefix("3 numbers"))
			Expect(jstr(t1, "headline")).To(HavePrefix("4 numbers"))
			Expect(jstr(t0, "answer", "answer", "answer")).To(HavePrefix("3 numbers"), "each case's own answer, exactly as the engine returned it")
			Expect(jstr(t0, "data_version_hash")).NotTo(Equal(jstr(t1, "data_version_hash")), "each turn is computed on the data of its own case (A holds a file, B none)")
			calls := f.eng.callsFrom(before)
			Expect(calls).To(HaveLen(2))
			Expect(calls[0].CaseID).To(Equal(caseA))
			Expect(calls[1].CaseID).To(Equal(caseB))
			Expect(calls[0].Question).To(Equal("Which numbers made the most calls?"))

			status, th := f.call(alice, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(th["cross_case"]).To(BeTrue())
			Expect(th["turn_count"]).To(BeEquivalentTo(2))
			Expect(jlen(th, "turns")).To(Equal(2))
			Expect(jstr(th, "turns", "0", "case_id")).To(Equal(caseA))
			Expect(jstr(th, "turns", "1", "case_id")).To(Equal(caseB))
			Expect(jstr(th, "last_headline")).To(HavePrefix("4 numbers"))
		})

		It("reads a follow-up in the context of the answers of the same case only", func() {
			before := f.eng.callCount()
			status, out := f.call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "And the same for last week?"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
			calls := f.eng.callsFrom(before)
			Expect(calls).To(HaveLen(2))
			for _, c := range calls {
				Expect(c.History).To(HaveLen(1), "case "+c.CaseID)
			}
			Expect(calls[0].History[0].Headline).To(HavePrefix("3 numbers"))
			Expect(calls[1].History[0].Headline).To(HavePrefix("4 numbers"))
			group := out["group"].(map[string]any)
			Expect(jstr(group, "cases", "0", "turn", "parent_turn_id")).To(Equal(calls[0].History[0].TurnID))
		})

		It("streams one clock, a turn for each case, the group, and a single 'done'", func() {
			status, events := f.stream(alice, "/conversations/"+tid+"/turns", map[string]any{"question": "Stream it"})
			Expect(status).To(Equal(http.StatusOK))
			Expect(events[0]["event"]).To(Equal("phase"))
			Expect(events[0]["phase"]).To(Equal("cases"))
			var turnCases []string
			dones, groups := 0, 0
			prev := -1.0
			for _, e := range events {
				at := e["at_ms"].(float64)
				Expect(at).To(BeNumerically(">=", prev), "the clock of the whole question does not run backwards")
				prev = at
				switch e["event"] {
				case "turn":
					turnCases = append(turnCases, jstr(e, "case_id"))
					Expect(jstr(e, "group_id")).NotTo(BeEmpty())
				case "done":
					dones++
				case "group":
					groups++
				}
			}
			Expect(turnCases).To(Equal([]string{caseA, caseB}))
			Expect(dones).To(Equal(1))
			Expect(groups).To(Equal(1))
			Expect(events[len(events)-1]["event"]).To(Equal("done"))
			Expect(events[len(events)-2]["event"]).To(Equal("group"))
			Expect(jstr(events[len(events)-2], "group", "summary")).To(ContainSubstring("answered in 2"))
		})

		It("returns the same turns for a retry of the same request and asks nothing", func() {
			body := map[string]any{"question": "Retry me", "client_request_id": "req-x-1"}
			status, first := f.call(alice, "POST", "/conversations/"+tid+"/turns", body)
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(first))
			n := f.eng.callCount()
			status, again := f.call(alice, "POST", "/conversations/"+tid+"/turns", body)
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(again))
			Expect(again["existing"]).To(BeTrue())
			Expect(f.eng.callCount()).To(Equal(n), "the engine was not asked again")
			Expect(jstr(again, "group", "group_id")).To(Equal(jstr(first, "group", "group_id")))
			Expect(jstr(again, "group", "cases", "0", "turn", "turn_id")).To(Equal(jstr(first, "group", "cases", "0", "turn", "turn_id")))
			Expect(jstr(again, "group", "cases", "1", "turn", "turn_id")).To(Equal(jstr(first, "group", "cases", "1", "turn", "turn_id")))
		})

		It("records the failure of one case as a failed turn of that case, and the others go on", func() {
			f.eng.configure(func(req EngineRequest) (int, []byte) {
				if req.CaseID == caseB {
					return http.StatusBadRequest, []byte(`{"error":"no data for this case"}`)
				}
				return ranked1(req)
			}, nil, 0)
			status, out := f.call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "One of them fails"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
			group := out["group"].(map[string]any)
			Expect(group["answered"]).To(BeEquivalentTo(1))
			Expect(group["not_answered"]).To(BeEquivalentTo(1))
			Expect(jstr(group, "cases", "0", "turn", "outcome")).To(Equal("answered"))
			Expect(jstr(group, "cases", "1", "turn", "state")).To(Equal("failed"))
			Expect(jstr(group, "cases", "1", "turn", "error")).To(ContainSubstring("no data for this case"))
			Expect(jstr(group, "summary")).To(ContainSubstring("answered in 1, not answered in 1"))
		})

		It("does not put a question below a chosen answer, nor replace one", func() {
			status, th := f.call(alice, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusOK))
			first := jstr(th, "turns", "0", "turn_id")
			status, out := f.call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "x", "parent_turn_id": first})
			Expect(status).To(Equal(http.StatusBadRequest), fmt.Sprint(out))
			Expect(jstr(out, "error")).To(ContainSubstring("Fork the answer of one case"))
			status, _ = f.call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "x", "edit_of_turn_id": first})
			Expect(status).To(Equal(http.StatusBadRequest))
		})

		It("asks nothing at all when the asker may no longer ask in one of the cases, and says which", func() {
			mine := f.newAcross(erin, "erin across", caseA, caseC)
			status, out := f.call(alice, "PUT", "/cases/"+caseC+"/members/erin", map[string]any{"role": "viewer"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			defer func() {
				st, o := f.call(alice, "PUT", "/cases/"+caseC+"/members/erin", map[string]any{"role": "analyst"})
				Expect(st).To(Equal(http.StatusOK), fmt.Sprint(o))
			}()
			n := f.eng.callCount()
			status, out = f.call(erin, "POST", "/conversations/"+mine+"/turns", map[string]any{"question": "Can I still ask?"})
			Expect(status).To(Equal(http.StatusForbidden), fmt.Sprint(out))
			Expect(jstr(out, "error")).To(ContainSubstring(caseC))
			Expect(f.eng.callCount()).To(Equal(n), "no case was asked")
			status, th := f.call(erin, "GET", "/conversations/"+mine, nil)
			Expect(status).To(Equal(http.StatusOK), "she may still read it")
			Expect(th["turn_count"]).To(BeEquivalentTo(0), "and no turn was written in either case")
		})
	})

	Describe("who sees one", func() {
		It("is private to its owner, then to everyone who may open every case it covers, and never to someone who may not open one of them", func() {
			tid := f.newAcross(alice, "A and C", caseA, caseC)
			status, _ := f.call(bob, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusNotFound), "private to alice")
			status, out := f.call(alice, "PATCH", "/conversations/"+tid, map[string]any{"visibility": "case"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			for _, who := range []caller{bob, erin, alice} {
				status, _ = f.call(who, "GET", "/conversations/"+tid, nil)
				Expect(status).To(Equal(http.StatusOK), who.subject)
			}
			status, _ = f.call(zed, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusNotFound), "zed may open A but not the controlled C")

			_, zedRail := f.call(zed, "GET", "/conversations?case_id="+caseA+"&limit=100", nil)
			Expect(threadIDs(zedRail)).NotTo(ContainElement(tid))
			_, bobRail := f.call(bob, "GET", "/conversations?case_id="+caseA+"&limit=100", nil)
			Expect(threadIDs(bobRail)).To(ContainElement(tid), "the rail of a case lists the conversations across cases that cover it")
			_, only := f.call(alice, "GET", "/conversations?cross_case=true&limit=100", nil)
			Expect(threadIDs(only)).To(ContainElement(tid))
			for _, it := range only["items"].([]any) {
				Expect(it.(map[string]any)["cross_case"]).To(BeTrue())
			}
			_, single := f.call(alice, "GET", "/conversations?cross_case=false&limit=100", nil)
			Expect(threadIDs(single)).NotTo(ContainElement(tid))
		})

		It("is shared only with people who are members of every controlled case it covers", func() {
			tid := f.newAcross(alice, "A and C again", caseA, caseC)
			status, out := f.call(alice, "PATCH", "/conversations/"+tid, map[string]any{"visibility": "selected", "shared_with": []string{"zed"}})
			Expect(status).To(Equal(http.StatusBadRequest), fmt.Sprint(out))
			Expect(jstr(out, "error")).To(ContainSubstring("not members"))
			status, out = f.call(alice, "PATCH", "/conversations/"+tid, map[string]any{"visibility": "selected", "shared_with": []string{"erin"}})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			status, _ = f.call(erin, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusOK))
			status, _ = f.call(bob, "GET", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusNotFound))
		})

		It("is read among private threads by an auditor of every case it covers, and by no one who audits only some", func() {
			cd := f.newAcross(alice, "C and D", caseC, caseD)
			ad := f.newAcross(alice, "A and D", caseA, caseD)
			status, _ := f.call(dave, "GET", "/conversations/"+cd, nil)
			Expect(status).To(Equal(http.StatusNotFound), "dave audits D and has no role in C")
			status, out := f.call(alice, "PUT", "/cases/"+caseC+"/members/dave", map[string]any{"role": "auditor"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			status, _ = f.call(dave, "GET", "/conversations/"+cd, nil)
			Expect(status).To(Equal(http.StatusOK), "dave audits both C and D now")
			status, _ = f.call(dave, "GET", "/conversations/"+ad, nil)
			Expect(status).To(Equal(http.StatusNotFound), "in the open A dave is an analyst, not an auditor")
			status, _ = f.call(root, "GET", "/conversations/"+ad, nil)
			Expect(status).To(Equal(http.StatusOK), "a global administrator reads all")

			// hidden after being shared with the cases: found by an auditor of EVERY case it covers, an administrator and its owner, not by an auditor of some
			for _, t := range []string{cd, ad} {
				status, out = f.call(alice, "PATCH", "/conversations/"+t, map[string]any{"visibility": "case"})
				Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
				status, _ = f.call(alice, "DELETE", "/conversations/"+t, nil)
				Expect(status).To(Equal(http.StatusNoContent))
			}
			status, _ = f.call(dave, "GET", "/conversations/"+cd, nil)
			Expect(status).To(Equal(http.StatusOK), "an auditor of both C and D finds it hidden")
			status, _ = f.call(dave, "GET", "/conversations/"+ad, nil)
			Expect(status).To(Equal(http.StatusNotFound), "an auditor of D only, who is an analyst in the open A, does not")
			status, _ = f.call(erin, "GET", "/conversations/"+cd, nil)
			Expect(status).To(Equal(http.StatusNotFound), "erin may open both and is not an auditor")
			status, _ = f.call(root, "GET", "/conversations/"+ad, nil)
			Expect(status).To(Equal(http.StatusOK))
			status, _ = f.call(alice, "GET", "/conversations/"+ad, nil)
			Expect(status).To(Equal(http.StatusOK), "its owner finds it")
		})
	})

	Describe("what it leaves in the audit trail", func() {
		It("writes a row in the chain of each case, says that it spans cases and how many, and names none of the others", func() {
			tid := f.newAcross(alice, "audit across", caseC, caseD)
			for _, c := range []string{caseC, caseD} {
				other := caseD
				if c == caseD {
					other = caseC
				}
				items := f.auditItems(alice, c, tid, "thread.created")
				Expect(items).To(HaveLen(1), c)
				Expect(jget(items[0], "details", "cross_case")).To(BeTrue())
				Expect(jget(items[0], "details", "cases")).To(BeEquivalentTo(2))
				raw, _ := json.Marshal(items[0])
				Expect(string(raw)).NotTo(ContainSubstring(other), "the audit trail of one case does not name another")
			}
			status, out := f.call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "audit me"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
			for _, c := range []string{caseC, caseD} {
				started := 0
				for _, it := range f.auditItems(alice, c, "", "turn.started") {
					if jstr(it, "details", "thread") == tid {
						started++
						Expect(jstr(it, "details", "group")).To(Equal(jstr(out, "group", "group_id")))
					}
				}
				Expect(started).To(Equal(1), "one turn of the question in "+c)
			}
			status, _ = f.call(alice, "DELETE", "/conversations/"+tid, nil)
			Expect(status).To(Equal(http.StatusNoContent))
			status, _ = f.call(alice, "POST", "/conversations/"+tid+"/restore", nil)
			Expect(status).To(Equal(http.StatusOK))
			for _, c := range []string{caseC, caseD} {
				Expect(f.auditItems(alice, c, tid, "thread.hidden")).To(HaveLen(1), c)
				Expect(f.auditItems(alice, c, tid, "thread.restored")).To(HaveLen(1), c)
				f.chainOK(alice, c)
			}
		})
	})

	Describe("what can be done with a turn of one", func() {
		It("is done in the case of the turn: pin, feedback, re-run, compare, fork; and a conversation across cases has no tags", func() {
			tid := f.newAcross(alice, "turn actions", caseA, caseB)
			status, out := f.call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "Which numbers made the most calls?"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
			turnB := jstr(out, "group", "cases", "1", "turn", "turn_id")

			status, pin := f.call(alice, "POST", "/turns/"+turnB+"/pin", nil)
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(pin))
			Expect(jstr(pin, "finding", "case_id")).To(Equal(caseB), "a finding belongs to the case of the answer it came from")

			status, fb := f.call(alice, "POST", "/turns/"+turnB+"/feedback", map[string]any{"verdict": "correct"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(fb))

			f.eng.configure(func(req EngineRequest) (int, []byte) { return ranked1(req) }, nil, 0)
			status, re := f.call(alice, "POST", "/turns/"+turnB+"/rerun", nil)
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(re))
			Expect(jstr(re, "turn", "case_id")).To(Equal(caseB), "a re-run is asked in the case of the turn")
			Expect(jstr(re, "turn", "kind")).To(Equal("rerun"))
			Expect(jget(re, "diff", "changed")).To(BeTrue())
			status, diff := f.call(alice, "GET", "/turns/"+jstr(re, "turn", "turn_id")+"/diff", nil)
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(diff))

			status, fork := f.call(alice, "POST", "/turns/"+turnB+"/fork", map[string]any{"title": "only B"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(fork))
			Expect(fork["cross_case"]).To(BeFalse(), "a fork is a conversation about the one case of the turn")
			Expect(fork["case_id"]).To(Equal(caseB))
			status, next := f.call(alice, "POST", "/conversations/"+jstr(fork, "thread_id")+"/turns", map[string]any{"question": "And in B alone?"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(next))
			Expect(jstr(next, "turn", "parent_turn_id")).To(Equal(turnB))

			status, tags := f.call(alice, "PUT", "/cases/"+caseA+"/objects/thread/"+tid+"/tags", map[string]any{"tags": []string{"x"}})
			Expect(status).To(Equal(http.StatusBadRequest), fmt.Sprint(tags))
			Expect(jstr(tags, "error")).To(ContainSubstring("across cases cannot be tagged"))
			status, _ = f.call(alice, "PUT", "/cases/"+caseB+"/objects/turn/"+turnB+"/tags", map[string]any{"tags": []string{"checked"}})
			Expect(status).To(Equal(http.StatusOK), "a turn is one case's and can be tagged there")
		})
	})

	Describe("exporting one", func() {
		It("takes the answers of every case into one bundle, and leaves counts, never names, in the audit trail of each case", func() {
			tid := f.newAcross(alice, "export across", caseA, caseB)
			status, ans := f.call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "Which numbers made the most calls?"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(ans))
			status, out := f.call(alice, "POST", "/conversations/"+tid+"/export", nil)
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			Expect(out["case_ids"]).To(Equal([]any{caseA, caseB}))
			Expect(jlen(out, "exhibits")).To(Equal(2))
			Expect(jstr(out, "exhibits", "0", "case_id")).To(Equal(caseA))
			Expect(jstr(out, "exhibits", "1", "case_id")).To(Equal(caseB))
			Expect(jstr(out, "exhibits", "0", "answer", "headline")).To(HavePrefix("3 numbers"))
			Expect(jstr(out, "exhibits", "1", "answer", "headline")).To(HavePrefix("4 numbers"))
			for _, c := range []string{caseA, caseB} {
				other := caseB
				if c == caseB {
					other = caseA
				}
				items := f.auditItems(alice, c, tid, "thread.exported")
				Expect(items).To(HaveLen(1), c)
				Expect(jget(items[0], "details", "exhibits")).To(BeEquivalentTo(1))
				Expect(jget(items[0], "details", "cross_case")).To(BeTrue())
				Expect(jstr(items[0], "details", "bundle_sha256")).To(Equal(jstr(out, "bundle_sha256")))
				raw, _ := json.Marshal(items[0])
				Expect(string(raw)).NotTo(ContainSubstring(other), "the audit trail of one case does not name another")
			}
		})

		It("is refused to someone whose role in one of the cases does not allow it, and allowed to someone whose role does in all", func() {
			tid := f.newAcross(alice, "A and C, shared", caseA, caseC)
			status, out := f.call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "Which numbers made the most calls?"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
			status, out = f.call(alice, "PATCH", "/conversations/"+tid, map[string]any{"visibility": "case"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			status, out = f.call(bob, "POST", "/conversations/"+tid+"/export", nil)
			Expect(status).To(Equal(http.StatusForbidden), fmt.Sprint(out))
			Expect(jstr(out, "error")).To(ContainSubstring(caseC))
			status, out = f.call(erin, "POST", "/conversations/"+tid+"/export", nil)
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			Expect(jlen(out, "exhibits")).To(Equal(2))
		})
	})

	Describe("a credential bound to one case", func() {
		It("sees nothing of a conversation across cases, nor of the conversations and answers of another case, even by their ids", func() {
			across := f.newAcross(alice, "bound", caseA, caseB)
			singleB := f.newThread(alice, caseB, "only in B")
			turnB := jstr(f.ask(alice, singleB, "Which numbers made the most calls?"), "turn_id")
			bound := caller{subject: "alice", role: "user", bound: caseA}
			for _, path := range []string{"/conversations/" + across, "/conversations/" + singleB, "/turns/" + turnB, "/turns/" + turnB + "/ancestry", "/turns/" + turnB + "/feedback"} {
				status, out := f.call(bound, "GET", path, nil)
				Expect(status).To(Equal(http.StatusNotFound), path+" "+fmt.Sprint(out))
			}
			status, _ := f.call(bound, "POST", "/turns/"+turnB+"/rerun", nil)
			Expect(status).To(Equal(http.StatusNotFound))
			status, _ = f.call(bound, "POST", "/conversations/"+singleB+"/turns", map[string]any{"question": "x"})
			Expect(status).To(Equal(http.StatusNotFound))
			status, rail := f.call(bound, "GET", "/conversations?limit=100", nil)
			Expect(status).To(Equal(http.StatusOK))
			Expect(threadIDs(rail)).NotTo(ContainElement(across))
			Expect(threadIDs(rail)).NotTo(ContainElement(singleB))
		})
	})
})
