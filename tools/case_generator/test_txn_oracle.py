"""Gates G1, G3 and the database part of G5 of docs/data/transactions-family-20261028.md on a THROWAWAY PostgreSQL: generated cases WITH two bank statements and a wallet list are written by the PRODUCTION
typed ingest, and the five views of migration 030 equal the generator's oracle. No model, no real data.

    python -u -X utf8 tools/case_generator/test_txn_oracle.py                 # development case 2, the quick check
    python -u -X utf8 tools/case_generator/test_txn_oracle.py --cases 1,2,3   # chosen development cases
    python -u -X utf8 tools/case_generator/test_txn_oracle.py --all           # all eight development cases

What it proves, through the read-only analyst role and the settings the application sets:
  * the migrations: 029 (the table, applied with the others before layouts are registered) is verified and applied again; 030 applies, verifies, applies again, rolls back and applies again; the
    rollback of 029 refuses while a layout of its kind is registered and, once none is, drops the table and restores the old constraint; both are then applied again and the layouts registered again;
  * G1: the three exports are read as bank-stmt-t1 and wallet-t1 into bank_transaction, rows loaded equal the oracle's unique transactions plus the planted repeats, the quarantined rows are the planted ones,
    the flags are the planted ones and no other (duplicate_row, balance_break on the one row after the row left out, unmapped_label:channel, date_only on every row of the dated statement), a second ingest
    writes nothing;
  * G3: v_bank_transactions equals the oracle row for row (account, time, precision, direction, amount, balance, counterparty, channel, reference), the one balance break is the planted one, the totals and the
    daily activity of every account equal it, every counterparty of every account (count and total) equals it, the counterparties two accounts share equal it, the pass-through pairs equal it at THREE settings of
    the share and the window, the debits under a limit equal it at three settings of the limit and the share, the dated statement never appears in the flows, another collection or no scope shows nothing, the
    role cannot read the table, and no view has a column that could hold a narration or a name.
"""
import argparse
import collections
import json
import os
import sys
import tempfile
from decimal import Decimal

import case_set
import generate
import load_case
from ingestion.forensic_records.layout_registry import load_registry
from test_colocation_views import batch
from test_ingest_v2 import PsqlSink
from test_subscriber_oracle import ingest_all
from txn_case import LIMIT, Oracle

MIG = "/tmp/mig/"
MIGRATIONS = ("016_effective_records.sql", "018_effective_ipdr.sql", "017_analyst_views.sql", "019_observed_cell_positions.sql", "020_contact_links_and_inventory.sql", "021_case_record.sql",
              "024_colocation_and_meetings.sql", "025_successor_candidates.sql", "026_movement_and_rhythm.sql", "027_plate_sightings.sql", "028_subscriber_device_views.sql")
LOCAL = "'YYYY-MM-DD HH24:MI:SS'"
ZONE = "Asia/Karachi"
VIEWS = ("v_bank_transactions", "v_account_counterparties", "v_account_activity", "v_account_flows", "v_account_limit_bands")
TXNS = ("SELECT account, to_char(txn_utc AT TIME ZONE '%s', %s) AS local, time_precision, direction, amount::text AS amount, balance_after::text AS balance, "
        "coalesce(counterparty_account, counterparty_msisdn) AS counterparty, channel, reference, balance_break FROM analyst.v_bank_transactions" % (ZONE, LOCAL))
FLOWS = ("SELECT account, to_char(credit_utc AT TIME ZONE '%s', %s) AS credit_local, credit_amount::text AS credit_amount, to_char(debit_utc AT TIME ZONE '%s', %s) AS debit_local, "
         "debit_amount::text AS debit_amount FROM analyst.v_account_flows ORDER BY 1, 2, 4" % (ZONE, LOCAL, ZONE, LOCAL))
BANDS = "SELECT account, transactions, total_amount::text AS total FROM analyst.v_account_limit_bands ORDER BY 1"


def k(value):
    return "" if value is None else str(value)


