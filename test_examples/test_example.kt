// Kotlin test file for funcfinder

package com.example.test

import kotlin.math.sqrt

// Top-level function
fun greet(name: String): String {
    return "Hello, $name!"
}

// Extension function
fun String.addExclamation(): String {
    return this + "!"
}

// Suspend function
suspend fun fetchData(url: String): String {
    // Simulated async operation
    return "data from $url"
}

// Generic function
fun <T> swap(list: MutableList<T>, i: Int, j: Int) {
    val temp = list[i]
    list[i] = list[j]
    list[j] = temp
}

// Inline function
inline fun measureTime(block: () -> Unit): Long {
    val start = System.currentTimeMillis()
    block()
    return System.currentTimeMillis() - start
}

// Class
class Person(val name: String, var age: Int) {
    fun describe(): String {
        return "$name, age $age"
    }

    companion object {
        fun create(name: String): Person {
            return Person(name, 0)
        }
    }
}

// Data class
data class Point(val x: Double, val y: Double) {
    fun distance(other: Point): Double {
        val dx = x - other.x
        val dy = y - other.y
        return sqrt(dx * dx + dy * dy)
    }
}

// Sealed class
sealed class Result<out T> {
    data class Success<T>(val data: T) : Result<T>()
    data class Error(val message: String) : Result<Nothing>()

    fun isSuccess(): Boolean = this is Success
}

// Object (singleton)
object Logger {
    fun log(message: String) {
        println("[LOG] $message")
    }

    fun error(message: String) {
        println("[ERROR] $message")
    }
}

// Interface
interface Drawable {
    fun draw()
    fun resize(scale: Double)
}

// Enum class
enum class Direction {
    NORTH, SOUTH, EAST, WEST;

    fun opposite(): Direction {
        return when (this) {
            NORTH -> SOUTH
            SOUTH -> NORTH
            EAST -> WEST
            WEST -> EAST
        }
    }
}

// Abstract class
abstract class Shape {
    abstract fun area(): Double
    abstract fun perimeter(): Double

    fun describe(): String {
        return "Area: ${area()}, Perimeter: ${perimeter()}"
    }
}

// Class implementing interface
class Circle(private val radius: Double) : Shape(), Drawable {
    override fun area(): Double = Math.PI * radius * radius
    override fun perimeter(): Double = 2 * Math.PI * radius
    override fun draw() { println("Drawing circle") }
    override fun resize(scale: Double) { /* resize */ }
}

// Lambda (should not be detected as function)
val multiply: (Int, Int) -> Int = { a, b -> a * b }
