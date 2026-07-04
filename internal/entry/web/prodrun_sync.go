package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/voocel/ainovel-cli/internal/domain"
)

var (
	errSyncRunNotFound       = errors.New("run not found")
	errSyncRunActive         = errors.New("cannot sync an active run")
	errSyncRunNoOutput       = errors.New("run has no output to sync")
	errSyncHostHasProgress   = errors.New("workspace already has progress")
	errSyncWorkspaceDiverged = errors.New("workspace changed since this continue run was seeded")
)

// syncOptions controls whether the sync may overwrite an existing workspace.
type syncOptions struct {
	Force bool `json:"force"`
}

// syncResult reports what was copied into the host workspace.
type syncResult struct {
	CopiedFiles int    `json:"copiedFiles"`
	Mode        string `json:"mode"`
	FastForward bool   `json:"fastForward"`
}

// hostHasProgress reports whether the host workspace already contains completed
// chapters or final chapter files. It is used to prevent accidental overwrites.
func hostHasProgress(hostDir string) (bool, error) {
	progressPath := filepath.Join(hostDir, "meta", "progress.json")
	data, err := os.ReadFile(progressPath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read host progress: %w", err)
	}
	if err == nil {
		var p domain.Progress
		if err := json.Unmarshal(data, &p); err != nil {
			return false, fmt.Errorf("parse host progress: %w", err)
		}
		if len(p.CompletedChapters) > 0 {
			return true, nil
		}
	}

	// Also treat any existing chapter file as progress, because the engine may
	// have written chapters without a matching progress update.
	entries, err := os.ReadDir(filepath.Join(hostDir, "chapters"))
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("scan host chapters: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			return true, nil
		}
	}
	return false, nil
}

// syncRunOutputIntoHost copies selected files from a finished production run
// into the main host workspace. progress.json is copied last so the UI only
// updates after all data is in place.
func syncRunOutputIntoHost(runOutDir, hostDir string, opts syncOptions) (*syncResult, error) {
	if _, err := os.Stat(runOutDir); err != nil {
		if os.IsNotExist(err) {
			return nil, errSyncRunNoOutput
		}
		return nil, fmt.Errorf("stat run output dir: %w", err)
	}

	if !opts.Force {
		hasProgress, err := hostHasProgress(hostDir)
		if err != nil {
			return nil, err
		}
		if hasProgress {
			return nil, errSyncHostHasProgress
		}
	}

	if err := ensureHostDirs(hostDir); err != nil {
		return nil, err
	}

	if opts.Force {
		if err := clearHostWorkspace(hostDir); err != nil {
			return nil, fmt.Errorf("clear host workspace: %w", err)
		}
	}

	result := &syncResult{Mode: prodRunKindFreshProfile}

	// Foundation root files.
	foundationFiles := []string{
		"premise.md",
		"outline.json", "outline.md",
		"layered_outline.json", "layered_outline.md",
		"characters.json", "characters.md",
		"world_rules.json", "world_rules.md",
		"timeline.json", "timeline.md",
		"foreshadow_ledger.json", "foreshadow_ledger.md",
		"relationship_state.json", "relationship_state.md",
	}
	for _, f := range foundationFiles {
		n, err := copyFileIfExists(filepath.Join(hostDir, f), filepath.Join(runOutDir, f))
		if err != nil {
			return nil, fmt.Errorf("copy %s: %w", f, err)
		}
		result.CopiedFiles += n
	}

	// World/meta state files (excluding progress, usage, checkpoints, runtime).
	metaFiles := []string{
		"meta/compass.json",
		"meta/style_rules.json",
		"meta/state_changes.json",
		"meta/cast_ledger.json",
		"meta/signals/last_commit.json",
		"meta/signals/pending_commit.json",
		"meta/signals/last_review.json",
	}
	for _, f := range metaFiles {
		n, err := copyFileIfExists(filepath.Join(hostDir, f), filepath.Join(runOutDir, f))
		if err != nil {
			return nil, fmt.Errorf("copy %s: %w", f, err)
		}
		result.CopiedFiles += n
	}

	// Recursive directory copies.
	dirCopies := []struct{ src, dst string }{
		{filepath.Join(runOutDir, "chapters"), filepath.Join(hostDir, "chapters")},
		{filepath.Join(runOutDir, "drafts"), filepath.Join(hostDir, "drafts")},
		{filepath.Join(runOutDir, "summaries"), filepath.Join(hostDir, "summaries")},
		{filepath.Join(runOutDir, "reviews"), filepath.Join(hostDir, "reviews")},
		{filepath.Join(runOutDir, "meta", "snapshots"), filepath.Join(hostDir, "meta", "snapshots")},
	}
	for _, dc := range dirCopies {
		if _, err := os.Stat(dc.src); os.IsNotExist(err) {
			continue
		}
		n, err := copyDirRecursive(dc.dst, dc.src)
		if err != nil {
			return nil, fmt.Errorf("copy %s: %w", dc.src, err)
		}
		result.CopiedFiles += n
	}

	// Copy progress.json last so the sidebar updates atomically from the UI's
	// point of view.
	n, err := copyFileIfExists(filepath.Join(hostDir, "meta", "progress.json"), filepath.Join(runOutDir, "meta", "progress.json"))
	if err != nil {
		return nil, fmt.Errorf("copy progress.json: %w", err)
	}
	result.CopiedFiles += n

	return result, nil
}

