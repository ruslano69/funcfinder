#include <iostream>
#include <string>
#include <vector>
#include <memory>

using namespace std;

// Simple function
int add(int a, int b)
{
    return a + b;
}

// Function with default parameters
void printMessage(string msg, int times = 1)
{
    for (int i = 0; i < times; i++) {
        cout << msg << endl;
    }
}

// Template function
template<typename T>
T maximum(T a, T b)
{
    return (a > b) ? a : b;
}

// Function with multiple template parameters
template<typename K, typename V>
V getValueOrDefault(const map<K, V>& m, const K& key, const V& defaultValue)
{
    auto it = m.find(key);
    return (it != m.end()) ? it->second : defaultValue;
}

// Class with methods
class Calculator {
private:
    int value;

public:
    Calculator() : value(0) {}

    void add(int n)
    {
        value += n;
    }

    int getValue() const
    {
        return value;
    }

    // Static method
    static int multiply(int a, int b)
    {
        return a * b;
    }

    // Virtual method
    virtual void display()
    {
        cout << "Value: " << value << endl;
    }
};

// Lambda function example
auto createMultiplier(int factor)
{
    return [factor](int x) { return x * factor; };
}

// Const member function
void processData(const vector<int>& data) const
{
    for (auto val : data) {
        cout << val << " ";
    }
    cout << endl;
}

// Main function
int main()
{
    cout << "Sum: " << add(10, 20) << endl;
    Calculator calc;
    calc.add(5);
    cout << "Result: " << calc.getValue() << endl;
    return 0;
}
