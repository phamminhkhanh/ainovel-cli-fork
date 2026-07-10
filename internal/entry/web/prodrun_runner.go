package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/voocel/ainovel-cli/internal/bootstrap"
	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/store"
)

// prodRunRunner spawns and polls a single headless child process per run.
type prodRunRunner struct {
	store         *prodRunStore
	binPath       string
	repoRoot      string
	hostDir       string
	baseCfg       bootstrap.Config
	cmdFactory    func(name string, arg ...string) *exec.Cmd
	onCmdStarted  func(*exec.Cmd)
	onCmdFinished func(*exec.Cmd, error)

	mu           sync.Mutex
	running      map[string]*runningProc
	pollInterval time.Duration
	reapTimeout  time.Duration
}

type runningProc struct {
	cmd     *exec.Cmd
	logFile *os.File
	done    chan struct{}
}

func newProdRunRunner(store *prodRunStore, binPath, repoRoot, hostDir string, baseCfg bootstrap.Config) *prodRunRunner {
	return &prodRunRunner{
		store:        store,
		binPath:      binPath,
		repoRoot:     repoRoot,
		hostDir:      hostDir,
		baseCfg:      baseCfg,
		cmdFactory:   exec.Command,
		running:      make(map[string]*runningProc),
		pollInterval: 5 * time.Second,
		reapTimeout:  prodRunReapTimeout,
	}
}

// start transitions a queued run to running and spawns the child process.
// MVP: only one production run may be active at a time.
func (rr *prodRunRunner) start(id string) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if len(rr.running) > 0 {
		return errAnotherRunActive
	}

	r, err := rr.store.update(id, func(r *ProdRun) {
		if r.Status != prodRunQueued {
			return
		}
		r.Status = prodRunRunning
		r.StartedAt = time.Now()
		r.StoppedAt = time.Time{}
		r.StopReason = ""
	})
	if err != nil {
		return fmt.Errorf("update run status: %w", err)
	}
	if r == nil {
		return fmt.Errorf("run %q not found", id)
	}
	if r.Status != prodRunRunning {
		return fmt.Errorf("run %q is not queued (status=%s)", id, r.Status)
	}

	runDir := rr.store.runDir(id)
	if err := prepareRunDir(runDir, rr.repoRoot, rr.hostDir, r, rr.baseCfg); err != nil {
		rr.markFailed(id)
		return fmt.Errorf("prepare run dir: %w", err)
	}
	if r.kind() == prodRunKindContinueWorkspace && r.SeededFrom != nil && !r.SeededFrom.SeededAt.IsZero() {
		seededAt := r.SeededFrom.SeededAt
		if _, err := rr.store.update(id, func(stored *ProdRun) {
			if stored.SeededFrom != nil {
				stored.SeededFrom.SeededAt = seededAt
			}
		}); err != nil {
			rr.markFailed(id)
			return fmt.Errorf("persist seed metadata: %w", err)
		}
	}

	logPath := filepath.Join(runDir, "run.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		rr.markFailed(id)
		return fmt.Errorf("open run log: %w", err)
	}

	args := []string{"--headless"}
	if r.kind() == prodRunKindFreshProfile && !runDirHasExistingOutput(runDir) {
		// Only pass --prompt-file on a truly fresh run dir. If output/novel
		// already has a progress.json, this is a Foundation Gate approve-resume:
		// the run dir already contains a seeded book at
		// phase=writing, so headless must go through native Resume() instead
		// of re-running startup.PrepareQuick on the same profile.
		args = append(args, "--prompt-file", "profile.md")
	}
	cmd := rr.cmdFactory(rr.binPath, args...)
	cmd.Dir = runDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		rr.markFailed(id)
		return fmt.Errorf("start headless process: %w", err)
	}

	if _, err := rr.store.update(id, func(r *ProdRun) {
		r.ChildPID = cmd.Process.Pid
		r.LogPath = logPath
	}); err != nil {
		fmt.Fprintf(os.Stderr, "prodrun: failed to persist child pid: %v\n", err)
	}

	if rr.onCmdStarted != nil {
		rr.onCmdStarted(cmd)
	}

	proc := &runningProc{cmd: cmd, logFile: logFile, done: make(chan struct{})}
	rr.running[id] = proc

	go rr.waitProc(id, proc)
	go rr.pollLoop(id, proc)

	return nil
}

