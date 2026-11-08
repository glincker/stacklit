open Base

let local x = x

let run x = helper (local x)
