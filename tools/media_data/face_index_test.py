#!/usr/bin/env python3
"""pgvector HNSW with per-case filtering (gates I1 and I2 of docs/data/face-image-search-preregistration-20261119.md).

Runs in the evaluation image on an INTERNAL docker network next to a throw-away Postgres container started from the product's database image; nothing touches the live database.

    docker network create --internal nx-face-index-net
    docker run -d --name nx-face-index --network nx-face-index-net --memory 3500m --memory-swap 3500m -e POSTGRES_PASSWORD=x timescale/timescaledb:latest-pg16 -c shared_buffers=768MB -c maintenance_work_mem=1GB -c max_parallel_maintenance_workers=2
    docker run --rm --network nx-face-index-net --memory 2500m -v C:/tmp/nexusai-media-data/work-faces:/work -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-face:20261119 \
        /repo/tools/media_data/face_index_test.py --host nx-face-index --rows 10000 100000

Vectors: the real AuraFace embeddings of FairFace (10,954 faces, 512-d) are the "parents". The 10 K tier uses the real vectors only. Larger tiers are SYNTHETIC EXPANSIONS: each new vector is a real parent plus Gaussian
noise of relative size 0.7, re-normalised (cosine to its parent about 0.82), so clusters of near-identical "same person" vectors exist as in real galleries; this tests the index, not real-world embedding geometry.
Cases: each row gets a case id in 0..99 (about 1% of rows each). Selectivity 1% = one case, 10% = ten cases, 100% = no filter. Queries are noisy variants of random parents (relative noise 0.7, another seed): "a probe of
somebody who is in the gallery". Ground truth = exact search (index scans off). Recall@10 = share of the exact top ten returned (a short answer counts the missing rows as misses).
"""
import argparse
import json
import sys
import time
from pathlib import Path

import numpy as np
import psycopg

sys.path.insert(0, "/repo/tools/media_data")
import face_common as fc  # noqa: E402


def noisy(parents, rng, scale=0.7):
    g = rng.standard_normal(parents.shape).astype(np.float32)
    g /= np.linalg.norm(g, axis=1, keepdims=True)
    return fc.l2n(parents + scale * g)


def vec(v):
    return "[" + ",".join(f"{x:.6f}" for x in v) + "]"


