"""The bank statements and the wallet list of a generated case, with their planted truth and an oracle (docs/data/transactions-family-20261028.md section 4).

Kept OUT of the default case build on purpose: `generate.write_case(params, out, with_transactions=True)` adds two bank statements and a wallet list to a case, and the default build (the register of 28 cases
and every hash in it) is unchanged. Nothing here uses the random streams of the default build: the transactions come from their own stream, seeded by the case's seed, and from the world's people (so that
the wallets are the numbers of the call records).

What is invented is accounts and amounts only; every account is of the reserved synthetic form PK00SYNT + 16 digits (tools/real_data/reserved.py), every wallet is a reserved synthetic mobile number. Planted,
so that a wrong reading shows:
  T1 shared counterparties   two pairs of subjects both pay one third account; every subject pays a utility (the decoy that everyone has in common).
  T2 pass-through            credits followed within two to nine minutes by a debit of 92 to 99 percent of them, and look-alikes (the debit comes 75 minutes later; the debit is half of the credit).
  T3 amounts under a limit   five debits at 95 to 99 percent of a limit of 500,000 within days, for two subjects, and four debits at 60 to 80 percent for a third (the control).
  T4 running balance         one row of one account is left out of the file, so that the row after it does not follow from the one before; two rows are written twice.
  T5 periodic credits        a salary-like credit on the first of each month.
  T6 wallet                  wallet transfers between the subjects' mobile numbers and other numbers of the world, so that a question can cross to the call records.
  T7 format defects          an account written with spaces, a debit written as "Rs. 1,500.00", a channel word the layout does not know, rows without a reference, and three rows that must be quarantined
                             (an unreadable amount, a debit and a credit together, no account); the last two accounts are written in a statement with dates only (dd/mm/yyyy).
The truth is the list of unique transactions; the oracle answers from it and shares no code with the views, the interpreter or the engine.
"""
import collections
import csv
import datetime
import os
import random
import sys
from decimal import Decimal

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "real_data"))
import reserved  # noqa: E402

BANK_LAYOUT, WALLET_LAYOUT = "bank-stmt-t1", "wallet-t1"
BANK_HEADER = ["Account No", "Txn Date", "Value Date", "Narration", "Reference", "Debit", "Credit", "Balance", "Channel", "Counterparty Account", "Counterparty Name"]
WALLET_HEADER = ["Wallet No", "Transaction ID", "Date Time", "Type", "Amount", "Balance", "Counterparty Mobile", "Status"]
CHANNELS = ("IBFT", "Raast", "ATM", "POS", "Cheque", "Cash", "Branch", "Online")
CHANNEL_MEANING = {"IBFT": "ibft", "Raast": "raast", "ATM": "atm", "POS": "pos", "Cheque": "cheque", "Cash": "cash", "Branch": "branch", "Online": "online", "Mystery": "other"}
LIMIT = Decimal("500000")
TWO = Decimal("0.01")


def money(value):
    return Decimal(value).quantize(TWO)


def _local(moment):
    return moment.strftime("%Y-%m-%d %H:%M:%S")


