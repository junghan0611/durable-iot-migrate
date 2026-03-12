(ns iot.semantic.cli
  "durable-iot-migrate CLI 엔트리포인트."
  (:require [iot.semantic.expr :as expr]
            [iot.semantic.equiv :as equiv]
            [iot.semantic.parser.homeassistant :as ha]
            [clojure.data.json :as json]
            [clojure.string :as str])
  (:gen-class))

(defn cmd-parse
  "파일을 파싱하여 Expr 레시피로 변환한다."
  [platform filepath]
  (let [data (slurp filepath)
        recipes (case platform
                  "ha" (ha/parse-string data)
                  (do (println (str "❌ 지원하지 않는 플랫폼: " platform))
                      nil))]
    (when recipes
      (println (str "🔍 " (count recipes) "개 레시피 파싱 완료"))
      (doseq [r recipes]
        (println)
        (println (str "  " (:id r) " — " (:name r)))
        (when-let [t (:trigger r)]
          (println (str "    trigger:   " (:op t))))
        (when-let [c (:condition r)]
          (println (str "    condition: " (:op c))))
        (when-let [a (:actions r)]
          (println (str "    actions:   " (:op a) " (" (count (:children a [])) "개)")))
        (println (str "    devices:   " (str/join ", " (expr/device-refs-recipe r))))))))

(defn cmd-equiv
  "두 파일의 레시피를 비교한다."
  [platform file-a file-b]
  (let [recipes-a (case platform
                    "ha" (ha/parse-string (slurp file-a)))
        recipes-b (case platform
                    "ha" (ha/parse-string (slurp file-b)))]
    (println (str "비교: " (count recipes-a) " vs " (count recipes-b) " 레시피"))
    (doseq [[a b] (map vector recipes-a recipes-b)]
      (let [t-eq (equiv/structural-equiv? (:trigger a) (:trigger b))
            a-eq (equiv/structural-equiv? (:actions a) (:actions b))]
        (println (str "  " (:name a)))
        (println (str "    trigger:  " (if t-eq "≡" "≠")))
        (println (str "    actions:  " (if a-eq "≡" "≠")))))))

(defn cmd-json
  "파일을 파싱하여 JSON으로 출력한다."
  [platform filepath]
  (let [data (slurp filepath)
        recipes (case platform
                  "ha" (ha/parse-string data))]
    (println (json/write-str recipes :indent true))))

(defn -main [& args]
  (let [cmd (first args)
        rest-args (rest args)]
    (case cmd
      "parse" (let [[platform filepath] rest-args]
                (if (and platform filepath)
                  (cmd-parse platform filepath)
                  (println "Usage: durable-iot-migrate parse <platform> <file>")))
      "json"  (let [[platform filepath] rest-args]
                (if (and platform filepath)
                  (cmd-json platform filepath)
                  (println "Usage: durable-iot-migrate json <platform> <file>")))
      "equiv" (let [[platform fa fb] rest-args]
                (if (and platform fa fb)
                  (cmd-equiv platform fa fb)
                  (println "Usage: durable-iot-migrate equiv <platform> <file-a> <file-b>")))
      (do
        (println "durable-iot-migrate — IoT 자동화 레시피 변환 CLI")
        (println)
        (println "Usage:")
        (println "  parse <platform> <file>           레시피 파싱")
        (println "  json  <platform> <file>           JSON 출력")
        (println "  equiv <platform> <file-a> <file-b> 동치 비교")
        (println)
        (println "Platforms: ha (Home Assistant)")))))
