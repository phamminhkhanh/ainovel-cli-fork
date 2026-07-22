package web

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Language-aware rule selection for production runs.
//
// Problem this solves: the engine scans EVERY *.md in the rules dir with no
// language awareness (internal/rules/raw.go). Left alone, a Spanish run also
// loads lang-vi.md / prose-rhythm-vi.md and the two rule sets fight inside
// user_rules (e.g. Vietnamese staccato vs Spanish long sentences).
//
// Fix, entirely in the web adapter (additive): each production run carries a
// canonical language code. prepareRunDir copies only the neutral rules plus the
// rules that match the run's language into the sandbox, and start() re-roots the
// child's HOME at the sandbox so the engine's "global" rules path resolves to
// that same filtered set instead of the user's real ~/.ainovel/rules. Result:
// the run sees only its own language's rules — no cross-language contamination.

// knownRuleLangs is the set of canonical language codes the fork ships rule
// packs for. Extend when a new language pack is added.
var knownRuleLangs = map[string]bool{"vi": true, "es": true, "en": true}

// langAliases maps free-text (UI language field) and filename tokens to a
// canonical code. Keys are lower-cased; lookups lower-case their input.
var langAliases = map[string]string{
	"vi": "vi", "vn": "vi", "vie": "vi", "vietnamese": "vi",
	"tiếng việt": "vi", "tieng viet": "vi",
	"es": "es", "esp": "es", "spanish": "es",
	"español": "es", "espanol": "es", "castellano": "es",
	"en": "en", "eng": "en", "english": "en",
}

// normalizeLangCode maps arbitrary user text to a canonical rule-language code,
// or "" when it can't be resolved (caller keeps its no-filter fallback).
func normalizeLangCode(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	if c, ok := langAliases[s]; ok {
		return c
	}
	if knownRuleLangs[s] {
		return s
	}
	return ""
}

// ruleFileLang returns the canonical language code a rule file is scoped to,
// inferred from a trailing "-<code>" on the filename stem (e.g. "lang-es.md" →
// "es", "prose-rhythm-vi.md" → "vi"). Returns "" for language-neutral files —
// those with no recognizable code suffix are copied into every run regardless
// of language.
func ruleFileLang(name string) string {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	idx := strings.LastIndex(stem, "-")
	if idx < 0 || idx == len(stem)-1 {
		return ""
	}
	return normalizeLangCode(stem[idx+1:])
}

var profileLangMarker = regexp.MustCompile(`(?i)<!--\s*ainovel:lang\s*=\s*([a-zA-Zñ]+)\s*-->`)

// detectProfileLang resolves a profile's language without an explicit UI choice.
// Order: an embedded "<!-- ainovel:lang=xx -->" marker (authoritative, written
// by the Studio generator), then a filename suffix heuristic ("...-es.md",
// "...-vn.md"). Returns "" when neither yields a known code.
func detectProfileLang(profilePath string) string {
	if data, err := os.ReadFile(profilePath); err == nil {
		if m := profileLangMarker.FindSubmatch(data); m != nil {
			if c := normalizeLangCode(string(m[1])); c != "" {
				return c
			}
		}
	}
	return ruleFileLang(filepath.Base(profilePath))
}

// profileLangComment renders the machine-readable marker embedded at the top of
// generated profiles so a run can recover the language later. Empty code → "".
func profileLangComment(code string) string {
	code = normalizeLangCode(code)
	if code == "" {
		return ""
	}
	return fmt.Sprintf("<!-- ainovel:lang=%s -->", code)
}

// ruleFileMatchesLang reports whether a rule file should be copied for a run in
// language lang. Neutral files (no code suffix) always match. When lang is ""
// (undetermined) every file matches — preserving the pre-language behavior so
// nothing silently disappears for runs we couldn't classify.
func ruleFileMatchesLang(name, lang string) bool {
	fileLang := ruleFileLang(name)
	if fileLang == "" || lang == "" {
		return true
	}
	return fileLang == lang
}

// copyLangFilteredRules copies the *.md rule files from srcDir into dstDir,
// skipping files whose filename is scoped to a different language than lang.
// Returns the sorted list of copied filenames (for run.RuleFiles / UI display).
// Mirrors copyDirFiles' scan conventions (top-level, case-insensitive ".md").
func copyLangFilteredRules(dstDir, srcDir, lang string) ([]string, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, err
	}
	var copied []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		if !ruleFileMatchesLang(name, lang) {
			continue
		}
		if err := copyFile(filepath.Join(dstDir, name), filepath.Join(srcDir, name)); err != nil {
			return nil, err
		}
		copied = append(copied, name)
	}
	sort.Strings(copied)
	return copied, nil
}
