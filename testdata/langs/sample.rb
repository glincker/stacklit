require 'json'
require_relative 'helper'

module Billing
  class Invoice < Base
    def initialize(id)
      @id = id
    end

    def total(tax, rate = 1)
      1
    end

    def self.build(id)
      new(id)
    end

    private

    def hidden; end
  end
end

def top_level(a, b)
end