func (rr *prodRunRunner) markFailed(id string) {
	if _, err := rr.store.update(id, func(r *ProdRun) {
		r.Status = prodRunFailed
		r.StopReason = stopReasonError
		r.StoppedAt = time.Now()
	}); err != nil {
		fmt.Fprintf(os.Stderr, "prodrun: failed to mark run %s failed: %v\n", id, err)
	}
}

// waitProc waits for the child to exit and records the final state.
func (rr *prodRunRunner) waitProc(id string, proc *runningProc) {
	err := proc.cmd.Wait()

	// Resolve the stored status before releasing the process slot so that a
	// concurrent start() cannot observe a running status with no process.
	if _, saveErr := rr.store.update(id, func(r *ProdRun) {
		if r.Status == prodRunCancelled || r.Status == prodRunCompleted {
			return
		}
		if r.Status == prodRunRunning || r.Status == prodRunPaused {
			if err != nil {
				r.Status = prodRunFailed
				r.StopReason = stopReasonError
			} else {
				r.Status = prodRunCompleted
				r.StopReason = stopReasonCompleted
			}
			r.StoppedAt = time.Now()
		}
	}); saveErr != nil {
		fmt.Fprintf(os.Stderr, "prodrun: failed to persist terminal status for %s: %v\n", id, saveErr)
	}

	_ = proc.logFile.Close()

	if rr.onCmdFinished != nil {
		rr.onCmdFinished(proc.cmd, err)
	}

	// Free the single-run slot before signaling done so that waitReaped,
	// which blocks on proc.done, observes a cleared rr.running entry. Closing
	// done first would let a waiter proceed while start() could still see the
	// slot occupied for a brief instant.
	rr.mu.Lock()
	delete(rr.running, id)
	rr.mu.Unlock()
	close(proc.done)
}

// waitReaped blocks until the run's child has exited and its single-run slot
// has been freed, or until timeout elapses. Returns nil when the slot is
// already free (no child, or already reaped). Approve/Revise call this before
// restarting so the gate's async killProcess cannot race the new start: by the
// time waitReaped returns nil, start()'s len(rr.running)==0 check is stable.
func (rr *prodRunRunner) waitReaped(id string, timeout time.Duration) error {
	rr.mu.Lock()
	proc := rr.running[id]
	rr.mu.Unlock()
	if proc == nil {
		return nil
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case <-proc.done:
		return nil
	case <-t.C:
		return errReapTimeout
	}
}

// pollLoop reads progress from the child output directory every 5 seconds.
func (rr *prodRunRunner) pollLoop(id string, proc *runningProc) {
	interval := rr.pollInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-proc.done:
			return
		case <-ticker.C:
			rr.poll(id)
		}
	}
}

func (rr *prodRunRunner) poll(id string) {
	runDir := rr.store.runDir(id)
	progressPath := filepath.Join(runDir, "output", "novel", "meta", "progress.json")

	chapters := readCompletedChapters(progressPath)
	reviews, rewrites := countReviewsAndRewrites(filepath.Join(runDir, "output", "novel", "reviews"))
	cost := readCostUSD(filepath.Join(runDir, "output", "novel", "meta", "usage.json"))

	if _, err := rr.store.update(id, func(r *ProdRun) {
		r.Chapters = chapters
		r.Reviews = reviews
		r.Rewrites = rewrites
		r.CostUSD = cost
	}); err != nil {
		fmt.Fprintf(os.Stderr, "prodrun: failed to persist stats for %s: %v\n", id, err)
	}

	// Foundation Gate: a fresh_profile run whose Architect just finished the
	// foundation flips Phase to writing. Catch that transition here and stop
	// the child so the user can review premise/outline/world/characters before
	// committing to the bulk of the book. Best-effort (5s poll): the phase flip
	// + writer dispatch happen synchronously in save_foundation, so worst case
	// the Writer already drafted part of chapter 1 before this tick lands.
	// continue_workspace runs are seeded already in phase=writing, so this
	// check must not apply to them or every continue run would be paused
	// immediately after start.
	r := rr.store.get(id)
	if r != nil && r.kind() == prodRunKindFreshProfile && r.Status == prodRunRunning && chapters == 0 && !r.FoundationApproved {
		if readWorkspacePhase(progressPath) == string(domain.PhaseWriting) {
			if _, err := rr.store.update(id, func(r *ProdRun) {
				if r.Status == prodRunRunning {
					r.Status = prodRunAwaitingReview
					r.StopReason = stopReasonFoundationReady
					r.StoppedAt = time.Now()
				}
			}); err != nil {
				fmt.Fprintf(os.Stderr, "prodrun: failed to persist awaiting_review status for %s: %v\n", id, err)
			}
			rr.killProcess(id)
			return
		}
	}

	// Pause detection: the engine prints a Chinese prompt when it waits for user input.
	logTail := readLogTail(filepath.Join(runDir, "run.log"), 4<<10)
	if hasPauseMarker(logTail) {
		if _, err := rr.store.update(id, func(r *ProdRun) {
			if r.Status == prodRunRunning {
				r.Status = prodRunPaused
			}
		}); err != nil {
			fmt.Fprintf(os.Stderr, "prodrun: failed to persist pause status for %s: %v\n", id, err)
		}
	}

	// Hard target-chapters stop.
	targetReached := false
	if _, err := rr.store.update(id, func(r *ProdRun) {
		if r.Status == prodRunRunning && r.TargetChapters > 0 && chapters >= r.TargetChapters {
			r.Status = prodRunCompleted
			r.StopReason = stopReasonTargetReached
			r.StoppedAt = time.Now()
			targetReached = true
		}
	}); err != nil {
		fmt.Fprintf(os.Stderr, "prodrun: failed to persist target-reached status for %s: %v\n", id, err)
		// Không return: process vẫn phải bị kill để không tiếp tục tiêu tiền
		// sau khi đã đạt target-chapters. In-memory đã set targetReached=true
		// trong closure (chạy trước saveLocked), nên killProcess phải chạy.
		// PersistError đã được update() ghi nội bộ → UI health strip sẽ báo.
	}
	if targetReached {
		rr.killProcess(id)
	}
}

