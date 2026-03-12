(ns iot.semantic.expr
  "IoT 자동화 레시피의 의미를 표현하는 중간 표현(IR).

  핵심 통찰: Clojure에서 Expr은 '타입'이 아니라 '맵'이다.
  Go에서 struct + Op enum + Validate()로 100줄 걸리던 것이
  여기서는 그냥 데이터다.

  {:op :eq
   :children [{:op :state-ref :device \"sensor\" :attr \"motion\"}
              {:op :lit :value true}]}

  이것이 전부다. 괄호가 이미 AST이고, 맵이 이미 타입이다.
  Sussman 교수님이 Scheme으로 가르친 것의 핵심:
  코드와 데이터의 구분이 사라지면, 변환은 자연스러워진다.

  설계 원칙:
  1. 데이터 우선 — defrecord/deftype 대신 순수한 맵
  2. 열린 확장 — :op은 키워드. 누구든 :ha/choose, :tuya/precondition 추가 가능
  3. Spec으로 검증 — 구조 검증은 clojure.spec이 담당
  4. 조합자(combinator) — 함수가 Expr을 만들고, 함수가 Expr을 변환한다")

;; ─── 리프 생성자 ─────────────────────────────────────────────

(defn lit
  "리터럴 값. 숫자, 문자열, 불리언.
  Go에서 Lit(v any)가 타입을 잃어버리는 문제가 여기서는 없다.
  Clojure의 값은 자기 타입을 알고 있다."
  [v]
  {:op :lit :value v})

(defn state-ref
  "디바이스 상태 참조.
  플랫폼마다 ID 체계가 다르지만, 구조는 같다:
  '어떤 장치의 어떤 속성'."
  ([device attr]
   {:op :state-ref :device device :attr attr})
  ([device component attr]
   {:op :state-ref :device device :component component :attr attr}))

(defn time-ref
  "시간 참조. 'now', '22:00', 'sunrise+30m'."
  [v]
  {:op :time-ref :value v})

;; ─── 비교 조합자 ─────────────────────────────────────────────

(defn eq      [a b] {:op :eq      :children [a b]})
(defn ne      [a b] {:op :ne      :children [a b]})
(defn gt      [a b] {:op :gt      :children [a b]})
(defn ge      [a b] {:op :ge      :children [a b]})
(defn lt      [a b] {:op :lt      :children [a b]})
(defn le      [a b] {:op :le      :children [a b]})
(defn between [v lo hi] {:op :between :children [v lo hi]})

(defn in-set
  "멤버십 검사. val ∈ {a, b, c}."
  [val & members]
  {:op :in :children (into [val] members)})

(defn contains-str [a b] {:op :contains :children [a b]})

;; ─── 논리 조합자 ─────────────────────────────────────────────

(defn and-expr
  "모든 조건이 참. Go의 AndExpr(...*Expr)과 같지만,
  가변 인자가 자연스럽다."
  [& children]
  {:op :and :children (vec children)})

(defn or-expr  [& children] {:op :or  :children (vec children)})
(defn not-expr [child]      {:op :not :children [child]})

;; ─── 시퀀스 조합자 ─────────────────────────────────────────────

(defn seq-expr
  "순차 실행. 순서가 의미를 가진다.
  (seq-expr (cmd ...) (delay 5) (cmd ...))"
  [& children]
  {:op :seq :children (vec children)})

(defn parallel [& children] {:op :parallel :children (vec children)})

;; ─── 액션 생성자 ─────────────────────────────────────────────

(defn cmd
  "디바이스 명령."
  [device attr value]
  {:op :command :device device :attr attr :value value})

(defn delay-expr
  "지연(초 단위)."
  [seconds]
  {:op :delay :value seconds})

(defn notify
  "알림 메시지."
  [message]
  {:op :notify :value message})

(defn scene
  "씬 활성화."
  [scene-id]
  {:op :scene :value scene-id})

;; ─── 메타데이터 ─────────────────────────────────────────────

(defn with-meta*
  "플랫폼 특수 메타데이터 부착.
  Clojure의 metadata와 구분하기 위해 :meta 키 사용.
  org-mode의 #+ATTR_LATEX:과 같은 역할."
  [expr k v]
  (assoc-in expr [:meta k] v))

;; ─── 레시피: trigger + condition + action의 묶음 ────────────

(defn recipe
  "하나의 자동화 규칙.
  IoT recipe는 결국 이 세 가지의 조합이다:
  - trigger:   무엇이 일어나면
  - condition: 어떤 조건 아래서
  - actions:   무엇을 한다"
  [& {:keys [id name trigger condition actions]}]
  (cond-> {:id id :name name :actions actions}
    trigger   (assoc :trigger trigger)
    condition (assoc :condition condition)))

;; ─── 트리 순회 유틸 ─────────────────────────────────────────

(defn walk-expr
  "Expr 트리의 모든 노드에 f를 적용한다 (bottom-up).
  Lisp에서는 이것이 내장이다. Go에서는 매번 재귀를 짰다.

  (walk-expr identity expr)           ; 항등 변환
  (walk-expr #(assoc % :visited true) expr)  ; 모든 노드에 표시"
  [f expr]
  (if-let [children (:children expr)]
    (f (assoc expr :children (mapv (partial walk-expr f) children)))
    (f expr)))

(defn fold-expr
  "Expr 트리를 하나의 값으로 접는다 (bottom-up).
  노드 수 세기, 디바이스 참조 수집 등.

  (fold-expr (fn [acc node] (inc acc)) 0 expr)  ; 노드 수"
  [f init expr]
  (let [child-result (reduce (fn [acc child]
                               (fold-expr f acc child))
                             init
                             (:children expr []))]
    (f child-result expr)))

(defn device-refs
  "트리에서 참조된 모든 디바이스 ID를 수집한다."
  [expr]
  (fold-expr
   (fn [acc node]
     (if-let [d (:device node)]
       (conj acc d)
       acc))
   #{}
   expr))

(defn depth
  "트리의 최대 깊이."
  [expr]
  (if-let [children (seq (:children expr))]
    (inc (apply max (map depth children)))
    1))

(defn node-count
  "트리의 총 노드 수."
  [expr]
  (fold-expr (fn [acc _] (inc acc)) 0 expr))
