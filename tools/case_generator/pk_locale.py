"""Pakistani locale pack for the showcase cases (reference cases programme, section 9.2).

The pack is chosen by `Params.locale` (a key of CITIES). With no locale, nothing here is used and every file of the 20-case register is unchanged.

What is real and what is not:
* The grid origin of each city is a real position; the sites on it are synthetic (about 2 km apart, the same geometry as the fictional grid), and the area label of a site is the nearest named
  area that OpenStreetMap Nominatim reported for the centre of the site (checked 2026-10-28, `reverse` at zoom 16). A label says where the synthetic site is drawn, never that a real tower stands there.
* A person's name is a common combination of a given name and a surname of the person's community: it can coincide with a real person's. It only ever appears in the local showcase files, always
  with a national ID number that begins 00000 (no province or district has that code, so no such number is real) and the line SYNTHETIC.
* The last digit of the national ID number follows the real rule (odd for men, even for women); the other digits are a family number from a reserved range.
"""
import math
import random

NATIONAL_ID_PREFIX = "00000"

# origin (latitude, longitude) of the south-west site; site idx has row idx // 4 (north) and column idx % 4 (east); areas read back from OpenStreetMap Nominatim (2026-10-28)
CITIES = {
    "lahore": {
        "city": "Lahore", "province": "Punjab", "origin": (31.46, 74.28),
        "areas": ["Johar Town (west)", "Johar Town (College Road)", "Newtown", "Kot Lakhpat",
                  "Johar Town (Canal Bank)", "Faisal Town", "Model Town", "Al Noor Town",
                  "Mustafa Town", "Muslim Town", "Garden Town", "Gulberg"],
    },
}

# (given names, surnames) by community index; both lists hold (Latin, Urdu) pairs
MALE = [("Muhammad", "محمد"), ("Ahmed", "احمد"), ("Ali", "علی"), ("Hassan", "حسن"), ("Hussain", "حسین"), ("Usman", "عثمان"), ("Bilal", "بلال"), ("Imran", "عمران"), ("Kamran", "کامران"),
        ("Faisal", "فیصل"), ("Tariq", "طارق"), ("Shahid", "شاہد"), ("Naveed", "نوید"), ("Asif", "آصف"), ("Zubair", "زبیر"), ("Adnan", "عدنان"), ("Salman", "سلمان"), ("Hamza", "حمزہ"),
        ("Junaid", "جنید"), ("Rashid", "راشد"), ("Saeed", "سعید"), ("Waqas", "وقاص"), ("Farhan", "فرحان"), ("Irfan", "عرفان"), ("Khalid", "خالد")]
FEMALE = [("Ayesha", "عائشہ"), ("Fatima", "فاطمہ"), ("Zainab", "زینب"), ("Sana", "ثنا"), ("Hina", "حنا"), ("Maryam", "مریم"), ("Nadia", "نادیہ"), ("Saima", "صائمہ"), ("Bushra", "بشریٰ"),
          ("Rabia", "رابعہ"), ("Sidra", "سدرہ"), ("Mehwish", "مہوش"), ("Amna", "آمنہ"), ("Farah", "فرح"), ("Uzma", "عظمیٰ")]
SURNAMES = [
    [("Cheema", "چیمہ"), ("Gill", "گل"), ("Butt", "بٹ"), ("Malik", "ملک"), ("Chaudhry", "چوہدری"), ("Sheikh", "شیخ"), ("Rana", "رانا"), ("Bajwa", "باجوہ"), ("Virk", "ورک")],          # Punjabi
    [("Khan", "خان"), ("Yousafzai", "یوسفزئی"), ("Afridi", "آفریدی"), ("Khattak", "خٹک"), ("Mohmand", "مہمند"), ("Orakzai", "اورکزئی")],                                           # Pashtun
    [("Memon", "میمن"), ("Soomro", "سومرو"), ("Jatoi", "جتوئی"), ("Chandio", "چانڈیو"), ("Abro", "ابڑو")],                                                                          # Sindhi
    [("Baloch", "بلوچ"), ("Rind", "رند"), ("Marri", "مری"), ("Raisani", "رئیسانی"), ("Mengal", "مینگل"), ("Jamali", "جمالی")],                                                       # Baloch
    [("Siddiqui", "صدیقی"), ("Qureshi", "قریشی"), ("Ansari", "انصاری"), ("Hashmi", "ہاشمی"), ("Rizvi", "رضوی"), ("Naqvi", "نقوی"), ("Farooqi", "فاروقی"), ("Zaidi", "زیدی")],         # Muhajir
    [("Mir", "میر"), ("Dar", "ڈار"), ("Bhat", "بھٹ"), ("Wani", "وانی"), ("Shah", "شاہ")],                                                                                           # Kashmiri
]

