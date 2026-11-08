defmodule App.Main do
  alias App.Base

  def run(x) do
    Base.helper(x)
    local(x)
  end

  defp local(x), do: x
end
