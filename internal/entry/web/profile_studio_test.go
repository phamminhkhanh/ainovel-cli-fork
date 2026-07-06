package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandleProfileGenerateRequiresIdea checks the idea guard fires before any
// model is touched (so it works without a configured provider).
func TestHandleProfileGenerateRequiresIdea(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	for _, body := range []string{`{}`, `{"idea":"   "}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/profiles/generate", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.handleProfileGenerate(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("missing idea should be 400, got %d for body %s", rec.Code, body)
		}
	}
}

func TestHandleProfileGenerateRejectsPartialModelOverride(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	for _, body := range []string{
		`{"idea":"x","provider":"kakalot"}`,
		`{"idea":"x","model":"deepseek-v4-pro"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/profiles/generate", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		s.handleProfileGenerate(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("partial model override should be 400, got %d for body %s", rec.Code, body)
		}
		if !strings.Contains(rec.Body.String(), "provider and model") {
			t.Fatalf("partial model override error = %s", rec.Body.String())
		}
	}
}

func TestStripCodeFence(t *testing.T) {
	cases := []struct{ in, want string }{
		{"# Title\nbody", "# Title\nbody"},
		{"```markdown\n# Title\nbody\n```", "# Title\nbody"},
		{"```\n# Title\n```", "# Title"},
		{"no fence here", "no fence here"},
	}
	for _, c := range cases {
		if got := stripCodeFence(c.in); got != c.want {
			t.Fatalf("stripCodeFence(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
