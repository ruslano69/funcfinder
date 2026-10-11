package internal

import (
	"strings"
	"testing"
)

// TypeScript arrow functions whose variable carries a type annotation —
// `const X: React.FC<P> = ({...}) => {...}`, `const run: Handler = (a) => ...` —
// must be found like their unannotated form. The TS func_pattern used to allow
// only `const X = (`, so every typed React component was invisible to
// funcfinder, complexity and callgraph (only its inner functions showed up).
func TestTypeScriptTypedArrowFunctions(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	ts := config["ts"]
	if ts == nil {
		t.Fatal("ts config not found")
	}

	code := `export const Topbar: React.FC<TopbarProps> = ({
  onOpenMenu,
  notifications = [],
  onMark = () => {},
}) => {
  return null;
};

const onPool: Run = (sql, params) => {
  return query(sql, params);
};

export const Provider: React.FC<{ userKey: string | null; children: React.ReactNode }> = ({ children }) => {
  return children;
};

const load: () => Promise<void> = async () => {
  await work();
};

export const Plain = ({ x }: { x: number }) => {
  return x;
};

const total: number = (1 + 2) * 3;

export default function App() {
  return null;
}

export default async function loadAll() {
  return [];
}`
	lines := strings.Split(code, "\n")
	finder := NewFinder(ts, nil, true, false, false)
	result, err := finder.FindFunctionsInLines(lines, 1, "test.tsx")
	if err != nil {
		t.Fatalf("FindFunctionsInLines() error = %v", err)
	}

	got := map[string][2]int{}
	for _, fn := range result.Functions {
		got[fn.Name] = [2]int{fn.Start, fn.End}
	}

	want := map[string][2]int{
		"Topbar":   {1, 7},
		"onPool":   {9, 11},
		"Provider": {13, 15},
		"Plain":    {21, 23},
		"App":      {27, 29},
		"loadAll":  {31, 33},
	}
	for name, span := range want {
		if g, ok := got[name]; !ok {
			t.Errorf("function %q not found; got %v", name, got)
		} else if g != span {
			t.Errorf("%s span = %v, want %v", name, g, span)
		}
	}
	// A typed arrow whose annotation itself contains "=>" is out of reach of a
	// line regex; it must not be misreported under a wrong name either.
	if _, ok := got["Promise"]; ok {
		t.Errorf("type annotation parsed as a function name: %v", got)
	}
	// A typed variable initialised with a parenthesised expression is not a function.
	if _, ok := got["total"]; ok {
		t.Errorf("typed non-function variable reported as function: %v", got)
	}
}
