import std.stdio;
import std.string;
import std.algorithm;

// Simple function
int add(int a, int b)
{
    return a + b;
}

// Function with ref parameters
void swap(ref int a, ref int b)
{
    int temp = a;
    a = b;
    b = temp;
}

// Function returning array
int[] createRange(int start, int end)
{
    int[] result;
    for (int i = start; i <= end; i++) {
        result ~= i;
    }
    return result;
}

// Template function
T maximum(T)(T a, T b)
{
    return a > b ? a : b;
}

// Function with out parameter
bool tryParse(string s, out int result)
{
    try {
        result = to!int(s);
        return true;
    }
    catch (Exception e) {
        return false;
    }
}

// Variadic function
void printAll(T...)(T args)
{
    foreach (arg; args) {
        write(arg, " ");
    }
    writeln();
}

// Struct with methods
struct Point
{
    int x, y;

    int distanceSquared()
    {
        return x * x + y * y;
    }

    void move(int dx, int dy)
    {
        x += dx;
        y += dy;
    }
}

// Main function
void main()
{
    writeln("Sum: ", add(10, 20));

    int x = 5, y = 10;
    swap(x, y);
    writefln("After swap: x=%d, y=%d", x, y);

    Point p = Point(3, 4);
    writeln("Distance squared: ", p.distanceSquared());
}
