(ns iot.semantic.expr-test
  "아기방 카메라부터 온도 센서까지 — 실제 IoT 레시피를 Clojure로.

  Go에서 170줄 struct + 생성자가 여기서는 그냥 맵이다.
  테스트가 곧 문서이고, 데이터가 곧 코드다."
  (:require [clojure.test :refer [deftest is testing]]
            [iot.semantic.expr :as e]
            [iot.semantic.equiv :as eq]))

;; ═══════════════════════════════════════════════════════════
;; 1. 데이터가 곧 코드 — S-expression으로 레시피 만들기
;; ═══════════════════════════════════════════════════════════

(deftest expr-is-just-data
  (testing "Expr은 특별한 타입이 아니다. 그냥 맵이다."
    (let [motion (e/eq (e/state-ref "sensor" "motion")
                       (e/lit true))]
      (is (= :eq (:op motion)))
      (is (= 2 (count (:children motion))))
      (is (map? motion) "맵이니까 assoc, dissoc, merge 전부 된다")

      ;; Go에서는 불가능한 것: 맵처럼 다루기
      (is (= :eq (:op (assoc motion :meta {:origin "test"})))
          "아무 키나 붙일 수 있다 — 열린 확장"))))

(deftest recipe-baby-camera
  (testing "아기방 움직임 → 녹화 + 알림 — 안전 최우선 레시피"
    (let [r (e/recipe
             :id "baby_camera_motion"
             :name "아기방 움직임 감지"
             :trigger (e/eq (e/state-ref "camera.baby" "motion")
                            (e/lit true))
             :condition (e/between (e/time-ref "now")
                                   (e/lit "22:00")
                                   (e/lit "06:00"))
             :actions (e/seq-expr
                       (e/cmd "camera.baby" "recording" "start")
                       (e/delay-expr 5)
                       (e/notify "아기방 움직임 감지!")))]

      (is (= "아기방 움직임 감지" (:name r)))
      (is (= :eq (get-in r [:trigger :op])))
      (is (= :seq (get-in r [:actions :op])))
      (is (= 3 (count (get-in r [:actions :children]))))

      ;; 디바이스 참조 수집
      (is (= #{"camera.baby"}
             (e/device-refs (:actions r)))))))

;; ═══════════════════════════════════════════════════════════
;; 2. 교차 플랫폼 동치 — 같은 규칙, 다른 표현
;; ═══════════════════════════════════════════════════════════

(deftest cross-platform-motion-light
  (testing "5개 플랫폼이 '움직임 감지 → 불 켜기'를 다르게 표현한다"
    (let [tuya   (e/eq (e/state-ref "tuya-pir"   "dp_1")           (e/lit true))
          ha     (e/eq (e/state-ref "sensor.pir"  "state")          (e/lit "on"))
          st     (e/eq (e/state-ref "st-pir"      "motion")         (e/lit "active"))
          homey  (e/eq (e/state-ref "homey-pir"   "alarm_motion")   (e/lit true))
          google (e/eq (e/state-ref "google-pir"  "MotionDetection") (e/lit "motionDetected"))
          all    [tuya ha st homey google]]

      (testing "구조적으로 전부 같다: (eq (state-ref ...) (lit ...))"
        (doseq [i (range (count all))
                j (range (inc i) (count all))]
          (is (eq/structural-equiv? (nth all i) (nth all j))
              (str "platform " i " ≡ platform " j)))))))

(deftest commutative-and-or
  (testing "And/Or은 교환법칙 성립 — (and A B) ≡ (and B A)"
    (let [temp-high  (e/gt (e/state-ref "s" "temperature") (e/lit 25))
          humid-high (e/gt (e/state-ref "s" "humidity")    (e/lit 80))
          night      (e/between (e/time-ref "now") (e/lit "22:00") (e/lit "06:00"))

          ;; 플랫폼 A: (temp OR humid) AND night
          expr-a (e/and-expr (e/or-expr temp-high humid-high) night)
          ;; 플랫폼 B: night AND (humid OR temp) — 순서 뒤바뀜
          expr-b (e/and-expr night (e/or-expr humid-high temp-high))]

      (is (eq/equiv? expr-a expr-b)
          "같은 의미다 — 순서만 다르다"))))

