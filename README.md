# durable-iot-migrate

**작은 허브에서 실제로 도는 스마트홈 자동화 코어.**
장소(place)와 레시피(recipe)를 표현하고, 컴파일하고, 의미가 보존됐는지 검증한다.

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

> **방향 전환 중 (2026-07-28).** 이 리포는 원래 *"플랫폼 A → 플랫폼 B 마이그레이션 프레임워크"*
> 였다. 지금은 축을 옮긴다 — **capability 기반 허브 위에 얹히는 최소 자동화 층**.
> 옛 축은 [히스토리](#히스토리)에 남기고, Expr IR과 파서는 그대로 승계한다.
> 리포 이름은 아직 옛 축을 가리킨다 — [`NEXT.md`](NEXT.md) 참조.

---

## 한 줄 원칙

> **허브에서 돌 수 있는 것만 레시피다.**

표현력이 먼저가 아니다. **실행 가능성이 먼저**고, 표현력은 그 제약이 허락하는 만큼만 연다.

---

## 왜 이 층이 비어 있는가

스마트홈 자동화에는 사실상 두 극단만 있다.

| | 표현력 | 런타임 | 오프라인 | 의미 검증 |
|---|---|---|---|---|
| **Home Assistant** 류 | 무한 (템플릿·스크립트) | Python 전체 + 수백 MB | 가능하지만 무겁다 | 없음 |
| **벤더 클라우드 씬** | 폐쇄·고정 | 남의 서버 | 불가 | 불가 |

그 사이가 비어 있다: **작은 허브에서 오프라인으로 돌면서, 의미가 검증 가능한 자동화 코어.**

### HA와 호환한다. HA가 되지는 않는다.

HA는 이 세계의 사실상 표준이고, 거기 쌓인 automation·블루프린트·커뮤니티 레시피는 자산이다.
**그러니 읽는다.** 그리고 다시 내보낸다 — 우리 허브를 쓰다 떠나는 사람이 잠기지 않도록.

하지만 HA를 **이식하지는 않는다.** automation의 표현력만 떼어올 수는 없기 때문이다.
`template:` 한 줄이 Jinja2를 데려오고, `condition: template` 한 줄이 임의 Python 평가를
데려온다. 프론트엔드를 포크하면 그쪽 전체가 또 따라온다. 그러면:

1. 자동화는 **서버에서만** 돈다.
2. 인터넷이 끊긴 집은 스위치도 못 켠다.
3. 레시피가 무엇을 할지 **정적으로 알 수 없다** — 검증도 시뮬레이션도 불가능해진다.

허브급 보드(ARM64 hub-class, 가용 RAM 300MB대, 대부분을 Zigbee 스택이 쓴다)에서
이 부채는 갚을 수 있는 종류가 아니다.

그래서 **호환은 가장자리에 두고, 중심은 작게 유지한다.**

```
HA YAML ──► [파서] ──► Expr IR ──► [컴파일] ──► 허브에서 도는 레시피
                          │
                          └────► [에미터] ──► HA YAML   (되돌아갈 문)
             ▲                                    ▲
        가장자리                              가장자리
                    ↑ 중심은 이 안에만 있다 ↑
```

읽다가 중심이 표현할 수 없는 것을 만나면 **조용히 근사하지 않는다.**
명시적으로 거부하고, **몇 개를 못 읽었는지 센다.** 추측된 자동화는 밤에 잘못 켜진다.

### 그리고 device 계약에는 **방이 없다**

capability 기반 device 계약은 기기를 정확히 말한다 — identity, capability instance,
inventory / state / action / result. 앱은 `kind + access`만으로 컨트롤을 그릴 수 있다.
DeviceType 분기가 없다는 뜻이고, 그건 좋은 설계다.

그런데 그 계약 어디에도 **장소**가 없다. 기기 목록이 평평하게 있을 뿐이다.
사람이 공간을 다루는 방식은 그 축 위에 선다:

```
가정:  방1 · 방2 · 거실 · 화장실
공장:  섹터1 · 섹터2 · 라인A
```

**같은 구조다. 이름만 다르다.** place는 이 리포가 여는 축이다.

---

## durable의 두 축

| 축 | 보존 대상 | 메커니즘 |
|---|---|---|
| **durable semantics** | 레시피의 **의미**가 표현·변환·이식에서 살아남는다 | Expr IR + `equiv?` + `diff` |
| **durable execution** | 레시피의 **실행**이 재부팅·단절에서 살아남는다 | bounded 컴파일 + 관측 기반 완료 판정 |

두 번째 축을 워크플로 엔진(Temporal)으로 풀던 것이 옛 설계다. **쓰지 않는다** —
그런 프레임워크는 흔하고, 집 안에 서버를 하나 더 두는 것은 미니멀 세트가 아니다.
durable execution은 다르게 산다:

- 실행 상태가 **고정 크기로 직렬화**된다 → 전원이 끊겨도 다음 부팅에서 이어진다.
- **dispatch는 완료가 아니다.** action을 보냈다는 사실은 상태가 아니다.
  "레시피가 실행됐다"의 진실은 **관측된 state**다.

---

## 세 층

```
  ┌ 가장자리 ┐   ┌──────────────────────────────────────┐   ┌ 가장자리 ┐
  │  파서    │──►│  3. Verify  equiv? · diff · simulate │──►│  에미터  │
  │          │   ├──────────────────────────────────────┤   │          │
  │ HA · ST  │   │  2. Recipe  Expr IR · bounded 컴파일 │   │ HA · ST  │
  │ Tuya     │   ├──────────────────────────────────────┤   │ Tuya     │
  │ Google   │   │  1. Place   area · 집합 참조         │   │ Google   │
  │ Homey    │   ╞══════════════════════════════════════╡   │ Homey    │
  └──────────┘   │  capability device 계약 (이 리포 밖) │   └──────────┘
                 └──────────────────────────────────────┘
                        ↑ 중심. 작게 유지한다 ↑
```

이 리포가 소유하는 것과 소유하지 않는 것은 [`docs/boundaries.md`](docs/boundaries.md)가 정본이다.
한 줄 요약: **IR·컴파일·검증은 여기, 런타임과 transport는 허브·서버 몫.**

### 1. Place — 장소

`area`는 기기의 속성이 아니라 **기기를 담는 집합**이다. 레시피는 개별 기기가 아니라
집합을 겨눌 수 있다:

```clojure
;; "거실의 모든 조명"  — 기기가 늘어도 레시피는 그대로다
(cap-set :area "거실" :cap "switch")
```

이 참조가 컴파일 시점에 **구체 기기 목록으로 접힌다.** 허브는 집합을 해석하지 않는다 —
접힌 목록만 받는다. 그래야 허브 쪽에 동적 질의가 안 생긴다.

### 2. Recipe — 레시피

모든 플랫폼이 공유하는 구조: **trigger → condition → actions**.

```clojure
(recipe
  :id        "night_bathroom_light"
  :name      "야간 화장실 조명"
  :trigger   (eq (state-ref "sensor.bath.motion" "motion") (lit true))
  :condition (between (time-ref "now") (lit "23:00") (lit "06:00"))
  :actions   (seq-expr
               (cmd "light.bath" "switch" true)
               (delay-expr 180)
               (cmd "light.bath" "switch" false)))
```

Expr은 타입이 아니라 **맵**이다. 괄호가 이미 AST이고 맵이 이미 타입이다.

### 3. Verify — 검증

```clojure
(structural-equiv? a b)      ; 구조가 같은가
(equiv? a b value-mapper)    ; 값 매핑까지 포함해 의미가 같은가
(diff a b)                   ; 다르다면 어디서 갈라졌는가
```

---

## 허브 실행 가능성 — IR에 박는 제약

"미니멀 durable set"의 실제 정의는 이 목록이다. 컴파일러가 **거부**한다:

| 제약 | 이유 |
|---|---|
| 무한 루프 없음 — 반복은 컴파일 시 상한이 정해진다 | 100ms 루프를 굶기지 않는다 |
| 동적 할당 없음 — 레시피당 상태 슬롯이 고정 | 메모리 상한이 정적으로 계산된다 |
| 임의 코드 평가 없음 — 템플릿·스크립트·eval 금지 | 정적으로 무엇을 할지 알 수 있어야 검증·시뮬레이션이 산다 |
| 모든 대기에 상한 — `delay`는 bounded | 영원히 매달린 레시피가 없다 |
| 집합 참조는 컴파일 시 접힌다 | 허브에 질의 엔진이 안 생긴다 |

> 이 제약은 첫 런타임이 서버든 허브든 **지금부터** 강제한다.
> 나중에 허브로 옮길 때 IR을 갈아엎지 않기 위해서다.

---

## 왜 Clojure인가

Sussman의 SDF(Software Design for Flexibility) 철학이 뼈대다. 코드가 곧 데이터이므로
레시피를 **표현하고, 변환하고, 검증하는** 데 이만한 도구가 없다.

같은 범위(5 플랫폼 파서 + Expr IR + 동치 검증 + 테스트)를 Go와 Clojure로 구현한 결과:

| 언어 | 파일 | 코드 라인 |
|---|---|---|
| Go | 20 | 3,444 |
| **Clojure** | **11** | **1,309** (62%↓) |

| Go에 필요했던 것 | Clojure에서 불필요한 이유 |
|---|---|
| `Expr` struct + `Op` enum 22종 | 맵이 곧 타입, 키워드가 곧 Op |
| `Validate()` 130줄 | 맵 구조가 자기 기술적 |
| `Walk` 재귀 함수 | `walk-expr` 5줄 |
| struct 태그 / `interface{}` 캐스팅 | 불필요 |

> *"코드가 줄어든 게 아니라, 불필요했던 의례(ceremony)가 사라진 것이다."*

산출물은 **라이브러리 + GraalVM 네이티브 CLI**다. 허브(Zig)와 서버(Go)가 Clojure를
링크하는 것이 아니라, 이 리포가 낸 **컴파일 결과와 스펙**을 각자 읽는다.

---

## 지금 있는 것

```
src/iot/semantic/
  expr.clj              ✅ Expr IR — 조합자, walk, fold
  equiv.clj             ✅ structural-equiv?, equiv?, diff
  cli.clj               ✅ CLI (parse/json/equiv)
  parser/
    homeassistant.clj   ✅ HA YAML → Expr
    smartthings.clj     ✅ ST JSON → Expr
    tuya.clj            ✅ Tuya JSON → Expr
    google.clj          ✅ Google YAML → Expr
    homey.clj           ✅ Homey JSON → Expr

  emitter/ha.clj        🔄 Expr → HA YAML          ← 지금 칸
  fidelity.clj          🔄 왕복 측정 + 등급 채점    ← 지금 칸

  place.clj             ⬜ area 모델                (중간 폼이 증명된 뒤)
  compile.clj           ⬜ bounded 컴파일           (중간 폼이 증명된 뒤)
  simulate.clj          ⬜ 결정론적 시뮬레이션      (중간 폼이 증명된 뒤)

archive/go/             Go 참조 구현 (154 tests)
```

**19 tests, 109 assertions** — `./run.sh test`

5개 파서는 옛 축에서 왔지만 이제 **호환 표면**으로 승격한다. 목적지가 남의 플랫폼에서
우리 허브로 바뀌었을 뿐, 하는 일은 같다 — 남의 어휘를 중심의 어휘로 옮긴다.
HA 파서가 그중 첫 번째 시민이다.

### 지금 칸 — 중간 폼이 성립하는가

place도 실행도 그 위층이다. 먼저 **중간 폼이 남의 실물 레시피를 담고 되돌리는지**를 잰다.
우리가 쓴 예제로 우리 IR을 통과시키는 것은 증명이 아니므로, 코퍼스는 **남이 쓴 automation**이고
점수는 **왕복**으로 낸다.

| 등급 | 주장 | 재는 법 |
|---|---|---|
| G0 파싱 | 읽힌다 | 예외 없이 통과 |
| G1 표현 | 손실 없이 담긴다 | `unsupported` 0건 |
| **G2 왕복** | 되돌려도 같다 | `parse → emit → parse` 가 `structural-equiv?` |
| G3 문법 유효 | 대상 플랫폼이 받아준다 | emit 결과가 스키마 통과 |
| G4 행동 동치 | 실제로 같게 동작한다 | **범위 밖. 주장하지 않는다** |

**이번 목표는 G2.** IR을 늘릴지도 취향이 아니라 **코퍼스의 미지원 구문 빈도**가 정한다.
자세한 것은 [`NEXT.md`](NEXT.md).

---

## Quick Start

```bash
nix develop                      # GraalVM + Clojure 1.12
./run.sh test                    # 19 tests, 109 assertions
./run.sh run parse ha my-automations.yaml
./run.sh native-build            # → target/durable-iot-migrate
```

---

## 히스토리

| 날짜 | 마일스톤 |
|---|---|
| 2026-03-11 | Go core — mock adapter, 5 converters, Expr AST (154 tests, 5,221 lines) |
| 2026-03-12 | **Go → Clojure 전환** — 5 parsers 이식, Go 아카이브 (19 tests, 1,309 lines) |
| 2026-07-28 | **축 전환** — 플랫폼 마이그레이션 → 허브에 붙는 최소 자동화 층 |

---

## 설계 철학

> "최고의 시스템은 진화할 수 있는 유연성을 갖췄다. 기존 코드를 수정하는 대신
> 새 코드를 추가해 새로운 상황에 적응하는 가산적 프로그래밍을 활용한다."
> — Gerald Jay Sussman, *SDF* 서문

> "코드는 다음 프로젝트의 프롬프트다." — 정한

---

## Related

- [HomeAgent](https://github.com/junghan0611/homeagent-config) — minimal hub BSP (보드·이미지 레인)
- [Zigbee2MQTT](https://www.zigbee2mqtt.io/) — Zigbee 위임 대상
- [Matter](https://csa-iot.org/all-solutions/matter/) — 예약된 두 번째 backend
- [SDF](https://mitpress.mit.edu/9780262045490/) — Software Design for Flexibility

## License

MIT
