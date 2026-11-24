#!/usr/bin/env python3
"""Index the images of a case for image search: reprocess each image evidence through the governed reprocess route (the worker then computes and stores its descriptors), one at a time.

    python tools/media_data/index_case_images.py --collection orion-lahore-reference-20261030 [--limit 5] [--dry]

Only images that have no hash row for their current version are sent. The reprocess route is the product's own (POST /evidence/{id}/reprocess: authenticated, audited, idempotent per key); this script never writes to
the database. It reads the API key and the tenant from the stack's secrets file by name and never prints them. Reprocessing runs the image's whole media pipeline again (OCR and plates as configured), so the
worker is busy for a while per image; the script waits for each job to finish before the next.
"""
import argparse
import json
import subprocess
import sys
import time
import urllib.error
import urllib.request

SECRETS = r"C:\Users\sheik\Workspace\Office\Projects\NexusAI\.env.forensic-runtime.local"
API = "http://127.0.0.1:8091"
PG = ["docker", "exec", "-i", "nexusai-forensic-postgres-1", "psql", "-U", "localrecall", "-d", "localrecall", "-At", "-F", "|"]


def secret(name):
    for line in open(SECRETS, encoding="utf-8"):
        if line.startswith(name + "="):
            return line.split("=", 1)[1].strip()
    sys.exit(f"{name} is not in the secrets file")


def sql(query):
    done = subprocess.run(PG + ["-c", query], capture_output=True, text=True)
    if done.returncode != 0:
        sys.exit(done.stderr.strip()[:300])
    return [line.split("|") for line in done.stdout.strip().splitlines() if line.strip()]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--collection", required=True)
    parser.add_argument("--limit", type=int, default=0)
    parser.add_argument("--dry", action="store_true")
    parser.add_argument("--timeout", type=int, default=600, help="seconds to wait for one image")
    parser.add_argument("--key-suffix", default="", help="added to the idempotency key; the route replays an earlier job for the same key, so a second pass over pictures that were skipped needs a new suffix")
    parser.add_argument("--tiles", action="store_true", help="select the images that have descriptors but no window (tile) vectors yet, and wait for those")
    args = parser.parse_args()
    key, tenant = secret("FORENSIC_RECORDS_API_KEY"), secret("FORENSIC_RECORDS_TENANT_ID") or "default"
    c = args.collection.replace("'", "")
    tile_exists = "exists (select 1 from forensic.content_embedding t where t.tenant_id = e.tenant_id and t.collection_id = e.collection_id and t.evidence_id = e.evidence_id and t.model_id like '%#tile' and t.chunk_ref like 'v:' || e.current_version_id::text || ':tile:%')"
    missing = f"not {tile_exists}" if args.tiles else "not exists (select 1 from forensic.image_hash h where h.tenant_id = e.tenant_id and h.collection_id = e.collection_id and h.evidence_id = e.evidence_id and h.version_id = e.current_version_id)"
    rows = sql(f"""select e.evidence_id, e.original_filename from forensic.evidence_items e
                   where e.tenant_id = '{tenant}' and e.collection_id = '{c}' and e.modality = 'image' and e.processing_status = 'completed' and e.current_version_id is not null
                     and {missing}
                   order by e.created_at""")
    total = sql(f"select count(*) from forensic.evidence_items where tenant_id = '{tenant}' and collection_id = '{c}' and modality = 'image'")[0][0]
    print(f"{total} images in {c}; {len(rows)} without descriptors")
    if args.limit:
        rows = rows[: args.limit]
    if args.dry:
        for evidence_id, name in rows:
            print("  would reprocess", evidence_id, name)
        return 0
    done_count = failed = 0
    for n, (evidence_id, name) in enumerate(rows, start=1):
        body = json.dumps({"tenant_id": tenant, "collection_id": c, "reason": "image search window vectors (module M1)" if args.tiles else "image search indexing (module M1)"}).encode()
        request = urllib.request.Request(
            f"{API}/evidence/{evidence_id}/reprocess", data=body, method="POST",
            headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json", "Idempotency-Key": f"img-search-{'tiles' if args.tiles else 'index'}-20261123{args.key_suffix}-{evidence_id}",
                     "X-Forensic-Tenant-ID": tenant, "X-Forensic-Collection-ID": c, "X-Forensic-Actor-ID": "image-search-indexer", "X-Forensic-Subject-ID": "image-search-indexer", "X-Forensic-Actor-Role": "user"})
        try:
            with urllib.request.urlopen(request, timeout=60) as response:
                answer = json.loads(response.read().decode())
        except urllib.error.HTTPError as error:
            print(f"  [{n}/{len(rows)}] {name}: HTTP {error.code} {error.read().decode()[:160]}")
            failed += 1
            continue
        except OSError as error:
            print(f"  [{n}/{len(rows)}] {name}: {error}")
            failed += 1
            continue
        # wait until the worker has finished the image: its hash row appears (or the job ends without one)
        started = time.time()
        state = "waiting"
        while time.time() - started < args.timeout:
            if args.tiles:
                have = sql(f"select 1 from forensic.content_embedding t join forensic.evidence_items e on e.evidence_id = t.evidence_id where t.evidence_id = '{evidence_id}' and t.model_id like '%#tile' and t.chunk_ref like 'v:' || e.current_version_id::text || ':tile:%' limit 1")
            else:
                have = sql(f"select 1 from forensic.image_hash h join forensic.evidence_items e on e.evidence_id = h.evidence_id and e.current_version_id = h.version_id where h.evidence_id = '{evidence_id}' limit 1")
            if have:
                state = "indexed"
                break
            job = sql(f"select status::text from forensic.records_ingest_jobs where id = '{answer.get('job_id', '')}'")
            if job and job[0][0] in ("failed", "dead_letter"):
                state = "job " + job[0][0]
                break
            if job and job[0][0] == "completed":
                time.sleep(3)
                recheck = sql(f"select 1 from forensic.content_embedding t where t.evidence_id = '{evidence_id}' and t.model_id like '%#tile' limit 1") if args.tiles else sql(f"select 1 from forensic.image_hash where evidence_id = '{evidence_id}' limit 1")
                if recheck:
                    state = "indexed"
                else:
                    state = "job completed but no " + ("window vectors" if args.tiles else "descriptors") + " (the service was not reachable or the file is not an image)"
                break
            time.sleep(3)
        done_count += state == "indexed"
        failed += state != "indexed"
        print(f"  [{n}/{len(rows)}] {name}: {state} ({time.time() - started:.0f} s)", flush=True)
    print(f"done: {done_count} indexed, {failed} not")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
