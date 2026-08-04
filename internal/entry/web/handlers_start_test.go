package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/host"
)

func TestHandleStartRejectsConcurrentWorkspaceMutation(t *testing.T) {
	s := &server{}
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/api/start", strings.NewReader(`{"prompt":"new book"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleStart(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "workspace mutation already in progress") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestClassifyStartConflict(t *testing.T) {
	tests := []struct {
		name    string
		snap    host.UISnapshot
		force   bool
		code    string
		blocked bool
	}{
		{name: "no recovery", snap: host.UISnapshot{}, blocked: false},
		{name: "startup residue asks confirmation", snap: host.UISnapshot{RecoveryLabel: "恢复"}, code: "recoverable", blocked: true},
		{name: "forced startup residue proceeds", snap: host.UISnapshot{RecoveryLabel: "恢复"}, force: true, blocked: false},
		{name: "existing book blocks without force", snap: host.UISnapshot{RecoveryLabel: "恢复", CompletedCount: 3}, code: "existing_book", blocked: true},
		{name: "existing book still blocks with force", snap: host.UISnapshot{RecoveryLabel: "恢复", CompletedCount: 3}, force: true, code: "existing_book", blocked: true},
		{name: "completed book blocks without recovery label", snap: host.UISnapshot{CompletedCount: 3}, force: true, code: "existing_book", blocked: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, blocked := classifyStartConflict(tt.snap, tt.force)
			if code != tt.code || blocked != tt.blocked {
				t.Fatalf("got code=%q blocked=%v, want code=%q blocked=%v", code, blocked, tt.code, tt.blocked)
			}
		})
	}
}

func TestEmbeddedJSStartUsesSingleFlightGuard(t *testing.T) {
	js, err := assetFS.ReadFile("assets/app.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(js)
	for _, want := range []string{
		"let startInFlight = false", "if (startInFlight)", "sendBtn.disabled = true",
		"Đang khởi tạo…", "let shouldForce = !!force", "shouldForce = true", "continue;",
		"data.code === 'existing_book'",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("app.js missing start guard %q", want)
		}
	}
}
