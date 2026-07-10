package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Sentinel errors for production-run operations.
var (
	errDeleteRunNotFound     = errors.New("run not found")
	errDeleteRunActive       = errors.New("cannot delete an active run")
	errSeedNoWorkspace       = errors.New("workspace has no recoverable progress")
	errSeedHostRunning       = errors.New("host is running; stop before seeding a continue run")
	errSeedWorkspaceChanged  = errors.New("workspace changed since the continue run was created")
	errSeedWorkspaceComplete = errors.New("workspace is complete and cannot be resumed")
	// errAnotherRunActive is returned by start() when the single-run limit is
	// hit. Callers (approve/revise retry) match on this typed error rather than
	// a fragile substring so behavior does not silently break if the message
	// wording changes.
	errAnotherRunActive = errors.New("another production run is already running")
	// errReapTimeout is returned by waitReaped when the just-killed child has
	// not exited within the budget. Approve/Revise revert/abort on this so the
	// user can retry instead of being stranded in queued.
	errReapTimeout = errors.New("previous child process is still stopping")
)

// Production-run lifecycle statuses.
const (
	prodRunQueued  = "queued"
	prodRunRunning = "running"
	prodRunPaused  = "paused"
	// prodRunAwaitingReview: fresh_profile run auto-stopped right after the
	// foundation (premise/outline/world/characters) was saved and Phase just
	// flipped to writing, before any chapter draft starts. The child process
	// has already exited; there is no live Host to resume in-place. Approve
	// restarts the same run dir (native headless Resume(), see
	// FoundationApproved); Reject deletes the run. See
	// docs/de-xuat-cai-tien-chat-luong.md §1 and docs/journals/260705-foundation-gate.md.
	prodRunAwaitingReview = "awaiting_review"
	prodRunCompleted      = "completed"
	prodRunFailed         = "failed"
	prodRunCancelled      = "cancelled"
)

// Production-run kinds. Empty kind is treated as fresh_profile for legacy jobs.
const (
	prodRunKindFreshProfile      = "fresh_profile"
	prodRunKindContinueWorkspace = "continue_workspace"
)

// prodRunReapTimeout is the budget Approve/Revise wait for a just-killed child
// to be reaped before restarting. SIGKILL/TerminateProcess reaps in tens of ms,
// so 5s is a generous ceiling; a timeout means the OS is wedged and the caller
// reverts/aborts rather than stranding the run in queued.
const prodRunReapTimeout = 5 * time.Second

// Stop reasons written to ProdRun.StopReason.
const (
	stopReasonCompleted       = "completed"
	stopReasonTargetReached   = "target_reached"
	stopReasonCancelled       = "cancelled"
	stopReasonError           = "error"
	stopReasonUnclean         = "unclean_shutdown"
	stopReasonFoundationReady = "foundation_ready"
)

// defaultProdRunBudgetUSD is the fallback cost cap when the user/global config
// does not specify one. It is a safety guard, not a product promise.
const defaultProdRunBudgetUSD = 5.0

