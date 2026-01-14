// C++ test file for findstruct prototype
// Tests: classes, structs, enums, fields

#include <iostream>
#include <string>
#include <vector>

// Simple struct
struct Point {
    int x;
    int y;
    double z;
};

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
};

// Class with various field types
class User {
private:
    int id;
    std::string name;
    std::string email;
    bool active;
    static int userCount;

public:
    std::vector<std::string> roles;
    const std::string createdAt;

    User(std::string n, std::string e) : name(n), email(e), active(true), createdAt("2024-01-01") {
        id = ++userCount;
    }

    void activate() {
        active = true;
    }

    bool isActive() const {
        return active;
    }
};

// Enum
enum class UserRole {
    Admin = 0,
    Moderator = 1,
    User = 2,
    Guest = 3
};

// Nested class
class Container {
private:
    std::string name;
    std::vector<int> items;

public:
    class Iterator {
    private:
        std::vector<int>::iterator it;

    public:
        Iterator(std::vector<int>::iterator i) : it(i) {}

        int operator*() {
            return *it;
        }

        Iterator& operator++() {
            ++it;
            return *this;
        }

        bool operator!=(const Iterator& other) const {
            return it != other.it;
        }
    };

    Container(std::string n) : name(n) {}

    void addItem(int item) {
        items.push_back(item);
    }

    Iterator begin() {
        return Iterator(items.begin());
    }

    Iterator end() {
        return Iterator(items.end());
    }
};

// Template struct
template<typename T>
struct Box {
    T value;
    bool empty;

    Box() : value(T()), empty(true) {}

    void set(T v) {
        value = v;
        empty = false;
    }

    T get() {
        return value;
    }
};

// Union
union Data {
    int intValue;
    float floatValue;
    char charValue;
};

// Struct with inheritance
struct Animal {
    std::string name;
    int age;

    void speak() {
        std::cout << "Animal sound" << std::endl;
    }
};

struct Dog : Animal {
    std::string breed;

    void bark() {
        std::cout << "Woof!" << std::endl;
    }
};
