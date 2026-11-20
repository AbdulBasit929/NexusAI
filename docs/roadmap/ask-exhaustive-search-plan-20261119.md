# Ask: exhaustive, tolerant, runtime-LLM search (plan, 2026-11-19)

Baseline and diagnosis: `docs/data/ask-exhaustive-search-baseline-20261119.md` (30 natural questions, 20% fully answered). Rule: no predefined templates or phrase rules for questions; the model reads the question at runtime; a search returns all occurrences across every source, with the closeness of each match stated.

## Increments (each measured on bank-B before the next; deploy only on the owner's word)

**I-1. "No source named" means every source.** When the text-intent step says `words_in_any_source`, run the governed retrieval over documents, image text and recording transcripts (and the plate readings when the literal is plate-shaped) and merge: one answer, grouped by kind of source, each file named with its count of passages. Removes the clarification dead end. The model's proposal is still validated by the server; no keyword decides anything.

**I-2. Tolerant matching, labelled.** Matching tiers in `pkg/forensictext`: exact (as today); normalised (digit scripts folded: ۰۱۲۳ and ٠١٢٣ to 0123; separators and spacing in numbers and codes ignored; case, Unicode form, Arabic and Urdu letter variants folded); approximate (bounded edit distance for tokens of six or more characters, to absorb reading errors). Every hit says which tier matched; exact hits are never hidden behind approximate ones; approximate hits are shown as such and never counted as exact.

**I-3. The model claims lookups of text.** Rewrite the text-intent instruction (no example questions, as today) so that a question asking where, in which files or evidence, a number, account, name, plate or word appears, is mentioned or is written is a search for words in text, not a table question; questions about counts, durations, who called whom stay with the structured route. Measured on bank-B and on the verified bank so structured questions do not regress.

**I-4. Completeness is real.** Remove the budget stop for small scopes (the search covers the whole case before it declares anything incomplete), keep an explicit state (EXHAUSTIVE or INCOMPLETE with the reason), and list every matching file in the answer text and in the rows, never "and N others".

**I-5. Cross-source identity of the same thing.** A plate read from an image, written in a statement with other digits, or stored as a sighting is one entity: the answer lists all places with the kind of source and the tier; plates keep the "unreviewed model reading" caveat.

## Measures and gates (fixed before building)

Bank-B (new wordings and new probes, written after the build): full recall at least **90%** of questions, mean recall at least **0.95**, precision at least **0.9**, no question that cites nothing where a true file exists, median under 15 s for lookups; the 484-question verified bank: **no regression** (correct stays correct). Time budget per question stays within the engine's existing limit.

## Order and risk

I-1 and I-4 first (largest effect, smallest risk), then I-2, then I-3 (prompt change, the one with regression risk, measured last on both banks). Everything behind existing switches or new ones defaulting off until measured; the API image is rebuilt by the project's recipe and rolled back by tag.

## Status 2026-11-20

I-1, I-2, I-4 and the plate part of I-5 are built behind `FORENSIC_TEXT_SEARCH_ALL` (default off) and measured on a staging copy: `docs/data/ask-exhaustive-search-results-20261119.md`. Gates not met (bank-A 60%, bank-B 74% fully answered, against 20% live); the limit is I-3: the 4B intent model cannot be told, by prompt or by a second question, to separate "where is this number written" from "how many records does this number have" without regressing the intent step. Owner options are listed in the results note. Nothing is deployed.
