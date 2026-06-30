package internal

import (
	"strings"
	"testing"
)

func getTSConfig(t *testing.T) *LanguageConfig {
	t.Helper()
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	tsConfig := config["ts"]
	if tsConfig == nil {
		t.Fatal("TypeScript config not found")
	}
	return tsConfig
}

func getRubyConfig(t *testing.T) *LanguageConfig {
	t.Helper()
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	rbConfig := config["ruby"]
	if rbConfig == nil {
		t.Fatal("Ruby config not found")
	}
	return rbConfig
}

// --- CreateStructFinder dispatch (determineFinderType) ---

func TestCreateStructFinder_DispatchesByLanguage(t *testing.T) {
	factory := NewStructFinderFactory()

	t.Run("python uses indent-based finder", func(t *testing.T) {
		finder := factory.CreateStructFinder(getPyConfig(t), "", true, false)
		if _, ok := finder.(*PythonStructFinder); !ok {
			t.Errorf("got %T, want *PythonStructFinder", finder)
		}
	})

	t.Run("typescript uses hybrid finder", func(t *testing.T) {
		finder := factory.CreateStructFinder(getTSConfig(t), "", true, false)
		if _, ok := finder.(*HybridStructFinder); !ok {
			t.Errorf("got %T, want *HybridStructFinder", finder)
		}
	})

	t.Run("go uses brace-based finder", func(t *testing.T) {
		finder := factory.CreateStructFinder(getGoConfig(t), "", true, false)
		if _, ok := finder.(*StructFinder); !ok {
			t.Errorf("got %T, want *StructFinder", finder)
		}
	})

	t.Run("ruby (block-end-keyword) uses brace-based finder despite being indent-like", func(t *testing.T) {
		finder := factory.CreateStructFinder(getRubyConfig(t), "", true, false)
		if _, ok := finder.(*StructFinder); !ok {
			t.Errorf("got %T, want *StructFinder", finder)
		}
	})
}

// --- HybridStructFinder: type detection across class/interface/type alias/enum ---

func TestHybridStructFinder_AllTypeKinds(t *testing.T) {
	// Brace-based kinds only (class/interface/enum). A bare type_alias with
	// no braces has its own dedicated test below, since it interacts with
	// brace-depth tracking differently — see
	// TestHybridStructFinder_TypeAliasWithoutBraces_SwallowsFollowingLines.
	tsConfig := getTSConfig(t)
	code := `export class Server {
    start() {}
}

export interface User {
    id: number;
}

enum Status {
    Active,
    Inactive,
}
`
	factory := NewStructFinderFactory()
	finder := factory.CreateStructFinder(tsConfig, "", true, false)

	result, err := finder.FindStructuresInLines(strings.Split(code, "\n"), 1, "server.ts")
	if err != nil {
		t.Fatalf("FindStructuresInLines() error = %v", err)
	}

	kinds := map[string]string{}
	for _, typ := range result.Types {
		kinds[typ.Name] = typ.Kind
	}

	want := map[string]string{
		"Server": "class",
		"User":   "interface",
		"Status": "enum",
	}
	for name, wantKind := range want {
		gotKind, ok := kinds[name]
		if !ok {
			t.Errorf("type %q not found; got types: %+v", name, result.Types)
			continue
		}
		if gotKind != wantKind {
			t.Errorf("type %q kind = %q, want %q", name, gotKind, wantKind)
		}
	}
}

func TestHybridStructFinder_TypeAliasWithoutBraces_SingleType(t *testing.T) {
	tsConfig := getTSConfig(t)
	lines := []string{"type Handler = (req: Request) => void;"}

	factory := NewStructFinderFactory()
	finder := factory.CreateStructFinder(tsConfig, "", true, false)

	result, err := finder.FindStructuresInLines(lines, 1, "handler.ts")
	if err != nil {
		t.Fatalf("FindStructuresInLines() error = %v", err)
	}
	if len(result.Types) != 1 || result.Types[0].Name != "Handler" {
		t.Fatalf("got types %+v, want a single Handler type_alias", result.Types)
	}
	if result.Types[0].Start != 1 || result.Types[0].End != 1 {
		t.Errorf("Handler span = [%d,%d], want [1,1] for a single-line alias", result.Types[0].Start, result.Types[0].End)
	}
}

