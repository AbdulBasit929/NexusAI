# A4 A COUNT OF A FIELD SAYS WHICH FIELD IT COUNTED -- THRESHOLDS

Written 2026-09-29, BEFORE the switch was built into an image or any arm was scored.

## The defect

Three ungrouped distinct counts on the deployed build answer without naming what was counted:

    CDR-08           "How many unique phone numbers appear as callers in the CDRs?"
                     -> "The count across CDR records is 10."      (COUNT_DISTINCT cdr.msisdn)
    ANPR-05          "How many distinct license plates were captured?"
                     -> "The count across ANPR sightings is 6."    (COUNT_DISTINCT anpr.plate_number)
    H1-CDR-HANDSETS  -> "The count across CDR records is 4,997."   (COUNT_DISTINCT cdr.imei)

CDR-08 matters beyond wording. The plan counted distinct SUBSCRIBER numbers, not obviously the same as
"callers". Naming the field makes that interpretation visible rather than hidden.

A second gap with no corpus question: an ungrouped COUNT of a field counts only the rows where that
field is present, but it was stated as "There are N CDR records in this case", which reads as every
record.

## The change, `FORENSIC_COUNT_NAMES_FIELD` (default off)

- Ungrouped COUNT_DISTINCT gives "There are 10 distinct subscriber number values across CDR records."
- Ungrouped COUNT of a field gives "8,000 CDR records have a device IMEI value."
- The curated display name is used, and an acronym is never lower-cased ("device IMEI").
- COUNT(*) is unchanged, which includes the plate-read search. Grouped breakdowns are unchanged.

## Arms

    off   A4 OFF, with A1.1, A2 and A3 in their shipped or measured state
    on    A4 ON
    Each: corpus, 14 everyday, 38 pre-flight, 16 relational, plate probe.

## Pass criteria

    targets     CDR-08, ANPR-05 and H1-CDR-HANDSETS stay CORRECT, and their text names the field:
                "distinct subscriber number values", "distinct plate number values", "distinct
                device IMEI values"
    corpus      no other verdict moves; answer_text changes ONLY on ungrouped COUNT_DISTINCT or
                COUNT(field) plans, and every changed text is read
    inert off   the off arm reproduces the build it runs on (verdicts and answer_text)
    probes      pre-flight 38/38 with no pass->fail; plate 8/8 with no leaks and plate texts
                identical (COUNT(*) is untouched); everyday and relational changes only on the same
                plan shapes, each read
    data grid   keys, rows and headers byte-identical between arms (A4 changes text only)

Fail any line -> A4 stays OFF.