(deftest seq-order-matters
  (testing "Seq는 순서가 의미다 — 켜고→끄기 ≠ 끄고→켜기"
    (let [on  (e/cmd "light" "state" "on")
          off (e/cmd "light" "state" "off")
          w   (e/delay-expr 300)]

      (is (not (eq/equiv? (e/seq-expr on w off)
                          (e/seq-expr off w on)))
          "순서가 다르면 의미가 다르다")

      (is (eq/structural-equiv? (e/seq-expr on w off)
                                (e/seq-expr off w on))
          "하지만 구조는 같다 (cmd, delay, cmd)"))))

;; ═══════════════════════════════════════════════════════════
;; 3. 값 매핑 — true ↔ "on" ↔ "active"
;; ═══════════════════════════════════════════════════════════

(deftest value-mapping
  (testing "다른 플랫폼의 값이 같은 의미인지 매핑으로 검증"
    (let [tuya  (e/eq (e/state-ref "dev" "motion") (e/lit true))
          ha    (e/eq (e/state-ref "dev" "motion") (e/lit "on"))
          st    (e/eq (e/state-ref "dev" "motion") (e/lit "active"))

          mapper (fn [_ref v]
                   (case v
                     (true "on" "active") :motion-detected
                     (false "off" "inactive") :no-motion
                     v))]

      (is (not (eq/equiv? tuya ha))
          "매핑 없이는 true ≠ \"on\"")

      (is (eq/equiv? tuya ha mapper)
          "매핑 있으면 true ≡ \"on\"")
      (is (eq/equiv? ha st mapper)
          "\"on\" ≡ \"active\"")
      (is (eq/equiv? tuya st mapper)
          "true ≡ \"active\""))))

;; ═══════════════════════════════════════════════════════════
;; 4. diff — 어디서 갈라졌는가
;; ═══════════════════════════════════════════════════════════

(deftest diff-report
  (testing "equiv?가 false일 때, diff가 '어디서'를 알려준다"
    (let [a (e/and-expr (e/eq (e/state-ref "s" "temp") (e/lit 25))
                        (e/eq (e/state-ref "s" "humid") (e/lit 80)))
          ;; b는 80 대신 90
          b (e/and-expr (e/eq (e/state-ref "s" "temp") (e/lit 25))
                        (e/eq (e/state-ref "s" "humid") (e/lit 90)))
          d (eq/diff a b)]

      (is (not (eq/equiv? a b)))
      (is (seq d) "차이가 있다")
      (is (some #(= :value-mismatch (:type %)) d)
          "값 불일치를 보고한다"))))

;; ═══════════════════════════════════════════════════════════
;; 5. 트리 유틸 — walk, fold, depth
;; ═══════════════════════════════════════════════════════════

(deftest tree-utilities
  (testing "walk-expr: 트리의 모든 노드를 변환"
    (let [expr (e/and-expr (e/eq (e/state-ref "s" "t") (e/lit 25))
                           (e/eq (e/state-ref "s" "h") (e/lit 80)))
          ;; 모든 노드에 :visited 표시
          walked (e/walk-expr #(assoc % :visited true) expr)]
      (is (:visited walked))
      (is (every? :visited (:children walked)))))

  (testing "depth와 node-count"
    (let [simple (e/eq (e/state-ref "s" "t") (e/lit 25))
          nested (e/and-expr simple (e/or-expr simple simple))]
      (is (= 2 (e/depth simple)))    ; eq(1) → state-ref|lit(1)
      (is (= 3 (e/node-count simple)))
      (is (= 4 (e/depth nested)))    ; and → or → eq → leaf
      (is (= 11 (e/node-count nested))))))  ; and(1)+eq(3)+or(1)+eq(3)+eq(3))

;; ═══════════════════════════════════════════════════════════
;; 6. 열린 확장 — 플랫폼 전용 Op
;; ═══════════════════════════════════════════════════════════

(deftest open-extension
  (testing "새 Op은 키워드 하나면 된다 — 코드 수정 없음"
    (let [;; HA의 choose (if-else 분기) — Go의 22종 Op에 없던 것
          ha-choose {:op :ha/choose
                     :children [{:op :eq
                                 :children [(e/state-ref "s" "t") (e/lit "on")]
                                 :then (e/cmd "light" "state" "on")}
                                {:op :eq
                                 :children [(e/state-ref "s" "t") (e/lit "off")]
                                 :then (e/cmd "light" "state" "off")}]}]

      (is (= :ha/choose (:op ha-choose))
          "네임스페이스 키워드로 충돌 없이 확장")

      ;; walk-expr도 그냥 작동한다 — Op을 몰라도 children만 있으면
      (is (= :ha/choose (:op (e/walk-expr identity ha-choose)))
          "walk는 구조만 보고 순회 — Op을 알 필요 없다"))))