def rehearse_migrations(db, registry):
    db.psql("-f", MIG + "ops/029_bank_transactions/preflight.sql")
    db.psql("-f", MIG + "ops/029_bank_transactions/verify.sql")
    db.psql("-f", MIG + "029_bank_transactions.sql")
    db.psql("-f", MIG + "ops/029_bank_transactions/verify.sql")
    db.psql("-f", MIG + "ops/030_transaction_views/preflight.sql")
    db.psql("-f", MIG + "030_transaction_views.sql")
    db.psql("-f", MIG + "ops/030_transaction_views/verify.sql")
    db.psql("-f", MIG + "030_transaction_views.sql")
    db.psql("-f", MIG + "ops/030_transaction_views/verify.sql")
    db.psql("-f", MIG + "ops/030_transaction_views/rollback.sql")
    assert db.sql("SELECT to_regclass('analyst.v_bank_transactions') IS NULL AND to_regclass('analyst.v_account_flows') IS NULL") == "t", "the rollback of 030 left a view"
    # 029 cannot be rolled back while a layout of its kind is registered, and then changes nothing
    try:
        db.psql("-f", MIG + "ops/029_bank_transactions/rollback.sql")
        raise AssertionError("the rollback of 029 ran with a layout of its kind registered")
    except RuntimeError as error:
        assert "a layout of kind bank_transaction is registered" in str(error), str(error)[:200]
    assert db.sql("SELECT to_regclass('forensic.bank_transaction') IS NOT NULL") == "t", "the refused rollback changed something"
    db.sql("DELETE FROM forensic.layout_versions WHERE dataset_kind = 'bank_transaction'")
    db.psql("-f", MIG + "ops/029_bank_transactions/rollback.sql")
    assert db.sql("SELECT to_regclass('forensic.bank_transaction') IS NULL") == "t", "the rollback of 029 left the table"
    assert "bank_transaction" not in db.sql("SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = 'layout_versions_kind_chk'"), "the old constraint was not restored"
    db.psql("-f", MIG + "029_bank_transactions.sql")
    db.script(registry.register_sql())
    db.psql("-f", MIG + "ops/029_bank_transactions/verify.sql")
    db.psql("-f", MIG + "030_transaction_views.sql")
    db.psql("-f", MIG + "ops/030_transaction_views/verify.sql")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--cases", default="2")
    parser.add_argument("--all", action="store_true")
    args = parser.parse_args()
    wanted = list(range(1, 9)) if args.all else [int(n) for n in args.cases.split(",")]
    table = {seed: overrides for kind, seed, overrides in case_set.CASES if kind == "development"}
    registry = load_registry()
    problems, totals = [], collections.Counter()
    with tempfile.TemporaryDirectory() as tmp:
        db = load_case.Throwaway()
        try:
            db.start()
            load_case.prepare(db, registry)
            for name in MIGRATIONS:
                db.psql("-f", MIG + name)
            rehearse_migrations(db, registry)
            print("migrations 029 and 030: verify, apply twice, rollback, refused and then allowed rollback of 029, apply again: OK", flush=True)
            sink = PsqlSink(db)
            for seed in wanted:
                params = case_set.params_for(seed, table[seed])
                root = generate.write_case(params, tmp, with_transactions=True)
                key, summaries = ingest_all(db, root, registry, sink)
                truth = key["transaction_truth"]
                oracle = Oracle(truth)
                planted = truth["planted"]
                label = "case %d" % seed
                # G1
                unique = collections.Counter(r["file"] for r in truth["transactions"])
                for item in key["transactions"]:
                    sheet = summaries[item["name"]]["sheets"][0]
                    source = item["source"]
                    if sheet["layout_id"] != item["layout_id"] or sheet["table"] != "bank_transaction":
                        problems.append("%s %s: read as %s into %s" % (label, item["name"], sheet["layout_id"], sheet["table"]))
                    repeats = planted["duplicate_rows"] if source == "A" else 0
                    if sheet["loaded"] != unique[source] + repeats:
                        problems.append("%s %s: %d rows loaded, the truth is %d unique + %d repeats" % (label, item["name"], sheet["loaded"], unique[source], repeats))
                    quarantined = {x.split(":", 1)[1]: v for x, v in sheet["quarantined"].items()}
                    wanted_q = planted["quarantined"] if source == "A" else {}
                    if quarantined != wanted_q:
                        problems.append("%s %s: quarantined %s, planted %s" % (label, item["name"], quarantined, wanted_q))
                    expected = {}
                    if source == "A":
                        expected = {"duplicate_row": repeats, "balance_break": len(truth["plants"]["break_references"])}
                        if planted["unmapped_channel"]:
                            expected["unmapped_label:channel"] = planted["unmapped_channel"]
                        expected = {f: n for f, n in expected.items() if n}
                    elif source == "B":
                        expected = {"date_only": planted["date_only_rows"]}
                    if sheet["flags"] != expected:
                        problems.append("%s %s: flags %s, expected %s" % (label, item["name"], sheet["flags"], expected))
                    totals["rows"] += sheet["loaded"]
                again = ingest_all(db, root, registry, sink, register=False)[1]
                for item in key["transactions"]:
                    if again[item["name"]]["rows_inserted"] != 0:
                        problems.append("%s: a second ingest of %s inserted %d rows" % (label, item["name"], again[item["name"]]["rows_inserted"]))
                # G3
                scope = {"TimeZone": ZONE, "app.tenant_id": load_case.TENANT, "app.collection_id": key["case_id"]}
                accounts = oracle.accounts()
                got = batch(db, scope, {"txns": TXNS,
                                        "totals": "SELECT account, sum(credits)::text AS credits, sum(debits)::text AS debits, sum(transactions)::text AS n FROM analyst.v_account_activity GROUP BY account ORDER BY 1",
                                        "cps": "SELECT account, counterparty, direction, transactions, total_amount::text AS total FROM analyst.v_account_counterparties"})
                have = sorted((r["account"], r["local"], r["time_precision"], r["direction"], r["amount"], r["balance"], k(r["counterparty"]), k(r["channel"]), k(r["reference"])) for r in got["txns"])
                want = sorted((r["account"], r["local"], r["precision"], r["direction"], str(r["amount"]), str(r["balance_after"]), k(r["counterparty"]), k(r["channel"]), k(r["reference"])) for r in oracle.rows)
                if have != want:
                    problems.append("%s: v_bank_transactions differs from the truth: %d rows against %d (extra %s, missing %s)" % (label, len(have), len(want), sorted(set(have) - set(want))[:2], sorted(set(want) - set(have))[:2]))
                breaks = [r for r in got["txns"] if r["balance_break"]]
                wanted_breaks = [r["reference"] for r in oracle.rows if r["key"] in truth["plants"]["break_references"]]
                if sorted(k(r["reference"]) for r in breaks) != sorted(k(x) for x in wanted_breaks):
                    problems.append("%s: the balance breaks are %s, the planted one is %s" % (label, [r["reference"] for r in breaks], wanted_breaks))
                day_totals = {r["account"]: (Decimal(r["credits"]), Decimal(r["debits"]), int(r["n"])) for r in got["totals"]}
                for account in accounts:
                    if day_totals.get(account) != oracle.totals(account):
                        problems.append("%s: the totals of %s are %s, the oracle says %s" % (label, account[-6:], day_totals.get(account), oracle.totals(account)))
                    totals["accounts"] += 1
                by_cp = collections.defaultdict(dict)
                for r in got["cps"]:
                    by_cp[r["account"]][(r["counterparty"], r["direction"])] = (r["transactions"], Decimal(r["total"]))
                for account in accounts:
                    if by_cp.get(account, {}) != oracle.counterparties(account):
                        problems.append("%s: the counterparties of %s differ from the oracle" % (label, account[-6:]))
                for pair in truth["plants"]["pairs"]:
                    shared = batch(db, scope, {"s": "SELECT counterparty FROM analyst.v_account_counterparties WHERE account = %s INTERSECT SELECT counterparty FROM analyst.v_account_counterparties WHERE account = %s ORDER BY 1"
                                                    % (load_case.sql_text(pair["x"]), load_case.sql_text(pair["y"]))})["s"]
                    if [r["counterparty"] for r in shared] != oracle.shared(pair["x"], pair["y"]) or pair["third"] not in oracle.shared(pair["x"], pair["y"]):
                        problems.append("%s: the counterparties shared by %s and %s differ from the oracle" % (label, pair["x"][-4:], pair["y"][-4:]))
                    totals["shared"] += 1
                # the daily activity of an account: the totals of its days equal the oracle's day by day, and the closing balance is the balance after the last row of the day
                sample = [a for a in accounts if len(a) == 24][:3]
                days = {a: sorted({r["local"][:10] for r in oracle.of(a)})[:6] for a in sample}
                activity = batch(db, scope, {"%s|%s" % (a, d): "SELECT transactions, credits::text AS credits, debits::text AS debits, closing_balance::text AS closing FROM analyst.v_account_activity WHERE account = %s AND day = '%s'"
                                                               % (load_case.sql_text(a), d) for a in sample for d in days[a]})
                for name, rows in activity.items():
                    account, day = name.split("|")
                    wanted_a = oracle.activity(account, day)
                    shown = (rows[0]["transactions"], Decimal(rows[0]["credits"]), Decimal(rows[0]["debits"]), Decimal(rows[0]["closing"])) if rows else None
                    if shown != wanted_a:
                        problems.append("%s: the activity of %s on %s is %s, the oracle says %s" % (label, account[-6:], day, shown, wanted_a))
                    totals["days"] += 1
                # the pass-through pairs at three settings, and the debits under a limit at three settings
                for share, window in ((90, 10), (90, 80), (50, 10)):
                    flows = batch(db, dict(scope, **{"app.flow_window_minutes": str(window), "app.flow_share_percent": str(share)}), {"f": FLOWS})["f"]
                    have_f = sorted((r["account"], r["credit_local"], r["credit_amount"], r["debit_local"], r["debit_amount"]) for r in flows)
                    if have_f != oracle.flows(share, window):
                        problems.append("%s: the pass-through pairs at %d percent within %d minutes are %d, the oracle says %d" % (label, share, window, len(have_f), len(oracle.flows(share, window))))
                    if any(r["account"] in truth["plants"]["dated_accounts"] for r in flows):
                        problems.append("%s: a row that states only a date is in the flows" % label)
                    totals["flow_settings"] += 1
                if len(oracle.flows(90, 10)) != 9 or len(oracle.flows(90, 80)) <= len(oracle.flows(90, 10)) or len(oracle.flows(50, 10)) <= len(oracle.flows(90, 10)):
                    problems.append("%s: the plants of the pass-through are not what the look-alikes are meant to be: %d, %d, %d pairs" % (label, len(oracle.flows(90, 10)), len(oracle.flows(90, 80)), len(oracle.flows(50, 10))))
                for limit, share in ((LIMIT, 95), (LIMIT, 60), (Decimal("400000"), 95)):
                    bands = batch(db, dict(scope, **{"app.limit_amount": str(limit), "app.limit_share_percent": str(share)}), {"b": BANDS})["b"]
                    have_b = {r["account"]: (r["transactions"], Decimal(r["total"])) for r in bands}
                    if have_b != oracle.near_limit(limit, share):
                        problems.append("%s: the debits between %d percent of %s and it are %s, the oracle says %s" % (label, share, limit, have_b, oracle.near_limit(limit, share)))
                    totals["band_settings"] += 1
                if batch(db, scope, {"b": BANDS})["b"]:
                    problems.append("%s: the limit view has rows with no limit set" % label)
                # the largest debit of an account, scope, the role, and what a view may hold
                account = accounts[0]
                largest = batch(db, scope, {"m": "SELECT max(amount)::text AS m FROM analyst.v_bank_transactions WHERE account = %s AND direction = 'debit'" % load_case.sql_text(account)})["m"][0]["m"]
                if Decimal(largest) != oracle.largest(account, "debit")[0]:
                    problems.append("%s: the largest debit of an account differs from the oracle" % label)
                nobody = batch(db, dict(scope, **{"app.collection_id": "nobody"}), {v: "SELECT count(*) AS n FROM analyst.%s" % v for v in VIEWS})
                unset = batch(db, {"TimeZone": ZONE}, {v: "SELECT count(*) AS n FROM analyst.%s" % v for v in VIEWS})
                if any(x[0]["n"] for x in list(nobody.values()) + list(unset.values())):
                    problems.append("%s: scope fails open" % label)
                print("%s: %d transactions loaded (%d unique), %d accounts, %d pass-through pairs: checked" % (label, sum(summaries[i["name"]]["sheets"][0]["loaded"] for i in key["transactions"]),
                                                                                                                  len(oracle.rows), len(accounts), len(oracle.flows())), flush=True)
            try:
                db.psql("-At", "-c", "SET ROLE forensic_analyst_ro; SELECT count(*) FROM forensic.bank_transaction")
                problems.append("the analyst role can read forensic.bank_transaction")
            except RuntimeError as error:
                if "permission denied" not in str(error):
                    problems.append("forensic.bank_transaction: unexpected error %s" % str(error)[:120])
            exposed = db.sql("SELECT string_agg(table_name || '.' || column_name, ', ') FROM information_schema.columns WHERE table_schema = 'analyst' AND table_name IN ('%s') "
                             "AND column_name IN ('narration', 'counterparty_label', 'raw_payload', 'counterparty_name')" % "', '".join(VIEWS))
            if exposed:
                problems.append("a view exposes %s" % exposed)
        finally:
            left = db.stop()
            if left:
                print("WARNING: " + left)
    print("checked: %d transaction rows, %d accounts, %d shared-counterparty pairs, %d days, %d flow settings, %d limit settings" %
          (totals["rows"], totals["accounts"], totals["shared"], totals["days"], totals["flow_settings"], totals["band_settings"]))
    if problems:
        print("TXNORACLE=FAIL")
        for line in problems[:30]:
            print("  " + line)
        return 1
    print("TXNORACLE=PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
