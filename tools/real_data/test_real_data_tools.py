"""Tests for the real-data tools. Synthetic files only; no real value appears here, and values that look real are assembled at run
time so the scanner does not flag this file.

    python -I -X utf8 tools/real_data/test_real_data_tools.py        # plain runner (no pytest needed)
    python -m pytest tools/real_data/test_real_data_tools.py         # also works under pytest
"""
import contextlib
import io
import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import traceback
import zipfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import guard  # noqa: E402
import profile_structure as ps  # noqa: E402
import reconcile_aggregates as ra  # noqa: E402
import reserved  # noqa: E402
import scan_for_real_data as scan  # noqa: E402

# Values that look real, assembled from parts so no 12-digit literal sits in this file.
LOOKS_REAL_MSISDN = "92" + "345" + "9999999"
LOOKS_REAL_LOCAL = "0" + "345" + "9999999"
LOOKS_REAL_IMSI = "410" + "01" + "1234567890"
LOOKS_REAL_CNIC = "35202" + "-" + "1234567" + "-" + "1"
PUBLIC_IP = ".".join(["45", "33", "21", "9"])
SYNTH_NUMBER = reserved.synthetic_msisdn(7)
PLANTED_TEXT = "PLANTED" + "NAME" + "MARKER"


def _make_writable_and_retry(func, path, _exc_info):
    os.chmod(path, stat.S_IWRITE)
    func(path)


@contextlib.contextmanager
def workdir():
    path = tempfile.mkdtemp(prefix="rdtools-")
    try:
        yield path
    finally:
        shutil.rmtree(path, onerror=_make_writable_and_retry)  # git object files are read-only on Windows


def write_csv(path, header, rows, delimiter=",", encoding="utf-8"):
    with open(path, "w", encoding=encoding, newline="") as handle:
        for row in [header] + rows:
            handle.write(delimiter.join(str(c) for c in row) + "\r\n")


def synthetic_rows(n=40):
    rows = []
    for i in range(n):
        rows.append([
            reserved.synthetic_msisdn(i % 5),
            reserved.synthetic_msisdn(100 + i % 9),
            "Incoming" if i % 3 else "Outgoing",
            "2026-01-%02d 10:%02d:00" % (1 + i % 28, i % 60),
            str(30 + i),
            PLANTED_TEXT + str(i),
        ])
    return rows


HEADER = ["MSISDN", "B_PARTY", "DIRECTION", "CALL_START", "DURATION", "REMARKS"]


def profile(path, mode="shareable", **kw):
    return ps.build_report(path, mode, kw.get("sheet"), kw.get("all_sheets", False), kw.get("max_rows", 0), kw.get("max_distinct", 200000))


# --------------------------------------------------------------------------------------------------------------------------
# reserved
# --------------------------------------------------------------------------------------------------------------------------
def test_canonical_msisdn_forms():
    core = "345" + "9999999"
    for text in ("+92" + core, "0092" + core, "92" + core, "0" + core, core, "92 345 9999999", "+92-345-9999999"):  # real-data-ok: invented all-nines number
        assert reserved.canonical_msisdn(text) == "92" + core, text
    assert reserved.canonical_msisdn("92" + "21" + "35800000") is None  # a landline is not a mobile number
    assert reserved.canonical_msisdn("hello") is None


def test_reserved_msisdn_block():
    assert reserved.is_reserved_msisdn(SYNTH_NUMBER)
    assert reserved.is_reserved_msisdn("0" + SYNTH_NUMBER[2:])
    assert not reserved.is_reserved_msisdn(LOOKS_REAL_MSISDN)
    for n in (0, 99999):
        assert reserved.synthetic_msisdn(n).startswith("9230000") and len(reserved.synthetic_msisdn(n)) == 12
    try:
        reserved.synthetic_msisdn(100000)
    except ValueError:
        pass
    else:
        raise AssertionError("n beyond the block must be refused")


