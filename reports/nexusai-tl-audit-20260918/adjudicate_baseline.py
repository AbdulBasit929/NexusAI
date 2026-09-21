"""Apply the 2026-09-18 manual adjudication to the automated baseline grading.

Every raw response under baseline/raw/ was read by hand; the automated grader's
verdict is kept alongside for transparency. Categories:
  CORRECT    right answer in the requested shape
  PARTIAL    right data present, but not the asked shape/total
  SAFE_FAIL  clarified or declined honestly (capability gap, not a wrong answer)
  WRONG      confidently wrong, wrong-shaped, or false negative
  ERROR      HTTP 500
"""
import collections
import json
import os

BASE = os.path.dirname(os.path.abspath(__file__))

MANUAL = {
    "CDR-01": ("WRONG", "spurious GROUP BY manual_review_required (embedding-selected); nondeterministic - an earlier identical call returned 8642"),
    "CDR-02": ("WRONG", "total question answered with type x direction breakdown; no total"),
    "CDR-03": ("WRONG", "counted ALL families (12,912 rows) grouped by source_file"),
    "CDR-04": ("PARTIAL", "breakdown is type x direction; per-type totals (GPRS 5,863) not given"),
    "CDR-05": ("PARTIAL", "same as CDR-04"),
    "CDR-06": ("SAFE_FAIL", "no case-wide top-talker operation; asks for a target"),
    "CDR-07": ("SAFE_FAIL", "no distinct-counterparty ranking; asks for a target"),
    "CDR-08": ("SAFE_FAIL", "distinct count misrouted to frequent_contacts clarification"),
    "CDR-09": ("ERROR", "HTTP 500: projection SQL argument mismatch"),
    "CDR-10": ("CORRECT", "ranking correct; 425 self-call rows silently excluded without a data-quality note"),
    "CDR-11": ("WRONG", "target filter dropped; counted all 12,912 rows grouped by called_number_role"),
    "CDR-12": ("SAFE_FAIL", "no case-wide cell ranking; asks for a target"),
    "CDR-13": ("CORRECT", "values correct; scope scanned all families"),
    "CDR-14": ("WRONG", "August date filter dropped; same all-rows answer as CDR-11"),
    "CDR-15": ("WRONG", "longest_call returned per-day duration totals, not the longest call"),
    "CDR-16": ("WRONG", "false negative: reported no matching records"),
    "IPDR-01": ("CORRECT", ""),
    "IPDR-02": ("WRONG", "IPDR domain question routed to CDR top_locations (cell towers)"),
    "IPDR-03": ("CORRECT", ""),
    "IPDR-04": ("PARTIAL", "hourly byte totals only; grand total not computed"),
    "IPDR-05": ("WRONG", "entity_activity over CDR phones, not IPDR subscribers"),
    "ANPR-01": ("CORRECT", ""),
    "ANPR-02": ("WRONG", "20-row listing instead of the count 87"),
    "ANPR-03": ("CORRECT", "first/last seen and 87 observations present"),
    "ANPR-04": ("WRONG", "raw sightings listing, no camera ranking"),
    "ANPR-05": ("PARTIAL", "distinct plates per camera, not case-wide (coincidentally 6)"),
    "ANPR-06": ("WRONG", "raw sightings listing, no plate ranking"),
    "ACC-01": ("CORRECT", ""),
    "ACC-02": ("WRONG", "routed to collection_overview"),
    "ACC-03": ("WRONG", "entity_activity over CDR phones"),
    "ACC-04": ("WRONG", "routed to ingest data_quality parser errors"),
    "SUB-01": ("PARTIAL", "status breakdown sums to 11 but the total is not stated"),
    "SUB-02": ("PARTIAL", "ACTIVE rows 4 + 2 = 6 (correct: one row uses source column account_status); total not stated. P0 oracle error corrected"),
    "SUB-03": ("CORRECT", "CNIC correctly masked by privacy policy (oracle expected unmasked)"),
    "TWR-01": ("WRONG", "routed to CDR top_locations"),
    "TWR-02": ("WRONG", "returned schema field-role profile, not tower coordinates"),
    "TXN-01": ("PARTIAL", "per-status totals (98,700 + 31,000); grand total not stated"),
    "TXN-02": ("WRONG", "returned COUNT=4 instead of the max amount"),
    "TXN-03": ("PARTIAL", "ranked by amount, not transaction count; top account coincides"),
    "CASE-01": ("WRONG", "schema-detection output instead of grouped counts (known)"),
    "CASE-02": ("PARTIAL", "file-level ingest table, no analyst-level overview"),
    "CASE-03": ("ERROR", "HTTP 500: projection SQL argument mismatch"),
    "CASE-04": ("WRONG", "'explain' classified as general-domain definition; no case data"),
    "NEG-01": ("WRONG", "nonexistent number -> confident counts of all rows"),
    "NEG-02": ("CORRECT", ""),
    "NEG-03": ("CORRECT", "declined, but the analyst answer text is empty"),
    "NEG-04": ("WRONG", "'emails' -> confident counts of all records"),
    "DOC-01": ("CORRECT", ""),
    "DOC-02": ("CORRECT", ""),
    "DOC-03": ("CORRECT", ""),
    "DOC-04": ("WRONG", "returned the PDF passage, not the case-notes passage; 82s (Role A generated 79s then rejected)"),
    "DOC-05": ("WRONG", "routed to access_failed_events"),
    "DOC-06": ("CORRECT", ""),
    "IMG-01": ("CORRECT", ""),
    "IMG-02": ("WRONG", "false negative via cross_family_correlation"),
    "IMG-03": ("ERROR", "HTTP 500: projection SQL argument mismatch"),
    "IMG-04": ("WRONG", "counted structured records, not images"),
    "AUD-01": ("CORRECT", "answer text is only a generic incompleteness disclaimer"),
    "AUD-02": ("WRONG", "returned the 0.0s segment, not the 11.28s coconut-sugar segment"),
    "AUD-03": ("WRONG", "returned the unrelated Japanese-cuisine segment as a match; real match is urdu-english-identifier.wav @2.84s"),
    "VID-01": ("SAFE_FAIL", "asks for a target; no case-wide video plate listing"),
    "X-01": ("WRONG", "false negative via top_locations; number is in the PDF and an audio file"),
}
FALSE_NEGATIVE = {"CDR-16", "IMG-02", "X-01"}


