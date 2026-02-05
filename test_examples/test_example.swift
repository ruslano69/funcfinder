// Swift test file for funcfinder

import Foundation

// Simple function
func greet(name: String) -> String {
    return "Hello, \(name)!"
}

// Static function
public static func calculate(a: Int, b: Int) -> Int {
    return a + b
}

// Async function
func fetchData() async throws -> Data {
    let url = URL(string: "https://example.com")!
    let (data, _) = try await URLSession.shared.data(from: url)
    return data
}

// Class with methods
public class Person {
    var name: String
    var age: Int

    init(name: String, age: Int) {
        self.name = name
        self.age = age
    }

    func describe() -> String {
        return "\(name), age \(age)"
    }

    static func create(name: String) -> Person {
        return Person(name: name, age: 0)
    }
}

// Struct
struct Point {
    var x: Double
    var y: Double

    func distance(to other: Point) -> Double {
        let dx = x - other.x
        let dy = y - other.y
        return sqrt(dx*dx + dy*dy)
    }

    mutating func move(dx: Double, dy: Double) {
        x += dx
        y += dy
    }
}

// Enum with methods
enum Direction {
    case north, south, east, west

    func opposite() -> Direction {
        switch self {
        case .north: return .south
        case .south: return .north
        case .east: return .west
        case .west: return .east
        }
    }
}

// Protocol
protocol Drawable {
    func draw()
    func resize(scale: Double)
}

// Extension
extension String {
    func reversed() -> String {
        return String(self.reversed())
    }
}

// Generic function
func swap<T>(_ a: inout T, _ b: inout T) {
    let temp = a
    a = b
    b = temp
}

// Closure as property (should not be detected as function)
let multiply: (Int, Int) -> Int = { a, b in
    return a * b
}

// Guard let else (test for false positive fix)
func safeDivide(_ a: Int, _ b: Int) -> Int? {
    guard b != 0 else {
        return nil
    }
    guard let result = Optional(a / b) else {
        return nil
    }
    return result
}
