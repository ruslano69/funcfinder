// STRESS TEST: Complex C++ with templates, nested classes, multiple inheritance
#include <iostream>
#include <vector>
#include <map>
#include <string>
#include <memory>
#include <functional>

// === TEMPLATE CLASSES ===
template<typename T, typename U = int>
class TemplateClass {
public:
    T value;
    U secondary;
    std::vector<T> items;
    std::map<std::string, T> mapping;
    
    template<typename V>
    V convert(V input) { return input; }
};

template<typename T>
class SpecializedTemplate : public TemplateClass<T> {
private:
    std::unique_ptr<T> ptr;
protected:
    T* raw_ptr;
public:
    explicit SpecializedTemplate(T val) : ptr(std::make_unique<T>(val)), raw_ptr(ptr.get()) {}
    virtual ~SpecializedTemplate() = default;
};

// === NESTED CLASSES ===
class OuterClass {
private:
    int outer_field;
    
    class NestedLevel1 {
    private:
        std::string nested1_field;
        
        class NestedLevel2 {
        private:
            double nested2_field;
            
            class NestedLevel3 {
            private:
                bool nested3_field;
            public:
                NestedLevel3() : nested3_field(false) {}
                bool getValue() const { return nested3_field; }
            } nl3;
        public:
            NestedLevel2() : nested2_field(0.0) {}
            double getValue() const { return nested2_field; }
        } nl2;
    public:
        NestedLevel1() : nested1_field("") {}
        std::string getValue() const { return nested1_field; }
    } nl1;
    
    class StaticNested {
    public:
        static int static_field;
        static constexpr double PI = 3.14159;
        static void staticMethod() {}
    };
public:
    OuterClass() : outer_field(0) {}
    int getOuter() const { return outer_field; }
};

// === MULTIPLE INHERITANCE ===
class Base1 {
protected:
    int base1_field;
public:
    virtual ~Base1() = default;
    virtual void method1() = 0;
};

class Base2 {
protected:
    std::string base2_field;
public:
    virtual ~Base2() = default;
    virtual void method2() = 0;
};

class DerivedMultiple : public Base1, public Base2 {
private:
    double derived_field;
public:
    void method1() override { base1_field = 0; }
    void method2() override { base2_field = ""; }
    double getValue() const { return derived_field; }
};

// === VIRTUAL INHERITANCE ===
class VirtualBase {
protected:
    int virtual_base_field;
public:
    VirtualBase(int v) : virtual_base_field(v) {}
    virtual ~VirtualBase() = default;
    virtual int getVirtualBase() const = 0;
};

class VirtualDerived1 : virtual public VirtualBase {
protected:
    int virtual_derived1_field;
public:
    VirtualDerived1(int v) : VirtualBase(v), virtual_derived1_field(v) {}
    int getVirtualBase() const override { return virtual_base_field; }
};

class VirtualDerived2 : virtual public VirtualBase {
protected:
    int virtual_derived2_field;
public:
    VirtualDerived2(int v) : VirtualBase(v), virtual_derived2_field(v) {}
    int getVirtualBase() const override { return virtual_base_field + 1; }
};

class VirtualFinal : public VirtualDerived1, public VirtualDerived2 {
public:
    VirtualFinal(int v) : VirtualBase(v), VirtualDerived1(v), VirtualDerived2(v) {}
};

// === ABSTRACT CLASSES ===
template<typename T>
class AbstractContainer {
public:
    virtual bool add(T item) = 0;
    virtual bool remove(T item) = 0;
    virtual bool contains(T item) const = 0;
    virtual size_t size() const = 0;
    virtual ~AbstractContainer() = default;
};

// === STL-LIKE CLASSES ===
template<typename T>
class MyVector : public AbstractContainer<T> {
private:
    T* data;
    size_t capacity_;
    size_t size_;
    
    void grow() {
        capacity_ *= 2;
        T* new_data = new T[capacity_];
        std::copy(data, data + size_, new_data);
        delete[] data;
        data = new_data;
    }
public:
    MyVector() : data(nullptr), capacity_(0), size_(0) {}
    ~MyVector() override { delete[] data; }
    
    bool add(T item) override {
        if (size_ >= capacity_) grow();
        data[size_++] = item;
        return true;
    }
    
    bool remove(T item) override { return false; }
    bool contains(T item) const override { return false; }
    size_t size() const override { return size_; }
};

// === ENUM CLASS ===
enum class Color : int {
    RED = 0,
    GREEN = 1,
    BLUE = 2,
    ALPHA = 4
};

enum OldStyleEnum {
    CONSTANT_A = 100,
    CONSTANT_B = 200
};

// === UNION ===
union DataUnion {
    int int_val;
    double double_val;
    char char_val;
    struct {
        int x;
        int y;
    } point;
};

// === EXTERN "C" ===
extern "C" {
    struct CStruct {
        int field1;
        int field2;
    };
    
    CStruct createCStruct(int a, int b) {
        CStruct s = {a, b};
        return s;
    }
}

