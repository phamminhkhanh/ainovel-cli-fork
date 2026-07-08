package web

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/voocel/ainovel-cli/internal/store"
)

// handleProfilesList returns markdown production profiles from project, global,
// and legacy locations.
func (s *server) handleProfilesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	profiles, err := s.listProfiles()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

type profileItem struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Source string `json:"source"`
}

func (s *server) listProfiles() ([]profileItem, error) {
	return listProfileItems(s.repoRoot)
}

func (s *server) validateProfilePath(path string) error {
	if path == "" {
		return fmt.Errorf("profile is required")
	}
	_, err := resolveExistingProfilePath(path, s.repoRoot)
	return err
}

// handleProdRunsList returns all production runs.
func (s *server) handleProdRunsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	writeJSON(w, http.StatusOK, newProdRunViews(s.prodRunManager.List()))
}

func writeProdRunView(w http.ResponseWriter, status int, run *ProdRun) {
	if run == nil {
		writeErr(w, http.StatusNotFound, fmt.Errorf("run not found"))
		return
	}
	writeJSON(w, status, newProdRunView(run))
}

// handleProdRunCreate queues a new production run.
func (s *server) handleProdRunCreate(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var body struct {
		Kind           string  `json:"kind"`
		Name           string  `json:"name"`
		Profile        string  `json:"profile"`
		Model          string  `json:"model"`
		Provider       string  `json:"provider"`
		TargetChapters int     `json:"targetChapters"`
		BudgetUSD      float64 `json:"budgetUsd"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("name is required"))
		return
	}
	kind := strings.TrimSpace(body.Kind)
	if kind == "" {
		kind = prodRunKindFreshProfile
	}
	var run *ProdRun
	var err error
	switch kind {
	case prodRunKindFreshProfile:
		if err := s.validateProfilePath(body.Profile); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		run, err = s.prodRunManager.Create(body.Name, body.Profile, body.Model, body.Provider, body.TargetChapters, body.BudgetUSD)
	case prodRunKindContinueWorkspace:
		if s.hostIsRunning() {
			writeErr(w, http.StatusConflict, errSeedHostRunning)
			return
		}
		run, err = s.prodRunManager.CreateContinue(body.Name, body.Model, body.Provider, body.TargetChapters, body.BudgetUSD)
	default:
		writeErr(w, http.StatusBadRequest, fmt.Errorf("unsupported run kind %q", kind))
		return
	}
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errSeedNoWorkspace) || errors.Is(err, errSeedWorkspaceComplete) || strings.Contains(err.Error(), "target chapters") {
			status = http.StatusConflict
		}
		writeErr(w, status, err)
		return
	}
	writeProdRunView(w, http.StatusCreated, run)
}

// handleProdRunGet returns a single production run.
func (s *server) handleProdRunGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	id := r.PathValue("id")
	run := s.prodRunManager.Get(id)
	if run == nil {
		writeErr(w, http.StatusNotFound, fmt.Errorf("run not found"))
		return
	}
	writeProdRunView(w, http.StatusOK, run)
}

// handleProdRunStart starts the child headless process for a run.
func (s *server) handleProdRunStart(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	id := r.PathValue("id")
	run := s.prodRunManager.Get(id)
	if run != nil && run.kind() == prodRunKindContinueWorkspace && s.hostIsRunning() {
		writeErr(w, http.StatusConflict, errSeedHostRunning)
		return
	}
	if err := s.prodRunManager.Start(id); err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	writeProdRunView(w, http.StatusOK, s.prodRunManager.Get(id))
}

// handleProdRunStop hard-kills the child process for a run.
func (s *server) handleProdRunStop(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	id := r.PathValue("id")
	if err := s.prodRunManager.Stop(id); err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	writeProdRunView(w, http.StatusOK, s.prodRunManager.Get(id))
}

// handleProdRunDelete removes a finished run and its run directory.
func (s *server) handleProdRunDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	id := r.PathValue("id")
	if err := s.prodRunManager.Delete(id); err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, errDeleteRunNotFound):
			status = http.StatusNotFound
		case errors.Is(err, errDeleteRunActive):
			status = http.StatusConflict
		}
		writeErr(w, status, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleProdRunSync copies a finished production run's output into the main
// host workspace so the interactive UI can see it.
func (s *server) handleProdRunSync(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	if s.hostIsRunning() {
		writeErr(w, http.StatusConflict, fmt.Errorf("host is running; stop before syncing"))
		return
	}
	id := r.PathValue("id")
	var body struct {
		Force bool `json:"force"`
	}
	// Body is optional; ignore parse errors and treat as no-force.
	_ = decodeJSON(r, &body)
	res, err := s.prodRunManager.Sync(id, syncOptions{Force: body.Force})
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, errSyncRunNotFound):
			status = http.StatusNotFound
		case errors.Is(err, errSyncRunActive), errors.Is(err, errSyncHostHasProgress), errors.Is(err, errSyncWorkspaceDiverged):
			status = http.StatusConflict
		case errors.Is(err, errSyncRunNoOutput):
			status = http.StatusBadRequest
		}
		writeErr(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleProdRunLog returns the tail of the run log as text/plain.
func (s *server) handleProdRunLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	id := r.PathValue("id")
	const maxLines = 1000
	lines := 50
	if v := r.URL.Query().Get("lines"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > maxLines {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid lines parameter (must be 1-%d)", maxLines))
			return
		}
		lines = n
	}
	logLines, err := s.prodRunManager.ReadLogTail(id, lines)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(strings.Join(logLines, "\n")))
}

// handleProdRunExport creates the TXT export and returns its path.
func (s *server) handleProdRunExport(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	id := r.PathValue("id")
	var body struct {
		Format string `json:"format"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if body.Format != "txt" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("unsupported export format %q (only txt is supported)", body.Format))
		return
	}
	path, err := s.prodRunManager.ExportTXT(id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errExportRunNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, errExportNoChaptersDir) || errors.Is(err, errExportNoChapters) {
			status = http.StatusBadRequest
		}
		writeErr(w, status, err)
		return
	}
	rel, _ := filepath.Rel(s.prodRunManager.RunDir(id), path)
	writeJSON(w, http.StatusOK, map[string]any{"path": rel, "abs": path})
}