def test_luhn_and_synthetic_identifiers():
    assert reserved.luhn_ok("79927398713")
    assert not reserved.luhn_ok("79927398710")
    imei = reserved.synthetic_imei(12345)
    assert len(imei) == 15 and imei.startswith("00") and reserved.is_test_imei(imei)
    imsi = reserved.synthetic_imsi(42)
    assert len(imsi) == 15 and reserved.is_test_imsi(imsi)
    assert reserved.synthetic_cnic(5).startswith("00000-")


def test_reserved_ips():
    for ok in (reserved.synthetic_ipv4(0), reserved.synthetic_ipv4(500), "10.1.2.3", "192.168.0.7", "100.64.1.1", "127.0.0.1",
               reserved.synthetic_ipv6(3), "fe80::1", "fc00::5"):
        assert reserved.is_reserved_ip(ok), ok
    assert not reserved.is_reserved_ip(PUBLIC_IP)
    assert not reserved.is_reserved_ip("2606:4700:4700::1111")  # real-data-ok: a public resolver address as the negative case
    assert not reserved.is_reserved_ip("not an address")
    seen = {reserved.synthetic_ipv4(i) for i in range(762)}
    assert len(seen) == 762


# --------------------------------------------------------------------------------------------------------------------------
# guard
# --------------------------------------------------------------------------------------------------------------------------
def test_guard_flags_repo_cloud_and_scratch():
    with workdir() as root:
        os.makedirs(os.path.join(root, "repo", ".git"))
        inside = os.path.join(root, "repo", "data.csv")
        open(inside, "w").close()
        problems = guard.location_problems(inside)
        assert any("git work tree" in p for p in problems), problems
        cloud = os.path.join(root, "OneDrive", "docs", "x.csv")
        assert any("cloud" in p for p in guard.location_problems(cloud))
        assert any("Downloads" in p or "scratch" in p for p in guard.location_problems(os.path.join(root, "Downloads", "x.csv")))
        try:
            guard.require_safe_location(inside)
        except SystemExit as exc:
            assert "refusing" in str(exc)
        else:
            raise AssertionError("an input inside a git work tree must be refused")
        assert guard.require_safe_location(inside, allow_unsafe=True)  # explicit override returns the problems


# --------------------------------------------------------------------------------------------------------------------------
# profile_structure
# --------------------------------------------------------------------------------------------------------------------------
def test_mask_shapes():
    assert ps.mask("abc-1234") == "aaa-9999"
    assert ps.mask("ABC") == "AAA"
    assert ps.mask("92300" + "1234567") == "9{12}"
    assert ps.mask("ابج") == "uuu"
    assert ps.mask("x" * 100, limit=10) == "a{100}"[:10]
    assert ps.magnitude_class(0) == "0" and ps.magnitude_class(7) == "1-9" and ps.magnitude_class(1234) == "10^3..10^4"


def test_value_classes_never_return_the_value():
    for value, expected in ((LOOKS_REAL_MSISDN, "92_prefix_12_digits"), (LOOKS_REAL_LOCAL, "03_prefix_11_digits"),
                            ("345" + "9999999", "3_prefix_10_digits"), ("12345", "short_code_up_to_6"), ("BANKNAME", "alpha_label"),
                            ("INFO1", "alnum_sender_id"), ("", "empty")):
        got = ps.value_class(value)
        assert got == expected, (got, expected)
        assert value not in got or value == ""


def test_profile_csv_shareable_has_no_values():
    with workdir() as d:
        path = os.path.join(d, "calls.csv")
        write_csv(path, HEADER, synthetic_rows())
        text = json.dumps(profile(path))
        assert PLANTED_TEXT not in text
        assert SYNTH_NUMBER not in text and reserved.synthetic_msisdn(0) not in text
        report = json.loads(text)
        sheet = report["sheets"][0]
        by_name = {c["header"]: c for c in sheet["columns"]}
        assert list(by_name) == HEADER
        assert "vocabulary" in by_name["DIRECTION"] and set(by_name["DIRECTION"]["vocabulary"]) == {"Incoming", "Outgoing"}
        assert "vocabulary" not in by_name["REMARKS"] and "vocabulary" not in by_name["MSISDN"]
        assert by_name["MSISDN"]["value_classes"] == {"92_prefix_12_digits": 1.0}
        assert by_name["CALL_START"]["time"]["formats"] == {"iso_naive": 1.0}
        assert "row_count" not in sheet and "sha256" not in report["file"] and sheet["row_count_class"] == "10^1..10^2"
        assert "min" not in by_name["CALL_START"]["time"]
        assert by_name["DURATION"].get("numeric_range") is None


