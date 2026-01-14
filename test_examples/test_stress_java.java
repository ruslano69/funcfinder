// STRESS TEST: Complex Java with generics, interfaces, enums, annotations
package com.factory.stress;

import java.util.*;
import java.util.concurrent.*;
import java.util.function.*;
import java.util.stream.*;

// === GENERIC CLASSES ===
class GenericContainer<T, U extends Number> {
    private T primary;
    private U secondary;
    private List<T> items = new ArrayList<>();
    private Map<String, T> mapping = new HashMap<>();
    
    public void setPrimary(T value) { this.primary = value; }
    public void setSecondary(U value) { this.secondary = value; }
    public T getPrimary() { return primary; }
    public U getSecondary() { return secondary; }
    
    public <V extends Comparable<V>> V compare(V a, V b) {
        return a.compareTo(b) > 0 ? a : b;
    }
}

class WildcardContainer<?> {
    private List<?> items;
    
    public void processItems(List<?> list) {
        // Wildcard capture
        for (Object item : list) {
            System.out.println(item);
        }
    }
    
    public void processWithBounds(List<? extends Number> numbers) {
        for (Number n : numbers) {
            System.out.println(n.doubleValue());
        }
    }
    
    public void addWithBounds(List<? super Integer> integers) {
        integers.add(42);
    }
}

// === INTERFACE HIERARCHY ===
interface BaseInterface {
    void baseMethod();
    int baseField = 0;
}

interface MiddleInterface extends BaseInterface {
    void middleMethod();
    String middleField = "";
}

interface DerivedInterface extends MiddleInterface, BaseInterface {
    void derivedMethod();
    List<String> derivedField = new ArrayList<>();
}

// === FUNCTIONAL INTERFACES ===
@FunctionalInterface
interface SimpleFunction<T, R> {
    R apply(T input);
    default String description() { return "Function"; }
}

interface Supplier<T> {
    T get();
}

interface Consumer<T> {
    void accept(T t);
}

interface BiFunction<T, U, R> {
    R apply(T t, U u);
}

interface Predicate<T> {
    boolean test(T t);
    default Predicate<T> and(Predicate<? super T> other) {
        return t -> this.test(t) && other.test(t);
    }
    default Predicate<T> or(Predicate<? super T> other) {
        return t -> this.test(t) || other.test(t);
    }
}

interface Comparator<T> {
    int compare(T o1, T o2);
    
    static <T extends Comparable<? super T>> Comparator<T> naturalOrder() {
        return (a, b) -> a.compareTo(b);
    }
    
    default Comparator<T> reversed() {
        return (a, b) -> this.compare(b, a);
    }
}

// === ABSTRACT CLASSES ===
abstract class AbstractBase<T> {
    protected T field;
    public abstract T getField();
    public abstract void setField(T value);
    
    public void templateMethod() {
        step1();
        step2();
        step3();
    }
    
    protected void step1() {}
    protected void step2() {}
    protected void step3() {}
}

abstract class AbstractIntermediate<T, U> extends AbstractBase<T> {
    protected U secondary;
    public abstract U getSecondary();
    public abstract void setSecondary(U value);
}

// === CONCRETE IMPLEMENTATIONS ===
class ConcreteImplementation extends AbstractIntermediate<String, Integer> {
    @Override
    public String getField() { return field; }
    
    @Override
    public void setField(String value) { this.field = value; }
    
    @Override
    public Integer getSecondary() { return secondary; }
    
    @Override
    public void setSecondary(Integer value) { this.secondary = value; }
}

class GenericImpl<T extends Number> extends AbstractBase<T> {
    @Override
    public T getField() { return field; }
    
    @Override
    public void setField(T value) { this.field = value; }
}

// === NESTED CLASSES ===
class OuterClass {
    private int outerField = 0;
    
    class MiddleClass {
        private int middleField = 1;
        
        class InnerClass {
            private int innerField = 2;
            
            public void accessAll() {
                System.out.println(outerField);
                System.out.println(middleField);
                System.out.println(innerField);
            }
        }
    }
    
    static class StaticNested {
        private int staticNestedField = 3;
        public void display() {
            // Cannot access outerField here - it's not static
            System.out.println(staticNestedField);
        }
    }
    
    private class PrivateInner {
        private int privateField = 4;
    }
}

// === LOCAL CLASS ===
class LocalClassDemo {
    public void demonstrate() {
        interface LocalInterface {
            void localMethod();
        }
        
        class LocalClass implements LocalInterface {
            @Override
            public void localMethod() {
                System.out.println("Local class method");
            }
        }
        
        LocalClass local = new LocalClass();
        local.localMethod();
    }
    
