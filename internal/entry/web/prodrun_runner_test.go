package web

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/voocel/ainovel-cli/internal/bootstrap"
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
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, bootstrap.Config{})
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
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, bootstrap.Config{})
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
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, bootstrap.Config{})
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
	rr := newProdRunRunner(ps, "ainovel-cli", repoRoot, bootstrap.Config{})

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
	runner := newProdRunRunner(ps, "ainovel-cli", repoRoot, bootstrap.Config{})
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