def test_profile_local_mode_has_exact_aggregates():
    with workdir() as d:
        path = os.path.join(d, "calls.csv")
        write_csv(path, HEADER, synthetic_rows(40))
        report = profile(path, "local")
        sheet = report["sheets"][0]
        by_name = {c["header"]: c for c in sheet["columns"]}
        assert sheet["row_count"] == 40 and len(report["file"]["sha256"]) == 64
        assert by_name["MSISDN"]["distinct"] == 5 and by_name["B_PARTY"]["distinct"] == 9
        assert by_name["DIRECTION"]["vocabulary"] == {"Incoming": 26, "Outgoing": 14}
        assert by_name["CALL_START"]["time"]["min"] <= by_name["CALL_START"]["time"]["max"]
        assert by_name["DURATION"]["numeric_range"] == [30.0, 69.0]


def test_profile_text_variants():
    with workdir() as d:
        tsv16 = os.path.join(d, "ipdr.tsv")
        write_csv(tsv16, HEADER, synthetic_rows(12), delimiter="\t", encoding="utf-16")
        report = profile(tsv16, "local")
        assert report["file"]["encoding"] == "utf-16" and report["file"]["delimiter"] == "tab"
        assert report["sheets"][0]["row_count"] == 12
        semi = os.path.join(d, "x.csv")
        write_csv(semi, HEADER, synthetic_rows(5), delimiter=";", encoding="utf-8-sig")
        assert profile(semi, "local")["file"]["delimiter"] == "semicolon"


def test_preamble_and_footer_are_masked():
    with workdir() as d:
        path = os.path.join(d, "ufdr.csv")
        with open(path, "w", encoding="utf-8", newline="") as handle:
            handle.write("Basic Information\r\n")
            handle.write("Target Number,%s\r\n" % SYNTH_NUMBER)
            handle.write("Records shown,40\r\n\r\n")
            handle.write(",".join(HEADER) + "\r\n")
            for row in synthetic_rows(10):
                handle.write(",".join(str(c) for c in row) + "\r\n")
            handle.write("Total records,10\r\n")
        report = profile(path, "local")
        text = json.dumps(report)
        assert SYNTH_NUMBER not in text and "Target Number" not in text
        sheet = report["sheets"][0]
        assert sheet["header_row_index"] == 4 and len(sheet["preamble_rows"]) == 3
        assert sheet["footer_masked"] and "9" in sheet["footer_masked"][-1]


def test_time_day_month_order():
    with workdir() as d:
        ambiguous = os.path.join(d, "a.csv")
        write_csv(ambiguous, ["MSISDN", "B_PARTY", "CALL_START", "DURATION"],
                  [[SYNTH_NUMBER, SYNTH_NUMBER, "03/04/2026 10:00:00", "5"] for _ in range(8)])
        t = profile(ambiguous, "local")["sheets"][0]["columns"][2]["time"]
        assert t["day_month_order"] == "ambiguous" and "min" not in t
        dayfirst = os.path.join(d, "b.csv")
        write_csv(dayfirst, ["MSISDN", "B_PARTY", "CALL_START", "DURATION"],
                  [[SYNTH_NUMBER, SYNTH_NUMBER, "%02d/04/2026 10:00:00" % day, "5"] for day in (3, 17, 25, 9, 11)])
        t = profile(dayfirst, "local")["sheets"][0]["columns"][2]["time"]
        assert t["day_month_order"] == "day_first" and t["min"] == "2026-04-03 10:00:00" and t["max"] == "2026-04-25 10:00:00"
        monthfirst = os.path.join(d, "c.csv")
        write_csv(monthfirst, ["MSISDN", "B_PARTY", "CALL_START", "DURATION"],
                  [[SYNTH_NUMBER, SYNTH_NUMBER, "4/%d/2026 3:00:00 PM" % day, "5"] for day in (3, 17, 25)])
        t = profile(monthfirst, "local")["sheets"][0]["columns"][2]["time"]
        assert t["day_month_order"] == "month_first" and t["min"] == "2026-04-03 15:00:00" and t["max"] == "2026-04-25 15:00:00"


