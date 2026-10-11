package internal

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// rustTreeSample is store.rs from a small crate that exposed the problems: a
// trait, a struct, an inherent impl of the struct, and a trait impl for it.
const rustTreeSample = `pub trait Named {
    fn name(&self) -> String;
}

pub struct Store {
    data: HashMap<String, i32>,
}

impl Store {
    pub fn new() -> Self {
        Store { data: HashMap::new() }
    }

    pub fn put(&mut self, key: &str, value: i32) {
        self.data.insert(key.to_string(), value);
        self.check();
    }

    fn check(&self) {
        if self.data.len() > 10 {
            println!("big");
        }
    }
}

impl Named for Store {
    fn name(&self) -> String {
        String::from("store")
    }
}
`

// rustTree builds the --tree structure the way the directory processor does for
// the "all" mode: functions from the function finder and types from the struct
// finder, both reading the file, merged into one FindResult.
func rustTree(t *testing.T, src string) []*TreeNode {
	t.Helper()
	cfg := getRustConfig(t)
	path := filepath.Join(t.TempDir(), "store.rs")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	funcs, err := CreateFinder(cfg, "", "map", false, false).FindFunctions(path)
	if err != nil {
		t.Fatalf("FindFunctions: %v", err)
	}
	structs, err := NewStructFinderFactory().CreateStructFinder(cfg, "", true, false).FindStructures(path)
	if err != nil {
		t.Fatalf("FindStructures: %v", err)
	}

	result := &FindResult{Functions: funcs.Functions, Filename: path}
	for _, typ := range structs.Types {
		result.Classes = append(result.Classes, ClassBounds{Name: typ.Name, Start: typ.Start, End: typ.End})
	}
	return BuildTree(result)
}

// classNodes returns the top-level class nodes with the given name.
func classNodes(tree []*TreeNode, name string) []*TreeNode {
	var out []*TreeNode
	for _, n := range tree {
		if n.Type == NodeTypeClass && n.Name == name {
			out = append(out, n)
		}
	}
	return out
}

// methodNames returns the sorted names of a class node's children.
func methodNames(n *TreeNode) []string {
	var names []string
	for _, c := range n.Children {
		names = append(names, c.Name)
	}
	sort.Strings(names)
	return names
}

// Methods of one type are one class in the tree, whatever number of blocks
// declare them. The struct and its inherent impl must not become two nodes,
// each carrying every method.
func TestRustTree_StructAndImplAreOneClass(t *testing.T) {
	tree := rustTree(t, rustTreeSample)

	stores := classNodes(tree, "Store")
	if len(stores) != 1 {
		t.Fatalf("got %d top-level Store nodes, want 1", len(stores))
	}
	want := []string{"check", "name", "new", "put"}
	if got := methodNames(stores[0]); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Store methods = %v, want %v", got, want)
	}
}

// A trait impl's method belongs to the implementing type, not to the trait.
// The trait keeps only its own declaration.
func TestRustTree_TraitImplMethodBelongsToType(t *testing.T) {
	tree := rustTree(t, rustTreeSample)

	named := classNodes(tree, "Named")
	if len(named) != 1 {
		t.Fatalf("got %d Named nodes, want 1 (the trait)", len(named))
	}
	if got := methodNames(named[0]); strings.Join(got, ",") != "name" {
		t.Fatalf("Named methods = %v, want just the trait declaration", got)
	}
	// The trait declaration is line 2, the impl's method is line 27.
	if named[0].Children[0].Start != 2 {
		t.Fatalf("Named's method is at line %d, want the trait declaration at line 2", named[0].Children[0].Start)
	}

	store := classNodes(tree, "Store")[0]
	found := false
	for _, c := range store.Children {
		if c.Name == "name" && c.Start == 27 {
			found = true
		}
	}
	if !found {
		t.Fatal("the impl Named for Store method (line 27) is not under Store")
	}
}

// Each function appears at most once in the whole tree.
func TestRustTree_NoFunctionAppearsTwice(t *testing.T) {
	tree := rustTree(t, rustTreeSample)

	seen := map[int]string{}
	var walk func(nodes []*TreeNode)
	walk = func(nodes []*TreeNode) {
		for _, n := range nodes {
			if n.Type == NodeTypeFunction {
				if prev, dup := seen[n.Start]; dup {
					t.Errorf("function at line %d appears twice (%s and %s)", n.Start, prev, n.Name)
				}
				seen[n.Start] = n.Name
			}
			walk(n.Children)
		}
	}
	walk(tree)
	if len(seen) != 5 {
		t.Fatalf("found %d functions in the tree, want 5 (trait decl, new, put, check, impl name)", len(seen))
	}
}

// A generic impl declared with no space before the type parameters is still one
// class, named by the type.
func TestRustTree_GenericImplGroupsByType(t *testing.T) {
	src := `pub struct Wrapper<T> {
    value: T,
}

impl<T> Wrapper<T> {
    pub fn get(&self) -> &T {
        &self.value
    }
}
`
	tree := rustTree(t, src)

	wrappers := classNodes(tree, "Wrapper")
	if len(wrappers) != 1 {
		t.Fatalf("got %d Wrapper nodes, want 1", len(wrappers))
	}
	if got := methodNames(wrappers[0]); strings.Join(got, ",") != "get" {
		t.Fatalf("Wrapper methods = %v, want [get]", got)
	}
}
