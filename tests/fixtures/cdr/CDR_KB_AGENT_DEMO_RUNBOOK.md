# CDR Knowledge Base + Agent Demo Runbook

This runbook explains how to demonstrate the Knowledge Base feature honestly and accurately for CDR/IPDR-style data.

## Key Message For The Team

Knowledge Base is an evidence retrieval layer. It stores files and retrieves relevant chunks from uploaded evidence.

Agent + Knowledge Base is an answering layer. The agent can turn retrieved evidence into a cleaner response.

For large CDR analytics, exact calculations should come from a structured CDR parser/query tool, then the agent should explain the result with KB evidence.

## Why We Added Answer Cards / Hints

Raw CDR CSV is not enough for reliable semantic retrieval.

Example:

`Which call lasted the longest?`

To answer this exactly, the system must parse every row and compute the maximum `duration_seconds`. A vector Knowledge Base does not guarantee that. It retrieves text chunks by similarity.

So answer cards are useful for:

- Demo reliability.
- Testing expected answer paths.
- Storing precomputed summaries.
- Making common investigation questions retrievable.

Answer cards are not a replacement for a production CDR query engine.

## Files To Use

Raw evidence:

`tests/fixtures/cdr/cdr_kb_seed.csv`

Search-ready text:

`tests/fixtures/cdr/cdr_kb_seed_rag.txt`

Demo answer index:

`tests/fixtures/cdr/cdr_kb_demo_answer_index.txt`

Atomic answer cards:

`tests/fixtures/cdr/kb_cards/answer_shortest_completed_voice_call.txt`
`tests/fixtures/cdr/kb_cards/answer_target_numbers_923001112222.txt`
`tests/fixtures/cdr/kb_cards/answer_gulberg_calls.txt`
`tests/fixtures/cdr/kb_cards/answer_failed_call.txt`
`tests/fixtures/cdr/kb_cards/answer_longest_call.txt`
`tests/fixtures/cdr/kb_cards/answer_target_923334445555.txt`
`tests/fixtures/cdr/kb_cards/answer_cell_lhr_gul_014.txt`
`tests/fixtures/cdr/kb_cards/answer_roaming_call.txt`

Query packs:

`tests/fixtures/cdr/cdr_kb_sample_queries.md`
`tests/fixtures/cdr/cdr_kb_raw_query_test_pack.csv`
`tests/fixtures/cdr/cdr_kb_raw_queries_only.txt`

## Recommended Demo Collections

Use two collections:

1. `cdr-demo-raw-20260713`
   - Purpose: raw file storage and viewing.
   - Upload: `cdr_kb_seed.csv`.

2. `cdr-demo-cards-20260713`
   - Purpose: KB retrieval demo.
   - Upload: `cdr_kb_seed.csv`, plus the atomic answer cards.

Avoid mixing too many large broad summary files into the same demo collection. Broad chunks can dominate top search results.

## Demo 1: Raw Evidence Upload

Goal: show that KB stores the original CDR evidence.

Steps:

1. Open `http://localhost:3000/app/collections`.
2. Create or open `cdr-demo-raw-20260713`.
3. Upload `tests/fixtures/cdr/cdr_kb_seed.csv`.
4. Open the Entries tab.
5. Click the eye/view button.
6. Confirm the raw CSV content opens.

Say:

`This is the raw uploaded CDR evidence. It remains available for inspection and citation.`

## Demo 2: Knowledge Base Search

Goal: show retrieval, not final reasoning.

Steps:

1. Open `http://localhost:3000/app/collections/cdr-demo-cards-20260713`.
2. Go to Search.
3. For single-answer questions, use `Max Results = 1` to `3`.
4. For list questions, use `Max Results = 5` or `10`.

Good queries:

`Which CDR record has the shortest completed voice call overall?`

Expected evidence:

`CALL-0003`, `duration_seconds=7`, `area=Gulberg`.

`List all target numbers called by 923001112222.`

Expected evidence:

`923334445555`, `923336667777`, `923339999000`, `923331111222`.

`Which calls happened in Gulberg?`

Expected evidence:

`CALL-0001`, `CALL-0003`, `CALL-0007`, `CALL-0009`, `CALL-0012`.

Say:

