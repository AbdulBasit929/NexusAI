package main

// Database acceptance tests of the case-wide image search (gates X1, C1, A1 and L1 of docs/data/image-search-m1-preregistration-20261120.md).
// They need a throwaway PostgreSQL with migrations 001 to 015 and 038 applied (db/forensic_records/ops/038_image_search/Rehearse-038.ps1 -KeepContainer -PublishPort 55438):
//
//	IMAGE_SEARCH_TEST_DB=postgres://localrecall:<password>@127.0.0.1:55438/localrecall go test ./api/forensic_records -run TestImageCaseSearchDB -count=1 -v
//	IMAGE_SEARCH_PERF=1 (same variable) -run TestImageCaseSearchLatency30K -timeout 40m
//
// Without IMAGE_SEARCH_TEST_DB they skip. They write only into tenants named it-* and perf-*, never into a case database.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func imageSearchTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("IMAGE_SEARCH_TEST_DB")
	if url == "" {
		t.Skip("IMAGE_SEARCH_TEST_DB is not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

type stubEmbed struct {
	text   []float64
	probe  imageProbeVectors
	called int
}

func (s *stubEmbed) EmbedText(_ context.Context, _ string) ([]float64, error) {
	s.called++
	return s.text, nil
}
func (s *stubEmbed) EmbedImage(_ context.Context, _ []byte) (imageProbeVectors, error) {
	s.called++
	return s.probe, nil
}

type stubTranslator struct{ out string }

func (s stubTranslator) ToEnglish(_ context.Context, _ string) (string, error) { return s.out, nil }

type seedImageSpec struct {
	Tenant, Collection, Case, ID, SHA, Status, Hash, Dino, Siglip string
}

func seedImage(ctx context.Context, tx pgx.Tx, s seedImageSpec) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO forensic.evidence_items (evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension, size_bytes, sha256,
		  modality, detected_type, classifier_confidence, processing_route, processing_status, raw_storage_ref, metadata)
		VALUES ($1::uuid, $2, $3, $4, 'it-user', $5, 'photo.jpg', 'image/jpeg', 'jpg', 1000, $6, 'image', 'image', 1, 'unified_media_worker', $7, 'immutable://it/' || $6, '{"request_id":"it"}')`,
		s.ID, s.Tenant, s.Collection, s.Case, "photo-"+s.ID[:8]+".jpg", s.SHA, s.Status); err != nil {
		return fmt.Errorf("evidence %s: %w", s.ID, err)
	}
	if s.Hash == "" && s.Dino == "" && s.Siglip == "" {
		return nil
	}
	var version string
	if err := tx.QueryRow(ctx, `SELECT current_version_id::text FROM forensic.evidence_items WHERE evidence_id = $1::uuid`, s.ID).Scan(&version); err != nil {
		return err
	}
	var run, artifact string
	if err := tx.QueryRow(ctx, `
		INSERT INTO forensic.processing_runs (tenant_id, collection_id, evidence_id, version_id, run_kind, pipeline_id, idempotency_key, status, input_sha256)
		VALUES ($1, $2, $3::uuid, $4::uuid, 'ingest', 'it', 'it:' || gen_random_uuid()::text, 'succeeded', $5) RETURNING run_id::text`,
		s.Tenant, s.Collection, s.ID, version, s.SHA).Scan(&run); err != nil {
		return fmt.Errorf("run %s: %w", s.ID, err)
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO forensic.derived_artifacts (tenant_id, collection_id, evidence_id, version_id, run_id, artifact_type, media_type, processing_status)
		VALUES ($1, $2, $3::uuid, $4::uuid, $5::uuid, 'forensics.image-hash-observation/v1', 'application/json', 'completed') RETURNING artifact_id::text`,
		s.Tenant, s.Collection, s.ID, version, run).Scan(&artifact); err != nil {
		return fmt.Errorf("artifact %s: %w", s.ID, err)
	}
	if s.Hash != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO forensic.image_hash (tenant_id, collection_id, evidence_id, version_id, derived_artifact_id, chunk_ref, algorithm, pdq256)
			VALUES ($1, $2, $3::uuid, $4::uuid, $5::uuid, 'v:' || $4 || ':full', 'pdq-style-256-nexusai-v1', $6::bit(256))`,
			s.Tenant, s.Collection, s.ID, version, artifact, s.Hash); err != nil {
			return fmt.Errorf("hash %s: %w", s.ID, err)
		}
	}
	for model, vector := range map[string]string{imageSearchDinoModelID: s.Dino, imageSearchSiglipModelID: s.Siglip} {
		if vector == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO forensic.content_embedding (tenant_id, collection_id, evidence_id, derived_artifact_id, modality, chunk_ref, model_id, dim, embedding)
			VALUES ($1, $2, $3::uuid, $5::uuid, 'image', 'v:' || $4 || ':full', $6, vector_dims($7::vector), $7::vector)`,
			s.Tenant, s.Collection, s.ID, version, artifact, model, vector); err != nil {
			return fmt.Errorf("vector %s: %w", s.ID, err)
		}
	}
	return nil
}

