open Core
module L = List
include Printf

type color = Red | Green | Blue

type point = { x : int; y : int }

exception Not_found_here of string

module type SHAPE = sig
  val area : float -> float
end

module Circle = struct
  let area r = r *. r
end

let add a b = a + b

let rec fact n = if n <= 1 then 1 else n * fact (n - 1)

let pi = 3.14