`This Search tab retrieves relevant evidence chunks. It does not synthesize a final answer. For final answer generation, we use an Agent connected to this KB.`

## Demo 3: Connect An Agent To The KB

Goal: show better user-facing answers.

Steps in UI:

1. Open `Build > Agents`.
2. Click Create/New Agent.
3. Basic Info:
   - Name: `cdr-demo-agent-20260713`
   - Description: `CDR investigation assistant using Knowledge Base evidence`
   - Model: use the available chat model, for example `llama-test` for demo or a stronger chat model if installed.
4. Knowledge Base / Memory settings:
   - Enable Knowledge Base: ON
   - KB Mode: `both` if available, otherwise `auto_search`
   - KB Results: `5` or `10`
5. System Prompt:

```text
You are a telecom CDR investigation assistant.
Use only Knowledge Base evidence for CDR facts.
Return exact call IDs, phone numbers, dates, durations, areas, cell IDs, IMEI, and IMSI exactly as written.
If the question asks for a calculation and the answer is not explicitly present in retrieved evidence, say that a structured CDR calculation tool is required.
Do not invent identities, ownership, guilt, locations, or relationships.
Keep answers concise and cite the relevant call IDs.
```

6. Save/Create the agent.
7. Open the agent chat.
8. Ask:

`Which CDR record has the shortest completed voice call overall? Give exact call ID and duration.`

`List all target numbers called by 923001112222. Give only target numbers and call IDs.`

`Which calls happened in Gulberg? Return call IDs, source, target, and duration.`

Exact config fields for API/import verification:

```json
{
  "name": "cdr-demo-agent-20260713",
  "description": "CDR investigation assistant using Knowledge Base evidence",
  "model": "llama-test",
  "enable_kb": true,
  "kb_mode": "both",
  "kb_results": 10,
  "system_prompt": "You are a telecom CDR investigation assistant. Use only Knowledge Base evidence for CDR facts. Return exact call IDs, phone numbers, dates, durations, areas, cell IDs, IMEI, and IMSI exactly as written. If the question asks for a calculation and the answer is not explicitly present in retrieved evidence, say that a structured CDR calculation tool is required. Do not invent identities, ownership, guilt, locations, or relationships. Keep answers concise and cite the relevant call IDs."
}
```

Important: in the current implementation, an agent's KB collection usually matches the agent name. For easiest demo, create the collection with the same name as the agent, or create the agent first and upload files into the collection that appears for that agent.

Recommended agent demo collection name:

`cdr-demo-agent-20260713`

Upload these into that collection:

1. `cdr_kb_seed.csv`
2. Atomic answer cards from `tests/fixtures/cdr/kb_cards/`

## Will It Work Without Answer Cards?

Partly.

Works reasonably without answer cards:

- Find exact call ID.
- Retrieve rows mentioning a phone number.
- Retrieve rows mentioning an area or cell ID.
- Show raw uploaded evidence.

Not reliable without answer cards or tools:

- Shortest call across all rows.
- Longest call across all rows.
- Count calls by area/date.
- List all targets for a source across large batches.
- Top N calls by duration.
- Any query requiring full-dataset aggregation.

For those, use one of these:

1. Precomputed answer cards for demo or known reports.
2. A structured CDR query tool for production.

## Best Production Architecture

```text
CDR batch upload
  -> raw file stored in KB
  -> structured parser stores rows in DB
  -> agent receives natural-language question
  -> agent calls CDR query tool for exact computation
  -> agent uses KB for evidence/citations
  -> final answer with exact values and source references
```

## What To Tell The Team

Use this wording:

`The Knowledge Base is good for evidence retrieval and source grounding. The Agent is good for converting evidence into a readable answer. For exact CDR analytics at scale, we should add a structured CDR tool/API so calculations are deterministic. The best product is KB + Agent + CDR query tool together.`

## Recommended Presentation Script

1. Show raw CDR upload in KB.
2. Show raw CSV view in Entries.
3. Show KB Search retrieving evidence with natural-language queries.
4. Explain that Search is retrieval, not final answering.
5. Open Agent chat connected to KB.
6. Ask the same question and show cleaner answer.
7. Close with the production architecture: KB for evidence, Agent for interface, structured tool for exact CDR calculations.

