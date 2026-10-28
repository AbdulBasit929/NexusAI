"""Text documents for a Pakistani showcase case, with the mention truth (reference cases programme, stage S3a; design and gates in section 10 of the programme document).

    built = build(params, world, key)     # params.locale must be set; key is the case key after the other families were added
    write(built, root)                    # files/docs/...

Fourteen documents (English, Urdu, Roman Urdu) are composed from the planted truth of the world: the accused, the complainant and the witness are persons of the world, and every number,
account, national ID number, plate, handset and SIM they carry is the one the other families hold. The text is built from parts: plain text, a mention (kind, normalised value, the form it is
written in, the person it belongs to) or a decoy (text that looks like an identifier and is not). The truth is read off the parts, so it shares no code with the extractor.
Own random stream ("docs:<seed>"): the default build, and with it every hash of the register, is unchanged. Every document says SYNTHETIC in its first lines.
"""
import datetime
import os
import random
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "real_data"))
import docx_writer  # noqa: E402
import pk_locale  # noqa: E402
import reserved  # noqa: E402

URDU_DIGITS = str.maketrans("0123456789", "۰۱۲۳۴۵۶۷۸۹")
LAYOUT = "documents-s3a"


class M:
    """A mention: kind, normalised value, the text as written, and the person it belongs to."""
    def __init__(self, kind, value, surface, entity=None):
        self.kind, self.value, self.surface, self.entity = kind, value, surface, entity


class D:
    """A decoy: text that looks like an identifier of a kind and is not one."""
    def __init__(self, surface, resembles):
        self.surface, self.resembles = surface, resembles


class N:
    """A name written in the text (not extracted: names stay dark until the masking design; kept in the truth for later)."""
    def __init__(self, entity, surface):
        self.entity, self.surface = entity, surface


def text_of(parts):
    return "".join(part if isinstance(part, str) else part.surface for part in parts)


def p(*parts, rtl=False, bold=False):
    return ("p", list(parts), {"rtl": rtl, "bold": bold})


def table(rows, rtl=False):
    return ("table", [[list(cell) if isinstance(cell, (list, tuple)) else [cell] for cell in row] for row in rows], {"rtl": rtl})


# ------------------------------------------------------------------------------------------------ surfaces
def msisdn_surface(msisdn, form, urdu=False):
    prefix, body = msisdn[2:5], msisdn[5:]
    forms = ["0%s-%s", "0%s %s", "0%s%s", "+92 %s %s", "+92-%s-%s", "0092 %s %s", "92 %s %s", "(0%s) %s"]
    text = forms[form % len(forms)] % (prefix, body)
    return text.translate(URDU_DIGITS) if urdu else text


def nid_surface(national_id, form, urdu=False):
    digits = national_id.replace("-", "")
    forms = [national_id, "%s %s %s" % (digits[:5], digits[5:12], digits[12]), digits]
    text = forms[form % len(forms)]
    return text.translate(URDU_DIGITS) if urdu else text


def iban_surface(iban, form):
    spaced = " ".join([iban[:4], iban[4:8]] + [iban[i:i + 4] for i in range(8, len(iban), 4)])
    return [spaced, iban, iban.lower(), spaced][form % 4]


def plate_surface(key, form, urdu=False):
    letters, number = key[:4], key[4:]
    text = ["%s-%s", "%s %s", "%s%s", "%s %s"][form % 4] % (letters, number)
    if form % 4 == 3:
        text = text.lower()
    return text.translate(URDU_DIGITS) if urdu else text


