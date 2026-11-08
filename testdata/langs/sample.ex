defmodule MyApp.Accounts do
  @moduledoc "Accounts context"
  use GenServer
  import Ecto.Query
  alias MyApp.Repo
  alias MyApp.Accounts.{User, Role}
  require Logger

  defstruct [:id, :name]

  @type t :: %__MODULE__{id: integer(), name: String.t()}

  @callback fetch(id :: integer()) :: {:ok, t()} | :error

  def get_user(id), do: Repo.get(User, id)

  def create_user(attrs, opts \\ []) do
    Repo.insert(attrs)
  end

  defp secret(x), do: x

  defmacro log_it(msg) do
    quote do: Logger.info(unquote(msg))
  end

  defmodule Inner do
    def hello, do: :world
  end
end

defprotocol MyApp.Printable do
  def print(data)
end