def main():
    res = json.load(open(os.path.join(BASE, "baseline", "results.json"), encoding="utf-8"))
    for r in res:
        verdict, note = MANUAL[r["id"]]
        r["manual_verdict"], r["manual_note"] = verdict, note
        r["false_negative"] = r["id"] in FALSE_NEGATIVE
        r["confident_wrong"] = verdict == "WRONG" and not r["false_negative"]
    json.dump(res, open(os.path.join(BASE, "baseline", "results_adjudicated.json"), "w", encoding="utf-8"), ensure_ascii=False, indent=1)

    total = collections.Counter(r["manual_verdict"] for r in res)
    fam = collections.defaultdict(collections.Counter)
    for r in res:
        fam[r["family"]][r["manual_verdict"]] += 1
    lat = sorted(r["latency_ms"] for r in res)
    answered = [r for r in res if r["manual_verdict"] in ("CORRECT", "PARTIAL") and r["check"] in ("number", "top", "contains")]
    out = {
        "n": len(res), "verdicts": dict(total),
        "confident_wrong": sum(r["confident_wrong"] for r in res),
        "false_negative": sum(r["false_negative"] for r in res),
        "latency_ms": {"p50": lat[len(lat) // 2], "p95": lat[int(len(lat) * 0.95)], "max": lat[-1]},
        "answer_stated_in_text": [sum(1 for r in answered if r["stated_in_answer"]), len(answered)],
        "by_family": {k: dict(v) for k, v in sorted(fam.items())},
    }
    json.dump(out, open(os.path.join(BASE, "baseline", "scorecard.json"), "w", encoding="utf-8"), indent=1)
    print(json.dumps(out, indent=1))


if __name__ == "__main__":
    main()
