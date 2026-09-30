# B1 CONVERSATION ROUTER AND GLOSSARY -- THRESHOLDS

Written 2026-09-29, BEFORE the switch was built into an image or any arm was scored.

## The defect

On the deployed build, a new user's first three messages were all refusals
(`reports/template-states-20260929/on/adhoc`):

    "hello"                     -> "I could not map that request to a deterministic forensic workflow."
    "What can this system do?"  -> the same refusal
    "What is a CDR?"            -> "A bounded general definition is unavailable for that term."

The glossary also matched substrings, so "rag" answered for "storage" and "paragraph".

## The change, `FORENSIC_CONVERSATION_ROUTER` (default off)

- A message that is only a greeting, or only a capability question, goes to the existing product-help
  class. The whole message must match, a message with evidence context is never rerouted, and a class
  the classifier already decided is kept.
- The product-help text is built from the capability registry. Operational families are named as
  supported, and limited families as "model observations to review". Foundation and planned families
  are not offered. No case value is stated.
- The glossary adds CDR, ANPR, LAC, CGI, cell ID, BTS, CNIC, SIM, USSD, EXIF, SHA-256 and speech-to-text,
  and matches whole words only.

Offline census (unit test): with the switch on, **0 of the 103 corpus questions change request class**.

## Arms

    off   B1 OFF, with every other switch in its shipped state (A4 as measured)
    on    B1 ON
    Each: corpus, 14 everyday, 38 pre-flight, 16 relational, plate probe, and the 15-question
    conversation probe (conversation_probe.py).

## Pass criteria

    conversation  on arm 15/15: G1-G3 and C1-C3 are terminal product help with the expected text;
                  D1-D4 are terminal definitions; D5 ("storage") stays "unavailable"; N1-N4 are NOT
                  terminal; N3 states 8,642 and N4 states 923009998887
    corpus        0 verdicts moved; answer_text identical on all 103
    probes        pre-flight 38/38 identical; relational 16 and plate 8/8 identical; everyday 14
                  identical except the three conversational rows (greeting, capability, concept),
                  each read
    safety        no product-help or definition text contains a case value (3+ digit number, plate,
                  file name)

Fail any line -> B1 stays OFF.