// handleProdRunExportDownload regenerates and serves the TXT export as a download.
func (s *server) handleProdRunExportDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	id := r.PathValue("id")
	path, err := s.prodRunManager.ExportTXT(id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errExportRunNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, errExportNoChaptersDir) || errors.Is(err, errExportNoChapters) {
			status = http.StatusBadRequest
		}
		writeErr(w, status, err)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(path)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *server) hostIsRunning() bool {
	if s == nil || s.eng == nil {
		return false
	}
	return s.eng.Snapshot().IsRunning
}

// runStoreEngine adapts a run's seeded output/novel dir to the contentEngine
// interface so the existing outline/world/characters handlers in content.go
// can be reused as-is for the Foundation Gate preview.
type runStoreEngine struct{ st *store.Store }

func (e runStoreEngine) Store() *store.Store { return e.st }

func (s *server) runOutputStore(id string) (*store.Store, error) {
	r := s.prodRunManager.Get(id)
	if r == nil {
		return nil, fmt.Errorf("run not found")
	}
	dir := filepath.Join(s.prodRunManager.RunDir(id), "output", "novel")
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("run has no output yet")
	}
	return store.NewStore(dir), nil
}

// handleProdRunFoundation returns premise/outline/world/characters/compass
// for a run awaiting foundation review (Foundation Gate). It
// reuses the read-only content handlers against the run's own sandboxed
// output/novel dir instead of the main host workspace.
func (s *server) handleProdRunFoundation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	id := r.PathValue("id")
	st, err := s.runOutputStore(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	eng := runStoreEngine{st: st}
	switch r.URL.Query().Get("section") {
	case "world":
		serveWorld(eng, w, r)
	case "characters":
		serveCharacters(eng, w, r)
	default:
		serveOutline(eng, w, r)
	}
}

// handleProdRunApprove accepts the reviewed foundation of a run in
// awaiting_review and restarts the SAME run dir (its own output/novel already
// has the seeded foundation at phase=writing), so the Writer/Editor loop
// resumes natively via headless Resume() instead of requiring a live Host to
// Steer into. See docs/de-xuat-cai-tien-chat-luong.md §1 and
// docs/journals/260705-foundation-gate.md.
func (s *server) handleProdRunApprove(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	id := r.PathValue("id")
	run := s.prodRunManager.Get(id)
	if run == nil {
		writeErr(w, http.StatusNotFound, fmt.Errorf("run not found"))
		return
	}
	if run.Status != prodRunAwaitingReview {
		writeErr(w, http.StatusConflict, fmt.Errorf("run %q is not awaiting review (status=%s)", id, run.Status))
		return
	}
	next, err := s.prodRunManager.ApproveFoundation(id)
	if err != nil {
		writeErr(w, http.StatusConflict, err)
		return
	}
	writeProdRunView(w, http.StatusOK, next)
}

// handleProdRunReveal opens a run's seeded foundation dir in the OS file
// manager so the user can hand-edit premise/outline/characters before Approve.
// Loopback-only (same guard as handleReveal); the dir is derived server-side
// from the validated run id — no client-supplied path.
func (s *server) handleProdRunReveal(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	if !isLoopbackHostPort(s.addr) {
		writeErr(w, http.StatusForbidden, fmt.Errorf("m\u1edf th\u01b0 m\u1ee5c ch\u1ec9 kh\u1ea3 d\u1ee5ng khi bind loopback"))
		return
	}
	id := r.PathValue("id")
	run := s.prodRunManager.Get(id)
	if run == nil {
		writeErr(w, http.StatusNotFound, fmt.Errorf("run not found"))
		return
	}
	dir := filepath.Join(s.prodRunManager.RunDir(id), "output", "novel")
	if _, err := os.Stat(dir); err != nil {
		writeErr(w, http.StatusNotFound, fmt.Errorf("run ch\u01b0a c\u00f3 th\u01b0 m\u1ee5c n\u1ec1n m\u00f3ng"))
		return
	}
	if err := revealOpen(dir); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "dir": dir})
}

// handleProdRunRevise regenerates an awaiting_review run's foundation with a
// user steering note appended to the profile prompt. See ReviseFoundation.
func (s *server) handleProdRunRevise(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	id := r.PathValue("id")
	var body struct {
		Feedback string `json:"feedback"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(body.Feedback) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("feedback is required"))
		return
	}
	next, err := s.prodRunManager.ReviseFoundation(id, body.Feedback)
	if err != nil {
		status := http.StatusConflict
		if errors.Is(err, errDeleteRunNotFound) {
			status = http.StatusNotFound
		}
		writeErr(w, status, err)
		return
	}
	writeProdRunView(w, http.StatusOK, next)
}

// handleProdRunReject discards a run awaiting foundation review without
// spending any Writer/Editor tokens on it.
func (s *server) handleProdRunReject(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	id := r.PathValue("id")
	run := s.prodRunManager.Get(id)
	if run == nil {
		writeErr(w, http.StatusNotFound, fmt.Errorf("run not found"))
		return
	}
	if run.Status != prodRunAwaitingReview {
		writeErr(w, http.StatusConflict, fmt.Errorf("run %q is not awaiting review (status=%s)", id, run.Status))
		return
	}
	if err := s.prodRunManager.Delete(id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