def test_excel_serial_and_epoch_only_for_time_columns():
    with workdir() as d:
        path = os.path.join(d, "x.csv")
        rows = [[SYNTH_NUMBER, "3" + "451234567", "45123.5", "5"] for _ in range(6)]
        write_csv(path, ["MSISDN", "B_PARTY", "CALL_START", "DURATION"], rows)
        cols = {c["header"]: c for c in profile(path, "local")["sheets"][0]["columns"]}
        assert cols["CALL_START"]["time"]["formats"] == {"excel_serial": 1.0}
        assert "time" not in cols["B_PARTY"]  # a ten-digit number is not an epoch time unless the header says so


def xlsx_bytes(path, sheets):
    """Write a minimal XLSX with the given {name: rows}. Cells are inline strings or numbers."""
    def cell(ref, value):
        if isinstance(value, (int, float)):
            return '<c r="%s"><v>%s</v></c>' % (ref, value)
        return '<c r="%s" t="inlineStr"><is><t>%s</t></is></c>' % (ref, str(value).replace("&", "&amp;").replace("<", "&lt;"))

    ns = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
    rel_ns = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as z:
        z.writestr("[Content_Types].xml", '<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>')
        sheet_tags, rel_tags = [], []
        for i, (name, rows) in enumerate(sheets.items(), start=1):
            sheet_tags.append('<sheet name="%s" sheetId="%d" r:id="rId%d"/>' % (name, i, i))
            rel_tags.append('<Relationship Id="rId%d" Type="worksheet" Target="worksheets/sheet%d.xml"/>' % (i, i))
            body = []
            for r, row in enumerate(rows, start=1):
                cells = "".join(cell("%s%d" % (chr(65 + c), r), v) for c, v in enumerate(row) if v != "")
                body.append('<row r="%d">%s</row>' % (r, cells))
            z.writestr("xl/worksheets/sheet%d.xml" % i, '<worksheet xmlns="%s"><sheetData>%s</sheetData></worksheet>' % (ns, "".join(body)))
        z.writestr("xl/workbook.xml", '<workbook xmlns="%s" xmlns:r="%s"><sheets>%s</sheets></workbook>' % (ns, rel_ns, "".join(sheet_tags)))
        z.writestr("xl/_rels/workbook.xml.rels", '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">%s</Relationships>' % "".join(rel_tags))


def test_profile_xlsx_and_sheet_name_masking():
    with workdir() as d:
        path = os.path.join(d, "apps.xlsx")
        xlsx_bytes(path, {"Sheet1": [HEADER] + synthetic_rows(15), "Other": [["a", "b"]]})
        report = profile(path, "local")
        assert report["file"]["kind"] == "xlsx" and report["file"]["sheet_count"] == 2
        sheet = report["sheets"][0]
        assert sheet["row_count"] == 15 and sheet["name"] == "Sheet1"
        assert PLANTED_TEXT not in json.dumps(profile(path, "shareable"))
        chinese = os.path.join(d, "cn.xlsx")
        xlsx_bytes(chinese, {"应用": [HEADER] + synthetic_rows(6)})
        name = profile(chinese)["sheets"][0]["name"]
        assert name == "<masked:uu>"


def test_profile_spreadsheetml():
    with workdir() as d:
        path = os.path.join(d, "report.xls")
        rows = "".join("<Row>%s</Row>" % "".join('<Cell><Data ss:Type="String">%s</Data></Cell>' % c for c in row)
                       for row in [HEADER] + synthetic_rows(9))
        with open(path, "w", encoding="utf-8") as handle:
            handle.write('<?xml version="1.0"?><Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet" '
                         'xmlns:ss="urn:schemas-microsoft-com:office:spreadsheet"><Worksheet ss:Name="Sheet1"><Table>%s</Table>'
                         "</Worksheet></Workbook>" % rows)
        report = profile(path, "local")
        assert report["file"]["kind"] == "spreadsheetml" and report["sheets"][0]["row_count"] == 9