// stop kills a running child process and marks the run as cancelled.
// It returns an error if the run is not currently active.
func (rr *prodRunRunner) stop(id string) error {
	r := rr.store.get(id)
	if r == nil {
		return fmt.Errorf("run %q not found", id)
	}
	if r.Status != prodRunRunning && r.Status != prodRunPaused {
		return fmt.Errorf("run %q is not active (status=%s)", id, r.Status)
	}
	return rr.kill(id, stopReasonCancelled)
}

func (rr *prodRunRunner) kill(id, reason string) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.killLocked(id, reason)
}

func (rr *prodRunRunner) killLocked(id, reason string) error {
	proc, ok := rr.running[id]
	if !ok {
		// The process slot is gone; reconcile stored status only.
		if _, err := rr.store.update(id, func(r *ProdRun) {
			if r.Status == prodRunCompleted {
				return
			}
			if r.Status == prodRunRunning || r.Status == prodRunPaused {
				r.Status = prodRunCancelled
				r.StopReason = reason
				r.StoppedAt = time.Now()
			}
		}); err != nil {
			fmt.Fprintf(os.Stderr, "prodrun: failed to persist cancelled status for %s: %v\n", id, err)
		}
		return nil
	}

	if proc.cmd.Process != nil {
		_ = proc.cmd.Process.Kill()
	}

	if _, err := rr.store.update(id, func(r *ProdRun) {
		if r.Status == prodRunCompleted {
			return
		}
		r.Status = prodRunCancelled
		r.StopReason = reason
		r.StoppedAt = time.Now()
	}); err != nil {
		fmt.Fprintf(os.Stderr, "prodrun: failed to persist cancelled status for %s: %v\n", id, err)
	}
	return nil
}

// killProcess terminates the child process without touching the stored status.
// It is used after the status has already been resolved (e.g. target reached).
func (rr *prodRunRunner) killProcess(id string) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	proc, ok := rr.running[id]
	if !ok || proc.cmd.Process == nil {
		return
	}
	_ = proc.cmd.Process.Kill()
}

