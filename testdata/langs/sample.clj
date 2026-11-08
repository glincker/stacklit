(ns my.app.core
  (:require [clojure.string :as str]
            [my.app.util :refer [helper]]
            [clojure.set])
  (:import (java.util Date)))

(def version "1.0")

(defn greet
  "Says hello"
  [name greeting]
  (str greeting " " name))

(defn- private-fn [x] x)

(defmacro unless [c & body] `(if ~c nil (do ~@body)))

(defprotocol Shape
  (area [this]))

(defrecord Circle [r])