    public void anonymousClassDemo() {
        Runnable anonymous = new Runnable() {
            @Override
            public void run() {
                System.out.println("Anonymous run");
            }
        };
        
        Supplier<String> lambda = () -> "Lambda result";
        Consumer<String> consumer = s -> System.out.println(s);
        BiFunction<Integer, Integer, Integer> sum = (a, b) -> a + b;
    }
}

// === ENUM TYPES ===
enum SimpleEnum {
    VALUE_A, VALUE_B, VALUE_C
}

enum EnumWithFields {
    FIRST("first", 1),
    SECOND("second", 2),
    THIRD("third", 3);
    
    private final String name;
    private final int ordinalValue;
    
    EnumWithFields(String name, int ordinalValue) {
        this.name = name;
        this.ordinalValue = ordinalValue;
    }
    
    public String getName() { return name; }
    public int getOrdinalValue() { return ordinalValue; }
}

enum EnumWithMethods implements Runnable {
    A { public void run() { System.out.println("A"); } },
    B { public void run() { System.out.println("B"); } };
    
    public abstract void run();
    
    public void commonMethod() {
        System.out.println("Common to all");
    }
}

enum EnumWithConstants {
    MONDAY(Calendar.MONDAY),
    TUESDAY(Calendar.TUESDAY),
    WEDNESDAY(Calendar.WEDNESDAY);
    
    private final int calendarConstant;
    
    EnumWithConstants(int calendarConstant) {
        this.calendarConstant = calendarConstant;
    }
    
    public int getCalendarConstant() { return calendarConstant; }
}

// === ANNOTATIONS ===
@interface SimpleAnnotation {
    String value() default "";
}

@interface ComplexAnnotation {
    String name();
    int count() default 0;
    Class<?> type() default void.class;
    String[] tags() default {};
}

@SimpleAnnotation("test")
@ComplexAnnotation(name = "complex", count = 5, tags = {"tag1", "tag2"})
class AnnotatedClass {
    @SimpleAnnotation("field")
    private int annotatedField;
    
    @SimpleAnnotation("method")
    public void annotatedMethod() {}
}

// === RECORD TYPES (Java 16+) ===
record SimpleRecord(String name, int value) {
    public String getName() { return name; }
}

record RecordWithValidation(int id, String email) {
    public RecordWithValidation {
        if (id < 0) throw new IllegalArgumentException("ID must be positive");
        if (!email.contains("@")) throw new IllegalArgumentException("Invalid email");
    }
    
    public String getEmailDomain() {
        return email.substring(email.indexOf("@") + 1);
    }
}

record GenericRecord<T>(T value, String description) {}

// === SEALED CLASSES (Java 17+) ===
sealed class SealedBase permits SealedDerived1, SealedDerived2, SealedDerived3 {
    public abstract String getType();
}

final class SealedDerived1 extends SealedBase {
    public String getType() { return "Derived1"; }
}

final class SealedDerived2 extends SealedBase {
    public String getType() { return "Derived2"; }
}

sealed class SealedDerived3 extends SealedBase permits FurtherSealed {
    public String getType() { return "Derived3"; }
}

final class FurtherSealed extends SealedDerived3 {
    public String getType() { return "Further"; }
}

// === INNER CLASSES IN ENUM ===
enum EnumWithInnerClass {
    OPTION_A {
        @Override
        public String getValue() { return "A"; }
    },
    OPTION_B {
        @Override
        public String getValue() { return "B"; }
    };
    
    public abstract String getValue();
    
    private String privateHelper() { return "helper"; }
    
    public static class StaticInner {
        public static String staticValue = "Static";
    }
}

// === GENERIC INTERFACE ===
interface GenericInterface<T, U> {
    T process(U input);
    U reverse(T input);
    static <V> GenericInterface<V, V> identity() {
        return v -> v;
    }
}

class GenericInterfaceImpl implements GenericInterface<String, Integer> {
    @Override
    public String process(Integer input) { return String.valueOf(input); }
    
    @Override
    public Integer reverse(String input) { return Integer.parseInt(input); }
}

// === VARIANCE ===
interface Producer<T> {
    T produce();
}

interface Consumer<T> {
    void consume(T item);
}

// Covariant: Producer<? extends T> can produce T or subtypes
interface CovariantProducer<T> extends Producer<? extends T> {
    // Already covariant by nature of ? extends
}

// Contravariant: Consumer<? super T> can consume T or supertypes
interface ContravariantConsumer<T> extends Consumer<? super T> {
    // Already contravariant by nature of ? super
}