// === NAMESPACE NESTING ===
namespace Level1 {
    namespace Level2 {
        namespace Level3 {
            struct DeepNestedStruct {
                int deep_field;
                std::string deep_string;
                std::vector<int> deep_vector;
            };
        }
    }
}

// === CLASS WITH TEMPLATE TEMPLATE PARAMETER ===
template<template<typename> class Container, typename T>
class TemplateTemplateClass {
private:
    Container<T> container;
public:
    void add(T item) { container.add(item); }
    size_t count() const { return container.size(); }
};

// === OPERATOR OVERLOADING CLASS ===
class BigNumber {
private:
    std::vector<int> digits;
    bool negative;
public:
    BigNumber() : negative(false) {}
    
    BigNumber operator+(const BigNumber& other) const {
        BigNumber result;
        return result;
    }
    
    BigNumber operator-(const BigNumber& other) const {
        BigNumber result;
        return result;
    }
    
    bool operator==(const BigNumber& other) const { return true; }
};

// === CRTP PATTERN ===
template<typename Derived>
class BaseCRTP {
public:
    void commonMethod() {
        static_cast<Derived*>(this)->specificMethod();
    }
};

class DerivedCRTP : public BaseCRTP<DerivedCRTP> {
public:
    void specificMethod() {}
};

// === TYPE ALIASES ===
using IntVector = std::vector<int>;
using StringMap = std::map<std::string, std::string>;
using Callback = std::function<void(int, int)>;
template<typename T>
using Unique = std::unique_ptr<T>;

// === FRIEND DECLARATIONS ===
class FriendClass {
private:
    int secret_field;
    friend class FriendDeclaration;
    friend void friendFunction(FriendClass& fc);
public:
    void accessFriend(FriendClass& other) {
        other.secret_field = 42;
    }
};

class FriendDeclaration {
public:
    void reveal(FriendClass& fc) {
        fc.secret_field = 0;
    }
};

// === DEFAULT/DELETE METHODS ===
class SpecialMethods {
private:
    int field;
public:
    SpecialMethods() = default;
    SpecialMethods(const SpecialMethods&) = delete;
    SpecialMethods& operator=(const SpecialMethods&) = delete;
    SpecialMethods(SpecialMethods&&) noexcept = default;
    SpecialMethods& operator=(SpecialMethods&&) noexcept = default;
    ~SpecialMethods() = default;
};

// === BITFIELD STRUCT ===
struct BitFields {
    unsigned int flag1 : 1;
    unsigned int flag2 : 1;
    unsigned int value : 6;
    int normal_field;
};

// === ATTRIBUTE ANNOTATIONS ===
struct [[deprecated]] OldStruct {
    int old_field;
};

struct [[nodiscard]] NewStruct {
    int new_field;
};

// === EXPLICIT SPECIALIZATION ===
template<typename T>
struct TypeHolder {
    static constexpr const char* type_name = "unknown";
};

template<>
struct TypeHolder<int> {
    static constexpr const char* type_name = "int";
};

template<>
struct TypeHolder<double> {
    static constexpr const char* type_name = "double";
};

// === COMPLEX CONSTRUCTOR ===
class ComplexInit {
private:
    const int constant_field;
    int& reference_field;
    std::vector<int> vector_field;
    std::map<std::string, int> map_field;
public:
    ComplexInit(int& ref) : constant_field(42), reference_field(ref), vector_field({1, 2, 3}) {
        map_field["key"] = 100;
    }
};

// === LAMBDA WRAPPER ===
template<typename F>
class LambdaWrapper {
private:
    F func;
public:
    explicit LambdaWrapper(F f) : func(std::move(f)) {}
    template<typename... Args>
    auto operator()(Args&&... args) {
        return func(std::forward<Args>(args)...);
    }
};

// === FINAL CLASS ===
class FinalClass final {
private:
    int final_field;
public:
    int get() const { return final_field; }
};

// === STRUCT IN FUNCTION ===
void functionWithStruct() {
    struct LocalStruct {
        int local_field;
        std::string local_string;
    };
    
    LocalStruct local = {42, "test"};
}

// === MOVE SEMANTICS CLASS ===
class MoveClass {
private:
    std::vector<int> data;
    std::string* allocated;
public:
    MoveClass() : data(), allocated(new std::string()) {}
    
    MoveClass(MoveClass&& other) noexcept 
        : data(std::move(other.data)), allocated(other.allocated) {
        other.allocated = nullptr;
    }
    
    MoveClass& operator=(MoveClass&& other) noexcept {
        if (this != &other) {
            delete allocated;
            data = std::move(other.data);
            allocated = other.allocated;
            other.allocated = nullptr;
        }
        return *this;
    }
    
    ~MoveClass() { delete allocated; }
};

// === RTTI CLASSES ===
class BaseVirtual {
public:
    virtual ~BaseVirtual() = default;
    virtual const char* identify() const { return "BaseVirtual"; }
};

class DerivedVirtual : public BaseVirtual {
public:
    const char* identify() const override { return "DerivedVirtual"; }
};
