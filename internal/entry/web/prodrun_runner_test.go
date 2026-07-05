package web

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/voocel/ainovel-cli/internal/bootstrap"
	"github.com/voocel/ainovel-cli/internal/domain"
)

// TestProdrunHelperProcess is invoked by exec.Command in runner tests.
// It sleeps for the duration given by GO_HELPER_SLEEP_MS, then exits.
func TestProdrunHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	ms, _ := strconv.Atoi(os.Getenv("GO_HELPER_SLEEP_MS"))
	if ms <= 0 {
		ms = 2000
	}
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func helperCommand(sleepMs int) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestProdrunHelperProcess", "-test.v")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "GO_HELPER_SLEEP_MS="+strconv.Itoa(sleepMs))
	return cmd
}

func TestRunnerStartCreatesRunDirAndConfig(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(profileDir, "spike.md")
	if err := os.WriteFile(profilePath, []byte("# prompt"), 0o644); err != nil {
		t.Fatal(err)
	}

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, t.TempDir(), bootstrap.Config{})
	runner.pollInterval = 200 * time.Millisecond

	var capturedArgs []string
	var capturedDir string
	runner.cmdFactory = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return helperCommand(200)
	}
	runner.onCmdStarted = func(cmd *exec.Cmd) { capturedDir = cmd.Dir }

	r, err := ps.create("test", "profiles/spike.md", "m", "p", 5, 1)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := runner.start(r.ID); err != nil {
		t.Fatal(err)
	}

	// Wait for the short-lived helper to finish.
	for i := 0; i < 50; i++ {
		run := ps.get(r.ID)
		if run.Status != prodRunRunning {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	run := ps.get(r.ID)
	if run.Status != prodRunCompleted && run.Status != prodRunFailed {
		t.Fatalf("expected terminal status, got %s", run.Status)
	}

	if len(capturedArgs) < 3 || capturedArgs[1] != "--headless" || capturedArgs[2] != "--prompt-file" || capturedArgs[3] != "profile.md" {
		t.Fatalf("unexpected args: %v", capturedArgs)
	}

	runDir := ps.runDir(r.ID)
	if capturedDir != runDir {
		t.Fatalf("expected cwd %s, got %s", runDir, capturedDir)
	}
	if _, err := os.Stat(filepath.Join(runDir, "profile.md")); err != nil {
		t.Fatalf("profile.md missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runDir, ".ainovel", "config.json")); err != nil {
		t.Fatalf("config.json missing: %v", err)
	}
}

func TestRunnerStartContinueSeedsWorkspaceAndResumes(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	hostDir := t.TempDir()
	writeWorkspaceProgress(t, hostDir, []int{1, 2}, domain.PhaseWriting)
	if err := os.MkdirAll(filepath.Join(hostDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "chapters", "01.md"), []byte("chapter 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, hostDir, bootstrap.Config{})
	runner.pollInterval = 200 * time.Millisecond

	var capturedArgs []string
	runner.cmdFactory = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return helperCommand(200)
	}

	seed, err := seedMetaForWorkspace(hostDir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ps.createWithOptions(prodRunCreateOptions{
		Kind:           prodRunKindContinueWorkspace,
		Name:           "continue",
		TargetChapters: 5,
		BudgetUSD:      1,
		SeededFrom:     seed,
		Chapters:       seed.CompletedChapters,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := runner.start(r.ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if ps.get(r.ID).Status != prodRunRunning {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if len(capturedArgs) != 2 || capturedArgs[1] != "--headless" {
		t.Fatalf("continue run must start headless without prompt args, got %v", capturedArgs)
	}
	runDir := ps.runDir(r.ID)
	if _, err := os.Stat(filepath.Join(runDir, "profile.md")); !os.IsNotExist(err) {
		t.Fatalf("continue run must not create profile.md, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(runDir, "output", "novel", "chapters", "01.md")); err != nil {
		t.Fatalf("seeded chapter missing: %v", err)
	}
	if ps.get(r.ID).SeededFrom == nil || ps.get(r.ID).SeededFrom.SeededAt.IsZero() {
		t.Fatal("expected SeededAt to be persisted after seed")
	}
}

func TestRunnerStartContinueRejectsChangedWorkspace(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	hostDir := t.TempDir()
	writeWorkspaceProgress(t, hostDir, []int{1}, domain.PhaseWriting)
	if err := os.MkdirAll(filepath.Join(hostDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "chapters", "01.md"), []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, hostDir, bootstrap.Config{})
	seed, err := seedMetaForWorkspace(hostDir)
	if err != nil {
		t.Fatal(err)
	}
	r, err := ps.createWithOptions(prodRunCreateOptions{
		Kind:           prodRunKindContinueWorkspace,
		Name:           "continue",
		TargetChapters: 5,
		BudgetUSD:      1,
		SeededFrom:     seed,
		Chapters:       seed.CompletedChapters,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "chapters", "01.md"), []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runner.start(r.ID); !errors.Is(err, errSeedWorkspaceChanged) {
		t.Fatalf("expected errSeedWorkspaceChanged, got %v", err)
	}
	if ps.get(r.ID).Status != prodRunFailed {
		t.Fatalf("expected failed status, got %s", ps.get(r.ID).Status)
	}
}

func TestRunnerStopKillsProcess(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.WriteFile(filepath.Join(profileDir, "x.md"), []byte("# x"), 0o644)

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, t.TempDir(), bootstrap.Config{})
	runner.pollInterval = 200 * time.Millisecond
	finished := make(chan struct{})
	runner.cmdFactory = func(name string, args ...string) *exec.Cmd {
		return helperCommand(5000)
	}
	runner.onCmdFinished = func(cmd *exec.Cmd, err error) { close(finished) }

	r, err := ps.create("stopme", "profiles/x.md", "", "", 100, 1)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := runner.start(r.ID); err != nil {
		t.Fatal(err)
	}
	// Give Windows a moment to fully initialize the process handle before we
	// issue a kill, otherwise Process.Kill can fail silently.
	time.Sleep(50 * time.Millisecond)

	// Wait until the run is recorded as running.
	for i := 0; i < 50; i++ {
		if ps.get(r.ID).Status == prodRunRunning {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if ps.get(r.ID).Status != prodRunRunning {
		t.Fatal("run never reached running status")
	}

	if err := runner.stop(r.ID); err != nil {
		t.Fatal(err)
	}

	// Wait for the wait goroutine to record the stop and release handles.
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("wait goroutine did not finish")
	}

	run := ps.get(r.ID)
	if run.Status != prodRunCancelled || run.StopReason != stopReasonCancelled {
		t.Fatalf("expected cancelled, got status=%s reason=%s", run.Status, run.StopReason)
	}
}

func TestRunnerTargetChaptersKillsProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		// The test relies on the helper process pattern which works on Windows,
		// but file/directory polling timing is slightly flaky in CI; keep it.
	}

	dir := t.TempDir()
	repoRoot := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.WriteFile(filepath.Join(profileDir, "x.md"), []byte("# x"), 0o644)

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, t.TempDir(), bootstrap.Config{})
	runner.pollInterval = 200 * time.Millisecond
	finished := make(chan struct{})
	runner.cmdFactory = func(name string, args ...string) *exec.Cmd {
		return helperCommand(30000)
	}
	runner.onCmdFinished = func(cmd *exec.Cmd, err error) { close(finished) }

	r, err := ps.create("target", "profiles/x.md", "", "", 1, 1)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := runner.start(r.ID); err != nil {
		t.Fatal(err)
	}
	// Let Windows initialize the child process handle before the first poll
	// issues a kill.
	time.Sleep(50 * time.Millisecond)

	// Pre-create progress.json with one completed chapter so the first poll sees target reached.
	progressDir := filepath.Join(ps.runDir(r.ID), "output", "novel", "meta")
	if err := os.MkdirAll(progressDir, 0o755); err != nil {
		t.Fatal(err)
	}
	progress := []byte(`{"completed_chapters":[1]}`)
	if err := os.WriteFile(filepath.Join(progressDir, "progress.json"), progress, 0o644); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 200; i++ {
		if ps.get(r.ID).StopReason == stopReasonTargetReached {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("wait goroutine did not finish after target kill")
	}

	run := ps.get(r.ID)
	if run.Status != prodRunCompleted || run.StopReason != stopReasonTargetReached {
		t.Fatalf("expected completed/target_reached, got status=%s reason=%s", run.Status, run.StopReason)
	}
	if run.Chapters != 1 {
		t.Fatalf("expected 1 chapter, got %d", run.Chapters)
	}
}

func TestRunnerPollReadsProgressAndReviews(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.WriteFile(filepath.Join(profileDir, "x.md"), []byte("# x"), 0o644)

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rr := newProdRunRunner(ps, "ainovel-cli", repoRoot, t.TempDir(), bootstrap.Config{})

	r, err := ps.create("pollstats", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	runDir := ps.runDir(r.ID)
	progressDir := filepath.Join(runDir, "output", "novel", "meta")
	if err := os.MkdirAll(progressDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(progressDir, "progress.json"), []byte(`{"completed_chapters":[1,2,3]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	reviewDir := filepath.Join(runDir, "output", "novel", "reviews")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeReviewFile := func(name, verdict string) {
		data := []byte(`{"chapter":1,"scope":"chapter","verdict":"` + verdict + `","summary":""}`)
		if err := os.WriteFile(filepath.Join(reviewDir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeReviewFile("01.json", "accept")
	writeReviewFile("02.json", "rewrite")
	if err := os.WriteFile(filepath.Join(reviewDir, "03.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	rr.poll(r.ID)

	run := ps.get(r.ID)
	if run.Chapters != 3 {
		t.Fatalf("expected 3 chapters, got %d", run.Chapters)
	}
	if run.Reviews != 2 {
		t.Fatalf("expected 2 reviews, got %d", run.Reviews)
	}
	if run.Rewrites != 1 {
		t.Fatalf("expected 1 rewrite, got %d", run.Rewrites)
	}
}

func TestRunnerStartRejectsConcurrentRun(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.WriteFile(filepath.Join(profileDir, "x.md"), []byte("# x"), 0o644)

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, t.TempDir(), bootstrap.Config{})
	runner.cmdFactory = func(name string, args ...string) *exec.Cmd {
		return helperCommand(5000)
	}

	r1, err := ps.create("first", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatalf("create first run: %v", err)
	}
	if err := runner.start(r1.ID); err != nil {
		t.Fatalf("start first run: %v", err)
	}
	// Ensure the process handle is ready before attempting a concurrent start
	// or a stop.
	time.Sleep(50 * time.Millisecond)

	r2, err := ps.create("second", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatalf("create second run: %v", err)
	}
	if err := runner.start(r2.ID); err == nil {
		t.Fatal("expected error when starting a second concurrent production run")
	}

	finished := make(chan struct{})
	runner.onCmdFinished = func(cmd *exec.Cmd, err error) { close(finished) }

	if err := runner.stop(r1.ID); err != nil {
		t.Fatalf("stop first run: %v", err)
	}
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("first run wait goroutine did not finish")
	}
}

// TestRunnerPollDetectsFoundationReady verifies the Foundation Gate:
// a fresh_profile run whose progress.json flips to
// phase=writing with zero completed chapters is auto-stopped into
// awaiting_review before any Writer token is spent.
func TestRunnerPollDetectsFoundationReady(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.WriteFile(filepath.Join(profileDir, "x.md"), []byte("# x"), 0o644)

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rr := newProdRunRunner(ps, "ainovel-cli", repoRoot, t.TempDir(), bootstrap.Config{})

	r, err := ps.create("foundation", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	// poll() reads rr.store.get(id).Status, so mark it running as start() would.
	if _, err := ps.update(r.ID, func(r *ProdRun) { r.Status = prodRunRunning }); err != nil {
		t.Fatal(err)
	}

	progressDir := filepath.Join(ps.runDir(r.ID), "output", "novel", "meta")
	if err := os.MkdirAll(progressDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Foundation just saved: phase flipped to writing, zero chapters committed yet.
	if err := os.WriteFile(filepath.Join(progressDir, "progress.json"), []byte(`{"phase":"writing","completed_chapters":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	rr.poll(r.ID)

	run := ps.get(r.ID)
	if run.Status != prodRunAwaitingReview {
		t.Fatalf("expected awaiting_review, got status=%s", run.Status)
	}
	if run.StopReason != stopReasonFoundationReady {
		t.Fatalf("expected stopReason=foundation_ready, got %q", run.StopReason)
	}
}

// TestRunnerPollIgnoresFoundationReadyForContinueWorkspace verifies that a
// continue_workspace run — which is seeded already at phase=writing — is
// never mistaken for a fresh foundation-ready transition.
func TestRunnerPollIgnoresFoundationReadyForContinueWorkspace(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	hostDir := t.TempDir()

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rr := newProdRunRunner(ps, "ainovel-cli", repoRoot, hostDir, bootstrap.Config{})

	r, err := ps.createWithOptions(prodRunCreateOptions{
		Kind:           prodRunKindContinueWorkspace,
		Name:           "continue",
		TargetChapters: 5,
		BudgetUSD:      1,
		SeededFrom:     &SeedMeta{HostDir: hostDir, CompletedChapters: 2, Fingerprint: "x"},
		Chapters:       2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.update(r.ID, func(r *ProdRun) { r.Status = prodRunRunning }); err != nil {
		t.Fatal(err)
	}

	progressDir := filepath.Join(ps.runDir(r.ID), "output", "novel", "meta")
	if err := os.MkdirAll(progressDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(progressDir, "progress.json"), []byte(`{"phase":"writing","completed_chapters":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	rr.poll(r.ID)

	run := ps.get(r.ID)
	if run.Status != prodRunRunning {
		t.Fatalf("continue_workspace run must not be gated by Foundation Gate, got status=%s", run.Status)
	}
}

// TestApproveFoundationResumesRunDir verifies the approve path: a run in
// awaiting_review is restarted against its own already-seeded run dir
// (output/novel already has progress.json at phase=writing), so the runner
// must skip --prompt-file and let headless resume natively.
func TestApproveFoundationResumesRunDir(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.WriteFile(filepath.Join(profileDir, "x.md"), []byte("# x"), 0o644)

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, t.TempDir(), bootstrap.Config{})
	runner.pollInterval = 200 * time.Millisecond
	var capturedArgs []string
	runner.cmdFactory = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return helperCommand(200)
	}
	finished := make(chan struct{})
	runner.onCmdFinished = func(cmd *exec.Cmd, err error) { close(finished) }
	pm := &prodRunManager{store: ps, runner: runner, hostDir: t.TempDir()}

	r, err := ps.create("foundation", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the state after the first kill-on-foundation-ready poll: the
	// run dir already has a seeded book at phase=writing.
	progressDir := filepath.Join(ps.runDir(r.ID), "output", "novel", "meta")
	if err := os.MkdirAll(progressDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(progressDir, "progress.json"), []byte(`{"phase":"writing","completed_chapters":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ps.update(r.ID, func(r *ProdRun) {
		r.Status = prodRunAwaitingReview
		r.StopReason = stopReasonFoundationReady
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := pm.ApproveFoundation(r.ID); err != nil {
		t.Fatalf("ApproveFoundation: %v", err)
	}

	// Wait for the wait goroutine to fully close the child's log file handle
	// before the test's t.TempDir() cleanup runs (Windows locks open files).
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("wait goroutine did not finish")
	}

	if len(capturedArgs) != 2 || capturedArgs[1] != "--headless" {
		t.Fatalf("approve-resume must start headless without --prompt-file, got %v", capturedArgs)
	}
	if !ps.get(r.ID).FoundationApproved {
		t.Fatal("expected FoundationApproved=true to be persisted so the gate does not re-fire")
	}
}

// TestApproveFoundationRejectsWrongStatus verifies ApproveFoundation refuses
// to act on a run that is not awaiting_review.
func TestApproveFoundationRejectsWrongStatus(t *testing.T) {
	dir := t.TempDir()
	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := newProdRunRunner(ps, "ainovel-cli", t.TempDir(), t.TempDir(), bootstrap.Config{})
	pm := &prodRunManager{store: ps, runner: runner, hostDir: t.TempDir()}

	r, err := ps.create("queued-run", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pm.ApproveFoundation(r.ID); err == nil {
		t.Fatal("expected error approving a run that is not awaiting_review")
	}
}

// TestRunnerPollDoesNotReGateAfterApprove is a regression test for a bug found
// during adversarial code review: after ApproveFoundation restarts a run, its
// output/novel/progress.json still reports phase=writing with 0 completed
// chapters for as long as the real Writer takes to draft+commit chapter 1
// (routinely longer than one 5s poll tick). Without FoundationApproved,
// poll() would re-detect "foundation just saved" and kill the freshly-approved
// run again before it ever got to write a single chapter.
func TestRunnerPollDoesNotReGateAfterApprove(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.WriteFile(filepath.Join(profileDir, "x.md"), []byte("# x"), 0o644)

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	rr := newProdRunRunner(ps, "ainovel-cli", repoRoot, t.TempDir(), bootstrap.Config{})

	r, err := ps.create("approved", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	progressDir := filepath.Join(ps.runDir(r.ID), "output", "novel", "meta")
	if err := os.MkdirAll(progressDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Same state a real slow Writer leaves progress.json in right after
	// approve-resume: still phase=writing, still 0 committed chapters.
	if err := os.WriteFile(filepath.Join(progressDir, "progress.json"), []byte(`{"phase":"writing","completed_chapters":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ps.update(r.ID, func(r *ProdRun) {
		r.Status = prodRunRunning
		r.FoundationApproved = true
	}); err != nil {
		t.Fatal(err)
	}

	rr.poll(r.ID)

	if got := ps.get(r.ID).Status; got != prodRunRunning {
		t.Fatalf("approved run must not be re-gated into awaiting_review, got status=%s", got)
	}
}

// TestApproveFoundationRetriesThroughReapWindow verifies the approve path
// tolerates the brief window where the gate has killed the child but waitProc
// has not yet removed it from rr.running. ApproveFoundation retries start()
// until the slot clears instead of failing with "already running".
func TestApproveFoundationRetriesThroughReapWindow(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.WriteFile(filepath.Join(profileDir, "x.md"), []byte("# x"), 0o644)

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, t.TempDir(), bootstrap.Config{})
	runner.pollInterval = 10 * time.Second // keep poll out of the way
	finished := make(chan struct{}, 2)
	runner.cmdFactory = func(name string, args ...string) *exec.Cmd { return helperCommand(150) }
	runner.onCmdFinished = func(cmd *exec.Cmd, err error) {
		select {
		case finished <- struct{}{}:
		default:
		}
	}

	r, err := ps.create("reapwindow", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Seed an already-started run dir at phase=writing and mark awaiting_review.
	progressDir := filepath.Join(ps.runDir(r.ID), "output", "novel", "meta")
	_ = os.MkdirAll(progressDir, 0o755)
	_ = os.WriteFile(filepath.Join(progressDir, "progress.json"), []byte(`{"phase":"writing","completed_chapters":[]}`), 0o644)

	// Occupy the running slot as the gate's killProcess path would leave it
	// mid-reap, then release it shortly after to mimic waitProc catching up.
	if err := runner.start(r.ID); err != nil {
		t.Fatalf("prime start: %v", err)
	}
	<-finished // first child exited
	// Force awaiting_review as the gate would.
	if _, err := ps.update(r.ID, func(r *ProdRun) {
		r.Status = prodRunAwaitingReview
		r.StopReason = stopReasonFoundationReady
	}); err != nil {
		t.Fatal(err)
	}

	pm := &prodRunManager{store: ps, runner: runner, hostDir: t.TempDir()}
	if _, err := pm.ApproveFoundation(r.ID); err != nil {
		t.Fatalf("approve should succeed once slot clears: %v", err)
	}
	<-finished // approved child exited
	if !ps.get(r.ID).FoundationApproved {
		t.Fatal("expected FoundationApproved persisted")
	}
}

// TestApproveFoundationResumesEvenIfProfileDeleted is a regression test for a
// bug found during a live smoke test: prepareRunDir's fresh_profile branch
// re-resolved and re-copied profile.md on every start, including approve-
// resume. Since approve-resume Resume()s from the already-seeded run dir and
// never uses the profile, a moved/deleted profile made approve fail with
// status=failed instead of resuming. The fix skips the profile copy when the
// run dir already has seeded output.
func TestApproveFoundationResumesEvenIfProfileDeleted(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	_ = os.MkdirAll(profileDir, 0o755)
	profilePath := filepath.Join(profileDir, "x.md")
	_ = os.WriteFile(profilePath, []byte("# x"), 0o644)

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, t.TempDir(), bootstrap.Config{})
	runner.pollInterval = 10 * time.Second
	var capturedArgs []string
	runner.cmdFactory = func(name string, args ...string) *exec.Cmd {
		capturedArgs = append([]string{name}, args...)
		return helperCommand(150)
	}
	finished := make(chan struct{}, 1)
	runner.onCmdFinished = func(cmd *exec.Cmd, err error) {
		select {
		case finished <- struct{}{}:
		default:
		}
	}

	r, err := ps.create("deleted-profile", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Seed an already-started run dir at phase=writing.
	metaDir := filepath.Join(ps.runDir(r.ID), "output", "novel", "meta")
	_ = os.MkdirAll(metaDir, 0o755)
	_ = os.WriteFile(filepath.Join(metaDir, "progress.json"), []byte(`{"phase":"writing","completed_chapters":[]}`), 0o644)
	if _, err := ps.update(r.ID, func(r *ProdRun) {
		r.Status = prodRunAwaitingReview
		r.StopReason = stopReasonFoundationReady
	}); err != nil {
		t.Fatal(err)
	}

	// Delete the source profile: approve must not depend on it.
	if err := os.Remove(profilePath); err != nil {
		t.Fatal(err)
	}

	pm := &prodRunManager{store: ps, runner: runner, hostDir: t.TempDir()}
	if _, err := pm.ApproveFoundation(r.ID); err != nil {
		t.Fatalf("approve must succeed without the profile: %v", err)
	}
	<-finished
	if len(capturedArgs) != 2 || capturedArgs[1] != "--headless" {
		t.Fatalf("approve-resume must start headless without --prompt-file, got %v", capturedArgs)
	}
	if got := ps.get(r.ID).Status; got == prodRunFailed {
		t.Fatalf("approve must not fail on missing profile, status=%s", got)
	}
}

// TestReviseFoundationCreatesNewRunWithNote verifies the Foundation Gate
// revise path: it creates a new fresh_profile run carrying the revision note,
// starts it, deletes the old awaiting_review run, and the note is appended to
// the spawned run's profile.md.
func TestReviseFoundationCreatesNewRunWithNote(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.WriteFile(filepath.Join(profileDir, "x.md"), []byte("# base prompt"), 0o644)

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, t.TempDir(), bootstrap.Config{})
	runner.pollInterval = 10 * time.Second
	runner.cmdFactory = func(name string, args ...string) *exec.Cmd { return helperCommand(150) }
	finished := make(chan struct{}, 1)
	runner.onCmdFinished = func(cmd *exec.Cmd, err error) {
		select {
		case finished <- struct{}{}:
		default:
		}
	}
	pm := &prodRunManager{store: ps, runner: runner, hostDir: t.TempDir()}

	old, err := ps.create("story", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.update(old.ID, func(r *ProdRun) {
		r.Status = prodRunAwaitingReview
		r.StopReason = stopReasonFoundationReady
	}); err != nil {
		t.Fatal(err)
	}

	newRun, err := pm.ReviseFoundation(old.ID, "\u0111\u1ed5i twist Vol3")
	if err != nil {
		t.Fatalf("revise: %v", err)
	}
	<-finished

	if newRun.ID == old.ID {
		t.Fatal("revise must create a new run")
	}
	if oldRun := pm.Get(old.ID); oldRun == nil {
		t.Fatal("old awaiting_review run must be KEPT as a fallback after revise")
	} else if oldRun.Status != prodRunAwaitingReview {
		t.Fatalf("old run must stay awaiting_review, got %s", oldRun.Status)
	}
	if pm.Get(newRun.ID).RevisionNote != "\u0111\u1ed5i twist Vol3" {
		t.Fatalf("revision note not persisted: %q", pm.Get(newRun.ID).RevisionNote)
	}
	// Note appended to the spawned run's profile.md.
	data, err := os.ReadFile(filepath.Join(ps.runDir(newRun.ID), "profile.md"))
	if err != nil {
		t.Fatalf("read new profile: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "# base prompt") || !strings.Contains(body, "\u0111\u1ed5i twist Vol3") {
		t.Fatalf("profile.md must contain base prompt + revision note, got: %q", body)
	}
}

// TestReviseFoundationRejectsEmptyOrWrongStatus guards the preconditions.
func TestReviseFoundationRejectsEmptyOrWrongStatus(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.WriteFile(filepath.Join(profileDir, "x.md"), []byte("# x"), 0o644)

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, t.TempDir(), bootstrap.Config{})
	pm := &prodRunManager{store: ps, runner: runner, hostDir: t.TempDir()}

	r, err := ps.create("queued", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pm.ReviseFoundation(r.ID, "note"); err == nil {
		t.Fatal("expected error revising a non-awaiting_review run")
	}
	if _, err := ps.update(r.ID, func(r *ProdRun) { r.Status = prodRunAwaitingReview }); err != nil {
		t.Fatal(err)
	}
	if _, err := pm.ReviseFoundation(r.ID, "   "); err == nil {
		t.Fatal("expected error on empty feedback")
	}
}

// TestReviseFoundationKeepsOldRunWhenNewFailsToStart verifies the round-3
// blocker fix: if the new run can't start (e.g. another run is active), the
// reviewed old candidate is preserved and the dead new run is cleaned up.
func TestReviseFoundationKeepsOldRunWhenNewFailsToStart(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.WriteFile(filepath.Join(profileDir, "x.md"), []byte("# x"), 0o644)

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, t.TempDir(), bootstrap.Config{})
	// Occupy the single run slot with a long-lived helper so revise's start
	// fails with errAnotherRunActive through the whole retry window.
	blocker, err := ps.create("blocker", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	runner.cmdFactory = func(name string, args ...string) *exec.Cmd { return helperCommand(5000) }
	if err := runner.start(blocker.ID); err != nil {
		t.Fatal(err)
	}
	defer runner.stop(blocker.ID)
	time.Sleep(50 * time.Millisecond)

	pm := &prodRunManager{store: ps, runner: runner, hostDir: t.TempDir()}
	old, err := ps.create("story", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.update(old.ID, func(r *ProdRun) { r.Status = prodRunAwaitingReview }); err != nil {
		t.Fatal(err)
	}
	before := len(ps.list())

	if _, err := pm.ReviseFoundation(old.ID, "note"); err == nil {
		t.Fatal("expected revise to fail while another run is active")
	}
	if pm.Get(old.ID) == nil {
		t.Fatal("old candidate must be preserved when revise fails to start")
	}
	if got := len(ps.list()); got != before {
		t.Fatalf("dead new run must be cleaned up: run count %d != %d", got, before)
	}
}

// TestApproveFoundationRevertsOnStartFailure verifies approve does not strand a
// run in queued (which can't be rejected) when start fails.
func TestApproveFoundationRevertsOnStartFailure(t *testing.T) {
	dir := t.TempDir()
	repoRoot := t.TempDir()
	profileDir := filepath.Join(repoRoot, "profiles")
	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.WriteFile(filepath.Join(profileDir, "x.md"), []byte("# x"), 0o644)

	ps, err := newProdRunStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, t.TempDir(), bootstrap.Config{})
	runner.cmdFactory = func(name string, args ...string) *exec.Cmd { return helperCommand(5000) }
	blocker, err := ps.create("blocker", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.start(blocker.ID); err != nil {
		t.Fatal(err)
	}
	defer runner.stop(blocker.ID)
	time.Sleep(50 * time.Millisecond)

	pm := &prodRunManager{store: ps, runner: runner, hostDir: t.TempDir()}
	r, err := ps.create("story", "profiles/x.md", "", "", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	metaDir := filepath.Join(ps.runDir(r.ID), "output", "novel", "meta")
	_ = os.MkdirAll(metaDir, 0o755)
	_ = os.WriteFile(filepath.Join(metaDir, "progress.json"), []byte(`{"phase":"writing","completed_chapters":[]}`), 0o644)
	if _, err := ps.update(r.ID, func(r *ProdRun) { r.Status = prodRunAwaitingReview }); err != nil {
		t.Fatal(err)
	}

	if _, err := pm.ApproveFoundation(r.ID); err == nil {
		t.Fatal("expected approve to fail while another run is active")
	}
	if got := pm.Get(r.ID).Status; got != prodRunAwaitingReview {
		t.Fatalf("approve failure must revert to awaiting_review, got %s", got)
	}
}