// === ABSTRACT METHODS ONLY INTERFACE ===
interface FunctionalInterface<T, R> {
    R apply(T t);
}

// === COMPLEX GENERICS ===
class ComplexGeneric<T extends Comparable<T>> {
    private List<T> elements = new ArrayList<>();
    private Map<T, Integer> counts = new HashMap<>();
    
    public void add(T element) {
        elements.add(element);
        counts.merge(element, 1, Integer::sum);
    }
    
    public Optional<T> find(Predicate<T> predicate) {
        return elements.stream().filter(predicate).findFirst();
    }
    
    public Map<T, Integer> getCounts() { return counts; }
    
    public <R extends Comparable<R>> R max(Function<T, R> extractor) {
        return elements.stream().map(extractor).max(Comparable::compareTo).orElse(null);
    }
}

// === EXCEPTION CLASSES ===
class CustomException extends Exception {
    private int errorCode;
    
    public CustomException(String message, int errorCode) {
        super(message);
        this.errorCode = errorCode;
    }
    
    public int getErrorCode() { return errorCode; }
}

class CustomRuntimeException extends RuntimeException {
    private final String errorDetails;
    
    public CustomRuntimeException(String message, String details) {
        super(message);
        this.errorDetails = details;
    }
    
    public String getErrorDetails() { return errorDetails; }
}

// === TYPEDEF-LIKE PATTERNS ===
class TypeDefinitions {
    public interface ActionListener {
        void onAction();
    }
    
    public interface EventHandler<T> {
        void handle(T event);
    }
    
    public interface Callback<R> {
        R call();
    }
    
    // Using these
    private Map<String, ActionListener> listeners = new HashMap<>();
    private Map<Class<?>, EventHandler<?>> handlers = new HashMap<>();
}

// === BUILDER PATTERN CLASS ===
class BuilderPattern {
    private final String required;
    private final int optionalInt;
    private final String optionalString;
    private final List<String> listField;
    private final Map<String, Integer> mapField;
    
    private BuilderPattern(Builder builder) {
        this.required = builder.required;
        this.optionalInt = builder.optionalInt;
        this.optionalString = builder.optionalString;
        this.listField = List.copyOf(builder.listField);
        this.mapField = Map.copyOf(builder.mapField);
    }
    
    public static class Builder {
        private final String required;
        private int optionalInt = 0;
        private String optionalString = "";
        private List<String> listField = new ArrayList<>();
        private Map<String, Integer> mapField = new HashMap<>();
        
        public Builder(String required) {
            this.required = required;
        }
        
        public Builder optionalInt(int value) {
            this.optionalInt = value;
            return this;
        }
        
        public Builder optionalString(String value) {
            this.optionalString = value;
            return this;
        }
        
        public Builder listField(String... values) {
            this.listField = Arrays.asList(values);
            return this;
        }
        
        public Builder mapField(Map<String, Integer> map) {
            this.mapField = new HashMap<>(map);
            return this;
        }
        
        public BuilderPattern build() {
            return new BuilderPattern(this);
        }
    }
}

// === SINGLETON ===
final class Singleton {
    private static final Singleton INSTANCE = new Singleton();
    private final int value;
    
    private Singleton() {
        this.value = 42;
    }
    
    public static Singleton getInstance() { return INSTANCE; }
    
    public int getValue() { return value; }
}

// === PROXY CLASS ===
interface ServiceInterface {
    void execute();
    String query(String input);
}

class ServiceImpl implements ServiceInterface {
    @Override
    public void execute() {
        System.out.println("Executing");
    }
    
    @Override
    public String query(String input) {
        return "Response to: " + input;
    }
}

class ServiceProxy implements ServiceInterface {
    private final ServiceImpl realService;
    
    public ServiceProxy() {
        this.realService = new ServiceImpl();
    }
    
    @Override
    public void execute() {
        realService.execute();
    }
    
    @Override
    public String query(String input) {
        return realService.query(input);
    }
}

// === FLUENT INTERFACE ===
class FluentBuilder {
    private String field1;
    private int field2;
    private boolean field3;
    
    public FluentBuilder field1(String value) {
        this.field1 = value;
        return this;
    }
    
    public FluentBuilder field2(int value) {
        this.field2 = value;
        return this;
    }
    
    public FluentBuilder field3(boolean value) {
        this.field3 = value;
        return this;
    }
    
    public FluentBuilder reset() {
        this.field1 = null;
        this.field2 = 0;
        this.field3 = false;
        return this;
    }
    
    public FluentBuilder copyFrom(FluentBuilder other) {
        this.field1 = other.field1;
        this.field2 = other.field2;
        this.field3 = other.field3;
        return this;
    }
}
