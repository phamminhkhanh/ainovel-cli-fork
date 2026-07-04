package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/voocel/ainovel-cli/internal/domain"
)

var workspaceSeedExcludePrefixes = []string{
	"diag/",
	"diagnostics/",
	"exports/",
	"logs/",
	"tmp/",
	"temp/",
}

var workspaceSeedExcludeNames = map[string]bool{
	".ds_store": true,
	"thumbs.db": true,
}

func loadWorkspaceProgress(hostDir string) (*domain.Progress, error) {
	data, err := os.ReadFile(filepath.Join(hostDir, "meta", "progress.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errSeedNoWorkspace
		}
		return nil, fmt.Errorf("read workspace progress: %w", err)
	}
	var p domain.Progress
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse workspace progress: %w", err)
	}
	if p.Phase == domain.PhaseComplete {
		return nil, errSeedWorkspaceComplete
	}
	if len(p.CompletedChapters) == 0 {
		return nil, errSeedNoWorkspace
	}
	return &p, nil
}

func seedMetaForWorkspace(hostDir string) (*SeedMeta, error) {
	progress, err := loadWorkspaceProgress(hostDir)
	if err != nil {
		return nil, err
	}
	fingerprint, err := fingerprintHostWorkspace(hostDir)
	if err != nil {
		return nil, err
	}
	return &SeedMeta{
		HostDir:           hostDir,
		CompletedChapters: len(progress.CompletedChapters),
		Fingerprint:       fingerprint,
		CapturedAt:        time.Now(),
	}, nil
}

func fingerprintHostWorkspace(hostDir string) (string, error) {
	relPaths, err := fingerprintPaths(hostDir)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, rel := range relPaths {
		data, err := os.ReadFile(filepath.Join(hostDir, filepath.FromSlash(rel)))
		if err != nil {
			return "", fmt.Errorf("read fingerprint file %s: %w", rel, err)
		}
		_, _ = h.Write([]byte("path:" + rel + "\n"))
		_, _ = h.Write([]byte("sha256:"))
		sum := sha256.Sum256(data)
		_, _ = h.Write([]byte(hex.EncodeToString(sum[:]) + "\n"))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fingerprintPaths(hostDir string) ([]string, error) {
	seen := make(map[string]bool)
	var out []string
	add := func(rel string) {
		rel = filepath.ToSlash(filepath.Clean(rel))
		if rel == "." || rel == "" || seen[rel] {
			return
		}
		seen[rel] = true
		out = append(out, rel)
	}
	if err := filepath.WalkDir(hostDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(hostDir, path)
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
		if d.IsDir() {
			return nil
		}
		add(relSlash)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func copyWorkspaceSeed(dstDir, srcDir string) (int, error) {
	copied := 0
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
				return fmt.Errorf("create seed dir %s: %w", dstPath, err)
			}
			return nil
		}
		if err := copyFile(dstPath, path); err != nil {
			return fmt.Errorf("copy seed file %s: %w", relSlash, err)
		}
		copied++
		return nil
	}); err != nil {
		return copied, err
	}
	return copied, nil
}

func shouldExcludeWorkspaceSeed(relSlash string, d fs.DirEntry) bool {
	name := strings.ToLower(d.Name())
	if workspaceSeedExcludeNames[name] || strings.HasSuffix(name, ".tmp") || strings.HasSuffix(name, ".lock") || strings.HasSuffix(name, ".log") {
		return true
	}
	relLower := strings.ToLower(relSlash)
	if d.IsDir() && (strings.HasPrefix(name, ".") || name == "node_modules") {
		return true
	}
	for _, prefix := range workspaceSeedExcludePrefixes {
		if strings.HasPrefix(relLower, prefix) {
			return true
		}
	}
	return false
}
