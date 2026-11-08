module Geometry

using LinearAlgebra
import Base: show
using Statistics: mean, std

export area, Point

struct Point
    x::Float64
    y::Float64
end

abstract type Shape end

mutable struct Circle <: Shape
    r::Float64
end

function area(c::Circle)::Float64
    return pi * c.r^2
end

square(x) = x * x

const TAU = 6.28

end
