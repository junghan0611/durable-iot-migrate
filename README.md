# durable-iot-migrate

**IoT 플랫폼 마이그레이션 프레임워크 — Clojure semantic layer + Temporal durable execution.**

디바이스와 자동화 레시피를 IoT 플랫폼 간에 안전하게 이전한다.
자동 재시도, Saga 롤백, 의미 보존 검증.

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

## Two Axes of "Durable"

| 축 | 보존 대상 | 메커니즘 |
|----|----------|---------|
| **Durable execution** | 마이그레이션 프로세스가 크래시에서 살아남음 | Temporal workflow + Saga rollback |
| **Durable semantics** | 자동화 규칙의 의미가 변환에서 살아남음 | Clojure Expr IR + equivalence verification |

유연한 것이 durable한 것이다.
구조 있는 유연성 = durable semantics.

## Why Clojure

Sussman 교수의 SDF(Software Design for Flexibility) 철학이 이 프로젝트의 뼈대다.
SDF의 뿌리는 Scheme/Lisp에 있고, Clojure는 그 철학을 JVM 위에 실현한 언어다.

**코드가 곧 데이터(homoiconicity)** — IoT 레시피를 표현하고, 변환하고, 검증하는 데
이보다 자연스러운 도구는 없다:

```clojure
;; 아기방 움직임 감지 → 녹화 + 알림
(recipe
  :id "baby_camera_motion"
  :name "아기방 움직임 감지"
  :trigger   (eq (state-ref "camera.baby" "motion") (lit true))
  :condition (between (time-ref "now") (lit "22:00") (lit "06:00"))
  :actions   (seq-expr
               (cmd "camera.baby" "recording" "start")
               (delay-expr 5)
               (notify "아기방 움직임 감지!")))
```

### Go → Clojure: 동일 기능, 62% 코드 감소

같은 범위(5 플랫폼 파서 + Expr IR + 동치 검증 + 테스트)를 Go와 Clojure로 구현한 결과:

| 언어 | 파일 수 | 코드 라인 | 비고 |
|------|---------|----------|------|
| **Go** | 20 | 3,444 | converters + expr + converter + models |
| **Clojure** | 11 | 1,309 | src + test 전체 |
| **차이** | -9 (45%↓) | -2,135 (**62%↓**) | |

이 차이는 단순한 "짧게 쓰기"가 아니다. 구조적 이유가 있다:

| Go에 필요했던 것 | Clojure에서 불필요한 이유 |
|----------------|------------------------|
| `Expr` struct 정의 | 맵이 곧 타입 |
| `Op` enum 22종 | 키워드가 곧 Op (`:eq`, `:and`, `:ha/choose`) |
| `DeviceRef` struct | 맵 필드 (`:device`, `:attr`) |
| `Validate()` 130줄 | 맵 구조가 자기 기술적(self-describing) |
| `Walk` 재귀 함수 | `walk-expr` 5줄 (clojure.walk 패턴) |
| `fmt.Sprintf("%v", v)` 비교 | `=` 하나 (네이티브 타입 보존) |
| struct 태그 (`yaml:"..."`) | 불필요 (YAML → map 직행) |
| `interface{}` / `any` 캐스팅 | 불필요 (동적 타입) |

그리고 Clojure에서 **Go에 없던 것이 추가됨:**
- `diff` — equiv?가 false일 때 *어디서* 갈라졌는지 구조적 보고
- 열린 확장 — `:ha/choose`, `:tuya/precondition` 등 네임스페이스 키워드로 코드 수정 없이 확장

> *"코드가 줄어든 게 아니라, 불필요했던 의례(ceremony)가 사라진 것이다."*

## Expr — 레시피의 중간 표현(IR)

모든 IoT 플랫폼은 trigger → condition → action 패턴을 공유한다.
표현만 다를 뿐이다. Expr은 이 차이를 흡수하는 보편 문법이다:

```
HA YAML ───→ Parser ──→ Expr 맵 ──→ Emitter ──→ SmartThings JSON
Tuya JSON ──→ Parser ──→ Expr 맵 ──→ Emitter ──→ Google Home YAML
Homey JSON ─→ Parser ──→ Expr 맵 ──→ Emitter ──→ HA YAML
```

### 5 Platforms

| Platform | Format | Parser | Emitter |
|----------|--------|--------|---------|
| **Home Assistant** | YAML | ✅ | 🔄 planned |
| **SmartThings** | JSON (Rules API) | ✅ | 🔄 planned |
| **Google Home** | YAML (Scripted) | ✅ | 🔄 planned |
| **Tuya** | JSON (Scene API) | ✅ | 🔄 planned |
| **Homey** | JSON (Flow API) | ✅ | 🔄 planned |

### Equivalence — 의미 동치 검증

