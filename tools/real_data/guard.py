"""Location guards: real data must not be read from, or reports written to, a place that could publish or sync them.

The tools in this folder call these before touching a file. A guard failure is a refusal, not a warning; the only way past it is
the explicit allow_unsafe flag (used by the tests, which run on synthetic files).
"""
import os
import re

CLOUD_SYNC_MARKERS = re.compile(r"(^|[\\/])(onedrive|dropbox|google ?drive|icloud|icloud drive|box|mega|sync\.com)([^\\/]*)([\\/]|$)", re.I)
SCRATCH_MARKERS = re.compile(r"(^|[\\/])(appdata[\\/]local[\\/]temp|temp|tmp|scratch|scratchpad|downloads)([\\/]|$)", re.I)


def find_git_root(path):
    """The nearest ancestor directory that contains a .git entry, or None."""
    current = os.path.abspath(path)
    if os.path.isfile(current):
        current = os.path.dirname(current)
    while True:
        if os.path.exists(os.path.join(current, ".git")):
            return current
        parent = os.path.dirname(current)
        if parent == current:
            return None
        current = parent


def location_problems(path):
    """Reasons the path is an unsafe place for real data. An empty list means it is acceptable."""
    full = os.path.abspath(path)
    problems = []
    root = find_git_root(full)
    if root:
        problems.append("inside a git work tree (%s): real data must live outside every repository" % os.path.basename(root))
    if CLOUD_SYNC_MARKERS.search(full):
        problems.append("inside a cloud-synchronised folder: real data must not be uploaded by a sync client")
    if SCRATCH_MARKERS.search(full):
        problems.append("inside a temporary, scratch or Downloads folder: move the file into the vault first")
    return problems


def require_safe_location(path, allow_unsafe=False, role="input"):
    problems = location_problems(path)
    if problems and not allow_unsafe:
        raise SystemExit("refusing to use %s %r: %s. Use the vault (tools/real_data/Initialize-Vault.ps1)." % (
            role, os.path.basename(path), "; ".join(problems)))
    return problems
