# durable-iot-migrate

**Durable IoT platform migration framework built on [Temporal](https://temporal.io).**

Migrate devices and automations (recipes, rule chains, scenes) between IoT platforms — with automatic retry, rollback, and full audit trail.

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

## The Problem

Migrating IoT devices between platforms is a universal pain:

- **Google IoT Core shutdown (2023)** — Thousands of devices, no standard migration tool
- **Home Assistant** — No automation import/export ([WTH issue](https://community.home-assistant.io/t/wth-is-there-no-way-to-import-export-a-solution/804060) with hundreds of comments)
- **ThingsBoard** — Rule chain export exists, but device binding is separate, no rollback
- **Platform switches** — Homey → HA, proprietary → open source — always manual

The industry standard is **"run a script and pray"**. No checkpointing, no rollback, no audit trail.

## The Solution

Use **durable execution** to make IoT migration reliable:

```
Device 1 ✅ → Device 2 ✅ → Device 3 ✅ → Device 4 💥 crash
                                                    │
                                              (server restarts)
                                                    │
                                          Device 4 resumes ✅
```

- **Temporal** orchestrates the migration as a durable workflow
- **Saga pattern** provides automatic rollback on failure
- **Doltgres** (git-like PostgreSQL) records row-level audit trail
- **Adapters** connect any IoT platform as source or target

## Architecture

```
┌─ core/ ─────────────────────────────────────────────┐
│                                                       │
│  workflows/                                           │
│    ├─ device_migration.go      Phase 1: Device transfer│
│    ├─ automation_migration.go  Phase 2: Automation     │
│    ├─ verification.go          Phase 3: Verify         │
│    └─ rollback.go              Saga compensation       │
│                                                       │
│  activities/                                          │
│    ├─ interfaces.go  ← SourcePlatform / TargetPlatform│
│    ├─ audit.go       Doltgres audit log               │
│    └─ notification.go                                 │
│                                                       │
│  models/                                              │
│    ├─ device.go      Platform-agnostic device model   │
│    └─ automation.go  Platform-agnostic automation     │
│                                                       │
└───────────────────────────────────────────────────────┘

┌─ adapters/ ─────────────────────────────────────────┐
│  homeassistant/    Home Assistant YAML/API            │
│  thingsboard/      ThingsBoard Rule Chain API        │
│  matter/           matter.js direct control          │
│  your-platform/    Implement the interface, plug in  │
└───────────────────────────────────────────────────────┘
```

### Key Interfaces

```go
// SourcePlatform — where devices come from
type SourcePlatform interface {
    ListDevices(ctx context.Context) ([]Device, error)
    ListAutomations(ctx context.Context) ([]Automation, error)
    UnbindDevice(ctx context.Context, deviceID string) error
    RebindDevice(ctx context.Context, deviceID string) error  // compensation
}

// TargetPlatform — where devices go to
type TargetPlatform interface {
    BindDevice(ctx context.Context, device Device) error
    CreateAutomation(ctx context.Context, auto Automation) error
    VerifyDevice(ctx context.Context, deviceID string) (bool, error)
    UnbindDevice(ctx context.Context, deviceID string) error       // compensation
    DeleteAutomation(ctx context.Context, autoID string) error     // compensation
}
```

Implement these two interfaces for your platform. The core handles everything else.

## Saga Pattern — Automatic Rollback

Each migration step has a compensation action:

| Step | Forward | Compensation |
|------|---------|-------------|
| 1 | Unbind from source | Rebind to source |
| 2 | Bind to target | Unbind from target |
| 3 | Verify connectivity | (no compensation needed) |
| 4 | Transfer automation | Delete automation from target |

If step 3 fails after migrating 500 devices, Temporal automatically runs compensations in reverse for all 500 devices.

## Reproducibility

Every migration run is fully auditable:

```
Reproducibility = Doltgres commit hash (pre/post migration snapshot)
                + Temporal RunID (workflow execution history)
                + git commit (migration code version)
                + API version (source/target platform versions)
```

## Gradual Migration (Canary)

IoT devices bind to one platform at a time, so traffic splitting doesn't apply. Instead, use batch-based gradual migration:

```
Batch 1:  10 test devices    → migrate → monitor 72h
Batch 2:  100 pilot devices  → migrate → monitor 1 week
Batch 3:  10% of fleet       → migrate → verify
Batch 4:  50%                → migrate → verify
Batch 5:  remaining 50%      → complete
```

Each batch is an independent Temporal workflow. Auto-halt if success rate drops below threshold.

## Prerequisites

- [Temporal Server](https://docs.temporal.io/self-hosted-guide) (Docker or native)
- [Doltgres](https://github.com/dolthub/doltgresql) (optional, for audit trail)
- Go 1.22+

## Status

🚧 **Early stage** — Architecture defined, interfaces designed, implementation starting.

Contributions welcome. See [AGENTS.md](AGENTS.md) for coding guidelines.

## Related Projects

- [Temporal](https://temporal.io) — Durable execution platform
- [Doltgres](https://github.com/dolthub/doltgresql) — Git-like PostgreSQL
- [HomeAgent](https://github.com/junghan0611/homeagent-config) — Open-source Matter smart home platform
- [Open Home Foundation](https://www.openhomefoundation.io/) — Sustainable smart home ecosystem
- [matter.js](https://github.com/matter-js/matterjs-server) — Matter server for Home Assistant

## License

MIT
