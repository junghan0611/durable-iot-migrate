(ns iot.semantic.parser.multi-platform-test
  "5 플랫폼 파서 통합 테스트 — Go 테스트와 동일한 데이터."
  (:require [clojure.test :refer [deftest is testing]]
            [iot.semantic.parser.smartthings :as st]
            [iot.semantic.parser.tuya :as tuya]
            [iot.semantic.parser.google :as google]
            [iot.semantic.parser.homey :as homey]
            [iot.semantic.expr :as e]
            [iot.semantic.equiv :as eq]))

;; ═══════════════════════════════════════════════════════════
;; SmartThings
;; ═══════════════════════════════════════════════════════════

(def st-json "[
  {\"name\": \"Turn on light when door opens\",
   \"id\": \"st-rule-001\",
   \"actions\": [{\"if\": {
     \"equals\": {
       \"left\": {\"device\": {\"deviceId\": \"contact-01\", \"component\": \"main\",
                  \"capability\": \"contactSensor\", \"attribute\": \"contact\"}},
       \"right\": {\"string\": \"open\"}},
     \"then\": [{\"command\": {\"devices\": [\"light-01\"],
                \"commands\": [{\"component\": \"main\", \"capability\": \"switch\",
                               \"command\": \"on\"}]}}]}}]},
  {\"name\": \"Temperature alert\",
   \"id\": \"st-rule-002\",
   \"actions\": [{\"if\": {
     \"greaterThan\": {
       \"left\": {\"device\": {\"deviceId\": \"temp-01\", \"component\": \"main\",
                  \"capability\": \"temperatureMeasurement\", \"attribute\": \"temperature\"}},
       \"right\": {\"integer\": 30}},
     \"then\": [{\"command\": {\"devices\": [\"ac-01\"],
                \"commands\": [{\"component\": \"main\", \"capability\": \"switch\",
                               \"command\": \"on\"}]}},
               {\"sleep\": {\"duration\": {\"value\": 300, \"unit\": \"Second\"}}}]}}]},
  {\"name\": \"Sunset routine\",
   \"id\": \"st-rule-003\",
   \"actions\": [{\"every\": {
     \"specific\": {\"reference\": \"Sunset\", \"offset\": {\"value\": -30, \"unit\": \"Minute\"}},
     \"actions\": [{\"command\": {\"devices\": [\"light-02\"],
                    \"commands\": [{\"component\": \"main\", \"capability\": \"switchLevel\",
                                   \"command\": \"setLevel\", \"arguments\": [50]}]}}]}}]}
]")

(deftest smartthings-parse
  (testing "SmartThings Rules API JSON 파싱"
    (let [recipes (st/parse-string st-json)]
      (is (= 3 (count recipes)))

      ;; Rule 1: door → light
      (let [r (nth recipes 0)]
        (is (= "Turn on light when door opens" (:name r)))
        (is (= :eq (get-in r [:trigger :op]))))

      ;; Rule 2: temp > 30 → AC on + delay
      (let [r (nth recipes 1)]
        (is (= :gt (get-in r [:trigger :op])))
        (is (= :seq (get-in r [:actions :op])))
        (is (= 2 (count (get-in r [:actions :children])))))

      ;; Rule 3: sunset → lights
      (let [r (nth recipes 2)]
        (is (= :eq (get-in r [:trigger :op])))))))

;; ═══════════════════════════════════════════════════════════
;; Tuya
;; ═══════════════════════════════════════════════════════════

(def tuya-json "[
  {\"scene_id\": \"tuya-001\",
   \"name\": \"Night Mode\",
   \"conditions\": [
     {\"entity_type\": 1, \"entity_id\": \"dev-01\",
      \"expr\": {\"dp_id\": \"1\", \"comparator\": \"==\", \"value\": true}}],
   \"preconditions\": [
     {\"cond_type\": \"timeCheck\", \"expr\": {\"start\": \"22:00\", \"end\": \"06:00\"}}],
   \"actions\": [
     {\"entity_type\": 1, \"entity_id\": \"dev-02\",
      \"executor_property\": {\"dp_id\": \"1\", \"value\": false}},
     {\"entity_type\": 7, \"entity_id\": \"\",
      \"executor_property\": {\"seconds\": 10}},
     {\"entity_type\": 2, \"entity_id\": \"\",
      \"executor_property\": {\"message\": \"Night mode on\"}}]}
]")

(deftest tuya-parse
  (testing "Tuya Scene JSON 파싱"
    (let [recipes (tuya/parse-string tuya-json)]
      (is (= 1 (count recipes)))

      (let [r (first recipes)]
        (is (= "Night Mode" (:name r)))
        ;; trigger: dp_1 == true
        (is (= :eq (get-in r [:trigger :op])))
        ;; condition: time between 22:00-06:00
        (is (= :between (get-in r [:condition :op])))
        ;; actions: cmd + delay + notify
        (is (= :seq (get-in r [:actions :op])))
        (is (= 3 (count (get-in r [:actions :children]))))
        (is (= :command (get-in r [:actions :children 0 :op])))
        (is (= :delay   (get-in r [:actions :children 1 :op])))
        (is (= :notify  (get-in r [:actions :children 2 :op])))))))

;; ═══════════════════════════════════════════════════════════
;; Google Home
;; ═══════════════════════════════════════════════════════════

(def google-yaml "
metadata:
  name: 'Sunset lights'
automations:
  - starters:
      - type: device.state
        device: 'Motion Sensor'
        state: motion
        is: detected
    condition:
      type: device.state
      device: 'Light'
      state: on
      is: false
    actions:
      - type: device.command.OnOff
        devices: 'Light'
        on: true
")

(deftest google-parse
  (testing "Google Home Scripted YAML 파싱"
    (let [recipes (google/parse-string google-yaml)]
      (is (= 1 (count recipes)))

      (let [r (first recipes)]
        (is (= "Sunset lights" (:name r)))
        (is (= :eq (get-in r [:trigger :op])))
        (is (= :eq (get-in r [:condition :op])))
        (is (= :command (get-in r [:actions :op])))))))

;; ═══════════════════════════════════════════════════════════
;; Homey
;; ═══════════════════════════════════════════════════════════

(def homey-json "[
  {\"id\": \"homey-001\",
   \"name\": \"Motion Light\",
   \"enabled\": true,
   \"trigger\": {
     \"uri\": \"homey:device:sensor01\",
     \"id\": \"alarm_motion_true\",
     \"args\": {\"device\": {\"id\": \"sensor01\"}}},
   \"conditions\": [
     {\"uri\": \"homey:device:light01\",
      \"id\": \"onoff_false\",
      \"args\": {\"device\": {\"id\": \"light01\"}},
      \"inverted\": false}],
   \"actions\": [
     {\"uri\": \"homey:device:light01\",
      \"id\": \"on\",
      \"args\": {\"device\": {\"id\": \"light01\"}}},
     {\"uri\": \"homey:manager:flow\",
      \"id\": \"delay\",
      \"args\": {\"delay\": 300}}]}
]")

(deftest homey-parse
  (testing "Homey Flow JSON 파싱"
    (let [recipes (homey/parse-string homey-json)]
      (is (= 1 (count recipes)))

      (let [r (first recipes)]
        (is (= "Motion Light" (:name r)))
        ;; trigger: alarm_motion == true
        (is (= :eq (get-in r [:trigger :op])))
        ;; condition: onoff == false (not inverted)
        (is (= :eq (get-in r [:condition :op])))
        ;; actions: cmd + delay
        (is (= :seq (get-in r [:actions :op])))
        (is (= 2 (count (get-in r [:actions :children]))))))))

;; ═══════════════════════════════════════════════════════════
;; 교차 플랫폼 구조 동치
;; ═══════════════════════════════════════════════════════════

(deftest cross-platform-motion-structural
  (testing "모든 플랫폼의 '움직임 → 불 켜기'가 구조적으로 동치"
    ;; 각 플랫폼의 trigger는 전부 (eq (state-ref ...) (lit ...))
    (let [ha-trigger    (e/eq (e/state-ref "sensor" "motion") (e/lit "on"))
          st-trigger    (get-in (first (st/parse-string st-json)) [:trigger])
          tuya-trigger  (get-in (first (tuya/parse-string tuya-json)) [:trigger])
          homey-trigger (get-in (first (homey/parse-string homey-json)) [:trigger])
          google-trigger (get-in (first (google/parse-string google-yaml)) [:trigger])]

      ;; 전부 :eq 구조
      (is (= :eq (:op ha-trigger)))
      (is (= :eq (:op st-trigger)))
      (is (= :eq (:op tuya-trigger)))
      (is (= :eq (:op homey-trigger)))
      (is (= :eq (:op google-trigger)))

      ;; 구조적 동치: 전부 (eq X Y) 형태
      (doseq [[name t] [["HA" ha-trigger] ["ST" st-trigger]
                         ["Tuya" tuya-trigger] ["Homey" homey-trigger]
                         ["Google" google-trigger]]]
        (is (eq/structural-equiv? ha-trigger t)
            (str "HA ≡ " name " (structural)"))))))
