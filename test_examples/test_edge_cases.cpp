// EDGE CASES TEST - Things that might confuse findstruct

#include <iostream>
#include <string>

// === STRINGS THAT LOOK LIKE STRUCT DEFINITIONS ===
const char* fake_struct = "struct { int x; }";  // String literal - should NOT be detected
std::string another_fake = R"(struct Foo { int bar; })";  // Raw string - should NOT be detected

// === COMMENTS THAT LOOK LIKE STRUCTS ===
/*
struct FakeComment {
    int field;
    int another;
};
*/

// Line comment that looks like struct:
// struct ShouldNotDetect { int x; }

// === CODE IN STRINGS ===
class StringExamples {
    const char* sql = "CREATE TABLE users (id INT, name VARCHAR(255))";  // SQL looks like struct
    const char* json = "{\"type\": \"struct\", \"fields\": []}";  // JSON looks like struct
    const char* javascript = "function struct() { return {x: 1}; }";  // JS object
};

// === ESCAPED CHARACTERS ===
struct EscapedBraces {
    int field = 0;
    const char* brace_in_string = "}{";  // Escaped braces
    const char* nested_braces = "{{{{}}}}";  // Multiple braces
};

// === TEMPLATE INSTANTIATIONS THAT LOOK LIKE STRUCTS ===
template<typename T>
class TemplateClass {};

TemplateClass<struct InnerType> instance1;  // Anonymous struct in template
TemplateClass<class> instance2;  // Anonymous class in template

// === ATTRIBUTE ANNOTATIONS ===
[[deprecated]]
struct DeprecatedStruct {
    int old_field;
};

[[nodiscard]]
struct NewStruct {
    int new_field;
};

struct MultipleAttributes {
    int field;
} [[maybe_unused]];

// === NAMESPACE SIMULATIONS IN STRINGS ===
const char* namespace_like = "std::vector<std::pair<int, std::string>>";

// === PREPROCESSOR THAT LOOKS LIKE STRUCT ===
#define FAKE_STRUCT struct { int fake; }

// === TEMPLATE SPECIALIZATIONS ===
template<typename T>
struct TemplateBase {
    T value;
};

template<>
struct TemplateBase<int> {
    int int_value;
};

template<>
struct TemplateBase<double> {
    double double_value;
};

// === FORWARD DECLARATIONS ===
struct ForwardDeclared;  // Should NOT be detected as full struct

struct UsingForwardDeclared {
    ForwardDeclared* ptr;
};

// === ANONYMOUS STRUCTS ===
struct {
    int anonymous_field;
} anonymous_instance;

struct Container {
    struct {
        int nested_anonymous;
    } nested;
};

// === EXTERN "C" NAMESPACE ===
extern "C" {
    struct CLinkage {
        int field;
    };
}

// === STATIC ASSERT ===
static_assert(sizeof(int) == 4, "int must be 4 bytes");

struct AfterStaticAssert {
    int after_field;
};

// === MACRO EXPANSION LINES ===
#define CREATE_STRUCT(name) struct name { int field; }
CREATE_STRUCT(MacroStruct);  // Expanded: struct MacroStruct { int field; }

// === COMMENTED CODE BLOCKS ===
// struct CommentedStruct { int field1; int field2; }  // Entire line commented
/* struct BlockComment { int x; } */  // Inline block comment

// === BACKTICK QUOTES IN STRINGS ===
const char* backticks = "`struct { invalid }`";  // Looks like struct but is string

// === REGEX PATTERNS IN STRINGS ===
const char* regex = "^\\s*struct\\s+(\\w+)\\s*\\{";  // Regex pattern looking like struct

// === NESTED ANGLE BRACKETS (NOT BRACES!) ===
template<typename T>
class Vector {};

Vector<std::map<std::pair<int, int>, std::string>> complex_template;  // Should NOT confuse brace counting

// === MULTI-LINE STRING THAT LOOKS LIKE STRUCT ===
const char* multiline =
    "struct { int x; "
    "int y; "
    "}";

