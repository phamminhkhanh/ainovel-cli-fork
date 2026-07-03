package web

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveProfilePathSources(t *testing.T) {
	homeDir := t.TempDir()
	setTestHome(t, homeDir)
	repoRoot := t.TempDir()

	cases := []struct {
		name    string
		profile string
		want    string
	}{
		{"project", "project/foo.md", filepath.Join(repoRoot, ".ainovel", "profiles", "foo.md")},
		{"global", "global/foo.md", filepath.Join(homeDir, ".ainovel", "profiles", "foo.md")},
		{"legacy", "legacy/foo.md", filepath.Join(repoRoot, "profiles", "foo.md")},
		{"old legacy", "profiles/foo.md", filepath.Join(repoRoot, "profiles", "foo.md")},
		{"nested", "project/romance/foo.md", filepath.Join(repoRoot, ".ainovel", "profiles", "romance", "foo.md")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveProfilePath(tc.profile, repoRoot)
			if err != nil {
				t.Fatalf("resolveProfilePath: %v", err)
			}
			if got != filepath.Clean(tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveProfilePathRejectsUnsafeRefs(t *testing.T) {
	homeDir := t.TempDir()
	setTestHome(t, homeDir)
	repoRoot := t.TempDir()

	for _, profile := range []string{
		"",
		"/tmp/foo.md",
		"other/foo.md",
		"project/../foo.md",
		"project/foo.txt",
		"global/../../config.json",
		"legacy/..\\secret.md",
	} {
		t.Run(profile, func(t *testing.T) {
			if got, err := resolveProfilePath(profile, repoRoot); err == nil {
				t.Fatalf("expected error, got %q", got)
			}
		})
	}
}

func TestListProfileItemsListsAllSourcesAndNestedFiles(t *testing.T) {
	homeDir := t.TempDir()
	setTestHome(t, homeDir)
	repoRoot := t.TempDir()

	files := []string{
		filepath.Join(repoRoot, ".ainovel", "profiles", "spike.md"),
		filepath.Join(homeDir, ".ainovel", "profiles", "spike.md"),
		filepath.Join(repoRoot, "profiles", "spike.md"),
		filepath.Join(repoRoot, ".ainovel", "profiles", "romance", "werewolf.md"),
		filepath.Join(repoRoot, ".ainovel", "profiles", "ignore.txt"),
	}
	for _, file := range files {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("# prompt"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := listProfileItems(repoRoot)
	if err != nil {
		t.Fatalf("listProfileItems: %v", err)
	}
	want := []profileItem{
		{Name: "romance/werewolf.md", Path: "project/romance/werewolf.md", Source: "project"},
		{Name: "spike.md", Path: "project/spike.md", Source: "project"},
		{Name: "spike.md", Path: "global/spike.md", Source: "global"},
		{Name: "spike.md", Path: "legacy/spike.md", Source: "legacy"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestResolveExistingProfilePathRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	homeDir := t.TempDir()
	setTestHome(t, homeDir)
	repoRoot := t.TempDir()

	outside := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	profileDir := filepath.Join(repoRoot, ".ainovel", "profiles")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(profileDir, "leak.md")); err != nil {
		t.Fatal(err)
	}

	if got, err := resolveExistingProfilePath("project/leak.md", repoRoot); err == nil {
		t.Fatalf("expected symlink escape error, got %q", got)
	}
}