def build(params, world):
    """The statements and the wallet list of one case: {"files": {name: rows}, "truth": the key section}."""
    rng = random.Random("txn:%d" % params.seed)
    start = datetime.datetime.strptime(params.start_date, "%Y-%m-%d")
    days = params.period_days
    subjects = [p for p in world.persons if p.role == "subject"]
    others = [p for p in world.persons if p.role != "subject"]
    order = list(subjects)
    rng.shuffle(order)
    dated = order[-2:] if len(order) >= 5 else []                    # the last two accounts are written with dates only
    timed = [p for p in order if p not in dated]
    account_of = {p.pid: reserved.synthetic_iban(1000 + p.n) for p in subjects}
    externals = [reserved.synthetic_iban(5000 + i) for i in range(12)]
    utility, employer = reserved.synthetic_iban(9001), reserved.synthetic_iban(9002)
    shared = [reserved.synthetic_iban(7001), reserved.synthetic_iban(7002)]
    book = collections.defaultdict(list)   # account -> [{"at": datetime, "dir", "amount", "cp", "channel", "ref", "plant"}]
    counter = {"ref": 0}

    def reference():
        counter["ref"] += 1
        return "REF%08d" % counter["ref"]

    def add(account, moment, direction, amount, cp, channel=None, plant="background"):
        book[account].append({"at": moment, "dir": direction, "amount": money(amount), "cp": cp, "channel": channel or rng.choice(CHANNELS), "ref": reference(), "plant": plant})

    def stamp(day_offset, hour_low=8, hour_high=19):
        return start + datetime.timedelta(days=day_offset, hours=rng.randint(hour_low, hour_high), minutes=rng.randint(0, 59), seconds=rng.randint(0, 59))

    for index, person in enumerate(order):
        account = account_of[person.pid]
        for _ in range(rng.randint(22, 32)):
            direction = "credit" if rng.random() < 0.4 else "debit"
            add(account, stamp(rng.randrange(days)), direction, Decimal(rng.randint(10000, 9000000)) / 100, rng.choice(externals))
        month = start.replace(day=1)
        while month < start + datetime.timedelta(days=days):
            if month >= start:
                add(account, month.replace(hour=9, minute=rng.randint(0, 30), second=rng.randint(0, 59)), "credit", 85000 + 1000 * index, employer, "IBFT", "T5")
            payday = month.replace(day=5)
            if start <= payday < start + datetime.timedelta(days=days):
                add(account, payday.replace(hour=rng.randint(10, 17), minute=rng.randint(0, 59), second=rng.randint(0, 59)), "debit", rng.randint(300000, 600000) / 100, utility, "Online", "T1")
            month = (month + datetime.timedelta(days=32)).replace(day=1)
    # T1: two pairs of subjects both pay a third account (the utility is paid by everyone)
    pairs = [(timed[0], timed[1]), (timed[2], timed[3])] if len(timed) >= 4 else []
    planted_pairs = []
    for third, (x, y) in zip(shared, pairs):
        for person, times in ((x, rng.randint(2, 4)), (y, rng.randint(2, 3))):
            for _ in range(times):
                add(account_of[person.pid], stamp(rng.randrange(days)), "debit", Decimal(rng.randint(500000, 8000000)) / 100, third, "IBFT", "T1")
        planted_pairs.append({"x": account_of[x.pid], "y": account_of[y.pid], "third": third})
    # T2: pass-through, and the look-alikes that must not be found
    passes = []
    for person in timed[:3]:
        account = account_of[person.pid]
        for share in (0.92, 0.95, 0.99):
            credit_at = stamp(rng.randrange(days - 2), 9, 16)
            amount = Decimal(rng.randint(2000000, 30000000)) / 100
            minutes = rng.randint(2, 9)
            add(account, credit_at, "credit", amount, rng.choice(externals), "IBFT", "T2")
            add(account, credit_at + datetime.timedelta(minutes=minutes, seconds=rng.randint(0, 59)), "debit", amount * Decimal(str(share)), rng.choice(externals), "IBFT", "T2")
        credit_at = stamp(rng.randrange(days - 2), 9, 14)
        amount = Decimal(rng.randint(2000000, 30000000)) / 100
        add(account, credit_at, "credit", amount, rng.choice(externals), "IBFT", "T2:late")
        add(account, credit_at + datetime.timedelta(minutes=75), "debit", amount * Decimal("0.95"), rng.choice(externals), "IBFT", "T2:late")
        credit_at = stamp(rng.randrange(days - 2), 9, 14)
        amount = Decimal(rng.randint(2000000, 30000000)) / 100
        add(account, credit_at, "credit", amount, rng.choice(externals), "IBFT", "T2:half")
        add(account, credit_at + datetime.timedelta(minutes=5), "debit", amount * Decimal("0.5"), rng.choice(externals), "IBFT", "T2:half")
        passes.append(account)
    # T3: debits just under a limit, and a control
    near = []
    for person in timed[:2]:
        base = rng.randrange(max(1, days - 10))
        for k in range(5):
            add(account_of[person.pid], stamp(base + 2 * k), "debit", LIMIT * Decimal(str(round(rng.uniform(0.95, 0.99), 3))), rng.choice(externals), "Cheque", "T3")
        near.append(account_of[person.pid])
    control = timed[2] if len(timed) > 2 else timed[0]
    base = rng.randrange(max(1, days - 10))
    for k in range(4):
        add(account_of[control.pid], stamp(base + 2 * k), "debit", LIMIT * Decimal(str(round(rng.uniform(0.60, 0.80), 3))), rng.choice(externals), "Cheque", "T3:control")

    # the running balance of each account, and the rows of the statements
    opening = {}
    transactions, files = [], {"A": [], "B": [], "W": []}
    for index, person in enumerate(order):
        account = account_of[person.pid]
        rows = sorted(book[account], key=lambda r: (r["at"], r["ref"]))
        balance = money(6000000 + 10000 * index)
        opening[account] = balance
        is_dated = person in dated
        target = files["B" if is_dated else "A"]
        for position, row in enumerate(rows):
            balance = money(balance + row["amount"] if row["dir"] == "credit" else balance - row["amount"])
            at = row["at"].replace(hour=0, minute=0, second=0) if is_dated else row["at"]
            row.update({"account": account, "balance": balance, "file": "B" if is_dated else "A", "local": _local(at), "precision": "date" if is_dated else "second"})
            transactions.append(row)
    # T6: the wallets of the subjects, with the numbers of the world
    wallet_pool = [p.msisdn for p in order + others]
    wallet_rows = []
    for index, person in enumerate(order):
        wallet = person.msisdn
        balance = money(5000000)
        for k in range(rng.randint(12, 18)):
            moment = stamp(rng.randrange(days), 7, 22)
            kind = rng.choice(("Sent", "Sent", "Received", "Cash In", "Cash Out", "Bill Payment"))
            amount = money(Decimal(rng.randint(10000, 500000)) / 100)
            direction = "credit" if kind in ("Received", "Cash In") else "debit"
            cp = rng.choice([m for m in wallet_pool if m != wallet]) if kind in ("Sent", "Received") else ""
            wallet_rows.append({"at": moment, "account": wallet, "dir": direction, "amount": amount, "cp": cp, "kind": kind,
                                "channel": {"Sent": "wallet", "Received": "wallet", "Cash In": "cash", "Cash Out": "cash", "Bill Payment": "online"}[kind], "plant": "T6"})
    wallet_rows.sort(key=lambda r: (r["account"], r["at"]))
    seen_balance = {}
    for number, row in enumerate(wallet_rows, start=1):
        balance = seen_balance.get(row["account"], money(5000000))
        balance = money(balance + row["amount"] if row["dir"] == "credit" else balance - row["amount"])
        seen_balance[row["account"]] = balance
        row.update({"balance": balance, "file": "W", "local": _local(row["at"]), "precision": "second", "ref": "TXN%08d" % number})
        transactions.append(row)

    # the statement rows as the files write them
    def money_text(value, rupees=False):
        return ("Rs. " if rupees else "") + "{:,.2f}".format(value)

    a_rows, b_rows = [], []
    spaced = {"done": 0}
    mystery = {"done": 0}
    blank_ref = {"done": 0}
    rupee = {"done": 0}
    removed = None
    a_accounts = [account_of[p.pid] for p in timed]
    victim_account = a_accounts[0]
    victim_rows = [r for r in transactions if r["file"] == "A" and r["account"] == victim_account and r["plant"] == "background"]
    removed_row = victim_rows[len(victim_rows) // 2]
    removed = removed_row["ref"]
    for row in transactions:
        if row["file"] not in ("A", "B"):
            continue
        if row is removed_row:
            continue
        shown_account = row["account"]
        channel_word = row["channel"]              # the word the statement writes (the meaning is CHANNEL_MEANING[word])
        reference_text = row["ref"]
        rupees = False
        if row["file"] == "A":
            if spaced["done"] < 3 and row["account"] == a_accounts[-1]:
                shown_account = " ".join(row["account"][i:i + 4] for i in range(0, len(row["account"]), 4))
                spaced["done"] += 1
            if mystery["done"] < 1 and row["plant"] == "background" and row["dir"] == "debit" and row["account"] == a_accounts[-1]:
                channel_word = row["channel"] = "Mystery"
                mystery["done"] += 1
            if blank_ref["done"] < 2 and row["plant"] == "background" and row["account"] == a_accounts[-1]:
                reference_text, row["blank_ref"] = "", True
                blank_ref["done"] += 1
            if rupee["done"] < 1 and row["dir"] == "debit" and row["plant"] == "background" and row["account"] == a_accounts[0]:
                rupees = True
                rupee["done"] += 1
        when = row["local"] if row["file"] == "A" else "%02d/%02d/%d" % (row["at"].day, row["at"].month, row["at"].year)
        cells = [shown_account, when, "", "Synthetic narration", reference_text, money_text(row["amount"], rupees) if row["dir"] == "debit" else "", money_text(row["amount"]) if row["dir"] == "credit" else "",
                 money_text(row["balance"]), channel_word, row["cp"], ""]
        (a_rows if row["file"] == "A" else b_rows).append(cells)
        row["written_ref"] = reference_text
    duplicates = 0
    for position in (len(a_rows) // 5, len(a_rows) // 2 + 3):
        a_rows.insert(position + 1, list(a_rows[position]))
        duplicates += 1
    junk_account = account_of[order[0].pid]
    quarantined = {"amount_unreadable": 1, "amount_ambiguous": 1, "account_missing": 1}
    a_rows.insert(len(a_rows) // 3, [junk_account, _local(start + datetime.timedelta(days=1, hours=10)), "", "Synthetic narration", "REFJUNK1", "lots", "", "", "IBFT", externals[0], ""])
    a_rows.insert(len(a_rows) // 2, [junk_account, _local(start + datetime.timedelta(days=1, hours=11)), "", "Synthetic narration", "REFJUNK2", "10.00", "10.00", "", "IBFT", externals[0], ""])
    a_rows.insert(len(a_rows) // 4, ["", _local(start + datetime.timedelta(days=1, hours=12)), "", "Synthetic narration", "REFJUNK3", "10.00", "", "", "IBFT", externals[0], ""])
    w_rows = [[r["account"], r["ref"], r["local"], r["kind"], "{:.2f}".format(r["amount"]), "{:.2f}".format(r["balance"]), r["cp"] or "", "Completed"] for r in wallet_rows]
    # the row after the removed one does not follow from the one before it
    after = [r for r in transactions if r["file"] == "A" and r["account"] == victim_account and r is not removed_row and (r["at"], r["ref"]) > (removed_row["at"], removed_row["ref"])]
    after.sort(key=lambda r: (r["at"], r["ref"]))
    breaks = [after[0]["ref"]] if after else []
    for number, r in enumerate(transactions):
        r["seq"] = number            # the order the rows are written in a file: the order a day's rows are kept in when they share a time (a statement with dates only)
    truth_rows = []
    for r in transactions:
        if r is removed_row:
            continue
        truth_rows.append({"file": r["file"], "account": r["account"], "local": r["local"], "precision": r["precision"], "direction": r["dir"], "amount": str(r["amount"]),
                           "balance_after": str(r["balance"]), "counterparty": r["cp"] or None, "channel": r["channel"] if r["file"] == "W" else CHANNEL_MEANING[r["channel"]],
                           "reference": None if r.get("blank_ref") else r["ref"], "key": r["ref"], "seq": r["seq"], "plant": r["plant"]})
    section = {
        "bank_layout": BANK_LAYOUT, "wallet_layout": WALLET_LAYOUT, "transactions": truth_rows, "limit": str(LIMIT),
        "plants": {"pairs": planted_pairs, "utility": utility, "employer": employer, "pass_through_accounts": passes, "near_limit_accounts": near, "control_account": account_of[control.pid],
                   "dated_accounts": [account_of[p.pid] for p in dated], "removed_reference": removed, "break_references": breaks, "victim_account": victim_account},
        "planted": {"duplicate_rows": duplicates, "quarantined": quarantined, "unmapped_channel": mystery["done"], "date_only_rows": len(b_rows),
                    "rows_in_file": {"A": len(a_rows), "B": len(b_rows), "W": len(w_rows)}},
    }
    return {"files": {"A": a_rows, "B": b_rows, "W": w_rows}, "truth": section}


def write(rows, path, header):
    with open(path, "w", encoding="utf-8", newline="") as handle:
        writer = csv.writer(handle, lineterminator="\n")
        writer.writerow(header)
        writer.writerows(rows)


# ---------------------------------------------------------------- the oracle: answers from the planted truth alone

class Oracle:
    def __init__(self, truth):
        self.truth = truth
        self.rows = [dict(r, amount=Decimal(r["amount"]), balance_after=Decimal(r["balance_after"])) for r in truth["transactions"]]

    def accounts(self):
        return sorted({r["account"] for r in self.rows})

    def of(self, account):
        return sorted((r for r in self.rows if r["account"] == account), key=lambda r: (r["local"], r["seq"]))

    def totals(self, account, start=None, end=None):
        """(credits, debits, transactions) of an account between two local times (both optional; the end is exclusive)."""
        rows = [r for r in self.of(account) if (start is None or r["local"] >= start) and (end is None or r["local"] < end)]
        return (sum((r["amount"] for r in rows if r["direction"] == "credit"), Decimal("0")), sum((r["amount"] for r in rows if r["direction"] == "debit"), Decimal("0")), len(rows))

    def counterparties(self, account):
        out = collections.defaultdict(lambda: [0, Decimal("0")])
        for r in self.of(account):
            if r["counterparty"]:
                key = (r["counterparty"], r["direction"])
                out[key][0] += 1
                out[key][1] += r["amount"]
        return {k: (v[0], v[1]) for k, v in out.items()}

    def shared(self, first, second):
        a = {cp for (cp, _) in self.counterparties(first)}
        b = {cp for (cp, _) in self.counterparties(second)}
        return sorted(a & b)

    def largest(self, account, direction, n=1):
        return sorted((r["amount"] for r in self.of(account) if r["direction"] == direction), reverse=True)[:n]

    def activity(self, account, day):
        rows = [r for r in self.of(account) if r["local"][:10] == day]
        if not rows:
            return None
        closing = next((r["balance_after"] for r in reversed(rows)), None)
        return (len(rows), sum((r["amount"] for r in rows if r["direction"] == "credit"), Decimal("0")), sum((r["amount"] for r in rows if r["direction"] == "debit"), Decimal("0")), closing)

    def flows(self, share_percent=90, window_minutes=10):
        """Pass-through candidates: a credit and a later debit of the same account within the window, of at least the share and at most the credit (rows that state a time of day only)."""
        import datetime
        out = []
        for account in self.accounts():
            rows = [r for r in self.of(account) if r["precision"] != "date"]
            for credit in (r for r in rows if r["direction"] == "credit" and r["amount"] > 0):
                c_at = datetime.datetime.strptime(credit["local"], "%Y-%m-%d %H:%M:%S")
                for debit in (r for r in rows if r["direction"] == "debit"):
                    d_at = datetime.datetime.strptime(debit["local"], "%Y-%m-%d %H:%M:%S")
                    if c_at < d_at <= c_at + datetime.timedelta(minutes=window_minutes) and debit["amount"] <= credit["amount"] and debit["amount"] * 100 >= credit["amount"] * share_percent:
                        out.append((account, credit["local"], str(credit["amount"]), debit["local"], str(debit["amount"])))
        return sorted(out)

    def near_limit(self, limit, share_percent=95):
        out = {}
        for account in self.accounts():
            rows = [r for r in self.of(account) if r["direction"] == "debit" and r["amount"] * 100 >= limit * share_percent and r["amount"] < limit]
            if rows:
                out[account] = (len(rows), sum((r["amount"] for r in rows), Decimal("0")))
        return out

    def wallet_between(self, first, second):
        return sorted((r["local"], str(r["amount"]), r["direction"]) for r in self.rows if r["file"] == "W" and r["account"] == first and r["counterparty"] == second)
