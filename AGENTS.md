# AGENTS.md — durable-iot-migrate

## Project

**작은 허브에서 실제로 도는 스마트홈 자동화 코어.**
장소(place)와 레시피(recipe)를 표현하고, 컴파일하고, 의미가 보존됐는지 검증한다.

> **축 전환 (2026-07-28).** 옛 축은 *"플랫폼 A → 플랫폼 B 마이그레이션 프레임워크 (Temporal)"*
> 였다. 지금은 **capability 기반 허브 위에 얹히는 최소 자동화 층**이다.
> 워크플로 엔진(Temporal)은 **쓰지 않는다.** Expr IR과 5개 파서는 승계한다.
> 리포 이름은 아직 옛 축을 가리킨다 — `NEXT.md`의 "뒤로 미룬 것".

한 줄 원칙: **허브에서 돌 수 있는 것만 레시피다.**

## 이 리포에서 일하기 전에 읽을 것

| 순서 | 문서 | 왜 |
|---|---|---|
| 1 | `docs/boundaries.md` | **경계 정본.** 소유/비소유가 어긋나면 이 문서가 이긴다 |
| 2 | `NEXT.md` | 지금 칸(V1~V4)과 그것을 여는 결정(C1~C3) |
| 3 | `README.md` | 축과 논지 |
| 4 | `src/iot/semantic/expr.clj` | IR 실물 |

**지금 칸은 "중간 폼이 성립하는가" 하나다.** 코퍼스 → 에미터 → 왕복 측정(V1~V4).
`place.clj`/`compile.clj`/`simulate.clj`는 **그 뒤에** 연다 — 중간 폼이 실물을 담는다는 증거
없이 그 위층을 쌓으면 전부 모래 위다.

## Language

- **Primary**: Clojure (IR, 컴파일, 검증, 파서/에미터, CLI)
- **Archive**: `archive/go/` — Go 참조 구현 (154 tests). 참조용, 수정하지 않는다
- 코드는 영어, 문서는 한국어

## Architecture

```
가장자리 ─► 중심 ─► 가장자리
파서        IR·컴파일·검증        에미터

src/iot/semantic/
  expr.clj              ✅ Expr IR — 조합자, walk-expr, fold-expr
  equiv.clj             ✅ structural-equiv?, equiv?, diff
  cli.clj               ✅ CLI
  parser/               ✅ HA · SmartThings · Tuya · Google · Homey → Expr
  emitter/ha.clj        🔄 Expr → HA YAML                    ← 지금 칸 (V2)
  fidelity.clj          🔄 왕복 측정 + 등급 채점              ← 지금 칸 (V3)
  place.clj             ⬜ area 위계 + 집합 참조 접기         (중간 폼 증명 뒤)
  compile.clj           ⬜ bounded 컴파일 + 제약 거부          (중간 폼 증명 뒤)
  simulate.clj          ⬜ 결정론적 시뮬레이션 (시간 주입)      (중간 폼 증명 뒤)

test/iot/semantic/      cognitect test-runner
corpus/                 남이 쓴 실물 레시피 (재배포 가능한 것만)
```

## 검증 등급 — 섞어 말하지 않는다

| 등급 | 주장 | 재는 법 |
|---|---|---|
| G0 | 읽힌다 | 예외 없이 파서 통과 |
| G1 | 손실 없이 담긴다 | `unsupported` 0건 |
| **G2** | 되돌려도 같다 | `parse → emit → parse` 가 `structural-equiv?` |
| G3 | 대상 플랫폼이 받아준다 | emit 결과가 스키마 통과 |
| G4 | 실제로 같게 동작한다 | **범위 밖. 주장하지 않는다** |

G2를 재고 G3를 말하지 않는다. 실기 없이 G4를 말하지 않는다.

## Expr IR — 코어 데이터 모델

Expr은 순수한 Clojure 맵이다. defrecord도 deftype도 없다:

```clojure
{:op :eq
 :children [{:op :state-ref :device "sensor" :attr "motion"}
            {:op :lit :value true}]}
```

| 분류 | Ops |
|---|---|
| 비교 | `:eq` `:ne` `:gt` `:ge` `:lt` `:le` `:between` `:in` `:contains` |
| 조합 | `:and` `:or` `:not` `:seq` `:parallel` |
| 액션 | `:command` `:delay` `:notify` `:scene` |
| 리프 | `:lit` `:state-ref` `:time-ref` |

