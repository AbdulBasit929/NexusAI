"""Reserved ranges for synthetic data, and the pure helpers that decide whether a value could be real.

Everything here is a constant or a pure function, so the profiler, the scanner and the synthetic-golden generators agree on what
"cannot be a real value" means. See docs/data/real-data-handling-policy.md, section 5.

What is genuinely unreal, and what is only conventional:

* Documentation and test address blocks (RFC 5737, RFC 3849) and the private, loopback and link-local ranges can never be a
  subscriber's public address. These are safe.
* The ITU test network (mobile country code 001, network code 01) cannot identify a subscriber. Safe for IMSI-like values.
* A CNIC-like number whose first five digits are zeros has no valid province or district code. Safe by construction.
* Mobile numbers are the weak point: no block of valid-shaped Pakistani mobile numbers is guaranteed unassigned. The reserved
  block below is a project convention that keeps fixtures recognisable and lets the scanner tell them from real numbers; a
  fixture number may coincide with a real subscriber by chance. Never call, message or look up a synthetic number.
* TAC prefix 00 is documented by the GSM Association as reserved for test devices (verify against TS.06 before relying on it).
"""
import ipaddress
import re

DOC_IPV4 = ("192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24")
DOC_IPV6 = ("2001:db8::/32",)

# Not routable on the public internet, so a fixture may use them freely.
NON_PUBLIC_IPV4 = (
    "0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.168.0.0/16",
    "224.0.0.0/4", "240.0.0.0/4",
)
NON_PUBLIC_IPV6 = ("::/128", "::1/128", "fc00::/7", "fe80::/10", "ff00::/8")

SYNTHETIC_MSISDN_PREFIX = "9230000"  # 923000000000 to 923000099999: 12 digits, country code 92 included
TEST_PLMN = "00101"  # mobile country code 001 (ITU test), network code 01
TEST_TAC_PREFIX = "00"
SYNTHETIC_CNIC_PREFIX = "00000"

_DOC_NETS = [ipaddress.ip_network(n) for n in DOC_IPV4 + DOC_IPV6]
_NON_PUBLIC_NETS = [ipaddress.ip_network(n) for n in NON_PUBLIC_IPV4 + NON_PUBLIC_IPV6]


def is_reserved_ip(text):
    """True for documentation, private, loopback, link-local, multicast and unspecified addresses (never a real public host)."""
    try:
        addr = ipaddress.ip_address(text.strip())
    except ValueError:
        return False
    for net in _DOC_NETS + _NON_PUBLIC_NETS:
        if addr.version == net.version and addr in net:
            return True
    return False


def canonical_msisdn(text):
    """Return the 12-digit national form (92 + 10 digits) of a Pakistani mobile number, or None if the text is not one.

    Accepts +92..., 0092..., 92..., 03xx... and 3xx... with spaces or dashes. Landlines and other countries return None.
    """
    digits = re.sub(r"[\s-]", "", text.strip())
    if digits.startswith("+"):
        digits = digits[1:]
    if not digits.isdigit():
        return None
    if digits.startswith("0092"):
        digits = digits[2:]
    if digits.startswith("92") and len(digits) == 12 and digits[2] == "3":
        return digits
    if digits.startswith("03") and len(digits) == 11:
        return "92" + digits[1:]
    if digits.startswith("3") and len(digits) == 10:
        return "92" + digits
    return None


def is_reserved_msisdn(text):
    canon = canonical_msisdn(text)
    return bool(canon) and canon.startswith(SYNTHETIC_MSISDN_PREFIX)


def luhn_ok(digits):
    """Luhn check as used for IMEI numbers (15 digits including the check digit)."""
    if not digits.isdigit() or len(digits) < 2:
        return False
    total = 0
    for i, ch in enumerate(reversed(digits)):
        n = int(ch)
        if i % 2 == 1:
            n *= 2
            if n > 9:
                n -= 9
        total += n
    return total % 10 == 0


def luhn_check_digit(body):
    """The digit that makes body + digit pass luhn_ok."""
    for d in range(10):
        if luhn_ok(body + str(d)):
            return str(d)
    raise ValueError("no check digit for %r" % body)


def is_test_imei(digits):
    return len(digits) == 15 and digits.isdigit() and digits.startswith(TEST_TAC_PREFIX) and luhn_ok(digits)


def is_test_imsi(digits):
    return len(digits) == 15 and digits.isdigit() and digits.startswith(TEST_PLMN)


# Generators for synthetic goldens. Each takes a small integer and returns a value from a reserved range.
def synthetic_msisdn(n):
    if not 0 <= n < 100000:
        raise ValueError("n must be below 100000")
    return SYNTHETIC_MSISDN_PREFIX + "%05d" % n


def synthetic_imsi(n):
    if not 0 <= n < 10 ** 10:
        raise ValueError("n must be below 10**10")
    return TEST_PLMN + "%010d" % n


def synthetic_imei(n):
    if not 0 <= n < 10 ** 12:
        raise ValueError("n must be below 10**12")
    body = TEST_TAC_PREFIX + "%012d" % n
    return body + luhn_check_digit(body)


def synthetic_ipv4(n):
    """An address from the documentation blocks (762 usable addresses)."""
    blocks = ("192.0.2.", "198.51.100.", "203.0.113.")
    n = n % (254 * len(blocks))
    return blocks[n // 254] + str(n % 254 + 1)


def synthetic_ipv6(n):
    return "2001:db8::%x" % (n + 1)


def synthetic_cnic(n):
    """A CNIC-shaped value with an invalid province and district prefix (00000-xxxxxxx-x)."""
    return "%s-%07d-%d" % (SYNTHETIC_CNIC_PREFIX, n % 10 ** 7, n % 10)
