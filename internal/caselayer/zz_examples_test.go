package caselayer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TEMPORARY (not committed): records the responses the contract document quotes, from the running code on the throwaway database.
func trimAnswer(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			if k == "answer" {
				if m, ok := val.(map[string]any); ok && m["policy"] != nil {
					out[k] = map[string]any{"...": "the engine's response exactly as POST /query/hybrid returns it"}
					continue
				}
			}
			out[k] = trimAnswer(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = trimAnswer(e)
		}
		return out
	}
	return v
}

var _ = Describe("recording the examples of the contract", Ordered, func() {
	It("records", func() {
		dir := os.Getenv("CASELAYER_EXAMPLES_DIR") //nolint:forbidigo // temporary recording
		if dir == "" {
			Skip("not recording")
		}
		f := newLiveFixture()
		defer f.close()
		orion, meridian := "orion-lahore-reference-20261030", "meridian-islamabad-reference-20261102"
		f.makeCase(orion)
		f.makeCase(meridian)
		f.addEvidence(orion, "calls.csv")
		f.eng.configure(func(req EngineRequest) (int, []byte) {
			if req.CaseID == meridian {
				return ranked2(req)
			}
			return ranked1(req)
		}, nil, 0)
		save := func(name string, v any) {
			raw, err := json.MarshalIndent(trimAnswer(v), "", "  ")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(filepath.Join(dir, name), raw, 0o600)).To(Succeed())
		}

		status, thread := f.call(alice, "POST", "/conversations", map[string]any{"case_ids": []string{orion, meridian}, "title": "Who called the most, in both cases"})
		Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(thread))
		save("01-create-across.json", thread)
		tid := jstr(thread, "thread_id")

		status, asked := f.call(alice, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "Which numbers made the most calls?", "client_request_id": "ask-1"})
		Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(asked))
		save("02-ask-across.json", asked)

		resp, raw := alice.do(f.srv.URL, f.tenant, "POST", "/conversations/"+tid+"/turns", map[string]any{"question": "and for the second week?"}, "application/x-ndjson")
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		var lines []string
		for _, e := range ndjsonEvents(raw) {
			b, _ := json.Marshal(trimTurn(e))
			lines = append(lines, string(b))
		}
		Expect(os.WriteFile(filepath.Join(dir, "03-stream-across.ndjson"), []byte(strings.Join(lines, "\n")+"\n"), 0o600)).To(Succeed())

		status, got := f.call(alice, "GET", "/conversations/"+tid+"?answers=none", nil)
		Expect(status).To(Equal(http.StatusOK))
		save("04-get-across.json", got)

		status, bundle := f.call(alice, "POST", "/conversations/"+tid+"/export", map[string]any{"max_rows": 3})
		Expect(status).To(Equal(http.StatusOK), fmt.Sprint(bundle))
		md := jstr(bundle, "markdown")
		save("05-export-across.json", bundle)
		Expect(os.WriteFile(filepath.Join(dir, "05-export-across.md"), []byte(md), 0o600)).To(Succeed())

		single := f.newThread(alice, orion, "For the report")
		turn := f.ask(alice, single, "Which numbers made the most calls?")
		status, pin := f.call(alice, "POST", "/turns/"+jstr(turn, "turn_id")+"/pin", map[string]any{"title": "Top callers", "note": "The first number is the second handset."})
		Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(pin))
		status, ex := f.call(alice, "POST", "/cases/"+orion+"/exhibits", map[string]any{"finding_ids": []string{jstr(pin, "finding", "finding_id")}, "title": "Report one", "max_rows": 3})
		Expect(status).To(Equal(http.StatusOK), fmt.Sprint(ex))
		save("06-exhibits.json", ex)

		items := f.auditItems(alice, orion, tid, "")
		save("07-audit-across.json", map[string]any{"items": items})
	})
})

func trimTurn(e map[string]any) map[string]any {
	if t, ok := e["turn"].(map[string]any); ok {
		out := map[string]any{}
		for k, v := range e {
			out[k] = v
		}
		short := map[string]any{}
		for _, k := range []string{"turn_id", "case_id", "group_id", "idx", "state", "outcome", "headline"} {
			short[k] = t[k]
		}
		out["turn"] = short
		return out
	}
	return e
}
