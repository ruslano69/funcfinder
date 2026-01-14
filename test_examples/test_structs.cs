// C# test file for findstruct prototype
// Tests: classes, structs, interfaces, enums, records, fields

using System;
using System.Collections.Generic;

// Simple struct
public struct Point {
    public int X;
    public int Y;
    public double Z;

    public Point(int x, int y) {
        X = x;
        Y = y;
        Z = 0.0;
    }

    public double DistanceTo(Point other) {
        return Math.Sqrt(Math.Pow(other.X - X, 2) + Math.Pow(other.Y - Y, 2));
    }
}

// Class with various field types
public class User {
    private int _id;
    private string _name;
    private string _email;
    private bool _active;
    private static int _userCount = 0;

    public List<string> Roles;
    public readonly string CreatedAt;
    protected DateTime LastLogin;

    public User(string name, string email) {
        _name = name;
        _email = email;
        _active = true;
        CreatedAt = DateTime.UtcNow.ToString("yyyy-MM-dd");
        _id = ++_userCount;
        Roles = new List<string>();
    }

    public void Activate() {
        _active = true;
    }

    public bool IsActive => _active;
    public int Id => _id;
    public string Name => _name;
    public string Email => _email;
}

// Interface
public interface IDrawable {
    void Draw();
    void SetColor(string color);
}

public interface IResizable {
    void Resize(int width, int height);
}

// Enum
public enum UserRole {
    Admin = 0,
    Moderator = 1,
    User = 2,
    Guest = 3
}

// Record (C# 9+)
public record UserRecord(
    int Id,
    string Name,
    string Email
) {
    public string DisplayName => $"{Name} ({Email})";
}

// Class implementing multiple interfaces
public class Circle : IDrawable, IResizable {
    private double _radius;
    private string _color;
    private const double Pi = Math.PI;

    public Circle(double radius) {
        _radius = radius;
        _color = "black";
    }

    public void Draw() {
        Console.WriteLine($"Drawing circle with radius {_radius}");
    }

    public void SetColor(string color) {
        _color = color;
    }

    public void Resize(int width, int height) {
        _radius = width / 2.0;
    }

    public double GetArea() => Pi * _radius * _radius;
}

// Abstract class
public abstract class Shape {
    protected string Color;

    public abstract double GetArea();
    public abstract double GetPerimeter();

    public void SetColor(string color) {
        Color = color;
    }

    public string GetColor() => Color;
}

public class Rectangle : Shape {
    private double _width;
    private double _height;

    public Rectangle(double width, double height) {
        _width = width;
        _height = height;
        Color = "black";
    }

    public override double GetArea() => _width * _height;
    public override double GetPerimeter() => 2 * (_width + _height);
}

// Struct with methods
public struct RectangleStruct {
    public double Width;
    public double Height;

    public RectangleStruct(double width, double height) {
        Width = width;
        Height = height;
    }

    public double Area => Width * Height;
    public double Perimeter => 2 * (Width + Height);
}

// Nested class
public class Container {
    private string _name;
    private List<int> _items;

    public class Iterator {
        private int _index = 0;
        private readonly List<int> _items;

        public Iterator(List<int> items) {
            _items = items;
        }

        public bool HasNext => _index < _items.Count;
        public int Current => _items[_index];

        public int Next() {
            if (HasNext) {
                return _items[_index++];
            }
            throw new InvalidOperationException();
        }
    }

    public Container(string name) {
        _name = name;
        _items = new List<int>();
    }

    public void AddItem(int item) {
        _items.Add(item);
    }

    public Iterator GetIterator() {
        return new Iterator(_items);
    }
}

// Generic class
public class Box<T> {
    private T _content;
    private bool _empty;

    public Box() {
        _empty = true;
    }

    public void Set(T item) {
        _content = item;
        _empty = false;
    }

    public T Get() => _content;
    public bool IsEmpty() => _empty;
}

// Static class
public static class MathUtils {
    public const double Pi = 3.14159;

    public static double CalculateArea(double radius) {
        return Pi * radius * radius;
    }

    public static int Add(int a, int b) => a + b;
}

// Delegate
public delegate void ActionHandler(string message);

// Event class
public class EventPublisher {
    public event ActionHandler OnAction;

    public void Trigger(string message) {
        OnAction?.Invoke(message);
    }
}

// Exception class
public class CustomException : Exception {
    private readonly string _errorCode;

    public CustomException(string message, string errorCode) : base(message) {
        _errorCode = errorCode;
    }

    public string ErrorCode => _errorCode;
}

// Attribute
[AttributeUsage(AttributeTargets.Class | AttributeTargets.Method)]
public class AuthorAttribute : Attribute {
    public string Name { get; set; }
    public string Date { get; set; }
}

// Class with inheritance
public class Animal {
    public string Name;
    public int Age;

    public virtual void Speak() {
        Console.WriteLine("Animal sound");
    }
}

public class Dog : Animal {
    public string Breed;

    public override void Speak() {
        Console.WriteLine("Woof!");
    }

    public void Bark() {
        Console.WriteLine("Woof woof!");
    }
}

// Partial class
public partial class PartialClass {
    public int Part1Field;
}

public partial class PartialClass {
    public int Part2Field;
}