// prepareRunDir creates the per-run sandbox. Fresh runs copy a profile prompt;
// continue runs seed output/novel from the host workspace and intentionally do
// not create profile.md so headless enters native Resume().
func prepareRunDir(runDir, repoRoot, hostDir string, r *ProdRun, baseCfg bootstrap.Config) error {
	cfgDir := filepath.Join(runDir, ".ainovel")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(cfgDir, "rules"), 0o755); err != nil {
		return err
	}

	switch r.kind() {
	case prodRunKindFreshProfile:
		// Foundation Gate approve-resume: the run dir already holds a seeded
		// book at phase=writing and the child will Resume() natively (no
		// --prompt-file), so the profile is neither needed nor copied. Requiring
		// it here would make approve fail if the source profile was moved/deleted
		// after the initial run — and re-copying would be pointless I/O.
		if !runDirHasExistingOutput(runDir) {
			srcProfile, err := resolveExistingProfilePath(r.Profile, repoRoot)
			if err != nil {
				return fmt.Errorf("resolve profile: %w", err)
			}
			dstProfile := filepath.Join(runDir, "profile.md")
			if err := copyFile(dstProfile, srcProfile); err != nil {
				return fmt.Errorf("copy profile: %w", err)
			}
			// Foundation Gate revise: append the user's steering note so the
			// Architect regenerates the foundation with it in mind.
			if strings.TrimSpace(r.RevisionNote) != "" {
				note := "\n\n## 用户修订要求 (User revision request)\n" + r.RevisionNote + "\n"
				f, err := os.OpenFile(dstProfile, os.O_APPEND|os.O_WRONLY, 0o644)
				if err != nil {
					return fmt.Errorf("open profile for revision note: %w", err)
				}
				if _, err := f.WriteString(note); err != nil {
					_ = f.Close()
					return fmt.Errorf("append revision note: %w", err)
				}
				if err := f.Close(); err != nil {
					return fmt.Errorf("close profile after revision note: %w", err)
				}
			}
		}
	case prodRunKindContinueWorkspace:
		if r.SeededFrom == nil || r.SeededFrom.Fingerprint == "" {
			return fmt.Errorf("continue run is missing seed metadata")
		}
		before, err := fingerprintHostWorkspace(hostDir)
		if err != nil {
			return err
		}
		if before != r.SeededFrom.Fingerprint {
			return errSeedWorkspaceChanged
		}
		runOutDir := filepath.Join(runDir, "output", "novel")
		if err := os.RemoveAll(runOutDir); err != nil {
			return fmt.Errorf("clear seed output dir: %w", err)
		}
		if _, err := copyWorkspaceSeed(runOutDir, hostDir); err != nil {
			return fmt.Errorf("seed workspace: %w", err)
		}
		after, err := fingerprintHostWorkspace(hostDir)
		if err != nil {
			return err
		}
		if after != before {
			return errSeedWorkspaceChanged
		}
		r.SeededFrom.SeededAt = time.Now()
	default:
		return fmt.Errorf("unsupported production run kind %q", r.Kind)
	}

	homeRules := filepath.Join(bootstrap.DefaultConfigDir(), "rules")
	if err := copyDirFiles(filepath.Join(cfgDir, "rules"), homeRules, ".md"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("copy home rules: %w", err)
	}

	cfg := buildRunConfig(baseCfg, r)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), data, 0o644); err != nil {
		return fmt.Errorf("write run config: %w", err)
	}

	return nil
}

func buildRunConfig(baseCfg bootstrap.Config, r *ProdRun) bootstrap.Config {
	cfg := baseCfg

	// Strip runtime-only fields by re-serializing is not needed because OutputDir is json:"-".
	if r.Provider != "" {
		cfg.Provider = r.Provider
	}
	if r.Model != "" {
		cfg.ModelName = r.Model
	}
	cfg.Budget = bootstrap.BudgetConfig{
		BookUSD:   r.BudgetUSD,
		WarnRatio: baseCfg.Budget.WarnRatio,
		HardStop:  true,
	}
	return cfg
}

func readCompletedChapters(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var p domain.Progress
	if err := json.Unmarshal(data, &p); err != nil {
		return 0
	}
	return len(p.CompletedChapters)
}

// runDirHasExistingOutput reports whether a run's output/novel/meta/progress.json
// already exists, meaning this run dir was already started once (Foundation
// Gate approve-resume) rather than being a brand-new fresh_profile run.
func runDirHasExistingOutput(runDir string) bool {
	_, err := os.Stat(filepath.Join(runDir, "output", "novel", "meta", "progress.json"))
	return err == nil
}

// readWorkspacePhase returns the Phase string from a progress.json, or ""
// if the file is missing/unreadable. Used by the Foundation Gate poll check.
func readWorkspacePhase(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var p domain.Progress
	if err := json.Unmarshal(data, &p); err != nil {
		return ""
	}
	return string(p.Phase)
}

func countReviewsAndRewrites(dir string) (int, int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}
	reviews := 0
	rewrites := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var r domain.ReviewEntry
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		reviews++
		if r.Verdict == "rewrite" {
			rewrites++
		}
	}
	return reviews, rewrites
}

func readCostUSD(path string) float64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var state domain.UsageState
	if err := json.Unmarshal(data, &state); err != nil {
		return 0
	}
	return state.Overall.Cost
}

func readLogTail(path string, maxBytes int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return ""
	}
	offset := int64(0)
	if stat.Size() > maxBytes {
		offset = stat.Size() - maxBytes
	}
	buf := make([]byte, stat.Size()-offset)
	_, _ = f.ReadAt(buf, offset)
	return string(buf)
}

