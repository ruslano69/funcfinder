// Scala test file for funcfinder

package com.example.test

import scala.math.sqrt

// Simple function
def greet(name: String): String = {
  s"Hello, $name!"
}

// One-line function
def add(a: Int, b: Int): Int = a + b

// Generic function
def swap[T](list: Array[T], i: Int, j: Int): Unit = {
  val temp = list(i)
  list(i) = list(j)
  list(j) = temp
}

// Higher-order function
def withLogging[T](block: => T): T = {
  println("[START]")
  val result = block
  println("[END]")
  result
}

// Curried function
def multiply(a: Int)(b: Int): Int = a * b

// Class
class Person(val name: String, var age: Int) {
  def describe(): String = s"$name, age $age"

  def birthday(): Unit = {
    age += 1
  }

  override def toString: String = describe()
}

// Companion object
object Person {
  def create(name: String): Person = new Person(name, 0)

  def apply(name: String, age: Int): Person = new Person(name, age)
}

// Case class
case class Point(x: Double, y: Double) {
  def distance(other: Point): Double = {
    val dx = x - other.x
    val dy = y - other.y
    sqrt(dx * dx + dy * dy)
  }

  def +(other: Point): Point = Point(x + other.x, y + other.y)
}

// Trait
trait Drawable {
  def draw(): Unit
  def resize(scale: Double): Unit
}

trait Area {
  def area(): Double
}

// Abstract class
abstract class Shape {
  def area(): Double
  def perimeter(): Double

  def describe(): String = s"Area: ${area()}, Perimeter: ${perimeter()}"
}

// Class extending abstract class and trait
class Circle(val radius: Double) extends Shape with Drawable {
  override def area(): Double = Math.PI * radius * radius

  override def perimeter(): Double = 2 * Math.PI * radius

  override def draw(): Unit = println(s"Drawing circle with radius $radius")

  override def resize(scale: Double): Unit = {
    // Can't resize immutable val, would need var
  }
}

// Sealed trait
sealed trait Result[+T]
case class Success[T](value: T) extends Result[T]
case class Failure(message: String) extends Result[Nothing]

// Object (singleton)
object Logger {
  def log(message: String): Unit = println(s"[LOG] $message")

  def error(message: String): Unit = println(s"[ERROR] $message")

  private def internal(): Unit = {
    // private method
  }
}

// Enum (Scala 3 style, but also works as sealed trait pattern)
enum Direction {
  case North, South, East, West

  def opposite: Direction = this match {
    case North => South
    case South => North
    case East => West
    case West => East
  }
}

// Generic class
class Container[T](private var value: T) {
  def get: T = value

  def set(newValue: T): Unit = {
    value = newValue
  }

  def map[U](f: T => U): Container[U] = new Container(f(value))
}

// Implicit class (extension methods)
implicit class StringOps(val s: String) extends AnyVal {
  def exclaim: String = s + "!"

  def repeat(n: Int): String = s * n
}

// Lambda (should not be detected as function)
val multiply: (Int, Int) => Int = (a, b) => a * b
val square: Int => Int = x => x * x

// Main object
object Main {
  def main(args: Array[String]): Unit = {
    val p = Point(3, 4)
    println(p.distance(Point(0, 0)))
  }
}
