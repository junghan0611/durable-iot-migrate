(ns iot.semantic.parser.homeassistant-test
  "HA parser 테스트 — Go 테스트(archive/go)와 동일한 YAML 사용."
  (:require [clojure.test :refer [deftest is testing]]
            [iot.semantic.parser.homeassistant :as ha]
            [iot.semantic.expr :as e]
            [iot.semantic.equiv :as eq]))

;; Go 테스트에서 가져온 실제 HA YAML
(def ha-yaml "
- id: '1001'
  alias: 'Turn on lights at sunset'
  triggers:
    - trigger: sun
      event: sunset
      offset: '-01:00:00'
  conditions:
    - condition: state
      entity_id: group.people
      state: 'home'
  actions:
    - action: light.turn_on
      target:
        entity_id: light.living_room

- id: '1002'
  alias: 'Morning routine'
  triggers:
    - trigger: time
      at: '07:00:00'
  conditions:
    - condition: time
      after: '06:00:00'
      before: '08:00:00'
  actions:
    - action: light.turn_on
      target:
        entity_id: light.bedroom
      data:
        brightness_pct: 50
    - delay: '00:05:00'
    - action: media_player.play_media
      target:
        entity_id: media_player.kitchen
      data:
        media_content_id: 'http://radio.example.com'
        media_content_type: 'music'

- id: '1003'
  alias: 'Doorbell camera notification'
  triggers:
    - trigger: state
      entity_id: binary_sensor.doorbell
      to: 'on'
  actions:
    - action: notify.mobile_app
      data:
        message: 'Someone is at the door!'

- id: '1004'
  alias: 'Temperature alert'
  triggers:
    - trigger: state
      entity_id: sensor.temperature
  conditions:
    - condition: numeric_state
      entity_id: sensor.temperature
      above: 30
  actions:
    - action: climate.set_temperature
      target:
        entity_id: climate.living_room
      data:
        temperature: 24
    - action: notify.notify
      data:
        message: 'Temperature above 30C, AC activated'

- id: '1005'
  alias: 'Activate scene at night'
  triggers:
    - trigger: time
      at: '22:00:00'
  actions:
    - scene: scene.good_night
")

(deftest parse-basic
  (testing "5개 HA 자동화 파싱"
    (let [recipes (ha/parse-string ha-yaml)]
      (is (= 5 (count recipes)))

      ;; Automation 1: Sun trigger
      (let [r (nth recipes 0)]
        (is (= "1001" (:id r)))
        (is (= "Turn on lights at sunset" (:name r)))
        (is (= :eq (get-in r [:trigger :op])))
        (is (= :eq (get-in r [:condition :op])))
        (is (= :command (get-in r [:actions :op])))
        (is (contains? (e/device-refs-recipe r) "light.living_room")))

      ;; Automation 2: Schedule + delay (3 actions → seq)
      (let [r (nth recipes 1)]
        (is (= "Morning routine" (:name r)))
        (is (= :eq (get-in r [:trigger :op])))
        (is (= :between (get-in r [:condition :op])))
        (is (= :seq (get-in r [:actions :op])))
        (is (= 3 (count (get-in r [:actions :children])))))

      ;; Automation 3: Device state + notify
      (let [r (nth recipes 2)]
        (is (= "Doorbell camera notification" (:name r)))
        (is (= :eq (get-in r [:trigger :op])))
        (is (= :notify (get-in r [:actions :op]))))

      ;; Automation 4: Numeric condition (above: 30)
      (let [r (nth recipes 3)]
        (is (= :gt (get-in r [:condition :op])))
        (is (= :seq (get-in r [:actions :op])))
        (is (= 2 (count (get-in r [:actions :children])))))

      ;; Automation 5: Scene
      (let [r (nth recipes 4)]
        (is (= :scene (get-in r [:actions :op])))))))

(deftest parse-legacy
  (testing "legacy format: platform → trigger, service → action"
    (let [legacy "
- id: 'legacy-001'
  alias: 'Legacy automation'
  triggers:
    - platform: state
      entity_id: switch.garage
      to: 'off'
  actions:
    - service: switch.turn_on
      target:
        entity_id: switch.garage
"
          recipes (ha/parse-string legacy)]
      (is (= 1 (count recipes)))
      (is (= :eq (get-in (first recipes) [:trigger :op])))
      (is (= :command (get-in (first recipes) [:actions :op]))))))

(deftest parse-empty
  (testing "빈 입력"
    (let [recipes (ha/parse-string "[]")]
      (is (empty? recipes)))))

(deftest expr-structural-equiv
  (testing "같은 HA YAML을 두 번 파싱하면 구조적으로 동치"
    (let [a (ha/parse-string ha-yaml)
          b (ha/parse-string ha-yaml)]
      (doseq [i (range (count a))]
        (is (eq/structural-equiv? (:trigger (nth a i))
                                  (:trigger (nth b i)))
            (str "trigger " i " structural equiv"))
        (is (eq/structural-equiv? (:actions (nth a i))
                                  (:actions (nth b i)))
            (str "actions " i " structural equiv"))))))

(deftest device-refs-collected
  (testing "레시피에서 디바이스 참조가 올바르게 수집되는가"
    (let [recipes (ha/parse-string ha-yaml)
          ;; Morning routine: light.bedroom, media_player.kitchen
          r2-refs (e/device-refs-recipe (nth recipes 1))]
      (is (contains? r2-refs "light.bedroom"))
      (is (contains? r2-refs "media_player.kitchen")))))
