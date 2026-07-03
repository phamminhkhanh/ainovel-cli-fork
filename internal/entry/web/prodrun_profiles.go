package web

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/voocel/ainovel-cli/internal/bootstrap"
)

const (
	profileSourceProject = "project"
	profileSourceGlobal  = "global"
	profileSourceLegacy  = "legacy"
)

type profileRoot struct {
	source string
	dir    string
}

func profileRoots(repoRoot string) []profileRoot {
	roots := []profileRoot{
		{source: profileSourceProject, dir: filepath.Join(repoRoot, ".ainovel", "profiles")},
	}
	if base := bootstrap.DefaultConfigDir(); base != "" {
		roots = append(roots, profileRoot{source: profileSourceGlobal, dir: filepath.Join(base, "profiles")})
	}
	roots = append(roots, profileRoot{source: profileSourceLegacy, dir: filepath.Join(repoRoot, "profiles")})
	return roots
}

func listProfileItems(repoRoot string) ([]profileItem, error) {
	out := []profileItem{}
	for _, root := range profileRoots(repoRoot) {
		if root.dir == "" {
			continue
		}
		err := filepath.WalkDir(root.dir, func(filePath string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if entry.Type()&fs.ModeSymlink != 0 {
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
				return nil
			}
			rel, err := filepath.Rel(root.dir, filePath)
			if err != nil {
				return err
			}
			name := filepath.ToSlash(rel)
			out = append(out, profileItem{
				Name:   name,
				Path:   path.Join(root.source, name),
				Source: root.source,
			})
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return out, nil
}

func resolveExistingProfilePath(profile, repoRoot string) (string, error) {
	resolved, base, err := resolveProfilePathWithBase(profile, repoRoot)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("profile not found: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("profile must be a file")
	}
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", fmt.Errorf("resolve profile base: %w", err)
	}
	realPath, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve profile symlink: %w", err)
	}
	if !isWithinDir(realPath, realBase) {
		return "", fmt.Errorf("profile path outside profiles directory")
	}
	return resolved, nil
}

func resolveProfilePath(profile, repoRoot string) (string, error) {
	resolved, _, err := resolveProfilePathWithBase(profile, repoRoot)
	return resolved, err
}

func resolveProfilePathWithBase(profile, repoRoot string) (string, string, error) {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return "", "", fmt.Errorf("profile is required")
	}
	normalized := strings.ReplaceAll(profile, "\\", "/")
	if strings.HasPrefix(normalized, "/") || filepath.IsAbs(profile) {
		return "", "", fmt.Errorf("profile path must be relative")
	}
	cleanRef := path.Clean(normalized)
	if cleanRef == "." || strings.HasPrefix(cleanRef, "../") || cleanRef == ".." {
		return "", "", fmt.Errorf("profile path outside profiles directory")
	}

	source, rel, ok := strings.Cut(cleanRef, "/")
	if !ok || rel == "" {
		return "", "", fmt.Errorf("profile path must include a source")
	}

	base, err := profileBaseDir(source, repoRoot)
	if err != nil {
		return "", "", err
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".md") {
		return "", "", fmt.Errorf("profile must be a markdown file")
	}

	resolved := filepath.Clean(filepath.Join(base, filepath.FromSlash(rel)))
	if !isWithinDir(resolved, base) {
		return "", "", fmt.Errorf("profile path outside profiles directory")
	}
	return resolved, base, nil
}

func profileBaseDir(source, repoRoot string) (string, error) {
	switch source {
	case profileSourceProject:
		return filepath.Join(repoRoot, ".ainovel", "profiles"), nil
	case profileSourceGlobal:
		base := bootstrap.DefaultConfigDir()
		if base == "" {
			return "", fmt.Errorf("home config directory unavailable")
		}
		return filepath.Join(base, "profiles"), nil
	case profileSourceLegacy:
		return filepath.Join(repoRoot, "profiles"), nil
	case "profiles":
		return filepath.Join(repoRoot, "profiles"), nil
	default:
		return "", fmt.Errorf("unknown profile source %q", source)
	}
}

func isWithinDir(filePath, baseDir string) bool {
	rel, err := filepath.Rel(filepath.Clean(baseDir), filepath.Clean(filePath))
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
