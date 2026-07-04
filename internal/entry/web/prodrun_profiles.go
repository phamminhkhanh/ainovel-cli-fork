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

var evalProfileSymlinks = filepath.EvalSymlinks

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
	realBase, realPath, err := resolveProfileRealPaths(base, resolved)
	if err != nil {
		return "", err
	}
	if !isWithinDir(realPath, realBase) {
		return "", fmt.Errorf("profile path outside profiles directory")
	}
	return resolved, nil
}

func resolveProfileRealPaths(base, resolved string) (string, string, error) {
	realBase, baseErr := evalProfileSymlinks(base)
	realPath, pathErr := evalProfileSymlinks(resolved)
	if baseErr == nil && pathErr == nil {
		return realBase, realPath, nil
	}

	// On some Windows test/runtime environments, filepath.EvalSymlinks returns
	// Access denied for ordinary directories under the temp root. Fall back to
	// absolute cleaned paths only after proving the profile path contains no
	// symlink segment, so a symlink escape cannot bypass the profiles boundary.
	if err := ensureNoProfileSymlinkSegments(base, resolved); err != nil {
		return "", "", err
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", "", fmt.Errorf("resolve profile base: %w", err)
	}
	absPath, err := filepath.Abs(resolved)
	if err != nil {
		return "", "", fmt.Errorf("resolve profile symlink: %w", err)
	}
	return filepath.Clean(absBase), filepath.Clean(absPath), nil
}

func ensureNoProfileSymlinkSegments(base, resolved string) error {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return fmt.Errorf("resolve profile base: %w", err)
	}
	absPath, err := filepath.Abs(resolved)
	if err != nil {
		return fmt.Errorf("resolve profile symlink: %w", err)
	}
	absBase = filepath.Clean(absBase)
	absPath = filepath.Clean(absPath)
	if !isWithinDir(absPath, absBase) {
		return fmt.Errorf("profile path outside profiles directory")
	}

	parts, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return fmt.Errorf("resolve profile symlink: %w", err)
	}
	paths := []string{absBase}
	cur := absBase
	for _, part := range strings.Split(parts, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		paths = append(paths, cur)
	}
	for _, candidate := range paths {
		info, err := os.Lstat(candidate)
		if err != nil {
			return fmt.Errorf("inspect profile symlink: %w", err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("profile path outside profiles directory")
		}
	}
	return nil
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
