package web

import (
	"bytes"
	"encoding/json"
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
)

// prodRunRunner spawns and polls a single headless child process per run.
type prodRunRunner struct {
	store         *prodRunStore
	binPath       string
	repoRoot      string
	baseCfg       bootstrap.Config
	cmdFactory    func(name string, arg ...string) *exec.Cmd
	onCmdStarted  func(*exec.Cmd)
	onCmdFinished func(*exec.Cmd, error)

	mu           sync.Mutex
	running      map[string]*runningProc
	pollInterval time.Duration
}

type runningProc struct {
	cmd     *exec.Cmd
	logFile *os.File
	done    chan struct{}
}

func newProdRunRunner(store *prodRunStore, binPath, repoRoot string, baseCfg bootstrap.Config) *prodRunRunner {
	return &prodRunRunner{
		store:        store,
		binPath:      binPath,
		repoRoot:     repoRoot,
		baseCfg:      baseCfg,
		cmdFactory:   exec.Command,
		running:      make(map[string]*runningProc),
		pollInterval: 5 * time.Second,
	}
}

// start transitions a queued run to running and spawns the child process.
// MVP: only one production run may be active at a time.
func (rr *prodRunRunner) start(id string) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if len(rr.running) > 0 {
		return fmt.Errorf("another production run is already running")
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
	if err := prepareRunDir(runDir, rr.repoRoot, r, rr.baseCfg); err != nil {
		rr.markFailed(id)
		return fmt.Errorf("prepare run dir: %w", err)
	}

	logPath := filepath.Join(runDir, "run.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		rr.markFailed(id)
		return fmt.Errorf("open run log: %w", err)
	}

	cmd := rr.cmdFactory(rr.binPath, "--headless", "--prompt-file", "profile.md")
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

	close(proc.done)
	_ = proc.logFile.Close()

	if rr.onCmdFinished != nil {
		rr.onCmdFinished(proc.cmd, err)
	}

	rr.mu.Lock()
	delete(rr.running, id)
	rr.mu.Unlock()
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

	chapters := readCompletedChapters(filepath.Join(runDir, "output", "novel", "meta", "progress.json"))
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
		return
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

// prepareRunDir creates the per-run directory, copies the profile and home rules,
// and writes the override config so the child engine sees its own output dir.
func prepareRunDir(runDir, repoRoot string, r *ProdRun, baseCfg bootstrap.Config) error {
	cfgDir := filepath.Join(runDir, ".ainovel")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(cfgDir, "rules"), 0o755); err != nil {
		return err
	}

	srcProfile := filepath.Join(repoRoot, r.Profile)
	dstProfile := filepath.Join(runDir, "profile.md")
	if err := copyFile(dstProfile, srcProfile); err != nil {
		return fmt.Errorf("copy profile: %w", err)
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
	runner := newProdRunRunner(store, binPath, repoRoot, baseCfg)
	return &prodRunManager{store: store, runner: runner, hostDir: hostDir}, nil
}

func (pm *prodRunManager) Create(name, profile, model, provider string, targetChapters int, budgetUSD float64) (*ProdRun, error) {
	return pm.store.create(name, profile, model, provider, targetChapters, budgetUSD)
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

// sanitizeFileName strips unsafe characters from a run name for use as a file name.
func sanitizeFileName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.', r == ' ':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	name := strings.TrimSpace(b.String())
	if name == "" {
		return "export"
	}
	return strings.ReplaceAll(name, " ", "_")
}

// stringSliceToInts parses chapter numbers from file names for sorting.
func stringSliceToInts(ss []string) ([]int, error) {
	out := make([]int, len(ss))
	for i, s := range ss {
		s = strings.TrimSuffix(s, filepath.Ext(s))
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

// sortChapterFiles sorts chapter file names numerically when possible.
func sortChapterFiles(files []string) {
	nums, err := stringSliceToInts(files)
	if err != nil {
		sort.Strings(files)
		return
	}
	sort.Slice(files, func(i, j int) bool {
		return nums[i] < nums[j]
	})
}

// bytesBufferPool is used by exportRunTXT to avoid repeated allocations.
var bytesBufferPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}
