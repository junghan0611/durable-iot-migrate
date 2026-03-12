(ns iot.semantic.parser.homeassistant
  "Home Assistant 자동화 YAML → Expr 레시피 변환.

  Go 버전(archive/go/converters/homeassistant/parser.go)에서는
  YAML → struct → Config map[string]any 경로를 거쳤다.
  Clojure에서는 YAML → map → Expr map으로 직행한다.

  중간 struct가 없다. YAML이 이미 맵이고, Expr도 맵이니까.

  HA YAML 포맷:
    - id: '1234'
      alias: 'Turn on lights at sunset'
      triggers:
        - trigger: sun          # 또는 platform: sun (legacy)
          event: sunset
      conditions:
        - condition: state
          entity_id: group.people
          state: 'home'
      actions:
        - action: light.turn_on  # 또는 service: (legacy)
          target:
            entity_id: light.living_room"
  (:require [clj-yaml.core :as yaml]
            [iot.semantic.expr :as e]
            [clojure.string :as str]))

;; ─── 트리거 변환 ─────────────────────────────────────────

(defn- entity-id
  "entity_id를 문자열로 정규화. 배열이면 첫 번째."
  [v]
  (cond
    (string? v) v
    (sequential? v) (first v)
    :else nil))

(defn- convert-trigger
  "HA trigger map → Expr"
  [{:keys [trigger platform entity_id event offset to from at] :as t}]
  (let [trig-type (or trigger platform)]
    (case trig-type
      "state"
      (let [device (entity-id entity_id)]
        (cond-> (e/eq (e/state-ref device "state")
                      (e/lit (or to "any")))
          from (e/with-meta* :ha/from from)
          true (e/with-meta* :ha/trigger-type "state")))

      "time"
      (-> (e/eq (e/time-ref "schedule") (e/lit at))
          (e/with-meta* :ha/trigger-type "schedule"))

      "sun"
      (cond-> (e/eq (e/time-ref (or event "sunset"))
                    (e/lit (or offset "0")))
        true (e/with-meta* :ha/trigger-type "sun"))

      "webhook"
      (-> {:op :webhook :meta {:ha/trigger-type "webhook"}}
          (merge (when-let [wid (:webhook_id t)] {:value wid})))

      ;; zone, numeric_state 등
      (-> {:op :unknown-trigger
           :value trig-type
           :meta (merge {:ha/trigger-type trig-type}
                        (dissoc t :trigger :platform))}))))

;; ─── 조건 변환 ─────────────────────────────────────────

(defn- convert-condition
  "HA condition map → Expr"
  [{:keys [condition entity_id state after before above below] :as c}]
  (case condition
    "state"
    (e/eq (e/state-ref (entity-id entity_id) "state")
          (e/lit state))

    "time"
    (e/between (e/time-ref "now")
               (e/lit (or after "00:00"))
               (e/lit (or before "23:59")))

    "numeric_state"
    (let [device (entity-id entity_id)]
      (cond
        (and above below)
        (e/between (e/state-ref device "value")
                   (e/lit above)
                   (e/lit below))
        above
        (e/gt (e/state-ref device "value") (e/lit above))
        below
        (e/lt (e/state-ref device "value") (e/lit below))
        :else
        (e/eq (e/state-ref device "value") (e/lit nil))))

    "and" {:op :and :children [] :meta {:ha/condition-type "and"}}
    "or"  {:op :or  :children [] :meta {:ha/condition-type "or"}}

    ;; 알 수 없는 조건 — 정보 보존
    {:op :unknown-condition
     :value condition
     :meta (dissoc c :condition)}))

;; ─── 액션 변환 ─────────────────────────────────────────

(defn- target-entity
  "target에서 entity_id 추출."
  [target]
  (when target
    (entity-id (or (:entity_id target)
                   (:device_id target)
                   (:area_id target)))))

(defn- convert-action
  "HA action map → Expr"
  [{:keys [action service target data delay scene] :as a}]
  (cond
    ;; delay
    delay
    (e/delay-expr (cond
                    (number? delay) delay
                    (string? delay) delay  ; "00:05:00" 형식 보존
                    (map? delay) delay
                    :else 0))

    ;; scene
    (some? scene)
    (e/scene scene)

    ;; action/service (device command or notify)
    :else
    (let [svc (or action service "unknown")]
      (if (or (= svc "notify") (str/starts-with? svc "notify."))
        ;; notify
        (-> (e/notify (get-in data [:message] ""))
            (e/with-meta* :ha/service svc)
            (cond-> data (e/with-meta* :ha/data data)))
        ;; device command
        (let [device (target-entity target)]
          (-> (e/cmd (or device "unknown") svc nil)
              (e/with-meta* :ha/service svc)
              (cond-> target (e/with-meta* :ha/target target))
              (cond-> data   (e/with-meta* :ha/data data))))))))

;; ─── 레시피 조립 ─────────────────────────────────────────

(defn- combine-exprs
  "여러 Expr을 하나로 결합. 1개면 그대로, 2개 이상이면 and/seq."
  [combinator exprs]
  (case (count exprs)
    0 nil
    1 (first exprs)
    (apply combinator exprs)))

(defn- convert-automation
  "하나의 HA automation map → recipe."
  [{:keys [id alias triggers conditions actions]}]
  (let [trigger-exprs  (mapv convert-trigger  (or triggers []))
        condition-exprs (mapv convert-condition (or conditions []))
        action-exprs   (mapv convert-action    (or actions []))]
    (e/recipe
     :id id
     :name alias
     :trigger   (combine-exprs e/and-expr trigger-exprs)
     :condition (combine-exprs e/and-expr condition-exprs)
     :actions   (combine-exprs e/seq-expr action-exprs))))

;; ─── Public API ──────────────────────────────────────────

(defn parse-string
  "HA 자동화 YAML 문자열 → recipe 벡터."
  [yaml-str]
  (let [parsed (yaml/parse-string yaml-str)]
    (if (sequential? parsed)
      (mapv convert-automation parsed)
      [(convert-automation parsed)])))

(defn parse-file
  "HA 자동화 YAML 파일 → recipe 벡터."
  [filepath]
  (parse-string (slurp filepath)))