def test_unsupported_and_hostile_inputs_are_refused():
    with workdir() as d:
        ole = os.path.join(d, "old.xls")
        with open(ole, "wb") as handle:
            handle.write(b"\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1" + b"\x00" * 100)
        rar = os.path.join(d, "x.rar")
        with open(rar, "wb") as handle:
            handle.write(b"Rar!\x1a\x07\x00" + b"\x00" * 50)
        plain_zip = os.path.join(d, "x.zip")
        with zipfile.ZipFile(plain_zip, "w") as z:
            z.writestr("a.txt", "x")
        bomb = os.path.join(d, "bomb.xml")
        with open(bomb, "w", encoding="utf-8") as handle:
            handle.write('<?xml version="1.0"?><!DOCTYPE x [<!ENTITY a "b">]><Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet"/>')
        for path in (ole, rar, plain_zip, bomb):
            try:
                profile(path)
            except ps.Unsupported:
                continue
            raise AssertionError("%s should have been refused" % os.path.basename(path))


def test_distinct_cap_is_reported_as_lower_bound():
    with workdir() as d:
        path = os.path.join(d, "big.csv")
        write_csv(path, ["MSISDN", "B_PARTY", "CALL_START", "DURATION"],
                  [[reserved.synthetic_msisdn(i), reserved.synthetic_msisdn(i), "2026-01-01 00:00:00", "1"] for i in range(300)])
        col = profile(path, "local", max_distinct=100)["sheets"][0]["columns"][0]
        assert col["distinct_is_lower_bound"] is True and col["distinct"] == 100


def test_cli_refuses_scratch_input_unless_overridden():
    with workdir() as d:
        src = os.path.join(d, "calls.csv")
        write_csv(src, HEADER, synthetic_rows(10))
        out = os.path.join(d, "report.json")
        try:
            ps.main([src, "--out", out])  # this folder is under the temporary directory: not a vault
        except SystemExit as exc:
            assert "refusing to use input file" in str(exc)
        else:
            raise AssertionError("an input in a scratch folder must be refused without the override")
        assert not os.path.exists(out)
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            assert ps.main([src, "--out", out, "--allow-unsafe-location"]) == 0
        assert os.path.exists(out) and "wrote report.json" in buf.getvalue() and SYNTH_NUMBER not in buf.getvalue()


# --------------------------------------------------------------------------------------------------------------------------
# scan_for_real_data
# --------------------------------------------------------------------------------------------------------------------------
ALLOW = {"msisdn": set(), "ips": set(), "paths": [], "ignore_paths": []}


def rules_of(path, text, allow=ALLOW):
    return {(f.severity, f.rule) for f in scan.scan_text(path, text, allow)}


def test_scan_msisdn_rules():
    assert ("block", "num.msisdn") in rules_of("a.csv", "x,%s,y" % LOOKS_REAL_MSISDN)
    assert ("block", "num.msisdn") in rules_of("a.md", "call %s now" % LOOKS_REAL_LOCAL)
    assert ("block", "num.msisdn") in rules_of("a.md", "call +92 %s %s" % ("345", "9999999"))
    assert not rules_of("a.csv", "x,%s,y" % SYNTH_NUMBER)
    assert not rules_of("a.csv", "x,%s,y" % ("0" + SYNTH_NUMBER[2:]))
    allow = dict(ALLOW, msisdn={LOOKS_REAL_MSISDN})
    assert not rules_of("a.csv", LOOKS_REAL_MSISDN, allow)
    assert not rules_of("a.csv", "id 1" + LOOKS_REAL_MSISDN + " is longer")  # inside a longer digit run


