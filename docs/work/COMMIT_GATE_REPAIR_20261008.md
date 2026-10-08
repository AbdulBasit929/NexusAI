# The Go commit gate cannot pass in this repository: what is wrong and how to repair it (2026-10-08)

**Why this note exists.** Three Go commits on the lane branch needed a one-time `--no-verify` (the product owner authorised each: `b111b585`, `93f1489d`, `fbafd02b`), and `AGENTS.md` forbids bypassing the hook. Every Go commit will fail the hook until the causes below are repaired. Nothing here is specific to the governed SQL lane; the lane's own package passes its checks (`golangci-lint` 0 issues, `go vet` clean, specs green).

## What the hook runs

`.githooks/pre-commit`, when a `.go` file is staged: (1) `make lint LINT_NEW_FROM=<ref>`, a repo-wide `golangci-lint`; (2) `make test-coverage-check`, the coverage ratchet (baseline `coverage-baseline.txt`, 48.5%). It measures `./pkg`, `./core` and `tests/e2e` only; `api/` is not in it, so no lane change can move it.

## The causes, as found

| # | Cause | Where | Effect |
|---|---|---|---|
| 1 | `make test-coverage-check` depends on `test-coverage`, which depends on `prepare-test`, which builds `tests/e2e/mock-backend`. That directory does not exist (`tests/e2e/` holds only `distributed/`); it has been missing since the initial import | `Makefile` (`prepare-test: protogen-go build-mock-backend`) | the coverage gate fails on every Go commit, whatever the code |
| 2 | `tests/e2e/distributed` does not compile: five call sites use the old argument lists | `agent_distributed_test.go:124,126` and `sse_routes_test.go:86` (`PublishStatus(agent, user, status)`; the function is now `PublishStatus(agentName, userID, messageID, status string, caseIDs ...string)`); `agent_distributed_test.go:157` and `agent_native_executor_test.go:395` (`CancelExecution(agent, user, messageID)`; now `CancelExecution(agentName, userID, caseID, messageID string)`) | `golangci-lint` reports a type-checking error and exits 7 even with "0 issues"; the package has no build tags, so this fails on Linux too. Already listed in `LINT_DEBT_20261005.md` |
| 3 | 19 packages under `backend/go/*` call `purego.Dlopen`, which exists on Linux and macOS only; a cgo-only package (`backend/go/supertonic`, onnxruntime) fails when cross-checking as Linux | `make lint` over `go list ./...` | on Windows `golangci-lint` cannot type-check them (exit 7). Not a defect: a Windows-only limit |

## What is available on this laptop

A Linux environment exists: WSL 2 has an `Ubuntu` distro (stopped) beside `docker-desktop`. A hook run there removes cause 3 and shows what else fails.

## Repair, in order, with what needs the product owner

1. **Cause 2, mechanical.** Add the missing argument at the five call sites (a message id for `PublishStatus`, a case id for `CancelExecution`) and confirm the package compiles with `go vet ./tests/e2e/distributed/`. Go change, small; the compile is heavy, so run it when no measurement is running. Needs no approval beyond the commit route in step 4.
2. **Cause 1, a decision.** Either restore `tests/e2e/mock-backend` from the upstream LocalAI source (a download: **needs the owner's approval** with filename, source and size stated first), or make `build-mock-backend` skip when the directory is absent. The second changes what the gate builds, so it is the owner's call and must not lower the baseline or widen its tolerance (`AGENTS.md`).
3. **Cause 3, in Linux.** In the Ubuntu distro: install Go 1.26, `golangci-lint` and `protoc` (**downloads: the owner's approval**), then run the two hook commands exactly as the hook does, with the repository checked out inside the distro's own file system (not through `/mnt/c`, which is slow). The coverage run builds and runs the `pkg` and `core` suites: expect a long run and a large memory use; do it when no measurement is running.
4. **Commit route.** Land steps 1 and 2 as their own commits made from the Linux environment with the hook running, with no bypass. After that, a lane Go commit goes through the hook there.
5. **Until then**, a Go commit on this laptop needs the owner's explicit one-time authorisation, stated in the commit message (as the three above do).

## What this does not change

The lane's own checks are the ones that matter for correctness and they are run by hand on every change: `golangci-lint run --new-from-merge-base=master ./api/forensic_records/`, `go vet` for Windows and `GOOS=linux`, the lane specs with and without a database, and a mutation check that the new specs fail when the change is undone.
