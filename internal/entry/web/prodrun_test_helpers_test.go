package web

import (
	"path/filepath"
	"strings"
	"testing"
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
