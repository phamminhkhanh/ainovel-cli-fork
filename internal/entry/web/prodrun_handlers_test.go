package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/bootstrap"
)

func newTestServerForProdruns(t *testing.T) (*server, string) {
	t.Helper()
	repoRoot := t.TempDir()
	jobsDir := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "spike.md"), []byte("# prompt"), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr, err := newProdRunManager(jobsDir, os.Args[0], repoRoot, t.TempDir(), bootstrap.Config{})
	if err != nil {
		t.Fatal(err)
	}
	s := &server{repoRoot: repoRoot, prodRunManager: mgr}
	return s, repoRoot
}

func TestHandleProfilesList(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	req := httptest.NewRequest(http.MethodGet, "/api/profiles", nil)
	rec := httptest.NewRecorder()
	s.handleProfilesList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var profiles []profileItem
	if err := json.Unmarshal(rec.Body.Bytes(), &profiles); err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].Name != "spike.md" {
		t.Fatalf("unexpected profiles: %+v", profiles)
	}
}

func TestHandleProdRunCreateValidation(t *testing.T) {
	s, _ := newTestServerForProdruns(t)

	cases := []struct {
		name string
		body string
		want int
	}{
		{"missing name", `{"profile":"profiles/spike.md"}`, http.StatusBadRequest},
		{"missing profile", `{"name":"x"}`, http.StatusBadRequest},
		{"path traversal", `{"name":"x","profile":"../etc/passwd"}`, http.StatusBadRequest},
		{"outside profiles", `{"name":"x","profile":"other/file.md"}`, http.StatusBadRequest},
		{"valid", `{"name":"x","profile":"profiles/spike.md","targetChapters":2}`, http.StatusCreated},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/prodruns", bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			s.handleProdRunCreate(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("expected %d, got %d: %s", tc.want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHandleProdRunListAndGet(t *testing.T) {
	s, _ := newTestServerForProdruns(t)

	// Create a run.
	body := `{"name":"listtest","profile":"profiles/spike.md","targetChapters":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/prodruns", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleProdRunCreate(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", rec.Code, rec.Body.String())
	}
	var created ProdRun
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	// List.
	req = httptest.NewRequest(http.MethodGet, "/api/prodruns", nil)
	rec = httptest.NewRecorder()
	s.handleProdRunsList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list failed: %d %s", rec.Code, rec.Body.String())
	}
	var list []*ProdRun
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 run, got %d", len(list))
	}

	// Get.
	req = httptest.NewRequest(http.MethodGet, "/api/prodruns/"+created.ID, nil)
	rec = httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get failed: %d %s", rec.Code, rec.Body.String())
	}

	// Missing run.
	req = httptest.NewRequest(http.MethodGet, "/api/prodruns/does-not-exist", nil)
	rec = httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleProdRunLogEmpty(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	body := `{"name":"logtest","profile":"profiles/spike.md"}`
	req := httptest.NewRequest(http.MethodPost, "/api/prodruns", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleProdRunCreate(rec, req)
	var created ProdRun
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	req = httptest.NewRequest(http.MethodGet, "/api/prodruns/"+created.ID+"/log?lines=10", nil)
	rec = httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("expected text/plain, got %s", rec.Header().Get("Content-Type"))
	}
}
