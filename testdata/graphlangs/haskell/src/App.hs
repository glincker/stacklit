module App where

import Lib.Base (helper)

run :: Int -> Int
run x = helper x + local x

local :: Int -> Int
local y = y