func hasPauseMarker(tail string) bool {
	// Pause markers observed in headless engine output. If upstream changes the
	// wording, add the new marker here. The list is intentionally conservative:
	// a false positive just flips status to paused, which is recoverable by Stop.
	markers := []string{"等待用户输入", "等待输入", "paused", "用户暂停"}
	lower := strings.ToLower(tail)
	for _, m := range markers {
		if strings.Contains(lower, strings.ToLower(m)) {
			return true
		}
	}
	return false
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.ReadFrom(in)
	return err
}

func copyDirFiles(dstDir, srcDir, suffix string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if suffix != "" && !strings.HasSuffix(strings.ToLower(name), suffix) {
			continue
		}
		if err := copyFile(filepath.Join(dstDir, name), filepath.Join(srcDir, name)); err != nil {
			return err
		}
	}
	return nil
}

// ── prodRunManager wires store + runner for handlers ──

type prodRunManager struct {
	store   *prodRunStore
	runner  *prodRunRunner
	hostDir string // main workspace dir to sync production runs into
}

func newProdRunManager(jobsDir, binPath, repoRoot, hostDir string, baseCfg bootstrap.Config) (*prodRunManager, error) {
	store, err := newProdRunStore(jobsDir)
	if err != nil {
		return nil, err
	}
	runner := newProdRunRunner(store, binPath, repoRoot, hostDir, baseCfg)
	return &prodRunManager{store: store, runner: runner, hostDir: hostDir}, nil
}

func (pm *prodRunManager) Create(name, profile, model, provider string, targetChapters int, budgetUSD float64) (*ProdRun, error) {
	return pm.store.create(name, profile, model, provider, targetChapters, budgetUSD)
}

func (pm *prodRunManager) CreateContinue(name, model, provider string, targetChapters int, budgetUSD float64) (*ProdRun, error) {
	seed, err := seedMetaForWorkspace(pm.hostDir)
	if err != nil {
		return nil, err
	}
	if targetChapters <= seed.CompletedChapters {
		return nil, fmt.Errorf("target chapters must be greater than current completed chapters (%d)", seed.CompletedChapters)
	}
	return pm.store.createWithOptions(prodRunCreateOptions{
		Kind:           prodRunKindContinueWorkspace,
		Name:           name,
		Model:          model,
		Provider:       provider,
		TargetChapters: targetChapters,
		BudgetUSD:      budgetUSD,
		SeededFrom:     seed,
		Chapters:       seed.CompletedChapters,
	})
}

// ApproveFoundation resumes a run that is awaiting_review (Foundation Gate).
// The run's own sandbox dir already has a seeded book at
// phase=writing from the first (killed) headless invocation, so this simply
// flips status back to queued and restarts the same run dir: prepareRunDir's
// fresh_profile branch is idempotent, and the runner's --prompt-file guard
// (runDirHasExistingOutput) makes the child skip startup.PrepareQuick and go
// through native Resume() instead.
func (pm *prodRunManager) ApproveFoundation(id string) (*ProdRun, error) {
	// The gate's killProcess is async: the just-stopped child may still occupy
	// the single-run slot. Block on its reap BEFORE flipping status to queued,
	// so the run never enters a "queued while child still alive" window where a
	// concurrent Delete (which only blocks running/paused) could RemoveAll the
	// runDir out from under the live child. waitReaped reads the slot under
	// rr.mu; if the run was already reaped this returns immediately.
	reapTimeout := pm.runner.reapTimeout
	if reapTimeout <= 0 {
		reapTimeout = prodRunReapTimeout
	}
	if reapErr := pm.runner.waitReaped(id, reapTimeout); reapErr != nil {
		return nil, reapErr
	}
	// Re-check status after the reap wait: the user may have rejected/deleted
	// the run while we were waiting. Only proceed if still awaiting_review.
	r, err := pm.store.update(id, func(r *ProdRun) {
		if r.Status != prodRunAwaitingReview {
			return
		}
		r.Status = prodRunQueued
		r.StopReason = ""
		r.StoppedAt = time.Time{}
		r.FoundationApproved = true
	})
	if err != nil {
		return nil, err
	}
	if r.Status != prodRunQueued {
		return nil, fmt.Errorf("run %q is no longer awaiting review (status=%s)", id, r.Status)
	}
	if startErr := pm.startWithReapRetry(id); startErr != nil {
		// Don't strand the run in queued (it could no longer be rejected).
		// Revert to awaiting_review only if start() failed before setting
		// running (errAnotherRunActive / not-queued). If start() already
		// markFailed the run (prepareRunDir/cmd.Start failure after the
		// running flip), the guard below leaves that failed status intact.
		if _, revErr := pm.store.update(id, func(r *ProdRun) {
			if r.Status == prodRunQueued {
				r.Status = prodRunAwaitingReview
				r.StopReason = stopReasonFoundationReady
			}
		}); revErr != nil {
			fmt.Fprintf(os.Stderr, "prodrun: approve failed and could not revert %s: %v\n", id, revErr)
		}
		return nil, startErr
	}
	return pm.store.get(id), nil
}

