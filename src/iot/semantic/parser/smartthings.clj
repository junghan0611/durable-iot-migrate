(ns iot.semantic.parser.smartthings
  "SmartThings Rules API JSON → Expr 레시피 변환.

  ST 포맷은 if-then-else 트리 구조:
  {\"actions\": [{\"if\": {\"equals\": {\"left\": {\"device\": ...}, \"right\": ...},
                          \"then\": [{\"command\": ...}]}}]}"
  (:require [clojure.data.json :as json]
            [iot.semantic.expr :as e]))

;; ─── Operand → 값/참조 ──────────────────────────────────

(defn- operand->expr
  "ST operand → Expr leaf"
  [op]
  (cond
    (:device op)
    (let [{:keys [deviceId component capability attribute]} (:device op)]
      (e/state-ref deviceId component (str capability "." attribute)))

    (contains? op :string)  (e/lit (:string op))
    (contains? op :integer) (e/lit (:integer op))
    (contains? op :double)  (e/lit (:double op))
    (contains? op :boolean) (e/lit (:boolean op))
    :else (e/lit nil)))

;; ─── 비교 → Expr ─────────────────────────────────────────

(defn- convert-equals [eq]
  (e/eq (operand->expr (:left eq))
        (operand->expr (:right eq))))

(defn- convert-compare [cmp op-fn]
  (op-fn (operand->expr (:left cmp))
         (operand->expr (:right cmp))))

;; ─── 액션 변환 ───────────────────────────────────────────

(declare convert-actions)

(defn- convert-command [cmd]
  (for [dev (:devices cmd [])
        c   (:commands cmd [])]
    (-> (e/cmd dev (str (:capability c) "." (:command c)) (:arguments c))
        (e/with-meta* :st/component (:component c)))))

(defn- convert-sleep [sleep]
  (let [{:keys [value unit]} (:duration sleep)
        seconds (case unit
                  "Second" (if (number? value) value (parse-long (str value)))
                  "Minute" (* 60 (if (number? value) value (parse-long (str value))))
                  value)]
    (e/delay-expr seconds)))

(defn- convert-if-action [if-action]
  {:trigger (cond
              (:equals if-action)             (convert-equals (:equals if-action))
              (:greaterThan if-action)         (convert-compare (:greaterThan if-action) e/gt)
              (:lessThan if-action)            (convert-compare (:lessThan if-action) e/lt)
              (:greaterThanOrEquals if-action)  (convert-compare (:greaterThanOrEquals if-action) e/ge)
              (:lessThanOrEquals if-action)     (convert-compare (:lessThanOrEquals if-action) e/le))
   :actions (convert-actions (:then if-action []))})

(defn- convert-every [every]
  (let [trigger (cond
                  (:specific every)
                  (let [{:keys [reference offset]} (:specific every)]
                    (case reference
                      ("Sunrise" "Sunset")
                      (e/eq (e/time-ref reference) (e/lit (or (:value offset) "0")))
                      ;; default schedule
                      (e/eq (e/time-ref "schedule") (e/lit reference))))

                  (:interval every)
                  (e/eq (e/time-ref "interval")
                        (e/lit (str (:value (:interval every)) " " (:unit (:interval every))))))]
    {:trigger trigger
     :actions (convert-actions (:actions every []))}))

(defn- convert-actions
  "ST actions 배열 → Expr 리스트"
  [actions]
  (reduce (fn [acc action]
            (cond
              (:command action) (into acc (convert-command (:command action)))
              (:sleep action)   (conj acc (convert-sleep (:sleep action)))
              (:if action)      (let [{:keys [actions]} (convert-if-action (:if action))]
                                  (into acc actions))
              :else acc))
          []
          actions))

;; ─── Public API ──────────────────────────────────────────

(defn- convert-rule [rule]
  (let [parts (reduce (fn [acc action]
                        (cond
                          (:if action)
                          (let [result (convert-if-action (:if action))]
                            (-> acc
                                (update :triggers conj (:trigger result))
                                (update :actions into (:actions result))))

                          (:every action)
                          (let [result (convert-every (:every action))]
                            (-> acc
                                (update :triggers conj (:trigger result))
                                (update :actions into (:actions result))))

                          (:command action)
                          (update acc :actions into (convert-command (:command action)))

                          (:sleep action)
                          (update acc :actions conj (convert-sleep (:sleep action)))

                          :else acc))
                      {:triggers [] :actions []}
                      (:actions rule []))
        triggers (:triggers parts)
        actions  (:actions parts)]
    (e/recipe
     :id   (or (:id rule) (:name rule))
     :name (:name rule)
     :trigger (case (count triggers)
                0 nil
                1 (first triggers)
                (apply e/and-expr triggers))
     :actions (case (count actions)
                0 nil
                1 (first actions)
                (apply e/seq-expr actions)))))

(defn parse-string
  "SmartThings Rules JSON 문자열 → recipe 벡터."
  [json-str]
  (let [parsed (json/read-str json-str :key-fn keyword)]
    (if (sequential? parsed)
      (mapv convert-rule parsed)
      [(convert-rule parsed)])))