class Context:
    def __init__(self, params, world, key):
        self.params, self.world, self.key = params, world, key
        self.city = params.locale
        self.rng = random.Random("docs:%d" % params.seed)
        self.counter = {"form": 0}
        self.persons = {p_.pid: p_ for p_ in world.persons}
        self.start = datetime.datetime.strptime(params.start_date, "%Y-%m-%d")
        self.days = params.period_days

    def form(self):
        self.counter["form"] += 1
        return self.counter["form"] + self.rng.randrange(8)

    def ident(self, person):
        return pk_locale.identity(self.city, person.n, person.community, person.home_cell)

    def name(self, person, urdu=False):
        who = self.ident(person)
        return N(person.pid, who["name_urdu"] if urdu else who["name"])

    def female(self, person):
        return self.ident(person)["gender"] == "F"

    def accused_ur(self, person):
        return "ملزمہ" if self.female(person) else "ملزم"

    def son_ur(self, person):
        return " دختر " if self.female(person) else " ولد "

    def father(self, person, urdu=False):
        who = self.ident(person)
        return who["father_urdu"] if urdu else who["father"]

    def date(self, urdu=False, offset=None):
        day = self.start + datetime.timedelta(days=self.rng.randrange(10, max(11, self.days)) if offset is None else offset)
        text = day.strftime("%d/%m/%Y")
        return text.translate(URDU_DIGITS) if urdu else text

    def msisdn(self, person, urdu=False, form=None):
        return M("msisdn", person.msisdn, msisdn_surface(person.msisdn, self.form() if form is None else form, urdu), person.pid)

    def number(self, msisdn, owner=None, urdu=False):
        return M("msisdn", msisdn, msisdn_surface(msisdn, self.form(), urdu), owner)

    def nid(self, person, urdu=False):
        return M("national_id", self.ident(person)["national_id"], nid_surface(self.ident(person)["national_id"], self.form(), urdu), person.pid)

    def iban(self, person):
        value = reserved.synthetic_iban(1000 + person.n)
        return M("account", value, iban_surface(value, self.form()), person.pid)

    def iccid(self, person):
        value = reserved.synthetic_iccid(person.n)
        return M("iccid", value, value, person.pid)

    def imei(self, person):
        return M("imei", person.imei, person.imei, person.pid)

    def imsi(self, person):
        return M("imsi", person.imsi, person.imsi, person.pid)

    def plate(self, key, owner=None, urdu=False):
        return M("plate", key, plate_surface(key, self.form(), urdu), owner)

    def place(self, site, urdu=False):
        base = pk_locale.area_name(self.city, site)
        value = base.lower()
        return M("place", value, pk_locale.AREAS_URDU[base] if urdu else base)