// ResumeFailed cho phép tiếp tục một run đã dừng (status=failed hoặc cancelled).
// Dùng khi run fail/cancel do lỗi transient (rule stale, rate limit, context overflow)
// hoặc do user chủ động bấm Dừng rồi muốn nấu tiếp. prepareRunDir sẽ copy home rules
// mới nhất (dòng 459-461) nên writer thấy rule cập nhật, và headless Resume()
// natively từ sandbox dir có sẵn output.
//
// steer (tùy chọn):干预文本, ghi vào sandbox meta/run.json qua store API
// (SetPendingSteer + AppendSteerEntry) TRƯỚC khi start child. Headless Resume()
// (host.go:374) đọc pending_steer và inject vào Coordinator ngay chương kế — đây
// là seam host đã có sẵn, web adapter chỉ ghi file, zero đụng host/headless.
// Lưu ý: steer là干预 mềm (Coordinator đánh giá & áp theo coordinator.md), không
// phải structural rule cứng. Chỉ ăn ở ranh giới resume (run đang dừng), không
// steer được khi child đang chạy.
//
// KHÔNG set FoundationApproved (giữ giá trị cũ): run đã approve thì giữ, run
// fail trước foundation gate thì vẫn cần duyệt sau.
func (pm *prodRunManager) ResumeFailed(id, steer string) (*ProdRun, error) {
	reapTimeout := pm.runner.reapTimeout
	if reapTimeout <= 0 {
		reapTimeout = prodRunReapTimeout
	}
	if reapErr := pm.runner.waitReaped(id, reapTimeout); reapErr != nil {
		return nil, reapErr
	}
	r, err := pm.store.update(id, func(r *ProdRun) {
		if r.Status != prodRunFailed && r.Status != prodRunCancelled {
			return
		}
		r.Status = prodRunQueued
		r.StopReason = ""
		r.StoppedAt = time.Time{}
	})
	if err != nil {
		return nil, err
	}
	if r.Status != prodRunQueued {
		return nil, fmt.Errorf("run %q is not failed or cancelled (status=%s), cannot resume", id, r.Status)
	}
	// Ghi steer干预 vào sandbox meta/run.json trước khi start child. Store API
	// an toàn lock + encoding; path = runDir/output/novel (eng.Dir() của child).
	// Gate runDirHasExistingOutput: chỉ ghi khi có output (đường Resume). Nếu run
	// fail trước foundation (0 output) → child chạy --prompt-file (fresh
	// StartPrepared, KHÔNG Resume) → pending_steer không được đọc, lại tạo ra
	// run.json mồ côi trong sandbox. Lỗi ghi steer không block resume — run vẫn
	// nấu tiếp, chỉ thiếu干预.
	if steer = strings.TrimSpace(steer); steer != "" {
		runDir := pm.store.runDir(id)
		if runDirHasExistingOutput(runDir) {
			runOutDir := filepath.Join(runDir, "output", "novel")
			st := store.NewStore(runOutDir)
			if err := st.RunMeta.SetPendingSteer(steer); err != nil {
				fmt.Fprintf(os.Stderr, "prodrun: resume steer SetPendingSteer %s: %v\n", id, err)
			} else if err := st.RunMeta.AppendSteerEntry(domain.SteerEntry{
				Input:     steer,
				Timestamp: time.Now().Format(time.RFC3339),
			}); err != nil {
				fmt.Fprintf(os.Stderr, "prodrun: resume steer AppendSteerEntry %s: %v\n", id, err)
			}
		} else {
			fmt.Fprintf(os.Stderr, "prodrun: resume steer skipped %s: no existing output (fresh StartPrepared, not Resume)\n", id)
		}
	}
	if startErr := pm.startWithReapRetry(id); startErr != nil {
		// Revert về failed nếu start fail trước khi set running.
		if _, revErr := pm.store.update(id, func(r *ProdRun) {
			if r.Status == prodRunQueued {
				r.Status = prodRunFailed
				r.StopReason = stopReasonError
				r.StoppedAt = time.Now()
			}
		}); revErr != nil {
			fmt.Fprintf(os.Stderr, "prodrun: resume-failed start failed and could not revert %s: %v\n", id, revErr)
		}
		return nil, startErr
	}
	return pm.store.get(id), nil
}
// is still occupied by a just-killed child that waitProc hasn't reaped yet
// (the gate's killProcess is async). Any non-"active" error returns immediately.
func (pm *prodRunManager) startWithReapRetry(id string) error {
	var startErr error
	for i := 0; i < 20; i++ {
		startErr = pm.runner.start(id)
		if startErr == nil {
			return nil
		}
		if !errors.Is(startErr, errAnotherRunActive) {
			return startErr
		}
		time.Sleep(50 * time.Millisecond)
	}
	return startErr
}

