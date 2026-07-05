package web

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Profile Library: read / create / edit / delete production profile .md files
// entirely from the Web UI, so the user never has to hand-edit files outside
// the app. Profiles are the SSOT artifact a run is created from — they are
// authored and reviewed here BEFORE a run exists, never auto-generated inside
// job creation. All new files are written to the PROJECT profiles dir
// (./.ainovel/profiles/); global/legacy profiles remain read-only here.
//
// Additive: lives entirely in internal/entry/web/, calls only exported
// helpers already present in this package (resolveExistingProfilePath,
// sanitizeFileName, profileBaseDir).

// MaxProfileBytes caps how large a profile file may be read/written.
const MaxProfileBytes = 256 << 10 // 256 KB — briefs are small; guards against abuse.

type profileContentResponse struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Source  string `json:"source"`
	Content string `json:"content"`
}

// handleProfileContent returns the raw markdown of one profile.
// Query: ?path=project/foo.md (validated + within-dir + symlink-guarded).
func (s *server) handleProfileContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	ref := strings.TrimSpace(r.URL.Query().Get("path"))
	if ref == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("path is required"))
		return
	}
	resolved, err := resolveExistingProfilePath(ref, s.repoRoot)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	info, err := os.Stat(resolved)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if info.Size() > MaxProfileBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, fmt.Errorf("profile exceeds %d byte limit", MaxProfileBytes))
		return
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	source, _, _ := strings.Cut(strings.ReplaceAll(ref, "\\", "/"), "/")
	writeJSON(w, http.StatusOK, profileContentResponse{
		Path:    ref,
		Name:    filepath.Base(resolved),
		Source:  source,
		Content: string(data),
	})
}

// handleProfileSave creates or overwrites a profile in the PROJECT profiles
// dir. Body: {name, content}. Name is sanitized to a bare .md filename; there
// is no way to write outside ./.ainovel/profiles/.
func (s *server) handleProfileSave(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var body struct {
		Name      string `json:"name"`
		Content   string `json:"content"`
		Overwrite bool   `json:"overwrite"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	name, err := profileFileName(body.Name)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(body.Content) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("content is required"))
		return
	}
	if len(body.Content) > MaxProfileBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, fmt.Errorf("content exceeds %d byte limit", MaxProfileBytes))
		return
	}
	base, err := profileBaseDir(profileSourceProject, s.repoRoot)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("create profiles dir: %w", err))
		return
	}
	dst := filepath.Join(base, name)
	// Defense in depth: dst must stay inside the project profiles dir.
	if !isWithinDir(dst, base) {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid profile name"))
		return
	}
	// Profile is a SSOT artifact — never silently clobber. Require an explicit
	// overwrite flag when the file already exists so the UI can confirm.
	if !body.Overwrite {
		if _, statErr := os.Stat(dst); statErr == nil {
			writeErr(w, http.StatusConflict, fmt.Errorf("profile %q already exists", name))
			return
		}
	}
	// safeWriteFile (tmp + rename with retry) rather than a raw write: the
	// profile is an SSOT artifact, and atomic replace tolerates Windows file
	// locks / avoids leaving a half-written profile on error.
	if err := safeWriteFile(dst, []byte(body.Content)); err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("write profile: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, profileItem{
		Name:   name,
		Path:   path.Join(profileSourceProject, name),
		Source: profileSourceProject,
	})
}

// handleProfileDelete removes a profile. Body: {path}. Only PROJECT profiles
// (./.ainovel/profiles/) may be deleted — global/legacy are read-only here, to
// match the UI contract and avoid clobbering the user's shared/global profiles
// or the repo's sample profiles via a crafted request.
func (s *server) handleProfileDelete(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	ref := strings.TrimSpace(body.Path)
	if ref == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("path is required"))
		return
	}
	resolved, err := resolveExistingProfilePath(ref, s.repoRoot)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	// Enforce project-only deletion by verifying the RESOLVED path lives inside
	// the project profiles dir (robust against source-string spoofing).
	projectBase, err := profileBaseDir(profileSourceProject, s.repoRoot)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !isWithinDir(resolved, projectBase) {
		writeErr(w, http.StatusForbidden, fmt.Errorf("only project profiles can be deleted"))
		return
	}
	if err := os.Remove(resolved); err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("delete profile: %w", err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// profileFileName turns a user-supplied name into a safe bare ".md" filename
// under the project profiles dir. Rejects path separators / traversal.
func profileFileName(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("name is required")
	}
	if strings.ContainsAny(raw, "/\\") || strings.Contains(raw, "..") {
		return "", fmt.Errorf("name must not contain path separators")
	}
	// Reject names that are only dots/spaces (would yield a hidden ".md" file).
	if strings.Trim(raw, ". ") == "" {
		return "", fmt.Errorf("name must contain at least one letter or digit")
	}
	name := sanitizeFileName(raw) // strips unsafe chars, never empty
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		name += ".md"
	}
	return name, nil
}
