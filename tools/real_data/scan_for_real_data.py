#!/usr/bin/env python3
"""Stop real operator data, personal identifiers and secrets from entering this public repository.

    python -I -X utf8 tools/real_data/scan_for_real_data.py --staged                 # what a commit is about to add
    python -I -X utf8 tools/real_data/scan_for_real_data.py --rev-range origin/main..HEAD   # what a push would publish
    python -I -X utf8 tools/real_data/scan_for_real_data.py --paths PATH [PATH ...]  # whole files; a folder is read recursively (audit)
    python -I -X utf8 tools/real_data/scan_for_real_data.py --all-tracked            # the whole tree (audit, slower)

It reads ADDED lines only for --staged and --rev-range, so existing content never blocks new work, and it drops a finding whose
value already occurs in the base revision (re-adding a value that is already public is not new exposure; one batched git grep,
disable with --no-known-values). The audit modes (--paths, --all-tracked) report everything: that is how values committed
before this tool existed get reviewed. Findings print as severity, rule, file:line and a MASKED token; a matching line is never
echoed.

Severities: block (fails the run) and review (printed; fails only with --fail-on review).

Rules (see docs/data/real-data-handling-policy.md, section 7):
  block   secret.*          private key blocks, GitHub/AWS/Anthropic/Slack tokens
  block   id.cnic           national ID number written with dashes (a valid province digit 1 to 7)
  block   id.imsi_pk        15-digit IMSI under mobile country code 410 (Pakistan)
  block   num.msisdn        Pakistani mobile number outside the reserved synthetic block
  block   net.public_ip     public IP address in a data-like file (csv, tsv, txt, json, jsonl, xml, log)
  block   file.real_name    file name that carries a long subscriber, IMSI, IMEI or CNIC number
  review  net.public_ip     public IP address in code or documentation
  review  num.imei_like     15-digit number that passes the IMEI check digit and is not a test IMEI
  review  id.cnic_context   13-digit number on a line that talks about a CNIC or NIC
  review  geo.pk_coords     coordinate pair inside Pakistan with four or more decimals
  review  file.media_or_archive  newly added photo, video, audio, archive, spreadsheet, PDF or capture file
A line that contains the marker real-data-ok is skipped (use it on an intentional, documented synthetic value).
"""
import argparse
import ipaddress
import json
import os
import re
import subprocess
import sys
import tempfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))  # python -I does not add the script folder itself
import reserved  # noqa: E402

DATA_EXTENSIONS = {".csv", ".tsv", ".txt", ".json", ".jsonl", ".ndjson", ".xml", ".log", ".dat"}
MEDIA_EXTENSIONS = {".jpg", ".jpeg", ".png", ".bmp", ".tif", ".tiff", ".gif", ".webp", ".heic", ".mp4", ".avi", ".mov", ".mkv",
                    ".wav", ".mp3", ".m4a", ".flac", ".ogg", ".opus", ".rar", ".zip", ".7z", ".gz", ".tar", ".xlsx", ".xls",
                    ".docx", ".doc", ".pdf", ".pcap", ".pcapng"}
