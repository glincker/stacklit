local json = require("cjson")
local utils = require "app.utils"
local M = {}

function M.add(a, b)
  return a + b
end

function M:greet(name)
  return "hi " .. name
end

local function helper(x)
  return x
end

function global_fn(a)
  return a
end

M.sub = function(a, b) return a - b end

return M
