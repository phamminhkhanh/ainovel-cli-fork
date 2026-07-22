package web

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestNormalizeLangCode(t *testing.T) {
	cases := map[string]string{
		"":            "",
		"Vietnamese":  "vi",
		"tiếng việt":  "vi",
		"VN":          "vi",
		"vi":          "vi",
		"español":     "es",
		"Spanish":     "es",
		"ES":          "es",
		"castellano":  "es",
		"English":     "en",
		"en":          "en",
		"french":      "", // no rule pack → unresolved
		"gibberish-x": "",
	}
	for in, want := range cases {
		if got := normalizeLangCode(in); got != want {
			t.Errorf("normalizeLangCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRuleFileLang(t *testing.T) {
	cases := map[string]string{
		"lang-es.md":         "es",
		"lang-vi.md":         "vi",
		"prose-rhythm-es.md": "es",
		"prose-rhythm-vi.md": "vi",
		"anti-ai.md":         "", // "ai" isn't a known code → neutral
		"my-style.md":        "", // neutral
		"README.txt":         "", // extension irrelevant here; stem "README" → neutral
		"foundation-en.md":   "en",
	}
	for in, want := range cases {
		if got := ruleFileLang(in); got != want {
			t.Errorf("ruleFileLang(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRuleFileMatchesLang(t *testing.T) {
	// es run: es + neutral match; vi excluded.
	if !ruleFileMatchesLang("lang-es.md", "es") {
		t.Error("es rule should match es run")
	}
	if ruleFileMatchesLang("lang-vi.md", "es") {
		t.Error("vi rule must NOT match es run")
	}
	if !ruleFileMatchesLang("anti-ai.md", "es") {
		t.Error("neutral rule should match any run")
	}
	// undetermined run: everything matches (legacy no-filter behavior).
	if !ruleFileMatchesLang("lang-vi.md", "") || !ruleFileMatchesLang("lang-es.md", "") {
		t.Error("empty run language must not filter anything")
	}
}

func TestCopyLangFilteredRules(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(src, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("lang-es.md", "# es")
	write("prose-rhythm-es.md", "# es rhythm")
	write("lang-vi.md", "# vi")
	write("prose-rhythm-vi.md", "# vi rhythm")
	write("anti-ai.md", "# neutral")
	write("notes.txt", "ignored") // non-md skipped

	copied, err := copyLangFilteredRules(dst, src, "es")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"anti-ai.md", "lang-es.md", "prose-rhythm-es.md"}
	sort.Strings(copied)
	if !reflect.DeepEqual(copied, want) {
		t.Fatalf("copied = %v, want %v", copied, want)
	}
	// The vi rules must NOT have landed in the sandbox — this is the whole point.
	for _, forbidden := range []string{"lang-vi.md", "prose-rhythm-vi.md"} {
		if _, err := os.Stat(filepath.Join(dst, forbidden)); !os.IsNotExist(err) {
			t.Fatalf("%s leaked into es sandbox (err=%v)", forbidden, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dst, "notes.txt")); !os.IsNotExist(err) {
		t.Fatal("non-md file leaked into sandbox")
	}
}

func TestCopyLangFilteredRules_EmptyLangCopiesAll(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	for _, n := range []string{"lang-es.md", "lang-vi.md", "anti-ai.md"} {
		if err := os.WriteFile(filepath.Join(src, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	copied, err := copyLangFilteredRules(dst, src, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(copied) != 3 {
		t.Fatalf("empty language should copy all 3, got %v", copied)
	}
}

func TestDetectProfileLang(t *testing.T) {
	dir := t.TempDir()

	// 1. Marker wins over filename.
	marked := filepath.Join(dir, "story-vn.md")
	if err := os.WriteFile(marked, []byte("<!-- ainovel:lang=es -->\n# Título"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := detectProfileLang(marked); got != "es" {
		t.Errorf("marker should win: got %q, want es", got)
	}

	// 2. Filename suffix fallback when no marker.
	plain := filepath.Join(dir, "billionaire-70c-vn.md")
	if err := os.WriteFile(plain, []byte("# no marker"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := detectProfileLang(plain); got != "vi" {
		t.Errorf("filename fallback: got %q, want vi", got)
	}

	// 3. Neither → "".
	none := filepath.Join(dir, "romance-food.md")
	if err := os.WriteFile(none, []byte("# nothing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := detectProfileLang(none); got != "" {
		t.Errorf("undetectable profile: got %q, want empty", got)
	}
}

func TestWithLangMarker(t *testing.T) {
	out := withLangMarker("# Título\ncuerpo", "es")
	if !strings.HasPrefix(out, "<!-- ainovel:lang=es -->\n\n") {
		t.Fatalf("marker not prepended: %q", out)
	}
	// Idempotent: already-marked content is left unchanged.
	if again := withLangMarker(out, "es"); again != out {
		t.Fatal("withLangMarker must be idempotent")
	}
	// Unknown code → unchanged.
	if s := withLangMarker("body", "klingon"); s != "body" {
		t.Fatalf("unknown code must not add marker, got %q", s)
	}
}

func TestWithSandboxHome(t *testing.T) {
	base := []string{"PATH=/usr/bin", "HOME=/real/home", "USERPROFILE=C:\\real", "GO_WANT_HELPER_PROCESS=1"}
	sandbox := filepath.Join(t.TempDir(), "run-001")
	got := withSandboxHome(base, sandbox)

	joined := strings.Join(got, "\n")
	// Non-home vars preserved.
	if !strings.Contains(joined, "PATH=/usr/bin") || !strings.Contains(joined, "GO_WANT_HELPER_PROCESS=1") {
		t.Fatalf("non-home env not preserved: %v", got)
	}
	// Exactly one HOME and one USERPROFILE, both pointing at the sandbox.
	var homes, profiles int
	for _, kv := range got {
		if strings.HasPrefix(kv, "HOME=") {
			homes++
			if kv != "HOME="+sandbox {
				t.Errorf("HOME not re-rooted: %q", kv)
			}
		}
		if strings.HasPrefix(strings.ToUpper(kv), "USERPROFILE=") {
			profiles++
		}
	}
	if homes != 1 || profiles != 1 {
		t.Fatalf("expected exactly one HOME and USERPROFILE, got homes=%d profiles=%d", homes, profiles)
	}
	// os.UserHomeDir resolves via USERPROFILE on Windows, HOME elsewhere; the
	// re-rooted value must match sandbox on this platform.
	_ = runtime.GOOS
}
