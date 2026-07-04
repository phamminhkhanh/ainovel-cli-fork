package web

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/voocel/ainovel-cli/internal/bootstrap"
	"github.com/voocel/ainovel-cli/internal/domain"
)

func TestHostHasProgress(t *testing.T) {
	hostDir := t.TempDir()

	// Empty host has no progress.
	has, err := hostHasProgress(hostDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if has {
		t.Fatal("expected no progress for empty host")
	}

	// Host with a chapter file has progress.
	if err := os.MkdirAll(filepath.Join(hostDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "chapters", "01.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	has, err = hostHasProgress(hostDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !has {
		t.Fatal("expected progress when chapter file exists")
	}

	// Host with progress.json containing completed chapters has progress.
	hostDir2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(hostDir2, "meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := domain.Progress{CompletedChapters: []int{1}}
	data, _ := json.Marshal(p)
	if err := os.WriteFile(filepath.Join(hostDir2, "meta", "progress.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	has, err = hostHasProgress(hostDir2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !has {
		t.Fatal("expected progress when CompletedChapters non-empty")
	}
}

func TestSyncRunOutputIntoHost(t *testing.T) {
	runDir := t.TempDir()
	hostDir := t.TempDir()

	// Set up run output.
	if err := os.MkdirAll(filepath.Join(runDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "chapters", "01.md"), []byte("chapter 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(runDir, "meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := domain.Progress{CompletedChapters: []int{1}, NovelName: "Test"}
	data, _ := json.Marshal(p)
	if err := os.WriteFile(filepath.Join(runDir, "meta", "progress.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "premise.md"), []byte("# Test"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := syncRunOutputIntoHost(runDir, hostDir, syncOptions{})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if res.CopiedFiles == 0 {
		t.Fatal("expected copied files")
	}

	// Verify files in host.
	if _, err := os.Stat(filepath.Join(hostDir, "chapters", "01.md")); err != nil {
		t.Fatalf("chapter not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(hostDir, "premise.md")); err != nil {
		t.Fatalf("premise not copied: %v", err)
	}
	progData, err := os.ReadFile(filepath.Join(hostDir, "meta", "progress.json"))
	if err != nil {
		t.Fatalf("progress not copied: %v", err)
	}
	var hp domain.Progress
	if err := json.Unmarshal(progData, &hp); err != nil {
		t.Fatal(err)
	}
	if len(hp.CompletedChapters) != 1 {
		t.Fatalf("expected 1 completed chapter, got %v", hp.CompletedChapters)
	}
}

func TestSyncRunOutputIntoHost_RejectsHostWithProgress(t *testing.T) {
	runDir := t.TempDir()
	hostDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(hostDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "chapters", "01.md"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := syncRunOutputIntoHost(runDir, hostDir, syncOptions{})
	if !errors.Is(err, errSyncHostHasProgress) {
		t.Fatalf("expected errSyncHostHasProgress, got %v", err)
	}
}

func TestSyncRunOutputIntoHost_ForceOverwrites(t *testing.T) {
	runDir := t.TempDir()
	hostDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(runDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "chapters", "02.md"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(runDir, "meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := domain.Progress{CompletedChapters: []int{2}}
	data, _ := json.Marshal(p)
	if err := os.WriteFile(filepath.Join(runDir, "meta", "progress.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(hostDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "chapters", "01.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := syncRunOutputIntoHost(runDir, hostDir, syncOptions{Force: true})
	if err != nil {
		t.Fatalf("force sync failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(hostDir, "chapters", "02.md")); err != nil {
		t.Fatalf("new chapter not copied: %v", err)
	}
	// Force mode clears host-only files before copying.
	if _, err := os.Stat(filepath.Join(hostDir, "chapters", "01.md")); err == nil {
		t.Fatal("host-only chapter should have been removed")
	}
	progData, err := os.ReadFile(filepath.Join(hostDir, "meta", "progress.json"))
	if err != nil {
		t.Fatalf("progress not overwritten: %v", err)
	}
	var hp domain.Progress
	if err := json.Unmarshal(progData, &hp); err != nil {
		t.Fatal(err)
	}
	if len(hp.CompletedChapters) != 1 || hp.CompletedChapters[0] != 2 {
		t.Fatalf("expected progress overwritten to chapter 2, got %v", hp.CompletedChapters)
	}
}

func TestSyncContinueRunOutputIntoHostFastForward(t *testing.T) {
	runDir := t.TempDir()
	hostDir := t.TempDir()
	writeWorkspaceProgress(t, hostDir, []int{1}, domain.PhaseWriting)
	if err := os.MkdirAll(filepath.Join(hostDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "chapters", "01.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	seed, err := seedMetaForWorkspace(hostDir)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(runDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "chapters", "02.md"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceProgress(t, runDir, []int{1, 2}, domain.PhaseWriting)

	res, err := syncContinueRunOutputIntoHost(runDir, hostDir, seed, syncOptions{})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if res.Mode != prodRunKindContinueWorkspace || !res.FastForward {
		t.Fatalf("expected continue fast-forward result, got %+v", res)
	}
	if _, err := os.Stat(filepath.Join(hostDir, "chapters", "02.md")); err != nil {
		t.Fatalf("new chapter not copied: %v", err)
	}
}

func TestSyncContinueRunOutputIntoHostDivergedRequiresForceAndBacksUp(t *testing.T) {
	runDir := t.TempDir()
	hostDir := t.TempDir()
	writeWorkspaceProgress(t, hostDir, []int{1}, domain.PhaseWriting)
	if err := os.MkdirAll(filepath.Join(hostDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "chapters", "01.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	seed, err := seedMetaForWorkspace(hostDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "chapters", "01.md"), []byte("user changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(runDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "chapters", "02.md"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceProgress(t, runDir, []int{1, 2}, domain.PhaseWriting)

	if _, err := syncContinueRunOutputIntoHost(runDir, hostDir, seed, syncOptions{}); !errors.Is(err, errSyncWorkspaceDiverged) {
		t.Fatalf("expected diverged error, got %v", err)
	}
	res, err := syncContinueRunOutputIntoHost(runDir, hostDir, seed, syncOptions{Force: true})
	if err != nil {
		t.Fatalf("force sync failed: %v", err)
	}
	if res.FastForward {
		t.Fatalf("forced diverged sync must not report fast-forward: %+v", res)
	}
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(hostDir), "backups", "pre-sync-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) == 0 {
		t.Fatal("expected force sync to create a pre-sync backup")
	}
}

func TestWorkspaceFingerprintIgnoresLogs(t *testing.T) {
	hostDir := t.TempDir()
	writeWorkspaceProgress(t, hostDir, []int{1}, domain.PhaseWriting)
	if err := os.MkdirAll(filepath.Join(hostDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "chapters", "01.md"), []byte("chapter 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	before, err := fingerprintHostWorkspace(hostDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(hostDir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "logs", "web.log"), []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	afterCreate, err := fingerprintHostWorkspace(hostDir)
	if err != nil {
		t.Fatal(err)
	}
	if afterCreate != before {
		t.Fatalf("log file must not affect fingerprint: before=%s after=%s", before, afterCreate)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "logs", "web.log"), []byte("first\nsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	afterAppend, err := fingerprintHostWorkspace(hostDir)
	if err != nil {
		t.Fatal(err)
	}
	if afterAppend != before {
		t.Fatalf("log append must not affect fingerprint: before=%s after=%s", before, afterAppend)
	}
}

func TestCopyTreeFileByFileExcludesLogs(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(srcDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "chapters", "02.md"), []byte("chapter 2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "logs", "headless.log"), []byte("noise"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "run.log"), []byte("root noise"), 0o644); err != nil {
		t.Fatal(err)
	}

	copied, err := copyTreeFileByFile(dstDir, srcDir)
	if err != nil {
		t.Fatal(err)
	}
	if copied != 1 {
		t.Fatalf("expected only chapter file copied, got %d", copied)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "chapters", "02.md")); err != nil {
		t.Fatalf("expected chapter copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "logs", "headless.log")); !os.IsNotExist(err) {
		t.Fatalf("logs must not be copied, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "run.log")); !os.IsNotExist(err) {
		t.Fatalf("root .log files must not be copied, stat err=%v", err)
	}
}

func TestProdRunManagerSync_RejectsActiveRun(t *testing.T) {
	jobsDir := t.TempDir()
	hostDir := t.TempDir()
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "profiles", "p.md"), []byte("# p"), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr, err := newProdRunManager(jobsDir, os.Args[0], repoRoot, hostDir, bootstrap.Config{})
	if err != nil {
		t.Fatal(err)
	}
	run, err := mgr.Create("test", "profiles/p.md", "", "", 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.store.update(run.ID, func(r *ProdRun) { r.Status = prodRunRunning }); err != nil {
		t.Fatal(err)
	}

	_, err = mgr.Sync(run.ID, syncOptions{})
	if !errors.Is(err, errSyncRunActive) {
		t.Fatalf("expected errSyncRunActive, got %v", err)
	}
}

func TestProdRunManagerSync_HappyPath(t *testing.T) {
	jobsDir := t.TempDir()
	hostDir := t.TempDir()
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "profiles", "p.md"), []byte("# p"), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr, err := newProdRunManager(jobsDir, os.Args[0], repoRoot, hostDir, bootstrap.Config{})
	if err != nil {
		t.Fatal(err)
	}
	run, err := mgr.Create("test", "profiles/p.md", "", "", 10, 5)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a completed run with one chapter.
	runDir := mgr.store.runDir(run.ID)
	outDir := filepath.Join(runDir, "output", "novel")
	if err := os.MkdirAll(filepath.Join(outDir, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "chapters", "01.md"), []byte("c1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(outDir, "meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := domain.Progress{CompletedChapters: []int{1}}
	data, _ := json.Marshal(p)
	if err := os.WriteFile(filepath.Join(outDir, "meta", "progress.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = mgr.Sync(run.ID, syncOptions{})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(hostDir, "chapters", "01.md")); err != nil {
		t.Fatalf("chapter not synced: %v", err)
	}
}
