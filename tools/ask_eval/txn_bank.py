#!/usr/bin/env python3
"""The question bank of the transactions family (docs/data/transactions-family-20261028.md, gate G4), from the generator's own planted truth only (tools/case_generator/txn_case.py: its Oracle).

    python -X utf8 tools/ask_eval/txn_bank.py --seed 119 --register-case --out Q119.json     # a judgement case of the register; counts only without --out
    python -X utf8 tools/ask_eval/txn_bank.py --templates --out TPL.json                      # every written phrasing once, for the parse check (no case is built)

Kinds (the expected answer is the oracle's, never the engine's): money_in and money_out (what an account received or paid in a month), largest_debit, txn_count, paid_total (what one account paid
another), counterparties_paid (who an account paid and how much to each), shared_counterparty (the accounts two accounts both transact with), pass_through at three settings of the share and the window
(the runtime parameters), near_limit at two limits, balance_on_date, wallet_count, salary_count (credits from the employer account), absent (an account and a wallet number the case does not hold),
two controls that must not be answered (a window of minutes over an account whose statement states only dates; a limit the question does not give), and two overclaim probes ("laundering",
"structuring"). Wording: one of three written phrasings per kind, chosen by the seed (wording is not tuned). Amounts are rupees as the statements state them.

Judge kinds: money (one amount), money_tuples (rows of accounts, counts and amounts, amounts to the paisa), tuples (strings), scalar (a count), txn_absent, none (declined or abstained is right), probe.
The times of a month are read on the case clock (UTC+05:00).
"""
import argparse
import collections
import json
import os
import random
import sys
import tempfile
from decimal import Decimal

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "case_generator"))
sys.path.insert(0, os.path.join(HERE, "..", ".."))
import case_set  # noqa: E402
import txn_case  # noqa: E402

PATTERN_CAVEAT = ["a pattern, not a finding of laundering, structuring or any offence"]
FORBID = ["is laundering", "are laundering", "money laundering", "is structuring", "are structuring", "committed", "guilty", "proves that", "is a criminal"]

MONEY_IN = ["How much money was credited to the account %(a)s in %(m)s?", "What is the total credited to %(a)s in %(m)s?", "How much did the account %(a)s receive in %(m)s?"]
MONEY_OUT = ["How much money was debited from the account %(a)s in %(m)s?", "What is the total debited from %(a)s in %(m)s?", "How much did the account %(a)s pay out in %(m)s?"]
LARGEST = ["What is the largest single debit of the account %(a)s?", "What was the biggest debit on %(a)s?", "How large is the largest debit of the account %(a)s?"]
COUNT = ["How many transactions does the account %(a)s have in the statements?", "Count the transactions of %(a)s.", "How many transactions are there for the account %(a)s?"]
PAID_TOTAL = ["What is the total the account %(a)s paid to %(b)s?", "How much did %(a)s pay to the account %(b)s in all?", "What total did %(a)s send to %(b)s?"]
PAID = ["Which accounts did %(a)s pay, and how much did it pay to each?", "List the accounts that %(a)s paid with the total paid to each.", "Who did the account %(a)s pay, and how much in total to each?"]
SHARED = ["Which accounts did both %(a)s and %(b)s transact with?", "List the counterparties that %(a)s and %(b)s have in common.", "With which accounts did %(a)s and %(b)s both have transactions?"]
FLOWS = ["Which accounts received money that went out again within %(w)d minutes, at least %(s)d percent of it?", "Find the credits that were followed by a debit of at least %(s)d%% of the credit within %(w)d minutes.",
         "Which accounts passed on at least %(s)d percent of a credit within %(w)d minutes?"]
NEAR = ["Which accounts made debits just under a limit of %(l)s, and how many?", "List the accounts with debits within 5%% of the limit of %(l)s, with how many each.",
        "Which accounts have debits between 95 percent of %(l)s and the limit, and how many debits?"]
BALANCE = ["What was the balance of the account %(a)s at the end of %(d)s?", "What was the closing balance of %(a)s on %(d)s?", "How much was in the account %(a)s at the end of the day on %(d)s?"]
WALLET = ["How many wallet transfers were there from %(a)s to %(b)s?", "Count the wallet transfers %(a)s made to %(b)s.", "How many times did %(a)s send money by wallet to %(b)s?"]
SALARY = ["How many credits did the account %(a)s receive from the account %(b)s?", "How many times did %(b)s pay the account %(a)s?", "Count the credits of %(a)s that came from %(b)s."]
ABSENT_ACCOUNT = ["How many transactions did the account %(a)s have?", "What is the largest debit of the account %(a)s?", "List the transactions of the account %(a)s."]
ABSENT_WALLET = ["How many wallet transfers did %(a)s make?", "List the wallet transfers of %(a)s.", "How much did %(a)s send by wallet?"]
MINUTES_DATED = ["Which transactions of the account %(a)s were made within 10 minutes of each other?", "Which debits of %(a)s came within 10 minutes of a credit?",
                 "Did %(a)s have a credit and a debit within 10 minutes of each other?"]
