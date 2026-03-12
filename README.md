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

Go 대비 Clojure의 장점:

| 영역 | Go | Clojure |
|------|-----|---------|
| 타입 표현 | struct + Op enum + Validate() | 맵이 곧 타입 |
| 트리 변환 | 매번 재귀 함수 작성 | `walk-expr` 한 줄 |
| 열린 확장 | `Op string` (관례적 열림) | 네임스페이스 키워드 (`:ha/choose`) |
| 값 비교 | `fmt.Sprintf("%v", v)` (타입 손실) | 네이티브 타입 보존 비교 |
| 직렬화 | JSON 태그 | EDN (타입 보존) + JSON |

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
| **Home Assistant** | YAML | 🔄 porting | — |
| **SmartThings** | JSON (Rules API) | 🔄 porting | — |
| **Google Home** | YAML (Scripted) | 🔄 porting | — |
| **Tuya** | JSON (Scene API) | 🔄 porting | — |
| **Homey** | JSON (Flow API) | 🔄 porting | — |

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

Go에서는 22종 Op으로 모든 플랫폼을 커버해야 했다.
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
  validate.clj          → spec/malli 기반 검증 (planned)
  parser/
    homeassistant.clj   → HA YAML → Expr (porting)
    smartthings.clj     → ST JSON → Expr (porting)
    tuya.clj            → Tuya JSON → Expr (porting)
    google.clj          → Google YAML → Expr (porting)
    homey.clj           → Homey JSON → Expr (porting)
  emitter/              → Expr → platform format (planned)
  fidelity.clj          → round-trip 검증 (planned)

test/iot/semantic/
  expr_test.clj         ✅ 9 tests, 37 assertions

archive/go/             Go 참조 구현 (154 tests, 12 packages)
deps.edn                Clojure dependencies
flake.nix               NixOS dev environment
```

## Quick Start

```bash
# Dev environment
nix develop

# Run tests
clj -M:test

# Go reference (archive)
cd archive/go && go test ./...
```

## History

| Date | Milestone |
|------|-----------|
| 2026-03-11 | Project genesis — Go core, mock adapter, 5 converters, Expr AST (154 tests) |
| 2026-03-12 | **Clojure 전환** — semantic layer in Clojure, Go code archived |

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
