require_relative "base"

class App < Base
  def run(b)
    helper(1)
    greet
    Base.new.greet
    b.greet
  end
end
