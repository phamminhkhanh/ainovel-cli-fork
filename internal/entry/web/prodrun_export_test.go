package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportTXTConcatenatesChapters(t *testing.T) {
	dir := t.TempDir()
	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ps.create("novel", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	chapDir := filepath.Join(ps.runDir(r.ID), "output", "novel", "chapters")
	if err := os.MkdirAll(chapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Deliberately out of order to verify numeric sorting.
	files := map[string]string{
		"02.md": "Second chapter text.",
		"10.md": "Tenth chapter text.",
		"01.md": "First chapter text.",
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(chapDir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	path, err := exportRunTXT(ps, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "novel.txt") {
		t.Fatalf("expected export path ending in novel.txt, got %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, "Chapter 01") || !strings.Contains(out, "Chapter 10") {
		t.Fatalf("missing chapter headers in export: %s", out)
	}
	first := strings.Index(out, "First chapter text.")
	second := strings.Index(out, "Second chapter text.")
	tenth := strings.Index(out, "Tenth chapter text.")
	if !(first < second && second < tenth) {
		t.Fatalf("chapters not in numeric order: %d %d %d", first, second, tenth)
	}
}

func TestExportTXTMissingChapters(t *testing.T) {
	dir := t.TempDir()
	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ps.create("empty", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	_, err = exportRunTXT(ps, r.ID)
	if err == nil {
		t.Fatal("expected error when chapters dir is missing")
	}
}
