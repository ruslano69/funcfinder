

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Simple function
int add(int a, int b)
{
    return a + b;
}

// Function with pointer parameters
void swap(int *a, int *b)
{
    int temp = *a;
    *a = *b;
    *b = temp;
}

// Function returning pointer
char* create_string(const char* text)
{
    char* result = malloc(strlen(text) + 1);
    strcpy(result, text);
    return result;
}

// Static function (file scope)
static void helper_function()
{
    printf("Helper function\n");
}

// Function with struct parameter
typedef struct {
    int x;
    int y;
} Point;

int calculate_distance(Point p1, Point p2)
{
    int dx = p2.x - p1.x;
    int dy = p2.y - p1.y;
    return dx * dx + dy * dy;
}

// Variadic function
int sum_all(int count, ...)
{
    va_list args;
    va_start(args, count);

    int total = 0;
    for (int i = 0; i < count; i++) {
        total += va_arg(args, int);
    }

    va_end(args);
    return total;
}

// Main function
int main(void)
{
    printf("Result: %d\n", add(5, 3));

    int x = 10, y = 20;
    swap(&x, &y);
    printf("After swap: x=%d, y=%d\n", x, y);

    return 0;
}
