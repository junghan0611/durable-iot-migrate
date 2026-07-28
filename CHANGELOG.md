# CHANGELOG

닫힌 것이 여기로 내려온다. 열려 있는 것은 [`NEXT.md`](NEXT.md)에 있다.

태그는 CalVer 스냅샷(`vYYYY.M.D[-suffix]`)이다. 패키지 릴리즈가 아니라 **책갈피**다.

---

## Unreleased

### 축 전환 — 플랫폼 마이그레이션 → 허브 자동화 코어 (2026-07-28)

이 리포의 목적을 바꿨다. *"플랫폼 A → 플랫폼 B 마이그레이션 프레임워크"* 에서
**"작은 허브에서 실제로 도는 스마트홈 자동화 코어"** 로.

전환의 근거는 셋이다.

1. **허브 계약에 장소가 없다.** capability 기반 device 계약은 기기를 정확히 말하지만
   방도 섹터도 없다. 자동화 층도 통째로 비어 있다.
2. **HA를 포크하면 런타임이 따라온다.** `template:` 한 줄이 Jinja2를, 프론트엔드 포크가
   그쪽 전체를 데려온다. 허브급 보드에서 갚을 수 있는 부채가 아니다.
3. **집합 축이 보편적이다.** 가정의 `방1·방2·거실·화장실`과 공장의 `섹터1·섹터2·라인A`는
   같은 구조다. 이름만 다르다.

**Breaking — 방향**

- 축을 옮겼다: durable **execution**(워크플로 엔진)에서 durable **semantics + bounded 실행**으로.
- **워크플로 엔진(Temporal)을 쓰지 않는다.** 그런 프레임워크는 흔하고, 집 안에 서버를 하나 더
  두는 것은 미니멀 세트가 아니다. `flake.nix`에서 `temporal-cli` 제거.
- **HA는 호환 대상이지 포크 대상이 아니다.** 호환은 파서·에미터 가장자리에만 두고 중심은 작게.
- 5개 파서(HA·SmartThings·Tuya·Google·Homey)를 **호환 표면**으로 승격. 목적지가 남의 플랫폼에서
  우리 허브로 바뀌었을 뿐 하는 일은 같다.

**추가**

- `docs/boundaries.md` — **경계 정본.** 소유/비소유, 자르는 세 기준(순수/컴파일-평가/언어 경계),
  실행 진실 규율 3줄, 런타임 배치 선택지 A·B·C와 권고.
- `NEXT.md` — 핸드오프. 지금 칸(V1~V4)과 그것을 여는 데 필요한 결정(C1~C3).
- `CHANGELOG.md` — 이 파일.
- **허브 실행 가능성 제약**을 문서로 못 박음: 무한 루프·동적 할당·임의 코드 평가·무한 대기·
  런타임 집합 질의 금지. 컴파일러가 거부한다.
- **호환 규율**: 중심이 표현할 수 없는 것은 근사하지 않고 거부 + 카운트.
- **검증 등급 G0~G4** 도입 — 파싱 / 표현 / **왕복** / 문법 유효 / 행동 동치.
  등급을 섞어 말하지 않는다. 실기 없이 G4를 주장하지 않는다.
- **발명 금지 규율** — 코퍼스는 남의 것, op은 빈도가 연다, 못 담은 것은 센다,
  낮은 점수는 결과이지 실패가 아니다. *"아이디어 내서 만들면 검증할 수 없다."*

**첫 칸 확정**

- **중간 폼의 성립 증명**이 첫 칸이다. place·런타임 배치·bounded 컴파일은 그 뒤로 미뤘다 —
  중간 폼이 실물을 담는다는 증거 없이 위층을 쌓으면 전부 모래 위다.

**제거**

- **beads(`br`) 이슈 트래커 전부.** 추적 파일 삭제, `.gitignore`·`AGENTS.md`에서 제거.
  열린 것은 `NEXT.md`, 닫힌 것은 `CHANGELOG.md` 둘뿐이다.
- `flake.nix`의 Temporal 도구와 셸 힌트.

**변경 없음**

- Clojure 코드 한 줄도 건드리지 않았다. **19 tests / 109 assertions** 그대로.
- `archive/go/` — Go 참조 구현(154 tests) 그대로.

---

## 태그 이전 (2026-03)

이 리포는 2026-07-28까지 태그 없이 진행됐다. 아래는 그 이전에 닫힌 작업이다.

### 2026-03-12 — Go → Clojure 전환

- **Clojure semantic layer**: Expr IR(조합자·`walk-expr`·`fold-expr`) + `structural-equiv?` ·
  `equiv?` · `diff`.
- **5 플랫폼 파서 이식**: Home Assistant(YAML) · SmartThings(Rules JSON) · Tuya(Scene JSON) ·
  Google Home(YAML) · Homey(Flow JSON) → Expr.
- CLI(`parse`/`json`/`equiv`) + GraalVM uberjar 빌드 + Clojure용 `flake.nix`.
- Go 구현을 `archive/go/`로 아카이브(154 tests, 참조 보존).
- **동일 범위에서 62% 코드 감소** — Go 20파일 3,444줄 → Clojure 11파일 1,309줄.
  맵이 곧 타입, 키워드가 곧 Op, `Validate()` 130줄이 사라짐.
- Clojure에서 새로 생긴 것: `diff`(어디서 갈라졌는지 구조적 보고), 네임스페이스 키워드
  기반 열린 확장(`:ha/choose` 등 — 코드 수정 없이 확장).

### 2026-03-11 — Go core genesis

- 코어 프레임워크: 모델·인터페이스·워크플로·mock 어댑터.
- 5 플랫폼 컨버터(HA · SmartThings · Google Home · Tuya · Homey).
- Expr 타입 시스템(struct + 22종 Op enum).
- Account 모델, SafetyClass, fleet 시뮬레이션(1M devices / 1.5s).
- **154 tests, 98.6% 라이브러리 커버리지**, 5,221 lines.
