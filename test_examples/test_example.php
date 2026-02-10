<?php
// PHP test file for funcfinder

namespace App\Example;

// Simple function
function greet(string $name): string {
    return "Hello, $name!";
}

// Function with default parameter
function add(int $a, int $b = 0): int {
    return $a + $b;
}

// Arrow function (PHP 7.4+)
$multiply = fn($a, $b) => $a * $b;

// Class
class Person {
    private string $name;
    private int $age;

    public function __construct(string $name, int $age) {
        $this->name = $name;
        $this->age = $age;
    }

    public function getName(): string {
        return $this->name;
    }

    public function setName(string $name): void {
        $this->name = $name;
    }

    public function describe(): string {
        return "{$this->name}, age {$this->age}";
    }

    public static function create(string $name): Person {
        return new Person($name, 0);
    }

    private function internalMethod(): void {
        // private method
    }
}

// Interface
interface Drawable {
    public function draw(): void;
    public function resize(float $scale): void;
}

// Trait
trait Loggable {
    public function log(string $message): void {
        echo "[LOG] $message\n";
    }

    protected function error(string $message): void {
        echo "[ERROR] $message\n";
    }
}

// Abstract class
abstract class Shape {
    abstract public function area(): float;
    abstract public function perimeter(): float;

    public function describe(): string {
        return "Area: " . $this->area() . ", Perimeter: " . $this->perimeter();
    }
}

// Class using trait and implementing interface
class Circle extends Shape implements Drawable {
    use Loggable;

    private float $radius;

    public function __construct(float $radius) {
        $this->radius = $radius;
    }

    public function area(): float {
        return M_PI * $this->radius * $this->radius;
    }

    public function perimeter(): float {
        return 2 * M_PI * $this->radius;
    }

    public function draw(): void {
        $this->log("Drawing circle with radius {$this->radius}");
    }

    public function resize(float $scale): void {
        $this->radius *= $scale;
    }
}

// Enum (PHP 8.1+)
enum Direction {
    case North;
    case South;
    case East;
    case West;

    public function opposite(): Direction {
        return match($this) {
            self::North => self::South,
            self::South => self::North,
            self::East => self::West,
            self::West => self::East,
        };
    }
}

// Final class
final class Config {
    private static ?Config $instance = null;

    private function __construct() {}

    public static function getInstance(): Config {
        if (self::$instance === null) {
            self::$instance = new Config();
        }
        return self::$instance;
    }

    public function get(string $key): mixed {
        return null;
    }
}

// Anonymous class usage (the anonymous class itself shouldn't be named)
function createLogger(): object {
    return new class {
        public function log(string $msg): void {
            echo $msg;
        }
    };
}
