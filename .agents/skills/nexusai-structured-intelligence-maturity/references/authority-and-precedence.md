# Authority, governance, and safe execution

## Precedence

Follow the latest explicit product-owner instruction, then `NEXUSAI_MASTER_DIRECTIVE.md`, `AGENTS.md`, task-relevant `.agents/*.md`, the STIM skill, current roadmap, phase ledger, `NEXUSAI_CONTINUATION.md`, maturity matrix, source, and tests. Safety rules remain non-bypassable. Current source wins over stale historical reports.

## Reuse and scope

Preserve accepted APF query understanding, capability resolution, governed execution, follow-up semantics, multilingual routing, KB/hybrid execution, composition, provenance, security guards, History, Ask, and Analyst Portal behavior. Extend through bounded tested vertical slices; never perform a big-bang rewrite.

Do not reopen APF-3. Carry its deferred capability-guard and synthesis latency into STIM-5 and citation wording into STIM-6.

## Approval boundaries

Require explicit approval before model/dataset/backend downloads, database migration or backfill, retained evidence upload/reprocess, destructive cleanup, worker rebuild, model/profile change, deployment/rebuild, staging, commit, push, or PR. Prepare exact PowerShell deployment commands for the product owner; do not run governed deployment.

Never reset hard, clean the worktree, perform destructive checkout, remove volumes, or discard unrelated changes. Preserve dirty-worktree ownership.

## Living artifacts

Maintain one current roadmap, one phase ledger, one machine-readable STIM maturity matrix and issue register, one current STIM architecture/maturity document, and `NEXUSAI_CONTINUATION.md`. Record exact evidence and status after each meaningful phase; avoid report sprawl.