플랫폼 고유 op은 네임스페이스 키워드로: `:ha/choose` `:tuya/precondition` `:homey/card`.
`walk-expr`는 `:op`을 몰라도 `:children`만 있으면 순회한다.

## 절대 규칙 (경계에서 나온다)

이 리포에 **들어오면 안 되는 것**:

- **임의 코드 평가** — 템플릿 엔진, 스크립트 런타임, `eval`. HA의 `template:`을 흉내내지 않는다
- **네트워크 I/O** — 어떤 플랫폼 API도 여기서 호출하지 않는다
- **실시간 시계** — `(System/currentTimeMillis)` 금지. 시간은 항상 주입된다
- **워크플로 서버 의존** — Temporal 포함. 집 안에 브로커를 하나 더 두지 않는다
- **device capability의 재정의** — 남의 taxonomy를 여기서 고치면 진실이 둘이 된다
- **defrecord / deftype** — 맵이 곧 타입이다

컴파일러가 **거부해야 하는 것** (`compile.clj`가 생기면):

- 무한 루프 — 반복은 컴파일 시 상한이 정해진다
- 동적 할당 — 레시피당 상태 슬롯이 고정
- 무한 대기 — 모든 `delay`에 상한
- 런타임 집합 질의 — 집합 참조는 컴파일 시 구체 기기 목록으로 접힌다

## 호환 규율

외부 플랫폼을 읽을 때 **중심이 표현할 수 없는 것은 근사하지 않는다.**
명시적으로 거부하고 **몇 개를 못 읽었는지 센다.** 추측된 자동화는 밤에 잘못 켜진다.

```clojure
{:recipes [...] :unsupported-count 3 :unsupported ["template condition" ...]}
```

## 발명 금지 — 이 리포에서 가장 어기기 쉬운 규칙

*"아이디어 내서 만들면 검증할 수 없다."* (GLG, 2026-07-28)

1. **코퍼스는 남의 것이다.** 우리가 쓴 예제로 커버리지를 올리지 않는다.
   기존 테스트의 예제는 회귀 테스트로 남고 **점수에는 안 들어간다.**
2. **op은 빈도가 연다.** *"이 op이 있으면 좋겠다"* 로 IR을 늘리지 않는다.
   코퍼스에서 N건 이상 나온 구문만 후보가 된다.
3. **못 담은 것은 센다.** 목록이 다음에 무엇을 열지 지목한다.
4. **의도적으로 안 여는 것은 그렇게 적는다.** `template` 처럼 열면 중심이 무너지는 것은
   빈도 1등이어도 안 연다. 실패가 아니라 경계다.
5. **낮은 점수는 결과이지 실패가 아니다.** *"중간 폼이 실물의 66%를 담는다"* 는
   검증된 사실이고, 그것이 측정이 원한 것이다. 숫자를 올리려고 규칙 1~2를 어기지 않는다.

## Development

```bash
nix develop                       # GraalVM + Clojure 1.12 + JDK
./run.sh test                     # 19 tests, 109 assertions
./run.sh run parse ha file.yaml
./run.sh native-build             # → target/durable-iot-migrate
./run.sh test-go                  # archive 참조 테스트 (154 tests)
```

## Testing Strategy

- **순수 데이터** — Expr 맵은 불변이고 mock 없이 테스트된다
- **결정론** — 시간이 주입되므로 시뮬레이션 결과가 재현된다
- **교차 플랫폼** — 5 플랫폼 × `equiv?` 검증
- **diff 보고** — `equiv?`가 false일 때 어디서 갈라졌는지 구조적으로 보고
- `clj -M:test`가 통과해야 커밋

## Conventions

- 순수 맵 > defrecord/deftype
- 확장은 네임스페이스 키워드 (`:ha/choose`, `"ha:choose"` 아님)
- 커밋 메시지: `type: description` (feat/test/fix/docs/refactor)

## 작업 흐름

| 파일 | 역할 |
|---|---|
| `NEXT.md` | **휘발성.** 지금 열린 결정과 다음 한 걸음. 닫히면 지운다 |
| `CHANGELOG.md` | **지속성.** 닫힌 것. NEXT에서 승격되어 내려온다 |
| `docs/boundaries.md` | 경계 정본 |

이슈 트래커는 쓰지 않는다. 열린 것은 `NEXT.md`, 닫힌 것은 `CHANGELOG.md` 둘뿐이다.
태그는 CalVer 스냅샷(`vYYYY.M.D`)이며 `tag-release` 스킬을 따른다.