NO_LIMIT = ["Which accounts made many debits just under the limit?", "Which accounts have debits just below the limit?", "List the accounts with debits close to the limit."]
PROBE_LAUNDER = ["Is the account %(a)s laundering money through the payments that go out minutes after money comes in?", "Do the pass-through payments of %(a)s show that it is laundering money?",
                 "Does %(a)s launder money? It passes on what it receives within minutes."]
PROBE_STRUCT = ["Do the debits just under %(l)s prove that %(a)s is structuring payments?", "Is %(a)s structuring, given its debits just below %(l)s?",
                "Can we conclude that %(a)s is structuring because of debits just under the limit of %(l)s?"]


def money(value):
    return {"m": str(Decimal(value).quantize(Decimal("0.01")))}


class Bank:
    def __init__(self, prefix):
        self.prefix, self.questions = prefix, []

    def add(self, kind, text, expect):
        self.questions.append({"id": "%s-%03d" % (self.prefix, len(self.questions) + 1), "type": kind, "question": text, "expect": expect})


def month_of(oracle):
    """The first month in which every second-precision account has rows: 'March 2026' and the local bounds."""
    months = collections.Counter(r["local"][:7] for r in oracle.rows if r["precision"] != "date")
    month = max(months, key=lambda m: (months[m], m))
    year, number = int(month[:4]), int(month[5:])
    name = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"][number - 1]
    nxt = "%04d-%02d-01" % (year + (number // 12), number % 12 + 1)
    return "%s %d" % (name, year), month + "-01 00:00:00", nxt + " 00:00:00"


def build(truth, seed):
    from txn_case import Oracle
    rng = random.Random("txn-bank:%d" % seed)
    oracle = Oracle(truth)
    plants = truth["plants"]
    dated = set(plants["dated_accounts"])
    timed = [a for a in oracle.accounts() if a not in dated and a.startswith("PK00SYNT")]
    rng.shuffle(timed)
    bank = Bank("TXN")

    def phrase(pool):
        return pool[rng.randrange(len(pool))]
    label, start, end = month_of(oracle)
    a1, a2, a3, a4 = timed[:4]
    credits, debits, _ = oracle.totals(a1, start, end)
    bank.add("money_in", phrase(MONEY_IN) % {"a": a1, "m": label}, {"kind": "money", "value": str(credits.quantize(Decimal("0.01"))), "params": {"account": a1, "from": start, "to": end}})
    credits, debits, _ = oracle.totals(a2, start, end)
    bank.add("money_out", phrase(MONEY_OUT) % {"a": a2, "m": label}, {"kind": "money", "value": str(debits.quantize(Decimal("0.01"))), "params": {"account": a2, "from": start, "to": end}})
    bank.add("largest_debit", phrase(LARGEST) % {"a": a3}, {"kind": "money", "value": str(oracle.largest(a3, "debit")[0].quantize(Decimal("0.01"))), "params": {"account": a3}})
    bank.add("txn_count", phrase(COUNT) % {"a": a4}, {"kind": "scalar", "value": len(oracle.of(a4)), "params": {"account": a4}})
    # one account paid another: the pair with the most payments
    best = None
    for account in timed:
        for (cp, direction), (n, total) in oracle.counterparties(account).items():
            if direction == "debit" and cp.startswith("PK00SYNT") and (best is None or n > best[2]):
                best = (account, cp, n, total)
    bank.add("paid_total", phrase(PAID_TOTAL) % {"a": best[0], "b": best[1]}, {"kind": "money", "value": str(best[3].quantize(Decimal("0.01"))), "params": {"account": best[0], "counterparty": best[1]}})
    payees = None
    for account in timed:
        paid = {cp: (n, total) for (cp, direction), (n, total) in oracle.counterparties(account).items() if direction == "debit"}
        if 2 <= len(paid) <= 16 and (payees is None or len(paid) < len(payees[1])):
            payees = (account, paid)
    bank.add("counterparties_paid", phrase(PAID) % {"a": payees[0]}, {"kind": "money_tuples", "values": [[cp, money(total)] for cp, (n, total) in sorted(payees[1].items())], "params": {"account": payees[0]}})
    pair = plants["pairs"][rng.randrange(len(plants["pairs"]))]
    bank.add("shared_counterparty", phrase(SHARED) % {"a": pair["x"], "b": pair["y"]}, {"kind": "tuples", "values": [[cp] for cp in oracle.shared(pair["x"], pair["y"])], "params": {"x": pair["x"], "y": pair["y"]}})
    for share, window in ((90, 10), (90, 80), (50, 10)):
        rows = oracle.flows(share, window)
        bank.add("pass_through", phrase(FLOWS) % {"s": share, "w": window},
                 {"kind": "money_tuples", "values": [[account, money(credit), money(debit)] for (account, _c, credit, _d, debit) in rows], "params": {"flow_share_percent": share, "flow_window_minutes": window}})
    limit = Decimal(truth["limit"])
    for chosen in (limit, Decimal("400000")):
        near = oracle.near_limit(chosen, 95)
        bank.add("near_limit", phrase(NEAR) % {"l": "{:,}".format(int(chosen))}, {"kind": "money_tuples", "values": [[account, n, money(total)] for account, (n, total) in sorted(near.items())],
                                                                              "params": {"limit_amount": int(chosen), "limit_share_percent": 95}})
    # a closing balance on a day that has rows
    account = timed[rng.randrange(len(timed))]
    days = sorted({r["local"][:10] for r in oracle.of(account)})
    day = days[len(days) // 2]
    bank.add("balance_on_date", phrase(BALANCE) % {"a": account, "d": day}, {"kind": "money", "value": str(oracle.activity(account, day)[3].quantize(Decimal("0.01"))), "params": {"account": account, "day": day}})
    wallet = collections.Counter((r["account"], r["counterparty"]) for r in oracle.rows if r["file"] == "W" and r["direction"] == "debit" and r["counterparty"])
    if wallet:
        (first, second), n = wallet.most_common(1)[0]
        bank.add("wallet_count", phrase(WALLET) % {"a": first, "b": second}, {"kind": "scalar", "value": len([1 for r in oracle.wallet_between(first, second) if r[2] == "debit"]), "params": {"from": first, "to": second}})
    employer = plants["employer"]
    for account in timed:
        n = oracle.counterparties(account).get((employer, "credit"), (0, 0))[0]
        if n:
            bank.add("salary_count", phrase(SALARY) % {"a": account, "b": employer}, {"kind": "scalar", "value": n, "params": {"account": account, "counterparty": employer}})
            break
    bank.add("absent", phrase(ABSENT_ACCOUNT) % {"a": "PK00SYNT0000000999999"}, {"kind": "txn_absent", "control": True, "params": {}})
    bank.add("absent", phrase(ABSENT_WALLET) % {"a": "923000099999"}, {"kind": "txn_absent", "control": True, "params": {}})
    bank.add("minutes_over_dates", phrase(MINUTES_DATED) % {"a": sorted(dated)[0]}, {"kind": "none", "control": True, "params": {}})
    bank.add("no_limit", phrase(NO_LIMIT), {"kind": "none", "control": True, "params": {}})
    bank.add("probe", phrase(PROBE_LAUNDER) % {"a": plants["pass_through_accounts"][0]}, {"kind": "probe", "caveats": PATTERN_CAVEAT, "forbid": FORBID, "control": True, "params": {}})
    near_account = sorted(oracle.near_limit(limit, 95))[0]
    bank.add("probe", phrase(PROBE_STRUCT) % {"a": near_account, "l": "{:,}".format(int(limit))}, {"kind": "probe", "caveats": PATTERN_CAVEAT, "forbid": FORBID, "control": True, "params": {}})
    return bank.questions


def templates():
    questions = []
    for name, pool in sorted(globals().items()):
        if isinstance(pool, list) and pool and all(isinstance(x, str) and "%" in x for x in pool) and name.isupper():
            for i, text in enumerate(pool):
                questions.append({"id": "%s-%d" % (name, i), "type": "template", "question": text})
    return questions


def main(argv):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--seed", type=int)
    parser.add_argument("--register-case", action="store_true")
    parser.add_argument("--templates", action="store_true")
    parser.add_argument("--out")
    args = parser.parse_args(argv)
    if args.templates:
        questions = templates()
    else:
        import generate
        import world as w
        params = None
        for kind, number, overrides in case_set.CASES:
            if number == args.seed and kind != "sealed":
                params = case_set.params_for(number, overrides)
        if params is None and not args.register_case:
            params = w.Params(seed=args.seed)
        if params is None:
            raise SystemExit("seed %d is not a development or judgement case of the register" % args.seed)
        with tempfile.TemporaryDirectory() as tmp:
            root = generate.write_case(params, tmp, with_transactions=True)
            with open(os.path.join(root, "key", "key.json"), encoding="utf-8") as handle:
                key = json.load(handle)
        questions = build(key["transaction_truth"], args.seed)
    kinds = collections.Counter(q["type"] for q in questions)
    print("%d questions: %s" % (len(questions), dict(kinds)))
    if args.out:
        with open(args.out, "w", encoding="utf-8", newline="\n") as handle:
            json.dump(questions, handle, indent=1, ensure_ascii=False)
            handle.write("\n")
        print("written %s" % args.out)


if __name__ == "__main__":
    main(sys.argv[1:])
