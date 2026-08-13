package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// advance gate / reopen endpoint guards.
// Chỉ test input validation (method/decode/mode) — chạy trước workspaceMu.Lock.
// Không test concurrent-guard bằng lock-then-call: sync.Mutex không try-lock → deadlock
// (khác handleStart có blockIfRecoverable check trước Lock). Host behavior do upstream cover.

// TestHandleAdvanceModeRejectsBadMode: mode không hợp lệ → 400 (guard trước khi chạm eng).
func TestHandleAdvanceModeRejectsBadMode(t *testing.T) {
	s := &server{}
	req := httptest.NewRequest(http.MethodPost, "/api/advance/mode", strings.NewReader(`{"mode":"bogus"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleAdvanceMode(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "auto") || !strings.Contains(rec.Body.String(), "review") {
		t.Fatalf("error should mention auto/review, got: %s", rec.Body.String())
	}
}

// TestHandleAdvanceModeRejectsBadJSON: body hỏng → 400 (decodeJSON fail trước Lock).
func TestHandleAdvanceModeRejectsBadJSON(t *testing.T) {
	s := &server{}
	req := httptest.NewRequest(http.MethodPost, "/api/advance/mode", strings.NewReader(`{not json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleAdvanceMode(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

// TestHandleAdvanceModeRejectsGET: GET → 405 (requirePOST trước mọi thứ).
func TestHandleAdvanceModeRejectsGET(t *testing.T) {
	s := &server{}
	req := httptest.NewRequest(http.MethodGet, "/api/advance/mode", nil)
	rec := httptest.NewRecorder()
	s.handleAdvanceMode(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405, body=%s", rec.Code, rec.Body.String())
	}
}

// TestHandleAdvanceNextRejectsGET: GET → 405.
func TestHandleAdvanceNextRejectsGET(t *testing.T) {
	s := &server{}
	req := httptest.NewRequest(http.MethodGet, "/api/advance/next", nil)
	rec := httptest.NewRecorder()
	s.handleAdvanceNext(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405, body=%s", rec.Code, rec.Body.String())
	}
}

// TestHandleReopenRejectsGET: GET → 405.
func TestHandleReopenRejectsGET(t *testing.T) {
	s := &server{}
	req := httptest.NewRequest(http.MethodGet, "/api/reopen", nil)
	rec := httptest.NewRecorder()
	s.handleReopen(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405, body=%s", rec.Code, rec.Body.String())
	}
}

// TestHandleReopenRejectsBadJSON: body hỏng → 400.
func TestHandleReopenRejectsBadJSON(t *testing.T) {
	s := &server{}
	req := httptest.NewRequest(http.MethodPost, "/api/reopen", strings.NewReader(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handleReopen(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}
