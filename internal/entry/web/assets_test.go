package web

import (
	"strings"
	"testing"
)

func TestEmbeddedCSSPreservesHiddenAttribute(t *testing.T) {
	css, err := assetFS.ReadFile("assets/app.css")
	if err != nil {
		t.Fatalf("read embedded css: %v", err)
	}
	text := string(css)
	if !strings.Contains(text, "[hidden]") || !strings.Contains(text, "display: none !important") {
		t.Fatalf("app.css must preserve native hidden semantics; modal overlays set display:flex")
	}
}

func TestEmbeddedJSRoutesThinkingStream(t *testing.T) {
	js, err := assetFS.ReadFile("assets/app.js")
	if err != nil {
		t.Fatalf("read embedded js: %v", err)
	}
	text := string(js)
	for _, want := range []string{
		`const THINKING_SEP = '\x02'`,
		"let streamIsThinking = false",
		"appendThinking(part)",
		"appendDraft(part)",
		"return appendedDraft",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("app.js missing thinking-stream routing %q", want)
		}
	}
}

func TestEmbeddedJSCoCreateStartUsesRecoverableGuard(t *testing.T) {
	studio, err := assetFS.ReadFile("assets/app-studio.js")
	if err != nil {
		t.Fatalf("read embedded studio js: %v", err)
	}
	text := string(studio)
	if !strings.Contains(text, "startNovel(draft, false)") {
		t.Fatal("cold co-create start must reuse startNovel so recoverable sessions get confirm+force handling")
	}
	if strings.Contains(text, "'/api/cocreate/start', { prompt: draft }") {
		t.Fatal("cold co-create start must not bypass recoverable guard with raw post")
	}
}

func TestEmbeddedJSTranslatesEngineEventSummaries(t *testing.T) {
	js, err := assetFS.ReadFile("assets/app.js")
	if err != nil {
		t.Fatalf("read embedded js: %v", err)
	}
	if !strings.Contains(string(js), "translateSummary(ev.Summary)") {
		t.Fatalf("app.js must call translateSummary on event summaries")
	}
	i18n, err := assetFS.ReadFile("assets/app-i18n.js")
	if err != nil {
		t.Fatalf("read embedded i18n js: %v", err)
	}
	text := string(i18n)
	for _, want := range []string{
		"function translateSummary(",
		"'开始创作': 'Bắt đầu sáng tác'",
		"const EVENT_PREFIX_MAP",
		"const RETRY_RE",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("app-i18n.js missing event-summary i18n %q", want)
		}
	}
}

func TestEmbeddedJSSSEBootBuffersLiveFrames(t *testing.T) {
	js, err := assetFS.ReadFile("assets/app.js")
	if err != nil {
		t.Fatalf("read embedded js: %v", err)
	}
	text := string(js)
	for _, want := range []string{
		"const pendingLive = []",
		"m.type === 'hello'",
		"pendingLive.push(m)",
		"while (pendingLive.length) handle(pendingLive.shift())",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("app.js missing SSE boot buffering guard %q", want)
		}
	}
}

func TestEmbeddedHTMLHasThinkingStreamBox(t *testing.T) {
	html, err := assetFS.ReadFile("assets/index.html")
	if err != nil {
		t.Fatalf("read embedded html: %v", err)
	}
	text := string(html)
	for _, want := range []string{
		`id="thinkingStream"`,
		"thinking-box",
		"thinking-head",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("index.html missing thinking box %q", want)
		}
	}
}
