// test_structs_c.c - Test file for C struct detection
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Simple struct
struct Point {
    int x;
    int y;
};

// Nested struct
struct Rectangle {
    struct Point origin;
    double width;
    double height;
};

// Struct with bit fields
struct Flags {
    unsigned int is_active : 1;
    unsigned int has_error : 1;
    unsigned int reserved : 6;
};

// Union inside struct
struct Data {
    int type;
    union {
        int int_value;
        double double_value;
        char* string_value;
    } value;
};

// Typedef struct
typedef struct {
    char* name;
    int age;
} Person;

// Nested typedef struct
typedef struct {
    Person person;
    char* department;
    double salary;
} Employee;

// Struct with array
struct ArrayHolder {
    int numbers[10];
    char buffer[256];
    float values[5][5];
};

// Struct with pointer fields
struct Node {
    int value;
    struct Node* next;
    struct Node* prev;
};

// Empty struct (C11)
struct Empty {
};

// Struct with function pointer
struct Callback {
    void (*func)(int);
    int data;
};

// Struct with enum
enum Color { RED, GREEN, BLUE };

struct ColorPair {
    enum Color primary;
    enum Color secondary;
};

// Nested struct definition
struct Address {
    char* street;
    char* city;
    char* zip;
};

struct Contact {
    char* name;
    struct Address address;
    char* phone;
};

// Multi-level nesting
struct Company {
    char* name;
    struct Contact main_office;
    struct Contact* branch_offices;
    int office_count;
};

// Struct with volatile
struct VolatileData {
    volatile int counter;
    volatile char* buffer;
};

// Struct with const
struct Immutable {
    const int id;
    const char* name;
};

// Struct with packed attribute (C11)
struct PackedStruct {
    char a;
    int b;
    char c;
} __attribute__((packed));

// Struct with aligned attribute
struct AlignedStruct {
    char a;
    double b;
    char c;
} __attribute__((aligned(16)));

// Union definition
union DataType {
    int i;
    float f;
    char* s;
};

// Enum definition
enum Status {
    STATUS_OK = 0,
    STATUS_ERROR,
    STATUS_PENDING,
    STATUS_INVALID
};

// Anonymous struct (C11)
struct {
    int x;
    int y;
} anonymous_point;

// Struct with flexible array member (C99)
struct FlexibleArray {
    int size;
    int data[];
};

// Struct with _Static_assert (C11)
struct WithStaticAssert {
    int field;
    _Static_assert(sizeof(int) == 4, "int must be 4 bytes");
};

// Complex nested struct
struct Complex {
    struct Point point;
    struct Rectangle bounds;
    union DataType data;
    enum Status status;
    Person person;
    struct Node* nodes;
    int node_count;
};

// Inline struct definition (in function)
void process() {
    struct {
        int value;
        char* label;
    } local_struct = {42, "answer"};
    
    printf("%s: %d\n", local_struct.label, local_struct.value);
}

// Struct pointer usage
void use_structs() {
    struct Point p = {10, 20};
    struct Point* pp = &p;
    
    struct Rectangle r = {{0, 0}, 100, 50};
    struct Rectangle* pr = &r;
    
    Employee* emp = malloc(sizeof(Employee));
    emp->person.name = "John";
    emp->person.age = 30;
    
    free(emp);
}