# ------------------------------------------------------------------------------------------------ the documents
def documents(ctx):
    rng, key = ctx.rng, ctx.key
    persons = ctx.world.persons
    subjects = [p_ for p_ in persons if p_.role == "subject"]
    others = [p_ for p_ in persons if p_.role != "subject"]
    men = [p_ for p_ in others if ctx.ident(p_)["gender"] == "M"]
    a, b, c = rng.sample(subjects, 3)
    complainant, witness = rng.choice(others), rng.choice(men)
    plates = list(key.get("anpr", {}).get("of_interest") or [])
    plate_a, plate_b, plate_c = (plates + ["SYNT0000"] * 3)[:3]
    successor = next((x for x in key["plants"] if x.get("type") == "successor"), None)
    dual = next((x for x in key["plants"] if x.get("type") == "dual_sim"), None)
    holder = {x.msisdn: x for x in persons}
    out = []

    def site_of(person, shift=0):
        return (person.home_cell + shift) % 12

    # D01 First Information Report, English
    k_name = ctx.name(complainant)
    out.append(("D01-fir-english", ".docx", "en", [
        p("FIRST INFORMATION REPORT (SYNTHETIC TEST DOCUMENT)", bold=True),
        p("This document is synthetic. Every number in it is invented and of a reserved form; it is not a record of any real person."),
        p("Police Station: ", ctx.place(site_of(a)), ", District Lahore. FIR No. ", D("45/26", "plate"), " under ", D("Section 420 PPC", "plate"), " and ", D("Section 161", "plate"),
          ". Date: %s." % ctx.date()),
        table([["Complainant", [k_name, " (%s %s)" % (ctx.ident(complainant)["relation"], ctx.father(complainant))]],
               ["Complainant mobile", [ctx.msisdn(complainant)]],
               ["Accused", [ctx.name(a)]],
               ["Accused national ID", [ctx.nid(a)]],
               ["Accused mobile", [ctx.msisdn(a)]],
               ["Account named by the complainant", [ctx.iban(a)]],
               ["Vehicle", ["Honda City, registration ", ctx.plate(plate_a, a.pid)]]]),
        p("The complainant states that on %s the accused called from " % ctx.date(), ctx.msisdn(a), " and asked the complainant to deposit PKR ", D("350,000", "msisdn"),
          " into the account above. The accused was last seen in ", ctx.place(site_of(a, 1)), "."),
        p("Police station landline ", D("042-35761234", "msisdn"), ", reference ", D("REF00000394", "account"), ", a longer reference ", D("9903000001000555", "msisdn"), "."),
    ]))
    # D02 First Information Report, Urdu, right to left, Urdu digits
    k2, a2, b2 = complainant, b, c
    out.append(("D02-fir-urdu", ".docx", "ur", [
        p("ابتدائی اطلاعی رپورٹ (مصنوعی آزمائشی دستاویز)", rtl=True, bold=True),
        p("یہ دستاویز مصنوعی ہے؛ اس میں درج تمام نمبر فرضی ہیں اور یہ کسی حقیقی شخص کا ریکارڈ نہیں۔", rtl=True),
        p("تھانہ: ", ctx.place(site_of(a2), urdu=True), "، ضلع لاہور۔ مقدمہ نمبر ", D("۷۸/۲۶", "plate"), " زیر دفعہ ", D("۴۲۰", "msisdn"), " ت پ۔ تاریخ: %s۔" % ctx.date(urdu=True), rtl=True),
        p("مدعیہ " if ctx.female(k2) else "مدعی ", ctx.name(k2, urdu=True), ctx.son_ur(k2), ctx.father(k2, urdu=True), " نے بیان کیا کہ " + ctx.accused_ur(a2) + " ", ctx.name(a2, urdu=True), " نے موبائل نمبر ", ctx.msisdn(a2, urdu=True),
          " اور ", ctx.number(b2.msisdn, b2.pid, urdu=True), " سے رابطہ کیا۔", rtl=True),
        p(ctx.accused_ur(a2) + " نے رقم اکاؤنٹ ", ctx.iban(a2), " میں جمع کرانے کو کہا۔", rtl=True),
        p(ctx.accused_ur(a2) + " کو آخری بار ", ctx.place(site_of(a2, 2), urdu=True), " میں دیکھا گیا۔ " + ctx.accused_ur(a2) + " کا شناختی کارڈ نمبر ", ctx.nid(a2, urdu=True), " ہے۔", rtl=True),
    ]))
    # D03 Witness statement, Urdu, UTF-8
    out.append(("D03-witness-urdu", ".txt", "ur", [
        p("بیان گواہ (زیر دفعہ ۱۶۱ ضابطہ فوجداری) — مصنوعی آزمائشی دستاویز", rtl=True),
        p("میں ", ctx.name(witness, urdu=True), " ولد ", ctx.father(witness, urdu=True), " ساکن ", ctx.place(site_of(witness), urdu=True), " لاہور، بیان کرتا ہوں کہ مجھے " + ctx.accused_ur(c) + " کا موبائل نمبر ",
          ctx.msisdn(c, urdu=True), " معلوم ہے۔", rtl=True),
        p("میں نے " + ctx.accused_ur(c) + " کو ", ctx.place(site_of(c, 3), urdu=True), " میں اپنی آنکھوں سے دیکھا۔ اس کی گاڑی کا نمبر ", ctx.plate(plate_c, c.pid, urdu=True), " تھا۔", rtl=True),
    ], "utf-8"))
    # D04 Witness statement, Roman Urdu, UTF-16 with Windows line ends; the successor number
    old_n, new_n = (successor["old"], successor["new"]) if successor else (b.msisdn, c.msisdn)
    out.append(("D04-witness-roman-urdu", ".txt", "ur-Latn", [
        p("Bayan gawah (Section 161 CrPC) - SYNTHETIC test document"),
        p("Mera naam ", ctx.name(witness), " hai. Main ", ctx.place(site_of(witness)), " ka rehne wala hun."),
        p("Maine mulzim ko ", ctx.place(site_of(b, 1)), " mein dekha tha. Us ke paas gari number ", ctx.plate(plate_b, b.pid), " thi."),
        p("Pehle uska number ", ctx.number(old_n, holder[old_n].pid if old_n in holder else None), " tha, ab wo naye number ", ctx.number(new_n, holder[new_n].pid if new_n in holder else None),
          " se baat " + ("karti" if (successor and old_n in holder and ctx.female(holder[old_n])) else "karta") + " hai. Purana number band ho gaya hai."),
    ], "utf-16"))
    # D05 Seizure memo, table
    shared = holder[dual["numbers"][0]] if dual and dual["numbers"][0] in holder else a
    second = holder[dual["numbers"][1]] if dual and dual["numbers"][1] in holder else b
    out.append(("D05-seizure-memo", ".docx", "en", [
        p("RECOVERY (SEIZURE) MEMO (SYNTHETIC TEST DOCUMENT)", bold=True),
        p("Recovered from the accused ", ctx.name(shared), " at ", ctx.place(site_of(shared)), " in the presence of two witnesses on %s." % ctx.date()),
        table([["Item", "Description", "Identifier"],
               ["1", "Mobile handset", [ctx.imei(shared)]],
               ["2", "SIM card (slot 1)", [ctx.iccid(shared)]],
               ["3", "SIM card (slot 2)", [ctx.iccid(second)]],
               ["4", "Mobile number in slot 1", [ctx.msisdn(shared)]],
               ["5", "Mobile number in slot 2", [ctx.number(second.msisdn, second.pid)]],
               ["6", "Identity card copy", ["CNIC ", ctx.nid(shared)]],
               ["7", "Cash", ["PKR 85,000"]]]),
        p("Handset check value ", D("123456789012345", "imei"), " was struck out as misread (IMEI)."),
    ]))
    # D06 Request for call records, several numbers in several forms
    listed = rng.sample(subjects, 6)
    out.append(("D06-cdr-request", ".txt", "en", [
        p("To: The Manager Legal, Mobile Operator (SYNTHETIC TEST DOCUMENT)"),
        p("Subject: Request for call detail records for the period %s to %s." % (ctx.date(offset=2), ctx.date(offset=ctx.days - 2))),
        p("Please provide call detail records and cell site information for the following numbers:")] +
        [p("%d. " % (i + 1), ctx.msisdn(x)) for i, x in enumerate(listed)] +
        [p("Contact at the police station: ", D("042-35761234", "msisdn"), ".")], "utf-8-sig"))
    # D07 Request for a bank statement
    out.append(("D07-bank-request", ".txt", "en", [
        p("To: The Branch Manager, Specimen Bank Limited (SYNTHETIC TEST DOCUMENT)"),
        p("Subject: Statement of account for the period %s to %s." % (ctx.date(offset=2), ctx.date(offset=ctx.days - 2))),
        p("Please provide the statement of the following accounts: ", ctx.iban(a), " and ", ctx.iban(b), "."),
        p("Account holder national ID number: ", ctx.nid(b), ", mobile number ", ctx.msisdn(b), "."),
        p("Amounts for reference only: ", D("1,234,567.00", "msisdn"), ", date ", D("20260305", "msisdn"), "."),
    ]))
    # D08 Subscriber verification report
    out.append(("D08-subscriber-report", ".txt", "en", [
        p("SUBSCRIBER VERIFICATION REPORT (SYNTHETIC TEST DOCUMENT)"),
        p("Mobile number: ", ctx.msisdn(a)),
        p("SIM serial (ICCID): ", ctx.iccid(a)),
        p("IMSI: ", ctx.imsi(a)),
        p("Registered national ID number: ", ctx.nid(a)),
        p("Registered address area: ", ctx.place(site_of(a))),
        p("Second line: mobile number ", ctx.msisdn(b), ", SIM serial ", ctx.iccid(b), ", IMSI ", ctx.imsi(b), "."),
    ]))
    # D09 Utility bill
    out.append(("D09-utility-bill", ".txt", "en", [
        p("ELECTRICITY BILL (SYNTHETIC TEST DOCUMENT) - Specimen Electric Supply Company"),
        p("Consumer: ", ctx.name(c), ". Address: House %d, Street %d, " % (1 + rng.randrange(400), 1 + rng.randrange(30)), ctx.place(site_of(c)), ", Lahore."),
        p("Consumer reference ", D("REF00000394", "account"), ", meter ", D("112233445566", "msisdn"), ", amount due PKR ", D("1,234,567.00", "msisdn"), "."),
        p("Complaints helpline ", D("042-35761234", "msisdn"), "."),
    ]))
    # D10 Vehicle registration extract
    out.append(("D10-vehicle-register", ".txt", "en", [
        p("MOTOR VEHICLE REGISTRATION EXTRACT (SYNTHETIC TEST DOCUMENT)"),
        p("Registration number: ", ctx.plate(plate_a, a.pid), ". Make: Honda City. Owner national ID number: ", ctx.nid(a), "."),
        p("Second vehicle, registration ", ctx.plate(plate_b, b.pid), ", owner national ID number ", ctx.nid(b), "."),
        p("Reference only: ", D("SYNT4O24", "plate"), " (a misread, letter O), ", D("PPC 420", "plate"), "."),
    ]))
    # D11 Chat export
    t = ctx.start + datetime.timedelta(days=rng.randrange(20, max(21, ctx.days - 5)))
    stamp = lambda minute: (t + datetime.timedelta(minutes=minute)).strftime("%d/%m/%Y, %H:%M - ")
    out.append(("D11-chat-export", ".txt", "mixed", [
        p(stamp(0), "Messages and calls are end-to-end encrypted. This is a SYNTHETIC test chat."),
        p(stamp(1), ctx.name(a), ": Bhai number note kar lo ", ctx.msisdn(c)),
        p(stamp(2), ctx.name(b), ": theek hai, account number bhejo"),
        p(stamp(3), ctx.name(a), ": ", ctx.iban(b)),
        p(stamp(5), ctx.name(b), ": kal ", ctx.place(site_of(b, 1)), " mein milte hain"),
        p(stamp(6), ctx.name(a), ": ٹھیک ہے، گاڑی ", ctx.plate(plate_a, a.pid), " میں آؤں گا"),
        p(stamp(9), ctx.name(a), ": mera naya number ", ctx.msisdn(a), " hai, purana mat use karna"),
    ]))
    # D12 Complaint application, Urdu
    out.append(("D12-complaint-urdu", ".docx", "ur", [
        p("درخواست برائے اندراج مقدمہ (مصنوعی آزمائشی دستاویز)", rtl=True, bold=True),
        p("بخدمت جناب ایس ایچ او تھانہ ", ctx.place(site_of(complainant), urdu=True), "۔", rtl=True),
        p("گزارش ہے کہ مجھے نامعلوم نمبر ", ctx.msisdn(b, urdu=True), " سے دھمکی آمیز کال موصول ہوئی۔ کال کے وقت گاڑی نمبر پلیٹ ", ctx.plate(plate_b, b.pid, urdu=True), " موقع پر موجود تھی۔", rtl=True),
        p("میرا رابطہ نمبر ", ctx.msisdn(complainant, urdu=True), " ہے۔", rtl=True),
    ]))
    # D13 Extraction index
    out.append(("D13-extraction-index", ".txt", "en", [
        p("DEVICE EXTRACTION INDEX (SYNTHETIC TEST DOCUMENT)"),
        p("Handset 1, IMEI ", ctx.imei(a), ": numbers ", ctx.msisdn(a), " and ", ctx.number(a.msisdn, a.pid)),
        p("Handset 2, IMEI ", ctx.imei(b), ": number ", ctx.msisdn(b)),
        p("Handset 3, IMEI ", ctx.imei(c), ": number ", ctx.msisdn(c)),
        p("Extracted on %s at Lahore by the digital forensic unit (specimen)." % ctx.date()),
    ]))
    # D14 Control: names no identifier of the case
    out.append(("D14-control-report", ".txt", "en", [
        p("ROUTINE REPORT (SYNTHETIC TEST DOCUMENT)"),
        p("Patrol of the sector completed; no incident to report. See ", D("Section 161", "plate"), ", ", D("FIR 45/26", "plate"), " and ", D("PPC 420", "plate"), " of the register."),
        p("Helpline ", D("042-35761234", "msisdn"), ", ", D("REF00000394", "account"), ", total ", D("1,234,567.00", "msisdn"), " on ", D("20260305", "msisdn"), "."),
    ]))
    return out


