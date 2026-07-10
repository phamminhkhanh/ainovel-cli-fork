package web

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// seedRunWithChapters seeds a run's sandbox with real chapter files + a
// progress.json listing them completed, so exportRunEPUB can build an export.
func seedRunWithChapters(t *testing.T, ps *prodRunStore, name string, chapters map[int]string) *ProdRun {
	t.Helper()
	r, err := ps.create(name, "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	novelDir := filepath.Join(ps.runDir(r.ID), "output", "novel")
	chapDir := filepath.Join(novelDir, "chapters")
	metaDir := filepath.Join(novelDir, "meta")
	if err := os.MkdirAll(chapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	nums := make([]string, 0, len(chapters))
	for ch, text := range chapters {
		if err := os.WriteFile(filepath.Join(chapDir, fmt.Sprintf("%02d.md", ch)), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		nums = append(nums, strconv.Itoa(ch))
	}
	progress := `{"novel_name":"` + name + `","phase":"writing","completed_chapters":[` + strings.Join(nums, ",") + `]}`
	if err := os.WriteFile(filepath.Join(metaDir, "progress.json"), []byte(progress), 0o644); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestExportRunEPUBProducesValidEpub(t *testing.T) {
	dir := t.TempDir()
	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := seedRunWithChapters(t, ps, "My Novel", map[int]string{
		1: "# Chương 1: Tiếng Gọi Trong Đêm\n\nNội dung một.\n\n---\n\nĐoạn hai.",
		2: "# Chương 2: Sợi Dây Thứ Hai\n\nNội dung hai.",
	})

	path, err := exportRunEPUB(ps, r.ID)
	if err != nil {
		t.Fatalf("exportRunEPUB: %v", err)
	}
	if !strings.HasSuffix(path, ".epub") {
		t.Fatalf("expected .epub path, got %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("output is not a valid zip: %v", err)
	}
	// EPUB spec: first entry must be "mimetype", stored (not deflated),
	// content exactly "application/epub+zip".
	if len(zr.File) == 0 || zr.File[0].Name != "mimetype" {
		t.Fatalf("first zip entry must be mimetype, got %v", zipNames(zr))
	}
	if zr.File[0].Method != zip.Store {
		t.Fatalf("mimetype must be stored (uncompressed)")
	}
	rc, _ := zr.File[0].Open()
	mt, _ := io.ReadAll(rc)
	rc.Close()
	if string(mt) != "application/epub+zip" {
		t.Fatalf("mimetype content = %q", string(mt))
	}
	// Required structural files present.
	names := strings.Join(zipNames(zr), " ")
	for _, want := range []string{"META-INF/container.xml", "OEBPS/content.opf", "OEBPS/nav.xhtml", "OEBPS/chapter001.xhtml"} {
		if !strings.Contains(names, want) {
			t.Errorf("missing %s in epub (have: %s)", want, names)
		}
	}

	// Chapter heading must be the WRITER's own header (VN), never the engine's
	// hardcoded Chinese "第 N 章" — this is the whole reason for the web-side builder.
	ch1 := readZipEntry(t, zr, "OEBPS/chapter001.xhtml")
	if !strings.Contains(ch1, "Tiếng Gọi Trong Đêm") {
		t.Errorf("chapter001 missing writer heading; got:\n%s", ch1)
	}
	if strings.Contains(ch1, "第") {
		t.Errorf("chapter001 must not contain Chinese chapter label 第; got:\n%s", ch1)
	}
	if !strings.Contains(ch1, `<hr class="scene"`) {
		t.Errorf("chapter001 should render '---' as a scene break; got:\n%s", ch1)
	}
	nav := readZipEntry(t, zr, "OEBPS/nav.xhtml")
	if !strings.Contains(nav, "Sợi Dây Thứ Hai") {
		t.Errorf("nav TOC missing writer heading for ch2; got:\n%s", nav)
	}
}

func readZipEntry(t *testing.T, zr *zip.Reader, name string) string {
	t.Helper()
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			defer rc.Close()
			data, _ := io.ReadAll(rc)
			return string(data)
		}
	}
	t.Fatalf("zip entry %s not found", name)
	return ""
}

func TestExportRunEPUBNoChapters(t *testing.T) {
	dir := t.TempDir()
	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ps.create("empty", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exportRunEPUB(ps, r.ID); !errors.Is(err, errExportNoChapters) {
		t.Fatalf("want errExportNoChapters, got %v", err)
	}
}

func zipNames(zr *zip.Reader) []string {
	out := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		out = append(out, f.Name)
	}
	return out
}


