// test_structs_go.go - Test file for Go struct detection
package main

// Simple struct
type Point struct {
	X int
	Y int
}

// Nested struct
type Rectangle struct {
	Point
	Width  int
	Height int
}

// Struct with interface
type Shape interface {
	Area() float64
}

// Another struct implementing interface
type Circle struct {
	Radius float64
}

func (c Circle) Area() float64 {
	return 3.14 * c.Radius * c.Radius
}

// Type alias
type IntList []int

// Complex nested struct
type User struct {
	ID        int
	Name      string
	Email     string
	Address   Address
	Orders    []Order
	CreatedAt string
}

type Address struct {
	Street string
	City   string
	Zip    string
}

type Order struct {
	ID     int
	Amount float64
	Items  []string
}

// Interface with multiple methods
type Database interface {
	Connect() error
	Query(query string) ([]string, error)
	Execute(stmt string) error
	Close()
}

// Generic-like struct (using interface)
type Container struct {
	Items []interface{}
}

// Struct with tags
type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Debug    bool   `json:"debug"`
	Timeout  int    `json:"timeout"`
}

// Nested in function (not supported in basic findstruct but good for testing)
func CreatePerson() struct {
	Name string
	Age  int
} {
	return struct {
		Name string
		Age  int
	}{}
}

// Empty struct
type Empty struct{}

// Struct with embedded pointer
type Node struct {
	Value int
	Next  *Node
}

// Interface embedding
type ReadWriteClose interface {
	Database
}

// Struct with multiple embedded types
type Composite struct {
	Point
	Circle
}

// Struct with complex field types
type ComplexStruct struct {
	Numbers    []int
	Mapping    map[string]int
	Callback   func(int) bool
	Channel    chan int
	Pointer    *Point
	Interface  interface{}
}
