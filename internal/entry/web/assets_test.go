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

func TestEmbeddedHTMLHasWorkspaceTabs(t *testing.T) {
	html, err := assetFS.ReadFile("assets/index.html")
	if err != nil {
		t.Fatalf("read embedded html: %v", err)
	}
	text := string(html)
	for _, want := range []string{
		`id="workspaceTabs"`,
		`id="tab-stream"`,
		`id="tab-chapter"`,
		`id="tab-outline"`,
		`id="tab-world"`,
		`id="chapterText"`,
		`id="outlineDetail"`,
		`id="worldChars"`,
		`/app-workspace.js`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("index.html missing workspace tab element %q", want)
		}
	}
}

func TestEmbeddedJSWorkspaceHooksExist(t *testing.T) {
	js, err := assetFS.ReadFile("assets/app-workspace.js")
	if err != nil {
		t.Fatalf("read embedded workspace js: %v", err)
	}
	text := string(js)
	for _, want := range []string{
		"function switchTab(",
		"function selectChapter(",
		"function focusStreamTab(",
		"function loadOutlineTab(",
		"function loadWorldTab(",
		"sessionStorage.setItem(TAB_KEY",
		"/api/chapters/${n}",
		"/api/outline",
		"/api/world",
		"/api/characters",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("app-workspace.js missing hook or endpoint %q", want)
		}
	}
}

func TestEmbeddedJSDashboardCallsSelectChapter(t *testing.T) {
	js, err := assetFS.ReadFile("assets/app-dashboard.js")
	if err != nil {
		t.Fatalf("read embedded dashboard js: %v", err)
	}
	if !strings.Contains(string(js), "selectChapter(e.Chapter)") {
		t.Fatal("app-dashboard.js outline items must call selectChapter on click")
	}
}

func TestEmbeddedJSStreamSwitchesToStreamOnSend(t *testing.T) {
	js, err := assetFS.ReadFile("assets/app.js")
	if err != nil {
		t.Fatalf("read embedded js: %v", err)
	}
	if !strings.Contains(string(js), "focusStreamTab()") {
		t.Fatal("app.js send() must switch back to Stream tab on start/continue/steer")
	}
}

func TestEmbeddedHTMLHasChapterListAndActions(t *testing.T) {
	html, err := assetFS.ReadFile("assets/index.html")
	if err != nil {
		t.Fatalf("read embedded html: %v", err)
	}
	text := string(html)
	for _, want := range []string{
		`id="actionsCard"`,
		`id="actionsStack"`,
		`id="chapterCard"`,
		`id="chapterItems"`,
		`id="progressSummary"`,
		`/app-chapters.js`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("index.html missing chapter/action element %q", want)
		}
	}
}

func TestEmbeddedJSChaptersHooksExist(t *testing.T) {
	js, err := assetFS.ReadFile("assets/app-chapters.js")
	if err != nil {
		t.Fatalf("read embedded chapters js: %v", err)
	}
	text := string(js)
	for _, want := range []string{
		"function renderChapterList(",
		"function renderActions(",
		"function renderProgress(",
		"function chapterStatus(",
		"function chapterCount(",
		"PendingRewrites",
		"InProgressChapter",
		"CompletedCount",
		"selectChapter",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("app-chapters.js missing hook %q", want)
		}
	}
}

func TestEmbeddedHTMLScriptOrderChaptersBeforeDashboard(t *testing.T) {
	html, err := assetFS.ReadFile("assets/index.html")
	if err != nil {
		t.Fatalf("read embedded html: %v", err)
	}
	text := string(html)
	chapIdx := strings.Index(text, "/app-chapters.js")
	dashIdx := strings.Index(text, "/app-dashboard.js")
	workIdx := strings.Index(text, "/app-workspace.js")
	prodIdx := strings.Index(text, "/app-production.js")
	if chapIdx < 0 || dashIdx < 0 || workIdx < 0 || prodIdx < 0 {
		t.Fatal("missing script references")
	}
	if workIdx >= chapIdx {
		t.Fatal("app-workspace.js must load before app-chapters.js (selectChapter dependency)")
	}
	if chapIdx >= dashIdx {
		t.Fatal("app-chapters.js must load before app-dashboard.js (renderChapterList dependency)")
	}
	if prodIdx >= workIdx {
		t.Fatal("app-production.js must load before app-workspace.js (loadProductionTab dependency)")
	}
}

func TestEmbeddedHTMLBootCriticalSelectors(t *testing.T) {
	html, err := assetFS.ReadFile("assets/index.html")
	if err != nil {
		t.Fatalf("read embedded html: %v", err)
	}
	text := string(html)
	for _, id := range []string{
		"sendBtn", "abortBtn", "resumeBtn", "clearStream",
		"settingsBtn", "cmdBtn", "jobBar", "log", "input",
		"novelName", "stateBadge", "progressFill",
		"chapters", "completed", "words", "phase", "flow",
		"agents", "ctx", "ctxFill", "model", "cost",
	} {
		if !strings.Contains(text, `id="`+id+`"`) {
			t.Fatalf("index.html missing boot-critical selector #%s", id)
		}
	}
}

func TestEmbeddedHTMLHasProductionTabAndScript(t *testing.T) {
	html, err := assetFS.ReadFile("assets/index.html")
	if err != nil {
		t.Fatalf("read embedded html: %v", err)
	}
	text := string(html)
	for _, want := range []string{
		`data-tab="production"`,
		`id="tab-production"`,
		`/app-production.js`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("index.html missing production tab element %q", want)
		}
	}
}

func TestEmbeddedJSProductionHooksExist(t *testing.T) {
	js, err := assetFS.ReadFile("assets/app-production.js")
	if err != nil {
		t.Fatalf("read embedded production js: %v", err)
	}
	text := string(js)
	for _, want := range []string{
		"function renderProductionTab(",
		"function loadProductionTab(",
		"/api/profiles",
		"/api/prodruns",
		"/api/prodruns/",
		"export.txt",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("app-production.js missing hook or endpoint %q", want)
		}
	}
}

func TestEmbeddedJSDashboardCallsChapterRendering(t *testing.T) {
	js, err := assetFS.ReadFile("assets/app-dashboard.js")
	if err != nil {
		t.Fatalf("read embedded dashboard js: %v", err)
	}
	text := string(js)
	for _, want := range []string{
		"renderChapterList",
		"renderActions",
		"renderProgress",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("app-dashboard.js must call %s from renderDashboard", want)
		}
	}
}