def test_scan_identifiers():
    assert ("block", "id.imsi_pk") in rules_of("a.json", '{"imsi": "%s"}' % LOOKS_REAL_IMSI)
    assert not rules_of("a.json", '{"imsi": "%s"}' % reserved.synthetic_imsi(1))
    assert ("block", "id.cnic") in rules_of("a.md", "id %s" % LOOKS_REAL_CNIC)
    assert not rules_of("a.md", "id %s" % reserved.synthetic_cnic(3))
    real_imei = "35" + "0000000000000"
    body = real_imei[:14]
    real_imei = body + reserved.luhn_check_digit(body)
    assert ("review", "num.imei_like") in rules_of("a.csv", "imei,%s" % real_imei)
    assert not rules_of("a.csv", "imei,%s" % reserved.synthetic_imei(9))
    assert ("review", "id.cnic_context") in rules_of("a.md", "the cnic is %s" % ("3" + "520212345671"))


def test_scan_ips_and_coordinates():
    assert ("block", "net.public_ip") in rules_of("flows.csv", "src,%s,dst" % PUBLIC_IP)
    assert ("review", "net.public_ip") in rules_of("notes.md", "host %s" % PUBLIC_IP)
    assert not rules_of("flows.csv", "src,%s,dst" % reserved.synthetic_ipv4(5))
    assert not rules_of("flows.csv", "src,10.0.0.5,dst")
    assert not rules_of("notes.md", "resolver 8.8.8.8")
    assert not rules_of("notes.md", "version %s released" % PUBLIC_IP)
    assert ("block", "net.public_ip") in rules_of("flows.csv", "src,2606:4700:4700::1111,dst")  # real-data-ok: public resolver address
    assert not rules_of("flows.csv", "src,%s,dst" % reserved.synthetic_ipv6(1))
    assert not rules_of("flows.csv", "mac aa:bb:cc:dd:ee:ff at 12:30:45")
    assert ("review", "geo.pk_coords") in rules_of("x.csv", "33.6844,73.0479")
    assert not rules_of("x.csv", "33.7,73.0")


def test_scan_secrets_and_pragma():
    key = "-----BEGIN " + "RSA PRIVATE KEY-----"
    aws = "AK" + "IA" + "ABCDEFGHIJKLMNOP"
    token = "gh" + "p_" + "A" * 36
    for text in (key, aws, token, "sk-" + "ant-" + "x" * 30):
        found = rules_of("a.txt", text)
        assert any(sev == "block" and rule.startswith("secret.") for sev, rule in found), text
    assert not rules_of("a.csv", "%s  # real-data-ok" % LOOKS_REAL_MSISDN)


def test_scan_never_echoes_the_value():
    for text in ("a,%s,b" % LOOKS_REAL_MSISDN, "imsi %s" % LOOKS_REAL_IMSI, "id %s" % LOOKS_REAL_CNIC, "ip %s" % PUBLIC_IP):
        for f in scan.scan_text("a.csv", text, ALLOW):
            line = f.format()
            for secret in (LOOKS_REAL_MSISDN, LOOKS_REAL_IMSI, LOOKS_REAL_CNIC, PUBLIC_IP):
                assert secret not in line, line
            assert "a,%s" % LOOKS_REAL_MSISDN not in line


def test_scan_file_names_and_media():
    names = {(f.severity, f.rule) for f in scan.scan_path_name("MSISDN-%s.xlsx" % LOOKS_REAL_MSISDN, ALLOW)}
    assert ("block", "file.real_name") in names and ("review", "file.media_or_archive") in names
    assert not scan.scan_path_name("docs/readme.md", ALLOW)
    assert not scan.scan_path_name("fixtures/media/x.png", dict(ALLOW, paths=["fixtures/"]))
    assert [f.rule for f in scan.scan_path_name("photo.JPG", ALLOW)] == ["file.media_or_archive"]


def test_parse_added_lines():
    diff = "\n".join([
        "diff --git a/x.csv b/x.csv", "index 1..2 100644", "--- a/x.csv", "+++ b/x.csv",
        "@@ -1,0 +5,2 @@", "+first added", "+second added",
        "@@ -9 +20 @@", "-old", "+replaced",
        "diff --git a/gone.csv b/gone.csv", "--- a/gone.csv", "+++ /dev/null", "@@ -1 +0,0 @@", "-removed",
    ])
    assert list(scan.parse_added_lines(diff)) == [("x.csv", 5, "first added"), ("x.csv", 6, "second added"), ("x.csv", 20, "replaced")]


