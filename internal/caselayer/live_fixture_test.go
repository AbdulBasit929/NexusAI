package caselayer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/gomega"

	. "github.com/onsi/ginkgo/v2"
)

// liveFixture is a service on the throwaway database of tools/dev/live-window/rehearse-034.sh with its own tenant, so that the specs that use it see nothing of each other. It runs as the application
// role (row-level security in force); the owner connection is for fixtures only. The specs that use it are skipped when CASELAYER_TEST_DSN is not set.
type liveFixture struct {
	ctx    context.Context
	pool   *pgxpool.Pool
	owner  *pgxpool.Pool
	svc    *Service
	eng    *scriptedEngine
	srv    *httptest.Server
	tenant string
}

func newLiveFixture() *liveFixture {
	f := &liveFixture{ctx: context.Background(), tenant: "t-" + uuid.NewString()[:8]}
	dsn := os.Getenv("CASELAYER_TEST_DSN")            //nolint:forbidigo // opt-in switch for a throwaway-database spec
	ownerDSN := os.Getenv("CASELAYER_TEST_OWNER_DSN") //nolint:forbidigo // set by the rehearsal script only
	if dsn == "" || ownerDSN == "" {
		Skip("needs the throwaway database of tools/dev/live-window/rehearse-034.sh (KEEP=1)")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg.ConnConfig.Host).To(Equal("127.0.0.1"))
	f.pool, err = pgxpool.NewWithConfig(f.ctx, cfg)
	Expect(err).NotTo(HaveOccurred())
	ocfg, err := pgxpool.ParseConfig(ownerDSN)
	Expect(err).NotTo(HaveOccurred())
	Expect(ocfg.ConnConfig.Host).To(Equal("127.0.0.1"))
	f.owner, err = pgxpool.NewWithConfig(f.ctx, ocfg)
	Expect(err).NotTo(HaveOccurred())
	var marker bool
	Expect(f.owner.QueryRow(f.ctx, "SELECT to_regclass('public.nx_throwaway_marker') IS NOT NULL").Scan(&marker)).To(Succeed())
	Expect(marker).To(BeTrue(), "this is not a throwaway database")
	var current string
	Expect(f.pool.QueryRow(f.ctx, "SELECT current_user").Scan(&current)).To(Succeed())
	Expect(current).To(Equal("forensic_runtime"), "the layer must be tested as the application role, under row-level security")

	f.eng = &scriptedEngine{}
	f.eng.configure(ranked1, nil, 0)
	f.svc = NewService(NewStore(f.pool), f.eng, Config{Heartbeat: 20 * time.Millisecond, TurnTimeout: time.Minute, EngineVersion: "spec-build"})
	mux := http.NewServeMux()
	(&Handler{Svc: f.svc, Scope: testScope}).Register(mux)
	f.srv = httptest.NewServer(mux)
	return f
}

func (f *liveFixture) close() {
	if f.srv != nil {
		f.srv.Close()
	}
	if f.pool != nil {
		f.pool.Close()
	}
	if f.owner != nil {
		f.owner.Close()
	}
}

func (f *liveFixture) call(c caller, method, path string, body any) (int, map[string]any) {
	resp, raw := c.do(f.srv.URL, f.tenant, method, path, body, "")
	return resp.StatusCode, jsonMap(raw)
}

// stream calls a route that streams and returns its events.
func (f *liveFixture) stream(c caller, path string, body any) (int, []map[string]any) {
	resp, raw := c.do(f.srv.URL, f.tenant, "POST", path, body, "application/x-ndjson")
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, []map[string]any{jsonMap(raw)}
	}
	return resp.StatusCode, ndjsonEvents(raw)
}

// makeCase gives a case a record, which is what makes it a case that exists.
func (f *liveFixture) makeCase(caseID string) {
	_, err := f.owner.Exec(f.ctx, `INSERT INTO forensic.case_record (tenant_id, collection_id, display_name) VALUES ($1, $2, $3)`, f.tenant, caseID, "Spec case "+caseID)
	Expect(err).NotTo(HaveOccurred())
}

// addEvidence registers a file in a case (which moves the data version of the case).
func (f *liveFixture) addEvidence(caseID, name string) {
	sum := sha256.Sum256([]byte(name + uuid.NewString()))
	_, err := f.owner.Exec(f.ctx, `INSERT INTO forensic.evidence_items (tenant_id, collection_id, user_id, original_filename, source_file, sha256, size_bytes) VALUES ($1, $2, 'test', $3, $3, $4, 10)`,
		f.tenant, caseID, name, hex.EncodeToString(sum[:]))
	Expect(err).NotTo(HaveOccurred())
}

// control makes a case a controlled one: the first person named becomes its owner, the rest are added by that owner. roles maps a subject to its role; the owner is the first of owners.
func (f *liveFixture) control(caseID string, owner caller, others map[string]string) {
	status, out := f.call(owner, "PUT", "/cases/"+caseID+"/members/"+owner.subject, map[string]any{"role": "owner"})
	Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
	for subject, role := range others {
		status, out = f.call(owner, "PUT", "/cases/"+caseID+"/members/"+subject, map[string]any{"role": role})
		Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
	}
}

func (f *liveFixture) newThread(c caller, caseID, title string) string {
	status, out := f.call(c, "POST", "/conversations", map[string]any{"case_id": caseID, "title": title})
	Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
	return jstr(out, "thread_id")
}

func (f *liveFixture) newAcross(c caller, title string, caseIDs ...string) string {
	status, out := f.call(c, "POST", "/conversations", map[string]any{"case_ids": caseIDs, "title": title})
	Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
	return jstr(out, "thread_id")
}

// ask puts a question to a conversation about one case and returns the turn it made.
func (f *liveFixture) ask(c caller, threadID, question string) map[string]any {
	status, out := f.call(c, "POST", "/conversations/"+threadID+"/turns", map[string]any{"question": question})
	Expect(status).To(Equal(http.StatusCreated), fmt.Sprint(out))
	return out["turn"].(map[string]any)
}

// auditItems reads the audit trail of a case for one object (and, when given, one action).
func (f *liveFixture) auditItems(c caller, caseID, objectID, action string) []any {
	path := "/cases/" + caseID + "/audit?limit=100"
	if objectID != "" {
		path += "&object_id=" + objectID
	}
	if action != "" {
		path += "&action=" + action
	}
	status, out := f.call(c, "GET", path, nil)
	Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
	items, _ := out["items"].([]any)
	return items
}

func (f *liveFixture) chainOK(c caller, caseID string) {
	status, out := f.call(c, "GET", "/cases/"+caseID+"/audit/verify", nil)
	Expect(status).To(Equal(http.StatusOK), fmt.Sprint(out))
	Expect(out["ok"]).To(BeTrue(), "the audit chain of "+caseID+" is intact")
}
