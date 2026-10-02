# The goal: any question answered at run time, from the database, with no predefined answers (2026-10-02)

Written from what the product owner has already said, plus a read of the code and reports. Nothing here is implemented.

## What the owner has asked for, and where it is recorded

| Statement | Where |
|---|---|
| An LLM plans the query from the user's intent and does not depend on predefined templates. Any reasonable English question about the case data. | `NEXUSAI_TL_DIRECTIVE_PROMPT_20260918.md` §0 |
| The LLM classifies the question, writes the query against the live schema, runs it and presents the results. No per-question code, no answer templates. The LLM also handles greetings, concepts and product help. | `docs/work/RUNTIME_QUERY_PLAN_20260929.md` |
| Every real question is answered by queries built at run time, not predefined templates. | `docs/work/ROADMAP_20260929.md` (G1, B0 to B2r) |
| Do not add operation templates; the catalogue is frozen and will be deleted. | `AGENTS.md` |
| Answers come from the database; the model narrates and never computes numbers. Abstention is a success, a confident wrong answer never is. | `NEXUSAI_CONTINUATION.md` §4, §5 |
| Today (2026-10-02): the same, plus chatbot behaviour for greetings, conversation and concepts. | this session |

## Where it stands today (measured, from the repository)

**Data questions.** With templates off, 68 of the 103-question corpus are correct and 4 wrong (`reports/templates-off-20260929/RESULT.md`).
The split of the 70 correct answers with templates on: 30 from an LLM-written plan, 25 from the run-time compiler, 11 from keyword
templates (8 of them document, transcript and image text search), 4 terminal. Caveats that bind: the corpus is regression, not
capability (first held-out run: 0 wrong on the corpus, 7 of 13 wrong on held-out); abstention is 22.6%.

**Conversation, greetings, concepts.** This is the weakest part and it is itself canned:

- `api/forensic_records/terminal_request_contract.go` answers concepts from a hard-coded list of 8 definitions (IMSI, IMEI, ICCID,
  MSISDN, IPDR, OCR, RAG, similarity score). Anything else returns the fixed text "A bounded general definition is unavailable for that term."
- On the deployed build, "hello", "What can this system do?" and "What is a CDR?" were all refusals (`reports/conversation-router-20260929/`).
- The fix that was built (`FORENSIC_CONVERSATION_ROUTER`, default off) is keyword matching plus a bigger glossary. It does not use the LLM,
  so it does not meet the owner's direction and is parked.

## What "done" means

1. **Data questions.** Any reasonable question about a case's data, in phrasing nobody wrote for the system, gets one of: a correct
   answer computed by SQL or search over the database, a precise question back, or an honest "not in the evidence". Never a plausible
   number the database did not produce.
2. **Conversation.** Greetings, thanks, small talk, "what can you do", follow-ups and general or conceptual questions ("what is a CDR?",
   "how does tower triangulation work?") get a natural LLM answer, in the voice of an assistant.
3. **The line between them, enforced by the server and not by the prompt.** A reply that contains a case fact must come from a
   query result. A general answer is labelled as general and states nothing about the case. A data question is never answered from the
   model's memory. This is the one risk a free-chat model adds: it will happily invent a plausible answer.
4. **No templates in the path.** Templates retire family by family only when templates-off is at least as correct with 0 new wrong.

## Proposed design (one front door)

One enum-constrained classification call decides the request class, using the existing grammar mechanism so the model cannot invent a class:

| Class | Handled by |
|---|---|
| data question | the existing governed compiler path (plan, validate, SQL, verify, narrate) |
| follow-up | same, with unexpired context issued by the server |
| conversation (greeting, thanks, small talk) | a short LLM reply with a tiny prompt: no schema, no case data, so it is fast |
| concept / general knowledge | LLM reply labelled "general, not from your case"; no case facts allowed |
| product help | grounded in the capability registry (families, formats, limits), so it never over-claims |
| unsafe or out of scope | polite refusal |

A message that mixes the two ("hello, how many calls did 0300... make?") is split: the data part goes down the data path.

## Constraints that shape the order of work

- CPU only, about 7 tok/s writing. A 60-token chat reply is about 9 s plus reading the prompt; a tiny prompt keeps it there. A data
  question that needs a generated plan is 72 to 138 s today. Speed work (`L1B_SHARED_PREAMBLE_SPEC_20261002.md`) matters more for this goal than any model swap.
- Repo rules: no new templates, no model swaps outside the written decision rule, thresholds written before measuring, one switch per
  behaviour, default off.

## Proposed order

1. Measure what the system really does on new questions: a fresh set of unseen data questions plus a conversation set (greetings,
   small talk, concepts, product help, mixed, and traps such as "what was the number I called last week?" with no such evidence), written before running. Capability claims cite this set, not the corpus.
2. L1b prompt layout (speed), behind a switch.
3. LLM front door and conversation replies, behind a switch, against pre-registered thresholds, including: 0 case facts in any non-data reply; 0 data questions answered without a query result.
4. Close the data-question gaps the fresh set exposes, root cause by root cause.
5. Retire templates family by family.

## Decisions needed from the product owner

1. Confirm this reading of the goal (especially item 3 of "done": conversational answers may never state case facts).
2. Who writes the fresh question sets? They must not be written from the system's own corpus. The owner or analysts are best.
3. May the conversational and concept replies use the same 4B model for now? A different model is only evaluated under the written decision rule.
4. Order: speed (L1b) first, or the conversation front door first?
