// Java test file for findstruct prototype
// Tests: classes, interfaces, enums, fields

import java.util.*;
import java.time.LocalDate;

// Simple class
class Point {
    int x;
    int y;
    double z;

    Point(int x, int y) {
        this.x = x;
        this.y = y;
        this.z = 0.0;
    }

    double distanceTo(Point other) {
        return Math.sqrt(Math.pow(other.x - x, 2) + Math.pow(other.y - y, 2));
    }
}

// Class with various field types
class User {
    private int id;
    private String name;
    private String email;
    private boolean active;
    private static int userCount = 0;

    public List<String> roles;
    public final String createdAt;
    protected LocalDate lastLogin;

    User(String name, String email) {
        this.name = name;
        this.email = email;
        this.active = true;
        this.createdAt = LocalDate.now().toString();
        this.id = ++userCount;
        this.roles = new ArrayList<>();
    }

    public void activate() {
        active = true;
    }

    public boolean isActive() {
        return active;
    }

    public int getId() {
        return id;
    }
}

// Interface
interface Drawable {
    void draw();
    void setColor(String color);
}

interface Resizable {
    void resize(int width, int height);
}

// Enum
enum UserRole {
    ADMIN(0),
    MODERATOR(1),
    USER(2),
    GUEST(3);

    private final int level;

    UserRole(int level) {
        this.level = level;
    }

    public int getLevel() {
        return level;
    }
}

// Class implementing multiple interfaces
class Circle implements Drawable, Resizable {
    private double radius;
    private String color;
    private final double pi = Math.PI;

    Circle(double radius) {
        this.radius = radius;
        this.color = "black";
    }

    @Override
    public void draw() {
        System.out.println("Drawing circle with radius " + radius);
    }

    @Override
    public void setColor(String color) {
        this.color = color;
    }

    @Override
    public void resize(int width, int height) {
        this.radius = width / 2.0;
    }

    public double getArea() {
        return pi * radius * radius;
    }
}

// Abstract class
abstract class Shape {
    protected String color;

    abstract double getArea();
    abstract double getPerimeter();

    public void setColor(String color) {
        this.color = color;
    }

    public String getColor() {
        return color;
    }
}

class Rectangle extends Shape {
    private double width;
    private double height;

    Rectangle(double width, double height) {
        this.width = width;
        this.height = height;
        this.color = "black";
    }

    @Override
    double getArea() {
        return width * height;
    }

    @Override
    double getPerimeter() {
        return 2 * (width + height);
    }
}

// Nested class
class Container {
    private String name;
    private List<Integer> items;

    public class Iterator {
        private int index = 0;

        public boolean hasNext() {
            return index < items.size();
        }

        public Integer next() {
            if (hasNext()) {
                return items.get(index++);
            }
            return null;
        }
    }

    Container(String name) {
        this.name = name;
        this.items = new ArrayList<>();
    }

    public void addItem(Integer item) {
        items.add(item);
    }

    public Iterator getIterator() {
        return new Iterator();
    }
}

// Record (Java 16+)
record UserRecord(int id, String name, String email) {
    public UserRecord {
        if (id < 0) {
            throw new IllegalArgumentException("ID must be positive");
        }
    }

    public String getDisplayName() {
        return name + " (" + email + ")";
    }
}

// Generic class
class Box<T> {
    private T content;
    private boolean empty;

    public Box() {
        this.empty = true;
    }

    public void set(T item) {
        this.content = item;
        this.empty = false;
    }

    public T get() {
        return content;
    }

    public boolean isEmpty() {
        return empty;
    }
}

// Exception class
class CustomException extends Exception {
    private String errorCode;

    public CustomException(String message, String errorCode) {
        super(message);
        this.errorCode = errorCode;
    }

    public String getErrorCode() {
        return errorCode;
    }
}

// Annotation
@interface Author {
    String name();
    String date();
}
