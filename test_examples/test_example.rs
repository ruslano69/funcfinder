// Rust test file for funcfinder

use std::fmt;

// Simple function
fn greet(name: &str) -> String {
    format!("Hello, {}!", name)
}

// Public function
pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

// Generic function
fn swap<T>(a: &mut T, b: &mut T) {
    std::mem::swap(a, b);
}

// Async function
async fn fetch_data(url: &str) -> Result<String, Box<dyn std::error::Error>> {
    Ok(format!("data from {}", url))
}

// Const function
const fn square(x: i32) -> i32 {
    x * x
}

// Struct
struct Point {
    x: f64,
    y: f64,
}

// Impl block for struct
impl Point {
    fn new(x: f64, y: f64) -> Self {
        Point { x, y }
    }

    fn distance(&self, other: &Point) -> f64 {
        let dx = self.x - other.x;
        let dy = self.y - other.y;
        (dx * dx + dy * dy).sqrt()
    }

    fn move_by(&mut self, dx: f64, dy: f64) {
        self.x += dx;
        self.y += dy;
    }
}

// Tuple struct
struct Color(u8, u8, u8);

impl Color {
    fn rgb(r: u8, g: u8, b: u8) -> Self {
        Color(r, g, b)
    }

    fn to_hex(&self) -> String {
        format!("#{:02x}{:02x}{:02x}", self.0, self.1, self.2)
    }
}

// Enum
enum Direction {
    North,
    South,
    East,
    West,
}

impl Direction {
    fn opposite(&self) -> Direction {
        match self {
            Direction::North => Direction::South,
            Direction::South => Direction::North,
            Direction::East => Direction::West,
            Direction::West => Direction::East,
        }
    }
}

// Enum with data
enum Result<T, E> {
    Ok(T),
    Err(E),
}

// Trait
trait Drawable {
    fn draw(&self);
    fn resize(&mut self, scale: f64);
}

trait Area {
    fn area(&self) -> f64;
}

// Struct implementing trait
struct Circle {
    radius: f64,
}

impl Circle {
    fn new(radius: f64) -> Self {
        Circle { radius }
    }
}

impl Area for Circle {
    fn area(&self) -> f64 {
        std::f64::consts::PI * self.radius * self.radius
    }
}

impl Drawable for Circle {
    fn draw(&self) {
        println!("Drawing circle with radius {}", self.radius);
    }

    fn resize(&mut self, scale: f64) {
        self.radius *= scale;
    }
}

// Generic struct
struct Container<T> {
    value: T,
}

impl<T> Container<T> {
    fn new(value: T) -> Self {
        Container { value }
    }

    fn get(&self) -> &T {
        &self.value
    }

    fn set(&mut self, value: T) {
        self.value = value;
    }
}

// Union (unsafe)
union IntOrFloat {
    i: i32,
    f: f32,
}

// Implementing Display trait
impl fmt::Display for Point {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "({}, {})", self.x, self.y)
    }
}

// Module
mod utils {
    pub fn helper() -> i32 {
        42
    }

    fn private_helper() -> i32 {
        0
    }
}

// Closure (should not be detected as function)
fn main() {
    let multiply = |a, b| a * b;
    let result = multiply(3, 4);
    println!("{}", result);
}
