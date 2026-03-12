(ns iot.semantic.parser.google
  "Google Home Scripted Automations YAML → Expr 레시피 변환.

  Google Home 포맷:
    metadata:
      name: 'Turn on lights at sunset'
    automations:
      - starters:
          - type: time.schedule
            at: sunset
        condition:
          type: device.state
          device: 'Living Room Light'
          state: on
          is: false
        actions:
          - type: device.command.OnOff
            devices: 'Living Room Light'
            on: true"
  (:require [clj-yaml.core :as yaml]
            [iot.semantic.expr :as e]))

;; ─── Starter → Expr ──────────────────────────────────────

(defn- convert-starter [{:keys [type at offset device state is]}]
  (case type
    "time.schedule"
    (cond-> (e/eq (e/time-ref (or at "schedule")) (e/lit (or offset "0")))
      true (e/with-meta* :google/starter-type type))

    "device.state"
    (-> (e/eq (e/state-ref device (or state "state")) (e/lit is))
        (e/with-meta* :google/starter-type type))

    ;; sun
    ("time.sunrise" "time.sunset")
    (e/eq (e/time-ref type) (e/lit (or offset "0")))

    ;; default
    {:op :unknown-trigger :value type :meta {:device device :state state :is is}}))

;; ─── Condition → Expr ────────────────────────────────────

(defn- convert-condition [{:keys [type device state is conditions]}]
  (case type
    "device.state"
    (e/eq (e/state-ref device (or state "state")) (e/lit is))

    "and"
    (apply e/and-expr (map convert-condition conditions))

    "or"
    (apply e/or-expr (map convert-condition conditions))

    {:op :unknown-condition :value type}))

;; ─── Action → Expr ──────────────────────────────────────

(defn- devices-list [v]
  (if (sequential? v) v (if v [v] [])))

(defn- convert-action [{:keys [type devices on level] :as a}]
  (cond
    ;; delay
    (= type "delay")
    (e/delay-expr (or (:seconds a) 0))

    ;; scene
    (= type "scene")
    (e/scene (or (:scene a) ""))

    ;; notify
    (= type "assistant.command.Broadcast")
    (e/notify (or (:message a) ""))

    ;; device command
    :else
    (let [devs (devices-list devices)
          cmd-name (or type "unknown")]
      (if (= 1 (count devs))
        (let [value (cond
                      (some? on) on
                      (some? level) level
                      :else nil)]
          (-> (e/cmd (first devs) cmd-name value)
              (e/with-meta* :google/action-type type)))
        ;; 여러 디바이스 → parallel
        (apply e/parallel
               (for [d devs]
                 (e/cmd d cmd-name (or on level))))))))

;; ─── Public API ──────────────────────────────────────────

(defn- combine [combinator exprs]
  (case (count exprs)
    0 nil
    1 (first exprs)
    (apply combinator exprs)))

(defn- convert-automation [name-prefix {:keys [starters condition actions]}]
  (let [trigger-exprs (mapv convert-starter (or starters []))
        cond-expr     (when condition (convert-condition condition))
        action-exprs  (mapv convert-action (or actions []))]
    (e/recipe
     :id   name-prefix
     :name name-prefix
     :trigger   (combine e/and-expr trigger-exprs)
     :condition cond-expr
     :actions   (combine e/seq-expr action-exprs))))

(defn parse-string
  "Google Home Scripted YAML → recipe 벡터."
  [yaml-str]
  (let [parsed (yaml/parse-string yaml-str)
        name   (get-in parsed [:metadata :name] "unnamed")
        autos  (get parsed :automations [])]
    (if (seq autos)
      (mapv (partial convert-automation name) autos)
      [])))