// ReviseFoundation regenerates the foundation of an awaiting_review run with a
// user steering note (Foundation Gate). It creates a NEW fresh_profile run
// from the same profile with the note appended to the prompt and starts it
// (regenerating the foundation and pausing again at the gate). Cost is just the
// cheap Architect foundation pass (~$0.01); the new run hits the same
// best-effort gate, so worst case it too drafts a partial chapter 1.
//
// The OLD run is intentionally KEPT (not deleted): the new run may fail during
// regeneration (API/budget/profile), and deleting the reviewed candidate before
// the new one reaches awaiting_review would lose it. The user picks the new run
// and rejects the old when satisfied.
func (pm *prodRunManager) ReviseFoundation(id, feedback string) (*ProdRun, error) {
	feedback = strings.TrimSpace(feedback)
	if feedback == "" {
		return nil, fmt.Errorf("revision note is required")
	}
	old := pm.store.get(id)
	if old == nil {
		return nil, errDeleteRunNotFound
	}
	if old.Status != prodRunAwaitingReview {
		return nil, fmt.Errorf("run %q is not awaiting review (status=%s)", id, old.Status)
	}
	// The gate's killProcess is async and start() guards on the GLOBAL single-run
	// slot (len(rr.running)==0), so the OLD run's child still blocks the NEW
	// sibling. Wait for the OLD child to be reaped before creating+starting the
	// new run; on timeout, abort without spending any Architect tokens.
	reapTimeout := pm.runner.reapTimeout
	if reapTimeout <= 0 {
		reapTimeout = prodRunReapTimeout
	}
	if reapErr := pm.runner.waitReaped(id, reapTimeout); reapErr != nil {
		return nil, reapErr
	}
	// Re-fetch and revalidate the OLD run after the reap wait: the user may
	// have rejected/deleted it while we were waiting. Create the sibling only
	// against a run that is still awaiting_review, so we don't spend Architect
	// tokens on a revise for a run the user already discarded.
	old = pm.store.get(id)
	if old == nil {
		return nil, errDeleteRunNotFound
	}
	if old.Status != prodRunAwaitingReview {
		return nil, fmt.Errorf("run %q is no longer awaiting review (status=%s)", id, old.Status)
	}
	newRun, err := pm.store.createWithOptions(prodRunCreateOptions{
		Kind:           prodRunKindFreshProfile,
		Name:           old.Name + " (s\u1eeda)",
		Profile:        old.Profile,
		Model:          old.Model,
		Provider:       old.Provider,
		TargetChapters: old.TargetChapters,
		BudgetUSD:      old.BudgetUSD,
		RevisionNote:   feedback,
	})
	if err != nil {
		return nil, err
	}
	if startErr := pm.startWithReapRetry(newRun.ID); startErr != nil {
		// Clean up the dead new run; the old reviewed candidate stays intact.
		_ = pm.store.delete(newRun.ID)
		return nil, startErr
	}
	return pm.store.get(newRun.ID), nil
}

func (pm *prodRunManager) Get(id string) *ProdRun  { return pm.store.get(id) }
func (pm *prodRunManager) List() []*ProdRun        { return pm.store.list() }
func (pm *prodRunManager) Start(id string) error   { return pm.runner.start(id) }
func (pm *prodRunManager) Stop(id string) error    { return pm.runner.stop(id) }
func (pm *prodRunManager) RunDir(id string) string { return pm.store.runDir(id) }

func (pm *prodRunManager) Delete(id string) error {
	r := pm.store.get(id)
	if r == nil {
		return errDeleteRunNotFound
	}
	if r.Status == prodRunRunning || r.Status == prodRunPaused {
		return fmt.Errorf("%w (status=%s)", errDeleteRunActive, r.Status)
	}
	return pm.store.delete(id)
}