# banks that appear as the other side of a transfer; the statements' own issuer is fictional
COUNTERPARTY_BANKS = ["HBL", "UBL", "MCB", "Meezan Bank", "Bank Alfalah", "Allied Bank", "Askari Bank", "Faysal Bank", "Bank of Punjab", "JS Bank"]
MERCHANTS = ["Imtiaz Super Market", "Carrefour", "Al-Fatah Store", "Chase Up", "Servis", "Hyperstar", "Metro Cash and Carry", "Shell Pakistan", "PSO Station", "Pizza Hut", "KFC", "Daraz"]
UTILITIES = ["LESCO electricity bill", "SNGPL gas bill", "PTCL broadband bill", "WASA water bill"]
LOADS = ["Jazz", "Zong", "Ufone", "Telenor"]
EMPLOYERS = ["salary credit", "salary credit", "monthly salary", "salary and allowances"]


def known(city):
    if city not in CITIES:
        raise ValueError("unknown locale %r (known: %s)" % (city, ", ".join(sorted(CITIES))))
    return CITIES[city]


def grid_origin(city):
    return known(city)["origin"]


def lon_step(city):
    """0.02 degrees of latitude is about 2.2 km: the longitude step is widened by 1 / cos(latitude) so a site is about as far from its east neighbour as from its north one."""
    return 0.02 / math.cos(math.radians(known(city)["origin"][0]))


def area(city, idx):
    return known(city)["areas"][idx % 12]


def site_label(city, idx):
    """What an export writes in its location text column for a site (the area, and the site number the operator would show)."""
    return "%s Site %02d" % (area(city, idx), idx)


def _pick(rng, pool):
    return pool[rng.randrange(len(pool))]


def identity(city, person_n, community, home_site):
    """The identity of one person of the world, the same in every source that names the person. Deterministic in (person_n, community): no random state is shared with the world."""
    rng = random.Random("pk-identity:%d:%d" % (person_n, community))
    woman = rng.random() < 0.28
    given = _pick(rng, FEMALE if woman else MALE)
    surname = _pick(rng, SURNAMES[community % len(SURNAMES)])
    father = _pick(rng, MALE)
    family = rng.randrange(10 ** 7)
    digit = rng.choice((2, 4, 6, 8) if woman else (1, 3, 5, 7, 9))
    house, street = 1 + rng.randrange(480), 1 + rng.randrange(40)
    where = known(city)
    locality = area(city, home_site).split(" (")[0]
    latin_name = "%s %s" % (given[0], surname[0])
    return {
        "name": latin_name, "name_urdu": "%s %s" % (given[1], surname[1]),
        "relation": "d/o" if woman else "s/o", "father": "%s %s" % (father[0], surname[0]), "father_urdu": "%s %s" % (father[1], surname[1]),
        "national_id": "%s-%07d-%d" % (NATIONAL_ID_PREFIX, family, digit), "gender": "F" if woman else "M",
        "address": "House %d, Street %d, %s, %s" % (house, street, locality, where["city"]),
        "locality": locality, "city": where["city"], "province": where["province"],
    }


def narration(rng, channel, direction, other_label):
    """A statement narration in the vocabulary of Pakistani statements for a channel and a direction; other_label is a short tag of the other side (a counterparty index), never an account."""
    bank = COUNTERPARTY_BANKS[hash_index(other_label) % len(COUNTERPARTY_BANKS)]
    ref = "%06d" % rng.randrange(10 ** 6)
    if channel == "IBFT":
        return ("IBFT inward credit from %s, ref %s" if direction == "credit" else "IBFT outward transfer to %s, ref %s") % (bank, ref)
    if channel == "Raast":
        return ("Raast P2P received, ref %s" % ref) if direction == "credit" else ("Raast P2P sent via %s, ref %s" % (bank, ref))
    if channel == "ATM":
        return "ATM cash withdrawal 1LINK, ref %s" % ref
    if channel == "POS":
        return "POS purchase %s, ref %s" % (_pick(rng, MERCHANTS), ref)
    if channel == "Cheque":
        return ("Cheque no %s deposited, clearing" if direction == "credit" else "Cheque no %s cleared") % ref
    if channel == "Cash":
        return "Cash deposit at branch, ref %s" % ref if direction == "credit" else "Cash withdrawal at branch, ref %s" % ref
    if channel == "Branch":
        return "Branch counter transaction, ref %s" % ref
    if channel == "Online":
        return ("Online transfer received, ref %s" % ref) if direction == "credit" else ("%s, ref %s" % (_pick(rng, UTILITIES), ref))
    return "Transaction, ref %s" % ref


def hash_index(text):
    total = 0
    for char in str(text):
        total = (total * 131 + ord(char)) % 1000003
    return total
