package web

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sentinel errors returned by exportRunTXT so callers can choose the right
// HTTP status code.
var (
	errExportRunNotFound   = errors.New("run not found")
	errExportNoChaptersDir = errors.New("chapters directory not found")
	errExportNoChapters    = errors.New("no chapter files to export")
)

// exportRunTXT concatenates {runDir}/output/novel/chapters/*.md into a single
// TXT file under {runDir}/export/{runName}.txt. Chapters are sorted by file name
// (numeric when possible). It returns the absolute path of the exported file.
func exportRunTXT(store *prodRunStore, id string) (string, error) {
	r := store.get(id)
	if r == nil {
		return "", errExportRunNotFound
	}

	chaptersDir := filepath.Join(store.runDir(id), "output", "novel", "chapters")
	entries, err := os.ReadDir(chaptersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errExportNoChaptersDir
		}
		return "", fmt.Errorf("read chapters dir: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			files = append(files, name)
		}
	}
	if len(files) == 0 {
		return "", errExportNoChapters
	}
	sortChapterFiles(files)

	buf := bytesBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bytesBufferPool.Put(buf)

	for i, name := range files {
		if i > 0 {
			buf.WriteString("\n\n")
		}
		chapterLabel := strings.TrimSuffix(name, filepath.Ext(name))
		fmt.Fprintf(buf, "Chapter %s\n\n", chapterLabel)
		data, err := os.ReadFile(filepath.Join(chaptersDir, name))
		if err != nil {
			return "", fmt.Errorf("read chapter %s: %w", name, err)
		}
		buf.Write(data)
	}

	exportDir := filepath.Join(store.runDir(id), "export")
	outPath := filepath.Join(exportDir, sanitizeFileName(r.Name)+".txt")
	if err := safeWriteFile(outPath, buf.Bytes()); err != nil {
		return "", fmt.Errorf("write export file: %w", err)
	}
	return outPath, nil
}
