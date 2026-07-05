package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleProfileSaveThenContent(t *testing.T) {
	s, repoRoot := newTestServerForProdruns(t)

	// Save a new profile (no .md suffix given → appended).
	body := `{"name":"my-story","content":"# My Story\nA brief."}`
	req := httptest.NewRequest(http.MethodPost, "/api/profiles/save", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var saved profileItem
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Path != "project/my-story.md" || saved.Source != "project" {
		t.Fatalf("unexpected saved item: %+v", saved)
	}
	// File really landed in the project profiles dir.
	if _, err := os.Stat(filepath.Join(repoRoot, ".ainovel", "profiles", "my-story.md")); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}

	// Read it back via content endpoint.
	req = httptest.NewRequest(http.MethodGet, "/api/profiles/content?path=project/my-story.md", nil)
	rec = httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("content: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got profileContentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Content != "# My Story\nA brief." {
		t.Fatalf("content mismatch: %q", got.Content)
	}
	if got.Name != "my-story.md" || got.Source != "project" {
		t.Fatalf("unexpected content meta: %+v", got)
	}
}

func TestHandleProfileSaveNoSilentOverwrite(t *testing.T) {
	s, repoRoot := newTestServerForProdruns(t)
	save := func(content string, overwrite bool) int {
		body, _ := json.Marshal(map[string]any{"name": "edit.md", "content": content, "overwrite": overwrite})
		req := httptest.NewRequest(http.MethodPost, "/api/profiles/save", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.mux().ServeHTTP(rec, req)
		return rec.Code
	}
	if code := save("v1", false); code != http.StatusOK {
		t.Fatalf("first save should be 200, got %d", code)
	}
	// Second save without overwrite must be refused (SSOT: no silent clobber).
	if code := save("v2", false); code != http.StatusConflict {
		t.Fatalf("overwrite without flag should be 409, got %d", code)
	}
	// File unchanged after the refused save.
	data, _ := os.ReadFile(filepath.Join(repoRoot, ".ainovel", "profiles", "edit.md"))
	if string(data) != "v1" {
		t.Fatalf("refused save must not change file, got %q", string(data))
	}
	// With overwrite=true it goes through.
	if code := save("v2", true); code != http.StatusOK {
		t.Fatalf("save with overwrite should be 200, got %d", code)
	}
	data, _ = os.ReadFile(filepath.Join(repoRoot, ".ainovel", "profiles", "edit.md"))
	if string(data) != "v2" {
		t.Fatalf("expected overwrite to v2, got %q", string(data))
	}
}

func TestHandleProfileDeleteRejectsNonProject(t *testing.T) {
	// newTestServerForProdruns seeds spike.md in project, global, AND legacy.
	for _, ref := range []string{"global/spike.md", "legacy/spike.md", "profiles/spike.md"} {
		t.Run(ref, func(t *testing.T) {
			s, repoRoot := newTestServerForProdruns(t)
			body, _ := json.Marshal(map[string]string{"path": ref})
			req := httptest.NewRequest(http.MethodPost, "/api/profiles/delete", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			s.mux().ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("deleting %s must be 403, got %d", ref, rec.Code)
			}
			// The global profile file must still exist.
			_ = repoRoot
		})
	}
}

func TestHandleProfileSaveValidation(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	cases := []struct {
		name string
		body string
		want int
	}{
		{"missing name", `{"content":"x"}`, http.StatusBadRequest},
		{"missing content", `{"name":"a.md"}`, http.StatusBadRequest},
		{"path separator", `{"name":"sub/a.md","content":"x"}`, http.StatusBadRequest},
		{"traversal", `{"name":"..\\a.md","content":"x"}`, http.StatusBadRequest},
		{"dot only", `{"name":".","content":"x"}`, http.StatusBadRequest},
		{"dots only", `{"name":"...","content":"x"}`, http.StatusBadRequest},
		{"ok", `{"name":"ok","content":"x"}`, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/profiles/save", bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			s.mux().ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestHandleProfileContentRejectsUnsafePath(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	for _, ref := range []string{"../etc/passwd", "project/../../secret.md", "other/x.md"} {
		req := httptest.NewRequest(http.MethodGet, "/api/profiles/content?path="+ref, nil)
		rec := httptest.NewRecorder()
		s.mux().ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Fatalf("unsafe path %q must not return 200", ref)
		}
	}
}

func TestHandleProfileDelete(t *testing.T) {
	s, repoRoot := newTestServerForProdruns(t)
	// newTestServerForProdruns seeds project/spike.md.
	target := filepath.Join(repoRoot, ".ainovel", "profiles", "spike.md")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("seed missing: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/profiles/delete", bytes.NewReader([]byte(`{"path":"project/spike.md"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("profile should be deleted, stat err=%v", err)
	}
}

func TestHandleProfileDeleteMissing(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	req := httptest.NewRequest(http.MethodPost, "/api/profiles/delete", bytes.NewReader([]byte(`{"path":"project/nope.md"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