// === UNFINISHED STRUCT (MISSING CLOSING BRACE) ===
struct Unfinished {
    int field1;
    int field2;
    // Missing closing brace - should handle gracefully

// === CODE AFTER UNFINISHED BRACE ===
struct AfterUnfinished {
    int should_still_find;
};

// === CLASS KEYWORD IN COMMENTS AND STRINGS ===
const char* class_in_string = "class MyClass { public: void method(); }";  // Should NOT be detected

// === GENERICS SYNTAX (JAVA/C#) THAT LOOKS LIKE TEMPLATE ===
// This is just a comment, so it's fine
// List<int> generics_type;

// === EMPTY STRUCT (VALID IN C11) ===
struct EmptyStruct {};

// === SINGLE FIELD STRUCT ===
struct SingleField {
    int only_one;
};

// === STRUCT WITH INITIALIZER ===
struct WithInit {
    int x = 0;
    int y = 1 + 2;
};

// === STRUCT WITH ATTRIBUTES IN MIDDLE ===
struct AttributesMiddle {
    int before;
    [[deprecated]] int deprecated_field;
    int after;
};

// === NAMESPACE WITH COLONS ===
namespace outer {
    namespace inner {
        struct DeepNested {
            int deep_field;
        };
    }
}

// === TYPEDEF STRUCT ===
typedef struct {
    int typedef_field;
} TypedefStruct;

typedef struct NamedStruct {
    int named_field;
} NamedTypedef;

// === UNION IN STRUCT ===
struct WithUnion {
    int type;
    union {
        int int_val;
        double double_val;
    };
};

// === BITFIELD IN STRUCT ===
struct BitFields {
    unsigned int a : 1;
    unsigned int b : 1;
    unsigned int c : 6;
};

// === STRUCT WITH METHOD DECLARATIONS ===
struct WithMethods {
    int field;
    void method();
    int property() { return field; }
};

// === FINAL/SEALED CLASSES ===
class FinalClass final {
    int final_field;
};

class BaseClass {
    virtual void foo() = 0;
};

class DerivedClass final : public BaseClass {
    void foo() override {}
};

// === OVERRIDE AND FINAL METHODS ===
struct OverrideFinal {
    virtual void method1() override {}
    virtual void method2() final {}
};

// === ABSTRACT CLASS ===
class AbstractClass {
public:
    virtual void pure_virtual() = 0;
    virtual void virtual_method() {}
};

// === TEMPLATE WITH DEFAULT PARAMETER ===
template<typename T = int>
struct DefaultTemplate {
    T value;
};

// === STRUCT WITH COMPLEX INITIALIZERS ===
struct ComplexInit {
    int arr[3] = {1, 2, 3};
    std::string str = "hello";
    int computed = 1 + 2 + 3;
};

// === NAMESPACE ALIAS ===
namespace ns = outer::inner;

struct UsingNamespaceAlias {
    ns::DeepNested nested;
};

// === EXPLICIT SPECIFIERS ===
struct ExplicitSpecifiers {
    explicit ExplicitSpecifiers(int) {}
    explicit operator bool() { return true; }
};

// === REF QUALIFIERS ===
struct RefQualifiers {
    void method() & {}
    void method_rvalue() && {}
};

// === NOEXCEPT SPECIFIER ===
struct NoExcept {
    void safe_method() noexcept {}
    void potentially_throwing() noexcept(false) {}
};

// === CONSTEXPR ===
struct ConstExpr {
    static constexpr int value = 42;
    constexpr ConstExpr() : field(0) {}
    int field;
};

// === TRAILING RETURN TYPE ===
struct TrailingReturn {
    auto get_value() -> int { return 0; }
};

// === DECLTYPE AUTO ===
struct DecltypeAuto {
    decltype(auto) get() { return 0; }
};

// === VARIADIC TEMPLATE ===
template<typename... Args>
struct VariadicTemplate {
    std::tuple<Args...> args;
};

// === FOLD EXPRESSION (C++17) ===
struct FoldStruct {
    template<typename... Args>
    void print_all(Args... args) {
        ((std::cout << args << " "), ...);
    }
};

// === IF CONSTEXPR (C++17) ===
struct IfConstexpr {
    template<typename T>
    auto get_value(T value) {
        if constexpr (std::is_integral_v<T>) {
            return value * 2;
        } else {
            return value;
        }
    }
};

// === STRUCT WITH AUTO FIELD DEDUCTION (C++17) ===
struct AutoFields {
    auto x = 1;
    decltype(auto) y = 2;
};

// === STRUCT WITH INITIALIZER LIST (C++17) ===
struct InitList {
    std::vector<int> v = {1, 2, 3};
};

// === INLINE VARIABLE (C++17) ===
struct InlineVar {
    inline static int counter = 0;
};

// === NESTED NAMESPACE DEFINITION (C++17) ===
namespace nested::namespace2 {
    struct NestedNs {
        int field;
    };
}

// === STRUCT WITH CLASS TEMPLATE ARGUMENT DEDUCTION (C++17) ===
template<typename T>
struct deduction_guide {
    T value;
};

// deduction_guide guide(42);  // CTAD - Class Template Argument Deduction
