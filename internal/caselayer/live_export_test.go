package caselayer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// recomputeHashes checks a bundle the way a reader of the JSON would: each exhibit without its sha256 and context, keys in order, numbers as written, hashed; then the bundle from those.
func recomputeHashes(raw []byte) (exhibits []string, bundle string, caseIDs []string) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc map[string]any
	Expect(dec.Decode(&doc)).To(Succeed())
	for _, c := range doc["case_ids"].([]any) {
		caseIDs = append(caseIDs, c.(string))
	}
	for _, e := range doc["exhibits"].([]any) {
		ex := e.(map[string]any)
		stored := ex["sha256"].(string)
		delete(ex, "sha256")
		delete(ex, "context")
		canon, err := canonicalJSON(ex)
		Expect(err).NotTo(HaveOccurred())
		got := sha256Hex(canon)
		Expect(got).To(Equal(stored), "the hash of an exhibit is the SHA-256 of its canonical JSON without sha256 and context")
		exhibits = append(exhibits, got)
	}
	return exhibits, doc["bundle_sha256"].(string), caseIDs
}

var _ = Describe("export to the report builder on a throwaway database", Ordered, func() {
	var (
		f        *liveFixture
		id       = uuid.NewString()[:6]
		expA     = "ea-" + id // open
		expC     = "ec-" + id // controlled: alice owner, bob viewer, dave auditor, erin analyst
		expOther = "eo-" + id // open
		zed      = caller{subject: "zed", role: "user"}
		mode     = "ranked1"
	)
	respond := func(req EngineRequest) (int, []byte) {
		switch mode {
		case "abstain":
			return http.StatusOK, abstainBody()
		case "ranked2":
			return ranked2(req)
		}
		return ranked1(req)
	}
	exportJSON := func(c caller, tid string, body any) (int, []byte) {
		resp, raw := c.do(f.srv.URL, f.tenant, "POST", "/conversations/"+tid+"/export", body, "")
		return resp.StatusCode, raw
	}

	BeforeAll(func() {
		f = newLiveFixture()
		for _, c := range []string{expA, expC, expOther} {
			f.makeCase(c)
		}
		f.addEvidence(expA, "calls.csv")
		f.control(expC, alice, map[string]string{"bob": "viewer", "dave": "auditor", "erin": "analyst"})
	})
	AfterAll(func() {
		if f != nil {
			f.close()
		}
	})
	BeforeEach(func() {
		if f != nil {
			mode = "ranked1"
			f.eng.configure(respond, nil, 0)
		}
	})

	Describe("a conversation", func() {
		var tid, q1, q2, q3, rerun string
		var firstBundle []byte
		BeforeAll(func() {
			tid = f.newThread(alice, expA, "Calls of the number")
			q1 = jstr(f.ask(alice, tid, "Which numbers made the most calls?"), "turn_id")
			mode = "abstain"
			f.eng.configure(respond, nil, 0)
			q2 = jstr(f.ask(alice, tid, "What did the suspect have for lunch?"), "turn_id")
			mode = "ranked1"
			q3 = jstr(f.ask(alice, tid, "Which numbers received the most calls?"), "turn_id")
			mode = "ranked2"
			status, re := f.call(alice, "POST", "/turns/"+q1+"/rerun", nil)
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(re))
			rerun = jstr(re, "turn", "turn_id")
			mode = "ranked1"
		})

		It("exports the latest answered run of each question, in the order the questions were asked, and lists what it left out", func() {
			status, raw := exportJSON(alice, tid, nil)
			Expect(status).To(Equal(http.StatusOK), string(raw))
			firstBundle = raw
			out := jsonMap(raw)
			Expect(out["format"]).To(Equal("nexusai.exhibits/1"))
			Expect(out["title"]).To(Equal("Calls of the number"))
			Expect(out["case_ids"]).To(Equal([]any{expA}))
			Expect(out["exported_by"]).To(Equal("alice"))
			Expect(jlen(out, "exhibits")).To(Equal(2))
			ex0, ex1 := jget(out, "exhibits", "0"), jget(out, "exhibits", "1")
			Expect(jstr(ex0, "exhibit_id")).To(Equal("turn:"+rerun), "the re-run replaces the run it re-ran, where the first question stood")
			Expect(jstr(ex1, "exhibit_id")).To(Equal("turn:" + q3))
			Expect(jstr(ex0, "kind")).To(Equal("answer"))
			Expect(jstr(ex0, "case_id")).To(Equal(expA))
			Expect(jstr(ex0, "question")).To(Equal("Which numbers made the most calls?"))
			Expect(jstr(ex0, "answer", "headline")).To(HavePrefix("4 numbers"))
			Expect(jstr(ex0, "answer", "outcome")).To(Equal("answered"))
			Expect(jstr(ex0, "answer", "figures", "0", "label")).To(Equal("Number of records"))
			Expect(jget(ex0, "answer", "table", "rows_shown")).To(BeEquivalentTo(4))
			Expect(jget(ex0, "answer", "table", "truncated")).To(BeFalse())
			Expect(jstr(ex0, "answer", "table", "columns", "0", "header")).To(Equal("Number"))
			Expect(jlen(ex0, "evidence")).To(Equal(1))
			Expect(jstr(ex0, "evidence", "0", "source_file")).To(Equal("calls.csv"))
			Expect(jstr(ex0, "provenance", "run_kind")).To(Equal("rerun"))
			Expect(jstr(ex0, "provenance", "asked_by")).To(Equal("alice"))
			Expect(jstr(ex0, "provenance", "engine_version")).To(Equal("test-1"))
			Expect(jstr(ex0, "provenance", "model")).To(Equal("test-model"))
			Expect(jstr(ex0, "provenance", "query")).To(ContainSubstring("SELECT src AS label"))
			Expect(jstr(ex0, "provenance", "query_hash")).To(MatchRegexp(`^[0-9a-f]{64}$`))
			Expect(jstr(ex0, "provenance", "data_version_hash")).To(MatchRegexp(`^[0-9a-f]{64}$`))
			Expect(jget(ex0, "data", "recorded")).To(BeTrue())
			Expect(jlen(ex0, "data", "evidence")).To(Equal(1))
			Expect(jstr(ex0, "data", "evidence", "0", "name")).To(Equal("calls.csv"))
			Expect(jget(ex0, "context", "data_current")).To(BeTrue())
			Expect(jstr(ex0, "reproduce", "rerun")).To(Equal("POST /turns/" + rerun + "/rerun"))
			Expect(jlen(out, "skipped")).To(Equal(1))
			Expect(jstr(out, "skipped", "0", "ref")).To(Equal("turn:" + q2))
			Expect(jstr(out, "skipped", "0", "reason")).To(ContainSubstring("abstained"))
			md := jstr(out, "markdown")
			Expect(md).To(HavePrefix("# Calls of the number\n"))
			Expect(md).To(ContainSubstring(jstr(out, "bundle_sha256")))
			Expect(md).To(ContainSubstring("## Not exported"))

			hashes, bundle, cases := recomputeHashes(raw)
			Expect(bundle).To(Equal(bundleHash(cases, hashes)))
		})

		It("writes the hash of the bundle and the counts, never a word of the question, to the audit trail", func() {
			items := f.auditItems(alice, expA, tid, "thread.exported")
			Expect(items).To(HaveLen(1))
			out := jsonMap(firstBundle)
			Expect(jstr(items[0], "details", "bundle_sha256")).To(Equal(jstr(out, "bundle_sha256")))
			Expect(jget(items[0], "details", "exhibits")).To(BeEquivalentTo(2))
			Expect(jget(items[0], "details", "skipped")).To(BeEquivalentTo(1))
			Expect(jstr(items[0], "actor")).To(Equal("alice"))
			raw, _ := json.Marshal(items[0])
			Expect(string(raw)).NotTo(ContainSubstring("Which numbers"))
			f.chainOK(alice, expA)
		})

		It("says the data has changed when the case has moved on, and still has the same hashes", func() {
			f.addEvidence(expA, "late-arrival.csv")
			status, raw := exportJSON(alice, tid, nil)
			Expect(status).To(Equal(http.StatusOK), string(raw))
			out := jsonMap(raw)
			Expect(jget(out, "exhibits", "0", "context", "data_current")).To(BeFalse())
			Expect(jstr(out, "markdown")).To(ContainSubstring("The case's data has changed since this answer was computed."))
			Expect(out["bundle_sha256"]).To(Equal(jsonMap(firstBundle)["bundle_sha256"]), "the answer did not change because the case did")
			Expect(jstr(out, "exhibits", "0", "sha256")).To(Equal(jstr(jsonMap(firstBundle), "exhibits", "0", "sha256")))
			Expect(f.auditItems(alice, expA, tid, "thread.exported")).To(HaveLen(2), "every export is a row")
		})

		It("takes the answers named, in the order named, and lists a named question that was not answered", func() {
			status, raw := exportJSON(alice, tid, map[string]any{"turn_ids": []string{q3, q1}})
			Expect(status).To(Equal(http.StatusOK), string(raw))
			out := jsonMap(raw)
			Expect(jstr(out, "exhibits", "0", "exhibit_id")).To(Equal("turn:" + q3))
			Expect(jstr(out, "exhibits", "1", "exhibit_id")).To(Equal("turn:" + q1))
			Expect(jstr(out, "exhibits", "1", "answer", "headline")).To(HavePrefix("3 numbers"), "the first run, as it was")
			Expect(jlen(out, "skipped")).To(Equal(0))
			status, raw = exportJSON(alice, tid, map[string]any{"turn_ids": []string{q3, q2}})
			Expect(status).To(Equal(http.StatusOK), string(raw))
			Expect(jlen(jsonMap(raw), "exhibits")).To(Equal(1))
			Expect(jstr(jsonMap(raw), "skipped", "0", "ref")).To(Equal("turn:" + q2))
			status, raw = exportJSON(alice, tid, map[string]any{"turn_ids": []string{q2}})
			Expect(status).To(Equal(http.StatusConflict), string(raw))
			Expect(jsonMap(raw)["code"]).To(Equal("nothing_to_export"))
		})

		It("refuses what is not an answer of this conversation, and a request that makes no sense", func() {
			other := f.newThread(alice, expA, "another conversation")
			foreign := jstr(f.ask(alice, other, "Which numbers made the most calls?"), "turn_id")
			for name, body := range map[string]map[string]any{
				"an unknown answer":   {"turn_ids": []string{uuid.NewString()}},
				"another conversation": {"turn_ids": []string{foreign}},
			} {
				status, raw := exportJSON(alice, tid, body)
				Expect(status).To(Equal(http.StatusNotFound), name+" "+string(raw))
			}
			for name, body := range map[string]map[string]any{
				"not an identifier": {"turn_ids": []string{"not-a-uuid"}},
				"too many rows":     {"max_rows": 501},
				"no rows":           {"max_rows": 0},
				"a field of another call": {"finding_ids": []string{uuid.NewString()}},
				"a list that is not one":  {"turn_ids": "x"},
			} {
				status, raw := exportJSON(alice, tid, body)
				Expect(status).To(Equal(http.StatusBadRequest), name+" "+string(raw))
			}
		})

		It("prints as many rows as asked and says the table was cut", func() {
			status, raw := exportJSON(alice, tid, map[string]any{"max_rows": 2, "title": "Short"})
			Expect(status).To(Equal(http.StatusOK), string(raw))
			out := jsonMap(raw)
			Expect(out["title"]).To(Equal("Short"))
			Expect(jget(out, "exhibits", "0", "answer", "table", "rows_shown")).To(BeEquivalentTo(2))
			Expect(jget(out, "exhibits", "0", "answer", "table", "row_count")).To(BeEquivalentTo(4))
			Expect(jget(out, "exhibits", "0", "answer", "table", "truncated")).To(BeTrue())
			Expect(jstr(out, "markdown")).To(ContainSubstring("Showing 2 of 4 rows."))
		})

		It("gives the Markdown alone, with the hash of the bundle in a header", func() {
			resp, raw := alice.do(f.srv.URL, f.tenant, "POST", "/conversations/"+tid+"/export?format=markdown", nil, "")
			Expect(resp.StatusCode).To(Equal(http.StatusOK), string(raw))
			Expect(resp.Header.Get("Content-Type")).To(HavePrefix("text/markdown"))
			Expect(resp.Header.Get("X-Bundle-SHA256")).To(Equal(jstr(jsonMap(firstBundle), "bundle_sha256")))
			Expect(string(raw)).To(HavePrefix("# Calls of the number\n"))
		})
	})

	Describe("nothing to export", func() {
		It("is a conflict for a conversation with no answer, and for a hidden one", func() {
			empty := f.newThread(alice, expA, "empty")
			status, raw := exportJSON(alice, empty, nil)
			Expect(status).To(Equal(http.StatusConflict), string(raw))
			Expect(jsonMap(raw)["code"]).To(Equal("nothing_to_export"))
			mode = "abstain"
			f.eng.configure(respond, nil, 0)
			only := f.newThread(alice, expA, "only abstentions")
			f.ask(alice, only, "Why?")
			status, raw = exportJSON(alice, only, nil)
			Expect(status).To(Equal(http.StatusConflict), string(raw))
			Expect(jstr(jsonMap(raw), "error")).To(ContainSubstring("1 question(s) were not answered"))
			mode = "ranked1"
			f.eng.configure(respond, nil, 0)
			hidden := f.newThread(alice, expA, "hidden")
			f.ask(alice, hidden, "Which numbers made the most calls?")
			status, _ = f.call(alice, "DELETE", "/conversations/"+hidden, nil)
			Expect(status).To(Equal(http.StatusNoContent))
			status, raw = exportJSON(alice, hidden, nil)
			Expect(status).To(Equal(http.StatusConflict), string(raw))
			Expect(jsonMap(raw)["code"]).To(Equal("hidden"))
			status, _ = exportJSON(bob, hidden, nil)
			Expect(status).To(Equal(http.StatusNotFound))
		})
	})

	Describe("who may export", func() {
		It("is anyone who works on the case; a viewer and an auditor read it but cannot take it out; someone outside does not see it", func() {
			tid := f.newThread(alice, expC, "controlled")
			f.ask(alice, tid, "Which numbers made the most calls?")
			status, out := f.call(alice, "PATCH", "/conversations/"+tid, map[string]any{"visibility": "case"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			for _, who := range []caller{bob, dave} {
				status, raw := exportJSON(who, tid, nil)
				Expect(status).To(Equal(http.StatusForbidden), who.subject+" "+string(raw))
				Expect(jstr(jsonMap(raw), "error")).To(ContainSubstring("does not allow exporting"))
			}
			status, raw := exportJSON(erin, tid, nil)
			Expect(status).To(Equal(http.StatusOK), string(raw))
			items := f.auditItems(alice, expC, tid, "thread.exported")
			Expect(items).To(HaveLen(1), "only the export that was made")
			Expect(jstr(items[0], "actor")).To(Equal("erin"))
			Expect(jstr(items[0], "actor_role")).To(Equal("analyst"))
			status, _ = exportJSON(zed, tid, nil)
			Expect(status).To(Equal(http.StatusNotFound))
		})
	})

	Describe("findings and answers of a case", func() {
		var tid, t1, fid1, fid2 string
		BeforeAll(func() {
			tid = f.newThread(alice, expA, "for the report")
			t1 = jstr(f.ask(alice, tid, "Which numbers made the most calls?"), "turn_id")
			status, pin := f.call(alice, "POST", "/turns/"+t1+"/pin", map[string]any{"title": "Top callers", "note": "The first number is the second handset."})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(pin))
			fid1 = jstr(pin, "finding", "finding_id")
			status, plain := f.call(alice, "POST", "/cases/"+expA+"/findings", map[string]any{"title": "A finding with no answer behind it", "body": "Seen at the scene.", "basis": "inferred"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(plain))
			fid2 = jstr(plain, "finding_id")
		})

		It("exports findings first and answers after, in the order asked for, with the review of each", func() {
			status, out := f.call(alice, "POST", "/cases/"+expA+"/exhibits", map[string]any{"finding_ids": []string{fid1, fid2}, "turn_ids": []string{t1}, "title": "Report one"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			Expect(out["title"]).To(Equal("Report one"))
			Expect(jlen(out, "exhibits")).To(Equal(3))
			Expect(jstr(out, "exhibits", "0", "exhibit_id")).To(Equal("finding:" + fid1))
			Expect(jstr(out, "exhibits", "1", "exhibit_id")).To(Equal("finding:" + fid2))
			Expect(jstr(out, "exhibits", "2", "exhibit_id")).To(Equal("turn:" + t1))
			Expect(jstr(out, "exhibits", "0", "kind")).To(Equal("finding"))
			Expect(jstr(out, "exhibits", "0", "title")).To(Equal("Top callers"))
			Expect(jstr(out, "exhibits", "0", "finding", "body")).To(ContainSubstring("second handset"))
			Expect(jstr(out, "exhibits", "0", "finding", "review_state")).To(Equal("unreviewed"))
			Expect(jstr(out, "exhibits", "0", "answer", "headline")).To(HavePrefix("3 numbers"), "the answer as it was when pinned")
			Expect(jstr(out, "exhibits", "0", "provenance", "turn_id")).To(Equal(t1))
			Expect(jstr(out, "exhibits", "0", "provenance", "engine_version")).To(Equal("test-1"))
			Expect(jstr(out, "exhibits", "1", "finding", "basis")).To(Equal("inferred"))
			Expect(jget(out, "exhibits", "1", "answer")).To(BeNil())
			Expect(jget(out, "exhibits", "1", "reproduce")).To(BeNil())
			items := f.auditItems(alice, expA, expA, "exhibits.exported")
			Expect(items).To(HaveLen(1))
			Expect(jget(items[0], "details", "findings")).To(BeEquivalentTo(2))
			Expect(jget(items[0], "details", "answers")).To(BeEquivalentTo(1))
			Expect(jget(items[0], "details", "exhibits")).To(BeEquivalentTo(3))
			Expect(jstr(items[0], "object_kind")).To(Equal("case"))
		})

		It("leaves out a rejected finding, and with only_confirmed one that is not confirmed yet", func() {
			status, out := f.call(bob, "POST", "/findings/"+fid2+"/review", map[string]any{"state": "rejected", "note": "not what the file shows"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			status, out = f.call(alice, "POST", "/cases/"+expA+"/exhibits", map[string]any{"finding_ids": []string{fid1, fid2}, "turn_ids": []string{t1}})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			Expect(jlen(out, "exhibits")).To(Equal(2))
			Expect(jstr(out, "skipped", "0", "ref")).To(Equal("finding:" + fid2))
			Expect(jstr(out, "skipped", "0", "reason")).To(ContainSubstring("rejected"))
			status, out = f.call(alice, "POST", "/cases/"+expA+"/exhibits", map[string]any{"finding_ids": []string{fid1}, "turn_ids": []string{t1}, "only_confirmed": true})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			Expect(jlen(out, "exhibits")).To(Equal(1))
			Expect(jstr(out, "exhibits", "0", "exhibit_id")).To(Equal("turn:" + t1))
			Expect(jstr(out, "skipped", "0", "reason")).To(ContainSubstring("not confirmed"))
			status, out = f.call(bob, "POST", "/findings/"+fid1+"/review", map[string]any{"state": "confirmed"})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			status, out = f.call(alice, "POST", "/cases/"+expA+"/exhibits", map[string]any{"finding_ids": []string{fid1}, "only_confirmed": true})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			Expect(jstr(out, "exhibits", "0", "finding", "review_state")).To(Equal("confirmed"))
			Expect(jstr(out, "exhibits", "0", "finding", "reviewed_by")).To(Equal("bob"))
			Expect(jstr(out, "markdown")).To(ContainSubstring("Review: confirmed by bob"))
			status, out = f.call(alice, "POST", "/cases/"+expA+"/exhibits", map[string]any{"finding_ids": []string{fid2}})
			Expect(status).To(Equal(http.StatusConflict), fmt.Sprint(out))
			Expect(out["code"]).To(Equal("nothing_to_export"))
		})

		It("exports only what belongs to the case and to a conversation the caller may read, and asks what to export", func() {
			status, other := f.call(alice, "POST", "/cases/"+expOther+"/findings", map[string]any{"title": "A finding of another case", "body": "x"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(other))
			status, _ = f.call(alice, "POST", "/cases/"+expA+"/exhibits", map[string]any{"finding_ids": []string{jstr(other, "finding_id")}})
			Expect(status).To(Equal(http.StatusNotFound))
			otherThread := f.newThread(alice, expOther, "other")
			foreign := jstr(f.ask(alice, otherThread, "Which numbers made the most calls?"), "turn_id")
			status, _ = f.call(alice, "POST", "/cases/"+expA+"/exhibits", map[string]any{"turn_ids": []string{foreign}})
			Expect(status).To(Equal(http.StatusNotFound), "an answer of another case")
			status, _ = f.call(bob, "POST", "/cases/"+expA+"/exhibits", map[string]any{"turn_ids": []string{t1}})
			Expect(status).To(Equal(http.StatusNotFound), "bob does not see alice's private conversation, so it does not give up its answers")
			status, _ = f.call(alice, "POST", "/cases/"+expA+"/exhibits", map[string]any{})
			Expect(status).To(Equal(http.StatusBadRequest))
			status, _ = f.call(alice, "POST", "/cases/"+expA+"/exhibits", map[string]any{"finding_ids": []string{"x"}})
			Expect(status).To(Equal(http.StatusBadRequest))
			status, _ = f.call(bob, "POST", "/cases/"+expC+"/exhibits", map[string]any{"finding_ids": []string{uuid.NewString()}})
			Expect(status).To(Equal(http.StatusForbidden), "a viewer of the controlled case cannot take anything out")
			status, _ = f.call(zed, "POST", "/cases/"+expC+"/exhibits", map[string]any{"finding_ids": []string{uuid.NewString()}})
			Expect(status).To(Equal(http.StatusNotFound), "and someone who is not a member does not know that it exists")
			f.chainOK(alice, expA)
		})

		It("cannot have its Markdown broken by what a finding says", func() {
			status, bad := f.call(alice, "POST", "/cases/"+expA+"/findings", map[string]any{"title": "Pipe | and <b>markup</b>", "body": "line one\n# not a heading\n<script>alert(1)</script>"})
			Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(bad))
			status, out := f.call(alice, "POST", "/cases/"+expA+"/exhibits", map[string]any{"finding_ids": []string{jstr(bad, "finding_id")}})
			Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
			md := jstr(out, "markdown")
			Expect(md).NotTo(ContainSubstring("<script>"))
			Expect(md).NotTo(ContainSubstring("<b>"))
			Expect(md).To(ContainSubstring("&lt;script&gt;alert(1)&lt;/script&gt;"))
			Expect(strings.Contains(md, "\n# not a heading")).To(BeFalse(), "a line of a body is a line of a quote, not a heading")
		})
	})
})
