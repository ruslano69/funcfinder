package internal

import (
	"strings"
	"testing"
)

// The finder's span starts at the first decorator or annotation, so a replacement has to restate them.
func TestReplaceFunction_DecoratorsAreRestated(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	cases := []struct {
		lang, src, old, body string
		decorators           string
	}{
		{
			lang:       "py",
			src:        "import functools\n\n\n@functools.lru_cache(maxsize=None)\n@staticmethod\ndef add(a, b):\n    return a + b\n\n\ndef keep():\n    return 0\n",
			old:        "@functools.lru_cache(maxsize=None)\n@staticmethod\ndef add(a, b):\n    return a + b",
			body:       "def add(a, b):\n    return a * b",
			decorators: "@functools.lru_cache(maxsize=None)\n@staticmethod\n",
		},
		{
			lang:       "java",
			src:        "public class Demo {\n    @Override\n    public int add(int a, int b) {\n        return a + b;\n    }\n\n    public int keep() {\n        return 0;\n    }\n}\n",
			old:        "    @Override\n    public int add(int a, int b) {\n        return a + b;\n    }",
			body:       "    public int add(int a, int b) {\n        return a * b;\n    }",
			decorators: "    @Override\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			lc, err := cfg.GetLanguageConfig(tc.lang)
			if err != nil {
				t.Fatalf("config: %v", err)
			}
			// A replacement without decorators keeps the old ones: they are never lost by omission.
			res, err := ReplaceFunction(lc, []byte(tc.src), FuncTarget{Name: "add"}, nil, []byte(tc.body))
			if err != nil {
				t.Fatalf("replacement without decorators refused: %v", err)
			}
			want := strings.Replace(tc.src, tc.old, tc.decorators+tc.body, 1)
			if string(res.Output) != want {
				t.Fatalf("decorators lost or changed:\n%s\nwant:\n%s", res.Output, want)
			}
			// Restating the same decorators is accepted too.
			if _, err := ReplaceFunction(lc, []byte(tc.src), FuncTarget{Name: "add"}, nil, []byte(tc.decorators+tc.body)); err != nil {
				t.Fatalf("restated decorators refused: %v", err)
			}
			// Different decorators are refused, so a change of decorators is never silent.
			if _, err := ReplaceFunction(lc, []byte(tc.src), FuncTarget{Name: "add"}, nil, []byte(strings.Replace(tc.decorators, "@", "@Other", 1)+tc.body)); err == nil || !strings.Contains(err.Error(), "decorators") {
				t.Fatalf("different decorators accepted: %v", err)
			}
		})
	}
}
