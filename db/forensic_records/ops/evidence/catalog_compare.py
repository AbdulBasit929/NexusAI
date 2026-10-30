"""Compares two catalog listings (catalog_lines.sql output): lines added, lines removed, lines changed (same object, different definition).  Usage: catalog_compare.py BEFORE AFTER [--show-added]"""
import sys


def key(line):
    kind, rest = line.split("|", 1)
    parts = rest.split("|")
    # an object is identified by its kind and name (columns by table and ordinal+name; constraints, indexes and policies by table and name; the rest by name)
    if kind in ("column",):
        return (kind, parts[0], parts[2])
    if kind in ("constraint", "index", "policy", "comment"):
        return (kind, parts[0], parts[1])
    return (kind, parts[0])


def load(path):
    with open(path, encoding="utf-8") as handle:
        lines = [l.rstrip("\n") for l in handle if l.strip()]
    out = {}
    for line in lines:
        out.setdefault(key(line), []).append(line)
    return out, set(lines)


before, bset = load(sys.argv[1])
after, aset = load(sys.argv[2])
added = sorted(aset - bset)
removed = sorted(bset - aset)
changed, truly_removed, truly_added = [], [], []
for line in removed:
    k = key(line)
    if k in after:
        changed.append(line)
    else:
        truly_removed.append(line)
for line in added:
    if key(line) not in before:
        truly_added.append(line)
print("before %d lines, after %d lines" % (len(bset), len(aset)))
print("added objects: %d, removed objects: %d, changed objects: %d" % (len(truly_added), len(truly_removed), len(changed)))
kinds = {}
for line in truly_added:
    kinds[line.split("|", 1)[0]] = kinds.get(line.split("|", 1)[0], 0) + 1
print("added by kind:", dict(sorted(kinds.items())))
for line in truly_removed:
    print("REMOVED:", line[:240])
for line in changed:
    print("CHANGED (before):", line[:300])
    for new in after[key(line)]:
        if new not in bset:
            print("CHANGED (after): ", new[:300])
if "--show-added" in sys.argv:
    for line in truly_added:
        print("ADDED:", line[:200])
