package main

import (
	"fmt"
	"strings"
)

// SimpleFunction is a simple function without parameters
func SimpleFunction() {
	fmt.Println("Hello, World!")
}

// FunctionWithParams takes parameters and returns a value
func FunctionWithParams(x, y int) int {
	return x + y
}

// FunctionWithMultipleReturns returns multiple values
func FunctionWithMultipleReturns(name string) (string, int) {
	return strings.ToUpper(name), len(name)
}

// MethodWithReceiver is a method with a receiver
func (s *Server) MethodWithReceiver() string {
	return s.name
}

// NestedFunction contains a nested closure
func NestedFunction(x int) func(int) int {
	multiplier := 2

	return func(y int) int {
		return (x + y) * multiplier
	}
}

// VariadicFunction accepts variadic parameters
func VariadicFunction(prefix string, values ...int) string {
	result := prefix + ": "
	for _, v := range values {
		result += fmt.Sprintf("%d ", v)
	}
	return result
}

// GenericFunction is a generic function (Go 1.18+)
func GenericFunction[T any](value T) T {
	return value
}

// ComplexGenericFunction with multiple type parameters
func ComplexGenericFunction[K comparable, V any](m map[K]V, key K) (V, bool) {
	v, ok := m[key]
	return v, ok
}

type Server struct {
	name string
	port int
}

// Constructor pattern
func NewServer(name string, port int) *Server {
	return &Server{
		name: name,
		port: port,
	}
}

// MethodOnStruct demonstrates method on struct
func (s *Server) Start() error {
	fmt.Printf("Starting server %s on port %d\n", s.name, s.port)
	return nil
}

// PointerReceiver vs value receiver
func (s *Server) UpdateName(newName string) {
	s.name = newName
}

func (s Server) GetName() string {
	return s.name
}

// DeferPanicRecover demonstrates defer
func DeferPanicRecover() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered:", r)
		}
	}()

	panic("Something went wrong")
}

// init function (special case)
func init() {
	fmt.Println("Package initialized")
}

// main function
func main() {
	SimpleFunction()
	fmt.Println(FunctionWithParams(5, 3))
	server := NewServer("test", 8080)
	server.Start()
}