// ProdRun is a queued / running / finished headless novel-generation job.
type ProdRun struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Kind             string    `json:"kind,omitempty"`
	Profile          string    `json:"profile"` // profile ref, e.g. "project/foo.md", "global/foo.md", or legacy "profiles/foo.md"
	Model            string    `json:"model,omitempty"`
	Provider         string    `json:"provider,omitempty"`
	TargetChapters   int       `json:"targetChapters"`
	BudgetUSD        float64   `json:"budgetUsd"`
	Status           string    `json:"status"`
	StopReason       string    `json:"stopReason,omitempty"`
	ChildPID         int       `json:"childPid,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	StartedAt        time.Time `json:"startedAt,omitempty"`
	StoppedAt        time.Time `json:"stoppedAt,omitempty"`
	Chapters         int       `json:"chapters"`
	Reviews          int       `json:"reviews"`
	Rewrites         int       `json:"rewrites"`
	CostUSD          float64   `json:"costUsd"`
	LogPath          string    `json:"logPath,omitempty"`
	PossiblyOrphaned bool      `json:"possiblyOrphaned"`
	SeededFrom       *SeedMeta `json:"seededFrom,omitempty"`
	// PersistError ghi lỗi lần persist jobs.json gần nhất (thường do Windows
	// file lock khi IDE/editor mở jobs.json). Rỗng khi persist OK. UI health
	// strip dùng field này để hiện chip đỏ + toast báo user tắt editor.
	PersistError   string    `json:"persistError,omitempty"`
	PersistErrorAt time.Time `json:"persistErrorAt,omitempty"`
	// FoundationApproved is set once the user approves an awaiting_review run
	// (Foundation Gate). Without this, poll()'s foundation-ready
	// check would re-fire on every restart: right after approve-resume,
	// progress.json still reports phase=writing with 0 completed chapters for
	// as long as the Writer takes to draft+commit chapter 1 (routinely longer
	// than one poll tick), so the gate would kill the run again before it ever
	// gets to write. Confirmed by adversarial probe during code review.
	FoundationApproved bool `json:"foundationApproved,omitempty"`
	// RevisionNote is user feedback appended to the profile prompt when a run
	// is created via "revise" (Foundation Gate). It steers the Architect to
	// regenerate the foundation differently. Empty for normal runs.
	RevisionNote string `json:"revisionNote,omitempty"`
}

// SeedMeta captures the source workspace state for a continue_workspace run.
type SeedMeta struct {
	HostDir           string    `json:"hostDir"`
	CompletedChapters int       `json:"completedChapters"`
	Fingerprint       string    `json:"fingerprint"`
	CapturedAt        time.Time `json:"capturedAt"`
	SeededAt          time.Time `json:"seededAt,omitempty"`
}

func (r *ProdRun) kind() string {
	if r == nil || r.Kind == "" {
		return prodRunKindFreshProfile
	}
	return r.Kind
}

// Runtime returns the elapsed runtime for a started run.
func (r *ProdRun) Runtime() time.Duration {
	start := r.StartedAt
	if start.IsZero() {
		return 0
	}
	stop := r.StoppedAt
	if stop.IsZero() {
		stop = time.Now()
	}
	return stop.Sub(start)
}

// prodRunStore persists the production-run list to a single JSON file.
type prodRunStore struct {
	mu      sync.Mutex
	path    string
	jobsDir string
	runs    map[string]*ProdRun
	nextID  int
	// persistFailForTest, khi != nil, làm saveLocked trả error này thay vì persist
	// thật. Chỉ để test inject (unexported, nil ở production). Dùng cho regression
	// test P0: verify killProcess vẫn chạy khi persist fail.
	persistFailForTest error
}

// newProdRunStore creates or loads the store at jobsDir/jobs.json.
func newProdRunStore(jobsDir string) (*prodRunStore, error) {
	ps := &prodRunStore{
		path:    filepath.Join(jobsDir, "jobs.json"),
		jobsDir: jobsDir,
		runs:    make(map[string]*ProdRun),
		nextID:  1,
	}
	if err := ps.load(); err != nil {
		return nil, err
	}
	return ps, nil
}

// load reads the JSON store and marks any previously-running runs as unclean.
// It is safe to call repeatedly but is only called once at construction.
func (ps *prodRunStore) load() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	data, err := os.ReadFile(ps.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read prodrun store: %w", err)
	}

	var list []*ProdRun
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("parse prodrun store: %w", err)
	}

	max := 0
	for _, r := range list {
		ps.runs[r.ID] = r
		if n, ok := parseRunSeq(r.ID); ok && n > max {
			max = n
		}
		if r.Status == prodRunRunning || r.Status == prodRunPaused {
			r.Status = prodRunFailed
			r.StopReason = stopReasonUnclean
			r.PossiblyOrphaned = true
			r.StoppedAt = time.Now()
		}
		// Restart = fresh start: clear PersistError từ session cũ. Lỗi file
		// lock có thể đã hết (editor đóng), và run terminal không còn poll để
		// tự clear. Nếu lock vẫn còn, poll tiếp theo sẽ set lại PersistError.
		r.PersistError = ""
		r.PersistErrorAt = time.Time{}
	}
	ps.nextID = max + 1
	// Persist the unclean-shutdown recovery so the UI sees it after restart.
	return ps.saveLocked()
}

func parseRunSeq(id string) (int, bool) {
	prefix := "run-"
	if !strings.HasPrefix(id, prefix) {
		return 0, false
	}
	n, err := strconv.Atoi(id[len(prefix):])
	if err != nil {
		return 0, false
	}
	return n, true
}

func (ps *prodRunStore) save() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.saveLocked()
}

func (ps *prodRunStore) saveLocked() error {
	if ps.persistFailForTest != nil {
		return ps.persistFailForTest
	}
	if err := os.MkdirAll(filepath.Dir(ps.path), 0o755); err != nil {
		return fmt.Errorf("create jobs dir: %w", err)
	}
	list := ps.listLocked()
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal prodrun store: %w", err)
	}
	tmp := ps.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write prodrun store tmp: %w", err)
	}
	// Rename có retry: Windows lock khi IDE/editor mở jobs.json (Access is denied).
	// safeWriteFile (prodrun_runner.go) đã dùng cùng pattern cho workspace files;
	// jobs.json dễ bị lock nhất nên cũng cần retry. Linux/Mac rename atomic,
	// retry thành công lần đầu → không hại cross-platform.
	var lastErr error
	for i := 0; i < 5; i++ {
		if lastErr = os.Rename(tmp, ps.path); lastErr == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = os.Remove(tmp)
	return lastErr
}

type prodRunCreateOptions struct {
	Kind           string
	Name           string
	Profile        string
	Model          string
	Provider       string
	TargetChapters int
	BudgetUSD      float64
	SeededFrom     *SeedMeta
	Chapters       int
	RevisionNote   string
}

func (ps *prodRunStore) create(name, profile, model, provider string, targetChapters int, budgetUSD float64) (*ProdRun, error) {
	return ps.createWithOptions(prodRunCreateOptions{
		Kind:           prodRunKindFreshProfile,
		Name:           name,
		Profile:        profile,
		Model:          model,
		Provider:       provider,
		TargetChapters: targetChapters,
		BudgetUSD:      budgetUSD,
	})
}

func (ps *prodRunStore) createWithOptions(opts prodRunCreateOptions) (*ProdRun, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if opts.Kind == "" {
		opts.Kind = prodRunKindFreshProfile
	}
	if opts.TargetChapters <= 0 {
		opts.TargetChapters = 30
	}
	if opts.BudgetUSD <= 0 {
		opts.BudgetUSD = defaultProdRunBudgetUSD
	}

	r := &ProdRun{
		ID:             fmt.Sprintf("run-%03d", ps.nextID),
		Name:           strings.TrimSpace(opts.Name),
		Kind:           opts.Kind,
		Profile:        opts.Profile,
		Model:          opts.Model,
		Provider:       opts.Provider,
		TargetChapters: opts.TargetChapters,
		BudgetUSD:      opts.BudgetUSD,
		Status:         prodRunQueued,
		CreatedAt:      time.Now(),
		Chapters:       opts.Chapters,
		SeededFrom:     opts.SeededFrom,
		RevisionNote:   opts.RevisionNote,
	}
	ps.nextID++
	ps.runs[r.ID] = r
	if err := ps.saveLocked(); err != nil {
		return nil, err
	}
	cp := *r
	return &cp, nil
}

func (ps *prodRunStore) get(id string) *ProdRun {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	r := ps.runs[id]
	if r == nil {
		return nil
	}
	cp := *r
	return &cp
}

func (ps *prodRunStore) list() []*ProdRun {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.listLocked()
}

func (ps *prodRunStore) listLocked() []*ProdRun {
	list := make([]*ProdRun, 0, len(ps.runs))
	for _, r := range ps.runs {
		cp := *r
		list = append(list, &cp)
	}
	// Deterministic order: by CreatedAt, tie-broken by ID. Tiebreak is required
	// because `list` is built from a map (random iteration order) — back-to-back
	// creates can share a CreatedAt (coarse clock on Windows), and plain
	// sort.Slice is not stable, so ties would surface in random order. IDs are
	// sequential + unique (run-001, run-002…) so they fully disambiguate.
	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].ID < list[j].ID
		}
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})
	return list
}

// update calls fn with the run locked, persists the store, and returns a snapshot.
// The underlying run is mutated even if persistence fails, so the in-memory state
// does not get stuck. Khi saveLocked fail, PersistError/PersistErrorAt được ghi
// vào run (in-memory) để UI health strip báo user; persist lại best-effort (không
// đệ quy vô tận: nếu lại fail thì bỏ qua, lỗi cũ vẫn còn trong memory).
func (ps *prodRunStore) update(id string, fn func(*ProdRun)) (*ProdRun, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	r, ok := ps.runs[id]
	if !ok {
		return nil, fmt.Errorf("run %q not found", id)
	}
	fn(r)
	if err := ps.saveLocked(); err != nil {
		r.PersistError = err.Error()
		r.PersistErrorAt = time.Now()
		_ = ps.saveLocked() // best-effort ghi luôn lỗi; lại fail thì thôi
		cp := *r
		return &cp, err
	}
	r.PersistError = ""
	r.PersistErrorAt = time.Time{}
	cp := *r
	return &cp, nil
}

// delete removes a terminal run from the store and optionally its run directory.
func (ps *prodRunStore) delete(id string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if id == "" {
		return fmt.Errorf("run id is empty")
	}
	if _, ok := ps.runs[id]; !ok {
		return errDeleteRunNotFound
	}
	delete(ps.runs, id)
	if err := ps.saveLocked(); err != nil {
		return err
	}
	runDir := ps.runDirLocked(id)
	if runDir == "" || runDir == ps.jobsDir || runDir == filepath.Dir(ps.jobsDir) {
		return fmt.Errorf("invalid run directory %q", runDir)
	}
	return os.RemoveAll(runDir)
}

// runDirLocked returns the per-run working directory; caller must hold ps.mu.
func (ps *prodRunStore) runDirLocked(id string) string {
	return filepath.Join(ps.jobsDir, id)
}

// runDir returns the per-run working directory.
func (ps *prodRunStore) runDir(id string) string {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.runDirLocked(id)
}