// Sync copies a finished production run's output into the main host workspace.
func (pm *prodRunManager) Sync(id string, opts syncOptions) (*syncResult, error) {
	r := pm.store.get(id)
	if r == nil {
		return nil, errSyncRunNotFound
	}
	if r.Status == prodRunRunning || r.Status == prodRunPaused {
		return nil, fmt.Errorf("%w (status=%s)", errSyncRunActive, r.Status)
	}
	runOutDir := filepath.Join(pm.store.runDir(id), "output", "novel")
	if r.kind() == prodRunKindContinueWorkspace {
		return syncContinueRunOutputIntoHost(runOutDir, pm.hostDir, r.SeededFrom, opts)
	}
	return syncRunOutputIntoHost(runOutDir, pm.hostDir, opts)
}

// ExportTXT concatenates the run's chapter files into a single TXT file.
func (pm *prodRunManager) ExportTXT(id string) (string, error) {
	return exportRunTXT(pm.store, id)
}

// readLogLines returns the last n lines of the run log without loading the
// entire file into memory.
func (pm *prodRunManager) ReadLogTail(id string, n int) ([]string, error) {
	r := pm.store.get(id)
	if r == nil {
		return nil, fmt.Errorf("run not found")
	}
	path := r.LogPath
	if path == "" {
		path = filepath.Join(pm.store.runDir(id), "run.log")
	}
	return tailFileLines(path, n)
}

// tailFileLines reads the last n lines of a file by scanning from the end in
// fixed-size blocks. It avoids loading multi-GB logs into memory.
func tailFileLines(path string, n int) ([]string, error) {
	if n <= 0 {
		return []string{}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := stat.Size()
	if size == 0 {
		return []string{}, nil
	}

	const block = 4096
	var buf []byte
	pos := size
	for pos > 0 {
		readLen := block
		if int64(readLen) > pos {
			readLen = int(pos)
		}
		pos -= int64(readLen)
		chunk := make([]byte, readLen)
		if _, err := f.ReadAt(chunk, pos); err != nil {
			return nil, err
		}
		buf = append(chunk, buf...)
		if bytes.Count(buf, []byte{'\n'}) >= n {
			break
		}
	}

	all := strings.Split(strings.TrimRight(string(buf), "\r\n"), "\n")
	if len(all) > n {
		all = all[len(all)-n:]
	}
	return all, nil
}

// safeWriteFile writes data to a temporary file and renames it into place,
// retrying a few times to tolerate transient Windows file locks.
func safeWriteFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	var lastErr error
	for i := 0; i < 5; i++ {
		if lastErr = os.Rename(tmp, path); lastErr == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = os.Remove(tmp)
	return lastErr
}

// sanitizeFileName makes a run name safe to use as a file name while keeping
// non-ASCII letters (Vietnamese diacritics, CJK, etc.) intact. Only characters
// that are actually illegal in file paths on Windows/Unix are replaced; spaces
// are collapsed to single underscores for cleaner names.
func sanitizeFileName(s string) string {
	// Reserved/illegal characters on Windows (superset of Unix rules).
	const illegal = `/\:*?"<>|`
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x20: // drop control characters
			continue
		case strings.ContainsRune(illegal, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	// Fields splits on any whitespace run and drops empties, so joining with
	// "_" collapses runs of spaces/tabs into a single underscore.
	name := strings.Join(strings.Fields(b.String()), "_")
	if name == "" {
		return "export"
	}
	return name
}

// sortChapterFiles sorts chapter file names numerically ("2.md" < "10.md").
// Parses each name's number INSIDE the comparator (per-element) — the previous
// version precomputed a parallel `nums` slice then let sort.Slice permute
// `files` without permuting `nums`, so after the first swap the comparator read
// mismatched indices and produced a wrong order. Non-numeric names fall back to
// lexical comparison so a stray file never panics the sort.
func sortChapterFiles(files []string) {
	chapNum := func(name string) (int, bool) {
		n, err := strconv.Atoi(strings.TrimSuffix(name, filepath.Ext(name)))
		return n, err == nil
	}
	sort.Slice(files, func(i, j int) bool {
		ni, oki := chapNum(files[i])
		nj, okj := chapNum(files[j])
		if oki && okj {
			return ni < nj
		}
		return files[i] < files[j]
	})
}

// bytesBufferPool is used by exportRunTXT to avoid repeated allocations.
var bytesBufferPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}
