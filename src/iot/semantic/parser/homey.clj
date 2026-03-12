(ns iot.semantic.parser.homey
  "Homey Flow JSON → Expr 레시피 변환.

  Homey는 'card' 시스템: When(trigger) → And(conditions) → Then(actions).
  URI 기반: homey:device:sensor01, homey:manager:flow 등."
  (:require [clojure.data.json :as json]
            [clojure.string :as str]
            [iot.semantic.expr :as e]))

;; ─── URI → device ID ─────────────────────────────────────

(defn- card-device-id
  "카드에서 디바이스 ID 추출. URI 또는 args.device.id."
  [{:keys [uri args]}]
  (or (get-in args [:device :id])
      (when (and uri (str/starts-with? uri "homey:device:"))
        (subs uri (count "homey:device:")))))

(defn- card-capability
  "카드 id에서 capability 추출. 'alarm_motion_true' → 'alarm_motion'"
  [card-id]
  (when card-id
    (let [parts (str/split card-id #"_")]
      (if (and (> (count parts) 1)
               (#{"true" "false" "on" "off"} (last parts)))
        (str/join "_" (butlast parts))
        card-id))))

(defn- card-value
  "카드 id에서 값 추출. 'alarm_motion_true' → true"
  [card-id]
  (when card-id
    (let [suffix (last (str/split card-id #"_"))]
      (case suffix
        "true"  true
        "false" false
        "on"    "on"
        "off"   "off"
        card-id))))

;; ─── 카드 변환 ───────────────────────────────────────────

(defn- convert-trigger-card
  "Homey trigger card → Expr."
  [{:keys [uri id args] :as card}]
  (cond
    ;; 디바이스 트리거
    (and uri (str/starts-with? uri "homey:device:"))
    (let [device (card-device-id card)
          cap    (card-capability id)
          val    (card-value id)]
      (-> (e/eq (e/state-ref device cap) (e/lit val))
          (e/with-meta* :homey/uri uri)
          (e/with-meta* :homey/card-id id)))

    ;; 시간 트리거
    (and uri (str/starts-with? uri "homey:manager:cron"))
    (-> (e/eq (e/time-ref "schedule") (e/lit (get args :time "")))
        (e/with-meta* :homey/uri uri))

    ;; 기타
    :else
    {:op :unknown-trigger :value id :meta {:uri uri :args args}}))

(defn- convert-condition-card
  "Homey condition card → Expr."
  [{:keys [uri id args inverted] :as card}]
  (let [base (cond
               (and uri (str/starts-with? uri "homey:device:"))
               (let [device (card-device-id card)
                     cap    (card-capability id)
                     val    (card-value id)]
                 (e/eq (e/state-ref device cap) (e/lit val)))

               (and uri (str/starts-with? uri "homey:manager:cron"))
               (e/between (e/time-ref "now")
                          (e/lit (get args :startTime "00:00"))
                          (e/lit (get args :endTime "23:59")))

               :else
               {:op :unknown-condition :value id :meta {:uri uri}})]
    (if inverted
      (e/not-expr base)
      base)))

(defn- convert-action-card
  "Homey action card → Expr."
  [{:keys [uri id args] :as card}]
  (cond
    ;; delay
    (and uri (= uri "homey:manager:flow") (= id "delay"))
    (e/delay-expr (get args :delay 0))

    ;; 디바이스 커맨드
    (and uri (str/starts-with? uri "homey:device:"))
    (let [device (card-device-id card)]
      (-> (e/cmd device id (get args :value nil))
          (e/with-meta* :homey/uri uri)))

    ;; 다른 Flow 실행
    (and uri (str/starts-with? uri "homey:manager:flow"))
    (e/scene (get-in args [:flow :id] id))

    :else
    {:op :unknown-action :value id :meta {:uri uri :args args}}))

;; ─── Public API ──────────────────────────────────────────

(defn- combine [combinator exprs]
  (case (count exprs)
    0 nil
    1 (first exprs)
    (apply combinator exprs)))

(defn- convert-flow [{:keys [id name trigger conditions actions]}]
  (let [trigger-expr  (when trigger (convert-trigger-card trigger))
        cond-exprs    (mapv convert-condition-card (or conditions []))
        action-exprs  (mapv convert-action-card (or actions []))]
    (e/recipe
     :id   id
     :name name
     :trigger   trigger-expr
     :condition (combine e/and-expr cond-exprs)
     :actions   (combine e/seq-expr action-exprs))))

(defn parse-string
  "Homey Flow JSON → recipe 벡터."
  [json-str]
  (let [parsed (json/read-str json-str :key-fn keyword)]
    (if (sequential? parsed)
      (mapv convert-flow parsed)
      [(convert-flow parsed)])))
