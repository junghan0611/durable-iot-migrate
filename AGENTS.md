# AGENTS.md — durable-iot-migrate

## Project

Durable IoT platform migration framework built on Temporal.
Platform-agnostic: core has no vendor names, adapters connect platforms.

## Language

- Code & docs: English
- Go for workers/CLI, SQL for Doltgres schemas

## Architecture

```
core/           → Platform-independent workflows, activities, models
adapters/       → Platform-specific Source/Target implementations
  mock/         → In-memory adapter with error injection (testing)
cmd/            → Worker, API server, CLI binaries
schemas/        → Doltgres migration audit tables
docs/           → Architecture, patterns, examples
deploy/         → Docker Compose for Temporal + Doltgres
office/         → Private vendor-specific notes (gitignored)
.beads/         → Issue tracking (br)
```

## Key Interfaces

- `SourcePlatform` — ListDevices, ListAutomations, UnbindDevice, RebindDevice
- `TargetPlatform` — BindDevice, CreateAutomation, VerifyDevice, UnbindDevice, DeleteAutomation

## Principles

- **Durable Execution**: Every activity is a checkpoint. Crash → resume, not restart.
- **Saga Pattern**: Each forward action has a compensation. Failure → rollback in reverse.
- **Platform Agnostic**: Core knows interfaces, not vendors. Adapters are plugins.
- **Reproducibility**: Doltgres commit + Temporal RunID + git commit = full audit trail.
- **Simplicity First**: Start with the dumbest thing that works. Add complexity when needed.
- **Test First**: 98%+ library coverage required. No real IoT devices needed — mock adapter covers all protocol-level paths.

## Development

```bash
nix develop                       # Go 1.25 + temporal-cli 1.5.1 + psql
go test ./... -cover              # Tests must pass before commit
temporal server start-dev         # Local dev server (gRPC:7233, UI:8233)
go run ./cmd/worker               # Worker with mock adapter
go run ./cmd/cli start <batch-id> # Trigger migration
```

## Testing Strategy

- Mock adapter has error injection: `FailUnbindIDs`, `FailDeviceIDs`, `FailVerifyIDs`, `ErrorVerifyIDs`, `FailRebindIDs`, `FailAutoIDs`
- `mock.GenerateDevices(N, rng)` / `mock.GenerateAutomations(M, devices, rng)` — deterministic random generation
- Temporal `testsuite.TestWorkflowEnvironment` for workflow tests (no server needed)
- Temporal `testsuite.TestActivityEnvironment` for activity tests (heartbeat/context)
- Scale tests: 500 devices + random failures verified
- Every Saga compensation path (unbind→rollback, bind→rollback, verify→rollback) tested independently

## Conventions

- Go standard layout
- `go test ./...` must pass — coverage ≥ 98% for library code
- Temporal workflow functions must be deterministic (no side effects)
- Activity functions handle side effects (API calls, DB writes)
- `math/rand/v2` for random generation (not `math/rand`)
- Commit messages: `type: description` (feat/test/fix/docs/refactor)

## Gotchas

- `SuccessThreshold <= 0` is treated as "use default (0.95)" — set to small positive (e.g., 0.01) for near-zero thresholds
- Temporal test env retries activities per RetryPolicy — `env.OnActivity` mocks must account for retry behavior
- `mock2 "github.com/stretchr/testify/mock"` alias needed when `mock` is already the adapter package

## Issue Tracking (beads_rust)

```bash
br list                          # List issues
br show <id>                     # Show issue detail
br create "title"                # Create issue
br create "title" -p p0 -l "tag1,tag2" -t epic

br update <id> -s in_progress    # Status: open, in_progress, blocked, deferred, closed
br update <id> -p p1             # Priority: p0~p4

# Close — design/acceptance_criteria/notes must be set first (NOT NULL constraint)
br update <id> --design "..." --acceptance-criteria "..." --notes "..."
br close <id>

br comments add <id> "text"      # Add comment (note: "comments add", not "comment")
br sync --flush-only             # Export JSONL before git commit
```
