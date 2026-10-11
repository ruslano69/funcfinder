package internal

import (
	"reflect"
	"testing"
)

// An arrow function bound to a const is a function whether or not async is followed by a
// space: async() and async () are the same declaration. Minified code writes it without.
func TestDeclarations_TSAsyncArrowWithoutSpace(t *testing.T) {
	src := `const load=async()=>{
  return 1;
};
const save = async (x: number) => {
  return x;
};
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"load:1-3", "save:4-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}
