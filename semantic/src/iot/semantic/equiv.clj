(ns iot.semantic.equiv
  "Expr 트리의 의미 동치성(semantic equivalence) 검증.

  이것이 'durable semantics'의 핵심이다.
  변환 전후의 Expr이 '같은 의미'인지를 판정한다.

  세 가지 수준:
  1. structural-equiv? — 같은 모양인가 (Op + 자식 수)
  2. equiv?            — 같은 의미인가 (값 + 참조 포함)
  3. equiv-with-mapper — 값 매핑 포함 동치 (true ↔ \"on\" ↔ \"active\")

  Go에서 Equiv()가 100줄이었다.
  Clojure에서는 데이터 비교가 언어에 내장되어 있어서
  '어디가 같고 어디가 다른지'를 자연스럽게 표현할 수 있다.

  가장 중요한 통찰:
  And/Or은 교환법칙이 성립한다 — (and A B) ≡ (and B A).
  Seq은 아니다 — (seq 켜기 끄기) ≠ (seq 끄기 켜기).
  이것을 '알고 있는' 비교가 semantic equivalence다.")

;; ─── 구조적 동치 ─────────────────────────────────────────────

(defn structural-equiv?
  "트리의 형태(Op + arity)만 비교. 값과 참조는 무시.
  '같은 패턴, 다른 디바이스'를 검출한다."
  [a b]
  (cond
    (and (nil? a) (nil? b)) true
    (or (nil? a) (nil? b))  false
    (not= (:op a) (:op b))  false
    :else
    (let [ac (:children a [])
          bc (:children b [])]
      (and (= (count ac) (count bc))
           (every? true? (map structural-equiv? ac bc))))))

;; ─── 의미 동치: 핵심 ────────────────────────────────────────

(defn- commutative-op?
  "교환법칙이 성립하는 Op인가?
  And, Or, Parallel은 자식 순서가 무관하다."
  [op]
  (#{:and :or :parallel} op))

(defn- match-all?
  "as의 모든 원소가 bs에 대응되는가? (순서 무관, 1:1 매칭)
  가장 순수한 집합 동치 — Go에서 used[] 배열로 했던 것을
  Clojure에서는 재귀 소거로 표현한다."
  [equiv-fn as bs]
  (if (empty? as)
    (empty? bs)
    (let [a (first as)]
      (some (fn [i]
              (when (equiv-fn a (nth bs i))
                (match-all? equiv-fn
                            (rest as)
                            (into (subvec (vec bs) 0 i)
                                  (subvec (vec bs) (inc i))))))
            (range (count bs))))))

(defn- ref-equiv?
  "디바이스 참조 동치. attribute가 같으면 동치.
  device ID는 플랫폼마다 다르므로 무시."
  [a b]
  (= (:attr a) (:attr b)))

(defn- value-equiv?
  "값 동치. Go에서 fmt.Sprintf로 비교했던 그 부분.
  Clojure에서는 타입 인식 비교가 가능하다."
  [va vb mapper]
  (let [va' (if mapper (mapper nil va) va)
        vb' (if mapper (mapper nil vb) vb)]
    (= va' vb')))

(defn equiv?
  "의미 동치 판정.

  두 Expr이 '같은 자동화 규칙을 표현하는가'를 검증한다.
  And/Or은 순서 무관, Seq은 순서 유관."
  ([a b]
   (equiv? a b nil))
  ([a b mapper]
   (cond
     ;; nil 처리
     (and (nil? a) (nil? b)) true
     (or  (nil? a) (nil? b)) false

     ;; Op이 다르면 다르다
     (not= (:op a) (:op b)) false

     :else
     (case (:op a)
       ;; 리프 노드: 값 비교
       :lit       (value-equiv? (:value a) (:value b) mapper)
       :state-ref (ref-equiv? a b)
       :time-ref  (= (:value a) (:value b))

       ;; 교환 가능한 조합자: 순서 무관 매칭
       (:and :or :parallel)
       (and (= (count (:children a)) (count (:children b)))
            (match-all? #(equiv? %1 %2 mapper)
                        (:children a)
                        (:children b)))

       ;; Not: 정확히 1개 자식
       :not
       (and (= 1 (count (:children a)) (count (:children b)))
            (equiv? (first (:children a)) (first (:children b)) mapper))

       ;; Seq: 순서 유관 (켜기→끄기 ≠ 끄기→켜기)
       :seq
       (and (= (count (:children a)) (count (:children b)))
            (every? true?
                    (map #(equiv? %1 %2 mapper) (:children a) (:children b))))

       ;; 나머지: 자식 순서대로 + ref + value
       (and (= (count (:children a [])) (count (:children b [])))
            (every? true?
                    (map #(equiv? %1 %2 mapper)
                         (:children a []) (:children b [])))
            (ref-equiv? a b)
            (if (or (:value a) (:value b))
              (value-equiv? (:value a) (:value b) mapper)
              true))))))

;; ─── 차이 보고 ──────────────────────────────────────────────

(defn diff
  "두 Expr의 차이를 구조적으로 보고한다.
  Go에는 없던 기능 — equiv?가 false일 때 '어디서 갈라졌는지'를 알려준다.
  이것이 org-mode diff와 같은 역할이다."
  ([a b] (diff a b []))
  ([a b path]
   (cond
     (and (nil? a) (nil? b)) nil
     (nil? a) [{:path path :type :missing-left  :right b}]
     (nil? b) [{:path path :type :missing-right :left a}]

     (not= (:op a) (:op b))
     [{:path path :type :op-mismatch :left (:op a) :right (:op b)}]

     ;; 리프 노드: 값 차이
     (#{:lit :time-ref} (:op a))
     (when (not= (:value a) (:value b))
       [{:path path :type :value-mismatch
         :left (:value a) :right (:value b)}])

     (= :state-ref (:op a))
     (when-not (ref-equiv? a b)
       [{:path path :type :ref-mismatch
         :left {:device (:device a) :attr (:attr a)}
         :right {:device (:device b) :attr (:attr b)}}])

     ;; 내부 노드: 자식 재귀
     :else
     (let [ac (:children a [])
           bc (:children b [])
           len-diff (when (not= (count ac) (count bc))
                      [{:path path :type :arity-mismatch
                        :left (count ac) :right (count bc)}])
           child-diffs (mapcat (fn [i]
                                 (when (and (< i (count ac))
                                            (< i (count bc)))
                                   (diff (nth ac i) (nth bc i)
                                         (conj path i))))
                               (range (max (count ac) (count bc))))]
       (concat len-diff child-diffs)))))
