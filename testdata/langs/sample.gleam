import gleam/io
import gleam/list.{map}

pub type Shape {
  Circle(radius: Float)
  Square(side: Float)
}

pub fn area(shape: Shape) -> Float {
  1.0
}

fn private_helper(x: Int) -> Int {
  x
}

pub const pi = 3.14
