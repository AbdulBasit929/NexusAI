package main

// Database acceptance tests of the known sets (gates K1 to K7 of docs/data/image-known-sets-preregistration-20261124.md).
// They need a throwaway PostgreSQL with migrations 001 to 015, 038 and 039 applied
// (db/forensic_records/ops/039_image_known_sets/Rehearse-039.ps1 -KeepContainer -PublishPort 55439):
//
//	IMAGE_SEARCH_TEST_DB=postgres://localrecall:<password>@127.0.0.1:55439/localrecall go test ./api/forensic_records -run TestKnownSetsDB -count=1 -v
//
// Without IMAGE_SEARCH_TEST_DB they skip. Every run uses tenants with a random suffix, so a rerun never collides with an earlier one; they write nothing into a case database.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type knownCall struct {
	k      knownSetsDeps
	tenant string
}

func (c knownCall) do(t *testing.T, handler http.HandlerFunc, method, target string, body any, id string) (int, map[string]any) {
	t.Helper()
	var reader *strings.Reader
	if body == nil {
		reader = strings.NewReader("")
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = strings.NewReader(string(raw))
	}
	sep := "?"
	if strings.Contains(target, "?") {
		sep = "&"
	}
	req := httptest.NewRequest(method, target+sep+"tenant_id="+c.tenant, reader)
	if id != "" {
		req.SetPathValue("id", id)
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func randomHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func evidenceSet(m map[string]any, key string) map[string]bool {
	out := map[string]bool{}
	list, _ := m[key].([]any)
	for _, item := range list {
		if row, ok := item.(map[string]any); ok {
			if id, ok := row["evidence_id"].(string); ok {
				out[id] = true
			}
		}
	}
	return out
}

func TestKnownSetsDB(t *testing.T) {
	pool := imageSearchTestPool(t)
	ctx := context.Background()
	suffix := randomHex(t, 3)
	tenantA, tenantB := "kn-a-"+suffix, "kn-b-"+suffix
	const col, other = "col-1", "col-2"

	id := func() string { return uuid.NewString() }
	shaP := randomHex(t, 32)
	P, P2, P3, U1, U2, DUP, NOHASH, OTHERCOL, BONLY := id(), id(), id(), id(), id(), id(), id(), id(), id()
	specs := []seedImageSpec{
		{tenantA, col, "case-1", P, shaP, "completed", bitsOf(0), "", ""},                      // the set member
		{tenantA, col, "case-1", P2, randomHex(t, 32), "completed", bitsOf(12), "", ""},        // re-saved: 12 bits away
		{tenantA, col, "case-1", P3, randomHex(t, 32), "completed", bitsOf(90), "", ""},        // a heavy crop: beyond the 56 bits
		{tenantA, col, "case-1", U1, randomHex(t, 32), "completed", bitsOf(120), "", ""},       // unrelated
		{tenantA, col, "case-1", U2, randomHex(t, 32), "completed", bitsOf(128), "", ""},       // unrelated
		{tenantA, col, "case-1", DUP, shaP, "duplicate", "", "", ""},                           // an identical re-upload of P
		{tenantA, col, "case-1", NOHASH, randomHex(t, 32), "completed", "", "", ""},            // no hash yet
		{tenantA, other, "case-9", OTHERCOL, randomHex(t, 32), "completed", bitsOf(0), "", ""}, // same hash, another collection
		{tenantB, col, "case-1", BONLY, randomHex(t, 32), "completed", bitsOf(0), "", ""},      // another tenant, same hash
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range specs {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", s.Tenant); err != nil {
			t.Fatal(err)
		}
		if err := seedImage(ctx, tx, s); err != nil {
			_ = tx.Rollback(ctx)
			t.Skipf("seeding failed (a database with migrations 001-015, 038 and 039?): %v", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, err := pool.Begin(ctx)
		if err != nil {
			return
		}
		defer func() { _ = c.Rollback(ctx) }()
		for _, tenant := range []string{tenantA, tenantB} {
			for _, table := range []string{"image_known_event", "image_known_hash", "image_known_set", "image_hash"} {
				_, _ = c.Exec(ctx, "DELETE FROM forensic."+table+" WHERE tenant_id = $1", tenant)
			}
		}
		_ = c.Commit(ctx)
	})

	k := knownSetsDeps{db: pool, hashMax: 56}
	a := knownCall{k: k, tenant: tenantA}
	b := knownCall{k: k, tenant: tenantB}

	var setID string
	t.Run("create: a set, its name is unique among active sets, a bad kind is refused", func(t *testing.T) {
		code, out := a.do(t, k.create, "POST", "/images/known-sets", map[string]any{"name": "watch", "kind": "of_interest", "description": "test"}, "")
		if code != http.StatusCreated {
			t.Fatalf("HTTP %d %v", code, out)
		}
		setID = out["set"].(map[string]any)["set_id"].(string)
		if code, _ := a.do(t, k.create, "POST", "/images/known-sets", map[string]any{"name": " WATCH ", "kind": "of_interest"}, ""); code != http.StatusConflict {
			t.Errorf("a second active set of the same name: HTTP %d, want 409", code)
		}
		if code, _ := a.do(t, k.create, "POST", "/images/known-sets", map[string]any{"name": "x", "kind": "faces"}, ""); code != http.StatusBadRequest {
			t.Errorf("a bad kind: HTTP %d, want 400", code)
		}
	})

	t.Run("add: hashes from stored evidence, again (nothing doubled), from a table with a bad row", func(t *testing.T) {
		code, out := a.do(t, k.addHashes, "POST", "/images/known-sets/"+setID+"/hashes?collection_id="+col, map[string]any{"evidence_ids": []string{P, id()}}, setID)
		if code != http.StatusOK || out["added"].(float64) != 1 || out["evidence_not_found"].(float64) != 1 {
			t.Fatalf("add from evidence: HTTP %d %v", code, out)
		}
		code, out = a.do(t, k.addHashes, "POST", "/images/known-sets/"+setID+"/hashes?collection_id="+col, map[string]any{"evidence_ids": []string{P}}, setID)
		if code != http.StatusOK || out["added"].(float64) != 0 || out["already_in_set"].(float64) != 1 {
			t.Fatalf("the same evidence twice: HTTP %d %v", code, out)
		}
		csv := "sha256,label\n" + strings.Repeat("f", 64) + ",external list\nnot-a-hash,bad\n"
		code, out = a.do(t, k.addHashes, "POST", "/images/known-sets/"+setID+"/hashes", map[string]any{"csv": csv}, setID)
		if code != http.StatusOK || out["added"].(float64) != 1 || out["rejected_count"].(float64) != 1 || out["hashes_in_set"].(float64) != 2 {
			t.Fatalf("add from a table: HTTP %d %v", code, out)
		}
		if rejected := out["rejected"].([]any); rejected[0].(map[string]any)["line"].(float64) != 3 {
			t.Errorf("the rejected row should name line 3: %v", rejected)
		}
	})

	t.Run("K1 and K2: the identical file and the re-saved copy are flagged, the crop, the others and other collections are not; K3 the coverage line", func(t *testing.T) {
		code, out := a.do(t, k.flags, "GET", "/images/known?collection_id="+col, nil, "")
		if code != http.StatusOK {
			t.Fatalf("HTTP %d %v", code, out)
		}
		flagged := evidenceSet(out, "flags")
		for _, want := range []string{P, P2, DUP} {
			if !flagged[want] {
				t.Errorf("%s should be flagged", want)
			}
		}
		for _, not := range []string{P3, U1, U2, NOHASH, OTHERCOL, BONLY} {
			if flagged[not] {
				t.Errorf("%s must not be flagged", not)
			}
		}
		if len(flagged) != 3 || out["flagged_pictures"].(float64) != 3 {
			t.Errorf("exactly three pictures should be flagged, got %v", flagged)
		}
		tiers := map[string]string{}
		for _, f := range out["flags"].([]any) {
			row := f.(map[string]any)
			tiers[row["evidence_id"].(string)] = row["tier"].(string)
		}
		if tiers[P] != "identical" || tiers[DUP] != "identical" || tiers[P2] != "same_image" {
			t.Errorf("tiers %v", tiers)
		}
		cover := out["coverage"].(map[string]any)
		want := "Checked 5 of 6 images against 2 hashes in 1 set. 1 image has no perceptual hash yet and was checked by file hash only."
		if cover["statement"] != want || cover["images_in_case"].(float64) != 6 || cover["images_with_hash"].(float64) != 5 {
			t.Errorf("coverage %v", cover)
		}
		if out["by_kind"].(map[string]any)["of_interest"].(float64) != 3 {
			t.Errorf("by_kind %v", out["by_kind"])
		}
	})

	t.Run("a set filter that names no active set leaves no flag", func(t *testing.T) {
		code, out := a.do(t, k.flags, "GET", "/images/known?collection_id="+col+"&sets="+id(), nil, "")
		if code != http.StatusOK || len(out["flags"].([]any)) != 0 {
			t.Errorf("HTTP %d flags %v", code, out["flags"])
		}
	})

	t.Run("K4: another tenant sees no set, no flag, and cannot retire this set", func(t *testing.T) {
		code, out := b.do(t, k.list, "GET", "/images/known-sets", nil, "")
		if code != http.StatusOK || len(out["sets"].([]any)) != 0 {
			t.Errorf("tenant B should see no set: HTTP %d %v", code, out["sets"])
		}
		code, out = b.do(t, k.flags, "GET", "/images/known?collection_id="+col, nil, "")
		if code != http.StatusOK || len(out["flags"].([]any)) != 0 || !strings.HasPrefix(out["coverage"].(map[string]any)["statement"].(string), "No known set is active") {
			t.Errorf("tenant B flags: HTTP %d %v", code, out)
		}
		if code, _ := b.do(t, k.retire, "POST", "/images/known-sets/"+setID+"/retire", nil, setID); code != http.StatusNotFound {
			t.Errorf("tenant B retiring tenant A's set: HTTP %d, want 404", code)
		}
		if code, _ := b.do(t, k.addHashes, "POST", "/images/known-sets/"+setID+"/hashes", map[string]any{"csv": strings.Repeat("e", 64)}, setID); code != http.StatusNotFound {
			t.Errorf("tenant B adding to tenant A's set: HTTP %d, want 404", code)
		}
	})

	t.Run("retire: the flags stop, the hashes stay, the set cannot be extended, the name is free again", func(t *testing.T) {
		if code, out := a.do(t, k.retire, "POST", "/images/known-sets/"+setID+"/retire", nil, setID); code != http.StatusOK {
			t.Fatalf("retire: HTTP %d %v", code, out)
		}
		if code, _ := a.do(t, k.retire, "POST", "/images/known-sets/"+setID+"/retire", nil, setID); code != http.StatusConflict {
			t.Errorf("retiring twice: HTTP %d, want 409", code)
		}
		code, out := a.do(t, k.flags, "GET", "/images/known?collection_id="+col, nil, "")
		if code != http.StatusOK || len(out["flags"].([]any)) != 0 || out["coverage"].(map[string]any)["statement"] != "No known set is active, so nothing was checked." {
			t.Errorf("after retirement: HTTP %d %v", code, out)
		}
		code, out = a.do(t, k.list, "GET", "/images/known-sets?include_retired=1", nil, "")
		sets := out["sets"].([]any)
		if code != http.StatusOK || len(sets) != 1 || sets[0].(map[string]any)["hashes"].(float64) != 2 || sets[0].(map[string]any)["retired_at"] == nil {
			t.Errorf("the retired set and its two hashes should still be listed: %v", sets)
		}
		if code, _ := a.do(t, k.addHashes, "POST", "/images/known-sets/"+setID+"/hashes", map[string]any{"csv": strings.Repeat("e", 64)}, setID); code != http.StatusConflict {
			t.Errorf("adding to a retired set: HTTP %d, want 409", code)
		}
		if code, _ := a.do(t, k.create, "POST", "/images/known-sets", map[string]any{"name": "watch", "kind": "known_harmless"}, ""); code != http.StatusCreated {
			t.Errorf("a retired set frees its name: HTTP %d, want 201", code)
		}
	})

	t.Run("K5 and K6: no picture bytes anywhere, one audit event per action with an actor", func(t *testing.T) {
		var bytea int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema = 'forensic' AND table_name IN ('image_known_set', 'image_known_hash', 'image_known_event') AND data_type = 'bytea'`).Scan(&bytea); err != nil || bytea != 0 {
			t.Errorf("bytea columns: %d (%v)", bytea, err)
		}
		rows, err := pool.Query(ctx, `SELECT kind, count(*)::int, count(*) FILTER (WHERE btrim(actor) <> '')::int FROM forensic.image_known_event WHERE tenant_id = $1 GROUP BY kind`, tenantA)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		got := map[string]int{}
		for rows.Next() {
			var kind string
			var n, withActor int
			if err := rows.Scan(&kind, &n, &withActor); err != nil {
				t.Fatal(err)
			}
			if n != withActor {
				t.Errorf("%s: %d events, %d with an actor", kind, n, withActor)
			}
			got[kind] = n
		}
		// two sets created, three adds (evidence, evidence again, table), one retirement, three checks (the full one, the filtered one, the one after retirement)
		want := map[string]int{"set_created": 2, "hashes_added": 3, "set_retired": 1, "check": 3}
		for kind, n := range want {
			if got[kind] != n {
				t.Errorf("audit events %v, want %v", got, want)
				break
			}
		}
	})
}

// K7: the check of a case against sets of 1,000 and 10,000 hashes (exact scan, no index). The budget is one second each.
func TestKnownSetsDBSpeed(t *testing.T) {
	pool := imageSearchTestPool(t)
	ctx := context.Background()
	tenant := "kn-p-" + randomHex(t, 3)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenant); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 97; i++ {
		raw := make([]byte, 32)
		_, _ = rand.Read(raw)
		bits := make([]byte, 256)
		for j := range bits {
			bits[j] = '0' + (raw[j/8]>>(7-uint(j%8)))&1
		}
		if err := seedImage(ctx, tx, seedImageSpec{tenant, "col-perf", "case-1", uuid.NewString(), randomHex(t, 32), "completed", string(bits), "", ""}); err != nil {
			_ = tx.Rollback(ctx)
			t.Skipf("seeding failed: %v", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, err := pool.Begin(ctx)
		if err != nil {
			return
		}
		defer func() { _ = c.Rollback(ctx) }()
		for _, table := range []string{"image_known_hash", "image_known_set", "image_hash"} {
			_, _ = c.Exec(ctx, "DELETE FROM forensic."+table+" WHERE tenant_id = $1", tenant)
		}
		_ = c.Commit(ctx)
	})
	measure := func(pool *pgxpool.Pool, hashes int) time.Duration {
		c, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = c.Rollback(ctx) }()
		if _, err := c.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenant); err != nil {
			t.Fatal(err)
		}
		var setID string
		if err := c.QueryRow(ctx, `INSERT INTO forensic.image_known_set (tenant_id, name, kind, created_by) VALUES ($1, $2, 'of_interest', 'perf') RETURNING set_id::text`, tenant, fmt.Sprintf("perf-%d", hashes)).Scan(&setID); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Exec(ctx, `
			INSERT INTO forensic.image_known_hash (tenant_id, set_id, pdq256, source, added_by)
			SELECT $1, $2::uuid, ('x' || md5(g::text || $3) || md5($3 || g::text))::bit(256), 'api', 'perf' FROM generate_series(1, $4::int) g`, tenant, setID, randomHex(t, 4), hashes); err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		var n int
		if err := c.QueryRow(ctx, `SELECT count(*) FROM forensic.image_known_matches('col-perf', 56, NULL, NULL)`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		elapsed := time.Since(started)
		t.Logf("97 pictures against %d hashes: %v (%d flags)", hashes, elapsed, n)
		return elapsed
	}
	for _, hashes := range []int{1000, 10000} {
		if elapsed := measure(pool, hashes); elapsed > time.Second {
			t.Errorf("K7: %d hashes took %v, budget 1 s", hashes, elapsed)
		}
	}
}
