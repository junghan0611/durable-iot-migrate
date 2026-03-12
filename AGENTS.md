# AGENTS.md — durable-iot-migrate

## Project

Durable IoT platform migration framework.
Clojure semantic layer (Expr IR, equivalence, converters) + Temporal durable execution.
SDF (Software Design for Flexibility) philosophy: code is data, combinators compose freely.

## Language

- **Primary**: Clojure (semantic layer, converters, CLI, tests)
- **Archive**: Go reference implementation in `archive/go/` (154 tests, 12 packages)
- Docs: English code, Korean docs

## Architecture

```
src/iot/semantic/
  expr.clj           → Expr IR: combinators, walk-expr, fold-expr
  equiv.clj          → structural-equiv?, equiv?, diff
  validate.clj       → spec/malli validation (planned)
  parser/            → Platform-specific parsers → Expr maps
  emitter/           → Expr maps → platform-specific format
  fidelity.clj       → Round-trip verification (planned)

test/iot/semantic/   → Tests (cognitect test-runner)
archive/go/          → Go reference (preserved until Clojure reaches parity)
tmp/                 → temporal-clojure-sdk reference
```

## Expr IR — The Core Data Model

Expr is a plain Clojure map. No defrecord, no deftype:

```clojure
{:op :eq
 :children [{:op :state-ref :device "sensor" :attr "motion"}
            {:op :lit :value true}]}
```

### Standard Ops

| Category | Ops |
|----------|-----|
| Comparison | `:eq` `:ne` `:gt` `:ge` `:lt` `:le` `:between` `:in` `:contains` |
| Combinators | `:and` `:or` `:not` `:seq` `:parallel` |
| Actions | `:command` `:delay` `:notify` `:scene` |
| Leaf | `:lit` `:state-ref` `:time-ref` |

### Open Extension

Platform-specific ops use namespaced keywords:
`:ha/choose`, `:ha/repeat`, `:tuya/precondition`, `:homey/card`

`walk-expr` traverses any `:op` — it only needs `:children`.

## Key Interfaces (Protocols, planned)

```clojure
;; Converter layer
(defprotocol Parser
  (platform [this])
  (parse-bytes [this data]))

(defprotocol Emitter
  (emit-bytes [this recipes]))

;; Migration layer (via Temporal activities)
(defprotocol SourcePlatform
  (list-devices [this])
  (list-automations [this])
  (unbind-device [this device-id])
  (rebind-device [this device-id]))

(defprotocol TargetPlatform
  (bind-device [this device])
  (create-automation [this auto])
  (verify-device [this device-id])
  (target-unbind-device [this device-id])
  (delete-automation [this auto-id]))
```

## Development

```bash
nix develop                       # Clojure 1.12 + JDK 17 + temporal-cli
clj -M:test                       # Run tests
cd archive/go && go test ./...    # Go reference tests
```

## Testing Strategy

- Pure data: Expr maps are immutable, testable without mocks
- Cross-platform: 5 platforms × equiv? verification
- Value mapping: motion mapper (true ↔ "on" ↔ "active")
- Diff reporting: structural diff when equiv? fails
- Property-based: test.check for random Expr generation (planned)
- **Goal**: Clojure tests must reach Go's 154-test coverage before archive removal

## Conventions

- Pure Clojure maps over defrecord/deftype
- Namespaced keywords for extension (`:ha/choose`, not `"ha:choose"`)
- cognitect test-runner for tests
- `clj -M:test` must pass before commit
- Commit messages: `type: description` (feat/test/fix/docs/refactor)

## Issue Tracking (beads_rust)

```bash
br list                          # List issues
br create "title" -p p1 -l "clojure,tag"
br update <id> -s in_progress
br close <id>                    # After setting design/acceptance-criteria/notes
br sync --flush-only && git add .beads/ && git commit
```

## Migration Plan: Go → Clojure

| Phase | Task | Status |
|-------|------|--------|
| 1 | Archive Go to `archive/go/` | ✅ |
| 2 | Clojure Expr IR + equiv | ✅ |
| 3 | flake.nix for Clojure | ✅ |
| 4 | README + AGENTS.md rewrite | ✅ |
| 5 | Port 5 platform parsers | 🔄 |
| 6 | Clojure CLI | 🔄 |
| 7 | Temporal Clojure workers | 🔄 |
| 8 | Coverage parity → remove `archive/go/` | ⬜ |
