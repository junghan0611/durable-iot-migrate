(ns build
  (:require [clojure.tools.build.api :as b]))

(def basis (delay (b/create-basis {:project "deps.edn"})))
(def class-dir "target/classes")
(def uber-file "target/durable-iot-migrate.jar")

(defn clean [_]
  (b/delete {:path "target"}))

(defn uber [_]
  (clean nil)
  (b/copy-dir {:src-dirs ["src"] :target-dir class-dir})
  (b/compile-clj {:basis @basis :src-dirs ["src"] :class-dir class-dir})
  (b/uber {:class-dir class-dir :uber-file uber-file :basis @basis
           :main 'iot.semantic.cli}))
