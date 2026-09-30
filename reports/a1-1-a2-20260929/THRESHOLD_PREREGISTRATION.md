# A1.1 TABLE HEADER CASING + A2 CITATION TRUTH STATE -- THRESHOLDS

Written 2026-09-29, BEFORE either switch was built into an image or any arm was scored. Two
independent changes, each behind its own switch. They touch disjoint fields (data-grid header strings
vs provenance and citation fields), so they are measured together as the configuration that ships.

## A1.1 `FORENSIC_TABLE_HEADER_CASING`

Template (non-typed) result tables use `humanizeField`, which upper-cases every word of three letters
or fewer. Recorded on the deployed build: "Call END TS", "ROW Hash", "Completed AT", "Sampled NON
Empty", "Imei", "Cnic". The change: result-table headers only get sentence case, real acronyms (IMEI,
MSISDN, CNIC, HTTP, …) and a few one-reading expansions (num → number, ts → timestamp).
`humanizeField` itself (used for metric labels and the plumbing-label check) is unchanged.

## A2 `FORENSIC_CITATION_TRUTH_STATE`

Document, image-text and transcript search results cite derived artifacts but don't carry their stored
`metadata.source_truth_state`, so the UI shows "Source record" for a speech-to-text segment. The
change: the search SQL selects the stored value, and it is copied into the result metadata, the
provenance item and the fact-packet citation, only when present.

The stored truth (read-only census, `nexusai-multimodal-product-acceptance`):

    audio-timestamp-segment      derived_model_observation     11
    image-ocr-observation        derived_model_observation    309
    document-native-text-passage derived_native_text          115
    audio-roman-urdu-segment     derived_text_representation    7

## Arms

    off   both switches OFF (must reproduce reports/column-labels-20260929/on)
    on    both switches ON
    Each: corpus, 14 everyday, 38 pre-flight, 16 relational, plate probe.

## Pass criteria

    corpus      0 verdicts moved; answer_text identical on all 103
    A1.1        data-grid column KEYS and ROW VALUES byte-identical between arms; no header contains
                "END", "ROW", " AT", "NON", "Imei", "Imsi" or "Cnic" as a word in the on arm
    A2          every provenance item citing a derived text artifact carries the stored truth state
                for its artifact type (table above); no records_sql provenance item gains one; the
                fact-packet citations match
    probes      pre-flight 38/38 identical in pass and text; everyday 14 identical; relational 16
                identical; plate probe 8/8 with no leaks

Fail any line -> both switches stay OFF until the failing one is isolated.
