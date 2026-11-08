local base = require("base")

local App = {}

function App.run(x)
  base.helper(x)
  base.greet()
end

return App
