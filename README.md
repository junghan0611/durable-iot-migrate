# durable-iot-migrate

**Durable IoT platform migration framework + multi-platform automation converter.**

Migrate devices and automations between IoT platforms with automatic retry, rollback, and semantic verification. Convert automation recipes across Home Assistant, SmartThings, Google Home, Tuya, and Homey.

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

## Two Axes of "Durable"

| Axis | What's preserved | Mechanism |
|------|-----------------|-----------|
| **Durable execution** | Migration process survives crashes | Temporal workflows + Saga rollback |
| **Durable semantics** | Automation meaning survives conversion | Expr AST + equivalence verification |

Both are "durable". One is infrastructure, the other is logic. A flexible system that preserves structure IS durable.

## The Problem

Migrating IoT devices between platforms is a universal pain:

- **Google IoT Core shutdown (2023)** — Thousands of devices, no standard migration tool
- **Home Assistant** — No automation import/export ([WTH issue](https://community.home-assistant.io/t/wth-is-there-no-way-to-import-export-a-solution/804060) with hundreds of comments)
- **Platform switches** — Homey → HA, proprietary → open source — always manual
- **No semantic verification** — "Did the converted automation actually mean the same thing?"

The industry standard is **"run a script and pray"**. No checkpointing, no rollback, no verification.

## Multi-Platform Automation Converter

Every IoT platform uses trigger → condition → action. Only the syntax differs:

```
HA YAML ───→ Parser ──→ core.Automation ──→ Emitter ──→ SmartThings JSON
Tuya JSON ──→ Parser ──→ core.Automation ──→ Emitter ──→ Google Home YAML
Homey JSON ─→ Parser ──→     (Expr AST)  ──→ Emitter ──→ HA YAML
```

### 5 Platforms Supported

| Platform | Format | Parser | Emitter | Status |
|----------|--------|--------|---------|--------|
| **Home Assistant** | YAML | ✅ | ✅ | Full round-trip |
| **SmartThings** | JSON (Rules API) | ✅ | — | Parse only |
| **Google Home** | YAML (Scripted) | ✅ | — | Parse only |
| **Tuya** | JSON (Scene API) | ✅ | — | Parse + Expr |
| **Homey** | JSON (Flow API) | ✅ | — | Parse only |

Cross-conversion verified: 5×5 compatibility matrix (20 pairs).

### Standard Type System

```
Triggers (5):  device_state, schedule, sun, webhook, geofence
Conditions (6): device_state, time, numeric, zone, and, or
Actions (5):   device_command, notify, delay, scene, webhook
```

All platforms map to these standard types. Platform-specific details preserved in `Config` maps and structured `Expr` trees.

### Expr — Structured Expression AST

The core differentiator. `map[string]any` is fragile flexibility. Expr is durable flexibility:

```go
// "Temperature above 25°C" — same meaning, 5 different platform syntaxes:
//   Tuya:  dp_id=4, comparator=">", value=25
//   HA:    trigger: numeric_state, above: 25
//   ST:    if.greaterThan(device.main.temperature, 25)
//   All → GtExpr(State("sensor", "temperature"), Lit(25))

trigger := GtExpr(State("sensor", "temperature"), Lit(25))
assert.True(t, IsValid(trigger))               // structural check
assert.True(t, StructuralEquiv(tuya, ha))       // same shape
assert.True(t, EquivWithMapping(tuya, ha, m))   // same meaning
```

22 operators: 9 comparison (eq/ne/gt/lt/between/in/contains) + 5 combinators (and/or/not/seq/parallel) + 4 actions (command/delay/notify/scene) + 3 leaf types (literal/state_ref/time_ref).

## Durable Migration (Temporal)

```
Device 1 ✅ → Device 2 ✅ → Device 3 ✅ → Device 4 💥 crash
                                                    │
                                              (server restarts)
                                                    │
                                          Device 4 resumes ✅
```

### Saga Pattern — Automatic Rollback

| Step | Forward | Compensation |
|------|---------|-------------|
| 1 | Unbind from source | Rebind to source |
| 2 | Bind to target | Unbind from target |
| 3 | Verify connectivity | — |
| 4 | Transfer automation | Delete from target |

If step 3 fails after migrating 500 devices, Temporal runs compensations in reverse for all 500.

### Safety Classes

Devices are classified by safety criticality:

| Class | Devices | Policy |
|-------|---------|--------|
| **Critical** (~19%) | Camera, lock, smoke detector | Zero-tolerance, human approval gate |
| **Important** (~5%) | Thermostat, garage door | Auto-rollback on any failure |
| **Normal** (~76%) | Light, plug, sensor | Batch threshold halt |

### Fleet Scale

```go
fleet := mock.GenerateFleet(100_000, 0.3, rng)
// → 1.06M devices (202K safety-critical) in 1.5 seconds
// Power-law distribution: 60% have 1-5 devices, 5% have 40-100
```

No real hardware needed. All simulation is deterministic via PCG seeds.

## Architecture

```
┌─ core/ ───────────────────────────────────────────────┐
│  models/        Device, Automation, Account, SafetyClass│
│  expr/          Expr AST, Validate, Equiv, DeviceRef    │
│  converter/     Parser/Emitter interfaces, type system  │
│  activities/    Saga migration activities               │
│  workflows/     DeviceMigrationWorkflow (2-phase)       │
└─────────────────────────────────────────────────────────┘

┌─ converters/ ─────────────────────────────────────────┐
│  homeassistant/  YAML parser + emitter (round-trip)    │
│  smartthings/    JSON Rules API parser                  │
│  google/         YAML Scripted Automation parser        │
│  tuya/           JSON Scene API parser (+ Expr)         │
│  homey/          JSON Flow API parser                   │
└─────────────────────────────────────────────────────────┘

┌─ adapters/ ────────────────────────────────────────────┐
│  mock/           Fleet generator + 6 error injections   │
│  your-platform/  Implement SourcePlatform/TargetPlatform│
└─────────────────────────────────────────────────────────┘
```

### Key Interfaces

```go
// Converter layer: parse any format → core.Automation → emit any format
type Parser interface {
    Platform() string
    ParseBytes(data []byte) ([]models.Automation, error)
}
type Emitter interface {
    Platform() string
    EmitBytes(autos []models.Automation) ([]byte, error)
}

// Migration layer: move devices between platforms
type SourcePlatform interface {
    ListDevices(ctx context.Context) ([]Device, error)
    ListAutomations(ctx context.Context) ([]Automation, error)
    UnbindDevice(ctx context.Context, deviceID string) error
    RebindDevice(ctx context.Context, deviceID string) error  // compensation
}
type TargetPlatform interface {
    BindDevice(ctx context.Context, device Device) error
    CreateAutomation(ctx context.Context, auto Automation) error
    VerifyDevice(ctx context.Context, deviceID string) (bool, error)
    UnbindDevice(ctx context.Context, deviceID string) error       // compensation
    DeleteAutomation(ctx context.Context, autoID string) error     // compensation
}
```

## Quick Start

```bash
# 1. Enter dev environment (Go 1.25 + Temporal CLI 1.5 + psql 16)
nix develop

# 2. Run tests (no server needed)
go test ./... -cover

# 3. Start Temporal dev server
temporal server start-dev

# 4. Start worker + trigger migration
go run ./cmd/worker &
go run ./cmd/cli start my-first-batch
```

## Test Coverage

**154 tests, 86% coverage, 12 packages.**

```
core/models      100.0%   — Device, Automation, Account, SafetyClass
core/workflows   100.0%   — including 500-device scale test
core/activities   97.8%   — every Saga compensation path
core/converter    97.3%   — 5×5 compatibility matrix
core/expr         76.3%   — validation, equivalence, cross-platform
converters/*      79-93%  — 5 platform parsers + HA emitter
adapters/mock     98.1%   — fleet generation + error injection
```

Error injection: `FailUnbindIDs`, `FailDeviceIDs`, `FailVerifyIDs`, `ErrorVerifyIDs`, `FailRebindIDs`, `FailAutoIDs`.

## Roadmap

### P1 — Next
- [ ] Conversion fidelity test suite — round-trip Expr verification, coverage matrix, property-based testing (br: `1wd`)
- [ ] Realistic failure simulation — API rate limit, network latency, partial outage (br: `2nk`)
- [ ] Doltgres audit activity — row-level migration log with git-like diff (br: `1gu`)

### P2 — Planned
- [ ] IAIF spec — IoT Automation Interchange Format specification (br: `31c`)
- [ ] RefMapper — cross-platform attribute normalization (dp_1 ↔ state ↔ switch.switch)
- [ ] Emitters for SmartThings, Google Home, Tuya, Homey
- [ ] Fan-out concurrent device migration (br: `58i`)
- [ ] Temporal Query handler for real-time progress (br: `2ph`)
- [ ] ThingsBoard adapter (br: `3pk`)
- [ ] CI with coverage gate (br: `12w`)

### P3 — Future
- [ ] matter.js direct adapter (br: `16g`)
- [ ] Notification activity — webhook/chat alerts (br: `2gx`)

## History

| Date | Milestone | Stats |
|------|-----------|-------|
| 2026-03-11 AM | Project genesis — core, mock, workflow, CLI, worker | 48 tests |
| 2026-03-11 PM | Account model, SafetyClass, fleet simulation (100K→1M devices) | 77 tests |
| 2026-03-11 PM | Multi-converter: HA, SmartThings, Google Home, Tuya | 112 tests |
| 2026-03-11 PM | Homey converter — 5th platform | 119 tests |
| 2026-03-11 PM | **Expr type system** — structured AST, cross-platform equivalence | 154 tests |

## Design Philosophy

Inspired by Sussman's [Software Design for Flexibility](https://mitpress.mit.edu/9780262045490/):

> "Organizing systems using combinators to compose mix-and-match parts with standardized interfaces."

- **Additive programming**: New platform = new Parser/Emitter. No existing code changes.
- **Combinators**: `AndExpr(a, b)`, `SeqExpr(cmd, delay, cmd)` — compose freely.
- **Standardized interfaces**: `Parser`/`Emitter`/`SourcePlatform`/`TargetPlatform`.
- **Independent annotation layers**: `SafetyClass`, `Meta`, `SourceMeta`.

## Related Projects

- [Temporal](https://temporal.io) — Durable execution platform
- [Doltgres](https://github.com/dolthub/doltgresql) — Git-like PostgreSQL
- [HomeAgent](https://github.com/junghan0611/homeagent-config) — Open-source Matter smart home platform
- [Open Home Foundation](https://www.openhomefoundation.io/) — Sustainable smart home ecosystem

## License

MIT
