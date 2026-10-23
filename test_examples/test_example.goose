// Goose declaration and literal fixture.

struct Point { x: i64, y: i64 }
enum Value { Empty, Number { value: i64 } }
type Count = i64;

// External functions are declarations, not bodies.
extern fn native(value: i64) -> i64 = "native_value";

inline fn identity<T>(value: T) -> T { return value; }

fn Alpha() -> i64 { return 1; } fn Beta() -> i64 { return 2; }

fn main() {
    /* nested comments must stay opaque:
       /* inner */ fn NotAFunction() { }
    */
    let text = """
        fn Hidden() { }
        struct HiddenType { value: i64 }
        """;
    fn nested(value: i64) -> i64 { return identity(value); }
    print(text, nested(Alpha() + Beta()));
}