def git(repo, *args):
    subprocess.run(["git", "-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.com"] + list(args),
                   check=True, capture_output=True)


def run_scan(*argv):
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        code = scan.main(list(argv))
    return code, buf.getvalue()


def test_scan_staged_in_a_temporary_repository():
    with workdir() as repo:
        git(repo, "init", "-q")
        with open(os.path.join(repo, "ok.csv"), "w") as handle:
            handle.write("a,b\n%s,%s\n" % (SYNTH_NUMBER, reserved.synthetic_ipv4(1)))
        git(repo, "add", "ok.csv")
        code, out = run_scan("--staged", "--repo", repo)
        assert code == 0 and "0 block" in out, out
        with open(os.path.join(repo, "bad.csv"), "w") as handle:
            handle.write("a,b\n%s,%s\n" % (LOOKS_REAL_MSISDN, PUBLIC_IP))
        git(repo, "add", "bad.csv")
        code, out = run_scan("--staged", "--repo", repo)
        assert code == 1 and "bad.csv:2" in out and LOOKS_REAL_MSISDN not in out and PUBLIC_IP not in out, out
        code, _ = run_scan("--staged", "--repo", repo, "--fail-on", "never")
        assert code == 0
        git(repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "x")
        code, out = run_scan("--staged", "--repo", repo)
        assert code == 0, out  # nothing staged now: existing content never blocks
        code, out = run_scan("--all-tracked", "--repo", repo)
        assert code == 1 and "bad.csv" in out  # audit mode still finds it


def test_scan_sql_casts_are_not_ipv6_addresses():
    for text in ("WHERE evidence_id::text = 'x' AND d::int > 0", "to_regclass('forensic.x'::regclass)", "a::b::c", "id::uuid, e::text",
                 "SELECT 1::int"):
        assert not rules_of("m.sql", text), text


def test_scan_known_values_are_not_new_exposure():
    other = "92" + "346" + "8888888"
    with workdir() as repo:
        git(repo, "init", "-q")
        with open(os.path.join(repo, "old.csv"), "w") as handle:
            handle.write("n,ip\n%s,%s\n" % (LOOKS_REAL_MSISDN, PUBLIC_IP))
        git(repo, "add", "old.csv")
        git(repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "old")
        with open(os.path.join(repo, "new.csv"), "w") as handle:
            # the same number written another way and the same address again, plus one genuinely new number
            handle.write("n,ip\n+%s,%s\n%s,10.1.1.1\n" % (LOOKS_REAL_MSISDN, PUBLIC_IP, other))
        git(repo, "add", "new.csv")
        code, out = run_scan("--staged", "--repo", repo)
        assert code == 1 and "new.csv:3" in out and "new.csv:2" not in out, out
        assert out.count("num.msisdn") == 1 and "net.public_ip" not in out, out
        code, out = run_scan("--staged", "--repo", repo, "--no-known-values")
        assert out.count("num.msisdn") == 2 and "net.public_ip" in out, out
        assert LOOKS_REAL_MSISDN not in out and other not in out and PUBLIC_IP not in out


def test_scan_range_reads_added_lines_only():
    with workdir() as repo:
        git(repo, "init", "-q")
        with open(os.path.join(repo, "a.csv"), "w") as handle:
            handle.write("n\n%s\n" % LOOKS_REAL_MSISDN)
        git(repo, "add", "a.csv")
        git(repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "old")
        with open(os.path.join(repo, "a.csv"), "a") as handle:
            handle.write("%s\n" % SYNTH_NUMBER)
        git(repo, "add", "a.csv")
        git(repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "new")
        code, out = run_scan("--rev-range", "HEAD~1..HEAD", "--repo", repo)
        assert code == 0, out


