package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/store"
)

type fakeContentEngine struct {
	dir   string
	store *store.Store
}

func newFakeEngine(t *testing.T, totalChapters int) *fakeContentEngine {
	t.Helper()
	dir := t.TempDir()
	return &fakeContentEngine{dir: dir, store: store.NewStore(dir)}
}

func (f *fakeContentEngine) Store() *store.Store { return f.store }

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestServeChapterReturnsFinalText(t *testing.T) {
	eng := newFakeEngine(t, 2)
	writeFile(t, eng.dir, "chapters/01.md", "# Chương 1\n\nNội dung chương 1.")

	req := httptest.NewRequest(http.MethodGet, "/api/chapters/1", nil)
	req.SetPathValue("n", "1")
	rec := httptest.NewRecorder()
	serveChapter(eng, rec, req, false)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body chapterResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Kind != "final" || body.Chapter != 1 || body.Text == "" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestServeChapterDraftReturnsDraftText(t *testing.T) {
	eng := newFakeEngine(t, 1)
	writeFile(t, eng.dir, "drafts/01.draft.md", "draft content")

	req := httptest.NewRequest(http.MethodGet, "/api/chapters/1/draft", nil)
	req.SetPathValue("n", "1")
	rec := httptest.NewRecorder()
	serveChapter(eng, rec, req, true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body chapterResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Kind != "draft" || body.Text != "draft content" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestServeChapterMissingReturns404(t *testing.T) {
	eng := newFakeEngine(t, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/chapters/1", nil)
	req.SetPathValue("n", "1")
	rec := httptest.NewRecorder()
	serveChapter(eng, rec, req, false)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestServeChapterInvalidNumberReturns400(t *testing.T) {
	eng := newFakeEngine(t, 1)

	for _, n := range []string{"abc", "0", "-1"} {
		req := httptest.NewRequest(http.MethodGet, "/api/chapters/"+n, nil)
		req.SetPathValue("n", n)
		rec := httptest.NewRecorder()
		serveChapter(eng, rec, req, false)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("n=%s status = %d, want 400", n, rec.Code)
		}
	}
}

func TestServeChapterOutOfRangeReturns404(t *testing.T) {
	eng := newFakeEngine(t, 2)

	req := httptest.NewRequest(http.MethodGet, "/api/chapters/3", nil)
	req.SetPathValue("n", "3")
	rec := httptest.NewRecorder()
	serveChapter(eng, rec, req, false)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestServeChapterWhitespaceOnlyReturns404(t *testing.T) {
	eng := newFakeEngine(t, 1)
	writeFile(t, eng.dir, "chapters/01.md", "   \n\t  ")

	req := httptest.NewRequest(http.MethodGet, "/api/chapters/1", nil)
	req.SetPathValue("n", "1")
	rec := httptest.NewRecorder()
	serveChapter(eng, rec, req, false)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestServeChapterDraftOutOfRangeReturns404(t *testing.T) {
	eng := newFakeEngine(t, 2)

	req := httptest.NewRequest(http.MethodGet, "/api/chapters/3/draft", nil)
	req.SetPathValue("n", "3")
	rec := httptest.NewRecorder()
	serveChapter(eng, rec, req, true)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestServeChapterDraftMissingReturns404(t *testing.T) {
	eng := newFakeEngine(t, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/chapters/1/draft", nil)
	req.SetPathValue("n", "1")
	rec := httptest.NewRecorder()
	serveChapter(eng, rec, req, true)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestServeChapterDraftWhitespaceOnlyReturns404(t *testing.T) {
	eng := newFakeEngine(t, 1)
	writeFile(t, eng.dir, "drafts/01.draft.md", "   \n\t  ")

	req := httptest.NewRequest(http.MethodGet, "/api/chapters/1/draft", nil)
	req.SetPathValue("n", "1")
	rec := httptest.NewRecorder()
	serveChapter(eng, rec, req, true)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestServeChapterTooLargeReturns413(t *testing.T) {
	eng := newFakeEngine(t, 1)
	writeFile(t, eng.dir, "chapters/01.md", strings.Repeat("a", MaxChapterBytes+1))

	req := httptest.NewRequest(http.MethodGet, "/api/chapters/1", nil)
	req.SetPathValue("n", "1")
	rec := httptest.NewRecorder()
	serveChapter(eng, rec, req, false)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestServeChapterDraftTooLargeReturns413(t *testing.T) {
	eng := newFakeEngine(t, 1)
	writeFile(t, eng.dir, "drafts/01.draft.md", strings.Repeat("b", MaxChapterBytes+1))

	req := httptest.NewRequest(http.MethodGet, "/api/chapters/1/draft", nil)
	req.SetPathValue("n", "1")
	rec := httptest.NewRecorder()
	serveChapter(eng, rec, req, true)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestServeChapterIgnoresSnapshotTotalChapters(t *testing.T) {
	eng := newFakeEngine(t, 0)
	writeFile(t, eng.dir, "chapters/01.md", "# Chương 1\n\nNội dung.")

	req := httptest.NewRequest(http.MethodGet, "/api/chapters/1", nil)
	req.SetPathValue("n", "1")
	rec := httptest.NewRecorder()
	serveChapter(eng, rec, req, false)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body chapterResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Text != "# Chương 1\n\nNội dung." {
		t.Fatalf("unexpected text: %q", body.Text)
	}
}

func TestServeOutlineCorruptJSONReturns500(t *testing.T) {
	eng := newFakeEngine(t, 0)
	writeFile(t, eng.dir, "outline.json", `{invalid json`)

	req := httptest.NewRequest(http.MethodGet, "/api/outline", nil)
	rec := httptest.NewRecorder()
	serveOutline(eng, rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestContentRoutesSmoke(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "chapters/01.md", "chapter one")
	writeFile(t, dir, "drafts/01.draft.md", "draft one")

	srv := &server{store: store.NewStore(dir)}
	ts := httptest.NewServer(srv.mux())
	defer ts.Close()

	for _, path := range []string{"/api/chapters/1", "/api/chapters/1/draft", "/api/outline", "/api/world", "/api/characters"} {
		res, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", path, res.StatusCode)
		}
	}
}

func TestServeOutlineReturnsData(t *testing.T) {
	eng := newFakeEngine(t, 1)
	writeFile(t, eng.dir, "premise.md", "premise text")
	writeFile(t, eng.dir, "outline.json", `[{"chapter":1,"title":"One"}]`)
	writeFile(t, eng.dir, "layered_outline.json", `[]`)
	writeFile(t, eng.dir, "meta/compass.json", `{}`)

	req := httptest.NewRequest(http.MethodGet, "/api/outline", nil)
	rec := httptest.NewRecorder()
	serveOutline(eng, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body outlineResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Premise != "premise text" {
		t.Fatalf("premise = %q, want %q", body.Premise, "premise text")
	}
}

func TestServeWorldReturnsData(t *testing.T) {
	eng := newFakeEngine(t, 1)
	writeFile(t, eng.dir, "world_rules.json", `[{"rule":"r1"}]`)
	writeFile(t, eng.dir, "timeline.json", `[]`)
	writeFile(t, eng.dir, "meta/compass.json", `{}`)

	req := httptest.NewRequest(http.MethodGet, "/api/world", nil)
	rec := httptest.NewRecorder()
	serveWorld(eng, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body worldResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Rules == nil {
		t.Fatal("rules nil")
	}
}

func TestServeCharactersReturnsData(t *testing.T) {
	eng := newFakeEngine(t, 1)
	writeFile(t, eng.dir, "characters.json", `[{"name":"Alice"}]`)
	writeFile(t, eng.dir, "meta/cast_ledger.json", `[{"name":"Bob"}]`)

	req := httptest.NewRequest(http.MethodGet, "/api/characters", nil)
	rec := httptest.NewRecorder()
	serveCharacters(eng, rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body charactersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Characters == nil || body.Supporting == nil {
		t.Fatalf("unexpected body: %+v", body)
	}
}