# ------------------------------------------------------------------------------------------------ rendering and truth
def render(name, ext, language, blocks, encoding="utf-8"):
    """Bytes of the file and its truth, read off the parts."""
    mentions, decoys, names = [], [], []
    lines = []
    numbering = docx_writer.paragraph_numbers(blocks) if ext == ".docx" else None
    doc_blocks = []
    for index, (kind, content, opts) in enumerate(blocks):
        if kind == "p":
            cells = [content]
        else:
            cells = [cell for row in content for cell in row]
        for cell_index, parts in enumerate(cells):
            where = {"block": index, "line": len(lines) + 1}
            if numbering is not None:
                where["paragraph"] = numbering[index][cell_index]
            for part in parts:
                if isinstance(part, M):
                    mentions.append({"kind": part.kind, "value": part.value, "surface": part.surface, "entity": part.entity, **where})
                elif isinstance(part, D):
                    decoys.append({"surface": part.surface, "resembles": part.resembles, **where})
                elif isinstance(part, N):
                    names.append({"entity": part.entity, "surface": part.surface, **where})
        if kind == "p":
            lines.append(text_of(content))
            doc_blocks.append(("p", text_of(content), opts))
        else:
            rows = [[text_of(cell) for cell in row] for row in content]
            lines.extend(" | ".join(row) for row in rows)
            doc_blocks.append(("table", rows, opts))
    if ext == ".docx":
        data = docx_writer.build_docx(doc_blocks, title=name)
    else:
        text = ("\r\n" if encoding == "utf-16" else "\n").join(lines) + ("\r\n" if encoding == "utf-16" else "\n")
        data = text.encode(encoding)
    return data, {"mentions": mentions, "decoys": decoys, "names": names, "encoding": encoding if ext == ".txt" else None}


def build(params, world, key):
    if not params.locale:
        raise ValueError("documents are written for a case with a locale (Params.locale)")
    ctx = Context(params, world, key)
    built, entries, truth = {}, [], {}
    for item in documents(ctx):
        name, ext, language, blocks = item[:4]
        encoding = item[4] if len(item) > 4 else "utf-8"
        data, doc_truth = render(name, ext, language, blocks, encoding)
        rel = "files/docs/%s%s" % (name, ext)
        built[rel] = data
        entries.append({"name": name, "file": rel, "format": ext.lstrip("."), "language": language, "encoding": doc_truth.pop("encoding")})
        truth[name] = doc_truth
    return {"files": built, "documents": entries, "truth": {"layout": LAYOUT, "city": params.locale, "gazetteer": pk_locale.gazetteer(params.locale), "documents": truth}}


def write(built, root):
    for rel, data in built["files"].items():
        path = os.path.join(root, *rel.split("/"))
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "wb") as handle:
            handle.write(data)
