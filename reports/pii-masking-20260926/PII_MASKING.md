# PII MASKING AT PROJECTION — built 2026-09-26. NOT deployed, NOT measured live.

    SWITCH   FORENSIC_PII_MASKED_PROJECTION   default OFF
    SECRET   FORENSIC_PII_ALIAS_SECRET        required; masking FAILS CLOSED without it
    SCOPE    the two plate-candidate fields only. The five OCR/transcript fields
             stay WITHHELD, and the three subscriber fields have no scheme.

Implements the accepted WI-LAYER-7 curation ruling
([`semantic_layer/README.md`](../../semantic_layer/README.md), "PII projection ruling").

---

## 1. What the ruling said, and the clause that shaped the design

> `anpr_model_observation.plate_text`, `video_anpr_plate_group.plate_text` — `PII`/`MASKED`:
> stable alias. **Replace the whole value with one opaque, case-scoped alias shared across both
> families. Revealing a prefix, suffix or length is not approved.**

That last sentence rules out the `Plate candidate ••••-A7C2` shape an earlier handoff proposed. A
trailing fragment of a plate is not a safe remainder, and neither is its length.

The five free-text fields are `WITHHELD`, not masked: token masking was ruled insufficient because
OCR and transcript text carry names, addresses and contextual identifiers that no token syntax
recognises. The bounded patterns finding zero CNIC-, email-, IPv4- or IBAN-like rows **does not
make the remainder safe**, and the ruling says so explicitly.

## 2. Why the alias is keyed, and why it fails closed

**Plate strings are LOW ENTROPY — that is the stated reason last-four was rejected, and it applies
with equal force to an unkeyed hash.** A plausible registration space is a few million strings, so
`sha256(collection || plate)` is reversible by enumeration in seconds by anyone holding an alias
and the source file. An unkeyed digest is not an alias; it is the plate in another encoding.

So the alias is an HMAC under `FORENSIC_PII_ALIAS_SECRET`, and **when that secret is absent the
field is not admitted to the catalogue at all** — masking refuses rather than degrading to an
unkeyed digest. A scheme that silently weakens while still labelled MASKED is worse than none. Same
fail-closed posture as `FORENSIC_API_AUTH_REQUIRED`, and for the same reason: on 2026-09-17 a
missing value was treated as permission to proceed and the forensic API served real evidence
unauthenticated for a whole session.

Properties, each asserted by test:

    opaque        no prefix or suffix of the plate, of any length >= 2, survives
    fixed width   10 chars regardless of the plate behind it — length discloses nothing
    stable        the same plate yields the same alias within a case
    shared        both plate families declare observation.normalized_plate_text, so the
                  same vehicle reads as the same alias from a still or a video group
    case-scoped   the same plate in another case yields an unrelated alias
    keyed         a different secret yields a different alias

Separator and case differences fold to one alias: `ABC-123`, `abc 123` and `ABC123` are the SAME
vehicle, and two aliases would split one subject into two — a fabricated distinction, the same
class of error as merging two into one.

## 3. A BUG THIS INTRODUCED, AND THE TEST THAT CAUGHT IT

The first version admitted **every** `PII`/`MASKED` field. The layer has five:

    anpr_model_observation.plate_text     ruled, scheme implemented
    video_anpr_plate_group.plate_text     ruled, scheme implemented
    subscriber.full_name                  NOT ruled, no scheme
    subscriber.cnic                       NOT ruled, no scheme  <-- a national identity number
    subscriber.cnic_last4                 NOT ruled, no scheme

So `subscriber.cnic` reached the dynamic catalogue and would have been run through the **plate**
aliaser, labelling a national ID "Plate candidate …". Caught by
`TestWI4NoPIIFieldReachesTheDynamicCatalogue`, which was already there.

The fix is a scheme whitelist: the question is not *"is this field MASKED?"* but **"can this field
actually be masked?"** A field whose scheme is unimplemented stays withheld, and adding a scheme is
a deliberate act with its own ruling rather than a consequence of curation setting a redaction
value. Defence in depth — admission refuses, and `maskCuratedValue` refuses again if one ever
reaches it.

## 4. Where masking is applied

Every place a value can reach an analyst:

- **group labels** — grouping by a masked field puts the raw value in the row key, which is the
  same disclosure as projecting it; "how many reads per plate" would list every plate.
- **projection cells**.
- **sorting is DISABLED for masked fields.** This is the subtle one: SQL orders by the RAW value
  while the analyst sees aliases, so an ordered list of aliases discloses the raw lexical order of
  the plates behind them — enough, over a few queries, to reconstruct them. The alias hides the
  value; the ordering would hand it back.

## 5. Evidence

    go build                                                    OK
    suite, defaults                                             ok
    suite, shipped switches                                     ok
    suite, shipped + masking ON (with secret)                   ok
    suite, masking ON with NO secret (fail closed)              ok

**Database-backed, on the real path** — a unit test over a hand-written value proves the alias
function, not the PATH:

    masked GROUP BY over forensics.video-anpr-plate-group/v1
    24 group labels, all aliased
    24 distinct retained plates, NONE present raw anywhere in the payload

The assertion scans the **whole serialised payload**, not just the cells the test knows about: a
leak through a citation, a metadata echo or a field-catalogue entry is still a leak. The oracle is
the actual `normalized_plate_text` values read straight from the table.

Grouping by plate on `anpr_model_observation` is refused by the existing **group-cardinality guard**
(307 > 100). That guard is correct and was left alone; the 24-group video family exercises the same
masking path within it.

## 6. What this does NOT do, and one decision it forces

- **Not deployed and not measured live.** Switch is off; no live suite has been scored with it on.
- **Derived-row PROJECTION is still refused** for want of a derived provenance mapping. So this
  enables masked GROUPING and DISTINCT, not row listing.
- **The three subscriber fields remain withheld.** Masking a CNIC or a person name needs its own
  ruling and its own scheme.

**THE DECISION IT FORCES.** H1 "Which plates were read from the videos?" is an honesty probe whose
pre-registered threshold is CLARIFY. With masking ON it becomes answerable **in aliases** — which is
precisely what masking is for, and arguably the better answer: *"24 plate candidates were read,
shown as case-scoped aliases because plate text is restricted."*

That is a real behavioural change to a probe with a written threshold. It must not arrive as a side
effect of enabling a switch. **Before masking is turned on, H1's expected verdict has to be re-ruled
and the threshold rewritten** — and the tests pin the shipped posture explicitly so it cannot drift
in by default.