```clojure
;; Tuya: dp_1 == true
;; HA:   state == "on"
;; 구조적으로 같은가?
(structural-equiv? tuya-expr ha-expr)  ; => true

;; 값 매핑 포함 의미적으로 같은가?
(equiv? tuya-expr ha-expr motion-mapper)  ; => true

;; 어디서 갈라졌는가?
(diff expr-a expr-b)  ; => [{:path [1] :type :value-mismatch ...}]
```

### Open Extension

Go에서는 22종 Op enum으로 모든 플랫폼을 커버해야 했다.
Clojure에서는 키워드 하나면 확장 끝:

```clojure
;; HA의 choose (if-else 분기) — 표준 Op에 없어도 그냥 쓴다
{:op :ha/choose
 :children [{:op :eq ... :then (cmd ...)}
            {:op :eq ... :then (cmd ...)}]}

;; walk-expr는 :op을 몰라도 :children만 있으면 순회한다
(walk-expr identity ha-choose)  ; 그냥 작동
```

## Durable Migration (Temporal)

```
Device 1 ✅ → Device 2 ✅ → Device 3 ✅ → Device 4 💥 crash
                                                    │
                                              (server restarts)
                                                    │
                                          Device 4 resumes ✅
```

Temporal Clojure SDK ([manetu/temporal-clojure-sdk](https://github.com/manetu/temporal-clojure-sdk))로
워커와 워크플로우도 Clojure로 구현 예정.

## Project Structure

```
src/iot/semantic/
  expr.clj              ✅ Expr IR — combinators, walk, fold
  equiv.clj             ✅ structural-equiv?, equiv?, diff
  cli.clj               ✅ CLI entrypoint (parse/json/equiv)
  parser/
    homeassistant.clj   ✅ HA YAML → Expr
    smartthings.clj     ✅ ST JSON → Expr
    tuya.clj            ✅ Tuya JSON → Expr
    google.clj          ✅ Google YAML → Expr
    homey.clj           ✅ Homey JSON → Expr
  emitter/              → Expr → platform format (planned)
  fidelity.clj          → round-trip 검증 (planned)

test/iot/semantic/
  expr_test.clj         ✅ 9 tests, 37 assertions
  parser/
    homeassistant_test.clj  ✅ 5 tests, 35 assertions
    multi_platform_test.clj ✅ 5 tests, 37 assertions

archive/go/             Go 참조 구현 (154 tests, 5,221 lines)
build.clj               GraalVM uberjar build
run.sh                  Dev commands (test, native-build, etc.)
deps.edn                Clojure 1.12 + clj-yaml + data.json
flake.nix               NixOS: GraalVM + JVM dev shells
```

## Quick Start

```bash
# Dev environment (GraalVM)
nix develop

# Run tests
./run.sh test                    # 19 tests, 109 assertions

# Parse HA automation
./run.sh run parse ha my-automations.yaml

# GraalVM native binary
./run.sh native-build            # → target/durable-iot-migrate

# Go reference tests
./run.sh test-go
```

## Test Summary

**19 tests, 109 assertions** — Clojure

| Suite | Tests | Assertions | Coverage |
|-------|-------|------------|----------|
| expr (IR + equiv + diff) | 9 | 37 | combinators, cross-platform, walk/fold |
| HA parser | 5 | 35 | 5 automations, legacy format, structural equiv |
| Multi-platform | 5 | 37 | ST/Tuya/Google/Homey + cross-platform equiv |

**154 tests** — Go archive (reference, `./run.sh test-go`)

## History

| Date | Milestone | Stats |
|------|-----------|-------|
| 2026-03-11 | Project genesis — Go core, mock adapter, 5 converters, Expr AST | 154 tests, 5,221 lines |
| 2026-03-12 AM | Clojure semantic layer — Expr IR + equiv + diff | 9 tests |
| 2026-03-12 PM | **Go → Clojure 전환** — 5 parsers ported, Go archived | 19 tests, 1,309 lines |

## Design Philosophy

> "최고의 시스템은 진화할 수 있는 유연성을 갖췄다.
> 기존 코드를 수정하는 대신 새 코드를 추가해 새로운 상황에 적응하는
> 가산적 프로그래밍을 활용한다."
> — Gerald Jay Sussman, SDF 서문

> "코드는 다음 프로젝트의 프롬프트다."
> — 정한

이 프로젝트에서 만드는 코드는 단순한 마이그레이션 도구가 아니다.
다음 에이전트들이 "이 우주에서는 규칙을 이렇게 구조화하고 조립하는 거구나"라고
배우게 될 첫 번째 교과서다.

## Related

- [Temporal](https://temporal.io) — Durable execution platform
- [temporal-clojure-sdk](https://github.com/manetu/temporal-clojure-sdk) — Clojure SDK for Temporal
- [SDF](https://mitpress.mit.edu/9780262045490/) — Software Design for Flexibility (Sussman & Hanson)
- [HomeAgent](https://github.com/junghan0611/homeagent-config) — Open-source Matter smart home
- [Open Home Foundation](https://www.openhomefoundation.io/) — Sustainable smart home ecosystem

## License

MIT
