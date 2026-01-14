package internal

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestStructFinderIntegration(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Error loading config: %v", err)
	}
	
	goConfig := config["go"]
	
	factory := NewStructFinderFactory()
	// Use mapMode=true to find all types, or provide specific typeNames
	finder := factory.CreateStructFinder(goConfig, "", true, false)
	
	code := `package main

type User struct {
	ID      int
	Name    string
	Email   string
	Age     int
}

type Admin struct {
	User
	Permissions []string
}

func (u *User) GetName() string {
	return u.Name
}`
	
	lines := strings.Split(code, "\n")
	result, err := finder.FindStructuresInLines(lines, 1, "test.go")
	if err != nil {
		t.Fatalf("Error finding structs: %v", err)
	}
	
	fmt.Printf("Found %d types:\n", len(result.Types))
	for _, typeInfo := range result.Types {
		fmt.Printf("  - %s (line %d-%d, %d fields)\n", typeInfo.Name, typeInfo.Start, typeInfo.End, len(typeInfo.Fields))
	}
	
	if len(result.Types) != 2 {
		t.Errorf("Expected 2 types, got %d", len(result.Types))
	}
	
	// Check User struct
	userFound := false
	for _, typeInfo := range result.Types {
		if typeInfo.Name == "User" {
			userFound = true
			if len(typeInfo.Fields) != 4 {
				t.Errorf("Expected User to have 4 fields, got %d", len(typeInfo.Fields))
			}
			break
		}
	}
	if !userFound {
		t.Error("User struct not found")
	}
}

func TestStructFinderWithStrings(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Error loading config: %v", err)
	}
	
	goConfig := config["go"]
	factory := NewStructFinderFactory()
	// Use mapMode=true to find all types
	finder := factory.CreateStructFinder(goConfig, "", true, false)
	
	// Code with strings and comments that should be ignored
	code := `package main

type Product struct {
	Name        string
	Description string
	Price       float64
}

// This is a comment with "quotes"
func helper() {
	msg := "This is a string with } braces } inside"
	_ = msg
}`
	
	lines := strings.Split(code, "\n")
	result, err := finder.FindStructuresInLines(lines, 1, "test.go")
	if err != nil {
		t.Fatalf("Error finding structs: %v", err)
	}
	
	fmt.Printf("Found %d types in string/comment test:\n", len(result.Types))
	for _, typeInfo := range result.Types {
		fmt.Printf("  - %s (line %d-%d, %d fields)\n", typeInfo.Name, typeInfo.Start, typeInfo.End, len(typeInfo.Fields))
	}
	
	if len(result.Types) != 1 {
		t.Errorf("Expected 1 type, got %d", len(result.Types))
	}
	
	if len(result.Types) > 0 {
		product := result.Types[0]
		if product.Name != "Product" {
			t.Errorf("Expected Product struct, got %s", product.Name)
		}
		
		if len(product.Fields) != 3 {
			t.Errorf("Expected 3 fields, got %d", len(product.Fields))
		}
	}
}

func TestStructFinderMain(t *testing.T) {
	if os.Getenv("RUN_STRUCT_TESTS") != "1" {
		t.Skip("Skipping struct finder integration test (requires network/module)")
	}
	
	// This test would run the actual findstruct command
	// Skip for now as it requires proper module setup
}
