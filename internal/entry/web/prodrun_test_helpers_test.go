package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
)

func setTestHome(t *testing.T, homeDir string) {
	t.Helper()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	volume := filepath.VolumeName(homeDir)
	pathPart := strings.TrimPrefix(homeDir, volume)
	if volume == "" {
		volume = filepath.VolumeName(filepath.Clean(homeDir))
	}
	t.Setenv("HOMEDRIVE", volume)
	t.Setenv("HOMEPATH", pathPart)
}

func writeWorkspaceProgress(t *testing.T, hostDir string, completed []int, phase domain.Phase) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(hostDir, "meta"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := domain.Progress{Phase: phase, CompletedChapters: completed}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hostDir, "meta", "progress.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