func TestHybridStructFinder_TypeAliasWithoutBraces_SwallowsFollowingLines(t *testing.T) {
	// Known limitation, pinned here rather than silently relied upon: since a
	// brace-less type alias never triggers the depth>0 -> 0 transition that
	// closes a type, it stays "open" and absorbs every subsequent line —
	// including what looks like a separate class — until some later brace
	// pair drives depth back to 0. That brace event closes the *alias*, not
	// the class, so the class is never recorded as its own type.
	tsConfig := getTSConfig(t)
	code := `type Handler = (req: Request) => void;

class Server {
    start() {}
}
`
	factory := NewStructFinderFactory()
	finder := factory.CreateStructFinder(tsConfig, "", true, false)

	result, err := finder.FindStructuresInLines(strings.Split(code, "\n"), 1, "handler.ts")
	if err != nil {
		t.Fatalf("FindStructuresInLines() error = %v", err)
	}
	if len(result.Types) != 1 || result.Types[0].Name != "Handler" {
		t.Fatalf("got types %+v, want only Handler (Server is swallowed by current depth-tracking behavior)", result.Types)
	}
}

func TestHybridStructFinder_FindStructures_ReadsFromDisk(t *testing.T) {
	code := "export class Widget {\n    render() {}\n}\n"
	tmpfile := createTempFile(t, code, "test_hybrid_*.ts")

	factory := NewStructFinderFactory()
	finder := factory.CreateStructFinder(getTSConfig(t), "", true, false)

	result, err := finder.FindStructures(tmpfile)
	if err != nil {
		t.Fatalf("FindStructures() error = %v", err)
	}
	if len(result.Types) != 1 || result.Types[0].Name != "Widget" {
		t.Errorf("got types %+v, want a single Widget class", result.Types)
	}
	if result.Filename != tmpfile {
		t.Errorf("Filename = %q, want %q", result.Filename, tmpfile)
	}
}

func TestHybridStructFinder_FindStructures_MissingFileErrors(t *testing.T) {
	factory := NewStructFinderFactory()
	finder := factory.CreateStructFinder(getTSConfig(t), "", true, false)

	if _, err := finder.FindStructures("does-not-exist.ts"); err == nil {
		t.Error("FindStructures() on a missing file should return an error")
	}
}

func TestHybridStructFinder_TypeNamesFilter(t *testing.T) {
	tsConfig := getTSConfig(t)
	code := `class Wanted {}
class Ignored {}
`
	factory := NewStructFinderFactory()
	// mapMode=false with an explicit type name list: only "Wanted" should be picked up.
	finder := factory.CreateStructFinder(tsConfig, "Wanted", false, false)

	result, err := finder.FindStructuresInLines(strings.Split(code, "\n"), 1, "filtered.ts")
	if err != nil {
		t.Fatalf("FindStructuresInLines() error = %v", err)
	}
	if len(result.Types) != 1 || result.Types[0].Name != "Wanted" {
		t.Errorf("got types %+v, want only Wanted", result.Types)
	}
}

