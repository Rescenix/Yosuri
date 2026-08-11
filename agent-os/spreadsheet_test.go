package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCompanyInventoryCSVUsesRealFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), "company")
	home := filepath.Join(root, "researcher-01")
	if err := os.MkdirAll(filepath.Join(home, "projects", "001-demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "projects", "001-demo", "output-1.py"), []byte("print(1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	name, err := writeCompanyInventoryCSV(home)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, "outputs", name))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "output-1.py") || !strings.Contains(text, "researcher-01") {
		t.Fatalf("missing real evidence: %s", text)
	}
}
