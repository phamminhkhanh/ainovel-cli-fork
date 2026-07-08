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
	"github.com/voocel/ainovel-cli/internal/domain"
)

func newTestServerForProdruns(t *testing.T) (*server, string) {
	t.Helper()
	homeDir := t.TempDir()
	setTestHome(t, homeDir)
	repoRoot := t.TempDir()
	jobsDir := t.TempDir()
	for _, dir := range []string{
		filepath.Join(repoRoot, ".ainovel", "profiles"),
		filepath.Join(homeDir, ".ainovel", "profiles"),
		filepath.Join(repoRoot, "profiles"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "spike.md"), []byte("# prompt"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mgr, err := newProdRunManager(jobsDir, os.Args[0], repoRoot, t.TempDir(), bootstrap.Config{})
	if err != nil {
		t.Fatal(err)
	}
	s := &server{repoRoot: repoRoot, prodRunManager: mgr}
	return s, repoRoot
}

type prodRunViewResponse struct {
	ID     string    `json:"id"`
	Health runHealth `json:"health"`
}

func requireRunViewHealth(t *testing.T, body []byte) prodRunViewResponse {
	t.Helper()
	var view prodRunViewResponse
	if err := json.Unmarshal(body, &view); err != nil {
		t.Fatal(err)
	}
	if view.ID == "" {
		t.Fatal("run view missing id")
	}
	if len(view.Health.Metrics) == 0 {
		t.Fatalf("run view %q missing health metrics", view.ID)
	}
	return view
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
	if len(profiles) != 3 {
		t.Fatalf("unexpected profiles: %+v", profiles)
	}
	want := []profileItem{
		{Name: "spike.md", Path: "project/spike.md", Source: "project"},
		{Name: "spike.md", Path: "global/spike.md", Source: "global"},
		{Name: "spike.md", Path: "legacy/spike.md", Source: "legacy"},
	}
	for i := range want {
		if profiles[i] != want[i] {
			t.Fatalf("profile[%d] = %+v, want %+v", i, profiles[i], want[i])
		}
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
		{"project traversal", `{"name":"x","profile":"project/../etc/passwd"}`, http.StatusBadRequest},
		{"outside profiles", `{"name":"x","profile":"other/file.md"}`, http.StatusBadRequest},
		{"valid project", `{"name":"x","profile":"project/spike.md","targetChapters":2}`, http.StatusCreated},
		{"valid global", `{"name":"x","profile":"global/spike.md","targetChapters":2}`, http.StatusCreated},
		{"valid legacy", `{"name":"x","profile":"legacy/spike.md","targetChapters":2}`, http.StatusCreated},
		{"valid old legacy", `{"name":"x","profile":"profiles/spike.md","targetChapters":2}`, http.StatusCreated},
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

func TestHandleProdRunCreateContinue(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	hostDir := s.prodRunManager.hostDir
	writeWorkspaceProgress(t, hostDir, []int{1, 2}, domain.PhaseWriting)

	body := `{"kind":"continue_workspace","name":"continue","targetChapters":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/prodruns", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleProdRunCreate(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var created ProdRun
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Kind != prodRunKindContinueWorkspace {
		t.Fatalf("expected continue kind, got %q", created.Kind)
	}
	if created.Profile != "" {
		t.Fatalf("continue run should not require profile, got %q", created.Profile)
	}
	if created.SeededFrom == nil || created.SeededFrom.CompletedChapters != 2 {
		t.Fatalf("unexpected seed metadata: %+v", created.SeededFrom)
	}
}

func TestHandleProdRunCreateContinueRejectsCompleteWorkspace(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	writeWorkspaceProgress(t, s.prodRunManager.hostDir, []int{1, 2}, domain.PhaseComplete)

	body := `{"kind":"continue_workspace","name":"continue","targetChapters":5}`
	req := httptest.NewRequest(http.MethodPost, "/api/prodruns", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleProdRunCreate(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
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
	createdView := requireRunViewHealth(t, rec.Body.Bytes())
	var created ProdRun
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID != createdView.ID {
		t.Fatalf("created id mismatch: run=%q view=%q", created.ID, createdView.ID)
	}

	// List.
	req = httptest.NewRequest(http.MethodGet, "/api/prodruns", nil)
	rec = httptest.NewRecorder()
	s.handleProdRunsList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list failed: %d %s", rec.Code, rec.Body.String())
	}
	var list []prodRunViewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 run, got %d", len(list))
	}
	if list[0].ID != created.ID || len(list[0].Health.Metrics) == 0 {
		t.Fatalf("list view missing id/health: %+v", list[0])
	}

	// Get.
	req = httptest.NewRequest(http.MethodGet, "/api/prodruns/"+created.ID, nil)
	rec = httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get failed: %d %s", rec.Code, rec.Body.String())
	}
	gotView := requireRunViewHealth(t, rec.Body.Bytes())
	if gotView.ID != created.ID {
		t.Fatalf("get id = %q, want %q", gotView.ID, created.ID)
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

// createAwaitingReviewRun is a test helper: creates a fresh_profile run whose
// sandbox output/novel already has a seeded foundation at phase=writing, and
// marks it awaiting_review — the state a real run reaches after the
// Foundation Gate poll check kills the child post-foundation.
func createAwaitingReviewRun(t *testing.T, s *server) *ProdRun {
	t.Helper()
	body := `{"name":"gate","profile":"profiles/spike.md","targetChapters":5}`
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

	runDir := s.prodRunManager.RunDir(created.ID)
	metaDir := filepath.Join(runDir, "output", "novel", "meta")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, "progress.json"), []byte(`{"phase":"writing","completed_chapters":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "output", "novel", "premise.md"), []byte("# Premise\nA story."), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.prodRunManager.store.update(created.ID, func(r *ProdRun) {
		r.Status = prodRunAwaitingReview
		r.StopReason = stopReasonFoundationReady
	}); err != nil {
		t.Fatal(err)
	}
	return s.prodRunManager.Get(created.ID)
}

func TestHandleProdRunFoundationServesOutline(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	run := createAwaitingReviewRun(t, s)

	req := httptest.NewRequest(http.MethodGet, "/api/prodruns/"+run.ID+"/foundation", nil)
	rec := httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out outlineResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Premise, "A story") {
		t.Fatalf("expected premise content, got %q", out.Premise)
	}
}

func TestHandleProdRunFoundationMissingRun(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	req := httptest.NewRequest(http.MethodGet, "/api/prodruns/does-not-exist/foundation", nil)
	rec := httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleProdRunReject(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	run := createAwaitingReviewRun(t, s)

	req := httptest.NewRequest(http.MethodPost, "/api/prodruns/"+run.ID+"/reject", nil)
	rec := httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if s.prodRunManager.Get(run.ID) != nil {
		t.Fatal("expected run to be deleted after reject")
	}
}

func TestHandleProdRunRejectRejectsWrongStatus(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	body := `{"name":"queuedrun","profile":"profiles/spike.md"}`
	req := httptest.NewRequest(http.MethodPost, "/api/prodruns", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleProdRunCreate(rec, req)
	var created ProdRun
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	req = httptest.NewRequest(http.MethodPost, "/api/prodruns/"+created.ID+"/reject", nil)
	rec = httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 rejecting a non-awaiting_review run, got %d", rec.Code)
	}
}

func TestHandleProdRunApproveWrongStatus(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	body := `{"name":"queuedrun2","profile":"profiles/spike.md"}`
	req := httptest.NewRequest(http.MethodPost, "/api/prodruns", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleProdRunCreate(rec, req)
	var created ProdRun
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	req = httptest.NewRequest(http.MethodPost, "/api/prodruns/"+created.ID+"/approve", nil)
	rec = httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 approving a queued run, got %d", rec.Code)
	}
}

func TestHandleProdRunReviseValidation(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	run := createAwaitingReviewRun(t, s)

	// Empty feedback → 400.
	req := httptest.NewRequest(http.MethodPost, "/api/prodruns/"+run.ID+"/revise", bytes.NewReader([]byte(`{"feedback":"  "}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty feedback should be 400, got %d", rec.Code)
	}

	// Revise on a non-awaiting_review run → 409.
	body := `{"name":"q","profile":"profiles/spike.md"}`
	cr := httptest.NewRequest(http.MethodPost, "/api/prodruns", bytes.NewReader([]byte(body)))
	cr.Header.Set("Content-Type", "application/json")
	crec := httptest.NewRecorder()
	s.handleProdRunCreate(crec, cr)
	var queued ProdRun
	_ = json.Unmarshal(crec.Body.Bytes(), &queued)

	req = httptest.NewRequest(http.MethodPost, "/api/prodruns/"+queued.ID+"/revise", bytes.NewReader([]byte(`{"feedback":"x"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	s.mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("revise on queued run should be 409, got %d", rec.Code)
	}
}

func TestHandleProdRunRevealOpensFoundationDir(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	s.addr = "127.0.0.1:8787" // loopback so reveal is allowed
	run := createAwaitingReviewRun(t, s)

	var opened string
	orig := revealOpen
	revealOpen = func(dir string) error { opened = dir; return nil }
	defer func() { revealOpen = orig }()

	req := httptest.NewRequest(http.MethodPost, "/api/prodruns/"+run.ID+"/reveal", nil)
	req.SetPathValue("id", run.ID)
	rec := httptest.NewRecorder()
	s.handleProdRunReveal(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(filepath.ToSlash(opened), "output/novel") {
		t.Fatalf("reveal must open the run's output/novel dir, got %q", opened)
	}
}

func TestHandleProdRunRevealBlockedOnPublicBind(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	s.addr = "0.0.0.0:8787" // non-loopback
	run := createAwaitingReviewRun(t, s)

	called := false
	orig := revealOpen
	revealOpen = func(string) error { called = true; return nil }
	defer func() { revealOpen = orig }()

	req := httptest.NewRequest(http.MethodPost, "/api/prodruns/"+run.ID+"/reveal", nil)
	req.SetPathValue("id", run.ID)
	rec := httptest.NewRecorder()
	s.handleProdRunReveal(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("public bind must 403, got %d", rec.Code)
	}
	if called {
		t.Fatal("public bind must not open file manager")
	}
}
