package internal

import (
	"reflect"
	"testing"
)

// A Kotlin function declared inside another function is a function: it ends at its own
// closing brace, and the outer one still ends at its own.
func TestDeclarations_KotlinNestedFunction(t *testing.T) {
	src := `fun outer(x: Int): Int {
    fun inner(y: Int): Int {
        return y + 1
    }
    return inner(x)
}

fun after() {
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"after:8-9", "inner:2-4", "outer:1-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A bodyless declaration ends on its last line when a property or annotation follows it.
func TestDeclarations_KotlinBodylessBeforeProperty(t *testing.T) {
	src := `interface Store {
    fun load(id: Int): String
    val size: Int
    fun save(s: String)
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"load:2-2", "save:4-4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// An expression body goes on while the next line starts with a call chain: the function
// ends at the end of the chain, not at the block inside it.
func TestDeclarations_KotlinExpressionBodyContinues(t *testing.T) {
	src := `fun pick(flag: Boolean) = if (flag) 1 else 2

fun load(url: String) = fetch(url)
    .map { it.trim() }
    .orEmpty()

fun other() = 3
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"load:3-5", "other:7-7", "pick:1-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A line that holds only a string literal is code: the expression body goes on to it, even
// though the sanitizer blanks the literal.
func TestDeclarations_KotlinExpressionEndsOnStringLine(t *testing.T) {
	src := `private fun makeId(url: String): String =
    "${url}_id"

@Serializable
data class Task(val id: Int)
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"makeId:1-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}