def timed(cur, sql, params=None):
    t0 = time.perf_counter()
    cur.execute(sql, params)
    rows = cur.fetchall()
    return rows, time.perf_counter() - t0


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="nx-face-index")
    parser.add_argument("--rows", type=int, nargs="+", default=[10000, 100000])
    parser.add_argument("--queries", type=int, default=100)
    parser.add_argument("--partitioning-at", type=int, default=100000)
    args = parser.parse_args()
    real = np.load(Path(fc.WORK) / "fairface" / "emb-auraface.npy")
    real = fc.l2n(real[~np.isnan(real[:, 0])])
    rng = np.random.default_rng(20261119)
    out = {"parents": int(len(real)), "tiers": {}}
    conn = psycopg.connect(host=args.host, user="postgres", password="x", dbname="postgres", autocommit=True)
    cur = conn.cursor()
    cur.execute("CREATE EXTENSION IF NOT EXISTS vector")
    cur.execute("SELECT extversion FROM pg_extension WHERE extname='vector'")
    out["pgvector"] = cur.fetchone()[0]
    cur.execute("SELECT version()")
    out["postgres"] = cur.fetchone()[0][:40]
    print("pgvector", out["pgvector"], out["postgres"], flush=True)
    for n in args.rows:
        tier = {"rows": n, "synthetic": n > len(real)}
        cur.execute("DROP TABLE IF EXISTS faces CASCADE")
        cur.execute("CREATE TABLE faces (id bigint, case_id int, embedding vector(512))")
        if n <= len(real):
            data = real[rng.permutation(len(real))[:n]]
        else:
            parents = real[rng.integers(0, len(real), n)]
            data = noisy(parents, rng)
        cases = rng.integers(0, 100, n)
        t0 = time.perf_counter()
        with cur.copy("COPY faces (id, case_id, embedding) FROM STDIN") as copy:
            for i in range(n):
                copy.write_row((i, int(cases[i]), vec(data[i])))
        tier["load_seconds"] = round(time.perf_counter() - t0, 1)
        cur.execute("CREATE INDEX faces_case ON faces (case_id)")
        cur.execute("SET maintenance_work_mem = '768MB'")
        t0 = time.perf_counter()
        cur.execute("CREATE INDEX faces_hnsw ON faces USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64)")
        tier["hnsw_build_seconds"] = round(time.perf_counter() - t0, 1)
        cur.execute("SELECT pg_relation_size('faces_hnsw'), pg_relation_size('faces')")
        sizes = cur.fetchone()
        tier["hnsw_index_mb"], tier["table_mb"] = round(sizes[0] / 1e6), round(sizes[1] / 1e6)
        cur.execute("ANALYZE faces")
        print(f"tier {n}: loaded {tier['load_seconds']} s, hnsw {tier['hnsw_build_seconds']} s, index {tier['hnsw_index_mb']} MB, table {tier['table_mb']} MB", flush=True)
        queries = noisy(data[rng.integers(0, n, args.queries)], np.random.default_rng(777))
        results = []
        for label, where in (("1% (one case)", "case_id = 0"), ("10% (ten cases)", "case_id < 10"), ("100% (no filter)", "true")):
            truth = []
            exact_times = []
            for q in queries:
                cur.execute("SET enable_indexscan = off; SET enable_bitmapscan = off")
                rows, dt = timed(cur, f"SELECT id FROM faces WHERE {where} ORDER BY embedding <=> %s::vector LIMIT 10", (vec(q),))
                truth.append({r[0] for r in rows})
                exact_times.append(dt)
            cur.execute("RESET enable_indexscan; RESET enable_bitmapscan")
            results.append({"filter": label, "arm": "exact scan (index off)", "recall10": 1.0, "p50_ms": round(1000 * float(np.median(exact_times)), 1), "p95_ms": round(1000 * float(np.percentile(exact_times, 95)), 1)})
            for ef in (40, 200):
                for mode in ("off", "relaxed_order", "strict_order"):
                    cur.execute(f"SET hnsw.ef_search = {ef}")
                    cur.execute(f"SET hnsw.iterative_scan = {mode}")
                    cur.execute("SET hnsw.max_scan_tuples = 20000")
                    hits, returned, times = 0, 0, []
                    for q, t in zip(queries, truth):
                        rows, dt = timed(cur, f"SELECT id FROM faces WHERE {where} ORDER BY embedding <=> %s::vector LIMIT 10", (vec(q),))
                        got = {r[0] for r in rows}
                        hits += len(got & t)
                        returned += len(got)
                        times.append(dt)
                    results.append({"filter": label, "arm": f"hnsw ef_search={ef} iterative_scan={mode}", "recall10": round(hits / (10 * len(queries)), 4), "rows_returned_per_query": round(returned / len(queries), 2),
                                    "p50_ms": round(1000 * float(np.median(times)), 1), "p95_ms": round(1000 * float(np.percentile(times, 95)), 1)})
                    print(f"  {label:18s} {results[-1]['arm']:46s} recall {results[-1]['recall10']:.3f} returned {results[-1]['rows_returned_per_query']:5.2f} p50 {results[-1]['p50_ms']} ms p95 {results[-1]['p95_ms']} ms", flush=True)
        cur.execute("RESET hnsw.ef_search; RESET hnsw.iterative_scan")
        tier["queries"] = results
        # the way to return ALL matches above a threshold: an exact scan inside one case (case index + distance filter)
        cur.execute("SET enable_indexscan = off; SET enable_bitmapscan = off")
        times, counts = [], []
        for q in queries[:30]:
            rows, dt = timed(cur, "SELECT count(*) FROM faces WHERE case_id = 0 AND embedding <=> %s::vector <= 0.4", (vec(q),))
            times.append(dt)
            counts.append(rows[0][0])
        tier["exact_all_above_threshold_in_one_case"] = {"cosine_distance_max": 0.4, "p50_ms": round(1000 * float(np.median(times)), 1), "p95_ms": round(1000 * float(np.percentile(times, 95)), 1), "median_matches": float(np.median(counts)), "rows_in_case": int((cases == 0).sum())}
        cur.execute("RESET enable_indexscan; RESET enable_bitmapscan")
        print("  exact scan inside one case:", tier["exact_all_above_threshold_in_one_case"], flush=True)
        if n == args.partitioning_at:
            cur.execute("DROP TABLE IF EXISTS faces_p CASCADE")
            cur.execute("CREATE TABLE faces_p (id bigint, case_id int, embedding vector(512)) PARTITION BY LIST (case_id)")
            for c in range(100):
                cur.execute(f"CREATE TABLE faces_p_{c} PARTITION OF faces_p FOR VALUES IN ({c})")
            cur.execute("INSERT INTO faces_p SELECT id, case_id, embedding FROM faces")
            t0 = time.perf_counter()
            cur.execute("SELECT format('CREATE INDEX ON %I USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64)', c.relname) FROM pg_class c JOIN pg_inherits i ON i.inhrelid = c.oid JOIN pg_class p ON p.oid = i.inhparent WHERE p.relname = 'faces_p'")
            for (stmt,) in cur.fetchall():
                conn.execute(stmt)
            tier["partitioned_index_build_seconds"] = round(time.perf_counter() - t0, 1)
            cur.execute("ANALYZE faces_p")
            part = []
            for label, where in (("1% (one case)", "case_id = 0"), ("10% (ten cases)", "case_id < 10")):
                truth = []
                for q in queries:
                    cur.execute("SET enable_indexscan = off; SET enable_bitmapscan = off")
                    rows, _ = timed(cur, f"SELECT id FROM faces_p WHERE {where} ORDER BY embedding <=> %s::vector LIMIT 10", (vec(q),))
                    truth.append({r[0] for r in rows})
                cur.execute("RESET enable_indexscan; RESET enable_bitmapscan")
                cur.execute("SET hnsw.ef_search = 40")
                hits, times = 0, []
                for q, t in zip(queries, truth):
                    rows, dt = timed(cur, f"SELECT id FROM faces_p WHERE {where} ORDER BY embedding <=> %s::vector LIMIT 10", (vec(q),))
                    hits += len({r[0] for r in rows} & t)
                    times.append(dt)
                part.append({"filter": label, "arm": "partitioned by case, hnsw per partition, ef_search=40", "recall10": round(hits / (10 * len(queries)), 4), "p50_ms": round(1000 * float(np.median(times)), 1), "p95_ms": round(1000 * float(np.percentile(times, 95)), 1)})
                print("  partitioned:", part[-1], flush=True)
            tier["partitioned"] = part
        out["tiers"][str(n)] = tier
        json.dump(out, open(Path(fc.WORK) / "results" / "index-test.json", "w"), indent=1)
    print("written /work/results/index-test.json")


if __name__ == "__main__":
    Path(fc.WORK, "results").mkdir(parents=True, exist_ok=True)
    main()