func syncContinueRunOutputIntoHost(runOutDir, hostDir string, seed *SeedMeta, opts syncOptions) (*syncResult, error) {
	if _, err := os.Stat(runOutDir); err != nil {
		if os.IsNotExist(err) {
			return nil, errSyncRunNoOutput
		}
		return nil, fmt.Errorf("stat run output dir: %w", err)
	}
	if seed == nil || seed.Fingerprint == "" {
		return nil, fmt.Errorf("continue run is missing seed metadata")
	}

	fastForward := false
	current, err := fingerprintHostWorkspace(hostDir)
	if err != nil {
		return nil, err
	}
	if current == seed.Fingerprint {
		fastForward = true
	} else if !opts.Force {
		return nil, errSyncWorkspaceDiverged
	}

	if opts.Force {
		if err := backupHostWorkspace(hostDir); err != nil {
			return nil, fmt.Errorf("backup host workspace: %w", err)
		}
	}

	copied, err := copyTreeFileByFile(hostDir, runOutDir)
	if err != nil {
		return nil, err
	}
	return &syncResult{
		CopiedFiles: copied,
		Mode:        prodRunKindContinueWorkspace,
		FastForward: fastForward,
	}, nil
}

func backupHostWorkspace(hostDir string) error {
	if _, err := os.Stat(hostDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	parent := filepath.Dir(hostDir)
	backupRoot := filepath.Join(parent, "backups")
	name := "pre-sync-" + time.Now().Format("20060102-150405")
	_, err := copyWorkspaceSeed(filepath.Join(backupRoot, name), hostDir)
	return err
}

func copyTreeFileByFile(dstDir, srcDir string) (int, error) {
	var files []string
	if err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if shouldExcludeWorkspaceSeed(relSlash, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dstPath := filepath.Join(dstDir, rel)
		if d.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return fmt.Errorf("create dir %s: %w", dstPath, err)
			}
			return nil
		}
		files = append(files, relSlash)
		return nil
	}); err != nil {
		return 0, err
	}
	sort.SliceStable(files, func(i, j int) bool {
		return copyPriority(files[i]) < copyPriority(files[j])
	})
	copied := 0
	for _, relSlash := range files {
		srcPath := filepath.Join(srcDir, filepath.FromSlash(relSlash))
		dstPath := filepath.Join(dstDir, filepath.FromSlash(relSlash))
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return copied, fmt.Errorf("read %s: %w", relSlash, err)
		}
		if err := safeWriteFile(dstPath, data); err != nil {
			return copied, fmt.Errorf("write %s: %w", relSlash, err)
		}
		copied++
	}
	return copied, nil
}

func copyPriority(relSlash string) int {
	relSlash = strings.ToLower(relSlash)
	if relSlash == "meta/progress.json" {
		return 2
	}
	return 1
}

// ensureHostDirs creates the standard subdirectories inside the host workspace.
func ensureHostDirs(hostDir string) error {
	dirs := []string{
		"chapters", "drafts", "summaries", "reviews",
		"meta", "meta/snapshots", "meta/signals",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(hostDir, d), 0o755); err != nil {
			return fmt.Errorf("create host dir %s: %w", d, err)
		}
	}
	return nil
}

// clearHostWorkspace removes the files/directories that sync copies into, so a
// force sync does not leave stale host-only files behind.
func clearHostWorkspace(hostDir string) error {
	paths := []string{
		"premise.md",
		"outline.json", "outline.md",
		"layered_outline.json", "layered_outline.md",
		"characters.json", "characters.md",
		"world_rules.json", "world_rules.md",
		"timeline.json", "timeline.md",
		"foreshadow_ledger.json", "foreshadow_ledger.md",
		"relationship_state.json", "relationship_state.md",
		"meta/compass.json",
		"meta/style_rules.json",
		"meta/state_changes.json",
		"meta/cast_ledger.json",
		"meta/signals/last_commit.json",
		"meta/signals/pending_commit.json",
		"meta/signals/last_review.json",
		"chapters", "drafts", "summaries", "reviews", "meta/snapshots",
	}
	for _, p := range paths {
		full := filepath.Join(hostDir, p)
		if err := os.RemoveAll(full); err != nil {
			return fmt.Errorf("remove %s: %w", p, err)
		}
	}
	return ensureHostDirs(hostDir)
}

// copyFileIfExists copies src to dst only if src exists. It returns 1 if a file
// was copied, 0 otherwise.
func copyFileIfExists(dst, src string) (int, error) {
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if err := copyFile(dst, src); err != nil {
		return 0, err
	}
	return 1, nil
}

// copyDirRecursive copies all files from srcDir to dstDir, preserving the
// directory structure. Empty directories are created but not counted.
func copyDirRecursive(dstDir, srcDir string) (int, error) {
	copied := 0
	if err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dstDir, rel)
		if d.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return fmt.Errorf("create dir %s: %w", dstPath, err)
			}
			return nil
		}
		if err := copyFile(dstPath, path); err != nil {
			return fmt.Errorf("copy %s: %w", path, err)
		}
		copied++
		return nil
	}); err != nil {
		return copied, err
	}
	return copied, nil
}
