#!/usr/bin/env python3
"""Media role readiness, read-only (master plan 8.3, M0.1; decision D25).

Reads the worker container's media switches and checks that the model files each role names exist inside the container. Changes nothing; prints no secret.
A role is switched on only by the owner's go, one family at a time, on controlled data, and switched off again if its gate fails (the order is in docs/roadmap/master-plan-20261021.md, 8.4).

    python -X utf8 tools/media/activation_check.py [--container nexusai-forensic-records-worker-1]
"""
import argparse
import subprocess
import sys

ORDER = [
    ("D documents and printed OCR", ["FORENSIC_OCR_"]),
    ("I general image forensics (embeddings, fingerprints)", ["FORENSIC_IMAGE_EMBEDDING_"]),
    ("P plates on images", ["FORENSIC_ANPR_"]),
    ("A audio and speech", ["FORENSIC_ASR_"]),
    ("V video plates v3 (v2 is AGPL and blocked)", ["FORENSIC_VIDEO_ANPR_V3_", "FORENSIC_VIDEO_ANPR_VEHICLE", "FORENSIC_VIDEO_ANPR_PLATE", "FORENSIC_VIDEO_MAX"]),
    ("F faces (detection only, decision D5)", ["FORENSIC_FACE_"]),
]
SECRET = ("KEY", "SECRET", "PASSWORD", "TOKEN", "DSN")


def sh(*a):
    return subprocess.run(list(a), capture_output=True, text=True)


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--container", default="nexusai-forensic-records-worker-1")
    args = ap.parse_args(argv)
    out = sh("docker", "inspect", args.container, "--format", "{{range .Config.Env}}{{println .}}{{end}}")
    if out.returncode:
        print("cannot read the container:", out.stderr.strip()[:200]); return 1
    env = {}
    for line in out.stdout.splitlines():
        if "=" in line:
            k, v = line.split("=", 1)
            if not any(s in k for s in SECRET):
                env[k] = v
    print("container %s; the role switches and model files below are read-only facts\n" % args.container)
    print("%-52s %-9s %s" % ("family (in the order of the plan)", "role", "missing files"))
    for name, prefixes in ORDER:
        keys = sorted(k for k in env if any(k.startswith(p) for p in prefixes))
        flags = [k for k in keys if k.endswith("_ENABLED")]
        state = ", ".join("%s" % ("on" if env[k].lower() == "true" else "off") for k in flags) or "n/a"
        missing = []
        for k in keys:
            v = env[k]
            if k.endswith(("_PATH", "_DIR")) and v.startswith("/"):
                if sh("docker", "exec", args.container, "test", "-e", v).returncode:
                    missing.append(k)
        print("%-52s %-9s %s" % (name, state, ", ".join(missing) if missing else "none"))
    print("\nTo switch one role on: the owner's go, a rollback tag of the worker image, then the documented recreate with that one variable; run the family's pack first; keep it on only if its gate passes.")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
