// D test file for findstruct prototype
// Tests: structs, classes, interfaces, enums, unions, fields

import std.stdio;
import std.string;
import std.array;

// Simple struct
struct Point {
    int x;
    int y;
    double z;

    this(int x, int y) {
        this.x = x;
        this.y = y;
        this.z = 0.0;
    }

    double distanceTo(Point other) {
        return sqrt(pow(other.x - x, 2) + pow(other.y - y, 2));
    }
}

// Struct with methods
struct Rectangle {
    int width;
    int height;
    Point position;

    int getArea() {
        return width * height;
    }

    bool contains(Point p) {
        return p.x >= position.x && p.x <= position.x + width &&
               p.y >= position.y && p.y <= position.y + height;
    }
}

// Class with various field types
class User {
    private int _id;
    private string _name;
    private string _email;
    private bool _active;
    private static int _userCount = 0;

    public string[] roles;
    public immutable string createdAt;

    this(string name, string email) {
        _name = name;
        _email = email;
        _active = true;
        createdAt = "2024-01-01";
        _id = ++_userCount;
        roles = [];
    }

    void activate() {
        _active = true;
    }

    bool isActive() {
        return _active;
    }

    int getId() {
        return _id;
    }
}

// Interface
interface Drawable {
    void draw();
    void setColor(string color);
}

interface Resizable {
    void resize(int width, int height);
}

// Enum
enum UserRole {
    Admin = 0,
    Moderator = 1,
    User = 2,
    Guest = 3
}

// Class implementing multiple interfaces
class Circle : Drawable, Resizable {
    private double _radius;
    private string _color;
    private immutable double _pi = 3.14159;

    this(double radius) {
        _radius = radius;
        _color = "black";
    }

    override void draw() {
        writeln("Drawing circle with radius ", _radius);
    }

    override void setColor(string color) {
        _color = color;
    }

    override void resize(int width, int height) {
        _radius = width / 2.0;
    }

    double getArea() {
        return _pi * _radius * _radius;
    }
}

// Abstract class
abstract class Shape {
    protected string _color;

    abstract double getArea();
    abstract double getPerimeter();

    void setColor(string color) {
        _color = color;
    }

    string getColor() {
        return _color;
    }
}

class RectangleClass : Shape {
    private double _width;
    private double _height;

    this(double width, double height) {
        _width = width;
        _height = height;
        _color = "black";
    }

    override double getArea() {
        return _width * _height;
    }

    override double getPerimeter() {
        return 2 * (_width + _height);
    }
}

// Nested class
class Container {
    private string _name;
    private int[] _items;

    class Iterator {
        private size_t _index = 0;
        private int[] _items;

        this(int[] items) {
            _items = items;
        }

        bool hasNext() {
            return _index < _items.length;
        }

        int next() {
            if (hasNext()) {
                return _items[_index++];
            }
            throw new Exception("No more elements");
        }
    }

    this(string name) {
        _name = name;
        _items = [];
    }

    void addItem(int item) {
        _items ~= item;
    }

    Iterator getIterator() {
        return new Iterator(_items);
    }
}

// Template struct
struct Box(T) {
    T content;
    bool empty;

    this() {
        content = T.init;
        empty = true;
    }

    void set(T item) {
        content = item;
        empty = false;
    }

    T get() {
        return content;
    }
}

// Union
union Data {
    int intValue;
    float floatValue;
    char charValue;
}

// Struct with inheritance (D uses composition or alias this)
struct Animal {
    string name;
    int age;

    void speak() {
        writeln("Animal sound");
    }
}

struct Dog {
    Animal animal;
    string breed;

    alias animal this;

    void bark() {
        writeln("Woof!");
    }
}

// Final class
final class Singleton {
    private static Singleton _instance;
    private int _value;

    static Singleton getInstance() {
        if (_instance is null) {
            _instance = new Singleton();
        }
        return _instance;
    }

    private this() {
        _value = 42;
    }

    int getValue() {
        return _value;
    }
}

// Interface with default implementation (D 2.076+)
interface Greeter {
    void greet() {
        writeln("Hello!");
    }
}

class Person : Greeter {
    string name;

    this(string name) {
        this.name = name;
    }

    override void greet() {
        writeln("Hello, ", name, "!");
    }
}

// Exception class
class CustomException : Exception {
    private string _errorCode;

    this(string message, string errorCode) {
        super(message);
        _errorCode = errorCode;
    }

    string getErrorCode() {
        return _errorCode;
    }
}

// Struct with immutable and shared
struct Config {
    immutable string name;
    shared int counter;
    bool enabled;

    this(string name) {
        this.name = name;
        this.counter = 0;
        this.enabled = true;
    }
}