func bitsOf(ones int) string { return strings.Repeat("0", 256-ones) + strings.Repeat("1", ones) }

func TestImageCaseSearchDB(t *testing.T) {
	pool := imageSearchTestPool(t)
	ctx := context.Background()
	const (
		idP, idQ, idR, idS, idDup, idFail = "39000000-0000-4000-8000-000000000001", "39000000-0000-4000-8000-000000000002", "39000000-0000-4000-8000-000000000003",
			"39000000-0000-4000-8000-000000000004", "39000000-0000-4000-8000-000000000005", "39000000-0000-4000-8000-000000000006"
		idOtherCollection = "39000000-0000-4000-8000-0000000000a1"
		idOtherTenant     = "39000000-0000-4000-8000-0000000000b1"
		upload            = "uploaded-probe-bytes"
	)
	sum := sha256.Sum256([]byte(upload))
	uploadSHA := hex.EncodeToString(sum[:])
	shaP := strings.Repeat("a", 64)
	specs := []seedImageSpec{
		{"it-a", "col-1", "case-1", idP, shaP, "completed", bitsOf(0), "[1,0,0]", "[0,1,0]"},
		{"it-a", "col-1", "case-1", idQ, strings.Repeat("b", 64), "completed", bitsOf(3), "[0.999,0.04,0]", "[0,0.99,0.1]"},
		{"it-a", "col-1", "case-1", idR, strings.Repeat("c", 64), "completed", bitsOf(128), "[0.97,0.2,0]", ""},
		{"it-a", "col-1", "case-1", idS, uploadSHA, "completed", bitsOf(120), "[0,0,1]", ""},
		{"it-a", "col-1", "case-1", idDup, shaP, "duplicate", "", "", ""},
		{"it-a", "col-1", "case-1", idFail, strings.Repeat("d", 64), "failed", "", "", ""},
		{"it-a", "col-2", "case-9", idOtherCollection, shaP, "completed", bitsOf(0), "[1,0,0]", "[0,1,0]"},
		{"it-b", "col-1", "case-1", idOtherTenant, shaP, "completed", bitsOf(0), "[1,0,0]", "[0,1,0]"},
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// clean any earlier run of this test, then seed
	for _, tenant := range []string{"it-a", "it-b"} {
		for _, table := range []string{"image_search_log", "image_hash", "content_embedding"} {
			if _, err := tx.Exec(ctx, "DELETE FROM forensic."+table+" WHERE tenant_id = $1", tenant); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, s := range specs {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", s.Tenant); err != nil {
			t.Fatal(err)
		}
		_, _ = tx.Exec(ctx, "DELETE FROM forensic.derived_artifacts WHERE evidence_id = $1::uuid", s.ID)
		if err := seedImage(ctx, tx, s); err != nil {
			_ = tx.Rollback(ctx)
			t.Skipf("seeding failed (is this a database with migrations 001-015 and 038, and no earlier it-* rows?): %v", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// leave the database as found: remove what this test wrote
		c, err := pool.Begin(ctx)
		if err != nil {
			return
		}
		defer func() { _ = c.Rollback(ctx) }()
		for _, tenant := range []string{"it-a", "it-b"} {
			for _, table := range []string{"image_search_log", "image_hash", "content_embedding"} {
				_, _ = c.Exec(ctx, "DELETE FROM forensic."+table+" WHERE tenant_id = $1", tenant)
			}
		}
		_ = c.Commit(ctx)
	})

	settings := imageSearchSettings{HashMaxDistance: 10, HashBand: 6, DinoMaxDistance: 0.1, DinoBand: 0.03, Timeout: time.Minute}
	embed := &stubEmbed{text: []float64{0, 1, 0}, probe: imageProbeVectors{PDQHex: strings.Repeat("00", 32), Vectors: map[string][]float64{"dinov2": {1, 0, 0}}}}
	deps := imageSearchDeps{db: pool, settings: settings, embed: embed, translator: stubTranslator{out: "a red car"}}
	scope := forensicScope{TenantID: "it-a", ActorID: "it-actor", CollectionID: "col-1", CaseID: "case-1"}
	ids := func(result map[string]any) []string {
		var out []string
		for _, h := range result["results"].([]imageSearchHit) {
			out = append(out, h.EvidenceID)
		}
		return out
	}

	t.Run("copies of a stored image: tiers, coverage line, no leakage, no vector in the answer, one log row", func(t *testing.T) {
		result, err := runImageCaseSearch(ctx, deps, imageSearchRequest{Kind: "copies", Scope: scope, EvidenceID: idP, TopK: 20})
		if err != nil {
			t.Fatal(err)
		}
		hits := result["results"].([]imageSearchHit)
		got := map[string]string{}
		for _, h := range hits {
			got[h.EvidenceID] = h.Tier
		}
		want := map[string]string{idDup: "identical_file", idQ: "same_image", idR: "same_photograph"}
		if len(got) != len(want) {
			t.Fatalf("results %v, want %v", got, want)
		}
		for id, tier := range want {
			if got[id] != tier {
				t.Errorf("%s: tier %q, want %q", id, got[id], tier)
			}
		}
		for _, forbidden := range []string{idOtherCollection, idOtherTenant, idP, idS, idFail} {
			if _, present := got[forbidden]; present {
				t.Errorf("%s must not be in the results", forbidden)
			}
		}
		coverage := result["coverage"].(imageCoverage)
		if coverage.ImagesInCase != 5 || coverage.ImagesIndexed != 4 || coverage.ImagesFailed != 1 || coverage.ImagesIdenticalReupload != 1 {
			t.Errorf("coverage %+v, want 5/4/1/1", coverage)
		}
		if !strings.Contains(coverage.Statement, "Searched 4 of 5 images") {
			t.Errorf("coverage statement %q", coverage.Statement)
		}
		encoded, _ := json.Marshal(result)
		for _, banned := range []string{`"embedding"`, `"vector"`, "0.999", "pdq256"} {
			if strings.Contains(string(encoded), banned) {
				t.Errorf("the answer carries %s", banned)
			}
		}
		var count int
		var actor, kind string
		if err := pool.QueryRow(ctx, `SELECT count(*), max(searched_by), max(kind) FROM forensic.image_search_log WHERE tenant_id = 'it-a' AND search_id = $1::uuid`, result["search_id"]).Scan(&count, &actor, &kind); err != nil {
			t.Fatal(err)
		}
		if count != 1 || actor != "it-actor" || kind != "copies" {
			t.Errorf("log row: count %d actor %q kind %q", count, actor, kind)
		}
	})

	t.Run("a probe that belongs to another collection, another tenant or another case is not found", func(t *testing.T) {
		for name, id := range map[string]string{"other collection": idOtherCollection, "other tenant": idOtherTenant} {
			_, err := runImageCaseSearch(ctx, deps, imageSearchRequest{Kind: "copies", Scope: scope, EvidenceID: id, TopK: 20})
			var searchErr imageSearchError
			if !errors.As(err, &searchErr) || searchErr.Status != http.StatusNotFound {
				t.Errorf("%s: want 404, got %v", name, err)
			}
		}
		wrongCase := scope
		wrongCase.CaseID = "case-x"
		result, err := runImageCaseSearch(ctx, deps, imageSearchRequest{Kind: "by_words", Scope: wrongCase, Text: "anything", TopK: 20})
		if err != nil {
			t.Fatal(err)
		}
		if n := len(ids(result)); n != 0 || result["coverage"].(imageCoverage).ImagesInCase != 0 {
			t.Errorf("a case with no images returned %d results and coverage %+v", n, result["coverage"])
		}
	})

	t.Run("the other tenant and the other collection see only their own", func(t *testing.T) {
		for _, c := range []struct {
			scope forensicScope
			want  string
		}{
			{forensicScope{TenantID: "it-b", ActorID: "b", CollectionID: "col-1", CaseID: "case-1"}, idOtherTenant},
			{forensicScope{TenantID: "it-a", ActorID: "a", CollectionID: "col-2", CaseID: "case-9"}, idOtherCollection},
		} {
			result, err := runImageCaseSearch(ctx, deps, imageSearchRequest{Kind: "by_words", Scope: c.scope, Text: "anything", TopK: 20})
			if err != nil {
				t.Fatal(err)
			}
			got := ids(result)
			if len(got) != 1 || got[0] != c.want {
				t.Errorf("%s/%s saw %v, want only %s", c.scope.TenantID, c.scope.CollectionID, got, c.want)
			}
		}
	})

	t.Run("photos by words: ranked, top_k respected, Urdu script is translated first", func(t *testing.T) {
		result, err := runImageCaseSearch(ctx, deps, imageSearchRequest{Kind: "by_words", Scope: scope, Text: "a red car", TopK: 20})
		if err != nil {
			t.Fatal(err)
		}
		got := ids(result)
		if len(got) != 2 || got[0] != idP || got[1] != idQ {
			t.Errorf("ranked %v, want [P Q]", got)
		}
		hits := result["results"].([]imageSearchHit)
		if hits[0].Similarity == nil || *hits[0].Similarity < 0.999 || hits[0].Rank != 1 {
			t.Errorf("first hit %+v", hits[0])
		}
		one, err := runImageCaseSearch(ctx, deps, imageSearchRequest{Kind: "by_words", Scope: scope, Text: "a red car", TopK: 1})
		if err != nil || len(ids(one)) != 1 {
			t.Errorf("top_k 1 returned %v (%v)", ids(one), err)
		}
		urdu, err := runImageCaseSearch(ctx, deps, imageSearchRequest{Kind: "by_words", Scope: scope, Text: "سرخ گاڑی", TopK: 20, QueryLang: "ur"})
		if err != nil {
			t.Fatal(err)
		}
		if urdu["translated_query"] != "a red car" {
			t.Errorf("translated_query %v", urdu["translated_query"])
		}
	})

	t.Run("an uploaded probe is searched by hash, vector and file hash", func(t *testing.T) {
		result, err := runImageCaseSearch(ctx, deps, imageSearchRequest{Kind: "copies_of_upload", Scope: scope, Upload: []byte(upload), TopK: 20})
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, h := range result["results"].([]imageSearchHit) {
			got[h.EvidenceID] = h.Tier
		}
		// P: hash distance 0 (same image); Q: 3 bits; R: close vector; S: its file hash equals the uploaded bytes (identical file)
		want := map[string]string{idP: "same_image", idQ: "same_image", idR: "same_photograph", idS: "identical_file"}
		for id, tier := range want {
			if got[id] != tier {
				t.Errorf("%s: %q, want %q (all: %v)", id, got[id], tier, got)
			}
		}
		if probe := result["probe"].(map[string]any); probe["uploaded_file_sha256"] != uploadSHA {
			t.Errorf("probe %v", probe)
		}
	})

	t.Run("a stored image that was not processed gives a clear conflict, and more-like-this ranks neighbours", func(t *testing.T) {
		_, err := runImageCaseSearch(ctx, deps, imageSearchRequest{Kind: "copies", Scope: scope, EvidenceID: idFail, TopK: 20})
		var searchErr imageSearchError
		if !errors.As(err, &searchErr) || searchErr.Status != http.StatusConflict {
			t.Errorf("want 409, got %v", err)
		}
		result, err := runImageCaseSearch(ctx, deps, imageSearchRequest{Kind: "more_like_this", Scope: scope, EvidenceID: idP, TopK: 5})
		if err != nil {
			t.Fatal(err)
		}
		if got := ids(result); len(got) != 1 || got[0] != idQ {
			t.Errorf("more like P: %v, want [Q] (P itself excluded; R and S have no similarity vector)", got)
		}
	})

	t.Run("every search above wrote exactly one log row", func(t *testing.T) {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM forensic.image_search_log WHERE tenant_id IN ('it-a', 'it-b')`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n < 9 {
			t.Errorf("only %d log rows", n)
		}
	})
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------------------- latency at 30 000 images

func TestImageCaseSearchLatency30K(t *testing.T) {
	if os.Getenv("IMAGE_SEARCH_PERF") == "" {
		t.Skip("IMAGE_SEARCH_PERF is not set")
	}
	pool := imageSearchTestPool(t)
	ctx := context.Background()
	rows := 30000
	var have int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM forensic.evidence_items WHERE tenant_id = 'perf' AND collection_id = 'perf-c1'`).Scan(&have)
	if have < rows {
		started := time.Now()
		seedPerf(ctx, t, pool, rows)
		t.Logf("seeded %d images (hash and two 768-number vectors each) in %.0f s", rows, time.Since(started).Seconds())
	}
	var sample []string
	r, err := pool.Query(ctx, `SELECT evidence_id::text FROM forensic.evidence_items WHERE tenant_id = 'perf' AND collection_id = 'perf-c1' ORDER BY md5(evidence_id::text) LIMIT 60`)
	if err != nil {
		t.Fatal(err)
	}
	for r.Next() {
		var id string
		_ = r.Scan(&id)
		sample = append(sample, id)
	}
	r.Close()
	text := make([]float64, 768)
	for i := range text {
		text[i] = float64((i*37)%101)/101 - 0.5
	}
	settings := imageSearchSettings{HashMaxDistance: 91, HashBand: 6, DinoMaxDistance: 0.205, DinoBand: 0.03, Timeout: time.Minute}
	embed := &stubEmbed{text: text, probe: imageProbeVectors{PDQHex: strings.Repeat("a5", 32), Vectors: map[string][]float64{"dinov2": text, "siglip2": text}}}
	handler := imageCaseSearchHandlerWith(imageSearchDeps{db: pool, settings: settings, embed: embed, translator: stubTranslator{out: "x"}})
	measure := func(name, target string) {
		var times []float64
		for i := 0; i < 50; i++ {
			url := strings.ReplaceAll(target, "{id}", sample[i%len(sample)])
			start := time.Now()
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, url, nil))
			elapsed := time.Since(start).Seconds()
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s: HTTP %d %s", name, recorder.Code, recorder.Body.String())
			}
			times = append(times, elapsed)
		}
		sort.Float64s(times)
		p95 := times[int(0.95*float64(len(times)))-1]
		t.Logf("LATENCY %-34s n=50  median %.3f s  p95 %.3f s  max %.3f s", name, times[len(times)/2], p95, times[len(times)-1])
		if p95 > 1.0 {
			t.Errorf("%s: p95 %.3f s is above the 1.0 s gate", name, p95)
		}
	}
	base := "/images/search?tenant_id=perf&collection_id=perf-c1&case_id=k1"
	measure("photos by words", base+"&kind=by_words&q=a+dog+on+a+beach&top_k=20")
	measure("same photograph/same image (copies)", base+"&kind=copies&evidence_id={id}")
	measure("more like this", base+"&kind=more_like_this&evidence_id={id}&top_k=20")
}

func seedPerf(ctx context.Context, t *testing.T, pool *pgxpool.Pool, n int) {
	t.Helper()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	run := func(sql string, args ...any) {
		if _, err := conn.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
	}
	run("SELECT set_config('app.tenant_id', 'perf', false)")
	run("SELECT setseed(0.42)")
	run(`INSERT INTO forensic.evidence_items (evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension, size_bytes, sha256,
	       modality, detected_type, classifier_confidence, processing_route, processing_status, raw_storage_ref, metadata)
	     SELECT md5('perf-' || i)::uuid, 'perf', 'perf-c1', 'k1', 'perf', 'img-' || i || '.jpg', 'img.jpg', 'image/jpeg', 'jpg', 1000, md5('perf-' || i) || md5('perf-x-' || i),
	            'image', 'image', 1, 'unified_media_worker', 'completed', 'immutable://perf/' || i, '{"request_id":"perf"}'
	     FROM generate_series(1, $1) i ON CONFLICT DO NOTHING`, n)
	run(`INSERT INTO forensic.processing_runs (run_id, tenant_id, collection_id, evidence_id, version_id, run_kind, pipeline_id, idempotency_key, status, input_sha256)
	     SELECT 'aaaaaaaa-0000-4000-8000-000000000001'::uuid, e.tenant_id, e.collection_id, e.evidence_id, e.current_version_id, 'ingest', 'perf', 'perf-shared-run', 'succeeded', e.sha256
	     FROM forensic.evidence_items e WHERE e.tenant_id = 'perf' AND e.collection_id = 'perf-c1' ORDER BY e.evidence_id LIMIT 1 ON CONFLICT DO NOTHING`)
	run(`INSERT INTO forensic.derived_artifacts (artifact_id, tenant_id, collection_id, evidence_id, version_id, run_id, artifact_type, media_type, processing_status)
	     SELECT md5(e.evidence_id::text || 'a')::uuid, e.tenant_id, e.collection_id, e.evidence_id, e.current_version_id, 'aaaaaaaa-0000-4000-8000-000000000001'::uuid,
	            'forensics.image-hash-observation/v1', 'application/json', 'completed'
	     FROM forensic.evidence_items e WHERE e.tenant_id = 'perf' AND e.collection_id = 'perf-c1' ON CONFLICT DO NOTHING`)
	run(`INSERT INTO forensic.image_hash (tenant_id, collection_id, evidence_id, version_id, derived_artifact_id, chunk_ref, algorithm, pdq256)
	     SELECT e.tenant_id, e.collection_id, e.evidence_id, e.current_version_id, md5(e.evidence_id::text || 'a')::uuid, 'v:' || e.current_version_id::text || ':full', 'pdq-style-256-nexusai-v1',
	            ('x' || md5(e.sha256) || md5(e.sha256 || 'h'))::bit(256)
	     FROM forensic.evidence_items e WHERE e.tenant_id = 'perf' AND e.collection_id = 'perf-c1' ON CONFLICT DO NOTHING`)
	for _, model := range []string{imageSearchDinoModelID, imageSearchSiglipModelID} {
		run(`INSERT INTO forensic.content_embedding (tenant_id, collection_id, evidence_id, derived_artifact_id, modality, chunk_ref, model_id, dim, embedding)
		     SELECT e.tenant_id, e.collection_id, e.evidence_id, md5(e.evidence_id::text || 'a')::uuid, 'image', 'v:' || e.current_version_id::text || ':full', $1, 768, v.a::vector
		     FROM forensic.evidence_items e
		     CROSS JOIN LATERAL (SELECT array_agg((random() - 0.5)::real) a FROM generate_series(1, 768) g(i) WHERE e.evidence_id IS NOT NULL) v
		     WHERE e.tenant_id = 'perf' AND e.collection_id = 'perf-c1' ON CONFLICT DO NOTHING`, model)
	}
	run("ANALYZE forensic.content_embedding")
	run("ANALYZE forensic.image_hash")
}
