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
cmd/            → Worker, API server, CLI binaries
schemas/        → Doltgres migration audit tables
docs/           → Architecture, patterns, examples
deploy/         → Docker Compose for Temporal + Doltgres
office/         → Private vendor-specific notes (gitignored)
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

## Conventions

- Go standard layout
- `go test ./...` must pass
- Temporal workflow functions must be deterministic
- Activity functions handle side effects (API calls, DB writes)
