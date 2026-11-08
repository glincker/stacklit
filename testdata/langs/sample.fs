module Geometry.Shapes

open System
open System.Collections.Generic

type Color =
    | Red
    | Green

type Point = { X: float; Y: float }

type Circle(r: float) =
    member this.Area() = 3.14 * r * r

let add (a: int) (b: int) : int = a + b

let private hidden x = x

let rec fact n = if n <= 1 then 1 else n * fact (n - 1)
