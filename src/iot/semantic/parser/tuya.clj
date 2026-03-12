(ns iot.semantic.parser.tuya
  "Tuya Scene/Tap-to-Run JSON → Expr 레시피 변환.

  Tuya는 비교 연산자를 명시적으로 가지고 있어서 Expr 매핑이 가장 직접적이다:
  {\"dp_id\": \"1\", \"comparator\": \"==\", \"value\": true}
  → (eq (state-ref device \"dp_1\") (lit true))

  entity_type: 1=device, 2=notification, 3=weather, 5=geofence, 6=timer, 7=delay"
  (:require [clojure.data.json :as json]
            [iot.semantic.expr :as e]))

;; ─── comparator → Expr op ────────────────────────────────

(def ^:private comparator->fn
  {"==" e/eq  "!=" e/ne
   ">"  e/gt  ">=" e/ge
   "<"  e/lt  "<=" e/le})

(defn- tuya-comparison
  "Tuya expr map → Expr 비교 노드."
  [device-id expr-map]
  (let [dp-id (or (:dp_id expr-map) (get expr-map "dp_id"))
        comp  (or (:comparator expr-map) (get expr-map "comparator"))
        value (let [v (or (:value expr-map) (get expr-map "value"))] v)]
    (when dp-id
      (let [attr  (str "dp_" dp-id)
            op-fn (get comparator->fn comp e/eq)]
        (op-fn (e/state-ref device-id attr) (e/lit value))))))

;; ─── 트리거 (Tuya conditions) ────────────────────────────

(defn- convert-condition
  "Tuya condition (= trigger) → Expr."
  [{:keys [entity_type entity_id expr]}]
  (let [et (if (number? entity_type) (int entity_type) 0)]
    (case et
      1 ;; device DP
      (or (tuya-comparison entity_id expr)
          {:op :unknown-trigger :value "device" :meta expr})

      3 ;; sun/weather
      (e/eq (e/time-ref (or (:event expr) (get expr "event") "sunset"))
            (e/lit "0"))

      6 ;; timer
      (e/eq (e/time-ref "schedule") (e/lit (or (:timer expr) (get expr "timer") "")))

      5 ;; geofence
      {:op :geofence :meta expr}

      ;; default
      {:op :unknown-trigger :value et :meta expr})))

;; ─── 프리컨디션 (guard) ──────────────────────────────────

(defn- convert-precondition
  "Tuya precondition → Expr."
  [{:keys [cond_type expr]}]
  (case cond_type
    "timeCheck"
    (e/between (e/time-ref "now")
               (e/lit (or (:start expr) (get expr "start") "00:00"))
               (e/lit (or (:end expr) (get expr "end") "23:59")))

    ;; default
    {:op :unknown-condition :value cond_type :meta expr}))

;; ─── 액션 변환 ───────────────────────────────────────────

(defn- to-number [v]
  (cond (number? v) v
        (string? v) (try (Double/parseDouble v) (catch Exception _ 0))
        :else 0))

(defn- convert-action
  "Tuya action → Expr."
  [{:keys [entity_type entity_id executor_property action_executor]}]
  (let [et (if (number? entity_type) (int entity_type) 0)]
    (case et
      1 ;; device command
      (let [dp-id (or (:dp_id executor_property) (get executor_property "dp_id"))
            value (or (:value executor_property) (get executor_property "value"))]
        (-> (e/cmd entity_id (str "dp_" dp-id) value)
            (cond-> action_executor (e/with-meta* :tuya/executor action_executor))))

      2 ;; notification
      (e/notify (or (:message executor_property)
                    (get executor_property "message") ""))

      3 ;; scene
      (e/scene entity_id)

      7 ;; delay
      (let [secs (to-number (or (:seconds executor_property)
                                (get executor_property "seconds") 0))
            mins (to-number (or (:minutes executor_property)
                                (get executor_property "minutes") 0))]
        (e/delay-expr (+ secs (* mins 60))))

      ;; default
      {:op :unknown-action :value et :meta executor_property})))

;; ─── Public API ──────────────────────────────────────────

(defn- combine [combinator exprs]
  (case (count exprs)
    0 nil
    1 (first exprs)
    (apply combinator exprs)))

(defn- convert-scene [scene]
  (let [triggers (mapv convert-condition  (:conditions scene []))
        guards   (mapv convert-precondition (:preconditions scene []))
        actions  (mapv convert-action     (:actions scene []))]
    (e/recipe
     :id   (:scene_id scene)
     :name (:name scene)
     :trigger   (combine e/and-expr triggers)
     :condition (combine e/and-expr guards)
     :actions   (combine e/seq-expr actions))))

(defn parse-string
  "Tuya Scene JSON 문자열 → recipe 벡터."
  [json-str]
  (let [parsed (json/read-str json-str :key-fn keyword)]
    (if (sequential? parsed)
      (mapv convert-scene parsed)
      [(convert-scene parsed)])))