# --------------------------------------------------------------------------------------------------------------------------
# reconcile_aggregates
# --------------------------------------------------------------------------------------------------------------------------
def test_emit_sql_is_read_only_and_quoted():
    uuid = "34000000-0000-4000-8000-000000000001"
    sql = ra.emit_sql("comm_event", "t'x", "c", uuid, "Asia/Karachi")
    assert sql.count("SELECT") >= 1 and "t''x" in sql and "AT TIME ZONE 'Asia/Karachi'" in sql
    for word in ("INSERT", "UPDATE", "DELETE", "DROP", "TRUNCATE", "ALTER"):
        assert word not in sql.upper().replace("DISTINCT", ""), word
    for bad in ({"evidence_id": "not-a-uuid"}, {"tz": "UTC'; DROP"}):
        args = dict(table="comm_event", tenant="t", collection="c", evidence_id=uuid, tz="UTC")
        args.update(bad)
        try:
            ra.emit_sql(**args)
        except SystemExit:
            continue
        raise AssertionError("bad input accepted: %r" % bad)
    try:
        ra.emit_sql("not_a_table", "t", "c", uuid, "UTC")
    except SystemExit:
        pass
    else:
        raise AssertionError("unknown table accepted")


def reconciliation_inputs(d):
    path = os.path.join(d, "calls.csv")
    write_csv(path, HEADER, synthetic_rows(40))
    profile_path = os.path.join(d, "profile.json")
    assert ps.main([path, "--mode", "local", "--out", profile_path, "--allow-unsafe-location"]) == 0
    with open(profile_path) as handle:
        prof = json.load(handle)
    by_name = {c["header"]: c for c in prof["sheets"][0]["columns"]}
    loaded = {
        "table": "comm_event", "rows": 38,
        "distinct": {"a_raw": by_name["MSISDN"]["distinct"], "b_raw": by_name["B_PARTY"]["distinct"]},
        "nulls": {"a_raw": 0, "b_raw": 0},
        "time_local_min": by_name["CALL_START"]["time"]["min"], "time_local_max": by_name["CALL_START"]["time"]["max"],
        "histograms": {"direction": {"incoming": 26, "outgoing": 14}, "a_kind": {"person_like": 40}},
    }
    mapping = {
        "table": "comm_event",
        "columns": [{"source": "MSISDN", "target": "a_raw"}, {"source": "B_PARTY", "target": "b_raw"}],
        "time": {"source": "CALL_START", "target": "start_utc"},
        "histograms": [{"source": "DIRECTION", "target": "direction", "enum_map": {"Incoming": "incoming", "Outgoing": "outgoing"}}],
        "classes": [{"source": "MSISDN", "target": "a_kind", "class_map": {"92_prefix_12_digits": "person_like"}}],
    }
    return prof, loaded, mapping


def test_reconcile_passes_and_fails_on_aggregates():
    with workdir() as d:
        prof, loaded, mapping = reconciliation_inputs(d)
        good = ra.compare(prof, loaded, mapping, quarantined=2, duplicates=0)
        assert good.failed == 0, good.text()
        assert "RECONCILE=PASS" in good.text()
        assert SYNTH_NUMBER not in good.text() and PLANTED_TEXT not in good.text()
        short = ra.compare(prof, loaded, mapping, quarantined=1, duplicates=0)
        assert short.failed == 1 and "RECONCILE=FAIL" in short.text()  # one row unaccounted for
        broken = json.loads(json.dumps(loaded))
        broken["distinct"]["a_raw"] += 1
        broken["nulls"]["b_raw"] = 1
        broken["histograms"]["direction"]["incoming"] = 25
        broken["time_local_max"] = "2030-01-01 00:00:00"
        worse = ra.compare(prof, broken, mapping, quarantined=2, duplicates=0)
        assert worse.failed >= 4, worse.text()
        try:
            ra.compare(dict(prof, mode="shareable"), loaded, mapping, 2, 0)
        except SystemExit:
            pass
        else:
            raise AssertionError("a shareable profile must not be accepted for reconciliation")


def run_all():
    names = sorted(n for n in globals() if n.startswith("test_"))
    failed = 0
    for name in names:
        try:
            globals()[name]()
        except Exception:  # noqa: BLE001 - report every failure, then exit non-zero
            failed += 1
            print("FAIL  %s" % name)
            traceback.print_exc()
        else:
            print("ok    %s" % name)
    print("%d tests, %d failed" % (len(names), failed))
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(run_all())