IGNORED_PATH_PARTS = ("node_modules/", "vendor/", "third_party/", ".git/")
IGNORED_FILES = ("go.sum", "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "poetry.lock", "Cargo.lock")
PRAGMA = "real-data-ok"

SECRET_RULES = [
    ("secret.private_key", re.compile(r"-----BEGIN (?:[A-Z]+ )*PRIVATE KEY-----")),
    ("secret.github_token", re.compile(r"\b(?:gh[pousr]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{50,})\b")),
    ("secret.aws_access_key", re.compile(r"\bAKIA[0-9A-Z]{16}\b")),
    ("secret.anthropic_key", re.compile(r"\bsk-ant-[A-Za-z0-9_\-]{20,}\b")),
    ("secret.slack_token", re.compile(r"\bxox[baprs]-[A-Za-z0-9\-]{10,}\b")),
]
RE_MSISDN = re.compile(r"(?<![\d.])(?:\+\s?92|0092|92|0)[ \-]?3\d{2}[ \-]?\d{7}(?![\d])")
RE_CNIC = re.compile(r"(?<!\d)[1-7]\d{4}-\d{7}-\d(?!\d)")
RE_CNIC_PLAIN = re.compile(r"(?<!\d)[1-7]\d{12}(?!\d)")
RE_CNIC_WORD = re.compile(r"(?i)\b(cnic|nic|identity card|national id)\b")
RE_IMSI_PK = re.compile(r"(?<!\d)410\d{12}(?!\d)")
RE_DIGITS15 = re.compile(r"(?<![\d.])\d{15}(?!\d)")  # not the fraction of a decimal number (a 15-place coordinate is not an IMEI)
RE_IPV4 = re.compile(r"(?<![\d.])(?:\d{1,3}\.){3}\d{1,3}(?![\d]|\.\d)")
# Word boundaries on both sides keep SQL casts and identifiers (id::text, d::int) from reading as addresses.
RE_IPV6 = re.compile(r"(?<![0-9A-Za-z_:.])(?:[0-9A-Fa-f]{0,4}:){2,7}[0-9A-Fa-f]{0,4}(?![0-9A-Za-z_:.])")
RE_COORDS = re.compile(r"(?<![\d.])(?:2[3-9]|3[0-7])\.\d{4,}\s*[,;\t ]\s*(?:6\d|7[0-8])\.\d{4,}(?![\d])")
RE_VERSIONISH = re.compile(r"(?i)(version|ver\.|semver|go\.mod|requires?\s|==|~=|>=|<=|@v?\d)")
RE_REAL_NAME = re.compile(r"(?i)((msisdn|imsi|imei|cnic)[-_ ]?\d{8,}|(?<!\d)92\d{10}(?!\d))")
WELL_KNOWN_PUBLIC = {"8.8.8.8", "8.8.4.4", "1.1.1.1", "1.0.0.1", "9.9.9.9", "208.67.222.222", "208.67.220.220"}


class Finding(object):
    """One finding. token is the exact text to look up when asking whether a value is already in the repository; it is never
    printed (format() shows the masked excerpt only)."""

    def __init__(self, severity, rule, path, line, excerpt, token=None):
        self.severity, self.rule, self.path, self.line, self.excerpt = severity, rule, path, line, excerpt
        self.token = token

    def format(self):
        return "%-6s %-22s %s:%s  %s" % (self.severity, self.rule, self.path, self.line or "-", self.excerpt)


def mask_token(token, keep=3):
    token = str(token)
    return token[:keep] + "*" * max(0, len(token) - keep) + " (%d chars)" % len(token)


def mask_ip(token):
    if ":" in token:
        return token.split(":")[0] + ":*"
    parts = token.split(".")
    return ".".join(parts[:2] + ["*", "*"])


def load_allowlist(path):
    allow = {"msisdn": set(), "ips": set(), "paths": [], "ignore_paths": []}
    if path and os.path.exists(path):
        with open(path, "r", encoding="utf-8") as handle:
            data = json.load(handle)
        allow["msisdn"] = {reserved.canonical_msisdn(x) or x for x in data.get("msisdn", [])}
        allow["ips"] = set(data.get("ips", []))
        allow["paths"] = list(data.get("paths", []))
        allow["ignore_paths"] = list(data.get("ignore_paths", []))
    return allow


def path_ignored(path, allow):
    norm = path.replace("\\", "/")
    base = norm.rsplit("/", 1)[-1]
    return (base in IGNORED_FILES or any(part in norm for part in IGNORED_PATH_PARTS)
            or any(norm.startswith(p) for p in allow["ignore_paths"]))


def scan_line(path, line_no, text, allow):
    """All findings for one line of text. Pure; the tests call it directly."""
    if PRAGMA in text:
        return []
    out = []
    ext = os.path.splitext(path)[1].lower()
    data_like = ext in DATA_EXTENSIONS

    for rule, regex in SECRET_RULES:
        if regex.search(text):
            out.append(Finding("block", rule, path, line_no, "[secret pattern; value not shown]"))

    for m in RE_CNIC.finditer(text):
        out.append(Finding("block", "id.cnic", path, line_no, mask_token(m.group(0), 2), m.group(0)))
    if RE_CNIC_WORD.search(text):
        for m in RE_CNIC_PLAIN.finditer(text):
            out.append(Finding("review", "id.cnic_context", path, line_no, mask_token(m.group(0), 2), m.group(0)))

    for m in RE_IMSI_PK.finditer(text):
        out.append(Finding("block", "id.imsi_pk", path, line_no, mask_token(m.group(0), 3), m.group(0)))

    for m in RE_MSISDN.finditer(text):
        canon = reserved.canonical_msisdn(m.group(0))
        if not canon or reserved.is_reserved_msisdn(canon) or canon in allow["msisdn"]:
            continue
        # the last ten digits identify the number in any written form (+92, 0092, 92, 03...)
        out.append(Finding("block", "num.msisdn", path, line_no, mask_token(canon, 4), canon[-10:]))

    for m in RE_DIGITS15.finditer(text):
        digits = m.group(0)
        if digits.startswith("410") or reserved.is_test_imei(digits) or reserved.is_test_imsi(digits):
            continue
        if reserved.luhn_ok(digits):
            out.append(Finding("review", "num.imei_like", path, line_no, mask_token(digits, 2), digits))

    if not RE_VERSIONISH.search(text):
        for m in RE_IPV4.finditer(text):
            token = m.group(0)
            octets = token.split(".")
            if any(int(o) > 255 or (len(o) > 1 and o.startswith("0")) for o in octets):
                continue
            if reserved.is_reserved_ip(token) or token in WELL_KNOWN_PUBLIC or token in allow["ips"]:
                continue
            out.append(Finding("block" if data_like else "review", "net.public_ip", path, line_no, mask_ip(token), token))
    if text.count(":") >= 2:
        for m in RE_IPV6.finditer(text):
            token = m.group(0)
            if token.count(":") < 2 or len(token) < 5:
                continue
            try:
                addr = ipaddress.IPv6Address(token)
            except ValueError:
                continue
            if reserved.is_reserved_ip(token) or token in allow["ips"] or addr.is_unspecified or addr.ipv4_mapped:
                continue
            out.append(Finding("block" if data_like else "review", "net.public_ip", path, line_no, mask_ip(token), token))

    for m in RE_COORDS.finditer(text):
        out.append(Finding("review", "geo.pk_coords", path, line_no, "coordinate pair inside Pakistan (not shown)", m.group(0)))
    return out


def scan_path_name(path, allow, newly_added=True):
    findings = []
    norm = path.replace("\\", "/")
    if RE_REAL_NAME.search(norm.rsplit("/", 1)[-1]):
        findings.append(Finding("block", "file.real_name", path, None, "file name carries a long subscriber-style number"))
    ext = os.path.splitext(norm)[1].lower()
    if newly_added and ext in MEDIA_EXTENSIONS and not any(norm.startswith(p) for p in allow["paths"]):
        findings.append(Finding("review", "file.media_or_archive", path, None,
                                "new %s file: confirm it is synthetic or licensed, or add its folder to allowlist.json" % ext))
    return findings


def scan_text(path, text, allow, first_line=1):
    findings = []
    for i, line in enumerate(text.splitlines(), start=first_line):
        findings.extend(scan_line(path, i, line, allow))
    return findings


def scan_file(path, allow, max_bytes=5 * 1024 * 1024):
    findings = scan_path_name(path, allow, newly_added=True)
    try:
        if os.path.getsize(path) > max_bytes:
            return findings
        with open(path, "rb") as handle:
            raw = handle.read()
    except OSError:
        return findings
    if b"\x00" in raw[:8192]:
        return findings
    findings.extend(scan_text(path, raw.decode("utf-8", errors="replace"), allow))
    return findings


def run_git(args, cwd=None):
    result = subprocess.run(["git", "-c", "core.quotepath=false"] + args, cwd=cwd, capture_output=True)
    if result.returncode != 0:
        raise SystemExit("git %s failed: %s" % (" ".join(args[:2]), result.stderr.decode("utf-8", "replace").strip()[:200]))
    return result.stdout.decode("utf-8", errors="replace")


def parse_added_lines(diff_text):
    """Yield (path, line_number, text) for every added line of a unified diff produced with -U0."""
    path = None
    line_no = 0
    for raw in diff_text.splitlines():
        if raw.startswith("+++ "):
            target = raw[4:].strip()
            path = None if target == "/dev/null" else (target[2:] if target.startswith("b/") else target)
            continue
        if raw.startswith("--- ") or raw.startswith("diff --git") or raw.startswith("index ") or raw.startswith("Binary files"):
            continue
        m = re.match(r"@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@", raw)
        if m:
            line_no = int(m.group(1))
            continue
        if raw.startswith("+") and path:
            yield path, line_no, raw[1:]
            line_no += 1


def known_in_base(tokens, base_rev, cwd):
    """Which of the tokens already occur in the tree at base_rev, in one batched git grep.

    Re-adding a value that is already public adds no exposure, so such findings are dropped; the whole-tree audit
    (--all-tracked) is how pre-existing values get reviewed. Returns an empty set when there is nothing to compare with."""
    if not tokens or not base_rev:
        return set()
    handle = tempfile.NamedTemporaryFile("w", delete=False, encoding="utf-8", newline="\n", suffix=".patterns")
    try:
        handle.write("\n".join(sorted(tokens)) + "\n")
        handle.close()
        result = subprocess.run(["git", "-c", "core.quotepath=false", "grep", "-F", "-o", "-h", "-I", "-f", handle.name, base_rev, "--"],
                                cwd=cwd, capture_output=True)
    finally:
        os.unlink(handle.name)
    if result.returncode not in (0, 1):
        return set()
    found = set()
    for raw in result.stdout.decode("utf-8", errors="replace").splitlines():
        found.add(raw.strip())
    return found & set(tokens)


def drop_known(findings, base_rev, cwd):
    candidates = {f.token for f in findings if f.token}
    known = known_in_base(candidates, base_rev, cwd)
    return [f for f in findings if not (f.token and f.token in known)]


def scan_diff(args_for_diff, allow, cwd=None, base_rev=None):
    findings = []
    names = run_git(["diff", "-z", "--name-status", "--diff-filter=A"] + args_for_diff, cwd)
    parts = [p for p in names.split("\0") if p]
    added = [parts[i + 1] for i in range(0, len(parts) - 1, 2) if parts[i].startswith("A")]
    for path in added:
        if not path_ignored(path, allow):
            findings.extend(scan_path_name(path, allow, newly_added=True))
    diff = run_git(["diff", "-U0", "--no-color", "--no-ext-diff", "--diff-filter=ACMR"] + args_for_diff, cwd)
    for path, line_no, text in parse_added_lines(diff):
        if not path_ignored(path, allow):
            findings.extend(scan_line(path, line_no, text, allow))
    return drop_known(findings, base_rev, cwd)


SKIPPED_FOLDERS = {".git", "__pycache__", "node_modules"}


def expand_paths(paths):
    """The files named directly plus every file under a named folder, and the names that are neither (a typo must not pass as clean)."""
    files, missing = [], []
    for p in paths:
        if os.path.isfile(p):
            files.append(p)
        elif os.path.isdir(p):
            for root, folders, names in os.walk(p):
                folders[:] = sorted(f for f in folders if f not in SKIPPED_FOLDERS)
                files.extend(os.path.join(root, n) for n in sorted(names))
        else:
            missing.append(p)
    return files, missing


def main(argv=None):
    ap = argparse.ArgumentParser(description="Scan for real data, identifiers and secrets before they reach the public repository.")
    group = ap.add_mutually_exclusive_group(required=True)
    group.add_argument("--staged", action="store_true", help="added lines of the staged changes")
    group.add_argument("--rev-range", help="added lines of a revision range, for example origin/main..HEAD")
    group.add_argument("--paths", nargs="+", help="scan whole files; a folder is read recursively")
    group.add_argument("--all-tracked", action="store_true", help="scan every tracked file (audit)")
    ap.add_argument("--fail-on", choices=["block", "review", "never"], default="block")
    ap.add_argument("--allowlist", default=os.path.join(os.path.dirname(os.path.abspath(__file__)), "allowlist.json"))
    ap.add_argument("--repo", default=".", help="repository to inspect (default: current directory)")
    ap.add_argument("--no-known-values", action="store_true",
                    help="also report values that already occur in the base revision (default: they are not new exposure)")
    args = ap.parse_args(argv)

    allow = load_allowlist(args.allowlist)
    missing = []
    if args.staged:
        base = None if args.no_known_values else (
            "HEAD" if subprocess.run(["git", "rev-parse", "--verify", "-q", "HEAD"], cwd=args.repo, capture_output=True).returncode == 0 else None)
        findings = scan_diff(["--cached"], allow, args.repo, base)
    elif args.rev_range:
        base = None if args.no_known_values or ".." not in args.rev_range else args.rev_range.split("..")[0] or None
        findings = scan_diff([args.rev_range], allow, args.repo, base)
    elif args.paths:
        findings = []
        files, missing = expand_paths(args.paths)
        for p in files:
            if not path_ignored(p, allow):
                findings.extend(scan_file(p, allow))
        for p in missing:
            print("real-data scan: no such file or folder: %s" % p, file=sys.stderr)
    else:
        tracked = [p for p in run_git(["ls-files", "-z"], args.repo).split("\0") if p]
        findings = []
        for p in tracked:
            full = os.path.join(args.repo, p)
            if not path_ignored(p, allow) and os.path.isfile(full):
                findings.extend(scan_file(full, allow))

    findings.sort(key=lambda f: (0 if f.severity == "block" else 1, f.path, f.line or 0))
    for f in findings:
        print(f.format())
    blocks = sum(1 for f in findings if f.severity == "block")
    reviews = len(findings) - blocks
    print("real-data scan: %d block, %d review" % (blocks, reviews))
    if missing:
        return 2  # a path that does not exist is a usage error whatever --fail-on says
    if blocks and args.fail_on in ("block", "review"):
        print("Blocked. Replace real values with synthetic ones from tools/real_data/reserved.py, or mark an intentional "
              "synthetic value with '%s' (see docs/data/real-data-handling-policy.md)." % PRAGMA)
        return 1
    if reviews and args.fail_on == "review":
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