func TestHybridStructFinder_NestedBracesTrackDepthToCorrectEnd(t *testing.T) {
	tsConfig := getTSConfig(t)
	code := `class Outer {
    handler() {
        if (true) {
            doStuff();
        }
    }
}
class After {}
`
	factory := NewStructFinderFactory()
	finder := factory.CreateStructFinder(tsConfig, "", true, false)

	result, err := finder.FindStructuresInLines(strings.Split(code, "\n"), 1, "nested.ts")
	if err != nil {
		t.Fatalf("FindStructuresInLines() error = %v", err)
	}

	var outer, after *TypeBounds
	for i := range result.Types {
		switch result.Types[i].Name {
		case "Outer":
			outer = &result.Types[i]
		case "After":
			after = &result.Types[i]
		}
	}
	if outer == nil || after == nil {
		t.Fatalf("expected both Outer and After types, got %+v", result.Types)
	}
	// Outer spans through its nested if-block to the closing brace on line 7.
	if outer.End != 7 {
		t.Errorf("Outer.End = %d, want 7 (nested braces should not close it early)", outer.End)
	}
	if after.Start != 8 {
		t.Errorf("After.Start = %d, want 8", after.Start)
	}
}

func TestHybridStructFinder_FieldsTypeBeforeNameSyntaxDetected(t *testing.T) {
	// field_pattern expects "[modifiers] Type fieldName ;" ordering. This is
	// not idiomatic TypeScript (which uses "fieldName: Type"), but it is what
	// the configured regex actually matches — this test pins that behavior.
	tsConfig := getTSConfig(t)
	code := `class Config {
    public string Host;
    private number Port;
}
`
	factory := NewStructFinderFactory()
	finder := factory.CreateStructFinder(tsConfig, "", true, false)

	result, err := finder.FindStructuresInLines(strings.Split(code, "\n"), 1, "config.ts")
	if err != nil {
		t.Fatalf("FindStructuresInLines() error = %v", err)
	}
	if len(result.Types) != 1 {
		t.Fatalf("got %d types, want 1", len(result.Types))
	}
	fields := result.Types[0].Fields
	names := map[string]bool{}
	for _, f := range fields {
		names[f.Name] = true
	}
	if !names["Host"] || !names["Port"] {
		t.Errorf("expected fields Host and Port, got %+v", fields)
	}
}

func TestHybridStructFinder_FieldsIdiomaticColonSyntaxNotDetected(t *testing.T) {
	// Documents the inverse of the above: idiomatic TS "name: Type;" field
	// syntax does NOT match field_pattern, so interfaces/classes written the
	// normal TypeScript way currently report zero fields.
	tsConfig := getTSConfig(t)
	code := `interface User {
    id: number;
    name: string;
}
`
	factory := NewStructFinderFactory()
	finder := factory.CreateStructFinder(tsConfig, "", true, false)

	result, err := finder.FindStructuresInLines(strings.Split(code, "\n"), 1, "user.ts")
	if err != nil {
		t.Fatalf("FindStructuresInLines() error = %v", err)
	}
	if len(result.Types) != 1 {
		t.Fatalf("got %d types, want 1", len(result.Types))
	}
	if len(result.Types[0].Fields) != 0 {
		t.Errorf("expected 0 fields detected for colon-syntax interface, got %+v", result.Types[0].Fields)
	}
}

func TestHybridStructFinder_NilFieldPatternReturnsEmpty(t *testing.T) {
	// findFieldsForType must short-circuit to an empty slice rather than
	// dereference a nil regex when a language has no field_pattern compiled.
	tsConfig := *getTSConfig(t) // shallow copy
	tsConfig.fieldRegex = nil

	factory := NewStructFinderFactory()
	finder := factory.CreateStructFinder(&tsConfig, "", true, false)

	code := "class Thing {\n    public string value;\n}\n"
	result, err := finder.FindStructuresInLines(strings.Split(code, "\n"), 1, "thing.ts")
	if err != nil {
		t.Fatalf("FindStructuresInLines() error = %v", err)
	}
	if len(result.Types) != 1 {
		t.Fatalf("got %d types, want 1", len(result.Types))
	}
	if len(result.Types[0].Fields) != 0 {
		t.Errorf("expected no fields with a nil field pattern, got %+v", result.Types[0].Fields)
	}
}
