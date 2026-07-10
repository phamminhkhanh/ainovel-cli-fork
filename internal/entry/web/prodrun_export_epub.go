package web

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/voocel/ainovel-cli/internal/domain"
)

// EPUB 3 export for a production run — built WEB-SIDE (not via internal/host/exp).
//
// Why not reuse exp.Run: it hardcodes Chinese chapter labels "第 N 章" and
// xml:lang="zh-CN", which is wrong for this fork's VN/EN/ES novels. Here we use
// each chapter file's OWN first-line heading (the writer's "# Chương N: …" /
// "# Chapter N: …") verbatim, so headings are always in the story's language.
//
// Windows-safe: only READS chapters/*.md and atomic-writes its own .epub via
// safeWriteFile (temp + rename of the epub, never renames chapter files).

// epubLang is the dc:language / xml:lang metadata. Chapter HEADINGS come from
// the writer (any language); this tag is minor metadata most readers ignore for
// display. Defaults to the fork's primary market; parameterize per-run later if
// EN/ES metadata correctness matters.
const epubLang = "vi"

// exportRunEPUB builds {runDir}/export/{name}.epub from the run's completed
// chapters and returns the file path.
func exportRunEPUB(ps *prodRunStore, id string) (string, error) {
	r := ps.get(id)
	if r == nil {
		return "", errExportRunNotFound
	}
	novelDir := filepath.Join(ps.runDir(id), "output", "novel")

	prog, err := loadRunProgress(filepath.Join(novelDir, "meta", "progress.json"))
	if err != nil || prog == nil || len(prog.CompletedChapters) == 0 {
		return "", errExportNoChapters
	}

	chapters := append([]int(nil), prog.CompletedChapters...)
	sort.Ints(chapters)

	headings := make(map[int]string, len(chapters))
	bodies := make(map[int]string, len(chapters))
	got := 0
	for _, ch := range chapters {
		data, err := os.ReadFile(filepath.Join(novelDir, "chapters", fmt.Sprintf("%02d.md", ch)))
		if err != nil {
			continue // progress lists it but file missing — skip, don't fail whole export
		}
		heading, body := splitChapterHeadingBody(string(data))
		if heading == "" {
			heading = fmt.Sprintf("%d", ch)
		}
		headings[ch] = stripXMLIllegal(heading)
		bodies[ch] = stripXMLIllegal(body)
		got++
	}
	if got == 0 {
		return "", errExportNoChapters
	}
	// Keep only chapters we actually read.
	kept := chapters[:0]
	for _, ch := range chapters {
		if _, ok := bodies[ch]; ok {
			kept = append(kept, ch)
		}
	}
	chapters = kept

	data, err := renderRunEPUB(stripXMLIllegal(strings.TrimSpace(prog.NovelName)), chapters, headings, bodies)
	if err != nil {
		return "", fmt.Errorf("render epub: %w", err)
	}
	name := strings.TrimSpace(prog.NovelName)
	if name == "" {
		name = r.Name
	}
	outPath := filepath.Join(ps.runDir(id), "export", sanitizeFileName(name)+".epub")
	if err := safeWriteFile(outPath, data); err != nil {
		return "", fmt.Errorf("write epub: %w", err)
	}
	return outPath, nil
}

// ExportEPUB builds the run's EPUB and returns its path. Mirrors ExportTXT.
func (pm *prodRunManager) ExportEPUB(id string) (string, error) {
	return exportRunEPUB(pm.store, id)
}

func loadRunProgress(path string) (*domain.Progress, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p domain.Progress
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// splitChapterHeadingBody takes a chapter markdown file and returns the first
// H1 line (with leading '#'/spaces trimmed) as the heading and the rest as body.
// If the first non-empty line is not an H1, the whole text is body (no heading).
func splitChapterHeadingBody(md string) (heading, body string) {
	md = strings.ReplaceAll(md, "\r\n", "\n")
	parts := strings.SplitN(strings.TrimLeft(md, "\n"), "\n", 2)
	first := strings.TrimSpace(parts[0])
	if !strings.HasPrefix(first, "#") {
		return "", strings.TrimSpace(md)
	}
	heading = strings.TrimSpace(strings.TrimLeft(first, "#"))
	if len(parts) > 1 {
		body = strings.TrimSpace(parts[1])
	}
	return heading, body
}

// renderRunEPUB packs chapters into an EPUB 3 container. Structure/boilerplate
// mirror the engine's exporter (spec-correct), but headings are the writer's.
func renderRunEPUB(novelName string, chapters []int, headings, bodies map[int]string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// mimetype MUST be the first entry and stored (uncompressed), no BOM.
	mt, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		return nil, err
	}
	if _, err := mt.Write([]byte("application/epub+zip")); err != nil {
		return nil, err
	}

	if err := epubWrite(zw, "META-INF/container.xml", epubContainerXML); err != nil {
		return nil, err
	}
	if err := epubWrite(zw, "OEBPS/style.css", epubStyleCSS); err != nil {
		return nil, err
	}

	hasCover := strings.TrimSpace(novelName) != ""
	if hasCover {
		if err := epubWrite(zw, "OEBPS/cover.xhtml", epubCoverXHTML(novelName)); err != nil {
			return nil, err
		}
	}
	for _, ch := range chapters {
		if err := epubWrite(zw, "OEBPS/"+epubChapterFile(ch), epubChapterXHTML(headings[ch], bodies[ch])); err != nil {
			return nil, err
		}
	}
	if err := epubWrite(zw, "OEBPS/nav.xhtml", epubNavXHTML(hasCover, chapters, headings)); err != nil {
		return nil, err
	}
	if err := epubWrite(zw, "OEBPS/content.opf", epubOPF(novelName, hasCover, chapters)); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func epubWrite(zw *zip.Writer, name, content string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(content))
	return err
}

