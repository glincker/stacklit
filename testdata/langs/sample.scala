package com.example.app

import scala.collection.mutable
import java.util.{List, Map}
import cats.effect._

trait Greeter {
  def greet(name: String): String
}

class Person(val name: String, age: Int) extends Greeter {
  def greet(name: String): String = s"Hi $name"
  private def secret(): Int = 1
}

case class Point(x: Int, y: Int)

object Main {
  def main(args: Array[String]): Unit = println("hi")
  val answer: Int = 42
}

enum Color { case Red, Green }

type Alias = Map[String, Int]
