package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestReveal_RejectsUnknownTarget: target lạ → 400, không gọi opener.
func TestReveal_RejectsUnknownTarget(t *testing.T) {
	called := false
	orig := revealOpen
	revealOpen = func(string) error { called = true; return nil }
	defer func() { revealOpen = orig }()

	s := &server{addr: "127.0.0.1:8787"}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/reveal", strings.NewReader(`{"target":"bogus"}`))
	s.handleReveal(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("target lạ nên trả 400, got %d", rr.Code)
	}
	if called {
		t.Fatal("không được mở file manager với target lạ")
	}
}

// TestReveal_BlockedOnPublicBind: bind non-loopback → 403 trước khi decode/mở gì.
func TestReveal_BlockedOnPublicBind(t *testing.T) {
	called := false
	orig := revealOpen
	revealOpen = func(string) error { called = true; return nil }
	defer func() { revealOpen = orig }()

	s := &server{addr: "0.0.0.0:8787"}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/reveal", strings.NewReader(`{"target":"novel"}`))
	s.handleReveal(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("public bind nên trả 403, got %d", rr.Code)
	}
	if called {
		t.Fatal("public bind không được chạy opener")
	}
}
