package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProcessFile_AllMode_NoDuplicateClasses(t *testing.T) {
	tmpDir := t.TempDir()

	// Swift file with struct — both class_pattern and struct_type_patterns match
	swiftFile := filepath.Join(tmpDir, "test.swift")
	code := `public struct Logger {
    public let level: Int

    public func log(_ msg: String) {
        print(msg)
    }
}

public enum LogLevel {
    case info
    case error
}
`
	if err := os.WriteFile(swiftFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	dp := NewDirProcessor(config, 1, false, false, "all")

	job := Job{
		Path:      swiftFile,
		Extension: ".swift",
		LangKey:   "swift",
	}

	result := dp.processFile(job)
	if result.Error != nil {
		t.Fatalf("processFile() error = %v", result.Error)
	}

	// Check for duplicates: no two classes should share (Name, Start)
	seen := map[string]bool{}
	for _, c := range result.Classes {
		key := c.Name + ":" + string(rune(c.Start+'0'))
		if seen[key] {
			t.Errorf("Duplicate class found: %s at line %d", c.Name, c.Start)
		}
		seen[key] = true
	}

	// Logger and LogLevel should each appear exactly once
	names := map[string]int{}
	for _, c := range result.Classes {
		names[c.Name]++
	}
	for name, count := range names {
		if count > 1 {
			t.Errorf("Class %q appears %d times, want 1", name, count)
		}
	}
}

func TestProcessFile_AllMode_PreservesImplBlocks(t *testing.T) {
	tmpDir := t.TempDir()

	// Rust file with impl blocks — class_pattern catches impl, struct_type_patterns doesn't
	rustFile := filepath.Join(tmpDir, "test.rs")
	code := `pub struct Config {
    port: u16,
}

pub trait Handler {
    fn handle(&self);
}

impl Config {
    pub fn new() -> Self {
        Config { port: 8080 }
    }
}

impl Handler for Config {
    fn handle(&self) {
        println!("handling");
    }
}
`
	if err := os.WriteFile(rustFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	dp := NewDirProcessor(config, 1, false, false, "all")

	job := Job{
		Path:      rustFile,
		Extension: ".rs",
		LangKey:   "rust",
	}

	result := dp.processFile(job)
	if result.Error != nil {
		t.Fatalf("processFile() error = %v", result.Error)
	}

	// Should have 4 unique classes: Config(struct), Handler(trait), Config(impl), Handler(impl)
	if len(result.Classes) != 4 {
		t.Errorf("Got %d classes, want 4 (2 types + 2 impl blocks). Classes: %v",
			len(result.Classes), result.Classes)
	}

	// Verify no duplicates by (Name, Start)
	seen := map[string]bool{}
	for _, c := range result.Classes {
		key := c.Name + ":" + string(rune(c.Start+'0'))
		if seen[key] {
			t.Errorf("Duplicate class found: %s at line %d", c.Name, c.Start)
		}
		seen[key] = true
	}
}
