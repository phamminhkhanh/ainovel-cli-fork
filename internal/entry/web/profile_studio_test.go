package web

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/voocel/agentcore"
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

// stubTimeoutModel emits one StreamEventError then closes, so runProfileGeneration's
// error path can be driven without a real provider.
type stubTimeoutModel struct{ err error }

func (stubTimeoutModel) Generate(context.Context, []agentcore.Message, []agentcore.ToolSpec, ...agentcore.CallOption) (*agentcore.LLMResponse, error) {
	return nil, errors.New("stubTimeoutModel: Generate not used")
}

func (m stubTimeoutModel) GenerateStream(context.Context, []agentcore.Message, []agentcore.ToolSpec, ...agentcore.CallOption) (<-chan agentcore.StreamEvent, error) {
	ch := make(chan agentcore.StreamEvent, 1)
	ch <- agentcore.StreamEvent{Type: agentcore.StreamEventError, Err: m.err}
	close(ch)
	return ch, nil
}

func (stubTimeoutModel) SupportsTools() bool { return false }

// TestRunProfileGenerationClassifiesTimeout locks the F4 contract: a model-side
// timeout (ctx deadline OR agentcore stream-idle) must surface as an error that
// agentcore.ClassifyProvider still maps to the right sentinel AFTER
// runProfileGeneration wraps it with "profile generate: %w". The handler's 504
// branch depends on this classification surviving the wrap.
func TestRunProfileGenerationClassifiesTimeout(t *testing.T) {
	s, _ := newTestServerForProdruns(t)
	for _, tc := range []struct {
		name     string
		err      error
		sentinel error
	}{
		{"context deadline", context.DeadlineExceeded, agentcore.ErrProviderTimeout},
		{"stream idle", agentcore.ErrProviderStreamIdle, agentcore.ErrProviderStreamIdle},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.runProfileGeneration(context.Background(), stubTimeoutModel{err: tc.err}, "rough idea")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			classified := agentcore.ClassifyProvider(err)
			if !errors.Is(classified, tc.sentinel) {
				t.Fatalf("ClassifyProvider(err) = %v, want %v (err=%v)", classified, tc.sentinel, err)
			}
		})
	}
}