func epubChapterFile(ch int) string { return fmt.Sprintf("chapter%03d.xhtml", ch) }
func epubChapterID(ch int) string   { return fmt.Sprintf("ch%03d", ch) }

// stripXMLIllegal drops C0 control characters that are illegal in XML 1.0 even
// when entity-escaped (0x00–0x08, 0x0B, 0x0C, 0x0E–0x1F). Tab (0x09), LF (0x0A)
// and CR (0x0D) are the only control chars XML permits, so they are kept. A
// stray control char in chapter text would otherwise make the XHTML malformed
// and strict EPUB readers (epubcheck, Apple Books) reject the whole file.
func stripXMLIllegal(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return r
		}
		if r < 0x20 {
			return -1
		}
		return r
	}, s)
}

const epubContainerXML = `<?xml version="1.0" encoding="utf-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`

const epubStyleCSS = `body { font-family: serif; line-height: 1.7; margin: 1em; }
h1.book-title { font-size: 2em; text-align: center; margin: 4em 0 1em; }
h1.chapter-title { font-size: 1.4em; text-align: center; margin: 2em 0 1.5em; }
p { text-indent: 2em; margin: 0.5em 0; }
hr.scene { border: 0; text-align: center; margin: 1.5em 0; }
hr.scene::after { content: "* * *"; letter-spacing: 0.5em; }
`

func epubChapterXHTML(heading, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="%s">
<head>
  <title>%s</title>
  <link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body>
  <h1 class="chapter-title">%s</h1>
`, epubLang, html.EscapeString(heading), html.EscapeString(heading))
	for _, para := range epubSplitParagraphs(body) {
		if para == "---" {
			b.WriteString("  <hr class=\"scene\"/>\n")
			continue
		}
		fmt.Fprintf(&b, "  <p>%s</p>\n", html.EscapeString(para))
	}
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

// epubSplitParagraphs splits by blank line; a block that is only dashes becomes
// the sentinel "---" (rendered as a scene break); intra-paragraph newlines
// become spaces (XHTML <p> does not preserve them).
func epubSplitParagraphs(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	out := make([]string, 0, 16)
	for _, part := range strings.Split(body, "\n\n") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if strings.Trim(p, "-*= ") == "" {
			out = append(out, "---")
			continue
		}
		out = append(out, strings.ReplaceAll(p, "\n", " "))
	}
	return out
}

func epubCoverXHTML(novelName string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="%s">
<head>
  <title>%s</title>
  <link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body>
  <h1 class="book-title">%s</h1>
</body>
</html>
`, epubLang, html.EscapeString(novelName), html.EscapeString(novelName))
}

func epubNavXHTML(hasCover bool, chapters []int, headings map[int]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="%s">
<head>
  <title>Mục lục</title>
  <link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body>
  <nav epub:type="toc">
    <ol>
`, epubLang)
	if hasCover {
		b.WriteString("      <li><a href=\"cover.xhtml\">Bìa</a></li>\n")
	}
	for _, ch := range chapters {
		fmt.Fprintf(&b, "      <li><a href=\"%s\">%s</a></li>\n",
			epubChapterFile(ch), html.EscapeString(headings[ch]))
	}
	b.WriteString("    </ol>\n  </nav>\n</body>\n</html>\n")
	return b.String()
}

func epubOPF(novelName string, hasCover bool, chapters []int) string {
	title := strings.TrimSpace(novelName)
	if title == "" {
		title = "Untitled"
	}
	modified := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid" xml:lang="%s">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="bookid">%s</dc:identifier>
    <dc:title>%s</dc:title>
    <dc:language>%s</dc:language>
    <dc:creator>ainovel-cli</dc:creator>
    <meta property="dcterms:modified">%s</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="css" href="style.css" media-type="text/css"/>
`, epubLang, html.EscapeString(epubBookID(title)), html.EscapeString(title), epubLang, modified)
	if hasCover {
		b.WriteString(`    <item id="cover" href="cover.xhtml" media-type="application/xhtml+xml"/>` + "\n")
	}
	for _, ch := range chapters {
		fmt.Fprintf(&b, `    <item id="%s" href="%s" media-type="application/xhtml+xml"/>`+"\n",
			epubChapterID(ch), epubChapterFile(ch))
	}
	b.WriteString("  </manifest>\n  <spine>\n")
	if hasCover {
		b.WriteString(`    <itemref idref="cover"/>` + "\n")
	}
	b.WriteString(`    <itemref idref="nav"/>` + "\n")
	for _, ch := range chapters {
		fmt.Fprintf(&b, `    <itemref idref="%s"/>`+"\n", epubChapterID(ch))
	}
	b.WriteString("  </spine>\n</package>\n")
	return b.String()
}

// epubBookID derives a stable urn:uuid-style identifier from the title, so
// re-exporting the same book keeps the same ID (readers treat it as an update).
func epubBookID(title string) string {
	sum := sha1.Sum([]byte(title))
	return fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x",
		sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

